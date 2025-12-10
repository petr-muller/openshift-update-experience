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
   - One insight per MachineConfigPool (e.g., master, worker, custom pools)
   - Monitors `machineconfigpools.machineconfiguration.openshift.io` resources
   - Status currently includes: pool name, scope type (ControlPlane for master, WorkerPool for others)
   - Future: Will include assessment, completion percentage, node summaries, and conditions
   - Automatically deleted when corresponding MachineConfigPool is deleted

5. **UpdateHealthInsight** - Overall update health (not yet fully implemented)

### Controller Architecture

Controllers follow a two-layer pattern:
- **Thin wrapper** (`internal/controller/*_controller.go`) - Handles controller-runtime setup, watches, RBAC markers
- **Implementation** (`internal/controller/{clusterversions,clusteroperators,nodes,machineconfigpools}/impl.go`) - Contains reconciliation logic

Each controller:
- Watches upstream OpenShift resources (ClusterVersion, ClusterOperator, Node, MachineConfigPool)
- Reconciles when changes occur
- Creates, updates, or deletes corresponding ProgressInsight CRD
- Uses status subresource for updates to avoid conflicts

### Key Internal Packages

- **internal/controller/{clusterversions,clusteroperators,nodes,machineconfigpools}** - Controller implementation logic
  - `impl.go` - Main reconciliation logic
  - `impl_test.go` - Unit tests using fake client and table-driven tests
- **internal/controller/nodes/** - Additional node controller components:
  - `mcpselectorcache.go` - Caches MachineConfigPool selectors for node->pool mapping
  - `mcversioncache.go` - Caches MachineConfig->version mappings
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

- **Controller Manager**: `cmd/main.go` - Sets up manager with all four controllers
- **CLI Plugin**: `cmd/oc-update-status/main.go` - Cobra-based CLI

## Development Notes

### Testing Philosophy
- Controllers use envtest (real Kubernetes API server) for integration tests with Ginkgo/Gomega
- Unit tests use fake clients with `fake.NewClientBuilder()` and `WithStatusSubresource()` for proper status handling
- Tests use table-driven patterns with descriptive test names
- CLI plugin uses table-driven tests with `t.Parallel()` for formatter functions
- Mock data fixtures available in `cmd/oc-update-status/examples/` for CLI testing
- TDD approach: write tests first, see them fail, then implement to make them pass
- Test data should be comprehensive (e.g., all 7 node summary types for MCP tests)
- Always test deletion scenarios when controllers manage resource lifecycle

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
- `--enable-cluster-version-controller` (default: true)
- `--enable-cluster-operator-controller` (default: true)
- `--enable-node-controller` (default: true)
- `--enable-machine-config-pool-controller` (default: true)
- Standard controller-runtime flags (metrics, leader election, etc.)
- When adding new functionality, always add appropritate test coverage