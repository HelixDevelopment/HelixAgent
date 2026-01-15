package skills

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService() *Service {
	config := DefaultSkillConfig()
	config.MinConfidence = 0.5
	service := NewService(config)

	// Register test skills
	skills := []*Skill{
		createTestSkill("docker-compose-creator", "devops", []string{"docker compose"}),
		createTestSkill("kubernetes-deployment", "devops", []string{"kubernetes"}),
		createTestSkill("security-scanner", "security", []string{"security scan"}),
	}

	for _, s := range skills {
		s.Instructions = "1. Step one\n2. Step two\n3. Step three"
		service.RegisterSkill(s)
	}

	return service
}

func TestProtocolSkillAdapter_RegisterAllSkillsAsTools(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)

	err := adapter.RegisterAllSkillsAsTools()
	require.NoError(t, err)

	// Check MCP tools
	mcpTools := adapter.GetMCPTools()
	assert.Len(t, mcpTools, 3)

	// Check ACP actions
	acpActions := adapter.GetACPActions()
	assert.Len(t, acpActions, 3)

	// Check LSP commands
	lspCommands := adapter.GetLSPCommands()
	assert.Len(t, lspCommands, 3)
}

func TestProtocolSkillAdapter_MCPToolFormat(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()

	mcpTools := adapter.GetMCPTools()
	require.NotEmpty(t, mcpTools)

	tool := mcpTools[0]
	assert.Contains(t, tool.Name, "skill_")
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.InputSchema)
	assert.NotNil(t, tool.Skill)

	// Check input schema structure
	schema := tool.InputSchema
	assert.Equal(t, "object", schema["type"])
	props := schema["properties"].(map[string]interface{})
	assert.Contains(t, props, "query")
	assert.Contains(t, props, "context")
}

func TestProtocolSkillAdapter_ACPActionFormat(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()

	actions := adapter.GetACPActions()
	require.NotEmpty(t, actions)

	action := actions[0]
	assert.Contains(t, action.Name, "skill.")
	assert.NotEmpty(t, action.Description)
	assert.NotNil(t, action.Parameters)
	assert.NotNil(t, action.Skill)
}

func TestProtocolSkillAdapter_LSPCommandFormat(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()

	commands := adapter.GetLSPCommands()
	require.NotEmpty(t, commands)

	cmd := commands[0]
	assert.Contains(t, cmd.Command, "helixagent.skill.")
	assert.NotEmpty(t, cmd.Title)
	assert.NotNil(t, cmd.Skill)
}

func TestProtocolSkillAdapter_InvokeMCPTool(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()
	ctx := context.Background()

	result, err := adapter.InvokeMCPTool(ctx, "skill_docker-compose-creator", map[string]interface{}{
		"query": "create a docker compose file",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, ProtocolMCP, result.Protocol)
	assert.Equal(t, "docker-compose-creator", result.SkillName)
	assert.False(t, result.IsError)
	assert.NotEmpty(t, result.Content)
	assert.NotEmpty(t, result.SkillsUsed)
	assert.Len(t, result.SkillsUsed, 1)
	assert.Equal(t, "docker-compose-creator", result.SkillsUsed[0].SkillName)
}

func TestProtocolSkillAdapter_InvokeACPAction(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()
	ctx := context.Background()

	result, err := adapter.InvokeACPAction(ctx, "skill.kubernetes-deployment", map[string]interface{}{
		"query": "deploy to kubernetes",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, ProtocolACP, result.Protocol)
	assert.Equal(t, "kubernetes-deployment", result.SkillName)
	assert.False(t, result.IsError)
}

func TestProtocolSkillAdapter_InvokeLSPCommand(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()
	ctx := context.Background()

	result, err := adapter.InvokeLSPCommand(ctx, "helixagent.skill.security-scanner", []interface{}{
		"scan for vulnerabilities",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, ProtocolLSP, result.Protocol)
	assert.Equal(t, "security-scanner", result.SkillName)
	assert.False(t, result.IsError)
}

func TestProtocolSkillAdapter_InvokeNonexistentSkill(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()
	ctx := context.Background()

	result, err := adapter.InvokeMCPTool(ctx, "skill_nonexistent", map[string]interface{}{
		"query": "test",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Error, "skill not found")
}

func TestProtocolSkillAdapter_ToMCPToolList(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()

	data, err := adapter.ToMCPToolList()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	tools := result["tools"].([]interface{})
	assert.Len(t, tools, 3)
}

func TestProtocolSkillAdapter_ToACPActionList(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()

	data, err := adapter.ToACPActionList()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	actions := result["actions"].([]interface{})
	assert.Len(t, actions, 3)
}

func TestProtocolSkillAdapter_ToLSPCommandList(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)
	adapter.RegisterAllSkillsAsTools()

	data, err := adapter.ToLSPCommandList()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	commands := result["commands"].([]interface{})
	assert.Len(t, commands, 3)
}

func TestGetSkillUsageHeader(t *testing.T) {
	usages := []SkillUsage{
		{SkillName: "docker-compose-creator"},
		{SkillName: "kubernetes-deployment"},
	}

	header := GetSkillUsageHeader(usages)
	assert.NotEmpty(t, header)

	var result map[string]interface{}
	err := json.Unmarshal([]byte(header), &result)
	require.NoError(t, err)

	assert.Equal(t, float64(2), result["count"])
	skillsUsed := result["skills_used"].([]interface{})
	assert.Len(t, skillsUsed, 2)
}

func TestGetSkillUsageHeader_Empty(t *testing.T) {
	header := GetSkillUsageHeader(nil)
	assert.Empty(t, header)

	header = GetSkillUsageHeader([]SkillUsage{})
	assert.Empty(t, header)
}

func TestExtractSkillName(t *testing.T) {
	service := setupTestService()
	adapter := NewProtocolSkillAdapter(service)

	tests := []struct {
		protocol   ProtocolType
		identifier string
		want       string
	}{
		{ProtocolMCP, "skill_docker-compose", "docker-compose"},
		{ProtocolACP, "skill.kubernetes", "kubernetes"},
		{ProtocolLSP, "helixagent.skill.security", "security"},
		{ProtocolMCP, "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.protocol)+"_"+tt.identifier, func(t *testing.T) {
			got := adapter.extractSkillName(tt.protocol, tt.identifier)
			assert.Equal(t, tt.want, got)
		})
	}
}
