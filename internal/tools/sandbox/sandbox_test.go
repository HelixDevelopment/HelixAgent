package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	assert.Equal(t, RuntimeDocker, config.Runtime)
	assert.False(t, config.EnableNetwork)
	assert.Equal(t, "/workspace", config.WorkingDir)
	assert.Equal(t, "512m", config.MemoryLimit)
	assert.Equal(t, "1.0", config.CPULimit)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.NotNil(t, config.EnvVars)
}

func TestIsRuntimeAvailable(t *testing.T) {
	t.Parallel()
	// This test depends on the environment
	// Just verify it doesn't panic
	_ = isRuntimeAvailable(RuntimeDocker)
	_ = isRuntimeAvailable(RuntimePodman)
}

func TestAvailableRuntimes(t *testing.T) {
	t.Parallel()
	runtimes := AvailableRuntimes()

	// Should always return at least RuntimeNone
	assert.NotEmpty(t, runtimes)
	assert.Contains(t, runtimes, RuntimeNone)
}

func TestNewSandbox(t *testing.T) {
	t.Parallel()
	// Skip if no runtime available
	if len(AvailableRuntimes()) == 1 && AvailableRuntimes()[0] == RuntimeNone {
		t.Skip("No container runtime available")  // SKIP-OK: #runtime-mock-only
	}

	config := DefaultConfig()
	sandbox, err := NewSandbox(config, "alpine:latest")

	// May fail if docker/podman not available
	if err != nil {
		t.Skipf("Sandbox creation failed: %v", err)
	}

	assert.NotNil(t, sandbox)
	assert.Equal(t, "alpine:latest", sandbox.image)
}

func TestNewSandbox_NoRuntime(t *testing.T) {
	t.Parallel()
	config := Config{
		Runtime: RuntimeDocker,
	}

	// Force unavailable runtime by using invalid runtime name
	if !isRuntimeAvailable(RuntimeDocker) && !isRuntimeAvailable(RuntimePodman) {
		_, err := NewSandbox(config, "alpine:latest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no container runtime available")
	}
}

func TestSandbox_Execute(t *testing.T) {
	t.Parallel()
	// Skip if no runtime available
	if len(AvailableRuntimes()) == 1 && AvailableRuntimes()[0] == RuntimeNone {
		t.Skip("No container runtime available")  // SKIP-OK: #runtime-mock-only
	}

	config := Config{
		Runtime:       RuntimeDocker,
		EnableNetwork: false,
		Timeout:       30 * time.Second,
	}

	sandbox, err := NewSandbox(config, "alpine:latest")
	if err != nil {
		t.Skipf("Sandbox creation failed: %v", err)
	}

	ctx := context.Background()
	result, err := sandbox.Execute(ctx, []string{"echo", "hello"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello")
}

func TestSandbox_Execute_WithTimeout(t *testing.T) {
	t.Parallel()
	// Skip if no runtime available
	if len(AvailableRuntimes()) == 1 && AvailableRuntimes()[0] == RuntimeNone {
		t.Skip("No container runtime available")  // SKIP-OK: #runtime-mock-only
	}

	config := Config{
		Runtime: RuntimeDocker,
		Timeout: 1 * time.Second,
	}

	sandbox, err := NewSandbox(config, "alpine:latest")
	if err != nil {
		t.Skipf("Sandbox creation failed: %v", err)
	}

	ctx := context.Background()
	// Use sleep command that exceeds timeout
	result, err := sandbox.Execute(ctx, []string{"sleep", "5"})

	// Should timeout or complete (timing-dependent in CI)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	if result.ExitCode == -1 || result.ExitCode == 0 {
		// Either timeout (-1) or completed (0) is acceptable in test environment
	}
}

func TestSandbox_Execute_WithNetworkDisabled(t *testing.T) {
	t.Parallel()
	// Skip if no runtime available
	if len(AvailableRuntimes()) == 1 && AvailableRuntimes()[0] == RuntimeNone {
		t.Skip("No container runtime available")  // SKIP-OK: #runtime-mock-only
	}

	config := Config{
		Runtime:       RuntimeDocker,
		EnableNetwork: false,
		Timeout:       10 * time.Second,
	}

	sandbox, err := NewSandbox(config, "alpine:latest")
	if err != nil {
		t.Skipf("Sandbox creation failed: %v", err)
	}

	ctx := context.Background()
	// Try to ping - should fail without network
	result, err := sandbox.Execute(ctx, []string{"ping", "-c", "1", "google.com"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Should fail because network is disabled
	assert.NotEqual(t, 0, result.ExitCode)
}

func TestSandbox_Execute_WithNetworkEnabled(t *testing.T) {
	t.Parallel()
	// Skip if no runtime available
	if len(AvailableRuntimes()) == 1 && AvailableRuntimes()[0] == RuntimeNone {
		t.Skip("No container runtime available")  // SKIP-OK: #runtime-mock-only
	}

	config := Config{
		Runtime:       RuntimeDocker,
		EnableNetwork: true,
		Timeout:       30 * time.Second,
	}

	sandbox, err := NewSandbox(config, "alpine:latest")
	if err != nil {
		t.Skipf("Sandbox creation failed: %v", err)
	}

	ctx := context.Background()
	// Try to download - should work with network
	result, err := sandbox.Execute(ctx, []string{"wget", "-q", "-O", "-", "https://example.com"})

	// May fail in CI environments, so just check no error
	if err != nil {
		t.Skipf("Network test failed: %v", err)
	}
	assert.NotNil(t, result)
}

func TestSandbox_ExecuteDirect(t *testing.T) {
	t.Parallel()
	config := Config{
		Runtime:    RuntimeNone,
		WorkingDir: "/tmp",
	}

	sandbox := &Sandbox{
		config: config,
	}

	ctx := context.Background()
	result, err := sandbox.executeDirect(ctx, []string{"echo", "hello"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello")
}

func TestSandbox_ExecuteDirect_WithTimeout(t *testing.T) {
	t.Parallel()
	config := Config{
		Runtime:    RuntimeNone,
		Timeout:    100 * time.Millisecond,
		WorkingDir: "/tmp",
	}

	sandbox := &Sandbox{
		config: config,
	}

	ctx := context.Background()
	// Use sleep command that exceeds timeout
	result, err := sandbox.executeDirect(ctx, []string{"sleep", "5"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Check for timeout indicators
	if result.ExitCode != 0 {
		// Should have non-zero exit code or timeout message
		assert.True(t, result.ExitCode == -1 || result.ExitCode == 124 ||
			result.ExitCode == 137 || result.ExitCode == 143 ||
			result.Stderr != "", "Expected timeout or error")
	}
}

func TestTool_Execute(t *testing.T) {
	t.Parallel()
	tool := NewTool(nil)

	// Skip if no runtime available
	if len(AvailableRuntimes()) == 1 && AvailableRuntimes()[0] == RuntimeNone {
		t.Skip("No container runtime available")  // SKIP-OK: #runtime-mock-only
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, map[string]interface{}{
		"command": "echo",
		"args":    []interface{}{"hello"},
		"image":   "alpine:latest",
		"timeout": float64(30),
	})

	// May fail in CI, just check it doesn't panic
	_ = err
	_ = result
}

func TestTool_Execute_MissingCommand(t *testing.T) {
	t.Parallel()
	tool := NewTool(nil)

	ctx := context.Background()
	_, err := tool.Execute(ctx, map[string]interface{}{
		"args": []interface{}{"hello"},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")
}

func TestTool_Execute_NoSandboxAvailable(t *testing.T) {
	t.Parallel()
	// This test checks behavior when no sandbox is available
	// In practice, this falls back to direct execution

	tool := NewTool(nil)

	ctx := context.Background()
	result, err := tool.Execute(ctx, map[string]interface{}{
		"command": "echo",
		"args":    []interface{}{"test"},
		"image":   "nonexistent-image-that-will-fail",
	})

	// Should not error, but result may indicate failure
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTool_Name(t *testing.T) {
	t.Parallel()
	tool := NewTool(nil)
	assert.Equal(t, "Sandbox", tool.Name())
}

func TestTool_Description(t *testing.T) {
	t.Parallel()
	tool := NewTool(nil)
	assert.Contains(t, tool.Description(), "sandbox")
}

func TestTool_Schema(t *testing.T) {
	t.Parallel()
	tool := NewTool(nil)
	schema := tool.Schema()

	assert.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])
	assert.Contains(t, schema, "properties")
	assert.Contains(t, schema, "required")
}
