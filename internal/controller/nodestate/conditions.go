package nodestate

import (
	"strings"
	"time"

	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DetermineConditions evaluates node state and returns conditions, message, and update phase.
// This function encapsulates the complex logic for determining a node's update status
// based on MCO annotations, pool configuration, and node state.
func DetermineConditions(
	pool *openshiftmachineconfigurationv1.MachineConfigPool,
	node *corev1.Node,
	isUpdating, isUpdated, isUnavailable, isDegraded bool,
	lns *mco.LayeredNodeState,
	existingConditions []metav1.Condition,
	now time.Time,
) ([]metav1.Condition, string, UpdatePhase) {
	metaNow := metav1.Time{Time: now}

	// Start with only the conditions we manage to avoid accumulating stale conditions
	// meta.SetStatusCondition will preserve LastTransitionTime when status doesn't change
	conditions := []metav1.Condition{}
	for _, cond := range existingConditions {
		if cond.Type == string(ouev1alpha1.NodeStatusInsightUpdating) ||
			cond.Type == string(ouev1alpha1.NodeStatusInsightAvailable) ||
			cond.Type == string(ouev1alpha1.NodeStatusInsightDegraded) {
			conditions = append(conditions, cond)
		}
	}

	var phase UpdatePhase

	updating := metav1.Condition{
		Type:               string(ouev1alpha1.NodeStatusInsightUpdating),
		Status:             metav1.ConditionUnknown,
		Reason:             string(ouev1alpha1.NodeCannotDetermine),
		Message:            "Cannot determine whether the node is updating",
		LastTransitionTime: metaNow,
	}
	available := metav1.Condition{
		Type:               string(ouev1alpha1.NodeStatusInsightAvailable),
		Status:             metav1.ConditionTrue,
		Reason:             "AsExpected",
		Message:            "The node is available",
		LastTransitionTime: metaNow,
	}
	degraded := metav1.Condition{
		Type:               string(ouev1alpha1.NodeStatusInsightDegraded),
		Status:             metav1.ConditionFalse,
		Reason:             "AsExpected",
		Message:            "The node is not degraded",
		LastTransitionTime: metaNow,
	}

	if isUpdating && isNodeDraining(node, isUpdating) {
		phase = UpdatePhaseDraining
		updating.Status = metav1.ConditionTrue
		updating.Reason = string(ouev1alpha1.NodeDraining)
		updating.Message = "The node is draining"
	} else if isUpdating {
		state := node.Annotations[mco.MachineConfigDaemonStateAnnotationKey]
		switch state {
		case mco.MachineConfigDaemonStateRebooting:
			phase = UpdatePhaseRebooting
			updating.Status = metav1.ConditionTrue
			updating.Reason = string(ouev1alpha1.NodeRebooting)
			updating.Message = "The node is rebooting"
		case mco.MachineConfigDaemonStateDone:
			phase = UpdatePhaseCompleted
			updating.Status = metav1.ConditionFalse
			updating.Reason = string(ouev1alpha1.NodeCompleted)
			updating.Message = "The node is updated"
		default:
			phase = UpdatePhaseUpdating
			updating.Status = metav1.ConditionTrue
			updating.Reason = string(ouev1alpha1.NodeUpdating)
			updating.Message = "The node is updating"
		}

	} else if isUpdated {
		phase = UpdatePhaseCompleted
		updating.Status = metav1.ConditionFalse
		updating.Reason = string(ouev1alpha1.NodeCompleted)
		updating.Message = "The node is updated"
	} else if pool.Spec.Paused {
		phase = UpdatePhasePaused
		updating.Status = metav1.ConditionFalse
		updating.Reason = string(ouev1alpha1.NodePaused)
		updating.Message = "The update of the node is paused"
	} else {
		phase = UpdatePhasePending
		updating.Status = metav1.ConditionFalse
		updating.Reason = string(ouev1alpha1.NodeUpdatePending)
		updating.Message = "The update of the node is pending"
	}

	// ATM, the insight's message is set only for the interesting cases: (isUnavailable && !isUpdating) || isDegraded
	// Moreover, the degraded message overwrites the unavailable one.
	// Those cases are inherited from the "oc adm upgrade" command as the baseline for the insight's message.
	// https://github.com/openshift/oc/blob/0cd37758b5ebb182ea911c157256c1b812c216c5/pkg/cli/admin/upgrade/status/workerpool.go#L194
	// We may add more cases in the future as needed
	var message string
	if isUnavailable && !isUpdating {
		available.Status = metav1.ConditionFalse
		// TODO: Reason should be more informative (e.g., specific unavailability type) but we will handle that in the future
		available.Reason = "Unavailable"
		available.Message = lns.GetUnavailableMessage()
		// Preserve the actual unavailability time from node state, stripping monotonic clock.
		// If the unavailability time is not known (zero value), fall back to the current time
		unavailableSince := lns.GetUnavailableSince()
		if unavailableSince.IsZero() {
			available.LastTransitionTime = metaNow
		} else {
			available.LastTransitionTime = metav1.Time{Time: unavailableSince.Truncate(0)}
		}
		message = available.Message
	}

	if isDegraded {
		degraded.Status = metav1.ConditionTrue
		// TODO: Reason should be more informative (e.g., specific degradation type) but we will handle that in the future
		degraded.Reason = "Degraded"
		degraded.Message = node.Annotations[mco.MachineConfigDaemonReasonAnnotationKey]
		message = degraded.Message
	}

	// Use meta.SetStatusCondition to properly handle LastTransitionTime
	// It only updates LastTransitionTime when the status actually changes
	meta.SetStatusCondition(&conditions, updating)

	// Handle Available condition: When a node is unavailable, we manually set LastTransitionTime
	// to preserve the actual unavailability timestamp from GetUnavailableSince() (the time when
	// the node actually became unavailable according to the Machine Config Operator).
	// This is more accurate than using meta.SetStatusCondition, which would set it to the time
	// we first detected the unavailability in our reconciliation loop.
	//
	// We cannot use meta.SetStatusCondition when we've manually set LastTransitionTime because
	// it manages that field automatically and would either overwrite our timestamp or cause
	// unnecessary updates. Therefore, we manually manage the Available condition in this case
	// by removing any existing Available condition and appending our manually-timestamped one.
	if isUnavailable && !isUpdating {
		// Remove existing Available condition and add our manually-timestamped one
		var filteredConditions []metav1.Condition
		for _, c := range conditions {
			if c.Type != string(ouev1alpha1.NodeStatusInsightAvailable) {
				filteredConditions = append(filteredConditions, c)
			}
		}
		filteredConditions = append(filteredConditions, available)
		conditions = filteredConditions
	} else {
		meta.SetStatusCondition(&conditions, available)
	}

	meta.SetStatusCondition(&conditions, degraded)

	return conditions, message, phase
}

// isNodeDraining checks if a node is currently in the draining phase.
func isNodeDraining(node *corev1.Node, isUpdating bool) bool {
	desiredDrain := node.Annotations[mco.DesiredDrainerAnnotationKey]
	appliedDrain := node.Annotations[mco.LastAppliedDrainerAnnotationKey]

	if appliedDrain == "" || desiredDrain == "" {
		return false
	}

	if desiredDrain != appliedDrain {
		desiredVerb := strings.Split(desiredDrain, "-")[0]
		if desiredVerb == mco.DrainerStateDrain {
			return true
		}
	}

	// Node is supposed to be updating but MCD hasn't had the time to update
	// its state from original `Done` to `Working` and start the drain process.
	// Default to drain process so that we don't report completed.
	mcdState := node.Annotations[mco.MachineConfigDaemonStateAnnotationKey]
	return isUpdating && mcdState == mco.MachineConfigDaemonStateDone
}
