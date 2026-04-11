// Package superset provides Superset agent integration.
// Superset: AI-powered data visualization and analytics.
package superset

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Superset provides Superset integration
type Superset struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Endpoint string
}

// New creates a new Superset integration
func New() *Superset {
	info := agents.AgentInfo{
		Type:        agents.TypeSuperset,
		Name:        "Superset",
		Description: "AI data visualization",
		Vendor:      "Apache",
		Version:     "1.0.0",
		Capabilities: []string{
			"visualization",
			"analytics",
			"dashboards",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Superset{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Endpoint: "http://localhost:8088",
		},
	}
}

// Initialize initializes Superset
func (s *Superset) Initialize(ctx context.Context, config interface{}) error {
	if err := s.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		s.config = cfg
	}

	return nil
}

// Execute executes a command
func (s *Superset) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !s.IsStarted() {
		if err := s.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "dashboard":
		return s.dashboard(ctx, params)
	case "chart":
		return s.chart(ctx, params)
	case "status":
		return s.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// dashboard creates dashboard
func (s *Superset) dashboard(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	return map[string]interface{}{
		"name": name,
		"url":  fmt.Sprintf("%s/dashboard/%s", s.config.Endpoint, name),
	}, nil
}

// chart creates chart
func (s *Superset) chart(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	chartType, _ := params["type"].(string)
	if chartType == "" {
		chartType = "bar"
	}

	return map[string]interface{}{
		"type": chartType,
		"data": "Chart data",
	}, nil
}

// status returns status
func (s *Superset) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": s.IsAvailable(),
		"endpoint":  s.config.Endpoint,
	}, nil
}

// IsAvailable checks availability
func (s *Superset) IsAvailable() bool {
	return s.config.Endpoint != ""
}

var _ agents.AgentIntegration = (*Superset)(nil)
