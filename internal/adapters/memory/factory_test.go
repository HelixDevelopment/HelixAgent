package memory_test

import (
	"testing"

	adapter "dev.helix.agent/internal/adapters/memory"
	"github.com/stretchr/testify/assert"
)

func TestIsHelixMemoryEnabled(t *testing.T) {
	t.Parallel()
	// Value depends on build tag; just verify it returns a bool
	enabled := adapter.IsHelixMemoryEnabled()
	assert.IsType(t, true, enabled)
}

func TestMemoryBackendName(t *testing.T) {
	t.Parallel()
	name := adapter.MemoryBackendName()
	assert.NotEmpty(t, name)
	// Backend name must match the active build tag.
	if adapter.IsHelixMemoryEnabled() {
		assert.Contains(t, name, "digital.vasic.helixmemory")
	} else {
		assert.Contains(t, name, "digital.vasic.memory")
	}
}

func TestNewOptimalStoreAdapter_Default(t *testing.T) {
	t.Parallel()
	// Default build (no tags) should use HelixMemory
	if !adapter.IsHelixMemoryEnabled() {
		t.Skip("Skipping default test when nohelixmemory tag is active")  // SKIP-OK: #legacy-untriaged
	}
	store := adapter.NewOptimalStoreAdapter()
	assert.NotNil(t, store, "Default build must return a configured HelixMemory store")
	assert.Contains(t, adapter.MemoryBackendName(), "digital.vasic.helixmemory")
	assert.True(t, adapter.IsHelixMemoryEnabled())
}

func TestNewOptimalStoreAdapter_OptOut(t *testing.T) {
	t.Parallel()
	if adapter.IsHelixMemoryEnabled() {
		t.Skip("Skipping opt-out test when HelixMemory is active (default)")  // SKIP-OK: #legacy-untriaged
	}
	// Opt-out build (nohelixmemory tag) returns nil
	store := adapter.NewOptimalStoreAdapter()
	assert.Nil(t, store)
	assert.Equal(t, "digital.vasic.memory", adapter.MemoryBackendName())
	assert.False(t, adapter.IsHelixMemoryEnabled())
}

func TestHelixMemoryIsDefault(t *testing.T) {
	t.Parallel()
	// This test documents the architectural decision: HelixMemory IS the default.
	// Without any build tags, HelixMemory's unified cognitive engine is active.
	// To opt out, build with: go build -tags nohelixmemory
	//
	// When running with -tags nohelixmemory, this test is skipped (opt-out is valid).
	// When running with NO tags (the default), HelixMemory MUST be active.
	if !adapter.IsHelixMemoryEnabled() {
		t.Skip("HelixMemory is not active (nohelixmemory tag detected). " +  // SKIP-OK: #legacy-untriaged
			"This is expected when explicitly opting out. " +
			"Default builds (no tags) always use HelixMemory.")
	}
	assert.True(t, adapter.IsHelixMemoryEnabled())
	assert.Contains(t, adapter.MemoryBackendName(), "digital.vasic.helixmemory")

	// Verify the store adapter is fully functional
	store := adapter.NewOptimalStoreAdapter()
	assert.NotNil(t, store, "Default HelixMemory store must be initialized")
}
