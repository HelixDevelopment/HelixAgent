// Package lovable provides tests for the Lovable agent integration
package lovable

import (
	"context"
	"errors"
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
		// Reconciled (§11.4.120): create_app/add_feature/connect_database USED to
		// return fabricated project URLs, file/component lists, and success
		// statuses with wantErr:false — that codified BLUFF-001. Lovable is a
		// hosted web-only builder with no wired client; these now return an
		// HONEST error.
		{
			name:    "create_app command (honest error — hosted-only)",
			command: "create_app",
			params:  map[string]interface{}{"name": "MyApp", "description": "Test app"},
			wantErr: true,
			errMsg:  "refusing to fabricate",
		},
		{
			name:    "create_app without name",
			command: "create_app",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "name required",
		},
		{
			name:    "edit command (honest error — hosted-only)",
			command: "edit",
			params:  map[string]interface{}{"project_id": "proj-1", "prompt": "Change color"},
			wantErr: true,
			errMsg:  "refusing to fabricate",
		},
		{
			name:    "deploy command (honest error — hosted-only)",
			command: "deploy",
			params:  map[string]interface{}{"project_id": "proj-1"},
			wantErr: true,
			errMsg:  "refusing to fabricate",
		},
		{
			name:    "add_feature command (honest error — hosted-only)",
			command: "add_feature",
			params:  map[string]interface{}{"project_id": "proj-1", "feature": "auth"},
			wantErr: true,
			errMsg:  "refusing to fabricate",
		},
		{
			name:    "connect_database command (honest error — hosted-only)",
			command: "connect_database",
			params:  map[string]interface{}{"project_id": "proj-1", "type": "mysql"},
			wantErr: true,
			errMsg:  "refusing to fabricate",
		},
		{
			// list_projects reads REAL local state (the project registry) — stays
			// non-error, returns the real (empty) list.
			name:    "list_projects command (real local registry)",
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
			name:    "export_code command (honest error — hosted-only)",
			command: "export_code",
			params:  map[string]interface{}{"project_id": "proj-1"},
			wantErr: true,
			errMsg:  "refusing to fabricate",
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

// Reconciled (§11.4.120): this test USED to assert that create_app →
// edit/deploy/export_code all succeed with fabricated "created"/"edited"/
// "deployed"/"exported" statuses — the ENTIRE flow codified BLUFF-001 (no real
// Lovable operation ever ran). Lovable is a hosted web-only builder with no
// wired client; the flow now asserts every builder command returns the honest
// hosted-only error rather than fabricating a successful build pipeline.
func TestLovable_ExecuteWithCreatedProject(t *testing.T) {
	t.Parallel()
	l := New()
	ctx := context.Background()

	require.NoError(t, l.Initialize(ctx, nil))

	// create_app must NOT fabricate a project — honest hosted-only error.
	_, err := l.Execute(ctx, "create_app", map[string]interface{}{
		"name":        "TestApp",
		"description": "Test application",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to fabricate")

	for _, cmd := range []struct {
		name   string
		params map[string]interface{}
	}{
		{"edit", map[string]interface{}{"project_id": "proj-x", "prompt": "Change colors"}},
		{"deploy", map[string]interface{}{"project_id": "proj-x"}},
		{"export_code", map[string]interface{}{"project_id": "proj-x"}},
	} {
		t.Run(cmd.name+" is honest error", func(t *testing.T) {
			_, err := l.Execute(ctx, cmd.name, cmd.params)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrHostedOnly),
				"%s must wrap ErrHostedOnly, got: %v", cmd.name, err)
		})
	}
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
