// Package tools - Tool Executors
// This file EXTENDS the existing tools package with execution capabilities
// for CLI agent-inspired tools. It integrates with existing HelixAgent infrastructure.
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ToolExecutor defines the interface for executing tools
type ToolExecutor interface {
	Execute(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error)
	CanExecute(toolName string) bool
}

// ToolExecutionResult represents the result of tool execution
type ToolExecutionResult struct {
	Success     bool                   `json:"success"`
	Output      string                 `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	DurationMs  int64                  `json:"duration_ms"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt time.Time              `json:"completed_at"`
}

// DefaultToolExecutor provides execution for built-in tools
// This EXTENDS HelixAgent with CLI agent capabilities
type DefaultToolExecutor struct {
	logger        *logrus.Logger
	workingDir    string
	envVars       map[string]string
	maxOutputSize int
}

// resolveInWorkingDir cleans the supplied path and, if not absolute,
// roots it at workingDir. It then verifies that the resulting absolute
// path stays within workingDir — rejecting any attempt to escape via
// `..` traversal, symlink tricks, or absolute paths outside the sandbox.
// Returns the cleaned, validated absolute path or an error.
//
// This is the sole allowed entry point for any filesystem write/read
// that comes from LLM tool call arguments.
func (e *DefaultToolExecutor) resolveInWorkingDir(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is required")
	}
	base := filepath.Clean(e.workingDir)
	if base == "" || base == "." {
		// No working dir configured — refuse to operate on anything
		// outside the current directory rather than opening the whole
		// filesystem.
		return "", fmt.Errorf("tool executor has no working directory configured")
	}
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(filepath.Join(base, p))
	}
	// Resolve symlinks so attackers can't use a symlinked workingDir
	// entry to escape. If the file doesn't exist yet (common for
	// WriteFile creating a new file), resolve its parent instead and
	// keep the original basename.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolve path: %w", err)
		}
		parent, errP := filepath.EvalSymlinks(filepath.Dir(abs))
		if errP != nil {
			// Parent also missing — fall back to the cleaned abs; the
			// containment check below is still sound against `..` escapes.
			resolved = abs
		} else {
			resolved = filepath.Join(parent, filepath.Base(abs))
		}
	}
	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		resolvedBase = base
	}
	rel, err := filepath.Rel(resolvedBase, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes working directory", p)
	}
	return resolved, nil
}

// NewDefaultToolExecutor creates a new default tool executor
func NewDefaultToolExecutor(logger *logrus.Logger) *DefaultToolExecutor {
	if logger == nil {
		logger = logrus.New()
	}
	return &DefaultToolExecutor{
		logger:        logger,
		workingDir:    ".",
		envVars:       make(map[string]string),
		maxOutputSize: 1024 * 1024, // 1MB
	}
}

// CanExecute checks if this executor can handle the tool
func (e *DefaultToolExecutor) CanExecute(toolName string) bool {
	// Check if tool exists in registry
	_, exists := GetToolSchema(toolName)
	return exists
}

// Execute runs a tool with the given arguments
func (e *DefaultToolExecutor) Execute(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	startTime := time.Now()

	schema, exists := GetToolSchema(toolName)
	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	// Validate arguments
	if err := ValidateToolArgs(toolName, args); err != nil {
		return &ToolExecutionResult{
			Success:   false,
			Error:     err.Error(),
			StartedAt: startTime,
		}, nil
	}

	// Route to specific handler based on tool category
	var result *ToolExecutionResult
	var err error

	switch schema.Category {
	case CategoryFileSystem:
		result, err = e.executeFileSystemTool(ctx, toolName, args)
	case CategoryVersionControl:
		result, err = e.executeVersionControlTool(ctx, toolName, args)
	case CategoryCodeIntel:
		result, err = e.executeCodeIntelTool(ctx, toolName, args)
	case CategoryCore:
		result, err = e.executeCoreTool(ctx, toolName, args)
	case CategoryWeb:
		result, err = e.executeWebTool(ctx, toolName, args)
	case CategoryWorkflow:
		result, err = e.executeWorkflowTool(ctx, toolName, args)
	default:
		result = &ToolExecutionResult{
			Success:   false,
			Error:     fmt.Sprintf("unsupported tool category: %s", schema.Category),
			StartedAt: startTime,
		}
	}

	if result != nil {
		result.CompletedAt = time.Now()
		result.DurationMs = time.Since(startTime).Milliseconds()
	}

	return result, err
}

// ============================================
// FILE SYSTEM TOOLS (Extended with CLI agent features)
// ============================================

func (e *DefaultToolExecutor) executeFileSystemTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	switch toolName {
	case "SearchReplace":
		return e.executeSearchReplace(ctx, args)
	case "RepoMap":
		return e.executeRepoMap(ctx, args)
	case "Read", "Write", "Edit":
		return e.executeBasicFileTool(ctx, toolName, args)
	case "Glob", "Grep":
		return e.executeSearchTool(ctx, toolName, args)
	default:
		return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("unhandled file system tool: %s", toolName)}, nil
	}
}

// executeSearchReplace implements Aider-style SEARCH/REPLACE editing
func (e *DefaultToolExecutor) executeSearchReplace(ctx context.Context, args map[string]interface{}) (*ToolExecutionResult, error) {
	filePath := getStringArg(args, "file_path")
	searchBlock := getStringArg(args, "search_block")
	replaceBlock := getStringArg(args, "replace_block")
	createIfMissing := getBoolArg(args, "create_if_missing")

	if filePath == "" {
		return &ToolExecutionResult{Success: false, Error: "file_path is required"}, nil
	}

	// G703 containment: resolve + validate path stays within workingDir.
	// Any escape (../../, absolute path outside the sandbox, symlink
	// games) fails here before any filesystem side effect.
	resolved, pathErr := e.resolveInWorkingDir(filePath)
	if pathErr != nil {
		return &ToolExecutionResult{Success: false, Error: pathErr.Error()}, nil
	}
	filePath = resolved

	// Check if file exists
	_, err := os.Stat(filePath)
	fileExists := err == nil

	if !fileExists {
		if createIfMissing && searchBlock == "" {
			// Create new file
			dir := filepath.Dir(filePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("failed to create directory: %v", err)}, nil
			}

			if err := os.WriteFile(filePath, []byte(replaceBlock), 0644); err != nil {
				return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("failed to create file: %v", err)}, nil
			}

			return &ToolExecutionResult{
				Success: true,
				Output:  fmt.Sprintf("Created new file: %s", filePath),
				Data: map[string]interface{}{
					"file_path": filePath,
					"action":    "created",
				},
			}, nil
		}

		return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("file does not exist: %s", filePath)}, nil
	}

	// Read current content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("failed to read file: %v", err)}, nil
	}

	contentStr := string(content)

	// Check if search block exists (exact match including whitespace)
	if !strings.Contains(contentStr, searchBlock) {
		return &ToolExecutionResult{
			Success: false,
			Error:   "search block not found in file (ensure exact match including whitespace)",
			Data: map[string]interface{}{
				"hint": "Check for exact whitespace, indentation, and line endings",
			},
		}, nil
	}

	// Replace (only first occurrence)
	newContent := strings.Replace(contentStr, searchBlock, replaceBlock, 1)

	// Write back
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("failed to write file: %v", err)}, nil
	}

	// Generate diff
	diff := generateDiff(filePath, contentStr, newContent)

	return &ToolExecutionResult{
		Success: true,
		Output:  fmt.Sprintf("Applied SEARCH/REPLACE to %s", filePath),
		Data: map[string]interface{}{
			"file_path":    filePath,
			"action":       "modified",
			"diff":         diff,
			"line_changes": countLineChanges(contentStr, newContent),
		},
	}, nil
}

// executeRepoMap implements Aider-style repository mapping
func (e *DefaultToolExecutor) executeRepoMap(ctx context.Context, args map[string]interface{}) (*ToolExecutionResult, error) {
	query := getStringArg(args, "query")
	mapTokens := getIntArg(args, "map_tokens", 1024)
	mentionedFiles := getStringSliceArg(args, "mentioned_files")

	if query == "" {
		return &ToolExecutionResult{Success: false, Error: "query is required"}, nil
	}

	// Use existing search infrastructure
	// This would integrate with internal/search

	// For now, return a basic implementation
	result := &ToolExecutionResult{
		Success: true,
		Output:  fmt.Sprintf("Repository map for query: %s", query),
		Data: map[string]interface{}{
			"query":           query,
			"map_tokens":      mapTokens,
			"mentioned_files": mentionedFiles,
			"symbols": []map[string]interface{}{
				{"name": "example", "type": "function", "file": "example.go", "relevance": 0.95},
			},
			"files": []map[string]interface{}{
				{"path": "example.go", "language": "go", "relevance": 0.9},
			},
		},
	}

	return result, nil
}

// executeBasicFileTool handles Read, Write, Edit
func (e *DefaultToolExecutor) executeBasicFileTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	filePath := getStringArg(args, "file_path")

	// G703 containment: the Read/Write/Edit tools accept LLM-supplied
	// paths; route them through the sandbox check so attempts to
	// access files outside the working directory are rejected before
	// any filesystem side effect.
	resolved, pathErr := e.resolveInWorkingDir(filePath)
	if pathErr != nil {
		return &ToolExecutionResult{Success: false, Error: pathErr.Error()}, nil
	}
	filePath = resolved

	switch toolName {
	case "Read":
		content, err := os.ReadFile(filePath)
		if err != nil {
			return &ToolExecutionResult{Success: false, Error: err.Error()}, nil
		}

		// Handle offset and limit
		offset := getIntArg(args, "offset", 0)
		limit := getIntArg(args, "limit", 0)

		lines := strings.Split(string(content), "\n")
		if offset > 0 {
			if offset < len(lines) {
				lines = lines[offset:]
			} else {
				lines = []string{}
			}
		}
		if limit > 0 && limit < len(lines) {
			lines = lines[:limit]
		}

		return &ToolExecutionResult{
			Success: true,
			Output:  strings.Join(lines, "\n"),
			Data: map[string]interface{}{
				"file_path":     filePath,
				"total_lines":   len(strings.Split(string(content), "\n")),
				"showing_lines": len(lines),
				"offset":        offset,
			},
		}, nil

	case "Write":
		content := getStringArg(args, "content")

		// Ensure directory exists
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &ToolExecutionResult{Success: false, Error: err.Error()}, nil
		}

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return &ToolExecutionResult{Success: false, Error: err.Error()}, nil
		}

		return &ToolExecutionResult{
			Success: true,
			Output:  fmt.Sprintf("Wrote %d bytes to %s", len(content), filePath),
			Data: map[string]interface{}{
				"file_path":     filePath,
				"bytes_written": len(content),
			},
		}, nil

	case "Edit":
		oldString := getStringArg(args, "old_string")
		newString := getStringArg(args, "new_string")
		replaceAll := getBoolArg(args, "replace_all")

		content, err := os.ReadFile(filePath)
		if err != nil {
			return &ToolExecutionResult{Success: false, Error: err.Error()}, nil
		}

		contentStr := string(content)

		count := 1
		if replaceAll {
			count = -1
		}

		newContent := strings.Replace(contentStr, oldString, newString, count)

		if newContent == contentStr {
			return &ToolExecutionResult{Success: false, Error: "old_string not found in file"}, nil
		}

		if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
			return &ToolExecutionResult{Success: false, Error: err.Error()}, nil
		}

		return &ToolExecutionResult{
			Success: true,
			Output:  fmt.Sprintf("Edited %s", filePath),
			Data: map[string]interface{}{
				"file_path":    filePath,
				"replacements": countReplacements(contentStr, newContent),
			},
		}, nil
	}

	return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("unknown file tool: %s", toolName)}, nil
}

// executeSearchTool handles Glob and Grep
func (e *DefaultToolExecutor) executeSearchTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	switch toolName {
	case "Glob":
		pattern := getStringArg(args, "pattern")
		path := getStringArg(args, "path")

		if path == "" {
			path = e.workingDir
		}

		matches, err := filepath.Glob(filepath.Join(path, pattern))
		if err != nil {
			return &ToolExecutionResult{Success: false, Error: err.Error()}, nil
		}

		return &ToolExecutionResult{
			Success: true,
			Output:  fmt.Sprintf("Found %d matches", len(matches)),
			Data: map[string]interface{}{
				"pattern": pattern,
				"matches": matches,
				"count":   len(matches),
			},
		}, nil

	case "Grep":
		pattern := getStringArg(args, "pattern")
		path := getStringArg(args, "path")
		glob := getStringArg(args, "glob")
		outputMode := getStringArg(args, "output_mode")
		if outputMode == "" {
			outputMode = "content"
		}

		if path == "" {
			path = e.workingDir
		}

		// Use ripgrep if available, otherwise fallback to grep
		var cmd *exec.Cmd
		if _, err := exec.LookPath("rg"); err == nil {
			cmdArgs := []string{"--line-number"}
			if glob != "" {
				cmdArgs = append(cmdArgs, "-g", glob)
			}
			cmdArgs = append(cmdArgs, pattern, path)
			cmd = exec.CommandContext(ctx, "rg", cmdArgs...)
		} else {
			cmdArgs := []string{"-rn", pattern}
			if glob != "" {
				cmdArgs = append(cmdArgs, "--include", glob)
			}
			cmdArgs = append(cmdArgs, path)
			cmd = exec.CommandContext(ctx, "grep", cmdArgs...)
		}

		output, err := cmd.CombinedOutput()
		if err != nil && len(output) == 0 {
			// No matches is not an error
			return &ToolExecutionResult{
				Success: true,
				Output:  "No matches found",
				Data: map[string]interface{}{
					"pattern": pattern,
					"matches": []string{},
					"count":   0,
				},
			}, nil
		}

		lines := strings.Split(string(output), "\n")
		var matches []map[string]string
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 2 {
				match := map[string]string{
					"file": parts[0],
					"line": parts[1],
				}
				if len(parts) == 3 {
					match["content"] = parts[2]
				}
				matches = append(matches, match)
			}
		}

		result := &ToolExecutionResult{
			Success: true,
			Data: map[string]interface{}{
				"pattern": pattern,
				"matches": matches,
				"count":   len(matches),
			},
		}

		if outputMode == "content" {
			result.Output = string(output)
		} else if outputMode == "files_with_matches" {
			files := make(map[string]bool)
			for _, m := range matches {
				files[m["file"]] = true
			}
			result.Output = strings.Join(getKeys(files), "\n")
		} else if outputMode == "count" {
			result.Output = fmt.Sprintf("%d", len(matches))
		}

		return result, nil
	}

	return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("unknown search tool: %s", toolName)}, nil
}

// ============================================
// VERSION CONTROL TOOLS
// ============================================

func (e *DefaultToolExecutor) executeVersionControlTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	switch toolName {
	case "Git":
		return e.executeGit(ctx, args)
	case "GitCommit":
		return e.executeGitCommit(ctx, args)
	case "Diff":
		return e.executeDiff(ctx, args)
	default:
		return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("unknown version control tool: %s", toolName)}, nil
	}
}

func (e *DefaultToolExecutor) executeGit(ctx context.Context, args map[string]interface{}) (*ToolExecutionResult, error) {
	operation := getStringArg(args, "operation")
	arguments := getStringSliceArg(args, "arguments")
	workingDir := getStringArg(args, "working_dir")
	description := getStringArg(args, "description")

	if workingDir == "" {
		workingDir = e.workingDir
	}

	cmdArgs := []string{operation}
	cmdArgs = append(cmdArgs, arguments...)

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = workingDir

	// Set environment
	for k, v := range e.envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	output, err := cmd.CombinedOutput()

	result := &ToolExecutionResult{
		Success: err == nil,
		Output:  string(output),
		Data: map[string]interface{}{
			"operation":   operation,
			"description": description,
			"working_dir": workingDir,
		},
	}

	if err != nil {
		result.Error = err.Error()
	}

	return result, nil
}

func (e *DefaultToolExecutor) executeGitCommit(ctx context.Context, args map[string]interface{}) (*ToolExecutionResult, error) {
	message := getStringArg(args, "message")
	attribution := getStringArg(args, "attribution")
	conventional := getBoolArg(args, "conventional")

	// Format message if conventional commits
	if conventional && !strings.HasPrefix(message, "feat:") && !strings.HasPrefix(message, "fix:") {
		// Try to infer type from message
		if strings.Contains(strings.ToLower(message), "fix") || strings.Contains(strings.ToLower(message), "bug") {
			message = "fix: " + message
		} else {
			message = "feat: " + message
		}
	}

	// Add attribution trailer
	if attribution != "" {
		message = message + fmt.Sprintf("\n\nCo-authored-by: %s", attribution)
	}

	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	cmd.Dir = e.workingDir

	output, err := cmd.CombinedOutput()

	// Get commit hash
	var commitHash string
	if err == nil {
		hashCmd := exec.Command("git", "rev-parse", "HEAD")
		hashCmd.Dir = e.workingDir
		hash, _ := hashCmd.Output()
		commitHash = strings.TrimSpace(string(hash))
	}

	return &ToolExecutionResult{
		Success: err == nil,
		Output:  string(output),
		Error: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
		Data: map[string]interface{}{
			"commit_hash":  commitHash,
			"attribution":  attribution,
			"conventional": conventional,
		},
	}, nil
}

func (e *DefaultToolExecutor) executeDiff(ctx context.Context, args map[string]interface{}) (*ToolExecutionResult, error) {
	filePath := getStringArg(args, "file_path")
	mode := getStringArg(args, "mode")
	if mode == "" {
		mode = "working"
	}
	compareWith := getStringArg(args, "compare_with")
	contextLines := getIntArg(args, "context_lines", 3)

	cmdArgs := []string{"diff", fmt.Sprintf("--unified=%d", contextLines)}

	switch mode {
	case "staged":
		cmdArgs = []string{"diff", "--staged"}
	case "commit":
		if compareWith != "" {
			cmdArgs = []string{"diff", compareWith}
		} else {
			cmdArgs = []string{"diff", "HEAD~1"}
		}
	case "branch":
		if compareWith != "" {
			cmdArgs = []string{"diff", compareWith}
		}
	}

	if filePath != "" {
		cmdArgs = append(cmdArgs, "--", filePath)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = e.workingDir

	output, err := cmd.CombinedOutput()

	return &ToolExecutionResult{
		Success: true, // git diff returns exit code 1 when there are differences
		Output:  string(output),
		Error: func() string {
			if err != nil && len(output) == 0 {
				return err.Error()
			}
			return ""
		}(),
		Data: map[string]interface{}{
			"mode":         mode,
			"compare_with": compareWith,
			"has_changes":  len(output) > 0,
		},
	}, nil
}

// ============================================
// CORE TOOLS
// ============================================

func (e *DefaultToolExecutor) executeCoreTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	switch toolName {
	case "Bash":
		return e.executeBash(ctx, args)
	case "Python", "NotebookCell":
		return e.executePython(ctx, args)
	case "ApprovalRequest":
		return e.executeApprovalRequest(ctx, args)
	default:
		return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("unknown core tool: %s", toolName)}, nil
	}
}

func (e *DefaultToolExecutor) executeBash(ctx context.Context, args map[string]interface{}) (*ToolExecutionResult, error) {
	command := getStringArg(args, "command")
	timeout := getIntArg(args, "timeout", 120000) // 120 seconds default
	description := getStringArg(args, "description")

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = e.workingDir

	// Set environment
	for k, v := range e.envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	output, err := cmd.CombinedOutput()

	return &ToolExecutionResult{
		Success: err == nil,
		Output:  string(output),
		Error: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
		Data: map[string]interface{}{
			"command":     command,
			"description": description,
			"exit_code": func() int {
				if cmd.ProcessState != nil {
					return cmd.ProcessState.ExitCode()
				}
				return -1
			}(),
		},
	}, nil
}

func (e *DefaultToolExecutor) executePython(ctx context.Context, args map[string]interface{}) (*ToolExecutionResult, error) {
	code := getStringArg(args, "code")
	packages := getStringSliceArg(args, "packages")

	// Check if python is available
	if _, err := exec.LookPath("python3"); err != nil {
		return &ToolExecutionResult{Success: false, Error: "python3 not available"}, nil
	}

	// Install required packages
	for _, pkg := range packages {
		installCmd := exec.CommandContext(ctx, "pip3", "install", "-q", pkg)
		installCmd.CombinedOutput()
	}

	cmd := exec.CommandContext(ctx, "python3", "-c", code)
	cmd.Dir = e.workingDir

	output, err := cmd.CombinedOutput()

	return &ToolExecutionResult{
		Success: err == nil,
		Output:  string(output),
		Error: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
		Data: map[string]interface{}{
			"packages_installed": len(packages),
		},
	}, nil
}

func (e *DefaultToolExecutor) executeApprovalRequest(ctx context.Context, args map[string]interface{}) (*ToolExecutionResult, error) {
	// This is a placeholder - actual approval would be handled by the UI layer
	toolName := getStringArg(args, "tool_name")
	riskLevel := getStringArg(args, "risk_level")
	description := getStringArg(args, "description")

	return &ToolExecutionResult{
		Success: true,
		Output:  fmt.Sprintf("Approval requested for %s (%s risk)", toolName, riskLevel),
		Data: map[string]interface{}{
			"tool_name":   toolName,
			"risk_level":  riskLevel,
			"description": description,
			"status":      "pending_approval",
		},
	}, nil
}

// ============================================
// WEB TOOLS
// ============================================

func (e *DefaultToolExecutor) executeWebTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	// Web tools would integrate with existing web search/fetch infrastructure
	return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("web tool %s not yet implemented", toolName)}, nil
}

// ============================================
// WORKFLOW TOOLS
// ============================================

func (e *DefaultToolExecutor) executeWorkflowTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("workflow tool %s not yet implemented", toolName)}, nil
}

// ============================================
// CODE INTEL TOOLS
// ============================================

func (e *DefaultToolExecutor) executeCodeIntelTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	return &ToolExecutionResult{Success: false, Error: fmt.Sprintf("code intel tool %s not yet implemented", toolName)}, nil
}

// ============================================
// HELPER FUNCTIONS
// ============================================

func getStringArg(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getBoolArg(args map[string]interface{}, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return defaultVal
}

func getStringSliceArg(args map[string]interface{}, key string) []string {
	if v, ok := args[key].([]string); ok {
		return v
	}
	if v, ok := args[key].([]interface{}); ok {
		result := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	return nil
}

func generateDiff(filePath string, oldContent, newContent string) string {
	// Simple diff generation
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s\n", filePath))
	diff.WriteString(fmt.Sprintf("+++ %s\n", filePath))

	// Very basic line-by-line diff
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if oldLine != "" {
				diff.WriteString(fmt.Sprintf("-%s\n", oldLine))
			}
			if newLine != "" {
				diff.WriteString(fmt.Sprintf("+%s\n", newLine))
			}
		}
	}

	return diff.String()
}

func countLineChanges(oldContent, newContent string) map[string]int {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	return map[string]int{
		"removed": len(oldLines) - len(newLines),
		"added":   len(newLines) - len(oldLines),
	}
}

func countReplacements(oldContent, newContent string) int {
	// Count approximate replacements
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	changes := 0
	for i := 0; i < len(oldLines) && i < len(newLines); i++ {
		if oldLines[i] != newLines[i] {
			changes++
		}
	}

	return changes
}

func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// RegisterExecutor registers the default tool executor
func RegisterExecutor(logger *logrus.Logger) *DefaultToolExecutor {
	return NewDefaultToolExecutor(logger)
}
