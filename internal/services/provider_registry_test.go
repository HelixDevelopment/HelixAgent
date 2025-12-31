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
		SupportsStreaming:    true,
		SupportedFeatures:    []string{"text"},
		SupportedModels:      []string{"test-model"},
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

		registry := NewProviderRegistry(cfg, nil)

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
	registry := NewProviderRegistry(cfg, nil)

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
	registry := NewProviderRegistry(cfg, nil)

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
	registry := NewProviderRegistry(cfg, nil)

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
	registry := NewProviderRegistry(cfg, nil)

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
	registry := NewProviderRegistry(cfg, nil)

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
	registry := NewProviderRegistry(cfg, nil)

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
	registry := NewProviderRegistry(cfg, nil)

	t.Run("empty registry health check", func(t *testing.T) {
		results := registry.HealthCheck()
		assert.Empty(t, results)
	})

	t.Run("healthy providers", func(t *testing.T) {
		_ = registry.RegisterProvider("healthy-provider", &MockLLMProviderForRegistry{
			name:           "healthy-provider",
			healthCheckErr: nil,
		})

		results := registry.HealthCheck()
		assert.Len(t, results, 1)
		assert.NoError(t, results["healthy-provider"])
	})

	t.Run("unhealthy provider", func(t *testing.T) {
		_ = registry.RegisterProvider("unhealthy-provider", &MockLLMProviderForRegistry{
			name:           "unhealthy-provider",
			healthCheckErr: errors.New("health check failed"),
		})

		results := registry.HealthCheck()
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
		SupportsStreaming:    true,
		SupportedFeatures:    []string{"text", "code"},
		SupportedModels:      []string{"model-1", "model-2"},
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
	registry := NewProviderRegistry(cfg, nil)
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
	registry := NewProviderRegistry(cfg, nil)
	for i := 0; i < 10; i++ {
		name := "bench-provider-" + string(rune(i))
		_ = registry.RegisterProvider(name, &MockLLMProviderForRegistry{name: name})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.ListProviders()
	}
}
