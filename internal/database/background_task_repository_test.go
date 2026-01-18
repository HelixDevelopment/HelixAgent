package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

// =============================================================================
// Test Helper Functions for BackgroundTask Repository
// =============================================================================

func setupBackgroundTaskTestDB(t *testing.T) (*pgxpool.Pool, *BackgroundTaskRepository) {
	ctx := context.Background()
	connString := getTestDBConnString()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
		return nil, nil
	}

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	repo := NewBackgroundTaskRepository(pool, log)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v", err)
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupBackgroundTaskTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM background_tasks WHERE task_name LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup background_tasks: %v", err)
	}
}

func createTestBackgroundTask() *models.BackgroundTask {
	estimatedDuration := 60
	return &models.BackgroundTask{
		TaskType:                 "test-type",
		TaskName:                 "test-" + time.Now().Format("20060102150405"),
		Payload:                  json.RawMessage(`{"test": "data"}`),
		Priority:                 models.TaskPriorityNormal,
		Status:                   models.TaskStatusPending,
		MaxRetries:               3,
		RetryDelaySeconds:        30,
		RequiredCPUCores:         1,
		RequiredMemoryMB:         256,
		EstimatedDurationSeconds: &estimatedDuration,
		Tags:                     json.RawMessage(`["test", "unit"]`),
		Metadata:                 json.RawMessage(`{"source": "test"}`),
		ScheduledAt:              time.Now(),
		Config:                   models.DefaultTaskConfig(),
		NotificationConfig: models.NotificationConfig{
			OnEvents: []string{models.TaskEventCompleted, models.TaskEventFailed},
		},
	}
}

// =============================================================================
// Unit Tests (no database required)
// =============================================================================

func TestNewBackgroundTaskRepository(t *testing.T) {
	log := logrus.New()
	repo := NewBackgroundTaskRepository(nil, log)
	assert.NotNil(t, repo)
}

func TestBackgroundTaskRepository_NilPool(t *testing.T) {
	log := logrus.New()
	repo := NewBackgroundTaskRepository(nil, log)
	assert.NotNil(t, repo)
}

func TestBackgroundTask_Fields(t *testing.T) {
	now := time.Now()
	workerID := "worker-1"
	processPID := 12345
	correlationID := "corr-123"
	progressMessage := "Processing..."
	estimatedDuration := 120

	task := &models.BackgroundTask{
		ID:                       "task-id",
		TaskType:                 "llm-request",
		TaskName:                 "process-prompt",
		CorrelationID:            &correlationID,
		Payload:                  json.RawMessage(`{"prompt": "test"}`),
		Priority:                 models.TaskPriorityHigh,
		Status:                   models.TaskStatusRunning,
		Progress:                 50.0,
		ProgressMessage:          &progressMessage,
		MaxRetries:               3,
		RetryCount:               1,
		RetryDelaySeconds:        30,
		WorkerID:                 &workerID,
		ProcessPID:               &processPID,
		StartedAt:                &now,
		RequiredCPUCores:         2,
		RequiredMemoryMB:         512,
		EstimatedDurationSeconds: &estimatedDuration,
		Tags:                     json.RawMessage(`["llm", "prompt"]`),
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	assert.Equal(t, "task-id", task.ID)
	assert.Equal(t, "llm-request", task.TaskType)
	assert.Equal(t, "process-prompt", task.TaskName)
	assert.Equal(t, models.TaskPriorityHigh, task.Priority)
	assert.Equal(t, models.TaskStatusRunning, task.Status)
	assert.Equal(t, 50.0, task.Progress)
	assert.Equal(t, "worker-1", *task.WorkerID)
	assert.Equal(t, 12345, *task.ProcessPID)
	assert.Equal(t, 2, task.RequiredCPUCores)
	assert.Equal(t, 512, task.RequiredMemoryMB)
}

func TestTaskStatus_Constants(t *testing.T) {
	assert.Equal(t, models.TaskStatus("pending"), models.TaskStatusPending)
	assert.Equal(t, models.TaskStatus("queued"), models.TaskStatusQueued)
	assert.Equal(t, models.TaskStatus("running"), models.TaskStatusRunning)
	assert.Equal(t, models.TaskStatus("completed"), models.TaskStatusCompleted)
	assert.Equal(t, models.TaskStatus("failed"), models.TaskStatusFailed)
	assert.Equal(t, models.TaskStatus("cancelled"), models.TaskStatusCancelled)
}

func TestTaskPriority_Constants(t *testing.T) {
	assert.Equal(t, models.TaskPriority("critical"), models.TaskPriorityCritical)
	assert.Equal(t, models.TaskPriority("high"), models.TaskPriorityHigh)
	assert.Equal(t, models.TaskPriority("normal"), models.TaskPriorityNormal)
	assert.Equal(t, models.TaskPriority("low"), models.TaskPriorityLow)
	assert.Equal(t, models.TaskPriority("background"), models.TaskPriorityBackground)
}

func TestBackgroundTask_JSONMarshal(t *testing.T) {
	task := createTestBackgroundTask()
	task.ID = "test-marshal-id"

	data, err := json.Marshal(task)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-marshal-id")
	assert.Contains(t, string(data), "test-type")
}

func TestBackgroundTask_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": "task-123",
		"task_type": "llm-request",
		"task_name": "test-task",
		"priority": "high",
		"status": "pending"
	}`

	var task models.BackgroundTask
	err := json.Unmarshal([]byte(jsonData), &task)
	require.NoError(t, err)
	assert.Equal(t, "task-123", task.ID)
	assert.Equal(t, "llm-request", task.TaskType)
	assert.Equal(t, models.TaskPriorityHigh, task.Priority)
}

func TestTaskConfig_Fields(t *testing.T) {
	config := models.TaskConfig{
		TimeoutSeconds:        300,
		HeartbeatIntervalSecs: 10,
		StuckThresholdSecs:    60,
		AllowPause:            true,
		AllowCancel:           true,
		Endless:               false,
		GracefulShutdownSecs:  30,
		CaptureOutput:         true,
		CaptureStderr:         true,
	}

	assert.Equal(t, 300, config.TimeoutSeconds)
	assert.Equal(t, 10, config.HeartbeatIntervalSecs)
	assert.Equal(t, 60, config.StuckThresholdSecs)
	assert.True(t, config.AllowPause)
	assert.True(t, config.AllowCancel)
	assert.False(t, config.Endless)
}

func TestNotificationConfig_Fields(t *testing.T) {
	config := models.NotificationConfig{
		Webhooks: []models.WebhookConfig{
			{
				URL:      "https://example.com/webhook",
				Method:   "POST",
				Events:   []string{models.TaskEventCompleted},
				RetryMax: 3,
			},
		},
		SSE: &models.SSEConfig{
			Channel: "task-updates",
			Enabled: true,
		},
		WebSocket: &models.WSConfig{
			Room:    "task-room",
			Enabled: true,
		},
		OnEvents: []string{models.TaskEventCompleted, models.TaskEventFailed},
	}

	assert.Len(t, config.Webhooks, 1)
	assert.Equal(t, "https://example.com/webhook", config.Webhooks[0].URL)
	assert.NotNil(t, config.SSE)
	assert.True(t, config.SSE.Enabled)
	assert.NotNil(t, config.WebSocket)
	assert.Equal(t, "task-room", config.WebSocket.Room)
	assert.Len(t, config.OnEvents, 2)
}

func TestResourceSnapshot_Fields(t *testing.T) {
	now := time.Now()
	snapshot := &models.ResourceSnapshot{
		ID:             "snapshot-1",
		TaskID:         "task-1",
		CPUPercent:     45.5,
		CPUUserTime:    100.0,
		CPUSystemTime:  25.0,
		MemoryRSSBytes: 1024 * 1024 * 256, // 256MB
		MemoryVMSBytes: 1024 * 1024 * 512, // 512MB
		MemoryPercent:  25.5,
		IOReadBytes:    1024 * 1024,
		IOWriteBytes:   512 * 1024,
		NetBytesSent:   2048,
		NetBytesRecv:   4096,
		OpenFiles:      50,
		OpenFDs:        100,
		ThreadCount:    8,
		ProcessState:   "running",
		SampledAt:      now,
	}

	assert.Equal(t, "snapshot-1", snapshot.ID)
	assert.Equal(t, "task-1", snapshot.TaskID)
	assert.Equal(t, 45.5, snapshot.CPUPercent)
	assert.Equal(t, int64(1024*1024*256), snapshot.MemoryRSSBytes)
	assert.Equal(t, "running", snapshot.ProcessState)
}

func TestTaskExecutionHistory_Fields(t *testing.T) {
	now := time.Now()
	workerID := "worker-1"
	history := &models.TaskExecutionHistory{
		ID:        "history-1",
		TaskID:    "task-1",
		EventType: "started",
		EventData: json.RawMessage(`{"message": "Task started"}`),
		WorkerID:  &workerID,
		CreatedAt: now,
	}

	assert.Equal(t, "history-1", history.ID)
	assert.Equal(t, "task-1", history.TaskID)
	assert.Equal(t, "started", history.EventType)
	assert.Equal(t, "worker-1", *history.WorkerID)
}

func TestBackgroundTask_NilOptionalFields(t *testing.T) {
	task := &models.BackgroundTask{
		ID:       "task-id",
		TaskType: "test-type",
		TaskName: "test-name",
		Priority: models.TaskPriorityNormal,
		Status:   models.TaskStatusPending,
	}

	assert.Nil(t, task.ParentTaskID)
	assert.Nil(t, task.WorkerID)
	assert.Nil(t, task.ProcessPID)
	assert.Nil(t, task.StartedAt)
	assert.Nil(t, task.CompletedAt)
	assert.Nil(t, task.LastHeartbeat)
	assert.Nil(t, task.Deadline)
	assert.Nil(t, task.LastError)
	assert.Nil(t, task.DeletedAt)
}

func TestCreateTestBackgroundTask_ValidValues(t *testing.T) {
	task := createTestBackgroundTask()

	assert.Equal(t, "test-type", task.TaskType)
	assert.Contains(t, task.TaskName, "test-")
	assert.Equal(t, models.TaskPriorityNormal, task.Priority)
	assert.Equal(t, models.TaskStatusPending, task.Status)
	assert.Equal(t, 3, task.MaxRetries)
	assert.Equal(t, 30, task.RetryDelaySeconds)
	assert.Equal(t, 1, task.RequiredCPUCores)
	assert.Equal(t, 256, task.RequiredMemoryMB)
	// Tags is json.RawMessage, verify it's valid JSON array
	var tags []string
	err := json.Unmarshal(task.Tags, &tags)
	require.NoError(t, err)
	assert.Len(t, tags, 2)
	assert.NotNil(t, task.Payload)
	assert.NotNil(t, task.Metadata)
}

// =============================================================================
// Integration Tests (require database)
// =============================================================================

func TestBackgroundTaskRepository_Create(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()

	err := repo.Create(ctx, task)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.UpdatedAt.IsZero())
}

func TestBackgroundTaskRepository_GetByID(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, retrieved.ID)
	assert.Equal(t, task.TaskType, retrieved.TaskType)
	assert.Equal(t, task.TaskName, retrieved.TaskName)
	assert.Equal(t, task.Priority, retrieved.Priority)
	assert.Equal(t, task.Status, retrieved.Status)
}

func TestBackgroundTaskRepository_GetByID_NotFound(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	_, err := repo.GetByID(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBackgroundTaskRepository_Update(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	task.Status = models.TaskStatusRunning
	task.Progress = 50.0
	progressMsg := "Halfway done"
	task.ProgressMessage = &progressMsg
	now := time.Now()
	task.StartedAt = &now
	workerID := "worker-test"
	task.WorkerID = &workerID

	err = repo.Update(ctx, task)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.TaskStatusRunning, retrieved.Status)
	assert.Equal(t, 50.0, retrieved.Progress)
	assert.Equal(t, "Halfway done", *retrieved.ProgressMessage)
	assert.NotNil(t, retrieved.WorkerID)
	assert.Equal(t, "worker-test", *retrieved.WorkerID)
}

func TestBackgroundTaskRepository_UpdateStatus(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	err = repo.UpdateStatus(ctx, task.ID, models.TaskStatusCompleted)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.TaskStatusCompleted, retrieved.Status)
}

func TestBackgroundTaskRepository_UpdateProgress(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	err = repo.UpdateProgress(ctx, task.ID, 75.5, "Processing step 3 of 4")
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, 75.5, retrieved.Progress)
	assert.Equal(t, "Processing step 3 of 4", retrieved.ProgressMessage)
}

func TestBackgroundTaskRepository_UpdateHeartbeat(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	err = repo.UpdateHeartbeat(ctx, task.ID)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.LastHeartbeat)
}

func TestBackgroundTaskRepository_SaveCheckpoint(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	checkpoint := []byte(`{"step": 5, "processed": 500}`)
	err = repo.SaveCheckpoint(ctx, task.ID, checkpoint)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, checkpoint, retrieved.Checkpoint)
}

func TestBackgroundTaskRepository_Delete(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	err = repo.Delete(ctx, task.ID)
	require.NoError(t, err)

	// Task should still exist but with deleted_at set (soft delete)
	retrieved, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.DeletedAt)
}

func TestBackgroundTaskRepository_HardDelete(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()
	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	err = repo.HardDelete(ctx, task.ID)
	require.NoError(t, err)

	// Task should be completely gone
	_, err = repo.GetByID(ctx, task.ID)
	assert.Error(t, err)
}

func TestBackgroundTaskRepository_GetByStatus(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()

	// Create tasks with different statuses
	for i := 0; i < 3; i++ {
		task := createTestBackgroundTask()
		task.Status = models.TaskStatusPending
		err := repo.Create(ctx, task)
		require.NoError(t, err)
	}

	tasks, err := repo.GetByStatus(ctx, models.TaskStatusPending, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tasks), 3)

	for _, task := range tasks {
		assert.Equal(t, models.TaskStatusPending, task.Status)
	}
}

func TestBackgroundTaskRepository_GetPendingTasks(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()

	// Create tasks with different priorities
	priorities := []models.TaskPriority{
		models.TaskPriorityLow,
		models.TaskPriorityCritical,
		models.TaskPriorityNormal,
	}

	for _, priority := range priorities {
		task := createTestBackgroundTask()
		task.Priority = priority
		err := repo.Create(ctx, task)
		require.NoError(t, err)
	}

	tasks, err := repo.GetPendingTasks(ctx, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tasks), 3)

	// First task should be critical priority
	if len(tasks) > 0 && tasks[0].Priority != models.TaskPriorityCritical {
		// May include tasks from other tests, just verify critical comes before low
		for i := 0; i < len(tasks)-1; i++ {
			if tasks[i].Priority == models.TaskPriorityLow && tasks[i+1].Priority == models.TaskPriorityCritical {
				t.Error("Critical priority should come before low priority")
			}
		}
	}
}

func TestBackgroundTaskRepository_CountByStatus(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()

	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	counts, err := repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.NotNil(t, counts)
	assert.GreaterOrEqual(t, counts[models.TaskStatusPending], int64(1))
}

func TestBackgroundTaskRepository_LogEvent(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()

	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	workerID := "worker-test"
	eventData := map[string]interface{}{
		"message":  "Task started",
		"worker":   workerID,
		"priority": "normal",
	}

	err = repo.LogEvent(ctx, task.ID, "started", eventData, &workerID)
	require.NoError(t, err)
}

func TestBackgroundTaskRepository_GetTaskHistory(t *testing.T) {
	pool, repo := setupBackgroundTaskTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupBackgroundTaskTestDB(t, pool)

	ctx := context.Background()

	task := createTestBackgroundTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	// Log some events
	workerID := "worker-test"
	err = repo.LogEvent(ctx, task.ID, "created", map[string]interface{}{"action": "create"}, nil)
	require.NoError(t, err)
	err = repo.LogEvent(ctx, task.ID, "started", map[string]interface{}{"action": "start"}, &workerID)
	require.NoError(t, err)

	history, err := repo.GetTaskHistory(ctx, task.ID, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(history), 2)
}
