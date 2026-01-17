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
