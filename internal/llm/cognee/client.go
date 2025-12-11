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

// AutoContainerize starts Cognee in a container if not running
func (c *Client) AutoContainerize() error {
	// Check if Cognee is already running by testing connection
	if c.testConnection() {
		return nil // Already running
	}

	// Try to start Cognee container
	return c.startCogneeContainer()
}

// testConnection checks if Cognee is already running
func (c *Client) testConnection() bool {
	url := fmt.Sprintf("%s/health", c.baseURL)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// startCogneeContainer starts Cognee using Docker
func (c *Client) startCogneeContainer() error {
	// For now, return a placeholder error
	// In production, this would use Docker SDK to start a container
	return fmt.Errorf("Cognee is not running and auto-containerization is not implemented. Please start Cognee manually")
}
