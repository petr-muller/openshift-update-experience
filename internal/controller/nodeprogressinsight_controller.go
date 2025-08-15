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
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodes"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NodeProgressInsightReconciler reconciles a NodeProgressInsight object
type NodeProgressInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	impl *nodes.Reconciler
}

// NewNodeProgressInsightReconciler creates a new NodeProgressInsightReconciler
func NewNodeProgressInsightReconciler(client client.Client, scheme *runtime.Scheme) *NodeProgressInsightReconciler {
	return &NodeProgressInsightReconciler{
		Client: client,
		Scheme: scheme,

		impl: nodes.NewReconciler(client, scheme),
	}
}

// +kubebuilder:rbac:groups=openshift.muller.dev,resources=nodeprogressinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=nodeprogressinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=nodeprogressinsights/finalizers,verbs=update
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigs,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodeProgressInsight object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *NodeProgressInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.impl.Reconcile(ctx, req)
}

var mcpDeleted = predicate.Funcs{
	CreateFunc:  func(e event.TypedCreateEvent[client.Object]) bool { return false },
	UpdateFunc:  func(e event.TypedUpdateEvent[client.Object]) bool { return false },
	DeleteFunc:  func(e event.TypedDeleteEvent[client.Object]) bool { return true },
	GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool { return false },
}

var mcpSelectorEvents = predicate.Funcs{
	CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
		_, ok := e.Object.(*openshiftmachineconfigurationv1.MachineConfigPool)
		return ok
	},
	UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
		_, ok := e.ObjectNew.(*openshiftmachineconfigurationv1.MachineConfigPool)
		return ok
	},
	DeleteFunc:  func(e event.TypedDeleteEvent[client.Object]) bool { return false },
	GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool { return false },
}

var mcDeleted = predicate.Funcs{
	CreateFunc:  func(e event.TypedCreateEvent[client.Object]) bool { return false },
	UpdateFunc:  func(e event.TypedUpdateEvent[client.Object]) bool { return false },
	DeleteFunc:  func(e event.TypedDeleteEvent[client.Object]) bool { return true },
	GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool { return false },
}

var mcVersionEvents = predicate.Funcs{
	CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
		_, ok := e.Object.(*openshiftmachineconfigurationv1.MachineConfig)
		return ok
	},
	UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
		_, ok := e.ObjectNew.(*openshiftmachineconfigurationv1.MachineConfig)
		return ok
	},
	DeleteFunc:  func(e event.TypedDeleteEvent[client.Object]) bool { return false },
	GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool { return false },
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ouev1alpha1.NodeProgressInsight{}).
		Named("nodeprogressinsight").
		Watches(
			&openshiftmachineconfigurationv1.MachineConfigPool{},
			handler.EnqueueRequestsFromMapFunc(r.impl.HandleDeletedMachineConfigPool),
			builder.WithPredicates(mcpDeleted),
		).
		Watches(
			&openshiftmachineconfigurationv1.MachineConfigPool{},
			handler.EnqueueRequestsFromMapFunc(r.impl.HandleMachineConfigPool),
			builder.WithPredicates(mcpSelectorEvents),
		).
		Watches(
			&openshiftmachineconfigurationv1.MachineConfig{},
			handler.EnqueueRequestsFromMapFunc(r.impl.HandleDeletedMachineConfig),
			builder.WithPredicates(mcDeleted),
		).
		Watches(
			&openshiftmachineconfigurationv1.MachineConfig{},
			handler.EnqueueRequestsFromMapFunc(r.impl.HandleMachineConfig),
			builder.WithPredicates(mcVersionEvents),
		).
		Complete(r)
}
