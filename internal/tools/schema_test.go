package tools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolSchemaRegistry verifies all tools are properly registered
func TestToolSchemaRegistry(t *testing.T) {
	// Expected tools count: 9 existing + 12 new = 21 total
	expectedTools := []string{
		// Existing tools
		"Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch", "WebSearch", "Task",
		// New tools
		"Git", "Diff", "Test", "Lint", "TreeView", "FileInfo", "Symbols", "References", "Definition", "PR", "Issue", "Workflow",
	}

	for _, toolName := range expectedTools {
		t.Run(toolName, func(t *testing.T) {
			schema, ok := GetToolSchema(toolName)
			assert.True(t, ok, "Tool %s should be registered", toolName)
			assert.NotNil(t, schema, "Tool %s schema should not be nil", toolName)
			assert.NotEmpty(t, schema.Name, "Tool %s should have a name", toolName)
			assert.NotEmpty(t, schema.Description, "Tool %s should have a description", toolName)
			assert.NotEmpty(t, schema.Category, "Tool %s should have a category", toolName)
			assert.NotNil(t, schema.Parameters, "Tool %s should have parameters", toolName)
		})
	}
}

// TestToolSchemaRequiredFields verifies all tools have required fields defined
func TestToolSchemaRequiredFields(t *testing.T) {
	expectedRequiredFields := map[string][]string{
		"Bash":       {"command", "description"},
		"Read":       {"file_path"},
		"Write":      {"file_path", "content"},
		"Edit":       {"file_path", "old_string", "new_string"},
		"Glob":       {"pattern"},
		"Grep":       {"pattern"},
		"WebFetch":   {"url", "prompt"},
		"WebSearch":  {"query"},
		"Task":       {"prompt", "description", "subagent_type"},
		"Git":        {"operation", "description"},
		"Diff":       {"description"},
		"Test":       {"description"},
		"Lint":       {"description"},
		"TreeView":   {"description"},
		"FileInfo":   {"file_path", "description"},
		"Symbols":    {"description"},
		"References": {"symbol", "description"},
		"Definition": {"symbol", "description"},
		"PR":         {"action", "description"},
		"Issue":      {"action", "description"},
		"Workflow":   {"action", "description"},
	}

	for toolName, expectedFields := range expectedRequiredFields {
		t.Run(toolName, func(t *testing.T) {
			fields := GetRequiredFields(toolName)
			require.NotNil(t, fields, "Tool %s should have required fields", toolName)
			assert.ElementsMatch(t, expectedFields, fields,
				"Tool %s required fields mismatch", toolName)
		})
	}
}

// TestToolSchemaAliases verifies aliases work correctly
func TestToolSchemaAliases(t *testing.T) {
	aliasTests := []struct {
		alias    string
		expected string
	}{
		{"bash", "Bash"},
		{"shell", "Bash"},
		{"Shell", "Bash"},
		{"read", "Read"},
		{"write", "Write"},
		{"edit", "Edit"},
		{"glob", "Glob"},
		{"grep", "Grep"},
		{"webfetch", "WebFetch"},
		{"websearch", "WebSearch"},
		{"task", "Task"},
		{"git", "Git"},
		{"diff", "Diff"},
		{"test", "Test"},
		{"lint", "Lint"},
		{"treeview", "TreeView"},
		{"tree", "TreeView"},
		{"fileinfo", "FileInfo"},
		{"symbols", "Symbols"},
		{"references", "References"},
		{"refs", "References"},
		{"definition", "Definition"},
		{"goto", "Definition"},
		{"pr", "PR"},
		{"pullrequest", "PR"},
		{"issue", "Issue"},
		{"workflow", "Workflow"},
		{"ci", "Workflow"},
	}

	for _, tc := range aliasTests {
		t.Run(tc.alias, func(t *testing.T) {
			schema, ok := GetToolSchema(tc.alias)
			assert.True(t, ok, "Alias %s should resolve to a tool", tc.alias)
			if ok {
				assert.Equal(t, tc.expected, schema.Name,
					"Alias %s should resolve to %s", tc.alias, tc.expected)
			}
		})
	}
}

// TestValidateToolArgs validates tool argument validation
func TestValidateToolArgs(t *testing.T) {
	testCases := []struct {
		name      string
		toolName  string
		args      map[string]interface{}
		expectErr bool
	}{
		// Bash tool tests
		{
			name:     "Bash with all required fields",
			toolName: "Bash",
			args: map[string]interface{}{
				"command":     "go test -v ./...",
				"description": "Run tests",
			},
			expectErr: false,
		},
		{
			name:     "Bash missing description",
			toolName: "Bash",
			args: map[string]interface{}{
				"command": "go test -v ./...",
			},
			expectErr: true,
		},
		{
			name:     "Bash missing command",
			toolName: "Bash",
			args: map[string]interface{}{
				"description": "Run tests",
			},
			expectErr: true,
		},
		{
			name:     "Bash empty description",
			toolName: "Bash",
			args: map[string]interface{}{
				"command":     "go test",
				"description": "",
			},
			expectErr: true,
		},
		// Git tool tests
		{
			name:     "Git with required fields",
			toolName: "Git",
			args: map[string]interface{}{
				"operation":   "status",
				"description": "Check git status",
			},
			expectErr: false,
		},
		{
			name:     "Git missing operation",
			toolName: "Git",
			args: map[string]interface{}{
				"description": "Check git status",
			},
			expectErr: true,
		},
		// Test tool tests
		{
			name:     "Test with required fields",
			toolName: "Test",
			args: map[string]interface{}{
				"description": "Run tests",
			},
			expectErr: false,
		},
		// PR tool tests
		{
			name:     "PR with required fields",
			toolName: "PR",
			args: map[string]interface{}{
				"action":      "list",
				"description": "List PRs",
			},
			expectErr: false,
		},
		{
			name:     "PR missing action",
			toolName: "PR",
			args: map[string]interface{}{
				"description": "List PRs",
			},
			expectErr: true,
		},
		// Unknown tool
		{
			name:      "Unknown tool",
			toolName:  "UnknownTool",
			args:      map[string]interface{}{},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateToolArgs(tc.toolName, tc.args)
			if tc.expectErr {
				assert.Error(t, err, "Expected error for %s", tc.name)
			} else {
				assert.NoError(t, err, "Expected no error for %s", tc.name)
			}
		})
	}
}

// TestGenerateOpenAIToolDefinition verifies OpenAI tool definition generation
func TestGenerateOpenAIToolDefinition(t *testing.T) {
	for toolName, schema := range ToolSchemaRegistry {
		t.Run(toolName, func(t *testing.T) {
			def := GenerateOpenAIToolDefinition(schema)

			// Verify structure
			assert.Equal(t, "function", def["type"], "Tool %s type should be 'function'", toolName)

			function, ok := def["function"].(map[string]interface{})
			require.True(t, ok, "Tool %s should have function field", toolName)

			assert.Equal(t, schema.Name, function["name"], "Tool %s name mismatch", toolName)
			assert.Equal(t, schema.Description, function["description"], "Tool %s description mismatch", toolName)

			params, ok := function["parameters"].(map[string]interface{})
			require.True(t, ok, "Tool %s should have parameters field", toolName)

			assert.Equal(t, "object", params["type"], "Tool %s parameters type should be 'object'", toolName)

			properties, ok := params["properties"].(map[string]interface{})
			require.True(t, ok, "Tool %s should have properties field", toolName)

			// Verify all parameters are included
			for paramName := range schema.Parameters {
				_, exists := properties[paramName]
				assert.True(t, exists, "Tool %s should have parameter %s", toolName, paramName)
			}

			// Verify required fields are correct
			required, ok := params["required"].([]string)
			require.True(t, ok, "Tool %s should have required field as []string", toolName)
			assert.ElementsMatch(t, schema.RequiredFields, required,
				"Tool %s required fields mismatch in OpenAI definition", toolName)
		})
	}
}

// TestGenerateAllToolDefinitions verifies all tool definitions generation
func TestGenerateAllToolDefinitions(t *testing.T) {
	definitions := GenerateAllToolDefinitions()

	// Should have all registered tools
	assert.Len(t, definitions, len(ToolSchemaRegistry),
		"Should generate definitions for all registered tools")

	// Each definition should be valid JSON
	for i, def := range definitions {
		_, err := json.Marshal(def)
		assert.NoError(t, err, "Definition %d should be valid JSON", i)
	}
}

// TestToolSchemaToJSON verifies JSON serialization
func TestToolSchemaToJSON(t *testing.T) {
	for toolName, schema := range ToolSchemaRegistry {
		t.Run(toolName, func(t *testing.T) {
			jsonStr, err := schema.ToJSON()
			assert.NoError(t, err, "Tool %s should serialize to JSON", toolName)
			assert.NotEmpty(t, jsonStr, "Tool %s JSON should not be empty", toolName)

			// Verify it can be parsed back
			var parsed ToolSchema
			err = json.Unmarshal([]byte(jsonStr), &parsed)
			assert.NoError(t, err, "Tool %s JSON should be valid", toolName)
			assert.Equal(t, schema.Name, parsed.Name, "Tool %s name should match after roundtrip", toolName)
		})
	}
}

// TestGetAllToolNames verifies all tool names are returned
func TestGetAllToolNames(t *testing.T) {
	names := GetAllToolNames()
	assert.Len(t, names, len(ToolSchemaRegistry), "Should return all tool names")

	// Verify each name exists in registry
	for _, name := range names {
		_, ok := ToolSchemaRegistry[name]
		assert.True(t, ok, "Tool name %s should exist in registry", name)
	}
}

// TestGetToolsByCategory verifies category filtering
func TestGetToolsByCategory(t *testing.T) {
	categories := []string{
		CategoryCore,
		CategoryFileSystem,
		CategoryVersionControl,
		CategoryCodeIntel,
		CategoryWorkflow,
		CategoryWeb,
	}

	for _, category := range categories {
		t.Run(category, func(t *testing.T) {
			tools := GetToolsByCategory(category)
			assert.NotEmpty(t, tools, "Category %s should have at least one tool", category)

			for _, tool := range tools {
				assert.Equal(t, category, tool.Category,
					"Tool %s should be in category %s", tool.Name, category)
			}
		})
	}
}

// TestToolSchemaCategories verifies all tools have valid categories
func TestToolSchemaCategories(t *testing.T) {
	validCategories := map[string]bool{
		CategoryCore:           true,
		CategoryFileSystem:     true,
		CategoryVersionControl: true,
		CategoryCodeIntel:      true,
		CategoryWorkflow:       true,
		CategoryWeb:            true,
	}

	for toolName, schema := range ToolSchemaRegistry {
		t.Run(toolName, func(t *testing.T) {
			assert.True(t, validCategories[schema.Category],
				"Tool %s has invalid category: %s", toolName, schema.Category)
		})
	}
}

// TestToolParameterTypes verifies all parameters have valid types
func TestToolParameterTypes(t *testing.T) {
	validTypes := map[string]bool{
		"string":  true,
		"integer": true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}

	for toolName, schema := range ToolSchemaRegistry {
		for paramName, param := range schema.Parameters {
			t.Run(toolName+"/"+paramName, func(t *testing.T) {
				assert.True(t, validTypes[param.Type],
					"Tool %s parameter %s has invalid type: %s", toolName, paramName, param.Type)
				assert.NotEmpty(t, param.Description,
					"Tool %s parameter %s should have description", toolName, paramName)
			})
		}
	}
}

// TestToolEnumParameters verifies enum parameters have valid values
func TestToolEnumParameters(t *testing.T) {
	for toolName, schema := range ToolSchemaRegistry {
		for paramName, param := range schema.Parameters {
			if len(param.Enum) > 0 {
				t.Run(toolName+"/"+paramName, func(t *testing.T) {
					assert.Greater(t, len(param.Enum), 1,
						"Tool %s parameter %s enum should have more than 1 value", toolName, paramName)

					// Check no duplicates
					seen := make(map[string]bool)
					for _, v := range param.Enum {
						assert.False(t, seen[v],
							"Tool %s parameter %s has duplicate enum value: %s", toolName, paramName, v)
						seen[v] = true
					}
				})
			}
		}
	}
}
