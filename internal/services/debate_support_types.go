package services

import (
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// DebateStatus represents the current status of a debate
type DebateStatus struct {
	DebateID         string              `json:"debate_id"`
	Status           string              `json:"status"`
	CurrentRound     int                 `json:"current_round"`
	TotalRounds      int                 `json:"total_rounds"`
	StartTime        time.Time           `json:"start_time"`
	EstimatedEndTime time.Time           `json:"estimated_end_time,omitempty"`
	Participants     []ParticipantStatus `json:"participants"`
	Errors           []string            `json:"errors,omitempty"`
	Metadata         map[string]any      `json:"metadata,omitempty"`
}

// ParticipantStatus represents a participant's status
type ParticipantStatus struct {
	ParticipantID   string        `json:"participant_id"`
	ParticipantName string        `json:"participant_name"`
	Status          string        `json:"status"`
	CurrentResponse string        `json:"current_response,omitempty"`
	ResponseTime    time.Duration `json:"response_time,omitempty"`
	Error           string        `json:"error,omitempty"`
}

// PerformanceMetrics represents performance metrics
type PerformanceMetrics struct {
	Duration      time.Duration `json:"duration"`
	TotalRounds   int           `json:"total_rounds"`
	QualityScore  float64       `json:"quality_score"`
	Throughput    float64       `json:"throughput"`
	Latency       time.Duration `json:"latency"`
	ErrorRate     float64       `json:"error_rate"`
	ResourceUsage ResourceUsage `json:"resource_usage"`
}

// ResourceUsage represents resource usage metrics
type ResourceUsage struct {
	CPU     float64 `json:"cpu"`
	Memory  uint64  `json:"memory"`
	Network uint64  `json:"network"`
}

// HistoryFilters represents filters for querying debate history
type HistoryFilters struct {
	StartTime       *time.Time `json:"start_time,omitempty"`
	EndTime         *time.Time `json:"end_time,omitempty"`
	ParticipantIDs  []string   `json:"participant_ids,omitempty"`
	MinQualityScore *float64   `json:"min_quality_score,omitempty"`
	MaxQualityScore *float64   `json:"max_quality_score,omitempty"`
	Limit           int        `json:"limit,omitempty"`
	Offset          int        `json:"offset,omitempty"`
}

// TimeRange represents a time range for metrics
type TimeRange struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// DebateReport represents a generated debate report
type DebateReport struct {
	ReportID        string             `json:"report_id"`
	DebateID        string             `json:"debate_id"`
	GeneratedAt     time.Time          `json:"generated_at"`
	Summary         string             `json:"summary"`
	KeyFindings     []string           `json:"key_findings"`
	Recommendations []string           `json:"recommendations"`
	Metrics         PerformanceMetrics `json:"metrics"`
	Appendices      map[string]any     `json:"appendices,omitempty"`
}

// DebateConfig represents the configuration for a debate
type DebateConfig struct {
	DebateID     string              `json:"debate_id"`
	Topic        string              `json:"topic"`
	Participants []ParticipantConfig `json:"participants"`
	MaxRounds    int                 `json:"max_rounds"`
	Timeout      time.Duration       `json:"timeout"`
	Strategy     string              `json:"strategy"`
	EnableCognee bool                `json:"enable_cognee"`
	Metadata     map[string]any      `json:"metadata,omitempty"`
	// RoleKeywords allows overriding the default keyword-to-role mapping for
	// specialized role selection. Keys are role names (e.g. "generator",
	// "debugger"), values are keyword slices. Keywords prefixed with "~"
	// require whole-word matching. When nil, built-in defaults are used.
	RoleKeywords map[string][]string `json:"role_keywords,omitempty"`
	// Tools available for debate participants to call (OpenAI function-calling format).
	// Populated from the original OpenAI-compatible request so that each participant
	// LLM can emit tool_calls (e.g. read_file, write_file) back to the caller.
	Tools      []models.Tool `json:"tools,omitempty"`
	ToolChoice interface{}   `json:"tool_choice,omitempty"`
}

// ParticipantConfig represents a participant configuration
type ParticipantConfig struct {
	ParticipantID string        `json:"participant_id"`
	Name          string        `json:"name"`
	Role          string        `json:"role"`
	LLMProvider   string        `json:"llm_provider"`
	LLMModel      string        `json:"llm_model"`
	MaxRounds     int           `json:"max_rounds"`
	Timeout       time.Duration `json:"timeout"`
	Weight        float64       `json:"weight"`
	// Single-provider multi-instance support
	Temperature  float64 `json:"temperature,omitempty"`   // Custom temperature for diversity
	SystemPrompt string  `json:"system_prompt,omitempty"` // Custom system prompt for unique perspective
	Priority     int     `json:"priority,omitempty"`      // Participant priority in debate order
	// Fallback chain - when primary LLM fails or returns empty, try these in order
	Fallbacks []FallbackConfig `json:"fallbacks,omitempty"`
	// CRITICAL: Provider instance from verification - used instead of registry lookup
	// This ensures verified providers (e.g., CLI-based OAuth providers) are used
	// instead of API-based providers from the registry.
	// When set, DebateService uses this instance directly instead of looking up by name.
	ProviderInstance llm.LLMProvider `json:"-"`
}

// FallbackConfig defines a fallback provider for when the primary fails
type FallbackConfig struct {
	Provider string `json:"provider"` // Provider name (e.g., "mistral", "zen")
	Model    string `json:"model"`    // Model name (e.g., "mistral-large-latest", "grok-code")
	// CRITICAL: Provider instance from verification - used instead of registry lookup
	// When set, DebateService uses this instance directly instead of looking up by name.
	ProviderInstance llm.LLMProvider `json:"-"`
}
