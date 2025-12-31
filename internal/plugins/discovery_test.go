package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiscovery(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{"/tmp"})
	paths := []string{"/tmp/plugins", "/opt/plugins"}

	discovery := NewDiscovery(loader, validator, paths)

	require.NotNil(t, discovery)
	assert.Equal(t, loader, discovery.loader)
	assert.Equal(t, validator, discovery.validator)
	assert.Equal(t, paths, discovery.paths)
}

func TestDiscovery_DiscoverAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})
	paths := []string{tmpDir}

	discovery := NewDiscovery(loader, validator, paths)

	t.Run("discover in empty directory", func(t *testing.T) {
		err := discovery.DiscoverAndLoad()
		// Should not error even if no plugins found
		assert.NoError(t, err)
	})

	t.Run("discover skips non-so files", func(t *testing.T) {
		// Create a non-plugin file
		err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("test"), 0644)
		require.NoError(t, err)

		err = discovery.DiscoverAndLoad()
		assert.NoError(t, err)
	})
}

func TestDiscovery_DiscoverInPath(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	t.Run("discover in non-existent path", func(t *testing.T) {
		err := discovery.discoverInPath("/non-existent-path")
		// Should return error for non-existent path
		assert.Error(t, err)
	})

	t.Run("discover in valid directory", func(t *testing.T) {
		err := discovery.discoverInPath(tmpDir)
		assert.NoError(t, err)
	})

	t.Run("discover with subdirectories", func(t *testing.T) {
		subDir := filepath.Join(tmpDir, "subdir")
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		err = discovery.discoverInPath(tmpDir)
		assert.NoError(t, err)
	})
}

func TestDiscovery_OnPluginChange(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	t.Run("plugin change notification", func(t *testing.T) {
		// This will fail to load (no actual .so file) but shouldn't panic
		discovery.onPluginChange(filepath.Join(tmpDir, "test.so"))
	})
}

func TestDiscovery_LoadPlugin_SecurityValidation(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	// Validator only allows /other/path
	validator := NewSecurityValidator([]string{"/other/path"})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	t.Run("reject plugin outside allowed paths", func(t *testing.T) {
		pluginPath := filepath.Join(tmpDir, "plugin.so")
		err := discovery.loadPlugin(pluginPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "security validation failed")
	})
}

func TestDiscovery_LoadPlugin_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	t.Run("attempt to load non-existent plugin", func(t *testing.T) {
		pluginPath := filepath.Join(tmpDir, "nonexistent.so")
		err := discovery.loadPlugin(pluginPath)
		// Will fail because file doesn't exist, but security check passes
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load plugin")
	})
}
