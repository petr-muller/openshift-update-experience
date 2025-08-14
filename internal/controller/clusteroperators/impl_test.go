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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_assessClusterOperator(t *testing.T) {
	now := metav1.Now()

	var minutesAgo [60]metav1.Time
	for i := 0; i < 60; i++ {
		minutesAgo[i] = metav1.Time{Time: now.Add(-time.Duration(i) * time.Minute)}
	}

	var (
		coProgressingFalse415 = openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionFalse,
			Reason:             "AsExpected",
			Message:            "All is well",
			LastTransitionTime: minutesAgo[45],
		}

		coProgressingTrue415 = openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionTrue,
			Reason:             "ChuggingAlong",
			Message:            "Deploying Deployments and exorcising DaemonSets",
			LastTransitionTime: minutesAgo[45],
		}

		coProgressingTrue416 = openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionTrue,
			Reason:             "ChuggingAlong",
			Message:            "Deploying Deployments and exorcising DaemonSets for 4.16.0",
			LastTransitionTime: minutesAgo[45],
		}

		coProgressingFalse416 = openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionFalse,
			Reason:             "AsExpected",
			Message:            "All is well on 4.16.0",
			LastTransitionTime: minutesAgo[30],
		}
	)

	testCases := []struct {
		name     string
		operator openshiftconfigv1.ClusterOperatorStatus
		version  string

		expected *openshiftv1alpha1.ClusterOperatorProgressInsightStatus
	}{
		{
			name: "ClusterOperator is Updating=False|Reason=Completed before the update",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingFalse415,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
			version: "4.15.0",
			expected: &openshiftv1alpha1.ClusterOperatorProgressInsightStatus{
				Name: "test-operator",
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:  metav1.ConditionFalse,
						Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
						Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse415.Message),
					},
				},
			},
		},
		{
			name: "ClusterOperator is Updating=False|Reason=Completed before the update even when Progressing=True",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingTrue415,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
			version: "4.15.0",
			expected: &openshiftv1alpha1.ClusterOperatorProgressInsightStatus{
				Name: "test-operator",
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:  metav1.ConditionFalse,
						Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
						Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue415.Message),
					},
				},
			},
		},
		{
			name: "ClusterOperator is Updating=False|Reason=Pending after update started",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingFalse415,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
			version: "4.16.0",
			expected: &openshiftv1alpha1.ClusterOperatorProgressInsightStatus{
				Name: "test-operator",
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:  metav1.ConditionFalse,
						Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonPending),
						Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse415.Message),
					},
				},
			},
		},
		{
			name: "ClusterOperator is Updating=True|Reason=Progressing after update started, progressing, old version",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingTrue416,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.15.0"},
				},
			},
			version: "4.16.0",
			expected: &openshiftv1alpha1.ClusterOperatorProgressInsightStatus{
				Name: "test-operator",
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:  metav1.ConditionTrue,
						Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonProgressing),
						Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue416.Message),
					},
				},
			},
		},
		{
			name: "ClusterOperator is Updating=False|Reason=Updated after update started, progressing, new version",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingTrue416,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.16.0"},
				},
			},
			version: "4.16.0",
			expected: &openshiftv1alpha1.ClusterOperatorProgressInsightStatus{
				Name: "test-operator",
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:  metav1.ConditionFalse,
						Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
						Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue416.Message),
					},
				},
			},
		},
		{
			name: "ClusterOperator is Updating=False|Reason=Updated after update started, not progressing, new version",
			operator: openshiftconfigv1.ClusterOperatorStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					coProgressingFalse416,
				},
				Versions: []openshiftconfigv1.OperandVersion{
					{Name: "operator", Version: "4.16.0"},
				},
			},
			version: "4.16.0",
			expected: &openshiftv1alpha1.ClusterOperatorProgressInsightStatus{
				Name: "test-operator",
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:  metav1.ConditionFalse,
						Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
						Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse416.Message),
					},
				},
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
			result := assessClusterOperator(context.Background(), co, tc.version, nil, now)
			if diff := cmp.Diff(tc.expected, result, ignoreLastTransitionTime); diff != "" {
				t.Errorf("assessClusterOperator() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
