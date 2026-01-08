# Module 8: MCP/LSP Integration

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 8: MCP/LSP Integration
- Duration: 60 minutes
- Protocol-Based AI Extensions

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Understand protocol support in HelixAgent
- Configure MCP, LSP, and ACP servers
- Implement protocol-based workflows
- Use embeddings for semantic search

---

## Slide 3: Protocol Overview

**Supported Protocols:**

| Protocol | Purpose |
|----------|---------|
| MCP | Model Context Protocol - Tool execution |
| LSP | Language Server Protocol - Code intelligence |
| ACP | Agent Client Protocol - Agent communication |
| Embeddings | Vector operations, semantic search |

---

## Slide 4: Unified Protocol Manager

**Architecture:**

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

---

## Slide 5: Protocol Request Format

**Unified Request Structure:**

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

---

## Slide 6: MCP Overview

**Model Context Protocol:**

- Tool execution for AI agents
- File system operations
- Web scraping capabilities
- External service integration
- Extends LLM capabilities

---

## Slide 7: MCP Configuration

**Setting Up MCP Servers:**

```yaml
protocols:
  mcp:
    enabled: true
    servers:
      - id: filesystem-tools
        name: File System Tools
        type: local
        command: ["node", "/path/to/mcp-filesystem.js"]
        enabled: true

      - id: web-scraper
        name: Web Scraping Tools
        type: local
        command: ["python", "/path/to/scraper.py"]
        enabled: true
```

---

## Slide 8: MCP API - List Servers

**Get Available MCP Servers:**

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

---

## Slide 9: MCP Tool Execution

**Execute an MCP Tool:**

```bash
POST /v1/mcp/servers/filesystem-tools/execute

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

---

## Slide 10: LSP Overview

**Language Server Protocol:**

- Code intelligence features
- Autocomplete suggestions
- Go-to-definition
- Find references
- Diagnostics and linting

---

## Slide 11: LSP Configuration

**Setting Up LSP Servers:**

```yaml
protocols:
  lsp:
    enabled: true
    servers:
      - id: typescript-lsp
        name: TypeScript Language Server
        language: typescript
        command: typescript-language-server
        enabled: true

      - id: go-lsp
        name: Go Language Server
        language: go
        command: gopls
        enabled: true
```

---

## Slide 12: LSP API

**Execute LSP Request:**

```bash
POST /v1/lsp/execute

{
  "serverId": "typescript-language-server",
  "toolName": "completion",
  "arguments": {
    "filePath": "/path/to/file.ts",
    "line": 10,
    "character": 5
  }
}

Response:
{
  "completions": [
    {
      "label": "console",
      "kind": "variable",
      "detail": "Console object"
    }
  ]
}
```

---

## Slide 13: ACP Overview

**Agent Client Protocol:**

- Agent-to-agent communication
- Task delegation
- Collaborative workflows
- Agent orchestration

---

## Slide 14: ACP Configuration

**Setting Up ACP Servers:**

```yaml
protocols:
  acp:
    enabled: true
    servers:
      - id: opencode-agent
        name: OpenCode Agent
        url: ws://localhost:7061/agent
        enabled: true
        version: "1.0.0"
```

---

## Slide 15: ACP API

**Execute ACP Action:**

```bash
POST /v1/acp/execute

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
  "data": "Action executed successfully",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

---

## Slide 16: Embeddings Overview

**Vector Operations:**

- Text embedding generation
- Semantic similarity search
- Document retrieval
- Clustering and classification

---

## Slide 17: Embeddings Configuration

**Setting Up Embeddings:**

```yaml
protocols:
  embeddings:
    enabled: true
    provider: openai
    model: text-embedding-ada-002
    dimension: 384
    cache_ttl: 1h
```

---

## Slide 18: Embeddings API

**Generate Embeddings:**

```bash
POST /v1/embeddings/generate

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

---

## Slide 19: Batch Embeddings

**Generate Multiple Embeddings:**

```bash
POST /v1/embeddings/generate-batch

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
    {"success": true, "embeddings": [0.1, 0.2, ...]},
    {"success": true, "embeddings": [0.3, 0.4, ...]},
    {"success": true, "embeddings": [0.5, 0.6, ...]}
  ]
}
```

---

## Slide 20: Semantic Similarity

**Compare Text Similarity:**

```bash
POST /v1/embeddings/compare

{
  "text1": "Hello world",
  "text2": "Hi universe"
}

Response:
{
  "similarity": 0.85
}
```

---

## Slide 21: Unified Protocol API

**Single Endpoint for All Protocols:**

```bash
POST /v1/protocols/execute

{
  "protocolType": "mcp",
  "serverId": "filesystem-tools",
  "toolName": "read_file",
  "arguments": {
    "path": "/etc/hosts"
  }
}
```

---

## Slide 22: Protocol Metrics

**Monitoring Protocol Usage:**

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
  }
}
```

---

## Slide 23: Protocol Health

**Health Monitoring:**

```bash
GET /v1/protocols/health

Response:
{
  "mcp": "healthy",
  "lsp": "healthy",
  "acp": "degraded",
  "embeddings": "healthy"
}
```

---

## Slide 24: Python Client Example

**Using Protocols from Python:**

```python
import requests

class HelixAgentClient:
    def __init__(self, base_url="http://localhost:7061"):
        self.base_url = base_url

    def execute_mcp_tool(self, server_id, tool_name, args):
        return requests.post(
            f"{self.base_url}/v1/protocols/execute",
            json={
                "protocolType": "mcp",
                "serverId": server_id,
                "toolName": tool_name,
                "arguments": args
            }
        ).json()

client = HelixAgentClient()
result = client.execute_mcp_tool(
    "filesystem-tools",
    "read_file",
    {"path": "/etc/hosts"}
)
```

---

## Slide 25: Environment Variables

**Protocol Configuration:**

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

# Embeddings Configuration
EMBEDDINGS_PROVIDER=openai
EMBEDDINGS_MODEL=text-embedding-ada-002
```

---

## Slide 26: Security

**Protocol Security:**

| Aspect | Implementation |
|--------|----------------|
| Authentication | JWT/API Key |
| Authorization | RBAC |
| Input Validation | Sanitization |
| Rate Limiting | Per-protocol limits |
| Timeouts | Multi-level |

---

## Slide 27: Hands-On Lab

**Lab Exercise 8.1: Protocol Integration**

Tasks:
1. Configure an MCP server
2. Execute MCP tools via API
3. Set up embedding generation
4. Compare text similarity
5. Monitor protocol metrics

Time: 25 minutes

---

## Slide 28: Module Summary

**Key Takeaways:**

- 4 protocols supported: MCP, LSP, ACP, Embeddings
- Unified Protocol Manager for single interface
- MCP for tool execution
- LSP for code intelligence
- Embeddings for semantic search
- Comprehensive monitoring and health checks

**Next: Module 9 - Optimization Features**

---

## Speaker Notes

### Slide 3 Notes
Explain each protocol's purpose briefly. MCP is most commonly used, embeddings are essential for semantic search.

### Slide 9 Notes
Demonstrate MCP tool execution live. Show reading a file and returning its contents.

### Slide 20 Notes
Show practical example of similarity comparison. This is useful for duplicate detection, clustering, etc.
