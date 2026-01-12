package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"dev.helix.agent/internal/agents"
)

// AgentHandler handles requests for CLI agent registry
type AgentHandler struct{}

// NewAgentHandler creates a new agent handler
func NewAgentHandler() *AgentHandler {
	return &AgentHandler{}
}

// AgentResponse represents a single agent in the response
type AgentResponse struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Language       string            `json:"language"`
	ConfigFormat   string            `json:"config_format"`
	APIPattern     string            `json:"api_pattern"`
	EntryPoint     string            `json:"entry_point"`
	Features       []string          `json:"features"`
	ToolSupport    []string          `json:"tool_support"`
	Protocols      []string          `json:"protocols"`
	ConfigLocation string            `json:"config_location"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	SystemPrompt   string            `json:"system_prompt"`
}

// AgentListResponse represents the list of all agents
type AgentListResponse struct {
	Agents []AgentResponse `json:"agents"`
	Count  int             `json:"count"`
}

// ListAgents returns all registered CLI agents
// GET /v1/agents
func (h *AgentHandler) ListAgents(c *gin.Context) {
	allAgents := agents.GetAllAgents()

	response := AgentListResponse{
		Agents: make([]AgentResponse, 0, len(allAgents)),
		Count:  len(allAgents),
	}

	for _, agent := range allAgents {
		response.Agents = append(response.Agents, AgentResponse{
			Name:           agent.Name,
			Description:    agent.Description,
			Language:       agent.Language,
			ConfigFormat:   agent.ConfigFormat,
			APIPattern:     agent.APIPattern,
			EntryPoint:     agent.EntryPoint,
			Features:       agent.Features,
			ToolSupport:    agent.ToolSupport,
			Protocols:      agent.Protocols,
			ConfigLocation: agent.ConfigLocation,
			EnvVars:        agent.EnvVars,
			SystemPrompt:   agent.SystemPrompt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// GetAgent returns a specific CLI agent by name
// GET /v1/agents/:name
func (h *AgentHandler) GetAgent(c *gin.Context) {
	name := c.Param("name")

	agent, found := agents.GetAgent(name)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "agent_not_found",
			"message": "Agent not found: " + name,
		})
		return
	}

	c.JSON(http.StatusOK, AgentResponse{
		Name:           agent.Name,
		Description:    agent.Description,
		Language:       agent.Language,
		ConfigFormat:   agent.ConfigFormat,
		APIPattern:     agent.APIPattern,
		EntryPoint:     agent.EntryPoint,
		Features:       agent.Features,
		ToolSupport:    agent.ToolSupport,
		Protocols:      agent.Protocols,
		ConfigLocation: agent.ConfigLocation,
		EnvVars:        agent.EnvVars,
		SystemPrompt:   agent.SystemPrompt,
	})
}

// ListAgentsByProtocol returns agents that support a specific protocol
// GET /v1/agents/protocol/:protocol
func (h *AgentHandler) ListAgentsByProtocol(c *gin.Context) {
	protocol := c.Param("protocol")

	matchingAgents := agents.GetAgentsByProtocol(protocol)

	response := AgentListResponse{
		Agents: make([]AgentResponse, 0, len(matchingAgents)),
		Count:  len(matchingAgents),
	}

	for _, agent := range matchingAgents {
		response.Agents = append(response.Agents, AgentResponse{
			Name:           agent.Name,
			Description:    agent.Description,
			Language:       agent.Language,
			ConfigFormat:   agent.ConfigFormat,
			APIPattern:     agent.APIPattern,
			EntryPoint:     agent.EntryPoint,
			Features:       agent.Features,
			ToolSupport:    agent.ToolSupport,
			Protocols:      agent.Protocols,
			ConfigLocation: agent.ConfigLocation,
			EnvVars:        agent.EnvVars,
			SystemPrompt:   agent.SystemPrompt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// ListAgentsByTool returns agents that support a specific tool
// GET /v1/agents/tool/:tool
func (h *AgentHandler) ListAgentsByTool(c *gin.Context) {
	tool := c.Param("tool")

	matchingAgents := agents.GetAgentsByTool(tool)

	response := AgentListResponse{
		Agents: make([]AgentResponse, 0, len(matchingAgents)),
		Count:  len(matchingAgents),
	}

	for _, agent := range matchingAgents {
		response.Agents = append(response.Agents, AgentResponse{
			Name:           agent.Name,
			Description:    agent.Description,
			Language:       agent.Language,
			ConfigFormat:   agent.ConfigFormat,
			APIPattern:     agent.APIPattern,
			EntryPoint:     agent.EntryPoint,
			Features:       agent.Features,
			ToolSupport:    agent.ToolSupport,
			Protocols:      agent.Protocols,
			ConfigLocation: agent.ConfigLocation,
			EnvVars:        agent.EnvVars,
			SystemPrompt:   agent.SystemPrompt,
		})
	}

	c.JSON(http.StatusOK, response)
}
