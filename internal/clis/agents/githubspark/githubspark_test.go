// Package githubspark provides tests for the GitHub Spark agent integration
package githubspark

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	g := New()

	assert.NotNil(t, g)
	assert.NotNil(t, g.BaseIntegration)
	assert.NotNil(t, g.config)
	assert.NotNil(t, g.sparks)

	info := g.Info()
	assert.Equal(t, "GitHub Spark", info.Name)
	assert.Equal(t, "GitHub", info.Vendor)
	assert.Contains(t, info.Capabilities, "micro_app_generation")
	assert.Contains(t, info.Capabilities, "github_integration")
	assert.True(t, info.IsEnabled)
}

func TestGitHubSpark_Initialize(t *testing.T) {
	t.Parallel()
	g := New()
	ctx := context.Background()

	// Check for GitHub token - skip if not available
	if g.config.GitHubToken == "" {
		t.Skip("GitHub token not configured - skipping test")  // SKIP-OK: #legacy-untriaged
	}

	config := &Config{
		GitHubToken:       "test-token",
		AutoPublish:       true,
		DefaultVisibility: "private",
	}

	err := g.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-token", g.config.GitHubToken)
	assert.True(t, g.config.AutoPublish)
	assert.Equal(t, "private", g.config.DefaultVisibility)
}

func TestGitHubSpark_Execute(t *testing.T) {
	t.Parallel()
	g := New()
	ctx := context.Background()

	err := g.Initialize(ctx, nil)
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
			name:    "create command",
			command: "create",
			params:  map[string]interface{}{"name": "MyApp", "description": "Test app", "template": "react"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "created", m["status"])
				assert.NotNil(t, m["spark"])
				assert.NotNil(t, m["files"])
			},
		},
		{
			name:    "create without name",
			command: "create",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "name required",
		},
		{
			name:    "edit command",
			command: "edit",
			params:  map[string]interface{}{"spark_id": "nonexistent", "prompt": "Add feature"},
			wantErr: true, // Spark doesn't exist
			errMsg:  "spark not found",
		},
		{
			name:    "publish command",
			command: "publish",
			params:  map[string]interface{}{"spark_id": "nonexistent"},
			wantErr: true,
			errMsg:  "spark not found",
		},
		{
			name:    "list command",
			command: "list",
			params:  map[string]interface{}{},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["sparks"])
				// Count may vary based on test execution order
				assert.NotNil(t, m["count"])
			},
		},
		{
			name:    "clone command",
			command: "clone",
			params:  map[string]interface{}{"spark_id": "nonexistent", "new_name": "MyApp2"},
			wantErr: true,
			errMsg:  "spark not found",
		},
		{
			name:    "share command",
			command: "share",
			params:  map[string]interface{}{"spark_id": "nonexistent"},
			wantErr: true,
			errMsg:  "spark not found",
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

func TestGitHubSpark_ExecuteWithCreatedSpark(t *testing.T) {
	t.Parallel()
	g := New()
	ctx := context.Background()

	err := g.Initialize(ctx, nil)
	require.NoError(t, err)

	// Create a spark first
	result, err := g.Execute(ctx, "create", map[string]interface{}{
		"name":        "TestApp",
		"description": "Test app",
	})
	require.NoError(t, err)

	m := result.(map[string]interface{})
	spark := m["spark"].(Spark)
	sparkID := spark.ID

	// Now test edit. §11.4.120-reconciled: GitHub Spark is web-only with no
	// headless CLI, so a prompt-driven edit MUST return an honest
	// no-headless-CLI error after the spark is located — NEVER a fabricated
	// change list (BLUFF-001). This case previously asserted status:"edited".
	t.Run("edit existing spark returns honest no-CLI error (not fabricated)", func(t *testing.T) {
		result, err := g.Execute(ctx, "edit", map[string]interface{}{
			"spark_id": sparkID,
			"prompt":   "Add feature",
		})
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no headless CLI")
	})

	// Test publish
	t.Run("publish existing spark", func(t *testing.T) {
		result, err := g.Execute(ctx, "publish", map[string]interface{}{
			"spark_id": sparkID,
		})
		require.NoError(t, err)
		m := result.(map[string]interface{})
		assert.Equal(t, "published", m["status"])
	})

	// Test share
	t.Run("share existing spark", func(t *testing.T) {
		result, err := g.Execute(ctx, "share", map[string]interface{}{
			"spark_id": sparkID,
		})
		require.NoError(t, err)
		m := result.(map[string]interface{})
		assert.NotEmpty(t, m["share_url"])
	})
}

func TestGitHubSpark_IsAvailable(t *testing.T) {
	t.Parallel()
	g := New()
	assert.False(t, g.IsAvailable())

	g.config.GitHubToken = "test-token"
	assert.True(t, g.IsAvailable())
}

func TestSpark(t *testing.T) {
	t.Parallel()
	spark := Spark{
		ID:          "spark-1",
		Name:        "TestApp",
		Description: "Test application",
		Repository:  "github.com/user/testapp",
		URL:         "https://spark.github.com/testapp",
		Status:      "created",
	}
	assert.Equal(t, "spark-1", spark.ID)
	assert.Equal(t, "TestApp", spark.Name)
	assert.Equal(t, "created", spark.Status)
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		GitHubToken:       "token",
		AutoPublish:       false,
		DefaultVisibility: "public",
	}
	assert.Equal(t, "token", config.GitHubToken)
	assert.Equal(t, "public", config.DefaultVisibility)
}
