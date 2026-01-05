package llamaindex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	config := &ClientConfig{
		BaseURL: "http://localhost:8012",
		Timeout: 30 * time.Second,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8012", client.baseURL)
}

func TestNewClient_DefaultConfig(t *testing.T) {
	client := NewClient(nil)
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8012", client.baseURL)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "http://localhost:8012", config.BaseURL)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestClient_Query(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/query", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var req QueryRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.NotEmpty(t, req.Query)

		resp := &QueryResponse{
			Answer: "Machine learning is a subset of artificial intelligence.",
			Sources: []Source{
				{
					Content:  "Machine learning is a subset of artificial intelligence.",
					Score:    0.95,
					Metadata: map[string]interface{}{"source": "ml_intro.md"},
				},
				{
					Content:  "Deep learning uses neural networks with many layers.",
					Score:    0.87,
					Metadata: map[string]interface{}{"source": "dl_guide.md"},
				},
			},
			Confidence: 0.92,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	resp, err := client.Query(context.Background(), &QueryRequest{
		Query:     "What is machine learning?",
		TopK:      5,
		UseCognee: true,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Sources, 2)
	assert.Contains(t, resp.Sources[0].Content, "Machine learning")
	assert.Greater(t, resp.Sources[0].Score, 0.9)
	assert.Greater(t, resp.Confidence, 0.9)
}

func TestClient_QueryWithHyDE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/query", r.URL.Path)

		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotNil(t, req.QueryTransform)
		assert.Equal(t, "hyde", *req.QueryTransform)

		resp := &QueryResponse{
			Answer: "Quantum computing explanation",
			Sources: []Source{
				{Content: "Relevant document about quantum computing", Score: 0.92},
			},
			Confidence: 0.88,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.QueryWithHyDE(context.Background(), "Explain quantum computing", 5)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Answer)
}

func TestClient_HyDEExpand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/hyde", r.URL.Path)

		var req HyDERequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotEmpty(t, req.Query)

		resp := &HyDEResponse{
			OriginalQuery: req.Query,
			HypotheticalDocuments: []string{
				"A detailed hypothetical answer about quantum entanglement...",
				"Another perspective on quantum entanglement...",
			},
			CombinedEmbedding: []float64{0.1, 0.2, 0.3},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.HyDEExpand(context.Background(), &HyDERequest{
		Query:         "Explain quantum entanglement",
		NumHypotheses: 2,
	})

	require.NoError(t, err)
	assert.Len(t, resp.HypotheticalDocuments, 2)
}

func TestClient_DecomposeQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/decompose", r.URL.Path)

		var req DecomposeQueryRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := &DecomposeQueryResponse{
			OriginalQuery: req.Query,
			Subqueries: []string{
				"What are the economic policies of the US in 2023?",
				"What are the economic policies of the EU in 2023?",
				"How do US and EU economic policies differ?",
			},
			Reasoning: "Complex comparative query decomposed into simpler components",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.DecomposeQuery(context.Background(), &DecomposeQueryRequest{
		Query:         "Compare the economic policies of the US and EU in 2023",
		MaxSubqueries: 3,
	})

	require.NoError(t, err)
	assert.Len(t, resp.Subqueries, 3)
}

func TestClient_Rerank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rerank", r.URL.Path)

		var req RerankRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := &RerankResponse{
			RankedDocuments: []RankedDocument{
				{Content: "Machine learning is a subset of AI.", Score: 0.98, Rank: 1},
				{Content: "Deep learning uses neural networks.", Score: 0.85, Rank: 2},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Rerank(context.Background(), &RerankRequest{
		Query: "What is machine learning?",
		Documents: []string{
			"Machine learning is a subset of AI.",
			"The weather is nice today.",
			"Deep learning uses neural networks.",
		},
		TopK: 2,
	})

	require.NoError(t, err)
	assert.Len(t, resp.RankedDocuments, 2)
	assert.Greater(t, resp.RankedDocuments[0].Score, resp.RankedDocuments[1].Score)
}

func TestClient_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		resp := &HealthResponse{
			Status:             "healthy",
			Version:            "1.0.0",
			CogneeAvailable:    true,
			SuperagentAvailable: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	health, err := client.Health(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.True(t, health.CogneeAvailable)
}

func TestClient_IsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			resp := &HealthResponse{Status: "healthy"}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	available := client.IsAvailable(context.Background())
	assert.True(t, available)
}

func TestClient_IsAvailable_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	available := client.IsAvailable(context.Background())
	assert.False(t, available)
}

func TestClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.Query(context.Background(), &QueryRequest{Query: "test"})
	assert.Error(t, err)
}

func TestClient_QueryWithRerank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.True(t, req.Rerank)

		resp := &QueryResponse{
			Answer:     "Reranked answer",
			Sources:    []Source{{Content: "Result", Score: 0.95}},
			Confidence: 0.9,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Query(context.Background(), &QueryRequest{
		Query:  "test",
		Rerank: true,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Answer)
}

func TestClient_QueryWithDecomposition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotNil(t, req.QueryTransform)
		assert.Equal(t, "decompose", *req.QueryTransform)

		transformed := "Decomposed query"
		resp := &QueryResponse{
			Answer:           "Complex answer",
			Sources:          []Source{{Content: "Result", Score: 0.95}},
			TransformedQuery: &transformed,
			Confidence:       0.9,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.QueryWithDecomposition(context.Background(), "complex query", 5)

	require.NoError(t, err)
	assert.NotNil(t, resp.TransformedQuery)
}

func TestClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 50 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Query(ctx, &QueryRequest{Query: "test"})
	assert.Error(t, err)
}

func TestClient_QueryWithStepBack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/query", r.URL.Path)

		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotNil(t, req.QueryTransform)
		assert.Equal(t, "step_back", *req.QueryTransform)
		assert.True(t, req.UseCognee)
		assert.True(t, req.Rerank)

		transformed := "Step-back query: What are the general principles?"
		resp := &QueryResponse{
			Answer:           "Detailed answer using step-back prompting",
			Sources:          []Source{{Content: "Background document", Score: 0.94}},
			TransformedQuery: &transformed,
			Confidence:       0.88,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.QueryWithStepBack(context.Background(), "Why did X event happen in Y context?", 5)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Answer)
	assert.NotNil(t, resp.TransformedQuery)
}

func TestClient_QueryFusion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/query_fusion", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		type fusionReq struct {
			Query         string `json:"query"`
			NumVariations int    `json:"num_variations"`
			TopK          int    `json:"top_k"`
		}

		var req fusionReq
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotEmpty(t, req.Query)
		assert.Equal(t, 4, req.NumVariations)
		assert.Equal(t, 10, req.TopK)

		resp := &QueryFusionResponse{
			Query: req.Query,
			VariationsUsed: []string{
				"What is machine learning?",
				"Define machine learning",
				"Explain ML concepts",
				"Machine learning basics",
			},
			Results: []Source{
				{Content: "ML is a subset of AI", Score: 0.96},
				{Content: "Machine learning uses data", Score: 0.92},
				{Content: "Deep learning is a type of ML", Score: 0.88},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.QueryFusion(context.Background(), "What is machine learning?", 4, 10)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.VariationsUsed, 4)
	assert.Len(t, resp.Results, 3)
}

func TestClient_QueryFusion_Defaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type fusionReq struct {
			Query         string `json:"query"`
			NumVariations int    `json:"num_variations"`
			TopK          int    `json:"top_k"`
		}

		var req fusionReq
		json.NewDecoder(r.Body).Decode(&req)
		// Check defaults were applied
		assert.Equal(t, 3, req.NumVariations) // Default
		assert.Equal(t, 5, req.TopK)          // Default

		resp := &QueryFusionResponse{
			Query:          req.Query,
			VariationsUsed: []string{"var1", "var2", "var3"},
			Results:        []Source{{Content: "Result", Score: 0.9}},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.QueryFusion(context.Background(), "test query", 0, 0)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestClient_HyDEExpand_DefaultHypotheses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req HyDERequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 3, req.NumHypotheses) // Default value

		resp := &HyDEResponse{
			OriginalQuery:         req.Query,
			HypotheticalDocuments: []string{"hyp1", "hyp2", "hyp3"},
			CombinedEmbedding:     []float64{0.1, 0.2},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.HyDEExpand(context.Background(), &HyDERequest{
		Query:         "test query",
		NumHypotheses: 0, // Should use default
	})

	require.NoError(t, err)
	assert.Len(t, resp.HypotheticalDocuments, 3)
}

func TestClient_DecomposeQuery_DefaultSubqueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DecomposeQueryRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 3, req.MaxSubqueries) // Default value

		resp := &DecomposeQueryResponse{
			OriginalQuery: req.Query,
			Subqueries:    []string{"sub1", "sub2", "sub3"},
			Reasoning:     "Test reasoning",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.DecomposeQuery(context.Background(), &DecomposeQueryRequest{
		Query:         "complex query",
		MaxSubqueries: 0, // Should use default
	})

	require.NoError(t, err)
	assert.Len(t, resp.Subqueries, 3)
}

func TestClient_Rerank_DefaultTopK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RerankRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 5, req.TopK) // Default value

		resp := &RerankResponse{
			RankedDocuments: []RankedDocument{
				{Content: "Doc1", Score: 0.9, Rank: 1},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Rerank(context.Background(), &RerankRequest{
		Query:     "test",
		Documents: []string{"doc1", "doc2"},
		TopK:      0, // Should use default
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.RankedDocuments)
}

func TestClient_Query_DefaultTopK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 5, req.TopK) // Default value

		resp := &QueryResponse{
			Answer:     "Answer",
			Sources:    []Source{{Content: "Source", Score: 0.9}},
			Confidence: 0.8,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Query(context.Background(), &QueryRequest{
		Query: "test",
		TopK:  0, // Should use default
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Answer)
}

func TestClient_QueryFusion_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "fusion failed"}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.QueryFusion(context.Background(), "test", 3, 5)
	assert.Error(t, err)
}

func TestClient_HyDEExpand_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.HyDEExpand(context.Background(), &HyDERequest{Query: "test"})
	assert.Error(t, err)
}

func TestClient_DecomposeQuery_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.DecomposeQuery(context.Background(), &DecomposeQueryRequest{Query: "test"})
	assert.Error(t, err)
}

func TestClient_Rerank_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.Rerank(context.Background(), &RerankRequest{Query: "test", Documents: []string{"doc1"}})
	assert.Error(t, err)
}

// TestClient_QueryWithFilters tests query with metadata filters
func TestClient_QueryWithFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify filters were sent
		assert.NotNil(t, req.Filters)

		resp := &QueryResponse{
			Answer: "Filtered answer",
			Sources: []Source{
				{Content: "Filtered document", Score: 0.95},
			},
			Confidence: 0.9,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Query(context.Background(), &QueryRequest{
		Query: "test",
		Filters: map[string]interface{}{
			"category": "technical",
			"date":     "2024-01-01",
		},
		TopK: 5,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Answer)
}

// TestClient_ConcurrentQueries tests concurrent query handling
func TestClient_ConcurrentQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &QueryResponse{
			Answer:     "Concurrent answer",
			Sources:    []Source{{Content: "Source", Score: 0.9}},
			Confidence: 0.85,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := client.Query(ctx, &QueryRequest{
				Query: fmt.Sprintf("Query %d", idx),
				TopK:  5,
			})
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	var errCount int
	for range errors {
		errCount++
	}
	assert.Equal(t, 0, errCount)
}

// TestClient_QueryWithCognee tests query with Cognee integration
func TestClient_QueryWithCognee(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify Cognee flag
		assert.True(t, req.UseCognee)

		resp := &QueryResponse{
			Answer: "Cognee-enhanced answer",
			Sources: []Source{
				{Content: "Knowledge graph result", Score: 0.98, Metadata: map[string]interface{}{"source": "cognee"}},
			},
			Confidence: 0.95,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Query(context.Background(), &QueryRequest{
		Query:     "test with cognee",
		UseCognee: true,
		TopK:      5,
	})

	require.NoError(t, err)
	assert.Equal(t, "Cognee-enhanced answer", resp.Answer)
}

// TestClient_SourceMetadata tests source metadata handling
func TestClient_SourceMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &QueryResponse{
			Answer: "Answer with metadata",
			Sources: []Source{
				{
					Content: "Document content",
					Score:   0.9,
					Metadata: map[string]interface{}{
						"filename":   "doc.pdf",
						"page":       5,
						"section":    "Introduction",
						"created_at": "2024-01-15",
					},
				},
			},
			Confidence: 0.85,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Query(context.Background(), &QueryRequest{Query: "test", TopK: 5})

	require.NoError(t, err)
	assert.Len(t, resp.Sources, 1)
	assert.Equal(t, "doc.pdf", resp.Sources[0].Metadata["filename"])
	assert.Equal(t, float64(5), resp.Sources[0].Metadata["page"])
}

// TestClient_QueryAllTransformMethods tests all query transform methods
func TestClient_QueryAllTransformMethods(t *testing.T) {
	transforms := []string{"hyde", "decompose", "step_back"}

	for _, transform := range transforms {
		t.Run(transform, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req QueryRequest
				json.NewDecoder(r.Body).Decode(&req)
				assert.NotNil(t, req.QueryTransform)
				assert.Equal(t, transform, *req.QueryTransform)

				resp := &QueryResponse{
					Answer:           "Transformed answer",
					Sources:          []Source{{Content: "Source", Score: 0.9}},
					TransformedQuery: req.QueryTransform,
					Confidence:       0.8,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
			ctx := context.Background()

			var err error
			switch transform {
			case "hyde":
				_, err = client.QueryWithHyDE(ctx, "test", 5)
			case "decompose":
				_, err = client.QueryWithDecomposition(ctx, "test", 5)
			case "step_back":
				_, err = client.QueryWithStepBack(ctx, "test", 5)
			}

			assert.NoError(t, err)
		})
	}
}

// TestClient_RerankWithModel tests rerank with specific model
func TestClient_RerankWithModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RerankRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := &RerankResponse{
			RankedDocuments: []RankedDocument{
				{Content: req.Documents[0], Score: 0.95, Rank: 1},
				{Content: req.Documents[1], Score: 0.75, Rank: 2},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Rerank(context.Background(), &RerankRequest{
		Query:     "test query",
		Documents: []string{"Relevant document", "Less relevant doc"},
		TopK:      2,
	})

	require.NoError(t, err)
	assert.Len(t, resp.RankedDocuments, 2)
	assert.Equal(t, 1, resp.RankedDocuments[0].Rank)
	assert.Greater(t, resp.RankedDocuments[0].Score, resp.RankedDocuments[1].Score)
}

// TestClient_HyDEWithMultipleHypotheses tests HyDE with custom hypothesis count
func TestClient_HyDEWithMultipleHypotheses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req HyDERequest
		json.NewDecoder(r.Body).Decode(&req)

		hypotheses := make([]string, req.NumHypotheses)
		for i := range hypotheses {
			hypotheses[i] = fmt.Sprintf("Hypothesis %d about the query", i)
		}

		resp := &HyDEResponse{
			OriginalQuery:         req.Query,
			HypotheticalDocuments: hypotheses,
			CombinedEmbedding:     []float64{0.1, 0.2, 0.3},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.HyDEExpand(context.Background(), &HyDERequest{
		Query:         "complex query",
		NumHypotheses: 5,
	})

	require.NoError(t, err)
	assert.Len(t, resp.HypotheticalDocuments, 5)
}

// TestClient_QueryFusionWithCustomParams tests query fusion with custom parameters
func TestClient_QueryFusionWithCustomParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type fusionReq struct {
			Query         string `json:"query"`
			NumVariations int    `json:"num_variations"`
			TopK          int    `json:"top_k"`
		}

		var req fusionReq
		json.NewDecoder(r.Body).Decode(&req)

		variations := make([]string, req.NumVariations)
		for i := range variations {
			variations[i] = fmt.Sprintf("Variation %d of: %s", i, req.Query)
		}

		resp := &QueryFusionResponse{
			Query:          req.Query,
			VariationsUsed: variations,
			Results: []Source{
				{Content: "Fused result 1", Score: 0.95},
				{Content: "Fused result 2", Score: 0.85},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.QueryFusion(context.Background(), "test query", 5, 10)

	require.NoError(t, err)
	assert.Len(t, resp.VariationsUsed, 5)
	assert.Len(t, resp.Results, 2)
}

// TestClient_HealthCheckFields tests all health response fields
func TestClient_HealthCheckFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &HealthResponse{
			Status:              "healthy",
			Version:             "1.0.0",
			CogneeAvailable:     true,
			SuperagentAvailable: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	health, err := client.Health(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "1.0.0", health.Version)
	assert.True(t, health.CogneeAvailable)
	assert.True(t, health.SuperagentAvailable)
}

// TestClient_EmptyQueryResponse tests handling of empty query response
func TestClient_EmptyQueryResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &QueryResponse{
			Answer:     "",
			Sources:    []Source{},
			Confidence: 0.0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Query(context.Background(), &QueryRequest{Query: "test", TopK: 5})

	require.NoError(t, err)
	assert.Empty(t, resp.Answer)
	assert.Empty(t, resp.Sources)
}

// TestClient_QueryContextCancellation tests context cancellation handling
func TestClient_QueryContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		resp := &QueryResponse{Answer: "Should not reach"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	ctx, cancel := context.WithCancel(context.Background())

	// Start query in goroutine
	errChan := make(chan error, 1)
	go func() {
		_, err := client.Query(ctx, &QueryRequest{Query: "test", TopK: 5})
		errChan <- err
	}()

	// Cancel immediately
	cancel()

	err := <-errChan
	assert.Error(t, err)
}

// TestClient_MalformedJSONResponse tests handling of malformed JSON
func TestClient_MalformedJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.Query(context.Background(), &QueryRequest{Query: "test", TopK: 5})
	assert.Error(t, err)
}

// BenchmarkClient_Query benchmarks query performance
func BenchmarkClient_Query(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &QueryResponse{
			Answer:     "Benchmark answer",
			Sources:    []Source{{Content: "Source", Score: 0.9}},
			Confidence: 0.85,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Query(ctx, &QueryRequest{Query: "test", TopK: 5})
	}
}

// BenchmarkClient_Rerank benchmarks rerank performance
func BenchmarkClient_Rerank(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &RerankResponse{
			RankedDocuments: []RankedDocument{
				{Content: "Doc", Score: 0.9, Rank: 1},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	docs := make([]string, 100)
	for i := range docs {
		docs[i] = fmt.Sprintf("Document %d with some content", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Rerank(ctx, &RerankRequest{Query: "test", Documents: docs, TopK: 10})
	}
}
