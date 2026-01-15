package minio

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/sirupsen/logrus"
)

// Client provides an interface to interact with MinIO object storage
type Client struct {
	config      *Config
	minioClient *minio.Client
	logger      *logrus.Logger
	mu          sync.RWMutex
	connected   bool
}

// NewClient creates a new MinIO client
func NewClient(config *Config, logger *logrus.Logger) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &Client{
		config:    config,
		logger:    logger,
		connected: false,
	}, nil
}

// Connect establishes connection to MinIO
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	minioClient, err := minio.New(c.config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.config.AccessKey, c.config.SecretKey, ""),
		Secure: c.config.UseSSL,
		Region: c.config.Region,
	})
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	c.minioClient = minioClient

	// Verify connection by listing buckets
	_, err = minioClient.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to MinIO: %w", err)
	}

	c.connected = true
	c.logger.Info("Connected to MinIO")
	return nil
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	c.minioClient = nil
	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// HealthCheck checks the health of MinIO connection
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to MinIO")
	}

	_, err := c.minioClient.ListBuckets(ctx)
	return err
}

// ObjectInfo represents information about an object
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
	Metadata     map[string]string
}

// BucketInfo represents information about a bucket
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// CreateBucket creates a new bucket
func (c *Client) CreateBucket(ctx context.Context, bucketConfig *BucketConfig) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to MinIO")
	}

	exists, err := c.minioClient.BucketExists(ctx, bucketConfig.Name)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if exists {
		c.logger.WithField("bucket", bucketConfig.Name).Debug("Bucket already exists")
		return nil
	}

	opts := minio.MakeBucketOptions{
		Region:        c.config.Region,
		ObjectLocking: bucketConfig.ObjectLocking,
	}

	if err := c.minioClient.MakeBucket(ctx, bucketConfig.Name, opts); err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	// Enable versioning if configured
	if bucketConfig.Versioning {
		versionConfig := minio.BucketVersioningConfiguration{
			Status: "Enabled",
		}
		if err := c.minioClient.SetBucketVersioning(ctx, bucketConfig.Name, versionConfig); err != nil {
			return fmt.Errorf("failed to enable versioning: %w", err)
		}
	}

	// Set retention policy if configured
	if bucketConfig.RetentionDays > 0 {
		rule := lifecycle.Rule{
			ID:     "auto-expire",
			Status: "Enabled",
			Expiration: lifecycle.Expiration{
				Days: lifecycle.ExpirationDays(bucketConfig.RetentionDays),
			},
		}

		config := &lifecycle.Configuration{
			Rules: []lifecycle.Rule{rule},
		}

		if err := c.minioClient.SetBucketLifecycle(ctx, bucketConfig.Name, config); err != nil {
			c.logger.WithError(err).Warn("Failed to set lifecycle policy")
		}
	}

	c.logger.WithField("bucket", bucketConfig.Name).Info("Bucket created")
	return nil
}

// DeleteBucket deletes a bucket
func (c *Client) DeleteBucket(ctx context.Context, bucketName string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to MinIO")
	}

	if err := c.minioClient.RemoveBucket(ctx, bucketName); err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	c.logger.WithField("bucket", bucketName).Info("Bucket deleted")
	return nil
}

// ListBuckets returns all buckets
func (c *Client) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return nil, fmt.Errorf("not connected to MinIO")
	}

	buckets, err := c.minioClient.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	result := make([]BucketInfo, len(buckets))
	for i, bucket := range buckets {
		result[i] = BucketInfo{
			Name:         bucket.Name,
			CreationDate: bucket.CreationDate,
		}
	}

	return result, nil
}

// BucketExists checks if a bucket exists
func (c *Client) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return false, fmt.Errorf("not connected to MinIO")
	}

	return c.minioClient.BucketExists(ctx, bucketName)
}

// PutOption represents an option for put operations
type PutOption func(*minio.PutObjectOptions)

// WithContentType sets the content type
func WithContentType(contentType string) PutOption {
	return func(opts *minio.PutObjectOptions) {
		opts.ContentType = contentType
	}
}

// WithMetadata sets custom metadata
func WithMetadata(metadata map[string]string) PutOption {
	return func(opts *minio.PutObjectOptions) {
		opts.UserMetadata = metadata
	}
}

// PutObject uploads an object to a bucket
func (c *Client) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, opts ...PutOption) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to MinIO")
	}

	putOpts := minio.PutObjectOptions{
		PartSize: uint64(c.config.PartSize),
	}

	for _, opt := range opts {
		opt(&putOpts)
	}

	_, err := c.minioClient.PutObject(ctx, bucketName, objectName, reader, size, putOpts)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"object": objectName,
		"size":   size,
	}).Debug("Object uploaded")

	return nil
}

// GetObject downloads an object from a bucket
func (c *Client) GetObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return nil, fmt.Errorf("not connected to MinIO")
	}

	obj, err := c.minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return obj, nil
}

// DeleteObject deletes an object from a bucket
func (c *Client) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to MinIO")
	}

	if err := c.minioClient.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"object": objectName,
	}).Debug("Object deleted")

	return nil
}

// ListObjects lists objects in a bucket with a prefix
func (c *Client) ListObjects(ctx context.Context, bucketName, prefix string) ([]ObjectInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return nil, fmt.Errorf("not connected to MinIO")
	}

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}

	var objects []ObjectInfo
	for obj := range c.minioClient.ListObjects(ctx, bucketName, opts) {
		if obj.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", obj.Err)
		}
		objects = append(objects, ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ContentType:  obj.ContentType,
			ETag:         obj.ETag,
		})
	}

	return objects, nil
}

// StatObject returns information about an object
func (c *Client) StatObject(ctx context.Context, bucketName, objectName string) (*ObjectInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return nil, fmt.Errorf("not connected to MinIO")
	}

	info, err := c.minioClient.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	return &ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		LastModified: info.LastModified,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		Metadata:     info.UserMetadata,
	}, nil
}

// GetPresignedURL generates a presigned URL for an object
func (c *Client) GetPresignedURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return "", fmt.Errorf("not connected to MinIO")
	}

	presignedURL, err := c.minioClient.PresignedGetObject(ctx, bucketName, objectName, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

// GetPresignedPutURL generates a presigned URL for uploading
func (c *Client) GetPresignedPutURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return "", fmt.Errorf("not connected to MinIO")
	}

	presignedURL, err := c.minioClient.PresignedPutObject(ctx, bucketName, objectName, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

// CopyObject copies an object within MinIO
func (c *Client) CopyObject(ctx context.Context, srcBucket, srcObject, dstBucket, dstObject string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to MinIO")
	}

	src := minio.CopySrcOptions{
		Bucket: srcBucket,
		Object: srcObject,
	}

	dst := minio.CopyDestOptions{
		Bucket: dstBucket,
		Object: dstObject,
	}

	_, err := c.minioClient.CopyObject(ctx, dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"src_bucket": srcBucket,
		"src_object": srcObject,
		"dst_bucket": dstBucket,
		"dst_object": dstObject,
	}).Debug("Object copied")

	return nil
}

// SetLifecycleRule sets a lifecycle rule for a bucket
func (c *Client) SetLifecycleRule(ctx context.Context, bucketName string, rule *LifecycleRule) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to MinIO")
	}

	status := "Enabled"
	if !rule.Enabled {
		status = "Disabled"
	}

	lcRule := lifecycle.Rule{
		ID:     rule.ID,
		Status: status,
	}

	if rule.Prefix != "" {
		lcRule.RuleFilter = lifecycle.Filter{
			Prefix: rule.Prefix,
		}
	}

	if rule.ExpirationDays > 0 {
		lcRule.Expiration = lifecycle.Expiration{
			Days: lifecycle.ExpirationDays(rule.ExpirationDays),
		}
	}

	if rule.NoncurrentDays > 0 {
		lcRule.NoncurrentVersionExpiration = lifecycle.NoncurrentVersionExpiration{
			NoncurrentDays: lifecycle.ExpirationDays(rule.NoncurrentDays),
		}
	}

	if rule.DeleteMarkerExpiry {
		lcRule.Expiration.DeleteMarker = lifecycle.ExpireDeleteMarker(true)
	}

	// Get existing lifecycle config and add/update rule
	existingConfig, err := c.minioClient.GetBucketLifecycle(ctx, bucketName)
	if err != nil {
		// If no existing config, create new one
		existingConfig = &lifecycle.Configuration{}
	}

	// Replace or add the rule
	found := false
	for i, r := range existingConfig.Rules {
		if r.ID == rule.ID {
			existingConfig.Rules[i] = lcRule
			found = true
			break
		}
	}
	if !found {
		existingConfig.Rules = append(existingConfig.Rules, lcRule)
	}

	if err := c.minioClient.SetBucketLifecycle(ctx, bucketName, existingConfig); err != nil {
		return fmt.Errorf("failed to set lifecycle rule: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"rule":   rule.ID,
	}).Info("Lifecycle rule set")

	return nil
}

// RemoveLifecycleRule removes a lifecycle rule from a bucket
func (c *Client) RemoveLifecycleRule(ctx context.Context, bucketName, ruleID string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.minioClient == nil {
		return fmt.Errorf("not connected to MinIO")
	}

	existingConfig, err := c.minioClient.GetBucketLifecycle(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get lifecycle config: %w", err)
	}

	var newRules []lifecycle.Rule
	for _, r := range existingConfig.Rules {
		if r.ID != ruleID {
			newRules = append(newRules, r)
		}
	}

	existingConfig.Rules = newRules

	if len(newRules) == 0 {
		// Remove lifecycle config entirely if no rules left
		if err := c.minioClient.SetBucketLifecycle(ctx, bucketName, nil); err != nil {
			return fmt.Errorf("failed to remove lifecycle config: %w", err)
		}
	} else {
		if err := c.minioClient.SetBucketLifecycle(ctx, bucketName, existingConfig); err != nil {
			return fmt.Errorf("failed to update lifecycle config: %w", err)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"rule":   ruleID,
	}).Info("Lifecycle rule removed")

	return nil
}
