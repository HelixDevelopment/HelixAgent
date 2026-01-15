package background

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// testMetrics holds a single shared metrics instance to avoid Prometheus re-registration errors
var (
	testMetrics     *WorkerPoolMetrics
	testMetricsOnce sync.Once
)

func getTestMetrics() *WorkerPoolMetrics {
	testMetricsOnce.Do(func() {
		testMetrics = NewWorkerPoolMetrics()
	})
	return testMetrics
}

func TestNewWorkerPoolMetrics(t *testing.T) {
	metrics := getTestMetrics()

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.WorkersActive)
	assert.NotNil(t, metrics.WorkersTotal)
	assert.NotNil(t, metrics.ScalingEvents)
	assert.NotNil(t, metrics.TasksTotal)
	assert.NotNil(t, metrics.TasksInQueue)
	assert.NotNil(t, metrics.TaskDuration)
	assert.NotNil(t, metrics.TaskRetries)
	assert.NotNil(t, metrics.StuckTasks)
	assert.NotNil(t, metrics.DeadLetterTasks)
	assert.NotNil(t, metrics.TaskCPUPercent)
	assert.NotNil(t, metrics.TaskMemoryBytes)
	assert.NotNil(t, metrics.TaskIOReadBytes)
	assert.NotNil(t, metrics.TaskIOWriteBytes)
	assert.NotNil(t, metrics.TaskNetBytesSent)
	assert.NotNil(t, metrics.TaskNetBytesRecv)
	assert.NotNil(t, metrics.NotificationsSent)
	assert.NotNil(t, metrics.NotificationErrors)
	assert.NotNil(t, metrics.NotificationLatency)
	assert.NotNil(t, metrics.QueueDepth)
	assert.NotNil(t, metrics.DequeueLatency)
	assert.NotNil(t, metrics.EnqueueLatency)
}

func TestWorkerPoolMetrics_RecordResourceSnapshot(t *testing.T) {
	metrics := getTestMetrics()

	// Record snapshot - should not panic
	metrics.RecordResourceSnapshot(
		"task-123",
		25.5,   // cpuPercent
		1024,   // memoryBytes
		100,    // ioReadBytes
		50,     // ioWriteBytes
		200,    // netBytesSent
		300,    // netBytesRecv
	)

	// Verify we can set values (Prometheus will track them)
	// Note: We can't easily read back gauge values in tests
}

func TestWorkerPoolMetrics_CleanupTaskMetrics(t *testing.T) {
	metrics := getTestMetrics()

	// First record some metrics
	metrics.RecordResourceSnapshot("task-456", 50.0, 2048, 200, 100, 400, 600)

	// Clean up should not panic
	metrics.CleanupTaskMetrics("task-456")

	// Clean up non-existent task should not panic
	metrics.CleanupTaskMetrics("non-existent")
}

func TestWorkerPoolMetrics_UpdateQueueDepth(t *testing.T) {
	metrics := getTestMetrics()

	depths := map[string]int64{
		"critical": 5,
		"high":     10,
		"normal":   25,
		"low":      8,
	}

	// Should not panic
	metrics.UpdateQueueDepth(depths)

	// Update with empty map
	metrics.UpdateQueueDepth(map[string]int64{})
}

func TestGetGlobalMetrics(t *testing.T) {
	// Set known metrics first to avoid re-registration
	sharedMetrics := getTestMetrics()
	SetGlobalMetrics(sharedMetrics)

	metrics := GetGlobalMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, sharedMetrics, metrics)

	// Calling again should return the same instance
	metrics2 := GetGlobalMetrics()
	assert.Equal(t, metrics, metrics2)
}

func TestSetGlobalMetrics(t *testing.T) {
	sharedMetrics := getTestMetrics()

	SetGlobalMetrics(sharedMetrics)
	got := GetGlobalMetrics()
	assert.Equal(t, sharedMetrics, got)

	// Set a different instance (still using shared to avoid registration)
	SetGlobalMetrics(sharedMetrics)
	got2 := GetGlobalMetrics()
	assert.NotNil(t, got2)
}

func TestWorkerPoolMetrics_WorkerGauges(t *testing.T) {
	metrics := getTestMetrics()

	// Test setting gauge values - should not panic
	metrics.WorkersActive.Set(5)
	metrics.WorkersTotal.Set(10)
}

func TestWorkerPoolMetrics_ScalingEvents(t *testing.T) {
	metrics := getTestMetrics()

	// Test scaling events counter - should not panic
	metrics.ScalingEvents.WithLabelValues("up").Inc()
	metrics.ScalingEvents.WithLabelValues("down").Inc()
	metrics.ScalingEvents.WithLabelValues("up").Add(2)
}

func TestWorkerPoolMetrics_TaskCounters(t *testing.T) {
	metrics := getTestMetrics()

	// Test task counters - should not panic
	metrics.TasksTotal.WithLabelValues("command", "completed").Inc()
	metrics.TasksTotal.WithLabelValues("debate", "failed").Inc()
	metrics.TasksTotal.WithLabelValues("embedding", "cancelled").Inc()

	metrics.TaskRetries.WithLabelValues("command").Inc()
	metrics.TaskRetries.WithLabelValues("debate").Add(3)

	metrics.StuckTasks.Inc()
	metrics.DeadLetterTasks.Inc()
}

func TestWorkerPoolMetrics_TaskQueueGauges(t *testing.T) {
	metrics := getTestMetrics()

	// Test task queue gauges - should not panic
	metrics.TasksInQueue.WithLabelValues("critical").Set(2)
	metrics.TasksInQueue.WithLabelValues("high").Set(5)
	metrics.TasksInQueue.WithLabelValues("normal").Set(10)
	metrics.TasksInQueue.WithLabelValues("low").Set(3)
}

func TestWorkerPoolMetrics_TaskDurationHistogram(t *testing.T) {
	metrics := getTestMetrics()

	// Test duration histogram - should not panic
	metrics.TaskDuration.WithLabelValues("command").Observe(0.5)
	metrics.TaskDuration.WithLabelValues("debate").Observe(30.0)
	metrics.TaskDuration.WithLabelValues("embedding").Observe(2.5)
}

func TestWorkerPoolMetrics_NotificationMetrics(t *testing.T) {
	metrics := getTestMetrics()

	// Test notification counters - should not panic
	metrics.NotificationsSent.WithLabelValues("webhook", "task_completed").Inc()
	metrics.NotificationsSent.WithLabelValues("sse", "task_progress").Inc()
	metrics.NotificationsSent.WithLabelValues("websocket", "task_started").Inc()

	metrics.NotificationErrors.WithLabelValues("webhook").Inc()
	metrics.NotificationErrors.WithLabelValues("sse").Add(2)

	metrics.NotificationLatency.WithLabelValues("webhook").Observe(0.05)
	metrics.NotificationLatency.WithLabelValues("sse").Observe(0.001)
}

func TestWorkerPoolMetrics_QueueLatency(t *testing.T) {
	metrics := getTestMetrics()

	// Test queue latency histograms - should not panic
	metrics.DequeueLatency.Observe(0.0005)
	metrics.DequeueLatency.Observe(0.001)
	metrics.DequeueLatency.Observe(0.01)

	metrics.EnqueueLatency.Observe(0.0002)
	metrics.EnqueueLatency.Observe(0.0008)
	metrics.EnqueueLatency.Observe(0.005)
}
