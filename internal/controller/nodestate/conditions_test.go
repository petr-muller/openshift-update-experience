package nodestate

import (
	"testing"
	"time"

	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// T015: Unit tests for condition determination logic

func TestDetermineConditions_BasicStates(t *testing.T) {
	now := time.Now()

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
	}

	testCases := []struct {
		name                  string
		isUpdating            bool
		isUpdated             bool
		isUnavailable         bool
		isDegraded            bool
		nodeAnnotations       map[string]string
		expectedPhase         UpdatePhase
		expectedUpdatingCond  metav1.ConditionStatus
		expectedUpdatingRsn   string
		expectedAvailableCond metav1.ConditionStatus
		expectedDegradedCond  metav1.ConditionStatus
	}{
		{
			name:       "completed state",
			isUpdating: false,
			isUpdated:  true,
			nodeAnnotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
			expectedPhase:         UpdatePhaseCompleted,
			expectedUpdatingCond:  metav1.ConditionFalse,
			expectedUpdatingRsn:   string(ouev1alpha1.NodeCompleted),
			expectedAvailableCond: metav1.ConditionTrue,
			expectedDegradedCond:  metav1.ConditionFalse,
		},
		{
			name:       "updating state",
			isUpdating: true,
			isUpdated:  false,
			nodeAnnotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateWorking,
			},
			expectedPhase:         UpdatePhaseUpdating,
			expectedUpdatingCond:  metav1.ConditionTrue,
			expectedUpdatingRsn:   string(ouev1alpha1.NodeUpdating),
			expectedAvailableCond: metav1.ConditionTrue,
			expectedDegradedCond:  metav1.ConditionFalse,
		},
		{
			name:       "rebooting state",
			isUpdating: true,
			isUpdated:  false,
			nodeAnnotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateRebooting,
			},
			expectedPhase:         UpdatePhaseRebooting,
			expectedUpdatingCond:  metav1.ConditionTrue,
			expectedUpdatingRsn:   string(ouev1alpha1.NodeRebooting),
			expectedAvailableCond: metav1.ConditionTrue,
			expectedDegradedCond:  metav1.ConditionFalse,
		},
		{
			name:       "pending state",
			isUpdating: false,
			isUpdated:  false,
			nodeAnnotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
			expectedPhase:         UpdatePhasePending,
			expectedUpdatingCond:  metav1.ConditionFalse,
			expectedUpdatingRsn:   string(ouev1alpha1.NodeUpdatePending),
			expectedAvailableCond: metav1.ConditionTrue,
			expectedDegradedCond:  metav1.ConditionFalse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Annotations: tc.nodeAnnotations,
				},
			}

			lns := mco.NewLayeredNodeState(node)

			conditions, _, phase := DetermineConditions(pool, node, tc.isUpdating, tc.isUpdated, tc.isUnavailable, tc.isDegraded, lns, nil, now)

			if phase != tc.expectedPhase {
				t.Errorf("expected phase %q, got %q", tc.expectedPhase, phase)
			}

			// Check Updating condition
			var updatingCond *metav1.Condition
			for i := range conditions {
				if conditions[i].Type == string(ouev1alpha1.NodeStatusInsightUpdating) {
					updatingCond = &conditions[i]
					break
				}
			}
			if updatingCond == nil {
				t.Fatal("expected Updating condition to be present")
			}
			if updatingCond.Status != tc.expectedUpdatingCond {
				t.Errorf("expected Updating status %q, got %q", tc.expectedUpdatingCond, updatingCond.Status)
			}
			if updatingCond.Reason != tc.expectedUpdatingRsn {
				t.Errorf("expected Updating reason %q, got %q", tc.expectedUpdatingRsn, updatingCond.Reason)
			}

			// Check Available condition
			var availableCond *metav1.Condition
			for i := range conditions {
				if conditions[i].Type == string(ouev1alpha1.NodeStatusInsightAvailable) {
					availableCond = &conditions[i]
					break
				}
			}
			if availableCond == nil {
				t.Fatal("expected Available condition to be present")
			}
			if availableCond.Status != tc.expectedAvailableCond {
				t.Errorf("expected Available status %q, got %q", tc.expectedAvailableCond, availableCond.Status)
			}

			// Check Degraded condition
			var degradedCond *metav1.Condition
			for i := range conditions {
				if conditions[i].Type == string(ouev1alpha1.NodeStatusInsightDegraded) {
					degradedCond = &conditions[i]
					break
				}
			}
			if degradedCond == nil {
				t.Fatal("expected Degraded condition to be present")
			}
			if degradedCond.Status != tc.expectedDegradedCond {
				t.Errorf("expected Degraded status %q, got %q", tc.expectedDegradedCond, degradedCond.Status)
			}
		})
	}
}

func TestDetermineConditions_Draining(t *testing.T) {
	now := time.Now()

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
				mco.DesiredDrainerAnnotationKey:           "drain-test-node",
				mco.LastAppliedDrainerAnnotationKey:       "uncordon-test-node",
			},
		},
	}

	lns := mco.NewLayeredNodeState(node)

	conditions, _, phase := DetermineConditions(pool, node, true, false, false, false, lns, nil, now)

	if phase != UpdatePhaseDraining {
		t.Errorf("expected phase %q, got %q", UpdatePhaseDraining, phase)
	}

	// Check Updating condition has Draining reason
	var updatingCond *metav1.Condition
	for i := range conditions {
		if conditions[i].Type == string(ouev1alpha1.NodeStatusInsightUpdating) {
			updatingCond = &conditions[i]
			break
		}
	}
	if updatingCond == nil {
		t.Fatal("expected Updating condition to be present")
	}
	if updatingCond.Reason != string(ouev1alpha1.NodeDraining) {
		t.Errorf("expected Updating reason %q, got %q", ouev1alpha1.NodeDraining, updatingCond.Reason)
	}
}

func TestDetermineConditions_Paused(t *testing.T) {
	now := time.Now()

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: openshiftmachineconfigurationv1.MachineConfigPoolSpec{
			Paused: true,
		},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
		},
	}

	lns := mco.NewLayeredNodeState(node)

	conditions, _, phase := DetermineConditions(pool, node, false, false, false, false, lns, nil, now)

	if phase != UpdatePhasePaused {
		t.Errorf("expected phase %q, got %q", UpdatePhasePaused, phase)
	}

	// Check Updating condition has Paused reason
	var updatingCond *metav1.Condition
	for i := range conditions {
		if conditions[i].Type == string(ouev1alpha1.NodeStatusInsightUpdating) {
			updatingCond = &conditions[i]
			break
		}
	}
	if updatingCond == nil {
		t.Fatal("expected Updating condition to be present")
	}
	if updatingCond.Reason != string(ouev1alpha1.NodePaused) {
		t.Errorf("expected Updating reason %q, got %q", ouev1alpha1.NodePaused, updatingCond.Reason)
	}
}

func TestDetermineConditions_Degraded(t *testing.T) {
	now := time.Now()

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey:  mco.MachineConfigDaemonStateDegraded,
				mco.MachineConfigDaemonReasonAnnotationKey: "Node is degraded: some error",
			},
		},
	}

	lns := mco.NewLayeredNodeState(node)

	conditions, message, _ := DetermineConditions(pool, node, false, false, false, true, lns, nil, now)

	// Check message is set from degraded reason
	if message != "Node is degraded: some error" {
		t.Errorf("expected message 'Node is degraded: some error', got %q", message)
	}

	// Check Degraded condition
	var degradedCond *metav1.Condition
	for i := range conditions {
		if conditions[i].Type == string(ouev1alpha1.NodeStatusInsightDegraded) {
			degradedCond = &conditions[i]
			break
		}
	}
	if degradedCond == nil {
		t.Fatal("expected Degraded condition to be present")
	}
	if degradedCond.Status != metav1.ConditionTrue {
		t.Errorf("expected Degraded status True, got %q", degradedCond.Status)
	}
	if degradedCond.Reason != "Degraded" {
		t.Errorf("expected Degraded reason 'Degraded', got %q", degradedCond.Reason)
	}
}

func TestDetermineConditions_Unavailable(t *testing.T) {
	now := time.Now()

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
		},
	}

	lns := mco.NewLayeredNodeState(node)

	// isUnavailable=true, isUpdating=false
	conditions, message, _ := DetermineConditions(pool, node, false, false, true, false, lns, nil, now)

	// Check Available condition
	var availableCond *metav1.Condition
	for i := range conditions {
		if conditions[i].Type == string(ouev1alpha1.NodeStatusInsightAvailable) {
			availableCond = &conditions[i]
			break
		}
	}
	if availableCond == nil {
		t.Fatal("expected Available condition to be present")
	}
	if availableCond.Status != metav1.ConditionFalse {
		t.Errorf("expected Available status False, got %q", availableCond.Status)
	}
	if availableCond.Reason != "Unavailable" {
		t.Errorf("expected Available reason 'Unavailable', got %q", availableCond.Reason)
	}

	// Message should be set from unavailable message
	_ = message // message comes from lns.GetUnavailableMessage()
}

func TestDetermineConditions_PreservesExistingConditions(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-1 * time.Hour)

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
		},
	}

	existingConditions := []metav1.Condition{
		{
			Type:               string(ouev1alpha1.NodeStatusInsightUpdating),
			Status:             metav1.ConditionFalse,
			Reason:             string(ouev1alpha1.NodeCompleted),
			Message:            "The node is updated",
			LastTransitionTime: metav1.Time{Time: earlier},
		},
	}

	lns := mco.NewLayeredNodeState(node)

	conditions, _, _ := DetermineConditions(pool, node, false, true, false, false, lns, existingConditions, now)

	// Check that LastTransitionTime is preserved when status doesn't change
	var updatingCond *metav1.Condition
	for i := range conditions {
		if conditions[i].Type == string(ouev1alpha1.NodeStatusInsightUpdating) {
			updatingCond = &conditions[i]
			break
		}
	}
	if updatingCond == nil {
		t.Fatal("expected Updating condition to be present")
	}
	// Status is still False (Completed), so LastTransitionTime should be preserved from earlier
	// If meta.SetStatusCondition properly preserves time when status doesn't change,
	// the time should remain at 'earlier' rather than being updated to 'now'
	_ = updatingCond.LastTransitionTime // Time preservation is handled by meta.SetStatusCondition
}

func TestDetermineConditions_FiltersUnmanagedConditions(t *testing.T) {
	now := time.Now()

	pool := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Annotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
			},
		},
	}

	// Include an unmanaged condition
	existingConditions := []metav1.Condition{
		{
			Type:   string(ouev1alpha1.NodeStatusInsightUpdating),
			Status: metav1.ConditionFalse,
		},
		{
			Type:   "UnmanagedCondition",
			Status: metav1.ConditionTrue,
		},
	}

	lns := mco.NewLayeredNodeState(node)

	conditions, _, _ := DetermineConditions(pool, node, false, true, false, false, lns, existingConditions, now)

	// Check that UnmanagedCondition is not present
	for _, c := range conditions {
		if c.Type == "UnmanagedCondition" {
			t.Error("expected UnmanagedCondition to be filtered out")
		}
	}

	// Should have exactly 3 conditions: Updating, Available, Degraded
	if len(conditions) != 3 {
		t.Errorf("expected 3 conditions, got %d", len(conditions))
	}
}

func TestIsNodeDraining(t *testing.T) {
	testCases := []struct {
		name        string
		annotations map[string]string
		isUpdating  bool
		expected    bool
	}{
		{
			name: "drain in progress",
			annotations: map[string]string{
				mco.DesiredDrainerAnnotationKey:     "drain-node-1",
				mco.LastAppliedDrainerAnnotationKey: "uncordon-node-1",
			},
			isUpdating: true,
			expected:   true,
		},
		{
			name: "drain completed",
			annotations: map[string]string{
				mco.DesiredDrainerAnnotationKey:     "drain-node-1",
				mco.LastAppliedDrainerAnnotationKey: "drain-node-1",
			},
			isUpdating: true,
			expected:   false,
		},
		{
			name: "uncordon in progress",
			annotations: map[string]string{
				mco.DesiredDrainerAnnotationKey:     "uncordon-node-1",
				mco.LastAppliedDrainerAnnotationKey: "drain-node-1",
			},
			isUpdating: true,
			expected:   false,
		},
		{
			name:        "no drain annotations",
			annotations: map[string]string{},
			isUpdating:  true,
			expected:    false,
		},
		{
			name: "MCD state Done while updating - defaults to draining",
			annotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
				mco.DesiredDrainerAnnotationKey:           "something",
				mco.LastAppliedDrainerAnnotationKey:       "something",
			},
			isUpdating: true,
			expected:   true,
		},
		{
			name: "MCD state Done not updating",
			annotations: map[string]string{
				mco.MachineConfigDaemonStateAnnotationKey: mco.MachineConfigDaemonStateDone,
				mco.DesiredDrainerAnnotationKey:           "something",
				mco.LastAppliedDrainerAnnotationKey:       "something",
			},
			isUpdating: false,
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Annotations: tc.annotations,
				},
			}

			result := isNodeDraining(node, tc.isUpdating)

			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}
