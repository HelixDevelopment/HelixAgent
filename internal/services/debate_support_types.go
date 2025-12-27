package services

import (
	"time"
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
}
