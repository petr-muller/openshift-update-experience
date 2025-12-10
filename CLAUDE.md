# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubernetes operator and CLI tool for tracking OpenShift cluster update progress. The project provides:
- **Controller Manager**: Kubernetes controllers that watch cluster resources and generate insight CRDs
- **oc-update-status plugin**: CLI tool that reads insight CRDs and displays human-readable update status

The operator monitors ClusterVersion, ClusterOperator, and Node resources during OpenShift updates and creates corresponding "Progress Insight" CRDs that aggregate status information.

## Build Commands

### Building
- `make build` - Build the controller manager binary (`bin/manager`)
- `make build-plugin` - Build the oc plugin binary (`bin/oc-update-status`)
- `make docker-build` - Build container image for the controller manager

### Testing
- `make test` - Run all tests (includes manifests, generate, fmt, vet, setup-envtest)
- `go test ./internal/controller/nodes -v` - Run tests for a specific package
- `go test -run TestFunctionName ./path/to/package` - Run a single test

### Code Generation
- `make manifests` - Generate CRDs, RBAC, and webhook configurations
- `make generate` - Generate DeepCopy methods and other boilerplate

### Linting
- `make lint` - Run golangci-lint
- `make lint-fix` - Run golangci-lint with auto-fixes
- `golangci-lint run` - Run linter directly (available via hooks)

### Development
- `make run` - Run controller manager locally (all controllers enabled)
- `make run-only-cv` - Run only ClusterVersion controller
- `make run-only-co` - Run only ClusterOperator controller
- `make run-only-node` - Run only Node controller
- `make run-only-mcp` - Run only MachineConfigPool controller

### Deployment
- `make install` - Install CRDs to cluster
- `make deploy` - Deploy controller manager to cluster
- `make uninstall` - Remove CRDs from cluster
- `make undeploy` - Remove controller manager from cluster

## Architecture

### API Resources (api/v1alpha1)

The operator defines five Custom Resource Definitions (CRDs):

1. **ClusterVersionProgressInsight** - Tracks control plane update progress
   - Monitors `clusterversions.config.openshift.io` resource
   - Status includes: versions (previous/target), assessment, completion percentage, timing info
   - Conditions: Updating, Healthy

2. **ClusterOperatorProgressInsight** - Tracks individual control plane component updates
   - One insight per ClusterOperator (e.g., kube-apiserver, machine-config)
   - Status includes: name, conditions (Updating, Healthy)
   - Created for each `clusteroperators.config.openshift.io` resource

3. **NodeProgressInsight** - Tracks individual node updates
   - One insight per Node
   - Status includes: name, pool reference, version, scope (control plane vs worker)
   - Conditions: Updating, Degraded, Available
   - Updating reasons: Draining, Updating, Rebooting, Paused, Pending, Completed

4. **MachineConfigPoolProgressInsight** - Tracks worker pool update progress
   - One insight per MachineConfigPool (e.g., worker, custom pools)
   - Status includes: pool name, scope type, assessment, completion percentage, node summaries
   - Summaries track: Total, Available, Progressing, Outdated, Draining, Excluded, Degraded nodes
   - Conditions: Updating, Healthy
   - Assessment types: Pending, Progressing, Completed, Degraded, Excluded

5. **UpdateHealthInsight** - Overall update health (not yet fully implemented)

### Controller Architecture

Controllers follow a two-layer pattern:
- **Thin wrapper** (`internal/controller/*_controller.go`) - Handles controller-runtime setup, watches, RBAC markers
- **Implementation** (`internal/controller/{clusterversions,clusteroperators,nodes}/impl.go`) - Contains reconciliation logic

Each controller:
- Watches upstream OpenShift resources (ClusterVersion, ClusterOperator, Node, MachineConfigPool)
- Reconciles when changes occur
- Updates corresponding ProgressInsight CRD status

#### Central Node State Controller (NEW)

The **CentralNodeStateController** provides centralized node state evaluation:
- **Purpose**: Evaluates node update state once and provides consistent snapshots to downstream controllers
- **Location**: `internal/controller/nodestate/` package
- **Benefits**:
  - Single evaluation per node change (reduces redundant processing)
  - Consistent state across all downstream consumers
  - Notification channels for reactive updates via `source.Channel`
  - Graceful degradation (works with 0 to N downstream controllers)
- **Architecture**:
  - Evaluates node state in `Reconcile()` and stores in thread-safe `Store`
  - Downstream controllers read via `Provider` interface
  - Notifications sent via buffered channels (1024 buffer) on state changes
  - Implements `manager.Runnable` for lifecycle management
- **Observability**:
  - Prometheus metrics for evaluation duration, notification performance, node counts by phase
  - Structured logging with correlation IDs (node, resourceVersion, operation, pool)
- **Modes**:
  - **Provider mode** (default): NodeProgressInsight uses central controller's state
  - **Legacy mode**: NodeProgressInsight evaluates state independently (backwards compatibility)

### Key Internal Packages

- **internal/controller/nodestate/** - **Central node state controller (NEW)**
  - `impl.go` - Central controller with state evaluation and notification
  - `state.go` - NodeState struct, UpdatePhase constants, Store
  - `assess.go` - Node state evaluation logic (extracted from nodes controller)
  - `conditions.go` - Condition determination logic
  - `caches.go` - MCP selector and MC version caches
  - `metrics.go` - Prometheus metrics for observability
  - `*_test.go` - Comprehensive test coverage (83.4%)
- **internal/controller/{clusterversions,clusteroperators,nodes}** - Controller implementation logic
  - `impl.go` - Main reconciliation logic
  - `impl_test.go` - Unit tests
- **internal/health/** - Health assessment logic for insights
- **internal/mco/** - Constants for Machine Config Operator states (daemon, pool states)
- **internal/clusteroperators/** - ClusterOperator-specific utilities

### CLI Plugin (cmd/oc-update-status)

The plugin reads ProgressInsight CRDs and formats them for display:
- `main.go` - CLI setup, resource fetching, orchestration
- `controlplane.go` - Control plane status formatting
- `nodes.go` - Node status formatting
- `workerpools.go` - Worker pool status formatting with assessment, completion, and node status
- `mockresources.go` - Mock data loading for testing with generic `loadInsightListFromFile` helper
- `*_test.go` - Unit tests following table-driven test pattern with parallel execution
- Supports `--details={none,all,nodes,health,operators}` flag
- Supports `--mocks` flag for testing with fixture data

### Entry Points

- **Controller Manager**: `cmd/main.go` - Sets up manager with all three controllers
- **CLI Plugin**: `cmd/oc-update-status/main.go` - Cobra-based CLI

## Development Notes

### Testing Philosophy
- Controllers use envtest (real Kubernetes API server) for integration tests
- Tests use Ginkgo/Gomega framework
- CLI plugin uses table-driven tests with `t.Parallel()` for formatter functions
- Mock data fixtures available in `cmd/oc-update-status/examples/` for CLI testing
- TDD approach: write tests first, see them fail, then implement to make them pass
- Test data should be comprehensive (e.g., all 7 node summary types for MCP tests)

### Go Version
- Go 1.24.0+ required
- Uses go 1.24.4 toolchain
- godebug default=go1.23

### Kubebuilder
- Built with Kubebuilder v4
- Domain: `muller.dev`
- API group: `openshift.muller.dev`
- All CRDs are cluster-scoped

### Controller Flags
The controller manager supports:
- `--enable-cluster-version-controller` (default: true) - ClusterVersion progress tracking
- `--enable-cluster-operator-controller` (default: true) - ClusterOperator progress tracking
- `--enable-node-controller` (default: true) - NodeProgressInsight controller
  - When enabled: Central node state controller is automatically enabled
  - NodeProgressInsight reads pre-computed state from the central controller
- `--enable-machine-config-pool-controller` (default: true)
- Standard controller-runtime flags (metrics, leader election, etc.)

### Metrics (Central Node State Controller)

The central controller exposes Prometheus metrics:
- `openshift_update_experience_central_node_state_evaluation_duration_seconds` - Node state evaluation duration (histogram)
- `openshift_update_experience_central_node_state_evaluation_total` - Total evaluations by pool and result (counter)
- `openshift_update_experience_central_node_state_notification_duration_seconds` - Downstream notification duration (histogram)
- `openshift_update_experience_central_node_state_active_nodes` - Number of tracked nodes (gauge)
- `openshift_update_experience_central_node_state_nodes_by_phase` - Nodes grouped by update phase (gauge)

## Active Technologies
- Go 1.24+ (go 1.24.4 toolchain, godebug default=go1.23) + controller-runtime v0.x, client-go, Kubebuilder v4, OpenShift API types (MachineConfigPool, MachineConfig, ClusterVersion) (001-central-node-state-controller)
- In-memory state only (sync.Map for thread-safe caching); CRDs persisted via Kubernetes API (001-central-node-state-controller)
- Go 1.24.4 (godebug default=go1.23) (001-central-node-state-controller)
- In-memory only (sync.Map for central controller state cache) (001-central-node-state-controller)

## Recent Changes
- **Code Refactoring & Cleanup** (2026-01-29): Post-implementation cleanups and standardizations
  - **Interface Renames**: `NodeStateProvider` → `Provider`, `NodeStateStore` → `Store`, `NodeStateToUID()` → `ToUID()`
  - **Function Exports**: `isNodeDegraded()` → `IsNodeDegraded()`, `isNodeDraining()` → `IsNodeDraining()` (made public for cross-package reuse)
  - **Code Deduplication**: Removed ~200 lines of duplicate condition/assessment logic from `nodes/impl.go`, eliminated duplicate test setup code via helpers
  - **Removed Unused Code**: `AssessNodeForInsight()` wrapper, `mcpNotificationObject` type (replaced with actual MachineConfigPool objects)
  - **Import Organization**: Standardized to Go conventions (stdlib, third-party, local with blank line separators)
  - **Code Quality**: Spelling fixes, empty slice initialization improvements, handled ignored return values, variable shadowing fixes
  - **API Pattern**: Changed `openshiftconfigv1.AddToScheme()` → `openshiftconfigv1.Install()` (preferred OpenShift pattern)

- **Legacy Mode Removal & Automatic Dependency Enablement** (2025-01): Simplified central node state controller architecture
  - **BREAKING CHANGE**: Removed `--enable-node-state-controller` flag (previously defaulted to true)
  - Central controller now automatically enables when `--enable-node-controller=true`
  - Removed dual-mode support from NodeProgressInsight controller (provider mode only)
  - NodeProgressInsight always reads from central controller - no standalone mode
  - **Migration**: Remove `--enable-node-state-controller` from deployment configurations if present
  - All tests pass, provider mode maintains 83.7% coverage

- **Central Node State Controller** (2025-01): Implemented centralized node state evaluation controller
  - New package: `internal/controller/nodestate/` with 13 files, 83.4% test coverage
  - Single evaluation per node change, consistent state snapshots for all downstream controllers
  - Notification channels for reactive updates, graceful degradation (0 to N consumers)
  - Full observability: Prometheus metrics + structured logging with correlation IDs
  - Implements manager.Runnable for proper lifecycle management
