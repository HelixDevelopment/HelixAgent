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

// Tests for alert storage and retrieval (P1 fix)

func TestProtocolMonitor_StoreAlert(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("stores alert in history", func(t *testing.T) {
		alert := &Alert{
			ID:        "test-alert-1",
			RuleID:    "rule-1",
			Protocol:  "mcp",
			Message:   "Test alert message",
			Severity:  SeverityWarning,
			Value:     0.15,
			Threshold: 0.1,
			Timestamp: time.Now(),
		}

		monitor.storeAlert(alert)

		count := monitor.GetAlertCount()
		assert.Equal(t, 1, count)
	})

	t.Run("stores multiple alerts", func(t *testing.T) {
		monitor.ClearAlerts()

		for i := 0; i < 5; i++ {
			alert := &Alert{
				ID:        "test-alert-" + string(rune('a'+i)),
				RuleID:    "rule-1",
				Protocol:  "mcp",
				Message:   "Test alert message",
				Severity:  SeverityWarning,
				Timestamp: time.Now(),
			}
			monitor.storeAlert(alert)
		}

		count := monitor.GetAlertCount()
		assert.Equal(t, 5, count)
	})
}

func TestProtocolMonitor_AlertHistoryLimit(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("enforces default limit", func(t *testing.T) {
		monitor.ClearAlerts()
		monitor.SetAlertLimit(10) // Set small limit for testing

		// Store more alerts than the limit
		for i := 0; i < 15; i++ {
			alert := &Alert{
				ID:        "alert-" + string(rune('a'+i)),
				RuleID:    "rule-1",
				Protocol:  "mcp",
				Message:   "Test message",
				Severity:  SeverityInfo,
				Timestamp: time.Now(),
			}
			monitor.storeAlert(alert)
		}

		count := monitor.GetAlertCount()
		assert.Equal(t, 10, count)
	})

	t.Run("removes oldest alerts when limit exceeded", func(t *testing.T) {
		monitor.ClearAlerts()
		monitor.SetAlertLimit(5)

		// Store alerts with identifiable IDs
		for i := 0; i < 10; i++ {
			alert := &Alert{
				ID:        "order-" + string(rune('0'+i)),
				RuleID:    "rule-1",
				Protocol:  "mcp",
				Message:   "Message",
				Severity:  SeverityInfo,
				Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			}
			monitor.storeAlert(alert)
		}

		// Should have only the last 5 alerts
		alerts := monitor.GetAlerts(10)
		assert.Len(t, alerts, 5)

		// Most recent should be first
		assert.Equal(t, "order-9", alerts[0].ID)
	})
}

func TestProtocolMonitor_GetAlertsFiltered(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	// Setup test data
	baseTime := time.Now().Add(-time.Hour)
	testAlerts := []*Alert{
		{ID: "1", RuleID: "r1", Protocol: "mcp", Severity: SeverityInfo, Timestamp: baseTime, Resolved: false},
		{ID: "2", RuleID: "r2", Protocol: "lsp", Severity: SeverityWarning, Timestamp: baseTime.Add(10 * time.Minute), Resolved: false},
		{ID: "3", RuleID: "r3", Protocol: "mcp", Severity: SeverityError, Timestamp: baseTime.Add(20 * time.Minute), Resolved: true},
		{ID: "4", RuleID: "r4", Protocol: "acp", Severity: SeverityCritical, Timestamp: baseTime.Add(30 * time.Minute), Resolved: false},
		{ID: "5", RuleID: "r5", Protocol: "mcp", Severity: SeverityWarning, Timestamp: baseTime.Add(40 * time.Minute), Resolved: false},
	}

	monitor.ClearAlerts()
	for _, alert := range testAlerts {
		monitor.storeAlert(alert)
	}

	t.Run("filter by protocol", func(t *testing.T) {
		filter := &AlertFilter{
			Protocol:        "mcp",
			IncludeResolved: true,
		}
		alerts := monitor.GetAlertsFiltered(filter)

		assert.Len(t, alerts, 3)
		for _, alert := range alerts {
			assert.Equal(t, "mcp", alert.Protocol)
		}
	})

	t.Run("filter by severity", func(t *testing.T) {
		severity := SeverityWarning
		filter := &AlertFilter{
			Severity:        &severity,
			IncludeResolved: true,
		}
		alerts := monitor.GetAlertsFiltered(filter)

		assert.Len(t, alerts, 2)
		for _, alert := range alerts {
			assert.Equal(t, SeverityWarning, alert.Severity)
		}
	})

	t.Run("filter excludes resolved by default", func(t *testing.T) {
		filter := &AlertFilter{
			Protocol:        "mcp",
			IncludeResolved: false,
		}
		alerts := monitor.GetAlertsFiltered(filter)

		assert.Len(t, alerts, 2) // Excludes the resolved one
		for _, alert := range alerts {
			assert.False(t, alert.Resolved)
		}
	})

	t.Run("filter by time range", func(t *testing.T) {
		filter := &AlertFilter{
			StartTime:       baseTime.Add(15 * time.Minute),
			EndTime:         baseTime.Add(35 * time.Minute),
			IncludeResolved: true,
		}
		alerts := monitor.GetAlertsFiltered(filter)

		assert.Len(t, alerts, 2)
		for _, alert := range alerts {
			assert.True(t, alert.Timestamp.After(baseTime.Add(15*time.Minute)) || alert.Timestamp.Equal(baseTime.Add(15*time.Minute)))
			assert.True(t, alert.Timestamp.Before(baseTime.Add(35*time.Minute)) || alert.Timestamp.Equal(baseTime.Add(35*time.Minute)))
		}
	})

	t.Run("filter with limit", func(t *testing.T) {
		filter := &AlertFilter{
			Limit:           2,
			IncludeResolved: true,
		}
		alerts := monitor.GetAlertsFiltered(filter)

		assert.Len(t, alerts, 2)
		// Most recent first
		assert.Equal(t, "5", alerts[0].ID)
		assert.Equal(t, "4", alerts[1].ID)
	})

	t.Run("combined filters", func(t *testing.T) {
		severity := SeverityWarning
		filter := &AlertFilter{
			Protocol:        "mcp",
			Severity:        &severity,
			IncludeResolved: true,
		}
		alerts := monitor.GetAlertsFiltered(filter)

		assert.Len(t, alerts, 1)
		assert.Equal(t, "5", alerts[0].ID)
	})

	t.Run("nil filter returns all", func(t *testing.T) {
		alerts := monitor.GetAlertsFiltered(nil)
		// Should return all unresolved (default IncludeResolved is false)
		assert.Len(t, alerts, 4)
	})
}

func TestProtocolMonitor_GetAlerts_BackwardCompatible(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	monitor.ClearAlerts()

	// Store some alerts
	for i := 0; i < 10; i++ {
		alert := &Alert{
			ID:        "compat-" + string(rune('0'+i)),
			RuleID:    "rule-1",
			Protocol:  "mcp",
			Severity:  SeverityInfo,
			Timestamp: time.Now(),
		}
		monitor.storeAlert(alert)
	}

	t.Run("GetAlerts respects limit", func(t *testing.T) {
		alerts := monitor.GetAlerts(5)
		assert.Len(t, alerts, 5)
	})

	t.Run("GetAlerts returns all when limit exceeds count", func(t *testing.T) {
		alerts := monitor.GetAlerts(20)
		assert.Len(t, alerts, 10)
	})

	t.Run("GetAlerts returns most recent first", func(t *testing.T) {
		alerts := monitor.GetAlerts(10)
		assert.Equal(t, "compat-9", alerts[0].ID)
	})
}

func TestProtocolMonitor_SetAlertLimit(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("increases limit", func(t *testing.T) {
		monitor.ClearAlerts()
		monitor.SetAlertLimit(5)

		for i := 0; i < 5; i++ {
			monitor.storeAlert(&Alert{ID: "a" + string(rune('0'+i)), Timestamp: time.Now()})
		}

		monitor.SetAlertLimit(10)

		// Should still have all 5 alerts
		assert.Equal(t, 5, monitor.GetAlertCount())

		// Can now store more
		for i := 0; i < 5; i++ {
			monitor.storeAlert(&Alert{ID: "b" + string(rune('0'+i)), Timestamp: time.Now()})
		}

		assert.Equal(t, 10, monitor.GetAlertCount())
	})

	t.Run("decreases limit and trims history", func(t *testing.T) {
		monitor.ClearAlerts()
		monitor.SetAlertLimit(10)

		for i := 0; i < 10; i++ {
			monitor.storeAlert(&Alert{ID: "trim-" + string(rune('0'+i)), Timestamp: time.Now().Add(time.Duration(i) * time.Second)})
		}

		monitor.SetAlertLimit(5)

		assert.Equal(t, 5, monitor.GetAlertCount())

		// Should have kept the most recent 5
		alerts := monitor.GetAlerts(5)
		assert.Equal(t, "trim-9", alerts[0].ID)
	})

	t.Run("minimum limit is 1", func(t *testing.T) {
		monitor.SetAlertLimit(0)
		monitor.storeAlert(&Alert{ID: "min-test", Timestamp: time.Now()})
		assert.Equal(t, 1, monitor.GetAlertCount())
	})
}

func TestProtocolMonitor_ClearAlerts(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	// Store some alerts
	for i := 0; i < 5; i++ {
		monitor.storeAlert(&Alert{ID: "clear-" + string(rune('0'+i)), Timestamp: time.Now()})
	}

	assert.Equal(t, 5, monitor.GetAlertCount())

	monitor.ClearAlerts()

	assert.Equal(t, 0, monitor.GetAlertCount())
	alerts := monitor.GetAlerts(10)
	assert.Empty(t, alerts)
}

func TestProtocolMonitor_ResolveAlert(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("resolves existing alert", func(t *testing.T) {
		monitor.ClearAlerts()
		alert := &Alert{
			ID:        "resolve-test",
			RuleID:    "rule-1",
			Protocol:  "mcp",
			Timestamp: time.Now(),
			Resolved:  false,
		}
		monitor.storeAlert(alert)

		resolved := monitor.ResolveAlert("resolve-test")
		assert.True(t, resolved)

		// Check the alert is now resolved
		filter := &AlertFilter{IncludeResolved: true}
		alerts := monitor.GetAlertsFiltered(filter)
		require.Len(t, alerts, 1)
		assert.True(t, alerts[0].Resolved)
		assert.NotNil(t, alerts[0].ResolvedAt)
	})

	t.Run("returns false for non-existent alert", func(t *testing.T) {
		resolved := monitor.ResolveAlert("non-existent")
		assert.False(t, resolved)
	})

	t.Run("returns false for already resolved alert", func(t *testing.T) {
		monitor.ClearAlerts()
		resolvedTime := time.Now()
		alert := &Alert{
			ID:         "already-resolved",
			RuleID:     "rule-1",
			Protocol:   "mcp",
			Timestamp:  time.Now(),
			Resolved:   true,
			ResolvedAt: &resolvedTime,
		}
		monitor.storeAlert(alert)

		resolved := monitor.ResolveAlert("already-resolved")
		assert.False(t, resolved)
	})
}

func TestProtocolMonitor_GetAlertCount(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("empty history returns 0", func(t *testing.T) {
		monitor.ClearAlerts()
		assert.Equal(t, 0, monitor.GetAlertCount())
	})

	t.Run("returns correct count", func(t *testing.T) {
		monitor.ClearAlerts()
		for i := 0; i < 7; i++ {
			monitor.storeAlert(&Alert{ID: "count-" + string(rune('0'+i)), Timestamp: time.Now()})
		}
		assert.Equal(t, 7, monitor.GetAlertCount())
	})
}

func TestAlertFilter_Structure(t *testing.T) {
	severity := SeverityError
	now := time.Now()

	filter := AlertFilter{
		Severity:        &severity,
		Protocol:        "mcp",
		StartTime:       now.Add(-time.Hour),
		EndTime:         now,
		Limit:           100,
		IncludeResolved: true,
	}

	assert.Equal(t, SeverityError, *filter.Severity)
	assert.Equal(t, "mcp", filter.Protocol)
	assert.Equal(t, 100, filter.Limit)
	assert.True(t, filter.IncludeResolved)
}

func TestProtocolMonitor_ConcurrentAlertAccess(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			monitor.storeAlert(&Alert{
				ID:        "concurrent-" + string(rune(i)),
				Protocol:  "mcp",
				Timestamp: time.Now(),
			})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = monitor.GetAlerts(10)
			_ = monitor.GetAlertCount()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Should not panic or have race conditions
	count := monitor.GetAlertCount()
	assert.GreaterOrEqual(t, count, 1)
}

func BenchmarkProtocolMonitor_StoreAlert(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	alert := &Alert{
		ID:        "bench-alert",
		RuleID:    "rule-1",
		Protocol:  "mcp",
		Message:   "Benchmark alert",
		Severity:  SeverityInfo,
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.storeAlert(alert)
	}
}

func BenchmarkProtocolMonitor_GetAlertsFiltered(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	// Store some alerts
	for i := 0; i < 100; i++ {
		monitor.storeAlert(&Alert{
			ID:        "bench-" + string(rune(i)),
			Protocol:  "mcp",
			Severity:  SeverityInfo,
			Timestamp: time.Now(),
		})
	}

	filter := &AlertFilter{
		Protocol:        "mcp",
		Limit:           50,
		IncludeResolved: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = monitor.GetAlertsFiltered(filter)
	}
}

// Tests for system metrics functions

func TestCollectRealSystemMetrics(t *testing.T) {
	t.Run("returns valid system metrics", func(t *testing.T) {
		usage := collectRealSystemMetrics()

		// Memory should be positive (we're using memory)
		assert.GreaterOrEqual(t, usage.MemoryMB, 0.0)

		// CPU percent should be non-negative
		assert.GreaterOrEqual(t, usage.CPUPercent, 0.0)

		// Network bytes should be non-negative
		assert.GreaterOrEqual(t, usage.NetworkBytes, int64(0))

		// Disk usage should be non-negative
		assert.GreaterOrEqual(t, usage.DiskUsageMB, 0.0)
	})
}

func TestCollectCPUPercent(t *testing.T) {
	t.Run("returns valid CPU percentage", func(t *testing.T) {
		// First call initializes the stats
		cpuPercent1 := collectCPUPercent()
		assert.GreaterOrEqual(t, cpuPercent1, 0.0)

		// Wait a moment for stats to change
		time.Sleep(50 * time.Millisecond)

		// Second call should calculate delta
		cpuPercent2 := collectCPUPercent()
		assert.GreaterOrEqual(t, cpuPercent2, 0.0)
		assert.LessOrEqual(t, cpuPercent2, 100.0)
	})
}

func TestCollectNetworkBytes(t *testing.T) {
	t.Run("returns valid network bytes", func(t *testing.T) {
		// First call
		bytes1 := collectNetworkBytes()
		assert.GreaterOrEqual(t, bytes1, int64(0))

		// Second call should return delta
		bytes2 := collectNetworkBytes()
		assert.GreaterOrEqual(t, bytes2, int64(0))
	})
}

func TestCollectDiskUsage(t *testing.T) {
	t.Run("returns valid disk usage", func(t *testing.T) {
		diskUsage := collectDiskUsage()
		// Should be positive (we're using disk space)
		assert.GreaterOrEqual(t, diskUsage, 0.0)
	})
}

func TestProtocolMonitor_MetricsCollector(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)

	// Record some metrics first
	monitor.RecordRequest(context.Background(), "test-protocol", 100*time.Millisecond, true, "")

	// The metricsCollector runs in the background
	// Give it a moment to potentially collect metrics
	time.Sleep(100 * time.Millisecond)

	// Verify the monitor is functioning
	metrics, err := monitor.GetMetrics("test-protocol")
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	// Stop should clean up the collector goroutine
	monitor.Stop()
}

func TestProtocolMonitor_AlertChecker(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)

	// Add an alert rule that will be triggered
	rule := &AlertRule{
		ID:          "test-check-rule",
		Name:        "Test Alert",
		Description: "Test alert for checking",
		Protocol:    "test-protocol",
		Condition:   ConditionErrorRateAbove,
		Threshold:   0.1,
		Severity:    SeverityWarning,
		Cooldown:    0, // No cooldown for testing
		Enabled:     true,
	}
	monitor.AddAlertRule(rule)

	// Record failing requests to trigger the alert
	monitor.RecordRequest(context.Background(), "test-protocol", 100*time.Millisecond, false, "error")
	monitor.RecordRequest(context.Background(), "test-protocol", 100*time.Millisecond, false, "error")

	// Verify error rate is above threshold
	metrics, err := monitor.GetMetrics("test-protocol")
	require.NoError(t, err)
	assert.Equal(t, 1.0, metrics.ErrorRate)

	// Give alert checker time to run
	time.Sleep(100 * time.Millisecond)

	// Stop and clean up
	monitor.Stop()
}

func TestProtocolMonitor_CheckAlerts(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	t.Run("triggers error rate alert", func(t *testing.T) {
		monitor.ClearAlerts()

		rule := &AlertRule{
			ID:          "error-rate-test",
			Name:        "Error Rate Test",
			Description: "Test error rate",
			Protocol:    "error-test",
			Condition:   ConditionErrorRateAbove,
			Threshold:   0.5,
			Severity:    SeverityError,
			Cooldown:    0,
			Enabled:     true,
		}
		monitor.AddAlertRule(rule)

		// Record requests with high error rate
		monitor.RecordRequest(context.Background(), "error-test", 100*time.Millisecond, false, "error")
		monitor.RecordRequest(context.Background(), "error-test", 100*time.Millisecond, false, "error")

		// Manually trigger check
		monitor.checkAlerts()

		// Check alert was stored
		count := monitor.GetAlertCount()
		assert.GreaterOrEqual(t, count, 1)
	})

	t.Run("triggers latency alert", func(t *testing.T) {
		monitor.ClearAlerts()

		rule := &AlertRule{
			ID:          "latency-test",
			Name:        "Latency Test",
			Description: "Test latency",
			Protocol:    "latency-test",
			Condition:   ConditionLatencyAbove,
			Threshold:   50.0, // 50ms threshold
			Severity:    SeverityWarning,
			Cooldown:    0,
			Enabled:     true,
		}
		monitor.AddAlertRule(rule)

		// Record high latency request
		monitor.RecordRequest(context.Background(), "latency-test", 200*time.Millisecond, true, "")

		// Manually trigger check
		monitor.checkAlerts()

		// Check alert was stored
		count := monitor.GetAlertCount()
		assert.GreaterOrEqual(t, count, 1)
	})

	t.Run("triggers traffic alert", func(t *testing.T) {
		monitor.ClearAlerts()

		rule := &AlertRule{
			ID:          "traffic-test",
			Name:        "Traffic Test",
			Description: "Test traffic",
			Protocol:    "traffic-test",
			Condition:   ConditionGreaterThan,
			Threshold:   5, // Low threshold for testing
			Severity:    SeverityInfo,
			Cooldown:    0,
			Enabled:     true,
		}
		monitor.AddAlertRule(rule)

		// Record many requests
		for i := 0; i < 10; i++ {
			monitor.RecordRequest(context.Background(), "traffic-test", 100*time.Millisecond, true, "")
		}

		// Manually trigger check
		monitor.checkAlerts()

		// Check alert was stored
		count := monitor.GetAlertCount()
		assert.GreaterOrEqual(t, count, 1)
	})

	t.Run("respects cooldown period", func(t *testing.T) {
		// Use fresh monitor for this test
		cooldownMonitor := NewProtocolMonitor(log)
		defer cooldownMonitor.Stop()

		rule := &AlertRule{
			ID:          "cooldown-test",
			Name:        "Cooldown Test",
			Description: "Test cooldown",
			Protocol:    "cooldown-test",
			Condition:   ConditionErrorRateAbove,
			Threshold:   0.1,
			Severity:    SeverityWarning,
			Cooldown:    1 * time.Hour, // Long cooldown
			Enabled:     true,
			LastAlert:   time.Now(), // Just fired
		}
		cooldownMonitor.AddAlertRule(rule)

		// Record failing requests
		cooldownMonitor.RecordRequest(context.Background(), "cooldown-test", 100*time.Millisecond, false, "error")

		// Trigger check - should not fire due to cooldown
		initialCount := cooldownMonitor.GetAlertCount()
		cooldownMonitor.checkAlerts()

		// Count should not have increased
		assert.Equal(t, initialCount, cooldownMonitor.GetAlertCount())
	})

	t.Run("skips disabled rules", func(t *testing.T) {
		// Use fresh monitor for this test
		disabledMonitor := NewProtocolMonitor(log)
		defer disabledMonitor.Stop()

		rule := &AlertRule{
			ID:          "disabled-test",
			Name:        "Disabled Test",
			Description: "Test disabled",
			Protocol:    "disabled-test",
			Condition:   ConditionErrorRateAbove,
			Threshold:   0.1,
			Severity:    SeverityWarning,
			Cooldown:    0,
			Enabled:     false, // Disabled
		}
		disabledMonitor.AddAlertRule(rule)

		// Record failing requests
		disabledMonitor.RecordRequest(context.Background(), "disabled-test", 100*time.Millisecond, false, "error")

		// Trigger check - should not fire due to being disabled
		disabledMonitor.checkAlerts()

		// No alerts should be stored
		assert.Equal(t, 0, disabledMonitor.GetAlertCount())
	})

	t.Run("skips non-existent protocol", func(t *testing.T) {
		// Use fresh monitor for this test
		nonexistMonitor := NewProtocolMonitor(log)
		defer nonexistMonitor.Stop()

		rule := &AlertRule{
			ID:          "nonexistent-test",
			Name:        "Nonexistent Test",
			Description: "Test nonexistent",
			Protocol:    "does-not-exist",
			Condition:   ConditionErrorRateAbove,
			Threshold:   0.1,
			Severity:    SeverityWarning,
			Cooldown:    0,
			Enabled:     true,
		}
		nonexistMonitor.AddAlertRule(rule)

		// Trigger check - should not fire due to no metrics
		nonexistMonitor.checkAlerts()

		// No alerts should be stored
		assert.Equal(t, 0, nonexistMonitor.GetAlertCount())
	})
}

func TestProtocolMonitor_CollectSystemMetrics(t *testing.T) {
	log := newMonitorTestLogger()
	monitor := NewProtocolMonitor(log)
	defer monitor.Stop()

	// Record some requests first to create metrics
	monitor.RecordRequest(context.Background(), "system-test", 100*time.Millisecond, true, "")

	// Trigger system metrics collection
	monitor.collectSystemMetrics()

	// Verify metrics were updated
	metrics, err := monitor.GetMetrics("system-test")
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	// Resource usage should be populated
	assert.GreaterOrEqual(t, metrics.ResourceUsage.MemoryMB, 0.0)
}
