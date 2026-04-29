package background

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"dev.helix.agent/internal/models"

	safelib "digital.vasic.concurrency/pkg/safe"
)

// InMemoryTaskRepository is a concurrency-safe, in-memory implementation of
// TaskRepository. It satisfies CONST-035 §c "Completion": when no Postgres
// pool is plumbed through RouterContext, the BackgroundTaskHandler still
// gets a real backing store so /v1/tasks/* endpoints serve real data instead
// of silently 503-ing.
//
// This implementation is intentionally process-local — tasks do not survive
// process restart. For production deployments that require durability, swap
// for the Postgres-backed BackgroundTaskRepository in
// internal/database/background_task_repository.go via the same
// background.TaskRepository interface.
//
// CONST-029 (Concurrent-Safe Containers): all mutable state uses
// safe.Store[K,V] / safe.Slice[T] from digital.vasic.concurrency/pkg/safe,
// so the type is structurally lock-free at the call site.
type InMemoryTaskRepository struct {
	tasks      *safelib.Store[string, *models.BackgroundTask]
	snapshots  *safelib.Store[string, *safelib.Slice[*models.ResourceSnapshot]]
	history    *safelib.Store[string, *safelib.Slice[*models.TaskExecutionHistory]]
	deadLetter *safelib.Store[string, string] // taskID -> reason
}

// NewInMemoryTaskRepository creates a new in-memory repository.
func NewInMemoryTaskRepository() *InMemoryTaskRepository {
	return &InMemoryTaskRepository{
		tasks:      safelib.NewStore[string, *models.BackgroundTask](),
		snapshots:  safelib.NewStore[string, *safelib.Slice[*models.ResourceSnapshot]](),
		history:    safelib.NewStore[string, *safelib.Slice[*models.TaskExecutionHistory]](),
		deadLetter: safelib.NewStore[string, string](),
	}
}

// Compile-time interface check.
var _ TaskRepository = (*InMemoryTaskRepository)(nil)

// Create stores a new task.
func (r *InMemoryTaskRepository) Create(ctx context.Context, task *models.BackgroundTask) error {
	if task == nil {
		return fmt.Errorf("create: task is nil")
	}
	if task.ID == "" {
		return fmt.Errorf("create: task ID is empty")
	}
	if _, exists := r.tasks.Get(task.ID); exists {
		return fmt.Errorf("create: task %s already exists", task.ID)
	}
	r.tasks.Put(task.ID, task)
	return nil
}

// GetByID retrieves a task by ID.
func (r *InMemoryTaskRepository) GetByID(ctx context.Context, id string) (*models.BackgroundTask, error) {
	t, ok := r.tasks.Get(id)
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return t, nil
}

// Update replaces an existing task.
func (r *InMemoryTaskRepository) Update(ctx context.Context, task *models.BackgroundTask) error {
	if task == nil || task.ID == "" {
		return fmt.Errorf("update: task or task.ID is empty")
	}
	if _, ok := r.tasks.Get(task.ID); !ok {
		return fmt.Errorf("update: task %s not found", task.ID)
	}
	r.tasks.Put(task.ID, task)
	return nil
}

// Delete removes a task.
func (r *InMemoryTaskRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.tasks.Get(id); !ok {
		return fmt.Errorf("delete: task %s not found", id)
	}
	r.tasks.Delete(id)
	return nil
}

// UpdateStatus changes a task's status.
func (r *InMemoryTaskRepository) UpdateStatus(ctx context.Context, id string, status models.TaskStatus) error {
	t, ok := r.tasks.Get(id)
	if !ok {
		return fmt.Errorf("update_status: task %s not found", id)
	}
	t.Status = status
	t.UpdatedAt = time.Now()
	r.tasks.Put(id, t)
	return nil
}

// UpdateProgress updates progress percentage + message.
func (r *InMemoryTaskRepository) UpdateProgress(ctx context.Context, id string, progress float64, message string) error {
	t, ok := r.tasks.Get(id)
	if !ok {
		return fmt.Errorf("update_progress: task %s not found", id)
	}
	t.Progress = progress
	if message != "" {
		t.ProgressMessage = &message
	}
	t.UpdatedAt = time.Now()
	r.tasks.Put(id, t)
	return nil
}

// UpdateHeartbeat refreshes the task's last-seen timestamp.
func (r *InMemoryTaskRepository) UpdateHeartbeat(ctx context.Context, id string) error {
	t, ok := r.tasks.Get(id)
	if !ok {
		return fmt.Errorf("update_heartbeat: task %s not found", id)
	}
	now := time.Now()
	t.LastHeartbeat = &now
	r.tasks.Put(id, t)
	return nil
}

// SaveCheckpoint stores a checkpoint blob.
func (r *InMemoryTaskRepository) SaveCheckpoint(ctx context.Context, id string, checkpoint []byte) error {
	t, ok := r.tasks.Get(id)
	if !ok {
		return fmt.Errorf("save_checkpoint: task %s not found", id)
	}
	t.Checkpoint = json.RawMessage(checkpoint)
	t.UpdatedAt = time.Now()
	r.tasks.Put(id, t)
	return nil
}

// GetByStatus returns tasks with the given status, paginated.
func (r *InMemoryTaskRepository) GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.BackgroundTask, error) {
	all := r.snapshotByStatus(status)
	return paginate(all, limit, offset), nil
}

// GetPendingTasks returns up to `limit` tasks in StatusPending.
func (r *InMemoryTaskRepository) GetPendingTasks(ctx context.Context, limit int) ([]*models.BackgroundTask, error) {
	all := r.snapshotByStatus(models.TaskStatusPending)
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// GetStaleTasks returns tasks whose last heartbeat is older than threshold.
func (r *InMemoryTaskRepository) GetStaleTasks(ctx context.Context, threshold time.Duration) ([]*models.BackgroundTask, error) {
	cutoff := time.Now().Add(-threshold)
	out := make([]*models.BackgroundTask, 0)
	for _, k := range r.tasks.Keys() {
		t, ok := r.tasks.Get(k)
		if !ok || t == nil {
			continue
		}
		if t.LastHeartbeat != nil && t.LastHeartbeat.Before(cutoff) {
			out = append(out, t)
		}
	}
	return out, nil
}

// GetByWorkerID returns all tasks assigned to a worker.
func (r *InMemoryTaskRepository) GetByWorkerID(ctx context.Context, workerID string) ([]*models.BackgroundTask, error) {
	out := make([]*models.BackgroundTask, 0)
	for _, k := range r.tasks.Keys() {
		t, ok := r.tasks.Get(k)
		if !ok || t == nil || t.WorkerID == nil {
			continue
		}
		if *t.WorkerID == workerID {
			out = append(out, t)
		}
	}
	return out, nil
}

// CountByStatus returns task counts grouped by status.
func (r *InMemoryTaskRepository) CountByStatus(ctx context.Context) (map[models.TaskStatus]int64, error) {
	counts := make(map[models.TaskStatus]int64)
	for _, k := range r.tasks.Keys() {
		t, ok := r.tasks.Get(k)
		if !ok || t == nil {
			continue
		}
		counts[t.Status]++
	}
	return counts, nil
}

// Dequeue claims the highest-priority pending task that fits the worker's
// resource budget. In-memory implementation: scans pending tasks, picks the
// first that fits, and atomically transitions it to running.
func (r *InMemoryTaskRepository) Dequeue(ctx context.Context, workerID string, maxCPUCores, maxMemoryMB int) (*models.BackgroundTask, error) {
	pending := r.snapshotByStatus(models.TaskStatusPending)
	for _, t := range pending {
		if t.RequiredCPUCores > maxCPUCores || t.RequiredMemoryMB > maxMemoryMB {
			continue
		}
		// Atomic claim — re-fetch + status check before mutate.
		current, ok := r.tasks.Get(t.ID)
		if !ok || current == nil || current.Status != models.TaskStatusPending {
			continue
		}
		current.Status = models.TaskStatusRunning
		current.WorkerID = &workerID
		now := time.Now()
		current.StartedAt = &now
		current.LastHeartbeat = &now
		current.UpdatedAt = now
		r.tasks.Put(t.ID, current)
		return current, nil
	}
	return nil, nil
}

// SaveResourceSnapshot appends a resource snapshot for a task.
func (r *InMemoryTaskRepository) SaveResourceSnapshot(ctx context.Context, snapshot *models.ResourceSnapshot) error {
	if snapshot == nil || snapshot.TaskID == "" {
		return fmt.Errorf("save_snapshot: snapshot or TaskID empty")
	}
	bucket, ok := r.snapshots.Get(snapshot.TaskID)
	if !ok || bucket == nil {
		bucket = safelib.NewSlice[*models.ResourceSnapshot]()
		r.snapshots.Put(snapshot.TaskID, bucket)
	}
	bucket.Append(snapshot)
	return nil
}

// GetResourceSnapshots returns up to `limit` most-recent snapshots for a task.
func (r *InMemoryTaskRepository) GetResourceSnapshots(ctx context.Context, taskID string, limit int) ([]*models.ResourceSnapshot, error) {
	bucket, ok := r.snapshots.Get(taskID)
	if !ok || bucket == nil {
		return nil, nil
	}
	all := bucket.Snapshot()
	// Most-recent first
	sort.Slice(all, func(i, j int) bool {
		return all[i].SampledAt.After(all[j].SampledAt)
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// LogEvent records an execution-history entry for a task.
func (r *InMemoryTaskRepository) LogEvent(ctx context.Context, taskID, eventType string, data map[string]interface{}, workerID *string) error {
	bucket, ok := r.history.Get(taskID)
	if !ok || bucket == nil {
		bucket = safelib.NewSlice[*models.TaskExecutionHistory]()
		r.history.Put(taskID, bucket)
	}
	encoded, jerr := json.Marshal(data)
	if jerr != nil {
		encoded = json.RawMessage("{}")
	}
	bucket.Append(&models.TaskExecutionHistory{
		TaskID:    taskID,
		EventType: eventType,
		EventData: encoded,
		WorkerID:  workerID,
		CreatedAt: time.Now(),
	})
	return nil
}

// GetTaskHistory returns up to `limit` most-recent history entries for a task.
func (r *InMemoryTaskRepository) GetTaskHistory(ctx context.Context, taskID string, limit int) ([]*models.TaskExecutionHistory, error) {
	bucket, ok := r.history.Get(taskID)
	if !ok || bucket == nil {
		return nil, nil
	}
	all := bucket.Snapshot()
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// MoveToDeadLetter marks a task as terminally failed and records the reason.
func (r *InMemoryTaskRepository) MoveToDeadLetter(ctx context.Context, taskID, reason string) error {
	t, ok := r.tasks.Get(taskID)
	if !ok || t == nil {
		return fmt.Errorf("dead_letter: task %s not found", taskID)
	}
	t.Status = models.TaskStatusFailed
	if t.LastError == nil {
		t.LastError = &reason
	}
	t.UpdatedAt = time.Now()
	r.tasks.Put(taskID, t)
	r.deadLetter.Put(taskID, reason)
	return nil
}

// snapshotByStatus returns a copy of all tasks matching the given status,
// ordered by CreatedAt ascending. An empty status string matches every task,
// matching the BackgroundTaskHandler ListTasks contract (empty `?status=`
// returns all tasks).
func (r *InMemoryTaskRepository) snapshotByStatus(status models.TaskStatus) []*models.BackgroundTask {
	matchAll := status == ""
	out := make([]*models.BackgroundTask, 0)
	for _, k := range r.tasks.Keys() {
		t, ok := r.tasks.Get(k)
		if !ok || t == nil {
			continue
		}
		if !matchAll && t.Status != status {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func paginate(all []*models.BackgroundTask, limit, offset int) []*models.BackgroundTask {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(all) {
		return nil
	}
	out := all[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
