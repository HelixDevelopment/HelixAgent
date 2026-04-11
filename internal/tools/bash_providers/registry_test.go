package bash_providers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_Discover(t *testing.T) {
	t.Parallel()
	// Create temp directory with test tools
	tmpDir := t.TempDir()

	// Create a test tool
	testTool := `#!/usr/bin/env bash
set -e

# @describe Test tool for unit testing
# @option --name! The name to greet
# @option --count[=1] Number of greetings
# @env REQUIRED_VAR! Required variable
# @env OPTIONAL_VAR[=default] Optional variable

main() {
    echo "Hello $argc_name (x$argc_count)"
}

eval "$(argc --argc-eval "$0" "$@")"
`
	toolPath := filepath.Join(tmpDir, "test_greet.sh")
	err := os.WriteFile(toolPath, []byte(testTool), 0755)
	require.NoError(t, err)

	// Create registry and discover
	registry := NewRegistry(tmpDir)
	err = registry.Discover()
	require.NoError(t, err)

	// Verify tool was discovered
	tools := registry.List()
	require.Len(t, tools, 1)

	tool := tools[0]
	assert.Equal(t, "test_greet", tool.Name)
	assert.Equal(t, "Test tool for unit testing", tool.Description)
	assert.Len(t, tool.Parameters, 2)
	assert.Len(t, tool.EnvVars, 2)

	// Check parameters
	assert.Equal(t, "name", tool.Parameters[0].Name)
	assert.True(t, tool.Parameters[0].Required)
	assert.Equal(t, "count", tool.Parameters[1].Name)
	assert.False(t, tool.Parameters[1].Required)
	// Default field removed from Parameter struct
}

func TestBashTool_ToMCPTool(t *testing.T) {
	t.Parallel()
	tool := &BashTool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: []Parameter{
			{Name: "required_param", Required: true, Description: "Required", Type: "string"},
			{Name: "optional_param", Required: false, Description: "Optional", Type: "string"},
		},
	}

	mcpTool := tool.ToMCPTool()

	assert.Equal(t, "test_tool", mcpTool.Name)
	assert.Equal(t, "A test tool", mcpTool.Description)

	props := mcpTool.InputSchema.Properties
	assert.Contains(t, props, "required_param")
	assert.Contains(t, props, "optional_param")

	required := mcpTool.InputSchema.Required
	assert.Contains(t, required, "required_param")
	assert.NotContains(t, required, "optional_param")
}

func TestRegistry_Execute(t *testing.T) {
	t.Parallel()
	t.Skip("Requires argc tool to be installed")
	// Create temp directory with executable test tool
	tmpDir := t.TempDir()

	testTool := `#!/usr/bin/env bash
set -e

# @describe Echo test tool
# @option --message! Message to echo
# @env LLM_OUTPUT=/dev/stdout

main() {
    echo "Echo: $argc_message" >> "$LLM_OUTPUT"
}

eval "$(argc --argc-eval "$0" "$@")"
`
	toolPath := filepath.Join(tmpDir, "test_echo.sh")
	err := os.WriteFile(toolPath, []byte(testTool), 0755)
	require.NoError(t, err)

	// Create and populate registry
	registry := NewRegistry(tmpDir)

	// Manually add tool (skip discovery)
	registry.tools["test_echo"] = &BashTool{
		Name:        "test_echo",
		Description: "Echo test tool",
		ScriptPath:  toolPath,
		Parameters: []Parameter{
			{Name: "message", Required: true, Type: "string"},
		},
	}

	// Execute tool
	ctx := context.Background()
	result, err := registry.Execute(ctx, "test_echo", map[string]interface{}{
		"message": "Hello World",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "Echo: Hello World")
	assert.False(t, result.IsError)
}

func TestRegistry_Execute_MissingTool(t *testing.T) {
	t.Parallel()
	registry := NewRegistry("/tmp")

	ctx := context.Background()
	_, err := registry.Execute(ctx, "nonexistent", map[string]interface{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
}

func TestRegistry_Execute_MissingRequiredParam(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	testTool := `#!/usr/bin/env bash
set -e
# @describe Test
# @option --required! Required param
# @env LLM_OUTPUT=/dev/stdout
main() { echo "ok"; }
eval "$(argc --argc-eval "$0" "$@")"`

	toolPath := filepath.Join(tmpDir, "test.sh")
	os.WriteFile(toolPath, []byte(testTool), 0755)

	registry := NewRegistry(tmpDir)
	registry.tools["test"] = &BashTool{
		Name:       "test",
		ScriptPath: toolPath,
		Parameters: []Parameter{
			{Name: "required", Required: true},
		},
	}

	ctx := context.Background()
	_, err := registry.Execute(ctx, "test", map[string]interface{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required parameter")
}

func TestRegistry_Get(t *testing.T) {
	t.Parallel()
	registry := NewRegistry("/tmp")

	// Add test tool
	testTool := &BashTool{Name: "test"}
	registry.tools["test"] = testTool

	// Get existing tool
	tool, ok := registry.Get("test")
	assert.True(t, ok)
	assert.Equal(t, testTool, tool)

	// Get non-existent tool
	_, ok = registry.Get("nonexistent")
	assert.False(t, ok)
}

func BenchmarkRegistry_Discover(b *testing.B) {
	// Create temp directory with multiple tools
	tmpDir := b.TempDir()

	for i := 0; i < 50; i++ {
		toolContent := `#!/usr/bin/env bash
# @desc Test tool
# @opt --param! Param
main() { echo "test"; }
eval "$(argc --argc-eval "$0" "$@")"`

		toolPath := filepath.Join(tmpDir, "tool_"+string(rune('a'+i))+".sh")
		os.WriteFile(toolPath, []byte(toolContent), 0755)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		registry := NewRegistry(tmpDir)
		registry.Discover()
	}
}
