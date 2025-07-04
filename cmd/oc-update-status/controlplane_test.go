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
			name: "Progressing",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
			},
			expected: `= Control Plane =
Assessment:      Progressing
`,
		},
		{
			name: "Completed",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentCompleted),
			},
			expected: `= Control Plane =
Assessment:      Completed
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
			name: "Progressing",
			cvInsightStatus: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
			},
		},
		{
			name: "Completed",
			cvInsightStatus: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentCompleted,
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentCompleted),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := assessControlPlaneStatus(tc.cvInsightStatus)
			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("assessControlPlaneStatus() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}
