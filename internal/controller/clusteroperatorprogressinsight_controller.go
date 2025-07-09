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

	"github.com/google/go-cmp/cmp"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

// ClusterOperatorProgressInsightReconciler reconciles a ClusterOperatorProgressInsight object
type ClusterOperatorProgressInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	now func() metav1.Time
}

// NewClusterOperatorProgressInsightReconciler creates a new ClusterOperatorProgressInsightReconciler with the given client and scheme.
func NewClusterOperatorProgressInsightReconciler(client client.Client, scheme *runtime.Scheme) *ClusterOperatorProgressInsightReconciler {
	return &ClusterOperatorProgressInsightReconciler{
		Client: client,
		Scheme: scheme,
		now:    metav1.Now,
	}
}

type deploymentGetter func(ctx context.Context, what types.NamespacedName) (appsv1.Deployment, error)

const operatorVersionName = "operator"

// var errOperatorImageNotImplemented = errors.New("operator-image not implemented in the versions from cluster operator's status")

// func getImagePullSpec(ctx context.Context, name string, getDeployment deploymentGetter) (string, error) {
// 	// It is known that the image pull spec for co/machine-config can be accessed from the deployment
// 	if name == "machine-config" {
// 		mcoDeployment, err := getDeployment(ctx, types.NamespacedName{
// 			Namespace: "openshift-machine-config-operator",
// 			Name:      "machine-config-operator",
// 		})
// 		if err != nil {
// 			return "", err
// 		}
// 		for _, c := range mcoDeployment.Spec.Template.Spec.Containers {
// 			if c.Name == "machine-config-operator" {
// 				return c.Image, nil
// 			}
// 		}
// 		return "", errors.New("machine-config-operator container not found")
// 	}
// 	// We may add here retrieval of the image pull spec for other COs when they implement "operator-image" in the status.versions
// 	return "", errOperatorImageNotImplemented
// }

func assessClusterOperator(_ context.Context, operator *openshiftconfigv1.ClusterOperator, targetVersion string, _ deploymentGetter, now metav1.Time) *ouev1alpha1.ClusterOperatorProgressInsightStatus {
	updating := metav1.Condition{
		Type:               string(ouev1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:             metav1.ConditionUnknown,
		Reason:             string(ouev1alpha1.ClusterOperatorUpdatingCannotDetermine),
		LastTransitionTime: now,
	}

	// imagePullSpec, err := getImagePullSpec(ctx, operator.Name, getDeployment)
	// if err != nil && !errors.Is(err, errOperatorImageNotImplemented) {
	// 	return nil, err
	// }

	// noOperatorImageVersion := true
	// var operatorImageUpdated bool
	var versionUpdated bool
	for _, version := range operator.Status.Versions {
		// if version.Name == "operator-image" {
		// 	noOperatorImageVersion = false
		// 	if imagePullSpec != "" && imagePullSpec == version.Version {
		// 		operatorImageUpdated = true
		// 	}
		// }
		if version.Name == operatorVersionName && version.Version == targetVersion {
			versionUpdated = true
		}
	}

	// var available *openshiftconfigv1.ClusterOperatorStatusCondition
	// var degraded *openshiftconfigv1.ClusterOperatorStatusCondition
	var progressing *openshiftconfigv1.ClusterOperatorStatusCondition

	for _, condition := range operator.Status.Conditions {
		switch condition.Type {
		// case openshiftconfigv1.OperatorAvailable:
		// 	available = &condition
		// case openshiftconfigv1.OperatorDegraded:
		// 	degraded = &condition
		case openshiftconfigv1.OperatorProgressing:
			progressing = &condition
		}
	}

	// "operator-image" might not be implemented by every cluster operator
	// updated := (noOperatorImageVersion || operatorImageUpdated) && versionUpdated
	updated := versionUpdated
	if updated {
		updating.Status = metav1.ConditionFalse
		updating.Reason = string(ouev1alpha1.ClusterOperatorUpdatingReasonUpdated)
	}

	if progressing != nil {
		updating.Message = fmt.Sprintf("Progressing=%s: %s", progressing.Status, progressing.Message)
		if !updated {
			if progressing.Status == openshiftconfigv1.ConditionTrue {
				updating.Status = metav1.ConditionTrue
				updating.Reason = string(ouev1alpha1.ClusterOperatorUpdatingReasonProgressing)
			}
			if progressing.Status == openshiftconfigv1.ConditionFalse {
				updating.Status = metav1.ConditionFalse
				updating.Reason = string(ouev1alpha1.ClusterOperatorUpdatingReasonPending)
			}
		}
	}

	// health := metav1.Condition{
	// 	Type:               string(ouev1alpha1.ClusterOperatorProgressInsightHealthy),
	// 	Status:             metav1.ConditionTrue,
	// 	Reason:             string(ouev1alpha1.ClusterOperatorHealthyReasonAsExpected),
	// 	LastTransitionTime: now,
	// }

	// if available == nil {
	// 	health.Status = metav1.ConditionUnknown
	// 	health.Reason = string(ouev1alpha1.ClusterOperatorHealthyReasonUnavailable)
	// 	health.Message = "The cluster operator is unavailable because the available condition is not found in the cluster operator's status"
	// } else if available.Status != openshiftconfigv1.ConditionTrue {
	// 	health.Status = metav1.ConditionFalse
	// 	health.Reason = string(ouev1alpha1.ClusterOperatorHealthyReasonUnavailable)
	// 	health.Message = available.Message
	// } else if degraded != nil && degraded.Status == openshiftconfigv1.ConditionTrue {
	// 	health.Status = metav1.ConditionFalse
	// 	health.Reason = string(ouev1alpha1.ClusterOperatorHealthyReasonDegraded)
	// 	health.Message = degraded.Message
	// }

	return &ouev1alpha1.ClusterOperatorProgressInsightStatus{
		Name: operator.Name,
		// Conditions: []metav1.Condition{updating, health},
		Conditions: []metav1.Condition{updating},
	}
}

// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterOperatorProgressInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var clusterOperator openshiftconfigv1.ClusterOperator
	coErr := r.Get(ctx, req.NamespacedName, &clusterOperator)
	coNotFound := apierrors.IsNotFound(coErr)
	if coErr != nil && !coNotFound {
		logger.WithValues("ClusterOperator", req.NamespacedName).Error(coErr, "Failed to get ClusterOperator")
		return ctrl.Result{}, coErr
	}

	var progressInsight ouev1alpha1.ClusterOperatorProgressInsight
	piErr := r.Get(ctx, req.NamespacedName, &progressInsight)
	progressInsightNotFound := apierrors.IsNotFound(piErr)
	if piErr != nil && !progressInsightNotFound {
		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Error(piErr, "Failed to get ClusterOperatorProgressInsight")
		return ctrl.Result{}, piErr
	}

	if coNotFound {
		if progressInsightNotFound {
			// If both ClusterOperator and ClusterOperatorProgressInsight do not exist, we can return early
			logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("Both ClusterOperator and ClusterOperatorProgressInsight do not exist, nothing to reconcile")
		} else {
			logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("ClusterOperator does not exist, deleting ClusterOperatorProgressInsight")
			if err := r.Delete(ctx, &progressInsight); err != nil {
				logger.Error(err, "Failed to delete ClusterOperatorProgressInsight")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	var clusterVersion openshiftconfigv1.ClusterVersion
	if err := r.Get(ctx, client.ObjectKey{Name: "version"}, &clusterVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.WithValues("ClusterVersion", "version").Error(err, "Failed to get ClusterVersion")
		return ctrl.Result{}, err
	}
	targetVersion := clusterVersion.Status.Desired.Version

	now := r.now()
	// getDeployment := func(ctx context.Context, what types.NamespacedName) (appsv1.Deployment, error) {
	// 	var deployment appsv1.Deployment
	// 	err := r.Get(ctx, what, &deployment)
	// 	return deployment, err
	// }

	// coInsight, err := assessClusterOperator(ctx, &clusterOperator, targetVersion, getDeployment, now)
	coInsight := assessClusterOperator(ctx, &clusterOperator, targetVersion, nil, now)
	// if err != nil {
	// 	logger.WithValues("ClusterOperator", req.NamespacedName).Error(err, "Failed to assess ClusterOperator")
	// 	return ctrl.Result{}, err
	// }

	progressInsight.Name = clusterOperator.Name

	if progressInsightNotFound {
		if err := r.Create(ctx, &progressInsight); err != nil {
			logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Error(err, "Failed to create ClusterOperatorProgressInsight")
			return ctrl.Result{}, err
		}

		progressInsight.Status = *coInsight
		if err := r.Status().Update(ctx, &progressInsight); err != nil {
			logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Error(err, "Failed to update ClusterOperatorProgressInsight status")
			return ctrl.Result{}, err
		}

		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("Created ClusterOperatorProgressInsight")
		return ctrl.Result{}, nil
	}

	diff := cmp.Diff(&progressInsight.Status, coInsight)
	if diff == "" {
		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("No changes in ClusterOperatorProgressInsight, skipping update")
		return ctrl.Result{}, nil
	}
	logger.Info(diff)
	progressInsight.Status = *coInsight

	if err := r.Client.Status().Update(ctx, &progressInsight); err != nil {
		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Error(err, "Failed to update ClusterOperatorProgressInsight status")
		return ctrl.Result{}, err
	}
	logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("Updated ClusterOperatorProgressInsight status")
	return ctrl.Result{}, nil
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

	progressingBefore := findOperatorStatusCondition(before.Status.Conditions, openshiftconfigv1.OperatorProgressing)
	progressingAfter := findOperatorStatusCondition(after.Status.Conditions, openshiftconfigv1.OperatorProgressing)

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
					return o.GetLabels()[labelUpdateHealthInsightManager] == "clusteroperator"
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
