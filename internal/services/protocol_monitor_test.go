package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMonitorTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewProtocolMonitor(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	require.NotNil(t, monitor)
	assert.NotNil(t, monitor.metrics)
	assert.NotNil(t, monitor.alerts)
	assert.NotNil(t, monitor.alertChan)
	assert.NotNil(t, monitor.stopChan)
}

func TestProtocolMonitor_RecordRequest(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("record successful request", func(t *testing.T) {
		monitor.RecordRequest(context.Background(), "mcp", 100*time.Millisecond, true, "")

		metrics, err := monitor.GetMetrics("mcp")
		require.NoError(t, err)
		assert.Equal(t, int64(1), metrics.TotalRequests)
		assert.Equal(t, int64(1), metrics.SuccessfulRequests)
		assert.Equal(t, int64(0), metrics.FailedRequests)
		assert.Equal(t, float64(0), metrics.ErrorRate)
	})

	t.Run("record failed request", func(t *testing.T) {
		monitor.RecordRequest(context.Background(), "lsp", 200*time.Millisecond, false, "connection error")

		metrics, err := monitor.GetMetrics("lsp")
		require.NoError(t, err)
		assert.Equal(t, int64(1), metrics.TotalRequests)
		assert.Equal(t, int64(0), metrics.SuccessfulRequests)
		assert.Equal(t, int64(1), metrics.FailedRequests)
		assert.Equal(t, float64(1), metrics.ErrorRate)
	})

	t.Run("updates latency statistics", func(t *testing.T) {
		monitor.RecordRequest(context.Background(), "acp", 50*time.Millisecond, true, "")
		monitor.RecordRequest(context.Background(), "acp", 150*time.Millisecond, true, "")
		monitor.RecordRequest(context.Background(), "acp", 100*time.Millisecond, true, "")

		metrics, err := monitor.GetMetrics("acp")
		require.NoError(t, err)
		assert.Equal(t, int64(3), metrics.TotalRequests)
		assert.Equal(t, 50*time.Millisecond, metrics.MinLatency)
		assert.Equal(t, 150*time.Millisecond, metrics.MaxLatency)
		assert.True(t, metrics.AverageLatency > 0)
	})
}

func TestProtocolMonitor_UpdateConnections(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("update existing protocol connections", func(t *testing.T) {
		monitor.RecordRequest(context.Background(), "mcp", 100*time.Millisecond, true, "")
		monitor.UpdateConnections("mcp", 5)

		metrics, err := monitor.GetMetrics("mcp")
		require.NoError(t, err)
		assert.Equal(t, 5, metrics.ActiveConnections)
	})

	t.Run("update new protocol connections", func(t *testing.T) {
		monitor.UpdateConnections("new-protocol", 10)

		metrics, err := monitor.GetMetrics("new-protocol")
		require.NoError(t, err)
		assert.Equal(t, 10, metrics.ActiveConnections)
	})
}

func TestProtocolMonitor_UpdateCacheStats(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("update cache hit rate", func(t *testing.T) {
		monitor.UpdateCacheStats("mcp", 0.85)

		metrics, err := monitor.GetMetrics("mcp")
		require.NoError(t, err)
		assert.Equal(t, 0.85, metrics.CacheHitRate)
	})

	t.Run("update cache for new protocol", func(t *testing.T) {
		monitor.UpdateCacheStats("new-protocol", 0.95)

		metrics, err := monitor.GetMetrics("new-protocol")
		require.NoError(t, err)
		assert.Equal(t, 0.95, metrics.CacheHitRate)
	})
}

func TestProtocolMonitor_UpdateResourceUsage(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	usage := SystemResourceUsage{
		MemoryMB:     256.5,
		CPUPercent:   15.5,
		NetworkBytes: 1024000,
		DiskUsageMB:  500.0,
	}

	t.Run("update resource usage for existing protocol", func(t *testing.T) {
		monitor.RecordRequest(context.Background(), "mcp", 100*time.Millisecond, true, "")
		monitor.UpdateResourceUsage("mcp", usage)

		metrics, err := monitor.GetMetrics("mcp")
		require.NoError(t, err)
		assert.Equal(t, 256.5, metrics.ResourceUsage.MemoryMB)
		assert.Equal(t, 15.5, metrics.ResourceUsage.CPUPercent)
		assert.Equal(t, int64(1024000), metrics.ResourceUsage.NetworkBytes)
	})

	t.Run("update resource usage for new protocol", func(t *testing.T) {
		monitor.UpdateResourceUsage("new-protocol", usage)

		metrics, err := monitor.GetMetrics("new-protocol")
		require.NoError(t, err)
		assert.Equal(t, 256.5, metrics.ResourceUsage.MemoryMB)
	})
}

func TestProtocolMonitor_GetMetrics(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("get existing metrics", func(t *testing.T) {
		monitor.RecordRequest(context.Background(), "mcp", 100*time.Millisecond, true, "")

		metrics, err := monitor.GetMetrics("mcp")
		require.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Equal(t, "mcp", metrics.Protocol)
	})

	t.Run("get non-existent metrics", func(t *testing.T) {
		metrics, err := monitor.GetMetrics("non-existent")
		assert.Error(t, err)
		assert.Nil(t, metrics)
		assert.Contains(t, err.Error(), "no metrics found")
	})
}

func TestProtocolMonitor_GetAllMetrics(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	monitor.RecordRequest(context.Background(), "mcp", 100*time.Millisecond, true, "")
	monitor.RecordRequest(context.Background(), "lsp", 200*time.Millisecond, true, "")
	monitor.RecordRequest(context.Background(), "acp", 150*time.Millisecond, true, "")

	allMetrics := monitor.GetAllMetrics()

	assert.Len(t, allMetrics, 3)
	assert.Contains(t, allMetrics, "mcp")
	assert.Contains(t, allMetrics, "lsp")
	assert.Contains(t, allMetrics, "acp")
}

func TestProtocolMonitor_AddAlertRule(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	rule := &AlertRule{
		ID:          "test-rule",
		Name:        "Test Alert Rule",
		Description: "Test description",
		Protocol:    "mcp",
		Condition:   ConditionErrorRateAbove,
		Threshold:   0.1,
		Severity:    SeverityWarning,
		Cooldown:    5 * time.Minute,
		Enabled:     true,
	}

	monitor.AddAlertRule(rule)

	// Verify rule was added
	monitor.mu.RLock()
	assert.Len(t, monitor.alerts, 1)
	assert.Equal(t, "test-rule", monitor.alerts[0].ID)
	monitor.mu.RUnlock()
}

func TestProtocolMonitor_RemoveAlertRule(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	rule := &AlertRule{
		ID:       "remove-rule",
		Name:     "Rule to Remove",
		Protocol: "mcp",
		Enabled:  true,
	}

	monitor.AddAlertRule(rule)

	t.Run("remove existing rule", func(t *testing.T) {
		monitor.RemoveAlertRule("remove-rule")

		monitor.mu.RLock()
		assert.Len(t, monitor.alerts, 0)
		monitor.mu.RUnlock()
	})

	t.Run("remove non-existent rule", func(t *testing.T) {
		// Should not panic
		monitor.RemoveAlertRule("non-existent")
	})
}

func TestProtocolMonitor_GetAlerts(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	// Get alerts from empty channel
	alerts := monitor.GetAlerts(10)
	assert.Empty(t, alerts)
}

func TestProtocolMonitor_Alerts(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	alertChan := monitor.Alerts()
	assert.NotNil(t, alertChan)
}

func TestProtocolMonitor_Stop(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)

	// Stop should not panic
	monitor.Stop()
}

func TestNewErrorRateAlertRule(t *testing.T) {
	rule := NewErrorRateAlertRule("mcp", 0.1)

	assert.Equal(t, "error-rate-mcp", rule.ID)
	assert.Contains(t, rule.Name, "Error Rate Alert")
	assert.Equal(t, "mcp", rule.Protocol)
	assert.Equal(t, ConditionErrorRateAbove, rule.Condition)
	assert.Equal(t, 0.1, rule.Threshold)
	assert.Equal(t, SeverityError, rule.Severity)
	assert.True(t, rule.Enabled)
}

func TestNewLatencyAlertRule(t *testing.T) {
	rule := NewLatencyAlertRule("lsp", 500.0)

	assert.Equal(t, "latency-lsp", rule.ID)
	assert.Contains(t, rule.Name, "Latency Alert")
	assert.Equal(t, "lsp", rule.Protocol)
	assert.Equal(t, ConditionLatencyAbove, rule.Condition)
	assert.Equal(t, 500.0, rule.Threshold)
	assert.Equal(t, SeverityWarning, rule.Severity)
	assert.True(t, rule.Enabled)
}

func TestNewHighTrafficAlertRule(t *testing.T) {
	rule := NewHighTrafficAlertRule("acp", 10000)

	assert.Equal(t, "traffic-acp", rule.ID)
	assert.Contains(t, rule.Name, "High Traffic Alert")
	assert.Equal(t, "acp", rule.Protocol)
	assert.Equal(t, ConditionGreaterThan, rule.Condition)
	assert.Equal(t, float64(10000), rule.Threshold)
	assert.Equal(t, SeverityInfo, rule.Severity)
	assert.True(t, rule.Enabled)
}

func TestProtocolMetrics_Structure(t *testing.T) {
	metrics := &ProtocolMetrics{
		Protocol:           "test",
		TotalRequests:      100,
		SuccessfulRequests: 90,
		FailedRequests:     10,
		AverageLatency:     100 * time.Millisecond,
		MinLatency:         50 * time.Millisecond,
		MaxLatency:         500 * time.Millisecond,
		Throughput:         5.0,
		ErrorRate:          0.1,
		ActiveConnections:  5,
		CacheHitRate:       0.85,
	}

	assert.Equal(t, "test", metrics.Protocol)
	assert.Equal(t, int64(100), metrics.TotalRequests)
	assert.Equal(t, int64(90), metrics.SuccessfulRequests)
	assert.Equal(t, 0.1, metrics.ErrorRate)
}

func TestSystemResourceUsage_Structure(t *testing.T) {
	usage := SystemResourceUsage{
		MemoryMB:     512.0,
		CPUPercent:   25.0,
		NetworkBytes: 1024000,
		DiskUsageMB:  1000.0,
	}

	assert.Equal(t, 512.0, usage.MemoryMB)
	assert.Equal(t, 25.0, usage.CPUPercent)
	assert.Equal(t, int64(1024000), usage.NetworkBytes)
	assert.Equal(t, 1000.0, usage.DiskUsageMB)
}

func TestAlertRule_Structure(t *testing.T) {
	now := time.Now()
	rule := &AlertRule{
		ID:          "test-id",
		Name:        "Test Rule",
		Description: "Test description",
		Protocol:    "mcp",
		Condition:   ConditionErrorRateAbove,
		Threshold:   0.1,
		Severity:    SeverityWarning,
		Cooldown:    5 * time.Minute,
		LastAlert:   now,
		Enabled:     true,
	}

	assert.Equal(t, "test-id", rule.ID)
	assert.Equal(t, "Test Rule", rule.Name)
	assert.Equal(t, "mcp", rule.Protocol)
	assert.Equal(t, SeverityWarning, rule.Severity)
	assert.True(t, rule.Enabled)
}

func TestAlert_Structure(t *testing.T) {
	now := time.Now()
	resolved := now.Add(time.Hour)
	alert := &Alert{
		ID:         "alert-1",
		RuleID:     "rule-1",
		Protocol:   "mcp",
		Message:    "Error rate exceeded",
		Severity:   SeverityError,
		Value:      0.15,
		Threshold:  0.1,
		Timestamp:  now,
		Resolved:   true,
		ResolvedAt: &resolved,
	}

	assert.Equal(t, "alert-1", alert.ID)
	assert.Equal(t, "rule-1", alert.RuleID)
	assert.Equal(t, "mcp", alert.Protocol)
	assert.Equal(t, SeverityError, alert.Severity)
	assert.True(t, alert.Resolved)
	assert.NotNil(t, alert.ResolvedAt)
}

func TestAlertCondition_Constants(t *testing.T) {
	assert.Equal(t, AlertCondition(0), ConditionGreaterThan)
	assert.Equal(t, AlertCondition(1), ConditionLessThan)
	assert.Equal(t, AlertCondition(2), ConditionEqual)
	assert.Equal(t, AlertCondition(3), ConditionRateAbove)
	assert.Equal(t, AlertCondition(4), ConditionErrorRateAbove)
	assert.Equal(t, AlertCondition(5), ConditionLatencyAbove)
}

func TestAlertSeverity_Constants(t *testing.T) {
	assert.Equal(t, AlertSeverity(0), SeverityInfo)
	assert.Equal(t, AlertSeverity(1), SeverityWarning)
	assert.Equal(t, AlertSeverity(2), SeverityError)
	assert.Equal(t, AlertSeverity(3), SeverityCritical)
}

func BenchmarkProtocolMonitor_RecordRequest(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.RecordRequest(ctx, "bench-protocol", 100*time.Millisecond, true, "")
	}
}

func BenchmarkProtocolMonitor_GetMetrics(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	monitor.RecordRequest(context.Background(), "bench-protocol", 100*time.Millisecond, true, "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = monitor.GetMetrics("bench-protocol")
	}
}
