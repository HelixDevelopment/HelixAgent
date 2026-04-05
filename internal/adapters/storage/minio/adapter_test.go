package minio_test

import (
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/storage/minio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Config struct tests
// ============================================================================

func TestConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := &adapter.Config{
		Endpoint:          "localhost:9000",
		AccessKey:         "minioadmin",
		SecretKey:         "minioadmin",
		UseSSL:            false,
		Region:            "us-east-1",
		ConnectTimeout:    5 * time.Second,
		RequestTimeout:    30 * time.Second,
		MaxRetries:        3,
		PartSize:          64 * 1024 * 1024,
		ConcurrentUploads: 4,
	}

	assert.Equal(t, "localhost:9000", cfg.Endpoint)
	assert.Equal(t, "minioadmin", cfg.AccessKey)
	assert.Equal(t, "minioadmin", cfg.SecretKey)
	assert.False(t, cfg.UseSSL)
	assert.Equal(t, "us-east-1", cfg.Region)
	assert.Equal(t, 5*time.Second, cfg.ConnectTimeout)
	assert.Equal(t, 30*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, int64(64*1024*1024), cfg.PartSize)
	assert.Equal(t, 4, cfg.ConcurrentUploads)
}

// ============================================================================
// BucketConfig tests
// ============================================================================

func TestDefaultBucketConfig(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultBucketConfig("my-bucket")
	require.NotNil(t, cfg)
	assert.Equal(t, "my-bucket", cfg.Name)
	assert.False(t, cfg.Versioning)
	assert.False(t, cfg.ObjectLocking)
	assert.Equal(t, 0, cfg.RetentionDays)
}

func TestBucketConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := &adapter.BucketConfig{
		Name:          "test-bucket",
		Region:        "us-east-1",
		Versioning:    true,
		ObjectLocking: false,
		RetentionDays: 30,
	}
	assert.Equal(t, "test-bucket", cfg.Name)
	assert.Equal(t, "us-east-1", cfg.Region)
	assert.True(t, cfg.Versioning)
	assert.Equal(t, 30, cfg.RetentionDays)
}

// ============================================================================
// ObjectInfo tests
// ============================================================================

func TestObjectInfo_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	info := &adapter.ObjectInfo{
		Key:          "test/key.txt",
		Size:         1024,
		ETag:         "abc123",
		ContentType:  "text/plain",
		LastModified: now,
		Metadata:     map[string]string{"author": "test"},
	}

	assert.Equal(t, "test/key.txt", info.Key)
	assert.Equal(t, int64(1024), info.Size)
	assert.Equal(t, "abc123", info.ETag)
	assert.Equal(t, "text/plain", info.ContentType)
	assert.Equal(t, now, info.LastModified)
	assert.Equal(t, "test", info.Metadata["author"])
}

// ============================================================================
// BucketInfo tests
// ============================================================================

func TestBucketInfo_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	info := adapter.BucketInfo{
		Name:         "my-bucket",
		CreationDate: now,
	}
	assert.Equal(t, "my-bucket", info.Name)
	assert.Equal(t, now, info.CreationDate)
}

// ============================================================================
// LifecycleRule tests
// ============================================================================

func TestDefaultLifecycleRule(t *testing.T) {
	t.Parallel()
	rule := adapter.DefaultLifecycleRule("cleanup-rule", 90)
	require.NotNil(t, rule)
	assert.Equal(t, "cleanup-rule", rule.ID)
	assert.True(t, rule.Enabled)
	assert.Equal(t, 90, rule.ExpirationDays)
}

func TestLifecycleRule_Fields(t *testing.T) {
	t.Parallel()
	rule := &adapter.LifecycleRule{
		ID:             "my-rule",
		Prefix:         "logs/",
		Enabled:        true,
		ExpirationDays: 30,
		NoncurrentDays: 7,
	}
	assert.Equal(t, "my-rule", rule.ID)
	assert.Equal(t, "logs/", rule.Prefix)
	assert.True(t, rule.Enabled)
	assert.Equal(t, 30, rule.ExpirationDays)
	assert.Equal(t, 7, rule.NoncurrentDays)
}

// ============================================================================
// PutOption tests
// ============================================================================

func TestWithContentType(t *testing.T) {
	t.Parallel()
	// WithContentType should be a valid PutOption (compile-time test)
	opt := adapter.WithContentType("application/json")
	assert.NotNil(t, opt)
}

func TestWithMetadata(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"key": "value"}
	opt := adapter.WithMetadata(meta)
	assert.NotNil(t, opt)
}

// ============================================================================
// NewClient - fails without MinIO server
// ============================================================================

func TestNewClient_InvalidEndpoint(t *testing.T) {
	t.Parallel()
	cfg := &adapter.Config{
		Endpoint:  "invalid-host:9999",
		AccessKey: "access",
		SecretKey: "secret",
		UseSSL:    false,
	}

	// NewClient may succeed (no actual connection until Connect/HealthCheck)
	// or may fail with invalid config
	client, err := adapter.NewClient(cfg, nil)
	if err != nil {
		// Expected - invalid endpoint
		assert.Nil(t, client)
	} else {
		// Client created - but not connected
		require.NotNil(t, client)
		assert.False(t, client.IsConnected())
		client.Close()
	}
}

func TestNewClient_NilLogger(t *testing.T) {
	t.Parallel()
	cfg := &adapter.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "access",
		SecretKey: "secret",
	}
	// nil logger should be handled gracefully (uses default logrus)
	client, err := adapter.NewClient(cfg, nil)
	if err == nil {
		require.NotNil(t, client)
		assert.False(t, client.IsConnected())
		client.Close()
	}
}
