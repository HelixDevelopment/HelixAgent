// Package types defines Go types for the Theseus library.
// Go library for the Theseus autonomous agent framework (based on AutoGPT). Provides agent creation, task planning, tool integration, benchmark evaluation, and multi-agent arena competition. Experimental open-source attempt to make GPT-4 fully autonomous.
package types

import (
	"fmt"
	"strings"
)

// AgentConfig represents agentconfig data.
type AgentConfig struct {
	Model string
	MemoryType string
	Tools []string
	MaxIterations int
	Goals []string
	Description string
	Budget float64
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

// Agent represents agent data.
type Agent struct {
	Status string
	CurrentTask string
	TaskHistory []TaskEntry
	ID string
	Config AgentConfig
}

// Validate checks that the Agent is valid.
func (o *Agent) Validate() error {
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// TaskEntry represents taskentry data.
type TaskEntry struct {
	Result string
	Status string
	Timestamp string
	DurationMs int64
	Task string
}

// Validate checks that the TaskEntry is valid.
func (o *TaskEntry) Validate() error {
	if strings.TrimSpace(o.Task) == "" {
		return fmt.Errorf("task is required")
	}
	return nil
}

// BenchmarkConfig represents benchmarkconfig data.
type BenchmarkConfig struct {
	Timeout int
	Iterations int
	ChallengeSet string
	AgentID string
}

// BenchmarkResult represents benchmarkresult data.
type BenchmarkResult struct {
	AvgDurationMs int64
	Score float64
	Details []TaskEntry
	TasksFailed int
	TasksCompleted int
	AgentID string
}

