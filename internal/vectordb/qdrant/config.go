package qdrant

import (
	"fmt"
	"time"

	"dev.helix.agent/internal/netaddr"
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

// GetHTTPURL returns the HTTP API URL.
//
// HXC-286 (BLOCKING 2 of a 2026-08-12 review): this and GetGRPCAddress used
// to share a LOCAL hostPort helper that rolled its own bracket-strip plus
// net.JoinHostPort. That helper was a SECOND, divergent definition of "the
// host, unbracketed" living alongside internal/netaddr's, and it reproduced
// BOTH defects that round had just fixed in netaddr — it omitted TrimSpace
// and it had no RFC 6874 zone encoding. Measured against this repository's Go
// toolchain (2026-08-12), every one of these produced a URL that url.Parse
// rejects outright:
//
//	Host="  2001:db8::1  "  -> "http://[  2001:db8::1  ]:6333"  PARSE-FAIL
//	Host=" [2001:db8::1] "  -> "http://[ [2001:db8::1] ]:6333"  PARSE-FAIL
//	Host="fe80::1%eth0"     -> "http://[fe80::1%eth0]:6333"     PARSE-FAIL: invalid URL escape "%et"
//
// Reachability (§11.4.6) — this is REFERENCE-ONLY, not a live user-facing
// path, and an earlier revision of this comment claimed the opposite. It
// asserted "This is LIVE, not theoretical: internal/rag/qdrant_retriever.go
// reaches NewClient through internal/adapters/vectordb/qdrant/adapter.go";
// every link in that chain is false, and the file's own package doc three
// lines up already said so ("zero production imports ... retained only for
// reference"). Measured against this tree (2026-08-13):
//
//   - This package is in 0 of the 8 ./cmd/* dependency closures
//     (`go list -deps ./cmd/<x>` for each). The same instrument finds
//     internal/cache in 4 of them, so that is a real negative rather than a
//     tool limit.
//   - It has ZERO non-test importers module-wide.
//   - internal/adapters/vectordb/qdrant/adapter.go does not import this
//     package at all: it imports the EXTERNAL submodule
//     `digital.vasic.vectordb/pkg/qdrant` and calls extqdrant.NewClient.
//   - internal/rag/qdrant_retriever.go contains no NewClient call.
//
// So the defect these accessors carried was never end-user-reachable, and no
// closure note for this work may imply it was. What the fix IS worth: this
// file held a SECOND, divergent definition of "the host, unbracketed" beside
// internal/netaddr's, reproducing two defects netaddr had already fixed.
// Routing both accessors through netaddr deletes that divergent definition,
// so whoever later wires this package up (or reads it as the reference it is
// retained to be) inherits the correct behaviour instead of copying a broken
// pattern — the §11.4.74 "one definition, never two that can drift" property
// made true of this file too. Its RED test therefore proves a synthetic
// failure, not an end-user one.
//
// BaseURL is
// correct here rather than DialAddress because the result is read by a URL
// parser: it carries the zone encoding, and refuses rather than silently
// redirects a host carrying a URL gen-delimiter. Qdrant is a service
// HelixAgent does not own, so netaddr's no-default-substitution contract —
// never swap a malformed host for a placeholder — is the right one.
func (c *Config) GetHTTPURL() string {
	return netaddr.BaseURL("http", c.Host, c.HTTPPort)
}

// GetGRPCAddress returns the gRPC address.
//
// Uses netaddr.DialAddress, NOT BaseURL: this value is handed to a gRPC
// dialer rather than a URL parser, and net.Dial wants an IPv6 zone RAW. See
// GetHTTPURL for the divergent-helper defect both accessors closed.
func (c *Config) GetGRPCAddress() string {
	return netaddr.DialAddress(c.Host, c.GRPCPort)
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
