package gptcache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheRequest(t *testing.T) {
	t.Parallel()
	req := &CacheRequest{
		Query:    "What is the capital of France?",
		Model:    "gpt-4",
		Provider: "openai",
		Parameters: &GenerationParameters{
			Temperature: 0.7,
			MaxTokens:   100,
		},
	}

	assert.Equal(t, "What is the capital of France?", req.Query)
	assert.Equal(t, "gpt-4", req.Model)
	assert.Equal(t, "openai", req.Provider)
	assert.Equal(t, 0.7, req.Parameters.Temperature)
	assert.Equal(t, 100, req.Parameters.MaxTokens)
}

func TestCacheResponse(t *testing.T) {
	t.Parallel()
	resp := &CacheResponse{
		Response:   "The capital of France is Paris.",
		CacheHit:   true,
		Similarity: 0.95,
		CacheID:    "cache-123",
	}

	assert.Equal(t, "The capital of France is Paris.", resp.Response)
	assert.True(t, resp.CacheHit)
	assert.Equal(t, 0.95, resp.Similarity)
	assert.Equal(t, "cache-123", resp.CacheID)
}

func TestGenerationParameters(t *testing.T) {
	t.Parallel()
	params := &GenerationParameters{
		Temperature:      0.8,
		MaxTokens:        500,
		TopP:             0.9,
		TopK:             50,
		Stop:             []string{"\n", "###"},
		FrequencyPenalty: 0.5,
		PresencePenalty:  0.3,
	}

	assert.Equal(t, 0.8, params.Temperature)
	assert.Equal(t, 500, params.MaxTokens)
	assert.Equal(t, 0.9, params.TopP)
	assert.Equal(t, 50, params.TopK)
	assert.Contains(t, params.Stop, "\n")
	assert.Equal(t, 0.5, params.FrequencyPenalty)
	assert.Equal(t, 0.3, params.PresencePenalty)
}

func TestDefaultQueryNormalizer(t *testing.T) {
	t.Parallel()
	normalizer := &DefaultQueryNormalizer{}

	query := "What is AI?"
	normalized := normalizer.Normalize(query)

	assert.Equal(t, query, normalized)
}

func TestCacheEventTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, CacheEventType("hit"), CacheEventHit)
	assert.Equal(t, CacheEventType("miss"), CacheEventMiss)
	assert.Equal(t, CacheEventType("set"), CacheEventSet)
	assert.Equal(t, CacheEventType("evict"), CacheEventEvict)
	assert.Equal(t, CacheEventType("delete"), CacheEventDelete)
	assert.Equal(t, CacheEventType("clear"), CacheEventClear)
}

func TestDefaultCachePolicy(t *testing.T) {
	t.Parallel()
	policy := DefaultCachePolicy()

	assert.Equal(t, 5, policy.MinQueryLength)
	assert.Equal(t, 100000, policy.MaxQueryLength)
	assert.Equal(t, 10, policy.MinResponseLength)
	assert.Equal(t, 0.5, policy.CacheableTemperatureMax)
}

func TestCachePolicy_ShouldCache(t *testing.T) {
	t.Parallel()
	policy := DefaultCachePolicy()

	tests := []struct {
		name           string
		query          string
		responseLength int
		params         *GenerationParameters
		model          string
		provider       string
		expected       bool
	}{
		{
			name:           "valid request",
			query:          "What is the meaning of life?",
			responseLength: 100,
			params:         &GenerationParameters{Temperature: 0.3},
			expected:       true,
		},
		{
			name:           "query too short",
			query:          "Hi",
			responseLength: 100,
			expected:       false,
		},
		{
			name:           "response too short",
			query:          "What is AI?",
			responseLength: 5,
			expected:       false,
		},
		{
			name:           "temperature too high",
			query:          "What is AI?",
			responseLength: 100,
			params:         &GenerationParameters{Temperature: 1.0},
			expected:       false,
		},
		{
			name:           "nil params is valid",
			query:          "What is AI?",
			responseLength: 100,
			params:         nil,
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := &CacheRequest{
				Query:      tt.query,
				Parameters: tt.params,
				Model:      tt.model,
				Provider:   tt.provider,
			}
			result := policy.ShouldCache(req, tt.responseLength)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCachePolicy_ExcludedModels(t *testing.T) {
	t.Parallel()
	policy := DefaultCachePolicy()
	policy.ExcludedModels = []string{"gpt-3.5-turbo"}

	req := &CacheRequest{
		Query: "What is AI?",
		Model: "gpt-3.5-turbo",
	}

	assert.False(t, policy.ShouldCache(req, 100))

	req.Model = "gpt-4"
	assert.True(t, policy.ShouldCache(req, 100))
}

func TestCachePolicy_ExcludedProviders(t *testing.T) {
	t.Parallel()
	policy := DefaultCachePolicy()
	policy.ExcludedProviders = []string{"test-provider"}

	req := &CacheRequest{
		Query:    "What is AI?",
		Provider: "test-provider",
	}

	assert.False(t, policy.ShouldCache(req, 100))

	req.Provider = "openai"
	assert.True(t, policy.ShouldCache(req, 100))
}

func TestIndexTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, IndexType("flat"), IndexTypeFlat)
	assert.Equal(t, IndexType("ivf"), IndexTypeIVF)
	assert.Equal(t, IndexType("hnsw"), IndexTypeHNSW)
	assert.Equal(t, IndexType("lsh"), IndexTypeLSH)
}

func TestDefaultIndexConfig(t *testing.T) {
	t.Parallel()
	config := DefaultIndexConfig()

	assert.Equal(t, IndexTypeFlat, config.Type)
	assert.Equal(t, 100, config.NumPartitions)
	assert.Equal(t, 32, config.NumNeighbors)
	assert.Equal(t, 128, config.NumBits)
	assert.Equal(t, 8, config.NumTables)
}

func TestCacheMetrics(t *testing.T) {
	t.Parallel()
	metrics := &CacheMetrics{
		TotalQueries:      100,
		CacheHits:         75,
		CacheMisses:       25,
		HitRate:           0.75,
		AverageSimilarity: 0.92,
		AverageLatencyMs:  15.5,
		CacheSize:         1000,
		EvictionCount:     50,
	}

	assert.Equal(t, int64(100), metrics.TotalQueries)
	assert.Equal(t, int64(75), metrics.CacheHits)
	assert.Equal(t, 0.75, metrics.HitRate)
	assert.Equal(t, 0.92, metrics.AverageSimilarity)
}

func TestQueryContext(t *testing.T) {
	t.Parallel()
	ctx := &QueryContext{
		ConversationID: "conv-123",
		SessionID:      "session-456",
		UserID:         "user-789",
		Tags:           []string{"technical", "programming"},
	}

	assert.Equal(t, "conv-123", ctx.ConversationID)
	assert.Equal(t, "session-456", ctx.SessionID)
	assert.Equal(t, "user-789", ctx.UserID)
	assert.Contains(t, ctx.Tags, "technical")
}

func TestPersistenceConfig(t *testing.T) {
	t.Parallel()
	config := &PersistenceConfig{
		Enabled:     true,
		Path:        "/tmp/cache",
		Compression: true,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "/tmp/cache", config.Path)
	assert.True(t, config.Compression)
}

func TestClusterConfig(t *testing.T) {
	t.Parallel()
	config := &ClusterConfig{
		Enabled:           true,
		Nodes:             []string{"node1:8080", "node2:8080"},
		ReplicationFactor: 2,
		PartitionCount:    16,
	}

	assert.True(t, config.Enabled)
	assert.Len(t, config.Nodes, 2)
	assert.Equal(t, 2, config.ReplicationFactor)
	assert.Equal(t, 16, config.PartitionCount)
}
