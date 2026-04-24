package monitoring_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonitoring_DatabaseQueryDuration validates that database query durations
// are tracked per query type and table using histogram metrics with
// appropriate bucket boundaries.
func TestMonitoring_DatabaseQueryDuration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	queryDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "db_query_duration_seconds",
		Help: "Database query duration in seconds",
		Buckets: []float64{
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0,
		},
	}, []string{"query_type", "table"})
	registry.MustRegister(queryDuration)

	// Simulate queries
	queryDuration.WithLabelValues("SELECT", "users").Observe(0.003)
	queryDuration.WithLabelValues("SELECT", "sessions").Observe(0.008)
	queryDuration.WithLabelValues("INSERT", "llm_requests").Observe(0.012)
	queryDuration.WithLabelValues("UPDATE", "llm_responses").Observe(0.005)

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	queryCount := 0
	for _, mf := range metricFamilies {
		if mf.GetName() == "db_query_duration_seconds" {
			for _, m := range mf.GetMetric() {
				queryCount++
				hist := m.GetHistogram()
				assert.Equal(t, uint64(1), hist.GetSampleCount(),
					"each query type/table combination should have 1 sample")
			}
		}
	}
	assert.Equal(t, 4, queryCount,
		"should track 4 different query types")
}

// TestMonitoring_DatabaseConnectionPool validates that database connection
// pool metrics (active, idle, max) are correctly tracked and consistent.
func TestMonitoring_DatabaseConnectionPool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	activeConns := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_active",
		Help: "Number of active database connections",
	})
	registry.MustRegister(activeConns)

	idleConns := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_idle",
		Help: "Number of idle database connections",
	})
	registry.MustRegister(idleConns)

	maxConns := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_max",
		Help: "Maximum number of database connections",
	})
	registry.MustRegister(maxConns)

	// Simulate pool state
	maxConns.Set(20)
	activeConns.Set(8)
	idleConns.Set(12)

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	var active, idle, max float64
	for _, mf := range metricFamilies {
		switch mf.GetName() {
		case "db_connections_active":
			active = mf.GetMetric()[0].GetGauge().GetValue()
		case "db_connections_idle":
			idle = mf.GetMetric()[0].GetGauge().GetValue()
		case "db_connections_max":
			max = mf.GetMetric()[0].GetGauge().GetValue()
		}
	}

	assert.Equal(t, max, active+idle,
		"active + idle should equal max")
}

// TestMonitoring_DatabaseSlowQueries validates that slow queries are tracked
// separately with a counter and that the threshold-based classification works.
func TestMonitoring_DatabaseSlowQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	slowQueryCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "db_slow_queries_total",
		Help: "Total number of slow queries (> threshold)",
	}, []string{"query_type"})
	registry.MustRegister(slowQueryCount)

	queryDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "db_query_latency_seconds",
		Help: "Query latency distribution",
		Buckets: []float64{
			0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0,
		},
	}, []string{"query_type"})
	registry.MustRegister(queryDuration)

	// Simulate queries with a slow query threshold of 100ms
	const slowThreshold = 0.1

	queryLatencies := map[string][]float64{
		"SELECT": {0.002, 0.005, 0.150, 0.003, 0.200, 0.008},
		"INSERT": {0.010, 0.500, 0.012, 0.008},
		"UPDATE": {0.003, 0.004, 0.002},
	}

	for queryType, latencies := range queryLatencies {
		for _, lat := range latencies {
			queryDuration.WithLabelValues(queryType).Observe(lat)
			if lat > slowThreshold {
				slowQueryCount.WithLabelValues(queryType).Inc()
			}
		}
	}

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	slowCounts := make(map[string]float64)
	for _, mf := range metricFamilies {
		if mf.GetName() == "db_slow_queries_total" {
			for _, m := range mf.GetMetric() {
				for _, label := range m.GetLabel() {
					if label.GetName() == "query_type" {
						slowCounts[label.GetValue()] =
							m.GetCounter().GetValue()
					}
				}
			}
		}
	}

	assert.Equal(t, 2.0, slowCounts["SELECT"],
		"SELECT should have 2 slow queries (0.150, 0.200)")
	assert.Equal(t, 1.0, slowCounts["INSERT"],
		"INSERT should have 1 slow query (0.500)")
	_, hasUpdate := slowCounts["UPDATE"]
	assert.False(t, hasUpdate,
		"UPDATE should have no slow queries")
}

// TestMonitoring_DatabaseTransactionMetrics validates that database
// transaction commit/rollback counters and duration histograms work correctly.
func TestMonitoring_DatabaseTransactionMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	txOutcome := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "db_transactions_total",
		Help: "Total database transactions by outcome",
	}, []string{"outcome"})
	registry.MustRegister(txOutcome)

	txDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "db_transaction_duration_seconds",
		Help: "Transaction duration in seconds",
		Buckets: []float64{
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
		},
	})
	registry.MustRegister(txDuration)

	// Simulate 95 committed and 5 rolled-back transactions
	for i := 0; i < 100; i++ {
		duration := 0.005 + float64(i%20)*0.002 // 5-43ms
		txDuration.Observe(duration)
		if i%20 == 0 {
			txOutcome.WithLabelValues("rollback").Inc()
		} else {
			txOutcome.WithLabelValues("commit").Inc()
		}
	}

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	outcomes := make(map[string]float64)
	var totalDurationSamples uint64

	for _, mf := range metricFamilies {
		switch mf.GetName() {
		case "db_transactions_total":
			for _, m := range mf.GetMetric() {
				for _, label := range m.GetLabel() {
					if label.GetName() == "outcome" {
						outcomes[label.GetValue()] =
							m.GetCounter().GetValue()
					}
				}
			}
		case "db_transaction_duration_seconds":
			for _, m := range mf.GetMetric() {
				totalDurationSamples = m.GetHistogram().GetSampleCount()
			}
		}
	}

	assert.Equal(t, 95.0, outcomes["commit"],
		"should have 95 committed transactions")
	assert.Equal(t, 5.0, outcomes["rollback"],
		"should have 5 rolled-back transactions")
	assert.Equal(t, outcomes["commit"]+outcomes["rollback"],
		float64(totalDurationSamples),
		"duration samples should match total transactions")
}
