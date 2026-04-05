package gemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==============================================================================
// CLI Provider Tests
// ==============================================================================

func TestIsGeminiCLIInstalled(t *testing.T) {
	t.Parallel()
	installed := IsGeminiCLIInstalled()
	t.Logf("Gemini CLI installed: %v", installed)
}

func TestGeminiCLIProvider_Basics(t *testing.T) {
	t.Parallel()
	config := DefaultGeminiCLIConfig()
	p := NewGeminiCLIProvider(config)
	assert.NotNil(t, p)
	assert.Equal(t, "gemini-cli", p.GetName())
	assert.Equal(t, "gemini", p.GetProviderType())
}

func TestGeminiCLIProvider_KnownModels(t *testing.T) {
	t.Parallel()
	models := GetKnownGeminiCLIModels()
	assert.GreaterOrEqual(t, len(models), 7)
	assert.Contains(t, models, "gemini-2.5-pro")
	assert.Contains(t, models, "gemini-2.5-flash")
	assert.Contains(t, models, "gemini-3-pro-preview")
}

func TestGeminiCLIProvider_SessionTracking(t *testing.T) {
	t.Parallel()
	t.Run("has sessionID field", func(t *testing.T) {
		config := DefaultGeminiCLIConfig()
		p := NewGeminiCLIProvider(config)
		// sessionID starts empty
		assert.Equal(t, "", p.sessionID,
			"sessionID should start empty")
	})

	t.Run("SetModel works correctly", func(t *testing.T) {
			t.Parallel()
		config := DefaultGeminiCLIConfig()
		config.Model = "gemini-2.5-flash"
		p := NewGeminiCLIProvider(config)

		assert.Equal(t, "gemini-2.5-flash", p.GetCurrentModel())

		p.SetModel("gemini-2.5-pro")
		assert.Equal(t, "gemini-2.5-pro", p.GetCurrentModel())

		p.SetModel("gemini-3-pro-preview")
		assert.Equal(t, "gemini-3-pro-preview", p.GetCurrentModel())
	})

	t.Run("GetBestAvailableModel returns reasonable default", func(t *testing.T) {
			t.Parallel()
		config := DefaultGeminiCLIConfig()
		p := NewGeminiCLIProvider(config)

		bestModel := p.GetBestAvailableModel()
		assert.NotEmpty(t, bestModel,
			"GetBestAvailableModel should return a non-empty model name")

		// Should be one of the known models or the hardcoded default
		knownModels := GetKnownGeminiCLIModels()
		found := false
		for _, m := range knownModels {
			if bestModel == m {
				found = true
				break
			}
		}
		// Also accept the hardcoded fallback
		if bestModel == "gemini-2.5-pro" {
			found = true
		}
		assert.True(t, found,
			"GetBestAvailableModel should return a known model, got: %s", bestModel)
	})

	t.Run("provider name and type are correct", func(t *testing.T) {
			t.Parallel()
		config := DefaultGeminiCLIConfig()
		p := NewGeminiCLIProvider(config)

		assert.Equal(t, "gemini-cli", p.GetName())
		assert.Equal(t, "gemini", p.GetProviderType())
	})
}
