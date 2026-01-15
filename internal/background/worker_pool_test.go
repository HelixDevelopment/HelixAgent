package background

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/models"
)

func TestDefaultWorkerPoolConfig(t *testing.T) {
	config := DefaultWorkerPoolConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 2, config.MinWorkers)
	assert.GreaterOrEqual(t, config.MaxWorkers, 2) // At least num CPU * 2
	assert.Equal(t, 0.7, config.ScaleUpThreshold)
	assert.Equal(t, 0.3, config.ScaleDownThreshold)
	assert.Equal(t, 30*time.Second, config.ScaleInterval)
	assert.Equal(t, 5*time.Minute, config.WorkerIdleTimeout)
	assert.Equal(t, 1*time.Second, config.QueuePollInterval)
	assert.Equal(t, 10*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 5*time.Second, config.ResourceCheckInterval)
	assert.Equal(t, 80.0, config.MaxCPUPercent)
	assert.Equal(t, 80.0, config.MaxMemoryPercent)
	assert.Equal(t, 30*time.Second, config.GracefulShutdownTime)
}

func TestWorkerPoolConfig_CustomValues(t *testing.T) {
	config := &WorkerPoolConfig{
		MinWorkers:            4,
		MaxWorkers:            16,
		ScaleUpThreshold:      0.8,
		ScaleDownThreshold:    0.2,
		ScaleInterval:         1 * time.Minute,
		WorkerIdleTimeout:     10 * time.Minute,
		QueuePollInterval:     500 * time.Millisecond,
		HeartbeatInterval:     30 * time.Second,
		ResourceCheckInterval: 10 * time.Second,
		MaxCPUPercent:         90.0,
		MaxMemoryPercent:      85.0,
		GracefulShutdownTime:  1 * time.Minute,
	}

	assert.Equal(t, 4, config.MinWorkers)
	assert.Equal(t, 16, config.MaxWorkers)
	assert.Equal(t, 0.8, config.ScaleUpThreshold)
	assert.Equal(t, 0.2, config.ScaleDownThreshold)
	assert.Equal(t, 1*time.Minute, config.ScaleInterval)
	assert.Equal(t, 10*time.Minute, config.WorkerIdleTimeout)
}

func TestWorkerPoolConfig_ZeroValues(t *testing.T) {
	config := &WorkerPoolConfig{}

	assert.Equal(t, 0, config.MinWorkers)
	assert.Equal(t, 0, config.MaxWorkers)
	assert.Equal(t, 0.0, config.ScaleUpThreshold)
	assert.Equal(t, time.Duration(0), config.ScaleInterval)
}

func TestWorkerState_String(t *testing.T) {
	tests := []struct {
		state    workerState
		expected string
	}{
		{workerStateIdle, "idle"},
		{workerStateBusy, "busy"},
		{workerStateStopping, "stopping"},
		{workerStateStopped, "stopped"},
		{workerState(999), "unknown"},
		{workerState(-1), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.state.String())
		})
	}
}

func TestWorkerState_Constants(t *testing.T) {
	// Verify constants have expected values
	assert.Equal(t, workerState(0), workerStateIdle)
	assert.Equal(t, workerState(1), workerStateBusy)
	assert.Equal(t, workerState(2), workerStateStopping)
	assert.Equal(t, workerState(3), workerStateStopped)
}

func TestMin_ThreeArgs(t *testing.T) {
	assert.Equal(t, 1, min(1, 2, 3))
	assert.Equal(t, 1, min(2, 1, 3))
	assert.Equal(t, 1, min(3, 2, 1))
	assert.Equal(t, 0, min(0, 5, 10))
	assert.Equal(t, -5, min(-5, 0, 5))
	assert.Equal(t, -10, min(-5, -10, 5))
	assert.Equal(t, 5, min(5, 5, 5))
}

func TestIsTerminalStatus(t *testing.T) {
	tests := []struct {
		status   models.TaskStatus
		terminal bool
	}{
		{models.TaskStatusPending, false},
		{models.TaskStatusQueued, false},
		{models.TaskStatusRunning, false},
		{models.TaskStatusCompleted, true},
		{models.TaskStatusFailed, true},
		{models.TaskStatusCancelled, true},
		{models.TaskStatusDeadLetter, true},
		{models.TaskStatusStuck, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			assert.Equal(t, tc.terminal, isTerminalStatus(tc.status))
		})
	}
}

func TestWorker_Fields(t *testing.T) {
	now := time.Now()
	worker := &Worker{
		ID:             "worker-123",
		Status:         workerStateIdle,
		StartedAt:      now,
		LastActivity:   now,
		TasksCompleted: 10,
		TasksFailed:    2,
		TotalDuration:  5 * time.Minute,
	}

	assert.Equal(t, "worker-123", worker.ID)
	assert.Equal(t, workerStateIdle, worker.Status)
	assert.Equal(t, int64(10), worker.TasksCompleted)
	assert.Equal(t, int64(2), worker.TasksFailed)
	assert.Equal(t, 5*time.Minute, worker.TotalDuration)
}

func TestWorker_WithTask(t *testing.T) {
	task := &models.BackgroundTask{
		ID:       "task-1",
		TaskType: "command",
	}

	worker := &Worker{
		ID:          "worker-1",
		Status:      workerStateBusy,
		CurrentTask: task,
	}

	assert.Equal(t, workerStateBusy, worker.Status)
	assert.NotNil(t, worker.CurrentTask)
	assert.Equal(t, "task-1", worker.CurrentTask.ID)
}

func TestWorker_ZeroValues(t *testing.T) {
	worker := &Worker{}

	assert.Empty(t, worker.ID)
	assert.Equal(t, workerStateIdle, worker.Status) // 0 value is idle
	assert.Nil(t, worker.CurrentTask)
	assert.True(t, worker.StartedAt.IsZero())
	assert.True(t, worker.LastActivity.IsZero())
	assert.Equal(t, int64(0), worker.TasksCompleted)
	assert.Equal(t, int64(0), worker.TasksFailed)
}

func TestWorkerStatus_Fields(t *testing.T) {
	now := time.Now()
	status := WorkerStatus{
		ID:              "worker-abc",
		Status:          "busy",
		StartedAt:       now.Add(-time.Hour),
		LastActivity:    now,
		TasksCompleted:  50,
		TasksFailed:     3,
		AvgTaskDuration: 15 * time.Second,
	}

	assert.NotEmpty(t, status.ID)
	assert.Equal(t, "busy", status.Status)
	assert.Equal(t, int64(50), status.TasksCompleted)
	assert.Equal(t, int64(3), status.TasksFailed)
	assert.Equal(t, 15*time.Second, status.AvgTaskDuration)
}

func TestWorkerStatus_WithCurrentTask(t *testing.T) {
	task := &models.BackgroundTask{
		ID:       "task-123",
		TaskType: "test_task",
		TaskName: "Test Task",
	}

	status := WorkerStatus{
		ID:          "worker-1",
		Status:      "busy",
		CurrentTask: task,
	}

	assert.NotNil(t, status.CurrentTask)
	assert.Equal(t, "task-123", status.CurrentTask.ID)
	assert.Equal(t, "test_task", status.CurrentTask.TaskType)
}

func TestWorkerStatus_Empty(t *testing.T) {
	status := WorkerStatus{}

	assert.Empty(t, status.ID)
	assert.Empty(t, status.Status)
	assert.Nil(t, status.CurrentTask)
	assert.True(t, status.StartedAt.IsZero())
	assert.True(t, status.LastActivity.IsZero())
	assert.Equal(t, int64(0), status.TasksCompleted)
	assert.Equal(t, int64(0), status.TasksFailed)
	assert.Equal(t, time.Duration(0), status.AvgTaskDuration)
}

// Test worker state transitions
func TestWorkerState_Transitions(t *testing.T) {
	worker := &Worker{
		ID: "test-worker",
	}

	worker.Status = workerStateIdle
	assert.Equal(t, "idle", worker.Status.String())

	worker.Status = workerStateBusy
	assert.Equal(t, "busy", worker.Status.String())

	worker.Status = workerStateStopping
	assert.Equal(t, "stopping", worker.Status.String())

	worker.Status = workerStateStopped
	assert.Equal(t, "stopped", worker.Status.String())
}

// Test TaskEvent struct
func TestTaskEvent_Fields(t *testing.T) {
	workerID := "worker-1"
	event := TaskEvent{
		TaskID:    "task-1",
		EventType: "completed",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"duration_ms": 1500,
		},
		WorkerID: &workerID,
	}

	assert.Equal(t, "task-1", event.TaskID)
	assert.Equal(t, "completed", event.EventType)
	assert.NotZero(t, event.Timestamp)
	assert.NotNil(t, event.Data)
	assert.Equal(t, 1500, event.Data["duration_ms"])
	assert.NotNil(t, event.WorkerID)
	assert.Equal(t, "worker-1", *event.WorkerID)
}

func TestTaskEvent_NilWorkerID(t *testing.T) {
	event := TaskEvent{
		TaskID:    "task-1",
		EventType: "started",
	}

	assert.Nil(t, event.WorkerID)
}

// Test ExecutionResult struct
func TestExecutionResult_Success_WorkerPool(t *testing.T) {
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

func TestExecutionResult_Failure_WorkerPool(t *testing.T) {
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

func TestExecutionResult_WithResourceMetrics(t *testing.T) {
	snapshot := &models.ResourceSnapshot{
		TaskID:     "task-1",
		CPUPercent: 50.0,
	}

	result := ExecutionResult{
		TaskID:          "task-1",
		Status:          models.TaskStatusCompleted,
		ResourceMetrics: snapshot,
	}

	assert.NotNil(t, result.ResourceMetrics)
	assert.Equal(t, "task-1", result.ResourceMetrics.TaskID)
	assert.Equal(t, 50.0, result.ResourceMetrics.CPUPercent)
}

// Test WaitResult struct
func TestWaitResult_Success_WorkerPool(t *testing.T) {
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

func TestWaitResult_WithError(t *testing.T) {
	result := WaitResult{
		Task:     nil,
		Duration: 30 * time.Second,
		Error:    assert.AnError,
	}

	assert.Nil(t, result.Task)
	assert.NotNil(t, result.Error)
}

// Test ResourceRequirements struct
func TestResourceRequirements_Fields(t *testing.T) {
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

func TestResourceRequirements_ZeroValues(t *testing.T) {
	req := ResourceRequirements{}

	assert.Equal(t, 0, req.CPUCores)
	assert.Equal(t, 0, req.MemoryMB)
	assert.Equal(t, 0, req.DiskMB)
	assert.Equal(t, 0, req.GPUCount)
}

// Test SystemResources struct
func TestSystemResources_Fields_WorkerPool(t *testing.T) {
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
	assert.Equal(t, 26.76, res.MemoryUsedPercent)
}

func TestSystemResources_ZeroValues(t *testing.T) {
	res := SystemResources{}

	assert.Equal(t, 0, res.TotalCPUCores)
	assert.Equal(t, 0.0, res.AvailableCPUCores)
	assert.Equal(t, int64(0), res.TotalMemoryMB)
}
