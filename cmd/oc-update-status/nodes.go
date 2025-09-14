package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/petr-muller/openshift-update-experience/internal/mco"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type nodePhase uint32

const (
	phaseStateDraining nodePhase = iota
	phaseStateUpdating
	phaseStateRebooting
	phaseStatePaused
	phaseStatePending
	phaseStateUpdated
)

func (phase nodePhase) String() string {
	switch phase {
	case phaseStateDraining:
		return "Draining"
	case phaseStateUpdating:
		return "Updating"
	case phaseStateRebooting:
		return "Rebooting"
	case phaseStatePaused:
		return "Paused"
	case phaseStatePending:
		return "Pending"
	case phaseStateUpdated:
		return "Updated"
	default:
		return ""
	}
}

type nodeAssessment uint32

const (
	nodeAssessmentDegraded nodeAssessment = iota
	nodeAssessmentUnavailable
	nodeAssessmentProgressing
	nodeAssessmentExcluded
	nodeAssessmentOutdated
	nodeAssessmentCompleted
)

func (assessment nodeAssessment) String() string {
	switch assessment {
	case nodeAssessmentDegraded:
		return "Degraded"
	case nodeAssessmentUnavailable:
		return "Unavailable"
	case nodeAssessmentProgressing:
		return "Progressing"
	case nodeAssessmentExcluded:
		return "Excluded"
	case nodeAssessmentOutdated:
		return "Outdated"
	case nodeAssessmentCompleted:
		return "Completed"
	default:
		return ""
	}
}

type nodeDisplayData struct {
	Name          string
	Assessment    nodeAssessment
	Phase         nodePhase
	Version       string
	Estimate      string
	Message       string
	isUnavailable bool
	isDegraded    bool
	isUpdating    bool
	isUpdated     bool
}

// type nodesOverviewDisplayData struct {
// 	Total       int
// 	Available   int
// 	Progressing int
// 	Outdated    int
// 	Draining    int
// 	Excluded    int
// 	Degraded    int
// }

type poolDisplayData struct {
	Name       string
	Completion int // 0-100 percentage
	Nodes      []nodeDisplayData
	// TODO(muller): Enable as I am adding functionality to the plugin.
	// Assessment    assessmentState
	// Duration      time.Duration
	// NodesOverview nodesOverviewDisplayData

}

func (pool *poolDisplayData) WriteNodes(w io.Writer, detailed bool, maxlines int) {
	if len(pool.Nodes) == 0 {
		return
	}
	if pool.Name == mco.MachineConfigPoolMaster {
		if pool.Completion == 100 {
			_, _ = fmt.Fprintf(w, "\nAll control plane nodes successfully updated to %s\n", pool.Nodes[0].Version)
			return
		}
		_, _ = fmt.Fprintf(w, "\nControl Plane Nodes")
	} else {
		_, _ = fmt.Fprintf(w, "\nWorker Pool Nodes: %s", pool.Name)
	}

	tabw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	_, _ = tabw.Write([]byte("\nNAME\tASSESSMENT\tPHASE\tVERSION\tEST\tMESSAGE\n"))
	var total, completed, available, progressing, outdated, draining, excluded int
	for i, node := range pool.Nodes {
		if !detailed && i >= maxlines {
			// Limit displaying too many nodes when not in detailed mode
			// Display nodes in undesired states regardless their count
			if !node.isDegraded && (!node.isUnavailable || node.isUpdating) {
				total++
				if node.isUpdated {
					completed++
				} else {
					outdated++
				}
				if !node.isUnavailable {
					available++
				}
				if node.Phase == phaseStateDraining {
					draining++
				}
				if node.Phase == phaseStatePaused {
					excluded++
				}
				if node.Assessment == nodeAssessmentProgressing {
					progressing++
				}
				continue
			}
		}

		version := node.Version
		if version == "" {
			version = "?"
		}

		estimate := node.Estimate
		if estimate == "" {
			if node.isUpdated || node.Assessment == nodeAssessmentExcluded {
				estimate = "-"
			} else {
				estimate = "?"
			}
		}

		_, _ = tabw.Write([]byte(node.Name + "\t"))
		_, _ = tabw.Write([]byte(node.Assessment.String() + "\t"))
		_, _ = tabw.Write([]byte(node.Phase.String() + "\t"))
		_, _ = tabw.Write([]byte(version + "\t"))
		_, _ = tabw.Write([]byte(estimate + "\t"))
		_, _ = tabw.Write([]byte(node.Message + "\n"))
	}
	_ = tabw.Flush()
	if total > 0 {
		_, _ = fmt.Fprintf(w, "...\nOmitted additional %d Total, %d Completed, %d Available, %d Progressing, %d Outdated, %d Draining, %d Excluded, and 0 Degraded nodes.\nPass along --details=nodes to see all information.\n", total, completed, available, progressing, outdated, draining, excluded) //nolint:lll
	}
}

func assessPool(name string, insights []ouev1alpha1.NodeProgressInsight) poolDisplayData {
	pool := poolDisplayData{
		Name:       name,
		Completion: 0,
		Nodes:      nil,
	}
	var updated float64
	for _, insight := range insights {
		node := assessNode(insight)
		if node.isUpdated {
			updated++
		}
		pool.Nodes = append(pool.Nodes, node)
	}

	if len(insights) > 0 {
		total := float64(len(insights))
		// TODO(muller): This should be taken from the MCO status, not computed here.
		pool.Completion = int(updated / total * 100)
	}

	return pool
}

func assessNode(insight ouev1alpha1.NodeProgressInsight) nodeDisplayData {
	node := nodeDisplayData{
		Name:          insight.Status.Name,
		Assessment:    0,
		Phase:         0,
		Version:       insight.Status.Version,
		Estimate:      "",
		Message:       insight.Status.Message,
		isUnavailable: false,
		isDegraded:    false,
		isUpdating:    false,
		isUpdated:     false,
	}
	if insight.Status.EstimatedToComplete != nil {
		node.Estimate = shortDuration(insight.Status.EstimatedToComplete.Duration)
	}

	updatingType := string(ouev1alpha1.NodeStatusInsightUpdating)
	if updating := meta.FindStatusCondition(insight.Status.Conditions, updatingType); updating != nil {
		switch {
		case updating.Status == metav1.ConditionTrue:
			node.isUpdating = true
			node.Assessment = nodeAssessmentProgressing
			switch updating.Reason {
			case string(ouev1alpha1.NodeDraining):
				node.Phase = phaseStateDraining
			case string(ouev1alpha1.NodeUpdating):
				node.Phase = phaseStateUpdating
			case string(ouev1alpha1.NodeRebooting):
				node.Phase = phaseStateRebooting
			}
		case updating.Status == metav1.ConditionUnknown:
			// TODO: Unknown, should be handled somehow.
			// Implies Status == metav1.ConditionFalse in the remaining cases.
		case updating.Reason == string(ouev1alpha1.NodeCompleted):
			node.isUpdated = true
			node.Assessment = nodeAssessmentCompleted
			node.Phase = phaseStateUpdated
		case updating.Reason == string(ouev1alpha1.NodePaused):
			node.Assessment = nodeAssessmentExcluded
			node.Phase = phaseStatePaused
		case updating.Reason == string(ouev1alpha1.NodeUpdatePending):
			node.Assessment = nodeAssessmentOutdated
			node.Phase = phaseStatePending
		}
	}

	degradedType := string(ouev1alpha1.NodeStatusInsightDegraded)
	if degraded := meta.FindStatusCondition(insight.Status.Conditions, degradedType); degraded != nil {
		if node.isDegraded = degraded.Status == metav1.ConditionTrue; node.isDegraded {
			node.Assessment = nodeAssessmentDegraded
		}
	}

	availableType := string(ouev1alpha1.NodeStatusInsightAvailable)
	if available := meta.FindStatusCondition(insight.Status.Conditions, availableType); available != nil {
		if node.isUnavailable = available.Status != metav1.ConditionTrue; node.isUnavailable {
			node.Assessment = nodeAssessmentUnavailable
		}
	}

	return node
}
