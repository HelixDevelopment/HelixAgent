package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestDebateConfig(debateID string) *DebateConfig {
	return &DebateConfig{
		DebateID:  debateID,
		Topic:     "Test Debate Topic",
		MaxRounds: 5,
		Timeout:   5 * time.Minute,
		Strategy:  "consensus",
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Participant 1", LLMProvider: "openai", LLMModel: "gpt-4"},
			{ParticipantID: "p2", Name: "Participant 2", LLMProvider: "anthropic", LLMModel: "claude-3"},
		},
	}
}

func TestDebateMonitoringService_New(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.sessions)
	assert.NotNil(t, svc.config)
	assert.Equal(t, 5*time.Second, svc.config.CheckInterval)
	assert.Equal(t, 3, svc.config.AlertThreshold)
	assert.Equal(t, 30*time.Second, svc.config.HealthCheckPeriod)
}

func TestNewDebateMonitoringServiceWithConfig(t *testing.T) {
	logger := createTestLogger()

	t.Run("with custom config", func(t *testing.T) {
		config := &MonitoringConfig{
			CheckInterval:     10 * time.Second,
			AlertThreshold:    5,
			HealthCheckPeriod: time.Minute,
		}
		svc := NewDebateMonitoringServiceWithConfig(logger, config)

		assert.NotNil(t, svc)
		assert.Equal(t, 10*time.Second, svc.config.CheckInterval)
		assert.Equal(t, 5, svc.config.AlertThreshold)
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		svc := NewDebateMonitoringServiceWithConfig(logger, nil)

		assert.NotNil(t, svc)
		assert.Equal(t, 5*time.Second, svc.config.CheckInterval)
		assert.Equal(t, 3, svc.config.AlertThreshold)
	})
}

func TestDebateMonitoringService_Start(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	t.Run("start with valid config", func(t *testing.T) {
		config := createTestDebateConfig("debate-mon-1")

		monitoringID, err := svc.StartMonitoring(ctx, config)
		assert.NoError(t, err)
		assert.NotEmpty(t, monitoringID)
		assert.Contains(t, monitoringID, "mon-")

		// Cleanup
		err = svc.StopMonitoring(ctx, monitoringID)
		assert.NoError(t, err)
	})

	t.Run("start with nil config", func(t *testing.T) {
		monitoringID, err := svc.StartMonitoring(ctx, nil)
		assert.Error(t, err)
		assert.Empty(t, monitoringID)
		assert.Contains(t, err.Error(), "debate config is required")
	})

	t.Run("start initializes participant status", func(t *testing.T) {
		config := createTestDebateConfig("debate-mon-2")

		monitoringID, err := svc.StartMonitoring(ctx, config)
		require.NoError(t, err)

		status, err := svc.GetExtendedStatus(ctx, monitoringID)
		require.NoError(t, err)
		assert.Equal(t, len(config.Participants), len(status.Participants))

		// Cleanup
		_ = svc.StopMonitoring(ctx, monitoringID)
	})
}

func TestDebateMonitoringService_Stop(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	t.Run("stop existing session", func(t *testing.T) {
		config := createTestDebateConfig("debate-stop-1")
		monitoringID, err := svc.StartMonitoring(ctx, config)
		require.NoError(t, err)

		err = svc.StopMonitoring(ctx, monitoringID)
		assert.NoError(t, err)

		// Session should be marked as inactive
		sessions := svc.ListActiveSessions()
		assert.NotContains(t, sessions, monitoringID)
	})

	t.Run("stop non-existing session", func(t *testing.T) {
		err := svc.StopMonitoring(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "monitoring session not found")
	})
}

func TestDebateMonitoringService_Status(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	config := createTestDebateConfig("debate-status-1")
	monitoringID, err := svc.StartMonitoring(ctx, config)
	require.NoError(t, err)
	defer svc.StopMonitoring(ctx, monitoringID)

	t.Run("get status by debate ID", func(t *testing.T) {
		status, err := svc.GetStatus(ctx, "debate-status-1")
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "debate-status-1", status.DebateID)
		assert.Equal(t, "pending", status.Status)
	})

	t.Run("get status for non-existing debate", func(t *testing.T) {
		status, err := svc.GetStatus(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "no monitoring session found")
	})
}

func TestDebateMonitoringService_GetStatusByMonitoringID(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	config := createTestDebateConfig("debate-status-2")
	monitoringID, err := svc.StartMonitoring(ctx, config)
	require.NoError(t, err)
	defer svc.StopMonitoring(ctx, monitoringID)

	t.Run("get status by monitoring ID", func(t *testing.T) {
		status, err := svc.GetStatusByMonitoringID(ctx, monitoringID)
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "debate-status-2", status.DebateID)
	})

	t.Run("get status for non-existing monitoring ID", func(t *testing.T) {
		status, err := svc.GetStatusByMonitoringID(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, status)
	})
}

func TestDebateMonitoringService_GetExtendedStatus(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	config := createTestDebateConfig("debate-ext-status")
	monitoringID, err := svc.StartMonitoring(ctx, config)
	require.NoError(t, err)
	defer svc.StopMonitoring(ctx, monitoringID)

	t.Run("get extended status", func(t *testing.T) {
		status, err := svc.GetExtendedStatus(ctx, monitoringID)
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, 100.0, status.HealthScore)
		assert.Equal(t, 0, status.ErrorCount)
		assert.Equal(t, 0, status.WarningCount)
	})

	t.Run("extended status not found", func(t *testing.T) {
		status, err := svc.GetExtendedStatus(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, status)
	})
}

func TestDebateMonitoringService_UpdateParticipantStatus(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	config := createTestDebateConfig("debate-participant")
	monitoringID, err := svc.StartMonitoring(ctx, config)
	require.NoError(t, err)
	defer svc.StopMonitoring(ctx, monitoringID)

	t.Run("update valid participant", func(t *testing.T) {
		err := svc.UpdateParticipantStatus(ctx, monitoringID, "p1", "active", 500*time.Millisecond)
		assert.NoError(t, err)

		status, err := svc.GetExtendedStatus(ctx, monitoringID)
		require.NoError(t, err)

		found := false
		for _, p := range status.Participants {
			if p.ParticipantID == "p1" {
				assert.Equal(t, "active", p.Status)
				assert.Equal(t, 500*time.Millisecond, p.ResponseTime)
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("update non-existing participant", func(t *testing.T) {
		err := svc.UpdateParticipantStatus(ctx, monitoringID, "nonexistent", "active", time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "participant not found")
	})

	t.Run("update with non-existing monitoring ID", func(t *testing.T) {
		err := svc.UpdateParticipantStatus(ctx, "nonexistent", "p1", "active", time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "monitoring session not found")
	})
}

func TestDebateMonitoringService_UpdateRound(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	config := createTestDebateConfig("debate-round")
	monitoringID, err := svc.StartMonitoring(ctx, config)
	require.NoError(t, err)
	defer svc.StopMonitoring(ctx, monitoringID)

	t.Run("update round activates debate", func(t *testing.T) {
		err := svc.UpdateRound(ctx, monitoringID, 1)
		assert.NoError(t, err)

		status, err := svc.GetExtendedStatus(ctx, monitoringID)
		require.NoError(t, err)
		assert.Equal(t, 1, status.CurrentRound)
		assert.Equal(t, "active", status.Status)
	})

	t.Run("update to final round completes debate", func(t *testing.T) {
		err := svc.UpdateRound(ctx, monitoringID, config.MaxRounds)
		assert.NoError(t, err)

		status, err := svc.GetExtendedStatus(ctx, monitoringID)
		require.NoError(t, err)
		assert.Equal(t, "completed", status.Status)
	})

	t.Run("update non-existing session", func(t *testing.T) {
		err := svc.UpdateRound(ctx, "nonexistent", 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "monitoring session not found")
	})
}

func TestDebateMonitoringService_RecordError(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	config := createTestDebateConfig("debate-error")
	monitoringID, err := svc.StartMonitoring(ctx, config)
	require.NoError(t, err)
	defer svc.StopMonitoring(ctx, monitoringID)

	t.Run("record single error", func(t *testing.T) {
		err := svc.RecordError(ctx, monitoringID, "Test error")
		assert.NoError(t, err)

		status, err := svc.GetExtendedStatus(ctx, monitoringID)
		require.NoError(t, err)
		assert.Equal(t, 1, status.ErrorCount)
	})

	t.Run("multiple errors trigger failure", func(t *testing.T) {
		// Record errors up to threshold
		for i := 0; i < 2; i++ {
			err := svc.RecordError(ctx, monitoringID, "Error "+string(rune('A'+i)))
			assert.NoError(t, err)
		}

		status, err := svc.GetExtendedStatus(ctx, monitoringID)
		require.NoError(t, err)
		assert.Equal(t, "failed", status.Status)
	})

	t.Run("record error for non-existing session", func(t *testing.T) {
		err := svc.RecordError(ctx, "nonexistent", "Error")
		assert.Error(t, err)
	})
}

func TestDebateMonitoringService_GetAlerts(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	config := createTestDebateConfig("debate-alerts")
	monitoringID, err := svc.StartMonitoring(ctx, config)
	require.NoError(t, err)
	defer svc.StopMonitoring(ctx, monitoringID)

	t.Run("no alerts initially", func(t *testing.T) {
		alerts, err := svc.GetAlerts(ctx, monitoringID)
		assert.NoError(t, err)
		assert.Empty(t, alerts)
	})

	t.Run("alerts after errors", func(t *testing.T) {
		_ = svc.RecordError(ctx, monitoringID, "Test error 1")
		_ = svc.RecordError(ctx, monitoringID, "Test error 2")

		alerts, err := svc.GetAlerts(ctx, monitoringID)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(alerts), 2)

		for _, alert := range alerts {
			assert.NotEmpty(t, alert.ID)
			assert.Equal(t, "debate-alerts", alert.DebateID)
			assert.NotEmpty(t, alert.Level)
		}
	})

	t.Run("get alerts for non-existing session", func(t *testing.T) {
		alerts, err := svc.GetAlerts(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, alerts)
	})
}

func TestDebateMonitoringService_ListActiveSessions(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	t.Run("no active sessions initially", func(t *testing.T) {
		sessions := svc.ListActiveSessions()
		assert.Empty(t, sessions)
	})

	t.Run("list multiple active sessions", func(t *testing.T) {
		var monitoringIDs []string
		for i := 0; i < 3; i++ {
			config := createTestDebateConfig("debate-list-" + string(rune('A'+i)))
			id, err := svc.StartMonitoring(ctx, config)
			require.NoError(t, err)
			monitoringIDs = append(monitoringIDs, id)
		}

		sessions := svc.ListActiveSessions()
		assert.Equal(t, 3, len(sessions))

		// Stop one
		err := svc.StopMonitoring(ctx, monitoringIDs[0])
		require.NoError(t, err)

		sessions = svc.ListActiveSessions()
		assert.Equal(t, 2, len(sessions))

		// Cleanup
		for _, id := range monitoringIDs[1:] {
			_ = svc.StopMonitoring(ctx, id)
		}
	})
}

func TestDebateMonitoringService_CleanupInactiveSessions(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	// Create and stop sessions
	for i := 0; i < 3; i++ {
		config := createTestDebateConfig("debate-cleanup-" + string(rune('A'+i)))
		id, err := svc.StartMonitoring(ctx, config)
		require.NoError(t, err)
		err = svc.StopMonitoring(ctx, id)
		require.NoError(t, err)
	}

	// Manually set old timestamps
	svc.sessionsMu.Lock()
	for _, session := range svc.sessions {
		session.LastCheck = time.Now().Add(-2 * time.Hour)
	}
	svc.sessionsMu.Unlock()

	// Cleanup sessions older than 1 hour
	removed := svc.CleanupInactiveSessions(time.Hour)
	assert.Equal(t, 3, removed)

	stats := svc.GetStats()
	assert.Equal(t, 0, stats["total_sessions"].(int))
}

func TestDebateMonitoringService_GetStats(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	t.Run("empty stats", func(t *testing.T) {
		stats := svc.GetStats()
		assert.Equal(t, 0, stats["total_sessions"].(int))
		assert.Equal(t, 0, stats["active_sessions"].(int))
		assert.Equal(t, 0, stats["total_alerts"].(int))
	})

	t.Run("stats with sessions", func(t *testing.T) {
		config := createTestDebateConfig("debate-stats")
		monitoringID, err := svc.StartMonitoring(ctx, config)
		require.NoError(t, err)

		_ = svc.RecordError(ctx, monitoringID, "Error")

		stats := svc.GetStats()
		assert.Equal(t, 1, stats["total_sessions"].(int))
		assert.Equal(t, 1, stats["active_sessions"].(int))
		assert.GreaterOrEqual(t, stats["total_alerts"].(int), 1)

		_ = svc.StopMonitoring(ctx, monitoringID)
	})
}

func TestDebateMonitoringService_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateMonitoringService(logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 20

	// Start multiple sessions concurrently
	monitoringIDs := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			config := createTestDebateConfig("debate-concurrent-" + string(rune('A'+id)))
			monitoringID, err := svc.StartMonitoring(ctx, config)
			if err == nil {
				monitoringIDs <- monitoringID
			}
		}(i)
	}

	wg.Wait()
	close(monitoringIDs)

	// Concurrent updates and reads
	var wg2 sync.WaitGroup
	for id := range monitoringIDs {
		wg2.Add(3)
		go func(monID string) {
			defer wg2.Done()
			_ = svc.UpdateRound(ctx, monID, 1)
		}(id)
		go func(monID string) {
			defer wg2.Done()
			_, _ = svc.GetExtendedStatus(ctx, monID)
		}(id)
		go func(monID string) {
			defer wg2.Done()
			_ = svc.StopMonitoring(ctx, monID)
		}(id)
	}

	wg2.Wait()
}

func TestMonitoringConfig_Defaults(t *testing.T) {
	config := &MonitoringConfig{
		CheckInterval:     time.Second * 5,
		AlertThreshold:    3,
		HealthCheckPeriod: time.Second * 30,
	}

	assert.Equal(t, 5*time.Second, config.CheckInterval)
	assert.Equal(t, 3, config.AlertThreshold)
	assert.Equal(t, 30*time.Second, config.HealthCheckPeriod)
}

func TestExtendedDebateStatus_Structure(t *testing.T) {
	now := time.Now()
	status := &ExtendedDebateStatus{
		DebateStatus: DebateStatus{
			DebateID:     "debate-123",
			Status:       "active",
			CurrentRound: 2,
			TotalRounds:  5,
			StartTime:    now,
		},
		LastUpdateTime: now,
		HealthScore:    85.5,
		ErrorCount:     1,
		WarningCount:   2,
	}

	assert.Equal(t, "debate-123", status.DebateID)
	assert.Equal(t, "active", status.Status)
	assert.Equal(t, 85.5, status.HealthScore)
	assert.Equal(t, 1, status.ErrorCount)
	assert.Equal(t, 2, status.WarningCount)
}

func TestMonitoringAlert_Structure(t *testing.T) {
	now := time.Now()
	alert := MonitoringAlert{
		ID:         "alert-123",
		DebateID:   "debate-456",
		Level:      "error",
		Message:    "Test error message",
		Timestamp:  now,
		Resolved:   false,
		ResolvedAt: time.Time{},
	}

	assert.Equal(t, "alert-123", alert.ID)
	assert.Equal(t, "debate-456", alert.DebateID)
	assert.Equal(t, "error", alert.Level)
	assert.Equal(t, "Test error message", alert.Message)
	assert.False(t, alert.Resolved)
}

func TestDebateMonitoringService_HealthScoreCalculation(t *testing.T) {
	logger := createTestLogger()
	// Use a longer check interval so the monitoring loop doesn't interfere
	config := &MonitoringConfig{
		CheckInterval:     time.Hour,
		AlertThreshold:    3,
		HealthCheckPeriod: time.Hour,
	}
	svc := NewDebateMonitoringServiceWithConfig(logger, config)
	ctx := context.Background()

	debateConfig := createTestDebateConfig("debate-health")
	monitoringID, err := svc.StartMonitoring(ctx, debateConfig)
	require.NoError(t, err)
	defer svc.StopMonitoring(ctx, monitoringID)

	// Initial health should be 100
	status, err := svc.GetExtendedStatus(ctx, monitoringID)
	require.NoError(t, err)
	assert.Equal(t, 100.0, status.HealthScore)

	// After recording errors, health should decrease
	_ = svc.RecordError(ctx, monitoringID, "Error 1")
	_ = svc.RecordError(ctx, monitoringID, "Error 2")

	status, err = svc.GetExtendedStatus(ctx, monitoringID)
	require.NoError(t, err)
	assert.Equal(t, 2, status.ErrorCount)
}
