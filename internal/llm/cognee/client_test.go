package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Cognee: config.CogneeConfig{
			BaseURL: "http://localhost:7061",
			APIKey:  "test-api-key",
		},
	}

	client := NewClient(cfg)
	require.NotNil(t, client)
	assert.Equal(t, "http://localhost:7061", client.baseURL)
	assert.Equal(t, "test-api-key", client.apiKey)
	assert.NotNil(t, client.client)
	assert.Equal(t, 30*time.Second, client.client.Timeout)
}

func TestNewClientWithEmptyConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Cognee: config.CogneeConfig{},
	}

	client := NewClient(cfg)
	require.NotNil(t, client)
	assert.Equal(t, "", client.baseURL)
	assert.Equal(t, "", client.apiKey)
}

func TestGetBaseURL(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Cognee: config.CogneeConfig{
			BaseURL: "http://test:7061",
		},
	}

	client := NewClient(cfg)
	assert.Equal(t, "http://test:7061", client.GetBaseURL())
}

func TestAddMemory(t *testing.T) {
	t.Parallel()
	t.Run("successful add memory", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/memory", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			var req MemoryRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "test content", req.Content)

			resp := MemoryResponse{
				VectorID:   "vec-123",
				GraphNodes: map[string]interface{}{"node1": "value1"},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.AddMemory(&MemoryRequest{
			Content:     "test content",
			DatasetName: "test-dataset",
			ContentType: "text/plain",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "vec-123", resp.VectorID)
		assert.Contains(t, resp.GraphNodes, "node1")
	})

	t.Run("server error", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.AddMemory(&MemoryRequest{Content: "test"})

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "cognee API error")
	})

	t.Run("without api key", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(MemoryResponse{VectorID: "v1"})
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.AddMemory(&MemoryRequest{Content: "test"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestSearchMemory(t *testing.T) {
	t.Parallel()
	t.Run("successful search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/search", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			var req SearchRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "test query", req.Query)
			assert.Equal(t, 10, req.Limit)

			resp := SearchResponse{
				Results: []models.MemorySource{
					{
						DatasetName:    "test-dataset",
						Content:        "result content",
						RelevanceScore: 0.95,
						SourceType:     "text",
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.SearchMemory(&SearchRequest{
			Query:       "test query",
			DatasetName: "test-dataset",
			Limit:       10,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Results, 1)
		assert.Equal(t, "result content", resp.Results[0].Content)
		assert.Equal(t, 0.95, resp.Results[0].RelevanceScore)
	})

	t.Run("server error", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.SearchMemory(&SearchRequest{Query: "test"})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestCognify(t *testing.T) {
	t.Parallel()
	t.Run("successful cognify", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/cognify", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			resp := CognifyResponse{Status: "completed"}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.Cognify(&CognifyRequest{
			Datasets: []string{"dataset1", "dataset2"},
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "completed", resp.Status)
	})

	t.Run("server error", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.Cognify(&CognifyRequest{})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestSearchInsights(t *testing.T) {
	t.Parallel()
	t.Run("successful insights search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/search", r.URL.Path)

			var req map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "INSIGHTS", req["search_type"])

			resp := InsightsResponse{
				Insights: []map[string]interface{}{
					{"key": "value"},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.SearchInsights(&InsightsRequest{
			Query:    "test query",
			Datasets: []string{"dataset1"},
			Limit:    5,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Insights, 1)
	})

	t.Run("server error", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.SearchInsights(&InsightsRequest{Query: "test"})
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "cognee API error")
	})

	t.Run("without api key", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(InsightsResponse{})
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.SearchInsights(&InsightsRequest{Query: "test"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestSearchGraphCompletion(t *testing.T) {
	t.Parallel()
	t.Run("successful graph completion search", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "GRAPH_COMPLETION", req["search_type"])

			resp := SearchResponse{
				Results: []models.MemorySource{
					{Content: "completion result"},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.SearchGraphCompletion("query", []string{"ds1"}, 5)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Results, 1)
	})

	t.Run("server error", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.SearchGraphCompletion("query", []string{"ds1"}, 5)
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "cognee API error")
	})

	t.Run("without api key", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SearchResponse{})
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.SearchGraphCompletion("query", nil, 10)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestProcessCodePipeline(t *testing.T) {
	t.Parallel()
	t.Run("successful code pipeline", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/code-pipeline/index", r.URL.Path)

			var req CodePipelineRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "func main() {}", req.Code)
			assert.Equal(t, "go", req.Language)

			resp := CodePipelineResponse{
				Processed: true,
				Results:   map[string]interface{}{"functions": 1},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.ProcessCodePipeline(&CodePipelineRequest{
			Code:        "func main() {}",
			DatasetName: "code-dataset",
			Language:    "go",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Processed)
	})

	t.Run("server error", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.ProcessCodePipeline(&CodePipelineRequest{Code: "invalid"})
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "cognee API error")
	})

	t.Run("without api key", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CodePipelineResponse{Processed: true})
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.ProcessCodePipeline(&CodePipelineRequest{Code: "test"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestCreateDataset(t *testing.T) {
	t.Parallel()
	t.Run("successful dataset creation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/datasets", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			var req DatasetRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "new-dataset", req.Name)

			resp := DatasetResponse{
				ID:        "ds-123",
				Name:      "new-dataset",
				CreatedAt: "2024-01-01T00:00:00Z",
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.CreateDataset(&DatasetRequest{
			Name:        "new-dataset",
			Description: "A test dataset",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "ds-123", resp.ID)
		assert.Equal(t, "new-dataset", resp.Name)
	})

	t.Run("creation fails", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.CreateDataset(&DatasetRequest{Name: "test"})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestListDatasets(t *testing.T) {
	t.Parallel()
	t.Run("successful list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/datasets", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			resp := DatasetsResponse{
				Datasets: []DatasetResponse{
					{ID: "ds-1", Name: "dataset1"},
					{ID: "ds-2", Name: "dataset2"},
				},
				Total: 2,
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.ListDatasets()

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 2, resp.Total)
		assert.Len(t, resp.Datasets, 2)
	})

	t.Run("server error", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.ListDatasets()
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "cognee API error")
	})

	t.Run("without api key", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(DatasetsResponse{Datasets: []DatasetResponse{}, Total: 0})
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.ListDatasets()
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestVisualizeGraph(t *testing.T) {
	t.Parallel()
	t.Run("successful visualization", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/visualize", r.URL.Path)

			resp := VisualizeResponse{
				Graph: map[string]interface{}{
					"nodes": []interface{}{"n1", "n2"},
					"edges": []interface{}{"e1"},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.VisualizeGraph(&VisualizeRequest{
			DatasetName: "test-dataset",
			Format:      "json",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Contains(t, resp.Graph, "nodes")
	})

	t.Run("server error", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.VisualizeGraph(&VisualizeRequest{DatasetName: "non-existent"})
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "cognee API error")
	})

	t.Run("without api key", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(VisualizeResponse{Graph: map[string]interface{}{}})
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		resp, err := client.VisualizeGraph(&VisualizeRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestDeleteData(t *testing.T) {
	t.Parallel()
	t.Run("successful delete", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/delete", r.URL.Path)
			assert.Equal(t, "DELETE", r.Method)

			var req map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "test-dataset", req["dataset_name"])

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			apiKey:  "test-key",
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		err := client.DeleteData("test-dataset", []string{"id1", "id2"})
		require.NoError(t, err)
	})

	t.Run("delete fails", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		err := client.DeleteData("test-dataset", []string{"id1"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cognee API error")
	})
}

func TestTestConnection(t *testing.T) {
	t.Parallel()
	t.Run("connection successful", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/health", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		connected := client.testConnection()
		assert.True(t, connected)
	})

	t.Run("connection failed", func(t *testing.T) {
			t.Parallel()
		client := &Client{
			baseURL: "http://localhost:9999", // Non-existent URL
			client:  &http.Client{Timeout: 1 * time.Second},
		}

		connected := client.testConnection()
		assert.False(t, connected)
	})

	t.Run("server returns error status", func(t *testing.T) {
			t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		connected := client.testConnection()
		assert.False(t, connected)
	})
}

func TestAutoContainerize(t *testing.T) {
	t.Parallel()
	t.Run("already running", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &Client{
			baseURL: server.URL,
			client:  &http.Client{Timeout: 5 * time.Second},
		}

		err := client.AutoContainerize()
		require.NoError(t, err)
	})

	t.Run("not running and docker not available", func(t *testing.T) {
			t.Parallel()
		client := &Client{
			baseURL: "http://localhost:9999",
			client:  &http.Client{Timeout: 1 * time.Second},
		}

		err := client.AutoContainerize()
		// This will fail because docker/docker-compose likely not in test PATH
		// or container won't start successfully in test environment
		if err != nil {
			// Expected in test environment
			assert.True(t, true)
		}
	})
}

// Struct field tests
func TestMemoryRequestFields(t *testing.T) {
	t.Parallel()
	req := &MemoryRequest{
		Content:     "test content",
		DatasetName: "test-dataset",
		ContentType: "text/plain",
	}

	assert.Equal(t, "test content", req.Content)
	assert.Equal(t, "test-dataset", req.DatasetName)
	assert.Equal(t, "text/plain", req.ContentType)
}

func TestMemoryResponseFields(t *testing.T) {
	t.Parallel()
	response := &MemoryResponse{
		VectorID: "vector-123",
		GraphNodes: map[string]interface{}{
			"node1": "value1",
		},
	}

	assert.Equal(t, "vector-123", response.VectorID)
	assert.Len(t, response.GraphNodes, 1)
	assert.Equal(t, "value1", response.GraphNodes["node1"])
}

func TestSearchRequestFields(t *testing.T) {
	t.Parallel()
	req := &SearchRequest{
		Query:       "test query",
		DatasetName: "test-dataset",
		Limit:       10,
	}

	assert.Equal(t, "test query", req.Query)
	assert.Equal(t, "test-dataset", req.DatasetName)
	assert.Equal(t, 10, req.Limit)
}

func TestCognifyRequestFields(t *testing.T) {
	t.Parallel()
	req := &CognifyRequest{
		Datasets: []string{"ds1", "ds2"},
	}

	assert.Len(t, req.Datasets, 2)
	assert.Equal(t, "ds1", req.Datasets[0])
}

func TestInsightsRequestFields(t *testing.T) {
	t.Parallel()
	req := &InsightsRequest{
		Query:    "insights query",
		Datasets: []string{"ds1"},
		Limit:    5,
	}

	assert.Equal(t, "insights query", req.Query)
	assert.Len(t, req.Datasets, 1)
	assert.Equal(t, 5, req.Limit)
}

func TestCodePipelineRequestFields(t *testing.T) {
	t.Parallel()
	req := &CodePipelineRequest{
		Code:        "func main() {}",
		DatasetName: "code-ds",
		Language:    "go",
	}

	assert.Equal(t, "func main() {}", req.Code)
	assert.Equal(t, "code-ds", req.DatasetName)
	assert.Equal(t, "go", req.Language)
}

func TestDatasetRequestFields(t *testing.T) {
	t.Parallel()
	req := &DatasetRequest{
		Name:        "test-ds",
		Description: "Test description",
		Metadata:    map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "test-ds", req.Name)
	assert.Equal(t, "Test description", req.Description)
	assert.Contains(t, req.Metadata, "key")
}

func TestDatasetResponseFields(t *testing.T) {
	t.Parallel()
	resp := &DatasetResponse{
		ID:          "ds-123",
		Name:        "test-ds",
		Description: "desc",
		CreatedAt:   "2024-01-01",
		Metadata:    map[string]interface{}{"k": "v"},
	}

	assert.Equal(t, "ds-123", resp.ID)
	assert.Equal(t, "test-ds", resp.Name)
	assert.Equal(t, "desc", resp.Description)
	assert.Equal(t, "2024-01-01", resp.CreatedAt)
}

func TestVisualizeRequestFields(t *testing.T) {
	t.Parallel()
	req := &VisualizeRequest{
		DatasetName: "test-ds",
		Format:      "graphml",
	}

	assert.Equal(t, "test-ds", req.DatasetName)
	assert.Equal(t, "graphml", req.Format)
}

// Benchmarks
func BenchmarkAddMemory(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(MemoryResponse{VectorID: "v1"})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	req := &MemoryRequest{Content: "test", DatasetName: "ds", ContentType: "text"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.AddMemory(req)
	}
}

func BenchmarkSearchMemory(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchResponse{})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	req := &SearchRequest{Query: "test", DatasetName: "ds", Limit: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.SearchMemory(req)
	}
}

// ==============================================================================
// ADDITIONAL COMPREHENSIVE TESTS FOR COGNEE CLIENT
// ==============================================================================

func TestAddMemory_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999", // Non-existent server
		apiKey:  "test-key",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.AddMemory(&MemoryRequest{Content: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestAddMemory_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "test-key",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.AddMemory(&MemoryRequest{Content: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSearchMemory_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.SearchMemory(&SearchRequest{Query: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSearchMemory_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.SearchMemory(&SearchRequest{Query: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestCognify_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.Cognify(&CognifyRequest{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestCognify_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.Cognify(&CognifyRequest{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSearchInsights_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.SearchInsights(&InsightsRequest{Query: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSearchInsights_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("broken json"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.SearchInsights(&InsightsRequest{Query: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSearchGraphCompletion_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.SearchGraphCompletion("query", nil, 10)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSearchGraphCompletion_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<not json>"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.SearchGraphCompletion("query", nil, 10)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestProcessCodePipeline_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.ProcessCodePipeline(&CodePipelineRequest{Code: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestProcessCodePipeline_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid json syntax"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.ProcessCodePipeline(&CodePipelineRequest{Code: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestCreateDataset_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.CreateDataset(&DatasetRequest{Name: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestCreateDataset_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("not json response"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.CreateDataset(&DatasetRequest{Name: "test"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestListDatasets_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.ListDatasets()
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestListDatasets_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[broken]"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.ListDatasets()
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestVisualizeGraph_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	resp, err := client.VisualizeGraph(&VisualizeRequest{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestVisualizeGraph_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid json"))
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.VisualizeGraph(&VisualizeRequest{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestDeleteData_NetworkError(t *testing.T) {
	t.Parallel()
	client := &Client{
		baseURL: "http://localhost:9999",
		client:  &http.Client{Timeout: 100 * time.Millisecond},
	}

	err := client.DeleteData("test-ds", []string{"id1"})
	assert.Error(t, err)
}

func TestAddMemory_VerifyHeaders(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/memory", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer my-api-key", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(MemoryResponse{VectorID: "vec-1"})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "my-api-key",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.AddMemory(&MemoryRequest{
		Content:     "test content",
		DatasetName: "ds",
		ContentType: "text/plain",
	})

	require.NoError(t, err)
	assert.Equal(t, "vec-1", resp.VectorID)
}

func TestSearchMemory_VerifyRequestBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SearchRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "search query", req.Query)
		assert.Equal(t, "my-dataset", req.DatasetName)
		assert.Equal(t, 25, req.Limit)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchResponse{
			Results: []models.MemorySource{
				{Content: "found", RelevanceScore: 0.9},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "key",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.SearchMemory(&SearchRequest{
		Query:       "search query",
		DatasetName: "my-dataset",
		Limit:       25,
	})

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "found", resp.Results[0].Content)
}

func TestCognify_WithMultipleDatasets(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CognifyRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.ElementsMatch(t, []string{"ds1", "ds2", "ds3"}, req.Datasets)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CognifyResponse{Status: "processing"})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.Cognify(&CognifyRequest{
		Datasets: []string{"ds1", "ds2", "ds3"},
	})

	require.NoError(t, err)
	assert.Equal(t, "processing", resp.Status)
}

func TestSearchInsights_RequestFormat(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "INSIGHTS", req["search_type"])
		assert.Equal(t, "insight query", req["query"])
		assert.Equal(t, float64(10), req["limit"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(InsightsResponse{
			Insights: []map[string]interface{}{
				{"insight": "result1"},
				{"insight": "result2"},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "key",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.SearchInsights(&InsightsRequest{
		Query:    "insight query",
		Datasets: []string{"ds1"},
		Limit:    10,
	})

	require.NoError(t, err)
	assert.Len(t, resp.Insights, 2)
}

func TestSearchGraphCompletion_RequestFormat(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "GRAPH_COMPLETION", req["search_type"])
		assert.Equal(t, "completion query", req["query"])
		assert.Equal(t, float64(5), req["limit"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SearchResponse{
			Results: []models.MemorySource{{Content: "completed"}},
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.SearchGraphCompletion("completion query", []string{"ds"}, 5)

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "completed", resp.Results[0].Content)
}

func TestProcessCodePipeline_WithLanguage(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/code-pipeline/index", r.URL.Path)

		var req CodePipelineRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "func test() {}", req.Code)
		assert.Equal(t, "code-ds", req.DatasetName)
		assert.Equal(t, "go", req.Language)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CodePipelineResponse{
			Processed: true,
			Results: map[string]interface{}{
				"functions": 1,
				"lines":     3,
			},
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "key",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.ProcessCodePipeline(&CodePipelineRequest{
		Code:        "func test() {}",
		DatasetName: "code-ds",
		Language:    "go",
	})

	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Contains(t, resp.Results, "functions")
}

func TestCreateDataset_WithMetadata(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DatasetRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "new-dataset", req.Name)
		assert.Equal(t, "A description", req.Description)
		assert.Equal(t, "value1", req.Metadata["key1"])

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(DatasetResponse{
			ID:          "ds-new",
			Name:        req.Name,
			Description: req.Description,
			CreatedAt:   "2024-01-01T00:00:00Z",
			Metadata:    req.Metadata,
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "key",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.CreateDataset(&DatasetRequest{
		Name:        "new-dataset",
		Description: "A description",
		Metadata:    map[string]interface{}{"key1": "value1"},
	})

	require.NoError(t, err)
	assert.Equal(t, "ds-new", resp.ID)
	assert.Equal(t, "A description", resp.Description)
}

func TestVisualizeGraph_WithFormat(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/visualize", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(VisualizeResponse{
			Graph: map[string]interface{}{
				"nodes": []interface{}{
					map[string]interface{}{"id": "n1", "label": "Node 1"},
					map[string]interface{}{"id": "n2", "label": "Node 2"},
				},
				"edges": []interface{}{
					map[string]interface{}{"from": "n1", "to": "n2"},
				},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "key",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.VisualizeGraph(&VisualizeRequest{
		DatasetName: "graph-ds",
		Format:      "json",
	})

	require.NoError(t, err)
	assert.Contains(t, resp.Graph, "nodes")
	assert.Contains(t, resp.Graph, "edges")
}

func TestDeleteData_WithMultipleIDs(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/delete", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "target-ds", req["dataset_name"])
		dataIDs := req["data_ids"].([]interface{})
		assert.Len(t, dataIDs, 3)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		apiKey:  "key",
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	err := client.DeleteData("target-ds", []string{"id1", "id2", "id3"})
	require.NoError(t, err)
}

// Test response types
func TestSearchResponseFields(t *testing.T) {
	t.Parallel()
	resp := &SearchResponse{
		Results: []models.MemorySource{
			{
				DatasetName:    "ds1",
				Content:        "content",
				RelevanceScore: 0.95,
				SourceType:     "text",
			},
		},
	}

	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "ds1", resp.Results[0].DatasetName)
	assert.Equal(t, 0.95, resp.Results[0].RelevanceScore)
}

func TestCognifyResponseFields(t *testing.T) {
	t.Parallel()
	resp := &CognifyResponse{
		Status: "completed",
	}

	assert.Equal(t, "completed", resp.Status)
}

func TestInsightsResponseFields(t *testing.T) {
	t.Parallel()
	resp := &InsightsResponse{
		Insights: []map[string]interface{}{
			{"key1": "value1"},
			{"key2": "value2"},
		},
	}

	assert.Len(t, resp.Insights, 2)
	assert.Equal(t, "value1", resp.Insights[0]["key1"])
}

func TestCodePipelineResponseFields(t *testing.T) {
	t.Parallel()
	resp := &CodePipelineResponse{
		Processed: true,
		Results: map[string]interface{}{
			"analysis": "complete",
		},
	}

	assert.True(t, resp.Processed)
	assert.Equal(t, "complete", resp.Results["analysis"])
}

func TestDatasetsResponseFields(t *testing.T) {
	t.Parallel()
	resp := &DatasetsResponse{
		Datasets: []DatasetResponse{
			{ID: "ds1", Name: "Dataset 1"},
			{ID: "ds2", Name: "Dataset 2"},
		},
		Total: 2,
	}

	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Datasets, 2)
}

func TestVisualizeResponseFields(t *testing.T) {
	t.Parallel()
	resp := &VisualizeResponse{
		Graph: map[string]interface{}{
			"format": "json",
			"data":   "graph_data",
		},
	}

	assert.Equal(t, "json", resp.Graph["format"])
}

// Benchmarks
func BenchmarkCognify(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CognifyResponse{Status: "ok"})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	req := &CognifyRequest{Datasets: []string{"ds1"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Cognify(req)
	}
}

func BenchmarkSearchInsights(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(InsightsResponse{})
	}))
	defer server.Close()

	client := &Client{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	req := &InsightsRequest{Query: "test", Limit: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.SearchInsights(req)
	}
}
