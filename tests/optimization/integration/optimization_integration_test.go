package integration

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/optimization"
	"dev.helix.agent/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimizationService_Integration(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}

	// Start mock servers
	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	// Create service configuration pointing to mock servers
	config := optimization.DefaultConfig()
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

	// Create service
	service, err := optimization.NewService(config)
	require.NoError(t, err)
	require.NotNil(t, service)

	ctx := context.Background()

	t.Run("service creation with mock servers", func(t *testing.T) {
		assert.NotNil(t, service)
	})

	t.Run("service creation verified", func(t *testing.T) {
		// Service should be created successfully with mock servers
		assert.NotNil(t, service, "Service should be created")
	})

	t.Run("optimize request workflow", func(t *testing.T) {
		prompt := "What is the capital of France? Please provide a detailed explanation."
		embedding := make([]float64, 128)
		for i := range embedding {
			embedding[i] = float64(i) * 0.01
		}

		result, err := service.OptimizeRequest(ctx, prompt, embedding)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify result structure
		assert.NotEmpty(t, result.OptimizedPrompt)
	})

	t.Run("optimize response workflow", func(t *testing.T) {
		response := `{"answer": "Paris is the capital of France", "confidence": 0.95}`
		embedding := make([]float64, 128)
		for i := range embedding {
			embedding[i] = float64(i) * 0.01
		}
		query := "What is the capital of France?"

		result, err := service.OptimizeResponse(ctx, response, embedding, query, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

func TestOptimizationService_GracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}

	// Start mock servers and set them to fail
	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()
	mockServers.SetAllFailing(true)

	config := optimization.DefaultConfig()
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()
	config.LangChain.Enabled = true
	config.LangChain.Endpoint = mockServers.LangChain.URL()
	config.SGLang.Enabled = true
	config.SGLang.Endpoint = mockServers.SGLang.URL()

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("should handle failing services gracefully", func(t *testing.T) {
		prompt := "Test prompt for graceful degradation"
		embedding := make([]float64, 128)

		// Should not panic even with failing services
		result, err := service.OptimizeRequest(ctx, prompt, embedding)
		// The operation may return error or succeed with degraded result
		// Either way, it should not panic
		if err != nil {
			t.Logf("Expected error from failing services: %v", err)
		}
		if result != nil {
			assert.NotEmpty(t, result.OptimizedPrompt)
		}
	})
}

func TestOptimizationService_Timeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}

	// Start mock servers with delays
	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()
	mockServers.SetAllDelay(2 * time.Second)

	config := optimization.DefaultConfig()
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = mockServers.LlamaIndex.URL()
	config.LlamaIndex.Timeout = 100 * time.Millisecond // Short timeout
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	t.Run("should handle timeouts", func(t *testing.T) {
		prompt := "Test prompt for timeout handling"
		embedding := make([]float64, 128)

		result, err := service.OptimizeRequest(ctx, prompt, embedding)
		// Should return result even if external services timeout
		if err != nil {
			t.Logf("Expected timeout error: %v", err)
		}
		if result != nil {
			// Result should still be usable
			assert.NotEmpty(t, result.OptimizedPrompt)
		}
	})
}

func TestOptimizationPipeline_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}

	config := optimization.DefaultConfig()
	config.SemanticCache.Enabled = true
	config.Streaming.Enabled = true
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := optimization.NewService(config)
	require.NoError(t, err)

	pipelineConfig := optimization.DefaultPipelineConfig()
	pipelineConfig.EnableCacheCheck = true
	pipelineConfig.EnableContextRetrieval = false
	pipelineConfig.EnableTaskDecomposition = false
	pipelineConfig.EnablePrefixWarm = false

	pipeline := optimization.NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()

	t.Run("pipeline request optimization", func(t *testing.T) {
		prompt := "Explain quantum computing in simple terms"
		embedding := make([]float64, 128)
		for i := range embedding {
			embedding[i] = float64(i) * 0.01
		}

		result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.NotEmpty(t, result.OptimizedPrompt)
		assert.NotNil(t, result.StageTimings)
		assert.True(t, result.TotalTime > 0)
	})

	t.Run("pipeline response optimization", func(t *testing.T) {
		response := "Quantum computing uses quantum bits (qubits) instead of classical bits."
		embedding := make([]float64, 128)
		query := "What is quantum computing?"

		result, err := pipeline.OptimizeResponse(ctx, response, embedding, query, nil)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.True(t, result.TotalTime > 0)
	})
}

func TestMockServers_RequestCounting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode") // SKIP-OK: #short-mode
	}

	mockServers := mocks.NewOptimizationMockServers()
	defer mockServers.CloseAll()

	t.Run("LlamaIndex request counting", func(t *testing.T) {
		assert.Equal(t, 0, mockServers.LlamaIndex.QueryCount)
		assert.Equal(t, 0, mockServers.LlamaIndex.HealthCount)

		// Reset counters
		mockServers.LlamaIndex.Reset()
		assert.Equal(t, 0, mockServers.LlamaIndex.QueryCount)
	})

	t.Run("LangChain request counting", func(t *testing.T) {
		assert.Equal(t, 0, mockServers.LangChain.DecomposeCount)
		assert.Equal(t, 0, mockServers.LangChain.ExecuteCount)

		mockServers.LangChain.Reset()
		assert.Equal(t, 0, mockServers.LangChain.DecomposeCount)
	})

	t.Run("SGLang request counting", func(t *testing.T) {
		assert.Equal(t, 0, mockServers.SGLang.GenerateCount)
		assert.Equal(t, 0, mockServers.SGLang.WarmPrefixCount)

		mockServers.SGLang.Reset()
		assert.Equal(t, 0, mockServers.SGLang.GenerateCount)
	})

	t.Run("reset all servers", func(t *testing.T) {
		mockServers.ResetAll()
		assert.Equal(t, 0, mockServers.LlamaIndex.QueryCount)
		assert.Equal(t, 0, mockServers.LangChain.DecomposeCount)
		assert.Equal(t, 0, mockServers.SGLang.GenerateCount)
	})
}
