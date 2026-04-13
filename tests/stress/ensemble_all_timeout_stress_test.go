//go:build stress
// +build stress

package stress

import (
	"context"
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

// timeoutProvider is a minimal LLMProvider that always blocks until its
// context is cancelled, simulating a provider that never responds.
type timeoutProvider struct {
	name  string
	calls int64
}

func (p *timeoutProvider) Complete(
	ctx context.Context, _ *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	// Block until context expires — simulates a hung provider.
	<-ctx.Done()
	return nil, fmt.Errorf("provider %s: context cancelled: %w", p.name, ctx.Err())
}

func (p *timeoutProvider) CompleteStream(
	ctx context.Context, _ *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	ch := make(chan *models.LLMResponse)
	go func() {
		defer close(ch)
		<-ctx.Done()
	}()
	return ch, nil
}

// TestEnsembleAllTimeout simulates all providers timing out and asserts that
// the ensemble returns a graceful error rather than hanging indefinitely.
// This is the primary ST5 resilience check: no deadlock when all providers
// time out simultaneously.
func TestEnsembleAllTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		providerCount   = 5
		ensembleTimeout = 500 * time.Millisecond // short ensemble timeout
		concurrency     = 30
	)

	ensemble := services.NewEnsembleService("confidence_weighted", ensembleTimeout)
	providers := make([]*timeoutProvider, providerCount)
	for i := 0; i < providerCount; i++ {
		name := fmt.Sprintf("timeout-provider-%d", i)
		providers[i] = &timeoutProvider{name: name}
		ensemble.RegisterProvider(name, providers[i])
	}

	goroutinesBefore := runtime.NumGoroutine()

	var (
		wg        sync.WaitGroup
		errors_   int64
		successes int64
		panics    int64
	)

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-start

			// Caller context is generous — the ensemble's internal timeout fires first.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("all-timeout stress %d", id),
				ModelParams: models.ModelParameters{
					Model:     "test-model",
					MaxTokens: 50,
				},
			}

			result, err := ensemble.RunEnsemble(ctx, req)
			if err != nil {
				atomic.AddInt64(&errors_, 1)
			} else if result != nil {
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

	// The ensemble timeout is 500ms per call; 30 concurrent calls should all
	// complete within 10s even if serialised.
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: ensemble all-timeout stress hung — providers must not block forever")
	}

	assert.Zero(t, panics, "no panics when all providers time out")

	// Key assertion: every caller must receive a result or error — no hangs.
	assert.Equal(t, int64(concurrency), errors_+successes,
		"all callers must complete when providers time out (no hang)")

	// All providers timed out — we expect errors, not successes.
	assert.Greater(t, errors_, int64(0),
		"at least some callers must observe an error when all providers time out")

	// Goroutine leak check: background provider goroutines must be cleaned up.
	runtime.GC()
	time.Sleep(500 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 30,
		"goroutines from timed-out providers must be cleaned up (no leak)")

	// Verify providers were actually invoked (not silently skipped).
	var totalCalls int64
	for _, p := range providers {
		totalCalls += atomic.LoadInt64(&p.calls)
	}
	assert.Greater(t, totalCalls, int64(0),
		"providers must have been invoked before timing out")

	t.Logf("Ensemble all-timeout: concurrency=%d, errors=%d, successes=%d, panics=%d, provider_calls=%d",
		concurrency, errors_, successes, panics, totalCalls)
}
