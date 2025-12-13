package config

import (
	"os"
	"testing"
)

func TestConfig_GetString(t *testing.T) {
	c := Config{"key1": "value1", "key2": 123}

	// Test existing string key
	val, ok := c.GetString("key1")
	if !ok || val != "value1" {
		t.Errorf("Expected 'value1', true; got %s, %v", val, ok)
	}

	// Test non-existing key
	val, ok = c.GetString("key3")
	if ok || val != "" {
		t.Errorf("Expected '', false; got %s, %v", val, ok)
	}

	// Test non-string value
	val, ok = c.GetString("key2")
	if ok || val != "" {
		t.Errorf("Expected '', false; got %s, %v", val, ok)
	}
}

func TestConfig_GetStringWithDefault(t *testing.T) {
	c := Config{"key1": "value1"}

	// Test existing key
	result := c.GetStringWithDefault("key1", "default")
	if result != "value1" {
		t.Errorf("Expected 'value1', got %s", result)
	}

	// Test non-existing key
	result = c.GetStringWithDefault("key2", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got %s", result)
	}
}

func TestConfig_GetInt(t *testing.T) {
	c := Config{"int": 42, "int64": int64(100), "float": 3.14, "string": "123", "invalid": "abc"}

	tests := []struct {
		key      string
		expected int
		ok       bool
	}{
		{"int", 42, true},
		{"int64", 100, true},
		{"float", 3, true}, // float64 truncated to int
		{"string", 123, true},
		{"invalid", 0, false},
		{"missing", 0, false},
	}

	for _, test := range tests {
		val, ok := c.GetInt(test.key)
		if val != test.expected || ok != test.ok {
			t.Errorf("GetInt(%s): expected %d, %v; got %d, %v", test.key, test.expected, test.ok, val, ok)
		}
	}
}

func TestConfig_GetIntWithDefault(t *testing.T) {
	c := Config{"key1": 42}

	// Test existing key
	result := c.GetIntWithDefault("key1", 100)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	// Test non-existing key
	result = c.GetIntWithDefault("key2", 100)
	if result != 100 {
		t.Errorf("Expected 100, got %d", result)
	}
}

func TestConfig_GetBool(t *testing.T) {
	c := Config{"true": true, "false": false, "string_true": "true", "string_false": "false", "invalid": "maybe"}

	tests := []struct {
		key      string
		expected bool
		ok       bool
	}{
		{"true", true, true},
		{"false", false, true},
		{"string_true", true, true},
		{"string_false", false, true},
		{"invalid", false, false},
		{"missing", false, false},
	}

	for _, test := range tests {
		val, ok := c.GetBool(test.key)
		if val != test.expected || ok != test.ok {
			t.Errorf("GetBool(%s): expected %v, %v; got %v, %v", test.key, test.expected, test.ok, val, ok)
		}
	}
}

func TestConfig_GetBoolWithDefault(t *testing.T) {
	c := Config{"key1": true}

	// Test existing key
	result := c.GetBoolWithDefault("key1", false)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	// Test non-existing key
	result = c.GetBoolWithDefault("key2", true)
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestConfig_GetFloat(t *testing.T) {
	c := Config{"float": 3.14, "int": 42, "int64": int64(100), "string": "2.71", "invalid": "abc"}

	tests := []struct {
		key      string
		expected float64
		ok       bool
	}{
		{"float", 3.14, true},
		{"int", 42.0, true},
		{"int64", 100.0, true},
		{"string", 2.71, true},
		{"invalid", 0, false},
		{"missing", 0, false},
	}

	for _, test := range tests {
		val, ok := c.GetFloat(test.key)
		if val != test.expected || ok != test.ok {
			t.Errorf("GetFloat(%s): expected %f, %v; got %f, %v", test.key, test.expected, test.ok, val, ok)
		}
	}
}

func TestConfig_GetFloatWithDefault(t *testing.T) {
	c := Config{"key1": 3.14}

	// Test existing key
	result := c.GetFloatWithDefault("key1", 1.0)
	if result != 3.14 {
		t.Errorf("Expected 3.14, got %f", result)
	}

	// Test non-existing key
	result = c.GetFloatWithDefault("key2", 2.71)
	if result != 2.71 {
		t.Errorf("Expected 2.71, got %f", result)
	}
}

func TestConfig_Set(t *testing.T) {
	c := Config{}

	c.Set("key1", "value1")
	c.Set("key2", 42)

	if c["key1"] != "value1" {
		t.Errorf("Expected 'value1', got %v", c["key1"])
	}

	if c["key2"] != 42 {
		t.Errorf("Expected 42, got %v", c["key2"])
	}
}

func TestValidator_AddRule(t *testing.T) {
	v := NewValidator()

	rule := func(Config) error { return nil }
	v.AddRule("test", rule)

	if len(v.rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(v.rules))
	}

	if v.rules["test"] == nil {
		t.Error("Expected rule to be set")
	}
}

func TestValidator_Validate(t *testing.T) {
	v := NewValidator()

	// Add a passing rule
	v.AddRule("pass", func(Config) error { return nil })

	// Add a failing rule
	v.AddRule("fail", func(Config) error { return ValidationError{"test error"} })

	config := Config{}

	err := v.Validate(config)
	if err == nil {
		t.Error("Expected validation error")
	}

	if err.Error() != "validation failed for fail: test error" {
		t.Errorf("Expected specific error message, got %s", err.Error())
	}
}

// ValidationError is a mock error for testing
type ValidationError struct {
	message string
}

func (e ValidationError) Error() string {
	return e.message
}

func TestRequired(t *testing.T) {
	rule := Required("test_key")

	// Test missing key
	config := Config{}
	err := rule(config)
	if err == nil || err.Error() != "required field test_key is missing" {
		t.Errorf("Expected missing field error, got %v", err)
	}

	// Test empty string
	config = Config{"test_key": ""}
	err = rule(config)
	if err == nil || err.Error() != "required field test_key is empty" {
		t.Errorf("Expected empty field error, got %v", err)
	}

	// Test valid value
	config = Config{"test_key": "value"}
	err = rule(config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestOneOf(t *testing.T) {
	rule := OneOf("test_key", "option1", "option2")

	// Test valid value
	config := Config{"test_key": "option1"}
	err := rule(config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test invalid value
	config = Config{"test_key": "invalid"}
	err = rule(config)
	if err == nil {
		t.Error("Expected validation error")
	}
}

func TestMinLength(t *testing.T) {
	rule := MinLength("test_key", 3)

	// Test valid length
	config := Config{"test_key": "valid"}
	err := rule(config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test too short
	config = Config{"test_key": "ab"}
	err = rule(config)
	if err == nil {
		t.Error("Expected validation error")
	}
}

func TestConfig_LoadFromEnv(t *testing.T) {
	// Set up environment variables
	os.Setenv("TEST_PREFIX_KEY1", "value1")
	os.Setenv("TEST_PREFIX_KEY2", "42")
	os.Setenv("TEST_PREFIX_KEY3", "3.14")
	os.Setenv("TEST_PREFIX_KEY4", "true")
	os.Setenv("OTHER_VAR", "ignored")
	defer func() {
		os.Unsetenv("TEST_PREFIX_KEY1")
		os.Unsetenv("TEST_PREFIX_KEY2")
		os.Unsetenv("TEST_PREFIX_KEY3")
		os.Unsetenv("TEST_PREFIX_KEY4")
		os.Unsetenv("OTHER_VAR")
	}()

	c := Config{}
	c.LoadFromEnv("TEST_PREFIX_")

	if c["key1"] != "value1" {
		t.Errorf("Expected 'value1', got %v", c["key1"])
	}

	if c["key2"] != 42 {
		t.Errorf("Expected 42, got %v", c["key2"])
	}

	if c["key3"] != 3.14 {
		t.Errorf("Expected 3.14, got %v", c["key3"])
	}

	if c["key4"] != true {
		t.Errorf("Expected true, got %v", c["key4"])
	}

	if c["other"] != nil {
		t.Errorf("Expected other var to be ignored, got %v", c["other"])
	}
}

func TestProviderConfig_Validate(t *testing.T) {
	// Test valid config
	pc := &ProviderConfig{APIKey: "test-key"}
	err := pc.Validate()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test missing API key
	pc = &ProviderConfig{}
	err = pc.Validate()
	if err == nil {
		t.Error("Expected validation error for missing API key")
	}

	// Test empty API key
	pc = &ProviderConfig{APIKey: ""}
	err = pc.Validate()
	if err == nil {
		t.Error("Expected validation error for empty API key")
	}
}

func TestLoadProviderConfigFromEnv(t *testing.T) {
	// Set up environment variables
	os.Setenv("PROVIDER_API_KEY", "test-key")
	os.Setenv("PROVIDER_BASE_URL", "http://example.com")
	os.Setenv("PROVIDER_TIMEOUT", "60")
	os.Setenv("PROVIDER_MAX_RETRIES", "5")
	os.Setenv("PROVIDER_RATE_LIMIT", "20")
	os.Setenv("PROVIDER_RATE_LIMIT_BURST", "40")
	defer func() {
		os.Unsetenv("PROVIDER_API_KEY")
		os.Unsetenv("PROVIDER_BASE_URL")
		os.Unsetenv("PROVIDER_TIMEOUT")
		os.Unsetenv("PROVIDER_MAX_RETRIES")
		os.Unsetenv("PROVIDER_RATE_LIMIT")
		os.Unsetenv("PROVIDER_RATE_LIMIT_BURST")
	}()

	config := LoadProviderConfigFromEnv("PROVIDER_")

	if config.APIKey != "test-key" {
		t.Errorf("Expected 'test-key', got %s", config.APIKey)
	}

	if config.BaseURL != "http://example.com" {
		t.Errorf("Expected 'http://example.com', got %s", config.BaseURL)
	}

	if config.Timeout != 60 {
		t.Errorf("Expected 60, got %d", config.Timeout)
	}

	if config.MaxRetries != 5 {
		t.Errorf("Expected 5, got %d", config.MaxRetries)
	}

	if config.RateLimit != 20 {
		t.Errorf("Expected 20, got %d", config.RateLimit)
	}

	if config.RateLimitBurst != 40 {
		t.Errorf("Expected 40, got %d", config.RateLimitBurst)
	}
}
