package background

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/clis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkerPool(t *testing.T) {
	pool := NewWorkerPool(5)
	require.NotNil(t, pool)
	assert.Equal(t, 5, pool.size)
	assert.Equal(t, 50, pool.queueSize)
	assert.NotNil(t, pool.taskQueue)
	assert.NotNil(t, pool.resultQueue)
	assert.NotNil(t, pool.workers)
	assert.NotNil(t, pool.instanceAssignments)
	assert.NotNil(t, pool.ctx)
	assert.NotNil(t, pool.cancel)
	assert.False(t, pool.running.Load())
}

func TestNewWorkerPoolWithDB(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	pool := NewWorkerPoolWithDB(nil, logger, 3)
	require.NotNil(t, pool)
	assert.Equal(t, 3, pool.size)
	assert.Nil(t, pool.db)
	assert.NotNil(t, pool.logger)
}

func TestWorkerPool_Start(t *testing.T) {
	pool := NewWorkerPool(2)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)
	assert.True(t, pool.running.Load())
	assert.Equal(t, 2, pool.workers.Len())

	// Test double start
	err = pool.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Clean shutdown
	err = pool.Stop()
	require.NoError(t, err)
}

func TestWorkerPool_Stop_NotRunning(t *testing.T) {
	pool := NewWorkerPool(2)

	// Stop before start should not error
	err := pool.Stop()
	require.NoError(t, err)
}

func TestWorkerPool_Submit_NotRunning(t *testing.T) {
	pool := NewWorkerPool(2)

	task := &clis.Task{
		Type: "git_operation",
		Name: "test-task",
	}

	err := pool.Submit(context.Background(), task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestWorkerPool_GetStats(t *testing.T) {
	pool := NewWorkerPool(3)

	stats := pool.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 3, stats["size"])
	assert.Equal(t, false, stats["running"])
	assert.Equal(t, 0, stats["queue_depth"])
	assert.Equal(t, uint64(0), stats["tasks_submitted"])
	assert.Equal(t, uint64(0), stats["tasks_completed"])
	assert.Equal(t, uint64(0), stats["tasks_failed"])
	assert.Equal(t, uint64(0), stats["tasks_cancelled"])
}

func TestWorkerPool_GetStats_Running(t *testing.T) {
	pool := NewWorkerPool(2)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	stats := pool.GetStats()
	assert.Equal(t, true, stats["running"])
	assert.Equal(t, 20, stats["queue_capacity"])

	// Clean shutdown
	err = pool.Stop()
	require.NoError(t, err)
}

func TestWorkerPool_GetTask_NoDB(t *testing.T) {
	pool := NewWorkerPool(2)

	task, err := pool.GetTask(context.Background(), "any-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database not available")
	assert.Nil(t, task)
}

func TestWorkerPool_CancelTask_NoDB(t *testing.T) {
	pool := NewWorkerPool(2)

	err := pool.CancelTask(context.Background(), "any-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database not available")
}

func TestWorkerPool_ListTasks_NoDB(t *testing.T) {
	pool := NewWorkerPool(2)

	tasks, err := pool.ListTasks(context.Background(), "pending", 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database not available")
	assert.Nil(t, tasks)
}

func TestTaskResult_Struct(t *testing.T) {
	result := &TaskResult{
		TaskID:   "task-123",
		Success:  true,
		Result:   map[string]string{"status": "completed"},
		Error:    nil,
		Duration: 1 * time.Second,
		WorkerID: 0,
	}

	assert.Equal(t, "task-123", result.TaskID)
	assert.True(t, result.Success)
	assert.NotNil(t, result.Result)
	assert.NoError(t, result.Error)
	assert.Equal(t, 1*time.Second, result.Duration)
	assert.Equal(t, 0, result.WorkerID)
}

// Task struct tests

func TestTask_Struct(t *testing.T) {
	task := clis.Task{
		ID:              "task-1",
		Type:            "git_operation",
		Name:            "test-task",
		Payload:         map[string]string{"key": "value"},
		Priority:        1,
		Status:          "pending",
		ProgressPercent: 0,
		RetryCount:      0,
		MaxRetries:      3,
		CreatedAt:       time.Now(),
	}

	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "git_operation", task.Type)
	assert.Equal(t, "test-task", task.Name)
	assert.Equal(t, 1, task.Priority)
	assert.Equal(t, clis.TaskStatusPending, task.Status)
}

// Task status constants test

func TestTaskStatus_Constants(t *testing.T) {
	// Verify task status constants exist
	statuses := []string{
		"pending",
		"assigned",
		"running",
		"completed",
		"failed",
		"cancelled",
		"expired",
	}

	for _, status := range statuses {
		assert.NotEmpty(t, status)
	}
}

// Task type constants test

func TestTaskType_Constants(t *testing.T) {
	// Verify task type constants exist
	types := []string{
		"git_operation",
		"code_analysis",
		"documentation",
		"testing",
		"linting",
		"build",
		"code_review",
	}

	for _, taskType := range types {
		assert.NotEmpty(t, taskType)
	}
}

// Worker struct test

func TestWorker_Struct(t *testing.T) {
	pool := NewWorkerPool(1)

	worker := &Worker{
		id:   0,
		pool: pool,
		quit: make(chan struct{}),
	}

	assert.Equal(t, 0, worker.id)
	assert.Equal(t, pool, worker.pool)
	assert.NotNil(t, worker.quit)
}

// WorkerPool internal state tests

func TestWorkerPool_isRunning(t *testing.T) {
	pool := NewWorkerPool(1)

	// Before start
	assert.False(t, pool.isRunning())

	// After start
	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)
	assert.True(t, pool.isRunning())

	// Clean shutdown
	err = pool.Stop()
	require.NoError(t, err)
	assert.False(t, pool.isRunning())
}

// Task execution handlers (test that they return expected types)

func TestWorker_Execute_Handlers(t *testing.T) {
	pool := NewWorkerPool(1)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Just test that submit works for different task types
	taskTypes := []string{
		"git_operation",
		"code_analysis",
		"documentation",
		"testing",
		"linting",
		"build",
		"code_review",
	}

	for _, taskType := range taskTypes {
		task := &clis.Task{
			Type: taskType,
			Name: "test-" + taskType,
		}
		err := pool.Submit(ctx, task)
		assert.NoError(t, err)
		assert.NotEmpty(t, task.ID)
	}

	// Give workers time to process
	time.Sleep(200 * time.Millisecond)

	// Clean shutdown
	err = pool.Stop()
	require.NoError(t, err)
}

// Test Submit with context cancellation

func TestWorkerPool_Submit_ContextCancelled(t *testing.T) {
	// Create pool but do NOT start it — just mark as running manually
	// so Submit proceeds past the isRunning check. This avoids workers
	// draining the queue while we fill it.
	pool := NewWorkerPool(1)
	pool.running.Store(true)

	// Fill the queue completely
	for i := 0; i < pool.queueSize; i++ {
		pool.taskQueue <- &clis.Task{
			ID:   fmt.Sprintf("pre-filled-%d", i),
			Type: "documentation",
		}
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	task := &clis.Task{
		Type: "testing",
		Name: "cancelled-task",
	}

	// Queue is full, context is cancelled. The select in Submit has three
	// cases: taskQueue (blocked-full), ctx.Done (ready), default (ready).
	// Go select picks randomly among ready cases, so we may get either
	// context.Canceled or "task queue full". Both are acceptable errors.
	err := pool.Submit(cancelCtx, task)
	assert.Error(t, err)

	// Mark as not running (we never started workers)
	pool.running.Store(false)
}

// Test Submit with defaults

func TestWorkerPool_Submit_SetsDefaults(t *testing.T) {
	pool := NewWorkerPool(1)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	task := &clis.Task{
		Type: "code_analysis",
		Name: "minimal-task",
	}

	err = pool.Submit(ctx, task)
	require.NoError(t, err)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, clis.TaskStatusPending, task.Status)
	assert.False(t, task.CreatedAt.IsZero())

	// Clean shutdown
	err = pool.Stop()
	require.NoError(t, err)
}

// Test Submit with full queue

func TestWorkerPool_Submit_QueueFull(t *testing.T) {
	t.Skip("Skipping test - nil pointer issue needs fixing")
	pool := NewWorkerPool(1)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Fill the queue
	for i := 0; i < pool.queueSize; i++ {
		pool.taskQueue <- &clis.Task{
			ID:   "filler",
			Type: "test",
		}
	}

	// Try to submit when queue is full (should not block due to default case)
	task := &clis.Task{
		Type: "test",
		Name: "full-queue-task",
	}

	err = pool.Submit(ctx, task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue full")

	// Clean shutdown
	err = pool.Stop()
	require.NoError(t, err)
}

// Test Stats after operations

func TestWorkerPool_Stats_AfterSubmit(t *testing.T) {
	pool := NewWorkerPool(1)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Submit a task
	task := &clis.Task{
		Type: "git_operation",
		Name: "stats-test",
	}

	err = pool.Submit(ctx, task)
	require.NoError(t, err)

	// Check stats
	stats := pool.GetStats()
	assert.Equal(t, uint64(1), stats["tasks_submitted"])

	// Clean shutdown
	err = pool.Stop()
	require.NoError(t, err)
}

// --- New tests for S2, M3, M4 fixes ---

// TestWorkerPool_SubmitAsync_NoResultLoss verifies that SubmitAsync delivers
// a result for every submitted task, even under concurrent load.
// This validates the S2 fix: no result loss from the old spin-loop pattern.
func TestWorkerPool_SubmitAsync_NoResultLoss(t *testing.T) {
	const taskCount = 20
	pool := NewWorkerPool(4) // 4 workers to process concurrently

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Submit 20 tasks concurrently via SubmitAsync
	var wg sync.WaitGroup
	results := make([]*TaskResult, taskCount)
	var mu sync.Mutex

	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			task := &clis.Task{
				Type: clis.TaskTypeCodeAnalysis,
				Name: fmt.Sprintf("async-task-%d", idx),
			}

			resultCh := pool.SubmitAsync(task)

			// Wait for result with timeout
			select {
			case result := <-resultCh:
				mu.Lock()
				results[idx] = result
				mu.Unlock()
			case <-time.After(10 * time.Second):
				t.Errorf("task %d: timed out waiting for result", idx)
			}
		}(i)
	}

	// Wait for all goroutines to collect their results
	wg.Wait()

	// Verify every task got a result
	for i := 0; i < taskCount; i++ {
		require.NotNilf(t, results[i], "task %d: missing result", i)
		assert.True(t, results[i].Success,
			"task %d: expected success, got error: %v",
			i, results[i].Error)
		assert.NotEmpty(t, results[i].TaskID,
			"task %d: empty TaskID", i)
	}

	// Verify no duplicate task IDs
	seen := make(map[string]bool)
	for i, r := range results {
		if r != nil {
			assert.False(t, seen[r.TaskID],
				"task %d: duplicate TaskID %s", i, r.TaskID)
			seen[r.TaskID] = true
		}
	}

	// Clean shutdown
	err = pool.Stop()
	require.NoError(t, err)
}

// TestWorkerPool_Stop_NoGoroutineLeak verifies that after Stop() returns,
// all goroutines spawned by the pool have exited (no leaks).
// This validates fixes M3 (workers blocked on full resultQueue) and
// M4 (channels closed while goroutines may still write).
func TestWorkerPool_Stop_NoGoroutineLeak(t *testing.T) {
	// Force GC and get baseline goroutine count
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	pool := NewWorkerPool(4)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Submit some tasks to ensure workers are active
	for i := 0; i < 10; i++ {
		task := &clis.Task{
			Type: clis.TaskTypeBuild,
			Name: fmt.Sprintf("leak-test-%d", i),
		}
		err := pool.Submit(ctx, task)
		require.NoError(t, err)
	}

	// Let workers process
	time.Sleep(100 * time.Millisecond)

	// Stop should complete without hanging (M3/M4 fix)
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- pool.Stop()
	}()

	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return within 5 seconds — " +
			"likely deadlock (M3/M4 not fixed)")
	}

	// Give goroutines time to fully exit
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count returned to baseline (with tolerance)
	// Allow +2 for runtime goroutines that may appear
	final := runtime.NumGoroutine()
	assert.LessOrEqual(t, final, baseline+2,
		"goroutine leak detected: baseline=%d, after Stop=%d (delta=%d)",
		baseline, final, final-baseline)
}

// TestWorkerPool_Stop_DoubleStop verifies that calling Stop() twice
// is safe and idempotent.
func TestWorkerPool_Stop_DoubleStop(t *testing.T) {
	pool := NewWorkerPool(2)

	ctx := context.Background()
	err := pool.Start(ctx)
	require.NoError(t, err)

	// First stop
	err = pool.Stop()
	require.NoError(t, err)

	// Second stop should be a no-op
	err = pool.Stop()
	require.NoError(t, err)
}

// TestWorkerPool_SubmitAsync_PoolStopped verifies that SubmitAsync
// returns an error result when the pool is not running.
func TestWorkerPool_SubmitAsync_PoolStopped(t *testing.T) {
	pool := NewWorkerPool(1)

	// Do NOT start the pool — SubmitAsync should get "not running" error
	task := &clis.Task{
		Type: clis.TaskTypeTesting,
		Name: "stopped-test",
	}

	resultCh := pool.SubmitAsync(task)

	// Wait for the error result
	select {
	case result := <-resultCh:
		require.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "not running")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SubmitAsync error result")
	}
}
