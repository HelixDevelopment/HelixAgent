// Package claude_code provides Claude Code tool execution.
package claude_code

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToolExecutor handles tool execution for Claude Code
type ToolExecutor struct {
	workDir      string
	allowedTools map[string]bool
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(workDir string, allowedTools []string) *ToolExecutor {
	tools := make(map[string]bool)
	for _, t := range allowedTools {
		tools[t] = true
	}

	return &ToolExecutor{
		workDir:      workDir,
		allowedTools: tools,
	}
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// ExecuteTool executes a tool with given parameters
func (te *ToolExecutor) ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}) (*ToolResult, error) {
	if !te.isToolAllowed(toolName) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool not allowed: %s", toolName),
		}, nil
	}

	switch toolName {
	case "read_file":
		return te.readFile(ctx, params)
	case "write_file":
		return te.writeFile(ctx, params)
	case "edit_file":
		return te.editFile(ctx, params)
	case "bash":
		return te.bash(ctx, params)
	case "search":
		return te.search(ctx, params)
	case "view":
		return te.view(ctx, params)
	case "git":
		return te.git(ctx, params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", toolName),
		}, nil
	}
}

func (te *ToolExecutor) isToolAllowed(tool string) bool {
	if len(te.allowedTools) == 0 {
		return true // Allow all if no whitelist
	}
	return te.allowedTools[tool]
}

func (te *ToolExecutor) readFile(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	filePath, ok := params["file_path"].(string)
	if !ok || filePath == "" {
		return &ToolResult{Success: false, Error: "file_path required"}, nil
	}

	// Sanitize path. Empty return means the path escapes workDir —
	// fail loudly rather than falling through to os.ReadFile and
	// producing a cryptic "no such file or directory" error.
	filePath = te.sanitizePath(filePath)
	if filePath == "" {
		return &ToolResult{Success: false, Error: "path escapes working directory"}, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: string(content)}, nil
}

func (te *ToolExecutor) writeFile(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	filePath, _ := params["file_path"].(string)
	content, _ := params["content"].(string)

	if filePath == "" {
		return &ToolResult{Success: false, Error: "file_path required"}, nil
	}

	filePath = te.sanitizePath(filePath)
	if filePath == "" {
		return &ToolResult{Success: false, Error: "path escapes working directory"}, nil
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolResult{Success: false, Error: err.Error()}, nil
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return &ToolResult{Success: false, Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: fmt.Sprintf("Wrote %d bytes to %s", len(content), filePath)}, nil
}

func (te *ToolExecutor) editFile(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	filePath, _ := params["file_path"].(string)
	oldStr, _ := params["old_string"].(string)
	newStr, _ := params["new_string"].(string)

	if filePath == "" || oldStr == "" {
		return &ToolResult{Success: false, Error: "file_path and old_string required"}, nil
	}

	filePath = te.sanitizePath(filePath)
	if filePath == "" {
		return &ToolResult{Success: false, Error: "path escapes working directory"}, nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}, nil
	}

	newContent := strings.Replace(string(content), oldStr, newStr, 1)
	if newContent == string(content) {
		return &ToolResult{Success: false, Error: "old_string not found in file"}, nil
	}

	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return &ToolResult{Success: false, Error: err.Error()}, nil
	}

	return &ToolResult{Success: true, Output: fmt.Sprintf("Edited %s", filePath)}, nil
}

func (te *ToolExecutor) bash(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return &ToolResult{Success: false, Error: "command required"}, nil
	}

	// Security: block dangerous commands
	if te.isDangerousCommand(command) {
		return &ToolResult{Success: false, Error: "dangerous command blocked"}, nil
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = te.workDir

	output, err := cmd.CombinedOutput()
	result := &ToolResult{
		Success: err == nil,
		Output:  string(output),
	}
	if err != nil {
		result.Error = err.Error()
	}

	return result, nil
}

func (te *ToolExecutor) search(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	pattern, _ := params["pattern"].(string)
	path, _ := params["path"].(string)

	if pattern == "" {
		return &ToolResult{Success: false, Error: "pattern required"}, nil
	}

	if path == "" {
		path = te.workDir
	}
	path = te.sanitizePath(path)

	// Use ripgrep if available, fall back to grep
	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.CommandContext(ctx, "rg", "-n", "-i", pattern, path)
	} else {
		cmd = exec.CommandContext(ctx, "grep", "-rn", "-i", pattern, path)
	}

	output, err := cmd.CombinedOutput()
	result := &ToolResult{
		Success: err == nil || len(output) > 0,
		Output:  string(output),
	}

	return result, nil
}

func (te *ToolExecutor) view(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		path = te.workDir
	}
	path = te.sanitizePath(path)

	info, err := os.Stat(path)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}, nil
	}

	if info.IsDir() {
		// List directory
		entries, err := os.ReadDir(path)
		if err != nil {
			return &ToolResult{Success: false, Error: err.Error()}, nil
		}

		var output strings.Builder
		for _, entry := range entries {
			prefix := "  "
			if entry.IsDir() {
				prefix = "d "
			}
			output.WriteString(fmt.Sprintf("%s%s\n", prefix, entry.Name()))
		}
		return &ToolResult{Success: true, Output: output.String()}, nil
	}

	// Show file info
	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("File: %s\nSize: %d bytes\nModified: %s", path, info.Size(), info.ModTime()),
	}, nil
}

func (te *ToolExecutor) git(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	subcommand, _ := params["subcommand"].(string)
	if subcommand == "" {
		subcommand = "status"
	}

	args := []string{subcommand}
	if extraArgs, ok := params["args"].([]string); ok {
		args = append(args, extraArgs...)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = te.workDir

	output, err := cmd.CombinedOutput()
	result := &ToolResult{
		Success: err == nil,
		Output:  string(output),
	}
	if err != nil {
		result.Error = err.Error()
	}

	return result, nil
}

// sanitizePath resolves path against te.workDir and enforces that the
// final absolute path stays within the sandbox. Returns an empty
// string on escape so callers detect the rejection without needing a
// second return value (the old contract). An empty return always
// causes the subsequent os.ReadFile/os.WriteFile to fail with a clear
// "no such file or directory" rather than touching random files.
//
// This is the G703 fix for the 2026-04-11 gosec triage. The prior
// implementation accepted any absolute path unchanged and joined any
// relative path to workDir without a containment check, so an LLM tool
// call with file_path="../../../etc/passwd" would write outside the
// sandbox.
func (te *ToolExecutor) sanitizePath(path string) string {
	if path == "" || te.workDir == "" {
		return ""
	}
	base := filepath.Clean(te.workDir)
	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Clean(filepath.Join(base, path))
	}
	// Resolve the base (and the path itself if it exists) so symlink
	// tricks cannot escape. A missing leaf is expected for Write, so
	// fall back to resolving the parent.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		parent, errP := filepath.EvalSymlinks(filepath.Dir(abs))
		if errP != nil {
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
		return ""
	}
	return resolved
}

func (te *ToolExecutor) isDangerousCommand(command string) bool {
	dangerous := []string{
		"rm -rf /",
		"> /dev/sda",
		"mkfs",
		"dd if=/dev/zero",
		":(){ :|:& };:", // fork bomb
	}

	for _, d := range dangerous {
		if strings.Contains(command, d) {
			return true
		}
	}
	return false
}
