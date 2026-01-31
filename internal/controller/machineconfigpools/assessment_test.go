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
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
)

func TestAssessPoolFromNodeStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mcpName    string
		scope      ouev1alpha1.ScopeType
		isPaused   bool
		nodeStates []*nodestate.NodeState
		expected   *ouev1alpha1.MachineConfigPoolProgressInsightStatus
	}{
		{
			name:       "empty pool",
			mcpName:    "worker",
			scope:      ouev1alpha1.WorkerPoolScope,
			isPaused:   false,
			nodeStates: []*nodestate.NodeState{},
			expected: &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Name:       "worker",
				Scope:      ouev1alpha1.WorkerPoolScope,
				Assessment: ouev1alpha1.PoolPending,
				Completion: 0,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 0},
					{Type: ouev1alpha1.NodesAvailable, Count: 0},
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
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonPending),
					},
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
						Status: metav1.ConditionTrue,
						Reason: "NoNodes",
					},
				},
			},
		},
		{
			name:     "all nodes completed",
			mcpName:  "worker",
			scope:    ouev1alpha1.WorkerPoolScope,
			isPaused: false,
			nodeStates: []*nodestate.NodeState{
				{
					Name:           "node1",
					UID:            types.UID("uid1"),
					Version:        "4.18.0",
					DesiredVersion: "4.18.0",
					CurrentConfig:  "rendered-worker-123",
					DesiredConfig:  "rendered-worker-123",
					Phase:          nodestate.UpdatePhaseCompleted,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeCompleted)},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionTrue},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
					},
				},
				{
					Name:           "node2",
					UID:            types.UID("uid2"),
					Version:        "4.18.0",
					DesiredVersion: "4.18.0",
					CurrentConfig:  "rendered-worker-123",
					DesiredConfig:  "rendered-worker-123",
					Phase:          nodestate.UpdatePhaseCompleted,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeCompleted)},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionTrue},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
					},
				},
			},
			expected: &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Name:       "worker",
				Scope:      ouev1alpha1.WorkerPoolScope,
				Assessment: ouev1alpha1.PoolCompleted,
				Completion: 100,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 2},
					{Type: ouev1alpha1.NodesAvailable, Count: 2},
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
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
						Status: metav1.ConditionTrue,
						Reason: "AllNodesUpdated",
					},
				},
			},
		},
		{
			name:     "mixed states - some updating",
			mcpName:  "worker",
			scope:    ouev1alpha1.WorkerPoolScope,
			isPaused: false,
			nodeStates: []*nodestate.NodeState{
				{
					Name:           "node1",
					UID:            types.UID("uid1"),
					Version:        "4.18.0",
					DesiredVersion: "4.18.0",
					CurrentConfig:  "rendered-worker-123",
					DesiredConfig:  "rendered-worker-123",
					Phase:          nodestate.UpdatePhaseCompleted,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeCompleted)},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionTrue},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
					},
				},
				{
					Name:           "node2",
					UID:            types.UID("uid2"),
					Version:        "4.17.0",
					DesiredVersion: "4.18.0",
					CurrentConfig:  "rendered-worker-122",
					DesiredConfig:  "rendered-worker-123",
					Phase:          nodestate.UpdatePhaseUpdating,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeUpdating)},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionFalse},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
					},
				},
				{
					Name:           "node3",
					UID:            types.UID("uid3"),
					Version:        "4.17.0",
					DesiredVersion: "4.18.0",
					CurrentConfig:  "rendered-worker-122",
					DesiredConfig:  "rendered-worker-123",
					Phase:          nodestate.UpdatePhasePending,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeUpdatePending)},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionTrue},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
					},
				},
			},
			expected: &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Name:       "worker",
				Scope:      ouev1alpha1.WorkerPoolScope,
				Assessment: ouev1alpha1.PoolProgressing,
				Completion: 33, // 1 of 3 completed
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 3},
					{Type: ouev1alpha1.NodesAvailable, Count: 2},   // node1, node3
					{Type: ouev1alpha1.NodesProgressing, Count: 1}, // node2
					{Type: ouev1alpha1.NodesOutdated, Count: 2},    // node2, node3
					{Type: ouev1alpha1.NodesDraining, Count: 0},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
				Conditions: []metav1.Condition{
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
						Status: metav1.ConditionTrue,
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonProgressing),
					},
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
						Status: metav1.ConditionTrue,
						Reason: "NoProblems",
					},
				},
			},
		},
		{
			name:     "degraded node - pool becomes degraded",
			mcpName:  "worker",
			scope:    ouev1alpha1.WorkerPoolScope,
			isPaused: false,
			nodeStates: []*nodestate.NodeState{
				{
					Name:           "node1",
					UID:            types.UID("uid1"),
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
					UID:            types.UID("uid2"),
					Version:        "4.17.0",
					DesiredVersion: "4.18.0",
					Phase:          nodestate.UpdatePhaseUpdating,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionTrue},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionFalse},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionTrue, Reason: "UpdateFailed"},
					},
				},
			},
			expected: &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Name:       "worker",
				Scope:      ouev1alpha1.WorkerPoolScope,
				Assessment: ouev1alpha1.PoolDegraded,
				Completion: 50, // 1 of 2 completed
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 2},
					{Type: ouev1alpha1.NodesAvailable, Count: 1},
					{Type: ouev1alpha1.NodesProgressing, Count: 1},
					{Type: ouev1alpha1.NodesOutdated, Count: 1},
					{Type: ouev1alpha1.NodesDraining, Count: 0},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 1},
				},
				Conditions: []metav1.Condition{
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
						Status: metav1.ConditionTrue,
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonProgressing),
					},
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
						Status: metav1.ConditionFalse,
						Reason: "NodesDegraded",
					},
				},
			},
		},
		{
			name:     "paused pool with outdated nodes - excluded",
			mcpName:  "worker",
			scope:    ouev1alpha1.WorkerPoolScope,
			isPaused: true,
			nodeStates: []*nodestate.NodeState{
				{
					Name:           "node1",
					UID:            types.UID("uid1"),
					Version:        "4.17.0",
					DesiredVersion: "4.18.0",
					CurrentConfig:  "rendered-worker-122",
					DesiredConfig:  "rendered-worker-123",
					Phase:          nodestate.UpdatePhasePaused,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodePaused)},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionTrue},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
					},
				},
			},
			expected: &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Name:       "worker",
				Scope:      ouev1alpha1.WorkerPoolScope,
				Assessment: ouev1alpha1.PoolExcluded,
				Completion: 0,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 1},
					{Type: ouev1alpha1.NodesAvailable, Count: 1},
					{Type: ouev1alpha1.NodesProgressing, Count: 0},
					{Type: ouev1alpha1.NodesOutdated, Count: 1},
					{Type: ouev1alpha1.NodesDraining, Count: 0},
					{Type: ouev1alpha1.NodesExcluded, Count: 1},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
				Conditions: []metav1.Condition{
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
						Status: metav1.ConditionFalse,
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonPaused),
					},
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
						Status: metav1.ConditionTrue,
						Reason: "Paused",
					},
				},
			},
		},
		{
			name:     "all nodes pending",
			mcpName:  "worker",
			scope:    ouev1alpha1.WorkerPoolScope,
			isPaused: false,
			nodeStates: []*nodestate.NodeState{
				{
					Name:           "node1",
					UID:            types.UID("uid1"),
					Version:        "4.17.0",
					DesiredVersion: "4.18.0",
					Phase:          nodestate.UpdatePhasePending,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionFalse, Reason: string(ouev1alpha1.NodeUpdatePending)},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionTrue},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
					},
				},
			},
			expected: &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Name:       "worker",
				Scope:      ouev1alpha1.WorkerPoolScope,
				Assessment: ouev1alpha1.PoolPending,
				Completion: 0,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 1},
					{Type: ouev1alpha1.NodesAvailable, Count: 1},
					{Type: ouev1alpha1.NodesProgressing, Count: 0},
					{Type: ouev1alpha1.NodesOutdated, Count: 1},
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
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
						Status: metav1.ConditionTrue,
						Reason: "NoProblems",
					},
				},
			},
		},
		{
			name:     "draining node",
			mcpName:  "worker",
			scope:    ouev1alpha1.WorkerPoolScope,
			isPaused: false,
			nodeStates: []*nodestate.NodeState{
				{
					Name:           "node1",
					UID:            types.UID("uid1"),
					Version:        "4.17.0",
					DesiredVersion: "4.18.0",
					Phase:          nodestate.UpdatePhaseDraining,
					Conditions: []metav1.Condition{
						{Type: string(ouev1alpha1.NodeStatusInsightUpdating), Status: metav1.ConditionTrue, Reason: string(ouev1alpha1.NodeDraining)},
						{Type: string(ouev1alpha1.NodeStatusInsightAvailable), Status: metav1.ConditionFalse},
						{Type: string(ouev1alpha1.NodeStatusInsightDegraded), Status: metav1.ConditionFalse},
					},
				},
			},
			expected: &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
				Name:       "worker",
				Scope:      ouev1alpha1.WorkerPoolScope,
				Assessment: ouev1alpha1.PoolProgressing,
				Completion: 0,
				Summaries: []ouev1alpha1.NodeSummary{
					{Type: ouev1alpha1.NodesTotal, Count: 1},
					{Type: ouev1alpha1.NodesAvailable, Count: 0},
					{Type: ouev1alpha1.NodesProgressing, Count: 0},
					{Type: ouev1alpha1.NodesOutdated, Count: 1},
					{Type: ouev1alpha1.NodesDraining, Count: 1},
					{Type: ouev1alpha1.NodesExcluded, Count: 0},
					{Type: ouev1alpha1.NodesDegraded, Count: 0},
				},
				Conditions: []metav1.Condition{
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
						Status: metav1.ConditionTrue,
						Reason: string(ouev1alpha1.MachineConfigPoolUpdatingReasonProgressing),
					},
					{
						Type:   string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
						Status: metav1.ConditionTrue,
						Reason: "NoProblems",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := AssessPoolFromNodeStates(tt.mcpName, tt.scope, tt.isPaused, tt.nodeStates, nil)

			// Compare fields
			if result.Name != tt.expected.Name {
				t.Errorf("Name: expected %q, got %q", tt.expected.Name, result.Name)
			}
			if result.Scope != tt.expected.Scope {
				t.Errorf("Scope: expected %q, got %q", tt.expected.Scope, result.Scope)
			}
			if result.Assessment != tt.expected.Assessment {
				t.Errorf("Assessment: expected %q, got %q", tt.expected.Assessment, result.Assessment)
			}
			if result.Completion != tt.expected.Completion {
				t.Errorf("Completion: expected %d, got %d", tt.expected.Completion, result.Completion)
			}

			// Compare summaries using cmp.Diff (order-independent with ElementsMatch behavior via sorting in test)
			if diff := cmp.Diff(tt.expected.Summaries, result.Summaries); diff != "" {
				t.Errorf("Summaries mismatch (-want +got):\n%s", diff)
			}

			// Compare conditions (checking Type, Status, Reason but ignoring timestamps)
			if len(result.Conditions) != len(tt.expected.Conditions) {
				t.Errorf("Conditions count: expected %d, got %d", len(tt.expected.Conditions), len(result.Conditions))
			} else {
				for i, expectedCond := range tt.expected.Conditions {
					if result.Conditions[i].Type != expectedCond.Type {
						t.Errorf("Condition[%d].Type: expected %q, got %q", i, expectedCond.Type, result.Conditions[i].Type)
					}
					if result.Conditions[i].Status != expectedCond.Status {
						t.Errorf("Condition[%d].Status: expected %q, got %q", i, expectedCond.Status, result.Conditions[i].Status)
					}
					if result.Conditions[i].Reason != expectedCond.Reason {
						t.Errorf("Condition[%d].Reason: expected %q, got %q", i, expectedCond.Reason, result.Conditions[i].Reason)
					}
					// Verify LastTransitionTime is set
					if result.Conditions[i].LastTransitionTime.IsZero() {
						t.Errorf("Condition[%d].LastTransitionTime should be set, got zero value", i)
					}
				}
			}
		})
	}
}
