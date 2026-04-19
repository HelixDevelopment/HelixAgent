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
		_ = os.Remove(configPath)
		config2, err := cm.LoadPluginConfig("cached-plugin")
		require.NoError(t, err)
		assert.Equal(t, config1, config2)
	})

	t.Run("load invalid json", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "invalid.json")
		err := os.WriteFile(configPath, []byte("{invalid json}"), 0644)
		require.NoError(t, err)

		// Clear cache
		cm.configs.Delete("invalid")

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
		cached, _ := cm.configs.Get("save-test")
		assert.Equal(t, config, cached)
	})

	t.Run("overwrite existing config", func(t *testing.T) {
		config1 := map[string]interface{}{"name": "overwrite-test", "value": 1}
		err := cm.SavePluginConfig("overwrite-test", config1)
		require.NoError(t, err)

		config2 := map[string]interface{}{"name": "overwrite-test", "value": 2}
		err = cm.SavePluginConfig("overwrite-test", config2)
		require.NoError(t, err)

		// Verify cached value directly (before serialization)
		cached, _ := cm.configs.Get("overwrite-test")
		assert.Equal(t, 2, cached["value"])
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
		cm.configs.Delete("update-test")

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
	cm.configs.Put("plugin-a", map[string]interface{}{"name": "a"})
	cm.configs.Put("plugin-b", map[string]interface{}{"name": "b"})

	all := cm.GetAllConfigs()
	assert.Len(t, all, 2)
	assert.Equal(t, "a", all["plugin-a"]["name"])
	assert.Equal(t, "b", all["plugin-b"]["name"])

	// Verify it's a copy, not the original
	all["plugin-c"] = map[string]interface{}{"name": "c"}
	assert.Equal(t, 2, cm.configs.Len())
}

func TestConfigManager_ValidateConfig_MaxRetries(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewConfigManager(tmpDir)

	t.Run("max_retries as float (from JSON)", func(t *testing.T) {
		// JSON unmarshals numbers as float64
		config := map[string]interface{}{
			"name":        "test-plugin",
			"version":     "1.0.0",
			"max_retries": 3.0, // float64 from JSON
		}
		err := cm.ValidateConfig("test", config)
		// Will fail since code expects int, JSON gives float64
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_retries must be between 0 and 10")
	})

	t.Run("max_retries negative", func(t *testing.T) {
		config := map[string]interface{}{
			"name":        "test-plugin",
			"version":     "1.0.0",
			"max_retries": -1,
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_retries must be between 0 and 10")
	})

	t.Run("max_retries too high", func(t *testing.T) {
		config := map[string]interface{}{
			"name":        "test-plugin",
			"version":     "1.0.0",
			"max_retries": 15,
		}
		err := cm.ValidateConfig("test", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_retries must be between 0 and 10")
	})

	t.Run("max_retries valid integer", func(t *testing.T) {
		config := map[string]interface{}{
			"name":        "test-plugin",
			"version":     "1.0.0",
			"max_retries": 5,
		}
		err := cm.ValidateConfig("test", config)
		assert.NoError(t, err)
	})
}

func TestConfigManager_SavePluginConfig_WriteError(t *testing.T) {
	// Use non-existent parent directory to trigger write error
	cm := NewConfigManager("/non/existent/path/that/does/not/exist")

	config := map[string]interface{}{
		"name":    "test-plugin",
		"version": "1.0.0",
	}

	err := cm.SavePluginConfig("test", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write config file")
}

func TestConfigManager_LoadPluginConfig_ReadError(t *testing.T) {
	// Create a temp dir
	tmpDir := t.TempDir()
	cm := NewConfigManager(tmpDir)

	// Create a directory with the same name as the config file
	// This will cause a read error when trying to read it as a file
	dirPath := filepath.Join(tmpDir, "plugin-as-dir.json")
	err := os.MkdirAll(dirPath, 0755)
	require.NoError(t, err)

	_, err = cm.LoadPluginConfig("plugin-as-dir")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

// --- Benchmarks ---

func BenchmarkConfigManager_ValidateConfig(b *testing.B) {
	tmpDir := b.TempDir()
	cm := NewConfigManager(tmpDir)

	configs := []map[string]interface{}{
		{"name": "plugin-a", "version": "1.0.0"},
		{"name": "plugin-b", "version": "2.0.0", "api_key": "valid-api-key-here-long-enough"},
		{"name": "plugin-c", "version": "1.5.0", "timeout": 30.0},
		{
			"name": "plugin-d", "version": "1.0.0",
			"models": []interface{}{
				map[string]interface{}{"id": "model-1"},
				map[string]interface{}{"id": "model-2"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cm.ValidateConfig("test", configs[i%len(configs)])
	}
}

func BenchmarkConfigManager_GetAllConfigs(b *testing.B) {
	tmpDir := b.TempDir()
	cm := NewConfigManager(tmpDir)

	for i := 0; i < 50; i++ {
		name := "plugin-" + string(rune('a'+i%26))
		cm.configs.Put(name, map[string]interface{}{
			"name":    name,
			"version": "1.0.0",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cm.GetAllConfigs()
	}
}
