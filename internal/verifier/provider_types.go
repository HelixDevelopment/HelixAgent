// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"time"

	"dev.helix.agent/internal/llm"
)

// ProviderAuthType defines how a provider authenticates
type ProviderAuthType string

const (
	// AuthTypeAPIKey is for providers that use API key authentication (Bearer token)
	AuthTypeAPIKey ProviderAuthType = "api_key"

	// AuthTypeOAuth is for providers that use OAuth2 authentication (Claude, Qwen)
	AuthTypeOAuth ProviderAuthType = "oauth"

	// AuthTypeFree is for providers that offer free models without authentication
	AuthTypeFree ProviderAuthType = "free"

	// AuthTypeAnonymous is for providers that work with anonymous access (device ID)
	AuthTypeAnonymous ProviderAuthType = "anonymous"

	// AuthTypeLocal is for self-hosted providers (Ollama)
	AuthTypeLocal ProviderAuthType = "local"
)

// ProviderStatus represents the health status of a provider
type ProviderStatus string

const (
	// StatusUnknown indicates the provider has not been verified yet
	StatusUnknown ProviderStatus = "unknown"

	// StatusHealthy indicates the provider is verified and working
	StatusHealthy ProviderStatus = "healthy"

	// StatusVerified indicates the provider passed verification (alias for Healthy)
	StatusVerified ProviderStatus = "verified"

	// StatusUnhealthy indicates the provider failed verification
	StatusUnhealthy ProviderStatus = "unhealthy"

	// StatusFailed indicates verification failed for this provider
	StatusFailed ProviderStatus = "failed"

	// StatusDegraded indicates partial verification success
	StatusDegraded ProviderStatus = "degraded"

	// StatusRateLimited indicates the provider is temporarily unavailable due to rate limits
	StatusRateLimited ProviderStatus = "rate_limited"

	// StatusAuthFailed indicates authentication failed for this provider
	StatusAuthFailed ProviderStatus = "auth_failed"

	// StatusUnavailable indicates the provider is not accessible
	StatusUnavailable ProviderStatus = "unavailable"
)

// UnifiedProvider represents a provider with all verification data
// This is the single source of truth for provider information
type UnifiedProvider struct {
	// Identity
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	DisplayName string           `json:"display_name,omitempty"`
	Type        string           `json:"type"`
	AuthType    ProviderAuthType `json:"auth_type"`

	// Verification Results
	Verified    bool            `json:"verified"`
	VerifiedAt  time.Time       `json:"verified_at,omitempty"`
	Score       float64         `json:"score"`
	ScoreSuffix string          `json:"score_suffix,omitempty"`
	TestResults map[string]bool `json:"test_results,omitempty"`
	CodeVisible bool            `json:"code_visible"`

	// Failure Tracking (populated when Verified == false)
	FailureReason     string               `json:"failure_reason,omitempty"`
	FailureCategory   string               `json:"failure_category,omitempty"`
	TestDetails       []ProviderTestDetail `json:"test_details,omitempty"`
	VerificationMsg   string               `json:"verification_message,omitempty"`
	LastModelResponse string               `json:"last_model_response,omitempty"`

	// Models Available
	Models       []UnifiedModel `json:"models"`
	DefaultModel string         `json:"default_model"`
	PrimaryModel *UnifiedModel  `json:"primary_model,omitempty"`

	// Health Status
	Status           ProviderStatus `json:"status"`
	LastHealthCheck  time.Time      `json:"last_health_check,omitempty"`
	HealthCheckError string         `json:"health_check_error,omitempty"`

	// OAuth-specific (only for AuthTypeOAuth)
	OAuthTokenExpiry time.Time `json:"oauth_token_expiry,omitempty"`
	OAuthAutoRefresh bool      `json:"oauth_auto_refresh,omitempty"`

	// Configuration
	BaseURL  string `json:"base_url,omitempty"`
	APIKey   string `json:"-"` // Not serialized for security
	Tier     int    `json:"tier"`
	Priority int    `json:"priority"`

	// Provider Instance (not serialized)
	Instance llm.LLMProvider `json:"-"`

	// Error tracking
	ErrorMessage     string `json:"error_message,omitempty"`
	ConsecutiveFails int    `json:"consecutive_fails,omitempty"`
	ErrorCount       int    `json:"error_count,omitempty"`

	// Health tracking (aliases for compatibility)
	LastHealthAt time.Time `json:"last_health_at,omitempty"`

	// Metadata for additional information
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Subscription & Access
	Subscription *SubscriptionInfo     `json:"subscription,omitempty"`
	AccessConfig *ProviderAccessConfig `json:"access_config,omitempty"`
}

// ProviderTestDetail contains details of a single verification test for a provider.
// It maps from the internal TestResult type to an API-friendly representation.
type ProviderTestDetail struct {
	Name       string   `json:"name"`
	Passed     bool     `json:"passed"`
	Score      float64  `json:"score"`
	Details    []string `json:"details,omitempty"`
	DurationMs int64    `json:"duration_ms"`
}

// Failure category constants for categorizeFailure
const (
	FailureCategoryCodeVisibility = "code_visibility_failed"
	FailureCategoryScoreBelow     = "score_below_threshold"
	FailureCategoryAPIError       = "api_error"
	FailureCategoryTimeout        = "timeout"
	FailureCategoryAuthError      = "auth_error"
	FailureCategoryCannedResponse = "canned_response"
	FailureCategoryEmptyResponse  = "empty_response"
)

// UnifiedModel represents a model with its verification score
type UnifiedModel struct {
	// Identity
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Provider    string `json:"provider"`

	// Verification
	Score       float64       `json:"score"`
	ScoreSuffix string        `json:"score_suffix,omitempty"`
	Verified    bool          `json:"verified"`
	VerifiedAt  time.Time     `json:"verified_at,omitempty"`
	Latency     time.Duration `json:"latency,omitempty"`

	// Capabilities (multiple representations for flexibility)
	ContextWindow     int      `json:"context_window,omitempty"`
	MaxOutputTokens   int      `json:"max_output_tokens,omitempty"`
	SupportsStreaming bool     `json:"supports_streaming,omitempty"`
	SupportsTools     bool     `json:"supports_tools,omitempty"`
	SupportsFunctions bool     `json:"supports_functions,omitempty"`
	SupportsVision    bool     `json:"supports_vision,omitempty"`
	Capabilities      []string `json:"capabilities,omitempty"` // String list of capabilities

	// Pricing (per 1M tokens)
	CostPerInputToken  float64 `json:"cost_per_input_token,omitempty"`
	CostPerOutputToken float64 `json:"cost_per_output_token,omitempty"`

	// Test Results
	TestResults map[string]bool `json:"test_results,omitempty"`

	// Metadata for additional information
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// StartupResult contains the results of the startup verification pipeline
type StartupResult struct {
	// Counts
	TotalProviders int `json:"total_providers"`
	VerifiedCount  int `json:"verified_count"`
	FailedCount    int `json:"failed_count"`
	SkippedCount   int `json:"skipped_count"`

	// Provider breakdown by auth type
	APIKeyProviders int `json:"api_key_providers"`
	OAuthProviders  int `json:"oauth_providers"`
	FreeProviders   int `json:"free_providers"`

	// Timing
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	DurationMs  int64     `json:"duration_ms"`

	// Results
	Providers       []*UnifiedProvider `json:"providers"`
	RankedProviders []*UnifiedProvider `json:"ranked_providers"`

	// Debate Team (15 LLMs)
	DebateTeam *DebateTeamResult `json:"debate_team,omitempty"`

	// Errors
	Errors []StartupError `json:"errors,omitempty"`

	// Subscription breakdown
	FreeProviderCount         int `json:"free_provider_count"`
	FreeCreditProviderCount   int `json:"free_credit_provider_count"`
	PayAsYouGoProviderCount   int `json:"pay_as_you_go_provider_count"`
	SubscriptionDetectedCount int `json:"subscription_detected_count"`
}

// DebateTeamResult contains the selected LLMs for the AI debate team (up to 25)
type DebateTeamResult struct {
	// Team composition (5 positions × (1 primary + 2-4 fallbacks))
	Positions []*DebatePosition `json:"positions"`
	TotalLLMs int               `json:"total_llms"`

	// Selection criteria
	MinScore      float64 `json:"min_score"`
	SortedByScore bool    `json:"sorted_by_score"` // Always true - NO OAuth priority
	LLMReuseCount int     `json:"llm_reuse_count"` // Number of LLMs reused across positions

	// Timing
	SelectedAt time.Time `json:"selected_at"`
}

// DebatePosition represents a position in the AI debate team
type DebatePosition struct {
	Position  int          `json:"position"`
	Role      string       `json:"role"`
	Primary   *DebateLLM   `json:"primary"`
	Fallbacks []*DebateLLM `json:"fallbacks,omitempty"` // 2-4 fallbacks per position
}

// DebateLLM represents an LLM selected for the debate team
type DebateLLM struct {
	Provider     string           `json:"provider"`
	ProviderType string           `json:"provider_type"`
	ModelID      string           `json:"model_id"`
	ModelName    string           `json:"model_name"`
	AuthType     ProviderAuthType `json:"auth_type"`
	Score        float64          `json:"score"`
	Verified     bool             `json:"verified"`
	IsOAuth      bool             `json:"is_oauth"`
}

// StartupError represents an error during startup verification
type StartupError struct {
	Provider    string `json:"provider"`
	ModelID     string `json:"model_id,omitempty"`
	Phase       string `json:"phase"`
	Error       string `json:"error"`
	Recoverable bool   `json:"recoverable"`
}

// ProviderDiscoveryResult contains information about a discovered provider
type ProviderDiscoveryResult struct {
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	AuthType    ProviderAuthType `json:"auth_type"`
	Discovered  bool             `json:"discovered"`
	Source      string           `json:"source"`                // "env", "oauth", "auto"
	Credentials string           `json:"credentials,omitempty"` // Redacted
	BaseURL     string           `json:"base_url,omitempty"`
	Models      []string         `json:"models,omitempty"`
}

// StartupConfig holds configuration for the startup verification pipeline
type StartupConfig struct {
	// Verification settings
	ParallelVerification bool          `yaml:"parallel_verification" json:"parallel_verification"`
	MaxConcurrency       int           `yaml:"max_concurrency" json:"max_concurrency"`
	VerificationTimeout  time.Duration `yaml:"verification_timeout" json:"verification_timeout"`
	HealthCheckTimeout   time.Duration `yaml:"health_check_timeout" json:"health_check_timeout"`

	// Score thresholds
	MinScore              float64 `yaml:"min_score" json:"min_score"`
	RequireCodeVisibility bool    `yaml:"require_code_visibility" json:"require_code_visibility"`

	// Debate team settings
	DebateTeamSize       int `yaml:"debate_team_size" json:"debate_team_size"`
	PositionCount        int `yaml:"position_count" json:"position_count"`
	FallbacksPerPosition int `yaml:"fallbacks_per_position" json:"fallbacks_per_position"`

	// OAuth settings
	OAuthPriorityBoost    float64 `yaml:"oauth_priority_boost" json:"oauth_priority_boost"`
	TrustOAuthOnFailure   bool    `yaml:"trust_oauth_on_failure" json:"trust_oauth_on_failure"`
	OAuthTokenRefreshMins int     `yaml:"oauth_token_refresh_mins" json:"oauth_token_refresh_mins"`

	// Free provider settings
	FreeProviderBaseScore float64 `yaml:"free_provider_base_score" json:"free_provider_base_score"`
	EnableFreeProviders   bool    `yaml:"enable_free_providers" json:"enable_free_providers"`

	// Fallback strategy
	OAuthPrimaryNonOAuthFallback bool `yaml:"oauth_primary_non_oauth_fallback" json:"oauth_primary_non_oauth_fallback"`

	// Caching
	CacheVerificationResults bool          `yaml:"cache_verification_results" json:"cache_verification_results"`
	CacheTTL                 time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
}

// DefaultStartupConfig returns sensible defaults for startup configuration
func DefaultStartupConfig() *StartupConfig {
	return &StartupConfig{
		ParallelVerification:         true,
		MaxConcurrency:               10,
		VerificationTimeout:          120 * time.Second, // 2 minutes - enough for slow providers like Zen, ZAI
		HealthCheckTimeout:           10 * time.Second,
		MinScore:                     5.0,
		RequireCodeVisibility:        true,
		DebateTeamSize:               25, // 5 positions × (1 primary + 4 fallbacks) = 25 LLMs max
		PositionCount:                5,
		FallbacksPerPosition:         4,   // 2-4 fallbacks per position
		OAuthPriorityBoost:           0.0, // NO OAuth priority - all providers sorted purely by score
		TrustOAuthOnFailure:          true,
		OAuthTokenRefreshMins:        10,
		FreeProviderBaseScore:        6.5,
		EnableFreeProviders:          true,
		OAuthPrimaryNonOAuthFallback: false, // NO special fallback treatment - use score-based selection
		CacheVerificationResults:     true,
		CacheTTL:                     6 * time.Hour,
	}
}

// ProviderTypeInfo contains metadata about a provider type
type ProviderTypeInfo struct {
	Type        string           `json:"type"`
	DisplayName string           `json:"display_name"`
	AuthType    ProviderAuthType `json:"auth_type"`
	Tier        int              `json:"tier"`
	Priority    int              `json:"priority"`
	EnvVars     []string         `json:"env_vars"`
	BaseURL     string           `json:"base_url"`
	Models      []string         `json:"models"`
	Free        bool             `json:"free"`

	// Subscription & Access (linked from ProviderAccessRegistry)
	AccessConfig   *ProviderAccessConfig `json:"access_config,omitempty"`
	DefaultSubType SubscriptionType      `json:"default_sub_type,omitempty"`
}

// SupportedProviders defines all provider types supported by the system
var SupportedProviders = map[string]*ProviderTypeInfo{
	// Tier 1: Premium (OAuth2 + API Key)
	"claude": {
		Type:        "claude",
		DisplayName: "Claude (Anthropic)",
		AuthType:    AuthTypeOAuth, // Primary auth is OAuth, fallback to API key
		Tier:        1,
		Priority:    1,
		EnvVars:     []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"},
		BaseURL:     "https://api.anthropic.com/v1/messages",
		Models:      []string{"claude-opus-4-6", "claude-sonnet-4-6", "claude-opus-4-5-20251101", "claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001"},
		Free:        false,
	},
	"qwen": {
		Type:        "qwen",
		DisplayName: "Qwen (Alibaba)",
		AuthType:    AuthTypeOAuth,
		Tier:        2,
		Priority:    3,
		EnvVars:     []string{"QWEN_API_KEY", "DASHSCOPE_API_KEY"},
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		Models:      []string{"qwen-max", "qwen-plus", "qwen-turbo", "qwen-coder-turbo"},
		Free:        false,
	},

	// Tier 2: High-quality API key providers
	"gemini": {
		Type:        "gemini",
		DisplayName: "Gemini (Google)",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    2,
		EnvVars:     []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "ApiKey_Gemini"},
		BaseURL:     "https://generativelanguage.googleapis.com/v1beta",
		Models:      []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash"},
		Free:        false,
	},
	"deepseek": {
		Type:        "deepseek",
		DisplayName: "DeepSeek",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    3,
		EnvVars:     []string{"DEEPSEEK_API_KEY", "ApiKey_DeepSeek"},
		BaseURL:     "https://api.deepseek.com/v1/chat/completions",
		Models:      []string{"deepseek-chat", "deepseek-coder", "deepseek-reasoner"},
		Free:        false,
	},
	"mistral": {
		Type:        "mistral",
		DisplayName: "Mistral AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    3,
		EnvVars:     []string{"MISTRAL_API_KEY", "ApiKey_Mistral_AiStudio"},
		BaseURL:     "https://api.mistral.ai/v1/chat/completions",
		Models:      []string{"mistral-large-latest", "mistral-medium-latest", "codestral-latest"},
		Free:        false,
	},

	// Tier 3: Fast inference
	"groq": {
		Type:        "groq",
		DisplayName: "Groq",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"GROQ_API_KEY", "ApiKey_Groq"},
		BaseURL:     "https://api.groq.com/openai/v1/chat/completions",
		Models:      []string{"llama-3.1-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768"},
		Free:        false,
	},
	"cerebras": {
		Type:        "cerebras",
		DisplayName: "Cerebras",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"CEREBRAS_API_KEY", "ApiKey_Cerebras"},
		BaseURL:     "https://api.cerebras.ai/v1/chat/completions",
		Models:      []string{"llama3.1-8b"},
		Free:        false,
	},

	// Tier 4: Aggregators
	"openrouter": {
		Type:        "openrouter",
		DisplayName: "OpenRouter",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"OPENROUTER_API_KEY", "ApiKey_OpenRouter"},
		BaseURL:     "https://openrouter.ai/api/v1/chat/completions",
		Models:      []string{}, // Many models available
		Free:        true,       // Has free tier
	},

	// Tier 5: Free providers
	"zen": {
		Type:        "zen",
		DisplayName: "OpenCode Zen",
		AuthType:    AuthTypeFree,
		Tier:        5,
		Priority:    4,
		EnvVars:     []string{"OPENCODE_API_KEY", "ApiKey_OpenCode"}, // Optional
		BaseURL:     "https://opencode.ai/zen/v1/chat/completions",
		// ALL known Zen models - each will be evaluated individually
		// Models that pass verification will be considered for AI Debate Team
		Models: []string{
			// Primary stealth/anonymous models
			"big-pickle",
			"gpt-5-nano",
			"glm-4.7",
			// Qwen models
			"qwen3-coder",
			"qwen3-235b",
			"qwen3-32b",
			// Kimi models
			"kimi-k2",
			"kimi-latest",
			// Gemini models
			"gemini-3-flash",
			"gemini-2.5-pro",
			"gemini-2.0-flash",
			// DeepSeek models
			"deepseek-r1",
			"deepseek-v3",
			"deepseek-coder",
			// Grok models
			"grok-code",
			"grok-3",
			"grok-2",
			// Claude models (if available through Zen)
			"claude-sonnet-4",
			"claude-haiku-4",
			// Llama models
			"llama-4-maverick",
			"llama-4-scout",
			"llama-3.3-70b",
			// Mistral models
			"mistral-large",
			"codestral",
			// Other models
			"o3-mini",
			"o1-mini",
			"gpt-4o",
			"command-r-plus",
		},
		Free: true,
	},

	// Tier 6: Self-hosted
	"helixllm": {
		Type:        "helixllm",
		DisplayName: "HelixLLM (Local LLM)",
		AuthType:    AuthTypeLocal,
		Tier:        6,
		Priority:    25,
		EnvVars:     []string{"HELIX_LLM_ENDPOINT", "USE_HELIX_LLM"},
		BaseURL:     "https://localhost:8443",
		Models:      []string{"model.gguf"},
		Free:        true,
	},
	"ollama": {
		Type:        "ollama",
		DisplayName: "Ollama (Local)",
		AuthType:    AuthTypeLocal,
		Tier:        6,
		Priority:    20,
		EnvVars:     []string{"OLLAMA_BASE_URL", "OLLAMA_HOST"},
		BaseURL:     "http://localhost:11434",
		Models:      []string{"llama3.2", "codellama", "mistral"},
		Free:        true,
	},

	// Tier 7: Other API key providers
	"zai": {
		Type:        "zai",
		DisplayName: "ZAI (Zhipu GLM)",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    3,
		EnvVars:     []string{"ZAI_API_KEY", "ZHIPU_API_KEY", "ApiKey_ZAI"},
		BaseURL:     "https://api.z.ai/api/paas/v4/chat/completions",
		Models:      []string{"glm-5", "glm-4.7", "glm-4.6", "glm-4.5", "glm-4.5-air"},
		Free:        false,
	},

	// New Providers (Phase 1: Enhanced LLMsVerifier Integration)

	// Tier 2: High-quality API key providers (additions)
	"grok": {
		Type:        "grok",
		DisplayName: "Grok (xAI)",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    2,
		EnvVars:     []string{"XAI_API_KEY", "GROK_API_KEY"},
		BaseURL:     "https://api.x.ai/v1/chat/completions",
		Models:      []string{"grok-2", "grok-2-mini", "grok-2-vision", "grok-3"},
		Free:        false,
	},
	"perplexity": {
		Type:        "perplexity",
		DisplayName: "Perplexity AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    2,
		EnvVars:     []string{"PERPLEXITY_API_KEY", "PPLX_API_KEY"},
		BaseURL:     "https://api.perplexity.ai/chat/completions",
		Models:      []string{"llama-3.1-sonar-small-128k-online", "llama-3.1-sonar-large-128k-online", "llama-3.1-sonar-huge-128k-online"},
		Free:        false,
	},
	"cohere": {
		Type:        "cohere",
		DisplayName: "Cohere",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    3,
		EnvVars:     []string{"COHERE_API_KEY", "CO_API_KEY"},
		BaseURL:     "https://api.cohere.com/v2/chat",
		Models:      []string{"command-a-03-2025", "command-a-reasoning-08-2025", "command-r7b-12-2024", "command-r-plus-08-2024", "command-r-08-2024"},
		Free:        false,
	},
	"ai21": {
		Type:        "ai21",
		DisplayName: "AI21 Labs",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    3,
		EnvVars:     []string{"AI21_API_KEY"},
		BaseURL:     "https://api.ai21.com/studio/v1/chat/completions",
		Models:      []string{"jamba-1.5-large", "jamba-1.5-mini", "jamba-instruct"},
		Free:        false,
	},
	"together": {
		Type:        "together",
		DisplayName: "Together AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"TOGETHER_API_KEY", "TOGETHERAI_API_KEY"},
		BaseURL:     "https://api.together.xyz/v1/chat/completions",
		Models:      []string{"meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo", "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", "mistralai/Mixtral-8x22B-Instruct-v0.1"},
		Free:        false,
	},
	"fireworks": {
		Type:        "fireworks",
		DisplayName: "Fireworks AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"FIREWORKS_API_KEY", "ApiKey_Fireworks_AI"},
		BaseURL:     "https://api.fireworks.ai/inference/v1/chat/completions",
		Models:      []string{"accounts/fireworks/models/llama-v3p3-70b-instruct", "accounts/fireworks/models/deepseek-v3", "accounts/fireworks/models/qwen2p5-72b-instruct"},
		Free:        false,
	},
	"anyscale": {
		Type:        "anyscale",
		DisplayName: "Anyscale",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"ANYSCALE_API_KEY"},
		BaseURL:     "https://api.endpoints.anyscale.com/v1/chat/completions",
		Models:      []string{"meta-llama/Llama-3-70b-chat-hf", "mistralai/Mixtral-8x7B-Instruct-v0.1"},
		Free:        false,
	},
	"deepinfra": {
		Type:        "deepinfra",
		DisplayName: "DeepInfra",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"DEEPINFRA_API_KEY"},
		BaseURL:     "https://api.deepinfra.com/v1/openai/chat/completions",
		Models:      []string{"meta-llama/Meta-Llama-3.1-405B-Instruct", "meta-llama/Meta-Llama-3.1-70B-Instruct"},
		Free:        false,
	},
	"lepton": {
		Type:        "lepton",
		DisplayName: "Lepton AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"LEPTON_API_KEY"},
		BaseURL:     "https://llama3-1-405b.lepton.run/api/v1/chat/completions",
		Models:      []string{"llama3.1-405b", "llama3.1-70b"},
		Free:        false,
	},
	"sambanova": {
		Type:        "sambanova",
		DisplayName: "SambaNova",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"SAMBANOVA_API_KEY", "ApiKey_SambaNova_AI"},
		BaseURL:     "https://api.sambanova.ai/v1/chat/completions",
		Models:      []string{"Meta-Llama-3.1-405B-Instruct", "Meta-Llama-3.1-70B-Instruct"},
		Free:        false,
	},

	// Additional providers from .env (Phase 2: Complete Provider Coverage)
	"huggingface": {
		Type:        "huggingface",
		DisplayName: "HuggingFace",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    5,
		EnvVars:     []string{"HUGGINGFACE_API_KEY", "HF_API_KEY", "ApiKey_HuggingFace"},
		BaseURL:     "https://router.huggingface.co/v1/chat/completions",
		Models:      []string{"meta-llama/Llama-3.3-70B-Instruct", "mistralai/Mistral-7B-Instruct-v0.3"},
		Free:        false,
	},
	"nvidia": {
		Type:        "nvidia",
		DisplayName: "NVIDIA NIM",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    3,
		EnvVars:     []string{"NVIDIA_API_KEY", "NVIDIA_NIM_API_KEY", "ApiKey_Nvidia"},
		BaseURL:     "https://integrate.api.nvidia.com/v1/chat/completions",
		Models:      []string{"meta/llama-3.1-405b-instruct", "meta/llama-3.1-70b-instruct", "meta/llama-3.3-70b-instruct"},
		Free:        false,
	},
	"chutes": {
		Type:        "chutes",
		DisplayName: "Chutes AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"CHUTES_API_KEY", "ApiKey_Chutes"},
		BaseURL:     "https://llm.chutes.ai/v1/chat/completions",
		// Primary models used as Tier-3 hardcoded fallback when the
		// 3-tier dynamic discovery can't reach Chutes /v1/models.
		// Keep this list in sync with the current Chutes catalogue —
		// stale IDs cause verification to fail even with a valid key
		// (the probe asks for a model that no longer exists).
		// Last refreshed: 2026-04-21 from live https://llm.chutes.ai/v1/models.
		Models: []string{"Qwen/Qwen3-32B-TEE", "moonshotai/Kimi-K2.5-TEE", "deepseek-ai/DeepSeek-V3.2-Exp"},
		Free:   false,
	},
	"siliconflow": {
		Type:        "siliconflow",
		DisplayName: "SiliconFlow",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"SILICONFLOW_API_KEY", "ApiKey_SiliconFlow"},
		BaseURL:     "https://api.siliconflow.cn/v1/chat/completions",
		Models:      []string{"deepseek-ai/DeepSeek-V3", "Qwen/Qwen2.5-72B-Instruct"},
		Free:        false,
	},
	"kimi": {
		Type:        "kimi",
		DisplayName: "Kimi (Moonshot)",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    3,
		EnvVars:     []string{"KIMI_API_KEY", "MOONSHOT_API_KEY", "ApiKey_Kimi"},
		BaseURL:     "https://api.moonshot.cn/v1/chat/completions",
		Models:      []string{"moonshot-v1-128k", "moonshot-v1-32k", "moonshot-v1-8k"},
		Free:        false,
	},
	"vercel": {
		Type:        "vercel",
		DisplayName: "Vercel AI Gateway",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"VERCEL_API_KEY", "ApiKey_Vercel_Ai_Gateway"},
		BaseURL:     "https://api.vercel.com/v1/chat/completions",
		Models:      []string{},
		Free:        false,
	},
	"cloudflare": {
		Type:        "cloudflare",
		DisplayName: "Cloudflare Workers AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"CLOUDFLARE_API_KEY", "CF_API_KEY", "ApiKey_Cloudflare_Workers_AI"},
		BaseURL:     "https://api.cloudflare.com/client/v4/accounts/YOUR_ACCOUNT/ai/v1/chat/completions",
		Models: []string{
			"@cf/meta/llama-4-scout-17b-16e-instruct",
			"@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			"@cf/meta/llama-3.1-70b-instruct",
			"@cf/meta/llama-3.1-8b-instruct-fast",
			"@cf/meta/llama-3.1-8b-instruct",
			"@cf/nvidia/nemotron-3-120b-a12b",
			"@cf/openai/gpt-oss-120b",
			"@cf/mistral/mistral-small-3.1-24b-instruct",
			"@cf/google/gemma-3-12b-it",
			"@cf/qwen/qwq-32b",
			"@cf/qwen/qwen2.5-coder-32b-instruct",
			"@cf/deepseek/deepseek-r1-distill-qwen-32b",
			"@cf/zai/glm-4.7-flash",
		},
		Free: false,
	},
	"baseten": {
		Type:        "baseten",
		DisplayName: "Baseten",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"BASETEN_API_KEY", "ApiKey_Baseten"},
		BaseURL:     "https://inference.baseten.co/v1/chat/completions",
		Models:      []string{},
		Free:        false,
	},
	"novita": {
		Type:        "novita",
		DisplayName: "Novita AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"NOVITA_API_KEY", "ApiKey_Novita_AI"},
		BaseURL:     "https://api.novita.ai/v3/openai/chat/completions",
		Models:      []string{"meta-llama/llama-3.1-405b-instruct", "mistralai/mistral-large-instruct-2407"},
		Free:        false,
	},
	"upstage": {
		Type:        "upstage",
		DisplayName: "Upstage AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"UPSTAGE_API_KEY", "ApiKey_Upstage_AI"},
		BaseURL:     "https://api.upstage.ai/v1/solar/chat/completions",
		Models:      []string{"solar-pro", "solar-mini"},
		Free:        false,
	},
	"nlpcloud": {
		Type:        "nlpcloud",
		DisplayName: "NLP Cloud",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"NLP_API_KEY", "NLPCLOUD_API_KEY", "ApiKey_NLP_Cloud"},
		BaseURL:     "https://api.nlpcloud.io/v1/chat/completions",
		Models:      []string{},
		Free:        false,
	},
	"modal": {
		Type:        "modal",
		DisplayName: "Modal",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"MODAL_API_KEY", "MODAL_TOKEN_SECRET", "ApiKey_Modal_Token_Secret"},
		BaseURL:     "https://api.modal.com/v1/chat/completions",
		Models:      []string{},
		Free:        false,
	},
	"inference": {
		Type:        "inference",
		DisplayName: "Inference.net",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"INFERENCE_API_KEY", "ApiKey_Inference"},
		BaseURL:     "https://api.inference.net/v1/chat/completions",
		Models:      []string{"google/gemma-3-27b-instruct/bf-16"},
		Free:        false,
	},
	"hyperbolic": {
		Type:        "hyperbolic",
		DisplayName: "Hyperbolic",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"HYPERBOLIC_API_KEY", "ApiKey_Hyperbolic"},
		BaseURL:     "https://api.hyperbolic.xyz/v1/chat/completions",
		Models:      []string{"meta-llama/Meta-Llama-3.1-70B-Instruct"},
		Free:        false,
	},
	"replicate": {
		Type:        "replicate",
		DisplayName: "Replicate",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    5,
		EnvVars:     []string{"REPLICATE_API_KEY", "REPLICATE_API_TOKEN", "ApiKey_Replicate"},
		BaseURL:     "https://api.replicate.com/v1/predictions",
		Models:      []string{"meta/meta-llama-3-70b-instruct", "mistralai/mixtral-8x7b-instruct-v0.1"},
		Free:        false,
	},
	"sarvam": {
		Type:        "sarvam",
		DisplayName: "Sarvam AI (India)",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    6,
		EnvVars:     []string{"SARVAM_API_KEY", "ApiKey_Sarvam_AI_India"},
		BaseURL:     "https://api.sarvam.ai/v1/chat/completions",
		Models:      []string{},
		Free:        false,
	},
	"vulavula": {
		Type:        "vulavula",
		DisplayName: "Vulavula (Africa)",
		AuthType:    AuthTypeAPIKey,
		Tier:        4,
		Priority:    6,
		EnvVars:     []string{"VULAVULA_API_KEY", "ApiKey_Vulavula"},
		BaseURL:     "https://api.vulavula.com/v1/chat/completions",
		Models:      []string{},
		Free:        false,
	},
	"codestral": {
		Type:        "codestral",
		DisplayName: "Codestral (Mistral)",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    2,
		EnvVars:     []string{"CODESTRAL_API_KEY", "ApiKey_Codestral"},
		BaseURL:     "https://codestral.mistral.ai/v1/chat/completions",
		Models:      []string{"codestral-latest", "codestral-mamba-latest"},
		Free:        false,
	},

	// GitHub Models - GitHub's model inference API with generous free tier
	"github-models": {
		Type:        "github-models",
		DisplayName: "GitHub Models",
		AuthType:    AuthTypeAPIKey,
		Tier:        1,
		Priority:    2,
		EnvVars:     []string{"GITHUB_MODELS_API_KEY", "GITHUB_TOKEN"},
		BaseURL:     "https://models.github.ai/inference/chat/completions",
		Models:      []string{"openai/gpt-5", "openai/gpt-4.1", "openai/gpt-4o", "DeepSeek/DeepSeek-V3-0324", "Meta/Llama-4-Scout-17B-16E-Instruct", "Microsoft/Phi-4-reasoning"},
		Free:        false,
	},

	// Venice AI - Privacy-focused OpenAI-compatible inference
	"venice": {
		Type:        "venice",
		DisplayName: "Venice AI",
		AuthType:    AuthTypeAPIKey,
		Tier:        3,
		Priority:    4,
		EnvVars:     []string{"VENICE_API_KEY", "ApiKey_Venice"},
		BaseURL:     "https://api.venice.ai/api/v1/chat/completions",
		Models:      []string{"llama-3.3-70b", "llama-3.1-405b", "deepseek-r1-671b", "venice-uncensored", "qwen3-vl-235b-a22b", "qwen-2.5-vl", "zai-org-glm-4.7"},
		Free:        false,
	},

	// Junie (JetBrains) - CLI agent with BYOK support for multiple LLM providers
	"junie": {
		Type:        "junie",
		DisplayName: "Junie (JetBrains)",
		AuthType:    AuthTypeAPIKey,
		Tier:        2,
		Priority:    2,
		EnvVars:     []string{"JUNIE_API_KEY", "JUNIE_ANTHROPIC_API_KEY", "JUNIE_OPENAI_API_KEY", "JUNIE_GOOGLE_API_KEY", "JUNIE_GROK_API_KEY", "JUNIE_OPENROUTER_API_KEY"},
		BaseURL:     "",
		Models: []string{
			"sonnet", "opus", "gpt", "gpt-codex", "gemini-pro", "gemini-flash", "grok", "Default",
		},
		Free: false,
	},
}

// GetProviderInfo returns info for a provider type
func GetProviderInfo(providerType string) (*ProviderTypeInfo, bool) {
	info, ok := SupportedProviders[providerType]
	return info, ok
}

// IsOAuthProvider checks if a provider type uses OAuth authentication
func IsOAuthProvider(providerType string) bool {
	info, ok := SupportedProviders[providerType]
	if !ok {
		return false
	}
	return info.AuthType == AuthTypeOAuth
}

// IsFreeProvider checks if a provider type is free
func IsFreeProvider(providerType string) bool {
	info, ok := SupportedProviders[providerType]
	if !ok {
		return false
	}
	return info.Free
}

// GetProvidersByAuthType returns all providers of a specific auth type
func GetProvidersByAuthType(authType ProviderAuthType) []*ProviderTypeInfo {
	var result []*ProviderTypeInfo
	for _, info := range SupportedProviders {
		if info.AuthType == authType {
			result = append(result, info)
		}
	}
	return result
}

// GetProvidersByTier returns all providers in a specific tier
func GetProvidersByTier(tier int) []*ProviderTypeInfo {
	var result []*ProviderTypeInfo
	for _, info := range SupportedProviders {
		if info.Tier == tier {
			result = append(result, info)
		}
	}
	return result
}
