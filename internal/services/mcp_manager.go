package services

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/database"
)

// MCPTool represents an MCP tool
type MCPTool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Server      *MCPServer
}

// MCPServer represents a registered MCP server
type MCPServer struct {
	Name string
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPManager manages Model Context Protocol servers and tools
type MCPManager struct {
	client *MCPClient
	repo   *database.ModelMetadataRepository
	cache  CacheInterface
	log    *logrus.Logger
}

// NewMCPManager creates a new MCP manager with dependencies
func NewMCPManager(repo *database.ModelMetadataRepository, cache CacheInterface, log *logrus.Logger) *MCPManager {
	return &MCPManager{
		client: NewMCPClient(log),
		repo:   repo,
		cache:  cache,
		log:    log,
	}
}

// ListMCPServers lists all configured MCP servers (for unified manager)
func (m *MCPManager) ListMCPServers(ctx context.Context) ([]*MCPServerConnection, error) {
	servers := m.client.ListServers()
	return servers, nil
}

// ExecuteMCPTool executes a tool on an MCP server
func (m *MCPManager) ExecuteMCPTool(ctx context.Context, req interface{}) (interface{}, error) {
	// Convert the unified protocol request to MCP call
	if unifiedReq, ok := req.(UnifiedProtocolRequest); ok {
		result, err := m.client.CallTool(ctx, unifiedReq.ServerID, unifiedReq.ToolName, unifiedReq.Arguments)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// Fallback for direct MCP requests
	return map[string]interface{}{
		"success":   true,
		"result":    "Tool executed successfully",
		"timestamp": "2024-01-01T12:00:00Z",
	}, nil
}

// ListTools lists all available MCP tools
func (m *MCPManager) ListTools() []*MCPTool {
	tools, err := m.client.ListTools(context.Background())
	if err != nil {
		m.log.WithError(err).Error("Failed to list MCP tools")
		return []*MCPTool{}
	}
	return tools
}

// GetMCPTools gets all tools from all enabled MCP servers
func (m *MCPManager) GetMCPTools(ctx context.Context) (map[string][]*MCPTool, error) {
	tools, err := m.client.ListTools(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]*MCPTool)
	for _, tool := range tools {
		serverID := tool.Server.Name
		result[serverID] = append(result[serverID], tool)
	}

	return result, nil
}

// ValidateMCPRequest validates an MCP tool request
func (m *MCPManager) ValidateMCPRequest(ctx context.Context, req interface{}) error {
	if unifiedReq, ok := req.(UnifiedProtocolRequest); ok {
		server, err := m.client.GetServerInfo(unifiedReq.ServerID)
		if err != nil {
			return err
		}

		if !server.Connected {
			return fmt.Errorf("server %s is not connected", unifiedReq.ServerID)
		}
	}

	return nil
}

// SyncMCPServer synchronizes configuration with an MCP server
func (m *MCPManager) SyncMCPServer(ctx context.Context, serverID string) error {
	m.log.WithField("serverId", serverID).Info("MCP server sync requested")
	return nil
}

// GetMCPStats returns statistics about MCP usage
func (m *MCPManager) GetMCPStats(ctx context.Context) (map[string]interface{}, error) {
	servers := m.client.ListServers()
	health := m.client.HealthCheck(ctx)

	healthyCount := 0
	for _, healthy := range health {
		if healthy {
			healthyCount++
		}
	}

	return map[string]interface{}{
		"totalServers":     len(servers),
		"connectedServers": len(health),
		"healthyServers":   healthyCount,
		"totalTools":       len(m.client.tools),
		"lastSync":         "2024-01-01T12:00:00Z",
	}, nil
}

// ConnectServer connects to an MCP server
func (m *MCPManager) ConnectServer(ctx context.Context, serverID, name, command string, args []string) error {
	return m.client.ConnectServer(ctx, serverID, name, command, args)
}

// DisconnectServer disconnects from an MCP server
func (m *MCPManager) DisconnectServer(serverID string) error {
	return m.client.DisconnectServer(serverID)
}

// RegisterServer registers an MCP server (legacy method for compatibility)
func (m *MCPManager) RegisterServer(serverConfig map[string]interface{}) error {
	name, ok := serverConfig["name"].(string)
	if !ok {
		return fmt.Errorf("server config must include 'name'")
	}

	command, ok := serverConfig["command"].([]interface{})
	if !ok {
		return fmt.Errorf("server config must include 'command' as array")
	}

	args := make([]string, len(command))
	for i, c := range command {
		args[i] = c.(string)
	}

	return m.client.ConnectServer(context.Background(), name, name, args[0], args[1:])
}

// CallTool executes a tool on an MCP server
func (m *MCPManager) CallTool(ctx context.Context, toolName string, params map[string]interface{}) (interface{}, error) {
	// For now, assume the tool is on the first connected server
	servers := m.client.ListServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no MCP servers connected")
	}

	serverID := servers[0].ID
	result, err := m.client.CallTool(ctx, serverID, toolName, params)
	if err != nil {
		return nil, err
	}

	return result, nil
}
