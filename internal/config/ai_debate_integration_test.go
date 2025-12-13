package config

import (
	"os"
	"path/filepath"
	"testing"
	"strings"
)

func TestAIDebateConfigLoader_Integration(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "ai-debate-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("Load valid configuration file", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "valid-config.yaml")
		yamlContent := `
enabled: true
maximal_repeat_rounds: 3
debate_timeout: 300000
consensus_threshold: 0.75
max_response_time: 30000
max_context_length: 32000
quality_threshold: 0.7
enable_cognee: true
cognee_config:
  enabled: true
  dataset_name: "test_dataset"
  max_enhancement_time: 10000
  enhancement_strategy: "hybrid"
  enhance_responses: true
  analyze_consensus: true
  generate_insights: true
participants:
  - name: "TestParticipant1"
    role: "Analyst"
    enabled: true
    llms:
      - name: "Primary LLM"
        provider: "claude"
        model: "claude-3-sonnet"
        enabled: true
        api_key: "test-key"
        timeout: 30000
        max_tokens: 1000
        temperature: 0.7
    response_timeout: 30000
    weight: 1.0
    priority: 1
    debate_style: "analytical"
    argumentation_style: "logical"
    persuasion_level: 0.5
    openness_to_change: 0.5
    quality_threshold: 0.7
    min_response_length: 50
    max_response_length: 1000
  - name: "TestParticipant2"
    role: "Critic"
    enabled: true
    llms:
      - name: "Primary LLM"
        provider: "deepseek"
        model: "deepseek-coder"
        enabled: true
        api_key: "test-key-2"
        timeout: 30000
        max_tokens: 1000
        temperature: 0.7
    response_timeout: 30000
    weight: 1.0
    priority: 2
    debate_style: "critical"
    argumentation_style: "logical"
    persuasion_level: 0.5
    openness_to_change: 0.5
    quality_threshold: 0.7
    min_response_length: 50
    max_response_length: 1000
debate_strategy: "structured"
voting_strategy: "confidence_weighted"
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		loader := NewAIDebateConfigLoader(configPath)
		config, err := loader.Load()
		
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		
		if config == nil {
			t.Fatal("Load() returned nil config")
		}
		
		// Verify configuration
		if !config.Enabled {
			t.Error("Expected config to be enabled")
		}
		if config.MaximalRepeatRounds != 3 {
			t.Errorf("Expected MaximalRepeatRounds 3, got %d", config.MaximalRepeatRounds)
		}
		if len(config.Participants) != 2 {
			t.Errorf("Expected 2 participants, got %d", len(config.Participants))
		}
		if config.Participants[0].Name != "TestParticipant1" {
			t.Errorf("Expected first participant name 'TestParticipant1', got %s", config.Participants[0].Name)
		}
	})

	t.Run("Load non-existent file", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "non-existent.yaml")
		loader := NewAIDebateConfigLoader(configPath)
		_, err := loader.Load()
		
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Expected error about file not existing, got: %v", err)
		}
	})

	t.Run("Load invalid YAML file", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "invalid.yaml")
		invalidYAML := "invalid yaml content: ["
		if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		loader := NewAIDebateConfigLoader(configPath)
		_, err := loader.Load()
		
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
		if !strings.Contains(err.Error(), "failed to parse YAML") {
			t.Errorf("Expected YAML parsing error, got: %v", err)
		}
	})

	t.Run("Save and reload configuration", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "save-test.yaml")
		originalConfig := &AIDebateConfig{
			Enabled:             true,
			MaximalRepeatRounds: 5,
			DebateTimeout:       600000,
			ConsensusThreshold:  0.8,
			MaxResponseTime:     30000,
			MaxContextLength:    32000,
			QualityThreshold:    0.7,
			Participants: []DebateParticipant{
				{
					Name:               "SaveTestParticipant",
					Role:               "Test Analyst",
					Enabled:            true,
					LLMs: []LLMConfiguration{
						{
							Name:     "SaveTest LLM",
							Provider: "claude",
							Model:    "claude-3-sonnet",
							Enabled:  true,
							Timeout:  30000,
							MaxTokens: 1000,
							Temperature: 0.7,
						},
					},
					ResponseTimeout:    30000,
					Weight:             1.0,
					Priority:           1,
					DebateStyle:        "analytical",
					ArgumentationStyle: "logical",
					PersuasionLevel:    0.5,
					OpennessToChange:   0.5,
					QualityThreshold:   0.7,
					MinResponseLength:  50,
					MaxResponseLength:  1000,
				},
t			{
					Name:               "SaveTestParticipant2",
					Role:               "Test Critic",
					Enabled:            true,
					LLMs: []LLMConfiguration{
						{
							Name:     "SaveTest LLM2",
							Provider: "deepseek",
							Model:    "deepseek-coder",
							Enabled:  true,
							Timeout:  30000,
							MaxTokens: 1000,
							Temperature: 0.7,
					},
				},
				ResponseTimeout:    30000,
				Weight:             1.0,
				Priority:           2,
				DebateStyle:        "critical",
				ArgumentationStyle: "logical",
				PersuasionLevel:    0.5,
				OpennessToChange:   0.5,
				QualityThreshold:   0.7,
				MinResponseLength:  50,
				MaxResponseLength:  1000,
			},
			},
			DebateStrategy: "structured",
			VotingStrategy: "confidence_weighted",
		}

		loader := NewAIDebateConfigLoader(configPath)
		
		// Save configuration
		if err := loader.Save(originalConfig); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		
		// Reload configuration
		loadedConfig, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() after Save() error = %v", err)
		}
		
		// Verify loaded configuration matches original
		if loadedConfig.MaximalRepeatRounds != originalConfig.MaximalRepeatRounds {
			t.Errorf("Expected MaximalRepeatRounds %d, got %d", 
				originalConfig.MaximalRepeatRounds, loadedConfig.MaximalRepeatRounds)
		}
		if loadedConfig.ConsensusThreshold != originalConfig.ConsensusThreshold {
			t.Errorf("Expected ConsensusThreshold %f, got %f", 
				originalConfig.ConsensusThreshold, loadedConfig.ConsensusThreshold)
		}
		if len(loadedConfig.Participants) != len(originalConfig.Participants) {
			t.Errorf("Expected %d participants, got %d", 
				len(originalConfig.Participants), len(loadedConfig.Participants))
		}
	})

	t.Run("Configuration with environment variables", func(t *testing.T) {
		// Set environment variables
		os.Setenv("TEST_CLAUDE_KEY", "claude-test-key")
		os.Setenv("TEST_DEEPSEEK_KEY", "deepseek-test-key")
		os.Setenv("TEST_DATASET", "test-dataset")
		defer func() {
			os.Unsetenv("TEST_CLAUDE_KEY")
			os.Unsetenv("TEST_DEEPSEEK_KEY")
			os.Unsetenv("TEST_DATASET")
		}()

		configPath := filepath.Join(tempDir, "env-test.yaml")
		yamlContent := `
enabled: true
maximal_repeat_rounds: 3
debate_timeout: 300000
consensus_threshold: 0.75
max_response_time: 30000
max_context_length: 32000
quality_threshold: 0.7
enable_cognee: true
cognee_config:
  enabled: true
  dataset_name: "${TEST_DATASET}"
participants:
  - name: "TestParticipant1"
    role: "Analyst"
    enabled: true
    llms:
      - name: "Primary LLM"
        provider: "claude"
        model: "claude-3-sonnet"
        enabled: true
        api_key: "${TEST_CLAUDE_KEY}"
        timeout: 30000
        max_tokens: 1000
        temperature: 0.7
    response_timeout: 30000
    weight: 1.0
    priority: 1
    debate_style: "analytical"
    argumentation_style: "logical"
    persuasion_level: 0.5
    openness_to_change: 0.5
    quality_threshold: 0.7
    min_response_length: 50
    max_response_length: 1000
    enable_cognee: true
    cognee_settings:
      dataset_name: "${TEST_DATASET}_participant"
  - name: "TestParticipant2"
    role: "Critic"
    enabled: true
    llms:
      - name: "Primary LLM"
        provider: "deepseek"
        model: "deepseek-coder"
        enabled: true
        api_key: "${TEST_DEEPSEEK_KEY}"
        timeout: 30000
        max_tokens: 1000
        temperature: 0.7
    response_timeout: 30000
    weight: 1.0
    priority: 2
    debate_style: "critical"
    argumentation_style: "logical"
    persuasion_level: 0.5
    openness_to_change: 0.5
    quality_threshold: 0.7
    min_response_length: 50
    max_response_length: 1000
debate_strategy: "structured"
voting_strategy: "confidence_weighted"
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		loader := NewAIDebateConfigLoader(configPath)
		config, err := loader.Load()
		
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Verify environment variable substitution
		if config.CogneeConfig.DatasetName != "test-dataset" {
			t.Errorf("Expected CogneeConfig.DatasetName 'test-dataset', got %s", config.CogneeConfig.DatasetName)
		}

		if config.Participants[0].LLMs[0].APIKey != "claude-test-key" {
			t.Errorf("Expected first participant LLM APIKey 'claude-test-key', got %s", config.Participants[0].LLMs[0].APIKey)
		}

		if config.Participants[1].LLMs[0].APIKey != "deepseek-test-key" {
			t.Errorf("Expected second participant LLM APIKey 'deepseek-test-key', got %s", config.Participants[1].LLMs[0].APIKey)
		}

		if config.Participants[0].CogneeSettings.DatasetName != "test-dataset_participant" {
			t.Errorf("Expected participant CogneeSettings.DatasetName 'test-dataset_participant', got %s", config.Participants[0].CogneeSettings.DatasetName)
		}
	})
}

func TestAIDebateConfigLoader_Defaults(t *testing.T) {
	minimalYAML := `
enabled: true
maximal_repeat_rounds: 3
debate_timeout: 300000
consensus_threshold: 0.75
max_response_time: 30000
max_context_length: 32000
quality_threshold: 0.7
participants:
  - name: "MinimalParticipant"
    role: "Analyst"
    enabled: true
    llms:
      - name: "MinimalLLM"
        provider: "claude"
        model: "claude-3"
        enabled: true
        timeout: 30000
        max_tokens: 1000
        temperature: 0.7
    response_timeout: 30000
    weight: 1.0
    priority: 1
    debate_style: "analytical"
    argumentation_style: "logical"
    persuasion_level: 0.5
    openness_to_change: 0.5
    quality_threshold: 0.7
    min_response_length: 50
    max_response_length: 1000
`

	loader := NewAIDebateConfigLoader("")
	config, err := loader.LoadFromString(minimalYAML)
	
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	// Test global defaults
	if config.MaximalRepeatRounds != 3 {
		t.Errorf("Expected default MaximalRepeatRounds 3, got %d", config.MaximalRepeatRounds)
	}
	if config.DebateTimeout != 5*60*1000 {
		t.Errorf("Expected default DebateTimeout %d, got %d", 5*60*1000, config.DebateTimeout)
	}
	if config.ConsensusThreshold != 0.75 {
		t.Errorf("Expected default ConsensusThreshold 0.75, got %f", config.ConsensusThreshold)
	}
	if config.QualityThreshold != 0.7 {
		t.Errorf("Expected default QualityThreshold 0.7, got %f", config.QualityThreshold)
	}
	if config.MaxResponseTime != 30*1000 {
		t.Errorf("Expected default MaxResponseTime %d, got %d", 30*1000, config.MaxResponseTime)
	}
	if config.MaxContextLength != 32000 {
		t.Errorf("Expected default MaxContextLength 32000, got %d", config.MaxContextLength)
	}
	if config.DebateStrategy != "structured" {
		t.Errorf("Expected default DebateStrategy 'structured', got %s", config.DebateStrategy)
	}
	if config.VotingStrategy != "confidence_weighted" {
		t.Errorf("Expected default VotingStrategy 'confidence_weighted', got %s", config.VotingStrategy)
	}
	if config.ResponseFormat != "detailed" {
		t.Errorf("Expected default ResponseFormat 'detailed', got %s", config.ResponseFormat)
	}

	// Test participant defaults
	participant := config.Participants[0]
	if participant.ResponseTimeout != 30*1000 {
		t.Errorf("Expected default ResponseTimeout %d, got %d", 30*1000, participant.ResponseTimeout)
	}
	if participant.Weight != 1.0 {
		t.Errorf("Expected default Weight 1.0, got %f", participant.Weight)
	}
	if participant.Priority != 1 {
		t.Errorf("Expected default Priority 1, got %d", participant.Priority)
	}
	if participant.QualityThreshold != config.QualityThreshold {
		t.Errorf("Expected default QualityThreshold %f, got %f", config.QualityThreshold, participant.QualityThreshold)
	}
	if participant.MinResponseLength != 50 {
		t.Errorf("Expected default MinResponseLength 50, got %d", participant.MinResponseLength)
	}
	if participant.MaxResponseLength != 2000 {
		t.Errorf("Expected default MaxResponseLength 2000, got %d", participant.MaxResponseLength)
	}
	if participant.DebateStyle != "balanced" {
		t.Errorf("Expected default DebateStyle 'balanced', got %s", participant.DebateStyle)
	}
	if participant.ArgumentationStyle != "logical" {
		t.Errorf("Expected default ArgumentationStyle 'logical', got %s", participant.ArgumentationStyle)
	}
	if participant.PersuasionLevel != 0.5 {
		t.Errorf("Expected default PersuasionLevel 0.5, got %f", participant.PersuasionLevel)
	}
	if participant.OpennessToChange != 0.5 {
		t.Errorf("Expected default OpennessToChange 0.5, got %f", participant.OpennessToChange)
	}

	// Test LLM defaults
	llm := participant.LLMs[0]
	if llm.Timeout != 30*1000 {
		t.Errorf("Expected default LLM Timeout %d, got %d", 30*1000, llm.Timeout)
	}
	if llm.MaxRetries != 3 {
		t.Errorf("Expected default LLM MaxRetries 3, got %d", llm.MaxRetries)
	}
	if llm.Temperature != 0.7 {
		t.Errorf("Expected default LLM Temperature 0.7, got %f", llm.Temperature)
	}
	if llm.MaxTokens != 1000 {
		t.Errorf("Expected default LLM MaxTokens 1000, got %d", llm.MaxTokens)
	}
	if llm.Weight != 1.0 {
		t.Errorf("Expected default LLM Weight 1.0, got %f", llm.Weight)
	}
	if llm.RateLimitRPS != 10 {
		t.Errorf("Expected default LLM RateLimitRPS 10, got %d", llm.RateLimitRPS)
	}
	if llm.RequestTimeout != llm.Timeout {
		t.Errorf("Expected default LLM RequestTimeout to equal Timeout %d, got %d", llm.Timeout, llm.RequestTimeout)
	}
	if llm.HealthCheckInterval != 60*1000 {
		t.Errorf("Expected default LLM HealthCheckInterval %d, got %d", 60*1000, llm.HealthCheckInterval)
	}

	// Test Cognee defaults
	if !config.EnableCognee {
		t.Error("Expected Cognee to be enabled by default when not specified")
	}
	if config.CogneeConfig == nil {
		t.Fatal("Expected CogneeConfig to be created with defaults")
	}
	if config.CogneeConfig.DatasetName != "ai_debate_enhancement" {
		t.Errorf("Expected default CogneeConfig DatasetName 'ai_debate_enhancement', got %s", config.CogneeConfig.DatasetName)
	}
	if config.CogneeConfig.MaxEnhancementTime != 10*1000 {
		t.Errorf("Expected default CogneeConfig MaxEnhancementTime %d, got %d", 10*1000, config.CogneeConfig.MaxEnhancementTime)
	}
	if config.CogneeConfig.EnhancementStrategy != "hybrid" {
		t.Errorf("Expected default CogneeConfig EnhancementStrategy 'hybrid', got %s", config.CogneeConfig.EnhancementStrategy)
	}

	// Test participant Cognee defaults
	if !participant.EnableCognee {
		t.Error("Expected participant Cognee to be enabled by default when not specified")
	}
	if participant.CogneeSettings == nil {
		t.Fatal("Expected participant CogneeSettings to be created with defaults")
	}
	if participant.CogneeSettings.DatasetName != "participant_minimalparticipant_enhancement" {
		t.Errorf("Expected default CogneeSettings DatasetName 'participant_minimalparticipant_enhancement', got %s", participant.CogneeSettings.DatasetName)
	}
}

func TestAIDebateConfigLoader_ChaosTesting(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		description string
	}{
		{
			name: "Empty configuration",
			yamlContent: "",
			description: "Should handle completely empty configuration",
		},
		{
			name: "Only comments",
			yamlContent: `# This is a comment
# Another comment
# No actual configuration`,
			description: "Should handle configuration with only comments",
		},
		{
			name: "Malformed YAML",
			yamlContent: `
enabled: true
participants:
  - name: "Test"
    role: "Analyst"
    enabled: true
    llms:
      - name: "LLM"
        provider: "claude"
        model: "claude-3"
        enabled: true
        timeout: 30000
        max_tokens: 1000
        temperature: 0.7
    # Missing required fields
`,
			description: "Should handle malformed YAML gracefully",
		},
		{
			name: "Extremely large values",
			yamlContent: `
enabled: true
maximal_repeat_rounds: 1000
debate_timeout: 999999999999999999
consensus_threshold: 0.99
participants:
  - name: "Test"
    role: "Analyst"
    enabled: true
    maximal_repeat_rounds: 500
    response_timeout: 999999999999999999
    weight: 999.9
    priority: 999999
    persuasion_level: 0.99
    openness_to_change: 0.99
    quality_threshold: 0.99
    min_response_length: 999999
    max_response_length: 999999999
    llms:
      - name: "LLM"
        provider: "claude"
        model: "claude-3"
        enabled: true
        timeout: 999999999999999999
        max_tokens: 999999999
        temperature: 1.9
        top_p: 0.99
        frequency_penalty: 1.9
        presence_penalty: 1.9
        weight: 999.9
        rate_limit_rps: 999999
        request_timeout: 999999999999999999
`,
			description: "Should handle extremely large values",
		},
		{
			name: "Unicode and special characters",
			yamlContent: `
enabled: true
participants:
  - name: "测试参与者"
    role: "分析师🔍"
    description: "这是一个测试参与者 with emojis 🚀 and special chars: àáâãäåæçèéêë"
    enabled: true
    llms:
      - name: "测试模型"
        provider: "claude"
        model: "claude-3-sonnet"
        enabled: true
        timeout: 30000
        max_tokens: 1000
        temperature: 0.7
        custom_params:
          unicode_test: "测试文本 with emojis 🎉"
          special_chars: "café, naïve, résumé"
          math_symbols: "∑∏∫∆∇≤≥≠≈"
    response_timeout: 30000
    weight: 1.0
    priority: 1
    debate_style: "analytical"
    argumentation_style: "logical"
    persuasion_level: 0.5
    openness_to_change: 0.5
    quality_threshold: 0.7
    min_response_length: 50
    max_response_length: 1000
`,
			description: "Should handle Unicode and special characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewAIDebateConfigLoader("")
			_, err := loader.LoadFromString(tt.yamlContent)
			
			// For chaos testing, we just want to ensure the system doesn't crash
			// and provides meaningful error messages when appropriate
			t.Logf("Chaos test '%s' (%s): err = %v", tt.name, tt.description, err)
		})
	}
}

func TestAIDebateConfigLoader_ConcurrentAccess(t *testing.T) {
	// Create a valid configuration
	yamlContent := `
enabled: true
maximal_repeat_rounds: 3
debate_timeout: 300000
consensus_threshold: 0.75
max_response_time: 30000
max_context_length: 32000
quality_threshold: 0.7
participants:
  - name: "TestParticipant"
    role: "Analyst"
    enabled: true
    llms:
      - name: "Test LLM"
        provider: "claude"
        model: "claude-3-sonnet"
        enabled: true
        timeout: 30000
        max_tokens: 1000
        temperature: 0.7
    response_timeout: 30000
    weight: 1.0
    priority: 1
    debate_style: "analytical"
    argumentation_style: "logical"
    persuasion_level: 0.5
    openness_to_change: 0.5
    quality_threshold: 0.7
    min_response_length: 50
    max_response_length: 1000
`

	loader := NewAIDebateConfigLoader("")
	
	// Test concurrent loading
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			config, err := loader.LoadFromString(yamlContent)
			if err != nil {
				t.Errorf("Goroutine %d: LoadFromString() error = %v", id, err)
			}
			if config == nil {
				t.Errorf("Goroutine %d: LoadFromString() returned nil config", id)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestAIDebateConfigLoader_SaveValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "save-validation-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		config      *AIDebateConfig
		expectError bool
		errMsg      string
	}{
		{
			name: "Valid configuration",
			config: &AIDebateConfig{
				Enabled:             true,
				MaximalRepeatRounds: 3,
				DebateTimeout:       300000,
				ConsensusThreshold:  0.75,
				MaxResponseTime:     30000,
				MaxContextLength:    32000,
				QualityThreshold:    0.7,
				DebateStrategy:      "structured",
				VotingStrategy:      "confidence_weighted",
				Participants: []DebateParticipant{
					{
						Name:               "ValidParticipant",
						Role:               "Analyst",
						Enabled:            true,
						LLMs: []LLMConfiguration{
							{
								Name:     "ValidLLM",
								Provider: "claude",
								Model:    "claude-3",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout:    30000,
						Weight:             1.0,
						Priority:           1,
						DebateStyle:        "analytical",
						ArgumentationStyle: "logical",
						PersuasionLevel:    0.5,
						OpennessToChange:   0.5,
						QualityThreshold:   0.7,
						MinResponseLength:  50,
						MaxResponseLength:  1000,
					},
					{
						Name:               "ValidParticipant2",
						Role:               "Critic",
						Enabled:            true,
						LLMs: []LLMConfiguration{
							{
								Name:     "ValidLLM2",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout:    30000,
						Weight:             1.0,
						Priority:           2,
						DebateStyle:        "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel:    0.5,
						OpennessToChange:   0.5,
						QualityThreshold:   0.7,
						MinResponseLength:  50,
						MaxResponseLength:  1000,
					},
					{
						Name:               "SaveTestParticipant2",
						Role:               "Test Critic",
						Enabled:            true,
						LLMs: []LLMConfiguration{
							{
								Name:     "SaveTest LLM2",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout:    30000,
						Weight:             1.0,
						Priority:           2,
						DebateStyle:        "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel:    0.5,
						OpennessToChange:   0.5,
						QualityThreshold:   0.7,
						MinResponseLength:  50,
						MaxResponseLength:  1000,
					},
					{
						Name:               "SaveTestParticipant2",
						Role:               "Test Critic",
						Enabled:            true,
						LLMs: []LLMConfiguration{
							{
								Name:     "SaveTest LLM2",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout:    30000,
						Weight:             1.0,
						Priority:           2,
						DebateStyle:        "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel:    0.5,
						OpennessToChange:   0.5,
						QualityThreshold:   0.7,
						MinResponseLength:  50,
						MaxResponseLength:  1000,
					},
					{
						Name:               "SaveTestParticipant2",
						Role:               "Test Critic", 
						Enabled:            true,
						LLMs: []LLMConfiguration{
							{
								Name:     "SaveTest LLM2",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout:    30000,
						Weight:             1.0,
						Priority:           2,
						DebateStyle:        "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel:    0.5,
						OpennessToChange:   0.5,
						QualityThreshold:   0.7,
						MinResponseLength:  50,
						MaxResponseLength:  1000,
					},
					{
						Name:               "SaveTestParticipant2",
						Role:               "Test Critic",
						Enabled:            true,
						LLMs: []LLMConfiguration{
							{
								Name:     "SaveTest LLM2",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout:    30000,
						Weight:             1.0,
						Priority:           2,
						DebateStyle:        "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel:    0.5,
						OpennessToChange:   0.5,
						QualityThreshold:   0.7,
						MinResponseLength:  50,
						MaxResponseLength:  1000,
					},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid configuration - validation should fail",
			config: &AIDebateConfig{
				Enabled:             true,
				MaximalRepeatRounds: 0, // Invalid
				Participants: []DebateParticipant{
					{
						Name:    "InvalidParticipant",
						Role:    "Analyst",
						Enabled: true,
						LLMs:    []LLMConfiguration{}, // Invalid - no LLMs
					},
				},
			},
			expectError: true,
			errMsg:      "configuration validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tempDir, tt.name+".yaml")
			loader := NewAIDebateConfigLoader(configPath)
			
			err := loader.Save(tt.config)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				} else {
					// Verify file was created
					if _, err := os.Stat(configPath); os.IsNotExist(err) {
						t.Error("Configuration file was not created")
					}
				}
			}
		})
	}
}