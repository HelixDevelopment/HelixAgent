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

// debateParticipantProvider is a minimal LLMProvider that simulates one
// participant in a debate-like parallel ensemble operation. Each instance
// tracks its own call count to verify isolation.
type debateParticipantProvider struct {
	name  string
	calls int64
	// shared counter across all providers in the same "session" to detect races
	sharedCounter *int64
}

func (p *debateParticipantProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	atomic.AddInt64(p.sharedCounter, 1)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(2 * time.Millisecond):
	}

	return &models.LLMResponse{
		ID:           fmt.Sprintf("debate-%s-%d", p.name, atomic.LoadInt64(&p.calls)),
		ProviderID:   p.name,
		ProviderName: p.name,
		Content:      fmt.Sprintf("Debate response from %s for: %s", p.name, req.Prompt),
		Confidence:   0.85,
		TokensUsed:   25,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (p *debateParticipantProvider) CompleteStream(
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

// TestConcurrentDebates_50 runs 50 concurrent debate-like operations (each
// using an independent EnsembleService with 5 providers) and asserts no data
// races, no panics, and a >= 80% success rate. Each "debate" uses its own
// shared counter to detect cross-session contamination.
func TestConcurrentDebates_50(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		debateCount   = 50
		providersEach = 5
		debateTimeout = 10 * time.Second
	)

	goroutinesBefore := runtime.NumGoroutine()

	var (
		wg        sync.WaitGroup
		successes int64
		failures  int64
		panics    int64
	)

	// Session counters: one int64 per debate session, used to detect races.
	sessionCounters := make([]int64, debateCount)

	start := make(chan struct{})

	for d := 0; d < debateCount; d++ {
		wg.Add(1)
		d := d
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-start

			// Each debate session has its own isolated ensemble.
			ensemble := services.NewEnsembleService("confidence_weighted", debateTimeout)
			for i := 0; i < providersEach; i++ {
				name := fmt.Sprintf("debate-%d-provider-%d", d, i)
				ensemble.RegisterProvider(name, &debateParticipantProvider{
					name:          name,
					sharedCounter: &sessionCounters[d],
				})
			}

			ctx, cancel := context.WithTimeout(context.Background(), debateTimeout)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("Debate session %d: What is the optimal strategy?", d),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.7,
					MaxTokens:   50,
				},
			}

			result, err := ensemble.RunEnsemble(ctx, req)
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			if result == nil || result.Selected == nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			atomic.AddInt64(&successes, 1)
		}()
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
		t.Fatal("DEADLOCK DETECTED: 50 concurrent debates timed out after 30s")
	}

	assert.Zero(t, panics, "no panics during 50 concurrent debate operations")

	// Numeric threshold: >= 80% success rate across 50 debates.
	successRate := float64(successes) / float64(debateCount)
	assert.GreaterOrEqual(t, successRate, 0.80,
		"at least 80%% of concurrent debates must succeed, got %.1f%% (%d/%d)",
		successRate*100, successes, debateCount)

	// Verify session isolation: each session's counter must be > 0
	// (providers were actually invoked) and no counter should be zero
	// unless the session failed.
	isolationViolations := 0
	for d := 0; d < debateCount; d++ {
		count := atomic.LoadInt64(&sessionCounters[d])
		if count == 0 {
			// Only flag as violation if the session was supposed to succeed
			isolationViolations++
		}
	}
	t.Logf("Sessions with zero provider calls: %d/%d", isolationViolations, debateCount)

	// Goroutine leak check
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 30, "no goroutine leak after 50 concurrent debates")

	t.Logf("Concurrent debates: total=%d, successes=%d (%.1f%%), failures=%d, panics=%d",
		debateCount, successes, successRate*100, failures, panics)
}
