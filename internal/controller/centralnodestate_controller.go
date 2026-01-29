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

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"

	"github.com/petr-muller/openshift-update-experience/internal/clusterversions"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
)

// CentralNodeStateReconciler is the thin wrapper for CentralNodeStateController.
// It handles controller-runtime setup and watch configuration while delegating
// reconciliation logic to the implementation in internal/controller/nodestate/.
type CentralNodeStateReconciler struct {
	client.Client

	impl *nodestate.CentralNodeStateController
}

// NewCentralNodeStateReconciler creates a new CentralNodeStateReconciler.
func NewCentralNodeStateReconciler(client client.Client) *CentralNodeStateReconciler {
	return &CentralNodeStateReconciler{
		Client: client,
		impl:   nodestate.NewCentralNodeStateController(client),
	}
}

// GetStateProvider returns the NodeStateProvider interface for downstream controllers.
// This should be called after the reconciler is created to pass to downstream controllers
// that need to read node state.
func (r *CentralNodeStateReconciler) GetStateProvider() nodestate.Provider {
	return r.impl
}

// Predicates for filtering MachineConfigPool and MachineConfig events
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

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get;list;watch

// Reconcile evaluates node state and stores it in the central store.
// Downstream controllers read from GetNodeState() instead of evaluating themselves.
func (r *CentralNodeStateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.impl.Reconcile(ctx, req)
}

// SetupWithManager sets up the controller with the Manager.
// The controller watches:
//   - Nodes: primary resource, triggers reconciliation on changes
//   - MachineConfigPools: triggers all-node reconciliation on selector changes
//   - MachineConfigs: triggers all-node reconciliation on version changes
//   - ClusterVersion: triggers all-node reconciliation when target version changes
//
// T066: Register as Runnable for lifecycle management
func (r *CentralNodeStateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize caches before the controller starts
	ctx := context.Background()
	logger := logf.FromContext(ctx).WithName("centralnodestate-setup")
	if err := r.impl.InitializeCaches(ctx); err != nil {
		logger.Error(err, "Failed to initialize caches during setup")
		// Continue anyway - caches will be populated as events come in
	}

	// Register the implementation as a Runnable for lifecycle management
	// This ensures Start() is called on startup and channels are closed on shutdown
	if err := mgr.Add(r.impl); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("centralnodestate").
		// Primary watch: Nodes
		// Each node change triggers a reconciliation for that specific node
		Watches(
			&corev1.Node{},
			&handler.EnqueueRequestForObject{},
		).
		// MachineConfigPool watches: selector changes affect node->pool mapping
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
		// MachineConfig watches: version changes affect node version determination
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
		// ClusterVersion watch: target version changes affect all node evaluations
		Watches(
			&openshiftconfigv1.ClusterVersion{},
			handler.EnqueueRequestsFromMapFunc(r.impl.HandleClusterVersion),
			builder.WithPredicates(
				clusterversions.IsVersion,
				clusterversions.StartedOrCompletedUpdating,
			),
		).
		Complete(r)
}
