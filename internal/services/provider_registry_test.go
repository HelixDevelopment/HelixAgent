package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/models"
)

// MockLLMProviderForRegistry implements llm.LLMProvider for testing
type MockLLMProviderForRegistry struct {
	name           string
	completeFunc   func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	healthCheckErr error
	capabilities   *models.ProviderCapabilities
}

func (m *MockLLMProviderForRegistry) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return &models.LLMResponse{
		Content:      "test response",
		ProviderName: m.name,
		Confidence:   0.9,
	}, nil
}

func (m *MockLLMProviderForRegistry) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	ch <- &models.LLMResponse{Content: "stream response"}
	close(ch)
	return ch, nil
}

func (m *MockLLMProviderForRegistry) HealthCheck() error {
	return m.healthCheckErr
}

func (m *MockLLMProviderForRegistry) GetCapabilities() *models.ProviderCapabilities {
	if m.capabilities != nil {
		return m.capabilities
	}
	return &models.ProviderCapabilities{
		SupportsStreaming: true,
		SupportedFeatures: []string{"text"},
		SupportedModels:   []string{"test-model"},
	}
}

func (m *MockLLMProviderForRegistry) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

func TestGetDefaultRegistryConfig(t *testing.T) {
	cfg := getDefaultRegistryConfig()

	require.NotNil(t, cfg)
	assert.Equal(t, 30*time.Second, cfg.DefaultTimeout)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.True(t, cfg.HealthCheck.Enabled)
	assert.True(t, cfg.CircuitBreaker.Enabled)
	assert.NotNil(t, cfg.Providers)
	assert.NotNil(t, cfg.Ensemble)
	assert.NotNil(t, cfg.Routing)
}

func TestNewProviderRegistry(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		registry := NewProviderRegistry(nil, nil)

		require.NotNil(t, registry)
		assert.NotNil(t, registry.providers)
		assert.NotNil(t, registry.circuitBreakers)
		assert.NotNil(t, registry.config)
		assert.NotNil(t, registry.ensemble)
		assert.NotNil(t, registry.requestService)
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &RegistryConfig{
			DefaultTimeout: 60 * time.Second,
			MaxRetries:     5,
			HealthCheck: HealthCheckConfig{
				Enabled:          true,
				Interval:         30 * time.Second,
				Timeout:          5 * time.Second,
				FailureThreshold: 2,
			},
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 3,
				RecoveryTimeout:  30 * time.Second,
				SuccessThreshold: 1,
			},
			Providers: make(map[string]*ProviderConfig),
			Ensemble: &models.EnsembleConfig{
				Strategy: "majority_vote",
			},
			Routing: &RoutingConfig{
				Strategy: "round_robin",
			},
		}

		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

		require.NotNil(t, registry)
		assert.Equal(t, 60*time.Second, registry.config.DefaultTimeout)
		assert.Equal(t, 5, registry.config.MaxRetries)
	})
}

func TestProviderRegistry_RegisterProvider(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			RecoveryTimeout:  60 * time.Second,
			SuccessThreshold: 2,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("register new provider", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "test-provider"}
		err := registry.RegisterProvider("test-provider", provider)

		assert.NoError(t, err)

		registered, err := registry.GetProvider("test-provider")
		assert.NoError(t, err)
		assert.NotNil(t, registered)
	})

	t.Run("register duplicate provider fails", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "duplicate-provider"}
		err := registry.RegisterProvider("duplicate-provider", provider)
		assert.NoError(t, err)

		err = registry.RegisterProvider("duplicate-provider", provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("register provider with circuit breaker", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "cb-provider"}
		err := registry.RegisterProvider("cb-provider", provider)
		assert.NoError(t, err)

		// Circuit breaker should be created
		cb := registry.GetCircuitBreaker("cb-provider")
		assert.NotNil(t, cb)
	})
}

func TestProviderRegistry_UnregisterProvider(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("unregister existing provider", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "unregister-provider"}
		_ = registry.RegisterProvider("unregister-provider", provider)

		err := registry.UnregisterProvider("unregister-provider")
		assert.NoError(t, err)

		_, err = registry.GetProvider("unregister-provider")
		assert.Error(t, err)
	})

	t.Run("unregister non-existent provider fails", func(t *testing.T) {
		err := registry.UnregisterProvider("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRegistry_GetProvider(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("get existing provider", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "get-provider"}
		_ = registry.RegisterProvider("get-provider", provider)

		result, err := registry.GetProvider("get-provider")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("get non-existent provider fails", func(t *testing.T) {
		result, err := registry.GetProvider("non-existent")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRegistry_ListProviders(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	// Use constructor without auto-discovery for predictable test behavior
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("empty registry", func(t *testing.T) {
		providers := registry.ListProviders()
		assert.Empty(t, providers)
	})

	t.Run("with registered providers", func(t *testing.T) {
		_ = registry.RegisterProvider("provider1", &MockLLMProviderForRegistry{name: "provider1"})
		_ = registry.RegisterProvider("provider2", &MockLLMProviderForRegistry{name: "provider2"})

		providers := registry.ListProviders()
		assert.Len(t, providers, 2)
		assert.Contains(t, providers, "provider1")
		assert.Contains(t, providers, "provider2")
	})
}

func TestProviderRegistry_ConfigureProvider(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("configure non-existent provider fails", func(t *testing.T) {
		providerCfg := &ProviderConfig{
			Name:    "non-existent",
			Enabled: true,
		}
		err := registry.ConfigureProvider("non-existent", providerCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("disable provider via configure", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "disable-provider"}
		_ = registry.RegisterProvider("disable-provider", provider)

		providerCfg := &ProviderConfig{
			Name:    "disable-provider",
			Enabled: false,
		}
		err := registry.ConfigureProvider("disable-provider", providerCfg)
		assert.NoError(t, err)

		// Provider should be unregistered
		_, err = registry.GetProvider("disable-provider")
		assert.Error(t, err)
	})
}

func TestProviderRegistry_GetProviderConfig(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("get config for existing provider", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "config-provider"}
		_ = registry.RegisterProvider("config-provider", provider)

		providerCfg, err := registry.GetProviderConfig("config-provider")
		assert.NoError(t, err)
		assert.NotNil(t, providerCfg)
		assert.Equal(t, "config-provider", providerCfg.Name)
		assert.True(t, providerCfg.Enabled)
	})

	t.Run("get config for non-existent provider fails", func(t *testing.T) {
		providerCfg, err := registry.GetProviderConfig("non-existent")
		assert.Error(t, err)
		assert.Nil(t, providerCfg)
	})
}

func TestProviderRegistry_HealthCheck(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}

	t.Run("empty registry health check", func(t *testing.T) {
		// Use constructor without auto-discovery for predictable test behavior
		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
		results := registry.HealthCheck()
		assert.Empty(t, results)
	})

	t.Run("healthy providers", func(t *testing.T) {
		// Use constructor without auto-discovery for predictable test behavior
		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
		_ = registry.RegisterProvider("healthy-provider", &MockLLMProviderForRegistry{
			name:           "healthy-provider",
			healthCheckErr: nil,
		})

		results := registry.HealthCheck()
		assert.Len(t, results, 1)
		assert.NoError(t, results["healthy-provider"])
	})

	t.Run("unhealthy provider", func(t *testing.T) {
		// Use constructor without auto-discovery for predictable test behavior
		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
		_ = registry.RegisterProvider("unhealthy-provider", &MockLLMProviderForRegistry{
			name:           "unhealthy-provider",
			healthCheckErr: errors.New("health check failed"),
		})

		results := registry.HealthCheck()
		assert.Len(t, results, 1)
		assert.Error(t, results["unhealthy-provider"])
	})
}

func TestProviderRegistry_GetEnsembleService(t *testing.T) {
	registry := NewProviderRegistry(nil, nil)

	ensemble := registry.GetEnsembleService()
	assert.NotNil(t, ensemble)
}

func TestProviderRegistry_GetRequestService(t *testing.T) {
	registry := NewProviderRegistry(nil, nil)

	requestService := registry.GetRequestService()
	assert.NotNil(t, requestService)
}

func TestCircuitBreakerProvider_Complete(t *testing.T) {
	provider := &MockLLMProviderForRegistry{
		name: "cb-test-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{Content: "success"}, nil
		},
	}

	cb := NewCircuitBreaker(5, 2, 60*time.Second)
	cbProvider := &circuitBreakerProvider{
		provider:       provider,
		circuitBreaker: cb,
		name:           "cb-test-provider",
	}

	t.Run("successful complete", func(t *testing.T) {
		resp, err := cbProvider.Complete(context.Background(), &models.LLMRequest{Prompt: "test"})
		assert.NoError(t, err)
		assert.Equal(t, "success", resp.Content)
	})

	t.Run("complete with failure", func(t *testing.T) {
		failingProvider := &MockLLMProviderForRegistry{
			completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return nil, errors.New("provider error")
			},
		}
		failingCBProvider := &circuitBreakerProvider{
			provider:       failingProvider,
			circuitBreaker: NewCircuitBreaker(5, 2, 60*time.Second),
			name:           "failing-provider",
		}

		resp, err := failingCBProvider.Complete(context.Background(), &models.LLMRequest{Prompt: "test"})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestCircuitBreakerProvider_CompleteStream(t *testing.T) {
	provider := &MockLLMProviderForRegistry{name: "stream-provider"}
	cb := NewCircuitBreaker(5, 2, 60*time.Second)
	cbProvider := &circuitBreakerProvider{
		provider:       provider,
		circuitBreaker: cb,
		name:           "stream-provider",
	}

	stream, err := cbProvider.CompleteStream(context.Background(), &models.LLMRequest{Prompt: "test"})
	assert.NoError(t, err)
	assert.NotNil(t, stream)
}

func TestCircuitBreakerProvider_HealthCheck(t *testing.T) {
	t.Run("healthy provider", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{healthCheckErr: nil}
		cb := NewCircuitBreaker(5, 2, 60*time.Second)
		cbProvider := &circuitBreakerProvider{
			provider:       provider,
			circuitBreaker: cb,
			name:           "healthy",
		}

		err := cbProvider.HealthCheck()
		assert.NoError(t, err)
	})

	t.Run("unhealthy provider", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{healthCheckErr: errors.New("unhealthy")}
		cb := NewCircuitBreaker(5, 2, 60*time.Second)
		cbProvider := &circuitBreakerProvider{
			provider:       provider,
			circuitBreaker: cb,
			name:           "unhealthy",
		}

		err := cbProvider.HealthCheck()
		assert.Error(t, err)
	})
}

func TestCircuitBreakerProvider_GetCapabilities(t *testing.T) {
	caps := &models.ProviderCapabilities{
		SupportsStreaming: true,
		SupportedFeatures: []string{"text", "code"},
		SupportedModels:   []string{"model-1", "model-2"},
	}
	provider := &MockLLMProviderForRegistry{capabilities: caps}
	cb := NewCircuitBreaker(5, 2, 60*time.Second)
	cbProvider := &circuitBreakerProvider{
		provider:       provider,
		circuitBreaker: cb,
		name:           "caps-provider",
	}

	result := cbProvider.GetCapabilities()
	assert.NotNil(t, result)
	assert.Len(t, result.SupportedModels, 2)
	assert.True(t, result.SupportsStreaming)
}

func TestCircuitBreakerProvider_ValidateConfig(t *testing.T) {
	provider := &MockLLMProviderForRegistry{}
	cb := NewCircuitBreaker(5, 2, 60*time.Second)
	cbProvider := &circuitBreakerProvider{
		provider:       provider,
		circuitBreaker: cb,
		name:           "validate-provider",
	}

	valid, issues := cbProvider.ValidateConfig(map[string]interface{}{"key": "value"})
	assert.True(t, valid)
	assert.Empty(t, issues)
}

func TestProviderAdapter(t *testing.T) {
	mockProvider := &MockLLMProviderForRegistry{
		name: "adapter-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{Content: "adapter response"}, nil
		},
	}

	adapter := &providerAdapter{provider: mockProvider}

	t.Run("Complete", func(t *testing.T) {
		resp, err := adapter.Complete(context.Background(), &models.LLMRequest{Prompt: "test"})
		assert.NoError(t, err)
		assert.Equal(t, "adapter response", resp.Content)
	})

	t.Run("CompleteStream", func(t *testing.T) {
		stream, err := adapter.CompleteStream(context.Background(), &models.LLMRequest{Prompt: "test"})
		assert.NoError(t, err)
		assert.NotNil(t, stream)
	})
}

func TestProviderConfig_Structure(t *testing.T) {
	config := ProviderConfig{
		Name:           "test-provider",
		Type:           "openai",
		Enabled:        true,
		APIKey:         "test-key",
		BaseURL:        "https://api.example.com",
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		HealthCheckURL: "https://api.example.com/health",
		Weight:         1.0,
		Tags:           []string{"primary", "fast"},
		Capabilities:   map[string]string{"streaming": "true"},
		CustomSettings: map[string]any{"temperature": 0.7},
		Models: []ModelConfig{
			{
				ID:           "gpt-4",
				Name:         "GPT-4",
				Enabled:      true,
				Weight:       1.0,
				Capabilities: []string{"chat", "code"},
			},
		},
	}

	assert.Equal(t, "test-provider", config.Name)
	assert.True(t, config.Enabled)
	assert.Len(t, config.Models, 1)
	assert.Equal(t, "gpt-4", config.Models[0].ID)
}

func TestHealthCheckConfig_Structure(t *testing.T) {
	config := HealthCheckConfig{
		Enabled:          true,
		Interval:         60 * time.Second,
		Timeout:          10 * time.Second,
		FailureThreshold: 3,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 60*time.Second, config.Interval)
	assert.Equal(t, 3, config.FailureThreshold)
}

func TestRoutingConfig_Structure(t *testing.T) {
	config := RoutingConfig{
		Strategy: "round_robin",
		Weights: map[string]float64{
			"provider1": 1.0,
			"provider2": 0.5,
		},
	}

	assert.Equal(t, "round_robin", config.Strategy)
	assert.Equal(t, 1.0, config.Weights["provider1"])
}

func TestCircuitBreakerConfig_Structure(t *testing.T) {
	config := CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 5,
		RecoveryTimeout:  60 * time.Second,
		SuccessThreshold: 2,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 5, config.FailureThreshold)
	assert.Equal(t, 60*time.Second, config.RecoveryTimeout)
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Run("returns default when env not set", func(t *testing.T) {
		result := getEnvOrDefault("NONEXISTENT_VAR_12345", "default_value")
		assert.Equal(t, "default_value", result)
	})

	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_ENV_VAR_FOR_COVERAGE", "env-value")
		result := getEnvOrDefault("TEST_ENV_VAR_FOR_COVERAGE", "default-value")
		assert.Equal(t, "env-value", result)
	})

	t.Run("returns default when env is empty string", func(t *testing.T) {
		t.Setenv("TEST_EMPTY_ENV_VAR", "")
		result := getEnvOrDefault("TEST_EMPTY_ENV_VAR", "default-value")
		assert.Equal(t, "default-value", result)
	})
}

// Tests for ConfigureProvider config storage
func TestProviderRegistry_ConfigureProvider_StoresConfig(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("stores and retrieves configuration", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "config-storage-provider"}
		_ = registry.RegisterProvider("config-storage-provider", provider)

		// Configure provider with detailed config
		providerCfg := &ProviderConfig{
			Name:           "config-storage-provider",
			Type:           "test-type",
			Enabled:        true,
			APIKey:         "test-api-key",
			BaseURL:        "https://api.example.com",
			Timeout:        60 * time.Second,
			MaxRetries:     5,
			HealthCheckURL: "https://api.example.com/health",
			Weight:         1.5,
			Tags:           []string{"primary", "fast"},
			Capabilities:   map[string]string{"streaming": "true", "json_mode": "true"},
			CustomSettings: map[string]any{"temperature": 0.7, "max_tokens": 1000},
			Models: []ModelConfig{
				{
					ID:           "test-model-1",
					Name:         "Test Model 1",
					Enabled:      true,
					Weight:       1.0,
					Capabilities: []string{"chat", "completion"},
					CustomParams: map[string]any{"context_window": 128000},
				},
			},
		}
		err := registry.ConfigureProvider("config-storage-provider", providerCfg)
		assert.NoError(t, err)

		// Retrieve and verify the stored config
		retrievedCfg, err := registry.GetProviderConfig("config-storage-provider")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedCfg)

		// Verify all fields
		assert.Equal(t, "config-storage-provider", retrievedCfg.Name)
		assert.Equal(t, "test-type", retrievedCfg.Type)
		assert.True(t, retrievedCfg.Enabled)
		assert.Equal(t, "test-api-key", retrievedCfg.APIKey)
		assert.Equal(t, "https://api.example.com", retrievedCfg.BaseURL)
		assert.Equal(t, 60*time.Second, retrievedCfg.Timeout)
		assert.Equal(t, 5, retrievedCfg.MaxRetries)
		assert.Equal(t, 1.5, retrievedCfg.Weight)
		assert.Contains(t, retrievedCfg.Tags, "primary")
		assert.Contains(t, retrievedCfg.Tags, "fast")
		assert.Equal(t, "true", retrievedCfg.Capabilities["streaming"])
		assert.Equal(t, 0.7, retrievedCfg.CustomSettings["temperature"])

		// Verify models
		require.Len(t, retrievedCfg.Models, 1)
		assert.Equal(t, "test-model-1", retrievedCfg.Models[0].ID)
		assert.Equal(t, "Test Model 1", retrievedCfg.Models[0].Name)
		assert.Contains(t, retrievedCfg.Models[0].Capabilities, "chat")
	})

	t.Run("updates existing configuration", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "update-config-provider"}
		_ = registry.RegisterProvider("update-config-provider", provider)

		// Initial config
		initialCfg := &ProviderConfig{
			Name:    "update-config-provider",
			Enabled: true,
			Weight:  1.0,
			Tags:    []string{"initial"},
		}
		err := registry.ConfigureProvider("update-config-provider", initialCfg)
		assert.NoError(t, err)

		// Updated config
		updatedCfg := &ProviderConfig{
			Name:    "update-config-provider",
			Enabled: true,
			Weight:  2.0,
			Tags:    []string{"updated"},
		}
		err = registry.ConfigureProvider("update-config-provider", updatedCfg)
		assert.NoError(t, err)

		// Verify update
		retrievedCfg, err := registry.GetProviderConfig("update-config-provider")
		assert.NoError(t, err)
		assert.Equal(t, 2.0, retrievedCfg.Weight)
		assert.Contains(t, retrievedCfg.Tags, "updated")
		assert.NotContains(t, retrievedCfg.Tags, "initial")
	})

	t.Run("returned config is a copy", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "copy-test-provider"}
		_ = registry.RegisterProvider("copy-test-provider", provider)

		providerCfg := &ProviderConfig{
			Name:    "copy-test-provider",
			Enabled: true,
			Tags:    []string{"original"},
		}
		_ = registry.ConfigureProvider("copy-test-provider", providerCfg)

		// Modify the returned config
		retrievedCfg, _ := registry.GetProviderConfig("copy-test-provider")
		retrievedCfg.Tags = append(retrievedCfg.Tags, "modified")

		// Original should be unchanged
		secondRetrieve, _ := registry.GetProviderConfig("copy-test-provider")
		assert.NotContains(t, secondRetrieve.Tags, "modified")
	})
}

// Tests for RemoveProvider with request tracking
func TestProviderRegistry_RemoveProvider_RequestTracking(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}

	t.Run("force removes provider with active requests", func(t *testing.T) {
		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
		provider := &MockLLMProviderForRegistry{name: "force-remove-provider"}
		_ = registry.RegisterProvider("force-remove-provider", provider)

		// Simulate active requests
		registry.IncrementActiveRequests("force-remove-provider")
		registry.IncrementActiveRequests("force-remove-provider")

		// Force removal should succeed
		err := registry.RemoveProvider("force-remove-provider", true)
		assert.NoError(t, err)

		// Provider should be removed
		_, err = registry.GetProvider("force-remove-provider")
		assert.Error(t, err)
	})

	t.Run("removes provider with no active requests", func(t *testing.T) {
		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
		provider := &MockLLMProviderForRegistry{name: "no-requests-provider"}
		_ = registry.RegisterProvider("no-requests-provider", provider)

		// No active requests
		err := registry.RemoveProvider("no-requests-provider", false)
		assert.NoError(t, err)

		// Provider should be removed
		_, err = registry.GetProvider("no-requests-provider")
		assert.Error(t, err)
	})

	t.Run("fails to remove non-existent provider", func(t *testing.T) {
		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

		err := registry.RemoveProvider("non-existent-provider", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// Tests for active request tracking methods
func TestProviderRegistry_ActiveRequestTracking(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("increment and decrement active requests", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "tracking-provider"}
		_ = registry.RegisterProvider("tracking-provider", provider)

		// Initial count should be 0
		count := registry.GetActiveRequestCount("tracking-provider")
		assert.Equal(t, int64(0), count)

		// Increment
		ok := registry.IncrementActiveRequests("tracking-provider")
		assert.True(t, ok)
		count = registry.GetActiveRequestCount("tracking-provider")
		assert.Equal(t, int64(1), count)

		// Increment again
		registry.IncrementActiveRequests("tracking-provider")
		count = registry.GetActiveRequestCount("tracking-provider")
		assert.Equal(t, int64(2), count)

		// Decrement
		ok = registry.DecrementActiveRequests("tracking-provider")
		assert.True(t, ok)
		count = registry.GetActiveRequestCount("tracking-provider")
		assert.Equal(t, int64(1), count)

		// Decrement again
		registry.DecrementActiveRequests("tracking-provider")
		count = registry.GetActiveRequestCount("tracking-provider")
		assert.Equal(t, int64(0), count)
	})

	t.Run("operations on non-existent provider return false/-1", func(t *testing.T) {
		ok := registry.IncrementActiveRequests("non-existent")
		assert.False(t, ok)

		ok = registry.DecrementActiveRequests("non-existent")
		assert.False(t, ok)

		count := registry.GetActiveRequestCount("non-existent")
		assert.Equal(t, int64(-1), count)
	})
}

// Test graceful drain timeout
func TestProviderRegistry_DrainTimeout(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}

	t.Run("set and use custom drain timeout", func(t *testing.T) {
		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
		provider := &MockLLMProviderForRegistry{name: "drain-test-provider"}
		_ = registry.RegisterProvider("drain-test-provider", provider)

		// Set a very short drain timeout
		registry.SetDrainTimeout(200 * time.Millisecond)

		// Add active request
		registry.IncrementActiveRequests("drain-test-provider")

		// Start a goroutine to decrement after 100ms
		go func() {
			time.Sleep(100 * time.Millisecond)
			registry.DecrementActiveRequests("drain-test-provider")
		}()

		// Non-force removal should succeed after drain
		err := registry.RemoveProvider("drain-test-provider", false)
		assert.NoError(t, err)
	})

	t.Run("drain timeout fails when requests dont complete", func(t *testing.T) {
		registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
		provider := &MockLLMProviderForRegistry{name: "drain-fail-provider"}
		_ = registry.RegisterProvider("drain-fail-provider", provider)

		// Set a very short drain timeout
		registry.SetDrainTimeout(100 * time.Millisecond)

		// Add active request that won't complete
		registry.IncrementActiveRequests("drain-fail-provider")

		// Non-force removal should fail
		err := registry.RemoveProvider("drain-fail-provider", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "active requests")
	})
}

// Benchmarks
func BenchmarkProviderRegistry_GetProvider(b *testing.B) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
	_ = registry.RegisterProvider("bench-provider", &MockLLMProviderForRegistry{name: "bench-provider"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = registry.GetProvider("bench-provider")
	}
}

func BenchmarkProviderRegistry_ListProviders(b *testing.B) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
	for i := 0; i < 10; i++ {
		name := "bench-provider-" + string(rune(i))
		_ = registry.RegisterProvider(name, &MockLLMProviderForRegistry{name: name})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.ListProviders()
	}
}

// Tests for LoadRegistryConfigFromAppConfig
func TestLoadRegistryConfigFromAppConfig(t *testing.T) {
	t.Run("returns default config when app config is nil", func(t *testing.T) {
		cfg := LoadRegistryConfigFromAppConfig(nil)

		require.NotNil(t, cfg)
		assert.Equal(t, 30*time.Second, cfg.DefaultTimeout)
		assert.Equal(t, 3, cfg.MaxRetries)
		assert.NotNil(t, cfg.Providers)
	})
}

// Tests for RegisterProviderFromConfig
func TestProviderRegistry_RegisterProviderFromConfig(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("returns error for empty provider name", func(t *testing.T) {
		providerCfg := ProviderConfig{
			Name: "",
			Type: "claude",
		}
		err := registry.RegisterProviderFromConfig(providerCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider name is required")
	})

	t.Run("returns error for unsupported provider type", func(t *testing.T) {
		providerCfg := ProviderConfig{
			Name: "test-unsupported",
			Type: "unsupported-type",
		}
		err := registry.RegisterProviderFromConfig(providerCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported provider type")
	})
}

// Tests for UpdateProvider
func TestProviderRegistry_UpdateProvider(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("returns error for non-existent provider", func(t *testing.T) {
		updateCfg := ProviderConfig{
			Name:   "non-existent",
			APIKey: "new-key",
		}
		err := registry.UpdateProvider("non-existent", updateCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("updates existing provider config in registry", func(t *testing.T) {
		// Register a provider first
		provider := &MockLLMProviderForRegistry{name: "update-reg-test"}
		err := registry.RegisterProvider("update-reg-test", provider)
		require.NoError(t, err)

		// Store initial config in registry's config
		registry.mu.Lock()
		registry.config.Providers["update-reg-test"] = &ProviderConfig{
			Name:    "update-reg-test",
			Enabled: true,
			Weight:  1.0,
			APIKey:  "initial-key",
		}
		registry.mu.Unlock()

		// Update the config
		updateCfg := ProviderConfig{
			Name:    "update-reg-test",
			APIKey:  "updated-key",
			Weight:  2.0,
			Enabled: true,
		}
		err = registry.UpdateProvider("update-reg-test", updateCfg)
		assert.NoError(t, err)

		// UpdateProvider updates registry.config.Providers, not providerConfigs
		// Verify by accessing the internal state
		registry.mu.RLock()
		storedCfg := registry.config.Providers["update-reg-test"]
		registry.mu.RUnlock()
		require.NotNil(t, storedCfg)
		assert.Equal(t, "updated-key", storedCfg.APIKey)
		assert.Equal(t, 2.0, storedCfg.Weight)
	})
}

// Tests for RemoveProvider with drain
func TestProviderRegistry_RemoveProvider(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("removes existing provider with drain", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "remove-test"}
		err := registry.RegisterProvider("remove-test", provider)
		require.NoError(t, err)

		err = registry.RemoveProvider("remove-test", false)
		assert.NoError(t, err)

		// Provider should no longer exist
		_, err = registry.GetProvider("remove-test")
		assert.Error(t, err)
	})

	t.Run("removes existing provider with force", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "remove-force-test"}
		err := registry.RegisterProvider("remove-force-test", provider)
		require.NoError(t, err)

		err = registry.RemoveProvider("remove-force-test", true)
		assert.NoError(t, err)

		// Provider should no longer exist
		_, err = registry.GetProvider("remove-force-test")
		assert.Error(t, err)
	})

	t.Run("returns error for non-existent provider", func(t *testing.T) {
		err := registry.RemoveProvider("non-existent-for-remove", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// Tests for DrainProviderRequests
func TestProviderRegistry_DrainProviderRequests(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
	registry.SetDrainTimeout(100 * time.Millisecond)

	t.Run("drains with no active requests", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "drain-test"}
		err := registry.RegisterProvider("drain-test", provider)
		require.NoError(t, err)

		// Should complete quickly since no active requests
		err = registry.RemoveProvider("drain-test", false)
		assert.NoError(t, err)
	})

	t.Run("waits for active requests during drain", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "drain-active-test"}
		err := registry.RegisterProvider("drain-active-test", provider)
		require.NoError(t, err)

		// Increment active requests
		registry.IncrementActiveRequests("drain-active-test")

		// Start a goroutine to decrement after a short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			registry.DecrementActiveRequests("drain-active-test")
		}()

		// Removal should wait for drain
		start := time.Now()
		err = registry.RemoveProvider("drain-active-test", false)
		elapsed := time.Since(start)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
	})
}

// Tests for ActiveRequestCount
func TestProviderRegistry_ActiveRequestCount(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("returns -1 for non-existent provider", func(t *testing.T) {
		count := registry.GetActiveRequestCount("non-existent-provider")
		assert.Equal(t, int64(-1), count)
	})

	t.Run("increment and decrement work correctly", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "count-test"}
		_ = registry.RegisterProvider("count-test", provider)

		registry.IncrementActiveRequests("count-test")
		assert.Equal(t, int64(1), registry.GetActiveRequestCount("count-test"))

		registry.IncrementActiveRequests("count-test")
		assert.Equal(t, int64(2), registry.GetActiveRequestCount("count-test"))

		registry.DecrementActiveRequests("count-test")
		assert.Equal(t, int64(1), registry.GetActiveRequestCount("count-test"))

		registry.DecrementActiveRequests("count-test")
		assert.Equal(t, int64(0), registry.GetActiveRequestCount("count-test"))
	})

	t.Run("decrement on registered provider", func(t *testing.T) {
		provider := &MockLLMProviderForRegistry{name: "decrement-test"}
		_ = registry.RegisterProvider("decrement-test", provider)

		// Increment first, then decrement twice
		registry.IncrementActiveRequests("decrement-test")
		registry.DecrementActiveRequests("decrement-test")
		count := registry.GetActiveRequestCount("decrement-test")
		assert.Equal(t, int64(0), count)
	})
}

// Tests for SetDrainTimeout
func TestProviderRegistry_SetDrainTimeout(t *testing.T) {
	cfg := &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled: false,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
		},
	}
	registry := NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	t.Run("sets drain timeout", func(t *testing.T) {
		registry.SetDrainTimeout(5 * time.Second)
		// No direct way to verify, but should not panic
	})

	t.Run("zero timeout is accepted", func(t *testing.T) {
		registry.SetDrainTimeout(0)
		// Should not panic
	})
}

// Tests for getFirstModel helper
func TestGetFirstModel(t *testing.T) {
	t.Run("returns first model ID from list", func(t *testing.T) {
		models := []ModelConfig{
			{ID: "model-1"},
			{ID: "model-2"},
		}
		result := getFirstModel(models)
		assert.Equal(t, "model-1", result)
	})

	t.Run("returns empty string for empty list", func(t *testing.T) {
		result := getFirstModel([]ModelConfig{})
		assert.Equal(t, "", result)
	})

	t.Run("returns empty string for nil list", func(t *testing.T) {
		result := getFirstModel(nil)
		assert.Equal(t, "", result)
	})
}
