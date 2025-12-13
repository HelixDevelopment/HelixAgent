package config

import (
	"os"
	"testing"
	"strings"
)

func TestAIDebateConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *AIDebateConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid configuration",
			config: &AIDebateConfig{
				Enabled:             true,
				MaximalRepeatRounds: 3,
				DebateTimeout:       5 * 60 * 1000,
				ConsensusThreshold:  0.75,
				QualityThreshold:    0.7,
				MaxResponseTime:     30000,
				MaxContextLength:    32000,
				Participants: []DebateParticipant{
					{
						Name:               "Participant1",
						Role:               "Analyst",
						Enabled:            true,
						LLMs: []LLMConfiguration{
							{
								Name:     "Primary LLM",
								Provider: "claude",
								Model:    "claude-3-sonnet",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight:          1.0,
						Priority:        1,
						DebateStyle:     "analytical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
					{
						Name:               "Participant2",
						Role:               "Critic",
						Enabled:            true,
						LLMs: []LLMConfiguration{
							{
								Name:     "Primary LLM",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight:          1.0,
						Priority:        2,
						DebateStyle:     "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
				},
				DebateStrategy: "structured",
				VotingStrategy: "confidence_weighted",
				ResponseFormat: "detailed",
			},
			wantErr: false,
		},
		{
			name: "Disabled configuration",
			config: &AIDebateConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "Invalid maximal repeat rounds - too low",
			config: &AIDebateConfig{
				Enabled:             true,
				MaximalRepeatRounds: 0,
				DebateTimeout:       300000,
				ConsensusThreshold:  0.75,
				MaxResponseTime:     30000,
				MaxContextLength:    32000,
				QualityThreshold:    0.7,
				Participants: []DebateParticipant{
					{
						Name: "Participant1",
						Role: "Analyst",
						Enabled: true,
						LLMs: []LLMConfiguration{
							{
								Name:     "LLM1",
								Provider: "claude",
								Model:    "claude-3",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight: 1.0,
						Priority: 1,
						DebateStyle: "analytical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
					{
						Name: "Participant2",
						Role: "Critic",
						Enabled: true,
						LLMs: []LLMConfiguration{
							{
								Name:     "LLM2",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight: 1.0,
						Priority: 2,
						DebateStyle: "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
				},
			},
			wantErr: true,
		errMsg:  "maximal_repeat_rounds must be at least 1",
		},
		{
			name: "Invalid consensus threshold",
			config: &AIDebateConfig{
				Enabled:             true,
				MaximalRepeatRounds: 3,
				DebateTimeout:       300000,
				ConsensusThreshold:  1.5, // Invalid: > 1.0
				MaxResponseTime:     30000,
				MaxContextLength:    32000,
				QualityThreshold:    0.7,
				Participants: []DebateParticipant{
					{
						Name: "Participant1",
						Role: "Analyst",
						Enabled: true,
						LLMs: []LLMConfiguration{
							{
								Name:     "LLM1",
								Provider: "claude",
								Model:    "claude-3",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight: 1.0,
						Priority: 1,
						DebateStyle: "analytical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
					{
						Name: "Participant2",
						Role: "Critic",
						Enabled: true,
						LLMs: []LLMConfiguration{
							{
								Name:     "LLM2",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight: 1.0,
						Priority: 2,
						DebateStyle: "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
				},
			},
			wantErr: true,
		errMsg:  "consensus_threshold must be between 0.0 and 1.0",
		},
		{
			name: "Too few participants",
			config: &AIDebateConfig{
				Enabled:             true,
				MaximalRepeatRounds: 3,
				DebateTimeout:       300000,
				ConsensusThreshold:  0.75,
				MaxResponseTime:     30000,
				MaxContextLength:    32000,
				QualityThreshold:    0.7,
				Participants: []DebateParticipant{
					{
						Name: "Participant1",
						Role: "Analyst",
						Enabled: true,
						LLMs: []LLMConfiguration{
							{
								Name:     "LLM1",
								Provider: "claude",
								Model:    "claude-3",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight: 1.0,
						Priority: 1,
						DebateStyle: "analytical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
				},
			},
			wantErr: true,
		errMsg:  "at least 2 participants are required",
		},
		{
			name: "Duplicate participant names",
			config: &AIDebateConfig{
				Enabled:             true,
				MaximalRepeatRounds: 3,
				DebateTimeout:       300000,
				ConsensusThreshold:  0.75,
				MaxResponseTime:     30000,
				MaxContextLength:    32000,
				QualityThreshold:    0.7,
				Participants: []DebateParticipant{
					{
						Name: "Participant1",
						Role: "Analyst",
						Enabled: true,
						LLMs: []LLMConfiguration{
							{
								Name:     "LLM1",
								Provider: "claude",
								Model:    "claude-3",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight: 1.0,
						Priority: 1,
						DebateStyle: "analytical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
					{
						Name: "Participant1", // Duplicate name
						Role: "Critic",
						Enabled: true,
						LLMs: []LLMConfiguration{
							{
								Name:     "LLM2",
								Provider: "deepseek",
								Model:    "deepseek-coder",
								Enabled:  true,
								Timeout:  30000,
								MaxTokens: 1000,
								Temperature: 0.7,
							},
						},
						ResponseTimeout: 30000,
						Weight: 1.0,
						Priority: 2,
						DebateStyle: "critical",
						ArgumentationStyle: "logical",
						PersuasionLevel: 0.5,
						OpennessToChange: 0.5,
						QualityThreshold: 0.7,
						MinResponseLength: 50,
						MaxResponseLength: 1000,
					},
				},
			},
			wantErr: true,
		errMsg:  "duplicate participant name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestDebateParticipant_Validate(t *testing.T) {
	tests := []struct {
		name          string
		participant   DebateParticipant
		globalMaxRounds int
		wantErr       bool
		errMsg        string
	}{
		{
			name: "Valid participant",
			participant: DebateParticipant{
				Name:               "TestParticipant",
				Role:               "Analyst",
				Enabled:            true,
				LLMs: []LLMConfiguration{
					{
						Name:     "Primary LLM",
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
			globalMaxRounds: 3,
			wantErr:         false,
		},
		{
			name: "Disabled participant",
			participant: DebateParticipant{
				Name:    "DisabledParticipant",
				Role:    "Analyst",
				Enabled: false,
			},
			globalMaxRounds: 3,
			wantErr:         false,
		},
		{
			name: "Missing name",
			participant: DebateParticipant{
				Role:    "Analyst",
				Enabled: true,
			},
			globalMaxRounds: 3,
			wantErr:         true,
			errMsg:          "participant name is required",
		},
		{
			name: "Missing role",
			participant: DebateParticipant{
				Name:    "TestParticipant",
				Enabled: true,
			},
			globalMaxRounds: 3,
			wantErr:         true,
			errMsg:          "participant role is required",
		},
		{
			name: "No LLMs configured",
			participant: DebateParticipant{
				Name:               "TestParticipant",
				Role:               "Analyst",
				Enabled:            true,
				LLMs:               []LLMConfiguration{},
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
			globalMaxRounds: 3,
			wantErr:         true,
			errMsg:          "must have at least one LLM configuration",
		},
		{
			name: "No enabled LLMs",
			participant: DebateParticipant{
				Name:    "TestParticipant",
				Role:    "Analyst",
				Enabled: true,
				LLMs: []LLMConfiguration{
					{
						Name:     "Disabled LLM",
						Provider: "claude",
						Model:    "claude-3",
						Enabled:  false,
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
			globalMaxRounds: 3,
			wantErr:         true,
			errMsg:          "must have at least one enabled LLM",
		},
		{
			name: "Invalid maximal repeat rounds",
			participant: DebateParticipant{
				Name:               "TestParticipant",
				Role:               "Analyst",
				Enabled:            true,
				MaximalRepeatRounds: intPtr(0),
				LLMs: []LLMConfiguration{
					{
						Name:     "Primary LLM",
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
			globalMaxRounds: 3,
			wantErr:         true,
			errMsg:          "maximal_repeat_rounds must be at least 1",
		},
		{
			name: "Invalid debate style",
			participant: DebateParticipant{
				Name:               "TestParticipant",
				Role:               "Analyst",
				Enabled:            true,
				LLMs: []LLMConfiguration{
					{
						Name:     "Primary LLM",
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
				DebateStyle:        "invalid_style",
				ArgumentationStyle: "logical",
				PersuasionLevel:    0.5,
				OpennessToChange:   0.5,
				QualityThreshold:   0.7,
				MinResponseLength:  50,
				MaxResponseLength:  1000,
			},
			globalMaxRounds: 3,
			wantErr:         true,
			errMsg:          "invalid debate_style",
		},
		{
			name: "Invalid persuasion level",
			participant: DebateParticipant{
				Name:               "TestParticipant",
				Role:               "Analyst",
				Enabled:            true,
				LLMs: []LLMConfiguration{
					{
						Name:     "Primary LLM",
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
				PersuasionLevel:    1.5, // Invalid: > 1.0
				OpennessToChange:   0.5,
				QualityThreshold:   0.7,
				MinResponseLength:  50,
				MaxResponseLength:  1000,
			},
			globalMaxRounds: 3,
			wantErr:         true,
			errMsg:          "persuasion_level must be between 0.0 and 1.0",
		},
		{
			name: "Min response length > max response length",
			participant: DebateParticipant{
				Name:               "TestParticipant",
				Role:               "Analyst",
				Enabled:            true,
				LLMs: []LLMConfiguration{
					{
						Name:     "Primary LLM",
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
				MinResponseLength:  1000, // > max
				MaxResponseLength:  500,
			},
			globalMaxRounds: 3,
			wantErr:         true,
			errMsg:          "cannot be greater than max_response_length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.participant.Validate(tt.globalMaxRounds)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestLLMConfiguration_Validate(t *testing.T) {
	tests := []struct {
		name string
		llm  LLMConfiguration
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid LLM configuration",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Provider: "claude",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxRetries: 3,
				Temperature: 0.7,
				MaxTokens: 1000,
				Weight: 1.0,
			},
			wantErr: false,
		},
		{
			name: "Disabled LLM",
			llm: LLMConfiguration{
				Name:     "Disabled LLM",
				Provider: "claude",
				Model:    "claude-3-sonnet",
				Enabled:  false,
			},
			wantErr: false,
		},
		{
			name: "Missing name",
			llm: LLMConfiguration{
				Provider: "claude",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 1000,
				Temperature: 0.7,
			},
			wantErr: true,
			errMsg:  "LLM name is required",
		},
		{
			name: "Missing provider",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 1000,
				Temperature: 0.7,
			},
			wantErr: true,
			errMsg:  "LLM provider is required",
		},
		{
			name: "Missing model",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Provider: "claude",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 1000,
				Temperature: 0.7,
			},
			wantErr: true,
			errMsg:  "LLM model is required",
		},
		{
			name: "Invalid provider",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Provider: "invalid_provider",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 1000,
				Temperature: 0.7,
			},
			wantErr: true,
			errMsg:  "invalid provider",
		},
		{
			name: "Invalid temperature - too high",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Provider: "claude",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 1000,
				Temperature: 2.5, // > 2.0
			},
			wantErr: true,
			errMsg:  "temperature must be between 0.0 and 2.0",
		},
		{
			name: "Invalid max tokens",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Provider: "claude",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 0, // <= 0
				Temperature: 0.7,
			},
			wantErr: true,
			errMsg:  "max_tokens must be positive",
		},
		{
			name: "Invalid top_p",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Provider: "claude",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 1000,
				Temperature: 0.7,
				TopP: 1.5, // > 1.0
			},
			wantErr: true,
			errMsg:  "top_p must be between 0.0 and 1.0",
		},
		{
			name: "Invalid frequency penalty",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Provider: "claude",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 1000,
				Temperature: 0.7,
				FrequencyPenalty: 2.5, // > 2.0
			},
			wantErr: true,
			errMsg:  "frequency_penalty must be between -2.0 and 2.0",
		},
		{
			name: "Invalid weight - negative",
			llm: LLMConfiguration{
				Name:     "Test LLM",
				Provider: "claude",
				Model:    "claude-3-sonnet",
				Enabled:  true,
				Timeout:  30000,
				MaxTokens: 1000,
				Temperature: 0.7,
				Weight: -1.0, // < 0
			},
			wantErr: true,
			errMsg:  "weight must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.llm.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestAIDebateConfigLoader_LoadFromString(t *testing.T) {
	validYAML := `
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
  - name: "TestParticipant2"
    role: "Critic"
    enabled: true
    llms:
      - name: "Test LLM2"
        provider: "deepseek"
        model: "deepseek-coder"
        enabled: true
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

	tests := []struct {
		name        string
		yamlContent string
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "Valid YAML configuration",
			yamlContent: validYAML,
			wantErr:     false,
		},
		{
			name:        "Invalid YAML syntax",
			yamlContent: "invalid yaml content: [",
			wantErr:     true,
			errMsg:      "failed to parse YAML",
		},
		{
			name: "Invalid configuration values",
			yamlContent: `
enabled: true
maximal_repeat_rounds: 0  # Invalid: too low
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
`,
			wantErr: true,
			errMsg:  "configuration validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewAIDebateConfigLoader("")
			config, err := loader.LoadFromString(tt.yamlContent)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("LoadFromString() error = %v, want error containing %v", err, tt.errMsg)
			}
			
			if !tt.wantErr && config == nil {
				t.Error("LoadFromString() returned nil config without error")
			}
		})
	}
}

func TestAIDebateConfigLoader_EnvironmentSubstitution(t *testing.T) {
	// Set environment variables
	os.Setenv("TEST_API_KEY", "test-api-key-value")
	os.Setenv("TEST_BASE_URL", "https://test.api.com")
	os.Setenv("TEST_DATASET", "test-dataset")
	defer func() {
		os.Unsetenv("TEST_API_KEY")
		os.Unsetenv("TEST_BASE_URL")
		os.Unsetenv("TEST_DATASET")
	}()

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
  max_enhancement_time: 10000
  enhancement_strategy: "hybrid"
  enhance_responses: true
  analyze_consensus: true
  generate_insights: true
participants:
  - name: "TestParticipant"
    role: "Analyst"
    enabled: true
    llms:
      - name: "Test LLM"
        provider: "claude"
        model: "claude-3-sonnet"
        enabled: true
        api_key: "${TEST_API_KEY}"
        base_url: "${TEST_BASE_URL}"
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
      enhance_responses: true
      analyze_sentiment: true
      extract_entities: true
      generate_summary: true
      dataset_name: "${TEST_DATASET}_participant"
  - name: "TestParticipant2"
    role: "Critic"
    enabled: true
    llms:
      - name: "Test LLM2"
        provider: "deepseek"
        model: "deepseek-coder"
        enabled: true
        api_key: "test-api-key-2"
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

	loader := NewAIDebateConfigLoader("")
	config, err := loader.LoadFromString(yamlContent)
	
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	// Verify environment variable substitution
	if config.CogneeConfig.DatasetName != "test-dataset" {
		t.Errorf("Expected CogneeConfig.DatasetName 'test-dataset', got %s", config.CogneeConfig.DatasetName)
	}

	participant := config.Participants[0]
	llm := participant.LLMs[0]
	if llm.APIKey != "test-api-key-value" {
		t.Errorf("Expected LLM APIKey 'test-api-key-value', got %s", llm.APIKey)
	}
	if llm.BaseURL != "https://test.api.com" {
		t.Errorf("Expected LLM BaseURL 'https://test.api.com', got %s", llm.BaseURL)
	}
	if participant.CogneeSettings.DatasetName != "test-dataset_participant" {
		t.Errorf("Expected CogneeSettings.DatasetName 'test-dataset_participant', got %s", participant.CogneeSettings.DatasetName)
	}
}

func TestDebateParticipant_GetEffectiveMaxRounds(t *testing.T) {
	tests := []struct {
		name            string
		participant     DebateParticipant
		globalMaxRounds int
		want            int
	}{
		{
			name: "Use participant-specific rounds",
			participant: DebateParticipant{
				MaximalRepeatRounds: intPtr(5),
			},
			globalMaxRounds: 3,
			want:           5,
		},
		{
			name: "Use global rounds when participant-specific not set",
			participant: DebateParticipant{
				MaximalRepeatRounds: nil,
			},
			globalMaxRounds: 3,
			want:           3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.participant.GetEffectiveMaxRounds(tt.globalMaxRounds); got != tt.want {
				t.Errorf("GetEffectiveMaxRounds() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDebateParticipant_LLMManagement(t *testing.T) {
	participant := DebateParticipant{
		LLMs: []LLMConfiguration{
			{
				Name:     "Primary LLM",
				Provider: "claude",
				Model:    "claude-3",
				Enabled:  true,
				Weight:   1.0,
			},
			{
				Name:     "Fallback LLM 1",
				Provider: "deepseek",
				Model:    "deepseek-coder",
				Enabled:  true,
				Weight:   0.9,
			},
			{
				Name:     "Disabled LLM",
				Provider: "gemini",
				Model:    "gemini-pro",
				Enabled:  false,
				Weight:   0.8,
			},
			{
				Name:     "Fallback LLM 2",
				Provider: "qwen",
				Model:    "qwen-turbo",
				Enabled:  true,
				Weight:   0.7,
			},
		},
	}

	t.Run("GetPrimaryLLM", func(t *testing.T) {
		primary := participant.GetPrimaryLLM()
		if primary == nil {
			t.Fatal("GetPrimaryLLM() returned nil")
		}
		if primary.Name != "Primary LLM" {
			t.Errorf("Expected primary LLM name 'Primary LLM', got %s", primary.Name)
		}
	})

	t.Run("GetFallbackLLMs", func(t *testing.T) {
		fallbacks := participant.GetFallbackLLMs()
		if len(fallbacks) != 2 {
			t.Errorf("Expected 2 fallback LLMs, got %d", len(fallbacks))
		}
		// Should only include enabled fallback LLMs
		for _, llm := range fallbacks {
			if !llm.Enabled {
				t.Errorf("Fallback LLM %s should be enabled", llm.Name)
			}
			if llm.Name == "Primary LLM" {
				t.Error("Primary LLM should not be in fallback list")
			}
		}
	})

	t.Run("GetEnabledLLMs", func(t *testing.T) {
		enabled := participant.GetEnabledLLMs()
		if len(enabled) != 3 {
			t.Errorf("Expected 3 enabled LLMs, got %d", len(enabled))
		}
		for _, llm := range enabled {
			if !llm.Enabled {
				t.Errorf("Enabled LLM %s should be enabled", llm.Name)
			}
		}
	})
}

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()
	
	if config == nil {
		t.Fatal("GetDefaultConfig() returned nil")
	}
	
	// Test basic configuration
	if !config.Enabled {
		t.Error("Expected default config to be enabled")
	}
	if config.MaximalRepeatRounds != 3 {
		t.Errorf("Expected MaximalRepeatRounds 3, got %d", config.MaximalRepeatRounds)
	}
	if len(config.Participants) != 3 {
		t.Errorf("Expected 3 default participants, got %d", len(config.Participants))
	}
	
	// Test Cognee configuration
	if !config.EnableCognee {
		t.Error("Expected default config to have Cognee enabled")
	}
	if config.CogneeConfig == nil {
		t.Fatal("Expected CogneeConfig to be set")
	}
	if !config.CogneeConfig.Enabled {
		t.Error("Expected CogneeConfig to be enabled")
	}
	
	// Test participant configurations
	for i, participant := range config.Participants {
		if !participant.Enabled {
			t.Errorf("Expected participant %d to be enabled", i)
		}
		if len(participant.LLMs) == 0 {
			t.Errorf("Expected participant %d to have LLMs configured", i)
		}
		if participant.GetPrimaryLLM() == nil {
			t.Errorf("Expected participant %d to have a primary LLM", i)
		}
	}
	
	// Validate the default configuration
	if err := config.Validate(); err != nil {
		t.Errorf("Default configuration validation failed: %v", err)
	}
}

func TestCogneeDebateConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *CogneeDebateConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Cognee config",
			config: &CogneeDebateConfig{
				Enabled:             true,
				EnhanceResponses:    true,
				AnalyzeConsensus:    true,
				GenerateInsights:    true,
				DatasetName:         "test_dataset",
				MaxEnhancementTime:  10 * 1000,
				EnhancementStrategy: "hybrid",
				MemoryIntegration:   true,
				ContextualAnalysis:  true,
			},
			wantErr: false,
		},
		{
			name: "Disabled Cognee config",
			config: &CogneeDebateConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "Missing dataset name",
			config: &CogneeDebateConfig{
				Enabled:             true,
				EnhanceResponses:    true,
				DatasetName:         "", // Missing
				MaxEnhancementTime:  10 * 1000,
				EnhancementStrategy: "hybrid",
			},
			wantErr: true,
			errMsg:  "dataset_name is required",
		},
		{
			name: "Invalid enhancement strategy",
			config: &CogneeDebateConfig{
				Enabled:             true,
				EnhanceResponses:    true,
				DatasetName:         "test_dataset",
				MaxEnhancementTime:  10 * 1000,
				EnhancementStrategy: "invalid_strategy",
			},
			wantErr: true,
			errMsg:  "invalid enhancement_strategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}