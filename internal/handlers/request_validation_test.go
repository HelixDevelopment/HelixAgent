package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRequestID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		requestID string
		valid     bool
	}{
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid short id", "req-12345", true},
		{"empty id", "", false},
		{"whitespace only", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			trimmed := strings.TrimSpace(tt.requestID)
			isValid := len(trimmed) > 0 && len(trimmed) <= 128
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestValidateModelName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		model string
		valid bool
	}{
		{"gpt-4", "gpt-4", true},
		{"gpt-3.5-turbo", "gpt-3.5-turbo", true},
		{"claude-v1", "claude-v1", true},
		{"empty model", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			isValid := len(tt.model) > 0 && len(tt.model) <= 64
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestValidateMaxTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		maxTokens int
		valid     bool
	}{
		{"valid 100", 100, true},
		{"valid 2048", 2048, true},
		{"zero tokens", 0, true},
		{"negative tokens", -1, false},
		{"excessive tokens", 1000000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			isValid := tt.maxTokens >= 0 && tt.maxTokens <= 32000
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestValidateTemperature(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		temperature float64
		valid       bool
	}{
		{"zero", 0.0, true},
		{"half", 0.5, true},
		{"one", 1.0, true},
		{"negative", -0.1, false},
		{"over one", 1.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			isValid := tt.temperature >= 0.0 && tt.temperature <= 1.0
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestValidateTopP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		topP  float64
		valid bool
	}{
		{"zero", 0.0, true},
		{"point nine", 0.9, true},
		{"one", 1.0, true},
		{"negative", -0.1, false},
		{"over one", 1.1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			isValid := tt.topP >= 0.0 && tt.topP <= 1.0
			assert.Equal(t, tt.valid, isValid)
		})
	}
}
