package verifier

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// VerifierMetrics contains all Prometheus metrics for the verifier
type VerifierMetrics struct {
	// Verification metrics
	VerificationTotal        *prometheus.CounterVec
	VerificationDuration     *prometheus.HistogramVec
	VerificationErrors       *prometheus.CounterVec
	VerifiedModels           prometheus.Gauge
	CodeVisibilityTests      *prometheus.CounterVec

	// Scoring metrics
	ScoreCalculations        *prometheus.CounterVec
	ScoreCacheHits           prometheus.Counter
	ScoreCacheMisses         prometheus.Counter
	ModelScores              *prometheus.GaugeVec
	ScoringDuration          *prometheus.HistogramVec

	// Health metrics
	ProviderHealthChecks     *prometheus.CounterVec
	ProviderHealthStatus     *prometheus.GaugeVec
	CircuitBreakerState      *prometheus.GaugeVec
	ProviderLatency          *prometheus.HistogramVec
	FailoverAttempts         *prometheus.CounterVec
	FailoverSuccess          *prometheus.CounterVec

	// Provider metrics
	ProviderRequests         *prometheus.CounterVec
	ProviderErrors           *prometheus.CounterVec
	ProviderResponseTime     *prometheus.HistogramVec
	ActiveProviders          prometheus.Gauge

	// Database metrics
	DatabaseOperations       *prometheus.CounterVec
	DatabaseErrors           *prometheus.CounterVec
	DatabaseLatency          *prometheus.HistogramVec
	SyncOperations           *prometheus.CounterVec
}

// NewVerifierMetrics creates new verifier metrics
func NewVerifierMetrics(namespace string) *VerifierMetrics {
	if namespace == "" {
		namespace = "superagent_verifier"
	}

	return &VerifierMetrics{
		// Verification metrics
		VerificationTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "verifications_total",
				Help:      "Total number of model verifications",
			},
			[]string{"provider", "model", "result"},
		),
		VerificationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "verification_duration_seconds",
				Help:      "Duration of model verifications",
				Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"provider", "model"},
		),
		VerificationErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "verification_errors_total",
				Help:      "Total number of verification errors",
			},
			[]string{"provider", "model", "error_type"},
		),
		VerifiedModels: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "verified_models",
				Help:      "Number of currently verified models",
			},
		),
		CodeVisibilityTests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "code_visibility_tests_total",
				Help:      "Total code visibility tests performed",
			},
			[]string{"provider", "model", "language", "result"},
		),

		// Scoring metrics
		ScoreCalculations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "score_calculations_total",
				Help:      "Total number of score calculations",
			},
			[]string{"model", "data_source"},
		),
		ScoreCacheHits: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "score_cache_hits_total",
				Help:      "Total score cache hits",
			},
		),
		ScoreCacheMisses: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "score_cache_misses_total",
				Help:      "Total score cache misses",
			},
		),
		ModelScores: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "model_score",
				Help:      "Current score for each model",
			},
			[]string{"model", "provider", "component"},
		),
		ScoringDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "scoring_duration_seconds",
				Help:      "Duration of score calculations",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1, 2},
			},
			[]string{"model"},
		),

		// Health metrics
		ProviderHealthChecks: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "health_checks_total",
				Help:      "Total provider health checks",
			},
			[]string{"provider", "result"},
		),
		ProviderHealthStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "provider_healthy",
				Help:      "Provider health status (1=healthy, 0=unhealthy)",
			},
			[]string{"provider"},
		),
		CircuitBreakerState: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "circuit_breaker_state",
				Help:      "Circuit breaker state (0=closed, 1=half-open, 2=open)",
			},
			[]string{"provider"},
		),
		ProviderLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "provider_latency_seconds",
				Help:      "Provider response latency",
				Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
			},
			[]string{"provider"},
		),
		FailoverAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "failover_attempts_total",
				Help:      "Total failover attempts",
			},
			[]string{"from_provider", "to_provider"},
		),
		FailoverSuccess: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "failover_success_total",
				Help:      "Total successful failovers",
			},
			[]string{"from_provider", "to_provider"},
		),

		// Provider metrics
		ProviderRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "provider_requests_total",
				Help:      "Total provider requests",
			},
			[]string{"provider", "model", "operation"},
		),
		ProviderErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "provider_errors_total",
				Help:      "Total provider errors",
			},
			[]string{"provider", "model", "error_type"},
		),
		ProviderResponseTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "provider_response_time_seconds",
				Help:      "Provider response times",
				Buckets:   []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
			},
			[]string{"provider", "model"},
		),
		ActiveProviders: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_providers",
				Help:      "Number of active providers",
			},
		),

		// Database metrics
		DatabaseOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "database_operations_total",
				Help:      "Total database operations",
			},
			[]string{"operation", "table"},
		),
		DatabaseErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "database_errors_total",
				Help:      "Total database errors",
			},
			[]string{"operation", "table", "error_type"},
		),
		DatabaseLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "database_latency_seconds",
				Help:      "Database operation latency",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
			},
			[]string{"operation", "table"},
		),
		SyncOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sync_operations_total",
				Help:      "Total database sync operations",
			},
			[]string{"direction", "table", "result"},
		),
	}
}

// RecordVerification records a verification event
func (m *VerifierMetrics) RecordVerification(provider, model string, success bool, duration float64) {
	result := "success"
	if !success {
		result = "failure"
	}
	m.VerificationTotal.WithLabelValues(provider, model, result).Inc()
	m.VerificationDuration.WithLabelValues(provider, model).Observe(duration)
}

// RecordVerificationError records a verification error
func (m *VerifierMetrics) RecordVerificationError(provider, model, errorType string) {
	m.VerificationErrors.WithLabelValues(provider, model, errorType).Inc()
}

// SetVerifiedModelsCount sets the verified models gauge
func (m *VerifierMetrics) SetVerifiedModelsCount(count int) {
	m.VerifiedModels.Set(float64(count))
}

// RecordCodeVisibilityTest records a code visibility test
func (m *VerifierMetrics) RecordCodeVisibilityTest(provider, model, language string, visible bool) {
	result := "visible"
	if !visible {
		result = "not_visible"
	}
	m.CodeVisibilityTests.WithLabelValues(provider, model, language, result).Inc()
}

// RecordScoreCalculation records a score calculation
func (m *VerifierMetrics) RecordScoreCalculation(model, dataSource string, duration float64) {
	m.ScoreCalculations.WithLabelValues(model, dataSource).Inc()
	m.ScoringDuration.WithLabelValues(model).Observe(duration)
}

// RecordScoreCacheHit records a score cache hit
func (m *VerifierMetrics) RecordScoreCacheHit() {
	m.ScoreCacheHits.Inc()
}

// RecordScoreCacheMiss records a score cache miss
func (m *VerifierMetrics) RecordScoreCacheMiss() {
	m.ScoreCacheMisses.Inc()
}

// SetModelScore sets the score for a model
func (m *VerifierMetrics) SetModelScore(model, provider string, overall, speed, efficiency, cost, capability, recency float64) {
	m.ModelScores.WithLabelValues(model, provider, "overall").Set(overall)
	m.ModelScores.WithLabelValues(model, provider, "speed").Set(speed)
	m.ModelScores.WithLabelValues(model, provider, "efficiency").Set(efficiency)
	m.ModelScores.WithLabelValues(model, provider, "cost").Set(cost)
	m.ModelScores.WithLabelValues(model, provider, "capability").Set(capability)
	m.ModelScores.WithLabelValues(model, provider, "recency").Set(recency)
}

// RecordHealthCheck records a health check
func (m *VerifierMetrics) RecordHealthCheck(provider string, healthy bool, latency float64) {
	result := "success"
	if !healthy {
		result = "failure"
	}
	m.ProviderHealthChecks.WithLabelValues(provider, result).Inc()
	m.ProviderLatency.WithLabelValues(provider).Observe(latency)

	healthyValue := 0.0
	if healthy {
		healthyValue = 1.0
	}
	m.ProviderHealthStatus.WithLabelValues(provider).Set(healthyValue)
}

// SetCircuitBreakerState sets the circuit breaker state
func (m *VerifierMetrics) SetCircuitBreakerState(provider string, state int) {
	m.CircuitBreakerState.WithLabelValues(provider).Set(float64(state))
}

// RecordFailover records a failover attempt
func (m *VerifierMetrics) RecordFailover(fromProvider, toProvider string, success bool) {
	m.FailoverAttempts.WithLabelValues(fromProvider, toProvider).Inc()
	if success {
		m.FailoverSuccess.WithLabelValues(fromProvider, toProvider).Inc()
	}
}

// RecordProviderRequest records a provider request
func (m *VerifierMetrics) RecordProviderRequest(provider, model, operation string, duration float64, err error) {
	m.ProviderRequests.WithLabelValues(provider, model, operation).Inc()
	m.ProviderResponseTime.WithLabelValues(provider, model).Observe(duration)

	if err != nil {
		m.ProviderErrors.WithLabelValues(provider, model, "request_error").Inc()
	}
}

// SetActiveProviders sets the active providers gauge
func (m *VerifierMetrics) SetActiveProviders(count int) {
	m.ActiveProviders.Set(float64(count))
}

// RecordDatabaseOperation records a database operation
func (m *VerifierMetrics) RecordDatabaseOperation(operation, table string, duration float64, err error) {
	m.DatabaseOperations.WithLabelValues(operation, table).Inc()
	m.DatabaseLatency.WithLabelValues(operation, table).Observe(duration)

	if err != nil {
		m.DatabaseErrors.WithLabelValues(operation, table, "operation_error").Inc()
	}
}

// RecordSync records a sync operation
func (m *VerifierMetrics) RecordSync(direction, table string, success bool) {
	result := "success"
	if !success {
		result = "failure"
	}
	m.SyncOperations.WithLabelValues(direction, table, result).Inc()
}

// DefaultMetrics is the default metrics instance
var DefaultMetrics = NewVerifierMetrics("")
