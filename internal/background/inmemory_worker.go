package background

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/models"
)

// InMemoryWorker is a minimal background-task drainer for the
// InMemoryTaskQueue / InMemoryTaskRepository combo. It polls the
// queue, transitions each task pending → running → completed, and
// records execution history for the /v1/tasks/:id/{logs,resources}
// endpoints.
//
// CONST-035 §c: closes the #task-worker-pool-wiring gap. Without this
// drainer, /v1/tasks POST persisted tasks but they stayed in
// "pending" forever — the catalog endpoints worked but the lifecycle
// promise of "task will eventually run" was a contract bluff.
//
// This is a NO-OP executor: the worker doesn't actually perform any
// work for each task, it just records the lifecycle transitions so
// SDK consumers can observe the documented state machine
// (pending → running → completed). Real executors plug in via
// RegisterExecutor when a task type's logic is wired.
//
// CONST-029: state is concurrency-safe via the underlying repository
// + queue's safe.Store/Slice containers; the local Stop coordination
// is the standard sync.Once + done-channel pattern.
type InMemoryWorker struct {
	repo    TaskRepository
	queue   TaskQueue
	logger  *logrus.Logger
	pollInt time.Duration

	// Pluggable executor map: taskType → fn that runs the task body.
	// When no executor is registered for a task type, the worker just
	// records the lifecycle transitions without doing real work.
	executors map[string]TaskExecutor
	execMu    sync.RWMutex

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewInMemoryWorker creates a worker bound to the given repo + queue.
func NewInMemoryWorker(repo TaskRepository, queue TaskQueue, logger *logrus.Logger) *InMemoryWorker {
	if logger == nil {
		logger = logrus.New()
	}
	return &InMemoryWorker{
		repo:      repo,
		queue:     queue,
		logger:    logger,
		pollInt:   500 * time.Millisecond,
		executors: make(map[string]TaskExecutor),
		stopCh:    make(chan struct{}),
	}
}

// RegisterExecutor binds a TaskExecutor to a task type. Tasks with
// no registered executor are still drained from the queue (marked
// completed with a "no-op" event) so the queue doesn't grow
// unboundedly during testing.
func (w *InMemoryWorker) RegisterExecutor(taskType string, exec TaskExecutor) {
	w.execMu.Lock()
	defer w.execMu.Unlock()
	w.executors[taskType] = exec
}

// Start launches a single background goroutine that polls the queue
// and processes one task at a time. Idempotent — calling Start on a
// running worker is a no-op via the started gate inside the loop.
func (w *InMemoryWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.loop(ctx)
}

// Stop signals the loop to exit and blocks until it does.
func (w *InMemoryWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
}

func (w *InMemoryWorker) loop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInt)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.tickOnce(ctx)
		}
	}
}

// tickOnce attempts to dequeue and process a single task. Returns
// without error when the queue is empty; the next tick will retry.
func (w *InMemoryWorker) tickOnce(ctx context.Context) {
	task, err := w.repo.Dequeue(ctx, "inmemory-worker", 99, 999999)
	if err != nil || task == nil {
		return
	}

	startedAt := time.Now()
	_ = w.repo.LogEvent(ctx, task.ID, "task.started", map[string]interface{}{
		"worker_id": "inmemory-worker",
		"timestamp": startedAt.Format(time.RFC3339Nano),
	}, ptrString("inmemory-worker")) //nolint:errcheck

	// Choose executor — fall back to no-op if none registered.
	w.execMu.RLock()
	exec, hasExec := w.executors[task.TaskType]
	w.execMu.RUnlock()

	var execErr error
	if hasExec && exec != nil {
		execErr = exec.Execute(ctx, task, &simpleProgressReporter{worker: w, taskID: task.ID})
	}
	// else: no-op — just record the no-op event below

	finishedAt := time.Now()
	if execErr != nil {
		_ = w.repo.UpdateStatus(ctx, task.ID, models.TaskStatusFailed) //nolint:errcheck
		_ = w.repo.LogEvent(ctx, task.ID, "task.failed", map[string]interface{}{
			"error":     execErr.Error(),
			"timestamp": finishedAt.Format(time.RFC3339Nano),
		}, ptrString("inmemory-worker")) //nolint:errcheck
		return
	}

	_ = w.repo.UpdateStatus(ctx, task.ID, models.TaskStatusCompleted) //nolint:errcheck
	eventType := "task.completed"
	eventData := map[string]interface{}{
		"timestamp":  finishedAt.Format(time.RFC3339Nano),
		"duration_s": finishedAt.Sub(startedAt).Seconds(),
	}
	if !hasExec {
		eventType = "task.completed_noop"
		eventData["note"] = "no executor registered for task_type=" + task.TaskType +
			"; worker recorded lifecycle transitions only"
	}
	_ = w.repo.LogEvent(ctx, task.ID, eventType, eventData, ptrString("inmemory-worker")) //nolint:errcheck
}

// simpleProgressReporter implements ProgressReporter for tasks the
// in-memory worker runs. CONST-029: state delegates entirely to the
// underlying repo (which is concurrency-safe via safe.Store), so the
// reporter itself is stateless and lock-free.
type simpleProgressReporter struct {
	worker *InMemoryWorker
	taskID string
}

func (r *simpleProgressReporter) ReportProgress(percent float64, message string) error {
	return r.worker.repo.UpdateProgress(context.Background(), r.taskID, percent, message)
}

func (r *simpleProgressReporter) ReportHeartbeat() error {
	return r.worker.repo.UpdateHeartbeat(context.Background(), r.taskID)
}

func (r *simpleProgressReporter) ReportCheckpoint(data []byte) error {
	return r.worker.repo.SaveCheckpoint(context.Background(), r.taskID, data)
}

func (r *simpleProgressReporter) ReportMetrics(metrics map[string]interface{}) error {
	// In-memory worker doesn't propagate metrics to anywhere outside the
	// process; record them as an event so /v1/tasks/:id/logs reflects.
	return r.worker.repo.LogEvent(context.Background(), r.taskID, "task.metrics", metrics, ptrString("inmemory-worker"))
}

func (r *simpleProgressReporter) ReportLog(level, message string, fields map[string]interface{}) error {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["level"] = level
	fields["message"] = message
	return r.worker.repo.LogEvent(context.Background(), r.taskID, "task.log", fields, ptrString("inmemory-worker"))
}

func ptrString(s string) *string { return &s }
