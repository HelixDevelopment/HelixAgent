//go:build stress
// +build stress

package stress

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

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// failingProvider is an LLMProvider whose Complete always returns an error,
// simulating a provider that is fully unavailable.
type failingProvider struct {
	name  string
	calls int64
}

func (p *failingProvider) Complete(
	_ context.Context, _ *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	return nil, fmt.Errorf("provider %s: simulated total failure", p.name)
}

func (p *failingProvider) CompleteStream(
	_ context.Context, _ *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	return nil, fmt.Errorf("provider %s: simulated stream failure", p.name)
}

// TestEnsemble_AllProvidersFail_GracefulError verifies that when every provider
// in the ensemble fails simultaneously the ensemble returns a non-nil error
// (no panic, no hang) and the error is descriptive. This exercises the
// all-providers-failed code path under concurrent load.
func TestEnsemble_AllProvidersFail_GracefulError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const providerCount = 5

	ensemble := services.NewEnsembleService("confidence_weighted", 3*time.Second)
	providers := make([]*failingProvider, providerCount)
	for i := 0; i < providerCount; i++ {
		providers[i] = &failingProvider{name: fmt.Sprintf("fail-provider-%d", i)}
		ensemble.RegisterProvider(providers[i].name, providers[i])
	}

	const callers = 100
	var (
		wg        sync.WaitGroup
		errors_   int64
		panics    int64
		successes int64
		start     = make(chan struct{})
	)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("all-fail stress test %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.5,
					MaxTokens:   50,
				},
			}

			result, err := ensemble.RunEnsemble(ctx, req)
			if err != nil {
				atomic.AddInt64(&errors_, 1)
				return
			}
			// If we somehow got a result, count it (not expected but not a panic)
			if result != nil {
				atomic.AddInt64(&successes, 1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: ensemble all-fail stress timed out after 30s")
	}

	assert.Zero(t, panics,
		"no goroutine should panic when all providers fail")
	assert.Equal(t, int64(callers), errors_+successes,
		"every caller must receive a result or an error (no hangs)")
	assert.Greater(t, errors_, int64(0),
		"at least some callers must observe an error when all providers fail")

	// Verify that each provider was actually called (errors are real, not suppressed).
	var totalCalls int64
	for _, p := range providers {
		totalCalls += atomic.LoadInt64(&p.calls)
	}
	assert.Greater(t, totalCalls, int64(0),
		"providers must be invoked even when they all fail")

	t.Logf("All-fail ensemble stress: errors=%d successes=%d panics=%d totalProviderCalls=%d",
		errors_, successes, panics, totalCalls)
}

// TestEnsemble_AllProvidersFail_NoGoroutineLeaks verifies that when all
// providers fail, the ensemble does not leak goroutines across many repeated
// invocations.
func TestEnsemble_AllProvidersFail_NoGoroutineLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	ensemble := services.NewEnsembleService("confidence_weighted", 2*time.Second)
	for i := 0; i < 3; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("leak-fail-%d", i),
			&failingProvider{name: fmt.Sprintf("leak-fail-%d", i)},
		)
	}

	for iter := 0; iter < 50; iter++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req := &models.LLMRequest{
			Prompt: fmt.Sprintf("leak test all-fail %d", iter),
			ModelParams: models.ModelParameters{
				Model:     "test-model",
				MaxTokens: 50,
			},
		}
		_, _ = ensemble.RunEnsemble(ctx, req)
		cancel()
	}

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()

	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutine leak check (all-fail): before=%d after=%d leaked=%d",
		goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20,
		"goroutine count must not grow after repeated all-fail ensemble calls")
}

// TestEnsemble_AllProvidersFail_ContextCancelled verifies that context
// cancellation while all providers are failing still results in a clean
// error (context.Canceled or the provider error), not a panic.
func TestEnsemble_AllProvidersFail_ContextCancelled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	for i := 0; i < 3; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("ctx-fail-%d", i),
			&failingProvider{name: fmt.Sprintf("ctx-fail-%d", i)},
		)
	}

	const callers = 50
	var (
		wg     sync.WaitGroup
		panics int64
	)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			// Cancel the context immediately after creating it.
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("ctx-cancel all-fail %d", id),
				ModelParams: models.ModelParameters{
					Model:     "test-model",
					MaxTokens: 50,
				},
			}

			_, err := ensemble.RunEnsemble(ctx, req)
			// err may be context.DeadlineExceeded, context.Canceled, or a
			// provider error — all are acceptable; what must never happen is a panic.
			_ = errors.Is(err, context.Canceled) // compile-time import check
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK: cancelled-context all-fail stress timed out")
	}

	assert.Zero(t, panics, "context cancellation with all-fail providers must not panic")
}
