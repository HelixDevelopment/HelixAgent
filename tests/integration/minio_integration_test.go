package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/adapters/storage/minio"
	"dev.helix.agent/internal/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minioGetEnv helper function to get environment variable or default
func minioGetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// MinIO integration test configuration
var (
	minioEndpoint  = minioGetEnv("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey = minioGetEnv("MINIO_ACCESS_KEY", "minioadmin")
	minioSecretKey = minioGetEnv("MINIO_SECRET_KEY", "minioadmin")
	minioUseSSL    = minioGetEnv("MINIO_USE_SSL", "false") == "true"
)

// skipMinIOIfUnavailable skips the test if MinIO is not available
func skipMinIOIfUnavailable(t *testing.T) *minio.Client {
	t.Helper()

	testutil.RequireExternalService(t, "minio", "localhost", "9000")

	config := &minio.Config{
		Endpoint:          minioEndpoint,
		AccessKey:         minioAccessKey,
		SecretKey:         minioSecretKey,
		UseSSL:            minioUseSSL,
		Region:            "us-east-1",
		ConnectTimeout:    5 * time.Second,
		RequestTimeout:    30 * time.Second,
		PartSize:          5 * 1024 * 1024, // 5MB
		ConcurrentUploads: 4,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	client, err := minio.NewClient(config, logger)
	if err != nil {
		t.Skipf("Skipping MinIO integration test: failed to create client: %v (SKIP-OK: #integration-only)", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Skipf("Skipping MinIO integration test: MinIO not available at %s: %v (SKIP-OK: #integration-only)", minioEndpoint, err)
		return nil
	}

	return client
}

// =====================================================
// Connection Tests
// =====================================================

func TestMinIOIntegration_Connect(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	assert.True(t, client.IsConnected())
}

func TestMinIOIntegration_HealthCheck(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	err := client.HealthCheck(context.Background())
	require.NoError(t, err)
}

func TestMinIOIntegration_Close(t *testing.T) {
	client := skipMinIOIfUnavailable(t)

	err := client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())

	// Close again should be safe
	err = client.Close()
	require.NoError(t, err)
}

// =====================================================
// Bucket Tests
// =====================================================

func TestMinIOIntegration_CreateBucket(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-bucket-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)

	// Verify bucket exists
	exists, err := client.BucketExists(ctx, bucketName)
	require.NoError(t, err)
	assert.True(t, exists)

	// Cleanup
	err = client.DeleteBucket(ctx, bucketName)
	require.NoError(t, err)
}

func TestMinIOIntegration_CreateBucket_AlreadyExists(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-bucket-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)

	// Create again should not error (idempotent)
	err = client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)

	// Cleanup
	err = client.DeleteBucket(ctx, bucketName)
	require.NoError(t, err)
}

func TestMinIOIntegration_CreateBucket_WithVersioning(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-versioned-%d", time.Now().UnixNano())
	bucketConfig := &minio.BucketConfig{
		Name:       bucketName,
		Versioning: true,
	}

	ctx := context.Background()

	// Create bucket with versioning
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)

	// Verify bucket exists
	exists, err := client.BucketExists(ctx, bucketName)
	require.NoError(t, err)
	assert.True(t, exists)

	// Cleanup
	err = client.DeleteBucket(ctx, bucketName)
	require.NoError(t, err)
}

func TestMinIOIntegration_CreateBucket_WithRetention(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-retention-%d", time.Now().UnixNano())
	bucketConfig := &minio.BucketConfig{
		Name:          bucketName,
		RetentionDays: 7,
	}

	ctx := context.Background()

	// Create bucket with retention policy
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)

	// Verify bucket exists
	exists, err := client.BucketExists(ctx, bucketName)
	require.NoError(t, err)
	assert.True(t, exists)

	// Cleanup
	err = client.DeleteBucket(ctx, bucketName)
	require.NoError(t, err)
}

func TestMinIOIntegration_ListBuckets(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-list-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)

	// List buckets
	buckets, err := client.ListBuckets(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, buckets)

	// Find our bucket
	found := false
	for _, b := range buckets {
		if b.Name == bucketName {
			found = true
			assert.False(t, b.CreationDate.IsZero())
			break
		}
	}
	assert.True(t, found, "Created bucket should be in the list")

	// Cleanup
	err = client.DeleteBucket(ctx, bucketName)
	require.NoError(t, err)
}

func TestMinIOIntegration_BucketExists(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	ctx := context.Background()

	// Check non-existent bucket
	exists, err := client.BucketExists(ctx, "nonexistent-bucket-12345")
	require.NoError(t, err)
	assert.False(t, exists)

	// Create and check existing bucket
	bucketName := fmt.Sprintf("test-exists-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	err = client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)

	exists, err = client.BucketExists(ctx, bucketName)
	require.NoError(t, err)
	assert.True(t, exists)

	// Cleanup
	err = client.DeleteBucket(ctx, bucketName)
	require.NoError(t, err)
}

func TestMinIOIntegration_DeleteBucket(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-delete-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)

	// Delete bucket
	err = client.DeleteBucket(ctx, bucketName)
	require.NoError(t, err)

	// Verify bucket no longer exists
	exists, err := client.BucketExists(ctx, bucketName)
	require.NoError(t, err)
	assert.False(t, exists)
}

// =====================================================
// Object Tests
// =====================================================

func TestMinIOIntegration_PutObject(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-put-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "test-object.txt")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Put object
	content := []byte("Hello, MinIO!")
	reader := bytes.NewReader(content)
	err = client.PutObject(ctx, bucketName, "test-object.txt", reader, int64(len(content)))
	require.NoError(t, err)
}

func TestMinIOIntegration_PutObject_WithContentType(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-content-type-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "test.json")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Put object with content type
	content := []byte(`{"message": "hello"}`)
	reader := bytes.NewReader(content)
	err = client.PutObject(ctx, bucketName, "test.json", reader, int64(len(content)),
		minio.WithContentType("application/json"))
	require.NoError(t, err)

	// Verify content type
	info, err := client.StatObject(ctx, bucketName, "test.json")
	require.NoError(t, err)
	assert.Equal(t, "application/json", info.ContentType)
}

func TestMinIOIntegration_PutObject_WithMetadata(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-metadata-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "test-metadata.txt")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Put object with metadata
	content := []byte("metadata test")
	reader := bytes.NewReader(content)
	metadata := map[string]string{
		"Custom-Header": "custom-value",
		"Project":       "HelixAgent",
	}
	err = client.PutObject(ctx, bucketName, "test-metadata.txt", reader, int64(len(content)),
		minio.WithMetadata(metadata))
	require.NoError(t, err)
}

func TestMinIOIntegration_GetObject(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-get-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "test-get.txt")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Put object
	originalContent := []byte("This is test content for GetObject")
	err = client.PutObject(ctx, bucketName, "test-get.txt", bytes.NewReader(originalContent), int64(len(originalContent)))
	require.NoError(t, err)

	// Get object
	obj, err := client.GetObject(ctx, bucketName, "test-get.txt")
	require.NoError(t, err)
	defer obj.Close()

	// Read content
	content, err := io.ReadAll(obj)
	require.NoError(t, err)
	assert.Equal(t, originalContent, content)
}

func TestMinIOIntegration_DeleteObject(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-delete-obj-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Put object
	content := []byte("delete me")
	err = client.PutObject(ctx, bucketName, "to-delete.txt", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	// Delete object
	err = client.DeleteObject(ctx, bucketName, "to-delete.txt")
	require.NoError(t, err)
}

func TestMinIOIntegration_ListObjects(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-list-obj-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "folder/file1.txt")
		client.DeleteObject(ctx, bucketName, "folder/file2.txt")
		client.DeleteObject(ctx, bucketName, "other/file3.txt")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Put multiple objects
	content := []byte("test")
	err = client.PutObject(ctx, bucketName, "folder/file1.txt", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	err = client.PutObject(ctx, bucketName, "folder/file2.txt", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	err = client.PutObject(ctx, bucketName, "other/file3.txt", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	// List all objects
	objects, err := client.ListObjects(ctx, bucketName, "")
	require.NoError(t, err)
	assert.Len(t, objects, 3)

	// List objects with prefix
	objects, err = client.ListObjects(ctx, bucketName, "folder/")
	require.NoError(t, err)
	assert.Len(t, objects, 2)
}

func TestMinIOIntegration_StatObject(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-stat-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "stat-test.txt")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Put object
	content := []byte("stat test content")
	err = client.PutObject(ctx, bucketName, "stat-test.txt", bytes.NewReader(content), int64(len(content)),
		minio.WithContentType("text/plain"))
	require.NoError(t, err)

	// Stat object
	info, err := client.StatObject(ctx, bucketName, "stat-test.txt")
	require.NoError(t, err)
	assert.Equal(t, "stat-test.txt", info.Key)
	assert.Equal(t, int64(len(content)), info.Size)
	assert.Equal(t, "text/plain", info.ContentType)
	assert.NotEmpty(t, info.ETag)
	assert.False(t, info.LastModified.IsZero())
}

// =====================================================
// Presigned URL Tests
// =====================================================

func TestMinIOIntegration_GetPresignedURL(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-presigned-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "presigned.txt")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Put object
	content := []byte("presigned url test")
	err = client.PutObject(ctx, bucketName, "presigned.txt", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	// Get presigned URL
	url, err := client.GetPresignedURL(ctx, bucketName, "presigned.txt", time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, bucketName)
	assert.Contains(t, url, "presigned.txt")
}

func TestMinIOIntegration_GetPresignedPutURL(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-presigned-put-%d", time.Now().UnixNano())
	bucketConfig := minio.DefaultBucketConfig(bucketName)

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Get presigned PUT URL
	url, err := client.GetPresignedPutURL(ctx, bucketName, "upload-via-presigned.txt", time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, bucketName)
	assert.Contains(t, url, "upload-via-presigned.txt")
}

// =====================================================
// Copy Object Tests
// =====================================================

func TestMinIOIntegration_CopyObject(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	srcBucket := fmt.Sprintf("test-copy-src-%d", time.Now().UnixNano())
	dstBucket := fmt.Sprintf("test-copy-dst-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create buckets
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(srcBucket))
	require.NoError(t, err)
	err = client.CreateBucket(ctx, minio.DefaultBucketConfig(dstBucket))
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, srcBucket, "source.txt")
		client.DeleteObject(ctx, dstBucket, "destination.txt")
		client.DeleteBucket(ctx, srcBucket)
		client.DeleteBucket(ctx, dstBucket)
	}()

	// Put source object
	content := []byte("copy test content")
	err = client.PutObject(ctx, srcBucket, "source.txt", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	// Copy object
	err = client.CopyObject(ctx, srcBucket, "source.txt", dstBucket, "destination.txt")
	require.NoError(t, err)

	// Verify destination
	obj, err := client.GetObject(ctx, dstBucket, "destination.txt")
	require.NoError(t, err)
	defer obj.Close()

	destContent, err := io.ReadAll(obj)
	require.NoError(t, err)
	assert.Equal(t, content, destContent)
}

func TestMinIOIntegration_CopyObject_SameBucket(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-copy-same-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "original.txt")
		client.DeleteObject(ctx, bucketName, "copy.txt")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Put source object
	content := []byte("same bucket copy")
	err = client.PutObject(ctx, bucketName, "original.txt", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	// Copy within same bucket
	err = client.CopyObject(ctx, bucketName, "original.txt", bucketName, "copy.txt")
	require.NoError(t, err)

	// Verify both exist
	objects, err := client.ListObjects(ctx, bucketName, "")
	require.NoError(t, err)
	assert.Len(t, objects, 2)
}

// =====================================================
// Lifecycle Rule Tests
// =====================================================

func TestMinIOIntegration_SetLifecycleRule(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-lifecycle-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Set lifecycle rule
	rule := minio.DefaultLifecycleRule("expire-30-days", 30)
	err = client.SetLifecycleRule(ctx, bucketName, rule)
	require.NoError(t, err)
}

func TestMinIOIntegration_SetLifecycleRule_WithPrefix(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-lifecycle-prefix-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Set lifecycle rule with prefix
	rule := &minio.LifecycleRule{
		ID:             "logs-cleanup",
		Prefix:         "logs/",
		Enabled:        true,
		ExpirationDays: 14,
	}
	err = client.SetLifecycleRule(ctx, bucketName, rule)
	require.NoError(t, err)
}

func TestMinIOIntegration_SetLifecycleRule_WithNoncurrentExpiry(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-lifecycle-noncurrent-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket with versioning
	bucketConfig := &minio.BucketConfig{
		Name:       bucketName,
		Versioning: true,
	}
	err := client.CreateBucket(ctx, bucketConfig)
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Set lifecycle rule with noncurrent expiry
	rule := &minio.LifecycleRule{
		ID:             "noncurrent-cleanup",
		Enabled:        true,
		NoncurrentDays: 7,
		ExpirationDays: 90,
	}
	err = client.SetLifecycleRule(ctx, bucketName, rule)
	require.NoError(t, err)
}

func TestMinIOIntegration_SetLifecycleRule_Disabled(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-lifecycle-disabled-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Set disabled lifecycle rule
	rule := &minio.LifecycleRule{
		ID:             "disabled-rule",
		Enabled:        false,
		ExpirationDays: 30,
	}
	err = client.SetLifecycleRule(ctx, bucketName, rule)
	require.NoError(t, err)
}

func TestMinIOIntegration_SetLifecycleRule_UpdateExisting(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-lifecycle-update-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Set initial rule
	rule := &minio.LifecycleRule{
		ID:             "update-me",
		Enabled:        true,
		ExpirationDays: 30,
	}
	err = client.SetLifecycleRule(ctx, bucketName, rule)
	require.NoError(t, err)

	// Update the same rule
	rule.ExpirationDays = 60
	err = client.SetLifecycleRule(ctx, bucketName, rule)
	require.NoError(t, err)
}

func TestMinIOIntegration_RemoveLifecycleRule(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-remove-lifecycle-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Set lifecycle rule
	rule := minio.DefaultLifecycleRule("to-remove", 30)
	err = client.SetLifecycleRule(ctx, bucketName, rule)
	require.NoError(t, err)

	// Remove lifecycle rule
	err = client.RemoveLifecycleRule(ctx, bucketName, "to-remove")
	require.NoError(t, err)
}

func TestMinIOIntegration_RemoveLifecycleRule_LastRule(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-remove-last-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Set single lifecycle rule
	rule := minio.DefaultLifecycleRule("only-rule", 30)
	err = client.SetLifecycleRule(ctx, bucketName, rule)
	require.NoError(t, err)

	// Remove the only rule - should remove entire lifecycle config
	err = client.RemoveLifecycleRule(ctx, bucketName, "only-rule")
	require.NoError(t, err)
}

// =====================================================
// Large File Tests
// =====================================================

func TestMinIOIntegration_LargeObject(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-large-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer func() {
		client.DeleteObject(ctx, bucketName, "large-file.bin")
		client.DeleteBucket(ctx, bucketName)
	}()

	// Create 1MB content
	size := 1024 * 1024
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	// Upload large file
	err = client.PutObject(ctx, bucketName, "large-file.bin", bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)

	// Verify size
	info, err := client.StatObject(ctx, bucketName, "large-file.bin")
	require.NoError(t, err)
	assert.Equal(t, int64(size), info.Size)

	// Download and verify
	obj, err := client.GetObject(ctx, bucketName, "large-file.bin")
	require.NoError(t, err)
	defer obj.Close()

	downloaded, err := io.ReadAll(obj)
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)
}

// =====================================================
// Error Handling Tests
// =====================================================

func TestMinIOIntegration_DeleteNonExistentBucket(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	err := client.DeleteBucket(context.Background(), "nonexistent-bucket-12345")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete bucket")
}

func TestMinIOIntegration_GetNonExistentObject(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-error-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Try to get non-existent object
	obj, err := client.GetObject(ctx, bucketName, "nonexistent.txt")
	require.NoError(t, err) // GetObject doesn't fail immediately
	defer obj.Close()

	// Reading will fail
	_, err = io.ReadAll(obj)
	require.Error(t, err)
}

func TestMinIOIntegration_StatNonExistentObject(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-stat-error-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer client.DeleteBucket(ctx, bucketName)

	// Stat non-existent object
	info, err := client.StatObject(ctx, bucketName, "nonexistent.txt")
	require.Error(t, err)
	assert.Nil(t, info)
}

// =====================================================
// Concurrent Operations Tests
// =====================================================

func TestMinIOIntegration_ConcurrentUploads(t *testing.T) {
	client := skipMinIOIfUnavailable(t)
	defer client.Close()

	bucketName := fmt.Sprintf("test-concurrent-%d", time.Now().UnixNano())

	ctx := context.Background()

	// Create bucket
	err := client.CreateBucket(ctx, minio.DefaultBucketConfig(bucketName))
	require.NoError(t, err)
	defer func() {
		objects, _ := client.ListObjects(ctx, bucketName, "")
		for _, obj := range objects {
			client.DeleteObject(ctx, bucketName, obj.Key)
		}
		client.DeleteBucket(ctx, bucketName)
	}()

	// Upload multiple objects concurrently
	numObjects := 10
	errChan := make(chan error, numObjects)

	for i := 0; i < numObjects; i++ {
		go func(idx int) {
			content := []byte(fmt.Sprintf("content-%d", idx))
			key := fmt.Sprintf("object-%d.txt", idx)
			errChan <- client.PutObject(ctx, bucketName, key, bytes.NewReader(content), int64(len(content)))
		}(i)
	}

	// Wait for all uploads
	for i := 0; i < numObjects; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}

	// Verify all objects exist
	objects, err := client.ListObjects(ctx, bucketName, "")
	require.NoError(t, err)
	assert.Len(t, objects, numObjects)
}
