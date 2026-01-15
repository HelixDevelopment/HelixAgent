package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/background"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/notifications"
)

// mockTaskRepository is a test implementation of TaskRepository
type mockTaskRepository struct {
	tasks              map[string]*models.BackgroundTask
	history            map[string][]*models.TaskExecutionHistory
	resourceSnapshots  map[string][]*models.ResourceSnapshot
	statusCounts       map[models.TaskStatus]int64
	getByIDError       error
	updateError        error
	deleteError        error
	getByStatusError   error
	countByStatusError error
	historyError       error
	snapshotsError     error
}

func newMockTaskRepository() *mockTaskRepository {
	return &mockTaskRepository{
		tasks:             make(map[string]*models.BackgroundTask),
		history:           make(map[string][]*models.TaskExecutionHistory),
		resourceSnapshots: make(map[string][]*models.ResourceSnapshot),
		statusCounts:      make(map[models.TaskStatus]int64),
	}
}

func (m *mockTaskRepository) Create(ctx context.Context, task *models.BackgroundTask) error {
	m.tasks[task.ID] = task
	return nil
}

func (m *mockTaskRepository) GetByID(ctx context.Context, id string) (*models.BackgroundTask, error) {
	if m.getByIDError != nil {
		return nil, m.getByIDError
	}
	task, exists := m.tasks[id]
	if !exists {
		return nil, errors.New("task not found")
	}
	return task, nil
}

func (m *mockTaskRepository) Update(ctx context.Context, task *models.BackgroundTask) error {
	if m.updateError != nil {
		return m.updateError
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *mockTaskRepository) Delete(ctx context.Context, id string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.tasks, id)
	return nil
}

func (m *mockTaskRepository) UpdateStatus(ctx context.Context, id string, status models.TaskStatus) error {
	if task, exists := m.tasks[id]; exists {
		task.Status = status
	}
	return nil
}

func (m *mockTaskRepository) UpdateProgress(ctx context.Context, id string, progress float64, message string) error {
	if task, exists := m.tasks[id]; exists {
		task.Progress = progress
		task.ProgressMessage = &message
	}
	return nil
}

func (m *mockTaskRepository) UpdateHeartbeat(ctx context.Context, id string) error {
	return nil
}

func (m *mockTaskRepository) SaveCheckpoint(ctx context.Context, id string, checkpoint []byte) error {
	return nil
}

func (m *mockTaskRepository) GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.BackgroundTask, error) {
	if m.getByStatusError != nil {
		return nil, m.getByStatusError
	}
	var result []*models.BackgroundTask
	for _, task := range m.tasks {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}
	return result, nil
}

func (m *mockTaskRepository) GetPendingTasks(ctx context.Context, limit int) ([]*models.BackgroundTask, error) {
	return m.GetByStatus(ctx, models.TaskStatusPending, limit, 0)
}

func (m *mockTaskRepository) GetStaleTasks(ctx context.Context, threshold time.Duration) ([]*models.BackgroundTask, error) {
	return nil, nil
}

func (m *mockTaskRepository) GetByWorkerID(ctx context.Context, workerID string) ([]*models.BackgroundTask, error) {
	return nil, nil
}

func (m *mockTaskRepository) CountByStatus(ctx context.Context) (map[models.TaskStatus]int64, error) {
	if m.countByStatusError != nil {
		return nil, m.countByStatusError
	}
	return m.statusCounts, nil
}

func (m *mockTaskRepository) Dequeue(ctx context.Context, workerID string, maxCPUCores, maxMemoryMB int) (*models.BackgroundTask, error) {
	return nil, nil
}

func (m *mockTaskRepository) SaveResourceSnapshot(ctx context.Context, snapshot *models.ResourceSnapshot) error {
	return nil
}

func (m *mockTaskRepository) GetResourceSnapshots(ctx context.Context, taskID string, limit int) ([]*models.ResourceSnapshot, error) {
	if m.snapshotsError != nil {
		return nil, m.snapshotsError
	}
	return m.resourceSnapshots[taskID], nil
}

func (m *mockTaskRepository) LogEvent(ctx context.Context, taskID, eventType string, data map[string]interface{}, workerID *string) error {
	eventData, _ := json.Marshal(data)
	m.history[taskID] = append(m.history[taskID], &models.TaskExecutionHistory{
		TaskID:    taskID,
		EventType: eventType,
		EventData: eventData,
		WorkerID:  workerID,
		CreatedAt: time.Now(),
	})
	return nil
}

func (m *mockTaskRepository) GetTaskHistory(ctx context.Context, taskID string, limit int) ([]*models.TaskExecutionHistory, error) {
	if m.historyError != nil {
		return nil, m.historyError
	}
	history, exists := m.history[taskID]
	if !exists {
		return nil, errors.New("task not found")
	}
	return history, nil
}

func (m *mockTaskRepository) MoveToDeadLetter(ctx context.Context, taskID, reason string) error {
	return nil
}

// mockTaskQueue is a test implementation of TaskQueue
type mockTaskQueue struct {
	tasks          []*models.BackgroundTask
	enqueueError   error
	dequeueError   error
	requeueError   error
	pendingCount   int64
	runningCount   int64
	queueDepth     map[models.TaskPriority]int64
}

func newMockTaskQueue() *mockTaskQueue {
	return &mockTaskQueue{
		tasks:      make([]*models.BackgroundTask, 0),
		queueDepth: make(map[models.TaskPriority]int64),
	}
}

func (m *mockTaskQueue) Enqueue(ctx context.Context, task *models.BackgroundTask) error {
	if m.enqueueError != nil {
		return m.enqueueError
	}
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *mockTaskQueue) Dequeue(ctx context.Context, workerID string, requirements background.ResourceRequirements) (*models.BackgroundTask, error) {
	if m.dequeueError != nil {
		return nil, m.dequeueError
	}
	if len(m.tasks) == 0 {
		return nil, nil
	}
	task := m.tasks[0]
	m.tasks = m.tasks[1:]
	return task, nil
}

func (m *mockTaskQueue) Peek(ctx context.Context, count int) ([]*models.BackgroundTask, error) {
	return m.tasks, nil
}

func (m *mockTaskQueue) Requeue(ctx context.Context, taskID string, delay time.Duration) error {
	return m.requeueError
}

func (m *mockTaskQueue) MoveToDeadLetter(ctx context.Context, taskID string, reason string) error {
	return nil
}

func (m *mockTaskQueue) GetPendingCount(ctx context.Context) (int64, error) {
	return m.pendingCount, nil
}

func (m *mockTaskQueue) GetRunningCount(ctx context.Context) (int64, error) {
	return m.runningCount, nil
}

func (m *mockTaskQueue) GetQueueDepth(ctx context.Context) (map[models.TaskPriority]int64, error) {
	return m.queueDepth, nil
}

// mockWorkerPool is a test implementation of WorkerPool
type mockWorkerPool struct {
	workerCount     int
	activeTaskCount int
	workerStatus    []background.WorkerStatus
}

func newMockWorkerPool() *mockWorkerPool {
	return &mockWorkerPool{
		workerCount:     5,
		activeTaskCount: 2,
		workerStatus: []background.WorkerStatus{
			{ID: "worker-1", Status: "idle"},
			{ID: "worker-2", Status: "busy"},
		},
	}
}

func (m *mockWorkerPool) Start(ctx context.Context) error { return nil }
func (m *mockWorkerPool) Stop(gracePeriod time.Duration) error { return nil }
func (m *mockWorkerPool) RegisterExecutor(taskType string, executor background.TaskExecutor) {}
func (m *mockWorkerPool) GetWorkerCount() int { return m.workerCount }
func (m *mockWorkerPool) GetActiveTaskCount() int { return m.activeTaskCount }
func (m *mockWorkerPool) GetWorkerStatus() []background.WorkerStatus { return m.workerStatus }
func (m *mockWorkerPool) Scale(targetCount int) error { return nil }

// mockResourceMonitor is a test implementation of ResourceMonitor
type mockResourceMonitor struct {
	resources *background.SystemResources
}

func newMockResourceMonitor() *mockResourceMonitor {
	return &mockResourceMonitor{
		resources: &background.SystemResources{
			TotalCPUCores:     8,
			AvailableCPUCores: 4.0,
			TotalMemoryMB:     16384,
			AvailableMemoryMB: 8192,
			CPULoadPercent:    50.0,
			MemoryUsedPercent: 50.0,
		},
	}
}

func (m *mockResourceMonitor) GetSystemResources() (*background.SystemResources, error) {
	return m.resources, nil
}
func (m *mockResourceMonitor) GetProcessResources(pid int) (*models.ResourceSnapshot, error) { return nil, nil }
func (m *mockResourceMonitor) StartMonitoring(taskID string, pid int, interval time.Duration) error { return nil }
func (m *mockResourceMonitor) StopMonitoring(taskID string) error { return nil }
func (m *mockResourceMonitor) GetLatestSnapshot(taskID string) (*models.ResourceSnapshot, error) { return nil, nil }
func (m *mockResourceMonitor) IsResourceAvailable(requirements background.ResourceRequirements) bool { return true }

// mockStuckDetector is a test implementation of StuckDetector
type mockStuckDetector struct {
	isStuck  bool
	reason   string
}

func newMockStuckDetector() *mockStuckDetector {
	return &mockStuckDetector{
		isStuck: false,
		reason:  "",
	}
}

func (m *mockStuckDetector) IsStuck(ctx context.Context, task *models.BackgroundTask, snapshots []*models.ResourceSnapshot) (bool, string) {
	return m.isStuck, m.reason
}

func (m *mockStuckDetector) GetStuckThreshold(taskType string) time.Duration {
	return 5 * time.Minute
}

func (m *mockStuckDetector) SetThreshold(taskType string, threshold time.Duration) {}

// setupTestHandler creates a test handler with mock dependencies
func setupTestHandler() (*BackgroundTaskHandler, *mockTaskRepository, *mockTaskQueue) {
	repo := newMockTaskRepository()
	queue := newMockTaskQueue()
	workerPool := newMockWorkerPool()
	resourceMonitor := newMockResourceMonitor()
	stuckDetector := newMockStuckDetector()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := &BackgroundTaskHandler{
		repository:      repo,
		queue:           queue,
		workerPool:      workerPool,
		resourceMonitor: resourceMonitor,
		stuckDetector:   stuckDetector,
		logger:          logger,
	}

	return handler, repo, queue
}

// TestCreateTask tests the CreateTask handler
func TestCreateTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Creates task successfully", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		body := CreateTaskRequest{
			TaskType: "test-task",
			TaskName: "Test Task",
			Payload:  map[string]interface{}{"key": "value"},
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.CreateTask(c)

		assert.Equal(t, http.StatusAccepted, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "test-task", response["task_type"])
		assert.Equal(t, "Test Task", response["task_name"])
		assert.NotEmpty(t, response["id"])
		assert.Equal(t, string(models.TaskStatusPending), response["status"])
	})

	t.Run("Returns error with invalid request body", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.CreateTask(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Returns error with missing required fields", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		body := CreateTaskRequest{
			TaskType: "test-task",
			// TaskName missing
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.CreateTask(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Returns error when queue fails", func(t *testing.T) {
		handler, _, queue := setupTestHandler()
		queue.enqueueError = errors.New("queue error")

		body := CreateTaskRequest{
			TaskType: "test-task",
			TaskName: "Test Task",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.CreateTask(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Handles custom priority", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		body := CreateTaskRequest{
			TaskType: "test-task",
			TaskName: "High Priority Task",
			Priority: "high",
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.CreateTask(c)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run("Handles task config options", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		body := CreateTaskRequest{
			TaskType: "test-task",
			TaskName: "Configured Task",
			Config: &TaskConfigRequest{
				TimeoutSeconds:    300,
				MaxRetries:        5,
				RetryDelaySeconds: 30,
				Endless:           false,
				AllowPause:        true,
				AllowCancel:       true,
			},
		}
		bodyBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.CreateTask(c)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})
}

// TestGetTask tests the GetTask handler
func TestGetTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns task successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:        "test-task-id",
			TaskType:  "test-task",
			TaskName:  "Test Task",
			Status:    models.TaskStatusRunning,
			Priority:  models.TaskPriorityNormal,
			Progress:  50.0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/test-task-id", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.GetTask(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "test-task-id", response["id"])
		assert.Equal(t, "test-task", response["task_type"])
		assert.Equal(t, float64(50.0), response["progress"])
	})

	t.Run("Returns 404 for non-existent task", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/non-existent", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		handler.GetTask(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestGetTaskStatus tests the GetTaskStatus handler
func TestGetTaskStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns status successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		startedAt := time.Now().Add(-5 * time.Minute)
		progressMsg := "Processing items..."
		task := &models.BackgroundTask{
			ID:              "test-task-id",
			Status:          models.TaskStatusRunning,
			Progress:        75.0,
			ProgressMessage: &progressMsg,
			StartedAt:       &startedAt,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/test-task-id/status", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.GetTaskStatus(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "test-task-id", response["id"])
		assert.Equal(t, string(models.TaskStatusRunning), response["status"])
		assert.Equal(t, float64(75.0), response["progress"])
		assert.Equal(t, "Processing items...", response["progress_message"])
	})

	t.Run("Returns 404 for non-existent task", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/non-existent/status", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		handler.GetTaskStatus(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestGetTaskLogs tests the GetTaskLogs handler
func TestGetTaskLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns logs successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		taskID := "test-task-id"
		eventData1, _ := json.Marshal(map[string]interface{}{"worker": "worker-1"})
		eventData2, _ := json.Marshal(map[string]interface{}{"percent": 50})
		repo.history[taskID] = []*models.TaskExecutionHistory{
			{
				TaskID:    taskID,
				EventType: "started",
				EventData: eventData1,
				CreatedAt: time.Now(),
			},
			{
				TaskID:    taskID,
				EventType: "progress",
				EventData: eventData2,
				CreatedAt: time.Now(),
			},
		}

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/test-task-id/logs", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: taskID}}

		handler.GetTaskLogs(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, taskID, response["task_id"])
		assert.Equal(t, float64(2), response["count"])
	})

	t.Run("Returns 404 for non-existent task", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/non-existent/logs", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		handler.GetTaskLogs(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestGetTaskResources tests the GetTaskResources handler
func TestGetTaskResources(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns resources successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		taskID := "test-task-id"
		repo.resourceSnapshots[taskID] = []*models.ResourceSnapshot{
			{
				TaskID:         taskID,
				CPUPercent:     25.5,
				MemoryRSSBytes: 104857600,
				MemoryPercent:  12.5,
				SampledAt:      time.Now(),
			},
		}

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/test-task-id/resources", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: taskID}}

		handler.GetTaskResources(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, taskID, response["task_id"])
		assert.Equal(t, float64(1), response["count"])
	})
}

// TestPauseTask tests the PauseTask handler
func TestPauseTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Pauses task successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:       "test-task-id",
			Status:   models.TaskStatusRunning,
			Config:   models.TaskConfig{AllowPause: true},
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/test-task-id/pause", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.PauseTask(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Task paused", response["message"])
		assert.Equal(t, string(models.TaskStatusPaused), response["status"])
	})

	t.Run("Returns 404 for non-existent task", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/non-existent/pause", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		handler.PauseTask(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Returns error if task cannot be paused", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:     "test-task-id",
			Status: models.TaskStatusCompleted,
			Config: models.TaskConfig{AllowPause: false},
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/test-task-id/pause", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.PauseTask(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestResumeTask tests the ResumeTask handler
func TestResumeTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Resumes task successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:     "test-task-id",
			Status: models.TaskStatusPaused,
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/test-task-id/resume", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.ResumeTask(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Task resumed", response["message"])
	})

	t.Run("Returns 404 for non-existent task", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/non-existent/resume", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		handler.ResumeTask(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Returns error if task is not paused", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:     "test-task-id",
			Status: models.TaskStatusRunning,
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/test-task-id/resume", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.ResumeTask(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestCancelTask tests the CancelTask handler
func TestCancelTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Cancels task successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:     "test-task-id",
			Status: models.TaskStatusRunning,
			Config: models.TaskConfig{AllowCancel: true},
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/test-task-id/cancel", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.CancelTask(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Task cancelled", response["message"])
		assert.Equal(t, string(models.TaskStatusCancelled), response["status"])
	})

	t.Run("Returns 404 for non-existent task", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/non-existent/cancel", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		handler.CancelTask(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Returns error if task cannot be cancelled", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:     "test-task-id",
			Status: models.TaskStatusCompleted,
			Config: models.TaskConfig{AllowCancel: false},
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks/test-task-id/cancel", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.CancelTask(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestDeleteTask tests the DeleteTask handler
func TestDeleteTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Deletes completed task successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:     "test-task-id",
			Status: models.TaskStatusCompleted,
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodDelete, "/v1/tasks/test-task-id", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.DeleteTask(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Returns 404 for non-existent task", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodDelete, "/v1/tasks/non-existent", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		handler.DeleteTask(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Returns error if task is still running", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:     "test-task-id",
			Status: models.TaskStatusRunning,
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodDelete, "/v1/tasks/test-task-id", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.DeleteTask(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Returns error if task is pending", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:     "test-task-id",
			Status: models.TaskStatusPending,
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodDelete, "/v1/tasks/test-task-id", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.DeleteTask(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestListTasks tests the ListTasks handler
func TestListTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns all tasks", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		repo.tasks["task-1"] = &models.BackgroundTask{
			ID:        "task-1",
			TaskType:  "type-a",
			Status:    models.TaskStatusRunning,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repo.tasks["task-2"] = &models.BackgroundTask{
			ID:        "task-2",
			TaskType:  "type-b",
			Status:    models.TaskStatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.ListTasks(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["count"])
	})

	t.Run("Filters by status", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		repo.tasks["task-1"] = &models.BackgroundTask{
			ID:        "task-1",
			Status:    models.TaskStatusRunning,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repo.tasks["task-2"] = &models.BackgroundTask{
			ID:        "task-2",
			Status:    models.TaskStatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks?status=running", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.ListTasks(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(1), response["count"])
	})
}

// TestGetQueueStats tests the GetQueueStats handler
func TestGetQueueStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns queue statistics", func(t *testing.T) {
		handler, _, queue := setupTestHandler()
		queue.pendingCount = 10
		queue.runningCount = 5
		queue.queueDepth[models.TaskPriorityNormal] = 8
		queue.queueDepth[models.TaskPriorityHigh] = 2

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/queue/stats", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.GetQueueStats(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(10), response["pending_count"])
		assert.Equal(t, float64(5), response["running_count"])
		assert.Equal(t, float64(5), response["workers_active"])
		assert.Equal(t, float64(2), response["active_task_count"])
	})
}

// TestPollEvents tests the PollEvents handler
func TestPollEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns 503 when polling not available", func(t *testing.T) {
		handler, _, _ := setupTestHandler()
		handler.pollingStore = nil

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/events", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.PollEvents(c)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("Returns events when polling available", func(t *testing.T) {
		handler, _, _ := setupTestHandler()
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)
		handler.pollingStore = notifications.NewPollingStore(nil, logger)

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/events?task_id=test-id", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.PollEvents(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestAnalyzeTask tests the AnalyzeTask handler
func TestAnalyzeTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Analyzes task successfully", func(t *testing.T) {
		handler, repo, _ := setupTestHandler()

		task := &models.BackgroundTask{
			ID:        "test-task-id",
			Status:    models.TaskStatusRunning,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repo.tasks[task.ID] = task

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/test-task-id/analyze", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "test-task-id"}}

		handler.AnalyzeTask(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Returns 404 for non-existent task", func(t *testing.T) {
		handler, _, _ := setupTestHandler()

		req, _ := http.NewRequest(http.MethodGet, "/v1/tasks/non-existent/analyze", nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

		handler.AnalyzeTask(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestTaskToResponse tests the internal taskToResponse helper
func TestTaskToResponse(t *testing.T) {
	handler, _, _ := setupTestHandler()

	startedAt := time.Now().Add(-10 * time.Minute)
	completedAt := time.Now()
	deadline := time.Now().Add(1 * time.Hour)
	correlationID := "corr-123"
	parentTaskID := "parent-456"
	workerID := "worker-1"
	processPID := 12345
	lastError := "some error"
	progressMsg := "Processing..."

	task := &models.BackgroundTask{
		ID:              "test-id",
		TaskType:        "test-type",
		TaskName:        "Test Task",
		CorrelationID:   &correlationID,
		ParentTaskID:    &parentTaskID,
		Status:          models.TaskStatusFailed,
		Priority:        models.TaskPriorityHigh,
		Progress:        75.0,
		ProgressMessage: &progressMsg,
		WorkerID:        &workerID,
		ProcessPID:      &processPID,
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		Deadline:        &deadline,
		LastError:       &lastError,
		RetryCount:      2,
		MaxRetries:      3,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ScheduledAt:     time.Now(),
	}

	response := handler.taskToResponse(task)

	assert.Equal(t, "test-id", response["id"])
	assert.Equal(t, "test-type", response["task_type"])
	assert.Equal(t, "Test Task", response["task_name"])
	assert.Equal(t, correlationID, response["correlation_id"])
	assert.Equal(t, parentTaskID, response["parent_task_id"])
	assert.Equal(t, workerID, response["worker_id"])
	assert.Equal(t, processPID, response["process_pid"])
	assert.Equal(t, lastError, response["last_error"])
	assert.Equal(t, 2, response["retry_count"])
	assert.Equal(t, 3, response["max_retries"])
	assert.NotNil(t, response["started_at"])
	assert.NotNil(t, response["completed_at"])
	assert.NotNil(t, response["deadline"])
}

// TestNilIfEmpty tests the nilIfEmpty helper
func TestNilIfEmpty(t *testing.T) {
	t.Run("Returns nil for empty string", func(t *testing.T) {
		result := nilIfEmpty("")
		assert.Nil(t, result)
	})

	t.Run("Returns pointer for non-empty string", func(t *testing.T) {
		result := nilIfEmpty("test")
		assert.NotNil(t, result)
		assert.Equal(t, "test", *result)
	})
}

// TestRegisterRoutes tests route registration
func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _, _ := setupTestHandler()
	router := gin.New()
	rg := router.Group("/v1")
	handler.RegisterRoutes(rg)

	// Verify routes are registered
	routes := router.Routes()

	expectedRoutes := map[string]bool{
		"POST /v1/tasks":            false,
		"GET /v1/tasks":             false,
		"GET /v1/tasks/queue/stats": false,
		"GET /v1/tasks/events":      false,
		"GET /v1/tasks/:id":         false,
		"GET /v1/tasks/:id/status":  false,
		"GET /v1/tasks/:id/logs":    false,
		"DELETE /v1/tasks/:id":      false,
	}

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if _, exists := expectedRoutes[key]; exists {
			expectedRoutes[key] = true
		}
	}

	for route, found := range expectedRoutes {
		assert.True(t, found, "Route %s should be registered", route)
	}
}

// TestNewBackgroundTaskHandler tests handler construction
func TestNewBackgroundTaskHandler(t *testing.T) {
	repo := newMockTaskRepository()
	queue := newMockTaskQueue()
	workerPool := newMockWorkerPool()
	resourceMonitor := newMockResourceMonitor()
	stuckDetector := newMockStuckDetector()
	logger := logrus.New()

	handler := NewBackgroundTaskHandler(
		repo,
		queue,
		workerPool,
		resourceMonitor,
		stuckDetector,
		nil, // notificationHub
		nil, // sseManager
		nil, // wsServer
		nil, // webhookDispatcher
		nil, // pollingStore
		logger,
	)

	assert.NotNil(t, handler)
	assert.Equal(t, repo, handler.repository)
	assert.Equal(t, queue, handler.queue)
	assert.Equal(t, workerPool, handler.workerPool)
	assert.Equal(t, resourceMonitor, handler.resourceMonitor)
	assert.Equal(t, stuckDetector, handler.stuckDetector)
	assert.Equal(t, logger, handler.logger)
}
