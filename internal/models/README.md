# Models Package

The models package contains data structures and types used throughout SuperAgent.

## Overview

This package defines:
- Request/Response structures for APIs
- Database entity models
- Protocol-specific types (MCP, LSP, ACP)
- Enum definitions

## Core Models

### Completion Models

```go
// CompletionRequest represents an LLM completion request
type CompletionRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
    Temperature float64   `json:"temperature,omitempty"`
    Stream      bool      `json:"stream,omitempty"`
}

// CompletionResponse represents an LLM completion response
type CompletionResponse struct {
    ID      string   `json:"id"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}
```

### Message Types

```go
type Message struct {
    Role    string `json:"role"`    // "system", "user", "assistant"
    Content string `json:"content"`
}

type Choice struct {
    Index        int     `json:"index"`
    Message      Message `json:"message"`
    FinishReason string  `json:"finish_reason"`
}
```

### Provider Models

```go
type Provider struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    BaseURL     string `json:"base_url"`
    APIKey      string `json:"-"` // Hidden in JSON
    Enabled     bool   `json:"enabled"`
}

type Model struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Provider      string `json:"provider"`
    MaxTokens     int    `json:"max_tokens"`
    ContextWindow int    `json:"context_window"`
}
```

## Protocol Types

### MCP (Model Context Protocol)

```go
type MCPRequest struct {
    Method string      `json:"method"`
    Params interface{} `json:"params"`
}

type MCPResponse struct {
    Result interface{} `json:"result"`
    Error  *MCPError   `json:"error,omitempty"`
}
```

### LSP (Language Server Protocol)

```go
type LSPRequest struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      int         `json:"id"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params"`
}
```

## Enums

```go
type ProviderType string

const (
    ProviderOpenAI    ProviderType = "openai"
    ProviderAnthropic ProviderType = "anthropic"
    ProviderGoogle    ProviderType = "google"
    // ... more providers
)

type MessageRole string

const (
    RoleSystem    MessageRole = "system"
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
)
```

## Validation

Models include validation tags for request validation:

```go
type CreateUserRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
    Name     string `json:"name" binding:"required"`
}
```
