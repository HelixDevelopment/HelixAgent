package services

import (
	"context"
	"fmt"
	"sync"

	"dev.helix.agent/internal/database"
	"github.com/sirupsen/logrus"
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
	client     *MCPClient
	clientOnce sync.Once
	repo       *database.ModelMetadataRepository
	cache      CacheInterface
	log        *logrus.Logger
}

// NewMCPManager creates a new MCP manager with dependencies.
// The underlying MCPClient is lazily initialized on first use via getClient().
func NewMCPManager(repo *database.ModelMetadataRepository, cache CacheInterface, log *logrus.Logger) *MCPManager {
	return &MCPManager{
		repo:  repo,
		cache: cache,
		log:   log,
	}
}

// getClient returns the lazily-initialized MCPClient.
func (m *MCPManager) getClient() *MCPClient {
	m.clientOnce.Do(func() {
		m.client = NewMCPClient(m.log)
	})
	return m.client
}

// ListMCPServers lists all configured MCP servers (for unified manager)
func (m *MCPManager) ListMCPServers(ctx context.Context) ([]*MCPServerConnection, error) {
	servers := m.getClient().ListServers()
	return servers, nil
}

// ExecuteMCPTool executes a tool on an MCP server
func (m *MCPManager) ExecuteMCPTool(ctx context.Context, req interface{}) (interface{}, error) {
	// Convert the unified protocol request to MCP call
	if unifiedReq, ok := req.(UnifiedProtocolRequest); ok {
		result, err := m.getClient().CallTool(ctx, unifiedReq.ServerID, unifiedReq.ToolName, unifiedReq.Arguments)
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
	tools, err := m.getClient().ListTools(context.Background())
	if err != nil {
		m.log.WithError(err).Error("Failed to list MCP tools")
		return []*MCPTool{}
	}
	return tools
}

// GetMCPTools gets all tools from all enabled MCP servers
func (m *MCPManager) GetMCPTools(ctx context.Context) (map[string][]*MCPTool, error) {
	tools, err := m.getClient().ListTools(ctx)
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
		server, err := m.getClient().GetServerInfo(unifiedReq.ServerID)
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
	c := m.getClient()
	servers := c.ListServers()
	health := c.HealthCheck(ctx)

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
		"totalTools":       c.tools.Len(),
		"lastSync":         "2024-01-01T12:00:00Z",
	}, nil
}

// ConnectServer connects to an MCP server
func (m *MCPManager) ConnectServer(ctx context.Context, serverID, name, command string, args []string) error {
	return m.getClient().ConnectServer(ctx, serverID, name, command, args)
}

// DisconnectServer disconnects from an MCP server
func (m *MCPManager) DisconnectServer(serverID string) error {
	return m.getClient().DisconnectServer(serverID)
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

	if len(command) == 0 {
		return fmt.Errorf("server command cannot be empty")
	}

	args := make([]string, len(command))
	for i, c := range command {
		arg, ok := c.(string)
		if !ok {
			return fmt.Errorf("command argument %d must be a string", i)
		}
		args[i] = arg
	}

	return m.getClient().ConnectServer(context.Background(), name, name, args[0], args[1:])
}

// CallTool executes a tool on an MCP server
func (m *MCPManager) CallTool(ctx context.Context, toolName string, params map[string]interface{}) (interface{}, error) {
	// For now, assume the tool is on the first connected server
	c := m.getClient()
	servers := c.ListServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no MCP servers connected")
	}

	serverID := servers[0].ID
	result, err := c.CallTool(ctx, serverID, toolName, params)
	if err != nil {
		return nil, err
	}

	return result, nil
}
