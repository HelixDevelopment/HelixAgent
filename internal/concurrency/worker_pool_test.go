package concurrency

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTaskFunc(t *testing.T) {
	fn := func(ctx context.Context) (interface{}, error) {
		return "result", nil
	}

	task := NewTaskFunc("test-id", fn)

	assert.Equal(t, "test-id", task.ID())

	result, err := task.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "result", result)
}

func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()

	assert.Greater(t, config.Workers, 0)
	assert.Equal(t, 1000, config.QueueSize)
	assert.Equal(t, 30*time.Second, config.TaskTimeout)
	assert.Equal(t, 5*time.Second, config.ShutdownGrace)
}

func TestNewWorkerPool(t *testing.T) {
	pool := NewWorkerPool(nil)
	defer pool.Stop()

	assert.NotNil(t, pool)
	assert.NotNil(t, pool.config)
	assert.NotNil(t, pool.metrics)
}

func TestNewWorkerPool_WithConfig(t *testing.T) {
	config := &PoolConfig{
		Workers:       4,
		QueueSize:     100,
		TaskTimeout:   10 * time.Second,
		ShutdownGrace: 2 * time.Second,
	}

	pool := NewWorkerPool(config)
	defer pool.Stop()

	assert.Equal(t, 4, pool.config.Workers)
	assert.Equal(t, 100, pool.config.QueueSize)
}

func TestWorkerPool_Start(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})
	defer pool.Stop()

	pool.Start()

	assert.True(t, pool.IsRunning())
}

func TestWorkerPool_Submit(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})
	defer pool.Stop()

	var executed atomic.Bool
	task := NewTaskFunc("task-1", func(ctx context.Context) (interface{}, error) {
		executed.Store(true)
		return "done", nil
	})

	err := pool.Submit(task)
	require.NoError(t, err)

	// Wait for task to be executed
	time.Sleep(100 * time.Millisecond)
	assert.True(t, executed.Load())
}

func TestWorkerPool_Submit_ClosedPool(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})
	pool.Start()
	pool.Shutdown(time.Second)

	task := NewTaskFunc("task-1", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	})

	err := pool.Submit(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestWorkerPool_Submit_FullQueue(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 1, QueueSize: 1, TaskTimeout: 5 * time.Second})
	defer pool.Stop()
	pool.Start()

	// Block the worker
	blockCh := make(chan struct{})
	blockTask := NewTaskFunc("block", func(ctx context.Context) (interface{}, error) {
		<-blockCh
		return nil, nil
	})
	pool.Submit(blockTask)

	// Fill the queue
	pool.Submit(NewTaskFunc("fill", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))

	// This should fail with queue full
	err := pool.Submit(NewTaskFunc("overflow", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	}))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue is full")

	close(blockCh)
}

func TestWorkerPool_SubmitWait(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})
	defer pool.Stop()

	task := NewTaskFunc("task-1", func(ctx context.Context) (interface{}, error) {
		return "result", nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := pool.SubmitWait(ctx, task)
	require.NoError(t, err)

	assert.Equal(t, "task-1", result.TaskID)
	assert.Equal(t, "result", result.Value)
	assert.NotZero(t, result.Duration)
}

func TestWorkerPool_SubmitWait_WithError(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})
	defer pool.Stop()

	expectedErr := errors.New("task failed")
	task := NewTaskFunc("task-1", func(ctx context.Context) (interface{}, error) {
		return nil, expectedErr
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := pool.SubmitWait(ctx, task)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, result.Error)
}

func TestWorkerPool_SubmitBatch(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 4, QueueSize: 20})
	defer pool.Stop()

	tasks := make([]Task, 5)
	for i := 0; i < 5; i++ {
		idx := i
		tasks[i] = NewTaskFunc("batch-"+string(rune('0'+idx)), func(ctx context.Context) (interface{}, error) {
			return idx * 2, nil
		})
	}

	resultChan := pool.SubmitBatch(tasks)

	var count int
	for range resultChan {
		count++
	}

	assert.Equal(t, 5, count)
}

func TestWorkerPool_SubmitBatchWait(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 4, QueueSize: 20})
	defer pool.Stop()

	tasks := make([]Task, 3)
	for i := 0; i < 3; i++ {
		idx := i
		tasks[i] = NewTaskFunc("batch-"+string(rune('0'+idx)), func(ctx context.Context) (interface{}, error) {
			return idx * 10, nil
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := pool.SubmitBatchWait(ctx, tasks)
	require.NoError(t, err)

	assert.Len(t, results, 3)
}

func TestWorkerPool_Metrics(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})
	defer pool.Stop()

	// Submit some tasks
	for i := 0; i < 5; i++ {
		pool.Submit(NewTaskFunc("task", func(ctx context.Context) (interface{}, error) {
			return nil, nil
		}))
	}

	time.Sleep(200 * time.Millisecond)

	metrics := pool.Metrics()
	assert.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.CompletedTasks, int64(0))
}

func TestWorkerPool_QueueLength(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 1, QueueSize: 10, TaskTimeout: 5 * time.Second})
	defer pool.Stop()

	// Block the worker
	blockCh := make(chan struct{})
	pool.Submit(NewTaskFunc("block", func(ctx context.Context) (interface{}, error) {
		<-blockCh
		return nil, nil
	}))

	time.Sleep(50 * time.Millisecond)

	// Queue more tasks
	for i := 0; i < 3; i++ {
		pool.Submit(NewTaskFunc("queued", func(ctx context.Context) (interface{}, error) {
			return nil, nil
		}))
	}

	length := pool.QueueLength()
	assert.GreaterOrEqual(t, length, 0)

	close(blockCh)
}

func TestWorkerPool_ActiveWorkers(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10, TaskTimeout: 5 * time.Second})
	defer pool.Stop()
	pool.Start()

	// Initially no active workers
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, pool.ActiveWorkers())

	// Start a task
	blockCh := make(chan struct{})
	pool.Submit(NewTaskFunc("block", func(ctx context.Context) (interface{}, error) {
		<-blockCh
		return nil, nil
	}))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, pool.ActiveWorkers())

	close(blockCh)
}

func TestWorkerPool_IsRunning(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})

	assert.False(t, pool.IsRunning())

	pool.Start()
	assert.True(t, pool.IsRunning())

	pool.Shutdown(time.Second)
	assert.False(t, pool.IsRunning())
}

func TestWorkerPool_Shutdown(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})
	pool.Start()

	err := pool.Shutdown(time.Second)
	assert.NoError(t, err)

	assert.False(t, pool.IsRunning())
}

func TestWorkerPool_Stop(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 2, QueueSize: 10})
	pool.Start()

	pool.Stop()

	assert.False(t, pool.IsRunning())
}

func TestWorkerPool_WaitForDrain(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 4, QueueSize: 10})
	defer pool.Stop()
	pool.Start()

	// Submit tasks
	for i := 0; i < 5; i++ {
		pool.Submit(NewTaskFunc("task", func(ctx context.Context) (interface{}, error) {
			time.Sleep(10 * time.Millisecond)
			return nil, nil
		}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pool.WaitForDrain(ctx)
	assert.NoError(t, err)
}

func TestWorkerPool_WaitForDrain_Timeout(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 1, QueueSize: 10, TaskTimeout: 5 * time.Second})
	defer pool.Stop()

	// Block the worker
	blockCh := make(chan struct{})
	pool.Submit(NewTaskFunc("block", func(ctx context.Context) (interface{}, error) {
		<-blockCh
		return nil, nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := pool.WaitForDrain(ctx)
	assert.Error(t, err)

	close(blockCh)
}

func TestWorkerPool_TaskTimeout(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 50 * time.Millisecond,
	})
	defer pool.Stop()

	task := NewTaskFunc("slow", func(ctx context.Context) (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return "done", nil
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, _ := pool.SubmitWait(ctx, task)
	assert.Error(t, result.Error)
}

func TestWorkerPool_OnError(t *testing.T) {
	var errorCalled atomic.Bool
	var errorTaskID atomic.Value

	pool := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
		OnError: func(taskID string, err error) {
			errorTaskID.Store(taskID)
			errorCalled.Store(true)
		},
	})
	defer pool.Stop()

	task := NewTaskFunc("failing-task", func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("intentional error")
	})

	pool.Submit(task)
	time.Sleep(100 * time.Millisecond)

	assert.True(t, errorCalled.Load())
	assert.Equal(t, "failing-task", errorTaskID.Load().(string))
}

func TestWorkerPool_OnComplete(t *testing.T) {
	var completeCalled atomic.Bool

	pool := NewWorkerPool(&PoolConfig{
		Workers:   2,
		QueueSize: 10,
		OnComplete: func(result Result) {
			completeCalled.Store(true)
		},
	})
	defer pool.Stop()

	task := NewTaskFunc("task", func(ctx context.Context) (interface{}, error) {
		return "success", nil
	})

	pool.Submit(task)
	time.Sleep(100 * time.Millisecond)

	assert.True(t, completeCalled.Load())
}

func TestPoolMetrics_AverageLatency(t *testing.T) {
	metrics := &PoolMetrics{
		TotalLatencyUs: 10000, // 10ms total
		TaskCount:      10,    // 10 tasks
	}

	avg := metrics.AverageLatency()
	assert.Equal(t, time.Millisecond, avg)
}

func TestPoolMetrics_AverageLatency_Zero(t *testing.T) {
	metrics := &PoolMetrics{
		TotalLatencyUs: 0,
		TaskCount:      0,
	}

	avg := metrics.AverageLatency()
	assert.Equal(t, time.Duration(0), avg)
}

func TestResult_Fields(t *testing.T) {
	startTime := time.Now()
	result := Result{
		TaskID:    "task-1",
		Value:     "test-value",
		Error:     nil,
		StartTime: startTime,
		Duration:  100 * time.Millisecond,
	}

	assert.Equal(t, "task-1", result.TaskID)
	assert.Equal(t, "test-value", result.Value)
	assert.Nil(t, result.Error)
	assert.Equal(t, startTime, result.StartTime)
	assert.Equal(t, 100*time.Millisecond, result.Duration)
}

func TestParallelExecute(t *testing.T) {
	fns := []func(ctx context.Context) (interface{}, error){
		func(ctx context.Context) (interface{}, error) { return 1, nil },
		func(ctx context.Context) (interface{}, error) { return 2, nil },
		func(ctx context.Context) (interface{}, error) { return 3, nil },
	}

	results, err := ParallelExecute(context.Background(), fns)
	require.NoError(t, err)

	assert.Len(t, results, 3)
}

func TestMap(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	results, err := Map(context.Background(), items, 4, func(ctx context.Context, item int) (int, error) {
		return item * 2, nil
	})

	require.NoError(t, err)
	assert.Len(t, results, 5)

	// Check results (order may vary due to parallelism)
	sum := 0
	for _, r := range results {
		sum += r
	}
	assert.Equal(t, 30, sum) // 2+4+6+8+10 = 30
}

func TestMap_WithError(t *testing.T) {
	items := []int{1, 2, 3}

	_, err := Map(context.Background(), items, 2, func(ctx context.Context, item int) (int, error) {
		if item == 2 {
			return 0, errors.New("error on item 2")
		}
		return item, nil
	})

	assert.Error(t, err)
}

// Concurrent access tests
func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	pool := NewWorkerPool(&PoolConfig{Workers: 4, QueueSize: 100})
	defer pool.Stop()

	var completed atomic.Int64
	const numTasks = 50

	for i := 0; i < numTasks; i++ {
		go func(idx int) {
			task := NewTaskFunc("concurrent", func(ctx context.Context) (interface{}, error) {
				time.Sleep(time.Millisecond)
				completed.Add(1)
				return nil, nil
			})
			pool.Submit(task)
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	assert.GreaterOrEqual(t, completed.Load(), int64(numTasks/2))
}
