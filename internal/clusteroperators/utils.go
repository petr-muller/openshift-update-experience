package clusteroperators

import openshiftconfigv1 "github.com/openshift/api/config/v1"

func FindOperatorStatusCondition(conditions []openshiftconfigv1.ClusterOperatorStatusCondition, conditionType openshiftconfigv1.ClusterStatusConditionType) *openshiftconfigv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}
