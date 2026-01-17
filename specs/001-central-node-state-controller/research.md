# Research: Centralized Node State Controller

**Date**: 2026-01-16
**Feature Branch**: `001-central-node-state-controller`

## Research Questions

This document resolves technical unknowns identified during planning.

---

## 1. Controller-Runtime source.Channel for Inter-Controller Communication

### Decision
Use `source.Channel` with `event.GenericEvent` to notify downstream controllers when central node state changes.

### Rationale
- `source.Channel` is the official controller-runtime mechanism for external/internal event sources
- Supports granular per-resource notifications (send GenericEvent with specific node object)
- Thread-safe with built-in buffering (default 1024 items)
- Integrates naturally with controller watches via `Watches()` builder method
- Non-blocking by design; uses `sync.Once` internally for startup safety

### Implementation Pattern

```go
// Central controller creates and owns channels
type CentralNodeStateController struct {
    // Channels for downstream notification
    nodeInsightEvents chan event.GenericEvent
    mcpInsightEvents  chan event.GenericEvent

    // Internal state
    nodeStates sync.Map  // string -> *NodeState
}

// Downstream controller watches the channel
func (r *NodeProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager, central *CentralNodeStateController) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&ouev1alpha1.NodeProgressInsight{}).
        Watches(
            source.Channel(
                central.nodeInsightEvents,
                &handler.EnqueueRequestForObject{},
            ),
        ).
        Complete(r)
}

// Central controller sends notifications
func (c *CentralNodeStateController) notifyNodeInsightControllers(ctx context.Context, node *corev1.Node) {
    select {
    case c.nodeInsightEvents <- event.GenericEvent{Object: node}:
        // Event sent
    case <-ctx.Done():
        // Shutdown
    }
}
```

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Watch internal CRD | Would add API latency; defeats purpose of avoiding duplicate evaluations |
| Shared callback interface | Less idiomatic; source.Channel integrates with controller-runtime lifecycle |
| Direct function calls | Bypasses workqueue; loses deduplication and rate limiting benefits |
| Custom event bus | Over-engineering; source.Channel provides exactly what's needed |

---

## 2. Internal Node State Data Structure

### Decision
Use a dedicated `NodeState` struct loosely coupled from `NodeProgressInsightStatus`, stored in a `sync.Map` keyed by node name.

### Rationale
- Allows internal representation to evolve independently from CRD schema
- `sync.Map` provides thread-safe access without explicit locking for typical read/write patterns
- Decoupling enables storing additional internal data (timestamps, metrics) not exposed in CRD
- Downstream controllers can access state directly via pointer (zero-copy for reads)

### Implementation Pattern

```go
// Internal state - richer than CRD status
type NodeState struct {
    // Core state (maps to CRD status)
    Name             string
    PoolRef          ouev1alpha1.ResourceRef
    Scope            ouev1alpha1.ScopeType
    Version          string
    Conditions       []metav1.Condition

    // Internal tracking (not in CRD)
    LastEvaluated    time.Time
    EvaluationCount  int64
    SourceGeneration int64  // Node resource version that triggered this state
}

// State store
type NodeStateStore struct {
    states sync.Map  // map[string]*NodeState
}

func (s *NodeStateStore) Get(nodeName string) (*NodeState, bool) {
    v, ok := s.states.Load(nodeName)
    if !ok {
        return nil, false
    }
    return v.(*NodeState), true
}

func (s *NodeStateStore) Set(nodeName string, state *NodeState) {
    s.states.Store(nodeName, state)
}
```

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Reuse `NodeProgressInsightStatus` directly | Couples internal and external representations; can't store internal metrics |
| Regular map with mutex | sync.Map is more efficient for concurrent read-heavy workloads |
| Per-node channels | Overhead for many nodes; harder to maintain; batch operations difficult |

---

## 3. Downstream Controller Registration Mechanism

### Decision
Pass channel references during controller construction in `cmd/main.go`. No runtime registration/deregistration.

### Rationale
- Controllers are static during operator lifetime (enabled/disabled at startup)
- Spec assumption: "Registration/deregistration operations are infrequent"
- Simpler than runtime registry; avoids synchronization complexity
- Matches existing pattern where controllers are configured in `main.go`

### Implementation Pattern

```go
// cmd/main.go
func main() {
    // Create central controller first
    centralController := nodestate.NewCentralNodeStateController(mgr.GetClient(), mgr.GetScheme())

    // Pass to downstream controllers
    if controllers.enableNode {
        nodeReconciler := controller.NewNodeProgressInsightReconciler(
            mgr.GetClient(),
            mgr.GetScheme(),
            centralController,  // Dependency injection
        )
        nodeReconciler.SetupWithManager(mgr)
    }

    // Register central controller with manager (for lifecycle)
    centralController.SetupWithManager(mgr)
}
```

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Runtime registry with Register/Deregister methods | Over-engineering for static controller set; adds synchronization complexity |
| Interface-based discovery | Adds abstraction without benefit; downstream controllers known at compile time |
| Service locator pattern | Anti-pattern; explicit dependency injection is clearer |

---

## 4. Metrics Implementation

### Decision
Use Prometheus metrics via controller-runtime's metrics registry. Implement custom collectors for node state evaluation and notification latency.

### Rationale
- Controller-runtime already sets up Prometheus metrics endpoint
- Custom metrics integrate naturally with existing `/metrics` endpoint
- Prometheus is the standard for Kubernetes operator observability
- Histogram metrics for latency provide percentile insights

### Implementation Pattern

```go
// internal/controller/nodestate/metrics.go
package nodestate

import (
    "github.com/prometheus/client_golang/prometheus"
    "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
    nodeStateEvaluationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "oue_node_state_evaluation_duration_seconds",
            Help:    "Duration of node state evaluation",
            Buckets: prometheus.DefBuckets,
        },
        []string{"node"},
    )

    nodeStateEvaluationTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "oue_node_state_evaluation_total",
            Help: "Total number of node state evaluations",
        },
        []string{"node", "result"},  // result: success, error
    )

    downstreamNotificationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "oue_downstream_notification_duration_seconds",
            Help:    "Duration to notify all downstream controllers",
            Buckets: prometheus.DefBuckets,
        },
        []string{"controller_type"},  // node_insight, mcp_insight
    )

    activeNodeStates = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "oue_active_node_states",
            Help: "Number of nodes with active state tracking",
        },
    )
)

func init() {
    metrics.Registry.MustRegister(
        nodeStateEvaluationDuration,
        nodeStateEvaluationTotal,
        downstreamNotificationDuration,
        activeNodeStates,
    )
}
```

### Metrics Defined

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `oue_node_state_evaluation_duration_seconds` | Histogram | node | Track per-node evaluation timing |
| `oue_node_state_evaluation_total` | Counter | node, result | Count evaluations and errors |
| `oue_downstream_notification_duration_seconds` | Histogram | controller_type | Track notification latency |
| `oue_active_node_states` | Gauge | - | Track state store size |

---

## 5. Tracing Implementation

### Decision
Use structured logging with correlation IDs rather than full distributed tracing (OpenTelemetry). Add tracing spans using controller-runtime's context-based logger.

### Rationale
- Full OpenTelemetry tracing adds significant complexity and dependencies
- Structured logging with correlation IDs (node name, reconcile ID) provides sufficient observability
- Controller-runtime uses zap logger which supports structured fields
- Can add OpenTelemetry later if needed without changing architecture

### Implementation Pattern

```go
// Use context logger with correlation fields
func (c *CentralNodeStateController) evaluateNode(ctx context.Context, node *corev1.Node) (*NodeState, error) {
    logger := log.FromContext(ctx).WithValues(
        "node", node.Name,
        "resourceVersion", node.ResourceVersion,
        "operation", "evaluateNode",
    )

    start := time.Now()
    defer func() {
        logger.V(1).Info("node state evaluation complete",
            "duration", time.Since(start),
        )
    }()

    logger.V(1).Info("starting node state evaluation")

    // ... evaluation logic ...

    return state, nil
}
```

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Full OpenTelemetry integration | Adds significant complexity; not required for initial implementation |
| No observability | Violates spec requirement for "full observability" |
| Custom tracing library | Reinventing the wheel; structured logging is sufficient |

---

## 6. Existing Code to Extract and Reuse

### Decision
Extract and relocate existing node state evaluation logic from `internal/controller/nodes/impl.go`.

### Components to Move to `internal/controller/nodestate/`

| Current Location | New Location | Purpose |
|------------------|--------------|---------|
| `nodes/mcpselectorcache.go` | `nodestate/mcpselectorcache.go` | MCP selector caching |
| `nodes/mcversioncache.go` | `nodestate/mcversioncache.go` | MachineConfig version caching |
| `nodes/impl.go:assessNode()` | `nodestate/assess.go` | Node state assessment logic |
| `nodes/impl.go:determineConditions()` | `nodestate/conditions.go` | Condition determination |

### Rationale
- Caches are needed by central controller, not downstream
- Assessment logic is the core of centralized evaluation
- Keeps node controller package focused on CRD management only

---

## 7. Controller Lifecycle and Shutdown

### Decision
Central controller implements `manager.Runnable` interface for proper lifecycle integration. Channels are closed during Stop().

### Implementation Pattern

```go
// Implement Runnable for manager integration
type CentralNodeStateController struct {
    // ... fields ...
    started bool
    mu      sync.Mutex
}

// Start is called by the manager
func (c *CentralNodeStateController) Start(ctx context.Context) error {
    c.mu.Lock()
    c.started = true
    c.mu.Unlock()

    // Block until context cancelled
    <-ctx.Done()

    // Cleanup
    close(c.nodeInsightEvents)
    close(c.mcpInsightEvents)

    return nil
}

// Register with manager
func (c *CentralNodeStateController) SetupWithManager(mgr ctrl.Manager) error {
    return mgr.Add(c)  // Adds as Runnable
}
```

### Rationale
- Manager handles graceful shutdown via context cancellation
- Closing channels signals downstream controllers cleanly
- Matches constitution requirement: "controllers remain registered with manager and follow standard lifecycle"

---

## Summary

All technical unknowns have been resolved:

1. **Inter-controller communication**: source.Channel with GenericEvent
2. **Internal state structure**: NodeState struct in sync.Map
3. **Registration mechanism**: Compile-time dependency injection in main.go
4. **Metrics**: Prometheus via controller-runtime registry
5. **Tracing**: Structured logging with correlation IDs
6. **Code reuse**: Extract existing assessment logic to nodestate package
7. **Lifecycle**: Implement manager.Runnable for proper shutdown

---

# Followup Research: Legacy Mode Removal

**Date**: 2026-01-17
**Phase**: Followup refactoring (removing dual-mode architecture)

## Followup Research Question 1: Automatic Controller Enablement Patterns

### Decision
Use simple boolean conditional in main.go before controller setup: `enableNodeState := controllers.enableNode`

### Rationale
- ✅ Simple and transparent (visible in main.go setup logic)
- ✅ No framework magic or hidden dependencies
- ✅ Easy to debug (log message explains why controller was enabled)
- ✅ Follows existing controller-runtime patterns (explicit setup order)

### Implementation Pattern

```go
// Automatic dependency enablement
enableNodeState := controllers.enableNode
if enableNodeState {
    setupLog.Info("CentralNodeState controller auto-enabled",
        "reason", "NodeProgressInsight controller is enabled")
}
```

**Note**: No MCP Progress Controller logic included. That controller does not exist yet and will be handled in future work.

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Dependency injection framework | Too heavyweight for single use case |
| Manager extension with dependency tracking | Would require custom manager wrapper, too complex |
| Environment variable | Less discoverable than command-line flag logic |

---

## Followup Research Question 2: Flag Removal Migration

### Decision
Remove `--enable-node-state-controller` flag entirely with documented breaking change.

### Rationale
- Spec explicitly states "breaking change acceptable"
- Standard practice for refactoring work
- Clean codebase with no legacy compatibility code
- Migration is simple: remove flag from deployment configs

### Go Flag Package Behavior
Go's flag.Parse() returns error "flag provided but not defined" for unknown flags. Users providing the removed flag will get a clear error on startup.

### Migration Documentation Required

```markdown
## Breaking Change: --enable-node-state-controller Removed

The `--enable-node-state-controller` flag has been removed. The Central Node State
Controller is now automatically enabled when the NodeProgressInsight controller is
enabled via `--enable-node-controller=true`.

**Migration**: Remove `--enable-node-state-controller` from deployment configurations.
No other changes required.
```

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Dummy flag with deprecation warning | Adds unnecessary code complexity |
| Silently ignore unknown flags | Would require custom flag parsing |

---

## Followup Research Question 3: Error Handling for Missing Dependency

### Decision
Fail fast in main.go setup logic with explicit nil check after automatic enablement, before NodeProgressInsight setup.

### Implementation Pattern

```go
if controllers.enableNode {
    if centralNodeState == nil {
        setupLog.Error(nil, "NodeProgressInsight controller requires CentralNodeState controller",
            "required-by", "NodeProgressInsight",
            "missing", "CentralNodeState",
            "fix", "This is a bug in automatic enablement logic")
        os.Exit(1)
    }
    nodeInformer := controller.NewNodeProgressInsightReconcilerWithProvider(
        mgr.GetClient(),
        mgr.GetScheme(),
        centralNodeState.GetStateProvider(),
    )
    if err = nodeInformer.SetupWithManager(mgr); err != nil {
        setupLog.Error(err, "unable to create controller", "controller", "NodeProgressInsight")
        os.Exit(1)
    }
}
```

### Additional Safety: Constructor Validation

```go
func NewNodeProgressInsightReconcilerWithProvider(
    client client.Client,
    scheme *runtime.Scheme,
    provider nodestate.NodeStateProvider,
) *NodeProgressInsightReconciler {
    if provider == nil {
        // This should never happen due to main.go check, but defensive
        panic("NodeProgressInsightReconciler requires non-nil NodeStateProvider")
    }
    return &NodeProgressInsightReconciler{
        Client:        client,
        Scheme:        scheme,
        stateProvider: provider,
    }
}
```

### Rationale
- ✅ Fails immediately during manager setup (before manager starts)
- ✅ Clear error message identifies missing dependency
- ✅ Logs suggest root cause ("bug in automatic enablement logic")
- ✅ No runtime errors or reconciliation failures

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| Silent degradation | Would hide bugs in automatic enablement |
| Runtime error in Reconcile | Harder to debug, continuous failures |
| Constructor validation only | Would panic, less clear error message |

---

## Followup Research Question 4: Test Coverage Verification

### Decision
Use phased test migration: audit → verify → validate

### Phased Approach

1. **Pre-deletion audit** (before removing any code):
   - Run `go test -cover` on current codebase
   - Document baseline coverage percentage
   - List all test cases in legacy mode

2. **Verification** (during refactoring):
   - Ensure provider mode tests cover 100% of functional scenarios
   - Add missing provider mode tests if needed
   - Target: No coverage decrease for provider mode code paths

3. **Post-deletion validation**:
   - Run `make test` after legacy mode removal
   - Verify all tests pass
   - Compare coverage percentage (should be equal or higher)

### Coverage Analysis

**Legacy mode test cases to audit**:
- ✅ Node state evaluation: Covered by central controller tests (nodestate package)
- ✅ Status updates: Covered by provider mode integration tests
- ✅ Watch setup: Covered by provider mode (SetupWithManager unchanged)
- ⚠️ **Standalone reconciliation**: Legacy mode reconciles independently; provider mode uses central controller (intentionally removed)

**Coverage gate**: `make test` must pass with no failing tests after legacy mode removal.

### Alternatives Considered

| Alternative | Rejected Because |
|-------------|------------------|
| No formal audit | Risky, might miss test coverage gaps |
| Convert legacy tests to provider tests | Unnecessary duplication, provider tests already exist |
| Keep legacy tests as "defense in depth" | Would test non-existent code path |

---

## Followup Summary

### Key Decisions for Refactoring

| Research Question | Decision | Rationale |
|------------------|----------|-----------|
| Automatic enablement pattern | Simple conditional in main.go | Transparent, debuggable, follows controller-runtime patterns |
| Flag removal handling | Remove entirely, document breaking change | Clean removal, spec allows breaking changes |
| Missing dependency error | Fail fast in main.go setup | Clear error message, fails before manager starts |
| Test coverage verification | Phased audit → verify → validate | Ensures no coverage loss, systematic approach |

### Implementation Checklist

From research findings, the followup refactoring requires:

- [ ] Add automatic enablement logic before controller setup
- [ ] Add nil check for central controller before NodeProgressInsight setup
- [ ] Remove `--enable-node-state-controller` flag declaration
- [ ] Remove `enableNodeState` from controllersConfig struct
- [ ] Remove legacy constructor `NewNodeProgressInsightReconciler`
- [ ] Remove `impl` field from NodeProgressInsightReconciler
- [ ] Add nil check in `NewNodeProgressInsightReconcilerWithProvider` constructor
- [ ] Audit provider mode test coverage before removing legacy tests
- [ ] Remove legacy mode test cases
- [ ] Document breaking change in CLAUDE.md or migration guide
- [ ] Validate `make test` passes after all changes

### Open Questions (Resolved)

1. **MCP Progress Controller flag**: ✅ RESOLVED
   - **Decision**: Do NOT include any MCP Progress Controller logic
   - **Rationale**: Controller does not exist yet, will be handled in future work
   - **Code**: `enableNodeState := controllers.enableNode`

2. **Flag deprecation warning**: ✅ RESOLVED
   - **Decision**: Remove flag entirely, no backward compatibility needed
   - **Rationale**: Still in development phase, clean removal is better

3. **Provider nil check location**: ✅ RESOLVED
   - **Decision**: Both constructor AND main.go
   - **Rationale**: Primary check in main.go (clear error), defensive check in constructor (panic)
