package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/models"
)

// BackgroundTaskRepository handles background task database operations
type BackgroundTaskRepository struct {
	pool *pgxpool.Pool
	log  *logrus.Logger
}

// NewBackgroundTaskRepository creates a new BackgroundTaskRepository
func NewBackgroundTaskRepository(pool *pgxpool.Pool, log *logrus.Logger) *BackgroundTaskRepository {
	return &BackgroundTaskRepository{
		pool: pool,
		log:  log,
	}
}

// Create creates a new background task in the database
func (r *BackgroundTaskRepository) Create(ctx context.Context, task *models.BackgroundTask) error {
	query := `
		INSERT INTO background_tasks (
			task_type, task_name, correlation_id, parent_task_id,
			payload, config, priority, status, progress, progress_message,
			max_retries, retry_delay_seconds, required_cpu_cores, required_memory_mb,
			estimated_duration_seconds, notification_config, user_id, session_id,
			tags, metadata, scheduled_at, deadline
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		RETURNING id, created_at, updated_at
	`

	configJSON, err := json.Marshal(task.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	notificationConfigJSON, err := json.Marshal(task.NotificationConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal notification config: %w", err)
	}

	err = r.pool.QueryRow(ctx, query,
		task.TaskType, task.TaskName, task.CorrelationID, task.ParentTaskID,
		task.Payload, configJSON, task.Priority, task.Status, task.Progress, task.ProgressMessage,
		task.MaxRetries, task.RetryDelaySeconds, task.RequiredCPUCores, task.RequiredMemoryMB,
		task.EstimatedDurationSeconds, notificationConfigJSON, task.UserID, task.SessionID,
		task.Tags, task.Metadata, task.ScheduledAt, task.Deadline,
	).Scan(&task.ID, &task.CreatedAt, &task.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create background task: %w", err)
	}

	r.log.WithFields(logrus.Fields{
		"task_id":   task.ID,
		"task_type": task.TaskType,
		"task_name": task.TaskName,
	}).Debug("Created background task")

	return nil
}

// GetByID retrieves a task by its ID
func (r *BackgroundTaskRepository) GetByID(ctx context.Context, id string) (*models.BackgroundTask, error) {
	query := `
		SELECT id, task_type, task_name, correlation_id, parent_task_id,
			payload, config, priority, status, progress, progress_message, checkpoint,
			max_retries, retry_count, retry_delay_seconds, last_error, error_history,
			worker_id, process_pid, started_at, completed_at, last_heartbeat, deadline,
			required_cpu_cores, required_memory_mb, estimated_duration_seconds, actual_duration_seconds,
			notification_config, user_id, session_id, tags, metadata,
			created_at, updated_at, scheduled_at, deleted_at
		FROM background_tasks
		WHERE id = $1
	`

	task := &models.BackgroundTask{}
	var configJSON, notificationConfigJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&task.ID, &task.TaskType, &task.TaskName, &task.CorrelationID, &task.ParentTaskID,
		&task.Payload, &configJSON, &task.Priority, &task.Status, &task.Progress, &task.ProgressMessage, &task.Checkpoint,
		&task.MaxRetries, &task.RetryCount, &task.RetryDelaySeconds, &task.LastError, &task.ErrorHistory,
		&task.WorkerID, &task.ProcessPID, &task.StartedAt, &task.CompletedAt, &task.LastHeartbeat, &task.Deadline,
		&task.RequiredCPUCores, &task.RequiredMemoryMB, &task.EstimatedDurationSeconds, &task.ActualDurationSeconds,
		&notificationConfigJSON, &task.UserID, &task.SessionID, &task.Tags, &task.Metadata,
		&task.CreatedAt, &task.UpdatedAt, &task.ScheduledAt, &task.DeletedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	if err := json.Unmarshal(configJSON, &task.Config); err != nil {
		r.log.WithError(err).Warn("Failed to unmarshal task config")
	}
	if err := json.Unmarshal(notificationConfigJSON, &task.NotificationConfig); err != nil {
		r.log.WithError(err).Warn("Failed to unmarshal notification config")
	}

	return task, nil
}

// Update updates a task in the database
func (r *BackgroundTaskRepository) Update(ctx context.Context, task *models.BackgroundTask) error {
	query := `
		UPDATE background_tasks
		SET task_type = $2, task_name = $3, correlation_id = $4, parent_task_id = $5,
			payload = $6, config = $7, priority = $8, status = $9, progress = $10,
			progress_message = $11, checkpoint = $12, max_retries = $13, retry_count = $14,
			retry_delay_seconds = $15, last_error = $16, error_history = $17,
			worker_id = $18, process_pid = $19, started_at = $20, completed_at = $21,
			last_heartbeat = $22, deadline = $23, required_cpu_cores = $24, required_memory_mb = $25,
			estimated_duration_seconds = $26, actual_duration_seconds = $27, notification_config = $28,
			user_id = $29, session_id = $30, tags = $31, metadata = $32, scheduled_at = $33
		WHERE id = $1
		RETURNING updated_at
	`

	configJSON, _ := json.Marshal(task.Config)
	notificationConfigJSON, _ := json.Marshal(task.NotificationConfig)

	err := r.pool.QueryRow(ctx, query,
		task.ID, task.TaskType, task.TaskName, task.CorrelationID, task.ParentTaskID,
		task.Payload, configJSON, task.Priority, task.Status, task.Progress,
		task.ProgressMessage, task.Checkpoint, task.MaxRetries, task.RetryCount,
		task.RetryDelaySeconds, task.LastError, task.ErrorHistory,
		task.WorkerID, task.ProcessPID, task.StartedAt, task.CompletedAt,
		task.LastHeartbeat, task.Deadline, task.RequiredCPUCores, task.RequiredMemoryMB,
		task.EstimatedDurationSeconds, task.ActualDurationSeconds, notificationConfigJSON,
		task.UserID, task.SessionID, task.Tags, task.Metadata, task.ScheduledAt,
	).Scan(&task.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

// UpdateStatus updates only the task status
func (r *BackgroundTaskRepository) UpdateStatus(ctx context.Context, id string, status models.TaskStatus) error {
	query := `UPDATE background_tasks SET status = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}
	return nil
}

// UpdateProgress updates task progress
func (r *BackgroundTaskRepository) UpdateProgress(ctx context.Context, id string, progress float64, message string) error {
	query := `UPDATE background_tasks SET progress = $2, progress_message = $3 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, progress, message)
	if err != nil {
		return fmt.Errorf("failed to update progress: %w", err)
	}
	return nil
}

// UpdateHeartbeat updates the last heartbeat timestamp
func (r *BackgroundTaskRepository) UpdateHeartbeat(ctx context.Context, id string) error {
	query := `UPDATE background_tasks SET last_heartbeat = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}
	return nil
}

// SaveCheckpoint saves a checkpoint for pause/resume capability
func (r *BackgroundTaskRepository) SaveCheckpoint(ctx context.Context, id string, checkpoint []byte) error {
	query := `UPDATE background_tasks SET checkpoint = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, checkpoint)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}
	return nil
}

// GetByStatus retrieves tasks by status with pagination
func (r *BackgroundTaskRepository) GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.BackgroundTask, error) {
	query := `
		SELECT id, task_type, task_name, correlation_id, parent_task_id,
			payload, config, priority, status, progress, progress_message, checkpoint,
			max_retries, retry_count, retry_delay_seconds, last_error, error_history,
			worker_id, process_pid, started_at, completed_at, last_heartbeat, deadline,
			required_cpu_cores, required_memory_mb, estimated_duration_seconds, actual_duration_seconds,
			notification_config, user_id, session_id, tags, metadata,
			created_at, updated_at, scheduled_at, deleted_at
		FROM background_tasks
		WHERE status = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// GetPendingTasks retrieves pending tasks ordered by priority and creation time
func (r *BackgroundTaskRepository) GetPendingTasks(ctx context.Context, limit int) ([]*models.BackgroundTask, error) {
	query := `
		SELECT id, task_type, task_name, correlation_id, parent_task_id,
			payload, config, priority, status, progress, progress_message, checkpoint,
			max_retries, retry_count, retry_delay_seconds, last_error, error_history,
			worker_id, process_pid, started_at, completed_at, last_heartbeat, deadline,
			required_cpu_cores, required_memory_mb, estimated_duration_seconds, actual_duration_seconds,
			notification_config, user_id, session_id, tags, metadata,
			created_at, updated_at, scheduled_at, deleted_at
		FROM background_tasks
		WHERE status = 'pending' AND scheduled_at <= NOW() AND deleted_at IS NULL
		ORDER BY
			CASE priority
				WHEN 'critical' THEN 0
				WHEN 'high' THEN 1
				WHEN 'normal' THEN 2
				WHEN 'low' THEN 3
				WHEN 'background' THEN 4
			END,
			created_at ASC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending tasks: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// GetStaleTasks retrieves tasks that haven't sent a heartbeat within the threshold
func (r *BackgroundTaskRepository) GetStaleTasks(ctx context.Context, threshold time.Duration) ([]*models.BackgroundTask, error) {
	query := `
		SELECT id, task_type, task_name, correlation_id, parent_task_id,
			payload, config, priority, status, progress, progress_message, checkpoint,
			max_retries, retry_count, retry_delay_seconds, last_error, error_history,
			worker_id, process_pid, started_at, completed_at, last_heartbeat, deadline,
			required_cpu_cores, required_memory_mb, estimated_duration_seconds, actual_duration_seconds,
			notification_config, user_id, session_id, tags, metadata,
			created_at, updated_at, scheduled_at, deleted_at
		FROM background_tasks
		WHERE status = 'running'
			AND last_heartbeat < NOW() - $1::interval
			AND deleted_at IS NULL
	`

	rows, err := r.pool.Query(ctx, query, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query stale tasks: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// GetByWorkerID retrieves tasks assigned to a specific worker
func (r *BackgroundTaskRepository) GetByWorkerID(ctx context.Context, workerID string) ([]*models.BackgroundTask, error) {
	query := `
		SELECT id, task_type, task_name, correlation_id, parent_task_id,
			payload, config, priority, status, progress, progress_message, checkpoint,
			max_retries, retry_count, retry_delay_seconds, last_error, error_history,
			worker_id, process_pid, started_at, completed_at, last_heartbeat, deadline,
			required_cpu_cores, required_memory_mb, estimated_duration_seconds, actual_duration_seconds,
			notification_config, user_id, session_id, tags, metadata,
			created_at, updated_at, scheduled_at, deleted_at
		FROM background_tasks
		WHERE worker_id = $1 AND status = 'running' AND deleted_at IS NULL
	`

	rows, err := r.pool.Query(ctx, query, workerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks by worker: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// Dequeue atomically dequeues a task from the queue
func (r *BackgroundTaskRepository) Dequeue(ctx context.Context, workerID string, maxCPUCores, maxMemoryMB int) (*models.BackgroundTask, error) {
	query := `SELECT * FROM dequeue_background_task($1, $2, $3)`

	var taskID *string
	var taskType, taskName *string
	var payload []byte
	var configJSON []byte
	var priority *models.TaskPriority

	err := r.pool.QueryRow(ctx, query, workerID, maxCPUCores, maxMemoryMB).Scan(
		&taskID, &taskType, &taskName, &payload, &configJSON, &priority,
	)

	if err == pgx.ErrNoRows || taskID == nil {
		return nil, nil // No task available
	}
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	// Fetch the full task
	return r.GetByID(ctx, *taskID)
}

// Delete soft-deletes a task
func (r *BackgroundTaskRepository) Delete(ctx context.Context, id string) error {
	query := `UPDATE background_tasks SET deleted_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	return nil
}

// HardDelete permanently deletes a task
func (r *BackgroundTaskRepository) HardDelete(ctx context.Context, id string) error {
	query := `DELETE FROM background_tasks WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to hard delete task: %w", err)
	}
	return nil
}

// CountByStatus returns count of tasks by status
func (r *BackgroundTaskRepository) CountByStatus(ctx context.Context) (map[models.TaskStatus]int64, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM background_tasks
		WHERE deleted_at IS NULL
		GROUP BY status
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count tasks: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.TaskStatus]int64)
	for rows.Next() {
		var status models.TaskStatus
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
		counts[status] = count
	}

	return counts, nil
}

// SaveResourceSnapshot saves a resource snapshot for a task
func (r *BackgroundTaskRepository) SaveResourceSnapshot(ctx context.Context, snapshot *models.ResourceSnapshot) error {
	query := `
		INSERT INTO task_resource_snapshots (
			task_id, cpu_percent, cpu_user_time, cpu_system_time,
			memory_rss_bytes, memory_vms_bytes, memory_percent,
			io_read_bytes, io_write_bytes, io_read_count, io_write_count,
			net_bytes_sent, net_bytes_recv, net_connections,
			open_files, open_fds, process_state, thread_count
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id, sampled_at
	`

	err := r.pool.QueryRow(ctx, query,
		snapshot.TaskID, snapshot.CPUPercent, snapshot.CPUUserTime, snapshot.CPUSystemTime,
		snapshot.MemoryRSSBytes, snapshot.MemoryVMSBytes, snapshot.MemoryPercent,
		snapshot.IOReadBytes, snapshot.IOWriteBytes, snapshot.IOReadCount, snapshot.IOWriteCount,
		snapshot.NetBytesSent, snapshot.NetBytesRecv, snapshot.NetConnections,
		snapshot.OpenFiles, snapshot.OpenFDs, snapshot.ProcessState, snapshot.ThreadCount,
	).Scan(&snapshot.ID, &snapshot.SampledAt)

	if err != nil {
		return fmt.Errorf("failed to save resource snapshot: %w", err)
	}

	return nil
}

// GetResourceSnapshots retrieves resource snapshots for a task
func (r *BackgroundTaskRepository) GetResourceSnapshots(ctx context.Context, taskID string, limit int) ([]*models.ResourceSnapshot, error) {
	query := `
		SELECT id, task_id, cpu_percent, cpu_user_time, cpu_system_time,
			memory_rss_bytes, memory_vms_bytes, memory_percent,
			io_read_bytes, io_write_bytes, io_read_count, io_write_count,
			net_bytes_sent, net_bytes_recv, net_connections,
			open_files, open_fds, process_state, thread_count, sampled_at
		FROM task_resource_snapshots
		WHERE task_id = $1
		ORDER BY sampled_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, taskID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query resource snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*models.ResourceSnapshot
	for rows.Next() {
		snapshot := &models.ResourceSnapshot{}
		err := rows.Scan(
			&snapshot.ID, &snapshot.TaskID, &snapshot.CPUPercent, &snapshot.CPUUserTime, &snapshot.CPUSystemTime,
			&snapshot.MemoryRSSBytes, &snapshot.MemoryVMSBytes, &snapshot.MemoryPercent,
			&snapshot.IOReadBytes, &snapshot.IOWriteBytes, &snapshot.IOReadCount, &snapshot.IOWriteCount,
			&snapshot.NetBytesSent, &snapshot.NetBytesRecv, &snapshot.NetConnections,
			&snapshot.OpenFiles, &snapshot.OpenFDs, &snapshot.ProcessState, &snapshot.ThreadCount, &snapshot.SampledAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// LogEvent logs a task execution event
func (r *BackgroundTaskRepository) LogEvent(ctx context.Context, taskID, eventType string, data map[string]interface{}, workerID *string) error {
	query := `
		INSERT INTO task_execution_history (task_id, event_type, event_data, worker_id)
		VALUES ($1, $2, $3, $4)
	`

	eventDataJSON, _ := json.Marshal(data)

	_, err := r.pool.Exec(ctx, query, taskID, eventType, eventDataJSON, workerID)
	if err != nil {
		return fmt.Errorf("failed to log event: %w", err)
	}

	return nil
}

// GetTaskHistory retrieves execution history for a task
func (r *BackgroundTaskRepository) GetTaskHistory(ctx context.Context, taskID string, limit int) ([]*models.TaskExecutionHistory, error) {
	query := `
		SELECT id, task_id, event_type, event_data, worker_id, created_at
		FROM task_execution_history
		WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, taskID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query task history: %w", err)
	}
	defer rows.Close()

	var history []*models.TaskExecutionHistory
	for rows.Next() {
		h := &models.TaskExecutionHistory{}
		if err := rows.Scan(&h.ID, &h.TaskID, &h.EventType, &h.EventData, &h.WorkerID, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}
		history = append(history, h)
	}

	return history, nil
}

// MoveToDeadLetter moves a failed task to the dead-letter queue
func (r *BackgroundTaskRepository) MoveToDeadLetter(ctx context.Context, taskID, reason string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get the task data
	task, err := r.GetByID(ctx, taskID)
	if err != nil {
		return err
	}

	// Serialize task data
	taskDataJSON, _ := json.Marshal(task)

	// Insert into dead letter queue
	query := `
		INSERT INTO background_tasks_dead_letter (original_task_id, task_data, failure_reason)
		VALUES ($1, $2, $3)
	`
	_, err = tx.Exec(ctx, query, taskID, taskDataJSON, reason)
	if err != nil {
		return fmt.Errorf("failed to insert into dead letter: %w", err)
	}

	// Update task status
	_, err = tx.Exec(ctx, `UPDATE background_tasks SET status = 'dead_letter' WHERE id = $1`, taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// scanTasks scans rows into BackgroundTask slice
func (r *BackgroundTaskRepository) scanTasks(rows pgx.Rows) ([]*models.BackgroundTask, error) {
	var tasks []*models.BackgroundTask

	for rows.Next() {
		task := &models.BackgroundTask{}
		var configJSON, notificationConfigJSON []byte

		err := rows.Scan(
			&task.ID, &task.TaskType, &task.TaskName, &task.CorrelationID, &task.ParentTaskID,
			&task.Payload, &configJSON, &task.Priority, &task.Status, &task.Progress, &task.ProgressMessage, &task.Checkpoint,
			&task.MaxRetries, &task.RetryCount, &task.RetryDelaySeconds, &task.LastError, &task.ErrorHistory,
			&task.WorkerID, &task.ProcessPID, &task.StartedAt, &task.CompletedAt, &task.LastHeartbeat, &task.Deadline,
			&task.RequiredCPUCores, &task.RequiredMemoryMB, &task.EstimatedDurationSeconds, &task.ActualDurationSeconds,
			&notificationConfigJSON, &task.UserID, &task.SessionID, &task.Tags, &task.Metadata,
			&task.CreatedAt, &task.UpdatedAt, &task.ScheduledAt, &task.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		if err := json.Unmarshal(configJSON, &task.Config); err != nil {
			r.log.WithError(err).Warn("Failed to unmarshal task config")
		}
		if err := json.Unmarshal(notificationConfigJSON, &task.NotificationConfig); err != nil {
			r.log.WithError(err).Warn("Failed to unmarshal notification config")
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}
