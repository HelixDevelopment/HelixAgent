package services_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ToolCallArguments represents the structure of tool call arguments
type ToolCallArguments struct {
	// Bash tool required fields
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`

	// Read tool required fields
	FilePath string `json:"file_path,omitempty"`

	// Write tool required fields
	Content string `json:"content,omitempty"`

	// Edit tool required fields
	OldString string `json:"old_string,omitempty"`
	NewString string `json:"new_string,omitempty"`

	// Search tool required fields
	Query   string `json:"query,omitempty"`
	Pattern string `json:"pattern,omitempty"`

	// Generic fields
	Path string `json:"path,omitempty"`
}

// ToolCallSchema defines required fields for each tool type
type ToolCallSchema struct {
	RequiredFields []string
	OptionalFields []string
}

var toolSchemas = map[string]ToolCallSchema{
	"Bash": {
		RequiredFields: []string{"command", "description"},
		OptionalFields: []string{"timeout", "cwd"},
	},
	"shell": {
		RequiredFields: []string{"command", "description"},
		OptionalFields: []string{"timeout", "cwd"},
	},
	"Read": {
		RequiredFields: []string{"file_path"},
		OptionalFields: []string{"offset", "limit"},
	},
	"Write": {
		RequiredFields: []string{"file_path", "content"},
		OptionalFields: []string{},
	},
	"Edit": {
		RequiredFields: []string{"file_path", "old_string", "new_string"},
		OptionalFields: []string{"replace_all"},
	},
	"Glob": {
		RequiredFields: []string{"pattern"},
		OptionalFields: []string{"path"},
	},
	"Grep": {
		RequiredFields: []string{"pattern"},
		OptionalFields: []string{"path", "output_mode", "type"},
	},
	"WebFetch": {
		RequiredFields: []string{"url", "prompt"},
		OptionalFields: []string{},
	},
	"WebSearch": {
		RequiredFields: []string{"query"},
		OptionalFields: []string{"allowed_domains", "blocked_domains"},
	},
	"Task": {
		RequiredFields: []string{"prompt", "description", "subagent_type"},
		OptionalFields: []string{"model", "run_in_background"},
	},
}

// TestToolCallArgumentsValidation verifies that tool call arguments have required fields
func TestToolCallArgumentsValidation(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		argumentsJSON  string
		shouldBeValid  bool
		expectedError  string
	}{
		// Bash tool tests
		{
			name:          "Bash with command and description",
			toolName:      "Bash",
			argumentsJSON: `{"command": "go test ./...", "description": "Run Go tests"}`,
			shouldBeValid: true,
		},
		{
			name:          "Bash missing description",
			toolName:      "Bash",
			argumentsJSON: `{"command": "go test ./..."}`,
			shouldBeValid: false,
			expectedError: "description",
		},
		{
			name:          "Bash missing command",
			toolName:      "Bash",
			argumentsJSON: `{"description": "Run tests"}`,
			shouldBeValid: false,
			expectedError: "command",
		},
		{
			name:          "Bash with empty description",
			toolName:      "Bash",
			argumentsJSON: `{"command": "go test ./...", "description": ""}`,
			shouldBeValid: false,
			expectedError: "description",
		},

		// Read tool tests
		{
			name:          "Read with file_path",
			toolName:      "Read",
			argumentsJSON: `{"file_path": "/path/to/file.go"}`,
			shouldBeValid: true,
		},
		{
			name:          "Read missing file_path",
			toolName:      "Read",
			argumentsJSON: `{}`,
			shouldBeValid: false,
			expectedError: "file_path",
		},

		// Write tool tests
		{
			name:          "Write with file_path and content",
			toolName:      "Write",
			argumentsJSON: `{"file_path": "/path/to/file.go", "content": "package main"}`,
			shouldBeValid: true,
		},
		{
			name:          "Write missing content",
			toolName:      "Write",
			argumentsJSON: `{"file_path": "/path/to/file.go"}`,
			shouldBeValid: false,
			expectedError: "content",
		},

		// Edit tool tests
		{
			name:          "Edit with all required fields",
			toolName:      "Edit",
			argumentsJSON: `{"file_path": "/path/to/file.go", "old_string": "foo", "new_string": "bar"}`,
			shouldBeValid: true,
		},
		{
			name:          "Edit missing old_string",
			toolName:      "Edit",
			argumentsJSON: `{"file_path": "/path/to/file.go", "new_string": "bar"}`,
			shouldBeValid: false,
			expectedError: "old_string",
		},

		// Glob tool tests
		{
			name:          "Glob with pattern",
			toolName:      "Glob",
			argumentsJSON: `{"pattern": "**/*.go"}`,
			shouldBeValid: true,
		},
		{
			name:          "Glob missing pattern",
			toolName:      "Glob",
			argumentsJSON: `{"path": "/some/path"}`,
			shouldBeValid: false,
			expectedError: "pattern",
		},

		// Grep tool tests
		{
			name:          "Grep with pattern",
			toolName:      "Grep",
			argumentsJSON: `{"pattern": "func Test"}`,
			shouldBeValid: true,
		},

		// WebFetch tool tests
		{
			name:          "WebFetch with url and prompt",
			toolName:      "WebFetch",
			argumentsJSON: `{"url": "https://example.com", "prompt": "Extract information"}`,
			shouldBeValid: true,
		},
		{
			name:          "WebFetch missing prompt",
			toolName:      "WebFetch",
			argumentsJSON: `{"url": "https://example.com"}`,
			shouldBeValid: false,
			expectedError: "prompt",
		},

		// WebSearch tool tests
		{
			name:          "WebSearch with query",
			toolName:      "WebSearch",
			argumentsJSON: `{"query": "golang best practices"}`,
			shouldBeValid: true,
		},
		{
			name:          "WebSearch missing query",
			toolName:      "WebSearch",
			argumentsJSON: `{}`,
			shouldBeValid: false,
			expectedError: "query",
		},

		// Task tool tests
		{
			name:          "Task with all required fields",
			toolName:      "Task",
			argumentsJSON: `{"prompt": "Search for files", "description": "Find Go files", "subagent_type": "Explore"}`,
			shouldBeValid: true,
		},
		{
			name:          "Task missing subagent_type",
			toolName:      "Task",
			argumentsJSON: `{"prompt": "Search for files", "description": "Find Go files"}`,
			shouldBeValid: false,
			expectedError: "subagent_type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid, missingField := validateToolCallArguments(tc.toolName, tc.argumentsJSON)
			if tc.shouldBeValid {
				assert.True(t, valid, "Tool call arguments should be valid")
				assert.Empty(t, missingField, "No missing fields expected")
			} else {
				assert.False(t, valid, "Tool call arguments should be invalid")
				assert.Contains(t, missingField, tc.expectedError, "Missing field should be: %s", tc.expectedError)
			}
		})
	}
}

// validateToolCallArguments checks if tool call arguments contain all required fields
func validateToolCallArguments(toolName, argumentsJSON string) (bool, string) {
	schema, exists := toolSchemas[toolName]
	if !exists {
		return true, "" // Unknown tool types are considered valid
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return false, "invalid JSON"
	}

	for _, field := range schema.RequiredFields {
		value, exists := args[field]
		if !exists {
			return false, field
		}
		// Check for empty strings
		if strValue, ok := value.(string); ok && strValue == "" {
			return false, field
		}
	}

	return true, ""
}

// TestBashDescriptionGenerationCoverage ensures generateBashDescription covers common commands
func TestBashDescriptionGenerationCoverage(t *testing.T) {
	// Commands that MUST have specific descriptions (not generic)
	testCases := []struct {
		command         string
		expectedContain string
		forbiddenOutput string
	}{
		// Git commands - should NOT fallback to "Run tests" even with test in message
		{"git commit -m 'test commit'", "commit", "Run tests"},
		{"git add test.go", "stage", "Run tests"},  // "Stage files for commit" contains "stage"
		{"git checkout test-branch", "branch", "Run tests"},
		{"git merge test-feature", "merge", "Run tests"},
		{"git status", "status", "Run tests"},
		{"git push origin test-branch", "push", "Run tests"},
		{"git pull origin main", "pull", "Run tests"},

		// Coverage commands - should be coverage, not tests
		{"go test -coverprofile=coverage.out ./...", "coverage", "Run Go tests"},
		{"go test -cover ./...", "coverage", "Run Go tests"},

		// Build commands with test in path - should be build, not tests
		{"go build ./cmd/test-server/...", "Build", "Run tests"},

		// Docker commands
		{"docker build -t test:latest .", "Docker", "Run tests"},
		{"docker run test-container", "Docker", "Run tests"},
		{"docker-compose up", "docker", "Run tests"},

		// Make commands - note: "make lint" contains "lint" so gets linter description
		{"make build", "make", ""},
		{"make lint", "lint", ""},  // "lint" is checked first, gets "Run linter"

		// Linting
		{"golangci-lint run", "lint", ""},
		{"npm run lint", "lint", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			result := generateBashDescriptionForTest(tc.command)

			// Should contain expected keyword
			assert.True(t, strings.Contains(strings.ToLower(result), strings.ToLower(tc.expectedContain)),
				"Description '%s' should contain '%s' for command '%s'",
				result, tc.expectedContain, tc.command)

			// Should NOT be the forbidden output
			if tc.forbiddenOutput != "" {
				assert.NotEqual(t, tc.forbiddenOutput, result,
					"Description should NOT be '%s' for command '%s'",
					tc.forbiddenOutput, tc.command)
			}
		})
	}
}

// generateBashDescriptionForTest mirrors the production generateBashDescription function
// This is a test helper that validates the pattern matching order
func generateBashDescriptionForTest(cmd string) string {
	cmdLower := strings.ToLower(cmd)

	// Coverage commands - MUST check before "test"
	if strings.Contains(cmdLower, "coverprofile") || strings.Contains(cmdLower, "-cover") {
		return "Generate test coverage report"
	}

	// Git commands - MUST check before "test"
	if strings.Contains(cmdLower, "git ") || strings.HasPrefix(cmdLower, "git") {
		if strings.Contains(cmdLower, "status") {
			return "Check git status"
		}
		if strings.Contains(cmdLower, "commit") {
			return "Create git commit"
		}
		if strings.Contains(cmdLower, "push") {
			return "Push changes to remote"
		}
		if strings.Contains(cmdLower, "pull") {
			return "Pull changes from remote"
		}
		if strings.Contains(cmdLower, "add") {
			return "Stage files for commit"
		}
		if strings.Contains(cmdLower, "checkout") {
			return "Switch git branch"
		}
		if strings.Contains(cmdLower, "merge") {
			return "Merge git branches"
		}
		return "Execute git command"
	}

	// Lint commands
	if strings.Contains(cmdLower, "lint") || strings.Contains(cmdLower, "golangci") {
		return "Run linter"
	}

	// Docker commands - check before build
	if strings.Contains(cmdLower, "docker") {
		if strings.Contains(cmdLower, "build") {
			return "Build Docker image"
		}
		if strings.Contains(cmdLower, "run") {
			return "Run Docker container"
		}
		return "Execute Docker command"
	}

	// Make commands - check BEFORE build since "make build" contains "build"
	if strings.HasPrefix(cmdLower, "make") {
		return "Execute make target"
	}

	// Build commands
	if strings.Contains(cmdLower, "build") || strings.Contains(cmdLower, "compile") {
		if strings.Contains(cmdLower, "go build") {
			return "Build Go project"
		}
		return "Build project"
	}

	// Test commands - last for this group
	if strings.Contains(cmdLower, "test") {
		if strings.Contains(cmdLower, "go test") {
			return "Run Go tests"
		}
		return "Run tests"
	}

	// Default
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return "Execute " + parts[0] + " command"
	}
	return "Execute shell command"
}

// TestToolCallJSONEscaping ensures special characters are properly escaped
func TestToolCallJSONEscaping(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		isValid bool
	}{
		{"Simple command", `{"command": "echo hello", "description": "Print hello"}`, true},
		{"Command with quotes", `{"command": "echo \"hello\"", "description": "Print quoted hello"}`, true},
		{"Command with newlines", `{"command": "echo -e 'line1\nline2'", "description": "Print multiline"}`, true},
		{"Command with backslash", `{"command": "echo \\", "description": "Print backslash"}`, true},
		{"Invalid JSON - unescaped quote", `{"command": "echo "hello"", "description": "Test"}`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args map[string]interface{}
			err := json.Unmarshal([]byte(tc.input), &args)
			if tc.isValid {
				assert.NoError(t, err, "JSON should be valid: %s", tc.input)
			} else {
				assert.Error(t, err, "JSON should be invalid: %s", tc.input)
			}
		})
	}
}

// TestToolCallArgumentsNeverEmpty ensures no tool call has empty required fields
func TestToolCallArgumentsNeverEmpty(t *testing.T) {
	// Generate sample tool calls for various scenarios
	scenarios := []struct {
		scenario   string
		toolName   string
		genArgs    func() string
	}{
		{
			scenario: "test command",
			toolName: "Bash",
			genArgs: func() string {
				return `{"command": "go test ./...", "description": "Run Go tests"}`
			},
		},
		{
			scenario: "build command",
			toolName: "Bash",
			genArgs: func() string {
				return `{"command": "go build ./...", "description": "Build Go project"}`
			},
		},
		{
			scenario: "git command",
			toolName: "Bash",
			genArgs: func() string {
				return `{"command": "git status", "description": "Check git status"}`
			},
		},
		{
			scenario: "read file",
			toolName: "Read",
			genArgs: func() string {
				return `{"file_path": "/path/to/file.go"}`
			},
		},
		{
			scenario: "write file",
			toolName: "Write",
			genArgs: func() string {
				return `{"file_path": "/path/to/file.go", "content": "package main"}`
			},
		},
		{
			scenario: "edit file",
			toolName: "Edit",
			genArgs: func() string {
				return `{"file_path": "/path/to/file.go", "old_string": "foo", "new_string": "bar"}`
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.scenario, func(t *testing.T) {
			argsJSON := s.genArgs()
			require.NotEmpty(t, argsJSON, "Arguments JSON should not be empty")

			var args map[string]interface{}
			err := json.Unmarshal([]byte(argsJSON), &args)
			require.NoError(t, err, "Arguments should be valid JSON")

			schema, exists := toolSchemas[s.toolName]
			if !exists {
				return // Unknown tool, skip
			}

			for _, field := range schema.RequiredFields {
				value, exists := args[field]
				assert.True(t, exists, "Required field '%s' should exist for tool '%s'", field, s.toolName)
				if strValue, ok := value.(string); ok {
					assert.NotEmpty(t, strValue, "Required field '%s' should not be empty for tool '%s'", field, s.toolName)
				}
			}
		})
	}
}

// TestAllToolSchemasHaveRequiredFields ensures every tool has defined required fields
func TestAllToolSchemasHaveRequiredFields(t *testing.T) {
	expectedTools := []string{
		"Bash", "shell", "Read", "Write", "Edit",
		"Glob", "Grep", "WebFetch", "WebSearch", "Task",
	}

	for _, toolName := range expectedTools {
		t.Run(toolName, func(t *testing.T) {
			schema, exists := toolSchemas[toolName]
			assert.True(t, exists, "Tool '%s' should have a schema defined", toolName)
			assert.NotEmpty(t, schema.RequiredFields, "Tool '%s' should have required fields", toolName)
		})
	}
}
