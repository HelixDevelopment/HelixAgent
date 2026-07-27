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
	"github.com/stretchr/testify/require"

	"digital.vasic.debate/protocol"
	"digital.vasic.debate/reflexion"
	"digital.vasic.debate/topology"
	"digital.vasic.debate/voting"
)

// Resource limit per CLAUDE.md rule 15
func init() {
	runtime.GOMAXPROCS(2)
}

// stressMockInvoker is a protocol.AgentInvoker with configurable delay.
type stressMockInvoker struct {
	invocations int64
}

func (m *stressMockInvoker) Invoke(
	ctx context.Context,
	agent *topology.Agent,
	prompt string,
	debateCtx protocol.DebateContext,
) (*protocol.PhaseResponse, error) {
	atomic.AddInt64(&m.invocations, 1)

	return &protocol.PhaseResponse{
		AgentID:    agent.ID,
		Role:       agent.Role,
		Provider:   agent.Provider,
		Model:      agent.Model,
		Content:    "Stress test response",
		Confidence: 0.7,
		Vote:       "solution-A",
		Score:      agent.Score,
		Latency:    1 * time.Millisecond,
		Timestamp:  time.Now(),
	}, nil
}

// TestDebate_ConcurrentVoting tests the voting system under heavy
// concurrent access from 50 goroutines simultaneously adding votes
// and calculating results.
func TestDebate_ConcurrentVoting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	config := voting.VotingConfig{
		MinimumVotes:         3,
		MinimumConfidence:    0.1,
		EnableDiversityBonus: true,
		DiversityWeight:      0.1,
		EnableTieBreaking:    true,
		TieBreakMethod:       voting.TieBreakByHighestConfidence,
	}

	voteSystem := voting.NewWeightedVotingSystem(config)

	const goroutineCount = 50
	var wg sync.WaitGroup
	var addErrors int64
	var calcErrors int64
	var calcSuccesses int64

	// Pre-seed some votes so Calculate can work
	for i := 0; i < 5; i++ {
		err := voteSystem.AddVote(&voting.Vote{
			AgentID:    fmt.Sprintf("seed-%d", i),
			Choice:     "choice-A",
			Confidence: 0.8,
			Score:      7.5,
			Timestamp:  time.Now(),
		})
		require.NoError(t, err)
	}

	// Launch goroutines that concurrently add votes and calculate
	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine adds a vote
			err := voteSystem.AddVote(&voting.Vote{
				AgentID:        fmt.Sprintf("agent-%d", id),
				Choice:         fmt.Sprintf("choice-%c", 'A'+rune(id%5)),
				Confidence:     0.5 + float64(id%50)/100.0,
				Score:          6.0 + float64(id%40)/10.0,
				Specialization: fmt.Sprintf("spec-%d", id%3),
				Role:           fmt.Sprintf("role-%d", id%4),
				Timestamp:      time.Now(),
			})
			if err != nil {
				atomic.AddInt64(&addErrors, 1)
				return
			}

			// Every 5th goroutine also calculates
			if id%5 == 0 {
				ctx := context.Background()
				_, calcErr := voteSystem.Calculate(ctx)
				if calcErr != nil {
					atomic.AddInt64(&calcErrors, 1)
				} else {
					atomic.AddInt64(&calcSuccesses, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent voting: %d goroutines, add_errors=%d, "+
		"calc_errors=%d, calc_successes=%d",
		goroutineCount, addErrors, calcErrors, calcSuccesses)

	// No add errors should occur
	assert.Equal(t, int64(0), addErrors,
		"No vote add errors should occur under concurrency")

	// Final calculation should succeed
	ctx := context.Background()
	result, err := voteSystem.Calculate(ctx)
	require.NoError(t, err, "Final calculation should succeed")
	require.NotNil(t, result)

	assert.Greater(t, result.TotalVotes, 0,
		"Should have recorded votes")
	assert.NotEmpty(t, result.WinningChoice,
		"Should have a winning choice")

	// Statistics should be consistent
	stats := voteSystem.GetStatistics()
	assert.Greater(t, stats.TotalVotes, 0,
		"Statistics should show votes")
	assert.Greater(t, stats.AvgConfidence, 0.0,
		"Average confidence should be positive")
}

// TestDebate_MemoryUnderLoad creates many episodes in the episodic
// memory buffer and verifies that memory usage stays bounded.
func TestDebate_MemoryUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	maxSize := 500
	memory := reflexion.NewEpisodicMemoryBuffer(maxSize)

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	const totalEpisodes = 5000
	var wg sync.WaitGroup
	var storeErrors int64

	// Concurrently store episodes from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < totalEpisodes/10; j++ {
				ep := &reflexion.Episode{
					AgentID:         fmt.Sprintf("agent-%d", workerID),
					SessionID:       fmt.Sprintf("session-%d-%d", workerID, j),
					TaskDescription: fmt.Sprintf("Task from worker %d iteration %d", workerID, j),
					AttemptNumber:   j + 1,
					Code:            fmt.Sprintf("func worker%d_%d() {}", workerID, j),
					Confidence:      float64(j%100) / 100.0,
					Timestamp:       time.Now(),
				}
				if err := memory.Store(ep); err != nil {
					atomic.AddInt64(&storeErrors, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	// Memory should be bounded at maxSize
	assert.LessOrEqual(t, memory.Size(), maxSize,
		"Buffer size should not exceed max")

	liveHeapMB := float64(memAfter.HeapInuse) / 1024 / 1024
	t.Logf("Memory under load: stored %d episodes (max %d), "+
		"store_errors=%d, live heap=%.2f MB",
		totalEpisodes, maxSize, storeErrors, liveHeapMB)

	// Live heap should be reasonable after GC
	assert.Less(t, liveHeapMB, 200.0,
		"Memory increase should be bounded")

	// Verify retrieval still works
	recent := memory.GetRecent(10)
	assert.LessOrEqual(t, len(recent), 10,
		"GetRecent should return at most 10 episodes")
}

// TestDebate_DeadlockDetection runs concurrent topology operations
// (add agents, assign roles, get agents, close) to detect potential
// deadlocks in the topology implementation.
func TestDebate_DeadlockDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	cfg := topology.DefaultTopologyConfig(topology.TopologyGraphMesh)
	topo, err := topology.NewTopology(topology.TopologyGraphMesh, cfg)
	require.NoError(t, err)

	// Initialize with a base set of agents
	baseAgents := []*topology.Agent{
		topology.CreateAgentFromSpec("dl-1", topology.RoleModerator,
			"mock", "m", 8.0, "reasoning"),
		topology.CreateAgentFromSpec("dl-2", topology.RoleProposer,
			"mock", "m", 7.5, "code"),
		topology.CreateAgentFromSpec("dl-3", topology.RoleCritic,
			"mock", "m", 7.0, "reasoning"),
		topology.CreateAgentFromSpec("dl-4", topology.RoleReviewer,
			"mock", "m", 7.2, "reasoning"),
		topology.CreateAgentFromSpec("dl-5", topology.RoleOptimizer,
			"mock", "m", 7.1, "code"),
	}
	err = topo.Initialize(context.Background(), baseAgents)
	require.NoError(t, err)

	done := make(chan struct{})

	// Set a hard deadline for deadlock detection
	go func() {
		select {
		case <-done:
			return
		case <-time.After(30 * time.Second):
			t.Error("Potential deadlock detected: operations did not " +
				"complete within 30 seconds")
		}
	}()

	var wg sync.WaitGroup
	const goroutineCount = 20

	// Readers: repeatedly get agents and metrics
	for i := 0; i < goroutineCount/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = topo.GetAgents()
				_ = topo.GetAgentsByRole(topology.AgentRole(
					fmt.Sprintf("role-%d", j%5)))
				_, _ = topo.GetAgent(fmt.Sprintf("dl-%d", j%5+1))
				_ = topo.GetMetrics()
				_ = topo.GetChannels()
			}
		}(i)
	}

	// Writers: assign roles concurrently
	for i := 0; i < goroutineCount/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			roles := []topology.AgentRole{
				topology.RoleProposer,
				topology.RoleCritic,
				topology.RoleReviewer,
				topology.RoleOptimizer,
				topology.RoleModerator,
			}
			for j := 0; j < 50; j++ {
				agentID := fmt.Sprintf("dl-%d", j%5+1)
				role := roles[j%len(roles)]
				_ = topo.AssignRole(agentID, role)
			}
		}(i)
	}

	wg.Wait()
	close(done)

	// If we reach here, no deadlock occurred
	err = topo.Close()
	assert.NoError(t, err, "Topology close should not error")

	t.Logf("Deadlock detection: %d goroutines completed without deadlock",
		goroutineCount)
}

// TestDebate_ConcurrentProtocolExecution runs multiple protocol instances
// concurrently to verify they do not interfere with each other.
func TestDebate_ConcurrentProtocolExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	const debateCount = 5
	var wg sync.WaitGroup
	var successes int64
	var failures int64

	for i := 0; i < debateCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			cfg := topology.DefaultTopologyConfig(topology.TopologyGraphMesh)
			topo, err := topology.NewTopology(topology.TopologyGraphMesh, cfg)
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			defer topo.Close()

			agents := []*topology.Agent{
				topology.CreateAgentFromSpec(
					fmt.Sprintf("p%d-mod", idx), topology.RoleModerator,
					"mock", "m", 8.0, "reasoning"),
				topology.CreateAgentFromSpec(
					fmt.Sprintf("p%d-prop", idx), topology.RoleProposer,
					"mock", "m", 7.5, "code"),
				topology.CreateAgentFromSpec(
					fmt.Sprintf("p%d-crit", idx), topology.RoleCritic,
					"mock", "m", 7.0, "reasoning"),
				topology.CreateAgentFromSpec(
					fmt.Sprintf("p%d-rev", idx), topology.RoleReviewer,
					"mock", "m", 7.2, "reasoning"),
				topology.CreateAgentFromSpec(
					fmt.Sprintf("p%d-opt", idx), topology.RoleOptimizer,
					"mock", "m", 7.1, "code"),
			}

			if err := topo.Initialize(context.Background(), agents); err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}

			invoker := &stressMockInvoker{}
			protoCfg := protocol.DefaultDebateConfig()
			protoCfg.Topic = fmt.Sprintf("Stress debate %d", idx)
			protoCfg.MaxRounds = 1
			protoCfg.Timeout = 30 * time.Second

			proto := protocol.NewProtocol(protoCfg, topo, invoker)
			result, err := proto.Execute(context.Background())

			if err != nil || result == nil || !result.Success {
				atomic.AddInt64(&failures, 1)
			} else {
				atomic.AddInt64(&successes, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent protocols: %d successes, %d failures out of %d",
		successes, failures, debateCount)

	assert.Equal(t, int64(debateCount), successes,
		"All concurrent protocols should succeed")
	assert.Equal(t, int64(0), failures,
		"No concurrent protocols should fail")
}

// TestDebate_VotingSystemReset verifies that the voting system can be
// reset and reused many times without resource leaks.
func TestDebate_VotingSystemReset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	config := voting.DefaultVotingConfig()
	voteSystem := voting.NewWeightedVotingSystem(config)

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		// Add votes
		for j := 0; j < 10; j++ {
			_ = voteSystem.AddVote(&voting.Vote{
				AgentID:    fmt.Sprintf("agent-%d", j),
				Choice:     fmt.Sprintf("choice-%d", j%3),
				Confidence: 0.7,
				Score:      7.0,
				Timestamp:  time.Now(),
			})
		}

		// Calculate
		ctx := context.Background()
		_, _ = voteSystem.Calculate(ctx)

		// Reset
		voteSystem.Reset()
	}

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	liveHeapMB := float64(memAfter.HeapInuse) / 1024 / 1024
	t.Logf("Voting reset stress: %d iterations, live heap=%.2f MB",
		iterations, liveHeapMB)

	// After 1000 resets, live heap should be bounded
	assert.Less(t, liveHeapMB, 200.0,
		"Memory should not grow unbounded after repeated resets")

	// Final vote count should be 0 after last reset
	assert.Equal(t, 0, voteSystem.VoteCount(),
		"Vote count should be 0 after reset")
}
