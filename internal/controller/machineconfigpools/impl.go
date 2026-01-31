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

	"github.com/google/go-cmp/cmp"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// stateProvider provides access to central node state
	stateProvider nodestate.Provider
}

func NewReconciler(client client.Client, scheme *runtime.Scheme, provider nodestate.Provider) *Reconciler {
	if provider == nil {
		panic("MCP Reconciler requires non-nil NodeStateProvider")
	}

	return &Reconciler{
		Client:        client,
		Scheme:        scheme,
		stateProvider: provider,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Fetch the MachineConfigPool
	var mcp openshiftmachineconfigurationv1.MachineConfigPool
	mcpErr := r.Get(ctx, req.NamespacedName, &mcp)
	if mcpErr != nil && !apierrors.IsNotFound(mcpErr) {
		logger.WithValues("MachineConfigPool", req.NamespacedName).Error(mcpErr, "Failed to get MachineConfigPool")
		return ctrl.Result{}, mcpErr
	}

	// Fetch the MachineConfigPoolProgressInsight
	progressInsight := ouev1alpha1.MachineConfigPoolProgressInsight{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
	}
	err := r.Get(ctx, req.NamespacedName, &progressInsight)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.WithValues("MachineConfigPoolProgressInsight", req.NamespacedName).Error(err, "Failed to get MachineConfigPoolProgressInsight")
		return ctrl.Result{}, err
	}

	// If both MCP and insight don't exist, nothing to do
	if apierrors.IsNotFound(mcpErr) && apierrors.IsNotFound(err) {
		logger.WithValues("MachineConfigPoolProgressInsight", req.NamespacedName).Info("Both MachineConfigPool and insight do not exist, nothing to reconcile")
		return ctrl.Result{}, nil
	}

	// If MCP doesn't exist but insight does, delete the insight
	if apierrors.IsNotFound(mcpErr) && !apierrors.IsNotFound(err) {
		logger.WithValues("MachineConfigPoolProgressInsight", req.NamespacedName).Info("MachineConfigPool does not exist, deleting MachineConfigPoolProgressInsight")
		deleteErr := r.Delete(ctx, &progressInsight)
		if deleteErr != nil {
			logger.Error(deleteErr, "Failed to delete MachineConfigPoolProgressInsight")
		}
		return ctrl.Result{}, deleteErr
	}

	// Get node states for this pool from the central controller
	nodeStates := r.stateProvider.GetNodeStatesByPool(mcp.Name)

	// Determine scope based on MCP name
	var scope ouev1alpha1.ScopeType
	if mcp.Name == mco.MachineConfigPoolMaster {
		scope = ouev1alpha1.ControlPlaneScope
	} else {
		scope = ouev1alpha1.WorkerPoolScope
	}

	// Assess the pool from node states
	mcpInsight := AssessPoolFromNodeStates(mcp.Name, scope, mcp.Spec.Paused, nodeStates)

	// Create insight if it doesn't exist
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, &progressInsight); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				logger.WithValues("MachineConfigPoolProgressInsight", req.NamespacedName).Error(err, "Failed to create MachineConfigPoolProgressInsight")
				return ctrl.Result{}, err
			}
			// Object was created by another reconciliation, requeue to retry
			logger.V(1).Info("MachineConfigPoolProgressInsight already exists, will retry", "MachineConfigPoolProgressInsight", req.NamespacedName)
			return ctrl.Result{Requeue: true}, nil
		}
		logger.WithValues("MachineConfigPoolProgressInsight", req.NamespacedName).Info("Created MachineConfigPoolProgressInsight")
	}

	// Check if status update is needed
	diff := cmp.Diff(&progressInsight.Status, mcpInsight)
	if diff == "" {
		logger.WithValues("MachineConfigPoolProgressInsight", req.NamespacedName).Info("No changes in status, skipping update")
		return ctrl.Result{}, nil
	}

	// Update status
	logger.Info(diff)
	progressInsight.Status = *mcpInsight
	if err := r.Client.Status().Update(ctx, &progressInsight); err != nil {
		if apierrors.IsConflict(err) {
			// Conflict means another reconciliation updated it, requeue to try again
			logger.V(1).Info("Conflict updating status, will retry", "MachineConfigPoolProgressInsight", req.NamespacedName)
			return ctrl.Result{Requeue: true}, nil
		}
		logger.WithValues("MachineConfigPoolProgressInsight", req.NamespacedName).Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}
	logger.WithValues("MachineConfigPoolProgressInsight", req.NamespacedName).Info("Updated MachineConfigPoolProgressInsight status")
	return ctrl.Result{}, nil
}
