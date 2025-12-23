package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/petr-muller/openshift-update-experience/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestRoundDuration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{
			name:     "positive duration > 10m rounds to minutes",
			input:    15*time.Minute + 30*time.Second,
			expected: 16 * time.Minute,
		},
		{
			name:     "positive duration < 10m rounds to seconds",
			input:    5*time.Minute + 500*time.Millisecond,
			expected: 5*time.Minute + 1*time.Second,
		},
		{
			name:     "positive duration exactly 10m rounds to seconds",
			input:    10*time.Minute + 500*time.Millisecond,
			expected: 10 * time.Minute, // 10.5s rounds to 10s (ties round to even)
		},
		{
			name:     "negative duration < -10m rounds to minutes",
			input:    -15*time.Minute - 30*time.Second,
			expected: -16 * time.Minute,
		},
		{
			name:     "negative duration > -10m rounds to seconds",
			input:    -5*time.Minute - 500*time.Millisecond,
			expected: -5*time.Minute - 1*time.Second,
		},
		{
			name:     "negative duration exactly -10m rounds to seconds",
			input:    -10*time.Minute - 500*time.Millisecond,
			expected: -10 * time.Minute, // -10.5s rounds to -10s (ties round to even)
		},
		{
			name:     "zero duration",
			input:    0,
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := roundDuration(tc.input)
			if result != tc.expected {
				t.Errorf("roundDuration(%v) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

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
			expected:  "N/A; multi-architecture migration",
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

var (
	updatingTrue = metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionTrue,
		Reason:  "OperatorGoesBrrr",
		Message: "Operator goes brrr",
	}

	updatingFalsePending = metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionFalse,
		Reason:  "Pending",
		Message: "Operator will go brrr soon",
	}

	updatingFalseUpdated = metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightUpdating),
		Status:  metav1.ConditionFalse,
		Reason:  "Updated",
		Message: "Operator went brrr",
	}

	healthyTrue = metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightHealthy),
		Status:  metav1.ConditionTrue,
		Reason:  "AsExpected",
		Message: "All is well",
	}

	healthyFalseUnavailable = metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightHealthy),
		Status:  metav1.ConditionFalse,
		Reason:  "Unavailable",
		Message: "Operator is not available",
	}

	healthyFalseDegraded = metav1.Condition{
		Type:    string(v1alpha1.ClusterOperatorProgressInsightHealthy),
		Status:  metav1.ConditionFalse,
		Reason:  "Degraded",
		Message: "Operator is degraded",
	}
)

func TestControlPlaneStatusDisplayDataWrite_Assessment(t *testing.T) {
	t.Parallel()

	templateData := controlPlaneStatusDisplayData{
		Completion: 33,
		Duration:   36 * time.Second,
		Operators: operators{
			Total:    3,
			Updated:  []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
			Waiting:  []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
			Updating: []operator{{Name: "test-operator-3", Condition: updatingTrue}},
		},
		TargetVersion: versions{
			previous: "4.10.0",
			target:   "4.11.0",
		},
	}

	for assessment, expectedLine := range map[v1alpha1.ClusterVersionAssessment]string{
		v1alpha1.ClusterVersionAssessmentUnknown:     "Assessment:      Unknown",
		v1alpha1.ClusterVersionAssessmentProgressing: "Assessment:      Progressing",
		v1alpha1.ClusterVersionAssessmentCompleted:   "Assessment:      Completed",
		v1alpha1.ClusterVersionAssessmentDegraded:    "Assessment:      Degraded",
	} {
		t.Run(string(assessment), func(t *testing.T) {
			t.Parallel()

			data := templateData
			data.Assessment = assessmentState(assessment)
			var buf bytes.Buffer
			if err := data.Write(&buf, false, time.Now()); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var assessmentLine string
			for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
				if bytes.HasPrefix(line, []byte("Assessment:")) {
					assessmentLine = string(line)
					break
				}
			}

			if assessmentLine == "" {
				t.Fatalf("Expected assessment line not found in output: %s", buf.String())
			}

			if diff := cmp.Diff(expectedLine, assessmentLine); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func TestControlPlaneStatusDisplayDataWrite_TargetVersion(t *testing.T) {
	t.Parallel()

	templateData := controlPlaneStatusDisplayData{
		Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
		Completion: 33,
		Duration:   36 * time.Second,
		Operators: operators{
			Total:    3,
			Updated:  []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
			Waiting:  []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
			Updating: []operator{{Name: "test-operator-3", Condition: updatingTrue}},
		},
	}

	testCases := []struct {
		name     string
		versions versions
		expected string
	}{
		{
			name: "installation",
			versions: versions{
				target:          "4.11.0",
				isTargetInstall: true,
			},
			expected: "Target Version:  4.11.0 (installation)",
		},
		{
			name: "update",
			versions: versions{
				previous: "4.10.0",
				target:   "4.11.0",
			},
			expected: "Target Version:  4.11.0 (from 4.10.0)",
		},
		{
			name: "update from partial",
			versions: versions{
				previous:          "4.10.0",
				target:            "4.11.0",
				isPreviousPartial: true,
			},
			expected: "Target Version:  4.11.0 (from incomplete 4.10.0)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := templateData
			data.TargetVersion = tc.versions

			var buf bytes.Buffer
			if err := data.Write(&buf, false, time.Now()); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var targetVersionLine string
			for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
				if bytes.HasPrefix(line, []byte("Target Version: ")) {
					targetVersionLine = string(line)
					break
				}
			}

			if targetVersionLine == "" {
				t.Fatalf("Expected target version line not found in output: %s", buf.String())
			}

			if diff := cmp.Diff(tc.expected, targetVersionLine); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func TestControlPlaneStatusDisplayDataWrite_UpdatingOperators(t *testing.T) {
	t.Parallel()

	templateData := controlPlaneStatusDisplayData{
		Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
		Completion: 33,
		Duration:   36 * time.Second,
		Operators: operators{
			Total:   3,
			Updated: []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
			Waiting: []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
		},
		TargetVersion: versions{
			previous: "4.10.0",
			target:   "4.11.0",
		},
	}

	testCases := []struct {
		name              string
		updatingOperators []operator
		expected          string
	}{
		{
			name:              "no updating operators",
			updatingOperators: nil,
			expected:          "",
		},
		{
			name: "single updating operator",
			updatingOperators: []operator{
				{Name: "test-operator-1", Condition: updatingTrue},
			},
			expected: "Updating:        test-operator-1",
		},
		{
			name: "multiple updating operators",
			updatingOperators: []operator{
				{Name: "test-operator-1", Condition: updatingTrue},
				{Name: "test-operator-2", Condition: updatingTrue},
				{Name: "test-operator-3", Condition: updatingTrue},
			},
			expected: "Updating:        test-operator-1, test-operator-2, test-operator-3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := templateData
			data.Operators.Updating = tc.updatingOperators

			var buf bytes.Buffer
			if err := data.Write(&buf, false, time.Now()); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var updatingLine string
			for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
				if bytes.HasPrefix(line, []byte("Updating:")) {
					updatingLine = string(line)
					break
				}
			}

			if diff := cmp.Diff(tc.expected, updatingLine); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func TestControlPlaneStatusDisplayDataWrite_UpdatingOperatorsTable(t *testing.T) {
	t.Parallel()

	now := metav1.Now()

	to1Updating := *updatingTrue.DeepCopy()
	to1Updating.LastTransitionTime = metav1.NewTime(now.Add(-5*time.Minute - 15*time.Second))
	to2Updating := *updatingTrue.DeepCopy()
	to2Updating.LastTransitionTime = metav1.NewTime(now.Add(-10*time.Minute - 30*time.Second))
	to3Updating := *updatingTrue.DeepCopy()
	to3Updating.LastTransitionTime = metav1.NewTime(now.Add(-15*time.Minute - 45*time.Second))

	templateData := controlPlaneStatusDisplayData{
		Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
		Completion: 33,
		Duration:   36 * time.Second,
		Operators: operators{
			Total:   3,
			Updated: []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
			Waiting: []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
		},
		TargetVersion: versions{
			previous: "4.10.0",
			target:   "4.11.0",
		},
	}

	testCases := []struct {
		name              string
		updatingOperators []operator
		expectLines       []string
	}{
		{
			name:              "no updating operators",
			updatingOperators: nil,
		},
		{
			name: "single updating operator",
			updatingOperators: []operator{
				{Name: "test-operator-1", Condition: to1Updating},
			},
			expectLines: []string{
				"NAME              SINCE   REASON             MESSAGE",
				"test-operator-1   5m15s   OperatorGoesBrrr   Operator goes brrr",
			},
		},
		{
			name: "multiple updating operators",
			updatingOperators: []operator{
				{Name: "test-operator-1", Condition: to1Updating},
				{Name: "test-operator-2", Condition: to2Updating},
				{Name: "test-operator-3", Condition: to3Updating},
			},
			expectLines: []string{
				"NAME              SINCE    REASON             MESSAGE",
				"test-operator-1   5m15s    OperatorGoesBrrr   Operator goes brrr",
				"test-operator-2   10m30s   OperatorGoesBrrr   Operator goes brrr",
				"test-operator-3   15m45s   OperatorGoesBrrr   Operator goes brrr",
			},
		},
		{
			name: "operator with multiline message",
			updatingOperators: []operator{
				{
					Name: "dns",
					Condition: metav1.Condition{
						Type:   string(v1alpha1.ClusterOperatorProgressInsightUpdating),
						Status: metav1.ConditionTrue,
						Reason: "Progressing",
						Message: "Progressing=True: DNS \"default\" reports Progressing=True\n" +
							"Upgrading operator to \"4.22.0\"\n" +
							"Upgrading coredns",
						LastTransitionTime: metav1.NewTime(now.Add(-1*time.Minute - 45*time.Second)),
					},
				},
			},
			expectLines: []string{
				"NAME   SINCE   REASON        MESSAGE",
				"dns    1m45s   Progressing   Progressing=True: DNS \"default\" reports Progressing=True",
				"                             Upgrading operator to \"4.22.0\"",
				"                             Upgrading coredns",
			},
		},
		{
			name: "operator with trailing newlines in message",
			updatingOperators: []operator{
				{
					Name: "authentication",
					Condition: metav1.Condition{
						Type:   string(v1alpha1.ClusterOperatorProgressInsightUpdating),
						Status: metav1.ConditionTrue,
						Reason: "Progressing",
						Message: "Updating authentication operator\n" +
							"Redeploying pods\n\n\n",
						LastTransitionTime: metav1.NewTime(now.Add(-30 * time.Second)),
					},
				},
			},
			expectLines: []string{
				"NAME             SINCE   REASON        MESSAGE",
				"authentication   30s     Progressing   Updating authentication operator",
				"                                       Redeploying pods",
			},
		},
		{
			name: "operator with empty lines in middle of message",
			updatingOperators: []operator{
				{
					Name: "network",
					Condition: metav1.Condition{
						Type:               string(v1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:             metav1.ConditionTrue,
						Reason:             "Updating",
						Message:            "Starting network update\n\nConfiguring network policies\n\nFinishing update",
						LastTransitionTime: metav1.NewTime(now.Add(-2 * time.Minute)),
					},
				},
			},
			expectLines: []string{
				"NAME      SINCE   REASON     MESSAGE",
				"network   2m      Updating   Starting network update",
				"                             Configuring network policies",
				"                             Finishing update",
			},
		},
		{
			name: "operator with single line message",
			updatingOperators: []operator{
				{
					Name: "console",
					Condition: metav1.Condition{
						Type:               string(v1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:             metav1.ConditionTrue,
						Reason:             "Rolling",
						Message:            "Rolling out console deployment",
						LastTransitionTime: metav1.NewTime(now.Add(-45 * time.Second)),
					},
				},
			},
			expectLines: []string{
				"NAME      SINCE   REASON    MESSAGE",
				"console   45s     Rolling   Rolling out console deployment",
			},
		},
		{
			name: "operator with empty message",
			updatingOperators: []operator{
				{
					Name: "monitoring",
					Condition: metav1.Condition{
						Type:               string(v1alpha1.ClusterOperatorProgressInsightUpdating),
						Status:             metav1.ConditionTrue,
						Reason:             "Progressing",
						Message:            "",
						LastTransitionTime: metav1.NewTime(now.Add(-15 * time.Second)),
					},
				},
			},
			expectLines: []string{
				"NAME         SINCE   REASON        MESSAGE",
				"monitoring   15s     Progressing",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := templateData
			data.Operators.Updating = tc.updatingOperators

			var buf bytes.Buffer
			if err := data.Write(&buf, false, now.Time); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			output := buf.String()
			if hasOperatorsSection := strings.Contains(output, "Updating Cluster Operators"); hasOperatorsSection {
				t.Errorf("Should not have Updating Cluster Operators section with detailed=false:\n%s", output)
			}

			buf.Reset()
			if err := data.Write(&buf, true, now.Time); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			output = buf.String()
			expectOperatorsSection := len(tc.updatingOperators) > 0
			hasOperatorsSection := strings.Contains(output, "Updating Cluster Operators")

			if expectOperatorsSection != hasOperatorsSection {
				t.Errorf(
					"Expected Updating Cluster Operators section presence: %v, got: %v\n%s",
					expectOperatorsSection,
					hasOperatorsSection,
					output,
				)
			}

			if expectOperatorsSection {
				expectedOperatorSection := strings.Join(append([]string{
					"Updating Cluster Operators",
				}, tc.expectLines...), "\n")

				if !strings.Contains(output, expectedOperatorSection) {
					t.Errorf("Expected operator section:\n%s\nGot output:\n%s", expectedOperatorSection, output)
				}
			}
		})
	}
}

func TestControlPlaneStatusDisplayDataWrite_Completion(t *testing.T) {
	t.Parallel()

	templateData := controlPlaneStatusDisplayData{
		Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
		Duration:   36 * time.Second,
		Operators: operators{
			Total:   3,
			Updated: []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
			Waiting: []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
		},
		TargetVersion: versions{
			previous: "4.10.0",
			target:   "4.11.0",
		},
	}

	placeholder := regexp.MustCompile(`\(.*\)`)

	for completion, expectedLine := range map[float64]string{
		0:   "Completion:      0% <OPERATORS>",
		25:  "Completion:      25% <OPERATORS>",
		50:  "Completion:      50% <OPERATORS>",
		75:  "Completion:      75% <OPERATORS>",
		100: "Completion:      100% <OPERATORS>",
	} {
		t.Run(expectedLine, func(t *testing.T) {
			t.Parallel()

			data := templateData
			data.Completion = completion

			var buf bytes.Buffer
			if err := data.Write(&buf, false, time.Now()); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var completionLine string
			for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
				if bytes.HasPrefix(line, []byte("Completion:")) {
					completionLine = placeholder.ReplaceAllString(string(line), "<OPERATORS>")
					break
				}
			}

			if completionLine == "" {
				t.Fatalf("Expected completion line not found in output: %s", buf.String())
			}

			if diff := cmp.Diff(expectedLine, completionLine); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func TestControlPlaneStatusDisplayDataWrite_CompletionOperators(t *testing.T) {
	t.Parallel()

	templateData := controlPlaneStatusDisplayData{
		Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
		Duration:   36 * time.Second,
		Completion: 50,
		TargetVersion: versions{
			previous: "4.10.0",
			target:   "4.11.0",
		},
	}

	testCases := []struct {
		name      string
		operators operators
		expected  string
	}{
		{
			name: "all pending",
			operators: operators{
				Total:   3,
				Updated: []operator{},
				Waiting: []operator{
					{Name: "test-operator-1", Condition: updatingFalsePending},
					{Name: "test-operator-2", Condition: updatingFalsePending},
					{Name: "test-operator-3", Condition: updatingFalsePending},
				},
			},
			expected: "Completion:      <PERCENTAGE> (0 operators updated, 0 updating, 3 waiting)",
		},
		{
			name: "all updated",
			operators: operators{
				Total: 3,
				Updated: []operator{
					{Name: "test-operator-1", Condition: updatingFalseUpdated},
					{Name: "test-operator-2", Condition: updatingFalseUpdated},
					{Name: "test-operator-3", Condition: updatingFalseUpdated},
				},
			},
			expected: "Completion:      <PERCENTAGE> (3 operators updated, 0 updating, 0 waiting)",
		},
		{
			name: "one of each",
			operators: operators{
				Total:    3,
				Updated:  []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
				Waiting:  []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
				Updating: []operator{{Name: "test-operator-3", Condition: updatingTrue}},
			},
			expected: "Completion:      <PERCENTAGE> (1 operators updated, 1 updating, 1 waiting)",
		},
	}

	placeholder := regexp.MustCompile(`\d+%`)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := templateData
			data.Operators = tc.operators

			var buf bytes.Buffer
			if err := data.Write(&buf, false, time.Now()); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var completionLine string
			for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
				if bytes.HasPrefix(line, []byte("Completion:")) {
					completionLine = placeholder.ReplaceAllString(string(line), "<PERCENTAGE>")
					break
				}
			}

			if completionLine == "" {
				t.Fatalf("Expected completion line not found in output: %s", buf.String())
			}

			if diff := cmp.Diff(tc.expected, completionLine); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
			}

		})
	}
}

func TestControlPlaneStatusDisplayDataWrite_Duration(t *testing.T) {
	t.Parallel()

	templateData := controlPlaneStatusDisplayData{
		Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
		Completion: 33,
		Operators: operators{
			Total:   3,
			Updated: []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
			Waiting: []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
		},
		TargetVersion: versions{
			previous: "4.10.0",
			target:   "4.11.0",
		},
	}

	for duration, expectedLine := range map[time.Duration]string{
		time.Second:                           "Duration:        1s",
		time.Minute:                           "Duration:        1m",
		time.Hour:                             "Duration:        1h",
		time.Hour + time.Minute + time.Second: "Duration:        1h1m1s",
		10*time.Hour + 10*time.Minute:         "Duration:        10h10m",
	} {
		t.Run(expectedLine, func(t *testing.T) {
			t.Parallel()

			data := templateData
			data.Duration = duration

			var buf bytes.Buffer
			if err := data.Write(&buf, false, time.Now()); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var durationLine string
			for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
				if bytes.HasPrefix(line, []byte("Duration:")) {
					durationLine = string(line)
					break
				}
			}

			if durationLine == "" {
				t.Fatalf("Expected duration line not found in output: %s", buf.String())
			}

			if diff := cmp.Diff(expectedLine, durationLine); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func TestControlPlaneStatusDisplayDataWrite_OperatorHealth(t *testing.T) {
	t.Parallel()

	templateData := controlPlaneStatusDisplayData{
		Assessment:        assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
		Completion:        33,
		Duration:          42 * time.Minute,
		EstTimeToComplete: 18 * time.Minute,
		EstDuration:       60 * time.Minute,
		TargetVersion: versions{
			previous: "4.10.0",
			target:   "4.11.0",
		},
	}

	testCases := []struct {
		name      string
		operators operators
		expected  string
	}{
		{
			name:      "no operators",
			operators: operators{Total: 0},
			expected:  "Operator Health: 0 healthy",
		},
		{
			name: "all healthy",
			operators: operators{
				Total: 3,
				Healthy: []operator{
					{Name: "test-operator-1", Condition: healthyTrue},
					{Name: "test-operator-2", Condition: healthyTrue},
					{Name: "test-operator-3", Condition: healthyTrue},
				},
			},
			expected: "Operator Health: 3 healthy",
		},
		{
			name: "some healthy",
			operators: operators{
				Total: 3,
				Healthy: []operator{
					{Name: "test-operator-1", Condition: healthyTrue},
					{Name: "test-operator-2", Condition: healthyTrue},
				},
				Unavailable: []operator{
					{Name: "test-operator-3", Condition: healthyFalseUnavailable},
				},
			},
			expected: "Operator Health: 2 healthy, 1 unavailable",
		},
		{
			name: "some degraded",
			operators: operators{
				Total: 3,
				Healthy: []operator{
					{Name: "test-operator-1", Condition: healthyTrue},
					{Name: "test-operator-2", Condition: healthyTrue},
				},
				Degraded: []operator{
					{Name: "test-operator-3", Condition: healthyFalseDegraded},
				},
			},
			expected: "Operator Health: 2 healthy, 1 available but degraded",
		},
		{
			name: "one of each",
			operators: operators{
				Total:       3,
				Healthy:     []operator{{Name: "test-operator-1", Condition: healthyTrue}},
				Unavailable: []operator{{Name: "test-operator-2", Condition: healthyFalseUnavailable}},
				Degraded:    []operator{{Name: "test-operator-3", Condition: healthyFalseDegraded}},
			},
			expected: "Operator Health: 1 healthy, 1 unavailable, 1 available but degraded",
		},
		{
			name: "one unknown",
			operators: operators{
				Total: 3,
				Healthy: []operator{
					{Name: "test-operator-1", Condition: healthyTrue},
					{Name: "test-operator-2", Condition: healthyTrue},
				},
			},
			expected: "Operator Health: 2 healthy, 1 unknown",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := templateData
			data.Operators = tc.operators

			var buf bytes.Buffer
			if err := data.Write(&buf, false, time.Now()); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var healthLine string
			for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
				if bytes.HasPrefix(line, []byte("Operator Health:")) {
					healthLine = string(line)
					break
				}
			}

			if healthLine == "" {
				t.Fatalf("Expected operator health line not found in output: %s", buf.String())
			}

			if diff := cmp.Diff(tc.expected, healthLine); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func TestControlPlaneStatusDisplayDataWrite_DurationEstimate(t *testing.T) {
	t.Parallel()

	templateData := controlPlaneStatusDisplayData{
		Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
		Completion: 33,
		Duration:   42 * time.Minute,
		Operators: operators{
			Total:   3,
			Updated: []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
			Waiting: []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
		},
		TargetVersion: versions{
			previous: "4.10.0",
			target:   "4.11.0",
		},
	}

	testCases := []struct {
		name          string
		estToComplete time.Duration
		estDuration   time.Duration
		expected      string
	}{
		{
			name:          "normal estimate",
			estToComplete: 18 * time.Minute,
			estDuration:   60 * time.Minute,
			expected:      "Duration:        <DURATION> (Est. Time Remaining: 18m)",
		},
		{
			name:          "close to estimate",
			estToComplete: 2 * time.Minute,
			estDuration:   44 * time.Minute,
			expected:      "Duration:        <DURATION> (Est. Time Remaining: <10m)",
		},
		{
			name:          "slightly over estimate",
			estToComplete: -2 * time.Minute,
			estDuration:   40 * time.Minute,
			expected:      "Duration:        <DURATION> (Est. Time Remaining: <10m)",
		},
		{
			name:          "over estimate",
			estToComplete: -12 * time.Minute,
			estDuration:   30 * time.Minute,
			expected:      "Duration:        <DURATION> (Est. Time Remaining: N/A; estimate duration was 30m)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := templateData
			data.EstTimeToComplete = tc.estToComplete
			data.EstDuration = tc.estDuration

			var buf bytes.Buffer
			if err := data.Write(&buf, false, time.Now()); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var durationLine string
			for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
				if bytes.HasPrefix(line, []byte("Duration:")) {
					durationLine = strings.Replace(string(line), "42m", "<DURATION>", 1)
					break
				}
			}

			if durationLine == "" {
				t.Fatalf("Expected duration line not found in output: %s", buf.String())
			}

			if diff := cmp.Diff(tc.expected, durationLine); diff != "" {
				t.Errorf("controlPlaneStatusDisplayData.Write() mismatch (-expected +got):\n%s", diff)
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
			name: "Progressing update",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Completion: 33,
				Duration:   36 * time.Second,
				Operators: operators{
					Total: 3,
					Updated: []operator{
						{Name: "test-operator-1", Condition: updatingFalseUpdated},
					},
					Waiting: []operator{
						{Name: "test-operator-2", Condition: updatingFalsePending},
					},
					Updating: []operator{
						{Name: "test-operator-3", Condition: updatingTrue},
					},
					Healthy: []operator{
						{Name: "test-operator-1", Condition: healthyTrue},
					},
					Unavailable: []operator{
						{Name: "test-operator-2", Condition: healthyFalseUnavailable},
					},
					Degraded: []operator{
						{Name: "test-operator-3", Condition: healthyFalseDegraded},
					},
				},
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
			},
			expected: `= Control Plane =
Assessment:      Progressing
Target Version:  4.11.0 (from 4.10.0)
Updating:        test-operator-3
Completion:      33% (1 operators updated, 1 updating, 1 waiting)
Duration:        36s
Operator Health: 1 healthy, 1 unavailable, 1 available but degraded
`,
		},
		{
			name: "Completed",
			data: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentCompleted),
				Duration:   56 * time.Minute,
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
				Completion: 100,
				Operators: operators{
					Total: 1,
					Updated: []operator{
						{Name: "test-operator", Condition: updatingFalseUpdated},
					},
					Healthy: []operator{
						{Name: "test-operator", Condition: healthyTrue},
					},
				},
			},
			expected: `= Control Plane =
Assessment:      Completed
Target Version:  4.11.0 (from 4.10.0)
Completion:      100% (1 operators updated, 0 updating, 0 waiting)
Duration:        56m
Operator Health: 1 healthy
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

var (
	cvInsight = v1alpha1.ClusterVersionProgressInsightStatus{
		Name:       "version",
		Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
		Versions: v1alpha1.ControlPlaneUpdateVersions{
			Previous: &v1alpha1.Version{
				Version: "4.10.0",
			},
			Target: v1alpha1.Version{
				Version: "4.11.0",
			},
		},
		Completion:           50,
		StartedAt:            metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		CompletedAt:          nil,
		EstimatedCompletedAt: nil,
		Conditions:           nil,
	}
)

func Test_assessControlPlaneStatus_Assessment(t *testing.T) {
	t.Parallel()

	for _, assessment := range []v1alpha1.ClusterVersionAssessment{
		v1alpha1.ClusterVersionAssessmentUnknown,
		v1alpha1.ClusterVersionAssessmentProgressing,
		v1alpha1.ClusterVersionAssessmentCompleted,
		v1alpha1.ClusterVersionAssessmentDegraded,
	} {
		t.Run(string(assessment), func(t *testing.T) {
			t.Parallel()

			insight := cvInsight.DeepCopy()
			insight.Assessment = assessment

			controlPlaneStatus := assessControlPlaneStatus(insight, nil, time.Now())
			if controlPlaneStatus.Assessment != assessmentState(assessment) {
				t.Errorf("Expected assessment %s, got %s", assessment, controlPlaneStatus.Assessment)
			}
		})
	}

}

func Test_assessControlPlaneStatus_Completion(t *testing.T) {
	t.Parallel()

	for _, completion := range []int32{0, 25, 50, 75, 100} {
		t.Run(string(completion), func(t *testing.T) {
			t.Parallel()

			insight := cvInsight.DeepCopy()
			insight.Completion = completion

			controlPlaneStatus := assessControlPlaneStatus(insight, nil, time.Now())
			if controlPlaneStatus.Completion != float64(completion) {
				t.Errorf("Expected completion %f, got %f", float64(completion), controlPlaneStatus.Completion)
			}
		})
	}

}

func Test_assessControlPlaneStatus_Duration(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	for started, expected := range map[metav1.Time]time.Duration{
		now: 0 * time.Second,
		metav1.NewTime(now.Add(-30 * time.Minute)):                    30 * time.Minute,
		metav1.NewTime(now.Add(-5*time.Minute - 10*time.Millisecond)): 5 * time.Minute, // Rounded to seconds when under 10m
		metav1.NewTime(now.Add(-55*time.Second - 9*time.Minute)):      55*time.Second + 9*time.Minute,
		metav1.NewTime(now.Add(-55*time.Second - 10*time.Minute)):     11 * time.Minute, // Rounded to minutes when over 10m
	} {
		t.Run(expected.String(), func(t *testing.T) {
			t.Parallel()

			insight := cvInsight.DeepCopy()
			insight.StartedAt = started

			controlPlaneStatus := assessControlPlaneStatus(insight, nil, now.Time)
			if controlPlaneStatus.Duration != expected {
				t.Errorf(
					"Expected duration %s (between started %s and now %s), got %s",
					expected,
					started.Format(time.TimeOnly),
					now.Format(time.TimeOnly),
					controlPlaneStatus.Duration,
				)
			}
		})

		t.Run(fmt.Sprintf("CompletedAt: started+%s", expected.String()), func(t *testing.T) {
			t.Parallel()

			insight := cvInsight.DeepCopy()
			adjustedStarted := started.Add(-30 * time.Minute)
			insight.StartedAt = metav1.NewTime(adjustedStarted)
			insight.CompletedAt = ptr.To(metav1.NewTime(adjustedStarted.Add(expected)))

			controlPlaneStatus := assessControlPlaneStatus(insight, nil, now.Time)
			if controlPlaneStatus.Duration != expected {
				t.Errorf(
					"Expected duration %s (between started %s and completed %s with now %s), got %s",
					expected,
					adjustedStarted.Format(time.TimeOnly),
					insight.CompletedAt.Format(time.TimeOnly),
					now.Format(time.TimeOnly),
					controlPlaneStatus.Duration,
				)
			}
		})
	}
}

func Test_assessControlPlaneStatus_Estimates(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	testCases := []struct {
		name                  string
		startedAt             metav1.Time
		estimatedDuration     time.Duration
		expectedEstDuration   time.Duration
		expectedEstToComplete time.Duration
	}{
		{
			name:                  "exact minutes - 41m remaining",
			startedAt:             now,
			estimatedDuration:     41 * time.Minute,
			expectedEstDuration:   41 * time.Minute,
			expectedEstToComplete: 41 * time.Minute,
		},
		{
			name:                  "exact minutes - 12m remaining",
			startedAt:             metav1.NewTime(now.Add(-30 * time.Minute)),
			estimatedDuration:     42 * time.Minute,
			expectedEstDuration:   42 * time.Minute,
			expectedEstToComplete: 12 * time.Minute,
		},
		{
			name:                  "exact minutes - negative 47m (past estimate)",
			startedAt:             metav1.NewTime(now.Add(-90 * time.Minute)),
			estimatedDuration:     43 * time.Minute,
			expectedEstDuration:   43 * time.Minute,
			expectedEstToComplete: -47 * time.Minute,
		},
		{
			name:                  "duration with seconds > 10m rounds to minutes",
			startedAt:             now,
			estimatedDuration:     15*time.Minute + 30*time.Second,
			expectedEstDuration:   16 * time.Minute,
			expectedEstToComplete: 16 * time.Minute,
		},
		{
			name:                  "duration with milliseconds < 10m rounds to seconds",
			startedAt:             now,
			estimatedDuration:     5*time.Minute + 500*time.Millisecond,
			expectedEstDuration:   5*time.Minute + 1*time.Second,
			expectedEstToComplete: 5*time.Minute + 1*time.Second,
		},
		{
			name:                "negative remaining time < -10m rounds to minutes",
			startedAt:           metav1.NewTime(now.Add(-30 * time.Minute)),
			estimatedDuration:   15*time.Minute + 30*time.Second,
			expectedEstDuration: 16 * time.Minute,
			// (now-30m)+16m = now-14m, started with 15m30s rounds to 16m, so -14m30s rounds to -15m
			expectedEstToComplete: -15 * time.Minute,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			insight := cvInsight.DeepCopy()
			insight.StartedAt = tc.startedAt
			insight.EstimatedCompletedAt = ptr.To(metav1.NewTime(tc.startedAt.Add(tc.estimatedDuration)))

			controlPlaneStatus := assessControlPlaneStatus(insight, nil, now.Time)
			if controlPlaneStatus.EstDuration != tc.expectedEstDuration {
				t.Errorf("Expected estimated duration %s, got %s", tc.expectedEstDuration, controlPlaneStatus.EstDuration)
			}

			if controlPlaneStatus.EstTimeToComplete != tc.expectedEstToComplete {
				t.Errorf(
					"Expected estimated time to complete %s, got %s",
					tc.expectedEstToComplete,
					controlPlaneStatus.EstTimeToComplete,
				)
			}
		})
	}
}

func Test_assessControlPlaneStatus_TargetVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		versions v1alpha1.ControlPlaneUpdateVersions
		expected versions
	}{
		{
			name: "installation",
			versions: v1alpha1.ControlPlaneUpdateVersions{
				Target: v1alpha1.Version{
					Version:  "4.11.0",
					Metadata: []v1alpha1.VersionMetadata{{Key: v1alpha1.InstallationMetadata}},
				},
			},
			expected: versions{
				target:          "4.11.0",
				isTargetInstall: true,
			},
		},
		{
			name: "update",
			versions: v1alpha1.ControlPlaneUpdateVersions{
				Previous: &v1alpha1.Version{Version: "4.10.0"},
				Target:   v1alpha1.Version{Version: "4.11.0"},
			},
			expected: versions{
				previous: "4.10.0",
				target:   "4.11.0",
			},
		},
		{
			name: "update from partial",
			versions: v1alpha1.ControlPlaneUpdateVersions{
				Previous: &v1alpha1.Version{
					Version:  "4.10.0",
					Metadata: []v1alpha1.VersionMetadata{{Key: v1alpha1.PartialMetadata}},
				},
				Target: v1alpha1.Version{Version: "4.11.0"},
			},
			expected: versions{
				previous:          "4.10.0",
				target:            "4.11.0",
				isPreviousPartial: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			insight := cvInsight.DeepCopy()
			insight.Versions = tc.versions

			controlPlaneStatus := assessControlPlaneStatus(insight, nil, time.Now())
			if diff := cmp.Diff(tc.expected, controlPlaneStatus.TargetVersion, cmp.AllowUnexported(versions{})); diff != "" {
				t.Errorf("TargetVersion mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func Test_assessControlPlaneStatus_Operators(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		coInsights []v1alpha1.ClusterOperatorProgressInsightStatus
		expected   operators
	}{
		{
			name: "all operators pending",
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{Name: "test-operator-1", Conditions: []metav1.Condition{updatingFalsePending}},
				{Name: "test-operator-2", Conditions: []metav1.Condition{updatingFalsePending}},
				{Name: "test-operator-3", Conditions: []metav1.Condition{updatingFalsePending}},
			},
			expected: operators{
				Total: 3,
				Waiting: []operator{
					{Name: "test-operator-1", Condition: updatingFalsePending},
					{Name: "test-operator-2", Condition: updatingFalsePending},
					{Name: "test-operator-3", Condition: updatingFalsePending},
				},
			},
		},
		{
			name: "all operators updated",
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{Name: "test-operator-1", Conditions: []metav1.Condition{updatingFalseUpdated}},
				{Name: "test-operator-2", Conditions: []metav1.Condition{updatingFalseUpdated}},
			},
			expected: operators{
				Total: 2,
				Updated: []operator{
					{Name: "test-operator-1", Condition: updatingFalseUpdated},
					{Name: "test-operator-2", Condition: updatingFalseUpdated},
				},
			},
		},
		{
			name: "one of each",
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{Name: "test-operator-1", Conditions: []metav1.Condition{updatingFalseUpdated}},
				{Name: "test-operator-2", Conditions: []metav1.Condition{updatingFalsePending}},
				{Name: "test-operator-3", Conditions: []metav1.Condition{updatingTrue}},
			},
			expected: operators{
				Total:    3,
				Updated:  []operator{{Name: "test-operator-1", Condition: updatingFalseUpdated}},
				Waiting:  []operator{{Name: "test-operator-2", Condition: updatingFalsePending}},
				Updating: []operator{{Name: "test-operator-3", Condition: updatingTrue}},
			},
		},
		{
			name: "healthy operator",
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{Name: "test-operator-1", Conditions: []metav1.Condition{healthyTrue}},
			},
			expected: operators{
				Total:   1,
				Healthy: []operator{{Name: "test-operator-1", Condition: healthyTrue}},
			},
		},
		{
			name: "unavailable operator",
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{Name: "test-operator-1", Conditions: []metav1.Condition{healthyFalseUnavailable}},
			},
			expected: operators{
				Total:       1,
				Unavailable: []operator{{Name: "test-operator-1", Condition: healthyFalseUnavailable}},
			},
		},
		{
			name: "degraded operator",
			coInsights: []v1alpha1.ClusterOperatorProgressInsightStatus{
				{Name: "test-operator-1", Conditions: []metav1.Condition{healthyFalseDegraded}},
			},
			expected: operators{
				Total:    1,
				Degraded: []operator{{Name: "test-operator-1", Condition: healthyFalseDegraded}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			insight := cvInsight.DeepCopy()
			controlPlaneStatus := assessControlPlaneStatus(insight, tc.coInsights, time.Now())

			if diff := cmp.Diff(tc.expected, controlPlaneStatus.Operators, cmp.AllowUnexported(operators{})); diff != "" {
				t.Errorf("Operators mismatch (-expected +got):\n%s", diff)
			}
		})
	}
}

func Test_assessControlPlaneStatus(t *testing.T) {
	t.Parallel()
	now := time.Now()
	var minutesAgo [60]metav1.Time
	for i := range minutesAgo {
		minutesAgo[i] = metav1.NewTime(now.Add(-time.Duration(i) * time.Minute))
	}

	testCases := []struct {
		name       string
		cvInsight  *v1alpha1.ClusterVersionProgressInsightStatus
		coInsights []v1alpha1.ClusterOperatorProgressInsightStatus
		expected   controlPlaneStatusDisplayData
	}{
		{
			name: "Progressing update",
			cvInsight: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment: v1alpha1.ClusterVersionAssessmentProgressing,
				Completion: 50,
				StartedAt:  minutesAgo[30],
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
				{Name: "test-operator", Conditions: []metav1.Condition{updatingTrue}},
				{Name: "test-operator-2", Conditions: []metav1.Condition{updatingFalsePending}},
				{Name: "test-operator-3", Conditions: []metav1.Condition{updatingFalseUpdated}},
				{Name: "test-operator-4", Conditions: []metav1.Condition{updatingFalseUpdated}},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentProgressing),
				Completion: 50,
				Duration:   30 * time.Minute,
				Operators: operators{
					Total: 4,
					Updating: []operator{
						{Name: "test-operator", Condition: updatingTrue},
					},
					Waiting: []operator{
						{Name: "test-operator-2", Condition: updatingFalsePending},
					},
					Updated: []operator{
						{Name: "test-operator-3", Condition: updatingFalseUpdated},
						{Name: "test-operator-4", Condition: updatingFalseUpdated},
					},
				},
				TargetVersion: versions{
					previous: "4.10.0",
					target:   "4.11.0",
				},
			},
		},
		{
			name: "Completed installation",
			cvInsight: &v1alpha1.ClusterVersionProgressInsightStatus{
				Assessment:  v1alpha1.ClusterVersionAssessmentCompleted,
				Completion:  100,
				StartedAt:   minutesAgo[50],
				CompletedAt: &minutesAgo[10],
				Versions: v1alpha1.ControlPlaneUpdateVersions{
					Target: v1alpha1.Version{
						Version:  "4.11.0",
						Metadata: []v1alpha1.VersionMetadata{{Key: v1alpha1.InstallationMetadata}},
					},
				},
			},
			expected: controlPlaneStatusDisplayData{
				Assessment: assessmentState(v1alpha1.ClusterVersionAssessmentCompleted),
				Duration:   40 * time.Minute,
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
			result := assessControlPlaneStatus(tc.cvInsight, tc.coInsights, now)
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
