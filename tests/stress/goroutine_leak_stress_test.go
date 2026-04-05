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

// loadProvider is a minimal LLMProvider for goroutine leak detection tests.
// It completes quickly without holding resources.
type loadProvider struct {
	name  string
	calls int64
}

func (p *loadProvider) Complete(
	ctx context.Context, _ *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Millisecond):
	}
	return &models.LLMResponse{
		ID:           fmt.Sprintf("resp-%s-%d", p.name, atomic.LoadInt64(&p.calls)),
		ProviderID:   p.name,
		ProviderName: p.name,
		Content:      "goroutine leak test response",
		Confidence:   0.8,
		TokensUsed:   10,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (p *loadProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, err := p.Complete(ctx, req)
		if err != nil {
			return
		}
		select {
		case ch <- resp:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

// TestGoroutineLeak_LoadThenStop applies sustained load to the ensemble
// service, then stops all activity and asserts the goroutine count returns
// to within ±10 of the baseline. This catches goroutines that are spawned
// per-request but never properly terminated.
func TestGoroutineLeak_LoadThenStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		providerCount = 3
		loadDuration  = 3 * time.Second
		concurrency   = 8 // bounded to respect resource limits
		leakTolerance = 10
	)

	ensemble := services.NewEnsembleService("confidence_weighted", 2*time.Second)
	for i := 0; i < providerCount; i++ {
		name := fmt.Sprintf("leak-provider-%d", i)
		ensemble.RegisterProvider(name, &loadProvider{name: name})
	}

	// Settle and record baseline.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	t.Logf("Goroutine baseline (before load): %d", baseline)

	// Apply sustained load for loadDuration.
	ctx, cancel := context.WithTimeout(context.Background(), loadDuration)
	defer cancel()

	var wg sync.WaitGroup
	var totalOps int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				reqCtx, reqCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
				req := &models.LLMRequest{
					Prompt:      fmt.Sprintf("leak test worker %d op %d", id, atomic.LoadInt64(&totalOps)),
					ModelParams: models.ModelParameters{Model: "test-model", MaxTokens: 10},
				}
				_, _ = ensemble.RunEnsemble(reqCtx, req)
				reqCancel()
				atomic.AddInt64(&totalOps, 1)
			}
		}(i)
	}

	// Wait for load phase to complete.
	<-ctx.Done()

	loadDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(loadDone)
	}()

	select {
	case <-loadDone:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: load goroutines did not exit after context cancellation")
	}

	// Allow background cleanup and GC to settle.
	runtime.GC()
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()
	delta := after - baseline

	t.Logf("Goroutines: baseline=%d, after_load=%d, delta=%d (ops=%d)",
		baseline, after, delta, totalOps)

	// Numeric threshold: goroutine count must return to baseline ± leakTolerance.
	assert.LessOrEqual(t, delta, leakTolerance,
		"goroutine count must return to baseline ±%d after load stops (delta=%d) — possible goroutine leak",
		leakTolerance, delta)

	assert.Greater(t, totalOps, int64(0),
		"at least some operations must have completed during load phase")
}
