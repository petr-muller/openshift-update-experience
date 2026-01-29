package nodestate

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

// T007: Unit tests for NodeState struct and UpdatePhase constants

func TestUpdatePhaseConstants(t *testing.T) {
	testCases := []struct {
		phase    UpdatePhase
		expected string
	}{
		{UpdatePhasePending, "Pending"},
		{UpdatePhaseDraining, "Draining"},
		{UpdatePhaseUpdating, "Updating"},
		{UpdatePhaseRebooting, "Rebooting"},
		{UpdatePhasePaused, "Paused"},
		{UpdatePhaseCompleted, "Completed"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			if string(tc.phase) != tc.expected {
				t.Errorf("expected phase %q, got %q", tc.expected, string(tc.phase))
			}
		})
	}
}

func TestNodeState_ComputeHash(t *testing.T) {
	baseState := &NodeState{
		Name: "node-1",
		UID:  types.UID("uid-1"),
		PoolRef: ouev1alpha1.ResourceRef{
			Name:     "worker",
			Group:    "machineconfiguration.openshift.io",
			Resource: "machineconfigpools",
		},
		Scope:          ouev1alpha1.WorkerPoolScope,
		Version:        "4.12.0",
		DesiredVersion: "4.13.0",
		CurrentConfig:  "rendered-worker-abc",
		DesiredConfig:  "rendered-worker-xyz",
		Phase:          UpdatePhaseUpdating,
		Message:        "Updating node",
		Conditions: []metav1.Condition{
			{Type: "Updating", Status: metav1.ConditionTrue, Reason: "NodeUpdating", Message: "Node is updating"},
		},
		LastEvaluated:    time.Now(),
		EvaluationCount:  5,
		SourceGeneration: 10,
	}

	hash1 := baseState.ComputeHash()
	if hash1 == 0 {
		t.Error("expected non-zero hash")
	}

	// Same state should produce same hash
	hash2 := baseState.ComputeHash()
	if hash1 != hash2 {
		t.Errorf("expected same hash for unchanged state, got %d and %d", hash1, hash2)
	}

	// Internal tracking fields should NOT affect hash
	modifiedState := *baseState
	modifiedState.LastEvaluated = time.Now().Add(time.Hour)
	modifiedState.EvaluationCount = 100
	modifiedState.SourceGeneration = 200
	hash3 := modifiedState.ComputeHash()
	if hash1 != hash3 {
		t.Errorf("expected same hash when only internal tracking fields change, got %d and %d", hash1, hash3)
	}

	// Changing external fields SHOULD affect hash
	testCases := []struct {
		name   string
		modify func(*NodeState)
	}{
		{"Name", func(s *NodeState) { s.Name = "node-2" }},
		{"PoolRef.Name", func(s *NodeState) { s.PoolRef.Name = "master" }},
		{"PoolRef.Group", func(s *NodeState) { s.PoolRef.Group = "other.group" }},
		{"PoolRef.Resource", func(s *NodeState) { s.PoolRef.Resource = "other" }},
		{"Scope", func(s *NodeState) { s.Scope = ouev1alpha1.ControlPlaneScope }},
		{"Version", func(s *NodeState) { s.Version = "4.14.0" }},
		{"DesiredVersion", func(s *NodeState) { s.DesiredVersion = "4.14.0" }},
		{"CurrentConfig", func(s *NodeState) { s.CurrentConfig = "other-config" }},
		{"DesiredConfig", func(s *NodeState) { s.DesiredConfig = "other-config" }},
		{"Phase", func(s *NodeState) { s.Phase = UpdatePhaseCompleted }},
		{"Message", func(s *NodeState) { s.Message = "Different message" }},
		{"Condition.Type", func(s *NodeState) { s.Conditions[0].Type = "Different" }},
		{"Condition.Status", func(s *NodeState) { s.Conditions[0].Status = metav1.ConditionFalse }},
		{"Condition.Reason", func(s *NodeState) { s.Conditions[0].Reason = "DifferentReason" }},
		{"Condition.Message", func(s *NodeState) { s.Conditions[0].Message = "Different condition message" }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			modified := *baseState
			modified.Conditions = make([]metav1.Condition, len(baseState.Conditions))
			copy(modified.Conditions, baseState.Conditions)
			tc.modify(&modified)
			modifiedHash := modified.ComputeHash()
			if modifiedHash == hash1 {
				t.Errorf("expected different hash when %s changes", tc.name)
			}
		})
	}
}

func TestNodeState_ComputeHash_EmptyConditions(t *testing.T) {
	state1 := &NodeState{
		Name:       "node-1",
		Phase:      UpdatePhasePending,
		Conditions: nil,
	}
	state2 := &NodeState{
		Name:       "node-1",
		Phase:      UpdatePhasePending,
		Conditions: []metav1.Condition{},
	}

	hash1 := state1.ComputeHash()
	hash2 := state2.ComputeHash()
	if hash1 != hash2 {
		t.Errorf("expected same hash for nil and empty conditions, got %d and %d", hash1, hash2)
	}
}

// T008: Unit tests for NodeStateStore

func TestNodeStateStore_GetSetDelete(t *testing.T) {
	store := &Store{}

	// Get on empty store returns false
	state, ok := store.Get("node-1")
	if ok {
		t.Error("expected Get to return false for non-existent node")
	}
	if state != nil {
		t.Error("expected Get to return nil for non-existent node")
	}

	// Set and Get
	nodeState := &NodeState{
		Name:    "node-1",
		UID:     types.UID("uid-1"),
		Phase:   UpdatePhasePending,
		Version: "4.12.0",
	}
	store.Set("node-1", nodeState)

	retrieved, ok := store.Get("node-1")
	if !ok {
		t.Error("expected Get to return true for existing node")
	}
	if diff := cmp.Diff(nodeState, retrieved); diff != "" {
		t.Errorf("retrieved state differs from stored (-want +got):\n%s", diff)
	}

	// Delete existing node
	deleted := store.Delete("node-1")
	if !deleted {
		t.Error("expected Delete to return true for existing node")
	}

	// Verify deletion
	_, ok = store.Get("node-1")
	if ok {
		t.Error("expected Get to return false after Delete")
	}

	// Delete non-existent node
	deleted = store.Delete("node-1")
	if deleted {
		t.Error("expected Delete to return false for non-existent node")
	}
}

func TestNodeStateStore_Range(t *testing.T) {
	store := &Store{}

	// Set up test data
	nodes := []*NodeState{
		{Name: "node-1", Phase: UpdatePhasePending},
		{Name: "node-2", Phase: UpdatePhaseUpdating},
		{Name: "node-3", Phase: UpdatePhaseCompleted},
	}
	for _, n := range nodes {
		store.Set(n.Name, n)
	}

	// Range over all
	visited := make(map[string]bool)
	store.Range(func(name string, state *NodeState) bool {
		visited[name] = true
		return true
	})

	if len(visited) != 3 {
		t.Errorf("expected to visit 3 nodes, visited %d", len(visited))
	}
	for _, n := range nodes {
		if !visited[n.Name] {
			t.Errorf("expected to visit node %s", n.Name)
		}
	}

	// Range with early termination
	visitCount := 0
	store.Range(func(name string, state *NodeState) bool {
		visitCount++
		return false // Stop after first
	})
	if visitCount != 1 {
		t.Errorf("expected Range to stop after first iteration, visited %d", visitCount)
	}
}

func TestNodeStateStore_Count(t *testing.T) {
	store := &Store{}

	// Empty store
	if count := store.Count(); count != 0 {
		t.Errorf("expected count 0 for empty store, got %d", count)
	}

	// Add nodes
	store.Set("node-1", &NodeState{Name: "node-1"})
	if count := store.Count(); count != 1 {
		t.Errorf("expected count 1 after adding one node, got %d", count)
	}

	store.Set("node-2", &NodeState{Name: "node-2"})
	store.Set("node-3", &NodeState{Name: "node-3"})
	if count := store.Count(); count != 3 {
		t.Errorf("expected count 3 after adding three nodes, got %d", count)
	}

	// Delete node
	store.Delete("node-2")
	if count := store.Count(); count != 2 {
		t.Errorf("expected count 2 after deleting one node, got %d", count)
	}
}

func TestNodeStateStore_GetAll(t *testing.T) {
	store := &Store{}

	// Empty store returns empty slice
	all := store.GetAll()
	if len(all) != 0 {
		t.Errorf("expected empty slice for empty store, got %d elements", len(all))
	}

	// Add nodes
	nodes := []*NodeState{
		{Name: "node-1", Phase: UpdatePhasePending},
		{Name: "node-2", Phase: UpdatePhaseUpdating},
		{Name: "node-3", Phase: UpdatePhaseCompleted},
	}
	for _, n := range nodes {
		store.Set(n.Name, n)
	}

	all = store.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(all))
	}

	// Verify all nodes are present (order not guaranteed)
	foundNames := make(map[string]bool)
	for _, s := range all {
		foundNames[s.Name] = true
	}
	for _, n := range nodes {
		if !foundNames[n.Name] {
			t.Errorf("expected to find node %s in GetAll result", n.Name)
		}
	}
}

func TestNodeStateStore_GetByPool(t *testing.T) {
	store := &Store{}

	// Set up test data with different pools
	nodes := []*NodeState{
		{Name: "master-0", PoolRef: ouev1alpha1.ResourceRef{Name: "master"}, Phase: UpdatePhasePending},
		{Name: "master-1", PoolRef: ouev1alpha1.ResourceRef{Name: "master"}, Phase: UpdatePhaseUpdating},
		{Name: "worker-0", PoolRef: ouev1alpha1.ResourceRef{Name: "worker"}, Phase: UpdatePhaseCompleted},
		{Name: "worker-1", PoolRef: ouev1alpha1.ResourceRef{Name: "worker"}, Phase: UpdatePhasePending},
		{Name: "worker-2", PoolRef: ouev1alpha1.ResourceRef{Name: "worker"}, Phase: UpdatePhaseDraining},
		{Name: "custom-0", PoolRef: ouev1alpha1.ResourceRef{Name: "custom"}, Phase: UpdatePhaseRebooting},
	}
	for _, n := range nodes {
		store.Set(n.Name, n)
	}

	// Get master pool nodes
	masterNodes := store.GetByPool("master")
	if len(masterNodes) != 2 {
		t.Errorf("expected 2 master nodes, got %d", len(masterNodes))
	}
	for _, n := range masterNodes {
		if n.PoolRef.Name != "master" {
			t.Errorf("expected master pool node, got pool %s", n.PoolRef.Name)
		}
	}

	// Get worker pool nodes
	workerNodes := store.GetByPool("worker")
	if len(workerNodes) != 3 {
		t.Errorf("expected 3 worker nodes, got %d", len(workerNodes))
	}
	for _, n := range workerNodes {
		if n.PoolRef.Name != "worker" {
			t.Errorf("expected worker pool node, got pool %s", n.PoolRef.Name)
		}
	}

	// Get custom pool nodes
	customNodes := store.GetByPool("custom")
	if len(customNodes) != 1 {
		t.Errorf("expected 1 custom node, got %d", len(customNodes))
	}

	// Get non-existent pool
	nonExistent := store.GetByPool("nonexistent")
	if len(nonExistent) != 0 {
		t.Errorf("expected 0 nodes for non-existent pool, got %d", len(nonExistent))
	}
}

func TestNodeStateStore_UpdateExisting(t *testing.T) {
	store := &Store{}

	// Initial state
	initial := &NodeState{
		Name:    "node-1",
		Phase:   UpdatePhasePending,
		Version: "4.12.0",
	}
	store.Set("node-1", initial)

	// Update with new state
	updated := &NodeState{
		Name:    "node-1",
		Phase:   UpdatePhaseUpdating,
		Version: "4.12.0",
	}
	store.Set("node-1", updated)

	// Verify update
	retrieved, ok := store.Get("node-1")
	if !ok {
		t.Error("expected Get to return true after update")
	}

	if retrieved == nil {
		t.Fatalf("expected updated phase %s, got retrieved=nil", UpdatePhaseUpdating)
	}

	if retrieved.Phase != UpdatePhaseUpdating {
		t.Errorf("expected updated phase %s, got %s", UpdatePhaseUpdating, retrieved.Phase)
	}

	// Count should still be 1
	if count := store.Count(); count != 1 {
		t.Errorf("expected count 1 after update, got %d", count)
	}
}
