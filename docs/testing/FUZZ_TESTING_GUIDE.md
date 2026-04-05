# Fuzz Testing Guide

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent uses Go's native fuzzing framework (`testing.F`) to discover panics, crashes, and unexpected behavior in parsers, validators, and protocol handlers. The fuzz test suite contains 52 fuzz functions across 17 test files, organized in `tests/fuzz/`.

---

## Fuzz Target Inventory

### 1. JSON Parsing (8 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzJSONRequestParsing` | `fuzz_test.go` | LLM request JSON deserialization |
| `FuzzLLMRequestParsing` | `json_parsing_fuzz_test.go` | Full LLM request structure with messages |
| `FuzzMessageParsing` | `json_parsing_fuzz_test.go` | Individual message object parsing |
| `FuzzProviderResponseParsing` | `json_parsing_fuzz_test.go` | Provider response JSON handling |
| `FuzzYAMLLikeConfigParsing` | `config_parsing_fuzz_test.go` | YAML config key-value parsing |
| `FuzzJSONConfigParsing` | `config_parsing_fuzz_test.go` | JSON config deserialization |
| `FuzzEnvVarParsing` | `config_parsing_fuzz_test.go` | Environment variable string parsing |
| `FuzzCacheKeyRoundTrip` | `cache_key_fuzz_test.go` | Cache key encode/decode round-trip |

### 2. Authentication & Security (4 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzJWTTokenValidation` | `auth_token_fuzz_test.go` | JWT token validation edge cases |
| `FuzzJWTTokenExtraction` | `auth_token_fuzz_test.go` | Bearer token extraction from headers |
| `FuzzJWTClaimsParsing` | `auth_token_fuzz_test.go` | JWT claims deserialization |
| `FuzzAPIKeyValidation` | `auth_token_fuzz_test.go` | API key format validation |

### 3. Protocol Handling (6 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzSSEParsing` | `fuzz_test.go` | Server-Sent Events data parsing |
| `FuzzJSONRPCRequestParsing` | `protocol_parsing_fuzz_test.go` | JSON-RPC 2.0 request parsing |
| `FuzzJSONRPCResponseParsing` | `protocol_parsing_fuzz_test.go` | JSON-RPC 2.0 response parsing |
| `FuzzMCPProtocolBatch` | `protocol_parsing_fuzz_test.go` | MCP batch request handling |
| `FuzzMCPJSONRPCParsing` | `mcp_message_fuzz_test.go` | MCP JSON-RPC message parsing |
| `FuzzMCPResponseParsing` | `mcp_message_fuzz_test.go` | MCP response structure parsing |

### 4. Streaming (1 function)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzStreamingDataParsing` | `streaming_data_fuzz_test.go` | SSE/streaming chunk parsing |

### 5. Memory & RAG (5 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzMemoryStoreSearch` | `memory_rag_fuzz_test.go` | Memory store search query handling |
| `FuzzMemoryEntitySearch` | `memory_rag_fuzz_test.go` | Entity graph search inputs |
| `FuzzMemoryJSONParsing` | `memory_rag_fuzz_test.go` | Memory record JSON parsing |
| `FuzzRAGSearchOptionsParsing` | `memory_rag_fuzz_test.go` | RAG search option parsing |
| `FuzzRAGDocumentParsing` | `memory_rag_fuzz_test.go` | RAG document structure parsing |

### 6. Vector Database (3 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzVectorQueryConstruction` | `vector_query_fuzz_test.go` | Vector query builder |
| `FuzzVectorFilterParsing` | `vector_query_fuzz_test.go` | Vector filter expression parsing |
| `FuzzVectorConfigValidation` | `vector_query_fuzz_test.go` | Vector DB config validation |

### 7. Embeddings (2 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzEmbeddingInputProcessing` | `embedding_input_fuzz_test.go` | Embedding input normalization |
| `FuzzEmbeddingResponseParsing` | `embedding_input_fuzz_test.go` | Embedding response parsing |

### 8. Debate (2 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzDebateMessageProcessing` | `debate_message_fuzz_test.go` | Debate message structure handling |
| `FuzzDebateResultParsing` | `debate_message_fuzz_test.go` | Debate result JSON parsing |

### 9. Tools & Agentic (5 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzToolValidation` | `tool_schema_fuzz_test.go` | Tool schema validation |
| `FuzzToolSchemaJSON` | `tool_schema_fuzz_test.go` | Tool schema JSON round-trip |
| `FuzzToolCallArgumentParsing` | `tool_schema_fuzz_test.go` | Tool call argument parsing |
| `FuzzAgenticToolCallParameters` | `agentic_tool_call_fuzz_test.go` | Agentic tool call params |
| `FuzzAgenticPlanParsing` | `agentic_tool_call_fuzz_test.go` | Agentic plan JSON parsing |

### 10. Miscellaneous (6 functions)

| Function | File | What It Tests |
|----------|------|---------------|
| `FuzzToolSchemaValidation` | `fuzz_test.go` | Tool schema validation (legacy) |
| `FuzzModelIDParsing` | `fuzz_test.go` | Model ID string parsing |
| `FuzzHTTPHeaderParsing` | `fuzz_test.go` | HTTP header extraction |
| `FuzzCacheKeyGeneration` | `cache_key_fuzz_test.go` | Cache key generation |
| `FuzzRateLimitKeyExtraction` | `rate_limit_fuzz_test.go` | Rate limit key extraction |
| `FuzzRateLimitPathConfig` | `rate_limit_fuzz_test.go` | Rate limit path configuration |
| `FuzzHealthCheckResponseParsing` | `health_check_fuzz_test.go` | Health check response parsing |
| `FuzzHealthCheckEndpointParsing` | `health_check_fuzz_test.go` | Health check URL parsing |
| `FuzzPromptTemplateRendering` | `prompt_template_fuzz_test.go` | Prompt template rendering |
| `FuzzDebatePromptTemplate` | `prompt_template_fuzz_test.go` | Debate prompt templates |
| `FuzzPromptVariableSubstitution` | `prompt_template_fuzz_test.go` | Variable substitution |

Additional fuzz targets exist in submodules: `LLMOrchestrator/pkg/parser/` (3 functions) and `Toolkit/pkg/toolkit/common/` (2 functions).

---

## Running Fuzz Tests

### Corpus Replay (CI-safe, deterministic)

```bash
make test-fuzz
```

This replays all previously-discovered corpus entries without generating new inputs. Fast and deterministic -- suitable for pre-commit validation.

### Short Fuzz Run (30 seconds per target)

```bash
GOMAXPROCS=2 nice -n 19 go test -tags=fuzz -fuzz=. -fuzztime=30s ./tests/fuzz/ -p 1
```

### Single Target Extended Run

```bash
GOMAXPROCS=2 nice -n 19 go test -tags=fuzz \
  -fuzz=FuzzJWTTokenValidation -fuzztime=5m \
  ./tests/fuzz/ -p 1
```

### All Targets Extended Run

```bash
for f in $(go test -tags=fuzz -list='Fuzz.*' ./tests/fuzz/ 2>/dev/null | grep '^Fuzz'); do
  echo "Fuzzing $f..."
  GOMAXPROCS=2 nice -n 19 go test -tags=fuzz \
    -fuzz="^${f}$" -fuzztime=2m \
    ./tests/fuzz/ -p 1
done
```

---

## Corpus Management

Go stores fuzz corpora in `testdata/fuzz/<FunctionName>/` within the test package directory. Interesting inputs (those that increase coverage or trigger new code paths) are automatically saved.

**Corpus location:** `tests/fuzz/testdata/fuzz/`

**Adding seed corpus entries:**
```go
func FuzzMyParser(f *testing.F) {
    // Add seed corpus entries
    f.Add([]byte(`{"valid": "json"}`))
    f.Add([]byte(`{}`))
    f.Add([]byte(`invalid`))
    f.Add([]byte(``))

    f.Fuzz(func(t *testing.T, data []byte) {
        // Should not panic
        _ = MyParser(data)
    })
}
```

**Committing corpus:** Corpus files found by the fuzzer should be committed to the repository so that future runs benefit from them.

---

## Adding New Fuzz Targets

1. Create a test file in `tests/fuzz/` with the `//go:build fuzz` build tag
2. Name the function `Fuzz<TargetDescription>`
3. Add seed corpus entries that cover known edge cases
4. The fuzz function body should call the target and assert it does not panic
5. For parsers: verify round-trip consistency where possible
6. Update this document's inventory table

**Template:**
```go
//go:build fuzz

package fuzz

import "testing"

func FuzzNewTarget(f *testing.F) {
    f.Add([]byte("seed1"))
    f.Add([]byte("seed2"))

    f.Fuzz(func(t *testing.T, data []byte) {
        // Must not panic on any input
        result, err := targetFunction(data)
        if err != nil {
            return // errors are acceptable; panics are not
        }
        _ = result
    })
}
```

---

## Cross-References

- Test strategy: `docs/testing/TEST_STRATEGY.md`
- Stress testing: `docs/testing/STRESS_TESTING_GUIDE.md`
- Security scanning: `docs/security/SCANNING_GUIDE.md`
