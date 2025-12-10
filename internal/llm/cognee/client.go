package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
)

type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type MemoryRequest struct {
	Content     string `json:"content"`
	DatasetName string `json:"dataset_name"`
	ContentType string `json:"content_type"`
}

type MemoryResponse struct {
	VectorID   string                 `json:"vector_id"`
	GraphNodes map[string]interface{} `json:"graph_nodes"`
}

type SearchRequest struct {
	Query       string `json:"query"`
	DatasetName string `json:"dataset_name"`
	Limit       int    `json:"limit"`
}

type SearchResponse struct {
	Results []models.MemorySource `json:"results"`
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL: cfg.Cognee.BaseURL,
		apiKey:  cfg.Cognee.APIKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) AddMemory(req *MemoryRequest) (*MemoryResponse, error) {
	url := fmt.Sprintf("%s/api/memory", c.baseURL)

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cognee API error: %s", resp.Status)
	}

	var response MemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *Client) SearchMemory(req *SearchRequest) (*SearchResponse, error) {
	url := fmt.Sprintf("%s/api/search", c.baseURL)

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cognee API error: %s", resp.Status)
	}

	var response SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetBaseURL returns the base URL of the client
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// AutoContainerize starts Cognee in a container if not running.
// This is a placeholder for auto-containerization logic.
func (c *Client) AutoContainerize() error {
	// TODO: Implement Docker container management
	// Check if Cognee is running, start container if not
	return nil
}
