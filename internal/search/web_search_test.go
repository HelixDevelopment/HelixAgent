package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewTavilyProvider(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	p := NewTavilyProvider(logger, "test-key")

	require.NotNil(t, p)
	assert.Equal(t, "test-key", p.apiKey)
	assert.NotNil(t, p.client)
	assert.NotNil(t, p.logger)
}

func TestTavilyProvider_Name(t *testing.T) {
	t.Parallel()

	p := &TavilyProvider{}
	assert.Equal(t, "tavily", p.Name())
}

func TestTavilyProvider_Search_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.Equal(t, "test-key", reqBody["api_key"])
		assert.Equal(t, "golang concurrency", reqBody["query"])

		resp := map[string]interface{}{
			"query":  "golang concurrency",
			"answer": "Go uses goroutines for concurrency.",
			"results": []map[string]interface{}{
				{
					"title":   "Go Concurrency",
					"url":     "https://example.com/go-concurrency",
					"content": "Go provides goroutines and channels.",
					"score":   0.95,
				},
				{
					"title":   "Concurrency Patterns",
					"url":     "https://example.com/patterns",
					"content": "Common patterns include fan-in, fan-out.",
					"score":   0.88,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := zap.NewNop()
	p := &TavilyProvider{
		logger: logger,
		apiKey: "test-key",
		client: &http.Client{Timeout: 5 * time.Second},
	}

	// Override the URL by creating a custom client that redirects
	origSearch := p.Search
	_ = origSearch

	// We need to test with the real method but point at our test server.
	// Use a custom transport to redirect requests.
	p.client = server.Client()
	transport := &urlRewriteTransport{
		base:    http.DefaultTransport,
		baseURL: server.URL,
	}
	p.client.Transport = transport

	result, err := p.Search(context.Background(), "golang concurrency", WebSearchOptions{
		Limit:         5,
		IncludeAnswer: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "golang concurrency", result.Query)
	assert.Equal(t, "Go uses goroutines for concurrency.", result.Answer)
	assert.Equal(t, "tavily", result.Provider)
	assert.Len(t, result.Results, 2)
	assert.Equal(t, "Go Concurrency", result.Results[0].Title)
	assert.Equal(t, "https://example.com/go-concurrency", result.Results[0].URL)
	assert.Equal(t, 2, result.TotalCount)
	assert.True(t, result.SearchTime > 0)
}

func TestTavilyProvider_Search_DefaultMaxResults(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		// When Limit is 0, MaxResults should default to 5
		assert.Equal(t, float64(5), reqBody["max_results"])

		resp := map[string]interface{}{
			"query":   "test",
			"results": []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &TavilyProvider{
		logger: zap.NewNop(),
		apiKey: "key",
		client: &http.Client{
			Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
			Timeout:   5 * time.Second,
		},
	}

	result, err := p.Search(context.Background(), "test", WebSearchOptions{Limit: 0})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Results)
}

func TestTavilyProvider_Search_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := &TavilyProvider{
		logger: zap.NewNop(),
		apiKey: "key",
		client: &http.Client{
			Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
			Timeout:   5 * time.Second,
		},
	}

	result, err := p.Search(context.Background(), "test", WebSearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
	assert.Nil(t, result)
}

func TestTavilyProvider_Search_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	p := &TavilyProvider{
		logger: zap.NewNop(),
		apiKey: "key",
		client: &http.Client{
			Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
			Timeout:   5 * time.Second,
		},
	}

	result, err := p.Search(context.Background(), "test", WebSearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
	assert.Nil(t, result)
}

func TestTavilyProvider_Search_CancelledContext(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // slow response
	}))
	defer server.Close()

	p := &TavilyProvider{
		logger: zap.NewNop(),
		apiKey: "key",
		client: &http.Client{
			Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
			Timeout:   10 * time.Second,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Search(ctx, "test", WebSearchOptions{})
	assert.Error(t, err)
}

func TestNewPerplexityProvider(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	p := NewPerplexityProvider(logger, "pplx-key")

	require.NotNil(t, p)
	assert.Equal(t, "pplx-key", p.apiKey)
	assert.NotNil(t, p.client)
	assert.NotNil(t, p.logger)
}

func TestPerplexityProvider_Name(t *testing.T) {
	t.Parallel()

	p := &PerplexityProvider{}
	assert.Equal(t, "perplexity", p.Name())
}

func TestPerplexityProvider_Search_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer pplx-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		assert.Equal(t, "sonar-pro", reqBody["model"])

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "Here is the answer about Go.",
						"citations": []map[string]interface{}{
							{"url": "https://go.dev", "title": "Go Website"},
							{"url": "https://golang.org/doc", "title": "Go Docs"},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &PerplexityProvider{
		logger: zap.NewNop(),
		apiKey: "pplx-key",
		client: &http.Client{
			Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
			Timeout:   5 * time.Second,
		},
	}

	result, err := p.Search(context.Background(), "what is Go", WebSearchOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "what is Go", result.Query)
	assert.Equal(t, "Here is the answer about Go.", result.Answer)
	assert.Equal(t, "perplexity", result.Provider)
	assert.Len(t, result.Results, 2)
	assert.Equal(t, "Go Website", result.Results[0].Title)
	assert.Equal(t, "https://go.dev", result.Results[0].URL)
}

func TestPerplexityProvider_Search_NoChoices(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &PerplexityProvider{
		logger: zap.NewNop(),
		apiKey: "key",
		client: &http.Client{
			Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
			Timeout:   5 * time.Second,
		},
	}

	result, err := p.Search(context.Background(), "test", WebSearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no results from Perplexity")
	assert.Nil(t, result)
}

func TestPerplexityProvider_Search_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	p := &PerplexityProvider{
		logger: zap.NewNop(),
		apiKey: "bad-key",
		client: &http.Client{
			Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
			Timeout:   5 * time.Second,
		},
	}

	result, err := p.Search(context.Background(), "test", WebSearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 401")
	assert.Nil(t, result)
}

func TestPerplexityProvider_Search_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{bad json"))
	}))
	defer server.Close()

	p := &PerplexityProvider{
		logger: zap.NewNop(),
		apiKey: "key",
		client: &http.Client{
			Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
			Timeout:   5 * time.Second,
		},
	}

	result, err := p.Search(context.Background(), "test", WebSearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
	assert.Nil(t, result)
}

func TestWebSearchOptions_Defaults(t *testing.T) {
	t.Parallel()

	opts := WebSearchOptions{}
	assert.Equal(t, 0, opts.Limit)
	assert.Equal(t, time.Duration(0), opts.Timeout)
	assert.False(t, opts.IncludeAnswer)
	assert.Equal(t, 0, opts.RecencyDays)
	assert.False(t, opts.SafeSearch)
}

func TestWebSearchResult_Fields(t *testing.T) {
	t.Parallel()

	result := WebSearchResult{
		Query:      "test query",
		Answer:     "test answer",
		Results:    []SearchItem{{Title: "t1", URL: "u1"}},
		TotalCount: 1,
		Provider:   "tavily",
		SearchTime: 100 * time.Millisecond,
	}

	assert.Equal(t, "test query", result.Query)
	assert.Equal(t, "test answer", result.Answer)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "tavily", result.Provider)
	assert.Equal(t, 100*time.Millisecond, result.SearchTime)
}

func TestSearchItem_Fields(t *testing.T) {
	t.Parallel()

	item := SearchItem{
		Title:       "Test Title",
		URL:         "https://example.com",
		Snippet:     "A snippet",
		Content:     "Full content",
		PublishedAt: "2026-01-01",
		Score:       0.92,
	}

	assert.Equal(t, "Test Title", item.Title)
	assert.Equal(t, "https://example.com", item.URL)
	assert.Equal(t, "A snippet", item.Snippet)
	assert.Equal(t, "Full content", item.Content)
	assert.Equal(t, "2026-01-01", item.PublishedAt)
	assert.Equal(t, 0.92, item.Score)
}

func TestWebSearchResult_JSON(t *testing.T) {
	t.Parallel()

	result := WebSearchResult{
		Query:      "go test",
		Answer:     "answer",
		Results:    []SearchItem{{Title: "r1", URL: "u1"}},
		TotalCount: 1,
		Provider:   "tavily",
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"query":"go test"`)
	assert.Contains(t, string(data), `"provider":"tavily"`)
}

func TestWebSearchProviderInterface(t *testing.T) {
	t.Parallel()

	// Verify both providers implement the interface
	var _ WebSearchProvider = (*TavilyProvider)(nil)
	var _ WebSearchProvider = (*PerplexityProvider)(nil)
}

// urlRewriteTransport rewrites all request URLs to point at a test server
type urlRewriteTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *urlRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point at the test server
	req.URL.Scheme = "http"
	req.URL.Host = t.baseURL[len("http://"):]
	return t.base.RoundTrip(req)
}
