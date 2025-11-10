package clusterversions

import (
	"testing"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func Test_IsVersion(t *testing.T) {
	testCases := []struct {
		name     string
		objName  string
		expected bool
	}{
		{
			name:     "version object",
			objName:  "version",
			expected: true,
		},
		{
			name:     "other object",
			objName:  "other",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock object with the test name
			obj := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.objName,
				},
			}

			// Test the predicate function directly
			result := IsVersion.Create(event.CreateEvent{Object: obj})
			if result != tc.expected {
				t.Errorf("cvVersion.Create() = %v, expected %v", result, tc.expected)
			}

			result = IsVersion.Update(event.UpdateEvent{ObjectOld: obj, ObjectNew: obj})
			if result != tc.expected {
				t.Errorf("cvVersion.Update() = %v, expected %v", result, tc.expected)
			}

			result = IsVersion.Delete(event.DeleteEvent{Object: obj})
			if result != tc.expected {
				t.Errorf("cvVersion.Delete() = %v, expected %v", result, tc.expected)
			}

			result = IsVersion.Generic(event.GenericEvent{Object: obj})
			if result != tc.expected {
				t.Errorf("cvVersion.Generic() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func Test_cvProgressingChanged(t *testing.T) {
	testCases := []struct {
		name     string
		before   *openshiftconfigv1.ClusterVersion
		after    *openshiftconfigv1.ClusterVersion
		expected bool
	}{
		{
			name:     "both nil progressing conditions",
			before:   &openshiftconfigv1.ClusterVersion{},
			after:    &openshiftconfigv1.ClusterVersion{},
			expected: false,
		},
		{
			name:   "progressing added",
			before: &openshiftconfigv1.ClusterVersion{},
			after: &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						{
							Type:   openshiftconfigv1.OperatorProgressing,
							Status: openshiftconfigv1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "progressing removed",
			before: &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						{
							Type:   openshiftconfigv1.OperatorProgressing,
							Status: openshiftconfigv1.ConditionTrue,
						},
					},
				},
			},
			after:    &openshiftconfigv1.ClusterVersion{},
			expected: true,
		},
		{
			name: "progressing status changed",
			before: &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						{
							Type:   openshiftconfigv1.OperatorProgressing,
							Status: openshiftconfigv1.ConditionFalse,
						},
					},
				},
			},
			after: &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						{
							Type:   openshiftconfigv1.OperatorProgressing,
							Status: openshiftconfigv1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "progressing status unchanged",
			before: &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						{
							Type:   openshiftconfigv1.OperatorProgressing,
							Status: openshiftconfigv1.ConditionTrue,
						},
					},
				},
			},
			after: &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						{
							Type:   openshiftconfigv1.OperatorProgressing,
							Status: openshiftconfigv1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			predicate := cvProgressingChanged{}
			result := predicate.Update(event.UpdateEvent{
				ObjectOld: tc.before,
				ObjectNew: tc.after,
			})
			if result != tc.expected {
				t.Errorf("cvProgressingChanged.Update() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func Test_cvHistoryChanged(t *testing.T) {
	testCases := []struct {
		name     string
		before   []openshiftconfigv1.UpdateHistory
		after    []openshiftconfigv1.UpdateHistory
		expected bool
	}{
		{
			name:     "both empty history",
			expected: false,
		},
		{
			name: "history added",
			after: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				},
			},
			expected: true,
		},
		{
			name: "new history entry added",
			before: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				},
			},
			after: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.PartialUpdate,
					Version: "4.16.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
				},
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				},
			},
			expected: true,
		},
		{
			name: "top history state changed",
			before: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.PartialUpdate,
					Version: "4.16.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
				},
			},
			after: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.16.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
				},
			},
			expected: true,
		},
		{
			name: "top history version changed",
			before: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				},
			},
			after: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.16.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				},
			},
			expected: true,
		},
		{
			name: "top history image changed",
			before: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				},
			},
			after: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-aarch64",
				},
			},
			expected: true,
		},
		{
			name: "top history unchanged",
			before: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				},
			},
			after: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
					Image:   "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			beforeCV := &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					History: tc.before,
				},
			}
			afterCV := &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					History: tc.after,
				},
			}

			predicate := cvHistoryChanged{}
			result := predicate.Update(event.UpdateEvent{
				ObjectOld: beforeCV,
				ObjectNew: afterCV,
			})
			if result != tc.expected {
				t.Errorf("cvHistoryChanged.Update() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func Test_cvDesiredVersionChanged(t *testing.T) {
	testCases := []struct {
		name     string
		before   string
		after    string
		expected bool
	}{
		{
			name:     "both empty version",
			expected: false,
		},
		{
			name:     "version added",
			after:    "4.15.0",
			expected: true,
		},
		{
			name:     "version changed",
			before:   "4.15.0",
			after:    "4.16.0",
			expected: true,
		},
		{
			name:     "version unchanged",
			before:   "4.15.0",
			after:    "4.15.0",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			beforeCV := &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					Desired: openshiftconfigv1.Release{
						Version: tc.before,
					},
				},
			}
			afterCV := &openshiftconfigv1.ClusterVersion{
				Status: openshiftconfigv1.ClusterVersionStatus{
					Desired: openshiftconfigv1.Release{
						Version: tc.after,
					},
				},
			}

			predicate := cvDesiredVersionChanged{}
			result := predicate.Update(event.UpdateEvent{
				ObjectOld: beforeCV,
				ObjectNew: afterCV,
			})
			if result != tc.expected {
				t.Errorf("cvDesiredVersionChanged.Update() = %v, expected %v", result, tc.expected)
			}
		})
	}
}
