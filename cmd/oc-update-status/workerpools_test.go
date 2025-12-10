package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

func Test_formatPoolAssessment(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		status             ouev1alpha1.MachineConfigPoolProgressInsightStatus
		expectedAssessment string
	}{
		{
			name: "Pending",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Assessment: ouev1alpha1.PoolPending,
			},
			expectedAssessment: "Pending",
		},
		{
			name: "Completed",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Assessment: ouev1alpha1.PoolCompleted,
			},
			expectedAssessment: "Completed",
		},
		{
			name: "Degraded",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Assessment: ouev1alpha1.PoolDegraded,
			},
			expectedAssessment: "Degraded",
		},
		{
			name: "Excluded",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Assessment: ouev1alpha1.PoolExcluded,
			},
			expectedAssessment: "Excluded",
		},
		{
			name: "Progressing",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Assessment: ouev1alpha1.PoolProgressing,
			},
			expectedAssessment: "Progressing",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := formatPoolAssessment(tc.status)

			if diff := cmp.Diff(tc.expectedAssessment, actual); diff != "" {
				t.Errorf("formatPoolAssessment() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func Test_formatPoolCompletion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		status             ouev1alpha1.MachineConfigPoolProgressInsightStatus
		expectedCompletion string
	}{
		{
			name: "0% - all nodes outdated",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Completion: 0,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 3},
					{Type: ouev1alpha1.NodesAvailable, Count: 3},
					{Type: ouev1alpha1.NodesProgressing, Count: 0},
					{Type: ouev1alpha1.NodesOutdated, Count: 3},
					{Type: ouev1alpha1.NodesDraining, Count: 0},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
			},
			expectedCompletion: "0% (0/3)",
		},
		{
			name: "100% - one node completed",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Completion: 100,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 1},
					{Type: ouev1alpha1.NodesAvailable, Count: 1},
					{Type: ouev1alpha1.NodesProgressing, Count: 0},
					{Type: ouev1alpha1.NodesOutdated, Count: 0},
					{Type: ouev1alpha1.NodesDraining, Count: 0},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
			},
			expectedCompletion: "100% (1/1)",
		},
		{
			name: "50% - one completed, one progressing (draining, degraded)",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Completion: 50,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 2},
					{Type: ouev1alpha1.NodesAvailable, Count: 2},
					{Type: ouev1alpha1.NodesProgressing, Count: 1},
					{Type: ouev1alpha1.NodesOutdated, Count: 1},
					{Type: ouev1alpha1.NodesDraining, Count: 1},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 1},
				},
			},
			expectedCompletion: "50% (1/2)",
		},
		{
			name: "25% - two progressing, one excluded, one completed",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Completion: 25,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 4},
					{Type: ouev1alpha1.NodesAvailable, Count: 1},
					{Type: ouev1alpha1.NodesProgressing, Count: 2},
					{Type: ouev1alpha1.NodesOutdated, Count: 3},
					{Type: ouev1alpha1.NodesDraining, Count: 1},
					{Type: ouev1alpha1.NodesExcluded, Count: 1},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
			},
			expectedCompletion: "25% (1/4)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := formatPoolCompletion(tc.status)

			if diff := cmp.Diff(tc.expectedCompletion, actual); diff != "" {
				t.Errorf("formatPoolCompletion() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func Test_formatPoolStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		status         ouev1alpha1.MachineConfigPoolProgressInsightStatus
		expectedStatus string
	}{
		{
			name: "0% - all nodes outdated (not updating)",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Completion: 0,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 3},
					{Type: ouev1alpha1.NodesAvailable, Count: 3},
					{Type: ouev1alpha1.NodesProgressing, Count: 0},
					{Type: ouev1alpha1.NodesOutdated, Count: 3},
					{Type: ouev1alpha1.NodesDraining, Count: 0},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
				Conditions: []metav1.Condition{
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
						Status: metav1.ConditionFalse,
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonPending),
					},
				},
			},
			expectedStatus: "3 Available",
		},
		{
			name: "100% - one node completed (not updating)",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Completion: 100,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 1},
					{Type: ouev1alpha1.NodesAvailable, Count: 1},
					{Type: ouev1alpha1.NodesProgressing, Count: 0},
					{Type: ouev1alpha1.NodesOutdated, Count: 0},
					{Type: ouev1alpha1.NodesDraining, Count: 0},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
				Conditions: []metav1.Condition{
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
						Status: metav1.ConditionFalse,
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonUpdated),
					},
				},
			},
			expectedStatus: "1 Available",
		},
		{
			name: "50% - one completed, one progressing (updating, draining, degraded)",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Completion: 50,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 2},
					{Type: ouev1alpha1.NodesAvailable, Count: 2},
					{Type: ouev1alpha1.NodesProgressing, Count: 1},
					{Type: ouev1alpha1.NodesOutdated, Count: 1},
					{Type: ouev1alpha1.NodesDraining, Count: 1},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 1},
				},
				Conditions: []metav1.Condition{
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
						Status: metav1.ConditionTrue,
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonProgressing),
					},
				},
			},
			expectedStatus: "2 Available, 1 Progressing, 1 Draining, 1 Degraded",
		},
		{
			name: "25% - two progressing, one excluded, one completed (updating)",
			status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Completion: 25,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 4},
					{Type: ouev1alpha1.NodesAvailable, Count: 1},
					{Type: ouev1alpha1.NodesProgressing, Count: 2},
					{Type: ouev1alpha1.NodesOutdated, Count: 3},
					{Type: ouev1alpha1.NodesDraining, Count: 1},
					{Type: ouev1alpha1.NodesExcluded, Count: 1},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
				Conditions: []metav1.Condition{
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
						Status: metav1.ConditionTrue,
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonProgressing),
					},
				},
			},
			expectedStatus: "1 Available, 2 Progressing, 1 Draining, 1 Excluded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := formatPoolStatus(tc.status)

			if diff := cmp.Diff(tc.expectedStatus, actual); diff != "" {
				t.Errorf("formatPoolStatus() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}
