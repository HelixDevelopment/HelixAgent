//go:build performance
// +build performance

// Package performance — benchmark-only test helper.
//
// This file defines benchMockProvider, a deterministic, zero-I/O implementation
// of the llm.LLMProvider interface used exclusively by benchmark code under
// the `performance` build tag.
//
// CONST-050(A) compliance: this helper is permitted because every source file
// in tests/performance/ is gated behind the `performance` build tag and is
// only ever compiled with `go test -tags=performance`. It is NEVER linked
// into production binaries (helix_agent/cmd/*, helix_agent/applications/*)
// and is NEVER imported by non-test code. It exists solely so benchmarks
// can isolate the pipeline overhead they intend to measure (lazy-loading,
// circuit-breaker, router dispatch, etc.) from the latency of real LLM
// HTTP round-trips.
//
// Anti-bluff note (CONST-035 / Article XI §11.9): this helper does NOT
// simulate LLM output to be shown to users. It returns the caller-supplied
// canned response so benchmarks remain reproducible. Any benchmark that
// later compares quality of output MUST swap this for a real provider via
// the test infrastructure stack (docker-compose.full-test.yml) per Rule 5.

package performance

import (
	"context"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// benchMockProvider is a test-helper mock; NOT production code.
//
// It implements llm.LLMProvider with deterministic, in-memory responses so
// benchmarks can measure surrounding pipeline overhead without incurring
// real network or LLM-inference cost. Configure response and (optional)
// per-call delay at construction time:
//
//	p := &benchMockProvider{
//	    response: &models.LLMResponse{Content: "ok"},
//	    delay:    0,           // measure pure overhead
//	}
//
// Concurrency: every method is safe for concurrent use without internal
// locking because the struct fields are written once at construction and
// read-only thereafter. The struct is therefore safe for b.RunParallel.
type benchMockProvider struct {
	// response is the canned LLMResponse returned from Complete and
	// emitted on the channel returned from CompleteStream. May be nil,
	// in which case Complete returns (nil, nil) and CompleteStream
	// returns an immediately-closed channel.
	response *models.LLMResponse

	// delay, if > 0, pauses each Complete/CompleteStream/HealthCheck call
	// for the configured duration. Use this to model provider latency in
	// pipeline-overhead benchmarks. Default 0 = no artificial delay.
	delay time.Duration

	// healthErr, if non-nil, is returned from HealthCheck. Lets benchmarks
	// exercise unhealthy-provider code paths (circuit breaker tripping,
	// failover dispatch, etc.) deterministically.
	healthErr error

	// capabilities, if non-nil, is returned from GetCapabilities. When nil
	// a zero-value *models.ProviderCapabilities is returned so callers
	// never receive a nil pointer.
	capabilities *models.ProviderCapabilities
}

// Compile-time assertion that benchMockProvider satisfies llm.LLMProvider.
// If the interface gains a new method, this line breaks the build of the
// performance benchmarks, surfacing the missing implementation immediately
// rather than at runtime.
var _ llm.LLMProvider = (*benchMockProvider)(nil)

// Complete returns the canned response after the optional configured delay,
// honouring context cancellation. It never makes a network call.
func (m *benchMockProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.response, nil
}

// CompleteStream emits the canned response (if any) on a buffered channel
// then closes it. Honours context cancellation between emission and close.
func (m *benchMockProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		if m.delay > 0 {
			select {
			case <-time.After(m.delay):
			case <-ctx.Done():
				return
			}
		}
		if m.response != nil {
			select {
			case ch <- m.response:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

// HealthCheck returns the configured healthErr (nil by default) after the
// optional delay. Sufficient to drive health-monitor and circuit-breaker
// benchmark paths.
func (m *benchMockProvider) HealthCheck() error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.healthErr
}

// GetCapabilities returns the configured capabilities or, when none were
// set, an empty *models.ProviderCapabilities so callers never need a nil
// check.
func (m *benchMockProvider) GetCapabilities() *models.ProviderCapabilities {
	if m.capabilities != nil {
		return m.capabilities
	}
	return &models.ProviderCapabilities{}
}

// ValidateConfig accepts any configuration. Benchmarks that need to drive
// validation-failure paths should construct a dedicated provider; this
// default permissive behaviour matches the existing mockProvider in
// internal/llm/health_monitor_test.go.
func (m *benchMockProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
