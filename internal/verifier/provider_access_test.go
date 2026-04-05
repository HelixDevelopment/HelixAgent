package verifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderAccessRegistry_Completeness(t *testing.T) {
	t.Parallel()
	// All core providers must be in the registry
	coreProviders := []string{
		"openai", "claude", "gemini", "deepseek", "mistral",
		"groq", "cerebras", "openrouter", "together", "fireworks",
		"cohere", "ai21", "perplexity", "grok", "huggingface",
		"replicate", "sambanova", "chutes", "zai", "qwen",
		"zen", "ollama",
	}

	for _, provider := range coreProviders {
		t.Run(provider, func(t *testing.T) {
				t.Parallel()
			config := GetProviderAccessConfig(provider)
			require.NotNil(t, config, "provider %s should be in registry", provider)
			assert.Equal(t, provider, config.ProviderType)
		})
	}

	assert.GreaterOrEqual(t, len(ProviderAccessRegistry), 22,
		"registry should have at least 22 providers")
}

func TestProviderAccessRegistry_AuthMechanisms(t *testing.T) {
	t.Parallel()
	for name, config := range ProviderAccessRegistry {
		t.Run(name, func(t *testing.T) {
				t.Parallel()
			assert.NotEmpty(t, config.AuthMechanisms,
				"provider %s should have at least one auth mechanism", name)

			// Primary auth should have a header name or NoAuth
			primary := config.PrimaryAuth
			hasAuth := primary.HeaderName != "" || primary.NoAuth || primary.QueryParam != ""
			assert.True(t, hasAuth,
				"provider %s primary auth should have header, query param, or NoAuth", name)
		})
	}
}

func TestProviderAccessRegistry_AvailableTiers(t *testing.T) {
	t.Parallel()
	for name, config := range ProviderAccessRegistry {
		t.Run(name, func(t *testing.T) {
				t.Parallel()
			assert.NotEmpty(t, config.AvailableTiers,
				"provider %s should have available tiers", name)
			assert.True(t, config.DefaultSubscription.IsValid(),
				"provider %s should have valid default subscription type", name)
		})
	}
}

func TestProviderAccessConfig_Anthropic_XAPIKey(t *testing.T) {
	t.Parallel()
	config := GetProviderAccessConfig("claude")
	require.NotNil(t, config)

	// Anthropic uses x-api-key, not Bearer
	assert.Equal(t, "x-api-key", config.PrimaryAuth.HeaderName)
	assert.Empty(t, config.PrimaryAuth.HeaderPrefix)
	assert.NotNil(t, config.PrimaryAuth.ExtraHeaders)
	assert.Equal(t, "2023-06-01", config.PrimaryAuth.ExtraHeaders["anthropic-version"])
}

func TestProviderAccessConfig_Gemini_XGoogAPIKey(t *testing.T) {
	t.Parallel()
	config := GetProviderAccessConfig("gemini")
	require.NotNil(t, config)

	// Gemini uses x-goog-api-key
	assert.Equal(t, "x-goog-api-key", config.PrimaryAuth.HeaderName)

	// Also supports query param
	assert.Len(t, config.AuthMechanisms, 2)
	assert.Equal(t, "key", config.AuthMechanisms[1].QueryParam)
}

func TestProviderAccessConfig_Zen_Anonymous(t *testing.T) {
	t.Parallel()
	config := GetProviderAccessConfig("zen")
	require.NotNil(t, config)

	assert.True(t, config.PrimaryAuth.NoAuth)
	assert.Equal(t, "X-Device-ID", config.PrimaryAuth.DeviceIDHeader)
	assert.Equal(t, SubTypeFree, config.DefaultSubscription)
}

func TestProviderAccessConfig_Ollama_NoAuth(t *testing.T) {
	t.Parallel()
	config := GetProviderAccessConfig("ollama")
	require.NotNil(t, config)

	assert.True(t, config.PrimaryAuth.NoAuth)
	assert.Equal(t, SubTypeFree, config.DefaultSubscription)
	assert.Len(t, config.AvailableTiers, 1)
	assert.Equal(t, SubTypeFree, config.AvailableTiers[0])
}

func TestGetProviderAccessConfig_Unknown(t *testing.T) {
	t.Parallel()
	config := GetProviderAccessConfig("nonexistent_provider")
	assert.Nil(t, config)
}

func TestGetProvidersWithSubscriptionAPI(t *testing.T) {
	t.Parallel()
	providers := GetProvidersWithSubscriptionAPI()
	assert.GreaterOrEqual(t, len(providers), 2,
		"at least OpenRouter and Cohere should have subscription check APIs")

	// Verify known providers with subscription APIs
	found := make(map[string]bool)
	for _, p := range providers {
		found[p] = true
	}
	assert.True(t, found["openrouter"], "openrouter should have subscription API")
	assert.True(t, found["cohere"], "cohere should have subscription API")
}

func TestGetProvidersWithRateLimitHeaders(t *testing.T) {
	t.Parallel()
	providers := GetProvidersWithRateLimitHeaders()
	assert.GreaterOrEqual(t, len(providers), 4,
		"at least 4 providers should have rate limit header mappings")
}
