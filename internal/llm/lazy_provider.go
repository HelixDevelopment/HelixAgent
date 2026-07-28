package llm

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/adapters"
	"dev.helix.agent/internal/models"
)

// ProviderFactory is a function that creates an LLMProvider
type ProviderFactory func() (LLMProvider, error)

// LazyProviderConfig holds configuration for lazy provider initialization
type LazyProviderConfig struct {
	// InitTimeout is the maximum time to wait for initialization
	InitTimeout time.Duration
	// RetryAttempts is the number of times to retry initialization
	RetryAttempts int
	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration
	// PrewarmOnAccess triggers background warm-up after first access
	PrewarmOnAccess bool
	// EventBus for publishing provider events
	EventBus *adapters.EventBus
}

// DefaultLazyProviderConfig returns default configuration
func DefaultLazyProviderConfig() *LazyProviderConfig {
	return &LazyProviderConfig{
		InitTimeout:     30 * time.Second,
		RetryAttempts:   3,
		RetryDelay:      1 * time.Second,
		PrewarmOnAccess: false,
	}
}

// LazyProviderMetrics tracks lazy provider statistics
type LazyProviderMetrics struct {
	InitializationCount   int64
	InitializationLatency int64 // in microseconds
	InitializationErrors  int64
	AccessCount           int64
	LastAccessTime        int64 // unix nano
	LastInitTime          int64 // unix nano
}

// LazyProvider wraps a provider with lazy initialization
type LazyProvider struct {
	name        string
	factory     ProviderFactory
	config      *LazyProviderConfig
	provider    LLMProvider
	initErr     error
	initTime    time.Duration
	once        sync.Once
	mu          sync.RWMutex
	initialized bool
	metrics     *LazyProviderMetrics
}

// NewLazyProvider creates a new lazy provider
func NewLazyProvider(name string, factory ProviderFactory, config *LazyProviderConfig) *LazyProvider {
	if config == nil {
		config = DefaultLazyProviderConfig()
	}

	return &LazyProvider{
		name:    name,
		factory: factory,
		config:  config,
		metrics: &LazyProviderMetrics{},
	}
}

// Get returns the provider, initializing on first call
func (p *LazyProvider) Get() (LLMProvider, error) {
	atomic.AddInt64(&p.metrics.AccessCount, 1)
	atomic.StoreInt64(&p.metrics.LastAccessTime, time.Now().UnixNano())

	p.once.Do(func() {
		p.initialize()
	})

	p.mu.RLock()
	provider := p.provider
	err := p.initErr
	p.mu.RUnlock()

	return provider, err
}

// initialize performs the actual provider initialization with retries
func (p *LazyProvider) initialize() {
	startTime := time.Now()
	atomic.AddInt64(&p.metrics.InitializationCount, 1)

	var lastErr error

	for attempt := 1; attempt <= p.config.RetryAttempts; attempt++ {
		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), p.config.InitTimeout)

		// Try to create the provider
		provider, err := p.createProviderWithContext(ctx)
		cancel()

		if err == nil {
			p.mu.Lock()
			p.provider = provider
			p.initialized = true
			p.initTime = time.Since(startTime)
			p.mu.Unlock()

			atomic.StoreInt64(&p.metrics.InitializationLatency, p.initTime.Microseconds())
			atomic.StoreInt64(&p.metrics.LastInitTime, time.Now().UnixNano())

			// Publish success event
			if p.config.EventBus != nil {
				p.config.EventBus.Publish(adapters.NewEvent(
					adapters.EventProviderRegistered,
					"lazy_provider",
					map[string]interface{}{
						"name":     p.name,
						"duration": p.initTime.String(),
						"attempt":  attempt,
					},
				))
			}

			return
		}

		lastErr = err
		atomic.AddInt64(&p.metrics.InitializationErrors, 1)

		// Don't retry on last attempt
		if attempt < p.config.RetryAttempts {
			time.Sleep(p.config.RetryDelay)
		}
	}

	// All attempts failed
	p.mu.Lock()
	p.initErr = fmt.Errorf("failed to initialize provider after %d attempts: %w", p.config.RetryAttempts, lastErr)
	p.initTime = time.Since(startTime)
	p.mu.Unlock()

	// Publish failure event
	if p.config.EventBus != nil {
		p.config.EventBus.Publish(adapters.NewEvent(
			adapters.EventProviderHealthChanged,
			"lazy_provider",
			map[string]interface{}{
				"name":   p.name,
				"error":  lastErr.Error(),
				"health": false,
			},
		))
	}
}

// createProviderWithContext creates the provider with context awareness.
//
// Lifecycle: the factory runs in its own goroutine so context cancellation
// does not block on a slow factory. Results are posted to a buffered
// channel so the goroutine can always complete its send and exit cleanly
// even if the parent has already returned on ctx cancel — this avoids the
// data race that the previous implementation had by writing to
// outer-scope vars from the goroutine.
//
// Known limitation: if p.factory() hangs forever, the goroutine will
// outlive the ctx-cancelled parent. That is an unavoidable cost of the
// factory contract (p.factory is a caller-provided closure without a
// context parameter) — we cannot forcibly kill a goroutine in Go. The
// fix here limits the damage to one goroutine per lazy-init attempt and
// removes the data race, which were the concrete bugs.
func (p *LazyProvider) createProviderWithContext(ctx context.Context) (LLMProvider, error) {
	type result struct {
		provider LLMProvider
		err      error
	}
	// Buffered: the sender always succeeds so the goroutine exits even if
	// the receiver (this function) has already returned on ctx cancel.
	done := make(chan result, 1)

	go func() {
		prov, err := p.factory()
		done <- result{provider: prov, err: err}
	}()

	// Priority pre-check: Go's select chooses UNIFORMLY AT RANDOM among all
	// ready cases. If scheduler delay lets both ctx.Done() and done become
	// ready before this goroutine is scheduled, a naked select below could
	// pick the stale factory result even though the caller already
	// cancelled/timed-out. A non-blocking check on ctx.Done() first ensures
	// cancellation always wins when it has already fired.
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("initialization timed out: %w", ctx.Err())
	default:
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("initialization timed out: %w", ctx.Err())
	case r := <-done:
		// Second, immediate re-check: the blocking select above is itself a
		// scheduling/yield point (it can park waiting for either channel),
		// so ctx can be cancelled/expire and the factory result can arrive
		// together WHILE it is parked, letting the random pick land on
		// `done` anyway. Re-checking ctx.Done() here, with no further yield
		// point in between, closes that window: a closed Done channel stays
		// ready forever, so any cancellation that raced the select above is
		// still visible here, right before the stale result is returned.
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("initialization timed out: %w", ctx.Err())
		default:
		}
		return r.provider, r.err
	}
}

// IsInitialized returns whether the provider has been initialized
func (p *LazyProvider) IsInitialized() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initialized
}

// InitializationTime returns the time taken to initialize
func (p *LazyProvider) InitializationTime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initTime
}

// Error returns any initialization error
func (p *LazyProvider) Error() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initErr
}

// Name returns the provider name
func (p *LazyProvider) Name() string {
	return p.name
}

// Metrics returns provider metrics
func (p *LazyProvider) Metrics() *LazyProviderMetrics {
	return &LazyProviderMetrics{
		InitializationCount:   atomic.LoadInt64(&p.metrics.InitializationCount),
		InitializationLatency: atomic.LoadInt64(&p.metrics.InitializationLatency),
		InitializationErrors:  atomic.LoadInt64(&p.metrics.InitializationErrors),
		AccessCount:           atomic.LoadInt64(&p.metrics.AccessCount),
		LastAccessTime:        atomic.LoadInt64(&p.metrics.LastAccessTime),
		LastInitTime:          atomic.LoadInt64(&p.metrics.LastInitTime),
	}
}

// Reset resets the lazy provider to allow re-initialization
// This should be used carefully as it may cause race conditions
func (p *LazyProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.provider = nil
	p.initErr = nil
	p.initialized = false
	p.initTime = 0
	p.once = sync.Once{}
}

// LLMProvider interface implementation - forwards to underlying provider

// Complete forwards the request to the underlying provider
func (p *LazyProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	provider, err := p.Get()
	if err != nil {
		return nil, fmt.Errorf("provider not available: %w", err)
	}
	return provider.Complete(ctx, req)
}

// CompleteStream forwards the streaming request to the underlying provider
func (p *LazyProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	provider, err := p.Get()
	if err != nil {
		return nil, fmt.Errorf("provider not available: %w", err)
	}
	return provider.CompleteStream(ctx, req)
}

// HealthCheck forwards the health check to the underlying provider
func (p *LazyProvider) HealthCheck() error {
	provider, err := p.Get()
	if err != nil {
		return fmt.Errorf("provider not available: %w", err)
	}
	return provider.HealthCheck()
}

// GetCapabilities returns capabilities, initializing provider if needed
func (p *LazyProvider) GetCapabilities() *models.ProviderCapabilities {
	provider, err := p.Get()
	if err != nil {
		return &models.ProviderCapabilities{}
	}
	return provider.GetCapabilities()
}

// ValidateConfig forwards config validation to the underlying provider
func (p *LazyProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	provider, err := p.Get()
	if err != nil {
		return false, []string{fmt.Sprintf("provider not available: %v", err)}
	}
	return provider.ValidateConfig(config)
}

// LazyProviderRegistry manages multiple lazy providers.
//
// Concurrent-safe by construction (CONST-029): providers is a safe.Store.
// LazyProvider has its own internal sync once-init (no collection
// adjacency in its own struct — audit-clean).
type LazyProviderRegistry struct {
	providers *safe.Store[string, *LazyProvider]
	config    *LazyProviderConfig
	eventBus  *adapters.EventBus
}

// NewLazyProviderRegistry creates a new lazy provider registry
func NewLazyProviderRegistry(config *LazyProviderConfig, eventBus *adapters.EventBus) *LazyProviderRegistry {
	if config == nil {
		config = DefaultLazyProviderConfig()
	}

	return &LazyProviderRegistry{
		providers: safe.NewStore[string, *LazyProvider](),
		config:    config,
		eventBus:  eventBus,
	}
}

// Register registers a provider factory
func (r *LazyProviderRegistry) Register(name string, factory ProviderFactory) {
	config := *r.config
	config.EventBus = r.eventBus

	r.providers.Put(name, NewLazyProvider(name, factory, &config))
}

// Get returns a lazy provider by name
func (r *LazyProviderRegistry) Get(name string) (*LazyProvider, bool) {
	return r.providers.Get(name)
}

// GetProvider returns the initialized provider by name
func (r *LazyProviderRegistry) GetProvider(name string) (LLMProvider, error) {
	lazy, ok := r.providers.Get(name)
	if !ok {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return lazy.Get()
}

// List returns all registered provider names
func (r *LazyProviderRegistry) List() []string {
	return r.providers.Keys()
}

// InitializedProviders returns names of initialized providers
func (r *LazyProviderRegistry) InitializedProviders() []string {
	names := make([]string, 0)
	r.providers.Range(func(name string, provider *LazyProvider) bool {
		if provider.IsInitialized() {
			names = append(names, name)
		}
		return true
	})
	return names
}

// Preload initializes specified providers in background
func (r *LazyProviderRegistry) Preload(ctx context.Context, names ...string) error {
	toInit := make([]*LazyProvider, 0, len(names))
	for _, name := range names {
		if provider, ok := r.providers.Get(name); ok {
			toInit = append(toInit, provider)
		}
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(toInit))

	for _, provider := range toInit {
		wg.Add(1)
		go func(p *LazyProvider) {
			defer wg.Done()
			if _, err := p.Get(); err != nil {
				errChan <- fmt.Errorf("failed to preload %s: %w", p.Name(), err)
			}
		}(provider)
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d providers failed to preload", len(errors))
	}

	return nil
}

// PreloadAll initializes all providers in background
func (r *LazyProviderRegistry) PreloadAll(ctx context.Context) error {
	return r.Preload(ctx, r.List()...)
}

// Reset resets all providers
func (r *LazyProviderRegistry) Reset() {
	r.providers.Range(func(_ string, provider *LazyProvider) bool {
		provider.Reset()
		return true
	})
}
