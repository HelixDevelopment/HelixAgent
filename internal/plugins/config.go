package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/superagent/superagent/internal/utils"
)

// ConfigManager handles plugin configuration management
type ConfigManager struct {
	configDir string
	configs   map[string]map[string]interface{}
	mu        sync.RWMutex
}

func NewConfigManager(configDir string) *ConfigManager {
	return &ConfigManager{
		configDir: configDir,
		configs:   make(map[string]map[string]interface{}),
	}
}

func (c *ConfigManager) LoadPluginConfig(pluginName string) (map[string]interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	configPath := filepath.Join(c.configDir, pluginName+".json")

	// Check if already loaded
	if config, exists := c.configs[pluginName]; exists {
		return config, nil
	}

	// Load from file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default empty config
			defaultConfig := make(map[string]interface{})
			c.configs[pluginName] = defaultConfig
			return defaultConfig, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	c.configs[pluginName] = config
	utils.GetLogger().Infof("Loaded configuration for plugin %s", pluginName)
	return config, nil
}

func (c *ConfigManager) SavePluginConfig(pluginName string, config map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	configPath := filepath.Join(c.configDir, pluginName+".json")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	c.configs[pluginName] = config
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]map[string]interface{})
	for name, config := range c.configs {
		result[name] = config
	}
	return result
}
