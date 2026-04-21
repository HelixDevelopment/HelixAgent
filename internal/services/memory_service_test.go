package services

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	llm "dev.helix.agent/internal/llm/cognee"
	"dev.helix.agent/internal/models"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeCacheEntry creates a cache entry with the given sources and TTL
func makeCacheEntry(sources []models.MemorySource, ttl time.Duration) *memoryCacheEntry {
	now := time.Now()
	return &memoryCacheEntry{
		sources:   sources,
		createdAt: now,
		expiresAt: now.Add(ttl),
	}
}

// makeExpiredCacheEntry creates an already-expired cache entry
func makeExpiredCacheEntry(sources []models.MemorySource) *memoryCacheEntry {
	past := time.Now().Add(-1 * time.Hour)
	return &memoryCacheEntry{
		sources:   sources,
		createdAt: past,
		expiresAt: past.Add(30 * time.Minute), // expired 30 minutes ago
	}
}

// newTestMemoryService is a test-only helper that constructs a MemoryService
// with initialised safe containers but without launching the cleanup goroutine,
// so tests can inject fixture state and exercise individual methods in isolation.
func newTestMemoryService() *MemoryService {
	ms := &MemoryService{
		cache:  safe.NewStore[string, *memoryCacheEntry](),
		stopCh: make(chan struct{}),
	}
	return ms
}

func TestNewMemoryService_NilConfig(t *testing.T) {
	ms := NewMemoryService(nil)
	defer ms.Stop()
	require.NotNil(t, ms)
	assert.False(t, ms.enabled)
	assert.NotNil(t, ms.cache)
	assert.Equal(t, 5.0, ms.ttl.Minutes())
}

func TestNewMemoryService_DisabledCognee(t *testing.T) {
	cfg := &config.Config{
		Cognee: config.CogneeConfig{
			AutoCognify: false,
		},
	}
	ms := NewMemoryService(cfg)
	defer ms.Stop()
	require.NotNil(t, ms)
	assert.False(t, ms.enabled)
	assert.NotNil(t, ms.cache)
}

func TestMemoryService_EnabledByDefault(t *testing.T) {
	cfg := config.Load()
	assert.True(t, cfg.MemoryEnabled, "MemoryEnabled should default to true")
}

func TestMemoryService_Mem0Fallback(t *testing.T) {
	// When MemoryEnabled is true but Cognee is disabled, service falls back to
	// disabled state (cache-only) since Mem0 is the primary memory backend
	cfg := &config.Config{
		MemoryEnabled: true,
		Cognee: config.CogneeConfig{
			Enabled:     false,
			AutoCognify: false,
		},
	}
	ms := NewMemoryService(cfg)
	defer ms.Stop()
	require.NotNil(t, ms)
	assert.False(t, ms.enabled, "Cognee-based service disabled when Cognee not configured")
	assert.NotNil(t, ms.cache, "Cache still available for Mem0 integration")
}

func TestMemoryService_MemoryDisabled(t *testing.T) {
	cfg := &config.Config{
		MemoryEnabled: false,
		Cognee: config.CogneeConfig{
			Enabled:     true,
			AutoCognify: true,
			BaseURL:     "http://localhost:8000",
		},
	}
	ms := NewMemoryService(cfg)
	defer ms.Stop()
	require.NotNil(t, ms)
	assert.False(t, ms.enabled, "Service disabled when MemoryEnabled is false")
}

func TestNewMemoryService_EnabledCognee(t *testing.T) {
	cfg := &config.Config{
		MemoryEnabled: true,
		Cognee: config.CogneeConfig{
			Enabled:     true,
			AutoCognify: true,
			BaseURL:     "http://localhost:8000",
		},
	}
	ms := NewMemoryService(cfg)
	defer ms.Stop()
	require.NotNil(t, ms)
	assert.True(t, ms.enabled)
	assert.NotNil(t, ms.client)
	assert.Equal(t, "default", ms.dataset)
	assert.NotNil(t, ms.cache)
}

func TestMemoryService_SwitchDataset(t *testing.T) {
	ms := newTestMemoryService()
	ms.dataset = "original"

	// Add something to cache
	ms.cache.Put("test-key", makeCacheEntry([]models.MemorySource{{Content: "test"}}, 5*time.Minute))
	assert.Equal(t, 1, ms.cache.Len())

	// Switch dataset
	ms.SwitchDataset("new-dataset")

	assert.Equal(t, "new-dataset", ms.dataset)
	assert.Equal(t, 0, ms.cache.Len()) // Cache should be cleared
}

func TestMemoryService_GetCurrentDataset(t *testing.T) {
	ms := &MemoryService{
		dataset: "test-dataset",
	}
	assert.Equal(t, "test-dataset", ms.GetCurrentDataset())
}

func TestMemoryService_IsEnabled(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		ms := &MemoryService{enabled: true}
		assert.True(t, ms.IsEnabled())
	})

	t.Run("disabled", func(t *testing.T) {
		ms := &MemoryService{enabled: false}
		assert.False(t, ms.IsEnabled())
	})
}

func TestMemoryService_ClearCache(t *testing.T) {
	ms := newTestMemoryService()
	ms.cache.Put("key1", makeCacheEntry([]models.MemorySource{{Content: "content1"}}, 5*time.Minute))
	ms.cache.Put("key2", makeCacheEntry([]models.MemorySource{{Content: "content2"}}, 5*time.Minute))

	assert.Equal(t, 2, ms.cache.Len())
	ms.ClearCache()
	assert.Equal(t, 0, ms.cache.Len())
}

func TestMemoryService_CacheCleanup(t *testing.T) {
	// Test with mixed expired and non-expired entries
	ms := newTestMemoryService()
	ms.cache.Put("expired1", makeExpiredCacheEntry([]models.MemorySource{{Content: "expired content1"}}))
	ms.cache.Put("expired2", makeExpiredCacheEntry([]models.MemorySource{{Content: "expired content2"}}))
	ms.cache.Put("notexpired1", makeCacheEntry([]models.MemorySource{{Content: "valid content1"}}, 5*time.Minute))

	assert.Equal(t, 3, ms.cache.Len())

	// Run cleanup
	stats := ms.CacheCleanup()

	// Only expired entries should be removed
	assert.Equal(t, 1, ms.cache.Len()) // Only notexpired1 should remain
	assert.True(t, ms.cache.Has("notexpired1"))
	assert.False(t, ms.cache.Has("expired1"))
	assert.False(t, ms.cache.Has("expired2"))

	// Check cleanup stats
	assert.Equal(t, 2, stats.EntriesRemoved)
	assert.Equal(t, 1, stats.EntriesKept)
	assert.False(t, stats.CleanupTime.IsZero())
	assert.True(t, stats.TimeTaken >= 0)

	// Verify lastCleanup was updated
	assert.NotZero(t, ms.lastCleanup.Load())
}

func TestMemoryService_GetStats(t *testing.T) {
	ms := newTestMemoryService()
	ms.enabled = true
	ms.dataset = "test-dataset"
	ms.ttl = 5 * time.Minute
	ms.cleanupInterval.Store(int64(1 * time.Minute))
	ms.cache.Put("key1", makeCacheEntry([]models.MemorySource{{Content: "content1"}}, 5*time.Minute))
	ms.cache.Put("key2", makeCacheEntry([]models.MemorySource{{Content: "content2"}}, 5*time.Minute))

	stats := ms.GetStats()
	assert.Equal(t, true, stats["enabled"])
	assert.Equal(t, 2, stats["cache_size"])
	assert.Equal(t, "test-dataset", stats["dataset"])
	assert.Equal(t, 5.0, stats["ttl_minutes"])
	assert.Equal(t, "", stats["cognee_url"]) // No client
	assert.Equal(t, "1m0s", stats["cleanup_interval"])
}

func TestMemoryService_GetStats_WithClient(t *testing.T) {
	cfg := &config.Config{
		MemoryEnabled: true,
		Cognee: config.CogneeConfig{
			Enabled:     true,
			AutoCognify: true,
			BaseURL:     "http://test-cognee:8000",
		},
	}
	ms := NewMemoryService(cfg)
	defer ms.Stop()

	stats := ms.GetStats()
	assert.Equal(t, true, stats["enabled"])
	assert.Contains(t, stats["cognee_url"], "http://test-cognee:8000")
}

func TestMemoryService_extractKeywords(t *testing.T) {
	ms := &MemoryService{}

	t.Run("from prompt only", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "What is machine learning",
		}
		keywords := ms.extractKeywords(req)
		assert.Contains(t, keywords, "What")
		assert.Contains(t, keywords, "machine")
		assert.Contains(t, keywords, "learning")
	})

	t.Run("from messages", func(t *testing.T) {
		req := &models.LLMRequest{
			Messages: []models.Message{
				{Content: "Tell me about AI"},
				{Content: "And neural networks"},
			},
		}
		keywords := ms.extractKeywords(req)
		assert.Contains(t, keywords, "Tell")
		assert.Contains(t, keywords, "AI")
		assert.Contains(t, keywords, "neural")
		assert.Contains(t, keywords, "networks")
	})

	t.Run("limits to 10 keywords", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12",
		}
		keywords := ms.extractKeywords(req)
		// Should be limited to first 10 words joined by space
		words := len(keywords) - len(strings.ReplaceAll(keywords, " ", "")) + 1
		assert.LessOrEqual(t, words, 10)
	})
}

func TestMemoryService_detectLanguage(t *testing.T) {
	ms := &MemoryService{}

	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{"python import", "import numpy as np", "python"},
		{"python from", "from os import path", "python"},
		{"python def", "def hello():", "python"},
		{"go func", "func main() {}", "go"},
		{"go package", "package main", "go"},
		{"javascript function", "function test() {}", "javascript"},
		{"javascript const", "const x = 5", "javascript"},
		{"javascript let", "let y = 10", "javascript"},
		{"java class", "public class MyClass {}", "java"},
		{"c include", "#include <stdio.h>", "c"},
		{"c main", "int main() { return 0; }", "c"},
		{"unknown", "some random text", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ms.detectLanguage(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMemoryService_extractCodeFromRequest(t *testing.T) {
	ms := &MemoryService{}

	t.Run("no code blocks", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "Just a regular prompt without code",
		}
		code := ms.extractCodeFromRequest(req)
		assert.Empty(t, code)
	})

	t.Run("code block in prompt", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "Here is some code:\n```python\ndef hello():\n    print('hello')\n```",
		}
		code := ms.extractCodeFromRequest(req)
		assert.Contains(t, code, "python")
		assert.Contains(t, code, "def hello()")
	})

	t.Run("code block in messages", func(t *testing.T) {
		req := &models.LLMRequest{
			Messages: []models.Message{
				{Content: "Check this:\n```go\nfunc main() {}\n```"},
			},
		}
		code := ms.extractCodeFromRequest(req)
		assert.Contains(t, code, "go")
		assert.Contains(t, code, "func main()")
	})

	t.Run("multiple code blocks", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "First:\n```js\nconst x = 1;\n```\nSecond:\n```py\ny = 2\n```",
		}
		code := ms.extractCodeFromRequest(req)
		assert.Contains(t, code, "js")
		assert.Contains(t, code, "const x = 1")
		assert.Contains(t, code, "py")
		assert.Contains(t, code, "y = 2")
	})
}

func TestMemoryService_convertToMemorySources(t *testing.T) {
	ms := &MemoryService{}

	t.Run("nil response", func(t *testing.T) {
		result := ms.convertToMemorySources(nil)
		assert.Nil(t, result)
	})

	t.Run("empty graph nodes", func(t *testing.T) {
		resp := &llm.MemoryResponse{
			GraphNodes: map[string]interface{}{},
		}
		result := ms.convertToMemorySources(resp)
		assert.Empty(t, result)
	})

	t.Run("graph nodes with values", func(t *testing.T) {
		resp := &llm.MemoryResponse{
			GraphNodes: map[string]interface{}{
				"node1": "content1",
				"node2": "content2",
				"node3": 123, // Non-string should be skipped
				"node4": "content3",
			},
		}
		result := ms.convertToMemorySources(resp)
		// Check that we got some results (at least the string values)
		assert.NotEmpty(t, result)
		// All results should have correct defaults
		for _, r := range result {
			assert.Equal(t, "default", r.DatasetName)
			assert.Equal(t, 1.0, r.RelevanceScore)
			assert.Equal(t, "cognee", r.SourceType)
		}
	})
}

func TestMemoryService_convertToMemorySourcesFromSearch(t *testing.T) {
	ms := &MemoryService{}

	t.Run("nil response", func(t *testing.T) {
		result := ms.convertToMemorySourcesFromSearch(nil)
		assert.Nil(t, result)
	})

	t.Run("empty results", func(t *testing.T) {
		resp := &llm.SearchResponse{
			Results: []models.MemorySource{},
		}
		result := ms.convertToMemorySourcesFromSearch(resp)
		assert.Empty(t, result)
	})

	t.Run("with results", func(t *testing.T) {
		resp := &llm.SearchResponse{
			Results: []models.MemorySource{
				{
					DatasetName:    "dataset1",
					Content:        "search result 1",
					RelevanceScore: 0.95,
				},
				{
					DatasetName:    "dataset2",
					Content:        "search result 2",
					RelevanceScore: 0.85,
				},
			},
		}
		result := ms.convertToMemorySourcesFromSearch(resp)
		require.Len(t, result, 2)

		assert.Equal(t, "dataset1", result[0].DatasetName)
		assert.Equal(t, "search result 1", result[0].Content)
		assert.Equal(t, 0.95, result[0].RelevanceScore)
		assert.Equal(t, "cognee", result[0].SourceType)

		assert.Equal(t, "dataset2", result[1].DatasetName)
		assert.Equal(t, "search result 2", result[1].Content)
		assert.Equal(t, 0.85, result[1].RelevanceScore)
	})
}

func TestMemoryService_convertInsightsToMemorySources(t *testing.T) {
	ms := &MemoryService{}

	t.Run("nil response", func(t *testing.T) {
		result := ms.convertInsightsToMemorySources(nil)
		assert.Nil(t, result)
	})

	t.Run("empty insights", func(t *testing.T) {
		resp := &llm.InsightsResponse{
			Insights: []map[string]interface{}{},
		}
		result := ms.convertInsightsToMemorySources(resp)
		assert.Empty(t, result)
	})

	t.Run("insights with string content", func(t *testing.T) {
		resp := &llm.InsightsResponse{
			Insights: []map[string]interface{}{
				{"content": "insight 1"},
				{"content": "insight 2"},
			},
		}
		result := ms.convertInsightsToMemorySources(resp)
		require.Len(t, result, 2)

		assert.Equal(t, "insight 1", result[0].Content)
		assert.Equal(t, "insights", result[0].DatasetName)
		assert.Equal(t, 1.0, result[0].RelevanceScore)
		assert.Equal(t, "cognee_insights", result[0].SourceType)
	})

	t.Run("insights without string content", func(t *testing.T) {
		resp := &llm.InsightsResponse{
			Insights: []map[string]interface{}{
				{"key1": "value1", "key2": 123},
			},
		}
		result := ms.convertInsightsToMemorySources(resp)
		require.Len(t, result, 1)

		// Should be JSON serialized
		assert.Contains(t, result[0].Content, "key1")
		assert.Contains(t, result[0].Content, "value1")
	})
}

func TestMemoryService_DisabledServiceErrors(t *testing.T) {
	ms := newTestMemoryService()
	ms.enabled = false
	ctx := context.Background()

	t.Run("AddMemory returns error when disabled", func(t *testing.T) {
		err := ms.AddMemory(ctx, &MemoryRequest{Content: "test", DatasetName: "test", ContentType: "text"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})

	t.Run("SearchMemory returns error when disabled", func(t *testing.T) {
		_, err := ms.SearchMemory(ctx, &SearchRequest{Query: "test", DatasetName: "test", Limit: 10})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})

	t.Run("SearchMemoryWithInsights returns error when disabled", func(t *testing.T) {
		_, err := ms.SearchMemoryWithInsights(ctx, &SearchRequest{Query: "test", DatasetName: "test", Limit: 10})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})

	t.Run("SearchMemoryWithGraphCompletion returns error when disabled", func(t *testing.T) {
		_, err := ms.SearchMemoryWithGraphCompletion(ctx, &SearchRequest{Query: "test", DatasetName: "test", Limit: 10})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})

	t.Run("CognifyDataset returns error when disabled", func(t *testing.T) {
		err := ms.CognifyDataset(ctx, []string{"dataset1"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})

	t.Run("ProcessCodeForMemory returns error when disabled", func(t *testing.T) {
		err := ms.ProcessCodeForMemory(ctx, "code", "go", "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})

	t.Run("CreateDataset returns error when disabled", func(t *testing.T) {
		err := ms.CreateDataset(ctx, "test", "description")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})

	t.Run("ListDatasets returns error when disabled", func(t *testing.T) {
		_, err := ms.ListDatasets(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})

	t.Run("GetMemorySources returns error when disabled", func(t *testing.T) {
		_, err := ms.GetMemorySources(ctx, &models.LLMRequest{Prompt: "test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory service is disabled")
	})
}

func TestMemoryService_EnhanceRequest_DisabledOrNoEnhancement(t *testing.T) {
	ctx := context.Background()

	t.Run("returns nil when disabled", func(t *testing.T) {
		ms := &MemoryService{enabled: false}
		req := &models.LLMRequest{Prompt: "test", MemoryEnhanced: true}
		err := ms.EnhanceRequest(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("returns nil when MemoryEnhanced is false", func(t *testing.T) {
		ms := &MemoryService{enabled: true}
		req := &models.LLMRequest{Prompt: "test", MemoryEnhanced: false}
		err := ms.EnhanceRequest(ctx, req)
		assert.NoError(t, err)
	})
}

func TestMemoryService_EnhanceCodeRequest_Disabled(t *testing.T) {
	ms := &MemoryService{enabled: false}
	req := &models.LLMRequest{Prompt: "```go\nfunc main() {}\n```"}
	err := ms.EnhanceCodeRequest(context.Background(), req)
	assert.NoError(t, err) // Should return nil when disabled
}

func TestMemoryService_EnhanceCodeRequest_NoCode(t *testing.T) {
	ms := &MemoryService{enabled: true}
	req := &models.LLMRequest{Prompt: "no code here"}
	err := ms.EnhanceCodeRequest(context.Background(), req)
	assert.NoError(t, err) // Should return nil when no code
}

// Tests for cache hit paths

func TestMemoryService_AddMemory_CacheHit(t *testing.T) {
	ms := newTestMemoryService()
	ms.enabled = true
	ms.ttl = 5 * time.Minute
	ctx := context.Background()

	// The cache key format is: ContentType:lowercase(first 50 chars or full content if shorter)
	// Use a short content so the entire content is used as the key
	testContent := "Test Content"
	// Generate exact cache key: text:test content (lowercase)
	cacheKey := "text:test content"
	ms.cache.Put(cacheKey, makeCacheEntry([]models.MemorySource{{Content: "cached content"}}, 5*time.Minute))

	// Request with content that matches cache key
	req := &MemoryRequest{
		Content:     testContent,
		DatasetName: "test-dataset",
		ContentType: "text",
	}

	// This should hit the cache and return nil (no error)
	err := ms.AddMemory(ctx, req)
	assert.NoError(t, err)
}

func TestMemoryService_SearchMemory_CacheHit(t *testing.T) {
	ms := newTestMemoryService()
	ms.enabled = true
	ms.ttl = 5 * time.Minute
	ctx := context.Background()

	// Pre-populate cache
	cacheKey := "search:test query"
	expectedSources := []models.MemorySource{
		{Content: "cached result 1", DatasetName: "test", RelevanceScore: 0.9},
		{Content: "cached result 2", DatasetName: "test", RelevanceScore: 0.8},
	}
	ms.cache.Put(cacheKey, makeCacheEntry(expectedSources, 5*time.Minute))

	// Search should return cached results
	req := &SearchRequest{
		Query:       "test query",
		DatasetName: "test",
		Limit:       10,
	}

	sources, err := ms.SearchMemory(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, expectedSources, sources)
}

func TestMemoryService_SearchMemoryWithInsights_CacheHit(t *testing.T) {
	ms := newTestMemoryService()
	ms.enabled = true
	ms.ttl = 5 * time.Minute
	ctx := context.Background()

	// Pre-populate cache
	cacheKey := "insights:test insights query"
	expectedSources := []models.MemorySource{
		{Content: "insight 1", DatasetName: "insights", RelevanceScore: 1.0},
	}
	ms.cache.Put(cacheKey, makeCacheEntry(expectedSources, 5*time.Minute))

	req := &SearchRequest{
		Query:       "test insights query",
		DatasetName: "test",
		Limit:       10,
	}

	sources, err := ms.SearchMemoryWithInsights(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, expectedSources, sources)
}

func TestMemoryService_SearchMemoryWithGraphCompletion_CacheHit(t *testing.T) {
	ms := newTestMemoryService()
	ms.enabled = true
	ms.ttl = 5 * time.Minute
	ctx := context.Background()

	// Pre-populate cache
	cacheKey := "graph:test graph query"
	expectedSources := []models.MemorySource{
		{Content: "graph result", DatasetName: "graph", RelevanceScore: 0.95},
	}
	ms.cache.Put(cacheKey, makeCacheEntry(expectedSources, 5*time.Minute))

	req := &SearchRequest{
		Query:       "test graph query",
		DatasetName: "test",
		Limit:       10,
	}

	sources, err := ms.SearchMemoryWithGraphCompletion(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, expectedSources, sources)
}

func TestMemoryRequest_Fields(t *testing.T) {
	req := MemoryRequest{
		Content:     "test content",
		DatasetName: "my-dataset",
		ContentType: "text",
	}
	assert.Equal(t, "test content", req.Content)
	assert.Equal(t, "my-dataset", req.DatasetName)
	assert.Equal(t, "text", req.ContentType)
}

func TestSearchRequest_Fields(t *testing.T) {
	req := SearchRequest{
		Query:       "search term",
		DatasetName: "my-dataset",
		Limit:       20,
	}
	assert.Equal(t, "search term", req.Query)
	assert.Equal(t, "my-dataset", req.DatasetName)
	assert.Equal(t, 20, req.Limit)
}

// Tests for TTL-based cache cleanup

func TestMemoryService_CacheCleanup_AllExpired(t *testing.T) {
	ms := newTestMemoryService()
	ms.cache.Put("expired1", makeExpiredCacheEntry([]models.MemorySource{{Content: "content1"}}))
	ms.cache.Put("expired2", makeExpiredCacheEntry([]models.MemorySource{{Content: "content2"}}))
	ms.cache.Put("expired3", makeExpiredCacheEntry([]models.MemorySource{{Content: "content3"}}))

	stats := ms.CacheCleanup()

	assert.Equal(t, 0, ms.cache.Len())
	assert.Equal(t, 3, stats.EntriesRemoved)
	assert.Equal(t, 0, stats.EntriesKept)
}

func TestMemoryService_CacheCleanup_NoneExpired(t *testing.T) {
	ms := newTestMemoryService()
	ms.cache.Put("valid1", makeCacheEntry([]models.MemorySource{{Content: "content1"}}, 10*time.Minute))
	ms.cache.Put("valid2", makeCacheEntry([]models.MemorySource{{Content: "content2"}}, 10*time.Minute))

	stats := ms.CacheCleanup()

	assert.Equal(t, 2, ms.cache.Len())
	assert.Equal(t, 0, stats.EntriesRemoved)
	assert.Equal(t, 2, stats.EntriesKept)
}

func TestMemoryService_CacheCleanup_EmptyCache(t *testing.T) {
	ms := newTestMemoryService()

	stats := ms.CacheCleanup()

	assert.Equal(t, 0, ms.cache.Len())
	assert.Equal(t, 0, stats.EntriesRemoved)
	assert.Equal(t, 0, stats.EntriesKept)
	assert.True(t, stats.TimeTaken >= 0)
}

func TestMemoryService_CacheCleanup_StatsTracking(t *testing.T) {
	ms := newTestMemoryService()
	ms.cache.Put("expired", makeExpiredCacheEntry([]models.MemorySource{{Content: "expired"}}))
	ms.cache.Put("valid", makeCacheEntry([]models.MemorySource{{Content: "valid"}}, 10*time.Minute))

	stats := ms.CacheCleanup()

	// Verify stats are correct
	assert.Equal(t, 1, stats.EntriesRemoved)
	assert.Equal(t, 1, stats.EntriesKept)
	assert.False(t, stats.CleanupTime.IsZero())
	assert.True(t, stats.TimeTaken >= 0)

	// Verify lastCleanupStats is set
	lastStats := ms.GetLastCleanupStats()
	assert.NotNil(t, lastStats)
	assert.Equal(t, stats.EntriesRemoved, lastStats.EntriesRemoved)
	assert.Equal(t, stats.EntriesKept, lastStats.EntriesKept)
}

func TestMemoryService_getCachedSources_ExpiredEntry(t *testing.T) {
	ms := newTestMemoryService()
	ms.cache.Put("expired-key", makeExpiredCacheEntry([]models.MemorySource{{Content: "expired content"}}))

	// Should return nil for expired entry
	sources := ms.getCachedSources("expired-key")
	assert.Nil(t, sources)
}

func TestMemoryService_getCachedSources_ValidEntry(t *testing.T) {
	expectedSources := []models.MemorySource{{Content: "valid content"}}
	ms := newTestMemoryService()
	ms.cache.Put("valid-key", makeCacheEntry(expectedSources, 10*time.Minute))

	sources := ms.getCachedSources("valid-key")
	assert.Equal(t, expectedSources, sources)
}

func TestMemoryService_getCachedSources_NonExistent(t *testing.T) {
	ms := newTestMemoryService()

	sources := ms.getCachedSources("non-existent-key")
	assert.Nil(t, sources)
}

func TestMemoryService_setCachedSources(t *testing.T) {
	ms := newTestMemoryService()
	ms.ttl = 5 * time.Minute

	expectedSources := []models.MemorySource{{Content: "test content"}}
	ms.setCachedSources("test-key", expectedSources)

	// Verify entry was created
	entry, exists := ms.cache.Get("test-key")
	require.True(t, exists)
	assert.Equal(t, expectedSources, entry.sources)
	assert.False(t, entry.createdAt.IsZero())
	assert.False(t, entry.expiresAt.IsZero())

	// Verify expiration is set correctly
	assert.True(t, entry.expiresAt.After(entry.createdAt))
	expectedExpiry := entry.createdAt.Add(5 * time.Minute)
	assert.True(t, entry.expiresAt.Equal(expectedExpiry) || entry.expiresAt.After(expectedExpiry.Add(-time.Millisecond)))
}

func TestMemoryService_CleanupInterval(t *testing.T) {
	ms := &MemoryService{}
	ms.cleanupInterval.Store(int64(2 * time.Minute))

	// Test GetCleanupInterval
	assert.Equal(t, 2*time.Minute, ms.GetCleanupInterval())

	// Test SetCleanupInterval
	ms.SetCleanupInterval(5 * time.Minute)
	assert.Equal(t, 5*time.Minute, ms.GetCleanupInterval())
}

func TestMemoryService_StopCleanupRoutine(t *testing.T) {
	ms := newTestMemoryService()
	ms.cleanupInterval.Store(int64(100 * time.Millisecond))

	// Start cleanup routine and stop immediately (tests idempotency)
	ms.wg.Add(1)
	go ms.cleanupRoutine()
	ms.Stop()

	// Calling Stop again should not panic (idempotent)
	ms.Stop()

	assert.True(t, ms.stopped.Load())
}

func TestMemoryService_BackgroundCleanup(t *testing.T) {
	ms := newTestMemoryService()
	ms.cleanupInterval.Store(int64(50 * time.Millisecond))
	ms.ttl = 10 * time.Millisecond

	// Add an entry that will expire quickly
	now := time.Now()
	ms.cache.Put("will-expire", &memoryCacheEntry{
		sources:   []models.MemorySource{{Content: "expiring"}},
		createdAt: now.Add(-1 * time.Second),
		expiresAt: now.Add(-500 * time.Millisecond), // Already expired
	})

	// Add a valid entry
	ms.cache.Put("will-stay", makeCacheEntry([]models.MemorySource{{Content: "staying"}}, 1*time.Hour))

	// Start cleanup routine with WaitGroup tracking
	ms.wg.Add(1)
	go ms.cleanupRoutine()

	// Wait for cleanup to run (cleanupInterval is 50ms)
	require.Eventually(t, func() bool {
		return !ms.cache.Has("will-expire")
	}, 2*time.Second, 10*time.Millisecond, "Expired entry should have been removed by cleanup routine")

	// Stop the routine
	ms.Stop()

	// Verify valid entry is still present
	assert.True(t, ms.cache.Has("will-stay"), "Valid entry should still exist")
}

func TestMemoryService_GetStatsWithCleanupStats(t *testing.T) {
	ms := newTestMemoryService()
	ms.enabled = true
	ms.dataset = "test"
	ms.ttl = 5 * time.Minute
	ms.cleanupInterval.Store(int64(1 * time.Minute))

	// Add some entries and run cleanup
	ms.cache.Put("expired", makeExpiredCacheEntry([]models.MemorySource{{Content: "old"}}))
	ms.cache.Put("valid", makeCacheEntry([]models.MemorySource{{Content: "new"}}, 10*time.Minute))

	ms.CacheCleanup()

	stats := ms.GetStats()

	// Verify cleanup stats are included
	cleanupStats, ok := stats["last_cleanup_stats"].(map[string]interface{})
	require.True(t, ok, "last_cleanup_stats should be a map")
	assert.Equal(t, 1, cleanupStats["entries_removed"])
	assert.Equal(t, 1, cleanupStats["entries_kept"])
	assert.NotEmpty(t, cleanupStats["time_taken"])
}

func TestNewMemoryServiceWithOptions(t *testing.T) {
	cfg := &config.Config{
		Cognee: config.CogneeConfig{
			AutoCognify: false,
		},
	}

	customTTL := 10 * time.Minute
	customCleanupInterval := 2 * time.Minute

	ms := NewMemoryServiceWithOptions(cfg, customTTL, customCleanupInterval)
	defer ms.Stop()

	require.NotNil(t, ms)
	assert.Equal(t, customTTL, ms.ttl)
	assert.Equal(t, customCleanupInterval, ms.GetCleanupInterval())
	assert.False(t, ms.enabled)
	assert.NotNil(t, ms.cache)
	assert.NotNil(t, ms.stopCh)
}

func TestNewMemoryServiceWithOptions_Enabled(t *testing.T) {
	cfg := &config.Config{
		MemoryEnabled: true,
		Cognee: config.CogneeConfig{
			Enabled:     true,
			AutoCognify: true,
			BaseURL:     "http://localhost:8000",
		},
	}

	ms := NewMemoryServiceWithOptions(cfg, 15*time.Minute, 3*time.Minute)
	defer ms.Stop()

	require.NotNil(t, ms)
	assert.True(t, ms.enabled)
	assert.Equal(t, 15*time.Minute, ms.ttl)
	assert.Equal(t, 3*time.Minute, ms.GetCleanupInterval())
}

func TestMemoryService_ConcurrentAccess(t *testing.T) {
	ms := newTestMemoryService()
	ms.ttl = 5 * time.Minute
	ms.cleanupInterval.Store(int64(100 * time.Millisecond))

	// Start cleanup routine with WaitGroup tracking
	ms.wg.Add(1)
	go ms.cleanupRoutine()
	defer ms.Stop()

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				ms.setCachedSources(key, []models.MemorySource{{Content: key}})
				ms.getCachedSources(key)
			}
			done <- true
		}(i)
	}

	// Also run cleanup concurrently
	go func() {
		for i := 0; i < 10; i++ {
			ms.CacheCleanup()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 11; i++ {
		<-done
	}

	// If we get here without deadlock or race condition, test passes
}

// compile-time assertion: atomic.Int64 is used correctly in MemoryService.
// This keeps the `sync/atomic` import live in case the only usage in tests
// is indirect via the MemoryService fields themselves.
var _ = atomic.Int64{}
