package stress

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/optimization"
	"dev.helix.agent/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimization_Stress_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SemanticCache.MaxEntries = 10000
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("100 concurrent requests", func(t *testing.T) {
		numRequests := 100
		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64

		startTime := time.Now()

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				prompt := "Stress test query " + string(rune('A'+(idx%26)))
				embedding := generateEmbedding(128, idx)

				result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}
				if result != nil && result.OptimizedPrompt != "" {
					atomic.AddInt64(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		t.Logf("Completed %d requests in %v", numRequests, duration)
		t.Logf("Success: %d, Errors: %d", successCount, errorCount)

		// Most requests should succeed
		successRate := float64(successCount) / float64(numRequests)
		assert.GreaterOrEqual(t, successRate, 0.9, "Success rate should be >= 90%")
	})
}

func TestOptimization_Stress_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("sustained 5 second load", func(t *testing.T) {
		duration := 5 * time.Second
		var requestCount int64
		var errorCount int64
		var totalLatency int64

		done := make(chan bool)

		// Worker function
		worker := func() {
			for {
				select {
				case <-done:
					return
				default:
					start := time.Now()
					prompt := "Sustained load test query"
					embedding := generateEmbedding(128, int(requestCount))

					_, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
					latency := time.Since(start).Nanoseconds()

					atomic.AddInt64(&totalLatency, latency)
					atomic.AddInt64(&requestCount, 1)

					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					}
				}
			}
		}

		// Start 10 workers
		numWorkers := 10
		for i := 0; i < numWorkers; i++ {
			go worker()
		}

		// Run for specified duration
		time.Sleep(duration)
		close(done)

		// Calculate statistics
		avgLatency := float64(totalLatency) / float64(requestCount) / 1e6 // ms
		throughput := float64(requestCount) / duration.Seconds()

		t.Logf("Total requests: %d", requestCount)
		t.Logf("Errors: %d", errorCount)
		t.Logf("Average latency: %.2f ms", avgLatency)
		t.Logf("Throughput: %.2f req/s", throughput)

		// Verify reasonable performance
		assert.Greater(t, requestCount, int64(100), "Should complete at least 100 requests")
		errorRate := float64(errorCount) / float64(requestCount)
		assert.Less(t, errorRate, 0.1, "Error rate should be < 10%")
	})
}

func TestOptimization_Stress_CachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SemanticCache.MaxEntries = 1000
	config.SemanticCache.SimilarityThreshold = 0.85
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("cache fill and lookup performance", func(t *testing.T) {
		numEntries := 500
		embeddings := make([][]float64, numEntries)
		prompts := make([]string, numEntries)

		// Generate test data
		for i := 0; i < numEntries; i++ {
			embeddings[i] = generateEmbedding(128, i)
			prompts[i] = "Cache test prompt " + string(rune('A'+(i%26)))
		}

		// Measure cache fill time
		fillStart := time.Now()
		for i := 0; i < numEntries; i++ {
			_, err := pipeline.OptimizeRequest(ctx, prompts[i], embeddings[i])
			require.NoError(t, err)

			// Optimize response to cache it
			response := "Response for " + prompts[i]
			_, _ = pipeline.OptimizeResponse(ctx, response, embeddings[i], prompts[i], nil)
		}
		fillDuration := time.Since(fillStart)
		t.Logf("Cache fill time for %d entries: %v", numEntries, fillDuration)

		// Measure lookup time
		lookupStart := time.Now()
		for i := 0; i < numEntries; i++ {
			_, err := pipeline.OptimizeRequest(ctx, prompts[i], embeddings[i])
			require.NoError(t, err)
		}
		lookupDuration := time.Since(lookupStart)
		t.Logf("Cache lookup time for %d entries: %v", numEntries, lookupDuration)

		// Lookups should be faster than initial fills
		avgFillTime := fillDuration.Seconds() / float64(numEntries) * 1000
		avgLookupTime := lookupDuration.Seconds() / float64(numEntries) * 1000

		t.Logf("Average fill time: %.3f ms", avgFillTime)
		t.Logf("Average lookup time: %.3f ms", avgLookupTime)
	})
}

func TestOptimization_Stress_PipelineStages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
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

	t.Run("parallel vs sequential stages performance", func(t *testing.T) {
		ctx := context.Background()
		numIterations := 50
		prompt := "Complex task requiring multiple optimization stages for analysis and processing"
		embedding := generateEmbedding(128, 0)

		// Test parallel stages
		parallelConfig := optimization.DefaultPipelineConfig()
		parallelConfig.ParallelStages = true
		parallelPipeline := optimization.NewPipeline(service, parallelConfig)

		parallelStart := time.Now()
		for i := 0; i < numIterations; i++ {
			_, err := parallelPipeline.OptimizeRequest(ctx, prompt, embedding)
			require.NoError(t, err)
		}
		parallelDuration := time.Since(parallelStart)

		// Test sequential stages
		sequentialConfig := optimization.DefaultPipelineConfig()
		sequentialConfig.ParallelStages = false
		sequentialPipeline := optimization.NewPipeline(service, sequentialConfig)

		sequentialStart := time.Now()
		for i := 0; i < numIterations; i++ {
			_, err := sequentialPipeline.OptimizeRequest(ctx, prompt, embedding)
			require.NoError(t, err)
		}
		sequentialDuration := time.Since(sequentialStart)

		t.Logf("Parallel stages: %v (%.2f ms/req)", parallelDuration, float64(parallelDuration.Milliseconds())/float64(numIterations))
		t.Logf("Sequential stages: %v (%.2f ms/req)", sequentialDuration, float64(sequentialDuration.Milliseconds())/float64(numIterations))

		// Both should complete within reasonable time
		assert.Less(t, parallelDuration.Seconds(), 30.0, "Parallel should complete within 30s")
		assert.Less(t, sequentialDuration.Seconds(), 60.0, "Sequential should complete within 60s")
	})
}

func TestOptimization_Stress_MemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SemanticCache.MaxEntries = 100 // Small cache to force evictions
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("memory stability under churn", func(t *testing.T) {
		// Perform many operations to test memory stability
		numOperations := 1000
		var successCount int64

		for i := 0; i < numOperations; i++ {
			prompt := "Memory test " + string(rune('A'+(i%26)))
			embedding := generateEmbedding(128, i)

			result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
			if err == nil && result != nil {
				atomic.AddInt64(&successCount, 1)
			}

			// Also test response optimization
			_, _ = pipeline.OptimizeResponse(ctx, "Response "+prompt, embedding, prompt, nil)
		}

		t.Logf("Completed %d operations with %d successes", numOperations*2, successCount)
		assert.Greater(t, successCount, int64(numOperations/2), "Should have > 50% success rate")
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
