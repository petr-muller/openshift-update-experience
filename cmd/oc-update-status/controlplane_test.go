package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/petr-muller/openshift-update-experience/api/v1alpha1"
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
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (installation)
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
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (from 4.10.0)
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
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (from incomplete 4.10.0)
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
			},
			expected: `= Control Plane =
Assessment:      Completed
Target Version:  4.11.0 (from 4.10.0)
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
	testCases := []struct {
		name            string
		cvInsightStatus *v1alpha1.ClusterVersionProgressInsightStatus
		expected        controlPlaneStatusDisplayData
	}{
		{
			name: "Progressing update",
			cvInsightStatus: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
				Versions: v1alpha1.ControlPlaneUpdateVersions{
					Previous: &v1alpha1.Version{
						Version: "4.10.0",
					},
					Target: v1alpha1.Version{
						Version: "4.11.0",
					},
				},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
			},
		},
		{
			name: "Progressing update from partial previous",
			cvInsightStatus: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
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
				TargetVersion: versions{
					previous:          "4.10.0",
					target:            "4.11.0",
					isPreviousPartial: true,
				},
			},
		},
		{
			name: "Progressing installation",
			cvInsightStatus: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
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
				TargetVersion: versions{
					target:          "4.11.0",
					isTargetInstall: true,
				},
			},
		},
		{
			name: "Completed installation",
			cvInsightStatus: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentCompleted,
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
				TargetVersion: versions{
					target:          "4.11.0",
					isTargetInstall: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := assessControlPlaneStatus(tc.cvInsightStatus)
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
