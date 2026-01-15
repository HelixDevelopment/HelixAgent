// Package features provides agent capability mapping for HelixAgent.
// This file maps CLI agents to their supported features, allowing
// automatic feature detection and configuration.
package features

import (
	"strings"
	"sync"
)

// AgentCapability defines the features supported by a CLI agent
type AgentCapability struct {
	// AgentName is the name of the CLI agent
	AgentName string `json:"agent_name"`
	// DisplayName is the human-readable name
	DisplayName string `json:"display_name"`
	// SupportedFeatures lists features the agent supports
	SupportedFeatures []Feature `json:"supported_features"`
	// PreferredFeatures lists features to enable by default for this agent
	PreferredFeatures []Feature `json:"preferred_features"`
	// UnsupportedFeatures lists features explicitly not supported
	UnsupportedFeatures []Feature `json:"unsupported_features"`
	// TransportProtocol indicates the primary transport (http1.1, http2, http3)
	TransportProtocol string `json:"transport_protocol"`
	// CompressionSupport lists supported compression algorithms
	CompressionSupport []string `json:"compression_support"`
	// StreamingSupport lists supported streaming methods
	StreamingSupport []string `json:"streaming_support"`
	// MaxConcurrentRequests is the recommended max concurrent requests
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
	// Notes provides additional information about the agent
	Notes string `json:"notes,omitempty"`
}

// agentCapabilities maps agent names to their capabilities
var agentCapabilities = map[string]*AgentCapability{
	// OpenCode - Go-based, supports MCP
	"opencode": {
		AgentName:   "OpenCode",
		DisplayName: "OpenCode AI",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureWebSocket, FeatureSSE, FeatureJSONL,
			FeatureGzip, FeatureMCP, FeatureEmbeddings, FeatureVision,
			FeatureDebate, FeatureBatchRequests, FeatureToolCalling,
			FeatureCaching, FeatureRateLimiting, FeatureMetrics,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip, FeatureMCP, FeatureToolCalling,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureGRPC,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 10,
	},

	// Crush - TypeScript terminal assistant
	"crush": {
		AgentName:   "Crush",
		DisplayName: "Crush Terminal AI",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureEmbeddings, FeatureCaching, FeatureToolCalling,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 5,
		Notes:                 "Lightweight terminal tool with basic streaming",
	},

	// HelixCode - Go-based distributed AI platform (ADVANCED)
	"helixcode": {
		AgentName:   "HelixCode",
		DisplayName: "HelixCode Distributed AI",
		SupportedFeatures: []Feature{
			FeatureGraphQL, FeatureTOON, FeatureHTTP2, FeatureHTTP3,
			FeatureWebSocket, FeatureSSE, FeatureJSONL,
			FeatureBrotli, FeatureGzip, FeatureZstd,
			FeatureMCP, FeatureACP, FeatureLSP, FeatureGRPC,
			FeatureEmbeddings, FeatureVision, FeatureCognee, FeatureDebate,
			FeatureBatchRequests, FeatureToolCalling, FeatureMultiPass,
			FeatureCaching, FeatureRateLimiting, FeatureMetrics, FeatureTracing,
		},
		PreferredFeatures: []Feature{
			FeatureGraphQL, FeatureTOON, FeatureHTTP3, FeatureBrotli,
			FeatureWebSocket, FeatureMCP, FeatureACP, FeatureDebate,
			FeatureMultiPass, FeatureTracing,
		},
		UnsupportedFeatures: []Feature{},
		TransportProtocol:   "http3",
		CompressionSupport:  []string{"brotli", "gzip", "zstd"},
		StreamingSupport:    []string{"websocket", "sse", "jsonl"},
		MaxConcurrentRequests: 50,
		Notes: "Full-featured HelixCode supports all advanced features including HTTP/3 QUIC",
	},

	// Kiro - Python AI agent with steering files
	"kiro": {
		AgentName:   "Kiro",
		DisplayName: "Kiro AI Agent",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureMCP, FeatureEmbeddings, FeatureVision,
			FeatureBatchRequests, FeatureToolCalling, FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip, FeatureMCP, FeatureToolCalling,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 10,
	},

	// Aider - Python pair programming
	"aider": {
		AgentName:   "Aider",
		DisplayName: "Aider Pair Programming",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureEmbeddings, FeatureToolCalling, FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 5,
	},

	// ClaudeCode - TypeScript Anthropic CLI
	"claudecode": {
		AgentName:   "ClaudeCode",
		DisplayName: "Claude Code CLI",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureWebSocket, FeatureSSE, FeatureJSONL,
			FeatureGzip, FeatureMCP, FeatureEmbeddings, FeatureVision,
			FeatureBatchRequests, FeatureToolCalling, FeatureCaching,
			FeatureMetrics,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip, FeatureMCP, FeatureToolCalling,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"websocket", "sse", "jsonl"},
		MaxConcurrentRequests: 10,
	},

	// Cline - VS Code extension with gRPC
	"cline": {
		AgentName:   "Cline",
		DisplayName: "Cline VS Code Agent",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureWebSocket, FeatureSSE, FeatureJSONL,
			FeatureGzip, FeatureMCP, FeatureGRPC, FeatureEmbeddings,
			FeatureVision, FeatureBatchRequests, FeatureToolCalling,
			FeatureCaching, FeatureMetrics,
		},
		PreferredFeatures: []Feature{
			FeatureWebSocket, FeatureGzip, FeatureMCP, FeatureGRPC,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"websocket", "sse", "jsonl"},
		MaxConcurrentRequests: 15,
	},

	// CodenameGoose - Rust profile-based assistant
	"codenamegoose": {
		AgentName:   "CodenameGoose",
		DisplayName: "Goose AI Assistant",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureEmbeddings, FeatureToolCalling, FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 5,
	},

	// DeepSeekCLI - TypeScript with Ollama support
	"deepseekcli": {
		AgentName:   "DeepSeekCLI",
		DisplayName: "DeepSeek CLI",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureEmbeddings, FeatureToolCalling, FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 5,
	},

	// Forge - Rust workflow orchestrator
	"forge": {
		AgentName:   "Forge",
		DisplayName: "Forge Agent Orchestrator",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureWebSocket, FeatureSSE, FeatureJSONL,
			FeatureGzip, FeatureBrotli, FeatureEmbeddings, FeatureVision,
			FeatureBatchRequests, FeatureToolCalling, FeatureCaching,
			FeatureMetrics,
		},
		PreferredFeatures: []Feature{
			FeatureWebSocket, FeatureGzip, FeatureBatchRequests,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureZstd,
			FeatureMCP, FeatureACP, FeatureLSP, FeatureGRPC,
			FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip", "brotli"},
		StreamingSupport:      []string{"websocket", "sse", "jsonl"},
		MaxConcurrentRequests: 20,
		Notes:                 "Workflow orchestrator with batch support",
	},

	// GeminiCLI - TypeScript Google Gemini
	"geminicli": {
		AgentName:   "GeminiCLI",
		DisplayName: "Gemini CLI",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureEmbeddings, FeatureVision, FeatureToolCalling,
			FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip, FeatureVision,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 10,
	},

	// GPTEngineer - Python project scaffolding
	"gptengineer": {
		AgentName:   "GPTEngineer",
		DisplayName: "GPT Engineer",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureToolCalling, FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureEmbeddings, FeatureVision,
			FeatureDebate, FeatureMultiPass, FeatureBatchRequests,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 3,
	},

	// KiloCode - TypeScript 50+ provider support
	"kilocode": {
		AgentName:   "KiloCode",
		DisplayName: "Kilo Code Multi-Provider",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureWebSocket, FeatureSSE, FeatureJSONL,
			FeatureGzip, FeatureBrotli, FeatureMCP, FeatureEmbeddings,
			FeatureVision, FeatureBatchRequests, FeatureToolCalling,
			FeatureCaching, FeatureMetrics,
		},
		PreferredFeatures: []Feature{
			FeatureWebSocket, FeatureGzip, FeatureMCP, FeatureToolCalling,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureZstd,
			FeatureACP, FeatureLSP, FeatureGRPC, FeatureDebate,
			FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip", "brotli"},
		StreamingSupport:      []string{"websocket", "sse", "jsonl"},
		MaxConcurrentRequests: 20,
	},

	// MistralCode - TypeScript Mistral CLI
	"mistralcode": {
		AgentName:   "MistralCode",
		DisplayName: "Mistral Code CLI",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureEmbeddings, FeatureToolCalling, FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 5,
	},

	// OllamaCode - TypeScript local models
	"ollamacode": {
		AgentName:   "OllamaCode",
		DisplayName: "Ollama Code Local",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureEmbeddings, FeatureToolCalling, FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 5,
		Notes:                 "Local-only, privacy-focused",
	},

	// Plandex - Go plan-based development
	"plandex": {
		AgentName:   "Plandex",
		DisplayName: "Plandex Plan-Based",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureToolCalling, FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureEmbeddings, FeatureVision,
			FeatureDebate, FeatureMultiPass, FeatureBatchRequests,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 3,
	},

	// QwenCode - TypeScript Alibaba Qwen
	"qwencode": {
		AgentName:   "QwenCode",
		DisplayName: "Qwen Code CLI",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureSSE, FeatureJSONL, FeatureGzip,
			FeatureEmbeddings, FeatureVision, FeatureToolCalling,
			FeatureCaching,
		},
		PreferredFeatures: []Feature{
			FeatureSSE, FeatureGzip,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureWebSocket, FeatureMCP, FeatureACP,
			FeatureLSP, FeatureGRPC, FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"sse", "jsonl"},
		MaxConcurrentRequests: 10,
	},

	// AmazonQ - Rust AWS integration
	"amazonq": {
		AgentName:   "AmazonQ",
		DisplayName: "Amazon Q Developer",
		SupportedFeatures: []Feature{
			FeatureHTTP2, FeatureWebSocket, FeatureSSE, FeatureJSONL,
			FeatureGzip, FeatureMCP, FeatureEmbeddings, FeatureVision,
			FeatureBatchRequests, FeatureToolCalling, FeatureCaching,
			FeatureMetrics,
		},
		PreferredFeatures: []Feature{
			FeatureWebSocket, FeatureGzip, FeatureMCP, FeatureToolCalling,
		},
		UnsupportedFeatures: []Feature{
			FeatureHTTP3, FeatureGraphQL, FeatureTOON, FeatureBrotli,
			FeatureZstd, FeatureACP, FeatureLSP, FeatureGRPC,
			FeatureDebate, FeatureMultiPass,
		},
		TransportProtocol:     "http2",
		CompressionSupport:    []string{"gzip"},
		StreamingSupport:      []string{"websocket", "sse", "jsonl"},
		MaxConcurrentRequests: 20,
	},
}

// CapabilityRegistry provides thread-safe access to agent capabilities
type CapabilityRegistry struct {
	mu           sync.RWMutex
	capabilities map[string]*AgentCapability
}

// globalCapabilityRegistry is the singleton capability registry
var globalCapabilityRegistry *CapabilityRegistry
var capabilityRegistryOnce sync.Once

// GetCapabilityRegistry returns the global capability registry
func GetCapabilityRegistry() *CapabilityRegistry {
	capabilityRegistryOnce.Do(func() {
		globalCapabilityRegistry = &CapabilityRegistry{
			capabilities: make(map[string]*AgentCapability),
		}
		// Copy all capabilities
		for k, v := range agentCapabilities {
			cap := *v
			globalCapabilityRegistry.capabilities[k] = &cap
		}
	})
	return globalCapabilityRegistry
}

// GetCapability returns capability info for an agent
func (r *CapabilityRegistry) GetCapability(agentName string) (*AgentCapability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cap, ok := r.capabilities[strings.ToLower(agentName)]
	return cap, ok
}

// GetAllCapabilities returns all registered agent capabilities
func (r *CapabilityRegistry) GetAllCapabilities() []*AgentCapability {
	r.mu.RLock()
	defer r.mu.RUnlock()
	caps := make([]*AgentCapability, 0, len(r.capabilities))
	for _, c := range r.capabilities {
		caps = append(caps, c)
	}
	return caps
}

// IsFeatureSupported checks if an agent supports a specific feature
func (r *CapabilityRegistry) IsFeatureSupported(agentName string, feature Feature) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, ok := r.capabilities[strings.ToLower(agentName)]
	if !ok {
		// Unknown agent - assume basic HTTP/2 support
		return isBasicFeature(feature)
	}

	// Check if explicitly unsupported
	for _, f := range cap.UnsupportedFeatures {
		if f == feature {
			return false
		}
	}

	// Check if explicitly supported
	for _, f := range cap.SupportedFeatures {
		if f == feature {
			return true
		}
	}

	return false
}

// IsFeaturePreferred checks if an agent prefers a specific feature
func (r *CapabilityRegistry) IsFeaturePreferred(agentName string, feature Feature) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, ok := r.capabilities[strings.ToLower(agentName)]
	if !ok {
		return false
	}

	for _, f := range cap.PreferredFeatures {
		if f == feature {
			return true
		}
	}

	return false
}

// GetAgentFeatureDefaults returns the default feature settings for an agent
func (r *CapabilityRegistry) GetAgentFeatureDefaults(agentName string) map[Feature]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defaults := make(map[Feature]bool)
	registry := GetRegistry()

	cap, ok := r.capabilities[strings.ToLower(agentName)]
	if !ok {
		// Unknown agent - return global defaults for basic features only
		for _, f := range registry.GetAllFeatures() {
			if isBasicFeature(f.Name) {
				defaults[f.Name] = f.DefaultValue
			} else {
				defaults[f.Name] = false
			}
		}
		return defaults
	}

	// Start with all features off
	for _, f := range registry.GetAllFeatures() {
		defaults[f.Name] = false
	}

	// Enable supported features based on global defaults
	for _, f := range cap.SupportedFeatures {
		if info, ok := registry.GetFeature(f); ok {
			defaults[f] = info.DefaultValue
		}
	}

	// Override with preferred features (always on)
	for _, f := range cap.PreferredFeatures {
		defaults[f] = true
	}

	return defaults
}

// GetSupportedStreamingMethods returns streaming methods supported by an agent
func (r *CapabilityRegistry) GetSupportedStreamingMethods(agentName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, ok := r.capabilities[strings.ToLower(agentName)]
	if !ok {
		return []string{"sse", "jsonl"} // Default streaming methods
	}

	return cap.StreamingSupport
}

// GetSupportedCompression returns compression methods supported by an agent
func (r *CapabilityRegistry) GetSupportedCompression(agentName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, ok := r.capabilities[strings.ToLower(agentName)]
	if !ok {
		return []string{"gzip"} // Default compression
	}

	return cap.CompressionSupport
}

// GetTransportProtocol returns the transport protocol for an agent
func (r *CapabilityRegistry) GetTransportProtocol(agentName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, ok := r.capabilities[strings.ToLower(agentName)]
	if !ok {
		return "http2" // Default transport
	}

	return cap.TransportProtocol
}

// GetAgentsByFeature returns agents that support a specific feature
func (r *CapabilityRegistry) GetAgentsByFeature(feature Feature) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agents []string
	for name, cap := range r.capabilities {
		for _, f := range cap.SupportedFeatures {
			if f == feature {
				agents = append(agents, name)
				break
			}
		}
	}
	return agents
}

// RegisterCapability adds or updates an agent capability
func (r *CapabilityRegistry) RegisterCapability(cap *AgentCapability) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.capabilities[strings.ToLower(cap.AgentName)] = cap
}

// isBasicFeature returns true for features that are universally supported
func isBasicFeature(feature Feature) bool {
	basicFeatures := map[Feature]bool{
		FeatureHTTP2:      true,
		FeatureSSE:        true,
		FeatureJSONL:      true,
		FeatureGzip:       true,
		FeatureEmbeddings: true,
		FeatureCaching:    true,
	}
	return basicFeatures[feature]
}

// FullFeatureAgents returns agents that support all advanced features
func (r *CapabilityRegistry) FullFeatureAgents() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agents []string
	advancedFeatures := []Feature{
		FeatureGraphQL, FeatureTOON, FeatureHTTP3,
		FeatureBrotli, FeatureWebSocket, FeatureDebate,
	}

	for name, cap := range r.capabilities {
		supportsAll := true
		for _, required := range advancedFeatures {
			found := false
			for _, f := range cap.SupportedFeatures {
				if f == required {
					found = true
					break
				}
			}
			if !found {
				supportsAll = false
				break
			}
		}
		if supportsAll {
			agents = append(agents, name)
		}
	}
	return agents
}
