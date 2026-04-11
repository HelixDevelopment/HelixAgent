package database

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Pool Method - Additional Coverage
// ============================================================================

// TestPool_EnsureConnectedFails covers line 160-162 error path
func TestPool_EnsureConnectedFails(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Pool should return nil when ensureConnected fails
	pool := client.Pool()
	assert.Nil(t, pool)
}

// TestPool_TestPGNil_ReturnsNilPool covers line 165-167
func TestPool_TestPGNil_ReturnsNilPool(t *testing.T) {
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// When testPG is set, Pool returns nil (line 165-166)
	pool := client.Pool()
	assert.Nil(t, pool)
}

// ============================================================================
// Ping Method - Additional Coverage
// ============================================================================

// TestPing_EnsureConnectedFails covers line 188-189
func TestPing_EnsureConnectedFails(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Ping should fail when ensureConnected fails
	err = client.Ping()
	assert.Error(t, err)
}

// ============================================================================
// HealthCheck Method - Additional Coverage
// ============================================================================

// TestHealthCheck_EnsureConnectedFails covers line 199-200
func TestHealthCheck_EnsureConnectedFails(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// HealthCheck should fail when ensureConnected fails
	err = client.HealthCheck()
	assert.Error(t, err)
}

// ============================================================================
// Exec Method - Additional Coverage
// ============================================================================

// TestExec_EnsureConnectedFails covers line 213-214
func TestExec_EnsureConnectedFails(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Exec should fail when ensureConnected fails
	err = client.Exec("SELECT 1")
	assert.Error(t, err)
}

// TestExec_RealPGPathError covers line 220-221 error path
func TestExec_RealPGPathError(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Exec with real pg path (testPG is nil)
	err = client.Exec("SELECT 1")
	assert.Error(t, err)
}

// ============================================================================
// Query Method - Additional Coverage
// ============================================================================

// TestQuery_EnsureConnectedFails covers line 227-228
func TestQuery_EnsureConnectedFails(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Query should fail when ensureConnected fails
	results, err := client.Query("SELECT 1")
	assert.Error(t, err)
	assert.Nil(t, results)
}

// TestQuery_RealPGPath covers line 234-236
func TestQuery_RealPGPath(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Query with real pg path (testPG is nil)
	results, err := client.Query("SELECT 1")
	assert.Error(t, err)
	assert.Nil(t, results)
}

// ============================================================================
// QueryRow Method - Additional Coverage
// ============================================================================

// TestQueryRow_EnsureConnectedFails covers line 254-256
func TestQueryRow_EnsureConnectedFails(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// QueryRow should return ErrorRow when ensureConnected fails
	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)

	// Scan should return the connection error
	var dest int
	err = row.Scan(&dest)
	assert.Error(t, err)
}

// ============================================================================
// Begin Method - Additional Coverage
// ============================================================================

// TestBegin_InitConnectionFails covers line 267-268
func TestBegin_InitConnectionFails(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Begin should fail when initConnection fails
	tx, err := client.Begin(context.Background())
	assert.Error(t, err)
	assert.Nil(t, tx)
}

// ============================================================================
// Migrate Method - Additional Coverage
// ============================================================================

// TestMigrate_InitConnectionFails covers line 278-279
func TestMigrate_InitConnectionFails(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	migrations := []string{"CREATE TABLE test (id INT)"}

	// Migrate should fail when initConnection fails
	err = client.Migrate(context.Background(), migrations)
	assert.Error(t, err)
}

// TestMigrate_RealPGPath covers line 281
func TestMigrate_RealPGPath(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	migrations := []string{"CREATE TABLE test (id INT)"}

	// Migrate with real pg path
	err = client.Migrate(context.Background(), migrations)
	assert.Error(t, err) // Will fail due to no connection
}

// ============================================================================
// initConnection Method - Additional Coverage
// ============================================================================

// TestInitConnection_ConnectionError covers line 63 error path
func TestInitConnection_ConnectionError(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// This will fail at pg.Connect (line 63)
	err = client.initConnection(context.Background())
	assert.Error(t, err)
	assert.NotNil(t, client.connectErr)
}

// TestInitConnection_PoolAssignment covers line 65-66
func TestInitConnection_PoolAssignment_Final(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// This exercises pool assignment code (lines 65-66) even on failure
	_ = client.initConnection(context.Background())
	// connectErr should be set
	assert.NotNil(t, client.connectErr)
}

// ============================================================================
// buildPostgresConfig - Additional Coverage
// ============================================================================

// TestBuildPostgresConfig_AllEnvVars covers lines 116-146 env var branches
func TestBuildPostgresConfig_AllEnvVars(t *testing.T) {
	// Set all env vars
	os.Setenv("DB_HOST", "envhost")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_USER", "envuser")
	os.Setenv("DB_PASSWORD", "envpass")
	os.Setenv("DB_NAME", "envdb")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
	}()

	cfg := &config.Config{}
	result := buildPostgresConfig(cfg)

	assert.Equal(t, "envhost", result.Host)
	assert.Equal(t, 5433, result.Port)
	assert.Equal(t, "envuser", result.User)
	assert.Equal(t, "envpass", result.Password)
	assert.Equal(t, "envdb", result.DBName)
}

// TestBuildPostgresConfig_InvalidConfigPort covers line 121-122
func TestBuildPostgresConfig_InvalidConfigPort(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Port: "not-a-number",
		},
	}
	result := buildPostgresConfig(cfg)

	// Should use default port when config port is invalid
	assert.Equal(t, 5432, result.Port)
}

// TestBuildPostgresConfig_SSLModeFromConfig covers line 148-150
func TestBuildPostgresConfig_SSLModeFromConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			SSLMode: "require",
		},
	}
	result := buildPostgresConfig(cfg)

	assert.Equal(t, "require", result.SSLMode)
}

// TestBuildPostgresConfig_ApplicationName covers line 152
func TestBuildPostgresConfig_ApplicationName(t *testing.T) {
	cfg := &config.Config{}
	result := buildPostgresConfig(cfg)

	assert.Equal(t, "helixagent", result.ApplicationName)
}

// ============================================================================
// ErrorRow - Additional Coverage
// ============================================================================

// TestErrorRow_Scan_ErrorPreserved ensures error is preserved
func TestErrorRow_Scan_ErrorPreserved(t *testing.T) {
	testErr := errors.New("connection failed")
	row := &ErrorRow{err: testErr}

	var dest string
	err := row.Scan(&dest)

	assert.Equal(t, testErr, err)
}

// ============================================================================
// MemoryDB - Additional Coverage
// ============================================================================

// TestMemoryDB_HealthCheck_Enabled covers line 214
func TestMemoryDB_HealthCheck_Enabled_Final(t *testing.T) {
	m := NewMemoryDB()

	// Health check should pass when enabled
	err := m.HealthCheck()
	assert.NoError(t, err)
}

// TestMemoryDB_HealthCheck_Disabled covers line 211-213
func TestMemoryDB_HealthCheck_Disabled_Final(t *testing.T) {
	m := NewMemoryDB()

	// Close to disable
	err := m.Close()
	require.NoError(t, err)

	// Health check should fail when disabled
	err = m.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// ============================================================================
// Query Real PG Path
// ============================================================================

// TestQuery_TestPGNil_QueryError covers line 234-236 error
func TestQuery_TestPGNil_QueryError(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Query with real pg path - will fail
	results, err := client.Query("SELECT * FROM test")
	assert.Error(t, err)
	assert.Nil(t, results)
}

// ============================================================================
// Close Method - Additional Coverage
// ============================================================================

// TestClose_RealPGPath covers line 183
func TestClose_RealPGPath(t *testing.T) {
	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Close with real pg path (testPG is nil)
	err = client.Close()
	// Should not error even if not connected
	assert.NoError(t, err)
}

// ============================================================================
// Ping with Real PG Path
// ============================================================================

// TestPing_RealPGPathError covers line 194
func TestPing_RealPGPathError(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Ping with real pg path
	err = client.Ping()
	assert.Error(t, err)
}

// ============================================================================
// HealthCheck with Real PG Path
// ============================================================================

// TestHealthCheck_RealPGPathError covers line 207
func TestHealthCheck_RealPGPathError(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// HealthCheck with real pg path
	err = client.HealthCheck()
	assert.Error(t, err)
}

// ============================================================================
// QueryRow with Real PG Path
// ============================================================================

// TestQueryRow_RealPGPathError covers line 262
func TestQueryRow_RealPGPathError(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// QueryRow with real pg path
	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)

	var dest int
	err = row.Scan(&dest)
	assert.Error(t, err)
}

// ============================================================================
// Begin with Real PG Path
// ============================================================================

// TestBegin_RealPGPathError covers line 273
func TestBegin_RealPGPathError(t *testing.T) {
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

	// Reset to force connection attempt
	client.connectOnce = sync.Once{}

	// Begin with real pg path
	tx, err := client.Begin(context.Background())
	assert.Error(t, err)
	assert.Nil(t, tx)
}

// ============================================================================
// Context Deadline Tests
// ============================================================================

// TestInitConnection_WithExistingDeadline covers line 58
func TestInitConnection_WithExistingDeadline_Success(t *testing.T) {
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// Reset sync.Once
	client.connectOnce = sync.Once{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Context has deadline, so line 58 condition is true
	err := client.initConnection(ctx)
	assert.NoError(t, err)
}

// TestInitConnection_WithoutDeadline covers line 59-62
func TestInitConnection_WithoutDeadline_Success(t *testing.T) {
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

	// Reset sync.Once
	client.connectOnce = sync.Once{}

	ctx := context.Background()

	// Context has no deadline, so line 59-62 will add timeout
	err = client.initConnection(ctx)
	assert.Error(t, err) // Will fail due to invalid host
}

// ============================================================================
// TestPG Hook Path
// ============================================================================

// TestInitConnection_TestPGHook covers line 53-55
func TestInitConnection_TestPGHook_Final(t *testing.T) {
	mock := &mockDatabase{}
	client := newTestClient(mock)

	// Reset sync.Once
	client.connectOnce = sync.Once{}

	// testPG is set, so line 53-55 should return early with nil error
	err := client.initConnection(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, client.connectErr)
}

// ============================================================================
// QueryRow TestPG Path
// ============================================================================

// TestQueryRow_TestPG_ReturnsRow covers line 259-260
func TestQueryRow_TestPG_ReturnsRow(t *testing.T) {
	mockRow := mockRow{}
	mock := &mockDatabase{queryRowResult: mockRow}
	client := newTestClient(mock)

	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)
	assert.True(t, mock.queryRowCalled)
}

// ============================================================================
// Begin TestPG Path
// ============================================================================

// TestBegin_TestPG_ReturnsTx covers line 270-271
func TestBegin_TestPG_ReturnsTx(t *testing.T) {
	mockTx := &mockTx{}
	mock := &mockDatabase{beginTx: mockTx}
	client := newTestClient(mock)

	tx, err := client.Begin(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, tx)
	assert.True(t, mock.beginCalled)
}
