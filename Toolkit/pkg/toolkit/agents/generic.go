package agents

import (
	"context"
	"fmt"

	"github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
)

// GenericAgent provides a basic AI assistant implementation
type GenericAgent struct {
	name        string
	description string
	provider    toolkit.Provider
	config      map[string]interface{}
}

// NewGenericAgent creates a new generic agent
func NewGenericAgent(name, description string, provider toolkit.Provider) *GenericAgent {
	return &GenericAgent{
		name:        name,
		description: description,
		provider:    provider,
		config:      make(map[string]interface{}),
	}
}

// Name returns the agent name
func (a *GenericAgent) Name() string {
	return a.name
}

// Execute performs a task using the underlying provider
func (a *GenericAgent) Execute(ctx context.Context, task string, config interface{}) (string, error) {
	// Convert config to map if needed
	var cfg map[string]interface{}
	if config != nil {
		if c, ok := config.(map[string]interface{}); ok {
			cfg = c
		}
	}

	// Prepare the chat request
	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{
			{
				Role:    "system",
				Content: fmt.Sprintf("You are %s, a helpful AI assistant. %s", a.name, a.description),
			},
			{
				Role:    "user",
				Content: task,
			},
		},
		MaxTokens: 1000,
	}

	// Add configuration from agent config
	if model, ok := cfg["model"].(string); ok {
		req.Model = model
	}
	if temp, ok := cfg["temperature"].(float64); ok {
		req.Temperature = temp
	}
	if maxTokens, ok := cfg["max_tokens"].(int); ok {
		req.MaxTokens = maxTokens
	}

	// Execute the chat completion
	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to execute task: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// ValidateConfig validates the agent configuration
func (a *GenericAgent) ValidateConfig(config interface{}) error {
	if config == nil {
		return nil
	}

	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map[string]interface{}")
	}

	// Validate known configuration keys
	validKeys := map[string]bool{
		"model":       true,
		"temperature": true,
		"max_tokens":  true,
		"top_p":       true,
		"top_k":       true,
		"stop":        true,
	}

	for key := range cfg {
		if !validKeys[key] {
			return fmt.Errorf("unknown configuration key: %s", key)
		}
	}

	return nil
}

// Capabilities returns the agent's capabilities
func (a *GenericAgent) Capabilities() []string {
	return []string{
		"chat",
		"task_execution",
		"general_assistance",
	}
}

// SetConfig sets agent configuration
func (a *GenericAgent) SetConfig(key string, value interface{}) {
	a.config[key] = value
}

// GetConfig gets agent configuration
func (a *GenericAgent) GetConfig(key string) interface{} {
	return a.config[key]
}
