# Implementation Plan: Centralized Node State Controller

**Branch**: `001-central-node-state-controller` | **Date**: 2026-01-16 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-central-node-state-controller/spec.md`

## Summary

Create a centralized node state controller that evaluates and maintains internal node update states, then notifies downstream controllers (NodeProgressInsight and future MCPProgressInsight) via source.Channels. This eliminates duplicate state evaluation, ensures consistent node state views across insight controllers, and enables efficient batch processing. The central controller watches Nodes, MachineConfigPools, and MachineConfigs, maintaining internal state separate from the API surface, while downstream controllers register for granular notifications and copy state to their respective CRDs.

## Technical Context

**Language/Version**: Go 1.24+ (go 1.24.4 toolchain, godebug default=go1.23)
**Primary Dependencies**: controller-runtime v0.x, client-go, Kubebuilder v4, OpenShift API types (MachineConfigPool, MachineConfig, ClusterVersion)
**Storage**: In-memory state only (sync.Map for thread-safe caching); CRDs persisted via Kubernetes API
**Testing**: Ginkgo/Gomega for BDD tests, envtest for integration tests with real API server, fake client for unit tests
**Target Platform**: Kubernetes/OpenShift cluster (runs as in-cluster operator)
**Project Type**: Kubernetes Operator (single process, multiple controllers)
**Performance Goals**: Single evaluation per node state change; downstream notification within same reconciliation cycle
**Constraints**: Moderate controller-runtime deviation allowed; controllers must remain manager-registered; full observability (tracing, metrics)
**Scale/Scope**: Clusters with 10-1000+ nodes, 2-5 downstream controllers, rapid state changes during updates

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Kubebuilder-First Architecture | PASS | Two-layer pattern (wrapper + impl) will be followed. No new CRDs required. |
| II. Controller-Runtime Integration | PASS with justified deviation | Central controller uses internal channels and shared state (approved deviation). Controllers remain registered with manager and follow standard lifecycle. |
| III. Test Coverage (NON-NEGOTIABLE) | PASS | Integration tests with envtest required; unit tests for impl.go; mock support for testing notification flow. |
| IV. Separation of Concerns | PASS | New `internal/controller/nodestate/` package for central controller; downstream controllers in existing packages. |
| V. Upstream Resource Monitoring | PASS | Central controller watches Nodes, MCPs, MachineConfigs read-only; only writes to internal state. |
| VI. Status-Only CRD Updates | PASS | Downstream controllers update only status fields of existing ProgressInsight CRDs. |
| VII. CLI as CRD Consumer | N/A | No CLI changes required for this internal refactoring. |

**Deviation Justification**: The spec explicitly allows moderate deviation from controller-runtime best practices. Using internal source.Channels for inter-controller communication and shared in-memory state are approved deviations that enable the efficiency goals while keeping all controllers registered with the manager.

## Project Structure

### Documentation (this feature)

```text
specs/001-central-node-state-controller/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (internal Go interfaces, not API contracts)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/controller/
├── nodestate/                          # NEW: Central node state controller package
│   ├── impl.go                         # Central controller implementation
│   ├── impl_test.go                    # Unit tests
│   ├── state.go                        # Internal node state data structures
│   ├── state_test.go                   # State data structure tests
│   ├── registry.go                     # Downstream controller registration
│   ├── registry_test.go                # Registry tests
│   ├── metrics.go                      # Prometheus metrics
│   └── tracing.go                      # OpenTelemetry tracing
├── nodestate_controller.go             # NEW: Thin wrapper for central controller
├── nodes/
│   ├── impl.go                         # MODIFY: Refactor to consume from central state
│   ├── impl_test.go                    # MODIFY: Update tests for new flow
│   ├── mcpselectorcache.go             # MOVE: To nodestate/ package
│   └── mcversioncache.go               # MOVE: To nodestate/ package
├── nodeprogressinsight_controller.go   # MODIFY: Register with central controller
└── ... (other existing controllers unchanged)

cmd/
└── main.go                             # MODIFY: Add central controller registration, new flag
```

**Structure Decision**: Follow existing Kubebuilder pattern with thin wrapper + implementation package. Create new `internal/controller/nodestate/` package for the central controller. Move shared caches from `nodes/` to `nodestate/` since they're now centrally managed. Existing controller packages remain but refactored to consume from central state.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Internal source.Channels (not standard watches) | Enables efficient notification without API round-trips; downstream controllers don't need to re-fetch state they already have access to | Standard watches would require downstream controllers to independently fetch and evaluate the same node state, defeating the purpose of centralization |
| Shared in-memory state between controllers | Eliminates duplicate state evaluation; ensures consistent snapshots across downstream controllers | Passing state via CRD status would add API latency and potential race conditions between separate reconciliation cycles |

## Constitution Check (Post-Design Re-evaluation)

*Re-evaluation after Phase 1 design completion.*

| Principle | Status | Post-Design Notes |
|-----------|--------|-------------------|
| I. Kubebuilder-First Architecture | PASS | Design confirms two-layer pattern: `nodestate_controller.go` (wrapper) + `nodestate/impl.go` (implementation). Package structure follows existing patterns. |
| II. Controller-Runtime Integration | PASS with justified deviation | Design uses `source.Channel` (official controller-runtime API) for notifications. Central controller implements `manager.Runnable`. All controllers registered with manager. Deviation is within controller-runtime's supported patterns. |
| III. Test Coverage (NON-NEGOTIABLE) | PASS | Design specifies: unit tests for state.go, assess.go, conditions.go; integration tests with envtest for controller lifecycle; fake client for downstream controller tests. |
| IV. Separation of Concerns | PASS | Clear separation: `nodestate/` owns evaluation and state; `nodes/` owns CRD updates; caches moved to appropriate owner; interfaces define contracts between layers. |
| V. Upstream Resource Monitoring | PASS | Central controller watches Nodes, MCPs, MachineConfigs read-only. No modifications to upstream resources. Writes only to internal sync.Map state. |
| VI. Status-Only CRD Updates | PASS | Downstream controllers remain responsible for CRD status updates. Central controller never touches CRDs. |
| VII. CLI as CRD Consumer | N/A | No CLI changes. CRD schema unchanged. CLI continues to work without modification. |

**Post-Design Verdict**: All constitution principles satisfied. Justified deviations use official controller-runtime APIs (source.Channel, manager.Runnable) within supported patterns. Ready for task generation.

## Generated Artifacts

| Artifact | Path | Status |
|----------|------|--------|
| Implementation Plan | `plan.md` | Complete |
| Technical Research | `research.md` | Complete |
| Data Model | `data-model.md` | Complete |
| Quickstart Guide | `quickstart.md` | Complete |
| Go Interface Contracts | `contracts/interfaces.go.md` | Complete |
| Task List | `tasks.md` | Pending (`/speckit.tasks`) |

## Next Steps

1. Run `/speckit.tasks` to generate the implementation task list
2. Execute tasks in order defined by quickstart.md
3. Run `make test lint` after each major change
4. Update CLAUDE.md with new controller documentation when complete
