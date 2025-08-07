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
	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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

			co := openshiftconfigv1.ClusterOperatorList{}
			Expect(k8sClient.List(ctx, &co)).To(Succeed())
			for _, item := range co.Items {
				Expect(k8sClient.Delete(ctx, &item)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: item.Name}, &openshiftconfigv1.ClusterOperator{})
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
				LastTransitionTime: minutesAgo[20],
			}

			cvProgressingTrue16 = openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:               openshiftconfigv1.OperatorProgressing,
				Status:             openshiftconfigv1.ConditionTrue,
				Message:            "Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver",
				LastTransitionTime: minutesAgo[50],
			}

			cvHistoryCompleted15 = openshiftconfigv1.UpdateHistory{
				State:          openshiftconfigv1.CompletedUpdate,
				StartedTime:    minutesAgo[50],
				CompletionTime: &minutesAgo[20],
				Version:        "4.18.15",
				Image:          "quay.io/something/openshift-release:4.18.15-x86_64",
			}

			cvHistoryPartial15 = openshiftconfigv1.UpdateHistory{
				State:       openshiftconfigv1.PartialUpdate,
				StartedTime: minutesAgo[50],
				Version:     "4.18.15",
				Image:       "quay.io/something/openshift-release:4.18.15-x86_64",
			}

			cvHistoryPartial16 = openshiftconfigv1.UpdateHistory{
				State:       openshiftconfigv1.PartialUpdate,
				StartedTime: minutesAgo[10],
				Version:     "4.18.16",
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
							Version: "4.18.15",
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
							Version: "4.18.15",
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
							Version: "4.18.15",
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
							Version: "4.18.16",
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
							Version: "4.18.16",
						},
					},
				},
			}

			_ = openshiftconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-controller-manager",
				},
				Status: openshiftconfigv1.ClusterOperatorStatus{
					Versions: []openshiftconfigv1.OperandVersion{
						{
							Name:    "operator",
							Version: "4.18.16",
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

		type testCase struct {
			name             string
			clusterVersion   *openshiftconfigv1.ClusterVersion
			clusterOperators []*openshiftconfigv1.ClusterOperator

			expectedUpdatingCondition *metav1.Condition
			expectedVersions          *openshiftv1alpha1.ControlPlaneUpdateVersions
			expectedCompletion        *int32
			expectedStartedAt         *metav1.Time
			// completed is a pointer so distinguish when to test and when to check for nil
			expectedCompletedAt **metav1.Time
		}

		DescribeTable("should create progress insight with matching status",
			func(tc testCase) {
				By("Creating the input ClusterVersion")
				status := tc.clusterVersion.Status.DeepCopy()
				Expect(k8sClient.Create(ctx, tc.clusterVersion)).To(Succeed())

				tc.clusterVersion.Status = *status
				Expect(k8sClient.Status().Update(ctx, tc.clusterVersion)).To(Succeed())

				By("Creating the input ClusterOperators")
				for _, co := range tc.clusterOperators {
					co := co.DeepCopy()
					status := co.Status.DeepCopy()
					Expect(k8sClient.Create(ctx, co)).To(Succeed())
					co.Status = *status
					Expect(k8sClient.Status().Update(ctx, co)).To(Succeed())
				}

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

				if tc.expectedVersions != nil {
					By("Verifying the insight has expected versions")
					Expect(progressInsight.Status.Versions).To(Equal(*tc.expectedVersions))
				}

				if tc.expectedCompletion != nil {
					By("Verifying the insight has expected completion")
					Expect(progressInsight.Status.Completion).To(Equal(*tc.expectedCompletion))
				}

				if tc.expectedStartedAt != nil {
					By("Verifying the insight has expected started at time")
					Expect(progressInsight.Status.StartedAt.Time).To(BeTemporally("~", tc.expectedStartedAt.Time, time.Second))
				}

				if tc.expectedCompletedAt != nil {
					By("Verifying the insight has expected completed at time")
					if *tc.expectedCompletedAt != nil {
						Expect(progressInsight.Status.CompletedAt.Time).To(BeTemporally("~", (*tc.expectedCompletedAt).Time, time.Second))
					} else {
						Expect(progressInsight.Status.CompletedAt).To(BeNil())
					}
				}

				By("Cleanup")
				Expect(k8sClient.Delete(ctx, tc.clusterVersion)).To(Succeed())
				Expect(k8sClient.Delete(ctx, progressInsight)).To(Succeed())
				for _, co := range tc.clusterOperators {
					Expect(k8sClient.Delete(ctx, co)).To(Succeed())
				}
			},
			Entry("ClusterVersion with Progressing=False", testCase{
				name: "ClusterVersion with Progressing=False",
				clusterVersion: &openshiftconfigv1.ClusterVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "version",
					},
					Status: openshiftconfigv1.ClusterVersionStatus{
						Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
							cvProgressingFalse15,
						},
						Desired: openshiftconfigv1.Release{
							Version: "4.18.15",
						},
						History: []openshiftconfigv1.UpdateHistory{
							cvHistoryCompleted15,
						},
					},
				},
				clusterOperators: []*openshiftconfigv1.ClusterOperator{
					&etcd15,
					&kubeAPIServer15,
					&kubeControllerManager15,
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
					Status:  metav1.ConditionFalse,
					Reason:  string(openshiftv1alpha1.ClusterVersionNotProgressing),
					Message: "ClusterVersion has Progressing=False(Reason=) | Message='Cluster version is 4.18.15'",
				},
				expectedVersions: &openshiftv1alpha1.ControlPlaneUpdateVersions{
					Target: openshiftv1alpha1.Version{
						Version: "4.18.15",
						Metadata: []openshiftv1alpha1.VersionMetadata{
							{
								Key: openshiftv1alpha1.InstallationMetadata,
							},
						},
					},
				},
				expectedCompletion:  ptr.To(int32(100)),
				expectedStartedAt:   &cvHistoryCompleted15.StartedTime,
				expectedCompletedAt: &cvHistoryCompleted15.CompletionTime,
			}),
			Entry("ClusterVersion with Progressing=True (installation)", testCase{
				name: "ClusterVersion with Progressing=True (installation)",
				clusterVersion: &openshiftconfigv1.ClusterVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "version",
					},
					Status: openshiftconfigv1.ClusterVersionStatus{
						Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
							cvProgressingTrue16,
						},
						Desired: openshiftconfigv1.Release{
							Version: "4.18.16",
						},
						History: []openshiftconfigv1.UpdateHistory{
							cvHistoryPartial16,
						},
					},
				},
				clusterOperators: []*openshiftconfigv1.ClusterOperator{
					&etcd16,
					&kubeAPIServer16,
					&kubeControllerManagerNo,
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
					Status:  metav1.ConditionTrue,
					Reason:  string(openshiftv1alpha1.ClusterVersionProgressing),
					Message: "ClusterVersion has Progressing=True(Reason=) | Message='Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver'",
				},
				expectedVersions: &openshiftv1alpha1.ControlPlaneUpdateVersions{
					Target: openshiftv1alpha1.Version{
						Version: "4.18.16",
						Metadata: []openshiftv1alpha1.VersionMetadata{
							{
								Key: openshiftv1alpha1.InstallationMetadata,
							},
						},
					},
				},
				expectedCompletion:  ptr.To(int32(66)),
				expectedStartedAt:   &cvHistoryPartial16.StartedTime,
				expectedCompletedAt: ptr.To[*metav1.Time](nil),
			}),
			Entry("ClusterVersion with Progressing=True (update)", testCase{
				name: "ClusterVersion with Progressing=True (update)",
				clusterVersion: &openshiftconfigv1.ClusterVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "version",
					},
					Status: openshiftconfigv1.ClusterVersionStatus{
						Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
							cvProgressingTrue16,
						},
						Desired: openshiftconfigv1.Release{
							Version: "4.18.16",
						},
						History: []openshiftconfigv1.UpdateHistory{
							cvHistoryPartial16,
							cvHistoryCompleted15,
						},
					},
				},
				clusterOperators: []*openshiftconfigv1.ClusterOperator{
					&etcd16,
					&kubeAPIServer15,
					&kubeControllerManager15,
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
					Status:  metav1.ConditionTrue,
					Reason:  string(openshiftv1alpha1.ClusterVersionProgressing),
					Message: "ClusterVersion has Progressing=True(Reason=) | Message='Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver'",
				},
				expectedVersions: &openshiftv1alpha1.ControlPlaneUpdateVersions{
					Target: openshiftv1alpha1.Version{
						Version: "4.18.16",
					},
					Previous: &openshiftv1alpha1.Version{
						Version: "4.18.15",
					},
				},
				expectedCompletion:  ptr.To(int32(33)),
				expectedStartedAt:   &cvHistoryPartial16.StartedTime,
				expectedCompletedAt: ptr.To[*metav1.Time](nil),
			}),
			Entry("ClusterVersion with Progressing=True (update from partial)", testCase{
				name: "ClusterVersion with Progressing=True (installation)",
				clusterVersion: &openshiftconfigv1.ClusterVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "version",
					},
					Status: openshiftconfigv1.ClusterVersionStatus{
						Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
							cvProgressingTrue16,
						},
						Desired: openshiftconfigv1.Release{
							Version: "4.18.16",
						},
						History: []openshiftconfigv1.UpdateHistory{
							cvHistoryPartial16,
							cvHistoryPartial15,
						},
					},
				},
				clusterOperators: []*openshiftconfigv1.ClusterOperator{
					&etcd15,
					&kubeAPIServer15,
					&kubeControllerManager15,
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:    string(openshiftv1alpha1.ClusterVersionProgressInsightUpdating),
					Status:  metav1.ConditionTrue,
					Reason:  string(openshiftv1alpha1.ClusterVersionProgressing),
					Message: "ClusterVersion has Progressing=True(Reason=) | Message='Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver'",
				},
				expectedVersions: &openshiftv1alpha1.ControlPlaneUpdateVersions{
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
				expectedCompletion:  ptr.To(int32(0)),
				expectedStartedAt:   &cvHistoryPartial16.StartedTime,
				expectedCompletedAt: ptr.To[*metav1.Time](nil),
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

var (
	cvProgressingFalse15 = openshiftconfigv1.ClusterOperatorStatusCondition{
		Type:    openshiftconfigv1.OperatorProgressing,
		Status:  openshiftconfigv1.ConditionFalse,
		Message: "Cluster version is 4.18.15",
	}

	cvProgressingTrue16 = openshiftconfigv1.ClusterOperatorStatusCondition{
		Type:    openshiftconfigv1.OperatorProgressing,
		Status:  openshiftconfigv1.ConditionTrue,
		Message: "Working towards 4.18.16: 106 of 863 done (12% complete), waiting on etcd, kube-apiserver",
	}

	cvHistoryCompleted15 = openshiftconfigv1.UpdateHistory{
		State:   openshiftconfigv1.CompletedUpdate,
		Version: "4.18.15",
		Image:   "quay.io/something/openshift-release:4.18.15-x86_64",
	}

	// cvHistoryPartial15 = openshiftconfigv1.UpdateHistory{
	// 	State:   openshiftconfigv1.PartialUpdate,
	// 	Version: "4.18.15",
	// 	Image:   "quay.io/something/openshift-release:4.18.15-x86_64",
	// }

	cvHistoryPartial16 = openshiftconfigv1.UpdateHistory{
		State:   openshiftconfigv1.PartialUpdate,
		Version: "4.18.16",
		Image:   "quay.io/something/openshift-release:4.18.16-x86_64",
	}
)

// TODO: func Test_AssessClusterVersion_Conditions(t *testing.T) {}

// TODO: func Test_AssessClusterVersion_Assessment(t *testing.T) {}

// TODO: func Test_AssessClusterVersion_Versions(t *testing.T) {}

func Test_AssessClusterVersion_Completion(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	minutesAgo := func(m int) metav1.Time {
		return metav1.NewTime(now.Add(-(time.Duration(m)) * time.Minute))
	}

	condition := func(c openshiftconfigv1.ClusterOperatorStatusCondition, ltt metav1.Time) openshiftconfigv1.ClusterOperatorStatusCondition {
		n := c.DeepCopy()
		n.LastTransitionTime = ltt
		return *n
	}

	history := func(h openshiftconfigv1.UpdateHistory, started metav1.Time, completion *metav1.Time) openshiftconfigv1.UpdateHistory {
		n := h.DeepCopy()
		n.StartedTime = started
		n.CompletionTime = completion
		return *n
	}

	co := func(name string, versions ...string) openshiftconfigv1.ClusterOperator {
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

	installed15 := func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			condition(cvProgressingFalse15, minutesAgo(40)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			history(cvHistoryCompleted15, minutesAgo(50), ptr.To(minutesAgo(40))),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}

	updating15to16 := func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			condition(cvProgressingTrue16, minutesAgo(30)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			history(cvHistoryPartial16, minutesAgo(30), nil),
			history(cvHistoryCompleted15, minutesAgo(50), ptr.To(minutesAgo(40))),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}

	testCases := []struct {
		name             string
		mutateBaselineCV func(*openshiftconfigv1.ClusterVersion)
		cos              []openshiftconfigv1.ClusterOperator

		expected int32
	}{
		{
			name:     "Progressing=False means completed update or installation",
			expected: 100,

			mutateBaselineCV: installed15,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "operator", cvHistoryCompleted15.Version),
			},
		},
		{
			name:     "Progressing=False means completed even if there are COs not on desired version (inconsistent state)",
			expected: 100,

			mutateBaselineCV: func(cv *openshiftconfigv1.ClusterVersion) {
				cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
					condition(cvProgressingFalse15, minutesAgo(40)),
				}
				cv.Status.History = []openshiftconfigv1.UpdateHistory{
					history(cvHistoryCompleted15, minutesAgo(50), ptr.To(minutesAgo(40))),
				}
				cv.Status.Desired.Version = cv.Status.History[0].Version
			},
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-controller-manager", "operator", "4.18.14"),
			},
		},
		{
			name:             "Progressing=True means update in progress | Pending operators are not updated",
			expected:         0,
			mutateBaselineCV: updating15to16,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "operator", cvHistoryCompleted15.Version),
			},
		},
		{
			name:             "Progressing=True means update in progress | Operators without a version are not updated",
			expected:         0,
			mutateBaselineCV: updating15to16,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "not-operator", cvHistoryPartial16.Version),
			},
		},
		{
			name:             "Progressing=True means update in progress | Updated operators",
			expected:         50,
			mutateBaselineCV: updating15to16,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "operator", cvHistoryPartial16.Version),
			},
		},
		{
			name:             "Progressing=True means update in progress | All updated operators",
			expected:         100,
			mutateBaselineCV: updating15to16,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryPartial16.Version),
				co("kube-apiserver", "operator", cvHistoryPartial16.Version),
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

func Test_AssessClusterVersion_LastObservedProgress(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	minutesAgo := func(m int) metav1.Time {
		return metav1.NewTime(now.Add(-(time.Duration(m)) * time.Minute))
	}

	condition := func(c openshiftconfigv1.ClusterOperatorStatusCondition, ltt metav1.Time) openshiftconfigv1.ClusterOperatorStatusCondition {
		n := c.DeepCopy()
		n.LastTransitionTime = ltt
		return *n
	}

	history := func(h openshiftconfigv1.UpdateHistory, started metav1.Time, completion *metav1.Time) openshiftconfigv1.UpdateHistory {
		n := h.DeepCopy()
		n.StartedTime = started
		n.CompletionTime = completion
		return *n
	}

	co := func(name string, versions ...string) openshiftconfigv1.ClusterOperator {
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

	installed15 := func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			condition(cvProgressingFalse15, minutesAgo(40)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			history(cvHistoryCompleted15, minutesAgo(50), ptr.To(minutesAgo(40))),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}

	updating15to16 := func(cv *openshiftconfigv1.ClusterVersion) {
		cv.Status.Conditions = []openshiftconfigv1.ClusterOperatorStatusCondition{
			condition(cvProgressingTrue16, minutesAgo(30)),
		}
		cv.Status.History = []openshiftconfigv1.UpdateHistory{
			history(cvHistoryPartial16, minutesAgo(30), nil),
			history(cvHistoryCompleted15, minutesAgo(50), ptr.To(minutesAgo(40))),
		}
		cv.Status.Desired.Version = cv.Status.History[0].Version
	}

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

			mutateBaselineCV: installed15,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "operator", cvHistoryCompleted15.Version),
			},
		},
		{
			name:               "was updating, is completed",
			previousCompletion: 99,
			expectChange:       true,

			mutateBaselineCV: installed15,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "operator", cvHistoryCompleted15.Version),
			},
		},
		{
			name:               "was completed, started updating",
			previousCompletion: 100,
			expectChange:       true,

			mutateBaselineCV: updating15to16,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "operator", cvHistoryPartial16.Version),
			},
		},
		{
			name:               "was updating, progressed",
			previousCompletion: 33,
			expectChange:       true,

			mutateBaselineCV: updating15to16,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "operator", cvHistoryPartial16.Version),
				co("openshift-apiserver", "operator", cvHistoryPartial16.Version),
			},
		},
		{
			name:               "was updating, did not progress",
			previousCompletion: 33,
			expectChange:       false,

			mutateBaselineCV: updating15to16,
			cos: []openshiftconfigv1.ClusterOperator{
				co("etcd", "operator", cvHistoryCompleted15.Version),
				co("kube-apiserver", "operator", cvHistoryCompleted15.Version),
				co("openshift-apiserver", "operator", cvHistoryPartial16.Version),
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

// TODO: func Test_AssessClusterVersion_StartedCompleted(t *testing.T) {}

// TODO: func Test_AssessClusterVersion_EstimatedCompletedAt(t *testing.T) {}

// TODO: func Test_AssessClusterVersion_ForcedHealthInsight(t *testing.T) {}

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
			Version:        "4.18.15",
			Image:          "quay.io/something/openshift-release:4.18.15-x86_64",
		}

		cvHistoryPartial15 = openshiftconfigv1.UpdateHistory{
			State:       openshiftconfigv1.PartialUpdate,
			StartedTime: minutesAgo[50],
			Version:     "4.18.15",
			Image:       "quay.io/something/openshift-release:4.18.15-x86_64",
		}

		cvHistoryPartial16 = openshiftconfigv1.UpdateHistory{
			State:       openshiftconfigv1.PartialUpdate,
			StartedTime: minutesAgo[30],
			Version:     "4.18.16",
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
						Version: "4.18.15",
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
						Version: "4.18.15",
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
						Version: "4.18.15",
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
						Version: "4.18.16",
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
						Version: "4.18.16",
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
						Version: "4.18.15",
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
					Version: "4.18.16",
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
						Version: "4.18.16",
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
					Version: "4.18.16",
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
						Version: "4.18.16",
					},
					Previous: &openshiftv1alpha1.Version{
						Version: "4.18.15",
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
		},
	}

	// TODO(muller): Remove ignored fields as I add functionality
	ignoreInProgressInsight := cmpopts.IgnoreFields(
		openshiftv1alpha1.ClusterVersionProgressInsightStatus{},
		"EstimatedCompletedAt",
	)
	ignoreLastTransitionTime := cmpopts.IgnoreFields(
		metav1.Condition{},
		"LastTransitionTime",
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

func Test_coOperatorVersionChanged(t *testing.T) {
	testCases := []struct {
		name     string
		before   []openshiftconfigv1.OperandVersion
		after    []openshiftconfigv1.OperandVersion
		expected bool
	}{
		{
			name:     "no change in versions",
			before:   []openshiftconfigv1.OperandVersion{{Name: "operator", Version: "1.0.0"}},
			after:    []openshiftconfigv1.OperandVersion{{Name: "operator", Version: "1.0.0"}},
			expected: false,
		},
		{
			name:     "version changed",
			before:   []openshiftconfigv1.OperandVersion{{Name: "operator", Version: "1.0.0"}},
			after:    []openshiftconfigv1.OperandVersion{{Name: "operator", Version: "1.0.1"}},
			expected: true,
		},
		{
			name:     "operand version changed, operator version unchanged",
			before:   []openshiftconfigv1.OperandVersion{{Name: "operator", Version: "1.0.0"}, {Name: "operand", Version: "1.0.0"}},
			after:    []openshiftconfigv1.OperandVersion{{Name: "operator", Version: "1.0.0"}, {Name: "operand", Version: "1.0.1"}},
			expected: false,
		},
		{
			name:     "one side missing operator version",
			before:   []openshiftconfigv1.OperandVersion{{Name: "operand", Version: "1.0.0"}},
			after:    []openshiftconfigv1.OperandVersion{{Name: "operand", Version: "1.0.0"}, {Name: "operator", Version: "1.0.0"}},
			expected: true,
		},
		{
			name:     "both sides missing operator version",
			before:   []openshiftconfigv1.OperandVersion{{Name: "operand", Version: "1.0.0"}},
			after:    []openshiftconfigv1.OperandVersion{{Name: "operand", Version: "1.0.0"}},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			coBefore := openshiftconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "authentication"},
				Status:     openshiftconfigv1.ClusterOperatorStatus{Versions: tc.before},
			}
			coAfter := openshiftconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "authentication"},
				Status:     openshiftconfigv1.ClusterOperatorStatus{Versions: tc.after},
			}

			create := event.CreateEvent{Object: &coAfter}
			createResult := coOperatorVersionChanged.Create(create)
			if !createResult {
				t.Errorf("create: expected true, got false")
			}

			update := event.UpdateEvent{ObjectNew: &coAfter, ObjectOld: &coBefore}
			updateResult := coOperatorVersionChanged.Update(update)
			if updateResult != tc.expected {
				t.Errorf("update: expected %v, got %v", tc.expected, updateResult)
			}

			deleted := event.DeleteEvent{Object: &coAfter}
			deleteResult := coOperatorVersionChanged.Delete(deleted)
			if !deleteResult {
				t.Errorf("delete: expected true, got false")
			}
			generic := event.GenericEvent{Object: &coAfter}
			genericResult := coOperatorVersionChanged.Generic(generic)
			if !genericResult {
				t.Errorf("generic: expected true, got false")
			}
		})
	}
}

// TODO(muller): Test nameForHealthInsight
// TODO(muller): Test reconcileHealthInsights
