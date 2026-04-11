// Package services provides complete MCP (Model Context Protocol) types
// Compatible with protocol version 2024-11-05
package services

import (
	"encoding/json"
	"fmt"
)

// MCP Protocol Version
const (
	MCPProtocolVersion = "2024-11-05"
)

// ============================================================================
// JSON-RPC Base Types
// ============================================================================

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements the error interface
func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC Error %d: %s", e.Code, e.Message)
}

// Standard JSON-RPC error codes
const (
	JSONRPCParseError     = -32700
	JSONRPCInvalidRequest = -32600
	JSONRPCMethodNotFound = -32601
	JSONRPCInvalidParams  = -32602
	JSONRPCInternalError  = -32603
	JSONRPCServerError    = -32000
)

// JSONRPCNotification represents a JSON-RPC 2.0 notification (no ID)
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// ============================================================================
// MCP Core Types
// ============================================================================

// Implementation describes the name and version of an MCP implementation
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Capabilities represents capabilities that can be supported by client or server
type Capabilities struct {
	// Experimental features
	Experimental map[string]interface{} `json:"experimental,omitempty"`

	// Tool capabilities
	Tools *ToolCapabilities `json:"tools,omitempty"`

	// Resource capabilities
	Resources *ResourceCapabilities `json:"resources,omitempty"`

	// Prompt capabilities
	Prompts *PromptCapabilities `json:"prompts,omitempty"`

	// Logging capabilities
	Logging *LoggingCapabilities `json:"logging,omitempty"`
}

// ToolCapabilities represents capabilities for tools
type ToolCapabilities struct {
	// Whether the tool supports list changes notifications
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourceCapabilities represents capabilities for resources
type ResourceCapabilities struct {
	// Whether the resource supports subscribing to changes
	Subscribe bool `json:"subscribe,omitempty"`

	// Whether the resource supports list changes notifications
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptCapabilities represents capabilities for prompts
type PromptCapabilities struct {
	// Whether the prompt supports list changes notifications
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapabilities represents capabilities for logging
type LoggingCapabilities struct{}

// ============================================================================
// Lifecycle Types
// ============================================================================

// MCPInitializeRequest is sent from client to server during initialization
// This is a unique type name to avoid conflict with ACP's InitializeRequest
type MCPInitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]string      `json:"clientInfo"`
}

// MCPInitializeResult is the server's response to initialize
// This is a unique type name to avoid conflict with ACP's InitializeResult
type MCPInitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      map[string]string      `json:"serverInfo"`
	Instructions    string                 `json:"instructions,omitempty"`
}

// InitializedNotification is sent from client to server after initialization
type InitializedNotification struct{}

// ============================================================================
// Tool Types
// ============================================================================

// MCPToolDefinition represents a tool that the server provides
// This is a unique name to avoid conflict with other Tool types
type MCPToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ToolInputSchema represents the JSON Schema for tool input
type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// ListToolsRequest requests a list of available tools
type ListToolsRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListToolsResult is the server's response to tools/list
type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ToolCallRequest calls a specific tool
type ToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolCallResult is the result of calling a tool
type ToolCallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents content returned from tool calls
type Content struct {
	Type string `json:"type"`

	// For text content
	Text string `json:"text,omitempty"`

	// For image content
	Image *ImageContent `json:"image,omitempty"`

	// For embedded resource
	Resource *EmbeddedResource `json:"resource,omitempty"`
}

// ImageContent represents image data
type ImageContent struct {
	Data     string `json:"data"` // base64 encoded
	MimeType string `json:"mimeType"`
}

// EmbeddedResource represents an embedded resource
type EmbeddedResource struct {
	Type string          `json:"type"`
	Text string          `json:"text,omitempty"`
	Blob string          `json:"blob,omitempty"` // base64 encoded
	URI  string          `json:"uri,omitempty"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

// ToolListChangedNotification notifies that tool list has changed
type ToolListChangedNotification struct{}

// ============================================================================
// Resource Types
// ============================================================================

// Resource represents a known resource that the server can read
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContents represents the contents of a resource
type ResourceContents struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`

	// For text resources
	Text string `json:"text,omitempty"`

	// For binary resources
	Blob string `json:"blob,omitempty"` // base64 encoded
}

// ListResourcesRequest requests a list of available resources
type ListResourcesRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListResourcesResult is the server's response to resources/list
type ListResourcesResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

// ReadResourceRequest reads a specific resource
type ReadResourceRequest struct {
	URI string `json:"uri"`
}

// ReadResourceResult is the result of reading a resource
type ReadResourceResult struct {
	Contents []ResourceContents `json:"contents"`
}

// SubscribeRequest subscribes to resource updates
type SubscribeRequest struct {
	URI string `json:"uri"`
}

// UnsubscribeRequest unsubscribes from resource updates
type UnsubscribeRequest struct {
	URI string `json:"uri"`
}

// ResourceUpdatedNotification notifies that a resource has been updated
type ResourceUpdatedNotification struct {
	URI string `json:"uri"`
}

// ResourceListChangedNotification notifies that resource list has changed
type ResourceListChangedNotification struct{}

// ============================================================================
// Prompt Types
// ============================================================================

// Prompt represents a prompt template
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// ListPromptsRequest requests a list of available prompts
type ListPromptsRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListPromptsResult is the server's response to prompts/list
type ListPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// GetPromptRequest retrieves a specific prompt
type GetPromptRequest struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// GetPromptResult is the result of getting a prompt
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage represents a message in a prompt
type PromptMessage struct {
	Role    string  `json:"role"` // "user" or "assistant"
	Content Content `json:"content"`
}

// PromptListChangedNotification notifies that prompt list has changed
type PromptListChangedNotification struct{}

// ============================================================================
// Root Types
// ============================================================================

// Root represents a root directory or URI that the server can operate within
type Root struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

// ListRootsRequest requests a list of available roots from the client
type ListRootsRequest struct{}

// ListRootsResult is the client's response to roots/list
type ListRootsResult struct {
	Roots []Root `json:"roots"`
}

// RootsListChangedNotification notifies that roots list has changed
type RootsListChangedNotification struct{}

// ============================================================================
// Sampling Types (LLM Requests)
// ============================================================================

// CreateMessageRequest requests the LLM to sample a message
type CreateMessageRequest struct {
	Messages         []SamplingMessage `json:"messages"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	SystemPrompt     string            `json:"systemPrompt,omitempty"`
	IncludeContext   string            `json:"includeContext,omitempty"` // "none", "thisServer", "allServers"
	Temperature      float64           `json:"temperature,omitempty"`
	MaxTokens        int               `json:"maxTokens,omitempty"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	Meta             json.RawMessage   `json:"_meta,omitempty"`
}

// SamplingMessage represents a message in sampling
type SamplingMessage struct {
	Role    string  `json:"role"` // "user" or "assistant"
	Content Content `json:"content"`
}

// ModelPreferences represents preferences for model selection
type ModelPreferences struct {
	// Hints for model selection
	Hints []ModelHint `json:"hints,omitempty"`

	// Cost priority (0-1, higher = more important)
	CostPriority float64 `json:"costPriority,omitempty"`

	// Speed priority (0-1, higher = more important)
	SpeedPriority float64 `json:"speedPriority,omitempty"`

	// Intelligence priority (0-1, higher = more important)
	IntelligencePriority float64 `json:"intelligencePriority,omitempty"`
}

// ModelHint provides hints for model selection
type ModelHint struct {
	Name string `json:"name,omitempty"`
}

// CreateMessageResult is the result of sampling
type CreateMessageResult struct {
	Role       string          `json:"role"`
	Content    Content         `json:"content"`
	Model      string          `json:"model,omitempty"`      // Name of model used
	StopReason string          `json:"stopReason,omitempty"` // "endTurn", "stopSequence", "maxTokens"
	Meta       json.RawMessage `json:"_meta,omitempty"`
}

// ============================================================================
// Logging Types
// ============================================================================

// SetLevelRequest sets the minimum logging level
type SetLevelRequest struct {
	Level LoggingLevel `json:"level"`
}

// LoggingLevel represents the logging level
type LoggingLevel string

const (
	LoggingLevelDebug     LoggingLevel = "debug"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelWarning   LoggingLevel = "warning"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelEmergency LoggingLevel = "emergency"
)

// LoggingMessageNotification represents a log message
type LoggingMessageNotification struct {
	Level  LoggingLevel `json:"level"`
	Logger string       `json:"logger,omitempty"`
	Data   interface{}  `json:"data"`
}

// ============================================================================
// Progress Types
// ============================================================================

// ProgressToken is used to associate progress notifications with requests
type ProgressToken interface{}

// ProgressNotification reports progress for long-running operations
type ProgressNotification struct {
	ProgressToken ProgressToken `json:"progressToken"`
	Progress      float64       `json:"progress"`        // 0-1 or absolute
	Total         float64       `json:"total,omitempty"` // If absolute progress
}

// ============================================================================
// Cancellation Types
// ============================================================================

// CancelledNotification notifies that a request has been cancelled
type CancelledNotification struct {
	RequestID interface{} `json:"requestId"`
	Reason    string      `json:"reason,omitempty"`
}

// ============================================================================
// Pagination Types
// ============================================================================

// PaginatedRequest represents a request that supports pagination
type PaginatedRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// PaginatedResult represents a result that supports pagination
type PaginatedResult struct {
	NextCursor string `json:"nextCursor,omitempty"`
}

// ============================================================================
// HelixAgent MCP Extensions
// ============================================================================

// MCPServerInfo extends MCP server with HelixAgent metadata
type MCPServerInfo struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Version     string              `json:"version"`
	Package     string              `json:"package,omitempty"`
	Category    string              `json:"category,omitempty"`
	CostModel   string              `json:"costModel,omitempty"` // "free", "freemium", "paid"
	Tools       []MCPToolDefinition `json:"tools,omitempty"`
	Resources   []Resource          `json:"resources,omitempty"`
	Prompts     []Prompt            `json:"prompts,omitempty"`
	Config      map[string]string   `json:"config,omitempty"`
	Enabled     bool                `json:"enabled"`
}

// MCPConnectionState represents the connection state
type MCPConnectionState string

const (
	ConnectionStateDisconnected MCPConnectionState = "disconnected"
	ConnectionStateConnecting   MCPConnectionState = "connecting"
	ConnectionStateConnected    MCPConnectionState = "connected"
	ConnectionStateError        MCPConnectionState = "error"
)

// MCPConnectionStats provides connection statistics
type MCPConnectionStats struct {
	ServerID         string             `json:"serverId"`
	State            MCPConnectionState `json:"state"`
	ConnectedAt      *int64             `json:"connectedAt,omitempty"`
	LastActivityAt   int64              `json:"lastActivityAt"`
	MessagesSent     int64              `json:"messagesSent"`
	MessagesReceived int64              `json:"messagesReceived"`
	Errors           int64              `json:"errors"`
	LatencyMs        int64              `json:"latencyMs,omitempty"`
}

// MCPRegistry represents the MCP server registry
type MCPRegistry struct {
	Servers map[string]*MCPServerInfo `json:"servers"`
}

// MCPConfiguration represents MCP configuration for HelixAgent
type MCPConfiguration struct {
	Enabled        bool                        `json:"enabled"`
	Timeout        int                         `json:"timeout,omitempty"`
	MaxConcurrent  int                         `json:"maxConcurrent,omitempty"`
	DefaultServers []string                    `json:"defaultServers,omitempty"`
	Servers        map[string]*MCPServerConfig `json:"servers"`
}

// MCPServerConfig represents configuration for a specific MCP server
type MCPServerConfig struct {
	Enabled bool                   `json:"enabled"`
	Package string                 `json:"package,omitempty"`
	Command string                 `json:"command,omitempty"`
	Args    []string               `json:"args,omitempty"`
	Env     map[string]string      `json:"env,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}
