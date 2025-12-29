package services

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// RequestService handles LLM request routing and load balancing
type RequestService struct {
	providers map[string]LLMProvider
	ensemble  *EnsembleService
	memory    *MemoryService
	strategy  RoutingStrategy
	mu        sync.RWMutex
}

// RoutingStrategy defines different routing approaches
type RoutingStrategy interface {
	SelectProvider(providers map[string]LLMProvider, req *models.LLMRequest) (string, error)
}

// ProviderHealth tracks health and performance metrics for providers
type ProviderHealth struct {
	Name          string
	Healthy       bool
	LastCheck     time.Time
	ResponseTime  int64   // Average response time in milliseconds
	SuccessRate   float64 // Success rate (0.0 to 1.0)
	ErrorCount    int64
	TotalRequests int64
	LastError     string
	Weight        float64 // Dynamic weight based on performance
	mu            sync.RWMutex
}

// Routing strategies

// RoundRobinStrategy implements round-robin load balancing
type RoundRobinStrategy struct {
	counter int64
	mu      sync.Mutex
}

// WeightedStrategy implements weighted load balancing based on performance
type WeightedStrategy struct{}

// HealthBasedStrategy implements health-based routing
type HealthBasedStrategy struct{}

// LatencyBasedStrategy implements latency-based routing
type LatencyBasedStrategy struct{}

// RandomStrategy implements random provider selection
type RandomStrategy struct{}

func NewRequestService(strategy string, ensemble *EnsembleService, memory *MemoryService) *RequestService {
	var routingStrategy RoutingStrategy

	switch strategy {
	case "round_robin":
		routingStrategy = &RoundRobinStrategy{}
	case "weighted":
		routingStrategy = &WeightedStrategy{}
	case "health_based":
		routingStrategy = &HealthBasedStrategy{}
	case "latency_based":
		routingStrategy = &LatencyBasedStrategy{}
	case "random":
		routingStrategy = &RandomStrategy{}
	default:
		routingStrategy = &WeightedStrategy{} // Default
	}

	return &RequestService{
		providers: make(map[string]LLMProvider),
		ensemble:  ensemble,
		memory:    memory,
		strategy:  routingStrategy,
	}
}

func (r *RequestService) RegisterProvider(name string, provider LLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

func (r *RequestService) RemoveProvider(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}

func (r *RequestService) GetProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

func (r *RequestService) ProcessRequest(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	r.mu.RLock()
	providers := make(map[string]LLMProvider)
	for k, v := range r.providers {
		providers[k] = v
	}
	r.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Enhance request with memory if enabled
	if r.memory != nil && req.MemoryEnhanced {
		if err := r.memory.EnhanceRequest(ctx, req); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Memory enhancement failed: %v\n", err)
		}
	}

	// Check if ensemble is requested and we have multiple providers
	if req.EnsembleConfig != nil && len(providers) >= req.EnsembleConfig.MinProviders {
		result, err := r.ensemble.RunEnsemble(ctx, req)
		if err != nil {
			// Fall back to single provider if ensemble fails
			return r.processSingleProvider(ctx, req, providers)
		}
		return result.Selected, nil
	}

	// Process with single provider
	return r.processSingleProvider(ctx, req, providers)
}

func (r *RequestService) ProcessRequestStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	r.mu.RLock()
	providers := make(map[string]LLMProvider)
	for k, v := range r.providers {
		providers[k] = v
	}
	r.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Enhance request with memory if enabled
	if r.memory != nil && req.MemoryEnhanced {
		if err := r.memory.EnhanceRequest(ctx, req); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Memory enhancement failed: %v\n", err)
		}
	}

	// Check if ensemble is requested and we have multiple providers
	if req.EnsembleConfig != nil && len(providers) >= req.EnsembleConfig.MinProviders {
		return r.ensemble.RunEnsembleStream(ctx, req)
	}

	// Process with single provider
	return r.processSingleProviderStream(ctx, req, providers)
}

func (r *RequestService) processSingleProvider(ctx context.Context, req *models.LLMRequest, providers map[string]LLMProvider) (*models.LLMResponse, error) {
	// Select provider based on routing strategy
	providerName, err := r.strategy.SelectProvider(providers, req)
	if err != nil {
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}

	provider, exists := providers[providerName]
	if !exists {
		return nil, fmt.Errorf("selected provider %s not found", providerName)
	}

	// Execute request
	resp, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("provider %s failed: %w", providerName, err)
	}

	// Add provider metadata
	resp.ProviderID = providerName
	resp.ProviderName = providerName

	return resp, nil
}

func (r *RequestService) processSingleProviderStream(ctx context.Context, req *models.LLMRequest, providers map[string]LLMProvider) (<-chan *models.LLMResponse, error) {
	// Select provider based on routing strategy
	providerName, err := r.strategy.SelectProvider(providers, req)
	if err != nil {
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}

	provider, exists := providers[providerName]
	if !exists {
		return nil, fmt.Errorf("selected provider %s not found", providerName)
	}

	// Execute streaming request
	streamChan, err := provider.CompleteStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("provider %s failed: %w", providerName, err)
	}

	// Wrap responses with provider info
	wrappedChan := make(chan *models.LLMResponse)
	go func() {
		defer close(wrappedChan)
		for resp := range streamChan {
			resp.ProviderID = providerName
			resp.ProviderName = providerName
			wrappedChan <- resp
		}
	}()

	return wrappedChan, nil
}

// Routing strategy implementations

// RoundRobinStrategy
func (s *RoundRobinStrategy) SelectProvider(providers map[string]LLMProvider, req *models.LLMRequest) (string, error) {
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}

	selected := names[s.counter%int64(len(names))]
	s.counter++
	return selected, nil
}

// WeightedStrategy
func (s *WeightedStrategy) SelectProvider(providers map[string]LLMProvider, req *models.LLMRequest) (string, error) {
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	// For now, use equal weights
	// In a real implementation, you'd track performance and adjust weights
	weights := make(map[string]float64)
	totalWeight := 0.0

	for name := range providers {
		weight := 1.0 // Equal weight for all providers
		if req.EnsembleConfig != nil {
			// Apply preference weights
			for i, preferred := range req.EnsembleConfig.PreferredProviders {
				if name == preferred {
					weight = 2.0 - float64(i)*0.1 // Higher weight for more preferred providers
					break
				}
			}
		}
		weights[name] = weight
		totalWeight += weight
	}

	// Select based on weighted random selection
	random := rand.Float64() * totalWeight
	current := 0.0

	for name, weight := range weights {
		current += weight
		if random <= current {
			return name, nil
		}
	}

	// Fallback to first provider
	for name := range providers {
		return name, nil
	}

	return "", fmt.Errorf("failed to select provider")
}

// HealthBasedStrategy
func (s *HealthBasedStrategy) SelectProvider(providers map[string]LLMProvider, req *models.LLMRequest) (string, error) {
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	// Filter healthy providers
	healthyProviders := make([]string, 0)
	for name := range providers {
		// In a real implementation, you'd check actual health status
		// For now, assume all providers are healthy
		healthyProviders = append(healthyProviders, name)
	}

	if len(healthyProviders) == 0 {
		return "", fmt.Errorf("no healthy providers available")
	}

	// Select first healthy provider
	return healthyProviders[0], nil
}

// LatencyBasedStrategy
func (s *LatencyBasedStrategy) SelectProvider(providers map[string]LLMProvider, req *models.LLMRequest) (string, error) {
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	// For now, select randomly
	// In a real implementation, you'd track actual latency metrics
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}

	if len(names) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	selected := names[rand.Intn(len(names))]
	return selected, nil
}

// RandomStrategy
func (s *RandomStrategy) SelectProvider(providers map[string]LLMProvider, req *models.LLMRequest) (string, error) {
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}

	if len(names) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	selected := names[rand.Intn(len(names))]
	return selected, nil
}

// ProviderHealth management

func (r *RequestService) UpdateProviderHealth(name string, healthy bool, responseTime int64, err error) {
	// This would be used to track provider health and performance
	// Implementation would involve maintaining a health registry
}

func (r *RequestService) GetProviderHealth(name string) *ProviderHealth {
	// Return health information for a specific provider
	// Implementation would query the health registry
	return &ProviderHealth{
		Name:         name,
		Healthy:      true,
		LastCheck:    time.Now(),
		ResponseTime: 1000,
		SuccessRate:  0.95,
	}
}

func (r *RequestService) GetAllProviderHealth() map[string]*ProviderHealth {
	// Return health information for all providers
	health := make(map[string]*ProviderHealth)
	r.mu.RLock()
	for name := range r.providers {
		health[name] = &ProviderHealth{
			Name:         name,
			Healthy:      true,
			LastCheck:    time.Now(),
			ResponseTime: 1000,
			SuccessRate:  0.95,
		}
	}
	r.mu.RUnlock()
	return health
}

// Advanced routing features

// CircuitBreakerPattern implements circuit breaker for failing providers
type CircuitBreakerPattern struct {
	providers map[string]*RequestCircuitBreaker
	mu        sync.RWMutex
}

type RequestCircuitBreaker struct {
	Name             string
	State            RequestCircuitState
	FailureCount     int64
	LastFailTime     time.Time
	SuccessCount     int64
	Timeout          time.Duration
	FailureThreshold int64
	RecoveryTimeout  time.Duration
	mu               sync.RWMutex
}

type RequestCircuitState int

const (
	RequestStateClosed RequestCircuitState = iota
	RequestStateOpen
	RequestStateHalfOpen
)

func NewCircuitBreakerPattern() *CircuitBreakerPattern {
	return &CircuitBreakerPattern{
		providers: make(map[string]*RequestCircuitBreaker),
	}
}

func (c *CircuitBreakerPattern) GetCircuitBreaker(name string) *RequestCircuitBreaker {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cb, exists := c.providers[name]
	if !exists {
		cb = &RequestCircuitBreaker{
			Name:             name,
			State:            RequestStateClosed,
			FailureThreshold: 5,
			Timeout:          60 * time.Second,
			RecoveryTimeout:  30 * time.Second,
		}
		c.providers[name] = cb
	}

	return cb
}

func (cb *RequestCircuitBreaker) Call(ctx context.Context, operation func() (*models.LLMResponse, error)) (*models.LLMResponse, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.State {
	case RequestStateOpen:
		if time.Since(cb.LastFailTime) > cb.RecoveryTimeout {
			cb.State = RequestStateHalfOpen
		} else {
			return nil, fmt.Errorf("circuit breaker is open for provider %s", cb.Name)
		}
	case RequestStateHalfOpen:
		// Allow one request through
		resp, err := operation()
		if err != nil {
			cb.FailureCount++
			cb.LastFailTime = time.Now()
			cb.State = RequestStateOpen
			return resp, err
		}
		cb.SuccessCount++
		cb.State = RequestStateClosed
		return resp, nil
	case RequestStateClosed:
		// Normal operation
		resp, err := operation()
		if err != nil {
			cb.FailureCount++
			if cb.FailureCount >= cb.FailureThreshold {
				cb.State = RequestStateOpen
				cb.LastFailTime = time.Now()
			}
			return resp, err
		}
		cb.SuccessCount++
		return resp, nil
	}

	return nil, fmt.Errorf("unknown circuit breaker state")
}

// RetryPattern implements retry logic with exponential backoff
type RetryPattern struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

func NewRetryPattern(maxRetries int, initialDelay, maxDelay time.Duration, backoffFactor float64) *RetryPattern {
	return &RetryPattern{
		MaxRetries:    maxRetries,
		InitialDelay:  initialDelay,
		MaxDelay:      maxDelay,
		BackoffFactor: backoffFactor,
	}
}

func (r *RetryPattern) Execute(ctx context.Context, operation func() (*models.LLMResponse, error)) (*models.LLMResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		resp, err := operation()
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Don't wait on the last attempt
		if attempt < r.MaxRetries {
			delay := time.Duration(float64(r.InitialDelay) * math.Pow(r.BackoffFactor, float64(attempt)))
			if delay > r.MaxDelay {
				delay = r.MaxDelay
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	return nil, lastErr
}
