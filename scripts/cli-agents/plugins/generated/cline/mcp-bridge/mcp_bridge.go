// MCP Bridge Plugin
// Bridges MCP protocol between CLI agent and HelixAgent

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var PluginName = "mcp-bridge"
var PluginVersion = "1.0.0"

// MCPRequest represents an MCP request
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   *MCPError              `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPBridge handles MCP protocol bridging
type MCPBridge struct {
	helixURL   string
	httpClient *http.Client
}

// NewMCPBridge creates a new MCP bridge
func NewMCPBridge(helixURL string) *MCPBridge {
	return &MCPBridge{
		helixURL: helixURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Forward forwards an MCP request to HelixAgent
func (b *MCPBridge) Forward(ctx context.Context, req *MCPRequest) (*MCPResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := b.helixURL + "/v1/mcp"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url,
		strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var mcpResp MCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, err
	}

	return &mcpResp, nil
}

// ListTools lists available MCP tools
func (b *MCPBridge) ListTools(ctx context.Context) ([]string, error) {
	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	resp, err := b.Forward(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", resp.Error.Message)
	}

	tools, ok := resp.Result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools response")
	}

	var names []string
	for _, t := range tools {
		if tool, ok := t.(map[string]interface{}); ok {
			if name, ok := tool["name"].(string); ok {
				names = append(names, name)
			}
		}
	}

	return names, nil
}

func Init() error {
	fmt.Println("[mcp-bridge] Plugin initialized")
	return nil
}

func Shutdown() error {
	fmt.Println("[mcp-bridge] Plugin shutdown")
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("%s v%s\n", PluginName, PluginVersion)
		return
	}
	fmt.Println("MCP Bridge Plugin")
}
