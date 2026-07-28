package qdrant

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Config Comprehensive Tests
// =============================================================================

func TestConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "port at lower boundary",
			config: &Config{
				Host:         "localhost",
				HTTPPort:     1,
				GRPCPort:     1,
				Timeout:      time.Second,
				MaxRetries:   0,
				DefaultLimit: 1,
			},
			expectError: false,
		},
		{
			name: "port at upper boundary",
			config: &Config{
				Host:         "localhost",
				HTTPPort:     65535,
				GRPCPort:     65535,
				Timeout:      time.Second,
				MaxRetries:   0,
				DefaultLimit: 1,
			},
			expectError: false,
		},
		{
			name: "http port above upper boundary",
			config: &Config{
				Host:         "localhost",
				HTTPPort:     65536,
				GRPCPort:     6334,
				Timeout:      time.Second,
				MaxRetries:   0,
				DefaultLimit: 1,
			},
			expectError: true,
			errorMsg:    "http_port must be between 1 and 65535",
		},
		{
			name: "negative http port",
			config: &Config{
				Host:         "localhost",
				HTTPPort:     -1,
				GRPCPort:     6334,
				Timeout:      time.Second,
				MaxRetries:   0,
				DefaultLimit: 1,
			},
			expectError: true,
			errorMsg:    "http_port must be between 1 and 65535",
		},
		{
			name: "negative timeout",
			config: &Config{
				Host:         "localhost",
				HTTPPort:     6333,
				GRPCPort:     6334,
				Timeout:      -1 * time.Second,
				MaxRetries:   0,
				DefaultLimit: 1,
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name: "whitespace only host",
			config: &Config{
				Host:         "   ",
				HTTPPort:     6333,
				GRPCPort:     6334,
				Timeout:      time.Second,
				MaxRetries:   0,
				DefaultLimit: 1,
			},
			expectError: false, // Note: whitespace is technically valid (though not useful)
		},
		{
			name: "very large max retries",
			config: &Config{
				Host:         "localhost",
				HTTPPort:     6333,
				GRPCPort:     6334,
				Timeout:      time.Second,
				MaxRetries:   1000000,
				DefaultLimit: 1,
			},
			expectError: false,
		},
		{
			name: "very large default limit",
			config: &Config{
				Host:         "localhost",
				HTTPPort:     6333,
				GRPCPort:     6334,
				Timeout:      time.Second,
				MaxRetries:   0,
				DefaultLimit: 1000000,
			},
			expectError: false,
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

func TestConfig_GetHTTPURL_Variations(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "localhost standard port",
			host:     "localhost",
			port:     6333,
			expected: "http://localhost:6333",
		},
		{
			name:     "IP address",
			host:     "192.168.1.100",
			port:     6333,
			expected: "http://192.168.1.100:6333",
		},
		{
			// Reconciled: this case previously asserted "http://::1:6333",
			// which net/url rejects with `invalid port "::1:6333" after
			// host`. The assertion pinned a defect, so it was rewritten to
			// the bracketed authority GetHTTPURL now emits rather than
			// reverting the fix.
			name:     "IPv6 address",
			host:     "::1",
			port:     6333,
			expected: "http://[::1]:6333",
		},
		{
			// An operator may already write the address in bracketed form;
			// it must NOT be double-bracketed into "http://[[::1]]:6333".
			name:     "IPv6 address already bracketed",
			host:     "[::1]",
			port:     6333,
			expected: "http://[::1]:6333",
		},
		{
			name:     "domain name",
			host:     "qdrant.example.com",
			port:     443,
			expected: "http://qdrant.example.com:443",
		},
		{
			name:     "port 80",
			host:     "localhost",
			port:     80,
			expected: "http://localhost:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Host:     tt.host,
				HTTPPort: tt.port,
			}
			assert.Equal(t, tt.expected, config.GetHTTPURL())
		})
	}
}

func TestConfig_GetGRPCAddress_Variations(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "localhost standard port",
			host:     "localhost",
			port:     6334,
			expected: "localhost:6334",
		},
		{
			name:     "IP address",
			host:     "192.168.1.100",
			port:     6334,
			expected: "192.168.1.100:6334",
		},
		{
			name:     "domain name",
			host:     "qdrant.example.com",
			port:     443,
			expected: "qdrant.example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Host:     tt.host,
				GRPCPort: tt.port,
			}
			assert.Equal(t, tt.expected, config.GetGRPCAddress())
		})
	}
}

// =============================================================================
// CollectionConfig Comprehensive Tests
// =============================================================================

func TestCollectionConfig_AllDistances(t *testing.T) {
	distances := []Distance{
		DistanceCosine,
		DistanceEuclid,
		DistanceDot,
		DistanceManhattan,
	}

	for _, d := range distances {
		t.Run(string(d), func(t *testing.T) {
			config := &CollectionConfig{
				Name:       "test",
				VectorSize: 128,
				Distance:   d,
			}
			err := config.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestCollectionConfig_InvalidDistances(t *testing.T) {
	invalidDistances := []Distance{
		"",
		"Invalid",
		"cosine",    // lowercase
		"COSINE",    // uppercase
		"L2",        // alternative name
		"euclidean", // alternative name
	}

	for _, d := range invalidDistances {
		t.Run(string(d), func(t *testing.T) {
			config := &CollectionConfig{
				Name:       "test",
				VectorSize: 128,
				Distance:   d,
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid distance metric")
		})
	}
}

func TestCollectionConfig_VectorSizeBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		vectorSize  int
		expectError bool
	}{
		{"size 0", 0, true},
		{"size -1", -1, true},
		{"size 1", 1, false},
		{"size 128", 128, false},
		{"size 768", 768, false},
		{"size 1536", 1536, false},
		{"size 4096", 4096, false},
		{"size large", 100000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &CollectionConfig{
				Name:       "test",
				VectorSize: tt.vectorSize,
				Distance:   DistanceCosine,
			}
			err := config.Validate()
			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCollectionConfig_NameValidation(t *testing.T) {
	tests := []struct {
		name        string
		collName    string
		expectError bool
	}{
		{"empty name", "", true},
		{"single char", "a", false},
		{"alphanumeric", "test123", false},
		{"with underscore", "test_collection", false},
		{"with hyphen", "test-collection", false},
		{"with space", "test collection", false}, // Spaces are allowed (though not recommended)
		{"unicode", "test_\u4e2d\u6587", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &CollectionConfig{
				Name:       tt.collName,
				VectorSize: 128,
				Distance:   DistanceCosine,
			}
			err := config.Validate()
			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCollectionConfig_ChainingMethods(t *testing.T) {
	// Test that all chaining methods work correctly
	config := DefaultCollectionConfig("test", 768)

	// Each method should return the same config instance
	c1 := config.WithDistance(DistanceEuclid)
	assert.Same(t, config, c1)

	c2 := config.WithOnDiskPayload()
	assert.Same(t, config, c2)

	c3 := config.WithIndexingThreshold(10000)
	assert.Same(t, config, c3)

	c4 := config.WithShards(5)
	assert.Same(t, config, c4)

	c5 := config.WithReplication(3)
	assert.Same(t, config, c5)

	// Verify all values were set
	assert.Equal(t, DistanceEuclid, config.Distance)
	assert.True(t, config.OnDiskPayload)
	assert.Equal(t, 10000, config.IndexingThreshold)
	assert.Equal(t, 5, config.ShardNumber)
	assert.Equal(t, 3, config.ReplicationFactor)
}

// =============================================================================
// SearchOptions Comprehensive Tests
// =============================================================================

func TestSearchOptions_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		opts *SearchOptions
	}{
		{
			name: "zero limit",
			opts: DefaultSearchOptions().WithLimit(0),
		},
		{
			name: "negative limit",
			opts: DefaultSearchOptions().WithLimit(-1),
		},
		{
			name: "zero offset",
			opts: DefaultSearchOptions().WithOffset(0),
		},
		{
			name: "negative offset",
			opts: DefaultSearchOptions().WithOffset(-1),
		},
		{
			name: "zero score threshold",
			opts: DefaultSearchOptions().WithScoreThreshold(0.0),
		},
		{
			name: "negative score threshold",
			opts: DefaultSearchOptions().WithScoreThreshold(-0.5),
		},
		{
			name: "score threshold above 1",
			opts: DefaultSearchOptions().WithScoreThreshold(1.5),
		},
		{
			name: "nil filter",
			opts: DefaultSearchOptions().WithFilter(nil),
		},
		{
			name: "empty filter",
			opts: DefaultSearchOptions().WithFilter(map[string]interface{}{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			assert.NotNil(t, tt.opts)
		})
	}
}

func TestSearchOptions_ComplexFilter(t *testing.T) {
	filter := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"key":   "category",
				"match": map[string]interface{}{"value": "tech"},
			},
			{
				"key": "price",
				"range": map[string]interface{}{
					"gte": 10.0,
					"lte": 100.0,
				},
			},
		},
		"should": []map[string]interface{}{
			{
				"key":   "brand",
				"match": map[string]interface{}{"value": "acme"},
			},
		},
		"must_not": []map[string]interface{}{
			{
				"key":   "status",
				"match": map[string]interface{}{"value": "discontinued"},
			},
		},
	}

	opts := DefaultSearchOptions().WithFilter(filter)
	assert.NotNil(t, opts.Filter)
	assert.Equal(t, filter, opts.Filter)
}

func TestSearchOptions_ChainingMethods(t *testing.T) {
	opts := DefaultSearchOptions()

	// Each method should return the same options instance
	o1 := opts.WithLimit(20)
	assert.Same(t, opts, o1)

	o2 := opts.WithOffset(10)
	assert.Same(t, opts, o2)

	o3 := opts.WithScoreThreshold(0.5)
	assert.Same(t, opts, o3)

	o4 := opts.WithVectorsEnabled()
	assert.Same(t, opts, o4)

	o5 := opts.WithFilter(map[string]interface{}{"test": true})
	assert.Same(t, opts, o5)

	// Verify all values were set
	assert.Equal(t, 20, opts.Limit)
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, float32(0.5), opts.ScoreThreshold)
	assert.True(t, opts.WithVectors)
	assert.NotNil(t, opts.Filter)
}

// =============================================================================
// Type Tests
// =============================================================================

func TestPoint_Comprehensive(t *testing.T) {
	point := Point{
		ID:     "test-id-123",
		Vector: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		Payload: map[string]interface{}{
			"string":  "value",
			"int":     42,
			"float":   3.14,
			"bool":    true,
			"array":   []int{1, 2, 3},
			"nested":  map[string]interface{}{"key": "nested_value"},
			"nil_val": nil,
		},
	}

	assert.Equal(t, "test-id-123", point.ID)
	assert.Len(t, point.Vector, 5)
	assert.Equal(t, "value", point.Payload["string"])
	assert.Equal(t, 42, point.Payload["int"])
	assert.Equal(t, 3.14, point.Payload["float"])
	assert.Equal(t, true, point.Payload["bool"])
	assert.Nil(t, point.Payload["nil_val"])
}

func TestScoredPoint_Comprehensive(t *testing.T) {
	sp := ScoredPoint{
		ID:      "result-id",
		Version: 5,
		Score:   0.95,
		Payload: map[string]interface{}{"key": "value"},
		Vector:  []float32{0.1, 0.2},
	}

	assert.Equal(t, "result-id", sp.ID)
	assert.Equal(t, 5, sp.Version)
	assert.Equal(t, float32(0.95), sp.Score)
	assert.Equal(t, "value", sp.Payload["key"])
	assert.Len(t, sp.Vector, 2)
}

func TestCollectionInfo_Comprehensive(t *testing.T) {
	info := CollectionInfo{
		Name:          "my-collection",
		Status:        "green",
		VectorCount:   10000,
		PointsCount:   10000,
		SegmentsCount: 5,
	}

	assert.Equal(t, "my-collection", info.Name)
	assert.Equal(t, "green", info.Status)
	assert.Equal(t, int64(10000), info.VectorCount)
	assert.Equal(t, int64(10000), info.PointsCount)
	assert.Equal(t, 5, info.SegmentsCount)
}

// =============================================================================
// Default Values Tests
// =============================================================================

func TestDefaultConfig_AllFields(t *testing.T) {
	config := DefaultConfig()

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

	// Validate should pass
	err := config.Validate()
	assert.NoError(t, err)
}

func TestDefaultCollectionConfig_AllFields(t *testing.T) {
	config := DefaultCollectionConfig("test-collection", 1536)

	assert.Equal(t, "test-collection", config.Name)
	assert.Equal(t, 1536, config.VectorSize)
	assert.Equal(t, DistanceCosine, config.Distance)
	assert.False(t, config.OnDiskPayload)
	assert.Equal(t, 20000, config.IndexingThreshold)
	assert.Equal(t, 1, config.ReplicationFactor)
	assert.Equal(t, 1, config.WriteConsistency)
	assert.Equal(t, 1, config.ShardNumber)

	// Validate should pass
	err := config.Validate()
	assert.NoError(t, err)
}

func TestDefaultSearchOptions_AllFields(t *testing.T) {
	opts := DefaultSearchOptions()

	assert.Equal(t, 10, opts.Limit)
	assert.Equal(t, 0, opts.Offset)
	assert.Equal(t, float32(0.0), opts.ScoreThreshold)
	assert.True(t, opts.WithPayload)
	assert.False(t, opts.WithVectors)
	assert.Nil(t, opts.Filter)
}
