package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/llm/providers/ai21"
	"dev.helix.agent/internal/llm/providers/anthropic"
	"dev.helix.agent/internal/llm/providers/cerebras"
	"dev.helix.agent/internal/llm/providers/chutes"
	"dev.helix.agent/internal/llm/providers/claude"
	"dev.helix.agent/internal/llm/providers/cohere"
	"dev.helix.agent/internal/llm/providers/deepseek"
	"dev.helix.agent/internal/llm/providers/fireworks"
	"dev.helix.agent/internal/llm/providers/gemini"
	"dev.helix.agent/internal/llm/providers/githubmodels"
	"dev.helix.agent/internal/llm/providers/groq"
	"dev.helix.agent/internal/llm/providers/huggingface"
	"dev.helix.agent/internal/llm/providers/hyperbolic"
	"dev.helix.agent/internal/llm/providers/junie"
	"dev.helix.agent/internal/llm/providers/kimi"
	"dev.helix.agent/internal/llm/providers/mistral"
	"dev.helix.agent/internal/llm/providers/novita"
	"dev.helix.agent/internal/llm/providers/nvidia"
	"dev.helix.agent/internal/llm/providers/ollama"
	"dev.helix.agent/internal/llm/providers/openai"
	"dev.helix.agent/internal/llm/providers/openrouter"
	"dev.helix.agent/internal/llm/providers/perplexity"
	"dev.helix.agent/internal/llm/providers/qwen"
	"dev.helix.agent/internal/llm/providers/replicate"
	"dev.helix.agent/internal/llm/providers/sambanova"
	"dev.helix.agent/internal/llm/providers/siliconflow"
	"dev.helix.agent/internal/llm/providers/together"
	"dev.helix.agent/internal/llm/providers/upstage"
	"dev.helix.agent/internal/llm/providers/venice"
	"dev.helix.agent/internal/llm/providers/xai"
	"dev.helix.agent/internal/llm/providers/zai"
	"dev.helix.agent/internal/llm/providers/zen"
	"dev.helix.agent/internal/models"
	"github.com/sirupsen/logrus"
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

// ProviderDiscovery handles automatic detection of LLM providers from environment variables.
//
// Concurrent-safe by construction: providers and scores are safe.Store.
// configMu (Pattern Zeta, renamed from mu) serialises verifierScores /
// useDynamicScoring scalar fields; it no longer pairs with any bare
// map/slice field.
type ProviderDiscovery struct {
	providers         *safe.Store[string, *DiscoveredProvider]
	scores            *safe.Store[string, *ProviderScore]
	configMu          sync.RWMutex
	log               *logrus.Logger
	verifyOnStartup   bool
	verifierScores    LLMsVerifierScoreProvider // Dynamic LLMsVerifier score provider
	useDynamicScoring bool                      // Use LLMsVerifier scores instead of hardcoded
}

// VerifiedModel represents a model that has passed verification testing
type VerifiedModel struct {
	Name     string  `json:"name"`
	Score    float64 `json:"score"`
	Verified bool    `json:"verified"`
}

// DiscoveredProvider represents a provider discovered from environment.
//
// CONST-029: SupportsModels and VerifiedModels are now *safe.Slice;
// mu retained as Pattern-Zeta to preserve the joint invariant on the
// scalar cluster (Status, Verified, VerifiedAt, Error, Score) — these
// fields must transition atomically (e.g., Verified=true must never
// be observed with Status=Unhealthy). Custom MarshalJSON snapshots
// the safe.Slices to preserve the JSON wire format 1:1.
type DiscoveredProvider struct {
	mu           sync.RWMutex                 // Protects Status, Verified, VerifiedAt, Error, Score fields (Pattern-Zeta: joint invariant)
	Name         string                       `json:"name"`
	Type         string                       `json:"type"`
	APIKeyEnvVar string                       `json:"api_key_env_var"`
	APIKey       string                       `json:"-"` // Hidden in JSON
	BaseURL      string                       `json:"base_url"`
	DefaultModel string                       `json:"default_model"`
	Provider     llm.LLMProvider              `json:"-"`
	Status       ProviderHealthStatus         `json:"status"`
	Score        float64                      `json:"score"`
	Verified     bool                         `json:"verified"`
	VerifiedAt   time.Time                    `json:"verified_at,omitempty"`
	Error        string                       `json:"error,omitempty"`
	Capabilities *models.ProviderCapabilities `json:"capabilities,omitempty"`

	// SupportsModels lists model IDs the provider advertises support for.
	// Migrated to *safe.Slice (CONST-029); JSON wire format preserved via
	// MarshalJSON snapshot.
	SupportsModels *safe.Slice[string] `json:"-"`
	// VerifiedModels contains models that passed individual verification testing.
	// This is used for free providers (Zen, OpenRouter free tier) where each model
	// is tested individually for proper functionality (not canned error responses).
	// Migrated to *safe.Slice (CONST-029); JSON wire format preserved via
	// MarshalJSON snapshot.
	VerifiedModels *safe.Slice[VerifiedModel] `json:"-"`
}

// MarshalJSON preserves the historical wire format (plain []string for
// SupportsModels, plain []VerifiedModel for VerifiedModels) after the
// CONST-029 migration to *safe.Slice.
func (d *DiscoveredProvider) MarshalJSON() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var supports []string
	if d.SupportsModels != nil {
		supports = d.SupportsModels.Snapshot()
	}
	var verified []VerifiedModel
	if d.VerifiedModels != nil {
		verified = d.VerifiedModels.Snapshot()
	}

	return json.Marshal(&struct {
		Name           string                       `json:"name"`
		Type           string                       `json:"type"`
		APIKeyEnvVar   string                       `json:"api_key_env_var"`
		BaseURL        string                       `json:"base_url"`
		DefaultModel   string                       `json:"default_model"`
		Status         ProviderHealthStatus         `json:"status"`
		Score          float64                      `json:"score"`
		Verified       bool                         `json:"verified"`
		VerifiedAt     time.Time                    `json:"verified_at,omitempty"`
		Error          string                       `json:"error,omitempty"`
		Capabilities   *models.ProviderCapabilities `json:"capabilities,omitempty"`
		SupportsModels []string                     `json:"supported_models,omitempty"`
		VerifiedModels []VerifiedModel              `json:"verified_models,omitempty"`
	}{
		Name:           d.Name,
		Type:           d.Type,
		APIKeyEnvVar:   d.APIKeyEnvVar,
		BaseURL:        d.BaseURL,
		DefaultModel:   d.DefaultModel,
		Status:         d.Status,
		Score:          d.Score,
		Verified:       d.Verified,
		VerifiedAt:     d.VerifiedAt,
		Error:          d.Error,
		Capabilities:   d.Capabilities,
		SupportsModels: supports,
		VerifiedModels: verified,
	})
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
// Updated 2026-01-28: Comprehensive mapping with all alternative key names for maximum compatibility
var providerMappings = []ProviderMapping{
	// Tier 1: Premium providers (direct API access)
	// Claude/Anthropic - Multiple key name variations (updated to Claude 4.6)
	{EnvVar: "ANTHROPIC_API_KEY", ProviderType: "claude", ProviderName: "claude", BaseURL: "https://api.anthropic.com/v1/messages", DefaultModel: "claude-sonnet-4-6", Priority: 1},
	{EnvVar: "CLAUDE_API_KEY", ProviderType: "claude", ProviderName: "claude", BaseURL: "https://api.anthropic.com/v1/messages", DefaultModel: "claude-sonnet-4-6", Priority: 1},
	{EnvVar: "ApiKey_Claude", ProviderType: "claude", ProviderName: "claude", BaseURL: "https://api.anthropic.com/v1/messages", DefaultModel: "claude-sonnet-4-6", Priority: 1},
	{EnvVar: "ApiKey_Anthropic", ProviderType: "claude", ProviderName: "claude", BaseURL: "https://api.anthropic.com/v1/messages", DefaultModel: "claude-sonnet-4-6", Priority: 1},

	// OpenAI - Multiple key name variations
	{EnvVar: "OPENAI_API_KEY", ProviderType: "openai", ProviderName: "openai", BaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o", Priority: 1},
	{EnvVar: "ApiKey_OpenAI", ProviderType: "openai", ProviderName: "openai", BaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o", Priority: 1},
	{EnvVar: "OPENAI_KEY", ProviderType: "openai", ProviderName: "openai", BaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o", Priority: 1},

	// Gemini/Google - Multiple key name variations
	{EnvVar: "GEMINI_API_KEY", ProviderType: "gemini", ProviderName: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta", DefaultModel: "gemini-2.0-flash", Priority: 2},
	{EnvVar: "GOOGLE_API_KEY", ProviderType: "gemini", ProviderName: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta", DefaultModel: "gemini-2.0-flash", Priority: 2},
	{EnvVar: "ApiKey_Gemini", ProviderType: "gemini", ProviderName: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta", DefaultModel: "gemini-2.0-flash", Priority: 2},
	{EnvVar: "GOOGLE_AI_API_KEY", ProviderType: "gemini", ProviderName: "gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta", DefaultModel: "gemini-2.0-flash", Priority: 2},

	// Tier 2: High-quality specialized providers
	// DeepSeek - Multiple key name variations
	{EnvVar: "DEEPSEEK_API_KEY", ProviderType: "deepseek", ProviderName: "deepseek", BaseURL: "https://api.deepseek.com/v1/chat/completions", DefaultModel: "deepseek-chat", Priority: 3},
	{EnvVar: "ApiKey_DeepSeek", ProviderType: "deepseek", ProviderName: "deepseek", BaseURL: "https://api.deepseek.com/v1/chat/completions", DefaultModel: "deepseek-chat", Priority: 3},
	{EnvVar: "DEEPSEEK_KEY", ProviderType: "deepseek", ProviderName: "deepseek", BaseURL: "https://api.deepseek.com/v1/chat/completions", DefaultModel: "deepseek-chat", Priority: 3},

	// Mistral - Multiple key name variations
	{EnvVar: "MISTRAL_API_KEY", ProviderType: "mistral", ProviderName: "mistral", BaseURL: "https://api.mistral.ai/v1", DefaultModel: "mistral-large-latest", Priority: 3},
	{EnvVar: "ApiKey_Mistral", ProviderType: "mistral", ProviderName: "mistral", BaseURL: "https://api.mistral.ai/v1", DefaultModel: "mistral-large-latest", Priority: 3},
	{EnvVar: "CODESTRAL_API_KEY", ProviderType: "mistral", ProviderName: "codestral", BaseURL: "https://codestral.mistral.ai/v1", DefaultModel: "codestral-latest", Priority: 3},

	// Qwen/DashScope - Multiple key name variations
	{EnvVar: "QWEN_API_KEY", ProviderType: "qwen", ProviderName: "qwen", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", DefaultModel: "qwen-max", Priority: 4},
	{EnvVar: "ApiKey_Qwen", ProviderType: "qwen", ProviderName: "qwen", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", DefaultModel: "qwen-max", Priority: 4},
	{EnvVar: "DASHSCOPE_API_KEY", ProviderType: "qwen", ProviderName: "qwen", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", DefaultModel: "qwen-max", Priority: 4},
	{EnvVar: "ALIBABA_API_KEY", ProviderType: "qwen", ProviderName: "qwen", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", DefaultModel: "qwen-max", Priority: 4},

	// xAI/Grok - Multiple key name variations
	{EnvVar: "XAI_API_KEY", ProviderType: "xai", ProviderName: "xai", BaseURL: "https://api.x.ai/v1", DefaultModel: "grok-2-latest", Priority: 3},
	{EnvVar: "GROK_API_KEY", ProviderType: "xai", ProviderName: "xai", BaseURL: "https://api.x.ai/v1", DefaultModel: "grok-2-latest", Priority: 3},
	{EnvVar: "ApiKey_XAI", ProviderType: "xai", ProviderName: "xai", BaseURL: "https://api.x.ai/v1", DefaultModel: "grok-2-latest", Priority: 3},
	{EnvVar: "ApiKey_Grok", ProviderType: "xai", ProviderName: "xai", BaseURL: "https://api.x.ai/v1", DefaultModel: "grok-2-latest", Priority: 3},

	// ZAI/Zhipu/GLM - Multiple key name variations (GLM-4.7 is latest model)
	// Tier 2 providers have Priority 3
	{EnvVar: "ZAI_API_KEY", ProviderType: "zai", ProviderName: "zai", BaseURL: "https://api.z.ai/api/paas/v4", DefaultModel: "glm-4.7", Priority: 3},
	{EnvVar: "ApiKey_ZAI", ProviderType: "zai", ProviderName: "zai", BaseURL: "https://api.z.ai/api/paas/v4", DefaultModel: "glm-4.7", Priority: 3},
	{EnvVar: "ZHIPU_API_KEY", ProviderType: "zai", ProviderName: "zai", BaseURL: "https://api.z.ai/api/paas/v4", DefaultModel: "glm-4.7", Priority: 3},
	{EnvVar: "GLM_API_KEY", ProviderType: "zai", ProviderName: "zai", BaseURL: "https://api.z.ai/api/paas/v4", DefaultModel: "glm-4.7", Priority: 3},
	{EnvVar: "BIGMODEL_API_KEY", ProviderType: "zai", ProviderName: "zai", BaseURL: "https://api.z.ai/api/paas/v4", DefaultModel: "glm-4.7", Priority: 3},

	// GitHub Models - GitHub's model inference API
	{EnvVar: "GITHUB_MODELS_API_KEY", ProviderType: "github-models", ProviderName: "github-models", BaseURL: "https://models.github.ai/inference/chat/completions", DefaultModel: "openai/gpt-4.1", Priority: 3},
	{EnvVar: "GITHUB_TOKEN", ProviderType: "github-models", ProviderName: "github-models", BaseURL: "https://models.github.ai/inference/chat/completions", DefaultModel: "openai/gpt-4.1", Priority: 8},

	// Cohere - Multiple key name variations
	{EnvVar: "COHERE_API_KEY", ProviderType: "cohere", ProviderName: "cohere", BaseURL: "https://api.cohere.com/v2", DefaultModel: "command-a-03-2025", Priority: 4},
	{EnvVar: "CO_API_KEY", ProviderType: "cohere", ProviderName: "cohere", BaseURL: "https://api.cohere.com/v2", DefaultModel: "command-a-03-2025", Priority: 4},
	{EnvVar: "ApiKey_Cohere", ProviderType: "cohere", ProviderName: "cohere", BaseURL: "https://api.cohere.com/v2", DefaultModel: "command-a-03-2025", Priority: 4},

	// Perplexity - Multiple key name variations
	{EnvVar: "PERPLEXITY_API_KEY", ProviderType: "perplexity", ProviderName: "perplexity", BaseURL: "https://api.perplexity.ai", DefaultModel: "llama-3.1-sonar-large-128k-online", Priority: 4},
	{EnvVar: "PPLX_API_KEY", ProviderType: "perplexity", ProviderName: "perplexity", BaseURL: "https://api.perplexity.ai", DefaultModel: "llama-3.1-sonar-large-128k-online", Priority: 4},
	{EnvVar: "ApiKey_Perplexity", ProviderType: "perplexity", ProviderName: "perplexity", BaseURL: "https://api.perplexity.ai", DefaultModel: "llama-3.1-sonar-large-128k-online", Priority: 4},

	// AI21 - Multiple key name variations
	{EnvVar: "AI21_API_KEY", ProviderType: "ai21", ProviderName: "ai21", BaseURL: "https://api.ai21.com/studio/v1", DefaultModel: "jamba-1.5-large", Priority: 5},
	{EnvVar: "ApiKey_AI21", ProviderType: "ai21", ProviderName: "ai21", BaseURL: "https://api.ai21.com/studio/v1", DefaultModel: "jamba-1.5-large", Priority: 5},

	// Tier 3: Fast inference providers
	// Groq - Multiple key name variations
	{EnvVar: "GROQ_API_KEY", ProviderType: "groq", ProviderName: "groq", BaseURL: "https://api.groq.com/openai/v1/chat/completions", DefaultModel: "llama-3.1-70b-versatile", Priority: 5},
	{EnvVar: "ApiKey_Groq", ProviderType: "groq", ProviderName: "groq", BaseURL: "https://api.groq.com/openai/v1/chat/completions", DefaultModel: "llama-3.1-70b-versatile", Priority: 5},

	// Cerebras - Multiple key name variations
	{EnvVar: "CEREBRAS_API_KEY", ProviderType: "cerebras", ProviderName: "cerebras", BaseURL: "https://api.cerebras.ai/v1/chat/completions", DefaultModel: "llama3.1-8b", Priority: 5},
	{EnvVar: "ApiKey_Cerebras", ProviderType: "cerebras", ProviderName: "cerebras", BaseURL: "https://api.cerebras.ai/v1/chat/completions", DefaultModel: "llama3.1-8b", Priority: 5},

	// SambaNova
	{EnvVar: "SAMBANOVA_API_KEY", ProviderType: "sambanova", ProviderName: "sambanova", BaseURL: "https://api.sambanova.ai/v1", DefaultModel: "Meta-Llama-3.1-70B-Instruct", Priority: 5},
	{EnvVar: "ApiKey_SambaNova", ProviderType: "sambanova", ProviderName: "sambanova", BaseURL: "https://api.sambanova.ai/v1", DefaultModel: "Meta-Llama-3.1-70B-Instruct", Priority: 5},

	// Tier 4: Alternative providers
	// Fireworks - Multiple key name variations
	{EnvVar: "FIREWORKS_API_KEY", ProviderType: "fireworks", ProviderName: "fireworks", BaseURL: "https://api.fireworks.ai/inference/v1/chat/completions", DefaultModel: "accounts/fireworks/models/llama-v3p1-70b-instruct", Priority: 6},
	{EnvVar: "ApiKey_Fireworks", ProviderType: "fireworks", ProviderName: "fireworks", BaseURL: "https://api.fireworks.ai/inference/v1/chat/completions", DefaultModel: "accounts/fireworks/models/llama-v3p1-70b-instruct", Priority: 6},

	// Together - Multiple key name variations
	{EnvVar: "TOGETHERAI_API_KEY", ProviderType: "together", ProviderName: "together", BaseURL: "https://api.together.xyz/v1/chat/completions", DefaultModel: "meta-llama/Llama-3.2-90B-Vision-Instruct-Turbo", Priority: 6},
	{EnvVar: "TOGETHER_API_KEY", ProviderType: "together", ProviderName: "together", BaseURL: "https://api.together.xyz/v1/chat/completions", DefaultModel: "meta-llama/Llama-3.2-90B-Vision-Instruct-Turbo", Priority: 6},
	{EnvVar: "ApiKey_Together", ProviderType: "together", ProviderName: "together", BaseURL: "https://api.together.xyz/v1/chat/completions", DefaultModel: "meta-llama/Llama-3.2-90B-Vision-Instruct-Turbo", Priority: 6},

	// Hyperbolic
	{EnvVar: "HYPERBOLIC_API_KEY", ProviderType: "hyperbolic", ProviderName: "hyperbolic", BaseURL: "https://api.hyperbolic.xyz/v1", DefaultModel: "meta-llama/Llama-3.3-70B-Instruct", Priority: 6},
	{EnvVar: "ApiKey_Hyperbolic", ProviderType: "hyperbolic", ProviderName: "hyperbolic", BaseURL: "https://api.hyperbolic.xyz/v1", DefaultModel: "meta-llama/Llama-3.3-70B-Instruct", Priority: 6},

	// Tier 5: Specialized providers
	// Replicate - Multiple key name variations
	{EnvVar: "REPLICATE_API_KEY", ProviderType: "replicate", ProviderName: "replicate", BaseURL: "https://api.replicate.com/v1", DefaultModel: "meta/llama-2-70b-chat", Priority: 7},
	{EnvVar: "REPLICATE_API_TOKEN", ProviderType: "replicate", ProviderName: "replicate", BaseURL: "https://api.replicate.com/v1", DefaultModel: "meta/llama-2-70b-chat", Priority: 7},
	{EnvVar: "ApiKey_Replicate", ProviderType: "replicate", ProviderName: "replicate", BaseURL: "https://api.replicate.com/v1", DefaultModel: "meta/llama-2-70b-chat", Priority: 7},

	// SiliconFlow
	{EnvVar: "SILICONFLOW_API_KEY", ProviderType: "siliconflow", ProviderName: "siliconflow", BaseURL: "https://api.siliconflow.cn/v1", DefaultModel: "Qwen/Qwen2.5-72B-Instruct", Priority: 7},
	{EnvVar: "ApiKey_SiliconFlow", ProviderType: "siliconflow", ProviderName: "siliconflow", BaseURL: "https://api.siliconflow.cn/v1", DefaultModel: "Qwen/Qwen2.5-72B-Instruct", Priority: 7},

	// Cloudflare
	{EnvVar: "CLOUDFLARE_API_KEY", ProviderType: "cloudflare", ProviderName: "cloudflare", BaseURL: "https://api.cloudflare.com/client/v4", DefaultModel: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", Priority: 7},
	{EnvVar: "CF_API_KEY", ProviderType: "cloudflare", ProviderName: "cloudflare", BaseURL: "https://api.cloudflare.com/client/v4", DefaultModel: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", Priority: 7},

	// NVIDIA
	{EnvVar: "NVIDIA_API_KEY", ProviderType: "nvidia", ProviderName: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1", DefaultModel: "meta/llama-3.1-70b-instruct", Priority: 7},
	{EnvVar: "NGC_API_KEY", ProviderType: "nvidia", ProviderName: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1", DefaultModel: "meta/llama-3.1-70b-instruct", Priority: 7},
	{EnvVar: "ApiKey_NVIDIA", ProviderType: "nvidia", ProviderName: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1", DefaultModel: "meta/llama-3.1-70b-instruct", Priority: 7},

	// Tier 6: Regional/specialized providers
	// Kimi/Moonshot
	{EnvVar: "KIMI_API_KEY", ProviderType: "kimi", ProviderName: "kimi", BaseURL: "https://api.moonshot.cn/v1", DefaultModel: "moonshot-v1-128k", Priority: 8},
	{EnvVar: "MOONSHOT_API_KEY", ProviderType: "kimi", ProviderName: "kimi", BaseURL: "https://api.moonshot.cn/v1", DefaultModel: "moonshot-v1-128k", Priority: 8},
	{EnvVar: "ApiKey_Kimi", ProviderType: "kimi", ProviderName: "kimi", BaseURL: "https://api.moonshot.cn/v1", DefaultModel: "moonshot-v1-128k", Priority: 8},

	// HuggingFace - Multiple key name variations
	{EnvVar: "HUGGINGFACE_API_KEY", ProviderType: "huggingface", ProviderName: "huggingface", BaseURL: "https://api-inference.huggingface.co", DefaultModel: "meta-llama/Llama-3.2-3B-Instruct", Priority: 8},
	{EnvVar: "HF_API_KEY", ProviderType: "huggingface", ProviderName: "huggingface", BaseURL: "https://api-inference.huggingface.co", DefaultModel: "meta-llama/Llama-3.2-3B-Instruct", Priority: 8},
	{EnvVar: "HF_TOKEN", ProviderType: "huggingface", ProviderName: "huggingface", BaseURL: "https://api-inference.huggingface.co", DefaultModel: "meta-llama/Llama-3.2-3B-Instruct", Priority: 8},
	{EnvVar: "HUGGINGFACE_TOKEN", ProviderType: "huggingface", ProviderName: "huggingface", BaseURL: "https://api-inference.huggingface.co", DefaultModel: "meta-llama/Llama-3.2-3B-Instruct", Priority: 8},
	{EnvVar: "ApiKey_HuggingFace", ProviderType: "huggingface", ProviderName: "huggingface", BaseURL: "https://api-inference.huggingface.co", DefaultModel: "meta-llama/Llama-3.2-3B-Instruct", Priority: 8},

	// Novita
	{EnvVar: "NOVITA_API_KEY", ProviderType: "novita", ProviderName: "novita", BaseURL: "https://api.novita.ai/v3/openai", DefaultModel: "meta-llama/llama-3.1-70b-instruct", Priority: 8},
	{EnvVar: "ApiKey_Novita", ProviderType: "novita", ProviderName: "novita", BaseURL: "https://api.novita.ai/v3/openai", DefaultModel: "meta-llama/llama-3.1-70b-instruct", Priority: 8},

	// Upstage
	{EnvVar: "UPSTAGE_API_KEY", ProviderType: "upstage", ProviderName: "upstage", BaseURL: "https://api.upstage.ai/v1/solar", DefaultModel: "solar-pro", Priority: 8},
	{EnvVar: "ApiKey_Upstage", ProviderType: "upstage", ProviderName: "upstage", BaseURL: "https://api.upstage.ai/v1/solar", DefaultModel: "solar-pro", Priority: 8},

	// Chutes - NOTE: inference API uses llm.chutes.ai, NOT api.chutes.ai
	{EnvVar: "CHUTES_API_KEY", ProviderType: "chutes", ProviderName: "chutes", BaseURL: "https://llm.chutes.ai/v1/chat/completions", DefaultModel: "deepseek-ai/DeepSeek-V3", Priority: 8},
	{EnvVar: "ApiKey_Chutes", ProviderType: "chutes", ProviderName: "chutes", BaseURL: "https://llm.chutes.ai/v1/chat/completions", DefaultModel: "deepseek-ai/DeepSeek-V3", Priority: 8},

	// Venice AI
	{EnvVar: "VENICE_API_KEY", ProviderType: "venice", ProviderName: "venice", BaseURL: "https://api.venice.ai/api/v1/chat/completions", DefaultModel: "llama-3.3-70b", Priority: 4},
	{EnvVar: "ApiKey_Venice", ProviderType: "venice", ProviderName: "venice", BaseURL: "https://api.venice.ai/api/v1/chat/completions", DefaultModel: "llama-3.3-70b", Priority: 4},

	// Tier 7: Aggregators (use as fallback)
	// OpenRouter - Multiple key name variations
	{EnvVar: "OPENROUTER_API_KEY", ProviderType: "openrouter", ProviderName: "openrouter", BaseURL: "https://openrouter.ai/api/v1", DefaultModel: "anthropic/claude-3.5-sonnet", Priority: 10},
	{EnvVar: "ApiKey_OpenRouter", ProviderType: "openrouter", ProviderName: "openrouter", BaseURL: "https://openrouter.ai/api/v1", DefaultModel: "anthropic/claude-3.5-sonnet", Priority: 10},

	// Tier 7.5: OpenCode Zen (free models gateway)
	{EnvVar: "OPENCODE_API_KEY", ProviderType: "zen", ProviderName: "zen", BaseURL: "https://opencode.ai/zen/v1/chat/completions", DefaultModel: "big-pickle", Priority: 4},
	{EnvVar: "ZEN_API_KEY", ProviderType: "zen", ProviderName: "zen", BaseURL: "https://opencode.ai/zen/v1/chat/completions", DefaultModel: "big-pickle", Priority: 4},

	// Tier 7.6: Junie (JetBrains AI coding agent)
	{EnvVar: "JUNIE_API_KEY", ProviderType: "junie", ProviderName: "junie", BaseURL: "", DefaultModel: "sonnet", Priority: 2},
	{EnvVar: "ApiKey_Junie", ProviderType: "junie", ProviderName: "junie", BaseURL: "", DefaultModel: "sonnet", Priority: 2},

	// Tier 8: Self-hosted (local)
	// Ollama - Multiple configuration options
	{EnvVar: "OLLAMA_BASE_URL", ProviderType: "ollama", ProviderName: "ollama", BaseURL: "http://localhost:11434", DefaultModel: "llama3.2", Priority: 20},
	{EnvVar: "OLLAMA_HOST", ProviderType: "ollama", ProviderName: "ollama", BaseURL: "http://localhost:11434", DefaultModel: "llama3.2", Priority: 20},
	{EnvVar: "OLLAMA_API_URL", ProviderType: "ollama", ProviderName: "ollama", BaseURL: "http://localhost:11434", DefaultModel: "llama3.2", Priority: 20},
}

// NewProviderDiscovery creates a new provider discovery service
func NewProviderDiscovery(log *logrus.Logger, verifyOnStartup bool) *ProviderDiscovery {
	if log == nil {
		log = logrus.New()
	}

	return &ProviderDiscovery{
		providers:         safe.NewStore[string, *DiscoveredProvider](),
		scores:            safe.NewStore[string, *ProviderScore](),
		log:               log,
		verifyOnStartup:   verifyOnStartup,
		useDynamicScoring: true, // Enable dynamic LLMsVerifier scoring by default
	}
}

// SetVerifierScoreProvider sets the LLMsVerifier score provider for dynamic scoring
func (pd *ProviderDiscovery) SetVerifierScoreProvider(provider LLMsVerifierScoreProvider) {
	pd.configMu.Lock()
	defer pd.configMu.Unlock()
	pd.verifierScores = provider
	pd.useDynamicScoring = provider != nil

	if provider != nil {
		pd.log.Info("LLMsVerifier dynamic scoring enabled - provider selection will use real verification scores")
	}
}

// EnableDynamicScoring enables or disables dynamic LLMsVerifier scoring
func (pd *ProviderDiscovery) EnableDynamicScoring(enabled bool) {
	pd.configMu.Lock()
	defer pd.configMu.Unlock()
	pd.useDynamicScoring = enabled && pd.verifierScores != nil
}

// DiscoverProviders scans environment variables and discovers available providers
func (pd *ProviderDiscovery) DiscoverProviders() ([]*DiscoveredProvider, error) {
	pd.configMu.Lock()
	defer pd.configMu.Unlock()

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
				dp.SupportsModels = safe.NewSlice(dp.Capabilities.SupportedModels...)
			}
		}

		pd.providers.Put(mapping.ProviderName, dp)
		discovered = append(discovered, dp)
		seen[mapping.ProviderName] = true
	}

	// Discover OAuth-based providers (Claude Code and Qwen Code CLI)
	oauthProviders := pd.discoverOAuthProviders(seen)
	for _, dp := range oauthProviders {
		pd.providers.Put(dp.Name, dp)
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
	// IMPORTANT: Claude OAuth tokens from Claude Code CLI are PRODUCT-RESTRICTED and cannot
	// be used for direct API access. Instead, we forward requests to Claude Code CLI.
	if !seen["claude"] && !seen["claude-oauth"] && oauth_credentials.IsClaudeOAuthEnabled() {
		if oauthReader.HasValidClaudeCredentials() {
			pd.log.WithFields(logrus.Fields{
				"provider": "claude-oauth",
				"type":     "claude",
				"source":   "oauth_credentials",
			}).Info("Discovered Claude OAuth credentials - checking if Claude Code CLI is available")

			// Check if Claude Code CLI is installed and authenticated
			// OAuth tokens can ONLY be used through Claude Code CLI, not direct API
			if claude.IsClaudeCodeInstalled() && claude.IsClaudeCodeAuthenticated() {
				pd.log.Info("Claude Code CLI is installed and authenticated - using CLI facade for OAuth access")

				// Create Claude CLI provider (forwards requests to claude command)
				cliProvider := claude.NewClaudeCLIProviderWithModel("claude-sonnet-4-20250514")

				valid, errs := cliProvider.ValidateConfig(nil)
				if !valid {
					pd.log.WithField("errors", errs).Warn("Claude CLI provider validation failed")
				} else {
					dp := &DiscoveredProvider{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
						Name:         "claude-oauth",
						Type:         "claude",
						APIKeyEnvVar: "CLI:~/.claude/.credentials.json",
						APIKey:       "via-claude-cli", // Not an actual key, indicates CLI usage
						BaseURL:      "claude-cli",     // Indicates CLI-based access
						DefaultModel: "claude-sonnet-4-20250514",
						Provider:     cliProvider,
						Status:       ProviderStatusUnknown,
						Score:        0,
						Verified:     false,
					}

					if cliProvider != nil {
						dp.Capabilities = cliProvider.GetCapabilities()
						if dp.Capabilities != nil {
							dp.SupportsModels = safe.NewSlice(dp.Capabilities.SupportedModels...)
						}
					}

					discovered = append(discovered, dp)
					pd.log.Info("Claude OAuth provider discovered successfully (using Claude Code CLI facade)")
				}
			} else {
				// Claude Code CLI not installed or not authenticated
				reason := "not installed"
				if claude.IsClaudeCodeInstalled() {
					reason = "not authenticated"
				}
				pd.log.WithField("reason", reason).Warn(
					"Claude OAuth credentials found but Claude Code CLI is not available. " +
						"OAuth tokens are product-restricted and can only be used through Claude Code CLI. " +
						"Install and authenticate Claude Code CLI to use OAuth access.")
			}
		}
	}

	// Discover Qwen OAuth provider
	// NOTE: Qwen OAuth tokens from Qwen Code are product-restricted and cannot be used for general API calls.
	// They are only valid for portal.qwen.ai, not for DashScope API.
	// We use the Qwen Code CLI to forward requests, which handles authentication internally.
	if !seen["qwen"] && !seen["qwen-oauth"] && oauth_credentials.IsQwenOAuthEnabled() {
		if oauthReader.HasValidQwenCredentials() {
			pd.log.WithFields(logrus.Fields{
				"provider": "qwen-oauth",
				"type":     "qwen",
				"source":   "oauth_credentials",
			}).Info("Discovered Qwen OAuth credentials, checking Qwen Code CLI availability")

			// Qwen OAuth tokens are product-restricted and cannot be used for general API calls.
			// We MUST use Qwen Code CLI to forward requests.
			if qwen.IsQwenCodeInstalled() && qwen.IsQwenCodeAuthenticated() {
				// Create CLI provider which forwards requests to qwen command
				cliProvider := qwen.NewQwenCLIProviderWithModel("qwen-plus")

				// Validate CLI provider works
				valid, errs := cliProvider.ValidateConfig(nil)
				if !valid {
					pd.log.WithField("errors", errs).Warn("Qwen CLI provider validation failed")
				} else {
					dp := &DiscoveredProvider{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
						Name:         "qwen-oauth",
						Type:         "qwen",
						APIKeyEnvVar: "CLI:~/.qwen/oauth_creds.json",
						APIKey:       "qwen-cli", // Marker for CLI-based auth
						BaseURL:      "qwen-cli://local",
						DefaultModel: "qwen-plus",
						Provider:     cliProvider,
						Status:       ProviderStatusUnknown,
						Score:        0,
						Verified:     false,
					}

					if cliProvider != nil {
						dp.Capabilities = cliProvider.GetCapabilities()
						if dp.Capabilities != nil {
							dp.SupportsModels = safe.NewSlice(dp.Capabilities.SupportedModels...)
						}
					}

					discovered = append(discovered, dp)
					pd.log.Info("Qwen OAuth provider discovered via Qwen Code CLI")
				}
			} else {
				// Qwen Code CLI not installed or not authenticated
				reason := "not installed"
				if qwen.IsQwenCodeInstalled() {
					reason = "not authenticated"
				}
				pd.log.WithField("reason", reason).Warn(
					"Qwen OAuth credentials found but Qwen Code CLI is not available. " +
						"OAuth tokens are product-restricted and can only be used through Qwen Code CLI. " +
						"Install and authenticate Qwen Code CLI to use OAuth access.")
			}
		}
	}

	// Discover Zen provider (supports anonymous mode for free models - no API key required)
	if !seen["zen"] {
		apiKey := os.Getenv("OPENCODE_API_KEY")

		pd.log.WithFields(logrus.Fields{
			"provider":  "zen",
			"type":      "zen",
			"anonymous": apiKey == "",
		}).Info("Discovering OpenCode Zen provider (supports free models without API key)")

		// Create Zen provider - API key for direct access, then HTTP server, then CLI proxy
		var provider llm.LLMProvider
		if apiKey == "" {
			// Free mode - try HTTP server first (best), then CLI proxy, then anonymous API
			// Priority: HTTP server > CLI proxy > Anonymous API

			// Try HTTP server first (opencode serve)
			httpProvider := zen.NewZenHTTPProviderWithModel("big-pickle")
			if httpProvider.IsServerRunning() {
				provider = httpProvider
				pd.log.Info("Created Zen provider with HTTP server (opencode serve)")
			} else if zen.CanUseZenHTTP() {
				// HTTP available but server not running - auto-start will happen on first request
				provider = httpProvider
				pd.log.Info("Created Zen provider with HTTP server (will auto-start on first request)")
			}

			// Fallback to CLI proxy
			if provider == nil {
				cliProvider := zen.NewZenCLIProviderWithModel("big-pickle")
				if cliProvider.IsCLIAvailable() {
					provider = cliProvider
					pd.log.Info("Created Zen provider with CLI proxy (free models via opencode command)")
				}
			}

			// Final fallback to anonymous API
			if provider == nil {
				provider = zen.NewZenProviderAnonymous("big-pickle")
				pd.log.Info("Created Zen provider in anonymous mode (CLI not available, using direct API)")
			}
		} else {
			provider = zen.NewZenProvider(apiKey, "https://opencode.ai/zen/v1/chat/completions", "big-pickle")
		}

		if provider != nil {
			dp := &DiscoveredProvider{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
				Name:         "zen",
				Type:         "zen",
				APIKeyEnvVar: "OPENCODE_API_KEY (optional - anonymous mode for free models)",
				APIKey:       apiKey,
				BaseURL:      "https://opencode.ai/zen/v1/chat/completions",
				DefaultModel: "big-pickle",
				Provider:     provider,
				Status:       ProviderStatusUnknown,
				Score:        7.5, // Good base score for free models
				Verified:     false,
			}

			if provider != nil { //nolint:govet
				dp.Capabilities = provider.GetCapabilities()
				if dp.Capabilities != nil {
					dp.SupportsModels = safe.NewSlice(dp.Capabilities.SupportedModels...)
				}
			}

			discovered = append(discovered, dp)
			pd.log.Info("Zen provider discovered successfully (free models available)")
		}
	}

	return discovered
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

	case "zen":
		// Use native Zen provider for OpenCode Zen
		// Priority: API key > HTTP server > CLI proxy > Anonymous API
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://opencode.ai/zen/v1/chat/completions"
		}
		// If no API key provided, try HTTP server, then CLI proxy
		if apiKey == "" {
			// Try HTTP server first (opencode serve)
			if zen.CanUseZenHTTP() {
				logrus.Info("Creating Zen provider with HTTP server (opencode serve)")
				return zen.NewZenHTTPProviderWithModel(mapping.DefaultModel), nil
			}
			// Fallback to CLI proxy
			cliProvider := zen.NewZenCLIProviderWithModel(mapping.DefaultModel)
			if cliProvider.IsCLIAvailable() {
				logrus.Info("Creating Zen provider with CLI proxy (free models via opencode command)")
				return cliProvider, nil
			}
			// Final fallback to anonymous API
			logrus.Info("Creating Zen provider in anonymous mode (CLI not available)")
			return zen.NewZenProviderAnonymous(mapping.DefaultModel), nil
		}
		return zen.NewZenProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "openai":
		// Use native OpenAI provider for direct API access
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return openai.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "xai":
		// Use native xAI provider for Grok models
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.x.ai/v1"
		}
		return xai.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "junie":
		if junie.CanUseJunieCLI() {
			logrus.Info("Creating Junie provider with CLI mode (JetBrains Junie)")
			config := junie.DefaultJunieConfig()
			if apiKey != "" {
				config.APIKey = apiKey
			}
			return junie.NewJunieProvider(config), nil
		}
		logrus.Warn("Junie CLI not available: ", apiKey)
		// Use native Groq provider for fast inference
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.groq.com/openai/v1"
		}
		return groq.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "cohere":
		// Use native Cohere provider for Command models
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.cohere.com/v2"
		}
		return cohere.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "together":
		// Use native Together AI provider
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.together.xyz/v1"
		}
		return together.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "perplexity":
		// Use native Perplexity provider for search-focused LLM
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.perplexity.ai"
		}
		return perplexity.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "ai21":
		// Use native AI21 provider for Jamba models
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.ai21.com/studio/v1"
		}
		return ai21.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "fireworks":
		// Use native Fireworks AI provider for fast inference
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.fireworks.ai/inference/v1"
		}
		return fireworks.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "anthropic":
		// Use native Anthropic provider for direct Claude API access
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		return anthropic.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "replicate":
		// Use native Replicate provider for async predictions
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.replicate.com/v1/predictions"
		}
		return replicate.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "huggingface":
		// Use native HuggingFace provider for Inference API
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api-inference.huggingface.co/models"
		}
		return huggingface.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "chutes":
		// Use native Chutes provider for OpenAI-compatible inference
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://llm.chutes.ai/v1/chat/completions"
		}
		return chutes.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "github-models":
		// Use native GitHub Models provider for GitHub's inference API
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://models.github.ai/inference/chat/completions"
		}
		return githubmodels.NewGitHubModelsProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "venice":
		// Use native Venice AI provider
		baseURL := mapping.BaseURL
		if baseURL == "" {
			baseURL = "https://api.venice.ai/api/v1/chat/completions"
		}
		return venice.NewProvider(apiKey, baseURL, mapping.DefaultModel), nil

	case "groq":
		return groq.NewProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil

	case "zai":
		return zai.NewZAIProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil

	// Native provider implementations — previously mis-routed through an
	// OpenRouter-compatible fallback, which caused "failed to decode
	// OpenRouter response" errors when these providers return their own
	// response format. Each native provider speaks its own API shape.
	case "siliconflow":
		return siliconflow.NewSiliconFlowProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil
	case "hyperbolic":
		return hyperbolic.NewHyperbolicProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil
	case "sambanova":
		return sambanova.NewSambaNovaProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil
	case "nvidia":
		return nvidia.NewNvidiaProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil
	case "kimi":
		return kimi.NewKimiProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil
	case "novita":
		return novita.NewNovitaProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil
	case "upstage":
		return upstage.NewUpstageProvider(apiKey, mapping.BaseURL, mapping.DefaultModel), nil

	// Cloudflare needs an extra account ID; leave it on the generic
	// fallback until CLOUDFLARE_ACCOUNT_ID discovery lands.
	case "cloudflare":
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
	providers := make([]*DiscoveredProvider, 0, pd.providers.Len())
	pd.providers.Range(func(_ string, p *DiscoveredProvider) bool {
		providers = append(providers, p)
		return true
	})

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
		provider.mu.Lock()
		provider.Status = ProviderStatusUnhealthy
		provider.Error = "provider not initialized"
		provider.mu.Unlock()
		return
	}

	start := time.Now()

	// Special handling for Zen anonymous mode - STRICT verification required
	// We MUST test actual model completion to detect canned error responses
	if provider.Type == "zen" && provider.APIKey == "" {
		pd.log.WithFields(logrus.Fields{
			"provider": provider.Name,
			"type":     provider.Type,
		}).Info("Verifying Zen provider with strict model completion testing")

		// Try the health check first
		if err := provider.Provider.HealthCheck(); err != nil {
			pd.log.WithFields(logrus.Fields{
				"provider": provider.Name,
				"error":    err.Error(),
			}).Warn("Zen health check failed - provider will not be verified")

			provider.mu.Lock()
			provider.Status = ProviderStatusUnhealthy
			provider.Verified = false
			provider.Score = 0
			provider.Error = "health check failed: " + err.Error()
			provider.VerifiedAt = time.Now()
			provider.mu.Unlock()
			return
		}

		// CRITICAL: Do actual model completion test to detect canned error responses
		pd.log.WithField("provider", provider.Name).Debug("Starting model completion test for Zen")

		testReq := &models.LLMRequest{
			ID:        fmt.Sprintf("verify_zen_%d", time.Now().UnixNano()),
			SessionID: "verification",
			Prompt:    "You are a helpful assistant. Reply concisely.",
			Messages: []models.Message{
				{Role: "user", Content: "What is 2 + 2? Reply with just the number."},
			},
			ModelParams: models.ModelParameters{
				Model:       provider.DefaultModel,
				MaxTokens:   10,
				Temperature: 0.0,
			},
			Status:    "pending",
			CreatedAt: time.Now(),
		}

		verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		testStart := time.Now()
		resp, err := provider.Provider.Complete(verifyCtx, testReq)
		testLatency := time.Since(testStart)
		cancel()

		provider.mu.Lock()
		defer provider.mu.Unlock()
		provider.VerifiedAt = time.Now()

		if err != nil {
			pd.log.WithFields(logrus.Fields{
				"provider": provider.Name,
				"error":    err.Error(),
			}).Warn("Zen model completion test failed")

			provider.Status = ProviderStatusUnhealthy
			provider.Verified = false
			provider.Score = 0
			provider.Error = "model completion test failed: " + err.Error()
			return
		}

		// Check for canned error responses
		if resp == nil || resp.Content == "" {
			pd.log.WithField("provider", provider.Name).Warn("Zen returned empty response")
			provider.Status = ProviderStatusUnhealthy
			provider.Verified = false
			provider.Score = 0
			provider.Error = "empty response from model"
			return
		}

		// Check for canned error patterns
		cannedPatterns := []string{
			"unable to provide", "unable to analyze", "at this time",
			"cannot provide", "cannot analyze", "i apologize, but i cannot",
			"error occurred", "service unavailable", "failed to generate",
		}
		loweredContent := strings.ToLower(resp.Content)
		for _, pattern := range cannedPatterns {
			if strings.Contains(loweredContent, pattern) {
				pd.log.WithFields(logrus.Fields{
					"provider": provider.Name,
					"pattern":  pattern,
					"response": resp.Content,
				}).Warn("Zen returned canned error response - NOT VERIFIED")

				provider.Status = ProviderStatusUnhealthy
				provider.Verified = false
				provider.Score = 0
				provider.Error = fmt.Sprintf("canned error response: %s", pattern)
				return
			}
		}

		// Check for suspiciously fast response (< 100ms typically indicates cached error)
		if testLatency < 100*time.Millisecond && len(resp.Content) < 20 {
			pd.log.WithFields(logrus.Fields{
				"provider": provider.Name,
				"latency":  testLatency,
				"response": resp.Content,
			}).Warn("Suspiciously fast response from Zen - may be cached error")
			// Don't fail, but log warning
		}

		// STRICT: Response must contain "4" for 2+2 test
		if !strings.Contains(resp.Content, "4") {
			pd.log.WithFields(logrus.Fields{
				"provider": provider.Name,
				"expected": "4",
				"got":      resp.Content,
			}).Warn("Zen failed basic math test - NOT VERIFIED")

			provider.Status = ProviderStatusUnhealthy
			provider.Verified = false
			provider.Score = 0
			provider.Error = fmt.Sprintf("failed basic math test: expected '4', got: %s", resp.Content)
			return
		}

		// Model passed verification!
		pd.log.WithFields(logrus.Fields{
			"provider": provider.Name,
			"response": resp.Content,
			"latency":  testLatency,
		}).Info("Zen model passed strict verification")

		provider.Status = ProviderStatusHealthy
		provider.Verified = true
		provider.Score = 7.5 // Good score for verified free models
		provider.Error = ""

		// Populate VerifiedModels with the verified model
		if provider.VerifiedModels == nil {
			provider.VerifiedModels = safe.NewSlice[VerifiedModel]()
		}
		provider.VerifiedModels.Append(VerifiedModel{
			Name:     provider.DefaultModel,
			Score:    7.5,
			Verified: true,
		})

		return
	}

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

	// Lock the provider to protect concurrent writes to verification fields
	provider.mu.Lock()
	defer provider.mu.Unlock()

	provider.VerifiedAt = time.Now()

	if err != nil {
		errStr := strings.ToLower(err.Error())

		// Categorize the error. The log message differs per category so
		// operators can quickly distinguish stale-credential problems
		// (user-fixable) from URL/format/network bugs (code-fixable).
		var logMsg string
		switch {
		case containsAny(errStr, "429", "quota", "rate", "resource_exhausted", "too many"):
			provider.Status = ProviderStatusRateLimited
			provider.Error = "rate limited or quota exceeded"
			logMsg = "Provider verification rate-limited"
		case containsAny(errStr, "401", "403", "unauthorized", "invalid", "authentication", "api_key", "forbidden"):
			provider.Status = ProviderStatusAuthFailed
			provider.Error = "authentication failed or invalid API key"
			logMsg = "Provider API key rejected (HTTP 401/403) — likely stale or revoked"
		default:
			provider.Status = ProviderStatusUnhealthy
			provider.Error = err.Error()
			logMsg = "Provider verification failed"
		}
		provider.Verified = false

		pd.log.WithFields(logrus.Fields{
			"provider": provider.Name,
			"status":   provider.Status,
			"error":    provider.Error,
			"duration": responseTime,
		}).Warn(logMsg)
		return
	}

	// Check for valid response
	if resp != nil && resp.Content != "" {
		// Check for canned error patterns in response
		cannedPatterns := []string{
			"unable to provide", "unable to analyze", "at this time",
			"cannot provide", "cannot analyze", "i apologize, but i cannot",
			"error occurred", "service unavailable", "failed to generate",
		}
		loweredContent := strings.ToLower(resp.Content)
		isCannedError := false
		var cannedPattern string
		for _, pattern := range cannedPatterns {
			if strings.Contains(loweredContent, pattern) {
				isCannedError = true
				cannedPattern = pattern
				break
			}
		}

		if isCannedError {
			pd.log.WithFields(logrus.Fields{
				"provider": provider.Name,
				"pattern":  cannedPattern,
				"response": resp.Content,
			}).Warn("Provider returned canned error response - NOT VERIFIED")

			provider.Status = ProviderStatusUnhealthy
			provider.Verified = false
			provider.Score = 0
			provider.Error = fmt.Sprintf("canned error response: %s", cannedPattern)
			return
		}

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
	pd.scores.Put(provider.Name, &ProviderScore{
		Provider:        provider.Name,
		OverallScore:    totalScore,
		ResponseSpeed:   responseBonus,
		Capabilities:    capBonus,
		Reliability:     baseScore * 0.5,
		VerifiedWorking: provider.Verified,
		ScoredAt:        time.Now(),
	})

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
	pd.configMu.RLock()
	defer pd.configMu.RUnlock()

	// Collect verified providers with their scores (thread-safe read)
	type providerWithScore struct {
		provider *DiscoveredProvider
		score    float64
	}
	verified := make([]providerWithScore, 0)
	pd.providers.Range(func(_ string, p *DiscoveredProvider) bool {
		p.mu.RLock()
		isVerified := p.Verified
		isHealthy := p.Status == ProviderStatusHealthy
		score := p.Score
		p.mu.RUnlock()
		if isVerified && isHealthy {
			verified = append(verified, providerWithScore{provider: p, score: score})
		}
		return true
	})

	// Sort by score (descending)
	sort.Slice(verified, func(i, j int) bool {
		return verified[i].score > verified[j].score
	})

	// Extract providers in sorted order
	result := make([]*DiscoveredProvider, len(verified))
	for i, v := range verified {
		result[i] = v.provider
	}

	// Return top N
	if n <= 0 || n > len(result) {
		return result
	}
	return result[:n]
}

// GetAllProviders returns all discovered providers
func (pd *ProviderDiscovery) GetAllProviders() []*DiscoveredProvider {
	pd.configMu.RLock()
	defer pd.configMu.RUnlock()

	// Collect providers with their scores (thread-safe read)
	type providerWithScore struct {
		provider *DiscoveredProvider
		score    float64
	}
	items := make([]providerWithScore, 0, pd.providers.Len())
	pd.providers.Range(func(_ string, p *DiscoveredProvider) bool {
		p.mu.RLock()
		score := p.Score
		p.mu.RUnlock()
		items = append(items, providerWithScore{provider: p, score: score})
		return true
	})

	// Sort by score
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	// Extract providers in sorted order
	result := make([]*DiscoveredProvider, len(items))
	for i, item := range items {
		result[i] = item.provider
	}

	return result
}

// GetProviderByName returns a specific discovered provider
func (pd *ProviderDiscovery) GetProviderByName(name string) *DiscoveredProvider {
	v, _ := pd.providers.Get(name)
	return v
}

// GetProviderScore returns the score for a provider
func (pd *ProviderDiscovery) GetProviderScore(name string) *ProviderScore {
	v, _ := pd.scores.Get(name)
	return v
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
	pd.configMu.RLock()
	defer pd.configMu.RUnlock()

	var healthy, rateLimited, authFailed, unhealthy, unknown int
	var totalScore float64
	providerList := make([]map[string]interface{}, 0)

	pd.providers.Range(func(_ string, p *DiscoveredProvider) bool {
		// Lock provider for reading its verification fields
		p.mu.RLock()
		status := p.Status
		score := p.Score
		verified := p.Verified
		errStr := p.Error
		p.mu.RUnlock()

		switch status {
		case ProviderStatusHealthy:
			healthy++
			totalScore += score
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
			"status":   status,
			"score":    score,
			"verified": verified,
			"error":    errStr,
		})
		return true
	})

	// Sort by score
	sort.Slice(providerList, func(i, j int) bool {
		//nolint:errcheck // score is guaranteed to be float64
		return providerList[i]["score"].(float64) > providerList[j]["score"].(float64)
	})

	avgScore := 0.0
	if healthy > 0 {
		avgScore = totalScore / float64(healthy)
	}

	return map[string]interface{}{
		"total_discovered": pd.providers.Len(),
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
	pd.configMu.RLock()
	defer pd.configMu.RUnlock()

	registered := 0
	pd.providers.Range(func(_ string, p *DiscoveredProvider) bool {
		if p.Verified && p.Status == ProviderStatusHealthy && p.Provider != nil {
			if err := registry.RegisterProvider(p.Name, p.Provider); err != nil {
				pd.log.WithError(err).Warnf("Failed to register provider %s", p.Name)
				return true
			}
			registered++
			pd.log.WithFields(logrus.Fields{
				"provider": p.Name,
				"score":    p.Score,
			}).Info("Registered verified provider to registry")
		}
		return true
	})

	pd.log.Infof("Registered %d verified providers to registry", registered)
	return nil
}
