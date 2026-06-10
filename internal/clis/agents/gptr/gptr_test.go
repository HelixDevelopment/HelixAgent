// Package gptr provides tests for the GPTR agent integration
package gptr

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	g := New()

	assert.NotNil(t, g)
	assert.NotNil(t, g.BaseIntegration)
	assert.NotNil(t, g.config)
	assert.NotNil(t, g.tasks)

	info := g.Info()
	assert.Equal(t, "GPTR", info.Name)
	assert.Equal(t, "GPTR", info.Vendor)
	assert.Contains(t, info.Capabilities, "task_runner")
	assert.Contains(t, info.Capabilities, "code_execution")
	assert.True(t, info.IsEnabled)
}

func TestGPTR_Initialize(t *testing.T) {
	t.Parallel()
	g := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Model:     "gpt-4-turbo",
		MaxTokens: 8192,
		Timeout:   120,
	}

	err := g.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4-turbo", g.config.Model)
	assert.Equal(t, 8192, g.config.MaxTokens)
	assert.Equal(t, 120, g.config.Timeout)
}

func TestGPTR_Execute(t *testing.T) {
	t.Parallel()
	g := New()
	ctx := context.Background()

	err := g.Initialize(ctx, &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		command   string
		params    map[string]interface{}
		wantErr   bool
		errMsg    string
		checkFunc func(t *testing.T, result interface{})
	}{
		{
			// Reconciled (§11.4.120): run USED to return the fabricated
			// "Result for: Process data" template + status "completed" with
			// wantErr:false — that codified BLUFF-001. GPTR has no headless LLM
			// runner wired; run now returns an HONEST error.
			name:    "run command (honest error — no runner wired)",
			command: "run",
			params:  map[string]interface{}{"prompt": "Process data"},
			wantErr: true,
			errMsg:  "refusing to fabricate",
		},
		{
			name:    "run without prompt",
			command: "run",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "prompt required",
		},
		{
			name:    "create_task command",
			command: "create_task",
			params:  map[string]interface{}{"name": "Task1", "prompt": "Do something"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "created", m["status"])
			},
		},
		{
			name:    "list_tasks command",
			command: "list_tasks",
			params:  map[string]interface{}{},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["tasks"])
			},
		},
		{
			name:    "get_result command",
			command: "get_result",
			params:  map[string]interface{}{"task_id": "nonexistent-task"},
			wantErr: true, // Task doesn't exist
			errMsg:  "task not found",
		},
		{
			name:    "unknown command",
			command: "unknown",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "unknown command: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := g.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestGPTR_ExecuteWithCreatedTask(t *testing.T) {
	t.Parallel()
	g := New()
	ctx := context.Background()

	err := g.Initialize(ctx, &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	// Create a task first
	result, err := g.Execute(ctx, "create_task", map[string]interface{}{
		"name":   "TestTask",
		"prompt": "Do something",
	})
	require.NoError(t, err)

	m := result.(map[string]interface{})
	task := m["task"].(Task)
	taskID := task.ID

	// Now test get_result
	t.Run("get_result for existing task", func(t *testing.T) {
		t.Parallel()
		result, err := g.Execute(ctx, "get_result", map[string]interface{}{
			"task_id": taskID,
		})
		require.NoError(t, err)
		m := result.(map[string]interface{})
		assert.NotNil(t, m["task"])
	})
}

func TestGPTR_IsAvailable(t *testing.T) {
	t.Parallel()
	g := New()
	// GPTR is always available
	assert.True(t, g.IsAvailable())
}

func TestTask(t *testing.T) {
	t.Parallel()
	task := Task{
		ID:     "task-1",
		Name:   "TestTask",
		Prompt: "Do something",
		Status: "created",
		Result: "",
		Tools:  []string{"tool1", "tool2"},
	}
	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "TestTask", task.Name)
	assert.Equal(t, "created", task.Status)
	assert.Len(t, task.Tools, 2)
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		Model:     "gpt-4",
		MaxTokens: 4096,
		Timeout:   60,
	}
	assert.Equal(t, "gpt-4", config.Model)
	assert.Equal(t, 4096, config.MaxTokens)
	assert.Equal(t, 60, config.Timeout)
}
