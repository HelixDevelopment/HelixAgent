// Package config provides shared configuration structures and utilities.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config represents a generic configuration map.
type Config map[string]interface{}

// GetString retrieves a string value from the config.
func (c Config) GetString(key string) (string, bool) {
	val, ok := c[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetStringWithDefault retrieves a string value with a default.
func (c Config) GetStringWithDefault(key, defaultValue string) string {
	if val, ok := c.GetString(key); ok {
		return val
	}
	return defaultValue
}

// GetInt retrieves an int value from the config.
func (c Config) GetInt(key string) (int, bool) {
	val, ok := c[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}

// GetIntWithDefault retrieves an int value with a default.
func (c Config) GetIntWithDefault(key string, defaultValue int) int {
	if val, ok := c.GetInt(key); ok {
		return val
	}
	return defaultValue
}

// GetBool retrieves a bool value from the config.
func (c Config) GetBool(key string) (bool, bool) {
	val, ok := c[key]
	if !ok {
		return false, false
	}
	switch v := val.(type) {
	case bool:
		return v, true
	case string:
		if b, err := strconv.ParseBool(v); err == nil {
			return b, true
		}
	}
	return false, false
}

// GetBoolWithDefault retrieves a bool value with a default.
func (c Config) GetBoolWithDefault(key string, defaultValue bool) bool {
	if val, ok := c.GetBool(key); ok {
		return val
	}
	return defaultValue
}

// GetFloat retrieves a float64 value from the config.
func (c Config) GetFloat(key string) (float64, bool) {
	val, ok := c[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// GetFloatWithDefault retrieves a float64 value with a default.
func (c Config) GetFloatWithDefault(key string, defaultValue float64) float64 {
	if val, ok := c.GetFloat(key); ok {
		return val
	}
	return defaultValue
}

// Set sets a value in the config.
func (c Config) Set(key string, value interface{}) {
	c[key] = value
}

// ValidateFunc is a function that validates a configuration.
type ValidateFunc func(Config) error

// Validator holds validation rules for configuration.
type Validator struct {
	rules map[string]ValidateFunc
}

// NewValidator creates a new configuration validator.
func NewValidator() *Validator {
	return &Validator{
		rules: make(map[string]ValidateFunc),
	}
}

// AddRule adds a validation rule for a key.
func (v *Validator) AddRule(key string, rule ValidateFunc) {
	v.rules[key] = rule
}

// Validate validates the configuration.
func (v *Validator) Validate(config Config) error {
	for key, rule := range v.rules {
		if err := rule(config); err != nil {
			return fmt.Errorf("validation failed for %s: %w", key, err)
		}
	}
	return nil
}

// Common validation rules

// Required checks if a key is present and not empty.
func Required(key string) ValidateFunc {
	return func(config Config) error {
		val, ok := config[key]
		if !ok {
			return fmt.Errorf("required field %s is missing", key)
		}
		if str, ok := val.(string); ok && strings.TrimSpace(str) == "" {
			return fmt.Errorf("required field %s is empty", key)
		}
		return nil
	}
}

// OneOf checks if a value is one of the allowed values.
func OneOf(key string, allowed ...string) ValidateFunc {
	return func(config Config) error {
		val, ok := config.GetString(key)
		if !ok {
			return nil // Let Required handle missing values
		}
		for _, a := range allowed {
			if val == a {
				return nil
			}
		}
		return fmt.Errorf("field %s must be one of %v, got %s", key, allowed, val)
	}
}

// MinLength checks if a string value has minimum length.
func MinLength(key string, minLen int) ValidateFunc {
	return func(config Config) error {
		val, ok := config.GetString(key)
		if !ok {
			return nil
		}
		if len(val) < minLen {
			return fmt.Errorf("field %s must be at least %d characters long", key, minLen)
		}
		return nil
	}
}

// Environment variable handling

// LoadFromEnv loads configuration from environment variables.
// Environment variables are converted to config keys by removing prefix and lowercasing.
func (c Config) LoadFromEnv(prefix string) {
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}
		key, value := pair[0], pair[1]

		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}

		// Remove prefix and convert to config key
		configKey := strings.ToLower(strings.TrimPrefix(key, prefix))

		// Try to parse as different types
		if intVal, err := strconv.Atoi(value); err == nil {
			c[configKey] = intVal
		} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			c[configKey] = floatVal
		} else if boolVal, err := strconv.ParseBool(value); err == nil {
			c[configKey] = boolVal
		} else {
			c[configKey] = value
		}
	}
}

// ProviderConfig represents common provider configuration.
type ProviderConfig struct {
	APIKey         string `json:"api_key"`
	BaseURL        string `json:"base_url"`
	Timeout        int    `json:"timeout"` // in seconds
	MaxRetries     int    `json:"max_retries"`
	RateLimit      int    `json:"rate_limit"` // requests per second
	RateLimitBurst int    `json:"rate_limit_burst"`
}

// Validate validates the provider configuration.
func (pc *ProviderConfig) Validate() error {
	validator := NewValidator()
	validator.AddRule("api_key", Required("api_key"))
	validator.AddRule("api_key", MinLength("api_key", 1))

	config := Config{
		"api_key": pc.APIKey,
	}

	return validator.Validate(config)
}

// LoadProviderConfigFromEnv loads provider config from environment variables.
func LoadProviderConfigFromEnv(prefix string) *ProviderConfig {
	config := &ProviderConfig{}
	envConfig := Config{}
	envConfig.LoadFromEnv(prefix)

	config.APIKey = envConfig.GetStringWithDefault("api_key", "")
	config.BaseURL = envConfig.GetStringWithDefault("base_url", "")
	config.Timeout = envConfig.GetIntWithDefault("timeout", 30)
	config.MaxRetries = envConfig.GetIntWithDefault("max_retries", 3)
	config.RateLimit = envConfig.GetIntWithDefault("rate_limit", 10)
	config.RateLimitBurst = envConfig.GetIntWithDefault("rate_limit_burst", 20)

	return config
}
