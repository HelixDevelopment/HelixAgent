// Package servers provides MCP server adapters for various services.
package servers

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

func TestNewQdrantAdapter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		config      QdrantAdapterConfig
		expectedURL string
		expectedKey string
	}{
		{
			name: "with custom config",
			config: QdrantAdapterConfig{
				BaseURL: "http://localhost:6333",
				APIKey:  "test-key",
				Timeout: 60 * time.Second,
			},
			expectedURL: "http://localhost:6333",
			expectedKey: "test-key",
		},
		{
			name: "with default timeout",
			config: QdrantAdapterConfig{
				BaseURL: "http://qdrant:6333",
			},
			expectedURL: "http://qdrant:6333",
			expectedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adapter := NewQdrantAdapter(tt.config)
			assert.NotNil(t, adapter)
			assert.Equal(t, tt.expectedURL, adapter.baseURL)
			assert.Equal(t, tt.expectedKey, adapter.apiKey)
			assert.NotNil(t, adapter.httpClient)
		})
	}
}

func TestQdrantAdapter_Connect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "successful connection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/readyz", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name: "connection failure",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			err := adapter.Connect(context.Background())
			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, adapter.IsConnected())
			} else {
				assert.NoError(t, err)
				assert.True(t, adapter.IsConnected())
			}
		})
	}
}

func TestQdrantAdapter_Health(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "healthy",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name: "unhealthy",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			err := adapter.Health(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQdrantAdapter_ListCollections(t *testing.T) {
	t.Parallel()
	response := QdrantCollectionsResponse{
		Result: struct {
			Collections []QdrantCollection `json:"collections"`
		}{
			Collections: []QdrantCollection{
				{Name: "collection1", Status: "green"},
				{Name: "collection2", Status: "green"},
			},
		},
		Status: "ok",
		Time:   0.001,
	}

	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
		expectedLen   int
	}{
		{
			name: "successful list",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/collections", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(response)
			},
			expectError: false,
			expectedLen: 2,
		},
		{
			name: "decode error",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid json"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			collections, err := adapter.ListCollections(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, collections, tt.expectedLen)
			}
		})
	}
}

func TestQdrantAdapter_CreateCollection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collName      string
		vectorSize    uint64
		distance      string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful create with cosine",
			collName:   "new-collection",
			vectorSize: 1536,
			distance:   "Cosine",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/collections/new-collection", r.URL.Path)
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				vectors := body["vectors"].(map[string]interface{})
				assert.Equal(t, float64(1536), vectors["size"])
				assert.Equal(t, "Cosine", vectors["distance"])
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": true, "status": "ok"})
			},
			expectError: false,
		},
		{
			name:       "successful create with default distance",
			collName:   "new-collection",
			vectorSize: 768,
			distance:   "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				vectors := body["vectors"].(map[string]interface{})
				assert.Equal(t, "Cosine", vectors["distance"])
				w.WriteHeader(http.StatusCreated)
			},
			expectError: false,
		},
		{
			name:       "create failure",
			collName:   "existing",
			vectorSize: 1536,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"status":{"error":"collection already exists"}}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			err := adapter.CreateCollection(context.Background(), tt.collName, tt.vectorSize, tt.distance)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQdrantAdapter_DeleteCollection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collName      string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:     "successful delete",
			collName: "test-collection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "DELETE", r.Method)
				assert.Equal(t, "/collections/test-collection", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:     "delete failure",
			collName: "nonexistent",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			err := adapter.DeleteCollection(context.Background(), tt.collName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQdrantAdapter_CollectionExists(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collName      string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expected      bool
		expectError   bool
	}{
		{
			name:     "collection exists",
			collName: "test-collection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expected:    true,
			expectError: false,
		},
		{
			name:     "collection does not exist",
			collName: "nonexistent",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expected:    false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			exists, err := adapter.CollectionExists(context.Background(), tt.collName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, exists)
			}
		})
	}
}

func TestQdrantAdapter_GetCollectionInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collName      string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:     "successful get",
			collName: "test-collection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"result": QdrantCollection{Name: "test-collection", Status: "green"},
					"status": "ok",
				})
			},
			expectError: false,
		},
		{
			name:     "collection not found",
			collName: "nonexistent",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
		{
			name:     "decode error",
			collName: "test",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			info, err := adapter.GetCollectionInfo(context.Background(), tt.collName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, info)
			}
		})
	}
}

func TestQdrantAdapter_UpsertPoints(t *testing.T) {
	t.Parallel()
	points := []QdrantPoint{
		{ID: uint64(1), Vector: []float32{0.1, 0.2, 0.3}, Payload: map[string]interface{}{"key": "value"}},
		{ID: "point-2", Vector: []float32{0.4, 0.5, 0.6}},
	}

	tests := []struct {
		name          string
		collection    string
		points        []QdrantPoint
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful upsert",
			collection: "test-collection",
			points:     points,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/collections/test-collection/points", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]string{"status": "completed"}, "status": "ok"})
			},
			expectError: false,
		},
		{
			name:       "upsert failure",
			collection: "test-collection",
			points:     points,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":{"error":"invalid points"}}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			err := adapter.UpsertPoints(context.Background(), tt.collection, tt.points)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQdrantAdapter_DeletePoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collection    string
		ids           []interface{}
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful delete",
			collection: "test-collection",
			ids:        []interface{}{uint64(1), "point-2"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/delete")
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:       "delete failure",
			collection: "test-collection",
			ids:        []interface{}{uint64(1)},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			err := adapter.DeletePoints(context.Background(), tt.collection, tt.ids)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQdrantAdapter_Search(t *testing.T) {
	t.Parallel()
	searchResponse := QdrantSearchResponse{
		Result: []QdrantSearchResult{
			{ID: uint64(1), Score: 0.95, Payload: map[string]interface{}{"key": "value"}},
			{ID: "point-2", Score: 0.85},
		},
		Status: "ok",
		Time:   0.001,
	}

	tests := []struct {
		name          string
		collection    string
		vector        []float32
		limit         int
		filter        map[string]interface{}
		withPayload   bool
		withVector    bool
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:        "successful search without filter",
			collection:  "test-collection",
			vector:      []float32{0.1, 0.2, 0.3},
			limit:       10,
			filter:      nil,
			withPayload: true,
			withVector:  false,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/search")
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotNil(t, body["vector"])
				assert.Equal(t, float64(10), body["limit"])
				assert.True(t, body["with_payload"].(bool))
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(searchResponse)
			},
			expectError: false,
		},
		{
			name:        "successful search with filter",
			collection:  "test-collection",
			vector:      []float32{0.1, 0.2, 0.3},
			limit:       5,
			filter:      map[string]interface{}{"must": []map[string]interface{}{{"key": "category", "match": map[string]string{"value": "test"}}}},
			withPayload: true,
			withVector:  true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotNil(t, body["filter"])
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(searchResponse)
			},
			expectError: false,
		},
		{
			name:       "search failure",
			collection: "nonexistent",
			vector:     []float32{0.1, 0.2},
			limit:      10,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"status":{"error":"collection not found"}}`))
			},
			expectError: true,
		},
		{
			name:       "decode error",
			collection: "test",
			vector:     []float32{0.1},
			limit:      10,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			results, err := adapter.Search(context.Background(), tt.collection, tt.vector, tt.limit, tt.filter, tt.withPayload, tt.withVector)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, results)
			}
		})
	}
}

func TestQdrantAdapter_SearchBatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		collection string
		searches   []struct {
			Vector      []float32
			Limit       int
			Filter      map[string]interface{}
			WithPayload bool
		}
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful batch search",
			collection: "test-collection",
			searches: []struct {
				Vector      []float32
				Limit       int
				Filter      map[string]interface{}
				WithPayload bool
			}{
				{Vector: []float32{0.1, 0.2}, Limit: 5, WithPayload: true},
				{Vector: []float32{0.3, 0.4}, Limit: 10, Filter: map[string]interface{}{"key": "value"}, WithPayload: false},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/batch")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"result": [][]QdrantSearchResult{
						{{ID: uint64(1), Score: 0.9}},
						{{ID: uint64(2), Score: 0.8}},
					},
				})
			},
			expectError: false,
		},
		{
			name:       "batch search failure",
			collection: "test-collection",
			searches: []struct {
				Vector      []float32
				Limit       int
				Filter      map[string]interface{}
				WithPayload bool
			}{
				{Vector: []float32{0.1, 0.2}, Limit: 5},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":{"error":"invalid request"}}`))
			},
			expectError: true,
		},
		{
			name:       "decode error",
			collection: "test",
			searches: []struct {
				Vector      []float32
				Limit       int
				Filter      map[string]interface{}
				WithPayload bool
			}{
				{Vector: []float32{0.1}, Limit: 5},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			results, err := adapter.SearchBatch(context.Background(), tt.collection, tt.searches)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, results)
			}
		})
	}
}

func TestQdrantAdapter_GetPoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collection    string
		ids           []interface{}
		withPayload   bool
		withVector    bool
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:        "successful get",
			collection:  "test-collection",
			ids:         []interface{}{uint64(1), "point-2"},
			withPayload: true,
			withVector:  true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"result": []QdrantPoint{
						{ID: uint64(1), Vector: []float32{0.1, 0.2}, Payload: map[string]interface{}{"key": "value"}},
					},
				})
			},
			expectError: false,
		},
		{
			name:       "get failure",
			collection: "test-collection",
			ids:        []interface{}{uint64(999)},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":{"error":"invalid ids"}}`))
			},
			expectError: true,
		},
		{
			name:       "decode error",
			collection: "test",
			ids:        []interface{}{uint64(1)},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			points, err := adapter.GetPoints(context.Background(), tt.collection, tt.ids, tt.withPayload, tt.withVector)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, points)
			}
		})
	}
}

func TestQdrantAdapter_CountPoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collection    string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expected      uint64
		expectError   bool
	}{
		{
			name:       "successful count",
			collection: "test-collection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/count")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"result": map[string]uint64{"count": 42},
				})
			},
			expected:    42,
			expectError: false,
		},
		{
			name:       "count failure",
			collection: "nonexistent",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
		{
			name:       "decode error",
			collection: "test",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			count, err := adapter.CountPoints(context.Background(), tt.collection)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, count)
			}
		})
	}
}

func TestQdrantAdapter_Scroll(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collection    string
		offset        interface{}
		limit         int
		withPayload   bool
		withVector    bool
		filter        map[string]interface{}
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:        "successful scroll without offset",
			collection:  "test-collection",
			offset:      nil,
			limit:       100,
			withPayload: true,
			withVector:  false,
			filter:      nil,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/scroll")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"result": map[string]interface{}{
						"points":           []QdrantPoint{{ID: uint64(1), Vector: []float32{0.1}}},
						"next_page_offset": uint64(2),
					},
				})
			},
			expectError: false,
		},
		{
			name:        "successful scroll with offset and filter",
			collection:  "test-collection",
			offset:      uint64(10),
			limit:       50,
			withPayload: true,
			withVector:  true,
			filter:      map[string]interface{}{"key": "value"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotNil(t, body["offset"])
				assert.NotNil(t, body["filter"])
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"result": map[string]interface{}{
						"points":           []QdrantPoint{},
						"next_page_offset": nil,
					},
				})
			},
			expectError: false,
		},
		{
			name:       "scroll failure",
			collection: "nonexistent",
			limit:      100,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"status":{"error":"collection not found"}}`))
			},
			expectError: true,
		},
		{
			name:       "decode error",
			collection: "test",
			limit:      10,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: server.URL})
			points, nextOffset, err := adapter.Scroll(context.Background(), tt.collection, tt.offset, tt.limit, tt.withPayload, tt.withVector, tt.filter)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, points)
				_ = nextOffset // Can be nil
			}
		})
	}
}

func TestQdrantAdapter_Close(t *testing.T) {
	t.Parallel()
	adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: "http://localhost:6333"})
	adapter.connected = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.IsConnected())
}

func TestQdrantAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: "http://localhost:6333"})
	tools := adapter.GetMCPTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	expectedTools := []string{
		"qdrant_list_collections",
		"qdrant_create_collection",
		"qdrant_delete_collection",
		"qdrant_upsert_points",
		"qdrant_search",
		"qdrant_delete_points",
		"qdrant_count_points",
	}

	for _, expected := range expectedTools {
		assert.True(t, toolNames[expected], "Expected tool %s not found", expected)
	}
}

func TestQdrantAdapter_doRequest_WithAPIKey(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("api-key")
		assert.Equal(t, "test-api-key", apiKey)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewQdrantAdapter(QdrantAdapterConfig{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
	})

	err := adapter.Health(context.Background())
	require.NoError(t, err)
}

func TestQdrantAdapter_IsConnected(t *testing.T) {
	t.Parallel()
	adapter := NewQdrantAdapter(QdrantAdapterConfig{BaseURL: "http://localhost:6333"})

	// Initially not connected
	assert.False(t, adapter.IsConnected())

	// Simulate connection
	adapter.mu.Lock()
	adapter.connected = true
	adapter.mu.Unlock()

	assert.True(t, adapter.IsConnected())
}
