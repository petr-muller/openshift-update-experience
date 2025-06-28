/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

var _ = Describe("ClusterVersionProgressInsight Controller", Serial, func() {
	Context("When creating new ClusterVersionProgressInsight from cluster state", func() {
		ctx := context.Background()

		BeforeEach(func() {
			By("Cleaning up any existing resources")
			cv := &openshiftconfigv1.ClusterVersion{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "version"}, cv)
			if err == nil {
				Expect(k8sClient.Delete(ctx, cv)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: "version"}, &openshiftconfigv1.ClusterVersion{})
				}).Should(MatchError(ContainSubstring("not found")))
			}

			pi := &openshiftv1alpha1.ClusterVersionProgressInsight{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "version"}, pi)
			if err == nil {
				Expect(k8sClient.Delete(ctx, pi)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: "version"}, &openshiftv1alpha1.ClusterVersionProgressInsight{})
				}).Should(MatchError(ContainSubstring("not found")))
			}
		})

		var minutesAgo [60]metav1.Time
		now := metav1.Now()
		for i := 0; i < 60; i++ {
			minutesAgo[i] = metav1.Time{Time: now.Add(-time.Duration(i) * time.Minute)}
		}

		var (
			cvProgressingFalse = openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:               openshiftconfigv1.OperatorProgressing,
				Status:             openshiftconfigv1.ConditionFalse,
				Message:            "Cluster version is 4.18.15",
				LastTransitionTime: metav1.Now(),
			}

			cvHistoryInstallation = openshiftconfigv1.UpdateHistory{
				State:          openshiftconfigv1.CompletedUpdate,
				StartedTime:    minutesAgo[40],
				CompletionTime: &minutesAgo[10],
				Version:        "4.18.15",
				Image:          "quay.io/something/openshift-release:4.18.15-x86_64",
			}
		)

		type testCase struct {
			name           string
			clusterVersion *openshiftconfigv1.ClusterVersion

			expectedUpdatingCondition *metav1.Condition
		}

		DescribeTable("should create progress insight with matching status",
			func(tc testCase) {
				By("Creating the input ClusterVersion")
				status := tc.clusterVersion.Status.DeepCopy()
				Expect(k8sClient.Create(ctx, tc.clusterVersion)).To(Succeed())

				tc.clusterVersion.Status = *status
				Expect(k8sClient.Status().Update(ctx, tc.clusterVersion)).To(Succeed())

				By("Reconciling to create the progress insight")
				controllerReconciler := &ClusterVersionProgressInsightReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: tc.clusterVersion.Name},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying the progress insight was created")
				progressInsight := &openshiftv1alpha1.ClusterVersionProgressInsight{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: tc.clusterVersion.Name}, progressInsight)
				Expect(err).NotTo(HaveOccurred())

				if tc.expectedUpdatingCondition != nil {
					By("Verifying the insight has expected updating condition")
					updatingCondition := meta.FindStatusCondition(progressInsight.Status.Conditions, string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating))
					Expect(updatingCondition).NotTo(BeNil())
					Expect(*updatingCondition).To(
						MatchFields(
							IgnoreExtras,
							Fields{
								"Type":    Equal(string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating)),
								"Status":  Equal(tc.expectedUpdatingCondition.Status),
								"Reason":  Equal(tc.expectedUpdatingCondition.Reason),
								"Message": Equal(tc.expectedUpdatingCondition.Message),
							},
						),
					)
				}

				By("Cleanup")
				Expect(k8sClient.Delete(ctx, tc.clusterVersion)).To(Succeed())
				Expect(k8sClient.Delete(ctx, progressInsight)).To(Succeed())
			},
			Entry("ClusterVersion with Progressing=False", testCase{
				name: "ClusterVersion with Progressing=False",
				clusterVersion: &openshiftconfigv1.ClusterVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "version",
					},
					Status: openshiftconfigv1.ClusterVersionStatus{
						Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
							cvProgressingFalse,
						},
						History: []openshiftconfigv1.UpdateHistory{
							cvHistoryInstallation,
						},
					},
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
					Status:  metav1.ConditionFalse,
					Reason:  string(openshiftv1alpha1.ClusterVersionNotProgressing),
					Message: "ClusterVersion has Progressing=False(Reason=) | Message='Cluster version is 4.18.15'",
				},
			}),
		)
	})
})

func Test_UpdateHealthInsightPredicate(t *testing.T) {
	testCases := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name:     "UpdateHealthInsight without manager label",
			labels:   map[string]string{},
			expected: false,
		},
		{
			name:     "UpdateHealthInsight with not our manager label",
			labels:   map[string]string{labelUpdateHealthInsightManager: "other-manager"},
			expected: false,
		},
		{
			name:     "UpdateHealthInsight with our manager label",
			labels:   map[string]string{labelUpdateHealthInsightManager: "clusterversionprogressinsight"},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			insight := openshiftv1alpha1.UpdateHealthInsight{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tc.labels,
				},
			}

			create := event.CreateEvent{Object: &insight}
			createResult := healthInsightsManagedByClusterVersionProgressInsight.Create(create)
			if createResult != tc.expected {
				t.Errorf("create: expected %v, got %v", tc.expected, createResult)
			}

			old := insight.DeepCopy()
			old.Labels = nil
			update := event.UpdateEvent{ObjectNew: &insight, ObjectOld: old}
			updateResult := healthInsightsManagedByClusterVersionProgressInsight.Update(update)
			if updateResult != tc.expected {
				t.Errorf("update: expected %v, got %v", tc.expected, updateResult)
			}

			deleted := event.DeleteEvent{Object: &insight}
			deleteResult := healthInsightsManagedByClusterVersionProgressInsight.Delete(deleted)
			if deleteResult != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, deleteResult)
			}
			generic := event.GenericEvent{Object: &insight}
			genericResult := healthInsightsManagedByClusterVersionProgressInsight.Generic(generic)
			if genericResult != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, genericResult)
			}
		})
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
		Version:        "4.18.15",
		Image:          "quay.io/something/openshift-release:4.18.15-x86_64",
	}

	partialHistoryItem := openshiftconfigv1.UpdateHistory{
		State:       openshiftconfigv1.PartialUpdate,
		StartedTime: minutesAge[50],
		Version:     "4.18.15",
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
		Version:        "4.18.15",
		Image:          "quay.io/something/openshift-release:4.18.15-x86_64",
	}

	partial := openshiftconfigv1.UpdateHistory{
		State:       openshiftconfigv1.PartialUpdate,
		StartedTime: minutesAgo[5],
		Version:     "4.18.15",
		Image:       "quay.io/something/openshift-release:4.18.15-x86_64",
	}

	completed16 := openshiftconfigv1.UpdateHistory{
		State:          openshiftconfigv1.CompletedUpdate,
		StartedTime:    minutesAgo[5],
		CompletionTime: &minutesAgo[2],
		Version:        "4.18.16",
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
					Version: "4.18.15",
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
					Version: "4.18.16",
				},
				Previous: &openshiftv1alpha1.Version{
					Version: "4.18.15",
				},
			},
		},
		{
			name:    "update from partial",
			history: []openshiftconfigv1.UpdateHistory{completed16, partial},
			expected: openshiftv1alpha1.ControlPlaneUpdateVersions{
				Target: openshiftv1alpha1.Version{
					Version: "4.18.16",
				},
				Previous: &openshiftv1alpha1.Version{
					Version: "4.18.15",
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
	now := time.Now()
	testCases := []struct {
		name     string
		started  time.Time
		expected time.Time
	}{
		{
			name:     "estimate from now",
			started:  now,
			expected: now.Add(60 * time.Minute),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := estimateCompletion(tc.started)
			if !actual.Equal(tc.expected) {
				t.Errorf("expected completion %v, got %v", tc.expected, actual)
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

// TODO(muller): Test assessClusterVersion
// TODO(muller): Test nameForHealthInsight
// TODO(muller): Test reconcileHealthInsights
