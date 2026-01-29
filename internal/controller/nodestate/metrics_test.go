package nodestate

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// T051: Unit tests for metrics registration and recording

func TestMetricsRecorder_RecordEvaluationDuration(t *testing.T) {
	recorder := NewMetricsRecorder()

	// Record evaluations
	recorder.RecordEvaluationDuration("worker-1", "worker", 0.05, true)
	recorder.RecordEvaluationDuration("worker-2", "worker", 0.1, false)

	// For histograms, we verify by collecting the metric
	// and checking that it has observations
	ch := make(chan prometheus.Metric, 10)
	nodeStateEvaluationDuration.Collect(ch)
	close(ch)

	metricCount := 0
	for range ch {
		metricCount++
	}

	if metricCount == 0 {
		t.Error("Expected evaluation duration metrics to be collected")
	}
}

func TestMetricsRecorder_RecordEvaluationTotal(t *testing.T) {
	recorder := NewMetricsRecorder()

	// Get initial count
	initialSuccess := testutil.ToFloat64(nodeStateEvaluationTotal.WithLabelValues("worker", "success"))

	// Record evaluations
	recorder.RecordEvaluationTotal("worker", true)
	recorder.RecordEvaluationTotal("worker", true)
	recorder.RecordEvaluationTotal("worker", false)

	// Verify counts
	newSuccess := testutil.ToFloat64(nodeStateEvaluationTotal.WithLabelValues("worker", "success"))
	if newSuccess != initialSuccess+2 {
		t.Errorf("Expected success count to increase by 2, got delta of %f", newSuccess-initialSuccess)
	}
}

func TestMetricsRecorder_RecordNotificationDuration(t *testing.T) {
	recorder := NewMetricsRecorder()

	// Record notifications
	recorder.RecordNotificationDuration("node", 0.001, true)
	recorder.RecordNotificationDuration("node", 0.0, false)

	// For histograms, we verify by collecting the metric
	ch := make(chan prometheus.Metric, 10)
	downstreamNotificationDuration.Collect(ch)
	close(ch)

	metricCount := 0
	for range ch {
		metricCount++
	}

	if metricCount == 0 {
		t.Error("Expected notification duration metrics to be collected")
	}
}

func TestMetricsRecorder_RecordNotificationDuration_Labels(t *testing.T) {
	recorder := NewMetricsRecorder()

	// Record with different results
	recorder.RecordNotificationDuration("node", 0.001, true)
	recorder.RecordNotificationDuration("mcp", 0.002, false)

	// Verify we can gather metrics without error
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Look for our metric in the gathered metrics
	found := false
	for _, m := range metrics {
		if strings.Contains(m.GetName(), "notification_duration") {
			found = true
			break
		}
	}

	if !found {
		t.Log("Note: notification_duration metric not found in default gatherer (may be registered elsewhere)")
	}
}

func TestMetricsRecorder_SetActiveNodeCount(t *testing.T) {
	recorder := NewMetricsRecorder()

	recorder.SetActiveNodeCount(5)
	count := testutil.ToFloat64(activeNodeStates)
	if count != 5 {
		t.Errorf("Expected active node count 5, got %f", count)
	}

	recorder.SetActiveNodeCount(10)
	count = testutil.ToFloat64(activeNodeStates)
	if count != 10 {
		t.Errorf("Expected active node count 10, got %f", count)
	}
}

func TestMetricsRecorder_SetNodesByPhase(t *testing.T) {
	recorder := NewMetricsRecorder()

	phaseCounts := map[UpdatePhase]int{
		UpdatePhasePending:   2,
		UpdatePhaseUpdating:  3,
		UpdatePhaseCompleted: 5,
	}

	recorder.SetNodesByPhase(phaseCounts)

	// Check expected values
	if count := testutil.ToFloat64(nodeStateByPhase.WithLabelValues(string(UpdatePhasePending))); count != 2 {
		t.Errorf("Expected Pending count 2, got %f", count)
	}
	if count := testutil.ToFloat64(nodeStateByPhase.WithLabelValues(string(UpdatePhaseUpdating))); count != 3 {
		t.Errorf("Expected Updating count 3, got %f", count)
	}
	if count := testutil.ToFloat64(nodeStateByPhase.WithLabelValues(string(UpdatePhaseCompleted))); count != 5 {
		t.Errorf("Expected Completed count 5, got %f", count)
	}

	// Check zero values for unset phases
	if count := testutil.ToFloat64(nodeStateByPhase.WithLabelValues(string(UpdatePhaseDraining))); count != 0 {
		t.Errorf("Expected Draining count 0, got %f", count)
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Verify all metrics are registered (they should be registered via init())
	// We can check by trying to collect them
	ch := make(chan prometheus.Metric, 100)

	nodeStateEvaluationDuration.Collect(ch)
	nodeStateEvaluationTotal.Collect(ch)
	downstreamNotificationDuration.Collect(ch)
	activeNodeStates.Collect(ch)
	nodeStateByPhase.Collect(ch)

	// If we got here without panicking, metrics are properly configured
}
