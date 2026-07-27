package monitoring_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonitoring_ProviderLatency_Histogram validates that provider request
// latencies are correctly recorded in histogram buckets with appropriate
// sample counts and sums.
func TestMonitoring_ProviderLatency_Histogram(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	latencyHist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "provider_request_duration_seconds",
		Help:    "Request duration in seconds per provider",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
	}, []string{"provider", "method"})
	registry.MustRegister(latencyHist)

	// Simulate latency observations
	providers := map[string][]float64{
		"openai":    {0.15, 0.22, 0.18, 0.45, 0.12},
		"anthropic": {0.08, 0.11, 0.09, 0.15, 0.07},
		"deepseek":  {0.30, 0.55, 0.42, 0.38, 0.61},
	}

	for provider, latencies := range providers {
		for _, lat := range latencies {
			latencyHist.WithLabelValues(provider, "Complete").Observe(lat)
		}
	}

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	for _, mf := range metricFamilies {
		if mf.GetName() == "provider_request_duration_seconds" {
			for _, m := range mf.GetMetric() {
				hist := m.GetHistogram()
				assert.Greater(t, hist.GetSampleCount(), uint64(0),
					"histogram should have samples")
				assert.Greater(t, hist.GetSampleSum(), 0.0,
					"histogram sum should be positive")
			}
		}
	}
}

// TestMonitoring_ProviderLatency_Summary validates that provider response
// time summaries correctly compute quantiles (p50, p90, p99) from observed
// latency data.
func TestMonitoring_ProviderLatency_Summary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	latencySummary := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "provider_response_time_ms",
		Help:       "Response time in milliseconds",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"provider"})
	registry.MustRegister(latencySummary)

	// Simulate 100 observations
	for i := 0; i < 100; i++ {
		latency := float64(50 + i%100) // 50-149ms
		latencySummary.WithLabelValues("test-provider").Observe(latency)
	}

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	for _, mf := range metricFamilies {
		if mf.GetName() == "provider_response_time_ms" {
			for _, m := range mf.GetMetric() {
				summary := m.GetSummary()
				assert.Equal(t, uint64(100), summary.GetSampleCount())

				// Check p50 is reasonable (should be around 99)
				for _, q := range summary.GetQuantile() {
					if q.GetQuantile() == 0.5 {
						assert.InDelta(t, 99.0, q.GetValue(), 10.0,
							"p50 latency should be around 99ms")
					}
				}
			}
		}
	}

	_ = time.Now() // use time package
}

// TestMonitoring_ProviderLatency_BucketDistribution validates that histogram
// bucket counts are monotonically non-decreasing and that observations are
// correctly distributed across bucket boundaries.
func TestMonitoring_ProviderLatency_BucketDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	latencyHist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "provider_latency_bucket_distribution",
		Help:    "Latency distribution across buckets",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	})
	registry.MustRegister(latencyHist)

	// Observe values that should distribute across buckets
	observations := []float64{
		0.005, 0.008, // <= 0.01 bucket
		0.02, 0.03, // <= 0.05 bucket
		0.07, 0.09, // <= 0.1 bucket
		0.15, 0.20, // <= 0.25 bucket
		0.35, 0.45, // <= 0.5 bucket
		0.7, 0.9, // <= 1.0 bucket
		1.5, 2.0, // <= 2.5 bucket
		3.0, 4.0, // <= 5.0 bucket
	}

	for _, obs := range observations {
		latencyHist.Observe(obs)
	}

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	for _, mf := range metricFamilies {
		if mf.GetName() == "provider_latency_bucket_distribution" {
			for _, m := range mf.GetMetric() {
				hist := m.GetHistogram()
				require.NotNil(t, hist)

				assert.Equal(t, uint64(len(observations)),
					hist.GetSampleCount(),
					"total sample count must match observations")

				// Verify cumulative bucket counts are non-decreasing
				buckets := hist.GetBucket()
				var prevCount uint64
				for _, b := range buckets {
					assert.GreaterOrEqual(t, b.GetCumulativeCount(),
						prevCount,
						"bucket counts must be monotonically non-decreasing")
					prevCount = b.GetCumulativeCount()
				}

				// The last bucket should contain all observations
				if len(buckets) > 0 {
					lastBucket := buckets[len(buckets)-1]
					assert.Equal(t, uint64(len(observations)),
						lastBucket.GetCumulativeCount(),
						"last bucket should contain all observations")
				}
			}
		}
	}
}

// TestMonitoring_ProviderLatency_PerMethodTracking validates that latency
// is tracked independently per HTTP method (Complete vs CompleteStream).
func TestMonitoring_ProviderLatency_PerMethodTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	latencyHist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "provider_method_duration_seconds",
		Help:    "Request duration per provider and method",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider", "method"})
	registry.MustRegister(latencyHist)

	// Complete requests are typically faster
	for i := 0; i < 10; i++ {
		latencyHist.WithLabelValues("openai", "Complete").
			Observe(0.1 + float64(i)*0.01)
	}

	// Streaming requests typically take longer
	for i := 0; i < 10; i++ {
		latencyHist.WithLabelValues("openai", "CompleteStream").
			Observe(0.5 + float64(i)*0.05)
	}

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	methodCounts := make(map[string]uint64)
	methodSums := make(map[string]float64)

	for _, mf := range metricFamilies {
		if mf.GetName() == "provider_method_duration_seconds" {
			for _, m := range mf.GetMetric() {
				var method string
				for _, label := range m.GetLabel() {
					if label.GetName() == "method" {
						method = label.GetValue()
					}
				}
				hist := m.GetHistogram()
				methodCounts[method] = hist.GetSampleCount()
				methodSums[method] = hist.GetSampleSum()
			}
		}
	}

	assert.Equal(t, uint64(10), methodCounts["Complete"],
		"Complete method should have 10 samples")
	assert.Equal(t, uint64(10), methodCounts["CompleteStream"],
		"CompleteStream method should have 10 samples")

	// Streaming should have higher total latency
	assert.Greater(t, methodSums["CompleteStream"], methodSums["Complete"],
		"streaming method should have higher total latency")
}
