// Package background provides background task processing for HelixAgent.
package background

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"dev.helix.agent/internal/clis"
	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
)

// DefaultMaxPendingResults caps the number of concurrent in-flight SubmitAsync
// calls a WorkerPool will accept. SubmitAsync creates a per-task delivery
// channel stored in pendingResults; while each entry is removed by defer, a
// pathological caller could accumulate entries faster than workers drain them.
// The cap makes the map growth observable and bounded. 10_000 was chosen as a
// generous headroom over the task queue capacity (size*10 with typical
// size=64 → 640 queue slots) while still fitting comfortably in memory
// (~<1 MB of channels) and preventing runaway growth from stuck callers.
const DefaultMaxPendingResults = 10_000

// SubmitAsyncTimeout bounds how long SubmitAsync will wait for a worker to
// deliver a result. Exposed as a var so tests can tune it.
var SubmitAsyncTimeout = 30 * time.Second

// WorkerPool manages background agent workers.
type WorkerPool struct {
	db     *sql.DB
	logger *log.Logger

	// Pool configuration
	size              int
	queueSize         int
	maxPendingResults atomic.Int64 // hard cap on pendingResults size; 0 = use default

	// Task queue
	taskQueue chan *clis.Task

	// Result queue
	resultQueue chan *TaskResult

	// Per-task result channels for SubmitAsync callers.
	// Size is tracked explicitly via pendingCount so consumers (monitoring,
	// health checks, admission control) can observe growth without walking
	// the map. Every Store must be paired with exactly one Delete.
	pendingResults sync.Map // taskID -> chan *TaskResult
	pendingCount   int64    // atomic; current number of entries in pendingResults

	// Workers — mutated only inside Start() / Stop() serialised by startStopMu.
	workers     *safe.Slice[*Worker]
	startStopMu sync.Mutex // serialises Start/Stop state transitions

	// Instance assignment — concurrent-safe map for taskID -> instanceID.
	instanceAssignments *safe.Store[string, string]

	// Metrics
	tasksSubmitted uint64
	tasksCompleted uint64
	tasksFailed    uint64
	tasksCancelled uint64
	tasksRejected  uint64 // incremented when SubmitAsync trips the pending cap

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Running flag (atomic)
	running atomic.Bool
}

// Worker represents a single worker.
type Worker struct {
	id   int
	pool *WorkerPool
	quit chan struct{}
}

// TaskResult represents the result of a task execution.
type TaskResult struct {
	TaskID   string
	Success  bool
	Result   interface{}
	Error    error
	Duration time.Duration
	WorkerID int
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(size int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		size:                size,
		queueSize:           size * 10,
		taskQueue:           make(chan *clis.Task, size*10),
		resultQueue:         make(chan *TaskResult, size*10),
		workers:             safe.NewSlice[*Worker](),
		instanceAssignments: safe.NewStore[string, string](),
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// NewWorkerPoolWithDB creates a pool with database persistence.
func NewWorkerPoolWithDB(db *sql.DB, logger *log.Logger, size int) *WorkerPool {
	pool := NewWorkerPool(size)
	pool.db = db
	pool.logger = logger
	return pool
}

// Start initializes and starts all workers.
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.startStopMu.Lock()
	defer wp.startStopMu.Unlock()

	if !wp.running.CompareAndSwap(false, true) {
		return fmt.Errorf("worker pool already running")
	}

	// Create workers
	for i := 0; i < wp.size; i++ {
		worker := &Worker{
			id:   i,
			pool: wp,
			quit: make(chan struct{}),
		}
		wp.workers.Append(worker)

		wp.wg.Add(1)
		go worker.run()
	}

	// Start result collector
	wp.wg.Add(1)
	go wp.collectResults()

	// Start maintenance loop
	wp.wg.Add(1)
	go wp.maintenanceLoop()

	if wp.logger != nil {
		wp.logger.Printf("Worker pool started with %d workers", wp.size)
	}

	return nil
}

// Submit submits a task to the pool.
func (wp *WorkerPool) Submit(ctx context.Context, task *clis.Task) error {
	if !wp.isRunning() {
		return fmt.Errorf("worker pool not running")
	}

	// Set defaults
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	if task.Status == "" {
		task.Status = clis.TaskStatusPending
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	// Persist to database if available
	if wp.db != nil {
		if err := wp.persistTask(ctx, task); err != nil {
			return fmt.Errorf("persist task: %w", err)
		}
	}

	// Submit to queue
	select {
	case wp.taskQueue <- task:
		atomic.AddUint64(&wp.tasksSubmitted, 1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("task queue full")
	}
}

// PendingCount returns the current number of in-flight SubmitAsync tasks.
// Safe to call concurrently; intended for monitoring / health checks.
func (wp *WorkerPool) PendingCount() int64 {
	return atomic.LoadInt64(&wp.pendingCount)
}

// maxPending returns the configured cap, falling back to the package default.
func (wp *WorkerPool) maxPending() int64 {
	if v := wp.maxPendingResults.Load(); v > 0 {
		return v
	}
	return DefaultMaxPendingResults
}

// SetMaxPendingResults overrides the pending-results cap. Must be called
// before Start or while no SubmitAsync calls are in flight. Zero restores
// the default.
func (wp *WorkerPool) SetMaxPendingResults(n int64) {
	wp.maxPendingResults.Store(n)
}

// storePending registers a per-task delivery channel under admission control.
// Returns false if the pending cap has been reached — caller must not proceed.
// Every successful storePending must be paired with exactly one deletePending.
func (wp *WorkerPool) storePending(taskID string, ch chan *TaskResult) bool {
	// Reserve a slot via compare-and-increment so the cap is honoured even
	// under concurrent admission. We inc first and roll back on overflow;
	// this keeps the fast path to a single atomic op.
	n := atomic.AddInt64(&wp.pendingCount, 1)
	if n > wp.maxPending() {
		atomic.AddInt64(&wp.pendingCount, -1)
		return false
	}
	wp.pendingResults.Store(taskID, ch)
	return true
}

// deletePending removes a per-task delivery channel and decrements the counter.
// Idempotent: safe to call even if the entry was already removed.
func (wp *WorkerPool) deletePending(taskID string) {
	if _, loaded := wp.pendingResults.LoadAndDelete(taskID); loaded {
		atomic.AddInt64(&wp.pendingCount, -1)
	}
}

// SubmitAsync submits a task asynchronously and returns a channel that
// will receive exactly one TaskResult when the task completes.
// The result is delivered directly via a per-task channel — no polling
// of resultQueue, so no result loss or contention.
//
// Admission control: if the pool already has DefaultMaxPendingResults (or the
// configured cap) in-flight SubmitAsync calls, this returns a channel that
// immediately delivers a rejection error — the task is NOT queued and workers
// are not touched. The rejection is tracked via tasksRejected for monitoring.
func (wp *WorkerPool) SubmitAsync(task *clis.Task) <-chan *TaskResult {
	resultCh := make(chan *TaskResult, 1)

	// Set task ID early so we can register the pending channel and report
	// a stable ID even on rejection.
	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	// Register an internal per-task delivery channel before spawning the
	// waiter goroutine. Workers will find this and send the result here
	// directly, bypassing the shared resultQueue entirely.
	deliveryCh := make(chan *TaskResult, 1)
	if !wp.storePending(task.ID, deliveryCh) {
		atomic.AddUint64(&wp.tasksRejected, 1)
		resultCh <- &TaskResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Errorf("worker pool rejected task: pending cap reached (%d)", wp.maxPending()),
		}
		close(resultCh)
		return resultCh
	}

	go func() {
		defer close(resultCh)
		defer wp.deletePending(task.ID)

		if err := wp.Submit(wp.ctx, task); err != nil {
			resultCh <- &TaskResult{
				TaskID:  task.ID,
				Success: false,
				Error:   err,
			}
			return
		}

		// Use an explicit timer so we can Stop() it on the hot paths and
		// avoid the time.After leak pattern (timers pinned in the runtime
		// heap until they fire, even after the select has resolved).
		timer := time.NewTimer(SubmitAsyncTimeout)
		defer timer.Stop()

		resultCh <- wp.waitForAsyncResult(task.ID, deliveryCh, timer)
	}()

	return resultCh
}

// waitForAsyncResult waits for a worker to deliver a result via deliveryCh,
// or for the pool context to be cancelled (shutdown), or for the
// SubmitAsyncTimeout timer to fire — whichever happens first — and returns
// the TaskResult to report back to the SubmitAsync caller.
//
// Extracted out of SubmitAsync's goroutine (rather than left inline) so its
// priority-select behaviour can be exercised and regression-tested directly
// and deterministically; see
// TestWorkerPool_WaitForAsyncResult_DeliveredResultAlwaysWinsOverShutdown.
//
// Once wp.ctx is cancelled (pool Stop()), wp.ctx.Done() stays PERMANENTLY
// ready for the rest of the process's lifetime. Go's select statement
// chooses UNIFORMLY AT RANDOM among all cases ready at the instant it is
// evaluated — so if a genuine, already-computed successful result is
// sitting buffered in deliveryCh (capacity 1) at the same moment wp.ctx is
// already cancelled, a naked/unprioritized select has real (measured
// ~50%) odds of discarding the already-delivered result in favour of a
// fabricated "worker pool stopped" error, for the entire remaining
// lifetime of every task still in flight at shutdown.
//
// The non-blocking pre-check on deliveryCh below, and the re-check inside
// the wp.ctx.Done() branch, give priority to an already-buffered genuine
// result over the fabricated shutdown error — the mirror image of the
// priority-select fix in lazy_provider.go / debate_log_repository.go
// (those give priority to ctx cancellation over a stale/late result; here
// a real, already-delivered result must win over a fabricated error).
//
// HONESTY (§11.4.107 / no-bluff): this does NOT claim the race is fully
// closed in the general case where a worker's deliverResult() call writes
// into deliveryCh WHILE this select is already parked (rather than before
// it is ever entered) — under Go's async goroutine preemption (>=1.14) no
// select-based implementation can make that window provably zero-width.
// What this genuinely, provably guarantees: a result already buffered in
// deliveryCh at the moment a decision point below is evaluated always wins
// over a fabricated "worker pool stopped" error.
func (wp *WorkerPool) waitForAsyncResult(taskID string, deliveryCh chan *TaskResult, timer *time.Timer) *TaskResult {
	// Priority pre-check: an already-delivered, buffered result must never
	// lose to a fabricated shutdown error just because wp.ctx also happens
	// to be (permanently, post-shutdown) ready. A non-blocking check on
	// deliveryCh first ensures an already-buffered result always wins.
	select {
	case result := <-deliveryCh:
		return result
	default:
	}

	select {
	case result := <-deliveryCh:
		return result
	case <-wp.ctx.Done():
		// Second, immediate re-check: the blocking select above is itself a
		// scheduling/yield point (it can park waiting for any of the three
		// channels), so a worker's deliverResult() can write into
		// deliveryCh WHILE it is parked, letting the random pick land on
		// wp.ctx.Done() anyway even though a genuine result now sits
		// buffered. Re-checking deliveryCh here (non-blocking) narrows that
		// window: an already-buffered result at THIS final decision point
		// always wins over the fabricated error. This does not close the
		// window completely (see the honesty note on this method) but it
		// is the strongest guarantee a select-based implementation can
		// offer.
		select {
		case result := <-deliveryCh:
			return result
		default:
		}
		return &TaskResult{
			TaskID:  taskID,
			Success: false,
			Error:   fmt.Errorf("worker pool stopped"),
		}
	case <-timer.C:
		return &TaskResult{
			TaskID:  taskID,
			Success: false,
			Error:   fmt.Errorf("timeout waiting for result after %s", SubmitAsyncTimeout),
		}
	}
}

// GetTask retrieves a task by ID.
func (wp *WorkerPool) GetTask(ctx context.Context, taskID string) (*clis.Task, error) {
	if wp.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var task clis.Task
	var payloadJSON []byte

	err := wp.db.QueryRowContext(ctx,
		`SELECT id, task_type, task_name, payload, priority, status, 
		        progress_percent, retry_count, max_retries, created_at
		 FROM background_tasks WHERE id = $1`,
		taskID,
	).Scan(
		&task.ID, &task.Type, &task.Name, &payloadJSON, &task.Priority,
		&task.Status, &task.ProgressPercent, &task.RetryCount, &task.MaxRetries,
		&task.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	if err != nil {
		return nil, err
	}

	// Parse payload
	if err := json.Unmarshal(payloadJSON, &task.Payload); err != nil {
		return nil, err
	}

	return &task, nil
}

// CancelTask cancels a pending or running task.
func (wp *WorkerPool) CancelTask(ctx context.Context, taskID string) error {
	if wp.db == nil {
		return fmt.Errorf("database not available")
	}

	_, err := wp.db.ExecContext(ctx,
		"UPDATE background_tasks SET status = $1 WHERE id = $2 AND status IN ($3, $4)",
		clis.TaskStatusCancelled, taskID, clis.TaskStatusPending, clis.TaskStatusAssigned,
	)

	return err
}

// ListTasks returns tasks matching the filter.
func (wp *WorkerPool) ListTasks(
	ctx context.Context,
	status clis.TaskStatus,
	limit int,
) ([]*clis.Task, error) {
	if wp.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	query := `SELECT id, task_type, task_name, payload, priority, status, 
	                 progress_percent, retry_count, max_retries, created_at
	          FROM background_tasks`

	var args []interface{}
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, limit)
	}

	rows, err := wp.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*clis.Task
	for rows.Next() {
		var task clis.Task
		var payloadJSON []byte

		err := rows.Scan(
			&task.ID, &task.Type, &task.Name, &payloadJSON, &task.Priority,
			&task.Status, &task.ProgressPercent, &task.RetryCount, &task.MaxRetries,
			&task.CreatedAt,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(payloadJSON, &task.Payload)
		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}

// GetStats returns pool statistics.
func (wp *WorkerPool) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"size":                wp.size,
		"running":             wp.running.Load(),
		"queue_depth":         len(wp.taskQueue),
		"queue_capacity":      cap(wp.taskQueue),
		"tasks_submitted":     atomic.LoadUint64(&wp.tasksSubmitted),
		"tasks_completed":     atomic.LoadUint64(&wp.tasksCompleted),
		"tasks_failed":        atomic.LoadUint64(&wp.tasksFailed),
		"tasks_cancelled":     atomic.LoadUint64(&wp.tasksCancelled),
		"tasks_rejected":      atomic.LoadUint64(&wp.tasksRejected),
		"pending_results":     atomic.LoadInt64(&wp.pendingCount),
		"pending_results_cap": wp.maxPending(),
	}
}

// Stop shuts down the worker pool gracefully.
// It signals all goroutines to stop via context cancellation, waits for
// them to finish, and only then closes channels — ensuring no goroutine
// writes to a closed channel.
func (wp *WorkerPool) Stop() error {
	wp.startStopMu.Lock()
	defer wp.startStopMu.Unlock()

	if !wp.running.CompareAndSwap(true, false) {
		return nil
	}

	// Step 1: Signal all goroutines to stop via context cancellation.
	// Workers, collectResults, and maintenanceLoop all select on ctx.Done().
	wp.cancel()

	// Step 2: Signal workers via quit channels as a secondary signal.
	for _, worker := range wp.workers.Snapshot() {
		select {
		case <-worker.quit:
			// Already closed
		default:
			close(worker.quit)
		}
	}

	// Step 3: Wait for ALL goroutines (workers, collectResults,
	// maintenanceLoop) to exit. At this point no goroutine is reading
	// from or writing to taskQueue/resultQueue.
	wp.wg.Wait()

	// Step 4: Now safe to close channels — no writers remain.
	close(wp.taskQueue)
	close(wp.resultQueue)

	if wp.logger != nil {
		wp.logger.Printf("Worker pool stopped")
	}

	return nil
}

// Internal methods

func (wp *WorkerPool) isRunning() bool {
	return wp.running.Load()
}

func (wp *WorkerPool) persistTask(ctx context.Context, task *clis.Task) error {
	payloadJSON, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}

	_, err = wp.db.ExecContext(ctx,
		`INSERT INTO background_tasks (
			id, task_type, task_name, payload, priority, status,
			retry_count, max_retries, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			retry_count = EXCLUDED.retry_count,
			progress_percent = EXCLUDED.progress_percent`,
		task.ID, task.Type, task.Name, payloadJSON, task.Priority, task.Status,
		task.RetryCount, task.MaxRetries, task.CreatedAt,
		task.CreatedAt.Add(24*time.Hour), // Default 24h expiration
	)

	return err
}

func (wp *WorkerPool) updateTaskStatus(
	ctx context.Context,
	taskID string,
	status clis.TaskStatus,
	progress int,
	result interface{},
	err error,
) error {
	if wp.db == nil {
		return nil
	}

	resultJSON, _ := json.Marshal(result)
	var errorMsg *string
	if err != nil {
		msg := err.Error()
		errorMsg = &msg
	}

	var startedAt, completedAt interface{}
	if status == clis.TaskStatusRunning {
		startedAt = time.Now()
	}
	if status == clis.TaskStatusCompleted || status == clis.TaskStatusFailed {
		completedAt = time.Now()
	}

	_, dbErr := wp.db.ExecContext(ctx,
		`UPDATE background_tasks SET
			status = $1,
			progress_percent = $2,
			result = $3,
			error_message = $4,
			started_at = COALESCE($5, started_at),
			completed_at = $6
		 WHERE id = $7`,
		status, progress, resultJSON, errorMsg, startedAt, completedAt, taskID,
	)

	return dbErr
}

func (w *Worker) run() {
	defer w.pool.wg.Done()

	for {
		select {
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				return // Channel closed
			}

			// Check context before executing
			select {
			case <-w.pool.ctx.Done():
				return
			default:
			}

			// Check if task was cancelled
			if task.Status == clis.TaskStatusCancelled {
				continue
			}

			// Execute task
			result := w.execute(task)

			// Deliver result: prefer per-task channel (SubmitAsync),
			// fall back to shared resultQueue.
			w.pool.deliverResult(result)

			// Update metrics
			if result.Success {
				atomic.AddUint64(&w.pool.tasksCompleted, 1)
			} else {
				atomic.AddUint64(&w.pool.tasksFailed, 1)
			}

		case <-w.quit:
			return

		case <-w.pool.ctx.Done():
			return
		}
	}
}

// deliverResult sends a task result to the per-task channel if one was
// registered by SubmitAsync, otherwise to the shared resultQueue.
// It never blocks: if the shared resultQueue is full, the result is
// dropped (logged) rather than blocking the worker during shutdown.
func (wp *WorkerPool) deliverResult(result *TaskResult) {
	// Check for a per-task delivery channel (registered by SubmitAsync)
	if ch, ok := wp.pendingResults.Load(result.TaskID); ok {
		if deliveryCh, ok := ch.(chan *TaskResult); ok {
			select {
			case deliveryCh <- result:
				return
			default:
				// Channel already has a result or was closed; skip
			}
		}
	}

	// Fall back to the shared resultQueue (non-blocking)
	select {
	case wp.resultQueue <- result:
	case <-wp.ctx.Done():
		// Pool is shutting down; discard result
	default:
		// resultQueue full; discard to avoid blocking worker
		if wp.logger != nil {
			wp.logger.Printf(
				"Warning: resultQueue full, discarding result for task %s",
				result.TaskID,
			)
		}
	}
}

func (w *Worker) execute(task *clis.Task) *TaskResult {
	start := time.Now()

	// Update status to running
	if w.pool.db != nil {
		w.pool.updateTaskStatus(
			w.pool.ctx,
			task.ID,
			clis.TaskStatusRunning,
			0,
			nil,
			nil,
		)
	}

	// Route to appropriate handler
	var result interface{}
	var err error

	switch task.Type {
	case clis.TaskTypeGitOperation:
		result, err = w.executeGitOperation(task)
	case clis.TaskTypeCodeAnalysis:
		result, err = w.executeCodeAnalysis(task)
	case clis.TaskTypeDocumentation:
		result, err = w.executeDocumentation(task)
	case clis.TaskTypeTesting:
		result, err = w.executeTesting(task)
	case clis.TaskTypeLinting:
		result, err = w.executeLinting(task)
	case clis.TaskTypeBuild:
		result, err = w.executeBuild(task)
	case clis.TaskTypeCodeReview:
		result, err = w.executeCodeReview(task)
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	duration := time.Since(start)

	// Update final status
	status := clis.TaskStatusCompleted
	if err != nil {
		status = clis.TaskStatusFailed
	}

	if w.pool.db != nil {
		w.pool.updateTaskStatus(
			w.pool.ctx,
			task.ID,
			status,
			100,
			result,
			err,
		)
	}

	return &TaskResult{
		TaskID:   task.ID,
		Success:  err == nil,
		Result:   result,
		Error:    err,
		Duration: duration,
		WorkerID: w.id,
	}
}

// Task execution handlers

func (w *Worker) executeGitOperation(task *clis.Task) (interface{}, error) {
	// Implementation would use Aider git integration
	return map[string]string{"status": "git_operation_executed"}, nil
}

func (w *Worker) executeCodeAnalysis(task *clis.Task) (interface{}, error) {
	// Implementation would use repo map and analysis tools
	return map[string]string{"status": "code_analysis_completed"}, nil
}

func (w *Worker) executeDocumentation(task *clis.Task) (interface{}, error) {
	return map[string]string{"status": "documentation_generated"}, nil
}

func (w *Worker) executeTesting(task *clis.Task) (interface{}, error) {
	return map[string]string{"status": "tests_executed"}, nil
}

func (w *Worker) executeLinting(task *clis.Task) (interface{}, error) {
	return map[string]string{"status": "linting_completed"}, nil
}

func (w *Worker) executeBuild(task *clis.Task) (interface{}, error) {
	return map[string]string{"status": "build_completed"}, nil
}

func (w *Worker) executeCodeReview(task *clis.Task) (interface{}, error) {
	return map[string]string{"status": "code_review_completed"}, nil
}

func (wp *WorkerPool) collectResults() {
	defer wp.wg.Done()

	for {
		select {
		case result, ok := <-wp.resultQueue:
			if !ok {
				return // Channel closed
			}
			// Could trigger callbacks here
			if wp.logger != nil && !result.Success {
				wp.logger.Printf(
					"Task %s failed: %v", result.TaskID, result.Error,
				)
			}
		case <-wp.ctx.Done():
			// Drain any remaining results before exiting
			for {
				select {
				case result, ok := <-wp.resultQueue:
					if !ok {
						return
					}
					if wp.logger != nil && !result.Success {
						wp.logger.Printf(
							"Task %s failed: %v",
							result.TaskID, result.Error,
						)
					}
				default:
					return
				}
			}
		}
	}
}

func (wp *WorkerPool) maintenanceLoop() {
	defer wp.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wp.cleanupExpiredTasks()
			wp.retryFailedTasks()

		case <-wp.ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) cleanupExpiredTasks() {
	if wp.db == nil {
		return
	}

	_, err := wp.db.ExecContext(wp.ctx,
		`UPDATE background_tasks 
		 SET status = $1
		 WHERE status IN ($2, $3) AND expires_at < NOW()`,
		clis.TaskStatusExpired,
		clis.TaskStatusPending, clis.TaskStatusAssigned,
	)

	if err != nil && wp.logger != nil {
		wp.logger.Printf("Error cleaning expired tasks: %v", err)
	}
}

func (wp *WorkerPool) retryFailedTasks() {
	if wp.db == nil {
		return
	}

	rows, err := wp.db.QueryContext(wp.ctx,
		`SELECT id, retry_count, max_retries FROM background_tasks
		 WHERE status = $1 AND retry_count < max_retries`,
		clis.TaskStatusFailed,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var retryCount, maxRetries int
		if err := rows.Scan(&id, &retryCount, &maxRetries); err != nil {
			continue
		}

		// Reset to pending for retry
		wp.db.ExecContext(wp.ctx,
			`UPDATE background_tasks 
			 SET status = $1, retry_count = retry_count + 1
			 WHERE id = $2`,
			clis.TaskStatusPending, id,
		)
	}
}
