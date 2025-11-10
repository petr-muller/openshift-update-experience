package nodes

import (
	"context"
	"testing"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcile_NodeWithoutMCP_DeletesStaleInsight(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)
	_ = openshiftconfigv1.Install(scheme)

	// Create a node without labels that match any MCP
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"no-mcp": "true", // This won't match any MCP selector
			},
		},
	}

	// Create a stale NodeProgressInsight that should be deleted
	insight := &ouev1alpha1.NodeProgressInsight{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Status: ouev1alpha1.NodeProgressInsightStatus{
			Name: "test-node",
		},
	}

	// Create ClusterVersion (required by reconciliation logic)
	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
				},
			},
		},
	}

	// Create a fake client with the node, insight, and ClusterVersion
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, insight, cv).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
		now:    metav1.Now,
	}

	// Note: mcpSelectors is not initialized, so whichMCP will return ""
	// This simulates a node not belonging to any MCP

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-node"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Errorf("Reconcile() returned unexpected error: %v", err)
	}
	if result.Requeue {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify the NodeProgressInsight was deleted
	fetchedInsight := &ouev1alpha1.NodeProgressInsight{}
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-node"}, fetchedInsight)
	if !errors.IsNotFound(getErr) {
		t.Errorf("Expected NodeProgressInsight to be deleted, but got error: %v", getErr)
	}
}

func TestReconcile_NodeWithoutMCP_NoInsight_DoesNothing(t *testing.T) {
	// Create a scheme and register types
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = ouev1alpha1.AddToScheme(scheme)
	_ = openshiftmachineconfigurationv1.Install(scheme)
	_ = openshiftconfigv1.Install(scheme)

	// Create a node without labels that match any MCP
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"no-mcp": "true",
			},
		},
	}

	// Create ClusterVersion
	cv := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
		Status: openshiftconfigv1.ClusterVersionStatus{
			History: []openshiftconfigv1.UpdateHistory{
				{
					State:   openshiftconfigv1.CompletedUpdate,
					Version: "4.15.0",
				},
			},
		},
	}

	// Create a fake client with just the node and ClusterVersion (no insight)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node, cv).
		Build()

	// Create the reconciler
	reconciler := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
		now:    metav1.Now,
	}

	// Reconcile
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-node"},
	}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Verify no error and no requeue
	if err != nil {
		t.Errorf("Reconcile() returned unexpected error: %v", err)
	}
	if result.Requeue {
		t.Errorf("Reconcile() should not requeue")
	}

	// Verify no NodeProgressInsight was created
	fetchedInsight := &ouev1alpha1.NodeProgressInsight{}
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-node"}, fetchedInsight)
	if !errors.IsNotFound(getErr) {
		t.Errorf("Expected no NodeProgressInsight to exist, but found one or got error: %v", getErr)
	}
}
