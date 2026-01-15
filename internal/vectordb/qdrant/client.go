package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Client provides an interface to interact with Qdrant vector database
type Client struct {
	config     *Config
	httpClient *http.Client
	logger     *logrus.Logger
	mu         sync.RWMutex
	connected  bool
}

// NewClient creates a new Qdrant client
func NewClient(config *Config, logger *logrus.Logger) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger:    logger,
		connected: false,
	}, nil
}

// Connect verifies connectivity to Qdrant
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.healthCheckLocked(ctx); err != nil {
		return fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	c.connected = true
	c.logger.Info("Connected to Qdrant")
	return nil
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// HealthCheck checks the health of Qdrant
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthCheckLocked(ctx)
}

func (c *Client) healthCheckLocked(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.config.GetHTTPURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.config.APIKey != "" {
		req.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.config.GetHTTPURL(), path)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// CollectionInfo represents information about a collection
type CollectionInfo struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	VectorCount  int64  `json:"vectors_count"`
	PointsCount  int64  `json:"points_count"`
	SegmentsCount int   `json:"segments_count"`
}

// CreateCollection creates a new vector collection
func (c *Client) CreateCollection(ctx context.Context, config *CollectionConfig) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Qdrant")
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid collection config: %w", err)
	}

	reqBody := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     config.VectorSize,
			"distance": string(config.Distance),
		},
	}

	if config.OnDiskPayload {
		reqBody["on_disk_payload"] = true
	}

	if config.IndexingThreshold > 0 {
		reqBody["optimizers_config"] = map[string]interface{}{
			"indexing_threshold": config.IndexingThreshold,
		}
	}

	if config.ShardNumber > 1 {
		reqBody["shard_number"] = config.ShardNumber
	}

	if config.ReplicationFactor > 1 {
		reqBody["replication_factor"] = config.ReplicationFactor
	}

	path := fmt.Sprintf("/collections/%s", config.Name)
	_, err := c.doRequest(ctx, http.MethodPut, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	c.logger.WithField("collection", config.Name).Info("Collection created")
	return nil
}

// DeleteCollection deletes a collection
func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Qdrant")
	}

	path := fmt.Sprintf("/collections/%s", name)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	c.logger.WithField("collection", name).Info("Collection deleted")
	return nil
}

// CollectionExists checks if a collection exists
func (c *Client) CollectionExists(ctx context.Context, name string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return false, fmt.Errorf("not connected to Qdrant")
	}

	path := fmt.Sprintf("/collections/%s", name)
	_, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		// Collection doesn't exist
		return false, nil
	}

	return true, nil
}

// ListCollections returns all collections
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Qdrant")
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, "/collections", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	var response struct {
		Result struct {
			Collections []struct {
				Name string `json:"name"`
			} `json:"collections"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	names := make([]string, len(response.Result.Collections))
	for i, col := range response.Result.Collections {
		names[i] = col.Name
	}

	return names, nil
}

// GetCollectionInfo returns information about a collection
func (c *Client) GetCollectionInfo(ctx context.Context, name string) (*CollectionInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Qdrant")
	}

	path := fmt.Sprintf("/collections/%s", name)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}

	var response struct {
		Result struct {
			Status         string `json:"status"`
			VectorsCount   int64  `json:"vectors_count"`
			PointsCount    int64  `json:"points_count"`
			SegmentsCount  int    `json:"segments_count"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &CollectionInfo{
		Name:          name,
		Status:        response.Result.Status,
		VectorCount:   response.Result.VectorsCount,
		PointsCount:   response.Result.PointsCount,
		SegmentsCount: response.Result.SegmentsCount,
	}, nil
}

// Point represents a vector point in Qdrant
type Point struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// ScoredPoint represents a search result with score
type ScoredPoint struct {
	ID      string                 `json:"id"`
	Version int                    `json:"version"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Vector  []float32              `json:"vector,omitempty"`
}

// UpsertPoints inserts or updates points in a collection
func (c *Client) UpsertPoints(ctx context.Context, collection string, points []Point) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Qdrant")
	}

	if len(points) == 0 {
		return nil
	}

	// Ensure all points have IDs
	for i := range points {
		if points[i].ID == "" {
			points[i].ID = uuid.New().String()
		}
	}

	reqBody := map[string]interface{}{
		"points": points,
	}

	path := fmt.Sprintf("/collections/%s/points", collection)
	_, err := c.doRequest(ctx, http.MethodPut, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"collection": collection,
		"count":      len(points),
	}).Debug("Points upserted")

	return nil
}

// DeletePoints deletes points by IDs
func (c *Client) DeletePoints(ctx context.Context, collection string, ids []string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Qdrant")
	}

	if len(ids) == 0 {
		return nil
	}

	reqBody := map[string]interface{}{
		"points": ids,
	}

	path := fmt.Sprintf("/collections/%s/points/delete", collection)
	_, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"collection": collection,
		"count":      len(ids),
	}).Debug("Points deleted")

	return nil
}

// Search performs a vector similarity search
func (c *Client) Search(ctx context.Context, collection string, vector []float32, opts *SearchOptions) ([]ScoredPoint, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Qdrant")
	}

	if opts == nil {
		opts = DefaultSearchOptions()
	}

	reqBody := map[string]interface{}{
		"vector":       vector,
		"limit":        opts.Limit,
		"offset":       opts.Offset,
		"with_payload": opts.WithPayload,
		"with_vector":  opts.WithVectors,
	}

	if opts.ScoreThreshold > 0 {
		reqBody["score_threshold"] = opts.ScoreThreshold
	}

	if opts.Filter != nil {
		reqBody["filter"] = opts.Filter
	}

	path := fmt.Sprintf("/collections/%s/points/search", collection)
	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	var response struct {
		Result []ScoredPoint `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Result, nil
}

// SearchBatch performs multiple searches in a single request
func (c *Client) SearchBatch(ctx context.Context, collection string, vectors [][]float32, opts *SearchOptions) ([][]ScoredPoint, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Qdrant")
	}

	if opts == nil {
		opts = DefaultSearchOptions()
	}

	searches := make([]map[string]interface{}, len(vectors))
	for i, vector := range vectors {
		searches[i] = map[string]interface{}{
			"vector":       vector,
			"limit":        opts.Limit,
			"with_payload": opts.WithPayload,
			"with_vector":  opts.WithVectors,
		}
		if opts.ScoreThreshold > 0 {
			searches[i]["score_threshold"] = opts.ScoreThreshold
		}
		if opts.Filter != nil {
			searches[i]["filter"] = opts.Filter
		}
	}

	reqBody := map[string]interface{}{
		"searches": searches,
	}

	path := fmt.Sprintf("/collections/%s/points/search/batch", collection)
	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to batch search: %w", err)
	}

	var response struct {
		Result []struct {
			Result []ScoredPoint `json:"result"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	results := make([][]ScoredPoint, len(response.Result))
	for i, r := range response.Result {
		results[i] = r.Result
	}

	return results, nil
}

// GetPoint retrieves a single point by ID
func (c *Client) GetPoint(ctx context.Context, collection, id string) (*Point, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Qdrant")
	}

	path := fmt.Sprintf("/collections/%s/points/%s", collection, id)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get point: %w", err)
	}

	var response struct {
		Result struct {
			ID      string                 `json:"id"`
			Vector  []float32              `json:"vector"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &Point{
		ID:      response.Result.ID,
		Vector:  response.Result.Vector,
		Payload: response.Result.Payload,
	}, nil
}

// GetPoints retrieves multiple points by IDs
func (c *Client) GetPoints(ctx context.Context, collection string, ids []string) ([]Point, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Qdrant")
	}

	reqBody := map[string]interface{}{
		"ids":          ids,
		"with_payload": true,
		"with_vector":  true,
	}

	path := fmt.Sprintf("/collections/%s/points", collection)
	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get points: %w", err)
	}

	var response struct {
		Result []struct {
			ID      string                 `json:"id"`
			Vector  []float32              `json:"vector"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	points := make([]Point, len(response.Result))
	for i, r := range response.Result {
		points[i] = Point{
			ID:      r.ID,
			Vector:  r.Vector,
			Payload: r.Payload,
		}
	}

	return points, nil
}

// Scroll retrieves points with pagination
func (c *Client) Scroll(ctx context.Context, collection string, limit int, offset *string, filter map[string]interface{}) ([]Point, *string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, nil, fmt.Errorf("not connected to Qdrant")
	}

	reqBody := map[string]interface{}{
		"limit":        limit,
		"with_payload": true,
		"with_vector":  true,
	}

	if offset != nil {
		reqBody["offset"] = *offset
	}

	if filter != nil {
		reqBody["filter"] = filter
	}

	path := fmt.Sprintf("/collections/%s/points/scroll", collection)
	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scroll points: %w", err)
	}

	var response struct {
		Result struct {
			NextPageOffset *string `json:"next_page_offset"`
			Points         []struct {
				ID      string                 `json:"id"`
				Vector  []float32              `json:"vector"`
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	points := make([]Point, len(response.Result.Points))
	for i, r := range response.Result.Points {
		points[i] = Point{
			ID:      r.ID,
			Vector:  r.Vector,
			Payload: r.Payload,
		}
	}

	return points, response.Result.NextPageOffset, nil
}

// CountPoints returns the number of points in a collection
func (c *Client) CountPoints(ctx context.Context, collection string, filter map[string]interface{}) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return 0, fmt.Errorf("not connected to Qdrant")
	}

	reqBody := map[string]interface{}{
		"exact": true,
	}

	if filter != nil {
		reqBody["filter"] = filter
	}

	path := fmt.Sprintf("/collections/%s/points/count", collection)
	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return 0, fmt.Errorf("failed to count points: %w", err)
	}

	var response struct {
		Result struct {
			Count int64 `json:"count"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Result.Count, nil
}

// CreateSnapshot creates a snapshot of a collection
func (c *Client) CreateSnapshot(ctx context.Context, collection string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return "", fmt.Errorf("not connected to Qdrant")
	}

	path := fmt.Sprintf("/collections/%s/snapshots", collection)
	respBody, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}

	var response struct {
		Result struct {
			Name string `json:"name"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"collection": collection,
		"snapshot":   response.Result.Name,
	}).Info("Snapshot created")

	return response.Result.Name, nil
}

// GetMetrics returns Qdrant telemetry/metrics
func (c *Client) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Qdrant")
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, "/telemetry", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	var response struct {
		Result map[string]interface{} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Result, nil
}

// WaitForCollection waits for a collection to be ready
func (c *Client) WaitForCollection(ctx context.Context, collection string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for collection %s to be ready", collection)
			}

			info, err := c.GetCollectionInfo(ctx, collection)
			if err != nil {
				c.logger.WithError(err).Debug("Collection not ready yet")
				continue
			}

			if info.Status == "green" {
				return nil
			}
		}
	}
}
