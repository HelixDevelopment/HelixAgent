package challenges

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/assertion"
	"digital.vasic.challenges/pkg/plugin"
	"digital.vasic.challenges/pkg/registry"
)

func TestHelixPlugin_Name(t *testing.T) {
	t.Parallel()
	p := NewHelixPlugin(nil)
	assert.Equal(t, "helix-agent", p.Name())
}

func TestHelixPlugin_Version(t *testing.T) {
	t.Parallel()
	p := NewHelixPlugin(nil)
	assert.Equal(t, "1.0.0", p.Version())
}

func TestHelixPlugin_Init_RegistersChallenges(t *testing.T) {
	t.Parallel()
	providers := []ProviderInfo{
		{Name: "test-provider", Verified: true, Score: 8.0},
	}
	p := NewHelixPlugin(providers)

	reg := registry.NewRegistry()
	engine := assertion.NewEngine()

	ctx := &plugin.PluginContext{
		Config: map[string]interface{}{
			"registry":         reg,
			"assertion_engine": engine,
		},
	}
	err := p.Init(ctx)
	require.NoError(t, err)

	// Should have registered 3 challenges.
	all := reg.List()
	assert.Len(t, all, 3)
}

func TestHelixPlugin_Init_RegistersAssertions(t *testing.T) {
	t.Parallel()
	p := NewHelixPlugin(nil)

	reg := registry.NewRegistry()
	engine := assertion.NewEngine()

	ctx := &plugin.PluginContext{
		Config: map[string]interface{}{
			"registry":         reg,
			"assertion_engine": engine,
		},
	}
	err := p.Init(ctx)
	require.NoError(t, err)

	// Verify custom evaluators were registered.
	assert.True(t, engine.HasEvaluator("provider_verified"))
	assert.True(t, engine.HasEvaluator("min_provider_score"))
}

func TestHelixPlugin_Init_NilContext(t *testing.T) {
	t.Parallel()
	p := NewHelixPlugin(nil)
	err := p.Init(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestHelixPlugin_Init_EmptyContext(t *testing.T) {
	t.Parallel()
	p := NewHelixPlugin(nil)
	ctx := &plugin.PluginContext{
		Config: map[string]interface{}{},
	}
	err := p.Init(ctx)
	require.NoError(t, err)
}

func TestHelixPlugin_ImplementsPluginInterface(t *testing.T) {
	t.Parallel()
	var _ plugin.Plugin = (*HelixPlugin)(nil)
}
