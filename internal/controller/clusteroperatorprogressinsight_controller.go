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
	"errors"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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

type deploymentGetter func(ctx context.Context, what types.NamespacedName) (appsv1.Deployment, error)

var operatorImageNotImplemented = errors.New("operator-image not implemented in the versions from cluster operator's status")

func getImagePullSpec(ctx context.Context, name string, getDeployment deploymentGetter) (string, error) {
	// It is known that the image pull spec for co/machine-config can be accessed from the deployment
	if name == "machine-config" {
		mcoDeployment, err := getDeployment(ctx, types.NamespacedName{
			Namespace: "openshift-machine-config-operator",
			Name:      "machine-config-operator",
		})
		if err != nil {
			return "", err
		}
		for _, c := range mcoDeployment.Spec.Template.Spec.Containers {
			if c.Name == "machine-config-operator" {
				return c.Image, nil
			}
		}
		return "", errors.New("machine-config-operator container not found")
	}
	// We may add here retrieval of the image pull spec for other COs when they implement "operator-image" in the status.versions
	return "", operatorImageNotImplemented
}

func assessClusterOperator(ctx context.Context, operator *openshiftconfigv1.ClusterOperator, targetVersion string, getDeployment deploymentGetter, now metav1.Time) (*ouev1alpha1.ClusterOperatorProgressInsightStatus, error) {
	updating := metav1.Condition{
		Type:               string(ouev1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:             metav1.ConditionUnknown,
		Reason:             string(ouev1alpha1.ClusterOperatorUpdatingCannotDetermine),
		LastTransitionTime: now,
	}

	imagePullSpec, err := getImagePullSpec(ctx, operator.Name, getDeployment)
	if err != nil && !errors.Is(err, operatorImageNotImplemented) {
		return nil, err
	}

	noOperatorImageVersion := true
	var operatorImageUpdated, versionUpdated bool
	for _, version := range operator.Status.Versions {
		if version.Name == "operator-image" {
			noOperatorImageVersion = false
			if imagePullSpec != "" && imagePullSpec == version.Version {
				operatorImageUpdated = true
			}
		}
		if version.Name == "operator" && version.Version == targetVersion {
			versionUpdated = true
		}
	}

	// "operator-image" might not be implemented by every cluster operator
	updated := (noOperatorImageVersion || operatorImageUpdated) && versionUpdated
	if updated {
		updating.Status = metav1.ConditionFalse
		updating.Reason = string(ouev1alpha1.ClusterOperatorUpdatingReasonUpdated)
	}

	var available *openshiftconfigv1.ClusterOperatorStatusCondition
	var degraded *openshiftconfigv1.ClusterOperatorStatusCondition
	var progressing *openshiftconfigv1.ClusterOperatorStatusCondition

	for _, condition := range operator.Status.Conditions {
		condition := condition
		switch {
		case condition.Type == openshiftconfigv1.OperatorAvailable:
			available = &condition
		case condition.Type == openshiftconfigv1.OperatorDegraded:
			degraded = &condition
		case condition.Type == openshiftconfigv1.OperatorProgressing:
			progressing = &condition
		}
	}

	if !updated && progressing != nil {
		if progressing.Status == openshiftconfigv1.ConditionTrue {
			updating.Status = metav1.ConditionTrue
			updating.Reason = string(ouev1alpha1.ClusterOperatorUpdatingReasonProgressing)
			updating.Message = progressing.Message
		}
		if progressing.Status == openshiftconfigv1.ConditionFalse {
			updating.Status = metav1.ConditionFalse
			updating.Reason = string(ouev1alpha1.ClusterOperatorUpdatingReasonPending)
			updating.Message = progressing.Message
		}
	}

	health := metav1.Condition{
		Type:               string(ouev1alpha1.ClusterOperatorProgressInsightHealthy),
		Status:             metav1.ConditionTrue,
		Reason:             string(ouev1alpha1.ClusterOperatorHealthyReasonAsExpected),
		LastTransitionTime: now,
	}

	if available == nil {
		health.Status = metav1.ConditionUnknown
		health.Reason = string(ouev1alpha1.ClusterOperatorHealthyReasonUnavailable)
		health.Message = "The cluster operator is unavailable because the available condition is not found in the cluster operator's status"
	} else if available.Status != openshiftconfigv1.ConditionTrue {
		health.Status = metav1.ConditionFalse
		health.Reason = string(ouev1alpha1.ClusterOperatorHealthyReasonUnavailable)
		health.Message = available.Message
	} else if degraded != nil && degraded.Status == openshiftconfigv1.ConditionTrue {
		health.Status = metav1.ConditionFalse
		health.Reason = string(ouev1alpha1.ClusterOperatorHealthyReasonDegraded)
		health.Message = degraded.Message
	}

	return &ouev1alpha1.ClusterOperatorProgressInsightStatus{
		Name:       operator.Name,
		Conditions: []metav1.Condition{updating, health},
	}, nil
}

// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusteroperatorprogressinsights/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterOperatorProgressInsight object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ClusterOperatorProgressInsightReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var clusterOperator openshiftconfigv1.ClusterOperator
	coErr := r.Client.Get(ctx, req.NamespacedName, &clusterOperator)
	if coErr != nil && !apierrors.IsNotFound(coErr) {
		logger.WithValues("ClusterOperator", req.NamespacedName).Error(coErr, "Failed to get ClusterOperator")
		return ctrl.Result{}, coErr
	}

	var progressInsight ouev1alpha1.ClusterOperatorProgressInsight
	err := r.Client.Get(ctx, req.NamespacedName, &progressInsight)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Error(err, "Failed to get ClusterOperatorProgressInsight")
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(coErr) && apierrors.IsNotFound(err) {
		// If both ClusterOperator and ClusterOperatorProgressInsight do not exist, we can return early
		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("Both ClusterOperator and ClusterOperatorProgressInsight do not exist, nothing to reconcile")
		return ctrl.Result{}, nil
	}

	if apierrors.IsNotFound(coErr) && !apierrors.IsNotFound(err) {
		// If ClusterOperator does not exist but ClusterOperatorProgressInsight does, we can delete the insight
		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("ClusterOperator does not exist, deleting ClusterOperatorProgressInsight")
		if err := r.Client.Delete(ctx, &progressInsight); err != nil {
			logger.Error(err, "Failed to delete ClusterOperatorProgressInsight")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var clusterVersion openshiftconfigv1.ClusterVersion
	if err := r.Client.Get(ctx, client.ObjectKey{Name: "version"}, &clusterVersion); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.WithValues("ClusterVersion", "version").Error(err, "Failed to get ClusterVersion")
		return ctrl.Result{}, err
	}
	targetVersion := clusterVersion.Status.Desired.Version

	now := r.now()
	getDeployment := func(ctx context.Context, what types.NamespacedName) (appsv1.Deployment, error) {
		var deployment appsv1.Deployment
		err := r.Client.Get(ctx, what, &deployment)
		return deployment, err
	}

	coInsight, err := assessClusterOperator(ctx, &clusterOperator, targetVersion, getDeployment, now)
	if err != nil {
		logger.WithValues("ClusterOperator", req.NamespacedName).Error(err, "Failed to assess ClusterOperator")
		return ctrl.Result{}, err
	}
	progressInsight.Status = *coInsight
	progressInsight.Name = clusterOperator.Name

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, &progressInsight); err != nil {
			logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Error(err, "Failed to create ClusterOperatorProgressInsight")
			return ctrl.Result{}, err
		}
		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("Created ClusterOperatorProgressInsight")
		return ctrl.Result{}, nil
	}

	if err := r.Client.Status().Update(ctx, &progressInsight); err != nil {
		logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Error(err, "Failed to update ClusterOperatorProgressInsight status")
		return ctrl.Result{}, err
	}
	logger.WithValues("ClusterOperatorProgressInsight", req.NamespacedName).Info("Updated ClusterOperatorProgressInsight status")
	return ctrl.Result{}, nil
}

type predicateStartedUpdating struct {
	predicate.Funcs
}

func (p predicateStartedUpdating) Update(e event.UpdateEvent) bool {
	before, ok := e.ObjectOld.(*ouev1alpha1.ClusterVersionProgressInsight)
	if !ok {
		return false
	}
	after, ok := e.ObjectNew.(*ouev1alpha1.ClusterVersionProgressInsight)
	if !ok {
		return false
	}

	updatingName := string(ouev1alpha1.ClusterVersionProgressInsightUpdating)
	updatingAfter := meta.FindStatusCondition(after.Status.Conditions, updatingName)
	if updatingAfter == nil || updatingAfter.Status != metav1.ConditionTrue {
		return false
	}

	updatingBefore := meta.FindStatusCondition(before.Status.Conditions, updatingName)
	return updatingBefore == nil || updatingBefore.Status != metav1.ConditionTrue
}

func (p predicateStartedUpdating) Create(e event.CreateEvent) bool {
	insight, ok := e.Object.(*ouev1alpha1.ClusterVersionProgressInsight)
	if !ok {
		return false
	}

	updatingName := string(ouev1alpha1.ClusterVersionProgressInsightUpdating)
	updating := meta.FindStatusCondition(insight.Status.Conditions, updatingName)
	return updating != nil && updating.Status == metav1.ConditionTrue
}

func (p predicateStartedUpdating) Delete(_ event.DeleteEvent) bool {
	return false
}

func (r *ClusterOperatorProgressInsightReconciler) allClusterOperatorsProgressInsightsMapFunc(ctx context.Context, _ client.Object) []reconcile.Request {
	operators := &openshiftconfigv1.ClusterOperatorList{}
	if err := r.Client.List(ctx, operators); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list ClusterOperators")
		return nil
	}

	var requests []reconcile.Request
	for _, operator := range operators.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: operator.Name},
		})
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterOperatorProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ouev1alpha1.ClusterOperatorProgressInsight{}).
		Owns(&ouev1alpha1.UpdateHealthInsight{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
			return o.GetLabels()[labelUpdateHealthInsightManager] == "clusteroperator"
		}))).
		Named("clusteroperatorprogressinsight").
		Watches(
			&openshiftconfigv1.ClusterOperator{},
			&handler.EnqueueRequestForObject{},
		).
		Watches(
			&ouev1alpha1.ClusterVersionProgressInsight{},
			handler.EnqueueRequestsFromMapFunc(r.allClusterOperatorsProgressInsightsMapFunc),
			builder.WithPredicates(predicateStartedUpdating{}),
		).
		Complete(r)
}
