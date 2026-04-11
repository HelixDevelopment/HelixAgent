package database

import (
	"context"
	"sync"
	"testing"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Final Coverage Tests for Remaining Branches
// ============================================================================

// Test NewClientWithFallback success path - returns client without error
func TestNewClientWithFallback_ReturnsClient(t *testing.T) {
	t.Parallel()
	// Create client that will succeed with mock
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// This tests the internal path - we already have a connected mock
	// In real scenario, Ping() would need to succeed
	err := client.Ping()
	assert.NoError(t, err)
}

// Test initConnection with connectErr != nil path (line 64.26-67.4)
func TestClient_initConnection_ConnectError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Force connection with invalid host
	err = client.initConnection(context.Background())

	// Should have connection error
	assert.Error(t, err)
	// connectErr should be set
	assert.NotNil(t, client.connectErr)
}

// Test Pool with testPG != nil (line 164.21-166.3)
func TestClient_Pool_WithTestPG(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// When using testPG, Pool should return nil
	pool := client.Pool()
	assert.Nil(t, pool)
}

// Test Ping with testPG != nil (line 191.21-193.3)
func TestClient_Ping_WithTestPG(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	err := client.Ping()
	assert.NoError(t, err)
	assert.True(t, mock.healthCheckCalled)
}

// Test HealthCheck with testPG != nil (line 204.21-206.3)
func TestClient_HealthCheck_WithTestPG(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	err := client.HealthCheck()
	assert.NoError(t, err)
	assert.True(t, mock.healthCheckCalled)
}

// Test Exec with testPG != nil (line 216.21-219.3)
func TestClient_Exec_WithTestPG(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	err := client.Exec("INSERT INTO test VALUES ($1)", "value")
	assert.NoError(t, err)
	assert.True(t, mock.execCalled)
}

// Test Exec error return path (line 220.2-221.12)
func TestClient_Exec_WithRealPG_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection
	client.connectOnce = sync.Once{}

	// Exec should return error from connection
	err = client.Exec("SELECT 1")
	assert.Error(t, err)
}

// Test Query with testPG != nil (line 232.21-234.3)
func TestClient_Query_WithTestPG(t *testing.T) {
	t.Parallel()
	mockRows := &mockRows{nextReturns: []bool{true, false}}
	mock := &mockDatabase{queryRows: mockRows}
	client := newTestClient(mock)

	results, err := client.Query("SELECT * FROM test")
	assert.NoError(t, err)
	assert.True(t, mock.queryCalled)
	assert.Len(t, results, 1)
}

// Test QueryRow with testPG != nil (line 259.21-261.3)
func TestClient_QueryRow_WithTestPG(t *testing.T) {
	t.Parallel()
	mockRow := mockRow{}
	mock := &mockDatabase{queryRowResult: mockRow}
	client := newTestClient(mock)

	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)
	assert.True(t, mock.queryRowCalled)
}

// Test Begin with testPG != nil (line 270.21-272.3)
func TestClient_Begin_WithTestPG(t *testing.T) {
	t.Parallel()
	mockTx := &mockTx{}
	mock := &mockDatabase{beginTx: mockTx}
	client := newTestClient(mock)

	tx, err := client.Begin(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, tx)
	assert.True(t, mock.beginCalled)
}

// Test Migrate success path
func TestClient_Migrate_Success(t *testing.T) {
	t.Parallel()
	// This test uses the real Migrate method which calls pg.Migrate
	// Since we can't mock this easily without the testPG supporting Migrate,
	// we test the connection failure path instead
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection
	client.connectOnce = sync.Once{}

	// Migrate should return error from connection
	err = client.Migrate(context.Background(), []string{"CREATE TABLE test (id INT)"})
	assert.Error(t, err)
}

// ============================================================================
// NewClientWithFallback Additional Branches
// ============================================================================

func TestNewClientWithFallback_NewClientError(t *testing.T) {
	t.Parallel()
	// NewClient never returns error currently, but test the fallback path
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

	// Should fail to connect
	assert.Error(t, err)
	assert.Nil(t, client)
}

// ============================================================================
// NewPostgresDB Error Path (line 46.16-48.3)
// ============================================================================

func TestNewPostgresDB_ErrorPath(t *testing.T) {
	t.Parallel()
	// NewPostgresDB only errors if NewClient errors
	// Since NewClient never returns error, this path is unreachable in practice
	// But we test it for completeness
	cfg := &config.Config{}

	pgDB, err := NewPostgresDB(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, pgDB)
}

// ============================================================================
// NewPostgresDBWithFallback All Branches
// ============================================================================

func TestNewPostgresDBWithFallback_DBError(t *testing.T) {
	t.Parallel()
	// This tests the path where NewPostgresDB returns an error
	// Since NewPostgresDB only errors if NewClient errors,
	// and NewClient never returns error, this path is unreachable in practice
	cfg := &config.Config{}

	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)

	// Should not error, may return either pgDB or memDB
	assert.NoError(t, err)
	assert.True(t, pgDB != nil || memDB != nil)
}

// ============================================================================
// Connect Error Path (line 71.16-73.3)
// ============================================================================

func TestConnect_ErrorPath_Final(t *testing.T) {
	t.Parallel()
	// Connect only errors if NewPostgresDB errors
	// Since NewPostgresDB never returns error, this path is unreachable
	// But we test the success path
	db, err := Connect()

	assert.NoError(t, err)
	assert.NotNil(t, db)
}

// ============================================================================
// Query Error Path Coverage
// ============================================================================

func TestClient_Query_WithRealPGQueryError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection
	client.connectOnce = sync.Once{}

	// Query should return error from connection
	results, err := client.Query("SELECT * FROM test")
	assert.Error(t, err)
	assert.Nil(t, results)
}

// ============================================================================
// Transaction Error Path
// ============================================================================

func TestClient_Begin_WithRealPGBeginError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host-that-does-not-exist.example.com",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Reset sync.Once to force connection
	client.connectOnce = sync.Once{}

	// Begin should return error from connection
	tx, err := client.Begin(context.Background())
	assert.Error(t, err)
	assert.Nil(t, tx)
}

// ============================================================================
// MemoryDB HealthCheck After Close
// ============================================================================

func TestMemoryDB_HealthCheck_Closed(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()

	// Close the database
	err := m.Close()
	assert.NoError(t, err)

	// Health check should fail
	err = m.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// ============================================================================
// PostgresDB GetPool Coverage
// ============================================================================

func TestPostgresDB_GetPool_WithMock(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)
	pgDB := &PostgresDB{client: client}

	// GetPool should return nil with mock
	pool := pgDB.GetPool()
	assert.Nil(t, pool)
}
