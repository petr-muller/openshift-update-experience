# Go Interface Contracts: Centralized Node State Controller

**Date**: 2026-01-16
**Feature Branch**: `001-central-node-state-controller`

This document defines the Go interfaces that represent contracts between components. These are **design specifications**, not actual code files.

---

## 1. NodeStateProvider Interface

**Purpose**: Contract for components that provide access to evaluated node state.

**Implementer**: `CentralNodeStateController`
**Consumers**: `NodeProgressInsightReconciler`, future `MCPProgressInsightReconciler`

```go
// Package: internal/controller/nodestate

// NodeStateProvider provides read access to evaluated node states.
// This interface allows downstream controllers to access the centrally
// maintained node state without direct coupling to the central controller.
type NodeStateProvider interface {
    // GetNodeState returns the current evaluated state for a node.
    // Returns (state, true) if found, (nil, false) if not tracked.
    GetNodeState(nodeName string) (*NodeState, bool)

    // GetAllNodeStates returns all currently tracked node states.
    // Used for bulk operations like MCP summary calculations.
    GetAllNodeStates() []*NodeState

    // GetNodeStatesByPool returns all nodes belonging to a specific MCP.
    // Used by MCP insight controller to calculate pool summaries.
    GetNodeStatesByPool(poolName string) []*NodeState

    // NodeInsightChannel returns a receive-only channel for node insight
    // notifications. Downstream controllers watch this to trigger reconciliation.
    NodeInsightChannel() <-chan event.GenericEvent

    // MCPInsightChannel returns a receive-only channel for MCP insight
    // notifications. Used by future MCP progress insight controller.
    MCPInsightChannel() <-chan event.GenericEvent
}
```

---

## 2. StateEvaluator Interface

**Purpose**: Contract for the node state evaluation logic.

**Implementer**: `CentralNodeStateController` (internal use)
**Used by**: Testing, potential future alternative implementations

```go
// Package: internal/controller/nodestate

// StateEvaluator evaluates the update state of a node given its
// associated resources. This interface encapsulates the evaluation
// logic for testability.
type StateEvaluator interface {
    // EvaluateNode computes the current state of a node based on:
    // - The Node resource itself
    // - Its associated MachineConfigPool
    // - The ClusterVersion (for target version)
    // - Existing conditions (for timestamp preservation)
    //
    // Returns the evaluated NodeState or an error if evaluation fails.
    EvaluateNode(
        ctx context.Context,
        node *corev1.Node,
        mcp *machineconfigurationv1.MachineConfigPool,
        clusterVersion *configv1.ClusterVersion,
        existingConditions []metav1.Condition,
    ) (*NodeState, error)
}
```

---

## 3. CacheManager Interface

**Purpose**: Contract for managing the MCP selector and MC version caches.

**Implementer**: `CentralNodeStateController` (embeds caches)
**Used by**: Internal implementation

```go
// Package: internal/controller/nodestate

// MCPSelectorCache manages MachineConfigPool label selector caching.
type MCPSelectorCache interface {
    // WhichMCP returns the name of the MachineConfigPool that matches
    // the given node labels. Returns empty string if no match.
    WhichMCP(nodeLabels labels.Labels) string

    // Ingest adds or updates a cached selector for an MCP.
    // Returns (modified, reason) indicating if cache changed.
    Ingest(mcpName string, selector *metav1.LabelSelector) (bool, string)

    // Forget removes a cached selector.
    // Returns true if it was present.
    Forget(mcpName string) bool
}

// MCVersionCache manages MachineConfig to version mappings.
type MCVersionCache interface {
    // VersionFor returns the OCP version for a MachineConfig.
    // Returns (version, true) if found, ("", false) otherwise.
    VersionFor(machineConfigName string) (string, bool)

    // Ingest adds or updates a version mapping from an MC resource.
    // Returns true if the cache was modified.
    Ingest(mc *machineconfigurationv1.MachineConfig) bool

    // Forget removes a cached version mapping.
    // Returns true if it was present.
    Forget(mcName string) bool
}
```

---

## 4. NotificationSender Interface

**Purpose**: Contract for sending state change notifications to downstream controllers.

**Implementer**: `CentralNodeStateController`
**Used by**: Internal implementation, testing

```go
// Package: internal/controller/nodestate

// NotificationSender sends state change events to downstream controllers.
type NotificationSender interface {
    // NotifyNodeInsightControllers sends a notification that a node's
    // state has changed. The downstream NodeProgressInsight controller
    // will be triggered to reconcile.
    NotifyNodeInsightControllers(ctx context.Context, node *corev1.Node) error

    // NotifyMCPInsightControllers sends a notification that MCP-level
    // state may have changed (e.g., node added/removed from pool, or
    // member node state changed). The downstream MCPProgressInsight
    // controller will be triggered to reconcile.
    NotifyMCPInsightControllers(ctx context.Context, poolName string) error

    // NotifyAllDownstream sends notifications to all downstream
    // controllers. Used when global state changes (e.g., ClusterVersion).
    NotifyAllDownstream(ctx context.Context) error
}
```

---

## 5. Metrics Interface

**Purpose**: Contract for recording observability metrics.

**Implementer**: Prometheus-based implementation in `metrics.go`
**Used by**: `CentralNodeStateController`

```go
// Package: internal/controller/nodestate

// MetricsRecorder records metrics for the central node state controller.
type MetricsRecorder interface {
    // RecordEvaluation records a node state evaluation event.
    RecordEvaluation(nodeName string, duration time.Duration, success bool)

    // RecordNotification records a downstream notification event.
    RecordNotification(controllerType DownstreamControllerType, duration time.Duration)

    // SetActiveNodeCount updates the gauge of active node states.
    SetActiveNodeCount(count int)

    // RecordStateTransition records a node state phase transition.
    RecordStateTransition(nodeName string, fromPhase, toPhase UpdatePhase)
}
```

---

## 6. Controller Lifecycle Interface

**Purpose**: Ensure central controller integrates with controller-runtime manager.

**Implementer**: `CentralNodeStateController`
**Used by**: `ctrl.Manager`

```go
// Package: internal/controller/nodestate

// The CentralNodeStateController must implement these interfaces
// from controller-runtime to integrate with the manager:

// manager.Runnable - for lifecycle management
type Runnable interface {
    Start(ctx context.Context) error
}

// healthz.Checker - for health checks (optional but recommended)
type Checker interface {
    Check(req *http.Request) error
}
```

---

## Interface Relationships

```
┌─────────────────────────────────────────────────────────────────┐
│                  CentralNodeStateController                      │
│                                                                  │
│  implements:                                                     │
│    - NodeStateProvider                                          │
│    - StateEvaluator                                             │
│    - NotificationSender                                         │
│    - manager.Runnable                                           │
│                                                                  │
│  uses:                                                          │
│    - MCPSelectorCache (embedded)                                │
│    - MCVersionCache (embedded)                                  │
│    - MetricsRecorder (dependency)                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ provides NodeStateProvider
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              Downstream Controllers                              │
│                                                                  │
│  NodeProgressInsightReconciler                                  │
│    - Receives: NodeInsightChannel()                             │
│    - Reads: GetNodeState()                                      │
│                                                                  │
│  MCPProgressInsightReconciler (future)                          │
│    - Receives: MCPInsightChannel()                              │
│    - Reads: GetNodeStatesByPool()                               │
└─────────────────────────────────────────────────────────────────┘
```

---

## Design Notes

### Why Interfaces?

1. **Testability**: Mock implementations for unit tests
2. **Decoupling**: Downstream controllers depend on interface, not concrete type
3. **Documentation**: Interfaces document the contract explicitly
4. **Future flexibility**: Alternative implementations possible

### Implementation vs Interface

The interfaces defined here are **design contracts**. The actual implementation:
- May combine multiple interfaces into one struct
- May use embedded structs for cache implementations
- May add additional methods beyond the interface

### Thread Safety

All interface methods must be safe for concurrent use. Implementations use:
- `sync.Map` for state storage
- Non-blocking channel operations with context
- No shared mutable state without synchronization
