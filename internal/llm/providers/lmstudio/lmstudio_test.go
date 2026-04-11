package lmstudio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:1234", "test-model")
	caps := p.GetCapabilities()
	assert.NotNil(t, caps)
}

func TestProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		baseURL   string
		model     string
		wantValid bool
		wantErrs  int
	}{
		{
			name:      "valid config",
			baseURL:   "http://localhost:1234",
			model:     "local-model",
			wantValid: true,
			wantErrs:  0,
		},
		{
			name:      "empty baseURL uses default",
			baseURL:   "",
			model:     "local-model",
			wantValid: true,
			wantErrs:  0,
		},
		{
			name:      "missing model",
			baseURL:   "http://localhost:1234",
			model:     "",
			wantValid: false,
			wantErrs:  1,
		},
		{
			name:      "missing both",
			baseURL:   "",
			model:     "",
			wantValid: false,
			wantErrs:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := NewProvider(tt.baseURL, tt.model)
			valid, errs := p.ValidateConfig(nil)
			assert.Equal(t, tt.wantValid, valid)
			assert.Len(t, errs, tt.wantErrs)
		})
	}
}

func TestNewProvider(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:1234", "test-model")
	assert.NotNil(t, p)

	// Test default baseURL
	p2 := NewProvider("", "test-model")
	assert.NotNil(t, p2)
}

func TestProvider_HealthCheck(t *testing.T) {
	t.Parallel()
	p := NewProvider("http://localhost:1234", "test-model")
	err := p.HealthCheck()
	_ = err
}
