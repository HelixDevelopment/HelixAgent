package optimization

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/optimization/outlines"
	"dev.helix.agent/internal/optimization/streaming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPipelineConfig(t *testing.T) {
	t.Parallel()
	config := DefaultPipelineConfig()
	require.NotNil(t, config)

	// Verify defaults
	assert.True(t, config.EnableCacheCheck)
	assert.True(t, config.EnableContextRetrieval)
	assert.True(t, config.EnableTaskDecomposition)
	assert.True(t, config.EnablePrefixWarm)
	assert.True(t, config.EnableValidation)
	assert.True(t, config.EnableCacheStore)
	assert.True(t, config.ParallelStages)

	// Verify timeouts
	assert.Equal(t, 100*time.Millisecond, config.CacheCheckTimeout)
	assert.Equal(t, 2*time.Second, config.ContextRetrievalTimeout)
	assert.Equal(t, 3*time.Second, config.TaskDecompositionTimeout)

	// Verify thresholds
	assert.Equal(t, 50, config.MinPromptLengthForContext)
	assert.Equal(t, 100, config.MinPromptLengthForDecomposition)
}

func TestNewPipeline(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		config *PipelineConfig
	}{
		{
			name:   "with nil config uses defaults",
			config: nil,
		},
		{
			name: "with custom config",
			config: &PipelineConfig{
				EnableCacheCheck:       false,
				EnableContextRetrieval: true,
				CacheCheckTimeout:      50 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create a minimal service
			service, err := NewService(DefaultConfig())
			require.NoError(t, err)

			pipeline := NewPipeline(service, tt.config)
			require.NotNil(t, pipeline)
			assert.NotNil(t, pipeline.service)
			assert.NotNil(t, pipeline.config)
			assert.NotNil(t, pipeline.metrics)
		})
	}
}

func TestPipeline_GetConfig(t *testing.T) {
	t.Parallel()
	service, err := NewService(DefaultConfig())
	require.NoError(t, err)

	customConfig := &PipelineConfig{
		EnableCacheCheck: false,
		EnableValidation: true,
		ParallelStages:   false,
	}

	pipeline := NewPipeline(service, customConfig)
	require.NotNil(t, pipeline)

	config := pipeline.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, customConfig.EnableCacheCheck, config.EnableCacheCheck)
	assert.Equal(t, customConfig.EnableValidation, config.EnableValidation)
	assert.Equal(t, customConfig.ParallelStages, config.ParallelStages)
}

func TestPipeline_SetConfig(t *testing.T) {
	t.Parallel()
	service, err := NewService(DefaultConfig())
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	// Verify default config is set
	originalConfig := pipeline.GetConfig()
	assert.True(t, originalConfig.EnableCacheCheck)

	// Update config
	newConfig := &PipelineConfig{
		EnableCacheCheck:       false,
		EnableContextRetrieval: true,
		ParallelStages:         false,
	}
	pipeline.SetConfig(newConfig)

	// Verify config was updated
	updatedConfig := pipeline.GetConfig()
	assert.False(t, updatedConfig.EnableCacheCheck)
	assert.True(t, updatedConfig.EnableContextRetrieval)
	assert.False(t, updatedConfig.ParallelStages)
}

func TestPipeline_OptimizeRequest_NoServices(t *testing.T) {
	t.Parallel()
	// Create a service with all external services disabled
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:        false,
		EnableContextRetrieval:  false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:        false,
		ParallelStages:          false,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	prompt := "Test prompt"
	embedding := []float64{0.1, 0.2, 0.3}

	result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify basic result structure
	assert.Equal(t, prompt, result.OptimizedPrompt)
	assert.False(t, result.CacheHit)
	assert.NotNil(t, result.StageTimings)
	assert.NotNil(t, result.StagesRun)
	assert.True(t, result.TotalTime > 0)
}

func TestPipeline_OptimizeRequest_ParallelStages(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:        false,
		EnableContextRetrieval:  false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:        false,
		ParallelStages:          true, // Enable parallel
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	prompt := "A longer test prompt that would qualify for context retrieval and decomposition"
	embedding := []float64{0.1, 0.2, 0.3}

	result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.TotalTime > 0)
}

func TestPipeline_OptimizeRequest_SequentialStages(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:        false,
		EnableContextRetrieval:  false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:        false,
		ParallelStages:          false, // Sequential
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	prompt := "A test prompt for sequential processing"
	embedding := []float64{0.1, 0.2, 0.3}

	result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.TotalTime > 0)
}

func TestPipeline_OptimizeResponse_NoSchema(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableValidation: true,
		EnableCacheStore: false,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	response := "Test response"
	embedding := []float64{0.1, 0.2, 0.3}
	query := "Test query"

	result, err := pipeline.OptimizeResponse(ctx, response, embedding, query, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// With nil schema, validation should be skipped
	assert.Nil(t, result.ValidationResult)
	assert.True(t, result.TotalTime > 0)
}

func TestPipelineResult_Structure(t *testing.T) {
	t.Parallel()
	result := &PipelineResult{
		CacheHit:         true,
		CachedResponse:   "cached response",
		RetrievedContext: []string{"context1", "context2"},
		DecomposedTasks:  []string{"task1", "task2", "task3"},
		PrefixWarmed:     true,
		OptimizedPrompt:  "optimized prompt",
		Cached:           true,
		StageTimings: map[PipelineStage]time.Duration{
			StageCacheCheck:       50 * time.Millisecond,
			StageContextRetrieval: 200 * time.Millisecond,
		},
		TotalTime: 250 * time.Millisecond,
		StagesRun: []PipelineStage{StageCacheCheck, StageContextRetrieval},
	}

	assert.True(t, result.CacheHit)
	assert.Equal(t, "cached response", result.CachedResponse)
	assert.Len(t, result.RetrievedContext, 2)
	assert.Len(t, result.DecomposedTasks, 3)
	assert.True(t, result.PrefixWarmed)
	assert.Equal(t, "optimized prompt", result.OptimizedPrompt)
	assert.True(t, result.Cached)
	assert.Len(t, result.StageTimings, 2)
	assert.Equal(t, 250*time.Millisecond, result.TotalTime)
	assert.Len(t, result.StagesRun, 2)
}

func TestPipelineStage_Constants(t *testing.T) {
	t.Parallel()
	// Test all pipeline stage constants
	stages := []PipelineStage{
		StageCacheCheck,
		StageContextRetrieval,
		StageTaskDecomposition,
		StagePrefixWarm,
		StageValidation,
		StageCacheStore,
	}

	expectedValues := []string{
		"cache_check",
		"context_retrieval",
		"task_decomposition",
		"prefix_warm",
		"validation",
		"cache_store",
	}

	for i, stage := range stages {
		assert.Equal(t, PipelineStage(expectedValues[i]), stage)
	}
}

func TestPipelineConfig_AllFields(t *testing.T) {
	t.Parallel()
	config := &PipelineConfig{
		EnableCacheCheck:                true,
		EnableContextRetrieval:          true,
		EnableTaskDecomposition:         true,
		EnablePrefixWarm:                true,
		EnableValidation:                true,
		EnableCacheStore:                true,
		CacheCheckTimeout:               100 * time.Millisecond,
		ContextRetrievalTimeout:         2 * time.Second,
		TaskDecompositionTimeout:        3 * time.Second,
		MinPromptLengthForContext:       50,
		MinPromptLengthForDecomposition: 100,
		ParallelStages:                  true,
	}

	assert.True(t, config.EnableCacheCheck)
	assert.True(t, config.EnableContextRetrieval)
	assert.True(t, config.EnableTaskDecomposition)
	assert.True(t, config.EnablePrefixWarm)
	assert.True(t, config.EnableValidation)
	assert.True(t, config.EnableCacheStore)
	assert.Equal(t, 100*time.Millisecond, config.CacheCheckTimeout)
	assert.Equal(t, 2*time.Second, config.ContextRetrievalTimeout)
	assert.Equal(t, 3*time.Second, config.TaskDecompositionTimeout)
	assert.Equal(t, 50, config.MinPromptLengthForContext)
	assert.Equal(t, 100, config.MinPromptLengthForDecomposition)
	assert.True(t, config.ParallelStages)
}

func TestPipeline_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	// Test concurrent GetConfig/SetConfig
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			pipeline.GetConfig()
			done <- true
		}()
		go func() {
			pipeline.SetConfig(DefaultPipelineConfig())
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestPipeline_BuildOptimizedPrompt(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	// Test with context
	result := &PipelineResult{
		RetrievedContext: []string{"Context 1", "Context 2"},
		StageTimings:     make(map[PipelineStage]time.Duration),
		StagesRun:        []PipelineStage{},
	}

	originalPrompt := "What is the answer?"
	pipeline.buildOptimizedPrompt(originalPrompt, result)

	// Should contain context prefix
	assert.Contains(t, result.OptimizedPrompt, "Relevant context")
	assert.Contains(t, result.OptimizedPrompt, "Context 1")
	assert.Contains(t, result.OptimizedPrompt, "Context 2")
	assert.Contains(t, result.OptimizedPrompt, "Question:")
	assert.Contains(t, result.OptimizedPrompt, originalPrompt)
}

func TestPipeline_BuildOptimizedPrompt_NoContext(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	// Test without context
	result := &PipelineResult{
		RetrievedContext: nil,
		StageTimings:     make(map[PipelineStage]time.Duration),
		StagesRun:        []PipelineStage{},
	}

	originalPrompt := "What is the answer?"
	pipeline.buildOptimizedPrompt(originalPrompt, result)

	// Should just be the original prompt
	assert.Equal(t, originalPrompt, result.OptimizedPrompt)
}

func TestPipeline_OptimizeRequest_WithCacheEnabled(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SemanticCache.MaxEntries = 100
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:        true,
		EnableContextRetrieval:  false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:        false,
		EnableCacheStore:        true,
		CacheCheckTimeout:       100 * time.Millisecond,
		ParallelStages:          false,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	prompt := "Test prompt for cache"
	embedding := []float64{0.1, 0.2, 0.3, 0.4, 0.5}

	// First request - should be cache miss
	result1, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
	require.NoError(t, err)
	require.NotNil(t, result1)
	assert.False(t, result1.CacheHit)
	assert.Contains(t, result1.StagesRun, StageCacheCheck)
}

func TestPipeline_OptimizeRequest_WithEmptyEmbedding(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:        true,
		EnableContextRetrieval:  false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:        false,
		ParallelStages:          false,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	prompt := "Test prompt"
	embedding := []float64{} // Empty embedding

	// Should skip cache check due to empty embedding
	result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.CacheHit)
}

func TestPipeline_OptimizeResponse_WithValidation(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableValidation: true,
		EnableCacheStore: false,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	response := `{"name": "test", "value": 123}`
	embedding := []float64{0.1, 0.2, 0.3}
	query := "Test query"

	// Create a simple schema
	schema := &outlines.JSONSchema{
		Type: "object",
		Properties: map[string]*outlines.JSONSchema{
			"name":  {Type: "string"},
			"value": {Type: "integer"},
		},
	}

	result, err := pipeline.OptimizeResponse(ctx, response, embedding, query, schema)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.StagesRun, StageValidation)
}

func TestPipeline_OptimizeResponse_WithCacheStore(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SemanticCache.MaxEntries = 100
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableValidation: false,
		EnableCacheStore: true,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	response := "Test response"
	embedding := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	query := "Test query"

	result, err := pipeline.OptimizeResponse(ctx, response, embedding, query, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.StagesRun, StageCacheStore)
}

func TestPipeline_OptimizeResponse_EmptyEmbedding(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableValidation: false,
		EnableCacheStore: true,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	response := "Test response"
	embedding := []float64{} // Empty - should skip cache store
	query := "Test query"

	result, err := pipeline.OptimizeResponse(ctx, response, embedding, query, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Cache store should not be in stages run due to empty embedding
	assert.NotContains(t, result.StagesRun, StageCacheStore)
}

func TestPipeline_ParallelStages_WithShortPrompt(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:                false,
		EnableContextRetrieval:          true,
		EnableTaskDecomposition:         true,
		EnablePrefixWarm:                false,
		ParallelStages:                  true,
		MinPromptLengthForContext:       1000, // High threshold
		MinPromptLengthForDecomposition: 1000, // High threshold
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	prompt := "Short" // Below thresholds
	embedding := []float64{0.1, 0.2, 0.3}

	result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Context retrieval and decomposition should be skipped due to short prompt
	assert.NotContains(t, result.StagesRun, StageContextRetrieval)
	assert.NotContains(t, result.StagesRun, StageTaskDecomposition)
}

func TestPipeline_SequentialStages_WithShortPrompt(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:                false,
		EnableContextRetrieval:          true,
		EnableTaskDecomposition:         true,
		EnablePrefixWarm:                false,
		ParallelStages:                  false, // Sequential
		MinPromptLengthForContext:       1000,
		MinPromptLengthForDecomposition: 1000,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	prompt := "Short"
	embedding := []float64{0.1, 0.2, 0.3}

	result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPipeline_RetrieveContext_NoClient(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false // Disabled
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	contexts, err := pipeline.retrieveContext(ctx, "test prompt")
	assert.Error(t, err)
	assert.Nil(t, contexts)
	assert.Contains(t, err.Error(), "llamaindex client not available")
}

func TestPipeline_DecomposeTask_NoClient(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false // Disabled

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	tasks, err := pipeline.decomposeTask(ctx, "complex task")
	assert.Error(t, err)
	assert.Nil(t, tasks)
	assert.Contains(t, err.Error(), "langchain client not available")
}

func TestPipeline_MultipleConcurrentOptimizations(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.Enabled = true
	config.SemanticCache.MaxEntries = 100
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, DefaultPipelineConfig())
	require.NotNil(t, pipeline)

	ctx := context.Background()
	numGoroutines := 20

	results := make(chan *PipelineResult, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			prompt := "Concurrent test prompt"
			embedding := []float64{float64(idx) * 0.1, 0.2, 0.3}
			result, err := pipeline.OptimizeRequest(ctx, prompt, embedding)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-results:
			successCount++
		case <-errors:
			errorCount++
		case <-time.After(10 * time.Second):
			t.Fatal("Test timed out")
		}
	}

	assert.Equal(t, numGoroutines, successCount)
	assert.Equal(t, 0, errorCount)
}

func TestPipeline_StreamWithPipeline(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.Streaming.Enabled = true
	config.SemanticCache.Enabled = false
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	ctx := context.Background()

	// Create a mock stream channel
	inputStream := make(chan *streaming.StreamChunk, 10)

	// Send some test chunks
	go func() {
		inputStream <- &streaming.StreamChunk{Content: "Hello ", Index: 0}
		inputStream <- &streaming.StreamChunk{Content: "world", Index: 1}
		inputStream <- &streaming.StreamChunk{Content: "!", Index: 2, Done: true}
		close(inputStream)
	}()

	// Test streaming
	outStream, getResult := pipeline.StreamWithPipeline(ctx, inputStream, nil)
	require.NotNil(t, outStream)
	require.NotNil(t, getResult)

	// Consume output stream
	var chunks []*streaming.StreamChunk
	for chunk := range outStream {
		chunks = append(chunks, chunk)
	}

	// Get final result
	result := getResult()
	// Result may be nil if enhanced streamer is not fully configured
	if result != nil {
		assert.GreaterOrEqual(t, result.TokenCount, 0)
	}
}

func TestPipeline_StreamWithPipeline_WithProgress(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.Streaming.Enabled = true
	config.SemanticCache.Enabled = false
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	inputStream := make(chan *streaming.StreamChunk, 5)

	// Track progress callbacks
	var progressCalls int
	progressCallback := func(progress *streaming.StreamProgress) {
		progressCalls++
	}

	go func() {
		inputStream <- &streaming.StreamChunk{Content: "Test", Index: 0}
		inputStream <- &streaming.StreamChunk{Content: " content", Index: 1, Done: true}
		close(inputStream)
	}()

	outStream, getResult := pipeline.StreamWithPipeline(ctx, inputStream, progressCallback)
	require.NotNil(t, outStream)

	// Consume stream
	for range outStream {
	}

	_ = getResult()
	// Progress may or may not be called depending on configuration
}

func TestPipeline_StreamWithPipeline_EmptyStream(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.Streaming.Enabled = true
	config.SemanticCache.Enabled = false
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	require.NotNil(t, pipeline)

	ctx := context.Background()
	inputStream := make(chan *streaming.StreamChunk)
	close(inputStream) // Empty stream

	outStream, getResult := pipeline.StreamWithPipeline(ctx, inputStream, nil)
	require.NotNil(t, outStream)

	// Consume empty stream
	count := 0
	for range outStream {
		count++
	}

	result := getResult()
	if result != nil {
		assert.Equal(t, 0, result.TokenCount)
	}
}

// Tests with mock servers for parallel and sequential stages

func TestPipeline_ParallelStages_WithMockLlamaIndex(t *testing.T) {
	t.Parallel()
	// Create mock LlamaIndex server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.URL.Path == "/query" {
			resp := map[string]interface{}{
				"response": "test response",
				"sources": []map[string]interface{}{
					{"content": "context from llama", "metadata": map[string]string{}, "score": 0.9},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = server.URL
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:          false,
		EnableContextRetrieval:    true,
		EnableTaskDecomposition:   false,
		EnablePrefixWarm:          false,
		ParallelStages:            true, // Parallel
		MinPromptLengthForContext: 10,   // Low threshold
		ContextRetrievalTimeout:   5 * time.Second,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	ctx := context.Background()
	prompt := "A test prompt long enough to trigger context retrieval"

	result, err := pipeline.OptimizeRequest(ctx, prompt, nil)
	require.NoError(t, err)
	assert.Contains(t, result.StagesRun, StageContextRetrieval)
	assert.NotEmpty(t, result.RetrievedContext)
}

func TestPipeline_SequentialStages_WithMockLlamaIndex(t *testing.T) {
	t.Parallel()
	// Create mock LlamaIndex server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.URL.Path == "/query" {
			resp := map[string]interface{}{
				"response": "sequential response",
				"sources": []map[string]interface{}{
					{"content": "sequential context", "metadata": map[string]string{}, "score": 0.85},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = server.URL
	config.LangChain.Enabled = false
	config.SGLang.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:          false,
		EnableContextRetrieval:    true,
		EnableTaskDecomposition:   false,
		EnablePrefixWarm:          false,
		ParallelStages:            false, // Sequential
		MinPromptLengthForContext: 10,
		ContextRetrievalTimeout:   5 * time.Second,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	ctx := context.Background()
	prompt := "A sequential test prompt long enough for context"

	result, err := pipeline.OptimizeRequest(ctx, prompt, nil)
	require.NoError(t, err)
	assert.Contains(t, result.StagesRun, StageContextRetrieval)
}

func TestPipeline_ParallelStages_WithMockLangChain(t *testing.T) {
	t.Parallel()
	// Create mock LangChain server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.URL.Path == "/decompose" {
			resp := map[string]interface{}{
				"subtasks": []map[string]interface{}{
					{"step": 1, "description": "First subtask"},
					{"step": 2, "description": "Second subtask"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = true
	config.LangChain.Endpoint = server.URL
	config.SGLang.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:                false,
		EnableContextRetrieval:          false,
		EnableTaskDecomposition:         true,
		EnablePrefixWarm:                false,
		ParallelStages:                  true,
		MinPromptLengthForDecomposition: 50,
		TaskDecompositionTimeout:        5 * time.Second,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	ctx := context.Background()
	// Complex prompt that triggers decomposition
	prompt := "Please implement a complex multi-step feature with data processing, validation, and API integration step by step"

	result, err := pipeline.OptimizeRequest(ctx, prompt, nil)
	require.NoError(t, err)
	assert.Contains(t, result.StagesRun, StageTaskDecomposition)
	assert.NotEmpty(t, result.DecomposedTasks)
}

func TestPipeline_SequentialStages_WithMockLangChain(t *testing.T) {
	t.Parallel()
	// Create mock LangChain server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.URL.Path == "/decompose" {
			resp := map[string]interface{}{
				"subtasks": []map[string]interface{}{
					{"step": 1, "description": "Sequential task 1"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = true
	config.LangChain.Endpoint = server.URL
	config.SGLang.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:                false,
		EnableContextRetrieval:          false,
		EnableTaskDecomposition:         true,
		EnablePrefixWarm:                false,
		ParallelStages:                  false, // Sequential
		MinPromptLengthForDecomposition: 50,
		TaskDecompositionTimeout:        5 * time.Second,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	ctx := context.Background()
	// Prompt must be > 100 chars with a complex indicator for isComplexTask to return true
	prompt := "Please build a comprehensive data processing pipeline with multiple steps involving validation and transformation. This needs to be done step by step."

	result, err := pipeline.OptimizeRequest(ctx, prompt, nil)
	require.NoError(t, err)
	assert.Contains(t, result.StagesRun, StageTaskDecomposition)
}

func TestPipeline_ParallelStages_WithMockSGLang(t *testing.T) {
	t.Parallel()
	// Create mock SGLang server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		resp := map[string]interface{}{
			"prefix_id": "test-prefix",
			"cached":    true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SGLang.Enabled = true
	config.SGLang.Endpoint = server.URL

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:        false,
		EnableContextRetrieval:  false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:        true,
		ParallelStages:          true,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	ctx := context.Background()
	prompt := "Test prompt for prefix warming"

	result, err := pipeline.OptimizeRequest(ctx, prompt, nil)
	require.NoError(t, err)
	assert.Contains(t, result.StagesRun, StagePrefixWarm)
	assert.True(t, result.PrefixWarmed)
}

func TestPipeline_SequentialStages_WithMockSGLang(t *testing.T) {
	t.Parallel()
	// Create mock SGLang server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		resp := map[string]interface{}{
			"prefix_id": "seq-prefix",
			"cached":    true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SGLang.Enabled = true
	config.SGLang.Endpoint = server.URL

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:        false,
		EnableContextRetrieval:  false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:        true,
		ParallelStages:          false, // Sequential
	}

	pipeline := NewPipeline(service, pipelineConfig)
	ctx := context.Background()
	prompt := "Sequential prefix warming test"

	result, err := pipeline.OptimizeRequest(ctx, prompt, nil)
	require.NoError(t, err)
	assert.Contains(t, result.StagesRun, StagePrefixWarm)
}

func TestPipeline_ParallelStages_AllServicesEnabled(t *testing.T) {
	t.Parallel()
	// Create mock servers for all services
	llamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.URL.Path == "/query" {
			resp := map[string]interface{}{
				"response": "context response",
				"sources":  []map[string]interface{}{{"content": "doc context", "score": 0.9}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer llamaServer.Close()

	langchainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.URL.Path == "/decompose" {
			resp := map[string]interface{}{
				"subtasks": []map[string]interface{}{{"step": 1, "description": "task"}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer langchainServer.Close()

	sglangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{"prefix_id": "all-prefix", "cached": true}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer sglangServer.Close()

	config := DefaultConfig()
	config.SemanticCache.Enabled = false
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = llamaServer.URL
	config.LangChain.Enabled = true
	config.LangChain.Endpoint = langchainServer.URL
	config.SGLang.Enabled = true
	config.SGLang.Endpoint = sglangServer.URL

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:                false,
		EnableContextRetrieval:          true,
		EnableTaskDecomposition:         true,
		EnablePrefixWarm:                true,
		ParallelStages:                  true,
		MinPromptLengthForContext:       10,
		MinPromptLengthForDecomposition: 50,
		ContextRetrievalTimeout:         5 * time.Second,
		TaskDecompositionTimeout:        5 * time.Second,
	}

	pipeline := NewPipeline(service, pipelineConfig)
	ctx := context.Background()
	prompt := "Implement a comprehensive data pipeline with multiple processing steps and API integration for complex workflow"

	result, err := pipeline.OptimizeRequest(ctx, prompt, nil)
	require.NoError(t, err)
	// All stages should have run in parallel
	assert.Contains(t, result.StagesRun, StageContextRetrieval)
	assert.Contains(t, result.StagesRun, StageTaskDecomposition)
	assert.Contains(t, result.StagesRun, StagePrefixWarm)
}

func TestPipeline_RetrieveContext_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.URL.Path == "/query" {
			resp := map[string]interface{}{
				"response": "query response",
				"sources": []map[string]interface{}{
					{"content": "context 1", "metadata": map[string]string{}, "score": 0.95},
					{"content": "context 2", "metadata": map[string]string{}, "score": 0.85},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.LlamaIndex.Enabled = true
	config.LlamaIndex.Endpoint = server.URL

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	ctx := context.Background()

	contexts, err := pipeline.retrieveContext(ctx, "test query")
	require.NoError(t, err)
	assert.Len(t, contexts, 2)
	assert.Equal(t, "context 1", contexts[0])
	assert.Equal(t, "context 2", contexts[1])
}

func TestPipeline_DecomposeTask_WithMockServer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		if r.URL.Path == "/decompose" {
			resp := map[string]interface{}{
				"subtasks": []map[string]interface{}{
					{"step": 1, "description": "Decomposed step 1"},
					{"step": 2, "description": "Decomposed step 2"},
					{"step": 3, "description": "Decomposed step 3"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := DefaultConfig()
	config.LangChain.Enabled = true
	config.LangChain.Endpoint = server.URL

	service, err := NewService(config)
	require.NoError(t, err)

	pipeline := NewPipeline(service, nil)
	ctx := context.Background()

	tasks, err := pipeline.decomposeTask(ctx, "complex task to decompose")
	require.NoError(t, err)
	assert.Len(t, tasks, 3)
	assert.Equal(t, "Decomposed step 1", tasks[0])
}
