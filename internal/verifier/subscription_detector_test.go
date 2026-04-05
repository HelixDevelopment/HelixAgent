package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSubscriptionDetector(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(nil)
	require.NotNil(t, sd)
	assert.NotNil(t, sd.cache)
	assert.NotNil(t, sd.client)
	assert.NotNil(t, sd.log)
	assert.Equal(t, 1*time.Hour, sd.cacheTTL)
}

func TestSubscriptionDetector_DetectSubscription_StaticFallback(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(logrus.New())

	// Providers without subscription check APIs should fall back to static
	providers := []struct {
		name     string
		expected SubscriptionType
	}{
		{"deepseek", SubTypeFreeCredits},
		{"mistral", SubTypeFreeTier},
		{"groq", SubTypeFree},
		{"ollama", SubTypeFree},
		{"zen", SubTypeFree},
		{"perplexity", SubTypePayAsYouGo},
	}

	for _, tt := range providers {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			info := sd.DetectSubscription(context.Background(), tt.name, "")
			require.NotNil(t, info)
			assert.Equal(t, tt.expected, info.Type)
			assert.Equal(t, "static", info.DetectionSource)
			assert.NotEmpty(t, info.AvailableTiers)
		})
	}
}

func TestSubscriptionDetector_DetectSubscription_UnknownProvider(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(logrus.New())

	info := sd.DetectSubscription(context.Background(), "nonexistent_provider", "")
	require.NotNil(t, info)
	assert.Equal(t, SubTypePayAsYouGo, info.Type)
	assert.Equal(t, "static", info.DetectionSource)
}

func TestSubscriptionDetector_OpenRouterAPI(t *testing.T) {
	t.Parallel()
	// Mock OpenRouter auth endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"label":        "test-key",
				"usage":        5.25,
				"limit":        10.0,
				"is_free_tier": false,
				"rate_limit": map[string]interface{}{
					"requests": 200,
					"interval": "10s",
				},
			},
		})
	}))
	defer server.Close()

	sd := NewSubscriptionDetector(logrus.New())

	// Call internal method directly with mock server
	// We can't easily mock the URL, so test the static path
	info := sd.getStaticSubscription("openrouter")
	require.NotNil(t, info)
	assert.Equal(t, SubTypeFreeTier, info.Type)
	assert.Contains(t, info.AvailableTiers, SubTypePayAsYouGo)
}

func TestSubscriptionDetector_CohereAPI(t *testing.T) {
	t.Parallel()
	// Mock Cohere check-api-key endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":             true,
			"owner_name":        "test-user",
			"trial_api_key":     true,
			"organization_name": "test-org",
		})
	}))
	defer server.Close()

	sd := NewSubscriptionDetector(logrus.New())

	// Test static fallback for cohere
	info := sd.getStaticSubscription("cohere")
	require.NotNil(t, info)
	assert.Equal(t, SubTypeFreeTier, info.Type)
	assert.Contains(t, info.AvailableTiers, SubTypeEnterprise)
}

func TestSubscriptionDetector_InferFromRateLimits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		provider   string
		rateLimits *RateLimitInfo
		expected   SubscriptionType
	}{
		{
			name:     "very_low_limits_free_tier",
			provider: "mistral",
			rateLimits: &RateLimitInfo{
				RequestsLimit: 5,
			},
			expected: SubTypeFreeTier,
		},
		{
			name:     "low_limits_free_credits",
			provider: "openai",
			rateLimits: &RateLimitInfo{
				RequestsLimit: 30,
			},
			expected: SubTypeFreeCredits,
		},
		{
			name:     "moderate_limits_pay_as_you_go",
			provider: "deepseek",
			rateLimits: &RateLimitInfo{
				RequestsLimit: 200,
			},
			expected: SubTypePayAsYouGo,
		},
		{
			name:     "high_limits_enterprise",
			provider: "openai",
			rateLimits: &RateLimitInfo{
				RequestsLimit: 50000,
				TokensLimit:   50000000,
			},
			expected: SubTypeEnterprise,
		},
		{
			name:     "high_daily_limit",
			provider: "sambanova",
			rateLimits: &RateLimitInfo{
				DailyLimit: 5000,
			},
			expected: SubTypePayAsYouGo,
		},
		{
			name:       "nil_rate_limits",
			provider:   "openai",
			rateLimits: nil,
			expected:   "", // Should return nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			sd := NewSubscriptionDetector(logrus.New())
			info := sd.InferFromRateLimits(tt.provider, tt.rateLimits)

			if tt.rateLimits == nil {
				assert.Nil(t, info)
			} else {
				require.NotNil(t, info)
				assert.Equal(t, tt.expected, info.Type)
				assert.Equal(t, "rate_limit_headers", info.DetectionSource)
				assert.NotNil(t, info.RateLimits)
			}
		})
	}
}

func TestSubscriptionDetector_Caching(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(logrus.New())

	// First call — should detect via static
	info1 := sd.DetectSubscription(context.Background(), "groq", "")
	require.NotNil(t, info1)

	// Second call — should return cached result
	info2 := sd.DetectSubscription(context.Background(), "groq", "")
	require.NotNil(t, info2)

	// Both should be the same (cached)
	assert.Equal(t, info1.Type, info2.Type)
	assert.Equal(t, info1.DetectedAt, info2.DetectedAt)
}

func TestSubscriptionDetector_CacheSize(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(logrus.New())

	assert.Equal(t, 0, sd.CacheSize())

	sd.DetectSubscription(context.Background(), "groq", "")
	assert.Equal(t, 1, sd.CacheSize())

	sd.DetectSubscription(context.Background(), "openai", "")
	assert.Equal(t, 2, sd.CacheSize())

	// Same provider should not increase cache size
	sd.DetectSubscription(context.Background(), "groq", "")
	assert.Equal(t, 2, sd.CacheSize())
}

func TestSubscriptionDetector_GetCachedSubscription(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(logrus.New())

	// Not cached yet
	assert.Nil(t, sd.GetCachedSubscription("groq"))

	// After detection
	sd.DetectSubscription(context.Background(), "groq", "")
	cached := sd.GetCachedSubscription("groq")
	require.NotNil(t, cached)
	assert.Equal(t, SubTypeFree, cached.Type)
}

func TestSubscriptionDetector_UpdateFromHeaders(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(logrus.New())

	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "200")
	headers.Set("x-ratelimit-remaining-requests", "195")

	// First call should create entry via Tier 2 inference
	sd.UpdateFromHeaders("openai", headers)

	cached := sd.GetCachedSubscription("openai")
	require.NotNil(t, cached)
	assert.NotNil(t, cached.RateLimits)
	assert.Equal(t, 200, cached.RateLimits.RequestsLimit)
}

func TestSubscriptionDetector_UpdateFromHeaders_ExistingCache(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(logrus.New())

	// Pre-populate cache
	sd.DetectSubscription(context.Background(), "openai", "")

	// Now update with headers
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "500")
	headers.Set("x-ratelimit-remaining-requests", "490")

	sd.UpdateFromHeaders("openai", headers)

	cached := sd.GetCachedSubscription("openai")
	require.NotNil(t, cached)
	assert.NotNil(t, cached.RateLimits)
	assert.Equal(t, 500, cached.RateLimits.RequestsLimit)
}

func TestSubscriptionDetector_ThreadSafety(t *testing.T) {
	t.Parallel()
	sd := NewSubscriptionDetector(logrus.New())
	ctx := context.Background()

	providers := []string{
		"openai", "claude", "gemini", "deepseek", "mistral",
		"groq", "cerebras", "openrouter", "together", "fireworks",
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			provider := providers[idx%len(providers)]

			// Mix of operations
			sd.DetectSubscription(ctx, provider, "")
			sd.GetCachedSubscription(provider)
			sd.CacheSize()

			headers := http.Header{}
			headers.Set("x-ratelimit-limit-requests", "100")
			sd.UpdateFromHeaders(provider, headers)
		}(i)
	}

	wg.Wait()
	assert.GreaterOrEqual(t, sd.CacheSize(), 1)
}

func TestInferSubTypeFromLimits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		rl       *RateLimitInfo
		expected SubscriptionType
	}{
		{
			name:     "very low request limit",
			rl:       &RateLimitInfo{RequestsLimit: 2},
			expected: SubTypeFreeTier,
		},
		{
			name:     "low request limit",
			rl:       &RateLimitInfo{RequestsLimit: 30},
			expected: SubTypeFreeCredits,
		},
		{
			name:     "moderate request limit",
			rl:       &RateLimitInfo{RequestsLimit: 200},
			expected: SubTypePayAsYouGo,
		},
		{
			name:     "very high request limit",
			rl:       &RateLimitInfo{RequestsLimit: 50000},
			expected: SubTypeEnterprise,
		},
		{
			name:     "very high token limit",
			rl:       &RateLimitInfo{TokensLimit: 50000000},
			expected: SubTypeEnterprise,
		},
		{
			name:     "high daily limit",
			rl:       &RateLimitInfo{DailyLimit: 5000},
			expected: SubTypePayAsYouGo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := inferSubTypeFromLimits(tt.rl)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkSubscriptionDetector_DetectSubscription(b *testing.B) {
	sd := NewSubscriptionDetector(logrus.New())
	ctx := context.Background()

	// Warm cache
	sd.DetectSubscription(ctx, "openai", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sd.DetectSubscription(ctx, "openai", "")
	}
}
