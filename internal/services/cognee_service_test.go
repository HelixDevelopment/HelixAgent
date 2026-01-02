package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
)

// newTestLogger is defined in cache_factory_test.go

// =====================================================
// COGNEE SERVICE TESTS
// =====================================================

func TestNewCogneeService(t *testing.T) {
	logger := newTestLogger()

	t.Run("creates service with config", func(t *testing.T) {
		cfg := &config.Config{
			Cognee: config.CogneeConfig{
				Enabled: true,
				BaseURL: "http://localhost:8000",
				APIKey:  "test-key",
				Timeout: 30 * time.Second,
			},
		}

		service := NewCogneeService(cfg, logger)
		require.NotNil(t, service)
		assert.Equal(t, "http://localhost:8000", service.baseURL)
		assert.Equal(t, "test-key", service.apiKey)
		assert.NotNil(t, service.config)
		assert.NotNil(t, service.stats)
		assert.NotNil(t, service.feedbackLoop)
	})

	t.Run("creates service with nil logger", func(t *testing.T) {
		cfg := &config.Config{
			Cognee: config.CogneeConfig{
				Enabled: true,
				BaseURL: "http://localhost:8000",
			},
		}

		service := NewCogneeService(cfg, nil)
		require.NotNil(t, service)
		assert.NotNil(t, service.logger)
	})
}

func TestNewCogneeServiceWithConfig(t *testing.T) {
	logger := newTestLogger()

	t.Run("creates service with explicit config", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled:              true,
			BaseURL:              "http://test:8000",
			APIKey:               "explicit-key",
			Timeout:              60 * time.Second,
			AutoCognify:          true,
			EnhancePrompts:       true,
			TemporalAwareness:    true,
			EnableGraphReasoning: true,
			DefaultSearchLimit:   20,
			DefaultDataset:       "custom",
		}

		service := NewCogneeServiceWithConfig(cfg, logger)
		require.NotNil(t, service)
		assert.Equal(t, "http://test:8000", service.baseURL)
		assert.Equal(t, "explicit-key", service.apiKey)
		assert.Equal(t, 60*time.Second, service.client.Timeout)
		assert.True(t, service.config.AutoCognify)
		assert.Equal(t, "custom", service.config.DefaultDataset)
	})

	t.Run("default timeout when zero", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			BaseURL: "http://test:8000",
			Timeout: 0,
		}

		service := NewCogneeServiceWithConfig(cfg, logger)
		assert.Equal(t, 60*time.Second, service.client.Timeout)
	})
}

func TestCogneeService_IsHealthy(t *testing.T) {
	t.Run("healthy when service responds OK", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled: true,
			BaseURL: server.URL,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		assert.True(t, service.IsHealthy(ctx))
	})

	t.Run("unhealthy when service fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled: true,
			BaseURL: server.URL,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		assert.False(t, service.IsHealthy(ctx))
	})

	t.Run("unhealthy when service unreachable", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled: true,
			BaseURL: "http://localhost:99999", // Invalid port
			Timeout: 100 * time.Millisecond,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		assert.False(t, service.IsHealthy(ctx))
	})
}

func TestCogneeService_AddMemory(t *testing.T) {
	t.Run("adds memory successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/add" && r.Method == "POST" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id":          "mem-123",
					"vector_id":   "vec-456",
					"graph_nodes": []string{"node1", "node2"},
					"status":      "success",
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:        true,
			BaseURL:        server.URL,
			DefaultDataset: "default",
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		memory, err := service.AddMemory(ctx, "Test content", "", "text", nil)

		require.NoError(t, err)
		require.NotNil(t, memory)
		assert.Equal(t, "mem-123", memory.ID)
		assert.Equal(t, "vec-456", memory.VectorID)
		assert.Equal(t, "Test content", memory.Content)
		assert.Equal(t, "default", memory.Dataset)
	})

	t.Run("fails when disabled", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled: false,
			BaseURL: "http://localhost:8000",
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		_, err := service.AddMemory(ctx, "content", "", "text", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disabled")
	})
}

func TestCogneeService_SearchMemory(t *testing.T) {
	t.Run("searches memory successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/search" && r.Method == "POST" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []interface{}{
						map[string]interface{}{"text": "result 1"},
						map[string]interface{}{"text": "result 2"},
					},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:            true,
			BaseURL:            server.URL,
			DefaultDataset:     "default",
			DefaultSearchLimit: 10,
			SearchTypes:        []string{"VECTOR"},
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		result, err := service.SearchMemory(ctx, "test query", "", 0)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test query", result.Query)
	})

	t.Run("fails when disabled", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled: false,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		_, err := service.SearchMemory(ctx, "query", "", 10)
		assert.Error(t, err)
	})
}

func TestCogneeService_Cognify(t *testing.T) {
	t.Run("cognifies successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/cognify" && r.Method == "POST" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "completed",
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:        true,
			BaseURL:        server.URL,
			DefaultDataset: "default",
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		err := service.Cognify(ctx, []string{"default"})
		assert.NoError(t, err)
	})
}

func TestCogneeService_EnhanceRequest(t *testing.T) {
	t.Run("enhances request with context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"content":   "relevant context",
						"relevance": 0.9,
					},
				},
			})
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:            true,
			BaseURL:            server.URL,
			EnhancePrompts:     true,
			DefaultDataset:     "default",
			DefaultSearchLimit: 5,
			SearchTypes:        []string{"VECTOR"},
			RelevanceThreshold: 0.5,
			MaxContextSize:     4096,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())
		service.isReady = true

		ctx := context.Background()
		req := &models.LLMRequest{
			Prompt: "What is AI?",
		}

		enhanced, err := service.EnhanceRequest(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, enhanced)
		assert.Equal(t, "What is AI?", enhanced.OriginalPrompt)
	})

	t.Run("returns original when disabled", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled:        false,
			EnhancePrompts: true,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		req := &models.LLMRequest{
			Prompt: "Original prompt",
		}

		enhanced, err := service.EnhanceRequest(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "Original prompt", enhanced.EnhancedPrompt)
		assert.Equal(t, "none", enhanced.EnhancementType)
	})

	t.Run("extracts query from messages when prompt empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)
			// Verify query comes from user message
			assert.Equal(t, "User question here", reqBody["query"])
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{},
			})
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:        true,
			EnhancePrompts: true,
			BaseURL:        server.URL,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())
		service.isReady = true

		ctx := context.Background()
		req := &models.LLMRequest{
			Prompt: "",
			Messages: []models.Message{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "User question here"},
			},
		}
		enhanced, err := service.EnhanceRequest(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, enhanced)
	})

	t.Run("handles disabled service gracefully", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled:        false, // Service disabled
			EnhancePrompts: true,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		req := &models.LLMRequest{Prompt: "Hello world"}
		enhanced, err := service.EnhanceRequest(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, enhanced)
		assert.Equal(t, "none", enhanced.EnhancementType)
		assert.Equal(t, "Hello world", enhanced.EnhancedPrompt)
	})
}

func TestCogneeService_ProcessResponse(t *testing.T) {
	t.Run("stores response successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "mem-123",
				"status": "success",
			})
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:        true,
			BaseURL:        server.URL,
			StoreResponses: true,
			DefaultDataset: "default",
			AutoCognify:    false, // Disable to avoid background goroutine issues
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		req := &models.LLMRequest{
			SessionID: "session-1",
			UserID:    "user-1",
			Prompt:    "Hello",
		}
		resp := &models.LLMResponse{
			Content:      "Hi there!",
			ProviderName: "test",
		}

		err := service.ProcessResponse(ctx, req, resp)
		assert.NoError(t, err)
	})

	t.Run("skips when disabled", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled:        false,
			StoreResponses: true,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		err := service.ProcessResponse(ctx, &models.LLMRequest{}, &models.LLMResponse{})
		assert.NoError(t, err)
	})
}

func TestCogneeService_GetInsights(t *testing.T) {
	t.Run("gets insights successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"insights": []map[string]interface{}{
					{"text": "insight 1"},
					{"text": "insight 2"},
				},
			})
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:              true,
			BaseURL:              server.URL,
			EnableGraphReasoning: true,
			DefaultDataset:       "default",
			DefaultSearchLimit:   10,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		insights, err := service.GetInsights(ctx, "query", nil, 0)

		require.NoError(t, err)
		assert.Len(t, insights, 2)
	})

	t.Run("fails when graph reasoning disabled", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled:              true,
			EnableGraphReasoning: false,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		_, err := service.GetInsights(ctx, "query", nil, 10)
		assert.Error(t, err)
	})
}

func TestCogneeService_ProcessCode(t *testing.T) {
	t.Run("processes code successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/code-pipeline/index" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"processed": true,
					"summary":   "Function that adds numbers",
					"entities":  []string{"func", "add"},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:                true,
			BaseURL:                server.URL,
			EnableCodeIntelligence: true,
			DefaultDataset:         "default",
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		result, err := service.ProcessCode(ctx, "func add(a, b int) int { return a + b }", "go", "")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "go", result.Language)
		assert.Equal(t, "Function that adds numbers", result.Summary)
	})

	t.Run("fails when code intelligence disabled", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled:                true,
			EnableCodeIntelligence: false,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		_, err := service.ProcessCode(ctx, "code", "go", "")
		assert.Error(t, err)
	})
}

func TestCogneeService_DatasetManagement(t *testing.T) {
	t.Run("creates dataset", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/datasets" && r.Method == "POST" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"id": "ds-123"})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{Enabled: true, BaseURL: server.URL}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		err := service.CreateDataset(ctx, "test", "Test dataset", nil)
		assert.NoError(t, err)
	})

	t.Run("lists datasets", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/datasets" && r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"datasets": []map[string]interface{}{
						{"name": "ds1"},
						{"name": "ds2"},
					},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{Enabled: true, BaseURL: server.URL}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		datasets, err := service.ListDatasets(ctx)
		require.NoError(t, err)
		assert.Len(t, datasets, 2)
	})

	t.Run("deletes dataset", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/datasets/test" && r.Method == "DELETE" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{Enabled: true, BaseURL: server.URL}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		err := service.DeleteDataset(ctx, "test")
		assert.NoError(t, err)
	})
}

func TestCogneeService_VisualizeGraph(t *testing.T) {
	t.Run("visualizes graph successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/visualize" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"nodes": []interface{}{},
					"edges": []interface{}{},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{Enabled: true, BaseURL: server.URL, DefaultDataset: "default"}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		graph, err := service.VisualizeGraph(ctx, "", "json")

		require.NoError(t, err)
		assert.NotNil(t, graph)
	})
}

func TestCogneeService_Feedback(t *testing.T) {
	t.Run("records feedback successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "recorded"})
		}))
		defer server.Close()

		cfg := &CogneeServiceConfig{
			Enabled:            true,
			BaseURL:            server.URL,
			EnableFeedbackLoop: true,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		err := service.ProvideFeedback(ctx, "q-123", "query", "response", 0.9, true)
		assert.NoError(t, err)
	})

	t.Run("skips when feedback loop disabled", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled:            true,
			EnableFeedbackLoop: false,
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		ctx := context.Background()
		err := service.ProvideFeedback(ctx, "q-123", "query", "response", 0.9, true)
		assert.NoError(t, err)
	})
}

func TestCogneeService_Stats(t *testing.T) {
	t.Run("tracks statistics", func(t *testing.T) {
		cfg := &CogneeServiceConfig{
			Enabled: true,
			BaseURL: "http://localhost:8000",
		}
		service := NewCogneeServiceWithConfig(cfg, newTestLogger())

		// Manually update stats for testing
		service.stats.mu.Lock()
		service.stats.TotalMemoriesStored = 10
		service.stats.TotalSearches = 20
		service.stats.TotalCognifyOperations = 5
		service.stats.mu.Unlock()

		stats := service.GetStats()
		assert.Equal(t, int64(10), stats.TotalMemoriesStored)
		assert.Equal(t, int64(20), stats.TotalSearches)
		assert.Equal(t, int64(5), stats.TotalCognifyOperations)
	})
}

func TestCogneeService_GetConfig(t *testing.T) {
	cfg := &CogneeServiceConfig{
		Enabled:        true,
		BaseURL:        "http://test:8000",
		DefaultDataset: "custom",
	}
	service := NewCogneeServiceWithConfig(cfg, newTestLogger())

	config := service.GetConfig()
	assert.Equal(t, "http://test:8000", config.BaseURL)
	assert.Equal(t, "custom", config.DefaultDataset)
}

// =====================================================
// HELPER FUNCTION TESTS
// =====================================================

func TestTruncateText(t *testing.T) {
	t.Run("short text unchanged", func(t *testing.T) {
		result := truncateText("short", 100)
		assert.Equal(t, "short", result)
	})

	t.Run("long text truncated", func(t *testing.T) {
		result := truncateText("this is a very long text that should be truncated", 20)
		assert.Len(t, result, 20)
		assert.True(t, len(result) <= 20)
	})
}

func TestContainsCode(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"func main() {}", true},
		{"def hello():", true},
		{"class MyClass:", true},
		{"import os", true},
		{"package main", true},
		{"const x = 5", true},
		{"var y = 10", true},
		{"let z = 15", true},
		{"public void method()", true},
		{"return value", true},
		{"if (condition)", true},
		{"for (i = 0", true},
		{"just plain text", false},
		{"hello world", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := containsCode(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =====================================================
// BENCHMARK TESTS
// =====================================================

func BenchmarkCogneeService_EnhanceRequest(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
	}))
	defer server.Close()

	cfg := &CogneeServiceConfig{
		Enabled:        true,
		BaseURL:        server.URL,
		EnhancePrompts: true,
		SearchTypes:    []string{"VECTOR"},
	}
	service := NewCogneeServiceWithConfig(cfg, newTestLogger())
	service.isReady = true

	req := &models.LLMRequest{Prompt: "Test prompt"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.EnhanceRequest(ctx, req)
	}
}

func BenchmarkCogneeService_SearchMemory(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
	}))
	defer server.Close()

	cfg := &CogneeServiceConfig{
		Enabled:     true,
		BaseURL:     server.URL,
		SearchTypes: []string{"VECTOR"},
	}
	service := NewCogneeServiceWithConfig(cfg, newTestLogger())

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.SearchMemory(ctx, "test query", "", 10)
	}
}

// =====================================================
// SETREADY TESTS
// =====================================================

func TestCogneeService_SetReady(t *testing.T) {
	cfg := &CogneeServiceConfig{
		Enabled: true,
		BaseURL: "http://localhost:8000",
	}
	service := NewCogneeServiceWithConfig(cfg, newTestLogger())

	t.Run("set ready to true", func(t *testing.T) {
		service.SetReady(true)
		assert.True(t, service.IsReady())
	})

	t.Run("set ready to false", func(t *testing.T) {
		service.SetReady(false)
		assert.False(t, service.IsReady())
	})

	t.Run("toggle ready state", func(t *testing.T) {
		service.SetReady(true)
		assert.True(t, service.IsReady())
		service.SetReady(false)
		assert.False(t, service.IsReady())
		service.SetReady(true)
		assert.True(t, service.IsReady())
	})
}

// =====================================================
// COMBINESEARCHRESULTS TESTS
// =====================================================

func TestCogneeService_combineSearchResults(t *testing.T) {
	cfg := &CogneeServiceConfig{
		Enabled: true,
		BaseURL: "http://localhost:8000",
	}
	service := NewCogneeServiceWithConfig(cfg, newTestLogger())

	t.Run("combines vector results", func(t *testing.T) {
		result := &CogneeSearchResult{
			VectorResults: []MemoryEntry{
				{Content: "First result"},
				{Content: "Second result"},
			},
		}

		combined := service.combineSearchResults(result)
		assert.Contains(t, combined, "First result")
		assert.Contains(t, combined, "Second result")
	})

	t.Run("combines insights results", func(t *testing.T) {
		result := &CogneeSearchResult{
			InsightsResults: []map[string]interface{}{
				{"text": "Insight one"},
				{"text": "Insight two"},
			},
		}

		combined := service.combineSearchResults(result)
		assert.Contains(t, combined, "Insight one")
		assert.Contains(t, combined, "Insight two")
	})

	t.Run("combines both vector and insights", func(t *testing.T) {
		result := &CogneeSearchResult{
			VectorResults: []MemoryEntry{
				{Content: "Vector content"},
			},
			InsightsResults: []map[string]interface{}{
				{"text": "Insight content"},
			},
		}

		combined := service.combineSearchResults(result)
		assert.Contains(t, combined, "Vector content")
		assert.Contains(t, combined, "Insight content")
	})

	t.Run("handles empty results", func(t *testing.T) {
		result := &CogneeSearchResult{}
		combined := service.combineSearchResults(result)
		assert.Empty(t, combined)
	})

	t.Run("handles insights without text field", func(t *testing.T) {
		result := &CogneeSearchResult{
			InsightsResults: []map[string]interface{}{
				{"other_field": "value"},
				{"text": "Valid insight"},
			},
		}

		combined := service.combineSearchResults(result)
		assert.Contains(t, combined, "Valid insight")
		assert.NotContains(t, combined, "value")
	})
}

// =====================================================
// CALCULATERELEVANCESCORE TESTS
// =====================================================

func TestCogneeService_calculateRelevanceScore(t *testing.T) {
	cfg := &CogneeServiceConfig{
		Enabled: true,
		BaseURL: "http://localhost:8000",
	}
	service := NewCogneeServiceWithConfig(cfg, newTestLogger())

	t.Run("calculates average relevance", func(t *testing.T) {
		result := &CogneeSearchResult{
			TotalResults: 3,
			VectorResults: []MemoryEntry{
				{Relevance: 0.8},
				{Relevance: 0.9},
				{Relevance: 1.0},
			},
		}

		score := service.calculateRelevanceScore(result)
		assert.InDelta(t, 0.9, score, 0.001)
	})

	t.Run("returns zero for empty results", func(t *testing.T) {
		result := &CogneeSearchResult{
			TotalResults: 0,
		}

		score := service.calculateRelevanceScore(result)
		assert.Equal(t, 0.0, score)
	})

	t.Run("returns default for no relevance scores", func(t *testing.T) {
		result := &CogneeSearchResult{
			TotalResults: 2,
			VectorResults: []MemoryEntry{
				{Relevance: 0},
				{Relevance: 0},
			},
		}

		score := service.calculateRelevanceScore(result)
		assert.Equal(t, 0.5, score) // Default when no valid relevance
	})

	t.Run("handles mixed relevance values", func(t *testing.T) {
		result := &CogneeSearchResult{
			TotalResults: 4,
			VectorResults: []MemoryEntry{
				{Relevance: 0.6},
				{Relevance: 0},   // Should be ignored
				{Relevance: 0.8},
				{Relevance: 0},   // Should be ignored
			},
		}

		score := service.calculateRelevanceScore(result)
		// Only 0.6 and 0.8 are counted: (0.6 + 0.8) / 2 = 0.7
		assert.InDelta(t, 0.7, score, 0.001)
	})

	t.Run("handles single result", func(t *testing.T) {
		result := &CogneeSearchResult{
			TotalResults: 1,
			VectorResults: []MemoryEntry{
				{Relevance: 0.95},
			},
		}

		score := service.calculateRelevanceScore(result)
		assert.Equal(t, 0.95, score)
	})
}

func TestCogneeService_GetGraphCompletion(t *testing.T) {
	logger := newTestLogger()

	t.Run("returns error when disabled", func(t *testing.T) {
		config := &CogneeServiceConfig{
			Enabled:             false,
			EnableGraphReasoning: false,
		}
		service := NewCogneeServiceWithConfig(config, logger)

		ctx := context.Background()
		results, err := service.GetGraphCompletion(ctx, "test query", []string{"dataset1"}, 10)

		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "not enabled")
	})

	t.Run("returns error when graph reasoning disabled", func(t *testing.T) {
		config := &CogneeServiceConfig{
			Enabled:             true,
			EnableGraphReasoning: false,
		}
		service := NewCogneeServiceWithConfig(config, logger)

		ctx := context.Background()
		results, err := service.GetGraphCompletion(ctx, "test query", []string{"dataset1"}, 10)

		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "not enabled")
	})

	t.Run("uses default dataset when empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqBody)

			// Verify default dataset is used
			datasets := reqBody["datasets"].([]interface{})
			assert.Equal(t, "test-dataset", datasets[0].(string))

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{},
			})
		}))
		defer server.Close()

		config := &CogneeServiceConfig{
			Enabled:             true,
			EnableGraphReasoning: true,
			BaseURL:             server.URL,
			DefaultDataset:      "test-dataset",
		}
		service := NewCogneeServiceWithConfig(config, logger)
		service.isReady = true

		ctx := context.Background()
		results, err := service.GetGraphCompletion(ctx, "test query", []string{}, 10)

		assert.NoError(t, err)
		assert.NotNil(t, results)
	})

	t.Run("returns results successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{"entity": "Entity1", "relation": "REL"},
					{"entity": "Entity2", "relation": "REL2"},
				},
			})
		}))
		defer server.Close()

		config := &CogneeServiceConfig{
			Enabled:             true,
			EnableGraphReasoning: true,
			BaseURL:             server.URL,
		}
		service := NewCogneeServiceWithConfig(config, logger)
		service.isReady = true

		ctx := context.Background()
		results, err := service.GetGraphCompletion(ctx, "test query", []string{"dataset1"}, 10)

		assert.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		config := &CogneeServiceConfig{
			Enabled:             true,
			EnableGraphReasoning: true,
			BaseURL:             server.URL,
		}
		service := NewCogneeServiceWithConfig(config, logger)
		service.isReady = true

		ctx := context.Background()
		results, err := service.GetGraphCompletion(ctx, "test query", []string{"dataset1"}, 10)

		assert.Error(t, err)
		assert.Nil(t, results)
	})
}

func TestCogneeService_GetCodeContext(t *testing.T) {
	logger := newTestLogger()

	t.Run("returns nil when code intelligence disabled", func(t *testing.T) {
		config := &CogneeServiceConfig{
			Enabled:                true,
			EnableCodeIntelligence: false,
		}
		service := NewCogneeServiceWithConfig(config, logger)

		ctx := context.Background()
		results, err := service.GetCodeContext(ctx, "test query")

		assert.NoError(t, err)
		assert.Nil(t, results)
	})

	t.Run("returns code context successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"file":     "main.go",
						"content":  "func main() {}",
						"language": "go",
					},
					{
						"file":     "utils.go",
						"content":  "func helper() {}",
						"language": "go",
					},
				},
			})
		}))
		defer server.Close()

		config := &CogneeServiceConfig{
			Enabled:                true,
			EnableCodeIntelligence: true,
			BaseURL:                server.URL,
		}
		service := NewCogneeServiceWithConfig(config, logger)
		service.isReady = true

		ctx := context.Background()
		results, err := service.GetCodeContext(ctx, "test query")

		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, 2)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		config := &CogneeServiceConfig{
			Enabled:                true,
			EnableCodeIntelligence: true,
			BaseURL:                server.URL,
		}
		service := NewCogneeServiceWithConfig(config, logger)
		service.isReady = true

		ctx := context.Background()
		results, err := service.GetCodeContext(ctx, "test query")

		assert.Error(t, err)
		assert.Nil(t, results)
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		config := &CogneeServiceConfig{
			Enabled:                true,
			EnableCodeIntelligence: true,
			BaseURL:                server.URL,
		}
		service := NewCogneeServiceWithConfig(config, logger)
		service.isReady = true

		ctx := context.Background()
		results, err := service.GetCodeContext(ctx, "test query")

		assert.Error(t, err)
		assert.Nil(t, results)
	})
}
