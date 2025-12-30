<!--
============================================================================
SYNC IMPACT REPORT - Constitution Update
============================================================================
Version Change: Initial → 1.0.0
Rationale: First complete constitution for the project (MINOR bump from 0.x.x)

Modified Principles: None (initial creation)
Added Sections:
  - All core principles (I-VII)
  - Testing Requirements section
  - Development Workflow section
  - Governance section

Templates Status:
  ✅ plan-template.md - Reviewed, Constitution Check section aligns with principles
  ✅ spec-template.md - Reviewed, user story and requirements sections align
  ✅ tasks-template.md - Reviewed, test-first approach and parallel execution align
  ✅ agent-file-template.md - Reviewed, no constitution-specific updates needed
  ✅ checklist-template.md - Reviewed, no constitution-specific updates needed

Follow-up TODOs: None
============================================================================
-->

# OpenShift Update Experience Constitution

## Core Principles

### I. Kubebuilder-First Architecture
The project follows Kubebuilder patterns and conventions:
- Controllers MUST use the two-layer pattern: thin wrapper (setup/watches/RBAC) + implementation layer (reconciliation logic)
- All CRDs MUST be cluster-scoped under the `openshift.muller.dev` API group
- Code generation (manifests, DeepCopy) MUST be automated via `make manifests` and `make generate`
- Controller flags MUST follow controller-runtime patterns with enable/disable per controller

**Rationale**: Kubebuilder provides battle-tested patterns for Kubernetes operators. Consistency with these patterns ensures maintainability, reduces cognitive load, and leverages community best practices.

### II. Controller-Runtime Integration
Controllers MUST integrate with controller-runtime framework:
- Use envtest for integration tests with real Kubernetes API server
- Implement proper watches on both owned resources (ProgressInsight CRDs) and upstream OpenShift resources
- Follow reconciliation patterns: watch → reconcile → update status
- Support leader election, metrics, and health probes out of the box

**Rationale**: Controller-runtime is the standard framework for Kubernetes operators. Proper integration ensures production-readiness, observability, and operational excellence.

### III. Test Coverage (NON-NEGOTIABLE)
Testing is mandatory with specific coverage requirements:
- Controllers MUST have integration tests using envtest (Ginkgo/Gomega)
- Implementation logic MUST have unit tests in `*_test.go` files
- CLI plugin MUST support `--mocks` flag for testing with fixture data
- Test files MUST follow naming: `impl_test.go` for implementation, `*_controller_test.go` for controller integration

**Rationale**: Kubernetes operators have high reliability requirements. Operating on cluster state requires confidence that controllers behave correctly under all conditions. Test-first development catches bugs before they affect production clusters.

### IV. Separation of Concerns
Clear separation MUST be maintained between layers:
- **API layer** (`api/v1alpha1`): CRD definitions only
- **Controller layer** (`internal/controller/*_controller.go`): Setup, watches, RBAC markers
- **Implementation layer** (`internal/controller/{package}/impl.go`): Business logic
- **CLI layer** (`cmd/oc-update-status`): Read-only consumers of CRDs, formatting only
- **Internal packages** (`internal/*`): Shared utilities, health assessment, constants

**Rationale**: Layered architecture improves testability, enables independent evolution of components, and makes the codebase approachable for new contributors.

### V. Upstream Resource Monitoring
Controllers watch OpenShift and Kubernetes resources but NEVER modify them:
- MUST watch: `ClusterVersion`, `ClusterOperator`, `Node`, `MachineConfigPool`, `MachineConfig`
- MAY read upstream resources in reconciliation
- MUST NOT modify upstream resources (read-only relationship)
- MUST update only owned ProgressInsight CRDs

**Rationale**: The operator is an observer and aggregator, not an actor. Modifying cluster resources would violate separation of responsibilities and create unpredictable feedback loops.

### VI. Status-Only CRD Updates
ProgressInsight CRDs follow Kubernetes status subresource pattern:
- Controllers MUST update only `.status` fields of ProgressInsight CRDs
- Status MUST include: assessment, conditions, timing info, completion percentage
- Conditions MUST follow Kubernetes patterns: type, status, reason, message, lastTransitionTime
- Spec fields are immutable after creation (controllers reconcile from upstream resources)

**Rationale**: Kubernetes best practices separate desired state (spec) from observed state (status). Controllers are responsible for observing reality and reporting it in status subresources.

### VII. CLI as CRD Consumer
The oc-update-status plugin is a presentation layer only:
- MUST read ProgressInsight CRDs via Kubernetes client
- MUST NOT implement business logic (logic belongs in controllers)
- MUST support both human-readable and machine-readable output
- MUST support `--details` flag for varying verbosity levels
- MUST support `--mocks` for testing without cluster access

**Rationale**: Keeping CLI as a thin consumer ensures single source of truth (CRDs written by controllers). Business logic in controllers can be tested independently and evolves without CLI changes.

## Testing Requirements

### Controller Tests
- Integration tests MUST use envtest (real API server, no mocks)
- Tests MUST verify: resource creation, status updates, conditions, timing calculations
- Tests MUST cover: normal reconciliation, error handling, resource not found scenarios
- Tests MUST use Ginkgo/Gomega for consistency with controller-runtime ecosystem

### CLI Tests
- CLI MUST support fixture-based testing via `--mocks` flag
- Mock data MUST be loadable from `cmd/oc-update-status/` directory
- Tests MUST verify: output formatting, detail level filtering, error handling
- Manual testing guidance MUST be in CLAUDE.md with example commands

### Code Quality Gates
- `make test` MUST pass (includes: manifests, generate, fmt, vet, envtest setup)
- `make lint` MUST pass using golangci-lint
- All controller RBAC markers MUST generate correct manifests
- Generated code (CRDs, DeepCopy) MUST stay in sync via `make manifests generate`

## Development Workflow

### Feature Development
1. Design: Define CRD changes in `api/v1alpha1` if needed
2. Implementation: Write controller logic in `internal/controller/{package}/impl.go`
3. Integration: Wire controller in `internal/controller/*_controller.go`
4. Testing: Add integration tests before marking feature complete
5. CLI: Update `cmd/oc-update-status` formatting if CRD schema changed
6. Validation: Run `make test lint build build-plugin` before commit

### Testing Workflow
- Run specific package tests: `go test ./internal/controller/{package} -v`
- Run specific test: `go test -run TestName ./path/to/package`
- Run all tests: `make test` (includes code generation verification)
- CLI testing: `make build-plugin && bin/oc-update-status --mocks`

### Local Development
- Run all controllers: `make run`
- Run single controller: `make run-only-cv` / `run-only-co` / `run-only-node`
- Test CLI against real cluster: `make build-plugin && bin/oc-update-status`
- Deploy to cluster: `make install deploy` (requires cluster access)

## Governance

### Constitution Authority
- This constitution defines NON-NEGOTIABLE principles for the codebase
- All pull requests MUST comply with architecture principles (I, II, IV, V, VI, VII)
- All pull requests MUST include tests (Principle III is NON-NEGOTIABLE)
- Code reviews MUST verify: layer separation, test coverage, Kubebuilder patterns

### Amendment Process
- Amendments require: documentation of rationale, impact analysis, version bump
- Constitution violations MUST be justified with complexity tracking in plan.md
- Breaking constitution principles requires explicit approval and migration plan

### Compliance Review
- Pre-commit: Developer verifies test coverage, layer separation, linting
- Code review: Reviewer checks constitution compliance, especially Principle III
- CI/CD: Automated checks for tests, manifests sync, code generation, linting
- Post-merge: Monitor for technical debt, refactoring opportunities, pattern drift

### Runtime Guidance
- CLAUDE.md provides AI assistant context (build commands, architecture, testing)
- CLAUDE.md is the single source of truth for development workflow and conventions
- Constitution provides principles; CLAUDE.md provides practical implementation guidance

**Version**: 1.0.0 | **Ratified**: 2025-06-23 | **Last Amended**: 2025-12-30
