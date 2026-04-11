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

func TestNewChromaAdapter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		config        ChromaAdapterConfig
		expectedURL   string
		expectedToken string
	}{
		{
			name: "with custom config",
			config: ChromaAdapterConfig{
				BaseURL:   "http://localhost:8000",
				AuthToken: "test-token",
				Timeout:   60 * time.Second,
			},
			expectedURL:   "http://localhost:8000",
			expectedToken: "test-token",
		},
		{
			name: "with default timeout",
			config: ChromaAdapterConfig{
				BaseURL: "http://chroma:8000",
			},
			expectedURL:   "http://chroma:8000",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adapter := NewChromaAdapter(tt.config)
			assert.NotNil(t, adapter)
			assert.Equal(t, tt.expectedURL, adapter.baseURL)
			assert.Equal(t, tt.expectedToken, adapter.authToken)
			assert.NotNil(t, adapter.httpClient)
		})
	}
}

func TestChromaAdapter_Connect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "successful connection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/heartbeat", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]int64{"nanosecond heartbeat": time.Now().UnixNano()})
			},
			expectError: false,
		},
		{
			name: "connection failure - bad status",
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{
				BaseURL: server.URL,
			})

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

func TestChromaAdapter_IsConnected(t *testing.T) {
	t.Parallel()
	adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: "http://localhost:8000"})

	// Initially not connected
	assert.False(t, adapter.IsConnected())

	// Simulate connection
	adapter.mu.Lock()
	adapter.connected = true
	adapter.mu.Unlock()

	assert.True(t, adapter.IsConnected())
}

func TestChromaAdapter_Health(t *testing.T) {
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
				_ = json.NewEncoder(w).Encode(map[string]int64{"nanosecond heartbeat": time.Now().UnixNano()})
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			err := adapter.Health(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChromaAdapter_ListCollections(t *testing.T) {
	t.Parallel()
	expectedCollections := []ChromaCollection{
		{ID: "col1", Name: "test-collection-1"},
		{ID: "col2", Name: "test-collection-2"},
	}

	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
		expected      []ChromaCollection
	}{
		{
			name: "successful list",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/api/v1/collections", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(expectedCollections)
			},
			expectError: false,
			expected:    expectedCollections,
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			collections, err := adapter.ListCollections(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, collections)
			}
		})
	}
}

func TestChromaAdapter_CreateCollection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collName      string
		metadata      map[string]interface{}
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:     "successful create without metadata",
			collName: "new-collection",
			metadata: nil,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "new-collection", body["name"])

				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(ChromaCollection{ID: "1", Name: "new-collection"})
			},
			expectError: false,
		},
		{
			name:     "successful create with metadata",
			collName: "new-collection",
			metadata: map[string]interface{}{"key": "value"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "new-collection", body["name"])
				assert.NotNil(t, body["metadata"])

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(ChromaCollection{ID: "1", Name: "new-collection"})
			},
			expectError: false,
		},
		{
			name:     "create failure",
			collName: "existing-collection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error": "collection already exists"}`))
			},
			expectError: true,
		},
		{
			name:     "decode error on response",
			collName: "test-collection",
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			collection, err := adapter.CreateCollection(context.Background(), tt.collName, tt.metadata)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, collection)
				assert.Equal(t, tt.collName, collection.Name)
			}
		})
	}
}

func TestChromaAdapter_DeleteCollection(t *testing.T) {
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
				assert.Equal(t, "/api/v1/collections/test-collection", r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			err := adapter.DeleteCollection(context.Background(), tt.collName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChromaAdapter_GetCollection(t *testing.T) {
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
				assert.Equal(t, "GET", r.Method)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(ChromaCollection{ID: "1", Name: "test-collection"})
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
			name:     "server error",
			collName: "test",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
		{
			name:     "decode error",
			collName: "test",
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			collection, err := adapter.GetCollection(context.Background(), tt.collName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, collection)
			}
		})
	}
}

func TestChromaAdapter_AddDocuments(t *testing.T) {
	t.Parallel()
	docs := []ChromaDocument{
		{ID: "doc1", Document: "test document 1", Metadata: map[string]interface{}{"key": "value"}},
		{ID: "doc2", Document: "test document 2", Embedding: []float32{0.1, 0.2, 0.3}},
	}

	tests := []struct {
		name          string
		collection    string
		docs          []ChromaDocument
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful add",
			collection: "test-collection",
			docs:       docs,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/add")
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:       "add with all embeddings",
			collection: "test-collection",
			docs: []ChromaDocument{
				{ID: "doc1", Document: "test", Embedding: []float32{0.1, 0.2}},
				{ID: "doc2", Document: "test2", Embedding: []float32{0.3, 0.4}},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotNil(t, body["embeddings"])
				w.WriteHeader(http.StatusCreated)
			},
			expectError: false,
		},
		{
			name:       "add failure",
			collection: "test-collection",
			docs:       docs,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error": "invalid documents"}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			err := adapter.AddDocuments(context.Background(), tt.collection, tt.docs)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChromaAdapter_Query(t *testing.T) {
	t.Parallel()
	queryResult := ChromaQueryResult{
		IDs:       [][]string{{"doc1", "doc2"}},
		Documents: [][]string{{"document 1", "document 2"}},
		Distances: [][]float32{{0.1, 0.2}},
	}

	tests := []struct {
		name          string
		collection    string
		embeddings    [][]float32
		nResults      int
		where         map[string]interface{}
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful query without filter",
			collection: "test-collection",
			embeddings: [][]float32{{0.1, 0.2, 0.3}},
			nResults:   10,
			where:      nil,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/query")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(queryResult)
			},
			expectError: false,
		},
		{
			name:       "successful query with filter",
			collection: "test-collection",
			embeddings: [][]float32{{0.1, 0.2, 0.3}},
			nResults:   5,
			where:      map[string]interface{}{"key": "value"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotNil(t, body["where"])
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(queryResult)
			},
			expectError: false,
		},
		{
			name:       "query failure",
			collection: "nonexistent",
			embeddings: [][]float32{{0.1, 0.2}},
			nResults:   10,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error": "collection not found"}`))
			},
			expectError: true,
		},
		{
			name:       "decode error",
			collection: "test",
			embeddings: [][]float32{{0.1}},
			nResults:   10,
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			result, err := adapter.Query(context.Background(), tt.collection, tt.embeddings, tt.nResults, tt.where)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestChromaAdapter_DeleteDocuments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collection    string
		ids           []string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful delete",
			collection: "test-collection",
			ids:        []string{"doc1", "doc2"},
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
			ids:        []string{"nonexistent"},
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			err := adapter.DeleteDocuments(context.Background(), tt.collection, tt.ids)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChromaAdapter_UpdateDocuments(t *testing.T) {
	t.Parallel()
	docs := []ChromaDocument{
		{ID: "doc1", Document: "updated document 1"},
		{ID: "doc2", Document: "updated document 2", Embedding: []float32{0.1, 0.2}},
	}

	tests := []struct {
		name          string
		collection    string
		docs          []ChromaDocument
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful update",
			collection: "test-collection",
			docs:       docs,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/update")
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:       "update with all embeddings",
			collection: "test-collection",
			docs: []ChromaDocument{
				{ID: "doc1", Document: "test", Embedding: []float32{0.1, 0.2}},
				{ID: "doc2", Document: "test2", Embedding: []float32{0.3, 0.4}},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotNil(t, body["embeddings"])
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:       "update failure",
			collection: "test-collection",
			docs:       docs,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error": "invalid"}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			err := adapter.UpdateDocuments(context.Background(), tt.collection, tt.docs)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChromaAdapter_Count(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		collection    string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expected      int64
		expectError   bool
	}{
		{
			name:       "successful count",
			collection: "test-collection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/count")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(42)
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

			adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: server.URL})
			count, err := adapter.Count(context.Background(), tt.collection)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, count)
			}
		})
	}
}

func TestChromaAdapter_Close(t *testing.T) {
	t.Parallel()
	adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: "http://localhost:8000"})
	adapter.connected = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.IsConnected())
}

func TestChromaAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	adapter := NewChromaAdapter(ChromaAdapterConfig{BaseURL: "http://localhost:8000"})
	tools := adapter.GetMCPTools()

	assert.NotEmpty(t, tools)

	// Verify expected tools exist
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	expectedTools := []string{
		"chroma_list_collections",
		"chroma_create_collection",
		"chroma_delete_collection",
		"chroma_add_documents",
		"chroma_query",
		"chroma_delete_documents",
		"chroma_count",
	}

	for _, expected := range expectedTools {
		assert.True(t, toolNames[expected], "Expected tool %s not found", expected)
	}
}

func TestChromaAdapter_doRequest_WithAuth(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer test-token", authHeader)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewChromaAdapter(ChromaAdapterConfig{
		BaseURL:   server.URL,
		AuthToken: "test-token",
	})

	err := adapter.Health(context.Background())
	require.NoError(t, err)
}

func TestChromaAdapter_doRequest_MarshalError(t *testing.T) {
	t.Parallel()
	adapter := NewChromaAdapter(ChromaAdapterConfig{
		BaseURL: "http://localhost:8000",
	})

	// Test with invalid body that can't be marshaled (function type)
	// Using CreateCollection with a channel as metadata value
	ctx := context.Background()
	_, err := adapter.CreateCollection(ctx, "test", map[string]interface{}{
		"invalid": make(chan int), // channels can't be marshaled to JSON
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "marshal")
}
