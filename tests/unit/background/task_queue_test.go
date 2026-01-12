package background_test

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/background"
	"dev.helix.agent/internal/models"
)

func TestInMemoryTaskQueue_Enqueue(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	task := &models.BackgroundTask{
		TaskType: "test",
		TaskName: "Test Task",
		Priority: models.TaskPriorityNormal,
	}

	err := queue.Enqueue(context.Background(), task)
	require.NoError(t, err)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, models.TaskStatusPending, task.Status)
}

func TestInMemoryTaskQueue_Dequeue(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	// Enqueue task
	task := &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "Test Task",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second), // Already schedulable
	}
	err := queue.Enqueue(context.Background(), task)
	require.NoError(t, err)

	// Dequeue
	dequeued, err := queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{})
	require.NoError(t, err)
	require.NotNil(t, dequeued)

	assert.Equal(t, task.ID, dequeued.ID)
	assert.Equal(t, models.TaskStatusRunning, dequeued.Status)
	assert.NotNil(t, dequeued.WorkerID)
	assert.Equal(t, "worker-1", *dequeued.WorkerID)
}

func TestInMemoryTaskQueue_DequeueEmpty(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	dequeued, err := queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{})
	require.NoError(t, err)
	assert.Nil(t, dequeued)
}

func TestInMemoryTaskQueue_PriorityOrdering(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	// Enqueue tasks with different priorities
	lowPriority := &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "Low Priority",
		Priority:    models.TaskPriorityLow,
		ScheduledAt: time.Now().Add(-time.Second),
	}
	highPriority := &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "High Priority",
		Priority:    models.TaskPriorityHigh,
		ScheduledAt: time.Now().Add(-time.Second),
	}
	normalPriority := &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "Normal Priority",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second),
	}

	// Enqueue in reverse priority order
	queue.Enqueue(context.Background(), lowPriority)
	queue.Enqueue(context.Background(), normalPriority)
	queue.Enqueue(context.Background(), highPriority)

	// Dequeue should return high priority first
	first, _ := queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{})
	assert.Equal(t, "High Priority", first.TaskName)

	second, _ := queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{})
	assert.Equal(t, "Normal Priority", second.TaskName)

	third, _ := queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{})
	assert.Equal(t, "Low Priority", third.TaskName)
}

func TestInMemoryTaskQueue_ScheduledAt(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	// Enqueue task scheduled for the future
	futureTask := &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "Future Task",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(time.Hour),
	}
	queue.Enqueue(context.Background(), futureTask)

	// Should not dequeue future tasks
	dequeued, _ := queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{})
	assert.Nil(t, dequeued)
}

func TestInMemoryTaskQueue_ResourceRequirements(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	// Enqueue task with high resource requirements
	bigTask := &models.BackgroundTask{
		TaskType:         "test",
		TaskName:         "Big Task",
		Priority:         models.TaskPriorityNormal,
		ScheduledAt:      time.Now().Add(-time.Second),
		RequiredCPUCores: 8,
		RequiredMemoryMB: 16000,
	}
	queue.Enqueue(context.Background(), bigTask)

	// Should not dequeue if worker doesn't have enough resources
	dequeued, _ := queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{
		CPUCores: 4,
		MemoryMB: 8000,
	})
	assert.Nil(t, dequeued)

	// Should dequeue if worker has enough resources
	dequeued, _ = queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{
		CPUCores: 8,
		MemoryMB: 16000,
	})
	assert.NotNil(t, dequeued)
}

func TestInMemoryTaskQueue_Requeue(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	task := &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "Test Task",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second),
	}
	queue.Enqueue(context.Background(), task)

	// Dequeue
	dequeued, _ := queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{})
	require.NotNil(t, dequeued)

	// Requeue with delay
	err := queue.Requeue(context.Background(), task.ID, 0)
	require.NoError(t, err)

	// Should be available again
	requeued, _ := queue.Dequeue(context.Background(), "worker-2", background.ResourceRequirements{})
	assert.NotNil(t, requeued)
	assert.Equal(t, 1, requeued.RetryCount)
}

func TestInMemoryTaskQueue_MoveToDeadLetter(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	task := &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "Test Task",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second),
	}
	queue.Enqueue(context.Background(), task)

	err := queue.MoveToDeadLetter(context.Background(), task.ID, "max retries exceeded")
	require.NoError(t, err)

	// Task should be marked as dead letter
	retrieved := queue.GetTask(task.ID)
	assert.Equal(t, models.TaskStatusDeadLetter, retrieved.Status)
	assert.NotNil(t, retrieved.LastError)
	assert.Equal(t, "max retries exceeded", *retrieved.LastError)
}

func TestInMemoryTaskQueue_GetPendingCount(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	// Add 3 pending tasks
	for i := 0; i < 3; i++ {
		queue.Enqueue(context.Background(), &models.BackgroundTask{
			TaskType:    "test",
			TaskName:    "Test Task",
			Priority:    models.TaskPriorityNormal,
			ScheduledAt: time.Now().Add(-time.Second),
		})
	}

	count, err := queue.GetPendingCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Dequeue one
	queue.Dequeue(context.Background(), "worker-1", background.ResourceRequirements{})

	count, err = queue.GetPendingCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestInMemoryTaskQueue_GetQueueDepth(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	// Add tasks with different priorities
	queue.Enqueue(context.Background(), &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "High",
		Priority:    models.TaskPriorityHigh,
		ScheduledAt: time.Now().Add(-time.Second),
	})
	queue.Enqueue(context.Background(), &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "Normal 1",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second),
	})
	queue.Enqueue(context.Background(), &models.BackgroundTask{
		TaskType:    "test",
		TaskName:    "Normal 2",
		Priority:    models.TaskPriorityNormal,
		ScheduledAt: time.Now().Add(-time.Second),
	})

	depth, err := queue.GetQueueDepth(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(1), depth[models.TaskPriorityHigh])
	assert.Equal(t, int64(2), depth[models.TaskPriorityNormal])
}

func TestInMemoryTaskQueue_Peek(t *testing.T) {
	logger := logrus.New()
	queue := background.NewInMemoryTaskQueue(logger)

	// Add tasks
	for i := 0; i < 5; i++ {
		queue.Enqueue(context.Background(), &models.BackgroundTask{
			TaskType:    "test",
			TaskName:    "Test Task",
			Priority:    models.TaskPriorityNormal,
			ScheduledAt: time.Now().Add(-time.Second),
		})
	}

	// Peek at first 3
	tasks, err := queue.Peek(context.Background(), 3)
	require.NoError(t, err)
	assert.Len(t, tasks, 3)

	// Tasks should still be pending (not claimed)
	count, _ := queue.GetPendingCount(context.Background())
	assert.Equal(t, int64(5), count)
}
