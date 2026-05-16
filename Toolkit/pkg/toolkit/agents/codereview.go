package agents

import (
	"context"
	"fmt"

	"github.com/HelixDevelopment/helix_agent/Toolkit/pkg/toolkit"
)

// CodeReviewAgent provides code review and analysis capabilities
type CodeReviewAgent struct {
	name        string
	description string
	provider    toolkit.Provider
	config      map[string]interface{}
}

// NewCodeReviewAgent creates a new code review agent
func NewCodeReviewAgent(name string, provider toolkit.Provider) *CodeReviewAgent {
	return &CodeReviewAgent{
		name:        name,
		description: "I am a specialized code review agent that analyzes code for bugs, security issues, performance problems, and best practices. I provide detailed feedback and suggestions for improvement.",
		provider:    provider,
		config:      make(map[string]interface{}),
	}
}

// Name returns the agent name
func (a *CodeReviewAgent) Name() string {
	return a.name
}

// Execute performs code review analysis
func (a *CodeReviewAgent) Execute(ctx context.Context, task string, config interface{}) (string, error) {
	// Convert config to map if needed
	var cfg map[string]interface{}
	if config != nil {
		if c, ok := config.(map[string]interface{}); ok {
			cfg = c
		}
	}

	// Prepare the code review prompt
	systemPrompt := `You are an expert code reviewer with deep knowledge of software engineering best practices, security, performance, and maintainability. When reviewing code, consider:

1. **Functionality**: Does the code work correctly? Are there bugs or logical errors?
2. **Security**: Are there security vulnerabilities (e.g., SQL injection, XSS, buffer overflows)?
3. **Performance**: Are there performance issues or inefficiencies?
4. **Maintainability**: Is the code readable, well-structured, and easy to maintain?
5. **Best Practices**: Does it follow language-specific conventions and idioms?
6. **Testing**: Is the code adequately tested?
7. **Documentation**: Is the code properly documented?

Provide specific, actionable feedback with examples where possible. Be constructive and suggest improvements.`

	userPrompt := fmt.Sprintf("Please review the following code:\n\n%s", task)

	// Add language specification if provided
	if language, ok := cfg["language"].(string); ok {
		userPrompt = fmt.Sprintf("Please review the following %s code:\n\n%s", language, task)
	}

	req := toolkit.ChatRequest{
		Messages: []toolkit.Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		MaxTokens:   2000,
		Temperature: 0.3, // Lower temperature for more focused analysis
	}

	// Add configuration from agent config
	if model, ok := cfg["model"].(string); ok {
		req.Model = model
	}
	if maxTokens, ok := cfg["max_tokens"].(int); ok {
		req.MaxTokens = maxTokens
	}

	// Execute the chat completion
	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to perform code review: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// ValidateConfig validates the agent configuration
func (a *CodeReviewAgent) ValidateConfig(config interface{}) error {
	if config == nil {
		return nil
	}

	cfg, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("config must be a map[string]interface{}")
	}

	// Validate known configuration keys
	validKeys := map[string]bool{
		"model":      true,
		"language":   true,
		"max_tokens": true,
	}

	for key := range cfg {
		if !validKeys[key] {
			return fmt.Errorf("unknown configuration key: %s", key)
		}
	}

	return nil
}

// Capabilities returns the agent's capabilities
func (a *CodeReviewAgent) Capabilities() []string {
	return []string{
		"code_review",
		"security_analysis",
		"performance_analysis",
		"best_practices",
		"bug_detection",
	}
}

// SetConfig sets agent configuration
func (a *CodeReviewAgent) SetConfig(key string, value interface{}) {
	a.config[key] = value
}

// GetConfig gets agent configuration
func (a *CodeReviewAgent) GetConfig(key string) interface{} {
	return a.config[key]
}
