// Package types defines Go types for the Leda library.
// Go library for Leda (Mother of Agents) that autonomously generates and operationalizes teams of specialized AI agents from a single user prompt. Creates system prompts for each agent and generates executable scripts to run the multi-agent system with sequential execution and adaptive chain prompting.
package types

import (
	"fmt"
	"strings"
)

// AgentConfig represents agentconfig data.
type AgentConfig struct {
	Role string
	Model string
	Dependencies []string
	Temperature float64
	SystemPrompt string
	Outputs []string
	Description string
	Inputs []string
	Name string
}

// Validate checks that the AgentConfig is valid.
func (o *AgentConfig) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
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

// TeamConfig represents teamconfig data.
type TeamConfig struct {
	Model string
	TeamName string
	ExecutionMode string
	Idea string
	AgentCount int
}

// Validate checks that the TeamConfig is valid.
func (o *TeamConfig) Validate() error {
	if strings.TrimSpace(o.Model) == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}

// GeneratedTeam represents generatedteam data.
type GeneratedTeam struct {
	GeneratedScript string
	TeamName string
	Timestamp string
	ExecutionChain []string
	Agents []AgentConfig
}

// ExecutionResult represents executionresult data.
type ExecutionResult struct {
	Output string
	Input string
	AgentName string
	DurationMs int64
	Error string
	Success bool
}

// ChainResult represents chainresult data.
type ChainResult struct {
	TotalDurationMs int64
	FinalOutput string
	Results []ExecutionResult
	Team GeneratedTeam
}

