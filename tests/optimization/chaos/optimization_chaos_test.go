package chaos

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/optimization"
	"dev.helix.agent/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimization_Chaos_RandomServiceFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()
	config.LangChain.Enabled = true
	config.LangChain.Endpoint = mockServers.LangChain.URL()
	config.SGLang.Enabled = true
	config.SGLang.Endpoint = mockServers.SGLang.URL()

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("random service failures during operation", func(t *testing.T) {
		numIterations := 100
		var successCount int64
		var errorCount int64

		// Chaos goroutine that randomly toggles service failures
		done := make(chan bool)
		go func() {
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for {
				select {
				case <-done:
					return
				default:
					// Randomly toggle failures
					mockServers.LlamaIndex.ShouldFail = rng.Float32() < 0.3
					mockServers.LangChain.ShouldFail = rng.Float32() < 0.3
					mockServers.SGLang.ShouldFail = rng.Float32() < 0.3
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()

		for i := 0; i < numIterations; i++ {
			prompt := "Chaos test query " + string(rune('A'+(i%26)))
			embedding := generateEmbedding(128, i)

			result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else if result != nil && result.OptimizedPrompt != "" {
				atomic.AddInt64(&successCount, 1)
			}
		}

		close(done)

		t.Logf("Success: %d, Errors: %d (out of %d)", successCount, errorCount, numIterations)

		// System should still function even with failures
		assert.Greater(t, successCount, int64(0), "Should have some successes despite chaos")
	})
}

func TestOptimization_Chaos_ServiceFlapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	config := optimization.DefaultConfig()
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("service flapping (rapid on/off)", func(t *testing.T) {
		numIterations := 50
		var successCount int64

		// Flap service every 20ms
		done := make(chan bool)
		go func() {
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			state := false
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					state = !state
					mockServers.LlamaIndex.ShouldFail = state
				}
			}
		}()

		for i := 0; i < numIterations; i++ {
			prompt := "Flapping test " + string(rune('A'+(i%26)))
			embedding := generateEmbedding(128, i)

			result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
			if err == nil && result != nil {
				atomic.AddInt64(&successCount, 1)
			}
			time.Sleep(5 * time.Millisecond) // Small delay between requests
		}

		close(done)
		mockServers.LlamaIndex.ShouldFail = false

		t.Logf("Success: %d out of %d with flapping service", successCount, numIterations)
		assert.Greater(t, successCount, int64(0), "Should have some successes despite flapping")
	})
}

func TestOptimization_Chaos_VariableLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	config := optimization.DefaultConfig()
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()
	config.LlamaIndex.Timeout = 500 * time.Millisecond

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("variable service latency", func(t *testing.T) {
		numIterations := 30
		var successCount int64
		var timeoutCount int64

		// Randomly vary latency
		done := make(chan bool)
		go func() {
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for {
				select {
				case <-done:
					return
				default:
					// Random delay between 0-300ms
					delay := time.Duration(rng.Intn(300)) * time.Millisecond
					mockServers.LlamaIndex.ResponseDelay = delay
					time.Sleep(50 * time.Millisecond)
				}
			}
		}()

		for i := 0; i < numIterations; i++ {
			requestCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)

			prompt := "Latency test " + string(rune('A'+(i%26)))
			embedding := generateEmbedding(128, i)

			result, err := pipeline.OptimizeRequest(requestCtx, prompt, embedding)
			cancel()

			if err != nil {
				if requestCtx.Err() == context.DeadlineExceeded {
					atomic.AddInt64(&timeoutCount, 1)
				}
			} else if result != nil {
				atomic.AddInt64(&successCount, 1)
			}
		}

		close(done)
		mockServers.LlamaIndex.ResponseDelay = 0

		t.Logf("Success: %d, Timeouts: %d out of %d", successCount, timeoutCount, numIterations)
		// With variable latency, we expect some successes and some timeouts
		totalHandled := successCount + timeoutCount
		assert.Greater(t, totalHandled, int64(0), "Should handle requests despite variable latency")
	})
}

func TestOptimization_Chaos_ConcurrentConfigChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("config changes during requests", func(t *testing.T) {
		numIterations := 100
		var successCount int64
		var wg sync.WaitGroup

		// Config change goroutine
		done := make(chan bool)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					// Toggle config options
					cfg := optimization.DefaultPipelineConfig()
					cfg.ParallelStages = time.Now().UnixNano()%2 == 0
					cfg.EnableCacheCheck = time.Now().UnixNano()%3 != 0
					pipeline.SetConfig(cfg)
					time.Sleep(5 * time.Millisecond)
				}
			}
		}()

		// Request goroutines
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < numIterations/10; j++ {
					select {
					case <-done:
						return
					default:
						prompt := "Config chaos test"
						embedding := generateEmbedding(128, workerID*100+j)

						result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
						if err == nil && result != nil {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}
			}(i)
		}

		// Let it run for a bit
		time.Sleep(2 * time.Second)
		close(done)
		wg.Wait()

		t.Logf("Success: %d with concurrent config changes", successCount)
		// Should not crash despite config changes
		assert.Greater(t, successCount, int64(0), "Should complete requests despite config chaos")
	})
}

func TestOptimization_Chaos_AllServicesDown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	// Set all services to fail
	mockServers.SetAllFailing(true)

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()
	config.LangChain.Enabled = true
	config.LangChain.Endpoint = mockServers.LangChain.URL()
	config.SGLang.Enabled = true
	config.SGLang.Endpoint = mockServers.SGLang.URL()

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("graceful degradation when all services down", func(t *testing.T) {
		numIterations := 20
		var panicCount int64

		for i := 0; i < numIterations; i++ {
			func() {
				defer func() {
					if r := recover(); r != nil {
						atomic.AddInt64(&panicCount, 1)
						t.Logf("Panic recovered: %v", r)
					}
				}()

				prompt := "All services down test"
				embedding := generateEmbedding(128, i)

				result, _ := pipeline.OptimizeRequest(ctx, prompt, embedding)
				// Should return a result with at least the original prompt
				if result != nil {
					assert.NotEmpty(t, result.OptimizedPrompt)
				}
			}()
		}

		assert.Equal(t, int64(0), panicCount, "Should not panic when all services are down")
	})
}

func TestOptimization_Chaos_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	// Add delay to make cancellation more likely to hit mid-operation
	mockServers.SetAllDelay(100 * time.Millisecond)

	config := optimization.DefaultConfig()
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)

	t.Run("random context cancellation", func(t *testing.T) {
		numIterations := 30
		var cancelledCount int64
		var completedCount int64

		for i := 0; i < numIterations; i++ {
			// Random timeout between 10-150ms
			timeout := time.Duration(10+rand.Intn(140)) * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)

			prompt := "Context cancel test"
			embedding := generateEmbedding(128, i)

			result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
			cancel()

			if ctx.Err() != nil {
				atomic.AddInt64(&cancelledCount, 1)
			} else if err == nil && result != nil {
				atomic.AddInt64(&completedCount, 1)
			}
		}

		t.Logf("Completed: %d, Cancelled: %d", completedCount, cancelledCount)
		// Verify system handles cancellation gracefully
		totalHandled := completedCount + cancelledCount
		assert.Greater(t, totalHandled, int64(0), "Should handle context cancellations")
	})
}

// generateEmbedding creates a deterministic embedding vector for testing
func generateEmbedding(size int, seed int) []float64 {
	embedding := make([]float64, size)
	for i := range embedding {
		embedding[i] = float64((i+seed)%100) * 0.01
	}
	return embedding
}
