// Package framework provides the challenge runner implementation.
package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Runner implements ChallengeRunner.
type Runner struct {
	registry   *Registry
	logger     Logger
	timeout    time.Duration
	resultsDir string
}

// RunnerOption configures a Runner.
type RunnerOption func(*Runner)

// WithRegistry sets the challenge registry.
func WithRegistry(reg *Registry) RunnerOption {
	return func(r *Runner) {
		r.registry = reg
	}
}

// WithLogger sets the logger.
func WithLogger(logger Logger) RunnerOption {
	return func(r *Runner) {
		r.logger = logger
	}
}

// WithTimeout sets the default timeout.
func WithTimeout(timeout time.Duration) RunnerOption {
	return func(r *Runner) {
		r.timeout = timeout
	}
}

// WithResultsDir sets the results directory.
func WithResultsDir(dir string) RunnerOption {
	return func(r *Runner) {
		r.resultsDir = dir
	}
}

// NewRunner creates a new challenge runner.
func NewRunner(opts ...RunnerOption) *Runner {
	r := &Runner{
		registry: DefaultRegistry,
		timeout:  10 * time.Minute,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Run executes a single challenge.
func (r *Runner) Run(ctx context.Context, id ChallengeID, config *ChallengeConfig) (*ChallengeResult, error) {
	challenge, err := r.registry.Get(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge: %w", err)
	}

	return r.executeChallenge(ctx, challenge, config)
}

// RunAll executes all challenges in dependency order.
func (r *Runner) RunAll(ctx context.Context, config *ChallengeConfig) ([]*ChallengeResult, error) {
	ordered, err := r.registry.GetDependencyOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency order: %w", err)
	}

	var results []*ChallengeResult
	dependencyResults := make(map[ChallengeID]string)

	for _, challenge := range ordered {
		// Update config with dependency results
		cfg := *config
		cfg.ChallengeID = challenge.ID()
		cfg.Dependencies = dependencyResults

		result, err := r.executeChallenge(ctx, challenge, &cfg)
		if err != nil {
			return results, fmt.Errorf("challenge %s failed: %w", challenge.ID(), err)
		}

		results = append(results, result)

		// Store results path for dependent challenges
		if result.Status == StatusPassed {
			dependencyResults[challenge.ID()] = cfg.ResultsDir
		}
	}

	return results, nil
}

// RunSequence executes a specific sequence of challenges.
func (r *Runner) RunSequence(ctx context.Context, ids []ChallengeID, config *ChallengeConfig) ([]*ChallengeResult, error) {
	var results []*ChallengeResult
	dependencyResults := make(map[ChallengeID]string)

	for _, id := range ids {
		challenge, err := r.registry.Get(id)
		if err != nil {
			return results, fmt.Errorf("failed to get challenge %s: %w", id, err)
		}

		// Check dependencies are met
		for _, dep := range challenge.Dependencies() {
			if _, exists := dependencyResults[dep]; !exists {
				return results, fmt.Errorf("challenge %s has unmet dependency: %s", id, dep)
			}
		}

		// Update config with dependency results
		cfg := *config
		cfg.ChallengeID = id
		cfg.Dependencies = dependencyResults

		result, err := r.executeChallenge(ctx, challenge, &cfg)
		if err != nil {
			return results, fmt.Errorf("challenge %s failed: %w", id, err)
		}

		results = append(results, result)

		// Store results path for dependent challenges
		if result.Status == StatusPassed {
			dependencyResults[id] = cfg.ResultsDir
		}
	}

	return results, nil
}

// executeChallenge runs a single challenge with proper lifecycle.
func (r *Runner) executeChallenge(ctx context.Context, challenge Challenge, config *ChallengeConfig) (*ChallengeResult, error) {
	result := &ChallengeResult{
		ChallengeID:   challenge.ID(),
		ChallengeName: challenge.Name(),
		Status:        StatusRunning,
		StartTime:     time.Now(),
		Metrics:       make(map[string]MetricValue),
		Outputs:       make(map[string]string),
	}

	// Setup results directory
	if err := r.setupResultsDir(config); err != nil {
		result.Status = StatusError
		result.Error = fmt.Sprintf("failed to setup results directory: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	result.Logs = LogPaths{
		ChallengeLog: filepath.Join(config.LogsDir, "challenge.log"),
		OutputLog:    filepath.Join(config.LogsDir, "output.log"),
	}

	// Log start
	r.logEvent("challenge_started", map[string]any{
		"challenge_id": challenge.ID(),
		"challenge_name": challenge.Name(),
	})

	// Configure challenge
	if err := challenge.Configure(config); err != nil {
		result.Status = StatusError
		result.Error = fmt.Sprintf("configuration failed: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		r.logEvent("challenge_error", map[string]any{
			"challenge_id": challenge.ID(),
			"error": result.Error,
		})
		return result, nil
	}

	// Validate challenge
	if err := challenge.Validate(ctx); err != nil {
		result.Status = StatusSkipped
		result.Error = fmt.Sprintf("validation failed: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		r.logEvent("challenge_skipped", map[string]any{
			"challenge_id": challenge.ID(),
			"reason": result.Error,
		})
		return result, nil
	}

	// Execute challenge with timeout
	timeout := config.Timeout
	if timeout == 0 {
		timeout = r.timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execResult, execErr := challenge.Execute(execCtx)

	// Handle timeout
	if execCtx.Err() == context.DeadlineExceeded {
		result.Status = StatusTimedOut
		result.Error = "challenge execution timed out"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		r.logEvent("challenge_timeout", map[string]any{
			"challenge_id": challenge.ID(),
			"timeout_seconds": timeout.Seconds(),
		})
		_ = challenge.Cleanup(ctx)
		return result, nil
	}

	// Handle execution error
	if execErr != nil {
		result.Status = StatusError
		result.Error = fmt.Sprintf("execution failed: %v", execErr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		r.logEvent("challenge_error", map[string]any{
			"challenge_id": challenge.ID(),
			"error": result.Error,
		})
		_ = challenge.Cleanup(ctx)
		return result, nil
	}

	// Merge execution result
	if execResult != nil {
		result.Assertions = execResult.Assertions
		result.Metrics = execResult.Metrics
		result.Outputs = execResult.Outputs
	}

	// Determine final status based on assertions
	result.Status = StatusPassed
	for _, assertion := range result.Assertions {
		if !assertion.Passed {
			result.Status = StatusFailed
			break
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Log completion
	r.logEvent("challenge_completed", map[string]any{
		"challenge_id": challenge.ID(),
		"status": result.Status,
		"duration_seconds": result.Duration.Seconds(),
	})

	// Cleanup
	if err := challenge.Cleanup(ctx); err != nil {
		r.logEvent("cleanup_warning", map[string]any{
			"challenge_id": challenge.ID(),
			"warning": err.Error(),
		})
	}

	return result, nil
}

// setupResultsDir creates the results directory structure.
func (r *Runner) setupResultsDir(config *ChallengeConfig) error {
	if config.ResultsDir == "" {
		timestamp := time.Now().Format("20060102_150405")
		year := time.Now().Format("2006")
		month := time.Now().Format("01")
		day := time.Now().Format("02")

		baseDir := r.resultsDir
		if baseDir == "" {
			baseDir = "results"
		}

		config.ResultsDir = filepath.Join(
			baseDir,
			string(config.ChallengeID),
			year, month, day,
			timestamp,
		)
	}

	config.LogsDir = filepath.Join(config.ResultsDir, "logs")

	if err := os.MkdirAll(config.LogsDir, 0755); err != nil {
		return err
	}

	resultsDir := filepath.Join(config.ResultsDir, "results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return err
	}

	configDir := filepath.Join(config.ResultsDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	return nil
}

// logEvent logs a structured event.
func (r *Runner) logEvent(event string, data map[string]any) {
	if r.logger != nil {
		fields := make([]Field, 0, len(data))
		for k, v := range data {
			fields = append(fields, Field{Key: k, Value: v})
		}
		r.logger.Info(event, fields...)
	}
}
