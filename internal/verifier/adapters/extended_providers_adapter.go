// Package adapters provides provider-specific verification adapters for the startup verification system.
package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"

	verifier "dev.helix.agent/internal/verifier"
)

var extLog = logrus.New()

// ExtendedProviderConfig holds configuration for extended provider adapters
type ExtendedProviderConfig struct {
	VerificationTimeout        time.Duration
	HealthCheckTimeout         time.Duration
	MaxConcurrentVerifications int
	RetryAttempts              int
	RetryDelay                 time.Duration
	MinScoreThreshold          float64
}

// DefaultExtendedProviderConfig returns the default configuration
func DefaultExtendedProviderConfig() *ExtendedProviderConfig {
	return &ExtendedProviderConfig{
		VerificationTimeout:        30 * time.Second,
		HealthCheckTimeout:         10 * time.Second,
		MaxConcurrentVerifications: 5,
		RetryAttempts:              2,
		RetryDelay:                 1 * time.Second,
		MinScoreThreshold:          5.0,
	}
}

// ExtendedProvidersAdapter handles verification for newly added LLM providers
// (Grok, Perplexity, Cohere, AI21, Together, Fireworks, Anyscale, DeepInfra, Lepton, SambaNova).
//
// Concurrency model (CONST-029): verifiedModels and providerResults
// are safe.Store containers. Concurrent VerifyProvider calls publish
// results into both stores atomically without any caller-held lock.
// The previous design relied on a shared mu, but verifyModel's write
// to epa.verifiedModels was actually guarded by a local modelsMu
// (scoped to one provider's verification) — concurrent verifications
// of different providers therefore raced on epa.verifiedModels;
// safe.Store resolves that latent hazard.
type ExtendedProvidersAdapter struct {
	config     *ExtendedProviderConfig
	httpClient *http.Client

	// Cached results
	verifiedModels  *safe.Store[string, *verifier.UnifiedModel]
	providerResults *safe.Store[string, *verifier.UnifiedProvider]
}

// NewExtendedProvidersAdapter creates a new extended providers adapter
func NewExtendedProvidersAdapter(config *ExtendedProviderConfig) *ExtendedProvidersAdapter {
	if config == nil {
		config = DefaultExtendedProviderConfig()
	}

	return &ExtendedProvidersAdapter{
		config: config,
		httpClient: &http.Client{
			Timeout: config.VerificationTimeout,
		},
		verifiedModels:  safe.NewStore[string, *verifier.UnifiedModel](),
		providerResults: safe.NewStore[string, *verifier.UnifiedProvider](),
	}
}

// ProviderVerificationRequest contains the info needed to verify a provider
type ProviderVerificationRequest struct {
	ProviderID   string
	ProviderName string
	APIKey       string
	BaseURL      string
	Models       []string
	AuthType     verifier.ProviderAuthType
	Tier         int
	Priority     int
}

// OpenAICompletionRequest is the standard OpenAI-compatible request format
type OpenAICompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// OpenAIMessage represents a message in the OpenAI format
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAICompletionResponse is the standard OpenAI-compatible response format
type OpenAICompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      OpenAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// CohereRequest is the Cohere-specific request format
type CohereRequest struct {
	Model     string `json:"model"`
	Message   string `json:"message"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Preamble  string `json:"preamble,omitempty"`
}

// CohereResponse is the Cohere-specific response format
type CohereResponse struct {
	Text         string `json:"text"`
	GenerationID string `json:"generation_id"`
	FinishReason string `json:"finish_reason"`
}

// VerifyProvider verifies a single provider and returns UnifiedProvider
func (epa *ExtendedProvidersAdapter) VerifyProvider(ctx context.Context, req *ProviderVerificationRequest) (*verifier.UnifiedProvider, error) {
	extLog.WithFields(logrus.Fields{
		"provider": req.ProviderID,
		"models":   len(req.Models),
	}).Info("Starting provider verification")

	startTime := time.Now()

	// Verify each model
	models := make([]verifier.UnifiedModel, 0, len(req.Models))
	var wg sync.WaitGroup
	var modelsMu sync.Mutex
	sem := make(chan struct{}, epa.config.MaxConcurrentVerifications)

	for _, modelID := range req.Models {
		wg.Add(1)
		go func(mID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			model, err := epa.verifyModel(ctx, req, mID)
			if err != nil {
				extLog.WithFields(logrus.Fields{
					"provider": req.ProviderID,
					"model":    mID,
					"error":    err.Error(),
				}).Debug("Model verification failed")
				return
			}

			modelsMu.Lock()
			models = append(models, *model)
			modelsMu.Unlock()
			epa.verifiedModels.Put(fmt.Sprintf("%s:%s", req.ProviderID, mID), model)
		}(modelID)
	}

	wg.Wait()

	// Calculate provider score
	score := epa.calculateProviderScore(models, req)

	// Determine status
	status := verifier.StatusVerified
	if len(models) == 0 {
		status = verifier.StatusFailed
	} else if len(models) < len(req.Models) {
		status = verifier.StatusDegraded
	}

	provider := &verifier.UnifiedProvider{
		ID:           req.ProviderID,
		Name:         req.ProviderName,
		Type:         req.ProviderID,
		AuthType:     req.AuthType,
		Verified:     len(models) > 0,
		Score:        score,
		Models:       models,
		Status:       status,
		BaseURL:      req.BaseURL,
		APIKey:       req.APIKey,
		Tier:         req.Tier,
		Priority:     req.Priority,
		VerifiedAt:   time.Now(),
		LastHealthAt: time.Now(),
		ErrorCount:   0,
		Metadata: map[string]interface{}{
			"verification_time_ms": time.Since(startTime).Milliseconds(),
			"verified_models":      len(models),
			"total_models":         len(req.Models),
		},
	}

	// Set primary model (highest scoring)
	if len(models) > 0 {
		var bestModel *verifier.UnifiedModel
		for i := range models {
			if bestModel == nil || models[i].Score > bestModel.Score {
				bestModel = &models[i]
			}
		}
		provider.PrimaryModel = bestModel
		provider.DefaultModel = bestModel.ID
	}

	// Cache result
	epa.providerResults.Put(req.ProviderID, provider)

	extLog.WithFields(logrus.Fields{
		"provider":        req.ProviderID,
		"verified_models": len(models),
		"score":           score,
		"status":          status,
		"duration_ms":     time.Since(startTime).Milliseconds(),
	}).Info("Provider verification completed")

	return provider, nil
}

// verifyModel verifies a single model
func (epa *ExtendedProvidersAdapter) verifyModel(ctx context.Context, req *ProviderVerificationRequest, modelID string) (*verifier.UnifiedModel, error) {
	startTime := time.Now()

	var latency time.Duration
	var verified bool
	var verificationErr error
	var testResults = make(map[string]bool)

	for attempt := 0; attempt <= epa.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(epa.config.RetryDelay):
			}
		}

		// Test 1: Basic completion
		testStart := time.Now()
		err := epa.testCompletion(ctx, req, modelID)
		if err != nil {
			verificationErr = err
			testResults["basic_completion"] = false
			continue
		}
		testResults["basic_completion"] = true
		latency = time.Since(testStart)

		// Test 2: Code visibility (asks if model can see code)
		err = epa.testCodeVisibility(ctx, req, modelID)
		testResults["code_visibility"] = err == nil

		// Test 3: JSON mode (if supported)
		err = epa.testJSONMode(ctx, req, modelID)
		testResults["json_mode"] = err == nil

		verified = true
		break
	}

	if !verified {
		return nil, fmt.Errorf("verification failed after %d attempts: %v", epa.config.RetryAttempts+1, verificationErr)
	}

	// Calculate model score
	score := epa.calculateModelScore(latency, testResults, req)

	model := &verifier.UnifiedModel{
		ID:           modelID,
		Name:         getModelDisplayNameExt(modelID),
		Provider:     req.ProviderID,
		Verified:     verified,
		Score:        score,
		Latency:      latency,
		TestResults:  testResults,
		Capabilities: epa.inferCapabilities(req.ProviderID, modelID),
		Metadata: map[string]interface{}{
			"verification_time_ms": time.Since(startTime).Milliseconds(),
			"latency_ms":           latency.Milliseconds(),
			"tests_passed":         countPassedTests(testResults),
			"total_tests":          len(testResults),
		},
	}

	return model, nil
}

// testCompletion performs a basic completion test
func (epa *ExtendedProvidersAdapter) testCompletion(ctx context.Context, req *ProviderVerificationRequest, modelID string) error {
	// Handle Cohere's different API format
	if req.ProviderID == "cohere" {
		return epa.testCohereCompletion(ctx, req, modelID)
	}

	// Standard OpenAI-compatible completion
	reqBody := OpenAICompletionRequest{
		Model: modelID,
		Messages: []OpenAIMessage{
			{Role: "user", Content: "What is 2 + 2? Reply with just the number."},
		},
		MaxTokens:   10,
		Temperature: 0.0,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.BaseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	epa.setHeaders(httpReq, req)

	resp, err := epa.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("API returned status %d - failed to read response body: %v", resp.StatusCode, err)
		}
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response OpenAICompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("no choices in response")
	}

	content := response.Choices[0].Message.Content
	if content == "" {
		return fmt.Errorf("empty response content")
	}

	// Validate response contains "4"
	if !strings.Contains(content, "4") {
		extLog.WithFields(logrus.Fields{
			"expected": "4",
			"got":      content,
		}).Debug("Unexpected response content, but provider is responsive")
	}

	return nil
}

// testCohereCompletion tests Cohere's specific API format
func (epa *ExtendedProvidersAdapter) testCohereCompletion(ctx context.Context, req *ProviderVerificationRequest, modelID string) error {
	reqBody := CohereRequest{
		Model:     modelID,
		Message:   "What is 2 + 2? Reply with just the number.",
		MaxTokens: 10,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.BaseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	epa.setHeaders(httpReq, req)

	resp, err := epa.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("API returned status %d - failed to read response body: %v", resp.StatusCode, err)
		}
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response CohereResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Text == "" {
		return fmt.Errorf("empty response text")
	}

	return nil
}

// testCodeVisibility tests if the model can see code context
func (epa *ExtendedProvidersAdapter) testCodeVisibility(ctx context.Context, req *ProviderVerificationRequest, modelID string) error {
	if req.ProviderID == "cohere" {
		return nil // Skip for Cohere (different format)
	}

	reqBody := OpenAICompletionRequest{
		Model: modelID,
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a code assistant. Analyze code provided by the user."},
			{Role: "user", Content: "Can you see and analyze the following code?\n```go\nfunc hello() { fmt.Println(\"Hello\") }\n```\nReply YES or NO."},
		},
		MaxTokens:   20,
		Temperature: 0.0,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.BaseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	epa.setHeaders(httpReq, req)

	resp, err := epa.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var response OpenAICompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("no choices")
	}

	content := strings.ToUpper(response.Choices[0].Message.Content)
	if !strings.Contains(content, "YES") {
		return fmt.Errorf("model cannot see code")
	}

	return nil
}

// testJSONMode tests if the model supports JSON mode output
func (epa *ExtendedProvidersAdapter) testJSONMode(ctx context.Context, req *ProviderVerificationRequest, modelID string) error {
	if req.ProviderID == "cohere" || req.ProviderID == "perplexity" {
		return nil // Skip for providers without JSON mode
	}

	reqBody := map[string]interface{}{
		"model": modelID,
		"messages": []OpenAIMessage{
			{Role: "user", Content: "Return the result of 2+2 as JSON with a 'result' field."},
		},
		"max_tokens":      50,
		"temperature":     0.0,
		"response_format": map[string]string{"type": "json_object"},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.BaseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	epa.setHeaders(httpReq, req)

	resp, err := epa.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Some providers don't support JSON mode, that's OK
	if resp.StatusCode == http.StatusBadRequest {
		return fmt.Errorf("JSON mode not supported")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	return nil
}

// setHeaders sets the appropriate headers for the request
func (epa *ExtendedProvidersAdapter) setHeaders(httpReq *http.Request, req *ProviderVerificationRequest) {
	httpReq.Header.Set("Content-Type", "application/json")

	switch req.ProviderID {
	case "cohere":
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
		httpReq.Header.Set("Accept", "application/json")
	case "perplexity":
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	case "ai21":
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	default:
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}
}

// calculateProviderScore calculates the overall provider score
func (epa *ExtendedProvidersAdapter) calculateProviderScore(models []verifier.UnifiedModel, req *ProviderVerificationRequest) float64 {
	if len(models) == 0 {
		return 0.0
	}

	// Base score from model scores (average)
	var totalScore float64
	for _, m := range models {
		totalScore += m.Score
	}
	avgScore := totalScore / float64(len(models))

	// Tier bonus (higher tier = better base)
	tierBonus := float64(8-req.Tier) * 0.2 // Tier 1 = +1.4, Tier 7 = +0.2

	// Model count bonus
	modelBonus := 0.0
	if len(models) >= 3 {
		modelBonus = 0.3
	} else if len(models) >= 2 {
		modelBonus = 0.2
	}

	score := avgScore + tierBonus + modelBonus

	// Cap at 10
	if score > 10.0 {
		score = 10.0
	}

	return score
}

// calculateModelScore calculates an individual model's score
func (epa *ExtendedProvidersAdapter) calculateModelScore(latency time.Duration, testResults map[string]bool, req *ProviderVerificationRequest) float64 {
	// Base score from tier
	baseScore := float64(10 - req.Tier) // Tier 1 = 9, Tier 7 = 3

	// Latency adjustment
	latencyBonus := 0.0
	switch {
	case latency < 500*time.Millisecond:
		latencyBonus = 1.0
	case latency < 1*time.Second:
		latencyBonus = 0.7
	case latency < 2*time.Second:
		latencyBonus = 0.4
	case latency < 5*time.Second:
		latencyBonus = 0.1
	}

	// Test results bonus
	testBonus := 0.0
	if testResults["code_visibility"] {
		testBonus += 0.5
	}
	if testResults["json_mode"] {
		testBonus += 0.3
	}

	score := baseScore + latencyBonus + testBonus

	// Ensure minimum threshold
	if score < epa.config.MinScoreThreshold {
		score = epa.config.MinScoreThreshold
	}

	// Cap at 10
	if score > 10.0 {
		score = 10.0
	}

	return score
}

// inferCapabilities infers model capabilities based on provider and model ID
func (epa *ExtendedProvidersAdapter) inferCapabilities(providerID, modelID string) []string {
	caps := []string{"text_completion", "chat"}

	modelLower := strings.ToLower(modelID)

	// Streaming (all providers support)
	caps = append(caps, "streaming")

	// Vision
	if strings.Contains(modelLower, "vision") || strings.Contains(modelLower, "4o") ||
		providerID == "grok" && strings.Contains(modelLower, "vision") {
		caps = append(caps, "vision")
	}

	// Function calling / Tools
	if providerID == "grok" || providerID == "together" || providerID == "fireworks" ||
		providerID == "anyscale" || providerID == "deepinfra" {
		caps = append(caps, "function_calling", "tools")
	}

	// Code generation
	if strings.Contains(modelLower, "code") || strings.Contains(modelLower, "instruct") {
		caps = append(caps, "code_generation")
	}

	// Online search (Perplexity specific)
	if providerID == "perplexity" && strings.Contains(modelLower, "online") {
		caps = append(caps, "web_search", "realtime_info")
	}

	// RAG (Cohere specific)
	if providerID == "cohere" {
		caps = append(caps, "rag", "embeddings")
	}

	return caps
}

// VerifyAllExtendedProviders verifies all extended providers and returns verified ones
func (epa *ExtendedProvidersAdapter) VerifyAllExtendedProviders(ctx context.Context) ([]*verifier.UnifiedProvider, error) {
	extLog.Info("Starting verification of all extended providers")
	startTime := time.Now()

	// Extended providers to verify
	providersToVerify := []string{
		"grok", "perplexity", "cohere", "ai21", "together",
		"fireworks", "anyscale", "deepinfra", "lepton", "sambanova",
	}

	providers := make([]*verifier.UnifiedProvider, 0)
	var wg sync.WaitGroup
	var providersMu sync.Mutex
	sem := make(chan struct{}, epa.config.MaxConcurrentVerifications)

	for _, providerID := range providersToVerify {
		wg.Add(1)
		go func(pID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Get provider info
			info, ok := verifier.GetProviderInfo(pID)
			if !ok {
				extLog.WithField("provider", pID).Debug("Provider info not found")
				return
			}

			// Get API key from environment
			apiKey := epa.getAPIKey(info.EnvVars)
			if apiKey == "" {
				extLog.WithField("provider", pID).Debug("No API key found")
				return
			}

			// Create verification request
			req := &ProviderVerificationRequest{
				ProviderID:   pID,
				ProviderName: info.DisplayName,
				APIKey:       apiKey,
				BaseURL:      info.BaseURL,
				Models:       info.Models,
				AuthType:     info.AuthType,
				Tier:         info.Tier,
				Priority:     info.Priority,
			}

			// Verify provider
			provider, err := epa.VerifyProvider(ctx, req)
			if err != nil {
				extLog.WithFields(logrus.Fields{
					"provider": pID,
					"error":    err.Error(),
				}).Warn("Provider verification failed")
				return
			}

			if provider.Verified {
				providersMu.Lock()
				providers = append(providers, provider)
				providersMu.Unlock()
			}
		}(providerID)
	}

	wg.Wait()

	extLog.WithFields(logrus.Fields{
		"verified_providers": len(providers),
		"total_providers":    len(providersToVerify),
		"duration_ms":        time.Since(startTime).Milliseconds(),
	}).Info("Extended providers verification completed")

	return providers, nil
}

// getAPIKey retrieves API key from environment variables
func (epa *ExtendedProvidersAdapter) getAPIKey(envVars []string) string {
	for _, envVar := range envVars {
		if key := os.Getenv(envVar); key != "" {
			return key
		}
	}
	return ""
}

// GetVerifiedProviders returns cached verified providers
func (epa *ExtendedProvidersAdapter) GetVerifiedProviders() map[string]*verifier.UnifiedProvider {
	return epa.providerResults.Snapshot()
}

// Helper functions

// getModelDisplayNameExt returns a user-friendly display name for a model
func getModelDisplayNameExt(modelID string) string {
	// Remove provider prefix if present
	parts := strings.Split(modelID, "/")
	if len(parts) > 1 {
		modelID = parts[len(parts)-1]
	}

	// Clean up common suffixes
	name := modelID
	name = strings.ReplaceAll(name, "-Instruct-Turbo", " Instruct")
	name = strings.ReplaceAll(name, "-Instruct", " Instruct")
	name = strings.ReplaceAll(name, "-instruct", " Instruct")
	name = strings.ReplaceAll(name, "-chat-hf", " Chat")

	return name
}

// countPassedTests counts the number of passed tests
func countPassedTests(results map[string]bool) int {
	count := 0
	for _, passed := range results {
		if passed {
			count++
		}
	}
	return count
}
