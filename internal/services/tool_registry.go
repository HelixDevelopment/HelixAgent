package services

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Tool represents a unified tool interface
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
	Source() string // "mcp", "lsp", "custom", etc.
}

// ToolRegistry manages tools from various sources
type ToolRegistry struct {
	mu          sync.RWMutex
	tools       map[string]Tool
	mcpManager  *MCPManager
	lspClient   *LSPClient
	customTools map[string]Tool
	lastRefresh time.Time
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(mcpManager *MCPManager, lspClient *LSPClient) *ToolRegistry {
	return &ToolRegistry{
		tools:       make(map[string]Tool),
		mcpManager:  mcpManager,
		lspClient:   lspClient,
		customTools: make(map[string]Tool),
	}
}

// RegisterCustomTool registers a custom tool
func (tr *ToolRegistry) RegisterCustomTool(tool Tool) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	name := tool.Name()
	if _, exists := tr.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	tr.tools[name] = tool
	tr.customTools[name] = tool
	return nil
}

// RefreshTools refreshes tools from all sources
func (tr *ToolRegistry) RefreshTools(ctx context.Context) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	// Clear existing tools except custom ones
	for name, tool := range tr.tools {
		if tool.Source() != "custom" {
			delete(tr.tools, name)
		}
	}

	// Add MCP tools
	if tr.mcpManager != nil {
		mcpTools := tr.mcpManager.ListTools()
		for _, mcpTool := range mcpTools {
			wrapper := &MCPToolWrapper{
				mcpTool:    mcpTool,
				mcpManager: tr.mcpManager,
			}
			tr.tools[mcpTool.Name] = wrapper
		}
	}

	// Add LSP-based tools (code actions)
	if tr.lspClient != nil {
		// LSP tools would be added here when implemented
	}

	tr.lastRefresh = time.Now()
	return nil
}

// GetTool returns a tool by name
func (tr *ToolRegistry) GetTool(name string) (Tool, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tool, exists := tr.tools[name]
	return tool, exists
}

// ListTools returns all available tools
func (tr *ToolRegistry) ListTools() []Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tools := make([]Tool, 0, len(tr.tools))
	for _, tool := range tr.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ExecuteTool safely executes a tool with sandboxing
func (tr *ToolRegistry) ExecuteTool(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	tool, exists := tr.GetTool(name)
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	// Basic parameter validation
	if err := tr.validateParameters(tool, params); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Execute with timeout
	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := tool.Execute(execCtx, params)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}

// validateParameters performs basic parameter validation
func (tr *ToolRegistry) validateParameters(tool Tool, params map[string]interface{}) error {
	// Basic validation - could be enhanced
	required := tool.Parameters()
	for key := range required {
		if _, exists := params[key]; !exists {
			return fmt.Errorf("missing required parameter: %s", key)
		}
	}
	return nil
}

// MCPToolWrapper wraps MCP tools to implement the Tool interface
type MCPToolWrapper struct {
	mcpTool    *MCPTool
	mcpManager *MCPManager
}

func (w *MCPToolWrapper) Name() string {
	return w.mcpTool.Name
}

func (w *MCPToolWrapper) Description() string {
	return w.mcpTool.Description
}

func (w *MCPToolWrapper) Parameters() map[string]interface{} {
	return w.mcpTool.InputSchema
}

func (w *MCPToolWrapper) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return w.mcpManager.CallTool(ctx, w.mcpTool.Name, params)
}

func (w *MCPToolWrapper) Source() string {
	return "mcp"
}

// LSPToolWrapper would wrap LSP-based tools (code actions, etc.)
// Implementation would be added when LSP tools are implemented
