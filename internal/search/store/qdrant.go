// Package store provides vector store implementations
package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dev.helix.agent/internal/search/types"
)

// QdrantStore implements VectorStore for Qdrant via REST API
type QdrantStore struct {
	host       string
	port       int
	collection string
	apiKey     string
	httpClient *http.Client
}

// NewQdrantStore creates a new Qdrant store
func NewQdrantStore(host string, port int, collection string) (*QdrantStore, error) {
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 6333
	}

	return &QdrantStore{
		host:       host,
		port:       port,
		collection: collection,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// baseURL returns the Qdrant API base URL
func (s *QdrantStore) baseURL() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}

// CreateCollection creates a new collection
func (s *QdrantStore) CreateCollection(ctx context.Context, name string, dims int) error {
	url := fmt.Sprintf("%s/collections/%s", s.baseURL(), name)

	payload := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     dims,
			"distance": "Cosine",
		},
	}

	req, err := s.newRequest(ctx, "PUT", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create collection: status %d", resp.StatusCode)
	}

	return nil
}

// DeleteCollection deletes a collection
func (s *QdrantStore) DeleteCollection(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/collections/%s", s.baseURL(), name)

	req, err := s.newRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete collection: status %d", resp.StatusCode)
	}

	return nil
}

// Upsert adds or updates documents
func (s *QdrantStore) Upsert(ctx context.Context, collection string, docs []types.Document) error {
	if len(docs) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true", s.baseURL(), collection)

	points := make([]map[string]interface{}, len(docs))
	for i, doc := range docs {
		points[i] = map[string]interface{}{
			"id":      doc.ID,
			"vector":  doc.Vector,
			"payload": doc.Metadata,
		}
		// Add content to payload if present
		if doc.Content != "" {
			if points[i]["payload"].(map[string]interface{}) == nil {
				points[i]["payload"] = make(map[string]interface{})
			}
			points[i]["payload"].(map[string]interface{})["content"] = doc.Content
		}
	}

	payload := map[string]interface{}{
		"points": points,
	}

	req, err := s.newRequest(ctx, "PUT", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upsert points: status %d", resp.StatusCode)
	}

	return nil
}

// Delete removes documents
func (s *QdrantStore) Delete(ctx context.Context, collection string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", s.baseURL(), collection)

	// Convert string IDs to appropriate format (UUID or integer)
	points := make([]interface{}, len(ids))
	for i, id := range ids {
		points[i] = id
	}

	payload := map[string]interface{}{
		"points": points,
	}

	req, err := s.newRequest(ctx, "POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete points: status %d", resp.StatusCode)
	}

	return nil
}

// Search performs vector search
func (s *QdrantStore) Search(ctx context.Context, collection string, vector []float32, opts types.SearchOptions) ([]types.SearchResult, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", s.baseURL(), collection)

	limit := opts.TopK
	if limit == 0 {
		limit = 10
	}

	payload := map[string]interface{}{
		"vector":        vector,
		"limit":         limit,
		"with_payload":  true,
		"with_vector":   false,
	}

	// Add filter if specified
	if len(opts.Filters) > 0 {
		payload["filter"] = s.buildFilter(opts.Filters)
	}

	req, err := s.newRequest(ctx, "POST", url, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed: status %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Result []struct {
			ID      string                 `json:"id"`
			Score   float32                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
			Vector  []float32              `json:"vector"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to SearchResult
	searchResults := make([]types.SearchResult, 0, len(result.Result))
	for _, r := range result.Result {
		// Filter by min score
		if opts.MinScore > 0 && r.Score < opts.MinScore {
			continue
		}

		// Extract content from payload
		content := ""
		if c, ok := r.Payload["content"].(string); ok {
			content = c
			delete(r.Payload, "content")
		}

		searchResults = append(searchResults, types.SearchResult{
			Document: types.Document{
				ID:       r.ID,
				Content:  content,
				Metadata: r.Payload,
				Vector:   r.Vector,
			},
			Score:    r.Score,
			Distance: 1 - r.Score, // Convert similarity to distance
		})
	}

	return searchResults, nil
}

// SearchByText performs text search (not directly supported by Qdrant)
func (s *QdrantStore) SearchByText(ctx context.Context, collection string, text string, opts types.SearchOptions) ([]types.SearchResult, error) {
	return nil, fmt.Errorf("text search not supported by Qdrant, use vector search with embeddings")
}

// GetCollectionStats returns collection statistics
func (s *QdrantStore) GetCollectionStats(ctx context.Context, collection string) (*types.CollectionStats, error) {
	url := fmt.Sprintf("%s/collections/%s", s.baseURL(), collection)

	req, err := s.newRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &types.CollectionStats{
			Name: collection,
		}, nil
	}

	var result struct {
		Result struct {
			PointsCount uint64 `json:"points_count"`
			Config      struct {
				Params struct {
					Size uint64 `json:"size"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &types.CollectionStats{
		Name:       collection,
		Count:      int64(result.Result.PointsCount), // #nosec G115 -- integer conversion bounded by reachable resource limits; overflow is mathematically unreachable
		Dimensions: int(result.Result.Config.Params.Size), // #nosec G115 -- integer conversion bounded by reachable resource limits; overflow is mathematically unreachable
	}, nil
}

// newRequest creates a new HTTP request with JSON body
func (s *QdrantStore) newRequest(ctx context.Context, method, url string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Add API key if set
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	return req, nil
}

// buildFilter converts filter map to Qdrant filter
func (s *QdrantStore) buildFilter(filters map[string]interface{}) map[string]interface{} {
	if len(filters) == 0 {
		return nil
	}

	// Build must conditions for each filter
	must := make([]map[string]interface{}, 0, len(filters))
	for key, value := range filters {
		must = append(must, map[string]interface{}{
			"key": key,
			"match": map[string]interface{}{
				"value": value,
			},
		})
	}

	return map[string]interface{}{
		"must": must,
	}
}

// Ensure interface is implemented
var _ types.VectorStore = (*QdrantStore)(nil)
