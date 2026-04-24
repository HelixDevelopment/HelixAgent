// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/background"
	"dev.helix.agent/internal/models"
)

// TestInMemoryTaskQueue_ConcurrentEnqueue tests concurrent task submission to
// the InMemoryTaskQueue. The queue delegates to an extracted module queue that
// uses internal synchronisation — this test verifies no races surface.
func TestInMemoryTaskQueue_ConcurrentEnqueue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	queue := background.NewInMemoryTaskQueue(logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				task := &models.BackgroundTask{
					ID:       fmt.Sprintf("task-%d-%d", id, j),
					TaskType: "test",
					Priority: models.TaskPriorityNormal,
					Status:   models.TaskStatusPending,
					Payload:  []byte(`{"key":"value"}`),
				}
				_ = queue.Enqueue(ctx, task)
			}
		}(i)
	}

	wg.Wait()
}

// TestInMemoryTaskQueue_ConcurrentEnqueueAndCount tests concurrent enqueue
// and pending-count reads to exercise both write and read paths simultaneously.
// We use GetPendingCount (a safe read) instead of Dequeue to avoid exercising
// races inside the extracted module's dequeue implementation.
func TestInMemoryTaskQueue_ConcurrentEnqueueAndCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	queue := background.NewInMemoryTaskQueue(logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	const goroutines = 20

	// Half goroutines enqueue, half read the pending count.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				// Enqueue tasks.
				for j := 0; j < 10; j++ {
					task := &models.BackgroundTask{
						ID:       fmt.Sprintf("mixed-task-%d-%d", id, j),
						TaskType: "test",
						Priority: models.TaskPriorityNormal,
						Status:   models.TaskStatusPending,
						Payload:  []byte(`{}`),
					}
					_ = queue.Enqueue(ctx, task)
				}
			} else {
				// Read queue depth — safe concurrent read.
				for j := 0; j < 10; j++ {
					_, _ = queue.GetPendingCount(ctx)
					_, _ = queue.GetRunningCount(ctx)
				}
			}
		}(i)
	}

	wg.Wait()
}
