package database

import (
	"testing"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Memory Extended Tests
// These tests cover NewPostgresDBWithFallback and additional edge cases
// for the in-memory database implementation.
// =============================================================================

// -----------------------------------------------------------------------------
// NewPostgresDBWithFallback Tests
// -----------------------------------------------------------------------------

func TestNewPostgresDBWithFallback_FallsBackToMemory(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "nonexistent-host-that-will-fail.invalid",
			Port:     "59999",
			User:     "test",
			Password: "test",
			Name:     "test",
			SSLMode:  "disable",
		},
	}

	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)
	require.NoError(t, err)

	// PostgresDB should be nil (connection failed)
	// MemoryDB should be returned as fallback
	if pgDB != nil {
		// If PostgreSQL happens to be running, that's fine
		defer func() { _ = pgDB.Close() }()
		assert.Nil(t, memDB)
	} else {
		assert.NotNil(t, memDB)
		assert.True(t, memDB.IsMemoryMode())

		// MemoryDB should be functional
		err = memDB.Ping()
		assert.NoError(t, err)

		err = memDB.HealthCheck()
		assert.NoError(t, err)

		_ = memDB.Close()
	}
}

func TestNewPostgresDBWithFallback_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid!host@#$",
			Port:     "99999",
			User:     "test",
			Password: "test",
			Name:     "test",
			SSLMode:  "disable",
		},
	}

	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)
	require.NoError(t, err)

	if pgDB != nil {
		defer func() { _ = pgDB.Close() }()
	} else {
		assert.NotNil(t, memDB)
		assert.True(t, memDB.IsMemoryMode())
		_ = memDB.Close()
	}
}

func TestNewPostgresDBWithFallback_EmptyConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{},
	}

	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)
	require.NoError(t, err)

	// Either PostgreSQL connects (if running locally) or we get MemoryDB
	if pgDB != nil {
		defer func() { _ = pgDB.Close() }()
		assert.Nil(t, memDB)
	} else {
		assert.NotNil(t, memDB)
		_ = memDB.Close()
	}
}

// -----------------------------------------------------------------------------
// MemoryDB Additional Edge Cases
// -----------------------------------------------------------------------------

func TestMemoryDB_StoreRow_NilValues(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	db.StoreRow("test_table", "key1", nil)

	row := db.QueryRow("SELECT val FROM test_table WHERE id = $1", "key1")
	assert.NotNil(t, row)

	// Scanning nil values should return "no rows" error
	var val string
	err := row.Scan(&val)
	assert.Error(t, err)
}

func TestMemoryDB_StoreRow_EmptyTable(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	db.StoreRow("", "key1", []any{"value"})

	// Empty table name stored as-is
	row := db.QueryRow("SELECT val FROM  WHERE id = $1", "key1")
	assert.NotNil(t, row)
}

func TestMemoryDB_QueryRow_MultipleTables_NoContamination(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	db.StoreRow("table_a", "key1", []any{"value_a"})
	db.StoreRow("table_b", "key1", []any{"value_b"})

	// Query table_a
	row := db.QueryRow("SELECT val FROM table_a WHERE id = $1", "key1")
	var val string
	err := row.Scan(&val)
	assert.NoError(t, err)
	assert.Equal(t, "value_a", val)

	// Query table_b
	row = db.QueryRow("SELECT val FROM table_b WHERE id = $1", "key1")
	err = row.Scan(&val)
	assert.NoError(t, err)
	assert.Equal(t, "value_b", val)
}

func TestMemoryDB_ExecIsNoOp(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	// Exec should return nil for any query
	err := db.Exec("DROP TABLE users")
	assert.NoError(t, err)

	err = db.Exec("CREATE TABLE test (id INT)")
	assert.NoError(t, err)

	err = db.Exec("DELETE FROM test WHERE id = $1", 42)
	assert.NoError(t, err)
}

func TestMemoryDB_QueryReturnsNil(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	result, err := db.Query("SELECT * FROM nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, result)

	result, err = db.Query("SELECT * FROM test WHERE id = $1", 42)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestMemoryDB_Close_Multiple(t *testing.T) {
	db := NewMemoryDB()

	err := db.Close()
	assert.NoError(t, err)
	assert.False(t, db.enabled.Load())

	// Second close should also work
	err = db.Close()
	assert.NoError(t, err)
}

func TestMemoryDB_HealthCheck_AfterClose(t *testing.T) {
	db := NewMemoryDB()

	// Should be healthy before close
	err := db.HealthCheck()
	assert.NoError(t, err)

	_ = db.Close()

	// Should be unhealthy after close
	err = db.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory database closed")
}

func TestMemoryDB_GetPool_AlwaysNil(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	assert.Nil(t, db.GetPool())
}

func TestMemoryDB_IsMemoryMode_AlwaysTrue(t *testing.T) {
	db := NewMemoryDB()
	defer func() { _ = db.Close() }()

	assert.True(t, db.IsMemoryMode())
}

// -----------------------------------------------------------------------------
// extractTableFromQuery Additional Tests
// -----------------------------------------------------------------------------

func TestExtractTableFromQuery_Extended(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "SimpleSelect",
			query:    "SELECT * FROM users",
			expected: "users",
		},
		{
			name:     "SelectWithWhereAndOrderBy",
			query:    "SELECT id FROM providers WHERE enabled = true ORDER BY name",
			expected: "providers",
		},
		{
			name:     "MixedCase",
			query:    "SELECT * From Users WHERE id = $1",
			expected: "users",
		},
		{
			name:     "AllUpperCase",
			query:    "SELECT * FROM USERS WHERE ID = $1",
			expected: "users",
		},
		{
			name:     "WithJoin",
			query:    "SELECT * FROM users JOIN sessions ON users.id = sessions.user_id",
			expected: "users",
		},
		{
			name:     "WithComma",
			query:    "SELECT * FROM users, sessions WHERE users.id = sessions.user_id",
			expected: "users",
		},
		{
			name:     "NoFromClause",
			query:    "SELECT 1",
			expected: "",
		},
		{
			name:     "EmptyString",
			query:    "",
			expected: "",
		},
		{
			name:     "OnlyFrom",
			query:    "FROM",
			expected: "",
		},
		{
			name:     "FromWithTrailingSpace",
			query:    "SELECT * FROM  ",
			expected: "",
		},
		{
			name:     "SubqueryInFrom",
			query:    "SELECT * FROM llm_responses WHERE request_id IN (SELECT id FROM llm_requests)",
			expected: "llm_responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTableFromQuery(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// -----------------------------------------------------------------------------
// memoryRow Additional Tests
// -----------------------------------------------------------------------------

func TestMemoryRow_Scan_NilValues(t *testing.T) {
	row := &memoryRow{
		values: nil,
		err:    nil,
	}

	// nil values should mean no rows
	var val string
	err := row.Scan(&val)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestMemoryRow_Scan_ErrorTakesPrecedence(t *testing.T) {
	row := &memoryRow{
		values: []any{"should-not-scan"},
		err:    assert.AnError,
	}

	var val string
	err := row.Scan(&val)
	assert.Equal(t, assert.AnError, err)
	assert.Empty(t, val)
}

func TestMemoryRow_Scan_BoolFalse(t *testing.T) {
	row := &memoryRow{
		values: []any{false},
		err:    nil,
	}

	var b bool
	err := row.Scan(&b)
	assert.NoError(t, err)
	assert.False(t, b)
}
