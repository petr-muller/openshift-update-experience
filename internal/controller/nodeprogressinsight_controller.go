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
	"fmt"
	"strings"
	"sync"
	"time"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type machineConfigPoolSelectorCache struct {
	cache sync.Map
}

func (c *machineConfigPoolSelectorCache) whichMCP(l labels.Labels) string {
	var ret string
	c.cache.Range(func(k, v interface{}) bool {
		s := v.(labels.Selector)
		if k == mco.MachineConfigPoolMaster && s.Matches(l) {
			ret = mco.MachineConfigPoolMaster
			return false
		}
		if s.Matches(l) {
			ret = k.(string)
			return ret == mco.MachineConfigPoolWorker
		}
		return true
	})
	return ret
}

func (c *machineConfigPoolSelectorCache) ingest(pool *openshiftmachineconfigurationv1.MachineConfigPool) (bool, string) {
	s, err := metav1.LabelSelectorAsSelector(pool.Spec.NodeSelector)
	if err != nil {
		logger := logf.Log
		logger.WithValues("MachineConfigPool", pool.Name).Error(err, "Failed to convert node selector to label selector")
		v, loaded := c.cache.LoadAndDelete(pool.Name)
		if loaded {
			return true, fmt.Sprintf("the previous selector %s for MachineConfigPool %s deleted as its current node selector cannot be converted to a label selector: %v", v, pool.Name, err)
		} else {
			return false, ""
		}
	}

	previous, loaded := c.cache.Swap(pool.Name, s)
	if !loaded || previous.(labels.Selector).String() != s.String() {
		var vStr string
		if loaded {
			vStr = previous.(labels.Selector).String()
		}
		return true, fmt.Sprintf("selector for MachineConfigPool %s changed from %s to %s", pool.Name, vStr, s.String())
	}
	return false, ""
}

func (c *machineConfigPoolSelectorCache) forget(mcpName string) bool {
	_, loaded := c.cache.LoadAndDelete(mcpName)
	return loaded
}

type machineConfigVersionCache struct {
	cache sync.Map
}

func (c *machineConfigVersionCache) ingest(mc *openshiftmachineconfigurationv1.MachineConfig) (bool, string) {
	if mcVersion, annotated := mc.Annotations[mco.ReleaseImageVersionAnnotationKey]; annotated && mcVersion != "" {
		previous, loaded := c.cache.Swap(mc.Name, mcVersion)
		if !loaded || previous.(string) != mcVersion {
			var previousStr string
			if loaded {
				previousStr = previous.(string)
			}
			return true, fmt.Sprintf("version for MachineConfig %s changed from %s from %s", mc.Name, previousStr, mcVersion)
		} else {
			return false, ""
		}
	}

	_, loaded := c.cache.LoadAndDelete(mc.Name)
	if loaded {
		return true, fmt.Sprintf("the previous version for MachineConfig %s deleted as no version can be found now", mc.Name)
	}
	return false, ""
}

func (c *machineConfigVersionCache) forget(name string) bool {
	_, loaded := c.cache.LoadAndDelete(name)
	return loaded
}

func (c *machineConfigVersionCache) versionFor(key string) (string, bool) {
	v, loaded := c.cache.Load(key)
	if !loaded {
		return "", false
	}
	return v.(string), true
}

// NodeProgressInsightReconciler reconciles a NodeProgressInsight object
type NodeProgressInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// once does the tasks that need to be executed once and only once at the beginning of its sync function
	// for each nodeInformerController instance, e.g., initializing caches.
	once sync.Once

	// mcpSelectors caches the label selectors converted from the node selectors of the machine config pools by their names.
	mcpSelectors machineConfigPoolSelectorCache

	// machineConfigVersions caches machine config versions which stores the name of MC as the key
	// and the release image version as its value retrieved from the annotation of the MC.
	machineConfigVersions machineConfigVersionCache

	now func() metav1.Time
}

// NewNodeProgressInsightReconciler creates a new NodeProgressInsightReconciler
func NewNodeProgressInsightReconciler(client client.Client, scheme *runtime.Scheme) *NodeProgressInsightReconciler {
	return &NodeProgressInsightReconciler{
		Client: client,
		Scheme: scheme,
		now:    metav1.Now,
	}
}

func (r *NodeProgressInsightReconciler) initializeMachineConfigPools(ctx context.Context) error {
	logger := logf.FromContext(ctx)
	var machineConfigPools openshiftmachineconfigurationv1.MachineConfigPoolList
	if err := r.List(ctx, &machineConfigPools, &client.ListOptions{}); err != nil {
		return err
	}

	logger.WithValues("MachineConfigPools", len(machineConfigPools.Items)).Info("Ingesting MachineConfigPools to cache OCP versions")
	for _, pool := range machineConfigPools.Items {
		if ingested, message := r.mcpSelectors.ingest(&pool); ingested {
			logger.WithValues("MachineConfigPool", pool.Name).Info(message)
		}
	}
	return nil
}

func (r *NodeProgressInsightReconciler) initializeMachineConfigVersions(ctx context.Context) error {
	logger := logf.FromContext(ctx)
	var machineConfigs openshiftmachineconfigurationv1.MachineConfigList
	if err := r.List(ctx, &machineConfigs, &client.ListOptions{}); err != nil {
		return err
	}
	logger.WithValues("MachineConfigs", len(machineConfigs.Items)).Info("Ingesting MachineConfigs to cache OCP versions")

	for _, mc := range machineConfigs.Items {
		if ingested, message := r.machineConfigVersions.ingest(&mc); ingested {
			logger.WithValues("MachineConfig", mc.Name).Info(message)
		}
	}

	return nil
}

func isNodeDegraded(node *corev1.Node) bool {
	// Inspired by: https://github.com/openshift/machine-config-operator/blob/master/pkg/controller/node/status.go
	if node.Annotations == nil {
		return false
	}
	dconfig, ok := node.Annotations[mco.DesiredMachineConfigAnnotationKey]
	if !ok || dconfig == "" {
		return false
	}
	dstate, ok := node.Annotations[mco.MachineConfigDaemonStateAnnotationKey]
	if !ok || dstate == "" {
		return false
	}

	if dstate == mco.MachineConfigDaemonStateDegraded || dstate == mco.MachineConfigDaemonStateUnreconcilable {
		return true
	}
	return false
}

func toPointer(d time.Duration) *metav1.Duration {
	v := metav1.Duration{Duration: d}
	return &v
}
func isNodeDraining(node *corev1.Node, isUpdating bool) bool {
	desiredDrain := node.Annotations[mco.DesiredDrainerAnnotationKey]
	appliedDrain := node.Annotations[mco.LastAppliedDrainerAnnotationKey]

	if appliedDrain == "" || desiredDrain == "" {
		return false
	}

	if desiredDrain != appliedDrain {
		desiredVerb := strings.Split(desiredDrain, "-")[0]
		if desiredVerb == mco.DrainerStateDrain {
			return true
		}
	}

	// Node is supposed to be updating but MCD hasn't had the time to update
	// its state from original `Done` to `Working` and start the drain process.
	// Default to drain process so that we don't report completed.
	mcdState := node.Annotations[mco.MachineConfigDaemonStateAnnotationKey]
	return isUpdating && mcdState == mco.MachineConfigDaemonStateDone
}

func determineConditions(pool *openshiftmachineconfigurationv1.MachineConfigPool, node *corev1.Node, isUpdating, isUpdated, isUnavailable, isDegraded bool, lns *mco.LayeredNodeState, now metav1.Time) ([]metav1.Condition, string, *metav1.Duration) {
	var estimate *metav1.Duration

	updating := metav1.Condition{
		Type:               string(ouev1alpha1.NodeStatusInsightUpdating),
		Status:             metav1.ConditionUnknown,
		Reason:             string(ouev1alpha1.NodeCannotDetermine),
		Message:            "Cannot determine whether the node is updating",
		LastTransitionTime: now,
	}
	available := metav1.Condition{
		Type:               string(ouev1alpha1.NodeStatusInsightAvailable),
		Status:             metav1.ConditionTrue,
		Reason:             "AsExpected",
		Message:            "The node is available",
		LastTransitionTime: now,
	}
	degraded := metav1.Condition{
		Type:               string(ouev1alpha1.NodeStatusInsightDegraded),
		Status:             metav1.ConditionFalse,
		Reason:             "AsExpected",
		Message:            "The node is not degraded",
		LastTransitionTime: now,
	}

	if isUpdating && isNodeDraining(node, isUpdating) {
		estimate = toPointer(10 * time.Minute)
		updating.Status = metav1.ConditionTrue
		updating.Reason = string(ouev1alpha1.NodeDraining)
		updating.Message = "The node is draining"
	} else if isUpdating {
		state := node.Annotations[mco.MachineConfigDaemonStateAnnotationKey]
		switch state {
		case mco.MachineConfigDaemonStateRebooting:
			estimate = toPointer(10 * time.Minute)
			updating.Status = metav1.ConditionTrue
			updating.Reason = string(ouev1alpha1.NodeRebooting)
			updating.Message = "The node is rebooting"
		case mco.MachineConfigDaemonStateDone:
			estimate = toPointer(time.Duration(0))
			updating.Status = metav1.ConditionFalse
			updating.Reason = string(ouev1alpha1.NodeCompleted)
			updating.Message = "The node is updated"
		default:
			estimate = toPointer(10 * time.Minute)
			updating.Status = metav1.ConditionTrue
			updating.Reason = string(ouev1alpha1.NodeUpdating)
			updating.Message = "The node is updating"
		}

	} else if isUpdated {
		estimate = toPointer(time.Duration(0))
		updating.Status = metav1.ConditionFalse
		updating.Reason = string(ouev1alpha1.NodeCompleted)
		updating.Message = "The node is updated"
	} else if pool.Spec.Paused {
		estimate = toPointer(time.Duration(0))
		updating.Status = metav1.ConditionFalse
		updating.Reason = string(ouev1alpha1.NodePaused)
		updating.Message = "The update of the node is paused"
	} else {
		updating.Status = metav1.ConditionFalse
		updating.Reason = string(ouev1alpha1.NodeUpdatePending)
		updating.Message = "The update of the node is pending"
	}

	// ATM, the insight's message is set only for the interesting cases: (isUnavailable && !isUpdating) || isDegraded
	// Moreover, the degraded message overwrites the unavailable one.
	// Those cases are inherited from the "oc adm upgrade" command as the baseline for the insight's message.
	// https://github.com/openshift/oc/blob/0cd37758b5ebb182ea911c157256c1b812c216c5/pkg/cli/admin/upgrade/status/workerpool.go#L194
	// We may add more cases in the future as needed
	var message string
	if isUnavailable && !isUpdating {
		estimate = nil
		if isUpdated {
			estimate = toPointer(time.Duration(0))
		}
		available.Status = metav1.ConditionFalse
		available.Reason = lns.GetUnavailableReason()
		available.Message = lns.GetUnavailableMessage()
		available.LastTransitionTime = metav1.Time{Time: lns.GetUnavailableSince()}
		message = available.Message
	}

	if isDegraded {
		estimate = nil
		if isUpdated {
			estimate = toPointer(time.Duration(0))
		}
		degraded.Status = metav1.ConditionTrue
		degraded.Reason = node.Annotations[mco.MachineConfigDaemonReasonAnnotationKey]
		degraded.Message = node.Annotations[mco.MachineConfigDaemonReasonAnnotationKey]
		message = degraded.Message
	}

	return []metav1.Condition{updating, available, degraded}, message, estimate
}

func assessNode(node *corev1.Node, mcp *openshiftmachineconfigurationv1.MachineConfigPool, machineConfigToVersion func(string) (string, bool), mostRecentVersionInCVHistory string, now metav1.Time) *ouev1alpha1.NodeProgressInsightStatus {
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

	isDegraded := isNodeDegraded(node)
	isUpdated := foundCurrent && mostRecentVersionInCVHistory == currentVersion &&
		// The following condition is to handle the multi-arch migration because the version number stays the same there
		(noDesiredOnNode || currentConfig == desiredConfig)

	// foundCurrent makes sure we don't blip phase "updating" for nodes that we are not sure
	// of their actual phase, even though the conservative assumption is that the node is
	// at least updating or is updated.
	isUpdating := !isUpdated && foundCurrent && foundDesired && mostRecentVersionInCVHistory == desiredVersion

	conditions, message, estimate := determineConditions(mcp, node, isUpdating, isUpdated, isUnavailable, isDegraded, lns, now)

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
		EstimatedToComplete: estimate,
		Message:             message,
		Conditions:          conditions,
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
		} else {
			logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("Node does not exist, deleting NodeProgressInsight")
			err := r.Delete(ctx, &progressInsight)
			if err != nil {
				logger.Error(err, "Failed to delete NodeProgressInsight")
			}
			return ctrl.Result{}, err
		}
	}

	mcpName := r.mcpSelectors.whichMCP(labels.Set(node.Labels))
	if mcpName == "" {
		// We assume that every node belongs to an MCP at all time.
		// Although conceptually the assumption might not be true (see https://docs.openshift.com/container-platform/4.17/machine_configuration/index.html#architecture-machine-config-pools_machine-config-overview),
		// we will wait to hear from our users the issues for cluster updates and will handle them accordingly by then.
		logger.WithValues("Node", req.NamespacedName).Info("Node does not belong to any MachineConfigPool, skipping reconciliation")
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
	nodeInsight := assessNode(&node, &mcp, r.machineConfigVersions.versionFor, mostRecentVersionInCVHistory, now)
	progressInsight.Status = *nodeInsight
	progressInsight.Name = node.Name

	if progressInsightNotFound {
		if err := r.Create(ctx, &progressInsight); err != nil {
			logger.WithValues("NodeProgressInsight", req.NamespacedName).Error(err, "Failed to create NodeProgressInsight")
			return ctrl.Result{}, err
		}
		logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("Created NodeProgressInsight for Node")
		return ctrl.Result{}, nil
	}

	if err := r.Status().Update(ctx, &progressInsight); err != nil {
		logger.WithValues("NodeProgressInsight", req.NamespacedName).Error(err, "Failed to update NodeProgressInsight status")
		return ctrl.Result{}, err
	}
	logger.WithValues("NodeProgressInsight", req.NamespacedName).Info("Updated NodeProgressInsight status for Node")
	return ctrl.Result{}, nil
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

func (r *NodeProgressInsightReconciler) handleDeletedMachineConfigPool(ctx context.Context, object client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	pool, ok := object.(*openshiftmachineconfigurationv1.MachineConfigPool)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfigPool", object), "Failed to handle deleted MachineConfigPool")
		return nil
	}

	if !r.mcpSelectors.forget(pool.Name) {
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

func (r *NodeProgressInsightReconciler) handleMachineConfigPool(ctx context.Context, object client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	pool, ok := object.(*openshiftmachineconfigurationv1.MachineConfigPool)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfigPool", object), "Failed to handle MachineConfigPool")
		return nil
	}

	modified, reason := r.mcpSelectors.ingest(pool)
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

func (r *NodeProgressInsightReconciler) handleDeletedMachineConfig(ctx context.Context, object client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	mc, ok := object.(*openshiftmachineconfigurationv1.MachineConfig)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfig", object), "Failed to handle deleted MachineConfig")
		return nil
	}

	if !r.machineConfigVersions.forget(mc.Name) {
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

func (r *NodeProgressInsightReconciler) handleMachineConfig(ctx context.Context, object client.Object) []reconcile.Request {
	logger := logf.FromContext(ctx)
	mc, ok := object.(*openshiftmachineconfigurationv1.MachineConfig)
	if !ok {
		logger.Error(fmt.Errorf("object %T is not a MachineConfig", object), "Failed to handle MachineConfig")
		return nil
	}

	modified, reason := r.machineConfigVersions.ingest(mc)
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

func (r *NodeProgressInsightReconciler) requestsForAllNodes(ctx context.Context) ([]reconcile.Request, error) {
	// Reconcile all NodeProgressInsights as the change in the MachineConfigPool selector can potentially affect all of them.
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

// SetupWithManager sets up the controller with the Manager.
func (r *NodeProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ouev1alpha1.NodeProgressInsight{}).
		Named("nodeprogressinsight").
		Watches(
			&openshiftmachineconfigurationv1.MachineConfigPool{},
			handler.EnqueueRequestsFromMapFunc(r.handleDeletedMachineConfigPool),
			builder.WithPredicates(mcpDeleted),
		).
		Watches(
			&openshiftmachineconfigurationv1.MachineConfigPool{},
			handler.EnqueueRequestsFromMapFunc(r.handleMachineConfigPool),
			builder.WithPredicates(mcpSelectorEvents),
		).
		Watches(
			&openshiftmachineconfigurationv1.MachineConfig{},
			handler.EnqueueRequestsFromMapFunc(r.handleDeletedMachineConfig),
			builder.WithPredicates(mcDeleted),
		).
		Watches(
			&openshiftmachineconfigurationv1.MachineConfig{},
			handler.EnqueueRequestsFromMapFunc(r.handleMachineConfig),
			builder.WithPredicates(mcVersionEvents),
		).
		Complete(r)
}
