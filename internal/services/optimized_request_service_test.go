package services

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/optimization"
	"github.com/superagent/superagent/internal/optimization/outlines"
)

func TestNewOptimizedRequestService(t *testing.T) {
	t.Run("with all config options", func(t *testing.T) {
		log := logrus.New()
		log.SetLevel(logrus.PanicLevel)

		reqService := NewRequestService("random", nil, nil)

		config := OptimizedRequestServiceConfig{
			RequestService: reqService,
			Logger:         log,
		}

		service := NewOptimizedRequestService(config)
		assert.NotNil(t, service)
		assert.NotNil(t, service.log)
	})

	t.Run("with nil logger creates default", func(t *testing.T) {
		reqService := NewRequestService("random", nil, nil)

		config := OptimizedRequestServiceConfig{
			RequestService: reqService,
		}

		service := NewOptimizedRequestService(config)
		assert.NotNil(t, service)
		assert.NotNil(t, service.log)
	})
}

func TestOptimizedRequestService_ProcessRequest_NoOptimization(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	// Register a mock provider
	mockProvider := &MockLLMProviderForRequest{
		name: "test-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				Content:      "optimized test response",
				ProviderName: "test-provider",
			}, nil
		},
	}
	reqService.RegisterProvider("test-provider", mockProvider)

	config := OptimizedRequestServiceConfig{
		RequestService: reqService,
		Logger:         log,
	}

	service := NewOptimizedRequestService(config)

	t.Run("falls back to regular processing when no optimization service", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "test prompt",
		}

		resp, err := service.ProcessRequest(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "optimized test response", resp.Content)
	})
}

func TestOptimizedRequestService_ProcessRequestStream_NoOptimization(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	// Register a mock streaming provider
	mockProvider := &MockLLMProviderForRequest{
		name: "test-provider",
		streamFunc: func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
			ch := make(chan *models.LLMResponse, 1)
			go func() {
				defer close(ch)
				ch <- &models.LLMResponse{
					Content:      "streamed response",
					FinishReason: "stop",
				}
			}()
			return ch, nil
		},
	}
	reqService.RegisterProvider("test-provider", mockProvider)

	config := OptimizedRequestServiceConfig{
		RequestService: reqService,
		Logger:         log,
	}

	service := NewOptimizedRequestService(config)

	t.Run("falls back to regular streaming when no optimization service", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "test prompt",
		}

		stream, err := service.ProcessRequestStream(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, stream)

		// Read from stream
		resp := <-stream
		assert.Equal(t, "streamed response", resp.Content)
	})
}

func TestOptimizedRequestService_GetCacheStats(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	t.Run("returns disabled when no optimization service", func(t *testing.T) {
		config := OptimizedRequestServiceConfig{
			RequestService: reqService,
			Logger:         log,
		}

		service := NewOptimizedRequestService(config)
		stats := service.GetCacheStats()

		assert.NotNil(t, stats)
		assert.Equal(t, false, stats["enabled"])
	})

	t.Run("returns stats from optimization service", func(t *testing.T) {
		optConfig := &optimization.Config{
			Enabled: true,
			SemanticCache: optimization.SemanticCacheConfig{
				Enabled: true,
			},
		}
		optService, err := optimization.NewService(optConfig)
		assert.NoError(t, err)

		config := OptimizedRequestServiceConfig{
			RequestService:      reqService,
			OptimizationService: optService,
			Logger:              log,
		}

		service := NewOptimizedRequestService(config)
		stats := service.GetCacheStats()

		assert.NotNil(t, stats)
		// Stats should have cache info
		assert.Contains(t, stats, "enabled")
	})
}

func TestOptimizedRequestService_GetServiceStatus(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	t.Run("returns empty map when no optimization service", func(t *testing.T) {
		config := OptimizedRequestServiceConfig{
			RequestService: reqService,
			Logger:         log,
		}

		service := NewOptimizedRequestService(config)
		status := service.GetServiceStatus(context.Background())

		assert.NotNil(t, status)
		assert.Empty(t, status)
	})

	t.Run("returns status from optimization service", func(t *testing.T) {
		optConfig := &optimization.Config{
			Enabled: true,
		}
		optService, err := optimization.NewService(optConfig)
		assert.NoError(t, err)

		config := OptimizedRequestServiceConfig{
			RequestService:      reqService,
			OptimizationService: optService,
			Logger:              log,
		}

		service := NewOptimizedRequestService(config)
		status := service.GetServiceStatus(context.Background())

		assert.NotNil(t, status)
	})
}

func TestOptimizedRequestService_SetOptimizationService(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	config := OptimizedRequestServiceConfig{
		RequestService: reqService,
		Logger:         log,
	}

	service := NewOptimizedRequestService(config)

	// Initially no optimization service
	assert.Nil(t, service.optimizationService)

	// Set optimization service
	optConfig := &optimization.Config{Enabled: true}
	optService, err := optimization.NewService(optConfig)
	assert.NoError(t, err)

	service.SetOptimizationService(optService)
	assert.NotNil(t, service.optimizationService)
}

func TestOptimizedRequestService_SetEmbeddingManager(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	config := OptimizedRequestServiceConfig{
		RequestService: reqService,
		Logger:         log,
	}

	service := NewOptimizedRequestService(config)

	// Initially no embedding manager
	assert.Nil(t, service.embeddingManager)

	// Set embedding manager
	embeddingMgr := NewEmbeddingManager(nil, nil, log)

	service.SetEmbeddingManager(embeddingMgr)
	assert.NotNil(t, service.embeddingManager)
}

func TestOptimizedRequestService_ProviderDelegation(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	config := OptimizedRequestServiceConfig{
		RequestService: reqService,
		Logger:         log,
	}

	service := NewOptimizedRequestService(config)

	t.Run("GetProviders returns empty list initially", func(t *testing.T) {
		providers := service.GetProviders()
		assert.Empty(t, providers)
	})

	t.Run("RegisterProvider adds provider", func(t *testing.T) {
		mockProvider := &MockLLMProviderForRequest{name: "delegated-provider"}
		service.RegisterProvider("delegated-provider", mockProvider)

		providers := service.GetProviders()
		assert.Contains(t, providers, "delegated-provider")
	})

	t.Run("RemoveProvider removes provider", func(t *testing.T) {
		service.RemoveProvider("delegated-provider")

		providers := service.GetProviders()
		assert.NotContains(t, providers, "delegated-provider")
	})
}

func TestOptimizedRequestService_ProcessRequestWithOptimization(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	// Register a mock provider
	mockProvider := &MockLLMProviderForRequest{
		name: "test-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				Content:      "response from provider",
				ProviderName: "test-provider",
			}, nil
		},
	}
	reqService.RegisterProvider("test-provider", mockProvider)

	// Create optimization service with semantic cache disabled
	optConfig := &optimization.Config{
		Enabled: true,
		SemanticCache: optimization.SemanticCacheConfig{
			Enabled: false,
		},
	}
	optService, err := optimization.NewService(optConfig)
	assert.NoError(t, err)

	config := OptimizedRequestServiceConfig{
		RequestService:      reqService,
		OptimizationService: optService,
		Logger:              log,
	}

	service := NewOptimizedRequestService(config)

	t.Run("processes request through optimization pipeline", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "test prompt for optimization",
		}

		resp, err := service.ProcessRequest(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "response from provider", resp.Content)
	})
}

func TestOptimizedRequestService_ProcessRequestStreamWithOptimization(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	// Register a mock streaming provider
	mockProvider := &MockLLMProviderForRequest{
		name: "test-provider",
		streamFunc: func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
			ch := make(chan *models.LLMResponse, 2)
			go func() {
				defer close(ch)
				ch <- &models.LLMResponse{
					Content: "chunk 1",
				}
				ch <- &models.LLMResponse{
					Content:      "chunk 2",
					FinishReason: "stop",
				}
			}()
			return ch, nil
		},
	}
	reqService.RegisterProvider("test-provider", mockProvider)

	// Create optimization service
	optConfig := &optimization.Config{
		Enabled: true,
		SemanticCache: optimization.SemanticCacheConfig{
			Enabled: false,
		},
		Streaming: optimization.StreamingConfig{
			Enabled:    true,
			BufferType: "word",
		},
	}
	optService, err := optimization.NewService(optConfig)
	assert.NoError(t, err)

	config := OptimizedRequestServiceConfig{
		RequestService:      reqService,
		OptimizationService: optService,
		Logger:              log,
	}

	service := NewOptimizedRequestService(config)

	t.Run("streams through optimization pipeline", func(t *testing.T) {
		req := &models.LLMRequest{
			Prompt: "test prompt for streaming",
		}

		stream, err := service.ProcessRequestStream(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, stream)

		// Read all chunks
		var chunks []string
		for resp := range stream {
			chunks = append(chunks, resp.Content)
		}
		assert.Greater(t, len(chunks), 0)
	})
}

func TestOptimizedRequestService_ProcessRequestWithSchema_NoOptimization(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	// Register a mock provider that returns JSON
	mockProvider := &MockLLMProviderForRequest{
		name: "test-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				Content:      `{"name": "test", "age": 25}`,
				ProviderName: "test-provider",
			}, nil
		},
	}
	reqService.RegisterProvider("test-provider", mockProvider)

	config := OptimizedRequestServiceConfig{
		RequestService: reqService,
		Logger:         log,
	}

	service := NewOptimizedRequestService(config)

	t.Run("falls back to regular processing without optimization service", func(t *testing.T) {
		schema := &outlines.JSONSchema{
			Type: "object",
			Properties: map[string]*outlines.JSONSchema{
				"name": {Type: "string"},
				"age":  {Type: "integer"},
			},
		}

		req := &models.LLMRequest{
			Prompt: "return a JSON object with name and age",
		}

		resp, err := service.ProcessRequestWithSchema(context.Background(), req, schema)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Contains(t, resp.Content, "name")
	})
}

func TestOptimizedRequestService_ProcessRequestWithSchema_WithOptimization(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	reqService := NewRequestService("random", nil, nil)

	// Register a mock provider that returns JSON
	mockProvider := &MockLLMProviderForRequest{
		name: "test-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				Content:      `{"name": "John", "age": 30}`,
				ProviderName: "test-provider",
			}, nil
		},
	}
	reqService.RegisterProvider("test-provider", mockProvider)

	// Create optimization service with structured output enabled
	optConfig := &optimization.Config{
		Enabled: true,
		StructuredOutput: optimization.StructuredOutputConfig{
			Enabled:    true,
			StrictMode: false,
			MaxRetries: 1,
		},
	}
	optService, err := optimization.NewService(optConfig)
	assert.NoError(t, err)

	config := OptimizedRequestServiceConfig{
		RequestService:      reqService,
		OptimizationService: optService,
		Logger:              log,
	}

	service := NewOptimizedRequestService(config)

	t.Run("generates structured output with schema", func(t *testing.T) {
		schema := &outlines.JSONSchema{
			Type: "object",
			Properties: map[string]*outlines.JSONSchema{
				"name": {Type: "string"},
				"age":  {Type: "integer"},
			},
		}

		req := &models.LLMRequest{
			Prompt: "return a JSON object with name and age",
		}

		resp, err := service.ProcessRequestWithSchema(context.Background(), req, schema)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Contains(t, resp.Content, "name")
		// Metadata should contain structured output info
		assert.NotNil(t, resp.Metadata)
	})
}
