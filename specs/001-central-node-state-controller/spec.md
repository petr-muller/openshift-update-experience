# Feature Specification: Centralized Node State Controller

**Feature Branch**: `001-central-node-state-controller`
**Created**: 2026-01-16
**Status**: Implemented & Refactored (complete)
**Input**: User description: "I want to make the internal controller structure more efficient, possibly at the cost of not entirely respecting controller-runtime best practices. Both the MCP progress insights and Node progress insights actually need to have access to evaluated Node states. This could be implemented through a new central controller that would watch Nodes, and potentially MCPs and MachineConfigs, just like the current NodeProgressInsight does. This controller would serve as a backend, and would track basically NodeProgressInsights, but not in the k8s API surface, but internally (and in a data structure only loosely coupled with actual NodeProgressInsights). The controllers reconciling the actual Node- and MachineConfigPoolProgress insights would be downstream of this controller. They would register themselves with this controller internally, and their reconciliation would be triggered by the central controller after the central state is maintained."

## Implementation Status & Followup Work

### ✅ Current Implementation (Complete)

The Central Node State Controller has been fully implemented and refactored with the following architecture:
- **Automatic Dependency**: Central controller automatically enables when `--enable-node-controller=true` (no separate flag needed)
- **Provider Mode Only**: NodeProgressInsight always reads from centralized state (legacy mode removed)
- **Simplified Architecture**: Single code path, no dual-mode complexity

### ✅ Code Refactoring & Cleanup (Complete - 2026-01-29)

**Interface & Type Simplifications**:
- Renamed `NodeStateProvider` → `Provider` (shorter, clearer in package context)
- Renamed `NodeStateStore` → `Store` (shorter, clearer in package context)
- Renamed `NodeStateToUID()` → `ToUID()` (consistent with type rename)
- Exported `isNodeDegraded()` → `IsNodeDegraded()` (reusable across packages)
- Exported `isNodeDraining()` → `IsNodeDraining()` (reusable across packages)

**Code Deduplication**:
- Removed duplicate `determineConditions()` from `nodes/impl.go` (already exists in `nodestate.DetermineConditions()`)
- Removed duplicate `isNodeDegraded()` and `isNodeDraining()` from `nodes/impl.go` (now using exported versions from `nodestate`)
- Removed unused `AssessNodeForInsight()` wrapper function
- Removed `mcpNotificationObject` type (replaced with actual `MachineConfigPool` objects with name set)
- Created test helpers to eliminate 200+ lines of duplicate setup code:
  - `prepareCentralNodeStateController()` - common controller setup with node, pools, configs, ClusterVersion
  - `prepareController()` - simpler controller setup for basic tests
  - `preparePoolsWithMultipleNodes()` - multi-node test fixture setup
  - `checkUpdatingWithReason()` - assertion helper for update phase verification

**Code Quality Improvements**:
- Standardized import organization (stdlib, third-party, local with blank line separators)
- Fixed spelling: `cancelled` → `canceled` (American English)
- Improved empty slice initialization: `conditions := []metav1.Condition{}` → `var conditions []metav1.Condition`
- Handled ignored return values: `h.Write(bytes)` → `_, _ = h.Write(bytes)`
- Fixed variable shadowing: `ctrl` → `c` in `NewCentralNodeStateControllerWithOptions()`
- Removed unused imports from `nodes/impl.go` (strings, time, ptr, meta)
- Changed to preferred OpenShift API pattern: `openshiftconfigv1.AddToScheme()` → `openshiftconfigv1.Install()`

**Rationale**: These cleanups improve code maintainability, reduce duplication, and align with Go best practices. The refactoring maintains identical behavior while reducing code size and complexity.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consistent Node State Across Insights (Priority: P1)

As an operator developer, I need all insight controllers (Node and MachineConfigPool) to work with a consistent, pre-evaluated view of node states so that the insights they produce are coherent and reflect the same underlying cluster state at any given moment.

**Why this priority**: This is the core value proposition. Without consistent state across controllers, the system may produce conflicting or confusing insights where Node insights and MachineConfigPool insights disagree about the same nodes.

**Independent Test**: Can be fully tested by simulating cluster state changes and verifying that downstream controllers receive identical node state snapshots. Delivers coherent insight generation across controller boundaries.

**Acceptance Scenarios**:

1. **Given** a cluster with multiple nodes in various update states, **When** the central controller evaluates node states, **Then** both Node and MachineConfigPool insight controllers receive the same evaluated state for each node.

2. **Given** a node transitions from "Updating" to "Completed", **When** the central controller processes this change, **Then** all downstream controllers are notified with the updated state before they begin their reconciliation.

3. **Given** multiple nodes change state simultaneously, **When** the central controller processes these changes, **Then** downstream controllers receive a consistent batch of state updates rather than partial views.

---

### User Story 2 - Reduced Duplicate Processing (Priority: P1)

As an operator developer, I need to eliminate redundant evaluation of node states so that the system performs more efficiently, especially in large clusters with many nodes.

**Why this priority**: Performance is a key driver for this refactoring. Currently, multiple controllers may independently evaluate the same node states, wasting computational resources.

**Independent Test**: Can be tested by measuring reconciliation cycles and verifying that node state evaluation happens once per state change, regardless of how many downstream consumers exist.

**Acceptance Scenarios**:

1. **Given** a node state change occurs, **When** the central controller processes it, **Then** node state evaluation logic executes exactly once (not once per downstream controller).

2. **Given** 100 nodes in a cluster with 2 downstream insight controllers, **When** all nodes update simultaneously, **Then** the total node state evaluations equal 100 (not 200).

---

### User Story 3 - Downstream Controller Registration (Priority: P2)

As an operator developer, I need downstream controllers to register themselves with the central controller so that they can be notified when relevant state changes occur, enabling a clean separation of concerns.

**Why this priority**: Registration is the mechanism that enables the decoupled architecture. Without it, the central controller cannot notify downstream consumers.

**Independent Test**: Can be tested by registering mock downstream handlers and verifying they receive callbacks when state changes.

**Acceptance Scenarios**:

1. **Given** a downstream controller starts up, **When** it registers with the central controller, **Then** it begins receiving state change notifications.

2. **Given** a downstream controller is registered, **When** node state changes occur, **Then** the controller's reconciliation is triggered with the updated state.

3. **Given** multiple downstream controllers are registered, **When** state changes occur, **Then** all registered controllers are notified.

---

### User Story 4 - Automatic Dependency Enablement (Priority: P2) **[FOLLOWUP]**

As an operator deployer, I want the central node state controller to be automatically enabled when I enable dependent controllers so that I don't need to understand internal implementation details or manage multiple related flags.

**Why this priority**: This simplifies the user experience and reduces configuration errors. Users should only need to enable the features they want (NodeProgressInsight, MCP Progress Insight), not manage internal dependencies.

**Independent Test**: Can be tested by starting the controller manager with various flag combinations and verifying the central controller is only enabled when at least one dependent controller is enabled.

**Acceptance Scenarios**:

1. **Given** the controller manager starts with `--enable-node-controller=true`, **When** the manager initializes, **Then** the central node state controller is automatically started without requiring `--enable-node-state-controller` flag.

2. **Given** the controller manager starts with `--enable-node-controller=false` and `--enable-mcp-progress-controller=false`, **When** the manager initializes, **Then** the central node state controller is NOT started.

3. **Given** the controller manager starts with `--enable-mcp-progress-controller=true`, **When** the manager initializes, **Then** the central node state controller is automatically started to provide node state to the MCP controller.

4. **Given** legacy code supporting `--enable-node-state-controller` flag exists, **When** the followup refactoring is complete, **Then** the flag is removed and attempting to use it results in a clear deprecation error message.

---

### User Story 5 - Legacy Mode Removal (Priority: P2) **[FOLLOWUP]**

As an operator developer, I want to eliminate the legacy mode from NodeProgressInsight controller so that the codebase is simpler, easier to maintain, and has a single well-tested code path.

**Why this priority**: Maintaining dual code paths (provider mode and legacy mode) increases maintenance burden, testing complexity, and risk of bugs. With the central controller proven stable, the legacy mode is technical debt.

**Independent Test**: Can be tested by verifying that after refactoring, the NodeProgressInsight controller only contains provider mode logic and all legacy mode code is removed.

**Acceptance Scenarios**:

1. **Given** the followup refactoring is complete, **When** the NodeProgressInsight controller starts, **Then** it ONLY operates in provider mode (reads from central controller) with no legacy mode code path.

2. **Given** the central controller is disabled via the automatic dependency logic (all dependent controllers disabled), **When** NodeProgressInsight controller attempts to start, **Then** it fails fast with a clear error indicating it requires the central controller dependency.

3. **Given** legacy mode code existed in the original implementation, **When** the followup refactoring is complete, **Then** all legacy mode logic (independent state evaluation in NodeProgressInsight) is removed from the codebase.

4. **Given** the refactoring removes dual-mode support, **When** tests run, **Then** test coverage for NodeProgressInsight focuses solely on provider mode behavior with no legacy mode test cases.

---

### User Story 6 - Graceful Degradation Without Downstream Consumers (Priority: P3)

As an operator developer, I need the central controller to function correctly even when no downstream controllers are registered so that the system remains stable during startup or partial deployments.

**Why this priority**: Robustness is important but this is an edge case. The system should handle registration timing gracefully.

**Independent Test**: Can be tested by running the central controller without any registered downstream handlers and verifying it processes state without errors.

**Acceptance Scenarios**:

1. **Given** the central controller is running with no registered downstream controllers, **When** node state changes occur, **Then** the central controller processes and maintains internal state without errors.

2. **Given** a downstream controller registers after the central controller has completed startup reconciliation and processed subsequent state changes, **When** registration completes, **Then** the controller receives the current state snapshot from the fully populated cache.

---

### Edge Cases

- What happens when a downstream controller fails to process a notification? The system uses best-effort async delivery: failures are logged with error details and tracked via metrics, but the central controller continues processing and notifying other controllers without blocking or retrying.
- How does the system handle rapid state changes? The central controller should coalesce rapid changes and provide the latest consistent state.
- What happens when MachineConfigPool selectors or MachineConfig versions change? The central controller immediately re-evaluates all nodes affected by the change (e.g., all nodes matching the updated MCP selector) to ensure internal state remains consistent with the new pool configuration or version.
- What happens if the central controller restarts? On startup, the central controller performs full reconciliation by listing all nodes and evaluating their states to rebuild the internal cache. Downstream controllers re-register and receive the current state snapshot.
- How does the system handle downstream controller deregistration? The central controller should stop notifying deregistered controllers without affecting others.
- What happens when a node is deleted from the cluster? The central controller immediately removes the node's internal state from the concurrent map to prevent memory leaks and ensure the internal state accurately reflects active cluster nodes.
- **[FOLLOWUP]** What happens if someone tries to use the `--enable-node-state-controller` flag after the refactoring? The controller manager either produces a clear error indicating the flag has been removed, or simply ignores the unrecognized flag (depending on flag parsing library behavior).
- **[FOLLOWUP]** What happens if the automatic dependency logic fails to enable the central controller when needed? This should not occur due to deterministic startup logic, but if it does, the dependent controllers (NodeProgressInsight, MCP Progress Insight) will fail fast with clear error messages indicating the missing central controller dependency.
- **[FOLLOWUP]** How does the system behave during the transition from dual-mode to provider-only? This is a breaking change requiring redeployment. No gradual migration is supported; the cutover is clean and immediate upon deploying the refactored version.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a centralized component that watches Nodes, MachineConfigPools, and MachineConfigs.
- **FR-002**: System MUST evaluate and maintain internal representations of node update states using a thread-safe concurrent map (sync.Map or similar) for lockless reads and coordinated writes.
- **FR-003**: System MUST allow downstream insight controllers to register for state change notifications.
- **FR-004**: System MUST trigger downstream controller reconciliation via source.Channels with granular per-resource notifications after central state is updated.
- **FR-005**: System MUST ensure downstream controllers receive consistent state snapshots.
- **FR-006**: System MUST maintain internal node state data structures that are loosely coupled from the NodeProgressInsight CRD structure.
- **FR-007**: System MUST process state changes before notifying downstream controllers.
- **FR-008**: System MUST support multiple concurrent downstream controller registrations.
- **FR-009**: System MUST handle downstream controller registration and deregistration at any time.
- **FR-010**: System MUST continue functioning when no downstream controllers are registered.
- **FR-011**: System MUST provide detailed tracing for state evaluation and notification flows.
- **FR-012**: System MUST expose per-node metrics for state evaluation timing and transitions.
- **FR-013**: System MUST track and expose downstream notification latency metrics.
- **FR-014**: System MUST deliver notifications to downstream controllers using best-effort async semantics: failures are logged and tracked via metrics, but do not block the central controller or trigger automatic retries.
- **FR-015**: System MUST remove internal node state immediately when a node deletion event is received to prevent stale data accumulation.
- **FR-016**: System MUST perform full reconciliation on startup by listing all nodes, evaluating their states, and populating the internal state cache before processing watch events.
- **FR-017**: System MUST re-evaluate all affected nodes immediately when MachineConfigPool selectors or MachineConfig versions change to maintain state consistency.

#### Followup Refactoring Requirements

- **FR-018**: System MUST automatically enable the central node state controller when at least one dependent controller (`--enable-node-controller` or `--enable-mcp-progress-controller`) is enabled, without requiring a separate `--enable-node-state-controller` flag. **[FOLLOWUP]**
- **FR-019**: System MUST NOT start the central node state controller when all dependent controllers are disabled. **[FOLLOWUP]**
- **FR-020**: System MUST remove all legacy mode code from the NodeProgressInsight controller, retaining only provider mode (central controller integration). **[FOLLOWUP]**
- **FR-021**: System MUST fail fast with a clear error if NodeProgressInsight controller is enabled but the central controller is unavailable (should not occur with automatic dependency enablement, but protects against misconfiguration). **[FOLLOWUP]**
- **FR-022**: System MUST remove the `--enable-node-state-controller` flag entirely from the controller manager command-line interface. **[FOLLOWUP]**

### Key Entities

- **Central Node State**: The evaluated state of a node including its update progress, associated pool, version information, and current conditions. This is the internal representation maintained by the central controller in a thread-safe concurrent map (sync.Map or similar) keyed by node name.
- **Downstream Controller Registration**: A record of a controller that wants to be notified of state changes, including a callback mechanism for triggering reconciliation.
- **State Change Notification**: A granular per-resource trigger sent via source.Channels to downstream controllers. NodeProgressInsight controllers receive notifications for specific nodes; MCP Progress Insight controllers receive notifications for any member node change to recompute summaries. Pool distribution changes trigger reconciliation of all affected NodeProgressInsights.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Node state evaluation occurs exactly once per state change, regardless of the number of downstream consumers.
- **SC-002**: All registered downstream controllers receive notifications within the same reconciliation cycle when state changes.
- **SC-003**: The system maintains consistent behavior with 0 to N downstream controllers registered.
- **SC-004**: Downstream controllers produce identical results for the same cluster state, demonstrating they work from consistent state snapshots.
- **SC-005**: Total reconciliation overhead for node state processing decreases compared to current architecture when multiple insight types consume node data.
- **SC-006**: Operators can trace any node state evaluation from trigger to downstream notification completion.
- **SC-007**: Per-node state evaluation timing and downstream notification latency are queryable via metrics.

### Followup Refactoring Success Criteria

- **SC-008**: The controller manager starts successfully with only `--enable-node-controller` flag, automatically enabling the central controller without requiring `--enable-node-state-controller` flag. **[FOLLOWUP]**
- **SC-009**: The controller manager does NOT start the central controller when both `--enable-node-controller=false` and `--enable-mcp-progress-controller=false`. **[FOLLOWUP]**
- **SC-010**: The codebase contains ZERO references to legacy mode logic in the NodeProgressInsight controller after refactoring. **[FOLLOWUP]**
- **SC-011**: All tests for NodeProgressInsight controller pass using only provider mode (no legacy mode tests). **[FOLLOWUP]**
- **SC-012**: Attempting to use the removed `--enable-node-state-controller` flag produces a clear error message or is completely unrecognized. **[FOLLOWUP]**

## Assumptions

- The central controller will run in the same process as downstream controllers, enabling in-memory state sharing.
- Downstream controllers trust the central controller's state evaluation without independent verification.
- The existing node state evaluation logic can be extracted and reused in the central controller.
- Registration/deregistration operations are infrequent compared to state change notifications.
- The number of downstream controllers is small (2-5), not hundreds.
- **[FOLLOWUP]**: The central controller has been proven stable in production, making the legacy mode safety mechanism unnecessary.
- **[FOLLOWUP]**: Users do not need to explicitly control central controller enablement; automatic dependency management is acceptable.
- **[FOLLOWUP]**: The MCP Progress Insight controller (when implemented) will also depend on the central node state controller.

## Constraints

- **Controller-Runtime Compliance**: Moderate deviation from controller-runtime best practices is acceptable. Internal channels and shared in-memory state are allowed, but all controllers must remain registered with the manager and follow standard lifecycle (Start, Stop, health checks). Custom event loops or bypassing the manager entirely are not permitted.

## Scope Boundaries

**In Scope**:
- Central controller watching Nodes, MachineConfigPools, and MachineConfigs
- Internal state management for evaluated node states
- Registration mechanism for downstream controllers
- Notification mechanism to trigger downstream reconciliation
- NodeProgressInsight controller as a downstream consumer
- MachineConfigPoolProgressInsight controller as a downstream consumer
- **[FOLLOWUP]**: Removal of `--enable-node-state-controller` flag from controller manager
- **[FOLLOWUP]**: Automatic central controller enablement based on dependent controller flags
- **[FOLLOWUP]**: Removal of legacy mode from NodeProgressInsight controller
- **[FOLLOWUP]**: Removal of dual-mode support code and tests

**Out of Scope**:
- Changes to the ProgressInsight CRD definitions
- Changes to how insights are exposed to external consumers
- ClusterVersion or ClusterOperator insight controllers (they don't need node state)
- Persistence of internal state beyond process lifetime
- Distribution of central controller across multiple processes
- **[FOLLOWUP]**: Migration tooling or backwards compatibility for the removed flag (breaking change acceptable)
- **[FOLLOWUP]**: Gradual rollout or feature toggle for the refactoring (clean cutover expected)

## Dependencies

### Original Implementation
- Existing NodeProgressInsight controller logic for node state evaluation
- Existing MachineConfigPool selector cache and version cache implementations

### Followup Refactoring
- **[FOLLOWUP]**: Completed and stable central node state controller implementation (delivered in original work)
- **[FOLLOWUP]**: Existing provider mode implementation in NodeProgressInsight controller (to be retained)
- **[FOLLOWUP]**: Controller manager flag parsing and initialization logic (to be modified for automatic enablement)

## Clarifications

### Session 2026-01-16

- Q: What is the notification granularity for downstream controllers? → A: Use controller-runtime source.Channels with granular per-resource triggers. NodeProgressInsight controllers copy internal state to API; MCP Progress Insights react to all member node changes for summary recomputation. Pool distribution changes (e.g., MCP selector changes) may require reconciling all NodeProgressInsights.
- Q: What level of observability is needed? → A: Full observability with detailed tracing, per-node metrics, and downstream latency tracking.
- Q: How far can implementation deviate from controller-runtime best practices? → A: Moderate deviation - internal channels and shared state allowed, but controllers remain registered with the manager and follow standard lifecycle.

### Session 2026-01-17

- Q: How should the central controller handle downstream notification failures? → A: Best-effort async delivery with logging and metrics for failed notifications
- Q: What data structure should be used for storing internal node state with concurrent access? → A: Thread-safe concurrent map (sync.Map or similar)
- Q: How should the central controller handle state cleanup for deleted nodes? → A: Remove state immediately when node deletion event received
- Q: How should the central controller initialize its internal state on startup? → A: Perform full reconciliation on startup: list all nodes and evaluate states
- Q: How should the central controller handle MachineConfigPool selector or MachineConfig version changes? → A: Re-evaluate all affected nodes immediately
- Q: **[FOLLOWUP]** How should the central controller be enabled/disabled? → A: Remove the `--enable-node-state-controller` flag entirely. Automatically enable the central controller when at least one dependent controller (`--enable-node-controller` or `--enable-mcp-progress-controller`) is enabled. Remove legacy mode from NodeProgressInsight controller.
