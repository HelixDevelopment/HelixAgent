package config

import (
	"fmt"
)

// AIDebateConfig represents the complete AI debate configuration
type AIDebateConfig struct {
	// Global configuration
	Enabled               bool                    `yaml:"enabled" json:"enabled"`
	MaximalRepeatRounds   int                     `yaml:"maximal_repeat_rounds" json:"maximal_repeat_rounds"`
	DebateTimeout         int                     `yaml:"debate_timeout" json:"debate_timeout"`         // milliseconds
	ConsensusThreshold    float64                 `yaml:"consensus_threshold" json:"consensus_threshold"`
	EnableCognee          bool                    `yaml:"enable_cognee" json:"enable_cognee"`
	CogneeConfig          *CogneeDebateConfig     `yaml:"cognee_config,omitempty" json:"cognee_config,omitempty"`
	
	// Participant configuration
	Participants          []DebateParticipant     `yaml:"participants" json:"participants"`
	
	// Debate strategies and rules
	DebateStrategy        string                  `yaml:"debate_strategy" json:"debate_strategy"`
	VotingStrategy        string                  `yaml:"voting_strategy" json:"voting_strategy"`
	ResponseFormat        string                  `yaml:"response_format" json:"response_format"`
	
	// Memory and context management
	EnableMemory          bool                    `yaml:"enable_memory" json:"enable_memory"`
	MemoryRetention       int                     `yaml:"memory_retention" json:"memory_retention"`         // milliseconds
	MaxContextLength      int                     `yaml:"max_context_length" json:"max_context_length"`
	
	// Quality and performance settings
	QualityThreshold      float64                 `yaml:"quality_threshold" json:"quality_threshold"`
	MaxResponseTime       int                     `yaml:"max_response_time" json:"max_response_time"`         // milliseconds
	EnableStreaming       bool                    `yaml:"enable_streaming" json:"enable_streaming"`
	
	// Logging and monitoring
	EnableDebateLogging   bool                    `yaml:"enable_debate_logging" json:"enable_debate_logging"`
	LogDebateDetails      bool                    `yaml:"log_debate_details" json:"log_debate_details"`
	MetricsEnabled        bool                    `yaml:"metrics_enabled" json:"metrics_enabled"`
}

// CogneeDebateConfig holds Cognee AI specific configuration for debate enhancement
type CogneeDebateConfig struct {
	Enabled              bool     `yaml:"enabled" json:"enabled"`
	EnhanceResponses     bool     `yaml:"enhance_responses" json:"enhance_responses"`
	AnalyzeConsensus     bool     `yaml:"analyze_consensus" json:"analyze_consensus"`
	GenerateInsights     bool     `yaml:"generate_insights" json:"generate_insights"`
	DatasetName          string   `yaml:"dataset_name" json:"dataset_name"`
	MaxEnhancementTime   int           `yaml:"max_enhancement_time" json:"max_enhancement_time"` // milliseconds
	EnhancementStrategy  string   `yaml:"enhancement_strategy" json:"enhancement_strategy"`
	MemoryIntegration    bool     `yaml:"memory_integration" json:"memory_integration"`
	ContextualAnalysis   bool     `yaml:"contextual_analysis" json:"contextual_analysis"`
}

// DebateParticipant represents a participant in the AI debate
type DebateParticipant struct {
	// Basic participant information
	Name                string              `yaml:"name" json:"name"`
	Role                string              `yaml:"role" json:"role"`
	Description         string              `yaml:"description,omitempty" json:"description,omitempty"`
	Enabled             bool                `yaml:"enabled" json:"enabled"`
	
	// LLM configuration chain (first is main, others are fallbacks)
	LLMs                []LLMConfiguration  `yaml:"llms" json:"llms"`
	
	// Participant-specific settings
	MaximalRepeatRounds *int                `yaml:"maximal_repeat_rounds,omitempty" json:"maximal_repeat_rounds,omitempty"`
	ResponseTimeout     int                 `yaml:"response_timeout" json:"response_timeout"` // milliseconds
	Weight              float64             `yaml:"weight" json:"weight"`
	Priority            int                 `yaml:"priority" json:"priority"`
	
	// Debate behavior configuration
	DebateStyle         string              `yaml:"debate_style" json:"debate_style"`
	ArgumentationStyle  string              `yaml:"argumentation_style" json:"argumentation_style"`
	PersuasionLevel     float64             `yaml:"persuasion_level" json:"persuasion_level"`
	OpennessToChange    float64             `yaml:"openness_to_change" json:"openness_to_change"`
	
	// Quality and validation settings
	QualityThreshold    float64             `yaml:"quality_threshold" json:"quality_threshold"`
	MinResponseLength   int                 `yaml:"min_response_length" json:"min_response_length"`
	MaxResponseLength   int                 `yaml:"max_response_length" json:"max_response_length"`
	
	// Cognee AI enhancement
	EnableCognee        bool                `yaml:"enable_cognee" json:"enable_cognee"`
	CogneeSettings      *CogneeParticipantConfig `yaml:"cognee_settings,omitempty" json:"cognee_settings,omitempty"`
}

// CogneeParticipantConfig holds Cognee AI settings for individual participants
type CogneeParticipantConfig struct {
	EnhanceResponses   bool   `yaml:"enhance_responses" json:"enhance_responses"`
	AnalyzeSentiment   bool   `yaml:"analyze_sentiment" json:"analyze_sentiment"`
	ExtractEntities    bool   `yaml:"extract_entities" json:"extract_entities"`
	GenerateSummary    bool   `yaml:"generate_summary" json:"generate_summary"`
	DatasetName        string `yaml:"dataset_name" json:"dataset_name"`
}

// LLMConfiguration represents a single LLM configuration in the fallback chain
type LLMConfiguration struct {
	// Basic LLM identification
	Name             string                 `yaml:"name" json:"name"`
	Provider         string                 `yaml:"provider" json:"provider"`
	Model            string                 `yaml:"model" json:"model"`
	Enabled          bool                   `yaml:"enabled" json:"enabled"`
	
	// Connection and authentication
	APIKey           string                 `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	BaseURL          string                 `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	Timeout          int                    `yaml:"timeout" json:"timeout"` // milliseconds
	MaxRetries       int                    `yaml:"max_retries" json:"max_retries"`
	
	// Model parameters and behavior
	Temperature      float64                `yaml:"temperature" json:"temperature"`
	MaxTokens        int                    `yaml:"max_tokens" json:"max_tokens"`
	TopP             float64                `yaml:"top_p,omitempty" json:"top_p,omitempty"`
	FrequencyPenalty float64                `yaml:"frequency_penalty,omitempty" json:"frequency_penalty,omitempty"`
	PresencePenalty  float64                `yaml:"presence_penalty,omitempty" json:"presence_penalty,omitempty"`
	StopSequences    []string               `yaml:"stop_sequences,omitempty" json:"stop_sequences,omitempty"`
	
	// Provider-specific settings
	CustomParams     map[string]interface{} `yaml:"custom_params,omitempty" json:"custom_params,omitempty"`
	Capabilities     []string               `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	
	// Performance and quality settings
	Weight           float64                `yaml:"weight" json:"weight"`
	RateLimitRPS     int                    `yaml:"rate_limit_rps,omitempty" json:"rate_limit_rps,omitempty"`
	RequestTimeout   int                    `yaml:"request_timeout,omitempty" json:"request_timeout,omitempty"` // milliseconds
	
	// Health and monitoring
	HealthCheckURL   string                 `yaml:"health_check_url,omitempty" json:"health_check_url,omitempty"`
	HealthCheckInterval int                  `yaml:"health_check_interval,omitempty" json:"health_check_interval,omitempty"` // milliseconds
}

// Validate performs comprehensive validation of the AI debate configuration
func (c *AIDebateConfig) Validate() error {
	if !c.Enabled {
		return nil // Configuration is disabled, skip validation
	}

	// Validate global settings
	if c.MaximalRepeatRounds < 1 {
		return fmt.Errorf("maximal_repeat_rounds must be at least 1, got %d", c.MaximalRepeatRounds)
	}
	if c.MaximalRepeatRounds > 10 {
		return fmt.Errorf("maximal_repeat_rounds cannot exceed 10, got %d", c.MaximalRepeatRounds)
	}
	if c.DebateTimeout <= 0 {
		return fmt.Errorf("debate_timeout must be positive, got %d", c.DebateTimeout)
	}
	if c.ConsensusThreshold < 0.0 || c.ConsensusThreshold > 1.0 {
		return fmt.Errorf("consensus_threshold must be between 0.0 and 1.0, got %f", c.ConsensusThreshold)
	}
	if c.QualityThreshold < 0.0 || c.QualityThreshold > 1.0 {
		return fmt.Errorf("quality_threshold must be between 0.0 and 1.0, got %f", c.QualityThreshold)
	}
	if c.MaxResponseTime <= 0 {
		return fmt.Errorf("max_response_time must be positive, got %d", c.MaxResponseTime)
	}
	if c.MaxContextLength <= 0 {
		return fmt.Errorf("max_context_length must be positive, got %d", c.MaxContextLength)
	}

	// Validate Cognee configuration if enabled
	if c.EnableCognee {
		if err := c.CogneeConfig.Validate(); err != nil {
			return fmt.Errorf("invalid cognee_config: %w", err)
		}
	}

	// Validate participants
	if len(c.Participants) < 2 {
		return fmt.Errorf("at least 2 participants are required for debate, got %d", len(c.Participants))
	}
	if len(c.Participants) > 10 {
		return fmt.Errorf("maximum 10 participants allowed for debate, got %d", len(c.Participants))
	}

	// Validate each participant
	participantNames := make(map[string]bool)
	for i, participant := range c.Participants {
		if err := participant.Validate(c.MaximalRepeatRounds); err != nil {
			return fmt.Errorf("invalid participant at index %d: %w", i, err)
		}
		if participantNames[participant.Name] {
			return fmt.Errorf("duplicate participant name: %s", participant.Name)
		}
		participantNames[participant.Name] = true
	}

	// Validate debate and voting strategies
	validDebateStrategies := []string{"round_robin", "free_form", "structured", "adversarial", "collaborative"}
	if !contains(validDebateStrategies, c.DebateStrategy) {
		return fmt.Errorf("invalid debate_strategy: %s, must be one of %v", c.DebateStrategy, validDebateStrategies)
	}

	validVotingStrategies := []string{"majority", "weighted", "consensus", "confidence_weighted", "quality_weighted"}
	if !contains(validVotingStrategies, c.VotingStrategy) {
		return fmt.Errorf("invalid voting_strategy: %s, must be one of %v", c.VotingStrategy, validVotingStrategies)
	}

	return nil
}

// Validate validates the Cognee debate configuration
func (c *CogneeDebateConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.DatasetName == "" {
		return fmt.Errorf("dataset_name is required when Cognee is enabled")
	}
	if c.MaxEnhancementTime <= 0 {
		return fmt.Errorf("max_enhancement_time must be positive, got %d", c.MaxEnhancementTime)
	}

	validStrategies := []string{"semantic_enhancement", "contextual_analysis", "knowledge_integration", "hybrid"}
	if !contains(validStrategies, c.EnhancementStrategy) {
		return fmt.Errorf("invalid enhancement_strategy: %s, must be one of %v", c.EnhancementStrategy, validStrategies)
	}

	return nil
}

// Validate validates a debate participant configuration
func (p *DebateParticipant) Validate(globalMaxRounds int) error {
	if p.Name == "" {
		return fmt.Errorf("participant name is required")
	}
	if p.Role == "" {
		return fmt.Errorf("participant role is required for %s", p.Name)
	}
	if !p.Enabled {
		return nil // Participant is disabled, skip further validation
	}

	// Validate LLM configurations
	if len(p.LLMs) == 0 {
		return fmt.Errorf("participant %s must have at least one LLM configuration", p.Name)
	}

	// Validate maximal repeat rounds
	maxRounds := globalMaxRounds
	if p.MaximalRepeatRounds != nil {
		maxRounds = *p.MaximalRepeatRounds
		if maxRounds < 1 {
			return fmt.Errorf("participant %s maximal_repeat_rounds must be at least 1, got %d", p.Name, maxRounds)
		}
		if maxRounds > 10 {
			return fmt.Errorf("participant %s maximal_repeat_rounds cannot exceed 10, got %d", p.Name, maxRounds)
		}
	}

	// Validate participant-specific settings
	if p.ResponseTimeout <= 0 {
		return fmt.Errorf("participant %s response_timeout must be positive, got %d", p.Name, p.ResponseTimeout)
	}
	if p.Weight < 0.0 {
		return fmt.Errorf("participant %s weight must be non-negative, got %f", p.Name, p.Weight)
	}
	if p.Priority < 0 {
		return fmt.Errorf("participant %s priority must be non-negative, got %d", p.Name, p.Priority)
	}
	if p.QualityThreshold < 0.0 || p.QualityThreshold > 1.0 {
		return fmt.Errorf("participant %s quality_threshold must be between 0.0 and 1.0, got %f", p.Name, p.QualityThreshold)
	}
	if p.MinResponseLength < 0 {
		return fmt.Errorf("participant %s min_response_length must be non-negative, got %d", p.Name, p.MinResponseLength)
	}
	if p.MaxResponseLength <= 0 {
		return fmt.Errorf("participant %s max_response_length must be positive, got %d", p.Name, p.MaxResponseLength)
	}
	if p.MinResponseLength > p.MaxResponseLength {
		return fmt.Errorf("participant %s min_response_length (%d) cannot be greater than max_response_length (%d)", 
			p.Name, p.MinResponseLength, p.MaxResponseLength)
	}

	// Validate debate behavior settings
	validStyles := []string{"analytical", "creative", "balanced", "aggressive", "diplomatic", "technical", "critical"}
	if !contains(validStyles, p.DebateStyle) {
		return fmt.Errorf("participant %s invalid debate_style: %s, must be one of %v", p.Name, p.DebateStyle, validStyles)
	}

	validArgStyles := []string{"logical", "emotional", "evidence_based", "hypothetical", "socratic"}
	if !contains(validArgStyles, p.ArgumentationStyle) {
		return fmt.Errorf("participant %s invalid argumentation_style: %s, must be one of %v", p.Name, p.ArgumentationStyle, validArgStyles)
	}

	if p.PersuasionLevel < 0.0 || p.PersuasionLevel > 1.0 {
		return fmt.Errorf("participant %s persuasion_level must be between 0.0 and 1.0, got %f", p.Name, p.PersuasionLevel)
	}
	if p.OpennessToChange < 0.0 || p.OpennessToChange > 1.0 {
		return fmt.Errorf("participant %s openness_to_change must be between 0.0 and 1.0, got %f", p.Name, p.OpennessToChange)
	}

	// Validate LLM configurations
	hasEnabledLLM := false
	for i, llm := range p.LLMs {
		if err := llm.Validate(); err != nil {
			return fmt.Errorf("participant %s LLM at index %d: %w", p.Name, i, err)
		}
		if llm.Enabled {
			hasEnabledLLM = true
		}
	}

	if !hasEnabledLLM {
		return fmt.Errorf("participant %s must have at least one enabled LLM", p.Name)
	}

	// Validate Cognee settings if enabled
	if p.EnableCognee && p.CogneeSettings != nil {
		if err := p.CogneeSettings.Validate(); err != nil {
			return fmt.Errorf("participant %s invalid cognee_settings: %w", p.Name, err)
		}
	}

	return nil
}

// Validate validates an LLM configuration
func (l *LLMConfiguration) Validate() error {
	if l.Name == "" {
		return fmt.Errorf("LLM name is required")
	}
	if l.Provider == "" {
		return fmt.Errorf("LLM provider is required for %s", l.Name)
	}
	if l.Model == "" {
		return fmt.Errorf("LLM model is required for %s", l.Name)
	}
	if !l.Enabled {
		return nil // LLM is disabled, skip further validation
	}

	// Validate provider
	validProviders := []string{"claude", "deepseek", "gemini", "qwen", "zai", "ollama", "openrouter", "openai", "anthropic"}
	if !contains(validProviders, l.Provider) {
		return fmt.Errorf("LLM %s invalid provider: %s, must be one of %v", l.Name, l.Provider, validProviders)
	}

	// Validate timeout settings
	if l.Timeout <= 0 {
		return fmt.Errorf("LLM %s timeout must be positive, got %d", l.Name, l.Timeout)
	}
	if l.RequestTimeout > 0 && l.RequestTimeout < l.Timeout {
		return fmt.Errorf("LLM %s request_timeout (%d) cannot be less than timeout (%d)", l.Name, l.RequestTimeout, l.Timeout)
	}
	if l.MaxRetries < 0 {
		return fmt.Errorf("LLM %s max_retries must be non-negative, got %d", l.Name, l.MaxRetries)
	}

	// Validate model parameters
	if l.Temperature < 0.0 || l.Temperature > 2.0 {
		return fmt.Errorf("LLM %s temperature must be between 0.0 and 2.0, got %f", l.Name, l.Temperature)
	}
	if l.MaxTokens <= 0 {
		return fmt.Errorf("LLM %s max_tokens must be positive, got %d", l.Name, l.MaxTokens)
	}
	if l.TopP < 0.0 || l.TopP > 1.0 {
		return fmt.Errorf("LLM %s top_p must be between 0.0 and 1.0, got %f", l.Name, l.TopP)
	}
	if l.FrequencyPenalty < -2.0 || l.FrequencyPenalty > 2.0 {
		return fmt.Errorf("LLM %s frequency_penalty must be between -2.0 and 2.0, got %f", l.Name, l.FrequencyPenalty)
	}
	if l.PresencePenalty < -2.0 || l.PresencePenalty > 2.0 {
		return fmt.Errorf("LLM %s presence_penalty must be between -2.0 and 2.0, got %f", l.Name, l.PresencePenalty)
	}
	if l.Weight < 0.0 {
		return fmt.Errorf("LLM %s weight must be non-negative, got %f", l.Name, l.Weight)
	}
	if l.RateLimitRPS < 0 {
		return fmt.Errorf("LLM %s rate_limit_rps must be non-negative, got %d", l.Name, l.RateLimitRPS)
	}

	// Validate health check settings
	if l.HealthCheckInterval > 0 && l.HealthCheckInterval < 10000 {
		return fmt.Errorf("LLM %s health_check_interval must be at least 10000ms if specified, got %d", l.Name, l.HealthCheckInterval)
	}

	return nil
}

// Validate validates Cognee participant configuration
func (c *CogneeParticipantConfig) Validate() error {
	if !c.EnhanceResponses && !c.AnalyzeSentiment && !c.ExtractEntities && !c.GenerateSummary {
		return fmt.Errorf("at least one Cognee enhancement must be enabled")
	}
	if c.DatasetName == "" {
		return fmt.Errorf("dataset_name is required for Cognee participant config")
	}
	return nil
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetEffectiveMaxRounds returns the effective maximum rounds for a participant
func (p *DebateParticipant) GetEffectiveMaxRounds(globalMaxRounds int) int {
	if p.MaximalRepeatRounds != nil {
		return *p.MaximalRepeatRounds
	}
	return globalMaxRounds
}

// GetPrimaryLLM returns the primary (first enabled) LLM configuration
func (p *DebateParticipant) GetPrimaryLLM() *LLMConfiguration {
	for _, llm := range p.LLMs {
		if llm.Enabled {
			return &llm
		}
	}
	return nil
}

// GetFallbackLLMs returns all fallback LLM configurations (excluding primary)
func (p *DebateParticipant) GetFallbackLLMs() []LLMConfiguration {
	var fallbacks []LLMConfiguration
	foundPrimary := false
	for _, llm := range p.LLMs {
		if llm.Enabled {
			if !foundPrimary {
				foundPrimary = true
				continue
			}
			fallbacks = append(fallbacks, llm)
		}
	}
	return fallbacks
}

// GetEnabledLLMs returns all enabled LLM configurations
func (p *DebateParticipant) GetEnabledLLMs() []LLMConfiguration {
	var enabled []LLMConfiguration
	for _, llm := range p.LLMs {
		if llm.Enabled {
			enabled = append(enabled, llm)
		}
	}
	return enabled
}