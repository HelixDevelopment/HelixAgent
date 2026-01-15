package minio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "localhost:9000", config.Endpoint)
	assert.Equal(t, "minioadmin", config.AccessKey)
	assert.Equal(t, "minioadmin123", config.SecretKey)
	assert.False(t, config.UseSSL)
	assert.Equal(t, "us-east-1", config.Region)
	assert.Equal(t, 30*time.Second, config.ConnectTimeout)
	assert.Equal(t, 60*time.Second, config.RequestTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, int64(16*1024*1024), config.PartSize)
	assert.Equal(t, 4, config.ConcurrentUploads)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			modify:      func(c *Config) {},
			expectError: false,
		},
		{
			name: "empty endpoint",
			modify: func(c *Config) {
				c.Endpoint = ""
			},
			expectError: true,
			errorMsg:    "endpoint is required",
		},
		{
			name: "empty access key",
			modify: func(c *Config) {
				c.AccessKey = ""
			},
			expectError: true,
			errorMsg:    "access_key is required",
		},
		{
			name: "empty secret key",
			modify: func(c *Config) {
				c.SecretKey = ""
			},
			expectError: true,
			errorMsg:    "secret_key is required",
		},
		{
			name: "invalid connect timeout",
			modify: func(c *Config) {
				c.ConnectTimeout = 0
			},
			expectError: true,
			errorMsg:    "connect_timeout must be positive",
		},
		{
			name: "invalid request timeout",
			modify: func(c *Config) {
				c.RequestTimeout = -1
			},
			expectError: true,
			errorMsg:    "request_timeout must be positive",
		},
		{
			name: "negative max retries",
			modify: func(c *Config) {
				c.MaxRetries = -1
			},
			expectError: true,
			errorMsg:    "max_retries cannot be negative",
		},
		{
			name: "part size too small",
			modify: func(c *Config) {
				c.PartSize = 1024 // 1KB, less than 5MB minimum
			},
			expectError: true,
			errorMsg:    "part_size must be at least 5MB",
		},
		{
			name: "invalid concurrent uploads",
			modify: func(c *Config) {
				c.ConcurrentUploads = 0
			},
			expectError: true,
			errorMsg:    "concurrent_uploads must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.modify(config)

			err := config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultBucketConfig(t *testing.T) {
	config := DefaultBucketConfig("test-bucket")

	assert.Equal(t, "test-bucket", config.Name)
	assert.Equal(t, -1, config.RetentionDays)
	assert.False(t, config.Versioning)
	assert.False(t, config.ObjectLocking)
	assert.False(t, config.Public)
}

func TestBucketConfigChaining(t *testing.T) {
	config := DefaultBucketConfig("test-bucket").
		WithRetention(30).
		WithVersioning().
		WithObjectLocking().
		WithPublicAccess()

	assert.Equal(t, "test-bucket", config.Name)
	assert.Equal(t, 30, config.RetentionDays)
	assert.True(t, config.Versioning)
	assert.True(t, config.ObjectLocking)
	assert.True(t, config.Public)
}

func TestDefaultLifecycleRule(t *testing.T) {
	rule := DefaultLifecycleRule("expire-old", 90)

	assert.Equal(t, "expire-old", rule.ID)
	assert.Equal(t, "", rule.Prefix)
	assert.True(t, rule.Enabled)
	assert.Equal(t, 90, rule.ExpirationDays)
	assert.Equal(t, 0, rule.NoncurrentDays)
	assert.False(t, rule.DeleteMarkerExpiry)
}

func TestLifecycleRuleChaining(t *testing.T) {
	rule := DefaultLifecycleRule("expire-logs", 30).
		WithPrefix("logs/").
		WithNoncurrentExpiry(7)

	assert.Equal(t, "expire-logs", rule.ID)
	assert.Equal(t, "logs/", rule.Prefix)
	assert.Equal(t, 30, rule.ExpirationDays)
	assert.Equal(t, 7, rule.NoncurrentDays)
}
