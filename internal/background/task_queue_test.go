package background

import (
	"context"
	"errors"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

// mockTaskRepository is a test implementation of TaskRepository.
//
// Concurrency model (CONST-029): all three maps migrate to *safe.Store.
// Per-task mutations happen under Store.Update for read-modify-write
// atomicity.
type mockTaskRepository struct {
	tasks        *safe.Store[string, *models.BackgroundTask]
	history      *safe.Store[string, []*models.TaskExecutionHistory]
	createErr    error
	getByIDErr   error
	updateErr    error
	deleteErr    error
	dequeueErr   error
	countErr     error
	statusCounts *safe.Store[models.TaskStatus, int64]
}

func newMockTaskRepository() *mockTaskRepository {
	return &mockTaskRepository{
		tasks:        safe.NewStore[string, *models.BackgroundTask](),
		history:      safe.NewStore[string, []*models.TaskExecutionHistory](),
		statusCounts: safe.NewStore[models.TaskStatus, int64](),
	}
}

func (m *mockTaskRepository) Create(ctx context.Context, task *models.BackgroundTask) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.tasks.Put(task.ID, task)
	m.updateStatusCount(task.Status, 1)
	return nil
}

func (m *mockTaskRepository) GetByID(ctx context.Context, id string) (*models.BackgroundTask, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	task, exists := m.tasks.Get(id)
	if !exists {
		return nil, errors.New("task not found")
	}
	return task, nil
}

func (m *mockTaskRepository) Update(ctx context.Context, task *models.BackgroundTask) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.tasks.Put(task.ID, task)
	return nil
}

func (m *mockTaskRepository) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.tasks.Delete(id)
	return nil
}

func (m *mockTaskRepository) UpdateStatus(ctx context.Context, id string, status models.TaskStatus) error {
	m.tasks.Update(id, func(cur *models.BackgroundTask, present bool) (*models.BackgroundTask, bool) {
		if !present {
			return cur, false
		}
		cur.Status = status
		return cur, true
	})
	return nil
}

func (m *mockTaskRepository) UpdateProgress(ctx context.Context, id string, progress float64, message string) error {
	m.tasks.Update(id, func(cur *models.BackgroundTask, present bool) (*models.BackgroundTask, bool) {
		if !present {
			return cur, false
		}
		cur.Progress = progress
		cur.ProgressMessage = &message
		return cur, true
	})
	return nil
}

func (m *mockTaskRepository) UpdateHeartbeat(ctx context.Context, id string) error {
	m.tasks.Update(id, func(cur *models.BackgroundTask, present bool) (*models.BackgroundTask, bool) {
		if !present {
			return cur, false
		}
		now := time.Now()
		cur.LastHeartbeat = &now
		return cur, true
	})
	return nil
}

func (m *mockTaskRepository) SaveCheckpoint(ctx context.Context, id string, checkpoint []byte) error {
	return nil
}

func (m *mockTaskRepository) GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.BackgroundTask, error) {
	var result []*models.BackgroundTask
	m.tasks.Range(func(_ string, task *models.BackgroundTask) bool {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
		return true
	})
	return result, nil
}

func (m *mockTaskRepository) GetPendingTasks(ctx context.Context, limit int) ([]*models.BackgroundTask, error) {
	return m.GetByStatus(ctx, models.TaskStatusPending, limit, 0)
}

func (m *mockTaskRepository) GetStaleTasks(ctx context.Context, threshold time.Duration) ([]*models.BackgroundTask, error) {
	return nil, nil
}

func (m *mockTaskRepository) GetByWorkerID(ctx context.Context, workerID string) ([]*models.BackgroundTask, error) {
	var result []*models.BackgroundTask
	m.tasks.Range(func(_ string, task *models.BackgroundTask) bool {
		if task.WorkerID != nil && *task.WorkerID == workerID {
			result = append(result, task)
		}
		return true
	})
	return result, nil
}

func (m *mockTaskRepository) CountByStatus(ctx context.Context) (map[models.TaskStatus]int64, error) {
	if m.countErr != nil {
		return nil, m.countErr
	}
	counts := make(map[models.TaskStatus]int64)
	m.tasks.Range(func(_ string, task *models.BackgroundTask) bool {
		counts[task.Status]++
		return true
	})
	return counts, nil
}

func (m *mockTaskRepository) Dequeue(ctx context.Context, workerID string, maxCPUCores, maxMemoryMB int) (*models.BackgroundTask, error) {
	if m.dequeueErr != nil {
		return nil, m.dequeueErr
	}
	var picked *models.BackgroundTask
	m.tasks.Range(func(_ string, task *models.BackgroundTask) bool {
		if task.Status == models.TaskStatusPending {
			task.Status = models.TaskStatusRunning
			task.WorkerID = &workerID
			now := time.Now()
			task.StartedAt = &now
			picked = task
			return false
		}
		return true
	})
	return picked, nil
}

func (m *mockTaskRepository) SaveResourceSnapshot(ctx context.Context, snapshot *models.ResourceSnapshot) error {
	return nil
}

func (m *mockTaskRepository) GetResourceSnapshots(ctx context.Context, taskID string, limit int) ([]*models.ResourceSnapshot, error) {
	return nil, nil
}

func (m *mockTaskRepository) LogEvent(ctx context.Context, taskID, eventType string, data map[string]interface{}, workerID *string) error {
	m.history.Update(taskID, func(cur []*models.TaskExecutionHistory, _ bool) ([]*models.TaskExecutionHistory, bool) {
		return append(cur, &models.TaskExecutionHistory{
			TaskID:    taskID,
			EventType: eventType,
			WorkerID:  workerID,
			CreatedAt: time.Now(),
		}), true
	})
	return nil
}

func (m *mockTaskRepository) GetTaskHistory(ctx context.Context, taskID string, limit int) ([]*models.TaskExecutionHistory, error) {
	h, _ := m.history.Get(taskID)
	return h, nil
}

func (m *mockTaskRepository) MoveToDeadLetter(ctx context.Context, taskID, reason string) error {
	m.tasks.Update(taskID, func(cur *models.BackgroundTask, present bool) (*models.BackgroundTask, bool) {
		if !present {
			return cur, false
		}
		cur.Status = models.TaskStatusDeadLetter
		cur.LastError = &reason
		return cur, true
	})
	return nil
}

func (m *mockTaskRepository) updateStatusCount(status models.TaskStatus, delta int64) {
	m.statusCounts.Update(status, func(cur int64, _ bool) (int64, bool) {
		return cur + delta, true
	})
}

// TestNewPostgresTaskQueue tests the constructor
func TestNewPostgresTaskQueue(t *testing.T) {
	repo := newMockTaskRepository()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	queue := NewPostgresTaskQueue(repo, logger)

	assert.NotNil(t, queue)
	assert.NotNil(t, queue.repository)
	assert.NotNil(t, queue.logger)

}

// TestPostgresTaskQueue_Enqueue tests the Enqueue method
func TestPostgresTaskQueue_Enqueue(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Enqueues task successfully", func(t *testing.T) {
		repo := newMockTaskRepository()
		queue := NewPostgresTaskQueue(repo, logger)

		task := &models.BackgroundTask{
			ID:       "test-task-id",
			TaskType: "test-type",
			TaskName: "Test Task",
		}

		err := queue.Enqueue(context.Background(), task)

		require.NoError(t, err)
		assert.Equal(t, models.TaskStatusPending, task.Status)
		assert.Equal(t, models.TaskPriorityNormal, task.Priority)
		assert.False(t, task.ScheduledAt.IsZero())
	})

	t.Run("Returns error for nil task", func(t *testing.T) {
		repo := newMockTaskRepository()
		queue := NewPostgresTaskQueue(repo, logger)

		err := queue.Enqueue(context.Background(), nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task cannot be nil")
	})

	t.Run("Returns error when repository fails", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.createErr = errors.New("database error")
		queue := NewPostgresTaskQueue(repo, logger)

		task := &models.BackgroundTask{
			ID:       "test-task-id",
			TaskType: "test-type",
			TaskName: "Test Task",
		}

		err := queue.Enqueue(context.Background(), task)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to enqueue task")
	})

	t.Run("Preserves existing status if set", func(t *testing.T) {
		repo := newMockTaskRepository()
		queue := NewPostgresTaskQueue(repo, logger)

		task := &models.BackgroundTask{
			ID:       "test-task-id",
			TaskType: "test-type",
			TaskName: "Test Task",
			Status:   models.TaskStatusQueued, // Already set
		}

		err := queue.Enqueue(context.Background(), task)

		require.NoError(t, err)
		assert.Equal(t, models.TaskStatusQueued, task.Status)
	})

	t.Run("Preserves existing priority if set", func(t *testing.T) {
		repo := newMockTaskRepository()
		queue := NewPostgresTaskQueue(repo, logger)

		task := &models.BackgroundTask{
			ID:       "test-task-id",
			TaskType: "test-type",
			TaskName: "Test Task",
			Priority: models.TaskPriorityHigh,
		}

		err := queue.Enqueue(context.Background(), task)

		require.NoError(t, err)
		assert.Equal(t, models.TaskPriorityHigh, task.Priority)
	})
}

// TestPostgresTaskQueue_Dequeue tests the Dequeue method
func TestPostgresTaskQueue_Dequeue(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Dequeues task successfully", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{
			ID:       "task-1",
			Status:   models.TaskStatusPending,
			TaskType: "test-type",
		})
		queue := NewPostgresTaskQueue(repo, logger)

		task, err := queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})

		require.NoError(t, err)
		assert.NotNil(t, task)
		assert.Equal(t, "task-1", task.ID)
		assert.Equal(t, models.TaskStatusRunning, task.Status)
		assert.NotNil(t, task.WorkerID)
		assert.Equal(t, "worker-1", *task.WorkerID)
	})

	t.Run("Returns nil when no tasks available", func(t *testing.T) {
		repo := newMockTaskRepository()
		queue := NewPostgresTaskQueue(repo, logger)

		task, err := queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})

		require.NoError(t, err)
		assert.Nil(t, task)
	})

	t.Run("Returns error when repository fails", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.dequeueErr = errors.New("database error")
		queue := NewPostgresTaskQueue(repo, logger)

		task, err := queue.Dequeue(context.Background(), "worker-1", ResourceRequirements{})

		assert.Error(t, err)
		assert.Nil(t, task)
		assert.Contains(t, err.Error(), "failed to dequeue task")
	})
}

// TestPostgresTaskQueue_Peek tests the Peek method
func TestPostgresTaskQueue_Peek(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Returns pending tasks", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusPending,
		})
		repo.tasks.Put("task-2", &models.BackgroundTask{
			ID:     "task-2",
			Status: models.TaskStatusPending,
		})
		queue := NewPostgresTaskQueue(repo, logger)

		tasks, err := queue.Peek(context.Background(), 10)

		require.NoError(t, err)
		assert.Len(t, tasks, 2)
	})

	t.Run("Uses default count when zero", func(t *testing.T) {
		repo := newMockTaskRepository()
		queue := NewPostgresTaskQueue(repo, logger)

		_, err := queue.Peek(context.Background(), 0)

		require.NoError(t, err)
	})

	t.Run("Uses default count when negative", func(t *testing.T) {
		repo := newMockTaskRepository()
		queue := NewPostgresTaskQueue(repo, logger)

		_, err := queue.Peek(context.Background(), -5)

		require.NoError(t, err)
	})
}

// TestPostgresTaskQueue_Requeue tests the Requeue method
func TestPostgresTaskQueue_Requeue(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Requeues task successfully", func(t *testing.T) {
		repo := newMockTaskRepository()
		workerID := "worker-1"
		now := time.Now()
		repo.tasks.Put("task-1", &models.BackgroundTask{
			ID:            "task-1",
			Status:        models.TaskStatusRunning,
			WorkerID:      &workerID,
			StartedAt:     &now,
			LastHeartbeat: &now,
			RetryCount:    0,
		})
		queue := NewPostgresTaskQueue(repo, logger)

		err := queue.Requeue(context.Background(), "task-1", 0)

		require.NoError(t, err)
		task, _ := repo.tasks.Get("task-1")
		assert.Equal(t, models.TaskStatusPending, task.Status)
		assert.Nil(t, task.WorkerID)
		assert.Nil(t, task.StartedAt)
		assert.Nil(t, task.LastHeartbeat)
		assert.Equal(t, 1, task.RetryCount)
	})

	t.Run("Requeues with delay", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusRunning,
		})
		queue := NewPostgresTaskQueue(repo, logger)

		delay := 5 * time.Second
		beforeRequeue := time.Now()
		err := queue.Requeue(context.Background(), "task-1", delay)

		require.NoError(t, err)
		task, _ := repo.tasks.Get("task-1")
		assert.True(t, task.ScheduledAt.After(beforeRequeue.Add(delay-time.Second)))
	})

	t.Run("Returns error for non-existent task", func(t *testing.T) {
		repo := newMockTaskRepository()
		queue := NewPostgresTaskQueue(repo, logger)

		err := queue.Requeue(context.Background(), "non-existent", 0)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get task")
	})

	t.Run("Returns error when update fails", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusRunning,
		})
		repo.updateErr = errors.New("database error")
		queue := NewPostgresTaskQueue(repo, logger)

		err := queue.Requeue(context.Background(), "task-1", 0)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update task")
	})
}

// TestPostgresTaskQueue_MoveToDeadLetter tests the MoveToDeadLetter method
func TestPostgresTaskQueue_MoveToDeadLetter(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Moves task to dead letter successfully", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusFailed,
		})
		queue := NewPostgresTaskQueue(repo, logger)

		err := queue.MoveToDeadLetter(context.Background(), "task-1", "max retries exceeded")

		require.NoError(t, err)
		task, _ := repo.tasks.Get("task-1")
		assert.Equal(t, models.TaskStatusDeadLetter, task.Status)
		assert.NotNil(t, task.LastError)
		assert.Equal(t, "max retries exceeded", *task.LastError)
	})
}

// TestPostgresTaskQueue_GetPendingCount tests the GetPendingCount method
func TestPostgresTaskQueue_GetPendingCount(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Returns pending count", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{ID: "task-1", Status: models.TaskStatusPending})
		repo.tasks.Put("task-2", &models.BackgroundTask{ID: "task-2", Status: models.TaskStatusPending})
		repo.tasks.Put("task-3", &models.BackgroundTask{ID: "task-3", Status: models.TaskStatusRunning})
		queue := NewPostgresTaskQueue(repo, logger)

		count, err := queue.GetPendingCount(context.Background())

		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("Returns error when repository fails", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.countErr = errors.New("database error")
		queue := NewPostgresTaskQueue(repo, logger)

		_, err := queue.GetPendingCount(context.Background())

		assert.Error(t, err)
	})
}

// TestPostgresTaskQueue_GetRunningCount tests the GetRunningCount method
func TestPostgresTaskQueue_GetRunningCount(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Returns running count", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{ID: "task-1", Status: models.TaskStatusPending})
		repo.tasks.Put("task-2", &models.BackgroundTask{ID: "task-2", Status: models.TaskStatusRunning})
		repo.tasks.Put("task-3", &models.BackgroundTask{ID: "task-3", Status: models.TaskStatusRunning})
		queue := NewPostgresTaskQueue(repo, logger)

		count, err := queue.GetRunningCount(context.Background())

		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})
}

// TestPostgresTaskQueue_GetQueueDepth tests the GetQueueDepth method
func TestPostgresTaskQueue_GetQueueDepth(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Returns queue depth by priority", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{ID: "task-1", Status: models.TaskStatusPending, Priority: models.TaskPriorityHigh})
		repo.tasks.Put("task-2", &models.BackgroundTask{ID: "task-2", Status: models.TaskStatusPending, Priority: models.TaskPriorityHigh})
		repo.tasks.Put("task-3", &models.BackgroundTask{ID: "task-3", Status: models.TaskStatusPending, Priority: models.TaskPriorityNormal})
		queue := NewPostgresTaskQueue(repo, logger)

		depth, err := queue.GetQueueDepth(context.Background())

		require.NoError(t, err)
		assert.Equal(t, int64(2), depth[models.TaskPriorityHigh])
		assert.Equal(t, int64(1), depth[models.TaskPriorityNormal])
	})

	t.Run("Uses cache when fresh", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{ID: "task-1", Status: models.TaskStatusPending, Priority: models.TaskPriorityHigh})
		queue := NewPostgresTaskQueue(repo, logger)

		// First call populates cache
		depth1, _ := queue.GetQueueDepth(context.Background())

		// Add another task
		repo.tasks.Put("task-2", &models.BackgroundTask{ID: "task-2", Status: models.TaskStatusPending, Priority: models.TaskPriorityHigh})

		// Second call should return cached value
		depth2, _ := queue.GetQueueDepth(context.Background())

		assert.Equal(t, depth1[models.TaskPriorityHigh], depth2[models.TaskPriorityHigh])
	})
}

// TestPostgresTaskQueue_GetStats tests the GetStats method
func TestPostgresTaskQueue_GetStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("Returns queue stats", func(t *testing.T) {
		repo := newMockTaskRepository()
		repo.tasks.Put("task-1", &models.BackgroundTask{ID: "task-1", Status: models.TaskStatusPending, Priority: models.TaskPriorityHigh})
		repo.tasks.Put("task-2", &models.BackgroundTask{ID: "task-2", Status: models.TaskStatusRunning, Priority: models.TaskPriorityNormal})
		queue := NewPostgresTaskQueue(repo, logger)

		stats, err := queue.GetStats(context.Background())

		require.NoError(t, err)
		assert.Equal(t, int64(1), stats.PendingCount)
		assert.Equal(t, int64(1), stats.RunningCount)
		assert.NotNil(t, stats.DepthByPriority)
		assert.NotNil(t, stats.StatusCounts)
	})
}

// Additional tests for InMemoryTaskQueue that complement background_test.go
// (duplicate tests removed - they already exist in background_test.go)

// --- Benchmarks ---

func BenchmarkTaskEventType_Topic(b *testing.B) {
	eventTypes := []TaskEventType{
		TaskEventTypeCreated,
		TaskEventTypeStarted,
		TaskEventTypeProgress,
		TaskEventTypeHeartbeat,
		TaskEventTypeCompleted,
		TaskEventTypeFailed,
		TaskEventTypeStuck,
		TaskEventTypeCancelled,
		TaskEventTypeRetrying,
		TaskEventTypeDeadLetter,
		TaskEventTypeLog,
		TaskEventTypeResource,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		et := eventTypes[i%len(eventTypes)]
		_ = et.Topic()
	}
}

func BenchmarkWorkerState_String(b *testing.B) {
	states := []workerState{
		workerStateIdle,
		workerStateBusy,
		workerStateStopping,
		workerStateStopped,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := states[i%len(states)]
		_ = s.String()
	}
}

func BenchmarkIsTerminalStatus(b *testing.B) {
	statuses := []models.TaskStatus{
		models.TaskStatusPending,
		models.TaskStatusRunning,
		models.TaskStatusCompleted,
		models.TaskStatusFailed,
		models.TaskStatusCancelled,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isTerminalStatus(statuses[i%len(statuses)])
	}
}
