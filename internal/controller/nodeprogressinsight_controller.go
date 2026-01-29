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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
)

// NodeProgressInsightReconciler reconciles a NodeProgressInsight object
type NodeProgressInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// stateProvider is the central node state provider (required)
	stateProvider nodestate.Provider
}

// NewNodeProgressInsightReconciler creates a NodeProgressInsightReconciler that uses
// the central state provider for node state. This is the only supported mode.
func NewNodeProgressInsightReconciler(client client.Client, scheme *runtime.Scheme, provider nodestate.Provider) *NodeProgressInsightReconciler {
	if provider == nil {
		// This should never happen due to main.go check, but defensive programming
		panic("NodeProgressInsightReconciler requires non-nil NodeStateProvider")
	}
	return &NodeProgressInsightReconciler{
		Client:        client,
		Scheme:        scheme,
		stateProvider: provider,
	}
}

// +kubebuilder:rbac:groups=openshift.muller.dev,resources=nodeprogressinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=nodeprogressinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=nodeprogressinsights/finalizers,verbs=update
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// This method reads pre-computed node state from the central state provider
// and updates the CRD status.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *NodeProgressInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconcileWithProvider(ctx, req)
}

// reconcileWithProvider implements reconciliation using the central state provider.
// This reads pre-computed node state and copies it to the NodeProgressInsight CRD.
func (r *NodeProgressInsightReconciler) reconcileWithProvider(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	// Get the node state from central provider
	state, found := r.stateProvider.GetNodeState(req.Name)
	if !found {
		// Node state not tracked - might be deleted or not belong to any MCP
		// Try to clean up any existing insight
		var insight ouev1alpha1.NodeProgressInsight
		if err := r.Get(ctx, req.NamespacedName, &insight); err != nil {
			// Insight doesn't exist either - nothing to do
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		// Delete the insight since node state is gone
		logger.Info("Deleting NodeProgressInsight for untracked node")
		return ctrl.Result{}, r.Delete(ctx, &insight)
	}

	// Ensure the NodeProgressInsight CRD exists
	var insight ouev1alpha1.NodeProgressInsight
	if err := r.Get(ctx, req.NamespacedName, &insight); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		// Create the insight
		insight.Name = req.Name
		if err := r.Create(ctx, &insight); err != nil {
			logger.Error(err, "Failed to create NodeProgressInsight")
			return ctrl.Result{}, err
		}
		logger.Info("Created NodeProgressInsight")
	}

	// Copy state to insight status
	newStatus := nodeStateToInsightStatus(state)

	// Check if status update is needed (compare hashes or use deep equal)
	// For simplicity, we always update - controller-runtime handles no-ops
	insight.Status = *newStatus
	if err := r.Status().Update(ctx, &insight); err != nil {
		logger.Error(err, "Failed to update NodeProgressInsight status")
		return ctrl.Result{}, err
	}

	logger.V(4).Info("Updated NodeProgressInsight status from central state",
		"phase", state.Phase,
		"version", state.Version,
	)

	return ctrl.Result{}, nil
}

// nodeStateToInsightStatus converts internal NodeState to CRD status
func nodeStateToInsightStatus(state *nodestate.NodeState) *ouev1alpha1.NodeProgressInsightStatus {
	return &ouev1alpha1.NodeProgressInsightStatus{
		Name:         state.Name,
		PoolResource: state.PoolRef,
		Scope:        state.Scope,
		Version:      state.Version,
		Message:      state.Message,
		Conditions:   state.Conditions,
		// EstimatedToComplete is computed by the central evaluator and stored in state
		// If needed, add it to NodeState
	}
}

// SetupWithManager sets up the controller with the Manager.
// Watches the NodeInsightChannel for notifications from the central state controller.
func (r *NodeProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ouev1alpha1.NodeProgressInsight{}).
		Named("nodeprogressinsight").
		// Watch the notification channel from central controller
		WatchesRawSource(
			source.Channel(
				r.stateProvider.NodeInsightChannel(),
				&handler.EnqueueRequestForObject{},
			),
		).
		Complete(r)
}
