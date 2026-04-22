package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCogneeMockServer(t *testing.T) {
	server := NewCogneeMockServer(8080)
	assert.NotNil(t, server)
	assert.Equal(t, 8080, server.port)
	assert.NotNil(t, server.memories)
}

func TestHandleHealth(t *testing.T) {
	server := NewCogneeMockServer(8080)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "healthy", body["status"])
	assert.Equal(t, "cognee-mock", body["service"])
	assert.NotEmpty(t, body["timestamp"])
}

func TestHandleAdd(t *testing.T) {
	server := NewCogneeMockServer(8080)

	t.Run("successful add", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"content":      "Test memory content",
			"dataset":      "test-dataset",
			"content_type": "text",
			"metadata":     map[string]interface{}{"key": "value"},
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/add", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleAdd(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		assert.Equal(t, "success", body["status"])
		assert.NotEmpty(t, body["id"])

		// Verify memory was stored
		memories, _ := server.memories.Get("test-dataset")
		assert.Len(t, memories, 1)
		assert.Equal(t, "Test memory content", memories[0].Content)
	})

	t.Run("default dataset", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"content": "Another memory",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/add", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleAdd(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify memory was stored in default dataset
		memories, _ := server.memories.Get("default")
		assert.Len(t, memories, 1)
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/add", nil)
		w := httptest.NewRecorder()

		server.handleAdd(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/add", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleAdd(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandleSearch(t *testing.T) {
	server := NewCogneeMockServer(8080)

	// Add some test data first
	server.memories.Put("test-dataset", []MemoryEntry{
		{ID: "mem_1", Content: "First memory", Dataset: "test-dataset", CreatedAt: time.Now()},
		{ID: "mem_2", Content: "Second memory", Dataset: "test-dataset", CreatedAt: time.Now()},
		{ID: "mem_3", Content: "Third memory", Dataset: "test-dataset", CreatedAt: time.Now()},
	})

	t.Run("POST search", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"query":   "memory",
			"dataset": "test-dataset",
			"limit":   2,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleSearch(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		results := body["results"].([]interface{})
		assert.Len(t, results, 2)
	})

	t.Run("GET search with query params", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?query=test&dataset=test-dataset", nil)
		w := httptest.NewRecorder()

		server.handleSearch(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		assert.NotNil(t, body["results"])
	})

	t.Run("default limit", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"query":   "memory",
			"dataset": "test-dataset",
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewReader(bodyBytes))
		w := httptest.NewRecorder()

		server.handleSearch(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		// Should return all 3 since limit defaults to 10
		results := body["results"].([]interface{})
		assert.Len(t, results, 3)
	})
}

func TestHandleCognify(t *testing.T) {
	server := NewCogneeMockServer(8080)

	t.Run("successful cognify", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"datasets": []string{"dataset1", "dataset2"},
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/cognify", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.handleCognify(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		assert.Equal(t, "success", body["status"])
		assert.Equal(t, "Knowledge graph updated", body["message"])
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cognify", nil)
		w := httptest.NewRecorder()

		server.handleCognify(w, req)

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

func TestHandleGraphQuery(t *testing.T) {
	server := NewCogneeMockServer(8080)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph", nil)
	w := httptest.NewRecorder()

	server.handleGraphQuery(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.NotNil(t, body["nodes"])
	assert.NotNil(t, body["edges"])
}

func TestHandleInsights(t *testing.T) {
	server := NewCogneeMockServer(8080)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/insights", nil)
	w := httptest.NewRecorder()

	server.handleInsights(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	insights := body["insights"].([]interface{})
	assert.Len(t, insights, 1)

	insight := insights[0].(map[string]interface{})
	assert.Equal(t, "summary", insight["type"])
}

func TestHandleDatasets(t *testing.T) {
	server := NewCogneeMockServer(8080)

	// Add some test data
	server.memories.Put("dataset1", []MemoryEntry{{ID: "1"}})
	server.memories.Put("dataset2", []MemoryEntry{{ID: "2"}})
	server.memories.Put("dataset3", []MemoryEntry{{ID: "3"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets", nil)
	w := httptest.NewRecorder()

	server.handleDatasets(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	datasets := body["datasets"].([]interface{})
	assert.Len(t, datasets, 3)
}

func TestMemoryEntry(t *testing.T) {
	entry := MemoryEntry{
		ID:        "test-id",
		Content:   "test content",
		Dataset:   "test-dataset",
		Type:      "text",
		Metadata:  map[string]interface{}{"key": "value"},
		CreatedAt: time.Now(),
	}

	// Test JSON serialization
	data, err := json.Marshal(entry)
	require.NoError(t, err)

	var decoded MemoryEntry
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, entry.ID, decoded.ID)
	assert.Equal(t, entry.Content, decoded.Content)
	assert.Equal(t, entry.Dataset, decoded.Dataset)
}

func TestConcurrentAccess(t *testing.T) {
	server := NewCogneeMockServer(8080)

	// Test concurrent adds
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			reqBody := map[string]interface{}{
				"content": "Concurrent memory",
				"dataset": "concurrent-test",
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/add", bytes.NewReader(bodyBytes))
			w := httptest.NewRecorder()
			server.handleAdd(w, req)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all memories were added
	memories, _ := server.memories.Get("concurrent-test")

	assert.Len(t, memories, 10)
}

func TestServerRoutes(t *testing.T) {
	server := NewCogneeMockServer(8080)

	// Create a test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/api/v1/health", server.handleHealth)
	mux.HandleFunc("/api/v1/add", server.handleAdd)
	mux.HandleFunc("/api/v1/search", server.handleSearch)
	mux.HandleFunc("/api/v1/cognify", server.handleCognify)
	mux.HandleFunc("/api/v1/graph", server.handleGraphQuery)
	mux.HandleFunc("/api/v1/insights", server.handleInsights)
	mux.HandleFunc("/api/v1/datasets", server.handleDatasets)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test each endpoint
	endpoints := []struct {
		method string
		path   string
		body   io.Reader
		status int
	}{
		{http.MethodGet, "/health", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/health", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/graph", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/insights", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/datasets", nil, http.StatusOK},
	}

	client := &http.Client{}
	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req, err := http.NewRequest(ep.method, ts.URL+ep.path, ep.body)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, ep.status, resp.StatusCode)
		})
	}
}
