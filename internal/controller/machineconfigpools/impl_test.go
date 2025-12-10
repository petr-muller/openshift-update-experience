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

package machineconfigpools

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcile_CreatesMasterInsightWithCorrectScope(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a master MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "master",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "master"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result.Requeue {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the MachineConfigPoolProgressInsight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "master"}, fetchedInsight); err != nil {
		t.Fatalf("Expected MachineConfigPoolProgressInsight to be created, got error: %v", err)
	}

	// Verify the status has correct Name and Scope
	expectedStatus := ouev1alpha1.MachineConfigPoolProgressInsightStatus{
		Name:  "master",
		Scope: ouev1alpha1.ControlPlaneScope,
	}

	if diff := cmp.Diff(expectedStatus, fetchedInsight.Status); diff != "" {
		t.Errorf("MachineConfigPoolProgressInsight status mismatch (-want +got):\n%s", diff)
	}
}

func TestReconcile_CreatesWorkerInsightWithCorrectScope(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a worker MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result.Requeue {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the MachineConfigPoolProgressInsight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker"}, fetchedInsight); err != nil {
		t.Fatalf("Expected MachineConfigPoolProgressInsight to be created, got error: %v", err)
	}

	// Verify the status has correct Name and Scope (WorkerPool for non-master)
	expectedStatus := ouev1alpha1.MachineConfigPoolProgressInsightStatus{
		Name:  "worker",
		Scope: ouev1alpha1.WorkerPoolScope,
	}

	if diff := cmp.Diff(expectedStatus, fetchedInsight.Status); diff != "" {
		t.Errorf("MachineConfigPoolProgressInsight status mismatch (-want +got):\n%s", diff)
	}
}

func TestReconcile_CreatesCustomPoolInsightWithWorkerScope(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a custom MachineConfigPool (not master)
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom-pool",
		},
	}

	// Create a fake client with the MCP
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "custom-pool"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result.Requeue {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the MachineConfigPoolProgressInsight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "custom-pool"}, fetchedInsight); err != nil {
		t.Fatalf("Expected MachineConfigPoolProgressInsight to be created, got error: %v", err)
	}

	// Verify the status has correct Name and Scope (WorkerPool for non-master)
	expectedStatus := ouev1alpha1.MachineConfigPoolProgressInsightStatus{
		Name:  "custom-pool",
		Scope: ouev1alpha1.WorkerPoolScope,
	}

	if diff := cmp.Diff(expectedStatus, fetchedInsight.Status); diff != "" {
		t.Errorf("MachineConfigPoolProgressInsight status mismatch (-want +got):\n%s", diff)
	}
}

func TestReconcile_BothMCPAndInsightDontExist_DoesNothing(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a fake client with no objects
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result.Requeue {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify no MachineConfigPoolProgressInsight was created
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: "nonexistent"}, fetchedInsight)
	if !errors.IsNotFound(getErr) {
		t.Errorf("Expected no MachineConfigPoolProgressInsight to exist, but found one or got error: %v", getErr)
	}
}

func TestReconcile_MCPDoesNotExistButInsightDoes_DeletesInsight(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create an existing insight without a corresponding MCP
	insight := &ouev1alpha1.MachineConfigPoolProgressInsight{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deleted-pool",
		},
		Status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
			Name:  "deleted-pool",
			Scope: ouev1alpha1.WorkerPoolScope,
		},
	}

	// Create a fake client with just the insight (no MCP)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(insight).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "deleted-pool"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result.Requeue {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the insight was deleted
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: "deleted-pool"}, fetchedInsight)
	if !errors.IsNotFound(getErr) {
		t.Errorf("Expected insight to be deleted, but found one or got error: %v", getErr)
	}
}

func TestReconcile_InsightExistsButStatusUnchanged_SkipsUpdate(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)

	// Create a MachineConfigPool
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	// Create an existing insight with correct status
	insight := &ouev1alpha1.MachineConfigPoolProgressInsight{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
		Status: ouev1alpha1.MachineConfigPoolProgressInsightStatus{
			Name:  "worker",
			Scope: ouev1alpha1.WorkerPoolScope,
		},
	}

	// Create a fake client with both MCP and insight
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mcp, insight).
		WithStatusSubresource(&ouev1alpha1.MachineConfigPoolProgressInsight{}).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Fatalf("Reconcile() returned unexpected error: %v", err)
	}
	if result.Requeue {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the insight still exists with unchanged status
	fetchedInsight := &ouev1alpha1.MachineConfigPoolProgressInsight{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "worker"}, fetchedInsight); err != nil {
		t.Fatalf("Expected insight to exist, got error: %v", err)
	}

	expectedStatus := ouev1alpha1.MachineConfigPoolProgressInsightStatus{
		Name:  "worker",
		Scope: ouev1alpha1.WorkerPoolScope,
	}

	if diff := cmp.Diff(expectedStatus, fetchedInsight.Status); diff != "" {
		t.Errorf("MachineConfigPoolProgressInsight status should be unchanged (-want +got):\n%s", diff)
	}
}

func TestAssessMachineConfigPool_MasterPool(t *testing.T) {
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "master",
		},
	}

	status := assessMachineConfigPool(mcp)

	expectedStatus := &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
		Name:  "master",
		Scope: ouev1alpha1.ControlPlaneScope,
	}

	if diff := cmp.Diff(expectedStatus, status); diff != "" {
		t.Errorf("assessMachineConfigPool() mismatch (-want +got):\n%s", diff)
	}
}

func TestAssessMachineConfigPool_WorkerPool(t *testing.T) {
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker",
		},
	}

	status := assessMachineConfigPool(mcp)

	expectedStatus := &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
		Name:  "worker",
		Scope: ouev1alpha1.WorkerPoolScope,
	}

	if diff := cmp.Diff(expectedStatus, status); diff != "" {
		t.Errorf("assessMachineConfigPool() mismatch (-want +got):\n%s", diff)
	}
}

func TestAssessMachineConfigPool_CustomPool(t *testing.T) {
	mcp := &openshiftmachineconfigurationv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "infra",
		},
	}

	status := assessMachineConfigPool(mcp)

	expectedStatus := &ouev1alpha1.MachineConfigPoolProgressInsightStatus{
		Name:  "infra",
		Scope: ouev1alpha1.WorkerPoolScope,
	}

	if diff := cmp.Diff(expectedStatus, status); diff != "" {
		t.Errorf("assessMachineConfigPool() mismatch (-want +got):\n%s", diff)
	}
}
