package optimization

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig_Extended(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	require.NotNil(t, config)
	assert.True(t, config.Enabled)

	// Semantic cache
	assert.True(t, config.SemanticCache.Enabled)
	assert.Equal(t, 0.85, config.SemanticCache.SimilarityThreshold)
	assert.Equal(t, 10000, config.SemanticCache.MaxEntries)
	assert.Equal(t, 24*time.Hour, config.SemanticCache.TTL)
	assert.Equal(t, "text-embedding-3-small", config.SemanticCache.EmbeddingModel)
	assert.Equal(t, "lru_with_relevance", config.SemanticCache.EvictionPolicy)

	// Structured output
	assert.True(t, config.StructuredOutput.Enabled)
	assert.True(t, config.StructuredOutput.StrictMode)
	assert.True(t, config.StructuredOutput.RetryOnFail)
	assert.Equal(t, 3, config.StructuredOutput.MaxRetries)

	// Streaming
	assert.True(t, config.Streaming.Enabled)
	assert.Equal(t, "word", config.Streaming.BufferType)
	assert.Equal(t, 100*time.Millisecond, config.Streaming.ProgressInterval)

	// SGLang
	assert.True(t, config.SGLang.Enabled)
	assert.Equal(t, "http://localhost:30000", config.SGLang.Endpoint)
	assert.True(t, config.SGLang.FallbackOnUnavailable)

	// LlamaIndex
	assert.True(t, config.LlamaIndex.Enabled)
	assert.True(t, config.LlamaIndex.UseCogneeIndex)

	// LangChain
	assert.True(t, config.LangChain.Enabled)
	assert.Equal(t, "react", config.LangChain.DefaultChain)

	// Guidance
	assert.True(t, config.Guidance.Enabled)
	assert.True(t, config.Guidance.CachePrograms)

	// LMQL
	assert.True(t, config.LMQL.Enabled)
	assert.True(t, config.LMQL.CacheQueries)

	// Fallback
	assert.Equal(t, "skip", config.Fallback.OnServiceUnavailable)
	assert.Equal(t, 30*time.Second, config.Fallback.HealthCheckInterval)
	assert.Equal(t, 5*time.Minute, config.Fallback.RetryUnavailableAfter)
}

func TestConfig_Validate_Valid(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	err := config.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_FixesInvalidSimilarityThreshold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		threshold float64
		expected  float64
	}{
		{"negative threshold", -0.5, 0.85},
		{"too high threshold", 1.5, 0.85},
		{"valid threshold", 0.7, 0.7},
		{"zero threshold", 0.0, 0.0},
		{"one threshold", 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			config := DefaultConfig()
			config.SemanticCache.SimilarityThreshold = tt.threshold
			err := config.Validate()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, config.SemanticCache.SimilarityThreshold)
		})
	}
}

func TestConfig_Validate_FixesInvalidMaxEntries(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.SemanticCache.MaxEntries = -10
	err := config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 10000, config.SemanticCache.MaxEntries)
}

func TestConfig_Validate_FixesInvalidMaxRetries(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.StructuredOutput.MaxRetries = 0
	err := config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 3, config.StructuredOutput.MaxRetries)
}

func TestConfig_Validate_DisabledComponents(t *testing.T) {
	t.Parallel()
	config := &Config{
		Enabled: true,
		SemanticCache: SemanticCacheConfig{
			Enabled:             false,
			SimilarityThreshold: -1.0, // invalid but should not matter
		},
		StructuredOutput: StructuredOutputConfig{
			Enabled:    false,
			MaxRetries: -1, // invalid but should not matter
		},
	}

	err := config.Validate()
	assert.NoError(t, err)

	// Values should remain untouched since components are disabled
	assert.Equal(t, -1.0, config.SemanticCache.SimilarityThreshold)
	assert.Equal(t, -1, config.StructuredOutput.MaxRetries)
}

func TestIsComplexTask_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		prompt   string
		expected bool
	}{
		{"short simple", "hello", false},
		{"long enough with keyword", "Please implement a comprehensive solution step by step for the following complex problem that requires multiple approaches and careful consideration of edge cases", true},
		{"long without keyword", "a" + string(make([]byte, 500)), true},
		{"short with keyword", "implement this", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := isComplexTask(tt.prompt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsIgnoreCaseOptimization(t *testing.T) {
	t.Parallel()
	assert.True(t, containsIgnoreCase("Hello World", "hello"))
	assert.True(t, containsIgnoreCase("HELLO WORLD", "hello world"))
	assert.False(t, containsIgnoreCase("Hello", "xyz"))
}

func TestMin(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 3, min(3, 5))
	assert.Equal(t, 3, min(5, 3))
	assert.Equal(t, 0, min(0, 5))
	assert.Equal(t, -1, min(-1, 5))
	assert.Equal(t, 5, min(5, 5))
}
