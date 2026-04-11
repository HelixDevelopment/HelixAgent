// Package tools - CLI Agent Extensions
// This file extends the existing ToolSchemaRegistry with tools inspired by CLI agents
// It integrates Aider, Claude Code, Codex, Cline, and other CLI agent capabilities
// into the existing HelixAgent tool system.
package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	// Extend the existing ToolSchemaRegistry with CLI agent tools
	extendToolRegistry()
}

// extendToolRegistry adds new tools inspired by CLI agents
func extendToolRegistry() {
	// ============================================
	// AIDER-INSPIRED TOOLS (Git-Native Workflow)
	// ============================================

	ToolSchemaRegistry["SearchReplace"] = &ToolSchema{
		Name:           "SearchReplace",
		Description:    "Apply precise edits using SEARCH/REPLACE blocks (Aider-style diff editing). More reliable than simple text replacement for multi-line changes.",
		RequiredFields: []string{"file_path", "search_block", "replace_block"},
		OptionalFields: []string{"create_if_missing"},
		Aliases:        []string{"search_replace", "diff_edit", "precise_edit"},
		Category:       CategoryFileSystem,
		Parameters: map[string]Param{
			"file_path":         {Type: "string", Description: "Absolute path to the file to edit", Required: true},
			"search_block":      {Type: "string", Description: "Exact text to search for (including indentation). Must match exactly including whitespace.", Required: true},
			"replace_block":     {Type: "string", Description: "Replacement text (including indentation). Maintain same indentation style.", Required: true},
			"create_if_missing": {Type: "boolean", Description: "Create the file if it doesn't exist (search_block must be empty)", Required: false, Default: false},
		},
		Examples: []ToolExample{
			{Description: "Edit a function", Arguments: `{"file_path": "/app/main.go", "search_block": "func main() {\n    fmt.Println(\"Hello\")\n}", "replace_block": "func main() {\n    fmt.Println(\"Hello World\")\n}"}`},
			{Description: "Create new file", Arguments: `{"file_path": "/app/new.go", "search_block": "", "replace_block": "package main\n\nfunc New() {}", "create_if_missing": true}`},
		},
	}

	ToolSchemaRegistry["RepoMap"] = &ToolSchema{
		Name:           "RepoMap",
		Description:    "Generate a repository map showing code symbols ranked by relevance to a query. Inspired by Aider's repo mapping for better code understanding.",
		RequiredFields: []string{"query", "description"},
		OptionalFields: []string{"map_tokens", "mentioned_files"},
		Aliases:        []string{"repo_map", "codemap", "context_map"},
		Category:       CategoryCodeIntel,
		Parameters: map[string]Param{
			"query":           {Type: "string", Description: "The query or task to find relevant code for", Required: true},
			"map_tokens":      {Type: "integer", Description: "Maximum tokens for the repo map output", Required: false, Default: 1024},
			"mentioned_files": {Type: "array", Description: "Files already mentioned in the conversation for context", Required: false},
			"description":     {Type: "string", Description: "Human-readable description of the query", Required: true},
		},
		Examples: []ToolExample{
			{Description: "Find auth code", Arguments: `{"query": "user authentication flow", "description": "Find authentication related code"}`},
		},
	}

	ToolSchemaRegistry["GitCommit"] = &ToolSchema{
		Name:           "GitCommit",
		Description:    "Commit changes with proper attribution and conventional commit format. Supports Aider-style commit messages.",
		RequiredFields: []string{"message", "description"},
		OptionalFields: []string{"files", "attribution", "conventional"},
		Aliases:        []string{"git_commit", "commit"},
		Category:       CategoryVersionControl,
		Parameters: map[string]Param{
			"message":      {Type: "string", Description: "Commit message (will be formatted if conventional=true)", Required: true},
			"files":        {Type: "array", Description: "Specific files to commit (empty = all staged)", Required: false},
			"attribution":  {Type: "string", Description: "Who made the change: user, assistant, or both", Required: false, Default: "assistant"},
			"conventional": {Type: "boolean", Description: "Use conventional commits format", Required: false, Default: true},
			"description":  {Type: "string", Description: "Human-readable description", Required: true},
		},
		Examples: []ToolExample{
			{Description: "Commit changes", Arguments: `{"message": "Add user auth", "description": "Commit authentication changes"}`},
		},
	}

	// ============================================
	// CLAUDE CODE-INSPIRED TOOLS (Tool Use & UX)
	// ============================================

	ToolSchemaRegistry["CodeBlock"] = &ToolSchema{
		Name:           "CodeBlock",
		Description:    "Display formatted code with syntax highlighting, line numbers, and optional copy button (Claude Code-style terminal output).",
		RequiredFields: []string{"code", "language"},
		OptionalFields: []string{"line_numbers", "copy_button", "title"},
		Aliases:        []string{"code_block", "display_code"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"code":         {Type: "string", Description: "Code content to display", Required: true},
			"language":     {Type: "string", Description: "Programming language for syntax highlighting", Required: true},
			"line_numbers": {Type: "boolean", Description: "Show line numbers", Required: false, Default: true},
			"copy_button":  {Type: "boolean", Description: "Show copy button indicator", Required: false, Default: true},
			"title":        {Type: "string", Description: "Optional title for the code block", Required: false},
		},
	}

	ToolSchemaRegistry["Thinking"] = &ToolSchema{
		Name:           "Thinking",
		Description:    "Display thinking/reasoning process for reasoning models (like Claude 3.7 Sonnet thinking or Codex reasoning).",
		RequiredFields: []string{"thoughts"},
		OptionalFields: []string{"collapsed", "title"},
		Aliases:        []string{"thinking", "reasoning", "thought_process"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"thoughts":  {Type: "string", Description: "Thinking process content to display", Required: true},
			"collapsed": {Type: "boolean", Description: "Show in collapsed state by default", Required: false, Default: false},
			"title":     {Type: "string", Description: "Title for the thinking block", Required: false, Default: "Thinking"},
		},
	}

	ToolSchemaRegistry["ApprovalRequest"] = &ToolSchema{
		Name:           "ApprovalRequest",
		Description:    "Request user approval before executing sensitive operations (Claude Code-style approval system).",
		RequiredFields: []string{"tool_name", "arguments", "risk_level"},
		OptionalFields: []string{"description", "auto_approve_hint"},
		Aliases:        []string{"approval", "request_approval"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"tool_name":         {Type: "string", Description: "Name of the tool requesting approval", Required: true},
			"arguments":         {Type: "object", Description: "Arguments that will be passed to the tool", Required: true},
			"risk_level":        {Type: "string", Description: "Risk level: low, medium, high", Required: true, Enum: []string{"low", "medium", "high"}},
			"description":       {Type: "string", Description: "Human-readable description of what will happen", Required: false},
			"auto_approve_hint": {Type: "string", Description: "Hint for when this operation can be auto-approved in future", Required: false},
		},
	}

	// ============================================
	// CODEX-INSPIRED TOOLS (Code Interpreter)
	// ============================================

	ToolSchemaRegistry["Python"] = &ToolSchema{
		Name:           "Python",
		Description:    "Execute Python code in a sandboxed environment (Codex-style code interpreter). Supports data analysis, visualization, and computation.",
		RequiredFields: []string{"code", "description"},
		OptionalFields: []string{"timeout", "allow_file_access", "packages"},
		Aliases:        []string{"python", "py", "interpreter"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"code":              {Type: "string", Description: "Python code to execute", Required: true},
			"timeout":           {Type: "integer", Description: "Execution timeout in seconds", Required: false, Default: 30},
			"allow_file_access": {Type: "boolean", Description: "Allow reading/writing files in working directory", Required: false, Default: false},
			"packages":          {Type: "array", Description: "Additional Python packages to install", Required: false},
			"description":       {Type: "string", Description: "Human-readable description", Required: true},
		},
		Examples: []ToolExample{
			{Description: "Analyze data", Arguments: `{"code": "import pandas as pd; df = pd.read_csv('data.csv'); print(df.describe())", "description": "Analyze CSV data"}`},
		},
	}

	ToolSchemaRegistry["NotebookCell"] = &ToolSchema{
		Name:           "NotebookCell",
		Description:    "Execute a notebook cell with code and capture outputs including plots and tables (Jupyter-style execution).",
		RequiredFields: []string{"code", "description"},
		OptionalFields: []string{"language", "capture_output", "timeout"},
		Aliases:        []string{"notebook_cell", "cell", "jupyter_cell"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"code":           {Type: "string", Description: "Code to execute", Required: true},
			"language":       {Type: "string", Description: "Language: python, r, julia", Required: false, Default: "python", Enum: []string{"python", "r", "julia"}},
			"capture_output": {Type: "boolean", Description: "Capture all output including plots", Required: false, Default: true},
			"timeout":        {Type: "integer", Description: "Timeout in seconds", Required: false, Default: 60},
			"description":    {Type: "string", Description: "Human-readable description", Required: true},
		},
	}

	// ============================================
	// CLINE-INSPIRED TOOLS (Browser Automation)
	// ============================================

	ToolSchemaRegistry["BrowserNavigate"] = &ToolSchema{
		Name:           "BrowserNavigate",
		Description:    "Navigate browser to a URL (Cline-style browser automation).",
		RequiredFields: []string{"url"},
		OptionalFields: []string{"wait_until", "timeout"},
		Aliases:        []string{"browser_navigate", "navigate"},
		Category:       CategoryWeb,
		Parameters: map[string]Param{
			"url":        {Type: "string", Description: "URL to navigate to", Required: true},
			"wait_until": {Type: "string", Description: "When to consider navigation complete: load, domcontentloaded, networkidle", Required: false, Default: "networkidle", Enum: []string{"load", "domcontentloaded", "networkidle"}},
			"timeout":    {Type: "integer", Description: "Navigation timeout in ms", Required: false, Default: 30000},
		},
	}

	ToolSchemaRegistry["BrowserClick"] = &ToolSchema{
		Name:           "BrowserClick",
		Description:    "Click an element on the page by selector or description.",
		RequiredFields: []string{"selector"},
		OptionalFields: []string{"description", "wait_for_navigation"},
		Aliases:        []string{"browser_click", "click"},
		Category:       CategoryWeb,
		Parameters: map[string]Param{
			"selector":            {Type: "string", Description: "CSS selector or element description", Required: true},
			"description":         {Type: "string", Description: "Human-readable description of what is being clicked", Required: false},
			"wait_for_navigation": {Type: "boolean", Description: "Wait for navigation after click", Required: false, Default: false},
		},
	}

	ToolSchemaRegistry["BrowserType"] = &ToolSchema{
		Name:           "BrowserType",
		Description:    "Type text into an input field.",
		RequiredFields: []string{"selector", "text"},
		OptionalFields: []string{"submit", "clear_first"},
		Aliases:        []string{"browser_type", "type_text"},
		Category:       CategoryWeb,
		Parameters: map[string]Param{
			"selector":    {Type: "string", Description: "CSS selector for the input field", Required: true},
			"text":        {Type: "string", Description: "Text to type", Required: true},
			"submit":      {Type: "boolean", Description: "Press Enter after typing", Required: false, Default: false},
			"clear_first": {Type: "boolean", Description: "Clear field before typing", Required: false, Default: true},
		},
	}

	ToolSchemaRegistry["BrowserScreenshot"] = &ToolSchema{
		Name:           "BrowserScreenshot",
		Description:    "Take a screenshot of the current page.",
		RequiredFields: []string{"description"},
		OptionalFields: []string{"full_page", "selector"},
		Aliases:        []string{"browser_screenshot", "screenshot"},
		Category:       CategoryWeb,
		Parameters: map[string]Param{
			"full_page":   {Type: "boolean", Description: "Capture full page (not just viewport)", Required: false, Default: true},
			"selector":    {Type: "string", Description: "CSS selector to screenshot specific element", Required: false},
			"description": {Type: "string", Description: "Description of what to capture", Required: true},
		},
	}

	ToolSchemaRegistry["BrowserExtract"] = &ToolSchema{
		Name:           "BrowserExtract",
		Description:    "Extract text content from the page or specific elements.",
		RequiredFields: []string{"description"},
		OptionalFields: []string{"selector", "max_length"},
		Aliases:        []string{"browser_extract", "extract_content"},
		Category:       CategoryWeb,
		Parameters: map[string]Param{
			"selector":    {Type: "string", Description: "CSS selector to extract from (empty = whole page)", Required: false},
			"max_length":  {Type: "integer", Description: "Maximum characters to extract", Required: false, Default: 10000},
			"description": {Type: "string", Description: "What content to extract", Required: true},
		},
	}

	// ============================================
	// OPENHANDS-INSPIRED TOOLS (Sandboxing)
	// ============================================

	ToolSchemaRegistry["SandboxExec"] = &ToolSchema{
		Name:           "SandboxExec",
		Description:    "Execute command in isolated sandbox environment (OpenHands-style security).",
		RequiredFields: []string{"command", "description"},
		OptionalFields: []string{"timeout", "working_dir", "env_vars"},
		Aliases:        []string{"sandbox_exec", "secure_exec"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"command":     {Type: "string", Description: "Command to execute", Required: true},
			"timeout":     {Type: "integer", Description: "Timeout in seconds", Required: false, Default: 60},
			"working_dir": {Type: "string", Description: "Working directory in sandbox", Required: false},
			"env_vars":    {Type: "object", Description: "Environment variables to set", Required: false},
			"description": {Type: "string", Description: "Human-readable description", Required: true},
		},
	}

	// ============================================
	// KIRO-INSPIRED TOOLS (Project Memory)
	// ============================================

	ToolSchemaRegistry["Remember"] = &ToolSchema{
		Name:           "Remember",
		Description:    "Store important information in project memory for future context (Kiro-style memory).",
		RequiredFields: []string{"content", "category"},
		OptionalFields: []string{"importance", "tags", "expires_in"},
		Aliases:        []string{"remember", "store_memory"},
		Category:       CategoryCodeIntel,
		Parameters: map[string]Param{
			"content":    {Type: "string", Description: "Content to remember", Required: true},
			"category":   {Type: "string", Description: "Category: decision, insight, error, pattern, requirement", Required: true, Enum: []string{"decision", "insight", "error", "pattern", "requirement"}},
			"importance": {Type: "integer", Description: "Importance score 0-100", Required: false, Default: 50},
			"tags":       {Type: "array", Description: "Tags for organization", Required: false},
			"expires_in": {Type: "string", Description: "Expiration duration (e.g., 7d, 30d, never)", Required: false, Default: "never"},
		},
	}

	ToolSchemaRegistry["Recall"] = &ToolSchema{
		Name:           "Recall",
		Description:    "Search project memory for relevant information.",
		RequiredFields: []string{"query"},
		OptionalFields: []string{"category", "limit", "min_importance"},
		Aliases:        []string{"recall", "search_memory"},
		Category:       CategoryCodeIntel,
		Parameters: map[string]Param{
			"query":          {Type: "string", Description: "Search query", Required: true},
			"category":       {Type: "string", Description: "Filter by category", Required: false},
			"limit":          {Type: "integer", Description: "Maximum results", Required: false, Default: 5},
			"min_importance": {Type: "integer", Description: "Minimum importance score 0-100", Required: false, Default: 30},
		},
	}

	// ============================================
	// PLANDEX-INSPIRED TOOLS (Task Planning)
	// ============================================

	ToolSchemaRegistry["Plan"] = &ToolSchema{
		Name:           "Plan",
		Description:    "Create a structured plan for completing a complex task (Plandex-style planning).",
		RequiredFields: []string{"objective", "description"},
		OptionalFields: []string{"context", "max_steps", "deadline"},
		Aliases:        []string{"plan", "create_plan"},
		Category:       CategoryWorkflow,
		Parameters: map[string]Param{
			"objective":   {Type: "string", Description: "High-level objective to achieve", Required: true},
			"context":     {Type: "array", Description: "Additional context items", Required: false},
			"max_steps":   {Type: "integer", Description: "Maximum number of steps in plan", Required: false, Default: 10},
			"deadline":    {Type: "string", Description: "Target completion time", Required: false},
			"description": {Type: "string", Description: "Human-readable description", Required: true},
		},
	}

	ToolSchemaRegistry["ExecutePlan"] = &ToolSchema{
		Name:           "ExecutePlan",
		Description:    "Execute a previously created plan step by step.",
		RequiredFields: []string{"plan_id", "description"},
		OptionalFields: []string{"start_step", "auto_continue"},
		Aliases:        []string{"execute_plan", "run_plan"},
		Category:       CategoryWorkflow,
		Parameters: map[string]Param{
			"plan_id":       {Type: "string", Description: "ID of the plan to execute", Required: true},
			"start_step":    {Type: "integer", Description: "Step to start from", Required: false, Default: 0},
			"auto_continue": {Type: "boolean", Description: "Auto-continue after each step", Required: false, Default: false},
			"description":   {Type: "string", Description: "Human-readable description", Required: true},
		},
	}
}

// ============================================
// TOOL EXECUTION HELPERS
// ============================================

// SearchReplace executes a SEARCH/REPLACE block edit (Aider-style)
func ExecuteSearchReplace(filePath, searchBlock, replaceBlock string, createIfMissing bool) error {
	// Normalize file path
	filePath = filepath.Clean(filePath)

	// Check if file exists
	exists := false
	if _, err := exec.LookPath("test"); err == nil {
		cmd := exec.Command("test", "-f", filePath)
		if cmd.Run() == nil {
			exists = true
		}
	}

	if !exists {
		if createIfMissing && searchBlock == "" {
			// Create new file
			return os.WriteFile(filePath, []byte(replaceBlock), 0644)
		}
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read current content
	cmd := exec.Command("cat", filePath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := string(output)

	// Find and replace
	if !strings.Contains(content, searchBlock) {
		return fmt.Errorf("search block not found in file (ensure exact match including whitespace)")
	}

	newContent := strings.Replace(content, searchBlock, replaceBlock, 1)

	// Write back
	writeCmd := exec.Command("tee", filePath)
	writeCmd.Stdin = strings.NewReader(newContent)
	return writeCmd.Run()
}

// GetExtendedToolNames returns all tool names including CLI agent extensions
func GetExtendedToolNames() []string {
	return GetAllToolNames()
}

// GetCLITools returns tools inspired by CLI agents
func GetCLITools() []*ToolSchema {
	cliToolNames := []string{
		"SearchReplace", "RepoMap", "GitCommit",
		"CodeBlock", "Thinking", "ApprovalRequest",
		"Python", "NotebookCell",
		"BrowserNavigate", "BrowserClick", "BrowserType", "BrowserScreenshot", "BrowserExtract",
		"SandboxExec",
		"Remember", "Recall",
		"Plan", "ExecutePlan",
	}

	var tools []*ToolSchema
	for _, name := range cliToolNames {
		if schema, ok := ToolSchemaRegistry[name]; ok {
			tools = append(tools, schema)
		}
	}
	return tools
}
