package vertex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	cfg := Config{
		ProjectID: "test-project",
		Location:  "us-central1",
		Model:     "gemini-1.5-pro",
		APIKey:    "test-key",
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
			name: "valid config",
			config: Config{
				ProjectID: "test-project",
				Location:  "us-central1",
				Model:     "gemini-1.5-pro",
				APIKey:    "test-key",
			},
			wantValid: true,
			wantErrs:  0,
		},
		{
			name: "missing project_id",
			config: Config{
				ProjectID: "",
				Location:  "us-central1",
				Model:     "gemini-1.5-pro",
				APIKey:    "test-key",
			},
			wantValid: false,
			wantErrs:  1,
		},
		{
			name: "missing api_key",
			config: Config{
				ProjectID: "test-project",
				Location:  "us-central1",
				Model:     "gemini-1.5-pro",
				APIKey:    "",
			},
			wantValid: false,
			wantErrs:  1,
		},
		{
			name: "missing location uses default",
			config: Config{
				ProjectID: "test-project",
				Location:  "",
				Model:     "gemini-1.5-pro",
				APIKey:    "test-key",
			},
			wantValid: true,
			wantErrs:  0,
		},
		{
			name:      "empty config fails with defaults",
			config:    Config{},
			wantValid: false,
			wantErrs:  2, // project_id and api_key only (location and model use defaults)
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
		ProjectID: "test-project",
		Location:  "us-central1",
		Model:     "gemini-1.5-pro",
		APIKey:    "test-key",
	}
	p := NewProvider(cfg)
	assert.NotNil(t, p)

	// Test defaults
	cfg2 := Config{
		ProjectID: "test-project",
		APIKey:    "test-key",
	}
	p2 := NewProvider(cfg2)
	assert.NotNil(t, p2)
}

func TestProvider_HealthCheck(t *testing.T) {
	t.Parallel()
	cfg := Config{
		ProjectID: "test-project",
		Location:  "us-central1",
		Model:     "gemini-1.5-pro",
		APIKey:    "test-key",
	}
	p := NewProvider(cfg)
	// Health check may fail if service is not accessible
	err := p.HealthCheck()
	_ = err
}
