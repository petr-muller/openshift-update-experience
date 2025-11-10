package clusterversions

import (
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/petr-muller/openshift-update-experience/internal/clusteroperators"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// IsVersion is a predicate that holds for the ClusterVersion resources called "version".
var IsVersion = predicate.NewPredicateFuncs(
	func(obj client.Object) bool {
		return obj.GetName() == "version"
	},
)

type cvDesiredVersionChanged struct {
	predicate.Funcs
}

func (p cvDesiredVersionChanged) Update(e event.UpdateEvent) bool {
	beforeObj := e.ObjectOld
	afterObj := e.ObjectNew

	before, ok := beforeObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	after, ok := afterObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	return before.Status.Desired.Version != after.Status.Desired.Version
}

type cvHistoryChanged struct {
	predicate.Funcs
}

func (p cvHistoryChanged) Update(e event.UpdateEvent) bool {
	beforeObj := e.ObjectOld
	afterObj := e.ObjectNew

	before, ok := beforeObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	after, ok := afterObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	if len(before.Status.History) == 0 && len(after.Status.History) == 0 {
		return false
	}

	if len(before.Status.History) != len(after.Status.History) {
		return true
	}

	topBefore := before.Status.History[0]
	topAfter := after.Status.History[0]

	return topBefore.State != topAfter.State ||
		topBefore.Version != topAfter.Version ||
		topBefore.Image != topAfter.Image
}

type cvProgressingChanged struct {
	predicate.Funcs
}

func (p cvProgressingChanged) Update(e event.UpdateEvent) bool {
	beforeObj := e.ObjectOld
	afterObj := e.ObjectNew

	before, ok := beforeObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}
	after, ok := afterObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	progressingBefore := clusteroperators.FindOperatorStatusCondition(before.Status.Conditions, openshiftconfigv1.OperatorProgressing)
	progressingAfter := clusteroperators.FindOperatorStatusCondition(after.Status.Conditions, openshiftconfigv1.OperatorProgressing)

	if progressingBefore == nil && progressingAfter == nil {
		return false
	}

	if progressingBefore == nil || progressingAfter == nil {
		return true
	}

	return progressingBefore.Status != progressingAfter.Status
}

// StartedOrCompletedUpdating is a predicate that holds when the ClusterVersion resource changed in way that indicates
// an update started or finished
var StartedOrCompletedUpdating = predicate.Or(cvProgressingChanged{}, cvHistoryChanged{}, cvDesiredVersionChanged{})
