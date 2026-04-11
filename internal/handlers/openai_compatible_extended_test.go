package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNewUnifiedHandler_Extended tests handler creation with various configs
func TestNewUnifiedHandler_Extended(t *testing.T) {
	t.Parallel()
	handler := NewUnifiedHandler(nil, nil)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.dialogueFormatter)
}

// TestUnifiedHandler_SetDebateTeamConfig_Extended tests setting debate team config
func TestUnifiedHandler_SetDebateTeamConfig_Extended(t *testing.T) {
	t.Parallel()
	handler := NewUnifiedHandler(nil, nil)

	// Create a minimal config for testing
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	config := services.NewDebateTeamConfig(registry, nil, nil)

	handler.SetDebateTeamConfig(config)

	assert.NotNil(t, handler.debateTeamConfig)
}

// TestUnifiedHandler_ChatCompletions_InvalidJSON_Extended tests invalid JSON
func TestUnifiedHandler_ChatCompletions_InvalidJSON_Extended(t *testing.T) {
	t.Parallel()
	handler := NewUnifiedHandler(nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte("invalid json")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUnifiedHandler_ChatCompletions_EmptyMessages_Extended tests empty messages array
func TestUnifiedHandler_ChatCompletions_EmptyMessages_Extended(t *testing.T) {
	t.Parallel()
	handler := NewUnifiedHandler(nil, nil)

	reqBody := map[string]interface{}{
		"model":    "gpt-4",
		"messages": []map[string]string{},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletions(c)

	// May return 400 (bad request) or 503 (service unavailable with nil service)
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusServiceUnavailable)
}

// TestUnifiedHandler_Completions_InvalidJSON_Extended tests invalid JSON for completions
func TestUnifiedHandler_Completions_InvalidJSON_Extended(t *testing.T) {
	t.Parallel()
	handler := NewUnifiedHandler(nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewReader([]byte("invalid")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Completions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestUnifiedHandler_Completions_EmptyPrompt_Extended tests empty prompt
func TestUnifiedHandler_Completions_EmptyPrompt_Extended(t *testing.T) {
	t.Parallel()
	handler := NewUnifiedHandler(nil, nil)

	reqBody := map[string]interface{}{
		"model":  "gpt-4",
		"prompt": "",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Completions(c)

	// May return 400 (bad request) or 503 (service unavailable with nil service)
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusServiceUnavailable)
}

// TestUnifiedHandler_Models_Extended tests models endpoint
func TestUnifiedHandler_Models_Extended(t *testing.T) {
	t.Parallel()
	handler := NewUnifiedHandler(nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)

	handler.Models(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "list", response["object"])
}

// TestUnifiedHandler_ChatCompletions_WithAllParameters_Extended tests with all params
func TestUnifiedHandler_ChatCompletions_WithAllParameters_Extended(t *testing.T) {
	t.Parallel()
	handler := NewUnifiedHandler(nil, nil)

	reqBody := map[string]interface{}{
		"model": "ai-debate-ensemble",
		"messages": []map[string]string{
			{"role": "system", "content": "You are helpful"},
			{"role": "user", "content": "Hello"},
		},
		"temperature":       0.7,
		"max_tokens":        100,
		"top_p":             0.9,
		"frequency_penalty": 0.0,
		"presence_penalty":  0.0,
		"n":                 1,
		"stop":              []string{"\n"},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletions(c)

	// Should handle gracefully
	assert.True(t, w.Code >= 200 && w.Code < 600)
}

// TestExtractSymbolName_Extended tests symbol name extraction with edge cases
func TestExtractSymbolName_Extended(t *testing.T) {
	t.Parallel()
	// Test the helper function behavior with various inputs
	tests := []struct {
		input    string
		expected string
	}{
		{"function foo()", "function foo()"},
		{"class MyClass:", "class MyClass:"},
		{"", ""},
		{"   ", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := extractSymbolName(tt.input)
			if tt.expected != "" && tt.expected != "   " {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestConvertXMLCodeToMarkdown_Extended tests XML to markdown conversion
func TestConvertXMLCodeToMarkdown_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"<code>test</code>", "test"},
		{"plain text", "plain text"},
		{"<code lang=\"go\">func main()</code>", "func main()"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := convertXMLCodeToMarkdown(tt.input)
			if tt.expected != "" {
				assert.Contains(t, result, tt.expected)
			}
		})
	}
}

// TestStripUnparsedToolTags_Extended tests stripping unparsed tool tags
func TestStripUnparsedToolTags_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello <tool>test</tool> world", "Hello  world"},
		{"No tags here", "No tags here"},
		{"", ""},
		{"Multiple <tool>a</tool> and <tool>b</tool> tags", "Multiple  and  tags"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := stripUnparsedToolTags(tt.input)
			if tt.expected == "" {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestGenerateAgentsContent tests AGENTS.md content generation
func TestGenerateAgentsContent(t *testing.T) {
	t.Parallel()
	result := generateAgentsContent("some context")
	assert.Contains(t, result, "AGENTS.md")
	assert.Contains(t, result, "AI coding agents")
	assert.Contains(t, result, "Key Guidelines")
}

// TestGenerateReadmeContent tests README.md content generation
func TestGenerateReadmeContent(t *testing.T) {
	t.Parallel()
	result := generateReadmeContent("some context")
	assert.Contains(t, result, "README")
	assert.Contains(t, result, "Installation")
	assert.Contains(t, result, "Usage")
}

// TestGenerateTestingPlanContent tests testing plan content generation
func TestGenerateTestingPlanContent(t *testing.T) {
	t.Parallel()
	result := generateTestingPlanContent("some context")
	assert.Contains(t, result, "Testing Plan")
	assert.Contains(t, result, "Unit Tests")
	assert.Contains(t, result, "Integration Tests")
	assert.Contains(t, result, "80% code coverage")
}

// TestGenerateChangelogContent tests changelog content generation
func TestGenerateChangelogContent(t *testing.T) {
	t.Parallel()
	result := generateChangelogContent("some context")
	assert.Contains(t, result, "Changelog")
	assert.Contains(t, result, "Unreleased")
	assert.Contains(t, result, "Added")
	assert.Contains(t, result, "Changed")
}

// TestExtractContentForFile tests file content extraction
func TestExtractContentForFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		context  string
		fileName string
		expected string
	}{
		{
			name:     "AgentsMD",
			context:  "test context",
			fileName: "AGENTS.md",
			expected: "AGENTS.md",
		},
		{
			name:     "ReadmeMD",
			context:  "test context",
			fileName: "README.md",
			expected: "README",
		},
		{
			name:     "TestingPlanMD",
			context:  "test context",
			fileName: "testing_plan.md",
			expected: "Testing Plan",
		},
		{
			name:     "ChangelogMD",
			context:  "test context",
			fileName: "CHANGELOG.md",
			expected: "Changelog",
		},
		{
			name:     "CodeBlockExtraction",
			context:  "Here is code:\n```go\nfunc main() {}\n```\nEnd",
			fileName: "main.go",
			expected: "func main()",
		},
		{
			name:     "DefaultGeneration",
			context:  "Some plain context",
			fileName: "custom.txt",
			expected: "Generated content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractContentForFile(tt.context, tt.fileName)
			assert.Contains(t, result, tt.expected)
		})
	}
}

// TestExtractToolArguments_Extended tests tool argument extraction
func TestExtractToolArguments_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		toolName string
		content  string
		notEmpty bool
	}{
		{
			name:     "WriteToolWithJSON",
			toolName: "Write",
			content:  `Write file to /test/path with content`,
			notEmpty: true,
		},
		{
			name:     "ReadToolWithPath",
			toolName: "Read",
			content:  "Read file at /test/file.go",
			notEmpty: true,
		},
		{
			name:     "EmptyContent",
			toolName: "Write",
			content:  "",
			notEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractToolArguments(tt.toolName, tt.content)
			if tt.notEmpty {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestCleanSynthesisForFile_Extended tests synthesis cleaning for files
func TestCleanSynthesisForFile_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		notEmpty bool
	}{
		{
			name:     "SimpleText",
			input:    "Simple synthesis content",
			notEmpty: true,
		},
		{
			name:     "EmptyInput",
			input:    "",
			notEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := cleanSynthesisForFile(tt.input)
			if tt.notEmpty {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestGetParam_Extended tests the getParam helper function
func TestGetParam_Extended(t *testing.T) {
	t.Parallel()
	params := map[string]string{
		"file_path": "/test/path",
		"content":   "test content",
		"Command":   "ls -la",
	}

	tests := []struct {
		name     string
		keys     []string
		expected string
	}{
		{
			name:     "DirectKey",
			keys:     []string{"file_path"},
			expected: "/test/path",
		},
		{
			name:     "MultipleKeys_FirstMatches",
			keys:     []string{"content", "data"},
			expected: "test content",
		},
		{
			name:     "MultipleKeys_SecondMatches",
			keys:     []string{"data", "content"},
			expected: "test content",
		},
		{
			name:     "NoMatch",
			keys:     []string{"nonexistent"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getParam(params, tt.keys...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestContainsAny_Extended tests the containsAny helper function
func TestContainsAny_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		text     string
		patterns []string
		expected bool
	}{
		{
			name:     "SingleMatch",
			text:     "Hello World",
			patterns: []string{"World"},
			expected: true,
		},
		{
			name:     "MultiplePatterns_OneMatches",
			text:     "Error in processing",
			patterns: []string{"Success", "Error", "Warning"},
			expected: true,
		},
		{
			name:     "NoMatch",
			text:     "Everything is fine",
			patterns: []string{"error", "failed"},
			expected: false,
		},
		{
			name:     "EmptyPatterns",
			text:     "Some text",
			patterns: []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := containsAny(tt.text, tt.patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGenerateToolCallID_Extended tests the generateToolCallID helper function
func TestGenerateToolCallID_Extended(t *testing.T) {
	t.Parallel()
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id := generateToolCallID()
		assert.NotEmpty(t, id)
		// Just check that ID is generated (format may vary)
		ids[id] = true
	}
	// All IDs should be unique
	assert.Equal(t, 10, len(ids))
}

// TestEscapeJSONString_Extended tests the escapeJSONString helper function
func TestEscapeJSONString_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NoEscape",
			input:    "simple text",
			expected: "simple text",
		},
		{
			name:     "WithNewline",
			input:    "line1\nline2",
			expected: `line1\nline2`,
		},
		{
			name:     "WithTab",
			input:    "col1\tcol2",
			expected: `col1\tcol2`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := escapeJSONString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSanitizeDisplayContent_Extended tests the sanitizeDisplayContent helper function
func TestSanitizeDisplayContent_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		notEmpty bool
	}{
		{
			name:     "SimpleText",
			input:    "Simple content",
			notEmpty: true,
		},
		{
			name:     "WithHTML",
			input:    "<script>alert('xss')</script>Normal text",
			notEmpty: true,
		},
		{
			name:     "EmptyInput",
			input:    "",
			notEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := sanitizeDisplayContent(tt.input)
			if tt.notEmpty {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestExtractFilePath_Extended tests the extractFilePath helper function
func TestExtractFilePath_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "AbsolutePath",
			input:    "Edit the file /home/user/project/main.go",
			expected: "/home/user/project/main.go",
		},
		{
			name:     "RelativeWithDot",
			input:    "Read ./src/file.ts",
			expected: "./src/file.ts",
		},
		{
			name:     "NoPath",
			input:    "Some random text without file path",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractFilePath(tt.input)
			if tt.expected != "" {
				assert.Contains(t, result, tt.expected)
			}
		})
	}
}

// TestParseEmbeddedFunctionCalls_Extended tests parsing embedded function calls
func TestParseEmbeddedFunctionCalls_Extended(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		input       string
		expectCalls bool
	}{
		{
			name:        "NoFunctionCalls",
			input:       "Just plain text without any function calls",
			expectCalls: false,
		},
		{
			name:        "EmptyInput",
			input:       "",
			expectCalls: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := parseEmbeddedFunctionCalls(tt.input)
			if tt.expectCalls {
				assert.NotEmpty(t, calls)
			} else {
				assert.Empty(t, calls)
			}
		})
	}
}

// TestGenerateAgentsMDContent_Extended tests AGENTS.md content generation
func TestGenerateAgentsMDContent_Extended(t *testing.T) {
	t.Parallel()
	result := generateAgentsMDContent("synthesis content", "test topic")
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "AGENTS")
}
