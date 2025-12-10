package main

import (
	"fmt"
	"os"
	"path"
	"time"

	ouev1alpha1 "github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type mockData struct {
	path string

	cvInsights   ouev1alpha1.ClusterVersionProgressInsightList
	coInsights   ouev1alpha1.ClusterOperatorProgressInsightList
	nodeInsights ouev1alpha1.NodeProgressInsightList
	mcpInsights  ouev1alpha1.MachineConfigPoolProgressInsightList
	// TODO(muller): Enable as I am adding functionality to the plugin.
	// healthInsights *ouev1alpha1.UpdateHealthInsightList

	mockNow time.Time
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

	if err := o.loadClusterOperatorInsights(decoder); err != nil {
		return fmt.Errorf("failed to load ClusterOperator insights: %w", err)
	}

	if err := o.loadNodeInsights(decoder); err != nil {
		return fmt.Errorf("failed to load Node insights: %w", err)
	}

	if err := o.loadMachineConfigPoolInsights(decoder); err != nil {
		return fmt.Errorf("failed to load MachineConfigPool insights: %w", err)
	}

	// TODO(muller): Enable as I am adding functionality to the plugin.
	// if err := o.loadHealthInsights(decoder); err != nil {
	// 	return fmt.Errorf("failed to load Health insights: %w", err)
	// }

	return nil
}

func mockNowFromClusterVersionInsight(insight *ouev1alpha1.ClusterVersionProgressInsight) time.Time {
	var now time.Time
	if insight == nil {
		return now
	}

	for i := range insight.Status.Conditions {
		condition := insight.Status.Conditions[i]
		if now.Before(condition.LastTransitionTime.Time) {
			now = condition.LastTransitionTime.Time
		}
	}

	if now.Before(insight.Status.StartedAt.Time) {
		now = insight.Status.StartedAt.Time
	}

	if insight.Status.CompletedAt != nil && now.Before(insight.Status.CompletedAt.Time) {
		now = insight.Status.CompletedAt.Time
	}

	return now
}

func mockNowFromClusterOperatorInsight(insight *ouev1alpha1.ClusterOperatorProgressInsight) time.Time {
	var now time.Time
	if insight == nil {
		return now
	}

	for i := range insight.Status.Conditions {
		condition := insight.Status.Conditions[i]
		if now.Before(condition.LastTransitionTime.Time) {
			now = condition.LastTransitionTime.Time
		}
	}

	return now
}

func mockNowFromNodeInsight(insight *ouev1alpha1.NodeProgressInsight) time.Time {
	var now time.Time
	if insight == nil {
		return now
	}

	for i := range insight.Status.Conditions {
		condition := insight.Status.Conditions[i]
		if now.Before(condition.LastTransitionTime.Time) {
			now = condition.LastTransitionTime.Time
		}
	}

	return now
}

func mockNowFromMachineConfigPoolInsight(insight *ouev1alpha1.MachineConfigPoolProgressInsight) time.Time {
	var now time.Time
	if insight == nil {
		return now
	}

	for i := range insight.Status.Conditions {
		condition := insight.Status.Conditions[i]
		if now.Before(condition.LastTransitionTime.Time) {
			now = condition.LastTransitionTime.Time
		}
	}

	return now
}

//nolint:dupl // Each loader uses different types but same pattern via loadInsightListFromFile
func (o *mockData) loadClusterVersionInsights(decoder runtime.Decoder) error {
	list, err := loadInsightListFromFile(
		o,
		decoder,
		"cv-insights.yaml",
		"ClusterVersion",
		func(obj runtime.Object) (ouev1alpha1.ClusterVersionProgressInsightList, bool) {
			if typed, ok := obj.(*ouev1alpha1.ClusterVersionProgressInsightList); ok {
				return *typed, true
			}
			return ouev1alpha1.ClusterVersionProgressInsightList{}, false
		},
		func(coreList *corev1.List) (ouev1alpha1.ClusterVersionProgressInsightList, error) {
			items, err := asResourceList[ouev1alpha1.ClusterVersionProgressInsight](coreList, decoder)
			if err != nil {
				return ouev1alpha1.ClusterVersionProgressInsightList{},
					fmt.Errorf("error while parsing file cv-insights.yaml: %w", err)
			}
			return ouev1alpha1.ClusterVersionProgressInsightList{Items: items}, nil
		},
		func(list ouev1alpha1.ClusterVersionProgressInsightList) []ouev1alpha1.ClusterVersionProgressInsight {
			return list.Items
		},
		mockNowFromClusterVersionInsight,
	)
	if err != nil {
		return err
	}
	o.cvInsights = list
	return nil
}

// TODO(muller): Enable as I am adding functionality to the plugin.

//nolint:dupl // Each loader uses different types but same pattern via loadInsightListFromFile
func (o *mockData) loadClusterOperatorInsights(decoder runtime.Decoder) error {
	list, err := loadInsightListFromFile(
		o,
		decoder,
		"co-insights.yaml",
		"ClusterOperator",
		func(obj runtime.Object) (ouev1alpha1.ClusterOperatorProgressInsightList, bool) {
			if typed, ok := obj.(*ouev1alpha1.ClusterOperatorProgressInsightList); ok {
				return *typed, true
			}
			return ouev1alpha1.ClusterOperatorProgressInsightList{}, false
		},
		func(coreList *corev1.List) (ouev1alpha1.ClusterOperatorProgressInsightList, error) {
			items, err := asResourceList[ouev1alpha1.ClusterOperatorProgressInsight](coreList, decoder)
			if err != nil {
				return ouev1alpha1.ClusterOperatorProgressInsightList{},
					fmt.Errorf("error while parsing file co-insights.yaml: %w", err)
			}
			return ouev1alpha1.ClusterOperatorProgressInsightList{Items: items}, nil
		},
		func(list ouev1alpha1.ClusterOperatorProgressInsightList) []ouev1alpha1.ClusterOperatorProgressInsight {
			return list.Items
		},
		mockNowFromClusterOperatorInsight,
	)
	if err != nil {
		return err
	}
	o.coInsights = list
	return nil
}

// loadInsightListFromFile is a generic helper to load insight lists from YAML files and update mockNow
func loadInsightListFromFile[T any, TList any](
	o *mockData,
	decoder runtime.Decoder,
	filename string,
	resourceTypeName string,
	extractFromTyped func(runtime.Object) (TList, bool),
	extractFromCoreList func(*corev1.List) (TList, error),
	getItems func(TList) []T,
	extractMockNow func(*T) time.Time,
) (TList, error) {
	var zero TList
	filePath := path.Join(o.path, filename)

	insightsRaw, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return zero, nil
	}
	if err != nil {
		return zero, fmt.Errorf("failed to read %s insights file %s: %w", resourceTypeName, filePath, err)
	}

	insightsObj, err := runtime.Decode(decoder, insightsRaw)
	if err != nil {
		return zero, fmt.Errorf("failed to decode %s insights file %s: %w", resourceTypeName, filePath, err)
	}

	var list TList
	// Try to extract from typed list
	if result, ok := extractFromTyped(insightsObj); ok {
		list = result
	} else if coreList, ok := insightsObj.(*corev1.List); ok {
		// Try to extract from core v1 List
		list, err = extractFromCoreList(coreList)
		if err != nil {
			return zero, err
		}
	} else {
		return zero, fmt.Errorf("unexpected object type %T in %s insights file %s", insightsObj, resourceTypeName, filePath)
	}

	// Update mockNow from loaded insights
	items := getItems(list)
	for i := range items {
		if now := extractMockNow(&items[i]); o.mockNow.Before(now) {
			o.mockNow = now
		}
	}

	return list, nil
}

//nolint:dupl // Each loader uses different types but same pattern via loadInsightListFromFile
func (o *mockData) loadNodeInsights(decoder runtime.Decoder) error {
	list, err := loadInsightListFromFile(
		o,
		decoder,
		"node-insights.yaml",
		"Node",
		func(obj runtime.Object) (ouev1alpha1.NodeProgressInsightList, bool) {
			if typed, ok := obj.(*ouev1alpha1.NodeProgressInsightList); ok {
				return *typed, true
			}
			return ouev1alpha1.NodeProgressInsightList{}, false
		},
		func(coreList *corev1.List) (ouev1alpha1.NodeProgressInsightList, error) {
			items, err := asResourceList[ouev1alpha1.NodeProgressInsight](coreList, decoder)
			if err != nil {
				return ouev1alpha1.NodeProgressInsightList{},
					fmt.Errorf("error while parsing file node-insights.yaml: %w", err)
			}
			return ouev1alpha1.NodeProgressInsightList{Items: items}, nil
		},
		func(list ouev1alpha1.NodeProgressInsightList) []ouev1alpha1.NodeProgressInsight {
			return list.Items
		},
		mockNowFromNodeInsight,
	)
	if err != nil {
		return err
	}
	o.nodeInsights = list
	return nil
}

//nolint:dupl // Each loader uses different types but same pattern via loadInsightListFromFile
func (o *mockData) loadMachineConfigPoolInsights(decoder runtime.Decoder) error {
	list, err := loadInsightListFromFile(
		o,
		decoder,
		"mcp-insights.yaml",
		"MachineConfigPool",
		func(obj runtime.Object) (ouev1alpha1.MachineConfigPoolProgressInsightList, bool) {
			if typed, ok := obj.(*ouev1alpha1.MachineConfigPoolProgressInsightList); ok {
				return *typed, true
			}
			return ouev1alpha1.MachineConfigPoolProgressInsightList{}, false
		},
		func(coreList *corev1.List) (ouev1alpha1.MachineConfigPoolProgressInsightList, error) {
			items, err := asResourceList[ouev1alpha1.MachineConfigPoolProgressInsight](coreList, decoder)
			if err != nil {
				return ouev1alpha1.MachineConfigPoolProgressInsightList{},
					fmt.Errorf("error while parsing file mcp-insights.yaml: %w", err)
			}
			return ouev1alpha1.MachineConfigPoolProgressInsightList{Items: items}, nil
		},
		func(list ouev1alpha1.MachineConfigPoolProgressInsightList) []ouev1alpha1.MachineConfigPoolProgressInsight {
			return list.Items
		},
		mockNowFromMachineConfigPoolInsight,
	)
	if err != nil {
		return err
	}
	o.mcpInsights = list
	return nil
}

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
