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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

var _ = Describe("NodeProgressInsight Controller", Serial, func() {
	Context("When creating new NodeProgressInsight from cluster state", func() {
		ctx := context.Background()

		BeforeEach(func() {
			By("Cleaning up any existing resources")
			node := &corev1.Node{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-node"}, node)
			if err == nil {
				Expect(k8sClient.Delete(ctx, node)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: "test-node"}, &corev1.Node{})
				}).Should(MatchError(ContainSubstring("not found")))
			}

			pi := &openshiftv1alpha1.NodeProgressInsight{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-node"}, pi)
			if err == nil {
				Expect(k8sClient.Delete(ctx, pi)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: "test-node"}, &openshiftv1alpha1.NodeProgressInsight{})
				}).Should(MatchError(ContainSubstring("not found")))
			}
		})

		var minutesAgo [60]metav1.Time
		for i := 0; i < 60; i++ {
			minutesAgo[i] = metav1.Now()
			minutesAgo[i].Time = minutesAgo[i].Time.Add(-time.Duration(i) * time.Minute)
		}

		type testCase struct {
			name string
			node *corev1.Node
		}

		DescribeTable("should create progress insight with matching status",
			func(tc testCase) {
				By("Creating the input Node")
				status := tc.node.Status.DeepCopy()
				Expect(k8sClient.Create(ctx, tc.node)).To(Succeed())
				tc.node.Status = *status
				Expect(k8sClient.Status().Update(ctx, tc.node)).To(Succeed())

				By("Reconciling to create the progress insight")
				controllerReconciler := &NodeProgressInsightReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
					now:    metav1.Now,
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: tc.node.Name},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying the progress insight was created")
				progressInsight := &openshiftv1alpha1.NodeProgressInsight{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: tc.node.Name}, progressInsight)
				// TODO(muller): Implement tests when working on the functionality
				// Expect(err).NotTo(HaveOccurred())
				Expect(err).To(HaveOccurred())

				By("Cleanup")
				Expect(k8sClient.Delete(ctx, tc.node)).To(Succeed())
				// TODO(muller): Implement tests when working on the functionality
				// Expect(k8sClient.Delete(ctx, progressInsight)).To(Succeed())
				Expect(k8sClient.Delete(ctx, progressInsight)).NotTo(Succeed())
			},
			// TODO(muller): Implement tests when working on the functionality
			Entry("Node TODO", testCase{
				name: "TODO",
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				},
			}),
		)
	})
})
