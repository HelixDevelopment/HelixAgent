// Package optimization provides unified LLM optimization capabilities.
package optimization

import (
	"time"
)

// Config holds configuration for all optimization services.
type Config struct {
	// Global settings
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Semantic cache configuration
	SemanticCache SemanticCacheConfig `yaml:"semantic_cache" json:"semantic_cache"`

	// Structured output configuration
	StructuredOutput StructuredOutputConfig `yaml:"structured_output" json:"structured_output"`

	// Streaming configuration
	Streaming StreamingConfig `yaml:"streaming" json:"streaming"`

	// External service configurations
	SGLang     SGLangConfig     `yaml:"sglang" json:"sglang"`
	LlamaIndex LlamaIndexConfig `yaml:"llamaindex" json:"llamaindex"`
	LangChain  LangChainConfig  `yaml:"langchain" json:"langchain"`
	Guidance   GuidanceConfig   `yaml:"guidance" json:"guidance"`
	LMQL       LMQLConfig       `yaml:"lmql" json:"lmql"`

	// Fallback settings
	Fallback FallbackConfig `yaml:"fallback" json:"fallback"`
}

// SemanticCacheConfig holds semantic cache settings.
type SemanticCacheConfig struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	SimilarityThreshold float64       `yaml:"similarity_threshold" json:"similarity_threshold"`
	MaxEntries          int           `yaml:"max_entries" json:"max_entries"`
	TTL                 time.Duration `yaml:"ttl" json:"ttl"`
	EmbeddingModel      string        `yaml:"embedding_model" json:"embedding_model"`
	EvictionPolicy      string        `yaml:"eviction_policy" json:"eviction_policy"`
}

// StructuredOutputConfig holds structured output settings.
type StructuredOutputConfig struct {
	Enabled       bool `yaml:"enabled" json:"enabled"`
	StrictMode    bool `yaml:"strict_mode" json:"strict_mode"`
	RetryOnFail   bool `yaml:"retry_on_failure" json:"retry_on_failure"`
	MaxRetries    int  `yaml:"max_retries" json:"max_retries"`
}

// StreamingConfig holds streaming settings.
type StreamingConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	BufferType       string        `yaml:"buffer_type" json:"buffer_type"`
	ProgressInterval time.Duration `yaml:"progress_interval" json:"progress_interval"`
	RateLimit        float64       `yaml:"rate_limit" json:"rate_limit"`
}

// SGLangConfig holds SGLang service settings.
type SGLangConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	Endpoint             string        `yaml:"endpoint" json:"endpoint"`
	Timeout              time.Duration `yaml:"timeout" json:"timeout"`
	FallbackOnUnavailable bool         `yaml:"fallback_on_unavailable" json:"fallback_on_unavailable"`
}

// LlamaIndexConfig holds LlamaIndex service settings.
type LlamaIndexConfig struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`
	Endpoint      string        `yaml:"endpoint" json:"endpoint"`
	Timeout       time.Duration `yaml:"timeout" json:"timeout"`
	UseCogneeIndex bool         `yaml:"use_cognee_index" json:"use_cognee_index"`
}

// LangChainConfig holds LangChain service settings.
type LangChainConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	Endpoint     string        `yaml:"endpoint" json:"endpoint"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
	DefaultChain string        `yaml:"default_chain" json:"default_chain"`
}

// GuidanceConfig holds Guidance service settings.
type GuidanceConfig struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`
	Endpoint      string        `yaml:"endpoint" json:"endpoint"`
	Timeout       time.Duration `yaml:"timeout" json:"timeout"`
	CachePrograms bool          `yaml:"cache_programs" json:"cache_programs"`
}

// LMQLConfig holds LMQL service settings.
type LMQLConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	Endpoint     string        `yaml:"endpoint" json:"endpoint"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
	CacheQueries bool          `yaml:"cache_queries" json:"cache_queries"`
}

// FallbackConfig holds fallback behavior settings.
type FallbackConfig struct {
	OnServiceUnavailable  string        `yaml:"on_service_unavailable" json:"on_service_unavailable"` // skip, error, cache_only
	HealthCheckInterval   time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	RetryUnavailableAfter time.Duration `yaml:"retry_unavailable_after" json:"retry_unavailable_after"`
}

// DefaultConfig returns a default configuration with all optimizations enabled.
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,

		SemanticCache: SemanticCacheConfig{
			Enabled:             true,
			SimilarityThreshold: 0.85,
			MaxEntries:          10000,
			TTL:                 24 * time.Hour,
			EmbeddingModel:      "text-embedding-3-small",
			EvictionPolicy:      "lru_with_relevance",
		},

		StructuredOutput: StructuredOutputConfig{
			Enabled:     true,
			StrictMode:  true,
			RetryOnFail: true,
			MaxRetries:  3,
		},

		Streaming: StreamingConfig{
			Enabled:          true,
			BufferType:       "word",
			ProgressInterval: 100 * time.Millisecond,
			RateLimit:        0, // Unlimited
		},

		SGLang: SGLangConfig{
			Enabled:              true,
			Endpoint:             "http://localhost:30000",
			Timeout:              120 * time.Second,
			FallbackOnUnavailable: true,
		},

		LlamaIndex: LlamaIndexConfig{
			Enabled:        true,
			Endpoint:       "http://localhost:8012",
			Timeout:        120 * time.Second,
			UseCogneeIndex: true,
		},

		LangChain: LangChainConfig{
			Enabled:      true,
			Endpoint:     "http://localhost:8011",
			Timeout:      120 * time.Second,
			DefaultChain: "react",
		},

		Guidance: GuidanceConfig{
			Enabled:       true,
			Endpoint:      "http://localhost:8013",
			Timeout:       120 * time.Second,
			CachePrograms: true,
		},

		LMQL: LMQLConfig{
			Enabled:      true,
			Endpoint:     "http://localhost:8014",
			Timeout:      120 * time.Second,
			CacheQueries: true,
		},

		Fallback: FallbackConfig{
			OnServiceUnavailable:  "skip",
			HealthCheckInterval:   30 * time.Second,
			RetryUnavailableAfter: 5 * time.Minute,
		},
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.SemanticCache.Enabled {
		if c.SemanticCache.SimilarityThreshold < 0 || c.SemanticCache.SimilarityThreshold > 1 {
			c.SemanticCache.SimilarityThreshold = 0.85
		}
		if c.SemanticCache.MaxEntries <= 0 {
			c.SemanticCache.MaxEntries = 10000
		}
	}

	if c.StructuredOutput.Enabled {
		if c.StructuredOutput.MaxRetries <= 0 {
			c.StructuredOutput.MaxRetries = 3
		}
	}

	return nil
}
