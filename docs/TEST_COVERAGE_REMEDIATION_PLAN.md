# Test Coverage Remediation Plan

## Objective

Achieve 100% test coverage across all 20 extracted modules as required by the Mandatory Development Standards in CLAUDE.md.

## Current State

- **Total Modules**: 20
- **Modules at 100%**: 0
- **Average Coverage**: ~87%
- **Critical Packages (0-50%)**: 7
- **High Priority Packages (50-80%)**: 5

---

## Phase 1: Critical Priority (0-50% Coverage)

### 1.1 Containers Module - Internal Packages (0%)

**Target**: `containers/internal/exec/command.go` and `containers/internal/platform/detect.go`

**Files to Create**:
- `containers/internal/exec/command_test.go`
- `containers/internal/platform/detect_test.go`

**Functions Requiring Tests**:
```
internal/exec/command.go:
- Run(ctx, name, args...) (string, error)
- RunInDir(ctx, dir, name, args...) (string, error)

internal/platform/detect.go:
- IsLinux() bool
- IsDarwin() bool
- IsWindows() bool
```

**Test Strategy**:
- Use `exec.Command` mocking via interface injection
- Test platform detection with build tags for cross-platform validation
- Table-driven tests for various command scenarios

**Estimated Tests**: 12-15 tests

---

### 1.2 Database Module - PostgreSQL (47.7%)

**Target**: `Database/pkg/postgres/postgres.go`

**Missing Coverage**:
- Connection pool error handling
- Transaction rollback paths
- Query execution errors
- Prepared statement failures

**Test Strategy**:
- Use `pgxmock` for database mocking
- Test connection failures, timeouts, query errors
- Add transaction edge cases (commit fail, rollback)

**Functions Requiring Tests**:
```go
- NewPostgresDB(config) - connection error path
- Query(ctx, sql, args) - scan errors, no rows
- Exec(ctx, sql, args) - execution failures
- BeginTx(ctx) - transaction start failures
- Commit/Rollback - failure paths
```

**Estimated Tests**: 25-30 tests

---

### 1.3 Storage Module - S3 (47.9%)

**Target**: `Storage/pkg/s3/s3.go`

**Missing Coverage**:
- AWS SDK error handling
- Multipart upload failures
- Presigned URL generation errors
- Bucket operations (create, delete, list)

**Test Strategy**:
- Mock AWS S3 client interface
- Test all error paths from AWS SDK
- Add timeout and context cancellation tests

**Functions Requiring Tests**:
```go
- Upload(ctx, bucket, key, reader) - multipart errors
- Download(ctx, bucket, key) - object not found
- Delete(ctx, bucket, key) - permission denied
- ListObjects(ctx, bucket, prefix) - pagination errors
- CreateBucket/DeleteBucket - already exists, not empty
- GeneratePresignedURL - signing errors
```

**Estimated Tests**: 30-35 tests

---

### 1.4 Containers Module - Compose (49.4%)

**Target**: `containers/pkg/compose/orchestrator.go`

**Missing Coverage**:
- `NewDefaultOrchestrator` - 0%
- `Up`, `Down`, `Status`, `Logs` - 0%
- `run`, `output`, `detectComposeCmd` - 0%

**Test Strategy**:
- Mock `exec.Command` via interface
- Test compose command construction
- Test output parsing for various Docker Compose versions

**Functions Requiring Tests**:
```go
- NewDefaultOrchestrator() - compose detection
- Up(ctx, services...) - command execution, errors
- Down(ctx) - graceful shutdown
- Status(ctx) - parse running/stopped states
- Logs(ctx, service) - streaming logs
- detectComposeCmd() - docker-compose vs docker compose
```

**Estimated Tests**: 20-25 tests

---

### 1.5 Containers Module - Discovery (42.9%)

**Target**: `containers/pkg/discovery/dns.go`

**Missing Coverage**:
- `NewDNSDiscoverer` - 0%
- `Discover` (DNS) - 0%

**Test Strategy**:
- Mock DNS resolver
- Test SRV record parsing
- Test fallback mechanisms

**Functions Requiring Tests**:
```go
- NewDNSDiscoverer(config) - configuration validation
- Discover(ctx, service) - DNS lookup, SRV records
- parseSRVRecords - record parsing
```

**Estimated Tests**: 12-15 tests

---

### 1.6 Challenges Module - Monitor (53.1%)

**Target**: `challenges/pkg/monitor/websocket.go`

**Missing Coverage**:
- `NewWebSocketServer` - 0%
- `Start`, `Stop` - 0%
- `handleSSE`, `handleDashboard`, `broadcast` - 0%
- `BuildDashboardData` - 0%

**Test Strategy**:
- Use `httptest` for HTTP server testing
- Use `gorilla/websocket` test utilities
- Test SSE event streaming

**Functions Requiring Tests**:
```go
- NewWebSocketServer(config) - initialization
- Start(ctx) - server startup
- Stop() - graceful shutdown
- handleSSE(w, r) - SSE event streaming
- handleDashboard(w, r) - dashboard HTML
- broadcast(event) - event distribution
- BuildDashboardData(results) - data aggregation
```

**Estimated Tests**: 18-22 tests

---

### 1.7 Cache Module - Redis (62.0%)

**Target**: `Cache/pkg/redis/redis.go`

**Missing Coverage**:
- Connection error handling
- Serialization errors
- Pipeline execution failures
- Cluster mode operations

**Test Strategy**:
- Use `miniredis` for in-memory Redis testing
- Test connection failures, timeouts
- Test serialization edge cases

**Functions Requiring Tests**:
```go
- Get(ctx, key) - deserialize errors, key not found
- Set(ctx, key, value, ttl) - serialize errors, connection failures
- Delete(ctx, keys...) - partial failures
- Pipeline operations - batch errors
- Cluster operations - node failures
```

**Estimated Tests**: 20-25 tests

---

## Phase 2: High Priority (50-80% Coverage)

### 2.1 Containers Module - Logging (58.3%)

**Target**: `containers/pkg/logging/logger.go`, `noop.go`

**Missing Coverage**:
- `NewStdLogger` - 0%
- All `StdLogger` methods - 0%
- All `PrefixLogger` methods - 0%
- All `NoopLogger` methods - 0%

**Test Strategy**:
- Capture log output to buffer
- Verify log format and levels
- Test prefix propagation

**Estimated Tests**: 15-18 tests

---

### 2.2 Challenges Module - Logging (78.8%)

**Target**: `challenges/pkg/logging/multi_logger.go`, `null_logger.go`

**Missing Coverage**:
- `NewMultiLogger` - 0%
- All `MultiLogger` methods - 0%
- All `NullLogger` methods - 0%
- `RedactingLogger.Debug` - 0%

**Test Strategy**:
- Test multi-logger delegation
- Verify null logger no-ops
- Test field redaction

**Estimated Tests**: 20-24 tests

---

### 2.3 Containers Module - Monitor (77.6%)

**Target**: `containers/pkg/monitor/container.go`, `system.go`

**Missing Coverage**:
- `NewContainerCollector` - 0%
- `CollectAll`, `Collect` - 0%
- Linux-specific collection paths

**Test Strategy**:
- Mock proc filesystem reads
- Test metric parsing
- Platform-specific build tags

**Estimated Tests**: 15-18 tests

---

### 2.4 Optimization Module - Outlines (80.2%)

**Target**: `Optimization/pkg/outlines/outlines.go`

**Missing Coverage**:
- `Schema.String` - 0%
- `SchemaBuilder.StringType/NumberType/IntegerType/BooleanType` - 0%
- `SchemaBuilder.EnumValues`, `SetPattern` - 0%
- `validateObject` additional properties
- `extractJSON` array extraction

**Test Strategy**:
- Test all schema builder methods
- Add validation edge cases
- Test JSON extraction patterns

**Estimated Tests**: 25-30 tests

---

### 2.5 EventBus Module - Bus (80.9%)

**Target**: `EventBus/pkg/bus/bus.go`

**Missing Coverage**:
- Concurrent publish/subscribe
- Handler panic recovery
- Subscription cleanup on close

**Test Strategy**:
- Race condition tests
- Panic recovery verification
- Resource cleanup tests

**Estimated Tests**: 12-15 tests

---

## Phase 3: Medium Priority (80-90% Coverage)

### 3.1 Concurrency Module - Pool (88.7%)

**Functions Requiring Tests**:
- Worker panic recovery
- Pool resize during operation
- Task cancellation propagation

**Estimated Tests**: 8-10 tests

---

### 3.2 VectorDB Module - Milvus (87.2%)

**Functions Requiring Tests**:
- Connection retry logic
- Collection schema validation errors
- Search with filters edge cases

**Estimated Tests**: 10-12 tests

---

### 3.3 Embeddings Module - Bedrock (89.4%)

**Functions Requiring Tests**:
- `embedTitan` error paths
- `embedCohere` API error handling
- Batch embedding failures

**Estimated Tests**: 8-10 tests

---

### 3.4 MCP Module - Adapter (86.7%)

**Functions Requiring Tests**:
- Docker adapter Start/Stop errors
- HTTP adapter health check failures
- Stdio adapter process signal errors

**Estimated Tests**: 12-15 tests

---

### 3.5 Formatters Module - Service (85.5%)

**Functions Requiring Tests**:
- HTTP request creation errors
- Response body read errors
- Health check JSON parsing errors

**Estimated Tests**: 10-12 tests

---

### 3.6 Cache Module - Distributed (84.0%)

**Functions Requiring Tests**:
- Consistency protocol failures
- Node communication errors
- Split-brain scenarios

**Estimated Tests**: 12-15 tests

---

### 3.7 Database Module - SQLite (84.9%)

**Functions Requiring Tests**:
- File permission errors
- Concurrent write conflicts
- Migration rollback failures

**Estimated Tests**: 10-12 tests

---

### 3.8 Observability Module - Health (83.3%)

**Functions Requiring Tests**:
- `TCPCheck` function (currently 0%)
- `buildReport` degraded status paths

**Estimated Tests**: 8-10 tests

---

### 3.9 Optimization Module - Adapter (83.0%)

**Functions Requiring Tests**:
- Nil config handling
- HTTP request/decode errors
- Default parameter branches

**Estimated Tests**: 10-12 tests

---

### 3.10 Streaming Module - WebSocket (81.9%)

**Functions Requiring Tests**:
- Connection upgrade failures
- Ping/pong timeout handling
- Room broadcast errors

**Estimated Tests**: 12-15 tests

---

## Phase 4: Low Priority (90-99% Coverage)

These packages need only a few additional tests to reach 100%:

| Package | Current | Tests Needed |
|---------|---------|--------------|
| Memory/pkg/store | 98.9% | 2-3 |
| challenges/pkg/assertion | 99.2% | 1-2 |
| Formatters/pkg/registry | 97.0% | 3-4 |
| Formatters/pkg/executor | 96.9% | 3-4 |
| Memory/pkg/mem0 | 93.3% | 5-6 |
| RAG/pkg/chunker | 90.8% | 4-5 |
| Auth/pkg/oauth | 90.8% | 4-5 |
| VectorDB/pkg/pinecone | 90.6% | 4-5 |
| Formatters/pkg/native | 90.3% | 5-6 |

---

## Implementation Strategy

### Testing Patterns to Follow

1. **Table-Driven Tests**: All new tests should use table-driven format
2. **Testify Assertions**: Use `require` for setup, `assert` for verification
3. **Interface Mocking**: Create interfaces for external dependencies
4. **Build Tags**: Use `// +build integration` for infrastructure-dependent tests
5. **Race Detection**: All tests must pass with `-race` flag

### Mock Libraries

```go
// Recommended mocking approaches per module:
- Database: github.com/pashagolub/pgxmock/v4
- Redis: github.com/alicebob/miniredis/v2
- HTTP: net/http/httptest
- S3: Interface mocking (no external lib needed)
- Docker: Interface mocking with exec.Command wrapper
```

### Test File Naming

```
<package>_test.go          # Unit tests
<package>_integration_test.go  # Integration tests (build tagged)
<package>_bench_test.go    # Benchmarks
```

---

## Execution Timeline

| Phase | Packages | Estimated Tests | Priority |
|-------|----------|-----------------|----------|
| Phase 1 | 7 packages | ~150 tests | Critical |
| Phase 2 | 5 packages | ~95 tests | High |
| Phase 3 | 10 packages | ~100 tests | Medium |
| Phase 4 | 9 packages | ~35 tests | Low |
| **Total** | **31 packages** | **~380 tests** | - |

---

## Verification

After each phase, run:

```bash
# Per module verification
cd <Module> && go test -v -count=1 -race -cover ./...

# Full suite verification
for mod in EventBus Concurrency Observability Auth Storage Streaming \
           Security VectorDB Embeddings Database Cache \
           Messaging Formatters MCP_Module RAG Memory Optimization Plugins \
           Containers Challenges; do
  echo "Testing $mod..."
  (cd $mod && go test ./... -count=1 -race -cover)
done
```

---

## Success Criteria

- [ ] All 20 modules at 100% statement coverage
- [ ] All tests pass with `-race` flag
- [ ] No skipped or disabled tests
- [ ] Integration tests tagged and documented
- [ ] Coverage reports generated for each module
