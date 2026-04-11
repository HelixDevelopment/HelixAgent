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

func TestNewWeaviateAdapter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		config      WeaviateAdapterConfig
		expectedURL string
		expectedKey string
	}{
		{
			name: "with custom config",
			config: WeaviateAdapterConfig{
				BaseURL: "http://localhost:8080",
				APIKey:  "test-key",
				Timeout: 60 * time.Second,
			},
			expectedURL: "http://localhost:8080",
			expectedKey: "test-key",
		},
		{
			name: "with default timeout",
			config: WeaviateAdapterConfig{
				BaseURL: "http://weaviate:8080",
			},
			expectedURL: "http://weaviate:8080",
			expectedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adapter := NewWeaviateAdapter(tt.config)
			assert.NotNil(t, adapter)
			assert.Equal(t, tt.expectedURL, adapter.baseURL)
			assert.Equal(t, tt.expectedKey, adapter.apiKey)
			assert.NotNil(t, adapter.httpClient)
		})
	}
}

func TestWeaviateAdapter_Connect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "successful connection",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/.well-known/ready", r.URL.Path)
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
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

func TestWeaviateAdapter_Health(t *testing.T) {
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			err := adapter.Health(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWeaviateAdapter_GetSchema(t *testing.T) {
	t.Parallel()
	schema := WeaviateSchema{
		Classes: []WeaviateClass{
			{Class: "Article", Description: "News articles"},
			{Class: "Author", Description: "Article authors"},
		},
	}

	tests := []struct {
		name          string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "successful get schema",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/v1/schema", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(schema)
			},
			expectError: false,
		},
		{
			name: "get schema failure",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			result, err := adapter.GetSchema(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result.Classes, 2)
			}
		})
	}
}

func TestWeaviateAdapter_ListClasses(t *testing.T) {
	t.Parallel()
	schema := WeaviateSchema{
		Classes: []WeaviateClass{
			{Class: "Article"},
			{Class: "Author"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(schema)
	}))
	defer server.Close()

	adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
	classes, err := adapter.ListClasses(context.Background())
	assert.NoError(t, err)
	assert.Len(t, classes, 2)
}

func TestWeaviateAdapter_CreateClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		class         *WeaviateClass
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "successful create",
			class: &WeaviateClass{
				Class:       "NewClass",
				Description: "A new class",
				Properties: []WeaviateProperty{
					{Name: "title", DataType: []string{"text"}},
				},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v1/schema", r.URL.Path)
				var class WeaviateClass
				_ = json.NewDecoder(r.Body).Decode(&class)
				assert.Equal(t, "NewClass", class.Class)
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:  "create failure - already exists",
			class: &WeaviateClass{Class: "ExistingClass"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":[{"message":"class already exists"}]}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			err := adapter.CreateClass(context.Background(), tt.class)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWeaviateAdapter_DeleteClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		className     string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:      "successful delete",
			className: "TestClass",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "DELETE", r.Method)
				assert.Equal(t, "/v1/schema/TestClass", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name:      "delete failure - not found",
			className: "NonexistentClass",
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			err := adapter.DeleteClass(context.Background(), tt.className)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWeaviateAdapter_GetClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		className     string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:      "successful get",
			className: "Article",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(WeaviateClass{Class: "Article", Description: "Articles"})
			},
			expectError: false,
		},
		{
			name:      "class not found",
			className: "Nonexistent",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
		{
			name:      "server error",
			className: "Test",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
		{
			name:      "decode error",
			className: "Test",
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			class, err := adapter.GetClass(context.Background(), tt.className)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, class)
			}
		})
	}
}

func TestWeaviateAdapter_CreateObject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		obj           *WeaviateObject
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "successful create",
			obj: &WeaviateObject{
				Class:      "Article",
				Properties: map[string]interface{}{"title": "Test Article"},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v1/objects", r.URL.Path)
				var obj WeaviateObject
				_ = json.NewDecoder(r.Body).Decode(&obj)
				obj.ID = "generated-uuid"
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(obj)
			},
			expectError: false,
		},
		{
			name: "create with vector",
			obj: &WeaviateObject{
				Class:      "Article",
				Properties: map[string]interface{}{"title": "Test"},
				Vector:     []float32{0.1, 0.2, 0.3},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var obj WeaviateObject
				_ = json.NewDecoder(r.Body).Decode(&obj)
				assert.NotNil(t, obj.Vector)
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(obj)
			},
			expectError: false,
		},
		{
			name: "create failure",
			obj:  &WeaviateObject{Class: "Invalid"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":[{"message":"invalid object"}]}`))
			},
			expectError: true,
		},
		{
			name: "decode error",
			obj:  &WeaviateObject{Class: "Test"},
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			created, err := adapter.CreateObject(context.Background(), tt.obj)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, created)
			}
		})
	}
}

func TestWeaviateAdapter_BatchCreateObjects(t *testing.T) {
	t.Parallel()
	objects := []WeaviateObject{
		{Class: "Article", Properties: map[string]interface{}{"title": "Article 1"}},
		{Class: "Article", Properties: map[string]interface{}{"title": "Article 2"}},
	}

	tests := []struct {
		name          string
		objects       []WeaviateObject
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:    "successful batch create",
			objects: objects,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v1/batch/objects", r.URL.Path)
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.NotNil(t, body["objects"])
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{
					{"id": "uuid-1", "result": map[string]string{"status": "SUCCESS"}},
					{"id": "uuid-2", "result": map[string]string{"status": "SUCCESS"}},
				})
			},
			expectError: false,
		},
		{
			name:    "batch create failure",
			objects: objects,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":[{"message":"batch failed"}]}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			err := adapter.BatchCreateObjects(context.Background(), tt.objects)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWeaviateAdapter_GetObject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		className     string
		id            string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:      "successful get",
			className: "Article",
			id:        "test-uuid",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/v1/objects/Article/test-uuid", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(WeaviateObject{
					ID:         "test-uuid",
					Class:      "Article",
					Properties: map[string]interface{}{"title": "Test"},
				})
			},
			expectError: false,
		},
		{
			name:      "object not found",
			className: "Article",
			id:        "nonexistent",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectError: true,
		},
		{
			name:      "server error",
			className: "Article",
			id:        "test",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
		{
			name:      "decode error",
			className: "Article",
			id:        "test",
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			obj, err := adapter.GetObject(context.Background(), tt.className, tt.id)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, obj)
			}
		})
	}
}

func TestWeaviateAdapter_DeleteObject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		className     string
		id            string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:      "successful delete",
			className: "Article",
			id:        "test-uuid",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "DELETE", r.Method)
				assert.Equal(t, "/v1/objects/Article/test-uuid", r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
			},
			expectError: false,
		},
		{
			name:      "delete failure",
			className: "Article",
			id:        "nonexistent",
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			err := adapter.DeleteObject(context.Background(), tt.className, tt.id)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWeaviateAdapter_UpdateObject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		obj           *WeaviateObject
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name: "successful update",
			obj: &WeaviateObject{
				ID:         "test-uuid",
				Class:      "Article",
				Properties: map[string]interface{}{"title": "Updated Title"},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/v1/objects/Article/test-uuid", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectError: false,
		},
		{
			name: "update failure",
			obj: &WeaviateObject{
				ID:    "nonexistent",
				Class: "Article",
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":[{"message":"object not found"}]}`))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(tt.serverHandler))
			defer server.Close()

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			err := adapter.UpdateObject(context.Background(), tt.obj)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWeaviateAdapter_VectorSearch(t *testing.T) {
	t.Parallel()
	graphqlResponse := map[string]interface{}{
		"data": map[string]interface{}{
			"Get": map[string]interface{}{
				"Article": []interface{}{
					map[string]interface{}{
						"title": "Result 1",
						"_additional": map[string]interface{}{
							"id":        "uuid-1",
							"certainty": 0.95,
							"distance":  0.05,
						},
					},
					map[string]interface{}{
						"title": "Result 2",
						"_additional": map[string]interface{}{
							"id":        "uuid-2",
							"certainty": 0.85,
							"distance":  0.15,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		className     string
		vector        []float32
		limit         int
		certainty     float32
		properties    []string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful search with properties",
			className:  "Article",
			vector:     []float32{0.1, 0.2, 0.3},
			limit:      10,
			certainty:  0.7,
			properties: []string{"title", "content"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v1/graphql", r.URL.Path)
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Contains(t, body["query"], "nearVector")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(graphqlResponse)
			},
			expectError: false,
		},
		{
			name:       "successful search without properties",
			className:  "Article",
			vector:     []float32{0.1, 0.2},
			limit:      5,
			certainty:  0.8,
			properties: nil,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(graphqlResponse)
			},
			expectError: false,
		},
		{
			name:       "search failure",
			className:  "Nonexistent",
			vector:     []float32{0.1},
			limit:      10,
			certainty:  0.7,
			properties: nil,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"errors":[{"message":"class not found"}]}`))
			},
			expectError: true,
		},
		{
			name:      "decode error",
			className: "Article",
			vector:    []float32{0.1},
			limit:     10,
			certainty: 0.7,
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			results, err := adapter.VectorSearch(context.Background(), tt.className, tt.vector, tt.limit, tt.certainty, tt.properties)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, results)
			}
		})
	}
}

func TestWeaviateAdapter_HybridSearch(t *testing.T) {
	t.Parallel()
	graphqlResponse := map[string]interface{}{
		"data": map[string]interface{}{
			"Get": map[string]interface{}{
				"Article": []interface{}{
					map[string]interface{}{
						"title": "Hybrid Result 1",
						"_additional": map[string]interface{}{
							"id":    "uuid-1",
							"score": 0.95,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		className     string
		query         string
		vector        []float32
		limit         int
		alpha         float32
		properties    []string
		serverHandler func(w http.ResponseWriter, r *http.Request)
		expectError   bool
	}{
		{
			name:       "successful hybrid search",
			className:  "Article",
			query:      "test query",
			vector:     []float32{0.1, 0.2, 0.3},
			limit:      10,
			alpha:      0.5,
			properties: []string{"title"},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				assert.Contains(t, body["query"], "hybrid")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(graphqlResponse)
			},
			expectError: false,
		},
		{
			name:       "hybrid search without properties",
			className:  "Article",
			query:      "test",
			vector:     []float32{0.1},
			limit:      5,
			alpha:      0.7,
			properties: nil,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(graphqlResponse)
			},
			expectError: false,
		},
		{
			name:      "hybrid search failure",
			className: "Nonexistent",
			query:     "test",
			vector:    []float32{0.1},
			limit:     10,
			alpha:     0.5,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"errors":[{"message":"class not found"}]}`))
			},
			expectError: true,
		},
		{
			name:      "decode error",
			className: "Article",
			query:     "test",
			vector:    []float32{0.1},
			limit:     10,
			alpha:     0.5,
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

			adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
			results, err := adapter.HybridSearch(context.Background(), tt.className, tt.query, tt.vector, tt.limit, tt.alpha, tt.properties)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, results)
			}
		})
	}
}

func TestWeaviateAdapter_Close(t *testing.T) {
	t.Parallel()
	adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: "http://localhost:8080"})
	adapter.connected = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.IsConnected())
}

func TestWeaviateAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: "http://localhost:8080"})
	tools := adapter.GetMCPTools()

	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}

	expectedTools := []string{
		"weaviate_list_classes",
		"weaviate_create_class",
		"weaviate_delete_class",
		"weaviate_create_object",
		"weaviate_vector_search",
		"weaviate_hybrid_search",
		"weaviate_delete_object",
	}

	for _, expected := range expectedTools {
		assert.True(t, toolNames[expected], "Expected tool %s not found", expected)
	}
}

func TestWeaviateAdapter_doRequest_WithAPIKey(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer test-api-key", authHeader)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewWeaviateAdapter(WeaviateAdapterConfig{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
	})

	err := adapter.Health(context.Background())
	require.NoError(t, err)
}

func TestWeaviateAdapter_IsConnected(t *testing.T) {
	t.Parallel()
	adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: "http://localhost:8080"})

	// Initially not connected
	assert.False(t, adapter.IsConnected())

	// Simulate connection
	adapter.mu.Lock()
	adapter.connected = true
	adapter.mu.Unlock()

	assert.True(t, adapter.IsConnected())
}

func TestWeaviateAdapter_VectorSearch_ParseResults(t *testing.T) {
	t.Parallel()
	// Test that results are properly parsed from nested GraphQL response
	graphqlResponse := map[string]interface{}{
		"data": map[string]interface{}{
			"Get": map[string]interface{}{
				"TestClass": []interface{}{
					map[string]interface{}{
						"field1": "value1",
						"field2": 123,
						"_additional": map[string]interface{}{
							"id":        "uuid-1",
							"certainty": float64(0.95),
							"distance":  float64(0.05),
						},
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(graphqlResponse)
	}))
	defer server.Close()

	adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
	results, err := adapter.VectorSearch(context.Background(), "TestClass", []float32{0.1}, 10, 0.7, []string{"field1", "field2"})

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "uuid-1", results[0].ID)
	assert.Equal(t, "TestClass", results[0].Class)
	assert.Equal(t, float32(0.95), results[0].Certainty)
	assert.Equal(t, float32(0.05), results[0].Distance)
	assert.Equal(t, "value1", results[0].Properties["field1"])
}

func TestWeaviateAdapter_VectorSearch_EmptyResults(t *testing.T) {
	t.Parallel()
	graphqlResponse := map[string]interface{}{
		"data": map[string]interface{}{
			"Get": map[string]interface{}{
				"TestClass": []interface{}{},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(graphqlResponse)
	}))
	defer server.Close()

	adapter := NewWeaviateAdapter(WeaviateAdapterConfig{BaseURL: server.URL})
	results, err := adapter.VectorSearch(context.Background(), "TestClass", []float32{0.1}, 10, 0.7, nil)

	assert.NoError(t, err)
	assert.Empty(t, results)
}
