// Package strategy provides different execution strategies for Claude Code.
// Each strategy represents a different level of API integration and feature support.
package strategy

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/claude/api"
)

// Strategy defines the interface for Claude Code execution strategies
type Strategy interface {
	// Name returns the strategy name
	Name() string

	// Description returns a description of the strategy
	Description() string

	// Start starts the strategy
	Start(ctx context.Context) error

	// Stop stops the strategy
	Stop(ctx context.Context) error

	// CreateMessage creates a message
	CreateMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error)

	// CreateMessageStream creates a streaming message
	CreateMessageStream(ctx context.Context, req *api.MessageRequest) (<-chan api.StreamEvent, <-chan error)

	// SupportsFeature checks if a feature is supported
	SupportsFeature(feature string) bool

	// GetFeatures returns all supported features
	GetFeatures() []string
}

// Config holds strategy configuration
type Config struct {
	Type         string
	APIKey       string
	OAuthToken   string
	BaseURL      string
	DefaultModel string
}

// Available strategies
const (
	TypeStandard = "standard" // Basic API integration
	TypeFull     = "full"     // Full API + all features (DEFAULT)
	TypeOAuth    = "oauth"    // OAuth-based authentication
	TypeAPIKey   = "api_key"  // API key authentication
)

// New creates a new strategy based on type
func New(strategyType string, client *api.Client, config interface{}) (Strategy, error) {
	switch strategyType {
	case TypeStandard:
		return NewStandardStrategy(client), nil
	case TypeFull:
		return NewFullStrategy(client), nil
	case TypeOAuth:
		return NewOAuthStrategy(client), nil
	case TypeAPIKey:
		return NewAPIKeyStrategy(client), nil
	default:
		return nil, fmt.Errorf("unknown strategy type: %s", strategyType)
	}
}

// BaseStrategy provides common strategy functionality
type BaseStrategy struct {
	name        string
	description string
	client      *api.Client
	features    []string
}

// Name returns the strategy name
func (b *BaseStrategy) Name() string {
	return b.name
}

// Description returns the strategy description
func (b *BaseStrategy) Description() string {
	return b.description
}

// SupportsFeature checks if a feature is supported
func (b *BaseStrategy) SupportsFeature(feature string) bool {
	for _, f := range b.features {
		if f == feature {
			return true
		}
	}
	return false
}

// GetFeatures returns all supported features
func (b *BaseStrategy) GetFeatures() []string {
	return b.features
}

// StandardStrategy provides basic API integration
type StandardStrategy struct {
	BaseStrategy
}

// NewStandardStrategy creates a new standard strategy
func NewStandardStrategy(client *api.Client) *StandardStrategy {
	return &StandardStrategy{
		BaseStrategy: BaseStrategy{
			name:        TypeStandard,
			description: "Basic Claude API integration with standard features",
			client:      client,
			features: []string{
				"messages",
				"streaming",
				"basic_tools",
			},
		},
	}
}

// Start starts the standard strategy
func (s *StandardStrategy) Start(ctx context.Context) error {
	return nil
}

// Stop stops the standard strategy
func (s *StandardStrategy) Stop(ctx context.Context) error {
	return nil
}

// CreateMessage creates a message
func (s *StandardStrategy) CreateMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error) {
	return s.client.CreateMessage(ctx, req)
}

// CreateMessageStream creates a streaming message
func (s *StandardStrategy) CreateMessageStream(ctx context.Context, req *api.MessageRequest) (<-chan api.StreamEvent, <-chan error) {
	return s.client.CreateMessageStream(ctx, req)
}

// FullStrategy provides full API integration with all features (DEFAULT)
type FullStrategy struct {
	BaseStrategy
}

// NewFullStrategy creates a new full strategy
func NewFullStrategy(client *api.Client) *FullStrategy {
	return &FullStrategy{
		BaseStrategy: BaseStrategy{
			name:        TypeFull,
			description: "Full Claude Code integration with all features including MCP, BUDDY, KAIROS, Dream",
			client:      client,
			features: []string{
				"messages",
				"streaming",
				"tools",
				"advanced_tools",
				"mcp",
				"mcp_servers",
				"vision",
				"files",
				"file_upload",
				"buddy",
				"kairos",
				"dream",
				"usage_tracking",
				"rate_limiting",
				"oauth",
				"bootstrap",
				"feature_flags",
				"interleaved_thinking",
				"context_1m",
				"structured_outputs",
				"web_search",
				"fast_mode",
			},
		},
	}
}

// Start starts the full strategy
func (s *FullStrategy) Start(ctx context.Context) error {
	// Full strategy performs additional initialization
	return nil
}

// Stop stops the full strategy
func (s *FullStrategy) Stop(ctx context.Context) error {
	return nil
}

// CreateMessage creates a message with all features enabled
func (s *FullStrategy) CreateMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error) {
	// Full strategy can add additional processing
	return s.client.CreateMessage(ctx, req)
}

// CreateMessageStream creates a streaming message with full features
func (s *FullStrategy) CreateMessageStream(ctx context.Context, req *api.MessageRequest) (<-chan api.StreamEvent, <-chan error) {
	return s.client.CreateMessageStream(ctx, req)
}

// OAuthStrategy provides OAuth-specific optimizations
type OAuthStrategy struct {
	BaseStrategy
}

// NewOAuthStrategy creates a new OAuth strategy
func NewOAuthStrategy(client *api.Client) *OAuthStrategy {
	return &OAuthStrategy{
		BaseStrategy: BaseStrategy{
			name:        TypeOAuth,
			description: "OAuth-optimized strategy with subscription features (Pro/Max/Team/Enterprise)",
			client:      client,
			features: []string{
				"messages",
				"streaming",
				"tools",
				"mcp",
				"vision",
				"files",
				"oauth",
				"subscription_features",
				"extra_usage",
				"grove",
				"team_sharing",
			},
		},
	}
}

// Start starts the OAuth strategy
func (s *OAuthStrategy) Start(ctx context.Context) error {
	return nil
}

// Stop stops the OAuth strategy
func (s *OAuthStrategy) Stop(ctx context.Context) error {
	return nil
}

// CreateMessage creates a message
func (s *OAuthStrategy) CreateMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error) {
	return s.client.CreateMessage(ctx, req)
}

// CreateMessageStream creates a streaming message
func (s *OAuthStrategy) CreateMessageStream(ctx context.Context, req *api.MessageRequest) (<-chan api.StreamEvent, <-chan error) {
	return s.client.CreateMessageStream(ctx, req)
}

// APIKeyStrategy provides API key authentication strategy
type APIKeyStrategy struct {
	BaseStrategy
}

// NewAPIKeyStrategy creates a new API key strategy
func NewAPIKeyStrategy(client *api.Client) *APIKeyStrategy {
	return &APIKeyStrategy{
		BaseStrategy: BaseStrategy{
			name:        TypeAPIKey,
			description: "API key authentication strategy for programmatic access",
			client:      client,
			features: []string{
				"messages",
				"streaming",
				"tools",
				"api_key_auth",
			},
		},
	}
}

// Start starts the API key strategy
func (s *APIKeyStrategy) Start(ctx context.Context) error {
	return nil
}

// Stop stops the API key strategy
func (s *APIKeyStrategy) Stop(ctx context.Context) error {
	return nil
}

// CreateMessage creates a message
func (s *APIKeyStrategy) CreateMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error) {
	return s.client.CreateMessage(ctx, req)
}

// CreateMessageStream creates a streaming message
func (s *APIKeyStrategy) CreateMessageStream(ctx context.Context, req *api.MessageRequest) (<-chan api.StreamEvent, <-chan error) {
	return s.client.CreateMessageStream(ctx, req)
}

// StrategyInfo holds information about available strategies
type StrategyInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	IsDefault   bool     `json:"is_default"`
}

// GetAvailableStrategies returns information about all available strategies
func GetAvailableStrategies() []StrategyInfo {
	return []StrategyInfo{
		{
			Name:        TypeFull,
			Description: "Full Claude Code integration with all features including MCP, BUDDY, KAIROS, Dream",
			Features: []string{
				"messages", "streaming", "tools", "mcp", "vision", "files",
				"buddy", "kairos", "dream", "oauth", "interleaved_thinking",
				"context_1m", "structured_outputs", "web_search", "fast_mode",
			},
			IsDefault: true,
		},
		{
			Name:        TypeOAuth,
			Description: "OAuth-optimized strategy with subscription features (Pro/Max/Team/Enterprise)",
			Features: []string{
				"messages", "streaming", "tools", "mcp", "vision", "files",
				"oauth", "subscription_features", "extra_usage", "grove",
			},
			IsDefault: false,
		},
		{
			Name:        TypeAPIKey,
			Description: "API key authentication strategy for programmatic access",
			Features:    []string{"messages", "streaming", "tools", "api_key_auth"},
			IsDefault:   false,
		},
		{
			Name:        TypeStandard,
			Description: "Basic Claude API integration with standard features",
			Features:    []string{"messages", "streaming", "basic_tools"},
			IsDefault:   false,
		},
	}
}

// GetDefaultStrategy returns the default strategy name
func GetDefaultStrategy() string {
	return TypeFull
}

// IsValidStrategy checks if a strategy type is valid
func IsValidStrategy(strategyType string) bool {
	switch strategyType {
	case TypeStandard, TypeFull, TypeOAuth, TypeAPIKey:
		return true
	default:
		return false
	}
}
