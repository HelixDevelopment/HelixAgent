// Package claude_code provides MCP integration for Claude Code.
package claude_code

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/services"
)

// MCPIntegration provides MCP server integration for Claude Code
type MCPIntegration struct {
	enabled    bool
	configPath string
	servers    map[string]*MCPServer
}

// MCPServer represents an MCP server configuration
type MCPServer struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled"`
	Tools   []MCPTool         `json:"tools,omitempty"`
}

// MCPTool represents an available MCP tool
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// NewMCPIntegration creates a new MCP integration
func NewMCPIntegration(configPath string) *MCPIntegration {
	return &MCPIntegration{
		enabled:    configPath != "",
		configPath: configPath,
		servers:    make(map[string]*MCPServer),
	}
}

// LoadConfig loads MCP server configuration from file
func (m *MCPIntegration) LoadConfig() error {
	if m.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config
			return m.createDefaultConfig()
		}
		return err
	}

	var config struct {
		MCPServers map[string]*MCPServer `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	m.servers = config.MCPServers
	return nil
}

// createDefaultConfig creates a default MCP configuration
func (m *MCPIntegration) createDefaultConfig() error {
	// Create default servers
	m.servers = map[string]*MCPServer{
		"filesystem": {
			Name:    "filesystem",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/home/user"},
			Enabled: true,
		},
		"github": {
			Name:    "github",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-github"},
			Enabled: true,
		},
		"memory": {
			Name:    "memory",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-memory"},
			Enabled: true,
		},
		"fetch": {
			Name:    "fetch",
			Command: "uvx",
			Args:    []string{"mcp-server-fetch"},
			Enabled: true,
		},
	}

	defaultConfig := map[string]interface{}{
		"mcpServers": m.servers,
	}

	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// GetServers returns all configured MCP servers
func (m *MCPIntegration) GetServers() map[string]*MCPServer {
	return m.servers
}

// GetServer returns a specific MCP server
func (m *MCPIntegration) GetServer(name string) (*MCPServer, bool) {
	server, ok := m.servers[name]
	return server, ok
}

// IsEnabled returns whether MCP is enabled
func (m *MCPIntegration) IsEnabled() bool {
	return m.enabled && len(m.servers) > 0
}

// ListTools returns all available tools from all servers
func (m *MCPIntegration) ListTools(ctx context.Context) ([]MCPTool, error) {
	if !m.IsEnabled() {
		return nil, nil
	}

	var allTools []MCPTool
	for _, server := range m.servers {
		if server.Enabled {
			allTools = append(allTools, server.Tools...)
		}
	}

	return allTools, nil
}

// CallTool calls an MCP tool on a specific server
func (m *MCPIntegration) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (*services.ToolCallResult, error) {
	server, ok := m.servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	if !server.Enabled {
		return nil, fmt.Errorf("server is disabled: %s", serverName)
	}

	// In a full implementation, this would:
	// 1. Connect to the MCP server via stdio
	// 2. Send the tool call request
	// 3. Return the result

	// For now, return a simulated result
	return &services.ToolCallResult{
		Content: []services.Content{
			{
				Type: "text",
				Text: fmt.Sprintf("Called %s/%s with args: %v", serverName, toolName, args),
			},
		},
		IsError: false,
	}, nil
}

// AddServer adds a new MCP server
func (m *MCPIntegration) AddServer(server *MCPServer) error {
	if server.Name == "" {
		return fmt.Errorf("server name is required")
	}

	m.servers[server.Name] = server
	return m.saveConfig()
}

// RemoveServer removes an MCP server
func (m *MCPIntegration) RemoveServer(name string) error {
	delete(m.servers, name)
	return m.saveConfig()
}

// EnableServer enables/disables a server
func (m *MCPIntegration) EnableServer(name string, enabled bool) error {
	if server, ok := m.servers[name]; ok {
		server.Enabled = enabled
		return m.saveConfig()
	}
	return fmt.Errorf("server not found: %s", name)
}

// saveConfig saves the current configuration
func (m *MCPIntegration) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	config := map[string]interface{}{
		"mcpServers": m.servers,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// DefaultMCPConfigPath returns the default MCP config path
func DefaultMCPConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "mcp.json")
}
