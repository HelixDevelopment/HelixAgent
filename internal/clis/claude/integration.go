// Package claude provides full Claude Code CLI integration for HelixAgent.
// This package wires all Claude Code features into the HelixAgent system.
package claude

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"dev.helix.agent/internal/clis/claude/api"
	"dev.helix.agent/internal/clis/claude/features"
	"dev.helix.agent/internal/clis/claude/strategy"
)

// Integration provides the main entry point for Claude Code integration
type Integration struct {
	// API clients
	client   *api.Client
	mcpAPI   *api.MCPProxyAPI
	filesAPI *api.FilesAPI

	// Feature managers
	featureFlags   *api.FeatureFlagManager
	usageTracker   *api.UsageTracker
	rateChecker    *api.RateLimitChecker
	bootstrapCache *api.BootstrapCache

	// Internal features
	buddy  *features.BuddySystem
	kairos *features.KAIROS
	dream  *features.DreamSystem

	// Strategy
	strategy strategy.Strategy

	// Configuration
	config *Config

	// State
	mu        sync.RWMutex
	started   bool
	tokenInfo *api.TokenInfo
}

// Config holds Claude Code integration configuration
type Config struct {
	// Authentication
	APIKey       string
	OAuthToken   string
	RefreshToken string

	// Environment
	BaseURL     string // Production, Staging, or Custom
	MCPProxyURL string

	// Features - ALL ENABLED by default
	EnableAllFeatures bool

	// Individual feature toggles (override EnableAllFeatures)
	EnableStreaming     bool
	EnableTools         bool
	EnableMCP           bool
	EnableVision        bool
	EnableFiles         bool
	EnableBuddy         bool
	EnableKAIROS        bool
	EnableDream         bool
	EnableUsageTracking bool
	EnableRateLimiting  bool

	// Strategy configuration
	StrategyType string // "full", "oauth", "api_key", "standard"
	DefaultModel string

	// Cache settings
	BootstrapCacheTTL time.Duration
	FileCacheDir      string
	FileCacheMaxSize  int64
}

// DefaultConfig returns a default configuration with ALL features enabled
func DefaultConfig() *Config {
	return &Config{
		BaseURL:             api.ProductionBaseURL,
		EnableAllFeatures:   true,
		EnableStreaming:     true,
		EnableTools:         true,
		EnableMCP:           true,
		EnableVision:        true,
		EnableFiles:         true,
		EnableBuddy:         true,
		EnableKAIROS:        true,
		EnableDream:         true,
		EnableUsageTracking: true,
		EnableRateLimiting:  true,
		StrategyType:        "full",
		DefaultModel:        api.ModelClaudeSonnet4_5,
		BootstrapCacheTTL:   5 * time.Minute,
		FileCacheDir:        "/tmp/claude-cache",
		FileCacheMaxSize:    1024 * 1024 * 1024, // 1GB
	}
}

// NewIntegration creates a new Claude Code integration
func NewIntegration(config *Config) (*Integration, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create API client with all beta headers
	opts := []api.ClientOption{
		api.WithBetaHeaders(
			api.BetaOAuth,
			api.BetaInterleavedThinking,
			api.BetaContext1M,
			api.BetaStructuredOutputs,
			api.BetaWebSearch,
			api.BetaFastMode,
		),
	}

	if config.BaseURL != "" {
		opts = append(opts, api.WithBaseURL(config.BaseURL))
	}

	if config.APIKey != "" {
		opts = append(opts, api.WithAPIKey(config.APIKey))
	} else if config.OAuthToken != "" {
		opts = append(opts, api.WithOAuthToken(config.OAuthToken))
	}

	client := api.NewClient(opts...)

	// Create bootstrap cache
	bootstrapCache := api.NewBootstrapCache(client, config.BootstrapCacheTTL)
	featureFlags := api.NewFeatureFlagManager(bootstrapCache)

	// Create integration
	integration := &Integration{
		client:         client,
		mcpAPI:         api.NewMCPProxyAPI(client),
		filesAPI:       api.NewFilesAPI(client),
		featureFlags:   featureFlags,
		usageTracker:   api.NewUsageTracker(),
		rateChecker:    api.NewRateLimitChecker(),
		bootstrapCache: bootstrapCache,
		config:         config,
	}

	// Initialize strategy
	var err error
	integration.strategy, err = strategy.New(config.StrategyType, client, config)
	if err != nil {
		return nil, fmt.Errorf("create strategy: %w", err)
	}

	return integration, nil
}

// Start initializes and starts all Claude Code features
func (i *Integration) Start(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.started {
		return nil
	}

	log.Println("[Claude] Starting Claude Code integration...")

	// Fetch bootstrap configuration
	bootstrap, err := i.bootstrapCache.Get(ctx)
	if err != nil {
		log.Printf("[Claude] Warning: Failed to fetch bootstrap config: %v", err)
		// Continue with defaults
	} else {
		log.Printf("[Claude] Bootstrap config loaded. Minimum version: %s", bootstrap.ClientData.MinimumVersion)
	}

	// Check and display enabled features
	flags, _ := i.featureFlags.GetFlags(ctx)
	if flags != nil {
		log.Printf("[Claude] Feature flags loaded:")
		log.Printf("  - New Models: %v", flags.NewModelsEnabled)
		log.Printf("  - Context 1M: %v", flags.Context1MEnabled)
		log.Printf("  - Web Search: %v", flags.WebSearchEnabled)
		log.Printf("  - MCP: %v", flags.MCPEnabled)
		log.Printf("  - Buddy: %v", flags.BuddySystemEnabled)
		log.Printf("  - KAIROS: %v", flags.KAIROSEnabled)
		log.Printf("  - Dream: %v", flags.DreamSystemEnabled)
	}

	// Initialize internal features if enabled
	if i.config.EnableAllFeatures || i.config.EnableBuddy {
		i.buddy = features.NewBuddySystem()
		log.Println("[Claude] BUDDY system initialized")
	}

	if i.config.EnableAllFeatures || i.config.EnableKAIROS {
		i.kairos = features.NewKAIROS(i.client)
		log.Println("[Claude] KAIROS initialized")
	}

	if i.config.EnableAllFeatures || i.config.EnableDream {
		i.dream = features.NewDreamSystem()
		log.Println("[Claude] Dream system initialized")
	}

	// Start strategy
	if err := i.strategy.Start(ctx); err != nil {
		return fmt.Errorf("start strategy: %w", err)
	}

	i.started = true
	log.Println("[Claude] Integration started successfully")

	return nil
}

// Stop stops all Claude Code features
func (i *Integration) Stop(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.started {
		return nil
	}

	log.Println("[Claude] Stopping Claude Code integration...")

	// Stop internal features
	if i.kairos != nil {
		i.kairos.Stop()
	}

	if i.dream != nil {
		i.dream.Stop()
	}

	// Stop strategy
	if err := i.strategy.Stop(ctx); err != nil {
		return fmt.Errorf("stop strategy: %w", err)
	}

	i.started = false
	log.Println("[Claude] Integration stopped")

	return nil
}

// IsStarted returns whether the integration is started
func (i *Integration) IsStarted() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.started
}

// CreateMessage creates a message using the configured strategy
func (i *Integration) CreateMessage(ctx context.Context, req *api.MessageRequest) (*api.MessageResponse, error) {
	if !i.started {
		return nil, fmt.Errorf("integration not started")
	}

	// Check rate limits
	if i.config.EnableRateLimiting && i.rateChecker.ShouldCheck() {
		usage, err := i.client.GetUsage(ctx)
		if err != nil {
			log.Printf("[Claude] Warning: Failed to check usage: %v", err)
		} else {
			i.rateChecker.UpdateUsage(usage)
			if i.rateChecker.IsRateLimited(90) {
				return nil, fmt.Errorf("rate limit approaching threshold")
			}
		}
	}

	// Execute via strategy
	resp, err := i.strategy.CreateMessage(ctx, req)
	if err != nil {
		return nil, err
	}

	// Track usage
	if i.config.EnableUsageTracking {
		i.usageTracker.TrackRequest(resp.Usage)
	}

	return resp, nil
}

// CreateMessageStream creates a streaming message
func (i *Integration) CreateMessageStream(ctx context.Context, req *api.MessageRequest) (<-chan api.StreamEvent, <-chan error) {
	if !i.started {
		errors := make(chan error, 1)
		errors <- fmt.Errorf("integration not started")
		close(errors)
		return nil, errors
	}

	return i.strategy.CreateMessageStream(ctx, req)
}

// CallTool calls an MCP tool
func (i *Integration) CallTool(ctx context.Context, serverID string, tool string, params map[string]interface{}) (*api.MCPCallResponse, error) {
	if !i.started {
		return nil, fmt.Errorf("integration not started")
	}

	if !i.config.EnableMCP && !i.config.EnableAllFeatures {
		return nil, fmt.Errorf("MCP not enabled")
	}

	req := &api.MCPCallRequest{
		Tool:   tool,
		Params: params,
	}

	return i.mcpAPI.CallTool(ctx, serverID, req)
}

// GetBuddy returns the BUDDY system (creates if needed)
func (i *Integration) GetBuddy() (*features.BuddySystem, error) {
	if !i.config.EnableBuddy && !i.config.EnableAllFeatures {
		return nil, fmt.Errorf("BUDDY system not enabled")
	}

	if i.buddy == nil {
		return nil, fmt.Errorf("BUDDY system not initialized")
	}

	return i.buddy, nil
}

// GetKAIROS returns the KAIROS system
func (i *Integration) GetKAIROS() (*features.KAIROS, error) {
	if !i.config.EnableKAIROS && !i.config.EnableAllFeatures {
		return nil, fmt.Errorf("KAIROS not enabled")
	}

	if i.kairos == nil {
		return nil, fmt.Errorf("KAIROS not initialized")
	}

	return i.kairos, nil
}

// GetDreamSystem returns the Dream system
func (i *Integration) GetDreamSystem() (*features.DreamSystem, error) {
	if !i.config.EnableDream && !i.config.EnableAllFeatures {
		return nil, fmt.Errorf("Dream system not enabled")
	}

	if i.dream == nil {
		return nil, fmt.Errorf("Dream system not initialized")
	}

	return i.dream, nil
}

// GetUsageStats returns usage statistics
func (i *Integration) GetUsageStats() map[string]interface{} {
	return map[string]interface{}{
		"requests":               i.usageTracker.Requests,
		"total_input_tokens":     i.usageTracker.TotalInputTokens,
		"total_output_tokens":    i.usageTracker.TotalOutputTokens,
		"total_tokens":           i.usageTracker.TotalTokens(),
		"avg_tokens_per_request": i.usageTracker.AverageTokensPerRequest(),
		"estimated_cost":         i.usageTracker.CostEstimate(i.config.DefaultModel),
	}
}

// RefreshToken refreshes the OAuth token if needed
func (i *Integration) RefreshToken(ctx context.Context) error {
	if i.tokenInfo == nil || !i.tokenInfo.IsExpired() {
		return nil
	}

	config := api.ClaudeAIOAuthConfig
	newToken, err := i.client.RefreshToken(ctx, config, i.tokenInfo.RefreshToken)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}

	i.tokenInfo = newToken
	return nil
}

// GetEnabledFeatures returns a list of enabled features
func (i *Integration) GetEnabledFeatures() []string {
	var features []string

	if i.config.EnableAllFeatures {
		features = append(features, "all")
		return features
	}

	if i.config.EnableStreaming {
		features = append(features, "streaming")
	}
	if i.config.EnableTools {
		features = append(features, "tools")
	}
	if i.config.EnableMCP {
		features = append(features, "mcp")
	}
	if i.config.EnableVision {
		features = append(features, "vision")
	}
	if i.config.EnableFiles {
		features = append(features, "files")
	}
	if i.config.EnableBuddy {
		features = append(features, "buddy")
	}
	if i.config.EnableKAIROS {
		features = append(features, "kairos")
	}
	if i.config.EnableDream {
		features = append(features, "dream")
	}
	if i.config.EnableUsageTracking {
		features = append(features, "usage_tracking")
	}
	if i.config.EnableRateLimiting {
		features = append(features, "rate_limiting")
	}

	return features
}

// HealthCheck performs a health check on the integration
func (i *Integration) HealthCheck(ctx context.Context) error {
	if !i.started {
		return fmt.Errorf("integration not started")
	}

	// Try to get bootstrap config
	_, err := i.bootstrapCache.Get(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap check failed: %w", err)
	}

	return nil
}

// Singleton instance
var (
	instance *Integration
	once     sync.Once
)

// GetInstance returns the singleton integration instance
func GetInstance() *Integration {
	return instance
}

// Initialize initializes the singleton instance
func Initialize(config *Config) error {
	var err error
	once.Do(func() {
		instance, err = NewIntegration(config)
	})
	return err
}
