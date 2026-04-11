package templates

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ContextTemplate Tests

func TestContextTemplate_Validate(t *testing.T) {
	tests := []struct {
		name     string
		template ContextTemplate
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid template",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					ID:   "test-template",
					Name: "Test Template",
				},
			},
			wantErr: false,
		},
		{
			name: "missing api_version",
			template: ContextTemplate{
				Kind: "ContextTemplate",
				Metadata: TemplateMetadata{
					ID:   "test-template",
					Name: "Test Template",
				},
			},
			wantErr: true,
			errMsg:  "api_version is required",
		},
		{
			name: "missing metadata.id",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					Name: "Test Template",
				},
			},
			wantErr: true,
			errMsg:  "metadata.id is required",
		},
		{
			name: "missing metadata.name",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					ID: "test-template",
				},
			},
			wantErr: true,
			errMsg:  "metadata.name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.template.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContextTemplate_GetVariable(t *testing.T) {
	template := ContextTemplate{
		Metadata: TemplateMetadata{
			ID:   "test",
			Name: "Test",
		},
		Spec: TemplateSpec{
			Variables: []VariableDef{
				{Name: "var1", Description: "Variable 1", Required: true},
				{Name: "var2", Description: "Variable 2", Required: false, Default: "default2"},
				{Name: "var3", Description: "Variable 3", Required: false},
			},
		},
	}

	tests := []struct {
		name         string
		varName      string
		wantNil      bool
		wantName     string
		wantRequired bool
	}{
		{
			name:         "existing required variable",
			varName:      "var1",
			wantNil:      false,
			wantName:     "var1",
			wantRequired: true,
		},
		{
			name:         "existing optional variable with default",
			varName:      "var2",
			wantNil:      false,
			wantName:     "var2",
			wantRequired: false,
		},
		{
			name:    "non-existent variable",
			varName: "nonexistent",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := template.GetVariable(tt.varName)
			if tt.wantNil {
				assert.Nil(t, v)
			} else {
				require.NotNil(t, v)
				assert.Equal(t, tt.wantName, v.Name)
				assert.Equal(t, tt.wantRequired, v.Required)
			}
		})
	}
}

func TestContextTemplate_GetPrompt(t *testing.T) {
	template := ContextTemplate{
		Metadata: TemplateMetadata{
			ID:   "test",
			Name: "Test",
		},
		Spec: TemplateSpec{
			Prompts: []PromptDef{
				{Name: "prompt1", Description: "Prompt 1", Template: "Template 1"},
				{Name: "prompt2", Description: "Prompt 2", Template: "Template 2"},
			},
		},
	}

	tests := []struct {
		name       string
		promptName string
		wantNil    bool
		wantName   string
	}{
		{
			name:       "existing prompt",
			promptName: "prompt1",
			wantNil:    false,
			wantName:   "prompt1",
		},
		{
			name:       "non-existent prompt",
			promptName: "nonexistent",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := template.GetPrompt(tt.promptName)
			if tt.wantNil {
				assert.Nil(t, p)
			} else {
				require.NotNil(t, p)
				assert.Equal(t, tt.wantName, p.Name)
			}
		})
	}
}

// TemplateManager Tests

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	assert.NotEmpty(t, config.TemplatesDir)
	assert.Contains(t, config.TemplatesDir, ".helixagent")
	assert.Contains(t, config.TemplatesDir, "templates")
	assert.Equal(t, 100, config.MaxTemplates)
}

func TestNewManager(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)
	require.NotNil(t, manager)

	assert.Equal(t, tempDir, manager.templatesDir)
	assert.NotNil(t, manager.templates)

	// Verify built-in templates were loaded
	assert.NotEmpty(t, manager.templates)

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestNewManager_CreateDirectory(t *testing.T) {
	// Use a non-existent subdirectory
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "subdir", "templates")

	config := ManagerConfig{
		TemplatesDir: nonExistentDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// Verify directory was created
	_, err = os.Stat(nonExistentDir)
	assert.NoError(t, err)

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestManager_Create(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	tests := []struct {
		name        string
		template    ContextTemplate
		wantErr     bool
		errContains string
	}{
		{
			name: "valid new template",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					ID:   "custom-template",
					Name: "Custom Template",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid template - missing name",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					ID: "invalid-template",
				},
			},
			wantErr:     true,
			errContains: "metadata.name is required",
		},
		{
			name: "duplicate template ID",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					ID:   "custom-template", // Same as first test
					Name: "Duplicate Template",
				},
			},
			wantErr:     true,
			errContains: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.Create(&tt.template)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
				// Verify timestamps were set
				assert.False(t, tt.template.Metadata.CreatedAt.IsZero())
				assert.False(t, tt.template.Metadata.UpdatedAt.IsZero())
			}
		})
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestManager_Get(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Add a test template
	testTemplate := &ContextTemplate{
		APIVersion: "v1",
		Kind:       "ContextTemplate",
		Metadata: TemplateMetadata{
			ID:   "test-get",
			Name: "Test Get Template",
		},
	}
	err = manager.Create(testTemplate)
	require.NoError(t, err)

	tests := []struct {
		name       string
		templateID string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "existing template",
			templateID: "test-get",
			wantErr:    false,
		},
		{
			name:       "non-existent template",
			templateID: "non-existent",
			wantErr:    true,
			errMsg:     "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template, err := manager.Get(tt.templateID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, template)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, template)
				assert.Equal(t, tt.templateID, template.Metadata.ID)
			}
		})
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestManager_GetTemplate(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Add a test template
	testTemplate := &ContextTemplate{
		APIVersion: "v1",
		Kind:       "ContextTemplate",
		Metadata: TemplateMetadata{
			ID:   "test-alias",
			Name: "Test Alias Template",
		},
	}
	err = manager.Create(testTemplate)
	require.NoError(t, err)

	// Test that GetTemplate is an alias for Get
	template, err := manager.GetTemplate("test-alias")
	assert.NoError(t, err)
	assert.NotNil(t, template)
	assert.Equal(t, "test-alias", template.Metadata.ID)

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestManager_Update(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Create a test template
	testTemplate := &ContextTemplate{
		APIVersion: "v1",
		Kind:       "ContextTemplate",
		Metadata: TemplateMetadata{
			ID:          "test-update",
			Name:        "Original Name",
			Description: "Original Description",
		},
	}
	err = manager.Create(testTemplate)
	require.NoError(t, err)

	originalUpdatedAt := testTemplate.Metadata.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure time difference

	tests := []struct {
		name     string
		template ContextTemplate
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid update",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					ID:          "test-update",
					Name:        "Updated Name",
					Description: "Updated Description",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid template - missing name",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					ID: "test-update",
				},
			},
			wantErr: true,
			errMsg:  "metadata.name is required",
		},
		{
			name: "non-existent template",
			template: ContextTemplate{
				APIVersion: "v1",
				Kind:       "ContextTemplate",
				Metadata: TemplateMetadata{
					ID:   "non-existent",
					Name: "Non Existent",
				},
			},
			wantErr: true,
			errMsg:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.Update(&tt.template)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				// Verify UpdatedAt was updated
				assert.True(t, tt.template.Metadata.UpdatedAt.After(originalUpdatedAt))

				// Verify changes were persisted
				updated, _ := manager.Get(tt.template.Metadata.ID)
				assert.Equal(t, "Updated Name", updated.Metadata.Name)
			}
		})
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestManager_Delete(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Create a test template
	testTemplate := &ContextTemplate{
		APIVersion: "v1",
		Kind:       "ContextTemplate",
		Metadata: TemplateMetadata{
			ID:   "test-delete",
			Name: "Test Delete Template",
		},
	}
	err = manager.Create(testTemplate)
	require.NoError(t, err)

	// Verify template exists
	_, err = manager.Get("test-delete")
	assert.NoError(t, err)

	// Delete the template
	err = manager.Delete("test-delete")
	assert.NoError(t, err)

	// Verify template no longer exists
	_, err = manager.Get("test-delete")
	assert.Error(t, err)

	// Verify file was deleted
	path := filepath.Join(tempDir, "test-delete.yaml")
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))

	// Test deleting non-existent template
	err = manager.Delete("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestManager_List(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Create some test templates
	templates := []*ContextTemplate{
		{
			APIVersion: "v1",
			Kind:       "ContextTemplate",
			Metadata: TemplateMetadata{
				ID:   "template-1",
				Name: "Template 1",
			},
		},
		{
			APIVersion: "v1",
			Kind:       "ContextTemplate",
			Metadata: TemplateMetadata{
				ID:   "template-2",
				Name: "Template 2",
			},
		},
	}

	for _, tmpl := range templates {
		err := manager.Create(tmpl)
		require.NoError(t, err)
	}

	// List templates
	list := manager.List()

	// Should include built-in templates + our templates
	assert.GreaterOrEqual(t, len(list), 2)

	// Verify our templates are in the list
	ids := make(map[string]bool)
	for _, tmpl := range list {
		ids[tmpl.Metadata.ID] = true
	}
	assert.True(t, ids["template-1"])
	assert.True(t, ids["template-2"])

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestManager_ListByTag(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Create templates with different tags
	templates := []*ContextTemplate{
		{
			APIVersion: "v1",
			Kind:       "ContextTemplate",
			Metadata: TemplateMetadata{
				ID:   "tagged-1",
				Name: "Tagged 1",
				Tags: []string{"go", "backend"},
			},
		},
		{
			APIVersion: "v1",
			Kind:       "ContextTemplate",
			Metadata: TemplateMetadata{
				ID:   "tagged-2",
				Name: "Tagged 2",
				Tags: []string{"python", "backend"},
			},
		},
		{
			APIVersion: "v1",
			Kind:       "ContextTemplate",
			Metadata: TemplateMetadata{
				ID:   "no-tags",
				Name: "No Tags",
			},
		},
	}

	for _, tmpl := range templates {
		err := manager.Create(tmpl)
		require.NoError(t, err)
	}

	tests := []struct {
		name    string
		tag     string
		wantIDs []string
	}{
		{
			name:    "filter by go",
			tag:     "go",
			wantIDs: []string{"tagged-1"},
		},
		{
			name:    "filter by backend (case insensitive)",
			tag:     "BACKEND",
			wantIDs: []string{"tagged-1", "tagged-2"},
		},
		{
			name:    "filter by non-existent tag",
			tag:     "nonexistent",
			wantIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := manager.ListByTag(tt.tag)
			assert.Len(t, list, len(tt.wantIDs))

			ids := make(map[string]bool)
			for _, tmpl := range list {
				ids[tmpl.Metadata.ID] = true
			}
			for _, wantID := range tt.wantIDs {
				assert.True(t, ids[wantID], "expected template %s not found", wantID)
			}
		})
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestManager_ApplyTemplate(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Create a template with variables
	testTemplate := &ContextTemplate{
		APIVersion: "v1",
		Kind:       "ContextTemplate",
		Metadata: TemplateMetadata{
			ID:   "test-apply",
			Name: "Test Apply Template",
		},
		Spec: TemplateSpec{
			Instructions: "Hello {{name}}!",
			Variables: []VariableDef{
				{Name: "name", Required: true},
			},
		},
	}
	err = manager.Create(testTemplate)
	require.NoError(t, err)

	tests := []struct {
		name        string
		templateID  string
		vars        map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid application",
			templateID: "test-apply",
			vars:       map[string]string{"name": "World"},
			wantErr:    false,
		},
		{
			name:        "non-existent template",
			templateID:  "non-existent",
			vars:        map[string]string{},
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := manager.ApplyTemplate(tt.templateID, tt.vars)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ctx)
			}
		})
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

// Built-in template tests

func TestBuiltInTemplates(t *testing.T) {
	tempDir := t.TempDir()
	config := ManagerConfig{
		TemplatesDir: tempDir,
		MaxTemplates: 10,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Test that built-in templates exist
	builtInIDs := []string{"onboarding", "bug-fix", "code-review", "feature-dev"}

	for _, id := range builtInIDs {
		t.Run(id, func(t *testing.T) {
			template, err := manager.Get(id)
			assert.NoError(t, err)
			assert.NotNil(t, template)
			assert.NotEmpty(t, template.Metadata.Name)
			assert.NotEmpty(t, template.Metadata.Description)
			assert.NotEmpty(t, template.Spec.Instructions)
		})
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

// Type alias test

func TestManager_Alias(t *testing.T) {
	// Ensure Manager is an alias for TemplateManager
	// Just verify the types are compatible
	config := DefaultManagerConfig()
	tempDir := t.TempDir()
	config.TemplatesDir = tempDir

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Manager should work as expected
	assert.NotNil(t, manager)

	// Cleanup
	os.RemoveAll(tempDir)
}
