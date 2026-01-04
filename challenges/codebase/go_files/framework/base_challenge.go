// Package framework provides a base challenge implementation.
package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// BaseChallenge provides common functionality for challenge implementations.
// Embed this struct in your challenge to get default implementations.
type BaseChallenge struct {
	id           ChallengeID
	name         string
	description  string
	dependencies []ChallengeID
	config       *ChallengeConfig
	logger       Logger
	assertions   *Engine
}

// NewBaseChallenge creates a new base challenge.
func NewBaseChallenge(id ChallengeID, name, description string, deps []ChallengeID) *BaseChallenge {
	return &BaseChallenge{
		id:           id,
		name:         name,
		description:  description,
		dependencies: deps,
		assertions:   NewAssertionEngine(),
	}
}

// ID returns the challenge ID.
func (b *BaseChallenge) ID() ChallengeID {
	return b.id
}

// Name returns the challenge name.
func (b *BaseChallenge) Name() string {
	return b.name
}

// Description returns the challenge description.
func (b *BaseChallenge) Description() string {
	return b.description
}

// Dependencies returns the challenge dependencies.
func (b *BaseChallenge) Dependencies() []ChallengeID {
	return b.dependencies
}

// Configure sets up the challenge configuration.
func (b *BaseChallenge) Configure(config *ChallengeConfig) error {
	b.config = config
	return nil
}

// Validate checks if the challenge can run.
func (b *BaseChallenge) Validate(ctx context.Context) error {
	if b.config == nil {
		return fmt.Errorf("challenge not configured")
	}

	// Check dependencies are available
	for _, dep := range b.dependencies {
		if path, exists := b.config.Dependencies[dep]; !exists || path == "" {
			return fmt.Errorf("missing dependency: %s", dep)
		}
	}

	return nil
}

// Execute is a placeholder - must be overridden by concrete implementations.
func (b *BaseChallenge) Execute(ctx context.Context) (*ChallengeResult, error) {
	return nil, fmt.Errorf("Execute() not implemented for %s", b.id)
}

// Cleanup performs any necessary cleanup.
func (b *BaseChallenge) Cleanup(ctx context.Context) error {
	if b.logger != nil {
		return b.logger.Close()
	}
	return nil
}

// Config returns the current configuration.
func (b *BaseChallenge) Config() *ChallengeConfig {
	return b.config
}

// Logger returns the logger.
func (b *BaseChallenge) Logger() Logger {
	return b.logger
}

// SetLogger sets the logger.
func (b *BaseChallenge) SetLogger(logger Logger) {
	b.logger = logger
}

// AssertionEngine returns the assertion engine.
func (b *BaseChallenge) AssertionEngine() *Engine {
	return b.assertions
}

// Helper methods for common operations

// ResultsDir returns the path to the results directory.
func (b *BaseChallenge) ResultsDir() string {
	if b.config != nil {
		return filepath.Join(b.config.ResultsDir, "results")
	}
	return ""
}

// LogsDir returns the path to the logs directory.
func (b *BaseChallenge) LogsDir() string {
	if b.config != nil {
		return b.config.LogsDir
	}
	return ""
}

// WriteJSONResult writes a JSON result file.
func (b *BaseChallenge) WriteJSONResult(filename string, data any) error {
	resultsDir := b.ResultsDir()
	if resultsDir == "" {
		return fmt.Errorf("results directory not configured")
	}

	path := filepath.Join(resultsDir, filename)

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create result file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode result: %w", err)
	}

	return nil
}

// WriteMarkdownReport writes a markdown report file.
func (b *BaseChallenge) WriteMarkdownReport(filename, content string) error {
	resultsDir := b.ResultsDir()
	if resultsDir == "" {
		return fmt.Errorf("results directory not configured")
	}

	path := filepath.Join(resultsDir, filename)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// ReadDependencyResult reads a JSON result from a dependency.
func (b *BaseChallenge) ReadDependencyResult(depID ChallengeID, filename string, dest any) error {
	if b.config == nil {
		return fmt.Errorf("challenge not configured")
	}

	depPath, exists := b.config.Dependencies[depID]
	if !exists {
		return fmt.Errorf("dependency not found: %s", depID)
	}

	path := filepath.Join(depPath, "results", filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read dependency result: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to parse dependency result: %w", err)
	}

	return nil
}

// GetEnv retrieves an environment variable.
func (b *BaseChallenge) GetEnv(key string) string {
	if b.config != nil && b.config.Environment != nil {
		if val, exists := b.config.Environment[key]; exists {
			return val
		}
	}
	return os.Getenv(key)
}

// GetEnvDefault retrieves an environment variable with a default value.
func (b *BaseChallenge) GetEnvDefault(key, defaultValue string) string {
	if val := b.GetEnv(key); val != "" {
		return val
	}
	return defaultValue
}

// IsVerbose returns whether verbose mode is enabled.
func (b *BaseChallenge) IsVerbose() bool {
	return b.config != nil && b.config.Verbose
}

// EvaluateAssertions evaluates assertions against provided values.
func (b *BaseChallenge) EvaluateAssertions(assertions []AssertionDefinition, values map[string]any) []AssertionResult {
	return b.assertions.EvaluateAll(assertions, values)
}

// CreateResult creates a new challenge result with common fields populated.
func (b *BaseChallenge) CreateResult() *ChallengeResult {
	return &ChallengeResult{
		ChallengeID:   b.id,
		ChallengeName: b.name,
		Metrics:       make(map[string]MetricValue),
		Outputs:       make(map[string]string),
	}
}
