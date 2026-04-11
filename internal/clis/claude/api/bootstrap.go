// Package api provides Bootstrap API implementation for Claude Code integration.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// BootstrapResponse represents bootstrap configuration
type BootstrapResponse struct {
	ClientData             ClientData    `json:"client_data"`
	AdditionalModelOptions []ModelOption `json:"additional_model_options,omitempty"`
}

// ClientData represents client-specific data
type ClientData struct {
	FeatureFlags   FeatureFlags   `json:"feature_flags"`
	Announcements  []Announcement `json:"announcements"`
	MinimumVersion string         `json:"minimum_version"`
}

// FeatureFlags represents all available feature flags
type FeatureFlags struct {
	// Model and inference flags
	NewModelsEnabled     bool `json:"new_models_enabled"`
	ExperimentalFeatures bool `json:"experimental_features"`
	Context1MEnabled     bool `json:"context_1m_enabled,omitempty"`
	InterleavedThinking  bool `json:"interleaved_thinking,omitempty"`
	StructuredOutputs    bool `json:"structured_outputs,omitempty"`
	WebSearchEnabled     bool `json:"web_search_enabled,omitempty"`
	FastModeEnabled      bool `json:"fast_mode_enabled,omitempty"`
	AFKModeEnabled       bool `json:"afk_mode_enabled,omitempty"`
	RedactThinking       bool `json:"redact_thinking,omitempty"`

	// Tool and MCP flags
	MCPEnabled           bool `json:"mcp_enabled,omitempty"`
	AdvancedToolsEnabled bool `json:"advanced_tools_enabled,omitempty"`
	CustomToolsEnabled   bool `json:"custom_tools_enabled,omitempty"`

	// UI and experience flags
	NewUIEnabled       bool `json:"new_ui_enabled,omitempty"`
	AnimationsEnabled  bool `json:"animations_enabled,omitempty"`
	BuddySystemEnabled bool `json:"buddy_system_enabled,omitempty"`
	KAIROSEnabled      bool `json:"kairos_enabled,omitempty"`
	DreamSystemEnabled bool `json:"dream_system_enabled,omitempty"`

	// Billing and subscription flags
	ExtraUsageEnabled   bool `json:"extra_usage_enabled,omitempty"`
	GroveEnabled        bool `json:"grove_enabled,omitempty"`
	TeamFeaturesEnabled bool `json:"team_features_enabled,omitempty"`
	EnterpriseFeatures  bool `json:"enterprise_features_enabled,omitempty"`

	// Security and privacy flags
	YOLOClassifierEnabled bool `json:"yolo_classifier_enabled,omitempty"`
	PermissionSystemV2    bool `json:"permission_system_v2,omitempty"`
	PrivacyModeEnabled    bool `json:"privacy_mode_enabled,omitempty"`

	// Beta features
	BetaToolsEnabled    bool `json:"beta_tools_enabled,omitempty"`
	BetaAPIsEnabled     bool `json:"beta_apis_enabled,omitempty"`
	EarlyAccessFeatures bool `json:"early_access_features,omitempty"`
}

// Announcement represents a system announcement
type Announcement struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"` // "info", "warning", "critical"
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	URL         string     `json:"url,omitempty"`
	PublishedAt time.Time  `json:"published_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// ModelOption represents an available model option
type ModelOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Beta        bool   `json:"beta,omitempty"`
	Deprecated  bool   `json:"deprecated,omitempty"`
}

// GetBootstrap fetches bootstrap configuration and feature flags
func (c *Client) GetBootstrap(ctx context.Context) (*BootstrapResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/claude_cli/bootstrap", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result BootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// BootstrapCache provides caching for bootstrap data
type BootstrapCache struct {
	mu        sync.RWMutex
	data      *BootstrapResponse
	fetchedAt time.Time
	ttl       time.Duration
	client    *Client
}

// NewBootstrapCache creates a new bootstrap cache
func NewBootstrapCache(client *Client, ttl time.Duration) *BootstrapCache {
	return &BootstrapCache{
		ttl:    ttl,
		client: client,
	}
}

// Get retrieves bootstrap data (from cache or fresh)
func (c *BootstrapCache) Get(ctx context.Context) (*BootstrapResponse, error) {
	c.mu.RLock()
	if c.data != nil && time.Since(c.fetchedAt) < c.ttl {
		data := c.data
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.data != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.data, nil
	}

	data, err := c.client.GetBootstrap(ctx)
	if err != nil {
		return nil, err
	}

	c.data = data
	c.fetchedAt = time.Now()

	return data, nil
}

// Invalidate clears the cache
func (c *BootstrapCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = nil
}

// IsStale returns true if the cache is stale
func (c *BootstrapCache) IsStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data == nil || time.Since(c.fetchedAt) >= c.ttl
}

// LastFetched returns when the cache was last updated
func (c *BootstrapCache) LastFetched() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fetchedAt
}

// FeatureFlagManager provides convenient access to feature flags
type FeatureFlagManager struct {
	cache *BootstrapCache
}

// NewFeatureFlagManager creates a new feature flag manager
func NewFeatureFlagManager(cache *BootstrapCache) *FeatureFlagManager {
	return &FeatureFlagManager{cache: cache}
}

// GetFlags retrieves current feature flags
func (m *FeatureFlagManager) GetFlags(ctx context.Context) (*FeatureFlags, error) {
	bootstrap, err := m.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &bootstrap.ClientData.FeatureFlags, nil
}

// IsEnabled checks if a specific feature is enabled
func (m *FeatureFlagManager) IsEnabled(ctx context.Context, feature string) (bool, error) {
	flags, err := m.GetFlags(ctx)
	if err != nil {
		return false, err
	}

	switch feature {
	case "new_models":
		return flags.NewModelsEnabled, nil
	case "experimental":
		return flags.ExperimentalFeatures, nil
	case "context_1m":
		return flags.Context1MEnabled, nil
	case "interleaved_thinking":
		return flags.InterleavedThinking, nil
	case "structured_outputs":
		return flags.StructuredOutputs, nil
	case "web_search":
		return flags.WebSearchEnabled, nil
	case "fast_mode":
		return flags.FastModeEnabled, nil
	case "mcp":
		return flags.MCPEnabled, nil
	case "buddy":
		return flags.BuddySystemEnabled, nil
	case "kairos":
		return flags.KAIROSEnabled, nil
	case "dream":
		return flags.DreamSystemEnabled, nil
	case "extra_usage":
		return flags.ExtraUsageEnabled, nil
	case "grove":
		return flags.GroveEnabled, nil
	default:
		return false, fmt.Errorf("unknown feature flag: %s", feature)
	}
}

// RequireVersion checks if the current version meets minimum requirements
func (m *FeatureFlagManager) RequireVersion(ctx context.Context, currentVersion string) error {
	bootstrap, err := m.cache.Get(ctx)
	if err != nil {
		return err
	}

	minVersion := bootstrap.ClientData.MinimumVersion
	if minVersion == "" {
		return nil
	}

	// Simple version comparison (can be enhanced with semver library)
	if currentVersion < minVersion {
		return fmt.Errorf("version %s is below minimum required version %s",
			currentVersion, minVersion)
	}

	return nil
}

// GetAnnouncements retrieves active announcements
func (m *FeatureFlagManager) GetAnnouncements(ctx context.Context) ([]Announcement, error) {
	bootstrap, err := m.cache.Get(ctx)
	if err != nil {
		return nil, err
	}

	// Filter out expired announcements
	var active []Announcement
	now := time.Now()
	for _, ann := range bootstrap.ClientData.Announcements {
		if ann.ExpiresAt == nil || now.Before(*ann.ExpiresAt) {
			active = append(active, ann)
		}
	}

	return active, nil
}

// GetModelOptions retrieves available model options
func (m *FeatureFlagManager) GetModelOptions(ctx context.Context) ([]ModelOption, error) {
	bootstrap, err := m.cache.Get(ctx)
	if err != nil {
		return nil, err
	}
	return bootstrap.AdditionalModelOptions, nil
}

// GetRecommendedModel returns the recommended model for the user's subscription
func (m *FeatureFlagManager) GetRecommendedModel(ctx context.Context, subscriptionTier string) (string, error) {
	options, err := m.GetModelOptions(ctx)
	if err != nil {
		return "", err
	}

	if len(options) == 0 {
		return ModelClaudeSonnet4_5, nil // Default
	}

	// Return first non-deprecated option
	for _, opt := range options {
		if !opt.Deprecated {
			return opt.Value, nil
		}
	}

	return options[0].Value, nil
}

// DefaultFeatureFlags returns default feature flags for offline use
func DefaultFeatureFlags() *FeatureFlags {
	return &FeatureFlags{
		NewModelsEnabled:     true,
		ExperimentalFeatures: false,
		Context1MEnabled:     false,
		InterleavedThinking:  false,
		StructuredOutputs:    false,
		WebSearchEnabled:     false,
		FastModeEnabled:      false,
		MCPEnabled:           true,
		AdvancedToolsEnabled: true,
		BuddySystemEnabled:   false,
		KAIROSEnabled:        false,
		DreamSystemEnabled:   false,
		ExtraUsageEnabled:    false,
		GroveEnabled:         true,
	}
}
