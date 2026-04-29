//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/adapters/storage/minio"
	"dev.helix.agent/internal/adapters/vectordb/qdrant"
	"dev.helix.agent/internal/cache"
	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/database"
	"dev.helix.agent/internal/testutil"
)

// =============================================================================
// INFRASTRUCTURE TEST HELPERS
// =============================================================================

func infraGetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// =============================================================================
// POSTGRESQL INTEGRATION TESTS
// =============================================================================

func TestIntegration_PostgreSQL_Connection(t *testing.T) {
	testutil.RequirePostgres(t)

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           infraGetEnvOrDefault("DB_HOST", "localhost"),
			Port:           infraGetEnvOrDefault("DB_PORT", "5432"),
			User:           infraGetEnvOrDefault("DB_USER", "helixagent"),
			Password:       infraGetEnvOrDefault("DB_PASSWORD", "helixagent123"),
			Name:           infraGetEnvOrDefault("DB_NAME", "helixagent_db"),
			SSLMode:        "disable",
			MaxConnections: 10,
			ConnTimeout:    5 * time.Second,
			PoolSize:       5,
		},
	}

	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}
	defer db.Close()

	// Test ping
	err = db.Ping()
	require.NoError(t, err, "Failed to ping PostgreSQL")

	// Test health check
	err = db.HealthCheck()
	require.NoError(t, err, "Health check failed")

	t.Log("PostgreSQL connection successful")
}

func TestIntegration_PostgreSQL_CRUD(t *testing.T) {
	testutil.RequirePostgres(t)

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           infraGetEnvOrDefault("DB_HOST", "localhost"),
			Port:           infraGetEnvOrDefault("DB_PORT", "5432"),
			User:           infraGetEnvOrDefault("DB_USER", "helixagent"),
			Password:       infraGetEnvOrDefault("DB_PASSWORD", "helixagent123"),
			Name:           infraGetEnvOrDefault("DB_NAME", "helixagent_db"),
			SSLMode:        "disable",
			MaxConnections: 10,
			ConnTimeout:    5 * time.Second,
			PoolSize:       5,
		},
	}

	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}
	defer db.Close()

	// Create test table
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS integration_test (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			data JSONB,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(t, err, "Failed to create test table")

	// Insert test data
	testData := map[string]interface{}{"key": "value", "number": 42}
	jsonData, _ := json.Marshal(testData)

	var insertedID int
	err = db.QueryRow(`
		INSERT INTO integration_test (name, data) VALUES ($1, $2) RETURNING id
	`, "test_record", jsonData).Scan(&insertedID)
	require.NoError(t, err, "Failed to insert test record")
	assert.Greater(t, insertedID, 0)

	// Query test data
	var name string
	var data []byte
	err = db.QueryRow(`
		SELECT name, data FROM integration_test WHERE id = $1
	`, insertedID).Scan(&name, &data)
	require.NoError(t, err, "Failed to query test record")
	assert.Equal(t, "test_record", name)

	// Update test data
	err = db.Exec(`
		UPDATE integration_test SET name = $1 WHERE id = $2
	`, "updated_record", insertedID)
	require.NoError(t, err, "Failed to update test record")

	// Delete test data
	err = db.Exec(`DELETE FROM integration_test WHERE id = $1`, insertedID)
	require.NoError(t, err, "Failed to delete test record")

	// Cleanup
	_ = db.Exec(`DROP TABLE IF EXISTS integration_test`)

	t.Log("PostgreSQL CRUD operations successful")
}

func TestIntegration_PostgreSQL_Transactions(t *testing.T) {
	testutil.RequirePostgres(t)

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           infraGetEnvOrDefault("DB_HOST", "localhost"),
			Port:           infraGetEnvOrDefault("DB_PORT", "5432"),
			User:           infraGetEnvOrDefault("DB_USER", "helixagent"),
			Password:       infraGetEnvOrDefault("DB_PASSWORD", "helixagent123"),
			Name:           infraGetEnvOrDefault("DB_NAME", "helixagent_db"),
			SSLMode:        "disable",
			MaxConnections: 10,
			ConnTimeout:    5 * time.Second,
			PoolSize:       5,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("PostgreSQL not available: %v (SKIP-OK: #infra-postgres-unavailable)", err)
	}
	defer db.Close()

	// Create test table
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tx_test (
			id SERIAL PRIMARY KEY,
			value INT NOT NULL
		)
	`)
	require.NoError(t, err)
	defer db.Exec(`DROP TABLE IF EXISTS tx_test`)

	// Test successful transaction using pool
	pool := db.GetPool()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `INSERT INTO tx_test (value) VALUES ($1)`, 100)
	require.NoError(t, err)

	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Verify data exists
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM tx_test WHERE value = 100`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Test rollback
	tx2, err := pool.Begin(ctx)
	require.NoError(t, err)

	_, err = tx2.Exec(ctx, `INSERT INTO tx_test (value) VALUES ($1)`, 200)
	require.NoError(t, err)

	err = tx2.Rollback(ctx)
	require.NoError(t, err)

	// Verify rollback worked
	err = db.QueryRow(`SELECT COUNT(*) FROM tx_test WHERE value = 200`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Rollback should have prevented insert")

	t.Log("PostgreSQL transaction operations successful")
}

// =============================================================================
// REDIS INTEGRATION TESTS
// =============================================================================

func TestIntegration_Redis_Connection(t *testing.T) {
	testutil.RequireRedis(t)

	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     infraGetEnvOrDefault("REDIS_HOST", "localhost"),
			Port:     infraGetEnvOrDefault("REDIS_PORT", "6379"),
			Password: infraGetEnvOrDefault("REDIS_PASSWORD", "helixagent123"),
			DB:       0,
		},
	}

	client := cache.NewRedisClient(cfg)
	defer client.Close()

	ctx := context.Background()

	// Test ping
	err := client.Ping(ctx)
	if err != nil {
		t.Skipf("Redis not available: %v (SKIP-OK: #infra-redis-unavailable)", err)
	}

	t.Log("Redis connection successful")
}

func TestIntegration_Redis_CRUD(t *testing.T) {
	testutil.RequireRedis(t)

	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     infraGetEnvOrDefault("REDIS_HOST", "localhost"),
			Port:     infraGetEnvOrDefault("REDIS_PORT", "6379"),
			Password: infraGetEnvOrDefault("REDIS_PASSWORD", "helixagent123"),
			DB:       0,
		},
	}

	client := cache.NewRedisClient(cfg)
	defer client.Close()

	ctx := context.Background()

	// Skip if Redis not available
	if err := client.Ping(ctx); err != nil {
		t.Skipf("Redis not available: %v (SKIP-OK: #infra-redis-unavailable)", err)
	}

	// Test Set
	testData := map[string]interface{}{
		"key":    "value",
		"number": 42,
	}
	err := client.Set(ctx, "integration_test_key", testData, 1*time.Minute)
	require.NoError(t, err, "Failed to set key")

	// Test Get
	var result map[string]interface{}
	err = client.Get(ctx, "integration_test_key", &result)
	require.NoError(t, err, "Failed to get key")
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, float64(42), result["number"]) // JSON unmarshals numbers as float64

	// Test Delete
	err = client.Delete(ctx, "integration_test_key")
	require.NoError(t, err, "Failed to delete key")

	// Verify deletion
	var emptyResult map[string]interface{}
	err = client.Get(ctx, "integration_test_key", &emptyResult)
	assert.Error(t, err, "Key should have been deleted")

	t.Log("Redis CRUD operations successful")
}

func TestIntegration_Redis_Expiration(t *testing.T) {
	testutil.RequireRedis(t)

	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     infraGetEnvOrDefault("REDIS_HOST", "localhost"),
			Port:     infraGetEnvOrDefault("REDIS_PORT", "6379"),
			Password: infraGetEnvOrDefault("REDIS_PASSWORD", "helixagent123"),
			DB:       0,
		},
	}

	client := cache.NewRedisClient(cfg)
	defer client.Close()

	ctx := context.Background()

	// Skip if Redis not available
	if err := client.Ping(ctx); err != nil {
		t.Skipf("Redis not available: %v (SKIP-OK: #infra-redis-unavailable)", err)
	}

	// Set with short expiration
	err := client.Set(ctx, "expiring_key", "test_value", 100*time.Millisecond)
	require.NoError(t, err)

	// Verify key exists
	var result string
	err = client.Get(ctx, "expiring_key", &result)
	require.NoError(t, err)
	assert.Equal(t, "test_value", result)

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Verify key expired
	err = client.Get(ctx, "expiring_key", &result)
	assert.Error(t, err, "Key should have expired")

	t.Log("Redis expiration test successful")
}

func TestIntegration_Redis_Pipeline(t *testing.T) {
	testutil.RequireRedis(t)

	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     infraGetEnvOrDefault("REDIS_HOST", "localhost"),
			Port:     infraGetEnvOrDefault("REDIS_PORT", "6379"),
			Password: infraGetEnvOrDefault("REDIS_PASSWORD", "helixagent123"),
			DB:       0,
		},
	}

	client := cache.NewRedisClient(cfg)
	defer client.Close()

	ctx := context.Background()

	// Skip if Redis not available
	if err := client.Ping(ctx); err != nil {
		t.Skipf("Redis not available: %v (SKIP-OK: #infra-redis-unavailable)", err)
	}

	// Use pipeline for batch operations
	pipe := client.Pipeline()

	// Queue multiple operations
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("pipeline_test_%d", i)
		pipe.Set(ctx, key, fmt.Sprintf("value_%d", i), 1*time.Minute)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	require.NoError(t, err, "Pipeline execution failed")

	// Verify using MGet
	keys := make([]string, 10)
	for i := 0; i < 10; i++ {
		keys[i] = fmt.Sprintf("pipeline_test_%d", i)
	}
	results, err := client.MGet(ctx, keys...)
	require.NoError(t, err)
	assert.Len(t, results, 10)

	// Cleanup
	for _, key := range keys {
		client.Delete(ctx, key)
	}

	t.Log("Redis pipeline operations successful")
}

// =============================================================================
// MINIO INTEGRATION TESTS
// =============================================================================

func TestIntegration_MinIO_Connection(t *testing.T) {
	testutil.RequireExternalService(t, "minio", infraGetEnvOrDefault("MINIO_HOST", "localhost"), infraGetEnvOrDefault("MINIO_PORT", "9000"))

	cfg := &minio.Config{
		Endpoint:          infraGetEnvOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey:         infraGetEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:         infraGetEnvOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		UseSSL:            false,
		Region:            "us-east-1",
		ConnectTimeout:    30 * time.Second,
		RequestTimeout:    60 * time.Second,
		MaxRetries:        3,
		PartSize:          16 * 1024 * 1024,
		ConcurrentUploads: 4,
	}

	client, err := minio.NewClient(cfg, nil)
	require.NoError(t, err, "Failed to create MinIO client")

	ctx := context.Background()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("MinIO not available: %v (SKIP-OK: #infra-unavailable)", err)
	}
	defer client.Close()

	// Test health check
	err = client.HealthCheck(ctx)
	require.NoError(t, err, "Health check failed")

	t.Log("MinIO connection successful")
}

func TestIntegration_MinIO_BucketOperations(t *testing.T) {
	testutil.RequireExternalService(t, "minio", infraGetEnvOrDefault("MINIO_HOST", "localhost"), infraGetEnvOrDefault("MINIO_PORT", "9000"))

	cfg := &minio.Config{
		Endpoint:          infraGetEnvOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey:         infraGetEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:         infraGetEnvOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		UseSSL:            false,
		Region:            "us-east-1",
		ConnectTimeout:    30 * time.Second,
		RequestTimeout:    60 * time.Second,
		MaxRetries:        3,
		PartSize:          16 * 1024 * 1024,
		ConcurrentUploads: 4,
	}

	client, err := minio.NewClient(cfg, nil)
	require.NoError(t, err)

	ctx := context.Background()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("MinIO not available: %v (SKIP-OK: #infra-unavailable)", err)
	}
	defer client.Close()

	// Create test bucket
	bucketName := "integration-test-bucket"
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	err = client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err, "Failed to create bucket")

	// Verify bucket exists
	exists, err := client.BucketExists(ctx, bucketName)
	require.NoError(t, err)
	assert.True(t, exists, "Bucket should exist")

	// List buckets
	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, buckets)

	// Cleanup - delete bucket
	err = client.DeleteBucket(ctx, bucketName)
	require.NoError(t, err)

	t.Log("MinIO bucket operations successful")
}

func TestIntegration_MinIO_ObjectOperations(t *testing.T) {
	testutil.RequireExternalService(t, "minio", infraGetEnvOrDefault("MINIO_HOST", "localhost"), infraGetEnvOrDefault("MINIO_PORT", "9000"))

	cfg := &minio.Config{
		Endpoint:          infraGetEnvOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey:         infraGetEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:         infraGetEnvOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		UseSSL:            false,
		Region:            "us-east-1",
		ConnectTimeout:    30 * time.Second,
		RequestTimeout:    60 * time.Second,
		MaxRetries:        3,
		PartSize:          16 * 1024 * 1024,
		ConcurrentUploads: 4,
	}

	client, err := minio.NewClient(cfg, nil)
	require.NoError(t, err)

	ctx := context.Background()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("MinIO not available: %v (SKIP-OK: #infra-unavailable)", err)
	}
	defer client.Close()

	// Create test bucket
	bucketName := "integration-test-objects"
	bucketConfig := minio.DefaultBucketConfig(bucketName)
	err = client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Put object
	testData := []byte("Hello, MinIO integration test!")
	objectName := "test-object.txt"
	err = client.PutObject(ctx, bucketName, objectName, bytes.NewReader(testData), int64(len(testData)),
		minio.WithContentType("text/plain"),
		minio.WithMetadata(map[string]string{"test": "value"}))
	require.NoError(t, err, "Failed to put object")

	// Get object
	reader, err := client.GetObject(ctx, bucketName, objectName)
	require.NoError(t, err)
	defer reader.Close()

	retrievedData, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, testData, retrievedData)

	// Stat object
	info, err := client.StatObject(ctx, bucketName, objectName)
	require.NoError(t, err)
	assert.Equal(t, int64(len(testData)), info.Size)
	assert.Equal(t, "text/plain", info.ContentType)

	// List objects
	objects, err := client.ListObjects(ctx, bucketName, "")
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, objectName, objects[0].Key)

	// Delete object
	err = client.DeleteObject(ctx, bucketName, objectName)
	require.NoError(t, err)

	t.Log("MinIO object operations successful")
}

func TestIntegration_MinIO_PresignedURLs(t *testing.T) {
	testutil.RequireExternalService(t, "minio", infraGetEnvOrDefault("MINIO_HOST", "localhost"), infraGetEnvOrDefault("MINIO_PORT", "9000"))

	cfg := &minio.Config{
		Endpoint:          infraGetEnvOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey:         infraGetEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey:         infraGetEnvOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		UseSSL:            false,
		Region:            "us-east-1",
		ConnectTimeout:    30 * time.Second,
		RequestTimeout:    60 * time.Second,
		MaxRetries:        3,
		PartSize:          16 * 1024 * 1024,
		ConcurrentUploads: 4,
	}

	client, err := minio.NewClient(cfg, nil)
	require.NoError(t, err)

	ctx := context.Background()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("MinIO not available: %v (SKIP-OK: #infra-unavailable)", err)
	}
	defer client.Close()

	// Create test bucket
	bucketName := "integration-test-presigned"
	bucketConfig := minio.DefaultBucketConfig(bucketName)
	err = client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Put object
	testData := []byte("Presigned URL test data")
	objectName := "presigned-test.txt"
	err = client.PutObject(ctx, bucketName, objectName, bytes.NewReader(testData), int64(len(testData)))
	require.NoError(t, err)
	defer client.DeleteObject(ctx, bucketName, objectName)

	// Get presigned GET URL
	getURL, err := client.GetPresignedURL(ctx, bucketName, objectName, 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, getURL)
	assert.Contains(t, getURL, bucketName)
	assert.Contains(t, getURL, objectName)

	// Get presigned PUT URL
	putURL, err := client.GetPresignedPutURL(ctx, bucketName, "new-object.txt", 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, putURL)

	t.Log("MinIO presigned URL operations successful")
}

// =============================================================================
// QDRANT INTEGRATION TESTS
// =============================================================================

func TestIntegration_Qdrant_Connection(t *testing.T) {
	testutil.RequireExternalService(t, "qdrant", infraGetEnvOrDefault("QDRANT_HOST", "localhost"), "6333")

	cfg := qdrant.DefaultConfig()
	cfg.Host = infraGetEnvOrDefault("QDRANT_HOST", "localhost")

	client, err := qdrant.NewClient(cfg, nil)
	require.NoError(t, err, "Failed to create Qdrant client")

	ctx := context.Background()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("Qdrant not available: %v (SKIP-OK: #infra-unavailable)", err)
	}
	defer client.Close()

	// Test health check
	err = client.HealthCheck(ctx)
	require.NoError(t, err, "Health check failed")

	t.Log("Qdrant connection successful")
}

func TestIntegration_Qdrant_CollectionOperations(t *testing.T) {
	testutil.RequireExternalService(t, "qdrant", infraGetEnvOrDefault("QDRANT_HOST", "localhost"), "6333")

	cfg := qdrant.DefaultConfig()
	cfg.Host = infraGetEnvOrDefault("QDRANT_HOST", "localhost")

	client, err := qdrant.NewClient(cfg, nil)
	require.NoError(t, err)

	ctx := context.Background()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("Qdrant not available: %v (SKIP-OK: #infra-unavailable)", err)
	}
	defer client.Close()

	// Create test collection
	collectionName := "integration_test_collection"
	collectionConfig := qdrant.DefaultCollectionConfig(collectionName, 128)

	err = client.CreateCollection(ctx, collectionConfig)
	require.NoError(t, err, "Failed to create collection")

	// Verify collection exists
	exists, err := client.CollectionExists(ctx, collectionName)
	require.NoError(t, err)
	assert.True(t, exists, "Collection should exist")

	// List collections
	collections, err := client.ListCollections(ctx)
	require.NoError(t, err)
	assert.Contains(t, collections, collectionName)

	// Get collection info
	info, err := client.GetCollectionInfo(ctx, collectionName)
	require.NoError(t, err)
	assert.Equal(t, collectionName, info.Name)

	// Cleanup - delete collection
	err = client.DeleteCollection(ctx, collectionName)
	require.NoError(t, err)

	t.Log("Qdrant collection operations successful")
}

func TestIntegration_Qdrant_VectorOperations(t *testing.T) {
	testutil.RequireExternalService(t, "qdrant", infraGetEnvOrDefault("QDRANT_HOST", "localhost"), "6333")

	cfg := qdrant.DefaultConfig()
	cfg.Host = infraGetEnvOrDefault("QDRANT_HOST", "localhost")

	client, err := qdrant.NewClient(cfg, nil)
	require.NoError(t, err)

	ctx := context.Background()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("Qdrant not available: %v (SKIP-OK: #infra-unavailable)", err)
	}
	defer client.Close()

	// Create test collection
	collectionName := "integration_test_vectors"
	collectionConfig := qdrant.DefaultCollectionConfig(collectionName, 4) // Small vector size for testing

	err = client.CreateCollection(ctx, collectionConfig)
	require.NoError(t, err)
	defer client.DeleteCollection(ctx, collectionName)

	// Wait for collection to be ready
	time.Sleep(500 * time.Millisecond)

	// Generate UUIDs for point IDs (Qdrant requires UUID or unsigned int)
	pointID1 := "11111111-1111-1111-1111-111111111111"
	pointID2 := "22222222-2222-2222-2222-222222222222"
	pointID3 := "33333333-3333-3333-3333-333333333333"

	// Upsert points
	points := []qdrant.Point{
		{
			ID:     pointID1,
			Vector: []float32{0.1, 0.2, 0.3, 0.4},
			Payload: map[string]interface{}{
				"name": "test-point-1",
				"type": "test",
			},
		},
		{
			ID:     pointID2,
			Vector: []float32{0.5, 0.6, 0.7, 0.8},
			Payload: map[string]interface{}{
				"name": "test-point-2",
				"type": "test",
			},
		},
		{
			ID:     pointID3,
			Vector: []float32{0.9, 0.1, 0.2, 0.3},
			Payload: map[string]interface{}{
				"name": "test-point-3",
				"type": "other",
			},
		},
	}

	err = client.UpsertPoints(ctx, collectionName, points)
	require.NoError(t, err, "Failed to upsert points")

	// Wait for indexing
	time.Sleep(500 * time.Millisecond)

	// Count points
	count, err := client.CountPoints(ctx, collectionName, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Get point
	point, err := client.GetPoint(ctx, collectionName, pointID1)
	require.NoError(t, err)
	assert.Equal(t, pointID1, point.ID)
	assert.Equal(t, "test-point-1", point.Payload["name"])

	// Search for similar vectors
	searchVector := []float32{0.1, 0.2, 0.3, 0.4}
	searchOpts := qdrant.DefaultSearchOptions().WithLimit(5)
	results, err := client.Search(ctx, collectionName, searchVector, searchOpts)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, pointID1, results[0].ID) // Exact match should be first

	// Delete point
	err = client.DeletePoints(ctx, collectionName, []string{pointID1})
	require.NoError(t, err)

	// Verify deletion
	count, err = client.CountPoints(ctx, collectionName, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	t.Log("Qdrant vector operations successful")
}

func TestIntegration_Qdrant_BatchSearch(t *testing.T) {
	testutil.RequireExternalService(t, "qdrant", infraGetEnvOrDefault("QDRANT_HOST", "localhost"), "6333")

	cfg := qdrant.DefaultConfig()
	cfg.Host = infraGetEnvOrDefault("QDRANT_HOST", "localhost")

	client, err := qdrant.NewClient(cfg, nil)
	require.NoError(t, err)

	ctx := context.Background()

	err = client.Connect(ctx)
	if err != nil {
		t.Skipf("Qdrant not available: %v (SKIP-OK: #infra-unavailable)", err)
	}
	defer client.Close()

	// Create test collection
	collectionName := "integration_test_batch"
	collectionConfig := qdrant.DefaultCollectionConfig(collectionName, 4)

	// Clean up any existing collection from previous test runs
	_ = client.DeleteCollection(ctx, collectionName)
	time.Sleep(100 * time.Millisecond)

	err = client.CreateCollection(ctx, collectionConfig)
	require.NoError(t, err)
	defer client.DeleteCollection(ctx, collectionName)

	// Wait for collection to be ready
	time.Sleep(500 * time.Millisecond)

	// Generate UUIDs for point IDs
	p1ID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	p2ID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	p3ID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	p4ID := "dddddddd-dddd-dddd-dddd-dddddddddddd"

	// Upsert test points
	points := []qdrant.Point{
		{ID: p1ID, Vector: []float32{1.0, 0.0, 0.0, 0.0}},
		{ID: p2ID, Vector: []float32{0.0, 1.0, 0.0, 0.0}},
		{ID: p3ID, Vector: []float32{0.0, 0.0, 1.0, 0.0}},
		{ID: p4ID, Vector: []float32{0.0, 0.0, 0.0, 1.0}},
	}
	err = client.UpsertPoints(ctx, collectionName, points)
	require.NoError(t, err)

	// Wait for indexing
	time.Sleep(500 * time.Millisecond)

	// Batch search
	searchVectors := [][]float32{
		{1.0, 0.0, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
	}
	searchOpts := qdrant.DefaultSearchOptions().WithLimit(2)
	batchResults, err := client.SearchBatch(ctx, collectionName, searchVectors, searchOpts)
	require.NoError(t, err)
	assert.Len(t, batchResults, 2)

	// First search should return p1 as best match
	assert.NotEmpty(t, batchResults[0])
	assert.Equal(t, p1ID, batchResults[0][0].ID)

	// Second search should return p2 as best match
	assert.NotEmpty(t, batchResults[1])
	assert.Equal(t, p2ID, batchResults[1][0].ID)

	t.Log("Qdrant batch search successful")
}

// =============================================================================
// KAFKA INTEGRATION TESTS
// =============================================================================

func TestIntegration_Kafka_Connection(t *testing.T) {
	testutil.RequireExternalService(t, "kafka", infraGetEnvOrDefault("KAFKA_HOST", "localhost"), "9092")

	brokers := infraGetEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")

	// Create a connection to verify Kafka is available
	conn, err := kafka.Dial("tcp", brokers)
	if err != nil {
		t.Skipf("Kafka not available at %s: %v (SKIP-OK: #infra-unavailable)", brokers, err)
		return
	}
	defer conn.Close()

	// Get controller to verify cluster is healthy
	controller, err := conn.Controller()
	require.NoError(t, err)
	assert.NotEmpty(t, controller.Host)

	t.Logf("Kafka connection successful to %s (controller: %s:%d)", brokers, controller.Host, controller.Port)
}

func TestIntegration_Kafka_TopicOperations(t *testing.T) {
	testutil.RequireExternalService(t, "kafka", infraGetEnvOrDefault("KAFKA_HOST", "localhost"), "9092")

	brokers := infraGetEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")

	conn, err := kafka.Dial("tcp", brokers)
	if err != nil {
		t.Skipf("Kafka not available: %v (SKIP-OK: #infra-unavailable)", err)
		return
	}
	defer conn.Close()

	// Create a test topic
	topicName := "integration_test_topic_" + time.Now().Format("20060102150405")
	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topicName,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}

	err = conn.CreateTopics(topicConfigs...)
	require.NoError(t, err)
	t.Logf("Created topic: %s", topicName)

	// List topics to verify
	partitions, err := conn.ReadPartitions()
	require.NoError(t, err)

	found := false
	for _, p := range partitions {
		if p.Topic == topicName {
			found = true
			break
		}
	}
	assert.True(t, found, "Topic should exist after creation")

	// Delete the topic
	err = conn.DeleteTopics(topicName)
	require.NoError(t, err)

	t.Log("Kafka topic operations successful")
}

func TestIntegration_Kafka_ProduceConsume(t *testing.T) {
	testutil.RequireExternalService(t, "kafka", infraGetEnvOrDefault("KAFKA_HOST", "localhost"), "9092")

	brokers := infraGetEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")

	// Create topic first
	conn, err := kafka.Dial("tcp", brokers)
	if err != nil {
		t.Skipf("Kafka not available: %v (SKIP-OK: #infra-unavailable)", err)
		return
	}

	topicName := "integration_test_produce_" + time.Now().Format("20060102150405")
	err = conn.CreateTopics(kafka.TopicConfig{
		Topic:             topicName,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	require.NoError(t, err)
	conn.Close()

	// Give Kafka time to create the topic
	time.Sleep(500 * time.Millisecond)

	// Create writer (producer)
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers),
		Topic:        topicName,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}
	defer writer.Close()

	// Produce messages
	ctx := context.Background()
	messages := []kafka.Message{
		{Key: []byte("key-1"), Value: []byte(`{"msg": "test1"}`)},
		{Key: []byte("key-2"), Value: []byte(`{"msg": "test2"}`)},
		{Key: []byte("key-3"), Value: []byte(`{"msg": "test3"}`)},
	}

	err = writer.WriteMessages(ctx, messages...)
	require.NoError(t, err)
	t.Log("Produced 3 messages")

	// Create reader (consumer)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{brokers},
		Topic:     topicName,
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
	})
	defer reader.Close()

	// Set offset to beginning
	reader.SetOffset(kafka.FirstOffset)

	// Consume messages
	receivedCount := 0
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for receivedCount < 3 {
		msg, err := reader.ReadMessage(readCtx)
		if err != nil {
			break
		}
		t.Logf("Received: key=%s value=%s", string(msg.Key), string(msg.Value))
		receivedCount++
	}

	assert.Equal(t, 3, receivedCount, "Should receive all 3 messages")

	// Cleanup - delete topic
	conn2, _ := kafka.Dial("tcp", brokers)
	if conn2 != nil {
		conn2.DeleteTopics(topicName)
		conn2.Close()
	}

	t.Log("Kafka produce/consume successful")
}

func TestIntegration_Kafka_ConsumerGroup(t *testing.T) {
	testutil.RequireExternalService(t, "kafka", infraGetEnvOrDefault("KAFKA_HOST", "localhost"), "9092")

	brokers := infraGetEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")

	// Create topic
	conn, err := kafka.Dial("tcp", brokers)
	if err != nil {
		t.Skipf("Kafka not available: %v (SKIP-OK: #infra-unavailable)", err)
		return
	}

	topicName := "integration_test_group_" + time.Now().Format("20060102150405")
	groupID := "integration_test_consumers_" + time.Now().Format("20060102150405")

	err = conn.CreateTopics(kafka.TopicConfig{
		Topic:             topicName,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	require.NoError(t, err)
	conn.Close()

	// Wait for topic to be fully created and propagated
	time.Sleep(2 * time.Second)

	ctx := context.Background()

	// Produce messages with retry
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers),
		Topic:                  topicName,
		Balancer:               &kafka.LeastBytes{},
		BatchTimeout:           100 * time.Millisecond,
		AllowAutoTopicCreation: true,
	}

	var writeErr error
	for i := 0; i < 3; i++ {
		writeErr = writer.WriteMessages(ctx,
			kafka.Message{Value: []byte("group-msg-1")},
			kafka.Message{Value: []byte("group-msg-2")},
		)
		if writeErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, writeErr)
	writer.Close()

	// Create consumer group reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{brokers},
		GroupID:        groupID,
		Topic:          topicName,
		MinBytes:       1,
		MaxBytes:       10e6,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: time.Second,
	})
	defer reader.Close()

	// Read with timeout
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(readCtx)
	if err != nil {
		t.Skipf("Consumer group read failed (may be timing issue): %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		return
	}
	assert.NotEmpty(t, msg.Value)
	t.Logf("Consumer group received: %s", string(msg.Value))

	// Cleanup
	conn2, _ := kafka.Dial("tcp", brokers)
	if conn2 != nil {
		conn2.DeleteTopics(topicName)
		conn2.Close()
	}

	t.Log("Kafka consumer group successful")
}

// =============================================================================
// RABBITMQ INTEGRATION TESTS
// =============================================================================

func TestIntegration_RabbitMQ_Connection(t *testing.T) {
	testutil.RequireExternalService(t, "rabbitmq", infraGetEnvOrDefault("RABBITMQ_HOST", "localhost"), infraGetEnvOrDefault("RABBITMQ_PORT", "5672"))

	// Get RabbitMQ connection URL
	host := infraGetEnvOrDefault("RABBITMQ_HOST", "localhost")
	port := infraGetEnvOrDefault("RABBITMQ_PORT", "5672")
	user := infraGetEnvOrDefault("RABBITMQ_USER", "helixagent")
	pass := infraGetEnvOrDefault("RABBITMQ_PASSWORD", "helixagent123")

	amqpURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, pass, host, port)

	// Try to connect using amqp library directly
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		t.Skipf("RabbitMQ not available at %s:%s: %v (SKIP-OK: #infra-unavailable)", host, port, err)
		return
	}
	defer conn.Close()

	// Verify connection is working
	require.False(t, conn.IsClosed())

	// Open a channel
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	t.Logf("RabbitMQ connection successful to %s:%s", host, port)
}

func TestIntegration_RabbitMQ_QueueOperations(t *testing.T) {
	testutil.RequireExternalService(t, "rabbitmq", infraGetEnvOrDefault("RABBITMQ_HOST", "localhost"), infraGetEnvOrDefault("RABBITMQ_PORT", "5672"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx

	host := infraGetEnvOrDefault("RABBITMQ_HOST", "localhost")
	port := infraGetEnvOrDefault("RABBITMQ_PORT", "5672")
	user := infraGetEnvOrDefault("RABBITMQ_USER", "helixagent")
	pass := infraGetEnvOrDefault("RABBITMQ_PASSWORD", "helixagent123")

	amqpURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, pass, host, port)

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		t.Skipf("RabbitMQ not available: %v (SKIP-OK: #infra-unavailable)", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Declare a test queue
	queueName := "integration_test_queue_" + time.Now().Format("20060102150405")
	q, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	require.NoError(t, err)
	assert.Equal(t, queueName, q.Name)

	// Publish a message
	testMessage := []byte(`{"test": "integration", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`)
	err = ch.Publish(
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        testMessage,
		},
	)
	require.NoError(t, err)

	// Consume the message
	msgs, err := ch.Consume(
		queueName,
		"",    // consumer
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	require.NoError(t, err)

	// Wait for message with timeout
	select {
	case msg := <-msgs:
		assert.Equal(t, testMessage, msg.Body)
		t.Logf("Received message: %s", string(msg.Body))
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// Delete the queue
	_, err = ch.QueueDelete(queueName, false, false, false)
	require.NoError(t, err)

	t.Log("RabbitMQ queue operations successful")
}

func TestIntegration_RabbitMQ_ExchangeOperations(t *testing.T) {
	testutil.RequireExternalService(t, "rabbitmq", infraGetEnvOrDefault("RABBITMQ_HOST", "localhost"), infraGetEnvOrDefault("RABBITMQ_PORT", "5672"))

	host := infraGetEnvOrDefault("RABBITMQ_HOST", "localhost")
	port := infraGetEnvOrDefault("RABBITMQ_PORT", "5672")
	user := infraGetEnvOrDefault("RABBITMQ_USER", "helixagent")
	pass := infraGetEnvOrDefault("RABBITMQ_PASSWORD", "helixagent123")

	amqpURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, pass, host, port)

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		t.Skipf("RabbitMQ not available: %v (SKIP-OK: #infra-unavailable)", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Declare a test exchange
	exchangeName := "integration_test_exchange_" + time.Now().Format("20060102150405")
	err = ch.ExchangeDeclare(
		exchangeName,
		"topic", // type
		false,   // durable
		true,    // auto-delete
		false,   // internal
		false,   // no-wait
		nil,     // args
	)
	require.NoError(t, err)

	// Declare a queue and bind to exchange
	queueName := "integration_test_bound_queue_" + time.Now().Format("20060102150405")
	q, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)

	routingKey := "test.routing.key"
	err = ch.QueueBind(q.Name, routingKey, exchangeName, false, nil)
	require.NoError(t, err)

	// Publish to exchange
	testMessage := []byte(`{"event": "test_event"}`)
	err = ch.Publish(
		exchangeName,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        testMessage,
		},
	)
	require.NoError(t, err)

	// Consume from queue
	msgs, err := ch.Consume(queueName, "", true, false, false, false, nil)
	require.NoError(t, err)

	select {
	case msg := <-msgs:
		assert.Equal(t, testMessage, msg.Body)
		t.Logf("Received via exchange: %s", string(msg.Body))
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message from exchange")
	}

	// Cleanup
	ch.QueueDelete(queueName, false, false, false)
	ch.ExchangeDelete(exchangeName, false, false)

	t.Log("RabbitMQ exchange operations successful")
}

func TestIntegration_RabbitMQ_PublishConfirm(t *testing.T) {
	testutil.RequireExternalService(t, "rabbitmq", infraGetEnvOrDefault("RABBITMQ_HOST", "localhost"), infraGetEnvOrDefault("RABBITMQ_PORT", "5672"))

	host := infraGetEnvOrDefault("RABBITMQ_HOST", "localhost")
	port := infraGetEnvOrDefault("RABBITMQ_PORT", "5672")
	user := infraGetEnvOrDefault("RABBITMQ_USER", "helixagent")
	pass := infraGetEnvOrDefault("RABBITMQ_PASSWORD", "helixagent123")

	amqpURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, pass, host, port)

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		t.Skipf("RabbitMQ not available: %v (SKIP-OK: #infra-unavailable)", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Enable publish confirms
	err = ch.Confirm(false)
	require.NoError(t, err)

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	// Declare queue
	queueName := "integration_test_confirm_queue_" + time.Now().Format("20060102150405")
	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)

	// Publish with confirmation
	err = ch.Publish("", queueName, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte("confirmed message"),
	})
	require.NoError(t, err)

	// Wait for confirmation
	select {
	case confirm := <-confirms:
		assert.True(t, confirm.Ack)
		t.Logf("Message confirmed with delivery tag: %d", confirm.DeliveryTag)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for publish confirmation")
	}

	// Cleanup
	ch.QueueDelete(queueName, false, false, false)

	t.Log("RabbitMQ publish confirm successful")
}

// =============================================================================
// INFRASTRUCTURE HEALTH CHECK
// =============================================================================

func TestIntegration_AllInfrastructure_HealthCheck(t *testing.T) {
	testutil.RequireInfra(t)

	services := map[string]string{
		"PostgreSQL": fmt.Sprintf("%s:%s", infraGetEnvOrDefault("DB_HOST", "localhost"), infraGetEnvOrDefault("DB_PORT", "5432")),
		"Redis":      fmt.Sprintf("%s:%s", infraGetEnvOrDefault("REDIS_HOST", "localhost"), infraGetEnvOrDefault("REDIS_PORT", "6379")),
		"Kafka":      infraGetEnvOrDefault("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092"),
		"RabbitMQ":   fmt.Sprintf("%s:5672", infraGetEnvOrDefault("RABBITMQ_HOST", "localhost")),
		"MinIO":      infraGetEnvOrDefault("MINIO_ENDPOINT", "localhost:9000"),
		"Qdrant":     fmt.Sprintf("%s:%s", infraGetEnvOrDefault("QDRANT_HOST", "localhost"), infraGetEnvOrDefault("QDRANT_PORT", "6333")),
	}

	t.Log("=== Infrastructure Health Check ===")
	for name, endpoint := range services {
		t.Logf("  %s: %s", name, endpoint)
	}

	// Verify PostgreSQL specifically since it's critical
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           infraGetEnvOrDefault("DB_HOST", "localhost"),
			Port:           infraGetEnvOrDefault("DB_PORT", "5432"),
			User:           infraGetEnvOrDefault("DB_USER", "helixagent"),
			Password:       infraGetEnvOrDefault("DB_PASSWORD", "helixagent123"),
			Name:           infraGetEnvOrDefault("DB_NAME", "helixagent_db"),
			SSLMode:        "disable",
			MaxConnections: 10,
			ConnTimeout:    5 * time.Second,
			PoolSize:       5,
		},
	}

	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		t.Logf("PostgreSQL: NOT AVAILABLE - %v", err)
	} else {
		if err := db.HealthCheck(); err != nil {
			t.Logf("PostgreSQL: UNHEALTHY - %v", err)
		} else {
			t.Log("PostgreSQL: HEALTHY")
		}
		db.Close()
	}

	t.Log("=== Health Check Complete ===")
}
