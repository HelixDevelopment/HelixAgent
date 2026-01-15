package iceberg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Client provides an interface to interact with Apache Iceberg REST Catalog
type Client struct {
	config     *Config
	httpClient *http.Client
	logger     *logrus.Logger
	mu         sync.RWMutex
	connected  bool
}

// NewClient creates a new Iceberg REST Catalog client
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

// Connect verifies connectivity to the Iceberg catalog
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.healthCheckLocked(ctx); err != nil {
		return fmt.Errorf("failed to connect to Iceberg catalog: %w", err)
	}

	c.connected = true
	c.logger.Info("Connected to Iceberg catalog")
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

// HealthCheck checks the health of the catalog
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthCheckLocked(ctx)
}

func (c *Client) healthCheckLocked(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/config", c.config.CatalogURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.config.CatalogURI, path)

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

// CatalogConfig represents the catalog configuration
type CatalogConfig struct {
	Defaults   map[string]string `json:"defaults"`
	Overrides  map[string]string `json:"overrides"`
}

// GetCatalogConfig returns the catalog configuration
func (c *Client) GetCatalogConfig(ctx context.Context) (*CatalogConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Iceberg catalog")
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, "/v1/config", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get catalog config: %w", err)
	}

	var config CatalogConfig
	if err := json.Unmarshal(respBody, &config); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &config, nil
}

// Namespace represents an Iceberg namespace
type Namespace struct {
	Name       []string          `json:"namespace"`
	Properties map[string]string `json:"properties,omitempty"`
}

// CreateNamespace creates a new namespace
func (c *Client) CreateNamespace(ctx context.Context, name string, properties map[string]string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Iceberg catalog")
	}

	reqBody := map[string]interface{}{
		"namespace": []string{name},
	}
	if properties != nil {
		reqBody["properties"] = properties
	}

	_, err := c.doRequest(ctx, http.MethodPost, "/v1/namespaces", reqBody)
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	c.logger.WithField("namespace", name).Info("Namespace created")
	return nil
}

// ListNamespaces returns all namespaces
func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Iceberg catalog")
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, "/v1/namespaces", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	var response struct {
		Namespaces [][]string `json:"namespaces"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	names := make([]string, len(response.Namespaces))
	for i, ns := range response.Namespaces {
		if len(ns) > 0 {
			names[i] = ns[0]
		}
	}

	return names, nil
}

// GetNamespace returns namespace properties
func (c *Client) GetNamespace(ctx context.Context, name string) (*Namespace, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Iceberg catalog")
	}

	path := fmt.Sprintf("/v1/namespaces/%s", name)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace: %w", err)
	}

	var ns Namespace
	if err := json.Unmarshal(respBody, &ns); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &ns, nil
}

// DropNamespace deletes a namespace
func (c *Client) DropNamespace(ctx context.Context, name string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Iceberg catalog")
	}

	path := fmt.Sprintf("/v1/namespaces/%s", name)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("failed to drop namespace: %w", err)
	}

	c.logger.WithField("namespace", name).Info("Namespace dropped")
	return nil
}

// TableIdentifier represents a table identifier
type TableIdentifier struct {
	Namespace []string `json:"namespace"`
	Name      string   `json:"name"`
}

// TableMetadata represents Iceberg table metadata
type TableMetadata struct {
	FormatVersion    int               `json:"format-version"`
	TableUUID        string            `json:"table-uuid"`
	Location         string            `json:"location"`
	LastUpdatedMs    int64             `json:"last-updated-ms"`
	LastColumnID     int               `json:"last-column-id"`
	Schema           *Schema           `json:"schema"`
	CurrentSchemaID  int               `json:"current-schema-id"`
	Schemas          []Schema          `json:"schemas"`
	PartitionSpec    []PartitionField  `json:"partition-spec"`
	DefaultSpecID    int               `json:"default-spec-id"`
	PartitionSpecs   []interface{}     `json:"partition-specs"`
	LastPartitionID  int               `json:"last-partition-id"`
	DefaultSortOrderID int             `json:"default-sort-order-id"`
	SortOrders       []interface{}     `json:"sort-orders"`
	Properties       map[string]string `json:"properties"`
	CurrentSnapshotID *int64           `json:"current-snapshot-id"`
	Snapshots        []Snapshot        `json:"snapshots"`
	SnapshotLog      []SnapshotLogEntry `json:"snapshot-log"`
	MetadataLog      []MetadataLogEntry `json:"metadata-log"`
}

// Snapshot represents an Iceberg snapshot
type Snapshot struct {
	SnapshotID        int64             `json:"snapshot-id"`
	ParentSnapshotID  *int64            `json:"parent-snapshot-id,omitempty"`
	SequenceNumber    int64             `json:"sequence-number"`
	TimestampMs       int64             `json:"timestamp-ms"`
	ManifestList      string            `json:"manifest-list"`
	Summary           map[string]string `json:"summary"`
	SchemaID          *int              `json:"schema-id,omitempty"`
}

// SnapshotLogEntry represents a snapshot log entry
type SnapshotLogEntry struct {
	TimestampMs int64 `json:"timestamp-ms"`
	SnapshotID  int64 `json:"snapshot-id"`
}

// MetadataLogEntry represents a metadata log entry
type MetadataLogEntry struct {
	TimestampMs     int64  `json:"timestamp-ms"`
	MetadataFile    string `json:"metadata-file"`
}

// CreateTable creates a new table
func (c *Client) CreateTable(ctx context.Context, config *TableConfig) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Iceberg catalog")
	}

	reqBody := map[string]interface{}{
		"name": config.Name,
	}

	if config.Schema != nil {
		reqBody["schema"] = config.Schema
	}

	if len(config.PartitionSpec) > 0 {
		reqBody["partition-spec"] = config.PartitionSpec
	}

	if len(config.SortOrder) > 0 {
		reqBody["write-order"] = config.SortOrder
	}

	if config.Properties != nil {
		reqBody["properties"] = config.Properties
	}

	path := fmt.Sprintf("/v1/namespaces/%s/tables", config.Namespace)
	_, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"namespace": config.Namespace,
		"table":     config.Name,
	}).Info("Table created")

	return nil
}

// ListTables returns all tables in a namespace
func (c *Client) ListTables(ctx context.Context, namespace string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Iceberg catalog")
	}

	path := fmt.Sprintf("/v1/namespaces/%s/tables", namespace)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	var response struct {
		Identifiers []TableIdentifier `json:"identifiers"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	names := make([]string, len(response.Identifiers))
	for i, id := range response.Identifiers {
		names[i] = id.Name
	}

	return names, nil
}

// GetTable returns table metadata
func (c *Client) GetTable(ctx context.Context, namespace, name string) (*TableMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to Iceberg catalog")
	}

	path := fmt.Sprintf("/v1/namespaces/%s/tables/%s", namespace, name)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get table: %w", err)
	}

	var response struct {
		Metadata TableMetadata `json:"metadata"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response.Metadata, nil
}

// DropTable deletes a table
func (c *Client) DropTable(ctx context.Context, namespace, name string, purge bool) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Iceberg catalog")
	}

	path := fmt.Sprintf("/v1/namespaces/%s/tables/%s", namespace, name)
	if purge {
		path += "?purgeRequested=true"
	}

	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"table":     name,
		"purge":     purge,
	}).Info("Table dropped")

	return nil
}

// TableExists checks if a table exists
func (c *Client) TableExists(ctx context.Context, namespace, name string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return false, fmt.Errorf("not connected to Iceberg catalog")
	}

	path := fmt.Sprintf("/v1/namespaces/%s/tables/%s", namespace, name)
	_, err := c.doRequest(ctx, http.MethodHead, path, nil)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// RenameTable renames a table
func (c *Client) RenameTable(ctx context.Context, namespace, oldName, newName string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Iceberg catalog")
	}

	reqBody := map[string]interface{}{
		"source": map[string]interface{}{
			"namespace": []string{namespace},
			"name":      oldName,
		},
		"destination": map[string]interface{}{
			"namespace": []string{namespace},
			"name":      newName,
		},
	}

	_, err := c.doRequest(ctx, http.MethodPost, "/v1/tables/rename", reqBody)
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"old_name":  oldName,
		"new_name":  newName,
	}).Info("Table renamed")

	return nil
}

// UpdateTableProperties updates table properties
func (c *Client) UpdateTableProperties(ctx context.Context, namespace, name string, updates, removals map[string]string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to Iceberg catalog")
	}

	var requirements []map[string]interface{}
	var tableUpdates []map[string]interface{}

	if len(updates) > 0 {
		tableUpdates = append(tableUpdates, map[string]interface{}{
			"action":  "set-properties",
			"updates": updates,
		})
	}

	if len(removals) > 0 {
		keys := make([]string, 0, len(removals))
		for k := range removals {
			keys = append(keys, k)
		}
		tableUpdates = append(tableUpdates, map[string]interface{}{
			"action":   "remove-properties",
			"removals": keys,
		})
	}

	reqBody := map[string]interface{}{
		"requirements": requirements,
		"updates":      tableUpdates,
	}

	path := fmt.Sprintf("/v1/namespaces/%s/tables/%s", namespace, name)
	_, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to update table properties: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"table":     name,
	}).Info("Table properties updated")

	return nil
}

// GetSnapshots returns all snapshots for a table
func (c *Client) GetSnapshots(ctx context.Context, namespace, name string) ([]Snapshot, error) {
	metadata, err := c.GetTable(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	return metadata.Snapshots, nil
}

// GetCurrentSnapshot returns the current snapshot for a table
func (c *Client) GetCurrentSnapshot(ctx context.Context, namespace, name string) (*Snapshot, error) {
	metadata, err := c.GetTable(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	if metadata.CurrentSnapshotID == nil {
		return nil, nil
	}

	for _, snap := range metadata.Snapshots {
		if snap.SnapshotID == *metadata.CurrentSnapshotID {
			return &snap, nil
		}
	}

	return nil, nil
}

// GetSnapshotAtTimestamp returns the snapshot at a specific timestamp (time-travel)
func (c *Client) GetSnapshotAtTimestamp(ctx context.Context, namespace, name string, ts time.Time) (*Snapshot, error) {
	metadata, err := c.GetTable(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	timestampMs := ts.UnixMilli()

	// Find the latest snapshot before or at the given timestamp
	var result *Snapshot
	for i := range metadata.Snapshots {
		snap := &metadata.Snapshots[i]
		if snap.TimestampMs <= timestampMs {
			if result == nil || snap.TimestampMs > result.TimestampMs {
				result = snap
			}
		}
	}

	return result, nil
}

// WaitForTable waits for a table to be ready
func (c *Client) WaitForTable(ctx context.Context, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for table %s.%s to be ready", namespace, name)
			}

			exists, err := c.TableExists(ctx, namespace, name)
			if err != nil {
				c.logger.WithError(err).Debug("Table not ready yet")
				continue
			}

			if exists {
				return nil
			}
		}
	}
}
