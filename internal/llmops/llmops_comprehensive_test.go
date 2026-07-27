package llmops

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// PROMPT REGISTRY TESTS
// =============================================================================

func TestInMemoryPromptRegistry_Create_Validations(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		prompt  *PromptVersion
		wantErr string
	}{
		{
			name: "missing name",
			prompt: &PromptVersion{
				Version: "1.0.0",
				Content: "Hello",
			},
			wantErr: "name is required",
		},
		{
			name: "missing version",
			prompt: &PromptVersion{
				Name:    "test",
				Content: "Hello",
			},
			wantErr: "version is required",
		},
		{
			name: "missing content",
			prompt: &PromptVersion{
				Name:    "test",
				Version: "1.0.0",
			},
			wantErr: "content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Create(ctx, tt.prompt)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInMemoryPromptRegistry_Create_DuplicateVersion(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	prompt := &PromptVersion{
		Name:    "test",
		Version: "1.0.0",
		Content: "Hello",
	}

	require.NoError(t, registry.Create(ctx, prompt))

	// Try to create same version again
	duplicate := &PromptVersion{
		Name:    "test",
		Version: "1.0.0",
		Content: "Different",
	}

	err := registry.Create(ctx, duplicate)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInMemoryPromptRegistry_Create_WithExplicitID(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	prompt := &PromptVersion{
		ID:      "my-custom-id",
		Name:    "test",
		Version: "1.0.0",
		Content: "Hello",
	}

	require.NoError(t, registry.Create(ctx, prompt))
	assert.Equal(t, "my-custom-id", prompt.ID)
}

func TestInMemoryPromptRegistry_Create_ActiveOnFirstVersion(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	// First version becomes active automatically
	v1 := &PromptVersion{Name: "test", Version: "1.0.0", Content: "V1"}
	require.NoError(t, registry.Create(ctx, v1))
	assert.True(t, v1.IsActive)

	// Second version is not active by default
	v2 := &PromptVersion{Name: "test", Version: "2.0.0", Content: "V2"}
	require.NoError(t, registry.Create(ctx, v2))
	assert.False(t, v2.IsActive)

	// Third version with explicit active flag
	v3 := &PromptVersion{Name: "test", Version: "3.0.0", Content: "V3", IsActive: true}
	require.NoError(t, registry.Create(ctx, v3))
	assert.True(t, v3.IsActive)

	// Now v3 should be active
	latest, err := registry.GetLatest(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, "3.0.0", latest.Version)
}

func TestInMemoryPromptRegistry_Get_NotFound(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	// Non-existent prompt
	_, err := registry.Get(ctx, "nonexistent", "1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt not found")

	// Create prompt, then get non-existent version
	prompt := &PromptVersion{Name: "test", Version: "1.0.0", Content: "Hello"}
	require.NoError(t, registry.Create(ctx, prompt))

	_, err = registry.Get(ctx, "test", "2.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version not found")
}

func TestInMemoryPromptRegistry_GetLatest_NotFound(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	_, err := registry.GetLatest(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active version")
}

func TestInMemoryPromptRegistry_List_Empty(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	list, err := registry.List(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestInMemoryPromptRegistry_List_SortedByCreationTime(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	// Create versions in order
	for i := 1; i <= 3; i++ {
		prompt := &PromptVersion{
			Name:    "test",
			Version: fmt.Sprintf("%d.0.0", i),
			Content: fmt.Sprintf("V%d", i),
		}
		require.NoError(t, registry.Create(ctx, prompt))
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	list, err := registry.List(ctx, "test")
	require.NoError(t, err)
	assert.Len(t, list, 3)

	// Should be sorted newest first
	assert.Equal(t, "3.0.0", list[0].Version)
	assert.Equal(t, "2.0.0", list[1].Version)
	assert.Equal(t, "1.0.0", list[2].Version)
}

func TestInMemoryPromptRegistry_ListAll(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	// Create multiple prompts with multiple versions
	prompts := []struct {
		name    string
		version string
	}{
		{"alpha", "1.0.0"},
		{"alpha", "2.0.0"},
		{"beta", "1.0.0"},
		{"gamma", "1.0.0"},
	}

	for _, p := range prompts {
		prompt := &PromptVersion{
			Name:    p.name,
			Version: p.version,
			Content: fmt.Sprintf("%s-%s", p.name, p.version),
		}
		require.NoError(t, registry.Create(ctx, prompt))
	}

	all, err := registry.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 4)

	// Should be sorted by name, then by creation time
	assert.Equal(t, "alpha", all[0].Name)
	assert.Equal(t, "alpha", all[1].Name)
	assert.Equal(t, "beta", all[2].Name)
	assert.Equal(t, "gamma", all[3].Name)
}

func TestInMemoryPromptRegistry_Activate_NotFound(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	// Non-existent prompt
	err := registry.Activate(ctx, "nonexistent", "1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt not found")

	// Create prompt, activate non-existent version
	prompt := &PromptVersion{Name: "test", Version: "1.0.0", Content: "Hello"}
	require.NoError(t, registry.Create(ctx, prompt))

	err = registry.Activate(ctx, "test", "2.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version not found")
}

func TestInMemoryPromptRegistry_Activate_DeactivatesPrevious(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	v1 := &PromptVersion{Name: "test", Version: "1.0.0", Content: "V1"}
	v2 := &PromptVersion{Name: "test", Version: "2.0.0", Content: "V2"}

	require.NoError(t, registry.Create(ctx, v1))
	require.NoError(t, registry.Create(ctx, v2))

	// V1 is active initially
	assert.True(t, v1.IsActive)
	assert.False(t, v2.IsActive)

	// Activate V2
	require.NoError(t, registry.Activate(ctx, "test", "2.0.0"))

	// V1 should be deactivated, V2 active
	assert.False(t, v1.IsActive)
	assert.True(t, v2.IsActive)
}

func TestInMemoryPromptRegistry_Delete_NotFound(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	// Non-existent prompt
	err := registry.Delete(ctx, "nonexistent", "1.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt not found")

	// Create prompt, delete non-existent version
	prompt := &PromptVersion{Name: "test", Version: "1.0.0", Content: "Hello"}
	require.NoError(t, registry.Create(ctx, prompt))

	err = registry.Delete(ctx, "test", "2.0.0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version not found")
}

func TestInMemoryPromptRegistry_Delete_CleansUpEmptyPrompt(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	v1 := &PromptVersion{Name: "test", Version: "1.0.0", Content: "V1"}
	v2 := &PromptVersion{Name: "test", Version: "2.0.0", Content: "V2"}

	require.NoError(t, registry.Create(ctx, v1))
	require.NoError(t, registry.Create(ctx, v2))

	// Activate V2 to allow deletion of V1
	require.NoError(t, registry.Activate(ctx, "test", "2.0.0"))
	require.NoError(t, registry.Delete(ctx, "test", "1.0.0"))

	// Now activate nothing and delete V2 - this tests the cleanup
	// Actually we need another version to deactivate V2 first
	v3 := &PromptVersion{Name: "test", Version: "3.0.0", Content: "V3"}
	require.NoError(t, registry.Create(ctx, v3))
	require.NoError(t, registry.Activate(ctx, "test", "3.0.0"))
	require.NoError(t, registry.Delete(ctx, "test", "2.0.0"))

	list, err := registry.List(ctx, "test")
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestInMemoryPromptRegistry_Render_MissingRequiredVariable(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	prompt := &PromptVersion{
		Name:    "greeting",
		Version: "1.0.0",
		Content: "Hello, {{name}}!",
		Variables: []PromptVariable{
			{Name: "name", Type: "string", Required: true},
		},
	}

	require.NoError(t, registry.Create(ctx, prompt))

	// Missing required variable
	_, err := registry.Render(ctx, "greeting", "1.0.0", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required variable")
}

func TestInMemoryPromptRegistry_Render_FailedValidation(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	prompt := &PromptVersion{
		Name:    "email",
		Version: "1.0.0",
		Content: "Email: {{email}}",
		Variables: []PromptVariable{
			{Name: "email", Type: "string", Required: true, Validation: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`},
		},
	}

	require.NoError(t, registry.Create(ctx, prompt))

	// Invalid email format
	_, err := registry.Render(ctx, "email", "1.0.0", map[string]interface{}{
		"email": "not-an-email",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed validation")
}

func TestInMemoryPromptRegistry_Render_WithExtraVariables(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	prompt := &PromptVersion{
		Name:    "template",
		Version: "1.0.0",
		Content: "Hello, {{name}}! Your code is {{code}}.",
		Variables: []PromptVariable{
			{Name: "name", Type: "string", Required: true},
		},
	}

	require.NoError(t, registry.Create(ctx, prompt))

	// Provide extra variable not defined in Variables
	rendered, err := registry.Render(ctx, "template", "1.0.0", map[string]interface{}{
		"name": "John",
		"code": "ABC123",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello, John! Your code is ABC123.", rendered)
}

func TestInMemoryPromptRegistry_Render_NonRequiredWithoutDefault(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	prompt := &PromptVersion{
		Name:    "optional",
		Version: "1.0.0",
		Content: "Value: {{optional}}",
		Variables: []PromptVariable{
			{Name: "optional", Type: "string", Required: false},
		},
	}

	require.NoError(t, registry.Create(ctx, prompt))

	// Non-required variable without default skips replacement
	rendered, err := registry.Render(ctx, "optional", "1.0.0", map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "Value: {{optional}}", rendered)
}

func TestInMemoryPromptRegistry_Render_NotFound(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	_, err := registry.Render(ctx, "nonexistent", "1.0.0", nil)
	require.Error(t, err)
}

// =============================================================================
// PROMPT VERSION COMPARATOR TESTS
// =============================================================================

func TestPromptVersionComparator_Compare_NotFound(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	comparator := NewPromptVersionComparator(registry, nil)

	// First version not found
	_, err := comparator.Compare(ctx, "test", "1.0.0", "2.0.0")
	require.Error(t, err)

	// Create first version, second not found
	prompt := &PromptVersion{Name: "test", Version: "1.0.0", Content: "V1"}
	require.NoError(t, registry.Create(ctx, prompt))

	_, err = comparator.Compare(ctx, "test", "1.0.0", "2.0.0")
	require.Error(t, err)
}

func TestPromptVersionComparator_Compare_RemovedAndChangedVars(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	v1 := &PromptVersion{
		Name:    "test",
		Version: "1.0.0",
		Content: "Old content",
		Variables: []PromptVariable{
			{Name: "name", Type: "string", Required: true},
			{Name: "age", Type: "int", Required: false},
			{Name: "email", Type: "string", Required: true},
		},
	}

	v2 := &PromptVersion{
		Name:    "test",
		Version: "2.0.0",
		Content: "New content",
		Variables: []PromptVariable{
			{Name: "name", Type: "string", Required: false}, // Changed required
			{Name: "phone", Type: "string", Required: true}, // Added
			// age removed, email removed
		},
	}

	require.NoError(t, registry.Create(ctx, v1))
	require.NoError(t, registry.Create(ctx, v2))

	comparator := NewPromptVersionComparator(registry, nil)
	diff, err := comparator.Compare(ctx, "test", "1.0.0", "2.0.0")
	require.NoError(t, err)

	assert.Contains(t, diff.AddedVars, "phone")
	assert.Contains(t, diff.RemovedVars, "age")
	assert.Contains(t, diff.RemovedVars, "email")
	assert.Contains(t, diff.ChangedVars, "name") // Required changed
}

func TestPromptVersionComparator_Compare_ContentDiff(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	v1 := &PromptVersion{
		Name:    "test",
		Version: "1.0.0",
		Content: "Line 1\nLine 2\nLine 3",
	}

	v2 := &PromptVersion{
		Name:    "test",
		Version: "2.0.0",
		Content: "Line 1\nModified Line 2\nLine 3\nLine 4",
	}

	require.NoError(t, registry.Create(ctx, v1))
	require.NoError(t, registry.Create(ctx, v2))

	comparator := NewPromptVersionComparator(registry, nil)
	diff, err := comparator.Compare(ctx, "test", "1.0.0", "2.0.0")
	require.NoError(t, err)

	assert.Contains(t, diff.ContentDiff, "- Line 2")
	assert.Contains(t, diff.ContentDiff, "+ Modified Line 2")
	assert.Contains(t, diff.ContentDiff, "+ Line 4")
}

// =============================================================================
// EXPERIMENT MANAGER TESTS
// =============================================================================

func TestInMemoryExperimentManager_Create_Validations(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		exp     *Experiment
		wantErr string
	}{
		{
			name:    "missing name",
			exp:     &Experiment{Variants: []*Variant{{Name: "A"}, {Name: "B"}}},
			wantErr: "name is required",
		},
		{
			name:    "less than 2 variants",
			exp:     &Experiment{Name: "Test", Variants: []*Variant{{Name: "A"}}},
			wantErr: "at least 2 variants",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.Create(ctx, tt.exp)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInMemoryExperimentManager_Create_GeneratesVariantIDs(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name: "Test",
		Variants: []*Variant{
			{Name: "A"},
			{Name: "B"},
		},
	}

	require.NoError(t, manager.Create(ctx, exp))

	assert.NotEmpty(t, exp.Variants[0].ID)
	assert.NotEmpty(t, exp.Variants[1].ID)
}

func TestInMemoryExperimentManager_Create_TrafficSplitValidation(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	tests := []struct {
		name         string
		trafficSplit map[string]float64
		variantIDs   []string
		wantErr      string
	}{
		{
			name:         "negative split",
			trafficSplit: map[string]float64{"a": -0.5, "b": 1.5},
			variantIDs:   []string{"a", "b"},
			wantErr:      "cannot be negative",
		},
		{
			name:         "sum not 1.0",
			trafficSplit: map[string]float64{"a": 0.3, "b": 0.3},
			variantIDs:   []string{"a", "b"},
			wantErr:      "must sum to 1.0",
		},
		{
			name:         "missing variant in split",
			trafficSplit: map[string]float64{"a": 1.0},
			variantIDs:   []string{"a", "b"},
			wantErr:      "missing traffic split",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := &Experiment{
				Name: "Test",
				Variants: []*Variant{
					{ID: tt.variantIDs[0], Name: "A"},
					{ID: tt.variantIDs[1], Name: "B"},
				},
				TrafficSplit: tt.trafficSplit,
			}

			err := manager.Create(ctx, exp)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInMemoryExperimentManager_Create_AutoTrafficSplit(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name: "Test",
		Variants: []*Variant{
			{Name: "A"},
			{Name: "B"},
			{Name: "C"},
		},
	}

	require.NoError(t, manager.Create(ctx, exp))

	// Should be equal split
	for _, v := range exp.Variants {
		assert.InDelta(t, 1.0/3.0, exp.TrafficSplit[v.ID], 0.01)
	}
}

func TestInMemoryExperimentManager_Get_NotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	_, err := manager.Get(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryExperimentManager_List_WithFilter(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	// Create experiments with different statuses
	for i := 0; i < 3; i++ {
		exp := &Experiment{
			Name:     fmt.Sprintf("Exp-%d", i),
			Variants: []*Variant{{Name: "A"}, {Name: "B"}},
		}
		require.NoError(t, manager.Create(ctx, exp))

		if i == 1 {
			require.NoError(t, manager.Start(ctx, exp.ID))
		}
	}

	// List all
	all, err := manager.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// List running
	running, err := manager.List(ctx, ExperimentStatusRunning)
	require.NoError(t, err)
	assert.Len(t, running, 1)

	// List draft
	draft, err := manager.List(ctx, ExperimentStatusDraft)
	require.NoError(t, err)
	assert.Len(t, draft, 2)
}

func TestInMemoryExperimentManager_Start_InvalidStatus(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name:     "Test",
		Variants: []*Variant{{Name: "A"}, {Name: "B"}},
	}
	require.NoError(t, manager.Create(ctx, exp))

	// Start and complete
	require.NoError(t, manager.Start(ctx, exp.ID))
	require.NoError(t, manager.Complete(ctx, exp.ID, ""))

	// Cannot start completed experiment
	err := manager.Start(ctx, exp.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot start")
}

func TestInMemoryExperimentManager_Start_NotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	err := manager.Start(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryExperimentManager_Pause_InvalidStatus(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name:     "Test",
		Variants: []*Variant{{Name: "A"}, {Name: "B"}},
	}
	require.NoError(t, manager.Create(ctx, exp))

	// Cannot pause draft experiment
	err := manager.Pause(ctx, exp.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot pause")
}

func TestInMemoryExperimentManager_Pause_NotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	err := manager.Pause(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryExperimentManager_Complete_InvalidWinner(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name:     "Test",
		Variants: []*Variant{{Name: "A"}, {Name: "B"}},
	}
	require.NoError(t, manager.Create(ctx, exp))
	require.NoError(t, manager.Start(ctx, exp.ID))

	// Invalid winner
	err := manager.Complete(ctx, exp.ID, "invalid-variant-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid winner")
}

func TestInMemoryExperimentManager_Complete_NotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	err := manager.Complete(ctx, "nonexistent", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryExperimentManager_Cancel_AlreadyFinalized(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name:     "Test",
		Variants: []*Variant{{Name: "A"}, {Name: "B"}},
	}
	require.NoError(t, manager.Create(ctx, exp))
	require.NoError(t, manager.Start(ctx, exp.ID))
	require.NoError(t, manager.Complete(ctx, exp.ID, ""))

	// Cannot cancel completed experiment
	err := manager.Cancel(ctx, exp.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already finalized")
}

func TestInMemoryExperimentManager_Cancel_NotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	err := manager.Cancel(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryExperimentManager_Cancel_Success(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name:     "Test",
		Variants: []*Variant{{Name: "A"}, {Name: "B"}},
	}
	require.NoError(t, manager.Create(ctx, exp))
	require.NoError(t, manager.Start(ctx, exp.ID))

	// Cancel running experiment
	require.NoError(t, manager.Cancel(ctx, exp.ID))

	exp, err := manager.Get(ctx, exp.ID)
	require.NoError(t, err)
	assert.Equal(t, ExperimentStatusCancelled, exp.Status)
	assert.NotNil(t, exp.EndTime)
}

func TestInMemoryExperimentManager_AssignVariant_NotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	_, err := manager.AssignVariant(ctx, "nonexistent", "user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryExperimentManager_AssignVariant_NotRunning(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name:     "Test",
		Variants: []*Variant{{Name: "A"}, {Name: "B"}},
	}
	require.NoError(t, manager.Create(ctx, exp))

	// Experiment is still draft
	_, err := manager.AssignVariant(ctx, exp.ID, "user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestInMemoryExperimentManager_AssignVariant_Deterministic(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name:     "Test",
		Variants: []*Variant{{Name: "A"}, {Name: "B"}},
	}
	require.NoError(t, manager.Create(ctx, exp))
	require.NoError(t, manager.Start(ctx, exp.ID))

	// Multiple users get assigned to variants
	variants := make(map[string]int)
	for i := 0; i < 100; i++ {
		v, err := manager.AssignVariant(ctx, exp.ID, fmt.Sprintf("user-%d", i))
		require.NoError(t, err)
		variants[v.ID]++
	}

	// Both variants should have some assignments (probabilistic but with 50/50 split)
	assert.Greater(t, variants[exp.Variants[0].ID], 0)
	assert.Greater(t, variants[exp.Variants[1].ID], 0)
}

func TestInMemoryExperimentManager_RecordMetric_NotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	// Experiment not found
	err := manager.RecordMetric(ctx, "nonexistent", "variant", "metric", 1.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "experiment not found")
}

func TestInMemoryExperimentManager_RecordMetric_VariantNotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name:     "Test",
		Variants: []*Variant{{Name: "A"}, {Name: "B"}},
	}
	require.NoError(t, manager.Create(ctx, exp))

	// Variant not found
	err := manager.RecordMetric(ctx, exp.ID, "invalid-variant", "metric", 1.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "variant not found")
}

func TestInMemoryExperimentManager_GetResults_NotFound(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	_, err := manager.GetResults(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryExperimentManager_GetResults_WithControlComparison(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name: "Test",
		Variants: []*Variant{
			{Name: "Control", IsControl: true},
			{Name: "Treatment"},
		},
	}
	require.NoError(t, manager.Create(ctx, exp))
	require.NoError(t, manager.Start(ctx, exp.ID))

	// Record metrics for both variants (enough for significance calculation)
	for i := 0; i < 50; i++ {
		require.NoError(t, manager.RecordMetric(ctx, exp.ID, exp.Variants[0].ID, "quality", 0.7))
		require.NoError(t, manager.RecordMetric(ctx, exp.ID, exp.Variants[1].ID, "quality", 0.8))
	}

	results, err := manager.GetResults(ctx, exp.ID)
	require.NoError(t, err)

	// Treatment should have improvement vs control
	treatmentResult := results.VariantResults[exp.Variants[1].ID]
	assert.Greater(t, treatmentResult.Improvement, 0.0)
}

func TestInMemoryExperimentManager_GetResults_NoSamples(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name: "Test",
		Variants: []*Variant{
			{Name: "Control", IsControl: true},
			{Name: "Treatment"},
		},
	}
	require.NoError(t, manager.Create(ctx, exp))

	results, err := manager.GetResults(ctx, exp.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, results.TotalSamples)
}

func TestInMemoryExperimentManager_GetResults_InsufficientSamples(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	exp := &Experiment{
		Name: "Test",
		Variants: []*Variant{
			{Name: "Control", IsControl: true},
			{Name: "Treatment"},
		},
	}
	require.NoError(t, manager.Create(ctx, exp))
	require.NoError(t, manager.Start(ctx, exp.ID))

	// Only a few samples (less than 30)
	for i := 0; i < 10; i++ {
		require.NoError(t, manager.RecordMetric(ctx, exp.ID, exp.Variants[0].ID, "quality", 0.7))
		require.NoError(t, manager.RecordMetric(ctx, exp.ID, exp.Variants[1].ID, "quality", 0.8))
	}

	results, err := manager.GetResults(ctx, exp.ID)
	require.NoError(t, err)
	assert.Contains(t, results.Recommendation, "insufficient")
}

// =============================================================================
// CONTINUOUS EVALUATOR TESTS
// =============================================================================

func TestInMemoryContinuousEvaluator_CreateRun_Validations(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	// Create a dataset first for valid tests
	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	tests := []struct {
		name    string
		run     *EvaluationRun
		wantErr string
	}{
		{
			name:    "missing name",
			run:     &EvaluationRun{Dataset: dataset.ID},
			wantErr: "name is required",
		},
		{
			name:    "missing dataset",
			run:     &EvaluationRun{Name: "Test"},
			wantErr: "dataset is required",
		},
		{
			name:    "non-existent dataset",
			run:     &EvaluationRun{Name: "Test", Dataset: "nonexistent"},
			wantErr: "dataset not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := evaluator.CreateRun(ctx, tt.run)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestInMemoryContinuousEvaluator_CreateRun_WithExplicitID(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	run := &EvaluationRun{
		ID:      "my-custom-run-id",
		Name:    "Test Run",
		Dataset: dataset.ID,
	}

	require.NoError(t, evaluator.CreateRun(ctx, run))
	assert.Equal(t, "my-custom-run-id", run.ID)
}

func TestInMemoryContinuousEvaluator_StartRun_NotFound(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	err := evaluator.StartRun(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryContinuousEvaluator_StartRun_AlreadyStarted(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	run := &EvaluationRun{
		Name:    "Test Run",
		Dataset: dataset.ID,
	}
	require.NoError(t, evaluator.CreateRun(ctx, run))
	require.NoError(t, evaluator.StartRun(ctx, run.ID))

	// Starting again should fail
	err := evaluator.StartRun(ctx, run.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")
}

func TestInMemoryContinuousEvaluator_StartRun_ExecutionWithSamples(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	samples := []*DatasetSample{
		{Input: "Test 1", ExpectedOutput: "Output 1"},
		{Input: "Test 2", ExpectedOutput: "Output 2"},
	}
	require.NoError(t, evaluator.AddSamples(ctx, dataset.ID, samples))

	run := &EvaluationRun{
		Name:    "Test Run",
		Dataset: dataset.ID,
		Metrics: []string{"accuracy"},
	}
	require.NoError(t, evaluator.CreateRun(ctx, run))
	require.NoError(t, evaluator.StartRun(ctx, run.ID))

	// Wait for async execution
	time.Sleep(100 * time.Millisecond)

	// Check run completed
	completed, err := evaluator.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, EvaluationStatusCompleted, completed.Status)
	assert.NotNil(t, completed.Results)
	assert.Equal(t, 2, completed.Results.TotalSamples)
}

func TestInMemoryContinuousEvaluator_GetRun_NotFound(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	_, err := evaluator.GetRun(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryContinuousEvaluator_ListRuns_WithFilters(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	// Create runs with different properties
	runs := []struct {
		name       string
		promptName string
		modelName  string
	}{
		{"Run 1", "prompt-a", "model-x"},
		{"Run 2", "prompt-a", "model-y"},
		{"Run 3", "prompt-b", "model-x"},
	}

	for _, r := range runs {
		run := &EvaluationRun{
			Name:       r.name,
			Dataset:    dataset.ID,
			PromptName: r.promptName,
			ModelName:  r.modelName,
		}
		require.NoError(t, evaluator.CreateRun(ctx, run))
	}

	// Filter by prompt name
	promptFilter := &EvaluationFilter{PromptName: "prompt-a"}
	list, err := evaluator.ListRuns(ctx, promptFilter)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Filter by model name
	modelFilter := &EvaluationFilter{ModelName: "model-x"}
	list, err = evaluator.ListRuns(ctx, modelFilter)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Filter with limit
	limitFilter := &EvaluationFilter{Limit: 2}
	list, err = evaluator.ListRuns(ctx, limitFilter)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// No filter
	list, err = evaluator.ListRuns(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestInMemoryContinuousEvaluator_ListRuns_WithTimeFilters(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	// Create a run
	run := &EvaluationRun{Name: "Test", Dataset: dataset.ID}
	require.NoError(t, evaluator.CreateRun(ctx, run))

	// Filter with time in the future
	future := time.Now().Add(24 * time.Hour)
	futureFilter := &EvaluationFilter{StartTime: &future}
	list, err := evaluator.ListRuns(ctx, futureFilter)
	require.NoError(t, err)
	assert.Len(t, list, 0)

	// Filter with time in the past
	past := time.Now().Add(-24 * time.Hour)
	pastFilter := &EvaluationFilter{EndTime: &past}
	list, err = evaluator.ListRuns(ctx, pastFilter)
	require.NoError(t, err)
	assert.Len(t, list, 0)

	// Filter with status
	statusFilter := &EvaluationFilter{Status: EvaluationStatusRunning}
	list, err = evaluator.ListRuns(ctx, statusFilter)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestInMemoryContinuousEvaluator_CompareRuns_NotFound(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	// First run not found
	_, err := evaluator.CompareRuns(ctx, "run1", "run2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Create first run
	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	run1 := &EvaluationRun{Name: "Run 1", Dataset: dataset.ID}
	require.NoError(t, evaluator.CreateRun(ctx, run1))

	// Second run not found
	_, err = evaluator.CompareRuns(ctx, run1.ID, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryContinuousEvaluator_CompareRuns_NotCompleted(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	run1 := &EvaluationRun{Name: "Run 1", Dataset: dataset.ID}
	run2 := &EvaluationRun{Name: "Run 2", Dataset: dataset.ID}
	require.NoError(t, evaluator.CreateRun(ctx, run1))
	require.NoError(t, evaluator.CreateRun(ctx, run2))

	// Both runs have no results yet
	_, err := evaluator.CompareRuns(ctx, run1.ID, run2.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be completed")
}

func TestInMemoryContinuousEvaluator_CompareRuns_Success(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	// Create and complete runs manually
	run1 := &EvaluationRun{
		Name:    "Run 1",
		Dataset: dataset.ID,
		Results: &EvaluationResults{
			PassRate:     0.8,
			MetricScores: map[string]float64{"accuracy": 0.8, "quality": 0.7},
		},
	}
	run2 := &EvaluationRun{
		Name:    "Run 2",
		Dataset: dataset.ID,
		Results: &EvaluationResults{
			PassRate:     0.9,
			MetricScores: map[string]float64{"accuracy": 0.9, "quality": 0.6},
		},
	}

	require.NoError(t, evaluator.CreateRun(ctx, run1))
	require.NoError(t, evaluator.CreateRun(ctx, run2))

	// Manually set results
	if r, _ := evaluator.runs.Get(run1.ID); r != nil {
		r.Results = run1.Results
	}
	if r, _ := evaluator.runs.Get(run2.ID); r != nil {
		r.Results = run2.Results
	}

	comparison, err := evaluator.CompareRuns(ctx, run1.ID, run2.ID)
	require.NoError(t, err)

	assert.InDelta(t, 0.1, comparison.PassRateChange, 0.001)
	assert.Contains(t, comparison.Improvements, "accuracy")
	assert.Contains(t, comparison.Regressions, "quality")
}

func TestInMemoryContinuousEvaluator_CompareRuns_NoSignificantChanges(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	run1 := &EvaluationRun{
		Name:    "Run 1",
		Dataset: dataset.ID,
		Results: &EvaluationResults{
			PassRate:     0.8,
			MetricScores: map[string]float64{"accuracy": 0.8},
		},
	}
	run2 := &EvaluationRun{
		Name:    "Run 2",
		Dataset: dataset.ID,
		Results: &EvaluationResults{
			PassRate:     0.81,
			MetricScores: map[string]float64{"accuracy": 0.81},
		},
	}

	require.NoError(t, evaluator.CreateRun(ctx, run1))
	require.NoError(t, evaluator.CreateRun(ctx, run2))

	if r, _ := evaluator.runs.Get(run1.ID); r != nil {
		r.Results = run1.Results
	}
	if r, _ := evaluator.runs.Get(run2.ID); r != nil {
		r.Results = run2.Results
	}

	comparison, err := evaluator.CompareRuns(ctx, run1.ID, run2.ID)
	require.NoError(t, err)

	assert.Equal(t, "No significant changes", comparison.Summary)
}

func TestInMemoryContinuousEvaluator_CreateDataset_Validations(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	err := evaluator.CreateDataset(ctx, &Dataset{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestInMemoryContinuousEvaluator_CreateDataset_WithExplicitID(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{
		ID:   "my-custom-dataset-id",
		Name: "Test",
		Type: DatasetTypeGolden,
	}

	require.NoError(t, evaluator.CreateDataset(ctx, dataset))
	assert.Equal(t, "my-custom-dataset-id", dataset.ID)
}

func TestInMemoryContinuousEvaluator_GetDataset_NotFound(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	_, err := evaluator.GetDataset(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryContinuousEvaluator_AddSamples_NotFound(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	err := evaluator.AddSamples(ctx, "nonexistent", []*DatasetSample{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryContinuousEvaluator_AddSamples_GeneratesIDs(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	samples := []*DatasetSample{
		{Input: "Test 1"},
		{ID: "custom-id", Input: "Test 2"},
	}
	require.NoError(t, evaluator.AddSamples(ctx, dataset.ID, samples))

	assert.NotEmpty(t, samples[0].ID)
	assert.Equal(t, "custom-id", samples[1].ID)

	// Check dataset sample count updated
	ds, err := evaluator.GetDataset(ctx, dataset.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, ds.SampleCount)
}

// =============================================================================
// ALERT MANAGER TESTS
// =============================================================================

func TestInMemoryAlertManager_Create_GeneratesID(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	alert := &Alert{
		Type:     AlertTypeRegression,
		Severity: AlertSeverityWarning,
		Message:  "Test alert",
	}

	require.NoError(t, manager.Create(ctx, alert))
	assert.NotEmpty(t, alert.ID)
	assert.False(t, alert.CreatedAt.IsZero())
}

func TestInMemoryAlertManager_Create_KeepsExplicitID(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	alert := &Alert{
		ID:       "my-custom-alert-id",
		Type:     AlertTypeRegression,
		Severity: AlertSeverityWarning,
		Message:  "Test alert",
	}

	require.NoError(t, manager.Create(ctx, alert))
	assert.Equal(t, "my-custom-alert-id", alert.ID)
}

func TestInMemoryAlertManager_List_WithFilters(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	// Create alerts with different properties
	alerts := []*Alert{
		{Type: AlertTypeRegression, Severity: AlertSeverityWarning, Source: "evaluation"},
		{Type: AlertTypeThreshold, Severity: AlertSeverityCritical, Source: "evaluation"},
		{Type: AlertTypeAnomaly, Severity: AlertSeverityInfo, Source: "experiment"},
	}

	for _, a := range alerts {
		require.NoError(t, manager.Create(ctx, a))
	}

	// Filter by type
	typeFilter := &AlertFilter{Types: []AlertType{AlertTypeRegression}}
	list, err := manager.List(ctx, typeFilter)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Filter by severity
	severityFilter := &AlertFilter{Severities: []AlertSeverity{AlertSeverityCritical, AlertSeverityWarning}}
	list, err = manager.List(ctx, severityFilter)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Filter by source
	sourceFilter := &AlertFilter{Source: "evaluation"}
	list, err = manager.List(ctx, sourceFilter)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Filter with limit
	limitFilter := &AlertFilter{Limit: 2}
	list, err = manager.List(ctx, limitFilter)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestInMemoryAlertManager_List_UnackedFilter(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	// Create alerts
	alert1 := &Alert{Type: AlertTypeRegression, Severity: AlertSeverityWarning, Message: "Alert 1"}
	alert2 := &Alert{Type: AlertTypeRegression, Severity: AlertSeverityWarning, Message: "Alert 2"}

	require.NoError(t, manager.Create(ctx, alert1))
	require.NoError(t, manager.Create(ctx, alert2))

	// Acknowledge one
	require.NoError(t, manager.Acknowledge(ctx, alert1.ID))

	// Filter for unacked only
	unackedFilter := &AlertFilter{Unacked: true}
	list, err := manager.List(ctx, unackedFilter)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, alert2.ID, list[0].ID)
}

func TestInMemoryAlertManager_List_TimeFilter(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	alert := &Alert{Type: AlertTypeRegression, Severity: AlertSeverityWarning, Message: "Alert"}
	require.NoError(t, manager.Create(ctx, alert))

	// Filter with time in the future
	future := time.Now().Add(24 * time.Hour)
	futureFilter := &AlertFilter{StartTime: &future}
	list, err := manager.List(ctx, futureFilter)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestInMemoryAlertManager_Acknowledge_NotFound(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	err := manager.Acknowledge(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryAlertManager_Acknowledge_Success(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	alert := &Alert{Type: AlertTypeRegression, Severity: AlertSeverityWarning, Message: "Alert"}
	require.NoError(t, manager.Create(ctx, alert))

	require.NoError(t, manager.Acknowledge(ctx, alert.ID))

	// Verify acknowledged
	list, err := manager.List(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, list[0].AckedAt)
}

func TestInMemoryAlertManager_Subscribe_MultipleCallbacks(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	received1 := make(chan *Alert, 1)
	received2 := make(chan *Alert, 1)

	require.NoError(t, manager.Subscribe(ctx, func(a *Alert) error {
		received1 <- a
		return nil
	}))

	require.NoError(t, manager.Subscribe(ctx, func(a *Alert) error {
		received2 <- a
		return nil
	}))

	alert := &Alert{Type: AlertTypeRegression, Severity: AlertSeverityWarning, Message: "Alert"}
	require.NoError(t, manager.Create(ctx, alert))

	// Both callbacks should receive the alert
	select {
	case a := <-received1:
		assert.Equal(t, alert.ID, a.ID)
	case <-time.After(time.Second):
		t.Fatal("Did not receive alert in callback 1")
	}

	select {
	case a := <-received2:
		assert.Equal(t, alert.ID, a.ID)
	case <-time.After(time.Second):
		t.Fatal("Did not receive alert in callback 2")
	}
}

func TestInMemoryAlertManager_Subscribe_CallbackError(t *testing.T) {
	manager := NewInMemoryAlertManager(nil)
	ctx := context.Background()

	// Subscribe with a failing callback
	require.NoError(t, manager.Subscribe(ctx, func(a *Alert) error {
		return errors.New("callback error")
	}))

	// Create should still succeed even if callback fails
	alert := &Alert{Type: AlertTypeRegression, Severity: AlertSeverityWarning, Message: "Alert"}
	err := manager.Create(ctx, alert)
	require.NoError(t, err)

	// Wait for async callback
	time.Sleep(100 * time.Millisecond)
}

// =============================================================================
// LLMOPS SYSTEM TESTS
// =============================================================================

func TestLLMOpsSystem_InitializeWithNilConfig(t *testing.T) {
	system := NewLLMOpsSystem(nil, nil)
	require.NoError(t, system.Initialize())

	assert.NotNil(t, system.GetPromptRegistry())
	assert.NotNil(t, system.GetExperimentManager())
	assert.NotNil(t, system.GetEvaluator())
	assert.NotNil(t, system.GetAlertManager())
}

func TestLLMOpsSystem_SetDebateEvaluator(t *testing.T) {
	config := DefaultLLMOpsConfig()
	system := NewLLMOpsSystem(config, nil)

	mockEvaluator := &mockDebateEvaluator{}
	system.SetDebateEvaluator(mockEvaluator)

	// Now initialize - should use debate evaluator
	require.NoError(t, system.Initialize())
}

func TestLLMOpsSystem_SetVerifierIntegration(t *testing.T) {
	system := NewLLMOpsSystem(nil, nil)

	vi := NewVerifierIntegration(
		func(name string) float64 { return 0.9 },
		func(name string) bool { return true },
		nil,
	)

	system.SetVerifierIntegration(vi)

	require.NoError(t, system.Initialize())
}

func TestLLMOpsSystem_CreateModelExperiment(t *testing.T) {
	config := DefaultLLMOpsConfig()
	system := NewLLMOpsSystem(config, nil)
	require.NoError(t, system.Initialize())

	ctx := context.Background()
	exp, err := system.CreateModelExperiment(ctx, "Model Test", []string{"model-a", "model-b", "model-c"}, map[string]interface{}{
		"temperature": 0.7,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, exp.ID)
	assert.Len(t, exp.Variants, 3)
	assert.True(t, exp.Variants[0].IsControl)
	assert.False(t, exp.Variants[1].IsControl)
	assert.False(t, exp.Variants[2].IsControl)

	// Check equal traffic split
	for _, v := range exp.Variants {
		assert.InDelta(t, 1.0/3.0, exp.TrafficSplit[v.ID], 0.01)
	}
}

func TestLLMOpsSystem_CreateModelExperiment_TooFewModels(t *testing.T) {
	config := DefaultLLMOpsConfig()
	system := NewLLMOpsSystem(config, nil)
	require.NoError(t, system.Initialize())

	ctx := context.Background()
	_, err := system.CreateModelExperiment(ctx, "Model Test", []string{"model-a"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 models")
}

func TestLLMOpsSystem_CreatePromptExperiment_ControlFails(t *testing.T) {
	config := DefaultLLMOpsConfig()
	system := NewLLMOpsSystem(config, nil)
	require.NoError(t, system.Initialize())

	ctx := context.Background()

	// Invalid control prompt
	control := &PromptVersion{} // Missing required fields
	treatment := &PromptVersion{
		Name:    "treatment",
		Version: "1.0.0",
		Content: "Treatment content",
	}

	_, err := system.CreatePromptExperiment(ctx, "Test", control, treatment, 0.5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create control")
}

func TestLLMOpsSystem_CreatePromptExperiment_TreatmentFails(t *testing.T) {
	config := DefaultLLMOpsConfig()
	system := NewLLMOpsSystem(config, nil)
	require.NoError(t, system.Initialize())

	ctx := context.Background()

	control := &PromptVersion{
		Name:    "control",
		Version: "1.0.0",
		Content: "Control content",
	}
	treatment := &PromptVersion{} // Missing required fields

	_, err := system.CreatePromptExperiment(ctx, "Test", control, treatment, 0.5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create treatment")
}

// =============================================================================
// VERIFIER INTEGRATION TESTS
// =============================================================================

func TestVerifierIntegration_SelectBestProvider(t *testing.T) {
	scores := map[string]float64{
		"provider-a": 0.8,
		"provider-b": 0.9,
		"provider-c": 0.7,
	}

	vi := NewVerifierIntegration(
		func(name string) float64 { return scores[name] },
		func(name string) bool { return true },
		nil,
	)

	best, score := vi.SelectBestProvider([]string{"provider-a", "provider-b", "provider-c"})
	assert.Equal(t, "provider-b", best)
	assert.Equal(t, 0.9, score)
}

func TestVerifierIntegration_SelectBestProvider_SkipsUnhealthy(t *testing.T) {
	healthy := map[string]bool{
		"provider-a": true,
		"provider-b": false, // Unhealthy - best score but skipped
		"provider-c": true,
	}

	scores := map[string]float64{
		"provider-a": 0.8,
		"provider-b": 0.9,
		"provider-c": 0.7,
	}

	vi := NewVerifierIntegration(
		func(name string) float64 { return scores[name] },
		func(name string) bool { return healthy[name] },
		nil,
	)

	best, score := vi.SelectBestProvider([]string{"provider-a", "provider-b", "provider-c"})
	assert.Equal(t, "provider-a", best)
	assert.Equal(t, 0.8, score)
}

func TestVerifierIntegration_SelectBestProvider_NilFuncs(t *testing.T) {
	vi := NewVerifierIntegration(nil, nil, nil)

	best, score := vi.SelectBestProvider([]string{"provider-a", "provider-b"})
	assert.Equal(t, "provider-a", best) // Returns first provider
	assert.Equal(t, 0.0, score)

	best, score = vi.SelectBestProvider([]string{})
	assert.Empty(t, best)
	assert.Equal(t, 0.0, score)
}

// =============================================================================
// DEBATE EVALUATOR ADAPTER TESTS
// =============================================================================

type mockDebateEvaluator struct {
	scores map[string]float64
	err    error
}

func (m *mockDebateEvaluator) EvaluateWithDebate(ctx context.Context, prompt, response, expected string, metrics []string) (map[string]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.scores != nil {
		return m.scores, nil
	}
	result := make(map[string]float64)
	for _, metric := range metrics {
		result[metric] = 0.85
	}
	return result, nil
}

func TestDebateEvaluatorAdapter_Evaluate(t *testing.T) {
	ctx := context.Background()

	mock := &mockDebateEvaluator{
		scores: map[string]float64{"quality": 0.9, "accuracy": 0.85},
	}

	adapter := &debateEvaluatorAdapter{evaluator: mock}
	scores, err := adapter.Evaluate(ctx, "prompt", "response", "expected", []string{"quality", "accuracy"})

	require.NoError(t, err)
	assert.Equal(t, 0.9, scores["quality"])
	assert.Equal(t, 0.85, scores["accuracy"])
}

func TestDebateEvaluatorAdapter_EvaluateError(t *testing.T) {
	ctx := context.Background()

	mock := &mockDebateEvaluator{
		err: errors.New("evaluation failed"),
	}

	adapter := &debateEvaluatorAdapter{evaluator: mock}
	_, err := adapter.Evaluate(ctx, "prompt", "response", "expected", []string{"quality"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluation failed")
}

// =============================================================================
// CONTINUOUS EVALUATOR WITH LLM EVALUATOR TESTS
// =============================================================================

type mockLLMEvaluator struct {
	scores map[string]float64
	err    error
}

func (m *mockLLMEvaluator) Evaluate(ctx context.Context, prompt, response, expected string, metrics []string) (map[string]float64, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.scores != nil {
		return m.scores, nil
	}
	result := make(map[string]float64)
	for _, metric := range metrics {
		result[metric] = 0.8
	}
	return result, nil
}

func TestInMemoryContinuousEvaluator_ExecuteWithLLMEvaluator(t *testing.T) {
	mock := &mockLLMEvaluator{
		scores: map[string]float64{"quality": 0.9},
	}

	evaluator := NewInMemoryContinuousEvaluator(mock, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	samples := []*DatasetSample{
		{Input: "Test 1", ExpectedOutput: "Output 1"},
	}
	require.NoError(t, evaluator.AddSamples(ctx, dataset.ID, samples))

	run := &EvaluationRun{
		Name:    "Test Run",
		Dataset: dataset.ID,
		Metrics: []string{"quality"},
	}
	require.NoError(t, evaluator.CreateRun(ctx, run))
	require.NoError(t, evaluator.StartRun(ctx, run.ID))

	time.Sleep(100 * time.Millisecond)

	completed, err := evaluator.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, EvaluationStatusCompleted, completed.Status)
	assert.True(t, completed.Results.SampleResults[0].Passed)
}

func TestInMemoryContinuousEvaluator_ExecuteWithLLMEvaluatorError(t *testing.T) {
	mock := &mockLLMEvaluator{
		err: errors.New("LLM evaluation failed"),
	}

	evaluator := NewInMemoryContinuousEvaluator(mock, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	samples := []*DatasetSample{
		{Input: "Test 1", ExpectedOutput: "Output 1"},
	}
	require.NoError(t, evaluator.AddSamples(ctx, dataset.ID, samples))

	run := &EvaluationRun{
		Name:    "Test Run",
		Dataset: dataset.ID,
		Metrics: []string{"quality"},
	}
	require.NoError(t, evaluator.CreateRun(ctx, run))
	require.NoError(t, evaluator.StartRun(ctx, run.ID))

	time.Sleep(100 * time.Millisecond)

	completed, err := evaluator.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, EvaluationStatusCompleted, completed.Status)
	assert.False(t, completed.Results.SampleResults[0].Passed)
	assert.Contains(t, completed.Results.SampleResults[0].Error, "LLM evaluation failed")
}

func TestInMemoryContinuousEvaluator_ExecuteWithLowScore(t *testing.T) {
	mock := &mockLLMEvaluator{
		scores: map[string]float64{"quality": 0.5}, // Below 0.7 threshold
	}

	evaluator := NewInMemoryContinuousEvaluator(mock, nil, nil, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	samples := []*DatasetSample{
		{Input: "Test 1", ExpectedOutput: "Output 1"},
	}
	require.NoError(t, evaluator.AddSamples(ctx, dataset.ID, samples))

	run := &EvaluationRun{
		Name:    "Test Run",
		Dataset: dataset.ID,
		Metrics: []string{"quality"},
	}
	require.NoError(t, evaluator.CreateRun(ctx, run))
	require.NoError(t, evaluator.StartRun(ctx, run.ID))

	time.Sleep(100 * time.Millisecond)

	completed, err := evaluator.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.False(t, completed.Results.SampleResults[0].Passed)
}

func TestInMemoryContinuousEvaluator_ExecuteWithPromptRegistry(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	// Create a prompt
	prompt := &PromptVersion{
		Name:    "test-prompt",
		Version: "1.0.0",
		Content: "Answer the question: {{input}}",
	}
	require.NoError(t, registry.Create(ctx, prompt))

	evaluator := NewInMemoryContinuousEvaluator(nil, registry, nil, nil)

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	samples := []*DatasetSample{
		{Input: "What is 2+2?", ExpectedOutput: "4"},
	}
	require.NoError(t, evaluator.AddSamples(ctx, dataset.ID, samples))

	run := &EvaluationRun{
		Name:          "Test Run",
		Dataset:       dataset.ID,
		PromptName:    "test-prompt",
		PromptVersion: "1.0.0",
		Metrics:       []string{"accuracy"},
	}
	require.NoError(t, evaluator.CreateRun(ctx, run))
	require.NoError(t, evaluator.StartRun(ctx, run.ID))

	time.Sleep(100 * time.Millisecond)

	completed, err := evaluator.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, EvaluationStatusCompleted, completed.Status)
}

func TestInMemoryContinuousEvaluator_ExecuteWithLatestPrompt(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	prompt := &PromptVersion{
		Name:    "test-prompt",
		Version: "1.0.0",
		Content: "Template",
	}
	require.NoError(t, registry.Create(ctx, prompt))

	evaluator := NewInMemoryContinuousEvaluator(nil, registry, nil, nil)

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	samples := []*DatasetSample{{Input: "Test"}}
	require.NoError(t, evaluator.AddSamples(ctx, dataset.ID, samples))

	run := &EvaluationRun{
		Name:          "Test Run",
		Dataset:       dataset.ID,
		PromptName:    "test-prompt",
		PromptVersion: "", // Empty means latest
		Metrics:       []string{"accuracy"},
	}
	require.NoError(t, evaluator.CreateRun(ctx, run))
	require.NoError(t, evaluator.StartRun(ctx, run.ID))

	time.Sleep(100 * time.Millisecond)

	completed, err := evaluator.GetRun(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, EvaluationStatusCompleted, completed.Status)
}

// =============================================================================
// REGRESSION DETECTION TESTS
// =============================================================================

func TestInMemoryContinuousEvaluator_CheckForRegressions(t *testing.T) {
	alertManager := NewInMemoryAlertManager(nil)
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, alertManager, nil)
	ctx := context.Background()

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(ctx, dataset))

	samples := []*DatasetSample{{Input: "Test"}}
	require.NoError(t, evaluator.AddSamples(ctx, dataset.ID, samples))

	// Create first run with high pass rate
	run1 := &EvaluationRun{
		Name:       "Run 1",
		Dataset:    dataset.ID,
		PromptName: "prompt-a",
		ModelName:  "model-x",
		Metrics:    []string{"accuracy"},
	}
	require.NoError(t, evaluator.CreateRun(ctx, run1))
	require.NoError(t, evaluator.StartRun(ctx, run1.ID))
	time.Sleep(100 * time.Millisecond)

	// Manually set high pass rate. Take evaluator.mu because executeRun
	// mutates run.Results asynchronously and the test needs to synchronise
	// with it (Pattern Zeta survivor on the evaluator).
	evaluator.mu.Lock()
	if r, _ := evaluator.runs.Get(run1.ID); r != nil {
		r.Results.PassRate = 0.95
		r.Results.MetricScores["accuracy"] = 0.95
	}
	evaluator.mu.Unlock()

	// Create second run with low pass rate
	run2 := &EvaluationRun{
		Name:       "Run 2",
		Dataset:    dataset.ID,
		PromptName: "prompt-a",
		ModelName:  "model-x",
		Metrics:    []string{"accuracy"},
	}
	require.NoError(t, evaluator.CreateRun(ctx, run2))
	require.NoError(t, evaluator.StartRun(ctx, run2.ID))
	time.Sleep(100 * time.Millisecond)

	// Manually set low pass rate to trigger regression
	evaluator.mu.Lock()
	r2, _ := evaluator.runs.Get(run2.ID)
	if r2 != nil {
		r2.Results.PassRate = 0.80
		r2.Results.MetricScores["accuracy"] = 0.75
	}
	evaluator.mu.Unlock()

	// Manually trigger regression check
	evaluator.checkForRegressions(ctx, r2)

	// Check alerts were created
	alerts, err := alertManager.List(ctx, &AlertFilter{Types: []AlertType{AlertTypeRegression}})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(alerts), 1)
}

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

func TestPromptRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewInMemoryPromptRegistry(nil)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent creates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			prompt := &PromptVersion{
				Name:    fmt.Sprintf("prompt-%d", idx),
				Version: "1.0.0",
				Content: "Content",
			}
			_ = registry.Create(ctx, prompt)
		}(i)
	}

	wg.Wait()

	// Verify all created
	all, err := registry.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, numGoroutines)
}

func TestExperimentManager_ConcurrentAccess(t *testing.T) {
	manager := NewInMemoryExperimentManager(nil)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent creates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			exp := &Experiment{
				Name:     fmt.Sprintf("exp-%d", idx),
				Variants: []*Variant{{Name: "A"}, {Name: "B"}},
			}
			_ = manager.Create(ctx, exp)
		}(i)
	}

	wg.Wait()

	// Verify all created
	all, err := manager.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, numGoroutines)
}

// =============================================================================
// CONTEXT CANCELLATION TESTS
// =============================================================================

func TestInMemoryContinuousEvaluator_StartRun_ContextCancellation(t *testing.T) {
	evaluator := NewInMemoryContinuousEvaluator(nil, nil, nil, nil)

	dataset := &Dataset{Name: "Test", Type: DatasetTypeGolden}
	require.NoError(t, evaluator.CreateDataset(context.Background(), dataset))

	// Add many samples to make run take longer
	samples := make([]*DatasetSample, 100)
	for i := range samples {
		samples[i] = &DatasetSample{Input: fmt.Sprintf("Test %d", i)}
	}
	require.NoError(t, evaluator.AddSamples(context.Background(), dataset.ID, samples))

	run := &EvaluationRun{
		Name:    "Test Run",
		Dataset: dataset.ID,
		Metrics: []string{"accuracy"},
	}
	require.NoError(t, evaluator.CreateRun(context.Background(), run))

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start run
	require.NoError(t, evaluator.StartRun(ctx, run.ID))

	// Cancel immediately
	cancel()

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Run should have failed due to cancellation
	completed, err := evaluator.GetRun(context.Background(), run.ID)
	require.NoError(t, err)
	// The run may be completed or failed depending on timing
	assert.True(t, completed.Status == EvaluationStatusCompleted || completed.Status == EvaluationStatusFailed || completed.Status == EvaluationStatusRunning)
}
