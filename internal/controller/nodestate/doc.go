/*
Package nodestate provides a centralized node state controller for the OpenShift Update Experience operator.

# Overview

This package implements a central controller that watches Nodes, MachineConfigPools, and MachineConfigs,
evaluates node update states, and maintains an internal state store. Downstream controllers
(NodeProgressInsight and MCPProgressInsight) register for notifications via source.Channels
and receive consistent, pre-evaluated node state snapshots.

# Architecture

The centralized approach eliminates duplicate state evaluation across multiple insight controllers
and ensures all downstream consumers work with consistent state at any given moment.

Key components:

  - CentralNodeStateController: Main controller that watches upstream resources and maintains state
  - NodeState: Internal representation of evaluated node update state
  - NodeStateStore: Thread-safe storage for NodeState instances using sync.Map
  - MachineConfigPoolSelectorCache: Cache of MCP label selectors for node-to-pool mapping
  - MachineConfigVersionCache: Cache of MachineConfig to OCP version mappings

# Notification Pattern

The controller uses controller-runtime's source.Channel mechanism to notify downstream controllers:

  - nodeInsightEvents channel: Triggers NodeProgressInsight reconciliation
  - mcpInsightEvents channel: Triggers MCPProgressInsight reconciliation

Notifications are granular per-resource events, allowing downstream controllers to reconcile
specific resources rather than re-evaluating all state.

# Thread Safety

All operations are thread-safe:

  - State storage uses sync.Map for concurrent access
  - Channel sends are non-blocking with context cancellation support
  - No shared mutable state without synchronization

# Usage

The central controller should be created before downstream controllers in main.go:

	centralController := nodestate.NewCentralNodeStateController(mgr.GetClient(), mgr.GetScheme())
	centralController.SetupWithManager(mgr)

	// Pass to downstream controllers
	nodeReconciler := nodes.NewReconciler(client, scheme, centralController)
*/
package nodestate
