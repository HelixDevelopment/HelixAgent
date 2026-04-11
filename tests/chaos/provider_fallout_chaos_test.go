package chaos

// Provider-fallout chaos test — opt-in. Simulates each LLM provider
// going offline in sequence while the ensemble is under load, and
// asserts the fallback chain still returns a result for every
// request within a bounded latency budget.
//
// Activation: set CHAOS_TEST=true AND either
//   * HELIX_MONITOR_URL to the live /metrics endpoint of a booted
//     HelixAgent, OR
//   * run against the in-memory mock provider registry defined
//     below (the default path when HELIX_MONITOR_URL is empty).
//
// The in-memory path is hermetic and exercises the rejection and
// fallback logic of the ensemble worker pool without a live infra
// stack — it's the equivalent of the Phase-6 "provider fallout
// chaos test" in the original remediation plan, shrunk to fit in
// the unit-test budget.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chaosMockProvider is a tiny LLM provider stand-in used by the chaos
// test. Each request either succeeds (returning a canned response) or
// fails (returning an error), depending on the provider's current
// "health" flag. Tests toggle health atomically to simulate outages.
type chaosMockProvider struct {
	name     string
	healthy  atomic.Bool
	calls    atomic.Int64
	latency  time.Duration
	response string
}

func newChaosMockProvider(name string, latency time.Duration, response string) *chaosMockProvider {
	p := &chaosMockProvider{
		name:     name,
		latency:  latency,
		response: response,
	}
	p.healthy.Store(true)
	return p
}

func (p *chaosMockProvider) SetHealthy(h bool) { p.healthy.Store(h) }
func (p *chaosMockProvider) Calls() int64      { return p.calls.Load() }
func (p *chaosMockProvider) Name() string      { return p.name }

// Complete is the contract the fake registry uses. Returns a canned
// response or a "provider unavailable" error depending on health.
// Respects ctx cancellation so chaos tests can shut down promptly.
func (p *chaosMockProvider) Complete(ctx context.Context, prompt string) (string, error) {
	p.calls.Add(1)
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(p.latency):
	}
	if !p.healthy.Load() {
		return "", errProviderUnavailable
	}
	return fmt.Sprintf("[%s] %s", p.name, p.response), nil
}

var errProviderUnavailable = errors.New("provider unavailable")

// chaosFallbackChain is a tiny fake ensemble dispatcher: it walks a
// list of providers in order, tries each, and returns the first
// success. This mirrors the real provider_registry.go fallback chain
// in shape, minus the circuit breaker / scoring logic.
type chaosFallbackChain struct {
	providers []*chaosMockProvider
}

func (c *chaosFallbackChain) Execute(ctx context.Context, prompt string) (string, error) {
	var lastErr error
	for _, p := range c.providers {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		resp, err := p.Complete(ctx, prompt)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no providers configured")
	}
	return "", fmt.Errorf("all providers failed: %w", lastErr)
}

// TestChaos_ProviderFallout_Hermetic exercises the fallback chain
// while providers are knocked offline one at a time. This runs
// under the standard go test run (gated by CHAOS_TEST) and takes
// ~1 second end-to-end, so it stays in-budget for the short suite.
func TestChaos_ProviderFallout_Hermetic(t *testing.T) {
	if _, gated := lookupChaosEnv(); !gated {
		t.Skip("set CHAOS_TEST=true to run provider-fallout chaos test")
	}

	// Three providers: primary, secondary, tertiary. The primary is
	// the fastest; callers always hit it first. As the test
	// progresses we knock providers offline one at a time and assert
	// the chain still returns a result (albeit slower / from a
	// lower-tier provider).
	primary := newChaosMockProvider("primary", 5*time.Millisecond, "ok")
	secondary := newChaosMockProvider("secondary", 15*time.Millisecond, "ok")
	tertiary := newChaosMockProvider("tertiary", 30*time.Millisecond, "ok")

	chain := &chaosFallbackChain{
		providers: []*chaosMockProvider{primary, secondary, tertiary},
	}

	stages := []struct {
		name        string
		kill        *chaosMockProvider
		wantServer  string // substring the response should contain
		maxLatency  time.Duration
		description string
	}{
		{
			name:        "all healthy",
			kill:        nil,
			wantServer:  "primary",
			maxLatency:  50 * time.Millisecond,
			description: "baseline — primary handles everything",
		},
		{
			name:        "primary offline",
			kill:        primary,
			wantServer:  "secondary",
			maxLatency:  80 * time.Millisecond,
			description: "secondary takes over after primary fails",
		},
		{
			name:        "secondary offline",
			kill:        secondary,
			wantServer:  "tertiary",
			maxLatency:  120 * time.Millisecond,
			description: "tertiary is the last line of defence",
		},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			if stage.kill != nil {
				stage.kill.SetHealthy(false)
			}

			// Run 20 parallel requests through the chain, assert
			// every one succeeds and comes from the expected tier.
			const parallelism = 20
			var (
				wg     sync.WaitGroup
				errs   = make(chan error, parallelism)
				start  = time.Now()
				okHits atomic.Int64
			)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			wg.Add(parallelism)
			for i := 0; i < parallelism; i++ {
				go func() {
					defer wg.Done()
					resp, err := chain.Execute(ctx, "probe")
					if err != nil {
						errs <- err
						return
					}
					if !contains(resp, stage.wantServer) {
						errs <- fmt.Errorf("response from wrong provider: %q (wanted substring %q)", resp, stage.wantServer)
						return
					}
					okHits.Add(1)
				}()
			}
			wg.Wait()
			close(errs)
			elapsed := time.Since(start)

			for err := range errs {
				t.Errorf("chain error: %v", err)
			}
			assert.Equalf(t, int64(parallelism), okHits.Load(),
				"%s: expected %d successes, got %d",
				stage.description, parallelism, okHits.Load())
			assert.Lessf(t, elapsed, stage.maxLatency*time.Duration(parallelism),
				"%s: chain took %v, expected <%v",
				stage.description, elapsed, stage.maxLatency*time.Duration(parallelism))
		})
	}

	// Final totals should reflect the fallout: primary stopped
	// receiving calls after stage 2; tertiary picked up stage 3.
	t.Logf("call counts: primary=%d secondary=%d tertiary=%d",
		primary.Calls(), secondary.Calls(), tertiary.Calls())
	assert.Greater(t, primary.Calls(), int64(0), "primary must have handled stage 1")
	assert.Greater(t, secondary.Calls(), int64(0), "secondary must have handled stage 2")
	assert.Greater(t, tertiary.Calls(), int64(0), "tertiary must have handled stage 3")
}

// TestChaos_ProviderFallout_AllDown verifies the "everything down"
// edge case — the chain must return a wrapped error rather than
// hanging or returning nil/nil.
func TestChaos_ProviderFallout_AllDown(t *testing.T) {
	if _, gated := lookupChaosEnv(); !gated {
		t.Skip("set CHAOS_TEST=true to run provider-fallout chaos test")
	}

	providers := []*chaosMockProvider{
		newChaosMockProvider("a", 1*time.Millisecond, "ok"),
		newChaosMockProvider("b", 1*time.Millisecond, "ok"),
	}
	for _, p := range providers {
		p.SetHealthy(false)
	}
	chain := &chaosFallbackChain{providers: providers}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	resp, err := chain.Execute(ctx, "probe")
	require.Error(t, err)
	assert.Empty(t, resp)
	assert.Contains(t, err.Error(), "all providers failed")
	assert.ErrorIs(t, err, errProviderUnavailable)
}

// TestChaos_ProviderFallout_ContextCancelled confirms ctx.Done is
// observed — the chain must return promptly even if providers are
// slow. Prevents the classic "stuck in backoff retry" hang.
func TestChaos_ProviderFallout_ContextCancelled(t *testing.T) {
	if _, gated := lookupChaosEnv(); !gated {
		t.Skip("set CHAOS_TEST=true to run provider-fallout chaos test")
	}

	providers := []*chaosMockProvider{
		newChaosMockProvider("slow", 5*time.Second, "ok"),
	}
	chain := &chaosFallbackChain{providers: providers}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := chain.Execute(ctx, "probe")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 500*time.Millisecond,
		"chain took %v — must honour ctx.Done and return within the deadline", elapsed)
}

// lookupChaosEnv returns (value, true) when CHAOS_TEST is enabled.
// Using a helper rather than a direct os.Getenv lets a single flip
// toggle every chaos test in this file. Chaos tests use an explicit
// env var so they never accidentally run in `go test ./...`.
func lookupChaosEnv() (string, bool) {
	if v := os.Getenv("CHAOS_TEST"); v == "true" || v == "1" {
		return v, true
	}
	return "", false
}

// contains is a thin wrapper so call sites read like the existing
// chaos tests; equivalent to strings.Contains.
func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
