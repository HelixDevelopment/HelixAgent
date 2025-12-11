package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// MultiProviderConfig represents the full multi-provider configuration
type MultiProviderConfig struct {
	Server    MultiServerConfig       `yaml:"server"`
	Database  DatabaseConfig          `yaml:"database"`
	Redis     RedisConfig             `yaml:"redis"`
	Providers map[string]*LLMProvider `yaml:"providers"`
	Ensemble  *MultiEnsembleConfig    `yaml:"ensemble"`
	OpenAI    *OpenAIConfig           `yaml:"openai_compatible"`
	MCP       *MCPConfig              `yaml:"mcp"`
	LSP       *LSPConfig              `yaml:"lsp"`
	Memory    *MemoryConfig           `yaml:"memory"`
	Logging   *LoggingConfig          `yaml:"logging"`
	Metrics   *MultiMetricsConfig     `yaml:"metrics"`
}

// MultiServerConfig holds server configuration
type MultiServerConfig struct {
	Host    string        `yaml:"host"`
	Port    int           `yaml:"port"`
	Debug   bool          `yaml:"debug"`
	Timeout time.Duration `yaml:"timeout"`
}

// LLMProvider holds provider-specific configuration
type LLMProvider struct {
	Name           string                 `yaml:"name"`
	Type           string                 `yaml:"type"`
	Enabled        bool                   `yaml:"enabled"`
	APIKey         string                 `yaml:"api_key"`
	BaseURL        string                 `yaml:"base_url"`
	Timeout        time.Duration          `yaml:"timeout"`
	MaxRetries     int                    `yaml:"max_retries"`
	Weight         float64                `yaml:"weight"`
	Tags           []string               `yaml:"tags"`
	Capabilities   map[string]string      `yaml:"capabilities"`
	CustomSettings map[string]interface{} `yaml:"custom_settings"`
	Models         []ModelConfig          `yaml:"models"`
}

// ModelConfig holds model-specific configuration
type ModelConfig struct {
	ID           string                 `yaml:"id"`
	Name         string                 `yaml:"name"`
	Enabled      bool                   `yaml:"enabled"`
	Weight       float64                `yaml:"weight"`
	Capabilities []string               `yaml:"capabilities"`
	CustomParams map[string]interface{} `yaml:"custom_params"`
}

// MultiEnsembleConfig holds ensemble configuration
type MultiEnsembleConfig struct {
	Strategy            string             `yaml:"strategy"`
	MinProviders        int                `yaml:"min_providers"`
	MaxProviders        int                `yaml:"max_providers"`
	ConfidenceThreshold float64            `yaml:"confidence_threshold"`
	FallbackToBest      bool               `yaml:"fallback_to_best"`
	Timeout             time.Duration      `yaml:"timeout"`
	PreferredProviders  []string           `yaml:"preferred_providers"`
	ProviderWeights     map[string]float64 `yaml:"provider_weights"`
}

// OpenAIConfig holds OpenAI compatibility configuration
type OpenAIConfig struct {
	Enabled           bool   `yaml:"enabled"`
	BasePath          string `yaml:"base_path"`
	ExposeAllModels   bool   `yaml:"expose_all_models"`
	EnsembleModelName string `yaml:"ensemble_model_name"`
	EnableStreaming   bool   `yaml:"enable_streaming"`
	EnableFunctions   bool   `yaml:"enable_functions"`
	EnableTools       bool   `yaml:"enable_tools"`
}

// MCPConfig holds Model Context Protocol configuration
type MCPConfig struct {
	Enabled              bool `yaml:"enabled"`
	ExposeAllTools       bool `yaml:"expose_all_tools"`
	UnifiedToolNamespace bool `yaml:"unified_tool_namespace"`
}

// LSPConfig holds Language Server Protocol configuration
type LSPConfig struct {
	Enabled               bool `yaml:"enabled"`
	ExposeAllCapabilities bool `yaml:"expose_all_capabilities"`
}

// MemoryConfig holds memory configuration
type MemoryConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Provider         string `yaml:"provider"`
	MaxContextLength int    `yaml:"max_context_length"`
	RetentionDays    int    `yaml:"retention_days"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level                 string `yaml:"level"`
	Format                string `yaml:"format"`
	EnableRequestLogging  bool   `yaml:"enable_request_logging"`
	EnableResponseLogging bool   `yaml:"enable_response_logging"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Prometheus *PrometheusConfig `yaml:"prometheus"`
}

// MultiMetricsConfig holds metrics configuration
type MultiMetricsConfig struct {
	Enabled    bool                   `yaml:"enabled"`
	Prometheus *MultiPrometheusConfig `yaml:"prometheus"`
}

// MultiPrometheusConfig holds Prometheus-specific configuration
type MultiPrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

// LoadMultiProviderConfig loads configuration from YAML file
func LoadMultiProviderConfig(configPath string) (*MultiProviderConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config MultiProviderConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Substitute environment variables
	if err := substituteEnvVars(&config); err != nil {
		return nil, fmt.Errorf("failed to substitute environment variables: %w", err)
	}

	return &config, nil
}

// substituteEnvVars replaces ${VAR_NAME} placeholders with environment variable values
func substituteEnvVars(config *MultiProviderConfig) error {
	// Substitute provider API keys from environment
	for name, provider := range config.Providers {
		if provider.APIKey != "" {
			provider.APIKey = os.ExpandEnv(provider.APIKey)
		}
		if provider.BaseURL != "" {
			provider.BaseURL = os.ExpandEnv(provider.BaseURL)
		}

		// Substitute model-specific custom parameters
		for i := range provider.Models {
			for key, value := range provider.Models[i].CustomParams {
				if str, ok := value.(string); ok {
					provider.Models[i].CustomParams[key] = os.ExpandEnv(str)
				}
			}
		}

		config.Providers[name] = provider
	}

	// Substitute other environment variables
	config.Database.Host = os.ExpandEnv(config.Database.Host)
	config.Database.User = os.ExpandEnv(config.Database.User)
	config.Database.Password = os.ExpandEnv(config.Database.Password)
	config.Database.Name = os.ExpandEnv(config.Database.Name)

	config.Redis.Host = os.ExpandEnv(config.Redis.Host)
	config.Redis.Password = os.ExpandEnv(config.Redis.Password)

	return nil
}
