package main

import (
	"fmt"
	"os"
	"path"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type mockData struct {
	path string

	cvInsights ouev1alpha1.ClusterVersionProgressInsightList
	// TODO(muller): Enable as I am adding functionality to the plugin.
	// coInsights     *ouev1alpha1.ClusterOperatorProgressInsightList
	// nodeInsights   *ouev1alpha1.NodeProgressInsightList
	// healthInsights *ouev1alpha1.UpdateHealthInsightList
}

func asResourceList[T any](objects *corev1.List, decoder runtime.Decoder) ([]T, error) {
	outputItems := make([]T, 0, len(objects.Items))
	for i, item := range objects.Items {
		obj, err := runtime.Decode(decoder, item.Raw)
		if err != nil {
			return nil, err
		}
		typedObj, ok := any(obj).(*T)
		if !ok {
			return nil, fmt.Errorf("unexpected object type %T in List content at index %d", obj, i)
		}
		outputItems = append(outputItems, *typedObj)
	}
	return outputItems, nil
}

func (o *mockData) load() error {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	if err := corev1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := ouev1alpha1.AddToScheme(scheme); err != nil {
		return err
	}

	decoder := codecs.UniversalDecoder(corev1.SchemeGroupVersion, ouev1alpha1.GroupVersion)

	if err := o.loadClusterVersionInsights(decoder); err != nil {
		return fmt.Errorf("failed to load ClusterVersion insights: %w", err)
	}

	// TODO(muller): Enable as I am adding functionality to the plugin.

	// if err := o.loadClusterOperatorInsights(decoder); err != nil {
	// 	return fmt.Errorf("failed to load ClusterOperator insights: %w", err)
	// }

	// if err := o.loadNodeInsights(decoder); err != nil {
	// 	return fmt.Errorf("failed to load Node insights: %w", err)
	// }

	// if err := o.loadHealthInsights(decoder); err != nil {
	// 	return fmt.Errorf("failed to load Health insights: %w", err)
	// }

	return nil
}

func (o *mockData) loadClusterVersionInsights(decoder runtime.Decoder) error {
	insightsPath := path.Join(o.path, "cv-insights.yaml")
	insightsRaw, err := os.ReadFile(insightsPath)
	if err != nil {
		return fmt.Errorf("failed to read ClusterVersion insights file %s: %w", insightsPath, err)
	}
	insightsObj, err := runtime.Decode(decoder, insightsRaw)
	if err != nil {
		return fmt.Errorf("failed to decode ClusterVersion insights file %s: %w", insightsPath, err)
	}
	switch insightsObj := insightsObj.(type) {
	case *ouev1alpha1.ClusterVersionProgressInsightList:
		o.cvInsights = *insightsObj
	case *corev1.List:
		list, err := asResourceList[ouev1alpha1.ClusterVersionProgressInsight](insightsObj, decoder)
		if err != nil {
			return fmt.Errorf("error while parsing file %s: %w", insightsPath, err)
		}
		o.cvInsights = ouev1alpha1.ClusterVersionProgressInsightList{
			Items: list,
		}
	default:
		return fmt.Errorf("unexpected object type %T in ClusterVersion insights file %s", insightsObj, insightsPath)
	}
	return nil
}

// TODO(muller): Enable as I am adding functionality to the plugin.

// func (o *mockData) loadClusterOperatorInsights(decoder runtime.Decoder) error {
// 	insightsPath := path.Join(o.path, "co-insights.yaml")
// 	insightsRaw, err := os.ReadFile(insightsPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read ClusterOperator insights file %s: %w", insightsPath, err)
// 	}
// 	insightsObj, err := runtime.Decode(decoder, insightsRaw)
// 	if err != nil {
// 		return fmt.Errorf("failed to decode ClusterOperator insights file %s: %w", insightsPath, err)
// 	}
// 	switch insightsObj := insightsObj.(type) {
// 	case *ouev1alpha1.ClusterOperatorProgressInsightList:
// 		o.coInsights = insightsObj
// 	case *corev1.List:
// 		list, err := asResourceList[ouev1alpha1.ClusterOperatorProgressInsight](insightsObj, decoder)
// 		if err != nil {
// 			return fmt.Errorf("error while parsing file %s: %w", insightsPath, err)
// 		}
// 		o.coInsights = &ouev1alpha1.ClusterOperatorProgressInsightList{
// 			Items: list,
// 		}
// 	default:
// 		return fmt.Errorf("unexpected object type %T in ClusterOperator insights file %s", insightsObj, insightsPath)
// 	}
// 	return nil
// }

// func (o *mockData) loadNodeInsights(decoder runtime.Decoder) error {
// 	insightsPath := path.Join(o.path, "node-insights.yaml")
// 	insightsRaw, err := os.ReadFile(insightsPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read Node insights file %s: %w", insightsPath, err)
// 	}
// 	insightsObj, err := runtime.Decode(decoder, insightsRaw)
// 	if err != nil {
// 		return fmt.Errorf("failed to decode Node insights file %s: %w", insightsPath, err)
// 	}
// 	switch insightsObj := insightsObj.(type) {
// 	case *ouev1alpha1.NodeProgressInsightList:
// 		o.nodeInsights = insightsObj
// 	case *corev1.List:
// 		list, err := asResourceList[ouev1alpha1.NodeProgressInsight](insightsObj, decoder)
// 		if err != nil {
// 			return fmt.Errorf("error while parsing file %s: %w", insightsPath, err)
// 		}
// 		o.nodeInsights = &ouev1alpha1.NodeProgressInsightList{
// 			Items: list,
// 		}
// 	default:
// 		return fmt.Errorf("unexpected object type %T in Node insights file %s", insightsObj, insightsPath)
// 	}
// 	return nil
// }

// func (o *mockData) loadHealthInsights(decoder runtime.Decoder) error {
// 	insightsPath := path.Join(o.path, "health-insights.yaml")
// 	insightsRaw, err := os.ReadFile(insightsPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read Health insights file %s: %w", insightsPath, err)
// 	}
// 	insightsObj, err := runtime.Decode(decoder, insightsRaw)
// 	if err != nil {
// 		return fmt.Errorf("failed to decode Health insights file %s: %w", insightsPath, err)
// 	}
// 	switch insightsObj := insightsObj.(type) {
// 	case *ouev1alpha1.UpdateHealthInsightList:
// 		o.healthInsights = insightsObj
// 	case *corev1.List:
// 		list, err := asResourceList[ouev1alpha1.UpdateHealthInsight](insightsObj, decoder)
// 		if err != nil {
// 			return fmt.Errorf("error while parsing file %s: %w", insightsPath, err)
// 		}
// 		o.healthInsights = &ouev1alpha1.UpdateHealthInsightList{
// 			Items: list,
// 		}
// 	default:
// 		return fmt.Errorf("unexpected object type %T in Health insights file %s", insightsObj, insightsPath)
// 	}
// 	return nil
// }
