package replay

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/messaging/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewHandler(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultReplayConfig()
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, config, handler.config)
	assert.NotNil(t, handler.activeReplays)
	assert.NotNil(t, handler.semaphore)
}

func TestDefaultReplayConfig(t *testing.T) {
	config := DefaultReplayConfig()

	assert.Equal(t, 5, config.MaxConcurrentReplays)
	assert.Equal(t, 100, config.DefaultBatchSize)
	assert.Equal(t, 1000, config.MaxBatchSize)
	assert.Equal(t, 5*time.Minute, config.ReplayTimeout)
	assert.Equal(t, 10000, config.BufferSize)
}

func TestHandler_ValidateRequest(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultReplayConfig()
	logger := zap.NewNop()
	handler := NewHandler(broker, config, logger)

	tests := []struct {
		name      string
		request   *ReplayRequest
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid request",
			request: &ReplayRequest{
				ID:       "replay-1",
				Topic:    "test.topic",
				FromTime: time.Now().Add(-1 * time.Hour),
			},
			expectErr: false,
		},
		{
			name: "missing ID",
			request: &ReplayRequest{
				Topic:    "test.topic",
				FromTime: time.Now().Add(-1 * time.Hour),
			},
			expectErr: true,
			errMsg:    "request ID is required",
		},
		{
			name: "missing topic",
			request: &ReplayRequest{
				ID:       "replay-1",
				FromTime: time.Now().Add(-1 * time.Hour),
			},
			expectErr: true,
			errMsg:    "topic is required",
		},
		{
			name: "missing from_time",
			request: &ReplayRequest{
				ID:    "replay-1",
				Topic: "test.topic",
			},
			expectErr: true,
			errMsg:    "from_time is required",
		},
		{
			name: "to_time before from_time",
			request: &ReplayRequest{
				ID:       "replay-1",
				Topic:    "test.topic",
				FromTime: time.Now(),
				ToTime:   time.Now().Add(-1 * time.Hour),
			},
			expectErr: true,
			errMsg:    "to_time must be after from_time",
		},
		{
			name: "valid request with time range",
			request: &ReplayRequest{
				ID:       "replay-1",
				Topic:    "test.topic",
				FromTime: time.Now().Add(-1 * time.Hour),
				ToTime:   time.Now(),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.validateRequest(tt.request)
			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandler_StartReplay(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultReplayConfig()
	config.ReplayTimeout = 1 * time.Second
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	request := &ReplayRequest{
		ID:       "replay-1",
		Topic:    "test.topic",
		FromTime: time.Now().Add(-1 * time.Hour),
		Options: &ReplayOptions{
			DryRun: true,
		},
	}

	progress, err := handler.StartReplay(ctx, request)
	require.NoError(t, err)
	assert.NotNil(t, progress)
	assert.Equal(t, "replay-1", progress.RequestID)
	assert.NotZero(t, progress.StartTime)

	// Wait for replay to complete
	time.Sleep(500 * time.Millisecond)
}

func TestHandler_StartReplay_DuplicateID(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultReplayConfig()
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	request := &ReplayRequest{
		ID:       "replay-1",
		Topic:    "test.topic",
		FromTime: time.Now().Add(-1 * time.Hour),
	}

	_, err = handler.StartReplay(ctx, request)
	require.NoError(t, err)

	// Try to start with same ID
	_, err = handler.StartReplay(ctx, request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestHandler_GetProgress(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultReplayConfig()
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	request := &ReplayRequest{
		ID:       "replay-progress-test",
		Topic:    "test.topic",
		FromTime: time.Now().Add(-1 * time.Hour),
		Options: &ReplayOptions{
			DryRun: true,
		},
	}

	_, err = handler.StartReplay(ctx, request)
	require.NoError(t, err)

	// Get progress
	progress, err := handler.GetProgress("replay-progress-test")
	require.NoError(t, err)
	assert.NotNil(t, progress)
	assert.Equal(t, "replay-progress-test", progress.RequestID)
}

func TestHandler_GetProgress_NotFound(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultReplayConfig()
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	_, err := handler.GetProgress("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHandler_CancelReplay(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultReplayConfig()
	config.ReplayTimeout = 10 * time.Second
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	request := &ReplayRequest{
		ID:       "replay-cancel-test",
		Topic:    "test.topic",
		FromTime: time.Now().Add(-1 * time.Hour),
	}

	_, err = handler.StartReplay(ctx, request)
	require.NoError(t, err)

	// Wait a bit for replay to start
	time.Sleep(100 * time.Millisecond)

	// Try to cancel - may fail if replay already completed (which is fine)
	cancelErr := handler.CancelReplay("replay-cancel-test")

	// Check status - either cancelled or completed is acceptable
	progress, err := handler.GetProgress("replay-cancel-test")
	require.NoError(t, err)

	if cancelErr == nil {
		assert.Equal(t, ReplayStatusCancelled, progress.Status)
	} else {
		// Replay completed before we could cancel, which is also valid
		assert.Equal(t, ReplayStatusCompleted, progress.Status)
	}
}

func TestHandler_CancelReplay_NotFound(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultReplayConfig()
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	err := handler.CancelReplay("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHandler_ListReplays(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultReplayConfig()
	config.ReplayTimeout = 1 * time.Second
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	// Start multiple replays
	for i := 0; i < 3; i++ {
		request := &ReplayRequest{
			ID:       "replay-list-" + string(rune('a'+i)),
			Topic:    "test.topic",
			FromTime: time.Now().Add(-1 * time.Hour),
			Options: &ReplayOptions{
				DryRun: true,
			},
		}
		_, err = handler.StartReplay(ctx, request)
		require.NoError(t, err)
	}

	// List replays
	replays := handler.ListReplays()
	assert.Len(t, replays, 3)
}

func TestHandler_CleanupOldReplays(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultReplayConfig()
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	// Add completed replays
	handler.mu.Lock()
	handler.activeReplays["old-1"] = &ReplayProgress{
		RequestID: "old-1",
		Status:    ReplayStatusCompleted,
		EndTime:   time.Now().Add(-2 * time.Hour),
	}
	handler.activeReplays["old-2"] = &ReplayProgress{
		RequestID: "old-2",
		Status:    ReplayStatusFailed,
		EndTime:   time.Now().Add(-2 * time.Hour),
	}
	handler.activeReplays["recent"] = &ReplayProgress{
		RequestID: "recent",
		Status:    ReplayStatusCompleted,
		EndTime:   time.Now(),
	}
	handler.mu.Unlock()

	// Cleanup replays older than 1 hour
	removed := handler.CleanupOldReplays(1 * time.Hour)
	assert.Equal(t, 2, removed)

	// Verify remaining
	replays := handler.ListReplays()
	assert.Len(t, replays, 1)
	assert.Equal(t, "recent", replays[0].RequestID)
}

func TestHandler_MatchesFilter(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultReplayConfig()
	logger := zap.NewNop()

	handler := NewHandler(broker, config, logger)

	tests := []struct {
		name    string
		msg     *messaging.Message
		filter  *ReplayFilter
		matches bool
	}{
		{
			name:    "nil filter matches all",
			msg:     &messaging.Message{Type: "any.type"},
			filter:  nil,
			matches: true,
		},
		{
			name:    "empty filter matches all",
			msg:     &messaging.Message{Type: "any.type"},
			filter:  &ReplayFilter{},
			matches: true,
		},
		{
			name: "type filter matches",
			msg:  &messaging.Message{Type: "llm.response"},
			filter: &ReplayFilter{
				MessageTypes: []string{"llm.response", "debate.round"},
			},
			matches: true,
		},
		{
			name: "type filter doesn't match",
			msg:  &messaging.Message{Type: "other.type"},
			filter: &ReplayFilter{
				MessageTypes: []string{"llm.response", "debate.round"},
			},
			matches: false,
		},
		{
			name: "header filter matches",
			msg: &messaging.Message{
				Type:    "any.type",
				Headers: map[string]string{"trace-id": "trace-123"},
			},
			filter: &ReplayFilter{
				Headers: map[string]string{"trace-id": "trace-123"},
			},
			matches: true,
		},
		{
			name: "header filter doesn't match",
			msg: &messaging.Message{
				Type:    "any.type",
				Headers: map[string]string{"trace-id": "trace-456"},
			},
			filter: &ReplayFilter{
				Headers: map[string]string{"trace-id": "trace-123"},
			},
			matches: false,
		},
		{
			name: "combined filter matches",
			msg: &messaging.Message{
				Type:    "llm.response",
				Headers: map[string]string{"trace-id": "trace-123"},
			},
			filter: &ReplayFilter{
				MessageTypes: []string{"llm.response"},
				Headers:      map[string]string{"trace-id": "trace-123"},
			},
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.matchesFilter(tt.msg, tt.filter)
			assert.Equal(t, tt.matches, result)
		})
	}
}

func TestReplayStatus_Values(t *testing.T) {
	tests := []struct {
		status   ReplayStatus
		expected string
	}{
		{ReplayStatusPending, "pending"},
		{ReplayStatusRunning, "running"},
		{ReplayStatusCompleted, "completed"},
		{ReplayStatusFailed, "failed"},
		{ReplayStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestReplayProgress_JSON(t *testing.T) {
	progress := &ReplayProgress{
		RequestID:       "replay-json-test",
		Status:          ReplayStatusRunning,
		TotalMessages:   1000,
		ReplayedCount:   500,
		SkippedCount:    10,
		FailedCount:     2,
		StartTime:       time.Now(),
		CurrentOffset:   512,
		LastProcessedID: "msg-512",
		Rate:            100.5,
	}

	data, err := progress.JSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "replay-json-test")
	assert.Contains(t, string(data), "running")
}

func TestReplayOptions_Defaults(t *testing.T) {
	options := &ReplayOptions{}

	assert.Equal(t, 0, options.BatchSize)
	assert.Equal(t, time.Duration(0), options.DelayBetween)
	assert.False(t, options.DryRun)
	assert.False(t, options.PreserveOrder)
	assert.False(t, options.SkipDuplicates)
}

func TestReplayFilter_Empty(t *testing.T) {
	filter := &ReplayFilter{}

	assert.Empty(t, filter.MessageTypes)
	assert.Nil(t, filter.Headers)
	assert.Nil(t, filter.Metadata)
}
