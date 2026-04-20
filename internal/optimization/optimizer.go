// Package optimization provides unified LLM optimization capabilities.
package optimization

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/optimization/gptcache"
	"dev.helix.agent/internal/optimization/guidance"
	"dev.helix.agent/internal/optimization/langchain"
	"dev.helix.agent/internal/optimization/llamaindex"
	"dev.helix.agent/internal/optimization/lmql"
	"dev.helix.agent/internal/optimization/outlines"
	"dev.helix.agent/internal/optimization/sglang"
	"dev.helix.agent/internal/optimization/streaming"
)

// Service provides unified access to all optimization capabilities.
type Service struct {
	config *Config

	// Native Go components
	semanticCache    *gptcache.SemanticCache
	enhancedStreamer *streaming.EnhancedStreamer

	// External service clients
	sglangClient     *sglang.Client
	llamaindexClient *llamaindex.Client
	langchainClient  *langchain.Client
	guidanceClient   *guidance.Client
	lmqlClient       *lmql.Client

	// Service availability tracking.
	//
	// Concurrency model (CONST-029): three maps migrate to *safe.Store.
	// mu dropped — CheckServiceAvailability's compound "check
	// lastHealthCheck → probe availability → update both maps" is
	// acceptable without a coordinating mutex: stale reads between
	// the two stores are at worst a second health-check call and
	// cannot produce inconsistent state (each Put is atomic, and
	// readers tolerate the ~millisecond skew).
	serviceStatus    *safe.Store[string, bool]
	lastHealthCheck  *safe.Store[string, time.Time]
	unavailableUntil *safe.Store[string, time.Time]

	// Metrics
	cacheHits   int64
	cacheMisses int64
}

// NewService creates a new optimization service.
func NewService(config *Config) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	s := &Service{
		config:           config,
		serviceStatus:    safe.NewStore[string, bool](),
		lastHealthCheck:  safe.NewStore[string, time.Time](),
		unavailableUntil: safe.NewStore[string, time.Time](),
	}

	// Initialize native Go components
	if config.SemanticCache.Enabled {
		s.semanticCache = gptcache.NewSemanticCache(
			gptcache.WithSimilarityThreshold(config.SemanticCache.SimilarityThreshold),
			gptcache.WithMaxEntries(config.SemanticCache.MaxEntries),
			gptcache.WithTTL(config.SemanticCache.TTL),
		)
	}

	// Note: StructuredGenerator initialization is deferred until a schema is provided
	// since it requires an LLMProvider and schema

	if config.Streaming.Enabled {
		bufferType := streaming.BufferType(config.Streaming.BufferType)
		s.enhancedStreamer = streaming.NewEnhancedStreamer(&streaming.StreamConfig{
			BufferType:       bufferType,
			ProgressInterval: config.Streaming.ProgressInterval,
			RateLimit:        config.Streaming.RateLimit,
		})
	}

	// Initialize external service clients
	if config.SGLang.Enabled {
		s.sglangClient = sglang.NewClient(&sglang.ClientConfig{
			BaseURL: config.SGLang.Endpoint,
			Timeout: config.SGLang.Timeout,
		})
	}

	if config.LlamaIndex.Enabled {
		s.llamaindexClient = llamaindex.NewClient(&llamaindex.ClientConfig{
			BaseURL: config.LlamaIndex.Endpoint,
			Timeout: config.LlamaIndex.Timeout,
		})
	}

	if config.LangChain.Enabled {
		s.langchainClient = langchain.NewClient(&langchain.ClientConfig{
			BaseURL: config.LangChain.Endpoint,
			Timeout: config.LangChain.Timeout,
		})
	}

	if config.Guidance.Enabled {
		s.guidanceClient = guidance.NewClient(&guidance.ClientConfig{
			BaseURL: config.Guidance.Endpoint,
			Timeout: config.Guidance.Timeout,
		})
	}

	if config.LMQL.Enabled {
		s.lmqlClient = lmql.NewClient(&lmql.ClientConfig{
			BaseURL: config.LMQL.Endpoint,
			Timeout: config.LMQL.Timeout,
		})
	}

	return s, nil
}

// OptimizedRequest represents an optimized LLM request.
type OptimizedRequest struct {
	OriginalPrompt   string
	OptimizedPrompt  string
	CacheHit         bool
	CachedResponse   string
	RetrievedContext []string
	DecomposedTasks  []string
	WarmPrefix       bool
}

// OptimizedResponse represents an optimized LLM response.
type OptimizedResponse struct {
	Content          string
	Cached           bool
	StructuredOutput interface{}
	ValidationResult *outlines.ValidationResult
	StreamingMetrics *streaming.AggregatedStream
}

// OptimizeRequest prepares a request for optimal LLM processing.
func (s *Service) OptimizeRequest(ctx context.Context, prompt string, embedding []float64) (*OptimizedRequest, error) {
	result := &OptimizedRequest{
		OriginalPrompt:  prompt,
		OptimizedPrompt: prompt,
	}

	// Check semantic cache
	if s.semanticCache != nil && len(embedding) > 0 {
		hit, err := s.semanticCache.Get(ctx, embedding)
		if err == nil && hit != nil && hit.Entry != nil {
			result.CacheHit = true
			result.CachedResponse = hit.Entry.Response
			atomic.AddInt64(&s.cacheHits, 1)
			return result, nil
		}
		atomic.AddInt64(&s.cacheMisses, 1)
	}

	// Retrieve relevant context if LlamaIndex is available
	if s.llamaindexClient != nil && s.isServiceAvailable("llamaindex") {
		queryResp, err := s.llamaindexClient.Query(ctx, &llamaindex.QueryRequest{
			Query:     prompt,
			TopK:      5,
			UseCognee: s.config.LlamaIndex.UseCogneeIndex,
			Rerank:    true,
		})
		if err == nil && len(queryResp.Sources) > 0 {
			for _, source := range queryResp.Sources {
				result.RetrievedContext = append(result.RetrievedContext, source.Content)
			}
			// Augment prompt with context
			if len(result.RetrievedContext) > 0 {
				contextStr := ""
				for i, ctx := range result.RetrievedContext {
					contextStr += fmt.Sprintf("[Context %d]: %s\n", i+1, ctx)
				}
				result.OptimizedPrompt = fmt.Sprintf("Relevant context:\n%s\n\nQuestion: %s", contextStr, prompt)
			}
		}
	}

	// Decompose complex tasks if LangChain is available
	if s.langchainClient != nil && s.isServiceAvailable("langchain") && isComplexTask(prompt) {
		decomposeResp, err := s.langchainClient.Decompose(ctx, &langchain.DecomposeRequest{
			Task:     prompt,
			MaxSteps: 5,
		})
		if err == nil && len(decomposeResp.Subtasks) > 0 {
			for _, subtask := range decomposeResp.Subtasks {
				result.DecomposedTasks = append(result.DecomposedTasks, subtask.Description)
			}
		}
	}

	// Warm prefix cache if SGLang is available
	if s.sglangClient != nil && s.isServiceAvailable("sglang") {
		if _, err := s.sglangClient.WarmPrefix(ctx, prompt[:min(500, len(prompt))]); err == nil {
			result.WarmPrefix = true
		}
	}

	return result, nil
}

// OptimizeResponse processes and caches an LLM response.
func (s *Service) OptimizeResponse(ctx context.Context, response string, embedding []float64, query string, schema *outlines.JSONSchema) (*OptimizedResponse, error) {
	result := &OptimizedResponse{
		Content: response,
	}

	// Validate structured output if schema provided
	if schema != nil {
		validator, err := outlines.NewSchemaValidator(schema)
		if err == nil {
			validationResult := validator.Validate(response)
			result.ValidationResult = validationResult
			if validationResult.Valid {
				result.StructuredOutput = validationResult.Data
			}
		}
	}

	// Cache the response
	if s.semanticCache != nil && len(embedding) > 0 {
		_, err := s.semanticCache.Set(ctx, query, response, embedding, nil)
		if err == nil {
			result.Cached = true
		}
	}

	return result, nil
}

// StreamEnhanced provides enhanced streaming capabilities.
func (s *Service) StreamEnhanced(ctx context.Context, stream <-chan *streaming.StreamChunk, progress streaming.ProgressCallback) (<-chan *streaming.StreamChunk, func() *streaming.AggregatedStream) {
	if s.enhancedStreamer == nil {
		// Return passthrough if streaming not enabled
		aggregator := streaming.NewChunkAggregator()
		return aggregator.AggregateChunks(ctx, stream)
	}
	return s.enhancedStreamer.StreamEnhanced(ctx, stream, progress)
}

// GenerateStructured generates structured output following a schema.
func (s *Service) GenerateStructured(ctx context.Context, prompt string, schema *outlines.JSONSchema, generator func(string) (string, error)) (*outlines.StructuredResponse, error) {
	if !s.config.StructuredOutput.Enabled {
		return nil, fmt.Errorf("structured output not enabled")
	}

	// Generate response
	response, err := generator(prompt)
	if err != nil {
		return nil, err
	}

	// Validate and parse
	validator, err := outlines.NewSchemaValidator(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}
	validationResult := validator.Validate(response)

	// Convert validation errors to strings
	var errorStrings []string
	for _, err := range validationResult.Errors {
		errorStrings = append(errorStrings, err.Error())
	}

	return &outlines.StructuredResponse{
		Content:    response,
		ParsedData: validationResult.Data,
		Valid:      validationResult.Valid,
		Errors:     errorStrings,
	}, nil
}

// DecomposeTask decomposes a complex task into subtasks.
func (s *Service) DecomposeTask(ctx context.Context, task string) (*langchain.DecomposeResponse, error) {
	if s.langchainClient == nil || !s.isServiceAvailable("langchain") {
		return nil, fmt.Errorf("langchain service not available")
	}
	return s.langchainClient.Decompose(ctx, &langchain.DecomposeRequest{Task: task})
}

// RunReActAgent runs a ReAct reasoning agent.
func (s *Service) RunReActAgent(ctx context.Context, goal string, tools []string) (*langchain.ReActResponse, error) {
	if s.langchainClient == nil || !s.isServiceAvailable("langchain") {
		return nil, fmt.Errorf("langchain service not available")
	}
	return s.langchainClient.RunReActAgent(ctx, &langchain.ReActRequest{
		Goal:           goal,
		AvailableTools: tools,
	})
}

// QueryDocuments queries documents with advanced retrieval.
func (s *Service) QueryDocuments(ctx context.Context, query string, options *llamaindex.QueryRequest) (*llamaindex.QueryResponse, error) {
	if s.llamaindexClient == nil || !s.isServiceAvailable("llamaindex") {
		return nil, fmt.Errorf("llamaindex service not available")
	}
	if options == nil {
		options = &llamaindex.QueryRequest{Query: query}
	}
	options.Query = query
	return s.llamaindexClient.Query(ctx, options)
}

// GenerateConstrained generates text with constraints.
func (s *Service) GenerateConstrained(ctx context.Context, prompt string, constraints []lmql.Constraint) (*lmql.ConstrainedResponse, error) {
	if s.lmqlClient == nil || !s.isServiceAvailable("lmql") {
		return nil, fmt.Errorf("lmql service not available")
	}
	return s.lmqlClient.GenerateConstrained(ctx, &lmql.ConstrainedRequest{
		Prompt:      prompt,
		Constraints: constraints,
	})
}

// SelectFromOptions selects from constrained options.
func (s *Service) SelectFromOptions(ctx context.Context, prompt string, options []string) (string, error) {
	if s.guidanceClient == nil || !s.isServiceAvailable("guidance") {
		return "", fmt.Errorf("guidance service not available")
	}
	return s.guidanceClient.SelectOne(ctx, prompt, options)
}

// CreateSession creates a new conversation session with prefix caching.
func (s *Service) CreateSession(ctx context.Context, sessionID, systemPrompt string) error {
	if s.sglangClient == nil || !s.isServiceAvailable("sglang") {
		return fmt.Errorf("sglang service not available")
	}
	_, err := s.sglangClient.CreateSession(ctx, sessionID, systemPrompt)
	return err
}

// ContinueSession continues a conversation with prefix caching.
func (s *Service) ContinueSession(ctx context.Context, sessionID, message string) (string, error) {
	if s.sglangClient == nil || !s.isServiceAvailable("sglang") {
		return "", fmt.Errorf("sglang service not available")
	}
	return s.sglangClient.ContinueSession(ctx, sessionID, message)
}

// GetCacheStats returns semantic cache statistics.
func (s *Service) GetCacheStats() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled": s.semanticCache != nil,
		"hits":    atomic.LoadInt64(&s.cacheHits),
		"misses":  atomic.LoadInt64(&s.cacheMisses),
	}

	if s.semanticCache != nil {
		cacheStats := s.semanticCache.Stats(context.Background())
		if cacheStats != nil {
			stats["entries"] = cacheStats.TotalEntries
			stats["hit_rate"] = cacheStats.HitRate
		}
	}

	return stats
}

// GetServiceStatus returns the status of all external services.
func (s *Service) GetServiceStatus(ctx context.Context) map[string]bool {
	s.checkServiceHealth(ctx)
	return s.serviceStatus.Snapshot()
}

// isServiceAvailable checks if a service is available.
func (s *Service) isServiceAvailable(service string) bool {
	// Check if service is marked unavailable
	if until, ok := s.unavailableUntil.Get(service); ok {
		if time.Now().Before(until) {
			return false
		}
	}

	// Check cached status
	if status, ok := s.serviceStatus.Get(service); ok {
		if lastCheck, ok := s.lastHealthCheck.Get(service); ok {
			if time.Since(lastCheck) < s.config.Fallback.HealthCheckInterval {
				return status
			}
		}
	}

	return true // Assume available if not checked
}

// checkServiceHealth checks the health of all services.
func (s *Service) checkServiceHealth(ctx context.Context) {
	now := time.Now()
	checkInterval := s.config.Fallback.HealthCheckInterval

	// Helper function to check if health check is needed
	needsCheck := func(service string) bool {
		if lastCheck, ok := s.lastHealthCheck.Get(service); ok {
			return time.Since(lastCheck) >= checkInterval
		}
		return true
	}

	if s.sglangClient != nil && needsCheck("sglang") {
		s.serviceStatus.Put("sglang", s.sglangClient.IsAvailable(ctx))
		s.lastHealthCheck.Put("sglang", now)
	}

	if s.llamaindexClient != nil && needsCheck("llamaindex") {
		s.serviceStatus.Put("llamaindex", s.llamaindexClient.IsAvailable(ctx))
		s.lastHealthCheck.Put("llamaindex", now)
	}

	if s.langchainClient != nil && needsCheck("langchain") {
		s.serviceStatus.Put("langchain", s.langchainClient.IsAvailable(ctx))
		s.lastHealthCheck.Put("langchain", now)
	}

	if s.guidanceClient != nil && needsCheck("guidance") {
		s.serviceStatus.Put("guidance", s.guidanceClient.IsAvailable(ctx))
		s.lastHealthCheck.Put("guidance", now)
	}

	if s.lmqlClient != nil && needsCheck("lmql") {
		s.serviceStatus.Put("lmql", s.lmqlClient.IsAvailable(ctx))
		s.lastHealthCheck.Put("lmql", now)
	}
}

// markServiceUnavailable marks a service as temporarily unavailable.
//
//nolint:unused
func (s *Service) markServiceUnavailable(service string) {
	s.serviceStatus.Put(service, false)
	s.unavailableUntil.Put(service, time.Now().Add(s.config.Fallback.RetryUnavailableAfter))
}

// isComplexTask heuristically determines if a task is complex.
func isComplexTask(prompt string) bool {
	// Simple heuristics for complex tasks
	complexIndicators := []string{
		"step by step",
		"multi-step",
		"first, then",
		"implement",
		"create a",
		"build a",
		"design a",
		"analyze",
	}

	promptLower := prompt
	for _, indicator := range complexIndicators {
		if len(promptLower) > 100 && containsIgnoreCase(promptLower, indicator) {
			return true
		}
	}

	return len(prompt) > 500
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
