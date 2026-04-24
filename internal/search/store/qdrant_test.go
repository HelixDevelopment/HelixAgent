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

func TestNewQdrantStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		host       string
		port       int
		collection string
		wantHost   string
		wantPort   int
	}{
		{"defaults", "", 0, "col", "localhost", 6333},
		{"custom host", "qdrant.local", 0, "col", "qdrant.local", 6333},
		{"custom port", "", 7777, "col", "localhost", 7777},
		{"full custom", "db.host", 9999, "my_col", "db.host", 9999},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, err := NewQdrantStore(tc.host, tc.port, tc.collection)
			require.NoError(t, err)
			require.NotNil(t, s)
			assert.Equal(t, tc.wantHost, s.host)
			assert.Equal(t, tc.wantPort, s.port)
			assert.Equal(t, tc.collection, s.collection)
			assert.NotNil(t, s.httpClient)
		})
	}
}

func TestQdrantStore_BaseURL(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{host: "myhost", port: 6334}
	assert.Equal(t, "http://myhost:6334", s.baseURL())
}

func TestQdrantStore_BuildFilter_Empty(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{}
	result := s.buildFilter(nil)
	assert.Nil(t, result)

	result = s.buildFilter(map[string]interface{}{})
	assert.Nil(t, result)
}

func TestQdrantStore_BuildFilter_Single(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{}
	result := s.buildFilter(map[string]interface{}{
		"language": "go",
	})

	require.NotNil(t, result)
	must := result["must"].([]map[string]interface{})
	require.Len(t, must, 1)
	assert.Equal(t, "language", must[0]["key"])
	matchVal := must[0]["match"].(map[string]interface{})
	assert.Equal(t, "go", matchVal["value"])
}

func TestQdrantStore_BuildFilter_Multiple(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{}
	result := s.buildFilter(map[string]interface{}{
		"language": "go",
		"type":     "function",
	})

	require.NotNil(t, result)
	must := result["must"].([]map[string]interface{})
	assert.Len(t, must, 2)
}

func TestQdrantStore_CreateCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/collections/test_col")

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		vectors := body["vectors"].(map[string]interface{})
		assert.Equal(t, float64(128), vectors["size"])
		assert.Equal(t, "Cosine", vectors["distance"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	err := s.CreateCollection(context.Background(), "test_col", 128)
	assert.NoError(t, err)
}

func TestQdrantStore_CreateCollection_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	err := s.CreateCollection(context.Background(), "test_col", 128)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create collection")
}

func TestQdrantStore_DeleteCollection(t *testing.T) {
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

	s := newTestQdrantStore(t, server)
	err := s.DeleteCollection(context.Background(), "test_col")
	assert.NoError(t, err)
}

func TestQdrantStore_DeleteCollection_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	err := s.DeleteCollection(context.Background(), "test_col")
	assert.Error(t, err)
}

func TestQdrantStore_Upsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/points")
		assert.Contains(t, r.URL.RawQuery, "wait=true")

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		points := body["points"].([]interface{})
		assert.Len(t, points, 2)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	docs := []types.Document{
		{ID: "d1", Vector: []float32{0.1, 0.2}, Content: "doc1", Metadata: map[string]interface{}{"k": "v"}},
		{ID: "d2", Vector: []float32{0.3, 0.4}, Content: "doc2", Metadata: map[string]interface{}{"k": "v2"}},
	}

	err := s.Upsert(context.Background(), "col", docs)
	assert.NoError(t, err)
}

func TestQdrantStore_Upsert_EmptyDocs(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{}
	err := s.Upsert(context.Background(), "col", []types.Document{})
	assert.NoError(t, err)
}

func TestQdrantStore_Upsert_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	docs := []types.Document{{ID: "d1", Vector: []float32{0.1}, Content: "c", Metadata: map[string]interface{}{}}}

	err := s.Upsert(context.Background(), "col", docs)
	assert.Error(t, err)
}

func TestQdrantStore_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/delete")

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		points := body["points"].([]interface{})
		assert.Len(t, points, 2)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	err := s.Delete(context.Background(), "col", []string{"id1", "id2"})
	assert.NoError(t, err)
}

func TestQdrantStore_Delete_EmptyIDs(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{}
	err := s.Delete(context.Background(), "col", []string{})
	assert.NoError(t, err)
}

func TestQdrantStore_Delete_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	err := s.Delete(context.Background(), "col", []string{"id1"})
	assert.Error(t, err)
}

func TestQdrantStore_Search(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/search")

		resp := map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"id":    "doc1",
					"score": 0.95,
					"payload": map[string]interface{}{
						"language": "go",
						"content":  "func main() {}",
					},
				},
				{
					"id":    "doc2",
					"score": 0.80,
					"payload": map[string]interface{}{
						"language": "python",
						"content":  "def main():",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	results, err := s.Search(context.Background(), "col", []float32{0.1, 0.2}, types.SearchOptions{TopK: 5})

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "doc1", results[0].ID)
	assert.InDelta(t, float32(0.95), results[0].Score, 0.01)
	assert.InDelta(t, float32(0.05), results[0].Distance, 0.01) // 1 - 0.95
	assert.Equal(t, "func main() {}", results[0].Content)
	assert.Equal(t, "go", results[0].Metadata["language"])
}

func TestQdrantStore_Search_WithMinScore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"result": []map[string]interface{}{
				{"id": "good", "score": 0.9, "payload": map[string]interface{}{"content": "good"}},
				{"id": "bad", "score": 0.3, "payload": map[string]interface{}{"content": "bad"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	results, err := s.Search(context.Background(), "col", []float32{0.1}, types.SearchOptions{
		TopK:     10,
		MinScore: 0.5,
	})

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "good", results[0].ID)
}

func TestQdrantStore_Search_DefaultTopK(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, float64(10), body["limit"])

		resp := map[string]interface{}{"result": []map[string]interface{}{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	_, err := s.Search(context.Background(), "col", []float32{0.1}, types.SearchOptions{TopK: 0})
	assert.NoError(t, err)
}

func TestQdrantStore_Search_WithFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		assert.Contains(t, body, "filter")

		resp := map[string]interface{}{"result": []map[string]interface{}{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	_, err := s.Search(context.Background(), "col", []float32{0.1}, types.SearchOptions{
		TopK:    5,
		Filters: map[string]interface{}{"language": "go"},
	})
	assert.NoError(t, err)
}

func TestQdrantStore_Search_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	results, err := s.Search(context.Background(), "col", []float32{0.1}, types.SearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "search failed")
	assert.Nil(t, results)
}

func TestQdrantStore_SearchByText(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{}
	results, err := s.SearchByText(context.Background(), "col", "query", types.SearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "text search not supported")
	assert.Nil(t, results)
}

func TestQdrantStore_GetCollectionStats(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)

		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"points_count": 500,
				"config": map[string]interface{}{
					"params": map[string]interface{}{
						"size": 768,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	stats, err := s.GetCollectionStats(context.Background(), "test_col")

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, "test_col", stats.Name)
	assert.Equal(t, int64(500), stats.Count)
	assert.Equal(t, 768, stats.Dimensions)
}

func TestQdrantStore_GetCollectionStats_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	s := newTestQdrantStore(t, server)
	stats, err := s.GetCollectionStats(context.Background(), "missing")

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, "missing", stats.Name)
}

func TestQdrantStore_NewRequest_NilBody(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{host: "localhost", port: 6333}
	req, err := s.newRequest(context.Background(), "GET", "http://localhost:6333/test", nil)

	require.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
	assert.Empty(t, req.Header.Get("Content-Type"))
	assert.Empty(t, req.Header.Get("api-key"))
}

func TestQdrantStore_NewRequest_WithBody(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{host: "localhost", port: 6333}
	body := map[string]interface{}{"key": "value"}
	req, err := s.newRequest(context.Background(), "POST", "http://localhost:6333/test", body)

	require.NoError(t, err)
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestQdrantStore_NewRequest_WithAPIKey(t *testing.T) {
	t.Parallel()

	s := &QdrantStore{host: "localhost", port: 6333, apiKey: "my-secret-key"}
	req, err := s.newRequest(context.Background(), "GET", "http://localhost:6333/test", nil)

	require.NoError(t, err)
	assert.Equal(t, "my-secret-key", req.Header.Get("api-key"))
}

func TestQdrantStore_ImplementsVectorStore(t *testing.T) {
	t.Parallel()

	var _ types.VectorStore = (*QdrantStore)(nil)
}

// newTestQdrantStore creates a QdrantStore pointing to a test server
func newTestQdrantStore(t *testing.T, server *httptest.Server) *QdrantStore {
	t.Helper()

	s, err := NewQdrantStore("localhost", 6333, "test_col")
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
