// Package tools provides a centralized tool schema registry and handlers
// for HelixAgent's AI Debate Team tool calling system.
package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolSchema defines the schema for a tool including required and optional fields
type ToolSchema struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	RequiredFields []string          `json:"required_fields"`
	OptionalFields []string          `json:"optional_fields,omitempty"`
	Aliases        []string          `json:"aliases,omitempty"`
	Category       string            `json:"category"`
	Examples       []ToolExample     `json:"examples,omitempty"`
	Parameters     map[string]Param  `json:"parameters"`
}

// Param defines a parameter for a tool
type Param struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// ToolExample provides an example usage of a tool
type ToolExample struct {
	Description string `json:"description"`
	Arguments   string `json:"arguments"`
}

// ToolCategory constants
const (
	CategoryCore           = "core"
	CategoryFileSystem     = "filesystem"
	CategoryVersionControl = "version_control"
	CategoryCodeIntel      = "code_intelligence"
	CategoryWorkflow       = "workflow"
	CategoryWeb            = "web"
)

// ToolSchemaRegistry is the centralized registry of all supported tools
var ToolSchemaRegistry = map[string]*ToolSchema{
	// ============================================
	// CORE TOOLS (Existing)
	// ============================================
	"Bash": {
		Name:           "Bash",
		Description:    "Execute shell commands in a bash environment",
		RequiredFields: []string{"command", "description"},
		Aliases:        []string{"bash", "shell", "Shell"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"command":     {Type: "string", Description: "The shell command to execute", Required: true},
			"description": {Type: "string", Description: "Human-readable description of what the command does", Required: true},
			"timeout":     {Type: "integer", Description: "Timeout in milliseconds (max 600000)", Required: false, Default: 120000},
		},
		Examples: []ToolExample{
			{Description: "Run tests", Arguments: `{"command": "go test -v ./...", "description": "Run all Go tests"}`},
			{Description: "Build project", Arguments: `{"command": "make build", "description": "Build the project"}`},
		},
	},
	"Read": {
		Name:           "Read",
		Description:    "Read contents of a file from the filesystem",
		RequiredFields: []string{"file_path"},
		OptionalFields: []string{"offset", "limit"},
		Aliases:        []string{"read"},
		Category:       CategoryFileSystem,
		Parameters: map[string]Param{
			"file_path": {Type: "string", Description: "Absolute path to the file to read", Required: true},
			"offset":    {Type: "integer", Description: "Line number to start reading from", Required: false},
			"limit":     {Type: "integer", Description: "Number of lines to read", Required: false},
		},
	},
	"Write": {
		Name:           "Write",
		Description:    "Write content to a file, creating or overwriting it",
		RequiredFields: []string{"file_path", "content"},
		Aliases:        []string{"write"},
		Category:       CategoryFileSystem,
		Parameters: map[string]Param{
			"file_path": {Type: "string", Description: "Absolute path to the file to write", Required: true},
			"content":   {Type: "string", Description: "Content to write to the file", Required: true},
		},
	},
	"Edit": {
		Name:           "Edit",
		Description:    "Edit a file by replacing text",
		RequiredFields: []string{"file_path", "old_string", "new_string"},
		OptionalFields: []string{"replace_all"},
		Aliases:        []string{"edit"},
		Category:       CategoryFileSystem,
		Parameters: map[string]Param{
			"file_path":   {Type: "string", Description: "Absolute path to the file to edit", Required: true},
			"old_string":  {Type: "string", Description: "Text to find and replace", Required: true},
			"new_string":  {Type: "string", Description: "Replacement text", Required: true},
			"replace_all": {Type: "boolean", Description: "Replace all occurrences", Required: false, Default: false},
		},
	},
	"Glob": {
		Name:           "Glob",
		Description:    "Find files matching a glob pattern",
		RequiredFields: []string{"pattern"},
		OptionalFields: []string{"path"},
		Aliases:        []string{"glob"},
		Category:       CategoryFileSystem,
		Parameters: map[string]Param{
			"pattern": {Type: "string", Description: "Glob pattern to match files (e.g., **/*.go)", Required: true},
			"path":    {Type: "string", Description: "Directory to search in", Required: false},
		},
	},
	"Grep": {
		Name:           "Grep",
		Description:    "Search for content in files using regex patterns",
		RequiredFields: []string{"pattern"},
		OptionalFields: []string{"path", "glob", "output_mode"},
		Aliases:        []string{"grep"},
		Category:       CategoryFileSystem,
		Parameters: map[string]Param{
			"pattern":     {Type: "string", Description: "Regex pattern to search for", Required: true},
			"path":        {Type: "string", Description: "Directory to search in", Required: false},
			"glob":        {Type: "string", Description: "Glob pattern to filter files", Required: false},
			"output_mode": {Type: "string", Description: "Output mode: content, files_with_matches, count", Required: false, Enum: []string{"content", "files_with_matches", "count"}},
		},
	},
	"WebFetch": {
		Name:           "WebFetch",
		Description:    "Fetch content from a URL and process it",
		RequiredFields: []string{"url", "prompt"},
		Aliases:        []string{"webfetch"},
		Category:       CategoryWeb,
		Parameters: map[string]Param{
			"url":    {Type: "string", Description: "URL to fetch content from", Required: true},
			"prompt": {Type: "string", Description: "Prompt describing what information to extract", Required: true},
		},
	},
	"WebSearch": {
		Name:           "WebSearch",
		Description:    "Search the web for information",
		RequiredFields: []string{"query"},
		OptionalFields: []string{"allowed_domains", "blocked_domains"},
		Aliases:        []string{"websearch"},
		Category:       CategoryWeb,
		Parameters: map[string]Param{
			"query":           {Type: "string", Description: "Search query", Required: true},
			"allowed_domains": {Type: "array", Description: "Only include results from these domains", Required: false},
			"blocked_domains": {Type: "array", Description: "Exclude results from these domains", Required: false},
		},
	},
	"Task": {
		Name:           "Task",
		Description:    "Delegate a task to a specialized subagent",
		RequiredFields: []string{"prompt", "description", "subagent_type"},
		OptionalFields: []string{"model", "max_turns"},
		Aliases:        []string{"task"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"prompt":        {Type: "string", Description: "The task for the agent to perform", Required: true},
			"description":   {Type: "string", Description: "Short description of the task (3-5 words)", Required: true},
			"subagent_type": {Type: "string", Description: "Type of agent: Bash, general-purpose, Explore, Plan", Required: true},
			"model":         {Type: "string", Description: "Model to use: sonnet, opus, haiku", Required: false},
			"max_turns":     {Type: "integer", Description: "Maximum number of turns", Required: false},
		},
	},

	// ============================================
	// VERSION CONTROL TOOLS (New)
	// ============================================
	"Git": {
		Name:           "Git",
		Description:    "Execute Git version control operations",
		RequiredFields: []string{"operation", "description"},
		OptionalFields: []string{"arguments", "working_dir"},
		Aliases:        []string{"git"},
		Category:       CategoryVersionControl,
		Parameters: map[string]Param{
			"operation":   {Type: "string", Description: "Git operation: status, add, commit, push, pull, branch, checkout, merge, diff, log, clone, fetch, reset, stash", Required: true, Enum: []string{"status", "add", "commit", "push", "pull", "branch", "checkout", "merge", "diff", "log", "clone", "fetch", "reset", "stash"}},
			"arguments":   {Type: "array", Description: "Additional arguments for the operation", Required: false},
			"working_dir": {Type: "string", Description: "Working directory for the git command", Required: false},
			"description": {Type: "string", Description: "Human-readable description of the operation", Required: true},
		},
		Examples: []ToolExample{
			{Description: "Check status", Arguments: `{"operation": "status", "description": "Check git status"}`},
			{Description: "Commit changes", Arguments: `{"operation": "commit", "arguments": ["-m", "Fix bug"], "description": "Commit bug fix"}`},
			{Description: "Push to remote", Arguments: `{"operation": "push", "arguments": ["origin", "main"], "description": "Push to main branch"}`},
		},
	},
	"Diff": {
		Name:           "Diff",
		Description:    "Show differences between file versions or working tree",
		RequiredFields: []string{"description"},
		OptionalFields: []string{"file_path", "mode", "compare_with", "context_lines"},
		Aliases:        []string{"diff"},
		Category:       CategoryVersionControl,
		Parameters: map[string]Param{
			"file_path":     {Type: "string", Description: "File to diff (optional, diffs all if not specified)", Required: false},
			"mode":          {Type: "string", Description: "Diff mode: working, staged, commit, branch", Required: false, Default: "working", Enum: []string{"working", "staged", "commit", "branch"}},
			"compare_with":  {Type: "string", Description: "Revision, branch, or commit to compare with", Required: false},
			"context_lines": {Type: "integer", Description: "Number of context lines to show", Required: false, Default: 3},
			"description":   {Type: "string", Description: "Human-readable description", Required: true},
		},
	},

	// ============================================
	// TESTING TOOLS (New)
	// ============================================
	"Test": {
		Name:           "Test",
		Description:    "Run tests with coverage and reporting",
		RequiredFields: []string{"description"},
		OptionalFields: []string{"test_path", "test_type", "coverage", "verbose", "filter", "timeout"},
		Aliases:        []string{"test"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"test_path":   {Type: "string", Description: "Path or pattern for tests (e.g., ./..., ./tests/unit/...)", Required: false, Default: "./..."},
			"test_type":   {Type: "string", Description: "Type of tests: unit, integration, e2e, benchmark, all", Required: false, Default: "all", Enum: []string{"unit", "integration", "e2e", "benchmark", "all"}},
			"coverage":    {Type: "boolean", Description: "Generate coverage report", Required: false, Default: false},
			"verbose":     {Type: "boolean", Description: "Verbose output", Required: false, Default: true},
			"filter":      {Type: "string", Description: "Test name filter pattern (e.g., TestFoo)", Required: false},
			"timeout":     {Type: "string", Description: "Test timeout (e.g., 30s, 5m)", Required: false, Default: "5m"},
			"description": {Type: "string", Description: "Human-readable description", Required: true},
		},
		Examples: []ToolExample{
			{Description: "Run all tests", Arguments: `{"test_path": "./...", "description": "Run all tests"}`},
			{Description: "Run with coverage", Arguments: `{"test_path": "./...", "coverage": true, "description": "Run tests with coverage"}`},
			{Description: "Run specific test", Arguments: `{"filter": "TestUserAuth", "description": "Run user authentication tests"}`},
		},
	},
	"Lint": {
		Name:           "Lint",
		Description:    "Run code linting and static analysis",
		RequiredFields: []string{"description"},
		OptionalFields: []string{"path", "linter", "auto_fix", "config"},
		Aliases:        []string{"lint"},
		Category:       CategoryCore,
		Parameters: map[string]Param{
			"path":        {Type: "string", Description: "Path to lint (file or directory)", Required: false, Default: "./..."},
			"linter":      {Type: "string", Description: "Linter to use: auto, golangci-lint, eslint, pylint, rustfmt", Required: false, Default: "auto", Enum: []string{"auto", "golangci-lint", "eslint", "pylint", "rustfmt", "gofmt"}},
			"auto_fix":    {Type: "boolean", Description: "Automatically fix issues where possible", Required: false, Default: false},
			"config":      {Type: "string", Description: "Path to linter config file", Required: false},
			"description": {Type: "string", Description: "Human-readable description", Required: true},
		},
	},

	// ============================================
	// FILE INTELLIGENCE TOOLS (New)
	// ============================================
	"TreeView": {
		Name:           "TreeView",
		Description:    "Display directory structure as a tree",
		RequiredFields: []string{"description"},
		OptionalFields: []string{"path", "max_depth", "show_hidden", "ignore_patterns"},
		Aliases:        []string{"treeview", "tree"},
		Category:       CategoryFileSystem,
		Parameters: map[string]Param{
			"path":            {Type: "string", Description: "Root directory to display", Required: false, Default: "."},
			"max_depth":       {Type: "integer", Description: "Maximum depth to traverse", Required: false, Default: 3},
			"show_hidden":     {Type: "boolean", Description: "Show hidden files and directories", Required: false, Default: false},
			"ignore_patterns": {Type: "array", Description: "Patterns to ignore (e.g., node_modules, .git)", Required: false},
			"description":     {Type: "string", Description: "Human-readable description", Required: true},
		},
	},
	"FileInfo": {
		Name:           "FileInfo",
		Description:    "Get detailed information about a file",
		RequiredFields: []string{"file_path", "description"},
		OptionalFields: []string{"include_stats", "include_git"},
		Aliases:        []string{"fileinfo"},
		Category:       CategoryFileSystem,
		Parameters: map[string]Param{
			"file_path":     {Type: "string", Description: "Path to the file", Required: true},
			"include_stats": {Type: "boolean", Description: "Include file statistics (size, lines, etc.)", Required: false, Default: true},
			"include_git":   {Type: "boolean", Description: "Include git history information", Required: false, Default: false},
			"description":   {Type: "string", Description: "Human-readable description", Required: true},
		},
	},

	// ============================================
	// CODE INTELLIGENCE TOOLS (New)
	// ============================================
	"Symbols": {
		Name:           "Symbols",
		Description:    "Extract code symbols (functions, classes, types) from files",
		RequiredFields: []string{"description"},
		OptionalFields: []string{"file_path", "symbol_types", "recursive"},
		Aliases:        []string{"symbols"},
		Category:       CategoryCodeIntel,
		Parameters: map[string]Param{
			"file_path":    {Type: "string", Description: "File or directory to analyze", Required: false, Default: "."},
			"symbol_types": {Type: "array", Description: "Types to extract: function, class, type, const, var, interface", Required: false},
			"recursive":    {Type: "boolean", Description: "Search subdirectories", Required: false, Default: false},
			"description":  {Type: "string", Description: "Human-readable description", Required: true},
		},
	},
	"References": {
		Name:           "References",
		Description:    "Find all references to a symbol in the codebase",
		RequiredFields: []string{"symbol", "description"},
		OptionalFields: []string{"file_path", "include_declaration"},
		Aliases:        []string{"references", "refs"},
		Category:       CategoryCodeIntel,
		Parameters: map[string]Param{
			"symbol":              {Type: "string", Description: "Symbol name to find references for", Required: true},
			"file_path":           {Type: "string", Description: "Starting file for context", Required: false},
			"include_declaration": {Type: "boolean", Description: "Include the declaration in results", Required: false, Default: true},
			"description":         {Type: "string", Description: "Human-readable description", Required: true},
		},
	},
	"Definition": {
		Name:           "Definition",
		Description:    "Find the definition of a symbol",
		RequiredFields: []string{"symbol", "description"},
		OptionalFields: []string{"file_path", "line"},
		Aliases:        []string{"definition", "goto"},
		Category:       CategoryCodeIntel,
		Parameters: map[string]Param{
			"symbol":      {Type: "string", Description: "Symbol name to find definition for", Required: true},
			"file_path":   {Type: "string", Description: "Context file for disambiguation", Required: false},
			"line":        {Type: "integer", Description: "Context line number", Required: false},
			"description": {Type: "string", Description: "Human-readable description", Required: true},
		},
	},

	// ============================================
	// WORKFLOW TOOLS (New)
	// ============================================
	"PR": {
		Name:           "PR",
		Description:    "Manage pull requests (GitHub/GitLab)",
		RequiredFields: []string{"action", "description"},
		OptionalFields: []string{"title", "body", "base_branch", "pr_number", "labels"},
		Aliases:        []string{"pr", "pullrequest"},
		Category:       CategoryWorkflow,
		Parameters: map[string]Param{
			"action":      {Type: "string", Description: "PR action: create, list, view, approve, merge, close", Required: true, Enum: []string{"create", "list", "view", "approve", "merge", "close"}},
			"title":       {Type: "string", Description: "PR title (for create)", Required: false},
			"body":        {Type: "string", Description: "PR description body", Required: false},
			"base_branch": {Type: "string", Description: "Target branch for merge", Required: false, Default: "main"},
			"pr_number":   {Type: "integer", Description: "PR number (for view/approve/merge/close)", Required: false},
			"labels":      {Type: "array", Description: "Labels to add to the PR", Required: false},
			"description": {Type: "string", Description: "Human-readable description", Required: true},
		},
	},
	"Issue": {
		Name:           "Issue",
		Description:    "Manage issues (GitHub/GitLab)",
		RequiredFields: []string{"action", "description"},
		OptionalFields: []string{"title", "body", "issue_number", "labels", "assignees"},
		Aliases:        []string{"issue"},
		Category:       CategoryWorkflow,
		Parameters: map[string]Param{
			"action":       {Type: "string", Description: "Issue action: create, list, view, close, comment", Required: true, Enum: []string{"create", "list", "view", "close", "comment"}},
			"title":        {Type: "string", Description: "Issue title (for create)", Required: false},
			"body":         {Type: "string", Description: "Issue body or comment", Required: false},
			"issue_number": {Type: "integer", Description: "Issue number (for view/close/comment)", Required: false},
			"labels":       {Type: "array", Description: "Labels to add", Required: false},
			"assignees":    {Type: "array", Description: "Users to assign", Required: false},
			"description":  {Type: "string", Description: "Human-readable description", Required: true},
		},
	},
	"Workflow": {
		Name:           "Workflow",
		Description:    "Manage CI/CD workflows (GitHub Actions)",
		RequiredFields: []string{"action", "description"},
		OptionalFields: []string{"workflow_id", "branch", "run_id"},
		Aliases:        []string{"workflow", "ci"},
		Category:       CategoryWorkflow,
		Parameters: map[string]Param{
			"action":      {Type: "string", Description: "Workflow action: run, list, view, cancel, logs", Required: true, Enum: []string{"run", "list", "view", "cancel", "logs"}},
			"workflow_id": {Type: "string", Description: "Workflow file name or ID", Required: false},
			"branch":      {Type: "string", Description: "Branch to run workflow on", Required: false},
			"run_id":      {Type: "integer", Description: "Run ID (for view/cancel/logs)", Required: false},
			"description": {Type: "string", Description: "Human-readable description", Required: true},
		},
	},
}

// GetToolSchema returns the schema for a tool by name (case-insensitive, alias-aware)
func GetToolSchema(name string) (*ToolSchema, bool) {
	nameLower := strings.ToLower(name)

	// Check direct match first
	if schema, ok := ToolSchemaRegistry[name]; ok {
		return schema, true
	}

	// Check aliases
	for _, schema := range ToolSchemaRegistry {
		for _, alias := range schema.Aliases {
			if strings.ToLower(alias) == nameLower {
				return schema, true
			}
		}
	}

	return nil, false
}

// GetRequiredFields returns the required fields for a tool
func GetRequiredFields(toolName string) []string {
	schema, ok := GetToolSchema(toolName)
	if !ok {
		return nil
	}
	return schema.RequiredFields
}

// ValidateToolArgs validates that all required fields are present in the arguments
func ValidateToolArgs(toolName string, args map[string]interface{}) error {
	schema, ok := GetToolSchema(toolName)
	if !ok {
		return fmt.Errorf("unknown tool: %s", toolName)
	}

	for _, field := range schema.RequiredFields {
		val, exists := args[field]
		if !exists {
			return fmt.Errorf("missing required field '%s' for tool '%s'", field, toolName)
		}
		// Check for empty strings
		if str, ok := val.(string); ok && str == "" {
			return fmt.Errorf("required field '%s' cannot be empty for tool '%s'", field, toolName)
		}
	}

	return nil
}

// GetAllToolNames returns all tool names including aliases
func GetAllToolNames() []string {
	var names []string
	for name := range ToolSchemaRegistry {
		names = append(names, name)
	}
	return names
}

// GetToolsByCategory returns all tools in a category
func GetToolsByCategory(category string) []*ToolSchema {
	var tools []*ToolSchema
	for _, schema := range ToolSchemaRegistry {
		if schema.Category == category {
			tools = append(tools, schema)
		}
	}
	return tools
}

// GenerateOpenAIToolDefinition generates an OpenAI-compatible tool definition
func GenerateOpenAIToolDefinition(schema *ToolSchema) map[string]interface{} {
	properties := make(map[string]interface{})
	required := []string{}

	for name, param := range schema.Parameters {
		prop := map[string]interface{}{
			"type":        param.Type,
			"description": param.Description,
		}
		if len(param.Enum) > 0 {
			prop["enum"] = param.Enum
		}
		properties[name] = prop

		if param.Required {
			required = append(required, name)
		}
	}

	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        schema.Name,
			"description": schema.Description,
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

// GenerateAllToolDefinitions generates OpenAI-compatible definitions for all tools
func GenerateAllToolDefinitions() []map[string]interface{} {
	var tools []map[string]interface{}
	for _, schema := range ToolSchemaRegistry {
		tools = append(tools, GenerateOpenAIToolDefinition(schema))
	}
	return tools
}

// ToJSON returns the tool schema as JSON
func (s *ToolSchema) ToJSON() (string, error) {
	bytes, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
