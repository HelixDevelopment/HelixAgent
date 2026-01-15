package background

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

// TestInMemoryTaskQueue tests the in-memory task queue implementation
func TestInMemoryTaskQueue_Enqueue(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	task := &models.BackgroundTask{
		TaskType:  "test_task",
		TaskName:  "Test Task",
		Priority:  models.TaskPriorityNormal,
	}

	err := queue.Enqueue(context.Background(), task)
	require.NoError(t, err)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, models.TaskStatusPending, task.Status)
	assert.NotZero(t, task.CreatedAt)
}

func TestInMemoryTaskQueue_EnqueueWithID(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	task := &models.BackgroundTask{
		ID:        "custom-id-123",
		TaskType:  "test_task",
		TaskName:  "Test Task",
		Priority:  models.TaskPriorityHigh,
	}

	err := queue.Enqueue(context.Background(), task)
	require.NoError(t, err)

	assert.Equal(t, "custom-id-123", task.ID)
}

func TestInMemoryTaskQueue_Dequeue(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Enqueue a task
	task := &models.BackgroundTask{
		TaskType:    "test_task",
		TaskName:    "Test Task",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second), // Already scheduled
	}
	err := queue.Enqueue(context.Background(), task)
	require.NoError(t, err)

	// Dequeue the task
	dequeued, err := queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})
	require.NoError(t, err)
	require.NotNil(t, dequeued)

	assert.Equal(t, task.ID, dequeued.ID)
	assert.Equal(t, models.TaskStatusRunning, dequeued.Status)
	assert.NotNil(t, dequeued.WorkerID)
	assert.Equal(t, "worker-1", *dequeued.WorkerID)
	assert.NotNil(t, dequeued.StartedAt)
}

func TestInMemoryTaskQueue_DequeueEmptyQueue(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Dequeue from empty queue
	dequeued, err := queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})
	require.NoError(t, err)
	assert.Nil(t, dequeued)
}

func TestInMemoryTaskQueue_DequeueWithResourceRequirements(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Enqueue a task that requires 4 CPU cores
	task := &models.BackgroundTask{
		TaskType:         "heavy_task",
		TaskName:         "Heavy Task",
		Priority:         models.TaskPriorityNormal,
		RequiredCPUCores: 4,
		RequiredMemoryMB: 1024,
		ScheduledAt:      time.Now().Add(-time.Second),
	}
	err := queue.Enqueue(context.Background(), task)
	require.NoError(t, err)

	// Try to dequeue with insufficient resources
	dequeued, err := queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{
		CPUCores: 2, // Not enough
		MemoryMB: 512,
	})
	require.NoError(t, err)
	assert.Nil(t, dequeued, "Should not dequeue task requiring more resources")

	// Dequeue with sufficient resources
	dequeued, err = queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{
		CPUCores: 8,
		MemoryMB: 2048,
	})
	require.NoError(t, err)
	require.NotNil(t, dequeued)
	assert.Equal(t, task.ID, dequeued.ID)
}

func TestInMemoryTaskQueue_Peek(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Enqueue multiple tasks
	for i := 0; i < 5; i++ {
		task := &models.BackgroundTask{
			TaskType:    "test_task",
			TaskName:    "Test Task",
			Priority:    models.TaskPriorityNormal,
			ScheduledAt: time.Now().Add(-time.Second),
		}
		err := queue.Enqueue(context.Background(), task)
		require.NoError(t, err)
	}

	// Peek at tasks
	peeked, err := queue.Peek(context.Background(), 3)
	require.NoError(t, err)
	assert.Len(t, peeked, 3)

	// Verify tasks are still pending (not claimed)
	for _, task := range peeked {
		assert.Equal(t, models.TaskStatusPending, task.Status)
	}
}

func TestInMemoryTaskQueue_Requeue(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Enqueue and dequeue a task
	task := &models.BackgroundTask{
		TaskType:    "test_task",
		TaskName:    "Test Task",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second),
	}
	err := queue.Enqueue(context.Background(), task)
	require.NoError(t, err)

	dequeued, err := queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})
	require.NoError(t, err)
	require.NotNil(t, dequeued)

	// Requeue the task
	err = queue.Requeue(context.Background(), dequeued.ID, time.Second*5)
	require.NoError(t, err)

	// Verify task is pending again
	requeuedTask := queue.GetTask(dequeued.ID)
	require.NotNil(t, requeuedTask)
	assert.Equal(t, models.TaskStatusPending, requeuedTask.Status)
	assert.Nil(t, requeuedTask.WorkerID)
	assert.Equal(t, 1, requeuedTask.RetryCount)
}

func TestInMemoryTaskQueue_RequeueNonExistent(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	err := queue.Requeue(context.Background(), "non-existent-id", time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

func TestInMemoryTaskQueue_MoveToDeadLetter(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	task := &models.BackgroundTask{
		TaskType:    "test_task",
		TaskName:    "Test Task",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second),
	}
	err := queue.Enqueue(context.Background(), task)
	require.NoError(t, err)

	// Move to dead letter
	err = queue.MoveToDeadLetter(context.Background(), task.ID, "max retries exceeded")
	require.NoError(t, err)

	// Verify status
	deadTask := queue.GetTask(task.ID)
	require.NotNil(t, deadTask)
	assert.Equal(t, models.TaskStatusDeadLetter, deadTask.Status)
	assert.NotNil(t, deadTask.LastError)
	assert.Equal(t, "max retries exceeded", *deadTask.LastError)
}

func TestInMemoryTaskQueue_GetPendingCount(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Enqueue 3 tasks
	for i := 0; i < 3; i++ {
		task := &models.BackgroundTask{
			TaskType:    "test_task",
			TaskName:    "Test Task",
			Priority:    models.TaskPriorityNormal,
			ScheduledAt: time.Now().Add(-time.Second),
		}
		err := queue.Enqueue(context.Background(), task)
		require.NoError(t, err)
	}

	count, err := queue.GetPendingCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestInMemoryTaskQueue_GetRunningCount(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Enqueue and dequeue 2 tasks
	for i := 0; i < 2; i++ {
		task := &models.BackgroundTask{
			TaskType:    "test_task",
			TaskName:    "Test Task",
			Priority:    models.TaskPriorityNormal,
			ScheduledAt: time.Now().Add(-time.Second),
		}
		err := queue.Enqueue(context.Background(), task)
		require.NoError(t, err)

		_, err = queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})
		require.NoError(t, err)
	}

	count, err := queue.GetRunningCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestInMemoryTaskQueue_GetQueueDepth(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Enqueue tasks with different priorities
	priorities := []models.TaskPriority{
		models.TaskPriorityCritical,
		models.TaskPriorityHigh,
		models.TaskPriorityHigh,
		models.TaskPriorityNormal,
		models.TaskPriorityNormal,
		models.TaskPriorityNormal,
		models.TaskPriorityLow,
	}

	for _, priority := range priorities {
		task := &models.BackgroundTask{
			TaskType:    "test_task",
			TaskName:    "Test Task",
			Priority:    priority,
			ScheduledAt: time.Now().Add(-time.Second),
		}
		err := queue.Enqueue(context.Background(), task)
		require.NoError(t, err)
	}

	depth, err := queue.GetQueueDepth(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(1), depth[models.TaskPriorityCritical])
	assert.Equal(t, int64(2), depth[models.TaskPriorityHigh])
	assert.Equal(t, int64(3), depth[models.TaskPriorityNormal])
	assert.Equal(t, int64(1), depth[models.TaskPriorityLow])
}

func TestInMemoryTaskQueue_PriorityOrdering(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	// Enqueue tasks with different priorities (low first)
	task1 := &models.BackgroundTask{
		TaskType:    "test_task",
		TaskName:    "Low Priority",
		Priority:    models.TaskPriorityLow,
		ScheduledAt: time.Now().Add(-time.Second),
	}
	task2 := &models.BackgroundTask{
		TaskType:    "test_task",
		TaskName:    "Critical Priority",
		Priority:    models.TaskPriorityCritical,
		ScheduledAt: time.Now().Add(-time.Second),
	}
	task3 := &models.BackgroundTask{
		TaskType:    "test_task",
		TaskName:    "High Priority",
		Priority:    models.TaskPriorityHigh,
		ScheduledAt: time.Now().Add(-time.Second),
	}

	queue.Enqueue(context.Background(), task1)
	queue.Enqueue(context.Background(), task2)
	queue.Enqueue(context.Background(), task3)

	// Dequeue should return critical first
	dequeued, _ := queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})
	assert.Equal(t, models.TaskPriorityCritical, dequeued.Priority)

	// Then high
	dequeued, _ = queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})
	assert.Equal(t, models.TaskPriorityHigh, dequeued.Priority)

	// Then low
	dequeued, _ = queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})
	assert.Equal(t, models.TaskPriorityLow, dequeued.Priority)
}

// TestDefaultStuckDetector tests
func TestDefaultStuckDetector_NewDefaultStuckDetector(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	assert.NotNil(t, detector)
	assert.NotNil(t, detector.thresholds)
	assert.Equal(t, 5*time.Minute, detector.GetStuckThreshold("default"))
	assert.Equal(t, 3*time.Minute, detector.GetStuckThreshold("command"))
	assert.Equal(t, 10*time.Minute, detector.GetStuckThreshold("debate"))
	assert.Equal(t, time.Duration(0), detector.GetStuckThreshold("endless"))
}

func TestDefaultStuckDetector_GetStuckThreshold(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Known task types
	assert.Equal(t, 3*time.Minute, detector.GetStuckThreshold("command"))
	assert.Equal(t, 3*time.Minute, detector.GetStuckThreshold("llm_call"))
	assert.Equal(t, 2*time.Minute, detector.GetStuckThreshold("embedding"))

	// Unknown task type should return default
	assert.Equal(t, 5*time.Minute, detector.GetStuckThreshold("unknown_type"))
}

func TestDefaultStuckDetector_SetThreshold(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Set custom threshold
	detector.SetThreshold("custom_task", 15*time.Minute)

	assert.Equal(t, 15*time.Minute, detector.GetStuckThreshold("custom_task"))
}

func TestDefaultStuckDetector_IsStuck_NilTask(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	isStuck, reason := detector.IsStuck(context.Background(), nil, nil)

	assert.False(t, isStuck)
	assert.Empty(t, reason)
}

func TestDefaultStuckDetector_IsStuck_StaleHeartbeat(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	staleTime := time.Now().Add(-10 * time.Minute)
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "command",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &staleTime,
	}

	isStuck, reason := detector.IsStuck(context.Background(), task, nil)

	assert.True(t, isStuck)
	assert.Contains(t, reason, "no heartbeat")
}

func TestDefaultStuckDetector_IsStuck_TaskOverdue(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	recentTime := time.Now()
	deadline := time.Now().Add(-time.Hour) // Passed deadline
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "command",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &recentTime,
		Deadline:      &deadline,
	}

	isStuck, reason := detector.IsStuck(context.Background(), task, nil)

	assert.True(t, isStuck)
	assert.Contains(t, reason, "exceeded deadline")
}

func TestDefaultStuckDetector_IsStuck_ProcessFrozen(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	recentTime := time.Now()
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "command",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &recentTime,
	}

	// Create snapshots showing frozen process
	snapshots := []*models.ResourceSnapshot{
		{TaskID: "task-1", CPUPercent: 0.0, CPUUserTime: 1.0, CPUSystemTime: 0.5},
		{TaskID: "task-1", CPUPercent: 0.0, CPUUserTime: 1.0, CPUSystemTime: 0.5},
		{TaskID: "task-1", CPUPercent: 0.0, CPUUserTime: 1.0, CPUSystemTime: 0.5},
		{TaskID: "task-1", CPUPercent: 0.0, CPUUserTime: 1.0, CPUSystemTime: 0.5},
	}

	isStuck, reason := detector.IsStuck(context.Background(), task, snapshots)

	assert.True(t, isStuck)
	assert.Contains(t, reason, "frozen")
}

func TestDefaultStuckDetector_IsStuck_MemoryExhaustion(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	recentTime := time.Now()
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "command",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &recentTime,
	}

	// Create snapshots showing memory exhaustion
	snapshots := []*models.ResourceSnapshot{
		{TaskID: "task-1", CPUPercent: 50.0, MemoryPercent: 96.0, CPUUserTime: 10.0, CPUSystemTime: 5.0},
		{TaskID: "task-1", CPUPercent: 50.0, MemoryPercent: 95.0, CPUUserTime: 9.0, CPUSystemTime: 4.5},
		{TaskID: "task-1", CPUPercent: 50.0, MemoryPercent: 94.0, CPUUserTime: 8.0, CPUSystemTime: 4.0},
	}

	isStuck, reason := detector.IsStuck(context.Background(), task, snapshots)

	assert.True(t, isStuck)
	assert.Contains(t, reason, "memory exhaustion")
}

func TestDefaultStuckDetector_IsStuck_FileDescriptorExhaustion(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	recentTime := time.Now()
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "command",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &recentTime,
	}

	// Create snapshots showing FD exhaustion
	snapshots := []*models.ResourceSnapshot{
		{TaskID: "task-1", CPUPercent: 50.0, OpenFDs: 15000, CPUUserTime: 10.0, CPUSystemTime: 5.0},
		{TaskID: "task-1", CPUPercent: 50.0, OpenFDs: 14000, CPUUserTime: 9.0, CPUSystemTime: 4.5},
		{TaskID: "task-1", CPUPercent: 50.0, OpenFDs: 13000, CPUUserTime: 8.0, CPUSystemTime: 4.0},
	}

	isStuck, reason := detector.IsStuck(context.Background(), task, snapshots)

	assert.True(t, isStuck)
	assert.Contains(t, reason, "file descriptor")
}

func TestDefaultStuckDetector_IsStuck_EndlessTask(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	recentTime := time.Now()
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "endless",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &recentTime,
		Config:        models.TaskConfig{Endless: true},
	}

	// Even with stale heartbeat, endless tasks use different detection
	staleTime := time.Now().Add(-time.Hour)
	task.LastHeartbeat = &staleTime

	isStuck, reason := detector.IsStuck(context.Background(), task, nil)

	assert.False(t, isStuck, "Endless tasks should not be stuck based on heartbeat alone")
	assert.Empty(t, reason)
}

func TestDefaultStuckDetector_IsStuck_EndlessTaskZombie(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	recentTime := time.Now()
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "endless",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &recentTime,
		Config:        models.TaskConfig{Endless: true},
	}

	snapshots := []*models.ResourceSnapshot{
		{TaskID: "task-1", ProcessState: "zombie"},
	}

	isStuck, reason := detector.IsStuck(context.Background(), task, snapshots)

	assert.True(t, isStuck)
	assert.Contains(t, reason, "zombie")
}

func TestDefaultStuckDetector_AnalyzeTask(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	recentTime := time.Now()
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "command",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &recentTime,
	}

	snapshots := []*models.ResourceSnapshot{
		{
			TaskID:         "task-1",
			CPUPercent:     25.0,
			MemoryPercent:  50.0,
			MemoryRSSBytes: 1024 * 1024 * 100,
			OpenFDs:        100,
			ThreadCount:    10,
			IOReadBytes:    1000,
			IOWriteBytes:   500,
			NetConnections: 5,
		},
		{
			TaskID:         "task-1",
			CPUPercent:     20.0,
			MemoryPercent:  48.0,
			MemoryRSSBytes: 1024 * 1024 * 98,
			OpenFDs:        95,
			ThreadCount:    10,
			IOReadBytes:    800,
			IOWriteBytes:   400,
			NetConnections: 5,
		},
	}

	analysis := detector.AnalyzeTask(context.Background(), task, snapshots)

	assert.NotNil(t, analysis)
	assert.False(t, analysis.IsStuck)
	assert.Equal(t, 25.0, analysis.ResourceStatus.CPUPercent)
	assert.Equal(t, 50.0, analysis.ResourceStatus.MemoryPercent)
	assert.True(t, analysis.ActivityStatus.HasCPUActivity)
	assert.True(t, analysis.ActivityStatus.HasIOActivity)
}

func TestDefaultStuckDetector_AnalyzeTask_StuckTask(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	staleTime := time.Now().Add(-10 * time.Minute)
	task := &models.BackgroundTask{
		ID:            "task-1",
		TaskType:      "command",
		Status:        models.TaskStatusRunning,
		LastHeartbeat: &staleTime,
	}

	analysis := detector.AnalyzeTask(context.Background(), task, nil)

	assert.NotNil(t, analysis)
	assert.True(t, analysis.IsStuck)
	assert.NotEmpty(t, analysis.Reason)
	assert.True(t, analysis.HeartbeatStatus.IsStale)
	assert.Contains(t, analysis.Recommendations, "Task may need to be cancelled and restarted")
}

// Test utility functions
func TestMin3(t *testing.T) {
	assert.Equal(t, 3, min3(3, 5))
	assert.Equal(t, 3, min3(5, 3))
	assert.Equal(t, 0, min3(0, 5))
	assert.Equal(t, -1, min3(-1, 5))
}

// TestResourceRequirements tests
func TestResourceRequirements_Default(t *testing.T) {
	req := ResourceRequirements{}

	assert.Equal(t, 0, req.CPUCores)
	assert.Equal(t, 0, req.MemoryMB)
	assert.Equal(t, 0, req.DiskMB)
	assert.Equal(t, 0, req.GPUCount)
}

func TestResourceRequirements_WithValues(t *testing.T) {
	req := ResourceRequirements{
		CPUCores: 4,
		MemoryMB: 8192,
		DiskMB:   10240,
		GPUCount: 1,
		Priority: models.TaskPriorityHigh,
	}

	assert.Equal(t, 4, req.CPUCores)
	assert.Equal(t, 8192, req.MemoryMB)
	assert.Equal(t, 10240, req.DiskMB)
	assert.Equal(t, 1, req.GPUCount)
	assert.Equal(t, models.TaskPriorityHigh, req.Priority)
}

// TestSystemResources tests
func TestSystemResources_Default(t *testing.T) {
	res := SystemResources{}

	assert.Equal(t, 0, res.TotalCPUCores)
	assert.Equal(t, float64(0), res.AvailableCPUCores)
	assert.Equal(t, int64(0), res.TotalMemoryMB)
}

func TestSystemResources_WithValues(t *testing.T) {
	res := SystemResources{
		TotalCPUCores:     8,
		AvailableCPUCores: 6.5,
		TotalMemoryMB:     16384,
		AvailableMemoryMB: 12000,
		CPULoadPercent:    18.75,
		MemoryUsedPercent: 26.76,
		DiskUsedPercent:   45.0,
		LoadAvg1:          1.5,
		LoadAvg5:          2.0,
		LoadAvg15:         1.8,
	}

	assert.Equal(t, 8, res.TotalCPUCores)
	assert.Equal(t, 6.5, res.AvailableCPUCores)
	assert.Equal(t, int64(16384), res.TotalMemoryMB)
	assert.Equal(t, int64(12000), res.AvailableMemoryMB)
	assert.Equal(t, 18.75, res.CPULoadPercent)
}

// TestWorkerStatus tests
func TestWorkerStatus_Default(t *testing.T) {
	status := WorkerStatus{}

	assert.Empty(t, status.ID)
	assert.Empty(t, status.Status)
	assert.Nil(t, status.CurrentTask)
}

func TestWorkerStatus_WithValues(t *testing.T) {
	now := time.Now()
	status := WorkerStatus{
		ID:              "worker-1",
		Status:          "busy",
		StartedAt:       now.Add(-time.Hour),
		LastActivity:    now,
		TasksCompleted:  100,
		TasksFailed:     5,
		AvgTaskDuration: 30 * time.Second,
	}

	assert.Equal(t, "worker-1", status.ID)
	assert.Equal(t, "busy", status.Status)
	assert.Equal(t, int64(100), status.TasksCompleted)
	assert.Equal(t, int64(5), status.TasksFailed)
	assert.Equal(t, 30*time.Second, status.AvgTaskDuration)
}

// TestTaskEvent tests
func TestTaskEvent_Creation(t *testing.T) {
	workerID := "worker-1"
	event := TaskEvent{
		TaskID:    "task-1",
		EventType: "started",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"progress": 0.5,
		},
		WorkerID: &workerID,
	}

	assert.Equal(t, "task-1", event.TaskID)
	assert.Equal(t, "started", event.EventType)
	assert.NotZero(t, event.Timestamp)
	assert.Equal(t, 0.5, event.Data["progress"])
	assert.Equal(t, "worker-1", *event.WorkerID)
}

// TestExecutionResult tests
func TestExecutionResult_Success(t *testing.T) {
	result := ExecutionResult{
		TaskID:     "task-1",
		Status:     models.TaskStatusCompleted,
		Output:     []byte("success output"),
		Duration:   5 * time.Second,
		RetryCount: 0,
	}

	assert.Equal(t, "task-1", result.TaskID)
	assert.Equal(t, models.TaskStatusCompleted, result.Status)
	assert.Equal(t, "success output", string(result.Output))
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.Equal(t, 0, result.RetryCount)
	assert.Empty(t, result.Error)
}

func TestExecutionResult_Failure(t *testing.T) {
	result := ExecutionResult{
		TaskID:     "task-1",
		Status:     models.TaskStatusFailed,
		Error:      "execution failed: timeout",
		Duration:   30 * time.Second,
		RetryCount: 3,
	}

	assert.Equal(t, models.TaskStatusFailed, result.Status)
	assert.Equal(t, "execution failed: timeout", result.Error)
	assert.Equal(t, 3, result.RetryCount)
}

// TestTaskQueueStats tests
func TestTaskQueueStats_MarshalJSON(t *testing.T) {
	stats := &TaskQueueStats{
		PendingCount: 10,
		RunningCount: 5,
		DepthByPriority: map[models.TaskPriority]int64{
			models.TaskPriorityCritical: 2,
			models.TaskPriorityHigh:     3,
			models.TaskPriorityNormal:   5,
		},
		StatusCounts: map[models.TaskStatus]int64{
			models.TaskStatusPending:   10,
			models.TaskStatusRunning:   5,
			models.TaskStatusCompleted: 100,
		},
	}

	data, err := stats.MarshalJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), `"pending_count":10`)
	assert.Contains(t, string(data), `"running_count":5`)
}

// TestDefaultStuckDetectorConfig tests
func TestDefaultStuckDetectorConfig(t *testing.T) {
	config := DefaultStuckDetectorConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 5*time.Minute, config.DefaultThreshold)
	assert.Equal(t, 0.1, config.CPUActivityThreshold)
	assert.Equal(t, 0.5, config.MemoryGrowthThreshold)
	assert.Equal(t, int64(1024), config.IOActivityThreshold)
	assert.Equal(t, 3, config.MinSnapshotsForAnalysis)
}

// TestStuckAnalysis tests
func TestStuckAnalysis_NotStuck(t *testing.T) {
	analysis := &StuckAnalysis{
		IsStuck:         false,
		Recommendations: []string{},
		HeartbeatStatus: HeartbeatStatus{
			IsStale: false,
		},
		ResourceStatus: ResourceStatus{
			IsExhausted: false,
		},
		ActivityStatus: ActivityStatus{
			HasCPUActivity: true,
			HasIOActivity:  true,
		},
	}

	assert.False(t, analysis.IsStuck)
	assert.Empty(t, analysis.Recommendations)
	assert.False(t, analysis.HeartbeatStatus.IsStale)
	assert.True(t, analysis.ActivityStatus.HasCPUActivity)
}

func TestStuckAnalysis_Stuck(t *testing.T) {
	analysis := &StuckAnalysis{
		IsStuck: true,
		Reason:  "no heartbeat for 10m0s",
		Recommendations: []string{
			"Task may need to be cancelled and restarted",
		},
		HeartbeatStatus: HeartbeatStatus{
			IsStale:   true,
			Threshold: 5 * time.Minute,
		},
	}

	assert.True(t, analysis.IsStuck)
	assert.Contains(t, analysis.Reason, "no heartbeat")
	assert.Len(t, analysis.Recommendations, 1)
	assert.True(t, analysis.HeartbeatStatus.IsStale)
}

// TestWaitResult tests
func TestWaitResult_Success(t *testing.T) {
	task := &models.BackgroundTask{
		ID:     "task-1",
		Status: models.TaskStatusCompleted,
	}
	result := WaitResult{
		Task:     task,
		Output:   []byte("output data"),
		Duration: 5 * time.Second,
		Error:    nil,
	}

	assert.NotNil(t, result.Task)
	assert.Equal(t, models.TaskStatusCompleted, result.Task.Status)
	assert.Equal(t, "output data", string(result.Output))
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.Nil(t, result.Error)
}

// Concurrent access tests
func TestInMemoryTaskQueue_ConcurrentEnqueue(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	const numTasks = 100
	done := make(chan struct{})

	for i := 0; i < numTasks; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			task := &models.BackgroundTask{
				TaskType:    "test_task",
				TaskName:    "Test Task",
				Priority:    models.TaskPriorityNormal,
				ScheduledAt: time.Now().Add(-time.Second),
			}
			err := queue.Enqueue(context.Background(), task)
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numTasks; i++ {
		<-done
	}

	count, err := queue.GetPendingCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(numTasks), count)
}

func TestInMemoryTaskQueue_ConcurrentDequeue(t *testing.T) {
	logger := logrus.New()
	queue := NewInMemoryTaskQueue(logger)

	const numTasks = 50
	// Enqueue tasks first
	for i := 0; i < numTasks; i++ {
		task := &models.BackgroundTask{
			TaskType:    "test_task",
			TaskName:    "Test Task",
			Priority:    models.TaskPriorityNormal,
			ScheduledAt: time.Now().Add(-time.Second),
		}
		err := queue.Enqueue(context.Background(), task)
		require.NoError(t, err)
	}

	// Concurrent dequeue
	const numWorkers = 10
	results := make(chan *models.BackgroundTask, numTasks)
	done := make(chan struct{})

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer func() { done <- struct{}{} }()
			for {
				task, err := queue.Dequeue(context.Background(), "worker-"+string(rune(workerID)), ResourceRequirements{})
				if err != nil || task == nil {
					return
				}
				results <- task
			}
		}(w)
	}

	// Wait for workers
	for i := 0; i < numWorkers; i++ {
		<-done
	}
	close(results)

	// Count dequeued tasks - each task should be dequeued exactly once
	dequeued := make(map[string]bool)
	for task := range results {
		assert.False(t, dequeued[task.ID], "Task %s was dequeued twice", task.ID)
		dequeued[task.ID] = true
	}

	assert.Equal(t, numTasks, len(dequeued))
}

// Test isIOStarved function
func TestDefaultStuckDetector_IsIOStarved_InsufficientSnapshots(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0.5, IOReadBytes: 1000, IOWriteBytes: 500},
		{CPUPercent: 0.5, IOReadBytes: 1000, IOWriteBytes: 500},
	}

	starved := detector.isIOStarved(snapshots)
	assert.False(t, starved)
}

func TestDefaultStuckDetector_IsIOStarved_NotStarved(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Snapshots with IO activity
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 10.0, IOReadBytes: 2000, IOWriteBytes: 1500},
		{CPUPercent: 10.0, IOReadBytes: 1500, IOWriteBytes: 1200},
		{CPUPercent: 10.0, IOReadBytes: 1000, IOWriteBytes: 1000},
		{CPUPercent: 10.0, IOReadBytes: 500, IOWriteBytes: 500},
	}

	starved := detector.isIOStarved(snapshots)
	assert.False(t, starved)
}

func TestDefaultStuckDetector_IsIOStarved_Starved(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// CPU active but no IO activity (same values across snapshots)
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0.5, IOReadBytes: 1000, IOWriteBytes: 500},
		{CPUPercent: 0.5, IOReadBytes: 1000, IOWriteBytes: 500},
		{CPUPercent: 0.5, IOReadBytes: 1000, IOWriteBytes: 500},
		{CPUPercent: 0.5, IOReadBytes: 1000, IOWriteBytes: 500},
	}

	starved := detector.isIOStarved(snapshots)
	assert.True(t, starved)
}

func TestDefaultStuckDetector_IsIOStarved_HighCPU(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// High CPU usage - not considered starved even without IO
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 50.0, IOReadBytes: 1000, IOWriteBytes: 500},
		{CPUPercent: 45.0, IOReadBytes: 1000, IOWriteBytes: 500},
		{CPUPercent: 48.0, IOReadBytes: 1000, IOWriteBytes: 500},
		{CPUPercent: 42.0, IOReadBytes: 1000, IOWriteBytes: 500},
	}

	starved := detector.isIOStarved(snapshots)
	assert.False(t, starved)
}

// Test isNetworkHung function
func TestDefaultStuckDetector_IsNetworkHung_InsufficientSnapshots(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0.5, NetConnections: 10},
		{CPUPercent: 0.5, NetConnections: 10},
	}

	hung := detector.isNetworkHung(snapshots)
	assert.False(t, hung)
}

func TestDefaultStuckDetector_IsNetworkHung_NoConnections(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0.5, NetConnections: 0, NetBytesSent: 1000, NetBytesRecv: 2000},
		{CPUPercent: 0.5, NetConnections: 0, NetBytesSent: 1000, NetBytesRecv: 2000},
		{CPUPercent: 0.5, NetConnections: 0, NetBytesSent: 1000, NetBytesRecv: 2000},
		{CPUPercent: 0.5, NetConnections: 0, NetBytesSent: 1000, NetBytesRecv: 2000},
	}

	hung := detector.isNetworkHung(snapshots)
	assert.False(t, hung) // No connections, so can't be network hung
}

func TestDefaultStuckDetector_IsNetworkHung_Hung(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Active connections but no data transfer
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0.5, NetConnections: 10, NetBytesSent: 1000, NetBytesRecv: 2000},
		{CPUPercent: 0.5, NetConnections: 10, NetBytesSent: 1000, NetBytesRecv: 2000},
		{CPUPercent: 0.5, NetConnections: 10, NetBytesSent: 1000, NetBytesRecv: 2000},
		{CPUPercent: 0.5, NetConnections: 10, NetBytesSent: 1000, NetBytesRecv: 2000},
	}

	hung := detector.isNetworkHung(snapshots)
	assert.True(t, hung)
}

func TestDefaultStuckDetector_IsNetworkHung_NotHung(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Active connections with data transfer
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0.5, NetConnections: 10, NetBytesSent: 5000, NetBytesRecv: 10000},
		{CPUPercent: 0.5, NetConnections: 10, NetBytesSent: 4000, NetBytesRecv: 8000},
		{CPUPercent: 0.5, NetConnections: 10, NetBytesSent: 3000, NetBytesRecv: 6000},
		{CPUPercent: 0.5, NetConnections: 10, NetBytesSent: 2000, NetBytesRecv: 4000},
	}

	hung := detector.isNetworkHung(snapshots)
	assert.False(t, hung)
}

// Test hasMemoryLeak function
func TestDefaultStuckDetector_HasMemoryLeak_InsufficientSnapshots(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	snapshots := []*models.ResourceSnapshot{
		{MemoryRSSBytes: 200000000},
		{MemoryRSSBytes: 150000000},
		{MemoryRSSBytes: 100000000},
		{MemoryRSSBytes: 50000000},
	}

	hasLeak := detector.hasMemoryLeak(snapshots)
	assert.False(t, hasLeak) // Need 5+ snapshots
}

func TestDefaultStuckDetector_HasMemoryLeak_NotMonotonicallyIncreasing(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Memory goes up and down
	snapshots := []*models.ResourceSnapshot{
		{MemoryRSSBytes: 150000000},
		{MemoryRSSBytes: 200000000}, // Decrease
		{MemoryRSSBytes: 180000000}, // Increase
		{MemoryRSSBytes: 220000000}, // Decrease
		{MemoryRSSBytes: 100000000},
	}

	hasLeak := detector.hasMemoryLeak(snapshots)
	assert.False(t, hasLeak)
}

func TestDefaultStuckDetector_HasMemoryLeak_SmallGrowthRate(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Memory increases but less than 50%
	snapshots := []*models.ResourceSnapshot{
		{MemoryRSSBytes: 110000000}, // Latest - 110MB
		{MemoryRSSBytes: 108000000},
		{MemoryRSSBytes: 106000000},
		{MemoryRSSBytes: 104000000},
		{MemoryRSSBytes: 102000000},
		{MemoryRSSBytes: 100000000}, // Oldest - 100MB (only 10% growth)
	}

	hasLeak := detector.hasMemoryLeak(snapshots)
	assert.False(t, hasLeak) // Growth rate too small
}

func TestDefaultStuckDetector_HasMemoryLeak_Detected(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Memory monotonically increasing with >50% growth
	snapshots := []*models.ResourceSnapshot{
		{MemoryRSSBytes: 200000000}, // Latest - 200MB
		{MemoryRSSBytes: 180000000},
		{MemoryRSSBytes: 160000000},
		{MemoryRSSBytes: 140000000},
		{MemoryRSSBytes: 120000000}, // Oldest - 120MB (67% growth)
	}

	hasLeak := detector.hasMemoryLeak(snapshots)
	assert.True(t, hasLeak)
}

func TestDefaultStuckDetector_HasMemoryLeak_ZeroOldestMemory(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// Edge case - oldest memory is 0
	snapshots := []*models.ResourceSnapshot{
		{MemoryRSSBytes: 200000000},
		{MemoryRSSBytes: 180000000},
		{MemoryRSSBytes: 160000000},
		{MemoryRSSBytes: 140000000},
		{MemoryRSSBytes: 0}, // Oldest - 0
	}

	hasLeak := detector.hasMemoryLeak(snapshots)
	assert.False(t, hasLeak) // Can't calculate growth with 0 base
}

// Test isEndlessTaskStuck with more scenarios
func TestDefaultStuckDetector_IsEndlessTaskStuck_NoSnapshots(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	task := &models.BackgroundTask{
		ID:       "test-endless-nosnapshots",
		TaskType: "endless",
		Config:   models.TaskConfig{Endless: true},
	}

	isStuck, reason := detector.isEndlessTaskStuck(task, nil)
	assert.False(t, isStuck)
	assert.Empty(t, reason)
}

func TestDefaultStuckDetector_IsEndlessTaskStuck_CriticalMemory(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	task := &models.BackgroundTask{
		ID:       "test-endless-mem",
		TaskType: "endless",
		Config:   models.TaskConfig{Endless: true},
	}

	snapshots := []*models.ResourceSnapshot{
		{MemoryPercent: 99.0},
	}

	isStuck, reason := detector.isEndlessTaskStuck(task, snapshots)
	assert.True(t, isStuck)
	assert.Contains(t, reason, "critical memory")
}

func TestDefaultStuckDetector_IsEndlessTaskStuck_NoActivity(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	task := &models.BackgroundTask{
		ID:       "test-endless-noact",
		TaskType: "endless",
		Config:   models.TaskConfig{Endless: true},
	}

	// 5 snapshots with no activity
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 50},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 50},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 50},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 50},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 50},
	}

	isStuck, reason := detector.isEndlessTaskStuck(task, snapshots)
	assert.True(t, isStuck)
	assert.Contains(t, reason, "no activity")
}

func TestDefaultStuckDetector_IsEndlessTaskStuck_WithIOActivity(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	task := &models.BackgroundTask{
		ID:       "test-endless-ioact",
		TaskType: "endless",
		Config:   models.TaskConfig{Endless: true},
	}

	// 5 snapshots with IO activity (different read bytes)
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0, IOReadBytes: 500, IOWriteBytes: 50},
		{CPUPercent: 0, IOReadBytes: 400, IOWriteBytes: 50},
		{CPUPercent: 0, IOReadBytes: 300, IOWriteBytes: 50},
		{CPUPercent: 0, IOReadBytes: 200, IOWriteBytes: 50},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 50},
	}

	isStuck, reason := detector.isEndlessTaskStuck(task, snapshots)
	assert.False(t, isStuck)
	assert.Empty(t, reason)
}

func TestDefaultStuckDetector_IsEndlessTaskStuck_WithWriteActivity(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	task := &models.BackgroundTask{
		ID:       "test-endless-writeact",
		TaskType: "endless",
		Config:   models.TaskConfig{Endless: true},
	}

	// 5 snapshots with write activity (different write bytes)
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 250},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 200},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 150},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 100},
		{CPUPercent: 0, IOReadBytes: 100, IOWriteBytes: 50},
	}

	isStuck, reason := detector.isEndlessTaskStuck(task, snapshots)
	assert.False(t, isStuck)
	assert.Empty(t, reason)
}

// Test checkResourceExhaustion with empty snapshots
func TestDefaultStuckDetector_CheckResourceExhaustion_EmptySnapshots(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	reason := detector.checkResourceExhaustion(nil)
	assert.Empty(t, reason)

	reason = detector.checkResourceExhaustion([]*models.ResourceSnapshot{})
	assert.Empty(t, reason)
}

// Test isProcessFrozen with CPU time increasing
func TestDefaultStuckDetector_IsProcessFrozen_CPUTimeIncreasing(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	// CPU percent is 0 but CPU time is increasing
	snapshots := []*models.ResourceSnapshot{
		{CPUPercent: 0.0, CPUUserTime: 1.0, CPUSystemTime: 0.5},
		{CPUPercent: 0.0, CPUUserTime: 0.9, CPUSystemTime: 0.4},
		{CPUPercent: 0.0, CPUUserTime: 0.8, CPUSystemTime: 0.3},
		{CPUPercent: 0.0, CPUUserTime: 0.7, CPUSystemTime: 0.2},
		{CPUPercent: 0.0, CPUUserTime: 0.6, CPUSystemTime: 0.1},
	}

	frozen := detector.isProcessFrozen(snapshots)
	assert.False(t, frozen) // Not frozen because CPU time is increasing
}

// Test checkHeartbeatTimeout with custom threshold
func TestDefaultStuckDetector_CheckHeartbeatTimeout_CustomThreshold(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	staleTime := time.Now().Add(-30 * time.Second)
	task := &models.BackgroundTask{
		ID:            "test-custom-threshold",
		TaskType:      "command",
		LastHeartbeat: &staleTime,
		Config: models.TaskConfig{
			StuckThresholdSecs: 10, // 10 second threshold
		},
	}

	reason := detector.checkHeartbeatTimeout(task)
	assert.Contains(t, reason, "no heartbeat for")
}

// Test checkHeartbeatTimeout with endless task
func TestDefaultStuckDetector_CheckHeartbeatTimeout_EndlessTask(t *testing.T) {
	logger := logrus.New()
	detector := NewDefaultStuckDetector(logger)

	staleTime := time.Now().Add(-1 * time.Hour)
	task := &models.BackgroundTask{
		ID:            "test-endless-timeout",
		TaskType:      "endless",
		LastHeartbeat: &staleTime,
		Config:        models.TaskConfig{},
	}

	reason := detector.checkHeartbeatTimeout(task)
	assert.Empty(t, reason) // Endless tasks have 0 threshold
}

// ============================================================================
// ProcessResourceMonitor Tests
// ============================================================================

func TestNewProcessResourceMonitor(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.monitors)
	assert.Equal(t, 2*time.Second, monitor.cacheTTL)
}

func TestProcessResourceMonitor_GetSystemResources(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	resources, err := monitor.GetSystemResources()
	require.NoError(t, err)
	require.NotNil(t, resources)

	// Validate system resources are reasonable
	assert.Greater(t, resources.TotalCPUCores, 0)
	assert.GreaterOrEqual(t, resources.AvailableCPUCores, float64(0))
	assert.Greater(t, resources.TotalMemoryMB, int64(0))
	assert.GreaterOrEqual(t, resources.AvailableMemoryMB, int64(0))
	assert.GreaterOrEqual(t, resources.CPULoadPercent, float64(0))
	assert.LessOrEqual(t, resources.CPULoadPercent, float64(100))
	assert.GreaterOrEqual(t, resources.MemoryUsedPercent, float64(0))
	assert.LessOrEqual(t, resources.MemoryUsedPercent, float64(100))
}

func TestProcessResourceMonitor_GetSystemResources_Caching(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	// First call - populates cache
	resources1, err := monitor.GetSystemResources()
	require.NoError(t, err)
	require.NotNil(t, resources1)

	// Second call - should return cached value
	resources2, err := monitor.GetSystemResources()
	require.NoError(t, err)
	require.NotNil(t, resources2)

	// Values should be the same (from cache)
	assert.Equal(t, resources1.TotalCPUCores, resources2.TotalCPUCores)
	assert.Equal(t, resources1.TotalMemoryMB, resources2.TotalMemoryMB)
}

func TestProcessResourceMonitor_GetProcessResources(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	// Use current process PID
	pid := os.Getpid()

	snapshot, err := monitor.GetProcessResources(pid)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	// Validate snapshot fields
	assert.GreaterOrEqual(t, snapshot.CPUPercent, float64(0))
	assert.GreaterOrEqual(t, snapshot.MemoryRSSBytes, int64(0))
	assert.GreaterOrEqual(t, snapshot.MemoryPercent, float64(0))
	assert.NotZero(t, snapshot.SampledAt)
}

func TestProcessResourceMonitor_GetProcessResources_InvalidPID(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	// Use invalid PID
	_, err := monitor.GetProcessResources(999999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "process not found")
}

func TestProcessResourceMonitor_StartStopMonitoring(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	taskID := "test-task-monitor"
	pid := os.Getpid()

	// Start monitoring
	err := monitor.StartMonitoring(taskID, pid, 50*time.Millisecond)
	require.NoError(t, err)

	// Verify monitoring started
	monitor.mu.RLock()
	_, exists := monitor.monitors[taskID]
	monitor.mu.RUnlock()
	assert.True(t, exists)

	// Wait for at least one snapshot
	time.Sleep(100 * time.Millisecond)

	// Stop monitoring
	err = monitor.StopMonitoring(taskID)
	require.NoError(t, err)

	// Verify monitoring stopped
	monitor.mu.RLock()
	_, exists = monitor.monitors[taskID]
	monitor.mu.RUnlock()
	assert.False(t, exists)
}

func TestProcessResourceMonitor_StartMonitoring_AlreadyMonitoring(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	taskID := "test-task-duplicate"
	pid := os.Getpid()

	// Start monitoring first time
	err := monitor.StartMonitoring(taskID, pid, 100*time.Millisecond)
	require.NoError(t, err)
	defer monitor.StopMonitoring(taskID)

	// Try to start monitoring again
	err = monitor.StartMonitoring(taskID, pid, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "monitoring already started")
}

func TestProcessResourceMonitor_StopMonitoring_NotMonitoring(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	// Stop monitoring a task that was never started
	err := monitor.StopMonitoring("nonexistent-task")
	assert.NoError(t, err) // Should not error
}

func TestProcessResourceMonitor_GetLatestSnapshot(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	taskID := "test-task-snapshot"
	pid := os.Getpid()

	// Start monitoring
	err := monitor.StartMonitoring(taskID, pid, 50*time.Millisecond)
	require.NoError(t, err)
	defer monitor.StopMonitoring(taskID)

	// Wait for snapshot to be taken
	time.Sleep(100 * time.Millisecond)

	// Get latest snapshot
	snapshot, err := monitor.GetLatestSnapshot(taskID)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	assert.Equal(t, taskID, snapshot.TaskID)
	assert.GreaterOrEqual(t, snapshot.CPUPercent, float64(0))
}

func TestProcessResourceMonitor_GetLatestSnapshot_NotMonitoring(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	_, err := monitor.GetLatestSnapshot("nonexistent-task")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no monitoring active")
}

func TestProcessResourceMonitor_GetLatestSnapshot_NoSnapshot(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	taskID := "test-task-no-snapshot"
	pid := os.Getpid()

	// Start monitoring but don't wait
	err := monitor.StartMonitoring(taskID, pid, 10*time.Second) // Long interval
	require.NoError(t, err)
	defer monitor.StopMonitoring(taskID)

	// Try to get snapshot immediately (before first tick)
	_, err = monitor.GetLatestSnapshot(taskID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no snapshot available")
}

func TestProcessResourceMonitor_IsResourceAvailable(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	// Get current system resources to check load
	resources, err := monitor.GetSystemResources()
	require.NoError(t, err)

	// Test with zero requirements (should be available unless system is heavily loaded)
	available := monitor.IsResourceAvailable(ResourceRequirements{})
	// If system load is > 90%, IsResourceAvailable will return false regardless of requirements
	assert.True(t, available || resources.CPULoadPercent > 90 || resources.MemoryUsedPercent > 90)
}

func TestProcessResourceMonitor_IsResourceAvailable_WithRequirements(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	// Get current resources to understand limits
	resources, err := monitor.GetSystemResources()
	require.NoError(t, err)

	// Test with achievable requirements
	available := monitor.IsResourceAvailable(ResourceRequirements{
		CPUCores: 1,
		MemoryMB: 100,
	})
	// Should likely be true on most systems
	assert.True(t, available || resources.CPULoadPercent > 90 || resources.MemoryUsedPercent > 90)
}

func TestProcessResourceMonitor_IsResourceAvailable_ExcessiveCPU(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	// Test with excessive CPU requirements
	available := monitor.IsResourceAvailable(ResourceRequirements{
		CPUCores: 10000, // Excessive
	})
	assert.False(t, available)
}

func TestProcessResourceMonitor_IsResourceAvailable_ExcessiveMemory(t *testing.T) {
	logger := logrus.New()
	monitor := NewProcessResourceMonitor(nil, logger)

	// Test with excessive memory requirements
	available := monitor.IsResourceAvailable(ResourceRequirements{
		MemoryMB: 1000000000, // 1 petabyte
	})
	assert.False(t, available)
}
