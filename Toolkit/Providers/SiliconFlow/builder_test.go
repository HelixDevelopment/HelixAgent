package siliconflow

import (
	"testing"
)

func TestNewConfigBuilder(t *testing.T) {
	builder := NewConfigBuilder()

	if builder == nil {
		t.Error("Expected non-nil builder")
	}
}

func TestConfigBuilder_Build(t *testing.T) {
	builder := NewConfigBuilder()

	config := map[string]interface{}{
		"api_key":    "test-key",
		"base_url":   "https://custom.api.com",
		"timeout":    60000,
		"retries":    5,
		"rate_limit": 100,
	}

	result, err := builder.Build(config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	sfConfig, ok := result.(*Config)
	if !ok {
		t.Fatalf("Expected *Config, got %T", result)
	}

	if sfConfig.APIKey != "test-key" {
		t.Errorf("Expected APIKey 'test-key', got %s", sfConfig.APIKey)
	}

	if sfConfig.BaseURL != "https://custom.api.com" {
		t.Errorf("Expected BaseURL 'https://custom.api.com', got %s", sfConfig.BaseURL)
	}

	if sfConfig.Timeout != 60000 {
		t.Errorf("Expected Timeout 60000, got %d", sfConfig.Timeout)
	}

	if sfConfig.Retries != 5 {
		t.Errorf("Expected Retries 5, got %d", sfConfig.Retries)
	}

	if sfConfig.RateLimit != 100 {
		t.Errorf("Expected RateLimit 100, got %d", sfConfig.RateLimit)
	}
}

func TestConfigBuilder_Build_Defaults(t *testing.T) {
	builder := NewConfigBuilder()

	config := map[string]interface{}{
		"api_key": "test-key",
	}

	result, err := builder.Build(config)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	sfConfig := result.(*Config)

	if sfConfig.BaseURL != "https://api.siliconflow.com/v1" {
		t.Errorf("Expected default BaseURL, got %s", sfConfig.BaseURL)
	}

	if sfConfig.Timeout != 30000 {
		t.Errorf("Expected default Timeout 30000, got %d", sfConfig.Timeout)
	}

	if sfConfig.Retries != 3 {
		t.Errorf("Expected default Retries 3, got %d", sfConfig.Retries)
	}

	if sfConfig.RateLimit != 60 {
		t.Errorf("Expected default RateLimit 60, got %d", sfConfig.RateLimit)
	}
}

func TestConfigBuilder_Build_MissingAPIKey(t *testing.T) {
	builder := NewConfigBuilder()

	config := map[string]interface{}{
		"base_url": "https://api.example.com",
	}

	_, err := builder.Build(config)

	if err == nil {
		t.Error("Expected error for missing API key")
	}

	if err.Error() != "api_key is required" {
		t.Errorf("Expected 'api_key is required', got %s", err.Error())
	}
}

func TestConfigBuilder_Validate(t *testing.T) {
	builder := NewConfigBuilder()

	// Test valid config
	validConfig := &Config{APIKey: "test-key"}
	err := builder.Validate(validConfig)

	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}

	// Test missing API key
	invalidConfig := &Config{}
	err = builder.Validate(invalidConfig)

	if err == nil {
		t.Error("Expected error for missing API key")
	}

	// Test invalid type
	err = builder.Validate("invalid")

	if err == nil {
		t.Error("Expected error for invalid type")
	}

	if err.Error() != "invalid config type" {
		t.Errorf("Expected 'invalid config type', got %s", err.Error())
	}
}

func TestConfigBuilder_Merge(t *testing.T) {
	builder := NewConfigBuilder()

	base := &Config{
		APIKey:    "base-key",
		BaseURL:   "https://base.api.com",
		Timeout:   30000,
		Retries:   3,
		RateLimit: 60,
	}

	override := &Config{
		APIKey:    "override-key",
		BaseURL:   "https://override.api.com",
		Timeout:   60000,
		Retries:   5,
		RateLimit: 100,
	}

	result, err := builder.Merge(base, override)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	merged := result.(*Config)

	if merged.APIKey != "override-key" {
		t.Errorf("Expected APIKey 'override-key', got %s", merged.APIKey)
	}

	if merged.BaseURL != "https://override.api.com" {
		t.Errorf("Expected BaseURL 'https://override.api.com', got %s", merged.BaseURL)
	}

	if merged.Timeout != 60000 {
		t.Errorf("Expected Timeout 60000, got %d", merged.Timeout)
	}

	if merged.Retries != 5 {
		t.Errorf("Expected Retries 5, got %d", merged.Retries)
	}

	if merged.RateLimit != 100 {
		t.Errorf("Expected RateLimit 100, got %d", merged.RateLimit)
	}
}

func TestConfigBuilder_Merge_WithDefaults(t *testing.T) {
	builder := NewConfigBuilder()

	base := &Config{
		APIKey:    "base-key",
		BaseURL:   "https://base.api.com",
		Timeout:   30000,
		Retries:   3,
		RateLimit: 60,
	}

	override := &Config{} // Empty override

	result, err := builder.Merge(base, override)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	merged := result.(*Config)

	if merged.APIKey != "base-key" {
		t.Errorf("Expected APIKey 'base-key', got %s", merged.APIKey)
	}

	if merged.BaseURL != "https://base.api.com" {
		t.Errorf("Expected BaseURL 'https://base.api.com', got %s", merged.BaseURL)
	}
}

func TestConfigBuilder_Merge_InvalidTypes(t *testing.T) {
	builder := NewConfigBuilder()

	// Test invalid base type
	_, err := builder.Merge("invalid", &Config{})

	if err == nil {
		t.Error("Expected error for invalid base type")
	}

	// Test invalid override type
	_, err = builder.Merge(&Config{}, "invalid")

	if err == nil {
		t.Error("Expected error for invalid override type")
	}
}

func TestGetString(t *testing.T) {
	config := map[string]interface{}{
		"string_key": "value",
		"int_key":    42,
	}

	// Test existing string
	result := getString(config, "string_key", "default")
	if result != "value" {
		t.Errorf("Expected 'value', got %s", result)
	}

	// Test non-string value
	result = getString(config, "int_key", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got %s", result)
	}

	// Test missing key
	result = getString(config, "missing", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got %s", result)
	}
}

func TestGetInt(t *testing.T) {
	config := map[string]interface{}{
		"int_key":    42,
		"float_key":  3.14,
		"string_key": "not_a_number",
	}

	// Test int value
	result := getInt(config, "int_key", 100)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	// Test float value (converted to int)
	result = getInt(config, "float_key", 100)
	if result != 3 {
		t.Errorf("Expected 3, got %d", result)
	}

	// Test invalid value
	result = getInt(config, "string_key", 100)
	if result != 100 {
		t.Errorf("Expected 100, got %d", result)
	}

	// Test missing key
	result = getInt(config, "missing", 200)
	if result != 200 {
		t.Errorf("Expected 200, got %d", result)
	}
}
