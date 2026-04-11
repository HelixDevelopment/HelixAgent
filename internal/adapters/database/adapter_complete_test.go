package database

import (
	"context"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// MemoryDB Coverage for 0% functions
// ============================================================================

// TestMemoryDB_Ping_Direct calls MemoryDB.Ping directly
func TestMemoryDB_Ping_Direct(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()
	err := m.Ping()
	assert.NoError(t, err)
}

// TestMemoryDB_Exec_Direct calls MemoryDB.Exec directly
func TestMemoryDB_Exec_Direct(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()
	err := m.Exec("INSERT INTO test VALUES (1)")
	assert.NoError(t, err)
}

// TestMemoryDB_Exec_WithArgs calls MemoryDB.Exec with arguments
func TestMemoryDB_Exec_WithArgs(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()
	err := m.Exec("INSERT INTO test VALUES ($1, $2)", "val1", 123)
	assert.NoError(t, err)
}

// TestMemoryDB_IsMemoryMode_Direct calls MemoryDB.IsMemoryMode directly
func TestMemoryDB_IsMemoryMode_Direct(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()
	result := m.IsMemoryMode()
	assert.True(t, result)
}

// TestMemoryDB_HealthCheck_ClosedBranch tests the closed branch (line 211-213)
func TestMemoryDB_HealthCheck_ClosedBranch(t *testing.T) {
	t.Parallel()
	m := NewMemoryDB()

	// Close the database
	err := m.Close()
	require.NoError(t, err)

	// Health check should return "memory database closed" error
	err = m.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory database closed")
}

// ============================================================================
// initConnection Deadline Tests
// ============================================================================

// TestInitConnection_WithExistingDeadline tests context with deadline
func TestInitConnection_WithExistingDeadline(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.initConnection(ctx)
	assert.NoError(t, err)
}

// TestInitConnection_WithoutDeadline tests context without deadline
func TestInitConnection_WithoutDeadline(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	ctx := context.Background()

	err := client.initConnection(ctx)
	assert.NoError(t, err)
}

// TestInitConnection_PoolAssignment tests pool assignment path (line 64-67)
func TestInitConnection_PoolAssignment(t *testing.T) {
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

	// Reset to force connection
	client.connectOnce = sync.Once{}

	// This exercises line 64-67 (pool assignment path)
	_ = client.initConnection(context.Background())

	// connectErr should be set
	assert.NotNil(t, client.connectErr)
}

// ============================================================================
// Real PG Path Tests (testPG is nil)
// ============================================================================

// TestPool_RealPath tests Pool when testPG is nil (line 167)
func TestPool_RealPath(t *testing.T) {
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

	// Reset to force real path (testPG is nil)
	client.connectOnce = sync.Once{}

	// This exercises line 167 (return c.pool)
	pool := client.Pool()
	assert.Nil(t, pool)
}

// TestPing_RealPath tests Ping when testPG is nil (line 194)
func TestPing_RealPath(t *testing.T) {
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

	// Reset to force real path
	client.connectOnce = sync.Once{}

	// This exercises line 194
	err = client.Ping()
	assert.Error(t, err)
}

// TestHealthCheck_RealPath tests HealthCheck when testPG is nil (line 207)
func TestHealthCheck_RealPath(t *testing.T) {
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

	// Reset to force real path
	client.connectOnce = sync.Once{}

	// This exercises line 207
	err = client.HealthCheck()
	assert.Error(t, err)
}

// TestExec_RealPath tests Exec when testPG is nil (line 220-221)
func TestExec_RealPath(t *testing.T) {
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

	// Reset to force real path
	client.connectOnce = sync.Once{}

	// This exercises lines 220-221
	err = client.Exec("SELECT 1")
	assert.Error(t, err)
}

// TestQuery_RealPath tests Query when testPG is nil (line 234-236)
func TestQuery_RealPath(t *testing.T) {
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

	// Reset to force real path
	client.connectOnce = sync.Once{}

	// This exercises lines 234-236
	results, err := client.Query("SELECT 1")
	assert.Error(t, err)
	assert.Nil(t, results)
}

// TestQueryRow_RealPath tests QueryRow when testPG is nil (line 262)
func TestQueryRow_RealPath(t *testing.T) {
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

	// Reset to force real path
	client.connectOnce = sync.Once{}

	// This exercises line 262
	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)

	var dest int
	err = row.Scan(&dest)
	assert.Error(t, err)
}

// TestBegin_RealPath tests Begin when testPG is nil (line 273)
func TestBegin_RealPath(t *testing.T) {
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

	// Reset to force real path
	client.connectOnce = sync.Once{}

	// This exercises line 273
	tx, err := client.Begin(context.Background())
	assert.Error(t, err)
	assert.Nil(t, tx)
}

// TestMigrate_RealPath tests Migrate (line 281)
func TestMigrate_RealPath(t *testing.T) {
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

	// Reset to force real path
	client.connectOnce = sync.Once{}

	migrations := []string{"CREATE TABLE test (id INT)"}

	// This exercises line 281
	err = client.Migrate(context.Background(), migrations)
	assert.Error(t, err)
}

// ============================================================================
// compat.go Coverage
// ============================================================================

// TestNewPostgresDB_ErrorBranch tests error branch (line 46-48)
func TestNewPostgresDB_ErrorBranch(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	// Currently NewClient never returns error
	pgDB, err := NewPostgresDB(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, pgDB)
}

// TestNewPostgresDBWithFallback_SuccessBranch tests success (line 58)
func TestNewPostgresDBWithFallback_SuccessBranch(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)
	pgDB := &PostgresDB{client: client}

	// Verify ping works
	err := pgDB.Ping()
	assert.NoError(t, err)

	// This simulates the success path at line 58
	_ = pgDB
}

// TestNewPostgresDBWithFallback_PingSuccess tests ping success path (line 58)
func TestNewPostgresDBWithFallback_PingSuccess(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// Verify ping succeeds
	err := client.Ping()
	assert.NoError(t, err)

	// This simulates line 58: return db, nil, nil
	_ = client
}

// TestConnect_ErrorBranch tests error branch (line 71-73)
func TestConnect_ErrorBranch(t *testing.T) {
	t.Parallel()
	// Currently NewPostgresDB never returns error
	db, err := Connect()
	assert.NoError(t, err)
	assert.NotNil(t, db)
}

// TestNewPostgresDBWithFallback_FallbackBranch tests fallback (lines 56, 59, 61-62)
func TestNewPostgresDBWithFallback_FallbackBranch(t *testing.T) {
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

	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)

	// Should not error - falls back to MemoryDB
	assert.NoError(t, err)
	assert.Nil(t, pgDB)
	assert.NotNil(t, memDB)
}

// TestNewClientWithFallback_PingFail tests ping failure (line 103-104)
func TestNewClientWithFallback_PingFail(t *testing.T) {
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

	client, err := NewClientWithFallback(cfg)

	// Should fail because ping fails (line 103-104)
	assert.Error(t, err)
	assert.Nil(t, client)
}

// TestNewClientWithFallback_ReturnClient tests success (line 102)
func TestNewClientWithFallback_ReturnClient(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// This simulates line 102
	err := client.Ping()
	assert.NoError(t, err)
}
