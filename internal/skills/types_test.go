package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAllowedTools_EmptyString(t *testing.T) {
	result := ParseAllowedTools("")
	assert.Nil(t, result)
}

func TestParseAllowedTools_SingleTool(t *testing.T) {
	result := ParseAllowedTools("Read")
	require.Len(t, result, 1)
	assert.Equal(t, "Read", result[0].Name)
	assert.Nil(t, result[0].Constraints)
}

func TestParseAllowedTools_MultipleTools(t *testing.T) {
	result := ParseAllowedTools("Read, Write, Edit")
	require.Len(t, result, 3)
	assert.Equal(t, "Read", result[0].Name)
	assert.Equal(t, "Write", result[1].Name)
	assert.Equal(t, "Edit", result[2].Name)
}

func TestParseAllowedTools_ToolWithConstraints(t *testing.T) {
	result := ParseAllowedTools("Bash(cmd:*)")
	require.Len(t, result, 1)
	assert.Equal(t, "Bash", result[0].Name)
	require.NotNil(t, result[0].Constraints)
	assert.Equal(t, "*", result[0].Constraints["cmd"])
}

func TestParseAllowedTools_ToolWithMultipleConstraints(t *testing.T) {
	result := ParseAllowedTools("Glob(pattern:*.go, path:/src)")
	require.Len(t, result, 1)
	assert.Equal(t, "Glob", result[0].Name)
	require.NotNil(t, result[0].Constraints)
	assert.Equal(t, "*.go", result[0].Constraints["pattern"])
	assert.Equal(t, "/src", result[0].Constraints["path"])
}

func TestParseAllowedTools_MixedToolsWithAndWithoutConstraints(t *testing.T) {
	result := ParseAllowedTools("Read, Write, Bash(cmd:ls,cmd:cat), Edit")
	require.Len(t, result, 4)

	assert.Equal(t, "Read", result[0].Name)
	assert.Nil(t, result[0].Constraints)

	assert.Equal(t, "Write", result[1].Name)
	assert.Nil(t, result[1].Constraints)

	assert.Equal(t, "Bash", result[2].Name)
	require.NotNil(t, result[2].Constraints)
	assert.Equal(t, "ls,cat", result[2].Constraints["cmd"])

	assert.Equal(t, "Edit", result[3].Name)
	assert.Nil(t, result[3].Constraints)
}

func TestParseAllowedTools_ComplexConstraints(t *testing.T) {
	result := ParseAllowedTools("WebFetch(url:https://example.com)")
	require.Len(t, result, 1)
	assert.Equal(t, "WebFetch", result[0].Name)
	assert.Equal(t, "https://example.com", result[0].Constraints["url"])
}

func TestParseAllowedTools_WhitespaceHandling(t *testing.T) {
	result := ParseAllowedTools("  Read  ,   Write   ,  Edit  ")
	require.Len(t, result, 3)
	assert.Equal(t, "Read", result[0].Name)
	assert.Equal(t, "Write", result[1].Name)
	assert.Equal(t, "Edit", result[2].Name)
}

func TestParseAllowedTools_NestedParentheses(t *testing.T) {
	// Test that nested parentheses don't break parsing
	result := ParseAllowedTools("Bash(cmd:echo \"hello (world)\")")
	require.Len(t, result, 1)
	assert.Equal(t, "Bash", result[0].Name)
}

func TestParseToolString_SimpleToolName(t *testing.T) {
	result := parseToolString("Read")
	assert.Equal(t, "Read", result.Name)
	assert.Nil(t, result.Constraints)
}

func TestParseToolString_ToolWithConstraint(t *testing.T) {
	result := parseToolString("Bash(cmd:*)")
	assert.Equal(t, "Bash", result.Name)
	assert.Equal(t, "*", result.Constraints["cmd"])
}

func TestParseToolString_EmptyConstraints(t *testing.T) {
	result := parseToolString("Tool()")
	assert.Equal(t, "Tool", result.Name)
	assert.Empty(t, result.Constraints)
}

func TestDefaultSkillConfig(t *testing.T) {
	config := DefaultSkillConfig()
	assert.Equal(t, "skills", config.SkillsDirectory)
	assert.True(t, config.EnableSemanticMatching)
	assert.Equal(t, 0.7, config.MinConfidence)
	assert.Equal(t, 5, config.MaxConcurrentSkills)
	assert.True(t, config.TrackUsage)
	assert.True(t, config.HotReload)
}

func TestMatchType_Constants(t *testing.T) {
	assert.Equal(t, MatchType("exact"), MatchTypeExact)
	assert.Equal(t, MatchType("partial"), MatchTypePartial)
	assert.Equal(t, MatchType("semantic"), MatchTypeSemantic)
	assert.Equal(t, MatchType("fuzzy"), MatchTypeFuzzy)
}
