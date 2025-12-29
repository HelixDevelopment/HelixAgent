package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// AIDebateConfigLoader handles loading and managing AI debate configurations
type AIDebateConfigLoader struct {
	configPath string
	config     *AIDebateConfig
	mu         sync.RWMutex
}

// NewAIDebateConfigLoader creates a new AI debate configuration loader
func NewAIDebateConfigLoader(configPath string) *AIDebateConfigLoader {
	return &AIDebateConfigLoader{
		configPath: configPath,
	}
}

// Load loads the AI debate configuration from file
func (l *AIDebateConfigLoader) Load() (*AIDebateConfig, error) {
	if l.configPath == "" {
		return nil, fmt.Errorf("configuration path is required")
	}

	// Check if file exists
	if _, err := os.Stat(l.configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file does not exist: %s", l.configPath)
	}

	// Read configuration file
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse YAML configuration
	var config AIDebateConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML configuration: %w", err)
	}

	// Substitute environment variables
	if err := l.substituteEnvVars(&config); err != nil {
		return nil, fmt.Errorf("failed to substitute environment variables: %w", err)
	}

	// Apply default values
	l.applyDefaults(&config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	l.mu.Lock()
	l.config = &config
	l.mu.Unlock()
	return &config, nil
}

// LoadFromString loads configuration from a YAML string
func (l *AIDebateConfigLoader) LoadFromString(yamlContent string) (*AIDebateConfig, error) {
	var config AIDebateConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML configuration: %w", err)
	}

	// Substitute environment variables
	if err := l.substituteEnvVars(&config); err != nil {
		return nil, fmt.Errorf("failed to substitute environment variables: %w", err)
	}

	// Apply default values
	l.applyDefaults(&config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	l.mu.Lock()
	l.config = &config
	l.mu.Unlock()
	return &config, nil
}

// GetConfig returns the loaded configuration
func (l *AIDebateConfigLoader) GetConfig() *AIDebateConfig {
	return l.config
}

// Reload reloads the configuration from file
func (l *AIDebateConfigLoader) Reload() (*AIDebateConfig, error) {
	return l.Load()
}

// substituteEnvVars replaces ${VAR_NAME} placeholders with environment variable values
func (l *AIDebateConfigLoader) substituteEnvVars(config *AIDebateConfig) error {
	// Substitute global configuration
	if config.CogneeConfig != nil && config.CogneeConfig.DatasetName != "" {
		config.CogneeConfig.DatasetName = os.ExpandEnv(config.CogneeConfig.DatasetName)
	}

	// Substitute participant configurations
	for i := range config.Participants {
		participant := &config.Participants[i]

		// Substitute LLM configurations
		for j := range participant.LLMs {
			llm := &participant.LLMs[j]
			if llm.APIKey != "" {
				llm.APIKey = os.ExpandEnv(llm.APIKey)
			}
			if llm.BaseURL != "" {
				llm.BaseURL = os.ExpandEnv(llm.BaseURL)
			}
			if llm.HealthCheckURL != "" {
				llm.HealthCheckURL = os.ExpandEnv(llm.HealthCheckURL)
			}

			// Substitute custom parameters
			for key, value := range llm.CustomParams {
				if str, ok := value.(string); ok {
					llm.CustomParams[key] = os.ExpandEnv(str)
				}
			}
		}

		// Substitute Cognee settings
		if participant.CogneeSettings != nil && participant.CogneeSettings.DatasetName != "" {
			participant.CogneeSettings.DatasetName = os.ExpandEnv(participant.CogneeSettings.DatasetName)
		}
	}

	return nil
}

// applyDefaults applies default values to the configuration
func (l *AIDebateConfigLoader) applyDefaults(config *AIDebateConfig) {
	// Apply global defaults
	if config.MaximalRepeatRounds == 0 {
		config.MaximalRepeatRounds = 3
	}
	if config.DebateTimeout == 0 {
		config.DebateTimeout = 300000 // 5 minutes in milliseconds
	}
	if config.ConsensusThreshold == 0 {
		config.ConsensusThreshold = 0.75
	}
	if config.QualityThreshold == 0 {
		config.QualityThreshold = 0.7
	}
	if config.MaxResponseTime == 0 {
		config.MaxResponseTime = 30000 // 30 seconds in milliseconds
	}
	if config.MaxContextLength == 0 {
		config.MaxContextLength = 32000
	}
	if config.MemoryRetention == 0 {
		config.MemoryRetention = 2592000000 // 30 days in milliseconds
	}
	if config.DebateStrategy == "" {
		config.DebateStrategy = "structured"
	}
	if config.VotingStrategy == "" {
		config.VotingStrategy = "confidence_weighted"
	}
	if config.ResponseFormat == "" {
		config.ResponseFormat = "detailed"
	}

	// Apply Cognee defaults - enable by default
	if !config.EnableCognee {
		config.EnableCognee = true
	}
	if config.EnableCognee && config.CogneeConfig == nil {
		config.CogneeConfig = &CogneeDebateConfig{
			Enabled:             true,
			EnhanceResponses:    true,
			AnalyzeConsensus:    true,
			GenerateInsights:    true,
			DatasetName:         "ai_debate_enhancement",
			MaxEnhancementTime:  10000, // 10 seconds in milliseconds in milliseconds
			EnhancementStrategy: "hybrid",
			MemoryIntegration:   true,
			ContextualAnalysis:  true,
		}
	}

	// Apply participant defaults
	for i := range config.Participants {
		participant := &config.Participants[i]

		if participant.ResponseTimeout == 0 {
			participant.ResponseTimeout = 30000 // 30 seconds in milliseconds
		}
		if participant.Weight == 0 {
			participant.Weight = 1.0
		}
		if participant.Priority == 0 {
			participant.Priority = 1
		}
		if participant.QualityThreshold == 0 {
			participant.QualityThreshold = config.QualityThreshold
		}
		if participant.MinResponseLength == 0 {
			participant.MinResponseLength = 50
		}
		if participant.MaxResponseLength == 0 {
			participant.MaxResponseLength = 2000
		}
		if participant.DebateStyle == "" {
			participant.DebateStyle = "balanced"
		}
		if participant.ArgumentationStyle == "" {
			participant.ArgumentationStyle = "logical"
		}
		if participant.PersuasionLevel == 0 {
			participant.PersuasionLevel = 0.5
		}
		if participant.OpennessToChange == 0 {
			participant.OpennessToChange = 0.5
		}

		// Apply Cognee participant defaults - enable by default
		if !participant.EnableCognee {
			participant.EnableCognee = true
		}

		// Apply LLM defaults
		for j := range participant.LLMs {
			llm := &participant.LLMs[j]
			if llm.Timeout == 0 {
				llm.Timeout = 30000 // 30 seconds in milliseconds
			}
			if llm.MaxRetries == 0 {
				llm.MaxRetries = 3
			}
			if llm.Temperature == 0 {
				llm.Temperature = 0.7
			}
			if llm.MaxTokens == 0 {
				llm.MaxTokens = 1000
			}
			if llm.Weight == 0 {
				llm.Weight = 1.0
			}
			if llm.RateLimitRPS == 0 {
				llm.RateLimitRPS = 10
			}
			if llm.RequestTimeout == 0 {
				llm.RequestTimeout = llm.Timeout
			}
			if llm.HealthCheckInterval == 0 {
				llm.HealthCheckInterval = 60000 // 60 seconds in milliseconds
			}
		}

		// Apply Cognee participant defaults
		if participant.EnableCognee && participant.CogneeSettings == nil {
			participant.CogneeSettings = &CogneeParticipantConfig{
				EnhanceResponses: true,
				AnalyzeSentiment: true,
				ExtractEntities:  true,
				GenerateSummary:  true,
				DatasetName:      fmt.Sprintf("participant_%s_enhancement", strings.ToLower(participant.Name)),
			}
		}
	}
}

// Save saves the configuration to file
func (l *AIDebateConfigLoader) Save(config *AIDebateConfig) error {
	if l.configPath == "" {
		return fmt.Errorf("configuration path is required")
	}

	// Validate configuration before saving
	if err := config.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(l.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create configuration directory: %w", err)
	}

	// Marshal configuration to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to file
	if err := os.WriteFile(l.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	l.config = config
	return nil
}

// GetDefaultConfig returns a default AI debate configuration
func GetDefaultConfig() *AIDebateConfig {
	return &AIDebateConfig{
		Enabled:             true,
		MaximalRepeatRounds: 3,
		DebateTimeout:       300000, // 5 minutes in milliseconds
		ConsensusThreshold:  0.75,
		EnableCognee:        true,
		CogneeConfig: &CogneeDebateConfig{
			Enabled:             true,
			EnhanceResponses:    true,
			AnalyzeConsensus:    true,
			GenerateInsights:    true,
			DatasetName:         "ai_debate_enhancement",
			MaxEnhancementTime:  10 * 1000, // 10 seconds
			EnhancementStrategy: "hybrid",
			MemoryIntegration:   true,
			ContextualAnalysis:  true,
		},
		Participants: []DebateParticipant{
			{
				Name:        "Strongest",
				Role:        "Primary Analyst",
				Description: "Main analytical participant with comprehensive reasoning capabilities",
				Enabled:     true,
				LLMs: []LLMConfiguration{
					{
						Name:        "Primary LLM",
						Provider:    "claude",
						Model:       "claude-3-5-sonnet-20241022",
						Enabled:     true,
						APIKey:      "${CLAUDE_API_KEY}",
						Timeout:     30000, // 30 seconds in milliseconds
						MaxRetries:  3,
						Temperature: 0.1,
						MaxTokens:   2000,
						Weight:      1.0,
					},
					{
						Name:        "Fallback LLM 1",
						Provider:    "deepseek",
						Model:       "deepseek-coder",
						Enabled:     true,
						APIKey:      "${DEEPSEEK_API_KEY}",
						Timeout:     25000, // 25 seconds in milliseconds
						MaxRetries:  3,
						Temperature: 0.1,
						MaxTokens:   2000,
						Weight:      0.9,
					},
					{
						Name:        "Fallback LLM 2",
						Provider:    "gemini",
						Model:       "gemini-2.0-flash-exp",
						Enabled:     true,
						APIKey:      "${GEMINI_API_KEY}",
						Timeout:     35000, // 35 seconds in milliseconds
						MaxRetries:  3,
						Temperature: 0.1,
						MaxTokens:   2000,
						Weight:      0.8,
					},
				},
				ResponseTimeout:    30000, // 30 seconds in milliseconds
				Weight:             1.5,
				Priority:           1,
				DebateStyle:        "analytical",
				ArgumentationStyle: "logical",
				PersuasionLevel:    0.8,
				OpennessToChange:   0.3,
				QualityThreshold:   0.8,
				MinResponseLength:  100,
				MaxResponseLength:  2000,
				EnableCognee:       true,
			},
			{
				Name:                "Middle One",
				Role:                "Balanced Analyst",
				Description:         "Balanced participant with moderate reasoning capabilities",
				Enabled:             true,
				MaximalRepeatRounds: intPtr(2),
				LLMs: []LLMConfiguration{
					{
						Name:        "Primary LLM",
						Provider:    "qwen",
						Model:       "qwen-turbo",
						Enabled:     true,
						APIKey:      "${QWEN_API_KEY}",
						Timeout:     25000, // 25 seconds in milliseconds
						MaxRetries:  3,
						Temperature: 0.3,
						MaxTokens:   1500,
						Weight:      1.0,
					},
					{
						Name:        "Fallback LLM",
						Provider:    "ollama",
						Model:       "llama2",
						Enabled:     true,
						BaseURL:     "http://localhost:11434",
						Timeout:     20000, // 20 seconds in milliseconds
						MaxRetries:  2,
						Temperature: 0.3,
						MaxTokens:   1500,
						Weight:      0.7,
					},
				},
				ResponseTimeout:    25000, // 25 seconds in milliseconds
				Weight:             1.0,
				Priority:           2,
				DebateStyle:        "balanced",
				ArgumentationStyle: "evidence_based",
				PersuasionLevel:    0.5,
				OpennessToChange:   0.6,
				QualityThreshold:   0.7,
				MinResponseLength:  80,
				MaxResponseLength:  1500,
				EnableCognee:       true,
			},
			{
				Name:        "Creative Thinker",
				Role:        "Creative Analyst",
				Description: "Creative participant with innovative thinking capabilities",
				Enabled:     true,
				LLMs: []LLMConfiguration{
					{
						Name:        "Primary LLM",
						Provider:    "openrouter",
						Model:       "x-ai/grok-4",
						Enabled:     true,
						APIKey:      "${OPENROUTER_API_KEY}",
						Timeout:     35000, // 35 seconds in milliseconds
						MaxRetries:  3,
						Temperature: 0.7,
						MaxTokens:   1800,
						Weight:      1.0,
					},
					{
						Name:        "Fallback LLM",
						Provider:    "zai",
						Model:       "zai-large",
						Enabled:     true,
						APIKey:      "${ZAI_API_KEY}",
						Timeout:     30000, // 30 seconds in milliseconds
						MaxRetries:  3,
						Temperature: 0.7,
						MaxTokens:   1800,
						Weight:      0.8,
					},
				},
				ResponseTimeout:    35000, // 35 seconds in milliseconds
				Weight:             1.2,
				Priority:           3,
				DebateStyle:        "creative",
				ArgumentationStyle: "hypothetical",
				PersuasionLevel:    0.7,
				OpennessToChange:   0.8,
				QualityThreshold:   0.6,
				MinResponseLength:  120,
				MaxResponseLength:  1800,
				EnableCognee:       true,
			},
		},
		DebateStrategy:      "structured",
		VotingStrategy:      "confidence_weighted",
		ResponseFormat:      "detailed",
		EnableMemory:        true,
		MemoryRetention:     30 * 24 * 60 * 60 * 1000, // 30 days
		MaxContextLength:    32000,
		QualityThreshold:    0.7,
		MaxResponseTime:     30 * 1000, // 30 seconds
		EnableStreaming:     false,
		EnableDebateLogging: true,
		LogDebateDetails:    true,
		MetricsEnabled:      true,
	}
}

// intPtr is a helper function to create a pointer to an int
func intPtr(i int) *int {
	return &i
}
