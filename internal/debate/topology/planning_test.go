package topology

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==========================================================================
// PlanningStyleSelector — SelectStyle
// ==========================================================================

func TestPlanningStyleSelector_SelectStyle_SmallTeam(t *testing.T) {
	t.Parallel()
	selector := NewPlanningStyleSelector(PlanningAdaptive)

	// Small team (<=3) should select CPDE regardless of
	// complexity/ambiguity (unless ambiguity is very high)
	style := selector.SelectStyle(0.5, 0.2, 2)
	assert.Equal(t, PlanningCPDE, style,
		"small team (2 agents) with moderate complexity should use CPDE")

	style = selector.SelectStyle(0.3, 0.1, 3)
	assert.Equal(t, PlanningCPDE, style,
		"small team (3 agents) should use CPDE")
}

func TestPlanningStyleSelector_SelectStyle_LargeTeam(t *testing.T) {
	t.Parallel()
	selector := NewPlanningStyleSelector(PlanningAdaptive)

	// Large team (>8) should select DPDE
	style := selector.SelectStyle(0.5, 0.2, 9)
	assert.Equal(t, PlanningDPDE, style,
		"large team (9 agents) should use DPDE")

	style = selector.SelectStyle(0.4, 0.1, 15)
	assert.Equal(t, PlanningDPDE, style,
		"large team (15 agents) should use DPDE")
}

func TestPlanningStyleSelector_SelectStyle_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		complexity    float64
		ambiguity     float64
		agentCount    int
		defaultStyle  PlanningStyle
		expectedStyle PlanningStyle
	}{
		{
			name:          "high complexity low ambiguity => CPDE",
			complexity:    0.8,
			ambiguity:     0.2,
			agentCount:    5,
			defaultStyle:  PlanningAdaptive,
			expectedStyle: PlanningCPDE,
		},
		{
			name:          "high ambiguity => DPDE",
			complexity:    0.5,
			ambiguity:     0.7,
			agentCount:    5,
			defaultStyle:  PlanningCPDE,
			expectedStyle: PlanningDPDE,
		},
		{
			name:          "medium everything uses default",
			complexity:    0.5,
			ambiguity:     0.4,
			agentCount:    5,
			defaultStyle:  PlanningAdaptive,
			expectedStyle: PlanningAdaptive,
		},
		{
			name: "high complexity and high ambiguity " +
				"=> ambiguity wins (DPDE)",
			complexity:    0.9,
			ambiguity:     0.8,
			agentCount:    5,
			defaultStyle:  PlanningAdaptive,
			expectedStyle: PlanningDPDE,
		},
		{
			name: "high complexity exact boundary " +
				"(ambiguity=0.3) => CPDE",
			complexity:    0.75,
			ambiguity:     0.29,
			agentCount:    5,
			defaultStyle:  PlanningAdaptive,
			expectedStyle: PlanningCPDE,
		},
		{
			name:          "exactly 3 agents => CPDE (small team)",
			complexity:    0.4,
			ambiguity:     0.4,
			agentCount:    3,
			defaultStyle:  PlanningDPDE,
			expectedStyle: PlanningCPDE,
		},
		{
			name:          "exactly 8 agents => default (not large)",
			complexity:    0.4,
			ambiguity:     0.4,
			agentCount:    8,
			defaultStyle:  PlanningAdaptive,
			expectedStyle: PlanningAdaptive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			selector := NewPlanningStyleSelector(tc.defaultStyle)
			result := selector.SelectStyle(
				tc.complexity, tc.ambiguity, tc.agentCount,
			)
			assert.Equal(t, tc.expectedStyle, result)
		})
	}
}

// ==========================================================================
// NewTaskPlan
// ==========================================================================

func TestNewTaskPlan(t *testing.T) {
	t.Parallel()
	plan := NewTaskPlan("planner-1", PlanningCPDE)

	require.NotNil(t, plan)
	assert.Equal(t, "planner-1", plan.PlannerAgentID)
	assert.Equal(t, PlanningCPDE, plan.Style)
	assert.NotNil(t, plan.Tasks)
	assert.Empty(t, plan.Tasks)
	assert.NotNil(t, plan.Ordering)
	assert.Empty(t, plan.Ordering)
	assert.False(t, plan.CreatedAt.IsZero(),
		"CreatedAt should be set")
}

// ==========================================================================
// AddTask
// ==========================================================================

func TestTaskPlan_AddTask(t *testing.T) {
	t.Parallel()
	plan := NewTaskPlan("planner-1", PlanningCPDE)

	plan.AddTask("t1", "design API", "agent-a", 5)
	plan.AddTask("t2", "implement handler", "agent-b", 3, "t1")

	assert.Len(t, plan.Tasks, 2)
	assert.Equal(t, []string{"t1", "t2"}, plan.Ordering)

	// First task
	assert.Equal(t, "t1", plan.Tasks[0].ID)
	assert.Equal(t, "design API", plan.Tasks[0].Description)
	assert.Equal(t, "agent-a", plan.Tasks[0].AssignedTo)
	assert.Equal(t, 5, plan.Tasks[0].Priority)
	assert.Empty(t, plan.Tasks[0].Dependencies)
	assert.Equal(t, "pending", plan.Tasks[0].Status)

	// Second task with dependency
	assert.Equal(t, "t2", plan.Tasks[1].ID)
	assert.Equal(t, []string{"t1"}, plan.Tasks[1].Dependencies)
}

// ==========================================================================
// AddDependency (via AddTask with dependencies)
// ==========================================================================

func TestTaskPlan_AddDependency(t *testing.T) {
	t.Parallel()
	plan := NewTaskPlan("planner-1", PlanningCPDE)

	plan.AddTask("t1", "foundation", "agent-a", 5)
	plan.AddTask("t2", "walls", "agent-b", 3, "t1")
	plan.AddTask("t3", "roof", "agent-c", 2, "t1", "t2")

	require.Len(t, plan.Tasks, 3)

	// t1 has no dependencies
	assert.Empty(t, plan.Tasks[0].Dependencies)

	// t2 depends on t1
	assert.Equal(t, []string{"t1"}, plan.Tasks[1].Dependencies)

	// t3 depends on t1 and t2
	assert.Equal(t, []string{"t1", "t2"}, plan.Tasks[2].Dependencies)
}

// ==========================================================================
// GetReadyTasks
// ==========================================================================

func TestTaskPlan_GetReadyTasks(t *testing.T) {
	t.Parallel()
	t.Run("all tasks ready when no dependencies", func(t *testing.T) {
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("t1", "task 1", "a", 5)
		plan.AddTask("t2", "task 2", "b", 3)

		ready := plan.GetReadyTasks()
		assert.Len(t, ready, 2)
	})

	t.Run("only root tasks ready initially", func(t *testing.T) {
			t.Parallel()
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("t1", "design", "a", 5)
		plan.AddTask("t2", "implement", "b", 3, "t1")
		plan.AddTask("t3", "test", "c", 2, "t2")

		ready := plan.GetReadyTasks()
		require.Len(t, ready, 1)
		assert.Equal(t, "t1", ready[0].ID)
	})

	t.Run("dependent tasks become ready after completion",
		func(t *testing.T) {
			plan := NewTaskPlan("planner-1", PlanningCPDE)
			plan.AddTask("t1", "design", "a", 5)
			plan.AddTask("t2", "implement", "b", 3, "t1")
			plan.AddTask("t3", "test", "c", 2, "t1")

			err := plan.MarkCompleted("t1")
			require.NoError(t, err)

			ready := plan.GetReadyTasks()
			assert.Len(t, ready, 2,
				"t2 and t3 should be ready after t1 is completed")

			readyIDs := make([]string, len(ready))
			for i, r := range ready {
				readyIDs[i] = r.ID
			}
			assert.Contains(t, readyIDs, "t2")
			assert.Contains(t, readyIDs, "t3")
		})

	t.Run("completed tasks not in ready list", func(t *testing.T) {
			t.Parallel()
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("t1", "design", "a", 5)

		err := plan.MarkCompleted("t1")
		require.NoError(t, err)

		ready := plan.GetReadyTasks()
		assert.Empty(t, ready,
			"completed tasks should not appear in ready list")
	})

	t.Run("empty plan returns nil", func(t *testing.T) {
			t.Parallel()
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		ready := plan.GetReadyTasks()
		assert.Nil(t, ready)
	})
}

// ==========================================================================
// MarkCompleted
// ==========================================================================

func TestTaskPlan_MarkCompleted(t *testing.T) {
	t.Parallel()
	t.Run("marks existing task as completed", func(t *testing.T) {
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("t1", "do thing", "a", 5)

		err := plan.MarkCompleted("t1")
		require.NoError(t, err)
		assert.Equal(t, "completed", plan.Tasks[0].Status)
	})

	t.Run("non-existent task returns error", func(t *testing.T) {
			t.Parallel()
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("t1", "do thing", "a", 5)

		err := plan.MarkCompleted("t99")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "t99")
	})

	t.Run("IsComplete after all marked", func(t *testing.T) {
			t.Parallel()
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("t1", "first", "a", 5)
		plan.AddTask("t2", "second", "b", 3)

		assert.False(t, plan.IsComplete())

		_ = plan.MarkCompleted("t1")
		assert.False(t, plan.IsComplete())

		_ = plan.MarkCompleted("t2")
		assert.True(t, plan.IsComplete())
	})
}

// ==========================================================================
// TopologicalSort (GetExecutionOrder)
// ==========================================================================

func TestTaskPlan_TopologicalSort(t *testing.T) {
	t.Parallel()
	t.Run("linear dependency chain", func(t *testing.T) {
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("t1", "design", "a", 5)
		plan.AddTask("t2", "implement", "b", 3, "t1")
		plan.AddTask("t3", "test", "c", 2, "t2")

		order := plan.GetExecutionOrder()
		require.Len(t, order, 3)

		// Verify order: t1 before t2 before t3
		idxOf := func(id string) int {
			for i, o := range order {
				if o.ID == id {
					return i
				}
			}
			return -1
		}

		assert.Less(t, idxOf("t1"), idxOf("t2"))
		assert.Less(t, idxOf("t2"), idxOf("t3"))
	})

	t.Run("diamond dependency", func(t *testing.T) {
			t.Parallel()
		// t1 -> t2, t1 -> t3, t2 -> t4, t3 -> t4
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("t1", "root", "a", 5)
		plan.AddTask("t2", "left", "b", 4, "t1")
		plan.AddTask("t3", "right", "c", 3, "t1")
		plan.AddTask("t4", "merge", "d", 2, "t2", "t3")

		order := plan.GetExecutionOrder()
		require.Len(t, order, 4)

		idxOf := func(id string) int {
			for i, o := range order {
				if o.ID == id {
					return i
				}
			}
			return -1
		}

		assert.Equal(t, 0, idxOf("t1"),
			"t1 should be first (no deps)")
		assert.Less(t, idxOf("t1"), idxOf("t2"))
		assert.Less(t, idxOf("t1"), idxOf("t3"))
		assert.Less(t, idxOf("t2"), idxOf("t4"))
		assert.Less(t, idxOf("t3"), idxOf("t4"))
	})

	t.Run("independent tasks ordered by priority", func(t *testing.T) {
			t.Parallel()
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		plan.AddTask("low", "low priority", "a", 1)
		plan.AddTask("high", "high priority", "b", 5)
		plan.AddTask("med", "medium priority", "c", 3)

		order := plan.GetExecutionOrder()
		require.Len(t, order, 3)

		// All independent, should be sorted by priority desc
		assert.Equal(t, "high", order[0].ID)
		assert.Equal(t, "med", order[1].ID)
		assert.Equal(t, "low", order[2].ID)
	})

	t.Run("empty plan returns nil", func(t *testing.T) {
			t.Parallel()
		plan := NewTaskPlan("planner-1", PlanningCPDE)
		order := plan.GetExecutionOrder()
		assert.Nil(t, order)
	})

	t.Run("dependency on non-existent task is ignored",
		func(t *testing.T) {
			plan := NewTaskPlan("planner-1", PlanningCPDE)
			plan.AddTask("t1", "task", "a", 5, "phantom")

			order := plan.GetExecutionOrder()
			require.Len(t, order, 1)
			assert.Equal(t, "t1", order[0].ID)
		})
}
