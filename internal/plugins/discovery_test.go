package plugins

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

// =====================================================
// ADDITIONAL DISCOVERY TESTS FOR COVERAGE
// =====================================================

func TestDiscovery_WatchForChanges(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	t.Run("watch for changes starts watcher", func(t *testing.T) {
		// This should not panic
		// WatchForChanges runs in background, so we just verify it starts
		discovery.WatchForChanges()

		// Give it time to start
		time.Sleep(100 * time.Millisecond)
	})
}

func TestDiscovery_WatchForChanges_InvalidPath(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{"/tmp"})

	discovery := NewDiscovery(loader, validator, []string{"/nonexistent/path/that/does/not/exist"})

	t.Run("watch for changes with invalid path", func(t *testing.T) {
		// Should not panic, just log an error
		discovery.WatchForChanges()

		// Give it time to try
		time.Sleep(100 * time.Millisecond)
	})
}

func TestDiscovery_DiscoverAndLoad_MultipleDirectories(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir1, tmpDir2})

	discovery := NewDiscovery(loader, validator, []string{tmpDir1, tmpDir2})

	t.Run("discover in multiple directories", func(t *testing.T) {
		// Create .so files in both directories
		err := os.WriteFile(filepath.Join(tmpDir1, "plugin1.so"), []byte("fake"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir2, "plugin2.so"), []byte("fake"), 0644)
		require.NoError(t, err)

		// Should not error even if plugins fail to load
		err = discovery.DiscoverAndLoad()
		assert.NoError(t, err)
	})
}

func TestDiscovery_DiscoverInPath_WithSoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	t.Run("discover .so files", func(t *testing.T) {
		// Create a .so file
		soFile := filepath.Join(tmpDir, "test.so")
		err := os.WriteFile(soFile, []byte("fake plugin"), 0644)
		require.NoError(t, err)

		// Should not error even if loading fails
		err = discovery.discoverInPath(tmpDir)
		assert.NoError(t, err)
	})
}

func TestDiscovery_OnPluginChange_LoadError(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	// Validator that won't allow the path
	validator := NewSecurityValidator([]string{"/other/path"})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	t.Run("plugin change with security error", func(t *testing.T) {
		// Should not panic, just log error
		discovery.onPluginChange(filepath.Join(tmpDir, "bad-plugin.so"))
	})
}

func TestDiscovery_OnPluginChange_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	t.Run("plugin change notification for valid path", func(t *testing.T) {
		// Create an actual file
		soFile := filepath.Join(tmpDir, "valid-plugin.so")
		err := os.WriteFile(soFile, []byte("fake"), 0644)
		require.NoError(t, err)

		// Should not panic
		discovery.onPluginChange(soFile)
	})
}
