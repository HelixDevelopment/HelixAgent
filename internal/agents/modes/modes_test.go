package modes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	require.NotNil(t, registry)
	assert.NotNil(t, registry.modes)
	assert.GreaterOrEqual(t, len(registry.modes), 6) // Default modes
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	config := &ModeConfig{
		Name:        "Custom",
		Description: "Custom mode",
		Prompt:      "Custom prompt",
	}

	registry.Register(config)

	assert.True(t, registry.HasMode("custom"))
}

func TestRegistry_Get(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	config, ok := registry.Get(ModeCode)
	require.True(t, ok)
	assert.Equal(t, "Code", config.Name)
	assert.NotEmpty(t, config.Prompt)
}

func TestRegistry_Get_NotFound(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	_, ok := registry.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_List(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	modes := registry.List()

	assert.GreaterOrEqual(t, len(modes), 6)
	assert.Contains(t, modes, ModeCode)
	assert.Contains(t, modes, ModeArchitect)
	assert.Contains(t, modes, ModeAsk)
}

func TestRegistry_ListConfigs(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	configs := registry.ListConfigs()

	assert.GreaterOrEqual(t, len(configs), 6)
}

func TestRegistry_HasMode(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	assert.True(t, registry.HasMode(ModeCode))
	assert.True(t, registry.HasMode(ModeArchitect))
	assert.False(t, registry.HasMode("unknown"))
}

func TestNewAgent(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	agent, err := NewAgent(registry, ModeCode)

	require.NoError(t, err)
	require.NotNil(t, agent)
	assert.Equal(t, ModeCode, agent.mode)
}

func TestNewAgent_UnknownMode(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	_, err := NewAgent(registry, "unknown")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mode")
}

func TestAgent_GetMode(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeDebug)

	assert.Equal(t, ModeDebug, agent.GetMode())
}

func TestAgent_SetMode(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	err := agent.SetMode(ModeAsk)

	require.NoError(t, err)
	assert.Equal(t, ModeAsk, agent.GetMode())
}

func TestAgent_SetMode_Invalid(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	err := agent.SetMode("invalid")

	assert.Error(t, err)
}

func TestAgent_GetConfig(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	config := agent.GetConfig()

	require.NotNil(t, config)
	assert.Equal(t, "Code", config.Name)
}

func TestAgent_CanUseTool(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	// Code mode should allow edit
	agent, _ := NewAgent(registry, ModeCode)
	assert.True(t, agent.CanUseTool("edit"))
	assert.True(t, agent.CanUseTool("read"))

	// Ask mode should not allow edit
	agent, _ = NewAgent(registry, ModeAsk)
	assert.False(t, agent.CanUseTool("edit"))
	assert.True(t, agent.CanUseTool("read"))
}

func TestAgent_CanUseTool_Unknown(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	assert.False(t, agent.CanUseTool("unknown"))
}

func TestAgent_CanExecute(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	// Code mode allows write but not execute
	agent, _ := NewAgent(registry, ModeCode)
	assert.True(t, agent.CanExecute("write"))
	assert.False(t, agent.CanExecute("execute"))

	// Debug mode allows both
	agent, _ = NewAgent(registry, ModeDebug)
	assert.True(t, agent.CanExecute("write"))
	assert.True(t, agent.CanExecute("execute"))
}

func TestAgent_GetPrompt(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	prompt := agent.GetPrompt()

	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "CODE")
}

func TestAgent_GetTemperature(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	temp := agent.GetTemperature()

	// Code mode should have low temperature
	assert.Equal(t, 0.1, temp)
}

func TestAgent_GetMaxTokens(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	tokens := agent.GetMaxTokens()

	assert.Greater(t, tokens, 0)
}

func TestWithMode(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	ctx := WithMode(context.Background(), agent)

	mc, ok := ctx.(*ModeContext)
	require.True(t, ok)
	assert.Equal(t, ModeCode, mc.Mode)
	assert.NotNil(t, mc.Config)
}

func TestGetModeFromContext(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeDebug)

	ctx := WithMode(context.Background(), agent)
	mode, ok := GetModeFromContext(ctx)

	assert.True(t, ok)
	assert.Equal(t, ModeDebug, mode)
}

func TestGetModeFromContext_NotFound(t *testing.T) {
	t.Parallel()
	mode, ok := GetModeFromContext(context.Background())

	assert.False(t, ok)
	assert.Empty(t, mode)
}

func TestAgent_SwitchMode(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	ctx := context.Background()
	newCtx, err := agent.SwitchMode(ctx, ModeAsk)

	require.NoError(t, err)
	assert.Equal(t, ModeAsk, agent.GetMode())

	// Check context was updated
	mode, ok := GetModeFromContext(newCtx)
	assert.True(t, ok)
	assert.Equal(t, ModeAsk, mode)
}

func TestAgent_SwitchMode_Invalid(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	ctx := context.Background()
	_, err := agent.SwitchMode(ctx, "invalid")

	assert.Error(t, err)
}

func TestAgentMode_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "code", ModeCode.String())
	assert.Equal(t, "debug", ModeDebug.String())
}

func TestAgentMode_IsValid(t *testing.T) {
	t.Parallel()
	assert.True(t, ModeCode.IsValid())
	assert.True(t, ModeArchitect.IsValid())
	assert.True(t, ModeAsk.IsValid())
	assert.True(t, ModeDebug.IsValid())
	assert.True(t, ModeTest.IsValid())
	assert.True(t, ModeReview.IsValid())
	assert.False(t, AgentMode("invalid").IsValid())
}

func TestDefaultModesConfiguration(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()

	// Test Code mode
	code, _ := registry.Get(ModeCode)
	assert.Equal(t, "Code", code.Name)
	assert.NotEmpty(t, code.Tools)
	assert.Contains(t, code.Tools, "edit")
	assert.Greater(t, code.MaxTokens, 0)

	// Test Architect mode
	architect, _ := registry.Get(ModeArchitect)
	assert.Equal(t, "Architect", architect.Name)
	assert.Equal(t, 8192, architect.MaxTokens)

	// Test Ask mode
	ask, _ := registry.Get(ModeAsk)
	assert.Equal(t, "Ask", ask.Name)
	assert.False(t, ask.Permissions["write"])

	// Test Debug mode
	debug, _ := registry.Get(ModeDebug)
	assert.Equal(t, "Debug", debug.Name)
	assert.True(t, debug.Permissions["execute"])
}

func TestModeContext_ImplementsContext(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	agent, _ := NewAgent(registry, ModeCode)

	ctx := WithMode(context.Background(), agent)

	// Should not panic
	_, hasDeadline := ctx.Deadline()
	_ = hasDeadline
	// ctx.Done() returns nil when no deadline/timer set
	done := ctx.Done()
	_ = done
	assert.Nil(t, ctx.Err())
	assert.Nil(t, ctx.Value("key"))
}
