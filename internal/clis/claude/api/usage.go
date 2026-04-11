// Package api provides Usage API implementation for Claude Code integration.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// UsageResponse represents usage and rate limit information
type UsageResponse struct {
	FiveHour          *UsageWindow `json:"five_hour"`
	SevenDay          *UsageWindow `json:"seven_day"`
	SevenDayOAuthApps *UsageWindow `json:"seven_day_oauth_apps,omitempty"`
	SevenDayOpus      *UsageWindow `json:"seven_day_opus,omitempty"`
	SevenDaySonnet    *UsageWindow `json:"seven_day_sonnet,omitempty"`
	ExtraUsage        *ExtraUsage  `json:"extra_usage,omitempty"`
}

// UsageWindow represents a usage window
type UsageWindow struct {
	Utilization int       `json:"utilization"` // Percentage (0-100)
	ResetsAt    time.Time `json:"resets_at"`
}

// ExtraUsage represents extra usage credit information
type ExtraUsage struct {
	IsEnabled    bool `json:"is_enabled"`
	MonthlyLimit int  `json:"monthly_limit"`
	UsedCredits  int  `json:"used_credits"`
	Utilization  int  `json:"utilization"`
}

// GetUsage fetches rate limit and usage information
func (c *Client) GetUsage(ctx context.Context) (*UsageResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/oauth/usage", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result UsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// AccountSettings represents user account settings
type AccountSettings struct {
	GroveEnabled        bool                `json:"grove_enabled"`
	GroveNoticeViewedAt *time.Time          `json:"grove_notice_viewed_at,omitempty"`
	PrivacyPreferences  *PrivacyPreferences `json:"privacy_preferences,omitempty"`
}

// PrivacyPreferences represents privacy-related settings
type PrivacyPreferences struct {
	AnalyticsEnabled bool `json:"analytics_enabled"`
	DataSharing      bool `json:"data_sharing"`
}

// GetAccountSettings gets user account settings
func (c *Client) GetAccountSettings(ctx context.Context) (*AccountSettings, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/oauth/account/settings", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result AccountSettings
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// UpdateAccountSettings updates user account settings
func (c *Client) UpdateAccountSettings(ctx context.Context, settings *AccountSettings) error {
	resp, err := c.doRequest(ctx, "PATCH", "/api/oauth/account/settings", settings)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return handleErrorResponse(resp)
	}

	return nil
}

// MarkGroveNoticeViewed marks the Grove notice as viewed
func (c *Client) MarkGroveNoticeViewed(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "POST", "/api/oauth/account/grove_notice_viewed", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return handleErrorResponse(resp)
	}

	return nil
}

// GroveConfig represents Grove configuration
type GroveConfig struct {
	GroveEnabled            bool `json:"grove_enabled"`
	DomainExcluded          bool `json:"domain_excluded"`
	NoticeIsGracePeriod     bool `json:"notice_is_grace_period"`
	NoticeReminderFrequency *int `json:"notice_reminder_frequency,omitempty"`
}

// GetGroveConfig gets Grove configuration and eligibility
func (c *Client) GetGroveConfig(ctx context.Context) (*GroveConfig, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/claude_code_grove", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result GroveConfig
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// UsageTracker tracks usage across multiple requests
type UsageTracker struct {
	TotalInputTokens  int
	TotalOutputTokens int
	Requests          int
	Windows           []UsageWindow
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		Windows: make([]UsageWindow, 0),
	}
}

// TrackRequest tracks token usage from a response
func (t *UsageTracker) TrackRequest(usage Usage) {
	t.TotalInputTokens += usage.InputTokens
	t.TotalOutputTokens += usage.OutputTokens
	t.Requests++
}

// TotalTokens returns the total tokens used
func (t *UsageTracker) TotalTokens() int {
	return t.TotalInputTokens + t.TotalOutputTokens
}

// AverageTokensPerRequest returns the average tokens per request
func (t *UsageTracker) AverageTokensPerRequest() float64 {
	if t.Requests == 0 {
		return 0
	}
	return float64(t.TotalTokens()) / float64(t.Requests)
}

// CostEstimate estimates the cost based on token usage
// Note: These are example rates and should be updated with actual pricing
func (t *UsageTracker) CostEstimate(model string) float64 {
	// Example pricing per 1K tokens (should be updated with actual rates)
	pricing := map[string]struct {
		Input  float64
		Output float64
	}{
		ModelClaudeOpus4_1:   {Input: 0.015, Output: 0.075},
		ModelClaudeSonnet4_5: {Input: 0.003, Output: 0.015},
		ModelClaudeSonnet4_6: {Input: 0.003, Output: 0.015},
		ModelClaudeHaiku4_5:  {Input: 0.00025, Output: 0.00125},
	}

	prices, ok := pricing[model]
	if !ok {
		// Default to sonnet pricing if model not found
		prices = pricing[ModelClaudeSonnet4_5]
	}

	inputCost := float64(t.TotalInputTokens) / 1000 * prices.Input
	outputCost := float64(t.TotalOutputTokens) / 1000 * prices.Output

	return inputCost + outputCost
}

// RateLimitChecker helps check rate limits before making requests
type RateLimitChecker struct {
	usage         *UsageResponse
	lastCheck     time.Time
	checkInterval time.Duration
}

// NewRateLimitChecker creates a new rate limit checker
func NewRateLimitChecker() *RateLimitChecker {
	return &RateLimitChecker{
		checkInterval: 5 * time.Minute,
	}
}

// UpdateUsage updates the cached usage information
func (r *RateLimitChecker) UpdateUsage(usage *UsageResponse) {
	r.usage = usage
	r.lastCheck = time.Now()
}

// ShouldCheck returns true if it's time to check rate limits again
func (r *RateLimitChecker) ShouldCheck() bool {
	return time.Since(r.lastCheck) > r.checkInterval
}

// IsRateLimited checks if any rate limit window is exceeded
func (r *RateLimitChecker) IsRateLimited(threshold int) bool {
	if r.usage == nil {
		return false
	}

	windows := []*UsageWindow{
		r.usage.FiveHour,
		r.usage.SevenDay,
		r.usage.SevenDayOpus,
		r.usage.SevenDaySonnet,
	}

	for _, window := range windows {
		if window != nil && window.Utilization >= threshold {
			return true
		}
	}

	return false
}

// GetMostRestrictedWindow returns the window with highest utilization
func (r *RateLimitChecker) GetMostRestrictedWindow() (*UsageWindow, string) {
	if r.usage == nil {
		return nil, ""
	}

	windows := map[string]*UsageWindow{
		"5-hour":       r.usage.FiveHour,
		"7-day":        r.usage.SevenDay,
		"7-day-opus":   r.usage.SevenDayOpus,
		"7-day-sonnet": r.usage.SevenDaySonnet,
	}

	var maxUtil int
	var maxWindow *UsageWindow
	var maxName string

	for name, window := range windows {
		if window != nil && window.Utilization > maxUtil {
			maxUtil = window.Utilization
			maxWindow = window
			maxName = name
		}
	}

	return maxWindow, maxName
}
