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
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
)

// AssessPoolFromNodeStates creates a complete MachineConfigPoolProgressInsightStatus
// by aggregating node states into pool-level status.
// It preserves condition LastTransitionTime from previous status when condition content hasn't changed.
func AssessPoolFromNodeStates(
	mcpName string,
	scope ouev1alpha1.ScopeType,
	isPaused bool,
	nodeStates []*nodestate.NodeState,
	previous *ouev1alpha1.MachineConfigPoolProgressInsightStatus,
) *ouev1alpha1.MachineConfigPoolProgressInsightStatus {
	now := time.Now()
	summaries := calculateNodeSummaries(nodeStates)
	completion := calculatePoolCompletion(nodeStates)
	assessment := calculatePoolAssessment(isPaused, nodeStates, summaries)
	newConditions := determinePoolConditions(assessment, summaries, now)

	// Build the status with all fields except conditions
	status := &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
		Name:       mcpName,
		Scope:      scope,
		Assessment: assessment,
		Completion: completion,
		Summaries:  summaries,
	}

	// Preserve existing condition timestamps using the Kubernetes standard pattern:
	// For each new condition, use meta.SetStatusCondition which automatically:
	// - Preserves LastTransitionTime if Type/Status/Reason/Message are unchanged
	// - Updates LastTransitionTime if the condition changed
	for _, newCond := range newConditions {
		// First, add the old condition if it exists (so meta.SetStatusCondition can find it)
		if previous != nil {
			if oldCond := meta.FindStatusCondition(previous.Conditions, newCond.Type); oldCond != nil {
				status.Conditions = append(status.Conditions, *oldCond)
			}
		}
		// Then set the new condition (preserving timestamp if unchanged)
		meta.SetStatusCondition(&status.Conditions, newCond)
	}

	return status
}

// calculatePoolAssessment determines the pool's assessment based on node states.
// Assessment Priority (highest to lowest):
// 1. Degraded - Any node degraded → pool degraded
// 2. Excluded - Pool paused AND has outdated nodes → excluded
// 3. Progressing - Nodes actively updating → progressing
// 4. Completed - All nodes updated → completed
// 5. Pending - No nodes started → pending
func calculatePoolAssessment(
	isPaused bool,
	nodeStates []*nodestate.NodeState,
	summaries []ouev1alpha1.NodeSummary,
) ouev1alpha1.PoolAssessment {
	// Empty pool or no nodes
	if len(nodeStates) == 0 {
		return ouev1alpha1.PoolPending
	}

	// Get summary counts
	var degradedCount, outdatedCount int32
	for _, s := range summaries {
		switch s.Type {
		case ouev1alpha1.NodesDegraded:
			degradedCount = s.Count
		case ouev1alpha1.NodesOutdated:
			outdatedCount = s.Count
		}
	}

	// Priority 1: Degraded
	if degradedCount > 0 {
		return ouev1alpha1.PoolDegraded
	}

	// Priority 2: Excluded (paused with outdated nodes)
	if isPaused && outdatedCount > 0 {
		return ouev1alpha1.PoolExcluded
	}

	// Priority 3: Progressing (any node updating/draining)
	hasActiveUpdates := false
	for _, node := range nodeStates {
		if node.Phase == nodestate.UpdatePhaseUpdating ||
			node.Phase == nodestate.UpdatePhaseRebooting ||
			node.Phase == nodestate.UpdatePhaseDraining {
			hasActiveUpdates = true
			break
		}
	}
	if hasActiveUpdates {
		return ouev1alpha1.PoolProgressing
	}

	// Priority 4: Completed (all nodes updated)
	allCompleted := true
	for _, node := range nodeStates {
		if node.Phase != nodestate.UpdatePhaseCompleted {
			allCompleted = false
			break
		}
	}
	if allCompleted {
		return ouev1alpha1.PoolCompleted
	}

	// Priority 5: Pending (default)
	return ouev1alpha1.PoolPending
}

// calculatePoolCompletion returns percentage (0-100) of nodes that completed updating.
func calculatePoolCompletion(nodeStates []*nodestate.NodeState) int32 {
	if len(nodeStates) == 0 {
		return 0
	}

	completedCount := 0
	for _, node := range nodeStates {
		if node.Phase == nodestate.UpdatePhaseCompleted {
			completedCount++
		}
	}

	return int32((completedCount * 100) / len(nodeStates))
}

// calculateNodeSummaries generates all 7 summary types based on node states.
func calculateNodeSummaries(nodeStates []*nodestate.NodeState) []ouev1alpha1.NodeSummary {
	var total, available, progressing, outdated, draining, excluded, degraded int32

	total = int32(len(nodeStates))

	for _, node := range nodeStates {
		// Available: Available condition == True
		if hasCondition(node.Conditions, string(ouev1alpha1.NodeStatusInsightAvailable), metav1.ConditionTrue) {
			available++
		}

		// Progressing: Phase in {Updating, Rebooting}
		if node.Phase == nodestate.UpdatePhaseUpdating || node.Phase == nodestate.UpdatePhaseRebooting {
			progressing++
		}

		// Outdated: Version != DesiredVersion
		if node.Version != node.DesiredVersion {
			outdated++
		}

		// Draining: Phase == Draining
		if node.Phase == nodestate.UpdatePhaseDraining {
			draining++
		}

		// Excluded: Phase == Paused
		if node.Phase == nodestate.UpdatePhasePaused {
			excluded++
		}

		// Degraded: Degraded condition == True
		if hasCondition(node.Conditions, string(ouev1alpha1.NodeStatusInsightDegraded), metav1.ConditionTrue) {
			degraded++
		}
	}

	return []ouev1alpha1.NodeSummary{
		{Type: ouev1alpha1.NodesTotal, Count: total},
		{Type: ouev1alpha1.NodesAvailable, Count: available},
		{Type: ouev1alpha1.NodesProgressing, Count: progressing},
		{Type: ouev1alpha1.NodesOutdated, Count: outdated},
		{Type: ouev1alpha1.NodesDraining, Count: draining},
		{Type: ouev1alpha1.NodesExcluded, Count: excluded},
		{Type: ouev1alpha1.NodesDegraded, Count: degraded},
	}
}

// determinePoolConditions generates Updating and Healthy conditions based on assessment.
func determinePoolConditions(
	assessment ouev1alpha1.PoolAssessment,
	summaries []ouev1alpha1.NodeSummary,
	now time.Time,
) []metav1.Condition {
	metaNow := metav1.Time{Time: now}
	var updatingCond, healthyCond metav1.Condition

	// Check if any nodes are progressing (for Updating condition)
	var progressingCount int32
	for _, s := range summaries {
		if s.Type == ouev1alpha1.NodesProgressing {
			progressingCount = s.Count
			break
		}
	}

	// Determine Updating condition
	// Note: Degraded pools can also be updating (if nodes are actively progressing)
	if assessment == ouev1alpha1.PoolProgressing || (assessment == ouev1alpha1.PoolDegraded && progressingCount > 0) {
		updatingCond = metav1.Condition{
			Type:               string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
			Status:             metav1.ConditionTrue,
			Reason:             string(ouev1alpha1.MachineConfigPoolUpdatingReasonProgressing),
			LastTransitionTime: metaNow,
		}
	} else {
		switch assessment {
		case ouev1alpha1.PoolCompleted:
			updatingCond = metav1.Condition{
				Type:               string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
				Status:             metav1.ConditionFalse,
				Reason:             string(ouev1alpha1.MachineConfigPoolUpdatingReasonUpdated),
				LastTransitionTime: metaNow,
			}
		case ouev1alpha1.PoolExcluded:
			updatingCond = metav1.Condition{
				Type:               string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
				Status:             metav1.ConditionFalse,
				Reason:             string(ouev1alpha1.MachineConfigPoolUpdatingReasonPaused),
				LastTransitionTime: metaNow,
			}
		case ouev1alpha1.PoolPending, ouev1alpha1.PoolDegraded:
			updatingCond = metav1.Condition{
				Type:               string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating),
				Status:             metav1.ConditionFalse,
				Reason:             string(ouev1alpha1.MachineConfigPoolUpdatingReasonPending),
				LastTransitionTime: metaNow,
			}
		}
	}

	// Determine Healthy condition
	var degradedCount int32
	for _, s := range summaries {
		if s.Type == ouev1alpha1.NodesDegraded {
			degradedCount = s.Count
			break
		}
	}

	if degradedCount > 0 {
		healthyCond = metav1.Condition{
			Type:               string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
			Status:             metav1.ConditionFalse,
			Reason:             "NodesDegraded",
			LastTransitionTime: metaNow,
		}
	} else {
		// Healthy - determine reason based on state
		var reason string
		switch assessment {
		case ouev1alpha1.PoolCompleted:
			reason = "AllNodesUpdated"
		case ouev1alpha1.PoolExcluded:
			reason = "Paused"
		case ouev1alpha1.PoolPending:
			// Check if there are any nodes
			var totalCount int32
			for _, s := range summaries {
				if s.Type == ouev1alpha1.NodesTotal {
					totalCount = s.Count
					break
				}
			}
			if totalCount == 0 {
				reason = "NoNodes"
			} else {
				reason = "NoProblems"
			}
		default:
			reason = "NoProblems"
		}

		healthyCond = metav1.Condition{
			Type:               string(ouev1alpha1.MachineConfigPoolProgressInsightHealthy),
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			LastTransitionTime: metaNow,
		}
	}

	return []metav1.Condition{updatingCond, healthyCond}
}

// hasCondition checks if a condition with the given type and status exists.
func hasCondition(conditions []metav1.Condition, condType string, status metav1.ConditionStatus) bool {
	for _, c := range conditions {
		if c.Type == condType && c.Status == status {
			return true
		}
	}
	return false
}
