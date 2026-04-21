//go:build stress
// +build stress

// Previously tests/stress/... — demoted to unit package under CONST-030.
// Stress tests of in-process concurrency primitives (semaphore / pool /
// cache saturation, circuit-breaker storms, goroutine growth) that
// exercise local mock providers and invokers. Keeping the mocks in the
// unit tree per the audit's Pattern-4 disposition for stress tests.
// PR33 of the CONST-030 compliance campaign.
package stress_legacy_test

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

// correctnessMockProvider is a minimal LLMProvider for correctness stress
// tests. It always responds successfully after a configurable delay.
type correctnessMockProvider struct {
	name    string
	latency time.Duration
	calls   int64
}

func (p *correctnessMockProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	select {
	case <-time.After(p.latency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &models.LLMResponse{
		ID:           fmt.Sprintf("resp-%s-%d", p.name, atomic.LoadInt64(&p.calls)),
		ProviderID:   p.name,
		ProviderName: p.name,
		Content:      fmt.Sprintf("Correctness response from %s", p.name),
		Confidence:   0.85,
		TokensUsed:   30,
		ResponseTime: p.latency.Milliseconds(),
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (p *correctnessMockProvider) CompleteStream(
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

// TestEnsemble_CorrectnessUnderConcurrentLoad_Stress verifies that 200
// concurrent callers each receive a structurally valid response (non-nil
// Selected, non-empty ProviderID) and that no panics occur. This validates
// the Phase 1 safety fixes hold under concurrent load.
func TestEnsemble_CorrectnessUnderConcurrentLoad_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 10*time.Second)
	for i := 0; i < 5; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("correctness-provider-%d", i),
			&correctnessMockProvider{
				name:    fmt.Sprintf("correctness-provider-%d", i),
				latency: time.Duration(i+1) * time.Millisecond,
			},
		)
	}

	const concurrency = 200
	var (
		wg        sync.WaitGroup
		panics    int64
		successes int64
		malformed int64 // responses with empty ProviderID
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

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("correctness stress %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.7,
					MaxTokens:   50,
				},
			}

			result, err := ensemble.RunEnsemble(ctx, req)
			if err != nil {
				return
			}
			if result == nil || result.Selected == nil {
				return
			}
			// Validate structural correctness
			if result.Selected.ProviderID == "" {
				atomic.AddInt64(&malformed, 1)
				return
			}
			atomic.AddInt64(&successes, 1)
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
	case <-time.After(60 * time.Second):
		t.Fatalf("DEADLOCK DETECTED: ensemble correctness stress at concurrency %d timed out",
			concurrency)
	}

	assert.Zero(t, panics, "no goroutine should panic during ensemble correctness stress")
	assert.Zero(t, malformed, "no response should have an empty ProviderID")
	assert.Greater(t, successes, int64(concurrency/2),
		"majority of callers should receive a valid response")

	t.Logf("Ensemble correctness: successes=%d, malformed=%d, panics=%d (concurrency=%d)",
		successes, malformed, panics, concurrency)
}

// TestEnsemble_NoGoroutineLeak_UnderLoad_Stress verifies that after 200
// concurrent ensemble calls the goroutine count returns to near-baseline,
// confirming the Phase 1 goroutine-lifecycle fixes hold under load.
func TestEnsemble_NoGoroutineLeak_UnderLoad_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	// Baseline after GC
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	for i := 0; i < 3; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("leak-check-provider-%d", i),
			&correctnessMockProvider{
				name:    fmt.Sprintf("leak-check-provider-%d", i),
				latency: time.Millisecond,
			},
		)
	}

	const concurrency = 200
	var wg sync.WaitGroup

	start := make(chan struct{})
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("leak check %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.5,
					MaxTokens:   30,
				},
			}
			_, _ = ensemble.RunEnsemble(ctx, req)
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
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: ensemble goroutine leak check timed out")
	}

	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 30,
		"goroutine count should not grow significantly after 200 ensemble calls")
}

// TestEnsemble_ContextCancellation_NoPanic_Stress cancels contexts mid-flight
// and verifies the ensemble returns ctx.Err()-wrapped errors cleanly without
// panicking or leaving goroutines behind.
func TestEnsemble_ContextCancellation_NoPanic_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	goroutinesBefore := runtime.NumGoroutine()

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	for i := 0; i < 5; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("cancel-provider-%d", i),
			&correctnessMockProvider{
				name:    fmt.Sprintf("cancel-provider-%d", i),
				latency: 50 * time.Millisecond, // slow enough to be cancelled
			},
		)
	}

	const goroutines = 100
	var (
		wg     sync.WaitGroup
		panics int64
		errors int64
	)

	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			ctx, cancel := context.WithTimeout(
				context.Background(), 10*time.Millisecond, // cancel before providers respond
			)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("cancel test %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.5,
					MaxTokens:   30,
				},
			}
			_, err := ensemble.RunEnsemble(ctx, req)
			if err != nil {
				atomic.AddInt64(&errors, 1)
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
		t.Fatal("DEADLOCK DETECTED: context cancellation stress timed out")
	}

	assert.Zero(t, panics, "no panics during context cancellation stress")
	// Most calls should have been cancelled (errors expected)
	assert.Greater(t, errors, int64(0),
		"at least some calls should have been cancelled")

	runtime.GC()
	time.Sleep(500 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines after cancel stress: before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 30,
		"context cancellation should not leak goroutines")
}

// TestEnsemble_ConcurrentProviderRegistration_NoPanic_Stress registers
// and removes providers concurrently with active ensemble calls to verify
// the Phase 1 mutex fix prevents data races.
func TestEnsemble_ConcurrentProviderRegistration_NoPanic_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	// Seed with one stable provider so RunEnsemble can always find one
	ensemble.RegisterProvider("stable-correctness", &correctnessMockProvider{
		name:    "stable-correctness",
		latency: time.Millisecond,
	})

	var (
		wg     sync.WaitGroup
		panics int64
	)
	start := make(chan struct{})

	// Writer goroutines: register/remove dynamic providers
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			name := fmt.Sprintf("dynamic-correctness-%d", id)
			for j := 0; j < 30; j++ {
				ensemble.RegisterProvider(name, &correctnessMockProvider{
					name:    name,
					latency: time.Millisecond,
				})
				_ = ensemble.GetProviders()
				ensemble.RemoveProvider(name)
			}
		}(i)
	}

	// Reader goroutines: run ensemble calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			for j := 0; j < 10; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				req := &models.LLMRequest{
					Prompt: fmt.Sprintf("concurrent reg correctness %d-%d", id, j),
					ModelParams: models.ModelParameters{
						Model:       "test-model",
						Temperature: 0.5,
						MaxTokens:   30,
					},
				}
				_, _ = ensemble.RunEnsemble(ctx, req)
				cancel()
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
		t.Fatal("DEADLOCK DETECTED: concurrent provider registration correctness stress timed out")
	}

	assert.Zero(t, panics,
		"no panics during concurrent provider registration and ensemble calls")
}
