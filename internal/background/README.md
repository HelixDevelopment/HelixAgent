# Background Task System

The `background` package provides a robust task execution system for running long-running operations asynchronously with progress tracking, pause/resume capabilities, and resource monitoring.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Task Queue                               │
│  (PostgresTaskQueue / InMemoryTaskQueue)                        │
├─────────────────────────────────────────────────────────────────┤
│                         Worker Pool                              │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐            │
│  │ Worker 1│  │ Worker 2│  │ Worker 3│  │ Worker N│            │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘            │
│       │            │            │            │                   │
│       └────────────┴─────┬──────┴────────────┘                   │
│                          ▼                                       │
│              ┌─────────────────────┐                            │
│              │   Task Executors    │                            │
│              └─────────────────────┘                            │
├─────────────────────────────────────────────────────────────────┤
│  Resource Monitor          │          Stuck Detector            │
│  (CPU, Memory, Disk)       │          (Timeout, Progress)       │
└─────────────────────────────────────────────────────────────────┘
```

## Key Interfaces

### TaskExecutor

Defines how tasks are executed:

```go
type TaskExecutor interface {
    Execute(ctx context.Context, task *models.BackgroundTask, reporter ProgressReporter) error
    CanPause() bool
    Pause(ctx context.Context, task *models.BackgroundTask) ([]byte, error)
    Resume(ctx context.Context, task *models.BackgroundTask, checkpoint []byte) error
    Cancel(ctx context.Context, task *models.BackgroundTask) error
    GetResourceRequirements() ResourceRequirements
}
```

### TaskQueue

Manages task queueing and dequeuing:

```go
type TaskQueue interface {
    Enqueue(ctx context.Context, task *models.BackgroundTask) error
    Dequeue(ctx context.Context, workerID string, requirements ResourceRequirements) (*models.BackgroundTask, error)
    Peek(ctx context.Context, count int) ([]*models.BackgroundTask, error)
    Requeue(ctx context.Context, taskID string, delay time.Duration) error
    MoveToDeadLetter(ctx context.Context, taskID string, reason string) error
    GetPendingCount(ctx context.Context) (int64, error)
    GetRunningCount(ctx context.Context) (int64, error)
    GetQueueDepth(ctx context.Context) (map[models.TaskPriority]int64, error)
}
```

### WorkerPool

Manages background workers:

```go
type WorkerPool interface {
    Start(ctx context.Context) error
    Stop(gracePeriod time.Duration) error
    RegisterExecutor(taskType string, executor TaskExecutor)
    GetWorkerCount() int
    GetActiveTaskCount() int
    GetWorkerStatus() []WorkerStatus
    Scale(targetCount int) error
}
```

### ResourceMonitor

Tracks system resources:

```go
type ResourceMonitor interface {
    GetSystemResources() (*SystemResources, error)
    GetProcessResources(pid int) (*models.ResourceSnapshot, error)
    StartMonitoring(taskID string, pid int, interval time.Duration) error
    StopMonitoring(taskID string) error
    GetLatestSnapshot(taskID string) (*models.ResourceSnapshot, error)
    IsResourceAvailable(requirements ResourceRequirements) bool
}
```

### StuckDetector

Identifies stuck processes:

```go
type StuckDetector interface {
    IsStuck(ctx context.Context, task *models.BackgroundTask, snapshots []*models.ResourceSnapshot) (bool, string)
    GetStuckThreshold(taskType string) time.Duration
    SetThreshold(taskType string, threshold time.Duration)
}
```

## Task States

```
┌─────────┐     ┌────────┐     ┌─────────┐     ┌───────────┐
│ pending │────▶│ queued │────▶│ running │────▶│ completed │
└─────────┘     └────────┘     └────┬────┘     └───────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
               ┌────────┐     ┌─────────┐     ┌───────────┐
               │ failed │     │  stuck  │     │ cancelled │
               └────────┘     └─────────┘     └───────────┘
```

## Files

| File | Description |
|------|-------------|
| `interfaces.go` | Core interface definitions |
| `task_queue.go` | PostgresTaskQueue and InMemoryTaskQueue implementations |
| `worker_pool.go` | Worker pool implementation with auto-scaling |
| `resource_monitor.go` | System resource monitoring |
| `stuck_detector.go` | Stuck task detection logic |
| `metrics.go` | Prometheus metrics for monitoring |

## Usage

### Creating a Worker Pool

```go
pool := background.NewWorkerPool(
    taskQueue,
    taskRepo,
    resourceMonitor,
    logger,
    background.WorkerPoolConfig{
        MinWorkers:      2,
        MaxWorkers:      10,
        ScaleUpThreshold:   0.8,
        ScaleDownThreshold: 0.2,
    },
)

// Register executors for task types
pool.RegisterExecutor("command", commandExecutor)
pool.RegisterExecutor("analysis", analysisExecutor)

// Start the pool
pool.Start(ctx)
defer pool.Stop(30 * time.Second)
```

### Enqueueing Tasks

```go
task := &models.BackgroundTask{
    ID:       uuid.New().String(),
    Type:     "command",
    Priority: models.TaskPriorityHigh,
    Payload:  []byte(`{"command": "go test ./..."}`),
}

err := taskQueue.Enqueue(ctx, task)
```

### Progress Reporting

Task executors can report progress during execution:

```go
func (e *CommandExecutor) Execute(ctx context.Context, task *models.BackgroundTask, reporter ProgressReporter) error {
    reporter.ReportProgress(0, "Starting command execution")

    // Execute command...
    reporter.ReportProgress(50, "Command running")
    reporter.ReportHeartbeat()

    // Save checkpoint for pause/resume
    reporter.ReportCheckpoint(checkpointData)

    reporter.ReportProgress(100, "Command completed")
    return nil
}
```

## Testing

```bash
go test -v ./internal/background/...
```

The package includes comprehensive tests for:
- Task queue operations (enqueue, dequeue, requeue)
- Worker pool lifecycle and scaling
- Resource monitoring
- Stuck detection algorithms
- Concurrent access patterns
