package nodestate

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricsNamespace = "openshift_update_experience"
	metricsSubsystem = "central_node_state"

	// Metric result labels
	resultSuccess = "success"
	resultError   = "error"
	resultDropped = "dropped"
)

var (
	// nodeStateEvaluationDuration measures time spent evaluating node state
	nodeStateEvaluationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "evaluation_duration_seconds",
			Help:      "Duration of node state evaluation in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"node", "pool", "result"},
	)

	// nodeStateEvaluationTotal counts total state evaluations
	nodeStateEvaluationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "evaluation_total",
			Help:      "Total number of node state evaluations",
		},
		[]string{"pool", "result"},
	)

	// downstreamNotificationDuration measures time spent sending notifications
	downstreamNotificationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "notification_duration_seconds",
			Help:      "Duration of downstream notification in seconds",
			Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
		},
		[]string{"channel", "result"},
	)

	// activeNodeStates tracks the number of nodes currently tracked
	activeNodeStates = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "active_nodes",
			Help:      "Number of nodes currently tracked by the central controller",
		},
	)

	// nodeStateByPhase tracks nodes by their update phase
	nodeStateByPhase = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "nodes_by_phase",
			Help:      "Number of nodes in each update phase",
		},
		[]string{"phase"},
	)
)

func init() {
	// Register metrics with the controller-runtime metrics registry
	metrics.Registry.MustRegister(
		nodeStateEvaluationDuration,
		nodeStateEvaluationTotal,
		downstreamNotificationDuration,
		activeNodeStates,
		nodeStateByPhase,
	)
}

// MetricsRecorder provides methods to record metrics for the central controller.
type MetricsRecorder struct{}

// NewMetricsRecorder creates a new MetricsRecorder.
func NewMetricsRecorder() *MetricsRecorder {
	return &MetricsRecorder{}
}

// RecordEvaluationDuration records the duration of a node state evaluation.
func (m *MetricsRecorder) RecordEvaluationDuration(node, pool string, duration float64, success bool) {
	result := resultSuccess
	if !success {
		result = resultError
	}
	nodeStateEvaluationDuration.WithLabelValues(node, pool, result).Observe(duration)
}

// RecordEvaluationTotal increments the evaluation counter.
func (m *MetricsRecorder) RecordEvaluationTotal(pool string, success bool) {
	result := resultSuccess
	if !success {
		result = resultError
	}
	nodeStateEvaluationTotal.WithLabelValues(pool, result).Inc()
}

// RecordNotificationDuration records the duration of a downstream notification.
func (m *MetricsRecorder) RecordNotificationDuration(channel string, duration float64, success bool) {
	result := resultSuccess
	if !success {
		result = resultDropped
	}
	downstreamNotificationDuration.WithLabelValues(channel, result).Observe(duration)
}

// SetActiveNodeCount sets the current number of tracked nodes.
func (m *MetricsRecorder) SetActiveNodeCount(count int) {
	activeNodeStates.Set(float64(count))
}

// SetNodesByPhase sets the count of nodes in each phase.
func (m *MetricsRecorder) SetNodesByPhase(phaseCounts map[UpdatePhase]int) {
	// Reset all phase counts to 0 first
	for _, phase := range []UpdatePhase{
		UpdatePhasePending,
		UpdatePhaseDraining,
		UpdatePhaseUpdating,
		UpdatePhaseRebooting,
		UpdatePhasePaused,
		UpdatePhaseCompleted,
	} {
		nodeStateByPhase.WithLabelValues(string(phase)).Set(0)
	}

	// Set actual counts
	for phase, count := range phaseCounts {
		nodeStateByPhase.WithLabelValues(string(phase)).Set(float64(count))
	}
}
