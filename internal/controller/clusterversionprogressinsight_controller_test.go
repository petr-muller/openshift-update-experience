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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
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
