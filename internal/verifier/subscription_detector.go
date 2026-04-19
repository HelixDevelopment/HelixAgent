// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// SubscriptionDetector performs 3-tier subscription detection for providers.
// Tier 1: Query provider-specific APIs (OpenRouter /v1/auth/key, Cohere /check-api-key)
// Tier 2: Infer from rate limit headers in API responses
// Tier 3: Static fallback from ProviderAccessRegistry
//
// Concurrent-safe by construction (CONST-029): cache is a safe.Store.
// UpdateFromHeaders mutates *SubscriptionInfo fields under Update
// callback (Pattern Beta).
type SubscriptionDetector struct {
	cache    *safe.Store[string, *SubscriptionInfo]
	cacheTTL time.Duration
	client   *http.Client
	log      *logrus.Logger
}

// NewSubscriptionDetector creates a new subscription detector with default settings.
func NewSubscriptionDetector(log *logrus.Logger) *SubscriptionDetector {
	if log == nil {
		log = logrus.New()
	}
	return &SubscriptionDetector{
		cache:    safe.NewStore[string, *SubscriptionInfo](),
		cacheTTL: 1 * time.Hour,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		log: log,
	}
}

// DetectSubscription performs 3-tier subscription detection for a provider.
// Returns the best available subscription info, falling back through tiers.
func (sd *SubscriptionDetector) DetectSubscription(ctx context.Context, providerType, apiKey string) *SubscriptionInfo {
	// Check cache first — copy out under Update so concurrent
	// UpdateFromHeaders cannot race on the *SubscriptionInfo fields.
	if snapshot := sd.GetCachedSubscription(providerType); snapshot != nil {
		return snapshot
	}

	// Tier 1: Try provider-specific API detection
	if apiKey != "" {
		if info := sd.detectViaAPI(ctx, providerType, apiKey); info != nil {
			sd.cacheResult(providerType, info)
			sd.log.WithFields(logrus.Fields{
				"provider": providerType,
				"type":     info.Type,
				"source":   "api",
			}).Debug("Subscription detected via API")
			return info
		}
	}

	// Tier 2: skipped here — InferFromRateLimits is called separately after API calls
	// (rate limit headers are only available after making an API request)

	// Tier 3: Static fallback
	info := sd.getStaticSubscription(providerType)
	if info != nil {
		sd.cacheResult(providerType, info)
		sd.log.WithFields(logrus.Fields{
			"provider": providerType,
			"type":     info.Type,
			"source":   "static",
		}).Debug("Subscription detected via static fallback")
	}
	return info
}

// detectViaAPI attempts Tier 1 detection using provider-specific APIs
func (sd *SubscriptionDetector) detectViaAPI(ctx context.Context, providerType, apiKey string) *SubscriptionInfo {
	config := GetProviderAccessConfig(providerType)
	if config == nil || !config.HasSubscriptionCheckAPI() {
		return nil
	}

	switch providerType {
	case "openrouter":
		return sd.detectOpenRouterSubscription(ctx, apiKey)
	case "cohere":
		return sd.detectCohereSubscription(ctx, apiKey)
	default:
		return nil
	}
}

// detectOpenRouterSubscription queries OpenRouter's /v1/auth/key endpoint
func (sd *SubscriptionDetector) detectOpenRouterSubscription(ctx context.Context, apiKey string) *SubscriptionInfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/auth/key", nil)
	if err != nil {
		sd.log.WithError(err).Debug("Failed to create OpenRouter auth request")
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := sd.client.Do(req)
	if err != nil {
		sd.log.WithError(err).Debug("Failed to query OpenRouter auth endpoint")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result struct {
		Data struct {
			Label      string  `json:"label"`
			Usage      float64 `json:"usage"`
			Limit      float64 `json:"limit"`
			IsFreeTier bool    `json:"is_free_tier"`
			RateLimit  struct {
				Requests int    `json:"requests"`
				Interval string `json:"interval"`
			} `json:"rate_limit"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		sd.log.WithError(err).Debug("Failed to decode OpenRouter auth response")
		return nil
	}

	subType := SubTypePayAsYouGo
	if result.Data.IsFreeTier {
		subType = SubTypeFreeTier
	}

	credits := result.Data.Limit - result.Data.Usage
	info := &SubscriptionInfo{
		Type:             subType,
		AvailableTiers:   []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo},
		DetectedAt:       time.Now(),
		DetectionSource:  "api",
		CreditsRemaining: &credits,
		CreditsCurrency:  "USD",
		PlanName:         result.Data.Label,
	}

	if result.Data.IsFreeTier {
		info.Restrictions = []string{"limited to free models", "lower rate limits"}
	}

	return info
}

// detectCohereSubscription queries Cohere's /check-api-key endpoint
func (sd *SubscriptionDetector) detectCohereSubscription(ctx context.Context, apiKey string) *SubscriptionInfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.cohere.ai/check-api-key", nil)
	if err != nil {
		sd.log.WithError(err).Debug("Failed to create Cohere check request")
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := sd.client.Do(req)
	if err != nil {
		sd.log.WithError(err).Debug("Failed to query Cohere check endpoint")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result struct {
		Valid       bool   `json:"valid"`
		OwnerName   string `json:"owner_name"`
		TrialAPIKey bool   `json:"trial_api_key"`
		OrgName     string `json:"organization_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		sd.log.WithError(err).Debug("Failed to decode Cohere check response")
		return nil
	}

	if !result.Valid {
		return nil
	}

	subType := SubTypePayAsYouGo
	if result.TrialAPIKey {
		subType = SubTypeFreeTier
	}

	info := &SubscriptionInfo{
		Type:            subType,
		AvailableTiers:  []SubscriptionType{SubTypeFreeTier, SubTypePayAsYouGo, SubTypeEnterprise},
		DetectedAt:      time.Now(),
		DetectionSource: "api",
		PlanName:        fmt.Sprintf("%s (%s)", result.OrgName, result.OwnerName),
	}

	if result.TrialAPIKey {
		info.Restrictions = []string{"trial key", "rate limited"}
	}

	return info
}

// InferFromRateLimits performs Tier 2 detection by inferring subscription type
// from rate limit header values. Called after an API response is received.
func (sd *SubscriptionDetector) InferFromRateLimits(providerType string, rateLimits *RateLimitInfo) *SubscriptionInfo {
	if rateLimits == nil {
		return nil
	}

	config := GetProviderAccessConfig(providerType)
	availableTiers := []SubscriptionType{SubTypePayAsYouGo}
	if config != nil {
		availableTiers = config.AvailableTiers
	}

	// Infer subscription type from rate limit values
	subType := inferSubTypeFromLimits(rateLimits)

	info := &SubscriptionInfo{
		Type:            subType,
		AvailableTiers:  availableTiers,
		DetectedAt:      time.Now(),
		DetectionSource: "rate_limit_headers",
		RateLimits:      rateLimits,
	}

	sd.cacheResult(providerType, info)

	sd.log.WithFields(logrus.Fields{
		"provider":      providerType,
		"type":          subType,
		"requestsLimit": rateLimits.RequestsLimit,
		"tokensLimit":   rateLimits.TokensLimit,
	}).Debug("Subscription inferred from rate limits")

	return info
}

// inferSubTypeFromLimits infers subscription type based on rate limit thresholds
func inferSubTypeFromLimits(rl *RateLimitInfo) SubscriptionType {
	// Very low request limits suggest free tier
	if rl.RequestsLimit > 0 && rl.RequestsLimit <= 10 {
		return SubTypeFreeTier
	}

	// Low-to-medium limits suggest free credits or free tier
	if rl.RequestsLimit > 0 && rl.RequestsLimit <= 60 {
		return SubTypeFreeCredits
	}

	// High daily limits suggest pay-as-you-go
	if rl.DailyLimit > 0 && rl.DailyLimit > 1000 {
		return SubTypePayAsYouGo
	}

	// Very high limits suggest enterprise
	if rl.RequestsLimit > 10000 || rl.TokensLimit > 10000000 {
		return SubTypeEnterprise
	}

	// Default to pay-as-you-go for moderate limits
	if rl.RequestsLimit > 60 {
		return SubTypePayAsYouGo
	}

	return SubTypePayAsYouGo
}

// getStaticSubscription returns Tier 3 static fallback subscription info
func (sd *SubscriptionDetector) getStaticSubscription(providerType string) *SubscriptionInfo {
	config := GetProviderAccessConfig(providerType)
	if config == nil {
		return &SubscriptionInfo{
			Type:            SubTypePayAsYouGo,
			AvailableTiers:  []SubscriptionType{SubTypePayAsYouGo},
			DetectedAt:      time.Now(),
			DetectionSource: "static",
		}
	}

	return &SubscriptionInfo{
		Type:            config.DefaultSubscription,
		AvailableTiers:  config.AvailableTiers,
		DetectedAt:      time.Now(),
		DetectionSource: "static",
	}
}

// UpdateFromHeaders updates the cached subscription info from API response headers.
// This is the entry point for Tier 2 detection during normal API operation.
func (sd *SubscriptionDetector) UpdateFromHeaders(providerType string, headers http.Header) {
	rateLimits := ParseRateLimitHeaders(providerType, headers)
	if rateLimits == nil {
		return
	}

	// Check if we have an existing entry to update — atomic via Update.
	var hadExisting bool
	sd.cache.Update(providerType, func(existing *SubscriptionInfo, ok bool) (*SubscriptionInfo, bool) {
		if !ok {
			return nil, false
		}
		hadExisting = true
		existing.RateLimits = rateLimits
		existing.DetectedAt = time.Now()
		return existing, true
	})
	if hadExisting {
		return
	}

	// No existing entry — infer from rate limits (Tier 2)
	sd.InferFromRateLimits(providerType, rateLimits)
}

// GetCachedSubscription returns a snapshot of the cached subscription info
// for a provider. Returns nil if not cached or expired. The snapshot is a
// value copy taken under the Store's write lock so concurrent
// UpdateFromHeaders cannot race on the *SubscriptionInfo fields
// (Pattern Beta — read-and-copy via Update).
func (sd *SubscriptionDetector) GetCachedSubscription(providerType string) *SubscriptionInfo {
	var snapshot *SubscriptionInfo
	sd.cache.Update(providerType, func(cached *SubscriptionInfo, ok bool) (*SubscriptionInfo, bool) {
		if !ok {
			return nil, false
		}
		if time.Since(cached.DetectedAt) > sd.cacheTTL {
			return cached, true // keep
		}
		copy := *cached
		snapshot = &copy
		return cached, true
	})
	return snapshot
}

// cacheResult stores subscription info in the cache
func (sd *SubscriptionDetector) cacheResult(providerType string, info *SubscriptionInfo) {
	sd.cache.Put(providerType, info)
}

// CacheSize returns the number of cached subscription entries
func (sd *SubscriptionDetector) CacheSize() int {
	return sd.cache.Len()
}
