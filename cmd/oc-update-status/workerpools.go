package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

type workerPoolDisplayData struct {
	Name       string
	Assessment string
	Completion string
	Status     string
}

type workerPoolsDisplayData struct {
	Pools []workerPoolDisplayData
}

func (w *workerPoolsDisplayData) Write(out io.Writer) {
	if len(w.Pools) == 0 {
		return
	}

	_, _ = fmt.Fprintf(out, "\n= Worker Upgrade =\n\n")

	tabw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	_, _ = tabw.Write([]byte("WORKER POOL\tASSESSMENT\tCOMPLETION\tSTATUS\n"))

	for _, pool := range w.Pools {
		_, _ = tabw.Write([]byte(pool.Name + "\t"))
		_, _ = tabw.Write([]byte(pool.Assessment + "\t"))
		_, _ = tabw.Write([]byte(pool.Completion + "\t"))
		_, _ = tabw.Write([]byte(pool.Status + "\n"))
	}

	_ = tabw.Flush()
}

// formatPoolAssessment formats the assessment column for a worker pool
func formatPoolAssessment(status ouev1alpha1.MachineConfigPoolProgressInsightStatus) string {
	return string(status.Assessment)
}

// formatPoolCompletion formats the completion column for a worker pool
func formatPoolCompletion(status ouev1alpha1.MachineConfigPoolProgressInsightStatus) string {
	var total int32
	for _, summary := range status.Summaries {
		if summary.Type == ouev1alpha1.NodesTotal {
			total = summary.Count
			break
		}
	}

	// Calculate updated count from completion percentage
	var updated int32
	if total > 0 {
		updated = (status.Completion * total) / 100
	}

	return fmt.Sprintf("%d%% (%d/%d)", status.Completion, updated, total)
}

// formatPoolStatus formats the status column for a worker pool
func formatPoolStatus(status ouev1alpha1.MachineConfigPoolProgressInsightStatus) string {
	var available, progressing, draining, excluded, degraded int32
	for _, summary := range status.Summaries {
		switch summary.Type {
		case ouev1alpha1.NodesAvailable:
			available = summary.Count
		case ouev1alpha1.NodesProgressing:
			progressing = summary.Count
		case ouev1alpha1.NodesDraining:
			draining = summary.Count
		case ouev1alpha1.NodesExcluded:
			excluded = summary.Count
		case ouev1alpha1.NodesDegraded:
			degraded = summary.Count
		}
	}

	// Check if pool is updating
	isUpdating := false
	for _, condition := range status.Conditions {
		if condition.Type == string(ouev1alpha1.MachineConfigPoolProgressInsightUpdating) {
			isUpdating = condition.Status == metav1.ConditionTrue
			break
		}
	}

	// Always show Available
	result := fmt.Sprintf("%d Available", available)

	// Show Progressing and Draining based on updating status
	if isUpdating {
		// When updating, always show Progressing and Draining (even if 0)
		result += fmt.Sprintf(", %d Progressing, %d Draining", progressing, draining)
	} else {
		// When not updating, show Progressing and Draining only if non-zero
		if progressing > 0 {
			result += fmt.Sprintf(", %d Progressing", progressing)
		}
		if draining > 0 {
			result += fmt.Sprintf(", %d Draining", draining)
		}
	}

	// Show Excluded only when non-zero
	if excluded > 0 {
		result += fmt.Sprintf(", %d Excluded", excluded)
	}

	// Show Degraded only when non-zero
	if degraded > 0 {
		result += fmt.Sprintf(", %d Degraded", degraded)
	}

	return result
}

func assessWorkerPools(mcpInsights ouev1alpha1.MachineConfigPoolProgressInsightList) workerPoolsDisplayData {
	displayData := workerPoolsDisplayData{
		Pools: []workerPoolDisplayData{},
	}

	for _, mcp := range mcpInsights.Items {
		// Skip control plane pools
		if mcp.Status.Scope == ouev1alpha1.ControlPlaneScope {
			continue
		}

		pool := workerPoolDisplayData{
			Name:       mcp.Status.Name,
			Assessment: formatPoolAssessment(mcp.Status),
			Completion: formatPoolCompletion(mcp.Status),
			Status:     formatPoolStatus(mcp.Status),
		}

		displayData.Pools = append(displayData.Pools, pool)
	}

	return displayData
}
