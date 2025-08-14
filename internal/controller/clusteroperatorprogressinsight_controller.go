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
	"github.com/petr-muller/openshift-update-experience/internal/clusteroperators"
	"github.com/petr-muller/openshift-update-experience/internal/health"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	cocontroller "github.com/petr-muller/openshift-update-experience/internal/controller/clusteroperators"
)

// ClusterOperatorProgressInsightReconciler reconciles a ClusterOperatorProgressInsight object
type ClusterOperatorProgressInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	impl *cocontroller.Reconciler
}

// NewClusterOperatorProgressInsightReconciler creates a new ClusterOperatorProgressInsightReconciler with the given client and scheme.
func NewClusterOperatorProgressInsightReconciler(client client.Client, scheme *runtime.Scheme) *ClusterOperatorProgressInsightReconciler {
	return &ClusterOperatorProgressInsightReconciler{
		Client: client,
		Scheme: scheme,

		impl: cocontroller.NewReconciler(client),
	}
}

// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterOperatorProgressInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.impl.Reconcile(ctx, req)
}

type cvDesiredVersionChanged struct {
	predicate.Funcs
}

func (p cvDesiredVersionChanged) Update(e event.UpdateEvent) bool {
	beforeObj := e.ObjectOld
	afterObj := e.ObjectNew

	before, ok := beforeObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	after, ok := afterObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	return before.Status.Desired.Version != after.Status.Desired.Version
}

type cvHistoryChanged struct {
	predicate.Funcs
}

func (p cvHistoryChanged) Update(e event.UpdateEvent) bool {
	beforeObj := e.ObjectOld
	afterObj := e.ObjectNew

	before, ok := beforeObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	after, ok := afterObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	if len(before.Status.History) == 0 && len(after.Status.History) == 0 {
		return false
	}

	if len(before.Status.History) != len(after.Status.History) {
		return true
	}

	topBefore := before.Status.History[0]
	topAfter := after.Status.History[0]

	return topBefore.State != topAfter.State ||
		topBefore.Version != topAfter.Version ||
		topBefore.Image != topAfter.Image
}

type cvProgressingChanged struct {
	predicate.Funcs
}

func (p cvProgressingChanged) Update(e event.UpdateEvent) bool {
	beforeObj := e.ObjectOld
	afterObj := e.ObjectNew

	before, ok := beforeObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}
	after, ok := afterObj.(*openshiftconfigv1.ClusterVersion)
	if !ok {
		return false
	}

	progressingBefore := clusteroperators.FindOperatorStatusCondition(before.Status.Conditions, openshiftconfigv1.OperatorProgressing)
	progressingAfter := clusteroperators.FindOperatorStatusCondition(after.Status.Conditions, openshiftconfigv1.OperatorProgressing)

	if progressingBefore == nil && progressingAfter == nil {
		return false
	}

	if progressingBefore == nil || progressingAfter == nil {
		return true
	}

	return progressingBefore.Status != progressingAfter.Status
}

func allClusterOperators(c client.Client) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		operators := &openshiftconfigv1.ClusterOperatorList{}
		if err := c.List(ctx, operators); err != nil {
			logf.FromContext(ctx).Error(err, "Failed to list ClusterOperators")
			// TODO(muller): Dropping is not ideal but not many good options here. Maybe we use a special
			// reconcile key to trigger a special all-operator reconciliation
			return nil
		}

		if len(operators.Items) == 0 {
			return nil
		}

		requests := make([]reconcile.Request, 0, len(operators.Items))
		for _, operator := range operators.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: operator.Name},
			})
		}
		return requests
	}
}

var cvVersion = predicate.NewPredicateFuncs(
	func(obj client.Object) bool {
		return obj.GetName() == "version"
	},
)

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterOperatorProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ouev1alpha1.ClusterOperatorProgressInsight{}).
		Owns(&ouev1alpha1.UpdateHealthInsight{},
			builder.WithPredicates(
				predicate.NewPredicateFuncs(func(o client.Object) bool {
					return o.GetLabels()[health.InsightManagerLabel] == "clusteroperator"
				}),
			),
		).
		Named("clusteroperatorprogressinsight").
		Watches(
			&openshiftconfigv1.ClusterOperator{},
			&handler.EnqueueRequestForObject{},
			// TODO(muller): Only OCP payload ClusterOperators (label/annotation)
		).
		Watches(
			&openshiftconfigv1.ClusterVersion{},
			handler.EnqueueRequestsFromMapFunc(allClusterOperators(mgr.GetClient())),
			builder.WithPredicates(
				cvVersion,
				predicate.Or(cvProgressingChanged{}, cvHistoryChanged{}, cvDesiredVersionChanged{}),
			),
		).
		Complete(r)
}
