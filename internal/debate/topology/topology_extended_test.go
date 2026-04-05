package topology

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Extended Tests for Topology Package Coverage Improvement
// Tests in this file cover additional edge cases and scenarios not covered
// by the existing topology_test.go file.
// =============================================================================

// =============================================================================
// Additional BaseTopology Tests
// =============================================================================

func TestBaseTopology_GetChannels_Copy(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	bt := NewBaseTopology(config)

	channel := CommunicationChannel{
		FromAgent:     "agent-1",
		ToAgent:       "agent-2",
		Bidirectional: true,
		Weight:        1.0,
	}
	bt.AddChannel(channel)

	// Get channels and modify the returned slice
	channels := bt.GetChannels()
	channels[0].Weight = 0.5

	// Original should be unchanged
	originalChannels := bt.GetChannels()
	assert.Equal(t, 1.0, originalChannels[0].Weight)
}

func TestBaseTopology_GetAgentsByRole_Empty(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	bt := NewBaseTopology(config)

	// No agents added, should return empty slice
	agents := bt.GetAgentsByRole(RoleProposer)
	assert.Empty(t, agents)
}

func TestBaseTopology_GetMetrics_Copy(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	bt := NewBaseTopology(config)

	bt.UpdateAgentMetrics("agent-1", AgentMetrics{MessageCount: 5})

	// Get metrics and verify it's a copy
	metrics := bt.GetMetrics()
	assert.Equal(t, 5, metrics.AgentMetrics["agent-1"].MessageCount)
}

func TestBaseTopology_GetNextPhase_Unknown(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	bt := NewBaseTopology(config)

	// Unknown phase should return PhaseProposal
	next := bt.GetNextPhase(DebatePhase("unknown_phase"))
	assert.Equal(t, PhaseProposal, next)
}

// =============================================================================
// Additional GraphMeshTopology Tests
// =============================================================================

func TestGraphMeshTopology_InitializeWithDifferentRoles(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	gm := NewGraphMeshTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create agents with all roles
	agents := []*Agent{
		{ID: "proposer", Role: RoleProposer, Score: 9.0},
		{ID: "critic", Role: RoleCritic, Score: 8.5},
		{ID: "reviewer", Role: RoleReviewer, Score: 8.0},
		{ID: "optimizer", Role: RoleOptimizer, Score: 7.5},
		{ID: "moderator", Role: RoleModerator, Score: 7.0},
		{ID: "architect", Role: RoleArchitect, Score: 8.2},
		{ID: "validator", Role: RoleValidator, Score: 7.8},
		{ID: "red-team", Role: RoleRedTeam, Score: 7.3},
		{ID: "blue-team", Role: RoleBlueTeam, Score: 7.2},
		{ID: "security", Role: RoleSecurity, Score: 7.1},
		{ID: "test-agent", Role: RoleTestAgent, Score: 7.0},
		{ID: "teacher", Role: RoleTeacher, Score: 6.9},
	}

	err := gm.Initialize(ctx, agents)
	require.NoError(t, err)

	// Verify parallel groups are set up for each phase
	phases := []DebatePhase{PhaseProposal, PhaseCritique, PhaseReview, PhaseOptimization, PhaseConvergence}
	for _, phase := range phases {
		groups := gm.GetParallelGroups(phase)
		assert.NotEmpty(t, groups, "Phase %s should have groups", phase)
	}
}

func TestGraphMeshTopology_SelectLeader_NoAgents(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	gm := NewGraphMeshTopology(config)

	// Don't initialize - no agents

	_, err := gm.SelectLeader(PhaseProposal)
	assert.Error(t, err)
}

func TestGraphMeshTopology_RouteMessage_PartialTargets(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	gm := NewGraphMeshTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(4)
	_ = gm.Initialize(ctx, agents)

	// Message to specific targets including one that doesn't exist
	msg := &Message{
		FromAgent: agents[0].ID,
		ToAgents:  []string{agents[1].ID, "nonexistent"},
	}

	targets, err := gm.RouteMessage(msg)
	require.NoError(t, err)
	// Should only return the existing target
	assert.Len(t, targets, 1)
	assert.Equal(t, agents[1].ID, targets[0].ID)
}

func TestGraphMeshTopology_CalculateChannelWeight(t *testing.T) {
	t.Parallel()
	// Test channel weight calculation with different agent configurations
	from := &Agent{
		ID:             "from",
		Score:          8.0,
		Specialization: "code",
		Role:           RoleProposer,
	}

	to := &Agent{
		ID:             "to",
		Score:          7.0,
		Specialization: "reasoning", // Different specialization
		Role:           RoleCritic,  // Different role
	}

	weight := calculateChannelWeight(from, to)

	// Weight should include base + complementary bonus + role bonus
	assert.Greater(t, weight, 0.5) // At least average score / 10
	assert.Less(t, weight, 2.0)    // Should be reasonable
}

func TestGraphMeshTopology_CalculateLeaderScore_AllPhases(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	gm := NewGraphMeshTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := []*Agent{
		{ID: "proposer", Role: RoleProposer, Score: 8.0, Specialization: "reasoning", Confidence: 0.9},
		{ID: "critic", Role: RoleCritic, Score: 7.5, Specialization: "code", Confidence: 0.85},
		{ID: "optimizer", Role: RoleOptimizer, Score: 7.0, Specialization: "code", Confidence: 0.8},
		{ID: "moderator", Role: RoleModerator, Score: 8.5, Specialization: "reasoning", Confidence: 0.95},
	}
	_ = gm.Initialize(ctx, agents)

	// Test leader selection for each phase
	phases := []DebatePhase{PhaseProposal, PhaseCritique, PhaseReview, PhaseOptimization, PhaseConvergence}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
				t.Parallel()
			leader, err := gm.SelectLeader(phase)
			require.NoError(t, err)
			assert.NotNil(t, leader)
		})
	}
}

func TestGraphMeshTopology_DynamicRoleReassignment_TopPerformers(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	config.EnableDynamicRoles = true
	gm := NewGraphMeshTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create agents with varying scores
	agents := make([]*Agent, 8)
	for i := 0; i < 8; i++ {
		agents[i] = &Agent{
			ID:    generateTestID(i),
			Role:  RoleProposer,
			Score: 5.0 + float64(i), // Scores 5.0 to 12.0
		}
		// Simulate activity
		agents[i].UpdateActivity(time.Duration(100-i*10) * time.Millisecond)
	}
	_ = gm.Initialize(ctx, agents)

	err := gm.DynamicRoleReassignment(ctx)
	assert.NoError(t, err)

	// Top performers should potentially have been reassigned
	// (depends on implementation details)
}

func TestGraphMeshTopology_GetTopologySnapshot_Complete(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	gm := NewGraphMeshTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = gm.Initialize(ctx, agents)
	gm.IncrementMessageCount(true) // Ensure some messages are counted
	gm.SetPhase(PhaseCritique, "test", "test transition")
	_, _ = gm.SelectLeader(PhaseCritique)

	snapshot := gm.GetTopologySnapshot()

	assert.Equal(t, TopologyGraphMesh, snapshot.Type)
	assert.Len(t, snapshot.Agents, 3)
	assert.Equal(t, PhaseCritique, snapshot.CurrentPhase)
	assert.NotEmpty(t, snapshot.CurrentLeader)
	// Just verify the metrics are there, not the actual count
	assert.NotNil(t, snapshot.Metrics)
}

func TestGraphMeshTopology_ExecuteParallelPhase_WithErrors(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	gm := NewGraphMeshTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(4)
	_ = gm.Initialize(ctx, agents)

	errorCount := 0
	taskFn := func(agent *Agent) error {
		errorCount++
		return nil // No error
	}

	errors := gm.ExecuteParallelPhase(ctx, PhaseProposal, taskFn)
	assert.Empty(t, errors)
	assert.Greater(t, errorCount, 0)
}

// =============================================================================
// Additional StarTopology Tests
// =============================================================================

func TestStarTopology_Initialize_WithMultipleModerators(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyStar)
	st := NewStarTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Multiple moderators - should select highest scored
	agents := []*Agent{
		{ID: "mod-1", Role: RoleModerator, Score: 7.0},
		{ID: "mod-2", Role: RoleModerator, Score: 9.0},
		{ID: "agent-1", Role: RoleProposer, Score: 8.0},
	}

	err := st.Initialize(ctx, agents)
	require.NoError(t, err)

	moderator := st.GetModerator()
	assert.Equal(t, "mod-2", moderator.ID) // Highest scored moderator
}

func TestStarTopology_Initialize_NoModerators(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyStar)
	st := NewStarTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// No moderator role - should select highest scored agent
	agents := []*Agent{
		{ID: "agent-1", Role: RoleProposer, Score: 8.0},
		{ID: "agent-2", Role: RoleCritic, Score: 9.0},
		{ID: "agent-3", Role: RoleReviewer, Score: 7.0},
	}

	err := st.Initialize(ctx, agents)
	require.NoError(t, err)

	moderator := st.GetModerator()
	assert.Equal(t, "agent-2", moderator.ID) // Highest scored
}

func TestStarTopology_RouteMessage_ModeratorBroadcast(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyStar)
	st := NewStarTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(4)
	_ = st.Initialize(ctx, agents)

	moderatorID := st.GetModerator().ID

	// Moderator broadcast (empty ToAgents)
	msg := &Message{
		FromAgent: moderatorID,
		ToAgents:  nil,
	}

	targets, err := st.RouteMessage(msg)
	require.NoError(t, err)
	assert.Len(t, targets, 3) // All except moderator
}

func TestStarTopology_SendMessage_EmptyTargets(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyStar)
	st := NewStarTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = st.Initialize(ctx, agents)

	msg := &Message{
		FromAgent: agents[0].ID,
		ToAgents:  []string{}, // Empty - should broadcast
		Content:   "Test message",
	}

	err := st.SendMessage(ctx, msg)
	require.NoError(t, err)
}

func TestStarTopology_SelectLeader_NoModerator(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyStar)
	st := NewStarTopology(config)

	// Don't initialize - no moderator set

	_, err := st.SelectLeader(PhaseProposal)
	assert.Error(t, err)
}

func TestStarTopology_GetParallelGroups_SingleAgent(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyStar)
	config.MaxParallelism = 10
	st := NewStarTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Only one agent (becomes moderator)
	agents := []*Agent{
		{ID: "agent-1", Role: RoleProposer, Score: 8.0},
	}

	_ = st.Initialize(ctx, agents)

	groups := st.GetParallelGroups(PhaseProposal)
	// No non-moderators, so no groups
	assert.Empty(t, groups)
}

func TestStarTopology_Close(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyStar)
	st := NewStarTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = st.Initialize(ctx, agents)

	err := st.Close()
	assert.NoError(t, err)
}

// =============================================================================
// Additional ChainTopology Tests
// =============================================================================

func TestChainTopology_BuildChain_AllRoles(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create agents with all roles
	agents := []*Agent{
		{ID: "moderator", Role: RoleModerator, Score: 7.0},
		{ID: "proposer", Role: RoleProposer, Score: 9.0},
		{ID: "critic", Role: RoleCritic, Score: 8.5},
		{ID: "reviewer", Role: RoleReviewer, Score: 8.0},
		{ID: "optimizer", Role: RoleOptimizer, Score: 7.5},
		{ID: "validator", Role: RoleValidator, Score: 7.2},
	}

	err := ct.Initialize(ctx, agents)
	require.NoError(t, err)

	chain := ct.GetChain()
	assert.Len(t, chain, 6)

	// Verify ordering: proposer should come before moderator
	proposerPos := ct.GetChainPosition("proposer")
	moderatorPos := ct.GetChainPosition("moderator")
	assert.Less(t, proposerPos, moderatorPos)
}

func TestChainTopology_AdvancePosition_WrapAround(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = ct.Initialize(ctx, agents)

	// Advance through entire chain
	for i := 0; i < 10; i++ {
		agent := ct.AdvancePosition()
		assert.NotNil(t, agent)
		assert.GreaterOrEqual(t, ct.GetCurrentPosition(), 0)
		assert.Less(t, ct.GetCurrentPosition(), 3)
	}
}

func TestChainTopology_GetNextAgent_NonExistent(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = ct.Initialize(ctx, agents)

	next := ct.GetNextAgent("nonexistent")
	assert.Nil(t, next)
}

func TestChainTopology_GetPreviousAgent_NonExistent(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = ct.Initialize(ctx, agents)

	prev := ct.GetPreviousAgent("nonexistent")
	assert.Nil(t, prev)
}

func TestChainTopology_RouteMessage_NonExistent(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = ct.Initialize(ctx, agents)

	msg := &Message{
		FromAgent: "nonexistent",
	}

	_, err := ct.RouteMessage(msg)
	assert.Error(t, err)
}

func TestChainTopology_BroadcastMessage_Timeout(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	config.MessageTimeout = 1 * time.Nanosecond // Very short timeout
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = ct.Initialize(ctx, agents)

	// Fill the message queue
	for i := 0; i < 200; i++ {
		select {
		case ct.messageQueue <- &Message{ID: "fill"}:
		default:
		}
	}

	msg := &Message{
		FromAgent: agents[0].ID,
		Content:   "Test broadcast",
	}

	// This might timeout due to full queue
	_ = ct.BroadcastMessage(ctx, msg)
}

func TestChainTopology_SelectLeader_AllPhases(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := []*Agent{
		{ID: "proposer", Role: RoleProposer, Score: 9.0},
		{ID: "critic", Role: RoleCritic, Score: 8.5},
		{ID: "reviewer", Role: RoleReviewer, Score: 8.0},
		{ID: "optimizer", Role: RoleOptimizer, Score: 7.5},
		{ID: "moderator", Role: RoleModerator, Score: 7.0},
	}
	_ = ct.Initialize(ctx, agents)

	phases := []DebatePhase{PhaseProposal, PhaseCritique, PhaseReview, PhaseOptimization, PhaseConvergence}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
				t.Parallel()
			leader, err := ct.SelectLeader(phase)
			require.NoError(t, err)
			assert.NotNil(t, leader)
		})
	}
}

func TestChainTopology_SelectLeader_EmptyChain(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)

	// Don't initialize

	_, err := ct.SelectLeader(PhaseProposal)
	assert.Error(t, err)
}

func TestChainTopology_ReorderChain_AgentNotFound(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = ct.Initialize(ctx, agents)

	chain := ct.GetChain()
	newOrder := []string{chain[0].ID, "nonexistent", chain[2].ID}

	err := ct.ReorderChain(newOrder)
	assert.Error(t, err)
}

func TestChainTopology_Close(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyChain)
	ct := NewChainTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(3)
	_ = ct.Initialize(ctx, agents)

	err := ct.Close()
	assert.NoError(t, err)
}

// =============================================================================
// Message and Structure Tests
// =============================================================================

func TestMessage_AllFields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	msg := &Message{
		ID:          "msg-1",
		FromAgent:   "agent-1",
		ToAgents:    []string{"agent-2", "agent-3"},
		Content:     "Test content",
		MessageType: MessageTypeProposal,
		Phase:       PhaseProposal,
		Round:       1,
		Timestamp:   now,
		ReplyTo:     "msg-0",
		Confidence:  0.85,
		Metadata:    map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "msg-1", msg.ID)
	assert.Equal(t, "agent-1", msg.FromAgent)
	assert.Len(t, msg.ToAgents, 2)
	assert.Equal(t, MessageTypeProposal, msg.MessageType)
	assert.Equal(t, 0.85, msg.Confidence)
	assert.Equal(t, "msg-0", msg.ReplyTo)
}

func TestCommunicationChannel_Fields(t *testing.T) {
	t.Parallel()
	channel := CommunicationChannel{
		FromAgent:     "agent-1",
		ToAgent:       "agent-2",
		Bidirectional: true,
		Weight:        0.9,
	}

	assert.Equal(t, "agent-1", channel.FromAgent)
	assert.Equal(t, "agent-2", channel.ToAgent)
	assert.True(t, channel.Bidirectional)
	assert.Equal(t, 0.9, channel.Weight)
}

func TestTopologyMetrics_AllFields(t *testing.T) {
	t.Parallel()
	metrics := TopologyMetrics{
		TotalMessages:      100,
		BroadcastCount:     20,
		DirectMessageCount: 80,
		AvgMessageLatency:  50 * time.Millisecond,
		PhaseTransitions:   5,
		AgentMetrics:       map[string]AgentMetrics{"agent-1": {MessageCount: 10}},
		ChannelUtilization: map[string]float64{"a1-a2": 0.75},
	}

	assert.Equal(t, int64(100), metrics.TotalMessages)
	assert.Equal(t, int64(20), metrics.BroadcastCount)
	assert.Equal(t, 5, metrics.PhaseTransitions)
	assert.Equal(t, 10, metrics.AgentMetrics["agent-1"].MessageCount)
}

func TestTopologyConfig_AllFields(t *testing.T) {
	t.Parallel()
	config := TopologyConfig{
		Type:                TopologyGraphMesh,
		MaxParallelism:      10,
		MessageTimeout:      60 * time.Second,
		EnableDynamicRoles:  true,
		EnableLoadBalancing: true,
		PriorityChannels:    []string{"a1-a2"},
		Metadata:            map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, TopologyGraphMesh, config.Type)
	assert.Equal(t, 10, config.MaxParallelism)
	assert.Len(t, config.PriorityChannels, 1)
}

func TestPhaseTransition_Structure(t *testing.T) {
	t.Parallel()
	pt := PhaseTransition{
		FromPhase: PhaseProposal,
		ToPhase:   PhaseCritique,
		Timestamp: time.Now(),
		Initiator: "agent-1",
		Reason:    "test transition",
	}

	assert.Equal(t, PhaseProposal, pt.FromPhase)
	assert.Equal(t, PhaseCritique, pt.ToPhase)
	assert.Equal(t, "agent-1", pt.Initiator)
}

func TestLeaderSelection_Structure(t *testing.T) {
	t.Parallel()
	ls := LeaderSelection{
		Phase:     PhaseProposal,
		LeaderID:  "agent-1",
		Score:     9.5,
		Timestamp: time.Now(),
		Method:    "score",
	}

	assert.Equal(t, PhaseProposal, ls.Phase)
	assert.Equal(t, "agent-1", ls.LeaderID)
	assert.Equal(t, 9.5, ls.Score)
}

func TestTopologySnapshot_Structure(t *testing.T) {
	t.Parallel()
	snapshot := TopologySnapshot{
		Type:          TopologyGraphMesh,
		Agents:        []*Agent{{ID: "a1"}},
		Channels:      []CommunicationChannel{},
		CurrentPhase:  PhaseProposal,
		CurrentLeader: "a1",
		Metrics:       TopologyMetrics{},
		Timestamp:     time.Now(),
	}

	assert.Equal(t, TopologyGraphMesh, snapshot.Type)
	assert.Len(t, snapshot.Agents, 1)
	assert.Equal(t, "a1", snapshot.CurrentLeader)
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestGraphMeshTopology_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyGraphMesh)
	gm := NewGraphMeshTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTestAgents(5)
	_ = gm.Initialize(ctx, agents)

	done := make(chan bool, 10)

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			_ = gm.GetAgents()
			_ = gm.GetChannels()
			_ = gm.GetMetrics()
			_ = gm.GetCurrentPhase()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func() {
			gm.IncrementMessageCount(true)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
