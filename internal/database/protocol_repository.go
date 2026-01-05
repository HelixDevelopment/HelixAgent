package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MCPServer represents an MCP server configuration
type MCPServer struct {
	ID        string          `db:"id" json:"id"`
	Name      string          `db:"name" json:"name"`
	Type      string          `db:"type" json:"type"` // local or remote
	Command   *string         `db:"command" json:"command,omitempty"`
	URL       *string         `db:"url" json:"url,omitempty"`
	Enabled   bool            `db:"enabled" json:"enabled"`
	Tools     json.RawMessage `db:"tools" json:"tools"`
	LastSync  time.Time       `db:"last_sync" json:"last_sync"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

// LSPServer represents an LSP server configuration
type LSPServer struct {
	ID           string          `db:"id" json:"id"`
	Name         string          `db:"name" json:"name"`
	Language     string          `db:"language" json:"language"`
	Command      string          `db:"command" json:"command"`
	Enabled      bool            `db:"enabled" json:"enabled"`
	Workspace    string          `db:"workspace" json:"workspace"`
	Capabilities json.RawMessage `db:"capabilities" json:"capabilities"`
	LastSync     time.Time       `db:"last_sync" json:"last_sync"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
}

// ACPServer represents an ACP server configuration
type ACPServer struct {
	ID        string          `db:"id" json:"id"`
	Name      string          `db:"name" json:"name"`
	Type      string          `db:"type" json:"type"` // local or remote
	URL       *string         `db:"url" json:"url,omitempty"`
	Enabled   bool            `db:"enabled" json:"enabled"`
	Tools     json.RawMessage `db:"tools" json:"tools"`
	LastSync  time.Time       `db:"last_sync" json:"last_sync"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

// EmbeddingConfig represents embedding configuration
type EmbeddingConfig struct {
	ID          int       `db:"id" json:"id"`
	Provider    string    `db:"provider" json:"provider"`
	Model       string    `db:"model" json:"model"`
	Dimension   int       `db:"dimension" json:"dimension"`
	APIEndpoint *string   `db:"api_endpoint" json:"api_endpoint,omitempty"`
	APIKey      *string   `db:"api_key" json:"-"` // Don't expose API key in JSON
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ProtocolCache represents cached protocol data
type ProtocolCache struct {
	CacheKey  string          `db:"cache_key" json:"cache_key"`
	CacheData json.RawMessage `db:"cache_data" json:"cache_data"`
	ExpiresAt time.Time       `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

// ProtocolMetrics represents protocol operation metrics
type ProtocolMetrics struct {
	ID           int             `db:"id" json:"id"`
	ProtocolType string          `db:"protocol_type" json:"protocol_type"` // mcp, lsp, acp, embedding
	ServerID     *string         `db:"server_id" json:"server_id,omitempty"`
	Operation    string          `db:"operation" json:"operation"`
	Status       string          `db:"status" json:"status"` // success, error, timeout
	DurationMs   *int            `db:"duration_ms" json:"duration_ms,omitempty"`
	ErrorMessage *string         `db:"error_message" json:"error_message,omitempty"`
	Metadata     json.RawMessage `db:"metadata" json:"metadata"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
}

// ProtocolRepository handles all protocol-related database operations
type ProtocolRepository struct {
	pool *pgxpool.Pool
}

// NewProtocolRepository creates a new protocol repository
func NewProtocolRepository(pool *pgxpool.Pool) *ProtocolRepository {
	return &ProtocolRepository{pool: pool}
}

// ========== MCP Server Operations ==========

// CreateMCPServer creates a new MCP server configuration
func (r *ProtocolRepository) CreateMCPServer(ctx context.Context, server *MCPServer) error {
	query := `
		INSERT INTO mcp_servers (id, name, type, command, url, enabled, tools, last_sync, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, query,
		server.ID, server.Name, server.Type, server.Command, server.URL,
		server.Enabled, server.Tools, server.LastSync, time.Now(), time.Now())
	return err
}

// GetMCPServer retrieves an MCP server by ID
func (r *ProtocolRepository) GetMCPServer(ctx context.Context, id string) (*MCPServer, error) {
	query := `
		SELECT id, name, type, command, url, enabled, tools, last_sync, created_at, updated_at
		FROM mcp_servers WHERE id = $1
	`
	var server MCPServer
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&server.ID, &server.Name, &server.Type, &server.Command, &server.URL,
		&server.Enabled, &server.Tools, &server.LastSync, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// ListMCPServers retrieves all MCP servers, optionally filtering by enabled status
func (r *ProtocolRepository) ListMCPServers(ctx context.Context, enabledOnly bool) ([]*MCPServer, error) {
	query := `
		SELECT id, name, type, command, url, enabled, tools, last_sync, created_at, updated_at
		FROM mcp_servers
	`
	if enabledOnly {
		query += " WHERE enabled = true"
	}
	query += " ORDER BY name"

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*MCPServer
	for rows.Next() {
		var server MCPServer
		if err := rows.Scan(
			&server.ID, &server.Name, &server.Type, &server.Command, &server.URL,
			&server.Enabled, &server.Tools, &server.LastSync, &server.CreatedAt, &server.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, &server)
	}
	return servers, rows.Err()
}

// UpdateMCPServer updates an MCP server configuration
func (r *ProtocolRepository) UpdateMCPServer(ctx context.Context, server *MCPServer) error {
	query := `
		UPDATE mcp_servers
		SET name = $2, type = $3, command = $4, url = $5, enabled = $6, tools = $7, last_sync = $8, updated_at = $9
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query,
		server.ID, server.Name, server.Type, server.Command, server.URL,
		server.Enabled, server.Tools, server.LastSync, time.Now())
	return err
}

// DeleteMCPServer deletes an MCP server
func (r *ProtocolRepository) DeleteMCPServer(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM mcp_servers WHERE id = $1", id)
	return err
}

// ========== LSP Server Operations ==========

// CreateLSPServer creates a new LSP server configuration
func (r *ProtocolRepository) CreateLSPServer(ctx context.Context, server *LSPServer) error {
	query := `
		INSERT INTO lsp_servers (id, name, language, command, enabled, workspace, capabilities, last_sync, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, query,
		server.ID, server.Name, server.Language, server.Command, server.Enabled,
		server.Workspace, server.Capabilities, server.LastSync, time.Now(), time.Now())
	return err
}

// GetLSPServer retrieves an LSP server by ID
func (r *ProtocolRepository) GetLSPServer(ctx context.Context, id string) (*LSPServer, error) {
	query := `
		SELECT id, name, language, command, enabled, workspace, capabilities, last_sync, created_at, updated_at
		FROM lsp_servers WHERE id = $1
	`
	var server LSPServer
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&server.ID, &server.Name, &server.Language, &server.Command, &server.Enabled,
		&server.Workspace, &server.Capabilities, &server.LastSync, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// ListLSPServers retrieves all LSP servers
func (r *ProtocolRepository) ListLSPServers(ctx context.Context, enabledOnly bool) ([]*LSPServer, error) {
	query := `
		SELECT id, name, language, command, enabled, workspace, capabilities, last_sync, created_at, updated_at
		FROM lsp_servers
	`
	if enabledOnly {
		query += " WHERE enabled = true"
	}
	query += " ORDER BY language, name"

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*LSPServer
	for rows.Next() {
		var server LSPServer
		if err := rows.Scan(
			&server.ID, &server.Name, &server.Language, &server.Command, &server.Enabled,
			&server.Workspace, &server.Capabilities, &server.LastSync, &server.CreatedAt, &server.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, &server)
	}
	return servers, rows.Err()
}

// UpdateLSPServer updates an LSP server configuration
func (r *ProtocolRepository) UpdateLSPServer(ctx context.Context, server *LSPServer) error {
	query := `
		UPDATE lsp_servers
		SET name = $2, language = $3, command = $4, enabled = $5, workspace = $6, capabilities = $7, last_sync = $8, updated_at = $9
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query,
		server.ID, server.Name, server.Language, server.Command, server.Enabled,
		server.Workspace, server.Capabilities, server.LastSync, time.Now())
	return err
}

// DeleteLSPServer deletes an LSP server
func (r *ProtocolRepository) DeleteLSPServer(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM lsp_servers WHERE id = $1", id)
	return err
}

// ========== ACP Server Operations ==========

// CreateACPServer creates a new ACP server configuration
func (r *ProtocolRepository) CreateACPServer(ctx context.Context, server *ACPServer) error {
	query := `
		INSERT INTO acp_servers (id, name, type, url, enabled, tools, last_sync, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.pool.Exec(ctx, query,
		server.ID, server.Name, server.Type, server.URL, server.Enabled,
		server.Tools, server.LastSync, time.Now(), time.Now())
	return err
}

// GetACPServer retrieves an ACP server by ID
func (r *ProtocolRepository) GetACPServer(ctx context.Context, id string) (*ACPServer, error) {
	query := `
		SELECT id, name, type, url, enabled, tools, last_sync, created_at, updated_at
		FROM acp_servers WHERE id = $1
	`
	var server ACPServer
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&server.ID, &server.Name, &server.Type, &server.URL, &server.Enabled,
		&server.Tools, &server.LastSync, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// ListACPServers retrieves all ACP servers
func (r *ProtocolRepository) ListACPServers(ctx context.Context, enabledOnly bool) ([]*ACPServer, error) {
	query := `
		SELECT id, name, type, url, enabled, tools, last_sync, created_at, updated_at
		FROM acp_servers
	`
	if enabledOnly {
		query += " WHERE enabled = true"
	}
	query += " ORDER BY name"

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*ACPServer
	for rows.Next() {
		var server ACPServer
		if err := rows.Scan(
			&server.ID, &server.Name, &server.Type, &server.URL, &server.Enabled,
			&server.Tools, &server.LastSync, &server.CreatedAt, &server.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, &server)
	}
	return servers, rows.Err()
}

// UpdateACPServer updates an ACP server configuration
func (r *ProtocolRepository) UpdateACPServer(ctx context.Context, server *ACPServer) error {
	query := `
		UPDATE acp_servers
		SET name = $2, type = $3, url = $4, enabled = $5, tools = $6, last_sync = $7, updated_at = $8
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query,
		server.ID, server.Name, server.Type, server.URL, server.Enabled,
		server.Tools, server.LastSync, time.Now())
	return err
}

// DeleteACPServer deletes an ACP server
func (r *ProtocolRepository) DeleteACPServer(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM acp_servers WHERE id = $1", id)
	return err
}

// ========== Embedding Config Operations ==========

// GetEmbeddingConfig retrieves the current embedding configuration
func (r *ProtocolRepository) GetEmbeddingConfig(ctx context.Context) (*EmbeddingConfig, error) {
	query := `
		SELECT id, provider, model, dimension, api_endpoint, api_key, created_at, updated_at
		FROM embedding_config ORDER BY id LIMIT 1
	`
	var cfg EmbeddingConfig
	err := r.pool.QueryRow(ctx, query).Scan(
		&cfg.ID, &cfg.Provider, &cfg.Model, &cfg.Dimension,
		&cfg.APIEndpoint, &cfg.APIKey, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateEmbeddingConfig updates the embedding configuration
func (r *ProtocolRepository) UpdateEmbeddingConfig(ctx context.Context, cfg *EmbeddingConfig) error {
	query := `
		UPDATE embedding_config
		SET provider = $2, model = $3, dimension = $4, api_endpoint = $5, api_key = $6, updated_at = $7
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query,
		cfg.ID, cfg.Provider, cfg.Model, cfg.Dimension,
		cfg.APIEndpoint, cfg.APIKey, time.Now())
	return err
}

// ========== Protocol Cache Operations ==========

// SetCache stores data in the protocol cache
func (r *ProtocolRepository) SetCache(ctx context.Context, key string, data json.RawMessage, ttl time.Duration) error {
	query := `
		INSERT INTO protocol_cache (cache_key, cache_data, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cache_key) DO UPDATE SET cache_data = $2, expires_at = $3, updated_at = $5
	`
	expiresAt := time.Now().Add(ttl)
	_, err := r.pool.Exec(ctx, query, key, data, expiresAt, time.Now(), time.Now())
	return err
}

// GetCache retrieves data from the protocol cache
func (r *ProtocolRepository) GetCache(ctx context.Context, key string) (*ProtocolCache, error) {
	query := `
		SELECT cache_key, cache_data, expires_at, created_at, updated_at
		FROM protocol_cache WHERE cache_key = $1 AND expires_at > $2
	`
	var cache ProtocolCache
	err := r.pool.QueryRow(ctx, query, key, time.Now()).Scan(
		&cache.CacheKey, &cache.CacheData, &cache.ExpiresAt, &cache.CreatedAt, &cache.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

// DeleteCache removes an entry from the protocol cache
func (r *ProtocolRepository) DeleteCache(ctx context.Context, key string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM protocol_cache WHERE cache_key = $1", key)
	return err
}

// ClearExpiredCache removes all expired cache entries
func (r *ProtocolRepository) ClearExpiredCache(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx, "DELETE FROM protocol_cache WHERE expires_at <= $1", time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// ========== Protocol Metrics Operations ==========

// RecordMetric records a protocol operation metric
func (r *ProtocolRepository) RecordMetric(ctx context.Context, metric *ProtocolMetrics) error {
	query := `
		INSERT INTO protocol_metrics (protocol_type, server_id, operation, status, duration_ms, error_message, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.pool.Exec(ctx, query,
		metric.ProtocolType, metric.ServerID, metric.Operation, metric.Status,
		metric.DurationMs, metric.ErrorMessage, metric.Metadata, time.Now(), time.Now())
	return err
}

// GetMetrics retrieves protocol metrics with optional filtering
func (r *ProtocolRepository) GetMetrics(ctx context.Context, protocolType string, since time.Time, limit int) ([]*ProtocolMetrics, error) {
	query := `
		SELECT id, protocol_type, server_id, operation, status, duration_ms, error_message, metadata, created_at, updated_at
		FROM protocol_metrics WHERE created_at >= $1
	`
	args := []interface{}{since}
	argNum := 2

	if protocolType != "" {
		query += " AND protocol_type = $" + string('0'+byte(argNum))
		args = append(args, protocolType)
		argNum++
	}

	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += " LIMIT $" + string('0'+byte(argNum))
		args = append(args, limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []*ProtocolMetrics
	for rows.Next() {
		var m ProtocolMetrics
		if err := rows.Scan(
			&m.ID, &m.ProtocolType, &m.ServerID, &m.Operation, &m.Status,
			&m.DurationMs, &m.ErrorMessage, &m.Metadata, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		metrics = append(metrics, &m)
	}
	return metrics, rows.Err()
}

// GetMetricsSummary returns aggregated metrics for a protocol type
func (r *ProtocolRepository) GetMetricsSummary(ctx context.Context, protocolType string, since time.Time) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_operations,
			COUNT(CASE WHEN status = 'success' THEN 1 END) as successful,
			COUNT(CASE WHEN status = 'error' THEN 1 END) as errors,
			COUNT(CASE WHEN status = 'timeout' THEN 1 END) as timeouts,
			AVG(duration_ms) as avg_duration_ms,
			MAX(duration_ms) as max_duration_ms,
			MIN(duration_ms) as min_duration_ms
		FROM protocol_metrics
		WHERE protocol_type = $1 AND created_at >= $2
	`
	var total, successful, errors, timeouts int64
	var avgDuration, maxDuration, minDuration *float64

	err := r.pool.QueryRow(ctx, query, protocolType, since).Scan(
		&total, &successful, &errors, &timeouts,
		&avgDuration, &maxDuration, &minDuration)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_operations": total,
		"successful":       successful,
		"errors":           errors,
		"timeouts":         timeouts,
		"avg_duration_ms":  avgDuration,
		"max_duration_ms":  maxDuration,
		"min_duration_ms":  minDuration,
		"success_rate":     float64(successful) / float64(total) * 100,
	}, nil
}
