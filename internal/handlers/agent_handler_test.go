package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewAgentHandler(t *testing.T) {
	handler := NewAgentHandler()
	require.NotNil(t, handler)
}

func TestAgentHandler_ListAgents(t *testing.T) {
	handler := NewAgentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/agents", nil)

	handler.ListAgents(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AgentListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should have registered agents from the agents package
	assert.Greater(t, response.Count, 0)
	assert.Len(t, response.Agents, response.Count)

	// Verify first agent has expected fields
	if len(response.Agents) > 0 {
		agent := response.Agents[0]
		assert.NotEmpty(t, agent.Name)
		assert.NotEmpty(t, agent.Description)
	}
}

func TestAgentHandler_GetAgent_Found(t *testing.T) {
	handler := NewAgentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/agents/OpenCode", nil)
	c.Params = gin.Params{{Key: "name", Value: "OpenCode"}}

	handler.GetAgent(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AgentResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "OpenCode", response.Name)
	assert.NotEmpty(t, response.Description)
	assert.NotEmpty(t, response.Language)
}

func TestAgentHandler_GetAgent_NotFound(t *testing.T) {
	handler := NewAgentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/agents/NonExistentAgent", nil)
	c.Params = gin.Params{{Key: "name", Value: "NonExistentAgent"}}

	handler.GetAgent(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "agent_not_found", response["error"])
	assert.Contains(t, response["message"], "NonExistentAgent")
}

func TestAgentHandler_ListAgentsByProtocol(t *testing.T) {
	handler := NewAgentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/agents/protocol/MCP", nil)
	c.Params = gin.Params{{Key: "protocol", Value: "MCP"}}

	handler.ListAgentsByProtocol(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AgentListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should return agents that support MCP protocol
	assert.GreaterOrEqual(t, response.Count, 0)
	assert.Len(t, response.Agents, response.Count)

	// If there are MCP agents, verify they have MCP in protocols
	for _, agent := range response.Agents {
		assert.Contains(t, agent.Protocols, "MCP")
	}
}

func TestAgentHandler_ListAgentsByProtocol_NoMatches(t *testing.T) {
	handler := NewAgentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/agents/protocol/NonExistentProtocol", nil)
	c.Params = gin.Params{{Key: "protocol", Value: "NonExistentProtocol"}}

	handler.ListAgentsByProtocol(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AgentListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 0, response.Count)
	assert.Empty(t, response.Agents)
}

func TestAgentHandler_ListAgentsByTool(t *testing.T) {
	handler := NewAgentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/agents/tool/Bash", nil)
	c.Params = gin.Params{{Key: "tool", Value: "Bash"}}

	handler.ListAgentsByTool(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AgentListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should return agents that support Bash tool
	assert.Greater(t, response.Count, 0)
	assert.Len(t, response.Agents, response.Count)

	// Verify all returned agents have Bash in tool support
	for _, agent := range response.Agents {
		assert.Contains(t, agent.ToolSupport, "Bash")
	}
}

func TestAgentHandler_ListAgentsByTool_NoMatches(t *testing.T) {
	handler := NewAgentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/agents/tool/nonexistenttool", nil)
	c.Params = gin.Params{{Key: "tool", Value: "nonexistenttool"}}

	handler.ListAgentsByTool(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AgentListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 0, response.Count)
	assert.Empty(t, response.Agents)
}

// Test AgentResponse struct fields
func TestAgentResponse_Fields(t *testing.T) {
	resp := AgentResponse{
		Name:           "TestAgent",
		Description:    "A test agent",
		Language:       "Go",
		ConfigFormat:   "YAML",
		APIPattern:     "REST",
		EntryPoint:     "main.go",
		Features:       []string{"feature1", "feature2"},
		ToolSupport:    []string{"bash", "read"},
		Protocols:      []string{"MCP", "LSP"},
		ConfigLocation: "~/.config/test",
		EnvVars:        map[string]string{"KEY": "value"},
		SystemPrompt:   "You are a test agent",
	}

	assert.Equal(t, "TestAgent", resp.Name)
	assert.Equal(t, "A test agent", resp.Description)
	assert.Equal(t, "Go", resp.Language)
	assert.Equal(t, "YAML", resp.ConfigFormat)
	assert.Equal(t, "REST", resp.APIPattern)
	assert.Len(t, resp.Features, 2)
	assert.Len(t, resp.ToolSupport, 2)
	assert.Len(t, resp.Protocols, 2)
	assert.Equal(t, "value", resp.EnvVars["KEY"])
}

func TestAgentListResponse_Fields(t *testing.T) {
	resp := AgentListResponse{
		Agents: []AgentResponse{
			{Name: "Agent1"},
			{Name: "Agent2"},
		},
		Count: 2,
	}

	assert.Equal(t, 2, resp.Count)
	assert.Len(t, resp.Agents, 2)
	assert.Equal(t, "Agent1", resp.Agents[0].Name)
	assert.Equal(t, "Agent2", resp.Agents[1].Name)
}

func TestAgentResponse_JSONSerialization(t *testing.T) {
	resp := AgentResponse{
		Name:        "TestAgent",
		Description: "Test description",
		Language:    "Go",
		EnvVars:     map[string]string{"API_KEY": "secret"},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded AgentResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Name, decoded.Name)
	assert.Equal(t, resp.Description, decoded.Description)
	assert.Equal(t, "secret", decoded.EnvVars["API_KEY"])
}
