# CLAUDE.md - ACP Manager


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

<!-- TODO: replace this block with the exact command(s) that exercise this
     module end-to-end against real dependencies, and the expected output.
     The commands must run the real artifact (built binary, deployed
     container, real service) — no in-process fakes, no mocks, no
     `httptest.NewServer`, no Robolectric, no JSDOM as proof of done. -->

```bash
# TODO
```

## Module Overview

The ACP Manager implements the **Agent Communication Protocol** (ACP), a lightweight HTTP-based protocol for agent discovery and inter-agent communication. It serves as the central hub that connects various CLI agents with HelixAgent.

## Architecture

### Design Philosophy

ACP follows these principles:
1. **Simplicity**: Minimal complexity, easy to understand and debug
2. **RESTful**: HTTP-based for universal compatibility
3. **Capability-based**: Agents advertise what they can do
4. **Pre-registration**: Common agents pre-configured for zero-config setup

### System Context

```
┌─────────────────┐
│  CLI Agents     │
│  (Claude Code,  │
│   OpenCode,     │
│   Cline, etc.)  │
└────────┬────────┘
         │ HTTP
         ▼
┌─────────────────┐
│  ACP Manager    │
│  (Port 8766)    │
└────────┬────────┘
         │ HTTP
         ▼
┌─────────────────┐
│  HelixAgent     │
│  Services       │
└─────────────────┘
```

## Code Organization

### Main Components (main.go)

**Data Types:**
- `Agent` - Represents a registered agent
- `Message` - Inter-agent communication message
- `AgentRegistry` - Thread-safe agent storage

**Global Registry:**
```go
var registry = &AgentRegistry{
    agents: make(map[string]Agent),
}
```

**Pre-registered Agents:**
```go
var preregisteredAgents = []Agent{
    {ID: "claude-code", Name: "Claude Code", ...},
    {ID: "opencode", Name: "OpenCode", ...},
    // ... more agents
}
```

**HTTP Handlers:**
- `handleHealth()` - Health check endpoint
- `handleAgents()` - List/Get agents
- `handleAgentDetail()` - Get specific agent
- `handleMessages()` - Message routing

## Key Design Decisions

### In-Memory Storage

Agents are stored in-memory only:
- **Pros**: Fast, simple, no persistence needed
- **Cons**: Data lost on restart
- **Rationale**: Agents re-register on startup, data is ephemeral

### Pre-registration

11 CLI agents pre-registered:
- **Zero Configuration**: Works out of the box
- **Known Agents**: Common CLI agents recognized immediately
- **Extensible**: New agents can still register dynamically

### Capability System

Agents advertise capabilities as string tags:
```go
Capabilities: []string{"code", "chat", "tools"}
```

This allows:
- **Discovery**: Find agents by what they can do
- **Routing**: Send tasks to appropriate agents
- **Filtering**: Filter agents by required capabilities

## Implementation Details

### Thread Safety

Registry uses `sync.RWMutex`:
```go
type AgentRegistry struct {
    agents map[string]Agent
    mu     sync.RWMutex
}
```

**Read Operations:**
```go
registry.mu.RLock()
defer registry.mu.RUnlock()
return registry.agents[id]
```

**Write Operations:**
```go
registry.mu.Lock()
defer registry.mu.Unlock()
registry.agents[id] = agent
```

### HTTP Server

Standard library HTTP server with minimal middleware:

```go
mux := http.NewServeMux()
mux.HandleFunc("/health", handleHealth)
mux.HandleFunc("/agents", handleAgents)
mux.HandleFunc("/agents/", handleAgentDetail)
mux.HandleFunc("/messages", handleMessages)

server := &http.Server{
    Addr:    ":" + port,
    Handler: mux,
}
```

### Initialization

Pre-registered agents loaded at startup:

```go
func init() {
    for _, agent := range preregisteredAgents {
        agent.Status = "registered"
        agent.LastSeen = time.Now()
        registry.agents[agent.ID] = agent
    }
}
```

## API Design

### RESTful Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/agents` | List all agents |
| GET | `/agents/{id}` | Get specific agent |
| POST | `/agents` | Register new agent |
| GET | `/agents?capability=X` | Filter by capability |
| POST | `/messages` | Send message |

### Response Format

All responses are JSON:

```go
// Success
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(data)

// Error
w.WriteHeader(http.StatusNotFound)
json.NewEncoder(w).Encode(map[string]string{
    "error": "Agent not found",
})
```

## Testing

### Unit Tests

Test each handler:

```go
func TestHandleHealth(t *testing.T) {
    req := httptest.NewRequest("GET", "/health", nil)
    w := httptest.NewRecorder()
    
    handleHealth(w, req)
    
    if w.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", w.Code)
    }
}
```

### Integration Tests

Test full flow:

```go
func TestAgentRegistration(t *testing.T) {
    // Start server
    go main()
    time.Sleep(100 * time.Millisecond)
    
    // Register agent
    resp, _ := http.Post("http://localhost:8766/agents", ...)
    
    // Verify
    if resp.StatusCode != http.StatusCreated {
        t.Errorf("Registration failed")
    }
}
```

## Configuration

### Environment Variables

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8766"
}
```

### Docker Configuration

```dockerfile
FROM golang:1.24-alpine
WORKDIR /app
COPY . .
RUN go build -o acp-manager .
EXPOSE 8766
CMD ["./acp-manager"]
```

## Deployment

### Docker

```bash
docker build -t helixagent/acp-manager .
docker run -d -p 8766:8766 helixagent/acp-manager
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: acp-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: acp-manager
  template:
    metadata:
      labels:
        app: acp-manager
    spec:
      containers:
      - name: acp-manager
        image: helixagent/acp-manager:latest
        ports:
        - containerPort: 8766
```

## Monitoring

### Health Checks

Simple health endpoint:
```go
func handleHealth(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "healthy",
        "agents": len(registry.agents),
    })
}
```

### Future Metrics

Consider adding:
- Request count/latency
- Agent registration rate
- Message throughput
- Error rates

## Security Considerations

### Current Security Model

- **Internal Service**: Designed for internal network use
- **No Authentication**: Simplifies integration
- **No Encryption**: Relies on network isolation

### Future Security Enhancements

1. **API Key Authentication**: Simple key-based auth
2. **mTLS**: Mutual TLS for service-to-service
3. **Rate Limiting**: Prevent abuse
4. **Audit Logging**: Track all operations

## Integration with HelixAgent

### How HelixAgent Uses ACP

1. **Agent Discovery**: Query `/agents` to find available CLI agents
2. **Capability Matching**: Filter agents by required capabilities
3. **Message Routing**: Send tasks to appropriate agents
4. **Status Monitoring**: Check agent health and availability

### Example Workflow

```go
// HelixAgent discovers agents for code review
resp, _ := http.Get("http://acp-manager:8766/agents?capability=code")
var result struct {
    Agents []Agent `json:"agents"`
}
json.NewDecoder(resp.Body).Decode(&result)

// Select best agent (e.g., Claude Code)
selectedAgent := result.Agents[0]

// Send code review request
message := map[string]any{
    "from": "helixagent-debate",
    "to":   selectedAgent.ID,
    "type": "request",
    "payload": map[string]any{
        "action": "review_code",
        "code":   code,
    },
}
// ... send message
```

## Maintenance

### Regular Tasks

- Monitor agent health
- Update pre-registered agent list
- Review capability tags for consistency
- Monitor for memory leaks (in-memory storage)

### Adding New Pre-registered Agents

1. Add to `preregisteredAgents` slice:
```go
{ID: "new-agent", Name: "New Agent", Type: "cli-agent", ...}
```

2. Rebuild and redeploy
3. Update documentation

## Related Documentation

- [ACP Protocol Specification](../../docs/protocols/ACP.md)
- [Agent Integration Guide](../../docs/guides/AGENT_INTEGRATION.md)
- [HelixAgent Architecture](../../docs/ARCHITECTURE.md)

## Future Roadmap

### Short Term

- Add more CLI agents to pre-registration
- Implement message queue for reliability
- Add basic metrics endpoint

### Long Term

- WebSocket support for real-time communication
- Agent authentication and authorization
- Multi-region agent discovery
- Integration with service mesh

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | root HelixAgent |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
