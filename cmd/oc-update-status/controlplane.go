package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type assessmentState string

type versions struct {
	target               string
	previous             string
	isTargetInstall      bool
	isPreviousPartial    bool
	isMultiArchMigration bool
}

func (v versions) String() string {
	if v.isTargetInstall {
		return fmt.Sprintf("%s (installation)", v.target)
	}
	if v.isPreviousPartial {
		return fmt.Sprintf("%s (from incomplete %s)", v.target, v.previous)
	}
	if v.isMultiArchMigration {
		return fmt.Sprintf("%s to Multi-Architecture", v.target)
	}
	return fmt.Sprintf("%s (from %s)", v.target, v.previous)
}

type operator struct {
	Name      string
	Condition metav1.Condition
}

type operators struct {
	Total int
	// Healthy are operators that are known to be healthy (Healthy=True)
	Healthy []operator
	// Unavailable are operators that are not available (Healthy=False | Reason=Unavailable)
	Unavailable []operator
	// Degraded are operators that are available but degraded (Healthy=False | Reason=Degraded)
	Degraded []operator
	// Updated are operators that updated its version, no matter its conditions
	Updated []operator
	// Waiting are operators that have not updated its version and are not progressing
	Waiting []operator
	// Updating are operators that have not updated its version but are progressing
	Updating []operator
}

func (o operators) StatusSummary() string {
	res := []string{fmt.Sprintf("%d healthy", len(o.Healthy))}
	if unavailable := len(o.Unavailable); unavailable > 0 {
		res = append(res, fmt.Sprintf("%d unavailable", unavailable))
	}
	if degraded := len(o.Degraded); degraded > 0 {
		res = append(res, fmt.Sprintf("%d available but degraded", degraded))
	}
	if unknown := o.Total - len(o.Healthy) - len(o.Unavailable) - len(o.Degraded); unknown > 0 {
		res = append(res, fmt.Sprintf("%d unknown", unknown))
	}
	return strings.Join(res, ", ")
}

type controlPlaneStatusDisplayData struct {
	Assessment assessmentState
	Completion float64
	// CompletionAt         time.Time
	Duration             time.Duration
	EstDuration          time.Duration
	EstTimeToComplete    time.Duration
	IsMultiArchMigration bool
	Operators            operators
	TargetVersion        versions
}

//nolint:lll
const controlPlaneStatusTemplateRaw = `= Control Plane =
Assessment:      {{ .Assessment }}
Target Version:  {{ .TargetVersion }}
{{ with commaJoinOperatorNames .Operators.Updating -}}
Updating:        {{ . }}
{{ end -}}
Completion:      {{ printf "%.0f" .Completion }}% ({{ len .Operators.Updated }} operators updated, {{ len .Operators.Updating }} updating, {{ len .Operators.Waiting }} waiting)
Duration:        {{ shortDuration .Duration }}{{ if .EstTimeToComplete }} (Est. Time Remaining: {{ vagueUnder .EstTimeToComplete .EstDuration .IsMultiArchMigration }}){{ end }}
Operator Health: {{ .Operators.StatusSummary }}
`

func shortDuration(d time.Duration) string {
	orig := d.String()
	switch {
	case orig == "0h0m0s" || orig == "0s":
		return "now"
	case strings.HasSuffix(orig, "h0m0s"):
		return orig[:len(orig)-4]
	case strings.HasSuffix(orig, "m0s"):
		return orig[:len(orig)-2]
	case strings.HasSuffix(orig, "h0m"):
		return orig[:len(orig)-2]
	case strings.Contains(orig, "."):
		dStr := orig[:strings.Index(orig, ".")] + "s"
		newD, err := time.ParseDuration(dStr)
		if err != nil {
			return orig
		}
		return shortDuration(newD)
	}
	return orig
}

func vagueUnder(actual, estimated time.Duration, isMultiArchMigration bool) string {
	if isMultiArchMigration {
		return "N/A; multi-architecture migration"
	}
	threshold := 10 * time.Minute
	switch {
	case actual < -10*time.Minute:
		return fmt.Sprintf("N/A; estimate duration was %s", shortDuration(estimated))
	case actual < threshold:
		return fmt.Sprintf("<%s", shortDuration(threshold))
	default:
		return shortDuration(actual)
	}
}

func commaJoinOperatorNames(elems []operator) string {
	names := make([]string, 0, len(elems))
	for _, e := range elems {
		names = append(names, e.Name)
	}
	return strings.Join(names, ", ")
}

var controlPlaneStatusTemplate = template.Must(
	template.New("controlPlaneStatus").
		Funcs(template.FuncMap{
			"shortDuration":          shortDuration,
			"vagueUnder":             vagueUnder,
			"commaJoinOperatorNames": commaJoinOperatorNames,
		}).
		Parse(controlPlaneStatusTemplateRaw))

func (d *controlPlaneStatusDisplayData) Write(f io.Writer, detailed bool, now time.Time) error {
	//nolint:lll
	// TODO(muller): Uncomment as I add more functionality.
	// if d.Operators.Updated == d.Operators.Total {
	// 	_, err := f.Write([]byte(fmt.Sprintf("= Control Plane =\nUpdate to %s successfully completed at %s (duration: %s)\n", d.TargetVersion.target, d.CompletionAt.UTC().Format(time.RFC3339), shortDuration(d.Duration))))
	// 	return err
	// }
	if err := controlPlaneStatusTemplate.Execute(f, d); err != nil {
		return err
	}

	if detailed && len(d.Operators.Updating) > 0 {
		table := tabwriter.NewWriter(f, 0, 0, 3, ' ', 0)
		_, _ = f.Write([]byte("\nUpdating Cluster Operators"))
		_, _ = table.Write([]byte("\nNAME\tSINCE\tREASON\tMESSAGE\n"))
		for _, o := range d.Operators.Updating {
			reason := o.Condition.Reason
			if reason == "" {
				reason = "-"
			}
			// Split message by newlines and indent continuation lines
			lines := strings.Split(o.Condition.Message, "\n")
			for i, line := range lines {
				if i == 0 {
					// First line: write all columns
					_, _ = table.Write([]byte(o.Name + "\t"))
					_, _ = table.Write([]byte(shortDuration(now.Sub(o.Condition.LastTransitionTime.Time)) + "\t"))
					_, _ = table.Write([]byte(reason + "\t"))
				} else {
					// Continuation lines: empty columns for proper indentation
					_, _ = table.Write([]byte("\t\t\t"))
				}
				_, _ = table.Write([]byte(line + "\n"))
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	return nil
}

const (
// TODO(muller): Add a conversion from API assessment?
// assessmentStateProgressing assessmentState = "Progressing"
// assessmentStateProgressingSlow assessmentState = "Progressing - Slow"
// assessmentStateCompleted assessmentState = "Completed"
// assessmentStatePending         assessmentState = "Pending"
// assessmentStateExcluded        assessmentState = "Excluded"
// assessmentStateDegraded        assessmentState = "Degraded"
// assessmentStateStalled         assessmentState = "Stalled"

// clusterStatusFailing is set on the ClusterVersion status when a cluster
// cannot reach the desired state.
// clusterStatusFailing = v1.ClusterStatusConditionType("Failing")

// clusterVersionKind  string = "ClusterVersion"
// clusterOperatorKind string = "ClusterOperator"
)

// func multiArchMigration(history []v1.UpdateHistory) bool {
// 	return len(history) > 1 &&
// 		history[0].Version == history[1].Version &&
// 		history[0].Image != history[1].Image
// }

//nolint:lll
// func versionsFromHistory(history []v1.UpdateHistory) versions {
// 	versionData := versions{
// 		target:   "unknown",
// 		previous: "unknown",
// 	}
// 	if len(history) > 0 {
// 		versionData.target = history[0].Version
// 	}
// 	if len(history) == 1 {
// 		versionData.isTargetInstall = true
// 		return versionData
// 	}
// 	if len(history) > 1 {
// 		versionData.previous = history[1].Version
// 		versionData.isPreviousPartial = history[1].State == v1.PartialUpdate
// 		versionData.isMultiArchMigration = multiArchMigration(history)
// 	}
// var insights []updateInsight
// if !controlPlaneCompleted && versionData.isPreviousPartial {
// 	lastComplete := "unknown"
// 	if len(history) > 2 {
// 		for _, item := range history[2:] {
// 			if item.State == v1.CompletedUpdate {
// 				lastComplete = item.Version
// 				break
// 			}
// 		}
// 	}
// 	insights = []updateInsight{
// 		{
// 			startedAt: history[0].StartedTime.Time,
// 			scope: updateInsightScope{
// 				scopeType: scopeTypeControlPlane,
// 				resources: []scopeResource{cvScope},
// 			},
// 			impact: updateInsightImpact{
// 				level:       warningImpactLevel,
// 				impactType:  noneImpactType,
// 				summary:     fmt.Sprintf("Previous update to %s never completed, last complete update was %s", versionData.previous, lastComplete),
// 				description: fmt.Sprintf("Current update to %s was initiated while the previous update to version %s was still in progress", versionData.target, versionData.previous),
// 			},
// 			remediation: updateInsightRemediation{
// 				reference: "https://docs.openshift.com/container-platform/latest/updating/troubleshooting_updates/gathering-data-cluster-update.html#gathering-clusterversion-history-cli_troubleshooting_updates",
// 			},
// 		},
// 	}
// }
// return versionData
// }

func versionsFromClusterVersionProgressInsight(insightVersions v1alpha1.ControlPlaneUpdateVersions) versions {
	v := versions{
		target:   insightVersions.Target.Version,
		previous: "unknown",
	}

	var targetArch, previousArch string
	for _, targetMeta := range insightVersions.Target.Metadata {
		if targetMeta.Key == v1alpha1.InstallationMetadata {
			v.isTargetInstall = true
			v.previous = ""
			return v
		}
		if targetMeta.Key == v1alpha1.ArchitectureMetadata {
			targetArch = targetMeta.Value
		}
	}

	if insightVersions.Previous != nil {
		v.previous = insightVersions.Previous.Version
		for _, previousMeta := range insightVersions.Previous.Metadata {
			if previousMeta.Key == v1alpha1.PartialMetadata {
				v.isPreviousPartial = true
			}
			if previousMeta.Key == v1alpha1.ArchitectureMetadata {
				previousArch = previousMeta.Value
			}
		}
	}

	if targetArch == "multi" && previousArch != "multi" {
		v.isMultiArchMigration = true
	}

	return v
}

func assessControlPlaneStatus(
	cv *v1alpha1.ClusterVersionProgressInsightStatus,
	cos []v1alpha1.ClusterOperatorProgressInsightStatus,

	now time.Time,
) controlPlaneStatusDisplayData {
	var displayData controlPlaneStatusDisplayData
	displayData.Assessment = assessmentState(cv.Assessment)
	displayData.Completion = float64(cv.Completion)

	var updatingFor time.Duration
	if cv.CompletedAt == nil {
		updatingFor = now.Sub(cv.StartedAt.Time)
	} else {
		updatingFor = cv.CompletedAt.Sub(cv.StartedAt.Time)
	}

	// precision to seconds when under 10m
	if updatingFor > 10*time.Minute {
		updatingFor = updatingFor.Round(time.Minute)
	} else {
		updatingFor = updatingFor.Round(time.Second)
	}

	displayData.Duration = updatingFor

	if cv.EstimatedCompletedAt != nil {
		displayData.EstDuration = cv.EstimatedCompletedAt.Sub(cv.StartedAt.Time)
		displayData.EstTimeToComplete = cv.EstimatedCompletedAt.Sub(now)
	}

	for _, co := range cos {
		displayData.Operators.Total++

		updating := string(v1alpha1.ClusterOperatorProgressInsightUpdating)
		if updating := meta.FindStatusCondition(co.Conditions, updating); updating != nil {
			op := operator{Name: co.Name, Condition: *updating}
			switch {
			case updating.Status == metav1.ConditionTrue:
				displayData.Operators.Updating = append(displayData.Operators.Updating, op)
			case updating.Status == metav1.ConditionFalse &&
				updating.Reason == string(v1alpha1.ClusterOperatorUpdatingReasonUpdated):
				displayData.Operators.Updated = append(displayData.Operators.Updated, op)
			case updating.Status == metav1.ConditionFalse &&
				updating.Reason == string(v1alpha1.ClusterOperatorUpdatingReasonPending):
				displayData.Operators.Waiting = append(displayData.Operators.Waiting, op)
				// TODO(muller): Handle error cases (unknown, false with weird reasons)
			}
		}

		healthy := string(v1alpha1.ClusterOperatorProgressInsightHealthy)
		if healthy := meta.FindStatusCondition(co.Conditions, healthy); healthy != nil {
			reasonUnavailable := string(v1alpha1.ClusterOperatorHealthyReasonUnavailable)
			reasonDegraded := string(v1alpha1.ClusterOperatorHealthyReasonDegraded)
			op := operator{Name: co.Name, Condition: *healthy}
			switch {
			case healthy.Status == metav1.ConditionTrue:
				displayData.Operators.Healthy = append(displayData.Operators.Healthy, op)
			case healthy.Status == metav1.ConditionFalse && healthy.Reason == reasonUnavailable:
				displayData.Operators.Unavailable = append(displayData.Operators.Unavailable, op)
			case healthy.Status == metav1.ConditionFalse && healthy.Reason == reasonDegraded:
				displayData.Operators.Degraded = append(displayData.Operators.Degraded, op)
			}
		}
	}

	// var insights []updateInsight

	// cvGroupKind := scopeGroupKind{group: v1.GroupName, kind: clusterVersionKind}
	// cvScope := scopeResource{kind: cvGroupKind, name: cv.Name}

	//nolint:lll
	// if c := findClusterOperatorStatusCondition(cv.Status.Conditions, clusterStatusFailing); c == nil {
	// 	insight := updateInsight{
	// 		startedAt: at,
	// 		scope:     updateInsightScope{scopeType: scopeTypeControlPlane, resources: []scopeResource{cvScope}},
	// 		impact: updateInsightImpact{
	// 			level:       warningImpactLevel,
	// 			impactType:  updateStalledImpactType,
	// 			summary:     fmt.Sprintf("Cluster Version %s has no %s condition", cv.Name, clusterStatusFailing),
	// 			description: "Current status of Cluster Version reconciliation is unclear.  See 'oc -n openshift-cluster-version logs -l k8s-app=cluster-version-operator --tail -1' to debug.",
	// 		},
	// 		remediation: updateInsightRemediation{reference: "https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md"},
	// 	}
	// 	insights = append(insights, insight)
	// } else if c.Status != v1.ConditionFalse {
	// 	verb := "is"
	// 	if c.Status == v1.ConditionUnknown {
	// 		verb = "may be"
	// 	}
	// 	insight := updateInsight{
	// 		startedAt: c.LastTransitionTime.Time,
	// 		scope:     updateInsightScope{scopeType: scopeTypeControlPlane, resources: []scopeResource{{kind: cvGroupKind, name: cv.Name}}},
	// 		impact: updateInsightImpact{
	// 			level:       warningImpactLevel,
	// 			impactType:  updateStalledImpactType,
	// 			summary:     fmt.Sprintf("Cluster Version %s %s failing to proceed with the update (%s)", cv.Name, verb, c.Reason),
	// 			description: c.Message,
	// 		},
	// 		remediation: updateInsightRemediation{reference: "https://github.com/openshift/runbooks/blob/master/alerts/cluster-monitoring-operator/ClusterOperatorDegraded.md"},
	// 	}
	// 	insights = append(insights, insight)
	// }

	// var lastObservedProgress time.Time
	// var mcoStartedUpdating time.Time

	// controlPlaneCompleted := displayData.Operators.Updated == displayData.Operators.Total
	// if controlPlaneCompleted {
	// 	displayData.Assessment = assessmentStateCompleted
	// } else {
	// 	displayData.Assessment = assessmentStateProgressing
	// }

	// If MCO is the last updating operator, treat its progressing start as last observed progress
	// to avoid daemonset operators polluting last observed progress by flipping Progressing when
	// nodes reboot
	// if !mcoStartedUpdating.IsZero() && (displayData.Operators.Total-displayData.Operators.Updated == 1) {
	// 	lastObservedProgress = mcoStartedUpdating
	// }

	// toLastObservedProgress is started until last observed progress
	// var toLastObservedProgress time.Duration

	// if len(cv.Status.History) > 0 {
	// 	currentHistoryItem := cv.Status.History[0]
	// 	started := currentHistoryItem.StartedTime.Time
	// 	if !lastObservedProgress.After(started) {
	// 		lastObservedProgress = at
	// 	}
	// 	toLastObservedProgress = lastObservedProgress.Sub(started)
	// 	if currentHistoryItem.State == v1.CompletedUpdate {
	// 		displayData.CompletionAt = currentHistoryItem.CompletionTime.Time
	// 	} else {
	// 		displayData.CompletionAt = at
	// 	}
	// }

	versionData := versionsFromClusterVersionProgressInsight(cv.Versions)
	displayData.TargetVersion = versionData
	// insights = append(insights, versionInsights...)

	//nolint:lll
	// coCompletion := float64(displayData.Operators.Updated) / float64(displayData.Operators.Total)
	// displayData.Completion = coCompletion * 100.0
	// if coCompletion <= 1 && displayData.Assessment != assessmentStateCompleted {
	// 	historyBaseline := baselineDuration(cv.Status.History)
	// 	displayData.EstTimeToComplete = estimateCompletion(historyBaseline, toLastObservedProgress, updatingFor, coCompletion)
	// 	displayData.EstDuration = (updatingFor + displayData.EstTimeToComplete).Truncate(time.Minute)
	//
	// 	if displayData.EstTimeToComplete < -10*time.Minute {
	// 		displayData.Assessment = assessmentStateStalled
	// 	} else if displayData.EstTimeToComplete < 0 {
	// 		displayData.Assessment = assessmentStateProgressingSlow
	// 	}
	// }

	return displayData
}
