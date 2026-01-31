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

	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/machineconfigpools"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// MachineConfigPoolProgressInsightReconciler reconciles a MachineConfigPoolProgressInsight object
type MachineConfigPoolProgressInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// stateProvider is the central node state provider (required)
	stateProvider nodestate.Provider

	impl *machineconfigpools.Reconciler
}

// NewMachineConfigPoolProgressInsightReconciler creates a new MachineConfigPoolProgressInsightReconciler
func NewMachineConfigPoolProgressInsightReconciler(client client.Client, scheme *runtime.Scheme, provider nodestate.Provider) *MachineConfigPoolProgressInsightReconciler {
	if provider == nil {
		// This should never happen due to main.go check, but defensive programming
		panic("MachineConfigPoolProgressInsightReconciler requires non-nil NodeStateProvider")
	}

	return &MachineConfigPoolProgressInsightReconciler{
		Client:        client,
		Scheme:        scheme,
		stateProvider: provider,
		impl:          machineconfigpools.NewReconciler(client, scheme, provider),
	}
}

// +kubebuilder:rbac:groups=openshift.muller.dev,resources=machineconfigpoolprogressinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=machineconfigpoolprogressinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=machineconfigpoolprogressinsights/finalizers,verbs=update
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MachineConfigPoolProgressInsight object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *MachineConfigPoolProgressInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.impl.Reconcile(ctx, req)
}

// SetupWithManager sets up the controller with the Manager.
// Watches both MachineConfigPool resources and the MCPInsightChannel for notifications.
func (r *MachineConfigPoolProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openshiftv1alpha1.MachineConfigPoolProgressInsight{}).
		Named("machineconfigpoolprogressinsight").
		Watches(
			&openshiftmachineconfigurationv1.MachineConfigPool{},
			handler.EnqueueRequestsFromMapFunc(r.handleMachineConfigPoolEvent),
		).
		// Watch the MCP notification channel from central controller
		WatchesRawSource(
			source.Channel(
				r.stateProvider.MCPInsightChannel(),
				&handler.EnqueueRequestForObject{},
			),
		).
		Complete(r)
}

// handleMachineConfigPoolEvent maps MachineConfigPool events to reconcile requests
// Since insight names match MCP names 1:1, we just return a request with the same name
func (r *MachineConfigPoolProgressInsightReconciler) handleMachineConfigPoolEvent(ctx context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{
		{NamespacedName: client.ObjectKey{Name: obj.GetName()}},
	}
}
