// Package api provides MCP (Model Context Protocol) Proxy API implementation.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// MCPProxyAPI provides access to Anthropic's MCP proxy servers
type MCPProxyAPI struct {
	client  *Client
	baseURL string
}

// NewMCPProxyAPI creates a new MCP proxy API client
func NewMCPProxyAPI(client *Client) *MCPProxyAPI {
	return &MCPProxyAPI{
		client:  client,
		baseURL: MCPProxyBaseURL,
	}
}

// WithBaseURL sets a custom MCP proxy base URL
func (m *MCPProxyAPI) WithBaseURL(url string) *MCPProxyAPI {
	m.baseURL = url
	return m
}

// MCPServer represents an MCP server configuration
type MCPServer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Tools       []MCPTool         `json:"tools"`
	Resources   []MCPResource     `json:"resources,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
}

// MCPTool represents a tool provided by an MCP server
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// MCPResource represents a resource provided by an MCP server
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPCallRequest represents a call to an MCP tool
type MCPCallRequest struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

// MCPCallResponse represents the response from an MCP tool call
type MCPCallResponse struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent represents content returned by an MCP tool
type MCPContent struct {
	Type string `json:"type"` // "text", "image", "resource"

	// For text type
	Text string `json:"text,omitempty"`

	// For image type
	Image *MCPImageContent `json:"image,omitempty"`

	// For resource type
	Resource *MCPResourceContent `json:"resource,omitempty"`
}

// MCPImageContent represents image content
type MCPImageContent struct {
	Data     string `json:"data"` // base64
	MimeType string `json:"mimeType"`
}

// MCPResourceContent represents embedded resource content
type MCPResourceContent struct {
	URI  string `json:"uri"`
	Text string `json:"text,omitempty"`
	Blob string `json:"blob,omitempty"` // base64
}

// CallTool calls a tool on an MCP server
func (m *MCPProxyAPI) CallTool(ctx context.Context, serverID string, req *MCPCallRequest) (*MCPCallResponse, error) {
	path := fmt.Sprintf("/v1/mcp/%s", serverID)

	// Use the base client but override the base URL temporarily
	resp, err := m.doMCPRequest(ctx, "POST", path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result MCPCallResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListServers lists available MCP servers
func (m *MCPProxyAPI) ListServers(ctx context.Context) ([]MCPServer, error) {
	resp, err := m.doMCPRequest(ctx, "GET", "/v1/mcp", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result struct {
		Servers []MCPServer `json:"servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Servers, nil
}

// GetServer gets information about a specific MCP server
func (m *MCPProxyAPI) GetServer(ctx context.Context, serverID string) (*MCPServer, error) {
	path := fmt.Sprintf("/v1/mcp/%s", serverID)

	resp, err := m.doMCPRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result MCPServer
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListResources lists resources available from an MCP server
func (m *MCPProxyAPI) ListResources(ctx context.Context, serverID string) ([]MCPResource, error) {
	path := fmt.Sprintf("/v1/mcp/%s/resources", serverID)

	resp, err := m.doMCPRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result struct {
		Resources []MCPResource `json:"resources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Resources, nil
}

// ReadResource reads a resource from an MCP server
func (m *MCPProxyAPI) ReadResource(ctx context.Context, serverID, resourceURI string) (*MCPResourceContent, error) {
	path := fmt.Sprintf("/v1/mcp/%s/resources/read", serverID)

	req := struct {
		URI string `json:"uri"`
	}{
		URI: resourceURI,
	}

	resp, err := m.doMCPRequest(ctx, "POST", path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result MCPResourceContent
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// doMCPRequest performs an MCP proxy request
func (m *MCPProxyAPI) doMCPRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := m.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set auth headers from the main client
	m.client.setAuthHeaders(&req.Header)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return m.client.httpClient.Do(req)
}

// Predefined MCP Server IDs
const (
	MCPServerGitHub             = "github"
	MCPServerFilesystem         = "filesystem"
	MCPServerWebSearch          = "web-search"
	MCPServerBraveSearch        = "brave-search"
	MCPServerFetch              = "fetch"
	MCPServerMemory             = "memory"
	MCPServerSequentialThinking = "sequential-thinking"
)

// Common MCP Tools
const (
	MCPToolGitHubSearchRepos = "search_repositories"
	MCPToolGitHubGetFile     = "get_file_contents"
	MCPToolGitHubCreateIssue = "create_issue"
	MCPToolGitHubListPRs     = "list_pull_requests"
	MCPToolFilesystemRead    = "read_file"
	MCPToolFilesystemWrite   = "write_file"
	MCPToolFilesystemList    = "list_directory"
	MCPToolWebSearchSearch   = "search"
	MCPToolFetchURL          = "fetch_url"
	MCPToolMemoryRemember    = "remember"
	MCPToolMemoryRecall      = "recall"
	MCPToolSequentialThink   = "think"
)

// MCPManager manages multiple MCP server connections
type MCPManager struct {
	api     *MCPProxyAPI
	servers map[string]*MCPServer
}

// NewMCPManager creates a new MCP manager
func NewMCPManager(api *MCPProxyAPI) *MCPManager {
	return &MCPManager{
		api:     api,
		servers: make(map[string]*MCPServer),
	}
}

// DiscoverServers discovers and caches available MCP servers
func (mm *MCPManager) DiscoverServers(ctx context.Context) error {
	servers, err := mm.api.ListServers(ctx)
	if err != nil {
		return err
	}

	for _, server := range servers {
		mm.servers[server.ID] = &server
	}

	return nil
}

// GetServer gets a cached server by ID
func (mm *MCPManager) GetServer(serverID string) (*MCPServer, bool) {
	server, ok := mm.servers[serverID]
	return server, ok
}

// HasTool checks if a tool is available on a server
func (mm *MCPManager) HasTool(serverID, toolName string) bool {
	server, ok := mm.servers[serverID]
	if !ok {
		return false
	}

	for _, tool := range server.Tools {
		if tool.Name == toolName {
			return true
		}
	}

	return false
}

// Call calls an MCP tool by server and tool name
func (mm *MCPManager) Call(ctx context.Context, serverID, toolName string, params map[string]interface{}) (*MCPCallResponse, error) {
	req := &MCPCallRequest{
		Tool:   toolName,
		Params: params,
	}

	return mm.api.CallTool(ctx, serverID, req)
}

// ListAllTools returns all available tools across all servers
func (mm *MCPManager) ListAllTools() map[string][]MCPTool {
	result := make(map[string][]MCPTool)

	for id, server := range mm.servers {
		result[id] = server.Tools
	}

	return result
}

// FindTool finds a tool by name across all servers
func (mm *MCPManager) FindTool(toolName string) (serverID string, tool *MCPTool, found bool) {
	for id, server := range mm.servers {
		for i := range server.Tools {
			if server.Tools[i].Name == toolName {
				return id, &server.Tools[i], true
			}
		}
	}

	return "", nil, false
}
