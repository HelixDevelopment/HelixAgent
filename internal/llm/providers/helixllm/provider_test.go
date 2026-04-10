package helixllm

import (
	"crypto/tls"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider_TLSSecureByDefault(t *testing.T) {
	// Ensure env var does not interfere
	os.Unsetenv("HELIX_LLM_TLS_SKIP_VERIFY")
	defer os.Unsetenv("HELIX_LLM_TLS_SKIP_VERIFY")

	cfg := Config{
		Endpoint: "https://localhost:8443",
	}
	p := NewProvider(cfg)
	require.NotNil(t, p)

	transport, ok := p.httpClient.Transport.(*http.Transport)
	require.True(t, ok, "transport should be *http.Transport")
	require.NotNil(t, transport.TLSClientConfig)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify,
		"TLS verification must be enabled by default — InsecureSkipVerify should be false")
}

func TestNewProvider_TLSSkipVerifyExplicitOptIn(t *testing.T) {
	os.Unsetenv("HELIX_LLM_TLS_SKIP_VERIFY")
	defer os.Unsetenv("HELIX_LLM_TLS_SKIP_VERIFY")

	cfg := Config{
		Endpoint:      "https://localhost:8443",
		TLSSkipVerify: true,
	}
	p := NewProvider(cfg)
	require.NotNil(t, p)

	transport, ok := p.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify,
		"TLS skip verify should be enabled when explicitly configured")
}

func TestNewProvider_TLSSkipVerifyViaEnvVar(t *testing.T) {
	os.Setenv("HELIX_LLM_TLS_SKIP_VERIFY", "true")
	defer os.Unsetenv("HELIX_LLM_TLS_SKIP_VERIFY")

	cfg := Config{
		Endpoint: "https://localhost:8443",
	}
	p := NewProvider(cfg)
	require.NotNil(t, p)

	transport, ok := p.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify,
		"TLS skip verify should be enabled via HELIX_LLM_TLS_SKIP_VERIFY=true")
}

func TestNewProvider_TLSEnvVarFalseKeepsVerification(t *testing.T) {
	os.Setenv("HELIX_LLM_TLS_SKIP_VERIFY", "false")
	defer os.Unsetenv("HELIX_LLM_TLS_SKIP_VERIFY")

	cfg := Config{
		Endpoint: "https://localhost:8443",
	}
	p := NewProvider(cfg)
	require.NotNil(t, p)

	transport, ok := p.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify,
		"TLS verification should remain enabled when env var is false")
}

func TestNewProvider_DefaultEndpoint(t *testing.T) {
	os.Unsetenv("HELIX_LLM_ENDPOINT")
	defer os.Unsetenv("HELIX_LLM_ENDPOINT")

	p := NewProvider(Config{})
	assert.Equal(t, defaultEndpoint, p.endpoint)
}

func TestNewProvider_CustomEndpoint(t *testing.T) {
	cfg := Config{Endpoint: "https://custom:9443"}
	p := NewProvider(cfg)
	assert.Equal(t, "https://custom:9443", p.endpoint)
}

func TestNewProvider_DefaultModel(t *testing.T) {
	p := NewProvider(Config{})
	assert.Equal(t, defaultModel, p.model)
}

func TestNewProvider_DefaultTimeout(t *testing.T) {
	p := NewProvider(Config{})
	assert.Equal(t, defaultTimeout, p.timeout)
}

func TestNewProvider_CustomTimeout(t *testing.T) {
	cfg := Config{Timeout: 120 * time.Second}
	p := NewProvider(cfg)
	assert.Equal(t, 120*time.Second, p.timeout)
}

func TestNewProviderFromEnv(t *testing.T) {
	os.Setenv("HELIX_LLM_ENDPOINT", "https://env-endpoint:8443")
	os.Setenv("HELIX_LLM_API_KEY", "test-key-123")
	os.Setenv("HELIX_LLM_MODEL", "test-model")
	os.Setenv("HELIX_LLM_TLS_SKIP_VERIFY", "false")
	defer func() {
		os.Unsetenv("HELIX_LLM_ENDPOINT")
		os.Unsetenv("HELIX_LLM_API_KEY")
		os.Unsetenv("HELIX_LLM_MODEL")
		os.Unsetenv("HELIX_LLM_TLS_SKIP_VERIFY")
	}()

	p := NewProviderFromEnv()
	assert.Equal(t, "https://env-endpoint:8443", p.endpoint)
	assert.Equal(t, "test-key-123", p.apiKey)
	assert.Equal(t, "test-model", p.model)
}

func TestProvider_GetCapabilities(t *testing.T) {
	p := NewProvider(Config{})
	caps := p.GetCapabilities()
	require.NotNil(t, caps)
	assert.Equal(t, "helixllm", caps.Metadata["provider_name"])
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsReasoning)
	assert.True(t, caps.SupportsCodeCompletion)
	assert.True(t, caps.SupportsCodeAnalysis)
	assert.True(t, caps.SupportsRefactoring)
	assert.False(t, caps.SupportsVision)
	assert.False(t, caps.SupportsSearch)
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "embeddings")
	assert.Contains(t, caps.SupportedFeatures, "function_calling")
}

func TestProvider_ValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantValid bool
	}{
		{
			name:      "nil config is valid",
			config:    nil,
			wantValid: true,
		},
		{
			name:      "empty config is valid",
			config:    map[string]interface{}{},
			wantValid: true,
		},
		{
			name:      "config with empty endpoint is invalid",
			config:    map[string]interface{}{"endpoint": ""},
			wantValid: false,
		},
		{
			name:      "config with valid endpoint is valid",
			config:    map[string]interface{}{"endpoint": "https://localhost:8443"},
			wantValid: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider(Config{})
			valid, errs := p.ValidateConfig(tt.config)
			assert.Equal(t, tt.wantValid, valid)
			if !tt.wantValid {
				assert.NotEmpty(t, errs)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_HELIX_ENV", "custom-value")
	defer os.Unsetenv("TEST_HELIX_ENV")

	assert.Equal(t, "custom-value", getEnv("TEST_HELIX_ENV", "default"))
	assert.Equal(t, "default", getEnv("TEST_HELIX_ENV_MISSING", "default"))
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		envSet       bool
		defaultValue bool
		expected     bool
	}{
		{"unset returns default true", "", false, true, true},
		{"unset returns default false", "", false, false, false},
		{"true string", "true", true, false, true},
		{"1 string", "1", true, false, true},
		{"yes string", "yes", true, false, true},
		{"false string", "false", true, true, false},
		{"0 string", "0", true, true, false},
		{"no string", "no", true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				os.Setenv("TEST_BOOL_ENV", tt.envValue)
			} else {
				os.Unsetenv("TEST_BOOL_ENV")
			}
			defer os.Unsetenv("TEST_BOOL_ENV")
			result := getEnvBool("TEST_BOOL_ENV", tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Verify TLS config object is not nil and has expected type
func TestNewProvider_TLSConfigNotNil(t *testing.T) {
	p := NewProvider(Config{})
	transport, ok := p.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	assert.IsType(t, &tls.Config{}, transport.TLSClientConfig)
}
