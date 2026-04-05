package topology

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==========================================================================
// Helper: build agents with specific roles for tree tests
// ==========================================================================

func createTreeTestAgents() []*Agent {
	return []*Agent{
		{
			ID: "architect-1", Role: RoleArchitect,
			Provider: "p1", Model: "m1", Score: 9.0,
			Capabilities: []string{"design"},
			Metadata:     map[string]interface{}{},
		},
		{
			ID: "security-1", Role: RoleSecurity,
			Provider: "p2", Model: "m2", Score: 8.5,
			Capabilities: []string{"security"},
			Metadata:     map[string]interface{}{},
		},
		{
			ID: "perf-1", Role: RolePerformanceAnalyzer,
			Provider: "p3", Model: "m3", Score: 8.0,
			Capabilities: []string{"performance"},
			Metadata:     map[string]interface{}{},
		},
		{
			ID: "proposer-1", Role: RoleProposer,
			Provider: "p4", Model: "m4", Score: 7.5,
			Capabilities: []string{"code"},
			Metadata:     map[string]interface{}{},
		},
		{
			ID: "critic-1", Role: RoleCritic,
			Provider: "p5", Model: "m5", Score: 7.0,
			Capabilities: []string{"review"},
			Metadata:     map[string]interface{}{},
		},
	}
}

// ==========================================================================
// NewTreeTopology
// ==========================================================================

func TestNewTreeTopology(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyTree)
	tt := NewTreeTopology(config)

	require.NotNil(t, tt)
	assert.NotNil(t, tt.BaseTopology)
	assert.NotNil(t, tt.nodes)
	assert.Nil(t, tt.root)
	assert.Equal(t, 3, tt.maxDepth)
	assert.Equal(t, TopologyTree, tt.BaseTopology.config.Type)
}

// ==========================================================================
// Initialize
// ==========================================================================

func TestTreeTopology_Initialize(t *testing.T) {
	t.Parallel()
	t.Run("successful initialization", func(t *testing.T) {
		config := DefaultTopologyConfig(TopologyTree)
		tt := NewTreeTopology(config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		agents := createTreeTestAgents()
		err := tt.Initialize(ctx, agents)

		require.NoError(t, err)

		// Root should be the architect (preferred)
		root := tt.GetRoot()
		require.NotNil(t, root)
		assert.Equal(t, "architect-1", root.AgentID)
		assert.Equal(t, 0, root.Level)

		// All agents should be registered
		allAgents := tt.GetAgents()
		assert.Len(t, allAgents, 5)

		// Channels should exist (parent-child pairs)
		channels := tt.GetChannels()
		assert.NotEmpty(t, channels)
	})

	t.Run("empty agents returns error", func(t *testing.T) {
			t.Parallel()
		config := DefaultTopologyConfig(TopologyTree)
		tt := NewTreeTopology(config)
		ctx := context.Background()

		err := tt.Initialize(ctx, []*Agent{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one agent")
	})

	t.Run("single agent becomes root", func(t *testing.T) {
			t.Parallel()
		config := DefaultTopologyConfig(TopologyTree)
		tt := NewTreeTopology(config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		agent := &Agent{
			ID: "solo", Role: RoleProposer,
			Provider: "p1", Model: "m1", Score: 8.0,
			Capabilities: []string{"code"},
			Metadata:     map[string]interface{}{},
		}

		err := tt.Initialize(ctx, []*Agent{agent})
		require.NoError(t, err)

		root := tt.GetRoot()
		require.NotNil(t, root)
		assert.Equal(t, "solo", root.AgentID)
		assert.Empty(t, root.Children)
	})
}

// ==========================================================================
// GetType
// ==========================================================================

func TestTreeTopology_GetType(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyTree)
	tt := NewTreeTopology(config)

	assert.Equal(t, TopologyTree, tt.GetType())
}

// ==========================================================================
// CanCommunicate
// ==========================================================================

func TestTreeTopology_CanCommunicate(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyTree)
	tt := NewTreeTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTreeTestAgents()
	err := tt.Initialize(ctx, agents)
	require.NoError(t, err)

	root := tt.GetRoot()
	require.NotNil(t, root)

	t.Run("parent to child is allowed", func(t *testing.T) {
			t.Parallel()
		// Root (architect-1) should be able to communicate with
		// its direct children
		if len(root.Children) > 0 {
			childID := root.Children[0].AgentID
			assert.True(t, tt.CanCommunicate(root.AgentID, childID))
		}
	})

	t.Run("child to parent is allowed", func(t *testing.T) {
			t.Parallel()
		if len(root.Children) > 0 {
			childID := root.Children[0].AgentID
			assert.True(t, tt.CanCommunicate(childID, root.AgentID))
		}
	})

	t.Run("non-adjacent nodes cannot communicate directly", func(t *testing.T) {
			t.Parallel()
		// Find two leaf nodes under different subtrees
		if len(root.Children) >= 2 {
			lead1 := root.Children[0]
			lead2 := root.Children[1]
			// Leads are siblings, not parent-child => no direct comm
			assert.False(t, tt.CanCommunicate(
				lead1.AgentID, lead2.AgentID,
			))
		}
	})

	t.Run("same agent cannot communicate with itself", func(t *testing.T) {
			t.Parallel()
		assert.False(t, tt.CanCommunicate(
			root.AgentID, root.AgentID,
		))
	})

	t.Run("non-existent agent returns false", func(t *testing.T) {
			t.Parallel()
		assert.False(t, tt.CanCommunicate(
			root.AgentID, "non-existent",
		))
		assert.False(t, tt.CanCommunicate(
			"non-existent", root.AgentID,
		))
	})
}

// ==========================================================================
// GetCommunicationTargets
// ==========================================================================

func TestTreeTopology_GetCommunicationTargets(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyTree)
	tt := NewTreeTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTreeTestAgents()
	err := tt.Initialize(ctx, agents)
	require.NoError(t, err)

	root := tt.GetRoot()
	require.NotNil(t, root)

	t.Run("root targets are its children", func(t *testing.T) {
			t.Parallel()
		targets := tt.GetCommunicationTargets(root.AgentID)
		// Root has no parent, so targets = children only
		assert.Len(t, targets, len(root.Children))
	})

	t.Run("child targets include parent", func(t *testing.T) {
			t.Parallel()
		if len(root.Children) > 0 {
			childID := root.Children[0].AgentID
			targets := tt.GetCommunicationTargets(childID)
			// Should include at least the parent (root)
			found := false
			for _, target := range targets {
				if target.ID == root.AgentID {
					found = true
					break
				}
			}
			assert.True(t, found,
				"child's communication targets should include parent")
		}
	})

	t.Run("non-existent agent returns empty", func(t *testing.T) {
			t.Parallel()
		targets := tt.GetCommunicationTargets("non-existent")
		assert.Empty(t, targets)
	})
}

// ==========================================================================
// RouteMessage
// ==========================================================================

func TestTreeTopology_RouteMessage(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyTree)
	tt := NewTreeTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTreeTestAgents()
	err := tt.Initialize(ctx, agents)
	require.NoError(t, err)

	root := tt.GetRoot()
	require.NotNil(t, root)

	t.Run("broadcast when no targets specified", func(t *testing.T) {
			t.Parallel()
		msg := &Message{
			FromAgent: root.AgentID,
			ToAgents:  nil,
		}

		targets, err := tt.RouteMessage(msg)
		require.NoError(t, err)
		// Should return all agents except sender
		assert.Len(t, targets, len(agents)-1)
	})

	t.Run("route from parent to child", func(t *testing.T) {
			t.Parallel()
		if len(root.Children) > 0 {
			childID := root.Children[0].AgentID
			msg := &Message{
				FromAgent: root.AgentID,
				ToAgents:  []string{childID},
			}

			targets, err := tt.RouteMessage(msg)
			require.NoError(t, err)
			assert.NotEmpty(t, targets)

			targetIDs := make([]string, len(targets))
			for i, tgt := range targets {
				targetIDs[i] = tgt.ID
			}
			assert.Contains(t, targetIDs, childID)
		}
	})

	t.Run("route from child to parent", func(t *testing.T) {
			t.Parallel()
		if len(root.Children) > 0 {
			childID := root.Children[0].AgentID
			msg := &Message{
				FromAgent: childID,
				ToAgents:  []string{root.AgentID},
			}

			targets, err := tt.RouteMessage(msg)
			require.NoError(t, err)
			assert.NotEmpty(t, targets)

			targetIDs := make([]string, len(targets))
			for i, tgt := range targets {
				targetIDs[i] = tgt.ID
			}
			assert.Contains(t, targetIDs, root.AgentID)
		}
	})

	t.Run("non-existent sender returns error", func(t *testing.T) {
			t.Parallel()
		msg := &Message{
			FromAgent: "ghost-agent",
			ToAgents:  []string{root.AgentID},
		}

		_, err := tt.RouteMessage(msg)
		assert.Error(t, err)
	})

	t.Run("sibling routing goes through parent", func(t *testing.T) {
			t.Parallel()
		if len(root.Children) >= 2 {
			sibling1 := root.Children[0].AgentID
			sibling2 := root.Children[1].AgentID

			msg := &Message{
				FromAgent: sibling1,
				ToAgents:  []string{sibling2},
			}

			targets, err := tt.RouteMessage(msg)
			require.NoError(t, err)
			assert.NotEmpty(t, targets)

			// Should route through common parent (root)
			targetIDs := make([]string, len(targets))
			for i, tgt := range targets {
				targetIDs[i] = tgt.ID
			}
			assert.Contains(t, targetIDs, root.AgentID,
				"sibling message should route through parent")
		}
	})
}

// ==========================================================================
// BroadcastMessage
// ==========================================================================

func TestTreeTopology_BroadcastMessage(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyTree)
	config.MessageTimeout = 2 * time.Second
	tt := NewTreeTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTreeTestAgents()
	err := tt.Initialize(ctx, agents)
	require.NoError(t, err)

	t.Run("broadcast from root succeeds", func(t *testing.T) {
			t.Parallel()
		msg := &Message{
			FromAgent:   "architect-1",
			Content:     "test broadcast",
			MessageType: MessageTypeProposal,
		}

		err := tt.BroadcastMessage(ctx, msg)
		require.NoError(t, err)
		assert.NotEmpty(t, msg.ID, "broadcast should assign an ID")
	})

	t.Run("broadcast on nil root fails", func(t *testing.T) {
			t.Parallel()
		emptyTree := NewTreeTopology(config)
		msg := &Message{
			FromAgent: "nobody",
			Content:   "fail",
		}
		err := emptyTree.BroadcastMessage(ctx, msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no root")
	})
}

// ==========================================================================
// SelectLeader
// ==========================================================================

func TestTreeTopology_SelectLeader(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyTree)
	tt := NewTreeTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTreeTestAgents()
	err := tt.Initialize(ctx, agents)
	require.NoError(t, err)

	t.Run("proposal phase selects root", func(t *testing.T) {
			t.Parallel()
		leader, err := tt.SelectLeader(PhaseProposal)
		require.NoError(t, err)
		assert.Equal(t, "architect-1", leader.ID)
	})

	t.Run("convergence phase selects root", func(t *testing.T) {
			t.Parallel()
		leader, err := tt.SelectLeader(PhaseConvergence)
		require.NoError(t, err)
		assert.Equal(t, "architect-1", leader.ID)
	})

	t.Run("critique phase selects security lead if available", func(t *testing.T) {
			t.Parallel()
		leader, err := tt.SelectLeader(PhaseCritique)
		require.NoError(t, err)
		// Security agent is a lead in the tree
		assert.Equal(t, "security-1", leader.ID)
	})

	t.Run("optimization phase selects performance lead if available",
		func(t *testing.T) {
			leader, err := tt.SelectLeader(PhaseOptimization)
			require.NoError(t, err)
			assert.Equal(t, "perf-1", leader.ID)
		})

	t.Run("no root returns error", func(t *testing.T) {
			t.Parallel()
		emptyTree := NewTreeTopology(config)
		_, err := emptyTree.SelectLeader(PhaseProposal)
		assert.Error(t, err)
	})
}

// ==========================================================================
// GetParallelGroups
// ==========================================================================

func TestTreeTopology_GetParallelGroups(t *testing.T) {
	t.Parallel()
	config := DefaultTopologyConfig(TopologyTree)
	tt := NewTreeTopology(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agents := createTreeTestAgents()
	err := tt.Initialize(ctx, agents)
	require.NoError(t, err)

	t.Run("returns subtree groups", func(t *testing.T) {
			t.Parallel()
		groups := tt.GetParallelGroups(PhaseProposal)
		assert.NotEmpty(t, groups)

		// Total agents in groups should cover non-root agents
		totalInGroups := 0
		for _, g := range groups {
			totalInGroups += len(g)
		}
		// At least all non-root agents should be in groups
		assert.GreaterOrEqual(t, totalInGroups, 1)
	})

	t.Run("single-agent tree returns root as group", func(t *testing.T) {
			t.Parallel()
		singleTree := NewTreeTopology(config)
		ctxSingle, cancelSingle := context.WithCancel(
			context.Background(),
		)
		defer cancelSingle()

		solo := &Agent{
			ID: "solo", Role: RoleArchitect,
			Provider: "p", Model: "m", Score: 9.0,
			Capabilities: []string{"all"},
			Metadata:     map[string]interface{}{},
		}
		err := singleTree.Initialize(ctxSingle, []*Agent{solo})
		require.NoError(t, err)

		groups := singleTree.GetParallelGroups(PhaseProposal)
		require.Len(t, groups, 1)
		assert.Len(t, groups[0], 1)
		assert.Equal(t, "solo", groups[0][0].ID)
	})

	t.Run("nil root returns nil", func(t *testing.T) {
			t.Parallel()
		emptyTree := NewTreeTopology(config)
		groups := emptyTree.GetParallelGroups(PhaseProposal)
		assert.Nil(t, groups)
	})
}
