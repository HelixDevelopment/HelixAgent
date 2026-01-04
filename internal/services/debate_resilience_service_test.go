package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestDebateConfigForResilience(debateID string) *DebateConfig {
	return &DebateConfig{
		DebateID:  debateID,
		Topic:     "Resilience Test Topic",
		MaxRounds: 5,
		Timeout:   5 * time.Minute,
		Strategy:  "consensus",
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Participant 1", LLMProvider: "openai", LLMModel: "gpt-4"},
			{ParticipantID: "p2", Name: "Participant 2", LLMProvider: "anthropic", LLMModel: "claude-3"},
		},
	}
}

func TestDefaultResilienceConfig(t *testing.T) {
	config := DefaultResilienceConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryDelay)
	assert.True(t, config.CheckpointEnabled)
}

func TestDebateResilienceService_New(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.activeDebates)
	assert.Equal(t, 3, svc.maxRetries)
	assert.Equal(t, 2*time.Second, svc.retryDelay)
	assert.True(t, svc.checkpointEnabled)
}

func TestNewDebateResilienceServiceWithConfig(t *testing.T) {
	logger := createTestLogger()

	t.Run("with custom config", func(t *testing.T) {
		config := &ResilienceConfig{
			MaxRetries:        5,
			RetryDelay:        5 * time.Second,
			CheckpointEnabled: false,
		}
		svc := NewDebateResilienceServiceWithConfig(logger, config)

		assert.NotNil(t, svc)
		assert.Equal(t, 5, svc.maxRetries)
		assert.Equal(t, 5*time.Second, svc.retryDelay)
		assert.False(t, svc.checkpointEnabled)
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		svc := NewDebateResilienceServiceWithConfig(logger, nil)

		assert.NotNil(t, svc)
		assert.Equal(t, 3, svc.maxRetries)
		assert.Equal(t, 2*time.Second, svc.retryDelay)
		assert.True(t, svc.checkpointEnabled)
	})
}

func TestDebateResilienceService_SetDebateService(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)

	assert.Nil(t, svc.debateService)

	// Create a mock debate service (can be nil for testing)
	svc.SetDebateService(nil)
	assert.Nil(t, svc.debateService)
}

func TestDebateResilienceService_Failure(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)
	ctx := context.Background()

	t.Run("handle non-nil error", func(t *testing.T) {
		err := svc.HandleFailure(ctx, errors.New("test error"))
		assert.NoError(t, err)
	})

	t.Run("handle nil error", func(t *testing.T) {
		err := svc.HandleFailure(ctx, nil)
		assert.NoError(t, err)
	})
}

func TestDebateResilienceService_HandleDebateFailure(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)
	ctx := context.Background()

	// Register a debate first
	config := createTestDebateConfigForResilience("debate-failure")
	svc.RegisterDebate(config)

	t.Run("handle failure for registered debate", func(t *testing.T) {
		err := svc.HandleDebateFailure(ctx, "debate-failure", errors.New("test error"))
		assert.NoError(t, err)

		state, err := svc.GetDebateState("debate-failure")
		require.NoError(t, err)
		assert.Equal(t, 1, state.FailureCount)
		assert.Equal(t, "failed", state.Status)
		assert.Equal(t, "test error", state.LastError)
	})

	t.Run("handle failure for non-registered debate", func(t *testing.T) {
		err := svc.HandleDebateFailure(ctx, "nonexistent", errors.New("error"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active debate found")
	})
}

func TestDebateResilienceService_RegisterDebate(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)

	t.Run("register new debate", func(t *testing.T) {
		config := createTestDebateConfigForResilience("debate-register")

		state := svc.RegisterDebate(config)
		assert.NotNil(t, state)
		assert.Equal(t, "debate-register", state.DebateID)
		assert.Equal(t, config, state.Config)
		assert.Equal(t, 0, state.CurrentRound)
		assert.Equal(t, "active", state.Status)
		assert.Equal(t, 0, state.FailureCount)
		assert.Equal(t, 0, state.RecoveryAttempt)
		assert.NotNil(t, state.Responses)
	})

	t.Run("register multiple debates", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			config := createTestDebateConfigForResilience("debate-multi-" + string(rune('A'+i)))
			state := svc.RegisterDebate(config)
			assert.NotNil(t, state)
		}

		ids := svc.ListActiveDebates()
		assert.GreaterOrEqual(t, len(ids), 5)
	})
}

func TestDebateResilienceService_UpdateDebateProgress(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)

	config := createTestDebateConfigForResilience("debate-progress")
	svc.RegisterDebate(config)

	t.Run("update progress for registered debate", func(t *testing.T) {
		responses := []ParticipantResponse{
			{ParticipantID: "p1", Response: "Response 1"},
			{ParticipantID: "p2", Response: "Response 2"},
		}

		err := svc.UpdateDebateProgress("debate-progress", 1, responses)
		assert.NoError(t, err)

		state, err := svc.GetDebateState("debate-progress")
		require.NoError(t, err)
		assert.Equal(t, 1, state.CurrentRound)
		assert.Len(t, state.Responses, 2)
	})

	t.Run("update progress multiple times", func(t *testing.T) {
		responses := []ParticipantResponse{
			{ParticipantID: "p1", Response: "Response 3"},
		}

		err := svc.UpdateDebateProgress("debate-progress", 2, responses)
		assert.NoError(t, err)

		state, err := svc.GetDebateState("debate-progress")
		require.NoError(t, err)
		assert.Equal(t, 2, state.CurrentRound)
		assert.Len(t, state.Responses, 3)
	})

	t.Run("update progress for non-registered debate", func(t *testing.T) {
		err := svc.UpdateDebateProgress("nonexistent", 1, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active debate found")
	})
}

func TestDebateResilienceService_CompleteDebate(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)

	config := createTestDebateConfigForResilience("debate-complete")
	svc.RegisterDebate(config)

	t.Run("complete registered debate", func(t *testing.T) {
		err := svc.CompleteDebate("debate-complete")
		assert.NoError(t, err)

		state, err := svc.GetDebateState("debate-complete")
		require.NoError(t, err)
		assert.Equal(t, "completed", state.Status)
	})

	t.Run("complete non-registered debate", func(t *testing.T) {
		err := svc.CompleteDebate("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active debate found")
	})
}

func TestDebateResilienceService_Recover(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceServiceWithConfig(logger, &ResilienceConfig{
		MaxRetries:        3,
		RetryDelay:        10 * time.Millisecond, // Short delay for testing
		CheckpointEnabled: true,
	})
	ctx := context.Background()

	t.Run("recover non-existing debate", func(t *testing.T) {
		result, err := svc.RecoverDebate(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no debate state found")
	})

	t.Run("recover already completed debate", func(t *testing.T) {
		config := createTestDebateConfigForResilience("debate-recover-completed")
		svc.RegisterDebate(config)
		_ = svc.CompleteDebate("debate-recover-completed")

		result, err := svc.RecoverDebate(ctx, "debate-recover-completed")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "already completed")
	})

	t.Run("recover without debate service", func(t *testing.T) {
		config := createTestDebateConfigForResilience("debate-no-service")
		svc.RegisterDebate(config)
		_ = svc.HandleDebateFailure(ctx, "debate-no-service", errors.New("error"))

		result, err := svc.RecoverDebate(ctx, "debate-no-service")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "debate service not configured")
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		config := createTestDebateConfigForResilience("debate-max-retries")
		state := svc.RegisterDebate(config)

		// Manually set recovery attempts to max
		svc.debatesMu.Lock()
		state.RecoveryAttempt = 3
		state.Status = "failed"
		svc.debatesMu.Unlock()

		result, err := svc.RecoverDebate(ctx, "debate-max-retries")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "max recovery attempts")
	})

	t.Run("recover with context cancellation", func(t *testing.T) {
		config := createTestDebateConfigForResilience("debate-cancel")
		svc.RegisterDebate(config)
		_ = svc.HandleDebateFailure(ctx, "debate-cancel", errors.New("error"))

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		result, err := svc.RecoverDebate(cancelCtx, "debate-cancel")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestDebateResilienceService_GetDebateState(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)

	config := createTestDebateConfigForResilience("debate-state")
	svc.RegisterDebate(config)

	t.Run("get existing state", func(t *testing.T) {
		state, err := svc.GetDebateState("debate-state")
		assert.NoError(t, err)
		assert.NotNil(t, state)
		assert.Equal(t, "debate-state", state.DebateID)
	})

	t.Run("get non-existing state", func(t *testing.T) {
		state, err := svc.GetDebateState("nonexistent")
		assert.Error(t, err)
		assert.Nil(t, state)
		assert.Contains(t, err.Error(), "no debate state found")
	})

	t.Run("returned state is a copy", func(t *testing.T) {
		state, err := svc.GetDebateState("debate-state")
		require.NoError(t, err)

		// Modify the copy
		state.Status = "modified"

		// Original should be unchanged
		original, err := svc.GetDebateState("debate-state")
		require.NoError(t, err)
		assert.NotEqual(t, "modified", original.Status)
	})
}

func TestDebateResilienceService_ListActiveDebates(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)
	ctx := context.Background()

	t.Run("empty list", func(t *testing.T) {
		ids := svc.ListActiveDebates()
		assert.Empty(t, ids)
	})

	t.Run("list active debates only", func(t *testing.T) {
		// Register active debates
		for i := 0; i < 3; i++ {
			config := createTestDebateConfigForResilience("debate-active-" + string(rune('A'+i)))
			svc.RegisterDebate(config)
		}

		// Register and complete one
		config := createTestDebateConfigForResilience("debate-completed")
		svc.RegisterDebate(config)
		_ = svc.CompleteDebate("debate-completed")

		// Register and fail one
		config2 := createTestDebateConfigForResilience("debate-failed")
		svc.RegisterDebate(config2)
		_ = svc.HandleDebateFailure(ctx, "debate-failed", errors.New("error"))

		ids := svc.ListActiveDebates()
		assert.Equal(t, 3, len(ids)) // Only active ones, not completed or failed
	})

	t.Run("includes recovering debates", func(t *testing.T) {
		// Manually set a debate to recovering status
		config := createTestDebateConfigForResilience("debate-recovering")
		svc.RegisterDebate(config)

		svc.debatesMu.Lock()
		svc.activeDebates["debate-recovering"].Status = "recovering"
		svc.debatesMu.Unlock()

		ids := svc.ListActiveDebates()
		found := false
		for _, id := range ids {
			if id == "debate-recovering" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestDebateResilienceService_CleanupCompletedDebates(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)

	// Create and complete debates with old timestamps
	for i := 0; i < 5; i++ {
		config := createTestDebateConfigForResilience("debate-cleanup-" + string(rune('A'+i)))
		svc.RegisterDebate(config)
		_ = svc.CompleteDebate("debate-cleanup-" + string(rune('A'+i)))
	}

	// Set old timestamps
	svc.debatesMu.Lock()
	for _, state := range svc.activeDebates {
		if state.Status == "completed" {
			state.LastUpdated = time.Now().Add(-2 * time.Hour)
		}
	}
	svc.debatesMu.Unlock()

	// Create active debates (should not be cleaned up)
	for i := 0; i < 2; i++ {
		config := createTestDebateConfigForResilience("debate-active-cleanup-" + string(rune('A'+i)))
		svc.RegisterDebate(config)
	}

	// Cleanup debates older than 1 hour
	removed := svc.CleanupCompletedDebates(time.Hour)
	assert.Equal(t, 5, removed)

	// Active debates should still be there
	ids := svc.ListActiveDebates()
	assert.Equal(t, 2, len(ids))
}

func TestDebateResilienceService_GetStats(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)
	ctx := context.Background()

	t.Run("empty stats", func(t *testing.T) {
		stats := svc.GetStats()
		assert.Equal(t, 0, stats["total_debates"].(int))
		assert.Equal(t, 0, stats["active"].(int))
		assert.Equal(t, 0, stats["completed"].(int))
		assert.Equal(t, 0, stats["failed"].(int))
		assert.Equal(t, 0, stats["recovered"].(int))
		assert.Equal(t, 0, stats["recovering"].(int))
	})

	t.Run("stats with various states", func(t *testing.T) {
		// Active debates
		for i := 0; i < 3; i++ {
			config := createTestDebateConfigForResilience("debate-stats-active-" + string(rune('A'+i)))
			svc.RegisterDebate(config)
		}

		// Completed debate
		config := createTestDebateConfigForResilience("debate-stats-completed")
		svc.RegisterDebate(config)
		_ = svc.CompleteDebate("debate-stats-completed")

		// Failed debate
		config2 := createTestDebateConfigForResilience("debate-stats-failed")
		svc.RegisterDebate(config2)
		_ = svc.HandleDebateFailure(ctx, "debate-stats-failed", errors.New("error"))

		stats := svc.GetStats()
		assert.Equal(t, 5, stats["total_debates"].(int))
		assert.Equal(t, 3, stats["active"].(int))
		assert.Equal(t, 1, stats["completed"].(int))
		assert.Equal(t, 1, stats["failed"].(int))
		assert.GreaterOrEqual(t, stats["total_failures"].(int), 1)
	})
}

func TestDebateResilienceService_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			config := createTestDebateConfigForResilience("debate-concurrent-" + string(rune('A'+id%26)) + string(rune('A'+id/26)))
			svc.RegisterDebate(config)
		}(i)
	}

	wg.Wait()

	// Concurrent reads and updates
	debates := svc.ListActiveDebates()
	for _, debateID := range debates {
		wg.Add(3)
		go func(id string) {
			defer wg.Done()
			_, _ = svc.GetDebateState(id)
		}(debateID)
		go func(id string) {
			defer wg.Done()
			_ = svc.UpdateDebateProgress(id, 1, nil)
		}(debateID)
		go func(id string) {
			defer wg.Done()
			_ = svc.HandleDebateFailure(ctx, id, errors.New("error"))
		}(debateID)
	}

	wg.Wait()
}

func TestDebateState_Structure(t *testing.T) {
	now := time.Now()
	state := &DebateState{
		DebateID: "debate-123",
		Config: &DebateConfig{
			DebateID:  "debate-123",
			MaxRounds: 5,
		},
		CurrentRound: 2,
		Responses: []ParticipantResponse{
			{ParticipantID: "p1", Response: "Response 1"},
		},
		StartTime:       now.Add(-time.Hour),
		LastUpdated:     now,
		Status:          "active",
		FailureCount:    1,
		LastError:       "test error",
		RecoveryAttempt: 0,
	}

	assert.Equal(t, "debate-123", state.DebateID)
	assert.Equal(t, 2, state.CurrentRound)
	assert.Len(t, state.Responses, 1)
	assert.Equal(t, "active", state.Status)
	assert.Equal(t, 1, state.FailureCount)
	assert.Equal(t, "test error", state.LastError)
}

func TestResilienceConfig_Structure(t *testing.T) {
	config := &ResilienceConfig{
		MaxRetries:        5,
		RetryDelay:        3 * time.Second,
		CheckpointEnabled: true,
	}

	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 3*time.Second, config.RetryDelay)
	assert.True(t, config.CheckpointEnabled)
}

func TestDebateResilienceService_CheckpointLogging(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("checkpoint enabled", func(t *testing.T) {
		svc := NewDebateResilienceServiceWithConfig(logger, &ResilienceConfig{
			MaxRetries:        3,
			RetryDelay:        time.Second,
			CheckpointEnabled: true,
		})

		config := createTestDebateConfigForResilience("debate-checkpoint-enabled")
		svc.RegisterDebate(config)

		err := svc.UpdateDebateProgress("debate-checkpoint-enabled", 1, nil)
		assert.NoError(t, err)
	})

	t.Run("checkpoint disabled", func(t *testing.T) {
		svc := NewDebateResilienceServiceWithConfig(logger, &ResilienceConfig{
			MaxRetries:        3,
			RetryDelay:        time.Second,
			CheckpointEnabled: false,
		})

		config := createTestDebateConfigForResilience("debate-checkpoint-disabled")
		svc.RegisterDebate(config)

		err := svc.UpdateDebateProgress("debate-checkpoint-disabled", 1, nil)
		assert.NoError(t, err)
	})
}

func TestDebateResilienceService_MultipleFailures(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceService(logger)
	ctx := context.Background()

	config := createTestDebateConfigForResilience("debate-multi-failures")
	svc.RegisterDebate(config)

	// Record multiple failures
	for i := 0; i < 5; i++ {
		err := svc.HandleDebateFailure(ctx, "debate-multi-failures", errors.New("error "+string(rune('A'+i))))
		assert.NoError(t, err)
	}

	state, err := svc.GetDebateState("debate-multi-failures")
	require.NoError(t, err)
	assert.Equal(t, 5, state.FailureCount)
	assert.Equal(t, "failed", state.Status)
	assert.Contains(t, state.LastError, "error E") // Last error
}

func TestDebateResilienceService_RecoveryWithPartialProgress(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateResilienceServiceWithConfig(logger, &ResilienceConfig{
		MaxRetries:        3,
		RetryDelay:        10 * time.Millisecond,
		CheckpointEnabled: true,
	})

	config := createTestDebateConfigForResilience("debate-partial")
	svc.RegisterDebate(config)

	// Simulate partial progress
	responses := []ParticipantResponse{
		{ParticipantID: "p1", Response: "Response 1"},
		{ParticipantID: "p2", Response: "Response 2"},
	}
	_ = svc.UpdateDebateProgress("debate-partial", 2, responses)

	// Verify state
	state, err := svc.GetDebateState("debate-partial")
	require.NoError(t, err)
	assert.Equal(t, 2, state.CurrentRound)
	assert.Len(t, state.Responses, 2)
}
