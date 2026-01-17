package nodestate

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// T014: Unit tests for StateEvaluator interface implementation

func TestDefaultStateEvaluator_EvaluateNode_NilInputs(t *testing.T) {
	evaluator := NewDefaultStateEvaluator()
	now := time.Now()

	// Test nil node
	result := evaluator.EvaluateNode(nil, &openshiftmachineconfigurationv1.MachineConfigPool{}, noVersionLookup, "4.13.0", nil, now)
	if result != nil {
		t.Error("expected nil result for nil node")
	}

	// Test nil pool
	result = evaluator.EvaluateNode(&corev1.Node{}, nil, noVersionLookup, "4.13.0", nil, now)
	if result != nil {
		t.Error("expected nil result for nil pool")
	}

	// Test both nil
	result = evaluator.EvaluateNode(nil, nil, noVersionLookup, "4.13.0", nil, now)
	if result != nil {
		t.Error("expected nil result for both nil")
	}
}

func TestDefaultStateEvaluator_EvaluateNode_BasicFields(t *testing.T) {
	evaluator := NewDefaultStateEvaluator()
	now := time.Now()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			UID:  types.UID("test-uid-123"),
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey: "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey: "rendered-worker-xyz",
			},
		},
	}

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	versionLookup := func(mc string) (string, bool) {
		versions := map[string]string{
			"rendered-worker-abc": "4.12.0",
			"rendered-worker-xyz": "4.13.0",
		}
		v, ok := versions[mc]
		return v, ok
	}

	result := evaluator.EvaluateNode(node, pool, versionLookup, "4.13.0", nil, now)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify basic fields
	if result.Name != "worker-1" {
		t.Errorf("expected Name 'worker-1', got %q", result.Name)
	}
	if result.UID != types.UID("test-uid-123") {
		t.Errorf("expected UID 'test-uid-123', got %q", result.UID)
	}
	if result.PoolRef.Name != "worker" {
		t.Errorf("expected PoolRef.Name 'worker', got %q", result.PoolRef.Name)
	}
	if result.PoolRef.Group != openshiftmachineconfigurationv1.GroupName {
		t.Errorf("expected PoolRef.Group %q, got %q", openshiftmachineconfigurationv1.GroupName, result.PoolRef.Group)
	}
	if result.PoolRef.Resource != "machineconfigpools" {
		t.Errorf("expected PoolRef.Resource 'machineconfigpools', got %q", result.PoolRef.Resource)
	}
	if result.Scope != ouev1alpha1.WorkerPoolScope {
		t.Errorf("expected Scope %q, got %q", ouev1alpha1.WorkerPoolScope, result.Scope)
	}
	if result.Version != "4.12.0" {
		t.Errorf("expected Version '4.12.0', got %q", result.Version)
	}
	if result.DesiredVersion != "4.13.0" {
		t.Errorf("expected DesiredVersion '4.13.0', got %q", result.DesiredVersion)
	}
	if result.CurrentConfig != "rendered-worker-abc" {
		t.Errorf("expected CurrentConfig 'rendered-worker-abc', got %q", result.CurrentConfig)
	}
	if result.DesiredConfig != "rendered-worker-xyz" {
		t.Errorf("expected DesiredConfig 'rendered-worker-xyz', got %q", result.DesiredConfig)
	}
	if result.LastEvaluated != now {
		t.Errorf("expected LastEvaluated %v, got %v", now, result.LastEvaluated)
	}
	if result.StateHash == 0 {
		t.Error("expected non-zero StateHash")
	}
}

func TestDefaultStateEvaluator_EvaluateNode_MasterScope(t *testing.T) {
	evaluator := NewDefaultStateEvaluator()
	now := time.Now()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "master-0",
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey: "rendered-master-abc",
				mco.DesiredMachineConfigAnnotationKey: "rendered-master-abc",
			},
		},
	}

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: mco.MachineConfigPoolMaster,
		},
	}

	versionLookup := func(mc string) (string, bool) {
		if mc == "rendered-master-abc" {
			return "4.13.0", true
		}
		return "", false
	}

	result := evaluator.EvaluateNode(node, pool, versionLookup, "4.13.0", nil, now)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Scope != ouev1alpha1.ControlPlaneScope {
		t.Errorf("expected Scope %q for master pool, got %q", ouev1alpha1.ControlPlaneScope, result.Scope)
	}
}

func TestDefaultStateEvaluator_EvaluateNode_UpdatePhases(t *testing.T) {
	evaluator := NewDefaultStateEvaluator()
	now := time.Now()

	testCases := []struct {
		name          string
		annotations   map[string]string
		poolPaused    bool
		versionLookup func(string) (string, bool)
		desiredVer    string
		expectedPhase UpdatePhase
	}{
		{
			name: "completed - current matches desired",
			annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
			versionLookup: func(mc string) (string, bool) {
				if mc == "rendered-worker-xyz" {
					return "4.13.0", true
				}
				return "", false
			},
			desiredVer:    "4.13.0",
			expectedPhase: UpdatePhaseCompleted,
		},
		{
			name: "updating - working state",
			annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateWorking,
			},
			versionLookup: func(mc string) (string, bool) {
				versions := map[string]string{
					"rendered-worker-abc": "4.12.0",
					"rendered-worker-xyz": "4.13.0",
				}
				v, ok := versions[mc]
				return v, ok
			},
			desiredVer:    "4.13.0",
			expectedPhase: UpdatePhaseUpdating,
		},
		{
			name: "rebooting",
			annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateRebooting,
			},
			versionLookup: func(mc string) (string, bool) {
				versions := map[string]string{
					"rendered-worker-abc": "4.12.0",
					"rendered-worker-xyz": "4.13.0",
				}
				v, ok := versions[mc]
				return v, ok
			},
			desiredVer:    "4.13.0",
			expectedPhase: UpdatePhaseRebooting,
		},
		{
			name: "draining",
			annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
				mco.DesiredDrainerAnnotationKey:           "drain-worker-1",
				mco.LastAppliedDrainerAnnotationKey:       "uncordon-worker-1",
			},
			versionLookup: func(mc string) (string, bool) {
				versions := map[string]string{
					"rendered-worker-abc": "4.12.0",
					"rendered-worker-xyz": "4.13.0",
				}
				v, ok := versions[mc]
				return v, ok
			},
			desiredVer:    "4.13.0",
			expectedPhase: UpdatePhaseDraining,
		},
		{
			name: "paused",
			annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-xyz",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
			poolPaused: true,
			versionLookup: func(mc string) (string, bool) {
				versions := map[string]string{
					"rendered-worker-abc": "4.12.0",
					"rendered-worker-xyz": "4.12.0", // Same version - not updating to target
				}
				v, ok := versions[mc]
				return v, ok
			},
			desiredVer:    "4.13.0",
			expectedPhase: UpdatePhasePaused,
		},
		{
			name: "pending",
			annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
			versionLookup: func(mc string) (string, bool) {
				if mc == "rendered-worker-abc" {
					return "4.12.0", true
				}
				return "", false
			},
			desiredVer:    "4.13.0",
			expectedPhase: UpdatePhasePending,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "worker-1",
					Annotations: tc.annotations,
				},
			}

			pool := &openshiftmachineconfigurationv1.MachineConfigPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker",
				},
				Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
					Paused: tc.poolPaused,
				},
			}

			result := evaluator.EvaluateNode(node, pool, tc.versionLookup, tc.desiredVer, nil, now)

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Phase != tc.expectedPhase {
				t.Errorf("expected phase %q, got %q", tc.expectedPhase, result.Phase)
			}
		})
	}
}

func TestAssessNodeForInsight(t *testing.T) {
	now := metav1.Now()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
			Annotations: map[string]string{
				mco.CurrentMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.DesiredMachineConfigAnnotationKey:     "rendered-worker-abc",
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
		},
	}

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	versionLookup := func(mc string) (string, bool) {
		if mc == "rendered-worker-abc" {
			return "4.13.0", true
		}
		return "", false
	}

	result := AssessNodeForInsight(node, pool, versionLookup, "4.13.0", nil, now)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Name != "worker-1" {
		t.Errorf("expected Name 'worker-1', got %q", result.Name)
	}
	if result.PoolResource.Name != "worker" {
		t.Errorf("expected PoolResource.Name 'worker', got %q", result.PoolResource.Name)
	}
	if result.Scope != ouev1alpha1.WorkerPoolScope {
		t.Errorf("expected Scope %q, got %q", ouev1alpha1.WorkerPoolScope, result.Scope)
	}
	if result.Version != "4.13.0" {
		t.Errorf("expected Version '4.13.0', got %q", result.Version)
	}
}

func TestConvertStateToInsight(t *testing.T) {
	// Test nil input
	if result := ConvertStateToInsight(nil); result != nil {
		t.Error("expected nil result for nil state")
	}

	// Test valid state
	state := &NodeState{
		Name: "worker-1",
		PoolRef: ouev1alpha1.ResourceRef{
			Resource: "machineconfigpools",
			Group:    "machineconfiguration.openshift.io",
			Name:     "worker",
		},
		Scope:   ouev1alpha1.WorkerPoolScope,
		Version: "4.13.0",
		Phase:   UpdatePhaseCompleted,
		Message: "test message",
		Conditions: []metav1.Condition{
			{Type: "Updating", Status: metav1.ConditionFalse},
		},
	}

	result := ConvertStateToInsight(state)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Name != state.Name {
		t.Errorf("expected Name %q, got %q", state.Name, result.Name)
	}
	if diff := cmp.Diff(state.PoolRef, result.PoolResource); diff != "" {
		t.Errorf("PoolResource mismatch (-want +got):\n%s", diff)
	}
	if result.Scope != state.Scope {
		t.Errorf("expected Scope %q, got %q", state.Scope, result.Scope)
	}
	if result.Version != state.Version {
		t.Errorf("expected Version %q, got %q", state.Version, result.Version)
	}
	if result.Message != state.Message {
		t.Errorf("expected Message %q, got %q", state.Message, result.Message)
	}
}

func TestEstimateFromPhase(t *testing.T) {
	testCases := []struct {
		name       string
		phase      UpdatePhase
		conditions []metav1.Condition
		expectNil  bool
		expectZero bool
	}{
		{
			name:       "completed",
			phase:      UpdatePhaseCompleted,
			expectZero: true,
		},
		{
			name:      "pending",
			phase:     UpdatePhasePending,
			expectNil: true,
		},
		{
			name:      "paused",
			phase:     UpdatePhasePaused,
			expectNil: true,
		},
		{
			name:  "draining",
			phase: UpdatePhaseDraining,
		},
		{
			name:  "updating",
			phase: UpdatePhaseUpdating,
		},
		{
			name:  "rebooting",
			phase: UpdatePhaseRebooting,
		},
		{
			name:  "degraded node",
			phase: UpdatePhaseUpdating,
			conditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionTrue},
			},
			expectNil: true,
		},
		{
			name:  "unavailable node",
			phase: UpdatePhaseUpdating,
			conditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionFalse},
			},
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := EstimateFromPhase(tc.phase, tc.conditions)

			if tc.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if tc.expectZero && result.Duration != 0 {
				t.Errorf("expected zero duration, got %v", result.Duration)
			}

			if !tc.expectZero && result.Duration != 10*time.Minute {
				t.Errorf("expected 10 minute duration, got %v", result.Duration)
			}
		})
	}
}

func TestNodeStateToUID(t *testing.T) {
	if uid := NodeStateToUID(nil); uid != "" {
		t.Errorf("expected empty UID for nil state, got %q", uid)
	}

	state := &NodeState{
		UID: types.UID("test-uid-123"),
	}
	if uid := NodeStateToUID(state); uid != types.UID("test-uid-123") {
		t.Errorf("expected UID 'test-uid-123', got %q", uid)
	}
}

// Helper function for tests that don't need version lookup
func noVersionLookup(mc string) (string, bool) {
	return "", false
}
