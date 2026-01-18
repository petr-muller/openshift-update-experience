# Tasks: Central Node State Controller - Legacy Mode Removal

**Input**: Design documents from `/specs/001-central-node-state-controller/`
**Prerequisites**: plan.md, spec.md, research.md

**Tests**: Test tasks included per testing strategy in plan.md

**Organization**: This is a refactoring task focused on User Stories 4 and 5 (followup work). Tasks are organized to enable safe, incremental refactoring with validation at each step.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US4, US5)
- Include exact file paths in descriptions

## Path Conventions

This project uses Kubebuilder standard layout:
- **Controllers**: `cmd/main.go` (manager setup), `internal/controller/`
- **Tests**: `internal/controller/*_test.go`
- **Documentation**: `CLAUDE.md`, feature specs in `specs/`

---

## Phase 1: Pre-Refactoring Validation

**Purpose**: Establish baseline and ensure current implementation is stable before making changes

- [x] T001 Run test suite and capture baseline coverage: `go test -cover ./internal/controller/... > baseline-coverage.txt`
- [x] T002 Document current test coverage for provider mode in internal/controller/nodeprogressinsight_controller_test.go
- [x] T003 Document current test coverage for legacy mode in internal/controller/nodes/impl_test.go
- [x] T004 Verify all existing tests pass: `make test`
- [x] T005 Create git branch checkpoint before refactoring: `git commit -m "Pre-refactoring checkpoint"`

**Checkpoint**: Baseline established - refactoring can begin with confidence

---

## Phase 2: User Story 4 - Automatic Dependency Enablement (Priority: P2)

**Goal**: Remove the `--enable-node-state-controller` flag and implement automatic enablement logic so users only manage feature flags, not internal dependencies.

**Independent Test**: Start controller manager with `--enable-node-controller=true` and verify central controller starts automatically; start with `--enable-node-controller=false` and verify central controller does NOT start.

### Implementation for User Story 4

- [x] T006 [US4] Remove `enableNodeState` field from `controllersConfig` struct in cmd/main.go (line 66)
- [x] T007 [US4] Remove `--enable-node-state-controller` flag declaration in cmd/main.go (lines 103-104)
- [x] T008 [US4] Add automatic enablement logic before controller setup in cmd/main.go: `enableNodeState := controllers.enableNode`
- [x] T009 [US4] Add logging for automatic enablement decision in cmd/main.go: `setupLog.Info("CentralNodeState controller auto-enabled", "reason", "NodeProgressInsight controller is enabled")`
- [x] T010 [US4] Update central controller setup conditional to use new `enableNodeState` variable in cmd/main.go (line 251)

### Tests for User Story 4

- [x] T011 [P] [US4] Add integration test: manager starts with `--enable-node-controller=true` → central controller auto-enabled
- [x] T012 [P] [US4] Add integration test: manager starts with `--enable-node-controller=false` → central controller NOT started
- [x] T013 [US4] Run `make test` to verify automatic enablement tests pass

**Checkpoint**: Flag removed, automatic enablement working. Central controller now starts based on NodeProgressInsight enablement.

---

## Phase 3: User Story 5 - Legacy Mode Removal (Priority: P2)

**Goal**: Simplify the NodeProgressInsight controller by removing dual-mode support, leaving only provider mode (reads from central controller).

**Independent Test**: After refactoring, NodeProgressInsight controller only contains provider mode logic; `grep -r "legacy mode" internal/controller/` returns no results.

### Step 1: Add Provider Requirement Validation

- [x] T014 [US5] Add nil provider check in cmd/main.go after automatic enablement, before NodeProgressInsight setup (around line 262)
- [x] T015 [US5] Add error message if centralNodeState is nil when controllers.enableNode is true in cmd/main.go
- [x] T016 [US5] Add nil provider panic in NewNodeProgressInsightReconcilerWithProvider constructor in internal/controller/nodeprogressinsight_controller.go

### Step 2: Update Main.go Controller Setup

- [x] T017 [US5] Replace dual-mode constructor logic with provider-only constructor in cmd/main.go (lines 264-274)
- [x] T018 [US5] Remove conditional check `if centralNodeState != nil` from cmd/main.go
- [x] T019 [US5] Update NodeProgressInsight controller setup to ALWAYS use NewNodeProgressInsightReconcilerWithProvider in cmd/main.go

### Step 3: Remove Legacy Mode Code from Controller

- [x] T020 [US5] Remove `impl *nodes.Reconciler` field from NodeProgressInsightReconciler struct in internal/controller/nodeprogressinsight_controller.go (line 45)
- [x] T021 [US5] Remove `NewNodeProgressInsightReconciler` constructor (legacy mode) from internal/controller/nodeprogressinsight_controller.go (lines 51-59)
- [x] T022 [US5] Remove legacy mode conditional `if r.stateProvider == nil` from Reconcile method in internal/controller/nodeprogressinsight_controller.go (lines 90-92)
- [x] T023 [US5] Remove unused import `"github.com/petr-muller/openshift-update-experience/internal/controller/nodes"` if no longer needed in internal/controller/nodeprogressinsight_controller.go

### Step 4: Remove Legacy Mode Tests

- [x] T024 [US5] Identify all test cases that explicitly test `NewNodeProgressInsightReconciler` (legacy constructor) in internal/controller/nodeprogressinsight_controller_test.go
- [x] T025 [US5] Remove legacy mode test cases from internal/controller/nodeprogressinsight_controller_test.go
- [x] T026 [US5] Identify any test cases in internal/controller/nodes/impl_test.go that test standalone reconciliation (legacy mode)
- [x] T027 [US5] Remove or update legacy-specific test cases from internal/controller/nodes/impl_test.go
- [x] T028 [US5] Verify provider mode tests cover all scenarios previously tested in legacy mode

### Step 5: Validation

- [x] T029 [US5] Run `make test` to ensure all provider mode tests pass after legacy removal
- [x] T030 [US5] Verify no coverage decrease for provider mode: `go test -cover ./internal/controller/... > post-refactoring-coverage.txt`
- [x] T031 [US5] Compare coverage reports: `diff baseline-coverage.txt post-refactoring-coverage.txt`
- [x] T032 [US5] Run `grep -r "legacy" internal/controller/` to verify zero references to legacy mode
- [x] T033 [US5] Run `grep -r "impl \*nodes.Reconciler" internal/` to verify field removed
- [x] T034 [US5] Run `grep -r "NewNodeProgressInsightReconciler(" internal/` to verify legacy constructor removed (should only find WithProvider variant)

**Checkpoint**: Legacy mode completely removed. Only provider mode remains. All tests passing.

---

## Phase 4: Integration Testing & Validation

**Purpose**: Verify the refactored system works correctly end-to-end

### Integration Tests

- [ ] T035 Manual test: Deploy controller manager to test cluster with default flags
- [ ] T036 Manual test: Verify NodeProgressInsight CRDs are created and updated correctly
- [ ] T037 Manual test: Check logs for automatic enablement message: "CentralNodeState controller auto-enabled"
- [ ] T038 Manual test: Verify Prometheus metrics are correctly exposed at /metrics endpoint
- [ ] T039 Manual test: Start manager with `--enable-node-controller=false` → verify no NodeProgressInsight CRDs created
- [ ] T040 Manual test: Start manager with `--enable-node-controller=false` → verify central controller does NOT start

### Regression Tests

- [x] T041 Verify existing provider-mode integration tests pass unchanged: `go test ./internal/controller/nodestate/... -v`
- [x] T042 Verify central controller metrics match pre-refactoring baseline
- [x] T043 Verify node state evaluation happens exactly once per node change (check metrics)
- [x] T044 Run full test suite: `make test` (includes manifests, generate, fmt, vet, envtest)

**Checkpoint**: Full integration validated. Refactoring complete and working.

---

## Phase 5: Documentation & Migration

**Purpose**: Update documentation to reflect the simplified architecture

- [x] T045 [P] Update CLAUDE.md to remove `--enable-node-state-controller` from Controller Flags section
- [x] T046 [P] Update CLAUDE.md Recent Changes section to document legacy mode removal
- [x] T047 [P] Add migration note to CLAUDE.md or create MIGRATION.md documenting the breaking change
- [x] T048 [P] Update any deployment examples or README files that reference the removed flag
- [x] T049 [P] Search codebase for any remaining documentation references: `grep -r "enable-node-state-controller" . --exclude-dir=.git`

**Checkpoint**: Documentation updated. Users have clear migration guidance.

---

## Phase 6: Final Validation & Cleanup

**Purpose**: Final checks before considering the refactoring complete

- [x] T050 Run `make lint` to ensure code quality standards
- [x] T051 Run `make build` to verify controller manager builds successfully
- [x] T052 Run `make build-plugin` to verify oc-update-status plugin builds successfully
- [x] T053 Validate all success criteria from spec.md (SC-008 through SC-012)
- [x] T054 Create git commit with clear message documenting the refactoring
- [x] T055 Review diff to ensure no unintended changes: `git diff --stat`

**Checkpoint**: Refactoring complete, validated, and ready for review/merge.

---

## Dependencies & Execution Order

### User Story Dependencies

```text
Phase 1: Pre-Refactoring Validation
    ↓
Phase 2: User Story 4 (Automatic Dependency Enablement)
    ↓
Phase 3: User Story 5 (Legacy Mode Removal)
    ↓ (depends on US4 being complete)
Phase 4: Integration Testing
    ↓
Phase 5: Documentation (can run in parallel with Phase 6)
    ↓
Phase 6: Final Validation
```

### Critical Path

1. **T001-T005**: Baseline validation MUST complete first
2. **T006-T013**: Flag removal and automatic enablement MUST complete before legacy mode removal
3. **T014-T034**: Legacy mode removal tasks must execute in order (validation → main.go → controller → tests)
4. **T035-T044**: Integration tests verify everything works
5. **T045-T055**: Documentation and cleanup can happen in parallel

### Parallel Execution Opportunities

**Within Phase 2 (User Story 4)**:
- T011 and T012 can run in parallel (different test files)

**Within Phase 3 (User Story 5)**:
- T024-T028 (test removal) can be identified in parallel with T020-T023 (code removal)
- T029-T034 (validation tasks) can run in parallel after code/test removal

**Within Phase 5 (Documentation)**:
- T045, T046, T047, T048, T049 can all run in parallel (different files)

**Example Parallel Execution**:
```bash
# Phase 3, Step 4: Remove legacy tests (can run in parallel)
# Terminal 1
task T024  # Identify legacy constructor tests
task T025  # Remove legacy constructor tests

# Terminal 2 (parallel)
task T026  # Identify standalone reconciliation tests
task T027  # Remove/update standalone tests

# Terminal 3 (parallel)
task T028  # Verify provider coverage
```

---

## Success Criteria Validation

### SC-008: Controller manager starts successfully with only `--enable-node-controller` flag
- **Validated by**: T011, T036-T037

### SC-009: Controller manager does NOT start central controller when NodeProgressInsight disabled
- **Validated by**: T012, T039-T040

### SC-010: Codebase contains ZERO references to legacy mode logic
- **Validated by**: T032, T033, T034

### SC-011: All tests for NodeProgressInsight pass using only provider mode
- **Validated by**: T029, T044

### SC-012: Removed flag produces error or is ignored
- **Validated by**: Go flag package behavior (automatic), documented in research.md

---

## Implementation Strategy

### MVP Scope (Minimum Viable Product)

The MVP for this refactoring includes:
- **User Story 4**: Automatic dependency enablement (T006-T013)
- **User Story 5**: Legacy mode removal (T014-T034)
- **Basic validation**: Integration tests (T035-T044)

This delivers the core value: simplified architecture with automatic dependency management.

### Incremental Delivery

1. **Increment 1** (US4): Flag removal + automatic enablement
   - Delivers: Simplified user experience (fewer flags)
   - Testable: Can verify automatic enablement works
   - Checkpoint: T013

2. **Increment 2** (US5): Legacy mode removal
   - Delivers: Simplified codebase (single code path)
   - Testable: Can verify provider mode works independently
   - Checkpoint: T034

3. **Increment 3**: Documentation + final validation
   - Delivers: Complete refactoring with migration guidance
   - Testable: Full integration and regression tests
   - Checkpoint: T055

### Risk Mitigation

- **T001-T005**: Baseline validation prevents regressions
- **T029-T034**: Validation tasks catch issues early
- **T041-T044**: Regression tests ensure no behavior change
- **Git checkpoints**: Enable rollback at any point

---

## Task Summary

**Total Tasks**: 55
**User Story 4 Tasks**: 8 (T006-T013)
**User Story 5 Tasks**: 21 (T014-T034)
**Testing Tasks**: 15 (T011-T013, T024-T034, T035-T044)
**Documentation Tasks**: 5 (T045-T049)
**Validation Tasks**: 11 (T001-T005, T050-T055)

**Parallel Opportunities**: 12 tasks can run in parallel across different files
**Critical Path Length**: 43 tasks (assuming maximum parallelization)

**Estimated Scope**: Refactoring affects 2 main files (cmd/main.go, internal/controller/nodeprogressinsight_controller.go), removes ~150-200 lines of code, adds ~50-100 lines of automatic enablement logic.

---

**Status**: ✅ Ready for implementation
**Next Step**: Begin with Phase 1 (T001) to establish baseline
**Recommended Approach**: Execute phases sequentially, with parallel execution within phases where possible
