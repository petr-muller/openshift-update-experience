# Data Model: Centralized Node State Controller

**Date**: 2026-01-16
**Feature Branch**: `001-central-node-state-controller`

## Overview

This document defines the internal data structures for the centralized node state controller. These structures are **internal only** and do not affect the existing CRD API surface.

---

## Entities

### 1. NodeState

**Purpose**: Internal representation of evaluated node update state, maintained by the central controller.

**Package**: `internal/controller/nodestate`

```go
// NodeState represents the evaluated state of a single node's update progress.
// This is the internal representation maintained by the central controller,
// loosely coupled from the NodeProgressInsightStatus CRD structure.
type NodeState struct {
    // Identity
    Name             string                    // Node name (immutable key)
    UID              types.UID                 // Node UID for validation

    // Pool Association
    PoolRef          ouev1alpha1.ResourceRef   // MachineConfigPool reference
    Scope            ouev1alpha1.ScopeType     // ControlPlane or WorkerPool

    // Version Information
    Version          string                    // OCP version (from MachineConfig annotation)
    DesiredVersion   string                    // Target version from ClusterVersion
    CurrentConfig    string                    // Current MachineConfig name
    DesiredConfig    string                    // Desired MachineConfig name

    // Status Conditions
    Conditions       []metav1.Condition        // Updating, Available, Degraded

    // Update Progress
    Phase            UpdatePhase               // Current update phase
    Message          string                    // Human-readable status message

    // Internal Tracking (not exposed in CRD)
    LastEvaluated    time.Time                 // When state was last computed
    EvaluationCount  int64                     // Number of evaluations for this node
    SourceGeneration int64                     // Node resourceVersion that triggered evaluation
    StateHash        uint64                    // Hash of state for change detection
}

// UpdatePhase represents the current phase of a node's update process
type UpdatePhase string

const (
    UpdatePhasePending    UpdatePhase = "Pending"    // Waiting to start
    UpdatePhaseDraining   UpdatePhase = "Draining"   // Workloads being evicted
    UpdatePhaseUpdating   UpdatePhase = "Updating"   // Configuration being applied
    UpdatePhaseRebooting  UpdatePhase = "Rebooting"  // Node reboot in progress
    UpdatePhasePaused     UpdatePhase = "Paused"     // Update paused by pool config
    UpdatePhaseCompleted  UpdatePhase = "Completed"  // Update finished
)
```

**Relationships**:
- One NodeState per Node resource in the cluster
- References one MachineConfigPool via PoolRef
- Conditions mirror NodeProgressInsight conditions

**Validation Rules**:
- Name must be non-empty
- PoolRef.Name must match an existing MachineConfigPool
- Scope must be one of: ControlPlane, WorkerPool
- Version should match semver pattern when present

**State Transitions**:
```
Pending → Draining → Updating → Rebooting → Completed
              ↓                      ↓
           Paused ←─────────────────┘
```

---

### 2. NodeStateStore

**Purpose**: Thread-safe storage for all NodeState instances.

**Package**: `internal/controller/nodestate`

```go
// NodeStateStore provides thread-safe storage and retrieval of NodeState instances.
type NodeStateStore struct {
    states sync.Map  // map[string]*NodeState (key: node name)
}

// Methods:
// - Get(nodeName string) (*NodeState, bool)
// - Set(nodeName string, state *NodeState)
// - Delete(nodeName string) bool
// - Range(fn func(name string, state *NodeState) bool)
// - Count() int
// - GetAll() []*NodeState
// - GetByPool(poolName string) []*NodeState
```

**Characteristics**:
- Uses sync.Map for efficient concurrent access
- Key: node name (string)
- Value: pointer to NodeState
- Typical size: 10-1000+ entries (cluster node count)

---

### 3. DownstreamController

**Purpose**: Represents a registered downstream controller that receives state change notifications.

**Package**: `internal/controller/nodestate`

```go
// DownstreamController represents a controller that consumes node state updates.
type DownstreamController struct {
    // Identity
    Name          string                        // Controller name for logging/metrics
    Type          DownstreamControllerType      // Classification for metrics

    // Communication
    EventChannel  chan<- event.GenericEvent     // Channel to send notifications
    BufferSize    int                           // Channel buffer size

    // Subscription
    Filter        func(*NodeState) bool         // Optional: filter which states to receive
}

// DownstreamControllerType classifies downstream controllers for metrics
type DownstreamControllerType string

const (
    DownstreamTypeNodeInsight DownstreamControllerType = "node_insight"
    DownstreamTypeMCPInsight  DownstreamControllerType = "mcp_insight"
)
```

**Relationships**:
- Multiple DownstreamControllers can exist (currently 2: NodeProgressInsight, MCPProgressInsight)
- Each has its own EventChannel
- Central controller sends to all registered downstream controllers

---

### 4. StateChangeEvent

**Purpose**: Internal event representing a node state change for notification routing.

**Package**: `internal/controller/nodestate`

```go
// StateChangeEvent represents a change in node state that needs to be propagated
// to downstream controllers.
type StateChangeEvent struct {
    // What changed
    NodeName      string              // Affected node
    OldState      *NodeState          // Previous state (nil if new)
    NewState      *NodeState          // Current state (nil if deleted)
    ChangeType    StateChangeType     // Type of change

    // Context
    Timestamp     time.Time           // When change was detected
    TriggerSource string              // What triggered evaluation (Node, MCP, MC, CV)
}

// StateChangeType classifies the type of state change
type StateChangeType string

const (
    StateChangeCreated  StateChangeType = "Created"
    StateChangeUpdated  StateChangeType = "Updated"
    StateChangeDeleted  StateChangeType = "Deleted"
)
```

---

### 5. MachineConfigPoolSelectorCache (Moved)

**Purpose**: Cache of MachineConfigPool label selectors for efficient node-to-pool mapping.

**Package**: `internal/controller/nodestate` (moved from `internal/controller/nodes`)

```go
// MachineConfigPoolSelectorCache caches label selectors from MachineConfigPools
// to efficiently determine which pool a node belongs to.
type MachineConfigPoolSelectorCache struct {
    cache sync.Map  // map[string]labels.Selector (key: MCP name)
}

// Methods:
// - WhichMCP(nodeLabels labels.Labels) string
// - Ingest(mcpName string, selector *metav1.LabelSelector) (modified bool, reason string)
// - Forget(mcpName string) bool
// - GetAll() map[string]labels.Selector
```

**Unchanged from current implementation** - moved for centralized ownership.

---

### 6. MachineConfigVersionCache (Moved)

**Purpose**: Cache of MachineConfig to OCP version mappings.

**Package**: `internal/controller/nodestate` (moved from `internal/controller/nodes`)

```go
// MachineConfigVersionCache caches the OCP version extracted from MachineConfig
// annotations for efficient version lookup during node state evaluation.
type MachineConfigVersionCache struct {
    cache sync.Map  // map[string]string (key: MC name, value: version)
}

// Methods:
// - VersionFor(machineConfigName string) (version string, found bool)
// - Ingest(mc *machineconfigurationv1.MachineConfig) (modified bool)
// - Forget(mcName string) bool
```

**Unchanged from current implementation** - moved for centralized ownership.

---

## Entity Relationships Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    CentralNodeStateController                    │
│                                                                  │
│  ┌──────────────────┐    ┌──────────────────────────────────┐  │
│  │ NodeStateStore   │    │ MachineConfigPoolSelectorCache   │  │
│  │                  │    │                                   │  │
│  │  sync.Map:       │    │  sync.Map:                       │  │
│  │  nodeName →      │    │  mcpName → labels.Selector       │  │
│  │  *NodeState      │    └──────────────────────────────────┘  │
│  └──────────────────┘                                           │
│                          ┌──────────────────────────────────┐  │
│                          │ MachineConfigVersionCache        │  │
│                          │                                   │  │
│                          │  sync.Map:                       │  │
│                          │  mcName → version string         │  │
│                          └──────────────────────────────────┘  │
│                                                                  │
│  Downstream Notification Channels:                               │
│  ┌──────────────────┐    ┌──────────────────┐                  │
│  │ nodeInsightChan  │    │ mcpInsightChan   │                  │
│  │ GenericEvent     │    │ GenericEvent     │                  │
│  └────────┬─────────┘    └────────┬─────────┘                  │
│           │                        │                             │
└───────────┼────────────────────────┼─────────────────────────────┘
            │                        │
            ▼                        ▼
┌───────────────────────┐  ┌───────────────────────┐
│ NodeProgressInsight   │  │ MCPProgressInsight    │
│ Controller            │  │ Controller            │
│                       │  │                       │
│ Reads NodeState →     │  │ Reads NodeStates →    │
│ Updates CRD Status    │  │ Aggregates summaries  │
└───────────────────────┘  └───────────────────────┘
```

---

## Data Flow

### State Evaluation Flow

1. **Trigger**: Node/MCP/MachineConfig/ClusterVersion change detected
2. **Evaluation**: Central controller evaluates affected node(s)
3. **Storage**: Updated NodeState stored in NodeStateStore
4. **Notification**: GenericEvent sent to appropriate channel(s)
5. **Reconciliation**: Downstream controller reconciles, reads state from store
6. **CRD Update**: Downstream controller updates ProgressInsight CRD status

### Notification Routing

| Trigger | Nodes Affected | NodeInsight Notified | MCPInsight Notified |
|---------|----------------|----------------------|---------------------|
| Single Node change | 1 | Yes (that node) | Yes (if pool member) |
| MCP selector change | All in pool | Yes (all affected) | Yes |
| MachineConfig version change | All with that MC | Yes (all affected) | Yes |
| ClusterVersion change | All | Yes (all) | Yes |
| Node deletion | 1 | Yes (deletion) | Yes (for summary recalc) |

---

## Cardinality and Scale

| Entity | Expected Count | Notes |
|--------|----------------|-------|
| NodeState | 10-1000+ | One per cluster node |
| NodeStateStore | 1 | Singleton per central controller |
| DownstreamController | 2-5 | Currently: NodeInsight, MCPInsight |
| MCP Selectors cached | 2-10 | Typically: master, worker, plus custom pools |
| MC Versions cached | 10-100 | One per MachineConfig in use |

---

## Change Detection

NodeState includes a `StateHash` field for efficient change detection:

```go
func (s *NodeState) ComputeHash() uint64 {
    h := fnv.New64a()
    h.Write([]byte(s.Name))
    h.Write([]byte(s.PoolRef.Name))
    h.Write([]byte(s.Version))
    h.Write([]byte(s.Phase))
    for _, c := range s.Conditions {
        h.Write([]byte(c.Type))
        h.Write([]byte(c.Status))
        h.Write([]byte(c.Reason))
    }
    return h.Sum64()
}
```

This enables:
- Skip notification if state unchanged
- Efficient diff detection for logging/metrics
- Avoid CRD updates when status identical
