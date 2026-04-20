package background

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

// newTestWorkerPool creates an AdaptiveWorkerPool for testing, reusing shared metrics from metrics_test.go
func newTestWorkerPool(
	config *WorkerPoolConfig,
	queue TaskQueue,
	repository TaskRepository,
	resourceMonitor ResourceMonitor,
	stuckDetector StuckDetector,
	notifier NotificationService,
	logger *logrus.Logger,
) *AdaptiveWorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &AdaptiveWorkerPool{
		config:          config,
		queue:           queue,
		repository:      repository,
		executors:       safe.NewStore[string, TaskExecutor](),
		resourceMonitor: resourceMonitor,
		stuckDetector:   stuckDetector,
		notifier:        notifier,
		logger:          logger,
		metrics:         getTestMetrics(), // Use shared metrics
		workers:         safe.NewStore[string, *Worker](),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// mockTaskQueue implements TaskQueue interface for testing
type mockTaskQueue struct {
	mu            sync.RWMutex
	tasks         []*models.BackgroundTask
	pendingCount  int64
	runningCount  int64
	dequeueErr    error
	enqueueErr    error
	requeueErr    error
	deadLetterErr error
	dequeueDelay  time.Duration
}

func newMockTaskQueue() *mockTaskQueue {
	return &mockTaskQueue{
		tasks: make([]*models.BackgroundTask, 0),
	}
}

func (m *mockTaskQueue) Enqueue(ctx context.Context, task *models.BackgroundTask) error {
	if m.enqueueErr != nil {
		return m.enqueueErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = append(m.tasks, task)
	m.pendingCount++
	return nil
}

func (m *mockTaskQueue) Dequeue(ctx context.Context, workerID string, requirements ResourceRequirements) (*models.BackgroundTask, error) {
	if m.dequeueDelay > 0 {
		time.Sleep(m.dequeueDelay)
	}
	if m.dequeueErr != nil {
		return nil, m.dequeueErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, task := range m.tasks {
		if task.Status == models.TaskStatusPending {
			task.Status = models.TaskStatusRunning
			task.WorkerID = &workerID
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			m.pendingCount--
			m.runningCount++
			return task, nil
		}
	}
	return nil, nil
}

func (m *mockTaskQueue) Peek(ctx context.Context, count int) ([]*models.BackgroundTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*models.BackgroundTask
	for _, task := range m.tasks {
		if task.Status == models.TaskStatusPending {
			result = append(result, task)
			if len(result) >= count {
				break
			}
		}
	}
	return result, nil
}

func (m *mockTaskQueue) Requeue(ctx context.Context, taskID string, delay time.Duration) error {
	if m.requeueErr != nil {
		return m.requeueErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, task := range m.tasks {
		if task.ID == taskID {
			task.Status = models.TaskStatusPending
			task.WorkerID = nil
			m.pendingCount++
			m.runningCount--
			return nil
		}
	}
	// Task might have been removed - create new entry
	m.tasks = append(m.tasks, &models.BackgroundTask{ID: taskID, Status: models.TaskStatusPending})
	m.pendingCount++
	return nil
}

func (m *mockTaskQueue) MoveToDeadLetter(ctx context.Context, taskID string, reason string) error {
	if m.deadLetterErr != nil {
		return m.deadLetterErr
	}
	return nil
}

func (m *mockTaskQueue) GetPendingCount(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pendingCount, nil
}

func (m *mockTaskQueue) GetRunningCount(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runningCount, nil
}

func (m *mockTaskQueue) GetQueueDepth(ctx context.Context) (map[models.TaskPriority]int64, error) {
	return map[models.TaskPriority]int64{
		models.TaskPriorityNormal: m.pendingCount,
	}, nil
}

func (m *mockTaskQueue) AddTask(task *models.BackgroundTask) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = append(m.tasks, task)
	if task.Status == models.TaskStatusPending {
		m.pendingCount++
	}
}

func (m *mockTaskQueue) SetPendingCount(count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingCount = count
}

// mockStuckDetector implements StuckDetector interface for testing
type mockStuckDetector struct {
	isStuck     bool
	stuckReason string
	thresholds  map[string]time.Duration
}

func newMockStuckDetector() *mockStuckDetector {
	return &mockStuckDetector{
		thresholds: make(map[string]time.Duration),
	}
}

func (m *mockStuckDetector) IsStuck(ctx context.Context, task *models.BackgroundTask, snapshots []*models.ResourceSnapshot) (bool, string) {
	return m.isStuck, m.stuckReason
}

func (m *mockStuckDetector) GetStuckThreshold(taskType string) time.Duration {
	if d, ok := m.thresholds[taskType]; ok {
		return d
	}
	return 5 * time.Minute
}

func (m *mockStuckDetector) SetThreshold(taskType string, threshold time.Duration) {
	m.thresholds[taskType] = threshold
}

func (m *mockStuckDetector) SetStuck(isStuck bool, reason string) {
	m.isStuck = isStuck
	m.stuckReason = reason
}

// mockNotificationService implements NotificationService interface for testing
type mockNotificationService struct {
	mu         sync.Mutex
	events     []notificationEvent
	notifyErr  error
	sseClients map[string][]chan<- []byte
	wsClients  map[string][]WebSocketClient
}

type notificationEvent struct {
	taskID    string
	eventType string
	data      map[string]interface{}
}

func newMockNotificationService() *mockNotificationService {
	return &mockNotificationService{
		events:     make([]notificationEvent, 0),
		sseClients: make(map[string][]chan<- []byte),
		wsClients:  make(map[string][]WebSocketClient),
	}
}

func (m *mockNotificationService) NotifyTaskEvent(ctx context.Context, task *models.BackgroundTask, event string, data map[string]interface{}) error {
	if m.notifyErr != nil {
		return m.notifyErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, notificationEvent{
		taskID:    task.ID,
		eventType: event,
		data:      data,
	})
	return nil
}

func (m *mockNotificationService) RegisterSSEClient(ctx context.Context, taskID string, client chan<- []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sseClients[taskID] = append(m.sseClients[taskID], client)
	return nil
}

func (m *mockNotificationService) UnregisterSSEClient(ctx context.Context, taskID string, client chan<- []byte) error {
	return nil
}

func (m *mockNotificationService) RegisterWebSocketClient(ctx context.Context, taskID string, client WebSocketClient) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wsClients[taskID] = append(m.wsClients[taskID], client)
	return nil
}

func (m *mockNotificationService) BroadcastToTask(ctx context.Context, taskID string, message []byte) error {
	return nil
}

func (m *mockNotificationService) GetEvents() []notificationEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.events
}

// Helper to create a test logger
func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress log output during tests
	return logger
}

// TestNewAdaptiveWorkerPool tests the constructor
func TestNewAdaptiveWorkerPool(t *testing.T) {
	t.Run("Creates pool with default config when nil", func(t *testing.T) {
		queue := newMockTaskQueue()
		repo := newMockTaskRepository()
		logger := newTestLogger()

		pool := newTestWorkerPool(nil, queue, repo, nil, nil, nil, logger)

		assert.NotNil(t, pool)
		assert.NotNil(t, pool.config)
		assert.Equal(t, 2, pool.config.MinWorkers)
		assert.False(t, pool.IsStarted())
	})

	t.Run("Creates pool with custom config", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        4,
			MaxWorkers:        16,
			ScaleUpThreshold:  0.8,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: 10 * time.Minute,
		}
		queue := newMockTaskQueue()
		repo := newMockTaskRepository()
		logger := newTestLogger()

		pool := newTestWorkerPool(config, queue, repo, nil, nil, nil, logger)

		assert.NotNil(t, pool)
		assert.Equal(t, 4, pool.config.MinWorkers)
		assert.Equal(t, 16, pool.config.MaxWorkers)
		assert.Equal(t, 0.8, pool.config.ScaleUpThreshold)
	})

	t.Run("Creates pool with all dependencies", func(t *testing.T) {
		config := DefaultWorkerPoolConfig()
		queue := newMockTaskQueue()
		repo := newMockTaskRepository()
		resourceMonitor := NewMockResourceMonitor()
		stuckDetector := newMockStuckDetector()
		notifier := newMockNotificationService()
		logger := newTestLogger()

		pool := newTestWorkerPool(config, queue, repo, resourceMonitor, stuckDetector, notifier, logger)

		assert.NotNil(t, pool)
		assert.NotNil(t, pool.queue)
		assert.NotNil(t, pool.repository)
		assert.NotNil(t, pool.resourceMonitor)
		assert.NotNil(t, pool.stuckDetector)
		assert.NotNil(t, pool.notifier)
		assert.NotNil(t, pool.metrics)
		assert.NotNil(t, pool.executors)
		assert.NotNil(t, pool.workers)
	})
}

// TestAdaptiveWorkerPool_RegisterExecutor tests executor registration
func TestAdaptiveWorkerPool_RegisterExecutor(t *testing.T) {
	t.Run("Registers executor successfully", func(t *testing.T) {
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		executor := &mockTaskExecutor{}
		pool.RegisterExecutor("test-task", executor)

		registered, _ := pool.executors.Get("test-task")
		assert.NotNil(t, registered)
		assert.Equal(t, executor, registered)
	})

	t.Run("Overwrites existing executor", func(t *testing.T) {
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		executor1 := &mockTaskExecutor{canPause: true}
		executor2 := &mockTaskExecutor{canPause: false}

		pool.RegisterExecutor("test-task", executor1)
		pool.RegisterExecutor("test-task", executor2)

		registered, _ := pool.executors.Get("test-task")
		assert.Equal(t, executor2, registered)
	})

	t.Run("Registers multiple executors", func(t *testing.T) {
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		pool.RegisterExecutor("type-1", &mockTaskExecutor{})
		pool.RegisterExecutor("type-2", &mockTaskExecutor{})
		pool.RegisterExecutor("type-3", &mockTaskExecutor{})

		assert.Equal(t, 3, pool.executors.Len())
	})
}

// TestAdaptiveWorkerPool_StartStop tests the lifecycle methods
func TestAdaptiveWorkerPool_StartStop(t *testing.T) {
	t.Run("Starts pool successfully", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        2,
			MaxWorkers:        4,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: time.Minute,
			QueuePollInterval: 100 * time.Millisecond,
			HeartbeatInterval: time.Minute,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond // Slow down dequeue to prevent rapid loops
		repo := newMockTaskRepository()
		logger := newTestLogger()

		pool := newTestWorkerPool(config, queue, repo, nil, nil, nil, logger)

		err := pool.Start(context.Background())

		require.NoError(t, err)
		assert.True(t, pool.IsStarted())

		// Give workers time to start
		time.Sleep(100 * time.Millisecond)
		assert.GreaterOrEqual(t, pool.GetWorkerCount(), config.MinWorkers)

		// Cleanup
		_ = pool.Stop(time.Second)
	})

	t.Run("Returns error when already started", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        1,
			MaxWorkers:        2,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: time.Minute,
			QueuePollInterval: 100 * time.Millisecond,
			HeartbeatInterval: time.Minute,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond
		pool := newTestWorkerPool(config, queue, newMockTaskRepository(), nil, nil, nil, newTestLogger())

		_ = pool.Start(context.Background())
		defer func() { _ = pool.Stop(time.Second) }()

		err := pool.Start(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")
	})

	t.Run("Stops pool gracefully", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:           2,
			MaxWorkers:           4,
			ScaleInterval:        time.Minute,
			WorkerIdleTimeout:    time.Minute,
			QueuePollInterval:    100 * time.Millisecond,
			HeartbeatInterval:    time.Minute,
			GracefulShutdownTime: 2 * time.Second,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond
		pool := newTestWorkerPool(config, queue, newMockTaskRepository(), nil, nil, nil, newTestLogger())

		_ = pool.Start(context.Background())
		time.Sleep(100 * time.Millisecond)

		err := pool.Stop(time.Second)

		assert.NoError(t, err)
		assert.False(t, pool.IsStarted())
	})
}

// TestAdaptiveWorkerPool_GetWorkerCount tests the GetWorkerCount method
func TestAdaptiveWorkerPool_GetWorkerCount(t *testing.T) {
	t.Run("Returns zero when not started", func(t *testing.T) {
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		count := pool.GetWorkerCount()

		assert.Equal(t, 0, count)
	})

	t.Run("Returns min workers after start", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        3,
			MaxWorkers:        6,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: time.Minute,
			QueuePollInterval: 100 * time.Millisecond,
			HeartbeatInterval: time.Minute,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond
		pool := newTestWorkerPool(config, queue, newMockTaskRepository(), nil, nil, nil, newTestLogger())

		_ = pool.Start(context.Background())
		defer func() { _ = pool.Stop(time.Second) }()
		time.Sleep(100 * time.Millisecond)

		count := pool.GetWorkerCount()

		assert.Equal(t, 3, count)
	})
}

// TestAdaptiveWorkerPool_GetActiveTaskCount tests the GetActiveTaskCount method
func TestAdaptiveWorkerPool_GetActiveTaskCount(t *testing.T) {
	t.Run("Returns zero when no workers", func(t *testing.T) {
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		count := pool.GetActiveTaskCount()

		assert.Equal(t, 0, count)
	})

	t.Run("Returns zero when workers are idle", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        2,
			MaxWorkers:        4,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: time.Minute,
			QueuePollInterval: 100 * time.Millisecond,
			HeartbeatInterval: time.Minute,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond
		pool := newTestWorkerPool(config, queue, newMockTaskRepository(), nil, nil, nil, newTestLogger())

		_ = pool.Start(context.Background())
		defer func() { _ = pool.Stop(time.Second) }()
		time.Sleep(100 * time.Millisecond)

		count := pool.GetActiveTaskCount()

		assert.Equal(t, 0, count)
	})
}

// TestAdaptiveWorkerPool_GetWorkerStatus tests the GetWorkerStatus method
func TestAdaptiveWorkerPool_GetWorkerStatus(t *testing.T) {
	t.Run("Returns empty when not started", func(t *testing.T) {
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		statuses := pool.GetWorkerStatus()

		assert.Empty(t, statuses)
	})

	t.Run("Returns status for all workers", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        2,
			MaxWorkers:        4,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: time.Minute,
			QueuePollInterval: 100 * time.Millisecond,
			HeartbeatInterval: time.Minute,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond
		pool := newTestWorkerPool(config, queue, newMockTaskRepository(), nil, nil, nil, newTestLogger())

		_ = pool.Start(context.Background())
		defer func() { _ = pool.Stop(time.Second) }()
		time.Sleep(100 * time.Millisecond)

		statuses := pool.GetWorkerStatus()

		assert.Len(t, statuses, 2)
		for _, status := range statuses {
			assert.NotEmpty(t, status.ID)
			assert.Equal(t, "idle", status.Status)
			assert.False(t, status.StartedAt.IsZero())
		}
	})
}

// TestAdaptiveWorkerPool_Scale tests the Scale method
func TestAdaptiveWorkerPool_Scale(t *testing.T) {
	t.Run("Scales up workers", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        2,
			MaxWorkers:        6,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: time.Minute,
			QueuePollInterval: 100 * time.Millisecond,
			HeartbeatInterval: time.Minute,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond
		pool := newTestWorkerPool(config, queue, newMockTaskRepository(), nil, nil, nil, newTestLogger())

		_ = pool.Start(context.Background())
		defer func() { _ = pool.Stop(time.Second) }()
		time.Sleep(100 * time.Millisecond)

		err := pool.Scale(4)

		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, 4, pool.GetWorkerCount())
	})

	t.Run("Respects max workers limit", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        2,
			MaxWorkers:        4,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: time.Minute,
			QueuePollInterval: 100 * time.Millisecond,
			HeartbeatInterval: time.Minute,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond
		pool := newTestWorkerPool(config, queue, newMockTaskRepository(), nil, nil, nil, newTestLogger())

		_ = pool.Start(context.Background())
		defer func() { _ = pool.Stop(time.Second) }()
		time.Sleep(100 * time.Millisecond)

		err := pool.Scale(10) // Request more than max

		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, 4, pool.GetWorkerCount()) // Should cap at max
	})

	t.Run("Respects min workers limit", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:        3,
			MaxWorkers:        6,
			ScaleInterval:     time.Minute,
			WorkerIdleTimeout: time.Minute,
			QueuePollInterval: 100 * time.Millisecond,
			HeartbeatInterval: time.Minute,
		}
		queue := newMockTaskQueue()
		queue.dequeueDelay = 50 * time.Millisecond
		pool := newTestWorkerPool(config, queue, newMockTaskRepository(), nil, nil, nil, newTestLogger())

		_ = pool.Start(context.Background())
		defer func() { _ = pool.Stop(time.Second) }()
		time.Sleep(100 * time.Millisecond)

		err := pool.Scale(1) // Request less than min

		require.NoError(t, err)
		// Workers won't go below min immediately (they use idle timeout)
		assert.GreaterOrEqual(t, pool.GetWorkerCount(), 3)
	})
}

// TestAdaptiveWorkerPool_canScaleDown tests the canScaleDown method
func TestAdaptiveWorkerPool_canScaleDown(t *testing.T) {
	t.Run("Returns false when at min workers", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers: 2,
			MaxWorkers: 4,
		}
		pool := newTestWorkerPool(config, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		// Set active count to min
		pool.activeCount = 2

		assert.False(t, pool.canScaleDown())
	})

	t.Run("Returns true when above min workers", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers: 2,
			MaxWorkers: 4,
		}
		pool := newTestWorkerPool(config, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		// Set active count above min
		pool.activeCount = 3

		assert.True(t, pool.canScaleDown())
	})
}

// TestAdaptiveWorkerPool_calculateWorkerRequirements tests the calculateWorkerRequirements method
func TestAdaptiveWorkerPool_calculateWorkerRequirements(t *testing.T) {
	t.Run("Returns empty requirements when no resource monitor", func(t *testing.T) {
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		req := pool.calculateWorkerRequirements()

		assert.Equal(t, 0, req.CPUCores)
		assert.Equal(t, 0, req.MemoryMB)
	})

	t.Run("Returns calculated requirements with resource monitor", func(t *testing.T) {
		resourceMonitor := NewMockResourceMonitor()
		resourceMonitor.SetSystemResources(&SystemResources{
			AvailableCPUCores: 8,
			AvailableMemoryMB: 16000,
		})
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), resourceMonitor, nil, nil, newTestLogger())
		pool.activeCount = 4 // 4 workers

		req := pool.calculateWorkerRequirements()

		assert.Equal(t, 2, req.CPUCores)    // 8 / 4
		assert.Equal(t, 4000, req.MemoryMB) // 16000 / 4
	})

	t.Run("Handles zero workers", func(t *testing.T) {
		resourceMonitor := NewMockResourceMonitor()
		resourceMonitor.SetSystemResources(&SystemResources{
			AvailableCPUCores: 4,
			AvailableMemoryMB: 8000,
		})
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), resourceMonitor, nil, nil, newTestLogger())
		pool.activeCount = 0

		req := pool.calculateWorkerRequirements()

		// Should use 1 as minimum divisor
		assert.Equal(t, 4, req.CPUCores)
		assert.Equal(t, 8000, req.MemoryMB)
	})
}

// TestTaskProgressReporter tests the taskProgressReporter implementation
func TestTaskProgressReporter(t *testing.T) {
	t.Run("ReportProgress updates repository and notifies", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{ID: "task-1"}
		_ = repo.Create(context.Background(), task)
		notifier := newMockNotificationService()

		reporter := &taskProgressReporter{
			taskID:     "task-1",
			repository: repo,
			notifier:   notifier,
			task:       task,
			workerID:   "worker-1",
		}

		err := reporter.ReportProgress(50.0, "halfway done")

		require.NoError(t, err)
		updatedTask, _ := repo.GetByID(context.Background(), "task-1")
		assert.Equal(t, 50.0, updatedTask.Progress)
		assert.Equal(t, "halfway done", *updatedTask.ProgressMessage)
	})

	t.Run("ReportHeartbeat updates repository", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{ID: "task-1"}
		_ = repo.Create(context.Background(), task)

		reporter := &taskProgressReporter{
			taskID:     "task-1",
			repository: repo,
			task:       task,
			workerID:   "worker-1",
		}

		err := reporter.ReportHeartbeat()

		require.NoError(t, err)
		updatedTask, _ := repo.GetByID(context.Background(), "task-1")
		assert.NotNil(t, updatedTask.LastHeartbeat)
	})

	t.Run("ReportCheckpoint saves checkpoint", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{ID: "task-1"}
		_ = repo.Create(context.Background(), task)

		reporter := &taskProgressReporter{
			taskID:     "task-1",
			repository: repo,
			task:       task,
			workerID:   "worker-1",
		}

		err := reporter.ReportCheckpoint([]byte("checkpoint data"))

		require.NoError(t, err)
	})

	t.Run("ReportMetrics notifies with metrics", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{ID: "task-1"}
		notifier := newMockNotificationService()

		reporter := &taskProgressReporter{
			taskID:     "task-1",
			repository: repo,
			notifier:   notifier,
			task:       task,
			workerID:   "worker-1",
		}

		metrics := map[string]interface{}{
			"cpu_percent": 50.0,
			"memory_mb":   1024,
		}
		err := reporter.ReportMetrics(metrics)

		require.NoError(t, err)
		events := notifier.GetEvents()
		assert.NotEmpty(t, events)
	})

	t.Run("ReportMetrics returns nil when no notifier", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{ID: "task-1"}

		reporter := &taskProgressReporter{
			taskID:     "task-1",
			repository: repo,
			notifier:   nil, // No notifier
			task:       task,
			workerID:   "worker-1",
		}

		err := reporter.ReportMetrics(map[string]interface{}{})

		assert.NoError(t, err)
	})

	t.Run("ReportLog logs event and notifies", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{ID: "task-1"}
		_ = repo.Create(context.Background(), task)
		notifier := newMockNotificationService()

		reporter := &taskProgressReporter{
			taskID:     "task-1",
			repository: repo,
			notifier:   notifier,
			task:       task,
			workerID:   "worker-1",
		}

		err := reporter.ReportLog("info", "processing step 1", map[string]interface{}{"step": 1})

		require.NoError(t, err)
		history, _ := repo.GetTaskHistory(context.Background(), "task-1", 10)
		assert.NotEmpty(t, history)
	})
}

// TestAdaptiveWorkerPool_WaitForCompletion tests the WaitForCompletion method
func TestAdaptiveWorkerPool_WaitForCompletion(t *testing.T) {
	t.Run("Returns completed task", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:       "task-1",
			Status:   models.TaskStatusCompleted,
			Progress: 100,
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		result, err := pool.WaitForCompletion(context.Background(), "task-1", 5*time.Second, nil)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, models.TaskStatusCompleted, result.Status)
	})

	t.Run("Returns error for failed task", func(t *testing.T) {
		repo := newMockTaskRepository()
		errorMsg := "task execution failed"
		task := &models.BackgroundTask{
			ID:              "task-1",
			Status:          models.TaskStatusFailed,
			ProgressMessage: &errorMsg,
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		result, err := pool.WaitForCompletion(context.Background(), "task-1", 5*time.Second, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task failed")
		assert.NotNil(t, result)
	})

	t.Run("Returns error for cancelled task", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusCancelled,
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		_, err := pool.WaitForCompletion(context.Background(), "task-1", 5*time.Second, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled")
	})

	t.Run("Returns error for dead letter task", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusDeadLetter,
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		_, err := pool.WaitForCompletion(context.Background(), "task-1", 5*time.Second, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dead letter")
	})

	t.Run("Calls progress callback", func(t *testing.T) {
		repo := newMockTaskRepository()
		progressMsg := "50% complete"
		task := &models.BackgroundTask{
			ID:              "task-1",
			Status:          models.TaskStatusCompleted,
			Progress:        50,
			ProgressMessage: &progressMsg,
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		var callbackProgress float64
		var callbackMessage string
		callback := func(progress float64, message string) {
			callbackProgress = progress
			callbackMessage = message
		}

		_, err := pool.WaitForCompletion(context.Background(), "task-1", 5*time.Second, callback)

		require.NoError(t, err)
		assert.Equal(t, 50.0, callbackProgress)
		assert.Equal(t, "50% complete", callbackMessage)
	})

	t.Run("Returns error on timeout", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusRunning, // Not terminal
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		_, err := pool.WaitForCompletion(context.Background(), "task-1", 200*time.Millisecond, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("Returns error when task not found", func(t *testing.T) {
		repo := newMockTaskRepository()
		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		_, err := pool.WaitForCompletion(context.Background(), "non-existent", 5*time.Second, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get task")
	})

	t.Run("Respects context cancellation", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusRunning,
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		_, err := pool.WaitForCompletion(ctx, "task-1", 10*time.Second, nil)

		assert.Error(t, err)
	})
}

// TestAdaptiveWorkerPool_WaitForCompletionWithOutput tests the WaitForCompletionWithOutput method
func TestAdaptiveWorkerPool_WaitForCompletionWithOutput(t *testing.T) {
	t.Run("Returns task with output", func(t *testing.T) {
		repo := newMockTaskRepository()
		progressMsg := "output data here"
		task := &models.BackgroundTask{
			ID:              "task-1",
			Status:          models.TaskStatusCompleted,
			Progress:        100,
			ProgressMessage: &progressMsg,
			Config: models.TaskConfig{
				CaptureOutput: true,
			},
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		resultTask, output, err := pool.WaitForCompletionWithOutput(context.Background(), "task-1", 5*time.Second)

		require.NoError(t, err)
		assert.NotNil(t, resultTask)
		assert.Equal(t, "output data here", string(output))
	})

	t.Run("Returns nil output when capture disabled", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:       "task-1",
			Status:   models.TaskStatusCompleted,
			Progress: 100,
			Config: models.TaskConfig{
				CaptureOutput: false,
			},
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		_, output, err := pool.WaitForCompletionWithOutput(context.Background(), "task-1", 5*time.Second)

		require.NoError(t, err)
		assert.Nil(t, output)
	})

	t.Run("Returns error on failure", func(t *testing.T) {
		repo := newMockTaskRepository()
		errMsg := "task failed"
		task := &models.BackgroundTask{
			ID:              "task-1",
			Status:          models.TaskStatusFailed,
			ProgressMessage: &errMsg,
		}
		_ = repo.Create(context.Background(), task)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		_, _, err := pool.WaitForCompletionWithOutput(context.Background(), "task-1", 5*time.Second)

		assert.Error(t, err)
	})
}

// TestAdaptiveWorkerPool_WaitForMultiple tests the WaitForMultiple method
func TestAdaptiveWorkerPool_WaitForMultiple(t *testing.T) {
	t.Run("Waits for multiple tasks", func(t *testing.T) {
		repo := newMockTaskRepository()
		task1 := &models.BackgroundTask{
			ID:       "task-1",
			Status:   models.TaskStatusCompleted,
			Progress: 100,
		}
		task2 := &models.BackgroundTask{
			ID:       "task-2",
			Status:   models.TaskStatusCompleted,
			Progress: 100,
		}
		_ = repo.Create(context.Background(), task1)
		_ = repo.Create(context.Background(), task2)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		results := pool.WaitForMultiple(context.Background(), []string{"task-1", "task-2"}, 5*time.Second)

		assert.Len(t, results, 2)
		assert.NotNil(t, results["task-1"])
		assert.NotNil(t, results["task-2"])
		assert.Nil(t, results["task-1"].Error)
		assert.Nil(t, results["task-2"].Error)
	})

	t.Run("Handles mixed success and failure", func(t *testing.T) {
		repo := newMockTaskRepository()
		task1 := &models.BackgroundTask{
			ID:       "task-1",
			Status:   models.TaskStatusCompleted,
			Progress: 100,
		}
		errMsg := "failed"
		task2 := &models.BackgroundTask{
			ID:              "task-2",
			Status:          models.TaskStatusFailed,
			ProgressMessage: &errMsg,
		}
		_ = repo.Create(context.Background(), task1)
		_ = repo.Create(context.Background(), task2)

		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, nil, newTestLogger())

		results := pool.WaitForMultiple(context.Background(), []string{"task-1", "task-2"}, 5*time.Second)

		assert.Len(t, results, 2)
		assert.Nil(t, results["task-1"].Error)
		assert.NotNil(t, results["task-2"].Error)
	})

	t.Run("Returns empty map for empty task list", func(t *testing.T) {
		pool := newTestWorkerPool(nil, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())

		results := pool.WaitForMultiple(context.Background(), []string{}, 5*time.Second)

		assert.Len(t, results, 0)
	})
}

// TestAdaptiveWorkerPool_checkAndScale tests the checkAndScale method
func TestAdaptiveWorkerPool_checkAndScale(t *testing.T) {
	t.Run("Does nothing without resource monitor", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:         2,
			MaxWorkers:         4,
			ScaleUpThreshold:   0.7,
			ScaleDownThreshold: 0.3,
		}
		pool := newTestWorkerPool(config, newMockTaskQueue(), newMockTaskRepository(), nil, nil, nil, newTestLogger())
		pool.activeCount = 2

		// Should not panic
		pool.checkAndScale()

		assert.Equal(t, int32(2), pool.activeCount)
	})

	t.Run("Prevents concurrent scaling", func(t *testing.T) {
		config := &WorkerPoolConfig{
			MinWorkers:         2,
			MaxWorkers:         4,
			ScaleUpThreshold:   0.7,
			ScaleDownThreshold: 0.3,
		}
		resourceMonitor := NewMockResourceMonitor()
		pool := newTestWorkerPool(config, newMockTaskQueue(), newMockTaskRepository(), resourceMonitor, nil, nil, newTestLogger())
		pool.activeCount = 2

		// Set scaling flag
		pool.scaling = 1

		// Should return immediately without scaling
		pool.checkAndScale()

		assert.Equal(t, int32(2), pool.activeCount)
	})
}

// TestAdaptiveWorkerPool_handleStuckTask tests the handleStuckTask method
func TestAdaptiveWorkerPool_handleStuckTask(t *testing.T) {
	t.Run("Updates task status to stuck", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusRunning,
		}
		_ = repo.Create(context.Background(), task)

		notifier := newMockNotificationService()
		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, notifier, newTestLogger())

		pool.handleStuckTask(task, "no progress for 5 minutes")

		assert.Equal(t, models.TaskStatusStuck, task.Status)
		assert.NotNil(t, task.LastError)
		assert.Equal(t, "no progress for 5 minutes", *task.LastError)
	})

	t.Run("Notifies about stuck task", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:     "task-1",
			Status: models.TaskStatusRunning,
		}
		_ = repo.Create(context.Background(), task)

		notifier := newMockNotificationService()
		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, notifier, newTestLogger())

		pool.handleStuckTask(task, "stuck reason")

		events := notifier.GetEvents()
		assert.NotEmpty(t, events)
		found := false
		for _, e := range events {
			if e.eventType == models.TaskEventStuck {
				found = true
				break
			}
		}
		assert.True(t, found, "should have stuck event notification")
	})
}

// TestAdaptiveWorkerPool_handleTaskSuccess tests the handleTaskSuccess method
func TestAdaptiveWorkerPool_handleTaskSuccess(t *testing.T) {
	t.Run("Updates task to completed", func(t *testing.T) {
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:       "task-1",
			TaskType: "test-type",
			Status:   models.TaskStatusRunning,
		}
		_ = repo.Create(context.Background(), task)

		worker := &Worker{ID: "worker-1"}
		notifier := newMockNotificationService()
		pool := newTestWorkerPool(nil, newMockTaskQueue(), repo, nil, nil, notifier, newTestLogger())

		pool.handleTaskSuccess(task, worker, 5*time.Second)

		assert.Equal(t, models.TaskStatusCompleted, task.Status)
		assert.Equal(t, float64(100), task.Progress)
		assert.NotNil(t, task.CompletedAt)
		assert.Equal(t, int64(1), worker.TasksCompleted)
	})
}

// TestAdaptiveWorkerPool_handleTaskError tests the handleTaskError method
func TestAdaptiveWorkerPool_handleTaskError(t *testing.T) {
	t.Run("Requeues retryable task", func(t *testing.T) {
		queue := newMockTaskQueue()
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:                "task-1",
			TaskType:          "test-type",
			Status:            models.TaskStatusRunning,
			RetryCount:        0,
			MaxRetries:        3,
			RetryDelaySeconds: 1,
		}
		_ = repo.Create(context.Background(), task)

		worker := &Worker{ID: "worker-1"}
		pool := newTestWorkerPool(nil, queue, repo, nil, nil, nil, newTestLogger())

		pool.handleTaskError(task, worker, errors.New("temporary error"))

		// Task should be requeued
		assert.Equal(t, int64(1), worker.TasksFailed)
	})

	t.Run("Moves to dead letter when max retries exceeded", func(t *testing.T) {
		queue := newMockTaskQueue()
		repo := newMockTaskRepository()
		task := &models.BackgroundTask{
			ID:         "task-1",
			TaskType:   "test-type",
			Status:     models.TaskStatusRunning,
			RetryCount: 3,
			MaxRetries: 3, // Already at max
		}
		_ = repo.Create(context.Background(), task)

		worker := &Worker{ID: "worker-1"}
		notifier := newMockNotificationService()
		pool := newTestWorkerPool(nil, queue, repo, nil, nil, notifier, newTestLogger())

		pool.handleTaskError(task, worker, errors.New("final error"))

		assert.Equal(t, models.TaskStatusFailed, task.Status)
		assert.NotNil(t, task.LastError)
		assert.Equal(t, int64(1), worker.TasksFailed)
	})
}

// TestMin tests the min function
func TestMin_AllCombinations(t *testing.T) {
	tests := []struct {
		a, b, c  int
		expected int
	}{
		{1, 2, 3, 1},     // a is min
		{2, 1, 3, 1},     // b is min
		{3, 2, 1, 1},     // c is min
		{1, 1, 3, 1},     // a = b < c
		{1, 3, 1, 1},     // a = c < b
		{3, 1, 1, 1},     // b = c < a
		{1, 1, 1, 1},     // all equal
		{0, 0, 0, 0},     // all zero
		{-1, 0, 1, -1},   // negative
		{-5, -3, -1, -5}, // all negative
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, min(tc.a, tc.b, tc.c))
	}
}
