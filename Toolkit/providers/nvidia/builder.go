// Package nvidia provides configuration builders for Nvidia.
package nvidia

import (
	"fmt"
)

// ConfigBuilder implements the ConfigBuilder interface for Nvidia.
type ConfigBuilder struct{}

// NewConfigBuilder creates a new Nvidia config builder.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{}
}

// Build builds a Nvidia configuration from a map.
func (b *ConfigBuilder) Build(config map[string]interface{}) (interface{}, error) {
	nvidiaConfig := &Config{
		APIKey:    getString(config, "api_key", ""),
		BaseURL:   getString(config, "base_url", "https://integrate.api.nvidia.com/v1"),
		Timeout:   getInt(config, "timeout", 30000),
		Retries:   getInt(config, "retries", 3),
		RateLimit: getInt(config, "rate_limit", 60),
	}

	if nvidiaConfig.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}

	return nvidiaConfig, nil
}

// Validate validates a Nvidia configuration.
func (b *ConfigBuilder) Validate(config interface{}) error {
	c, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("invalid config type")
	}

	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}

	return nil
}

// Merge merges two Nvidia configurations.
func (b *ConfigBuilder) Merge(base, override interface{}) (interface{}, error) {
	baseConfig, ok := base.(*Config)
	if !ok {
		return nil, fmt.Errorf("base config must be *Config")
	}

	overrideConfig, ok := override.(*Config)
	if !ok {
		return nil, fmt.Errorf("override config must be *Config")
	}

	merged := &Config{
		APIKey:    overrideConfig.APIKey,
		BaseURL:   overrideConfig.BaseURL,
		Timeout:   overrideConfig.Timeout,
		Retries:   overrideConfig.Retries,
		RateLimit: overrideConfig.RateLimit,
	}

	if merged.APIKey == "" {
		merged.APIKey = baseConfig.APIKey
	}
	if merged.BaseURL == "" {
		merged.BaseURL = baseConfig.BaseURL
	}
	if merged.Timeout == 0 {
		merged.Timeout = baseConfig.Timeout
	}
	if merged.Retries == 0 {
		merged.Retries = baseConfig.Retries
	}
	if merged.RateLimit == 0 {
		merged.RateLimit = baseConfig.RateLimit
	}

	return merged, nil
}

// Config represents Nvidia-specific configuration.
type Config struct {
	APIKey    string `json:"api_key"`
	BaseURL   string `json:"base_url"`
	Timeout   int    `json:"timeout"`
	Retries   int    `json:"retries"`
	RateLimit int    `json:"rate_limit"`
}

// Helper functions for type-safe config extraction
func getString(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getInt(config map[string]interface{}, key string, defaultValue int) int {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return defaultValue
}
