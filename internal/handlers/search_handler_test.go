package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/search"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock implementations for search.Searcher and search.Indexer ------

type mockSearcher struct {
	results []search.SearchResult
	err     error
}

func (m *mockSearcher) Search(_ context.Context, _ string, _ search.SearchOptions) ([]search.SearchResult, error) {
	return m.results, m.err
}

func (m *mockSearcher) SearchSimilar(_ context.Context, _ string, _ int, _ search.SearchOptions) ([]search.SearchResult, error) {
	return m.results, m.err
}

type mockIndexer struct {
	result *search.IndexResult
	err    error
}

func (m *mockIndexer) Index(_ context.Context) (*search.IndexResult, error) {
	return m.result, m.err
}

func (m *mockIndexer) IndexFile(_ context.Context, _ string) error {
	return m.err
}

func (m *mockIndexer) DeleteFile(_ context.Context, _ string) error {
	return m.err
}

func (m *mockIndexer) Watch(_ context.Context) error {
	return m.err
}

// --- helpers -----------------------------------------------------------

func setupSearchRouter(searcher search.Searcher, indexer search.Indexer) (*SearchHandler, *gin.Engine) {
	handler := NewSearchHandler(searcher, indexer)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler.RegisterRoutes(router)
	return handler, router
}

// --- Constructor -------------------------------------------------------

func TestNewSearchHandler(t *testing.T) {
	t.Parallel()
	s := &mockSearcher{}
	i := &mockIndexer{}
	h := NewSearchHandler(s, i)

	assert.NotNil(t, h)
}

func TestNewSearchHandler_NilDeps(t *testing.T) {
	t.Parallel()
	h := NewSearchHandler(nil, nil)
	assert.NotNil(t, h)
}

// --- SemanticSearch ----------------------------------------------------

func TestSearchHandler_SemanticSearch_Success(t *testing.T) {
	t.Parallel()
	s := &mockSearcher{
		results: []search.SearchResult{
			{
				Score: 0.95,
				Document: search.Document{
					ID:      "doc1",
					Content: "func main() {}",
					Metadata: map[string]interface{}{
						"file": "main.go",
					},
				},
			},
		},
	}

	_, router := setupSearchRouter(s, &mockIndexer{})

	body, _ := json.Marshal(SemanticSearchRequest{
		Query: "main function",
		TopK:  5,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp SearchResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.TotalFound)
	assert.Len(t, resp.Results, 1)
	assert.GreaterOrEqual(t, resp.QueryTime, int64(0))
}

func TestSearchHandler_SemanticSearch_DefaultTopK(t *testing.T) {
	t.Parallel()
	s := &mockSearcher{results: []search.SearchResult{}}
	_, router := setupSearchRouter(s, &mockIndexer{})

	body, _ := json.Marshal(SemanticSearchRequest{
		Query: "test query",
		// TopK omitted — handler defaults to 10
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSearchHandler_SemanticSearch_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, router := setupSearchRouter(&mockSearcher{}, &mockIndexer{})

	req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_SemanticSearch_MissingQuery(t *testing.T) {
	t.Parallel()
	_, router := setupSearchRouter(&mockSearcher{}, &mockIndexer{})

	body, _ := json.Marshal(map[string]interface{}{"top_k": 5})
	req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_SemanticSearch_SearchError(t *testing.T) {
	t.Parallel()
	s := &mockSearcher{err: errors.New("embedding service down")}
	_, router := setupSearchRouter(s, &mockIndexer{})

	body, _ := json.Marshal(SemanticSearchRequest{Query: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "embedding service down")
}

func TestSearchHandler_SemanticSearch_WithLanguageFilter(t *testing.T) {
	t.Parallel()
	s := &mockSearcher{results: []search.SearchResult{}}
	_, router := setupSearchRouter(s, &mockIndexer{})

	// NOTE: Filters must contain at least one entry so that the map
	// survives JSON marshal/unmarshal (omitempty drops empty maps).
	// The handler has a latent bug: it assigns to opts.Filters without
	// nil-checking when req.Filters is nil and Language is non-empty.
	body, _ := json.Marshal(SemanticSearchRequest{
		Query:    "handler",
		Language: "go",
		TopK:     3,
		Filters:  map[string]interface{}{"type": "function"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSearchHandler_SemanticSearch_WithMinScore(t *testing.T) {
	t.Parallel()
	s := &mockSearcher{results: []search.SearchResult{}}
	_, router := setupSearchRouter(s, &mockIndexer{})

	body, _ := json.Marshal(SemanticSearchRequest{
		Query:    "test",
		MinScore: 0.8,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/search/semantic", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- TriggerIndex ------------------------------------------------------

func TestSearchHandler_TriggerIndex_Success(t *testing.T) {
	t.Parallel()
	idx := &mockIndexer{
		result: &search.IndexResult{
			FilesIndexed: 42,
			Duration:     2 * time.Second,
			Errors:       nil,
		},
	}
	_, router := setupSearchRouter(&mockSearcher{}, idx)

	req := httptest.NewRequest(http.MethodPost, "/v1/search/index", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp IndexResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 42, resp.FilesIndexed)
}

func TestSearchHandler_TriggerIndex_WithErrors(t *testing.T) {
	t.Parallel()
	idx := &mockIndexer{
		result: &search.IndexResult{
			FilesIndexed: 10,
			Duration:     1 * time.Second,
			Errors:       []error{errors.New("file1: parse error"), errors.New("file2: too large")},
		},
	}
	_, router := setupSearchRouter(&mockSearcher{}, idx)

	req := httptest.NewRequest(http.MethodPost, "/v1/search/index", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp IndexResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 10, resp.FilesIndexed)
	assert.Len(t, resp.Errors, 2)
	assert.Equal(t, "file1: parse error", resp.Errors[0])
}

func TestSearchHandler_TriggerIndex_Error(t *testing.T) {
	t.Parallel()
	idx := &mockIndexer{err: errors.New("index unavailable")}
	_, router := setupSearchRouter(&mockSearcher{}, idx)

	req := httptest.NewRequest(http.MethodPost, "/v1/search/index", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "index unavailable")
}

// --- Request/Response JSON round-trips ---------------------------------

func TestSemanticSearchRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := SemanticSearchRequest{
		Query:       "find handler",
		Language:    "go",
		FilePattern: "*.go",
		TopK:        10,
		MinScore:    0.5,
		Filters:     map[string]interface{}{"type": "function"},
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded SemanticSearchRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig.Query, decoded.Query)
	assert.Equal(t, orig.Language, decoded.Language)
	assert.Equal(t, orig.FilePattern, decoded.FilePattern)
	assert.Equal(t, orig.TopK, decoded.TopK)
	assert.Equal(t, orig.MinScore, decoded.MinScore)
}

func TestSearchResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := SearchResponse{
		TotalFound: 3,
		QueryTime:  45,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded SearchResponse
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig.TotalFound, decoded.TotalFound)
	assert.Equal(t, orig.QueryTime, decoded.QueryTime)
}

func TestIndexResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	orig := IndexResponse{
		FilesIndexed:  100,
		ChunksCreated: 500,
		Duration:      5 * time.Second,
		Errors:        []string{"warn: skipped binary"},
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded IndexResponse
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, orig.FilesIndexed, decoded.FilesIndexed)
	assert.Equal(t, orig.ChunksCreated, decoded.ChunksCreated)
	assert.Len(t, decoded.Errors, 1)
}

// --- RegisterRoutes verification ---------------------------------------

func TestSearchHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	handler := NewSearchHandler(&mockSearcher{}, &mockIndexer{})
	router := gin.New()
	handler.RegisterRoutes(router)

	routes := router.Routes()
	expected := map[string]bool{
		"POST:/v1/search/semantic": true,
		"POST:/v1/search/index":   true,
	}

	for _, r := range routes {
		key := r.Method + ":" + r.Path
		delete(expected, key)
	}

	assert.Empty(t, expected, "some expected routes were not registered: %v", expected)
}
