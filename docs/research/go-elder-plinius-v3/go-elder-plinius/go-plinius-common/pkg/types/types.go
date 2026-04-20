// Package types defines common types and interfaces used across all
// Plinius Go service clients. It provides shared data structures for
// service responses, pagination, health status, and version information.
package types

import (
	"context"
	"time"
)

// ServiceClient is the common interface implemented by all Plinius service clients.
// It provides lifecycle management and health checking capabilities.
type ServiceClient interface {
	// Connect establishes the connection to the gRPC service.
	// It is safe to call multiple times; subsequent calls are no-ops
	// if already connected.
	Connect(ctx context.Context) error

	// Close gracefully closes the connection to the gRPC service.
	// It is safe to call multiple times.
	Close() error

	// Health checks the health status of the remote service.
	Health(ctx context.Context) (*HealthStatus, error)

	// IsConnected returns true if the client is currently connected.
	IsConnected() bool
}

// HealthStatus represents the health check response from a service.
type HealthStatus struct {
	// Status is one of: "healthy", "degraded", "unhealthy", "unknown".
	Status string `json:"status"`

	// Service is the name of the service.
	Service string `json:"service"`

	// Version is the service version string.
	Version string `json:"version"`

	// Uptime is how long the service has been running.
	Uptime time.Duration `json:"uptime"`

	// Timestamp is when the health check was performed.
	Timestamp time.Time `json:"timestamp"`

	// Checks contains individual health check results.
	Checks []HealthCheck `json:"checks,omitempty"`
}

// HealthCheck is an individual health check component.
type HealthCheck struct {
	// Name is the name of the check.
	Name string `json:"name"`

	// Status is one of: "pass", "fail", "warn".
	Status string `json:"status"`

	// Message is an optional human-readable description.
	Message string `json:"message,omitempty"`

	// ResponseTime is how long the check took.
	ResponseTime time.Duration `json:"response_time,omitempty"`
}

// IsHealthy returns true if the overall status is healthy.
func (h *HealthStatus) IsHealthy() bool {
	return h.Status == "healthy"
}

// Pagination represents pagination parameters for list operations.
type Pagination struct {
	// Page is the 1-based page number.
	Page int `json:"page"`

	// PageSize is the number of items per page.
	PageSize int `json:"page_size"`

	// Offset is the number of items to skip (alternative to Page).
	Offset int `json:"offset,omitempty"`
}

// PaginatedResponse is embedded in list responses.
type PaginatedResponse struct {
	// Total is the total number of items available.
	Total int64 `json:"total"`

	// Page is the current page number.
	Page int `json:"page"`

	// PageSize is the number of items per page.
	PageSize int `json:"page_size"`

	// HasNext is true if more pages are available.
	HasNext bool `json:"has_next"`

	// HasPrev is true if previous pages are available.
	HasPrev bool `json:"has_prev"`
}

// VersionInfo contains version information for a service.
type VersionInfo struct {
	// Service is the service name.
	Service string `json:"service"`

	// Version is the semantic version string.
	Version string `json:"version"`

	// Commit is the Git commit hash.
	Commit string `json:"commit"`

	// BuildTime is when the binary was built.
	BuildTime time.Time `json:"build_time"`

	// GoVersion is the Go version used to build.
	GoVersion string `json:"go_version"`

	// PythonVersion is the Python version of the wrapped service.
	PythonVersion string `json:"python_version,omitempty"`
}

// String returns a formatted version string.
func (v *VersionInfo) String() string {
	return v.Service + "/" + v.Version + " (" + v.Commit + ")"
}

// TokenUsage tracks token consumption for LLM operations.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ScoreBreakdown represents a multi-dimensional scoring result.
type ScoreBreakdown struct {
	Relevance  float64 `json:"relevance"`
	Clarity    float64 `json:"clarity"`
	Utility    float64 `json:"utility"`
	Creativity float64 `json:"creativity"`
	Coherence  float64 `json:"coherence"`
	Safety     float64 `json:"safety"`
	Overall    float64 `json:"overall"`
}

// ProgressUpdate represents a progress notification from long-running operations.
type ProgressUpdate struct {
	// Stage is the current pipeline stage.
	Stage string `json:"stage"`

	// Status is one of: "running", "done", "error".
	Status string `json:"status"`

	// Message is a human-readable progress description.
	Message string `json:"message"`

	// PercentComplete is the estimated completion percentage (0-100).
	PercentComplete float64 `json:"percent_complete"`

	// Duration is how long the current stage has been running.
	Duration time.Duration `json:"duration"`

	// Details contains stage-specific information.
	Details map[string]interface{} `json:"details,omitempty"`
}

// ProgressCallback is a function that receives progress updates.
type ProgressCallback func(update ProgressUpdate)

// StageResult represents the result of a pipeline stage.
type StageResult struct {
	// Stage is the stage identifier.
	Stage string `json:"stage"`

	// Status is one of: "success", "failure", "skipped".
	Status string `json:"status"`

	// Message is a human-readable result description.
	Message string `json:"message"`

	// Duration is how long the stage took.
	Duration time.Duration `json:"duration"`

	// Details contains stage-specific result information.
	Details map[string]interface{} `json:"details,omitempty"`
}

// PipelineResult represents the overall result of a multi-stage pipeline.
type PipelineResult struct {
	// Stages contains results for each pipeline stage.
	Stages []StageResult `json:"stages"`

	// TotalDuration is the total pipeline execution time.
	TotalDuration time.Duration `json:"total_duration"`

	// Success is true if all stages succeeded.
	Success bool `json:"success"`
}

// StreamOption configures streaming behavior.
type StreamOption struct {
	// ChunkSize is the maximum size of each streamed chunk.
	ChunkSize int

	// MaxItems is the maximum number of items to stream (0 = unlimited).
	MaxItems int

	// Timeout is the maximum total streaming duration.
	Timeout time.Duration
}
