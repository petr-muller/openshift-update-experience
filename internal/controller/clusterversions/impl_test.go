package clusterversions

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	v41815 = "4.18.15"
	v41816 = "4.18.16"
)

var ignoreLastTransitionTime = cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime")

func cvProgressing(status bool, version string, ltt metav1.Time) openshiftconfigv1.ClusterOperatorStatusCondition {
	if status {
		return openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionTrue,
			LastTransitionTime: ltt,
			Message:            fmt.Sprintf("Working towards %s: 106 of 863 done (12%% complete), waiting on etcd, kube-apiserver", version),
		}
	}
	return openshiftconfigv1.ClusterOperatorStatusCondition{
		Type:               openshiftconfigv1.OperatorProgressing,
		Status:             openshiftconfigv1.ConditionFalse,
		LastTransitionTime: ltt,
		Message:            fmt.Sprintf("Cluster version is %s", version),
	}
}

func cvHistory(completed bool, version string, started metav1.Time, completion *metav1.Time) openshiftconfigv1.UpdateHistory {
	uh := openshiftconfigv1.UpdateHistory{
		StartedTime:    started,
		CompletionTime: completion,
		Version:        version,
		Image:          fmt.Sprintf("quay.io/something/openshift-release:%s-x86_64", version),
		State:          openshiftconfigv1.PartialUpdate,
	}
	if completed {
		uh.State = openshiftconfigv1.CompletedUpdate
	}

	return uh
}

func instantiateClusterOperator(name string, versions ...string) openshiftconfigv1.ClusterOperator {
	if len(versions)%2 != 0 {
		panic("versions must be even, each version must have a name and a version")
	}

	co := openshiftconfigv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     openshiftconfigv1.ClusterOperatorStatus{},
	}

	for i := 0; i < len(versions); i += 2 {
		co.Status.Versions = append(co.Status.Versions, openshiftconfigv1.OperandVersion{Name: versions[i], Version: versions[i+1]})
	}
	return co
}

func installing15(now time.Time) func(cv *openshiftconfigv1.ClusterVersion) {
	minutesAgo := func(m int32) metav1.Time {
		return metav1.NewTime(now.Add(-(time.Duration(m)) * time.Minute))
	}

	return func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			cvProgressing(true, v41815, minutesAgo(30)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			cvHistory(false, v41815, minutesAgo(30), nil),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}
}

func installed15(now time.Time) func(cv *openshiftconfigv1.ClusterVersion) {
	minutesAgo := func(m int32) metav1.Time {
		return metav1.NewTime(now.Add(-(time.Duration(m)) * time.Minute))
	}

	return func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			cvProgressing(false, v41815, minutesAgo(30)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			cvHistory(true, v41815, minutesAgo(50), ptr.To(minutesAgo(40))),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}
}

func updating15to16(now time.Time) func(cv *openshiftconfigv1.ClusterVersion) {
	minutesAgo := func(m int32) metav1.Time {
		return metav1.NewTime(now.Add(-(time.Duration(m)) * time.Minute))
	}

	return func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			cvProgressing(true, v41816, minutesAgo(30)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			cvHistory(false, v41816, minutesAgo(30), nil),
			cvHistory(true, v41815, minutesAgo(50), ptr.To(minutesAgo(40))),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}
}

func updated15to16(now time.Time) func(cv *openshiftconfigv1.ClusterVersion) {
	minutesAgo := func(m int32) metav1.Time {
		return metav1.NewTime(now.Add(-(time.Duration(m)) * time.Minute))
	}

	return func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			cvProgressing(false, v41816, minutesAgo(30)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			cvHistory(true, v41816, minutesAgo(30), ptr.To(minutesAgo(20))),
			cvHistory(true, v41815, minutesAgo(50), ptr.To(minutesAgo(40))),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}
}

func updatingPartial15to16(now time.Time) func(cv *openshiftconfigv1.ClusterVersion) {
	minutesAgo := func(m int32) metav1.Time {
		return metav1.NewTime(now.Add(-(time.Duration(m)) * time.Minute))
	}

	return func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			cvProgressing(true, v41816, minutesAgo(30)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			cvHistory(false, v41816, minutesAgo(30), nil),
			cvHistory(false, v41815, minutesAgo(50), nil),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}
}

func updatedPartial15to16(now time.Time) func(cv *openshiftconfigv1.ClusterVersion) {
	minutesAgo := func(m int32) metav1.Time {
		return metav1.NewTime(now.Add(-(time.Duration(m)) * time.Minute))
	}

	return func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			cvProgressing(false, v41816, minutesAgo(30)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			cvHistory(true, v41816, minutesAgo(30), ptr.To(minutesAgo(20))),
			cvHistory(false, v41815, minutesAgo(50), nil),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}
}

func Test_ProgressingToUpdating(t *testing.T) {
	testCases := []struct {
		name            string
		cvProgressing   openshiftconfigv1.ClusterOperatorStatusCondition
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name: "updating cluster is progressing=true",
			cvProgressing: openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorProgressing,
				Status:  openshiftconfigv1.ConditionTrue,
				Reason:  "UsuallyEmpty",
				Message: "Working towards 4.18.15",
			},
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  "Progressing",
			expectedMessage: "ClusterVersion has Progressing=True(Reason=UsuallyEmpty) | Message='Working towards 4.18.15'",
		},
		{
			name: "updated cluster is progressing=false",
			cvProgressing: openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorProgressing,
				Status:  openshiftconfigv1.ConditionFalse,
				Reason:  "AlsoUsuallyEmpty",
				Message: "Cluster version is 4.18.15",
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  "NotProgressing",
			expectedMessage: "ClusterVersion has Progressing=False(Reason=AlsoUsuallyEmpty) | Message='Cluster version is 4.18.15'",
		},
		{
			name: "bad cluster may have progressing=unknown",
			cvProgressing: openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorProgressing,
				Status:  openshiftconfigv1.ConditionUnknown,
				Reason:  "UnknownReason",
				Message: "Cluster version is in an unknown state",
			},
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  "CannotDetermineUpdating",
			expectedMessage: "ClusterVersion has Progressing=Unknown(Reason=UnknownReason) | Message='Cluster version is in an unknown state'",
		},
		{
			name: "status is just string so it may be whatever",
			cvProgressing: openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:    openshiftconfigv1.OperatorProgressing,
				Status:  "WhatEver",
				Reason:  "SomeReason",
				Message: "Someone is doing funky things",
			},
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  "CannotDetermineUpdating",
			expectedMessage: "ClusterVersion has Progressing=WhatEver(Reason=SomeReason) | Message='Someone is doing funky things'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, reason, message := cvProgressingToUpdating(tc.cvProgressing)

			if status != tc.expectedStatus {
				t.Errorf("expected status %v, got %v", tc.expectedStatus, status)
			}
			if reason != tc.expectedReason {
				t.Errorf("expected reason %s, got %s", tc.expectedReason, reason)
			}
			if message != tc.expectedMessage {
				t.Errorf("expected message %s, got %s", tc.expectedMessage, message)
			}
		})
	}
}

func Test_IsControlPlaneUpdating(t *testing.T) {
	now := metav1.Now()
	var minutesAge [60]metav1.Time
	for i := 0; i < 60; i++ {
		minutesAge[i] = metav1.Time{Time: now.Add(-time.Duration(i) * time.Minute)}
	}

	completedHistoryItem := openshiftconfigv1.UpdateHistory{
		State:          openshiftconfigv1.CompletedUpdate,
		StartedTime:    minutesAge[50],
		CompletionTime: &minutesAge[20],
		Version:        v41815,
		Image:          "quay.io/something/openshift-release:4.18.15-x86_64",
	}

	partialHistoryItem := openshiftconfigv1.UpdateHistory{
		State:       openshiftconfigv1.PartialUpdate,
		StartedTime: minutesAge[50],
		Version:     v41815,
		Image:       "quay.io/something/openshift-release:4.18.15-x86_64",
	}

	completedProgressing := openshiftconfigv1.ClusterOperatorStatusCondition{
		Type:               openshiftconfigv1.OperatorProgressing,
		Status:             openshiftconfigv1.ConditionFalse,
		Message:            "Cluster version is 4.18.15",
		LastTransitionTime: minutesAge[20],
	}

	progressing := openshiftconfigv1.ClusterOperatorStatusCondition{
		Type:               openshiftconfigv1.OperatorProgressing,
		Status:             openshiftconfigv1.ConditionTrue,
		Message:            "Working towards 4.18.15",
		LastTransitionTime: minutesAge[50],
	}

	testCases := []struct {
		name            string
		cvProgressing   *openshiftconfigv1.ClusterOperatorStatusCondition
		lastHistoryItem *openshiftconfigv1.UpdateHistory

		expectedUpdating    metav1.Condition
		expectedStartedAt   metav1.Time
		expectedCompletedAt metav1.Time
	}{
		{
			name:            "absent progressing condition",
			cvProgressing:   nil,
			lastHistoryItem: &completedHistoryItem,
			expectedUpdating: metav1.Condition{
				Type:    "Updating",
				Status:  "Unknown",
				Reason:  "CannotDetermineUpdating",
				Message: "No Progressing condition in ClusterVersion",
			},
		},
		{
			name:            "absent last history item",
			cvProgressing:   &completedProgressing,
			lastHistoryItem: nil,
			expectedUpdating: metav1.Condition{
				Type:    "Updating",
				Status:  "Unknown",
				Reason:  "CannotDetermineUpdating",
				Message: "Empty history in ClusterVersion",
			},
		},
		{
			name:            "mismatch between Progressing=True and completed history item",
			cvProgressing:   &progressing,
			lastHistoryItem: &completedHistoryItem,
			expectedUpdating: metav1.Condition{
				Type:    "Updating",
				Status:  "Unknown",
				Reason:  "CannotDetermineUpdating",
				Message: "Progressing=True in ClusterVersion but last history item is not Partial",
			},
		},
		{
			name:          "mismatch between Progressing=True and history item with completion time",
			cvProgressing: &progressing,
			lastHistoryItem: func(item *openshiftconfigv1.UpdateHistory) *openshiftconfigv1.UpdateHistory {
				modified := item.DeepCopy()
				modified.CompletionTime = &minutesAge[20]
				return modified
			}(&partialHistoryItem),
			expectedUpdating: metav1.Condition{
				Type:    "Updating",
				Status:  "Unknown",
				Reason:  "CannotDetermineUpdating",
				Message: "Progressing=True in ClusterVersion but last history item has completion time",
			},
		},
		{
			name:            "cluster is updating",
			cvProgressing:   &progressing,
			lastHistoryItem: &partialHistoryItem,
			expectedUpdating: metav1.Condition{
				Type:    "Updating",
				Status:  "True",
				Reason:  "Progressing",
				Message: "ClusterVersion has Progressing=True(Reason=) | Message='Working towards 4.18.15'",
			},
			expectedStartedAt:   minutesAge[50],
			expectedCompletedAt: metav1.Time{}, // No completion time for partial update
		},
		{
			name:            "mismatch between Progressing=False and partial history item",
			cvProgressing:   &completedProgressing,
			lastHistoryItem: &partialHistoryItem,
			expectedUpdating: metav1.Condition{
				Type:    "Updating",
				Status:  "Unknown",
				Reason:  "CannotDetermineUpdating",
				Message: "Progressing=False in ClusterVersion but last history item is not completed",
			},
		},
		{
			name:          "mismatch between Progressing=False and history item without completion time",
			cvProgressing: &completedProgressing,
			lastHistoryItem: func(item *openshiftconfigv1.UpdateHistory) *openshiftconfigv1.UpdateHistory {
				modified := item.DeepCopy()
				modified.CompletionTime = nil // No completion time
				return modified
			}(&completedHistoryItem),
			expectedUpdating: metav1.Condition{
				Type:    "Updating",
				Status:  "Unknown",
				Reason:  "CannotDetermineUpdating",
				Message: "Progressing=False in ClusterVersion but no completion in last history item",
			},
		},
		{
			name:            "cluster is not updating",
			cvProgressing:   &completedProgressing,
			lastHistoryItem: &completedHistoryItem,
			expectedUpdating: metav1.Condition{
				Type:    "Updating",
				Status:  "False",
				Reason:  "NotProgressing",
				Message: "ClusterVersion has Progressing=False(Reason=) | Message='Cluster version is 4.18.15'",
			},
			expectedStartedAt:   minutesAge[50],
			expectedCompletedAt: minutesAge[20],
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			updating, startedAt, completedAt := isControlPlaneUpdating(tc.cvProgressing, tc.lastHistoryItem)

			if diff := cmp.Diff(tc.expectedUpdating, updating); diff != "" {
				t.Errorf("expected updating condition mismatch (-want +got):\n%s", diff)
			}
			if !startedAt.Equal(&tc.expectedStartedAt) {
				t.Errorf("expected started at %v, got %v", tc.expectedStartedAt, startedAt)
			}
			if !completedAt.Equal(&tc.expectedCompletedAt) {
				t.Errorf("expected completed at %v, got %v", tc.expectedCompletedAt, completedAt)
			}
		})
	}
}

func Test_VersionsFromHistory(t *testing.T) {
	now := metav1.Now()
	var minutesAgo [60]metav1.Time
	for i := 0; i < 60; i++ {
		minutesAgo[i] = metav1.Time{Time: now.Add(-time.Duration(i) * time.Minute)}
	}

	completed := openshiftconfigv1.UpdateHistory{
		State:          openshiftconfigv1.CompletedUpdate,
		StartedTime:    minutesAgo[50],
		CompletionTime: &minutesAgo[15],
		Version:        v41815,
		Image:          "quay.io/something/openshift-release:4.18.15-x86_64",
	}

	partial := openshiftconfigv1.UpdateHistory{
		State:       openshiftconfigv1.PartialUpdate,
		StartedTime: minutesAgo[5],
		Version:     v41815,
		Image:       "quay.io/something/openshift-release:4.18.15-x86_64",
	}

	completed16 := openshiftconfigv1.UpdateHistory{
		State:          openshiftconfigv1.CompletedUpdate,
		StartedTime:    minutesAgo[5],
		CompletionTime: &minutesAgo[2],
		Version:        v41816,
		Image:          "quay.io/something/openshift-release:4.18.16-x86_64",
	}

	testCases := []struct {
		name     string
		history  []openshiftconfigv1.UpdateHistory
		expected openshiftv1alpha1.ControlPlaneUpdateVersions
	}{
		{
			name: "no history",
		},
		{
			name:    "empty history",
			history: []openshiftconfigv1.UpdateHistory{},
		},
		{
			name:    "single history item",
			history: []openshiftconfigv1.UpdateHistory{completed},
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target: openshiftv1alpha1.Version{
					Version: v41815,
					Metadata: []openshiftv1alpha1.VersionMetadata{
						{
							Key: openshiftv1alpha1.InstallationMetadata,
						},
					},
				},
			},
		},
		{
			name:    "update from completed",
			history: []openshiftconfigv1.UpdateHistory{completed16, completed},
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target: openshiftv1alpha1.Version{
					Version: v41816,
				},
				Previous: &openshiftv1alpha1.Version{
					Version: v41815,
				},
			},
		},
		{
			name:    "update from partial",
			history: []openshiftconfigv1.UpdateHistory{completed16, partial},
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target: openshiftv1alpha1.Version{
					Version: v41816,
				},
				Previous: &openshiftv1alpha1.Version{
					Version: v41815,
					Metadata: []openshiftv1alpha1.VersionMetadata{
						{
							Key: openshiftv1alpha1.PartialMetadata,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := versionsFromHistory(tc.history)
			if diff := cmp.Diff(tc.expected, actual); diff != "" {
				t.Errorf("expected versions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_EstimateCompletion(t *testing.T) {
	testCases := []struct {
		name                   string
		baseline               time.Duration
		toLastObservedProgress time.Duration
		updatingFor            time.Duration
		completion             float64

		expected time.Duration
	}{
		{
			name:                   "estimate is zero when completed",
			baseline:               time.Hour,
			toLastObservedProgress: 80 * time.Minute,
			updatingFor:            85 * time.Minute,
			completion:             1,

			expected: 0,
		},
		{
			name:                   "estimate uses baseline when no progress was observed yet",
			baseline:               70 * time.Minute,
			toLastObservedProgress: 0,
			updatingFor:            10 * time.Minute,
			completion:             0,

			expected: 72 * time.Minute, // 120% of 60m (70m baseline - 10m elapsed)
		},
		{
			name:                   "estimate uses baseline early in the update",
			baseline:               63 * time.Minute,
			toLastObservedProgress: 2 * time.Minute,
			updatingFor:            3 * time.Minute,
			completion:             0.03, // 3% complete

			expected: 72 * time.Minute, // 120% of 60m (63m baseline - 3m elapsed)
		},
		{
			name:                   "estimate is projected from last observed progress",
			baseline:               60 * time.Minute,
			toLastObservedProgress: 40 * time.Minute,
			updatingFor:            50 * time.Minute,
			completion:             0.97,

			// This is looks a bit arbitrary which is caused by timewiseComplete approximation
			// 97% completed is ~69% timewise so if 97% is done in 40m, the projection is ~57m20s
			// At 50m this means 7m20s remains, which is overestimated by 20% to ~8m45s
			expected: 8*time.Minute + 45*time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := estimateCompletion(tc.baseline, tc.toLastObservedProgress, tc.updatingFor, tc.completion)
			if diff := cmp.Diff(tc.expected, actual); diff != "" {
				t.Errorf("expected completion time mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_ForcedHealthInsight(t *testing.T) {
	now := metav1.Now()

	testCases := []struct {
		name        string
		Annotations map[string]string
		expected    *openshiftv1alpha1.UpdateHealthInsightStatus
	}{
		{
			name: "CV not annotated -> no forced insight",
		},
		{
			name:        "CV annotated with other Annotations -> no forced insight",
			Annotations: map[string]string{"some-other-annotation": "value"},
		},
		{
			name:        "CV with force insight annotation -> forced insight",
			Annotations: map[string]string{uscForceHealthInsightAnnotation: "yes"},
			expected: &openshiftv1alpha1.UpdateHealthInsightStatus{
				StartedAt: now,
				Scope: openshiftv1alpha1.InsightScope{
					Type: "ControlPlane",
					Resources: []openshiftv1alpha1.ResourceRef{
						{
							Group:    "config.openshift.io",
							Resource: "clusterversions",
							Name:     "version",
						},
					},
				},
				Impact: openshiftv1alpha1.InsightImpact{
					Level:       "Info",
					Type:        "None",
					Summary:     "Forced health insight for ClusterVersion version",
					Description: `The resource has a "oue.openshift.muller.dev/force-health-insight" annotation which forces USC to generate this health insight for testing purposes.`,
				},
				Remediation: openshiftv1alpha1.InsightRemediation{
					Reference: "https://issues.redhat.com/browse/OTA-1418",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "version",
					Annotations: tc.Annotations,
				},
			}

			forced := forcedHealthInsight(cv, now)
			if diff := cmp.Diff(tc.expected, forced); diff != "" {
				t.Errorf("expected forced health insight mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_AssessClusterVersion_Conditions(t *testing.T) {
	t.Parallel()

	anchor := time.Now()

	testCases := []struct {
		name     string
		expected []metav1.Condition

		mutateCV func(*openshiftconfigv1.ClusterVersion)
	}{
		{
			name: "Cluster is not updating",
			expected: []metav1.Condition{
				{
					Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
					Status:  metav1.ConditionFalse,
					Reason:  string(openshiftv1alpha1.ClusterVersionNotProgressing),
					Message: "ClusterVersion has Progressing=False(Reason=) | Message='Cluster version is 4.18.15'",
				},
			},
			mutateCV: installed15(anchor),
		},
		{
			name: "Cluster is updating",
			expected: []metav1.Condition{
				{
					Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
					Status:  metav1.ConditionTrue,
					Reason:  string(openshiftv1alpha1.ClusterVersionProgressing),
					Message: "ClusterVersion has Progressing=True(Reason=) | Message='Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver'",
				},
			},
			mutateCV: updating15to16(anchor),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     openshiftconfigv1.ClusterVersionStatus{},
			}

			tc.mutateCV(cv)
			var previous openshiftv1alpha1.ClusterVersionProgressInsightStatus
			cos := &openshiftconfigv1.ClusterOperatorList{
				Items: []openshiftconfigv1.ClusterOperator{
					instantiateClusterOperator("etcd", "operator", v41815),
					instantiateClusterOperator("kube-apiserver", "operator", v41815),
				},
			}

			insight, _ := assessClusterVersion(cv, &previous, cos)
			if diff := cmp.Diff(tc.expected, insight.Conditions, ignoreLastTransitionTime); diff != "" {
				t.Errorf("expected conditions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_AssessClusterVersion_Assessment(t *testing.T) {
	t.Parallel()

	anchor := time.Now()

	testCases := []struct {
		name     string
		expected openshiftv1alpha1.ClusterVersionAssessment

		mutateCV func(*openshiftconfigv1.ClusterVersion)
	}{
		{
			name:     "Cluster is not updating",
			expected: openshiftv1alpha1.ClusterVersionAssessmentCompleted,
			mutateCV: installed15(anchor),
		},
		{
			name:     "Cluster is updating",
			expected: openshiftv1alpha1.ClusterVersionAssessmentProgressing,
			mutateCV: updating15to16(anchor),
		},
		// TODO: degraded / slow
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     openshiftconfigv1.ClusterVersionStatus{},
			}

			tc.mutateCV(cv)
			var previous openshiftv1alpha1.ClusterVersionProgressInsightStatus
			cos := &openshiftconfigv1.ClusterOperatorList{
				Items: []openshiftconfigv1.ClusterOperator{
					instantiateClusterOperator("etcd", "operator", v41815),
					instantiateClusterOperator("kube-apiserver", "operator", v41815),
				},
			}

			insight, _ := assessClusterVersion(cv, &previous, cos)
			if diff := cmp.Diff(tc.expected, insight.Assessment); diff != "" {
				t.Errorf("expected assessment mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_AssessClusterVersion_Versions(t *testing.T) {
	t.Parallel()

	anchor := time.Now()

	testCases := []struct {
		name     string
		expected openshiftv1alpha1.ControlPlaneUpdateVersions

		mutateCV func(*openshiftconfigv1.ClusterVersion)
	}{
		{
			name: "Installation in progress",
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target: openshiftv1alpha1.Version{
					Version:  v41815,
					Metadata: []openshiftv1alpha1.VersionMetadata{{Key: openshiftv1alpha1.InstallationMetadata}},
				},
			},
			mutateCV: installing15(anchor),
		},
		{
			name: "Installation completed",
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target: openshiftv1alpha1.Version{
					Version:  v41815,
					Metadata: []openshiftv1alpha1.VersionMetadata{{Key: openshiftv1alpha1.InstallationMetadata}},
				},
			},
			mutateCV: installed15(anchor),
		},
		{
			name: "Update in progress",
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target:   openshiftv1alpha1.Version{Version: v41816},
				Previous: &openshiftv1alpha1.Version{Version: v41815},
			},
			mutateCV: updating15to16(anchor),
		},
		{
			name: "Update completed",
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target:   openshiftv1alpha1.Version{Version: v41816},
				Previous: &openshiftv1alpha1.Version{Version: v41815},
			},
			mutateCV: updated15to16(anchor),
		},
		{
			name: "Update from partial in progress",
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target: openshiftv1alpha1.Version{Version: v41816},
				Previous: &openshiftv1alpha1.Version{
					Version:  v41815,
					Metadata: []openshiftv1alpha1.VersionMetadata{{Key: openshiftv1alpha1.PartialMetadata}},
				},
			},
			mutateCV: updatingPartial15to16(anchor),
		},
		{
			name: "Update from partial completed",
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target: openshiftv1alpha1.Version{Version: v41816},
				Previous: &openshiftv1alpha1.Version{
					Version:  v41815,
					Metadata: []openshiftv1alpha1.VersionMetadata{{Key: openshiftv1alpha1.PartialMetadata}},
				},
			},
			mutateCV: updatedPartial15to16(anchor),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     openshiftconfigv1.ClusterVersionStatus{},
			}

			tc.mutateCV(cv)
			var previous openshiftv1alpha1.ClusterVersionProgressInsightStatus
			cos := &openshiftconfigv1.ClusterOperatorList{
				Items: []openshiftconfigv1.ClusterOperator{
					instantiateClusterOperator("etcd", "operator", v41815),
					instantiateClusterOperator("kube-apiserver", "operator", v41815),
				},
			}

			insight, _ := assessClusterVersion(cv, &previous, cos)
			if diff := cmp.Diff(tc.expected, insight.Versions); diff != "" {
				t.Errorf("expected versions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_AssessClusterVersion_Completion(t *testing.T) {
	t.Parallel()

	anchor := time.Now()

	testCases := []struct {
		name             string
		mutateBaselineCV func(*openshiftconfigv1.ClusterVersion)
		cos              []openshiftconfigv1.ClusterOperator

		expected int32
	}{
		{
			name:     "Progressing=False means completed update or installation",
			expected: 100,

			mutateBaselineCV: installed15(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name:     "Progressing=False means completed even if there are COs not on desired version (inconsistent state)",
			expected: 100,

			mutateBaselineCV: installed15(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-controller-manager", "operator", "4.18.14"),
			},
		},
		{
			name:             "Progressing=True means update in progress | Pending operators are not updated",
			expected:         0,
			mutateBaselineCV: updating15to16(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name:             "Progressing=True means update in progress | Operators without a version are not updated",
			expected:         0,
			mutateBaselineCV: updating15to16(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "not-operator", v41816),
			},
		},
		{
			name:             "Progressing=True means update in progress | Updated operators",
			expected:         50,
			mutateBaselineCV: updating15to16(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41816),
			},
		},
		{
			name:             "Progressing=True means update in progress | All updated operators",
			expected:         100,
			mutateBaselineCV: updating15to16(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41816),
				instantiateClusterOperator("kube-apiserver", "operator", v41816),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     openshiftconfigv1.ClusterVersionStatus{},
			}
			tc.mutateBaselineCV(cv)
			var previous openshiftv1alpha1.ClusterVersionProgressInsightStatus
			var cos openshiftconfigv1.ClusterOperatorList
			cos.Items = append(cos.Items, tc.cos...)
			assessed, _ := assessClusterVersion(cv, &previous, &cos)

			if assessed.Completion != tc.expected {
				t.Errorf("expected completion %d, got %d", tc.expected, assessed.Completion)
			}
		})
	}
}

func Test_AssessClusterVersion_StartedCompleted(t *testing.T) {
	t.Parallel()

	anchor := time.Now()
	minutesAgo := func(m int32) metav1.Time {
		return metav1.NewTime(anchor.Add(-(time.Duration(m)) * time.Minute))
	}

	testCases := []struct {
		name            string
		lastHistoryItem openshiftconfigv1.UpdateHistory

		expectedStarted   metav1.Time
		expectedCompleted *metav1.Time
	}{
		{
			name:              "Cluster is not updating, last history item completed",
			lastHistoryItem:   cvHistory(true, v41815, minutesAgo(30), ptr.To(minutesAgo(20))),
			expectedStarted:   minutesAgo(30),
			expectedCompleted: ptr.To(minutesAgo(20)),
		},
		{
			name:            "Cluster is updating, last history item partial",
			lastHistoryItem: cvHistory(false, v41815, minutesAgo(20), nil),
			expectedStarted: minutesAgo(20),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var progressing openshiftconfigv1.ClusterOperatorStatusCondition
			if tc.lastHistoryItem.State == openshiftconfigv1.CompletedUpdate {
				// Make Progressing condition different to test that UpdateHistory times are used
				ltt := (*tc.lastHistoryItem.CompletionTime).Add(-time.Minute)
				progressing = cvProgressing(false, tc.lastHistoryItem.Version, metav1.NewTime(ltt))
			} else {
				// Make Progressing condition different to test that UpdateHistory times are used
				ltt := tc.lastHistoryItem.StartedTime.Add(-time.Minute)
				progressing = cvProgressing(true, tc.lastHistoryItem.Version, metav1.NewTime(ltt))
			}

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status: openshiftconfigv1.ClusterVersionStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{progressing},
					History:    []openshiftconfigv1.UpdateHistory{tc.lastHistoryItem},
					Desired: openshiftconfigv1.Release{
						Version: tc.lastHistoryItem.Version,
					},
				},
			}
			var previous openshiftv1alpha1.ClusterVersionProgressInsightStatus
			cos := &openshiftconfigv1.ClusterOperatorList{
				Items: []openshiftconfigv1.ClusterOperator{
					instantiateClusterOperator("etcd", "operator", tc.lastHistoryItem.Version),
					instantiateClusterOperator("kube-apiserver", "operator", tc.lastHistoryItem.Version),
				},
			}

			insight, _ := assessClusterVersion(cv, &previous, cos)
			if diff := cmp.Diff(tc.expectedStarted, insight.StartedAt); diff != "" {
				t.Errorf("expected StartedAt mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.expectedCompleted, insight.CompletedAt); diff != "" {
				t.Errorf("expected CompletedAt mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_AssessClusterVersion_EstimatedCompletedAt(t *testing.T) {
	t.Parallel()
	anchor := time.Now()
	minutesAgo := func(m int32) metav1.Time {
		return metav1.NewTime(anchor.Add(-(time.Duration(m)) * time.Minute))
	}

	testCases := []struct {
		name     string
		expected *metav1.Time

		mutateCV               func(*openshiftconfigv1.ClusterVersion)
		lastObservedProgress   metav1.Time
		lastObservedCompletion int32
		cos                    []openshiftconfigv1.ClusterOperator
	}{
		{
			name:     "No estimation when not updating",
			expected: nil,

			mutateCV:             installed15(anchor),
			lastObservedProgress: minutesAgo(40),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name: "Early in the update, baseline is hardcoded if no history is available",
			// Baseline expectation is 60m + 20% = 72m, and we are 1m into the update => 71m
			expected: ptr.To(metav1.NewTime(anchor.Add(71 * time.Minute))),

			mutateCV: func(cv *openshiftconfigv1.ClusterVersion) {
				cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressing(true, v41816, minutesAgo(1)),
				}
				cv.Status.History = []openshiftconfigv1.UpdateHistory{
					cvHistory(false, v41816, minutesAgo(1), nil),
					cvHistory(true, v41815, minutesAgo(120), ptr.To(minutesAgo(60))),
				}
				cv.Status.Desired.Version = v41816
			},
			lastObservedProgress: minutesAgo(1),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name:     "Early in the update, last completed update is used as baseline",
			expected: ptr.To(metav1.NewTime(anchor.Add(23 * time.Minute))),

			mutateCV: func(cv *openshiftconfigv1.ClusterVersion) {
				cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressing(true, v41816, minutesAgo(1)),
				}
				cv.Status.History = []openshiftconfigv1.UpdateHistory{
					cvHistory(false, v41816, minutesAgo(1), nil),
					cvHistory(true, v41815, minutesAgo(60), ptr.To(minutesAgo(40))),
					cvHistory(true, "4.18.14", minutesAgo(120), ptr.To(minutesAgo(60))),
				}
				cv.Status.Desired.Version = v41816
			},
			lastObservedProgress: minutesAgo(1),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name:     "Baseline is used until enough progress is observed",
			expected: ptr.To(metav1.NewTime(anchor.Add(12 * time.Minute))),

			mutateCV: func(cv *openshiftconfigv1.ClusterVersion) {
				cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressing(true, v41816, minutesAgo(10)),
				}
				cv.Status.History = []openshiftconfigv1.UpdateHistory{
					cvHistory(false, v41816, minutesAgo(10), nil),
					cvHistory(true, v41815, minutesAgo(60), ptr.To(minutesAgo(40))),
					cvHistory(true, "4.18.14", minutesAgo(120), ptr.To(minutesAgo(60))),
				}
				cv.Status.Desired.Version = v41816
			},
			lastObservedProgress: minutesAgo(10),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name:     "Once enough progress is observed, estimation is based on the last observed progress",
			expected: ptr.To(metav1.NewTime(anchor.Add(75 * time.Minute))),

			mutateCV: func(cv *openshiftconfigv1.ClusterVersion) {
				cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressing(true, v41816, minutesAgo(40)),
				}
				cv.Status.History = []openshiftconfigv1.UpdateHistory{
					cvHistory(false, v41816, minutesAgo(40), nil),
					cvHistory(true, v41815, minutesAgo(60), ptr.To(minutesAgo(40))),
					cvHistory(true, "4.18.14", minutesAgo(120), ptr.To(minutesAgo(60))),
				}
				cv.Status.Desired.Version = v41816
			},
			// Actual last observed progress will be now, because completion is before assessment is zero
			// and this assessment will see etcd CO updated
			lastObservedProgress: minutesAgo(10),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41816),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name:     "Once enough progress is observed, estimation is based on earlier last observed progress ",
			expected: ptr.To(metav1.NewTime(anchor.Add(63 * time.Minute))),
			mutateCV: func(cv *openshiftconfigv1.ClusterVersion) {
				cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressing(true, v41816, minutesAgo(50)),
				}
				cv.Status.History = []openshiftconfigv1.UpdateHistory{
					cvHistory(false, v41816, minutesAgo(50), nil),
					cvHistory(true, v41815, minutesAgo(70), ptr.To(minutesAgo(50))),
					cvHistory(true, "4.18.14", minutesAgo(130), ptr.To(minutesAgo(70))),
				}
				cv.Status.Desired.Version = v41816
			},
			lastObservedProgress:   minutesAgo(10),
			lastObservedCompletion: 50,
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41816),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status: openshiftconfigv1.ClusterVersionStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{},
					History:    []openshiftconfigv1.UpdateHistory{},
				},
			}

			tc.mutateCV(cv)
			previous := openshiftv1alpha1.ClusterVersionProgressInsightStatus{
				LastObservedProgress: tc.lastObservedProgress,
				Completion:           tc.lastObservedCompletion,
			}
			var cos openshiftconfigv1.ClusterOperatorList
			cos.Items = append(cos.Items, tc.cos...)

			insight, _ := assessClusterVersion(cv, &previous, &cos)
			switch {
			case tc.expected != nil && insight.EstimatedCompletedAt != nil:
				if diff := cmp.Diff(*tc.expected, *insight.EstimatedCompletedAt, cmpopts.EquateApproxTime(time.Second)); diff != "" {
					t.Errorf("expected EstimatedCompletion mismatch (-want +got):\n%s", diff)
				}
			case tc.expected == nil && insight.EstimatedCompletedAt != nil:
				t.Errorf("expected no EstimatedCompletion, got %s", insight.EstimatedCompletedAt.String())
			case tc.expected != nil && insight.EstimatedCompletedAt == nil:
				t.Errorf("expected EstimatedCompletion %s, got nil", tc.expected.String())
			}
		})
	}
}

func Test_AssessClusterVersion(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	var minutesAgo [60]metav1.Time
	for i := 0; i < 60; i++ {
		minutesAgo[i] = metav1.Time{Time: now.Add(-time.Duration(i) * time.Minute)}
	}

	var (
		cvProgressingFalse15 = openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionFalse,
			Message:            "Cluster version is 4.18.15",
			LastTransitionTime: minutesAgo[40],
		}

		cvProgressingTrue16 = openshiftconfigv1.ClusterOperatorStatusCondition{
			Type:               openshiftconfigv1.OperatorProgressing,
			Status:             openshiftconfigv1.ConditionTrue,
			Message:            "Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver",
			LastTransitionTime: minutesAgo[30],
		}

		cvHistoryCompleted15 = openshiftconfigv1.UpdateHistory{
			State:          openshiftconfigv1.CompletedUpdate,
			StartedTime:    minutesAgo[50],
			CompletionTime: &minutesAgo[40],
			Version:        v41815,
			Image:          "quay.io/something/openshift-release:4.18.15-x86_64",
		}

		cvHistoryPartial15 = openshiftconfigv1.UpdateHistory{
			State:       openshiftconfigv1.PartialUpdate,
			StartedTime: minutesAgo[50],
			Version:     v41815,
			Image:       "quay.io/something/openshift-release:4.18.15-x86_64",
		}

		cvHistoryPartial16 = openshiftconfigv1.UpdateHistory{
			State:       openshiftconfigv1.PartialUpdate,
			StartedTime: minutesAgo[30],
			Version:     v41816,
			Image:       "quay.io/something/openshift-release:4.18.16-x86_64",
		}

		etcd15 = openshiftconfigv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "etcd",
			},
			Status: openshiftconfigv1.ClusterOperatorStatus{
				Versions: []openshiftconfigv1.OperandVersion{
					{
						Name:    "operator",
						Version: v41815,
					},
				},
			},
		}

		kubeAPIServer15 = openshiftconfigv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-apiserver",
			},
			Status: openshiftconfigv1.ClusterOperatorStatus{
				Versions: []openshiftconfigv1.OperandVersion{
					{
						Name:    "operator",
						Version: v41815,
					},
				},
			},
		}

		kubeControllerManager15 = openshiftconfigv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-controller-manager",
			},
			Status: openshiftconfigv1.ClusterOperatorStatus{
				Versions: []openshiftconfigv1.OperandVersion{
					{
						Name:    "operator",
						Version: v41815,
					},
				},
			},
		}

		etcd16 = openshiftconfigv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "etcd",
			},
			Status: openshiftconfigv1.ClusterOperatorStatus{
				Versions: []openshiftconfigv1.OperandVersion{
					{
						Name:    "operator",
						Version: v41816,
					},
				},
			},
		}

		kubeAPIServer16 = openshiftconfigv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-apiserver",
			},
			Status: openshiftconfigv1.ClusterOperatorStatus{
				Versions: []openshiftconfigv1.OperandVersion{
					{
						Name:    "operator",
						Version: v41816,
					},
				},
			},
		}

		kubeControllerManagerNo = openshiftconfigv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-controller-manager",
			},
			Status: openshiftconfigv1.ClusterOperatorStatus{
				Versions: []openshiftconfigv1.OperandVersion{},
			},
		}
	)

	testCases := []struct {
		name     string
		cv       openshiftconfigv1.ClusterVersionStatus
		previous openshiftv1alpha1.ClusterVersionProgressInsightStatus
		cos      []openshiftconfigv1.ClusterOperator

		expectedInsight        *openshiftv1alpha1.ClusterVersionProgressInsightStatus
		expectedHealthInsights []*openshiftv1alpha1.UpdateHealthInsightStatus
	}{
		{
			name: "ClusterVersion with Progressing=False",
			cv: openshiftconfigv1.ClusterVersionStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressingFalse15,
				},
				History: []openshiftconfigv1.UpdateHistory{
					cvHistoryCompleted15,
				},
			},
			cos: []openshiftconfigv1.ClusterOperator{
				etcd15,
				kubeAPIServer15,
				kubeControllerManager15,
			},
			expectedInsight: &openshiftv1alpha1.ClusterVersionProgressInsightStatus{
				Assessment:           openshiftv1alpha1.ClusterVersionAssessmentCompleted,
				Completion:           100,
				LastObservedProgress: now,
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
						Status:  metav1.ConditionFalse,
						Reason:  string(openshiftv1alpha1.ClusterVersionNotProgressing),
						Message: "ClusterVersion has Progressing=False(Reason=) | Message='Cluster version is 4.18.15'",
					},
				},
				StartedAt:   cvHistoryCompleted15.StartedTime,
				CompletedAt: cvHistoryCompleted15.CompletionTime,
				Versions: openshiftv1alpha1.ControlPlaneUpdateVersions{
					Target: openshiftv1alpha1.Version{
						Version: v41815,
						Metadata: []openshiftv1alpha1.VersionMetadata{
							{
								Key: openshiftv1alpha1.InstallationMetadata,
							},
						},
					},
				},
			},
		},
		{
			name: "ClusterVersion with Progressing=True (installation)",
			cv: openshiftconfigv1.ClusterVersionStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressingTrue16,
				},
				Desired: openshiftconfigv1.Release{
					Version: v41816,
				},
				History: []openshiftconfigv1.UpdateHistory{
					cvHistoryPartial16,
				},
			},
			cos: []openshiftconfigv1.ClusterOperator{
				etcd16,
				kubeAPIServer16,
				kubeControllerManagerNo,
			},
			expectedInsight: &openshiftv1alpha1.ClusterVersionProgressInsightStatus{
				Assessment:           openshiftv1alpha1.ClusterVersionAssessmentProgressing,
				Completion:           66,
				LastObservedProgress: now,
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
						Status:  metav1.ConditionTrue,
						Reason:  string(openshiftv1alpha1.ClusterVersionProgressing),
						Message: "ClusterVersion has Progressing=True(Reason=) | Message='Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver'",
					},
				},
				StartedAt: cvHistoryPartial16.StartedTime,
				Versions: openshiftv1alpha1.ControlPlaneUpdateVersions{
					Target: openshiftv1alpha1.Version{
						Version: v41816,
						Metadata: []openshiftv1alpha1.VersionMetadata{
							{
								Key: openshiftv1alpha1.InstallationMetadata,
							},
						},
					},
				},
			},
		},
		{
			name: "ClusterVersion with Progressing=True (update)",
			cv: openshiftconfigv1.ClusterVersionStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressingTrue16,
				},
				Desired: openshiftconfigv1.Release{
					Version: v41816,
				},
				History: []openshiftconfigv1.UpdateHistory{
					cvHistoryPartial16,
					cvHistoryCompleted15,
				},
			},
			cos: []openshiftconfigv1.ClusterOperator{
				etcd16,
				kubeAPIServer15,
				kubeControllerManager15,
			},
			expectedInsight: &openshiftv1alpha1.ClusterVersionProgressInsightStatus{
				Assessment:           openshiftv1alpha1.ClusterVersionAssessmentProgressing,
				Completion:           33,
				LastObservedProgress: now,
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
						Status:  metav1.ConditionTrue,
						Reason:  string(openshiftv1alpha1.ClusterVersionProgressing),
						Message: "ClusterVersion has Progressing=True(Reason=) | Message='Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver'",
					},
				},
				StartedAt: cvHistoryPartial16.StartedTime,
				Versions: openshiftv1alpha1.ControlPlaneUpdateVersions{
					Target: openshiftv1alpha1.Version{
						Version: v41816,
					},
					Previous: &openshiftv1alpha1.Version{
						Version: v41815,
					},
				},
			},
		},
		{
			name: "ClusterVersion with Progressing=True (update from partial)",
			cv: openshiftconfigv1.ClusterVersionStatus{
				Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
					cvProgressingTrue16,
				},
				History: []openshiftconfigv1.UpdateHistory{
					cvHistoryPartial16,
					cvHistoryPartial15,
				},
			},
			cos: []openshiftconfigv1.ClusterOperator{
				etcd15,
				kubeAPIServer15,
				kubeControllerManager15,
			},
			expectedInsight: &openshiftv1alpha1.ClusterVersionProgressInsightStatus{
				Assessment:           openshiftv1alpha1.ClusterVersionAssessmentProgressing,
				Completion:           0,
				LastObservedProgress: now,
				Conditions: []metav1.Condition{
					{
						Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
						Status:  metav1.ConditionTrue,
						Reason:  string(openshiftv1alpha1.ClusterVersionProgressing),
						Message: "ClusterVersion has Progressing=True(Reason=) | Message='Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver'",
					},
				},
				StartedAt: cvHistoryPartial16.StartedTime,
				Versions: openshiftv1alpha1.ControlPlaneUpdateVersions{
					Target: openshiftv1alpha1.Version{
						Version: v41816,
					},
					Previous: &openshiftv1alpha1.Version{
						Version: v41815,
						Metadata: []openshiftv1alpha1.VersionMetadata{
							{
								Key: openshiftv1alpha1.PartialMetadata,
							},
						},
					},
				},
			},
		},
	}

	// TODO(muller): Remove ignored fields as I add functionality
	ignoreInProgressInsight := cmpopts.IgnoreFields(
		openshiftv1alpha1.ClusterVersionProgressInsightStatus{},
		"EstimatedCompletedAt",
	)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name: "version",
				},
				Status: tc.cv,
			}
			tc.expectedInsight.Name = cv.Name

			cos := openshiftconfigv1.ClusterOperatorList{}
			cos.Items = append(cos.Items, tc.cos...)

			cvInsight, healthInsights := assessClusterVersion(cv, &tc.previous, &cos)
			if diff := cmp.Diff(tc.expectedInsight, cvInsight, ignoreInProgressInsight, ignoreLastTransitionTime, cmpopts.EquateApproxTime(time.Second)); diff != "" {
				t.Errorf("expected ClusterVersionProgressInsight mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(healthInsights, tc.expectedHealthInsights); diff != "" {
				t.Errorf("expected UpdateHealthInsight mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_AssessClusterVersion_ForcedHealthInsight(t *testing.T) {
	t.Parallel()

	anchor := time.Now()
	testCases := []struct {
		name     string
		expected *openshiftv1alpha1.UpdateHealthInsightStatus
		mutateCV func(*openshiftconfigv1.ClusterVersion)
	}{
		{
			name:     "No forced health insight when CV is not annotated",
			expected: nil,
			mutateCV: func(cv *openshiftconfigv1.ClusterVersion) {
				delete(cv.Annotations, uscForceHealthInsightAnnotation)
			},
		},
		{
			name: "Forced health insight when CV is annotated",
			expected: &openshiftv1alpha1.UpdateHealthInsightStatus{
				StartedAt: metav1.NewTime(anchor),
				Scope: openshiftv1alpha1.InsightScope{
					Type:      openshiftv1alpha1.ControlPlaneScope,
					Resources: []openshiftv1alpha1.ResourceRef{{Resource: "clusterversions", Group: openshiftconfigv1.GroupName, Name: "version"}},
				},
				Impact: openshiftv1alpha1.InsightImpact{
					Level:       openshiftv1alpha1.InfoImpactLevel,
					Type:        openshiftv1alpha1.NoneImpactType,
					Summary:     "Forced health insight for ClusterVersion version",
					Description: fmt.Sprintf("The resource has a %q annotation which forces USC to generate this health insight for testing purposes.", uscForceHealthInsightAnnotation),
				},
				Remediation: openshiftv1alpha1.InsightRemediation{
					Reference: "https://issues.redhat.com/browse/OTA-1418",
				},
			},
			mutateCV: func(cv *openshiftconfigv1.ClusterVersion) {
				if cv.Annotations == nil {
					cv.Annotations = make(map[string]string)
				}
				cv.Annotations[uscForceHealthInsightAnnotation] = "true"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name: "version",
				},
			}
			updating15to16(anchor)(cv)
			tc.mutateCV(cv)
			var previous openshiftv1alpha1.ClusterVersionProgressInsightStatus
			cos := &openshiftconfigv1.ClusterOperatorList{
				Items: []openshiftconfigv1.ClusterOperator{
					instantiateClusterOperator("etcd", "operator", v41815),
					instantiateClusterOperator("kube-apiserver", "operator", v41816),
				},
			}

			var expected []*openshiftv1alpha1.UpdateHealthInsightStatus
			if tc.expected != nil {
				expected = []*openshiftv1alpha1.UpdateHealthInsightStatus{tc.expected}
			}

			_, healthInsights := assessClusterVersion(cv, &previous, cos)
			if diff := cmp.Diff(expected, healthInsights, cmpopts.EquateApproxTime(time.Second)); diff != "" {
				t.Errorf("expected UpdateHealthInsight mismatch (-want +got):\n%s", diff)
			}
		})
	}

}

func Test_AssessClusterVersion_LastObservedProgress(t *testing.T) {
	t.Parallel()

	anchor := time.Now()

	testCases := []struct {
		name string

		previousCompletion int32
		mutateBaselineCV   func(*openshiftconfigv1.ClusterVersion)
		cos                []openshiftconfigv1.ClusterOperator

		expectChange bool
	}{
		{
			name:               "was completed, is completed",
			previousCompletion: 100,
			expectChange:       false,

			mutateBaselineCV: installed15(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name:               "was updating, is completed",
			previousCompletion: 99,
			expectChange:       true,

			mutateBaselineCV: installed15(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
			},
		},
		{
			name:               "was completed, started updating",
			previousCompletion: 100,
			expectChange:       true,

			mutateBaselineCV: updating15to16(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41816),
			},
		},
		{
			name:               "was updating, progressed",
			previousCompletion: 33,
			expectChange:       true,

			mutateBaselineCV: updating15to16(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41816),
				instantiateClusterOperator("openshift-apiserver", "operator", v41816),
			},
		},
		{
			name:               "was updating, did not progress",
			previousCompletion: 33,
			expectChange:       false,

			mutateBaselineCV: updating15to16(anchor),
			cos: []openshiftconfigv1.ClusterOperator{
				instantiateClusterOperator("etcd", "operator", v41815),
				instantiateClusterOperator("kube-apiserver", "operator", v41815),
				instantiateClusterOperator("openshift-apiserver", "operator", v41816),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			now := metav1.NewTime(anchor)
			cv := &openshiftconfigv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     openshiftconfigv1.ClusterVersionStatus{},
			}
			tc.mutateBaselineCV(cv)
			previous := openshiftv1alpha1.ClusterVersionProgressInsightStatus{
				Completion:           tc.previousCompletion,
				LastObservedProgress: now,
			}

			var cos openshiftconfigv1.ClusterOperatorList
			cos.Items = append(cos.Items, tc.cos...)
			insight, _ := assessClusterVersion(cv, &previous, &cos)
			if tc.expectChange {
				if tc.previousCompletion == insight.Completion {
					t.Errorf("expected Completion to change, but it did not")
				}
				if now == insight.LastObservedProgress {
					t.Errorf("expected LastObservedProgress to change, but it did not")
				}
			} else {
				if tc.previousCompletion != insight.Completion {
					t.Errorf("expected Completion to remain the same, but it changed from %d to %d", tc.previousCompletion, insight.Completion)
				}
				if now != insight.LastObservedProgress {
					t.Errorf("expected LastObservedProgress to remain the same, but it changed from %v to %v", now, insight.LastObservedProgress)
				}
			}
		})
	}
}
