// Package llamaindex provides HTTP client for the LlamaIndex service.
package llamaindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the LlamaIndex service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ClientConfig holds configuration for the LlamaIndex client.
type ClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL: "http://localhost:8012",
		Timeout: 120 * time.Second,
	}
}

// NewClient creates a new LlamaIndex client.
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	return &Client{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// QueryRequest represents a document query request.
type QueryRequest struct {
	Query          string                 `json:"query"`
	TopK           int                    `json:"top_k,omitempty"`
	UseCognee      bool                   `json:"use_cognee,omitempty"`
	Rerank         bool                   `json:"rerank,omitempty"`
	QueryTransform *string                `json:"query_transform,omitempty"`
	Filters        map[string]interface{} `json:"filters,omitempty"`
}

// Source represents a document source.
type Source struct {
	Content  string                 `json:"content"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// QueryResponse represents the query result.
type QueryResponse struct {
	Answer           string   `json:"answer"`
	Sources          []Source `json:"sources"`
	TransformedQuery *string  `json:"transformed_query,omitempty"`
	Confidence       float64  `json:"confidence"`
}

// HyDERequest represents a HyDE expansion request.
type HyDERequest struct {
	Query         string `json:"query"`
	NumHypotheses int    `json:"num_hypotheses,omitempty"`
}

// HyDEResponse represents the HyDE expansion result.
type HyDEResponse struct {
	OriginalQuery         string    `json:"original_query"`
	HypotheticalDocuments []string  `json:"hypothetical_documents"`
	CombinedEmbedding     []float64 `json:"combined_embedding"`
}

// DecomposeQueryRequest represents a query decomposition request.
type DecomposeQueryRequest struct {
	Query        string `json:"query"`
	MaxSubqueries int   `json:"max_subqueries,omitempty"`
}

// DecomposeQueryResponse represents the query decomposition result.
type DecomposeQueryResponse struct {
	OriginalQuery string   `json:"original_query"`
	Subqueries    []string `json:"subqueries"`
	Reasoning     string   `json:"reasoning"`
}

// RerankRequest represents a reranking request.
type RerankRequest struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopK      int      `json:"top_k,omitempty"`
}

// RankedDocument represents a reranked document.
type RankedDocument struct {
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	Rank    int     `json:"rank"`
}

// RerankResponse represents the reranking result.
type RerankResponse struct {
	RankedDocuments []RankedDocument `json:"ranked_documents"`
}

// QueryFusionResponse represents query fusion results.
type QueryFusionResponse struct {
	Query          string   `json:"query"`
	VariationsUsed []string `json:"variations_used"`
	Results        []Source `json:"results"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status             string `json:"status"`
	Version            string `json:"version"`
	CogneeAvailable    bool   `json:"cognee_available"`
	SuperagentAvailable bool  `json:"superagent_available"`
}

// Health checks the service health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// Query queries documents and generates an answer.
func (c *Client) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	if req.TopK == 0 {
		req.TopK = 5
	}

	resp, err := c.doRequest(ctx, "POST", "/query", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// QueryWithHyDE queries using HyDE (Hypothetical Document Embeddings).
func (c *Client) QueryWithHyDE(ctx context.Context, query string, topK int) (*QueryResponse, error) {
	transform := "hyde"
	return c.Query(ctx, &QueryRequest{
		Query:          query,
		TopK:           topK,
		UseCognee:      true,
		Rerank:         true,
		QueryTransform: &transform,
	})
}

// QueryWithDecomposition queries using query decomposition.
func (c *Client) QueryWithDecomposition(ctx context.Context, query string, topK int) (*QueryResponse, error) {
	transform := "decompose"
	return c.Query(ctx, &QueryRequest{
		Query:          query,
		TopK:           topK,
		UseCognee:      true,
		Rerank:         true,
		QueryTransform: &transform,
	})
}

// QueryWithStepBack queries using step-back prompting.
func (c *Client) QueryWithStepBack(ctx context.Context, query string, topK int) (*QueryResponse, error) {
	transform := "step_back"
	return c.Query(ctx, &QueryRequest{
		Query:          query,
		TopK:           topK,
		UseCognee:      true,
		Rerank:         true,
		QueryTransform: &transform,
	})
}

// HyDEExpand generates hypothetical documents for query expansion.
func (c *Client) HyDEExpand(ctx context.Context, req *HyDERequest) (*HyDEResponse, error) {
	if req.NumHypotheses == 0 {
		req.NumHypotheses = 3
	}

	resp, err := c.doRequest(ctx, "POST", "/hyde", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result HyDEResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// DecomposeQuery decomposes a complex query into simpler subqueries.
func (c *Client) DecomposeQuery(ctx context.Context, req *DecomposeQueryRequest) (*DecomposeQueryResponse, error) {
	if req.MaxSubqueries == 0 {
		req.MaxSubqueries = 3
	}

	resp, err := c.doRequest(ctx, "POST", "/decompose", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result DecomposeQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// Rerank reranks documents based on query relevance.
func (c *Client) Rerank(ctx context.Context, req *RerankRequest) (*RerankResponse, error) {
	if req.TopK == 0 {
		req.TopK = 5
	}

	resp, err := c.doRequest(ctx, "POST", "/rerank", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// QueryFusion performs query fusion with reciprocal rank fusion.
func (c *Client) QueryFusion(ctx context.Context, query string, numVariations, topK int) (*QueryFusionResponse, error) {
	if numVariations == 0 {
		numVariations = 3
	}
	if topK == 0 {
		topK = 5
	}

	type fusionReq struct {
		Query         string `json:"query"`
		NumVariations int    `json:"num_variations"`
		TopK          int    `json:"top_k"`
	}

	resp, err := c.doRequest(ctx, "POST", "/query_fusion", &fusionReq{
		Query:         query,
		NumVariations: numVariations,
		TopK:          topK,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result QueryFusionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

// IsAvailable checks if the service is available.
func (c *Client) IsAvailable(ctx context.Context) bool {
	health, err := c.Health(ctx)
	return err == nil && health.Status == "healthy"
}
