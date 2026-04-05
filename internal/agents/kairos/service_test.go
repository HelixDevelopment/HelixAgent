package kairos

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, 5*time.Minute, config.TickInterval)
	assert.Equal(t, 15*time.Second, config.BlockingBudget)
	assert.Equal(t, 30, config.LogRetentionDays)
	assert.True(t, config.EnableNotifications)
	assert.NotEmpty(t, config.WorkspacePath)
	assert.NotEmpty(t, config.LogPath)
}

func TestNewService(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	service := NewService(config, logger)

	assert.NotNil(t, service)
	assert.Equal(t, config, service.config)
	assert.NotNil(t, service.observations)
	assert.NotNil(t, service.actions)
	assert.NotNil(t, service.stopCh)
	assert.False(t, service.running)
}

func TestService_SetCallbacks(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	service := NewService(config, logger)

	observationCalled := false
	actionCalled := false
	decisionCalled := false

	service.SetCallbacks(
		func(o Observation) { observationCalled = true },
		func(a Action) { actionCalled = true },
		func(t TickPrompt) (Action, error) {
			decisionCalled = true
			return Action{}, nil
		},
	)

	// Test that callbacks are set
	service.onObservation(Observation{})
	service.onAction(Action{})
	service.onDecision(TickPrompt{})

	assert.True(t, observationCalled)
	assert.True(t, actionCalled)
	assert.True(t, decisionCalled)
}

func TestService_StartStop(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	config.Enabled = true
	config.TickInterval = 100 * time.Millisecond

	service := NewService(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)
	assert.True(t, service.IsRunning())

	// Stop service
	err = service.Stop()
	require.NoError(t, err)
	assert.False(t, service.IsRunning())
}

func TestService_Start_AlreadyRunning(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	config.Enabled = true
	config.TickInterval = 100 * time.Millisecond

	service := NewService(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)

	// Try to start again
	err = service.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	service.Stop()
}

func TestService_Start_Disabled(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	config.Enabled = false

	service := NewService(config, logger)

	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)
	assert.False(t, service.IsRunning())
}

func TestService_Observe(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	service := NewService(config, logger)

	observationCalled := false
	service.SetCallbacks(
		func(o Observation) { observationCalled = true },
		func(a Action) {},
		func(t TickPrompt) (Action, error) { return Action{}, nil },
	)

	service.Observe("test", "test_source", "test content", map[string]interface{}{"key": "value"})

	assert.True(t, observationCalled)

	observations := service.GetObservations()
	require.Len(t, observations, 1)
	assert.Equal(t, "test", observations[0].Type)
	assert.Equal(t, "test_source", observations[0].Source)
	assert.Equal(t, "test content", observations[0].Content)
}

func TestService_GetObservations(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	service := NewService(config, logger)

	// Add observations
	for i := 0; i < 5; i++ {
		service.Observe("test", "source", "content", nil)
	}

	observations := service.GetObservations()
	assert.Len(t, observations, 5)
}

func TestService_GetRecentObservations(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	service := NewService(config, logger)

	// Add observations
	for i := 0; i < 10; i++ {
		service.Observe("test", "source", "content", nil)
	}

	recent := service.getRecentObservations(3)
	assert.Len(t, recent, 3)

	recent = service.getRecentObservations(20)
	assert.Len(t, recent, 10)
}

func TestService_RecordAction(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	service := NewService(config, logger)

	action := Action{
		Type:        "test_action",
		Description: "Test action",
		Status:      "pending",
	}

	service.recordAction(action)

	actions := service.GetActions()
	assert.Len(t, actions, 1)
}

func TestService_UpdateAction(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	service := NewService(config, logger)

	action := Action{
		ID:          "action_123",
		Type:        "test_action",
		Description: "Test action",
		Status:      "pending",
	}

	service.recordAction(action)

	updatedAction := Action{
		ID:     "action_123",
		Status: "completed",
	}
	service.updateAction(updatedAction)

	actions := service.GetActions()
	require.Len(t, actions, 1)
	assert.Equal(t, "completed", actions[0].Status)
}

func TestService_GetDailySummary(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := DefaultConfig()
	service := NewService(config, logger)

	// Add observations and actions
	service.Observe("type1", "source", "content1", nil)
	service.Observe("type1", "source", "content2", nil)
	service.Observe("type2", "source", "content3", nil)

	service.recordAction(Action{Type: "action1", Status: "completed", Timestamp: time.Now()})
	service.recordAction(Action{Type: "action2", Status: "pending", Timestamp: time.Now()})

	summary := service.GetDailySummary(time.Now())

	assert.Equal(t, 3, summary["observations"])
	assert.Equal(t, 2, summary["actions"])

	observationTypes := summary["observation_types"].(map[string]int)
	assert.Equal(t, 2, observationTypes["type1"])
	assert.Equal(t, 1, observationTypes["type2"])

	actionStatuses := summary["action_statuses"].(map[string]int)
	assert.Equal(t, 1, actionStatuses["completed"])
	assert.Equal(t, 1, actionStatuses["pending"])
}

func TestCountByType(t *testing.T) {
	t.Parallel()
	observations := []Observation{
		{Type: "type1"},
		{Type: "type1"},
		{Type: "type2"},
	}

	counts := countByType(observations)
	assert.Equal(t, 2, counts["type1"])
	assert.Equal(t, 1, counts["type2"])
}

func TestCountActionStatuses(t *testing.T) {
	t.Parallel()
	actions := []Action{
		{Status: "completed"},
		{Status: "completed"},
		{Status: "pending"},
	}

	counts := countActionStatuses(actions)
	assert.Equal(t, 2, counts["completed"])
	assert.Equal(t, 1, counts["pending"])
}

func TestGenerateActionID(t *testing.T) {
	t.Parallel()
	id1 := generateActionID()
	id2 := generateActionID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "action_")
}

func TestService_CleanupOldLogs(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()

	service := NewService(config, logger)

	// This test mainly checks that the method doesn't panic
	// In a real test, we'd create mock log files
	err := service.CleanupOldLogs()
	// May error if directory doesn't exist, which is fine
	assert.NoError(t, err)
}

func TestService_Tick_WithinBudget(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	config.TickInterval = 50 * time.Millisecond
	config.BlockingBudget = 100 * time.Millisecond

	service := NewService(config, logger)

	service.SetCallbacks(
		func(o Observation) {},
		func(a Action) {},
		func(t TickPrompt) (Action, error) {
			return Action{
				Type:        "test",
				Description: "Test action",
				Duration:    10 * time.Millisecond, // Within budget
			}, nil
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := service.Start(ctx)
	require.NoError(t, err)

	// Wait for at least one tick
	time.Sleep(100 * time.Millisecond)

	service.Stop()

	// We mainly verify no panic occurred
}

func TestService_Tick_ExceedsBudget(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	config.TickInterval = 50 * time.Millisecond
	config.BlockingBudget = 10 * time.Millisecond

	service := NewService(config, logger)

	service.SetCallbacks(
		func(o Observation) {},
		func(a Action) {},
		func(t TickPrompt) (Action, error) {
			return Action{
				Type:        "test",
				Description: "Test action",
				Duration:    100 * time.Millisecond, // Exceeds budget
			}, nil
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := service.Start(ctx)
	require.NoError(t, err)

	// Wait for at least one tick
	time.Sleep(100 * time.Millisecond)

	actions := service.GetActions()
	service.Stop()

	// Action should be deferred (status = deferred)
	if len(actions) > 0 {
		assert.Equal(t, "deferred", actions[0].Status)
	}
}

func BenchmarkService_Observe(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	config := DefaultConfig()
	service := NewService(config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.Observe("test", "source", "content", nil)
	}
}
