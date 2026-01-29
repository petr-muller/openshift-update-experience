# Quickstart: Centralized Node State Controller

**Date**: 2026-01-16
**Feature Branch**: `001-central-node-state-controller`

## Overview

This document provides a quick reference for implementing the centralized node state controller feature.

---

## Prerequisites

- Go 1.24+
- Access to an OpenShift cluster (or use envtest for testing)
- Existing codebase cloned and dependencies installed

```bash
# Verify setup
make test  # Should pass
make build  # Should succeed
```

---

## Implementation Order

### Phase 1: Core Infrastructure

1. **Create nodestate package structure**
   ```
   internal/controller/nodestate/
   ├── doc.go              # Package documentation
   ├── state.go            # NodeState and Store
   ├── state_test.go
   ├── caches.go           # Moved MCP selector and MC version caches
   ├── caches_test.go
   └── metrics.go          # Prometheus metrics registration
   ```

2. **Implement NodeState and Store**
   - Define structs from data-model.md
   - Implement Get/Set/Delete/Range methods
   - Add hash computation for change detection

3. **Move caches from nodes/ to nodestate/**
   - Copy `mcpselectorcache.go` → `nodestate/caches.go`
   - Copy `mcversioncache.go` → merge into `nodestate/caches.go`
   - Update imports in nodes/impl.go to use new location

### Phase 2: Central Controller

1. **Implement central controller**
   ```
   internal/controller/nodestate/
   ├── impl.go             # Central controller implementation
   ├── impl_test.go
   ├── assess.go           # Node state assessment (extracted from nodes/)
   ├── assess_test.go
   ├── conditions.go       # Condition determination
   └── conditions_test.go
   ```

2. **Create thin wrapper**
   ```
   internal/controller/nodestate_controller.go  # SetupWithManager, watches
   ```

3. **Implement notification channels**
   - Create channels for downstream controllers
   - Implement non-blocking send with context

### Phase 3: Integration

1. **Update cmd/main.go**
   - Add `--enable-node-state-controller` flag
   - Create central controller before downstream controllers
   - Pass channel references to downstream controllers

2. **Refactor NodeProgressInsight controller**
   - Remove internal caches (use central)
   - Add source.Channel watch
   - Read state from central store
   - Simplify to just CRD status updates

3. **Prepare for MCPProgressInsight controller**
   - Add channel for future MCP insight controller
   - Document integration pattern

### Phase 4: Testing & Observability

1. **Add unit tests**
   - NodeState and store tests
   - Assessment function tests
   - Notification flow tests

2. **Add integration tests**
   - Controller lifecycle tests with envtest
   - Multi-controller coordination tests

3. **Implement metrics**
   - Register Prometheus metrics
   - Add metric recording to key operations

---

## Key Code Snippets

### Creating the Central Controller

```go
// internal/controller/nodestate/impl.go
package nodestate

type CentralNodeStateController struct {
    client.Client
    Scheme *runtime.Scheme

    // State management
    store          *Store
    mcpSelectors   *MachineConfigPoolSelectorCache
    mcVersions     *MachineConfigVersionCache

    // Downstream notification
    nodeInsightChan chan event.GenericEvent
    mcpInsightChan  chan event.GenericEvent

    // Lifecycle
    once    sync.Once
    started bool
    mu      sync.Mutex
}

func NewCentralNodeStateController(c client.Client, s *runtime.Scheme) *CentralNodeStateController {
    return &CentralNodeStateController{
        Client:          c,
        Scheme:          s,
        store:           &Store{},
        mcpSelectors:    &MachineConfigPoolSelectorCache{},
        mcVersions:      &MachineConfigVersionCache{},
        nodeInsightChan: make(chan event.GenericEvent, 1024),
        mcpInsightChan:  make(chan event.GenericEvent, 1024),
    }
}

func (c *CentralNodeStateController) NodeInsightChannel() <-chan event.GenericEvent {
    return c.nodeInsightChan
}

func (c *CentralNodeStateController) GetNodeState(name string) (*NodeState, bool) {
    return c.store.Get(name)
}
```

### Setting Up Watches

```go
// internal/controller/nodestate_controller.go
func (r *CentralNodeStateController) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        Named("central-node-state").
        // Watch Nodes
        Watches(
            &corev1.Node{},
            handler.EnqueueRequestsFromMapFunc(r.handleNodeEvent),
        ).
        // Watch MachineConfigPools
        Watches(
            &machineconfigurationv1.MachineConfigPool{},
            handler.EnqueueRequestsFromMapFunc(r.handleMCPEvent),
            builder.WithPredicates(mcpSelectorPredicate),
        ).
        // Watch MachineConfigs
        Watches(
            &machineconfigurationv1.MachineConfig{},
            handler.EnqueueRequestsFromMapFunc(r.handleMCEvent),
            builder.WithPredicates(mcVersionPredicate),
        ).
        Complete(r)
}
```

### Downstream Controller Integration

```go
// internal/controller/nodeprogressinsight_controller.go
func (r *NodeProgressInsightReconciler) SetupWithManager(
    mgr ctrl.Manager,
    central *nodestate.CentralNodeStateController,
) error {
    r.central = central

    return ctrl.NewControllerManagedBy(mgr).
        For(&ouev1alpha1.NodeProgressInsight{}).
        // Watch channel from central controller
        Watches(
            source.Channel(
                central.NodeInsightChannel(),
                &handler.EnqueueRequestForObject{},
            ),
        ).
        Complete(r)
}

func (r *NodeProgressInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Get state from central controller
    state, found := r.central.GetNodeState(req.Name)
    if !found {
        // Node deleted - delete insight
        return r.deleteInsight(ctx, req.Name)
    }

    // Copy state to CRD status
    return r.updateInsightStatus(ctx, req.Name, state)
}
```

---

## Testing Commands

```bash
# Run all tests
make test

# Run nodestate package tests only
go test ./internal/controller/nodestate/... -v

# Run specific test
go test -run TestStore ./internal/controller/nodestate -v

# Run with race detector
go test -race ./internal/controller/nodestate/...

# Run integration tests
go test ./internal/controller/... -v -tags=integration

# Run linter
make lint
```

---

## Validation Checklist

Before marking implementation complete:

- [ ] All existing tests pass (`make test`)
- [ ] New unit tests for nodestate package
- [ ] Integration tests for controller lifecycle
- [ ] Metrics exposed at `/metrics` endpoint
- [ ] Structured logging with correlation IDs
- [ ] No regression in NodeProgressInsight behavior
- [ ] Documentation updated in CLAUDE.md
- [ ] Lint passes (`make lint`)

---

## Common Issues

### Channel Deadlock
**Symptom**: Reconciliation hangs
**Solution**: Always use non-blocking send with context:
```go
select {
case c.nodeInsightChan <- event.GenericEvent{Object: node}:
case <-ctx.Done():
    return
}
```

### Race Condition in Tests
**Symptom**: Flaky tests
**Solution**: Use sync.Map methods properly; don't assume ordering

### Missing RBAC
**Symptom**: "forbidden" errors at runtime
**Solution**: Ensure RBAC markers in wrapper controller file, run `make manifests`

---

## Related Documents

- [spec.md](./spec.md) - Feature specification
- [research.md](./research.md) - Technical research and decisions
- [data-model.md](./data-model.md) - Data structure definitions
- [contracts/](./contracts) - Go interface definitions
