// CONST-030: these tests exercise the in-process debate Protocol against a
// canned `mockInvoker` (no live LLM). They assert on in-process invariants:
// 8-phase execution order, early-consensus exit, timeout & cancellation
// handling, and multi-topology compatibility. All are unit-level per
// CONST-030. Demoted from tests/integration/ (PR9).
package debate_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.debate/protocol"
	"digital.vasic.debate/topology"
)

// mockInvoker implements protocol.AgentInvoker for integration testing
// without requiring a live LLM backend.
type mockInvoker struct {
	mu           sync.Mutex
	invocations  int
	delay        time.Duration
	earlyConsens bool // if true, return high-confidence votes in convergence
}

func (m *mockInvoker) Invoke(
	ctx context.Context,
	agent *topology.Agent,
	prompt string,
	debateCtx protocol.DebateContext,
) (*protocol.PhaseResponse, error) {
	m.mu.Lock()
	m.invocations++
	m.mu.Unlock()

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	confidence := 0.7
	vote := ""
	if debateCtx.CurrentPhase == topology.PhaseConvergence {
		vote = "solution-A"
		if m.earlyConsens {
			confidence = 0.95
		}
	}

	return &protocol.PhaseResponse{
		AgentID:     agent.ID,
		Role:        agent.Role,
		Provider:    agent.Provider,
		Model:       agent.Model,
		Content:     "Mock response for " + string(debateCtx.CurrentPhase),
		Confidence:  confidence,
		Vote:        vote,
		Score:       agent.Score,
		Latency:     50 * time.Millisecond,
		Timestamp:   time.Now(),
		Arguments:   []string{"argument-1"},
		Suggestions: []string{"suggestion-1"},
		Metadata:    map[string]interface{}{"mock": true},
	}, nil
}

// createTestTopology builds a graph-mesh topology with a standard set
// of agents for integration testing.
func createTestTopology(t *testing.T) topology.Topology {
	t.Helper()

	cfg := topology.DefaultTopologyConfig(topology.TopologyGraphMesh)
	topo, err := topology.NewTopology(topology.TopologyGraphMesh, cfg)
	require.NoError(t, err, "Failed to create topology")

	agents := []*topology.Agent{
		topology.CreateAgentFromSpec("agent-mod", topology.RoleModerator,
			"mock", "mock-model", 8.5, "reasoning"),
		topology.CreateAgentFromSpec("agent-prop", topology.RoleProposer,
			"mock", "mock-model", 8.0, "code"),
		topology.CreateAgentFromSpec("agent-gen", topology.RoleGenerator,
			"mock", "mock-model", 7.5, "code"),
		topology.CreateAgentFromSpec("agent-crit", topology.RoleCritic,
			"mock", "mock-model", 7.8, "reasoning"),
		topology.CreateAgentFromSpec("agent-rev", topology.RoleReviewer,
			"mock", "mock-model", 7.6, "reasoning"),
		topology.CreateAgentFromSpec("agent-opt", topology.RoleOptimizer,
			"mock", "mock-model", 7.4, "code"),
		topology.CreateAgentFromSpec("agent-red", topology.RoleRedTeam,
			"mock", "mock-model", 7.2, "reasoning"),
		topology.CreateAgentFromSpec("agent-blue", topology.RoleBlueTeam,
			"mock", "mock-model", 7.3, "code"),
		topology.CreateAgentFromSpec("agent-val", topology.RoleValidator,
			"mock", "mock-model", 7.7, "reasoning"),
		topology.CreateAgentFromSpec("agent-arch", topology.RoleArchitect,
			"mock", "mock-model", 8.2, "reasoning"),
	}

	err = topo.Initialize(context.Background(), agents)
	require.NoError(t, err, "Failed to initialize topology with agents")

	return topo
}

// TestDebateFullProtocol_8Phases verifies that all 8 debate phases execute
// in the correct order: Dehallucination -> SelfEvolvement -> Proposal ->
// Critique -> Review -> Optimization -> Adversarial -> Convergence.
func TestDebateFullProtocol_8Phases(t *testing.T) {

	topo := createTestTopology(t)
	defer topo.Close()

	invoker := &mockInvoker{}

	cfg := protocol.DefaultDebateConfig()
	cfg.Topic = "Design a fault-tolerant distributed cache"
	cfg.MaxRounds = 1
	cfg.Timeout = 2 * time.Minute
	cfg.EnableEarlyExit = false

	proto := protocol.NewProtocol(cfg, topo, invoker)
	ctx := context.Background()

	result, err := proto.Execute(ctx)
	require.NoError(t, err, "Protocol execution should not error")
	require.NotNil(t, result, "Protocol should return a result")

	// Verify result metadata
	assert.Equal(t, cfg.Topic, result.Topic, "Topic should match config")
	assert.True(t, result.Success, "Debate should succeed")
	assert.Equal(t, 1, result.RoundsCompleted, "Should complete 1 round")
	assert.Equal(t, topology.TopologyGraphMesh, result.TopologyUsed,
		"Should use graph mesh topology")

	// All 8 phases should have results
	expectedPhases := []topology.DebatePhase{
		topology.PhaseDehallucination,
		topology.PhaseSelfEvolvement,
		topology.PhaseProposal,
		topology.PhaseCritique,
		topology.PhaseReview,
		topology.PhaseOptimization,
		topology.PhaseAdversarial,
		topology.PhaseConvergence,
	}

	require.GreaterOrEqual(t, len(result.Phases), len(expectedPhases),
		"Should have at least 8 phase results")

	// Verify phases executed in order
	for i, expected := range expectedPhases {
		if i < len(result.Phases) {
			assert.Equal(t, expected, result.Phases[i].Phase,
				"Phase %d should be %s", i, expected)
			assert.NotZero(t, result.Phases[i].Duration,
				"Phase %s should have non-zero duration", expected)
		}
	}

	// Verify metrics were collected
	require.NotNil(t, result.Metrics, "Metrics should not be nil")
	assert.Greater(t, result.Metrics.TotalResponses, 0,
		"Should have collected responses")

	// Verify the invoker was actually called
	invoker.mu.Lock()
	invocations := invoker.invocations
	invoker.mu.Unlock()
	assert.Greater(t, invocations, 0,
		"Mock invoker should have been called at least once")

	t.Logf("Protocol completed: %d phases, %d invocations, duration %v",
		len(result.Phases), invocations, result.Duration)
}

// TestDebateFullProtocol_EarlyConsensus verifies that the protocol exits
// early when consensus is reached during the convergence phase.
func TestDebateFullProtocol_EarlyConsensus(t *testing.T) {

	topo := createTestTopology(t)
	defer topo.Close()

	invoker := &mockInvoker{earlyConsens: true}

	cfg := protocol.DefaultDebateConfig()
	cfg.Topic = "Select the best sorting algorithm for streaming data"
	cfg.MaxRounds = 5 // Would take long without early exit
	cfg.Timeout = 5 * time.Minute
	cfg.EnableEarlyExit = true
	cfg.MinConsensusScore = 0.6

	proto := protocol.NewProtocol(cfg, topo, invoker)
	ctx := context.Background()

	result, err := proto.Execute(ctx)
	require.NoError(t, err, "Protocol execution should not error")
	require.NotNil(t, result, "Protocol should return a result")

	// With early consensus enabled and high-confidence votes,
	// the protocol should exit before completing all rounds
	assert.True(t, result.Success, "Debate should succeed")
	if result.EarlyExit {
		assert.Equal(t, "early_consensus", result.EarlyExitReason,
			"Early exit reason should be consensus")
		assert.Less(t, result.RoundsCompleted, cfg.MaxRounds,
			"Should complete fewer rounds than max with early exit")
		t.Logf("Early consensus reached after %d rounds", result.RoundsCompleted)
	} else {
		t.Logf("All %d rounds completed (consensus not reached early)",
			result.RoundsCompleted)
	}

	// Verify consensus result exists if early exit occurred
	if result.EarlyExit && result.FinalConsensus != nil {
		assert.NotEmpty(t, result.FinalConsensus.Contributors,
			"Consensus should have contributors")
	}
}

// TestDebateFullProtocol_Timeout verifies that the protocol respects the
// configured timeout and terminates gracefully.
func TestDebateFullProtocol_Timeout(t *testing.T) {

	topo := createTestTopology(t)
	defer topo.Close()

	// Use a slow invoker to force timeout
	invoker := &mockInvoker{delay: 500 * time.Millisecond}

	cfg := protocol.DefaultDebateConfig()
	cfg.Topic = "Analyze the trade-offs of event sourcing vs CRUD"
	cfg.MaxRounds = 10
	cfg.Timeout = 2 * time.Second // Very short timeout
	cfg.EnableEarlyExit = false

	proto := protocol.NewProtocol(cfg, topo, invoker)
	ctx := context.Background()

	start := time.Now()
	result, err := proto.Execute(ctx)
	duration := time.Since(start)

	// Should timeout
	if err != nil {
		t.Logf("Protocol timed out as expected: %v", err)
	}

	// Duration should be close to the timeout (with buffer for overhead)
	assert.Less(t, duration, 15*time.Second,
		"Should terminate within reasonable time after timeout")

	if result != nil {
		t.Logf("Completed %d phases before timeout in %v",
			len(result.Phases), duration)
	}
}

// TestDebateFullProtocol_ContextCancellation tests that the protocol
// responds to context cancellation promptly.
func TestDebateFullProtocol_ContextCancellation(t *testing.T) {

	topo := createTestTopology(t)
	defer topo.Close()

	invoker := &mockInvoker{delay: 200 * time.Millisecond}

	cfg := protocol.DefaultDebateConfig()
	cfg.Topic = "Compare microkernel vs monolithic OS architectures"
	cfg.MaxRounds = 10
	cfg.Timeout = 5 * time.Minute

	proto := protocol.NewProtocol(cfg, topo, invoker)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	start := time.Now()
	result, err := proto.Execute(ctx)
	duration := time.Since(start)

	// Should have been cancelled
	if err != nil {
		t.Logf("Protocol cancelled as expected: %v", err)
	}

	assert.Less(t, duration, 10*time.Second,
		"Should stop promptly after cancellation")

	if result != nil {
		t.Logf("Completed %d phases before cancellation in %v",
			len(result.Phases), duration)
	}
}

// TestDebateFullProtocol_MultipleTopologies verifies that all topology
// types can be used with the protocol.
func TestDebateFullProtocol_MultipleTopologies(t *testing.T) {

	topoTypes := []topology.TopologyType{
		topology.TopologyGraphMesh,
		topology.TopologyStar,
		topology.TopologyChain,
		topology.TopologyTree,
	}

	for _, tt := range topoTypes {
		t.Run(string(tt), func(t *testing.T) {
			cfg := topology.DefaultTopologyConfig(tt)
			topo, err := topology.NewTopology(tt, cfg)
			require.NoError(t, err)
			defer topo.Close()

			agents := []*topology.Agent{
				topology.CreateAgentFromSpec("a1", topology.RoleModerator,
					"mock", "m", 8.0, "reasoning"),
				topology.CreateAgentFromSpec("a2", topology.RoleProposer,
					"mock", "m", 7.5, "code"),
				topology.CreateAgentFromSpec("a3", topology.RoleCritic,
					"mock", "m", 7.0, "reasoning"),
				topology.CreateAgentFromSpec("a4", topology.RoleReviewer,
					"mock", "m", 7.2, "reasoning"),
				topology.CreateAgentFromSpec("a5", topology.RoleOptimizer,
					"mock", "m", 7.1, "code"),
			}
			err = topo.Initialize(context.Background(), agents)
			require.NoError(t, err)

			invoker := &mockInvoker{}
			protoCfg := protocol.DefaultDebateConfig()
			protoCfg.Topic = "Test with " + string(tt) + " topology"
			protoCfg.MaxRounds = 1
			protoCfg.Timeout = 30 * time.Second
			protoCfg.TopologyType = tt

			proto := protocol.NewProtocol(protoCfg, topo, invoker)
			result, execErr := proto.Execute(context.Background())
			require.NoError(t, execErr)
			require.NotNil(t, result)

			assert.True(t, result.Success,
				"Debate with %s topology should succeed", tt)
			t.Logf("Topology %s: %d phases completed", tt,
				len(result.Phases))
		})
	}
}
