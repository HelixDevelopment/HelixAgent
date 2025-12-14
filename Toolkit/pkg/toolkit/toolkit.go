package toolkit

import (
	"fmt"
)

// Toolkit represents the main toolkit instance
type Toolkit struct {
	providers map[string]Provider
	agents    map[string]Agent
}

// NewToolkit creates a new toolkit instance
func NewToolkit() *Toolkit {
	return &Toolkit{
		providers: make(map[string]Provider),
		agents:    make(map[string]Agent),
	}
}

// RegisterProvider registers a provider with the toolkit
func (t *Toolkit) RegisterProvider(name string, provider Provider) {
	t.providers[name] = provider
}

// GetProvider gets a registered provider
func (t *Toolkit) GetProvider(name string) (Provider, error) {
	provider, exists := t.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

// RegisterAgent registers an agent with the toolkit
func (t *Toolkit) RegisterAgent(name string, agent Agent) {
	t.agents[name] = agent
}

// GetAgent gets a registered agent
func (t *Toolkit) GetAgent(name string) (Agent, error) {
	agent, exists := t.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", name)
	}
	return agent, nil
}

// ListProviders returns a list of registered provider names
func (t *Toolkit) ListProviders() []string {
	names := make([]string, 0, len(t.providers))
	for name := range t.providers {
		names = append(names, name)
	}
	return names
}

// ListAgents returns a list of registered agent names
func (t *Toolkit) ListAgents() []string {
	names := make([]string, 0, len(t.agents))
	for name := range t.agents {
		names = append(names, name)
	}
	return names
}
