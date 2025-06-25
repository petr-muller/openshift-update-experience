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
	"sync"

	openshiftmachineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	openshiftv1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
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
		klog.Errorf("Failed to convert to a label selector from the node selector of MachineConfigPool %s: %v", pool.Name, err)
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
	var machineConfigPools openshiftmachineconfigurationv1.MachineConfigPoolList
	if err := r.List(ctx, &machineConfigPools, &client.ListOptions{}); err != nil {
		return err
	}

	for _, pool := range machineConfigPools.Items {
		r.mcpSelectors.ingest(&pool)
	}
	klog.V(2).Infof("Ingested %d machineConfigPools in the cache", len(machineConfigPools.Items))
	return nil
}

func (r *NodeProgressInsightReconciler) initializeMachineConfigVersions(ctx context.Context) error {
	var machineConfigs openshiftmachineconfigurationv1.MachineConfigList
	if err := r.List(ctx, &machineConfigs, &client.ListOptions{}); err != nil {
		return err
	}

	for _, mc := range machineConfigs.Items {
		r.machineConfigVersions.ingest(&mc)
	}

	klog.V(2).Infof("Ingested %d machineConfigs in the cache", len(machineConfigs.Items))
	return nil
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
	_ = logf.FromContext(ctx)

	// Warm up controller's caches.
	// This has to be called after informers caches have been synced and before the first event comes in.
	// The existing openshift-library does not provide such a hook.
	// In case any error occurs during cache initialization, we can still proceed which leads to stale insights that
	// will be corrected on the reconciliation when the caches are warmed up.
	r.once.Do(func() {
		if err := r.initializeMachineConfigPools(ctx); err != nil {
			klog.Warningf("Failed to initialize machineConfigPoolSelectorCache: %v", err)
		}
		if err := r.initializeMachineConfigVersions(ctx); err != nil {
			klog.Warningf("Failed to initialize machineConfigVersions: %v", err)
		}
	})

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
	pool, ok := object.(*openshiftmachineconfigurationv1.MachineConfigPool)
	if !ok {
		klog.Errorf("Object %T is not a MachineConfigPool", object)
		return nil
	}

	if !r.mcpSelectors.forget(pool.Name) {
		return nil
	}

	klog.V(2).Infof("MachineConfigPool %s deleted, removed from cache", pool.Name)

	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		klog.Errorf("Failed to get requests for all nodes: %v", err)
		return nil
	}
	return requests
}

func (r *NodeProgressInsightReconciler) handleMachineConfigPool(ctx context.Context, object client.Object) []reconcile.Request {
	pool, ok := object.(*openshiftmachineconfigurationv1.MachineConfigPool)
	if !ok {
		klog.Errorf("Object %T is not a MachineConfigPool", object)
		return nil
	}

	modified, reason := r.mcpSelectors.ingest(pool)
	if !modified {
		return []reconcile.Request{}
	}

	klog.V(2).Infof("MachineConfigPool %s changed: %s", pool.Name, reason)

	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		klog.Errorf("Failed to get requests for all nodes: %v", err)
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
	mc, ok := object.(*openshiftmachineconfigurationv1.MachineConfig)
	if !ok {
		klog.Errorf("Object %T is not a MachineConfig", object)
		return nil
	}

	if !r.machineConfigVersions.forget(mc.Name) {
		return nil
	}

	klog.V(2).Infof("MachineConfig %s deleted, removed from cache", mc.Name)

	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		klog.Errorf("Failed to get requests for all nodes: %v", err)
		return nil
	}
	return requests
}

func (r *NodeProgressInsightReconciler) handleMachineConfig(ctx context.Context, object client.Object) []reconcile.Request {
	mc, ok := object.(*openshiftmachineconfigurationv1.MachineConfig)
	if !ok {
		klog.Errorf("Object %T is not a MachineConfig", object)
		return nil
	}

	modified, reason := r.machineConfigVersions.ingest(mc)
	if !modified {
		return nil
	}

	klog.V(2).Infof("MachineConfig %s changed: %s", mc.Name, reason)

	requests, err := r.requestsForAllNodes(ctx)
	if err != nil {
		klog.Errorf("Failed to get requests for all nodes: %v", err)
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
		For(&openshiftv1alpha1.NodeProgressInsight{}).
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
