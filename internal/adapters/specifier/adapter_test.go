package specifier_test

import (
	"context"
	"testing"

	adapter "dev.helix.agent/internal/adapters/specifier"
	"github.com/stretchr/testify/assert"
)

func TestIsHelixSpecifierEnabled(t *testing.T) {
	t.Parallel()
	enabled := adapter.IsHelixSpecifierEnabled()
	assert.IsType(t, true, enabled)
}

func TestSpecifierBackendName(t *testing.T) {
	t.Parallel()
	name := adapter.SpecifierBackendName()
	assert.NotEmpty(t, name)
	assert.Contains(t, []string{
		"digital.vasic.helixspecifier", "internal.speckit",
	}, name)
}

func TestNewOptimalSpecAdapter_Default(t *testing.T) {
	t.Parallel()
	if !adapter.IsHelixSpecifierEnabled() {
		t.Skip(
			"Skipping: nohelixspecifier tag is active",
		)
	}
	sa := adapter.NewOptimalSpecAdapter()
	assert.NotNil(t, sa,
		"Default build must return a configured HelixSpecifier adapter")
	assert.Equal(t, "digital.vasic.helixspecifier",
		adapter.SpecifierBackendName())
	assert.True(t, adapter.IsHelixSpecifierEnabled())
}

func TestNewOptimalSpecAdapter_OptOut(t *testing.T) {
	t.Parallel()
	if adapter.IsHelixSpecifierEnabled() {
		t.Skip(
			"Skipping opt-out test when HelixSpecifier is active",
		)
	}
	sa := adapter.NewOptimalSpecAdapter()
	assert.Nil(t, sa)
	assert.Equal(t, "internal.speckit",
		adapter.SpecifierBackendName())
	assert.False(t, adapter.IsHelixSpecifierEnabled())
}

func TestHelixSpecifierIsDefault(t *testing.T) {
	t.Parallel()
	if !adapter.IsHelixSpecifierEnabled() {
		t.Skip("HelixSpecifier not active (nohelixspecifier tag). " +
			"Default builds always use HelixSpecifier.")
	}
	assert.True(t, adapter.IsHelixSpecifierEnabled())
	assert.Equal(t, "digital.vasic.helixspecifier",
		adapter.SpecifierBackendName())

	sa := adapter.NewOptimalSpecAdapter()
	assert.NotNil(t, sa,
		"Default HelixSpecifier adapter must be initialized")
	assert.True(t, sa.IsReady())
	assert.Equal(t, "HelixSpecifier", sa.Name())
	assert.Equal(t, "1.0.0", sa.Version())
}

func TestSpecAdapter_NilEngine(t *testing.T) {
	t.Parallel()
	sa := adapter.NewSpecAdapter(nil)
	assert.NotNil(t, sa, "NewSpecAdapter should return a no-op adapter, never nil")
	assert.False(t, sa.IsReady())
	assert.Equal(t, "", sa.Name())
	assert.Equal(t, "", sa.Version())

	_, err := sa.ClassifyEffort(context.Background(), "test")
	assert.Error(t, err)

	err = sa.Health(context.Background())
	assert.Error(t, err)

	ok := sa.SetDebateFunc(nil)
	assert.False(t, ok)
}

func TestSpecAdapter_Health(t *testing.T) {
	t.Parallel()
	if !adapter.IsHelixSpecifierEnabled() {
		t.Skip("HelixSpecifier not active")
	}
	sa := adapter.NewOptimalSpecAdapter()
	assert.NotNil(t, sa)
	err := sa.Health(context.Background())
	assert.NoError(t, err)
}

func TestSpecAdapter_ClassifyEffort(t *testing.T) {
	t.Parallel()
	if !adapter.IsHelixSpecifierEnabled() {
		t.Skip("HelixSpecifier not active")
	}
	sa := adapter.NewOptimalSpecAdapter()
	assert.NotNil(t, sa)

	cl, err := sa.ClassifyEffort(
		context.Background(),
		"Build a new authentication system",
	)
	assert.NoError(t, err)
	assert.NotNil(t, cl)
	assert.NotEmpty(t, cl.Level)
}

func TestSpecAdapter_SetDebateFunc(t *testing.T) {
	t.Parallel()
	if !adapter.IsHelixSpecifierEnabled() {
		t.Skip("HelixSpecifier not active")
	}
	sa := adapter.NewOptimalSpecAdapter()
	assert.NotNil(t, sa)

	called := false
	fn := func(
		ctx context.Context,
		topic string,
		rounds int,
		metadata map[string]interface{},
	) (string, float64, string, error) {
		called = true
		return "test output", 0.95, "test-id", nil
	}

	ok := sa.SetDebateFunc(fn)
	assert.True(t, ok,
		"SetDebateFunc should succeed on real "+
			"HelixSpecifier engine")

	// Verify function is used by running a flow
	cl, err := sa.ClassifyEffort(
		context.Background(),
		"Build auth system",
	)
	assert.NoError(t, err)

	result, err := sa.ExecuteFlow(
		context.Background(),
		"Build auth system",
		cl,
	)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, called,
		"Injected debate function should have been called")
	assert.Greater(t, result.OverallQualityScore, 0.75,
		"Score should reflect real debate function")
}
