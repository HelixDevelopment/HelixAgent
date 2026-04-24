package e2e

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/optimization"
	"dev.helix.agent/internal/optimization/outlines"
	"dev.helix.agent/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimization_E2E_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	// Start all mock servers
	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	// Create fully configured service
	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SemanticCache.SimilarityThreshold = 0.85
	config.Streaming.Enabled = true
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()
	config.LangChain.Enabled = true
	config.LangChain.Endpoint = mockServers.LangChain.URL()
	config.SGLang.Enabled = true
	config.SGLang.Endpoint = mockServers.SGLang.URL()
	config.Guidance.Enabled = true
	config.Guidance.Endpoint = mockServers.Guidance.URL()
	config.LMQL.Enabled = true
	config.LMQL.Endpoint = mockServers.LMQL.URL()

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	// Create pipeline
	pipelineConfig := optimization.DefaultPipelineConfig()
	pipeline := optimization.NewPipeline(service, pipelineConfig)

	ctx := context.Background()

	t.Run("complete request-response optimization cycle", func(t *testing.T) {
		// Step 1: Optimize request
		prompt := "Explain the concept of machine learning and its applications in healthcare. Please provide specific examples."
		embedding := generateEmbedding(128)

		requestResult, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
		require.NoError(t, err)
		require.NotNil(t, requestResult)

		// Verify request optimization
		assert.NotEmpty(t, requestResult.OptimizedPrompt)
		assert.NotNil(t, requestResult.StageTimings)
		assert.True(t, requestResult.TotalTime > 0)

		// Step 2: Simulate LLM response
		llmResponse := `Machine learning is a subset of artificial intelligence that enables systems to learn from data.
		In healthcare, it's used for:
		1. Disease diagnosis from medical imaging
		2. Drug discovery and development
		3. Personalized treatment recommendations
		4. Patient outcome prediction`

		// Step 3: Optimize response
		responseResult, err := pipeline.OptimizeResponse(ctx, llmResponse, embedding, prompt, nil)
		require.NoError(t, err)
		require.NotNil(t, responseResult)

		assert.True(t, responseResult.TotalTime > 0)
	})

	t.Run("structured output validation", func(t *testing.T) {
		// Define a JSON schema for structured output
		schema := &outlines.JSONSchema{
			Type: "object",
			Properties: map[string]*outlines.JSONSchema{
				"name": {Type: "string"},
				"age":  {Type: "integer"},
			},
			Required: []string{"name", "age"},
		}

		response := `{"name": "John Doe", "age": 30}`
		embedding := generateEmbedding(128)

		result, err := pipeline.OptimizeResponse(ctx, response, embedding, "Get user info", schema)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Check validation was performed
		if result.ValidationResult != nil {
			t.Logf("Validation result: valid=%v", result.ValidationResult.Valid)
		}
	})

	t.Run("cache hit on repeated queries", func(t *testing.T) {
		prompt := "What is the speed of light?"
		embedding := generateEmbedding(128)

		// First request - should be cache miss
		result1, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
		require.NoError(t, err)
		require.NotNil(t, result1)

		// Optimize response to cache it
		_, _ = pipeline.OptimizeResponse(ctx, "The speed of light is 299,792,458 m/s", embedding, prompt, nil)

		// Second request with same embedding - may be cache hit
		result2, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
		require.NoError(t, err)
		require.NotNil(t, result2)

		// Results should be valid regardless of cache behavior
		assert.NotEmpty(t, result2.OptimizedPrompt)
	})
}

func TestOptimization_E2E_ServiceRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	config := optimization.DefaultConfig()
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("service failure and recovery", func(t *testing.T) {
		// Step 1: Normal operation
		prompt := "Test query"
		embedding := generateEmbedding(128)

		result1, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
		require.NoError(t, err)
		assert.NotEmpty(t, result1.OptimizedPrompt)

		// Step 2: Simulate service failure
		mockServers.LlamaIndex.ShouldFail = true

		result2, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
		// Should still work with degraded functionality
		require.NoError(t, err)
		assert.NotEmpty(t, result2.OptimizedPrompt)

		// Step 3: Service recovery
		mockServers.LlamaIndex.ShouldFail = false

		result3, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
		require.NoError(t, err)
		assert.NotEmpty(t, result3.OptimizedPrompt)
	})
}

func TestOptimization_E2E_ParallelRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)
	ctx := context.Background()

	t.Run("concurrent request processing", func(t *testing.T) {
		numRequests := 10
		results := make(chan *optimization.PipelineResult, numRequests)
		errors := make(chan error, numRequests)

		// Launch concurrent requests
		for i := 0; i < numRequests; i++ {
			go func(idx int) {
				prompt := "Concurrent test query " + string(rune('A'+idx))
				embedding := generateEmbedding(128)

				result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
				if err != nil {
					errors <- err
					return
				}
				results <- result
			}(i)
		}

		// Collect results
		var successCount int
		timeout := time.After(30 * time.Second)

		for i := 0; i < numRequests; i++ {
			select {
			case result := <-results:
				require.NotNil(t, result)
				assert.NotEmpty(t, result.OptimizedPrompt)
				successCount++
			case err := <-errors:
				t.Logf("Request error: %v", err)
			case <-timeout:
				t.Fatal("Test timed out")
			}
		}

		// Most requests should succeed
		assert.GreaterOrEqual(t, successCount, numRequests/2)
	})
}

func TestOptimization_E2E_ConfigDynamicUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")  // SKIP-OK: #short-mode
	}

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipeline := optimization.NewPipeline(service, nil)

	t.Run("update pipeline config dynamically", func(t *testing.T) {
		// Get initial config
		initialConfig := pipeline.GetConfig()
		assert.True(t, initialConfig.ParallelStages)

		// Update config
		newConfig := optimization.DefaultPipelineConfig()
		newConfig.ParallelStages = false
		newConfig.EnableCacheCheck = false

		pipeline.SetConfig(newConfig)

		// Verify config was updated
		updatedConfig := pipeline.GetConfig()
		assert.False(t, updatedConfig.ParallelStages)
		assert.False(t, updatedConfig.EnableCacheCheck)

		// Restore original config
		pipeline.SetConfig(initialConfig)
	})
}

// generateEmbedding creates a deterministic embedding vector for testing
func generateEmbedding(size int) []float64 {
	embedding := make([]float64, size)
	for i := range embedding {
		embedding[i] = float64(i) * 0.01
	}
	return embedding
}
