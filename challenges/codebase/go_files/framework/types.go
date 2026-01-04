// Package framework provides core types and interfaces for the SuperAgent Challenges system.
package framework

import (
	"encoding/json"
	"time"
)

// ChallengeID uniquely identifies a challenge.
type ChallengeID string

// Challenge category types.
const (
	CategoryCore       = "core"
	CategoryValidation = "validation"
	CategoryCustom     = "custom"
)

// Challenge status values.
const (
	StatusPending    = "pending"
	StatusRunning    = "running"
	StatusPassed     = "passed"
	StatusFailed     = "failed"
	StatusSkipped    = "skipped"
	StatusTimedOut   = "timed_out"
	StatusError      = "error"
)

// ChallengeDefinition defines a challenge's metadata and configuration.
type ChallengeDefinition struct {
	ID               ChallengeID           `json:"id"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	Category         string                `json:"category"`
	Dependencies     []ChallengeID         `json:"dependencies"`
	EstimatedDuration string               `json:"estimated_duration"`
	Inputs           []ChallengeInput      `json:"inputs"`
	Outputs          []ChallengeOutput     `json:"outputs"`
	Assertions       []AssertionDefinition `json:"assertions"`
	Metrics          []string              `json:"metrics"`
	Configuration    json.RawMessage       `json:"configuration,omitempty"`
}

// ChallengeInput defines an input requirement for a challenge.
type ChallengeInput struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Required bool   `json:"required"`
}

// ChallengeOutput defines an output produced by a challenge.
type ChallengeOutput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// AssertionDefinition defines an assertion to validate challenge results.
type AssertionDefinition struct {
	Type    string  `json:"type"`
	Target  string  `json:"target"`
	Value   any     `json:"value,omitempty"`
	Values  []any   `json:"values,omitempty"`
	Message string  `json:"message"`
}

// ChallengeConfig holds runtime configuration for a challenge execution.
type ChallengeConfig struct {
	ChallengeID   ChallengeID       `json:"challenge_id"`
	ResultsDir    string            `json:"results_dir"`
	LogsDir       string            `json:"logs_dir"`
	Timeout       time.Duration     `json:"timeout"`
	Verbose       bool              `json:"verbose"`
	Environment   map[string]string `json:"environment"`
	Dependencies  map[ChallengeID]string `json:"dependencies"` // ID -> results path
}

// ChallengeResult holds the outcome of a challenge execution.
type ChallengeResult struct {
	ChallengeID   ChallengeID            `json:"challenge_id"`
	ChallengeName string                 `json:"challenge_name"`
	Status        string                 `json:"status"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Duration      time.Duration          `json:"duration"`
	Assertions    []AssertionResult      `json:"assertions"`
	Metrics       map[string]MetricValue `json:"metrics"`
	Outputs       map[string]string      `json:"outputs"` // output name -> file path
	Logs          LogPaths               `json:"logs"`
	Error         string                 `json:"error,omitempty"`
}

// AssertionResult holds the outcome of a single assertion.
type AssertionResult struct {
	Type     string `json:"type"`
	Target   string `json:"target"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
}

// MetricValue holds a metric measurement.
type MetricValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

// LogPaths holds paths to various log files.
type LogPaths struct {
	ChallengeLog string `json:"challenge_log"`
	OutputLog    string `json:"output_log"`
	APIRequests  string `json:"api_requests,omitempty"`
	APIResponses string `json:"api_responses,omitempty"`
}

// ProviderInfo holds information about an LLM provider.
type ProviderInfo struct {
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	APIKeySet   bool     `json:"api_key_set"`
	APIKeyMask  string   `json:"api_key_mask,omitempty"` // Redacted key for display
	BaseURL     string   `json:"base_url,omitempty"`
	Models      []string `json:"models,omitempty"`
}

// ModelScore holds scoring information for an LLM model.
type ModelScore struct {
	Provider     string             `json:"provider"`
	ModelID      string             `json:"model_id"`
	DisplayName  string             `json:"display_name"`
	TotalScore   float64            `json:"total_score"`
	ScoreBreakdown map[string]float64 `json:"score_breakdown"`
	Capabilities []string           `json:"capabilities"`
	Verified     bool               `json:"verified"`
	ResponseTime time.Duration      `json:"response_time"`
}

// DebateGroupMember represents a member of the AI debate group.
type DebateGroupMember struct {
	Position   int          `json:"position"`
	Role       string       `json:"role"` // "primary" or "fallback_N"
	Model      ModelScore   `json:"model"`
	Fallbacks  []ModelScore `json:"fallbacks,omitempty"`
}

// DebateGroup represents the complete AI debate group configuration.
type DebateGroup struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	CreatedAt     time.Time            `json:"created_at"`
	Members       []DebateGroupMember  `json:"members"`
	TotalModels   int                  `json:"total_models"`
	AverageScore  float64              `json:"average_score"`
	Configuration DebateConfiguration  `json:"configuration"`
}

// DebateConfiguration holds debate-specific settings.
type DebateConfiguration struct {
	DebateRounds       int     `json:"debate_rounds"`
	ConsensusThreshold float64 `json:"consensus_threshold"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	FallbackStrategy   string  `json:"fallback_strategy"`
}

// TestPrompt defines a test prompt for API quality testing.
type TestPrompt struct {
	ID              string   `json:"id"`
	Category        string   `json:"category"`
	Prompt          string   `json:"prompt"`
	ExpectedElements []string `json:"expected_elements,omitempty"`
	ExpectedMentions []string `json:"expected_mentions,omitempty"`
	ExpectedAnswer   string   `json:"expected_answer,omitempty"`
	MinLength        int      `json:"min_response_length,omitempty"`
	QualityThreshold float64  `json:"quality_threshold,omitempty"`
	RequiresReasoning bool    `json:"requires_reasoning,omitempty"`
}

// APITestResult holds the result of a single API test.
type APITestResult struct {
	TestID       string            `json:"test_id"`
	Category     string            `json:"category"`
	Prompt       string            `json:"prompt"`
	Response     string            `json:"response"`
	StatusCode   int               `json:"status_code"`
	ResponseTime time.Duration     `json:"response_time"`
	Assertions   []AssertionResult `json:"assertions"`
	Passed       bool              `json:"passed"`
	QualityScore float64           `json:"quality_score"`
	IsMocked     bool              `json:"is_mocked"`
}
