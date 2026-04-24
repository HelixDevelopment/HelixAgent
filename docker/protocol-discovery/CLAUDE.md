# CLAUDE.md - Protocol Discovery Service


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

The Protocol Discovery Service is a standalone HTTP service that provides centralized registry and semantic search for all protocol servers (MCP, LSP, ACP, Embedding) in the HelixAgent ecosystem.

## Architecture

### Design Philosophy

This service follows the **MCP Tool Search** pattern, which requires:
1. All tools must be discoverable via semantic search
2. Tools must have rich metadata (description, aliases, categories)
3. Search must support natural language queries
4. Results must be ranked by relevance

### Key Components

```go
type Registry struct {
    servers map[string]*ServerInfo    // All registered servers
    mu      sync.RWMutex               // Thread-safe access
}

type SearchEngine struct {
    tools     []*ToolSchema            // Indexed tools
    index     map[string][]string      // Inverted index
    tokenizer *Tokenizer               // Text tokenization
}
```

### Data Flow

```
1. Server Registration
   MCP Server → POST /api/v1/servers → Registry → Indexed

2. Tool Search
   Client Query → Search Engine → BM25 + Semantic Scoring → Ranked Results

3. Tool Execution
   Client → Select Best Match → Call MCP Server → Return Result
```

## Code Organization

### Main Components (main.go)

**Registry Management:**
- `RegisterServer()` - Add server to registry
- `DeregisterServer()` - Remove server
- `GetServer()` - Retrieve server info
- `ListServers()` - List all servers

**Search Engine:**
- `SearchTools()` - Main search function
- `calculateBM25()` - Text relevance scoring
- `calculateSemanticScore()` - Semantic similarity
- `tokenize()` - Text tokenization

**HTTP Handlers:**
- `handleSearch()` - GET /api/v1/search
- `handleRegister()` - POST /api/v1/servers
- `handleHealth()` - GET /health
- `handleMetrics()` - GET /metrics

## Key Algorithms

### BM25 Scoring

BM25 is a classic text retrieval scoring function:

```go
score = IDF * (freq * (k1 + 1)) / (freq + k1 * (1 - b + b * docLength/avgDocLength))
```

Parameters:
- `k1 = 1.2` - Controls term frequency saturation
- `b = 0.75` - Controls document length normalization

### Semantic Scoring

Uses word overlap and synonym matching:

```go
score = (matchedWords / totalQueryWords) * weight
```

### Combined Scoring

Final score is weighted combination:

```go
finalScore = 0.4*semantic + 0.35*bm25 + 0.15*exact + 0.1*alias
```

## Implementation Notes

### Thread Safety

All registry operations use `sync.RWMutex`:
- Read operations: `RLock()` / `RUnlock()`
- Write operations: `Lock()` / `Unlock()`

### Memory Management

- Servers stored in-memory only (no persistence)
- Tool index rebuilt on each registration
- Health checker runs in background goroutine

### HTTP Server

Uses standard `net/http` with:
- Graceful shutdown on SIGTERM
- Request timeout middleware
- Structured logging

## Configuration

Configuration via environment variables:

```go
port := getEnv("PORT", "8765")
maxServers := getEnvInt("MAX_SERVERS", 1000)
healthInterval := getEnvDuration("HEALTH_CHECK_INTERVAL", 30*time.Second)
```

## Testing

Tests should cover:

1. **Registry Operations**
   - Register/deregister servers
   - Concurrent access
   - Duplicate server handling

2. **Search Functionality**
   - Exact match queries
   - Semantic queries
   - Edge cases (empty, too long)

3. **HTTP API**
   - All endpoints
   - Error handling
   - JSON parsing

4. **Health Monitoring**
   - Server health checks
   - Unhealthy server removal
   - Metrics accuracy

## Deployment

### Docker

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o protocol-discovery main.go

FROM alpine:latest
COPY --from=builder /app/protocol-discovery /usr/local/bin/
EXPOSE 8765
CMD ["protocol-discovery"]
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: protocol-discovery
spec:
  replicas: 1
  selector:
    matchLabels:
      app: protocol-discovery
  template:
    metadata:
      labels:
        app: protocol-discovery
    spec:
      containers:
      - name: protocol-discovery
        image: helixagent/protocol-discovery:latest
        ports:
        - containerPort: 8765
        env:
        - name: PORT
          value: "8765"
        - name: MAX_SERVERS
          value: "1000"
```

## Monitoring

### Metrics

Prometheus metrics exposed at `/metrics`:

```go
var (
    serversTotal = prometheus.NewGauge(...)
    searchRequests = prometheus.NewCounter(...)
    searchLatency = prometheus.NewHistogram(...)
)
```

### Logging

Structured logging with levels:

```go
log.Printf("[INFO] Server registered: %s", serverID)
log.Printf("[ERROR] Health check failed: %v", err)
log.Printf("[DEBUG] Search query: %s", query)
```

## Security

Current security model (internal service):
- No authentication required
- Runs on internal Docker network
- No sensitive data in registry

Future enhancements:
- mTLS between services
- API key authentication
- Request signing

## Performance

### Optimization Strategies

1. **Indexing**: Pre-built inverted index for fast lookups
2. **Caching**: Cache frequent search results
3. **Concurrent Search**: Parallel scoring for multiple tools
4. **Connection Pooling**: Reuse HTTP connections

### Benchmarks

Target performance:
- Search latency: < 50ms for 1000 tools
- Registration: < 10ms
- Health check: < 100ms per server
- Memory usage: < 500MB for 1000 servers

## Related Documentation

- [MCP Protocol](../../docs/protocols/MCP.md)
- [LSP Protocol](../../docs/protocols/LSP.md)
- [Tool Search Technology](../../docs/features/TOOL_SEARCH.md)
- [Architecture Overview](../../docs/ARCHITECTURE.md)

## Maintenance

### Regular Tasks

- Monitor memory usage
- Review search performance metrics
- Update tool schemas when protocols change
- Clean up stale servers

### Upgrade Path

When adding new protocols:
1. Define protocol type constant
2. Add protocol-specific metadata
3. Update search indexing
4. Document new capabilities

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | root HelixAgent |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
