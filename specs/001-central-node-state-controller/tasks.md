# Tasks: Centralized Node State Controller

**Input**: Design documents from `/specs/001-central-node-state-controller/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Required per constitution principle III (Test Coverage - NON-NEGOTIABLE)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

This is a Kubernetes operator with existing structure:
- `internal/controller/` - Controller implementations
- `internal/controller/nodestate/` - NEW package for central controller
- `cmd/main.go` - Manager setup and controller registration

---

## Phase 1: Setup (Package Structure)

**Purpose**: Create new package structure and move shared code

- [x] T001 Create package directory internal/controller/nodestate/
- [x] T002 [P] Create package documentation in internal/controller/nodestate/doc.go
- [x] T003 [P] Move mcpselectorcache.go from internal/controller/nodes/ to internal/controller/nodestate/caches.go
- [x] T004 [P] Move mcversioncache.go content into internal/controller/nodestate/caches.go (merge with T003)
- [x] T005 Update imports in internal/controller/nodes/impl.go to use nodestate.MachineConfigPoolSelectorCache and nodestate.MachineConfigVersionCache
- [x] T006 Run `make test` to verify existing functionality preserved after cache move

**Checkpoint**: Package structure ready, existing tests pass

---

## Phase 2: Foundational (Core Data Structures)

**Purpose**: Implement core data structures that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational Phase

- [x] T007 [P] Unit tests for NodeState struct and UpdatePhase constants in internal/controller/nodestate/state_test.go
- [x] T008 [P] Unit tests for NodeStateStore (Get/Set/Delete/Range/Count/GetByPool) in internal/controller/nodestate/state_test.go

### Implementation for Foundational Phase

- [x] T009 [P] Implement NodeState struct with all fields per data-model.md in internal/controller/nodestate/state.go
- [x] T010 [P] Implement UpdatePhase type and constants (Pending, Draining, Updating, Rebooting, Paused, Completed) in internal/controller/nodestate/state.go
- [x] T011 Implement NodeStateStore with sync.Map backing in internal/controller/nodestate/state.go (depends on T009)
- [x] T012 Implement ComputeHash() method on NodeState for change detection in internal/controller/nodestate/state.go
- [x] T013 Run tests to verify foundational data structures work correctly

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 & 2 - Consistent State & Reduced Processing (Priority: P1) MVP

**Goal**: Central controller evaluates node state once and provides consistent view to all downstream controllers

**Independent Test**: Verify that node state evaluation happens exactly once per change, and multiple downstream accessors see identical state

**Note**: US1 and US2 are combined because they share implementation - you cannot have consistent state (US1) without centralized evaluation (US2)

### Tests for User Stories 1 & 2

- [x] T014 [P] [US1] Unit tests for StateEvaluator interface implementation in internal/controller/nodestate/assess_test.go
- [x] T015 [P] [US1] Unit tests for condition determination logic in internal/controller/nodestate/conditions_test.go
- [x] T016 [P] [US2] Unit tests verifying single evaluation per state change in internal/controller/nodestate/impl_test.go
- [x] T017 [US1] Integration test: multiple accessors see identical state snapshots in internal/controller/nodestate/impl_test.go

### Implementation for User Stories 1 & 2

- [x] T018 [P] [US1] Extract assessNode() logic from internal/controller/nodes/impl.go to internal/controller/nodestate/assess.go
- [x] T019 [P] [US1] Extract determineConditions() logic from internal/controller/nodes/impl.go to internal/controller/nodestate/conditions.go
- [x] T020 [US1] Implement CentralNodeStateController struct with state store and caches in internal/controller/nodestate/impl.go
- [x] T021 [US1] Implement Reconcile() method for central controller that evaluates and stores node state in internal/controller/nodestate/impl.go
- [x] T022 [US2] Add evaluation counting to verify single-evaluation behavior in internal/controller/nodestate/impl.go
- [x] T023 [US1] Implement GetNodeState() and GetAllNodeStates() methods (NodeStateProvider interface) in internal/controller/nodestate/impl.go
- [x] T024 [US1] Implement GetNodeStatesByPool() method for pool-based queries in internal/controller/nodestate/impl.go
- [x] T025 [US1] Create thin wrapper nodestate_controller.go with SetupWithManager and watch configuration in internal/controller/centralnodestate_controller.go
- [x] T026 [US1] Add RBAC markers for Nodes, MachineConfigPools, MachineConfigs watches in internal/controller/centralnodestate_controller.go
- [x] T027 Run `make manifests` to generate RBAC from markers
- [x] T028 Run tests to verify US1/US2 acceptance scenarios pass

**Checkpoint**: Central controller evaluates state once, provides consistent snapshots - core MVP complete

---

## Phase 4: User Story 3 - Downstream Controller Registration (Priority: P2)

**Goal**: Downstream controllers receive notifications via source.Channel when state changes

**Independent Test**: Register mock downstream handler, trigger state change, verify notification received

### Tests for User Story 3

- [x] T029 [P] [US3] Unit tests for notification channel creation and management in internal/controller/nodestate/impl_test.go
- [x] T030 [P] [US3] Unit tests for non-blocking notification send with context in internal/controller/nodestate/impl_test.go
- [x] T031 [US3] Integration test: downstream controller receives notification on state change in internal/controller/nodestate/impl_test.go

### Implementation for User Story 3

- [x] T032 [US3] Add nodeInsightEvents and mcpInsightEvents channels to CentralNodeStateController in internal/controller/nodestate/impl.go
- [x] T033 [US3] Implement NodeInsightChannel() and MCPInsightChannel() accessor methods in internal/controller/nodestate/impl.go
- [x] T034 [US3] Implement notifyNodeInsightControllers() with non-blocking send in internal/controller/nodestate/impl.go
- [x] T035 [US3] Implement notifyMCPInsightControllers() with non-blocking send in internal/controller/nodestate/impl.go
- [x] T036 [US3] Implement notifyAllDownstream() for ClusterVersion changes in internal/controller/nodestate/impl.go
- [x] T037 [US3] Integrate notification calls into Reconcile() flow after state updates in internal/controller/nodestate/impl.go
- [x] T038 [US3] Refactor NodeProgressInsightReconciler to accept NodeStateProvider dependency in internal/controller/nodeprogressinsight_controller.go
- [x] T039 [US3] Update NodeProgressInsightReconciler.SetupWithManager() to watch source.Channel in internal/controller/nodeprogressinsight_controller.go
- [x] T040 [US3] Simplify NodeProgressInsightReconciler.Reconcile() to read state from provider and update CRD in internal/controller/nodeprogressinsight_controller.go
- [x] T041 [US3] Update cmd/main.go to create central controller first and pass to downstream controllers
- [x] T042 [US3] Add --enable-node-state-controller flag to cmd/main.go
- [x] T043 Run tests to verify US3 acceptance scenarios pass

**Checkpoint**: Downstream controllers receive notifications, architecture is decoupled

---

## Phase 5: User Story 4 - Graceful Degradation (Priority: P3)

**Goal**: Central controller functions correctly with zero registered downstream controllers

**Independent Test**: Run central controller alone, verify it processes state without errors and late-registering controllers get current state

### Tests for User Story 4

- [x] T044 [P] [US4] Unit test: central controller processes state with no downstream channels in internal/controller/nodestate/impl_test.go
- [x] T045 [P] [US4] Unit test: late-registering controller receives current state snapshot in internal/controller/nodestate/impl_test.go
- [x] T046 [US4] Integration test: full lifecycle with delayed downstream registration in internal/controller/nodestate/impl_test.go

### Implementation for User Story 4

- [x] T047 [US4] Add nil-channel checks to notification methods in internal/controller/nodestate/impl.go
- [x] T048 [US4] Implement GetCurrentSnapshot() method for late registrations in internal/controller/nodestate/impl.go
- [x] T049 [US4] Add logging for downstream registration timing in internal/controller/nodestate/impl.go
- [x] T050 Run tests to verify US4 acceptance scenarios pass

**Checkpoint**: System robust with 0 to N downstream controllers

---

## Phase 6: Observability (Cross-Cutting)

**Purpose**: Full observability per spec requirements (FR-011, FR-012, FR-013)

### Tests for Observability

- [x] T051 [P] Unit tests for metrics registration and recording in internal/controller/nodestate/metrics_test.go
- [x] T052 [P] Unit tests for structured logging with correlation IDs in internal/controller/nodestate/impl_test.go

### Implementation for Observability

- [x] T053 [P] Implement Prometheus metrics registration in internal/controller/nodestate/metrics.go
- [x] T054 [P] Implement nodeStateEvaluationDuration histogram metric in internal/controller/nodestate/metrics.go
- [x] T055 [P] Implement nodeStateEvaluationTotal counter metric in internal/controller/nodestate/metrics.go
- [x] T056 [P] Implement downstreamNotificationDuration histogram metric in internal/controller/nodestate/metrics.go
- [x] T057 [P] Implement activeNodeStates gauge metric in internal/controller/nodestate/metrics.go
- [x] T058 Add metric recording calls to Reconcile() and notification methods in internal/controller/nodestate/impl.go
- [x] T059 Add structured logging with correlation IDs (node, resourceVersion, operation) in internal/controller/nodestate/impl.go
- [x] T060 Run tests to verify observability requirements met

**Checkpoint**: Full tracing and metrics available

---

## Phase 7: Lifecycle & Polish

**Purpose**: Proper controller lifecycle integration and cleanup

### Tests for Lifecycle

- [x] T061 [P] Unit test: Start() blocks until context cancelled in internal/controller/nodestate/impl_test.go
- [x] T062 [P] Unit test: channels closed on shutdown in internal/controller/nodestate/impl_test.go

### Implementation for Lifecycle

- [x] T063 Implement manager.Runnable interface (Start method) in internal/controller/nodestate/impl.go
- [x] T064 Implement graceful channel closure in Start() on context cancellation in internal/controller/nodestate/impl.go
- [x] T065 Add health check integration (NeedLeaderElection) in internal/controller/nodestate/impl.go
- [x] T066 Update centralnodestate_controller.go SetupWithManager to register as Runnable
- [x] T067 Verify cache separation (legacy mode retains caches for backwards compatibility)
- [x] T068 Run full test suite: `make test` - All tests passing, 83.4% coverage
- [x] T069 Run linter: `make lint` - 0 issues
- [x] T070 Update CLAUDE.md with new controller documentation and flags

**Checkpoint**: Implementation complete, all tests pass, documentation updated

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories 1 & 2 (Phase 3)**: Depends on Foundational - core MVP
- **User Story 3 (Phase 4)**: Depends on Phase 3 - notification mechanism
- **User Story 4 (Phase 5)**: Depends on Phase 4 - graceful degradation
- **Observability (Phase 6)**: Can start after Phase 3, parallel with Phases 4-5
- **Lifecycle & Polish (Phase 7)**: Depends on all previous phases

### User Story Dependencies

- **User Stories 1 & 2 (P1)**: Combined, form MVP - no dependencies on other stories
- **User Story 3 (P2)**: Depends on US1/US2 for state to notify about
- **User Story 4 (P3)**: Depends on US3 for notification channels to test graceful degradation

### Parallel Opportunities

**Phase 1**: T002, T003, T004 can run in parallel
**Phase 2**: T007, T008 (tests) and T009, T010 can run in parallel
**Phase 3**: T014, T015, T016 (tests) and T018, T019 can run in parallel
**Phase 4**: T029, T030 (tests) can run in parallel
**Phase 5**: T044, T045 (tests) can run in parallel
**Phase 6**: T051, T052 (tests) and T053-T057 (all metrics) can run in parallel
**Phase 7**: T061, T062 (tests) can run in parallel

---

## Parallel Example: Phase 3 (MVP)

```bash
# Launch all tests for US1/US2 together:
Task: "Unit tests for StateEvaluator interface implementation in internal/controller/nodestate/assess_test.go"
Task: "Unit tests for condition determination logic in internal/controller/nodestate/conditions_test.go"
Task: "Unit tests verifying single evaluation per state change in internal/controller/nodestate/impl_test.go"

# Launch extraction tasks together:
Task: "Extract assessNode() logic from internal/controller/nodes/impl.go to internal/controller/nodestate/assess.go"
Task: "Extract determineConditions() logic from internal/controller/nodes/impl.go to internal/controller/nodestate/conditions.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 & 2 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Stories 1 & 2
4. **STOP and VALIDATE**: Central controller evaluates state once, provides consistent view
5. Deploy/demo if ready - this is the core value

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1/US2 → Test independently → Deploy/Demo (MVP!)
3. Add US3 → Test independently → Deploy/Demo (decoupled architecture)
4. Add US4 → Test independently → Deploy/Demo (robust system)
5. Add Observability → Full production readiness
6. Each phase adds value without breaking previous functionality

### Single Developer Strategy

Execute phases sequentially:
1. Phase 1 (Setup) - ~30 min
2. Phase 2 (Foundational) - ~1-2 hours
3. Phase 3 (US1/US2 MVP) - ~3-4 hours
4. Phase 4 (US3 Notifications) - ~2-3 hours
5. Phase 5 (US4 Graceful Degradation) - ~1 hour
6. Phase 6 (Observability) - ~1-2 hours
7. Phase 7 (Polish) - ~1 hour

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Constitution requires tests (NON-NEGOTIABLE) - write tests first
- Commit after each task or logical group
- Run `make test lint` after each phase
- Stop at any checkpoint to validate story independently
