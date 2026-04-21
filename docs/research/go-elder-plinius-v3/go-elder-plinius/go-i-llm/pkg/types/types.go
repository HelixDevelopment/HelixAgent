// Package types defines Go types for the I-LLM library.
// Go library for I-LLM providing interactive LLM conversation patterns, chain-of-thought templates, ReAct agent implementations, and structured reasoning frameworks.
package types

import (
	"fmt"
	"strings"
)

// ConversationPattern represents conversationpattern data.
type ConversationPattern struct {
	Description string
	Category string
	Template string
	ID string
	Variables []string
	Example string
	Name string
}

// Validate checks that the ConversationPattern is valid.
func (o *ConversationPattern) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// ReActStep represents reactstep data.
type ReActStep struct {
	Action string
	StepNum int
	ActionInput string
	Observation string
	Thought string
}

// AgentConfig represents agentconfig data.
type AgentConfig struct {
	Model string
	Tools []Tool
	Name string
	MaxIterations int
	Temperature float64
	SystemPrompt string
}

// Validate checks that the AgentConfig is valid.
func (o *AgentConfig) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// Defaults applies default values for unset fields.
func (o *AgentConfig) Defaults() {
	if o.Temperature == 0 { o.Temperature = 0.7 }
}

// Tool represents tool data.
type Tool struct {
	Parameters map[string]string
	Description string
	Name string
}

// Validate checks that the Tool is valid.
func (o *Tool) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// ChainResult represents chainresult data.
type ChainResult struct {
	Steps []ReActStep
	FinalOutput string
	Iterations int
	TokenUsage int
	Success bool
}

// PromptChain represents promptchain data.
type PromptChain struct {
	Description string
	Category string
	Steps []ChainStep
	ID string
	Name string
}

// Validate checks that the PromptChain is valid.
func (o *PromptChain) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// Agent represents a ReAct agent instance.
type Agent struct {
	Config AgentConfig
	ID     string
}

// TreeResult represents tree-of-thought exploration result data.
type TreeResult struct {
	Branches    []ChainResult
	FinalOutput string
	Breadth     int
	TokenUsage  int
	Success     bool
}

// ChainStep represents chainstep data.
type ChainStep struct {
	PromptTemplate string
	Condition string
	OutputKey string
	Name string
}

// Validate checks that the ChainStep is valid.
func (o *ChainStep) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

