// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// noopCompleteFunc is a CompleteFunc that immediately returns an empty response.
// It avoids real LLM calls while still exercising the worker loop.
func noopCompleteFunc(_ context.Context, _ []models.Message) (*models.LLMResponse, error) {
	return &models.LLMResponse{Content: "done"}, nil
}

// TestAgentWorkerPool_ConcurrentDispatch tests concurrent DispatchTasks calls on
// the same pool instance. The pool uses a channel-based semaphore and
// sync.WaitGroup internally — this test drives multiple concurrent dispatches.
func TestAgentWorkerPool_ConcurrentDispatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	pool := services.NewAgentWorkerPool(5, logger)
	defer pool.Shutdown()

	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			tasks := [][]services.AgenticTask{
				{
					{
						ID:          fmt.Sprintf("task-%d", id),
						Description: fmt.Sprintf("concurrent task %d", id),
						Priority:    1,
					},
				},
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			resultCh, err := pool.DispatchTasks(ctx, tasks, noopCompleteFunc, nil, 1)
			if err != nil {
				return
			}
			// Drain results to unblock the pool goroutine.
			for range resultCh {
			}
		}(i)
	}

	wg.Wait()
}

// TestAgentWorkerPool_ConcurrentDispatchAndShutdown tests the race between
// DispatchTasks and Shutdown. Shutdown cancels the pool context — DispatchTasks
// goroutines must observe the cancellation cleanly.
func TestAgentWorkerPool_ConcurrentDispatchAndShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	pool := services.NewAgentWorkerPool(3, logger)

	var wg sync.WaitGroup
	const dispatchers = 5

	for i := 0; i < dispatchers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			tasks := [][]services.AgenticTask{
				{
					{
						ID:          fmt.Sprintf("shutdown-task-%d", id),
						Description: fmt.Sprintf("task racing with shutdown %d", id),
					},
				},
			}

			ctx := context.Background()
			resultCh, err := pool.DispatchTasks(ctx, tasks, noopCompleteFunc, nil, 1)
			if err != nil {
				return
			}
			for range resultCh {
			}
		}(i)
	}

	// Shutdown races with the dispatchers above.
	pool.Shutdown()
	wg.Wait()
}
