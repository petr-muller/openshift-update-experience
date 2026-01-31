/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package machineconfigpools

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestReconcile_CreatesMasterInsightWithCorrectScope(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a master MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "master",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create mock provider with empty pool
	mockProvider := NewMockNodeStateProvider()
	mockProvider.SetPoolNodes("master", []*nodestate.NodeState{})

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "master"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the MachineConfigPoolProgressInsight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "master"}, fetchedInsight); err != nil {
		t.Fatalf("Expected MachineConfigPoolProgressInsight to be created, got error: %v", err)
	}

	// Verify the status has correct Name and Scope (with empty pool)
	if fetchedInsight.Status.Name != "master" {
		t.Errorf("Expected name 'master', got '%s'", fetchedInsight.Status.Name)
	}
	if fetchedInsight.Status.Scope != ouev1alpha1.ControlPlaneScope {
		t.Errorf("Expected scope ControlPlaneScope, got %s", fetchedInsight.Status.Scope)
	}
	if fetchedInsight.Status.Assessment != ouev1alpha1.PoolPending {
		t.Errorf("Expected assessment Pending for empty pool, got %s", fetchedInsight.Status.Assessment)
	}
	if fetchedInsight.Status.Completion != 0 {
		t.Errorf("Expected completion 0%% for empty pool, got %d%%", fetchedInsight.Status.Completion)
	}
}

func TestReconcile_CreatesWorkerInsightWithCorrectScope(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a worker MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create mock provider with empty pool
	mockProvider := NewMockNodeStateProvider()
	mockProvider.SetPoolNodes("worker", []*nodestate.NodeState{})

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the MachineConfigPoolProgressInsight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker"}, fetchedInsight); err != nil {
		t.Fatalf("Expected MachineConfigPoolProgressInsight to be created, got error: %v", err)
	}

	// Verify the status has correct Name and Scope (with empty pool)
	if fetchedInsight.Status.Name != "worker" {
		t.Errorf("Expected name 'worker', got '%s'", fetchedInsight.Status.Name)
	}
	if fetchedInsight.Status.Scope != ouev1alpha1.WorkerPoolScope {
		t.Errorf("Expected scope WorkerPoolScope, got %s", fetchedInsight.Status.Scope)
	}
	if fetchedInsight.Status.Assessment != ouev1alpha1.PoolPending {
		t.Errorf("Expected assessment Pending for empty pool, got %s", fetchedInsight.Status.Assessment)
	}
}

func TestReconcile_CreatesCustomPoolInsightWithWorkerScope(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a custom MachineConfigPool (not master)
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-pool",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create mock provider with empty pool
	mockProvider := NewMockNodeStateProvider()
	mockProvider.SetPoolNodes("custom-pool", []*nodestate.NodeState{})

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "custom-pool"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the MachineConfigPoolProgressInsight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "custom-pool"}, fetchedInsight); err != nil {
		t.Fatalf("Expected MachineConfigPoolProgressInsight to be created, got error: %v", err)
	}

	// Verify the status has correct Name and Scope (with empty pool)
	if fetchedInsight.Status.Name != "custom-pool" {
		t.Errorf("Expected name 'custom-pool', got '%s'", fetchedInsight.Status.Name)
	}
	if fetchedInsight.Status.Scope != ouev1alpha1.WorkerPoolScope {
		t.Errorf("Expected scope WorkerPoolScope, got %s", fetchedInsight.Status.Scope)
	}
}

func TestReconcile_BothMCPAndInsightDontExist_DoesNothing(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a fake client with no objects
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create mock provider
	mockProvider := NewMockNodeStateProvider()

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify no MachineConfigPoolProgressInsight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: "nonexistent"}, fetchedInsight)
	if !errors.IsNotFound(getErr) {
		t.Errorf("Expected no MachineConfigPoolProgressInsight to exist, but found one or got error: %v", getErr)
	}
}

func TestReconcile_MCPDoesNotExistButInsightDoes_DeletesInsight(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create an existing insight without a corresponding MCP
	insight := &ouev1alpha1.MachineConfigPoolProgressInsight{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deleted-pool",
		},
		Status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
			Name:  "deleted-pool",
			Scope: ouev1alpha1.WorkerPoolScope,
		},
	}

	// Create a fake client with just the insight (no MCP)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(insight).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create mock provider
	mockProvider := NewMockNodeStateProvider()

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "deleted-pool"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the insight was deleted
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: "deleted-pool"}, fetchedInsight)
	if !errors.IsNotFound(getErr) {
		t.Errorf("Expected insight to be deleted, but found one or got error: %v", getErr)
	}
}

func TestReconcile_InsightExistsButStatusUnchanged_SkipsUpdate(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	// Create mock provider with empty pool (matches expected status)
	mockProvider := NewMockNodeStateProvider()
	mockProvider.SetPoolNodes("worker", []*nodestate.NodeState{})

	// Get expected status from assessment for empty pool
	expectedStatusPtr := AssessPoolFromNodeStates("worker", ouev1alpha1.WorkerPoolScope, false, []*nodestate.NodeState{}, nil)

	// Create an existing insight with correct status
	insight := &ouev1alpha1.MachineConfigPoolProgressInsight{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Status: *expectedStatusPtr,
	}

	// Create a fake client with both MCP and insight
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp, insight).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the insight still exists with unchanged status
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker"}, fetchedInsight); err != nil {
		t.Fatalf("Expected insight to exist, got error: %v", err)
	}

	// Status should match what we set initially (from AssessPoolFromNodeStates)
	// Ignore LastTransitionTime since it will differ between the two AssessPoolFromNodeStates calls
	ignoreTimestamps := cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime")
	if diff := cmp.Diff(*expectedStatusPtr, fetchedInsight.Status, ignoreTimestamps); diff != "" {
		t.Errorf("MachineConfigPoolProgressInsight status should be unchanged (-want +got):\n%s", diff)
	}
}

// MockNodeStateProvider implements the nodestate.Provider interface for testing.
type MockNodeStateProvider struct {
	poolNodes   map[string][]*nodestate.NodeState // poolName → nodes
	nodeChannel chan event.GenericEvent
	mcpChannel  chan event.GenericEvent
}

// NewMockNodeStateProvider creates a new mock provider for testing.
func NewMockNodeStateProvider() *MockNodeStateProvider {
	return &MockNodeStateProvider{
		poolNodes:   make(map[string][]*nodestate.NodeState),
		nodeChannel: make(chan event.GenericEvent, 10),
		mcpChannel:  make(chan event.GenericEvent, 10),
	}
}

// SetPoolNodes sets the nodes for a specific pool.
func (m *MockNodeStateProvider) SetPoolNodes(poolName string, nodes []*nodestate.NodeState) {
	m.poolNodes[poolName] = nodes
}

// GetNodeState implements Provider.GetNodeState.
func (m *MockNodeStateProvider) GetNodeState(nodeName string) (*nodestate.NodeState, bool) {
	for _, nodes := range m.poolNodes {
		for _, node := range nodes {
			if node.Name == nodeName {
				return node, true
			}
		}
	}
	return nil, false
}

// GetAllNodeStates implements Provider.GetAllNodeStates.
func (m *MockNodeStateProvider) GetAllNodeStates() []*nodestate.NodeState {
	var all []*nodestate.NodeState
	for _, nodes := range m.poolNodes {
		all = append(all, nodes...)
	}
	return all
}

// GetNodeStatesByPool implements Provider.GetNodeStatesByPool.
func (m *MockNodeStateProvider) GetNodeStatesByPool(poolName string) []*nodestate.NodeState {
	return m.poolNodes[poolName]
}

// NodeInsightChannel implements Provider.NodeInsightChannel.
func (m *MockNodeStateProvider) NodeInsightChannel() <-chan event.GenericEvent {
	return m.nodeChannel
}

// MCPInsightChannel implements Provider.MCPInsightChannel.
func (m *MockNodeStateProvider) MCPInsightChannel() <-chan event.GenericEvent {
	return m.mcpChannel
}

// Integration tests with node states

func TestReconcile_WithNodeStates_GeneratesCompleteStatus(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a worker MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create mock provider with mixed node states
	mockProvider := NewMockNodeStateProvider()
	mockProvider.SetPoolNodes("worker", []*nodestate.NodeState{
		{
			Name:           "node1",
			Version:        "4.18.0",
			DesiredVersion: "4.18.0",
			Phase:          nodestate.UpdatePhaseCompleted,
			Conditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse},
				{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionTrue},
				{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
			},
		},
		{
			Name:           "node2",
			Version:        "4.17.0",
			DesiredVersion: "4.18.0",
			Phase:          nodestate.UpdatePhaseUpdating,
			Conditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionTrue},
				{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionFalse},
				{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
			},
		},
	})

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	}
	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}

	// Verify the insight was created with complete status
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker"}, fetchedInsight); err != nil {
		t.Fatalf("Expected insight to be created, got error: %v", err)
	}

	// Verify complete status fields
	if fetchedInsight.Status.Name != "worker" {
		t.Errorf("Expected name 'worker', got '%s'", fetchedInsight.Status.Name)
	}
	if fetchedInsight.Status.Scope != ouev1alpha1.WorkerPoolScope {
		t.Errorf("Expected scope WorkerPoolScope, got %s", fetchedInsight.Status.Scope)
	}
	if fetchedInsight.Status.Assessment != ouev1alpha1.PoolProgressing {
		t.Errorf("Expected assessment Progressing, got %s", fetchedInsight.Status.Assessment)
	}
	if fetchedInsight.Status.Completion != 50 { // 1 of 2 completed
		t.Errorf("Expected completion 50%%, got %d%%", fetchedInsight.Status.Completion)
	}
	if len(fetchedInsight.Status.Summaries) != 7 {
		t.Errorf("Expected 7 summaries, got %d", len(fetchedInsight.Status.Summaries))
	}
	if len(fetchedInsight.Status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(fetchedInsight.Status.Conditions))
	}
}

func TestReconcile_EmptyPool_ZeroCompletion(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a worker MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create mock provider with empty pool
	mockProvider := NewMockNodeStateProvider()
	mockProvider.SetPoolNodes("worker", []*nodestate.NodeState{})

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	}
	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}

	// Verify the insight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker"}, fetchedInsight); err != nil {
		t.Fatalf("Expected insight to be created, got error: %v", err)
	}

	// Verify zero completion for empty pool
	if fetchedInsight.Status.Completion != 0 {
		t.Errorf("Expected completion 0%% for empty pool, got %d%%", fetchedInsight.Status.Completion)
	}
	if fetchedInsight.Status.Assessment != ouev1alpha1.PoolPending {
		t.Errorf("Expected assessment Pending for empty pool, got %s", fetchedInsight.Status.Assessment)
	}
}

func TestReconcile_DegradedNode_PoolDegraded(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a worker MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create mock provider with degraded node
	mockProvider := NewMockNodeStateProvider()
	mockProvider.SetPoolNodes("worker", []*nodestate.NodeState{
		{
			Name:           "node1",
			Version:        "4.17.0",
			DesiredVersion: "4.18.0",
			Phase:          nodestate.UpdatePhaseUpdating,
			Conditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionTrue},
				{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionFalse},
				{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionTrue},
			},
		},
	})

	// Create the reconciler
	reconciler := &Reconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		stateProvider: mockProvider,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	}
	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}

	// Verify the insight shows degraded pool
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker"}, fetchedInsight); err != nil {
		t.Fatalf("Expected insight to be created, got error: %v", err)
	}

	if fetchedInsight.Status.Assessment != ouev1alpha1.PoolDegraded {
		t.Errorf("Expected assessment Degraded, got %s", fetchedInsight.Status.Assessment)
	}

	// Find Healthy condition
	healthyFound := false
	for _, cond := range fetchedInsight.Status.Conditions {
		if cond.Type == string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy) {
			healthyFound = true
			if cond.Status != metav1.ConditionFalse {
				t.Errorf("Expected Healthy=False, got %s", cond.Status)
			}
		}
	}
	if !healthyFound {
		t.Error("Healthy condition not found")
	}
}
