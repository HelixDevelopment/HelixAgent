package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	p := &Provider{}
	caps := p.GetCapabilities()
	assert.NotNil(t, caps)
}

func TestProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantValid bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"api_key":    "test-key",
				"endpoint":   "https://test.openai.azure.com",
				"deployment": "gpt-4",
			},
			wantValid: true,
		},
		{
			name: "missing api_key",
			config: map[string]interface{}{
				"endpoint":   "https://test.openai.azure.com",
				"deployment": "gpt-4",
			},
			wantValid: false,
		},
		{
			name:      "empty config",
			config:    map[string]interface{}{},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Provider{}
			valid, errors := p.ValidateConfig(tt.config)
			assert.Equal(t, tt.wantValid, valid)
			if tt.wantValid {
				assert.Empty(t, errors)
			} else {
				assert.NotEmpty(t, errors)
			}
		})
	}
}

func TestNewProvider(t *testing.T) {
	t.Parallel()
	p := NewProvider(
		"https://test.openai.azure.com",
		"gpt-4",
		"test-api-key",
	)
	assert.NotNil(t, p)
}

func TestProvider_HealthCheck(t *testing.T) {
	t.Parallel()
	p := NewProvider(
		"https://test.openai.azure.com",
		"gpt-4",
		"test-api-key",
	)

	// Health check may fail if Azure is not accessible
	// This test just verifies the method exists and doesn't panic
	err := p.HealthCheck()
	_ = err
}
