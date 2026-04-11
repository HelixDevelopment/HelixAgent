// Package adapters provides MCP server adapter tests.
// This file tests the Brave Search adapter.
package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBraveSearchAdapter(t *testing.T) {
	t.Parallel()
	config := BraveSearchConfig{
		APIKey: "test-api-key",
	}

	adapter := NewBraveSearchAdapter(config)

	assert.NotNil(t, adapter)
	assert.Equal(t, "test-api-key", adapter.config.APIKey)
	assert.NotNil(t, adapter.httpClient)
}

func TestDefaultBraveSearchConfig(t *testing.T) {
	t.Parallel()
	config := DefaultBraveSearchConfig()

	assert.Equal(t, "https://api.search.brave.com/res/v1", config.BaseURL)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 10, config.MaxResults)
	assert.Equal(t, "moderate", config.SafeSearch)
	assert.Equal(t, "us", config.CountryCode)
	assert.Equal(t, "en", config.SearchLanguage)
}

func TestBraveSearchAdapter_GetServerInfo(t *testing.T) {
	t.Parallel()
	adapter := NewBraveSearchAdapter(DefaultBraveSearchConfig())

	info := adapter.GetServerInfo()

	assert.Equal(t, "brave-search", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "Brave Search")
	assert.NotEmpty(t, info.Capabilities)
	assert.Contains(t, info.Capabilities, "web_search")
	assert.Contains(t, info.Capabilities, "image_search")
	assert.Contains(t, info.Capabilities, "video_search")
	assert.Contains(t, info.Capabilities, "news_search")
	assert.Contains(t, info.Capabilities, "local_search")
}

func TestBraveSearchAdapter_ListTools(t *testing.T) {
	t.Parallel()
	adapter := NewBraveSearchAdapter(DefaultBraveSearchConfig())

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	assert.True(t, toolNames["brave_web_search"])
	assert.True(t, toolNames["brave_local_search"])
	assert.True(t, toolNames["brave_image_search"])
	assert.True(t, toolNames["brave_news_search"])
}

func TestBraveSearchAdapter_CallTool_UnknownTool(t *testing.T) {
	t.Parallel()
	adapter := NewBraveSearchAdapter(DefaultBraveSearchConfig())

	result, err := adapter.CallTool(context.Background(), "unknown_tool", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestBraveSearchAdapter_WebSearch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/web/search", r.URL.Path)
		assert.Equal(t, "test-api-key", r.Header.Get("X-Subscription-Token"))
		assert.Contains(t, r.URL.RawQuery, "q=golang")

		response := BraveWebSearchResponse{
			Web: struct {
				Results []struct {
					Title       string `json:"title"`
					URL         string `json:"url"`
					Description string `json:"description"`
				} `json:"results"`
			}{
				Results: []struct {
					Title       string `json:"title"`
					URL         string `json:"url"`
					Description string `json:"description"`
				}{
					{
						Title:       "Go Programming Language",
						URL:         "https://golang.org",
						Description: "Go is an open source programming language",
					},
					{
						Title:       "Go Tutorial",
						URL:         "https://tour.golang.org",
						Description: "An interactive tour of Go",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-api-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "golang",
		"count": 10,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content[0].Text, "Go Programming Language")
	assert.Contains(t, result.Content[0].Text, "golang.org")
}

func TestBraveSearchAdapter_WebSearchWithFreshness(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "freshness=pd")

		response := BraveWebSearchResponse{}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-api-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query":     "news",
		"freshness": "pd",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestBraveSearchAdapter_LocalSearch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/web/search", r.URL.Path)
		assert.Contains(t, r.URL.RawQuery, "q=coffee+shops")
		assert.Contains(t, r.URL.RawQuery, "result_filter=locations")

		response := BraveLocalSearchResponse{
			Locations: &struct {
				Results []struct {
					Title   string `json:"title"`
					Address string `json:"address"`
					Phone   string `json:"phone"`
					Rating  *struct {
						Value float64 `json:"ratingValue"`
						Count int     `json:"ratingCount"`
					} `json:"rating"`
				} `json:"results"`
			}{
				Results: []struct {
					Title   string `json:"title"`
					Address string `json:"address"`
					Phone   string `json:"phone"`
					Rating  *struct {
						Value float64 `json:"ratingValue"`
						Count int     `json:"ratingCount"`
					} `json:"rating"`
				}{
					{
						Title:   "Best Coffee Shop",
						Address: "123 Main St",
						Phone:   "555-1234",
						Rating: &struct {
							Value float64 `json:"ratingValue"`
							Count int     `json:"ratingCount"`
						}{Value: 4.5, Count: 100},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-api-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_local_search", map[string]interface{}{
		"query": "coffee shops",
		"count": 5,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Best Coffee Shop")
	assert.Contains(t, result.Content[0].Text, "123 Main St")
	assert.Contains(t, result.Content[0].Text, "4.5")
}

func TestBraveSearchAdapter_ImageSearch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images/search", r.URL.Path)
		assert.Contains(t, r.URL.RawQuery, "q=cats")

		response := BraveImageSearchResponse{
			Results: []struct {
				Title      string `json:"title"`
				URL        string `json:"url"`
				Source     string `json:"source"`
				Properties struct {
					Width  int `json:"width"`
					Height int `json:"height"`
				} `json:"properties"`
			}{
				{
					Title:  "Cute Cat",
					URL:    "https://example.com/cat.jpg",
					Source: "example.com",
					Properties: struct {
						Width  int `json:"width"`
						Height int `json:"height"`
					}{Width: 800, Height: 600},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-api-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_image_search", map[string]interface{}{
		"query": "cats",
		"count": 10,
		"size":  "large",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Cute Cat")
	assert.Contains(t, result.Content[0].Text, "800x600")
}

func TestBraveSearchAdapter_NewsSearch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/news/search", r.URL.Path)
		assert.Contains(t, r.URL.RawQuery, "q=technology")

		response := BraveNewsSearchResponse{
			Results: []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Source      string `json:"source"`
				Age         string `json:"age"`
			}{
				{
					Title:       "New Tech Breakthrough",
					URL:         "https://news.example.com/article",
					Description: "Scientists discover new technology",
					Source:      "Tech News",
					Age:         "2 hours ago",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-api-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_news_search", map[string]interface{}{
		"query":     "technology",
		"count":     10,
		"freshness": "pd",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "New Tech Breakthrough")
	assert.Contains(t, result.Content[0].Text, "Tech News")
	assert.Contains(t, result.Content[0].Text, "2 hours ago")
}

func TestBraveSearchAdapter_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "invalid-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "test",
	})

	require.NoError(t, err) // Error is returned in ToolResult
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "API error")
}

func TestBraveSearchAdapter_NetworkError(t *testing.T) {
	t.Parallel()
	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = "http://localhost:99999" // Invalid port
	config.Timeout = 100 * time.Millisecond
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "test",
	})

	require.NoError(t, err) // Error is returned in ToolResult
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestBraveSearchAdapter_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "test",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestBraveSearchAdapter_EmptyResults(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := BraveWebSearchResponse{}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "asdfghjklqwertyuiop",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "0 results")
}

func TestBraveSearchAdapter_DefaultCountValue(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "count=10") // Default value

		response := BraveWebSearchResponse{}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	_, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "test",
		// No count specified - should use default
	})

	require.NoError(t, err)
}

func TestBraveSearchAdapter_CustomCountValue(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "count=5")

		response := BraveWebSearchResponse{}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	_, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "test",
		"count": 5,
	})

	require.NoError(t, err)
}

func TestBraveSearchAdapter_SafeSearchConfig(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "safesearch=strict")

		response := BraveWebSearchResponse{}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = server.URL
	config.SafeSearch = "strict"
	adapter := NewBraveSearchAdapter(config)

	_, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "test",
	})

	require.NoError(t, err)
}

func TestBraveSearchAdapter_CountryAndLanguageConfig(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "country=de")
		assert.Contains(t, r.URL.RawQuery, "search_lang=de")

		response := BraveWebSearchResponse{}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = server.URL
	config.CountryCode = "de"
	config.SearchLanguage = "de"
	adapter := NewBraveSearchAdapter(config)

	_, err := adapter.CallTool(context.Background(), "brave_web_search", map[string]interface{}{
		"query": "test",
	})

	require.NoError(t, err)
}

func TestGetIntArg(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		def      int
		expected int
	}{
		{
			name:     "int value",
			args:     map[string]interface{}{"count": 5},
			key:      "count",
			def:      10,
			expected: 5,
		},
		{
			name:     "float64 value",
			args:     map[string]interface{}{"count": float64(15)},
			key:      "count",
			def:      10,
			expected: 15,
		},
		{
			name:     "missing key",
			args:     map[string]interface{}{},
			key:      "count",
			def:      10,
			expected: 10,
		},
		{
			name:     "wrong type",
			args:     map[string]interface{}{"count": "five"},
			key:      "count",
			def:      10,
			expected: 10,
		},
		{
			name:     "nil map",
			args:     nil,
			key:      "count",
			def:      10,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getIntArg(tt.args, tt.key, tt.def)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBraveSearchAdapter_LocalSearchNoLocations(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := BraveLocalSearchResponse{
			Locations: nil, // No locations found
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)

	result, err := adapter.CallTool(context.Background(), "brave_local_search", map[string]interface{}{
		"query": "nonexistent place",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestBraveSearchAdapter_ToolInputSchemas(t *testing.T) {
	t.Parallel()
	adapter := NewBraveSearchAdapter(DefaultBraveSearchConfig())
	tools := adapter.ListTools()

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			schema := tool.InputSchema
			assert.NotNil(t, schema)
			assert.Equal(t, "object", schema["type"])

			properties, ok := schema["properties"].(map[string]interface{})
			require.True(t, ok, "InputSchema should have properties")

			// All tools should have a query property
			query, ok := properties["query"].(map[string]interface{})
			require.True(t, ok, "All tools should have query property")
			assert.Equal(t, "string", query["type"])

			// Check required fields
			required, ok := schema["required"].([]string)
			require.True(t, ok, "InputSchema should have required fields")
			assert.Contains(t, required, "query")
		})
	}
}

// BenchmarkBraveSearchAdapter_WebSearch benchmarks web search
func BenchmarkBraveSearchAdapter_WebSearch(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := BraveWebSearchResponse{}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultBraveSearchConfig()
	config.APIKey = "test-key"
	config.BaseURL = server.URL
	adapter := NewBraveSearchAdapter(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.CallTool(ctx, "brave_web_search", map[string]interface{}{
			"query": "test",
		})
	}
}
