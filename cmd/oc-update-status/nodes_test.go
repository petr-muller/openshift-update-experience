package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestPoolDisplayData_WriteNodes(t *testing.T) {
	testCases := []struct {
		name     string
		data     poolDisplayData
		detailed bool
		maxlines int

		expected string
	}{
		{
			name: "empty pool",
			data: poolDisplayData{
				Name: "master",
			},
		},
		{
			name: "completed masters",
			data: poolDisplayData{
				Name:       "master",
				Completion: 100,
				Nodes: []nodeDisplayData{
					{Name: "node1", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: ""},
					{Name: "node2", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: ""},
					{Name: "node3", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: ""},
				},
			},
			maxlines: 3,
			expected: "\nAll control plane nodes successfully updated to 4.19.20\n",
		},
		{
			name: "updating masters",
			data: poolDisplayData{
				Name:       "master",
				Completion: 33,
				Nodes: []nodeDisplayData{
					{Name: "node1", Assessment: nodeAssessmentOutdated, Phase: phaseStatePending, Version: "4.19.20", Estimate: "5m", Message: "Something is happening"},
					{Name: "node2", Assessment: nodeAssessmentProgressing, Phase: phaseStateRebooting, Version: "4.19.20", Estimate: "3m", Message: "Rebooting"},
					{Name: "node3", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: ""},
				},
			},
			maxlines: 3,
			expected: `
Control Plane Nodes
NAME    ASSESSMENT    PHASE       VERSION   EST   MESSAGE
node1   Outdated      Pending     4.19.20   5m    Something is happening
node2   Progressing   Rebooting   4.19.20   3m    Rebooting
node3   Completed     Updated     4.19.20   0s    
`,
		},
		{
			name: "updating nodes",
			data: poolDisplayData{
				Name:       "worker",
				Completion: 50,
				Nodes: []nodeDisplayData{
					{Name: "node1", Assessment: nodeAssessmentOutdated, Phase: phaseStatePending, Version: "4.19.20", Estimate: "5m", Message: "Something is happening"},
					{Name: "node2", Assessment: nodeAssessmentProgressing, Phase: phaseStateRebooting, Version: "4.19.20", Estimate: "3m", Message: "Rebooting"},
				},
			},
			maxlines: 2,
			expected: `
Worker Pool Nodes: worker
NAME    ASSESSMENT    PHASE       VERSION   EST   MESSAGE
node1   Outdated      Pending     4.19.20   5m    Something is happening
node2   Progressing   Rebooting   4.19.20   3m    Rebooting
`,
		},
		{
			name: "nodes above maxlines are not shown",
			data: poolDisplayData{
				Name:       "worker",
				Completion: 100,
				Nodes: []nodeDisplayData{
					{Name: "node1", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: "", isUpdated: true},
					{Name: "node2", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: "", isUpdated: true},
					{Name: "node3", Assessment: nodeAssessmentOutdated, Phase: phaseStatePending, Version: "4.19.19", Estimate: "0s", Message: ""},
					{Name: "node4", Assessment: nodeAssessmentProgressing, Phase: phaseStateUpdating, Version: "4.19.19", Estimate: "0s", Message: "", isUpdating: true},
					{Name: "node5", Assessment: nodeAssessmentProgressing, Phase: phaseStateDraining, Version: "4.19.19", Estimate: "0s", Message: "", isUpdating: true},
					{Name: "node6", Assessment: nodeAssessmentExcluded, Phase: phaseStatePaused, Version: "4.19.19", Estimate: "0s", Message: ""},
				},
			},
			maxlines: 1,
			expected: `
Worker Pool Nodes: worker
NAME    ASSESSMENT   PHASE     VERSION   EST   MESSAGE
node1   Completed    Updated   4.19.20   0s    
...
Omitted additional 5 Total, 1 Completed, 5 Available, 2 Progressing, 4 Outdated, 1 Draining, 1 Excluded, and 0 Degraded nodes.
Pass along --details=nodes to see all information.
`,
		},
		{
			name: "nodes above maxlines are shown in detailed mode",
			data: poolDisplayData{
				Name:       "worker",
				Completion: 100,
				Nodes: []nodeDisplayData{
					{Name: "node1", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: "", isUpdated: true},
					{Name: "node2", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: "", isUpdated: true},
				},
			},
			detailed: true,
			maxlines: 1,
			expected: `
Worker Pool Nodes: worker
NAME    ASSESSMENT   PHASE     VERSION   EST   MESSAGE
node1   Completed    Updated   4.19.20   0s    
node2   Completed    Updated   4.19.20   0s    
`,
		},
		{
			name: "degraded nodes above maxlines are shown",
			data: poolDisplayData{
				Name:       "worker",
				Completion: 50,
				Nodes: []nodeDisplayData{
					{Name: "node1", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: "", isUpdated: true},
					{Name: "node2", Assessment: nodeAssessmentDegraded, Phase: phaseStateRebooting, Version: "4.19.20", Estimate: "3m", Message: "Something is wrong", isDegraded: true, isUpdating: true},
				},
			},
			maxlines: 1,
			expected: `
Worker Pool Nodes: worker
NAME    ASSESSMENT   PHASE       VERSION   EST   MESSAGE
node1   Completed    Updated     4.19.20   0s    
node2   Degraded     Rebooting   4.19.20   3m    Something is wrong
`,
		},
		{
			name: "unavailable nodes above maxlines are shown",
			data: poolDisplayData{
				Name:       "worker",
				Completion: 0,
				Nodes: []nodeDisplayData{
					{Name: "node1", Assessment: nodeAssessmentCompleted, Phase: phaseStateUpdated, Version: "4.19.20", Estimate: "0s", Message: "", isUpdated: true},
					{Name: "node1", Assessment: nodeAssessmentUnavailable, Phase: phaseStatePending, Version: "4.19.20", Estimate: "3m", Message: "Something is wrong", isUnavailable: true},
				},
			},
			maxlines: 1,
			expected: `
Worker Pool Nodes: worker
NAME    ASSESSMENT    PHASE     VERSION   EST   MESSAGE
node1   Completed     Updated   4.19.20   0s    
node1   Unavailable   Pending   4.19.20   3m    Something is wrong
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			tc.data.WriteNodes(&b, tc.detailed, tc.maxlines)

			if diff := cmp.Diff(tc.expected, b.String()); diff != "" {
				t.Errorf("unexpected output (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAssessPool(t *testing.T) {
	nodeInsightTemplate := ouev1alpha1.NodeProgressInsight{
		Status: ouev1alpha1.NodeProgressInsightStatus{
			Conditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionTrue},
				{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
			},
			Name: "node",
			PoolResource: ouev1alpha1.ResourceRef{
				Name:     "master",
				Resource: "machineconfigpools",
				Group:    "machineconfiguration.openshift.io",
			},
			Scope:               ouev1alpha1.ControlPlaneScope,
			Version:             "4.19.20",
			EstimatedToComplete: ptr.To(metav1.Duration{Duration: 5 * time.Minute}),
			Message:             "Something is happening",
		},
	}

	testCases := []struct {
		name               string
		updatingConditions []metav1.Condition

		expectCompletion int
	}{
		{
			name:             "empty pool",
			expectCompletion: 0,
		},
		{
			name: "all nodes not updated",
			updatingConditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeUpdatePending)},
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeUpdatePending)},
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeUpdatePending)},
			},
			expectCompletion: 0,
		},
		{
			name: "some nodes updated",
			updatingConditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeUpdatePending)},
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeRebooting)},
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeCompleted)},
			},
			expectCompletion: 33,
		},
		{
			name: "all nodes updated",
			updatingConditions: []metav1.Condition{
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeCompleted)},
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeCompleted)},
				{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeCompleted)},
			},
			expectCompletion: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var insights []ouev1alpha1.NodeProgressInsight
			for _, updating := range tc.updatingConditions {
				insight := nodeInsightTemplate.DeepCopy()
				insight.Status.Conditions = append(insight.Status.Conditions, updating)
				insights = append(insights, *insight)
			}

			pool := assessPool("master", insights)

			if pool.Name != "master" {
				t.Errorf("unexpected pool name %q, expected %q", pool.Name, "master")
			}
			if pool.Completion != tc.expectCompletion {
				t.Errorf("unexpected pool completion %d, expected %d", pool.Completion, tc.expectCompletion)
			}
			if len(pool.Nodes) != len(insights) {
				t.Errorf("unexpected number of nodes %d, expected %d", len(pool.Nodes), len(insights))
			}
		})
	}
}

func TestAssessNode(t *testing.T) {
	insight := ouev1alpha1.NodeProgressInsight{
		Status: ouev1alpha1.NodeProgressInsightStatus{
			Conditions: []metav1.Condition{
				{
					Type:   string(ouev1alpha1.NodeStatusInsightUpdating),
					Status: metav1.ConditionFalse,
					Reason: string(ouev1alpha1.NodeUpdatePending),
				},
				{
					Type:   string(ouev1alpha1.NodeStatusInsightAvailable),
					Status: metav1.ConditionTrue,
				},
				{
					Type:   string(ouev1alpha1.NodeStatusInsightDegraded),
					Status: metav1.ConditionFalse,
				},
			},
			Name: "node",
			PoolResource: ouev1alpha1.ResourceRef{
				Name:     "master",
				Resource: "machineconfigpools",
				Group:    "machineconfiguration.openshift.io",
			},
			Scope:               ouev1alpha1.ControlPlaneScope,
			Version:             "4.19.20",
			EstimatedToComplete: ptr.To(metav1.Duration{Duration: 5 * time.Minute}),
			Message:             "Something is happening",
		},
	}

	displayData := assessNode(insight)

	expected := nodeDisplayData{
		Name:          "node",
		Assessment:    nodeAssessmentOutdated,
		Phase:         phaseStatePending,
		Version:       "4.19.20",
		Estimate:      "5m",
		Message:       "Something is happening",
		isUnavailable: false,
		isDegraded:    false,
		isUpdating:    false,
		isUpdated:     false,
	}

	if diff := cmp.Diff(expected, displayData, cmp.AllowUnexported(nodeDisplayData{})); diff != "" {
		t.Errorf("unexpected display data (-want +got):\n%s", diff)
	}
}

func TestAssessNode_Assessment(t *testing.T) {
	trueCondition := metav1.Condition{Status: metav1.ConditionTrue}
	falseCondition := metav1.Condition{Status: metav1.ConditionFalse}

	testCases := []struct {
		name      string
		updating  metav1.Condition
		available metav1.Condition
		degraded  metav1.Condition

		expectAssessment    nodeAssessment
		expectPhase         nodePhase
		expectIsUnavailable bool
		expectIsDegraded    bool
		expectIsUpdating    bool
		expectIsUpdated     bool
	}{
		{
			name:      "pending node",
			updating:  metav1.Condition{Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeUpdatePending)},
			available: trueCondition,
			degraded:  falseCondition,

			expectAssessment:    nodeAssessmentOutdated,
			expectPhase:         phaseStatePending,
			expectIsUnavailable: false,
			expectIsDegraded:    false,
			expectIsUpdating:    false,
			expectIsUpdated:     false,
		},
		{
			name:      "progressing node",
			updating:  metav1.Condition{Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeUpdating)},
			available: trueCondition,
			degraded:  falseCondition,

			expectAssessment:    nodeAssessmentProgressing,
			expectPhase:         phaseStateUpdating,
			expectIsUnavailable: false,
			expectIsDegraded:    false,
			expectIsUpdating:    true,
			expectIsUpdated:     false,
		},
		{
			name:      "draining node",
			updating:  metav1.Condition{Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeDraining)},
			available: trueCondition,
			degraded:  falseCondition,

			expectAssessment:    nodeAssessmentProgressing,
			expectPhase:         phaseStateDraining,
			expectIsUnavailable: false,
			expectIsDegraded:    false,
			expectIsUpdating:    true,
			expectIsUpdated:     false,
		},
		{
			name:      "rebooting node",
			updating:  metav1.Condition{Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeRebooting)},
			available: trueCondition,
			degraded:  falseCondition,

			expectAssessment:    nodeAssessmentProgressing,
			expectPhase:         phaseStateRebooting,
			expectIsUnavailable: false,
			expectIsDegraded:    false,
			expectIsUpdating:    true,
			expectIsUpdated:     false,
		},
		{
			name:      "updated node",
			updating:  metav1.Condition{Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeCompleted)},
			available: trueCondition,
			degraded:  falseCondition,

			expectAssessment:    nodeAssessmentCompleted,
			expectPhase:         phaseStateUpdated,
			expectIsUnavailable: false,
			expectIsDegraded:    false,
			expectIsUpdating:    false,
			expectIsUpdated:     true,
		},
		{
			name:      "paused node",
			updating:  metav1.Condition{Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodePaused)},
			available: trueCondition,
			degraded:  falseCondition,

			expectAssessment:    nodeAssessmentExcluded,
			expectPhase:         phaseStatePaused,
			expectIsUnavailable: false,
			expectIsDegraded:    false,
			expectIsUpdating:    false,
			expectIsUpdated:     false,
		},
		{
			name:      "degraded node",
			updating:  metav1.Condition{Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeRebooting)},
			available: trueCondition,
			degraded:  trueCondition,

			expectAssessment:    nodeAssessmentDegraded,
			expectPhase:         phaseStateRebooting,
			expectIsUnavailable: false,
			expectIsDegraded:    true,
			expectIsUpdating:    true,
			expectIsUpdated:     false,
		},
		{
			name:      "unavailable node",
			updating:  metav1.Condition{Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeRebooting)},
			available: falseCondition,
			degraded:  falseCondition,

			expectAssessment:    nodeAssessmentUnavailable,
			expectPhase:         phaseStateRebooting,
			expectIsUnavailable: true,
			expectIsDegraded:    false,
			expectIsUpdating:    true,
			expectIsUpdated:     false,
		},
		{
			name:      "degraded and unavailable node",
			updating:  metav1.Condition{Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeRebooting)},
			available: falseCondition,
			degraded:  trueCondition,

			expectAssessment:    nodeAssessmentUnavailable,
			expectPhase:         phaseStateRebooting,
			expectIsUnavailable: true,
			expectIsDegraded:    true,
			expectIsUpdating:    true,
			expectIsUpdated:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.updating.Type = string(ouev1alpha1.NodeStatusInsightUpdating)
			tc.available.Type = string(ouev1alpha1.NodeStatusInsightAvailable)
			tc.degraded.Type = string(ouev1alpha1.NodeStatusInsightDegraded)

			insight := ouev1alpha1.NodeProgressInsight{
				Status: ouev1alpha1.NodeProgressInsightStatus{
					Conditions: []metav1.Condition{tc.updating, tc.available, tc.degraded},
					Name:       "node",
					PoolResource: ouev1alpha1.ResourceRef{
						Name:     "pool",
						Resource: "machineconfigpools",
						Group:    "machineconfiguration.openshift.io",
					},
					Scope:               "ControlPlane",
					Version:             "4.20.0",
					EstimatedToComplete: nil,
					Message:             "",
				},
			}

			displayData := assessNode(insight)

			if displayData.Assessment != tc.expectAssessment {
				t.Errorf("unexpected assessment %v, expected %v", displayData.Assessment, tc.expectAssessment)
			}
			if displayData.Phase != tc.expectPhase {
				t.Errorf("unexpected phase %v, expected %v", displayData.Phase, tc.expectPhase)
			}
			if displayData.isUnavailable != tc.expectIsUnavailable {
				t.Errorf("unexpected isUnavailable %v, expected %v", displayData.isUnavailable, tc.expectIsUnavailable)
			}
			if displayData.isDegraded != tc.expectIsDegraded {
				t.Errorf("unexpected isDegraded %v, expected %v", displayData.isDegraded, tc.expectIsDegraded)
			}
			if displayData.isUpdating != tc.expectIsUpdating {
				t.Errorf("unexpected isUpdating %v, expected %v", displayData.isUpdating, tc.expectIsUpdating)
			}
			if displayData.isUpdated != tc.expectIsUpdated {
				t.Errorf("unexpected isUpdated %v, expected %v", displayData.isUpdated, tc.expectIsUpdated)
			}
		})
	}
}
