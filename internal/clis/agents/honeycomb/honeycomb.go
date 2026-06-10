// Package honeycomb provides Honeycomb agent integration.
// Honeycomb: AI-powered observability and debugging. Honeycomb data lives only
// behind its hosted HTTP API (api.honeycomb.io, authed with an API key) — there
// is no local source for query results, traces, or AI analysis. Until that real
// client is wired, query/analyze/trace/alert return an HONEST error rather than
// fabricating results, spans, insights, or an "active" alert (BLUFF-001).
package honeycomb

import (
	"context"
	"errors"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ErrAPINotWired is returned by the data commands because Honeycomb's query,
// trace, AI-analysis, and alert surfaces exist only behind its hosted HTTP API
// and no real client is wired here. Per CONST-035 / BLUFF-001 the integration
// returns this honest error instead of fabricating results, hardcoded spans,
// "AI analysis of <metric>" insights, or an "active" alert.
var ErrAPINotWired = errors.New("honeycomb: the hosted Honeycomb HTTP API client (api.honeycomb.io) is not wired; " +
	"query/analyze/trace/alert require real authed API calls — refusing to fabricate results")

// Honeycomb provides Honeycomb integration
type Honeycomb struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Honeycomb configuration
type Config struct {
	base.BaseConfig
	APIKey  string
	Dataset string
	Service string
}

// New creates a new Honeycomb integration
func New() *Honeycomb {
	info := agents.AgentInfo{
		Type:        agents.TypeHoneycomb,
		Name:        "Honeycomb",
		Description: "AI-powered observability",
		Vendor:      "Honeycomb",
		Version:     "1.0.0",
		Capabilities: []string{
			"observability",
			"debugging",
			"tracing",
			"ai_analysis",
			"anomaly_detection",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Honeycomb{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Dataset: "production",
			Service: "helixagent",
		},
	}
}

// Initialize initializes Honeycomb
func (h *Honeycomb) Initialize(ctx context.Context, config interface{}) error {
	if err := h.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		h.config = cfg
	}

	return nil
}

// Execute executes a command
func (h *Honeycomb) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !h.IsStarted() {
		if err := h.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "query":
		return h.query(ctx, params)
	case "analyze":
		return h.analyze(ctx, params)
	case "trace":
		return h.trace(ctx, params)
	case "alert":
		return h.alert(ctx, params)
	case "status":
		return h.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// query runs a query. Honest error: no real Honeycomb API is wired, so we refuse
// to return a fabricated empty result set as if a query ran (BLUFF-001).
func (h *Honeycomb) query(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}
	return nil, fmt.Errorf("honeycomb query: %w", ErrAPINotWired)
}

// analyze analyzes data. Honest error: no real backend, so we refuse to
// fabricate an "AI analysis of <metric>" string and invented insights (BLUFF-001).
func (h *Honeycomb) analyze(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("honeycomb analyze: %w", ErrAPINotWired)
}

// trace retrieves traces. Honest error: no real backend, so we refuse to
// fabricate hardcoded spans (BLUFF-001).
func (h *Honeycomb) trace(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	traceID, _ := params["trace_id"].(string)
	if traceID == "" {
		return nil, fmt.Errorf("trace_id required")
	}
	return nil, fmt.Errorf("honeycomb trace: %w", ErrAPINotWired)
}

// alert configures alerts. Honest error: no real backend, so we refuse to claim
// an "active" alert was configured without a real API call (BLUFF-001).
func (h *Honeycomb) alert(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	condition, _ := params["condition"].(string)
	if condition == "" {
		return nil, fmt.Errorf("condition required")
	}
	return nil, fmt.Errorf("honeycomb alert: %w", ErrAPINotWired)
}

// status returns status
func (h *Honeycomb) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": h.IsAvailable(),
		"dataset":   h.config.Dataset,
		"service":   h.config.Service,
	}, nil
}

// IsAvailable checks availability
func (h *Honeycomb) IsAvailable() bool {
	return h.config.APIKey != ""
}

var _ agents.AgentIntegration = (*Honeycomb)(nil)
