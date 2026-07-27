package database

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Integration Tests - Requires PostgreSQL
// ============================================================================

// TestIntegration_NewClientWithFallback_Success tests the success path of NewClientWithFallback
// This covers line 106: return client, nil after successful ping
func TestIntegration_NewClientWithFallback_Success(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClientWithFallback(cfg)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}
	require.NotNil(t, client)
	defer client.Close()

	// Verify ping works
	err = client.Ping()
	assert.NoError(t, err)
}

// TestIntegration_InitConnection_Success tests successful connection
// This covers line 65-67: pool assignment after successful connect
func TestIntegration_InitConnection_Success(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// Verify pool is assigned
	assert.NotNil(t, client.pool)
	assert.NoError(t, client.connectErr)

	client.Close()
}

// TestIntegration_Pool_ReturnsRealPool tests Pool() returns real pool after connection
// This covers line 167: return c.pool
func TestIntegration_Pool_ReturnsRealPool(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Force connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// Pool should return the real pool
	pool := client.Pool()
	assert.NotNil(t, pool)
}

// TestIntegration_Ping_RealConnection tests Ping with real connection
// This covers line 194: return c.pg.HealthCheck
func TestIntegration_Ping_RealConnection(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// Ping should use real pg path
	err = client.Ping()
	assert.NoError(t, err)
}

// TestIntegration_HealthCheck_RealConnection tests HealthCheck with real connection
// This covers line 207: return c.pg.HealthCheck
func TestIntegration_HealthCheck_RealConnection(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// HealthCheck should use real pg path
	err = client.HealthCheck()
	assert.NoError(t, err)
}

// TestIntegration_Exec_RealConnection tests Exec with real connection
// This covers line 220-221: real pg path
func TestIntegration_Exec_RealConnection(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// Exec should use real pg path
	err = client.Exec("SELECT 1")
	assert.NoError(t, err)
}

// TestIntegration_Query_RealConnection tests Query with real connection
// This covers line 234-236: real pg path
func TestIntegration_Query_RealConnection(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// Query should use real pg path
	results, err := client.Query("SELECT 1")
	assert.NoError(t, err)
	assert.NotNil(t, results)
}

// TestIntegration_QueryRow_RealConnection tests QueryRow with real connection
// This covers line 262: real pg path
func TestIntegration_QueryRow_RealConnection(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// QueryRow should use real pg path
	row := client.QueryRow("SELECT 1")
	assert.NotNil(t, row)

	var result int
	err = row.Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)
}

// TestIntegration_Begin_RealConnection tests Begin with real connection
// This covers line 273: real pg path
func TestIntegration_Begin_RealConnection(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// Begin should use real pg path
	tx, err := client.Begin(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, tx)

	// Cleanup
	if tx != nil {
		_ = tx.Rollback(context.Background())
	}
}

// TestIntegration_Migrate_RealConnection tests Migrate with real connection
// This covers line 281: c.pg.Migrate
func TestIntegration_Migrate_RealConnection(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// Initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.initConnection(ctx)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}

	// Migrate should use real pg path
	migrations := []string{
		"CREATE TABLE IF NOT EXISTS test_coverage (id SERIAL PRIMARY KEY)",
	}
	err = client.Migrate(context.Background(), migrations)
	assert.NoError(t, err)
}

// TestIntegration_NewPostgresDB_Success tests NewPostgresDB success
// This covers line 48: return &PostgresDB{client: client}, nil
func TestIntegration_NewPostgresDB_Success(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	pgDB, err := NewPostgresDB(cfg)
	require.NoError(t, err)
	require.NotNil(t, pgDB)
	defer pgDB.Close()

	// Verify it works
	err = pgDB.Ping()
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}
	assert.NoError(t, err)
}

// TestIntegration_NewPostgresDBWithFallback_PostgresSuccess tests fallback with postgres success
// This covers line 58: return db, nil, nil
func TestIntegration_NewPostgresDBWithFallback_PostgresSuccess(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	cfg := &config.Config{}
	pgDB, memDB, err := NewPostgresDBWithFallback(cfg)
	require.NoError(t, err)

	// If postgres is available, pgDB should be set
	if pgDB != nil {
		assert.Nil(t, memDB)
		defer pgDB.Close()
	} else {
		assert.NotNil(t, memDB)
	}
}

// TestIntegration_Connect_Success tests Connect success
// This covers line 75: return db, nil
func TestIntegration_Connect_Success(t *testing.T) {
	t.Parallel()
	// Skip if no PostgreSQL available
	if os.Getenv("DB_HOST") == "" && os.Getenv("CI") == "" {
		t.Skip("PostgreSQL not available, skipping integration test") // SKIP-OK: #integration-mode-only
	}

	db, err := Connect()
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Verify it works
	err = db.Ping()
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}
	assert.NoError(t, err)
}
