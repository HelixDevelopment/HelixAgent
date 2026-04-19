// Package features provides feature configuration for HelixAgent.
// This file contains configuration structures and context management
// for feature flags across the application.
package features

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"digital.vasic.concurrency/pkg/safe"
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

// FeatureContext holds the resolved feature settings for a request.
//
// Concurrent-safe by construction (CONST-029): Features is a safe.Store.
// JSON marshalling round-trips via the lowercase "features" object key,
// preserving the wire format. The exported Features field type changed
// from map[Feature]bool to *safe.Store[Feature, bool] — callers that
// iterated the map should now use Snapshot()/Range().
type FeatureContext struct {
	// Features holds the enabled/disabled state of each feature.
	Features *safe.Store[Feature, bool] `json:"-"`

	// AgentName is the detected or specified agent name
	AgentName string `json:"agent_name,omitempty"`

	// Source indicates where the feature settings came from
	Source FeatureSource `json:"source"`

	// Endpoint is the request endpoint
	Endpoint string `json:"endpoint,omitempty"`

	// RequestID for tracing
	RequestID string `json:"request_id,omitempty"`
}

// snapshotFeatures returns a point-in-time copy of the feature map.
// Callers iterate the snapshot lock-free.
func (fc *FeatureContext) snapshotFeatures() map[Feature]bool {
	if fc.Features == nil {
		return nil
	}
	return fc.Features.Snapshot()
}

// MarshalJSON emits the features map alongside the scalar fields under the
// "features" key (preserves the pre-migration JSON shape).
func (fc *FeatureContext) MarshalJSON() ([]byte, error) {
	type alias FeatureContext
	return json.Marshal(struct {
		Features map[Feature]bool `json:"features"`
		*alias
	}{
		Features: fc.snapshotFeatures(),
		alias:    (*alias)(fc),
	})
}

// UnmarshalJSON restores the features map from the wire format and
// rebuilds the safe.Store.
func (fc *FeatureContext) UnmarshalJSON(data []byte) error {
	type alias FeatureContext
	aux := &struct {
		Features map[Feature]bool `json:"features"`
		*alias
	}{
		alias: (*alias)(fc),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	fc.Features = safe.NewStore[Feature, bool]()
	for k, v := range aux.Features {
		fc.Features.Put(k, v)
	}
	return nil
}

// featuresFromMap builds a fresh safe.Store from a plain map (helper
// for the various NewFeatureContext constructors).
func featuresFromMap(m map[Feature]bool) *safe.Store[Feature, bool] {
	store := safe.NewStore[Feature, bool]()
	for k, v := range m {
		store.Put(k, v)
	}
	return store
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
		Features: featuresFromMap(features),
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
		Features: featuresFromMap(features),
		Endpoint: endpoint,
		Source:   SourceEndpointConfig,
	}
}

// IsEnabled checks if a feature is enabled
func (fc *FeatureContext) IsEnabled(feature Feature) bool {
	enabled, _ := fc.Features.Get(feature)
	return enabled
}

// SetEnabled sets the enabled state of a feature
func (fc *FeatureContext) SetEnabled(feature Feature, enabled bool) {
	fc.Features.Put(feature, enabled)
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
	var enabled []Feature
	fc.Features.Range(func(f Feature, isEnabled bool) bool {
		if isEnabled {
			enabled = append(enabled, f)
		}
		return true
	})
	return enabled
}

// GetDisabledFeatures returns a list of disabled features
func (fc *FeatureContext) GetDisabledFeatures() []Feature {
	var disabled []Feature
	fc.Features.Range(func(f Feature, isEnabled bool) bool {
		if !isEnabled {
			disabled = append(disabled, f)
		}
		return true
	})
	return disabled
}

// ApplyAgentCapabilities applies agent-specific capabilities to the context
func (fc *FeatureContext) ApplyAgentCapabilities(agentName string) {
	capRegistry := GetCapabilityRegistry()
	defaults := capRegistry.GetAgentFeatureDefaults(agentName)

	for feature, enabled := range defaults {
		fc.Features.Put(feature, enabled)
	}

	fc.AgentName = agentName
	fc.Source = SourceAgentDetection
}

// ApplyOverrides applies feature overrides from a map
func (fc *FeatureContext) ApplyOverrides(overrides map[Feature]bool, source FeatureSource) {
	for feature, enabled := range overrides {
		fc.Features.Put(feature, enabled)
	}
	fc.Source = source
}

// Clone creates a copy of the feature context
func (fc *FeatureContext) Clone() *FeatureContext {
	return &FeatureContext{
		Features:  featuresFromMap(fc.snapshotFeatures()),
		AgentName: fc.AgentName,
		Source:    fc.Source,
		Endpoint:  fc.Endpoint,
		RequestID: fc.RequestID,
	}
}

// Validate checks if the current feature combination is valid
func (fc *FeatureContext) Validate() error {
	return GetRegistry().ValidateFeatureCombination(fc.snapshotFeatures())
}

// ToJSON serializes the context to JSON
func (fc *FeatureContext) ToJSON() ([]byte, error) {
	return json.Marshal(fc)
}

// FromJSON deserializes the context from JSON
func (fc *FeatureContext) FromJSON(data []byte) error {
	return json.Unmarshal(data, fc)
}

// ToHeaders converts enabled features to HTTP headers
func (fc *FeatureContext) ToHeaders() map[string]string {
	headers := make(map[string]string)
	registry := GetRegistry()

	fc.Features.Range(func(feature Feature, enabled bool) bool {
		if info, ok := registry.GetFeature(feature); ok {
			if enabled {
				headers[info.HeaderName] = "true"
			}
		}
		return true
	})

	return headers
}

// GetStreamingMethod returns the preferred streaming method
func (fc *FeatureContext) GetStreamingMethod() string {
	if fc.IsEnabled(FeatureWebSocket) {
		return "websocket"
	}
	if fc.IsEnabled(FeatureSSE) {
		return "sse"
	}
	if fc.IsEnabled(FeatureJSONL) {
		return "jsonl"
	}
	return "sse"
}

// GetCompressionMethod returns the preferred compression method
func (fc *FeatureContext) GetCompressionMethod() string {
	if fc.IsEnabled(FeatureBrotli) {
		return "br"
	}
	if fc.IsEnabled(FeatureZstd) {
		return "zstd"
	}
	if fc.IsEnabled(FeatureGzip) {
		return "gzip"
	}
	return ""
}

// GetTransportProtocol returns the preferred transport protocol
func (fc *FeatureContext) GetTransportProtocol() string {
	if fc.IsEnabled(FeatureHTTP3) {
		return "h3"
	}
	if fc.IsEnabled(FeatureHTTP2) {
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

// FeatureUsageTracker tracks feature usage across requests.
//
// Concurrent-safe by construction (CONST-029): stats is a safe.Store.
// Field mutations on *FeatureStats route through Update; reads copy
// the value out under Update to avoid Pattern Beta races on the
// counter fields.
type FeatureUsageTracker struct {
	stats *safe.Store[Feature, *FeatureStats]
}

// globalTracker is the singleton usage tracker
var globalTracker *FeatureUsageTracker
var trackerOnce sync.Once

// GetUsageTracker returns the global feature usage tracker
func GetUsageTracker() *FeatureUsageTracker {
	trackerOnce.Do(func() {
		globalTracker = &FeatureUsageTracker{
			stats: safe.NewStore[Feature, *FeatureStats](),
		}
		// Initialize stats for all features
		for _, f := range GetRegistry().GetAllFeatures() {
			globalTracker.stats.Put(f.Name, &FeatureStats{
				Feature: f.Name,
			})
		}
	})
	return globalTracker
}

// RecordUsage records feature usage for a request
func (t *FeatureUsageTracker) RecordUsage(fc *FeatureContext) {
	for feature, enabled := range fc.snapshotFeatures() {
		t.stats.Update(feature, func(stats *FeatureStats, ok bool) (*FeatureStats, bool) {
			if !ok {
				return nil, false
			}
			stats.TotalRequests++
			if enabled {
				stats.EnabledCount++
			} else {
				stats.DisabledCount++
			}
			return stats, true
		})
	}
}

// GetStats returns usage statistics for all features
func (t *FeatureUsageTracker) GetStats() []*FeatureStats {
	keys := t.stats.Keys()
	stats := make([]*FeatureStats, 0, len(keys))
	for _, k := range keys {
		if s := t.GetFeatureStats(k); s != nil {
			stats = append(stats, s)
		}
	}
	return stats
}

// GetFeatureStats returns statistics for a specific feature
func (t *FeatureUsageTracker) GetFeatureStats(feature Feature) *FeatureStats {
	var snapshot *FeatureStats
	t.stats.Update(feature, func(stats *FeatureStats, ok bool) (*FeatureStats, bool) {
		if !ok {
			return nil, false
		}
		copy := *stats
		snapshot = &copy
		return stats, true
	})
	return snapshot
}

// ResetStats resets all usage statistics
func (t *FeatureUsageTracker) ResetStats() {
	for _, k := range t.stats.Keys() {
		t.stats.Update(k, func(s *FeatureStats, ok bool) (*FeatureStats, bool) {
			if !ok {
				return nil, false
			}
			s.EnabledCount = 0
			s.DisabledCount = 0
			s.TotalRequests = 0
			return s, true
		})
	}
}
