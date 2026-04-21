// Package adapters provides provider-specific verification adapters for the startup verification system.
package adapters

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/llm/providers/openrouter"
	"dev.helix.agent/internal/llm/providers/zen"
	"dev.helix.agent/internal/models"
	verifier "dev.helix.agent/internal/verifier"
)

var freeLog = logrus.New()

// FreeProviderType defines the type of free provider
type FreeProviderType string

const (
	// FreeProviderZen is the OpenCode Zen free provider
	FreeProviderZen FreeProviderType = "zen"
	// FreeProviderOpenRouter is the OpenRouter free tier
	FreeProviderOpenRouter FreeProviderType = "openrouter"
)

// FreeAdapterConfig holds configuration for the free provider adapter
type FreeAdapterConfig struct {
	// VerificationTimeout is the timeout for verification requests
	VerificationTimeout time.Duration
	// HealthCheckTimeout is the timeout for health check requests
	HealthCheckTimeout time.Duration
	// MaxConcurrentVerifications limits concurrent verification requests
	MaxConcurrentVerifications int
	// MinHealthScore is the minimum health score to consider a provider verified
	MinHealthScore float64
	// BaseScore is the base score for free providers
	BaseScore float64
	// MaxScore is the maximum score for free providers
	MaxScore float64
	// RetryAttempts is the number of retry attempts for failed verifications
	RetryAttempts int
	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration
}

// DefaultFreeAdapterConfig returns the default configuration
func DefaultFreeAdapterConfig() *FreeAdapterConfig {
	return &FreeAdapterConfig{
		VerificationTimeout:        30 * time.Second,
		HealthCheckTimeout:         10 * time.Second,
		MaxConcurrentVerifications: 4,
		MinHealthScore:             0.5,
		BaseScore:                  6.0, // Free providers have lower base score (6.0-7.0)
		MaxScore:                   7.0,
		RetryAttempts:              2,
		RetryDelay:                 1 * time.Second,
	}
}

// FreeProviderAdapter handles verification for free providers (Zen, OpenRouter :free models).
//
// CONST-029: the previous implementation held an fa.mu sync.RWMutex for
// reader methods and used a per-call modelsMu sync.Mutex inside
// VerifyZenProvider / VerifyOpenRouterFreeModels for writer paths.
// Those two mutexes did NOT serialise against each other — a reader
// holding fa.mu.RLock() could observe partial map writes from either
// verify goroutine, and two concurrent verify calls could race on the
// shared maps (two different per-call mutexes, same map).
// safe.Store's internal synchronisation collapses both locks into one
// discipline-free primitive: the race is structurally impossible.
type FreeProviderAdapter struct {
	verifierSvc *verifier.VerificationService
	config      *FreeAdapterConfig
	httpClient  *http.Client

	// Provider instances
	zenProvider    *zen.ZenProvider
	zenCLIProvider *zen.ZenCLIProvider // CLI facade for failed API models

	// Cached verification results (all concurrent-safe; no external mutex).
	verifiedModels *safe.Store[string, *verifier.UnifiedModel]
	lastVerified   *safe.Store[string, time.Time]
	healthStatus   *safe.Store[string, bool]

	// Models that failed direct API verification.
	failedAPIModels *safe.Store[string, error]
}

// NewFreeProviderAdapter creates a new free provider adapter
func NewFreeProviderAdapter(verifierSvc *verifier.VerificationService, config *FreeAdapterConfig) *FreeProviderAdapter {
	if config == nil {
		config = DefaultFreeAdapterConfig()
	}

	adapter := &FreeProviderAdapter{
		verifierSvc: verifierSvc,
		config:      config,
		httpClient: &http.Client{
			Timeout: config.VerificationTimeout,
		},
		verifiedModels:  safe.NewStore[string, *verifier.UnifiedModel](),
		lastVerified:    safe.NewStore[string, time.Time](),
		healthStatus:    safe.NewStore[string, bool](),
		failedAPIModels: safe.NewStore[string, error](),
	}

	// Initialize ZenCLIProvider for fallback facade (lazy initialization)
	// Only check if opencode is installed, don't do expensive model discovery yet
	if zen.IsOpenCodeInstalled() {
		// Use a config with explicit model to avoid triggering model discovery
		adapter.zenCLIProvider = zen.NewZenCLIProvider(zen.ZenCLIConfig{
			Model:           zen.DefaultZenModel, // Use default model to avoid discovery
			Timeout:         120 * time.Second,
			MaxOutputTokens: 4096,
		})
		freeLog.Info("OpenCode CLI available - will use CLI facade for models that fail direct API verification")
	} else {
		freeLog.Info("OpenCode CLI not installed - CLI facade fallback not available")
	}

	return adapter
}

// VerifyZenProvider verifies the Zen free provider and returns a UnifiedProvider
// It uses a two-phase approach:
// 1. First, try direct API verification for all models
// 2. For models that fail direct API, attempt CLI facade verification via `opencode` command
func (fa *FreeProviderAdapter) VerifyZenProvider(ctx context.Context) (*verifier.UnifiedProvider, error) {
	freeLog.WithField("provider", "zen").Info("Starting Zen provider verification")
	startTime := time.Now()

	// Create anonymous Zen provider for free models
	fa.zenProvider = zen.NewZenProviderAnonymous(zen.DefaultZenModel)

	// Perform health check first
	healthErr := fa.zenProvider.HealthCheck()
	if healthErr != nil {
		freeLog.WithFields(logrus.Fields{
			"provider": "zen",
			"error":    healthErr.Error(),
		}).Warn("Zen health check failed, attempting verification anyway")
	}

	// Get free models
	freeModels := zen.FreeModels()
	models := make([]verifier.UnifiedModel, 0, len(freeModels))
	failedModels := make([]string, 0)

	// PHASE 1: Verify each free model via direct API.
	// The previous impl held a per-call modelsMu; safe.Store serialises
	// writes to fa.verifiedModels / lastVerified / failedAPIModels
	// internally, so we only need a local mu for the plain `models`
	// slice and `failedModels` slice that accumulate per-call state.
	var wg sync.WaitGroup
	var localMu sync.Mutex
	sem := make(chan struct{}, fa.config.MaxConcurrentVerifications)

	for _, modelID := range freeModels {
		wg.Add(1)
		go func(mID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			model, err := fa.verifyZenModel(ctx, mID)
			if err != nil {
				freeLog.WithFields(logrus.Fields{
					"provider": "zen",
					"model":    mID,
					"error":    err.Error(),
				}).Warn("Failed to verify Zen model via direct API")

				localMu.Lock()
				failedModels = append(failedModels, mID)
				localMu.Unlock()
				fa.failedAPIModels.Put(mID, err)
				return
			}

			localMu.Lock()
			models = append(models, *model)
			localMu.Unlock()
			fa.verifiedModels.Put(mID, model)
			fa.lastVerified.Put(mID, time.Now())
		}(modelID)
	}

	wg.Wait()

	// PHASE 2: Attempt CLI facade verification for models that failed direct API
	cliVerifiedModels := 0
	if len(failedModels) > 0 && fa.zenCLIProvider != nil && fa.zenCLIProvider.IsCLIAvailable() {
		freeLog.WithFields(logrus.Fields{
			"provider":      "zen",
			"failed_models": len(failedModels),
		}).Info("Attempting CLI facade verification for models that failed direct API")

		for _, modelID := range failedModels {
			// Mark the model as failed in the CLI provider
			fa.zenCLIProvider.MarkModelAsFailedAPI(modelID)

			// Attempt CLI-based verification
			model, err := fa.verifyZenModelViaCLI(ctx, modelID)
			if err != nil {
				freeLog.WithFields(logrus.Fields{
					"provider": "zen",
					"model":    modelID,
					"error":    err.Error(),
				}).Warn("Failed to verify Zen model via CLI facade")
				continue
			}

			localMu.Lock()
			models = append(models, *model)
			localMu.Unlock()
			fa.verifiedModels.Put(modelID, model)
			fa.lastVerified.Put(modelID, time.Now())
			// Remove from failed models since CLI verification succeeded
			fa.failedAPIModels.Delete(modelID)
			cliVerifiedModels++
		}

		freeLog.WithFields(logrus.Fields{
			"provider":               "zen",
			"cli_verified_models":    cliVerifiedModels,
			"total_failed_after_cli": len(failedModels) - cliVerifiedModels,
		}).Info("CLI facade verification completed")
	}

	// Calculate provider score based on verified models
	score := fa.calculateZenScore(models, healthErr == nil)

	// Determine status
	status := verifier.StatusVerified
	if len(models) == 0 {
		status = verifier.StatusFailed
	} else if len(models) < len(freeModels) {
		status = verifier.StatusDegraded
	}

	provider := &verifier.UnifiedProvider{
		ID:           "zen",
		Name:         "OpenCode Zen",
		Type:         "free",
		AuthType:     verifier.AuthTypeFree,
		Verified:     len(models) > 0,
		Score:        score,
		Models:       models,
		Status:       status,
		Instance:     fa.zenProvider,
		BaseURL:      zen.ZenAPIURL,
		VerifiedAt:   time.Now(),
		LastHealthAt: time.Now(),
		ErrorCount:   0,
		Metadata: map[string]interface{}{
			"verification_time_ms": time.Since(startTime).Milliseconds(),
			"verified_models":      len(models),
			"total_models":         len(freeModels),
			"health_check_passed":  healthErr == nil,
			"anonymous_mode":       true,
			"api_verified_models":  len(models) - cliVerifiedModels,
			"cli_verified_models":  cliVerifiedModels,
			"cli_facade_available": fa.zenCLIProvider != nil && fa.zenCLIProvider.IsCLIAvailable(),
		},
	}

	freeLog.WithFields(logrus.Fields{
		"provider":        "zen",
		"verified_models": len(models),
		"api_verified":    len(models) - cliVerifiedModels,
		"cli_verified":    cliVerifiedModels,
		"score":           score,
		"status":          status,
		"duration_ms":     time.Since(startTime).Milliseconds(),
	}).Info("Zen provider verification completed")

	return provider, nil
}

// verifyZenModel verifies a single Zen model
func (fa *FreeProviderAdapter) verifyZenModel(ctx context.Context, modelID string) (*verifier.UnifiedModel, error) {
	startTime := time.Now()

	// Create a provider for this specific model
	modelProvider := zen.NewZenProviderAnonymous(modelID)

	// Perform reduced verification (health check + simple completion)
	var verificationErr error
	var latency time.Duration
	verified := false

	for attempt := 0; attempt <= fa.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(fa.config.RetryDelay):
			}
		}

		// Health check
		if err := modelProvider.HealthCheck(); err != nil {
			verificationErr = err
			continue
		}

		// Simple completion test
		testStart := time.Now()
		if err := fa.testModelCompletion(ctx, modelProvider); err != nil {
			verificationErr = err
			continue
		}
		latency = time.Since(testStart)
		verified = true
		break
	}

	if !verified {
		return nil, fmt.Errorf("verification failed after %d attempts: %v", fa.config.RetryAttempts+1, verificationErr)
	}

	// Calculate model score (free models score between 6.0-7.0)
	score := fa.calculateModelScore(latency, verified)

	// Get capabilities from provider
	caps := modelProvider.GetCapabilities()

	model := &verifier.UnifiedModel{
		ID:           modelID,
		Name:         getModelDisplayName(modelID),
		Provider:     "zen",
		Verified:     verified,
		Score:        score,
		Latency:      latency,
		Capabilities: convertCapabilities(caps),
		Metadata: map[string]interface{}{
			"free_model":           true,
			"verification_time_ms": time.Since(startTime).Milliseconds(),
			"latency_ms":           latency.Milliseconds(),
			"anonymous_access":     true,
			"max_tokens":           caps.Limits.MaxTokens,
			"context_window":       caps.Limits.MaxInputLength,
		},
	}

	return model, nil
}

// verifyZenModelViaCLI verifies a Zen model using the OpenCode CLI facade
// This is used as a fallback when direct API verification fails
func (fa *FreeProviderAdapter) verifyZenModelViaCLI(ctx context.Context, modelID string) (*verifier.UnifiedModel, error) {
	startTime := time.Now()

	if fa.zenCLIProvider == nil {
		return nil, fmt.Errorf("CLI provider not available")
	}

	if !fa.zenCLIProvider.IsCLIAvailable() {
		return nil, fmt.Errorf("OpenCode CLI not installed")
	}

	// Set the model for this verification
	fa.zenCLIProvider.SetModel(modelID)

	// Perform reduced verification via CLI
	var verificationErr error
	var latency time.Duration
	verified := false

	for attempt := 0; attempt <= fa.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(fa.config.RetryDelay):
			}
		}

		// Simple completion test via CLI
		testStart := time.Now()
		if err := fa.testModelCompletionViaCLI(ctx, modelID); err != nil {
			verificationErr = err
			continue
		}
		latency = time.Since(testStart)
		verified = true
		break
	}

	if !verified {
		return nil, fmt.Errorf("CLI verification failed after %d attempts: %v", fa.config.RetryAttempts+1, verificationErr)
	}

	// Calculate model score (CLI-verified models get slightly lower score)
	score := fa.calculateModelScore(latency, verified)
	// CLI models get a small penalty since they're less direct
	score = score - 0.1
	if score < fa.config.BaseScore {
		score = fa.config.BaseScore
	}

	model := &verifier.UnifiedModel{
		ID:       modelID,
		Name:     getModelDisplayName(modelID) + " (CLI)",
		Provider: "zen",
		Verified: verified,
		Score:    score,
		Latency:  latency,
		Capabilities: []string{
			"text_completion",
			"chat",
			"streaming",
		},
		Metadata: map[string]interface{}{
			"free_model":           true,
			"verification_time_ms": time.Since(startTime).Milliseconds(),
			"latency_ms":           latency.Milliseconds(),
			"verified_via":         "cli_facade",
			"cli_command":          "opencode",
			"direct_api_failed":    true,
		},
	}

	freeLog.WithFields(logrus.Fields{
		"provider":   "zen",
		"model":      modelID,
		"via":        "cli_facade",
		"latency_ms": latency.Milliseconds(),
		"score":      score,
	}).Info("Model verified via CLI facade")

	return model, nil
}

// testModelCompletionViaCLI performs a completion test using the CLI facade
func (fa *FreeProviderAdapter) testModelCompletionViaCLI(ctx context.Context, modelID string) error {
	startTime := time.Now()

	// Create a simple test request
	testReq := &models.LLMRequest{
		ID:     fmt.Sprintf("cli-test-%d", time.Now().UnixNano()),
		Prompt: "You are a helpful assistant. Reply concisely.",
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "What is 2 + 2? Reply with just the number.",
			},
		},
		ModelParams: models.ModelParameters{
			Model:       modelID,
			MaxTokens:   10,
			Temperature: 0.0,
		},
	}

	// Perform completion via CLI
	resp, err := fa.zenCLIProvider.Complete(ctx, testReq)
	latency := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("CLI completion failed: %w", err)
	}

	if resp == nil {
		return fmt.Errorf("empty response from CLI")
	}

	if resp.Content == "" {
		return fmt.Errorf("empty content in CLI response")
	}

	// Check for suspicious response time
	if latency < 500*time.Millisecond {
		freeLog.WithFields(logrus.Fields{
			"model":      modelID,
			"latency_ms": latency.Milliseconds(),
			"content":    resp.Content,
		}).Debug("CLI response time - note: CLI typically has startup overhead")
	}

	// Check for canned error responses
	loweredContent := strings.ToLower(resp.Content)
	cannedErrorPatterns := []string{
		"unable to provide",
		"unable to analyze",
		"unable to process",
		"cannot provide",
		"cannot analyze",
		"cannot process",
		"error occurred",
		"service unavailable",
		"rate limit",
		"temporarily unavailable",
		"model not available",
		"failed to generate",
		"internal error",
	}

	for _, pattern := range cannedErrorPatterns {
		if strings.Contains(loweredContent, pattern) {
			return fmt.Errorf("CLI model returned canned error response: %s", resp.Content)
		}
	}

	// STRICT validation - response MUST contain "4" for "2+2" test
	if !strings.Contains(resp.Content, "4") {
		freeLog.WithFields(logrus.Fields{
			"model":      modelID,
			"expected":   "4",
			"got":        resp.Content,
			"latency_ms": latency.Milliseconds(),
		}).Warn("CLI model failed basic math test")
		return fmt.Errorf("CLI model failed basic verification test: expected '4' in response, got: %s", resp.Content)
	}

	freeLog.WithFields(logrus.Fields{
		"model":      modelID,
		"content":    resp.Content,
		"latency_ms": latency.Milliseconds(),
		"via":        "cli_facade",
	}).Debug("CLI model passed verification test")

	return nil
}

// testModelCompletion performs a simple completion test with strict validation
// This function MUST fail verification if:
// 1. The model returns an empty response
// 2. The model returns a canned error response
// 3. The model doesn't answer the test question correctly
// 4. The response time is suspiciously fast (< 100ms typically indicates cached error)
func (fa *FreeProviderAdapter) testModelCompletion(ctx context.Context, provider llm.LLMProvider) error {
	startTime := time.Now()

	// Create a simple test request using the provider's Complete method
	testReq := &models.LLMRequest{
		ID:     fmt.Sprintf("free-test-%d", time.Now().UnixNano()),
		Prompt: "You are a helpful assistant. Reply concisely.",
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "What is 2 + 2? Reply with just the number.",
			},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   10,
			Temperature: 0.0,
		},
	}

	// Perform direct completion test
	resp, err := provider.Complete(ctx, testReq)
	latency := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("completion failed: %w", err)
	}

	// Validate response
	if resp == nil {
		return fmt.Errorf("empty response from provider")
	}

	if resp.Content == "" {
		return fmt.Errorf("empty content in response")
	}

	// Check for suspicious response time (< 100ms usually indicates cached error response)
	if latency < 100*time.Millisecond {
		freeLog.WithFields(logrus.Fields{
			"latency_ms": latency.Milliseconds(),
			"content":    resp.Content,
		}).Warn("Suspiciously fast response - may be cached error")
	}

	// Check for known canned error/failure responses that indicate the model isn't working
	loweredContent := strings.ToLower(resp.Content)
	cannedErrorPatterns := []string{
		"unable to provide",
		"unable to analyze",
		"unable to process",
		"cannot provide",
		"cannot analyze",
		"cannot process",
		"i apologize, but i cannot",
		"i'm sorry, but i cannot",
		"error occurred",
		"service unavailable",
		"rate limit",
		"temporarily unavailable",
		"model not available",
		"failed to generate",
		"no response generated",
		"internal error",
		"at this time", // Catches "Unable to provide analysis at this time"
	}

	for _, pattern := range cannedErrorPatterns {
		if strings.Contains(loweredContent, pattern) {
			return fmt.Errorf("model returned canned error response: %s", resp.Content)
		}
	}

	// STRICT validation - response MUST contain "4" for "2+2" test
	// Models that can't answer basic math should NOT be verified
	if !strings.Contains(resp.Content, "4") {
		freeLog.WithFields(logrus.Fields{
			"expected":   "4",
			"got":        resp.Content,
			"latency_ms": latency.Milliseconds(),
		}).Warn("Model failed basic math test - marking as unverified")
		return fmt.Errorf("model failed basic verification test: expected '4' in response, got: %s", resp.Content)
	}

	freeLog.WithFields(logrus.Fields{
		"content":    resp.Content,
		"latency_ms": latency.Milliseconds(),
	}).Debug("Model passed verification test")

	return nil
}

// testModelCompletionWithModel performs a completion test with a specific model ID.
// This is used for providers like OpenRouter where the model is specified in the request.
func (fa *FreeProviderAdapter) testModelCompletionWithModel(ctx context.Context, provider llm.LLMProvider, modelID string) error {
	startTime := time.Now()

	// Create a simple test request with the specific model
	testReq := &models.LLMRequest{
		ID:     fmt.Sprintf("free-test-%d", time.Now().UnixNano()),
		Prompt: "You are a helpful assistant. Reply concisely.",
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "What is 2 + 2? Reply with just the number.",
			},
		},
		ModelParams: models.ModelParameters{
			Model:       modelID, // Specify the model to test
			MaxTokens:   10,
			Temperature: 0.0,
		},
	}

	// Perform direct completion test
	resp, err := provider.Complete(ctx, testReq)
	latency := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("completion failed: %w", err)
	}

	// Validate response
	if resp == nil {
		return fmt.Errorf("empty response from provider")
	}

	if resp.Content == "" {
		return fmt.Errorf("empty content in response")
	}

	// Check for suspicious response time (< 100ms usually indicates cached error response)
	if latency < 100*time.Millisecond {
		freeLog.WithFields(logrus.Fields{
			"model":      modelID,
			"latency_ms": latency.Milliseconds(),
			"content":    resp.Content,
		}).Warn("Suspiciously fast response - may be cached error")
	}

	// Check for known canned error/failure responses
	loweredContent := strings.ToLower(resp.Content)
	cannedErrorPatterns := []string{
		"unable to provide",
		"unable to analyze",
		"unable to process",
		"cannot provide",
		"cannot analyze",
		"cannot process",
		"i apologize, but i cannot",
		"i'm sorry, but i cannot",
		"error occurred",
		"service unavailable",
		"rate limit",
		"temporarily unavailable",
		"model not available",
		"failed to generate",
		"no response generated",
		"internal error",
		"at this time",
	}

	for _, pattern := range cannedErrorPatterns {
		if strings.Contains(loweredContent, pattern) {
			return fmt.Errorf("model returned canned error response: %s", resp.Content)
		}
	}

	// STRICT validation - response MUST contain "4" for "2+2" test
	if !strings.Contains(resp.Content, "4") {
		freeLog.WithFields(logrus.Fields{
			"model":      modelID,
			"expected":   "4",
			"got":        resp.Content,
			"latency_ms": latency.Milliseconds(),
		}).Warn("Model failed basic math test - marking as unverified")
		return fmt.Errorf("model failed basic verification test: expected '4' in response, got: %s", resp.Content)
	}

	freeLog.WithFields(logrus.Fields{
		"model":      modelID,
		"content":    resp.Content,
		"latency_ms": latency.Milliseconds(),
	}).Debug("Model passed verification test")

	return nil
}

// calculateZenScore calculates the overall Zen provider score
func (fa *FreeProviderAdapter) calculateZenScore(models []verifier.UnifiedModel, healthPassed bool) float64 {
	if len(models) == 0 {
		return 0.0
	}

	// Base score for free providers
	score := fa.config.BaseScore

	// Add bonus for successful health check
	if healthPassed {
		score += 0.2
	}

	// Add bonus based on number of verified models
	verifiedRatio := float64(len(models)) / float64(len(zen.FreeModels()))
	score += verifiedRatio * 0.5

	// Calculate average model latency bonus
	var totalLatency time.Duration
	for _, m := range models {
		totalLatency += m.Latency
	}
	avgLatency := totalLatency / time.Duration(len(models))

	// Latency bonus: faster responses get higher scores
	if avgLatency < 500*time.Millisecond {
		score += 0.3
	} else if avgLatency < 1*time.Second {
		score += 0.2
	} else if avgLatency < 2*time.Second {
		score += 0.1
	}

	// Cap at max score
	if score > fa.config.MaxScore {
		score = fa.config.MaxScore
	}

	return score
}

// calculateModelScore calculates an individual model's score
func (fa *FreeProviderAdapter) calculateModelScore(latency time.Duration, verified bool) float64 {
	if !verified {
		return 0.0
	}

	// Base score
	score := fa.config.BaseScore

	// Latency-based adjustment
	switch {
	case latency < 300*time.Millisecond:
		score += 0.6
	case latency < 500*time.Millisecond:
		score += 0.4
	case latency < 1*time.Second:
		score += 0.2
	case latency < 2*time.Second:
		score += 0.1
	}

	// Cap at max
	if score > fa.config.MaxScore {
		score = fa.config.MaxScore
	}

	return score
}

// VerifyOpenRouterFreeModels verifies OpenRouter :free models
func (fa *FreeProviderAdapter) VerifyOpenRouterFreeModels(ctx context.Context, openRouterAPIKey string) (*verifier.UnifiedProvider, error) {
	freeLog.WithField("provider", "openrouter").Info("Starting OpenRouter free models verification")
	startTime := time.Now()

	// OpenRouter free models have ":free" suffix
	freeModelPatterns := []string{
		"google/gemma-2-9b-it:free",
		"meta-llama/llama-3-8b-instruct:free",
		"mistralai/mistral-7b-instruct:free",
		"microsoft/phi-3-mini-128k-instruct:free",
		"qwen/qwen-2-7b-instruct:free",
	}

	models := make([]verifier.UnifiedModel, 0)
	var wg sync.WaitGroup
	var localMu sync.Mutex
	sem := make(chan struct{}, fa.config.MaxConcurrentVerifications)

	for _, modelID := range freeModelPatterns {
		wg.Add(1)
		go func(mID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			model, err := fa.verifyOpenRouterFreeModel(ctx, mID, openRouterAPIKey)
			if err != nil {
				freeLog.WithFields(logrus.Fields{
					"provider": "openrouter",
					"model":    mID,
					"error":    err.Error(),
				}).Debug("OpenRouter free model not available")
				return
			}

			localMu.Lock()
			models = append(models, *model)
			localMu.Unlock()
			fa.verifiedModels.Put(mID, model)
			fa.lastVerified.Put(mID, time.Now())
		}(modelID)
	}

	wg.Wait()

	// Calculate score
	score := fa.calculateOpenRouterScore(models)

	// Determine status
	status := verifier.StatusVerified
	if len(models) == 0 {
		status = verifier.StatusUnavailable
	} else if len(models) < 2 {
		status = verifier.StatusDegraded
	}

	provider := &verifier.UnifiedProvider{
		ID:           "openrouter-free",
		Name:         "OpenRouter Free Tier",
		Type:         "free",
		AuthType:     verifier.AuthTypeFree,
		Verified:     len(models) > 0,
		Score:        score,
		Models:       models,
		Status:       status,
		BaseURL:      "https://openrouter.ai/api/v1",
		APIKey:       openRouterAPIKey,
		VerifiedAt:   time.Now(),
		LastHealthAt: time.Now(),
		ErrorCount:   0,
		Metadata: map[string]interface{}{
			"verification_time_ms": time.Since(startTime).Milliseconds(),
			"verified_models":      len(models),
			"total_models":         len(freeModelPatterns),
			"free_tier":            true,
		},
	}

	freeLog.WithFields(logrus.Fields{
		"provider":        "openrouter",
		"verified_models": len(models),
		"score":           score,
		"status":          status,
		"duration_ms":     time.Since(startTime).Milliseconds(),
	}).Info("OpenRouter free models verification completed")

	return provider, nil
}

// verifyOpenRouterFreeModel verifies a single OpenRouter free model
// This now does PROPER model completion verification (not just health check)
// to detect canned error responses and ensure the model actually works.
func (fa *FreeProviderAdapter) verifyOpenRouterFreeModel(ctx context.Context, modelID, apiKey string) (*verifier.UnifiedModel, error) {
	startTime := time.Now()

	// Create an OpenRouter provider
	// OpenRouter free models can work without an API key for some models
	modelProvider := openrouter.NewSimpleOpenRouterProvider(apiKey)

	// Perform reduced verification (health check + simple completion)
	var verificationErr error
	var latency time.Duration
	verified := false

	for attempt := 0; attempt <= fa.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(fa.config.RetryDelay):
			}
		}

		// Health check
		if err := modelProvider.HealthCheck(); err != nil {
			verificationErr = err
			continue
		}

		// CRITICAL: Do actual model completion test with the specific model ID
		// This tests the model for canned error responses and proper functionality
		testStart := time.Now()
		if err := fa.testModelCompletionWithModel(ctx, modelProvider, modelID); err != nil {
			verificationErr = err
			freeLog.WithFields(logrus.Fields{
				"provider": "openrouter",
				"model":    modelID,
				"error":    err.Error(),
				"attempt":  attempt + 1,
			}).Debug("OpenRouter model completion test failed")
			continue
		}
		latency = time.Since(testStart)
		verified = true
		break
	}

	if !verified {
		return nil, fmt.Errorf("verification failed after %d attempts: %v", fa.config.RetryAttempts+1, verificationErr)
	}

	// Calculate model score (free models score between 6.0-7.0)
	score := fa.calculateModelScore(latency, verified)

	model := &verifier.UnifiedModel{
		ID:       modelID,
		Name:     getOpenRouterModelName(modelID),
		Provider: "openrouter",
		Verified: verified,
		Score:    score,
		Latency:  latency,
		Capabilities: []string{
			"text_completion",
			"chat",
			"streaming",
		},
		Metadata: map[string]interface{}{
			"free_model":           true,
			"verification_time_ms": time.Since(startTime).Milliseconds(),
			"latency_ms":           latency.Milliseconds(),
		},
	}

	return model, nil
}

// calculateOpenRouterScore calculates the OpenRouter free tier score
func (fa *FreeProviderAdapter) calculateOpenRouterScore(models []verifier.UnifiedModel) float64 {
	if len(models) == 0 {
		return 0.0
	}

	// Base score
	score := fa.config.BaseScore

	// Bonus for multiple verified models
	if len(models) >= 3 {
		score += 0.4
	} else if len(models) >= 2 {
		score += 0.2
	}

	// Average latency bonus
	var totalLatency time.Duration
	for _, m := range models {
		totalLatency += m.Latency
	}
	avgLatency := totalLatency / time.Duration(len(models))

	if avgLatency < 500*time.Millisecond {
		score += 0.3
	} else if avgLatency < 1*time.Second {
		score += 0.2
	}

	if score > fa.config.MaxScore {
		score = fa.config.MaxScore
	}

	return score
}

// VerifyAllFreeProviders verifies all free providers and returns them
func (fa *FreeProviderAdapter) VerifyAllFreeProviders(ctx context.Context, openRouterAPIKey string) ([]*verifier.UnifiedProvider, error) {
	freeLog.Info("Starting verification of all free providers")
	startTime := time.Now()

	providers := make([]*verifier.UnifiedProvider, 0, 2)
	var wg sync.WaitGroup
	var providersMu sync.Mutex

	// Verify Zen
	wg.Add(1)
	go func() {
		defer wg.Done()
		zenProvider, err := fa.VerifyZenProvider(ctx)
		if err != nil {
			freeLog.WithField("error", err.Error()).Warn("Failed to verify Zen provider")
			return
		}
		providersMu.Lock()
		providers = append(providers, zenProvider)
		providersMu.Unlock()
	}()

	// Verify OpenRouter free models (if API key provided)
	if openRouterAPIKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orProvider, err := fa.VerifyOpenRouterFreeModels(ctx, openRouterAPIKey)
			if err != nil {
				freeLog.WithField("error", err.Error()).Warn("Failed to verify OpenRouter free models")
				return
			}
			providersMu.Lock()
			providers = append(providers, orProvider)
			providersMu.Unlock()
		}()
	}

	wg.Wait()

	freeLog.WithFields(logrus.Fields{
		"verified_providers": len(providers),
		"duration_ms":        time.Since(startTime).Milliseconds(),
	}).Info("Free providers verification completed")

	return providers, nil
}

// GetVerifiedModels returns all verified free models
func (fa *FreeProviderAdapter) GetVerifiedModels() map[string]*verifier.UnifiedModel {
	return fa.verifiedModels.Snapshot()
}

// IsModelVerified checks if a model is verified
func (fa *FreeProviderAdapter) IsModelVerified(modelID string) bool {
	return fa.verifiedModels.Has(modelID)
}

// GetHealthStatus returns the health status of free providers
func (fa *FreeProviderAdapter) GetHealthStatus() map[string]bool {
	return fa.healthStatus.Snapshot()
}

// RefreshVerification re-verifies a specific provider
func (fa *FreeProviderAdapter) RefreshVerification(ctx context.Context, providerType FreeProviderType, apiKey string) (*verifier.UnifiedProvider, error) {
	switch providerType {
	case FreeProviderZen:
		return fa.VerifyZenProvider(ctx)
	case FreeProviderOpenRouter:
		return fa.VerifyOpenRouterFreeModels(ctx, apiKey)
	default:
		return nil, fmt.Errorf("unknown free provider type: %s", providerType)
	}
}

// IsCLIFacadeAvailable returns whether the OpenCode CLI facade is available
func (fa *FreeProviderAdapter) IsCLIFacadeAvailable() bool {
	return fa.zenCLIProvider != nil && fa.zenCLIProvider.IsCLIAvailable()
}

// GetCLIFacadeProvider returns the CLI facade provider instance (for external use)
func (fa *FreeProviderAdapter) GetCLIFacadeProvider() *zen.ZenCLIProvider {
	return fa.zenCLIProvider
}

// GetFailedAPIModels returns models that failed direct API verification
func (fa *FreeProviderAdapter) GetFailedAPIModels() map[string]error {
	return fa.failedAPIModels.Snapshot()
}

// IsModelUsingCLIFacade checks if a model is being used via CLI facade
func (fa *FreeProviderAdapter) IsModelUsingCLIFacade(modelID string) bool {
	model, ok := fa.verifiedModels.Get(modelID)
	if !ok {
		return false
	}
	if model.Metadata == nil {
		return false
	}
	verifiedVia, ok := model.Metadata["verified_via"]
	if !ok {
		return false
	}
	return verifiedVia == "cli_facade"
}

// GetCLIFacadeModels returns all models that are verified via CLI facade
func (fa *FreeProviderAdapter) GetCLIFacadeModels() []*verifier.UnifiedModel {
	var result []*verifier.UnifiedModel
	fa.verifiedModels.Range(func(_ string, model *verifier.UnifiedModel) bool {
		if model.Metadata != nil {
			if verifiedVia, ok := model.Metadata["verified_via"]; ok && verifiedVia == "cli_facade" {
				result = append(result, model)
			}
		}
		return true
	})
	return result
}

// Helper functions

// getModelDisplayName returns a user-friendly display name for a model
func getModelDisplayName(modelID string) string {
	nameMap := map[string]string{
		zen.ModelBigPickle:               "Big Pickle",
		zen.ModelGLM5Free:                "GLM 5 Free",
		zen.ModelMinimaxM25Free:          "Minimax M2.5 Free",
		zen.ModelMinimaxM21Free:          "Minimax M2.1 Free",
		zen.ModelTrinityLargePreviewFree: "Trinity Large Preview",
	}

	if name, ok := nameMap[modelID]; ok {
		return name
	}

	// Extract name from model ID
	parts := strings.Split(modelID, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return modelID
}

// getOpenRouterModelName returns a display name for OpenRouter models
func getOpenRouterModelName(modelID string) string {
	// Remove :free suffix and provider prefix
	name := strings.TrimSuffix(modelID, ":free")
	parts := strings.Split(name, "/")
	if len(parts) > 1 {
		return fmt.Sprintf("%s (Free)", parts[1])
	}
	return fmt.Sprintf("%s (Free)", name)
}

// convertCapabilities converts provider capabilities to a string slice
func convertCapabilities(caps *models.ProviderCapabilities) []string {
	if caps == nil {
		return []string{}
	}

	result := make([]string, 0, len(caps.SupportedFeatures))
	result = append(result, caps.SupportedFeatures...)

	if caps.SupportsStreaming {
		result = append(result, "streaming")
	}
	if caps.SupportsFunctionCalling {
		result = append(result, "function_calling")
	}
	if caps.SupportsVision {
		result = append(result, "vision")
	}
	if caps.SupportsTools {
		result = append(result, "tools")
	}
	if caps.SupportsReasoning {
		result = append(result, "reasoning")
	}

	return result
}
