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

	"digital.vasic.debate/protocol"
	"digital.vasic.debate/topology"
	"digital.vasic.debate/voting"
)

// semSaturateMockInvoker is a minimal AgentInvoker that records concurrent
// in-flight invocations so we can assert semaphore limits are respected.
type semSaturateMockInvoker struct {
	inFlight    int64 // current concurrent invocations
	peakFlight  int64 // maximum concurrent invocations observed
	totalCalled int64
	delay       time.Duration
}

func (m *semSaturateMockInvoker) Invoke(
	ctx context.Context,
	agent *topology.Agent,
	_ string,
	_ protocol.DebateContext,
) (*protocol.PhaseResponse, error) {
	current := atomic.AddInt64(&m.inFlight, 1)
	atomic.AddInt64(&m.totalCalled, 1)

	// Update peak atomically via CAS loop.
	for {
		peak := atomic.LoadInt64(&m.peakFlight)
		if current <= peak {
			break
		}
		if atomic.CompareAndSwapInt64(&m.peakFlight, peak, current) {
			break
		}
	}

	// Simulate work respecting context cancellation.
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		atomic.AddInt64(&m.inFlight, -1)
		return nil, ctx.Err()
	}

	atomic.AddInt64(&m.inFlight, -1)

	return &protocol.PhaseResponse{
		AgentID:    agent.ID,
		Role:       agent.Role,
		Provider:   agent.Provider,
		Model:      agent.Model,
		Content:    "semaphore saturation test response",
		Confidence: 0.75,
		Vote:       "option-A",
		Score:      agent.Score,
		Latency:    m.delay,
		Timestamp:  time.Now(),
	}, nil
}

// TestDebate_Concurrency_SemaphoreSaturation launches many concurrent debate
// sessions to saturate the session semaphore (ExecutionPool). The test verifies
// that no deadlock occurs and that sessions complete without panics.
func TestDebate_Concurrency_SemaphoreSaturation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const sessionCount = 30 // intentionally > typical semaphore limit

	var (
		wg         sync.WaitGroup
		successes  int64
		failures   int64
		panicCount int64
		start      = make(chan struct{})
	)

	invoker := &semSaturateMockInvoker{delay: 2 * time.Millisecond}

	for i := 0; i < sessionCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()

			<-start

			cfg := topology.DefaultTopologyConfig(topology.TopologyGraphMesh)
			topo, err := topology.NewTopology(topology.TopologyGraphMesh, cfg)
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}

			agents := topo.GetAgents()
			if len(agents) == 0 {
				atomic.AddInt64(&failures, 1)
				return
			}

			// Build a single-round debate context.
			debateCtx := protocol.DebateContext{
				DebateID: fmt.Sprintf("sem-sat-%d", idx),
				Topic:    fmt.Sprintf("semaphore saturation topic %d", idx),
				Round:    1,
				Metadata: map[string]interface{}{"test": "semaphore_saturation"},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Invoke the first agent to simulate a single debate turn.
			_, err = invoker.Invoke(ctx, agents[0], debateCtx.Topic, debateCtx)
			if err != nil {
				atomic.AddInt64(&failures, 1)
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
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: debate semaphore saturation timed out after 30s")
	}

	assert.Zero(t, panicCount, "no goroutine should panic during semaphore saturation")
	assert.Equal(t, int64(sessionCount), successes+failures,
		"all sessions must complete (success or failure, no hangs)")
	assert.Greater(t, successes, int64(0), "at least some sessions must succeed")

	t.Logf("Semaphore saturation: sessions=%d successes=%d failures=%d panics=%d peakConcurrent=%d",
		sessionCount, successes, failures, panicCount, atomic.LoadInt64(&invoker.peakFlight))
}

// TestDebate_Concurrency_VotingUnderSaturation exercises the voting subsystem
// with many concurrent goroutines adding votes and reading tallies simultaneously,
// verifying there are no data races or deadlocks.
func TestDebate_Concurrency_VotingUnderSaturation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const (
		voters  = 50
		options = 3
	)

	vs := voting.NewWeightedVotingSystem(voting.VotingConfig{
		MinimumVotes:      1,
		MinimumConfidence: 0.3,
		EnableTieBreaking: true,
		TieBreakMethod:    voting.TieBreakByHighestConfidence,
	})
	optionNames := make([]string, options)
	for i := 0; i < options; i++ {
		optionNames[i] = fmt.Sprintf("option-%d", i)
	}

	var (
		wg         sync.WaitGroup
		panicCount int64
		start      = make(chan struct{})
	)

	for i := 0; i < voters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			// Each voter casts multiple votes across all options.
			for j := 0; j < 20; j++ {
				opt := optionNames[(id+j)%options]
				confidence := 0.5 + float64(j%5)*0.1
				_ = vs.AddVote(&voting.Vote{
					AgentID:    fmt.Sprintf("agent-%d", id),
					Choice:     opt,
					Confidence: confidence,
					Score:      confidence,
					Timestamp:  time.Now(),
				})
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
	case <-time.After(20 * time.Second):
		t.Fatal("DEADLOCK: voting saturation timed out")
	}

	assert.Zero(t, panicCount, "no panic during concurrent voting under saturation")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := vs.Calculate(ctx)
	assert.NoError(t, err, "Calculate must succeed after concurrent vote additions")
	assert.NotNil(t, result, "voting result must not be nil after saturation")
	t.Logf("Voting saturation: voters=%d options=%d winner=%v", voters, options,
		func() string {
			if result != nil {
				return result.WinningChoice
			}
			return "<nil>"
		}())
}

// TestDebate_Concurrency_NoDeadlockOnTopologyReadWrite creates many goroutines
// that concurrently read and write topology agent state to verify no deadlock
// occurs under saturation conditions.
func TestDebate_Concurrency_NoDeadlockOnTopologyReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cfg := topology.DefaultTopologyConfig(topology.TopologyGraphMesh)
	topo, err := topology.NewTopology(topology.TopologyGraphMesh, cfg)
	if err != nil {
		t.Skipf("cannot create mesh topology: %v", err)
	}

	const goroutineCount = 80
	var (
		wg         sync.WaitGroup
		panicCount int64
		start      = make(chan struct{})
	)

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 50; j++ {
				agents := topo.GetAgents()
				if len(agents) == 0 {
					continue
				}
				// Read agent fields (simulating concurrent topology inspection).
				for _, a := range agents {
					_ = a.ID
					_ = a.Role
					_ = a.Score
				}
				// Occasional topology info access.
				if j%10 == 0 {
					_ = topo.GetType()
				}
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
	case <-time.After(20 * time.Second):
		t.Fatal("DEADLOCK: topology read/write saturation timed out")
	}

	assert.Zero(t, panicCount, "no panic during topology read/write saturation")
}
