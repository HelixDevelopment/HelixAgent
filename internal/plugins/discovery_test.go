package plugins

import (
	"fmt"
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

// =====================================================
// ADDITIONAL DISCOVERY TESTS FOR COMPREHENSIVE COVERAGE
// =====================================================

func TestDiscovery_ConcurrentDiscoverAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})
	paths := []string{tmpDir}

	discovery := NewDiscovery(loader, validator, paths)

	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func() {
			discovery.DiscoverAndLoad()
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	// Just verify no panic/deadlock
	assert.NotNil(t, discovery)
}

func TestDiscovery_DiscoverInPath_SymbolicLink(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	// Create a subdirectory with a file
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	soFile := filepath.Join(subDir, "plugin.so")
	err = os.WriteFile(soFile, []byte("fake"), 0644)
	require.NoError(t, err)

	// Create a symbolic link to the subdirectory
	linkPath := filepath.Join(tmpDir, "link")
	err = os.Symlink(subDir, linkPath)
	if err != nil {
		t.Skip("symbolic links not supported on this system")
	}

	// Should handle symbolic links gracefully
	err = discovery.discoverInPath(tmpDir)
	assert.NoError(t, err)
}

func TestDiscovery_DiscoverInPath_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping as root user")
	}

	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	// Create a directory with no read permission
	restrictedDir := filepath.Join(tmpDir, "restricted")
	err := os.MkdirAll(restrictedDir, 0000)
	require.NoError(t, err)
	defer os.Chmod(restrictedDir, 0755) // Restore for cleanup

	// Should handle permission errors gracefully
	err = discovery.discoverInPath(tmpDir)
	assert.NoError(t, err) // Main directory still accessible
}

func TestDiscovery_DiscoverAndLoad_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	// Create nested directories with .so files
	nested1 := filepath.Join(tmpDir, "level1", "level2", "level3")
	err := os.MkdirAll(nested1, 0755)
	require.NoError(t, err)

	// Create .so files at different levels
	os.WriteFile(filepath.Join(tmpDir, "root.so"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "level1.so"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(nested1, "deep.so"), []byte("fake"), 0644)

	// Should not error even if plugins fail to load
	err = discovery.DiscoverAndLoad()
	assert.NoError(t, err)
}

func TestDiscovery_LoadPlugin_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	err := discovery.loadPlugin("")
	assert.Error(t, err)
}

func TestDiscovery_LoadPlugin_DirectoryPath(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Try to load a directory as a plugin
	err = discovery.loadPlugin(subDir)
	assert.Error(t, err)
}

func TestDiscovery_DiscoverAndLoad_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	// Empty directory - should not error
	err := discovery.DiscoverAndLoad()
	assert.NoError(t, err)
}

func TestDiscovery_WatchForChanges_MultipleDirectories(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir1, tmpDir2})

	discovery := NewDiscovery(loader, validator, []string{tmpDir1, tmpDir2})

	// Start watching
	discovery.WatchForChanges()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Create files in both directories
	os.WriteFile(filepath.Join(tmpDir1, "plugin1.so"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(tmpDir2, "plugin2.so"), []byte("fake"), 0644)

	// Give it time to process
	time.Sleep(200 * time.Millisecond)
}

func TestDiscovery_DiscoverInPath_HiddenFiles(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	// Create hidden .so file
	hiddenFile := filepath.Join(tmpDir, ".hidden-plugin.so")
	err := os.WriteFile(hiddenFile, []byte("fake"), 0644)
	require.NoError(t, err)

	// Regular .so file
	regularFile := filepath.Join(tmpDir, "regular-plugin.so")
	err = os.WriteFile(regularFile, []byte("fake"), 0644)
	require.NoError(t, err)

	// Should handle hidden files
	err = discovery.discoverInPath(tmpDir)
	assert.NoError(t, err)
}

func TestDiscovery_Fields(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})
	paths := []string{tmpDir, "/opt/plugins"}

	discovery := NewDiscovery(loader, validator, paths)

	assert.Equal(t, loader, discovery.loader)
	assert.Equal(t, validator, discovery.validator)
	assert.Equal(t, paths, discovery.paths)
}

func TestDiscovery_NilLoader(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewSecurityValidator([]string{tmpDir})

	// Creating discovery with nil loader
	discovery := NewDiscovery(nil, validator, []string{tmpDir})

	assert.NotNil(t, discovery)
	assert.Nil(t, discovery.loader)

	// DiscoverAndLoad might panic or error with nil loader
	// Just verify construction works
}

func TestDiscovery_NilValidator(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)

	// Creating discovery with nil validator
	discovery := NewDiscovery(loader, nil, []string{tmpDir})

	assert.NotNil(t, discovery)
	assert.Nil(t, discovery.validator)
}

func TestDiscovery_EmptyPaths(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{})

	discovery := NewDiscovery(loader, validator, []string{})

	assert.NotNil(t, discovery)
	assert.Empty(t, discovery.paths)

	// DiscoverAndLoad with empty paths
	err := discovery.DiscoverAndLoad()
	assert.NoError(t, err)
}

func TestDiscovery_DiscoverInPath_LargeDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(registry)
	validator := NewSecurityValidator([]string{tmpDir})

	discovery := NewDiscovery(loader, validator, []string{tmpDir})

	// Create many files (mix of .so and other files)
	for i := 0; i < 100; i++ {
		if i%10 == 0 {
			os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("plugin%d.so", i)), []byte("fake"), 0644)
		} else {
			os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i)), []byte("text"), 0644)
		}
	}

	// Should handle large directories efficiently
	err := discovery.discoverInPath(tmpDir)
	assert.NoError(t, err)
}
