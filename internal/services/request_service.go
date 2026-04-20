package services

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/models"
)

// RequestService handles LLM request routing and load balancing.
//
// Concurrent-safe by construction: providers is a safe.Store; the prior
// sync.RWMutex is dropped.
type RequestService struct {
	providers *safe.Store[string, LLMProvider]
	ensemble  *EnsembleService
	memory    *MemoryService
	strategy  RoutingStrategy
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
}

// Routing strategies

// RoundRobinStrategy implements round-robin load balancing
type RoundRobinStrategy struct {
	counter int64
	mu      sync.Mutex
}

// ProviderMetrics tracks performance metrics for a provider with rolling window.
//
// Concurrent-safe by construction: atomic counters for SuccessCount,
// FailureCount, TotalLatencyMs; safe.Slice for LatencyHistory; lastUpdated
// as atomic.Int64 Unix-nanos (time.Time not atomic). No bare map or
// slice fields remain paired with a mutex.
type ProviderMetrics struct {
	SuccessCount   atomic.Int64
	FailureCount   atomic.Int64
	TotalLatencyMs atomic.Int64
	LatencyHistory *safe.Slice[int64] // Rolling window of recent latencies
	lastUpdatedNs  atomic.Int64       // UnixNano; read via GetLastUpdated()
}

// NewProviderMetrics constructs a fresh ProviderMetrics. Prefer this over
// a zero-value struct literal so LatencyHistory is initialised.
func NewProviderMetrics() *ProviderMetrics {
	pm := &ProviderMetrics{
		LatencyHistory: safe.NewSlice[int64](),
	}
	pm.lastUpdatedNs.Store(time.Now().UnixNano())
	return pm
}

// maxLatencyHistory bounds the rolling window of kept latencies.
const maxLatencyHistory = 100

// GetSuccessRate returns the success rate (0.0 to 1.0)
func (pm *ProviderMetrics) GetSuccessRate() float64 {
	success := pm.SuccessCount.Load()
	failure := pm.FailureCount.Load()
	total := success + failure
	if total == 0 {
		return 1.0 // Default to 1.0 for new providers
	}
	return float64(success) / float64(total)
}

// GetAverageLatency returns the average latency in milliseconds
func (pm *ProviderMetrics) GetAverageLatency() float64 {
	snap := pm.LatencyHistory.Snapshot()
	if len(snap) == 0 {
		return 1000.0 // Default latency for new providers
	}
	var sum int64
	for _, lat := range snap {
		sum += lat
	}
	return float64(sum) / float64(len(snap))
}

// GetLastUpdated returns the last-updated timestamp.
func (pm *ProviderMetrics) GetLastUpdated() time.Time {
	return time.Unix(0, pm.lastUpdatedNs.Load())
}

// RecordSuccess records a successful request with latency
func (pm *ProviderMetrics) RecordSuccess(latencyMs int64) {
	pm.SuccessCount.Add(1)
	pm.TotalLatencyMs.Add(latencyMs)
	pm.lastUpdatedNs.Store(time.Now().UnixNano())

	// Maintain rolling window of last 100 latencies.
	// Append+Snapshot+Replace is not atomic as a triple; under heavy
	// concurrency the window may briefly hold 101-102 entries before
	// trimming. Acceptable for a best-effort rolling window.
	pm.LatencyHistory.Append(latencyMs)
	snap := pm.LatencyHistory.Snapshot()
	if len(snap) > maxLatencyHistory {
		pm.LatencyHistory.Replace(snap[len(snap)-maxLatencyHistory:])
	}
}

// RecordFailure records a failed request
func (pm *ProviderMetrics) RecordFailure() {
	pm.FailureCount.Add(1)
	pm.lastUpdatedNs.Store(time.Now().UnixNano())
}

// MetricsRegistry is a thread-safe registry for provider metrics.
//
// Concurrent-safe by construction: metrics is a safe.Store. Store.Update
// gives the atomic get-or-create-and-insert semantics that previously
// needed double-checked locking.
type MetricsRegistry struct {
	metrics *safe.Store[string, *ProviderMetrics]
}

// GlobalMetricsRegistry is the singleton metrics registry
var GlobalMetricsRegistry = &MetricsRegistry{
	metrics: safe.NewStore[string, *ProviderMetrics](),
}

// GetMetrics returns metrics for a provider, creating if necessary
func (mr *MetricsRegistry) GetMetrics(providerName string) *ProviderMetrics {
	var result *ProviderMetrics
	mr.metrics.Update(providerName, func(existing *ProviderMetrics, present bool) (*ProviderMetrics, bool) {
		if present {
			result = existing
			return existing, true
		}
		result = NewProviderMetrics()
		return result, true
	})
	return result
}

// RecordRequest records the outcome of a request to the metrics registry
func (mr *MetricsRegistry) RecordRequest(providerName string, success bool, latencyMs int64) {
	pm := mr.GetMetrics(providerName)
	if success {
		pm.RecordSuccess(latencyMs)
	} else {
		pm.RecordFailure()
	}
}

// WeightedStrategy implements weighted load balancing based on performance
type WeightedStrategy struct {
	metricsRegistry *MetricsRegistry
}

// HealthBasedStrategy implements health-based routing
type HealthBasedStrategy struct {
	circuitBreakers *CircuitBreakerPattern
}

// LatencyBasedStrategy implements latency-based routing
type LatencyBasedStrategy struct {
	metricsRegistry *MetricsRegistry
}

// RandomStrategy implements random provider selection
type RandomStrategy struct{}

func NewRequestService(strategy string, ensemble *EnsembleService, memory *MemoryService) *RequestService {
	var routingStrategy RoutingStrategy

	switch strategy {
	case "round_robin":
		routingStrategy = &RoundRobinStrategy{}
	case "weighted":
		routingStrategy = &WeightedStrategy{
			metricsRegistry: GlobalMetricsRegistry,
		}
	case "health_based":
		routingStrategy = &HealthBasedStrategy{
			circuitBreakers: NewCircuitBreakerPattern(),
		}
	case "latency_based":
		routingStrategy = &LatencyBasedStrategy{
			metricsRegistry: GlobalMetricsRegistry,
		}
	case "random":
		routingStrategy = &RandomStrategy{}
	default:
		routingStrategy = &WeightedStrategy{
			metricsRegistry: GlobalMetricsRegistry,
		} // Default
	}

	return &RequestService{
		providers: safe.NewStore[string, LLMProvider](),
		ensemble:  ensemble,
		memory:    memory,
		strategy:  routingStrategy,
	}
}

func (r *RequestService) RegisterProvider(name string, provider LLMProvider) {
	r.providers.Put(name, provider)
}

func (r *RequestService) RemoveProvider(name string) {
	r.providers.Delete(name)
}

func (r *RequestService) GetProviders() []string {
	return r.providers.Keys()
}

func (r *RequestService) ProcessRequest(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	providers := r.providers.Snapshot()

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
	providers := r.providers.Snapshot()

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

	// Track request timing for metrics
	startTime := time.Now()

	// Execute request
	resp, err := provider.Complete(ctx, req)
	latencyMs := time.Since(startTime).Milliseconds()

	// Record metrics
	GlobalMetricsRegistry.RecordRequest(providerName, err == nil, latencyMs)

	// Update circuit breaker for health-based strategy
	if hbs, ok := r.strategy.(*HealthBasedStrategy); ok && hbs.circuitBreakers != nil {
		cb := hbs.circuitBreakers.GetCircuitBreaker(providerName)
		if err != nil {
			cb.mu.Lock()
			cb.FailureCount++
			cb.LastFailTime = time.Now()
			if cb.FailureCount >= cb.FailureThreshold {
				cb.State = RequestStateOpen
			}
			cb.mu.Unlock()
		} else {
			cb.mu.Lock()
			cb.SuccessCount++
			if cb.State == RequestStateHalfOpen {
				cb.State = RequestStateClosed
				cb.FailureCount = 0
			}
			cb.mu.Unlock()
		}
	}

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

	// Track request timing for metrics
	startTime := time.Now()

	// Execute streaming request
	streamChan, err := provider.CompleteStream(ctx, req)
	if err != nil {
		latencyMs := time.Since(startTime).Milliseconds()
		GlobalMetricsRegistry.RecordRequest(providerName, false, latencyMs)
		return nil, fmt.Errorf("provider %s failed: %w", providerName, err)
	}

	// Wrap responses with provider info and record metrics on completion
	wrappedChan := make(chan *models.LLMResponse)
	go func() {
		defer close(wrappedChan)
		hasResponses := false
		for resp := range streamChan {
			hasResponses = true
			resp.ProviderID = providerName
			resp.ProviderName = providerName
			wrappedChan <- resp
		}
		// Record metrics after stream completes
		latencyMs := time.Since(startTime).Milliseconds()
		GlobalMetricsRegistry.RecordRequest(providerName, hasResponses, latencyMs)
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

	// Get the metrics registry (use global if not set)
	registry := s.metricsRegistry
	if registry == nil {
		registry = GlobalMetricsRegistry
	}

	// Calculate dynamic weights based on performance metrics
	weights := make(map[string]float64)
	totalWeight := 0.0

	for name := range providers {
		metrics := registry.GetMetrics(name)

		// Base weight calculation:
		// - Success rate contributes 60% of the weight
		// - Inverse latency contributes 40% (faster = higher weight)
		successRate := metrics.GetSuccessRate()
		avgLatency := metrics.GetAverageLatency()

		// Normalize latency: lower latency = higher score (max 1.0 for latency < 100ms)
		latencyScore := 1.0
		if avgLatency > 0 {
			latencyScore = math.Min(1.0, 1000.0/avgLatency) // 1000ms baseline
		}

		// Calculate composite weight
		weight := (successRate * 0.6) + (latencyScore * 0.4)

		// Ensure minimum weight of 0.1 to give underperforming providers a chance
		weight = math.Max(0.1, weight)

		// Apply preference weights from ensemble config
		if req != nil && req.EnsembleConfig != nil {
			for i, preferred := range req.EnsembleConfig.PreferredProviders {
				if name == preferred {
					// Boost preferred providers by 50-100% based on position
					weight *= 1.5 + (0.5 * (1.0 - float64(i)/float64(len(req.EnsembleConfig.PreferredProviders))))
					break
				}
			}
		}

		weights[name] = weight
		totalWeight += weight
	}

	// Select based on weighted random selection
	// Note: Using math/rand for load balancing is acceptable - it doesn't require cryptographic randomness
	random := rand.Float64() * totalWeight // #nosec G404 - load balancing doesn't require cryptographic randomness
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

	// Get circuit breaker pattern (create if not set)
	cbPattern := s.circuitBreakers
	if cbPattern == nil {
		cbPattern = NewCircuitBreakerPattern()
	}

	// Filter healthy providers based on circuit breaker state and health metrics
	healthyProviders := make([]string, 0)
	halfOpenProviders := make([]string, 0)

	for name := range providers {
		cb := cbPattern.GetCircuitBreaker(name)

		cb.mu.RLock()
		state := cb.State
		lastFailTime := cb.LastFailTime
		recoveryTimeout := cb.RecoveryTimeout
		cb.mu.RUnlock()

		switch state {
		case RequestStateClosed:
			// Provider is healthy
			healthyProviders = append(healthyProviders, name)
		case RequestStateHalfOpen:
			// Provider is recovering, give it a chance
			halfOpenProviders = append(halfOpenProviders, name)
		case RequestStateOpen:
			// Check if enough time has passed to try again
			if time.Since(lastFailTime) >= recoveryTimeout {
				halfOpenProviders = append(halfOpenProviders, name)
			}
			// Otherwise, skip this provider
		}
	}

	// Prefer fully healthy providers
	if len(healthyProviders) > 0 {
		// Select based on success rate from metrics
		registry := GlobalMetricsRegistry
		bestProvider := healthyProviders[0]
		bestScore := 0.0

		for _, name := range healthyProviders {
			metrics := registry.GetMetrics(name)
			score := metrics.GetSuccessRate()
			if score > bestScore {
				bestScore = score
				bestProvider = name
			}
		}
		return bestProvider, nil
	}

	// Fall back to half-open providers if no fully healthy ones
	if len(halfOpenProviders) > 0 {
		// Select randomly among recovering providers - using math/rand is acceptable for load balancing
		return halfOpenProviders[rand.Intn(len(halfOpenProviders))], nil // #nosec G404 - load balancing doesn't require cryptographic randomness
	}

	return "", fmt.Errorf("no healthy providers available")
}

// SetCircuitBreakers sets the circuit breaker pattern for this strategy
func (s *HealthBasedStrategy) SetCircuitBreakers(cb *CircuitBreakerPattern) {
	s.circuitBreakers = cb
}

// LatencyBasedStrategy
func (s *LatencyBasedStrategy) SelectProvider(providers map[string]LLMProvider, req *models.LLMRequest) (string, error) {
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers available")
	}

	// Get the metrics registry (use global if not set)
	registry := s.metricsRegistry
	if registry == nil {
		registry = GlobalMetricsRegistry
	}

	// Find provider with lowest average latency
	var bestProvider string
	lowestLatency := math.MaxFloat64

	// Collect providers with metrics
	providersWithMetrics := make([]string, 0)
	providersWithoutMetrics := make([]string, 0)

	for name := range providers {
		metrics := registry.GetMetrics(name)

		hasHistory := metrics.LatencyHistory.Len() > 0

		if hasHistory {
			avgLatency := metrics.GetAverageLatency()
			providersWithMetrics = append(providersWithMetrics, name) //nolint:staticcheck

			if avgLatency < lowestLatency {
				lowestLatency = avgLatency
				bestProvider = name
			}
		} else {
			// Provider has no latency history yet
			providersWithoutMetrics = append(providersWithoutMetrics, name)
		}
	}

	// If we found a provider with the lowest latency, use it (with some randomization to allow exploration)
	// Note: Using math/rand for load balancing exploration is acceptable - it doesn't require cryptographic randomness
	if bestProvider != "" {
		// 10% of the time, pick a random provider to allow exploration
		// #nosec G404 - load balancing doesn't require cryptographic randomness
		if rand.Float64() < 0.1 {
			// Pick from all providers for exploration
			names := make([]string, 0, len(providers))
			for name := range providers {
				names = append(names, name)
			}
			return names[rand.Intn(len(names))], nil // #nosec G404 - load balancing doesn't require cryptographic randomness
		}
		return bestProvider, nil
	}

	// No providers with latency data yet, prefer those without metrics to build up data
	if len(providersWithoutMetrics) > 0 {
		return providersWithoutMetrics[rand.Intn(len(providersWithoutMetrics))], nil // #nosec G404 - load balancing doesn't require cryptographic randomness
	}

	// Fallback: select randomly from all providers
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	return names[rand.Intn(len(names))], nil // #nosec G404 - load balancing doesn't require cryptographic randomness
}

// RandomStrategy
// Note: Using math/rand for load balancing is acceptable - it doesn't require cryptographic randomness
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

	selected := names[rand.Intn(len(names))] // #nosec G404 - load balancing doesn't require cryptographic randomness
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
	r.providers.Range(func(name string, _ LLMProvider) bool {
		health[name] = &ProviderHealth{
			Name:         name,
			Healthy:      true,
			LastCheck:    time.Now(),
			ResponseTime: 1000,
			SuccessRate:  0.95,
		}
		return true
	})
	return health
}

// Advanced routing features

// CircuitBreakerPattern implements circuit breaker for failing providers.
//
// Concurrent-safe by construction: providers is a safe.Store; Store.Update
// gives atomic get-or-create semantics. Previous sync.RWMutex dropped.
type CircuitBreakerPattern struct {
	providers *safe.Store[string, *RequestCircuitBreaker]
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
		providers: safe.NewStore[string, *RequestCircuitBreaker](),
	}
}

func (c *CircuitBreakerPattern) GetCircuitBreaker(name string) *RequestCircuitBreaker {
	var cb *RequestCircuitBreaker
	c.providers.Update(name, func(existing *RequestCircuitBreaker, present bool) (*RequestCircuitBreaker, bool) {
		if present {
			cb = existing
			return existing, true
		}
		cb = &RequestCircuitBreaker{
			Name:             name,
			State:            RequestStateClosed,
			FailureThreshold: 5,
			Timeout:          60 * time.Second,
			RecoveryTimeout:  30 * time.Second,
		}
		return cb, true
	})
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
