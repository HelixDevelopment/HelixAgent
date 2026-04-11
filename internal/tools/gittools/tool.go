// Package gittools provides git operations as a tool
package gittools

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

// Tool provides git operations as a tool
type Tool struct {
	autoCommit *AutoCommit
	logger     *logrus.Logger
}

// ToolResult represents the result of tool execution
type ToolResult struct {
	Success bool    `json:"success"`
	Commit  *Commit `json:"commit,omitempty"`
	Output  string  `json:"output,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// NewTool creates a new git tools handler
func NewTool(basePath string, logger *logrus.Logger) *Tool {
	if logger == nil {
		logger = logrus.New()
	}
	config := DefaultAutoCommitConfig()
	return &Tool{
		autoCommit: NewAutoCommit(basePath, config, logger),
		logger:     logger,
	}
}

// Name returns the tool name
func (t *Tool) Name() string {
	return "GitTools"
}

// Description returns the tool description
func (t *Tool) Description() string {
	return "Git operations: commit, push, pull, branch management"
}

// Schema returns the tool schema
func (t *Tool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"commit", "push", "pull", "status", "branch", "stash"},
				"description": "Git operation to perform",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Commit message (for commit operation)",
			},
			"remote": map[string]interface{}{
				"type":        "string",
				"description": "Remote name (for push/pull)",
				"default":     "origin",
			},
			"branch": map[string]interface{}{
				"type":        "string",
				"description": "Branch name",
				"default":     "main",
			},
			"generate_message": map[string]interface{}{
				"type":        "boolean",
				"description": "Auto-generate commit message",
				"default":     true,
			},
		},
		"required": []string{"operation"},
	}
}

// Execute runs the git tool
func (t *Tool) Execute(ctx context.Context, input map[string]interface{}) (*ToolResult, error) {
	operation, ok := input["operation"].(string)
	if !ok || operation == "" {
		return &ToolResult{
			Success: false,
			Error:   "operation is required",
		}, nil
	}

	switch operation {
	case "commit":
		return t.handleCommit(ctx, input)
	case "push":
		return t.handlePush(ctx, input)
	case "pull":
		return t.handlePull(ctx, input)
	case "status":
		return t.handleStatus(ctx)
	case "branch":
		return t.handleBranch(ctx, input)
	case "stash":
		return t.handleStash(ctx, input)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown operation: %s", operation),
		}, nil
	}
}

func (t *Tool) handleCommit(ctx context.Context, input map[string]interface{}) (*ToolResult, error) {
	message := ""
	if msg, ok := input["message"].(string); ok {
		message = msg
	}

	// Override generate message setting if specified
	if gen, ok := input["generate_message"].(bool); ok {
		t.autoCommit.config.GenerateMessage = gen
	}

	commit, err := t.autoCommit.CommitChanges(ctx, message)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Commit:  commit,
		Output:  fmt.Sprintf("Committed %d file(s) with hash %s", len(commit.Files), commit.Hash[:7]),
	}, nil
}

func (t *Tool) handlePush(ctx context.Context, input map[string]interface{}) (*ToolResult, error) {
	remote := "origin"
	if r, ok := input["remote"].(string); ok && r != "" {
		remote = r
	}

	branch := ""
	if b, ok := input["branch"].(string); ok && b != "" {
		branch = b
	} else {
		// Get current branch
		var err error
		branch, err = t.autoCommit.GetCurrentBranch(ctx)
		if err != nil {
			branch = "main"
		}
	}

	if err := t.autoCommit.Push(ctx, remote, branch); err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Pushed to %s/%s", remote, branch),
	}, nil
}

func (t *Tool) handlePull(ctx context.Context, input map[string]interface{}) (*ToolResult, error) {
	remote := "origin"
	if r, ok := input["remote"].(string); ok && r != "" {
		remote = r
	}

	branch := ""
	if b, ok := input["branch"].(string); ok && b != "" {
		branch = b
	} else {
		// Get current branch
		var err error
		branch, err = t.autoCommit.GetCurrentBranch(ctx)
		if err != nil {
			branch = "main"
		}
	}

	if err := t.autoCommit.Pull(ctx, remote, branch); err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Pulled from %s/%s", remote, branch),
	}, nil
}

func (t *Tool) handleStatus(ctx context.Context) (*ToolResult, error) {
	changes, err := t.autoCommit.GetChanges()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if len(changes) == 0 {
		return &ToolResult{
			Success: true,
			Output:  "No uncommitted changes",
		}, nil
	}

	var output string
	for _, change := range changes {
		output += fmt.Sprintf("%s: %s\n", change.Operation, change.Path)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
	}, nil
}

func (t *Tool) handleBranch(ctx context.Context, input map[string]interface{}) (*ToolResult, error) {
	action := "current"
	if a, ok := input["action"].(string); ok {
		action = a
	}

	branchName := ""
	if b, ok := input["branch"].(string); ok {
		branchName = b
	}

	switch action {
	case "create":
		if branchName == "" {
			return &ToolResult{
				Success: false,
				Error:   "branch name is required for create",
			}, nil
		}
		if err := t.autoCommit.CreateBranch(ctx, branchName); err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Created branch %s", branchName),
		}, nil

	case "switch":
		if branchName == "" {
			return &ToolResult{
				Success: false,
				Error:   "branch name is required for switch",
			}, nil
		}
		if err := t.autoCommit.SwitchBranch(ctx, branchName); err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Switched to branch %s", branchName),
		}, nil

	default:
		current, err := t.autoCommit.GetCurrentBranch(ctx)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Current branch: %s", current),
		}, nil
	}
}

func (t *Tool) handleStash(ctx context.Context, input map[string]interface{}) (*ToolResult, error) {
	action := "push"
	if a, ok := input["action"].(string); ok {
		action = a
	}

	switch action {
	case "push":
		message := ""
		if m, ok := input["message"].(string); ok {
			message = m
		}
		if err := t.autoCommit.Stash(ctx, message); err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return &ToolResult{
			Success: true,
			Output:  "Changes stashed",
		}, nil

	case "pop":
		if err := t.autoCommit.PopStash(ctx); err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return &ToolResult{
			Success: true,
			Output:  "Stash popped",
		}, nil

	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown stash action: %s", action),
		}, nil
	}
}

// Commit performs a commit operation
func (t *Tool) Commit(ctx context.Context, message string) (*ToolResult, error) {
	return t.Execute(ctx, map[string]interface{}{
		"operation": "commit",
		"message":   message,
	})
}

// Push performs a push operation
func (t *Tool) Push(ctx context.Context, remote, branch string) (*ToolResult, error) {
	return t.Execute(ctx, map[string]interface{}{
		"operation": "push",
		"remote":    remote,
		"branch":    branch,
	})
}

// Pull performs a pull operation
func (t *Tool) Pull(ctx context.Context, remote, branch string) (*ToolResult, error) {
	return t.Execute(ctx, map[string]interface{}{
		"operation": "pull",
		"remote":    remote,
		"branch":    branch,
	})
}

// Status shows current status
func (t *Tool) Status(ctx context.Context) (*ToolResult, error) {
	return t.Execute(ctx, map[string]interface{}{
		"operation": "status",
	})
}
