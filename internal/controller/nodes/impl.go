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

package nodes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/nodestate"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
)

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// once does the tasks that need to be executed once and only once at the beginning of its sync function
	// for each nodeInformerController instance, e.g., initializing caches.
	once sync.Once

	// mcpSelectors caches the label selectors converted from the node selectors of the machine config pools by their names.
	mcpSelectors nodestate.MachineConfigPoolSelectorCache

	// machineConfigVersions caches machine config versions which stores the name of MC as the key
	// and the release image version as its value retrieved from the annotation of the MC.
	machineConfigVersions nodestate.MachineConfigVersionCache

	now func() metav1.Time
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Warm up controller's caches.
	// This has to be called after informers caches have been synced and before the first event comes in.
	// The existing openshift-library does not provide such a hook.
	// In case any error occurs during cache initialization, we can still proceed which leads to stale insights that
	// will be corrected on the reconciliation when the caches are warmed up.
	r.once.Do(func() {
		if err := r.initializeMachineConfigPools(ctx); err != nil {
			logger.Error(err, "Failed to initialize machineConfigPoolSelectorCache")
		}
		if err := r.initializeMachineConfigVersions(ctx); err != nil {
			logger.Error(err, "Failed to initialize machineConfigVersions")
		}
	})

	var node corev1.Node
	nodeErr := r.Get(ctx, req.NamespacedName, &node)
	nodeNotFound := errors.IsNotFound(nodeErr)
	if nodeErr != nil && !nodeNotFound {
		logger.WithValues("Node", req.NamespacedName).Error(nodeErr, "Failed to get Node")
		return ctrl.Result{}, nodeErr
	}

	var progressInsight ouev1alpha1.NodeProgressInsight
	piErr := r.Get(ctx, req.NamespacedName, &progressInsight)
	progressInsightNotFound := errors.IsNotFound(piErr)
	if piErr != nil && !progressInsightNotFound {
		logger.WithValues("NodeProgressInsight", req.NamespacedName).Error(piErr, "Failed to get NodeProgressInsight")
		return ctrl.Result{}, piErr
	}

	if nodeNotFound {
		if progressInsightNotFound {
			logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("Both Node and NodeProgressInsight do not exist, nothing to do")
			return ctrl.Result{}, nil
		}

		logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("Node does not exist, deleting NodeProgressInsight")
		err := r.Delete(ctx, &progressInsight)
		if err != nil {
			logger.Error(err, "Failed to delete NodeProgressInsight")
		}
		return ctrl.Result{}, err
	}

	mcpName := r.mcpSelectors.WhichMCP(labels.Set(node.Labels))
	if mcpName == "" {
		// Node doesn't belong to any MachineConfigPool - clean up stale insight if it exists
		logger.WithValues("Node", req.NamespacedName).Info("Node does not belong to any MachineConfigPool")

		if !progressInsightNotFound {
			logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("Deleting NodeProgressInsight for node without MCP")
			err := r.Delete(ctx, &progressInsight)
			if err != nil {
				logger.Error(err, "Failed to delete NodeProgressInsight")
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	logger.WithValues("Node", req.NamespacedName, "MachineConfigPool", mcpName).V(4).Info("Processing NodeProgressInsight for Node")

	var mcp openshiftmachineconfigurationv1.MachineConfigPool
	mcpErr := r.Get(ctx, client.ObjectKey{Name: mcpName}, &mcp)
	if mcpErr != nil {
		// If the MCP does not exist, it was deleted and its deletion will trigger another reconciliation for all nodes
		return ctrl.Result{}, client.IgnoreNotFound(mcpErr)
	}

	var cv openshiftconfigv1.ClusterVersion
	cvErr := r.Get(ctx, client.ObjectKey{Name: "version"}, &cv)
	if cvErr != nil {
		return ctrl.Result{}, cvErr
	}

	var mostRecentVersionInCVHistory string
	if len(cv.Status.History) > 0 {
		mostRecentVersionInCVHistory = cv.Status.History[0].Version
	}

	now := r.now()

	// Ensure the object exists first
	if progressInsightNotFound {
		progressInsight.Name = node.Name
		if err := r.Create(ctx, &progressInsight); err != nil {
			logger.WithValues("NodeProgressInsight", req.NamespacedName).Error(err, "Failed to create NodeProgressInsight")
			return ctrl.Result{}, err
		}
		// Create() populates progressInsight with resourceVersion, UID, etc. from the server
		logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("Created NodeProgressInsight for Node")
	}

	// When updating an existing insight, preserve existing conditions
	existingConditions := progressInsight.Status.Conditions
	nodeInsight := assessNode(&node, &mcp, r.machineConfigVersions.VersionFor, mostRecentVersionInCVHistory, existingConditions, now)

	// Check if status update is needed
	diff := cmp.Diff(&progressInsight.Status, nodeInsight)
	if diff == "" {
		logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("No changes in NodeProgressInsight, skipping update")
		return ctrl.Result{}, nil
	}

	// Update status
	logger.WithValues("NodeProgressInsight", req.NamespacedName).Info(diff)
	progressInsight.Status = *nodeInsight
	if err := r.Status().Update(ctx, &progressInsight); err != nil {
		if errors.IsConflict(err) {
			// Conflict means another reconciliation updated it, requeue to try again
			logger.V(1).Info("Conflict updating status, will retry", "NodeProgressInsight", req.NamespacedName)
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		logger.WithValues("NodeProgressInsight", req.NamespacedName).Error(err, "Failed to update NodeProgressInsight status")
		return ctrl.Result{}, err
	}
	logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("Updated NodeProgressInsight status for Node")
	return ctrl.Result{}, nil
}

func assessNode(node *corev1.Node, mcp *openshiftmachineconfigurationv1.MachineConfigPool, machineConfigToVersion func(string) (string, bool), mostRecentVersionInCVHistory string, existingConditions []metav1.Condition, now metav1.Time) *ouev1alpha1.NodeProgressInsightStatus {
	if node == nil || mcp == nil {
		return nil
	}

	desiredConfig, ok := node.Annotations[mco.DesiredMachineConfigAnnotationKey]
	noDesiredOnNode := !ok
	currentConfig := node.Annotations[mco.CurrentMachineConfigAnnotationKey]
	currentVersion, foundCurrent := machineConfigToVersion(currentConfig)
	desiredVersion, foundDesired := machineConfigToVersion(desiredConfig)

	lns := mco.NewLayeredNodeState(node)
	isUnavailable := lns.IsUnavailable(mcp)

	isDegraded := nodestate.IsNodeDegraded(node)
	isUpdated := foundCurrent && mostRecentVersionInCVHistory == currentVersion &&
		// The following condition is to handle the multi-arch migration because the version number stays the same there
		(noDesiredOnNode || currentConfig == desiredConfig)

	// foundCurrent makes sure we don't blip phase "updating" for nodes that we are not sure
	// of their actual phase, even though the conservative assumption is that the node is
	// at least updating or is updated.
	isUpdating := !isUpdated && foundCurrent && foundDesired && mostRecentVersionInCVHistory == desiredVersion

	conditions, message, phase := nodestate.DetermineConditions(mcp, node, isUpdating, isUpdated, isUnavailable, isDegraded, lns, existingConditions, now.Time)

	scope := ouev1alpha1.WorkerPoolScope
	if mcp.Name == mco.MachineConfigPoolMaster {
		scope = ouev1alpha1.ControlPlaneScope
	}

	return &ouev1alpha1.NodeProgressInsightStatus{
		Name: node.Name,
		PoolResource: ouev1alpha1.ResourceRef{
			Resource: "machineconfigpools",
			Group:    openshiftmachineconfigurationv1.GroupName,
			Name:     mcp.Name,
		},
		Scope:               scope,
		Version:             currentVersion,
		EstimatedToComplete: nodestate.EstimateFromPhase(phase, conditions),
		Message:             message,
		Conditions:          conditions,
	}
}

func (r *Reconciler) initializeMachineConfigPools(ctx context.Context) error {
	logger := logf.FromContext(ctx)
	var machineConfigPools openshiftmachineconfigurationv1.MachineConfigPoolList
	if err := r.List(ctx, &machineConfigPools, &client.ListOptions{}); err != nil {
		return err
	}

	logger.WithValues("MachineConfigPools", len(machineConfigPools.Items)).Info("Ingesting MachineConfigPools to cache OCP versions")
	for _, pool := range machineConfigPools.Items {
		if ingested, message := r.mcpSelectors.Ingest(pool.Name, pool.Spec.NodeSelector); ingested {
			logger.WithValues("MachineConfigPool", pool.Name).Info(message)
		}
	}
	return nil
}

func (r *Reconciler) initializeMachineConfigVersions(ctx context.Context) error {
	logger := logf.FromContext(ctx)
	var machineConfigs openshiftmachineconfigurationv1.MachineConfigList
	if err := r.List(ctx, &machineConfigs, &client.ListOptions{}); err != nil {
		return err
	}
	logger.WithValues("MachineConfigs", len(machineConfigs.Items)).Info("Ingesting MachineConfigs to cache OCP versions")

	for _, mc := range machineConfigs.Items {
		if ingested, message := r.machineConfigVersions.Ingest(&mc); ingested {
			logger.WithValues("MachineConfig", mc.Name).Info(message)
		}
	}

	return nil
}

func (r *Reconciler) HandleDeletedMachineConfigPool(ctx context.Context, object client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	pool, ok := object.(*openshiftmachineconfigurationv1.MachineConfigPool)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfigPool", object), "Failed to handle deleted MachineConfigPool")
		return nil
	}

	if !r.mcpSelectors.Forget(pool.Name) {
		return nil
	}

	logger.WithValues("MachineConfigPool", pool.Name).Info("MachineConfigPool deleted, removing from cache")

	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		logger.WithValues("MachineConfigPool", pool.Name).Error(err, "Failed to get requests for all nodes")
		return nil
	}
	return requests
}

func (r *Reconciler) HandleMachineConfigPool(ctx context.Context, object client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	pool, ok := object.(*openshiftmachineconfigurationv1.MachineConfigPool)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfigPool", object), "Failed to handle MachineConfigPool")
		return nil
	}

	modified, reason := r.mcpSelectors.Ingest(pool.Name, pool.Spec.NodeSelector)
	if !modified {
		return []reconcile.Request{}
	}

	logger.WithValues("MachineConfigPool", pool.Name).Info("MachineConfigPool changed:", "reason", reason)

	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		logger.WithValues("MachineConfigPool", pool.Name).Error(err, "Failed to get requests for all nodes")
		return nil
	}
	return requests
}

func (r *Reconciler) HandleDeletedMachineConfig(ctx context.Context, object client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	mc, ok := object.(*openshiftmachineconfigurationv1.MachineConfig)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfig", object), "Failed to handle deleted MachineConfig")
		return nil
	}

	if !r.machineConfigVersions.Forget(mc.Name) {
		return nil
	}

	logger.WithValues("MachineConfig", mc.Name).Info("MachineConfig deleted, removing from cache")

	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		logger.WithValues("MachineConfig", mc.Name).Error(err, "Failed to get requests for all nodes")
		return nil
	}
	return requests
}

func (r *Reconciler) HandleMachineConfig(ctx context.Context, object client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	mc, ok := object.(*openshiftmachineconfigurationv1.MachineConfig)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfig", object), "Failed to handle MachineConfig")
		return nil
	}

	modified, reason := r.machineConfigVersions.Ingest(mc)
	if !modified {
		return nil
	}

	logger.WithValues("MachineConfig", mc.Name).Info("MachineConfig changed:", "reason", reason)

	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		logger.WithValues("MachineConfig", mc.Name).Error(err, "Failed to get requests for all nodes")
		return nil
	}

	return requests
}

func (r *Reconciler) AllNodes(ctx context.Context, _ client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		logger.Error(err, "Failed to get requests for all nodes")
		return nil
	}
	return requests
}

func (r *Reconciler) requestsForAllNodes(ctx context.Context) ([]reconcile.Request, error) {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	requests := make([]reconcile.Request, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: node.Name},
		})
	}
	return requests, nil
}
