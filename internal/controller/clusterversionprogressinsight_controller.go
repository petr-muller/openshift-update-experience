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
	"time"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

// ClusterVersionProgressInsightReconciler reconciles a ClusterVersionProgressInsight object
type ClusterVersionProgressInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	now func() metav1.Time
}

// NewClusterVersionProgressInsightReconciler creates a new ClusterVersionProgressInsightReconciler with the given client and scheme.
func NewClusterVersionProgressInsightReconciler(client client.Client, scheme *runtime.Scheme) *ClusterVersionProgressInsightReconciler {
	return &ClusterVersionProgressInsightReconciler{
		Client: client,
		Scheme: scheme,
		now:    metav1.Now,
	}
}

func findOperatorStatusCondition(conditions []openshiftconfigv1.ClusterOperatorStatusCondition, conditionType openshiftconfigv1.ClusterStatusConditionType) *openshiftconfigv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func setCannotDetermineUpdating(cond *metav1.Condition, message string) {
	cond.Status = metav1.ConditionUnknown
	cond.Reason = string(ouev1alpha1.ClusterVersionCannotDetermineUpdating)
	cond.Message = message
}

// cvProgressingToUpdating returns a status, reason and message for the updating condition based on the cvProgressing
// condition.
func cvProgressingToUpdating(cvProgressing openshiftconfigv1.ClusterOperatorStatusCondition) (metav1.ConditionStatus, string, string) {
	status := metav1.ConditionStatus(cvProgressing.Status)
	var reason string
	switch status {
	case metav1.ConditionTrue:
		reason = string(ouev1alpha1.ClusterVersionProgressing)
	case metav1.ConditionFalse:
		reason = string(ouev1alpha1.ClusterVersionNotProgressing)
	case metav1.ConditionUnknown:
		reason = string(ouev1alpha1.ClusterVersionCannotDetermineUpdating)
	default:
		reason = string(ouev1alpha1.ClusterVersionCannotDetermineUpdating)
	}

	message := fmt.Sprintf("ClusterVersion has Progressing=%s(Reason=%s) | Message='%s'", cvProgressing.Status, cvProgressing.Reason, cvProgressing.Message)
	return status, reason, message
}

// isControlPlaneUpdating determines whether the control plane is updating based on the ClusterVersion's Progressing
// condition and the last history item. It returns an updating condition, the time the update started, and the time the
// update completed. If the updating condition cannot be determined, the condition will have Status=Unknown and the
// Reason and Message fields will explain why.
func isControlPlaneUpdating(cvProgressing *openshiftconfigv1.ClusterOperatorStatusCondition, lastHistoryItem *openshiftconfigv1.UpdateHistory) (metav1.Condition, metav1.Time, metav1.Time) {
	updating := metav1.Condition{
		Type: string(ouev1alpha1.ClusterVersionProgressInsightUpdating),
	}

	if cvProgressing == nil {
		setCannotDetermineUpdating(&updating, "No Progressing condition in ClusterVersion")
		return updating, metav1.Time{}, metav1.Time{}
	}
	if lastHistoryItem == nil {
		setCannotDetermineUpdating(&updating, "Empty history in ClusterVersion")
		return updating, metav1.Time{}, metav1.Time{}
	}

	updating.Status, updating.Reason, updating.Message = cvProgressingToUpdating(*cvProgressing)

	var started metav1.Time
	// Looks like we are updating
	if cvProgressing.Status == openshiftconfigv1.ConditionTrue {
		if lastHistoryItem.State != openshiftconfigv1.PartialUpdate {
			setCannotDetermineUpdating(&updating, "Progressing=True in ClusterVersion but last history item is not Partial")
		} else if lastHistoryItem.CompletionTime != nil {
			setCannotDetermineUpdating(&updating, "Progressing=True in ClusterVersion but last history item has completion time")
		} else {
			started = lastHistoryItem.StartedTime
		}
	}

	var completed metav1.Time
	// Looks like we are not updating
	if cvProgressing.Status == openshiftconfigv1.ConditionFalse {
		if lastHistoryItem.State != openshiftconfigv1.CompletedUpdate {
			setCannotDetermineUpdating(&updating, "Progressing=False in ClusterVersion but last history item is not completed")
		} else if lastHistoryItem.CompletionTime == nil {
			setCannotDetermineUpdating(&updating, "Progressing=False in ClusterVersion but not no completion in last history item")
		} else {
			started = lastHistoryItem.StartedTime
			completed = *lastHistoryItem.CompletionTime
		}
	}

	return updating, started, completed
}

// versionsFromHistory returns a ControlPlaneUpdateVersions struct with the target version and metadata from the given
// history.
func versionsFromHistory(history []openshiftconfigv1.UpdateHistory) ouev1alpha1.ControlPlaneUpdateVersions {
	var versions ouev1alpha1.ControlPlaneUpdateVersions

	if len(history) == 0 {
		return versions
	}

	versions.Target.Version = history[0].Version

	if len(history) == 1 {
		versions.Target.Metadata = []ouev1alpha1.VersionMetadata{{Key: ouev1alpha1.InstallationMetadata}}
	}
	if len(history) > 1 {
		versions.Previous = &ouev1alpha1.Version{Version: history[1].Version}
		if history[1].State == openshiftconfigv1.PartialUpdate {
			versions.Previous.Metadata = []ouev1alpha1.VersionMetadata{{Key: ouev1alpha1.PartialMetadata}}
		}
	}
	return versions
}

// estimateCompletion returns a time.Time that is 60 minutes after the given time. Proper estimation needs to be added
// once the controller starts handling ClusterOperators.
func estimateCompletion(started time.Time) time.Time {
	return started.Add(60 * time.Minute)
}

// assessClusterVersion produces a ClusterVersion status insight from the current state of the ClusterVersion resource.
// It does not take previous status insight into account. Many fields of the status insights (such as completion) cannot
// be properly calculated without also watching and processing ClusterOperators, so that functionality will need to be
// added later.
// TODO(muller): Port HealthInsights later
// func assessClusterVersion(cv *openshiftconfigv1.ClusterVersion, now metav1.Time) (*ouev1alpha1.ClusterVersionProgressInsightStatus, []*ouev1alpha1.HealthInsightStatus) {
func assessClusterVersion(cv *openshiftconfigv1.ClusterVersion, now metav1.Time) *ouev1alpha1.ClusterVersionProgressInsightStatus {

	var lastHistoryItem *openshiftconfigv1.UpdateHistory
	if len(cv.Status.History) > 0 {
		lastHistoryItem = &cv.Status.History[0]
	}
	cvProgressing := findOperatorStatusCondition(cv.Status.Conditions, openshiftconfigv1.OperatorProgressing)

	updating, startedAt, completedAt := isControlPlaneUpdating(cvProgressing, lastHistoryItem)
	updating.LastTransitionTime = now

	klog.V(2).Infof("CPI :: CV/%s :: Updating=%s Started=%s Completed=%s", cv.Name, updating.Status, startedAt, completedAt)

	var assessment ouev1alpha1.ClusterVersionAssessment
	var completion int32
	switch updating.Status {
	case metav1.ConditionTrue:
		assessment = ouev1alpha1.ClusterVersionAssessmentProgressing
	case metav1.ConditionFalse:
		assessment = ouev1alpha1.ClusterVersionAssessmentCompleted
		completion = 100
	case metav1.ConditionUnknown:
		assessment = ouev1alpha1.ClusterVersionAssessmentUnknown
	default:
		assessment = ouev1alpha1.ClusterVersionAssessmentUnknown
	}

	klog.V(2).Infof("CPI :: CV/%s :: Assessment=%s", cv.Name, assessment)

	insight := &ouev1alpha1.ClusterVersionProgressInsightStatus{
		Name:       cv.Name,
		Assessment: assessment,
		Versions:   versionsFromHistory(cv.Status.History),
		Completion: completion,
		StartedAt:  startedAt,
		Conditions: []metav1.Condition{updating},
	}

	if !completedAt.IsZero() {
		insight.CompletedAt = &completedAt
	}

	if est := estimateCompletion(startedAt.Time); !est.IsZero() {
		insight.EstimatedCompletedAt = &metav1.Time{Time: est}
	}

	// TODO(muller): Port HealthInsights later
	// var healthInsights []*ouev1alpha1.HealthInsightStatus
	// if forcedHealthInsight := forcedHealthInsight(cv, now); forcedHealthInsight != nil {
	// 	healthInsights = append(healthInsights, forcedHealthInsight)
	// }
	//
	// return insight, healthInsights
	return insight
}

// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusterversionprogressinsights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusterversionprogressinsights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openshift.muller.dev,resources=clusterversionprogressinsights/finalizers,verbs=update
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
	logger := logf.FromContext(ctx)

	var clusterVersion openshiftconfigv1.ClusterVersion
	cvErr := r.Get(ctx, req.NamespacedName, &clusterVersion)
	if cvErr != nil && !apierrors.IsNotFound(cvErr) {
		logger.WithValues("ClusterVersion", req.NamespacedName).Error(cvErr, "Failed to get ClusterVersion")
		return ctrl.Result{}, cvErr
	}

	var progressInsight ouev1alpha1.ClusterVersionProgressInsight
	err := r.Get(ctx, req.NamespacedName, &progressInsight)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Error(err, "Failed to get ClusterVersionProgressInsight")
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(cvErr) && apierrors.IsNotFound(err) {
		// If both ClusterVersion and ClusterVersionProgressInsight do not exist, we can return early
		logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Info("Both ClusterVersion and ClusterVersionProgressInsight do not exist, nothing to reconcile")
		return ctrl.Result{}, nil
	}

	if apierrors.IsNotFound(cvErr) && !apierrors.IsNotFound(err) {
		// If the ClusterVersion does not exist, we can delete the progress insight
		logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Info("ClusterVersion does not exist, deleting ClusterVersionProgressInsight")
		if err := r.Delete(ctx, &progressInsight); err != nil {
			logger.Error(err, "Failed to delete ClusterVersionProgressInsight")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	now := r.now()
	// TODO(muller): Port HealthInsights later
	// cvInsight, healthInsights := assessClusterVersion(clusterVersion, now)
	progressInsight.Status = *assessClusterVersion(&clusterVersion, now)
	progressInsight.Name = clusterVersion.Name

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, &progressInsight); err != nil {
			logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Error(err, "Failed to create ClusterVersionProgressInsight")
			return ctrl.Result{}, err
		}
		logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Info("Created ClusterVersionProgressInsight")
		return ctrl.Result{}, nil

	}

	if err := r.Client.Status().Update(ctx, &progressInsight); err != nil {
		logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Error(err, "Failed to update ClusterVersionProgressInsight status")
		return ctrl.Result{}, err
	}
	logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Info("Updated ClusterVersionProgressInsight status")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterVersionProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ouev1alpha1.ClusterVersionProgressInsight{}).
		Named("clusterversionprogressinsight").
		Watches(
			&openshiftconfigv1.ClusterVersion{},
			&handler.EnqueueRequestForObject{},
		).
		Complete(r)
}
