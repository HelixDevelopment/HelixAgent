package verifier

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRateLimitHeaders_OpenAI(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "200")
	headers.Set("x-ratelimit-remaining-requests", "195")
	headers.Set("x-ratelimit-limit-tokens", "40000")
	headers.Set("x-ratelimit-remaining-tokens", "39500")

	info := ParseRateLimitHeaders("openai", headers)
	require.NotNil(t, info)

	assert.Equal(t, 200, info.RequestsLimit)
	assert.Equal(t, 195, info.RequestsRemaining)
	assert.Equal(t, 40000, info.TokensLimit)
	assert.Equal(t, 39500, info.TokensRemaining)
	assert.False(t, info.UpdatedAt.IsZero())
}

func TestParseRateLimitHeaders_Anthropic(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "1000")
	headers.Set("anthropic-ratelimit-requests-remaining", "999")
	headers.Set("anthropic-ratelimit-tokens-limit", "100000")
	headers.Set("anthropic-ratelimit-tokens-remaining", "99500")

	info := ParseRateLimitHeaders("claude", headers)
	require.NotNil(t, info)

	assert.Equal(t, 1000, info.RequestsLimit)
	assert.Equal(t, 999, info.RequestsRemaining)
	assert.Equal(t, 100000, info.TokensLimit)
	assert.Equal(t, 99500, info.TokensRemaining)
}

func TestParseRateLimitHeaders_OpenRouter(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("X-RateLimit-Limit", "500")
	headers.Set("X-RateLimit-Remaining", "490")

	info := ParseRateLimitHeaders("openrouter", headers)
	require.NotNil(t, info)

	assert.Equal(t, 500, info.RequestsLimit)
	assert.Equal(t, 490, info.RequestsRemaining)
}

func TestParseRateLimitHeaders_SambaNova(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("X-RateLimit-Limit-RPM", "60")
	headers.Set("X-RateLimit-Limit-RPD", "1000")

	info := ParseRateLimitHeaders("sambanova", headers)
	require.NotNil(t, info)

	assert.Equal(t, 60, info.RequestsLimit)
	assert.Equal(t, 1000, info.DailyLimit)
}

func TestParseRateLimitHeaders_Groq(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "30")
	headers.Set("x-ratelimit-remaining-requests", "28")
	headers.Set("x-ratelimit-limit-tokens", "6000")
	headers.Set("x-ratelimit-remaining-tokens", "5800")

	info := ParseRateLimitHeaders("groq", headers)
	require.NotNil(t, info)

	assert.Equal(t, 30, info.RequestsLimit)
	assert.Equal(t, 28, info.RequestsRemaining)
	assert.Equal(t, 6000, info.TokensLimit)
}

func TestParseRateLimitHeaders_HuggingFace(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("RateLimit", "100")

	info := ParseRateLimitHeaders("huggingface", headers)
	require.NotNil(t, info)

	assert.Equal(t, 100, info.RequestsLimit)
}

func TestParseRateLimitHeaders_UnknownProvider(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("X-RateLimit-Limit", "100")
	headers.Set("X-RateLimit-Remaining", "90")

	// Unknown provider should use generic fallback headers
	info := ParseRateLimitHeaders("unknown_provider", headers)
	require.NotNil(t, info)

	assert.Equal(t, 100, info.RequestsLimit)
	assert.Equal(t, 90, info.RequestsRemaining)
}

func TestParseRateLimitHeaders_EmptyHeaders(t *testing.T) {
	t.Parallel()
	info := ParseRateLimitHeaders("openai", http.Header{})
	assert.Nil(t, info)
}

func TestParseRateLimitHeaders_NilHeaders(t *testing.T) {
	t.Parallel()
	info := ParseRateLimitHeaders("openai", nil)
	assert.Nil(t, info)
}

func TestParseRateLimitHeaders_WithTimestampReset(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "200")
	// Unix timestamp for reset
	headers.Set("x-ratelimit-reset-requests", "1739400000")

	info := ParseRateLimitHeaders("openai", headers)
	require.NotNil(t, info)

	assert.Equal(t, 200, info.RequestsLimit)
	assert.False(t, info.RequestsReset.IsZero(), "reset time should be parsed from Unix timestamp")
}

func TestParseRateLimitHeaders_WithRFC3339Reset(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "100")
	resetTime := time.Now().Add(1 * time.Minute).UTC().Format(time.RFC3339)
	headers.Set("anthropic-ratelimit-requests-reset", resetTime)

	info := ParseRateLimitHeaders("claude", headers)
	require.NotNil(t, info)

	assert.Equal(t, 100, info.RequestsLimit)
	assert.False(t, info.RequestsReset.IsZero(), "reset time should be parsed from RFC3339")
}

func TestParseHeaderInt(t *testing.T) {
	t.Parallel()
	headers := http.Header{}
	headers.Set("X-Limit", "100")
	headers.Set("X-Invalid", "abc")

	assert.Equal(t, 100, parseHeaderInt(headers, "X-Limit"))
	assert.Equal(t, 0, parseHeaderInt(headers, "X-Invalid"))
	assert.Equal(t, 0, parseHeaderInt(headers, "X-Missing"))
	assert.Equal(t, 0, parseHeaderInt(headers, ""))
}

func TestParseHeaderTime(t *testing.T) {
	t.Parallel()
	headers := http.Header{}

	// Unix timestamp
	headers.Set("X-Reset-Unix", "1739400000")
	t1 := parseHeaderTime(headers, "X-Reset-Unix")
	assert.False(t, t1.IsZero())

	// RFC3339
	now := time.Now().UTC().Format(time.RFC3339)
	headers.Set("X-Reset-RFC3339", now)
	t2 := parseHeaderTime(headers, "X-Reset-RFC3339")
	assert.False(t, t2.IsZero())

	// Duration
	headers.Set("X-Reset-Duration", "1m30s")
	t3 := parseHeaderTime(headers, "X-Reset-Duration")
	assert.False(t, t3.IsZero())

	// Invalid
	headers.Set("X-Reset-Invalid", "not-a-time")
	t4 := parseHeaderTime(headers, "X-Reset-Invalid")
	assert.True(t, t4.IsZero())

	// Missing
	t5 := parseHeaderTime(headers, "X-Missing")
	assert.True(t, t5.IsZero())

	// Empty name
	t6 := parseHeaderTime(headers, "")
	assert.True(t, t6.IsZero())
}

func TestRateLimitHeaderMap_Coverage(t *testing.T) {
	t.Parallel()
	// Verify known providers have entries
	expectedProviders := []string{"openai", "groq", "claude", "openrouter", "sambanova", "huggingface"}

	for _, provider := range expectedProviders {
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			headers, ok := RateLimitHeaderMap[provider]
			require.True(t, ok, "provider %s should have rate limit header mapping", provider)
			require.NotNil(t, headers)
			assert.NotEmpty(t, headers.RequestsLimit,
				"provider %s should have at least RequestsLimit header mapped", provider)
		})
	}

	assert.GreaterOrEqual(t, len(RateLimitHeaderMap), 6, "should have at least 6 entries")
}
