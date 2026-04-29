package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Client Close with Real PG
// ============================================================================

func TestClient_Close_RealClient(t *testing.T) {
	t.Parallel()
	// Create client with real postgres client (but not connected)
	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Close should work even if not connected
	err = client.Close()
	// No error expected - Close handles nil client gracefully
	assert.NoError(t, err)
}

// ============================================================================
// initConnection Context Deadline Branch
// ============================================================================

func TestClient_initConnection_WithExistingDeadline_EdgeCase(t *testing.T) {
	t.Parallel()
	// Create client
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

	// Create context with deadline
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// This will fail to connect but tests the branch with existing deadline
	err = client.initConnection(ctx)
	// Should get connection error
	assert.Error(t, err)
}

func TestClient_initConnection_WithoutExistingDeadline_EdgeCase(t *testing.T) {
	t.Parallel()
	// Create client
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

	// Reset sync.Once to force reconnection
	client.connectOnce = sync.Once{}

	// Use context without deadline - will trigger the timeout branch
	ctx := context.Background()

	// This will fail to connect but tests the branch without deadline
	err = client.initConnection(ctx)
	// Should get connection error
	assert.Error(t, err)
}

// ============================================================================
// Client Methods with Connection Success/Failure
// ============================================================================

func TestClient_Pool_NotConnected(t *testing.T) {
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

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	// Pool should return nil when not connected
	pool := client.Pool()
	assert.Nil(t, pool)
}

// ============================================================================
// Client Ping with All Branches
// ============================================================================

func TestClient_Ping_NotConnected_Error(t *testing.T) {
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

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	err = client.Ping()
	assert.Error(t, err)
}

func TestClient_Ping_WithMockSuccess(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{healthCheckErr: nil}
	client := newTestClient(mock)

	err := client.Ping()
	assert.NoError(t, err)
	assert.True(t, mock.healthCheckCalled)
}

// ============================================================================
// Client HealthCheck with All Branches
// ============================================================================

func TestClient_HealthCheck_NotConnected_Error(t *testing.T) {
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

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	err = client.HealthCheck()
	assert.Error(t, err)
}

// ============================================================================
// Client Exec with All Branches
// ============================================================================

func TestClient_Exec_NotConnected_Error(t *testing.T) {
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

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	err = client.Exec("SELECT 1")
	assert.Error(t, err)
}

func TestClient_Exec_WithMockErrorCase(t *testing.T) {
	t.Parallel()
	mock := &mockDatabase{execErr: errors.New("exec failed")}
	client := newTestClient(mock)

	err := client.Exec("INSERT INTO test VALUES ($1)", "value")
	assert.Error(t, err)
	assert.Equal(t, "exec failed", err.Error())
}

// ============================================================================
// Client Query with All Branches
// ============================================================================

func TestClient_Query_NotConnected_Error(t *testing.T) {
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

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	results, err := client.Query("SELECT * FROM test")
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestClient_Query_NoRows_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	mockRows := &mockRows{nextReturns: []bool{false}}
	mock := &mockDatabase{queryRows: mockRows}
	client := newTestClient(mock)

	results, err := client.Query("SELECT * FROM test")
	assert.NoError(t, err)
	assert.Empty(t, results)
}

// ============================================================================
// Client QueryRow with All Branches
// ============================================================================

func TestClient_QueryRow_NotConnected_Error(t *testing.T) {
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

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)

	// Scan should return error
	var val int
	err = row.Scan(&val)
	assert.Error(t, err)
}

func TestClient_QueryRow_WithMockSuccess(t *testing.T) {
	t.Parallel()
	mockRow := mockRow{}
	mock := &mockDatabase{queryRowResult: mockRow}
	client := newTestClient(mock)

	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)
	assert.True(t, mock.queryRowCalled)
}

// ============================================================================
// Client Begin with All Branches
// ============================================================================

func TestClient_Begin_NotConnected_Error(t *testing.T) {
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

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	tx, err := client.Begin(context.Background())
	assert.Error(t, err)
	assert.Nil(t, tx)
}

func TestClient_Begin_WithMockSuccess(t *testing.T) {
	t.Parallel()
	mockTx := &mockTx{}
	mock := &mockDatabase{beginTx: mockTx}
	client := newTestClient(mock)

	tx, err := client.Begin(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, tx)
	assert.True(t, mock.beginCalled)
}

// ============================================================================
// Client Migrate with All Branches
// ============================================================================

func TestClient_Migrate_NotConnected_Error(t *testing.T) {
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

	// Reset sync.Once to force connection attempt
	client.connectOnce = sync.Once{}

	err = client.Migrate(context.Background(), []string{"CREATE TABLE test (id INT)"})
	assert.Error(t, err)
}

// ============================================================================
// NewClientWithFallback Success Path
// ============================================================================

func TestNewClientWithFallback_WithRealConnection(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	client, err := NewClientWithFallback(cfg)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	assert.NotNil(t, client)
	assert.NoError(t, client.Ping())

	client.Close()
}

// ============================================================================
// NewPostgresDB Error Path
// ============================================================================

func TestNewPostgresDB_NoError(t *testing.T) {
	t.Parallel()
	// Create a config
	// NewClient doesn't return errors for invalid configs (connection is lazy)
	// So this test verifies the NewPostgresDB creates a client successfully
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host: "localhost",
			Port: "5432",
		},
	}

	pgDB, err := NewPostgresDB(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, pgDB)
}

// ============================================================================
// NewPostgresDBWithFallback All Branches
// ============================================================================

func TestNewPostgresDBWithFallback_ConnectError_Fallback(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "invalid-host",
			Port:     "1",
			User:     "test",
			Password: "test",
			Name:     "test",
		},
	}

	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)

	// Should not error - fallback to memory
	assert.NoError(t, err)
	assert.Nil(t, pgDB)
	assert.NotNil(t, memDB)
}

func TestNewPostgresDBWithFallback_PingFails_Fallback(t *testing.T) {
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

	// Should not error - fallback to memory
	assert.NoError(t, err)
	// We should have memory DB since ping will fail
	assert.NotNil(t, memDB)
	assert.Nil(t, pgDB)
}

// ============================================================================
// Connect Error Path
// ============================================================================

func TestConnect_Success(t *testing.T) {
	t.Parallel()
	// Connect creates a client with empty config
	// The connection won't happen until used (lazy)
	db, err := Connect()

	// Client creation succeeds
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// But ping should fail since no real PostgreSQL
	err = db.Ping()
	// If PostgreSQL is available, this might succeed
	_ = err
}
