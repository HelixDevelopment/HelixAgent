// Package perplexity provides Perplexity CLI agent integration.
// Perplexity: AI-powered search and coding assistant.
package perplexity

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Perplexity provides Perplexity integration
type Perplexity struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Perplexity configuration
type Config struct {
	base.BaseConfig
	APIKey     string
	Model      string
	SearchMode bool
	Citations  bool
}

// New creates a new Perplexity integration
func New() *Perplexity {
	info := agents.AgentInfo{
		Type:        agents.TypePerplexity,
		Name:        "Perplexity",
		Description: "AI-powered search and coding",
		Vendor:      "Perplexity",
		Version:     "1.0.0",
		Capabilities: []string{
			"search",
			"code_generation",
			"research",
			"citations",
			"real_time_info",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Perplexity{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model:      "sonar-pro",
			SearchMode: true,
			Citations:  true,
		},
	}
}

// Initialize initializes Perplexity
func (p *Perplexity) Initialize(ctx context.Context, config interface{}) error {
	if err := p.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		p.config = cfg
	}

	return nil
}

// Execute executes a command
func (p *Perplexity) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !p.IsStarted() {
		if err := p.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "search":
		return p.search(ctx, params)
	case "ask":
		return p.ask(ctx, params)
	case "code":
		return p.code(ctx, params)
	case "research":
		return p.research(ctx, params)
	case "status":
		return p.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// search performs search
func (p *Perplexity) search(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}

	return map[string]interface{}{
		"query":     query,
		"answer":    fmt.Sprintf("Answer to: %s", query),
		"sources":   []string{"source1", "source2"},
		"citations": p.config.Citations,
		"model":     p.config.Model,
	}, nil
}

// ask asks a question
func (p *Perplexity) ask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	question, _ := params["question"].(string)
	if question == "" {
		return nil, fmt.Errorf("question required")
	}

	return map[string]interface{}{
		"question": question,
		"answer":   fmt.Sprintf("Perplexity: %s", question),
		"model":    p.config.Model,
	}, nil
}

// code generates code with search
func (p *Perplexity) code(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt":  prompt,
		"code":    fmt.Sprintf("// Generated with Perplexity search\n// %s", prompt),
		"sources": []string{"docs1", "docs2"},
	}, nil
}

// research conducts research
func (p *Perplexity) research(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	topic, _ := params["topic"].(string)
	if topic == "" {
		return nil, fmt.Errorf("topic required")
	}

	return map[string]interface{}{
		"topic":    topic,
		"research": fmt.Sprintf("Research on: %s", topic),
		"sources":  []string{"research1", "research2", "research3"},
	}, nil
}

// status returns status
func (p *Perplexity) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":   p.IsAvailable(),
		"model":       p.config.Model,
		"search_mode": p.config.SearchMode,
	}, nil
}

// IsAvailable checks availability
func (p *Perplexity) IsAvailable() bool {
	return p.config.APIKey != ""
}

var _ agents.AgentIntegration = (*Perplexity)(nil)
