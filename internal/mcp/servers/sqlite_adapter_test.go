package servers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewSQLiteAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, nil)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.initialized)
	assert.True(t, adapter.config.InMemory)
}

func TestDefaultSQLiteAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()

	assert.True(t, config.InMemory)
	assert.False(t, config.ReadOnly)
	assert.True(t, config.CreateIfNotExists)
	assert.Equal(t, 30*1000000000, int(config.QueryTimeout))
	assert.Equal(t, 1, config.MaxOpenConns)
	assert.Equal(t, 5000, config.BusyTimeout)
}

func TestNewSQLiteAdapter_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := SQLiteAdapterConfig{}
	adapter := NewSQLiteAdapter(config, logrus.New())

	assert.Equal(t, 30*1000000000, int(adapter.config.QueryTimeout))
	assert.Equal(t, 1, adapter.config.MaxOpenConns)
	assert.Equal(t, 5000, adapter.config.BusyTimeout)
}

func TestSQLiteAdapter_Health_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSQLiteAdapter_Query_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	_, err := adapter.Query(context.Background(), "SELECT 1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSQLiteAdapter_Execute_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	_, err := adapter.Execute(context.Background(), "INSERT INTO test VALUES (1)")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSQLiteAdapter_ListTables_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	_, err := adapter.ListTables(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSQLiteAdapter_DescribeTable_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	_, err := adapter.DescribeTable(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSQLiteAdapter_ListIndexes_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	_, err := adapter.ListIndexes(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSQLiteAdapter_GetStats_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	_, err := adapter.GetStats(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSQLiteAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	tools := adapter.GetMCPTools()
	assert.Len(t, tools, 8)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "sqlite_query")
	assert.Contains(t, toolNames, "sqlite_execute")
	assert.Contains(t, toolNames, "sqlite_list_tables")
	assert.Contains(t, toolNames, "sqlite_describe_table")
	assert.Contains(t, toolNames, "sqlite_list_indexes")
	assert.Contains(t, toolNames, "sqlite_stats")
	assert.Contains(t, toolNames, "sqlite_create_table")
	assert.Contains(t, toolNames, "sqlite_drop_table")
}

func TestSQLiteAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	_, err := adapter.ExecuteTool(context.Background(), "sqlite_query", map[string]interface{}{
		"query": "SELECT 1",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSQLiteAdapter_ExecuteTool_UnknownTool(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())
	adapter.initialized = true

	_, err := adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestSQLiteAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	caps := adapter.GetCapabilities()
	assert.Equal(t, "sqlite", caps["name"])
	assert.Equal(t, true, caps["in_memory"])
	assert.Equal(t, false, caps["read_only"])
	assert.Equal(t, 8, caps["tools"])
	assert.Equal(t, false, caps["initialized"])
}

func TestSQLiteAdapter_Initialize_InMemory(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	assert.True(t, adapter.initialized)

	err = adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestSQLiteAdapter_Initialize_File(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := SQLiteAdapterConfig{
		DatabasePath:      dbPath,
		InMemory:          false,
		CreateIfNotExists: true,
	}
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	assert.True(t, adapter.initialized)

	// Verify file exists
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)

	err = adapter.Close()
	assert.NoError(t, err)
}

func TestSQLiteAdapter_Initialize_FileNotExists(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Use a file in an existing directory that doesn't exist
	dbPath := filepath.Join(tmpDir, "nonexistent.db")

	config := SQLiteAdapterConfig{
		DatabasePath:      dbPath,
		InMemory:          false,
		CreateIfNotExists: false,
	}
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestSQLiteAdapter_Initialize_NoPath(t *testing.T) {
	t.Parallel()
	config := SQLiteAdapterConfig{
		InMemory: false,
	}
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database path is required")
}

func TestSQLiteAdapter_Health(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	err = adapter.Health(context.Background())
	assert.NoError(t, err)
}

func TestSQLiteAdapter_Query(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.Query(context.Background(), "SELECT 1 as num, 'hello' as msg")
	assert.NoError(t, err)
	assert.Equal(t, 1, result.RowCount)
	assert.Contains(t, result.Columns, "num")
	assert.Contains(t, result.Columns, "msg")
}

func TestSQLiteAdapter_Query_WithParams(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.Query(context.Background(), "SELECT ? as num", 42)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.RowCount)
}

func TestSQLiteAdapter_Execute_CreateTable(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	assert.NoError(t, err)

	// Verify table was created
	tables, err := adapter.ListTables(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tables, 1)
	assert.Equal(t, "test", tables[0].Name)
}

func TestSQLiteAdapter_Execute_Insert(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	assert.NoError(t, err)

	result, err := adapter.Execute(context.Background(), "INSERT INTO test (name) VALUES (?)", "test1")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.AffectedRows)
	assert.Equal(t, int64(1), result.LastInsertID)

	// Insert another
	result, err = adapter.Execute(context.Background(), "INSERT INTO test (name) VALUES (?)", "test2")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), result.LastInsertID)
}

func TestSQLiteAdapter_Execute_ReadOnly(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	config.ReadOnly = true
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE test (id INTEGER)")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only mode")
}

func TestSQLiteAdapter_Query_ReadOnlyCheck(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	config.ReadOnly = true
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	// SELECT should work
	_, err = adapter.Query(context.Background(), "SELECT 1")
	assert.NoError(t, err)

	// INSERT via Query should fail
	_, err = adapter.Query(context.Background(), "INSERT INTO test VALUES (1)")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only mode")
}

func TestSQLiteAdapter_ListTables(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	// Create tables
	_, err = adapter.Execute(context.Background(), "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	assert.NoError(t, err)
	_, err = adapter.Execute(context.Background(), "CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT)")
	assert.NoError(t, err)

	tables, err := adapter.ListTables(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tables, 2)

	names := []string{tables[0].Name, tables[1].Name}
	assert.Contains(t, names, "users")
	assert.Contains(t, names, "posts")
}

func TestSQLiteAdapter_DescribeTable(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT)")
	assert.NoError(t, err)

	info, err := adapter.DescribeTable(context.Background(), "users")
	assert.NoError(t, err)
	assert.Equal(t, "users", info.Name)
	assert.Equal(t, "table", info.Type)
	assert.Len(t, info.Columns, 3)

	// Check columns
	colNames := make(map[string]SQLiteColumnInfo)
	for _, col := range info.Columns {
		colNames[col.Name] = col
	}

	assert.Contains(t, colNames, "id")
	assert.Contains(t, colNames, "name")
	assert.Contains(t, colNames, "email")
	assert.True(t, colNames["name"].NotNull)
	assert.False(t, colNames["email"].NotNull)
}

func TestSQLiteAdapter_DescribeTable_NotFound(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.DescribeTable(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table not found")
}

func TestSQLiteAdapter_DescribeTable_EmptyName(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.DescribeTable(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table name is required")
}

func TestSQLiteAdapter_ListIndexes(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)")
	assert.NoError(t, err)
	_, err = adapter.Execute(context.Background(), "CREATE UNIQUE INDEX idx_email ON users(email)")
	assert.NoError(t, err)
	_, err = adapter.Execute(context.Background(), "CREATE INDEX idx_name ON users(name)")
	assert.NoError(t, err)

	indexes, err := adapter.ListIndexes(context.Background(), "users")
	assert.NoError(t, err)
	assert.Len(t, indexes, 2)

	idxMap := make(map[string]SQLiteIndexInfo)
	for _, idx := range indexes {
		idxMap[idx.Name] = idx
	}

	assert.Contains(t, idxMap, "idx_email")
	assert.Contains(t, idxMap, "idx_name")
	assert.True(t, idxMap["idx_email"].Unique)
	assert.False(t, idxMap["idx_name"].Unique)
}

func TestSQLiteAdapter_ListIndexes_EmptyTable(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.ListIndexes(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table name is required")
}

func TestSQLiteAdapter_GetStats(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	stats, err := adapter.GetStats(context.Background())
	assert.NoError(t, err)
	assert.Greater(t, stats.PageSize, int64(0))
}

func TestSQLiteAdapter_CreateTable(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	columns := []SQLiteColumnInfo{
		{Name: "id", Type: "INTEGER", PrimaryKey: 1},
		{Name: "name", Type: "TEXT", NotNull: true},
		{Name: "email", Type: "TEXT"},
	}

	err = adapter.CreateTable(context.Background(), "users", columns)
	assert.NoError(t, err)

	// Verify table exists
	info, err := adapter.DescribeTable(context.Background(), "users")
	assert.NoError(t, err)
	assert.Equal(t, "users", info.Name)
	assert.Len(t, info.Columns, 3)
}

func TestSQLiteAdapter_CreateTable_EmptyParams(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	err = adapter.CreateTable(context.Background(), "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table name and columns are required")
}

func TestSQLiteAdapter_CreateTable_ReadOnly(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	config.ReadOnly = true
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	columns := []SQLiteColumnInfo{{Name: "id", Type: "INTEGER"}}
	err = adapter.CreateTable(context.Background(), "test", columns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only mode")
}

func TestSQLiteAdapter_DropTable(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE test (id INTEGER)")
	assert.NoError(t, err)

	err = adapter.DropTable(context.Background(), "test")
	assert.NoError(t, err)

	// Verify table doesn't exist
	tables, err := adapter.ListTables(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tables, 0)
}

func TestSQLiteAdapter_DropTable_EmptyName(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	err = adapter.DropTable(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table name is required")
}

func TestSQLiteAdapter_DropTable_ReadOnly(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	config.ReadOnly = true
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	err = adapter.DropTable(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read-only mode")
}

func TestSQLiteAdapter_ExecuteTool_Query(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	result, err := adapter.ExecuteTool(context.Background(), "sqlite_query", map[string]interface{}{
		"query": "SELECT 1 as num",
	})
	assert.NoError(t, err)
	queryResult := result.(*SQLiteQueryResult)
	assert.Equal(t, 1, queryResult.RowCount)
}

func TestSQLiteAdapter_ExecuteTool_Execute(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.ExecuteTool(context.Background(), "sqlite_execute", map[string]interface{}{
		"query": "CREATE TABLE test (id INTEGER)",
	})
	assert.NoError(t, err)
}

func TestSQLiteAdapter_ExecuteTool_ListTables(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE test (id INTEGER)")
	assert.NoError(t, err)

	result, err := adapter.ExecuteTool(context.Background(), "sqlite_list_tables", map[string]interface{}{})
	assert.NoError(t, err)
	tables := result.([]SQLiteTableInfo)
	assert.Len(t, tables, 1)
}

func TestSQLiteAdapter_ExecuteTool_DescribeTable(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE test (id INTEGER)")
	assert.NoError(t, err)

	result, err := adapter.ExecuteTool(context.Background(), "sqlite_describe_table", map[string]interface{}{
		"table": "test",
	})
	assert.NoError(t, err)
	info := result.(*SQLiteTableInfo)
	assert.Equal(t, "test", info.Name)
}

func TestSQLiteAdapter_ExecuteTool_CreateTable(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.ExecuteTool(context.Background(), "sqlite_create_table", map[string]interface{}{
		"table": "users",
		"columns": []interface{}{
			map[string]interface{}{"name": "id", "type": "INTEGER", "pk": float64(1)},
			map[string]interface{}{"name": "name", "type": "TEXT", "notnull": true},
		},
	})
	assert.NoError(t, err)

	info, err := adapter.DescribeTable(context.Background(), "users")
	assert.NoError(t, err)
	assert.Equal(t, "users", info.Name)
}

func TestSQLiteAdapter_ExecuteTool_DropTable(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	_, err = adapter.Execute(context.Background(), "CREATE TABLE test (id INTEGER)")
	assert.NoError(t, err)

	_, err = adapter.ExecuteTool(context.Background(), "sqlite_drop_table", map[string]interface{}{
		"table": "test",
	})
	assert.NoError(t, err)

	tables, err := adapter.ListTables(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tables, 0)
}

func TestSQLiteAdapter_Close_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestSQLiteAdapter_MarshalJSON(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	data, err := adapter.MarshalJSON()
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Contains(t, result, "initialized")
	assert.Contains(t, result, "capabilities")
}

func TestSQLiteAdapter_isReadOnlyQuery(t *testing.T) {
	t.Parallel()
	config := DefaultSQLiteAdapterConfig()
	adapter := NewSQLiteAdapter(config, logrus.New())

	// Read-only queries
	assert.True(t, adapter.isReadOnlyQuery("SELECT * FROM users"))
	assert.True(t, adapter.isReadOnlyQuery("  SELECT id FROM users"))
	assert.True(t, adapter.isReadOnlyQuery("WITH cte AS (SELECT 1) SELECT * FROM cte"))
	assert.True(t, adapter.isReadOnlyQuery("EXPLAIN SELECT * FROM users"))
	assert.True(t, adapter.isReadOnlyQuery("PRAGMA table_info(users)"))

	// Write queries
	assert.False(t, adapter.isReadOnlyQuery("INSERT INTO users VALUES (1)"))
	assert.False(t, adapter.isReadOnlyQuery("UPDATE users SET name = 'test'"))
	assert.False(t, adapter.isReadOnlyQuery("DELETE FROM users"))
	assert.False(t, adapter.isReadOnlyQuery("CREATE TABLE test (id int)"))
	assert.False(t, adapter.isReadOnlyQuery("DROP TABLE test"))
}
