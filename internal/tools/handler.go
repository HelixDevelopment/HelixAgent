package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"digital.vasic.concurrency/pkg/safe"
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

// ToolRegistry manages all tool handlers.
//
// Concurrent-safe by construction: handlers is a safe.Store so Register
// and Get need no external lock; the prior RWMutex is gone.
type ToolRegistry struct {
	handlers *safe.Store[string, ToolHandler]
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		handlers: safe.NewStore[string, ToolHandler](),
	}
}

// Register adds a tool handler to the registry
func (r *ToolRegistry) Register(handler ToolHandler) {
	r.handlers.Put(strings.ToLower(handler.Name()), handler)
}

// Get returns a tool handler by name
func (r *ToolRegistry) Get(name string) (ToolHandler, bool) {
	return r.handlers.Get(strings.ToLower(name))
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

var (
	defaultToolRegistry     *ToolRegistry
	defaultToolRegistryOnce sync.Once
)

// GetDefaultToolRegistry returns the global tool registry with all built-in handlers,
// initialized lazily on first access via sync.Once.
func GetDefaultToolRegistry() *ToolRegistry {
	defaultToolRegistryOnce.Do(func() {
		r := NewToolRegistry()
		r.Register(&ReadFileHandler{})
		r.Register(&GitHandler{})
		r.Register(&TestHandler{})
		r.Register(&LintHandler{})
		r.Register(&DiffHandler{})
		r.Register(&TreeViewHandler{})
		r.Register(&FileInfoHandler{})
		r.Register(&SymbolsHandler{})
		r.Register(&ReferencesHandler{})
		r.Register(&DefinitionHandler{})
		r.Register(&PRHandler{})
		r.Register(&IssueHandler{})
		r.Register(&WorkflowHandler{})
		defaultToolRegistry = r
	})
	return defaultToolRegistry
}

// ============================================
// READ FILE TOOL HANDLER
// ============================================

// ReadFileHandler handles the read_file tool which reads contents of files from the filesystem.
// Supports reading entire files or specific line ranges using offset/limit parameters.
// Uses cat for full file reads and sed for line-range reads.
//
// Parameters:
//   - file_path (required): Absolute path to the file to read
//   - offset (optional): Line number to start reading from (0-indexed)
//   - limit (optional): Number of lines to read (default: 2000)
//
// Examples:
//   - Read entire file: {"file_path": "/path/to/file.txt"}
//   - Read lines 10-20: {"file_path": "/path/to/file.txt", "offset": 10, "limit": 10}
type ReadFileHandler struct{}

func (h *ReadFileHandler) Name() string { return "read_file" }

func (h *ReadFileHandler) ValidateArgs(args map[string]interface{}) error {
	return ValidateToolArgs("read_file", args)
}

func (h *ReadFileHandler) GenerateDefaultArgs(context string) map[string]interface{} {
	return map[string]interface{}{
		"file_path":   "README.md",
		"offset":      0,
		"limit":       2000,
		"description": "Read file contents",
	}
}

func (h *ReadFileHandler) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	filePath, _ := args["file_path"].(string) //nolint:errcheck
	offset, _ := args["offset"].(float64)     //nolint:errcheck
	limit, _ := args["limit"].(float64)       //nolint:errcheck

	if filePath == "" {
		return ToolResult{
			Success: false,
			Error:   "file_path is required",
		}, nil
	}

	// Validate file path
	if !ValidatePath(filePath) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid file path: %s", filePath),
		}, nil
	}

	// Default limits
	if limit == 0 {
		limit = 2000
	}
	if offset < 0 {
		offset = 0
	}

	// Read file using cat with line range if offset/limit specified
	var cmd *exec.Cmd
	if offset > 0 || limit < 999999 {
		// Use sed to read specific line range
		// sed -n 'offset,offset+limit-1p' file
		endLine := int(offset + limit)
		startLine := int(offset) + 1 // sed uses 1-based indexing
		sedExpr := fmt.Sprintf("%d,%dp", startLine, endLine)
		cmd = exec.CommandContext(ctx, "sed", "-n", sedExpr, filePath) // #nosec G204
	} else {
		// Read entire file
		cmd = exec.CommandContext(ctx, "cat", filePath) // #nosec G204
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
// GIT TOOL HANDLER
// ============================================

// GitHandler handles git version control operations.
// Supports all standard git commands with validation to prevent command injection.
//
// Supported operations:
//   - clone, pull, push, checkout, merge, diff, log, stash
//   - status, add, commit, branch, tag, fetch, rebase, reset
//   - clean, init, remote, show, mv, rm
//
// Parameters:
//   - operation (required): Git command to execute (from allowed list)
//   - arguments (optional): Array of additional arguments for the command
//   - working_dir (optional): Working directory for the git command (default: ".")
//   - description (required): Human-readable description of what the operation does
//
// Security:
//   - Operations are validated against an allowed list
//   - Arguments are validated for shell safety
//   - Working directory path is validated
//
// Examples:
//   - Check status: {"operation": "status", "description": "Check git status"}
//   - Commit: {"operation": "commit", "arguments": ["-m", "Fix bug"], "description": "Commit bug fix"}
//   - Push: {"operation": "push", "arguments": ["origin", "main"], "description": "Push to main"}
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
	operation, _ := args["operation"].(string)        //nolint:errcheck
	arguments, _ := args["arguments"].([]interface{}) //nolint:errcheck
	workingDir, _ := args["working_dir"].(string)     //nolint:errcheck

	// Validate git operation to prevent command injection
	allowedOperations := []string{
		"clone", "pull", "push", "checkout", "merge", "diff", "log", "stash",
		"status", "add", "commit", "branch", "tag", "fetch", "rebase", "reset",
		"clean", "init", "remote", "show", "mv", "rm",
	}
	operationValid := false
	for _, op := range allowedOperations {
		if operation == op {
			operationValid = true
			break
		}
	}
	if !operationValid {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid git operation: %s", operation),
		}, nil
	}

	if workingDir == "" {
		workingDir = "."
	}
	// Validate working directory path (allow "." for current directory)
	if workingDir != "." && !ValidatePath(workingDir) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid working directory path: %s", workingDir),
		}, nil
	}

	cmdArgs := []string{operation}
	for _, arg := range arguments {
		if s, ok := arg.(string); ok {
			// Validate each argument for shell safety
			if !ValidateCommandArg(s) {
				return ToolResult{
					Success: false,
					Output:  "",
					Error:   fmt.Sprintf("invalid argument contains dangerous characters: %s", s),
				}, nil
			}
			cmdArgs = append(cmdArgs, s)
		}
	}

	// #nosec G204 - git operation and arguments validated, working directory validated
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

// TestHandler handles running Go tests with various configurations.
// Supports unit, integration, E2E, benchmark, and all test types.
// Can generate coverage reports and filter by test name patterns.
//
// Parameters:
//   - test_path (optional): Path or pattern for tests (default: "./...")
//   - test_type (optional): Type of tests - unit, integration, e2e, benchmark, all (default: "all")
//   - coverage (optional): Generate coverage report (default: false)
//   - verbose (optional): Verbose output (default: true)
//   - filter (optional): Test name filter pattern (e.g., "TestFoo")
//   - timeout (optional): Test timeout (e.g., "30s", "5m") (default: "5m")
//   - description (required): Human-readable description
//
// Test Type Mappings:
//   - unit: ./internal/...
//   - integration: ./tests/integration/...
//   - e2e: ./tests/e2e/...
//   - all: ./...
//
// Examples:
//   - Run all tests: {"description": "Run all tests"}
//   - With coverage: {"coverage": true, "description": "Run tests with coverage"}
//   - Specific test: {"filter": "TestUserAuth", "description": "Run auth tests"}
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
	testPath, ok := args["test_path"].(string)
	if !ok {
		testPath = ""
	}
	coverage, ok := args["coverage"].(bool)
	if !ok {
		coverage = false
	}
	verbose, ok := args["verbose"].(bool)
	if !ok {
		verbose = false
	}
	filter, ok := args["filter"].(string)
	if !ok {
		filter = ""
	}
	timeout, ok := args["timeout"].(string)
	if !ok {
		timeout = ""
	}

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

	cmd := exec.CommandContext(ctx, "go", cmdArgs...) // #nosec G204
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

// LintHandler handles code linting and static analysis.
// Supports multiple linters with automatic detection and optional auto-fixing.
//
// Supported linters:
//   - auto: Automatically detect linter based on project type
//   - golangci-lint: Go comprehensive linter
//   - gofmt: Go formatter
//   - eslint: JavaScript/TypeScript linter
//
// Parameters:
//   - path (optional): Path to lint (file or directory) (default: "./...")
//   - linter (optional): Linter to use (default: "auto")
//   - auto_fix (optional): Automatically fix issues where possible (default: false)
//   - config (optional): Path to linter config file
//   - description (required): Human-readable description
//
// Examples:
//   - Lint all: {"description": "Run linting"}
//   - Auto-fix: {"auto_fix": true, "description": "Fix lint issues"}
//   - Specific linter: {"linter": "golangci-lint", "description": "Run golangci-lint"}
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
	path, ok := args["path"].(string)
	if !ok {
		path = ""
	}
	linter, ok := args["linter"].(string)
	if !ok {
		linter = ""
	}
	autoFix, ok := args["auto_fix"].(bool)
	if !ok {
		autoFix = false
	}

	// Validate path if not a pattern
	if path != "" && !strings.Contains(path, "...") {
		if !ValidatePath(path) {
			return ToolResult{
				Success: false,
				Output:  "",
				Error:   fmt.Sprintf("invalid path: %s", path),
			}, nil
		}
	}

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
		cmd = exec.CommandContext(ctx, "golangci-lint", cmdArgs...) // #nosec G204
	case "gofmt":
		if autoFix {
			cmd = exec.CommandContext(ctx, "gofmt", "-w", path) // #nosec G204
		} else {
			cmd = exec.CommandContext(ctx, "gofmt", "-d", path) // #nosec G204
		}
	case "eslint":
		cmdArgs := []string{path}
		if autoFix {
			cmdArgs = append([]string{"--fix"}, cmdArgs...)
		}
		cmd = exec.CommandContext(ctx, "eslint", cmdArgs...) // #nosec G204
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

// DiffHandler shows differences between file versions or working tree using git diff.
//
// Diff modes:
//   - working: Changes in working directory (unstaged)
//   - staged: Changes in staging area (ready to commit)
//   - commit: Changes in a specific commit
//   - branch: Changes between branches
//
// Parameters:
//   - file_path (optional): Specific file to diff (diffs all files if not specified)
//   - mode (optional): Diff mode (default: "working")
//   - compare_with (optional): Revision, branch, or commit to compare with
//   - context_lines (optional): Number of context lines to show (default: 3)
//   - description (required): Human-readable description
//
// Examples:
//   - Working changes: {"mode": "working", "description": "Show unstaged changes"}
//   - Staged changes: {"mode": "staged", "description": "Show staged changes"}
//   - Branch diff: {"mode": "branch", "compare_with": "develop", "description": "Compare with develop"}
//   - File diff: {"file_path": "main.go", "description": "Show changes to main.go"}
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
	filePath, _ := args["file_path"].(string)          //nolint:errcheck
	mode, _ := args["mode"].(string)                   //nolint:errcheck
	compareWith, _ := args["compare_with"].(string)    //nolint:errcheck
	contextLines, _ := args["context_lines"].(float64) //nolint:errcheck

	// Validate inputs
	if filePath != "" && !ValidatePath(filePath) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid file path: %s", filePath),
		}, nil
	}
	if compareWith != "" && !ValidateGitRef(compareWith) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid git reference: %s", compareWith),
		}, nil
	}

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

	// #nosec G204 - inputs validated, git command with safe arguments
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

// TreeViewHandler displays directory structure as a tree.
// Uses the find command to traverse directories and formats output as a tree.
//
// Parameters:
//   - path (optional): Root directory to display (default: ".")
//   - max_depth (optional): Maximum depth to traverse (default: 3)
//   - show_hidden (optional): Show hidden files and directories (default: false)
//   - ignore_patterns (optional): Array of patterns to ignore (e.g., ["node_modules", ".git"])
//   - description (required): Human-readable description
//
// Examples:
//   - Show tree: {"description": "Display directory tree"}
//   - Deep tree: {"max_depth": 5, "description": "Show deep tree"}
//   - With hidden: {"show_hidden": true, "description": "Show all files including hidden"}
//   - Ignore patterns: {"ignore_patterns": ["node_modules", "vendor"], "description": "Show tree without dependencies"}
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
	path := ""
	if p, ok := args["path"].(string); ok {
		path = p
	}
	maxDepth := 0.0
	if md, ok := args["max_depth"].(float64); ok {
		maxDepth = md
	}
	showHidden := false
	if sh, ok := args["show_hidden"].(bool); ok {
		showHidden = sh
	}
	ignorePatterns := []interface{}{}
	if ip, ok := args["ignore_patterns"].([]interface{}); ok {
		ignorePatterns = ip
	}

	// Validate inputs
	if path != "." && !ValidatePath(path) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid path: %s", path),
		}, nil
	}
	for _, pattern := range ignorePatterns {
		if p, ok := pattern.(string); ok {
			if !ValidateCommandArg(p) {
				return ToolResult{
					Success: false,
					Output:  "",
					Error:   fmt.Sprintf("invalid ignore pattern contains dangerous characters: %s", p),
				}, nil
			}
		}
	}

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

	// #nosec G204 - inputs validated, find command with safe arguments
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

// FileInfoHandler gets detailed information about a file.
// Combines stat, wc, and git log to provide comprehensive file metadata.
//
// Information provided:
//   - Basic: Size, permissions, modification time (via stat)
//   - Stats: Line count, word count (via wc) [if include_stats=true]
//   - Git: Last 5 commits affecting the file (via git log) [if include_git=true]
//
// Parameters:
//   - file_path (required): Path to the file
//   - include_stats (optional): Include file statistics (default: true)
//   - include_git (optional): Include git history (default: false)
//   - description (required): Human-readable description
//
// Examples:
//   - Basic info: {"file_path": "main.go", "description": "Get file info"}
//   - With git: {"file_path": "main.go", "include_git": true, "description": "Get file info with history"}
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
	//nolint:errcheck // schema validation ensures correct types
	filePath, _ := args["file_path"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct types
	includeStats, _ := args["include_stats"].(bool) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct types
	includeGit, _ := args["include_git"].(bool) //nolint:errcheck // schema validation ensures correct type

	var result strings.Builder

	// Get basic file info using stat
	// #nosec G204 - filePath is validated by tool schema, binary is hardcoded
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
		// #nosec G204 - filePath is validated by tool schema, binary is hardcoded
		wcCmd := exec.CommandContext(ctx, "wc", "-l", filePath)
		wcOutput, _ := wcCmd.CombinedOutput() //nolint:errcheck
		result.WriteString("\n=== Line Count ===\n")
		result.Write(wcOutput)
	}

	if includeGit {
		// Get git log for file
		// #nosec G204 - filePath is validated by tool schema, binary is hardcoded
		gitCmd := exec.CommandContext(ctx, "git", "log", "--oneline", "-5", "--", filePath)
		gitOutput, _ := gitCmd.CombinedOutput() //nolint:errcheck
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

// SymbolsHandler extracts code symbols (functions, classes, types) from Go files.
// Uses grep to find symbol definitions matching Go syntax patterns.
//
// Symbols extracted:
//   - Functions: func Name(...)
//   - Types: type Name struct/interface
//   - Constants: const Name = ...
//   - Variables: var Name = ...
//
// Parameters:
//   - file_path (optional): File or directory to analyze (default: ".")
//   - recursive (optional): Search subdirectories (default: false)
//   - description (required): Human-readable description
//
// Output format:
//   - filename:line_number:symbol_definition
//
// Examples:
//   - Extract from file: {"file_path": "main.go", "description": "Extract symbols"}
//   - Recursive: {"file_path": "./internal", "recursive": true, "description": "Extract all symbols"}
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
	//nolint:errcheck // schema validation ensures correct types
	filePath, _ := args["file_path"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct types
	recursive, _ := args["recursive"].(bool) //nolint:errcheck // schema validation ensures correct type

	// Validate file path
	if filePath != "." && !ValidatePath(filePath) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid file path: %s", filePath),
		}, nil
	}

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

	// #nosec G204 - file path validated, grep command with safe arguments
	cmd := exec.CommandContext(ctx, "grep", cmdArgs...)
	output, _ := cmd.CombinedOutput() //nolint:errcheck

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// REFERENCES TOOL HANDLER
// ============================================

// ReferencesHandler finds all references to a symbol in the codebase.
// Uses grep to recursively search for symbol usage in .go files.
//
// Parameters:
//   - symbol (required): Symbol name to find references for
//   - file_path (optional): Starting directory for search (default: ".")
//   - include_declaration (optional): Include the declaration in results (default: true)
//   - description (required): Human-readable description
//
// Output format:
//   - filename:line_number:line_content
//
// Examples:
//   - Find refs: {"symbol": "UserAuth", "description": "Find all references to UserAuth"}
//   - In directory: {"symbol": "Config", "file_path": "./internal", "description": "Find Config references in internal"}
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
	//nolint:errcheck // schema validation ensures correct types
	symbol, _ := args["symbol"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct types
	filePath, _ := args["file_path"].(string) //nolint:errcheck // schema validation ensures correct type

	if symbol == "" {
		return ToolResult{
			Success: false,
			Error:   "symbol is required",
		}, nil
	}
	// Validate symbol
	if !ValidateSymbol(symbol) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid symbol: %s", symbol),
		}, nil
	}
	// Validate file path if provided
	if filePath != "" && !ValidatePath(filePath) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid file path: %s", filePath),
		}, nil
	}

	searchPath := "."
	if filePath != "" {
		searchPath = filePath
	}

	// Use grep to find references
	// #nosec G204 - symbol and searchPath are validated, binary is hardcoded
	cmd := exec.CommandContext(ctx, "grep", "-rn", "--include=*.go", symbol, searchPath)
	output, _ := cmd.CombinedOutput() //nolint:errcheck

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// DEFINITION TOOL HANDLER
// ============================================

// DefinitionHandler finds the definition of a symbol.
// Uses grep with patterns to match Go function/type/method definitions.
//
// Definition patterns matched:
//   - func SymbolName(...)
//   - func (receiver) SymbolName(...)
//   - type SymbolName struct/interface/...
//
// Parameters:
//   - symbol (required): Symbol name to find definition for
//   - file_path (optional): Context file for disambiguation (not currently used, searches all)
//   - line (optional): Context line number (not currently used)
//   - description (required): Human-readable description
//
// Output format:
//   - filename:line_number:definition_line
//
// Examples:
//   - Find definition: {"symbol": "UserAuth", "description": "Find UserAuth definition"}
//   - Find method: {"symbol": "Execute", "description": "Find Execute method definition"}
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
	//nolint:errcheck // schema validation ensures correct type
	symbol, _ := args["symbol"].(string) //nolint:errcheck // schema validation ensures correct type

	if symbol == "" {
		return ToolResult{
			Success: false,
			Error:   "symbol is required",
		}, nil
	}
	// Validate symbol
	if !ValidateSymbol(symbol) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid symbol: %s", symbol),
		}, nil
	}

	// Search for function/type definition
	pattern := fmt.Sprintf("^func %s|^func \\([^)]+\\) %s|^type %s ", symbol, symbol, symbol)
	// #nosec G204 - symbol is validated (alphanumeric/underscore), binary is hardcoded
	cmd := exec.CommandContext(ctx, "grep", "-rn", "-E", "--include=*.go", pattern, ".")
	output, _ := cmd.CombinedOutput() //nolint:errcheck

	return ToolResult{
		Success: true,
		Output:  string(output),
	}, nil
}

// ============================================
// PR TOOL HANDLER
// ============================================

// PRHandler manages GitHub/GitLab pull requests using the gh CLI.
//
// Supported actions:
//   - list: List all pull requests
//   - create: Create a new pull request
//   - view: View PR details (requires pr_number)
//   - merge: Merge a pull request (requires pr_number)
//   - close: Close a pull request (requires pr_number)
//
// Parameters:
//   - action (required): PR action to perform
//   - title (optional): PR title (for create)
//   - body (optional): PR description body (for create)
//   - base_branch (optional): Target branch for merge (default: "main")
//   - pr_number (optional): PR number (for view/merge/close)
//   - labels (optional): Array of labels to add to the PR
//   - description (required): Human-readable description
//
// Requirements:
//   - gh CLI must be installed and authenticated
//
// Examples:
//   - List PRs: {"action": "list", "description": "List all PRs"}
//   - Create PR: {"action": "create", "title": "Fix bug", "body": "Description", "description": "Create PR"}
//   - Merge PR: {"action": "merge", "pr_number": 123, "description": "Merge PR #123"}
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
	//nolint:errcheck // schema validation ensures correct type
	action, _ := args["action"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	title, _ := args["title"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	body, _ := args["body"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	baseBranch, _ := args["base_branch"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	prNumber, _ := args["pr_number"].(float64) //nolint:errcheck // schema validation ensures correct type

	// Validate action
	allowedActions := []string{"list", "create", "view", "merge", "close"}
	actionValid := false
	for _, a := range allowedActions {
		if action == a {
			actionValid = true
			break
		}
	}
	if !actionValid {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
	// Validate inputs
	if title != "" && !ValidateCommandArg(title) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid title contains dangerous characters: %s", title),
		}, nil
	}
	if body != "" && !ValidateCommandArg(body) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid body contains dangerous characters: %s", body),
		}, nil
	}
	if baseBranch != "" && !ValidateGitRef(baseBranch) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid base branch: %s", baseBranch),
		}, nil
	}

	if baseBranch == "" {
		baseBranch = "main"
	}

	// #nosec G204 - gh CLI commands with validated arguments, binary is hardcoded
	var cmd *exec.Cmd
	switch action {
	case "list":
		// #nosec G204 - inputs validated, gh command with safe arguments
		cmd = exec.CommandContext(ctx, "gh", "pr", "list")
	case "create":
		cmdArgs := []string{"pr", "create", "--base", baseBranch}
		if title != "" {
			cmdArgs = append(cmdArgs, "--title", title)
		}
		if body != "" {
			cmdArgs = append(cmdArgs, "--body", body)
		}
		// #nosec G204 - inputs validated, gh command with safe arguments
		cmd = exec.CommandContext(ctx, "gh", cmdArgs...)
	case "view":
		if prNumber > 0 {
			// #nosec G204 - inputs validated, gh command with safe arguments
			cmd = exec.CommandContext(ctx, "gh", "pr", "view", fmt.Sprintf("%d", int(prNumber)))
		} else {
			// #nosec G204 - inputs validated, gh command with safe arguments
			cmd = exec.CommandContext(ctx, "gh", "pr", "view")
		}
	case "merge":
		if prNumber > 0 {
			// #nosec G204 - inputs validated, gh command with safe arguments
			cmd = exec.CommandContext(ctx, "gh", "pr", "merge", fmt.Sprintf("%d", int(prNumber)))
		} else {
			return ToolResult{Success: false, Error: "pr_number required for merge"}, nil
		}
	case "close":
		if prNumber > 0 {
			// #nosec G204 - inputs validated, gh command with safe arguments
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

// IssueHandler manages GitHub/GitLab issues using the gh CLI.
//
// Supported actions:
//   - list: List all issues
//   - create: Create a new issue
//   - view: View issue details (requires issue_number)
//   - close: Close an issue (requires issue_number)
//
// Parameters:
//   - action (required): Issue action to perform
//   - title (optional): Issue title (for create)
//   - body (optional): Issue body or comment (for create)
//   - issue_number (optional): Issue number (for view/close)
//   - labels (optional): Array of labels to add
//   - assignees (optional): Array of users to assign
//   - description (required): Human-readable description
//
// Requirements:
//   - gh CLI must be installed and authenticated
//
// Examples:
//   - List issues: {"action": "list", "description": "List all issues"}
//   - Create issue: {"action": "create", "title": "Bug found", "body": "Details", "description": "Create issue"}
//   - Close issue: {"action": "close", "issue_number": 456, "description": "Close issue #456"}
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
	//nolint:errcheck // schema validation ensures correct type
	action, _ := args["action"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	title, _ := args["title"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	body, _ := args["body"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	issueNumber, _ := args["issue_number"].(float64) //nolint:errcheck // schema validation ensures correct type

	// Validate action
	allowedActions := []string{"list", "create", "view", "close"}
	actionValid := false
	for _, a := range allowedActions {
		if action == a {
			actionValid = true
			break
		}
	}
	if !actionValid {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
	// Validate inputs
	if title != "" && !ValidateCommandArg(title) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid title contains dangerous characters: %s", title),
		}, nil
	}
	if body != "" && !ValidateCommandArg(body) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid body contains dangerous characters: %s", body),
		}, nil
	}

	var cmd *exec.Cmd
	switch action {
	case "list":
		// #nosec G204 - inputs validated, gh command with safe arguments
		cmd = exec.CommandContext(ctx, "gh", "issue", "list")
	case "create":
		cmdArgs := []string{"issue", "create"}
		if title != "" {
			cmdArgs = append(cmdArgs, "--title", title)
		}
		if body != "" {
			cmdArgs = append(cmdArgs, "--body", body)
		}
		// #nosec G204 - inputs validated, gh command with safe arguments
		cmd = exec.CommandContext(ctx, "gh", cmdArgs...)
	case "view":
		if issueNumber > 0 {
			// #nosec G204 - inputs validated, gh command with safe arguments
			cmd = exec.CommandContext(ctx, "gh", "issue", "view", fmt.Sprintf("%d", int(issueNumber)))
		} else {
			return ToolResult{Success: false, Error: "issue_number required"}, nil
		}
	case "close":
		if issueNumber > 0 {
			// #nosec G204 - inputs validated, gh command with safe arguments
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

// WorkflowHandler manages CI/CD workflows (GitHub Actions) using the gh CLI.
//
// Supported actions:
//   - list: List all workflows
//   - run: Trigger a workflow run
//   - view: View workflow run details (requires run_id or shows latest)
//   - cancel: Cancel a workflow run (requires run_id)
//   - logs: View workflow run logs (requires run_id)
//
// Parameters:
//   - action (required): Workflow action to perform
//   - workflow_id (optional): Workflow file name or ID (for run)
//   - branch (optional): Branch to run workflow on (for run)
//   - run_id (optional): Run ID (for view/cancel/logs)
//   - description (required): Human-readable description
//
// Requirements:
//   - gh CLI must be installed and authenticated
//
// Examples:
//   - List workflows: {"action": "list", "description": "List all workflows"}
//   - Run workflow: {"action": "run", "workflow_id": "test.yml", "branch": "main", "description": "Run tests"}
//   - View logs: {"action": "logs", "run_id": 12345, "description": "View run logs"}
//   - Cancel run: {"action": "cancel", "run_id": 12345, "description": "Cancel workflow run"}
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
	//nolint:errcheck // schema validation ensures correct type
	action, _ := args["action"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	workflowID, _ := args["workflow_id"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	branch, _ := args["branch"].(string) //nolint:errcheck // schema validation ensures correct type
	//nolint:errcheck // schema validation ensures correct type
	runID, _ := args["run_id"].(float64) //nolint:errcheck // schema validation ensures correct type

	// Validate action
	allowedActions := []string{"list", "run", "view", "cancel", "logs"}
	actionValid := false
	for _, a := range allowedActions {
		if action == a {
			actionValid = true
			break
		}
	}
	if !actionValid {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
	// Validate inputs
	if workflowID != "" && !ValidateCommandArg(workflowID) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid workflow ID contains dangerous characters: %s", workflowID),
		}, nil
	}
	if branch != "" && !ValidateGitRef(branch) {
		return ToolResult{
			Success: false,
			Output:  "",
			Error:   fmt.Sprintf("invalid branch: %s", branch),
		}, nil
	}

	var cmd *exec.Cmd
	switch action {
	case "list":
		// #nosec G204 - inputs validated, gh command with safe arguments
		cmd = exec.CommandContext(ctx, "gh", "workflow", "list")
	case "run":
		cmdArgs := []string{"workflow", "run"}
		if workflowID != "" {
			cmdArgs = append(cmdArgs, workflowID)
		}
		if branch != "" {
			cmdArgs = append(cmdArgs, "--ref", branch)
		}
		// #nosec G204 - inputs validated, gh command with safe arguments
		cmd = exec.CommandContext(ctx, "gh", cmdArgs...)
	case "view":
		if runID > 0 {
			// #nosec G204 - inputs validated, gh command with safe arguments
			cmd = exec.CommandContext(ctx, "gh", "run", "view", fmt.Sprintf("%d", int(runID)))
		} else {
			// #nosec G204 - inputs validated, gh command with safe arguments
			cmd = exec.CommandContext(ctx, "gh", "run", "list")
		}
	case "cancel":
		if runID > 0 {
			// #nosec G204 - inputs validated, gh command with safe arguments
			cmd = exec.CommandContext(ctx, "gh", "run", "cancel", fmt.Sprintf("%d", int(runID)))
		} else {
			return ToolResult{Success: false, Error: "run_id required for cancel"}, nil
		}
	case "logs":
		if runID > 0 {
			// #nosec G204 - inputs validated, gh command with safe arguments
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
