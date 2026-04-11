package anthropic_cu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	cfg := Config{
		APIKey:    "test-key",
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
	}
	p := NewProvider(cfg)
	caps := p.GetCapabilities()
	assert.NotNil(t, caps)
}

func TestProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		config    Config
		wantValid bool
		wantErrs  int
	}{
		{
			name: "valid config with api_key",
			config: Config{
				APIKey:    "test-key",
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 4096,
			},
			wantValid: true,
			wantErrs:  0,
		},
		{
			name: "missing api_key",
			config: Config{
				APIKey:    "",
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 4096,
			},
			wantValid: false,
			wantErrs:  1,
		},
		{
			name: "missing model uses default",
			config: Config{
				APIKey:    "test-key",
				Model:     "",
				MaxTokens: 4096,
			},
			wantValid: true,
			wantErrs:  0,
		},
		{
			name:      "empty config fails",
			config:    Config{},
			wantValid: false,
			wantErrs:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := NewProvider(tt.config)
			valid, errs := p.ValidateConfig(nil)
			assert.Equal(t, tt.wantValid, valid)
			assert.Len(t, errs, tt.wantErrs)
		})
	}
}

func TestNewProvider(t *testing.T) {
	t.Parallel()
	cfg := Config{
		APIKey:    "test-key",
		Model:     "claude-3-opus-20240229",
		MaxTokens: 8192,
	}
	p := NewProvider(cfg)
	assert.NotNil(t, p)

	// Test default model
	cfg2 := Config{
		APIKey: "test-key",
	}
	p2 := NewProvider(cfg2)
	assert.NotNil(t, p2)
}

func TestProvider_HealthCheck(t *testing.T) {
	t.Parallel()
	cfg := Config{
		APIKey:    "test-key",
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
	}
	p := NewProvider(cfg)
	// Health check may fail if service is not accessible
	err := p.HealthCheck()
	_ = err
}
