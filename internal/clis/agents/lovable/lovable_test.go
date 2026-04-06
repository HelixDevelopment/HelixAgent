// Package lovable provides tests for the Lovable agent integration
package lovable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	l := New()

	assert.NotNil(t, l)
	assert.NotNil(t, l.BaseIntegration)
	assert.NotNil(t, l.config)
	assert.NotNil(t, l.projects)

	info := l.Info()
	assert.Equal(t, "Lovable", info.Name)
	assert.Equal(t, "Lovable", info.Vendor)
	assert.Contains(t, info.Capabilities, "visual_editing")
	assert.Contains(t, info.Capabilities, "fullstack_generation")
	assert.True(t, info.IsEnabled)
}

func TestLovable_Initialize(t *testing.T) {
	t.Parallel()
	l := New()
	ctx := context.Background()

	config := &Config{
		APIKey:       "test-api-key",
		DefaultStack: "vue-node-mysql",
		AutoDeploy:   true,
	}

	err := l.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", l.config.APIKey)
	assert.Equal(t, "vue-node-mysql", l.config.DefaultStack)
	assert.True(t, l.config.AutoDeploy)
}

func TestLovable_Execute(t *testing.T) {
	t.Parallel()
	l := New()
	ctx := context.Background()

	err := l.Initialize(ctx, nil)
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
			name:    "create_app command",
			command: "create_app",
			params:  map[string]interface{}{"name": "MyApp", "description": "Test app"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "created", m["status"])
				assert.NotNil(t, m["project"])
				assert.NotNil(t, m["files"])
			},
		},
		{
			name:    "create_app without name",
			command: "create_app",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "name required",
		},
		{
			name:    "edit command",
			command: "edit",
			params:  map[string]interface{}{"project_id": "proj-1", "prompt": "Change color"},
			wantErr: true, // Project doesn't exist
			errMsg:  "project not found",
		},
		{
			name:    "deploy command",
			command: "deploy",
			params:  map[string]interface{}{"project_id": "proj-1"},
			wantErr: true,
			errMsg:  "project not found",
		},
		{
			name:    "add_feature command",
			command: "add_feature",
			params:  map[string]interface{}{"project_id": "proj-1", "feature": "auth"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "auth", m["feature"])
				assert.Equal(t, "added", m["status"])
			},
		},
		{
			name:    "connect_database command",
			command: "connect_database",
			params:  map[string]interface{}{"project_id": "proj-1", "type": "mysql"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "mysql", m["database"])
				assert.Equal(t, "connected", m["status"])
			},
		},
		{
			name:    "list_projects command",
			command: "list_projects",
			params:  map[string]interface{}{},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["projects"])
			},
		},
		{
			name:    "export_code command",
			command: "export_code",
			params:  map[string]interface{}{"project_id": "proj-1"},
			wantErr: true,
			errMsg:  "project not found",
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
			result, err := l.Execute(ctx, tt.command, tt.params)
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

func TestLovable_ExecuteWithCreatedProject(t *testing.T) {
	t.Parallel()
	l := New()
	ctx := context.Background()

	err := l.Initialize(ctx, nil)
	require.NoError(t, err)

	// Create a project first
	result, err := l.Execute(ctx, "create_app", map[string]interface{}{
		"name":        "TestApp",
		"description": "Test application",
	})
	require.NoError(t, err)

	m := result.(map[string]interface{})
	project := m["project"].(Project)
	projectID := project.ID

	// Now test edit
	t.Run("edit existing project", func(t *testing.T) {
		result, err := l.Execute(ctx, "edit", map[string]interface{}{
			"project_id": projectID,
			"prompt":     "Change colors",
		})
		require.NoError(t, err)
		m := result.(map[string]interface{})
		assert.Equal(t, "edited", m["status"])
	})

	// Test deploy
	t.Run("deploy existing project", func(t *testing.T) {
		result, err := l.Execute(ctx, "deploy", map[string]interface{}{
			"project_id": projectID,
		})
		require.NoError(t, err)
		m := result.(map[string]interface{})
		assert.Equal(t, "deployed", m["status"])
	})

	// Test export_code
	t.Run("export_code for existing project", func(t *testing.T) {
		result, err := l.Execute(ctx, "export_code", map[string]interface{}{
			"project_id": projectID,
		})
		require.NoError(t, err)
		m := result.(map[string]interface{})
		assert.Equal(t, "exported", m["status"])
	})
}

func TestLovable_IsAvailable(t *testing.T) {
	t.Parallel()
	l := New()
	assert.False(t, l.IsAvailable())

	l.config.APIKey = "test-key"
	assert.True(t, l.IsAvailable())
}

func TestProject(t *testing.T) {
	t.Parallel()
	project := Project{
		ID:          "proj-1",
		Name:        "TestApp",
		Description: "Test application",
		Stack:       "react-node-postgres",
		Status:      "created",
		URL:         "https://lovable.dev/p/testapp",
	}
	assert.Equal(t, "proj-1", project.ID)
	assert.Equal(t, "TestApp", project.Name)
	assert.Equal(t, "created", project.Status)
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		APIKey:       "key",
		DefaultStack: "svelte-node-mongodb",
		AutoDeploy:   false,
	}
	assert.Equal(t, "key", config.APIKey)
	assert.Equal(t, "svelte-node-mongodb", config.DefaultStack)
	assert.False(t, config.AutoDeploy)
}
