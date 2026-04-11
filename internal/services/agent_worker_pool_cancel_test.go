package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/models"
)

// noopCompleteFunc is a no-op LLM completion function for tests that need
// DispatchTasks to execute without an actual LLM provider.
func noopCompleteFunc(_ context.Context, _ []models.Message) (*models.LLMResponse, error) {
	return &models.LLMResponse{Content: "test"}, nil
}

func TestAgentWorkerPool_DoubleCancel_NoPanic(t *testing.T) {
	pool := NewAgentWorkerPool(2, logrus.New())
	// Cancel pool's own context
	pool.cancel()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := pool.DispatchTasks(ctx, [][]AgenticTask{}, nil, nil, 0)
	assert.NoError(t, err)
	for range ch {
	}
	cancel()
	time.Sleep(100 * time.Millisecond)
	// If we reach here without panic, test passes
}

func TestAgentWorkerPool_Shutdown_NoPanic(t *testing.T) {
	pool := NewAgentWorkerPool(2, logrus.New())
	ctx := context.Background()
	ch, err := pool.DispatchTasks(ctx, [][]AgenticTask{
		{{ID: "t1", Description: "test task"}},
	}, noopCompleteFunc, nil, 1)
	assert.NoError(t, err)
	// Drain in background
	go func() {
		for range ch {
		}
	}()
	pool.Shutdown()
	time.Sleep(200 * time.Millisecond)
	// If we reach here without panic, test passes
}
