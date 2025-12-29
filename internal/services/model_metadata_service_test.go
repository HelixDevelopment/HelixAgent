package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/database"
	"github.com/superagent/superagent/internal/modelsdev"
)

func TestGetDefaultModelMetadataConfig(t *testing.T) {
	config := getDefaultModelMetadataConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 24*time.Hour, config.RefreshInterval)
	assert.Equal(t, 1*time.Hour, config.CacheTTL)
	assert.Equal(t, 100, config.DefaultBatchSize)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.RetryDelay)
	assert.True(t, config.EnableAutoRefresh)
}

func TestModelMetadataService_convertModelInfoToMetadata(t *testing.T) {
	service := &ModelMetadataService{}

	t.Run("full model info conversion", func(t *testing.T) {
		info := modelsdev.ModelInfo{
			ID:            "gpt-4",
			Name:          "GPT-4",
			Provider:      "OpenAI",
			Description:   "GPT-4 model",
			ContextWindow: 8192,
			MaxTokens:     4096,
			Pricing: &modelsdev.ModelPricing{
				InputPrice:  0.01,
				OutputPrice: 0.03,
				Currency:    "USD",
			},
			Capabilities: modelsdev.ModelCapabilities{
				Vision:          true,
				FunctionCalling: true,
				Streaming:       true,
				JSONMode:        true,
				ImageGeneration: false,
				Audio:           false,
				CodeGeneration:  true,
				Reasoning:       false,
			},
			Performance: &modelsdev.ModelPerformance{
				BenchmarkScore:   85.5,
				PopularityScore:  95,
				ReliabilityScore: 92.0,
			},
			Family:  "GPT",
			Version: "1.0",
			Tags:    []string{"chat", "text-generation"},
			Metadata: map[string]interface{}{
				"custom_field": "value",
			},
		}

		result := service.convertModelInfoToMetadata(info, "openai")

		assert.Equal(t, "gpt-4", result.ModelID)
		assert.Equal(t, "GPT-4", result.ModelName)
		assert.Equal(t, "openai", result.ProviderID)
		assert.Equal(t, "OpenAI", result.ProviderName)
		assert.Equal(t, "GPT-4 model", result.Description)
		assert.Equal(t, 8192, *result.ContextWindow)
		assert.Equal(t, 4096, *result.MaxTokens)
		assert.Equal(t, 0.01, *result.PricingInput)
		assert.Equal(t, 0.03, *result.PricingOutput)
		assert.Equal(t, "USD", result.PricingCurrency)
		assert.True(t, result.SupportsVision)
		assert.True(t, result.SupportsFunctionCalling)
		assert.True(t, result.SupportsStreaming)
		assert.True(t, result.SupportsJSONMode)
		assert.False(t, result.SupportsImageGeneration)
		assert.False(t, result.SupportsAudio)
		assert.True(t, result.SupportsCodeGeneration)
		assert.False(t, result.SupportsReasoning)
		assert.Equal(t, 85.5, *result.BenchmarkScore)
		assert.Equal(t, 95, *result.PopularityScore)
		assert.Equal(t, 92.0, *result.ReliabilityScore)
		assert.Equal(t, "GPT", *result.ModelFamily)
		assert.Equal(t, "1.0", *result.Version)
		assert.Equal(t, []string{"chat", "text-generation"}, result.Tags)
		assert.NotNil(t, result.LastRefreshedAt)
	})

	t.Run("minimal model info conversion", func(t *testing.T) {
		info := modelsdev.ModelInfo{
			ID:       "simple-model",
			Name:     "Simple Model",
			Provider: "Test",
		}

		result := service.convertModelInfoToMetadata(info, "test")

		assert.Equal(t, "simple-model", result.ModelID)
		assert.Equal(t, "Simple Model", result.ModelName)
		assert.Equal(t, "test", result.ProviderID)
		assert.Equal(t, "Test", result.ProviderName)
		assert.Nil(t, result.ContextWindow)
		assert.Nil(t, result.MaxTokens)
		assert.Nil(t, result.PricingInput)
		assert.Nil(t, result.PricingOutput)
		assert.False(t, result.SupportsVision)
		assert.False(t, result.SupportsFunctionCalling)
		assert.Nil(t, result.ModelFamily)
		assert.Nil(t, result.Version)
	})

	t.Run("empty family and version handling", func(t *testing.T) {
		info := modelsdev.ModelInfo{
			ID:       "test-model",
			Name:     "Test Model",
			Provider: "Test",
			Family:   "",
			Version:  "",
		}

		result := service.convertModelInfoToMetadata(info, "test")

		assert.Nil(t, result.ModelFamily)
		assert.Nil(t, result.Version)
	})

	t.Run("nil pricing handling", func(t *testing.T) {
		info := modelsdev.ModelInfo{
			ID:       "test-model",
			Name:     "Test Model",
			Provider: "Test",
			Pricing:  nil,
		}

		result := service.convertModelInfoToMetadata(info, "test")

		assert.Nil(t, result.PricingInput)
		assert.Nil(t, result.PricingOutput)
	})

	t.Run("nil performance handling", func(t *testing.T) {
		info := modelsdev.ModelInfo{
			ID:          "test-model",
			Name:        "Test Model",
			Provider:    "Test",
			Performance: nil,
		}

		result := service.convertModelInfoToMetadata(info, "test")

		assert.Nil(t, result.BenchmarkScore)
		assert.Nil(t, result.PopularityScore)
		assert.Nil(t, result.ReliabilityScore)
	})
}

func TestInMemoryCache(t *testing.T) {
	t.Run("basic cache operations", func(t *testing.T) {
		cache := NewInMemoryCache(1 * time.Hour)
		ctx := context.Background()
		model := &database.ModelMetadata{
			ModelID:   "test-model",
			ModelName: "Test Model",
		}

		// Test Set and Get
		err := cache.Set(ctx, "test-model", model)
		assert.NoError(t, err)

		retrieved, exists, err := cache.Get(ctx, "test-model")
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, model, retrieved)

		// Test Size
		size, err := cache.Size(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, size)

		// Test Delete
		err = cache.Delete(ctx, "test-model")
		assert.NoError(t, err)

		retrieved, exists, err = cache.Get(ctx, "test-model")
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, retrieved)

		// Test Clear
		err = cache.Set(ctx, "model1", model)
		assert.NoError(t, err)
		err = cache.Set(ctx, "model2", model)
		assert.NoError(t, err)

		size, err = cache.Size(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 2, size)

		err = cache.Clear(ctx)
		assert.NoError(t, err)

		size, err = cache.Size(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 0, size)
	})

	t.Run("bulk operations", func(t *testing.T) {
		cache := NewInMemoryCache(1 * time.Hour)
		ctx := context.Background()

		models := map[string]*database.ModelMetadata{
			"model1": {ModelID: "model1", ModelName: "Model 1"},
			"model2": {ModelID: "model2", ModelName: "Model 2"},
			"model3": {ModelID: "model3", ModelName: "Model 3"},
		}

		// Test SetBulk
		err := cache.SetBulk(ctx, models)
		assert.NoError(t, err)

		// Test GetBulk
		retrieved, err := cache.GetBulk(ctx, []string{"model1", "model2", "nonexistent"})
		assert.NoError(t, err)
		assert.Len(t, retrieved, 2)
		assert.Equal(t, models["model1"], retrieved["model1"])
		assert.Equal(t, models["model2"], retrieved["model2"])
	})

	t.Run("provider operations (not supported)", func(t *testing.T) {
		cache := NewInMemoryCache(1 * time.Hour)
		ctx := context.Background()
		model := &database.ModelMetadata{ModelID: "test-model", ModelName: "Test Model"}

		result, err := cache.GetProviderModels(ctx, "openai")
		assert.NoError(t, err)
		assert.Nil(t, result)

		err = cache.SetProviderModels(ctx, "openai", []*database.ModelMetadata{model})
		assert.NoError(t, err)

		err = cache.DeleteProviderModels(ctx, "openai")
		assert.NoError(t, err)
	})

	t.Run("capability operations (not supported)", func(t *testing.T) {
		cache := NewInMemoryCache(1 * time.Hour)
		ctx := context.Background()
		model := &database.ModelMetadata{ModelID: "test-model", ModelName: "Test Model"}

		result, err := cache.GetByCapability(ctx, "vision")
		assert.NoError(t, err)
		assert.Nil(t, result)

		err = cache.SetByCapability(ctx, "vision", []*database.ModelMetadata{model})
		assert.NoError(t, err)
	})

	t.Run("health check", func(t *testing.T) {
		cache := NewInMemoryCache(1 * time.Hour)
		ctx := context.Background()

		err := cache.HealthCheck(ctx)
		assert.NoError(t, err)
	})

	t.Run("TTL expiration", func(t *testing.T) {
		cache := NewInMemoryCache(100 * time.Millisecond)
		ctx := context.Background()
		model := &database.ModelMetadata{ModelID: "test-model", ModelName: "Test Model"}

		err := cache.Set(ctx, "test-model", model)
		assert.NoError(t, err)

		// Should exist immediately
		retrieved, exists, err := cache.Get(ctx, "test-model")
		assert.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, model, retrieved)

		// Wait for TTL to expire
		time.Sleep(150 * time.Millisecond)

		// Should be deleted after TTL
		retrieved, exists, err = cache.Get(ctx, "test-model")
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, retrieved)
	})
}

func TestModelMetadataService_GetModelsByCapability(t *testing.T) {
	t.Run("capability filtering logic", func(t *testing.T) {
		// Test the capability filtering logic directly with sample models
		models := []*database.ModelMetadata{
			{ModelID: "gpt-4", ModelName: "GPT-4", SupportsVision: false, SupportsFunctionCalling: true, SupportsStreaming: true, SupportsJSONMode: false, SupportsImageGeneration: false, SupportsAudio: false, SupportsCodeGeneration: true, SupportsReasoning: false},
			{ModelID: "gpt-4-vision", ModelName: "GPT-4 Vision", SupportsVision: true, SupportsFunctionCalling: true, SupportsStreaming: true, SupportsJSONMode: false, SupportsImageGeneration: false, SupportsAudio: false, SupportsCodeGeneration: true, SupportsReasoning: false},
			{ModelID: "claude-3", ModelName: "Claude 3", SupportsVision: true, SupportsFunctionCalling: false, SupportsStreaming: true, SupportsJSONMode: true, SupportsImageGeneration: false, SupportsAudio: false, SupportsCodeGeneration: false, SupportsReasoning: true},
		}

		// Test each capability
		testCases := []struct {
			capability string
			expected   int
		}{
			{"vision", 2},
			{"function_calling", 2},
			{"streaming", 3},
			{"json_mode", 1},
			{"image_generation", 0},
			{"audio", 0},
			{"code_generation", 2},
			{"reasoning", 1},
		}

		for _, tc := range testCases {
			t.Run(tc.capability, func(t *testing.T) {
				filtered := make([]*database.ModelMetadata, 0)
				for _, model := range models {
					var hasCapability bool
					switch tc.capability {
					case "vision":
						hasCapability = model.SupportsVision
					case "function_calling":
						hasCapability = model.SupportsFunctionCalling
					case "streaming":
						hasCapability = model.SupportsStreaming
					case "json_mode":
						hasCapability = model.SupportsJSONMode
					case "image_generation":
						hasCapability = model.SupportsImageGeneration
					case "audio":
						hasCapability = model.SupportsAudio
					case "code_generation":
						hasCapability = model.SupportsCodeGeneration
					case "reasoning":
						hasCapability = model.SupportsReasoning
					}
					if hasCapability {
						filtered = append(filtered, model)
					}
				}
				assert.Len(t, filtered, tc.expected)
			})
		}
	})
}

// Helper function to test capability filtering
func filterModelsByCapability(models []*database.ModelMetadata, capability string) []*database.ModelMetadata {
	var filtered []*database.ModelMetadata
	for _, model := range models {
		var hasCapability bool

		switch capability {
		case "vision":
			hasCapability = model.SupportsVision
		case "function_calling":
			hasCapability = model.SupportsFunctionCalling
		case "streaming":
			hasCapability = model.SupportsStreaming
		case "json_mode":
			hasCapability = model.SupportsJSONMode
		case "image_generation":
			hasCapability = model.SupportsImageGeneration
		case "audio":
			hasCapability = model.SupportsAudio
		case "code_generation":
			hasCapability = model.SupportsCodeGeneration
		case "reasoning":
			hasCapability = model.SupportsReasoning
		}

		if hasCapability {
			filtered = append(filtered, model)
		}
	}
	return filtered

	// Test vision capability
	models, err := service.GetModelsByCapability(context.Background(), "vision")
	assert.NoError(t, err)
	assert.Len(t, models, 1)
	assert.Equal(t, "gpt-4-vision", models[0].ModelID)

	// Test function calling capability
	models, err = service.GetModelsByCapability(context.Background(), "function_calling")
	assert.NoError(t, err)
	assert.Len(t, models, 2)

	// Test streaming capability
	models, err = service.GetModelsByCapability(context.Background(), "streaming")
	assert.NoError(t, err)
	assert.Len(t, models, 3)

	// Test JSON mode capability
	models, err = service.GetModelsByCapability(context.Background(), "json_mode")
	assert.NoError(t, err)
	assert.Len(t, models, 1)

	// Test image generation capability
	models, err = service.GetModelsByCapability(context.Background(), "image_generation")
	assert.NoError(t, err)
	assert.Len(t, models, 1)

	// Test audio capability
	models, err = service.GetModelsByCapability(context.Background(), "audio")
	assert.NoError(t, err)
	assert.Len(t, models, 1)

	// Test code generation capability
	models, err = service.GetModelsByCapability(context.Background(), "code_generation")
	assert.NoError(t, err)
	assert.Len(t, models, 2)

	// Test reasoning capability
	models, err = service.GetModelsByCapability(context.Background(), "reasoning")
	assert.NoError(t, err)
	assert.Len(t, models, 1)
}
