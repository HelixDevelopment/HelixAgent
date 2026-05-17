// Package claude_code provides MCP integration for Claude Code.
package claude_code

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/services"
)

// ErrMCPClientNotWired is returned by (*MCPIntegration).CallTool
// when no MCP JSON-RPC transport client has been wired into the
// integration. Forensic anchor (round-29 §11.4 audit): prior to the
// fix, CallTool fabricated a "simulated result" that echoed the
// caller's arguments back as text content with IsError=false. Any
// caller (an LLM tool-call dispatcher, an agent loop, a test) that
// believed the result was a real MCP server response was deceived —
// the actual tool was never invoked, no filesystem read happened,
// no GitHub API was hit, nothing. CRITICAL §11.4 contract-bluff at
// the agent/MCP boundary. Wire a transport via
// (*MCPIntegration).SetTransport before invoking CallTool.
var ErrMCPClientNotWired = errors.New("claude_code/mcp: MCP JSON-RPC transport has not been wired — CallTool previously returned a fabricated 'simulated result' echoing the caller's arguments back as text with IsError=false (CRITICAL §11.4 contract-bluff at the agent/MCP boundary). Wire a transport via (*MCPIntegration).SetTransport before invoking CallTool")

// MCPTransport is the wiring contract for a real MCP JSON-RPC
// transport (stdio, HTTP, websocket). Production wires an
// implementation that connects to the named MCP server, sends a
// `tools/call` request with the supplied arguments, and returns the
// decoded ToolCallResult. Unit tests under CONST-050(A) MAY supply
// a deterministic stub. CallTool returns ErrMCPClientNotWired when
// no transport has been installed.
type MCPTransport interface {
	// CallTool sends a `tools/call` JSON-RPC request to the given
	// MCP server using the server's transport (command/args/env
	// already configured on MCPServer) and returns the result. The
	// implementation MUST NOT fabricate content; on transport
	// failure it MUST return a non-nil error.
	CallTool(ctx context.Context, server *MCPServer, toolName string, args map[string]interface{}) (*services.ToolCallResult, error)
}

// MCPIntegration provides MCP server integration for Claude Code
type MCPIntegration struct {
	enabled    bool
	configPath string
	servers    map[string]*MCPServer
	// transport is the round-29 anti-bluff injection point. nil =
	// CallTool returns ErrMCPClientNotWired. Production wires a
	// real transport; tests MAY install a deterministic stub via
	// SetTransport.
	transport MCPTransport
}

// SetTransport installs the MCP JSON-RPC transport used by
// CallTool. Round-29 anti-bluff fix: production MUST call this with
// a real transport before invoking CallTool; otherwise CallTool
// surfaces ErrMCPClientNotWired instead of fabricating a result.
func (m *MCPIntegration) SetTransport(t MCPTransport) {
	m.transport = t
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

// CallTool dispatches a real MCP `tools/call` request to the named
// server via the injected MCPTransport and returns the server's
// result.
//
// Round-29 anti-bluff fix: when no transport has been wired in,
// CallTool returns ErrMCPClientNotWired (instead of the prior
// fabricated "simulated result" that echoed the caller's arguments
// back as a text content item with IsError=false). Wire a
// transport via (*MCPIntegration).SetTransport before invoking
// CallTool in production.
func (m *MCPIntegration) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (*services.ToolCallResult, error) {
	server, ok := m.servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	if !server.Enabled {
		return nil, fmt.Errorf("server is disabled: %s", serverName)
	}

	if m.transport == nil {
		return nil, fmt.Errorf("CallTool(server=%q, tool=%q): %w", serverName, toolName, ErrMCPClientNotWired)
	}

	return m.transport.CallTool(ctx, server, toolName, args)
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
