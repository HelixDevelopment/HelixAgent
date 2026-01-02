package optimization

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPipelineConfig(t *testing.T) {
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
	tests := []struct {
		name   string
		config *PipelineConfig
	}{
		{
			name:   "with nil config uses defaults",
			config: nil,
		},
		{
			name:   "with custom config",
			config: &PipelineConfig{
				EnableCacheCheck:       false,
				EnableContextRetrieval: true,
				CacheCheckTimeout:      50 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	// Create a service with all external services disabled
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:       false,
		EnableContextRetrieval: false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:       false,
		ParallelStages:         false,
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
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:       false,
		EnableContextRetrieval: false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:       false,
		ParallelStages:         true, // Enable parallel
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
	config := DefaultConfig()
	config.SGLang.Enabled = false
	config.LlamaIndex.Enabled = false
	config.LangChain.Enabled = false
	config.SemanticCache.Enabled = false

	service, err := NewService(config)
	require.NoError(t, err)

	pipelineConfig := &PipelineConfig{
		EnableCacheCheck:       false,
		EnableContextRetrieval: false,
		EnableTaskDecomposition: false,
		EnablePrefixWarm:       false,
		ParallelStages:         false, // Sequential
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
	config := &PipelineConfig{
		EnableCacheCheck:             true,
		EnableContextRetrieval:       true,
		EnableTaskDecomposition:      true,
		EnablePrefixWarm:             true,
		EnableValidation:             true,
		EnableCacheStore:             true,
		CacheCheckTimeout:            100 * time.Millisecond,
		ContextRetrievalTimeout:      2 * time.Second,
		TaskDecompositionTimeout:     3 * time.Second,
		MinPromptLengthForContext:    50,
		MinPromptLengthForDecomposition: 100,
		ParallelStages:               true,
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
