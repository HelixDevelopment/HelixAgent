# Video Course 05: Protocol Integration Mastery

## Course Overview

**Duration:** 3 hours
**Level:** Intermediate to Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 02 (AI Debate)

Learn how to leverage MCP, LSP, and ACP protocols to build powerful integrations with HelixAgent's AI ensemble system.

---

## Module 1: Model Context Protocol (MCP) Deep Dive

### Video 1.1: Introduction to MCP (15 min)

**Topics:**
- What is MCP and why it matters
- MCP architecture overview
- Tool vs Resource concepts
- Security model

**Demo:**
```bash
# List available MCP tools
curl http://localhost:8080/v1/mcp/tools | jq

# Execute a simple tool
curl -X POST http://localhost:8080/v1/mcp/tools/read_file \
  -d '{"path": "./README.md"}'
```

### Video 1.2: Building Custom MCP Tools (25 min)

**Topics:**
- Tool schema definition
- Input validation
- Output formatting
- Error handling

**Code Example:**
```go
// internal/mcp/tools/custom_analyzer.go
type AnalyzerTool struct {
    Name        string
    Description string
    Parameters  ToolParameters
}

func (t *AnalyzerTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
    filePath := params["path"].(string)

    // Analyze file...
    analysis := analyzeFile(filePath)

    return &ToolResult{
        Content: analysis,
        Metadata: map[string]interface{}{
            "lines_analyzed": analysis.LineCount,
            "issues_found":   len(analysis.Issues),
        },
    }, nil
}
```

### Video 1.3: MCP Resources and Context (20 min)

**Topics:**
- Resource types and templates
- Context injection
- Resource caching
- Dynamic resource discovery

**Demo:**
```yaml
# config/mcp-resources.yaml
resources:
  - name: project-context
    type: file-tree
    uri: "file://./src/**/*.go"
    refresh_interval: 60s

  - name: documentation
    type: markdown
    uri: "file://./docs/**/*.md"
```

---

## Module 2: Language Server Protocol (LSP) Integration

### Video 2.1: LSP Fundamentals (20 min)

**Topics:**
- LSP message types (request/response/notification)
- Initialization handshake
- Capability negotiation
- Document synchronization

**Architecture:**
```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   HelixAgent    │────▶│   LSP Manager   │────▶│ Language Server │
│   (Client)      │◀────│   (Proxy)       │◀────│   (gopls, etc)  │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

### Video 2.2: Code Intelligence Features (25 min)

**Topics:**
- Completion providers
- Hover information
- Go to definition
- Find references
- Code actions

**Demo:**
```bash
# Get completions
curl -X POST http://localhost:8080/v1/lsp/completions \
  -H "Content-Type: application/json" \
  -d '{
    "document": {"uri": "file:///project/main.go"},
    "position": {"line": 10, "character": 5}
  }'

# Go to definition
curl -X POST http://localhost:8080/v1/lsp/definition \
  -H "Content-Type: application/json" \
  -d '{
    "document": {"uri": "file:///project/main.go"},
    "position": {"line": 25, "character": 12}
  }'
```

### Video 2.3: Multi-Language Support (20 min)

**Topics:**
- Configuring multiple language servers
- Language detection
- Root marker files
- Workspace management

**Configuration:**
```yaml
lsp:
  servers:
    go:
      command: gopls
      args: ["serve"]
      root_markers: ["go.mod", "go.work"]
    python:
      command: pylsp
      root_markers: ["pyproject.toml", "setup.py", "requirements.txt"]
    typescript:
      command: typescript-language-server
      args: ["--stdio"]
      root_markers: ["package.json", "tsconfig.json"]
```

---

## Module 3: Agent Communication Protocol (ACP)

### Video 3.1: ACP Architecture (20 min)

**Topics:**
- Agent registration and discovery
- Message types and routing
- Capability advertisement
- Session management

**Architecture:**
```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Agent A       │────▶│   ACP Hub       │────▶│   Agent B       │
│ (Code Reviewer) │◀────│   (HelixAgent)  │◀────│ (Test Runner)   │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

### Video 3.2: Building Custom Agents (30 min)

**Topics:**
- Agent interface implementation
- Message handling
- Capability declaration
- Health reporting

**Code Example:**
```go
// agents/code_reviewer/main.go
type CodeReviewAgent struct {
    ID           string
    Capabilities []string
}

func (a *CodeReviewAgent) HandleMessage(ctx context.Context, msg *Message) (*Response, error) {
    switch msg.Type {
    case "review_request":
        return a.reviewCode(ctx, msg.Payload)
    case "capability_query":
        return a.getCapabilities()
    default:
        return nil, fmt.Errorf("unknown message type: %s", msg.Type)
    }
}

func (a *CodeReviewAgent) reviewCode(ctx context.Context, payload []byte) (*Response, error) {
    var req ReviewRequest
    json.Unmarshal(payload, &req)

    // Perform code review...
    findings := a.analyze(req.FilePath)

    return &Response{
        Type: "review_result",
        Data: findings,
    }, nil
}
```

### Video 3.3: Multi-Agent Workflows (25 min)

**Topics:**
- Orchestrating agent pipelines
- Parallel vs sequential execution
- Result aggregation
- Error handling and fallbacks

**Demo:**
```bash
# Create multi-agent workflow
curl -X POST http://localhost:8080/v1/acp/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "name": "code-quality-pipeline",
    "stages": [
      {
        "agent": "static-analyzer",
        "input": {"files": ["src/**/*.go"]}
      },
      {
        "agent": "security-scanner",
        "depends_on": ["static-analyzer"]
      },
      {
        "agent": "test-runner",
        "parallel": true
      }
    ]
  }'
```

---

## Module 4: Protocol Integration with AI Debate

### Video 4.1: Enhancing Debates with Protocols (25 min)

**Topics:**
- MCP tools in debate context
- LSP-assisted code analysis
- Agent collaboration in debates
- Context injection strategies

**Demo:**
```bash
# Start debate with protocol access
curl -X POST http://localhost:8080/v1/debates \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Analyze and improve this function",
    "protocol_context": {
      "mcp": {
        "tools": ["read_file", "search_files"],
        "resources": ["project-context"]
      },
      "lsp": {
        "enabled": true,
        "features": ["completions", "diagnostics"]
      }
    },
    "file": "/project/src/handler.go",
    "line_range": [100, 150]
  }'
```

### Video 4.2: Real-time Protocol Events (20 min)

**Topics:**
- SSE for protocol events
- WebSocket integration
- Event filtering
- Client-side handling

**Demo:**
```javascript
// Subscribe to protocol events
const eventSource = new EventSource('/v1/events/protocols');

eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);

    switch(data.type) {
        case 'mcp.tool_executed':
            console.log(`Tool ${data.tool} completed in ${data.duration_ms}ms`);
            break;
        case 'lsp.diagnostics':
            updateDiagnostics(data.diagnostics);
            break;
        case 'acp.agent_response':
            handleAgentResponse(data.agent, data.response);
            break;
    }
};
```

---

## Module 5: Advanced Topics

### Video 5.1: Protocol Security (20 min)

**Topics:**
- Tool sandboxing
- Path restrictions
- Agent authentication
- Audit logging

### Video 5.2: Performance Optimization (20 min)

**Topics:**
- Caching strategies
- Connection pooling
- Batch operations
- Timeout management

### Video 5.3: Troubleshooting Protocol Issues (15 min)

**Topics:**
- Debugging MCP tools
- LSP server diagnostics
- Agent connectivity issues
- Common error patterns

---

## Hands-on Labs

### Lab 1: Build a Custom MCP Tool
Create a tool that analyzes Go code for common patterns.

### Lab 2: Multi-Language LSP Setup
Configure LSP for a polyglot project (Go, Python, TypeScript).

### Lab 3: Agent Pipeline
Build a code quality pipeline with multiple agents.

### Lab 4: Protocol-Enhanced Debate
Create a debate that uses all three protocols for code review.

---

## Resources

- [MCP Specification](https://modelcontextprotocol.io)
- [LSP Specification](https://microsoft.github.io/language-server-protocol/)
- [HelixAgent Protocol Documentation](../user-manuals/07-protocols.md)
- [Sample Code Repository](https://github.com/helix-agent/protocol-examples)

---

## Next Course

[Course 06: Testing Strategies](course-06-testing.md) - Learn comprehensive testing approaches for HelixAgent applications.
