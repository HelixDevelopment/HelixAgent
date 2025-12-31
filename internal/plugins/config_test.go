package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigManager(t *testing.T) {
	cm := NewConfigManager("/tmp/plugins")
	require.NotNil(t, cm)
	assert.Equal(t, "/tmp/plugins", cm.configDir)
	assert.NotNil(t, cm.configs)
}

func TestConfigManager_LoadPluginConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConfigManager(tmpDir)

	t.Run("load non-existent config returns empty", func(t *testing.T) {
		config, err := cm.LoadPluginConfig("non-existent")
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Len(t, config, 0)
	})

	t.Run("load valid config file", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "valid-plugin.json")
		content := `{"name": "valid-plugin", "version": "1.0.0", "enabled": true}`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := cm.LoadPluginConfig("valid-plugin")
		require.NoError(t, err)
		assert.Equal(t, "valid-plugin", config["name"])
		assert.Equal(t, "1.0.0", config["version"])
		assert.Equal(t, true, config["enabled"])
	})

	t.Run("cached config returned on subsequent load", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "cached-plugin.json")
		content := `{"name": "cached-plugin"}`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config1, err := cm.LoadPluginConfig("cached-plugin")
		require.NoError(t, err)

		// Delete file and load again - should return cached
		os.Remove(configPath)
		config2, err := cm.LoadPluginConfig("cached-plugin")
		require.NoError(t, err)
		assert.Equal(t, config1, config2)
	})

	t.Run("load invalid json", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "invalid.json")
		err := os.WriteFile(configPath, []byte("{invalid json}"), 0644)
		require.NoError(t, err)

		// Clear cache
		delete(cm.configs, "invalid")

		_, err = cm.LoadPluginConfig("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse config")
	})
}

func TestConfigManager_SavePluginConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConfigManager(tmpDir)

	t.Run("save new config", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "save-test",
			"version": "1.0.0",
			"enabled": true,
		}

		err := cm.SavePluginConfig("save-test", config)
		require.NoError(t, err)

		// Verify file was created
		configPath := filepath.Join(tmpDir, "save-test.json")
		_, err = os.Stat(configPath)
		assert.NoError(t, err)

		// Verify cached
		assert.Equal(t, config, cm.configs["save-test"])
	})

	t.Run("overwrite existing config", func(t *testing.T) {
		config1 := map[string]interface{}{"name": "overwrite-test", "value": 1}
		err := cm.SavePluginConfig("overwrite-test", config1)
		require.NoError(t, err)

		config2 := map[string]interface{}{"name": "overwrite-test", "value": 2}
		err = cm.SavePluginConfig("overwrite-test", config2)
		require.NoError(t, err)

		// Verify cached value directly (before serialization)
		assert.Equal(t, 2, cm.configs["overwrite-test"]["value"])
	})
}

func TestConfigManager_UpdatePluginConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConfigManager(tmpDir)

	t.Run("update existing config", func(t *testing.T) {
		// Create initial config
		initial := map[string]interface{}{
			"name":    "update-test",
			"version": "1.0.0",
			"setting": "old",
		}
		err := cm.SavePluginConfig("update-test", initial)
		require.NoError(t, err)

		// Update
		updates := map[string]interface{}{
			"setting":    "new",
			"newSetting": "added",
		}
		err = cm.UpdatePluginConfig("update-test", updates)
		require.NoError(t, err)

		// Clear cache to force reload
		delete(cm.configs, "update-test")

		// Verify merged config
		loaded, err := cm.LoadPluginConfig("update-test")
		require.NoError(t, err)
		assert.Equal(t, "update-test", loaded["name"])
		assert.Equal(t, "1.0.0", loaded["version"])
		assert.Equal(t, "new", loaded["setting"])
		assert.Equal(t, "added", loaded["newSetting"])
	})

	t.Run("update non-existent creates new", func(t *testing.T) {
		updates := map[string]interface{}{
			"name":    "new-plugin",
			"version": "0.0.1",
		}
		err := cm.UpdatePluginConfig("new-from-update", updates)
		require.NoError(t, err)

		config, err := cm.LoadPluginConfig("new-from-update")
		require.NoError(t, err)
		assert.Equal(t, "new-plugin", config["name"])
	})
}

func TestConfigManager_ValidateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConfigManager(tmpDir)

	t.Run("nil config", func(t *testing.T) {
		err := cm.ValidateConfig("test", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("missing name field", func(t *testing.T) {
		config := map[string]interface{}{
			"version": "1.0.0",
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required field: name")
	})

	t.Run("missing version field", func(t *testing.T) {
		config := map[string]interface{}{
			"name": "test-plugin",
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required field: version")
	})

	t.Run("valid minimal config", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
		}
		err := cm.ValidateConfig("test", config)
		assert.NoError(t, err)
	})

	t.Run("api_key too short", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
			"api_key": "short",
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api_key must be a string with at least 10 characters")
	})

	t.Run("api_key valid", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
			"api_key": "valid-api-key-here",
		}
		err := cm.ValidateConfig("test", config)
		assert.NoError(t, err)
	})

	t.Run("timeout invalid - zero", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
			"timeout": 0.0,
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout must be between 0 and 300")
	})

	t.Run("timeout invalid - too high", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
			"timeout": 500.0,
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout must be between 0 and 300")
	})

	t.Run("timeout valid", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
			"timeout": 30.0,
		}
		err := cm.ValidateConfig("test", config)
		assert.NoError(t, err)
	})

	t.Run("models not an array", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
			"models":  "not-an-array",
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "models must be an array")
	})

	t.Run("model not an object", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
			"models":  []interface{}{"not-an-object"},
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model at index 0 must be an object")
	})

	t.Run("models valid", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-plugin",
			"version": "1.0.0",
			"models": []interface{}{
				map[string]interface{}{"id": "model-1"},
				map[string]interface{}{"id": "model-2"},
			},
		}
		err := cm.ValidateConfig("test", config)
		assert.NoError(t, err)
	})
}

func TestConfigManager_GetAllConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConfigManager(tmpDir)

	// Add some configs
	cm.configs["plugin-a"] = map[string]interface{}{"name": "a"}
	cm.configs["plugin-b"] = map[string]interface{}{"name": "b"}

	all := cm.GetAllConfigs()
	assert.Len(t, all, 2)
	assert.Equal(t, "a", all["plugin-a"]["name"])
	assert.Equal(t, "b", all["plugin-b"]["name"])

	// Verify it's a copy, not the original
	all["plugin-c"] = map[string]interface{}{"name": "c"}
	assert.Len(t, cm.configs, 2)
}
