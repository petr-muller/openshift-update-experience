package clusteroperators

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ignoreLastTransitionTime = cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime")

type a struct {
	metav1.Time
}

func anchor() a {
	return a{metav1.Now()}
}

func (n a) minutesAgo(minutes int) metav1.Time {
	return metav1.NewTime(n.Add(-time.Duration(minutes) * time.Minute))
}

func Test_assessClusterOperator_Conditions_Healthy(t *testing.T) {
	now := anchor()

	testCases := []struct {
		name        string
		coAvailable *openshiftconfigv1.ClusterOperatorStatusCondition
		coDegraded  *openshiftconfigv1.ClusterOperatorStatusCondition

		expected metav1.Condition
	}{
		{
			name: "Healthy=True when Available=True and Degraded=False",
			coAvailable: &openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:               openshiftconfigv1.OperatorAvailable,
				Status:             openshiftconfigv1.ConditionTrue,
				Reason:             "AsExpected",
				Message:            "All is well",
				LastTransitionTime: now.minutesAgo(15),
			},
			coDegraded: &openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:               openshiftconfigv1.OperatorDegraded,
				Status:             openshiftconfigv1.ConditionFalse,
				Reason:             "AsExpected",
				Message:            "All is well",
				LastTransitionTime: now.minutesAgo(20),
			},
			expected: metav1.Condition{
				Type:    "Healthy",
				Status:  metav1.ConditionTrue,
				Reason:  "AsExpected",
				Message: "",
			},
		},
		{
			name: "Healthy=False|Reason=Unavailable when Available=False",
			coAvailable: &openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorAvailable,
				Status:  openshiftconfigv1.ConditionFalse,
				Reason:  "Broken",
				Message: "The operator is not available",
			},
			coDegraded: &openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorDegraded,
				Status:  openshiftconfigv1.ConditionFalse,
				Reason:  "AsExpected",
				Message: "All is well",
			},
			expected: metav1.Condition{
				Type:    "Healthy",
				Status:  metav1.ConditionFalse,
				Reason:  "Unavailable",
				Message: "The operator is not available",
			},
		},
		{
			name: "Healthy=False|Reason=Unavailable when Available=False even when Degraded=True",
			coAvailable: &openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorAvailable,
				Status:  openshiftconfigv1.ConditionFalse,
				Reason:  "Broken",
				Message: "The operator is not available",
			},
			coDegraded: &openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorDegraded,
				Status:  openshiftconfigv1.ConditionTrue,
				Reason:  "AlsoBroken",
				Message: "The operator is also degraded",
			},
			expected: metav1.Condition{
				Type:    "Healthy",
				Status:  metav1.ConditionFalse,
				Reason:  "Unavailable",
				Message: "The operator is not available",
			},
		},
		{
			name: "Healthy=False|Reason=Degraded when Available=True and Degraded=True",
			coAvailable: &openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorAvailable,
				Status:  openshiftconfigv1.ConditionTrue,
				Reason:  "AsExpected",
				Message: "All is well",
			},
			coDegraded: &openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorDegraded,
				Status:  openshiftconfigv1.ConditionTrue,
				Reason:  "Broken",
				Message: "The operator is degraded",
			},
			expected: metav1.Condition{
				Type:    "Healthy",
				Status:  metav1.ConditionFalse,
				Reason:  "Degraded",
				Message: "The operator is degraded",
			},
		},
	}

	for _, tc := range testCases {
		co := &openshiftconfigv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{Name: "test-operator"},
			Status: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
		}
		if tc.coAvailable != nil {
			co.Status.Conditions = append(co.Status.Conditions, *tc.coAvailable)
		}
		if tc.coDegraded != nil {
			co.Status.Conditions = append(co.Status.Conditions, *tc.coDegraded)
		}

		insight := assessClusterOperator(context.Background(), co, "4.15.0", nil, now.Time)
		healthy := meta.FindStatusCondition(insight.Conditions, string(openshiftv1alpha1.ClusterOperatorProgressInsightHealthy))
		if healthy == nil {
			t.Fatal("assessClusterOperator() did not return expected Healthy condition")
		}

		if diff := cmp.Diff(tc.expected, *healthy, ignoreLastTransitionTime); diff != "" {
			t.Errorf("assessClusterOperator() mismatch (-want +got):\n%s", diff)
		}
	}
}

func Test_assessClusterOperator_Conditions_Updating(t *testing.T) {
	now := anchor()

	var (
		coProgressingFalse = openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionFalse,
			Reason:             "AsExpected",
			Message:            "All is well",
			LastTransitionTime: now.minutesAgo(15),
		}

		coProgressingTrue = openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionTrue,
			Reason:             "ChuggingAlong",
			Message:            "Deploying Deployments and exorcising DaemonSets",
			LastTransitionTime: now.minutesAgo(20),
		}
	)

	testCases := []struct {
		name     string
		operator openshiftconfigv1.ClusterOperatorStatus
		version  string

		expected metav1.Condition
	}{
		{
			name: "ClusterOperator is Updating=False|Reason=Completed before the update",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingFalse,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
			version: "4.15.0",
			expected: metav1.Condition{
				Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
				Status:  metav1.ConditionFalse,
				Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
				Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse.Message),
			},
		},
		{
			name: "ClusterOperator is Updating=False|Reason=Completed before the update even when Progressing=True",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingTrue,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
			version: "4.15.0",
			expected: metav1.Condition{
				Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
				Status:  metav1.ConditionFalse,
				Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
				Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue.Message),
			},
		},
		{
			name: "ClusterOperator is Updating=False|Reason=Pending after update started",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingFalse,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
			version: "4.16.0",
			expected: metav1.Condition{
				Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
				Status:  metav1.ConditionFalse,
				Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonPending),
				Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse.Message),
			},
		},
		{
			name: "ClusterOperator is Updating=True|Reason=Progressing after update started, progressing, old version",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingTrue,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
			version: "4.16.0",
			expected: metav1.Condition{
				Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
				Status:  metav1.ConditionTrue,
				Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonProgressing),
				Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue.Message),
			},
		},
		{
			name: "ClusterOperator is Updating=False|Reason=Updated after update started, progressing, new version",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingTrue,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.16.0"},
				},
			},
			version: "4.16.0",
			expected: metav1.Condition{
				Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
				Status:  metav1.ConditionFalse,
				Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
				Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue.Message),
			},
		},
		{
			name: "ClusterOperator is Updating=False|Reason=Updated after update started, not progressing, new version",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingFalse,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.16.0"},
				},
			},
			version: "4.16.0",
			expected: metav1.Condition{
				Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
				Status:  metav1.ConditionFalse,
				Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
				Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse.Message),
			},
		},
	}

	ignoreLastTransitionTime := cmpopts.IgnoreFields(
		metav1.Condition{},
		"LastTransitionTime",
	)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			co := &openshiftconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "test-operator"},
				Status:     tc.operator,
			}

			insight := assessClusterOperator(context.Background(), co, tc.version, nil, now.Time)
			updating := meta.FindStatusCondition(insight.Conditions, string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating))
			if updating == nil {
				t.Fatal("assessClusterOperator() did not return expected Updating condition")
			}

			if diff := cmp.Diff(tc.expected, *updating, ignoreLastTransitionTime); diff != "" {
				t.Errorf("assessClusterOperator() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
