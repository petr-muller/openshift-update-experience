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
	"crypto/md5"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

// ClusterVersionProgressInsightReconciler reconciles a ClusterVersionProgressInsight object
type ClusterVersionProgressInsightReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewClusterVersionProgressInsightReconciler creates a new ClusterVersionProgressInsightReconciler with the given client and scheme.
func NewClusterVersionProgressInsightReconciler(client client.Client, scheme *runtime.Scheme) *ClusterVersionProgressInsightReconciler {
	return &ClusterVersionProgressInsightReconciler{
		Client: client,
		Scheme: scheme,
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
		status = metav1.ConditionUnknown
	}

	message := fmt.Sprintf("ClusterVersion has Progressing=%s(Reason=%s) | Message='%s'", cvProgressing.Status, cvProgressing.Reason, cvProgressing.Message)
	return status, reason, message
}

// isControlPlaneUpdating determines whether the control plane is updating based on the ClusterVersion's Progressing
// condition and the last history item. It returns an updating condition, the time the update started, and the time the
// update is completed. If the updating condition cannot be determined, the condition will have Status=Unknown and the
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
	// It looks like we are updating
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
	// It looks like we are not updating
	if cvProgressing.Status == openshiftconfigv1.ConditionFalse {
		if lastHistoryItem.State != openshiftconfigv1.CompletedUpdate {
			setCannotDetermineUpdating(&updating, "Progressing=False in ClusterVersion but last history item is not completed")
		} else if lastHistoryItem.CompletionTime == nil {
			setCannotDetermineUpdating(&updating, "Progressing=False in ClusterVersion but no completion in last history item")
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

const (
	uscForceHealthInsightAnnotation = "oue.openshift.muller.dev/force-health-insight"
)

func forcedHealthInsight(cv *openshiftconfigv1.ClusterVersion, now metav1.Time) *ouev1alpha1.UpdateHealthInsightStatus {
	if _, ok := cv.Annotations[uscForceHealthInsightAnnotation]; !ok {
		return nil
	}

	return &ouev1alpha1.UpdateHealthInsightStatus{
		StartedAt: now,
		Scope: ouev1alpha1.InsightScope{
			Type:      ouev1alpha1.ControlPlaneScope,
			Resources: []ouev1alpha1.ResourceRef{{Resource: "clusterversions", Group: openshiftconfigv1.GroupName, Name: cv.Name}},
		},
		Impact: ouev1alpha1.InsightImpact{
			Level:       ouev1alpha1.InfoImpactLevel,
			Type:        ouev1alpha1.NoneImpactType,
			Summary:     fmt.Sprintf("Forced health insight for ClusterVersion %s", cv.Name),
			Description: fmt.Sprintf("The resource has a %q annotation which forces USC to generate this health insight for testing purposes.", uscForceHealthInsightAnnotation),
		},
		Remediation: ouev1alpha1.InsightRemediation{
			Reference: "https://issues.redhat.com/browse/OTA-1418",
		},
	}
}

// assessClusterVersion produces a ClusterVersion status insight from the current state of the ClusterVersion resource.
// It does not take previous status insight into account. Many fields of the status insights (such as completion) cannot
// be properly calculated without also watching and processing ClusterOperators, so that functionality will need to be
// added later.
func assessClusterVersion(cv *openshiftconfigv1.ClusterVersion, previous *ouev1alpha1.ClusterVersionProgressInsight) (*ouev1alpha1.ClusterVersionProgressInsightStatus, []*ouev1alpha1.UpdateHealthInsightStatus) {
	var lastHistoryItem *openshiftconfigv1.UpdateHistory
	if len(cv.Status.History) > 0 {
		lastHistoryItem = &cv.Status.History[0]
	}
	cvProgressing := findOperatorStatusCondition(cv.Status.Conditions, openshiftconfigv1.OperatorProgressing)

	updating, startedAt, completedAt := isControlPlaneUpdating(cvProgressing, lastHistoryItem)

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
	}

	if oldUpdating := meta.FindStatusCondition(previous.Status.Conditions, updating.Type); oldUpdating != nil {
		insight.Conditions = append(insight.Conditions, *oldUpdating)
	}
	meta.SetStatusCondition(&insight.Conditions, updating)

	if !completedAt.IsZero() {
		insight.CompletedAt = &completedAt
	}

	if est := estimateCompletion(startedAt.Time); !est.IsZero() {
		insight.EstimatedCompletedAt = &metav1.Time{Time: est}
	}

	var healthInsights []*ouev1alpha1.UpdateHealthInsightStatus
	if forcedHealthInsight := forcedHealthInsight(cv, metav1.Now()); forcedHealthInsight != nil {
		healthInsights = append(healthInsights, forcedHealthInsight)
	}

	return insight, healthInsights
}

func nameForHealthInsight(prefix string, healthInsight *ouev1alpha1.UpdateHealthInsightStatus) string {
	hasher := md5.New()
	hasher.Write([]byte(healthInsight.Impact.Summary))
	for i := range healthInsight.Scope.Resources {
		hasher.Write([]byte(healthInsight.Scope.Resources[i].Group))
		hasher.Write([]byte(healthInsight.Scope.Resources[i].Resource))
		hasher.Write([]byte(healthInsight.Scope.Resources[i].Namespace))
		hasher.Write([]byte(healthInsight.Scope.Resources[i].Name))
	}

	sum := hasher.Sum(nil)
	encoded := base32.StdEncoding.EncodeToString(sum)
	encoded = strings.ToLower(strings.TrimRight(encoded, "="))

	return fmt.Sprintf("%s-%s", prefix, encoded)
}

func (r *ClusterVersionProgressInsightReconciler) reconcileHealthInsights(ctx context.Context, cvProgressInsight *ouev1alpha1.ClusterVersionProgressInsight, healthInsights []*ouev1alpha1.UpdateHealthInsightStatus) error {
	var clusterInsights ouev1alpha1.UpdateHealthInsightList
	if err := r.List(ctx, &clusterInsights, client.MatchingLabels{labelUpdateHealthInsightManager: "clusterversion"}); err != nil {
		klog.ErrorS(err, "Failed to list existing UpdateHealthInsights")
		return err
	}

	clusterInsightNames := sets.NewString()
	clusterInsightsByName := make(map[string]*ouev1alpha1.UpdateHealthInsight, len(clusterInsights.Items))
	for _, insight := range clusterInsights.Items {
		clusterInsightNames.Insert(insight.Name)
		clusterInsightsByName[insight.Name] = &insight
	}

	ourInsightNames := sets.NewString()
	ourInsightsByName := make(map[string]*ouev1alpha1.UpdateHealthInsightStatus, len(healthInsights))
	for _, insight := range healthInsights {
		ourInsightNames.Insert(nameForHealthInsight("cv", insight))
		ourInsightsByName[nameForHealthInsight("cv", insight)] = insight
	}

	toCreate := ourInsightNames.Difference(clusterInsightNames)
	toDelete := clusterInsightNames.Difference(ourInsightNames)
	toUpdate := clusterInsightNames.Intersection(ourInsightNames)

	var createErrs []error
	for _, insight := range toCreate.UnsortedList() {
		healthInsight := &ouev1alpha1.UpdateHealthInsight{
			ObjectMeta: metav1.ObjectMeta{
				Name:   insight,
				Labels: map[string]string{labelUpdateHealthInsightManager: "clusterversion"},
			},
			Status: *ourInsightsByName[insight],
		}
		if err := ctrl.SetControllerReference(cvProgressInsight, healthInsight, r.Scheme); err != nil {
			klog.ErrorS(err, "Failed to set controller reference for UpdateHealthInsight", "name", healthInsight.Name)
			createErrs = append(createErrs, err)
			continue
		}
		if err := r.Create(ctx, healthInsight); err != nil {
			klog.ErrorS(err, "Failed to create UpdateHealthInsight", "name", healthInsight.Name)
			createErrs = append(createErrs, err)
		} else {
			klog.InfoS("Created UpdateHealthInsight", "name", healthInsight.Name)
		}
	}

	var updateErrs []error
	for _, insight := range toUpdate.UnsortedList() {
		healthInsight := clusterInsightsByName[insight]
		ourInsight := ourInsightsByName[insight]
		update := healthInsight.DeepCopy()
		update.Status = *ourInsight
		if err := r.Client.Status().Update(ctx, update); err != nil {
			klog.ErrorS(err, "Failed to update UpdateHealthInsight status", "name", healthInsight.Name)
			updateErrs = append(updateErrs, err)
		} else {
			klog.InfoS("Updated UpdateHealthInsight status", "name", healthInsight.Name)
		}
	}

	var deleteErrs []error
	for _, insight := range toDelete.UnsortedList() {
		healthInsight := clusterInsightsByName[insight]
		if err := r.Delete(ctx, healthInsight); err != nil {
			klog.ErrorS(err, "Failed to delete UpdateHealthInsight", "name", healthInsight.Name)
			deleteErrs = append(deleteErrs, err)
		} else {
			klog.InfoS("Deleted UpdateHealthInsight", "name", healthInsight.Name)
		}
	}

	return errors.NewAggregate(append(append(createErrs, updateErrs...), deleteErrs...))
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
	logger := logf.FromContext(ctx)

	var clusterVersion openshiftconfigv1.ClusterVersion
	cvErr := r.Get(ctx, req.NamespacedName, &clusterVersion)
	if cvErr != nil && !apierrors.IsNotFound(cvErr) {
		logger.WithValues("ClusterVersion", req.NamespacedName).Error(cvErr, "Failed to get ClusterVersion")
		return ctrl.Result{}, cvErr
	}

	progressInsight := ouev1alpha1.ClusterVersionProgressInsight{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
	}
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

	cvInsight, healthInsights := assessClusterVersion(&clusterVersion, &progressInsight)

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, &progressInsight); err != nil {
			logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Error(err, "Failed to create ClusterVersionProgressInsight")
			return ctrl.Result{}, err
		}
		progressInsight.Status = *cvInsight
		if err := r.Status().Update(ctx, &progressInsight); err != nil {
			logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Error(err, "Failed to update ClusterVersionProgressInsight status")
			return ctrl.Result{}, err
		}
		logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Info("Created ClusterVersionProgressInsight")
		return ctrl.Result{}, r.reconcileHealthInsights(ctx, &progressInsight, healthInsights)
	}

	diff := cmp.Diff(&progressInsight.Status, cvInsight)
	if diff == "" {
		logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Info("No changes in ClusterVersionProgressInsight, skipping update")
		return ctrl.Result{}, nil
	}
	logger.Info(diff)
	progressInsight.Status = *cvInsight
	if err := r.Client.Status().Update(ctx, &progressInsight); err != nil {
		logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Error(err, "Failed to update ClusterVersionProgressInsight status")
		return ctrl.Result{}, err
	}
	logger.WithValues("ClusterVersionProgressInsight", req.NamespacedName).Info("Updated ClusterVersionProgressInsight status")
	return ctrl.Result{}, r.reconcileHealthInsights(ctx, &progressInsight, healthInsights)
}

const (
	labelUpdateHealthInsightManager = "oue.openshift.muller.dev/update-health-insight-manager"
	controllerName                  = "clusterversionprogressinsight"
)

func predicateForHealthInsightsManagedBy(controller string) predicate.Funcs {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetLabels()[labelUpdateHealthInsightManager] == controller
	})
}

var healthInsightsManagedByClusterVersionProgressInsight = predicateForHealthInsightsManagedBy(controllerName)

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterVersionProgressInsightReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ouev1alpha1.ClusterVersionProgressInsight{}).
		Owns(&ouev1alpha1.UpdateHealthInsight{}, builder.WithPredicates(healthInsightsManagedByClusterVersionProgressInsight)).
		Named("clusterversionprogressinsight").
		Watches(
			&openshiftconfigv1.ClusterVersion{},
			&handler.EnqueueRequestForObject{},
		).
		Complete(r)
}
