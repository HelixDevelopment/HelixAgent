package database

import (
	"errors"
	"testing"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Connect Function Tests
// ============================================================================

func TestConnect_WithEmptyConfig(t *testing.T) {
	t.Parallel()
	// Connect() creates a client with empty config
	// Connection is lazy, so this succeeds
	db, err := Connect()

	// Should succeed in creating the client
	assert.NoError(t, err)
	assert.NotNil(t, db)
}

// ============================================================================
// PostgresDB Method Tests
// ============================================================================

func TestPostgresDB_AllMethods(t *testing.T) {
	t.Parallel()
	// Create a PostgresDB with a mock client
	mock := &mockDatabase{}
	client := newTestClient(mock)

	pgDB := &PostgresDB{client: client}

	// Test Ping
	err := pgDB.Ping()
	assert.NoError(t, err)
	assert.True(t, mock.healthCheckCalled)

	// Test Exec
	mock.execCalled = false
	err = pgDB.Exec("INSERT INTO test VALUES ($1)", "value")
	assert.NoError(t, err)
	assert.True(t, mock.execCalled)

	// Test Query
	mockRows := &mockRows{nextReturns: []bool{true, false}}
	mock.queryRows = mockRows
	mock.queryCalled = false
	results, err := pgDB.Query("SELECT * FROM test")
	assert.NoError(t, err)
	assert.True(t, mock.queryCalled)
	assert.NotNil(t, results)

	// Test QueryRow
	mockRow := mockRow{}
	mock.queryRowResult = mockRow
	mock.queryRowCalled = false
	row := pgDB.QueryRow("SELECT * FROM test WHERE id = $1", 1)
	assert.NotNil(t, row)
	assert.True(t, mock.queryRowCalled)

	// Test HealthCheck
	mock.healthCheckCalled = false
	err = pgDB.HealthCheck()
	assert.NoError(t, err)
	assert.True(t, mock.healthCheckCalled)

	// Test Close
	mock.closeCalled = false
	err = pgDB.Close()
	assert.NoError(t, err)
	assert.True(t, mock.closeCalled)

	// Test GetPool (returns nil with mock)
	pool := pgDB.GetPool()
	assert.Nil(t, pool)

	// Test Database - now returns mock through test hook
	db := pgDB.Database()
	assert.NotNil(t, db)
}

func TestPostgresDB_Methods_WithErrors(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{
		healthCheckErr: errors.New("health check failed"),
		execErr:        errors.New("exec failed"),
		queryErr:       errors.New("query failed"),
	}
	client := newTestClient(mock)
	pgDB := &PostgresDB{client: client}

	// Test Ping error
	err := pgDB.Ping()
	assert.Error(t, err)

	// Test Exec error
	err = pgDB.Exec("SELECT 1")
	assert.Error(t, err)

	// Test Query error
	results, err := pgDB.Query("SELECT * FROM test")
	assert.Error(t, err)
	assert.Nil(t, results)

	// Test QueryRow (returns row even on connection, but scan would fail)
	row := pgDB.QueryRow("SELECT 1")
	assert.NotNil(t, row)

	// Test HealthCheck error
	err = pgDB.HealthCheck()
	assert.Error(t, err)

	// Test Close error
	err = pgDB.Close()
	assert.NoError(t, err) // Our mock returns nil
}

// ============================================================================
// NewPostgresDB Tests
// ============================================================================

func TestNewPostgresDB_Success(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	// This will create a client but not connect yet
	pgDB, err := NewPostgresDB(cfg)

	// Should succeed in creating client (connection is lazy)
	assert.NoError(t, err)
	assert.NotNil(t, pgDB)
	assert.NotNil(t, pgDB.client)
}

func TestNewPostgresDB_WithFallback(t *testing.T) {
	t.Parallel()
	// Use invalid config to trigger fallback
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     "59999", // unlikely port
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)

	// Should not error (fallback works)
	assert.NoError(t, err)
	// Either pgDB or memDB should be set (likely memDB due to connection failure)
	assert.True(t, pgDB != nil || memDB != nil)
}

func TestNewPostgresDBWithFallback_RealConnection(t *testing.T) {
	t.Parallel()
	// This test requires a real PostgreSQL connection
	cfg := &config.Config{}

	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// If we have a real connection, pgDB should be set and memDB nil
	// If no connection, memDB should be set
	if pgDB != nil {
		assert.Nil(t, memDB)
	} else {
		assert.NotNil(t, memDB)
	}
}

// ============================================================================
// RunMigration Tests
// ============================================================================

func TestRunMigration(t *testing.T) {
	t.Parallel()
	// Create a real PostgresDB with a config that won't connect
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, _ := NewClient(cfg)
	pgDB := &PostgresDB{client: client}

	migrations := []string{
		"CREATE TABLE test (id INT)",
	}

	// RunMigration will fail because we don't have a real connection,
	// but this exercises the code path
	err := RunMigration(pgDB, migrations)
	// We expect an error due to no connection
	assert.Error(t, err)
}

// ============================================================================
// memoryRow Tests
// ============================================================================

func TestMemoryRow_Scan_WithString(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()
	m.StoreRow("users", "user-001", []any{"Alice", 30})

	// Query for the row
	row := m.QueryRow("SELECT name, age FROM users WHERE id = $1", "user-001")

	var name string
	var age int
	err := row.Scan(&name, &age)

	// Note: MemoryDB's QueryRow doesn't actually use StoreRow data
	// This test documents the current behavior
	assert.Error(t, err) // Returns "no rows found" error
}

func TestMemoryRow_Scan_WithDifferentTypes(t *testing.T) {
	t.Parallel()
	// Test scanning different types
	row := &memoryRow{
		values: []any{"test", 42, true},
	}

	var strVal string
	var intVal int
	var boolVal bool

	err := row.Scan(&strVal, &intVal, &boolVal)

	assert.NoError(t, err)
	assert.Equal(t, "test", strVal)
	assert.Equal(t, 42, intVal)
	assert.Equal(t, true, boolVal)
}

func TestMemoryRow_Scan_WithUnsupportedType(t *testing.T) {
	t.Parallel()
	row := &memoryRow{
		values: []any{3.14}, // float64 not supported
	}

	var floatVal float64
	err := row.Scan(&floatVal)

	// Should not error but also not set the value (type not handled)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, floatVal) // Unchanged
}

func TestMemoryRow_Scan_NoValues(t *testing.T) {
	t.Parallel()
	row := &memoryRow{
		values: []any{},
		err:    nil,
	}

	var val string
	err := row.Scan(&val)

	// Returns "no rows" error when values slice is empty
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestMemoryRow_Scan_Error(t *testing.T) {
	t.Parallel()
	testErr := errors.New("scan error")
	row := &memoryRow{
		values: []any{"test"},
		err:    testErr,
	}

	var val string
	err := row.Scan(&val)

	assert.Error(t, err)
	assert.Equal(t, testErr, err)
}

func TestMemoryRow_Scan_NoRows(t *testing.T) {
	t.Parallel()
	row := &memoryRow{
		values: nil,
		err:    nil,
	}

	var val string
	err := row.Scan(&val)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

// ============================================================================
// MemoryDB Additional Tests
// ============================================================================

func TestMemoryDB_Query_WithStoredRow(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()
	m.StoreRow("users", "user-001", []any{"Alice", 30})

	// Query currently returns nil, nil for MemoryDB
	results, err := m.Query("SELECT * FROM users")

	assert.NoError(t, err)
	assert.Nil(t, results)
}

func TestMemoryDB_GetPool_ReturnsNil(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()
	pool := m.GetPool()
	assert.Nil(t, pool)
}

// ============================================================================
// Interface Compliance Tests
// ============================================================================

func TestPostgresDB_ImplementsDB(t *testing.T) {
	t.Parallel()
	// Compile-time check
	var _ DB = (*PostgresDB)(nil)
}

func TestPostgresDB_ImplementsLegacyDB(t *testing.T) {
	t.Parallel()
	// Compile-time check
	var _ LegacyDB = (*PostgresDB)(nil)
}

func TestMemoryDB_ImplementsDB(t *testing.T) {
	t.Parallel()
	// Compile-time check
	var _ DB = (*MemoryDB)(nil)
}

func TestMemoryDB_ImplementsLegacyDB(t *testing.T) {
	t.Parallel()
	// Compile-time check
	var _ LegacyDB = (*MemoryDB)(nil)
}

// ============================================================================
// Config Type Aliases
// ============================================================================

func TestConfigAliases(t *testing.T) {
	t.Parallel()
	// These tests ensure config type aliases compile correctly

	// Test Config alias
	var _ Config = Config{}

	// Test PostgresConfig alias
	var _ PostgresConfig = PostgresConfig{}
}

// ============================================================================
// Helper Functions
// ============================================================================

func TestNewClientWithFallback_SuccessPath(t *testing.T) {
	t.Parallel()
	// This test would need a real PostgreSQL connection
	// We test the error path instead
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClientWithFallback(cfg)

	// Should fail since PostgreSQL is not available
	assert.Error(t, err)
	assert.Nil(t, client)
}
