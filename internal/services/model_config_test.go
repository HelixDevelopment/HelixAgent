package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelConfig_Validation(t *testing.T) {
	tests := []struct {
		name         string
		modelID      string
		modelName    string
		enabled      bool
		weight       float64
		capabilities []string
	}{
		{
			name:         "valid model config",
			modelID:      "gpt-4",
			modelName:    "GPT-4",
			enabled:      true,
			weight:       1.0,
			capabilities: []string{"chat", "completion"},
		},
		{
			name:         "disabled model",
			modelID:      "gpt-3.5",
			modelName:    "GPT-3.5",
			enabled:      false,
			weight:       0.5,
			capabilities: []string{"chat"},
		},
		{
			name:         "zero weight",
			modelID:      "deprecated",
			modelName:    "Deprecated Model",
			enabled:      true,
			weight:       0.0,
			capabilities: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ModelConfig{
				ID:           tt.modelID,
				Name:         tt.modelName,
				Enabled:      tt.enabled,
				Weight:       tt.weight,
				Capabilities: tt.capabilities,
			}

			assert.Equal(t, tt.modelID, config.ID)
			assert.Equal(t, tt.modelName, config.Name)
			assert.Equal(t, tt.enabled, config.Enabled)
			assert.Equal(t, tt.weight, config.Weight)
			assert.Equal(t, tt.capabilities, config.Capabilities)
		})
	}
}

func TestModelConfig_CustomParams(t *testing.T) {
	t.Run("stores custom parameters", func(t *testing.T) {
		config := ModelConfig{
			ID:   "custom-model",
			Name: "Custom Model",
			CustomParams: map[string]any{
				"temperature": 0.7,
				"max_tokens":  2048,
				"top_p":       0.9,
			},
		}

		assert.NotNil(t, config.CustomParams)
		assert.Equal(t, 0.7, config.CustomParams["temperature"])
		assert.Equal(t, 2048, config.CustomParams["max_tokens"])
		assert.Equal(t, 0.9, config.CustomParams["top_p"])
	})
}
