# CLAUDE.md - pkg/api


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# Compile gRPC-generated code and verify it's current vs. the .proto source.
cd pkg/api && go build ./...
# Optional round-trip — only runs if grpcurl is installed AND ./bin/helixagent
# is serving gRPC on :8112. Either being absent is a skip-not-fail per DoD.
if command -v grpcurl >/dev/null 2>&1 && curl -fsS -m 2 http://localhost:8100/v1/health >/dev/null 2>&1; then
  grpcurl -plaintext -d '{"messages":[{"role":"user","content":"ping"}]}' \
    localhost:8112 api.LLMFacade/Complete | jq -e '.content | length > 0'
else
  echo "SKIP: grpcurl or HelixAgent not available for round-trip; build-only verification green"
fi
```
Expect: `go build` exits 0; `grpcurl` returns a non-empty `content` field. Regenerate the pb.go files from `specs/001-helix-agent/contracts/llm-facade.proto` per `pkg/api/CLAUDE.md`.


## Module Overview

The `pkg/api` module provides the public gRPC API surface for HelixAgent. It contains Protocol Buffer definitions and generated Go code that enable external clients to interact with the ensemble LLM system programmatically.

## Architecture

### Protocol Buffer Organization

```
pkg/api/
├── llm-facade.proto          # Symlink/copy; original lives at ../../specs/001-helix-agent/contracts/llm-facade.proto
├── llm-facade.pb.go          # Generated message types
├── llm-facade_grpc.pb.go     # Generated gRPC client/server interfaces
├── go.mod                    # Module definition
└── README.md                 # User-facing documentation
```

### Key Design Decisions

1. **Separate Module**: API is a separate Go module to allow independent versioning and import by external clients without pulling in the entire HelixAgent dependency tree.

2. **Proto-First**: All API types are defined in Protocol Buffers, ensuring language-agnostic compatibility and forward compatibility.

3. **Streaming Support**: Both unary (Complete) and streaming (CompleteStream) endpoints are provided for flexibility.

4. **Ensemble Configuration**: API allows clients to configure ensemble behavior per-request, enabling dynamic provider selection.

## API Patterns

### Request/Response Flow

```
Client → gRPC Request → HelixAgent → Ensemble Orchestrator → LLM Providers
                                             ↓
Client ← gRPC Response ← Aggregated Result ← Provider Responses
```

### Streaming Pattern

```
Client → StreamRequest → HelixAgent
                              ↓
Client ← Chunk 1 ← Provider 1
Client ← Chunk 2 ← Provider 2
Client ← Chunk 3 ← Provider 1
       ...
Client ← FinalResponse ← Ensemble Aggregation
```

## Key Types

### CompletionRequest

The primary input type supporting:
- Single prompt or multi-message conversations
- Model parameters (temperature, max_tokens, etc.)
- Ensemble configuration (providers, strategy, voting)
- Memory context (session_id, memory_enhanced flag)
- Metadata for tracing and observability

### CompletionResponse

The primary output type containing:
- Generated content
- Individual provider responses with confidence scores
- Ensemble aggregation method and consensus level
- Usage statistics (tokens, latency)
- Provider metadata (model versions, etc.)

### EnsembleResult

Captures ensemble-specific metadata:
- Aggregation method used
- Consensus level achieved
- Provider rankings
- Confidence distribution

## Implementation Notes

### Generated Code

The `.pb.go` files are generated from Protocol Buffer definitions. **Never edit these files directly.**

Source of truth: `specs/001-helix-agent/contracts/llm-facade.proto` at the HelixAgent repo root. The `.pb.go` files in this directory were last regenerated from that source (check `// source: llm-facade.proto` + commit date in the file header).

Regeneration command (run from the repo root):
```bash
cd pkg/api
protoc --go_out=. --go-grpc_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  -I../../specs/001-helix-agent/contracts \
  ../../specs/001-helix-agent/contracts/llm-facade.proto
```

### Version Compatibility

- Follow semantic versioning for the module
- Protocol Buffer binary format provides forward/backward compatibility
- Add new fields without removing old ones
- Mark deprecated fields with `[deprecated = true]`

### Security Considerations

- API supports TLS/mTLS for transport security
- Authentication via API keys or JWT tokens (configured at server level)
- Input validation happens at the server, not in this package
- This package only defines types and client/server interfaces

## Integration Points

### Server Implementation

The server implementation is in `internal/handlers/grpc_server.go`:

```go
// Implements api.LLMFacadeServer interface
type GRPCServer struct {
    api.UnimplementedLLMFacadeServer
    ensemble *services.EnsembleService
    memory   *services.MemoryService
}
```

### Client Usage

External clients import this package:

```go
import api "dev.helix.agent/pkg/api"

func main() {
    conn, _ := grpc.Dial("localhost:8112", ...)
    client := api.NewLLMFacadeClient(conn)
    // ... use client
}
```

## Testing

### Unit Tests

This package contains only generated code, so no unit tests are needed here. Tests for the API behavior are in:
- `tests/e2e/` - End-to-end API tests
- `tests/integration/` - Integration tests
- `internal/handlers/grpc_server_test.go` - Server implementation tests

### Compatibility Testing

When modifying the proto definition:
1. Test backward compatibility with older clients
2. Test forward compatibility (new clients to old servers)
3. Verify all language bindings (if multi-language support exists)

## Best Practices

### For API Users

1. **Reuse Connections**: gRPC connections are expensive to create. Reuse them.
2. **Use Streaming**: For long completions, use CompleteStream for better UX.
3. **Set Timeouts**: Always set appropriate context timeouts.
4. **Handle Errors**: Check for specific gRPC status codes.

### For API Developers

1. **Don't Break Compatibility**: Never remove or change existing fields.
2. **Document Changes**: Update README.md when adding features.
3. **Version Properly**: Use semantic versioning for the module.
4. **Test Thoroughly**: Ensure all generated code compiles correctly.

## Common Issues

### Import Cycle

If you get import cycle errors when importing this package:
- Ensure you're not importing from internal packages
- This package should only depend on standard library and protobuf

### Proto Generation Issues

If `protoc` generation fails:
- Ensure protoc >= 3.15.0
- Install protoc-gen-go: `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
- Install protoc-gen-go-grpc: `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`

## Related Documentation

- [API Reference](../../docs/API_REFERENCE.md) - Full API documentation
- [gRPC Best Practices](https://grpc.io/docs/guides/best-practices/) - gRPC guidelines
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers/docs/gotutorial) - Proto guide

## Development Workflow

1. **Modify Proto**: Edit `llm-facade.proto` (stored separately or in docs/)
2. **Generate Code**: Run protoc generation command
3. **Update Documentation**: Update README.md with new features
4. **Version Bump**: Update go.mod version tag
5. **Test**: Run integration tests
6. **Commit**: Include both .proto and generated .go files

## Maintenance

### Regular Tasks

- Update protobuf dependencies quarterly
- Review and update documentation when features change
- Monitor for protobuf security advisories
- Keep generated code up to date with latest protoc versions

### Deprecation Strategy

When deprecating fields:
1. Mark field as deprecated in .proto: `[deprecated = true]`
2. Add comment explaining replacement
3. Maintain for at least 2 major versions
4. Log warnings when deprecated fields are used
5. Eventually remove in major version bump

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none (public gRPC surface) |
| Downstream (these import this module) | external clients (out of repo) |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
