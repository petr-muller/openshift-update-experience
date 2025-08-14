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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/clusterversions"
	"github.com/petr-muller/openshift-update-experience/internal/health"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

const (
	v41815 = "4.18.15"
	v41816 = "4.18.16"
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
				StartedTime: minutesAgo[10],
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

			_ = openshiftconfigv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-controller-manager",
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

					impl: clusterversions.NewReconciler(k8sClient, k8sClient.Scheme()),
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
							Version: v41815,
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
						Version: v41815,
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
							Version: v41816,
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
						Version: v41816,
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
							Version: v41816,
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
						Version: v41816,
					},
					Previous: &openshiftv1alpha1.Version{
						Version: v41815,
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
							Version: v41816,
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
			labels:   map[string]string{health.InsightManagerLabel: "other-manager"},
			expected: false,
		},
		{
			name:     "UpdateHealthInsight with our manager label",
			labels:   map[string]string{health.InsightManagerLabel: "clusterversionprogressinsight"},
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

			managedByUs := health.PredicateForInsightsManagedBy(controllerName)

			create := event.CreateEvent{Object: &insight}
			createResult := health.PredicateForInsightsManagedBy(controllerName).Create(create)
			if createResult != tc.expected {
				t.Errorf("create: expected %v, got %v", tc.expected, createResult)
			}

			old := insight.DeepCopy()
			old.Labels = nil
			update := event.UpdateEvent{ObjectNew: &insight, ObjectOld: old}
			updateResult := managedByUs.Update(update)
			if updateResult != tc.expected {
				t.Errorf("update: expected %v, got %v", tc.expected, updateResult)
			}

			deleted := event.DeleteEvent{Object: &insight}
			deleteResult := managedByUs.Delete(deleted)
			if deleteResult != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, deleteResult)
			}
			generic := event.GenericEvent{Object: &insight}
			genericResult := managedByUs.Generic(generic)
			if genericResult != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, genericResult)
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
