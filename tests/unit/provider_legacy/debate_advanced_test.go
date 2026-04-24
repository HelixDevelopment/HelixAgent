// Previously tests/integration/debate_advanced_integration_test.go.
// Demoted alongside provider_test.go in PR25 of the CONST-030 compliance
// campaign — the file exclusively wires DebatePerformanceOptimizer
// against local MockLLMProvider siblings, never touches the network.
package provider_legacy_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
)

// =============================================================================
// Helpers for debate optimizer tests
// =============================================================================

// newTestOptimizer creates a DebatePerformanceOptimizer with a registry containing
// the given named providers. Returns the optimizer and the registry.
func newTestOptimizer(
	t *testing.T,
	config services.DebateOptimizationConfig,
	providerMap map[string]*MockLLMProvider,
) (*services.DebatePerformanceOptimizer, *services.ProviderRegistry) {
	t.Helper()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	for name, p := range providerMap {
		err := registry.RegisterProvider(name, p)
		require.NoError(t, err)
	}

	optimizer := services.NewDebatePerformanceOptimizer(config, registry, logger)
	return optimizer, registry
}

// makeTeamMember creates a DebateTeamMember for testing.
func makeTeamMember(
	position services.DebateTeamPosition,
	providerName, modelName string,
	provider *MockLLMProvider,
	fallbacks []*services.DebateTeamMember,
) *services.DebateTeamMember {
	return &services.DebateTeamMember{
		Position:     position,
		Role:         services.RoleAnalyst,
		ProviderName: providerName,
		ModelName:    modelName,
		Provider:     provider,
		Fallbacks:    fallbacks,
		Score:        8.0,
		IsActive:     true,
	}
}

// =============================================================================
// Test: Performance Optimizer — Cache Hit (second identical request)
// =============================================================================

func TestIntegration_Debate_PerformanceOptimizer_CacheHit(t *testing.T) {
	provider := NewMockLLMProvider("cache-provider")

	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableResponseCaching = true
	cfg.CacheTTL = 1 * time.Minute

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"cache-provider": provider,
	})

	member := makeTeamMember(
		services.PositionAnalyst,
		"cache-provider", "mock-model",
		provider, nil,
	)

	prompt := "Explain the benefits of Go generics"
	ctx := context.Background()

	// First call — cache miss, actual LLM call
	resp1, err := optimizer.ExecuteWithOptimization(ctx, member, prompt, nil)
	require.NoError(t, err)
	require.NotNil(t, resp1)

	// Second call with identical prompt + model — should be a cache hit
	resp2, err := optimizer.ExecuteWithOptimization(ctx, member, prompt, nil)
	require.NoError(t, err)
	require.NotNil(t, resp2)

	// Verify stats
	stats := optimizer.GetStats()
	assert.Equal(t, int64(2), stats.TotalRequests, "Should track 2 total requests")
	assert.Equal(t, int64(1), stats.CacheHits, "Second call should be a cache hit")
	assert.Equal(t, int64(1), stats.CacheMisses, "First call should be a cache miss")

	// The provider should only have been called once (the cache-hit skips provider)
	assert.Equal(t, int64(1), provider.GetCompleteCalls(),
		"Provider should only be called once — second is served from cache")
}

// =============================================================================
// Test: Performance Optimizer — Parallel Execution with Semaphore
// =============================================================================

func TestIntegration_Debate_PerformanceOptimizer_Parallel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping parallel execution test in short mode")  // SKIP-OK: #short-mode
	}

	// Create providers that simulate work with a small delay
	providerMap := make(map[string]*MockLLMProvider)
	members := make([]*services.DebateTeamMember, 0)

	positions := []services.DebateTeamPosition{
		services.PositionAnalyst,
		services.PositionProposer,
		services.PositionCritic,
		services.PositionSynthesis,
		services.PositionMediator,
	}

	for i, pos := range positions {
		name := "parallel-provider-" + string(rune('A'+i))
		p := NewMockLLMProvider(name)
		p.SetResponseDelay(50 * time.Millisecond) // Small delay to simulate work
		providerMap[name] = p

		members = append(members, makeTeamMember(
			pos, name, "model-"+name, p, nil,
		))
	}

	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableParallelExecution = true
	cfg.MaxParallelRequests = 3 // Semaphore limits to 3 concurrent
	cfg.EnableResponseCaching = false
	cfg.RequestTimeout = 10 * time.Second

	optimizer, _ := newTestOptimizer(t, cfg, providerMap)

	ctx := context.Background()
	prompt := "Analyze the system architecture for improvements"

	startTime := time.Now()
	results := optimizer.ExecuteParallel(ctx, members, prompt)
	elapsed := time.Since(startTime)

	// All 5 members should produce results
	assert.Len(t, results, 5, "All 5 team members should produce results")

	// With semaphore=3, 5 requests (50ms each) should take ~100ms total (2 batches)
	// not 250ms (serial). Allow generous margin for CI.
	assert.Less(t, elapsed, 2*time.Second,
		"Parallel execution should be significantly faster than serial")

	// Verify stats tracked parallel requests
	stats := optimizer.GetStats()
	assert.Equal(t, int64(5), stats.ParallelRequests,
		"All 5 requests should be counted as parallel")
	assert.Equal(t, int64(5), stats.TotalRequests)
}

// =============================================================================
// Test: Performance Optimizer — Early Termination on High Consensus
// =============================================================================

func TestIntegration_Debate_EarlyTermination_HighConsensus(t *testing.T) {
	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableEarlyTermination = true
	cfg.EarlyTerminationThreshold = 0.50 // Low threshold so our test data triggers it

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"et-provider": NewMockLLMProvider("et-provider"),
	})

	// Responses that share many words — high consensus
	responses := map[services.DebateTeamPosition]string{
		services.PositionAnalyst:   "The microservices architecture provides scalability, maintainability, and deployment independence for modern applications.",
		services.PositionProposer:  "The microservices architecture enables scalability, maintainability, and independent deployment for modern systems.",
		services.PositionCritic:    "The microservices architecture offers scalability, maintainability, and deployment flexibility for modern applications.",
		services.PositionSynthesis: "The microservices architecture delivers scalability, maintainability, and independent deployment for modern applications.",
	}

	shouldTerminate := optimizer.ShouldTerminateEarly(responses)
	assert.True(t, shouldTerminate,
		"Early termination should trigger when responses share many key points")

	// Verify stats
	stats := optimizer.GetStats()
	assert.Equal(t, int64(1), stats.EarlyTerminations,
		"One early termination should be recorded")
}

func TestIntegration_Debate_EarlyTermination_LowConsensus(t *testing.T) {
	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableEarlyTermination = true
	cfg.EarlyTerminationThreshold = 0.95 // Very high threshold

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"et-provider": NewMockLLMProvider("et-provider"),
	})

	// Completely different responses — no consensus
	responses := map[services.DebateTeamPosition]string{
		services.PositionAnalyst:  "Quantum computing leverages superposition and entanglement phenomena.",
		services.PositionProposer: "Ancient Roman aqueducts demonstrated remarkable engineering achievements.",
		services.PositionCritic:   "Molecular gastronomy transforms cooking through chemical understanding.",
	}

	shouldTerminate := optimizer.ShouldTerminateEarly(responses)
	assert.False(t, shouldTerminate,
		"Early termination should NOT trigger when responses are unrelated")
}

func TestIntegration_Debate_EarlyTermination_TooFewResponses(t *testing.T) {
	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableEarlyTermination = true
	cfg.EarlyTerminationThreshold = 0.50

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"et-provider": NewMockLLMProvider("et-provider"),
	})

	// Fewer than 3 responses — early termination requires at least 3
	responses := map[services.DebateTeamPosition]string{
		services.PositionAnalyst:  "Same content across responses.",
		services.PositionProposer: "Same content across responses.",
	}

	shouldTerminate := optimizer.ShouldTerminateEarly(responses)
	assert.False(t, shouldTerminate,
		"Early termination should NOT trigger with fewer than 3 responses")
}

// =============================================================================
// Test: Performance Optimizer — Fallback Chain (primary fails, fallback succeeds)
// =============================================================================

func TestIntegration_Debate_FallbackChain_FirstFails(t *testing.T) {
	primaryProvider := NewMockLLMProvider("fallback-primary")
	primaryProvider.SetCompleteError(errors.New("primary API timeout"))

	fallbackProvider := NewMockLLMProvider("fallback-secondary")
	// fallbackProvider succeeds by default

	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableSmartFallback = true
	cfg.FallbackTimeout = 10 * time.Second
	cfg.EnableResponseCaching = false
	cfg.RequestTimeout = 10 * time.Second

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"fallback-primary":   primaryProvider,
		"fallback-secondary": fallbackProvider,
	})

	// Build member with fallback chain
	fallbackMember := makeTeamMember(
		services.PositionProposer,
		"fallback-secondary", "fallback-model",
		fallbackProvider, nil,
	)

	member := makeTeamMember(
		services.PositionAnalyst,
		"fallback-primary", "primary-model",
		primaryProvider,
		[]*services.DebateTeamMember{fallbackMember},
	)

	ctx := context.Background()
	resp, err := optimizer.ExecuteWithOptimization(ctx, member, "Test fallback chain", nil)
	require.NoError(t, err, "Should succeed via fallback after primary fails")
	require.NotNil(t, resp)

	// Verify stats show fallback was triggered
	stats := optimizer.GetStats()
	assert.Equal(t, int64(1), stats.FallbacksTriggered,
		"One fallback should have been triggered")
}

func TestIntegration_Debate_FallbackChain_AllFail(t *testing.T) {
	primaryProvider := NewMockLLMProvider("all-fail-primary")
	primaryProvider.SetCompleteError(errors.New("primary failed"))

	fallback1 := NewMockLLMProvider("all-fail-fb1")
	fallback1.SetCompleteError(errors.New("fallback 1 failed"))

	fallback2 := NewMockLLMProvider("all-fail-fb2")
	fallback2.SetCompleteError(errors.New("fallback 2 failed"))

	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableSmartFallback = true
	cfg.FallbackTimeout = 5 * time.Second
	cfg.EnableResponseCaching = false
	cfg.RequestTimeout = 5 * time.Second

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"all-fail-primary": primaryProvider,
		"all-fail-fb1":     fallback1,
		"all-fail-fb2":     fallback2,
	})

	fb2Member := makeTeamMember(
		services.PositionCritic, "all-fail-fb2", "fb2-model", fallback2, nil,
	)
	fb1Member := makeTeamMember(
		services.PositionProposer, "all-fail-fb1", "fb1-model", fallback1,
		[]*services.DebateTeamMember{fb2Member},
	)
	member := makeTeamMember(
		services.PositionAnalyst, "all-fail-primary", "primary-model", primaryProvider,
		[]*services.DebateTeamMember{fb1Member},
	)

	ctx := context.Background()
	_, err := optimizer.ExecuteWithOptimization(ctx, member, "Test all fail", nil)
	assert.Error(t, err, "Should return error when all providers in chain fail")
}

// =============================================================================
// Test: Performance Optimizer — Stats Tracking via Atomics
// =============================================================================

func TestIntegration_Debate_Stats_Tracking(t *testing.T) {
	provider := NewMockLLMProvider("stats-provider")

	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableResponseCaching = true
	cfg.CacheTTL = 5 * time.Minute
	cfg.EnableSmartFallback = false
	cfg.RequestTimeout = 10 * time.Second

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"stats-provider": provider,
	})

	member := makeTeamMember(
		services.PositionAnalyst,
		"stats-provider", "stats-model",
		provider, nil,
	)

	ctx := context.Background()

	// Execute 5 unique prompts — all cache misses
	for i := 0; i < 5; i++ {
		prompt := "Unique prompt number " + string(rune('0'+i)) + " for stats tracking test"
		resp, err := optimizer.ExecuteWithOptimization(ctx, member, prompt, nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
	}

	// Execute the same 5 prompts again — all cache hits
	for i := 0; i < 5; i++ {
		prompt := "Unique prompt number " + string(rune('0'+i)) + " for stats tracking test"
		resp, err := optimizer.ExecuteWithOptimization(ctx, member, prompt, nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
	}

	stats := optimizer.GetStats()
	assert.Equal(t, int64(10), stats.TotalRequests, "10 total requests")
	assert.Equal(t, int64(5), stats.CacheHits, "5 cache hits on repeated prompts")
	assert.Equal(t, int64(5), stats.CacheMisses, "5 cache misses on first calls")
	assert.Greater(t, stats.AverageLatencyMs, int64(0), "Average latency should be positive")

	// Provider should only have been called 5 times (cache hits skip provider)
	assert.Equal(t, int64(5), provider.GetCompleteCalls(),
		"Provider should be called only for cache misses")
}

// =============================================================================
// Test: Performance Optimizer — Cache Clear
// =============================================================================

func TestIntegration_Debate_CacheCleared_ForcesMiss(t *testing.T) {
	provider := NewMockLLMProvider("clear-cache-provider")

	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableResponseCaching = true
	cfg.CacheTTL = 5 * time.Minute

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"clear-cache-provider": provider,
	})

	member := makeTeamMember(
		services.PositionAnalyst,
		"clear-cache-provider", "clear-model",
		provider, nil,
	)

	ctx := context.Background()
	prompt := "Cache clear test prompt"

	// First call — cache miss
	_, err := optimizer.ExecuteWithOptimization(ctx, member, prompt, nil)
	require.NoError(t, err)

	// Second call — cache hit
	_, err = optimizer.ExecuteWithOptimization(ctx, member, prompt, nil)
	require.NoError(t, err)

	assert.Equal(t, int64(1), optimizer.GetStats().CacheHits)

	// Clear cache
	optimizer.ClearCache()

	// Third call — should be cache miss again (cache was cleared)
	_, err = optimizer.ExecuteWithOptimization(ctx, member, prompt, nil)
	require.NoError(t, err)

	stats := optimizer.GetStats()
	assert.Equal(t, int64(3), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.CacheHits, "Only the pre-clear hit should count")
	assert.Equal(t, int64(2), stats.CacheMisses, "Two misses: initial + post-clear")
}

// =============================================================================
// Test: Performance Optimizer — Disabled Caching
// =============================================================================

func TestIntegration_Debate_CachingDisabled(t *testing.T) {
	var callCount int64
	provider := NewMockLLMProvider("no-cache-provider")

	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableResponseCaching = false

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"no-cache-provider": provider,
	})

	member := makeTeamMember(
		services.PositionAnalyst,
		"no-cache-provider", "no-cache-model",
		provider, nil,
	)

	ctx := context.Background()
	prompt := "Same prompt repeated"

	for i := 0; i < 3; i++ {
		_, err := optimizer.ExecuteWithOptimization(ctx, member, prompt, nil)
		require.NoError(t, err)
		atomic.AddInt64(&callCount, 1)
	}

	// Every call should hit the provider (no caching)
	assert.Equal(t, int64(3), provider.GetCompleteCalls(),
		"Provider should be called for every request when caching is disabled")

	stats := optimizer.GetStats()
	assert.Equal(t, int64(3), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.CacheHits,
		"Cache hits should be 0 when caching is disabled")
	assert.Equal(t, int64(0), stats.CacheMisses,
		"Cache misses should be 0 when caching is disabled (not checked)")
}

// =============================================================================
// Test: Performance Optimizer — Early Termination Disabled
// =============================================================================

func TestIntegration_Debate_EarlyTermination_Disabled(t *testing.T) {
	cfg := services.DefaultDebateOptimizationConfig()
	cfg.EnableEarlyTermination = false

	optimizer, _ := newTestOptimizer(t, cfg, map[string]*MockLLMProvider{
		"et-disabled": NewMockLLMProvider("et-disabled"),
	})

	// Even with identical responses, early termination should not trigger
	responses := map[services.DebateTeamPosition]string{
		services.PositionAnalyst:   "Identical response content for all positions in this debate.",
		services.PositionProposer:  "Identical response content for all positions in this debate.",
		services.PositionCritic:    "Identical response content for all positions in this debate.",
		services.PositionSynthesis: "Identical response content for all positions in this debate.",
	}

	shouldTerminate := optimizer.ShouldTerminateEarly(responses)
	assert.False(t, shouldTerminate,
		"Early termination should NOT trigger when the feature is disabled")
}

// =============================================================================
// Test: Performance Optimizer — Default Config values
// =============================================================================

func TestIntegration_Debate_DefaultConfig(t *testing.T) {
	cfg := services.DefaultDebateOptimizationConfig()

	assert.True(t, cfg.EnableParallelExecution)
	assert.True(t, cfg.EnableResponseCaching)
	assert.True(t, cfg.EnableEarlyTermination)
	assert.True(t, cfg.EnableStreaming)
	assert.True(t, cfg.EnableSmartFallback)
	assert.Equal(t, 3, cfg.MaxParallelRequests)
	assert.Equal(t, 5*time.Minute, cfg.CacheTTL)
	assert.Equal(t, 0.95, cfg.EarlyTerminationThreshold)
	assert.Equal(t, 60*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 30*time.Second, cfg.FallbackTimeout)
}
