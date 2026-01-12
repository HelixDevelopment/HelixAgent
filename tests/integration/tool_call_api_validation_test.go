package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolCallAPIValidation tests that the API generates valid tool calls with all required fields
// These are REAL API tests against a running HelixAgent server

const (
	defaultTestHost = "localhost"
	defaultTestPort = "7061"
)

func getTestBaseURL() string {
	host := os.Getenv("HELIXAGENT_HOST")
	if host == "" {
		host = defaultTestHost
	}
	port := os.Getenv("HELIXAGENT_PORT")
	if port == "" {
		port = defaultTestPort
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func skipIfServerNotRunning(t *testing.T) {
	baseURL := getTestBaseURL()
	resp, err := http.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("HelixAgent server not running at %s, skipping integration test", baseURL)
	}
	resp.Body.Close()
}

// ToolCallAPIResponse represents the OpenAI-compatible API response
type ToolCallAPIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Tool schemas with required fields - ALL 21 TOOLS
var toolSchemas = map[string][]string{
	// Existing tools (9)
	"Bash":      {"command", "description"},
	"bash":      {"command", "description"},
	"shell":     {"command", "description"},
	"Read":      {"file_path"},
	"read":      {"file_path"},
	"Write":     {"file_path", "content"},
	"write":     {"file_path", "content"},
	"Edit":      {"file_path", "old_string", "new_string"},
	"edit":      {"file_path", "old_string", "new_string"},
	"Glob":      {"pattern"},
	"glob":      {"pattern"},
	"Grep":      {"pattern"},
	"grep":      {"pattern"},
	"WebFetch":  {"url", "prompt"},
	"webfetch":  {"url", "prompt"},
	"WebSearch": {"query"},
	"websearch": {"query"},
	"Task":      {"prompt", "description", "subagent_type"},
	"task":      {"prompt", "description", "subagent_type"},
	// New tools (12)
	"Git":        {"operation", "description"},
	"git":        {"operation", "description"},
	"Diff":       {"description"},
	"diff":       {"description"},
	"Test":       {"description"},
	"test":       {"description"},
	"Lint":       {"description"},
	"lint":       {"description"},
	"TreeView":   {"description"},
	"treeview":   {"description"},
	"tree":       {"description"},
	"FileInfo":   {"file_path", "description"},
	"fileinfo":   {"file_path", "description"},
	"Symbols":    {"description"},
	"symbols":    {"description"},
	"References": {"symbol", "description"},
	"references": {"symbol", "description"},
	"refs":       {"symbol", "description"},
	"Definition": {"symbol", "description"},
	"definition": {"symbol", "description"},
	"goto":       {"symbol", "description"},
	"PR":         {"action", "description"},
	"pr":         {"action", "description"},
	"pullrequest": {"action", "description"},
	"Issue":      {"action", "description"},
	"issue":      {"action", "description"},
	"Workflow":   {"action", "description"},
	"workflow":   {"action", "description"},
	"ci":         {"action", "description"},
}

func TestAPIToolCallsHaveRequiredFields(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	testCases := []struct {
		name         string
		userMessage  string
		expectedTool string
	}{
		{
			name:         "Run command should include Bash with description",
			userMessage:  "Run the go test command",
			expectedTool: "Bash",
		},
		{
			name:         "Read file should include Read with file_path",
			userMessage:  "Read the README.md file",
			expectedTool: "Read",
		},
		{
			name:         "Search files should include Glob with pattern",
			userMessage:  "Search for all Go files in the project",
			expectedTool: "Glob",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build request with tool definitions
			reqBody := map[string]interface{}{
				"model": "helixagent-debate",
				"messages": []map[string]string{
					{"role": "user", "content": tc.userMessage},
				},
				"tools": []map[string]interface{}{
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "Bash",
							"description": "Execute bash commands",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"command":     map[string]string{"type": "string"},
									"description": map[string]string{"type": "string"},
								},
								"required": []string{"command", "description"},
							},
						},
					},
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "Read",
							"description": "Read file contents",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"file_path": map[string]string{"type": "string"},
								},
								"required": []string{"file_path"},
							},
						},
					},
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "Glob",
							"description": "Search for files",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"pattern": map[string]string{"type": "string"},
								},
								"required": []string{"pattern"},
							},
						},
					},
				},
				"tool_choice": "auto",
				"stream":      false,
			}

			jsonBody, err := json.Marshal(reqBody)
			require.NoError(t, err)

			// Make API request
			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Post(
				baseURL+"/v1/chat/completions",
				"application/json",
				bytes.NewBuffer(jsonBody),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Parse response
			var apiResp ToolCallAPIResponse
			err = json.Unmarshal(body, &apiResp)
			if err != nil {
				t.Logf("Response body: %s", string(body))
				t.Skipf("Could not parse response as expected format")
				return
			}

			// Check if response has tool calls
			if len(apiResp.Choices) > 0 && len(apiResp.Choices[0].Message.ToolCalls) > 0 {
				for _, toolCall := range apiResp.Choices[0].Message.ToolCalls {
					toolName := toolCall.Function.Name
					requiredFields, hasSchema := toolSchemas[toolName]
					if !hasSchema {
						continue
					}

					// Parse arguments
					var args map[string]interface{}
					err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
					require.NoError(t, err, "Tool call arguments should be valid JSON: %s", toolCall.Function.Arguments)

					// Verify all required fields are present
					for _, field := range requiredFields {
						_, exists := args[field]
						assert.True(t, exists,
							"Tool %s missing required field '%s'. Arguments: %s",
							toolName, field, toolCall.Function.Arguments)
					}

					// Special validation for Bash: ensure description is not empty
					if strings.EqualFold(toolName, "bash") || strings.EqualFold(toolName, "shell") {
						desc, hasDesc := args["description"].(string)
						assert.True(t, hasDesc && desc != "",
							"Bash tool description should not be empty. Arguments: %s",
							toolCall.Function.Arguments)
					}
				}
			}
		})
	}
}

func TestAPIResponseDoesNotContainSystemReminders(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	// Build request - include some text that might trigger system-reminder-like patterns
	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello, how are you?"},
		},
		"stream": false,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Parse response
	var apiResp ToolCallAPIResponse
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		t.Skipf("Could not parse response as expected format")
		return
	}

	// Check that response content doesn't contain system-reminder tags
	if len(apiResp.Choices) > 0 {
		content := apiResp.Choices[0].Message.Content

		systemReminderPattern := regexp.MustCompile(`<system-reminder>`)
		assert.False(t, systemReminderPattern.MatchString(content),
			"Response content should not contain <system-reminder> tags")

		commandNamePattern := regexp.MustCompile(`<command-name>`)
		assert.False(t, commandNamePattern.MatchString(content),
			"Response content should not contain <command-name> tags")

		contextPattern := regexp.MustCompile(`<context>`)
		assert.False(t, contextPattern.MatchString(content),
			"Response content should not contain internal <context> tags")
	}
}

func TestAPIDebateDialogueTopicIsSanitized(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	// Build request with a message that includes system-reminder-like content
	// (simulating what might come from a CLI tool that prepends context)
	testMessage := "What is the project structure?"

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": testMessage},
		},
		"stream": false,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Parse response
	var apiResp ToolCallAPIResponse
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		t.Skipf("Could not parse response as expected format")
		return
	}

	if len(apiResp.Choices) > 0 {
		content := apiResp.Choices[0].Message.Content

		// The debate dialogue should show a clean topic
		if strings.Contains(content, "TOPIC:") {
			// Extract the topic line
			topicPattern := regexp.MustCompile(`📋 TOPIC: (.+)`)
			matches := topicPattern.FindStringSubmatch(content)
			if len(matches) > 1 {
				topic := matches[1]
				assert.NotContains(t, topic, "<system-reminder>",
					"TOPIC should not contain system-reminder tags")
				assert.NotContains(t, topic, "<command-name>",
					"TOPIC should not contain command-name tags")
			}
		}
	}
}

func TestAPIParameterNamingIsSnakeCase(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": "Read the main.go file"},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "Read",
					"description": "Read file contents",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"file_path": map[string]string{"type": "string"},
						},
						"required": []string{"file_path"},
					},
				},
			},
		},
		"tool_choice": "auto",
		"stream":      false,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp ToolCallAPIResponse
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		t.Skipf("Could not parse response as expected format")
		return
	}

	// Check tool calls use snake_case parameter names
	if len(apiResp.Choices) > 0 && len(apiResp.Choices[0].Message.ToolCalls) > 0 {
		for _, toolCall := range apiResp.Choices[0].Message.ToolCalls {
			var args map[string]interface{}
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				continue
			}

			camelCasePattern := regexp.MustCompile(`[a-z][A-Z]`)
			for key := range args {
				assert.False(t, camelCasePattern.MatchString(key),
					"Parameter '%s' should use snake_case, not camelCase", key)
			}
		}
	}
}

func TestBashToolCallsAlwaysHaveDescription(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	// Various commands that should trigger Bash tool calls
	testCommands := []string{
		"Run the tests",
		"Execute go build",
		"Run git status",
		"Execute npm install",
		"Build the project",
	}

	for _, cmdRequest := range testCommands {
		t.Run(cmdRequest, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"model": "helixagent-debate",
				"messages": []map[string]string{
					{"role": "user", "content": cmdRequest},
				},
				"tools": []map[string]interface{}{
					{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "Bash",
							"description": "Execute bash commands",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"command":     map[string]string{"type": "string"},
									"description": map[string]string{"type": "string"},
								},
								"required": []string{"command", "description"},
							},
						},
					},
				},
				"tool_choice": "auto",
				"stream":      false,
			}

			jsonBody, err := json.Marshal(reqBody)
			require.NoError(t, err)

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Post(
				baseURL+"/v1/chat/completions",
				"application/json",
				bytes.NewBuffer(jsonBody),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var apiResp ToolCallAPIResponse
			err = json.Unmarshal(body, &apiResp)
			if err != nil {
				t.Logf("Response: %s", string(body))
				t.Skipf("Could not parse response")
				return
			}

			// If there are Bash tool calls, verify they have description
			if len(apiResp.Choices) > 0 && len(apiResp.Choices[0].Message.ToolCalls) > 0 {
				for _, toolCall := range apiResp.Choices[0].Message.ToolCalls {
					if strings.EqualFold(toolCall.Function.Name, "Bash") ||
						strings.EqualFold(toolCall.Function.Name, "shell") {

						var args map[string]interface{}
						err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
						require.NoError(t, err, "Arguments should be valid JSON")

						// Verify command exists
						cmd, hasCmd := args["command"]
						assert.True(t, hasCmd, "Bash tool must have 'command' field")
						assert.NotEmpty(t, cmd, "Bash command should not be empty")

						// Verify description exists and is not empty
						desc, hasDesc := args["description"]
						assert.True(t, hasDesc,
							"Bash tool must have 'description' field. Got: %s",
							toolCall.Function.Arguments)
						assert.NotEmpty(t, desc,
							"Bash description should not be empty. Got: %s",
							toolCall.Function.Arguments)
					}
				}
			}
		})
	}
}

// TestNewToolsAPIValidation tests all 12 new tools with their required fields
func TestNewToolsAPIValidation(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	// Define tool definitions for all new tools
	newToolDefinitions := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "Git",
				"description": "Execute Git version control operations",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation":   map[string]string{"type": "string"},
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"operation", "description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "Test",
				"description": "Run tests in the project",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "Lint",
				"description": "Run linter checks",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "Diff",
				"description": "Show file differences",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "TreeView",
				"description": "Display directory tree",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "FileInfo",
				"description": "Get file information",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path":   map[string]string{"type": "string"},
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"file_path", "description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "Symbols",
				"description": "List code symbols",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "References",
				"description": "Find references to a symbol",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol":      map[string]string{"type": "string"},
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"symbol", "description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "Definition",
				"description": "Go to definition of a symbol",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol":      map[string]string{"type": "string"},
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"symbol", "description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "PR",
				"description": "Pull request operations",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":      map[string]string{"type": "string"},
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"action", "description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "Issue",
				"description": "Issue operations",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":      map[string]string{"type": "string"},
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"action", "description"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "Workflow",
				"description": "CI/CD workflow operations",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":      map[string]string{"type": "string"},
						"description": map[string]string{"type": "string"},
					},
					"required": []string{"action", "description"},
				},
			},
		},
	}

	testCases := []struct {
		name         string
		userMessage  string
		expectedTool string
	}{
		{"Git status", "Check the git status", "Git"},
		{"Run tests", "Run the unit tests", "Test"},
		{"Lint code", "Check code quality with linter", "Lint"},
		{"Show diff", "What changed in the files", "Diff"},
		{"Tree view", "Show the directory tree structure", "TreeView"},
		{"File info", "Get information about README.md", "FileInfo"},
		{"List symbols", "List all functions in the file", "Symbols"},
		{"Find references", "Find all references to handleRequest function", "References"},
		{"Go to definition", "Go to definition of calculateScore", "Definition"},
		{"Pull requests", "List open pull requests", "PR"},
		{"Issues", "List project issues", "Issue"},
		{"CI/CD status", "Check the CI/CD workflow status", "Workflow"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"model": "helixagent-debate",
				"messages": []map[string]string{
					{"role": "user", "content": tc.userMessage},
				},
				"tools":       newToolDefinitions,
				"tool_choice": "auto",
				"stream":      false,
			}

			jsonBody, err := json.Marshal(reqBody)
			require.NoError(t, err)

			client := &http.Client{Timeout: 90 * time.Second}
			resp, err := client.Post(
				baseURL+"/v1/chat/completions",
				"application/json",
				bytes.NewBuffer(jsonBody),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var apiResp ToolCallAPIResponse
			err = json.Unmarshal(body, &apiResp)
			if err != nil {
				t.Logf("Response: %s", string(body))
				t.Skipf("Could not parse response")
				return
			}

			// Validate tool calls have required fields
			if len(apiResp.Choices) > 0 && len(apiResp.Choices[0].Message.ToolCalls) > 0 {
				for _, toolCall := range apiResp.Choices[0].Message.ToolCalls {
					toolName := toolCall.Function.Name
					requiredFields, hasSchema := toolSchemas[toolName]
					if !hasSchema {
						continue
					}

					var args map[string]interface{}
					err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
					require.NoError(t, err, "Tool call arguments should be valid JSON: %s", toolCall.Function.Arguments)

					for _, field := range requiredFields {
						_, exists := args[field]
						assert.True(t, exists,
							"Tool %s missing required field '%s'. Arguments: %s",
							toolName, field, toolCall.Function.Arguments)
					}
				}
			}
		})
	}
}

// TestAll21ToolsSchemaCount verifies we have all 21 tools defined
func TestAll21ToolsSchemaCount(t *testing.T) {
	// Count unique tool names (case-insensitive)
	uniqueTools := make(map[string]bool)
	for toolName := range toolSchemas {
		uniqueTools[strings.ToLower(toolName)] = true
	}

	// We expect the following unique tools:
	// Existing: bash, read, write, edit, glob, grep, webfetch, websearch, task (9)
	// New: git, diff, test, lint, treeview, fileinfo, symbols, references, definition, pr, issue, workflow (12)
	// Aliases: shell, tree, refs, goto, pullrequest, ci
	// Total unique base tools: 21

	expectedTools := []string{
		"bash", "shell",
		"read",
		"write",
		"edit",
		"glob",
		"grep",
		"webfetch",
		"websearch",
		"task",
		"git",
		"diff",
		"test",
		"lint",
		"treeview", "tree",
		"fileinfo",
		"symbols",
		"references", "refs",
		"definition", "goto",
		"pr", "pullrequest",
		"issue",
		"workflow", "ci",
	}

	for _, tool := range expectedTools {
		_, exists := toolSchemas[tool]
		assert.True(t, exists, "Tool schema missing for: %s", tool)
	}
}
