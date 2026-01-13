# HelixAgent Protocol Support Documentation

## Overview

HelixAgent now supports comprehensive LLM protocol integration, transforming it from a model management system into a complete AI orchestration platform. The system supports four major protocols:

- **MCP (Model Context Protocol)** - Tool execution and agent integration
- **LSP (Language Server Protocol)** - Code intelligence and language services
- **ACP (Agent Client Protocol)** - Agent communication and coordination
- **Embeddings** - Vector operations and semantic search

## Architecture

### Unified Protocol Manager

The `UnifiedProtocolManager` provides a single interface for all protocol operations:

```go
type UnifiedProtocolManager struct {
    mcpManager       *MCPManager
    lspManager       *LSPManager
    acpManager       *ACPManager
    embeddingManager *EmbeddingManager
    cache            CacheInterface
    repo             *database.ModelMetadataRepository
    log              *logrus.Logger
}
```

### Protocol Request Format

All protocols use a unified request format:

```json
{
  "protocolType": "mcp|acp|lsp|embedding",
  "serverId": "server-identifier",
  "toolName": "operation-name",
  "arguments": {
    "key": "value"
  }
}
```

## API Endpoints

### Unified Protocol Endpoints

#### Execute Protocol Request
```bash
POST /v1/protocols/execute
Content-Type: application/json

{
  "protocolType": "mcp",
  "serverId": "filesystem-tools",
  "toolName": "read_file",
  "arguments": {
    "path": "/etc/hosts"
  }
}
```

#### List Protocol Servers
```bash
GET /v1/protocols/servers

Response:
{
  "mcp": [
    {"id": "filesystem-tools", "name": "File System Tools"},
    {"id": "web-scraper", "name": "Web Scraping Tools"}
  ],
  "acp": [
    {"id": "opencode-agent", "name": "OpenCode Agent"}
  ],
  "lsp": [
    {"id": "typescript-lsp", "name": "TypeScript Language Server"}
  ],
  "embedding": [
    {"name": "text-embedding-ada-002", "dimension": 384}
  ]
}
```

#### Get Protocol Metrics
```bash
GET /v1/protocols/metrics

Response:
{
  "overall": {
    "totalProtocols": 4,
    "activeRequests": 0,
    "cacheSize": 0
  },
  "mcp": {
    "totalServers": 2,
    "activeConnections": 1,
    "totalTools": 15
  },
  "lsp": {
    "totalServers": 1,
    "activeServers": 1,
    "totalRequests": 0
  }
}
```

#### Refresh Protocol Servers
```bash
POST /v1/protocols/refresh

Response:
{
  "message": "Protocol servers refreshed successfully"
}
```

#### Configure Protocols
```bash
POST /v1/protocols/configure
Content-Type: application/json

{
  "mcp": {
    "enabled": true,
    "servers": ["filesystem-tools", "web-scraper"]
  },
  "lsp": {
    "enabled": true
  },
  "acp": {
    "enabled": true
  },
  "embedding": {
    "enabled": true
  }
}
```

### MCP Protocol Endpoints

#### List MCP Servers
```bash
GET /v1/mcp/servers

Response:
[
  {
    "id": "filesystem-tools",
    "name": "File System Tools",
    "type": "local",
    "enabled": true,
    "tools": [
      {
        "name": "read_file",
        "description": "Read file contents",
        "inputSchema": {
          "type": "object",
          "properties": {
            "path": {"type": "string"}
          }
        }
      }
    ]
  }
]
```

#### Execute MCP Tool
```bash
POST /v1/mcp/servers/filesystem-tools/execute
Content-Type: application/json

{
  "toolName": "read_file",
  "arguments": {
    "path": "/etc/hosts"
  }
}

Response:
{
  "success": true,
  "result": "127.0.0.1 localhost\n...",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

#### List MCP Server Tools
```bash
GET /v1/mcp/servers/filesystem-tools/tools

Response:
[
  {
    "name": "read_file",
    "description": "Read file contents",
    "inputSchema": {
      "type": "object",
      "properties": {
        "path": {"type": "string"}
      }
    }
  }
]
```

#### Sync MCP Server
```bash
POST /v1/mcp/servers/filesystem-tools/sync

Response:
{
  "message": "MCP server synced successfully"
}
```

#### Get MCP Statistics
```bash
GET /v1/mcp/stats

Response:
{
  "totalServers": 2,
  "enabledServers": 2,
  "totalTools": 15,
  "lastSync": "2024-01-01T12:00:00Z"
}
```

## Protocol SSE Endpoints (CLI Agent Integration)

HelixAgent provides Server-Sent Events (SSE) endpoints for CLI agent integration, supporting OpenCode, Crush, HelixCode, and Kilo Code. These endpoints implement the MCP (Model Context Protocol) specification over HTTP with SSE transport.

### Supported Protocols

| Protocol | SSE Endpoint | Description |
|----------|-------------|-------------|
| MCP | `GET/POST /v1/mcp` | Model Context Protocol |
| ACP | `GET/POST /v1/acp` | Agent Communication Protocol |
| LSP | `GET/POST /v1/lsp` | Language Server Protocol |
| Embeddings | `GET/POST /v1/embeddings` | Vector embeddings |
| Vision | `GET/POST /v1/vision` | Image analysis and OCR |
| Cognee | `GET/POST /v1/cognee` | Knowledge graph and RAG |

### SSE Connection (GET)

Connect to receive real-time events:

```bash
GET /v1/mcp
Accept: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

# Server responds with endpoint event and heartbeats
event: endpoint
data: /v1/mcp

: heartbeat
```

### JSON-RPC Messages (POST)

Send JSON-RPC 2.0 messages to interact with the protocol:

#### Initialize Connection
```bash
POST /v1/mcp
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "opencode",
      "version": "1.0.0"
    }
  }
}

Response:
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "serverInfo": {
      "name": "helixagent-mcp",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {"listChanged": true},
      "prompts": {"listChanged": true},
      "resources": {"subscribe": true, "listChanged": true}
    }
  }
}
```

#### List Available Tools
```bash
POST /v1/mcp
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}

Response:
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "mcp_list_providers",
        "description": "List all available LLM providers",
        "inputSchema": {
          "type": "object",
          "properties": {}
        }
      },
      {
        "name": "mcp_get_capabilities",
        "description": "Get MCP server capabilities",
        "inputSchema": {
          "type": "object",
          "properties": {}
        }
      }
    ]
  }
}
```

#### Execute Tool
```bash
POST /v1/mcp
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "mcp_get_capabilities",
    "arguments": {}
  }
}

Response:
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"tools\":{\"listChanged\":true},...}"
      }
    ]
  }
}
```

#### Ping
```bash
POST /v1/mcp
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "ping"
}

Response:
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {}
}
```

### Protocol-Specific Tools

#### MCP Tools
| Tool | Description |
|------|-------------|
| `mcp_list_providers` | List all available LLM providers |
| `mcp_get_capabilities` | Get MCP server capabilities |
| `mcp_execute_tool` | Execute a tool on a specific provider |

#### ACP Tools
| Tool | Description |
|------|-------------|
| `acp_send_message` | Send a message to an agent |
| `acp_list_agents` | List available agents |

#### LSP Tools
| Tool | Description |
|------|-------------|
| `lsp_get_diagnostics` | Get diagnostics for a file |
| `lsp_go_to_definition` | Go to symbol definition |
| `lsp_find_references` | Find all references to a symbol |
| `lsp_list_servers` | List available LSP servers |

#### Embeddings Tools
| Tool | Description |
|------|-------------|
| `embeddings_generate` | Generate embeddings for text |
| `embeddings_search` | Search for similar content |

#### Vision Tools
| Tool | Description |
|------|-------------|
| `vision_analyze_image` | Analyze an image |
| `vision_ocr` | Extract text from an image |

#### Cognee Tools
| Tool | Description |
|------|-------------|
| `cognee_add` | Add content to knowledge graph |
| `cognee_search` | Search the knowledge graph |
| `cognee_visualize` | Visualize the knowledge graph |

### CLI Agent Integration Example (OpenCode)

```javascript
// OpenCode MCP client integration
const response = await fetch('http://localhost:7061/v1/mcp', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'initialize',
    params: {
      protocolVersion: '2024-11-05',
      capabilities: {
        roots: { listChanged: true },
        sampling: {}
      },
      clientInfo: {
        name: 'opencode',
        version: '1.0.0'
      }
    }
  })
});

const result = await response.json();
console.log('Server:', result.result.serverInfo.name);
// Output: Server: helixagent-mcp
```

### LSP Protocol Endpoints

#### List LSP Servers
```bash
GET /v1/lsp/servers

Response:
[
  {
    "id": "typescript-language-server",
    "name": "TypeScript Language Server",
    "language": "typescript",
    "command": "typescript-language-server",
    "enabled": true
  }
]
```

#### Execute LSP Request
```bash
POST /v1/lsp/execute
Content-Type: application/json

{
  "serverId": "typescript-language-server",
  "toolName": "completion",
  "arguments": {
    "filePath": "/path/to/file.ts",
    "line": 10,
    "character": 5
  }
}
```

#### Get LSP Statistics
```bash
GET /v1/lsp/stats

Response:
{
  "totalServers": 1,
  "enabledServers": 1,
  "totalRequests": 0,
  "averageLatencyMs": 0
}
```

### ACP Protocol Endpoints

#### List ACP Servers
```bash
GET /v1/acp/servers

Response:
[
  {
    "id": "opencode-1",
    "name": "OpenCode Agent",
    "url": "ws://localhost:7061/agent",
    "enabled": true,
    "version": "1.0.0"
  }
]
```

#### Execute ACP Action
```bash
POST /v1/acp/execute
Content-Type: application/json

{
  "serverId": "opencode-1",
  "action": "code_execution",
  "parameters": {
    "language": "python",
    "code": "print('Hello, World!')"
  }
}

Response:
{
  "success": true,
  "data": "Action code_execution executed successfully on server opencode-1",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Embeddings Endpoints

#### Generate Embedding
```bash
POST /v1/embeddings/generate
Content-Type: application/json

{
  "text": "Hello, world! This is a test document."
}

Response:
{
  "success": true,
  "embeddings": [0.1, 0.2, 0.3, ...],
  "timestamp": "2024-01-01T12:00:00Z"
}
```

#### Generate Batch Embeddings
```bash
POST /v1/embeddings/generate-batch
Content-Type: application/json

{
  "texts": [
    "First document",
    "Second document",
    "Third document"
  ]
}

Response:
{
  "embeddings": [
    {
      "success": true,
      "embeddings": [0.1, 0.2, ...],
      "timestamp": "2024-01-01T12:00:00Z"
    }
  ]
}
```

#### Compare Embeddings
```bash
POST /v1/embeddings/compare
Content-Type: application/json

{
  "text1": "Hello world",
  "text2": "Hi universe"
}

Response:
{
  "similarity": 0.85
}
```

#### List Embedding Providers
```bash
GET /v1/embeddings/providers

Response:
[
  {
    "name": "text-embedding-ada-002",
    "model": "text-embedding-ada-002",
    "dimension": 384,
    "enabled": true
  }
]
```

## Usage Examples

### Python Client Example

```python
import requests
import json

class HelixAgentClient:
    def __init__(self, base_url="http://localhost:7061"):
        self.base_url = base_url

    def execute_protocol_request(self, protocol_type, server_id, tool_name, arguments):
        """Execute a protocol request"""
        url = f"{self.base_url}/v1/protocols/execute"
        payload = {
            "protocolType": protocol_type,
            "serverId": server_id,
            "toolName": tool_name,
            "arguments": arguments
        }

        response = requests.post(url, json=payload)
        return response.json()

    def list_servers(self):
        """List all protocol servers"""
        url = f"{self.base_url}/v1/protocols/servers"
        response = requests.get(url)
        return response.json()

    def get_metrics(self):
        """Get protocol metrics"""
        url = f"{self.base_url}/v1/protocols/metrics"
        response = requests.get(url)
        return response.json()

# Usage
client = HelixAgentClient()

# List available servers
servers = client.list_servers()
print("Available servers:", json.dumps(servers, indent=2))

# Execute an MCP tool
result = client.execute_protocol_request(
    protocol_type="mcp",
    server_id="filesystem-tools",
    tool_name="read_file",
    arguments={"path": "/etc/hosts"}
)
print("MCP result:", result)

# Get metrics
metrics = client.get_metrics()
print("Metrics:", json.dumps(metrics, indent=2))
```

### JavaScript/Node.js Client Example

```javascript
class HelixAgentClient {
    constructor(baseURL = 'http://localhost:7061') {
        this.baseURL = baseURL;
    }

    async executeProtocolRequest(protocolType, serverId, toolName, arguments) {
        const url = `${this.baseURL}/v1/protocols/execute`;
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                protocolType,
                serverId,
                toolName,
                arguments
            })
        });
        return response.json();
    }

    async listServers() {
        const url = `${this.baseURL}/v1/protocols/servers`;
        const response = await fetch(url);
        return response.json();
    }

    async getMetrics() {
        const url = `${this.baseURL}/v1/protocols/metrics`;
        const response = await fetch(url);
        return response.json();
    }
}

// Usage
const client = new HelixAgentClient();

// Execute embedding generation
client.executeProtocolRequest(
    'embedding',
    null,
    null,
    { text: 'Hello, world!' }
).then(result => {
    console.log('Embedding result:', result);
});

// List servers
client.listServers().then(servers => {
    console.log('Available servers:', servers);
});
```

### Go Client Example

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type HelixAgentClient struct {
    baseURL string
    client  *http.Client
}

type ProtocolRequest struct {
    ProtocolType string                 `json:"protocolType"`
    ServerID     string                 `json:"serverId,omitempty"`
    ToolName     string                 `json:"toolName,omitempty"`
    Arguments    map[string]interface{} `json:"arguments,omitempty"`
}

type ProtocolResponse struct {
    Success   bool                   `json:"success"`
    Result    interface{}            `json:"result,omitempty"`
    Error     string                 `json:"error,omitempty"`
    Timestamp string                 `json:"timestamp"`
    Protocol  string                 `json:"protocol,omitempty"`
}

func NewHelixAgentClient(baseURL string) *HelixAgentClient {
    return &HelixAgentClient{
        baseURL: baseURL,
        client:  &http.Client{},
    }
}

func (c *HelixAgentClient) ExecuteProtocolRequest(req ProtocolRequest) (*ProtocolResponse, error) {
    url := c.baseURL + "/v1/protocols/execute"

    jsonData, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }

    resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result ProtocolResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return &result, nil
}

func main() {
    client := NewHelixAgentClient("http://localhost:7061")

    // Execute MCP tool
    req := ProtocolRequest{
        ProtocolType: "mcp",
        ServerID:     "filesystem-tools",
        ToolName:     "read_file",
        Arguments: map[string]interface{}{
            "path": "/etc/hosts",
        },
    }

    result, err := client.ExecuteProtocolRequest(req)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Result: %+v\n", result)
}
```

## Configuration

### Environment Variables

```bash
# Protocol Configuration
HELIXAGENT_PROTOCOLS_ENABLED=true
HELIXAGENT_MCP_ENABLED=true
HELIXAGENT_LSP_ENABLED=true
HELIXAGENT_ACP_ENABLED=true
HELIXAGENT_EMBEDDINGS_ENABLED=true

# MCP Configuration
MCP_SERVER_TIMEOUT=30s
MCP_MAX_RETRIES=3
MCP_CACHE_TTL=5m

# LSP Configuration
LSP_WORKSPACE_ROOT=/tmp/workspace
LSP_DEFAULT_LANGUAGE=go
LSP_SERVER_TIMEOUT=10s

# ACP Configuration
ACP_DEFAULT_URL=ws://localhost:7061/agent
ACP_CONNECTION_TIMEOUT=30s
ACP_HEARTBEAT_INTERVAL=30s

# Embeddings Configuration
EMBEDDINGS_PROVIDER=openai
EMBEDDINGS_MODEL=text-embedding-ada-002
EMBEDDINGS_DIMENSION=384
EMBEDDINGS_CACHE_TTL=1h
```

### YAML Configuration

```yaml
protocols:
  enabled: true
  mcp:
    enabled: true
    servers:
      - id: filesystem-tools
        name: File System Tools
        type: local
        command: ["node", "/path/to/mcp-filesystem.js"]
        enabled: true
  lsp:
    enabled: true
    servers:
      - id: typescript-lsp
        name: TypeScript Language Server
        language: typescript
        command: typescript-language-server
        enabled: true
  acp:
    enabled: true
    servers:
      - id: opencode-agent
        name: OpenCode Agent
        url: ws://localhost:7061/agent
        enabled: true
  embeddings:
    enabled: true
    provider: openai
    model: text-embedding-ada-002
    dimension: 384
    cache_ttl: 1h
```

## Monitoring and Health Checks

### Health Endpoints

```bash
# Overall protocol health
GET /v1/protocols/health

# Individual protocol health
GET /v1/mcp/health
GET /v1/lsp/health
GET /v1/acp/health
GET /v1/embeddings/health
```

### Metrics Integration

The system integrates with Prometheus for metrics collection:

```bash
# Protocol request metrics
protocol_requests_total{protocol="mcp", status="success"} 150
protocol_requests_duration_seconds{protocol="mcp", quantile="0.5"} 0.023

# Cache metrics
protocol_cache_hits_total{protocol="embedding"} 89
protocol_cache_misses_total{protocol="embedding"} 11

# Server health metrics
protocol_server_up{protocol="mcp", server="filesystem-tools"} 1
protocol_server_response_time_seconds{protocol="lsp", server="typescript"} 0.015
```

## Error Handling

### Standard Error Responses

```json
{
  "error": "detailed error message",
  "code": "ERROR_CODE",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Common Error Codes

- `INVALID_PROTOCOL_TYPE` - Unsupported protocol type
- `SERVER_NOT_FOUND` - Protocol server not found
- `TOOL_NOT_AVAILABLE` - Requested tool not available
- `EXECUTION_FAILED` - Protocol execution failed
- `CONFIGURATION_ERROR` - Invalid configuration
- `TIMEOUT_ERROR` - Request timed out

## Security Considerations

### Authentication

All protocol endpoints support authentication via:

- JWT tokens in Authorization header
- API keys in X-API-Key header
- OAuth2 bearer tokens

### Authorization

Role-based access control for protocol operations:

- `protocol:read` - View protocol servers and metrics
- `protocol:execute` - Execute protocol requests
- `protocol:admin` - Configure and manage protocol servers

### Input Validation

- All inputs are validated and sanitized
- Rate limiting applied to prevent abuse
- Request size limits enforced
- Timeout controls prevent hanging requests

## Performance Optimization

### Caching Strategy

- Protocol responses cached with configurable TTL
- Server capabilities cached for quick access
- Embedding vectors cached to reduce API calls

### Connection Pooling

- MCP servers maintain persistent connections
- LSP servers reuse language server processes
- ACP connections pooled for efficiency

### Batch Operations

- Support for batch embedding generation
- Bulk tool execution where supported
- Parallel processing for multiple requests

## Troubleshooting

### Common Issues

1. **Protocol server not responding**
   - Check server configuration
   - Verify network connectivity
   - Review server logs

2. **Tool execution timeout**
   - Increase timeout values
   - Check server performance
   - Verify tool parameters

3. **High latency**
   - Enable caching
   - Check network connectivity
   - Monitor server performance

### Debug Mode

Enable debug logging for detailed request tracing:

```bash
export LOG_LEVEL=debug
export PROTOCOL_DEBUG=true
```

### Health Checks

Regular health checks ensure protocol servers are operational:

```bash
# Check all protocols
curl http://localhost:7061/v1/protocols/health

# Check specific protocol
curl http://localhost:7061/v1/mcp/health
```

## Contributing

### Adding New Protocols

1. Create protocol manager in `internal/services/`
2. Implement required interfaces
3. Add handler in `internal/handlers/`
4. Update router configuration
5. Add tests and documentation

### Protocol Interface

```go
type ProtocolManager interface {
    ExecuteRequest(ctx context.Context, req UnifiedProtocolRequest) (UnifiedProtocolResponse, error)
    ListServers(ctx context.Context) ([]ServerInfo, error)
    GetMetrics(ctx context.Context) (map[string]interface{}, error)
    Refresh(ctx context.Context) error
}
```

This documentation provides comprehensive guidance for using HelixAgent's protocol support features. The system is designed to be extensible, allowing easy integration of additional AI protocols as they emerge.