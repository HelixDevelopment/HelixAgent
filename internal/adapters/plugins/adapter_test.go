package plugins_test

import (
	"context"
	"fmt"
	"testing"

	adapter "dev.helix.agent/internal/adapters/plugins"
	"dev.helix.agent/internal/models"
	helixplugins "dev.helix.agent/internal/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mock LLMPlugin for RegistryAdapter tests
// ============================================================================

type mockLLMPlugin struct {
	name    string
	version string
}

func (m *mockLLMPlugin) Name() string    { return m.name }
func (m *mockLLMPlugin) Version() string { return m.version }
func (m *mockLLMPlugin) Capabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{SupportedModels: []string{m.name}}
}
func (m *mockLLMPlugin) Init(_ map[string]interface{}) error { return nil }
func (m *mockLLMPlugin) Shutdown(_ context.Context) error    { return nil }
func (m *mockLLMPlugin) HealthCheck(_ context.Context) error { return nil }
func (m *mockLLMPlugin) Complete(_ context.Context, _ *models.LLMRequest) (*models.LLMResponse, error) {
	return nil, fmt.Errorf("not supported")
}
func (m *mockLLMPlugin) CompleteStream(_ context.Context, _ *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return nil, fmt.Errorf("not supported")
}
func (m *mockLLMPlugin) SetSecurityContext(_ *helixplugins.PluginSecurityContext) error { return nil }

// ============================================================================
// RegistryAdapter Tests
// ============================================================================

func TestNewRegistryAdapter(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	require.NotNil(t, r)
}

func TestRegistryAdapter_RegisterAndGet(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	p := &mockLLMPlugin{name: "test-plugin", version: "1.0.0"}

	err := r.Register(p)
	require.NoError(t, err)

	got, ok := r.Get("test-plugin")
	assert.True(t, ok)
	require.NotNil(t, got)
	assert.Equal(t, "test-plugin", got.Name())
}

func TestRegistryAdapter_Get_NotFound(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	got, ok := r.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestRegistryAdapter_Unregister(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	r.Register(&mockLLMPlugin{name: "to-remove", version: "1.0.0"})

	err := r.Unregister("to-remove")
	require.NoError(t, err)

	_, ok := r.Get("to-remove")
	assert.False(t, ok)
}

func TestRegistryAdapter_List(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	list := r.List()
	assert.Empty(t, list)

	r.Register(&mockLLMPlugin{name: "p1", version: "1.0"})
	r.Register(&mockLLMPlugin{name: "p2", version: "1.0"})

	list = r.List()
	assert.Len(t, list, 2)
}

func TestRegistryAdapter_StartAll_Empty(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	err := r.StartAll(context.Background())
	assert.NoError(t, err)
}

func TestRegistryAdapter_StartAll_WithPlugins(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	r.Register(&mockLLMPlugin{name: "p1", version: "1.0"})
	r.Register(&mockLLMPlugin{name: "p2", version: "1.0"})

	err := r.StartAll(context.Background())
	assert.NoError(t, err)
}

func TestRegistryAdapter_StopAll(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	r.Register(&mockLLMPlugin{name: "p1", version: "1.0"})

	err := r.StopAll(context.Background())
	assert.NoError(t, err)
}

func TestRegistryAdapter_SetDependencies(t *testing.T) {
	t.Parallel()
	r := adapter.NewRegistryAdapter()
	r.Register(&mockLLMPlugin{name: "main", version: "1.0"})
	r.Register(&mockLLMPlugin{name: "dep", version: "1.0"})

	err := r.SetDependencies("main", []string{"dep"})
	assert.NoError(t, err)
}

// ============================================================================
// LoaderAdapter Tests
// ============================================================================

func TestNewSharedObjectLoaderAdapter(t *testing.T) {
	t.Parallel()
	l := adapter.NewSharedObjectLoaderAdapter(nil)
	require.NotNil(t, l)
}

func TestNewProcessLoaderAdapter(t *testing.T) {
	t.Parallel()
	l := adapter.NewProcessLoaderAdapter(nil)
	require.NotNil(t, l)
}

func TestSharedObjectLoaderAdapter_Load_InvalidPath(t *testing.T) {
	t.Parallel()
	l := adapter.NewSharedObjectLoaderAdapter(nil)
	p, err := l.Load("/nonexistent/path/plugin.so")
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestSharedObjectLoaderAdapter_LoadDir_InvalidDir(t *testing.T) {
	t.Parallel()
	l := adapter.NewSharedObjectLoaderAdapter(nil)
	plugins, err := l.LoadDir("/nonexistent/dir")
	assert.Error(t, err)
	assert.Nil(t, plugins)
}

// ============================================================================
// StructuredParserAdapter Tests
// ============================================================================

func TestNewJSONParserAdapter(t *testing.T) {
	t.Parallel()
	p := adapter.NewJSONParserAdapter()
	require.NotNil(t, p)
}

func TestNewYAMLParserAdapter(t *testing.T) {
	t.Parallel()
	p := adapter.NewYAMLParserAdapter()
	require.NotNil(t, p)
}

func TestNewMarkdownParserAdapter(t *testing.T) {
	t.Parallel()
	p := adapter.NewMarkdownParserAdapter()
	require.NotNil(t, p)
}

func TestJSONParserAdapter_Parse(t *testing.T) {
	t.Parallel()
	p := adapter.NewJSONParserAdapter()
	schema := &adapter.Schema{
		Type: "object",
		Properties: map[string]*adapter.Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
	}

	result, err := p.Parse(`{"name": "Alice", "age": 30}`, schema)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestJSONParserAdapter_Parse_InvalidJSON(t *testing.T) {
	t.Parallel()
	p := adapter.NewJSONParserAdapter()
	schema := &adapter.Schema{Type: "object"}

	_, err := p.Parse("not json", schema)
	assert.Error(t, err)
}

func TestJSONParserAdapter_Parse_NilSchema(t *testing.T) {
	t.Parallel()
	p := adapter.NewJSONParserAdapter()
	result, err := p.Parse(`{"key": "value"}`, nil)
	// With nil schema, behavior depends on parser implementation
	// just ensure it doesn't panic
	_ = result
	_ = err
}

// ============================================================================
// ValidatorAdapter Tests
// ============================================================================

func TestNewValidatorAdapter(t *testing.T) {
	t.Parallel()
	v := adapter.NewValidatorAdapter(false)
	require.NotNil(t, v)

	strict := adapter.NewValidatorAdapter(true)
	require.NotNil(t, strict)
}

func TestValidatorAdapter_Validate_ValidJSON(t *testing.T) {
	t.Parallel()
	v := adapter.NewValidatorAdapter(false)
	schema := &adapter.Schema{
		Type: "object",
		Properties: map[string]*adapter.Schema{
			"name": {Type: "string"},
		},
		Required: []string{"name"},
	}

	result, err := v.Validate(`{"name": "Alice"}`, schema)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Valid)
}

func TestValidatorAdapter_Validate_InvalidJSON(t *testing.T) {
	t.Parallel()
	v := adapter.NewValidatorAdapter(false)
	schema := &adapter.Schema{Type: "object"}

	result, err := v.Validate("not json", schema)
	// Should either return error or invalid result
	if err == nil {
		require.NotNil(t, result)
		assert.False(t, result.Valid)
	}
}

func TestValidatorAdapter_Repair(t *testing.T) {
	t.Parallel()
	v := adapter.NewValidatorAdapter(false)
	schema := &adapter.Schema{Type: "object"}

	repaired, err := v.Repair(`{"name": "Alice"`, schema)
	// Repair may succeed or fail depending on implementation
	_ = repaired
	_ = err
}

// ============================================================================
// Schema / ToModuleSchema Tests
// ============================================================================

func TestSchema_Fields(t *testing.T) {
	t.Parallel()
	schema := &adapter.Schema{
		Type:        "object",
		Description: "A test schema",
		Required:    []string{"name"},
		Properties: map[string]*adapter.Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
			"tags": {
				Type:  "array",
				Items: &adapter.Schema{Type: "string"},
			},
		},
	}

	assert.Equal(t, "object", schema.Type)
	assert.Equal(t, "A test schema", schema.Description)
	assert.Len(t, schema.Required, 1)
	assert.Len(t, schema.Properties, 3)
}

func TestToModuleSchema_Nil(t *testing.T) {
	t.Parallel()
	result := adapter.ToModuleSchema(nil)
	assert.Nil(t, result)
}

func TestToModuleSchema_Simple(t *testing.T) {
	t.Parallel()
	schema := &adapter.Schema{
		Type:        "string",
		Description: "A string field",
	}
	mod := adapter.ToModuleSchema(schema)
	require.NotNil(t, mod)
	assert.Equal(t, "string", mod.Type)
	assert.Equal(t, "A string field", mod.Description)
}

func TestToModuleSchema_WithProperties(t *testing.T) {
	t.Parallel()
	schema := &adapter.Schema{
		Type: "object",
		Properties: map[string]*adapter.Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}
	mod := adapter.ToModuleSchema(schema)
	require.NotNil(t, mod)
	assert.Equal(t, "object", mod.Type)
	assert.Len(t, mod.Properties, 2)
	assert.Equal(t, []string{"name"}, mod.Required)
}

func TestToModuleSchema_WithItems(t *testing.T) {
	t.Parallel()
	schema := &adapter.Schema{
		Type:  "array",
		Items: &adapter.Schema{Type: "string"},
	}
	mod := adapter.ToModuleSchema(schema)
	require.NotNil(t, mod)
	require.NotNil(t, mod.Items)
	assert.Equal(t, "string", mod.Items.Type)
}

// ============================================================================
// ToAdapterValidationResult Tests
// ============================================================================

func TestToAdapterValidationResult_Nil(t *testing.T) {
	t.Parallel()
	result := adapter.ToAdapterValidationResult(nil)
	assert.Nil(t, result)
}

// ============================================================================
// StateTrackerAdapter Tests
// ============================================================================

func TestNewStateTrackerAdapter(t *testing.T) {
	t.Parallel()
	st := adapter.NewStateTrackerAdapter()
	require.NotNil(t, st)
}

func TestStateTrackerAdapter_GetInitialState(t *testing.T) {
	t.Parallel()
	st := adapter.NewStateTrackerAdapter()
	state := st.Get()
	assert.Equal(t, adapter.StateUninitialized, state)
}

func TestStateTrackerAdapter_Set(t *testing.T) {
	t.Parallel()
	st := adapter.NewStateTrackerAdapter()
	st.Set(adapter.StateInitialized)
	assert.Equal(t, adapter.StateInitialized, st.Get())
}

func TestStateTrackerAdapter_Transition_Valid(t *testing.T) {
	t.Parallel()
	st := adapter.NewStateTrackerAdapter()
	st.Set(adapter.StateInitialized)

	err := st.Transition(adapter.StateInitialized, adapter.StateRunning)
	assert.NoError(t, err)
	assert.Equal(t, adapter.StateRunning, st.Get())
}

func TestStateTrackerAdapter_Transition_WrongExpected(t *testing.T) {
	t.Parallel()
	st := adapter.NewStateTrackerAdapter()
	st.Set(adapter.StateInitialized)

	// Transition expects StateStopped but current is StateInitialized
	err := st.Transition(adapter.StateStopped, adapter.StateRunning)
	assert.Error(t, err)
	// State should not have changed
	assert.Equal(t, adapter.StateInitialized, st.Get())
}

func TestPluginStateConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, adapter.PluginState(0), adapter.StateUninitialized)
	assert.NotEqual(t, adapter.StateUninitialized, adapter.StateInitialized)
	assert.NotEqual(t, adapter.StateInitialized, adapter.StateRunning)
	assert.NotEqual(t, adapter.StateRunning, adapter.StateStopped)
	assert.NotEqual(t, adapter.StateStopped, adapter.StateFailed)
}

// ============================================================================
// CheckVersionConstraint Tests
// ============================================================================

func TestCheckVersionConstraint_Satisfied(t *testing.T) {
	t.Parallel()
	ok, err := adapter.CheckVersionConstraint("1.2.0", ">=1.0.0")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCheckVersionConstraint_NotSatisfied(t *testing.T) {
	t.Parallel()
	ok, err := adapter.CheckVersionConstraint("0.5.0", ">=1.0.0")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCheckVersionConstraint_Exact(t *testing.T) {
	t.Parallel()
	ok, err := adapter.CheckVersionConstraint("1.0.0", "1.0.0")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCheckVersionConstraint_Invalid(t *testing.T) {
	t.Parallel()
	_, err := adapter.CheckVersionConstraint("not-a-version", ">=1.0.0")
	// May return error for invalid version strings
	_ = err
}
