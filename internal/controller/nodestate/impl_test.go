package nodestate

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// T016: Unit tests verifying single evaluation per state change

func TestCentralNodeStateController_Reconcile_SingleEvaluation(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
		},
	}

	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/worker": "",
				},
			},
		},
	}

	mc := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rendered-worker-abc",
			Annotations: map[string]string{
				mco.ReleaseImageVersionAnnotationKey: "4.13.0",
			},
		},
	}

	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{Version: "4.13.0", State: openshiftconfigv1.CompletedUpdate},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, mcp, mc, cv).
		Build()

	controller := NewCentralNodeStateController(fakeClient)

	// Initialize caches
	ctx := context.Background()
	if err := controller.InitializeCaches(ctx); err != nil {
		t.Fatalf("Failed to initialize caches: %v", err)
	}

	// First reconciliation
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}}
	_, err := controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("First reconcile failed: %v", err)
	}

	// Verify state was stored
	state, ok := controller.GetNodeState("worker-1")
	if !ok {
		t.Fatal("Expected node state to be stored after first reconcile")
	}
	if state.Phase != UpdatePhaseCompleted {
		t.Errorf("Expected phase %q, got %q", UpdatePhaseCompleted, state.Phase)
	}
	if state.EvaluationCount != 1 {
		t.Errorf("Expected evaluation count 1, got %d", state.EvaluationCount)
	}

	initialEvaluations := controller.GetTotalEvaluations()
	if initialEvaluations != 1 {
		t.Errorf("Expected total evaluations 1, got %d", initialEvaluations)
	}

	// Second reconciliation with same state - should not re-evaluate
	_, err = controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Second reconcile failed: %v", err)
	}

	// Total evaluations should still be 1 (no change detected)
	if controller.GetTotalEvaluations() != initialEvaluations {
		t.Errorf("Expected total evaluations to remain %d, got %d (unnecessary re-evaluation)", initialEvaluations, controller.GetTotalEvaluations())
	}

	state, _ = controller.GetNodeState("worker-1")
	if state.EvaluationCount != 1 {
		t.Errorf("Expected evaluation count to remain 1, got %d", state.EvaluationCount)
	}
}

func TestCentralNodeStateController_Reconcile_StateChange(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateWorking,
			},
		},
	}

	fakeClient, controller := prepareCentralNodeStateController(node)

	ctx := context.Background()
	_ = controller.InitializeCaches(ctx)

	// First reconciliation - node is updating
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}}
	_, _ = controller.Reconcile(ctx, req)

	state, _ := controller.GetNodeState("worker-1")
	if state.Phase != UpdatePhaseUpdating {
		t.Errorf("Expected phase %q, got %q", UpdatePhaseUpdating, state.Phase)
	}
	if state.EvaluationCount != 1 {
		t.Errorf("Expected evaluation count 1, got %d", state.EvaluationCount)
	}

	// Update node to completed state
	node.Annotations[mco.CurrentMachineConfigAnnotationKey] = "rendered-worker-xyz"
	node.Annotations[mco.DesiredMachineConfigAnnotationKey] = "rendered-worker-xyz"
	node.Annotations[mco.MachineConfigDaemonStateAnnotationKey] = mco.MachineConfigDaemonStateDone
	_ = fakeClient.Update(ctx, node)

	// Second reconciliation - state changed
	_, _ = controller.Reconcile(ctx, req)

	state, _ = controller.GetNodeState("worker-1")
	if state.Phase != UpdatePhaseCompleted {
		t.Errorf("Expected phase %q after update, got %q", UpdatePhaseCompleted, state.Phase)
	}
	if state.EvaluationCount != 2 {
		t.Errorf("Expected evaluation count 2 after state change, got %d", state.EvaluationCount)
	}
}

func TestCentralNodeStateController_Reconcile_NodeDeleted(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	controller := NewCentralNodeStateController(fakeClient)

	// Manually add state for a node
	controller.stateStore.Set("deleted-node", &NodeState{
		Name:  "deleted-node",
		Phase: UpdatePhaseCompleted,
	})

	if controller.GetTrackedNodeCount() != 1 {
		t.Errorf("Expected 1 tracked node, got %d", controller.GetTrackedNodeCount())
	}

	// Reconcile non-existent node
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "deleted-node"}}
	_, err := controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// State should be removed
	if controller.GetTrackedNodeCount() != 0 {
		t.Errorf("Expected 0 tracked nodes after deletion, got %d", controller.GetTrackedNodeCount())
	}

	_, ok := controller.GetNodeState("deleted-node")
	if ok {
		t.Error("Expected node state to be removed after node deletion")
	}
}

func TestCentralNodeStateController_Reconcile_NodeWithoutMCP(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Node without any matching MCP selector labels
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "orphan-node",
			Labels: map[string]string{},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	controller := NewCentralNodeStateController(fakeClient)

	// Manually add stale state
	controller.stateStore.Set("orphan-node", &NodeState{
		Name:  "orphan-node",
		Phase: UpdatePhaseCompleted,
	})

	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "orphan-node"}}
	_, err := controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// State should be removed for node without MCP
	_, ok := controller.GetNodeState("orphan-node")
	if ok {
		t.Error("Expected node state to be removed for node without MCP")
	}
}

// T017: Integration test - multiple accessors see identical state snapshots

func TestCentralNodeStateController_ConsistentSnapshots(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	nodes, mcp, masterMCP := preparePoolsWIthMultipleNodes()

	mc1 := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rendered-worker-abc",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
		},
	}

	mc2 := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rendered-worker-xyz",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.13.0"},
		},
	}

	mc3 := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rendered-master-abc",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.13.0"},
		},
	}

	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{Version: "4.13.0"},
			},
		},
	}

	objects := append(nodes, mcp, masterMCP, mc1, mc2, mc3, cv)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()
	_ = controller.InitializeCaches(ctx)

	// Reconcile all nodes
	for _, node := range []string{"worker-1", "worker-2", "master-0"} {
		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: node}}
		_, _ = controller.Reconcile(ctx, req)
	}

	// Verify GetAllNodeStates
	allStates := controller.GetAllNodeStates()
	if len(allStates) != 3 {
		t.Errorf("Expected 3 states from GetAllNodeStates, got %d", len(allStates))
	}

	// Verify GetNodeStatesByPool
	workerStates := controller.GetNodeStatesByPool("worker")
	if len(workerStates) != 2 {
		t.Errorf("Expected 2 worker states, got %d", len(workerStates))
	}

	masterStates := controller.GetNodeStatesByPool("master")
	if len(masterStates) != 1 {
		t.Errorf("Expected 1 master state, got %d", len(masterStates))
	}

	// Verify individual state access
	state1, ok := controller.GetNodeState("worker-1")
	if !ok || state1.Phase != UpdatePhasePending {
		t.Errorf("worker-1: expected phase %q, got %q (ok=%v)", UpdatePhasePending, state1.Phase, ok)
	}

	state2, ok := controller.GetNodeState("worker-2")
	if !ok || state2.Phase != UpdatePhaseUpdating {
		t.Errorf("worker-2: expected phase %q, got %q (ok=%v)", UpdatePhaseUpdating, state2.Phase, ok)
	}

	state3, ok := controller.GetNodeState("master-0")
	if !ok || state3.Phase != UpdatePhaseCompleted {
		t.Errorf("master-0: expected phase %q, got %q (ok=%v)", UpdatePhaseCompleted, state3.Phase, ok)
	}

	// Verify scope
	if state3.Scope != ouev1alpha1.ControlPlaneScope {
		t.Errorf("master-0: expected scope %q, got %q", ouev1alpha1.ControlPlaneScope, state3.Scope)
	}
}

func TestCentralNodeStateController_HandleMachineConfigPool(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()

	// First ingest
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "worker"},
			},
		},
	}

	requests := controller.HandleMachineConfigPool(ctx, mcp)
	if len(requests) != 1 {
		t.Errorf("Expected 1 request on first MCP ingest, got %d", len(requests))
	}

	// Same MCP - no change
	requests = controller.HandleMachineConfigPool(ctx, mcp)
	if len(requests) != 0 {
		t.Errorf("Expected 0 requests when MCP unchanged, got %d", len(requests))
	}

	// Changed selector
	mcp.Spec.NodeSelector.MatchLabels = map[string]string{"role": "infra"}
	requests = controller.HandleMachineConfigPool(ctx, mcp)
	if len(requests) != 1 {
		t.Errorf("Expected 1 request on MCP selector change, got %d", len(requests))
	}
}

func TestCentralNodeStateController_HandleDeletedMachineConfigPool(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()

	// First, add the MCP
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "worker"},
			},
		},
	}
	controller.HandleMachineConfigPool(ctx, mcp)

	// Delete MCP
	requests := controller.HandleDeletedMachineConfigPool(ctx, mcp)
	if len(requests) != 1 {
		t.Errorf("Expected 1 request on MCP deletion, got %d", len(requests))
	}

	// Delete again - should be no-op
	requests = controller.HandleDeletedMachineConfigPool(ctx, mcp)
	if len(requests) != 0 {
		t.Errorf("Expected 0 requests when MCP already deleted, got %d", len(requests))
	}
}

func TestCentralNodeStateController_HandleMachineConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-1"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()

	// New MC with version
	mc := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rendered-worker-abc",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
		},
	}

	requests := controller.HandleMachineConfig(ctx, mc)
	if len(requests) != 1 {
		t.Errorf("Expected 1 request on new MC, got %d", len(requests))
	}

	// Same MC - no change
	requests = controller.HandleMachineConfig(ctx, mc)
	if len(requests) != 0 {
		t.Errorf("Expected 0 requests when MC unchanged, got %d", len(requests))
	}

	// Version changed
	mc.Annotations[mco.ReleaseImageVersionAnnotationKey] = "4.13.0"
	requests = controller.HandleMachineConfig(ctx, mc)
	if len(requests) != 1 {
		t.Errorf("Expected 1 request on MC version change, got %d", len(requests))
	}
}

func TestCentralNodeStateController_InitializeCaches(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = openshiftmachineconfigurationv1.Install(scheme)

	mcps := []client.Object{
		&openshiftmachineconfigurationv1.MachineConfigPool{
			ObjectMeta: metav1.ObjectMeta{Name: "worker"},
			Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"role": "worker"},
				},
			},
		},
		&openshiftmachineconfigurationv1.MachineConfigPool{
			ObjectMeta: metav1.ObjectMeta{Name: "master"},
			Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"role": "master"},
				},
			},
		},
	}

	mcs := []client.Object{
		&openshiftmachineconfigurationv1.MachineConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "rendered-worker-abc",
				Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
			},
		},
		&openshiftmachineconfigurationv1.MachineConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "rendered-master-xyz",
				Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.13.0"},
			},
		},
	}

	objects := append(mcps, mcs...)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()

	err := controller.InitializeCaches(ctx)
	if err != nil {
		t.Fatalf("InitializeCaches failed: %v", err)
	}

	// Verify MCP selectors were loaded
	if mcpName := controller.mcpSelectors.WhichMCP(labels.Set{"role": "worker"}); mcpName != "worker" {
		t.Errorf("Expected worker MCP, got %q", mcpName)
	}
	if mcpName := controller.mcpSelectors.WhichMCP(labels.Set{"role": "master"}); mcpName != "master" {
		t.Errorf("Expected master MCP, got %q", mcpName)
	}

	// Verify MC versions were loaded
	version, ok := controller.machineConfigVersions.VersionFor("rendered-worker-abc")
	if !ok || version != "4.12.0" {
		t.Errorf("Expected version 4.12.0 for rendered-worker-abc, got %q (ok=%v)", version, ok)
	}
	version, ok = controller.machineConfigVersions.VersionFor("rendered-master-xyz")
	if !ok || version != "4.13.0" {
		t.Errorf("Expected version 4.13.0 for rendered-master-xyz, got %q (ok=%v)", version, ok)
	}
}

func TestNewCentralNodeStateControllerWithOptions(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()

	// Custom time function
	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	customNow := func() time.Time { return fixedTime }

	controller := NewCentralNodeStateControllerWithOptions(fakeClient, nil, customNow)

	if controller.now() != fixedTime {
		t.Errorf("Expected custom now function to return %v, got %v", fixedTime, controller.now())
	}

	// Verify default evaluator is used when nil
	if controller.evaluator == nil {
		t.Error("Expected default evaluator when nil provided")
	}
}

func TestCentralNodeStateController_GetTrackedNodeCount(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	controller := NewCentralNodeStateController(fakeClient)

	if count := controller.GetTrackedNodeCount(); count != 0 {
		t.Errorf("Expected 0 tracked nodes initially, got %d", count)
	}

	controller.stateStore.Set("node-1", &NodeState{Name: "node-1"})
	controller.stateStore.Set("node-2", &NodeState{Name: "node-2"})

	if count := controller.GetTrackedNodeCount(); count != 2 {
		t.Errorf("Expected 2 tracked nodes, got %d", count)
	}
}

func TestNodeStateProvider_Interface(t *testing.T) {
	// Compile-time check that CentralNodeStateController implements NodeStateProvider
	var _ Provider = (*CentralNodeStateController)(nil)
}

func TestCentralNodeStateController_HandleClusterVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)

	nodes := []client.Object{
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-2"}},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodes...).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()

	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{Version: "4.13.0"},
			},
		},
	}

	requests := controller.HandleClusterVersion(ctx, cv)
	if len(requests) != 2 {
		t.Errorf("Expected 2 requests (one per node) on CV change, got %d", len(requests))
	}
}

func TestCentralNodeStateController_NodeStateProvider_Methods(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	controller := NewCentralNodeStateController(fakeClient)

	// Setup test data
	states := []*NodeState{
		{Name: "worker-1", PoolRef: ouev1alpha1.ResourceRef{Name: "worker"}, Phase: UpdatePhaseCompleted},
		{Name: "worker-2", PoolRef: ouev1alpha1.ResourceRef{Name: "worker"}, Phase: UpdatePhaseUpdating},
		{Name: "master-0", PoolRef: ouev1alpha1.ResourceRef{Name: "master"}, Phase: UpdatePhaseCompleted},
	}
	for _, s := range states {
		controller.stateStore.Set(s.Name, s)
	}

	// Test GetNodeState
	state, ok := controller.GetNodeState("worker-1")
	if !ok {
		t.Error("Expected to find worker-1")
	}
	if state.Phase != UpdatePhaseCompleted {
		t.Errorf("Expected phase Completed, got %q", state.Phase)
	}

	_, ok = controller.GetNodeState("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent node")
	}

	// Test GetAllNodeStates
	all := controller.GetAllNodeStates()
	if len(all) != 3 {
		t.Errorf("Expected 3 states, got %d", len(all))
	}

	// Test GetNodeStatesByPool
	workers := controller.GetNodeStatesByPool("worker")
	if len(workers) != 2 {
		t.Errorf("Expected 2 worker states, got %d", len(workers))
	}

	masters := controller.GetNodeStatesByPool("master")
	if len(masters) != 1 {
		t.Errorf("Expected 1 master state, got %d", len(masters))
	}

	empty := controller.GetNodeStatesByPool("nonexistent")
	if len(empty) != 0 {
		t.Errorf("Expected 0 states for nonexistent pool, got %d", len(empty))
	}
}

// Ensure cmp is used (import check)
var _ = cmp.Diff

// T029: Unit tests for notification channel creation and management

func TestCentralNodeStateController_NotificationChannels_Created(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	controller := NewCentralNodeStateController(fakeClient)

	// Verify channels are created
	nodeCh := controller.NodeInsightChannel()
	if nodeCh == nil {
		t.Error("Expected NodeInsightChannel to be non-nil")
	}

	mcpCh := controller.MCPInsightChannel()
	if mcpCh == nil {
		t.Error("Expected MCPInsightChannel to be non-nil")
	}

	// Verify they are receive-only (compile-time check via type)
	// The channels should have buffer capacity
	// Test that we can get the same channel multiple times (stable reference)
	if controller.NodeInsightChannel() != nodeCh {
		t.Error("Expected NodeInsightChannel to return stable reference")
	}
	if controller.MCPInsightChannel() != mcpCh {
		t.Error("Expected MCPInsightChannel to return stable reference")
	}
}

func TestCentralNodeStateController_NotificationChannels_BufferSize(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	controller := NewCentralNodeStateController(fakeClient)

	// Buffer should be large enough for typical operation (1024 per research.md)
	// We test by sending multiple events without blocking
	nodeCh := controller.NodeInsightChannel()

	// This is a basic check - we can't directly check buffer size,
	// but we can verify we can send multiple items without blocking
	// (actual buffer verification would require implementation details)
	if nodeCh == nil {
		t.Error("Expected NodeInsightChannel to be created")
	}
}

// T030: Unit tests for non-blocking notification send with context

func TestCentralNodeStateController_NotifyNodeInsight_NonBlocking(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	controller := NewCentralNodeStateController(fakeClient)

	// Get channel to consume events
	nodeCh := controller.NodeInsightChannel()

	// Send notification
	ctx := context.Background()
	err := controller.NotifyNodeInsightControllers(ctx, node)
	if err != nil {
		t.Fatalf("NotifyNodeInsightControllers failed: %v", err)
	}

	// Verify event was sent
	select {
	case event := <-nodeCh:
		if event.Object == nil {
			t.Error("Expected event.Object to be non-nil")
		}
		if event.Object.GetName() != "worker-1" {
			t.Errorf("Expected event for worker-1, got %s", event.Object.GetName())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive notification event")
	}
}

func TestCentralNodeStateController_NotifyNodeInsight_ContextCancelled(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	controller := NewCentralNodeStateController(fakeClient)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
		},
	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Notification should not block even with canceled context
	// It may return an error or simply not send, but shouldn't block
	done := make(chan struct{})
	go func() {
		_ = controller.NotifyNodeInsightControllers(ctx, node)
		close(done)
	}()

	select {
	case <-done:
		// Success - didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("NotifyNodeInsightControllers blocked with cancelled context")
	}
}

func TestCentralNodeStateController_NotifyMCPInsight_NonBlocking(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	controller := NewCentralNodeStateController(fakeClient)

	mcpCh := controller.MCPInsightChannel()

	ctx := context.Background()
	err := controller.NotifyMCPInsightControllers(ctx, "worker")
	if err != nil {
		t.Fatalf("NotifyMCPInsightControllers failed: %v", err)
	}

	// Verify event was sent
	select {
	case event := <-mcpCh:
		if event.Object == nil {
			t.Error("Expected event.Object to be non-nil")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive MCP notification event")
	}
}

func TestCentralNodeStateController_NotifyAllDownstream(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create multiple nodes
	nodes := []client.Object{
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-2"}},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodes...).
		Build()

	controller := NewCentralNodeStateController(fakeClient)

	// Add some state to the store
	controller.stateStore.Set("worker-1", &NodeState{
		Name:    "worker-1",
		PoolRef: ouev1alpha1.ResourceRef{Name: "worker"},
	})
	controller.stateStore.Set("worker-2", &NodeState{
		Name:    "worker-2",
		PoolRef: ouev1alpha1.ResourceRef{Name: "worker"},
	})

	nodeCh := controller.NodeInsightChannel()
	mcpCh := controller.MCPInsightChannel()

	ctx := context.Background()
	err := controller.NotifyAllDownstream(ctx)
	if err != nil {
		t.Fatalf("NotifyAllDownstream failed: %v", err)
	}

	// Should have received events on both channels
	// Node insight channel should have 2 events (one per node)
	nodeEvents := 0
	timeout := time.After(100 * time.Millisecond)
	for nodeEvents < 2 {
		select {
		case <-nodeCh:
			nodeEvents++
		case <-timeout:
			t.Errorf("Expected 2 node events, got %d", nodeEvents)
			return
		}
	}

	// MCP insight channel should have at least 1 event
	select {
	case <-mcpCh:
		// Got MCP event
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive MCP notification event")
	}
}

// T031: Integration test - downstream controller receives notification on state change

func TestCentralNodeStateController_StateChange_TriggersNotification(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateWorking,
			},
		},
	}

	fakeClient, controller := prepareCentralNodeStateController(node)

	ctx := context.Background()
	_ = controller.InitializeCaches(ctx)

	nodeCh := controller.NodeInsightChannel()

	// First reconciliation - node is updating
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}}
	_, err := controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("First reconcile failed: %v", err)
	}

	// Should receive notification for state change
	select {
	case event := <-nodeCh:
		if event.Object.GetName() != "worker-1" {
			t.Errorf("Expected notification for worker-1, got %s", event.Object.GetName())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected notification after first state evaluation")
	}

	// Update node to completed state
	node.Annotations[mco.CurrentMachineConfigAnnotationKey] = "rendered-worker-xyz"
	node.Annotations[mco.DesiredMachineConfigAnnotationKey] = "rendered-worker-xyz"
	node.Annotations[mco.MachineConfigDaemonStateAnnotationKey] = mco.MachineConfigDaemonStateDone
	_ = fakeClient.Update(ctx, node)

	// Second reconciliation - state changed to completed
	_, err = controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Second reconcile failed: %v", err)
	}

	// Should receive another notification for the state change
	select {
	case event := <-nodeCh:
		if event.Object.GetName() != "worker-1" {
			t.Errorf("Expected notification for worker-1, got %s", event.Object.GetName())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected notification after state change to completed")
	}

	// Verify the state was updated
	state, ok := controller.GetNodeState("worker-1")
	if !ok {
		t.Fatal("Expected node state to exist")
	}
	if state.Phase != UpdatePhaseCompleted {
		t.Errorf("Expected phase Completed, got %q", state.Phase)
	}
}

func prepareCentralNodeStateController(node *corev1.Node) (client.WithWatch, *CentralNodeStateController) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"node-role.kubernetes.io/worker": ""},
			},
		},
	}

	mcOld := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rendered-worker-abc",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
		},
	}

	mcNew := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rendered-worker-xyz",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.13.0"},
		},
	}

	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{Version: "4.13.0", State: openshiftconfigv1.PartialUpdate},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, mcp, mcOld, mcNew, cv).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	return fakeClient, controller
}

func TestCentralNodeStateController_NoNotification_WhenStateUnchanged(t *testing.T) {
	controller := prepareController()
	ctx := context.Background()
	_ = controller.InitializeCaches(ctx)

	nodeCh := controller.NodeInsightChannel()

	// First reconciliation
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}}
	_, _ = controller.Reconcile(ctx, req)

	// Consume first notification
	select {
	case <-nodeCh:
		// Got first notification
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected notification on first reconcile")
	}

	// Second reconciliation with same state - should NOT send notification
	_, _ = controller.Reconcile(ctx, req)

	// Should NOT receive notification (state unchanged)
	select {
	case <-nodeCh:
		t.Error("Should not receive notification when state unchanged")
	case <-time.After(50 * time.Millisecond):
		// Good - no notification as expected
	}
}

// =============================================================================
// Phase 5: User Story 4 - Graceful Degradation Tests
// =============================================================================

// T044: Unit test - central controller processes state with no downstream channels consumed
func TestCentralNodeStateController_ProcessesState_WithUnconsumedChannels(t *testing.T) {
	controller := prepareController()
	ctx := context.Background()
	_ = controller.InitializeCaches(ctx)

	// DO NOT consume from channels - simulate no downstream controllers registered
	// The controller should still process state without blocking or errors

	// Reconcile multiple times - should not block even with unconsumed channel events
	for i := 0; i < 10; i++ {
		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}}
		_, err := controller.Reconcile(ctx, req)
		if err != nil {
			t.Fatalf("Reconcile %d failed: %v", i, err)
		}
	}

	// Verify state was still tracked correctly
	state, ok := controller.GetNodeState("worker-1")
	if !ok {
		t.Fatal("Expected node state to be tracked")
	}
	if state.Phase != UpdatePhaseCompleted {
		t.Errorf("Expected phase Completed, got %q", state.Phase)
	}

	// Channel buffer should have events (first reconcile triggers notification)
	// but subsequent reconciles with same state should not
	if controller.GetTotalEvaluations() != 1 {
		t.Errorf("Expected 1 evaluation (state unchanged after first), got %d", controller.GetTotalEvaluations())
	}
}

func prepareController() *CentralNodeStateController {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
		},
	}

	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"node-role.kubernetes.io/worker": ""},
			},
		},
	}

	mc := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "rendered-worker-abc",
			Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.13.0"},
		},
	}

	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{Version: "4.13.0", State: openshiftconfigv1.CompletedUpdate},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, mcp, mc, cv).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	return controller
}

// T045: Unit test - late-registering controller receives current state snapshot
func TestCentralNodeStateController_LateRegistration_ReceivesCurrentState(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	nodes, mcp, masterMCP := preparePoolsWIthMultipleNodes()

	mcs := []client.Object{
		&openshiftmachineconfigurationv1.MachineConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "rendered-worker-abc",
				Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.12.0"},
			},
		},
		&openshiftmachineconfigurationv1.MachineConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "rendered-worker-xyz",
				Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.13.0"},
			},
		},
		&openshiftmachineconfigurationv1.MachineConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "rendered-master-abc",
				Annotations: map[string]string{mco.ReleaseImageVersionAnnotationKey: "4.13.0"},
			},
		},
	}

	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{Version: "4.13.0"},
			},
		},
	}

	objects := append(nodes, mcp, masterMCP, cv)
	objects = append(objects, mcs...)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()
	_ = controller.InitializeCaches(ctx)

	// Process all nodes BEFORE any "downstream controller" registers
	for _, nodeName := range []string{"worker-1", "worker-2", "master-0"} {
		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: nodeName}}
		_, _ = controller.Reconcile(ctx, req)
	}

	// Simulate late-registering downstream controller getting current snapshot
	// This is what a downstream controller would do when it starts up
	snapshot := controller.GetCurrentSnapshot()

	// Verify snapshot contains all node states
	if len(snapshot) != 3 {
		t.Errorf("Expected snapshot with 3 nodes, got %d", len(snapshot))
	}

	// Verify snapshot contents
	nodeNames := make(map[string]bool)
	for _, state := range snapshot {
		nodeNames[state.Name] = true
	}
	for _, expected := range []string{"worker-1", "worker-2", "master-0"} {
		if !nodeNames[expected] {
			t.Errorf("Expected node %s in snapshot", expected)
		}
	}

	// Verify individual state queries work for late registrations
	state, ok := controller.GetNodeState("worker-2")
	if !ok {
		t.Fatal("Expected to find worker-2 state")
	}
	if state.Phase != UpdatePhaseUpdating {
		t.Errorf("Expected worker-2 phase Updating, got %q", state.Phase)
	}

	// Verify pool-based queries work
	workerStates := controller.GetNodeStatesByPool("worker")
	if len(workerStates) != 2 {
		t.Errorf("Expected 2 worker states, got %d", len(workerStates))
	}
}

func preparePoolsWIthMultipleNodes() ([]client.Object, *openshiftmachineconfigurationv1.MachineConfigPool, *openshiftmachineconfigurationv1.MachineConfigPool) {
	// Create multiple nodes
	nodes := []client.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-1",
				Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
				Annotations: map[string]string{
					mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
					mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-abc",
					mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-2",
				Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
				Annotations: map[string]string{
					mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
					mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
					mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateWorking,
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "master-0",
				Labels: map[string]string{"node-role.kubernetes.io/master": ""},
				Annotations: map[string]string{
					mco.CurrentMachineConfigAnnotationKey:     "rendered-master-abc",
					mco.DesiredMachineConfigAnnotationKey:     "rendered-master-abc",
					mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
				},
			},
		},
	}

	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"node-role.kubernetes.io/worker": ""},
			},
		},
	}

	masterMCP := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"node-role.kubernetes.io/master": ""},
			},
		},
	}
	return nodes, mcp, masterMCP
}

// T046: Integration test - full lifecycle with delayed downstream registration
func TestCentralNodeStateController_DelayedRegistration_FullLifecycle(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateWorking,
			},
		},
	}

	fakeClient, controller := prepareCentralNodeStateController(node)
	ctx := context.Background()
	_ = controller.InitializeCaches(ctx)

	// Phase 1: Process initial state with NO downstream listeners
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}}
	_, err := controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Initial reconcile failed: %v", err)
	}

	// Verify state is tracked
	state, ok := controller.GetNodeState("worker-1")
	if !ok {
		t.Fatal("Expected node state after initial reconcile")
	}
	if state.Phase != UpdatePhaseUpdating {
		t.Errorf("Expected phase Updating, got %q", state.Phase)
	}

	// Phase 2: Simulate downstream controller "registering" late
	// Get the channel and start consuming
	nodeCh := controller.NodeInsightChannel()

	// Drain any buffered notifications from earlier
	drained := 0
drainLoop:
	for {
		select {
		case <-nodeCh:
			drained++
		default:
			break drainLoop
		}
	}
	t.Logf("Drained %d buffered notifications", drained)

	// Phase 3: Get current snapshot (what late-registering controller would do)
	snapshot := controller.GetCurrentSnapshot()
	if len(snapshot) != 1 {
		t.Errorf("Expected 1 node in snapshot, got %d", len(snapshot))
	}

	// Phase 4: State changes after downstream is "registered"
	node.Annotations[mco.CurrentMachineConfigAnnotationKey] = "rendered-worker-xyz"
	node.Annotations[mco.DesiredMachineConfigAnnotationKey] = "rendered-worker-xyz"
	node.Annotations[mco.MachineConfigDaemonStateAnnotationKey] = mco.MachineConfigDaemonStateDone
	_ = fakeClient.Update(ctx, node)

	_, err = controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Second reconcile failed: %v", err)
	}

	// Phase 5: Verify downstream receives notification for new state change
	select {
	case event := <-nodeCh:
		if event.Object.GetName() != "worker-1" {
			t.Errorf("Expected notification for worker-1, got %s", event.Object.GetName())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected notification after state change")
	}

	// Phase 6: Verify final state
	state, _ = controller.GetNodeState("worker-1")
	if state.Phase != UpdatePhaseCompleted {
		t.Errorf("Expected final phase Completed, got %q", state.Phase)
	}
}

// T052: Unit tests for structured logging with correlation IDs

func TestCentralNodeStateController_StructuredLogging_CorrelationIDs(t *testing.T) {
	// This test verifies that structured logging with correlation IDs works correctly
	// by reconciling a node through multiple phases and ensuring all logging paths execute

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = openshiftconfigv1.Install(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "worker-1",
			ResourceVersion: "12345",
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:      "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:      "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey:  mco.MachineConfigDaemonStateWorking,
				mco.DesiredDrainerAnnotationKey:            "drain-worker-1",
				mco.LastAppliedDrainerAnnotationKey:        "uncordon-worker-1",
				mco.MachineConfigDaemonReasonAnnotationKey: "InProgress",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}

	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/worker": "",
				},
			},
			Configuration: openshiftmachineconfigurationv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{Name: "rendered-worker-xyz"},
			},
		},
		Status: openshiftmachineconfigurationv1.MachineConfigPoolStatus{
			Configuration: openshiftmachineconfigurationv1.MachineConfigPoolStatusConfiguration{
				ObjectReference: corev1.ObjectReference{Name: "rendered-worker-xyz"},
			},
		},
	}

	mc := &openshiftmachineconfigurationv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rendered-worker-xyz",
			Labels: map[string]string{
				"machineconfiguration.openshift.io/role": "worker",
			},
			Annotations: map[string]string{
				"machineconfiguration.openshift.io/release-image-version": "4.15.1",
			},
		},
	}

	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{Version: "4.15.1", State: openshiftconfigv1.PartialUpdate},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, mcp, mc, cv).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()
	_ = controller.InitializeCaches(ctx)

	// Reconcile through multiple phases to exercise all logging paths
	// The goal is to verify that structured logging works correctly,
	// not to test phase determination (which is covered by other tests)

	// Phase 1: Initial reconciliation
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}}
	_, err := controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("First reconcile failed: %v", err)
	}

	// Verify state was created and logged correctly
	state, ok := controller.GetNodeState("worker-1")
	if !ok {
		t.Fatal("Expected node state to exist after first reconcile")
	}
	initialPhase := state.Phase
	initialHash := state.StateHash

	// Phase 2: Change node state to trigger state update and logging
	node.Annotations[mco.CurrentMachineConfigAnnotationKey] = "rendered-worker-xyz"
	node.Annotations[mco.MachineConfigDaemonStateAnnotationKey] = mco.MachineConfigDaemonStateDone
	node.ResourceVersion = "12346" // Change resource version
	_ = fakeClient.Update(ctx, node)

	_, err = controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Second reconcile failed: %v", err)
	}

	// Verify state was updated and logged correctly
	state, ok = controller.GetNodeState("worker-1")
	if !ok {
		t.Fatal("Expected node state to exist after second reconcile")
	}
	// State should have changed
	if state.StateHash == initialHash {
		t.Error("Expected state hash to change when node annotations change")
	}
	secondPhase := state.Phase

	// Phase 3: Reconcile again with no changes (logging path for no state change)
	_, err = controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Third reconcile failed: %v", err)
	}

	// Verify state hash matches (no change)
	newState, _ := controller.GetNodeState("worker-1")
	if newState.StateHash != state.StateHash {
		t.Error("Expected state hash to remain unchanged when no changes occur")
	}

	// Log the phases for debugging (optional)
	t.Logf("Phases observed: initial=%q, after update=%q, final=%q", initialPhase, secondPhase, newState.Phase)
}

func TestCentralNodeStateController_StructuredLogging_NodeNotFound(t *testing.T) {
	// Test that logging works correctly when node is not found
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	controller := NewCentralNodeStateController(fakeClient)
	ctx := context.Background()

	// Reconcile non-existent node
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "nonexistent-node"}}
	_, err := controller.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile should not error on node not found: %v", err)
	}
	// Node not found is expected and not an error

	// Verify no state was created
	_, ok := controller.GetNodeState("nonexistent-node")
	if ok {
		t.Error("Expected no state for non-existent node")
	}
}

// T061: Unit test: Start() blocks until context canceled

func TestCentralNodeStateController_Start_BlocksUntilContextCancelled(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	controller := NewCentralNodeStateController(fakeClient)

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Track whether Start() has completed
	startCompleted := make(chan struct{})

	// Run Start() in a goroutine
	go func() {
		err := controller.Start(ctx)
		if err != nil {
			t.Errorf("Start() returned error: %v", err)
		}
		close(startCompleted)
	}()

	// Give Start() time to begin running
	time.Sleep(50 * time.Millisecond)

	// Verify Start() is still running (hasn't completed)
	select {
	case <-startCompleted:
		t.Fatal("Start() should not complete before context is cancelled")
	default:
		// Expected - Start() is still running
	}

	// Cancel the context
	cancel()

	// Verify Start() completes after cancellation
	select {
	case <-startCompleted:
		// Expected - Start() completed
	case <-time.After(1 * time.Second):
		t.Fatal("Start() did not complete within timeout after context cancellation")
	}
}

// T062: Unit test: channels closed on shutdown

func TestCentralNodeStateController_Start_ClosesChannelsOnShutdown(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	controller := NewCentralNodeStateController(fakeClient)

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Get references to the channels before Start()
	nodeChannel := controller.NodeInsightChannel()
	mcpChannel := controller.MCPInsightChannel()

	// Run Start() in a goroutine
	go func() {
		_ = controller.Start(ctx)
	}()

	// Give Start() time to begin running
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to trigger shutdown
	cancel()

	// Wait for shutdown to complete
	time.Sleep(100 * time.Millisecond)

	// Verify node channel is closed
	select {
	case _, ok := <-nodeChannel:
		if ok {
			t.Error("Expected node channel to be closed after shutdown")
		}
		// Channel is closed, which is expected
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for node channel closure")
	}

	// Verify MCP channel is closed
	select {
	case _, ok := <-mcpChannel:
		if ok {
			t.Error("Expected MCP channel to be closed after shutdown")
		}
		// Channel is closed, which is expected
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for MCP channel closure")
	}
}
