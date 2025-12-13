// Package chutes provides a Go client for the Chutes API.
package chutes

import (
	"context"
	"time"

	"github.com/superagent/toolkit/pkg/toolkit"
	"github.com/superagent/toolkit/pkg/toolkit/common/http"
)

// Client represents a Chutes API client.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Chutes API client.
func NewClient(apiKey string, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.chutes.ai/v1"
	}
	httpClient := http.NewClient(http.ClientConfig{
		BaseURL:    baseURL,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	})
	httpClient.SetAuth("Authorization", "Bearer "+apiKey)

	return &Client{
		httpClient: httpClient,
	}
}

// ChatCompletion performs a chat completion request.
func (c *Client) ChatCompletion(ctx context.Context, req toolkit.ChatRequest) (toolkit.ChatResponse, error) {
	payload := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
	}

	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	if req.TopP > 0 {
		payload["top_p"] = req.TopP
	}
	if req.TopK > 0 {
		payload["top_k"] = req.TopK
	}
	if len(req.Stop) > 0 {
		payload["stop"] = req.Stop
	}
	if req.PresencePenalty != 0 {
		payload["presence_penalty"] = req.PresencePenalty
	}
	if req.FrequencyPenalty != 0 {
		payload["frequency_penalty"] = req.FrequencyPenalty
	}
	if len(req.LogitBias) > 0 {
		payload["logit_bias"] = req.LogitBias
	}

	var response toolkit.ChatResponse
	err := c.doRequest(ctx, "POST", "/chat/completions", payload, &response)
	return response, err
}

// CreateEmbeddings performs an embedding request.
func (c *Client) CreateEmbeddings(ctx context.Context, req toolkit.EmbeddingRequest) (toolkit.EmbeddingResponse, error) {
	payload := map[string]interface{}{
		"model": req.Model,
		"input": req.Input,
	}

	if req.EncodingFormat != "" {
		payload["encoding_format"] = req.EncodingFormat
	}
	if req.Dimensions > 0 {
		payload["dimensions"] = req.Dimensions
	}
	if req.User != "" {
		payload["user"] = req.User
	}

	var response toolkit.EmbeddingResponse
	err := c.doRequest(ctx, "POST", "/embeddings", payload, &response)
	return response, err
}

// CreateRerank performs a rerank request.
func (c *Client) CreateRerank(ctx context.Context, req toolkit.RerankRequest) (toolkit.RerankResponse, error) {
	payload := map[string]interface{}{
		"model":            req.Model,
		"query":            req.Query,
		"documents":        req.Documents,
		"top_n":            req.TopN,
		"return_documents": req.ReturnDocs,
	}

	var response toolkit.RerankResponse
	err := c.doRequest(ctx, "POST", "/rerank", payload, &response)
	return response, err
}

// GetModels retrieves available models from the API.
func (c *Client) GetModels(ctx context.Context) ([]ModelInfo, error) {
	var response struct {
		Data []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	}

	err := c.doRequest(ctx, "GET", "/models", nil, &response)
	if err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, model := range response.Data {
		models = append(models, ModelInfo{
			ID:   model.ID,
			Type: model.Type,
		})
	}

	return models, nil
}

// ModelInfo represents basic model information from the API.
type ModelInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// doRequest performs an HTTP request to the Chutes API.
func (c *Client) doRequest(ctx context.Context, method, endpoint string, payload interface{}, result interface{}) error {
	return c.httpClient.DoRequest(ctx, method, endpoint, payload, result)
}
