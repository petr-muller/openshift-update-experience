package nodestate

import (
	"time"

	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// StateEvaluator defines the interface for evaluating node update state.
// This interface allows for testing and decoupling state evaluation from the controller.
type StateEvaluator interface {
	// EvaluateNode computes the current state of a node's update progress.
	// It returns nil if the node or pool is nil.
	EvaluateNode(node *corev1.Node, pool *openshiftmachineconfigurationv1.MachineConfigPool,
		machineConfigToVersion func(string) (string, bool),
		desiredVersion string,
		existingConditions []metav1.Condition,
		now time.Time) *NodeState
}

// DefaultStateEvaluator implements StateEvaluator using the standard evaluation logic.
type DefaultStateEvaluator struct{}

// NewDefaultStateEvaluator creates a new DefaultStateEvaluator.
func NewDefaultStateEvaluator() *DefaultStateEvaluator {
	return &DefaultStateEvaluator{}
}

// EvaluateNode computes the current state of a node's update progress.
// It evaluates the node's annotations, MCP configuration, and determines:
//   - Version information (current and desired)
//   - Update phase (Pending, Draining, Updating, Rebooting, Paused, Completed)
//   - Conditions (Updating, Available, Degraded)
func (e *DefaultStateEvaluator) EvaluateNode(
	node *corev1.Node,
	pool *openshiftmachineconfigurationv1.MachineConfigPool,
	machineConfigToVersion func(string) (string, bool),
	desiredVersion string,
	existingConditions []metav1.Condition,
	now time.Time,
) *NodeState {
	if node == nil || pool == nil {
		return nil
	}

	desiredConfig, ok := node.Annotations[mco.DesiredMachineConfigAnnotationKey]
	noDesiredOnNode := !ok
	currentConfig := node.Annotations[mco.CurrentMachineConfigAnnotationKey]
	currentVersion, foundCurrent := machineConfigToVersion(currentConfig)
	desiredVersionFromMC, foundDesired := machineConfigToVersion(desiredConfig)

	lns := mco.NewLayeredNodeState(node)
	isUnavailable := lns.IsUnavailable(pool)

	isDegraded := isNodeDegraded(node)
	isUpdated := foundCurrent && desiredVersion == currentVersion &&
		// The following condition is to handle the multi-arch migration because the version number stays the same there
		(noDesiredOnNode || currentConfig == desiredConfig)

	// foundCurrent makes sure we don't blip phase "updating" for nodes that we are not sure
	// of their actual phase, even though the conservative assumption is that the node is
	// at least updating or is updated.
	isUpdating := !isUpdated && foundCurrent && foundDesired && desiredVersion == desiredVersionFromMC

	conditions, message, phase := DetermineConditions(pool, node, isUpdating, isUpdated, isUnavailable, isDegraded, lns, existingConditions, now)

	scope := ouev1alpha1.WorkerPoolScope
	if pool.Name == mco.MachineConfigPoolMaster {
		scope = ouev1alpha1.ControlPlaneScope
	}

	state := &NodeState{
		Name: node.Name,
		UID:  node.UID,
		PoolRef: ouev1alpha1.ResourceRef{
			Resource: "machineconfigpools",
			Group:    openshiftmachineconfigurationv1.GroupName,
			Name:     pool.Name,
		},
		Scope:          scope,
		Version:        currentVersion,
		DesiredVersion: desiredVersion,
		CurrentConfig:  currentConfig,
		DesiredConfig:  desiredConfig,
		Conditions:     conditions,
		Phase:          phase,
		Message:        message,
		LastEvaluated:  now,
	}

	state.StateHash = state.ComputeHash()
	return state
}

// isNodeDegraded checks if a node is in a degraded state based on MCD annotations.
func isNodeDegraded(node *corev1.Node) bool {
	// Inspired by: https://github.com/openshift/machine-config-operator/blob/master/pkg/controller/node/status.go
	if node.Annotations == nil {
		return false
	}
	dconfig, ok := node.Annotations[mco.DesiredMachineConfigAnnotationKey]
	if !ok || dconfig == "" {
		return false
	}
	dstate, ok := node.Annotations[mco.MachineConfigDaemonStateAnnotationKey]
	if !ok || dstate == "" {
		return false
	}

	if dstate == mco.MachineConfigDaemonStateDegraded || dstate == mco.MachineConfigDaemonStateUnreconcilable {
		return true
	}
	return false
}

// AssessNodeForInsight evaluates a node and returns the insight status.
// This function maintains backward compatibility with existing code by returning
// the insight status format used by NodeProgressInsight CRDs.
// It wraps EvaluateNode and converts the result to NodeProgressInsightStatus.
func AssessNodeForInsight(
	node *corev1.Node,
	mcp *openshiftmachineconfigurationv1.MachineConfigPool,
	machineConfigToVersion func(string) (string, bool),
	mostRecentVersionInCVHistory string,
	existingConditions []metav1.Condition,
	now metav1.Time,
) *ouev1alpha1.NodeProgressInsightStatus {
	evaluator := NewDefaultStateEvaluator()
	state := evaluator.EvaluateNode(node, mcp, machineConfigToVersion, mostRecentVersionInCVHistory, existingConditions, now.Time)

	if state == nil {
		return nil
	}

	return ConvertStateToInsight(state)
}

// ConvertStateToInsight converts internal NodeState to the CRD status format.
func ConvertStateToInsight(state *NodeState) *ouev1alpha1.NodeProgressInsightStatus {
	if state == nil {
		return nil
	}

	// Derive estimate from phase - this maintains backward compatibility
	estimate := EstimateFromPhase(state.Phase, state.Conditions)

	return &ouev1alpha1.NodeProgressInsightStatus{
		Name:                state.Name,
		PoolResource:        state.PoolRef,
		Scope:               state.Scope,
		Version:             state.Version,
		EstimatedToComplete: estimate,
		Message:             state.Message,
		Conditions:          state.Conditions,
	}
}

// EstimateFromPhase derives an estimated completion time from the update phase.
// Returns nil if completion cannot be estimated (degraded/unavailable states).
func EstimateFromPhase(phase UpdatePhase, conditions []metav1.Condition) *metav1.Duration {
	// Check for degraded or unavailable conditions - no estimate possible
	for _, c := range conditions {
		if c.Type == string(ouev1alpha1.NodeStatusInsightDegraded) && c.Status == metav1.ConditionTrue {
			return nil
		}
		if c.Type == string(ouev1alpha1.NodeStatusInsightAvailable) && c.Status == metav1.ConditionFalse {
			return nil
		}
	}

	switch phase {
	case UpdatePhaseCompleted:
		return &metav1.Duration{Duration: 0}
	case UpdatePhasePending, UpdatePhasePaused:
		// No estimate for pending - we don't know when it will start
		return nil
	case UpdatePhaseDraining, UpdatePhaseUpdating, UpdatePhaseRebooting:
		return &metav1.Duration{Duration: 10 * time.Minute}
	default:
		return nil
	}
}

// NodeStateToUID extracts the UID for quick lookups.
func NodeStateToUID(state *NodeState) types.UID {
	if state == nil {
		return ""
	}
	return state.UID
}
