package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShortDuration(t *testing.T) {
	testCases := []struct {
		duration string
		expected string
	}{
		{
			duration: "1s",
			expected: "1s",
		},
		{
			duration: "1m",
			expected: "1m",
		},
		{
			duration: "1h",
			expected: "1h",
		},
		{
			duration: "1h1m1s",
			expected: "1h1m1s",
		},
		{
			duration: "1h10m",
			expected: "1h10m",
		},
		{
			duration: "1h0m10s",
			expected: "1h0m10s",
		},
		{
			duration: "10h10m0s",
			expected: "10h10m",
		},
		{
			duration: "10h10m10s",
			expected: "10h10m10s",
		},
		{
			duration: "0h10m0s",
			expected: "10m",
		},
		{
			duration: "0h0m0s",
			expected: "now",
		},
		{
			duration: "45.000368975s",
			expected: "45s",
		},
		{
			duration: "2m0.000368975s",
			expected: "2m",
		},
		{
			duration: "3h0m",
			expected: "3h",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.duration, func(t *testing.T) {
			d, err := time.ParseDuration(tc.duration)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.expected, shortDuration(d)); diff != "" {
				t.Fatalf("Output differs from expected :\n%s", diff)
			}
		})
	}
}

func TestVagueUnder(t *testing.T) {
	testCases := []struct {
		name      string
		actual    time.Duration
		estimated time.Duration
		multi     bool

		expected string
	}{
		{
			name:      "multiarch migration",
			actual:    3 * time.Hour,
			estimated: 2 * time.Hour,
			multi:     true,
			expected:  "N/A for Multi-Architecture Migration",
		},
		{
			name:      "over 10m over estimate",
			actual:    -12 * time.Minute,
			estimated: 10 * time.Minute,
			expected:  "N/A; estimate duration was 10m",
		},
		{
			name:      "close to estimate",
			actual:    9 * time.Minute,
			estimated: 10 * time.Minute,
			expected:  "<10m",
		},
		{
			name:      "standard case",
			actual:    15 * time.Minute,
			estimated: 60 * time.Minute,
			expected:  "15m",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			if diff := cmp.Diff(tc.expected, vagueUnder(tc.actual, tc.estimated, tc.multi)); diff != "" {
				t.Fatalf("Output differs from expected :\n%s", diff)
			}
		})
	}
}

func TestControlPlaneStatusDisplayDataWrite(t *testing.T) {
	t.Parallel()

	updatingTrue := metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionTrue,
		Reason:  "OperatorGoesBrrr",
		Message: "Operator goes brrr",
	}

	updatingFalsePending := metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionFalse,
		Reason:  "Pending",
		Message: "Operator will go brrr soon",
	}

	updatingFalseUpdated := metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionFalse,
		Reason:  "Updated",
		Message: "Operator went brrr",
	}

	testCases := []struct {
		name     string
		data     controlPlaneStatusDisplayData
		expected string
	}{
		{
			name: "Progressing installation",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				TargetVersion: versions{
					target:          "4.11.0",
					isTargetInstall: true,
				},
				Completion: 3,
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (installation)
Completion:      3%
`,
		},
		{
			name: "Progressing update",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
				Completion: 33,
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (from 4.10.0)
Completion:      33%
`,
		},
		{
			name: "Progressing update with updating operator",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
				Operators: operators{
					Total:    1,
					Updating: []operator{{Name: "test-operator", Condition: updatingTrue}},
				},
				Completion: 66,
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (from 4.10.0)
Updating:        test-operator
Completion:      66%
`,
		},
		{
			name: "Progressing update with pending operator",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Operators: operators{
					Total:   1,
					Waiting: []operator{{Name: "test-operator", Condition: updatingFalsePending}},
				},
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
				Completion: 50,
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (from 4.10.0)
Completion:      50%
`,
		},
		{
			name: "Progressing update with updated operator",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Operators: operators{
					Total:   1,
					Updated: []operator{{Name: "test-operator", Condition: updatingFalseUpdated}},
				},
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
				Completion: 75,
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (from 4.10.0)
Completion:      75%
`,
		},
		{
			name: "Progressing update from partial",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				TargetVersion: versions{
					previous:          "4.10.0",
					target:            "4.11.0",
					isPreviousPartial: true,
				},
				Completion: 50,
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (from incomplete 4.10.0)
Completion:      50%
`,
		},
		{
			name: "Completed",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentCompleted),
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
				Completion: 100,
			},
			expected: `= Control Plane =
Assessment:      Completed
Target Version:  4.11.0 (from 4.10.0)
Completion:      100%
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			_ = tc.data.Write(&buf, false, time.Now())
			if diff := cmp.Diff(tc.expected, buf.String()); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
			}
		})
	}

}

func Test_assessControlPlaneStatus(t *testing.T) {
	updatingTrue := metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionTrue,
		Reason:  "OperatorGoesBrrr",
		Message: "Operator goes brrr",
	}
	updatingFalsePending := metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionFalse,
		Reason:  "Pending",
		Message: "Operator will go brrr soon",
	}
	updatingFalseUpdated := metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionFalse,
		Reason:  "Updated",
		Message: "Operator went brrr",
	}

	testCases := []struct {
		name       string
		cvInsight  *v1alpha1.ClusterVersionProgressInsightStatus
		coInsights []v1alpha1.ClusterOperatorProgressInsightStatus
		expected   controlPlaneStatusDisplayData
	}{
		{
			name: "Progressing update with pending ClusterOperator",
			cvInsight: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
				Completion: 3,
				Versions: v1alpha1.ControlPlaneUpdateVersions{
					Previous: &v1alpha1.Version{
						Version: "4.10.0",
					},
					Target: v1alpha1.Version{
						Version: "4.11.0",
					},
				},
			},
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{
					Name:       "test-operator",
					Conditions: []metav1.Condition{updatingFalsePending},
				},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Completion: 3,
				Operators: operators{
					Total: 1,
					Waiting: []operator{
						{
							Name:      "test-operator",
							Condition: updatingFalsePending,
						},
					},
				},
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
			},
		},
		{
			name: "Progressing update with updated ClusterOperator",
			cvInsight: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
				Completion: 66,
				Versions: v1alpha1.ControlPlaneUpdateVersions{
					Previous: &v1alpha1.Version{
						Version: "4.10.0",
					},
					Target: v1alpha1.Version{
						Version: "4.11.0",
					},
				},
			},
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{
					Name:       "test-operator",
					Conditions: []metav1.Condition{updatingFalseUpdated},
				},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Completion: 66,
				Operators: operators{
					Total: 1,
					Updated: []operator{
						{
							Name:      "test-operator",
							Condition: updatingFalseUpdated,
						},
					},
				},
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
			},
		},
		{
			name: "Progressing update with updating ClusterOperator",
			cvInsight: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
				Completion: 50,
				Versions: v1alpha1.ControlPlaneUpdateVersions{
					Previous: &v1alpha1.Version{
						Version: "4.10.0",
					},
					Target: v1alpha1.Version{
						Version: "4.11.0",
					},
				},
			},
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{
					Name:       "test-operator",
					Conditions: []metav1.Condition{updatingTrue},
				},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Completion: 50,
				Operators: operators{
					Total: 1,
					Updating: []operator{
						{
							Name:      "test-operator",
							Condition: updatingTrue,
						},
					},
				},
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
			},
		},
		{
			name: "Progressing update from partial previous",
			cvInsight: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
				Completion: 50,
				Versions: v1alpha1.ControlPlaneUpdateVersions{
					Previous: &v1alpha1.Version{
						Version: "4.10.0",
						Metadata: []v1alpha1.VersionMetadata{
							{
								Key: v1alpha1.PartialMetadata,
							},
						},
					},
					Target: v1alpha1.Version{
						Version: "4.11.0",
					},
				},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Completion: 50,
				TargetVersion: versions{
					previous:          "4.10.0",
					target:            "4.11.0",
					isPreviousPartial: true,
				},
			},
		},
		{
			name: "Progressing installation",
			cvInsight: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
				Completion: 3,
				Versions: v1alpha1.ControlPlaneUpdateVersions{
					Target: v1alpha1.Version{
						Version: "4.11.0",
						Metadata: []v1alpha1.VersionMetadata{
							{
								Key: v1alpha1.InstallationMetadata,
							},
						},
					},
				},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Completion: 3,
				TargetVersion: versions{
					target:          "4.11.0",
					isTargetInstall: true,
				},
			},
		},
		{
			name: "Completed installation",
			cvInsight: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentCompleted,
				Completion: 100,
				Versions: v1alpha1.ControlPlaneUpdateVersions{
					Target: v1alpha1.Version{
						Version: "4.11.0",
						Metadata: []v1alpha1.VersionMetadata{
							{
								Key: v1alpha1.InstallationMetadata,
							},
						},
					},
				},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentCompleted),
				Completion: 100,
				TargetVersion: versions{
					target:          "4.11.0",
					isTargetInstall: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := assessControlPlaneStatus(tc.cvInsight, tc.coInsights)
			if diff := cmp.Diff(tc.expected, result, cmp.AllowUnexported(versions{})); diff != "" {
				t.Errorf("assessControlPlaneStatus() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func Test_VersionsString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		versions versions
		expected string
	}{
		{
			name: "installation",
			versions: versions{
				target:          "1.0.0",
				isTargetInstall: true,
			},
			expected: "1.0.0 (installation)",
		},
		{
			name: "upgrade",
			versions: versions{
				previous: "0.9.0",
				target:   "1.0.0",
			},
			expected: "1.0.0 (from 0.9.0)",
		},
		{
			name: "upgrade from partial",
			versions: versions{
				previous:          "0.9.0",
				target:            "1.0.0",
				isPreviousPartial: true,
			},
			expected: "1.0.0 (from incomplete 0.9.0)",
		},
		{
			name: "multiarch migration",
			versions: versions{
				previous:             "1.0.0",
				target:               "1.0.0",
				isMultiArchMigration: true,
			},
			expected: "1.0.0 to Multi-Architecture",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.versions.String()
			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("versionsString() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func Test_VersionsFromClusterVersionProgressInsight(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		insightVersions v1alpha1.ControlPlaneUpdateVersions
		expected        versions
	}{
		{
			name: "installation",
			insightVersions: v1alpha1.ControlPlaneUpdateVersions{
				Target: v1alpha1.Version{
					Version: "4.11.0",
					Metadata: []v1alpha1.VersionMetadata{
						{
							Key: v1alpha1.InstallationMetadata,
						},
					},
				},
			},
			expected: versions{
				target:          "4.11.0",
				isTargetInstall: true,
			},
		},
		{
			name: "upgrade",
			insightVersions: v1alpha1.ControlPlaneUpdateVersions{
				Previous: &v1alpha1.Version{
					Version: "4.10.0",
				},
				Target: v1alpha1.Version{
					Version: "4.11.0",
				},
			},
			expected: versions{
				previous: "4.10.0",
				target:   "4.11.0",
			},
		},
		{
			name: "upgrade from partial",
			insightVersions: v1alpha1.ControlPlaneUpdateVersions{
				Previous: &v1alpha1.Version{
					Version: "4.10.0",
					Metadata: []v1alpha1.VersionMetadata{
						{
							Key: v1alpha1.PartialMetadata,
						},
					},
				},
				Target: v1alpha1.Version{
					Version: "4.11.0",
				},
			},
			expected: versions{
				previous:          "4.10.0",
				target:            "4.11.0",
				isPreviousPartial: true,
			},
		},
		{
			name: "multiarch migration",
			insightVersions: v1alpha1.ControlPlaneUpdateVersions{
				Previous: &v1alpha1.Version{
					Version: "4.10.0",
				},
				Target: v1alpha1.Version{
					Version: "4.11.0",
					Metadata: []v1alpha1.VersionMetadata{
						{
							Key:   v1alpha1.ArchitectureMetadata,
							Value: "multi",
						},
					},
				},
			},
			expected: versions{
				previous:             "4.10.0",
				target:               "4.11.0",
				isMultiArchMigration: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := versionsFromClusterVersionProgressInsight(tc.insightVersions)
			if diff := cmp.Diff(tc.expected, result, cmp.AllowUnexported(versions{})); diff != "" {
				t.Errorf("versionsFromClusterVersionProgressInsight() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}
