package servers

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/testutil"
)

func TestNewPostgresAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, nil)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.initialized)
	assert.Equal(t, "localhost", adapter.config.Host)
	assert.Equal(t, 5432, adapter.config.Port)
}

func TestDefaultPostgresAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 5432, config.Port)
	assert.Equal(t, "postgres", config.User)
	assert.Equal(t, "postgres", config.Database)
	assert.Equal(t, "disable", config.SSLMode)
	assert.Equal(t, 10, config.MaxOpenConns)
	assert.Equal(t, 5, config.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, config.ConnMaxLifetime)
	assert.Equal(t, 30*time.Second, config.QueryTimeout)
	assert.False(t, config.ReadOnly)
	assert.Contains(t, config.AllowedSchemas, "public")
}

func TestNewPostgresAdapter_DefaultConfig(t *testing.T) {
	t.Parallel()
	// Test with empty config - should apply defaults
	config := PostgresAdapterConfig{}
	adapter := NewPostgresAdapter(config, logrus.New())

	assert.Equal(t, "localhost", adapter.config.Host)
	assert.Equal(t, 5432, adapter.config.Port)
	assert.Equal(t, "disable", adapter.config.SSLMode)
	assert.Equal(t, 10, adapter.config.MaxOpenConns)
	assert.Equal(t, 5, adapter.config.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, adapter.config.ConnMaxLifetime)
	assert.Equal(t, 30*time.Second, adapter.config.QueryTimeout)
}

func TestPostgresAdapter_Health_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	err := adapter.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_Query_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	_, err := adapter.Query(context.Background(), "SELECT 1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_Execute_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	_, err := adapter.Execute(context.Background(), "INSERT INTO test VALUES (1)")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_ListTables_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	_, err := adapter.ListTables(context.Background(), "public")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_DescribeTable_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	_, err := adapter.DescribeTable(context.Background(), "public", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_ListSchemas_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	_, err := adapter.ListSchemas(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_ListIndexes_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	_, err := adapter.ListIndexes(context.Background(), "public", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_GetStats_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	_, err := adapter.GetStats(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_IsSchemaAllowed(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	config.AllowedSchemas = []string{"public", "app"}
	adapter := NewPostgresAdapter(config, logrus.New())

	assert.True(t, adapter.isSchemaAllowed("public"))
	assert.True(t, adapter.isSchemaAllowed("PUBLIC"))
	assert.True(t, adapter.isSchemaAllowed("app"))
	assert.False(t, adapter.isSchemaAllowed("private"))
	assert.False(t, adapter.isSchemaAllowed("system"))
}

func TestPostgresAdapter_IsSchemaAllowed_NoRestriction(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	config.AllowedSchemas = []string{}
	adapter := NewPostgresAdapter(config, logrus.New())

	assert.True(t, adapter.isSchemaAllowed("public"))
	assert.True(t, adapter.isSchemaAllowed("any_schema"))
}

func TestPostgresAdapter_IsReadOnlyQuery(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	// Read-only queries
	assert.True(t, adapter.isReadOnlyQuery("SELECT * FROM users"))
	assert.True(t, adapter.isReadOnlyQuery("  SELECT id FROM users"))
	assert.True(t, adapter.isReadOnlyQuery("WITH cte AS (SELECT 1) SELECT * FROM cte"))
	assert.True(t, adapter.isReadOnlyQuery("EXPLAIN SELECT * FROM users"))
	assert.True(t, adapter.isReadOnlyQuery("SHOW search_path"))

	// Write queries
	assert.False(t, adapter.isReadOnlyQuery("INSERT INTO users VALUES (1)"))
	assert.False(t, adapter.isReadOnlyQuery("UPDATE users SET name = 'test'"))
	assert.False(t, adapter.isReadOnlyQuery("DELETE FROM users"))
	assert.False(t, adapter.isReadOnlyQuery("CREATE TABLE test (id int)"))
	assert.False(t, adapter.isReadOnlyQuery("DROP TABLE test"))
	assert.False(t, adapter.isReadOnlyQuery("TRUNCATE users"))
}

func TestPostgresAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	tools := adapter.GetMCPTools()
	assert.Len(t, tools, 7)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "postgres_query")
	assert.Contains(t, toolNames, "postgres_execute")
	assert.Contains(t, toolNames, "postgres_list_tables")
	assert.Contains(t, toolNames, "postgres_describe_table")
	assert.Contains(t, toolNames, "postgres_list_schemas")
	assert.Contains(t, toolNames, "postgres_list_indexes")
	assert.Contains(t, toolNames, "postgres_stats")
}

func TestPostgresAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	_, err := adapter.ExecuteTool(context.Background(), "postgres_query", map[string]interface{}{
		"query": "SELECT 1",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPostgresAdapter_ExecuteTool_UnknownTool(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())
	adapter.initialized = true // Manually set for this test

	_, err := adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestPostgresAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	config.ReadOnly = true
	adapter := NewPostgresAdapter(config, logrus.New())

	caps := adapter.GetCapabilities()
	assert.Equal(t, "postgres", caps["name"])
	assert.Equal(t, "localhost", caps["host"])
	assert.Equal(t, 5432, caps["port"])
	assert.Equal(t, "postgres", caps["database"])
	assert.Equal(t, true, caps["read_only"])
	assert.Equal(t, 7, caps["tools"])
	assert.Equal(t, false, caps["initialized"])
}

func TestPostgresAdapter_Close_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	adapter := NewPostgresAdapter(config, logrus.New())

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestPostgresAdapter_SchemaNotAllowed(t *testing.T) {
	t.Parallel()
	config := DefaultPostgresAdapterConfig()
	config.AllowedSchemas = []string{"public"}
	adapter := NewPostgresAdapter(config, logrus.New())
	adapter.initialized = true

	// ListTables with non-allowed schema
	_, err := adapter.ListTables(context.Background(), "private")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema not allowed")

	// DescribeTable with non-allowed schema
	_, err = adapter.DescribeTable(context.Background(), "private", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema not allowed")
}

// Integration tests that require a running PostgreSQL instance
// These tests are skipped by default
func TestPostgresAdapter_Integration(t *testing.T) {
	t.Parallel()
	testutil.RequirePostgres(t)

	config := PostgresAdapterConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "testdb",
		SSLMode:  "disable",
	}

	adapter := NewPostgresAdapter(config, logrus.New())

	// Initialize
	err := adapter.Initialize(context.Background())
	if err != nil {
		t.Skipf("Could not connect to PostgreSQL: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	// Test query
	result, err := adapter.Query(context.Background(), "SELECT 1 as num")
	assert.NoError(t, err)
	assert.Equal(t, 1, result.RowCount)

	// Test list schemas
	schemas, err := adapter.ListSchemas(context.Background())
	assert.NoError(t, err)
	assert.Contains(t, schemas, "public")

	// Test health
	err = adapter.Health(context.Background())
	assert.NoError(t, err)

	// Test stats
	stats, err := adapter.GetStats(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, stats.Size)
}
