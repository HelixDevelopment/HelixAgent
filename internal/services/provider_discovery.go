package services

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/llm/providers/cerebras"
	"dev.helix.agent/internal/llm/providers/claude"
	"dev.helix.agent/internal/llm/providers/deepseek"
	"dev.helix.agent/internal/llm/providers/gemini"
	"dev.helix.agent/internal/llm/providers/mistral"
	"dev.helix.agent/internal/llm/providers/ollama"
	"dev.helix.agent/internal/llm/providers/openrouter"
	"dev.helix.agent/internal/llm/providers/qwen"
	"dev.helix.agent/internal/models"
)

// LLMsVerifierScoreProvider interface for getting dynamic scores from LLMsVerifier
type LLMsVerifierScoreProvider interface {
	// GetProviderScore returns the LLMsVerifier score for a provider (0-10)
	GetProviderScore(providerType string) (float64, bool)
	// GetModelScore returns the LLMsVerifier score for a specific model
	GetModelScore(modelID string) (float64, bool)
	// RefreshScores refreshes scores from LLMsVerifier
	RefreshScores(ctx context.Context) error
}

// ProviderDiscovery handles automatic detection of LLM providers from environment variables
type ProviderDiscovery struct {
	providers          map[string]*DiscoveredProvider
	scores             map[string]*ProviderScore
	mu                 sync.RWMutex
	log                *logrus.Logger
	verifyOnStartup    bool
	verifierScores     LLMsVerifierScoreProvider // Dynamic LLMsVerifier score provider
	useDynamicScoring  bool                       // Use LLMsVerifier scores instead of hardcoded
}

// DiscoveredProvider represents a provider discovered from environment
type DiscoveredProvider struct {
	Name            string                   `json:"name"`
	Type            string                   `json:"type"`
	APIKeyEnvVar    string                   `json:"api_key_env_var"`
	APIKey          string                   `json:"-"` // Hidden in JSON
	BaseURL         string                   `json:"base_url"`
	DefaultModel    string                   `json:"default_model"`
	Provider        llm.LLMProvider          `json:"-"`
	Status          ProviderHealthStatus     `json:"status"`
	Score           float64                  `json:"score"`
	Verified        bool                     `json:"verified"`
	VerifiedAt      time.Time                `json:"verified_at,omitempty"`
	Error           string                   `json:"error,omitempty"`
	Capabilities    *models.ProviderCapabilities `json:"capabilities,omitempty"`
	SupportsModels  []string                 `json:"supported_models,omitempty"`
}

// ProviderScore represents scoring metrics for a provider
type ProviderScore struct {
	Provider        string    `json:"provider"`
	OverallScore    float64   `json:"overall_score"`
	ResponseSpeed   float64   `json:"response_speed"`
	Reliability     float64   `json:"reliability"`
	CostEfficiency  float64   `json:"cost_efficiency"`
	Capabilities    float64   `json:"capabilities"`
	Recency         float64   `json:"recency"`
	VerifiedWorking bool      `json:"verified_working"`
	ScoredAt        time.Time `json:"scored_at"`
}

// ProviderMapping maps environment variable names to provider configurations
type ProviderMapping struct {
	EnvVar       string
	ProviderType string
	ProviderName string
	BaseURL      string
	DefaultModel string
	Priority     int // Lower is higher priority
}

// providerMappings defines all known API key to provider mappings
var providerMappings = []ProviderMapping{
	// Tier 1: Premium providers (direct API access)
	{EnvVar: "ANTHROPIC_API_KEY", ProviderType: "claude", ProviderName: "claude", BaseURL: "https://api.anthropic.com/v1/messages", DefaultModel: "claude-3-5-sonnet-20241022", Priority: 1},
	{EnvVar: "CLAUDE_API_KEY", ProviderType: "claude", ProviderName: "claude", BaseURL: "https://api.anthropic.com/v1/messages", DefaultModel: "claude-3-5-sonnet-20241022", Priority: 1},
	{EnvVar: "OPENAI_API_KEY", ProviderType: "openai", ProviderName: "openai", BaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o", Priority: 1},
	{EnvVar: "GEMINI_API_KEY", ProviderType: "gemini", ProviderName: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta", DefaultModel: "gemini-pro", Priority: 2},
	{EnvVar: "GOOGLE_API_KEY", ProviderType: "gemini", ProviderName: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta", DefaultModel: "gemini-pro", Priority: 2},

	// Tier 2: High-quality specialized providers
	{EnvVar: "DEEPSEEK_API_KEY", ProviderType: "deepseek", ProviderName: "deepseek", BaseURL: "https://api.deepseek.com/v1/chat/completions", DefaultModel: "deepseek-chat", Priority: 3},
	{EnvVar: "MISTRAL_API_KEY", ProviderType: "mistral", ProviderName: "mistral", BaseURL: "https://api.mistral.ai/v1", DefaultModel: "mistral-large-latest", Priority: 3},
	{EnvVar: "CODESTRAL_API_KEY", ProviderType: "mistral", ProviderName: "codestral", BaseURL: "https://codestral.mistral.ai/v1", DefaultModel: "codestral-latest", Priority: 3},
	{EnvVar: "QWEN_API_KEY", ProviderType: "qwen", ProviderName: "qwen", BaseURL: "https://dashscope.aliyuncs.com/api/v1", DefaultModel: "qwen-turbo", Priority: 4},

	// Tier 3: Fast inference providers
	{EnvVar: "GROQ_API_KEY", ProviderType: "groq", ProviderName: "groq", BaseURL: "https://api.groq.com/openai/v1", DefaultModel: "llama-3.1-70b-versatile", Priority: 5},
	{EnvVar: "CEREBRAS_API_KEY", ProviderType: "cerebras", ProviderName: "cerebras", BaseURL: "https://api.cerebras.ai/v1", DefaultModel: "llama-3.3-70b", Priority: 5},
	{EnvVar: "SAMBANOVA_API_KEY", ProviderType: "sambanova", ProviderName: "sambanova", BaseURL: "https://api.sambanova.ai/v1", DefaultModel: "Meta-Llama-3.1-70B-Instruct", Priority: 5},

	// Tier 4: Alternative providers
	{EnvVar: "FIREWORKS_API_KEY", ProviderType: "fireworks", ProviderName: "fireworks", BaseURL: "https://api.fireworks.ai/inference/v1", DefaultModel: "accounts/fireworks/models/llama-v3p1-70b-instruct", Priority: 6},
	{EnvVar: "TOGETHERAI_API_KEY", ProviderType: "together", ProviderName: "together", BaseURL: "https://api.together.xyz/v1", DefaultModel: "meta-llama/Llama-3.2-90B-Vision-Instruct-Turbo", Priority: 6},
	{EnvVar: "TOGETHER_API_KEY", ProviderType: "together", ProviderName: "together", BaseURL: "https://api.together.xyz/v1", DefaultModel: "meta-llama/Llama-3.2-90B-Vision-Instruct-Turbo", Priority: 6},
	{EnvVar: "HYPERBOLIC_API_KEY", ProviderType: "hyperbolic", ProviderName: "hyperbolic", BaseURL: "https://api.hyperbolic.xyz/v1", DefaultModel: "meta-llama/Llama-3.3-70B-Instruct", Priority: 6},

	// Tier 5: Specialized providers
	{EnvVar: "REPLICATE_API_KEY", ProviderType: "replicate", ProviderName: "replicate", BaseURL: "https://api.replicate.com/v1", DefaultModel: "meta/llama-2-70b-chat", Priority: 7},
	{EnvVar: "SILICONFLOW_API_KEY", ProviderType: "siliconflow", ProviderName: "siliconflow", BaseURL: "https://api.siliconflow.cn/v1", DefaultModel: "Qwen/Qwen2.5-72B-Instruct", Priority: 7},
	{EnvVar: "CLOUDFLARE_API_KEY", ProviderType: "cloudflare", ProviderName: "cloudflare", BaseURL: "https://api.cloudflare.com/client/v4", DefaultModel: "@cf/meta/llama-3.1-70b-instruct", Priority: 7},
	{EnvVar: "NVIDIA_API_KEY", ProviderType: "nvidia", ProviderName: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1", DefaultModel: "meta/llama-3.1-70b-instruct", Priority: 7},

	// Tier 6: Regional/specialized providers
	{EnvVar: "KIMI_API_KEY", ProviderType: "kimi", ProviderName: "kimi", BaseURL: "https://api.moonshot.cn/v1", DefaultModel: "moonshot-v1-128k", Priority: 8},
	{EnvVar: "HUGGINGFACE_API_KEY", ProviderType: "huggingface", ProviderName: "huggingface", BaseURL: "https://api-inference.huggingface.co", DefaultModel: "meta-llama/Llama-3.2-3B-Instruct", Priority: 8},
	{EnvVar: "NOVITA_API_KEY", ProviderType: "novita", ProviderName: "novita", BaseURL: "https://api.novita.ai/v3/openai", DefaultModel: "meta-llama/llama-3.1-70b-instruct", Priority: 8},
	{EnvVar: "UPSTAGE_API_KEY", ProviderType: "upstage", ProviderName: "upstage", BaseURL: "https://api.upstage.ai/v1/solar", DefaultModel: "solar-pro", Priority: 8},
	{EnvVar: "CHUTES_API_KEY", ProviderType: "chutes", ProviderName: "chutes", BaseURL: "https://llm.chutes.ai/v1", DefaultModel: "deepseek-ai/DeepSeek-V3", Priority: 8},

	// Tier 7: Aggregators (use as fallback)
	{EnvVar: "OPENROUTER_API_KEY", ProviderType: "openrouter", ProviderName: "openrouter", BaseURL: "https://openrouter.ai/api/v1", DefaultModel: "anthropic/claude-3.5-sonnet", Priority: 10},

	// Tier 8: Self-hosted (local)
	{EnvVar: "OLLAMA_BASE_URL", ProviderType: "ollama", ProviderName: "ollama", BaseURL: "http://localhost:11434", DefaultModel: "llama3.2", Priority: 20},
}

// NewProviderDiscovery creates a new provider discovery service
func NewProviderDiscovery(log *logrus.Logger, verifyOnStartup bool) *ProviderDiscovery {
	if log == nil {
		log = logrus.New()
	}

	return &ProviderDiscovery{
		providers:         make(map[string]*DiscoveredProvider),
		scores:            make(map[string]*ProviderScore),
		log:               log,
		verifyOnStartup:   verifyOnStartup,
		useDynamicScoring: true, // Enable dynamic LLMsVerifier scoring by default
	}
}

// NewProviderDiscoveryWithVerifier creates a provider discovery with LLMsVerifier integration
func NewProviderDiscoveryWithVerifier(log *logrus.Logger, verifyOnStartup bool, verifierScores LLMsVerifierScoreProvider) *ProviderDiscovery {
	pd := NewProviderDiscovery(log, verifyOnStartup)
	pd.verifierScores = verifierScores
	pd.useDynamicScoring = verifierScores != nil
	return pd
}

// SetVerifierScoreProvider sets the LLMsVerifier score provider for dynamic scoring
func (pd *ProviderDiscovery) SetVerifierScoreProvider(provider LLMsVerifierScoreProvider) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.verifierScores = provider
	pd.useDynamicScoring = provider != nil

	if provider != nil {
		pd.log.Info("LLMsVerifier dynamic scoring enabled - provider selection will use real verification scores")
	}
}

// EnableDynamicScoring enables or disables dynamic LLMsVerifier scoring
func (pd *ProviderDiscovery) EnableDynamicScoring(enabled bool) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.useDynamicScoring = enabled && pd.verifierScores != nil
}

// DiscoverProviders scans environment variables and discovers available providers
func (pd *ProviderDiscovery) DiscoverProviders() ([]*DiscoveredProvider, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	discovered := make([]*DiscoveredProvider, 0)
	seen := make(map[string]bool) // Track already discovered provider types

	for _, mapping := range providerMappings {
		// Skip if already discovered this provider type
		if seen[mapping.ProviderName] {
			continue
		}

		apiKey := os.Getenv(mapping.EnvVar)
		if apiKey == "" {
			continue
		}

		// Skip placeholder values
		if apiKey == "xxx" || apiKey == "your-api-key" || strings.HasPrefix(apiKey, "sk-xxx") {
			continue
		}

		pd.log.WithFields(logrus.Fields{
			"provider": mapping.ProviderName,
			"type":     mapping.ProviderType,
			"env_var":  mapping.EnvVar,
		}).Info("Discovered provider from environment")

		// Create provider instance
		provider, err := pd.createProvider(mapping, apiKey)
		if err != nil {
			pd.log.WithError(err).Warnf("Failed to create provider %s", mapping.ProviderName)
			continue
		}

		dp := &DiscoveredProvider{
			Name:         mapping.ProviderName,
			Type:         mapping.ProviderType,
			APIKeyEnvVar: mapping.EnvVar,
			APIKey:       apiKey,
			BaseURL:      mapping.BaseURL,
			DefaultModel: mapping.DefaultModel,
			Provider:     provider,
			Status:       ProviderStatusUnknown,
			Score:        0,
			Verified:     false,
		}

		if provider != nil {
			dp.Capabilities = provider.GetCapabilities()
			if dp.Capabilities != nil {
				dp.SupportsModels = dp.Capabilities.SupportedModels
			}
		}

		pd.providers[mapping.ProviderName] = dp
		discovered = append(discovered, dp)
		seen[mapping.ProviderName] = true
	}

	// Discover OAuth-based providers (Claude Code and Qwen Code CLI)
	oauthProviders := pd.discoverOAuthProviders(seen)
	for _, dp := range oauthProviders {
		pd.providers[dp.Name] = dp
		discovered = append(discovered, dp)
		seen[dp.Name] = true
	}

	pd.log.Infof("Discovered %d providers from environment (including %d OAuth providers)", len(discovered), len(oauthProviders))
	return discovered, nil
}

// discoverOAuthProviders discovers providers using OAuth credentials from CLI agents
func (pd *ProviderDiscovery) discoverOAuthProviders(seen map[string]bool) []*DiscoveredProvider {
	discovered := make([]*DiscoveredProvider, 0)
	oauthReader := oauth_credentials.GetGlobalReader()

	// Discover Claude OAuth provider
	if !seen["claude"] && !seen["claude-oauth"] && oauth_credentials.IsClaudeOAuthEnabled() {
		if oauthReader.HasValidClaudeCredentials() {
			pd.log.WithFields(logrus.Fields{
				"provider": "claude-oauth",
				"type":     "claude",
				"source":   "oauth_credentials",
			}).Info("Discovered Claude provider from OAuth credentials (Claude Code CLI)")

			// Create Claude provider with OAuth token (reads token internally)
			// Use full API URL for Claude Messages endpoint
			provider, err := claude.NewClaudeProviderWithOAuth("https://api.anthropic.com/v1/messages", "claude-sonnet-4-20250514")
			if err != nil {
				pd.log.WithError(err).Warn("Failed to create Claude OAuth provider")
			} else {
				// Get token for reference (masked in logs)
				accessToken, _ := oauthReader.GetClaudeAccessToken()
				maskedToken := maskToken(accessToken)

				dp := &DiscoveredProvider{
					Name:         "claude-oauth",
					Type:         "claude",
					APIKeyEnvVar: "OAUTH:~/.claude/.credentials.json",
					APIKey:       accessToken, // Store for internal use
					BaseURL:      "https://api.anthropic.com/v1/messages",
					DefaultModel: "claude-sonnet-4-20250514",
					Provider:     provider,
					Status:       ProviderStatusUnknown,
					Score:        0,
					Verified:     false,
				}

				if provider != nil {
					dp.Capabilities = provider.GetCapabilities()
					if dp.Capabilities != nil {
						dp.SupportsModels = dp.Capabilities.SupportedModels
					}
				}

				discovered = append(discovered, dp)
				pd.log.WithField("token", maskedToken).Info("Claude OAuth provider discovered successfully")
			}
		}
	}

	// Discover Qwen OAuth provider
	if !seen["qwen"] && !seen["qwen-oauth"] && oauth_credentials.IsQwenOAuthEnabled() {
		if oauthReader.HasValidQwenCredentials() {
			pd.log.WithFields(logrus.Fields{
				"provider": "qwen-oauth",
				"type":     "qwen",
				"source":   "oauth_credentials",
			}).Info("Discovered Qwen provider from OAuth credentials (Qwen Code CLI)")

			// Create Qwen provider with OAuth token (reads token internally)
			// OAuth uses compatible-mode endpoint which differs from regular API
			provider, err := qwen.NewQwenProviderWithOAuth("https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen-turbo")
			if err != nil {
				pd.log.WithError(err).Warn("Failed to create Qwen OAuth provider")
			} else {
				// Get token for reference (masked in logs)
				accessToken, _ := oauthReader.GetQwenAccessToken()
				maskedToken := maskToken(accessToken)

				dp := &DiscoveredProvider{
					Name:         "qwen-oauth",
					Type:         "qwen",
					APIKeyEnvVar: "OAUTH:~/.qwen/oauth_creds.json",
					APIKey:       accessToken, // Store for internal use
					BaseURL:      "https://dashscope.aliyuncs.com/compatible-mode/v1",
					DefaultModel: "qwen-turbo",
					Provider:     provider,
					Status:       ProviderStatusUnknown,
					Score:        0,
					Verified:     false,
				}

				if provider != nil {
					dp.Capabilities = provider.GetCapabilities()
					if dp.Capabilities != nil {
						dp.SupportsModels = dp.Capabilities.SupportedModels
					}
				}

				discovered = append(discovered, dp)
				pd.log.WithField("token", maskedToken).Info("Qwen OAuth provider discovered successfully")
			}
		}
	}

	return discovered
}

// maskToken masks the middle part of a token for safe logging
func maskToken(token string) string {
	if len(token) <= 10 {
		return "***"
	}
	return token[:5] + "..." + token[len(token)-5:]
}

// createProvider creates an LLM provider instance based on the mapping
func (pd *ProviderDiscovery) createProvider(mapping ProviderMapping, apiKey string) (llm.LLMProvider, error) {
	switch mapping.ProviderType {
	case "claude":
		return claude.NewClaudeProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil

	case "deepseek":
		return deepseek.NewDeepSeekProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil

	case "gemini":
		return gemini.NewGeminiProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil

	case "qwen":
		return qwen.NewQwenProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil

	case "openrouter":
		return openrouter.NewSimpleOpenRouterProviderWithBaseURL(apiKey, mapping.BaseURL), nil

	case "ollama":
		return ollama.NewOllamaProvider(mapping.BaseURL, mapping.DefaultModel), nil

	case "mistral":
		// Use native Mistral provider for direct API access
		baseURL := mapping.BaseURL
		if baseURL == "" || !strings.Contains(baseURL, "/chat/completions") {
			baseURL = "https://api.mistral.ai/v1/chat/completions"
		}
		return mistral.NewMistralProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "cerebras":
		// Use native Cerebras provider for direct API access
		baseURL := mapping.BaseURL
		if baseURL == "" || !strings.Contains(baseURL, "/chat/completions") {
			baseURL = "https://api.cerebras.ai/v1/chat/completions"
		}
		return cerebras.NewCerebrasProvider(apiKey, baseURL, mapping.DefaultModel), nil

	// For providers without native implementations, use OpenRouter as a proxy
	case "groq", "fireworks", "together", "hyperbolic",
		 "sambanova", "replicate", "siliconflow", "cloudflare", "nvidia",
		 "kimi", "huggingface", "novita", "upstage", "chutes", "openai":
		// Create OpenRouter-compatible provider
		return pd.createOpenAICompatibleProvider(mapping, apiKey)

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", mapping.ProviderType)
	}
}

// createOpenAICompatibleProvider creates a provider using OpenAI-compatible API
func (pd *ProviderDiscovery) createOpenAICompatibleProvider(mapping ProviderMapping, apiKey string) (llm.LLMProvider, error) {
	// Use OpenRouter provider with custom base URL for OpenAI-compatible APIs
	return openrouter.NewSimpleOpenRouterProviderWithBaseURL(apiKey, mapping.BaseURL), nil
}

// VerifyAllProviders verifies all discovered providers and updates their status
func (pd *ProviderDiscovery) VerifyAllProviders(ctx context.Context) map[string]*DiscoveredProvider {
	pd.mu.Lock()
	providers := make([]*DiscoveredProvider, 0, len(pd.providers))
	for _, p := range pd.providers {
		providers = append(providers, p)
	}
	pd.mu.Unlock()

	// Verify providers concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan *DiscoveredProvider, len(providers))

	for _, p := range providers {
		wg.Add(1)
		go func(provider *DiscoveredProvider) {
			defer wg.Done()
			pd.verifyProvider(ctx, provider)
			resultsChan <- provider
		}(p)
	}

	// Wait for all to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	results := make(map[string]*DiscoveredProvider)
	for p := range resultsChan {
		results[p.Name] = p
	}

	return results
}

// verifyProvider verifies a single provider with an actual API call
func (pd *ProviderDiscovery) verifyProvider(ctx context.Context, provider *DiscoveredProvider) {
	if provider.Provider == nil {
		provider.Status = ProviderStatusUnhealthy
		provider.Error = "provider not initialized"
		return
	}

	start := time.Now()

	// Create a test request
	testReq := &models.LLMRequest{
		ID:        fmt.Sprintf("verify_%s_%d", provider.Name, time.Now().UnixNano()),
		SessionID: "verification",
		Prompt:    "Say OK",
		Messages: []models.Message{
			{Role: "user", Content: "Say OK"},
		},
		ModelParams: models.ModelParameters{
			Model:       provider.DefaultModel,
			MaxTokens:   5,
			Temperature: 0.1,
		},
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	// Create context with timeout
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Make the actual API call
	resp, err := provider.Provider.Complete(verifyCtx, testReq)
	responseTime := time.Since(start)

	provider.VerifiedAt = time.Now()

	if err != nil {
		errStr := strings.ToLower(err.Error())

		// Categorize the error
		switch {
		case containsAny(errStr, "429", "quota", "rate", "resource_exhausted", "too many"):
			provider.Status = ProviderStatusRateLimited
			provider.Error = "rate limited or quota exceeded"
		case containsAny(errStr, "401", "403", "unauthorized", "invalid", "authentication", "api_key", "forbidden"):
			provider.Status = ProviderStatusAuthFailed
			provider.Error = "authentication failed or invalid API key"
		default:
			provider.Status = ProviderStatusUnhealthy
			provider.Error = err.Error()
		}
		provider.Verified = false

		pd.log.WithFields(logrus.Fields{
			"provider": provider.Name,
			"status":   provider.Status,
			"error":    provider.Error,
			"duration": responseTime,
		}).Warn("Provider verification failed")
		return
	}

	// Check for valid response
	if resp != nil && resp.Content != "" {
		provider.Status = ProviderStatusHealthy
		provider.Verified = true
		provider.Error = ""

		// Calculate initial score based on response time
		provider.Score = pd.calculateProviderScore(provider, responseTime)

		pd.log.WithFields(logrus.Fields{
			"provider": provider.Name,
			"status":   provider.Status,
			"score":    provider.Score,
			"duration": responseTime,
		}).Info("Provider verified successfully")
	} else {
		provider.Status = ProviderStatusUnhealthy
		provider.Error = "empty response from provider"
		provider.Verified = false
	}
}

// calculateProviderScore calculates a quality score for a provider
// DYNAMIC: Uses LLMsVerifier scores when available, falls back to hardcoded scores
func (pd *ProviderDiscovery) calculateProviderScore(provider *DiscoveredProvider, responseTime time.Duration) float64 {
	// DYNAMIC SCORING: Try to get score from LLMsVerifier first
	baseScore := pd.getDynamicBaseScore(provider)

	// Response time bonus (faster is better, up to 2 points)
	responseBonus := 0.0
	if responseTime < 1*time.Second {
		responseBonus = 2.0
	} else if responseTime < 2*time.Second {
		responseBonus = 1.5
	} else if responseTime < 5*time.Second {
		responseBonus = 1.0
	} else if responseTime < 10*time.Second {
		responseBonus = 0.5
	}

	// Capability bonus
	capBonus := 0.0
	if provider.Capabilities != nil {
		if provider.Capabilities.SupportsStreaming {
			capBonus += 0.3
		}
		if provider.Capabilities.SupportsFunctionCalling {
			capBonus += 0.5
		}
		if provider.Capabilities.SupportsVision {
			capBonus += 0.3
		}
		if provider.Capabilities.SupportsCodeCompletion {
			capBonus += 0.4
		}
	}

	totalScore := baseScore + responseBonus + capBonus
	if totalScore > 10.0 {
		totalScore = 10.0
	}

	// Determine score source for logging
	scoreSource := "static"
	if pd.useDynamicScoring && pd.verifierScores != nil {
		if _, found := pd.verifierScores.GetProviderScore(provider.Type); found {
			scoreSource = "llmsverifier"
		}
	}

	// Store detailed score
	pd.mu.Lock()
	pd.scores[provider.Name] = &ProviderScore{
		Provider:        provider.Name,
		OverallScore:    totalScore,
		ResponseSpeed:   responseBonus,
		Capabilities:    capBonus,
		Reliability:     baseScore * 0.5,
		VerifiedWorking: provider.Verified,
		ScoredAt:        time.Now(),
	}
	pd.mu.Unlock()

	pd.log.WithFields(logrus.Fields{
		"provider":     provider.Name,
		"base_score":   baseScore,
		"total_score":  totalScore,
		"score_source": scoreSource,
	}).Debug("Provider score calculated")

	return totalScore
}

// getDynamicBaseScore gets base score from LLMsVerifier if available, otherwise uses hardcoded fallback
func (pd *ProviderDiscovery) getDynamicBaseScore(provider *DiscoveredProvider) float64 {
	// Try LLMsVerifier dynamic scores first
	if pd.useDynamicScoring && pd.verifierScores != nil {
		// Try to get score by model first (more specific)
		if score, found := pd.verifierScores.GetModelScore(provider.DefaultModel); found {
			pd.log.WithFields(logrus.Fields{
				"provider": provider.Name,
				"model":    provider.DefaultModel,
				"score":    score,
			}).Debug("Using LLMsVerifier model score")
			return score
		}

		// Fall back to provider-level score
		if score, found := pd.verifierScores.GetProviderScore(provider.Type); found {
			pd.log.WithFields(logrus.Fields{
				"provider": provider.Name,
				"type":     provider.Type,
				"score":    score,
			}).Debug("Using LLMsVerifier provider score")
			return score
		}
	}

	// Fall back to static hardcoded scores
	return getBaseScoreForProvider(provider.Type)
}

// getBaseScoreForProvider returns a base quality score for a provider type
// DYNAMIC SCORING: This function is a FALLBACK ONLY when LLMsVerifier scores are unavailable.
// Scores should come from real LLMsVerifier verification results, not hardcoded values.
// The default score of 5.0 ensures all providers start equal and are differentiated by actual performance.
func getBaseScoreForProvider(providerType string) float64 {
	// CRITICAL: NO hardcoded provider scores - all providers get the same baseline
	// The actual differentiation comes from:
	// 1. LLMsVerifier real verification scores (primary source)
	// 2. Response time bonuses during verification
	// 3. Capability bonuses (streaming, function calling, etc.)
	//
	// This ensures the system is truly dynamic and doesn't favor any provider
	// based on arbitrary hardcoded values.
	return 5.0 // Neutral baseline - let real metrics differentiate providers
}

// GetBestProviders returns the top N verified providers sorted by score
func (pd *ProviderDiscovery) GetBestProviders(n int) []*DiscoveredProvider {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	// Collect verified providers
	verified := make([]*DiscoveredProvider, 0)
	for _, p := range pd.providers {
		if p.Verified && p.Status == ProviderStatusHealthy {
			verified = append(verified, p)
		}
	}

	// Sort by score (descending)
	sort.Slice(verified, func(i, j int) bool {
		return verified[i].Score > verified[j].Score
	})

	// Return top N
	if n <= 0 || n > len(verified) {
		return verified
	}
	return verified[:n]
}

// GetAllProviders returns all discovered providers
func (pd *ProviderDiscovery) GetAllProviders() []*DiscoveredProvider {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	result := make([]*DiscoveredProvider, 0, len(pd.providers))
	for _, p := range pd.providers {
		result = append(result, p)
	}

	// Sort by score
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}

// GetProviderByName returns a specific discovered provider
func (pd *ProviderDiscovery) GetProviderByName(name string) *DiscoveredProvider {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return pd.providers[name]
}

// GetProviderScore returns the score for a provider
func (pd *ProviderDiscovery) GetProviderScore(name string) *ProviderScore {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return pd.scores[name]
}

// GetDebateGroupProviders returns providers suitable for the debate AI group
// It selects the best verified providers up to the specified count
func (pd *ProviderDiscovery) GetDebateGroupProviders(minProviders, maxProviders int) []*DiscoveredProvider {
	best := pd.GetBestProviders(maxProviders)

	if len(best) < minProviders {
		pd.log.Warnf("Only %d verified providers available, need at least %d for optimal debate group", len(best), minProviders)
	}

	return best
}

// Summary returns a summary of discovered and verified providers
func (pd *ProviderDiscovery) Summary() map[string]interface{} {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	var healthy, rateLimited, authFailed, unhealthy, unknown int
	var totalScore float64
	providerList := make([]map[string]interface{}, 0)

	for _, p := range pd.providers {
		switch p.Status {
		case ProviderStatusHealthy:
			healthy++
			totalScore += p.Score
		case ProviderStatusRateLimited:
			rateLimited++
		case ProviderStatusAuthFailed:
			authFailed++
		case ProviderStatusUnhealthy:
			unhealthy++
		default:
			unknown++
		}

		providerList = append(providerList, map[string]interface{}{
			"name":     p.Name,
			"type":     p.Type,
			"status":   p.Status,
			"score":    p.Score,
			"verified": p.Verified,
			"error":    p.Error,
		})
	}

	// Sort by score
	sort.Slice(providerList, func(i, j int) bool {
		return providerList[i]["score"].(float64) > providerList[j]["score"].(float64)
	})

	avgScore := 0.0
	if healthy > 0 {
		avgScore = totalScore / float64(healthy)
	}

	return map[string]interface{}{
		"total_discovered": len(pd.providers),
		"healthy":          healthy,
		"rate_limited":     rateLimited,
		"auth_failed":      authFailed,
		"unhealthy":        unhealthy,
		"unknown":          unknown,
		"average_score":    avgScore,
		"debate_ready":     healthy >= 2,
		"providers":        providerList,
	}
}

// RegisterToRegistry registers all verified providers to a ProviderRegistry
func (pd *ProviderDiscovery) RegisterToRegistry(registry *ProviderRegistry) error {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	registered := 0
	for _, p := range pd.providers {
		if p.Verified && p.Status == ProviderStatusHealthy && p.Provider != nil {
			if err := registry.RegisterProvider(p.Name, p.Provider); err != nil {
				pd.log.WithError(err).Warnf("Failed to register provider %s", p.Name)
				continue
			}
			registered++
			pd.log.WithFields(logrus.Fields{
				"provider": p.Name,
				"score":    p.Score,
			}).Info("Registered verified provider to registry")
		}
	}

	pd.log.Infof("Registered %d verified providers to registry", registered)
	return nil
}
