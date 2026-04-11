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

// ChromaStore implements VectorStore for ChromaDB via REST API
type ChromaStore struct {
	host       string
	port       int
	collection string
	httpClient *http.Client
}

// NewChromaStore creates a new ChromaDB store
func NewChromaStore(host string, port int, collectionName string) (*ChromaStore, error) {
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 8000
	}

	return &ChromaStore{
		host:       host,
		port:       port,
		collection: collectionName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// baseURL returns the ChromaDB API base URL
func (s *ChromaStore) baseURL() string {
	return fmt.Sprintf("http://%s:%d/api/v1", s.host, s.port)
}

// CreateCollection creates a new collection
func (s *ChromaStore) CreateCollection(ctx context.Context, name string, dims int) error {
	url := fmt.Sprintf("%s/collections", s.baseURL())

	payload := map[string]interface{}{
		"name": name,
		"metadata": map[string]interface{}{
			"dimension":  dims,
			"hnsw:space": "cosine",
			"created_by": "helixagent",
			"created_at": time.Now().Unix(),
		},
	}

	req, err := s.newRequest(ctx, "POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create collection: status %d", resp.StatusCode)
	}

	return nil
}

// DeleteCollection deletes a collection
func (s *ChromaStore) DeleteCollection(ctx context.Context, name string) error {
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete collection: status %d", resp.StatusCode)
	}

	return nil
}

// Upsert adds or updates documents
func (s *ChromaStore) Upsert(ctx context.Context, collection string, docs []types.Document) error {
	if len(docs) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/collections/%s/upsert", s.baseURL(), collection)

	ids := make([]string, len(docs))
	embeddings := make([][]float32, len(docs))
	metadatas := make([]map[string]interface{}, len(docs))
	documents := make([]string, len(docs))

	for i, doc := range docs {
		ids[i] = doc.ID
		embeddings[i] = doc.Vector
		metadatas[i] = doc.Metadata
		documents[i] = doc.Content
	}

	payload := map[string]interface{}{
		"ids":        ids,
		"embeddings": embeddings,
		"metadatas":  metadatas,
		"documents":  documents,
	}

	req, err := s.newRequest(ctx, "POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upsert documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to upsert documents: status %d", resp.StatusCode)
	}

	return nil
}

// Delete removes documents
func (s *ChromaStore) Delete(ctx context.Context, collection string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/collections/%s/delete", s.baseURL(), collection)

	payload := map[string]interface{}{
		"ids": ids,
	}

	req, err := s.newRequest(ctx, "POST", url, payload)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete documents: status %d", resp.StatusCode)
	}

	return nil
}

// Search performs vector search
func (s *ChromaStore) Search(ctx context.Context, collection string, vector []float32, opts types.SearchOptions) ([]types.SearchResult, error) {
	url := fmt.Sprintf("%s/collections/%s/query", s.baseURL(), collection)

	nResults := opts.TopK
	if nResults == 0 {
		nResults = 10
	}

	payload := map[string]interface{}{
		"query_embeddings": [][]float32{vector},
		"n_results":        nResults,
		"include":          []string{"metadatas", "documents", "distances"},
	}

	// Add filters if specified
	if len(opts.Filters) > 0 {
		payload["where"] = s.buildFilter(opts.Filters)
	}

	req, err := s.newRequest(ctx, "POST", url, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query failed: status %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		IDs       [][]string                 `json:"ids"`
		Distances [][]float32                `json:"distances"`
		Documents [][]string                 `json:"documents"`
		Metadatas [][]map[string]interface{} `json:"metadatas"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to SearchResult
	var searchResults []types.SearchResult
	if len(result.IDs) > 0 && len(result.IDs[0]) > 0 {
		for i, id := range result.IDs[0] {
			distance := float32(0)
			if len(result.Distances) > 0 && len(result.Distances[0]) > i {
				distance = result.Distances[0][i]
			}

			content := ""
			if len(result.Documents) > 0 && len(result.Documents[0]) > i {
				content = result.Documents[0][i]
			}

			metadata := make(map[string]interface{})
			if len(result.Metadatas) > 0 && len(result.Metadatas[0]) > i {
				metadata = result.Metadatas[0][i]
			}

			// Convert distance to similarity score (Chroma returns distances, we want scores)
			// Cosine distance to similarity: score = 1 - distance
			score := float32(1.0) - distance

			// Filter by min score
			if opts.MinScore > 0 && score < opts.MinScore {
				continue
			}

			searchResults = append(searchResults, types.SearchResult{
				Document: types.Document{
					ID:       id,
					Content:  content,
					Metadata: metadata,
				},
				Score:    score,
				Distance: distance,
			})
		}
	}

	return searchResults, nil
}

// SearchByText performs text search (not supported by Chroma directly)
func (s *ChromaStore) SearchByText(ctx context.Context, collection string, text string, opts types.SearchOptions) ([]types.SearchResult, error) {
	return nil, fmt.Errorf("text search not supported by ChromaDB, use vector search with embeddings")
}

// GetCollectionStats returns collection statistics
func (s *ChromaStore) GetCollectionStats(ctx context.Context, collection string) (*types.CollectionStats, error) {
	url := fmt.Sprintf("%s/collections/%s/count", s.baseURL(), collection)

	req, err := s.newRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get count: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &types.CollectionStats{
			Name: collection,
		}, nil
	}

	var result struct {
		Count int64 `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &types.CollectionStats{
		Name:  collection,
		Count: result.Count,
	}, nil
}

// newRequest creates a new HTTP request with JSON body
func (s *ChromaStore) newRequest(ctx context.Context, method, url string, body interface{}) (*http.Request, error) {
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

	return req, nil
}

// buildFilter converts filter map to ChromaDB where clause
func (s *ChromaStore) buildFilter(filters map[string]interface{}) map[string]interface{} {
	if len(filters) == 0 {
		return nil
	}

	// Simple equality filters
	andConditions := make([]map[string]interface{}, 0, len(filters))
	for key, value := range filters {
		andConditions = append(andConditions, map[string]interface{}{
			key: map[string]interface{}{"$eq": value},
		})
	}

	if len(andConditions) == 1 {
		return andConditions[0]
	}

	return map[string]interface{}{
		"$and": andConditions,
	}
}

// Ensure interface is implemented
var _ types.VectorStore = (*ChromaStore)(nil)
