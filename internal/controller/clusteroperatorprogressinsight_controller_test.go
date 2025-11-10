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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	cocontroller "github.com/petr-muller/openshift-update-experience/internal/controller/clusteroperators"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

var _ = Describe("ClusterOperatorProgressInsight Controller", Serial, func() {
	Context("When creating new ClusterOperatorProgressInsight from cluster state", func() {
		ctx := context.Background()

		now := metav1.Now()
		var minutesAgo [60]metav1.Time
		for i := 0; i < 60; i++ {
			minutesAgo[i] = metav1.Time{Time: now.Add(-time.Duration(i) * time.Minute)}
		}

		testCvName := "version"
		testCoName := "authentication"

		var (
			cvHistoryCompleted415 = openshiftconfigv1.UpdateHistory{
				State:          openshiftconfigv1.CompletedUpdate,
				StartedTime:    minutesAgo[50],
				CompletionTime: &minutesAgo[45],
				Version:        "4.15.0",
				Image:          "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64",
			}

			cvHistoryPartial416 = openshiftconfigv1.UpdateHistory{
				State:       openshiftconfigv1.PartialUpdate,
				StartedTime: minutesAgo[30],
				Version:     "4.16.0",
				Image:       "quay.io/openshift-release-dev/ocp-release:4.16.0-x86_64",
			}

			cvProgressingFalse415 = openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:               openshiftconfigv1.OperatorProgressing,
				Status:             openshiftconfigv1.ConditionFalse,
				Message:            "Cluster version is 4.15.0",
				LastTransitionTime: minutesAgo[45],
			}

			cvProgressingTrue416 = openshiftconfigv1.ClusterOperatorStatusCondition{
				Type:               openshiftconfigv1.OperatorProgressing,
				Status:             openshiftconfigv1.ConditionTrue,
				Message:            "Working towards 4.18.16: 106 of 863 done (12% complete), waiting on authentication",
				LastTransitionTime: minutesAgo[40],
			}

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

		BeforeEach(func() {
			By("Cleaning up any existing resources")
			cv := &openshiftconfigv1.ClusterVersion{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: testCvName}, cv)
			if err == nil {
				Expect(k8sClient.Delete(ctx, cv)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: testCvName}, &openshiftconfigv1.ClusterVersion{})
				}).Should(MatchError(ContainSubstring("not found")))
			}

			co := &openshiftconfigv1.ClusterOperator{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: testCoName}, co)
			if err == nil {
				Expect(k8sClient.Delete(ctx, co)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: testCoName}, &openshiftconfigv1.ClusterOperator{})
				}).Should(MatchError(ContainSubstring("not found")))
			}

			pi := &openshiftv1alpha1.ClusterOperatorProgressInsight{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: testCoName}, pi)
			if err == nil {
				Expect(k8sClient.Delete(ctx, pi)).To(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: testCoName}, &openshiftv1alpha1.ClusterOperatorProgressInsight{})
				}).Should(MatchError(ContainSubstring("not found")))
			}
		})

		type testCase struct {
			name            string
			clusterVersion  openshiftconfigv1.ClusterVersionStatus
			clusterOperator openshiftconfigv1.ClusterOperatorStatus

			expectedUpdatingCondition *metav1.Condition
		}

		DescribeTable("should create progress insight with matching status",
			func(tc testCase) {
				By("Creating the input ClusterVersion")
				cv := &openshiftconfigv1.ClusterVersion{
					ObjectMeta: metav1.ObjectMeta{Name: testCvName},
				}
				Expect(k8sClient.Create(ctx, cv)).To(Succeed())
				cv.Status = tc.clusterVersion
				Expect(k8sClient.Status().Update(ctx, cv)).To(Succeed())

				By("Creating the input ClusterOperator")
				co := &openshiftconfigv1.ClusterOperator{
					ObjectMeta: metav1.ObjectMeta{Name: testCoName},
				}
				Expect(k8sClient.Create(ctx, co)).To(Succeed())

				co.Status = tc.clusterOperator
				Expect(k8sClient.Status().Update(ctx, co)).To(Succeed())

				By("Reconciling to create the progress insight")
				controllerReconciler := &ClusterOperatorProgressInsightReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
					impl:   cocontroller.NewReconciler(k8sClient),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: testCoName},
				})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying the progress insight was created")
				progressInsight := &openshiftv1alpha1.ClusterOperatorProgressInsight{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: testCoName}, progressInsight)
				Expect(err).NotTo(HaveOccurred())

				if tc.expectedUpdatingCondition != nil {
					updatingCondition := meta.FindStatusCondition(progressInsight.Status.Conditions, string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating))
					Expect(updatingCondition).ToNot(BeNil())
					Expect(*updatingCondition).To(
						gstruct.MatchFields(
							gstruct.IgnoreExtras,
							gstruct.Fields{
								"Type":    Equal(string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating)),
								"Status":  Equal(tc.expectedUpdatingCondition.Status),
								"Reason":  Equal(tc.expectedUpdatingCondition.Reason),
								"Message": Equal(tc.expectedUpdatingCondition.Message),
							},
						),
					)
				}

				By("Cleanup")
				Expect(k8sClient.Delete(ctx, cv)).To(Succeed())
				Expect(k8sClient.Delete(ctx, co)).To(Succeed())
				Expect(k8sClient.Delete(ctx, progressInsight)).To(Succeed())
			},
			Entry("ClusterOperator is Updating=False|Reason=Completed before the update", testCase{
				name: "CO Updating=False|Reason=Completed before update",
				clusterVersion: openshiftconfigv1.ClusterVersionStatus{
					Desired: openshiftconfigv1.Release{
						Version: "4.15.0",
					},
					History: []openshiftconfigv1.UpdateHistory{
						cvHistoryCompleted415,
					},
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						cvProgressingFalse415,
					},
				},
				clusterOperator: openshiftconfigv1.ClusterOperatorStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						coProgressingFalse415,
					},
					Versions: []openshiftconfigv1.OperandVersion{
						{Name: "operator", Version: "4.15.0"},
					},
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:   string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
					Status: metav1.ConditionFalse,
					// TODO(muller): Eventually do concatenated reasons like Updated::AsExpected
					Reason: string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
					// TODO(muller): The real reason is matching version, the message should say that (in all cases)
					Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse415.Message),
				},
			}),
			Entry("ClusterOperator is Updating=False|Reason=Completed before the update even when Progressing=True", testCase{
				name: "CO Updating=False|Reason=Completed before update with Progressing=True",
				clusterVersion: openshiftconfigv1.ClusterVersionStatus{
					Desired: openshiftconfigv1.Release{
						Version: "4.15.0",
					},
					History: []openshiftconfigv1.UpdateHistory{
						cvHistoryCompleted415,
					},
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						cvProgressingFalse415,
					},
				},
				clusterOperator: openshiftconfigv1.ClusterOperatorStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						coProgressingTrue415,
					},
					Versions: []openshiftconfigv1.OperandVersion{
						{Name: "operator", Version: "4.15.0"},
					},
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:   string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
					Status: metav1.ConditionFalse,
					// TODO(muller): Eventually do concatenated reasons like Updated::AsExpected
					Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
					Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue415.Message),
				},
			}),
			Entry("ClusterOperator is Updating=False|Reason=Pending after update started", testCase{
				name: "CO Updating=False|Reason=Pending after update started",
				clusterVersion: openshiftconfigv1.ClusterVersionStatus{
					Desired: openshiftconfigv1.Release{
						Version: "4.16.0",
					},
					History: []openshiftconfigv1.UpdateHistory{
						cvHistoryPartial416,
						cvHistoryCompleted415,
					},
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						cvProgressingTrue416,
					},
				},
				clusterOperator: openshiftconfigv1.ClusterOperatorStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						coProgressingFalse415,
					},
					Versions: []openshiftconfigv1.OperandVersion{
						{Name: "operator", Version: "4.15.0"},
					},
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:   string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
					Status: metav1.ConditionFalse,
					// TODO(muller): Eventually do concatenated reasons like Updated::AsExpected
					Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonPending),
					Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse415.Message),
				},
			}),
			Entry("ClusterOperator is Updating=True|Reason=Progressing after update started, progressing, old version", testCase{
				name: "CO Updating=False|Reason=Progressing after update started, progressing, old version",
				clusterVersion: openshiftconfigv1.ClusterVersionStatus{
					Desired: openshiftconfigv1.Release{
						Version: "4.16.0",
					},
					History: []openshiftconfigv1.UpdateHistory{
						cvHistoryPartial416,
						cvHistoryCompleted415,
					},
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						cvProgressingTrue416,
					},
				},
				clusterOperator: openshiftconfigv1.ClusterOperatorStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						coProgressingTrue416,
					},
					Versions: []openshiftconfigv1.OperandVersion{
						{Name: "operator", Version: "4.15.0"},
					},
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:   string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
					Status: metav1.ConditionTrue,
					// TODO(muller): Eventually do concatenated reasons like Updated::AsExpected
					Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonProgressing),
					Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue416.Message),
				},
			}),
			Entry("ClusterOperator is Updating=False|Reason=Updated after update started, progressing, new version", testCase{
				name: "CO Updating=False|Reason=Updated after update started, progressing, new version",
				clusterVersion: openshiftconfigv1.ClusterVersionStatus{
					Desired: openshiftconfigv1.Release{
						Version: "4.16.0",
					},
					History: []openshiftconfigv1.UpdateHistory{
						cvHistoryPartial416,
						cvHistoryCompleted415,
					},
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						cvProgressingTrue416,
					},
				},
				clusterOperator: openshiftconfigv1.ClusterOperatorStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						coProgressingTrue416,
					},
					Versions: []openshiftconfigv1.OperandVersion{
						{Name: "operator", Version: "4.16.0"},
					},
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:   string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
					Status: metav1.ConditionFalse,
					// TODO(muller): Eventually do concatenated reasons like Updated::AsExpected
					Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
					Message: fmt.Sprintf("Progressing=True: %s", coProgressingTrue416.Message),
				},
			}),
			Entry("ClusterOperator is Updating=False|Reason=Updated after update started, not progressing, new version", testCase{
				name: "CO Updating=False|Reason=Updated after update started, not progressing, new version",
				clusterVersion: openshiftconfigv1.ClusterVersionStatus{
					Desired: openshiftconfigv1.Release{
						Version: "4.16.0",
					},
					History: []openshiftconfigv1.UpdateHistory{
						cvHistoryPartial416,
						cvHistoryCompleted415,
					},
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						cvProgressingTrue416,
					},
				},
				clusterOperator: openshiftconfigv1.ClusterOperatorStatus{
					Conditions: []openshiftconfigv1.ClusterOperatorStatusCondition{
						coProgressingFalse416,
					},
					Versions: []openshiftconfigv1.OperandVersion{
						{Name: "operator", Version: "4.16.0"},
					},
				},
				expectedUpdatingCondition: &metav1.Condition{
					Type:   string(openshiftv1alpha1.ClusterOperatorProgressInsightUpdating),
					Status: metav1.ConditionFalse,
					// TODO(muller): Eventually do concatenated reasons like Updated::AsExpected
					Reason:  string(openshiftv1alpha1.ClusterOperatorUpdatingReasonUpdated),
					Message: fmt.Sprintf("Progressing=False: %s", coProgressingFalse416.Message),
				},
			}),
		)
	})
})

func Test_allClusterOperators(t *testing.T) {
	testCases := []struct {
		name      string
		operators []openshiftconfigv1.ClusterOperator

		expected []reconcile.Request
	}{
		{
			name: "no operators",
		},
		{
			name:      "one operator",
			operators: []openshiftconfigv1.ClusterOperator{{ObjectMeta: metav1.ObjectMeta{Name: "test-operator"}}},
			expected: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "test-operator"}},
			},
		},
		{
			name: "multiple operators",
			operators: []openshiftconfigv1.ClusterOperator{
				{ObjectMeta: metav1.ObjectMeta{Name: "test-operator-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "test-operator-2"}},
			},
			expected: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "test-operator-1"}},
				{NamespacedName: types.NamespacedName{Name: "test-operator-2"}},
			},
		},
	}

	// Create scheme and register OpenShift types
	s := runtime.NewScheme()
	err := openshiftconfigv1.Install(s)
	if err != nil {
		t.Fatalf("Failed to add OpenShift types to scheme: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fake client with test data
			var objs []client.Object
			for _, op := range tc.operators {
				objs = append(objs, &op)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()

			f := allClusterOperators(fakeClient)
			requests := f(context.Background(), nil)
			if diff := cmp.Diff(tc.expected, requests); diff != "" {
				t.Errorf("allClusterOperators() mismatch (-want +got):\n%s", diff)
			}
		})
	}

}

func Test_allClusterOperators_error(t *testing.T) {
	// Create scheme and register OpenShift types
	s := runtime.NewScheme()
	err := openshiftconfigv1.Install(s)
	if err != nil {
		t.Fatalf("Failed to add OpenShift types to scheme: %v", err)
	}

	// Create fake client with interceptor that returns error on List
	clientWithError := fake.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
		List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			return errors.New("fake list error")
		},
	}).Build()

	f := allClusterOperators(clientWithError)
	requests := f(context.Background(), nil)

	// When List returns error, allClusterOperators should return nil
	if requests != nil {
		t.Errorf("Expected nil requests when List returns error, got %v", requests)
	}
}
