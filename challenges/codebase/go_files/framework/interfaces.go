// Package framework provides core interfaces for the SuperAgent Challenges system.
package framework

import (
	"context"
	"io"
)

// Challenge defines the interface that all challenges must implement.
type Challenge interface {
	// ID returns the unique identifier for this challenge.
	ID() ChallengeID

	// Name returns the human-readable name of this challenge.
	Name() string

	// Description returns a detailed description of what this challenge does.
	Description() string

	// Dependencies returns the IDs of challenges that must run before this one.
	Dependencies() []ChallengeID

	// Configure sets up the challenge with the given configuration.
	Configure(config *ChallengeConfig) error

	// Validate checks if the challenge can run (dependencies met, config valid).
	Validate(ctx context.Context) error

	// Execute runs the challenge and returns the result.
	Execute(ctx context.Context) (*ChallengeResult, error)

	// Cleanup performs any necessary cleanup after execution.
	Cleanup(ctx context.Context) error
}

// ChallengeRegistry manages challenge registration and discovery.
type ChallengeRegistry interface {
	// Register adds a challenge to the registry.
	Register(challenge Challenge) error

	// Get retrieves a challenge by ID.
	Get(id ChallengeID) (Challenge, error)

	// List returns all registered challenges.
	List() []Challenge

	// ListByCategory returns challenges filtered by category.
	ListByCategory(category string) []Challenge

	// GetDependencyOrder returns challenges sorted by dependency order.
	GetDependencyOrder() ([]Challenge, error)
}

// ChallengeRunner orchestrates challenge execution.
type ChallengeRunner interface {
	// Run executes a single challenge.
	Run(ctx context.Context, id ChallengeID, config *ChallengeConfig) (*ChallengeResult, error)

	// RunAll executes all challenges in dependency order.
	RunAll(ctx context.Context, config *ChallengeConfig) ([]*ChallengeResult, error)

	// RunSequence executes a specific sequence of challenges.
	RunSequence(ctx context.Context, ids []ChallengeID, config *ChallengeConfig) ([]*ChallengeResult, error)
}

// Logger provides logging capabilities for challenges.
type Logger interface {
	// Info logs an informational message.
	Info(msg string, fields ...Field)

	// Warn logs a warning message.
	Warn(msg string, fields ...Field)

	// Error logs an error message.
	Error(msg string, fields ...Field)

	// Debug logs a debug message (only if verbose).
	Debug(msg string, fields ...Field)

	// WithFields returns a logger with additional fields.
	WithFields(fields ...Field) Logger

	// LogAPIRequest logs an API request.
	LogAPIRequest(request APIRequestLog)

	// LogAPIResponse logs an API response.
	LogAPIResponse(response APIResponseLog)

	// Close closes the logger and flushes any buffers.
	Close() error
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// APIRequestLog captures API request details.
type APIRequestLog struct {
	Timestamp   string            `json:"timestamp"`
	RequestID   string            `json:"request_id"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"` // Already redacted
	Body        string            `json:"body,omitempty"`
	BodyLength  int               `json:"body_length"`
}

// APIResponseLog captures API response details.
type APIResponseLog struct {
	Timestamp      string            `json:"timestamp"`
	RequestID      string            `json:"request_id"`
	StatusCode     int               `json:"status_code"`
	Headers        map[string]string `json:"headers"` // Already redacted
	BodyPreview    string            `json:"body_preview,omitempty"`
	BodyLength     int               `json:"body_length"`
	ResponseTimeMs int64             `json:"response_time_ms"`
}

// AssertionEngine evaluates assertions against values.
type AssertionEngine interface {
	// Evaluate runs an assertion and returns the result.
	Evaluate(assertion AssertionDefinition, value any) AssertionResult

	// EvaluateAll runs multiple assertions and returns all results.
	EvaluateAll(assertions []AssertionDefinition, values map[string]any) []AssertionResult

	// Register registers a custom assertion type.
	Register(assertionType string, evaluator AssertionEvaluator) error
}

// AssertionEvaluator evaluates a specific type of assertion.
type AssertionEvaluator func(assertion AssertionDefinition, value any) (bool, string)

// Reporter generates reports from challenge results.
type Reporter interface {
	// GenerateReport creates a report for a single challenge result.
	GenerateReport(result *ChallengeResult) ([]byte, error)

	// GenerateMasterSummary creates a summary of all challenge results.
	GenerateMasterSummary(results []*ChallengeResult) ([]byte, error)

	// WriteReport writes a report to the specified writer.
	WriteReport(w io.Writer, result *ChallengeResult) error
}

// EnvironmentLoader loads and manages environment variables.
type EnvironmentLoader interface {
	// Load loads environment variables from .env file.
	Load(path string) error

	// Get retrieves an environment variable value.
	Get(key string) string

	// GetRequired retrieves a required environment variable, returning error if not set.
	GetRequired(key string) (string, error)

	// GetWithDefault retrieves an environment variable with a default value.
	GetWithDefault(key, defaultValue string) string

	// GetAPIKey retrieves an API key for a provider.
	GetAPIKey(provider string) string

	// ListConfiguredProviders returns a list of providers with configured API keys.
	ListConfiguredProviders() []string

	// Redact returns a redacted version of a value.
	Redact(value string) string
}

// ProviderVerifier verifies LLM provider connectivity and capabilities.
type ProviderVerifier interface {
	// VerifyProvider tests connectivity to a provider.
	VerifyProvider(ctx context.Context, provider ProviderInfo) (*ProviderVerificationResult, error)

	// VerifyAllProviders tests all configured providers.
	VerifyAllProviders(ctx context.Context, providers []ProviderInfo) ([]*ProviderVerificationResult, error)

	// ScoreModel scores a model based on capabilities and performance.
	ScoreModel(ctx context.Context, provider string, modelID string) (*ModelScore, error)
}

// ProviderVerificationResult holds the result of verifying a provider.
type ProviderVerificationResult struct {
	Provider      string        `json:"provider"`
	Connected     bool          `json:"connected"`
	Authenticated bool          `json:"authenticated"`
	Capabilities  []string      `json:"capabilities"`
	Models        []ModelScore  `json:"models"`
	ResponseTime  int64         `json:"response_time_ms"`
	Error         string        `json:"error,omitempty"`
}

// DebateGroupFormation handles AI debate group creation.
type DebateGroupFormation interface {
	// FormGroup creates an optimal debate group from scored models.
	FormGroup(ctx context.Context, models []ModelScore, config DebateGroupConfig) (*DebateGroup, error)

	// ValidateGroup validates a debate group configuration.
	ValidateGroup(group *DebateGroup) error

	// SaveGroup persists a debate group configuration.
	SaveGroup(group *DebateGroup, path string) error

	// LoadGroup loads a debate group configuration.
	LoadGroup(path string) (*DebateGroup, error)
}

// DebateGroupConfig holds configuration for debate group formation.
type DebateGroupConfig struct {
	PrimaryCount        int                `json:"primary_count"`
	FallbacksPerPrimary int                `json:"fallbacks_per_primary"`
	MinimumScore        float64            `json:"minimum_score"`
	PreferDiversity     bool               `json:"prefer_diversity"`
	SelectionWeights    SelectionWeights   `json:"selection_weights"`
}

// SelectionWeights defines weights for model selection criteria.
type SelectionWeights struct {
	VerificationScore   float64 `json:"verification_score"`
	CapabilityCoverage  float64 `json:"capability_coverage"`
	ResponseSpeed       float64 `json:"response_speed"`
	ProviderDiversity   float64 `json:"provider_diversity"`
}

// APITester tests the SuperAgent API with quality assertions.
type APITester interface {
	// RunTest executes a single API test.
	RunTest(ctx context.Context, prompt TestPrompt) (*APITestResult, error)

	// RunAllTests executes all configured tests.
	RunAllTests(ctx context.Context, prompts []TestPrompt) ([]*APITestResult, error)

	// RunCategory executes tests in a specific category.
	RunCategory(ctx context.Context, category string, prompts []TestPrompt) ([]*APITestResult, error)
}

// APIClient provides HTTP client functionality for API testing.
type APIClient interface {
	// SendChatCompletion sends a chat completion request.
	SendChatCompletion(ctx context.Context, request ChatCompletionRequest) (*ChatCompletionResponse, error)

	// SetBaseURL sets the API base URL.
	SetBaseURL(url string)

	// SetAPIKey sets the API key.
	SetAPIKey(key string)
}

// ChatCompletionRequest represents an OpenAI-compatible chat completion request.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents an OpenAI-compatible chat completion response.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
	DebateMetadata *DebateMetadata `json:"debate_metadata,omitempty"`
}

// Choice represents a response choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// DebateMetadata contains metadata about the debate process.
type DebateMetadata struct {
	PrimariesUsed  int     `json:"primaries_used"`
	FallbacksUsed  int     `json:"fallbacks_used"`
	DebateRounds   int     `json:"debate_rounds"`
	ConsensusScore float64 `json:"consensus_score"`
	WinningModel   string  `json:"winning_model"`
}
