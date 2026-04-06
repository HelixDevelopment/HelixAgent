// Package servers provides MCP server adapters for various services.
package servers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

// SQLiteAdapterConfig holds configuration for SQLite MCP adapter
type SQLiteAdapterConfig struct {
	// DatabasePath is the path to the SQLite database file
	DatabasePath string `json:"database_path,omitempty"`
	// InMemory creates an in-memory database
	InMemory bool `json:"in_memory"`
	// ReadOnly opens the database in read-only mode
	ReadOnly bool `json:"read_only"`
	// CreateIfNotExists creates the database if it doesn't exist
	CreateIfNotExists bool `json:"create_if_not_exists"`
	// QueryTimeout is the default query timeout
	QueryTimeout time.Duration `json:"query_timeout,omitempty"`
	// MaxOpenConns is the maximum number of open connections
	MaxOpenConns int `json:"max_open_conns,omitempty"`
	// BusyTimeout is the busy timeout in milliseconds
	BusyTimeout int `json:"busy_timeout,omitempty"`
}

// DefaultSQLiteAdapterConfig returns default configuration
func DefaultSQLiteAdapterConfig() SQLiteAdapterConfig {
	return SQLiteAdapterConfig{
		InMemory:          true,
		ReadOnly:          false,
		CreateIfNotExists: true,
		QueryTimeout:      30 * time.Second,
		MaxOpenConns:      1, // SQLite works best with single connection
		BusyTimeout:       5000,
	}
}

// SQLiteAdapter implements MCP adapter for SQLite operations
type SQLiteAdapter struct {
	config      SQLiteAdapterConfig
	db          *sql.DB
	initialized bool
	mu          sync.RWMutex
	logger      *logrus.Logger
}

// NewSQLiteAdapter creates a new SQLite MCP adapter
func NewSQLiteAdapter(config SQLiteAdapterConfig, logger *logrus.Logger) *SQLiteAdapter {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}

	if config.QueryTimeout <= 0 {
		config.QueryTimeout = 30 * time.Second
	}
	if config.MaxOpenConns <= 0 {
		config.MaxOpenConns = 1
	}
	if config.BusyTimeout <= 0 {
		config.BusyTimeout = 5000
	}

	return &SQLiteAdapter{
		config: config,
		logger: logger,
	}
}

// Initialize initializes the SQLite adapter
func (s *SQLiteAdapter) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var dsn string
	if s.config.InMemory {
		dsn = ":memory:"
	} else {
		if s.config.DatabasePath == "" {
			return fmt.Errorf("database path is required for file-based SQLite")
		}

		// Ensure directory exists
		dir := filepath.Dir(s.config.DatabasePath)
		if dir != "" && dir != "." {
			// #nosec G301 -- sqlite database directory uses standard 0750 permissions
			if err := os.MkdirAll(dir, 0750); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		// Check if file exists
		if !s.config.CreateIfNotExists {
			if _, err := os.Stat(s.config.DatabasePath); os.IsNotExist(err) {
				return fmt.Errorf("database file does not exist: %s", s.config.DatabasePath)
			}
		}

		dsn = s.config.DatabasePath
		if s.config.ReadOnly {
			dsn += "?mode=ro"
		}
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(s.config.MaxOpenConns)

	// Set pragmas
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d", s.config.BusyTimeout),
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			s.logger.WithError(err).Warn("Failed to set pragma")
		}
	}

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	s.db = db
	s.initialized = true
	s.logger.WithFields(logrus.Fields{
		"path":      s.config.DatabasePath,
		"in_memory": s.config.InMemory,
		"read_only": s.config.ReadOnly,
	}).Info("SQLite adapter initialized")

	return nil
}

// Health returns health status
func (s *SQLiteAdapter) Health(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return fmt.Errorf("SQLite adapter not initialized")
	}

	return s.db.PingContext(ctx)
}

// Close closes the adapter and database connection
func (s *SQLiteAdapter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return err
		}
		s.db = nil
	}

	s.initialized = false
	return nil
}

// isReadOnlyQuery checks if a query is read-only
func (s *SQLiteAdapter) isReadOnlyQuery(query string) bool {
	query = strings.TrimSpace(strings.ToUpper(query))
	return strings.HasPrefix(query, "SELECT") ||
		strings.HasPrefix(query, "WITH") ||
		strings.HasPrefix(query, "EXPLAIN") ||
		strings.HasPrefix(query, "PRAGMA")
}

// SQLiteQueryResult represents the result of a query
type SQLiteQueryResult struct {
	Columns      []string                 `json:"columns"`
	Rows         []map[string]interface{} `json:"rows"`
	RowCount     int                      `json:"row_count"`
	AffectedRows int64                    `json:"affected_rows,omitempty"`
	LastInsertID int64                    `json:"last_insert_id,omitempty"`
	Duration     time.Duration            `json:"duration"`
}

// Query executes a SQL query and returns results
func (s *SQLiteAdapter) Query(ctx context.Context, query string, args ...interface{}) (*SQLiteQueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}

	// Check read-only restriction
	if s.config.ReadOnly && !s.isReadOnlyQuery(query) {
		return nil, fmt.Errorf("write operations not allowed in read-only mode")
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.QueryTimeout)
	defer cancel()

	start := time.Now()

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	result := &SQLiteQueryResult{
		Columns: columns,
		Rows:    []map[string]interface{}{},
	}

	// Create scan destinations
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert byte slices to strings for JSON compatibility
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		result.Rows = append(result.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	result.RowCount = len(result.Rows)
	result.Duration = time.Since(start)

	return result, nil
}

// Execute executes a SQL statement (INSERT, UPDATE, DELETE)
func (s *SQLiteAdapter) Execute(ctx context.Context, query string, args ...interface{}) (*SQLiteQueryResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}

	if s.config.ReadOnly {
		return nil, fmt.Errorf("write operations not allowed in read-only mode")
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.QueryTimeout)
	defer cancel()

	start := time.Now()

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execute failed: %w", err)
	}

	affected, _ := result.RowsAffected() //nolint:errcheck
	lastID, _ := result.LastInsertId()   //nolint:errcheck

	return &SQLiteQueryResult{
		AffectedRows: affected,
		LastInsertID: lastID,
		Duration:     time.Since(start),
	}, nil
}

// SQLiteTableInfo represents information about a table
type SQLiteTableInfo struct {
	Name    string             `json:"name"`
	Type    string             `json:"type"`
	SQL     string             `json:"sql,omitempty"`
	Columns []SQLiteColumnInfo `json:"columns,omitempty"`
}

// SQLiteColumnInfo represents information about a column
type SQLiteColumnInfo struct {
	CID          int         `json:"cid"`
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	NotNull      bool        `json:"notnull"`
	DefaultValue interface{} `json:"dflt_value"`
	PrimaryKey   int         `json:"pk"`
}

// ListTables returns a list of tables in the database
func (s *SQLiteAdapter) ListTables(ctx context.Context) ([]SQLiteTableInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}

	query := `SELECT name, type, sql FROM sqlite_master
			  WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
			  ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []SQLiteTableInfo
	for rows.Next() {
		var t SQLiteTableInfo
		var sqlStr sql.NullString
		if err := rows.Scan(&t.Name, &t.Type, &sqlStr); err != nil {
			return nil, fmt.Errorf("failed to scan table: %w", err)
		}
		if sqlStr.Valid {
			t.SQL = sqlStr.String
		}
		tables = append(tables, t)
	}

	return tables, nil
}

// DescribeTable returns detailed information about a table
func (s *SQLiteAdapter) DescribeTable(ctx context.Context, tableName string) (*SQLiteTableInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}

	if tableName == "" {
		return nil, fmt.Errorf("table name is required")
	}

	// Get table info
	info := &SQLiteTableInfo{Name: tableName}

	// Get table SQL
	row := s.db.QueryRowContext(ctx, "SELECT type, sql FROM sqlite_master WHERE name = ?", tableName)
	var sqlStr sql.NullString
	if err := row.Scan(&info.Type, &sqlStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("table not found: %s", tableName)
		}
		return nil, fmt.Errorf("failed to get table info: %w", err)
	}
	if sqlStr.Valid {
		info.SQL = sqlStr.String
	}

	// Get columns
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var col SQLiteColumnInfo
		var dflt sql.NullString
		if err := rows.Scan(&col.CID, &col.Name, &col.Type, &col.NotNull, &dflt, &col.PrimaryKey); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}
		if dflt.Valid {
			col.DefaultValue = dflt.String
		}
		info.Columns = append(info.Columns, col)
	}

	return info, nil
}

// SQLiteIndexInfo represents information about an index
type SQLiteIndexInfo struct {
	Name    string   `json:"name"`
	Table   string   `json:"table"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
	SQL     string   `json:"sql,omitempty"`
}

// ListIndexes returns indexes for a table
func (s *SQLiteAdapter) ListIndexes(ctx context.Context, tableName string) ([]SQLiteIndexInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}

	if tableName == "" {
		return nil, fmt.Errorf("table name is required")
	}

	query := `SELECT name, tbl_name, sql FROM sqlite_master
			  WHERE type = 'index' AND tbl_name = ?
			  ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	// First collect all basic index info to avoid nested query deadlock
	var indexes []SQLiteIndexInfo
	for rows.Next() {
		var idx SQLiteIndexInfo
		var sqlStr sql.NullString
		if err := rows.Scan(&idx.Name, &idx.Table, &sqlStr); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}
		if sqlStr.Valid {
			idx.SQL = sqlStr.String
			idx.Unique = strings.Contains(strings.ToUpper(idx.SQL), "UNIQUE")
		}
		indexes = append(indexes, idx)
	}
	_ = rows.Close()

	// Now get column info for each index (separate queries to avoid deadlock)
	for i := range indexes {
		colRows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA index_info(%s)", indexes[i].Name))
		if err == nil {
			for colRows.Next() {
				var seqno, cid int
				var name string
				if err := colRows.Scan(&seqno, &cid, &name); err == nil {
					indexes[i].Columns = append(indexes[i].Columns, name)
				}
			}
			_ = colRows.Close()
		}
	}

	return indexes, nil
}

// SQLiteDatabaseStats represents database statistics
type SQLiteDatabaseStats struct {
	PageSize      int64 `json:"page_size"`
	PageCount     int64 `json:"page_count"`
	DatabaseSize  int64 `json:"database_size"`
	FreelistCount int64 `json:"freelist_count"`
	SchemaVersion int64 `json:"schema_version"`
	UserVersion   int64 `json:"user_version"`
}

// GetStats returns database statistics
func (s *SQLiteAdapter) GetStats(ctx context.Context) (*SQLiteDatabaseStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}

	stats := &SQLiteDatabaseStats{}

	// Get various pragma values
	pragmas := map[string]*int64{
		"page_size":      &stats.PageSize,
		"page_count":     &stats.PageCount,
		"freelist_count": &stats.FreelistCount,
		"schema_version": &stats.SchemaVersion,
		"user_version":   &stats.UserVersion,
	}

	for pragma, dest := range pragmas {
		row := s.db.QueryRowContext(ctx, fmt.Sprintf("PRAGMA %s", pragma))
		_ = row.Scan(dest) //nolint:errcheck
	}

	stats.DatabaseSize = stats.PageSize * stats.PageCount

	return stats, nil
}

// CreateTable creates a new table
func (s *SQLiteAdapter) CreateTable(ctx context.Context, tableName string, columns []SQLiteColumnInfo) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return fmt.Errorf("adapter not initialized")
	}

	if s.config.ReadOnly {
		return fmt.Errorf("write operations not allowed in read-only mode")
	}

	if tableName == "" || len(columns) == 0 {
		return fmt.Errorf("table name and columns are required")
	}

	var columnDefs []string
	for _, col := range columns {
		def := fmt.Sprintf("%s %s", col.Name, col.Type)
		if col.NotNull {
			def += " NOT NULL"
		}
		if col.DefaultValue != nil {
			def += fmt.Sprintf(" DEFAULT %v", col.DefaultValue)
		}
		if col.PrimaryKey > 0 {
			def += " PRIMARY KEY"
		}
		columnDefs = append(columnDefs, def)
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(columnDefs, ", "))

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// DropTable drops a table
func (s *SQLiteAdapter) DropTable(ctx context.Context, tableName string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized || s.db == nil {
		return fmt.Errorf("adapter not initialized")
	}

	if s.config.ReadOnly {
		return fmt.Errorf("write operations not allowed in read-only mode")
	}

	if tableName == "" {
		return fmt.Errorf("table name is required")
	}

	_, err := s.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	return nil
}

// GetMCPTools returns the list of MCP tools provided by this adapter
func (s *SQLiteAdapter) GetMCPTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "sqlite_query",
			Description: "Execute a SQL query and return results",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "SQL query to execute",
					},
					"params": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Query parameters",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "sqlite_execute",
			Description: "Execute a SQL statement (INSERT, UPDATE, DELETE, CREATE)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "SQL statement to execute",
					},
					"params": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Query parameters",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "sqlite_list_tables",
			Description: "List all tables in the database",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "sqlite_describe_table",
			Description: "Get detailed information about a table",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table": map[string]interface{}{
						"type":        "string",
						"description": "Table name",
					},
				},
				"required": []string{"table"},
			},
		},
		{
			Name:        "sqlite_list_indexes",
			Description: "List indexes for a table",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table": map[string]interface{}{
						"type":        "string",
						"description": "Table name",
					},
				},
				"required": []string{"table"},
			},
		},
		{
			Name:        "sqlite_stats",
			Description: "Get database statistics",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "sqlite_create_table",
			Description: "Create a new table",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table": map[string]interface{}{
						"type":        "string",
						"description": "Table name",
					},
					"columns": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name":    map[string]interface{}{"type": "string"},
								"type":    map[string]interface{}{"type": "string"},
								"notnull": map[string]interface{}{"type": "boolean"},
								"pk":      map[string]interface{}{"type": "integer"},
							},
						},
						"description": "Column definitions",
					},
				},
				"required": []string{"table", "columns"},
			},
		},
		{
			Name:        "sqlite_drop_table",
			Description: "Drop a table",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table": map[string]interface{}{
						"type":        "string",
						"description": "Table name",
					},
				},
				"required": []string{"table"},
			},
		},
	}
}

// ExecuteTool executes an MCP tool
func (s *SQLiteAdapter) ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}) (interface{}, error) {
	s.mu.RLock()
	initialized := s.initialized
	s.mu.RUnlock()

	if !initialized {
		return nil, fmt.Errorf("adapter not initialized")
	}

	switch toolName {
	case "sqlite_query":
		query, _ := params["query"].(string) //nolint:errcheck // schema validation ensures correct type
		var args []interface{}
		if p, ok := params["params"].([]interface{}); ok {
			args = p
		}
		return s.Query(ctx, query, args...)

	case "sqlite_execute":
		query, _ := params["query"].(string) //nolint:errcheck // schema validation ensures correct type
		var args []interface{}
		if p, ok := params["params"].([]interface{}); ok {
			args = p
		}
		return s.Execute(ctx, query, args...)

	case "sqlite_list_tables":
		return s.ListTables(ctx)

	case "sqlite_describe_table":
		table, _ := params["table"].(string) //nolint:errcheck // schema validation ensures correct type
		return s.DescribeTable(ctx, table)

	case "sqlite_list_indexes":
		table, _ := params["table"].(string) //nolint:errcheck // schema validation ensures correct type
		return s.ListIndexes(ctx, table)

	case "sqlite_stats":
		return s.GetStats(ctx)

	case "sqlite_create_table":
		table, _ := params["table"].(string) //nolint:errcheck // schema validation ensures correct type
		var columns []SQLiteColumnInfo
		if cols, ok := params["columns"].([]interface{}); ok {
			for _, c := range cols {
				if colMap, ok := c.(map[string]interface{}); ok {
					col := SQLiteColumnInfo{
						Name: fmt.Sprintf("%v", colMap["name"]),
						Type: fmt.Sprintf("%v", colMap["type"]),
					}
					if nn, ok := colMap["notnull"].(bool); ok {
						col.NotNull = nn
					}
					if pk, ok := colMap["pk"].(float64); ok {
						col.PrimaryKey = int(pk)
					}
					columns = append(columns, col)
				}
			}
		}
		return nil, s.CreateTable(ctx, table, columns)

	case "sqlite_drop_table":
		table, _ := params["table"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, s.DropTable(ctx, table)

	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// GetCapabilities returns adapter capabilities
func (s *SQLiteAdapter) GetCapabilities() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"name":        "sqlite",
		"path":        s.config.DatabasePath,
		"in_memory":   s.config.InMemory,
		"read_only":   s.config.ReadOnly,
		"tools":       len(s.GetMCPTools()),
		"initialized": s.initialized,
	}
}

// MarshalJSON implements custom JSON marshaling
func (s *SQLiteAdapter) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"initialized":  s.initialized,
		"capabilities": s.GetCapabilities(),
	})
}
