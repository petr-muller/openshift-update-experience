package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	LabelUpdateHealthInsightManager = "oue.openshift.muller.dev/update-health-insight-manager"
)

func predicateForHealthInsightsManagedBy(controller string) predicate.Funcs {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetLabels()[LabelUpdateHealthInsightManager] == controller
	})
}
