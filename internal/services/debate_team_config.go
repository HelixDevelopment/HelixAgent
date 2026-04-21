package services

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/verifier"
	"github.com/sirupsen/logrus"
)

// TotalDebatePositions is the total number of positions in the AI debate team
const TotalDebatePositions = 5

// MinFallbacksPerPosition is the minimum number of fallbacks per position
const MinFallbacksPerPosition = 2

// MaxFallbacksPerPosition is the maximum number of fallbacks per position
const MaxFallbacksPerPosition = 4

// FallbacksPerPosition is the target number of fallbacks per position
// This will be adjusted based on available verified LLMs
const FallbacksPerPosition = MaxFallbacksPerPosition

// TotalDebateLLMs is the maximum number of LLMs used (positions * (1 primary + max fallbacks))
// Actual count may be lower if not enough LLMs are verified, but LLM reuse is allowed
const TotalDebateLLMs = TotalDebatePositions * (1 + MaxFallbacksPerPosition) // 25 LLMs max

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
// Updated 2026-02-26 with Claude 4.6 Sonnet
var ClaudeModels = struct {
	// Claude 4.6 (Latest generation - February 2026)
	Opus46   string // claude-opus-4-6 - Most capable, latest
	Sonnet46 string // claude-sonnet-4-6 - Latest balanced model

	// Claude 4.5 (November 2025)
	Opus45   string // claude-opus-4-5-20251101 - Previous flagship
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
	// Claude 4.6 (Latest - Primary model for AI Debate Team)
	Opus46:   "claude-opus-4-6",
	Sonnet46: "claude-sonnet-4-6",

	// Claude 4.5
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
	ZAI      string // Zhipu GLM-4.7 (latest)
	Chutes   string // Chutes AI DeepSeek-V3
}{
	DeepSeek: "deepseek-chat",
	Gemini:   "gemini-2.5-flash",
	Mistral:  "mistral-large-latest",
	Groq:     "llama-4-maverick-17b-128e-instruct",
	Cerebras: "llama3.1-8b",
	ZAI:      "glm-4.7",
	Chutes:   "deepseek-ai/DeepSeek-V3",
}

// OpenRouterFreeModels defines OpenRouter Zen free models (with :free suffix)
// These are high-quality models available for free through OpenRouter
var OpenRouterFreeModels = struct {
	// Llama 4 models (Meta)
	Llama4Maverick string
	Llama4Scout    string
	Llama3370B     string

	// DeepSeek models (reasoning)
	DeepSeekR1       string
	DeepSeekR1Zero   string
	DeepSeekChatV3   string
	DeepSeekR1Llama  string
	DeepSeekR1Qwen32 string
	DeepSeekR1Qwen14 string

	// Qwen models
	QwenQwQ32B string
	QwenVL3B   string

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

// ZenModels defines ALL OpenCode Zen free models (no API key required)
// These are high-quality models available through OpenCode's Zen API gateway
// ALL models are evaluated - only those passing verification join the debate team
// NOTE: Zen API requires model names WITHOUT "opencode/" prefix
var ZenModels = struct {
	// Primary models
	BigPickle string
	GPT5Nano  string
	GLM47     string
	// Qwen models
	Qwen3Coder string
	Qwen3235B  string
	Qwen332B   string
	// Kimi models
	KimiK2     string
	KimiLatest string
	// Gemini models
	Gemini3Flash  string
	Gemini25Pro   string
	Gemini20Flash string
	// DeepSeek models
	DeepSeekR1    string
	DeepSeekV3    string
	DeepSeekCoder string
	// Grok models
	GrokCode string
	Grok3    string
	Grok2    string
	// Claude models
	ClaudeSonnet4 string
	ClaudeHaiku4  string
	// Llama models
	Llama4Maverick string
	Llama4Scout    string
	Llama3370B     string
	// Mistral models
	MistralLarge string
	Codestral    string
	// Other models
	O3Mini   string
	O1Mini   string
	GPT4O    string
	CommandA string
}{
	BigPickle:      "big-pickle",
	GPT5Nano:       "gpt-5-nano",
	GLM47:          "glm-4.7",
	Qwen3Coder:     "qwen3-coder",
	Qwen3235B:      "qwen3-235b",
	Qwen332B:       "qwen3-32b",
	KimiK2:         "kimi-k2",
	KimiLatest:     "kimi-latest",
	Gemini3Flash:   "gemini-3-flash",
	Gemini25Pro:    "gemini-2.5-pro",
	Gemini20Flash:  "gemini-2.0-flash",
	DeepSeekR1:     "deepseek-r1",
	DeepSeekV3:     "deepseek-v3",
	DeepSeekCoder:  "deepseek-coder",
	GrokCode:       "grok-code",
	Grok3:          "grok-3",
	Grok2:          "grok-2",
	ClaudeSonnet4:  "claude-sonnet-4",
	ClaudeHaiku4:   "claude-haiku-4",
	Llama4Maverick: "llama-4-maverick",
	Llama4Scout:    "llama-4-scout",
	Llama3370B:     "llama-3.3-70b",
	MistralLarge:   "mistral-large",
	Codestral:      "codestral",
	O3Mini:         "o3-mini",
	O1Mini:         "o1-mini",
	GPT4O:          "gpt-4o",
	CommandA:       "command-a-03-2025",
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
	Position     DebateTeamPosition  `json:"position"`
	Role         DebateRole          `json:"role"`
	ProviderName string              `json:"provider_name"`
	ModelName    string              `json:"model_name"`
	Provider     llm.LLMProvider     `json:"-"`
	Fallback     *DebateTeamMember   `json:"fallback,omitempty"`  // Deprecated: use Fallbacks
	Fallbacks    []*DebateTeamMember `json:"fallbacks,omitempty"` // All fallbacks for this position (2-4)
	Score        float64             `json:"score"`
	IsActive     bool                `json:"is_active"`
	IsOAuth      bool                `json:"is_oauth"`
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
// 1. ALL verified providers sorted by LLMsVerifier score (NO OAuth priority)
// 2. Strongest LLMs get primary positions
// 3. Next strongest become fallbacks (2-4 per position)
// 4. If not enough unique LLMs, reuse strongest ones (independent instances)
// Total: Up to 25 LLMs (5 positions × (1 primary + 4 fallbacks))
func (dtc *DebateTeamConfig) InitializeTeam(ctx context.Context) error {
	dtc.mu.Lock()
	defer dtc.mu.Unlock()

	dtc.logger.Info("Initializing AI Debate Team (up to 25 LLMs)...")
	dtc.logger.Info("Strategy: ALL providers sorted by score (NO OAuth priority) - strongest LLMs first")

	// Step 1: Verify all providers and collect verified LLMs
	dtc.collectVerifiedLLMs(ctx)

	// Step 2: Sort verified LLMs PURELY by score (highest first) - NO OAuth priority
	sort.Slice(dtc.verifiedLLMs, func(i, j int) bool {
		// Sort purely by score - highest score first
		return dtc.verifiedLLMs[i].Score > dtc.verifiedLLMs[j].Score
	})

	dtc.logger.WithField("verified_count", len(dtc.verifiedLLMs)).Info("Collected verified LLMs sorted by score")

	// Step 3: Assign primary positions (5 positions) - strongest LLMs get primary
	dtc.assignPrimaryPositions()

	// Step 4: Assign fallbacks (2-4 per position) - next strongest LLMs
	dtc.assignAllFallbacks()

	// Step 5: Log final team composition
	dtc.logTeamComposition()

	dtc.logger.WithFields(logrus.Fields{
		"total_positions": TotalDebatePositions,
		"max_llms":        TotalDebateLLMs,
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
	} else {
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
	}

	// Filter out local/Ollama providers - debate MUST use remote API providers only.
	// Local models (Ollama, codellama, etc.) are unsuitable for multi-LLM debate because
	// they typically run on the same machine, have limited capabilities, and cannot provide
	// the diversity of reasoning needed for effective debate positions.
	dtc.filterLocalProviders()

	dtc.logger.WithFields(logrus.Fields{
		"total_verified":    len(dtc.verifiedLLMs),
		"oauth_count":       dtc.countOAuthLLMs(),
		"free_models_count": dtc.countFreeModels(),
	}).Info("Verified LLMs collected")
}

// filterLocalProviders removes Ollama, local-only, and slow CLI-proxy providers
// from the verified LLMs list. Debate requires fast remote API providers.
func (dtc *DebateTeamConfig) filterLocalProviders() {
	beforeCount := len(dtc.verifiedLLMs)
	filtered := make([]*VerifiedLLM, 0, len(dtc.verifiedLLMs))
	for _, llm := range dtc.verifiedLLMs {
		providerLower := strings.ToLower(llm.ProviderName)

		// Skip Ollama/local providers
		if providerLower == "ollama" || providerLower == "local" {
			dtc.logger.WithFields(logrus.Fields{
				"provider": llm.ProviderName,
				"model":    llm.ModelName,
			}).Debug("Skipping local/Ollama provider for debate team")
			continue
		}

		// Skip OAuth/CLI-proxy providers (Claude, Qwen, Junie, Zen) —
		// they use CLI subprocess calls that are too slow for real-time debate
		// (Claude CLI takes 45-180 seconds per call, blocking the entire round)
		if llm.IsOAuth {
			dtc.logger.WithFields(logrus.Fields{
				"provider": llm.ProviderName,
				"model":    llm.ModelName,
				"reason":   "OAuth/CLI proxy too slow for debate",
			}).Debug("Skipping OAuth/CLI provider for debate team (use API-key providers)")
			continue
		}

		// Also skip bare local model names
		modelLower := strings.ToLower(llm.ModelName)
		if isLocalOnlyModel(modelLower) {
			dtc.logger.WithFields(logrus.Fields{
				"provider": llm.ProviderName,
				"model":    llm.ModelName,
			}).Debug("Skipping local-only model for debate team")
			continue
		}
		filtered = append(filtered, llm)
	}
	dtc.verifiedLLMs = filtered
	removedCount := beforeCount - len(dtc.verifiedLLMs)
	if removedCount > 0 {
		dtc.logger.WithFields(logrus.Fields{
			"removed":   removedCount,
			"remaining": len(dtc.verifiedLLMs),
		}).Info("Filtered out local/Ollama providers from debate team")
	}
}

// isLocalOnlyModel checks if a model name indicates a local-only model (e.g., Ollama models
// that don't have a provider path prefix like "openrouter/meta-llama/llama-3.1").
func isLocalOnlyModel(modelLower string) bool {
	// Only match bare model names without path separators (e.g., "codellama", "llama3.2")
	// Models with path prefixes like "meta-llama/llama-3.1-8b:free" are remote and should NOT be filtered
	if strings.Contains(modelLower, "/") {
		return false
	}
	localModels := []string{"codellama", "llama3.2", "llama3.1", "llama2", "mistral-local", "phi-2", "phi-3"}
	for _, local := range localModels {
		if modelLower == local || strings.HasPrefix(modelLower, local+":") {
			return true
		}
	}
	return false
}

// collectFromStartupVerifier collects verified LLMs from the unified StartupVerifier
func (dtc *DebateTeamConfig) collectFromStartupVerifier() {
	rankedProviders := dtc.startupVerifier.GetRankedProviders()

	for _, provider := range rankedProviders {
		if !provider.Verified {
			continue
		}

		// Skip providers with nil Instance - they can't be used for debate
		if provider.Instance == nil {
			dtc.logger.WithFields(logrus.Fields{
				"provider": provider.Name,
				"reason":   "nil instance",
			}).Debug("Skipping provider with nil instance")
			continue
		}

		// Skip Ollama/local providers - debate requires remote API providers
		if strings.ToLower(provider.Type) == "ollama" || strings.ToLower(provider.Name) == "ollama" {
			dtc.logger.WithFields(logrus.Fields{
				"provider": provider.Name,
				"type":     provider.Type,
			}).Debug("Skipping Ollama provider from StartupVerifier for debate team")
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

// collectClaudeModels collects Claude models ONLY if a verified provider is available
//
// IMPORTANT: Claude OAuth tokens from Claude Code CLI are PRODUCT-RESTRICTED.
// They can ONLY be used with Claude Code itself - NOT with the standard Anthropic API.
// Therefore, Claude models should ONLY be added if:
// 1. A valid Anthropic API key is available, OR
// 2. The provider has been VERIFIED to work (not just "registered")
//
// We DO NOT fall back to registered-but-unverified providers because OAuth tokens
// from CLI tools cannot be used for general API access.
func (dtc *DebateTeamConfig) collectClaudeModels() {
	// ONLY use verified providers - do NOT fall back to registered-but-unverified
	provider := dtc.getVerifiedProvider("claude", "claude-oauth")
	if provider == nil {
		dtc.logger.Warn("Claude provider not verified - OAuth tokens from Claude Code CLI are product-restricted and cannot be used for API calls")
		return
	}

	// Add all Claude models (prioritized by generation and capability)
	// Claude 4.6 > 4.5 > 4.x > 3.5 > 3.x by score
	claudeModels := []struct {
		Name  string
		Score float64
	}{
		// Claude 4.6 (Latest - highest score)
		{ClaudeModels.Opus46, 9.95},   // Most capable Claude model (February 2026)
		{ClaudeModels.Sonnet46, 9.85}, // Latest balanced model (February 2026)

		// Claude 4.5 (Primary)
		{ClaudeModels.Opus45, 9.7},   // Previous flagship
		{ClaudeModels.Sonnet45, 9.5}, // High quality balanced
		{ClaudeModels.Haiku45, 9.0},  // Fast and efficient

		// Claude 4.x (Secondary)
		{ClaudeModels.Opus4, 9.3},
		{ClaudeModels.Sonnet4, 9.1},

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

	dtc.logger.WithField("models", len(claudeModels)).Info("Added Claude API models (verified provider)")
}

// collectQwenModels collects Qwen models ONLY if a verified provider is available
//
// IMPORTANT: Qwen OAuth tokens from Qwen CLI are for the Qwen Portal only.
// They cannot be used with the DashScope API.
// Therefore, Qwen models should ONLY be added if:
// 1. A valid DashScope API key is available, OR
// 2. The provider has been VERIFIED to work (not just "registered")
//
// We DO NOT fall back to registered-but-unverified providers because OAuth tokens
// from CLI tools cannot be used for general API access.
func (dtc *DebateTeamConfig) collectQwenModels() {
	// ONLY use verified providers - do NOT fall back to registered-but-unverified
	provider := dtc.getVerifiedProvider("qwen", "qwen-oauth")
	if provider == nil {
		dtc.logger.Warn("Qwen provider not verified - OAuth tokens from Qwen CLI are for Portal only, cannot be used for DashScope API")
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

	dtc.logger.WithField("models", len(qwenModels)).Info("Added Qwen API models (verified provider)")
}

// collectReliableAPIProviders collects API providers ONLY if they are verified
//
// IMPORTANT: This function now uses the verification system (discovery) to ensure
// providers actually work before adding them to the debate team. Previously, it
// added providers based solely on environment variables, which led to non-functional
// providers being included in the debate team.
//
// Providers are only added if:
// 1. The API key is present in the environment
// 2. The provider has been VERIFIED by the startup verification system
func (dtc *DebateTeamConfig) collectReliableAPIProviders() {
	// List of reliable API providers to check
	reliableProviders := []struct {
		name   string
		envVar string
		model  string
	}{
		{"cerebras", "CEREBRAS_API_KEY", LLMsVerifierModels.Cerebras},
		{"mistral", "MISTRAL_API_KEY", LLMsVerifierModels.Mistral},
		{"deepseek", "DEEPSEEK_API_KEY", LLMsVerifierModels.DeepSeek},
		{"gemini", "GEMINI_API_KEY", LLMsVerifierModels.Gemini},
		{"zai", "ZAI_API_KEY", LLMsVerifierModels.ZAI},
		{"chutes", "CHUTES_API_KEY", LLMsVerifierModels.Chutes},
	}

	for _, rp := range reliableProviders {
		if apiKey := os.Getenv(rp.envVar); apiKey == "" {
			dtc.logger.WithField("provider", rp.name).Debug("API key not set - provider not available")
			continue
		}

		// ONLY use verified providers from discovery
		provider := dtc.getVerifiedProvider(rp.name)
		if provider == nil {
			dtc.logger.WithFields(logrus.Fields{
				"provider": rp.name,
				"env_var":  rp.envVar,
			}).Warn("Provider has API key but verification failed - NOT adding to debate team")
			continue
		}

		// Get score from discovery if available
		score := 8.0 // Default score
		if dtc.discovery != nil {
			if discovered := dtc.discovery.GetProviderByName(rp.name); discovered != nil && discovered.Verified {
				score = discovered.Score
			}
		}

		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: rp.name,
			ModelName:    rp.model,
			Score:        score,
			Provider:     provider,
			IsOAuth:      false,
			Verified:     true,
		})
		dtc.logger.WithFields(logrus.Fields{
			"provider": rp.name,
			"model":    rp.model,
			"score":    score,
		}).Info("Added verified API provider to debate team")
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
		// IMPORTANT: Only use discovery which tracks verification status.
		// The registry does NOT track verification, so we skip it for verified providers.
		// This prevents non-functional OAuth providers from being used.
		if dtc.discovery != nil {
			if discovered := dtc.discovery.GetProviderByName(name); discovered != nil {
				if discovered.Verified && discovered.Provider != nil {
					dtc.logger.WithFields(logrus.Fields{
						"provider": name,
						"verified": true,
						"score":    discovered.Score,
					}).Debug("Found verified provider via discovery")
					return discovered.Provider
				}
				dtc.logger.WithFields(logrus.Fields{
					"provider": name,
					"verified": discovered.Verified,
				}).Debug("Provider found but not verified - skipping")
			}
		}
	}
	return nil
}

// collectOpenRouterFreeModels collects OpenRouter Zen free models (:free suffix)
// that have been VERIFIED through the FreeProviderAdapter verification pipeline.
// IMPORTANT: Only models that passed verification (including canned error checks) are added.
// DO NOT add hardcoded models with assumed Verified: true.
func (dtc *DebateTeamConfig) collectOpenRouterFreeModels() {
	// IMPORTANT: Only use discovery to get verified OpenRouter provider and models.
	// DO NOT add hardcoded models - unverified models cannot be in the debate team.
	if dtc.discovery == nil {
		dtc.logger.Debug("Discovery not available - cannot collect OpenRouter free models")
		return
	}

	discovered := dtc.discovery.GetProviderByName("openrouter")
	if discovered == nil {
		dtc.logger.Debug("OpenRouter provider not discovered")
		return
	}

	// Provider must be verified (passed health checks)
	if !discovered.Verified {
		dtc.logger.WithFields(logrus.Fields{
			"provider": "openrouter",
			"status":   discovered.Status,
			"error":    discovered.Error,
		}).Warn("OpenRouter provider not verified - free models will not be included in debate team")
		return
	}

	if discovered.Provider == nil {
		dtc.logger.Warn("OpenRouter provider instance not available")
		return
	}

	// Only add free models (:free suffix) that have been ACTUALLY VERIFIED
	// through the verification pipeline (model completion test with canned error detection)
	addedCount := 0
	verifiedModelsSnapshot := []VerifiedModel{}
	if discovered.VerifiedModels != nil {
		verifiedModelsSnapshot = discovered.VerifiedModels.Snapshot()
	}
	for _, model := range verifiedModelsSnapshot {
		// Only include free models (with :free suffix)
		if len(model.Name) <= 5 || model.Name[len(model.Name)-5:] != ":free" {
			continue
		}

		// CRITICAL: Only add models that passed verification
		if !model.Verified {
			dtc.logger.WithFields(logrus.Fields{
				"provider": "openrouter",
				"model":    model.Name,
			}).Debug("Skipping unverified OpenRouter free model")
			continue
		}

		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "openrouter",
			ModelName:    model.Name,
			Score:        model.Score,
			Provider:     discovered.Provider,
			IsOAuth:      false,
			Verified:     true, // Model actually passed verification
		})
		addedCount++
	}

	if addedCount == 0 {
		dtc.logger.Warn("No OpenRouter free models passed verification - they will not be included in debate team")
	} else {
		dtc.logger.WithField("models", addedCount).Info("Added verified OpenRouter Zen free models")
	}
}

// collectZenModels collects OpenCode Zen free models that have been VERIFIED
// through the FreeProviderAdapter verification pipeline.
// IMPORTANT: Only models that passed verification (including canned error checks) are added.
// DO NOT add hardcoded models with assumed Verified: true.
func (dtc *DebateTeamConfig) collectZenModels() {
	// IMPORTANT: Only use discovery to get verified Zen provider and models.
	// DO NOT fall back to registry - unverified providers cannot be in the debate team.
	if dtc.discovery == nil {
		dtc.logger.Debug("Discovery not available - cannot collect Zen models")
		return
	}

	discovered := dtc.discovery.GetProviderByName("zen")
	if discovered == nil {
		dtc.logger.Debug("Zen provider not discovered")
		return
	}

	// Provider must be verified (passed health checks and model completion tests)
	if !discovered.Verified {
		dtc.logger.WithFields(logrus.Fields{
			"provider": "zen",
			"status":   discovered.Status,
			"error":    discovered.Error,
		}).Warn("Zen provider not verified - models will not be included in debate team")
		return
	}

	if discovered.Provider == nil {
		dtc.logger.Warn("Zen provider instance not available")
		return
	}

	// Only add models that have been ACTUALLY VERIFIED through the verification pipeline
	// This means they passed the model completion test including canned error detection
	addedCount := 0
	verifiedModelsSnapshot := []VerifiedModel{}
	if discovered.VerifiedModels != nil {
		verifiedModelsSnapshot = discovered.VerifiedModels.Snapshot()
	}
	for _, model := range verifiedModelsSnapshot {
		// CRITICAL: Only add models that passed verification
		if !model.Verified {
			dtc.logger.WithFields(logrus.Fields{
				"provider": "zen",
				"model":    model.Name,
			}).Debug("Skipping unverified Zen model")
			continue
		}

		dtc.verifiedLLMs = append(dtc.verifiedLLMs, &VerifiedLLM{
			ProviderName: "zen",
			ModelName:    model.Name,
			Score:        model.Score,
			Provider:     discovered.Provider,
			IsOAuth:      false,
			Verified:     true, // Model actually passed verification
		})
		addedCount++
	}

	if addedCount == 0 {
		dtc.logger.Warn("No Zen models passed verification - they will not be included in debate team")
	} else {
		dtc.logger.WithField("models", addedCount).Info("Added verified OpenCode Zen free models")
	}
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

		// Find next available LLM WITH a valid Provider instance (can reuse if needed)
		for usedIdx < len(dtc.verifiedLLMs) {
			candidate := dtc.verifiedLLMs[usedIdx]
			usedIdx++
			if candidate.Provider != nil {
				llmToUse = candidate
				break
			}
			dtc.logger.WithFields(logrus.Fields{
				"provider": candidate.ProviderName,
				"model":    candidate.ModelName,
			}).Debug("Skipping LLM with nil Provider instance for primary position")
		}

		// If we exhausted the list, reuse the first LLM with a valid Provider
		if llmToUse == nil && len(dtc.verifiedLLMs) > 0 {
			for _, candidate := range dtc.verifiedLLMs {
				if candidate.Provider != nil {
					llmToUse = candidate
					dtc.logger.WithField("position", r.Position).Debug("Reusing LLM for position (not enough unique LLMs with valid Provider)")
					break
				}
			}
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

		// Get fallback LLMs (different from primary if possible, with OAuth diversity)
		fallbacks := dtc.getFallbackLLMs(member.ProviderName, member.ModelName, member.IsOAuth, FallbacksPerPosition)

		// Initialize Fallbacks slice
		member.Fallbacks = make([]*DebateTeamMember, 0, len(fallbacks))

		// Chain fallbacks (for backwards compatibility) and populate Fallbacks slice
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
			// Add to Fallbacks slice (new format)
			member.Fallbacks = append(member.Fallbacks, fallbackMember)
			// Chain for backwards compatibility (old format)
			current.Fallback = fallbackMember
			current = fallbackMember
		}

		dtc.logger.WithFields(logrus.Fields{
			"position":       pos,
			"fallback_count": len(fallbacks),
		}).Debug("Assigned fallbacks")
	}
}

// getFallbackLLMs returns fallback LLMs with provider diversity and OAuth awareness.
// Strategy: First pick the best model from each UNIQUE provider (excluding primary),
// sorted by score. This ensures all verified providers appear in the debate team.
// Then fill remaining slots by score. Maintains OAuth diversity when primary is OAuth.
func (dtc *DebateTeamConfig) getFallbackLLMs(primaryProvider, primaryModel string, primaryIsOAuth bool, count int) []*VerifiedLLM {
	fallbacks := make([]*VerifiedLLM, 0, count)
	usedProviders := map[string]bool{primaryProvider: true}
	hasNonOAuth := false

	// First pass: Pick best model from each UNIQUE provider (provider diversity)
	// verifiedLLMs is sorted by score, so the first hit for each provider is its best
	for _, llm := range dtc.verifiedLLMs {
		if len(fallbacks) >= count {
			break
		}
		// Skip primary LLM
		if llm.ProviderName == primaryProvider && llm.ModelName == primaryModel {
			continue
		}
		// Skip if we already have this provider represented
		if usedProviders[llm.ProviderName] {
			continue
		}
		fallbacks = append(fallbacks, llm)
		usedProviders[llm.ProviderName] = true
		if !llm.IsOAuth {
			hasNonOAuth = true
		}
		dtc.logger.WithFields(logrus.Fields{
			"primary_provider":  primaryProvider,
			"fallback_provider": llm.ProviderName,
			"fallback_model":    llm.ModelName,
			"fallback_score":    llm.Score,
			"fallback_rank":     len(fallbacks),
			"reason":            "provider diversity (unique provider)",
		}).Debug("Selected fallback for provider diversity")
	}

	// Second pass: If still need more, add best remaining models regardless of provider
	for _, llm := range dtc.verifiedLLMs {
		if len(fallbacks) >= count {
			break
		}
		if llm.ProviderName == primaryProvider && llm.ModelName == primaryModel {
			continue
		}
		alreadyUsed := false
		for _, fb := range fallbacks {
			if fb.ProviderName == llm.ProviderName && fb.ModelName == llm.ModelName {
				alreadyUsed = true
				break
			}
		}
		if alreadyUsed {
			continue
		}
		fallbacks = append(fallbacks, llm)
		if !llm.IsOAuth {
			hasNonOAuth = true
		}
		dtc.logger.WithFields(logrus.Fields{
			"primary_provider":  primaryProvider,
			"fallback_provider": llm.ProviderName,
			"fallback_model":    llm.ModelName,
			"fallback_score":    llm.Score,
			"fallback_rank":     len(fallbacks),
		}).Debug("Selected fallback by score")
	}

	// OAuth diversity check: If primary is OAuth but no non-OAuth fallbacks, ensure we have one
	if primaryIsOAuth && !hasNonOAuth {
		var nonOAuthCandidate *VerifiedLLM
		for _, llm := range dtc.verifiedLLMs {
			if llm.IsOAuth {
				continue
			}
			alreadyUsed := false
			for _, fb := range fallbacks {
				if fb.ProviderName == llm.ProviderName && fb.ModelName == llm.ModelName {
					alreadyUsed = true
					break
				}
			}
			if !alreadyUsed {
				nonOAuthCandidate = llm
				break
			}
		}
		if nonOAuthCandidate != nil {
			if len(fallbacks) < count {
				fallbacks = append(fallbacks, nonOAuthCandidate)
			} else {
				fallbacks[len(fallbacks)-1] = nonOAuthCandidate
			}
			dtc.logger.WithFields(logrus.Fields{
				"primary_provider":  primaryProvider,
				"fallback_provider": nonOAuthCandidate.ProviderName,
				"fallback_model":    nonOAuthCandidate.ModelName,
				"fallback_score":    nonOAuthCandidate.Score,
				"reason":            "OAuth diversity (OAuth primary needs non-OAuth fallback)",
			}).Debug("Added non-OAuth fallback for diversity")
		}
	}

	// Third pass: If not enough unique LLMs, reuse strongest ones (independent instances)
	reuseIdx := 0
	for len(fallbacks) < count && len(dtc.verifiedLLMs) > 0 {
		llm := dtc.verifiedLLMs[reuseIdx%len(dtc.verifiedLLMs)]
		fallbacks = append(fallbacks, llm)
		dtc.logger.WithFields(logrus.Fields{
			"primary_provider": primaryProvider,
			"reused_provider":  llm.ProviderName,
			"reused_model":     llm.ModelName,
			"reused_score":     llm.Score,
			"reason":           "LLM reuse for fallback completeness",
		}).Debug("Reusing LLM as independent instance for fallback")
		reuseIdx++
	}

	return fallbacks
}

// logTeamComposition logs the final team composition
func (dtc *DebateTeamConfig) logTeamComposition() {
	dtc.logger.Info("=== AI Debate Team Composition (up to 25 LLMs, sorted by score) ===")
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

// GetAllLLMs returns all 25 LLMs used in the debate team (primaries + all fallbacks)
// Note: For UI display, use GetPrimaryMembers() instead to get only 5 primary members
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

// GetPrimaryMembers returns only the 5 primary members (one per position)
// Each member has its Fallbacks slice populated with all fallback LLMs
// Use this for UI display to show each position once with its fallbacks
func (dtc *DebateTeamConfig) GetPrimaryMembers() []*DebateTeamMember {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	primaries := make([]*DebateTeamMember, 0, TotalDebatePositions)

	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member != nil {
			primaries = append(primaries, member)
		}
	}

	return primaries
}

// ToParticipantConfig converts a DebateTeamMember to a ParticipantConfig
// CRITICAL: This preserves the ProviderInstance from the verified member,
// ensuring that CLI-based OAuth providers are used instead of API-based
// providers from the registry lookup.
func (m *DebateTeamMember) ToParticipantConfig() ParticipantConfig {
	config := ParticipantConfig{
		ParticipantID:    fmt.Sprintf("%s-%s", m.ProviderName, m.ModelName),
		Name:             fmt.Sprintf("%s (%s)", m.ProviderName, m.ModelName),
		Role:             string(m.Role),
		LLMProvider:      m.ProviderName,
		LLMModel:         m.ModelName,
		Weight:           m.Score / 10.0, // Normalize score to weight
		ProviderInstance: m.Provider,     // CRITICAL: Use verified provider instance
	}

	// Convert fallbacks
	if len(m.Fallbacks) > 0 {
		config.Fallbacks = make([]FallbackConfig, len(m.Fallbacks))
		for i, fb := range m.Fallbacks {
			config.Fallbacks[i] = FallbackConfig{
				Provider:         fb.ProviderName,
				Model:            fb.ModelName,
				ProviderInstance: fb.Provider, // CRITICAL: Use verified provider instance
			}
		}
	}

	return config
}

// GetParticipantConfigs converts all primary members to ParticipantConfig slice
// This is the primary method to get verified participants for a debate.
func (dtc *DebateTeamConfig) GetParticipantConfigs() []ParticipantConfig {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	participants := make([]ParticipantConfig, 0, TotalDebatePositions)
	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member != nil && member.IsActive {
			participants = append(participants, member.ToParticipantConfig())
		}
	}
	return participants
}

// GetVerifiedProviderInstance returns the verified LLMProvider instance for a given
// provider name and model. This is used by DebateService to get the verified provider
// instance (which may be CLI-based for OAuth providers) instead of using registry lookup.
// Returns nil if no matching verified provider is found.
func (dtc *DebateTeamConfig) GetVerifiedProviderInstance(providerName, modelName string) llm.LLMProvider {
	dtc.mu.RLock()
	defer dtc.mu.RUnlock()

	// First, check the team members (which have Provider instances set from verification)
	for pos := PositionAnalyst; pos <= PositionMediator; pos++ {
		member := dtc.members[pos]
		if member == nil {
			continue
		}
		// Check primary
		if member.ProviderName == providerName && member.ModelName == modelName && member.Provider != nil {
			return member.Provider
		}
		// Check fallbacks (fallbacks may have different providers than primary)
		for _, fb := range member.Fallbacks {
			if fb != nil && fb.ProviderName == providerName && fb.ModelName == modelName && fb.Provider != nil {
				return fb.Provider
			}
		}
	}

	// Also check verifiedLLMs (contains all verified LLMs with their provider instances)
	for _, vllm := range dtc.verifiedLLMs {
		if vllm.ProviderName == providerName && vllm.ModelName == modelName && vllm.Provider != nil {
			return vllm.Provider
		}
	}

	return nil
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
		"team_name":           "HelixAgent AI Debate Team",
		"total_positions":     TotalDebatePositions,
		"total_llms":          totalLLMs,
		"max_llms":            TotalDebateLLMs,
		"min_fallbacks":       MinFallbacksPerPosition,
		"max_fallbacks":       MaxFallbacksPerPosition,
		"oauth_llms":          oauthCount,
		"llmsverifier_llms":   verifierCount,
		"active_positions":    len(dtc.GetActiveMembers()),
		"positions":           positions,
		"verified_llms_count": len(dtc.verifiedLLMs),
		"sorting_method":      "score_only", // NO OAuth priority - pure score-based sorting
		"claude_models": map[string]string{
			// Claude 4.6 (Latest)
			"opus_46":   ClaudeModels.Opus46,
			"sonnet_46": ClaudeModels.Sonnet46,
			// Claude 4.5
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
			"zai":      LLMsVerifierModels.ZAI,
			"chutes":   LLMsVerifierModels.Chutes,
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
			// Primary models
			"big_pickle": ZenModels.BigPickle,
			"gpt_5_nano": ZenModels.GPT5Nano,
			"glm_47":     ZenModels.GLM47,
			// Qwen models
			"qwen3_coder": ZenModels.Qwen3Coder,
			"qwen3_235b":  ZenModels.Qwen3235B,
			"qwen3_32b":   ZenModels.Qwen332B,
			// Kimi models
			"kimi_k2":     ZenModels.KimiK2,
			"kimi_latest": ZenModels.KimiLatest,
			// Gemini models
			"gemini_3_flash":  ZenModels.Gemini3Flash,
			"gemini_25_pro":   ZenModels.Gemini25Pro,
			"gemini_20_flash": ZenModels.Gemini20Flash,
			// DeepSeek models
			"deepseek_r1":    ZenModels.DeepSeekR1,
			"deepseek_v3":    ZenModels.DeepSeekV3,
			"deepseek_coder": ZenModels.DeepSeekCoder,
			// Grok models
			"grok_code": ZenModels.GrokCode,
			"grok_3":    ZenModels.Grok3,
			"grok_2":    ZenModels.Grok2,
			// Claude models
			"claude_sonnet_4": ZenModels.ClaudeSonnet4,
			"claude_haiku_4":  ZenModels.ClaudeHaiku4,
			// Llama models
			"llama_4_maverick": ZenModels.Llama4Maverick,
			"llama_4_scout":    ZenModels.Llama4Scout,
			"llama_33_70b":     ZenModels.Llama3370B,
			// Mistral models
			"mistral_large": ZenModels.MistralLarge,
			"codestral":     ZenModels.Codestral,
			// Other models
			"o3_mini":   ZenModels.O3Mini,
			"o1_mini":   ZenModels.O1Mini,
			"gpt_4o":    ZenModels.GPT4O,
			"command_a": ZenModels.CommandA,
		},
		"zen_models_count":    dtc.countZenModels(),
		"total_zen_available": 29, // Total number of Zen models to evaluate
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
