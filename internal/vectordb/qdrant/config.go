package qdrant

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// Config holds Qdrant connection configuration
type Config struct {
	// Connection settings
	Host     string `json:"host" yaml:"host"`
	HTTPPort int    `json:"http_port" yaml:"http_port"`
	GRPCPort int    `json:"grpc_port" yaml:"grpc_port"`
	APIKey   string `json:"api_key" yaml:"api_key"`
	UseGRPC  bool   `json:"use_grpc" yaml:"use_grpc"`

	// Connection options
	Timeout    time.Duration `json:"timeout" yaml:"timeout"`
	MaxRetries int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay"`

	// Search defaults
	DefaultLimit   int     `json:"default_limit" yaml:"default_limit"`
	ScoreThreshold float32 `json:"score_threshold" yaml:"score_threshold"`
	WithPayload    bool    `json:"with_payload" yaml:"with_payload"`
	WithVectors    bool    `json:"with_vectors" yaml:"with_vectors"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Host:           "localhost",
		HTTPPort:       6333,
		GRPCPort:       6334,
		APIKey:         "",
		UseGRPC:        false,
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
		DefaultLimit:   10,
		ScoreThreshold: 0.0,
		WithPayload:    true,
		WithVectors:    false,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("http_port must be between 1 and 65535")
	}
	if c.GRPCPort <= 0 || c.GRPCPort > 65535 {
		return fmt.Errorf("grpc_port must be between 1 and 65535")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	if c.DefaultLimit < 1 {
		return fmt.Errorf("default_limit must be at least 1")
	}
	return nil
}

// hostPort joins a host and a port into a valid network authority.
//
// It MUST be used instead of fmt.Sprintf("%s:%d", host, port): a bare IPv6
// literal such as "::1" formatted that way yields "::1:6333", which
// net/url rejects with `invalid port "::1:6333" after host`, so every request
// built from it fails.
//
// net.JoinHostPort brackets any host containing a ":", so a host that the
// operator already supplied in bracketed form ("[::1]") would be
// double-bracketed into "[[::1]]:6333". The surrounding brackets are
// therefore stripped first, making the function correct for BOTH the
// bracketed and the unbracketed spelling of the same address.
//
// Hostnames and IPv4 literals contain no ":" and are returned byte-identical
// to the previous fmt.Sprintf behaviour.
func hostPort(host string, port int) string {
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// GetHTTPURL returns the HTTP API URL
func (c *Config) GetHTTPURL() string {
	return "http://" + hostPort(c.Host, c.HTTPPort)
}

// GetGRPCAddress returns the gRPC address
func (c *Config) GetGRPCAddress() string {
	return hostPort(c.Host, c.GRPCPort)
}

// Distance represents the distance metric for vectors
type Distance string

const (
	DistanceCosine    Distance = "Cosine"
	DistanceEuclid    Distance = "Euclid"
	DistanceDot       Distance = "Dot"
	DistanceManhattan Distance = "Manhattan"
)

// CollectionConfig holds configuration for a vector collection
type CollectionConfig struct {
	Name              string   `json:"name" yaml:"name"`
	VectorSize        int      `json:"vector_size" yaml:"vector_size"`
	Distance          Distance `json:"distance" yaml:"distance"`
	OnDiskPayload     bool     `json:"on_disk_payload" yaml:"on_disk_payload"`
	IndexingThreshold int      `json:"indexing_threshold" yaml:"indexing_threshold"`
	ReplicationFactor int      `json:"replication_factor" yaml:"replication_factor"`
	WriteConsistency  int      `json:"write_consistency" yaml:"write_consistency"`
	ShardNumber       int      `json:"shard_number" yaml:"shard_number"`
}

// DefaultCollectionConfig returns a CollectionConfig with defaults
func DefaultCollectionConfig(name string, vectorSize int) *CollectionConfig {
	return &CollectionConfig{
		Name:              name,
		VectorSize:        vectorSize,
		Distance:          DistanceCosine,
		OnDiskPayload:     false,
		IndexingThreshold: 20000,
		ReplicationFactor: 1,
		WriteConsistency:  1,
		ShardNumber:       1,
	}
}

// Validate validates the collection configuration
func (cc *CollectionConfig) Validate() error {
	if cc.Name == "" {
		return fmt.Errorf("collection name is required")
	}
	if cc.VectorSize < 1 {
		return fmt.Errorf("vector_size must be at least 1")
	}
	validDistances := map[Distance]bool{
		DistanceCosine:    true,
		DistanceEuclid:    true,
		DistanceDot:       true,
		DistanceManhattan: true,
	}
	if !validDistances[cc.Distance] {
		return fmt.Errorf("invalid distance metric: %s", cc.Distance)
	}
	return nil
}

// WithDistance sets the distance metric and returns the config for chaining
func (cc *CollectionConfig) WithDistance(d Distance) *CollectionConfig {
	cc.Distance = d
	return cc
}

// WithOnDiskPayload enables on-disk payload storage
func (cc *CollectionConfig) WithOnDiskPayload() *CollectionConfig {
	cc.OnDiskPayload = true
	return cc
}

// WithIndexingThreshold sets the indexing threshold
func (cc *CollectionConfig) WithIndexingThreshold(threshold int) *CollectionConfig {
	cc.IndexingThreshold = threshold
	return cc
}

// WithShards sets the number of shards
func (cc *CollectionConfig) WithShards(n int) *CollectionConfig {
	cc.ShardNumber = n
	return cc
}

// WithReplication sets the replication factor
func (cc *CollectionConfig) WithReplication(factor int) *CollectionConfig {
	cc.ReplicationFactor = factor
	return cc
}

// SearchOptions holds options for vector search
type SearchOptions struct {
	Limit          int                    `json:"limit"`
	Offset         int                    `json:"offset"`
	ScoreThreshold float32                `json:"score_threshold"`
	WithPayload    bool                   `json:"with_payload"`
	WithVectors    bool                   `json:"with_vectors"`
	Filter         map[string]interface{} `json:"filter"`
}

// DefaultSearchOptions returns SearchOptions with defaults
func DefaultSearchOptions() *SearchOptions {
	return &SearchOptions{
		Limit:          10,
		Offset:         0,
		ScoreThreshold: 0.0,
		WithPayload:    true,
		WithVectors:    false,
		Filter:         nil,
	}
}

// WithLimit sets the limit
func (so *SearchOptions) WithLimit(limit int) *SearchOptions {
	so.Limit = limit
	return so
}

// WithOffset sets the offset
func (so *SearchOptions) WithOffset(offset int) *SearchOptions {
	so.Offset = offset
	return so
}

// WithScoreThreshold sets the minimum score threshold
func (so *SearchOptions) WithScoreThreshold(threshold float32) *SearchOptions {
	so.ScoreThreshold = threshold
	return so
}

// WithVectorsEnabled includes vectors in the response
func (so *SearchOptions) WithVectorsEnabled() *SearchOptions {
	so.WithVectors = true
	return so
}

// WithFilter sets a filter for the search
func (so *SearchOptions) WithFilter(filter map[string]interface{}) *SearchOptions {
	so.Filter = filter
	return so
}
