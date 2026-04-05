// Package promptfoo provides tests for Promptfoo agent integration
package promptfoo

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPromptfoo(t *testing.T) {
	t.Parallel()
	p := New()
	require.NotNil(t, p)
	
	info := p.Info()
	assert.Equal(t, agents.TypePromptfoo, info.Type)
	assert.Equal(t, "Promptfoo", info.Name)
	assert.Equal(t, "Promptfoo", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestPromptfooInitialize(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()
	
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		OutputFormat:   "yaml",
		MaxConcurrency: 8,
	}
	
	err := p.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "yaml", p.config.OutputFormat)
	assert.Equal(t, 8, p.config.MaxConcurrency)
}

func TestPromptfooInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()
	
	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "json", p.config.OutputFormat) // Default value
	assert.Equal(t, 4, p.config.MaxConcurrency)     // Default value
}

func TestPromptfooStartStop(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()
	
	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	
	err = p.Start(ctx)
	require.NoError(t, err)
	assert.True(t, p.IsStarted())
	
	err = p.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, p.IsStarted())
}

func TestPromptfooExecute(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()
	
	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "init command",
			command: "init",
			params: map[string]interface{}{
				"name": "test-project",
			},
			wantErr: false,
		},
		{
			name:    "init with default name",
			command: "init",
			params:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "eval command",
			command: "eval",
			params: map[string]interface{}{
				"config": "custom-config.yaml",
			},
			wantErr: false,
		},
		{
			name:    "create_suite command",
			command: "create_suite",
			params: map[string]interface{}{
				"name": "My Suite",
			},
			wantErr: false,
		},
		{
			name:    "create_suite without name fails",
			command: "create_suite",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "add_test command",
			command: "add_test",
			params: map[string]interface{}{
				"suite_id": "suite-1",
				"vars":     map[string]interface{}{"var1": "value1"},
				"expected": "result",
			},
			wantErr: false,
		},
		{
			name:    "add_test without suite_id fails",
			command: "add_test",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "run_suite command",
			command: "run_suite",
			params: map[string]interface{}{
				"suite_id": "suite-1",
			},
			wantErr: false,
		},
		{
			name:    "run_suite without suite_id fails",
			command: "run_suite",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "list_suites command",
			command: "list_suites",
			params:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "view command",
			command: "view",
			params:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "unknown command",
			command: "unknown",
			params:  map[string]interface{}{},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result, err := p.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestPromptfooCapabilities(t *testing.T) {
	t.Parallel()
	p := New()
	info := p.Info()
	
	expectedCaps := []string{"llm_testing", "prompt_evaluation", "regression_testing", "red_teaming", "multi_provider", "benchmarking"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestPromptfooIsAvailable(t *testing.T) {
	t.Parallel()
	p := New()
	assert.True(t, p.IsAvailable())
}

func TestPromptfooInitResult(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()
	
	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	
	result, err := p.Execute(ctx, "init", map[string]interface{}{
		"name": "my-project",
	})
	require.NoError(t, err)
	
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "my-project", resultMap["name"])
	assert.Equal(t, "initialized", resultMap["status"])
}

func TestPromptfooEvalResult(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()
	
	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	
	result, err := p.Execute(ctx, "eval", map[string]interface{}{
		"config": "promptfooconfig.yaml",
	})
	require.NoError(t, err)
	
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "promptfooconfig.yaml", resultMap["config"])
	assert.NotNil(t, resultMap["tests_run"])
	assert.NotNil(t, resultMap["passed"])
	assert.NotNil(t, resultMap["failed"])
	assert.NotNil(t, resultMap["score"])
}

func TestPromptfooCreateSuiteResult(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()
	
	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	
	result, err := p.Execute(ctx, "create_suite", map[string]interface{}{
		"name":        "Test Suite",
		"description": "A test suite",
	})
	require.NoError(t, err)
	
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, resultMap["suite"])
	assert.Equal(t, "created", resultMap["status"])
}
