package features

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCapabilityRegistry(t *testing.T) {
	registry := GetCapabilityRegistry()
	require.NotNil(t, registry)

	// Should be singleton
	registry2 := GetCapabilityRegistry()
	assert.Same(t, registry, registry2)
}

func TestCapabilityRegistryGetCapability(t *testing.T) {
	registry := GetCapabilityRegistry()

	tests := []struct {
		agentName string
		exists    bool
	}{
		{"helixcode", true},
		{"opencode", true},
		{"claudecode", true},
		{"unknown_agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			cap, ok := registry.GetCapability(tt.agentName)
			assert.Equal(t, tt.exists, ok)
			if tt.exists {
				assert.NotNil(t, cap)
			}
		})
	}
}

func TestCapabilityRegistryCaseInsensitive(t *testing.T) {
	registry := GetCapabilityRegistry()

	// Test case insensitivity
	cap1, ok1 := registry.GetCapability("HelixCode")
	cap2, ok2 := registry.GetCapability("helixcode")
	cap3, ok3 := registry.GetCapability("HELIXCODE")

	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.True(t, ok3)
	assert.Equal(t, cap1.AgentName, cap2.AgentName)
	assert.Equal(t, cap2.AgentName, cap3.AgentName)
}

func TestCapabilityRegistryGetAllCapabilities(t *testing.T) {
	registry := GetCapabilityRegistry()
	caps := registry.GetAllCapabilities()

	assert.NotEmpty(t, caps)
	assert.GreaterOrEqual(t, len(caps), 18) // Should have at least 18 agents
}

func TestCapabilityRegistryIsFeatureSupported(t *testing.T) {
	registry := GetCapabilityRegistry()

	tests := []struct {
		agentName string
		feature   Feature
		supported bool
	}{
		// HelixCode supports all features
		{"helixcode", FeatureGraphQL, true},
		{"helixcode", FeatureTOON, true},
		{"helixcode", FeatureHTTP3, true},
		{"helixcode", FeatureBrotli, true},
		// OpenCode has limited support
		{"opencode", FeatureGraphQL, false},
		{"opencode", FeatureTOON, false},
		{"opencode", FeatureHTTP3, false},
		{"opencode", FeatureHTTP2, true},
		{"opencode", FeatureSSE, true},
		// ClaudeCode
		{"claudecode", FeatureMCP, true},
		{"claudecode", FeatureGraphQL, false},
		// Unknown agent should support basic features only
		{"unknown", FeatureHTTP2, true},
		{"unknown", FeatureSSE, true},
		{"unknown", FeatureGraphQL, false},
	}

	for _, tt := range tests {
		t.Run(tt.agentName+"_"+string(tt.feature), func(t *testing.T) {
			supported := registry.IsFeatureSupported(tt.agentName, tt.feature)
			assert.Equal(t, tt.supported, supported)
		})
	}
}

func TestCapabilityRegistryIsFeaturePreferred(t *testing.T) {
	registry := GetCapabilityRegistry()

	tests := []struct {
		agentName string
		feature   Feature
		preferred bool
	}{
		{"helixcode", FeatureGraphQL, true},
		{"helixcode", FeatureTOON, true},
		{"helixcode", FeatureHTTP3, true},
		{"opencode", FeatureSSE, true},
		{"opencode", FeatureGraphQL, false},
		{"claudecode", FeatureMCP, true},
	}

	for _, tt := range tests {
		t.Run(tt.agentName+"_"+string(tt.feature), func(t *testing.T) {
			preferred := registry.IsFeaturePreferred(tt.agentName, tt.feature)
			assert.Equal(t, tt.preferred, preferred)
		})
	}
}

func TestCapabilityRegistryGetAgentFeatureDefaults(t *testing.T) {
	registry := GetCapabilityRegistry()

	// HelixCode should have advanced features enabled
	helixDefaults := registry.GetAgentFeatureDefaults("helixcode")
	assert.True(t, helixDefaults[FeatureGraphQL])
	assert.True(t, helixDefaults[FeatureTOON])
	assert.True(t, helixDefaults[FeatureHTTP3])
	assert.True(t, helixDefaults[FeatureBrotli])

	// OpenCode should have basic features only
	openCodeDefaults := registry.GetAgentFeatureDefaults("opencode")
	assert.False(t, openCodeDefaults[FeatureGraphQL])
	assert.False(t, openCodeDefaults[FeatureTOON])
	assert.False(t, openCodeDefaults[FeatureHTTP3])
	assert.True(t, openCodeDefaults[FeatureSSE])
	assert.True(t, openCodeDefaults[FeatureGzip])

	// Unknown agent should have basic defaults
	unknownDefaults := registry.GetAgentFeatureDefaults("unknown")
	assert.False(t, unknownDefaults[FeatureGraphQL])
	assert.True(t, unknownDefaults[FeatureHTTP2])
	assert.True(t, unknownDefaults[FeatureSSE])
}

func TestCapabilityRegistryGetSupportedStreamingMethods(t *testing.T) {
	registry := GetCapabilityRegistry()

	// HelixCode should support all streaming methods
	helixStreaming := registry.GetSupportedStreamingMethods("helixcode")
	assert.Contains(t, helixStreaming, "websocket")
	assert.Contains(t, helixStreaming, "sse")
	assert.Contains(t, helixStreaming, "jsonl")

	// Crush has limited streaming
	crushStreaming := registry.GetSupportedStreamingMethods("crush")
	assert.Contains(t, crushStreaming, "sse")
	assert.Contains(t, crushStreaming, "jsonl")

	// Unknown agent gets default
	unknownStreaming := registry.GetSupportedStreamingMethods("unknown")
	assert.Contains(t, unknownStreaming, "sse")
}

func TestCapabilityRegistryGetSupportedCompression(t *testing.T) {
	registry := GetCapabilityRegistry()

	// HelixCode should support all compression
	helixCompression := registry.GetSupportedCompression("helixcode")
	assert.Contains(t, helixCompression, "brotli")
	assert.Contains(t, helixCompression, "gzip")
	assert.Contains(t, helixCompression, "zstd")

	// OpenCode has limited compression
	openCodeCompression := registry.GetSupportedCompression("opencode")
	assert.Contains(t, openCodeCompression, "gzip")
	assert.NotContains(t, openCodeCompression, "brotli")

	// Unknown agent gets default
	unknownCompression := registry.GetSupportedCompression("unknown")
	assert.Contains(t, unknownCompression, "gzip")
}

func TestCapabilityRegistryGetTransportProtocol(t *testing.T) {
	registry := GetCapabilityRegistry()

	tests := []struct {
		agentName string
		protocol  string
	}{
		{"helixcode", "http3"},
		{"opencode", "http2"},
		{"claudecode", "http2"},
		{"unknown", "http2"},
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			protocol := registry.GetTransportProtocol(tt.agentName)
			assert.Equal(t, tt.protocol, protocol)
		})
	}
}

func TestCapabilityRegistryGetAgentsByFeature(t *testing.T) {
	registry := GetCapabilityRegistry()

	// GraphQL is only supported by HelixCode
	graphqlAgents := registry.GetAgentsByFeature(FeatureGraphQL)
	assert.Contains(t, graphqlAgents, "helixcode")
	assert.Len(t, graphqlAgents, 1)

	// HTTP2 is supported by all
	http2Agents := registry.GetAgentsByFeature(FeatureHTTP2)
	assert.GreaterOrEqual(t, len(http2Agents), 18)

	// MCP is supported by several
	mcpAgents := registry.GetAgentsByFeature(FeatureMCP)
	assert.Contains(t, mcpAgents, "helixcode")
	assert.Contains(t, mcpAgents, "opencode")
	assert.Contains(t, mcpAgents, "claudecode")
}

func TestCapabilityRegistryRegisterCapability(t *testing.T) {
	registry := GetCapabilityRegistry()

	newCap := &AgentCapability{
		AgentName:   "TestAgent",
		DisplayName: "Test Agent",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureGzip,
		},
		TransportProtocol:  "http2",
		CompressionSupport: []string{"gzip"},
		StreamingSupport:   []string{"sse"},
	}

	registry.RegisterCapability(newCap)

	// Should be retrievable
	cap, ok := registry.GetCapability("testagent")
	assert.True(t, ok)
	assert.Equal(t, "Test Agent", cap.DisplayName)
}

func TestCapabilityRegistryFullFeatureAgents(t *testing.T) {
	registry := GetCapabilityRegistry()
	fullFeatureAgents := registry.FullFeatureAgents()

	// Only HelixCode should support all advanced features
	assert.Contains(t, fullFeatureAgents, "helixcode")
	assert.NotContains(t, fullFeatureAgents, "opencode")
	assert.NotContains(t, fullFeatureAgents, "claudecode")
}

func TestAgentCapabilityFields(t *testing.T) {
	registry := GetCapabilityRegistry()
	cap, ok := registry.GetCapability("helixcode")
	require.True(t, ok)

	assert.Equal(t, "HelixCode", cap.AgentName)
	assert.NotEmpty(t, cap.DisplayName)
	assert.NotEmpty(t, cap.SupportedFeatures)
	assert.NotEmpty(t, cap.PreferredFeatures)
	assert.NotEmpty(t, cap.TransportProtocol)
	assert.NotEmpty(t, cap.CompressionSupport)
	assert.NotEmpty(t, cap.StreamingSupport)
	assert.Greater(t, cap.MaxConcurrentRequests, 0)
}

func TestAllAgentsHaveRequiredFields(t *testing.T) {
	registry := GetCapabilityRegistry()
	caps := registry.GetAllCapabilities()

	for _, cap := range caps {
		t.Run(cap.AgentName, func(t *testing.T) {
			assert.NotEmpty(t, cap.AgentName)
			assert.NotEmpty(t, cap.DisplayName)
			assert.NotEmpty(t, cap.SupportedFeatures)
			assert.NotEmpty(t, cap.TransportProtocol)
			assert.NotEmpty(t, cap.CompressionSupport)
			assert.NotEmpty(t, cap.StreamingSupport)
		})
	}
}
