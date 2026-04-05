# Comprehensive Report: Zen Model ID Fix and Response Quality Validation

**Date:** 2026-01-14
**Author:** Claude Code
**Status:** RESOLVED

## Executive Summary

Zen free models were returning "Unable to provide analysis at this time" despite passing LLMsVerifier verification. The root cause was a **model ID mismatch** between the configuration and the Zen API requirements. This report details the investigation, fix, and improvements made to prevent similar issues in the future.

## Problem Statement

### Symptoms
- Zen models (Big Pickle, Grok Code Fast, GLM 4.7 Free, GPT 5 Nano) passed LLMsVerifier verification
- Models received satisfactory scores (6.0-7.0 range)
- When used via OpenCode CLI, models returned error message: "Unable to provide analysis at this time"

### Root Cause
The Zen API requires model names **WITHOUT** the "opencode/" prefix:
- **WRONG:** `opencode/grok-code` (what was being sent)
- **CORRECT:** `grok-code` (what the API expects)

### Additional Finding
Two Zen models (`glm-4.7-free` and `gpt-5-nano`) have backend token counting errors on the Zen API side. These are upstream issues not related to HelixAgent.

## Investigation Process

### 1. Direct API Testing
Tested each Zen model directly against the API:

```bash
# FAILED - Model not supported
curl -X POST https://opencode.ai/zen/v1/chat/completions \
  -d '{"model": "opencode/grok-code", ...}'
# Response: "Model opencode/grok-code not supported"

# SUCCESS - Working correctly
curl -X POST https://opencode.ai/zen/v1/chat/completions \
  -d '{"model": "grok-code", ...}'
# Response: Valid completion
```

### 2. Model Availability Matrix

| Model ID | With Prefix | Without Prefix | Status |
|----------|-------------|----------------|--------|
| grok-code | FAIL | OK | Working |
| big-pickle | FAIL | OK | Working |
| glm-4.7-free | FAIL | FAIL | Backend token error |
| gpt-5-nano | FAIL | FAIL | Backend token error |

### 3. Verification Gap Analysis

The LLMsVerifier passed these models because:
1. Health check (models endpoint) returns 200 OK
2. Model list includes all 4 free models
3. Verification didn't test with actual model names used in production

## Resolution

### Files Modified

| File | Changes |
|------|---------|
| `internal/llm/providers/zen/zen.go` | Fixed model constants, added `normalizeModelID()` function |
| `internal/services/debate_team_config.go` | Updated ZenModels struct with correct IDs |
| `internal/verifier/scoring.go` | Added response quality validation and penalty system |
| `internal/verifier/adapters/free_adapter_test.go` | Updated tests for new model IDs |
| `tests/integration/zen_response_quality_test.go` | Created new integration tests |

### Key Code Changes

#### 1. Model ID Normalization (`zen.go`)
```go
// normalizeModelID strips the "opencode/" prefix if present
func normalizeModelID(modelID string) string {
    if strings.HasPrefix(modelID, "opencode/") {
        return strings.TrimPrefix(modelID, "opencode/")
    }
    if strings.HasPrefix(modelID, "opencode-") {
        return strings.TrimPrefix(modelID, "opencode-")
    }
    return modelID
}
```

#### 2. Response Quality Validation (`scoring.go`)
```go
// ValidateResponseQuality checks if a response is valid and meaningful
func ValidateResponseQuality(content string) *ResponseQualityResult {
    // Checks for:
    // - Empty responses (10.0 penalty)
    // - "Unable to provide" errors (8.0 penalty)
    // - "Model not supported" errors (10.0 penalty)
    // - Token counting errors (9.0 penalty)
    // ...
}
```

#### 3. Corrected Model Constants
```go
// Before (WRONG):
ModelBigPickle = "opencode/big-pickle"

// After (CORRECT):
ModelBigPickle = "big-pickle"

// Legacy support:
ModelBigPickleFull = "opencode/big-pickle" // For backward compat
```

## Verification Results

### Unit Tests
```
=== RUN   TestNewZenProvider
--- PASS: TestNewZenProvider (0.00s)
=== RUN   TestFreeModels
--- PASS: TestFreeModels (0.00s)
=== RUN   TestIsFreeModel
--- PASS: TestIsFreeModel (0.00s)
=== RUN   TestZenProvider_Complete
--- PASS: TestZenProvider_Complete (0.00s)
...
PASS
ok      dev.helix.agent/internal/llm/providers/zen    0.514s
```

### Integration Tests
```
=== CLI Agents Challenge (18 Agents) ===
Assertions: 42 passed, 0 failed
Status: PASSED
```

### Production Verification
```bash
curl http://localhost:7061/v1/chat/completions \
  -d '{"model": "helixagent-debate", "messages": [...]}'
# Response: {"content": "4"} ✓
```

## Improvements Made

### 1. Response Quality Validation System
Added a comprehensive response quality validation system to the LLMsVerifier scoring service:
- Empty response detection (10.0 penalty)
- Error message pattern detection (3.0-10.0 penalties)
- Very short response warnings (1.0 penalty)
- Batch validation and penalty application

### 2. Model ID Normalization
Added automatic normalization to handle both formats:
- `opencode/grok-code` → `grok-code`
- `opencode-grok-code` → `grok-code`
- `grok-code` → `grok-code` (unchanged)

### 3. New Integration Tests
Created `tests/integration/zen_response_quality_test.go` with:
- Per-model response quality validation
- Model ID normalization tests
- Provider capabilities verification
- Response quality validation logic tests

## Recommendations

### Short-term
1. **Skip unreliable models:** GLM 4.7 Free and GPT 5 Nano should be skipped due to backend issues
2. **Monitor Zen API:** Watch for changes in model availability
3. **Update OpenCode config:** Regenerate with correct model IDs

### Long-term
1. **Add response quality to verification:** LLMsVerifier should test actual completions, not just health checks
2. **Implement model allowlist:** Maintain a list of verified working models
3. **Add automatic fallback:** When a model returns errors, automatically fall back to next provider

## Appendix

### A. Working Zen Models
| Model | Status | Notes |
|-------|--------|-------|
| grok-code | Working | Default model, recommended |
| big-pickle | Working | Stealth model for covert operations |

### B. Non-Working Zen Models
| Model | Error | Notes |
|-------|-------|-------|
| glm-4.7-free | Backend token counting error | Upstream issue |
| gpt-5-nano | Backend token counting error | Upstream issue |

### C. Test Commands
```bash
# Build
make build

# Run Zen tests
go test -v ./internal/llm/providers/zen/...

# Run verifier tests
go test -v ./internal/verifier/...

# Run response quality tests
go test -v ./tests/integration/zen_response_quality_test.go
```

---

**Resolution Verified:** All unit tests pass. HelixAgent server returns correct responses. CLI Agents Challenge passes all 42 assertions.
