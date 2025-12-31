package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAnalyticsTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewProtocolAnalyticsService(t *testing.T) {
	log := newAnalyticsTestLogger()

	t.Run("with default config", func(t *testing.T) {
		service := NewProtocolAnalyticsService(nil, log)

		require.NotNil(t, service)
		assert.NotNil(t, service.metrics)
		assert.NotNil(t, service.performanceData)
		assert.NotNil(t, service.usagePatterns)
		assert.Equal(t, 1*time.Hour, service.collectionWindow)
		assert.Equal(t, 30*24*time.Hour, service.retentionPeriod)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &AnalyticsConfig{
			CollectionWindow:  30 * time.Minute,
			RetentionPeriod:   7 * 24 * time.Hour,
			MaxMetricsHistory: 500,
			SamplingRate:      0.5,
			EnableRealTime:    false,
		}

		service := NewProtocolAnalyticsService(config, log)

		require.NotNil(t, service)
		assert.Equal(t, 30*time.Minute, service.collectionWindow)
		assert.Equal(t, 7*24*time.Hour, service.retentionPeriod)
	})
}

func TestProtocolAnalyticsService_RecordRequest(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	t.Run("record successful request", func(t *testing.T) {
		err := service.RecordRequest(ctx, "mcp", "execute", 100*time.Millisecond, true, "")
		require.NoError(t, err)

		metrics, err := service.GetProtocolMetrics("mcp")
		require.NoError(t, err)
		assert.Equal(t, int64(1), metrics.TotalRequests)
		assert.Equal(t, int64(1), metrics.SuccessfulRequests)
		assert.Equal(t, int64(0), metrics.FailedRequests)
		assert.Equal(t, float64(0), metrics.ErrorRate)
	})

	t.Run("record failed request", func(t *testing.T) {
		err := service.RecordRequest(ctx, "lsp", "completion", 200*time.Millisecond, false, "timeout")
		require.NoError(t, err)

		metrics, err := service.GetProtocolMetrics("lsp")
		require.NoError(t, err)
		assert.Equal(t, int64(1), metrics.TotalRequests)
		assert.Equal(t, int64(0), metrics.SuccessfulRequests)
		assert.Equal(t, int64(1), metrics.FailedRequests)
		assert.Equal(t, float64(1), metrics.ErrorRate)
	})

	t.Run("updates latency statistics", func(t *testing.T) {
		service.RecordRequest(ctx, "acp", "method1", 50*time.Millisecond, true, "")
		service.RecordRequest(ctx, "acp", "method2", 150*time.Millisecond, true, "")
		service.RecordRequest(ctx, "acp", "method3", 100*time.Millisecond, true, "")

		metrics, err := service.GetProtocolMetrics("acp")
		require.NoError(t, err)
		assert.Equal(t, int64(3), metrics.TotalRequests)
		assert.Equal(t, 50*time.Millisecond, metrics.MinLatency)
		assert.Equal(t, 150*time.Millisecond, metrics.PeakLatency)
		assert.True(t, metrics.AverageLatency > 0)
	})

	t.Run("updates usage patterns", func(t *testing.T) {
		err := service.RecordRequest(ctx, "test-protocol", "method", 100*time.Millisecond, true, "")
		require.NoError(t, err)

		pattern, err := service.GetUsagePatterns("test-protocol")
		require.NoError(t, err)

		// At least one hour should have a count
		totalHourly := int64(0)
		for _, count := range pattern.HourlyUsage {
			totalHourly += count
		}
		assert.True(t, totalHourly > 0)
	})
}

func TestProtocolAnalyticsService_GetProtocolMetrics(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	t.Run("get existing metrics", func(t *testing.T) {
		service.RecordRequest(ctx, "mcp", "execute", 100*time.Millisecond, true, "")

		metrics, err := service.GetProtocolMetrics("mcp")
		require.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Equal(t, "mcp", metrics.Protocol)
	})

	t.Run("get non-existent metrics", func(t *testing.T) {
		metrics, err := service.GetProtocolMetrics("non-existent")
		assert.Error(t, err)
		assert.Nil(t, metrics)
		assert.Contains(t, err.Error(), "no metrics found")
	})
}

func TestProtocolAnalyticsService_GetAllProtocolMetrics(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	service.RecordRequest(ctx, "mcp", "execute", 100*time.Millisecond, true, "")
	service.RecordRequest(ctx, "lsp", "completion", 200*time.Millisecond, true, "")
	service.RecordRequest(ctx, "acp", "call", 150*time.Millisecond, true, "")

	allMetrics := service.GetAllProtocolMetrics()

	assert.Len(t, allMetrics, 3)
	assert.Contains(t, allMetrics, "mcp")
	assert.Contains(t, allMetrics, "lsp")
	assert.Contains(t, allMetrics, "acp")
}

func TestProtocolAnalyticsService_GetPerformanceStats(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	t.Run("get existing stats", func(t *testing.T) {
		service.RecordRequest(ctx, "mcp", "method1", 100*time.Millisecond, true, "")
		service.RecordRequest(ctx, "mcp", "method2", 200*time.Millisecond, false, "error1")

		stats, err := service.GetPerformanceStats("mcp")
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, "mcp", stats.Protocol)
		assert.Len(t, stats.ResponseTimes, 2)
		assert.Contains(t, stats.SuccessCounts, "method1")
		assert.Contains(t, stats.ErrorCounts, "error1")
	})

	t.Run("get non-existent stats", func(t *testing.T) {
		stats, err := service.GetPerformanceStats("non-existent")
		assert.Error(t, err)
		assert.Nil(t, stats)
	})
}

func TestProtocolAnalyticsService_GetUsagePatterns(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	t.Run("get existing patterns", func(t *testing.T) {
		service.RecordRequest(ctx, "mcp", "execute", 100*time.Millisecond, true, "")

		patterns, err := service.GetUsagePatterns("mcp")
		require.NoError(t, err)
		assert.NotNil(t, patterns)
		assert.Equal(t, "mcp", patterns.Protocol)
	})

	t.Run("get non-existent patterns", func(t *testing.T) {
		patterns, err := service.GetUsagePatterns("non-existent")
		assert.Error(t, err)
		assert.Nil(t, patterns)
	})
}

func TestProtocolAnalyticsService_AnalyzePerformance(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	t.Run("healthy protocol", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			service.RecordRequest(ctx, "healthy", "method", 100*time.Millisecond, true, "")
		}

		analysis, err := service.AnalyzePerformance("healthy")
		require.NoError(t, err)
		assert.NotNil(t, analysis)
		assert.Equal(t, "healthy", analysis.OverallHealth)
		assert.Equal(t, 100, analysis.OptimizationScore)
		assert.Empty(t, analysis.Bottlenecks)
	})

	t.Run("protocol with high error rate", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			service.RecordRequest(ctx, "error-prone", "method", 100*time.Millisecond, false, "error")
		}
		for i := 0; i < 50; i++ {
			service.RecordRequest(ctx, "error-prone", "method", 100*time.Millisecond, true, "")
		}

		analysis, err := service.AnalyzePerformance("error-prone")
		require.NoError(t, err)
		assert.Contains(t, analysis.Bottlenecks, "High error rate")
		assert.True(t, analysis.OptimizationScore < 100)
	})

	t.Run("protocol with high latency", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			service.RecordRequest(ctx, "slow", "method", 10*time.Second, true, "")
		}

		analysis, err := service.AnalyzePerformance("slow")
		require.NoError(t, err)
		assert.Contains(t, analysis.Bottlenecks, "High average latency")
	})

	t.Run("protocol with extreme latency spikes", func(t *testing.T) {
		service.RecordRequest(ctx, "spiky", "method", 45*time.Second, true, "")

		analysis, err := service.AnalyzePerformance("spiky")
		require.NoError(t, err)
		assert.Contains(t, analysis.Bottlenecks, "Extreme latency spikes")
		assert.Equal(t, "critical", analysis.OverallHealth)
	})

	t.Run("non-existent protocol", func(t *testing.T) {
		analysis, err := service.AnalyzePerformance("non-existent")
		assert.Error(t, err)
		assert.Nil(t, analysis)
	})
}

func TestProtocolAnalyticsService_GetTopProtocols(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	// Record different request counts
	for i := 0; i < 100; i++ {
		service.RecordRequest(ctx, "high-traffic", "method", 100*time.Millisecond, true, "")
	}
	for i := 0; i < 50; i++ {
		service.RecordRequest(ctx, "medium-traffic", "method", 100*time.Millisecond, true, "")
	}
	for i := 0; i < 10; i++ {
		service.RecordRequest(ctx, "low-traffic", "method", 100*time.Millisecond, true, "")
	}

	t.Run("get top 2 protocols", func(t *testing.T) {
		top := service.GetTopProtocols(2)
		assert.Len(t, top, 2)
		assert.Equal(t, "high-traffic", top[0].Protocol)
		assert.Equal(t, "medium-traffic", top[1].Protocol)
	})

	t.Run("get more protocols than exist", func(t *testing.T) {
		top := service.GetTopProtocols(10)
		assert.Len(t, top, 3)
	})
}

func TestProtocolAnalyticsService_GenerateUsageReport(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	service.RecordRequest(ctx, "mcp", "execute", 100*time.Millisecond, true, "")
	service.RecordRequest(ctx, "lsp", "completion", 200*time.Millisecond, false, "error")

	report := service.GenerateUsageReport()

	assert.NotNil(t, report)
	assert.False(t, report.GeneratedAt.IsZero())
	assert.Equal(t, 2, report.TotalProtocols)
	assert.NotNil(t, report.ProtocolMetrics)
	assert.NotNil(t, report.TopProtocols)
	assert.NotNil(t, report.Summary)
	assert.Equal(t, int64(2), report.Summary.TotalRequests)
	assert.Equal(t, int64(1), report.Summary.TotalErrors)
}

func TestProtocolAnalyticsService_UpdateConnectionCount(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	t.Run("update existing protocol", func(t *testing.T) {
		service.RecordRequest(ctx, "mcp", "execute", 100*time.Millisecond, true, "")
		service.UpdateConnectionCount("mcp", 10)

		metrics, _ := service.GetProtocolMetrics("mcp")
		assert.Equal(t, int32(10), metrics.ActiveConnections)
	})

	t.Run("update non-existent protocol", func(t *testing.T) {
		// Should not panic or create new entry
		service.UpdateConnectionCount("non-existent", 5)

		_, err := service.GetProtocolMetrics("non-existent")
		assert.Error(t, err) // Should still not exist
	})
}

func TestProtocolAnalyticsService_CleanOldData(t *testing.T) {
	log := newAnalyticsTestLogger()
	config := &AnalyticsConfig{
		CollectionWindow:  1 * time.Hour,
		RetentionPeriod:   1 * time.Millisecond, // Very short for testing
		MaxMetricsHistory: 1000,
		SamplingRate:      1.0,
		EnableRealTime:    true,
	}
	service := NewProtocolAnalyticsService(config, log)
	ctx := context.Background()

	service.RecordRequest(ctx, "old-protocol", "method", 100*time.Millisecond, true, "")

	// Wait for retention period to expire
	time.Sleep(5 * time.Millisecond)

	err := service.CleanOldData()
	require.NoError(t, err)

	// Protocol should be cleaned up
	_, err = service.GetProtocolMetrics("old-protocol")
	assert.Error(t, err)
}

func TestProtocolAnalyticsService_GetHealthStatus(t *testing.T) {
	log := newAnalyticsTestLogger()
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	t.Run("healthy status", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			service.RecordRequest(ctx, "healthy-protocol", "method", 100*time.Millisecond, true, "")
		}

		status := service.GetHealthStatus()
		assert.NotNil(t, status)
		assert.Equal(t, "healthy", status.OverallStatus)
		assert.Contains(t, status.ProtocolStatuses, "healthy-protocol")
		assert.False(t, status.LastUpdated.IsZero())
	})

	t.Run("degraded status with high error rate", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			service.RecordRequest(ctx, "error-protocol", "method", 100*time.Millisecond, false, "error")
		}
		for i := 0; i < 50; i++ {
			service.RecordRequest(ctx, "error-protocol", "method", 100*time.Millisecond, true, "")
		}

		status := service.GetHealthStatus()
		assert.NotNil(t, status)
		assert.Equal(t, "degraded", status.ProtocolStatuses["error-protocol"])
		assert.True(t, len(status.Alerts) > 0)
	})
}

func TestAnalyticsConfig_Structure(t *testing.T) {
	config := &AnalyticsConfig{
		CollectionWindow:  30 * time.Minute,
		RetentionPeriod:   7 * 24 * time.Hour,
		MaxMetricsHistory: 500,
		SamplingRate:      0.5,
		EnableRealTime:    true,
	}

	assert.Equal(t, 30*time.Minute, config.CollectionWindow)
	assert.Equal(t, 7*24*time.Hour, config.RetentionPeriod)
	assert.Equal(t, 500, config.MaxMetricsHistory)
	assert.Equal(t, 0.5, config.SamplingRate)
	assert.True(t, config.EnableRealTime)
}

func TestAnalyticsProtocolMetrics_Structure(t *testing.T) {
	now := time.Now()
	metrics := &AnalyticsProtocolMetrics{
		Protocol:           "test",
		TotalRequests:      100,
		SuccessfulRequests: 90,
		FailedRequests:     10,
		AverageLatency:     100 * time.Millisecond,
		PeakLatency:        500 * time.Millisecond,
		MinLatency:         50 * time.Millisecond,
		ErrorRate:          0.1,
		Throughput:         5.0,
		LastUsed:           now,
		ActiveConnections:  5,
	}

	assert.Equal(t, "test", metrics.Protocol)
	assert.Equal(t, int64(100), metrics.TotalRequests)
	assert.Equal(t, 0.1, metrics.ErrorRate)
}

func TestPerformanceAnalysis_Structure(t *testing.T) {
	analysis := &PerformanceAnalysis{
		Protocol:          "test",
		OverallHealth:     "degraded",
		Bottlenecks:       []string{"High latency", "Error rate"},
		Recommendations:   []string{"Add caching", "Fix errors"},
		OptimizationScore: 75,
	}

	assert.Equal(t, "test", analysis.Protocol)
	assert.Equal(t, "degraded", analysis.OverallHealth)
	assert.Len(t, analysis.Bottlenecks, 2)
	assert.Len(t, analysis.Recommendations, 2)
	assert.Equal(t, 75, analysis.OptimizationScore)
}

func TestUsageReport_Structure(t *testing.T) {
	now := time.Now()
	report := &UsageReport{
		GeneratedAt:     now,
		TotalProtocols:  3,
		ProtocolMetrics: map[string]*AnalyticsProtocolMetrics{},
		TopProtocols:    []*AnalyticsProtocolMetrics{},
		Summary: &UsageSummary{
			TotalRequests:  100,
			TotalErrors:    10,
			AverageLatency: 150 * time.Millisecond,
			ErrorRate:      0.1,
		},
	}

	assert.Equal(t, 3, report.TotalProtocols)
	assert.NotNil(t, report.Summary)
	assert.Equal(t, int64(100), report.Summary.TotalRequests)
}

func TestAnalyticsHealthStatus_Structure(t *testing.T) {
	now := time.Now()
	status := &AnalyticsHealthStatus{
		OverallStatus: "degraded",
		ProtocolStatuses: map[string]string{
			"mcp": "healthy",
			"lsp": "critical",
		},
		Alerts:      []string{"High error rate", "High latency"},
		LastUpdated: now,
	}

	assert.Equal(t, "degraded", status.OverallStatus)
	assert.Len(t, status.ProtocolStatuses, 2)
	assert.Len(t, status.Alerts, 2)
}

func BenchmarkProtocolAnalyticsService_RecordRequest(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.RecordRequest(ctx, "bench-protocol", "method", 100*time.Millisecond, true, "")
	}
}

func BenchmarkProtocolAnalyticsService_GetProtocolMetrics(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	service.RecordRequest(ctx, "bench-protocol", "method", 100*time.Millisecond, true, "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetProtocolMetrics("bench-protocol")
	}
}

func BenchmarkProtocolAnalyticsService_GenerateUsageReport(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	service := NewProtocolAnalyticsService(nil, log)
	ctx := context.Background()

	// Add some data
	for i := 0; i < 10; i++ {
		protocol := "protocol-" + string(rune('a'+i))
		for j := 0; j < 100; j++ {
			service.RecordRequest(ctx, protocol, "method", 100*time.Millisecond, true, "")
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.GenerateUsageReport()
	}
}
