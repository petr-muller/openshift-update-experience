# Feature Specification: Centralized Node State Controller

**Feature Branch**: `001-central-node-state-controller`
**Created**: 2026-01-16
**Status**: Draft
**Input**: User description: "I want to make the internal controller structure more efficient, possibly at the cost of not entirely respecting controller-runtime best practices. Both the MCP progress insights and Node progress insights actually need to have access to evaluated Node states. This could be implemented through a new central controller that would watch Nodes, and potentially MCPs and MachineConfigs, just like the current NodeProgressInsight does. This controller would served as a backend, and would track basically NodeProgressInsights, but not in the k8s API surface, but internally (and in a data structure only loosely coupled with actual NodeProgressInsights). The controllers reconciling the actual Node- and MachineConfigPoolProgress insights would be downstream of this controller. They would register themselves with this controller internally, and their reconciliation would be triggered by the central controller after the central state is maintained."

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

### User Story 4 - Graceful Degradation Without Downstream Consumers (Priority: P3)

As an operator developer, I need the central controller to function correctly even when no downstream controllers are registered so that the system remains stable during startup or partial deployments.

**Why this priority**: Robustness is important but this is an edge case. The system should handle registration timing gracefully.

**Independent Test**: Can be tested by running the central controller without any registered downstream handlers and verifying it processes state without errors.

**Acceptance Scenarios**:

1. **Given** the central controller is running with no registered downstream controllers, **When** node state changes occur, **Then** the central controller processes and maintains internal state without errors.

2. **Given** a downstream controller registers after state has already been processed, **When** registration completes, **Then** the controller receives the current state snapshot.

---

### Edge Cases

- What happens when a downstream controller fails to process a notification? The system should log the error and continue notifying other controllers.
- How does the system handle rapid state changes? The central controller should coalesce rapid changes and provide the latest consistent state.
- What happens if the central controller restarts? Downstream controllers should re-register and receive current state.
- How does the system handle downstream controller deregistration? The central controller should stop notifying deregistered controllers without affecting others.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a centralized component that watches Nodes, MachineConfigPools, and MachineConfigs.
- **FR-002**: System MUST evaluate and maintain internal representations of node update states.
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

### Key Entities

- **Central Node State**: The evaluated state of a node including its update progress, associated pool, version information, and current conditions. This is the internal representation maintained by the central controller.
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

## Assumptions

- The central controller will run in the same process as downstream controllers, enabling in-memory state sharing.
- Downstream controllers trust the central controller's state evaluation without independent verification.
- The existing node state evaluation logic can be extracted and reused in the central controller.
- Registration/deregistration operations are infrequent compared to state change notifications.
- The number of downstream controllers is small (2-5), not hundreds.

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

**Out of Scope**:
- Changes to the ProgressInsight CRD definitions
- Changes to how insights are exposed to external consumers
- ClusterVersion or ClusterOperator insight controllers (they don't need node state)
- Persistence of internal state beyond process lifetime
- Distribution of central controller across multiple processes

## Dependencies

- Existing NodeProgressInsight controller logic for node state evaluation
- Existing MachineConfigPool selector cache and version cache implementations

## Clarifications

### Session 2026-01-16

- Q: What is the notification granularity for downstream controllers? → A: Use controller-runtime source.Channels with granular per-resource triggers. NodeProgressInsight controllers copy internal state to API; MCP Progress Insights react to all member node changes for summary recomputation. Pool distribution changes (e.g., MCP selector changes) may require reconciling all NodeProgressInsights.
- Q: What level of observability is needed? → A: Full observability with detailed tracing, per-node metrics, and downstream latency tracking.
- Q: How far can implementation deviate from controller-runtime best practices? → A: Moderate deviation - internal channels and shared state allowed, but controllers remain registered with the manager and follow standard lifecycle.
