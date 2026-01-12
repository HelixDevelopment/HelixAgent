package models

import (
	"encoding/json"
	"time"
)

// TaskStatus represents the lifecycle state of a background task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusPaused     TaskStatus = "paused"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusStuck      TaskStatus = "stuck"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusDeadLetter TaskStatus = "dead_letter"
)

// IsTerminal returns true if the status is a terminal state
func (s TaskStatus) IsTerminal() bool {
	switch s {
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled, TaskStatusDeadLetter:
		return true
	default:
		return false
	}
}

// IsActive returns true if the task is currently active
func (s TaskStatus) IsActive() bool {
	switch s {
	case TaskStatusQueued, TaskStatusRunning:
		return true
	default:
		return false
	}
}

// TaskPriority defines execution priority for background tasks
type TaskPriority string

const (
	TaskPriorityCritical   TaskPriority = "critical"
	TaskPriorityHigh       TaskPriority = "high"
	TaskPriorityNormal     TaskPriority = "normal"
	TaskPriorityLow        TaskPriority = "low"
	TaskPriorityBackground TaskPriority = "background"
)

// Weight returns numeric weight for priority ordering (lower = higher priority)
func (p TaskPriority) Weight() int {
	switch p {
	case TaskPriorityCritical:
		return 0
	case TaskPriorityHigh:
		return 1
	case TaskPriorityNormal:
		return 2
	case TaskPriorityLow:
		return 3
	case TaskPriorityBackground:
		return 4
	default:
		return 2 // default to normal
	}
}

// BackgroundTask represents a background task in the system
type BackgroundTask struct {
	ID            string          `json:"id" db:"id"`
	TaskType      string          `json:"task_type" db:"task_type"`
	TaskName      string          `json:"task_name" db:"task_name"`
	CorrelationID *string         `json:"correlation_id,omitempty" db:"correlation_id"`
	ParentTaskID  *string         `json:"parent_task_id,omitempty" db:"parent_task_id"`

	// Task configuration
	Payload  json.RawMessage `json:"payload" db:"payload"`
	Config   TaskConfig      `json:"config" db:"config"`
	Priority TaskPriority    `json:"priority" db:"priority"`

	// State management
	Status          TaskStatus      `json:"status" db:"status"`
	Progress        float64         `json:"progress" db:"progress"`
	ProgressMessage *string         `json:"progress_message,omitempty" db:"progress_message"`
	Checkpoint      json.RawMessage `json:"checkpoint,omitempty" db:"checkpoint"`

	// Retry configuration
	MaxRetries        int             `json:"max_retries" db:"max_retries"`
	RetryCount        int             `json:"retry_count" db:"retry_count"`
	RetryDelaySeconds int             `json:"retry_delay_seconds" db:"retry_delay_seconds"`
	LastError         *string         `json:"last_error,omitempty" db:"last_error"`
	ErrorHistory      json.RawMessage `json:"error_history" db:"error_history"`

	// Execution tracking
	WorkerID      *string    `json:"worker_id,omitempty" db:"worker_id"`
	ProcessPID    *int       `json:"process_pid,omitempty" db:"process_pid"`
	StartedAt     *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty" db:"last_heartbeat"`
	Deadline      *time.Time `json:"deadline,omitempty" db:"deadline"`

	// Resource requirements
	RequiredCPUCores         int  `json:"required_cpu_cores" db:"required_cpu_cores"`
	RequiredMemoryMB         int  `json:"required_memory_mb" db:"required_memory_mb"`
	EstimatedDurationSeconds *int `json:"estimated_duration_seconds,omitempty" db:"estimated_duration_seconds"`
	ActualDurationSeconds    *int `json:"actual_duration_seconds,omitempty" db:"actual_duration_seconds"`

	// Notification configuration
	NotificationConfig NotificationConfig `json:"notification_config" db:"notification_config"`

	// User association
	UserID    *string         `json:"user_id,omitempty" db:"user_id"`
	SessionID *string         `json:"session_id,omitempty" db:"session_id"`
	Tags      json.RawMessage `json:"tags" db:"tags"`
	Metadata  json.RawMessage `json:"metadata" db:"metadata"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	ScheduledAt time.Time  `json:"scheduled_at" db:"scheduled_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TaskConfig holds task execution configuration
type TaskConfig struct {
	TimeoutSeconds        int  `json:"timeout_seconds"`
	HeartbeatIntervalSecs int  `json:"heartbeat_interval_seconds"`
	StuckThresholdSecs    int  `json:"stuck_threshold_seconds"`
	AllowPause            bool `json:"allow_pause"`
	AllowCancel           bool `json:"allow_cancel"`
	Endless               bool `json:"endless"`
	GracefulShutdownSecs  int  `json:"graceful_shutdown_seconds"`
	CaptureOutput         bool `json:"capture_output"`
	CaptureStderr         bool `json:"capture_stderr"`
}

// DefaultTaskConfig returns a TaskConfig with sensible defaults
func DefaultTaskConfig() TaskConfig {
	return TaskConfig{
		TimeoutSeconds:        1800, // 30 minutes
		HeartbeatIntervalSecs: 10,
		StuckThresholdSecs:    300, // 5 minutes
		AllowPause:            true,
		AllowCancel:           true,
		Endless:               false,
		GracefulShutdownSecs:  30,
		CaptureOutput:         true,
		CaptureStderr:         true,
	}
}

// NotificationConfig holds notification settings for a task
type NotificationConfig struct {
	Webhooks  []WebhookConfig `json:"webhooks,omitempty"`
	SSE       *SSEConfig      `json:"sse,omitempty"`
	WebSocket *WSConfig       `json:"websocket,omitempty"`
	OnEvents  []string        `json:"on_events,omitempty"`
}

// WebhookConfig for webhook notifications
type WebhookConfig struct {
	URL      string            `json:"url"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers,omitempty"`
	Secret   string            `json:"secret,omitempty"`
	Events   []string          `json:"events"`
	RetryMax int               `json:"retry_max"`
}

// SSEConfig for Server-Sent Events
type SSEConfig struct {
	Channel string `json:"channel"`
	Enabled bool   `json:"enabled"`
}

// WSConfig for WebSocket notifications
type WSConfig struct {
	Room    string `json:"room"`
	Enabled bool   `json:"enabled"`
}

// ResourceSnapshot captures resource usage at a point in time
type ResourceSnapshot struct {
	ID     string `json:"id" db:"id"`
	TaskID string `json:"task_id" db:"task_id"`

	// CPU metrics
	CPUPercent    float64 `json:"cpu_percent" db:"cpu_percent"`
	CPUUserTime   float64 `json:"cpu_user_time" db:"cpu_user_time"`
	CPUSystemTime float64 `json:"cpu_system_time" db:"cpu_system_time"`

	// Memory metrics
	MemoryRSSBytes int64   `json:"memory_rss_bytes" db:"memory_rss_bytes"`
	MemoryVMSBytes int64   `json:"memory_vms_bytes" db:"memory_vms_bytes"`
	MemoryPercent  float64 `json:"memory_percent" db:"memory_percent"`

	// I/O metrics
	IOReadBytes  int64 `json:"io_read_bytes" db:"io_read_bytes"`
	IOWriteBytes int64 `json:"io_write_bytes" db:"io_write_bytes"`
	IOReadCount  int64 `json:"io_read_count" db:"io_read_count"`
	IOWriteCount int64 `json:"io_write_count" db:"io_write_count"`

	// Network metrics
	NetBytesSent   int64 `json:"net_bytes_sent" db:"net_bytes_sent"`
	NetBytesRecv   int64 `json:"net_bytes_recv" db:"net_bytes_recv"`
	NetConnections int   `json:"net_connections" db:"net_connections"`

	// File descriptors
	OpenFiles int `json:"open_files" db:"open_files"`
	OpenFDs   int `json:"open_fds" db:"open_fds"`

	// Process state
	ProcessState string `json:"process_state" db:"process_state"`
	ThreadCount  int    `json:"thread_count" db:"thread_count"`

	SampledAt time.Time `json:"sampled_at" db:"sampled_at"`
}

// TaskExecutionHistory records state changes and events for a task
type TaskExecutionHistory struct {
	ID        string          `json:"id" db:"id"`
	TaskID    string          `json:"task_id" db:"task_id"`
	EventType string          `json:"event_type" db:"event_type"`
	EventData json.RawMessage `json:"event_data" db:"event_data"`
	WorkerID  *string         `json:"worker_id,omitempty" db:"worker_id"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

// DeadLetterTask represents a task in the dead-letter queue
type DeadLetterTask struct {
	ID             string          `json:"id" db:"id"`
	OriginalTaskID string          `json:"original_task_id" db:"original_task_id"`
	TaskData       json.RawMessage `json:"task_data" db:"task_data"`
	FailureReason  string          `json:"failure_reason" db:"failure_reason"`
	FailureCount   int             `json:"failure_count" db:"failure_count"`
	MovedAt        time.Time       `json:"moved_at" db:"moved_at"`
	ReprocessAfter *time.Time      `json:"reprocess_after,omitempty" db:"reprocess_after"`
	Reprocessed    bool            `json:"reprocessed" db:"reprocessed"`
}

// WebhookDelivery tracks webhook notification delivery
type WebhookDelivery struct {
	ID            string     `json:"id" db:"id"`
	TaskID        *string    `json:"task_id,omitempty" db:"task_id"`
	WebhookURL    string     `json:"webhook_url" db:"webhook_url"`
	EventType     string     `json:"event_type" db:"event_type"`
	Payload       string     `json:"payload" db:"payload"`
	Status        string     `json:"status" db:"status"`
	Attempts      int        `json:"attempts" db:"attempts"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty" db:"last_attempt_at"`
	LastError     *string    `json:"last_error,omitempty" db:"last_error"`
	ResponseCode  *int       `json:"response_code,omitempty" db:"response_code"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty" db:"delivered_at"`
}

// TaskError represents an error that occurred during task execution
type TaskError struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Code      string    `json:"code,omitempty"`
	Stack     string    `json:"stack,omitempty"`
	Retryable bool      `json:"retryable"`
}

// TaskEvent types for notification filtering
const (
	TaskEventCreated   = "task.created"
	TaskEventStarted   = "task.started"
	TaskEventProgress  = "task.progress"
	TaskEventHeartbeat = "task.heartbeat"
	TaskEventPaused    = "task.paused"
	TaskEventResumed   = "task.resumed"
	TaskEventCompleted = "task.completed"
	TaskEventFailed    = "task.failed"
	TaskEventStuck     = "task.stuck"
	TaskEventCancelled = "task.cancelled"
	TaskEventRetrying  = "task.retrying"
	TaskEventLog       = "task.log"
	TaskEventResource  = "task.resource"
)

// TaskLogEntry represents a log entry from a task
type TaskLogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"` // debug, info, warn, error
	Source    string                 `json:"source"`
	Message   string                 `json:"message"`
	LineNum   int                    `json:"line_num"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// TaskProgressUpdate represents a progress update from a task
type TaskProgressUpdate struct {
	TaskID            string  `json:"task_id"`
	Progress          float64 `json:"progress"` // 0-100
	Message           string  `json:"message,omitempty"`
	CurrentStep       string  `json:"current_step,omitempty"`
	TotalSteps        int     `json:"total_steps,omitempty"`
	CurrentStepNumber int     `json:"current_step_number,omitempty"`
	TokensGenerated   int     `json:"tokens_generated,omitempty"`
	TokensPerSecond   float64 `json:"tokens_per_second,omitempty"`
	EstimatedRemaining int64  `json:"estimated_remaining_ms,omitempty"`
}

// NewBackgroundTask creates a new BackgroundTask with default values
func NewBackgroundTask(taskType, taskName string, payload json.RawMessage) *BackgroundTask {
	now := time.Now()
	config := DefaultTaskConfig()

	return &BackgroundTask{
		TaskType:           taskType,
		TaskName:           taskName,
		Payload:            payload,
		Config:             config,
		Priority:           TaskPriorityNormal,
		Status:             TaskStatusPending,
		Progress:           0,
		MaxRetries:         3,
		RetryCount:         0,
		RetryDelaySeconds:  60,
		ErrorHistory:       json.RawMessage("[]"),
		RequiredCPUCores:   1,
		RequiredMemoryMB:   512,
		NotificationConfig: NotificationConfig{},
		Tags:               json.RawMessage("[]"),
		Metadata:           json.RawMessage("{}"),
		CreatedAt:          now,
		UpdatedAt:          now,
		ScheduledAt:        now,
	}
}

// CanRetry returns true if the task can be retried
func (t *BackgroundTask) CanRetry() bool {
	return t.RetryCount < t.MaxRetries
}

// CanPause returns true if the task can be paused
func (t *BackgroundTask) CanPause() bool {
	return t.Config.AllowPause && t.Status == TaskStatusRunning
}

// CanCancel returns true if the task can be cancelled
func (t *BackgroundTask) CanCancel() bool {
	return t.Config.AllowCancel && !t.Status.IsTerminal()
}

// CanResume returns true if the task can be resumed
func (t *BackgroundTask) CanResume() bool {
	return t.Status == TaskStatusPaused
}

// Duration returns the actual duration of the task if available
func (t *BackgroundTask) Duration() *time.Duration {
	if t.StartedAt == nil {
		return nil
	}

	var endTime time.Time
	if t.CompletedAt != nil {
		endTime = *t.CompletedAt
	} else if t.Status.IsTerminal() {
		endTime = t.UpdatedAt
	} else {
		endTime = time.Now()
	}

	d := endTime.Sub(*t.StartedAt)
	return &d
}

// IsOverdue returns true if the task has exceeded its deadline
func (t *BackgroundTask) IsOverdue() bool {
	if t.Deadline == nil {
		return false
	}
	return time.Now().After(*t.Deadline)
}

// HasStaleHeartbeat returns true if the heartbeat is older than the threshold
func (t *BackgroundTask) HasStaleHeartbeat(threshold time.Duration) bool {
	if t.LastHeartbeat == nil {
		return true
	}
	return time.Since(*t.LastHeartbeat) > threshold
}
