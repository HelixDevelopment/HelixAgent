// Package features provides a comprehensive feature flags system for HelixAgent.
// It allows toggling features like GraphQL, TOON encoding, streaming types,
// compression algorithms, and transport protocols based on CLI agent capabilities
// and user preferences.
package features

import (
	"strings"
	"sync"
)

// Feature represents a toggleable feature in HelixAgent
type Feature string

// Feature constants define all available features
const (
	// Transport Features
	FeatureGraphQL       Feature = "graphql"       // GraphQL API endpoint
	FeatureTOON          Feature = "toon"          // Token-Optimized Object Notation
	FeatureHTTP2         Feature = "http2"         // HTTP/2 support
	FeatureHTTP3         Feature = "http3"         // HTTP/3 QUIC support
	FeatureWebSocket     Feature = "websocket"     // WebSocket streaming
	FeatureSSE           Feature = "sse"           // Server-Sent Events streaming
	FeatureJSONL         Feature = "jsonl"         // JSON Lines streaming

	// Compression Features
	FeatureBrotli        Feature = "brotli"        // Brotli compression
	FeatureGzip          Feature = "gzip"          // Gzip compression
	FeatureZstd          Feature = "zstd"          // Zstandard compression

	// Protocol Features
	FeatureMCP           Feature = "mcp"           // Model Context Protocol
	FeatureACP           Feature = "acp"           // Agent Communication Protocol
	FeatureLSP           Feature = "lsp"           // Language Server Protocol
	FeatureGRPC          Feature = "grpc"          // gRPC protocol

	// API Features
	FeatureEmbeddings    Feature = "embeddings"    // Vector embeddings API
	FeatureVision        Feature = "vision"        // Vision/image analysis API
	FeatureCognee        Feature = "cognee"        // Cognee knowledge graph
	FeatureDebate        Feature = "debate"        // AI Debate system
	FeatureBatchRequests Feature = "batch"         // Batch request support
	FeatureToolCalling   Feature = "tool_calling"  // Tool/function calling

	// Advanced Features
	FeatureMultiPass     Feature = "multipass"     // Multi-pass validation
	FeatureCaching       Feature = "caching"       // Response caching
	FeatureRateLimiting  Feature = "rate_limiting" // Rate limiting
	FeatureMetrics       Feature = "metrics"       // Prometheus metrics
	FeatureTracing       Feature = "tracing"       // Distributed tracing
)

// FeatureCategory groups related features
type FeatureCategory string

const (
	CategoryTransport   FeatureCategory = "transport"
	CategoryCompression FeatureCategory = "compression"
	CategoryProtocol    FeatureCategory = "protocol"
	CategoryAPI         FeatureCategory = "api"
	CategoryAdvanced    FeatureCategory = "advanced"
)

// FeatureInfo contains metadata about a feature
type FeatureInfo struct {
	Name         Feature         `json:"name"`
	DisplayName  string          `json:"display_name"`
	Description  string          `json:"description"`
	Category     FeatureCategory `json:"category"`
	DefaultValue bool            `json:"default_value"`
	// RequiresFeatures lists features that must be enabled for this feature to work
	RequiresFeatures []Feature `json:"requires_features,omitempty"`
	// ConflictsWith lists features that cannot be enabled together
	ConflictsWith []Feature `json:"conflicts_with,omitempty"`
	// HeaderName is the HTTP header used to toggle this feature
	HeaderName string `json:"header_name"`
	// QueryParam is the URL query parameter used to toggle this feature
	QueryParam string `json:"query_param"`
}

// featureRegistry contains all feature definitions
var featureRegistry = map[Feature]*FeatureInfo{
	// Transport Features
	FeatureGraphQL: {
		Name:         FeatureGraphQL,
		DisplayName:  "GraphQL API",
		Description:  "Enable GraphQL query endpoint for flexible data fetching",
		Category:     CategoryTransport,
		DefaultValue: false, // Default off for backward compatibility
		HeaderName:   "X-Feature-GraphQL",
		QueryParam:   "graphql",
	},
	FeatureTOON: {
		Name:         FeatureTOON,
		DisplayName:  "TOON Encoding",
		Description:  "Token-Optimized Object Notation for efficient AI consumption",
		Category:     CategoryTransport,
		DefaultValue: false, // Default off for backward compatibility
		HeaderName:   "X-Feature-TOON",
		QueryParam:   "toon",
	},
	FeatureHTTP2: {
		Name:         FeatureHTTP2,
		DisplayName:  "HTTP/2",
		Description:  "HTTP/2 multiplexing and server push support",
		Category:     CategoryTransport,
		DefaultValue: true, // Default on - widely supported
		HeaderName:   "X-Feature-HTTP2",
		QueryParam:   "http2",
	},
	FeatureHTTP3: {
		Name:           FeatureHTTP3,
		DisplayName:    "HTTP/3 QUIC",
		Description:    "HTTP/3 with QUIC transport for improved latency",
		Category:       CategoryTransport,
		DefaultValue:   false, // Default off - limited client support
		ConflictsWith:  []Feature{FeatureHTTP2},
		HeaderName:     "X-Feature-HTTP3",
		QueryParam:     "http3",
	},
	FeatureWebSocket: {
		Name:         FeatureWebSocket,
		DisplayName:  "WebSocket Streaming",
		Description:  "WebSocket-based bidirectional streaming",
		Category:     CategoryTransport,
		DefaultValue: true, // Default on - widely supported
		HeaderName:   "X-Feature-WebSocket",
		QueryParam:   "websocket",
	},
	FeatureSSE: {
		Name:         FeatureSSE,
		DisplayName:  "Server-Sent Events",
		Description:  "Server-Sent Events for real-time updates",
		Category:     CategoryTransport,
		DefaultValue: true, // Default on - widely supported
		HeaderName:   "X-Feature-SSE",
		QueryParam:   "sse",
	},
	FeatureJSONL: {
		Name:         FeatureJSONL,
		DisplayName:  "JSON Lines Streaming",
		Description:  "JSON Lines format for streaming responses",
		Category:     CategoryTransport,
		DefaultValue: true, // Default on - widely supported
		HeaderName:   "X-Feature-JSONL",
		QueryParam:   "jsonl",
	},

	// Compression Features
	FeatureBrotli: {
		Name:         FeatureBrotli,
		DisplayName:  "Brotli Compression",
		Description:  "Brotli compression for efficient data transfer",
		Category:     CategoryCompression,
		DefaultValue: false, // Default off - not all clients support
		HeaderName:   "X-Feature-Brotli",
		QueryParam:   "brotli",
	},
	FeatureGzip: {
		Name:         FeatureGzip,
		DisplayName:  "Gzip Compression",
		Description:  "Gzip compression for data transfer",
		Category:     CategoryCompression,
		DefaultValue: true, // Default on - universally supported
		HeaderName:   "X-Feature-Gzip",
		QueryParam:   "gzip",
	},
	FeatureZstd: {
		Name:         FeatureZstd,
		DisplayName:  "Zstandard Compression",
		Description:  "Zstandard compression for high-ratio compression",
		Category:     CategoryCompression,
		DefaultValue: false, // Default off - limited client support
		HeaderName:   "X-Feature-Zstd",
		QueryParam:   "zstd",
	},

	// Protocol Features
	FeatureMCP: {
		Name:         FeatureMCP,
		DisplayName:  "Model Context Protocol",
		Description:  "MCP for tool and context sharing with AI models",
		Category:     CategoryProtocol,
		DefaultValue: true, // Default on - core feature
		HeaderName:   "X-Feature-MCP",
		QueryParam:   "mcp",
	},
	FeatureACP: {
		Name:         FeatureACP,
		DisplayName:  "Agent Communication Protocol",
		Description:  "ACP for inter-agent communication",
		Category:     CategoryProtocol,
		DefaultValue: true, // Default on - core feature
		HeaderName:   "X-Feature-ACP",
		QueryParam:   "acp",
	},
	FeatureLSP: {
		Name:         FeatureLSP,
		DisplayName:  "Language Server Protocol",
		Description:  "LSP for IDE integration and code intelligence",
		Category:     CategoryProtocol,
		DefaultValue: true, // Default on - widely used
		HeaderName:   "X-Feature-LSP",
		QueryParam:   "lsp",
	},
	FeatureGRPC: {
		Name:         FeatureGRPC,
		DisplayName:  "gRPC Protocol",
		Description:  "gRPC for high-performance RPC",
		Category:     CategoryProtocol,
		DefaultValue: false, // Default off - requires specific client setup
		HeaderName:   "X-Feature-gRPC",
		QueryParam:   "grpc",
	},

	// API Features
	FeatureEmbeddings: {
		Name:         FeatureEmbeddings,
		DisplayName:  "Embeddings API",
		Description:  "Vector embeddings generation API",
		Category:     CategoryAPI,
		DefaultValue: true, // Default on - OpenAI compatible
		HeaderName:   "X-Feature-Embeddings",
		QueryParam:   "embeddings",
	},
	FeatureVision: {
		Name:         FeatureVision,
		DisplayName:  "Vision API",
		Description:  "Image analysis and OCR capabilities",
		Category:     CategoryAPI,
		DefaultValue: true, // Default on - widely supported
		HeaderName:   "X-Feature-Vision",
		QueryParam:   "vision",
	},
	FeatureCognee: {
		Name:         FeatureCognee,
		DisplayName:  "Cognee Knowledge Graph",
		Description:  "Knowledge graph and RAG integration",
		Category:     CategoryAPI,
		DefaultValue: false, // Default off - requires Cognee setup
		HeaderName:   "X-Feature-Cognee",
		QueryParam:   "cognee",
	},
	FeatureDebate: {
		Name:         FeatureDebate,
		DisplayName:  "AI Debate System",
		Description:  "Multi-model debate for consensus building",
		Category:     CategoryAPI,
		DefaultValue: true, // Default on - core feature
		HeaderName:   "X-Feature-Debate",
		QueryParam:   "debate",
	},
	FeatureBatchRequests: {
		Name:         FeatureBatchRequests,
		DisplayName:  "Batch Requests",
		Description:  "Support for batched API requests",
		Category:     CategoryAPI,
		DefaultValue: true, // Default on - useful for efficiency
		HeaderName:   "X-Feature-Batch",
		QueryParam:   "batch",
	},
	FeatureToolCalling: {
		Name:         FeatureToolCalling,
		DisplayName:  "Tool Calling",
		Description:  "Function/tool calling support",
		Category:     CategoryAPI,
		DefaultValue: true, // Default on - core feature
		HeaderName:   "X-Feature-ToolCalling",
		QueryParam:   "tool_calling",
	},

	// Advanced Features
	FeatureMultiPass: {
		Name:             FeatureMultiPass,
		DisplayName:      "Multi-Pass Validation",
		Description:      "Multi-pass response validation and improvement",
		Category:         CategoryAdvanced,
		DefaultValue:     false, // Default off - increases latency
		RequiresFeatures: []Feature{FeatureDebate},
		HeaderName:       "X-Feature-MultiPass",
		QueryParam:       "multipass",
	},
	FeatureCaching: {
		Name:         FeatureCaching,
		DisplayName:  "Response Caching",
		Description:  "Cache responses for repeated queries",
		Category:     CategoryAdvanced,
		DefaultValue: true, // Default on - improves performance
		HeaderName:   "X-Feature-Caching",
		QueryParam:   "caching",
	},
	FeatureRateLimiting: {
		Name:         FeatureRateLimiting,
		DisplayName:  "Rate Limiting",
		Description:  "Request rate limiting per client",
		Category:     CategoryAdvanced,
		DefaultValue: true, // Default on - security feature
		HeaderName:   "X-Feature-RateLimiting",
		QueryParam:   "rate_limiting",
	},
	FeatureMetrics: {
		Name:         FeatureMetrics,
		DisplayName:  "Prometheus Metrics",
		Description:  "Prometheus metrics endpoint",
		Category:     CategoryAdvanced,
		DefaultValue: true, // Default on - observability
		HeaderName:   "X-Feature-Metrics",
		QueryParam:   "metrics",
	},
	FeatureTracing: {
		Name:         FeatureTracing,
		DisplayName:  "Distributed Tracing",
		Description:  "Distributed tracing with OpenTelemetry",
		Category:     CategoryAdvanced,
		DefaultValue: false, // Default off - requires setup
		HeaderName:   "X-Feature-Tracing",
		QueryParam:   "tracing",
	},
}

// Registry provides thread-safe access to feature definitions
type Registry struct {
	mu       sync.RWMutex
	features map[Feature]*FeatureInfo
	// customDefaults stores per-endpoint default overrides
	customDefaults map[string]map[Feature]bool
}

// globalRegistry is the singleton feature registry
var globalRegistry *Registry
var registryOnce sync.Once

// GetRegistry returns the global feature registry
func GetRegistry() *Registry {
	registryOnce.Do(func() {
		globalRegistry = &Registry{
			features:       make(map[Feature]*FeatureInfo),
			customDefaults: make(map[string]map[Feature]bool),
		}
		// Copy all features
		for k, v := range featureRegistry {
			info := *v
			globalRegistry.features[k] = &info
		}
	})
	return globalRegistry
}

// GetFeature returns feature info by name
func (r *Registry) GetFeature(name Feature) (*FeatureInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.features[name]
	return info, ok
}

// GetAllFeatures returns all registered features
func (r *Registry) GetAllFeatures() []*FeatureInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	features := make([]*FeatureInfo, 0, len(r.features))
	for _, f := range r.features {
		features = append(features, f)
	}
	return features
}

// GetFeaturesByCategory returns features in a specific category
func (r *Registry) GetFeaturesByCategory(category FeatureCategory) []*FeatureInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var features []*FeatureInfo
	for _, f := range r.features {
		if f.Category == category {
			features = append(features, f)
		}
	}
	return features
}

// GetDefaultValue returns the default value for a feature
func (r *Registry) GetDefaultValue(name Feature) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if info, ok := r.features[name]; ok {
		return info.DefaultValue
	}
	return false
}

// SetEndpointDefaults sets custom default values for a specific endpoint
func (r *Registry) SetEndpointDefaults(endpoint string, defaults map[Feature]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customDefaults[endpoint] = defaults
}

// GetEndpointDefault returns the default value for a feature on a specific endpoint
func (r *Registry) GetEndpointDefault(endpoint string, feature Feature) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check endpoint-specific defaults first
	if endpointDefaults, ok := r.customDefaults[endpoint]; ok {
		if val, ok := endpointDefaults[feature]; ok {
			return val
		}
	}

	// Fall back to global default
	if info, ok := r.features[feature]; ok {
		return info.DefaultValue
	}
	return false
}

// IsValidFeature checks if a feature name is valid
func (r *Registry) IsValidFeature(name Feature) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.features[name]
	return ok
}

// GetFeatureByHeader finds a feature by its header name
func (r *Registry) GetFeatureByHeader(headerName string) (Feature, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, f := range r.features {
		if strings.EqualFold(f.HeaderName, headerName) {
			return f.Name, true
		}
	}
	return "", false
}

// GetFeatureByQueryParam finds a feature by its query parameter
func (r *Registry) GetFeatureByQueryParam(param string) (Feature, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, f := range r.features {
		if strings.EqualFold(f.QueryParam, param) {
			return f.Name, true
		}
	}
	return "", false
}

// ValidateFeatureCombination checks if a set of features can be enabled together
func (r *Registry) ValidateFeatureCombination(features map[Feature]bool) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for feature, enabled := range features {
		if !enabled {
			continue
		}

		info, ok := r.features[feature]
		if !ok {
			continue
		}

		// Check required features
		for _, required := range info.RequiresFeatures {
			if !features[required] {
				return &FeatureValidationError{
					Feature: feature,
					Message: "requires feature: " + string(required),
				}
			}
		}

		// Check conflicting features
		for _, conflict := range info.ConflictsWith {
			if features[conflict] {
				return &FeatureValidationError{
					Feature: feature,
					Message: "conflicts with feature: " + string(conflict),
				}
			}
		}
	}

	return nil
}

// FeatureValidationError represents a feature validation error
type FeatureValidationError struct {
	Feature Feature
	Message string
}

func (e *FeatureValidationError) Error() string {
	return "feature " + string(e.Feature) + ": " + e.Message
}

// String returns the feature name as string
func (f Feature) String() string {
	return string(f)
}

// ParseFeature converts a string to a Feature
func ParseFeature(s string) Feature {
	return Feature(strings.ToLower(s))
}
