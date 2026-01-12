package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected bool
	}{
		{TaskStatusPending, false},
		{TaskStatusQueued, false},
		{TaskStatusRunning, false},
		{TaskStatusPaused, false},
		{TaskStatusCompleted, true},
		{TaskStatusFailed, true},
		{TaskStatusStuck, false},
		{TaskStatusCancelled, true},
		{TaskStatusDeadLetter, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsTerminal())
		})
	}
}

func TestTaskStatus_IsActive(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected bool
	}{
		{TaskStatusPending, false},
		{TaskStatusQueued, true},
		{TaskStatusRunning, true},
		{TaskStatusPaused, false},
		{TaskStatusCompleted, false},
		{TaskStatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsActive())
		})
	}
}

func TestTaskPriority_Weight(t *testing.T) {
	tests := []struct {
		priority TaskPriority
		expected int
	}{
		{TaskPriorityCritical, 0},
		{TaskPriorityHigh, 1},
		{TaskPriorityNormal, 2},
		{TaskPriorityLow, 3},
		{TaskPriorityBackground, 4},
		{TaskPriority("unknown"), 2}, // defaults to normal
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.priority.Weight())
		})
	}
}

func TestNewBackgroundTask(t *testing.T) {
	payload := json.RawMessage(`{"command": "echo hello"}`)
	task := NewBackgroundTask("command", "test-task", payload)

	assert.NotNil(t, task)
	assert.Equal(t, "command", task.TaskType)
	assert.Equal(t, "test-task", task.TaskName)
	assert.Equal(t, TaskStatusPending, task.Status)
	assert.Equal(t, TaskPriorityNormal, task.Priority)
	assert.Equal(t, float64(0), task.Progress)
	assert.Equal(t, 3, task.MaxRetries)
	assert.Equal(t, 0, task.RetryCount)
	assert.Equal(t, 60, task.RetryDelaySeconds)
	assert.Equal(t, 1, task.RequiredCPUCores)
	assert.Equal(t, 512, task.RequiredMemoryMB)
	assert.True(t, task.Config.AllowPause)
	assert.True(t, task.Config.AllowCancel)
	assert.False(t, task.Config.Endless)
}

func TestDefaultTaskConfig(t *testing.T) {
	config := DefaultTaskConfig()

	assert.Equal(t, 1800, config.TimeoutSeconds)
	assert.Equal(t, 10, config.HeartbeatIntervalSecs)
	assert.Equal(t, 300, config.StuckThresholdSecs)
	assert.True(t, config.AllowPause)
	assert.True(t, config.AllowCancel)
	assert.False(t, config.Endless)
	assert.Equal(t, 30, config.GracefulShutdownSecs)
	assert.True(t, config.CaptureOutput)
	assert.True(t, config.CaptureStderr)
}

func TestBackgroundTask_CanRetry(t *testing.T) {
	task := NewBackgroundTask("test", "test", nil)

	// Can retry when retry count is less than max
	assert.True(t, task.CanRetry())

	// Cannot retry when retry count equals max
	task.RetryCount = task.MaxRetries
	assert.False(t, task.CanRetry())

	// Cannot retry when retry count exceeds max
	task.RetryCount = task.MaxRetries + 1
	assert.False(t, task.CanRetry())
}

func TestBackgroundTask_CanPause(t *testing.T) {
	task := NewBackgroundTask("test", "test", nil)

	// Cannot pause pending task
	assert.False(t, task.CanPause())

	// Can pause running task if allowed
	task.Status = TaskStatusRunning
	assert.True(t, task.CanPause())

	// Cannot pause if not allowed
	task.Config.AllowPause = false
	assert.False(t, task.CanPause())
}

func TestBackgroundTask_CanCancel(t *testing.T) {
	task := NewBackgroundTask("test", "test", nil)

	// Can cancel pending task
	assert.True(t, task.CanCancel())

	// Can cancel running task
	task.Status = TaskStatusRunning
	assert.True(t, task.CanCancel())

	// Cannot cancel completed task
	task.Status = TaskStatusCompleted
	assert.False(t, task.CanCancel())

	// Cannot cancel if not allowed
	task.Status = TaskStatusRunning
	task.Config.AllowCancel = false
	assert.False(t, task.CanCancel())
}

func TestBackgroundTask_CanResume(t *testing.T) {
	task := NewBackgroundTask("test", "test", nil)

	// Cannot resume pending task
	assert.False(t, task.CanResume())

	// Can resume paused task
	task.Status = TaskStatusPaused
	assert.True(t, task.CanResume())

	// Cannot resume completed task
	task.Status = TaskStatusCompleted
	assert.False(t, task.CanResume())
}

func TestBackgroundTask_Duration(t *testing.T) {
	task := NewBackgroundTask("test", "test", nil)

	// No duration if not started
	assert.Nil(t, task.Duration())

	// Has duration if started
	startTime := time.Now().Add(-5 * time.Second)
	task.StartedAt = &startTime
	task.Status = TaskStatusRunning

	duration := task.Duration()
	require.NotNil(t, duration)
	assert.True(t, *duration >= 5*time.Second)

	// Uses completed time if available
	endTime := startTime.Add(10 * time.Second)
	task.CompletedAt = &endTime
	task.Status = TaskStatusCompleted

	duration = task.Duration()
	require.NotNil(t, duration)
	assert.Equal(t, 10*time.Second, *duration)
}

func TestBackgroundTask_IsOverdue(t *testing.T) {
	task := NewBackgroundTask("test", "test", nil)

	// Not overdue if no deadline
	assert.False(t, task.IsOverdue())

	// Not overdue if deadline is in future
	future := time.Now().Add(1 * time.Hour)
	task.Deadline = &future
	assert.False(t, task.IsOverdue())

	// Overdue if deadline is in past
	past := time.Now().Add(-1 * time.Hour)
	task.Deadline = &past
	assert.True(t, task.IsOverdue())
}

func TestBackgroundTask_HasStaleHeartbeat(t *testing.T) {
	task := NewBackgroundTask("test", "test", nil)
	threshold := 5 * time.Minute

	// Stale if no heartbeat
	assert.True(t, task.HasStaleHeartbeat(threshold))

	// Not stale if heartbeat is recent
	recent := time.Now()
	task.LastHeartbeat = &recent
	assert.False(t, task.HasStaleHeartbeat(threshold))

	// Stale if heartbeat is old
	old := time.Now().Add(-10 * time.Minute)
	task.LastHeartbeat = &old
	assert.True(t, task.HasStaleHeartbeat(threshold))
}

func TestTaskConfig_JSON(t *testing.T) {
	config := DefaultTaskConfig()

	// Marshal to JSON
	data, err := json.Marshal(config)
	require.NoError(t, err)
	assert.Contains(t, string(data), "timeout_seconds")

	// Unmarshal from JSON
	var parsed TaskConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Equal(t, config.TimeoutSeconds, parsed.TimeoutSeconds)
	assert.Equal(t, config.AllowPause, parsed.AllowPause)
}

func TestNotificationConfig_JSON(t *testing.T) {
	config := NotificationConfig{
		Webhooks: []WebhookConfig{
			{
				URL:      "https://example.com/webhook",
				Method:   "POST",
				Events:   []string{TaskEventCompleted, TaskEventFailed},
				RetryMax: 3,
			},
		},
		SSE: &SSEConfig{
			Channel: "task-events",
			Enabled: true,
		},
		WebSocket: &WSConfig{
			Room:    "task-room",
			Enabled: true,
		},
		OnEvents: []string{TaskEventStarted, TaskEventCompleted},
	}

	// Marshal to JSON
	data, err := json.Marshal(config)
	require.NoError(t, err)
	assert.Contains(t, string(data), "https://example.com/webhook")

	// Unmarshal from JSON
	var parsed NotificationConfig
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Len(t, parsed.Webhooks, 1)
	assert.Equal(t, "https://example.com/webhook", parsed.Webhooks[0].URL)
	assert.NotNil(t, parsed.SSE)
	assert.True(t, parsed.SSE.Enabled)
}

func TestResourceSnapshot(t *testing.T) {
	snapshot := &ResourceSnapshot{
		TaskID:         "task-123",
		CPUPercent:     45.5,
		MemoryRSSBytes: 1024 * 1024 * 256, // 256MB
		MemoryPercent:  25.0,
		IOReadBytes:    1024 * 1024,
		IOWriteBytes:   512 * 1024,
		NetBytesSent:   1024,
		NetBytesRecv:   2048,
		OpenFDs:        50,
		ThreadCount:    4,
		ProcessState:   "S",
		SampledAt:      time.Now(),
	}

	// Verify values
	assert.Equal(t, "task-123", snapshot.TaskID)
	assert.Equal(t, 45.5, snapshot.CPUPercent)
	assert.Equal(t, int64(256*1024*1024), snapshot.MemoryRSSBytes)
}

func TestTaskLogEntry(t *testing.T) {
	entry := TaskLogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Source:    "worker-1",
		Message:   "Task started",
		LineNum:   1,
		Fields: map[string]interface{}{
			"task_id": "task-123",
		},
	}

	assert.Equal(t, "info", entry.Level)
	assert.Equal(t, "Task started", entry.Message)
	assert.Contains(t, entry.Fields, "task_id")
}

func TestTaskProgressUpdate(t *testing.T) {
	update := TaskProgressUpdate{
		TaskID:             "task-123",
		Progress:           50.0,
		Message:            "Processing...",
		CurrentStep:        "Step 2",
		TotalSteps:         5,
		CurrentStepNumber:  2,
		TokensGenerated:    1000,
		TokensPerSecond:    42.5,
		EstimatedRemaining: 30000, // 30 seconds
	}

	assert.Equal(t, 50.0, update.Progress)
	assert.Equal(t, "Step 2", update.CurrentStep)
	assert.Equal(t, 42.5, update.TokensPerSecond)
}

func TestTaskEventConstants(t *testing.T) {
	// Verify event constants are defined
	events := []string{
		TaskEventCreated,
		TaskEventStarted,
		TaskEventProgress,
		TaskEventHeartbeat,
		TaskEventPaused,
		TaskEventResumed,
		TaskEventCompleted,
		TaskEventFailed,
		TaskEventStuck,
		TaskEventCancelled,
		TaskEventRetrying,
		TaskEventLog,
		TaskEventResource,
	}

	for _, event := range events {
		assert.NotEmpty(t, event)
		assert.Contains(t, event, "task.")
	}
}
