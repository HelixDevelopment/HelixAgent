// Package kiro provides Kiro CLI agent integration.
// Kiro: AI-powered memory and context management for coding.
package kiro

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Kiro provides Kiro CLI integration
type Kiro struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Kiro configuration
type Config struct {
	base.BaseConfig
	MemoryDir     string
	ContextWindow int
	AutoRecall    bool
}

// New creates a new Kiro integration
func New() *Kiro {
	info := agents.AgentInfo{
		Type:        agents.TypeKiro,
		Name:        "Kiro",
		Description: "AI-powered memory and context management",
		Vendor:      "Kiro",
		Version:     "1.0.0",
		Capabilities: []string{
			"memory_management",
			"context_awareness",
			"pattern_recognition",
			"knowledge_retrieval",
			"code_understanding",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Kiro{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			ContextWindow: 128000,
			AutoRecall:    true,
		},
	}
}

// Initialize initializes Kiro
func (k *Kiro) Initialize(ctx context.Context, config interface{}) error {
	if err := k.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		k.config = cfg
	}

	// Set up memory directory
	if k.config.MemoryDir == "" {
		home, _ := os.UserHomeDir()
		k.config.MemoryDir = filepath.Join(home, ".kiro", "memories")
	}

	// Create memory directory
	os.MkdirAll(k.config.MemoryDir, 0755)

	return nil
}

// Execute executes a Kiro command
func (k *Kiro) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !k.IsStarted() {
		if err := k.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "remember":
		return k.remember(ctx, params)
	case "recall":
		return k.recall(ctx, params)
	case "context":
		return k.getContext(ctx, params)
	case "sync":
		return k.sync(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// remember stores a memory
func (k *Kiro) remember(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	content, _ := params["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("content required")
	}

	tag, _ := params["tag"].(string)

	args := []string{"remember", content}
	if tag != "" {
		args = append(args, "--tag", tag)
	}

	output, err := k.ExecuteCommand(ctx, "kiro", args...)
	if err != nil {
		return nil, fmt.Errorf("kiro remember failed: %w", err)
	}

	return map[string]interface{}{
		"stored": true,
		"tag":    tag,
		"output": string(output),
	}, nil
}

// recall retrieves a memory
func (k *Kiro) recall(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}

	args := []string{"recall", query}

	limit, ok := params["limit"].(int)
	if ok && limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", limit))
	}

	output, err := k.ExecuteCommand(ctx, "kiro", args...)
	if err != nil {
		return nil, fmt.Errorf("kiro recall failed: %w", err)
	}

	return map[string]interface{}{
		"query":    query,
		"memories": string(output),
	}, nil
}

// getContext gets context for a task
func (k *Kiro) getContext(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}

	args := []string{"context", task}

	output, err := k.ExecuteCommand(ctx, "kiro", args...)
	if err != nil {
		return nil, fmt.Errorf("kiro context failed: %w", err)
	}

	return map[string]interface{}{
		"task":    task,
		"context": string(output),
	}, nil
}

// sync syncs memories
func (k *Kiro) sync(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	output, err := k.ExecuteCommand(ctx, "kiro", "sync")
	if err != nil {
		return nil, fmt.Errorf("kiro sync failed: %w", err)
	}

	return map[string]interface{}{
		"synced": true,
		"output": string(output),
	}, nil
}

// IsAvailable checks if Kiro is installed
func (k *Kiro) IsAvailable() bool {
	_, err := exec.LookPath("kiro")
	return err == nil
}

var _ agents.AgentIntegration = (*Kiro)(nil)
