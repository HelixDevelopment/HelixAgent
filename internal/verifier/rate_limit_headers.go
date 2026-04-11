// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"net/http"
	"strconv"
	"time"
)

// RateLimitHeaderMap maps provider types to their specific rate limit header names.
// Providers use different header naming conventions, so this map normalizes the lookup.
var RateLimitHeaderMap = map[string]*RateLimitHeaderNames{
	"openai": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
		RequestsLimit:     "x-ratelimit-limit-requests",
		RequestsRemaining: "x-ratelimit-remaining-requests",
		RequestsReset:     "x-ratelimit-reset-requests",
		TokensLimit:       "x-ratelimit-limit-tokens",
		TokensRemaining:   "x-ratelimit-remaining-tokens",
		TokensReset:       "x-ratelimit-reset-tokens",
	},
	"groq": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
		RequestsLimit:     "x-ratelimit-limit-requests",
		RequestsRemaining: "x-ratelimit-remaining-requests",
		RequestsReset:     "x-ratelimit-reset-requests",
		TokensLimit:       "x-ratelimit-limit-tokens",
		TokensRemaining:   "x-ratelimit-remaining-tokens",
		TokensReset:       "x-ratelimit-reset-tokens",
	},
	"claude": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
		RequestsLimit:     "anthropic-ratelimit-requests-limit",
		RequestsRemaining: "anthropic-ratelimit-requests-remaining",
		RequestsReset:     "anthropic-ratelimit-requests-reset",
		TokensLimit:       "anthropic-ratelimit-tokens-limit",
		TokensRemaining:   "anthropic-ratelimit-tokens-remaining",
		TokensReset:       "anthropic-ratelimit-tokens-reset",
	},
	"openrouter": {
		RequestsLimit:     "X-RateLimit-Limit",
		RequestsRemaining: "X-RateLimit-Remaining",
		RequestsReset:     "X-RateLimit-Reset",
	},
	"sambanova": {
		RequestsLimit: "X-RateLimit-Limit-RPM",
		DailyLimit:    "X-RateLimit-Limit-RPD",
	},
	"huggingface": {
		RequestsLimit: "RateLimit",
	},
}

// ParseRateLimitHeaders parses rate limit information from HTTP response headers
// for a given provider type. Returns nil if no rate limit headers are found.
func ParseRateLimitHeaders(providerType string, headers http.Header) *RateLimitInfo {
	if headers == nil {
		return nil
	}

	// Look up provider-specific header names
	headerNames := RateLimitHeaderMap[providerType]
	if headerNames == nil {
		// Try generic header names as fallback
		headerNames = &RateLimitHeaderNames{
			RequestsLimit:     "X-RateLimit-Limit",
			RequestsRemaining: "X-RateLimit-Remaining",
			RequestsReset:     "X-RateLimit-Reset",
		}
	}

	info := &RateLimitInfo{
		UpdatedAt: time.Now(),
	}

	hasData := false

	// Parse request limits
	if v := parseHeaderInt(headers, headerNames.RequestsLimit); v > 0 {
		info.RequestsLimit = v
		hasData = true
	}
	if v := parseHeaderInt(headers, headerNames.RequestsRemaining); v > 0 {
		info.RequestsRemaining = v
		hasData = true
	}
	if t := parseHeaderTime(headers, headerNames.RequestsReset); !t.IsZero() {
		info.RequestsReset = t
		hasData = true
	}

	// Parse token limits
	if v := parseHeaderInt(headers, headerNames.TokensLimit); v > 0 {
		info.TokensLimit = v
		hasData = true
	}
	if v := parseHeaderInt(headers, headerNames.TokensRemaining); v > 0 {
		info.TokensRemaining = v
		hasData = true
	}
	if t := parseHeaderTime(headers, headerNames.TokensReset); !t.IsZero() {
		info.TokensReset = t
		hasData = true
	}

	// Parse daily limits
	if headerNames.DailyLimit != "" {
		if v := parseHeaderInt(headers, headerNames.DailyLimit); v > 0 {
			info.DailyLimit = v
			hasData = true
		}
	}
	if headerNames.DailyRemaining != "" {
		if v := parseHeaderInt(headers, headerNames.DailyRemaining); v > 0 {
			info.DailyRemaining = v
			hasData = true
		}
	}

	if !hasData {
		return nil
	}

	return info
}

// parseHeaderInt parses an integer from an HTTP header value.
// Returns 0 if the header is missing or cannot be parsed.
func parseHeaderInt(headers http.Header, name string) int {
	if name == "" {
		return 0
	}
	val := headers.Get(name)
	if val == "" {
		return 0
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return n
}

// parseHeaderTime parses a time value from an HTTP header.
// Supports Unix timestamps (seconds), RFC3339 strings, and duration strings.
// Returns zero time if the header is missing or cannot be parsed.
func parseHeaderTime(headers http.Header, name string) time.Time {
	if name == "" {
		return time.Time{}
	}
	val := headers.Get(name)
	if val == "" {
		return time.Time{}
	}

	// Try Unix timestamp (seconds)
	if n, err := strconv.ParseInt(val, 10, 64); err == nil && n > 1000000000 {
		return time.Unix(n, 0)
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t
	}

	// Try duration format (e.g., "1m30s", "200ms")
	if d, err := time.ParseDuration(val); err == nil {
		return time.Now().Add(d)
	}

	return time.Time{}
}
