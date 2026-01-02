package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/optimization"
	"github.com/superagent/superagent/internal/optimization/outlines"
	"github.com/superagent/superagent/internal/optimization/streaming"
)

// OptimizedRequestService wraps RequestService with optimization capabilities.
type OptimizedRequestService struct {
	requestService      *RequestService
	optimizationService *optimization.Service
	embeddingManager    *EmbeddingManager
	log                 *logrus.Logger
	mu                  sync.RWMutex
}

// OptimizedRequestServiceConfig holds configuration for the optimized service.
type OptimizedRequestServiceConfig struct {
	RequestService      *RequestService
	OptimizationService *optimization.Service
	EmbeddingManager    *EmbeddingManager
	Logger              *logrus.Logger
}

// NewOptimizedRequestService creates a new optimized request service.
func NewOptimizedRequestService(config OptimizedRequestServiceConfig) *OptimizedRequestService {
	log := config.Logger
	if log == nil {
		log = logrus.New()
	}

	return &OptimizedRequestService{
		requestService:      config.RequestService,
		optimizationService: config.OptimizationService,
		embeddingManager:    config.EmbeddingManager,
		log:                 log,
	}
}

// ProcessRequest processes an LLM request with optimizations.
func (s *OptimizedRequestService) ProcessRequest(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// If no optimization service, fall back to standard processing
	if s.optimizationService == nil {
		return s.requestService.ProcessRequest(ctx, req)
	}

	// Generate embedding for the prompt if embedding manager is available
	var embedding []float64
	if s.embeddingManager != nil {
		embResp, err := s.embeddingManager.GenerateEmbedding(ctx, req.Prompt)
		if err != nil {
			s.log.WithError(err).Warn("Failed to generate embedding, proceeding without cache")
		} else {
			embedding = embResp.Embeddings
		}
	}

	// Optimize the request (check cache, retrieve context, decompose tasks)
	optimized, err := s.optimizationService.OptimizeRequest(ctx, req.Prompt, embedding)
	if err != nil {
		s.log.WithError(err).Warn("Optimization failed, proceeding with original request")
	} else {
		// Check for cache hit
		if optimized.CacheHit && optimized.CachedResponse != "" {
			s.log.Info("Cache hit - returning cached response")
			return &models.LLMResponse{
				Content:      optimized.CachedResponse,
				ProviderID:   "cache",
				ProviderName: "semantic_cache",
				Metadata: map[string]interface{}{
					"cached": true,
				},
			}, nil
		}

		// Use optimized prompt if different
		if optimized.OptimizedPrompt != req.Prompt {
			s.log.WithField("contextSources", len(optimized.RetrievedContext)).Debug("Using optimized prompt with context")
			req.Prompt = optimized.OptimizedPrompt
		}

		// Log decomposed tasks if any
		if len(optimized.DecomposedTasks) > 0 {
			s.log.WithField("subtasks", len(optimized.DecomposedTasks)).Debug("Task decomposed into subtasks")
		}
	}

	// Process with the underlying request service
	resp, err := s.requestService.ProcessRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Optimize the response (cache it, validate if schema provided)
	if len(embedding) > 0 && resp != nil {
		optimizedResp, optErr := s.optimizationService.OptimizeResponse(ctx, resp.Content, embedding, req.Prompt, nil)
		if optErr != nil {
			s.log.WithError(optErr).Warn("Response optimization failed")
		} else if optimizedResp.Cached {
			s.log.Debug("Response cached successfully")
		}
	}

	return resp, nil
}

// ProcessRequestWithSchema processes a request expecting structured output.
func (s *OptimizedRequestService) ProcessRequestWithSchema(ctx context.Context, req *models.LLMRequest, schema *outlines.JSONSchema) (*models.LLMResponse, error) {
	if s.optimizationService == nil {
		return s.requestService.ProcessRequest(ctx, req)
	}

	// Generate embedding
	var embedding []float64
	if s.embeddingManager != nil {
		embResp, err := s.embeddingManager.GenerateEmbedding(ctx, req.Prompt)
		if err == nil {
			embedding = embResp.Embeddings
		}
	}

	// Check cache
	optimized, _ := s.optimizationService.OptimizeRequest(ctx, req.Prompt, embedding)
	if optimized != nil && optimized.CacheHit {
		// Validate cached response against schema
		validator, err := outlines.NewSchemaValidator(schema)
		if err == nil {
			result := validator.Validate(optimized.CachedResponse)
			if result.Valid {
				s.log.Info("Cache hit with valid structured output")
				return &models.LLMResponse{
					Content:      optimized.CachedResponse,
					ProviderID:   "cache",
					ProviderName: "semantic_cache",
					Metadata: map[string]interface{}{
						"cached": true,
					},
				}, nil
			}
		}
	}

	// Use structured generation
	generator := func(prompt string) (string, error) {
		// Update request prompt
		origPrompt := req.Prompt
		req.Prompt = prompt

		// Add schema hint to prompt
		schemaHint := fmt.Sprintf("\n\nRespond with valid JSON matching this schema: %s", schema.Type)
		if len(schema.Properties) > 0 {
			schemaHint = fmt.Sprintf("\n\nRespond with valid JSON containing these fields: ")
			first := true
			for name, prop := range schema.Properties {
				if !first {
					schemaHint += ", "
				}
				schemaHint += fmt.Sprintf("%s (%s)", name, prop.Type)
				first = false
			}
		}
		req.Prompt = req.Prompt + schemaHint

		resp, err := s.requestService.ProcessRequest(ctx, req)
		req.Prompt = origPrompt
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}

	result, err := s.optimizationService.GenerateStructured(ctx, req.Prompt, schema, generator)
	if err != nil {
		return nil, fmt.Errorf("structured generation failed: %w", err)
	}

	// Cache the valid response
	if result.Valid && len(embedding) > 0 {
		s.optimizationService.OptimizeResponse(ctx, result.Content, embedding, req.Prompt, schema)
	}

	return &models.LLMResponse{
		Content: result.Content,
		Metadata: map[string]interface{}{
			"structured_output": result.ParsedData,
			"validation_valid":  result.Valid,
		},
	}, nil
}

// ProcessRequestStream processes a streaming request with optimizations.
func (s *OptimizedRequestService) ProcessRequestStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	// If no optimization service, fall back to standard streaming
	if s.optimizationService == nil {
		return s.requestService.ProcessRequestStream(ctx, req)
	}

	// Generate embedding for cache check
	var embedding []float64
	if s.embeddingManager != nil {
		embResp, err := s.embeddingManager.GenerateEmbedding(ctx, req.Prompt)
		if err == nil {
			embedding = embResp.Embeddings
		}
	}

	// Check cache first
	optimized, _ := s.optimizationService.OptimizeRequest(ctx, req.Prompt, embedding)
	if optimized != nil && optimized.CacheHit {
		// Return cached response as a single-chunk stream
		s.log.Info("Cache hit - streaming cached response")
		responseChan := make(chan *models.LLMResponse, 1)
		go func() {
			defer close(responseChan)
			responseChan <- &models.LLMResponse{
				Content:      optimized.CachedResponse,
				ProviderID:   "cache",
				ProviderName: "semantic_cache",
				FinishReason: "stop",
				Metadata: map[string]interface{}{
					"cached":       true,
					"stream_done":  true,
				},
			}
		}()
		return responseChan, nil
	}

	// Use optimized prompt
	if optimized != nil && optimized.OptimizedPrompt != req.Prompt {
		req.Prompt = optimized.OptimizedPrompt
	}

	// Get stream from underlying service
	stream, err := s.requestService.ProcessRequestStream(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert to StreamChunk channel for optimization
	chunkStream := make(chan *streaming.StreamChunk)
	go func() {
		defer close(chunkStream)
		index := 0
		for resp := range stream {
			// Check if streaming is done via FinishReason or Metadata
			done := resp.FinishReason == "stop" || resp.FinishReason == "end_turn"
			if resp.Metadata != nil {
				if d, ok := resp.Metadata["stream_done"].(bool); ok {
					done = d
				}
			}
			chunkStream <- &streaming.StreamChunk{
				Content: resp.Content,
				Index:   index,
				Done:    done,
			}
			index++
		}
	}()

	// Apply streaming optimizations
	enhancedStream, getResult := s.optimizationService.StreamEnhanced(ctx, chunkStream, nil)

	// Convert back to LLMResponse channel
	responseChan := make(chan *models.LLMResponse)
	go func() {
		defer close(responseChan)
		var fullContent string
		for chunk := range enhancedStream {
			fullContent += chunk.Content
			finishReason := ""
			if chunk.Done {
				finishReason = "stop"
			}
			responseChan <- &models.LLMResponse{
				Content:      chunk.Content,
				FinishReason: finishReason,
				Metadata: map[string]interface{}{
					"stream_done": chunk.Done,
				},
			}
		}

		// Cache the complete response
		result := getResult()
		if result != nil && len(embedding) > 0 {
			s.optimizationService.OptimizeResponse(ctx, result.FullContent, embedding, req.Prompt, nil)
		}
	}()

	return responseChan, nil
}

// GetCacheStats returns cache statistics.
func (s *OptimizedRequestService) GetCacheStats() map[string]interface{} {
	if s.optimizationService == nil {
		return map[string]interface{}{"enabled": false}
	}
	return s.optimizationService.GetCacheStats()
}

// GetServiceStatus returns the status of optimization services.
func (s *OptimizedRequestService) GetServiceStatus(ctx context.Context) map[string]bool {
	if s.optimizationService == nil {
		return map[string]bool{}
	}
	return s.optimizationService.GetServiceStatus(ctx)
}

// SetOptimizationService sets or updates the optimization service.
func (s *OptimizedRequestService) SetOptimizationService(svc *optimization.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.optimizationService = svc
}

// SetEmbeddingManager sets or updates the embedding manager.
func (s *OptimizedRequestService) SetEmbeddingManager(mgr *EmbeddingManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embeddingManager = mgr
}

// RegisterProvider delegates to the underlying RequestService.
func (s *OptimizedRequestService) RegisterProvider(name string, provider LLMProvider) {
	s.requestService.RegisterProvider(name, provider)
}

// RemoveProvider delegates to the underlying RequestService.
func (s *OptimizedRequestService) RemoveProvider(name string) {
	s.requestService.RemoveProvider(name)
}

// GetProviders delegates to the underlying RequestService.
func (s *OptimizedRequestService) GetProviders() []string {
	return s.requestService.GetProviders()
}
