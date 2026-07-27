// Previously tests/stress/debate_concurrent_stress_test.go — sibling
// depends on stressMockInvoker defined in debate_stress_test.go; moved
// together with PR33 batch.
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
	"digital.vasic.debate/topology"
	"digital.vasic.debate/voting"
)

// TestStress_DebateSessions_ConcurrentExecution runs 5 concurrent debate
// sessions (each with its own topology, protocol, and invoker) to verify
// that sessions do not interfere with each other and goroutine count
// remains stable afterward.
func TestStress_DebateSessions_ConcurrentExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const sessionCount = 5

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var successes, failures, panicCount int64

	// Memory baseline
	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	start := make(chan struct{})

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
			defer topo.Close()

			agents := []*topology.Agent{
				topology.CreateAgentFromSpec(
					fmt.Sprintf("s%d-mod", idx), topology.RoleModerator,
					"mock", "m", 8.0, "reasoning"),
				topology.CreateAgentFromSpec(
					fmt.Sprintf("s%d-prop", idx), topology.RoleProposer,
					"mock", "m", 7.5, "code"),
				topology.CreateAgentFromSpec(
					fmt.Sprintf("s%d-crit", idx), topology.RoleCritic,
					"mock", "m", 7.0, "reasoning"),
				topology.CreateAgentFromSpec(
					fmt.Sprintf("s%d-rev", idx), topology.RoleReviewer,
					"mock", "m", 7.2, "reasoning"),
				topology.CreateAgentFromSpec(
					fmt.Sprintf("s%d-opt", idx), topology.RoleOptimizer,
					"mock", "m", 7.1, "code"),
			}

			if err := topo.Initialize(context.Background(), agents); err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}

			invoker := &stressMockInvoker{}
			protoCfg := protocol.DefaultDebateConfig()
			protoCfg.Topic = fmt.Sprintf("Concurrent session %d: code quality", idx)
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

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent debate sessions timed out")
	}

	// Goroutine leak check
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	// Memory check
	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)
	// Use signed arithmetic to handle GC reclaiming memory (after < before)
	heapGrowthMB := float64(int64(memAfter.HeapInuse)-int64(memBefore.HeapInuse)) / 1024 / 1024

	assert.Zero(t, panicCount, "no panics during concurrent debate sessions")
	assert.Equal(t, int64(sessionCount), successes,
		"all concurrent debate sessions should succeed")
	assert.Zero(t, failures, "no debate session failures expected")
	assert.Less(t, leaked, 20,
		"goroutine count should stabilize after concurrent debates")
	assert.Less(t, heapGrowthMB, 200.0,
		"heap growth should be bounded after concurrent debates")
	t.Logf("Debate sessions stress: successes=%d, failures=%d, panics=%d, "+
		"goroutine_leak=%d, heap_growth=%.2fMB",
		successes, failures, panicCount, leaked, heapGrowthMB)
}

// TestStress_DebateVoting_HighConcurrency tests the voting system under
// heavy concurrent access with 50 goroutines simultaneously adding votes
// and querying results, verifying thread safety and correctness.
func TestStress_DebateVoting_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := voting.VotingConfig{
		MinimumVotes:         3,
		MinimumConfidence:    0.1,
		EnableDiversityBonus: true,
		DiversityWeight:      0.1,
		EnableTieBreaking:    true,
		TieBreakMethod:       voting.TieBreakByHighestConfidence,
	}

	voteSystem := voting.NewWeightedVotingSystem(config)

	// Seed initial votes so Calculate can work
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

	const goroutineCount = 50

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount, addErrors, calcErrors, calcSuccesses int64

	start := make(chan struct{})

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

			// Add a vote
			err := voteSystem.AddVote(&voting.Vote{
				AgentID:    fmt.Sprintf("stress-agent-%d", id),
				Choice:     fmt.Sprintf("choice-%c", 'A'+rune(id%4)),
				Confidence: 0.5 + float64(id%50)/100.0,
				Score:      6.0 + float64(id%40)/10.0,
				Timestamp:  time.Now(),
			})
			if err != nil {
				atomic.AddInt64(&addErrors, 1)
			}

			// Every 3rd goroutine also calculates
			if id%3 == 0 {
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

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: debate voting high concurrency timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount, "no panics during concurrent voting")
	assert.Zero(t, addErrors, "no vote add errors under concurrency")

	// Final calculation should succeed
	ctx := context.Background()
	result, err := voteSystem.Calculate(ctx)
	require.NoError(t, err, "final vote calculation should succeed")
	require.NotNil(t, result)
	assert.Greater(t, result.TotalVotes, 0, "should have recorded votes")
	assert.NotEmpty(t, result.WinningChoice, "should determine a winner")
	assert.Less(t, leaked, 10,
		"goroutine count should stabilize after voting stress")

	t.Logf("Debate voting stress: adds=%d, add_errors=%d, "+
		"calc_successes=%d, calc_errors=%d, panics=%d, goroutine_leak=%d",
		goroutineCount, addErrors, calcSuccesses, calcErrors, panicCount, leaked)
}

// TestStress_DebateTopology_ConcurrentReadWrite hammers a topology instance
// with concurrent agent lookups and role assignments to detect deadlocks
// in the topology's internal locking.
func TestStress_DebateTopology_ConcurrentReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cfg := topology.DefaultTopologyConfig(topology.TopologyGraphMesh)
	topo, err := topology.NewTopology(topology.TopologyGraphMesh, cfg)
	require.NoError(t, err)

	agents := []*topology.Agent{
		topology.CreateAgentFromSpec("rw-1", topology.RoleModerator,
			"mock", "m", 8.0, "reasoning"),
		topology.CreateAgentFromSpec("rw-2", topology.RoleProposer,
			"mock", "m", 7.5, "code"),
		topology.CreateAgentFromSpec("rw-3", topology.RoleCritic,
			"mock", "m", 7.0, "reasoning"),
		topology.CreateAgentFromSpec("rw-4", topology.RoleReviewer,
			"mock", "m", 7.2, "reasoning"),
		topology.CreateAgentFromSpec("rw-5", topology.RoleOptimizer,
			"mock", "m", 7.1, "code"),
	}
	err = topo.Initialize(context.Background(), agents)
	require.NoError(t, err)

	const (
		readerCount = 30
		writerCount = 20
	)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount int64
	var readOps, writeOps int64

	start := make(chan struct{})

	// Readers
	for i := 0; i < readerCount; i++ {
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
				_ = topo.GetAgents()
				_, _ = topo.GetAgent(fmt.Sprintf("rw-%d", j%5+1))
				_ = topo.GetAgentsByRole(topology.RoleProposer)
				_ = topo.GetMetrics()
				_ = topo.GetChannels()
				atomic.AddInt64(&readOps, 1)
			}
		}(i)
	}

	// Writers
	for i := 0; i < writerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			roles := []topology.AgentRole{
				topology.RoleProposer,
				topology.RoleCritic,
				topology.RoleReviewer,
				topology.RoleOptimizer,
				topology.RoleModerator,
			}
			for j := 0; j < 30; j++ {
				agentID := fmt.Sprintf("rw-%d", j%5+1)
				role := roles[j%len(roles)]
				_ = topo.AssignRole(agentID, role)
				atomic.AddInt64(&writeOps, 1)
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
		t.Fatal("DEADLOCK DETECTED: debate topology concurrent read/write timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	err = topo.Close()
	assert.NoError(t, err, "topology close should not error")

	assert.Zero(t, panicCount, "no panics during topology concurrent read/write")
	assert.Equal(t, int64(readerCount*50), readOps, "all read operations complete")
	assert.Equal(t, int64(writerCount*30), writeOps, "all write operations complete")
	assert.Less(t, leaked, 15,
		"goroutines should not leak after topology stress")
	t.Logf("Debate topology stress: reads=%d, writes=%d, panics=%d, goroutine_leak=%d",
		readOps, writeOps, panicCount, leaked)
}

// TestStress_DebateMemory_NoLeaksAfterSessions creates and tears down
// multiple debate sessions in sequence, tracking memory to ensure no
// progressive leak from accumulated session state.
func TestStress_DebateMemory_NoLeaksAfterSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	goroutinesBefore := runtime.NumGoroutine()

	const iterations = 10
	var totalSuccesses int64

	for iter := 0; iter < iterations; iter++ {
		cfg := topology.DefaultTopologyConfig(topology.TopologyGraphMesh)
		topo, err := topology.NewTopology(topology.TopologyGraphMesh, cfg)
		require.NoError(t, err)

		agents := []*topology.Agent{
			topology.CreateAgentFromSpec(
				fmt.Sprintf("ml-%d-mod", iter), topology.RoleModerator,
				"mock", "m", 8.0, "reasoning"),
			topology.CreateAgentFromSpec(
				fmt.Sprintf("ml-%d-prop", iter), topology.RoleProposer,
				"mock", "m", 7.5, "code"),
			topology.CreateAgentFromSpec(
				fmt.Sprintf("ml-%d-crit", iter), topology.RoleCritic,
				"mock", "m", 7.0, "reasoning"),
		}

		err = topo.Initialize(context.Background(), agents)
		require.NoError(t, err)

		invoker := &stressMockInvoker{}
		protoCfg := protocol.DefaultDebateConfig()
		protoCfg.Topic = fmt.Sprintf("Memory leak test iteration %d", iter)
		protoCfg.MaxRounds = 1
		protoCfg.Timeout = 10 * time.Second

		proto := protocol.NewProtocol(protoCfg, topo, invoker)
		result, err := proto.Execute(context.Background())
		if err == nil && result != nil && result.Success {
			totalSuccesses++
		}

		topo.Close()
	}

	var memAfter runtime.MemStats
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	runtime.ReadMemStats(&memAfter)

	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	// Use signed arithmetic to handle GC reclaiming memory (after < before)
	heapGrowthMB := float64(int64(memAfter.HeapInuse)-int64(memBefore.HeapInuse)) / 1024 / 1024

	assert.Equal(t, int64(iterations), totalSuccesses,
		"all sequential debate sessions should succeed")
	assert.Less(t, heapGrowthMB, 100.0,
		"heap should not grow unboundedly after repeated debate sessions")
	assert.Less(t, leaked, 20,
		"goroutines should not accumulate across debate sessions")
	t.Logf("Debate memory leak check: iterations=%d, successes=%d, "+
		"heap_growth=%.2fMB, goroutine_leak=%d",
		iterations, totalSuccesses, heapGrowthMB, leaked)
}
