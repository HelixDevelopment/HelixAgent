package gemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==============================================================================
// ACP Provider Tests
// ==============================================================================

func TestGeminiACPProvider_Basics(t *testing.T) {
	t.Parallel()
	config := DefaultGeminiACPConfig()
	p := NewGeminiACPProvider(config)
	assert.NotNil(t, p)
	assert.Equal(t, "gemini-acp", p.GetName())
	assert.Equal(t, "gemini", p.GetProviderType())
}

func TestGeminiACPProvider_IsAvailable(t *testing.T) {
	t.Parallel()
	available := IsGeminiACPAvailable()
	t.Logf("Gemini ACP available: %v", available)
}
