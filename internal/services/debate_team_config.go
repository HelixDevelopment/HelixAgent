package services

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/llm/providers/cerebras"
	"dev.helix.agent/internal/llm/providers/mistral"
	"dev.helix.agent/internal/verifier"
)

// TotalDebatePositions is the total number of positions in the AI debate team
const TotalDebatePositions = 5

// FallbacksPerPosition is the number of fallbacks per position
const FallbacksPerPosition = 2

// TotalDebateLLMs is the total number of LLMs used (positions * (1 primary + fallbacks))
const TotalDebateLLMs = TotalDebatePositions * (1 + FallbacksPerPosition) // 15 LLMs

// DebateTeamPosition represents a position in the AI debate team
type DebateTeamPosition int

const (
	PositionAnalyst   DebateTeamPosition = 1 // Primary analyst
	PositionProposer  DebateTeamPosition = 2 // Primary proposer
	PositionCritic    DebateTeamPosition = 3 // Primary critic
	PositionSynthesis DebateTeamPosition = 4 // Synthesis expert
	PositionMediator  DebateTeamPosition = 5 // Mediator/consensus
)

// DebateRole represents the role a team member plays
type DebateRole string

const (
	RoleAnalyst   DebateRole = "analyst"
	RoleProposer  DebateRole = "proposer"
	RoleCritic    DebateRole = "critic"
	RoleSynthesis DebateRole = "synthesis"
	RoleMediator  DebateRole = "mediator"
)

// ClaudeModels defines the available Claude models (OAuth2 provider)
// Updated 2025-01-13 with latest Claude 4.x and 4.5 models
var ClaudeModels = struct {
	// Claude 4.5 (Latest generation - November 2025)
	Opus45   string // claude-opus-4-5-20251101 - Most capable
	Sonnet45 string // claude-sonnet-4-5-20250929 - Balanced
	Haiku45  string // claude-haiku-4-5-20251001 - Fast, efficient

	// Claude 4.x (May 2025)
	Opus4   string // claude-opus-4-20250514 - Previous flagship
	Sonnet4 string // claude-sonnet-4-20250514 - Previous generation

	// Claude 3.5 (Legacy - can be used as fallbacks)
	Sonnet35 string // claude-3-5-sonnet-20241022
	Haiku35  string // claude-3-5-haiku-20241022

	// Claude 3 (Legacy - can be used as fallbacks)
	Opus3   string // claude-3-opus-20240229
	Sonnet3 string // claude-3-sonnet-20240229
	Haiku3  string // claude-3-haiku-20240307
}{
	// Claude 4.5 (Primary models for AI Debate Team)
	Opus45:   "claude-opus-4-5-20251101",
	Sonnet45: "claude-sonnet-4-5-20250929",
	Haiku45:  "claude-haiku-4-5-20251001",

	// Claude 4.x
	Opus4:   "claude-opus-4-20250514",
	Sonnet4: "claude-sonnet-4-20250514",

	// Claude 3.5 (Fallbacks)
	Sonnet35: "claude-3-5-sonnet-20241022",
	Haiku35:  "claude-3-5-haiku-20241022",

	// Claude 3 (Legacy fallbacks)
	Opus3:   "claude-3-opus-20240229",
	Sonnet3: "claude-3-sonnet-20240229",
	Haiku3:  "claude-3-haiku-20240307",
}

// QwenModels defines the available Qwen models (OAuth2 provider)
var QwenModels = struct {
	Turbo string // Fast, efficient model
	Plus  string // Balanced model
	Max   string // Most capable model
	Coder string // Code-focused model
	Long  string // Long context model
}{
	Turbo: "qwen-turbo",
	Plus:  "qwen-plus",
	Max:   "qwen-max",
	Coder: "qwen-coder-turbo",
	Long:  "qwen-long",
}

// LLMsVerifierModels defines LLMsVerifier-scored models
var LLMsVerifierModels = struct {
	DeepSeek string // High code quality
	Gemini   string // Strong synthesis
	Mistral  string // Good mediator
	Groq     string // Fast inference
	Cerebras string // Fast inference
}{
	DeepSeek: "deepseek-chat",
	Gemini:   "gemini-2.0-flash",
	Mistral:  "mistral-large-latest",
	Groq:     "llama-3.1-70b-versatile",
	Cerebras: "llama-3.3-70b",
}

// OpenRouterFreeModels defines OpenRouter Zen free models (with :free suffix)
// These are high-quality models available for free through OpenRouter
var OpenRouterFreeModels = struct {
	// Llama 4 models (Meta)
	Llama4Maverick string
	Llama4Scout    string
	Llama3370B     string

	// DeepSeek models (reasoning)
	DeepSeekR1      string
	DeepSeekR1Zero  string
	DeepSeekChatV3  string
	DeepSeekR1Llama string
	DeepSeekR1Qwen32 string
	DeepSeekR1Qwen14 string

	// Qwen models
	QwenQwQ32B  string
	QwenVL3B    string

	// Google models
	Gemini25ProExp   string
	Gemini20Flash    string
	Gemini20FlashExp string
	Gemma327B        string

	// Other models
	NvidiaLlama253B string
	Phi3Mini        string
	Phi3Medium      string
	Mistral7B       string
	Zephyr7B        string
	OpenChat7B      string
	Capybara7B      string
}{
	// Llama 4 (highest capability free models)
	Llama4Maverick: "meta-llama/llama-4-maverick:free",
	Llama4Scout:    "meta-llama/llama-4-scout:free",
	Llama3370B:     "meta-llama/llama-3.3-70b-instruct:free",

	// DeepSeek (excellent reasoning)
	DeepSeekR1:       "deepseek/deepseek-r1:free",
	DeepSeekR1Zero:   "deepseek/deepseek-r1-zero:free",
	DeepSeekChatV3:   "deepseek/deepseek-chat-v3-0324:free",
	DeepSeekR1Llama:  "deepseek/deepseek-r1-distill-llama-70b:free",
	DeepSeekR1Qwen32: "deepseek/deepseek-r1-distill-qwen-32b:free",
	DeepSeekR1Qwen14: "deepseek/deepseek-r1-distill-qwen-14b:free",

	// Qwen
	QwenQwQ32B: "qwen/qwq-32b:free",
	QwenVL3B:   "qwen/qwen2.5-vl-3b-instruct:free",

	// Google
	Gemini25ProExp:   "google/gemini-2.5-pro-exp-03-25:free",
	Gemini20Flash:    "google/gemini-2.0-flash-thinking-exp:free",
	Gemini20FlashExp: "google/gemini-2.0-flash-exp:free",
	Gemma327B:        "google/gemma-3-27b-it:free",

	// Other
	NvidiaLlama253B: "nvidia/llama-3.1-nemotron-ultra-253b-v1:free",
	Phi3Mini:        "microsoft/phi-3-mini-128k-instruct:free",
	Phi3Medium:      "microsoft/phi-3-medium-128k-instruct:free",
	Mistral7B:       "mistralai/mistral-7b-instruct:free",
	Zephyr7B:        "huggingfaceh4/zephyr-7b-beta:free",
	OpenChat7B:      "openchat/openchat-7b:free",
	Capybara7B:      "nousresearch/nous-capybara-7b:free",
}

// ZenModels defines OpenCode Zen free models (no API key required)
// These are high-quality models available through OpenCode's Zen API gateway
// NOTE: Zen API requires model names WITHOUT "opencode/" prefix
var ZenModels = struct {
	BigPickle     string // Stealth model
	GrokCodeFast  string // xAI Grok code model (default)
	GLM47Free     string // GLM 4.7 free tier
	GPT5Nano      string // GPT 5 Nano free tier
}{
	BigPickle:    "opencode/big-pickle",
	GrokCodeFast: "opencode/grok-code",
	GLM47Free:    "opencode/glm-4.7-free",
	GPT5Nano:     "opencode/gpt-5-nano",
}

// VerifiedLLM represents a verified LLM from LLMsVerifier
type VerifiedLLM struct {
	ProviderName string
	ModelName    string
	Score        float64
	Provider     llm.LLMProvider
	IsOAuth      bool // True if from OAuth2 provider (Claude/Qwen)
	Verified     bool
}

// DebateTeamMember represents a member of the AI debate team
type DebateTeamMember struct {
	Position     DebateTeamPosition `json:"position"`
	Role         DebateRole         `json:"role"`
	ProviderName string             `json:"provider_name"`
	ModelName    string             `json:"model_name"`
	Provider     llm.LLMProvider    `json:"-"`
	Fallback     *DebateTeamMember  `json:"fallback,omitempty"`
	Score        float64            `json:"score"`
	IsActive     bool               `json:"is_active"`
	IsOAuth      bool               `json:"is_oauth"`
}

// DebateTeamConfig manages the AI debate team configuration
type DebateTeamConfig struct {
	mu               sync.RWMutex
	members          map[DebateTeamPosition]*DebateTeamMember
	verifiedLLMs     []*VerifiedLLM // All verified LLMs sorted by score
	providerRegistry *ProviderRegistry
	discovery        *ProviderDiscovery
	startupVerifier  *verifier.StartupVerifier // Unified startup verification (optional)
	logger           *logrus.Logger
}

// NewDebateTeamConfig creates a new debate team configuration
func NewDebateTeamConfig(
	providerRegistry *ProviderRegistry,
	discovery *ProviderDiscovery,
	logger *logrus.Logger,
) *DebateTeamConfig {
	if logger == nil {
		logger = logrus.New()
	}
	config := &DebateTeamConfig{
		members:          make(map[DebateTeamPosition]*DebateTeamMember),
		verifiedLLMs:     make([]*VerifiedLLM, 0),
		providerRegistry: providerRegistry,
		discovery:        discovery,
		logger:           logger,
	}
	return config
}

// SetStartupVerifier sets the unified startup verifier
// When set, InitializeTeam will use the StartupVerifier's verified providers
// instead of performing manual verification
func (dtc *DebateTeamConfig) SetStartupVerifier(sv *verifier.StartupVerifier) {
	dtc.mu.Lock()
	defer dtc.mu.Unlock()
	dtc.startupVerifier = sv
}

// NewDebateTeamConfigWithStartupVerifier creates a new debate team config with StartupVerifier
func NewDebateTeamConfigWithStartupVerifier(
	sv *verifier.StartupVerifier,
	logger *logrus.Logger,
) *DebateTeamConfig {
	if logger == nil {
		logger = logrus.New()
	}
	config := &DebateTeamConfig{
		members:         make(map[DebateTeamPosition]*DebateTeamMember),
		verifiedLLMs:    make([]*VerifiedLLM, 0),
		startupVerifier: sv,
		logger:          logger,
	}
	return config
}

// InitializeTeam sets up the debate team using:
// 1. OAuth2 providers (Claude, Qwen) if available and verified by LLMsVerifier
// 2. LLMsVerifier-scored providers for remaining positions (best scores used)
// 3. Same LLM can be used in multiple instances if needed
// Total: 15 LLMs (5 positions × 3 LLMs each: 1 primary + 2 fallbacks)
func (dtc *DebateTeamConfig) InitializeTeam(ctx context.Context) error {
	dtc.mu.Lock()
	defer dtc.mu.Unlock()

	dtc.logger.Info("Initializing AI Debate Team (15 LLMs total)...")
	dtc.logger.Info("Strategy: OAuth2 providers (if verified) + LLMsVerifier best-scored providers")

	// Step 1: Verify all providers and collect verified LLMs
	dtc.collectVerifiedLLMs(ctx)

	// Step 2: Sort verified LLMs by score (highest first)
	sort.Slice(dtc.verifiedLLMs, func(i, j int) bool {
		// Prioritize OAuth providers, then by score
		if dtc.verifiedLLMs[i].IsOAuth != dtc.verifiedLLMs[j].IsOAuth {
			return dtc.verifiedLLMs[i].IsOAuth
		}
		return dtc.verifiedLLMs[i].Score > dtc.verifiedLLMs[j].Score
	})

	dtc.logger.WithField("verified_count", len(dtc.verifiedLLMs)).Info("Collected verified LLMs")

	// Step 3: Assign primary positions (5 positions)
	dtc.assignPrimaryPositions()

	// Step 4: Assign fallbacks (2 per position = 10 more slots)
	dtc.assignAllFallbacks()

	// Step 5: Log final team composition
	dtc.logTeamComposition()

	dtc.logger.WithFields(logrus.Fields{
		"total_positions": TotalDebatePositions,
		"total_llms":      TotalDebateLLMs,
		"assigned":        len(dtc.members),
	}).Info("AI Debate Team initialized")

	return nil
}

// collectVerifiedLLMs gathers all verified LLMs from OAuth2, OpenRouter free models, and LLMsVerifier
// If StartupVerifier is configured, it uses the unified verification pipeline instead
func (dtc *DebateTeamConfig) collectVerifiedLLMs(ctx context.Context) {
	dtc.verifiedLLMs = make([]*VerifiedLLM, 0)

	// Use StartupVerifier if available (unified pipeline)
	if dtc.startupVerifier != nil {
		dtc.collectFromStartupVerifier()
		return
	}

	// Legacy path: Use discovery and manual collection
	// Verify providers if discovery is available
	if dtc.discovery != nil {
		dtc.discovery.VerifyAllProviders(ctx)
	}

	// Collect OAuth2 Claude models (trust CLI credentials even if API verification fails)
	dtc.collectClaudeModels()

	// Collect OAuth2 Qwen models (trust CLI credentials even if API verification fails)
	dtc.collectQwenModels()

	// Collect reliable API-key providers (Cerebras, Mistral) - MUST be before free models
	// These are proven working providers that should be prioritized as fallbacks
	dtc.collectReliableAPIProviders()

	// Collect OpenRouter Zen free models (:free suffix)
	dtc.collectOpenRouterFreeModels()

	// Collect OpenCode Zen free models (Big Pickle, Grok Code Fast, GLM 4.7, GPT 5 Nano)
	dtc.collectZenModels()

	// Collect LLMsVerifier-scored providers
	dtc.collectLLMsVerifierProviders()

	dtc.logger.WithFields(logrus.Fields{
		"total_verified":   len(dtc.verifiedLLMs),
		"oauth_count":      dtc.countOAuthLLMs(),
		"free_models_count": dtc.countFreeModels(),
	}).Info("Verified LLMs collected")
}

// collectFromStartupVerifier collects verified LLMs from the unified StartupVerifier
func (dtc *DebateTeamConfig) collectFromStartupVerifier() {
	rankedProviders := dtc.startupVerifier.GetRankedProviders()

	for _, provider := range rankedProviders {
		if !provider.Verified {
			continue
		}

		// Add each model from the provider
		for _, model := range provider.Models {
			if !model.Verified {
				continue
			}

			isOAuth := provider.AuthType == verifier.AuthTypeOAuth

			dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
				ProviderName: provider.Name,
				ModelName:    model.ID,
				Score:        model.Score,
				Provider:     provider.Instance,
				IsOAuth:      isOAuth,
				Verified:     true,
			})
		}

		// If provider has no models but is verified, add with default model
		if len(provider.Models) == 0 && provider.DefaultModel != "" {
			isOAuth := provider.AuthType == verifier.AuthTypeOAuth

			dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
				ProviderName: provider.Name,
				ModelName:    provider.DefaultModel,
				Score:        provider.Score,
				Provider:     provider.Instance,
				IsOAuth:      isOAuth,
				Verified:     true,
			})
		}
	}

	dtc.logger.WithFields(logrus.Fields{
		"total_verified":     len(dtc.verifiedLLMs),
		"oauth_count":        dtc.countOAuthLLMs(),
		"free_models_count":  dtc.countFreeModels(),
		"source":             "startup_verifier",
		"providers_verified": len(rankedProviders),
	}).Info("Verified LLMs collected from StartupVerifier")
}

// collectClaudeModels collects Claude models if the provider is available
// For OAuth providers, we trust CLI credentials even if API verification fails
// because CLI tokens may be scoped differently than public API access
func (dtc *DebateTeamConfig) collectClaudeModels() {
	// First try verified provider, then fall back to any registered provider
	provider := dtc.getVerifiedProvider("claude", "claude-oauth")
	if provider == nil {
		provider = dtc.getRegisteredProvider("claude", "claude-oauth")
	}
	if provider == nil {
		dtc.logger.Debug("Claude provider not available")
		return
	}

	// Add all Claude models (prioritized by generation and capability)
	// Claude 4.5 models get highest scores, then 4.x, then 3.5, then 3.x
	claudeModels := []struct {
		Name  string
		Score float64
	}{
		// Claude 4.5 (Primary - highest scores)
		{ClaudeModels.Opus45, 9.8},   // Most capable Claude model
		{ClaudeModels.Sonnet45, 9.6}, // High quality balanced
		{ClaudeModels.Haiku45, 9.0},  // Fast and efficient

		// Claude 4.x (Secondary)
		{ClaudeModels.Opus4, 9.4},
		{ClaudeModels.Sonnet4, 9.2},

		// Claude 3.5 (Fallbacks)
		{ClaudeModels.Sonnet35, 8.8},
		{ClaudeModels.Haiku35, 8.4},

		// Claude 3 (Legacy fallbacks)
		{ClaudeModels.Opus3, 8.0},
		{ClaudeModels.Sonnet3, 7.5},
		{ClaudeModels.Haiku3, 7.0},
	}

	for _, m := range claudeModels {
		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "claude",
			ModelName:    m.Name,
			Score:        m.Score,
			Provider:     provider,
			IsOAuth:      true,
			Verified:     true,
		})
	}

	dtc.logger.WithField("models", len(claudeModels)).Info("Added Claude OAuth2 models")
}

// collectQwenModels collects Qwen models if the provider is available
// For OAuth providers, we trust CLI credentials even if API verification fails
// because CLI tokens may be scoped differently than public API access
func (dtc *DebateTeamConfig) collectQwenModels() {
	// First try verified provider, then fall back to any registered provider
	provider := dtc.getVerifiedProvider("qwen", "qwen-oauth")
	if provider == nil {
		provider = dtc.getRegisteredProvider("qwen", "qwen-oauth")
	}
	if provider == nil {
		dtc.logger.Debug("Qwen provider not available")
		return
	}

	// Add all Qwen models
	qwenModels := []struct {
		Name  string
		Score float64
	}{
		{QwenModels.Max, 8.0},
		{QwenModels.Plus, 7.8},
		{QwenModels.Turbo, 7.5},
		{QwenModels.Coder, 7.5},
		{QwenModels.Long, 7.5},
	}

	for _, m := range qwenModels {
		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "qwen",
			ModelName:    m.Name,
			Score:        m.Score,
			Provider:     provider,
			IsOAuth:      true,
			Verified:     true,
		})
	}

	dtc.logger.WithField("models", len(qwenModels)).Info("Added Qwen OAuth2 models")
}

// collectReliableAPIProviders collects Cerebras and Mistral providers that have proven API access
// These providers are critical for fallback when OAuth providers fail
// IMPORTANT: These MUST be included as fallbacks for OAuth primaries
func (dtc *DebateTeamConfig) collectReliableAPIProviders() {
	// Cerebras - ultra-fast inference, proven working
	if apiKey := os.Getenv("CEREBRAS_API_KEY"); apiKey != "" {
		provider := cerebras.NewCerebrasProvider(apiKey, "https://api.cerebras.ai/v1/chat/completions", LLMsVerifierModels.Cerebras)
		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "cerebras",
			ModelName:    LLMsVerifierModels.Cerebras,
			Score:        8.9, // High score - fast and reliable
			Provider:     provider,
			IsOAuth:      false,
			Verified:     true,
		})
		dtc.logger.Info("Added Cerebras as reliable API fallback provider")
	} else {
		dtc.logger.Warn("CEREBRAS_API_KEY not set - Cerebras will not be available as fallback")
	}

	// Mistral - high quality, proven working
	if apiKey := os.Getenv("MISTRAL_API_KEY"); apiKey != "" {
		provider := mistral.NewMistralProvider(apiKey, "https://api.mistral.ai/v1/chat/completions", LLMsVerifierModels.Mistral)
		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "mistral",
			ModelName:    LLMsVerifierModels.Mistral,
			Score:        8.7, // High score - quality and reliable
			Provider:     provider,
			IsOAuth:      false,
			Verified:     true,
		})
		dtc.logger.Info("Added Mistral as reliable API fallback provider")
	} else {
		dtc.logger.Warn("MISTRAL_API_KEY not set - Mistral will not be available as fallback")
	}

	// DeepSeek - excellent for code, proven working
	if apiKey := os.Getenv("DEEPSEEK_API_KEY"); apiKey != "" {
		// DeepSeek uses OpenAI-compatible format, get provider from registry or create
		provider := dtc.getRegisteredProvider("deepseek")
		if provider != nil {
			dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
				ProviderName: "deepseek",
				ModelName:    LLMsVerifierModels.DeepSeek,
				Score:        8.8, // High score - excellent for code
				Provider:     provider,
				IsOAuth:      false,
				Verified:     true,
			})
			dtc.logger.Info("Added DeepSeek as reliable API fallback provider")
		}
	}

	// Gemini - Google's model, proven working
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		provider := dtc.getRegisteredProvider("gemini")
		if provider != nil {
			dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
				ProviderName: "gemini",
				ModelName:    LLMsVerifierModels.Gemini,
				Score:        8.6, // High score - good all-rounder
				Provider:     provider,
				IsOAuth:      false,
				Verified:     true,
			})
			dtc.logger.Info("Added Gemini as reliable API fallback provider")
		}
	}
}

// collectLLMsVerifierProviders collects providers verified by LLMsVerifier
func (dtc *DebateTeamConfig) collectLLMsVerifierProviders() {
	if dtc.discovery == nil {
		return
	}

	// Get best providers from discovery (already verified and scored)
	bestProviders := dtc.discovery.GetBestProviders(20)

	for _, p := range bestProviders {
		// Skip if already added as OAuth provider
		if p.Type == "claude" || p.Type == "qwen" {
			continue
		}

		if p.Verified && p.Provider != nil {
			dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
				ProviderName: p.Name,
				ModelName:    p.DefaultModel,
				Score:        p.Score,
				Provider:     p.Provider,
				IsOAuth:      false,
				Verified:     true,
			})
		}
	}

	dtc.logger.WithField("llmsverifier_count", len(bestProviders)).Debug("Collected LLMsVerifier providers")
}

// getVerifiedProvider tries to get a verified provider by name(s)
func (dtc *DebateTeamConfig) getVerifiedProvider(names ...string) llm.LLMProvider {
	for _, name := range names {
		// Try registry first
		if dtc.providerRegistry != nil {
			if p, err := dtc.providerRegistry.GetProvider(name); err == nil && p != nil {
				return p
			}
		}
		// Try discovery
		if dtc.discovery != nil {
			if discovered := dtc.discovery.GetProviderByName(name); discovered != nil {
				if discovered.Verified && discovered.Provider != nil {
					return discovered.Provider
				}
			}
		}
	}
	return nil
}

// getRegisteredProvider gets any registered provider by name(s), even if not verified
// This is used for OAuth providers where CLI credentials are trusted even if API verification fails
func (dtc *DebateTeamConfig) getRegisteredProvider(names ...string) llm.LLMProvider {
	for _, name := range names {
		// Try registry
		if dtc.providerRegistry != nil {
			if p, err := dtc.providerRegistry.GetProvider(name); err == nil && p != nil {
				return p
			}
		}
		// Try discovery (without verification check)
		if dtc.discovery != nil {
			if discovered := dtc.discovery.GetProviderByName(name); discovered != nil && discovered.Provider != nil {
				return discovered.Provider
			}
		}
	}
	return nil
}

// collectOpenRouterFreeModels collects OpenRouter Zen free models (:free suffix)
// These are high-quality models available for free through OpenRouter
// IMPORTANT: Only add models if OpenRouter provider is healthy and verified
func (dtc *DebateTeamConfig) collectOpenRouterFreeModels() {
	// Check if OpenRouter provider is healthy through discovery
	if dtc.discovery != nil {
		discovered := dtc.discovery.GetProviderByName("openrouter")
		if discovered != nil && !discovered.Verified {
			dtc.logger.WithFields(logrus.Fields{
				"provider": "openrouter",
				"status":   discovered.Status,
				"error":    discovered.Error,
			}).Warn("OpenRouter provider not healthy - skipping free models")
			return
		}
	}

	// Get OpenRouter provider (only if verified/healthy)
	provider := dtc.getVerifiedProvider("openrouter")
	if provider == nil {
		dtc.logger.Debug("OpenRouter provider not verified - skipping free models (unhealthy providers cannot provide reliable fallbacks)")
		return
	}

	// Add top OpenRouter free models with scores
	// Higher scores for more capable models
	freeModels := []struct {
		Name  string
		Score float64
	}{
		// Llama 4 (highest priority - latest and most capable)
		{OpenRouterFreeModels.Llama4Maverick, 8.8},
		{OpenRouterFreeModels.Llama4Scout, 8.6},
		{OpenRouterFreeModels.Llama3370B, 8.4},

		// DeepSeek R1 (excellent reasoning capabilities)
		{OpenRouterFreeModels.DeepSeekR1, 8.5},
		{OpenRouterFreeModels.DeepSeekChatV3, 8.3},
		{OpenRouterFreeModels.DeepSeekR1Llama, 8.2},

		// Qwen (good general purpose)
		{OpenRouterFreeModels.QwenQwQ32B, 8.0},

		// Google Gemini (strong reasoning)
		{OpenRouterFreeModels.Gemini25ProExp, 8.7},
		{OpenRouterFreeModels.Gemini20Flash, 8.1},
		{OpenRouterFreeModels.Gemma327B, 7.8},

		// NVIDIA (large model)
		{OpenRouterFreeModels.NvidiaLlama253B, 8.0},

		// Microsoft Phi (efficient)
		{OpenRouterFreeModels.Phi3Medium, 7.5},
		{OpenRouterFreeModels.Phi3Mini, 7.2},

		// Other open-source models
		{OpenRouterFreeModels.Mistral7B, 7.3},
		{OpenRouterFreeModels.Zephyr7B, 7.0},
	}

	for _, m := range freeModels {
		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "openrouter",
			ModelName:    m.Name,
			Score:        m.Score,
			Provider:     provider,
			IsOAuth:      false,
			Verified:     true, // Free models don't require API key verification
		})
	}

	dtc.logger.WithField("models", len(freeModels)).Info("Added OpenRouter Zen free models")
}

// collectZenModels collects OpenCode Zen free models (no API key required)
// These are available through the OpenCode Zen API gateway without authentication
// Zen provider health is verified before adding models
func (dtc *DebateTeamConfig) collectZenModels() {
	// Get Zen provider - it should be discovered with anonymous mode support
	provider := dtc.getVerifiedProvider("zen")
	if provider == nil {
		provider = dtc.getRegisteredProvider("zen")
	}
	if provider == nil {
		dtc.logger.Warn("Zen provider not available - OpenCode Zen free models will not be included in debate team")
		// Note: Zen provider supports anonymous mode, but we need the provider registered
		// If not registered, free models will still be collected if provider discovery runs later
		return
	}

	// Add OpenCode Zen free models with scores
	// These models are available without API key through OpenCode's Zen gateway
	zenModels := []struct {
		Name  string
		Score float64
	}{
		{ZenModels.GrokCodeFast, 8.2}, // xAI Grok code model - fast and capable
		{ZenModels.BigPickle, 8.0},    // Stealth model - good general purpose
		{ZenModels.GLM47Free, 7.8},    // GLM 4.7 - good Chinese/English multilingual
		{ZenModels.GPT5Nano, 7.6},     // GPT 5 Nano - compact and efficient
	}

	for _, m := range zenModels {
		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "zen",
			ModelName:    m.Name,
			Score:        m.Score,
			Provider:     provider,
			IsOAuth:      false,
			Verified:     true, // Zen free models work without API key (anonymous mode)
		})
	}

	dtc.logger.WithField("models", len(zenModels)).Info("Added OpenCode Zen free models")
}

// countOAuthLLMs counts the number of OAuth2 LLMs in the verified list
func (dtc *DebateTeamConfig) countOAuthLLMs() int {
	count := 0
	for _, llm := range dtc.verifiedLLMs {
		if llm.IsOAuth {
			count++
		}
	}
	return count
}

// countFreeModels counts the number of free models (OpenRouter :free + OpenCode Zen) in the verified list
func (dtc *DebateTeamConfig) countFreeModels() int {
	count := 0
	for _, llm := range dtc.verifiedLLMs {
		// Count OpenRouter free models (:free suffix)
		if llm.ProviderName == "openrouter" && len(llm.ModelName) > 5 && llm.ModelName[len(llm.ModelName)-5:] == ":free" {
			count++
		}
		// Count OpenCode Zen free models (opencode/ prefix)
		if llm.ProviderName == "zen" && len(llm.ModelName) > 9 && llm.ModelName[:9] == "opencode/" {
			count++
		}
	}
	return count
}

// countZenModels counts the number of OpenCode Zen models in the verified list
func (dtc *DebateTeamConfig) countZenModels() int {
	count := 0
	for _, llm := range dtc.verifiedLLMs {
		if llm.ProviderName == "zen" {
			count++
		}
	}
	return count
}

// assignPrimaryPositions assigns the best 5 LLMs to primary positions
func (dtc *DebateTeamConfig) assignPrimaryPositions() {
	roles := []struct {
		Position DebateTeamPosition
		Role     DebateRole
	}{
		{PositionAnalyst, RoleAnalyst},
		{PositionProposer, RoleProposer},
		{PositionCritic, RoleCritic},
		{PositionSynthesis, RoleSynthesis},
		{PositionMediator, RoleMediator},
	}

	usedIdx := 0
	for _, r := range roles {
		var llmToUse *VerifiedLLM

		// Find next available LLM (can reuse if needed)
		if usedIdx < len(dtc.verifiedLLMs) {
			llmToUse = dtc.verifiedLLMs[usedIdx]
			usedIdx++
		} else if len(dtc.verifiedLLMs) > 0 {
			// Reuse best available LLM if we've exhausted the list
			llmToUse = dtc.verifiedLLMs[0]
			dtc.logger.WithField("position", r.Position).Debug("Reusing LLM for position (not enough unique LLMs)")
		}

		if llmToUse != nil {
			member := &DebateTeamMember{
				Position:     r.Position,
				Role:         r.Role,
				ProviderName: llmToUse.ProviderName,
				ModelName:    llmToUse.ModelName,
				Provider:     llmToUse.Provider,
				Score:        llmToUse.Score,
				IsActive:     true,
				IsOAuth:      llmToUse.IsOAuth,
			}
			dtc.members[r.Position] = member

			dtc.logger.WithFields(logrus.Fields{
				"position": r.Position,
				"role":     r.Role,
				"provider": llmToUse.ProviderName,
				"model":    llmToUse.ModelName,
				"score":    llmToUse.Score,
				"oauth":    llmToUse.IsOAuth,
			}).Info("Assigned primary position")
		} else {
			dtc.logger.WithField("position", r.Position).Warn("No LLM available for position")
		}
	}
}

// assignAllFallbacks assigns 2 fallbacks to each position
func (dtc *DebateTeamConfig) assignAllFallbacks() {
	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member == nil {
			continue
		}

		// Get fallback LLMs (different from primary if possible)
		fallbacks := dtc.getFallbackLLMs(member.ProviderName, member.ModelName, FallbacksPerPosition)

		// Chain fallbacks
		current := member
		for _, fb := range fallbacks {
			fallbackMember := &DebateTeamMember{
				Position:     pos,
				Role:         member.Role,
				ProviderName: fb.ProviderName,
				ModelName:    fb.ModelName,
				Provider:     fb.Provider,
				Score:        fb.Score,
				IsActive:     false,
				IsOAuth:      fb.IsOAuth,
			}
			current.Fallback = fallbackMember
			current = fallbackMember
		}

		dtc.logger.WithFields(logrus.Fields{
			"position":       pos,
			"fallback_count": len(fallbacks),
		}).Debug("Assigned fallbacks")
	}
}

// getFallbackLLMs returns fallback LLMs different from the primary
// IMPORTANT: For OAuth primaries, prioritize non-OAuth fallbacks to ensure
// fallback chain works when OAuth tokens are incompatible with public APIs
func (dtc *DebateTeamConfig) getFallbackLLMs(primaryProvider, primaryModel string, count int) []*VerifiedLLM {
	fallbacks := make([]*VerifiedLLM, 0, count)

	// Check if primary is OAuth
	primaryIsOAuth := false
	for _, llm := range dtc.verifiedLLMs {
		if llm.ProviderName == primaryProvider && llm.ModelName == primaryModel {
			primaryIsOAuth = llm.IsOAuth
			break
		}
	}

	// First pass: For OAuth primaries, prioritize non-OAuth providers as fallbacks
	// This ensures fallback works when OAuth tokens fail with public APIs
	if primaryIsOAuth {
		for _, llm := range dtc.verifiedLLMs {
			if len(fallbacks) >= count {
				break
			}
			// Prioritize non-OAuth providers for OAuth primaries
			if !llm.IsOAuth && (llm.ProviderName != primaryProvider || llm.ModelName != primaryModel) {
				fallbacks = append(fallbacks, llm)
				dtc.logger.WithFields(logrus.Fields{
					"primary_provider": primaryProvider,
					"fallback_provider": llm.ProviderName,
					"fallback_model":   llm.ModelName,
					"reason":           "non-oauth fallback for oauth primary",
				}).Debug("Selected non-OAuth fallback for OAuth primary")
			}
		}
	}

	// Second pass: If still need more fallbacks, add different provider/model
	for _, llm := range dtc.verifiedLLMs {
		if len(fallbacks) >= count {
			break
		}
		// Skip if already added
		alreadyUsed := false
		for _, fb := range fallbacks {
			if fb == llm {
				alreadyUsed = true
				break
			}
		}
		if alreadyUsed {
			continue
		}
		// Prefer different provider/model
		if llm.ProviderName != primaryProvider || llm.ModelName != primaryModel {
			fallbacks = append(fallbacks, llm)
		}
	}

	// Third pass: If still not enough, allow reuse (last resort)
	for i := 0; len(fallbacks) < count && i < len(dtc.verifiedLLMs); i++ {
		alreadyUsed := false
		for _, fb := range fallbacks {
			if fb == dtc.verifiedLLMs[i] {
				alreadyUsed = true
				break
			}
		}
		if !alreadyUsed {
			fallbacks = append(fallbacks, dtc.verifiedLLMs[i])
		}
	}

	return fallbacks
}

// logTeamComposition logs the final team composition
func (dtc *DebateTeamConfig) logTeamComposition() {
	dtc.logger.Info("=== AI Debate Team Composition (15 LLMs) ===")
	totalLLMs := 0

	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member != nil {
			totalLLMs++
			fields := logrus.Fields{
				"position": pos,
				"role":     member.Role,
				"provider": member.ProviderName,
				"model":    member.ModelName,
				"score":    member.Score,
				"oauth":    member.IsOAuth,
			}

			// Count fallbacks
			fallbackCount := 0
			fb := member.Fallback
			for fb != nil {
				fallbackCount++
				totalLLMs++
				fb = fb.Fallback
			}
			fields["fallbacks"] = fallbackCount

			dtc.logger.WithFields(fields).Info("Position assigned")
		} else {
			dtc.logger.WithField("position", pos).Warn("Position unassigned")
		}
	}

	dtc.logger.WithField("total_llms_used", totalLLMs).Info("Team composition complete")
}

// GetTeamMember returns the team member at the specified position
func (dtc *DebateTeamConfig) GetTeamMember(position DebateTeamPosition) *DebateTeamMember {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()
	return dtc.members[position]
}

// GetActiveMembers returns all active team members
func (dtc *DebateTeamConfig) GetActiveMembers() []*DebateTeamMember {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	members := make([]*DebateTeamMember, 0, len(dtc.members))
	for _, member := range dtc.members {
		if member != nil && member.IsActive {
			members = append(members, member)
		}
	}
	return members
}

// GetAllLLMs returns all 15 LLMs used in the debate team
func (dtc *DebateTeamConfig) GetAllLLMs() []*DebateTeamMember {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	allLLMs := make([]*DebateTeamMember, 0, TotalDebateLLMs)

	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		for member != nil {
			allLLMs = append(allLLMs, member)
			member = member.Fallback
		}
	}

	return allLLMs
}

// GetVerifiedLLMs returns the list of verified LLMs used for team formation
func (dtc *DebateTeamConfig) GetVerifiedLLMs() []*VerifiedLLM {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()
	return dtc.verifiedLLMs
}

// GetTeamSummary returns a summary of the debate team configuration
func (dtc *DebateTeamConfig) GetTeamSummary() map[string]interface{} {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	positions := make([]map[string]interface{}, 0, TotalDebatePositions)
	totalLLMs := 0
	oauthCount := 0
	verifierCount := 0

	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member == nil {
			positions = append(positions, map[string]interface{}{
				"position": pos,
				"status":   "unassigned",
			})
			continue
		}

		totalLLMs++
		if member.IsOAuth {
			oauthCount++
		} else {
			verifierCount++
		}

		posInfo := map[string]interface{}{
			"position":  pos,
			"role":      member.Role,
			"provider":  member.ProviderName,
			"model":     member.ModelName,
			"score":     member.Score,
			"is_active": member.IsActive,
			"is_oauth":  member.IsOAuth,
		}

		if member.Fallback != nil {
			fallbacks := []map[string]interface{}{}
			fb := member.Fallback
			for fb != nil {
				totalLLMs++
				if fb.IsOAuth {
					oauthCount++
				} else {
					verifierCount++
				}
				fallbacks = append(fallbacks, map[string]interface{}{
					"provider": fb.ProviderName,
					"model":    fb.ModelName,
					"score":    fb.Score,
					"is_oauth": fb.IsOAuth,
				})
				fb = fb.Fallback
			}
			posInfo["fallback_chain"] = fallbacks
		}

		positions = append(positions, posInfo)
	}

	return map[string]interface{}{
		"team_name":             "HelixAgent AI Debate Team",
		"total_positions":       TotalDebatePositions,
		"total_llms":            totalLLMs,
		"expected_llms":         TotalDebateLLMs,
		"oauth_llms":            oauthCount,
		"llmsverifier_llms":     verifierCount,
		"active_positions":      len(dtc.GetActiveMembers()),
		"positions":             positions,
		"verified_llms_count":   len(dtc.verifiedLLMs),
		"claude_models": map[string]string{
			// Claude 4.5 (Latest)
			"opus_45":   ClaudeModels.Opus45,
			"sonnet_45": ClaudeModels.Sonnet45,
			"haiku_45":  ClaudeModels.Haiku45,
			// Claude 4.x
			"opus_4":   ClaudeModels.Opus4,
			"sonnet_4": ClaudeModels.Sonnet4,
			// Claude 3.5 (Fallbacks)
			"sonnet_35": ClaudeModels.Sonnet35,
			"haiku_35":  ClaudeModels.Haiku35,
			// Claude 3 (Legacy)
			"opus_3":   ClaudeModels.Opus3,
			"sonnet_3": ClaudeModels.Sonnet3,
			"haiku_3":  ClaudeModels.Haiku3,
		},
		"qwen_models": map[string]string{
			"turbo": QwenModels.Turbo,
			"plus":  QwenModels.Plus,
			"max":   QwenModels.Max,
			"coder": QwenModels.Coder,
			"long":  QwenModels.Long,
		},
		"llmsverifier_models": map[string]string{
			"deepseek": LLMsVerifierModels.DeepSeek,
			"gemini":   LLMsVerifierModels.Gemini,
			"mistral":  LLMsVerifierModels.Mistral,
			"groq":     LLMsVerifierModels.Groq,
			"cerebras": LLMsVerifierModels.Cerebras,
		},
		"openrouter_free_models": map[string]string{
			"llama4_maverick":   OpenRouterFreeModels.Llama4Maverick,
			"llama4_scout":      OpenRouterFreeModels.Llama4Scout,
			"llama33_70b":       OpenRouterFreeModels.Llama3370B,
			"deepseek_r1":       OpenRouterFreeModels.DeepSeekR1,
			"deepseek_chat_v3":  OpenRouterFreeModels.DeepSeekChatV3,
			"qwen_qwq_32b":      OpenRouterFreeModels.QwenQwQ32B,
			"gemini_25_pro_exp": OpenRouterFreeModels.Gemini25ProExp,
			"gemini_20_flash":   OpenRouterFreeModels.Gemini20Flash,
			"nvidia_llama_253b": OpenRouterFreeModels.NvidiaLlama253B,
			"phi3_medium":       OpenRouterFreeModels.Phi3Medium,
		},
		"zen_models": map[string]string{
			"big_pickle":     ZenModels.BigPickle,
			"grok_code_fast": ZenModels.GrokCodeFast,
			"glm_47_free":    ZenModels.GLM47Free,
			"gpt_5_nano":     ZenModels.GPT5Nano,
		},
		"zen_models_count": dtc.countZenModels(),
	}
}

// ActivateFallback activates the fallback for a position when the primary fails
func (dtc *DebateTeamConfig) ActivateFallback(position DebateTeamPosition) (*DebateTeamMember, error) {
	dtc.mu.Lock()
	defer dtc.mu.Unlock()

	member := dtc.members[position]
	if member == nil {
		return nil, fmt.Errorf("no member at position %d", position)
	}

	if member.Fallback == nil {
		return nil, fmt.Errorf("no fallback available for position %d", position)
	}

	// Deactivate current member
	member.IsActive = false

	// Activate fallback
	fallback := member.Fallback
	fallback.IsActive = true
	dtc.members[position] = fallback

	dtc.logger.WithFields(logrus.Fields{
		"position":     position,
		"old_provider": member.ProviderName,
		"new_provider": fallback.ProviderName,
		"new_model":    fallback.ModelName,
	}).Info("Activated fallback for debate position")

	return fallback, nil
}

// GetProviderForPosition returns the appropriate provider for a debate position
func (dtc *DebateTeamConfig) GetProviderForPosition(position DebateTeamPosition) (llm.LLMProvider, string, error) {
	member := dtc.GetTeamMember(position)
	if member == nil {
		return nil, "", fmt.Errorf("no member assigned to position %d", position)
	}

	if member.Provider == nil {
		// Try to activate fallback
		fallback, err := dtc.ActivateFallback(position)
		if err != nil {
			return nil, "", fmt.Errorf("provider unavailable and no fallback: %w", err)
		}
		return fallback.Provider, fallback.ModelName, nil
	}

	return member.Provider, member.ModelName, nil
}

// CountTotalLLMs returns the total number of LLMs in the team (including fallbacks)
func (dtc *DebateTeamConfig) CountTotalLLMs() int {
	return len(dtc.GetAllLLMs())
}

// IsFullyPopulated returns true if all 15 LLM slots are filled
func (dtc *DebateTeamConfig) IsFullyPopulated() bool {
	return dtc.CountTotalLLMs() >= TotalDebateLLMs
}
