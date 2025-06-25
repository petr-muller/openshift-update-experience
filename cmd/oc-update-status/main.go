package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	unknownStatus = "Unknown"
)

var (
	rootCmd = &cobra.Command{
		Use:   "oc-update-status",
		Short: "List OpenShift update progress insights",
		Long:  `An oc plugin to list all progress insight instances from the cluster`,
		RunE:  run,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		return fmt.Errorf("failed to add to scheme: %w", err)
	}
	if err := ouev1alpha1.AddToScheme(s); err != nil {
		return fmt.Errorf("failed to add oue types to scheme: %w", err)
	}

	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: s})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	var cvInsights ouev1alpha1.ClusterVersionProgressInsightList
	if err := c.List(ctx, &cvInsights); err != nil {
		return fmt.Errorf("failed to get ClusterVersion insights: %w", err)
	}

	var coInsights ouev1alpha1.ClusterOperatorProgressInsightList
	if err := c.List(ctx, &coInsights); err != nil {
		return fmt.Errorf("failed to get ClusterOperator insights: %w", err)
	}

	var nodeInsights ouev1alpha1.NodeProgressInsightList
	if err := c.List(ctx, &nodeInsights); err != nil {
		return fmt.Errorf("failed to get Node insights: %w", err)
	}

	return printTable(&cvInsights, &coInsights, &nodeInsights)
}

func printTable(
	cvInsights *ouev1alpha1.ClusterVersionProgressInsightList,
	coInsights *ouev1alpha1.ClusterOperatorProgressInsightList,
	nodeInsights *ouev1alpha1.NodeProgressInsightList,
) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() {
		if err := w.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to flush output: %v\n", err)
		}
	}()

	_, _ = fmt.Fprintln(w, "TYPE\tNAME\tSTATUS")

	for _, item := range cvInsights.Items {
		status := item.Status.Assessment
		_, _ = fmt.Fprintf(w, "ClusterVersionProgressInsight\t%s\t%s\t\n", item.Name, status)
	}

	for _, item := range coInsights.Items {
		status := getClusterOperatorStatus(&item)
		_, _ = fmt.Fprintf(w, "ClusterOperatorProgressInsight\t%s\t%s\t\n", item.Name, status)
	}

	for _, item := range nodeInsights.Items {
		status := getNodeStatus(&item)
		_, _ = fmt.Fprintf(w, "NodeProgressInsight\t%s\t%s\t\n", item.Name, status)
	}

	return nil
}

func getNodeStatus(insight *ouev1alpha1.NodeProgressInsight) string {
	if insight.Status.Conditions == nil {
		return unknownStatus
	}

	// Check for Updating condition first
	for _, condition := range insight.Status.Conditions {
		if condition.Type == "Updating" {
			if condition.Status == metav1.ConditionTrue {
				return fmt.Sprintf("Updating (%s)", condition.Reason)
			}
		}
	}

	// Check for Available/Degraded conditions
	available := false
	degraded := false
	for _, condition := range insight.Status.Conditions {
		if condition.Type == "Available" && condition.Status == metav1.ConditionTrue {
			available = true
		}
		if condition.Type == "Degraded" && condition.Status == metav1.ConditionTrue {
			degraded = true
		}
	}

	if degraded {
		return "Degraded"
	}
	if available {
		return "Available"
	}
	return unknownStatus
}

func getClusterOperatorStatus(insight *ouev1alpha1.ClusterOperatorProgressInsight) string {
	if insight.Status.Conditions == nil {
		return unknownStatus
	}

	for _, condition := range insight.Status.Conditions {
		if condition.Type == "Updating" {
			if condition.Status == metav1.ConditionTrue {
				return fmt.Sprintf("Updating (%s)", condition.Reason)
			}
		}
	}

	for _, condition := range insight.Status.Conditions {
		if condition.Type == "Healthy" {
			if condition.Status == metav1.ConditionTrue {
				return "Healthy"
			} else {
				return fmt.Sprintf("Unhealthy (%s)", condition.Reason)
			}
		}
	}

	return unknownStatus
}
