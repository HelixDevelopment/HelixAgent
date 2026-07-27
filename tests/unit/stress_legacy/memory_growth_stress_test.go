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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// growthMockProvider is a minimal LLMProvider for memory growth testing.
// It returns a fixed-size response without blocking.
type growthMockProvider struct {
	name string
}

func (p *growthMockProvider) Complete(
	_ context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	return &models.LLMResponse{
		ID:           fmt.Sprintf("resp-%s", p.name),
		ProviderID:   p.name,
		ProviderName: p.name,
		Content:      "memory growth test response",
		Confidence:   0.9,
		TokensUsed:   20,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (p *growthMockProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, _ := p.Complete(ctx, req)
		select {
		case ch <- resp:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

// TestMemoryGrowth_10KOperations runs 10 000 ensemble operations and asserts
// that heap growth stays below 15% above the initial baseline. This detects
// steady-state memory leaks from retained objects, growing caches, or
// accumulating goroutine stacks.
func TestMemoryGrowth_10KOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		totalOps    = 10_000
		concurrency = 10 // controlled parallelism to stay within resource limits
	)

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("growth-provider-%d", i)
		ensemble.RegisterProvider(name, &growthMockProvider{name: name})
	}

	// Warm up: run a small batch first so that initialization allocations
	// do not skew the baseline.
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req := &models.LLMRequest{
			Prompt:      fmt.Sprintf("warmup %d", i),
			ModelParams: models.ModelParameters{Model: "test-model", MaxTokens: 20},
		}
		_, _ = ensemble.RunEnsemble(ctx, req)
		cancel()
	}

	// Capture baseline after warm-up + GC.
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	baselineHeapInuse := baseline.HeapInuse

	// Run 10 000 operations using a bounded semaphore (concurrency = 10).
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i := 0; i < totalOps; i++ {
		wg.Add(1)
		sem <- struct{}{} // acquire
		go func(id int) {
			defer wg.Done()
			defer func() { <-sem }() // release

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt:      fmt.Sprintf("memory growth op %d", id),
				ModelParams: models.ModelParameters{Model: "test-model", MaxTokens: 20},
			}
			_, _ = ensemble.RunEnsemble(ctx, req)
		}(i)
	}

	wg.Wait()

	// Force GC and measure heap after all operations.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	runtime.GC()

	var final runtime.MemStats
	runtime.ReadMemStats(&final)

	// Use HeapInuse (live objects only) to avoid counting GC-managed free pages.
	finalHeapInuse := final.HeapInuse
	growthBytes := int64(finalHeapInuse) - int64(baselineHeapInuse)
	growthPct := float64(growthBytes) / float64(baselineHeapInuse) * 100.0

	t.Logf("Memory growth after 10K ops: baseline=%.2f MB, final=%.2f MB, growth=%.2f MB (%.1f%%)",
		float64(baselineHeapInuse)/1024/1024,
		float64(finalHeapInuse)/1024/1024,
		float64(growthBytes)/1024/1024,
		growthPct,
	)

	// Numeric threshold: heap growth must be < 15% above initial baseline.
	assert.Less(t, growthPct, 15.0,
		"heap growth after 10K operations must be < 15%% (got %.1f%%) — possible memory leak",
		growthPct)
}
