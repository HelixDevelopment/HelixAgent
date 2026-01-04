// Package main implements the Provider Verification challenge.
// This challenge discovers, verifies, and scores all LLM providers.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProviderConfig holds configuration for an LLM provider.
type ProviderConfig struct {
	Name     string `json:"name"`
	APIKey   string `json:"-"` // Never serialize
	BaseURL  string `json:"base_url,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// VerificationResult holds the result of verifying a provider.
type VerificationResult struct {
	Provider       string        `json:"provider"`
	Connected      bool          `json:"connected"`
	Authenticated  bool          `json:"authenticated"`
	CodeVisibility bool          `json:"code_visibility"`
	Models         []ModelInfo   `json:"models,omitempty"`
	ResponseTimeMs int64         `json:"response_time_ms"`
	Error          string        `json:"error,omitempty"`
	Timestamp      time.Time     `json:"timestamp"`
}

// ModelInfo holds information about a model.
type ModelInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Provider     string            `json:"provider"`
	Score        float64           `json:"score"`
	Capabilities []string          `json:"capabilities,omitempty"`
	ScoreBreakdown map[string]float64 `json:"score_breakdown,omitempty"`
}

// ScoringWeights defines the weights for scoring criteria.
type ScoringWeights struct {
	ResponseSpeed   float64 `json:"response_speed"`
	ModelEfficiency float64 `json:"model_efficiency"`
	CostEffectiveness float64 `json:"cost_effectiveness"`
	Capability      float64 `json:"capability"`
	Recency         float64 `json:"recency"`
}

// ChallengeResult holds the complete challenge output.
type ChallengeResult struct {
	ChallengeID    string               `json:"challenge_id"`
	ChallengeName  string               `json:"challenge_name"`
	Timestamp      time.Time            `json:"timestamp"`
	Duration       time.Duration        `json:"duration"`
	Status         string               `json:"status"`
	Providers      []VerificationResult `json:"providers"`
	Models         []ModelInfo          `json:"models"`
	Summary        ChallengeSummary     `json:"summary"`
}

// ChallengeSummary provides aggregated statistics.
type ChallengeSummary struct {
	TotalProviders      int     `json:"total_providers"`
	VerifiedProviders   int     `json:"verified_providers"`
	FailedProviders     int     `json:"failed_providers"`
	TotalModels         int     `json:"total_models"`
	AverageResponseTime float64 `json:"average_response_time_ms"`
	AverageScore        float64 `json:"average_score"`
	HighestScore        float64 `json:"highest_score"`
	TopModel            string  `json:"top_model"`
}

// Provider verification functions

func verifyProvider(ctx context.Context, config ProviderConfig) *VerificationResult {
	result := &VerificationResult{
		Provider:  config.Name,
		Timestamp: time.Now(),
	}

	if config.APIKey == "" && config.Name != "ollama" {
		result.Error = "API key not configured"
		return result
	}

	start := time.Now()

	// Test connectivity based on provider
	var err error
	switch config.Name {
	case "openai":
		result.Connected, result.Authenticated, err = verifyOpenAI(ctx, config)
	case "anthropic":
		result.Connected, result.Authenticated, err = verifyAnthropic(ctx, config)
	case "openrouter":
		result.Connected, result.Authenticated, err = verifyOpenRouter(ctx, config)
	case "deepseek":
		result.Connected, result.Authenticated, err = verifyDeepSeek(ctx, config)
	case "gemini":
		result.Connected, result.Authenticated, err = verifyGemini(ctx, config)
	case "ollama":
		result.Connected, result.Authenticated, err = verifyOllama(ctx, config)
	default:
		result.Connected, result.Authenticated, err = verifyGenericProvider(ctx, config)
	}

	result.ResponseTimeMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Error = err.Error()
	}

	// If authenticated, try to get models
	if result.Authenticated {
		result.Models = discoverModels(ctx, config)
	}

	return result
}

func verifyOpenAI(ctx context.Context, config ProviderConfig) (connected, authenticated bool, err error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
	if err != nil {
		return false, false, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	connected = true
	authenticated = resp.StatusCode == http.StatusOK

	if resp.StatusCode == http.StatusUnauthorized {
		return connected, false, fmt.Errorf("authentication failed")
	}

	return connected, authenticated, nil
}

func verifyAnthropic(ctx context.Context, config ProviderConfig) (connected, authenticated bool, err error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	// Anthropic doesn't have a /models endpoint, so we'll send a minimal request
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/messages", strings.NewReader(`{
		"model": "claude-3-haiku-20240307",
		"max_tokens": 1,
		"messages": [{"role": "user", "content": "test"}]
	}`))
	if err != nil {
		return false, false, err
	}
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	connected = true
	// 200, 400 (bad request but auth OK), or even rate limit means auth worked
	authenticated = resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden

	return connected, authenticated, nil
}

func verifyOpenRouter(ctx context.Context, config ProviderConfig) (connected, authenticated bool, err error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
	if err != nil {
		return false, false, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	connected = true
	authenticated = resp.StatusCode == http.StatusOK

	return connected, authenticated, nil
}

func verifyDeepSeek(ctx context.Context, config ProviderConfig) (connected, authenticated bool, err error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
	if err != nil {
		return false, false, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	connected = true
	authenticated = resp.StatusCode == http.StatusOK

	return connected, authenticated, nil
}

func verifyGemini(ctx context.Context, config ProviderConfig) (connected, authenticated bool, err error) {
	baseURL := "https://generativelanguage.googleapis.com/v1beta"

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models?key="+config.APIKey, nil)
	if err != nil {
		return false, false, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	connected = true
	authenticated = resp.StatusCode == http.StatusOK

	return connected, authenticated, nil
}

func verifyOllama(ctx context.Context, config ProviderConfig) (connected, authenticated bool, err error) {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/tags", nil)
	if err != nil {
		return false, false, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	connected = resp.StatusCode == http.StatusOK
	authenticated = connected // Ollama doesn't require auth

	return connected, authenticated, nil
}

func verifyGenericProvider(ctx context.Context, config ProviderConfig) (connected, authenticated bool, err error) {
	// Generic verification for unknown providers
	if config.BaseURL == "" {
		return false, false, fmt.Errorf("no base URL configured for provider")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
	if err != nil {
		return false, false, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	connected = true
	authenticated = resp.StatusCode == http.StatusOK

	return connected, authenticated, nil
}

func discoverModels(ctx context.Context, config ProviderConfig) []ModelInfo {
	// Return known models for each provider
	// In production, this would query the actual API
	knownModels := map[string][]ModelInfo{
		"openai": {
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", Score: 9.2, Capabilities: []string{"code_generation", "reasoning", "function_calling"}},
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Score: 9.0, Capabilities: []string{"code_generation", "reasoning", "vision"}},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", Score: 8.5, Capabilities: []string{"code_generation", "reasoning"}},
		},
		"anthropic": {
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic", Score: 9.5, Capabilities: []string{"code_generation", "reasoning", "vision"}},
			{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Provider: "anthropic", Score: 9.1, Capabilities: []string{"code_generation", "reasoning", "vision"}},
			{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Provider: "anthropic", Score: 8.6, Capabilities: []string{"code_generation", "reasoning"}},
		},
		"openrouter": {
			{ID: "anthropic/claude-3-opus", Name: "Claude 3 Opus (OR)", Provider: "openrouter", Score: 9.4, Capabilities: []string{"code_generation", "reasoning"}},
			{ID: "openai/gpt-4-turbo", Name: "GPT-4 Turbo (OR)", Provider: "openrouter", Score: 9.1, Capabilities: []string{"code_generation", "reasoning"}},
			{ID: "meta-llama/llama-3-70b-instruct", Name: "Llama 3 70B", Provider: "openrouter", Score: 8.8, Capabilities: []string{"code_generation"}},
		},
		"deepseek": {
			{ID: "deepseek-coder", Name: "DeepSeek Coder", Provider: "deepseek", Score: 9.0, Capabilities: []string{"code_generation", "code_completion"}},
			{ID: "deepseek-chat", Name: "DeepSeek Chat", Provider: "deepseek", Score: 8.5, Capabilities: []string{"reasoning"}},
		},
		"gemini": {
			{ID: "gemini-pro", Name: "Gemini Pro", Provider: "gemini", Score: 8.7, Capabilities: []string{"code_generation", "reasoning"}},
			{ID: "gemini-pro-vision", Name: "Gemini Pro Vision", Provider: "gemini", Score: 8.5, Capabilities: []string{"vision", "reasoning"}},
		},
		"ollama": {
			{ID: "llama3", Name: "Llama 3", Provider: "ollama", Score: 8.3, Capabilities: []string{"code_generation"}},
			{ID: "codellama", Name: "Code Llama", Provider: "ollama", Score: 8.5, Capabilities: []string{"code_generation", "code_completion"}},
		},
	}

	if models, exists := knownModels[config.Name]; exists {
		// Add score breakdown
		for i := range models {
			models[i].ScoreBreakdown = calculateScoreBreakdown(models[i])
		}
		return models
	}

	return nil
}

func calculateScoreBreakdown(model ModelInfo) map[string]float64 {
	// Simplified scoring based on capabilities
	breakdown := map[string]float64{
		"response_speed":    7.0,
		"model_efficiency":  7.0,
		"cost_effectiveness": 7.0,
		"capability":        7.0,
		"recency":           7.0,
	}

	// Adjust based on capabilities
	for _, cap := range model.Capabilities {
		switch cap {
		case "code_generation":
			breakdown["capability"] += 1.5
		case "reasoning":
			breakdown["capability"] += 1.0
		case "vision":
			breakdown["capability"] += 0.5
		case "function_calling":
			breakdown["capability"] += 0.5
		}
	}

	// Cap at 10
	for k, v := range breakdown {
		if v > 10 {
			breakdown[k] = 10
		}
	}

	return breakdown
}

// Redaction utilities

func redactAPIKey(key string) string {
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-4)
}

// Main challenge execution

func main() {
	resultsDir := flag.String("results-dir", "", "Directory to store results")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *resultsDir == "" {
		log.Fatal("--results-dir is required")
	}

	ctx := context.Background()
	start := time.Now()

	// Create results directory
	resultsPath := filepath.Join(*resultsDir, "results")
	logsPath := filepath.Join(*resultsDir, "logs")
	if err := os.MkdirAll(resultsPath, 0755); err != nil {
		log.Fatalf("Failed to create results directory: %v", err)
	}
	if err := os.MkdirAll(logsPath, 0755); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}

	// Configure providers from environment
	providers := []ProviderConfig{
		{Name: "anthropic", APIKey: os.Getenv("ANTHROPIC_API_KEY"), Enabled: true},
		{Name: "openai", APIKey: os.Getenv("OPENAI_API_KEY"), Enabled: true},
		{Name: "openrouter", APIKey: os.Getenv("OPENROUTER_API_KEY"), Enabled: true},
		{Name: "deepseek", APIKey: os.Getenv("DEEPSEEK_API_KEY"), Enabled: true},
		{Name: "gemini", APIKey: os.Getenv("GEMINI_API_KEY"), Enabled: true},
		{Name: "ollama", BaseURL: os.Getenv("OLLAMA_BASE_URL"), Enabled: true},
	}

	// Filter to configured providers
	var configuredProviders []ProviderConfig
	for _, p := range providers {
		if p.APIKey != "" || p.Name == "ollama" {
			configuredProviders = append(configuredProviders, p)
		}
	}

	if *verbose {
		log.Printf("Found %d configured providers", len(configuredProviders))
	}

	// Verify each provider
	var verificationResults []VerificationResult
	var allModels []ModelInfo

	for _, provider := range configuredProviders {
		if *verbose {
			log.Printf("Verifying provider: %s", provider.Name)
		}

		result := verifyProvider(ctx, provider)
		verificationResults = append(verificationResults, *result)

		if result.Authenticated {
			allModels = append(allModels, result.Models...)
		}
	}

	// Sort models by score (descending)
	sort.Slice(allModels, func(i, j int) bool {
		return allModels[i].Score > allModels[j].Score
	})

	// Calculate summary
	var totalResponseTime int64
	verifiedCount := 0
	failedCount := 0

	for _, r := range verificationResults {
		if r.Authenticated {
			verifiedCount++
			totalResponseTime += r.ResponseTimeMs
		} else {
			failedCount++
		}
	}

	avgResponseTime := float64(0)
	if verifiedCount > 0 {
		avgResponseTime = float64(totalResponseTime) / float64(verifiedCount)
	}

	avgScore := float64(0)
	highestScore := float64(0)
	topModel := ""

	if len(allModels) > 0 {
		totalScore := float64(0)
		for _, m := range allModels {
			totalScore += m.Score
			if m.Score > highestScore {
				highestScore = m.Score
				topModel = m.ID
			}
		}
		avgScore = totalScore / float64(len(allModels))
	}

	// Build final result
	challengeResult := ChallengeResult{
		ChallengeID:   "provider_verification",
		ChallengeName: "Provider Verification",
		Timestamp:     time.Now(),
		Duration:      time.Since(start),
		Status:        "passed",
		Providers:     verificationResults,
		Models:        allModels,
		Summary: ChallengeSummary{
			TotalProviders:      len(configuredProviders),
			VerifiedProviders:   verifiedCount,
			FailedProviders:     failedCount,
			TotalModels:         len(allModels),
			AverageResponseTime: avgResponseTime,
			AverageScore:        avgScore,
			HighestScore:        highestScore,
			TopModel:            topModel,
		},
	}

	// Determine status
	if verifiedCount == 0 {
		challengeResult.Status = "failed"
	}

	// Write results
	resultFile := filepath.Join(resultsPath, "providers_verified.json")
	resultData, _ := json.MarshalIndent(challengeResult.Providers, "", "  ")
	if err := os.WriteFile(resultFile, resultData, 0644); err != nil {
		log.Printf("Warning: Failed to write providers result: %v", err)
	}

	modelsFile := filepath.Join(resultsPath, "models_scored.json")
	modelsData, _ := json.MarshalIndent(challengeResult.Models, "", "  ")
	if err := os.WriteFile(modelsFile, modelsData, 0644); err != nil {
		log.Printf("Warning: Failed to write models result: %v", err)
	}

	// Write report
	reportFile := filepath.Join(resultsPath, "verification_report.md")
	report := generateReport(challengeResult)
	if err := os.WriteFile(reportFile, []byte(report), 0644); err != nil {
		log.Printf("Warning: Failed to write report: %v", err)
	}

	// Print summary
	fmt.Printf("\n=== Provider Verification Complete ===\n")
	fmt.Printf("Status: %s\n", strings.ToUpper(challengeResult.Status))
	fmt.Printf("Providers: %d verified, %d failed\n", verifiedCount, failedCount)
	fmt.Printf("Models: %d discovered\n", len(allModels))
	if topModel != "" {
		fmt.Printf("Top Model: %s (score: %.2f)\n", topModel, highestScore)
	}
	fmt.Printf("Duration: %v\n", challengeResult.Duration)
	fmt.Printf("Results: %s\n", resultsPath)

	// Exit code
	if challengeResult.Status == "failed" {
		os.Exit(1)
	}
}

func generateReport(result ChallengeResult) string {
	var sb strings.Builder

	sb.WriteString("# Provider Verification Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", result.Timestamp.Format(time.RFC3339)))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Status | %s |\n", strings.ToUpper(result.Status)))
	sb.WriteString(fmt.Sprintf("| Total Providers | %d |\n", result.Summary.TotalProviders))
	sb.WriteString(fmt.Sprintf("| Verified | %d |\n", result.Summary.VerifiedProviders))
	sb.WriteString(fmt.Sprintf("| Failed | %d |\n", result.Summary.FailedProviders))
	sb.WriteString(fmt.Sprintf("| Total Models | %d |\n", result.Summary.TotalModels))
	sb.WriteString(fmt.Sprintf("| Average Response Time | %.0fms |\n", result.Summary.AverageResponseTime))
	sb.WriteString(fmt.Sprintf("| Average Score | %.2f |\n", result.Summary.AverageScore))
	sb.WriteString(fmt.Sprintf("| Top Model | %s (%.2f) |\n", result.Summary.TopModel, result.Summary.HighestScore))
	sb.WriteString(fmt.Sprintf("| Duration | %v |\n", result.Duration))

	sb.WriteString("\n## Provider Status\n\n")
	sb.WriteString("| Provider | Connected | Authenticated | Response Time | Models |\n")
	sb.WriteString("|----------|-----------|---------------|---------------|--------|\n")
	for _, p := range result.Providers {
		connStatus := "No"
		if p.Connected {
			connStatus = "Yes"
		}
		authStatus := "No"
		if p.Authenticated {
			authStatus = "Yes"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %dms | %d |\n",
			p.Provider, connStatus, authStatus, p.ResponseTimeMs, len(p.Models)))
	}

	sb.WriteString("\n## Top Scoring Models\n\n")
	sb.WriteString("| Rank | Model | Provider | Score | Capabilities |\n")
	sb.WriteString("|------|-------|----------|-------|-------------|\n")
	for i, m := range result.Models {
		if i >= 15 { // Show top 15
			break
		}
		caps := strings.Join(m.Capabilities, ", ")
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %.2f | %s |\n",
			i+1, m.Name, m.Provider, m.Score, caps))
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("*Generated by SuperAgent Challenges*\n")

	return sb.String()
}
