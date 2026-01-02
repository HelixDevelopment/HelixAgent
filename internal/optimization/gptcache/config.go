package gptcache

import (
	"time"
)

// Config holds configuration for the semantic cache.
type Config struct {
	// MaxEntries is the maximum number of entries to cache.
	MaxEntries int `yaml:"max_entries" json:"max_entries"`

	// SimilarityThreshold is the minimum similarity score (0-1) for a cache hit.
	SimilarityThreshold float64 `yaml:"similarity_threshold" json:"similarity_threshold"`

	// SimilarityMetric is the metric used to compute similarity.
	SimilarityMetric SimilarityMetric `yaml:"similarity_metric" json:"similarity_metric"`

	// TTL is the time-to-live for cache entries.
	TTL time.Duration `yaml:"ttl" json:"ttl"`

	// EvictionPolicy determines how entries are evicted when capacity is reached.
	EvictionPolicy EvictionPolicy `yaml:"eviction_policy" json:"eviction_policy"`

	// EmbeddingDimension is the expected dimension of embedding vectors.
	EmbeddingDimension int `yaml:"embedding_dimension" json:"embedding_dimension"`

	// NormalizeEmbeddings controls whether embeddings should be L2 normalized.
	NormalizeEmbeddings bool `yaml:"normalize_embeddings" json:"normalize_embeddings"`

	// DecayFactor is used for relevance-based eviction (0-1).
	DecayFactor float64 `yaml:"decay_factor" json:"decay_factor"`
}

// EvictionPolicy defines the eviction policy type.
type EvictionPolicy string

const (
	// EvictionLRU uses Least Recently Used eviction.
	EvictionLRU EvictionPolicy = "lru"
	// EvictionTTL uses Time-To-Live eviction.
	EvictionTTL EvictionPolicy = "ttl"
	// EvictionLRUWithTTL combines LRU and TTL eviction.
	EvictionLRUWithTTL EvictionPolicy = "lru_with_ttl"
	// EvictionRelevance uses relevance-based eviction.
	EvictionRelevance EvictionPolicy = "relevance"
)

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxEntries:          10000,
		SimilarityThreshold: 0.85,
		SimilarityMetric:    MetricCosine,
		TTL:                 24 * time.Hour,
		EvictionPolicy:      EvictionLRUWithTTL,
		EmbeddingDimension:  1536, // OpenAI text-embedding-3-small default
		NormalizeEmbeddings: true,
		DecayFactor:         0.95,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.MaxEntries <= 0 {
		c.MaxEntries = 10000
	}
	if c.SimilarityThreshold < 0 || c.SimilarityThreshold > 1 {
		c.SimilarityThreshold = 0.85
	}
	if c.TTL <= 0 {
		c.TTL = 24 * time.Hour
	}
	if c.EmbeddingDimension <= 0 {
		c.EmbeddingDimension = 1536
	}
	if c.DecayFactor <= 0 || c.DecayFactor > 1 {
		c.DecayFactor = 0.95
	}
	return nil
}

// ConfigOption is a functional option for configuring the cache.
type ConfigOption func(*Config)

// WithMaxEntries sets the maximum number of entries.
func WithMaxEntries(n int) ConfigOption {
	return func(c *Config) {
		c.MaxEntries = n
	}
}

// WithSimilarityThreshold sets the similarity threshold.
func WithSimilarityThreshold(threshold float64) ConfigOption {
	return func(c *Config) {
		c.SimilarityThreshold = threshold
	}
}

// WithSimilarityMetric sets the similarity metric.
func WithSimilarityMetric(metric SimilarityMetric) ConfigOption {
	return func(c *Config) {
		c.SimilarityMetric = metric
	}
}

// WithTTL sets the time-to-live.
func WithTTL(ttl time.Duration) ConfigOption {
	return func(c *Config) {
		c.TTL = ttl
	}
}

// WithEvictionPolicy sets the eviction policy.
func WithEvictionPolicy(policy EvictionPolicy) ConfigOption {
	return func(c *Config) {
		c.EvictionPolicy = policy
	}
}

// WithEmbeddingDimension sets the embedding dimension.
func WithEmbeddingDimension(dim int) ConfigOption {
	return func(c *Config) {
		c.EmbeddingDimension = dim
	}
}

// WithNormalizeEmbeddings enables or disables embedding normalization.
func WithNormalizeEmbeddings(normalize bool) ConfigOption {
	return func(c *Config) {
		c.NormalizeEmbeddings = normalize
	}
}

// WithDecayFactor sets the decay factor for relevance eviction.
func WithDecayFactor(factor float64) ConfigOption {
	return func(c *Config) {
		c.DecayFactor = factor
	}
}
