// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

// ProviderAccessRegistry maps all supported providers to their access configurations.
// This is the static registry for provider authentication, subscription tiers, and
// API endpoints used for subscription detection.
var ProviderAccessRegistry = map[string]*ProviderAccessConfig{
	"openai": {
		ProviderType: "openai",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		SubscriptionCheckURL: "",
		BillingCheckURL:      "https://api.openai.com/v1/dashboard/billing/credit_grants",
		ModelsURL:            "https://api.openai.com/v1/models",
		DefaultSubscription:  SubTypeFreeCredits,
		AvailableTiers:       []SubscriptionType{SubTypeFreeCredits, SubTypePayAsYouGo, SubTypeEnterprise},
		RateLimitHeaders: &RateLimitHeaderNames{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
			RequestsLimit:     "x-ratelimit-limit-requests",
			RequestsRemaining: "x-ratelimit-remaining-requests",
			RequestsReset:     "x-ratelimit-reset-requests",
			TokensLimit:       "x-ratelimit-limit-tokens",
			TokensRemaining:   "x-ratelimit-remaining-tokens",
			TokensReset:       "x-ratelimit-reset-tokens",
		},
	},

	"claude": {
		ProviderType: "claude",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "x-api-key",
			HeaderPrefix: "",
			ExtraHeaders: map[string]string{
				"anthropic-version": "2023-06-01",
			},
		},
		AuthMechanisms: []AuthMechanism{
			{
				HeaderName:   "x-api-key",
				HeaderPrefix: "",
				ExtraHeaders: map[string]string{"anthropic-version": "2023-06-01"},
			},
			{
				HeaderName:   "Authorization",
				HeaderPrefix: "Bearer ",
				ExtraHeaders: map[string]string{"anthropic-version": "2023-06-01"},
			},
		},
		ModelsURL:           "https://api.anthropic.com/v1/models",
		DefaultSubscription: SubTypePayAsYouGo,
		AvailableTiers:      []SubscriptionType{SubTypePayAsYouGo, SubTypeEnterprise},
		RateLimitHeaders: &RateLimitHeaderNames{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
			RequestsLimit:     "anthropic-ratelimit-requests-limit",
			RequestsRemaining: "anthropic-ratelimit-requests-remaining",
			RequestsReset:     "anthropic-ratelimit-requests-reset",
			TokensLimit:       "anthropic-ratelimit-tokens-limit",
			TokensRemaining:   "anthropic-ratelimit-tokens-remaining",
			TokensReset:       "anthropic-ratelimit-tokens-reset",
		},
	},

	"gemini": {
		ProviderType: "gemini",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "x-goog-api-key",
			HeaderPrefix: "",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "x-goog-api-key", HeaderPrefix: ""},
			{QueryParam: "key"},
		},
		ModelsURL:           "https://generativelanguage.googleapis.com/v1beta/models",
		DefaultSubscription: SubTypeFreeTier,
		AvailableTiers:      []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo, SubTypeEnterprise},
	},

	"deepseek": {
		ProviderType: "deepseek",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.deepseek.com/v1/models",
		DefaultSubscription: SubTypeFreeCredits,
		AvailableTiers:      []SubscriptionType{SubTypeFreeCredits, SubTypePayAsYouGo},
	},

	"mistral": {
		ProviderType: "mistral",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.mistral.ai/v1/models",
		DefaultSubscription: SubTypeFreeTier,
		AvailableTiers:      []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo, SubTypeEnterprise},
	},

	"groq": {
		ProviderType: "groq",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.groq.com/openai/v1/models",
		DefaultSubscription: SubTypeFree,
		AvailableTiers:      []SubscriptionType{SubTypeFree, SubTypePayAsYouGo, SubTypeEnterprise},
		RateLimitHeaders: &RateLimitHeaderNames{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
			RequestsLimit:     "x-ratelimit-limit-requests",
			RequestsRemaining: "x-ratelimit-remaining-requests",
			RequestsReset:     "x-ratelimit-reset-requests",
			TokensLimit:       "x-ratelimit-limit-tokens",
			TokensRemaining:   "x-ratelimit-remaining-tokens",
			TokensReset:       "x-ratelimit-reset-tokens",
		},
	},

	"cerebras": {
		ProviderType: "cerebras",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.cerebras.ai/v1/models",
		DefaultSubscription: SubTypeFree,
		AvailableTiers:      []SubscriptionType{SubTypeFree, SubTypePayAsYouGo, SubTypeEnterprise},
	},

	"openrouter": {
		ProviderType: "openrouter",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		SubscriptionCheckURL: "https://openrouter.ai/api/v1/auth/key",
		ModelsURL:            "https://openrouter.ai/api/v1/models",
		DefaultSubscription:  SubTypeFreeTier,
		AvailableTiers:       []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo},
		RateLimitHeaders: &RateLimitHeaderNames{
			RequestsLimit:     "X-RateLimit-Limit",
			RequestsRemaining: "X-RateLimit-Remaining",
			RequestsReset:     "X-RateLimit-Reset",
		},
	},

	"together": {
		ProviderType: "together",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.together.xyz/v1/models",
		DefaultSubscription: SubTypeFreeTier,
		AvailableTiers:      []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo},
	},

	"fireworks": {
		ProviderType: "fireworks",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.fireworks.ai/inference/v1/models",
		DefaultSubscription: SubTypeFreeCredits,
		AvailableTiers:      []SubscriptionType{SubTypeFreeCredits, SubTypePayAsYouGo},
	},

	"cohere": {
		ProviderType: "cohere",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		SubscriptionCheckURL: "https://api.cohere.com/check-api-key",
		ModelsURL:            "https://api.cohere.com/v2/models",
		DefaultSubscription:  SubTypeFreeTier,
		AvailableTiers:       []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo, SubTypeEnterprise},
	},

	"ai21": {
		ProviderType: "ai21",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.ai21.com/studio/v1/models",
		DefaultSubscription: SubTypeFreeCredits,
		AvailableTiers:      []SubscriptionType{SubTypeFreeCredits, SubTypePayAsYouGo, SubTypeEnterprise},
	},

	"perplexity": {
		ProviderType: "perplexity",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		DefaultSubscription: SubTypePayAsYouGo,
		AvailableTiers:      []SubscriptionType{SubTypePayAsYouGo},
	},

	"grok": {
		ProviderType: "grok",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.x.ai/v1/models",
		DefaultSubscription: SubTypeFreeCredits,
		AvailableTiers:      []SubscriptionType{SubTypeFreeCredits, SubTypePayAsYouGo, SubTypeEnterprise},
	},

	"huggingface": {
		ProviderType: "huggingface",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		DefaultSubscription: SubTypeFree,
		AvailableTiers:      []SubscriptionType{SubTypeFree, SubTypeMonthly, SubTypeEnterprise},
		RateLimitHeaders: &RateLimitHeaderNames{
			RequestsLimit: "RateLimit",
		},
	},

	"replicate": {
		ProviderType: "replicate",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.replicate.com/v1/models",
		DefaultSubscription: SubTypeFreeTier,
		AvailableTiers:      []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo},
	},

	"sambanova": {
		ProviderType: "sambanova",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.sambanova.ai/v1/models",
		DefaultSubscription: SubTypeFreeCredits,
		AvailableTiers:      []SubscriptionType{SubTypeFreeCredits, SubTypePayAsYouGo, SubTypeEnterprise},
		RateLimitHeaders: &RateLimitHeaderNames{
			RequestsLimit: "X-RateLimit-Limit-RPM",
			DailyLimit:    "X-RateLimit-Limit-RPD",
		},
	},

	"chutes": {
		ProviderType: "chutes",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://llm.chutes.ai/v1/models",
		DefaultSubscription: SubTypeFreeTier,
		AvailableTiers:      []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo},
	},

	"zai": {
		ProviderType: "zai",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://api.z.ai/api/paas/v4/models",
		DefaultSubscription: SubTypeMonthly,
		AvailableTiers:      []SubscriptionType{SubTypeFreeCredits, SubTypePayAsYouGo, SubTypeMonthly},
	},

	"qwen": {
		ProviderType: "qwen",
		PrimaryAuth: AuthMechanism{
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		},
		AuthMechanisms: []AuthMechanism{
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		ModelsURL:           "https://dashscope.aliyuncs.com/compatible-mode/v1/models",
		DefaultSubscription: SubTypeFreeTier,
		AvailableTiers:      []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo},
	},

	"zen": {
		ProviderType: "zen",
		PrimaryAuth: AuthMechanism{
			NoAuth:         true,
			DeviceIDHeader: "X-Device-ID",
		},
		AuthMechanisms: []AuthMechanism{
			{NoAuth: true, DeviceIDHeader: "X-Device-ID"},
			{HeaderName: "Authorization", HeaderPrefix: "Bearer "},
		},
		DefaultSubscription: SubTypeFree,
		AvailableTiers:      []SubscriptionType{SubTypeFree, SubTypePayAsYouGo},
	},

	"ollama": {
		ProviderType: "ollama",
		PrimaryAuth: AuthMechanism{
			NoAuth: true,
		},
		AuthMechanisms: []AuthMechanism{
			{NoAuth: true},
		},
		ModelsURL:           "http://localhost:11434/api/tags",
		DefaultSubscription: SubTypeFree,
		AvailableTiers:      []SubscriptionType{SubTypeFree},
	},
}

// GetProviderAccessConfig returns the access configuration for a provider type.
// Returns nil if the provider is not found in the registry.
func GetProviderAccessConfig(providerType string) *ProviderAccessConfig {
	config, ok := ProviderAccessRegistry[providerType]
	if !ok {
		return nil
	}
	return config
}

// GetAllProviderAccessConfigs returns all registered provider access configurations.
func GetAllProviderAccessConfigs() map[string]*ProviderAccessConfig {
	return ProviderAccessRegistry
}

// GetProvidersWithSubscriptionAPI returns providers that expose subscription check APIs.
func GetProvidersWithSubscriptionAPI() []string {
	var providers []string
	for name, config := range ProviderAccessRegistry {
		if config.HasSubscriptionCheckAPI() {
			providers = append(providers, name)
		}
	}
	return providers
}

// GetProvidersWithRateLimitHeaders returns providers that have known rate limit header mappings.
func GetProvidersWithRateLimitHeaders() []string {
	var providers []string
	for name, config := range ProviderAccessRegistry {
		if config.HasRateLimitHeaders() {
			providers = append(providers, name)
		}
	}
	return providers
}
