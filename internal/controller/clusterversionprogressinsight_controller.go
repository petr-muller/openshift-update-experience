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

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/petr-muller/openshift-update-experience/internal/health"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/controller/clusterversions"
)

const (
	controllerName = "clusterversionprogressinsight"
)

// ClusterVersionProgressInsightReconciler reconciles a ClusterVersionProgressInsight object
type ClusterVersionProgressInsightReconciler struct {
	client.Client

	impl *clusterversions.Reconciler
}

// NewClusterVersionProgressInsightReconciler creates a new ClusterVersionProgressInsightReconciler with the given client and scheme.
func NewClusterVersionProgressInsightReconciler(client client.Client, scheme *runtime.Scheme) *ClusterVersionProgressInsightReconciler {
	return &ClusterVersionProgressInsightReconciler{
		impl: clusterversions.NewReconciler(client, scheme),
	}
}

// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusterversionprogressinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusterversionprogressinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusterversionprogressinsights/finalizers,verbs=update
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=updatehealthinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=updatehealthinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=updatehealthinsights/finalizers,verbs=update
// +kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterVersionProgressInsight object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ClusterVersionProgressInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.impl.Reconcile(ctx, req)
}

var coOperatorVersionChanged = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		before, ok := e.ObjectOld.(*openshiftconfigv1.ClusterOperator)
		if !ok {
			return false
		}
		after, ok := e.ObjectNew.(*openshiftconfigv1.ClusterOperator)
		if !ok {
			return false
		}

		var oldVersion string
		for i, version := range before.Status.Versions {
			if version.Name == "operator" {
				oldVersion = before.Status.Versions[i].Version
				break
			}
		}
		var newVersion string
		for i, version := range after.Status.Versions {
			if version.Name == "operator" {
				newVersion = after.Status.Versions[i].Version
				break
			}
		}
		return oldVersion != newVersion
	},
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterVersionProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ouev1alpha1.ClusterVersionProgressInsight{}).
		Owns(
			&ouev1alpha1.UpdateHealthInsight{},
			builder.WithPredicates(health.PredicateForInsightsManagedBy(controllerName)),
		).
		Named("clusterversionprogressinsight").
		Watches(
			&openshiftconfigv1.ClusterVersion{},
			&handler.EnqueueRequestForObject{},
		).
		Watches(
			&openshiftconfigv1.ClusterOperator{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []ctrl.Request {
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "version"}}}
			}),
			// TODO(muller): Only OCP payload ClusterOperators (label/annotation)
			builder.WithPredicates(coOperatorVersionChanged),
		).
		Complete(r)
}
