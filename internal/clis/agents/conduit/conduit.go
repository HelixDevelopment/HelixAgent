// Package conduit provides Conduit agent integration.
// Conduit: AI-powered API integration and automation.
package conduit

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Conduit provides Conduit integration
type Conduit struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Endpoint string
	APIKey   string
}

// New creates a new Conduit integration
func New() *Conduit {
	info := agents.AgentInfo{
		Type:        agents.TypeConduit,
		Name:        "Conduit",
		Description: "API integration and automation",
		Vendor:      "Conduit",
		Version:     "1.0.0",
		Capabilities: []string{
			"api_integration",
			"automation",
			"webhooks",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Conduit{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
		},
	}
}

// Initialize initializes Conduit
func (c *Conduit) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a command
func (c *Conduit) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "connect":
		return c.connect(ctx, params)
	case "send":
		return c.send(ctx, params)
	case "status":
		return c.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// connect connects to API
func (c *Conduit) connect(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	endpoint, _ := params["endpoint"].(string)
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint required")
	}

	c.config.Endpoint = endpoint

	return map[string]interface{}{
		"endpoint": endpoint,
		"status":   "connected",
	}, nil
}

// errNoSendWired is returned by send: no real Conduit transport is wired, and
// no confirmed headless Conduit CLI/API invocation was found in the 2026-06-10
// blocked-agent research. Reporting a "sent" status without actually sending
// would be a BLUFF-001 violation — so send returns an honest error rather than
// fabricating a successful delivery.
var errNoSendWired = fmt.Errorf(
	"conduit send is not wired to a real transport (research-blocked: no confirmed " +
		"headless Conduit CLI/API); cannot report a send without performing one")

// send validates input then returns an honest not-wired error rather than a
// fabricated "sent" status (BLUFF-001).
func (c *Conduit) send(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	data, _ := params["data"].(string)
	if data == "" {
		return nil, fmt.Errorf("data required")
	}

	return nil, errNoSendWired
}

// status returns status
func (c *Conduit) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": c.IsAvailable(),
		"endpoint":  c.config.Endpoint,
	}, nil
}

// IsAvailable checks availability
func (c *Conduit) IsAvailable() bool {
	return c.config.Endpoint != ""
}

var _ agents.AgentIntegration = (*Conduit)(nil)
