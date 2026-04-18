package llm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	events "dev.helix.agent/internal/adapters"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lazyMockProvider is a test implementation of LLMProvider for lazy provider tests
type lazyMockProvider struct {
	name            string
	response        *models.LLMResponse
	err             error
	healthErr       error
	capabilities    *models.ProviderCapabilities
	validateResult  bool
	validateErrors  []string
	callCount       int32
	streamResponses []*models.LLMResponse
}

func (m *lazyMockProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	atomic.AddInt32(&m.callCount, 1)
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *lazyMockProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan *models.LLMResponse, len(m.streamResponses))
	for _, resp := range m.streamResponses {
		ch <- resp
	}
	close(ch)
	return ch, nil
}

func (m *lazyMockProvider) HealthCheck() error {
	return m.healthErr
}

func (m *lazyMockProvider) GetCapabilities() *models.ProviderCapabilities {
	if m.capabilities != nil {
		return m.capabilities
	}
	return &models.ProviderCapabilities{
		SupportsStreaming: true,
		SupportsTools:     true,
	}
}

func (m *lazyMockProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return m.validateResult, m.validateErrors
}

func TestDefaultLazyProviderConfig(t *testing.T) {
	config := DefaultLazyProviderConfig()

	assert.Equal(t, 30*time.Second, config.InitTimeout)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.False(t, config.PrewarmOnAccess)
	assert.Nil(t, config.EventBus)
}

func TestNewLazyProvider(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		factory := func() (LLMProvider, error) {
			return &lazyMockProvider{}, nil
		}

		lazy := NewLazyProvider("test-provider", factory, nil)

		assert.Equal(t, "test-provider", lazy.Name())
		assert.False(t, lazy.IsInitialized())
		assert.Nil(t, lazy.Error())
		assert.Equal(t, time.Duration(0), lazy.InitializationTime())
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &LazyProviderConfig{
			InitTimeout:     10 * time.Second,
			RetryAttempts:   5,
			RetryDelay:      500 * time.Millisecond,
			PrewarmOnAccess: true,
		}

		factory := func() (LLMProvider, error) {
			return &lazyMockProvider{}, nil
		}

		lazy := NewLazyProvider("custom-provider", factory, config)

		assert.Equal(t, "custom-provider", lazy.Name())
		assert.False(t, lazy.IsInitialized())
	})
}

func TestLazyProvider_Get_Success(t *testing.T) {
	mockProvider := &lazyMockProvider{
		name: "mock",
		response: &models.LLMResponse{
			Content:    "test response",
			Confidence: 0.9,
		},
	}

	factory := func() (LLMProvider, error) {
		return mockProvider, nil
	}

	lazy := NewLazyProvider("test", factory, nil)

	// First call initializes
	provider, err := lazy.Get()
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.True(t, lazy.IsInitialized())
	assert.NoError(t, lazy.Error())
	assert.Greater(t, lazy.InitializationTime(), time.Duration(0))

	// Second call returns same provider
	provider2, err := lazy.Get()
	require.NoError(t, err)
	assert.Equal(t, provider, provider2)

	// Check metrics
	metrics := lazy.Metrics()
	assert.Equal(t, int64(2), metrics.AccessCount)
	assert.Equal(t, int64(1), metrics.InitializationCount)
	assert.Equal(t, int64(0), metrics.InitializationErrors)
}

func TestLazyProvider_Get_FactoryError(t *testing.T) {
	factoryErr := errors.New("factory failed")
	callCount := 0

	factory := func() (LLMProvider, error) {
		callCount++
		return nil, factoryErr
	}

	config := &LazyProviderConfig{
		InitTimeout:   1 * time.Second,
		RetryAttempts: 2,
		RetryDelay:    10 * time.Millisecond,
	}

	lazy := NewLazyProvider("failing", factory, config)

	provider, err := lazy.Get()
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "failed to initialize provider")
	assert.False(t, lazy.IsInitialized())

	// Should have retried
	assert.Equal(t, 2, callCount)

	// Check metrics
	metrics := lazy.Metrics()
	assert.Equal(t, int64(2), metrics.InitializationErrors)
}

func TestLazyProvider_Get_Timeout(t *testing.T) {
	factory := func() (LLMProvider, error) {
		time.Sleep(200 * time.Millisecond)
		return &lazyMockProvider{}, nil
	}

	config := &LazyProviderConfig{
		InitTimeout:   50 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	lazy := NewLazyProvider("slow", factory, config)

	provider, err := lazy.Get()
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "timed out")
}

// TestLazyProvider_Get_TimeoutRaceFree is a regression for BUGFIX #15.
// The previous implementation of createProviderWithContext shared mutable
// `provider`/`err` variables between the main goroutine and the factory
// goroutine; on ctx timeout the main goroutine returned nil while the
// factory later wrote to the same slot — a data race caught by -race.
// The fixed implementation posts a result struct through a buffered
// channel, so the factory writes only to its own locals.
func TestLazyProvider_Get_TimeoutRaceFree(t *testing.T) {
	factoryDone := make(chan struct{})
	factory := func() (LLMProvider, error) {
		// Sleep past the timeout then write a value. On the old code this
		// race-tripped when the main goroutine read the shared var.
		time.Sleep(100 * time.Millisecond)
		defer close(factoryDone)
		return &lazyMockProvider{name: "eventually-ready"}, nil
	}

	config := &LazyProviderConfig{
		InitTimeout:   20 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    5 * time.Millisecond,
	}
	lazy := NewLazyProvider("slow-race", factory, config)

	prov, err := lazy.Get()
	require.Error(t, err)
	assert.Nil(t, prov)
	assert.Contains(t, err.Error(), "timed out")

	// Wait for the orphaned factory goroutine to finish — ensures go
	// race detector saw all writes and did not flag a hazard.
	select {
	case <-factoryDone:
	case <-time.After(2 * time.Second):
		t.Fatal("factory goroutine never completed")
	}
}

// TestLazyProvider_Get_AfterTimeoutRetrySucceeds covers the second
// concern of BUGFIX #15 — a failed-on-timeout LazyProvider must still be
// usable for subsequent Get() calls once a faster factory path is
// available (e.g. under retry config). The orphaned factory from the
// first attempt must not corrupt internal state.
func TestLazyProvider_Get_AfterTimeoutRetrySucceeds(t *testing.T) {
	var callCount int32
	factory := func() (LLMProvider, error) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			time.Sleep(80 * time.Millisecond) // will be timed out
			return &lazyMockProvider{name: "slow"}, nil
		}
		return &lazyMockProvider{name: "fast"}, nil
	}

	config := &LazyProviderConfig{
		InitTimeout:   20 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    5 * time.Millisecond,
	}
	lazy := NewLazyProvider("retry-me", factory, config)

	// First Get should time out.
	_, err := lazy.Get()
	require.Error(t, err)

	// Give the orphaned first-call factory goroutine time to exit.
	time.Sleep(120 * time.Millisecond)

	// Reset to allow another attempt.
	lazy.Reset()

	// Second Get should succeed.
	prov, err := lazy.Get()
	require.NoError(t, err)
	require.NotNil(t, prov)
}

func TestLazyProvider_Complete(t *testing.T) {
	expectedResponse := &models.LLMResponse{
		Content:    "Hello from lazy provider",
		Confidence: 0.95,
	}

	mockProvider := &lazyMockProvider{
		response: expectedResponse,
	}

	factory := func() (LLMProvider, error) {
		return mockProvider, nil
	}

	lazy := NewLazyProvider("test", factory, nil)

	req := &models.LLMRequest{
		ID:     "req-1",
		Prompt: "Hello",
	}

	resp, err := lazy.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.Content, resp.Content)
	assert.Equal(t, int32(1), atomic.LoadInt32(&mockProvider.callCount))
}

func TestLazyProvider_Complete_ProviderNotAvailable(t *testing.T) {
	factory := func() (LLMProvider, error) {
		return nil, errors.New("provider unavailable")
	}

	config := &LazyProviderConfig{
		InitTimeout:   100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	lazy := NewLazyProvider("unavailable", factory, config)

	req := &models.LLMRequest{ID: "req-1"}
	resp, err := lazy.Complete(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "provider not available")
}

func TestLazyProvider_CompleteStream(t *testing.T) {
	streamResponses := []*models.LLMResponse{
		{Content: "chunk1"},
		{Content: "chunk2"},
		{Content: "chunk3"},
	}

	mockProvider := &lazyMockProvider{
		streamResponses: streamResponses,
	}

	factory := func() (LLMProvider, error) {
		return mockProvider, nil
	}

	lazy := NewLazyProvider("stream-test", factory, nil)

	req := &models.LLMRequest{ID: "stream-req"}
	ch, err := lazy.CompleteStream(context.Background(), req)
	require.NoError(t, err)

	var received []*models.LLMResponse
	for resp := range ch {
		received = append(received, resp)
	}

	assert.Len(t, received, 3)
}

func TestLazyProvider_CompleteStream_ProviderNotAvailable(t *testing.T) {
	factory := func() (LLMProvider, error) {
		return nil, errors.New("unavailable")
	}

	config := &LazyProviderConfig{
		InitTimeout:   100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	lazy := NewLazyProvider("unavailable", factory, config)

	req := &models.LLMRequest{ID: "req-1"}
	ch, err := lazy.CompleteStream(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "provider not available")
}

func TestLazyProvider_HealthCheck(t *testing.T) {
	mockProvider := &lazyMockProvider{
		healthErr: nil,
	}

	factory := func() (LLMProvider, error) {
		return mockProvider, nil
	}

	lazy := NewLazyProvider("health-test", factory, nil)

	err := lazy.HealthCheck()
	assert.NoError(t, err)

	// Test with health error
	mockProvider.healthErr = errors.New("unhealthy")
	err = lazy.HealthCheck()
	assert.Error(t, err)
}

func TestLazyProvider_HealthCheck_ProviderNotAvailable(t *testing.T) {
	factory := func() (LLMProvider, error) {
		return nil, errors.New("unavailable")
	}

	config := &LazyProviderConfig{
		InitTimeout:   100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	lazy := NewLazyProvider("unavailable", factory, config)

	err := lazy.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider not available")
}

func TestLazyProvider_GetCapabilities(t *testing.T) {
	expectedCaps := &models.ProviderCapabilities{
		SupportsStreaming: true,
		SupportsTools:     true,
		SupportsVision:    true,
	}

	mockProvider := &lazyMockProvider{
		capabilities: expectedCaps,
	}

	factory := func() (LLMProvider, error) {
		return mockProvider, nil
	}

	lazy := NewLazyProvider("caps-test", factory, nil)

	caps := lazy.GetCapabilities()
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsVision)
}

func TestLazyProvider_GetCapabilities_ProviderNotAvailable(t *testing.T) {
	factory := func() (LLMProvider, error) {
		return nil, errors.New("unavailable")
	}

	config := &LazyProviderConfig{
		InitTimeout:   100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	lazy := NewLazyProvider("unavailable", factory, config)

	caps := lazy.GetCapabilities()
	// Should return empty capabilities
	assert.NotNil(t, caps)
}

func TestLazyProvider_ValidateConfig(t *testing.T) {
	mockProvider := &lazyMockProvider{
		validateResult: true,
		validateErrors: nil,
	}

	factory := func() (LLMProvider, error) {
		return mockProvider, nil
	}

	lazy := NewLazyProvider("validate-test", factory, nil)

	valid, errs := lazy.ValidateConfig(map[string]interface{}{"key": "value"})
	assert.True(t, valid)
	assert.Empty(t, errs)
}

func TestLazyProvider_ValidateConfig_ProviderNotAvailable(t *testing.T) {
	factory := func() (LLMProvider, error) {
		return nil, errors.New("unavailable")
	}

	config := &LazyProviderConfig{
		InitTimeout:   100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}

	lazy := NewLazyProvider("unavailable", factory, config)

	valid, errs := lazy.ValidateConfig(nil)
	assert.False(t, valid)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "provider not available")
}

func TestLazyProvider_Reset(t *testing.T) {
	callCount := 0
	factory := func() (LLMProvider, error) {
		callCount++
		return &lazyMockProvider{}, nil
	}

	lazy := NewLazyProvider("reset-test", factory, nil)

	// Initialize
	_, err := lazy.Get()
	require.NoError(t, err)
	assert.True(t, lazy.IsInitialized())
	assert.Equal(t, 1, callCount)

	// Reset
	lazy.Reset()
	assert.False(t, lazy.IsInitialized())
	assert.Nil(t, lazy.Error())
	assert.Equal(t, time.Duration(0), lazy.InitializationTime())

	// Re-initialize
	_, err = lazy.Get()
	require.NoError(t, err)
	assert.True(t, lazy.IsInitialized())
	assert.Equal(t, 2, callCount)
}

func TestLazyProvider_WithEventBus(t *testing.T) {
	eventBus := events.NewEventBus(nil)
	eventChan := eventBus.Subscribe(events.EventProviderRegistered)

	config := &LazyProviderConfig{
		InitTimeout:   1 * time.Second,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
		EventBus:      eventBus,
	}

	mockProvider := &lazyMockProvider{}
	factory := func() (LLMProvider, error) {
		return mockProvider, nil
	}

	lazy := NewLazyProvider("event-test", factory, config)

	// Initialize - should publish success event
	_, err := lazy.Get()
	require.NoError(t, err)

	// Wait for event with timeout
	select {
	case event := <-eventChan:
		assert.Equal(t, events.EventProviderRegistered, event.Type)
		// Check payload contains expected data
		payload, ok := event.Payload.(map[string]interface{})
		if ok {
			assert.Equal(t, "event-test", payload["name"])
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected event not received")
	}
}

func TestLazyProvider_WithEventBus_Failure(t *testing.T) {
	eventBus := events.NewEventBus(nil)
	eventChan := eventBus.Subscribe(events.EventProviderHealthChanged)

	config := &LazyProviderConfig{
		InitTimeout:   100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
		EventBus:      eventBus,
	}

	factory := func() (LLMProvider, error) {
		return nil, errors.New("initialization failed")
	}

	lazy := NewLazyProvider("fail-event-test", factory, config)

	// Initialize - should fail and publish failure event
	_, err := lazy.Get()
	assert.Error(t, err)

	// Wait for event with timeout
	select {
	case event := <-eventChan:
		assert.Equal(t, events.EventProviderHealthChanged, event.Type)
		payload, ok := event.Payload.(map[string]interface{})
		if ok {
			assert.Equal(t, "fail-event-test", payload["name"])
			assert.Equal(t, false, payload["health"])
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected failure event not received")
	}
}

func TestLazyProvider_ConcurrentAccess(t *testing.T) {
	initCount := int32(0)
	factory := func() (LLMProvider, error) {
		atomic.AddInt32(&initCount, 1)
		time.Sleep(50 * time.Millisecond) // Simulate slow init
		return &lazyMockProvider{}, nil
	}

	lazy := NewLazyProvider("concurrent-test", factory, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			provider, err := lazy.Get()
			assert.NoError(t, err)
			assert.NotNil(t, provider)
		}()
	}

	wg.Wait()

	// Should only initialize once despite concurrent calls
	assert.Equal(t, int32(1), atomic.LoadInt32(&initCount))
	assert.Equal(t, int64(100), lazy.Metrics().AccessCount)
}

// ============================================================================
// LazyProviderRegistry Tests
// ============================================================================

func TestNewLazyProviderRegistry(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		registry := NewLazyProviderRegistry(nil, nil)
		assert.NotNil(t, registry)
		assert.Empty(t, registry.List())
	})

	t.Run("with custom config and event bus", func(t *testing.T) {
		config := &LazyProviderConfig{
			InitTimeout:   5 * time.Second,
			RetryAttempts: 2,
		}
		eventBus := events.NewEventBus(nil)

		registry := NewLazyProviderRegistry(config, eventBus)
		assert.NotNil(t, registry)
	})
}

func TestLazyProviderRegistry_Register(t *testing.T) {
	registry := NewLazyProviderRegistry(nil, nil)

	factory := func() (LLMProvider, error) {
		return &lazyMockProvider{}, nil
	}

	registry.Register("provider1", factory)
	registry.Register("provider2", factory)

	names := registry.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "provider1")
	assert.Contains(t, names, "provider2")
}

func TestLazyProviderRegistry_Get(t *testing.T) {
	registry := NewLazyProviderRegistry(nil, nil)

	factory := func() (LLMProvider, error) {
		return &lazyMockProvider{}, nil
	}

	registry.Register("test-provider", factory)

	lazy, ok := registry.Get("test-provider")
	assert.True(t, ok)
	assert.NotNil(t, lazy)
	assert.Equal(t, "test-provider", lazy.Name())

	// Non-existent provider
	lazy, ok = registry.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, lazy)
}

func TestLazyProviderRegistry_GetProvider(t *testing.T) {
	registry := NewLazyProviderRegistry(nil, nil)

	mockProvider := &lazyMockProvider{}
	factory := func() (LLMProvider, error) {
		return mockProvider, nil
	}

	registry.Register("test", factory)

	provider, err := registry.GetProvider("test")
	require.NoError(t, err)
	assert.NotNil(t, provider)

	// Non-existent provider
	provider, err = registry.GetProvider("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "not found")
}

func TestLazyProviderRegistry_InitializedProviders(t *testing.T) {
	registry := NewLazyProviderRegistry(nil, nil)

	factory := func() (LLMProvider, error) {
		return &lazyMockProvider{}, nil
	}

	registry.Register("p1", factory)
	registry.Register("p2", factory)
	registry.Register("p3", factory)

	// No providers initialized yet
	assert.Empty(t, registry.InitializedProviders())

	// Initialize p1 and p3
	_, _ = registry.GetProvider("p1")
	_, _ = registry.GetProvider("p3")

	initialized := registry.InitializedProviders()
	assert.Len(t, initialized, 2)
	assert.Contains(t, initialized, "p1")
	assert.Contains(t, initialized, "p3")
	assert.NotContains(t, initialized, "p2")
}

func TestLazyProviderRegistry_Preload(t *testing.T) {
	registry := NewLazyProviderRegistry(nil, nil)

	factory := func() (LLMProvider, error) {
		return &lazyMockProvider{}, nil
	}

	registry.Register("p1", factory)
	registry.Register("p2", factory)
	registry.Register("p3", factory)

	// Preload specific providers
	err := registry.Preload(context.Background(), "p1", "p3")
	require.NoError(t, err)

	initialized := registry.InitializedProviders()
	assert.Len(t, initialized, 2)
	assert.Contains(t, initialized, "p1")
	assert.Contains(t, initialized, "p3")
}

func TestLazyProviderRegistry_Preload_WithErrors(t *testing.T) {
	registry := NewLazyProviderRegistry(&LazyProviderConfig{
		InitTimeout:   100 * time.Millisecond,
		RetryAttempts: 1,
		RetryDelay:    10 * time.Millisecond,
	}, nil)

	successFactory := func() (LLMProvider, error) {
		return &lazyMockProvider{}, nil
	}

	failFactory := func() (LLMProvider, error) {
		return nil, errors.New("failed")
	}

	registry.Register("success", successFactory)
	registry.Register("fail1", failFactory)
	registry.Register("fail2", failFactory)

	err := registry.Preload(context.Background(), "success", "fail1", "fail2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "2 providers failed to preload")
}

func TestLazyProviderRegistry_PreloadAll(t *testing.T) {
	registry := NewLazyProviderRegistry(nil, nil)

	factory := func() (LLMProvider, error) {
		return &lazyMockProvider{}, nil
	}

	registry.Register("p1", factory)
	registry.Register("p2", factory)

	err := registry.PreloadAll(context.Background())
	require.NoError(t, err)

	initialized := registry.InitializedProviders()
	assert.Len(t, initialized, 2)
}

func TestLazyProviderRegistry_Reset(t *testing.T) {
	registry := NewLazyProviderRegistry(nil, nil)

	factory := func() (LLMProvider, error) {
		return &lazyMockProvider{}, nil
	}

	registry.Register("p1", factory)
	registry.Register("p2", factory)

	// Initialize all
	_ = registry.PreloadAll(context.Background())
	assert.Len(t, registry.InitializedProviders(), 2)

	// Reset all
	registry.Reset()

	// All should be uninitialized
	assert.Empty(t, registry.InitializedProviders())
}

func TestLazyProviderRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewLazyProviderRegistry(nil, nil)

	factory := func() (LLMProvider, error) {
		time.Sleep(10 * time.Millisecond)
		return &lazyMockProvider{}, nil
	}

	// Register multiple providers
	for i := 0; i < 10; i++ {
		registry.Register("provider"+string(rune('0'+i)), factory)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			providerNum := n % 10
			_, _ = registry.GetProvider("provider" + string(rune('0'+providerNum)))
			_ = registry.List()
			_ = registry.InitializedProviders()
		}(i)
	}

	wg.Wait()
}

// Verify lazyMockProvider implements LLMProvider
var _ LLMProvider = (*lazyMockProvider)(nil)
