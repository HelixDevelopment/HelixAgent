package qdrant

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 6333, config.HTTPPort)
	assert.Equal(t, 6334, config.GRPCPort)
	assert.Empty(t, config.APIKey)
	assert.False(t, config.UseGRPC)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 10, config.DefaultLimit)
	assert.Equal(t, float32(0.0), config.ScoreThreshold)
	assert.True(t, config.WithPayload)
	assert.False(t, config.WithVectors)
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
			name: "empty host",
			modify: func(c *Config) {
				c.Host = ""
			},
			expectError: true,
			errorMsg:    "host is required",
		},
		{
			name: "invalid http port",
			modify: func(c *Config) {
				c.HTTPPort = 0
			},
			expectError: true,
			errorMsg:    "http_port must be between 1 and 65535",
		},
		{
			name: "invalid grpc port",
			modify: func(c *Config) {
				c.GRPCPort = 70000
			},
			expectError: true,
			errorMsg:    "grpc_port must be between 1 and 65535",
		},
		{
			name: "invalid timeout",
			modify: func(c *Config) {
				c.Timeout = 0
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
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
			name: "invalid default limit",
			modify: func(c *Config) {
				c.DefaultLimit = 0
			},
			expectError: true,
			errorMsg:    "default_limit must be at least 1",
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

func TestConfigGetHTTPURL(t *testing.T) {
	config := DefaultConfig()
	config.Host = "qdrant-server"
	config.HTTPPort = 6333

	assert.Equal(t, "http://qdrant-server:6333", config.GetHTTPURL())
}

func TestConfigGetGRPCAddress(t *testing.T) {
	config := DefaultConfig()
	config.Host = "qdrant-server"
	config.GRPCPort = 6334

	assert.Equal(t, "qdrant-server:6334", config.GetGRPCAddress())
}

func TestDefaultCollectionConfig(t *testing.T) {
	config := DefaultCollectionConfig("test-collection", 1536)

	assert.Equal(t, "test-collection", config.Name)
	assert.Equal(t, 1536, config.VectorSize)
	assert.Equal(t, DistanceCosine, config.Distance)
	assert.False(t, config.OnDiskPayload)
	assert.Equal(t, 20000, config.IndexingThreshold)
	assert.Equal(t, 1, config.ReplicationFactor)
	assert.Equal(t, 1, config.ShardNumber)
}

func TestCollectionConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *CollectionConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      DefaultCollectionConfig("test", 1536),
			expectError: false,
		},
		{
			name: "empty name",
			config: &CollectionConfig{
				Name:       "",
				VectorSize: 1536,
				Distance:   DistanceCosine,
			},
			expectError: true,
			errorMsg:    "collection name is required",
		},
		{
			name: "invalid vector size",
			config: &CollectionConfig{
				Name:       "test",
				VectorSize: 0,
				Distance:   DistanceCosine,
			},
			expectError: true,
			errorMsg:    "vector_size must be at least 1",
		},
		{
			name: "invalid distance",
			config: &CollectionConfig{
				Name:       "test",
				VectorSize: 1536,
				Distance:   "invalid",
			},
			expectError: true,
			errorMsg:    "invalid distance metric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCollectionConfigChaining(t *testing.T) {
	config := DefaultCollectionConfig("embeddings", 768).
		WithDistance(DistanceEuclid).
		WithOnDiskPayload().
		WithIndexingThreshold(50000).
		WithShards(3).
		WithReplication(2)

	assert.Equal(t, "embeddings", config.Name)
	assert.Equal(t, 768, config.VectorSize)
	assert.Equal(t, DistanceEuclid, config.Distance)
	assert.True(t, config.OnDiskPayload)
	assert.Equal(t, 50000, config.IndexingThreshold)
	assert.Equal(t, 3, config.ShardNumber)
	assert.Equal(t, 2, config.ReplicationFactor)
}

func TestDefaultSearchOptions(t *testing.T) {
	opts := DefaultSearchOptions()

	assert.Equal(t, 10, opts.Limit)
	assert.Equal(t, 0, opts.Offset)
	assert.Equal(t, float32(0.0), opts.ScoreThreshold)
	assert.True(t, opts.WithPayload)
	assert.False(t, opts.WithVectors)
	assert.Nil(t, opts.Filter)
}

func TestSearchOptionsChaining(t *testing.T) {
	filter := map[string]interface{}{
		"must": []map[string]interface{}{
			{"key": "category", "match": map[string]interface{}{"value": "tech"}},
		},
	}

	opts := DefaultSearchOptions().
		WithLimit(20).
		WithOffset(10).
		WithScoreThreshold(0.7).
		WithVectorsEnabled().
		WithFilter(filter)

	assert.Equal(t, 20, opts.Limit)
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, float32(0.7), opts.ScoreThreshold)
	assert.True(t, opts.WithVectors)
	assert.NotNil(t, opts.Filter)
}
