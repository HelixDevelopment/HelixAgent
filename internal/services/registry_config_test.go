package services

import (
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestRegistryConfig_DefaultTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"30 seconds", 30 * time.Second},
		{"1 minute", 1 * time.Minute},
		{"5 minutes", 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RegistryConfig{
				DefaultTimeout: tt.timeout,
			}
			assert.Equal(t, tt.timeout, config.DefaultTimeout)
		})
	}
}

func TestRegistryConfig_MaxRetries(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
	}{
		{"zero retries", 0},
		{"three retries", 3},
		{"five retries", 5},
		{"ten retries", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RegistryConfig{
				MaxRetries: tt.maxRetries,
			}
			assert.Equal(t, tt.maxRetries, config.MaxRetries)
		})
	}
}

func TestRegistryConfig_Ensemble(t *testing.T) {
	t.Run("configures ensemble", func(t *testing.T) {
		ensembleConfig := &models.EnsembleConfig{
			Strategy:            "confidence_weighted",
			MinProviders:        2,
			ConfidenceThreshold: 0.7,
		}

		config := &RegistryConfig{
			Ensemble: ensembleConfig,
		}

		assert.NotNil(t, config.Ensemble)
		assert.Equal(t, "confidence_weighted", config.Ensemble.Strategy)
		assert.Equal(t, 2, config.Ensemble.MinProviders)
		assert.Equal(t, 0.7, config.Ensemble.ConfidenceThreshold)
	})
}

func TestRegistryConfig_Routing(t *testing.T) {
	t.Run("configures routing", func(t *testing.T) {
		routingConfig := &RoutingConfig{
			Strategy: "weighted",
			Weights: map[string]float64{
				"provider-1": 1.0,
				"provider-2": 2.0,
				"provider-3": 0.5,
			},
		}

		config := &RegistryConfig{
			Routing: routingConfig,
		}

		assert.NotNil(t, config.Routing)
		assert.Equal(t, "weighted", config.Routing.Strategy)
		assert.Equal(t, 1.0, config.Routing.Weights["provider-1"])
		assert.Equal(t, 2.0, config.Routing.Weights["provider-2"])
		assert.Equal(t, 0.5, config.Routing.Weights["provider-3"])
	})
}

func TestRegistryConfig_Providers(t *testing.T) {
	t.Run("stores provider configurations", func(t *testing.T) {
		providers := map[string]*ProviderConfig{
			"openai": {
				Name:    "openai",
				Type:    "openai",
				Enabled: true,
			},
			"anthropic": {
				Name:    "anthropic",
				Type:    "anthropic",
				Enabled: true,
			},
		}

		config := &RegistryConfig{
			Providers: providers,
		}

		assert.Len(t, config.Providers, 2)
		assert.NotNil(t, config.Providers["openai"])
		assert.NotNil(t, config.Providers["anthropic"])
	})
}

func TestRegistryConfig_DisableAutoDiscovery(t *testing.T) {
	tests := []struct {
		name                 string
		disableAutoDiscovery bool
	}{
		{"auto discovery enabled", false},
		{"auto discovery disabled", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RegistryConfig{
				DisableAutoDiscovery: tt.disableAutoDiscovery,
			}
			assert.Equal(t, tt.disableAutoDiscovery, config.DisableAutoDiscovery)
		})
	}
}
