package main

import (
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

type assessmentState string

type controlPlaneStatusDisplayData struct {
	Assessment assessmentState
	// Completion           float64
	// CompletionAt         time.Time
	// Duration             time.Duration
	// EstDuration          time.Duration
	// EstTimeToComplete    time.Duration
	// Operators            operators
	// TargetVersion        versions
	// IsMultiArchMigration bool
}

const controlPlaneStatusTemplateRaw = `= Control Plane =
Assessment:      {{ .Assessment }}
`

//nolint:lll
// TODO(muller): Complete the template as I add more functionality.
// Target Version:  {{ .TargetVersion }}
// {{ with commaJoin .Operators.Updating -}}
// Updating:        {{ . }}
// {{ end -}}
// Completion:      {{ printf "%.0f" .Completion }}% ({{ .Operators.Updated }} operators updated, {{ len .Operators.Updating }} updating, {{ .Operators.Waiting }} waiting)
// Duration:        {{ shortDuration .Duration }}{{ if .EstTimeToComplete }} (Est. Time Remaining: {{ vagueUnder .EstTimeToComplete .EstDuration .IsMultiArchMigration }}){{ end }}
// Operator Health: {{ .Operators.StatusSummary }}

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
		return "N/A for Multi-Architecture Migration"
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

// TODO(muller): Uncomment as I add more functionality.
// func commaJoin(elems []UpdatingClusterOperator) string {
// 	var names []string
// 	for _, e := range elems {
// 		names = append(names, e.Name)
// 	}
// 	return strings.Join(names, ", ")
// }

var controlPlaneStatusTemplate = template.Must(
	template.New("controlPlaneStatus").
		Funcs(template.FuncMap{
			"shortDuration": shortDuration,
			"vagueUnder":    vagueUnder,
			// TODO(muller): Uncomment as I add more functionality.
			// "commaJoin":     commaJoin,
		}).
		Parse(controlPlaneStatusTemplateRaw))

func (d *controlPlaneStatusDisplayData) Write(f io.Writer, _ bool, _ time.Time) error {
	//nolint:lll
	// TODO(muller): Uncomment as I add more functionality.
	// if d.Operators.Updated == d.Operators.Total {
	// 	_, err := f.Write([]byte(fmt.Sprintf("= Control Plane =\nUpdate to %s successfully completed at %s (duration: %s)\n", d.TargetVersion.target, d.CompletionAt.UTC().Format(time.RFC3339), shortDuration(d.Duration))))
	// 	return err
	// }
	if err := controlPlaneStatusTemplate.Execute(f, d); err != nil {
		return err
	}

	// TODO(muller): Uncomment as I add more functionality.
	// if detailed && len(d.Operators.Updating) > 0 {
	// 	table := tabwriter.NewWriter(f, 0, 0, 3, ' ', 0)
	// 	f.Write([]byte("\nUpdating Cluster Operators"))
	// 	_, _ = table.Write([]byte("\nNAME\tSINCE\tREASON\tMESSAGE\n"))
	// 	for _, o := range d.Operators.Updating {
	// 		reason := o.Condition.Reason
	// 		if reason == "" {
	// 			reason = "-"
	// 		}
	// 		_, _ = table.Write([]byte(o.Name + "\t"))
	// 		_, _ = table.Write([]byte(shortDuration(now.Sub(o.Condition.LastTransitionTime.Time)) + "\t"))
	// 		_, _ = table.Write([]byte(reason + "\t"))
	// 		_, _ = table.Write([]byte(o.Condition.Message + "\n"))
	// 	}
	// 	if err := table.Flush(); err != nil {
	// 		return err
	// 	}
	// }
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

func assessControlPlaneStatus(cv *v1alpha1.ClusterVersionProgressInsightStatus) controlPlaneStatusDisplayData {
	var displayData controlPlaneStatusDisplayData
	displayData.Assessment = assessmentState(cv.Assessment)

	// var insights []updateInsight

	// targetVersion := cv.Status.Desired.Version
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

	// for _, operator := range operators {
	// 	var isPlatformOperator bool
	// 	for annotation := range operator.Annotations {
	// 		if strings.HasPrefix(annotation, "exclude.release.openshift.io/") ||
	// 			strings.HasPrefix(annotation, "include.release.openshift.io/") {
	// 			isPlatformOperator = true
	// 			break
	// 		}
	// 	}
	// 	if !isPlatformOperator {
	// 		continue
	// 	}
	//
	// 	var updated bool
	// 	var mcoOperatorImageUpgrading bool
	// 	if operator.Name == "machine-config" {
	// 		for _, version := range operator.Status.Versions {
	// 			if version.Name == "operator-image" {
	// 				if mcoImagePullSpec != version.Version {
	// 					mcoOperatorImageUpgrading = true
	// 					break
	// 				}
	// 			}
	// 		}
	// 	}
	//
	// 	if operator.Name != "machine-config" || !mcoOperatorImageUpgrading {
	// 		for _, version := range operator.Status.Versions {
	// 			if version.Name == "operator" && version.Version == targetVersion {
	// 				updated = true
	// 				break
	// 			}
	// 		}
	// 	}
	//
	// 	if updated {
	// 		displayData.Operators.Updated++
	// 	}
	//
	// 	var available *v1.ClusterOperatorStatusCondition
	// 	var degraded *v1.ClusterOperatorStatusCondition
	// 	var progressing *v1.ClusterOperatorStatusCondition
	//
	// 	displayData.Operators.Total++
	// 	for _, condition := range operator.Status.Conditions {
	// 		condition := condition
	// 		switch {
	// 		case condition.Type == v1.OperatorAvailable:
	// 			available = &condition
	// 		case condition.Type == v1.OperatorDegraded:
	// 			degraded = &condition
	// 		case condition.Type == v1.OperatorProgressing:
	// 			progressing = &condition
	// 		}
	// 	}
	//
	// 	if progressing != nil {
	// 		if progressing.LastTransitionTime.After(lastObservedProgress) {
	// 			lastObservedProgress = progressing.LastTransitionTime.Time
	// 		}
	// 		if !updated && progressing.Status == v1.ConditionTrue {
	// 			displayData.Operators.Updating = append(displayData.Operators.Updating,
	// 				UpdatingClusterOperator{Name: operator.Name, Condition: progressing})
	// 		}
	// 		if !updated && progressing.Status == v1.ConditionFalse {
	// 			displayData.Operators.Waiting++
	// 		}
	// 		if progressing.Status == v1.ConditionTrue && operator.Name == "machine-config" && !updated {
	// 			mcoStartedUpdating = progressing.LastTransitionTime.Time
	// 		}
	// 	}
	//
	// 	if available == nil || available.Status != v1.ConditionTrue {
	// 		displayData.Operators.Unavailable++
	// 	} else if degraded != nil && degraded.Status == v1.ConditionTrue {
	// 		displayData.Operators.Degraded++
	// 	}
	// 	insights = append(insights, coInsights(operator.Name, available, degraded, at)...)
	// }

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

	// updatingFor is started until now
	// var updatingFor time.Duration
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
	// 	updatingFor = displayData.CompletionAt.Sub(started)
	// 	// precision to seconds when under 60s
	// 	if updatingFor > 10*time.Minute {
	// 		displayData.Duration = updatingFor.Round(time.Minute)
	// 	} else {
	// 		displayData.Duration = updatingFor.Round(time.Second)
	// 	}
	// }

	// versionData, versionInsights := versionsFromHistory(cv.Status.History, cvScope, controlPlaneCompleted)
	// displayData.TargetVersion = versionData
	// displayData.IsMultiArchMigration = versionData.isMultiArchMigration
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
