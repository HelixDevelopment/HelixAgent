package subagent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOrchestrator(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)

	assert.NotNil(t, orchestrator)
	assert.Equal(t, manager, orchestrator.manager)
	assert.NotNil(t, orchestrator.sessions)
	assert.NotNil(t, orchestrator.shutdown)
}

func TestOrchestrator_CreateSession(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	session, err := orchestrator.CreateSession(ctx)
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, SessionStatusPending, session.Status)
	assert.NotNil(t, session.Results)
	assert.NotNil(t, session.Context)
}

func TestOrchestrator_GetSession(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Get non-existent session
	_, err := orchestrator.GetSession(ctx, "non-existent")
	assert.Error(t, err)

	// Create and get session
	session, err := orchestrator.CreateSession(ctx)
	require.NoError(t, err)

	retrieved, err := orchestrator.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrieved.ID)
}

func TestOrchestrator_ListSessions(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Empty list
	sessions, err := orchestrator.ListSessions(ctx)
	require.NoError(t, err)
	assert.Empty(t, sessions)

	// Create sessions
	for i := 0; i < 3; i++ {
		_, err := orchestrator.CreateSession(ctx)
		require.NoError(t, err)
	}

	sessions, err = orchestrator.ListSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, sessions, 3)
}

func TestOrchestrator_CancelSession(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Cancel non-existent session
	err := orchestrator.CancelSession(ctx, "non-existent")
	assert.Error(t, err)

	// Create session
	session, err := orchestrator.CreateSession(ctx)
	require.NoError(t, err)

	// Cancel non-running session should error
	err = orchestrator.CancelSession(ctx, session.ID)
	assert.Error(t, err)
}

func TestOrchestrator_Cleanup(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Create sessions
	for i := 0; i < 5; i++ {
		_, err := orchestrator.CreateSession(ctx)
		require.NoError(t, err)
	}

	sessions, _ := orchestrator.ListSessions(ctx)
	assert.Len(t, sessions, 5)

	// Mark some sessions as completed with old timestamp
	orchestrator.sessionsMu.Lock()
	count := 0
	for _, session := range orchestrator.sessions {
		if count < 3 {
			session.Status = SessionStatusCompleted
			session.UpdatedAt = time.Now().Add(-2 * time.Hour)
		}
		count++
	}
	orchestrator.sessionsMu.Unlock()

	// Cleanup sessions older than 1 hour
	err := orchestrator.Cleanup(ctx, time.Hour)
	require.NoError(t, err)

	sessions, _ = orchestrator.ListSessions(ctx)
	assert.Len(t, sessions, 2)
}

func TestOrchestrator_Shutdown(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		_, _ = orchestrator.CreateSession(ctx)
	}

	// Shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	err := orchestrator.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

func TestOrchestrator_ExecutePlan(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Create session
	session, err := orchestrator.CreateSession(ctx)
	require.NoError(t, err)

	// Create a simple plan
	plan := OrchestrationPlan{
		Name:        "test-plan",
		Description: "Test orchestration plan",
		Steps: []OrchestrationStep{
			{
				Name:        "step1",
				AgentType:   ExploreAgent,
				Description: "First step",
				DependsOn:   []string{},
			},
			{
				Name:        "step2",
				AgentType:   PlanAgent,
				Description: "Second step",
				DependsOn:   []string{"step1"},
			},
		},
	}

	// Execute plan
	err = orchestrator.ExecutePlan(ctx, session.ID, plan)
	require.NoError(t, err)

	// Verify session is completed
	updatedSession, err := orchestrator.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, SessionStatusCompleted, updatedSession.Status)
	assert.Len(t, updatedSession.Results, 2)
}

func TestOrchestrator_ExecutePlan_NonExistentSession(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	plan := OrchestrationPlan{
		Name: "test-plan",
		Steps: []OrchestrationStep{
			{Name: "step1", AgentType: ExploreAgent, Description: "Step 1"},
		},
	}

	err := orchestrator.ExecutePlan(ctx, "non-existent", plan)
	assert.Error(t, err)
}

func TestOrchestrator_ExecutePlan_Cancellation(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx, cancel := context.WithCancel(context.Background())

	// Create session
	session, _ := orchestrator.CreateSession(ctx)

	// Cancel context immediately
	cancel()

	// Create a plan with steps
	plan := OrchestrationPlan{
		Name: "test-plan",
		Steps: []OrchestrationStep{
			{Name: "step1", AgentType: ExploreAgent, Description: "Step 1"},
			{Name: "step2", AgentType: ExploreAgent, Description: "Step 2"},
		},
	}

	err := orchestrator.ExecutePlan(ctx, session.ID, plan)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestOrchestrator_ExecuteParallel(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Create session
	session, err := orchestrator.CreateSession(ctx)
	require.NoError(t, err)

	// Execute parallel tasks
	prompts := []string{
		"Task 1 description",
		"Task 2 description",
		"Task 3 description",
	}

	results, err := orchestrator.ExecuteParallel(ctx, session.ID, prompts, ExploreAgent)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	for _, result := range results {
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Content)
	}
}

func TestOrchestrator_ExecuteParallel_NonExistentSession(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	prompts := []string{"Task 1", "Task 2"}
	_, err := orchestrator.ExecuteParallel(ctx, "non-existent", prompts, ExploreAgent)
	assert.Error(t, err)
}

func TestCreateDefaultPlan(t *testing.T) {
	t.Parallel()
	objective := "Implement a new feature"
	plan := CreateDefaultPlan(objective)

	assert.Equal(t, "default-3-step", plan.Name)
	assert.NotEmpty(t, plan.Description)
	assert.Len(t, plan.Steps, 3)

	// Check explore step
	assert.Equal(t, "explore", plan.Steps[0].Name)
	assert.Equal(t, ExploreAgent, plan.Steps[0].AgentType)
	assert.Empty(t, plan.Steps[0].DependsOn)

	// Check plan step
	assert.Equal(t, "plan", plan.Steps[1].Name)
	assert.Equal(t, PlanAgent, plan.Steps[1].AgentType)
	assert.Equal(t, []string{"explore"}, plan.Steps[1].DependsOn)

	// Check execute step
	assert.Equal(t, "execute", plan.Steps[2].Name)
	assert.Equal(t, GeneralAgent, plan.Steps[2].AgentType)
	assert.Equal(t, []string{"plan"}, plan.Steps[2].DependsOn)
}

func TestOrchestrator_ConcurrentSessions(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Create multiple sessions concurrently
	done := make(chan string, 10)
	for i := 0; i < 10; i++ {
		go func() {
			session, err := orchestrator.CreateSession(ctx)
			if err == nil {
				done <- session.ID
			} else {
				done <- ""
			}
		}()
	}

	sessionIDs := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		id := <-done
		if id != "" {
			sessionIDs = append(sessionIDs, id)
		}
	}

	assert.Len(t, sessionIDs, 10)

	// Verify all sessions exist
	sessions, err := orchestrator.ListSessions(ctx)
	require.NoError(t, err)
	assert.Len(t, sessions, 10)
}

func TestOrchestrator_Integration(t *testing.T) {
	t.Parallel()
	manager := NewManager(&Config{
		ProviderType: "test",
		APIKey:       "test-key",
	})
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	// Create a session
	session, err := orchestrator.CreateSession(ctx)
	require.NoError(t, err)

	// Create a default plan
	plan := CreateDefaultPlan("Implement error handling system")

	// Execute the plan
	err = orchestrator.ExecutePlan(ctx, session.ID, plan)
	require.NoError(t, err)

	// Verify results
	completedSession, err := orchestrator.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, SessionStatusCompleted, completedSession.Status)
	assert.Len(t, completedSession.Results, 3)

	// Verify each step has results
	for _, step := range plan.Steps {
		result, exists := completedSession.Results[step.Name]
		assert.True(t, exists, "Result should exist for step %s", step.Name)
		assert.NotNil(t, result)
	}
}

func TestOrchestrator_ComplexDependencyChain(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	session, _ := orchestrator.CreateSession(ctx)

	// Create a plan with complex dependencies
	// A -> B -> D
	// A -> C -> D
	plan := OrchestrationPlan{
		Name: "complex-plan",
		Steps: []OrchestrationStep{
			{Name: "A", AgentType: ExploreAgent, Description: "Step A", DependsOn: []string{}},
			{Name: "B", AgentType: ExploreAgent, Description: "Step B", DependsOn: []string{"A"}},
			{Name: "C", AgentType: ExploreAgent, Description: "Step C", DependsOn: []string{"A"}},
			{Name: "D", AgentType: GeneralAgent, Description: "Step D", DependsOn: []string{"B", "C"}},
		},
	}

	err := orchestrator.ExecutePlan(ctx, session.ID, plan)
	require.NoError(t, err)

	// Verify all steps completed
	completedSession, _ := orchestrator.GetSession(ctx, session.ID)
	assert.Len(t, completedSession.Results, 4)
}
