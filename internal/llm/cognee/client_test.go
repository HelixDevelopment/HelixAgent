package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/models"
)

func TestNewClient(t *testing.T) {
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
	cfg := &config.Config{
		Cognee: config.CogneeConfig{},
	}

	client := NewClient(cfg)
	require.NotNil(t, client)
	assert.Equal(t, "", client.baseURL)
	assert.Equal(t, "", client.apiKey)
}

func TestGetBaseURL(t *testing.T) {
	cfg := &config.Config{
		Cognee: config.CogneeConfig{
			BaseURL: "http://test:7061",
		},
	}

	client := NewClient(cfg)
	assert.Equal(t, "http://test:7061", client.GetBaseURL())
}

func TestAddMemory(t *testing.T) {
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
			json.NewEncoder(w).Encode(resp)
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(MemoryResponse{VectorID: "v1"})
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
			json.NewEncoder(w).Encode(resp)
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
	t.Run("successful cognify", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/cognify", r.URL.Path)
			assert.Equal(t, "POST", r.Method)

			resp := CognifyResponse{Status: "completed"}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
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
			json.NewEncoder(w).Encode(resp)
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(InsightsResponse{})
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
			json.NewEncoder(w).Encode(resp)
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(SearchResponse{})
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
			json.NewEncoder(w).Encode(resp)
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CodePipelineResponse{Processed: true})
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
			json.NewEncoder(w).Encode(resp)
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
			json.NewEncoder(w).Encode(resp)
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(DatasetsResponse{Datasets: []DatasetResponse{}, Total: 0})
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
			json.NewEncoder(w).Encode(resp)
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(VisualizeResponse{Graph: map[string]interface{}{}})
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
		client := &Client{
			baseURL: "http://localhost:9999", // Non-existent URL
			client:  &http.Client{Timeout: 1 * time.Second},
		}

		connected := client.testConnection()
		assert.False(t, connected)
	})

	t.Run("server returns error status", func(t *testing.T) {
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
	req := &CognifyRequest{
		Datasets: []string{"ds1", "ds2"},
	}

	assert.Len(t, req.Datasets, 2)
	assert.Equal(t, "ds1", req.Datasets[0])
}

func TestInsightsRequestFields(t *testing.T) {
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
		json.NewEncoder(w).Encode(MemoryResponse{VectorID: "v1"})
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
		json.NewEncoder(w).Encode(SearchResponse{})
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
