package store

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/search/types"
)

func TestNewChromaStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		host       string
		port       int
		collection string
		wantHost   string
		wantPort   int
	}{
		{"defaults", "", 0, "col", "localhost", 8000},
		{"custom host", "chroma.local", 0, "col", "chroma.local", 8000},
		{"custom port", "", 9000, "col", "localhost", 9000},
		{"full custom", "db.host", 9999, "my_col", "db.host", 9999},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, err := NewChromaStore(tc.host, tc.port, tc.collection)
			require.NoError(t, err)
			require.NotNil(t, s)
			assert.Equal(t, tc.wantHost, s.host)
			assert.Equal(t, tc.wantPort, s.port)
			assert.Equal(t, tc.collection, s.collection)
			assert.NotNil(t, s.httpClient)
		})
	}
}

func TestChromaStore_BaseURL(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{host: "myhost", port: 8001}
	assert.Equal(t, "http://myhost:8001/api/v1", s.baseURL())
}

func TestChromaStore_BuildFilter_Empty(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{}
	result := s.buildFilter(nil)
	assert.Nil(t, result)

	result = s.buildFilter(map[string]interface{}{})
	assert.Nil(t, result)
}

func TestChromaStore_BuildFilter_Single(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{}
	result := s.buildFilter(map[string]interface{}{
		"language": "go",
	})

	require.NotNil(t, result)
	// Single filter should not wrap in $and
	eq, ok := result["language"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "go", eq["$eq"])
}

func TestChromaStore_BuildFilter_Multiple(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{}
	result := s.buildFilter(map[string]interface{}{
		"language": "go",
		"type":     "function",
	})

	require.NotNil(t, result)
	// Multiple filters should use $and
	andConditions, ok := result["$and"]
	require.True(t, ok)
	conditions := andConditions.([]map[string]interface{})
	assert.Len(t, conditions, 2)
}

func TestChromaStore_CreateCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/collections")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test_col", body["name"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	err := s.CreateCollection(context.Background(), "test_col", 128)
	assert.NoError(t, err)
}

func TestChromaStore_CreateCollection_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	err := s.CreateCollection(context.Background(), "test_col", 128)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create collection")
}

func TestChromaStore_DeleteCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/collections/test_col")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	err := s.DeleteCollection(context.Background(), "test_col")
	assert.NoError(t, err)
}

func TestChromaStore_DeleteCollection_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	err := s.DeleteCollection(context.Background(), "test_col")
	assert.Error(t, err)
}

func TestChromaStore_Upsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/upsert")

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		ids := body["ids"].([]interface{})
		assert.Len(t, ids, 2)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	docs := []types.Document{
		{ID: "d1", Vector: []float32{0.1, 0.2}, Content: "doc1", Metadata: map[string]interface{}{"k": "v"}},
		{ID: "d2", Vector: []float32{0.3, 0.4}, Content: "doc2", Metadata: map[string]interface{}{"k": "v2"}},
	}

	err := s.Upsert(context.Background(), "col", docs)
	assert.NoError(t, err)
}

func TestChromaStore_Upsert_EmptyDocs(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{}
	err := s.Upsert(context.Background(), "col", []types.Document{})
	assert.NoError(t, err)
}

func TestChromaStore_Upsert_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	docs := []types.Document{{ID: "d1", Vector: []float32{0.1}, Content: "c", Metadata: map[string]interface{}{}}}

	err := s.Upsert(context.Background(), "col", docs)
	assert.Error(t, err)
}

func TestChromaStore_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/delete")

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		ids := body["ids"].([]interface{})
		assert.Len(t, ids, 2)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	err := s.Delete(context.Background(), "col", []string{"id1", "id2"})
	assert.NoError(t, err)
}

func TestChromaStore_Delete_EmptyIDs(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{}
	err := s.Delete(context.Background(), "col", []string{})
	assert.NoError(t, err)
}

func TestChromaStore_Search(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/query")

		resp := map[string]interface{}{
			"ids":       [][]string{{"doc1", "doc2"}},
			"distances": [][]float64{{0.1, 0.3}},
			"documents": [][]string{{"content1", "content2"}},
			"metadatas": [][]map[string]interface{}{
				{{"language": "go"}, {"language": "python"}},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	results, err := s.Search(context.Background(), "col", []float32{0.1, 0.2}, types.SearchOptions{TopK: 5})

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "doc1", results[0].ID)
	assert.Equal(t, "content1", results[0].Content)
	assert.InDelta(t, float32(0.9), results[0].Score, 0.01) // 1 - 0.1
	assert.InDelta(t, float32(0.1), results[0].Distance, 0.01)
}

func TestChromaStore_Search_WithMinScore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"ids":       [][]string{{"doc1", "doc2"}},
			"distances": [][]float64{{0.1, 0.8}},
			"documents": [][]string{{"good", "bad"}},
			"metadatas": [][]map[string]interface{}{{{"k": "v"}, {"k": "v"}}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	results, err := s.Search(context.Background(), "col", []float32{0.1}, types.SearchOptions{
		TopK:     10,
		MinScore: 0.5,
	})

	require.NoError(t, err)
	// doc2 has distance 0.8, score = 1 - 0.8 = 0.2 < 0.5, should be filtered
	assert.Len(t, results, 1)
	assert.Equal(t, "doc1", results[0].ID)
}

func TestChromaStore_Search_DefaultTopK(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, float64(10), body["n_results"])

		resp := map[string]interface{}{
			"ids":       [][]string{},
			"distances": [][]float64{},
			"documents": [][]string{},
			"metadatas": [][]map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	_, err := s.Search(context.Background(), "col", []float32{0.1}, types.SearchOptions{TopK: 0})
	assert.NoError(t, err)
}

func TestChromaStore_Search_WithFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Contains(t, body, "where")

		resp := map[string]interface{}{
			"ids":       [][]string{},
			"distances": [][]float64{},
			"documents": [][]string{},
			"metadatas": [][]map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	_, err := s.Search(context.Background(), "col", []float32{0.1}, types.SearchOptions{
		TopK:    5,
		Filters: map[string]interface{}{"language": "go"},
	})
	assert.NoError(t, err)
}

func TestChromaStore_Search_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	results, err := s.Search(context.Background(), "col", []float32{0.1}, types.SearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query failed")
	assert.Nil(t, results)
}

func TestChromaStore_SearchByText(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{}
	results, err := s.SearchByText(context.Background(), "col", "query", types.SearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "text search not supported")
	assert.Nil(t, results)
}

func TestChromaStore_GetCollectionStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/count")

		resp := map[string]interface{}{"count": 42}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	stats, err := s.GetCollectionStats(context.Background(), "test_col")

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, "test_col", stats.Name)
	assert.Equal(t, int64(42), stats.Count)
}

func TestChromaStore_GetCollectionStats_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	s := newTestChromaStore(t, server)
	stats, err := s.GetCollectionStats(context.Background(), "missing")

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, "missing", stats.Name)
}

func TestChromaStore_NewRequest_NilBody(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{host: "localhost", port: 8000}
	req, err := s.newRequest(context.Background(), "GET", "http://localhost:8000/test", nil)

	require.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
	assert.Empty(t, req.Header.Get("Content-Type"))
}

func TestChromaStore_NewRequest_WithBody(t *testing.T) {
	t.Parallel()

	s := &ChromaStore{host: "localhost", port: 8000}
	body := map[string]interface{}{"key": "value"}
	req, err := s.newRequest(context.Background(), "POST", "http://localhost:8000/test", body)

	require.NoError(t, err)
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
}

func TestChromaStore_ImplementsVectorStore(t *testing.T) {
	t.Parallel()

	var _ types.VectorStore = (*ChromaStore)(nil)
}

// newTestChromaStore creates a ChromaStore pointing to a test server
func newTestChromaStore(t *testing.T, server *httptest.Server) *ChromaStore {
	t.Helper()

	s, err := NewChromaStore("localhost", 8000, "test_col")
	require.NoError(t, err)

	// Use a URL-rewriting transport to redirect all requests to the test server
	serverHost := server.URL[len("http://"):]
	s.httpClient = &http.Client{
		Transport: &urlRewriteTransport{
			base:       http.DefaultTransport,
			targetHost: serverHost,
		},
	}

	return s
}

// urlRewriteTransport rewrites all request URLs to point at a test server
type urlRewriteTransport struct {
	base       http.RoundTripper
	targetHost string
}

func (t *urlRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Host = t.targetHost
	req.URL.Scheme = "http"
	return t.base.RoundTrip(req)
}
