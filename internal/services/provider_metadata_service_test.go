package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/database"
)

// MockModelMetadataService implements ModelMetadataServiceInterface for testing
type MockModelMetadataService struct {
	models         map[string][]*database.ModelMetadata
	refreshError   error
	refreshCalled  bool
	providerCalled string
}

func (m *MockModelMetadataService) GetModel(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
	for _, models := range m.models {
		for _, model := range models {
			if model.ModelID == modelID {
				return model, nil
			}
		}
	}
	return nil, nil
}

func (m *MockModelMetadataService) ListModels(ctx context.Context, providerID string, modelType string, page int, limit int) ([]*database.ModelMetadata, int, error) {
	return m.models[providerID], len(m.models[providerID]), nil
}

func (m *MockModelMetadataService) SearchModels(ctx context.Context, query string, page int, limit int) ([]*database.ModelMetadata, int, error) {
	var results []*database.ModelMetadata
	return results, 0, nil
}

func (m *MockModelMetadataService) RefreshModels(ctx context.Context) error {
	m.refreshCalled = true
	return m.refreshError
}

func (m *MockModelMetadataService) RefreshProviderModels(ctx context.Context, providerID string) error {
	m.refreshCalled = true
	m.providerCalled = providerID
	return m.refreshError
}

func (m *MockModelMetadataService) GetRefreshHistory(ctx context.Context, limit int) ([]*database.ModelsRefreshHistory, error) {
	return nil, nil
}

func (m *MockModelMetadataService) GetProviderModels(ctx context.Context, providerID string) ([]*database.ModelMetadata, error) {
	models, exists := m.models[providerID]
	if !exists {
		return []*database.ModelMetadata{}, nil
	}
	return models, nil
}

func (m *MockModelMetadataService) CompareModels(ctx context.Context, modelIDs []string) ([]*database.ModelMetadata, error) {
	return nil, nil
}

func (m *MockModelMetadataService) GetModelsByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error) {
	return nil, nil
}

// MockProviderRegistry implements ProviderRegistryInterface for testing
type MockProviderRegistry struct {
	providers        map[string]*ProviderConfig
	configureError   error
	configureCalled  bool
}

func (m *MockProviderRegistry) ListProviders() []string {
	var providers []string
	for name := range m.providers {
		providers = append(providers, name)
	}
	return providers
}

func (m *MockProviderRegistry) GetProviderConfig(name string) (*ProviderConfig, error) {
	config, exists := m.providers[name]
	if !exists {
		return nil, nil
	}
	return config, nil
}

func (m *MockProviderRegistry) ConfigureProvider(name string, config *ProviderConfig) error {
	m.configureCalled = true
	if m.configureError != nil {
		return m.configureError
	}
	m.providers[name] = config
	return nil
}

func newProviderMetadataTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewProviderMetadataService(t *testing.T) {
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	logger := newProviderMetadataTestLogger()

	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	require.NotNil(t, service)
	assert.NotNil(t, service.modelMetadataService)
	assert.NotNil(t, service.providerRegistry)
	assert.NotNil(t, service.log)
	assert.NotNil(t, service.providerToModels)
}

func TestProviderMetadataService_LoadProviderMetadata(t *testing.T) {
	logger := newProviderMetadataTestLogger()

	t.Run("load metadata for multiple providers", func(t *testing.T) {
		benchmarkScore := 85.0
		mockModelService := &MockModelMetadataService{
			models: map[string][]*database.ModelMetadata{
				"openai": {
					{ModelID: "gpt-4", ModelName: "GPT-4", BenchmarkScore: &benchmarkScore},
					{ModelID: "gpt-3.5-turbo", ModelName: "GPT-3.5 Turbo"},
				},
				"anthropic": {
					{ModelID: "claude-3-opus", ModelName: "Claude 3 Opus"},
				},
			},
		}
		mockRegistry := &MockProviderRegistry{
			providers: map[string]*ProviderConfig{
				"openai":    {Name: "openai", Type: "llm"},
				"anthropic": {Name: "anthropic", Type: "llm"},
			},
		}

		service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

		err := service.LoadProviderMetadata(context.Background())
		require.NoError(t, err)

		// Check that models were loaded
		assert.Len(t, service.providerToModels, 2)
	})

	t.Run("handles empty providers", func(t *testing.T) {
		mockModelService := &MockModelMetadataService{
			models: make(map[string][]*database.ModelMetadata),
		}
		mockRegistry := &MockProviderRegistry{
			providers: make(map[string]*ProviderConfig),
		}

		service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

		err := service.LoadProviderMetadata(context.Background())
		require.NoError(t, err)
		assert.Empty(t, service.providerToModels)
	})
}

func TestProviderMetadataService_UpdateProviderConfigs(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	benchmarkScore := 90.0

	mockModelService := &MockModelMetadataService{
		models: map[string][]*database.ModelMetadata{
			"openai": {
				{
					ModelID:                  "gpt-4",
					ModelName:                "GPT-4",
					BenchmarkScore:           &benchmarkScore,
					SupportsVision:           true,
					SupportsFunctionCalling:  true,
					SupportsStreaming:        true,
				},
			},
		},
	}
	mockRegistry := &MockProviderRegistry{
		providers: map[string]*ProviderConfig{
			"openai": {Name: "openai", Type: "llm", Enabled: true},
		},
	}

	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)
	service.providerToModels = mockModelService.models

	updatedConfigs, err := service.UpdateProviderConfigs(context.Background())
	require.NoError(t, err)
	assert.Len(t, updatedConfigs, 1)
	assert.Contains(t, updatedConfigs, "openai")
}

func TestProviderMetadataService_calculateModelWeight(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	t.Run("basic weight without scores", func(t *testing.T) {
		model := &database.ModelMetadata{ModelID: "test"}
		weight := service.calculateModelWeight(model)
		assert.Equal(t, 1.0, weight)
	})

	t.Run("weight with benchmark score", func(t *testing.T) {
		benchmarkScore := 80.0
		model := &database.ModelMetadata{
			ModelID:        "test",
			BenchmarkScore: &benchmarkScore,
		}
		weight := service.calculateModelWeight(model)
		assert.Greater(t, weight, 1.0)
	})

	t.Run("weight with all scores", func(t *testing.T) {
		benchmarkScore := 90.0
		popularityScore := 80
		reliabilityScore := 0.95
		model := &database.ModelMetadata{
			ModelID:          "test",
			BenchmarkScore:   &benchmarkScore,
			PopularityScore:  &popularityScore,
			ReliabilityScore: &reliabilityScore,
		}
		weight := service.calculateModelWeight(model)
		assert.Greater(t, weight, 1.5)
		assert.LessOrEqual(t, weight, 2.0) // Cap check
	})

	t.Run("weight capped at maximum", func(t *testing.T) {
		benchmarkScore := 100.0
		popularityScore := 100
		reliabilityScore := 1.0
		model := &database.ModelMetadata{
			ModelID:          "test",
			BenchmarkScore:   &benchmarkScore,
			PopularityScore:  &popularityScore,
			ReliabilityScore: &reliabilityScore,
		}
		weight := service.calculateModelWeight(model)
		assert.Equal(t, 2.0, weight) // Should be capped at 2.0
	})
}

func TestProviderMetadataService_extractCapabilities(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	t.Run("extracts vision capability", func(t *testing.T) {
		model := &database.ModelMetadata{SupportsVision: true}
		caps := service.extractCapabilities(model)
		assert.Contains(t, caps, "vision")
	})

	t.Run("extracts multiple capabilities", func(t *testing.T) {
		model := &database.ModelMetadata{
			SupportsVision:          true,
			SupportsFunctionCalling: true,
			SupportsStreaming:       true,
			SupportsJSONMode:        true,
			SupportsCodeGeneration:  true,
			SupportsReasoning:       true,
		}
		caps := service.extractCapabilities(model)
		assert.Contains(t, caps, "vision")
		assert.Contains(t, caps, "function_calling")
		assert.Contains(t, caps, "streaming")
		assert.Contains(t, caps, "json_mode")
		assert.Contains(t, caps, "code_generation")
		assert.Contains(t, caps, "reasoning")
	})

	t.Run("extracts model type as capability", func(t *testing.T) {
		modelType := "Chat"
		model := &database.ModelMetadata{ModelType: &modelType}
		caps := service.extractCapabilities(model)
		assert.Contains(t, caps, "chat")
	})

	t.Run("handles audio and image generation", func(t *testing.T) {
		model := &database.ModelMetadata{
			SupportsAudio:           true,
			SupportsImageGeneration: true,
		}
		caps := service.extractCapabilities(model)
		assert.Contains(t, caps, "audio")
		assert.Contains(t, caps, "image_generation")
	})
}

func TestProviderMetadataService_createCustomParams(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	t.Run("creates params with context window", func(t *testing.T) {
		contextWindow := 128000
		model := &database.ModelMetadata{ContextWindow: &contextWindow}
		params := service.createCustomParams(model)
		assert.Equal(t, 128000, params["max_tokens"])
	})

	t.Run("creates params with max tokens", func(t *testing.T) {
		maxTokens := 4096
		model := &database.ModelMetadata{MaxTokens: &maxTokens}
		params := service.createCustomParams(model)
		assert.Equal(t, 4096, params["max_completion_tokens"])
	})

	t.Run("creates params with pricing", func(t *testing.T) {
		pricingInput := 10.0
		pricingOutput := 30.0
		model := &database.ModelMetadata{
			PricingInput:    &pricingInput,
			PricingOutput:   &pricingOutput,
			PricingCurrency: "USD",
		}
		params := service.createCustomParams(model)
		assert.Equal(t, 10.0, params["pricing_input_per_million"])
		assert.Equal(t, 30.0, params["pricing_output_per_million"])
		assert.Equal(t, "USD", params["pricing_currency"])
	})

	t.Run("creates params with benchmark score", func(t *testing.T) {
		benchmarkScore := 85.5
		model := &database.ModelMetadata{BenchmarkScore: &benchmarkScore}
		params := service.createCustomParams(model)
		assert.Equal(t, 85.5, params["benchmark_score"])
	})

	t.Run("includes capability flags", func(t *testing.T) {
		model := &database.ModelMetadata{
			SupportsVision:           true,
			SupportsFunctionCalling:  true,
			SupportsStreaming:        false,
		}
		params := service.createCustomParams(model)
		assert.Equal(t, true, params["supports_vision"])
		assert.Equal(t, true, params["supports_function_calling"])
		assert.Equal(t, false, params["supports_streaming"])
	})
}

func TestProviderMetadataService_GetModelsForProvider(t *testing.T) {
	logger := newProviderMetadataTestLogger()

	t.Run("returns cached models", func(t *testing.T) {
		mockModelService := &MockModelMetadataService{
			models: map[string][]*database.ModelMetadata{
				"openai": {{ModelID: "gpt-4"}},
			},
		}
		mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}

		service := NewProviderMetadataService(mockModelService, mockRegistry, logger)
		service.providerToModels["openai"] = mockModelService.models["openai"]

		models, err := service.GetModelsForProvider(context.Background(), "openai")
		require.NoError(t, err)
		assert.Len(t, models, 1)
		assert.Equal(t, "gpt-4", models[0].ModelID)
	})

	t.Run("loads models if not cached", func(t *testing.T) {
		mockModelService := &MockModelMetadataService{
			models: map[string][]*database.ModelMetadata{
				"anthropic": {{ModelID: "claude-3"}},
			},
		}
		mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}

		service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

		models, err := service.GetModelsForProvider(context.Background(), "anthropic")
		require.NoError(t, err)
		assert.Len(t, models, 1)
	})
}

func TestProviderMetadataService_hasRequiredCapabilities(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	t.Run("has all required capabilities", func(t *testing.T) {
		model := &database.ModelMetadata{
			SupportsVision:          true,
			SupportsFunctionCalling: true,
		}
		result := service.hasRequiredCapabilities(model, []string{"vision", "function_calling"})
		assert.True(t, result)
	})

	t.Run("missing required capability", func(t *testing.T) {
		model := &database.ModelMetadata{
			SupportsVision: true,
		}
		result := service.hasRequiredCapabilities(model, []string{"vision", "function_calling"})
		assert.False(t, result)
	})

	t.Run("checks streaming capability", func(t *testing.T) {
		model := &database.ModelMetadata{SupportsStreaming: true}
		result := service.hasRequiredCapabilities(model, []string{"streaming"})
		assert.True(t, result)
	})

	t.Run("checks json_mode capability", func(t *testing.T) {
		model := &database.ModelMetadata{SupportsJSONMode: true}
		result := service.hasRequiredCapabilities(model, []string{"json_mode"})
		assert.True(t, result)
	})

	t.Run("checks image_generation capability", func(t *testing.T) {
		model := &database.ModelMetadata{SupportsImageGeneration: true}
		result := service.hasRequiredCapabilities(model, []string{"image_generation"})
		assert.True(t, result)
	})

	t.Run("checks audio capability", func(t *testing.T) {
		model := &database.ModelMetadata{SupportsAudio: true}
		result := service.hasRequiredCapabilities(model, []string{"audio"})
		assert.True(t, result)
	})

	t.Run("checks code_generation capability", func(t *testing.T) {
		model := &database.ModelMetadata{SupportsCodeGeneration: true}
		result := service.hasRequiredCapabilities(model, []string{"code_generation"})
		assert.True(t, result)
	})

	t.Run("checks reasoning capability", func(t *testing.T) {
		model := &database.ModelMetadata{SupportsReasoning: true}
		result := service.hasRequiredCapabilities(model, []string{"reasoning"})
		assert.True(t, result)
	})

	t.Run("checks capability in tags", func(t *testing.T) {
		model := &database.ModelMetadata{Tags: []string{"custom_tag", "special"}}
		result := service.hasRequiredCapabilities(model, []string{"custom_tag"})
		assert.True(t, result)
	})

	t.Run("empty requirements returns true", func(t *testing.T) {
		model := &database.ModelMetadata{}
		result := service.hasRequiredCapabilities(model, []string{})
		assert.True(t, result)
	})
}

func TestProviderMetadataService_selectBestModel(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	t.Run("selects model with highest score", func(t *testing.T) {
		lowScore := 50.0
		highScore := 90.0
		models := []*database.ModelMetadata{
			{ModelID: "low", BenchmarkScore: &lowScore},
			{ModelID: "high", BenchmarkScore: &highScore},
		}
		best := service.selectBestModel(models)
		assert.Equal(t, "high", best.ModelID)
	})

	t.Run("returns nil for empty list", func(t *testing.T) {
		best := service.selectBestModel([]*database.ModelMetadata{})
		assert.Nil(t, best)
	})

	t.Run("returns single model", func(t *testing.T) {
		models := []*database.ModelMetadata{{ModelID: "only"}}
		best := service.selectBestModel(models)
		assert.Equal(t, "only", best.ModelID)
	})
}

func TestProviderMetadataService_calculateModelScore(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	t.Run("score with all factors", func(t *testing.T) {
		benchmarkScore := 80.0
		popularityScore := 70
		reliabilityScore := 0.9
		version := "v2"
		contextWindow := 32000
		model := &database.ModelMetadata{
			BenchmarkScore:   &benchmarkScore,
			PopularityScore:  &popularityScore,
			ReliabilityScore: &reliabilityScore,
			Version:          &version,
			ContextWindow:    &contextWindow,
		}
		score := service.calculateModelScore(model)
		assert.Greater(t, score, 100.0) // Should be significant
	})

	t.Run("score without optional fields", func(t *testing.T) {
		model := &database.ModelMetadata{ModelID: "basic"}
		score := service.calculateModelScore(model)
		assert.Equal(t, 0.0, score)
	})

	t.Run("context window score capped", func(t *testing.T) {
		contextWindow := 200000 // Very large context window
		model := &database.ModelMetadata{ContextWindow: &contextWindow}
		score := service.calculateModelScore(model)
		assert.LessOrEqual(t, score, 100.0) // Context window contribution capped
	})
}

func TestProviderMetadataService_GetRecommendedModel(t *testing.T) {
	logger := newProviderMetadataTestLogger()

	t.Run("returns best matching model", func(t *testing.T) {
		benchmarkScore := 90.0
		mockModelService := &MockModelMetadataService{
			models: map[string][]*database.ModelMetadata{
				"openai": {
					{ModelID: "gpt-4", SupportsVision: true, SupportsFunctionCalling: true, BenchmarkScore: &benchmarkScore},
					{ModelID: "gpt-3.5", SupportsVision: false, SupportsFunctionCalling: true},
				},
			},
		}
		mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
		service := NewProviderMetadataService(mockModelService, mockRegistry, logger)
		service.providerToModels = mockModelService.models

		model, err := service.GetRecommendedModel(context.Background(), "openai", []string{"vision"})
		require.NoError(t, err)
		assert.Equal(t, "gpt-4", model.ModelID)
	})

	t.Run("returns error when no matching models", func(t *testing.T) {
		mockModelService := &MockModelMetadataService{
			models: map[string][]*database.ModelMetadata{
				"openai": {
					{ModelID: "gpt-3.5", SupportsVision: false},
				},
			},
		}
		mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
		service := NewProviderMetadataService(mockModelService, mockRegistry, logger)
		service.providerToModels = mockModelService.models

		_, err := service.GetRecommendedModel(context.Background(), "openai", []string{"vision"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no models found")
	})
}

func TestProviderMetadataService_RefreshProviderMetadata(t *testing.T) {
	logger := newProviderMetadataTestLogger()

	t.Run("refreshes provider metadata successfully", func(t *testing.T) {
		mockModelService := &MockModelMetadataService{
			models: map[string][]*database.ModelMetadata{
				"openai": {{ModelID: "gpt-4"}},
			},
		}
		mockRegistry := &MockProviderRegistry{
			providers: map[string]*ProviderConfig{
				"openai": {Name: "openai", Type: "llm"},
			},
		}
		service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

		err := service.RefreshProviderMetadata(context.Background(), "openai")
		require.NoError(t, err)
		assert.True(t, mockModelService.refreshCalled)
		assert.Equal(t, "openai", mockModelService.providerCalled)
	})
}

func TestProviderMetadataService_GetProviderStats(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	benchmarkScore := 85.0

	mockModelService := &MockModelMetadataService{
		models: map[string][]*database.ModelMetadata{
			"openai": {
				{
					ModelID:        "gpt-4",
					BenchmarkScore: &benchmarkScore,
					SupportsVision: true,
					SupportsStreaming: true,
					LastRefreshedAt: time.Now(),
				},
				{ModelID: "gpt-3.5", LastRefreshedAt: time.Now().Add(-1 * time.Hour)},
			},
		},
	}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)
	service.providerToModels = mockModelService.models

	stats, err := service.GetProviderStats(context.Background())
	require.NoError(t, err)
	assert.Len(t, stats, 1)

	openaiStats := stats["openai"]
	assert.Equal(t, 2, openaiStats.TotalModels)
	assert.Equal(t, 2, openaiStats.EnabledModels)
	assert.Greater(t, openaiStats.AverageScore, 0.0)
	assert.Contains(t, openaiStats.Capabilities, "vision")
	assert.Contains(t, openaiStats.Capabilities, "streaming")
}

func TestProviderMetadataService_enhanceProviderConfig(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	t.Run("enhances config with model data", func(t *testing.T) {
		benchmarkScore := 85.0
		config := &ProviderConfig{
			Name:    "openai",
			Type:    "llm",
			Enabled: true,
			APIKey:  "sk-test",
		}
		models := []*database.ModelMetadata{
			{
				ModelID:                 "gpt-4",
				ModelName:               "GPT-4",
				BenchmarkScore:          &benchmarkScore,
				SupportsVision:          true,
				SupportsFunctionCalling: true,
			},
		}

		enhanced := service.enhanceProviderConfig(config, models)

		assert.Equal(t, "openai", enhanced.Name)
		assert.True(t, enhanced.Enabled)
		assert.Len(t, enhanced.Models, 1)
		assert.Equal(t, "gpt-4", enhanced.Models[0].ID)
		assert.Contains(t, enhanced.Models[0].Capabilities, "vision")
		assert.Contains(t, enhanced.Models[0].Capabilities, "function_calling")
	})
}

func TestProviderStats(t *testing.T) {
	stats := ProviderStats{
		TotalModels:     10,
		EnabledModels:   8,
		AverageScore:    75.5,
		Capabilities:    map[string]int{"vision": 5, "streaming": 10},
		LastRefreshedAt: time.Now(),
	}

	assert.Equal(t, 10, stats.TotalModels)
	assert.Equal(t, 8, stats.EnabledModels)
	assert.Equal(t, 75.5, stats.AverageScore)
	assert.Equal(t, 5, stats.Capabilities["vision"])
	assert.False(t, stats.LastRefreshedAt.IsZero())
}

func TestProviderMetadataService_StopAutoRefresh(t *testing.T) {
	logger := newProviderMetadataTestLogger()
	mockModelService := &MockModelMetadataService{}
	mockRegistry := &MockProviderRegistry{providers: make(map[string]*ProviderConfig)}
	service := NewProviderMetadataService(mockModelService, mockRegistry, logger)

	// Just verify it doesn't panic
	service.StopAutoRefresh()
}
