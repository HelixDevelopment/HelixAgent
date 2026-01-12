package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ToolHandler defines the interface for tool execution
type ToolHandler interface {
	Name() string
	Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error)
	ValidateArgs(args map[string]interface{}) error
	GenerateDefaultArgs(context string) map[string]interface{}
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool        `json:"success"`
	Output  string      `json:"output"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolRegistry manages all tool handlers
type ToolRegistry struct {
	handlers map[string]ToolHandler
	mu       sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		handlers: make(map[string]ToolHandler),
	}
}

// Register adds a tool handler to the registry
func (r *ToolRegistry) Register(handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[strings.ToLower(handler.Name())] = handler
}

// Get returns a tool handler by name
func (r *ToolRegistry) Get(name string) (ToolHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[strings.ToLower(name)]
	return handler, ok
}

// Execute runs a tool by name with the given arguments
func (r *ToolRegistry) Execute(ctx context.Context, toolName string, args map[string]interface{}) (ToolResult, error) {
	handler, ok := r.Get(toolName)
	if !ok {
		return ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", toolName)}, fmt.Errorf("unknown tool: %s", toolName)
	}

	if err := handler.ValidateArgs(args); err != nil {
		return ToolResult{Success: false, Error: err.Error()}, err
	}

	return handler.Execute(ctx, args)
}

// DefaultToolRegistry is the global tool registry with all built-in handlers
var DefaultToolRegistry = NewToolRegistry()

func init() {
	// Register all tool handlers
	DefaultToolRegistry.Register(&GitHandler{})
	DefaultToolRegistry.Register(&TestHandler{})
	DefaultToolRegistry.Register(&LintHandler{})
	DefaultToolRegistry.Register(&DiffHandler{})
	DefaultToolRegistry.Register(&TreeViewHandler{})
	DefaultToolRegistry.Register(&FileInfoHandler{})
	DefaultToolRegistry.Register(&SymbolsHandler{})
	DefaultToolRegistry.Register(&ReferencesHandler{})
	DefaultToolRegistry.Register(&DefinitionHandler{})
	DefaultToolRegistry.Register(&PRHandler{})
	DefaultToolRegistry.Register(&IssueHandler{})
	DefaultToolRegistry.Register(&WorkflowHandler{})
}

// ============================================
// GIT TOOL HANDLER
// ============================================

type GitHandler struct{}

func (h *GitHandler) Name() string { return "Git" }

func (h *GitHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("Git", args)
}

func (h *GitHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	contextLower := strings.ToLower(context)

	// Detect operation from context
	operation := "status"
	description := "Check git status"

	if strings.Contains(contextLower, "commit") {
		operation = "commit"
		description = "Create git commit"
	} else if strings.Contains(contextLower, "push") {
		operation = "push"
		description = "Push changes to remote"
	} else if strings.Contains(contextLower, "pull") {
		operation = "pull"
		description = "Pull changes from remote"
	} else if strings.Contains(contextLower, "branch") {
		operation = "branch"
		description = "List or create branches"
	} else if strings.Contains(contextLower, "checkout") {
		operation = "checkout"
		description = "Checkout branch or file"
	} else if strings.Contains(contextLower, "merge") {
		operation = "merge"
		description = "Merge branches"
	} else if strings.Contains(contextLower, "diff") {
		operation = "diff"
		description = "Show differences"
	} else if strings.Contains(contextLower, "log") {
		operation = "log"
		description = "Show commit history"
	} else if strings.Contains(contextLower, "stash") {
		operation = "stash"
		description = "Stash changes"
	}

	return map[string]interface{}{
		"operation":   operation,
		"description": description,
	}
}

func (h *GitHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	operation, _ := args["operation"].(string)
	arguments, _ := args["arguments"].([]interface{})
	workingDir, _ := args["working_dir"].(string)

	if workingDir == "" {
		workingDir = "."
	}

	cmdArgs := []string{operation}
	for _, arg := range arguments {
		if s, ok := arg.(string); ok {
			cmdArgs = append(cmdArgs, s)
		}
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// TEST TOOL HANDLER
// ============================================

type TestHandler struct{}

func (h *TestHandler) Name() string { return "Test" }

func (h *TestHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("Test", args)
}

func (h *TestHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	contextLower := strings.ToLower(context)

	testPath := "./..."
	testType := "all"
	coverage := false
	description := "Run tests"

	if strings.Contains(contextLower, "coverage") {
		coverage = true
		description = "Run tests with coverage"
	}
	if strings.Contains(contextLower, "unit") {
		testType = "unit"
		testPath = "./internal/..."
		description = "Run unit tests"
	} else if strings.Contains(contextLower, "integration") {
		testType = "integration"
		testPath = "./tests/integration/..."
		description = "Run integration tests"
	} else if strings.Contains(contextLower, "e2e") {
		testType = "e2e"
		testPath = "./tests/e2e/..."
		description = "Run end-to-end tests"
	}

	return map[string]interface{}{
		"test_path":   testPath,
		"test_type":   testType,
		"coverage":    coverage,
		"verbose":     true,
		"description": description,
	}
}

func (h *TestHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	testPath, _ := args["test_path"].(string)
	coverage, _ := args["coverage"].(bool)
	verbose, _ := args["verbose"].(bool)
	filter, _ := args["filter"].(string)
	timeout, _ := args["timeout"].(string)

	if testPath == "" {
		testPath = "./..."
	}
	if timeout == "" {
		timeout = "5m"
	}

	cmdArgs := []string{"test"}
	if verbose {
		cmdArgs = append(cmdArgs, "-v")
	}
	if coverage {
		cmdArgs = append(cmdArgs, "-coverprofile=coverage.out")
	}
	if filter != "" {
		cmdArgs = append(cmdArgs, "-run", filter)
	}
	cmdArgs = append(cmdArgs, "-timeout", timeout)
	cmdArgs = append(cmdArgs, testPath)

	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// LINT TOOL HANDLER
// ============================================

type LintHandler struct{}

func (h *LintHandler) Name() string { return "Lint" }

func (h *LintHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("Lint", args)
}

func (h *LintHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"path":        "./...",
		"linter":      "auto",
		"auto_fix":    false,
		"description": "Run code linting",
	}
}

func (h *LintHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	path, _ := args["path"].(string)
	linter, _ := args["linter"].(string)
	autoFix, _ := args["auto_fix"].(bool)

	if path == "" {
		path = "./..."
	}
	if linter == "" || linter == "auto" {
		linter = "golangci-lint"
	}

	var cmd *exec.Cmd
	switch linter {
	case "golangci-lint":
		cmdArgs := []string{"run"}
		if autoFix {
			cmdArgs = append(cmdArgs, "--fix")
		}
		cmdArgs = append(cmdArgs, path)
		cmd = exec.CommandContext(ctx, "golangci-lint", cmdArgs...)
	case "gofmt":
		if autoFix {
			cmd = exec.CommandContext(ctx, "gofmt", "-w", path)
		} else {
			cmd = exec.CommandContext(ctx, "gofmt", "-d", path)
		}
	case "eslint":
		cmdArgs := []string{path}
		if autoFix {
			cmdArgs = append([]string{"--fix"}, cmdArgs...)
		}
		cmd = exec.CommandContext(ctx, "eslint", cmdArgs...)
	default:
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported linter: %s", linter),
		}, nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// DIFF TOOL HANDLER
// ============================================

type DiffHandler struct{}

func (h *DiffHandler) Name() string { return "Diff" }

func (h *DiffHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("Diff", args)
}

func (h *DiffHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"mode":        "working",
		"description": "Show git diff",
	}
}

func (h *DiffHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	filePath, _ := args["file_path"].(string)
	mode, _ := args["mode"].(string)
	compareWith, _ := args["compare_with"].(string)
	contextLines, _ := args["context_lines"].(float64)

	if mode == "" {
		mode = "working"
	}

	cmdArgs := []string{"diff"}

	if contextLines > 0 {
		cmdArgs = append(cmdArgs, fmt.Sprintf("-U%d", int(contextLines)))
	}

	switch mode {
	case "staged":
		cmdArgs = append(cmdArgs, "--staged")
	case "commit":
		if compareWith != "" {
			cmdArgs = append(cmdArgs, compareWith)
		}
	case "branch":
		if compareWith != "" {
			cmdArgs = append(cmdArgs, compareWith+"...HEAD")
		}
	}

	if filePath != "" {
		cmdArgs = append(cmdArgs, "--", filePath)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// TREEVIEW TOOL HANDLER
// ============================================

type TreeViewHandler struct{}

func (h *TreeViewHandler) Name() string { return "TreeView" }

func (h *TreeViewHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("TreeView", args)
}

func (h *TreeViewHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"path":        ".",
		"max_depth":   3,
		"show_hidden": false,
		"description": "Display directory tree",
	}
}

func (h *TreeViewHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	path, _ := args["path"].(string)
	maxDepth, _ := args["max_depth"].(float64)
	showHidden, _ := args["show_hidden"].(bool)
	ignorePatterns, _ := args["ignore_patterns"].([]interface{})

	if path == "" {
		path = "."
	}
	if maxDepth == 0 {
		maxDepth = 3
	}

	// Build tree using find command
	cmdArgs := []string{path, "-maxdepth", fmt.Sprintf("%d", int(maxDepth))}

	if !showHidden {
		cmdArgs = append(cmdArgs, "-not", "-path", "*/.*")
	}

	for _, pattern := range ignorePatterns {
		if p, ok := pattern.(string); ok {
			cmdArgs = append(cmdArgs, "-not", "-path", "*"+p+"*")
		}
	}

	cmdArgs = append(cmdArgs, "-print")

	cmd := exec.CommandContext(ctx, "find", cmdArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	// Format as tree
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var tree strings.Builder
	for _, line := range lines {
		depth := strings.Count(line, string(filepath.Separator))
		indent := strings.Repeat("│   ", depth)
		name := filepath.Base(line)
		tree.WriteString(fmt.Sprintf("%s├── %s\n", indent, name))
	}

	return ToolResult{
		Success: true,
		Output:  tree.String(),
	}, nil
}

// ============================================
// FILEINFO TOOL HANDLER
// ============================================

type FileInfoHandler struct{}

func (h *FileInfoHandler) Name() string { return "FileInfo" }

func (h *FileInfoHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("FileInfo", args)
}

func (h *FileInfoHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"file_path":     "README.md",
		"include_stats": true,
		"include_git":   false,
		"description":   "Get file information",
	}
}

func (h *FileInfoHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	filePath, _ := args["file_path"].(string)
	includeStats, _ := args["include_stats"].(bool)
	includeGit, _ := args["include_git"].(bool)

	var result strings.Builder

	// Get basic file info using stat
	statCmd := exec.CommandContext(ctx, "stat", filePath)
	statOutput, err := statCmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(statOutput),
			Error:   err.Error(),
		}, nil
	}
	result.WriteString("=== File Information ===\n")
	result.Write(statOutput)

	if includeStats {
		// Get line count
		wcCmd := exec.CommandContext(ctx, "wc", "-l", filePath)
		wcOutput, _ := wcCmd.CombinedOutput()
		result.WriteString("\n=== Line Count ===\n")
		result.Write(wcOutput)
	}

	if includeGit {
		// Get git log for file
		gitCmd := exec.CommandContext(ctx, "git", "log", "--oneline", "-5", "--", filePath)
		gitOutput, _ := gitCmd.CombinedOutput()
		result.WriteString("\n=== Git History (last 5 commits) ===\n")
		result.Write(gitOutput)
	}

	return ToolResult{
		Success: true,
		Output:  result.String(),
	}, nil
}

// ============================================
// SYMBOLS TOOL HANDLER
// ============================================

type SymbolsHandler struct{}

func (h *SymbolsHandler) Name() string { return "Symbols" }

func (h *SymbolsHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("Symbols", args)
}

func (h *SymbolsHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"file_path":   ".",
		"recursive":   false,
		"description": "Extract code symbols",
	}
}

func (h *SymbolsHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	filePath, _ := args["file_path"].(string)
	recursive, _ := args["recursive"].(bool)

	if filePath == "" {
		filePath = "."
	}

	// Use grep to find function/type definitions for Go files
	pattern := "^func |^type |^const |^var "
	cmdArgs := []string{"-n", "-E", pattern}

	if recursive {
		cmdArgs = append([]string{"-r"}, cmdArgs...)
	}
	cmdArgs = append(cmdArgs, filePath)

	cmd := exec.CommandContext(ctx, "grep", cmdArgs...)
	output, _ := cmd.CombinedOutput()

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// REFERENCES TOOL HANDLER
// ============================================

type ReferencesHandler struct{}

func (h *ReferencesHandler) Name() string { return "References" }

func (h *ReferencesHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("References", args)
}

func (h *ReferencesHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"symbol":              "main",
		"include_declaration": true,
		"description":         "Find symbol references",
	}
}

func (h *ReferencesHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	symbol, _ := args["symbol"].(string)
	filePath, _ := args["file_path"].(string)

	if symbol == "" {
		return ToolResult{
			Success: false,
			Error:   "symbol is required",
		}, nil
	}

	searchPath := "."
	if filePath != "" {
		searchPath = filePath
	}

	// Use grep to find references
	cmd := exec.CommandContext(ctx, "grep", "-rn", "--include=*.go", symbol, searchPath)
	output, _ := cmd.CombinedOutput()

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// DEFINITION TOOL HANDLER
// ============================================

type DefinitionHandler struct{}

func (h *DefinitionHandler) Name() string { return "Definition" }

func (h *DefinitionHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("Definition", args)
}

func (h *DefinitionHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"symbol":      "main",
		"description": "Find symbol definition",
	}
}

func (h *DefinitionHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	symbol, _ := args["symbol"].(string)

	if symbol == "" {
		return ToolResult{
			Success: false,
			Error:   "symbol is required",
		}, nil
	}

	// Search for function/type definition
	pattern := fmt.Sprintf("^func %s|^func \\([^)]+\\) %s|^type %s ", symbol, symbol, symbol)
	cmd := exec.CommandContext(ctx, "grep", "-rn", "-E", "--include=*.go", pattern, ".")
	output, _ := cmd.CombinedOutput()

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// PR TOOL HANDLER
// ============================================

type PRHandler struct{}

func (h *PRHandler) Name() string { return "PR" }

func (h *PRHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("PR", args)
}

func (h *PRHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	contextLower := strings.ToLower(context)

	action := "list"
	description := "List pull requests"

	if strings.Contains(contextLower, "create") {
		action = "create"
		description = "Create pull request"
	} else if strings.Contains(contextLower, "merge") {
		action = "merge"
		description = "Merge pull request"
	} else if strings.Contains(contextLower, "view") {
		action = "view"
		description = "View pull request"
	}

	return map[string]interface{}{
		"action":      action,
		"description": description,
	}
}

func (h *PRHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	action, _ := args["action"].(string)
	title, _ := args["title"].(string)
	body, _ := args["body"].(string)
	baseBranch, _ := args["base_branch"].(string)
	prNumber, _ := args["pr_number"].(float64)

	if baseBranch == "" {
		baseBranch = "main"
	}

	var cmd *exec.Cmd
	switch action {
	case "list":
		cmd = exec.CommandContext(ctx, "gh", "pr", "list")
	case "create":
		cmdArgs := []string{"pr", "create", "--base", baseBranch}
		if title != "" {
			cmdArgs = append(cmdArgs, "--title", title)
		}
		if body != "" {
			cmdArgs = append(cmdArgs, "--body", body)
		}
		cmd = exec.CommandContext(ctx, "gh", cmdArgs...)
	case "view":
		if prNumber > 0 {
			cmd = exec.CommandContext(ctx, "gh", "pr", "view", fmt.Sprintf("%d", int(prNumber)))
		} else {
			cmd = exec.CommandContext(ctx, "gh", "pr", "view")
		}
	case "merge":
		if prNumber > 0 {
			cmd = exec.CommandContext(ctx, "gh", "pr", "merge", fmt.Sprintf("%d", int(prNumber)))
		} else {
			return ToolResult{Success: false, Error: "pr_number required for merge"}, nil
		}
	case "close":
		if prNumber > 0 {
			cmd = exec.CommandContext(ctx, "gh", "pr", "close", fmt.Sprintf("%d", int(prNumber)))
		} else {
			return ToolResult{Success: false, Error: "pr_number required for close"}, nil
		}
	default:
		return ToolResult{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// ISSUE TOOL HANDLER
// ============================================

type IssueHandler struct{}

func (h *IssueHandler) Name() string { return "Issue" }

func (h *IssueHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("Issue", args)
}

func (h *IssueHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"action":      "list",
		"description": "List issues",
	}
}

func (h *IssueHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	action, _ := args["action"].(string)
	title, _ := args["title"].(string)
	body, _ := args["body"].(string)
	issueNumber, _ := args["issue_number"].(float64)

	var cmd *exec.Cmd
	switch action {
	case "list":
		cmd = exec.CommandContext(ctx, "gh", "issue", "list")
	case "create":
		cmdArgs := []string{"issue", "create"}
		if title != "" {
			cmdArgs = append(cmdArgs, "--title", title)
		}
		if body != "" {
			cmdArgs = append(cmdArgs, "--body", body)
		}
		cmd = exec.CommandContext(ctx, "gh", cmdArgs...)
	case "view":
		if issueNumber > 0 {
			cmd = exec.CommandContext(ctx, "gh", "issue", "view", fmt.Sprintf("%d", int(issueNumber)))
		} else {
			return ToolResult{Success: false, Error: "issue_number required"}, nil
		}
	case "close":
		if issueNumber > 0 {
			cmd = exec.CommandContext(ctx, "gh", "issue", "close", fmt.Sprintf("%d", int(issueNumber)))
		} else {
			return ToolResult{Success: false, Error: "issue_number required"}, nil
		}
	default:
		return ToolResult{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// WORKFLOW TOOL HANDLER
// ============================================

type WorkflowHandler struct{}

func (h *WorkflowHandler) Name() string { return "Workflow" }

func (h *WorkflowHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("Workflow", args)
}

func (h *WorkflowHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"action":      "list",
		"description": "List workflows",
	}
}

func (h *WorkflowHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	action, _ := args["action"].(string)
	workflowID, _ := args["workflow_id"].(string)
	branch, _ := args["branch"].(string)
	runID, _ := args["run_id"].(float64)

	var cmd *exec.Cmd
	switch action {
	case "list":
		cmd = exec.CommandContext(ctx, "gh", "workflow", "list")
	case "run":
		cmdArgs := []string{"workflow", "run"}
		if workflowID != "" {
			cmdArgs = append(cmdArgs, workflowID)
		}
		if branch != "" {
			cmdArgs = append(cmdArgs, "--ref", branch)
		}
		cmd = exec.CommandContext(ctx, "gh", cmdArgs...)
	case "view":
		if runID > 0 {
			cmd = exec.CommandContext(ctx, "gh", "run", "view", fmt.Sprintf("%d", int(runID)))
		} else {
			cmd = exec.CommandContext(ctx, "gh", "run", "list")
		}
	case "cancel":
		if runID > 0 {
			cmd = exec.CommandContext(ctx, "gh", "run", "cancel", fmt.Sprintf("%d", int(runID)))
		} else {
			return ToolResult{Success: false, Error: "run_id required for cancel"}, nil
		}
	case "logs":
		if runID > 0 {
			cmd = exec.CommandContext(ctx, "gh", "run", "view", fmt.Sprintf("%d", int(runID)), "--log")
		} else {
			return ToolResult{Success: false, Error: "run_id required for logs"}, nil
		}
	default:
		return ToolResult{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}
