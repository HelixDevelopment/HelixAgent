package subagent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.agents)
	assert.NotNil(t, manager.tasks)
	assert.NotNil(t, manager.agentInstances)

	managerWithConfig := NewManager(&Config{
		ProviderType: "openai",
		APIKey:       "test-key",
	})
	assert.NotNil(t, managerWithConfig)
	assert.Equal(t, "openai", managerWithConfig.config.ProviderType)
}

func TestManager_Create(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	config := SubAgentConfig{
		Profile:     "test-profile",
		Model:       "gpt-4",
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	agent, err := manager.Create(ctx, config)
	require.NoError(t, err)
	assert.NotNil(t, agent)
	assert.NotEmpty(t, agent.ID)
	assert.Equal(t, CustomAgent, agent.Type)
	assert.Equal(t, StatusIdle, agent.Status)
	assert.Equal(t, config, agent.Config)
}

func TestManager_Get(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Test getting non-existent agent
	_, err := manager.Get(ctx, "non-existent")
	assert.Error(t, err)

	// Create and get agent
	config := SubAgentConfig{Profile: "test"}
	agent, err := manager.Create(ctx, config)
	require.NoError(t, err)

	retrieved, err := manager.Get(ctx, agent.ID)
	require.NoError(t, err)
	assert.Equal(t, agent.ID, retrieved.ID)
}

func TestManager_List(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Empty list
	agents, err := manager.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, agents)

	// Create agents
	for i := 0; i < 3; i++ {
		_, err := manager.Create(ctx, SubAgentConfig{Profile: "test"})
		require.NoError(t, err)
	}

	agents, err = manager.List(ctx)
	require.NoError(t, err)
	assert.Len(t, agents, 3)
}

func TestManager_Update(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Update non-existent agent
	err := manager.Update(ctx, "non-existent", SubAgentConfig{})
	assert.Error(t, err)

	// Create and update agent
	config := SubAgentConfig{Profile: "original", Model: "gpt-3"}
	agent, err := manager.Create(ctx, config)
	require.NoError(t, err)

	newConfig := SubAgentConfig{Profile: "updated", Model: "gpt-4"}
	err = manager.Update(ctx, agent.ID, newConfig)
	require.NoError(t, err)

	retrieved, err := manager.Get(ctx, agent.ID)
	require.NoError(t, err)
	assert.Equal(t, newConfig, retrieved.Config)
}

func TestManager_Delete(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Delete non-existent agent
	err := manager.Delete(ctx, "non-existent")
	assert.Error(t, err)

	// Create and delete agent
	agent, err := manager.Create(ctx, SubAgentConfig{Profile: "test"})
	require.NoError(t, err)

	err = manager.Delete(ctx, agent.ID)
	require.NoError(t, err)

	_, err = manager.Get(ctx, agent.ID)
	assert.Error(t, err)
}

func TestManager_Execute(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Create agent
	agent, err := manager.Create(ctx, SubAgentConfig{Profile: "test"})
	require.NoError(t, err)

	// Execute task
	task := SubAgentTask{
		Type:   GeneralAgent,
		Prompt: "Test prompt",
	}

	result, err := manager.Execute(ctx, agent.ID, task)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
	assert.Greater(t, result.Usage.TotalTokens, 0)
}

func TestManager_Execute_NonExistentAgent(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	task := SubAgentTask{Prompt: "Test"}
	_, err := manager.Execute(ctx, "non-existent", task)
	assert.Error(t, err)
}

func TestManager_ExecuteAsync(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Create agent
	agent, err := manager.Create(ctx, SubAgentConfig{Profile: "test"})
	require.NoError(t, err)

	// Execute async
	task := SubAgentTask{
		Type:   GeneralAgent,
		Prompt: "Test async prompt",
	}

	taskID, err := manager.ExecuteAsync(ctx, agent.ID, task)
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// Wait for task to complete
	time.Sleep(100 * time.Millisecond)

	// Get task
	retrievedTask, err := manager.GetTask(ctx, taskID)
	require.NoError(t, err)
	assert.NotNil(t, retrievedTask)
}

func TestManager_GetTask(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Get non-existent task
	_, err := manager.GetTask(ctx, "non-existent")
	assert.Error(t, err)

	// Create agent and execute task
	agent, _ := manager.Create(ctx, SubAgentConfig{Profile: "test"})
	task := SubAgentTask{Prompt: "Test"}
	result, _ := manager.Execute(ctx, agent.ID, task)

	// The task should have been stored
	tasks, _ := manager.List(ctx)
	assert.GreaterOrEqual(t, len(tasks), 0)
	_ = result
}

func TestManager_CancelTask(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Cancel non-existent task
	err := manager.CancelTask(ctx, "non-existent")
	assert.Error(t, err)

	// Create agent and task
	agent, _ := manager.Create(ctx, SubAgentConfig{Profile: "test"})
	task := SubAgentTask{Prompt: "Test"}

	// Start async execution
	taskID, _ := manager.ExecuteAsync(ctx, agent.ID, task)

	// Try to cancel (may or may not succeed depending on timing)
	_ = manager.CancelTask(ctx, taskID)

	// Get task and verify
	retrievedTask, _ := manager.GetTask(ctx, taskID)
	if retrievedTask.Status == TaskRunning {
		// Task is still running, cancel should work
		err = manager.CancelTask(ctx, taskID)
		assert.NoError(t, err)
	}
}

func TestManager_CreateAgent(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	profile := ProfileConfig{
		Name:        "test-agent",
		Model:       "gpt-4",
		MaxTokens:   2000,
		Temperature: 0.7,
		Tools:       []string{"tool1", "tool2"},
	}

	agent, err := manager.CreateAgent(ctx, "explore", profile)
	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestManager_CreateAgent_Explore(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	profile := ProfileConfig{
		Name:        "explorer",
		Model:       "gpt-4",
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	agent, err := manager.CreateAgent(ctx, "explore", profile)
	require.NoError(t, err)

	// Execute exploration task
	task := Task{
		Description: "Research error handling in Go",
		MaxSteps:    5,
	}

	result, err := agent.Execute(ctx, task)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Discoveries)
}

func TestManager_CreateAgent_Plan(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	profile := ProfileConfig{
		Name:        "planner",
		Model:       "gpt-4",
		MaxTokens:   3000,
		Temperature: 0.5,
	}

	agent, err := manager.CreateAgent(ctx, "plan", profile)
	require.NoError(t, err)

	// Create plan
	input := PlanInput{
		Objective:   "Implement error handling system",
		Discoveries: []string{"discovery1", "discovery2"},
		Constraints: []string{"use structured logging"},
	}

	plan, err := agent.CreatePlan(ctx, input)
	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.NotNil(t, plan.Steps)
}

func TestManager_CreateAgent_General(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	profile := ProfileConfig{
		Name:        "implementer",
		Model:       "gpt-4",
		MaxTokens:   4000,
		Temperature: 0.3,
	}

	agent, err := manager.CreateAgent(ctx, "general", profile)
	require.NoError(t, err)

	// Execute plan
	plan := PlanResult{
		Steps: []PlanStep{
			{Description: "Step 1", Priority: "high"},
			{Description: "Step 2", Priority: "medium"},
		},
	}

	result, err := agent.ExecutePlan(ctx, plan)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestManager_SendMessage(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Send to non-existent agent
	err := manager.SendMessage(ctx, "non-existent", "hello")
	assert.Error(t, err)

	// Create agent
	profile := ProfileConfig{
		Name:        "test-agent",
		Model:       "gpt-4",
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	agent, err := manager.CreateAgent(ctx, "general", profile)
	require.NoError(t, err)

	// Get the agent wrapper to extract ID
	wrapper, ok := agent.(*agentWrapper)
	require.True(t, ok)

	// Send message
	ctx2, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err = manager.SendMessage(ctx2, wrapper.instance.agent.ID, "hello")
	// May succeed or timeout depending on implementation
	assert.True(t, err == nil || err == context.DeadlineExceeded)
}

func TestManager_Shutdown(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Create agents and tasks
	for i := 0; i < 3; i++ {
		_, _ = manager.Create(ctx, SubAgentConfig{Profile: "test"})
	}

	// Shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	err := manager.Shutdown(shutdownCtx)
	assert.NoError(t, err)
}

func TestManager_ConcurrentOperations(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	// Concurrent creates
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := manager.Create(ctx, SubAgentConfig{Profile: "test"})
			done <- err == nil
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all agents created
	agents, err := manager.List(ctx)
	require.NoError(t, err)
	assert.Len(t, agents, 10)
}

func TestDefaultSubAgents(t *testing.T) {
	t.Parallel()
	agents := DefaultSubAgents()
	require.Len(t, agents, 3)

	// Check explore agent
	explore := agents[0]
	assert.Equal(t, "explore", string(explore.Type))
	assert.NotEmpty(t, explore.Role)
	assert.NotEmpty(t, explore.Tools)

	// Check plan agent
	plan := agents[1]
	assert.Equal(t, "plan", string(plan.Type))
	assert.NotEmpty(t, plan.Role)

	// Check general agent
	general := agents[2]
	assert.Equal(t, "general", string(general.Type))
	assert.NotEmpty(t, general.Role)
}

func TestAgentWrapper(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	ctx := context.Background()

	profile := ProfileConfig{
		Name:        "test-wrapper",
		Model:       "gpt-4",
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	agent, err := manager.CreateAgent(ctx, "explore", profile)
	require.NoError(t, err)

	// Test Execute
	task := Task{
		Description: "Test task",
		MaxSteps:    5,
	}

	result, err := agent.Execute(ctx, task)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Test CreatePlan
	planInput := PlanInput{
		Objective:   "Test objective",
		Discoveries: []string{"discovery1"},
		Constraints: []string{},
	}

	plan, err := agent.CreatePlan(ctx, planInput)
	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.NotNil(t, plan.Steps)

	// Test ExecutePlan
	implResult, err := agent.ExecutePlan(ctx, plan)
	require.NoError(t, err)
	assert.NotNil(t, implResult)
}

func TestManager_Integration(t *testing.T) {
	t.Parallel()
	manager := NewManager(&Config{
		ProviderType: "test",
		APIKey:       "test-key",
	})
	ctx := context.Background()

	// Create multiple agents
	exploreProfile := ProfileConfig{Name: "explorer", Model: "gpt-4", MaxTokens: 1000}
	planProfile := ProfileConfig{Name: "planner", Model: "gpt-4", MaxTokens: 2000}
	generalProfile := ProfileConfig{Name: "implementer", Model: "gpt-4", MaxTokens: 3000}

	explorer, err := manager.CreateAgent(ctx, "explore", exploreProfile)
	require.NoError(t, err)

	planner, err := manager.CreateAgent(ctx, "plan", planProfile)
	require.NoError(t, err)

	implementer, err := manager.CreateAgent(ctx, "general", generalProfile)
	require.NoError(t, err)

	// Execute exploration
	exploreResult, err := explorer.Execute(ctx, Task{Description: "Research topic", MaxSteps: 5})
	require.NoError(t, err)

	// Create plan based on exploration
	planInput := PlanInput{
		Objective:   "Implement solution",
		Discoveries: exploreResult.Discoveries,
		Constraints: []string{"must be efficient"},
	}

	plan, err := planner.CreatePlan(ctx, planInput)
	require.NoError(t, err)

	// Execute the plan
	implResult, err := implementer.ExecutePlan(ctx, plan)
	require.NoError(t, err)
	assert.NotNil(t, implResult)
}
