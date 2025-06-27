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
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

var _ = Describe("ClusterOperatorProgressInsight Controller", Serial, func() {
	Context("When creating new ClusterOperatorProgressInsight from cluster state", func() {
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

			co := &openshiftconfigv1.ClusterOperator{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "authentication"}, co)
			if err == nil {
				Expect(k8sClient.Delete(ctx, co)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: "authentication"}, &openshiftconfigv1.ClusterOperator{})
				}).Should(MatchError(ContainSubstring("not found")))
			}

			pi := &openshiftv1alpha1.ClusterOperatorProgressInsight{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "authentication"}, pi)
			if err == nil {
				Expect(k8sClient.Delete(ctx, pi)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: "authentication"}, &openshiftv1alpha1.ClusterOperatorProgressInsight{})
				}).Should(MatchError(ContainSubstring("not found")))
			}
		})

		var minutesAgo [60]metav1.Time
		now := metav1.Now()
		for i := 0; i < 60; i++ {
			minutesAgo[i] = metav1.Time{Time: now.Add(-time.Duration(i) * time.Minute)}
		}

		type testCase struct {
			name            string
			clusterVersion  *openshiftconfigv1.ClusterVersion
			clusterOperator *openshiftconfigv1.ClusterOperator
		}

		DescribeTable("should create progress insight with matching status",
			func(tc testCase) {
				By("Creating the input ClusterVersion")
				cvStatus := tc.clusterVersion.Status.DeepCopy()
				Expect(k8sClient.Create(ctx, tc.clusterVersion)).To(Succeed())
				tc.clusterVersion.Status = *cvStatus
				Expect(k8sClient.Status().Update(ctx, tc.clusterVersion)).To(Succeed())

				By("Creating the input ClusterOperator")
				status := tc.clusterOperator.Status.DeepCopy()
				Expect(k8sClient.Create(ctx, tc.clusterOperator)).To(Succeed())

				tc.clusterOperator.Status = *status
				Expect(k8sClient.Status().Update(ctx, tc.clusterOperator)).To(Succeed())

				By("Reconciling to create the progress insight")
				controllerReconciler := &ClusterOperatorProgressInsightReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
					now:    metav1.Now,
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: tc.clusterOperator.Name},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying the progress insight was created")
				progressInsight := &openshiftv1alpha1.ClusterOperatorProgressInsight{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: tc.clusterOperator.Name}, progressInsight)
				Expect(err).NotTo(HaveOccurred())

				By("Cleanup")
				Expect(k8sClient.Delete(ctx, tc.clusterOperator)).To(Succeed())
				Expect(k8sClient.Delete(ctx, progressInsight)).To(Succeed())
			},
			// TODO(muller): Implement tests when working on the functionality
			Entry("ClusterOperator TODO", testCase{
				name:           "TODO",
				clusterVersion: &openshiftconfigv1.ClusterVersion{ObjectMeta: metav1.ObjectMeta{Name: "version"}},
				clusterOperator: &openshiftconfigv1.ClusterOperator{
					ObjectMeta: metav1.ObjectMeta{
						Name: "authentication",
					},
				},
			}),
		)
	})
})
