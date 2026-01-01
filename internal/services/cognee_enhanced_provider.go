package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

// CogneeEnhancedProvider wraps any LLM provider with Cognee capabilities
type CogneeEnhancedProvider struct {
	provider      llm.LLMProvider
	cogneeService *CogneeService
	logger        *logrus.Logger
	config        *CogneeProviderConfig
	name          string
	mu            sync.RWMutex
	stats         *CogneeProviderStats
}

// CogneeProviderConfig configures the enhanced provider behavior
type CogneeProviderConfig struct {
	// Enhancement behavior
	EnhanceBeforeRequest   bool `json:"enhance_before_request"`
	StoreAfterResponse     bool `json:"store_after_response"`
	AutoCognifyResponses   bool `json:"auto_cognify_responses"`
	EnableGraphReasoning   bool `json:"enable_graph_reasoning"`
	EnableCodeIntelligence bool `json:"enable_code_intelligence"`

	// Context settings
	MaxContextInjection int     `json:"max_context_injection"`
	RelevanceThreshold  float64 `json:"relevance_threshold"`
	ContextPrefix       string  `json:"context_prefix"`
	ContextSuffix       string  `json:"context_suffix"`

	// Dataset settings
	DefaultDataset    string   `json:"default_dataset"`
	UseSessionDataset bool     `json:"use_session_dataset"`
	UseUserDataset    bool     `json:"use_user_dataset"`
	DatasetHierarchy  []string `json:"dataset_hierarchy"` // Order of datasets to search

	// Performance settings
	AsyncEnhancement   bool          `json:"async_enhancement"`
	EnhancementTimeout time.Duration `json:"enhancement_timeout"`
	CacheEnhancements  bool          `json:"cache_enhancements"`
	CacheTTL           time.Duration `json:"cache_ttl"`

	// Streaming settings
	EnhanceStreamingPrompt bool `json:"enhance_streaming_prompt"`
	StreamingBufferSize    int  `json:"streaming_buffer_size"`
}

// CogneeProviderStats tracks provider-level statistics
type CogneeProviderStats struct {
	mu                     sync.RWMutex
	TotalRequests          int64
	EnhancedRequests       int64
	StoredResponses        int64
	EnhancementErrors      int64
	StorageErrors          int64
	AverageEnhancementTime time.Duration
	AverageResponseTime    time.Duration
}

// NewCogneeEnhancedProvider wraps an LLM provider with Cognee capabilities
func NewCogneeEnhancedProvider(
	name string,
	provider llm.LLMProvider,
	cogneeService *CogneeService,
	logger *logrus.Logger,
) *CogneeEnhancedProvider {
	if logger == nil {
		logger = logrus.New()
	}

	return &CogneeEnhancedProvider{
		provider:      provider,
		cogneeService: cogneeService,
		logger:        logger,
		name:          name,
		config:        getDefaultCogneeProviderConfig(),
		stats:         &CogneeProviderStats{},
	}
}

// NewCogneeEnhancedProviderWithConfig creates an enhanced provider with custom config
func NewCogneeEnhancedProviderWithConfig(
	name string,
	provider llm.LLMProvider,
	cogneeService *CogneeService,
	config *CogneeProviderConfig,
	logger *logrus.Logger,
) *CogneeEnhancedProvider {
	if logger == nil {
		logger = logrus.New()
	}
	if config == nil {
		config = getDefaultCogneeProviderConfig()
	}

	return &CogneeEnhancedProvider{
		provider:      provider,
		cogneeService: cogneeService,
		logger:        logger,
		name:          name,
		config:        config,
		stats:         &CogneeProviderStats{},
	}
}

func getDefaultCogneeProviderConfig() *CogneeProviderConfig {
	return &CogneeProviderConfig{
		EnhanceBeforeRequest:   true,
		StoreAfterResponse:     true,
		AutoCognifyResponses:   true,
		EnableGraphReasoning:   true,
		EnableCodeIntelligence: true,
		MaxContextInjection:    2048,
		RelevanceThreshold:     0.7,
		ContextPrefix:          "## Relevant Knowledge Context:\n",
		ContextSuffix:          "\n\n---\n\n## User Request:\n",
		DefaultDataset:         "default",
		UseSessionDataset:      true,
		UseUserDataset:         true,
		DatasetHierarchy:       []string{"session", "user", "global", "default"},
		AsyncEnhancement:       false,
		EnhancementTimeout:     5 * time.Second,
		CacheEnhancements:      true,
		CacheTTL:               30 * time.Minute,
		EnhanceStreamingPrompt: true,
		StreamingBufferSize:    100,
	}
}

// Complete implements the LLMProvider interface with Cognee enhancement
func (p *CogneeEnhancedProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	startTime := time.Now()

	p.stats.mu.Lock()
	p.stats.TotalRequests++
	p.stats.mu.Unlock()

	// Enhance the request with Cognee context
	enhancedReq := req
	if p.config.EnhanceBeforeRequest && p.cogneeService != nil && p.cogneeService.IsReady() {
		enhancedReq = p.enhanceRequest(ctx, req)
	}

	// Call the underlying provider
	resp, err := p.provider.Complete(ctx, enhancedReq)
	if err != nil {
		return nil, err
	}

	// Store the response in Cognee
	if p.config.StoreAfterResponse && p.cogneeService != nil && p.cogneeService.IsReady() {
		go p.storeResponse(context.Background(), req, resp)
	}

	// Update stats
	p.stats.mu.Lock()
	p.stats.AverageResponseTime = (p.stats.AverageResponseTime + time.Since(startTime)) / 2
	p.stats.mu.Unlock()

	// Add Cognee metadata to response
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]interface{})
	}
	resp.Metadata["cognee_enhanced"] = p.config.EnhanceBeforeRequest
	resp.Metadata["cognee_stored"] = p.config.StoreAfterResponse

	return resp, nil
}

// CompleteStream implements streaming completion with Cognee enhancement
func (p *CogneeEnhancedProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	p.stats.mu.Lock()
	p.stats.TotalRequests++
	p.stats.mu.Unlock()

	// Enhance the request for streaming
	enhancedReq := req
	if p.config.EnhanceStreamingPrompt && p.cogneeService != nil && p.cogneeService.IsReady() {
		enhancedReq = p.enhanceRequest(ctx, req)
	}

	// Get the stream from the underlying provider
	stream, err := p.provider.CompleteStream(ctx, enhancedReq)
	if err != nil {
		return nil, err
	}

	// Wrap the stream to capture the full response for storage
	outputChan := make(chan *models.LLMResponse, p.config.StreamingBufferSize)

	go func() {
		defer close(outputChan)

		var fullContent strings.Builder
		var lastResp *models.LLMResponse

		for resp := range stream {
			fullContent.WriteString(resp.Content)
			lastResp = resp
			outputChan <- resp
		}

		// Store the complete response
		if p.config.StoreAfterResponse && lastResp != nil && p.cogneeService != nil {
			completeResp := &models.LLMResponse{
				ID:           lastResp.ID,
				Content:      fullContent.String(),
				ProviderName: lastResp.ProviderName,
				TokensUsed:   lastResp.TokensUsed,
				ResponseTime: lastResp.ResponseTime,
				FinishReason: lastResp.FinishReason,
			}
			go p.storeResponse(context.Background(), req, completeResp)
		}
	}()

	return outputChan, nil
}

// HealthCheck implements the LLMProvider interface
func (p *CogneeEnhancedProvider) HealthCheck() error {
	// Check underlying provider health
	if err := p.provider.HealthCheck(); err != nil {
		return err
	}

	// Optionally check Cognee health
	if p.cogneeService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if !p.cogneeService.IsHealthy(ctx) {
			p.logger.Warn("Cognee service is not healthy, enhancement disabled")
		}
	}

	return nil
}

// GetCapabilities implements the LLMProvider interface
func (p *CogneeEnhancedProvider) GetCapabilities() *models.ProviderCapabilities {
	caps := p.provider.GetCapabilities()

	// Add Cognee-specific capabilities
	if caps.Metadata == nil {
		caps.Metadata = make(map[string]string)
	}
	caps.Metadata["cognee_enhanced"] = "true"
	caps.Metadata["cognee_features"] = "memory_enhancement,graph_reasoning,temporal_awareness,code_intelligence,feedback_loop"

	// Add enhanced features
	caps.SupportedFeatures = append(caps.SupportedFeatures,
		"cognee_memory",
		"knowledge_graph",
		"semantic_search",
		"auto_cognify",
	)

	return caps
}

// ValidateConfig implements the LLMProvider interface
func (p *CogneeEnhancedProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return p.provider.ValidateConfig(config)
}

// enhanceRequest enhances the request with Cognee context
func (p *CogneeEnhancedProvider) enhanceRequest(ctx context.Context, req *models.LLMRequest) *models.LLMRequest {
	enhanceCtx, cancel := context.WithTimeout(ctx, p.config.EnhancementTimeout)
	defer cancel()

	startTime := time.Now()

	// Get enhanced context from Cognee
	enhanced, err := p.cogneeService.EnhanceRequest(enhanceCtx, req)
	if err != nil {
		p.logger.WithError(err).Warn("Failed to enhance request with Cognee")
		p.stats.mu.Lock()
		p.stats.EnhancementErrors++
		p.stats.mu.Unlock()
		return req
	}

	// Update stats
	p.stats.mu.Lock()
	p.stats.EnhancedRequests++
	p.stats.AverageEnhancementTime = (p.stats.AverageEnhancementTime + time.Since(startTime)) / 2
	p.stats.mu.Unlock()

	// Create enhanced request
	enhancedReq := *req // Copy

	// Apply enhancement based on confidence
	if enhanced.Confidence >= p.config.RelevanceThreshold {
		enhancedReq.Prompt = enhanced.EnhancedPrompt

		// Also enhance messages if present
		if len(enhancedReq.Messages) > 0 && len(enhanced.RelevantMemories) > 0 {
			enhancedReq.Messages = p.enhanceMessages(enhancedReq.Messages, enhanced)
		}
	}

	// Add Cognee metadata
	if enhancedReq.Memory == nil {
		enhancedReq.Memory = make(map[string]string)
	}
	enhancedReq.Memory["cognee_enhanced"] = "true"
	enhancedReq.Memory["cognee_confidence"] = fmt.Sprintf("%.2f", enhanced.Confidence)

	return &enhancedReq
}

// enhanceMessages enhances chat messages with Cognee context
func (p *CogneeEnhancedProvider) enhanceMessages(messages []models.Message, enhanced *EnhancedContext) []models.Message {
	if len(messages) == 0 || len(enhanced.RelevantMemories) == 0 {
		return messages
	}

	// Build context message
	var contextParts []string
	contextParts = append(contextParts, p.config.ContextPrefix)

	for i, mem := range enhanced.RelevantMemories {
		if i >= 5 {
			break
		}
		contextParts = append(contextParts, fmt.Sprintf("- %s", truncateText(mem.Content, 500)))
	}

	if len(enhanced.GraphInsights) > 0 {
		contextParts = append(contextParts, "\n## Knowledge Graph Insights:")
		for i, insight := range enhanced.GraphInsights {
			if i >= 3 {
				break
			}
			if text, ok := insight["text"].(string); ok {
				contextParts = append(contextParts, fmt.Sprintf("- %s", truncateText(text, 300)))
			}
		}
	}

	contextMessage := strings.Join(contextParts, "\n")

	// Insert context as a system message at the beginning
	result := make([]models.Message, 0, len(messages)+1)

	// Check if first message is already a system message
	if len(messages) > 0 && messages[0].Role == "system" {
		// Append to existing system message
		enhancedSystem := messages[0]
		enhancedSystem.Content = messages[0].Content + "\n\n" + contextMessage
		result = append(result, enhancedSystem)
		result = append(result, messages[1:]...)
	} else {
		// Add new system message
		result = append(result, models.Message{
			Role:    "system",
			Content: contextMessage,
		})
		result = append(result, messages...)
	}

	return result
}

// storeResponse stores the response in Cognee
func (p *CogneeEnhancedProvider) storeResponse(ctx context.Context, req *models.LLMRequest, resp *models.LLMResponse) {
	if err := p.cogneeService.ProcessResponse(ctx, req, resp); err != nil {
		p.logger.WithError(err).Warn("Failed to store response in Cognee")
		p.stats.mu.Lock()
		p.stats.StorageErrors++
		p.stats.mu.Unlock()
		return
	}

	p.stats.mu.Lock()
	p.stats.StoredResponses++
	p.stats.mu.Unlock()
}

// GetStats returns provider statistics
func (p *CogneeEnhancedProvider) GetStats() *CogneeProviderStats {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	return &CogneeProviderStats{
		TotalRequests:          p.stats.TotalRequests,
		EnhancedRequests:       p.stats.EnhancedRequests,
		StoredResponses:        p.stats.StoredResponses,
		EnhancementErrors:      p.stats.EnhancementErrors,
		StorageErrors:          p.stats.StorageErrors,
		AverageEnhancementTime: p.stats.AverageEnhancementTime,
		AverageResponseTime:    p.stats.AverageResponseTime,
	}
}

// GetConfig returns the provider configuration
func (p *CogneeEnhancedProvider) GetConfig() *CogneeProviderConfig {
	return p.config
}

// GetUnderlyingProvider returns the wrapped provider
func (p *CogneeEnhancedProvider) GetUnderlyingProvider() llm.LLMProvider {
	return p.provider
}

// GetName returns the provider name
func (p *CogneeEnhancedProvider) GetName() string {
	return p.name
}

// SetConfig updates the provider configuration
func (p *CogneeEnhancedProvider) SetConfig(config *CogneeProviderConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
}

// =====================================================
// COGNEE PROVIDER REGISTRY INTEGRATION
// =====================================================

// WrapProvidersWithCognee wraps all providers in a registry with Cognee enhancement
func WrapProvidersWithCognee(
	providers map[string]llm.LLMProvider,
	cogneeService *CogneeService,
	logger *logrus.Logger,
) map[string]llm.LLMProvider {
	wrapped := make(map[string]llm.LLMProvider)

	for name, provider := range providers {
		wrapped[name] = NewCogneeEnhancedProvider(name, provider, cogneeService, logger)
	}

	return wrapped
}

// EnhanceProviderRegistry wraps all providers in a registry with Cognee
func EnhanceProviderRegistry(registry *ProviderRegistry, cogneeService *CogneeService, logger *logrus.Logger) error {
	providers := registry.ListProviders()

	for _, name := range providers {
		provider, err := registry.GetProvider(name)
		if err != nil {
			continue
		}

		// Skip if already enhanced
		if _, ok := provider.(*CogneeEnhancedProvider); ok {
			continue
		}

		// Create enhanced provider
		enhanced := NewCogneeEnhancedProvider(name, provider, cogneeService, logger)

		// Re-register with enhancement
		if err := registry.UnregisterProvider(name); err != nil {
			logger.WithError(err).Warnf("Failed to unregister provider %s for enhancement", name)
			continue
		}

		if err := registry.RegisterProvider(name, enhanced); err != nil {
			logger.WithError(err).Warnf("Failed to re-register enhanced provider %s", name)
			continue
		}

		logger.Infof("Provider %s enhanced with Cognee capabilities", name)
	}

	return nil
}
