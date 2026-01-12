package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/background"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/notifications"
	"dev.helix.agent/internal/notifications/cli"
)

// BackgroundTaskHandler handles background task API endpoints
type BackgroundTaskHandler struct {
	repository       background.TaskRepository
	queue            background.TaskQueue
	workerPool       background.WorkerPool
	resourceMonitor  background.ResourceMonitor
	stuckDetector    background.StuckDetector
	notificationHub  *notifications.NotificationHub
	sseManager       *notifications.SSEManager
	wsServer         *notifications.WebSocketServer
	webhookDispatcher *notifications.WebhookDispatcher
	pollingStore     *notifications.PollingStore
	cliRenderer      *cli.Renderer
	logger           *logrus.Logger
}

// NewBackgroundTaskHandler creates a new background task handler
func NewBackgroundTaskHandler(
	repository background.TaskRepository,
	queue background.TaskQueue,
	workerPool background.WorkerPool,
	resourceMonitor background.ResourceMonitor,
	stuckDetector background.StuckDetector,
	notificationHub *notifications.NotificationHub,
	sseManager *notifications.SSEManager,
	wsServer *notifications.WebSocketServer,
	webhookDispatcher *notifications.WebhookDispatcher,
	pollingStore *notifications.PollingStore,
	logger *logrus.Logger,
) *BackgroundTaskHandler {
	return &BackgroundTaskHandler{
		repository:        repository,
		queue:             queue,
		workerPool:        workerPool,
		resourceMonitor:   resourceMonitor,
		stuckDetector:     stuckDetector,
		notificationHub:   notificationHub,
		sseManager:        sseManager,
		wsServer:          wsServer,
		webhookDispatcher: webhookDispatcher,
		pollingStore:      pollingStore,
		cliRenderer:       cli.NewRenderer(nil, cli.DetectCLIClient()),
		logger:            logger,
	}
}

// CreateTaskRequest represents the request to create a task
type CreateTaskRequest struct {
	TaskType           string                         `json:"task_type" binding:"required"`
	TaskName           string                         `json:"task_name" binding:"required"`
	CorrelationID      string                         `json:"correlation_id,omitempty"`
	ParentTaskID       string                         `json:"parent_task_id,omitempty"`
	Payload            map[string]interface{}         `json:"payload,omitempty"`
	Config             *TaskConfigRequest             `json:"config,omitempty"`
	Priority           string                         `json:"priority,omitempty"`
	ScheduledAt        *time.Time                     `json:"scheduled_at,omitempty"`
	Deadline           *time.Time                     `json:"deadline,omitempty"`
	RequiredCPUCores   int                            `json:"required_cpu_cores,omitempty"`
	RequiredMemoryMB   int                            `json:"required_memory_mb,omitempty"`
	NotificationConfig *NotificationConfigRequest     `json:"notification_config,omitempty"`
}

// TaskConfigRequest holds task configuration options
type TaskConfigRequest struct {
	TimeoutSeconds      int      `json:"timeout_seconds,omitempty"`
	MaxRetries          int      `json:"max_retries,omitempty"`
	RetryDelaySeconds   int      `json:"retry_delay_seconds,omitempty"`
	Endless             bool     `json:"endless,omitempty"`
	AllowPause          bool     `json:"allow_pause,omitempty"`
	AllowCancel         bool     `json:"allow_cancel,omitempty"`
	StuckThresholdSecs  int      `json:"stuck_threshold_secs,omitempty"`
	HeartbeatIntervalSecs int    `json:"heartbeat_interval_secs,omitempty"`
	Tags                []string `json:"tags,omitempty"`
}

// NotificationConfigRequest holds notification settings
type NotificationConfigRequest struct {
	EnableSSE       bool                    `json:"enable_sse,omitempty"`
	EnableWebSocket bool                    `json:"enable_websocket,omitempty"`
	EnablePolling   bool                    `json:"enable_polling,omitempty"`
	Webhooks        []WebhookConfigRequest  `json:"webhooks,omitempty"`
}

// WebhookConfigRequest holds webhook configuration
type WebhookConfigRequest struct {
	URL     string            `json:"url" binding:"required"`
	Secret  string            `json:"secret,omitempty"`
	Events  []string          `json:"events,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// CreateTask handles POST /v1/tasks
func (h *BackgroundTaskHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Invalid request body",
				"details": err.Error(),
			},
		})
		return
	}

	// Generate task ID
	taskID := uuid.New().String()

	// Set defaults
	priority := models.TaskPriority(req.Priority)
	if priority.Weight() < 0 || priority.Weight() > 4 {
		priority = models.TaskPriorityNormal
	}

	scheduledAt := time.Now()
	if req.ScheduledAt != nil {
		scheduledAt = *req.ScheduledAt
	}

	maxRetries := 3
	if req.Config != nil && req.Config.MaxRetries > 0 {
		maxRetries = req.Config.MaxRetries
	}

	// Build task
	task := &models.BackgroundTask{
		ID:               taskID,
		TaskType:         req.TaskType,
		TaskName:         req.TaskName,
		CorrelationID:    nilIfEmpty(req.CorrelationID),
		ParentTaskID:     nilIfEmpty(req.ParentTaskID),
		Priority:         priority,
		Status:           models.TaskStatusPending,
		ScheduledAt:      scheduledAt,
		Deadline:         req.Deadline,
		MaxRetries:       maxRetries,
		RequiredCPUCores: req.RequiredCPUCores,
		RequiredMemoryMB: req.RequiredMemoryMB,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Set payload
	if req.Payload != nil {
		payloadBytes, _ := json.Marshal(req.Payload)
		task.Payload = payloadBytes
	}

	// Set config
	if req.Config != nil {
		task.Config = models.TaskConfig{
			TimeoutSeconds:        req.Config.TimeoutSeconds,
			Endless:               req.Config.Endless,
			AllowPause:            req.Config.AllowPause,
			AllowCancel:           req.Config.AllowCancel,
			StuckThresholdSecs:    req.Config.StuckThresholdSecs,
			HeartbeatIntervalSecs: req.Config.HeartbeatIntervalSecs,
		}
		// Set retry config on task itself
		if req.Config.MaxRetries > 0 {
			task.MaxRetries = req.Config.MaxRetries
		}
		if req.Config.RetryDelaySeconds > 0 {
			task.RetryDelaySeconds = req.Config.RetryDelaySeconds
		}
	}

	// Set notification config
	if req.NotificationConfig != nil {
		task.NotificationConfig = models.NotificationConfig{}

		if req.NotificationConfig.EnableSSE {
			task.NotificationConfig.SSE = &models.SSEConfig{Enabled: true}
		}
		if req.NotificationConfig.EnableWebSocket {
			task.NotificationConfig.WebSocket = &models.WSConfig{Enabled: true}
		}

		for _, wh := range req.NotificationConfig.Webhooks {
			task.NotificationConfig.Webhooks = append(task.NotificationConfig.Webhooks, models.WebhookConfig{
				URL:     wh.URL,
				Secret:  wh.Secret,
				Events:  wh.Events,
				Headers: wh.Headers,
			})
		}
	}

	// Enqueue task
	if err := h.queue.Enqueue(c.Request.Context(), task); err != nil {
		h.logger.WithError(err).Error("Failed to enqueue task")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to create task",
				"details": err.Error(),
			},
		})
		return
	}

	// Register webhooks if configured
	if h.webhookDispatcher != nil && task.NotificationConfig.Webhooks != nil {
		h.webhookDispatcher.LoadWebhooksFromTask(task)
	}

	h.logger.WithFields(logrus.Fields{
		"task_id":   taskID,
		"task_type": req.TaskType,
		"priority":  priority,
	}).Info("Background task created")

	c.JSON(http.StatusAccepted, gin.H{
		"id":           taskID,
		"task_type":    req.TaskType,
		"task_name":    req.TaskName,
		"status":       task.Status,
		"priority":     task.Priority,
		"scheduled_at": task.ScheduledAt.Unix(),
		"created_at":   task.CreatedAt.Unix(),
		"message":      "Task created and queued. Use GET /v1/tasks/" + taskID + " to check status.",
	})
}

// GetTask handles GET /v1/tasks/:id
func (h *BackgroundTaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.repository.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	c.JSON(http.StatusOK, h.taskToResponse(task))
}

// GetTaskStatus handles GET /v1/tasks/:id/status
func (h *BackgroundTaskHandler) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.repository.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	status := gin.H{
		"id":       taskID,
		"status":   task.Status,
		"progress": task.Progress,
	}

	if task.ProgressMessage != nil && *task.ProgressMessage != "" {
		status["progress_message"] = *task.ProgressMessage
	}

	if task.StartedAt != nil {
		status["started_at"] = task.StartedAt.Unix()
		status["duration_seconds"] = int(time.Since(*task.StartedAt).Seconds())
	}

	if task.CompletedAt != nil {
		status["completed_at"] = task.CompletedAt.Unix()
	}

	if task.LastError != nil {
		status["last_error"] = *task.LastError
	}

	if task.RetryCount > 0 {
		status["retry_count"] = task.RetryCount
	}

	c.JSON(http.StatusOK, status)
}

// GetTaskLogs handles GET /v1/tasks/:id/logs
func (h *BackgroundTaskHandler) GetTaskLogs(c *gin.Context) {
	taskID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	history, err := h.repository.GetTaskHistory(c.Request.Context(), taskID, limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	logs := make([]gin.H, 0, len(history))
	for _, h := range history {
		logs = append(logs, gin.H{
			"event_type": h.EventType,
			"event_data": h.EventData,
			"worker_id":  h.WorkerID,
			"created_at": h.CreatedAt.Unix(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
		"logs":    logs,
		"count":   len(logs),
	})
}

// GetTaskResources handles GET /v1/tasks/:id/resources
func (h *BackgroundTaskHandler) GetTaskResources(c *gin.Context) {
	taskID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	snapshots, err := h.repository.GetResourceSnapshots(c.Request.Context(), taskID, limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found or no resource data available",
				"task_id": taskID,
			},
		})
		return
	}

	resources := make([]gin.H, 0, len(snapshots))
	for _, s := range snapshots {
		resources = append(resources, gin.H{
			"cpu_percent":       s.CPUPercent,
			"memory_rss_bytes":  s.MemoryRSSBytes,
			"memory_percent":    s.MemoryPercent,
			"io_read_bytes":     s.IOReadBytes,
			"io_write_bytes":    s.IOWriteBytes,
			"net_bytes_sent":    s.NetBytesSent,
			"net_bytes_recv":    s.NetBytesRecv,
			"open_fds":          s.OpenFDs,
			"thread_count":      s.ThreadCount,
			"process_state":     s.ProcessState,
			"sampled_at":        s.SampledAt.Unix(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":   taskID,
		"resources": resources,
		"count":     len(resources),
	})
}

// GetTaskEvents handles GET /v1/tasks/:id/events (SSE)
func (h *BackgroundTaskHandler) GetTaskEvents(c *gin.Context) {
	taskID := c.Param("id")

	// Verify task exists
	_, err := h.repository.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Create client channel
	clientChan := make(chan []byte, 100)

	// Register client
	if h.sseManager != nil {
		h.sseManager.RegisterClient(taskID, clientChan)
		defer h.sseManager.UnregisterClient(taskID, clientChan)
	}

	// Stream events
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-clientChan:
			if !ok {
				return false
			}
			c.SSEvent("message", string(msg))
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// HandleWebSocket handles GET /v1/ws/tasks/:id
func (h *BackgroundTaskHandler) HandleWebSocket(c *gin.Context) {
	if h.wsServer != nil {
		h.wsServer.HandleConnection(c)
	}
}

// PauseTask handles POST /v1/tasks/:id/pause
func (h *BackgroundTaskHandler) PauseTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.repository.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	if !task.CanPause() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Task cannot be paused",
				"status":  task.Status,
			},
		})
		return
	}

	task.Status = models.TaskStatusPaused
	if err := h.repository.Update(c.Request.Context(), task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to pause task",
				"details": err.Error(),
			},
		})
		return
	}

	h.repository.LogEvent(c.Request.Context(), taskID, models.TaskEventPaused, nil, nil)

	c.JSON(http.StatusOK, gin.H{
		"message": "Task paused",
		"task_id": taskID,
		"status":  task.Status,
	})
}

// ResumeTask handles POST /v1/tasks/:id/resume
func (h *BackgroundTaskHandler) ResumeTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.repository.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	if task.Status != models.TaskStatusPaused {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Task is not paused",
				"status":  task.Status,
			},
		})
		return
	}

	// Requeue the task
	if err := h.queue.Requeue(c.Request.Context(), taskID, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to resume task",
				"details": err.Error(),
			},
		})
		return
	}

	h.repository.LogEvent(c.Request.Context(), taskID, models.TaskEventResumed, nil, nil)

	c.JSON(http.StatusOK, gin.H{
		"message": "Task resumed",
		"task_id": taskID,
	})
}

// CancelTask handles POST /v1/tasks/:id/cancel
func (h *BackgroundTaskHandler) CancelTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.repository.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	if !task.CanCancel() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Task cannot be cancelled",
				"status":  task.Status,
			},
		})
		return
	}

	task.Status = models.TaskStatusCancelled
	now := time.Now()
	task.CompletedAt = &now
	if err := h.repository.Update(c.Request.Context(), task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to cancel task",
				"details": err.Error(),
			},
		})
		return
	}

	h.repository.LogEvent(c.Request.Context(), taskID, models.TaskEventCancelled, nil, nil)

	if h.notificationHub != nil {
		h.notificationHub.NotifyTaskEvent(context.Background(), task, models.TaskEventCancelled, nil)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task cancelled",
		"task_id": taskID,
		"status":  task.Status,
	})
}

// DeleteTask handles DELETE /v1/tasks/:id
func (h *BackgroundTaskHandler) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.repository.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	// Only allow deleting completed/failed/cancelled tasks
	if task.Status == models.TaskStatusRunning || task.Status == models.TaskStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Cannot delete active task. Cancel it first.",
				"status":  task.Status,
			},
		})
		return
	}

	if err := h.repository.Delete(c.Request.Context(), taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to delete task",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task deleted",
		"task_id": taskID,
	})
}

// ListTasks handles GET /v1/tasks
func (h *BackgroundTaskHandler) ListTasks(c *gin.Context) {
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	var tasks []*models.BackgroundTask
	var err error

	if status != "" {
		tasks, err = h.repository.GetByStatus(c.Request.Context(), models.TaskStatus(status), limit, offset)
	} else {
		// Get all tasks (paginated)
		tasks, err = h.repository.GetByStatus(c.Request.Context(), "", limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to list tasks",
				"details": err.Error(),
			},
		})
		return
	}

	taskList := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		taskList = append(taskList, h.taskToResponse(task))
	}

	// Get counts
	counts, _ := h.repository.CountByStatus(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"tasks":  taskList,
		"count":  len(taskList),
		"limit":  limit,
		"offset": offset,
		"status_counts": counts,
	})
}

// GetQueueStats handles GET /v1/tasks/queue/stats
func (h *BackgroundTaskHandler) GetQueueStats(c *gin.Context) {
	ctx := c.Request.Context()

	pending, _ := h.queue.GetPendingCount(ctx)
	running, _ := h.queue.GetRunningCount(ctx)
	depth, _ := h.queue.GetQueueDepth(ctx)
	counts, _ := h.repository.CountByStatus(ctx)

	stats := gin.H{
		"pending_count":     pending,
		"running_count":     running,
		"queue_by_priority": depth,
		"status_counts":     counts,
	}

	if h.workerPool != nil {
		stats["workers_active"] = h.workerPool.GetWorkerCount()
		stats["active_task_count"] = h.workerPool.GetActiveTaskCount()
		stats["worker_status"] = h.workerPool.GetWorkerStatus()
	}

	if h.resourceMonitor != nil {
		resources, err := h.resourceMonitor.GetSystemResources()
		if err == nil {
			stats["system_resources"] = resources
		}
	}

	c.JSON(http.StatusOK, stats)
}

// PollEvents handles GET /v1/tasks/events (polling)
func (h *BackgroundTaskHandler) PollEvents(c *gin.Context) {
	taskID := c.Query("task_id")
	sinceStr := c.Query("since")
	limitStr := c.DefaultQuery("limit", "100")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}

	var since *time.Time
	if sinceStr != "" {
		ts, err := strconv.ParseInt(sinceStr, 10, 64)
		if err == nil {
			t := time.Unix(ts, 0)
			since = &t
		}
	}

	if h.pollingStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Polling not available",
			},
		})
		return
	}

	response := h.pollingStore.Poll(&notifications.PollRequest{
		TaskID: taskID,
		Since:  since,
		Limit:  limit,
	})

	c.JSON(http.StatusOK, response)
}

// RegisterWebhook handles POST /v1/webhooks
func (h *BackgroundTaskHandler) RegisterWebhook(c *gin.Context) {
	var req WebhookConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Invalid request body",
				"details": err.Error(),
			},
		})
		return
	}

	if h.webhookDispatcher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Webhooks not available",
			},
		})
		return
	}

	webhook := &notifications.WebhookRegistration{
		URL:     req.URL,
		Secret:  req.Secret,
		Events:  req.Events,
		Headers: req.Headers,
	}

	if err := h.webhookDispatcher.RegisterWebhook(webhook); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to register webhook",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         webhook.ID,
		"url":        webhook.URL,
		"events":     webhook.Events,
		"created_at": webhook.CreatedAt.Unix(),
	})
}

// ListWebhooks handles GET /v1/webhooks
func (h *BackgroundTaskHandler) ListWebhooks(c *gin.Context) {
	if h.webhookDispatcher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Webhooks not available",
			},
		})
		return
	}

	webhooks := h.webhookDispatcher.ListWebhooks()
	list := make([]gin.H, 0, len(webhooks))

	for _, wh := range webhooks {
		item := gin.H{
			"id":         wh.ID,
			"url":        wh.URL,
			"events":     wh.Events,
			"enabled":    wh.Enabled,
			"created_at": wh.CreatedAt.Unix(),
			"fail_count": wh.FailCount,
		}
		if wh.LastSuccess != nil {
			item["last_success"] = wh.LastSuccess.Unix()
		}
		if wh.LastFailure != nil {
			item["last_failure"] = wh.LastFailure.Unix()
		}
		list = append(list, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"webhooks": list,
		"count":    len(list),
	})
}

// DeleteWebhook handles DELETE /v1/webhooks/:id
func (h *BackgroundTaskHandler) DeleteWebhook(c *gin.Context) {
	webhookID := c.Param("id")

	if h.webhookDispatcher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Webhooks not available",
			},
		})
		return
	}

	if _, exists := h.webhookDispatcher.GetWebhook(webhookID); !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Webhook not found",
			},
		})
		return
	}

	if err := h.webhookDispatcher.UnregisterWebhook(webhookID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Failed to delete webhook",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Webhook deleted",
		"id":      webhookID,
	})
}

// AnalyzeTask handles GET /v1/tasks/:id/analyze
func (h *BackgroundTaskHandler) AnalyzeTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.repository.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Task not found",
				"task_id": taskID,
			},
		})
		return
	}

	snapshots, _ := h.repository.GetResourceSnapshots(c.Request.Context(), taskID, 10)

	if detector, ok := h.stuckDetector.(*background.DefaultStuckDetector); ok {
		analysis := detector.AnalyzeTask(c.Request.Context(), task, snapshots)
		c.JSON(http.StatusOK, analysis)
	} else {
		isStuck, reason := h.stuckDetector.IsStuck(c.Request.Context(), task, snapshots)
		c.JSON(http.StatusOK, gin.H{
			"is_stuck": isStuck,
			"reason":   reason,
		})
	}
}

// taskToResponse converts a task to API response format
func (h *BackgroundTaskHandler) taskToResponse(task *models.BackgroundTask) gin.H {
	response := gin.H{
		"id":           task.ID,
		"task_type":    task.TaskType,
		"task_name":    task.TaskName,
		"status":       task.Status,
		"priority":     task.Priority,
		"progress":     task.Progress,
		"scheduled_at": task.ScheduledAt.Unix(),
		"created_at":   task.CreatedAt.Unix(),
		"updated_at":   task.UpdatedAt.Unix(),
	}

	if task.CorrelationID != nil {
		response["correlation_id"] = *task.CorrelationID
	}

	if task.ParentTaskID != nil {
		response["parent_task_id"] = *task.ParentTaskID
	}

	if task.ProgressMessage != nil && *task.ProgressMessage != "" {
		response["progress_message"] = *task.ProgressMessage
	}

	if task.WorkerID != nil {
		response["worker_id"] = *task.WorkerID
	}

	if task.ProcessPID != nil {
		response["process_pid"] = *task.ProcessPID
	}

	if task.StartedAt != nil {
		response["started_at"] = task.StartedAt.Unix()
	}

	if task.CompletedAt != nil {
		response["completed_at"] = task.CompletedAt.Unix()
	}

	if task.Deadline != nil {
		response["deadline"] = task.Deadline.Unix()
	}

	if task.LastError != nil {
		response["last_error"] = *task.LastError
	}

	if task.RetryCount > 0 {
		response["retry_count"] = task.RetryCount
		response["max_retries"] = task.MaxRetries
	}

	return response
}

// nilIfEmpty returns nil if the string is empty, otherwise returns a pointer to it
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// RegisterRoutes registers background task routes on a router group
func (h *BackgroundTaskHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// Task CRUD
	tasks := rg.Group("/tasks")
	{
		tasks.POST("", h.CreateTask)
		tasks.GET("", h.ListTasks)
		tasks.GET("/queue/stats", h.GetQueueStats)
		tasks.GET("/events", h.PollEvents)

		tasks.GET("/:id", h.GetTask)
		tasks.GET("/:id/status", h.GetTaskStatus)
		tasks.GET("/:id/logs", h.GetTaskLogs)
		tasks.GET("/:id/resources", h.GetTaskResources)
		tasks.GET("/:id/events", h.GetTaskEvents)
		tasks.GET("/:id/analyze", h.AnalyzeTask)

		tasks.POST("/:id/pause", h.PauseTask)
		tasks.POST("/:id/resume", h.ResumeTask)
		tasks.POST("/:id/cancel", h.CancelTask)
		tasks.DELETE("/:id", h.DeleteTask)
	}

	// Webhooks
	webhooks := rg.Group("/webhooks")
	{
		webhooks.POST("", h.RegisterWebhook)
		webhooks.GET("", h.ListWebhooks)
		webhooks.DELETE("/:id", h.DeleteWebhook)
	}

	// WebSocket
	ws := rg.Group("/ws")
	{
		ws.GET("/tasks/:id", h.HandleWebSocket)
		ws.GET("/notifications", h.HandleWebSocket)
	}

	// SSE notifications stream
	notifications := rg.Group("/notifications")
	{
		notifications.GET("/stream", h.streamAllEvents)
	}
}

// streamAllEvents handles GET /v1/notifications/stream (SSE for all events)
func (h *BackgroundTaskHandler) streamAllEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	clientChan := make(chan []byte, 100)

	if h.sseManager != nil {
		h.sseManager.RegisterGlobalClient(clientChan)
		defer h.sseManager.UnregisterGlobalClient(clientChan)
	}

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-clientChan:
			if !ok {
				return false
			}
			c.SSEvent("message", string(msg))
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
