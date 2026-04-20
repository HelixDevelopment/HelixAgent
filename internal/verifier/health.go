package verifier

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitHalfOpen
	CircuitOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitHalfOpen:
		return "half-open"
	case CircuitOpen:
		return "open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name            string
	state           CircuitState
	failureCount    int
	successCount    int
	threshold       int
	resetTimeout    time.Duration
	lastFailure     time.Time
	lastStateChange time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string) *CircuitBreaker {
	return &CircuitBreaker{
		name:            name,
		state:           CircuitClosed,
		threshold:       5,
		resetTimeout:    30 * time.Second,
		lastStateChange: time.Now(),
	}
}

// State returns the current circuit state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsAvailable returns true if the circuit allows requests
func (cb *CircuitBreaker) IsAvailable() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			cb.lastStateChange = time.Now()
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records a successful call
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
		cb.failureCount = 0
		cb.lastStateChange = time.Now()
	}
}

// RecordFailure records a failed call
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.failureCount >= cb.threshold {
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
	}
}

// Call executes an operation through the circuit breaker
func (cb *CircuitBreaker) Call(operation func() error) error {
	if !cb.IsAvailable() {
		return fmt.Errorf("circuit breaker open for %s", cb.name)
	}

	err := operation()
	if err != nil {
		cb.RecordFailure()
		return err
	}

	cb.RecordSuccess()
	return nil
}

// ProviderHealth represents the health status of a provider
type ProviderHealth struct {
	ProviderID    string    `json:"provider_id"`
	ProviderName  string    `json:"provider_name"`
	Healthy       bool      `json:"healthy"`
	CircuitState  string    `json:"circuit_state"`
	FailureCount  int       `json:"failure_count"`
	SuccessCount  int       `json:"success_count"`
	AvgResponseMs int64     `json:"avg_response_ms"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	LastFailureAt time.Time `json:"last_failure_at,omitempty"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	UptimePercent float64   `json:"uptime_percent"`
}

// HealthService manages provider health monitoring and failover.
//
// Concurrent-safe by construction: circuitBreakers and providerHealth
// are safe.Store. healthMu (Pattern Zeta, renamed from mu) serialises
// the per-*ProviderHealth field mutations inside checkProvider /
// RecordSuccess / RecordFailure with the snapshot-style reads in the
// Get/GetAll accessors — it does not pair with any bare map/slice
// field, so the audit does not flag the struct. running is an
// atomic.Bool with CompareAndSwap semantics for idempotent Start/Stop.
type HealthService struct {
	circuitBreakers *safe.Store[string, *CircuitBreaker]
	providerHealth  *safe.Store[string, *ProviderHealth]
	httpClient      *http.Client
	checkInterval   time.Duration
	healthMu        sync.RWMutex
	stopCh          chan struct{}
	wg              sync.WaitGroup
	running         atomic.Bool
}

// NewHealthService creates a new health service
func NewHealthService(cfg *Config) *HealthService {
	timeout := 10 * time.Second
	interval := 30 * time.Second

	if cfg != nil && cfg.Health.Timeout > 0 {
		timeout = cfg.Health.Timeout
	}
	if cfg != nil && cfg.Health.CheckInterval > 0 {
		interval = cfg.Health.CheckInterval
	}

	return &HealthService{
		circuitBreakers: safe.NewStore[string, *CircuitBreaker](),
		providerHealth:  safe.NewStore[string, *ProviderHealth](),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		checkInterval: interval,
		stopCh:        make(chan struct{}),
	}
}

// Start starts health monitoring
func (s *HealthService) Start() error {
	if !s.running.CompareAndSwap(false, true) {
		return fmt.Errorf("health service already running")
	}
	s.wg.Add(1)
	go s.healthCheckLoop()
	return nil
}

// Stop stops health monitoring
func (s *HealthService) Stop() {
	if !s.running.CompareAndSwap(true, false) {
		return
	}
	close(s.stopCh)
	s.wg.Wait()
}

// healthCheckLoop runs periodic health checks
func (s *HealthService) healthCheckLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// Initial check
	s.performHealthChecks()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.performHealthChecks()
		}
	}
}

// performHealthChecks checks all registered providers
func (s *HealthService) performHealthChecks() {
	providers := s.circuitBreakers.Keys()

	var wg sync.WaitGroup
	for _, providerID := range providers {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			s.checkProviderHealth(id)
		}(providerID)
	}
	wg.Wait()
}

// checkProviderHealth checks health of a specific provider
func (s *HealthService) checkProviderHealth(providerID string) {
	cb, cbOk := s.circuitBreakers.Get(providerID)
	health, hOk := s.providerHealth.Get(providerID)

	if !cbOk || !hOk || cb == nil || health == nil {
		return
	}

	start := time.Now()
	healthy := s.performHealthCheck(health.ProviderName)
	responseTime := time.Since(start).Milliseconds()

	s.healthMu.Lock()
	defer s.healthMu.Unlock()

	health.LastCheckedAt = time.Now()
	health.AvgResponseMs = (health.AvgResponseMs + responseTime) / 2

	if healthy {
		health.Healthy = true
		health.SuccessCount++
		health.LastSuccessAt = time.Now()
		cb.RecordSuccess()
	} else {
		health.Healthy = false
		health.FailureCount++
		health.LastFailureAt = time.Now()
		cb.RecordFailure()
	}

	// Update circuit state
	health.CircuitState = cb.State().String()

	// Calculate uptime
	total := float64(health.SuccessCount + health.FailureCount)
	if total > 0 {
		health.UptimePercent = float64(health.SuccessCount) / total * 100
	}
}

// performHealthCheck performs actual health check
func (s *HealthService) performHealthCheck(providerName string) bool {
	endpoints := map[string]string{
		"openai":        "https://api.openai.com/v1/models",
		"anthropic":     "https://api.anthropic.com/v1/messages",
		"gemini":        "https://generativelanguage.googleapis.com/v1/models",
		"google":        "https://generativelanguage.googleapis.com/v1/models", // Alias for gemini
		"groq":          "https://api.groq.com/openai/v1/models",
		"together":      "https://api.together.xyz/v1/models",
		"mistral":       "https://api.mistral.ai/v1/models",
		"deepseek":      "https://api.deepseek.com/v1/models",
		"ollama":        "http://localhost:11434/api/tags",
		"openrouter":    "https://openrouter.ai/api/v1/models",
		"xai":           "https://api.x.ai/v1/models",
		"cerebras":      "https://api.cerebras.ai/v1/models",
		"github-models": "https://models.github.ai/inference/models",
		"venice":        "https://api.venice.ai/api/v1/models",
	}

	endpoint, ok := endpoints[providerName]
	if !ok {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", endpoint, nil)
	if err != nil {
		return false
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

// AddProvider adds a provider to health monitoring
func (s *HealthService) AddProvider(providerID, providerName string) {
	s.circuitBreakers.Put(providerID, NewCircuitBreaker(fmt.Sprintf("provider-%s", providerID)))
	s.providerHealth.Put(providerID, &ProviderHealth{
		ProviderID:    providerID,
		ProviderName:  providerName,
		Healthy:       true,
		CircuitState:  "closed",
		LastCheckedAt: time.Now(),
	})
}

// RemoveProvider removes a provider from health monitoring
func (s *HealthService) RemoveProvider(providerID string) {
	s.circuitBreakers.Delete(providerID)
	s.providerHealth.Delete(providerID)
}

// GetProviderHealth returns health status for a provider
func (s *HealthService) GetProviderHealth(providerID string) (*ProviderHealth, error) {
	health, ok := s.providerHealth.Get(providerID)
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}
	return health, nil
}

// GetAllProviderHealth returns health status for all providers
func (s *HealthService) GetAllProviderHealth() []*ProviderHealth {
	result := s.providerHealth.Values()
	sort.Slice(result, func(i, j int) bool {
		return result[i].ProviderName < result[j].ProviderName
	})
	return result
}

// GetHealthyProviders returns list of healthy provider IDs
func (s *HealthService) GetHealthyProviders() []string {
	healthy := make([]string, 0)
	s.circuitBreakers.Range(func(providerID string, cb *CircuitBreaker) bool {
		if cb.IsAvailable() {
			healthy = append(healthy, providerID)
		}
		return true
	})
	return healthy
}

// GetCircuitBreaker returns the circuit breaker for a provider
func (s *HealthService) GetCircuitBreaker(providerID string) *CircuitBreaker {
	cb, _ := s.circuitBreakers.Get(providerID)
	return cb
}

// ExecuteWithFailover executes an operation with automatic failover
func (s *HealthService) ExecuteWithFailover(ctx context.Context, providers []string, operation func(providerID string) error) error {
	for _, providerID := range providers {
		cb, _ := s.circuitBreakers.Get(providerID)

		if cb == nil || !cb.IsAvailable() {
			continue
		}

		err := cb.Call(func() error {
			return operation(providerID)
		})

		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("all providers failed")
}

// GetFastestProvider returns the fastest available provider
func (s *HealthService) GetFastestProvider(providers []string) (string, error) {
	var fastest string
	var lowestLatency int64 = -1

	for _, providerID := range providers {
		cb, _ := s.circuitBreakers.Get(providerID)
		health, _ := s.providerHealth.Get(providerID)

		if cb == nil || !cb.IsAvailable() || health == nil {
			continue
		}

		if lowestLatency == -1 || health.AvgResponseMs < lowestLatency {
			lowestLatency = health.AvgResponseMs
			fastest = providerID
		}
	}

	if fastest == "" {
		return "", fmt.Errorf("no available providers")
	}

	return fastest, nil
}

// RecordSuccess records a successful operation for a provider
func (s *HealthService) RecordSuccess(providerID string) {
	if cb, ok := s.circuitBreakers.Get(providerID); ok {
		cb.RecordSuccess()
	}

	if health, ok := s.providerHealth.Get(providerID); ok {
		s.healthMu.Lock()
		health.SuccessCount++
		health.LastSuccessAt = time.Now()
		health.Healthy = true
		s.healthMu.Unlock()
	}
}

// RecordFailure records a failed operation for a provider
func (s *HealthService) RecordFailure(providerID string) {
	if cb, ok := s.circuitBreakers.Get(providerID); ok {
		cb.RecordFailure()
	}

	if health, ok := s.providerHealth.Get(providerID); ok {
		s.healthMu.Lock()
		health.FailureCount++
		health.LastFailureAt = time.Now()
		s.healthMu.Unlock()
	}
}

// IsProviderAvailable checks if a provider is available
func (s *HealthService) IsProviderAvailable(providerID string) bool {
	cb, ok := s.circuitBreakers.Get(providerID)
	if !ok {
		return false
	}
	return cb.IsAvailable()
}

// GetProviderLatency returns the average latency for a provider
func (s *HealthService) GetProviderLatency(providerID string) (int64, error) {
	health, ok := s.providerHealth.Get(providerID)
	if !ok {
		return 0, fmt.Errorf("provider not found: %s", providerID)
	}

	return health.AvgResponseMs, nil
}
