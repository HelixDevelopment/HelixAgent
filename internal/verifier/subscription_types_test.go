package verifier

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionType_Constants(t *testing.T) {
	t.Parallel()
	// Verify all subscription type constants are distinct and non-empty
	types := []SubscriptionType{
		SubTypeFree, SubTypeFreeCredits, SubTypeFreeTier,
		SubTypePayAsYouGo, SubTypeMonthly, SubTypeEnterprise,
	}

	seen := make(map[SubscriptionType]bool)
	for _, st := range types {
		assert.NotEmpty(t, string(st), "subscription type should not be empty")
		assert.False(t, seen[st], "duplicate subscription type: %s", st)
		seen[st] = true
	}

	assert.Len(t, seen, 6, "should have exactly 6 subscription types")
}

func TestSubscriptionType_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		st    SubscriptionType
		valid bool
	}{
		{"free", SubTypeFree, true},
		{"free_credits", SubTypeFreeCredits, true},
		{"free_tier", SubTypeFreeTier, true},
		{"pay_as_you_go", SubTypePayAsYouGo, true},
		{"monthly", SubTypeMonthly, true},
		{"enterprise", SubTypeEnterprise, true},
		{"unknown", SubscriptionType("unknown"), false},
		{"empty", SubscriptionType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tt.valid, tt.st.IsValid())
		})
	}
}

func TestSubscriptionType_IsFree(t *testing.T) {
	t.Parallel()
	assert.True(t, SubTypeFree.IsFree())
	assert.True(t, SubTypeFreeCredits.IsFree())
	assert.True(t, SubTypeFreeTier.IsFree())
	assert.False(t, SubTypePayAsYouGo.IsFree())
	assert.False(t, SubTypeMonthly.IsFree())
	assert.False(t, SubTypeEnterprise.IsFree())
}

func TestSubscriptionType_IsPaid(t *testing.T) {
	t.Parallel()
	assert.False(t, SubTypeFree.IsPaid())
	assert.False(t, SubTypeFreeCredits.IsPaid())
	assert.False(t, SubTypeFreeTier.IsPaid())
	assert.True(t, SubTypePayAsYouGo.IsPaid())
	assert.True(t, SubTypeMonthly.IsPaid())
	assert.True(t, SubTypeEnterprise.IsPaid())
}

func TestAuthMechanism_Fields(t *testing.T) {
	t.Parallel()
	am := AuthMechanism{
		HeaderName:     "Authorization",
		HeaderPrefix:   "Bearer ",
		QueryParam:     "key",
		ExtraHeaders:   map[string]string{"anthropic-version": "2023-06-01"},
		NoAuth:         false,
		DeviceIDHeader: "X-Device-ID",
	}

	assert.Equal(t, "Authorization", am.HeaderName)
	assert.Equal(t, "Bearer ", am.HeaderPrefix)
	assert.Equal(t, "key", am.QueryParam)
	assert.Equal(t, "2023-06-01", am.ExtraHeaders["anthropic-version"])
	assert.False(t, am.NoAuth)
	assert.Equal(t, "X-Device-ID", am.DeviceIDHeader)
}

func TestAuthMechanism_ApplyAuth_Bearer(t *testing.T) {
	t.Parallel()
	am := AuthMechanism{
		HeaderName:   "Authorization",
		HeaderPrefix: "Bearer ",
	}

	req, _ := http.NewRequest("GET", "https://api.example.com/v1/models", nil)
	am.ApplyAuth(req, "sk-test-key-123")

	assert.Equal(t, "Bearer sk-test-key-123", req.Header.Get("Authorization"))
}

func TestAuthMechanism_ApplyAuth_XAPIKey(t *testing.T) {
	t.Parallel()
	am := AuthMechanism{
		HeaderName: "x-api-key",
		ExtraHeaders: map[string]string{
			"anthropic-version": "2023-06-01",
		},
	}

	req, _ := http.NewRequest("GET", "https://api.anthropic.com/v1/models", nil)
	am.ApplyAuth(req, "sk-ant-key")

	assert.Equal(t, "sk-ant-key", req.Header.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", req.Header.Get("anthropic-version"))
}

func TestAuthMechanism_ApplyAuth_QueryParam(t *testing.T) {
	t.Parallel()
	am := AuthMechanism{
		QueryParam: "key",
	}

	u, _ := url.Parse("https://api.gemini.com/v1/models")
	req := &http.Request{URL: u, Header: http.Header{}}
	am.ApplyAuth(req, "gemini-key-123")

	assert.Equal(t, "gemini-key-123", req.URL.Query().Get("key"))
}

func TestAuthMechanism_ApplyAuth_NoAuth(t *testing.T) {
	t.Parallel()
	am := AuthMechanism{
		NoAuth:         true,
		DeviceIDHeader: "X-Device-ID",
	}

	req, _ := http.NewRequest("GET", "https://opencode.ai/zen/v1/models", nil)
	am.ApplyAuth(req, "device-123")

	assert.Equal(t, "device-123", req.Header.Get("X-Device-ID"))
	assert.Empty(t, req.Header.Get("Authorization"))
}

func TestSubscriptionInfo_JSONSerialization(t *testing.T) {
	t.Parallel()
	credits := 4.50
	expires := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	info := SubscriptionInfo{
		Type:             SubTypeFreeCredits,
		AvailableTiers:   []SubscriptionType{SubTypeFreeCredits, SubTypePayAsYouGo},
		DetectedAt:       time.Now(),
		DetectionSource:  "api",
		CreditsRemaining: &credits,
		CreditsCurrency:  "USD",
		CreditsExpiresAt: &expires,
		PlanName:         "free-trial",
		Restrictions:     []string{"rate limited", "limited models"},
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded SubscriptionInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, SubTypeFreeCredits, decoded.Type)
	assert.Equal(t, "api", decoded.DetectionSource)
	assert.NotNil(t, decoded.CreditsRemaining)
	assert.Equal(t, 4.50, *decoded.CreditsRemaining)
	assert.Equal(t, "USD", decoded.CreditsCurrency)
	assert.Equal(t, "free-trial", decoded.PlanName)
	assert.Len(t, decoded.Restrictions, 2)
	assert.Len(t, decoded.AvailableTiers, 2)
}

func TestRateLimitInfo_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	resetTime := now.Add(1 * time.Minute)

	info := RateLimitInfo{
		RequestsLimit:     100,
		RequestsRemaining: 95,
		RequestsReset:     resetTime,
		TokensLimit:       100000,
		TokensRemaining:   95000,
		TokensReset:       resetTime,
		DailyLimit:        1000,
		DailyRemaining:    950,
		UpdatedAt:         now,
	}

	assert.Equal(t, 100, info.RequestsLimit)
	assert.Equal(t, 95, info.RequestsRemaining)
	assert.Equal(t, 100000, info.TokensLimit)
	assert.Equal(t, 1000, info.DailyLimit)
	assert.False(t, info.UpdatedAt.IsZero())
}

func TestProviderAccessConfig_HasRateLimitHeaders(t *testing.T) {
	t.Parallel()
	// With rate limit headers
	config := &ProviderAccessConfig{
		RateLimitHeaders: &RateLimitHeaderNames{
			RequestsLimit: "x-ratelimit-limit-requests",
		},
	}
	assert.True(t, config.HasRateLimitHeaders())

	// Without rate limit headers
	config2 := &ProviderAccessConfig{}
	assert.False(t, config2.HasRateLimitHeaders())

	// With empty headers struct
	config3 := &ProviderAccessConfig{
		RateLimitHeaders: &RateLimitHeaderNames{},
	}
	assert.False(t, config3.HasRateLimitHeaders())
}

func TestProviderAccessConfig_HasSubscriptionCheckAPI(t *testing.T) {
	t.Parallel()
	config := &ProviderAccessConfig{
		SubscriptionCheckURL: "https://openrouter.ai/api/v1/auth/key",
	}
	assert.True(t, config.HasSubscriptionCheckAPI())

	config2 := &ProviderAccessConfig{}
	assert.False(t, config2.HasSubscriptionCheckAPI())
}
