package optimization

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMetrics(t *testing.T) {
	// GetMetrics should return the singleton instance
	metrics1 := GetMetrics()
	require.NotNil(t, metrics1)

	metrics2 := GetMetrics()
	require.NotNil(t, metrics2)

	// Should be the same instance (singleton)
	assert.Equal(t, metrics1, metrics2)
}

func TestMetrics_RecordCacheHit(t *testing.T) {
	metrics := GetMetrics()
	require.NotNil(t, metrics)

	// Record a cache hit with timing
	duration := 50 * time.Millisecond
	metrics.RecordCacheHit(duration)

	// We can't directly read counter values, but we can verify no panic
	// and that the function completes successfully
}

func TestMetrics_RecordCacheMiss(t *testing.T) {
	metrics := GetMetrics()
	require.NotNil(t, metrics)

	// Record a cache miss with timing
	duration := 100 * time.Millisecond
	metrics.RecordCacheMiss(duration)
}

func TestMetrics_RecordValidation(t *testing.T) {
	metrics := GetMetrics()
	require.NotNil(t, metrics)

	tests := []struct {
		name     string
		success  bool
		duration time.Duration
	}{
		{
			name:     "successful validation",
			success:  true,
			duration: 10 * time.Millisecond,
		},
		{
			name:     "failed validation",
			success:  false,
			duration: 5 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics.RecordValidation(tt.success, tt.duration)
		})
	}
}

func TestMetrics_RecordStreamComplete(t *testing.T) {
	metrics := GetMetrics()
	require.NotNil(t, metrics)

	tests := []struct {
		name     string
		duration time.Duration
		tokens   int
	}{
		{
			name:     "normal stream",
			duration: 5 * time.Second,
			tokens:   500,
		},
		{
			name:     "fast stream",
			duration: 1 * time.Second,
			tokens:   100,
		},
		{
			name:     "zero duration",
			duration: 0,
			tokens:   50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics.RecordStreamComplete(tt.duration, tt.tokens)
		})
	}
}

func TestMetrics_RecordServiceCall(t *testing.T) {
	metrics := GetMetrics()
	require.NotNil(t, metrics)

	tests := []struct {
		name     string
		service  string
		method   string
		duration time.Duration
		err      error
	}{
		{
			name:     "successful call to llamaindex",
			service:  "llamaindex",
			method:   "query",
			duration: 200 * time.Millisecond,
			err:      nil,
		},
		{
			name:     "failed call to langchain",
			service:  "langchain",
			method:   "decompose",
			duration: 500 * time.Millisecond,
			err:      assert.AnError,
		},
		{
			name:     "successful call to sglang",
			service:  "sglang",
			method:   "generate",
			duration: 1 * time.Second,
			err:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics.RecordServiceCall(tt.service, tt.method, tt.duration, tt.err)
		})
	}
}

func TestMetrics_SetServiceAvailable(t *testing.T) {
	metrics := GetMetrics()
	require.NotNil(t, metrics)

	tests := []struct {
		name      string
		service   string
		available bool
	}{
		{
			name:      "llamaindex available",
			service:   "llamaindex",
			available: true,
		},
		{
			name:      "langchain unavailable",
			service:   "langchain",
			available: false,
		},
		{
			name:      "sglang available",
			service:   "sglang",
			available: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics.SetServiceAvailable(tt.service, tt.available)
		})
	}
}

func TestMetrics_RecordOptimization(t *testing.T) {
	metrics := GetMetrics()
	require.NotNil(t, metrics)

	tests := []struct {
		name      string
		isRequest bool
		duration  time.Duration
	}{
		{
			name:      "request optimization",
			isRequest: true,
			duration:  100 * time.Millisecond,
		},
		{
			name:      "response optimization",
			isRequest: false,
			duration:  50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics.RecordOptimization(tt.isRequest, tt.duration)
		})
	}
}

func TestMetrics_GetCacheHitRate(t *testing.T) {
	metrics := GetMetrics()
	require.NotNil(t, metrics)

	// GetCacheHitRate returns 0 as noted in the implementation
	// because Prometheus counters don't support reading values directly
	rate := metrics.GetCacheHitRate()
	assert.Equal(t, float64(0), rate)
}

func TestMetricsSnapshot(t *testing.T) {
	// Test MetricsSnapshot struct
	snapshot := MetricsSnapshot{
		CacheHits:         100,
		CacheMisses:       20,
		CacheHitRate:      0.833,
		CacheSize:         1000,
		ValidationSuccess: 0.95,
		StreamsActive:     5,
		ServicesHealthy:   4,
	}

	assert.Equal(t, int64(100), snapshot.CacheHits)
	assert.Equal(t, int64(20), snapshot.CacheMisses)
	assert.InDelta(t, 0.833, snapshot.CacheHitRate, 0.001)
	assert.Equal(t, 1000, snapshot.CacheSize)
	assert.InDelta(t, 0.95, snapshot.ValidationSuccess, 0.001)
	assert.Equal(t, 5, snapshot.StreamsActive)
	assert.Equal(t, 4, snapshot.ServicesHealthy)
}
