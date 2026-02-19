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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
)

// mockNodeStateProvider is a simple mock for testing
type mockNodeStateProvider struct{}

func (m *mockNodeStateProvider) GetNodeState(_ string) (*nodestate.NodeState, bool) {
	return nil, false
}

func (m *mockNodeStateProvider) GetAllNodeStates() []*nodestate.NodeState {
	return nil
}

func (m *mockNodeStateProvider) GetNodeStatesByPool(_ string) []*nodestate.NodeState {
	return []*nodestate.NodeState{}
}

func (m *mockNodeStateProvider) NodeInsightChannel() <-chan event.GenericEvent {
	return make(chan event.GenericEvent)
}

func (m *mockNodeStateProvider) MCPInsightChannel() <-chan event.GenericEvent {
	return make(chan event.GenericEvent)
}

var _ = Describe("MachineConfigPoolProgressInsight Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name: resourceName,
		}
		machineconfigpoolprogressinsight := &openshiftv1alpha1.MachineConfigPoolProgressInsight{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind MachineConfigPoolProgressInsight")
			err := k8sClient.Get(ctx, typeNamespacedName, machineconfigpoolprogressinsight)
			if err != nil && errors.IsNotFound(err) {
				resource := &openshiftv1alpha1.MachineConfigPoolProgressInsight{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &openshiftv1alpha1.MachineConfigPoolProgressInsight{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance MachineConfigPoolProgressInsight")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			// Create a mock node state provider for testing
			mockProvider := &mockNodeStateProvider{}
			controllerReconciler := NewMachineConfigPoolProgressInsightReconciler(k8sClient, k8sClient.Scheme(), mockProvider)

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
