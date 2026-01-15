package features

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureConstants(t *testing.T) {
	// Test that all expected features are defined
	expectedFeatures := []Feature{
		FeatureGraphQL, FeatureTOON, FeatureHTTP2, FeatureHTTP3,
		FeatureWebSocket, FeatureSSE, FeatureJSONL,
		FeatureBrotli, FeatureGzip, FeatureZstd,
		FeatureMCP, FeatureACP, FeatureLSP, FeatureGRPC,
		FeatureEmbeddings, FeatureVision, FeatureCognee, FeatureDebate,
		FeatureBatchRequests, FeatureToolCalling, FeatureMultiPass,
		FeatureCaching, FeatureRateLimiting, FeatureMetrics, FeatureTracing,
	}

	for _, feature := range expectedFeatures {
		t.Run(string(feature), func(t *testing.T) {
			assert.NotEmpty(t, string(feature))
		})
	}
}

func TestFeatureCategories(t *testing.T) {
	categories := []FeatureCategory{
		CategoryTransport, CategoryCompression, CategoryProtocol,
		CategoryAPI, CategoryAdvanced,
	}

	for _, category := range categories {
		t.Run(string(category), func(t *testing.T) {
			assert.NotEmpty(t, string(category))
		})
	}
}

func TestGetRegistry(t *testing.T) {
	registry := GetRegistry()
	require.NotNil(t, registry)

	// Should be singleton
	registry2 := GetRegistry()
	assert.Same(t, registry, registry2)
}

func TestRegistryGetFeature(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		feature Feature
		exists  bool
	}{
		{FeatureGraphQL, true},
		{FeatureTOON, true},
		{FeatureHTTP3, true},
		{Feature("nonexistent"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.feature), func(t *testing.T) {
			info, ok := registry.GetFeature(tt.feature)
			assert.Equal(t, tt.exists, ok)
			if tt.exists {
				assert.NotNil(t, info)
				assert.Equal(t, tt.feature, info.Name)
			}
		})
	}
}

func TestRegistryGetAllFeatures(t *testing.T) {
	registry := GetRegistry()
	features := registry.GetAllFeatures()

	assert.NotEmpty(t, features)
	assert.GreaterOrEqual(t, len(features), 20) // Should have at least 20 features
}

func TestRegistryGetFeaturesByCategory(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		category    FeatureCategory
		minExpected int
	}{
		{CategoryTransport, 5},
		{CategoryCompression, 3},
		{CategoryProtocol, 4},
		{CategoryAPI, 5},
		{CategoryAdvanced, 4},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			features := registry.GetFeaturesByCategory(tt.category)
			assert.GreaterOrEqual(t, len(features), tt.minExpected)
		})
	}
}

func TestRegistryGetDefaultValue(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		feature  Feature
		expected bool
	}{
		{FeatureHTTP2, true},         // HTTP/2 enabled by default
		{FeatureSSE, true},           // SSE enabled by default
		{FeatureGzip, true},          // Gzip enabled by default
		{FeatureGraphQL, false},      // GraphQL disabled by default
		{FeatureTOON, false},         // TOON disabled by default
		{FeatureHTTP3, false},        // HTTP/3 disabled by default
		{FeatureBrotli, false},       // Brotli disabled by default
	}

	for _, tt := range tests {
		t.Run(string(tt.feature), func(t *testing.T) {
			assert.Equal(t, tt.expected, registry.GetDefaultValue(tt.feature))
		})
	}
}

func TestRegistryIsValidFeature(t *testing.T) {
	registry := GetRegistry()

	assert.True(t, registry.IsValidFeature(FeatureGraphQL))
	assert.True(t, registry.IsValidFeature(FeatureTOON))
	assert.False(t, registry.IsValidFeature(Feature("invalid")))
}

func TestRegistryGetFeatureByHeader(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		header  string
		feature Feature
		found   bool
	}{
		{"X-Feature-GraphQL", FeatureGraphQL, true},
		{"X-Feature-TOON", FeatureTOON, true},
		{"X-Feature-HTTP3", FeatureHTTP3, true},
		{"X-Invalid-Header", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			feature, found := registry.GetFeatureByHeader(tt.header)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.feature, feature)
			}
		})
	}
}

func TestRegistryGetFeatureByQueryParam(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		param   string
		feature Feature
		found   bool
	}{
		{"graphql", FeatureGraphQL, true},
		{"toon", FeatureTOON, true},
		{"http3", FeatureHTTP3, true},
		{"invalid", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.param, func(t *testing.T) {
			feature, found := registry.GetFeatureByQueryParam(tt.param)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.feature, feature)
			}
		})
	}
}

func TestRegistryValidateFeatureCombination(t *testing.T) {
	registry := GetRegistry()

	tests := []struct {
		name     string
		features map[Feature]bool
		valid    bool
	}{
		{
			name: "valid_combination",
			features: map[Feature]bool{
				FeatureHTTP2:     true,
				FeatureSSE:       true,
				FeatureGzip:      true,
			},
			valid: true,
		},
		{
			name: "multipass_without_debate",
			features: map[Feature]bool{
				FeatureMultiPass: true,
				FeatureDebate:    false,
			},
			valid: false,
		},
		{
			name: "multipass_with_debate",
			features: map[Feature]bool{
				FeatureMultiPass: true,
				FeatureDebate:    true,
			},
			valid: true,
		},
		{
			name: "http2_and_http3_conflict",
			features: map[Feature]bool{
				FeatureHTTP2: true,
				FeatureHTTP3: true,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.ValidateFeatureCombination(tt.features)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestRegistrySetEndpointDefaults(t *testing.T) {
	registry := GetRegistry()

	endpoint := "/v1/test/custom"
	defaults := map[Feature]bool{
		FeatureGraphQL: true,
		FeatureTOON:    true,
	}

	registry.SetEndpointDefaults(endpoint, defaults)

	assert.True(t, registry.GetEndpointDefault(endpoint, FeatureGraphQL))
	assert.True(t, registry.GetEndpointDefault(endpoint, FeatureTOON))
	// Non-custom endpoint should use global default
	assert.False(t, registry.GetEndpointDefault("/v1/other", FeatureGraphQL))
}

func TestFeatureString(t *testing.T) {
	assert.Equal(t, "graphql", FeatureGraphQL.String())
	assert.Equal(t, "toon", FeatureTOON.String())
	assert.Equal(t, "http3", FeatureHTTP3.String())
}

func TestParseFeature(t *testing.T) {
	tests := []struct {
		input    string
		expected Feature
	}{
		{"graphql", FeatureGraphQL},
		{"GRAPHQL", FeatureGraphQL},
		{"GraphQL", FeatureGraphQL},
		{"toon", FeatureTOON},
		{"http3", FeatureHTTP3},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseFeature(tt.input))
		})
	}
}

func TestFeatureInfoFields(t *testing.T) {
	registry := GetRegistry()
	info, ok := registry.GetFeature(FeatureGraphQL)
	require.True(t, ok)

	assert.Equal(t, FeatureGraphQL, info.Name)
	assert.NotEmpty(t, info.DisplayName)
	assert.NotEmpty(t, info.Description)
	assert.Equal(t, CategoryTransport, info.Category)
	assert.NotEmpty(t, info.HeaderName)
	assert.NotEmpty(t, info.QueryParam)
}

func TestFeatureValidationError(t *testing.T) {
	err := &FeatureValidationError{
		Feature: FeatureMultiPass,
		Message: "requires feature: debate",
	}

	assert.Contains(t, err.Error(), "multipass")
	assert.Contains(t, err.Error(), "requires feature: debate")
}
