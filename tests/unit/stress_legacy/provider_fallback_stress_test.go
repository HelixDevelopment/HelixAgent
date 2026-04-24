// Previously tests/stress/... — demoted to unit package under CONST-030.
// Stress tests of in-process concurrency primitives (semaphore / pool /
// cache saturation, circuit-breaker storms, goroutine growth) that
// exercise local mock providers and invokers. Keeping the mocks in the
// unit tree per the audit's Pattern-4 disposition for stress tests.
// PR33 of the CONST-030 compliance campaign.
package stress_legacy_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// fallbackMockProvider implements llm.LLMProvider for fallback chain tests.
// It tracks call counts and can be configured to fail deterministically.
type fallbackMockProvider struct {
	id         string
	shouldFail bool
	callCount  atomic.Int64
}

func newFallbackProvider(id string, shouldFail bool) *fallbackMockProvider {
	return &fallbackMockProvider{id: id, shouldFail: shouldFail}
}

func (p *fallbackMockProvider) Complete(
	_ context.Context, _ *models.LLMRequest,
) (*models.LLMResponse, error) {
	p.callCount.Add(1)
	if p.shouldFail {
		return nil, fmt.Errorf("provider %s: intentional failure", p.id)
	}
	return &models.LLMResponse{
		Content:    fmt.Sprintf("response from %s", p.id),
		ProviderID: p.id,
	}, nil
}

func (p *fallbackMockProvider) CompleteStream(
	_ context.Context, _ *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	if p.shouldFail {
		close(ch)
		return ch, fmt.Errorf("provider %s: stream failure", p.id)
	}
	go func() {
		defer close(ch)
		ch <- &models.LLMResponse{
			Content:    fmt.Sprintf("stream from %s", p.id),
			ProviderID: p.id,
		}
	}()
	return ch, nil
}

func (p *fallbackMockProvider) HealthCheck() error {
	if p.shouldFail {
		return fmt.Errorf("provider %s: unhealthy", p.id)
	}
	return nil
}

func (p *fallbackMockProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{"test-model"},
	}
}

func (p *fallbackMockProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// fallbackChain attempts providers in order until one succeeds.
// It respects circuit breaker state and records which provider responded.
func fallbackChain(
	ctx context.Context,
	breakers []*llm.CircuitBreaker,
	req *models.LLMRequest,
) (*models.LLMResponse, string, error) {
	for i, cb := range breakers {
		resp, err := cb.Complete(ctx, req)
		if err == nil {
			return resp, fmt.Sprintf("provider-%d", i), nil
		}
		// If open/half-open, skip to next; for real failures, continue chain
		if errors.Is(err, llm.ErrCircuitOpen) ||
			errors.Is(err, llm.ErrCircuitHalfOpenRejected) {
			continue
		}
		// Genuine provider error — circuit breaker has recorded it; try next
		continue
	}
	return nil, "", fmt.Errorf("all providers exhausted")
}

// TestStress_ProviderFallback_ChainWithCircuitBreakers exercises a 5-provider
// fallback chain where the first 3 providers always fail. 50 concurrent
// requests are sent; all should eventually succeed via providers 4 or 5.
// Verifies that failing providers' circuit breakers engage under load.
func TestStress_ProviderFallback_ChainWithCircuitBreakers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cbConfig := llm.CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 2,
	}

	// Providers 0-2 fail; providers 3-4 succeed
	providers := []*fallbackMockProvider{
		newFallbackProvider("fallback-0", true),  // always fails
		newFallbackProvider("fallback-1", true),  // always fails
		newFallbackProvider("fallback-2", true),  // always fails
		newFallbackProvider("fallback-3", false), // succeeds
		newFallbackProvider("fallback-4", false), // succeeds (backup)
	}

	breakers := make([]*llm.CircuitBreaker, len(providers))
	for i, p := range providers {
		breakers[i] = llm.NewCircuitBreaker(p.id, p, cbConfig)
	}

	const goroutineCount = 50
	var wg sync.WaitGroup
	var successCount, failCount, panicCount atomic.Int64
	var respondingProviders sync.Map // track which providers responded

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
				}
			}()
			<-start

			req := &models.LLMRequest{
				Prompt: "fallback chain test",
				ModelParams: models.ModelParameters{
					Model: "test-model",
				},
			}

			resp, providerID, err := fallbackChain(context.Background(), breakers, req)
			if err != nil {
				failCount.Add(1)
				return
			}
			if resp != nil {
				successCount.Add(1)
				respondingProviders.Store(providerID, true)
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: provider fallback chain stress test timed out")
	}

	assert.Zero(t, panicCount.Load(), "no panics in fallback chain under load")
	assert.Equal(t, int64(goroutineCount), successCount.Load()+failCount.Load(),
		"all requests must be accounted for")
	assert.Greater(t, successCount.Load(), int64(0),
		"at least some requests should succeed via fallback providers")

	// Verify that failing providers' circuit breakers engaged
	failedOpenCount := 0
	for i := 0; i < 3; i++ {
		state := breakers[i].GetState()
		if state == llm.CircuitOpen {
			failedOpenCount++
		}
		stats := breakers[i].GetStats()
		t.Logf("Failing provider %d: state=%s, calls=%d, failures=%d",
			i, state, stats.TotalRequests, stats.TotalFailures)
	}

	// Successful providers should remain closed
	for i := 3; i < 5; i++ {
		state := breakers[i].GetState()
		stats := breakers[i].GetStats()
		t.Logf("Healthy provider %d: state=%s, calls=%d", i, state, stats.TotalRequests)
		assert.Equal(t, llm.CircuitClosed, state,
			"healthy provider circuit breaker should remain closed")
	}

	t.Logf("Fallback chain: success=%d, fail=%d, panics=%d, failed_open=%d",
		successCount.Load(), failCount.Load(), panicCount.Load(), failedOpenCount)

	// After enough requests, failing providers should have tripped open
	assert.Greater(t, failedOpenCount, 0,
		"at least one failing provider's circuit breaker should be open")
}

// TestStress_ProviderFallback_AllFailThenRecover tests the scenario where all
// providers fail (causing all requests to fail), then providers recover. After
// recovery, new requests should succeed. Verifies the fallback chain correctly
// handles complete failure and recovery.
func TestStress_ProviderFallback_AllFailThenRecover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cbConfig := llm.CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    1,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}

	// All 3 providers start failing
	providers := []*fallbackMockProvider{
		newFallbackProvider("allf-0", true),
		newFallbackProvider("allf-1", true),
		newFallbackProvider("allf-2", true),
	}

	breakers := make([]*llm.CircuitBreaker, len(providers))
	for i, p := range providers {
		breakers[i] = llm.NewCircuitBreaker(p.id, p, cbConfig)
	}

	req := &models.LLMRequest{Prompt: "all fail test"}

	// Phase 1: all fail — 20 requests, all should fail
	var phase1Success, phase1Fail int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := fallbackChain(context.Background(), breakers, req)
			if err != nil {
				atomic.AddInt64(&phase1Fail, 1)
			} else {
				atomic.AddInt64(&phase1Success, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	t.Logf("Phase 1 (all fail): success=%d, fail=%d", phase1Success, phase1Fail)
	assert.Greater(t, phase1Fail, int64(0),
		"phase 1 should have failures when all providers fail")

	// Phase 2: wait for circuit breaker timeout, then recover providers
	time.Sleep(120 * time.Millisecond)
	for _, p := range providers {
		p.shouldFail = false
	}

	// Phase 2: recovery — 20 requests, should now succeed
	var phase2Success, phase2Fail int64
	start2 := make(chan struct{})

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start2
			_, _, err := fallbackChain(context.Background(), breakers, req)
			if err != nil {
				atomic.AddInt64(&phase2Fail, 1)
			} else {
				atomic.AddInt64(&phase2Success, 1)
			}
		}()
	}
	close(start2)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: all-fail-then-recover test timed out")
	}

	t.Logf("Phase 2 (recovery): success=%d, fail=%d", phase2Success, phase2Fail)
	assert.Greater(t, phase2Success, int64(0),
		"phase 2 should have successes after providers recover")
}

// TestStress_ProviderFallback_ConcurrentChainTraversal stress-tests
// the fallback chain with mixed failing/healthy providers under maximum
// concurrent load (100 goroutines), validating that the chain is always
// traversed correctly and no goroutine starves or deadlocks.
func TestStress_ProviderFallback_ConcurrentChainTraversal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cbConfig := llm.CircuitBreakerConfig{
		FailureThreshold:    10,
		SuccessThreshold:    2,
		Timeout:             60 * time.Second,
		HalfOpenMaxRequests: 2,
	}

	// Alternating fail/success pattern: 0=fail, 1=success, 2=fail, 3=success
	providers := []*fallbackMockProvider{
		newFallbackProvider("trav-0", true),
		newFallbackProvider("trav-1", false),
		newFallbackProvider("trav-2", true),
		newFallbackProvider("trav-3", false),
	}

	breakers := make([]*llm.CircuitBreaker, len(providers))
	for i, p := range providers {
		breakers[i] = llm.NewCircuitBreaker(p.id, p, cbConfig)
	}

	const goroutineCount = 100
	const requestsPerGoroutine = 10
	var wg sync.WaitGroup
	var successCount, failCount, panicCount atomic.Int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
				}
			}()
			<-start

			for j := 0; j < requestsPerGoroutine; j++ {
				req := &models.LLMRequest{
					Prompt: "chain traversal",
					ModelParams: models.ModelParameters{
						Model: "test-model",
					},
				}
				_, _, err := fallbackChain(context.Background(), breakers, req)
				if err != nil {
					failCount.Add(1)
				} else {
					successCount.Add(1)
				}
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent chain traversal timed out")
	}

	total := successCount.Load() + failCount.Load()
	assert.Equal(t, int64(goroutineCount*requestsPerGoroutine), total,
		"all requests must complete")
	assert.Zero(t, panicCount.Load(), "no panics during concurrent traversal")

	// With providers 1 and 3 healthy, most requests should succeed
	assert.Greater(t, successCount.Load(), int64(0),
		"requests should succeed via healthy providers in fallback chain")

	t.Logf("Chain traversal: success=%d, fail=%d, panics=%d, total=%d",
		successCount.Load(), failCount.Load(), panicCount.Load(), total)
}
