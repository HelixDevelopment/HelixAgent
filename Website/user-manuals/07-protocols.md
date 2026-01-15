# User Manual 07: Protocol Integration Guide

## Overview

HelixAgent supports multiple communication protocols for integrating with development tools, language servers, and AI agents. This guide covers the MCP, LSP, and ACP protocols.

## Supported Protocols

| Protocol | Purpose | Endpoint |
|----------|---------|----------|
| MCP | Model Context Protocol | `/v1/mcp` |
| LSP | Language Server Protocol | `/v1/lsp` |
| ACP | Agent Communication Protocol | `/v1/acp` |

## Model Context Protocol (MCP)

MCP enables AI models to interact with external tools and resources through a standardized interface.

### Available MCP Tools

```bash
# List available MCP tools
curl http://localhost:8080/v1/mcp/tools
```

Response:
```json
{
    "tools": [
        {
            "name": "read_file",
            "description": "Read contents of a file",
            "parameters": {
                "path": "string (required)"
            }
        },
        {
            "name": "write_file",
            "description": "Write contents to a file",
            "parameters": {
                "path": "string (required)",
                "content": "string (required)"
            }
        },
        {
            "name": "search_files",
            "description": "Search for files matching a pattern",
            "parameters": {
                "pattern": "string (required)",
                "directory": "string (optional)"
            }
        }
    ]
}
```

### Executing MCP Tools

```bash
# Execute a tool
curl -X POST http://localhost:8080/v1/mcp/tools/read_file \
  -H "Content-Type: application/json" \
  -d '{"path": "/path/to/file.txt"}'
```

Response:
```json
{
    "result": {
        "content": "File contents here...",
        "metadata": {
            "size": 1234,
            "modified": "2024-01-15T10:30:00Z"
        }
    }
}
```

### MCP Resources

Access external resources through MCP:

```bash
# List available resources
curl http://localhost:8080/v1/mcp/resources

# Get specific resource
curl http://localhost:8080/v1/mcp/resources/project-files
```

### MCP Configuration

Configure MCP in your environment:

```yaml
# config/mcp.yaml
mcp:
  enabled: true
  tools:
    - read_file
    - write_file
    - search_files
    - execute_command
  resources:
    - project-files
    - documentation
  security:
    sandbox: true
    allowed_paths:
      - /home/user/projects
    denied_commands:
      - rm -rf
```

## Language Server Protocol (LSP)

LSP integration provides IDE-like features for code analysis and completion.

### LSP Capabilities

- Code completion
- Hover information
- Go to definition
- Find references
- Diagnostics
- Code actions
- Formatting

### Code Completion

```bash
curl -X POST http://localhost:8080/v1/lsp/completions \
  -H "Content-Type: application/json" \
  -d '{
    "document": {
      "uri": "file:///path/to/main.go",
      "content": "package main\n\nfunc main() {\n\tfmt.Pr"
    },
    "position": {
      "line": 3,
      "character": 9
    }
  }'
```

Response:
```json
{
    "completions": [
        {
            "label": "Print",
            "kind": "Function",
            "detail": "func Print(a ...any) (n int, err error)",
            "documentation": "Print formats using default formats..."
        },
        {
            "label": "Printf",
            "kind": "Function",
            "detail": "func Printf(format string, a ...any) (n int, err error)"
        },
        {
            "label": "Println",
            "kind": "Function",
            "detail": "func Println(a ...any) (n int, err error)"
        }
    ]
}
```

### Hover Information

```bash
curl -X POST http://localhost:8080/v1/lsp/hover \
  -H "Content-Type: application/json" \
  -d '{
    "document": {
      "uri": "file:///path/to/main.go"
    },
    "position": {
      "line": 10,
      "character": 5
    }
  }'
```

### Go to Definition

```bash
curl -X POST http://localhost:8080/v1/lsp/definition \
  -H "Content-Type: application/json" \
  -d '{
    "document": {
      "uri": "file:///path/to/main.go"
    },
    "position": {
      "line": 15,
      "character": 12
    }
  }'
```

Response:
```json
{
    "location": {
        "uri": "file:///path/to/utils.go",
        "range": {
            "start": {"line": 25, "character": 0},
            "end": {"line": 25, "character": 15}
        }
    }
}
```

### Supported Language Servers

| Language | Server | Command |
|----------|--------|---------|
| Go | gopls | `gopls serve` |
| Python | pylsp | `pylsp` |
| TypeScript | tsserver | `typescript-language-server --stdio` |
| Rust | rust-analyzer | `rust-analyzer` |
| Java | jdtls | `jdtls` |

### LSP Configuration

```yaml
# config/lsp.yaml
lsp:
  enabled: true
  servers:
    go:
      command: gopls
      args: ["serve"]
      root_markers: ["go.mod"]
    python:
      command: pylsp
      root_markers: ["pyproject.toml", "setup.py"]
  timeout: 30s
  max_completions: 50
```

## Agent Communication Protocol (ACP)

ACP enables communication between AI agents for collaborative tasks.

### Agent Registration

```bash
# Register an agent
curl -X POST http://localhost:8080/v1/acp/agents \
  -H "Content-Type: application/json" \
  -d '{
    "id": "code-reviewer",
    "name": "Code Review Agent",
    "capabilities": ["code_review", "security_audit"],
    "endpoint": "http://localhost:8081/agent"
  }'
```

### Sending Messages to Agents

```bash
# Send message to agent
curl -X POST http://localhost:8080/v1/acp/messages \
  -H "Content-Type: application/json" \
  -d '{
    "to": "code-reviewer",
    "type": "request",
    "content": {
      "action": "review_code",
      "file": "/path/to/main.go"
    }
  }'
```

Response:
```json
{
    "message_id": "msg-123",
    "status": "delivered",
    "response": {
        "findings": [
            {
                "severity": "warning",
                "line": 42,
                "message": "Consider error handling here"
            }
        ]
    }
}
```

### Agent Discovery

```bash
# List available agents
curl http://localhost:8080/v1/acp/agents

# Get agent capabilities
curl http://localhost:8080/v1/acp/agents/code-reviewer/capabilities
```

### ACP Configuration

```yaml
# config/acp.yaml
acp:
  enabled: true
  discovery:
    enabled: true
    interval: 60s
  agents:
    - id: code-reviewer
      name: Code Review Agent
      endpoint: http://localhost:8081
    - id: test-runner
      name: Test Runner Agent
      endpoint: http://localhost:8082
  security:
    require_auth: true
    allowed_agents:
      - code-reviewer
      - test-runner
```

## Protocol Integration with AI Debate

HelixAgent's protocols integrate seamlessly with the AI debate system:

```bash
# Start debate with protocol access
curl -X POST http://localhost:8080/v1/debates \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Review this code for security issues",
    "context": {
      "mcp_tools": ["read_file", "search_files"],
      "lsp_enabled": true,
      "file": "/path/to/main.go"
    },
    "participants": ["claude", "gemini", "deepseek"]
  }'
```

## Real-time Protocol Events (SSE)

Subscribe to protocol events:

```bash
# Subscribe to MCP events
curl http://localhost:8080/v1/events/mcp

# Subscribe to LSP events
curl http://localhost:8080/v1/events/lsp

# Subscribe to ACP events
curl http://localhost:8080/v1/events/acp
```

Event format:
```
event: tool_execution
data: {"tool":"read_file","status":"completed","duration_ms":45}

event: completion_request
data: {"document":"main.go","completions_returned":15}

event: agent_message
data: {"from":"code-reviewer","type":"response"}
```

## Security Considerations

### MCP Security

- Sandbox tool execution
- Whitelist allowed paths
- Deny dangerous commands
- Rate limit tool calls

### LSP Security

- Validate document URIs
- Sandbox code execution
- Limit response sizes
- Timeout long operations

### ACP Security

- Authenticate agents
- Encrypt messages
- Validate payloads
- Audit message logs

## Troubleshooting

### MCP Issues

| Issue | Solution |
|-------|----------|
| Tool not found | Check tool is enabled in config |
| Permission denied | Verify path is in allowed_paths |
| Timeout | Increase timeout or check tool execution |

### LSP Issues

| Issue | Solution |
|-------|----------|
| No completions | Verify language server is running |
| Slow responses | Check server resource usage |
| Definition not found | Ensure project is indexed |

### ACP Issues

| Issue | Solution |
|-------|----------|
| Agent not found | Check agent registration |
| Message timeout | Verify agent endpoint |
| Auth failed | Check agent credentials |

## Next Steps

- [Troubleshooting Guide](08-troubleshooting.md)
- [API Reference](04-api-reference.md)
- [Deployment Guide](05-deployment-guide.md)
