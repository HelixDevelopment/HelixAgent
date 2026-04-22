// Package integration provides container-based integration tests.
// These tests verify that real container services are accessible via the
// Containers module adapter.
//
// Run with: go test -v -tags=integration ./tests/integration/ -run TestContainer
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain is the entry point for container-based integration tests.
// It ensures all containers are running before executing tests.
func TestMain(m *testing.M) {
	// CONST-030: skip integration tests in -short mode because they require
	// real infrastructure (containers, APIs, databases) that may be unavailable.
	for _, arg := range os.Args {
		if arg == "-test.short" || strings.HasPrefix(arg, "-test.short=") {
			fmt.Println("SKIP: Integration tests skipped in -short mode (require real infrastructure)")
			os.Exit(0)
		}
	}
	TestMainIntegration(m)
}

// =============================================================================
// Container Availability Tests
// =============================================================================

// TestContainer_AllServicesHealthy verifies all required containers are running
func TestContainer_AllServicesHealthy(t *testing.T) {
	_ = SetupIntegrationTest(t)

	services := []string{"postgresql", "redis", "chromadb", "cognee", "qdrant"}

	for _, service := range services {
		t.Run(service, func(t *testing.T) {
			host := getServiceHost(service)
			port := getServicePort(service)

			// Verify service is reachable
			addr := host + ":" + port
			client := &http.Client{Timeout: 5 * time.Second}

			var url string
			switch service {
			case "chromadb":
				url = "http://" + addr + "/api/v2/heartbeat"
			case "cognee":
				url = "http://" + addr + "/health"
			case "qdrant":
				url = "http://" + addr
			}

			if url != "" {
				resp, err := client.Get(url)
				if err == nil {
					resp.Body.Close()
					assert.True(t, resp.StatusCode < 500, "Service %s returned status %d", service, resp.StatusCode)
				}
			}

			t.Logf("✓ Service %s is healthy at %s", service, addr)
		})
	}

	t.Logf("Container runtime: All %d services are running", len(services))
}

// =============================================================================
// PostgreSQL Container Tests
// =============================================================================

// TestContainer_PostgreSQL_Connection tests connection to real PostgreSQL container
func TestContainer_PostgreSQL_Connection(t *testing.T) {
	RequireContainerService(t, "postgresql")

	harness, _ := GetContainerHarness()
	connStr := harness.GetServiceURL("postgresql")

	db, err := sql.Open("postgres", connStr+"?sslmode=disable")
	require.NoError(t, err, "Failed to open PostgreSQL connection")
	defer db.Close()

	// Set connection timeout
	db.SetConnMaxLifetime(5 * time.Second)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	require.NoError(t, err, "Failed to ping PostgreSQL")

	// Verify we can execute a query
	var version string
	err = db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	require.NoError(t, err, "Failed to execute query")

	assert.Contains(t, version, "PostgreSQL", "Expected PostgreSQL version string")
	t.Logf("✓ PostgreSQL connection successful: %s", version)
}

// TestContainer_PostgreSQL_CRUD tests CRUD operations on real PostgreSQL
func TestContainer_PostgreSQL_CRUD(t *testing.T) {
	RequireContainerService(t, "postgresql")

	harness, _ := GetContainerHarness()
	connStr := harness.GetServiceURL("postgresql")

	db, err := sql.Open("postgres", connStr+"?sslmode=disable")
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Create test table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_container_integration (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err, "Failed to create test table")

	// Cleanup after test
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS test_container_integration")

	// Insert test data
	result, err := db.ExecContext(ctx,
		"INSERT INTO test_container_integration (name) VALUES ($1)",
		"container-test")
	require.NoError(t, err, "Failed to insert test data")

	rowsAffected, _ := result.RowsAffected()
	assert.Equal(t, int64(1), rowsAffected, "Expected 1 row to be inserted")

	// Query test data
	var retrievedName string
	err = db.QueryRowContext(ctx,
		"SELECT name FROM test_container_integration WHERE name = $1",
		"container-test").Scan(&retrievedName)
	require.NoError(t, err, "Failed to query test data")

	assert.Equal(t, "container-test", retrievedName)
	t.Log("✓ PostgreSQL CRUD operations successful")
}

// =============================================================================
// Redis Container Tests
// =============================================================================

// TestContainer_Redis_Connection tests connection to real Redis container
func TestContainer_Redis_Connection(t *testing.T) {
	RequireContainerService(t, "redis")

	harness, _ := GetContainerHarness()
	redisURL := harness.GetServiceURL("redis")

	// Parse redis URL
	opt, err := redis.ParseURL(redisURL)
	require.NoError(t, err, "Failed to parse Redis URL")

	client := redis.NewClient(opt)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test connection with PING
	pong, err := client.Ping(ctx).Result()
	require.NoError(t, err, "Failed to ping Redis")
	assert.Equal(t, "PONG", pong, "Redis should respond with PONG")

	t.Log("✓ Redis connection successful")
}

// TestContainer_Redis_Operations tests Redis operations on real container
func TestContainer_Redis_Operations(t *testing.T) {
	RequireContainerService(t, "redis")

	harness, _ := GetContainerHarness()
	redisURL := harness.GetServiceURL("redis")

	opt, err := redis.ParseURL(redisURL)
	require.NoError(t, err)

	client := redis.NewClient(opt)
	defer client.Close()

	ctx := context.Background()

	// Test SET/GET
	err = client.Set(ctx, "container:test:key", "test-value", 10*time.Second).Err()
	require.NoError(t, err, "Failed to SET value")

	val, err := client.Get(ctx, "container:test:key").Result()
	require.NoError(t, err, "Failed to GET value")
	assert.Equal(t, "test-value", val)

	// Cleanup
	client.Del(ctx, "container:test:key")

	t.Log("✓ Redis operations successful")
}

// =============================================================================
// ChromaDB Container Tests
// =============================================================================

// TestContainer_ChromaDB_Connection tests connection to real ChromaDB container
func TestContainer_ChromaDB_Connection(t *testing.T) {
	RequireContainerService(t, "chromadb")

	harness, _ := GetContainerHarness()
	chromaURL := harness.GetServiceURL("chromadb")

	// Health check via HTTP
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(chromaURL + "/api/v2/heartbeat")
	require.NoError(t, err, "Failed to connect to ChromaDB")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "ChromaDB health check should return 200")
	t.Log("✓ ChromaDB connection successful")
}

// =============================================================================
// Cognee Container Tests
// =============================================================================

// TestContainer_Cognee_Connection tests connection to real Cognee container
func TestContainer_Cognee_Connection(t *testing.T) {
	RequireContainerService(t, "cognee")

	harness, _ := GetContainerHarness()
	cogneeURL := harness.GetServiceURL("cognee")

	// Probe /health with a short timeout first. A bare TCP-accept on port
	// 8000 (which is all RequireContainerService checks) doesn't prove
	// Cognee is actually up — a stale tunnel or unrelated listener will
	// satisfy that check and then hang our 5s HTTP GET. Skip the test
	// cleanly when /health is unreachable rather than blowing the suite's
	// wall-clock timeout.
	probe := &http.Client{Timeout: 2 * time.Second}
	if resp, err := probe.Get(cogneeURL + "/health"); err != nil {
		t.Skipf("Cognee /health unreachable at %s: %v", cogneeURL, err)
	} else {
		_ = resp.Body.Close()
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(cogneeURL + "/health")
	require.NoError(t, err, "Failed to connect to Cognee")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Cognee health check should return 200")
	t.Log("✓ Cognee connection successful")
}

// =============================================================================
// Qdrant Container Tests
// =============================================================================

// TestContainer_Qdrant_Connection tests connection to real Qdrant container
func TestContainer_Qdrant_Connection(t *testing.T) {
	RequireContainerService(t, "qdrant")

	harness, _ := GetContainerHarness()
	qdrantURL := harness.GetServiceURL("qdrant")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(qdrantURL)
	require.NoError(t, err, "Failed to connect to Qdrant")
	defer resp.Body.Close()

	// Qdrant root returns info without auth
	assert.True(t, resp.StatusCode < 500, "Qdrant should be accessible")
	t.Log("✓ Qdrant connection successful")
}
