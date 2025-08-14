package health

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	InsightManagerLabel = "oue.openshift.muller.dev/update-health-insight-manager"
)

func PredicateForInsightsManagedBy(controller string) predicate.Funcs {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetLabels()[InsightManagerLabel] == controller
	})
}
