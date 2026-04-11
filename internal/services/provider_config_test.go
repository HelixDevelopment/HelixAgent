package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProviderConfig_DefaultValues(t *testing.T) {
	t.Run("creates config with defaults", func(t *testing.T) {
		config := &ProviderConfig{
			Name:    "test-provider",
			Type:    "openai",
			Enabled: true,
			APIKey:  "test-key",
		}

		assert.Equal(t, "test-provider", config.Name)
		assert.Equal(t, "openai", config.Type)
		assert.True(t, config.Enabled)
		assert.Equal(t, "test-key", config.APIKey)
	})
}

func TestProviderConfig_TimeoutAndRetries(t *testing.T) {
	tests := []struct {
		name       string
		timeout    time.Duration
		maxRetries int
	}{
		{
			name:       "default timeout",
			timeout:    30 * time.Second,
			maxRetries: 3,
		},
		{
			name:       "extended timeout",
			timeout:    120 * time.Second,
			maxRetries: 5,
		},
		{
			name:       "quick timeout",
			timeout:    5 * time.Second,
			maxRetries: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ProviderConfig{
				Name:       "test",
				Timeout:    tt.timeout,
				MaxRetries: tt.maxRetries,
			}

			assert.Equal(t, tt.timeout, config.Timeout)
			assert.Equal(t, tt.maxRetries, config.MaxRetries)
		})
	}
}

func TestProviderConfig_WeightAndTags(t *testing.T) {
	t.Run("sets weight and tags", func(t *testing.T) {
		config := &ProviderConfig{
			Name:   "weighted-provider",
			Weight: 2.5,
			Tags:   []string{"production", "stable", "fast"},
		}

		assert.Equal(t, 2.5, config.Weight)
		assert.Equal(t, []string{"production", "stable", "fast"}, config.Tags)
		assert.Len(t, config.Tags, 3)
	})
}

func TestProviderConfig_Capabilities(t *testing.T) {
	t.Run("defines provider capabilities", func(t *testing.T) {
		config := &ProviderConfig{
			Name: "capable-provider",
			Capabilities: map[string]string{
				"chat":       "supported",
				"streaming":  "supported",
				"vision":     "unsupported",
				"embeddings": "supported",
			},
		}

		assert.Equal(t, "supported", config.Capabilities["chat"])
		assert.Equal(t, "supported", config.Capabilities["streaming"])
		assert.Equal(t, "unsupported", config.Capabilities["vision"])
		assert.Equal(t, "supported", config.Capabilities["embeddings"])
	})
}

func TestProviderConfig_CustomSettings(t *testing.T) {
	t.Run("stores custom settings", func(t *testing.T) {
		config := &ProviderConfig{
			Name: "custom-provider",
			CustomSettings: map[string]any{
				"organization": "acme-corp",
				"project":      "ai-initiative",
				"priority":     1,
				"features":     []string{"feature1", "feature2"},
			},
		}

		assert.NotNil(t, config.CustomSettings)
		assert.Equal(t, "acme-corp", config.CustomSettings["organization"])
		assert.Equal(t, "ai-initiative", config.CustomSettings["project"])
		assert.Equal(t, 1, config.CustomSettings["priority"])
	})
}
