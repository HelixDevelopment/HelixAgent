package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/utils"
)

// ConfigManager handles plugin configuration management
type ConfigManager struct {
	configDir string
	configs   *safe.Store[string, map[string]interface{}]
}

func NewConfigManager(configDir string) *ConfigManager {
	return &ConfigManager{
		configDir: configDir,
		configs:   safe.NewStore[string, map[string]interface{}](),
	}
}

func (c *ConfigManager) LoadPluginConfig(pluginName string) (map[string]interface{}, error) {
	if !utils.ValidatePath(pluginName) {
		return nil, fmt.Errorf("invalid plugin name: %s", pluginName)
	}

	// Check if already loaded
	if config, exists := c.configs.Get(pluginName); exists {
		return config, nil
	}

	configPath := filepath.Join(c.configDir, pluginName+".json")

	// Load from file
	data, err := os.ReadFile(configPath) // #nosec G304 - plugin name validated with utils.ValidatePath
	if err != nil {
		if os.IsNotExist(err) {
			// Return default empty config, racing PutIfAbsent so concurrent
			// first-loaders converge on the same cached instance.
			defaultConfig := make(map[string]interface{})
			cached, _ := c.configs.PutIfAbsent(pluginName, defaultConfig)
			return cached, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cached, stored := c.configs.PutIfAbsent(pluginName, config)
	if stored {
		utils.GetLogger().Infof("Loaded configuration for plugin %s", pluginName)
	}
	return cached, nil
}

func (c *ConfigManager) SavePluginConfig(pluginName string, config map[string]interface{}) error {
	configPath := filepath.Join(c.configDir, pluginName+".json")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Use 0600 permissions as plugin configs may contain sensitive data like API keys
	if err := os.WriteFile(configPath, data, 0600); err != nil { // #nosec G306 - 0600 permissions appropriate for sensitive plugin configs
		return fmt.Errorf("failed to write config file: %w", err)
	}

	c.configs.Put(pluginName, config)
	utils.GetLogger().Infof("Saved configuration for plugin %s", pluginName)
	return nil
}

func (c *ConfigManager) UpdatePluginConfig(pluginName string, updates map[string]interface{}) error {
	current, err := c.LoadPluginConfig(pluginName)
	if err != nil {
		return err
	}

	// Merge updates
	for key, value := range updates {
		current[key] = value
	}

	return c.SavePluginConfig(pluginName, current)
}

func (c *ConfigManager) ValidateConfig(pluginName string, config map[string]interface{}) error {
	utils.GetLogger().Infof("Validating configuration for plugin %s", pluginName)

	// Basic structure validation
	if config == nil {
		return fmt.Errorf("plugin config cannot be nil")
	}

	// Validate common required fields
	if _, exists := config["name"]; !exists {
		return fmt.Errorf("plugin config missing required field: name")
	}

	if _, exists := config["version"]; !exists {
		return fmt.Errorf("plugin config missing required field: version")
	}

	// Validate API key if required
	if apiKey, exists := config["api_key"]; exists {
		if apiKeyStr, ok := apiKey.(string); !ok || len(apiKeyStr) < 10 {
			return fmt.Errorf("plugin api_key must be a string with at least 10 characters")
		}
	}

	// Validate timeout
	if timeout, exists := config["timeout"]; exists {
		if timeoutFloat, ok := timeout.(float64); !ok || timeoutFloat <= 0 || timeoutFloat > 300 {
			return fmt.Errorf("plugin timeout must be between 0 and 300 seconds")
		}
	}

	// Validate retry configuration
	if maxRetries, exists := config["max_retries"]; exists {
		if retryInt, ok := maxRetries.(int); !ok || retryInt < 0 || retryInt > 10 {
			return fmt.Errorf("plugin max_retries must be between 0 and 10")
		}
	}

	// Validate model configuration
	if models, exists := config["models"]; exists {
		modelsSlice, ok := models.([]interface{})
		if !ok {
			return fmt.Errorf("plugin models must be an array")
		}

		for i, model := range modelsSlice {
			if _, ok := model.(map[string]interface{}); !ok {
				return fmt.Errorf("plugin model at index %d must be an object", i)
			}
		}
	}

	utils.GetLogger().Infof("Configuration validation successful for plugin %s", pluginName)
	return nil
}

func (c *ConfigManager) GetAllConfigs() map[string]map[string]interface{} {
	return c.configs.Snapshot()
}
