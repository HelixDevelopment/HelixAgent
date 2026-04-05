package adapters

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockS3Client implements S3Client for testing
type MockS3Client struct {
	buckets     []S3Bucket
	objects     []S3Object
	shouldError bool
}

func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		buckets: []S3Bucket{
			{Name: "my-bucket", CreationDate: time.Now().Add(-30 * 24 * time.Hour)},
			{Name: "logs-bucket", CreationDate: time.Now().Add(-60 * 24 * time.Hour)},
			{Name: "backups-bucket", CreationDate: time.Now().Add(-90 * 24 * time.Hour)},
		},
		objects: []S3Object{
			{Key: "folder/file1.txt", Size: 1024, LastModified: time.Now().Add(-time.Hour), ETag: "abc123", StorageClass: "STANDARD"},
			{Key: "folder/file2.txt", Size: 2048, LastModified: time.Now().Add(-2 * time.Hour), ETag: "def456", StorageClass: "STANDARD"},
			{Key: "images/photo.jpg", Size: 5000000, LastModified: time.Now().Add(-24 * time.Hour), ETag: "ghi789", StorageClass: "STANDARD_IA"},
		},
	}
}

func (m *MockS3Client) SetError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockS3Client) ListBuckets(ctx context.Context) ([]S3Bucket, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.buckets, nil
}

func (m *MockS3Client) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]S3Object, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	var result []S3Object
	for _, obj := range m.objects {
		if prefix == "" || strings.HasPrefix(obj.Key, prefix) {
			result = append(result, obj)
			if len(result) >= maxKeys {
				break
			}
		}
	}
	return result, nil
}

func (m *MockS3Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return io.NopCloser(strings.NewReader("file content here")), nil
}

func (m *MockS3Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockS3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockS3Client) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockS3Client) GetObjectMetadata(ctx context.Context, bucket, key string) (*S3ObjectMetadata, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return &S3ObjectMetadata{
		ContentType:   "text/plain",
		ContentLength: 1024,
		LastModified:  time.Now(),
		ETag:          "abc123",
		Metadata:      map[string]string{"custom": "value"},
		StorageClass:  "STANDARD",
	}, nil
}

func (m *MockS3Client) CreateBucket(ctx context.Context, bucket string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockS3Client) DeleteBucket(ctx context.Context, bucket string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

// Tests

func TestDefaultAWSS3Config(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()

	assert.Equal(t, "us-east-1", config.Region)
	assert.Equal(t, 60*time.Second, config.Timeout)
}

func TestNewAWSS3Adapter(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "aws-s3", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestAWSS3Adapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "s3_list_buckets")
	assert.Contains(t, toolNames, "s3_list_objects")
	assert.Contains(t, toolNames, "s3_get_object")
}

func TestAWSS3Adapter_ListBuckets(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_list_buckets", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestAWSS3Adapter_ListObjects(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_list_objects", map[string]interface{}{
		"bucket":   "my-bucket",
		"prefix":   "folder/",
		"max_keys": 100,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAWSS3Adapter_GetObject(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_get_object", map[string]interface{}{
		"bucket": "my-bucket",
		"key":    "folder/file1.txt",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAWSS3Adapter_PutObject(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_put_object", map[string]interface{}{
		"bucket":       "my-bucket",
		"key":          "new-file.txt",
		"content":      "Hello, World!",
		"content_type": "text/plain",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAWSS3Adapter_DeleteObject(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_delete_object", map[string]interface{}{
		"bucket": "my-bucket",
		"key":    "folder/file1.txt",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAWSS3Adapter_CopyObject(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_copy_object", map[string]interface{}{
		"source_bucket": "my-bucket",
		"source_key":    "folder/file1.txt",
		"dest_bucket":   "backups-bucket",
		"dest_key":      "archive/file1.txt",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAWSS3Adapter_GetObjectMetadata(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_get_object_metadata", map[string]interface{}{
		"bucket": "my-bucket",
		"key":    "folder/file1.txt",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAWSS3Adapter_CreateBucket(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_create_bucket", map[string]interface{}{
		"bucket": "new-bucket",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAWSS3Adapter_DeleteBucket(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_delete_bucket", map[string]interface{}{
		"bucket": "old-bucket",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAWSS3Adapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

func TestAWSS3Adapter_ErrorHandling(t *testing.T) {
	t.Parallel()
	config := DefaultAWSS3Config()
	client := NewMockS3Client()
	client.SetError(true)
	adapter := NewAWSS3Adapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "s3_list_buckets", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Type tests

func TestS3BucketTypes(t *testing.T) {
	t.Parallel()
	bucket := S3Bucket{
		Name:         "my-test-bucket",
		CreationDate: time.Now(),
	}

	assert.Equal(t, "my-test-bucket", bucket.Name)
	assert.False(t, bucket.CreationDate.IsZero())
}

func TestS3ObjectTypes(t *testing.T) {
	t.Parallel()
	object := S3Object{
		Key:          "folder/document.pdf",
		Size:         1024000,
		LastModified: time.Now(),
		ETag:         "d41d8cd98f00b204e9800998ecf8427e",
		StorageClass: "STANDARD",
	}

	assert.Equal(t, "folder/document.pdf", object.Key)
	assert.Equal(t, int64(1024000), object.Size)
	assert.Equal(t, "STANDARD", object.StorageClass)
}

func TestS3ObjectMetadataTypes(t *testing.T) {
	t.Parallel()
	metadata := S3ObjectMetadata{
		ContentType:   "application/pdf",
		ContentLength: 1024000,
		LastModified:  time.Now(),
		ETag:          "abc123",
		Metadata:      map[string]string{"author": "test"},
		StorageClass:  "STANDARD_IA",
		VersionID:     "v1",
	}

	assert.Equal(t, "application/pdf", metadata.ContentType)
	assert.Equal(t, int64(1024000), metadata.ContentLength)
	assert.Equal(t, "STANDARD_IA", metadata.StorageClass)
	assert.Equal(t, "test", metadata.Metadata["author"])
}

func TestAWSS3ConfigTypes(t *testing.T) {
	t.Parallel()
	config := AWSS3Config{
		Region:          "eu-west-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		SessionToken:    "session-token",
		Endpoint:        "https://s3.eu-west-1.amazonaws.com",
		Timeout:         120 * time.Second,
	}

	assert.Equal(t, "eu-west-1", config.Region)
	assert.NotEmpty(t, config.AccessKeyID)
	assert.NotEmpty(t, config.SecretAccessKey)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestS3ObjectWithPrefixFiltering(t *testing.T) {
	t.Parallel()
	client := NewMockS3Client()

	ctx := context.Background()
	objects, err := client.ListObjects(ctx, "my-bucket", "folder/", 10)

	assert.NoError(t, err)
	assert.Len(t, objects, 2) // file1.txt and file2.txt
	for _, obj := range objects {
		assert.True(t, strings.HasPrefix(obj.Key, "folder/"))
	}
}

func TestS3ObjectWithMaxKeysLimit(t *testing.T) {
	t.Parallel()
	client := NewMockS3Client()

	ctx := context.Background()
	objects, err := client.ListObjects(ctx, "my-bucket", "", 1)

	assert.NoError(t, err)
	assert.Len(t, objects, 1)
}
