# Implementation Plan: Central Node State Controller - Legacy Mode Removal

**Branch**: `001-central-node-state-controller` | **Date**: 2026-01-17 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-central-node-state-controller/spec.md`

**Note**: This plan addresses the **followup refactoring** work to remove the legacy mode and user-facing flag from the already-delivered Central Node State Controller feature.

## Summary

**Primary Requirement**: Simplify the Central Node State Controller architecture by removing the dual-mode support (provider + legacy) and the user-facing `--enable-node-state-controller` flag. Treat the central controller as an internal dependency that is automatically enabled when at least one dependent controller is active.

**Technical Approach**:
1. Remove the `--enable-node-state-controller` flag from `cmd/main.go`
2. Implement automatic enablement logic: central controller starts if `--enable-node-controller=true`
3. Remove legacy mode code from NodeProgressInsight controller (delete `impl *nodes.Reconciler` field and `NewNodeProgressInsightReconciler` constructor)
4. Require central controller dependency: NodeProgressInsight controller fails fast if central controller is not available
5. Remove all dual-mode tests and retain only provider-mode tests

**Impact**: Breaking change - deployments using `--enable-node-state-controller=false` will need to update configuration. Clean cutover with no gradual migration.

## Technical Context

**Language/Version**: Go 1.24.4 (godebug default=go1.23)
**Primary Dependencies**:
- controller-runtime v0.20.4 (Kubernetes operator framework)
- Kubebuilder v4 (scaffolding and patterns)
- OpenShift API types (Node, MachineConfigPool, MachineConfig)
**Storage**: In-memory only (sync.Map for central controller state cache)
**Testing**:
- Go test (unit tests)
- envtest (integration tests with real Kubernetes API server)
- Ginkgo/Gomega (BDD test framework)
**Target Platform**: Linux server (Kubernetes/OpenShift operator)
**Project Type**: Single (Kubernetes operator with CLI plugin)
**Performance Goals**:
- Single node state evaluation per change (vs. N evaluations for N consumers)
- No measurable performance regression from refactoring (automatic enablement overhead is negligible)
**Constraints**:
- Must maintain controller-runtime integration (lifecycle, health checks, metrics)
- Cannot break provider mode behavior (already tested and working)
- Breaking change to flag interface is acceptable
**Scale/Scope**:
- Affects 1 main file (`cmd/main.go`), 1 controller file (`internal/controller/nodeprogressinsight_controller.go`)
- Estimated 150-200 lines of code to remove
- Estimated 50-100 lines of new automatic enablement logic

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I: Kubebuilder-First Architecture ✅
- **Compliance**: Refactoring maintains Kubebuilder patterns
- **Impact**: No changes to controller scaffolding or RBAC markers
- **Verification**: Controller registration with manager remains unchanged

### Principle II: Controller-Runtime Integration ✅
- **Compliance**: Controllers continue to integrate with controller-runtime
- **Impact**: CentralNodeState controller lifecycle (Start, Stop) unchanged
- **Verification**: Provider mode already uses controller-runtime patterns; removing legacy mode does not affect this

### Principle III: Test Coverage (NON-NEGOTIABLE) ✅
- **Compliance**: Refactoring includes test updates
- **Impact**: Remove legacy mode tests, retain all provider mode tests
- **Verification**: `make test` must pass with no coverage decrease for provider mode paths
- **Action**: Update test suites to remove legacy mode test cases

### Principle IV: Separation of Concerns ✅
- **Compliance**: Layer separation is improved by removing dual-mode complexity
- **Impact**: NodeProgressInsight controller becomes simpler with single code path
- **Verification**: Controller layer remains thin wrapper; implementation logic stays in `nodes/` package

### Principle V: Upstream Resource Monitoring ✅
- **Compliance**: No changes to what resources are watched or how
- **Impact**: None - refactoring is purely about internal architecture
- **Verification**: Watch configuration in SetupWithManager remains unchanged

### Principle VI: Status-Only CRD Updates ✅
- **Compliance**: No changes to CRD update patterns
- **Impact**: Provider mode already follows status-only updates
- **Verification**: reconcileWithProvider logic unchanged

### Principle VII: CLI as CRD Consumer ✅
- **Compliance**: No CLI changes required
- **Impact**: CLI continues to read NodeProgressInsight CRDs
- **Verification**: oc-update-status plugin unaffected by this refactoring

### Summary
**All gates PASS**. This refactoring simplifies internal architecture without violating any constitution principles. It actually improves compliance by removing complexity (dual-mode support) and reducing user configuration surface.

## Project Structure

### Documentation (this feature)

```text
specs/001-central-node-state-controller/
├── spec.md              # Updated with followup requirements
├── plan.md              # This file
├── research.md          # Phase 0 output (automatic enablement patterns)
└── tasks.md             # Phase 2 output (generated by /speckit.tasks)
```

### Source Code (repository root)

**Existing structure** (Kubebuilder standard layout):

```text
cmd/
└── main.go              # MODIFIED: Remove flag, add auto-enable logic

internal/controller/
├── nodeprogressinsight_controller.go  # MODIFIED: Remove legacy mode
├── centralnodestate_controller.go     # UNCHANGED
├── nodes/
│   ├── impl.go          # UNCHANGED (used by central controller)
│   └── impl_test.go     # MODIFIED: Remove legacy mode test cases
└── nodestate/
    ├── impl.go          # UNCHANGED
    ├── state.go         # UNCHANGED
    └── *_test.go        # UNCHANGED

tests/
└── integration/         # MODIFIED: Remove legacy mode integration tests
```

**Key files to modify**:
1. `cmd/main.go`: Remove `--enable-node-state-controller` flag, add automatic enablement logic (lines 66, 103-104, 248-282)
2. `internal/controller/nodeprogressinsight_controller.go`: Remove legacy constructor, `impl` field, legacy mode reconciliation path (lines 44-58, 90-92)
3. `internal/controller/nodes/impl_test.go`: Remove legacy mode test cases

**Structure Decision**: Using existing Kubebuilder layout. No new directories needed. Refactoring removes code rather than adding structure.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

*No violations - table not needed.*

## Phase 0: Research & Unknowns

### Research Questions

1. **Automatic Controller Enablement Patterns**
   - **Question**: What is the best practice for automatic dependency enablement in controller-runtime applications?
   - **Context**: Need to enable CentralNodeState controller automatically when dependent controllers are enabled
   - **Research needed**: Review controller-runtime patterns, check if other operators do this

2. **Flag Removal Migration**
   - **Question**: How should we handle the removed `--enable-node-state-controller` flag?
   - **Context**: Flag will be unrecognized by flag parser
   - **Research needed**: Determine if Go flag package ignores unknown flags or errors, whether to add explicit deprecation warning

3. **Error Handling for Missing Dependency**
   - **Question**: What is the best fail-fast pattern when a required provider is missing?
   - **Context**: NodeProgressInsight should error if central controller is not available
   - **Research needed**: Review controller-runtime startup error patterns, check how other controllers handle missing dependencies

4. **Test Coverage Verification**
   - **Question**: How to ensure no accidental coverage loss when removing legacy mode tests?
   - **Context**: Need to verify provider mode has equivalent test coverage
   - **Research needed**: Review current test coverage metrics, identify any legacy-mode-only test cases that need provider-mode equivalents

### Expected Outputs

After Phase 0 research, `research.md` will document:
- Automatic enablement pattern (likely: simple conditional logic in main.go before controller setup)
- Flag handling decision (likely: flag parser ignores unknown flags, no special handling needed)
- Error handling pattern (likely: return error from SetupWithManager if provider is nil)
- Test coverage plan (audit of test cases to ensure provider mode has full coverage)

## Phase 1: Design Artifacts

### Data Model Changes

**No data model changes**. This refactoring does not modify:
- CRD schemas (NodeProgressInsight, CentralNodeState)
- Internal node state representation (already defined in `nodestate.NodeState`)
- API contracts between controllers

The refactoring is purely about controller lifecycle management and internal code organization.

### API Contracts

**No new API contracts**. Existing contracts remain:
- `nodestate.NodeStateProvider` interface (already implemented by CentralNodeStateReconciler)
- NodeProgressInsight CRD status updates (unchanged)

### Configuration Changes

**Changed configuration surface**:

**Before** (current):
```bash
--enable-cluster-version-controller=true
--enable-cluster-operator-controller=true
--enable-node-controller=true
--enable-node-state-controller=true  # User-facing, controllable
```

**After** (refactored):
```bash
--enable-cluster-version-controller=true
--enable-cluster-operator-controller=true
--enable-node-controller=true
# --enable-node-state-controller removed (automatic)
```

**Automatic enablement logic**:
```go
enableNodeState := controllers.enableNode
```

**Note**: No MCP Progress Controller logic included. That controller does not exist yet and will be handled in future work.

## Phase 2: Implementation Tasks

**Note**: Detailed task breakdown will be generated by `/speckit.tasks` command after this plan is complete.

### High-Level Task Groups

1. **Flag Removal** (Priority: P1)
   - Remove `enableNodeState` from `controllersConfig` struct
   - Remove flag declaration for `--enable-node-state-controller`
   - Remove flag parsing

2. **Automatic Enablement Logic** (Priority: P1)
   - Add conditional logic: `enableNodeState := controllers.enableNode`
   - Place logic before controller setup section
   - Add logging for automatic enablement decision

3. **Legacy Mode Removal** (Priority: P1)
   - Remove `impl` field from `NodeProgressInsightReconciler` struct
   - Remove `NewNodeProgressInsightReconciler` constructor (legacy mode)
   - Remove legacy mode conditional in `Reconcile()` method
   - Remove import of `nodes.Reconciler` (if no longer needed)

4. **Provider Requirement** (Priority: P1)
   - Update `NewNodeProgressInsightReconcilerWithProvider` to fail fast if provider is nil
   - Update main.go to always use provider constructor when NodeProgressInsight is enabled
   - Add error handling if central controller is not available

5. **Test Updates** (Priority: P1)
   - Audit current test coverage for legacy mode vs. provider mode
   - Remove legacy mode test cases
   - Verify provider mode tests cover all scenarios previously tested in legacy mode
   - Update integration tests to remove dual-mode scenarios

6. **Documentation Updates** (Priority: P2)
   - Update CLAUDE.md to remove `--enable-node-state-controller` from examples
   - Update README or deployment docs if they mention the flag
   - Add migration note for users currently using `--enable-node-state-controller=false`

### Dependencies Between Task Groups

```text
Flag Removal (1)
    ↓
Automatic Enablement Logic (2)
    ↓
Legacy Mode Removal (3) ← Must happen after flag logic updated
    ↓
Provider Requirement (4)
    ↓
Test Updates (5) ← Can run in parallel with (6)
    ↓
Documentation Updates (6)
```

## Phase 3: Testing Strategy

### Unit Tests

**Target**: `internal/controller/nodeprogressinsight_controller.go`
- Test that provider-mode constructor fails if provider is nil
- Test that Reconcile delegates to reconcileWithProvider (existing test, verify coverage)

### Integration Tests

**Target**: Controller lifecycle and automatic enablement
- Test: Start manager with `--enable-node-controller=true` → central controller starts automatically
- Test: Start manager with `--enable-node-controller=false` → central controller does NOT start
- Test: NodeProgressInsight controller fails to start if central controller is not running (provider unavailable)

### Regression Tests

**Ensure no behavior change in provider mode**:
- All existing provider-mode integration tests must pass unchanged
- Metrics validation (central controller metrics match pre-refactoring baseline)
- Node state evaluation count verification (single evaluation per node change)

### Manual Testing Checklist

1. Deploy refactored controller manager to test cluster
2. Verify NodeProgressInsight CRDs are created and updated
3. Check logs for automatic enablement message
4. Verify metrics are correctly exposed
5. Test with `--enable-node-controller=false` → no NodeProgressInsight CRDs created, no central controller running

## Success Criteria Validation

### From Spec (Followup Success Criteria)

- **SC-008**: Controller manager starts successfully with only `--enable-node-controller` flag
  - **Validation**: Integration test + manual test

- **SC-009**: Controller manager does NOT start central controller when NodeProgressInsight controller is disabled
  - **Validation**: Integration test with `--enable-node-controller=false`

- **SC-010**: Codebase contains ZERO references to legacy mode logic
  - **Validation**: `grep -r "legacy mode" internal/controller/` returns no results
  - **Validation**: `git diff` shows removal of `impl` field and `NewNodeProgressInsightReconciler`

- **SC-011**: All tests for NodeProgressInsight controller pass using only provider mode
  - **Validation**: `go test ./internal/controller/nodes -v` passes
  - **Validation**: `make test` passes

- **SC-012**: Attempting to use removed flag produces clear error or is ignored
  - **Validation**: Run manager with `--enable-node-state-controller=true` → flag ignored (Go flag package behavior)

## Risk Assessment

### Low Risk
- ✅ Provider mode is already implemented and tested
- ✅ Automatic enablement logic is simple conditional (low complexity)
- ✅ Breaking change is well-documented and acceptable

### Medium Risk
- ⚠️ Test coverage might have gaps if legacy mode tested scenarios not covered in provider mode
  - **Mitigation**: Audit test coverage in Phase 0, create provider-mode equivalents before removing legacy tests

### High Risk
- ❌ None identified

### Rollback Plan

If issues are discovered after deployment:
1. Revert commit removing legacy mode
2. Restore `--enable-node-state-controller` flag
3. Re-enable dual-mode support

**Note**: Clean cutover is preferred over gradual migration, so rollback is via code revert, not feature flag.

## Timeline Estimate

**Note**: Constitution discourages time estimates. This is task count only.

- Phase 0 (Research): ~4 research questions → `research.md`
- Phase 1 (Design): Minimal (no new designs, refactoring only)
- Phase 2 (Implementation): ~6 task groups (see High-Level Task Groups)
- Phase 3 (Testing): ~10 test scenarios (integration + manual)

## Open Questions - RESOLVED

All open questions have been resolved:

1. **MCP Progress Controller**: ✅ RESOLVED
   - **Decision**: Do NOT include any MCP Progress Controller logic. That controller does not exist yet.
   - **Impact**: Automatic enablement is simply `enableNodeState := controllers.enableNode`

2. **Flag Deprecation Warning**: ✅ RESOLVED
   - **Decision**: Clean removal, no backward compatibility needed (development phase)
   - **Impact**: Flag removed entirely from code, Go flag parser will error if users provide it

3. **Provider Nil Check Location**: ✅ RESOLVED
   - **Decision**: Both constructor AND main.go
   - **Impact**: Main.go has explicit check with clear error; constructor has defensive panic

## Next Steps

1. **Immediate**: Run `/speckit.tasks` to generate detailed task breakdown from this plan
2. **Phase 0**: Execute research tasks and document findings in `research.md`
3. **Phase 1**: Generate design artifacts (minimal for this refactoring)
4. **Phase 2**: Execute implementation tasks in dependency order
5. **Phase 3**: Validate all success criteria and run test suite

---

**Plan Status**: ✅ Complete (ready for task generation)
**Constitution Check**: ✅ All gates passed
**Research Required**: Yes (4 questions in Phase 0)
**Breaking Changes**: Yes (flag removal, acceptable per spec)
