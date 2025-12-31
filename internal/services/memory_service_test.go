package services

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/config"
	llm "github.com/superagent/superagent/internal/llm/cognee"
	"github.com/superagent/superagent/internal/models"
)

func TestNewMemoryService_NilConfig(t *testing.T) {
	ms := NewMemoryService(nil)
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
	require.NotNil(t, ms)
	assert.False(t, ms.enabled)
	assert.NotNil(t, ms.cache)
}

func TestNewMemoryService_EnabledCognee(t *testing.T) {
	cfg := &config.Config{
		Cognee: config.CogneeConfig{
			AutoCognify: true,
			BaseURL:     "http://localhost:8000",
		},
	}
	ms := NewMemoryService(cfg)
	require.NotNil(t, ms)
	assert.True(t, ms.enabled)
	assert.NotNil(t, ms.client)
	assert.Equal(t, "default", ms.dataset)
	assert.NotNil(t, ms.cache)
}

func TestMemoryService_SwitchDataset(t *testing.T) {
	ms := &MemoryService{
		dataset: "original",
		cache:   make(map[string][]models.MemorySource),
	}

	// Add something to cache
	ms.cache["test-key"] = []models.MemorySource{{Content: "test"}}
	assert.Len(t, ms.cache, 1)

	// Switch dataset
	ms.SwitchDataset("new-dataset")

	assert.Equal(t, "new-dataset", ms.dataset)
	assert.Empty(t, ms.cache) // Cache should be cleared
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
	ms := &MemoryService{
		cache: map[string][]models.MemorySource{
			"key1": {{Content: "content1"}},
			"key2": {{Content: "content2"}},
		},
	}

	assert.Len(t, ms.cache, 2)
	ms.ClearCache()
	assert.Empty(t, ms.cache)
}

func TestMemoryService_CacheCleanup(t *testing.T) {
	ms := &MemoryService{
		cache: map[string][]models.MemorySource{
			"key1": {{Content: "content1"}},
			"key2": {{Content: "content2"}},
		},
	}

	assert.Len(t, ms.cache, 2)
	ms.CacheCleanup()
	assert.Empty(t, ms.cache) // Cleanup removes all entries
	assert.False(t, ms.lastCleanup.IsZero())
}

func TestMemoryService_GetStats(t *testing.T) {
	ms := &MemoryService{
		enabled: true,
		dataset: "test-dataset",
		cache: map[string][]models.MemorySource{
			"key1": {{Content: "content1"}},
			"key2": {{Content: "content2"}},
		},
		ttl: 5 * 60 * 1e9, // 5 minutes in nanoseconds
	}

	stats := ms.GetStats()
	assert.Equal(t, true, stats["enabled"])
	assert.Equal(t, 2, stats["cache_size"])
	assert.Equal(t, "test-dataset", stats["dataset"])
	assert.Equal(t, 5.0, stats["ttl_minutes"])
	assert.Equal(t, "", stats["cognee_url"]) // No client
}

func TestMemoryService_GetStats_WithClient(t *testing.T) {
	cfg := &config.Config{
		Cognee: config.CogneeConfig{
			AutoCognify: true,
			BaseURL:     "http://test-cognee:8000",
		},
	}
	ms := NewMemoryService(cfg)

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
				"node3": 123,     // Non-string should be skipped
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
	ms := &MemoryService{
		enabled: false,
		cache:   make(map[string][]models.MemorySource),
	}
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
	ms := &MemoryService{
		enabled: true,
		cache:   make(map[string][]models.MemorySource),
	}
	ctx := context.Background()

	// The cache key format is: ContentType:lowercase(first 50 chars or full content if shorter)
	// Use a short content so the entire content is used as the key
	testContent := "Test Content"
	// Generate exact cache key: text:test content (lowercase)
	cacheKey := "text:test content"
	ms.cache[cacheKey] = []models.MemorySource{{Content: "cached content"}}

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
	ms := &MemoryService{
		enabled: true,
		cache:   make(map[string][]models.MemorySource),
	}
	ctx := context.Background()

	// Pre-populate cache
	cacheKey := "search:test query"
	expectedSources := []models.MemorySource{
		{Content: "cached result 1", DatasetName: "test", RelevanceScore: 0.9},
		{Content: "cached result 2", DatasetName: "test", RelevanceScore: 0.8},
	}
	ms.cache[cacheKey] = expectedSources

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
	ms := &MemoryService{
		enabled: true,
		cache:   make(map[string][]models.MemorySource),
	}
	ctx := context.Background()

	// Pre-populate cache
	cacheKey := "insights:test insights query"
	expectedSources := []models.MemorySource{
		{Content: "insight 1", DatasetName: "insights", RelevanceScore: 1.0},
	}
	ms.cache[cacheKey] = expectedSources

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
	ms := &MemoryService{
		enabled: true,
		cache:   make(map[string][]models.MemorySource),
	}
	ctx := context.Background()

	// Pre-populate cache
	cacheKey := "graph:test graph query"
	expectedSources := []models.MemorySource{
		{Content: "graph result", DatasetName: "graph", RelevanceScore: 0.95},
	}
	ms.cache[cacheKey] = expectedSources

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
