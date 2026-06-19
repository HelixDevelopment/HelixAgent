package services

import "testing"

// clearProviderEnvVarsForTest scrubs every env-var the provider
// auto-discovery mapping table recognises, for the duration of t.
// Without this, "unit" tests that exercise the discovery path
// silently pull in 25+ providers from the developer host's real
// API keys, take 15-25s each, and leave HTTP/2 read goroutines
// parked in TLS reads (CONST-035 anti-bluff: a unit test must not
// turn into an unannounced integration test against the cloud).
//
// Keep this list in sync with internal/services/provider_discovery.go's
// providerMappings table. Re-derive with:
//
//	grep -oP 'EnvVar:\s*"\K[^"]+' internal/services/provider_discovery.go | sort -u
func clearProviderEnvVarsForTest(t *testing.T) {
	t.Helper()
	for _, k := range providerDiscoveryEnvVars {
		t.Setenv(k, "")
	}
}

// providerDiscoveryEnvVars enumerates every env-var that the
// provider-discovery mapping table looks at (and therefore every
// env-var a unit test must scrub to keep discovery a no-op).
var providerDiscoveryEnvVars = []string{
	"AI21_API_KEY", "ALIBABA_API_KEY", "ANTHROPIC_API_KEY",
	"ApiKey_AI21", "ApiKey_Anthropic", "ApiKey_Cerebras",
	"ApiKey_Chutes", "ApiKey_Claude", "ApiKey_Cohere",
	"ApiKey_DeepSeek", "ApiKey_Fireworks", "ApiKey_Gemini",
	"ApiKey_Grok", "ApiKey_Groq", "ApiKey_HuggingFace",
	"ApiKey_Hyperbolic", "ApiKey_Junie", "ApiKey_Kimi",
	"ApiKey_Mistral", "ApiKey_Novita", "ApiKey_NVIDIA",
	"ApiKey_OpenAI", "ApiKey_OpenRouter", "ApiKey_Perplexity",
	"ApiKey_Qwen", "ApiKey_Replicate", "ApiKey_SambaNova",
	"ApiKey_SiliconFlow", "ApiKey_Together", "ApiKey_Upstage",
	"ApiKey_Venice", "ApiKey_XAI", "ApiKey_ZAI",
	"BIGMODEL_API_KEY", "CEREBRAS_API_KEY", "CF_API_KEY",
	"CHUTES_API_KEY", "CLAUDE_API_KEY", "CLOUDFLARE_API_KEY",
	"CO_API_KEY", "CODESTRAL_API_KEY", "COHERE_API_KEY",
	"DASHSCOPE_API_KEY", "DEEPSEEK_API_KEY", "DEEPSEEK_KEY",
	"FIREWORKS_API_KEY", "GEMINI_API_KEY", "GITHUB_MODELS_API_KEY",
	"GITHUB_TOKEN", "GLM_API_KEY", "GOOGLE_AI_API_KEY",
	"GOOGLE_API_KEY", "GROK_API_KEY", "GROQ_API_KEY",
	"HF_API_KEY", "HF_TOKEN", "HUGGINGFACE_API_KEY",
	"HUGGINGFACE_TOKEN", "HYPERBOLIC_API_KEY", "JUNIE_API_KEY",
	"KIMI_API_KEY", "MISTRAL_API_KEY", "MOONSHOT_API_KEY",
	"NGC_API_KEY", "NOVITA_API_KEY", "NVIDIA_API_KEY",
	"OLLAMA_API_URL", "OLLAMA_BASE_URL", "OLLAMA_HOST",
	"OPENAI_API_KEY", "OPENAI_KEY", "OPENCODE_API_KEY",
	"OPENROUTER_API_KEY", "PERPLEXITY_API_KEY", "PPLX_API_KEY",
	"QWEN_API_KEY", "REPLICATE_API_KEY", "REPLICATE_API_TOKEN",
	"SAMBANOVA_API_KEY", "SILICONFLOW_API_KEY", "TOGETHERAI_API_KEY",
	"TOGETHER_API_KEY", "UPSTAGE_API_KEY", "VENICE_API_KEY",
	"XAI_API_KEY", "XIAOMI_API_KEY", "XIAOMI_MIMO_API_KEY",
	"ApiKey_Xiaomi_MiMo", "ZAI_API_KEY", "ZEN_API_KEY",
	"ZHIPU_API_KEY",
}
