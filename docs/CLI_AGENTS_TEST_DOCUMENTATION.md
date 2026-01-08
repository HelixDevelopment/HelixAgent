# CLI Agents Test Documentation

## Overview

This document provides comprehensive documentation for the CLI Agents testing infrastructure in HelixAgent/HelixAgent. The test suite validates streaming response integrity, content quality, and compatibility across multiple CLI AI agents.

## Supported CLI Agents

| Agent | Location | Build Status | Test Coverage |
|-------|----------|--------------|---------------|
| **HelixCode** | `/Projects/HelixCode/HelixCode/` | Pre-built binary available | Full |
| **OpenCode** | `/Projects/HelixCode/Example_Projects/OpenCode/OpenCode/` | Built via `go build` | Full |
| **Cline** | `/Projects/HelixCode/Example_Projects/Cline/` | Requires gRPC generation | HelixAgent proxy |
| **Bear-Mail** (test target) | `/Projects/Bear-Mail/` | N/A (test project) | Full |

## Test Files

### 1. `tests/integration/cli_agents_test.go`

Primary test file for CLI agents with the following test suites:

#### HelixCode Tests
- `TestHelixCodeStreamingIntegrity`
  - `No_Word_Duplication` - Validates no content interleaving
  - `Coherent_Sentence_Structure` - Validates proper sentence completion
  - `Consistent_Stream_ID` - Validates stream ID consistency across chunks
- `TestHelixCodeCodebaseAnalysis`
  - `Project_Structure_Analysis` - Tests codebase analysis capabilities
  - `Build_Commands_Request` - Tests build command extraction

#### OpenCode Tests
- `TestOpenCodeStreamingIntegrity`
  - `No_Word_Duplication` - Validates no content interleaving
  - `Long_Response_No_Cutoff` - Validates long responses complete properly
  - `SSE_Format_Validity` - Validates proper SSE streaming format
- `TestOpenCodeBearMailAnalysis`
  - `Codebase_Visibility` - Tests codebase awareness

#### Cline Tests
- `TestClineStreamingIntegrity`
  - `No_Word_Duplication_Cline_Style` - Validates no interleaving with Cline persona
  - `Code_Generation_Task` - Validates code generation capabilities

#### Cross-Agent Tests
- `TestCrossAgentConsistency`
  - `Same_Query_Different_Agent_Contexts` - Validates consistent responses across agents
- `TestToolCallFormatAcrossAgents`
  - `Bash_Command_Format` - Validates bash command output
  - `File_Write_Format` - Validates file write instructions
  - `No_Incomplete_Tags` - Validates no malformed tags
- `TestResponseValidityAllAgents`
  - `No_Empty_Response` - Validates non-empty responses
  - `Response_Has_Expected_Content` - Validates expected content
  - `Numbered_List_Response` - Validates list formatting
- `TestNoResponseCutoffAllAgents`
  - `Medium_Response_Completes` - Validates medium responses complete
  - `Long_Response_Completes` - Validates long responses complete

### 2. `tests/integration/bearmail_opencode_simulation_test.go`

Bear-Mail specific OpenCode simulation tests:

- `TestBearMailOpenCodeConversation`
  - 4-step conversation simulation
- `TestBearMailContentQuality`
  - No hallucinated structure validation
- `TestBearMailResponseCompleteness`
  - Long response completion validation
- `TestBearMailMultiProviderParticipation`
  - Multi-provider debate validation
- `TestBearMailStreamingContentIntegrity`
  - Content interleaving detection
- `TestBearMailStreamingFormatValidity`
  - SSE format validation

## Test Infrastructure

### Helper Functions

```go
// Base URL retrieval
func cliAgentGetBaseURL() string
func bearMailGetBaseURL() string

// Skip conditions
func cliAgentSkipIfNotRunning(t *testing.T)
func bearMailSkipIfNotRunning(t *testing.T)

// Project existence checks
func ensureHelixCodeExists(t *testing.T)
func ensureOpenCodeBuilt(t *testing.T)
func ensureBearMailExists(t *testing.T)

// Request/Response handling
func sendCLIAgentRequest(t *testing.T, baseURL string, reqBody map[string]interface{}, timeout time.Duration) (*CLIAgentTestResponse, error)
func parseCLIAgentResponse(body string) *CLIAgentTestResponse
```

### Response Structure

```go
type CLIAgentTestResponse struct {
    Content              string   // Full response content
    ChunkCount           int      // Number of SSE chunks
    HasDoneMarker        bool     // Whether [DONE] marker present
    HasFinishReason      bool     // Whether finish_reason present
    StreamID             string   // Stream ID from first chunk
    Errors               []string // Parsing errors
    InterleavingDetected bool     // Whether content interleaving detected
    CutoffDetected       bool     // Whether premature cutoff detected
}
```

## Content Interleaving Detection

The test suite detects content interleaving patterns that indicate multi-provider stream merging issues:

```go
interleavingPatterns := []string{
    "YesYes", "NoNo", "HelloHello", "II ", " II",
    "andand", "thethe", "isis", "inin", "toto",
    "TheThe", "IsIs", "InIn", "ToTo",
}
```

## Running Tests

### All CLI Agent Tests
```bash
go test -v -timeout 600s ./tests/integration/... -run "TestHelixCode|TestOpenCode|TestCline|TestCrossAgent|TestToolCallFormat|TestResponseValidity|TestNoResponseCutoff"
```

### Specific Agent Tests
```bash
# HelixCode only
go test -v -timeout 120s ./tests/integration/... -run "TestHelixCode"

# OpenCode only
go test -v -timeout 300s ./tests/integration/... -run "TestOpenCode"

# Cline only
go test -v -timeout 120s ./tests/integration/... -run "TestCline"
```

### Bear-Mail Simulation Tests
```bash
go test -v -timeout 600s ./tests/integration/... -run "TestBearMail"
```

## Test Results Summary

Last comprehensive test run: **ALL PASS** (591.684 seconds)

| Test Suite | Tests | Status | Duration |
|------------|-------|--------|----------|
| TestOpenCodeToolCallFormat | 3 | PASS | 12.67s |
| TestHelixCodeStreamingIntegrity | 3 | PASS | 5.04s |
| TestHelixCodeCodebaseAnalysis | 2 | PASS | 34.63s |
| TestOpenCodeStreamingIntegrity | 3 | PASS | 64.33s |
| TestOpenCodeBearMailAnalysis | 1 | PASS | 7.61s |
| TestClineStreamingIntegrity | 2 | PASS | 7.91s |
| TestCrossAgentConsistency | 3 | PASS | 5.81s |
| TestToolCallFormatAcrossAgents | 3 | PASS | 8.64s |
| TestResponseValidityAllAgents | 3 | PASS | 5.74s |
| TestNoResponseCutoffAllAgents | 2 | PASS | 66.54s |
| TestOpenCodeComprehensiveRequest | 6 | PASS | 127.80s |
| TestOpenCode_* (simulation) | 6 | PASS | ~180s |
| TestOpenCodeConfig* | 15+ | PASS | <1s |

## Key Fixes Applied

### 1. WriteTimeout Fix (SSE Streaming)
**Issue**: Streaming responses timing out after 30 seconds with "unexpected EOF"

**Root Cause**: HTTP server WriteTimeout was 30 seconds, which limits total response time for SSE streaming.

**Fix**: Increased WriteTimeout to 300 seconds (5 minutes):
- `cmd/helixagent/main.go`
- `internal/router/gin_router.go`

```go
server := &http.Server{
    WriteTimeout: 300 * time.Second, // 5 minutes for SSE streaming
}
```

### 2. Content Interleaving Fix (Multi-Provider)
**Issue**: Garbled text like "Yes. It's anYes Android project" from multi-provider stream merging

**Root Cause**: `RunEnsembleStream` was merging ALL provider streams in parallel.

**Fix**: Use single provider for streaming in `internal/services/ensemble.go`:
```go
// STREAMING FIX: Use SINGLE provider stream to avoid content interleaving
selectedStream := streamChans[0]
// Close any additional streams (cleanup)
```

### 3. Helper Function Name Conflicts
**Issue**: Duplicate function definitions when running entire test package

**Fix**: Renamed helper functions with prefixes:
- `getBaseURL()` -> `bearMailGetBaseURL()` / `cliAgentGetBaseURL()`
- `skipIfNotRunning()` -> `bearMailSkipIfNotRunning()` / `cliAgentSkipIfNotRunning()`

## CLI Agent Binary Locations

| Agent | Binary Path | Build Command |
|-------|-------------|---------------|
| HelixCode | `/Projects/HelixCode/HelixCode/bin/helixcode` | `make build` |
| OpenCode | `/Projects/HelixCode/Example_Projects/OpenCode/OpenCode/opencode` | `go build -o opencode .` |
| Cline | `/Projects/HelixCode/Example_Projects/Cline/cli/cline` | Requires gRPC setup |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HELIXAGENT_URL` | `http://localhost:7061` | HelixAgent base URL |
| `BEAR_MAIL_PATH` | `/Projects/Bear-Mail` | Bear-Mail test project |
| `HELIX_CODE_PATH` | `/Projects/HelixCode` | HelixCode project |

## Known Issues

### Cline CLI Build
Cline CLI requires gRPC generated code that depends on a submodule:
```
github.com/cline/grpc-go@v0.0.0 (replaced by ./src/generated/grpc-go): missing go.mod
```

**Workaround**: Tests use HelixAgent directly with Cline system prompts.

### Long Response Timeouts
Some tests with high `max_tokens` (2500+) may approach the 5-minute timeout.

**Recommendation**: For very long responses, use the `/v1/debates` async endpoint.

## Future Improvements

1. **Cline gRPC Setup**: Add proto compilation step for Cline CLI build
2. **More CLI Agents**: Add tests for Aider, Forge, Gemini CLI, etc.
3. **Visual Rendering Tests**: Add tests for terminal rendering output
4. **Performance Benchmarks**: Add benchmark tests for response latency

## Maintenance

### Adding New CLI Agent Tests

1. Add constants for agent paths:
```go
const (
    newAgentPath    = "/path/to/agent"
    newAgentCLIPath = "/path/to/agent/binary"
)
```

2. Add existence check function:
```go
func ensureNewAgentExists(t *testing.T) {
    // Check if binary exists or build it
}
```

3. Add streaming integrity tests:
```go
func TestNewAgentStreamingIntegrity(t *testing.T) {
    cliAgentSkipIfNotRunning(t)
    ensureNewAgentExists(t)
    // Tests...
}
```

4. Run and verify:
```bash
go test -v ./tests/integration/... -run "TestNewAgent"
```
