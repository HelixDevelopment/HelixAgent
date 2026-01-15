// Package features provides feature configuration for HelixAgent.
// This file contains configuration structures and context management
// for feature flags across the application.
package features

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
)

// FeatureConfig holds the complete feature configuration
type FeatureConfig struct {
	// GlobalDefaults are the default settings for all requests
	GlobalDefaults map[Feature]bool `json:"global_defaults" yaml:"global_defaults"`

	// EndpointDefaults override global defaults for specific endpoints
	EndpointDefaults map[string]map[Feature]bool `json:"endpoint_defaults" yaml:"endpoint_defaults"`

	// AgentOverrides specify per-agent feature settings
	AgentOverrides map[string]map[Feature]bool `json:"agent_overrides" yaml:"agent_overrides"`

	// OpenAIEndpointGraphQL enables GraphQL by default for OpenAI-compatible endpoints
	OpenAIEndpointGraphQL bool `json:"openai_endpoint_graphql" yaml:"openai_endpoint_graphql"`

	// AllowFeatureHeaders allows clients to override features via headers
	AllowFeatureHeaders bool `json:"allow_feature_headers" yaml:"allow_feature_headers"`

	// AllowFeatureQueryParams allows clients to override features via query params
	AllowFeatureQueryParams bool `json:"allow_feature_query_params" yaml:"allow_feature_query_params"`

	// StrictValidation rejects requests with invalid feature combinations
	StrictValidation bool `json:"strict_validation" yaml:"strict_validation"`

	// LogFeatureUsage logs feature usage for analytics
	LogFeatureUsage bool `json:"log_feature_usage" yaml:"log_feature_usage"`
}

// DefaultFeatureConfig returns the default feature configuration
// that maintains backward compatibility with all CLI agents
func DefaultFeatureConfig() *FeatureConfig {
	registry := GetRegistry()
	globalDefaults := make(map[Feature]bool)

	// Copy all global defaults from registry
	for _, f := range registry.GetAllFeatures() {
		globalDefaults[f.Name] = f.DefaultValue
	}

	return &FeatureConfig{
		GlobalDefaults:          globalDefaults,
		EndpointDefaults:        make(map[string]map[Feature]bool),
		AgentOverrides:          make(map[string]map[Feature]bool),
		OpenAIEndpointGraphQL:   false, // Disabled by default for backward compatibility
		AllowFeatureHeaders:     true,  // Allow header-based feature toggling
		AllowFeatureQueryParams: true,  // Allow query param feature toggling
		StrictValidation:        false, // Be lenient by default
		LogFeatureUsage:         true,  // Log usage for analytics
	}
}

// FeatureContext holds the resolved feature settings for a request
type FeatureContext struct {
	// Features holds the enabled/disabled state of each feature
	Features map[Feature]bool `json:"features"`

	// AgentName is the detected or specified agent name
	AgentName string `json:"agent_name,omitempty"`

	// Source indicates where the feature settings came from
	Source FeatureSource `json:"source"`

	// Endpoint is the request endpoint
	Endpoint string `json:"endpoint,omitempty"`

	// RequestID for tracing
	RequestID string `json:"request_id,omitempty"`

	// mu protects Features map
	mu sync.RWMutex
}

// FeatureSource indicates where feature settings came from
type FeatureSource string

const (
	SourceGlobalDefault  FeatureSource = "global_default"
	SourceEndpointConfig FeatureSource = "endpoint_config"
	SourceAgentDetection FeatureSource = "agent_detection"
	SourceHeaderOverride FeatureSource = "header_override"
	SourceQueryOverride  FeatureSource = "query_override"
	SourceAPIOverride    FeatureSource = "api_override"
)

// NewFeatureContext creates a new feature context with default values
func NewFeatureContext() *FeatureContext {
	registry := GetRegistry()
	features := make(map[Feature]bool)

	for _, f := range registry.GetAllFeatures() {
		features[f.Name] = f.DefaultValue
	}

	return &FeatureContext{
		Features: features,
		Source:   SourceGlobalDefault,
	}
}

// NewFeatureContextFromConfig creates a context from configuration
func NewFeatureContextFromConfig(config *FeatureConfig, endpoint string) *FeatureContext {
	if config == nil {
		return NewFeatureContext()
	}

	features := make(map[Feature]bool)

	// Start with global defaults
	for k, v := range config.GlobalDefaults {
		features[k] = v
	}

	// Apply endpoint-specific defaults
	if endpointDefaults, ok := config.EndpointDefaults[endpoint]; ok {
		for k, v := range endpointDefaults {
			features[k] = v
		}
	}

	// Check for OpenAI endpoints - enable GraphQL if configured
	if config.OpenAIEndpointGraphQL && isOpenAIEndpoint(endpoint) {
		features[FeatureGraphQL] = true
		features[FeatureTOON] = true
	}

	return &FeatureContext{
		Features: features,
		Endpoint: endpoint,
		Source:   SourceEndpointConfig,
	}
}

// IsEnabled checks if a feature is enabled
func (fc *FeatureContext) IsEnabled(feature Feature) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	if enabled, ok := fc.Features[feature]; ok {
		return enabled
	}
	return false
}

// SetEnabled sets the enabled state of a feature
func (fc *FeatureContext) SetEnabled(feature Feature, enabled bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.Features[feature] = enabled
}

// EnableFeature enables a feature
func (fc *FeatureContext) EnableFeature(feature Feature) {
	fc.SetEnabled(feature, true)
}

// DisableFeature disables a feature
func (fc *FeatureContext) DisableFeature(feature Feature) {
	fc.SetEnabled(feature, false)
}

// GetEnabledFeatures returns a list of enabled features
func (fc *FeatureContext) GetEnabledFeatures() []Feature {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	var enabled []Feature
	for f, isEnabled := range fc.Features {
		if isEnabled {
			enabled = append(enabled, f)
		}
	}
	return enabled
}

// GetDisabledFeatures returns a list of disabled features
func (fc *FeatureContext) GetDisabledFeatures() []Feature {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	var disabled []Feature
	for f, isEnabled := range fc.Features {
		if !isEnabled {
			disabled = append(disabled, f)
		}
	}
	return disabled
}

// ApplyAgentCapabilities applies agent-specific capabilities to the context
func (fc *FeatureContext) ApplyAgentCapabilities(agentName string) {
	capRegistry := GetCapabilityRegistry()
	defaults := capRegistry.GetAgentFeatureDefaults(agentName)

	fc.mu.Lock()
	defer fc.mu.Unlock()

	for feature, enabled := range defaults {
		fc.Features[feature] = enabled
	}

	fc.AgentName = agentName
	fc.Source = SourceAgentDetection
}

// ApplyOverrides applies feature overrides from a map
func (fc *FeatureContext) ApplyOverrides(overrides map[Feature]bool, source FeatureSource) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	for feature, enabled := range overrides {
		fc.Features[feature] = enabled
	}
	fc.Source = source
}

// Clone creates a copy of the feature context
func (fc *FeatureContext) Clone() *FeatureContext {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	features := make(map[Feature]bool)
	for k, v := range fc.Features {
		features[k] = v
	}

	return &FeatureContext{
		Features:  features,
		AgentName: fc.AgentName,
		Source:    fc.Source,
		Endpoint:  fc.Endpoint,
		RequestID: fc.RequestID,
	}
}

// Validate checks if the current feature combination is valid
func (fc *FeatureContext) Validate() error {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	return GetRegistry().ValidateFeatureCombination(fc.Features)
}

// ToJSON serializes the context to JSON
func (fc *FeatureContext) ToJSON() ([]byte, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	return json.Marshal(fc)
}

// FromJSON deserializes the context from JSON
func (fc *FeatureContext) FromJSON(data []byte) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	return json.Unmarshal(data, fc)
}

// ToHeaders converts enabled features to HTTP headers
func (fc *FeatureContext) ToHeaders() map[string]string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	headers := make(map[string]string)
	registry := GetRegistry()

	for feature, enabled := range fc.Features {
		if info, ok := registry.GetFeature(feature); ok {
			if enabled {
				headers[info.HeaderName] = "true"
			}
		}
	}

	return headers
}

// GetStreamingMethod returns the preferred streaming method
func (fc *FeatureContext) GetStreamingMethod() string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// Priority: WebSocket > SSE > JSONL
	if fc.Features[FeatureWebSocket] {
		return "websocket"
	}
	if fc.Features[FeatureSSE] {
		return "sse"
	}
	if fc.Features[FeatureJSONL] {
		return "jsonl"
	}
	return "sse" // Default fallback
}

// GetCompressionMethod returns the preferred compression method
func (fc *FeatureContext) GetCompressionMethod() string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// Priority: Brotli > Zstd > Gzip > none
	if fc.Features[FeatureBrotli] {
		return "br"
	}
	if fc.Features[FeatureZstd] {
		return "zstd"
	}
	if fc.Features[FeatureGzip] {
		return "gzip"
	}
	return "" // No compression
}

// GetTransportProtocol returns the preferred transport protocol
func (fc *FeatureContext) GetTransportProtocol() string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// Priority: HTTP/3 > HTTP/2
	if fc.Features[FeatureHTTP3] {
		return "h3"
	}
	if fc.Features[FeatureHTTP2] {
		return "h2"
	}
	return "http/1.1"
}

// Context key for storing FeatureContext in context.Context
type featureContextKey struct{}

// WithFeatureContext adds a FeatureContext to a context
func WithFeatureContext(ctx context.Context, fc *FeatureContext) context.Context {
	return context.WithValue(ctx, featureContextKey{}, fc)
}

// GetFeatureContext retrieves a FeatureContext from a context
func GetFeatureContext(ctx context.Context) *FeatureContext {
	if fc, ok := ctx.Value(featureContextKey{}).(*FeatureContext); ok {
		return fc
	}
	return NewFeatureContext()
}

// isOpenAIEndpoint checks if the endpoint is an OpenAI-compatible endpoint
func isOpenAIEndpoint(endpoint string) bool {
	openAIEndpoints := []string{
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/embeddings",
		"/v1/models",
		"/v1/files",
		"/v1/images",
		"/v1/audio",
	}

	for _, e := range openAIEndpoints {
		if strings.HasPrefix(endpoint, e) {
			return true
		}
	}
	return false
}

// FeatureStats holds feature usage statistics
type FeatureStats struct {
	Feature       Feature `json:"feature"`
	EnabledCount  int64   `json:"enabled_count"`
	DisabledCount int64   `json:"disabled_count"`
	TotalRequests int64   `json:"total_requests"`
}

// FeatureUsageTracker tracks feature usage across requests
type FeatureUsageTracker struct {
	mu    sync.RWMutex
	stats map[Feature]*FeatureStats
}

// globalTracker is the singleton usage tracker
var globalTracker *FeatureUsageTracker
var trackerOnce sync.Once

// GetUsageTracker returns the global feature usage tracker
func GetUsageTracker() *FeatureUsageTracker {
	trackerOnce.Do(func() {
		globalTracker = &FeatureUsageTracker{
			stats: make(map[Feature]*FeatureStats),
		}
		// Initialize stats for all features
		for _, f := range GetRegistry().GetAllFeatures() {
			globalTracker.stats[f.Name] = &FeatureStats{
				Feature: f.Name,
			}
		}
	})
	return globalTracker
}

// RecordUsage records feature usage for a request
func (t *FeatureUsageTracker) RecordUsage(fc *FeatureContext) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for feature, enabled := range fc.Features {
		if stats, ok := t.stats[feature]; ok {
			stats.TotalRequests++
			if enabled {
				stats.EnabledCount++
			} else {
				stats.DisabledCount++
			}
		}
	}
}

// GetStats returns usage statistics for all features
func (t *FeatureUsageTracker) GetStats() []*FeatureStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := make([]*FeatureStats, 0, len(t.stats))
	for _, s := range t.stats {
		statCopy := *s
		stats = append(stats, &statCopy)
	}
	return stats
}

// GetFeatureStats returns statistics for a specific feature
func (t *FeatureUsageTracker) GetFeatureStats(feature Feature) *FeatureStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if stats, ok := t.stats[feature]; ok {
		statCopy := *stats
		return &statCopy
	}
	return nil
}

// ResetStats resets all usage statistics
func (t *FeatureUsageTracker) ResetStats() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, s := range t.stats {
		s.EnabledCount = 0
		s.DisabledCount = 0
		s.TotalRequests = 0
	}
}
