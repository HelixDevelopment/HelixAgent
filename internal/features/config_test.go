package features

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultFeatureConfig(t *testing.T) {
	config := DefaultFeatureConfig()
	require.NotNil(t, config)

	assert.NotNil(t, config.GlobalDefaults)
	assert.NotNil(t, config.EndpointDefaults)
	assert.NotNil(t, config.AgentOverrides)
	assert.True(t, config.AllowFeatureHeaders)
	assert.True(t, config.AllowFeatureQueryParams)
	assert.False(t, config.StrictValidation)
	assert.True(t, config.LogFeatureUsage)
	assert.False(t, config.OpenAIEndpointGraphQL)
}

func TestNewFeatureContext(t *testing.T) {
	fc := NewFeatureContext()
	require.NotNil(t, fc)

	// Should have all features with default values
	assert.NotNil(t, fc.Features)
	assert.Equal(t, SourceGlobalDefault, fc.Source)

	// Check some defaults
	assert.True(t, fc.IsEnabled(FeatureHTTP2))
	assert.True(t, fc.IsEnabled(FeatureSSE))
	assert.True(t, fc.IsEnabled(FeatureGzip))
	assert.False(t, fc.IsEnabled(FeatureGraphQL))
	assert.False(t, fc.IsEnabled(FeatureTOON))
}

func TestNewFeatureContextFromConfig(t *testing.T) {
	config := DefaultFeatureConfig()
	config.OpenAIEndpointGraphQL = true

	// Non-OpenAI endpoint
	fc1 := NewFeatureContextFromConfig(config, "/v1/custom")
	assert.False(t, fc1.IsEnabled(FeatureGraphQL))

	// OpenAI endpoint should have GraphQL enabled
	fc2 := NewFeatureContextFromConfig(config, "/v1/chat/completions")
	assert.True(t, fc2.IsEnabled(FeatureGraphQL))
	assert.True(t, fc2.IsEnabled(FeatureTOON))
}

func TestFeatureContextIsEnabled(t *testing.T) {
	fc := NewFeatureContext()

	tests := []struct {
		feature  Feature
		expected bool
	}{
		{FeatureHTTP2, true},
		{FeatureSSE, true},
		{FeatureGraphQL, false},
		{FeatureTOON, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.feature), func(t *testing.T) {
			assert.Equal(t, tt.expected, fc.IsEnabled(tt.feature))
		})
	}
}

func TestFeatureContextSetEnabled(t *testing.T) {
	fc := NewFeatureContext()

	// Enable GraphQL
	fc.SetEnabled(FeatureGraphQL, true)
	assert.True(t, fc.IsEnabled(FeatureGraphQL))

	// Disable SSE
	fc.SetEnabled(FeatureSSE, false)
	assert.False(t, fc.IsEnabled(FeatureSSE))
}

func TestFeatureContextEnableDisable(t *testing.T) {
	fc := NewFeatureContext()

	fc.EnableFeature(FeatureGraphQL)
	assert.True(t, fc.IsEnabled(FeatureGraphQL))

	fc.DisableFeature(FeatureGraphQL)
	assert.False(t, fc.IsEnabled(FeatureGraphQL))
}

func TestFeatureContextGetEnabledFeatures(t *testing.T) {
	fc := NewFeatureContext()
	fc.SetEnabled(FeatureGraphQL, true)
	fc.SetEnabled(FeatureTOON, true)

	enabled := fc.GetEnabledFeatures()
	assert.NotEmpty(t, enabled)
	assert.Contains(t, enabled, FeatureGraphQL)
	assert.Contains(t, enabled, FeatureTOON)
}

func TestFeatureContextGetDisabledFeatures(t *testing.T) {
	fc := NewFeatureContext()

	disabled := fc.GetDisabledFeatures()
	assert.NotEmpty(t, disabled)
	assert.Contains(t, disabled, FeatureGraphQL)
	assert.Contains(t, disabled, FeatureTOON)
}

func TestFeatureContextApplyAgentCapabilities(t *testing.T) {
	fc := NewFeatureContext()

	// Apply HelixCode capabilities
	fc.ApplyAgentCapabilities("helixcode")
	assert.True(t, fc.IsEnabled(FeatureGraphQL))
	assert.True(t, fc.IsEnabled(FeatureTOON))
	assert.True(t, fc.IsEnabled(FeatureHTTP3))
	assert.Equal(t, "helixcode", fc.AgentName)
	assert.Equal(t, SourceAgentDetection, fc.Source)

	// Apply OpenCode capabilities
	fc2 := NewFeatureContext()
	fc2.ApplyAgentCapabilities("opencode")
	assert.False(t, fc2.IsEnabled(FeatureGraphQL))
	assert.False(t, fc2.IsEnabled(FeatureTOON))
}

func TestFeatureContextApplyOverrides(t *testing.T) {
	fc := NewFeatureContext()

	overrides := map[Feature]bool{
		FeatureGraphQL: true,
		FeatureSSE:     false,
	}

	fc.ApplyOverrides(overrides, SourceHeaderOverride)

	assert.True(t, fc.IsEnabled(FeatureGraphQL))
	assert.False(t, fc.IsEnabled(FeatureSSE))
	assert.Equal(t, SourceHeaderOverride, fc.Source)
}

func TestFeatureContextClone(t *testing.T) {
	fc := NewFeatureContext()
	fc.AgentName = "test"
	fc.Endpoint = "/test"
	fc.SetEnabled(FeatureGraphQL, true)

	cloned := fc.Clone()

	assert.Equal(t, fc.AgentName, cloned.AgentName)
	assert.Equal(t, fc.Endpoint, cloned.Endpoint)
	assert.True(t, cloned.IsEnabled(FeatureGraphQL))

	// Modifying clone shouldn't affect original
	cloned.SetEnabled(FeatureGraphQL, false)
	assert.True(t, fc.IsEnabled(FeatureGraphQL))
}

func TestFeatureContextValidate(t *testing.T) {
	// Valid combination
	fc := NewFeatureContext()
	err := fc.Validate()
	assert.NoError(t, err)

	// Invalid: MultiPass without Debate
	fc2 := NewFeatureContext()
	fc2.SetEnabled(FeatureMultiPass, true)
	fc2.SetEnabled(FeatureDebate, false)
	err = fc2.Validate()
	assert.Error(t, err)

	// Invalid: HTTP2 and HTTP3 together
	fc3 := NewFeatureContext()
	fc3.SetEnabled(FeatureHTTP2, true)
	fc3.SetEnabled(FeatureHTTP3, true)
	err = fc3.Validate()
	assert.Error(t, err)
}

func TestFeatureContextToJSON(t *testing.T) {
	fc := NewFeatureContext()
	fc.AgentName = "test"

	data, err := fc.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "features")
	assert.Contains(t, string(data), "agent_name")
}

func TestFeatureContextFromJSON(t *testing.T) {
	fc := NewFeatureContext()
	fc.AgentName = "test"
	fc.SetEnabled(FeatureGraphQL, true)

	data, _ := fc.ToJSON()

	fc2 := NewFeatureContext()
	err := fc2.FromJSON(data)
	require.NoError(t, err)

	assert.Equal(t, fc.AgentName, fc2.AgentName)
	assert.Equal(t, fc.IsEnabled(FeatureGraphQL), fc2.IsEnabled(FeatureGraphQL))
}

func TestFeatureContextToHeaders(t *testing.T) {
	fc := NewFeatureContext()
	fc.SetEnabled(FeatureGraphQL, true)
	fc.SetEnabled(FeatureTOON, true)

	headers := fc.ToHeaders()
	assert.NotEmpty(t, headers)
	assert.Equal(t, "true", headers["X-Feature-GraphQL"])
	assert.Equal(t, "true", headers["X-Feature-TOON"])
}

func TestFeatureContextGetStreamingMethod(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*FeatureContext)
		expected string
	}{
		{
			name: "websocket_preferred",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureWebSocket, true)
				fc.SetEnabled(FeatureSSE, true)
				fc.SetEnabled(FeatureJSONL, true)
			},
			expected: "websocket",
		},
		{
			name: "sse_when_no_websocket",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureWebSocket, false)
				fc.SetEnabled(FeatureSSE, true)
				fc.SetEnabled(FeatureJSONL, true)
			},
			expected: "sse",
		},
		{
			name: "jsonl_when_no_others",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureWebSocket, false)
				fc.SetEnabled(FeatureSSE, false)
				fc.SetEnabled(FeatureJSONL, true)
			},
			expected: "jsonl",
		},
		{
			name: "fallback_to_sse",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureWebSocket, false)
				fc.SetEnabled(FeatureSSE, false)
				fc.SetEnabled(FeatureJSONL, false)
			},
			expected: "sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := NewFeatureContext()
			tt.setup(fc)
			assert.Equal(t, tt.expected, fc.GetStreamingMethod())
		})
	}
}

func TestFeatureContextGetCompressionMethod(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*FeatureContext)
		expected string
	}{
		{
			name: "brotli_preferred",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureBrotli, true)
				fc.SetEnabled(FeatureZstd, true)
				fc.SetEnabled(FeatureGzip, true)
			},
			expected: "br",
		},
		{
			name: "zstd_when_no_brotli",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureBrotli, false)
				fc.SetEnabled(FeatureZstd, true)
				fc.SetEnabled(FeatureGzip, true)
			},
			expected: "zstd",
		},
		{
			name: "gzip_when_no_others",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureBrotli, false)
				fc.SetEnabled(FeatureZstd, false)
				fc.SetEnabled(FeatureGzip, true)
			},
			expected: "gzip",
		},
		{
			name: "none_when_disabled",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureBrotli, false)
				fc.SetEnabled(FeatureZstd, false)
				fc.SetEnabled(FeatureGzip, false)
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := NewFeatureContext()
			tt.setup(fc)
			assert.Equal(t, tt.expected, fc.GetCompressionMethod())
		})
	}
}

func TestFeatureContextGetTransportProtocol(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*FeatureContext)
		expected string
	}{
		{
			name: "http3_preferred",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureHTTP3, true)
				fc.SetEnabled(FeatureHTTP2, true)
			},
			expected: "h3",
		},
		{
			name: "http2_when_no_http3",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureHTTP3, false)
				fc.SetEnabled(FeatureHTTP2, true)
			},
			expected: "h2",
		},
		{
			name: "http1_fallback",
			setup: func(fc *FeatureContext) {
				fc.SetEnabled(FeatureHTTP3, false)
				fc.SetEnabled(FeatureHTTP2, false)
			},
			expected: "http/1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := NewFeatureContext()
			tt.setup(fc)
			assert.Equal(t, tt.expected, fc.GetTransportProtocol())
		})
	}
}

func TestContextIntegration(t *testing.T) {
	fc := NewFeatureContext()
	fc.AgentName = "test"

	ctx := WithFeatureContext(context.Background(), fc)
	retrieved := GetFeatureContext(ctx)

	assert.Equal(t, fc.AgentName, retrieved.AgentName)
}

func TestGetFeatureContextFromEmptyContext(t *testing.T) {
	ctx := context.Background()
	fc := GetFeatureContext(ctx)

	// Should return default context
	assert.NotNil(t, fc)
	assert.NotNil(t, fc.Features)
}

func TestFeatureUsageTracker(t *testing.T) {
	tracker := GetUsageTracker()
	require.NotNil(t, tracker)

	// Reset for clean test
	tracker.ResetStats()

	// Record some usage
	fc := NewFeatureContext()
	fc.SetEnabled(FeatureGraphQL, true)
	tracker.RecordUsage(fc)
	tracker.RecordUsage(fc)

	// Check stats
	stats := tracker.GetStats()
	assert.NotEmpty(t, stats)

	graphqlStats := tracker.GetFeatureStats(FeatureGraphQL)
	require.NotNil(t, graphqlStats)
	assert.Equal(t, int64(2), graphqlStats.EnabledCount)
	assert.Equal(t, int64(2), graphqlStats.TotalRequests)
}

func TestFeatureUsageTrackerSingleton(t *testing.T) {
	tracker1 := GetUsageTracker()
	tracker2 := GetUsageTracker()
	assert.Same(t, tracker1, tracker2)
}

func TestIsOpenAIEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		expected bool
	}{
		{"/v1/chat/completions", true},
		{"/v1/completions", true},
		{"/v1/embeddings", true},
		{"/v1/models", true},
		{"/v1/graphql", false},
		{"/v1/mcp", false},
		{"/v1/custom", false},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			assert.Equal(t, tt.expected, isOpenAIEndpoint(tt.endpoint))
		})
	}
}
