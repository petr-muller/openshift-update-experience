package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	kcmdutil "k8s.io/kubectl/pkg/cmd/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
)

type options struct {
	genericiooptions.IOStreams
	client client.Client

	mockData       mockData
	detailedOutput string
}

func (o *options) Complete(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return kcmdutil.UsageErrorf(cmd, "positional arguments given")
	}

	if !sets.New[string](detailedOutputAllValues...).Has(o.detailedOutput) {
		return fmt.Errorf("invalid value for --details: %s (must be one of %s)", o.detailedOutput, strings.Join(detailedOutputAllValues, ", "))
	}

	if o.mockData.path != "" {
		return o.mockData.load()
	}

	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		return fmt.Errorf("failed to add to scheme: %w", err)
	}
	if err := ouev1alpha1.AddToScheme(s); err != nil {
		return fmt.Errorf("failed to add oue types to scheme: %w", err)
	}

	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: s})
	o.client = c
	return err
}

const (
	detailedOutputNone      = "none"
	detailedOutputAll       = "all"
	detailedOutputNodes     = "nodes"
	detailedOutputHealth    = "health"
	detailedOutputOperators = "operators"
)

var detailedOutputAllValues = []string{detailedOutputNone, detailedOutputAll, detailedOutputNodes, detailedOutputHealth, detailedOutputOperators}

func New() *cobra.Command {
	o := &options{
		IOStreams: genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}
	cmd := &cobra.Command{
		Use:   "oc-update-status",
		Short: "Display the status of the current cluster version update or multi-arch migration",
		Long:  `An oc plugin to display the status of the current cluster version update or multi-arch migration`,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, args))
			kcmdutil.CheckErr(o.Run(cmd.Context()))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&o.mockData.path, "mocks", "", "Path to a directory with insight manifests data to be used instead of a cluster, used for testing purposes")
	flags.StringVar(&o.detailedOutput, "details", "none", fmt.Sprintf("Show detailed output in selected section. One of: %s", strings.Join(detailedOutputAllValues, ", ")))

	return cmd
}

func (o *options) ClusterVersionProgressInsights(ctx context.Context) (ouev1alpha1.ClusterVersionProgressInsightList, error) {
	if o.mockData.path != "" {
		return o.mockData.cvInsights, nil
	}

	var cvInsights ouev1alpha1.ClusterVersionProgressInsightList
	err := o.client.List(ctx, &cvInsights)
	return cvInsights, err
}

// TODO(muller): Enable as I am adding functionality to the plugin.

// func (o *options) ClusterOperatorProgressInsights(ctx context.Context) (ouev1alpha1.ClusterOperatorProgressInsightList, error) {
// 	if o.mockData.path != "" {
// 		return o.mockData.coInsights, nil
// 	}
//
// 	var coInsights ouev1alpha1.ClusterOperatorProgressInsightList
// 	err := o.client.List(ctx, &cvInsights)
// 	return coInsights, err
// }

// func (o *options) NodeProgressInsights(ctx context.Context) (ouev1alpha1.NodeProgressInsightList, error) {
// 	if o.mockData.path != "" {
// 		return o.mockData.nodeInsights, nil
// 	}
//
// 	var nodeInsights ouev1alpha1.NodeProgressInsightList
// 	err := o.client.List(ctx, &nodeInsights)
// 	return nodeInsights, err
// }

// func (o *options) UpdateHealthInsights(ctx context.Context) (ouev1alpha1.UpdateHealthInsight, error) {
// 	if o.mockData.path != "" {
// 		return o.mockData.healthInsights, nil
// 	}
//
// 	var healthInsights ouev1alpha1.UpdateHealthInsightList
// 	err := o.client.List(ctx, &healthInsights)
// 	return healthInsights, err
// }

func (o *options) Run(ctx context.Context) error {
	cvInsights, err := o.ClusterVersionProgressInsights(ctx)
	if err != nil {
		return fmt.Errorf("failed to get ClusterVersion insights: %w", err)
	}

	// TODO(muller): Enable as I am adding functionality to the plugin.

	// coInsights, err := o.ClusterOperatorProgressInsights(ctx)
	// if err != nil {
	// 	return fmt.Errorf("failed to get ClusterOperator insights: %w", err)
	// }

	// nodeInsights, err := o.NodeProgressInsights(ctx)
	// if err != nil {
	// 	return fmt.Errorf("failed to get Node insights: %w", err)
	// }

	// healthInsights, err := o.UpdateHealthInsights(ctx)
	// if err != nil {
	// 	return fmt.Errorf("failed to get UpdateHealth insights: %w", err)
	// }

	return o.printTable(&cvInsights)
}

func (o *options) printTable(
	cvInsights *ouev1alpha1.ClusterVersionProgressInsightList,
) error {
	w := tabwriter.NewWriter(o.Out, 0, 0, 2, ' ', 0)
	defer func() {
		if err := w.Flush(); err != nil {
			_, _ = fmt.Fprintf(o.ErrOut, "Warning: failed to flush output: %v\n", err)
		}
	}()

	_, _ = fmt.Fprintln(w, "TYPE\tNAME\tSTATUS")

	for _, item := range cvInsights.Items {
		status := item.Status.Assessment
		_, _ = fmt.Fprintf(w, "ClusterVersionProgressInsight\t%s\t%s\t\n", item.Name, status)
	}

	// TODO(muller): Enable as I am adding functionality to the plugin.

	// for _, item := range coInsights.Items {
	// 	status := getClusterOperatorStatus(&item)
	// 	_, _ = fmt.Fprintf(w, "ClusterOperatorProgressInsight\t%s\t%s\t\n", item.Name, status)
	// }

	// for _, item := range nodeInsights.Items {
	// 	status := getNodeStatus(&item)
	// 	_, _ = fmt.Fprintf(w, "NodeProgressInsight\t%s\t%s\t\n", item.Name, status)
	// }

	return nil
}

// TODO(muller): Enable as I am adding functionality to the plugin.

// func getNodeStatus(insight *ouev1alpha1.NodeProgressInsight) string {
// 	if insight.Status.Conditions == nil {
// 		return unknownStatus
// 	}
//
// 	// Check for Updating condition first
// 	for _, condition := range insight.Status.Conditions {
// 		if condition.Type == "Updating" {
// 			if condition.Status == metav1.ConditionTrue {
// 				return fmt.Sprintf("Updating (%s)", condition.Reason)
// 			}
// 		}
// 	}
//
// 	// Check for Available/Degraded conditions
// 	available := false
// 	degraded := false
// 	for _, condition := range insight.Status.Conditions {
// 		if condition.Type == "Available" && condition.Status == metav1.ConditionTrue {
// 			available = true
// 		}
// 		if condition.Type == "Degraded" && condition.Status == metav1.ConditionTrue {
// 			degraded = true
// 		}
// 	}
//
// 	if degraded {
// 		return "Degraded"
// 	}
// 	if available {
// 		return "Available"
// 	}
// 	return unknownStatus
// }

// func getClusterOperatorStatus(insight *ouev1alpha1.ClusterOperatorProgressInsight) string {
// 	if insight.Status.Conditions == nil {
// 		return unknownStatus
// 	}
//
// 	for _, condition := range insight.Status.Conditions {
// 		if condition.Type == "Updating" {
// 			if condition.Status == metav1.ConditionTrue {
// 				return fmt.Sprintf("Updating (%s)", condition.Reason)
// 			}
// 		}
// 	}
//
// 	for _, condition := range insight.Status.Conditions {
// 		if condition.Type == "Healthy" {
// 			if condition.Status == metav1.ConditionTrue {
// 				return "Healthy"
// 			} else {
// 				return fmt.Sprintf("Unhealthy (%s)", condition.Reason)
// 			}
// 		}
// 	}
//
// 	return unknownStatus
// }

func main() {
	cmd := New()
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		os.Exit(1)
	}
}
