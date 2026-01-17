package nodestate

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// defaultChannelBufferSize is the buffer size for notification channels.
// 1024 provides sufficient buffering for clusters with many nodes.
const defaultChannelBufferSize = 1024

// NodeStateProvider defines the interface for accessing node state.
// Downstream controllers (NodeProgressInsight, MCPProgressInsight) use this interface
// to read the pre-computed node state instead of evaluating it themselves.
type NodeStateProvider interface {
	// GetNodeState returns the current evaluated state for a single node.
	// Returns (state, true) if found, (nil, false) if the node is not tracked.
	GetNodeState(nodeName string) (*NodeState, bool)

	// GetAllNodeStates returns all currently tracked node states.
	// Used for bulk operations and debugging.
	GetAllNodeStates() []*NodeState

	// GetNodeStatesByPool returns all nodes belonging to a specific MachineConfigPool.
	// Used by MCP insight controller to calculate pool summaries.
	GetNodeStatesByPool(poolName string) []*NodeState

	// NodeInsightChannel returns a receive-only channel for node insight
	// notifications. Downstream NodeProgressInsight controllers watch this
	// to trigger reconciliation when node state changes.
	NodeInsightChannel() <-chan event.GenericEvent

	// MCPInsightChannel returns a receive-only channel for MCP insight
	// notifications. Used by future MCP progress insight controller.
	MCPInsightChannel() <-chan event.GenericEvent
}

// CentralNodeStateController is the central controller for evaluating node update state.
// It watches Nodes, MachineConfigPools, and MachineConfigs, evaluates node state once,
// and provides consistent state snapshots to downstream controllers.
type CentralNodeStateController struct {
	client.Client

	// stateStore holds the evaluated node states
	stateStore NodeStateStore

	// mcpSelectors caches MCP label selectors for node->pool mapping
	mcpSelectors MachineConfigPoolSelectorCache

	// machineConfigVersions caches MC->version mappings
	machineConfigVersions MachineConfigVersionCache

	// evaluator performs node state evaluation
	evaluator StateEvaluator

	// totalEvaluations tracks total state evaluations (for metrics/testing)
	totalEvaluations atomic.Int64

	// clusterVersionName is the name of the ClusterVersion resource (usually "version")
	clusterVersionName string

	// now function for testability
	now func() time.Time

	// Downstream notification channels
	// nodeInsightEvents notifies NodeProgressInsight controllers of state changes
	nodeInsightEvents chan event.GenericEvent
	// mcpInsightEvents notifies MCPProgressInsight controllers of state changes
	mcpInsightEvents chan event.GenericEvent

	// metrics records controller metrics
	metrics *MetricsRecorder
}

// NewCentralNodeStateController creates a new CentralNodeStateController.
func NewCentralNodeStateController(client client.Client) *CentralNodeStateController {
	return &CentralNodeStateController{
		Client:             client,
		evaluator:          NewDefaultStateEvaluator(),
		clusterVersionName: "version",
		now:                time.Now,
		nodeInsightEvents:  make(chan event.GenericEvent, defaultChannelBufferSize),
		mcpInsightEvents:   make(chan event.GenericEvent, defaultChannelBufferSize),
		metrics:            NewMetricsRecorder(),
	}
}

// NewCentralNodeStateControllerWithOptions creates a controller with custom options for testing.
func NewCentralNodeStateControllerWithOptions(client client.Client, evaluator StateEvaluator, nowFunc func() time.Time) *CentralNodeStateController {
	ctrl := NewCentralNodeStateController(client)
	if evaluator != nil {
		ctrl.evaluator = evaluator
	}
	if nowFunc != nil {
		ctrl.now = nowFunc
	}
	return ctrl
}

// Reconcile evaluates node state and stores it in the central store.
// This is the single point of node state evaluation - downstream controllers
// read from GetNodeState() instead of evaluating themselves.
func (c *CentralNodeStateController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := c.now()

	// Structured logging with correlation IDs
	logger := logf.FromContext(ctx).WithValues(
		"node", req.Name,
		"operation", "reconcile",
	)

	// Get the node
	var node corev1.Node
	if err := c.Get(ctx, req.NamespacedName, &node); err != nil {
		if errors.IsNotFound(err) {
			// Node was deleted - remove from state store
			if c.stateStore.Delete(req.Name) {
				logger.V(4).Info("Removed deleted node from state store")
				c.updateMetrics()
			}
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Node")
		c.metrics.RecordEvaluationTotal("unknown", false)
		return ctrl.Result{}, err
	}

	// Add resourceVersion to logger for full correlation
	logger = logger.WithValues("resourceVersion", node.ResourceVersion)

	// Determine which MCP the node belongs to
	mcpName := c.mcpSelectors.WhichMCP(labels.Set(node.Labels))
	if mcpName == "" {
		// Node doesn't belong to any MCP - clean up stale state if present
		if c.stateStore.Delete(req.Name) {
			logger.V(4).Info("Removed node without MCP from state store")
			c.updateMetrics()
		}
		return ctrl.Result{}, nil
	}

	// Add pool to logger
	logger = logger.WithValues("pool", mcpName)

	// Get the MachineConfigPool
	var mcp openshiftmachineconfigurationv1.MachineConfigPool
	if err := c.Get(ctx, client.ObjectKey{Name: mcpName}, &mcp); err != nil {
		if errors.IsNotFound(err) {
			// MCP was deleted - will be handled when MCP deletion triggers reconciliation
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get MachineConfigPool")
		c.metrics.RecordEvaluationTotal(mcpName, false)
		return ctrl.Result{}, err
	}

	// Get the ClusterVersion to determine target version
	var cv openshiftconfigv1.ClusterVersion
	if err := c.Get(ctx, client.ObjectKey{Name: c.clusterVersionName}, &cv); err != nil {
		logger.Error(err, "Failed to get ClusterVersion")
		c.metrics.RecordEvaluationTotal(mcpName, false)
		return ctrl.Result{}, err
	}

	var desiredVersion string
	if len(cv.Status.History) > 0 {
		desiredVersion = cv.Status.History[0].Version
	}

	// Get existing state for condition preservation
	var existingConditions []metav1.Condition
	existingState, hasExisting := c.stateStore.Get(req.Name)
	if hasExisting {
		existingConditions = existingState.Conditions
	}

	evalStartTime := c.now()

	// Evaluate node state
	state := c.evaluator.EvaluateNode(&node, &mcp, c.machineConfigVersions.VersionFor, desiredVersion,
		existingConditions, evalStartTime)

	evalDuration := c.now().Sub(evalStartTime).Seconds()

	if state == nil {
		logger.V(4).Info("Evaluation returned nil state")
		c.metrics.RecordEvaluationDuration(req.Name, mcpName, evalDuration, false)
		return ctrl.Result{}, nil
	}

	// Check if state actually changed
	if hasExisting && existingState.StateHash == state.StateHash {
		logger.V(5).Info("Node state unchanged, skipping update",
			"duration", c.now().Sub(startTime).String(),
		)
		return ctrl.Result{}, nil
	}

	// Update state store
	state.EvaluationCount = 1
	if hasExisting {
		state.EvaluationCount = existingState.EvaluationCount + 1
	}

	c.stateStore.Set(req.Name, state)
	c.totalEvaluations.Add(1)

	// Record metrics
	c.metrics.RecordEvaluationDuration(req.Name, mcpName, evalDuration, true)
	c.metrics.RecordEvaluationTotal(mcpName, true)
	c.updateMetrics()

	logger.V(4).Info("Updated node state",
		"phase", state.Phase,
		"version", state.Version,
		"desiredVersion", state.DesiredVersion,
		"evaluationCount", state.EvaluationCount,
		"duration", c.now().Sub(startTime).String(),
	)

	// Notify downstream controllers of the state change
	notifyStart := c.now()
	if err := c.NotifyNodeInsightControllers(ctx, &node); err != nil {
		logger.V(4).Info("Failed to notify node insight controllers", "error", err)
		c.metrics.RecordNotificationDuration("node", c.now().Sub(notifyStart).Seconds(), false)
	} else {
		c.metrics.RecordNotificationDuration("node", c.now().Sub(notifyStart).Seconds(), true)
	}

	if state.PoolRef.Name != "" {
		mcpNotifyStart := c.now()
		if err := c.NotifyMCPInsightControllers(ctx, state.PoolRef.Name); err != nil {
			logger.V(4).Info("Failed to notify MCP insight controllers", "error", err)
			c.metrics.RecordNotificationDuration("mcp", c.now().Sub(mcpNotifyStart).Seconds(), false)
		} else {
			c.metrics.RecordNotificationDuration("mcp", c.now().Sub(mcpNotifyStart).Seconds(), true)
		}
	}

	return ctrl.Result{}, nil
}

// updateMetrics updates the active node count and phase distribution metrics.
func (c *CentralNodeStateController) updateMetrics() {
	c.metrics.SetActiveNodeCount(c.stateStore.Count())

	// Calculate phase distribution
	phaseCounts := make(map[UpdatePhase]int)
	c.stateStore.Range(func(_ string, state *NodeState) bool {
		phaseCounts[state.Phase]++
		return true
	})
	c.metrics.SetNodesByPhase(phaseCounts)
}

// GetNodeState implements NodeStateProvider.
func (c *CentralNodeStateController) GetNodeState(nodeName string) (*NodeState, bool) {
	return c.stateStore.Get(nodeName)
}

// GetAllNodeStates implements NodeStateProvider.
func (c *CentralNodeStateController) GetAllNodeStates() []*NodeState {
	return c.stateStore.GetAll()
}

// GetNodeStatesByPool implements NodeStateProvider.
func (c *CentralNodeStateController) GetNodeStatesByPool(poolName string) []*NodeState {
	return c.stateStore.GetByPool(poolName)
}

// GetTotalEvaluations returns the total number of state evaluations performed.
// This is primarily useful for testing to verify single-evaluation behavior.
func (c *CentralNodeStateController) GetTotalEvaluations() int64 {
	return c.totalEvaluations.Load()
}

// GetTrackedNodeCount returns the number of nodes currently in the state store.
func (c *CentralNodeStateController) GetTrackedNodeCount() int {
	return c.stateStore.Count()
}

// GetCurrentSnapshot returns a snapshot of all current node states.
// This is used by late-registering downstream controllers to get the current
// state of all nodes when they start up, without needing to wait for state
// changes to receive notifications.
func (c *CentralNodeStateController) GetCurrentSnapshot() []*NodeState {
	return c.stateStore.GetAll()
}

// NodeInsightChannel implements NodeStateProvider.
// Returns a receive-only channel for node insight notifications.
func (c *CentralNodeStateController) NodeInsightChannel() <-chan event.GenericEvent {
	return c.nodeInsightEvents
}

// MCPInsightChannel implements NodeStateProvider.
// Returns a receive-only channel for MCP insight notifications.
func (c *CentralNodeStateController) MCPInsightChannel() <-chan event.GenericEvent {
	return c.mcpInsightEvents
}

// NotifyNodeInsightControllers sends a notification that a node's state has changed.
// The notification is non-blocking - if the channel buffer is full, the notification
// is dropped (downstream controllers will eventually reconcile anyway).
func (c *CentralNodeStateController) NotifyNodeInsightControllers(ctx context.Context, node *corev1.Node) error {
	select {
	case c.nodeInsightEvents <- event.GenericEvent{Object: node}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel buffer full - notification dropped
		// This is acceptable as downstream controllers will reconcile eventually
		return nil
	}
}

// NotifyMCPInsightControllers sends a notification that MCP-level state may have changed.
// The notification is non-blocking.
func (c *CentralNodeStateController) NotifyMCPInsightControllers(ctx context.Context, poolName string) error {
	// Create a synthetic object to carry the pool name
	// We use a minimal metav1.Object that can carry the name
	syntheticObj := &mcpNotificationObject{name: poolName}
	select {
	case c.mcpInsightEvents <- event.GenericEvent{Object: syntheticObj}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel buffer full - notification dropped
		return nil
	}
}

// NotifyAllDownstream sends notifications to all downstream controllers.
// Used when global state changes (e.g., ClusterVersion changes).
func (c *CentralNodeStateController) NotifyAllDownstream(ctx context.Context) error {
	// Collect unique pools from all tracked nodes
	pools := make(map[string]struct{})

	// Notify for all tracked nodes
	c.stateStore.Range(func(name string, state *NodeState) bool {
		// Create synthetic node object for notification
		node := &corev1.Node{}
		node.Name = name
		select {
		case c.nodeInsightEvents <- event.GenericEvent{Object: node}:
		case <-ctx.Done():
			return false
		default:
			// Buffer full, continue
		}
		if state.PoolRef.Name != "" {
			pools[state.PoolRef.Name] = struct{}{}
		}
		return true
	})

	// Notify for all unique pools
	for poolName := range pools {
		select {
		case c.mcpInsightEvents <- event.GenericEvent{Object: &mcpNotificationObject{name: poolName}}:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Buffer full, continue
		}
	}

	return nil
}

// mcpNotificationObject is a minimal implementation of client.Object
// used to carry the MCP name in notifications.
type mcpNotificationObject struct {
	name string
}

func (m *mcpNotificationObject) GetName() string                               { return m.name }
func (m *mcpNotificationObject) GetNamespace() string                          { return "" }
func (m *mcpNotificationObject) SetName(name string)                           { m.name = name }
func (m *mcpNotificationObject) SetNamespace(namespace string)                 {}
func (m *mcpNotificationObject) GetGenerateName() string                       { return "" }
func (m *mcpNotificationObject) SetGenerateName(name string)                   {}
func (m *mcpNotificationObject) GetResourceVersion() string                    { return "" }
func (m *mcpNotificationObject) SetResourceVersion(version string)             {}
func (m *mcpNotificationObject) GetGeneration() int64                          { return 0 }
func (m *mcpNotificationObject) SetGeneration(generation int64)                {}
func (m *mcpNotificationObject) GetSelfLink() string                           { return "" }
func (m *mcpNotificationObject) SetSelfLink(selfLink string)                   {}
func (m *mcpNotificationObject) GetUID() types.UID                             { return "" }
func (m *mcpNotificationObject) SetUID(uid types.UID)                          {}
func (m *mcpNotificationObject) GetCreationTimestamp() metav1.Time             { return metav1.Time{} }
func (m *mcpNotificationObject) SetCreationTimestamp(timestamp metav1.Time)    {}
func (m *mcpNotificationObject) GetDeletionTimestamp() *metav1.Time            { return nil }
func (m *mcpNotificationObject) SetDeletionTimestamp(timestamp *metav1.Time)   {}
func (m *mcpNotificationObject) GetDeletionGracePeriodSeconds() *int64         { return nil }
func (m *mcpNotificationObject) SetDeletionGracePeriodSeconds(i *int64)        {}
func (m *mcpNotificationObject) GetLabels() map[string]string                  { return nil }
func (m *mcpNotificationObject) SetLabels(labels map[string]string)            {}
func (m *mcpNotificationObject) GetAnnotations() map[string]string             { return nil }
func (m *mcpNotificationObject) SetAnnotations(annotations map[string]string)  {}
func (m *mcpNotificationObject) GetFinalizers() []string                       { return nil }
func (m *mcpNotificationObject) SetFinalizers(finalizers []string)             {}
func (m *mcpNotificationObject) GetOwnerReferences() []metav1.OwnerReference   { return nil }
func (m *mcpNotificationObject) SetOwnerReferences([]metav1.OwnerReference)    {}
func (m *mcpNotificationObject) GetManagedFields() []metav1.ManagedFieldsEntry { return nil }
func (m *mcpNotificationObject) SetManagedFields([]metav1.ManagedFieldsEntry)  {}
func (m *mcpNotificationObject) GetObjectKind() schema.ObjectKind              { return schema.EmptyObjectKind }
func (m *mcpNotificationObject) DeepCopyObject() runtime.Object {
	return &mcpNotificationObject{name: m.name}
}

// HandleMachineConfigPool handles MCP create/update events.
// When an MCP selector changes, all nodes may need re-evaluation.
func (c *CentralNodeStateController) HandleMachineConfigPool(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	pool, ok := obj.(*openshiftmachineconfigurationv1.MachineConfigPool)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfigPool", obj), "Failed to handle MachineConfigPool")
		return nil
	}

	modified, reason := c.mcpSelectors.Ingest(pool.Name, pool.Spec.NodeSelector)
	if !modified {
		return nil
	}

	logger.V(4).Info("MachineConfigPool selector changed", "pool", pool.Name, "reason", reason)

	// Request reconciliation for all nodes
	return c.requestsForAllNodes(ctx)
}

// HandleDeletedMachineConfigPool handles MCP deletion events.
func (c *CentralNodeStateController) HandleDeletedMachineConfigPool(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	pool, ok := obj.(*openshiftmachineconfigurationv1.MachineConfigPool)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfigPool", obj), "Failed to handle deleted MachineConfigPool")
		return nil
	}

	if !c.mcpSelectors.Forget(pool.Name) {
		return nil
	}

	logger.V(4).Info("MachineConfigPool deleted", "pool", pool.Name)

	// Request reconciliation for all nodes
	return c.requestsForAllNodes(ctx)
}

// HandleMachineConfig handles MachineConfig create/update events.
func (c *CentralNodeStateController) HandleMachineConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	mc, ok := obj.(*openshiftmachineconfigurationv1.MachineConfig)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfig", obj), "Failed to handle MachineConfig")
		return nil
	}

	modified, reason := c.machineConfigVersions.Ingest(mc)
	if !modified {
		return nil
	}

	logger.V(4).Info("MachineConfig version changed", "mc", mc.Name, "reason", reason)

	// Request reconciliation for all nodes
	return c.requestsForAllNodes(ctx)
}

// HandleDeletedMachineConfig handles MachineConfig deletion events.
func (c *CentralNodeStateController) HandleDeletedMachineConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	mc, ok := obj.(*openshiftmachineconfigurationv1.MachineConfig)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfig", obj), "Failed to handle deleted MachineConfig")
		return nil
	}

	if !c.machineConfigVersions.Forget(mc.Name) {
		return nil
	}

	logger.V(4).Info("MachineConfig deleted", "mc", mc.Name)

	// Request reconciliation for all nodes
	return c.requestsForAllNodes(ctx)
}

// HandleClusterVersion handles ClusterVersion changes.
// When the target version changes, all nodes need re-evaluation.
func (c *CentralNodeStateController) HandleClusterVersion(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	cv, ok := obj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a ClusterVersion", obj), "Failed to handle ClusterVersion")
		return nil
	}

	logger.V(4).Info("ClusterVersion changed", "version", cv.Name)

	// Request reconciliation for all nodes when CV changes
	return c.requestsForAllNodes(ctx)
}

// requestsForAllNodes returns reconcile requests for all nodes in the cluster.
func (c *CentralNodeStateController) requestsForAllNodes(ctx context.Context) []reconcile.Request {
	logger := logf.FromContext(ctx)

	var nodes corev1.NodeList
	if err := c.List(ctx, &nodes); err != nil {
		logger.Error(err, "Failed to list nodes")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: node.Name},
		})
	}

	return requests
}

// InitializeCaches populates the caches from existing cluster resources.
// This should be called once during controller startup.
func (c *CentralNodeStateController) InitializeCaches(ctx context.Context) error {
	logger := logf.FromContext(ctx)

	// Initialize MCP selectors
	var mcps openshiftmachineconfigurationv1.MachineConfigPoolList
	if err := c.List(ctx, &mcps); err != nil {
		return fmt.Errorf("failed to list MachineConfigPools: %w", err)
	}
	logger.V(2).Info("Initializing MCP selector cache", "count", len(mcps.Items))
	for _, pool := range mcps.Items {
		if ingested, reason := c.mcpSelectors.Ingest(pool.Name, pool.Spec.NodeSelector); ingested {
			logger.V(4).Info("Ingested MCP selector", "pool", pool.Name, "reason", reason)
		}
	}

	// Initialize MC versions
	var mcs openshiftmachineconfigurationv1.MachineConfigList
	if err := c.List(ctx, &mcs); err != nil {
		return fmt.Errorf("failed to list MachineConfigs: %w", err)
	}
	logger.V(2).Info("Initializing MC version cache", "count", len(mcs.Items))
	for _, mc := range mcs.Items {
		if ingested, reason := c.machineConfigVersions.Ingest(&mc); ingested {
			logger.V(4).Info("Ingested MC version", "mc", mc.Name, "reason", reason)
		}
	}

	return nil
}

// Start implements manager.Runnable interface.
// It blocks until the context is cancelled, then performs graceful shutdown.
// T063: Implement manager.Runnable interface
func (c *CentralNodeStateController) Start(ctx context.Context) error {
	logger := logf.FromContext(ctx).WithName("central-node-state-controller")
	logger.Info("Starting central node state controller")

	// Block until context is cancelled
	<-ctx.Done()

	// T064: Graceful shutdown - close notification channels
	logger.Info("Shutting down central node state controller, closing notification channels")

	if c.nodeInsightEvents != nil {
		close(c.nodeInsightEvents)
	}
	if c.mcpInsightEvents != nil {
		close(c.mcpInsightEvents)
	}

	logger.Info("Central node state controller shutdown complete")
	return nil
}

// NeedLeaderElection implements manager.LeaderElectionRunnable.
// Returns true to indicate this controller should only run on the leader.
// T065: Health check integration via leader election
func (c *CentralNodeStateController) NeedLeaderElection() bool {
	return true
}
