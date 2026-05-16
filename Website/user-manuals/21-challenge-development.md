# User Manual 21: Challenge Development

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Challenge Architecture](#challenge-architecture)
4. [Challenge Types](#challenge-types)
5. [Shell-Based Challenges](#shell-based-challenges)
6. [Go-Native Challenges](#go-native-challenges)
7. [Assertion Engine](#assertion-engine)
8. [Challenge Runner](#challenge-runner)
9. [Reporting and Metrics](#reporting-and-metrics)
10. [CI Integration](#ci-integration)
11. [Creating a New Challenge](#creating-a-new-challenge)
12. [Scoring](#scoring)
13. [Challenge Directory Structure](#challenge-directory-structure)
14. [Troubleshooting](#troubleshooting)
15. [Related Resources](#related-resources)

## Overview

Challenges are HelixAgent's mechanism for validating real-life use cases beyond traditional testing. Unlike unit or integration tests, challenges verify actual system behavior: they start real services, send real HTTP requests, inspect real responses, and confirm that the system works end-to-end as a user would experience it.

Every component in HelixAgent must have challenge scripts that validate its functionality. Challenges must never produce false successes -- they must validate actual behavior, not merely check return codes.

The challenge framework is provided by the Challenges module (`digital.vasic.challenges`), which includes an assertion engine with 19 evaluators, a challenge registry, a runner, reporting, monitoring, metrics, a plugin system, and userflow testing adapters.

## Prerequisites

- HelixAgent built: `make build`
- Infrastructure running (started automatically by HelixAgent boot or via `make test-infra-start`)
- Bash 4.0+ for shell-based challenges
- Go 1.24+ for Go-native challenges
- `curl` and `jq` available on PATH

## Challenge Architecture

```
+-------------------+     +-------------------+     +------------------+
|  Challenge Script |     |  Challenge Runner  |     |  Assertion Engine |
|  (shell / Go)     +---->+  (orchestration)   +---->+  (19 evaluators)  |
+-------------------+     +--------+----------+     +------------------+
                                   |
                          +--------v----------+
                          |   Reporting &     |
                          |   Metrics         |
                          +--------+----------+
                                   |
                          +--------v----------+
                          |   Pass / Fail     |
                          |   Summary         |
                          +-------------------+
```

## Challenge Types

| Type | Language | Location | Use Case |
|---|---|---|---|
| Shell challenges | Bash | `challenges/scripts/*.sh` | System-level validation, HTTP endpoint checks |
| Go-native challenges | Go | `tests/challenge/` | Complex validation with Go assertions |
| Userflow challenges | Go | `internal/challenges/` | 22 userflow scenarios with dependency graphs |

## Shell-Based Challenges

### Full Template

```bash
#!/bin/bash
set -euo pipefail

# ============================================================
# Challenge: Feature Name Validation
# Tests: XX tests
# ============================================================

CHALLENGE_NAME="Feature Name Challenge"
PASSED=0
FAILED=0
TOTAL=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()    { echo -e "${YELLOW}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; PASSED=$((PASSED + 1)); TOTAL=$((TOTAL + 1)); }
log_error()   { echo -e "${RED}[FAIL]${NC} $1"; FAILED=$((FAILED + 1)); TOTAL=$((TOTAL + 1)); }

HELIX_URL="${HELIX_URL:-http://localhost:7061}"

# ----------------------------------------------------------
# Test 1: Verify endpoint exists
# ----------------------------------------------------------
test_endpoint_exists() {
    local response
    response=$(curl -s -o /dev/null -w "%{http_code}" "${HELIX_URL}/v1/feature")

    if [ "$response" = "200" ]; then
        log_success "Feature endpoint returns 200"
    else
        log_error "Feature endpoint returned $response (expected 200)"
    fi
}

# ----------------------------------------------------------
# Test 2: Verify response structure
# ----------------------------------------------------------
test_response_structure() {
    local response
    response=$(curl -s "${HELIX_URL}/v1/feature")

    if echo "$response" | jq -e '.data' > /dev/null 2>&1; then
        log_success "Response contains 'data' field"
    else
        log_error "Response missing 'data' field: $response"
    fi
}

# ----------------------------------------------------------
# Test 3: Verify with real data (no false positives)
# ----------------------------------------------------------
test_real_data() {
    local response
    response=$(curl -s -X POST "${HELIX_URL}/v1/feature" \
        -H "Content-Type: application/json" \
        -d '{"input": "test data"}')

    local actual_value
    actual_value=$(echo "$response" | jq -r '.result')

    if [ "$actual_value" != "null" ] && [ -n "$actual_value" ]; then
        log_success "Feature processes real data: $actual_value"
    else
        log_error "Feature returned empty result for real data"
    fi
}

# ----------------------------------------------------------
# Summary
# ----------------------------------------------------------
print_summary() {
    echo ""
    echo "============================================"
    echo " $CHALLENGE_NAME"
    echo "============================================"
    echo " Passed: $PASSED / $TOTAL"
    echo " Failed: $FAILED / $TOTAL"
    echo "============================================"

    if [ "$FAILED" -gt 0 ]; then
        exit 1
    fi
}

main() {
    log_info "Starting $CHALLENGE_NAME"
    echo ""

    test_endpoint_exists
    test_response_structure
    test_real_data

    print_summary
}

main "$@"
```

### Key Rules for Shell Challenges

1. **Always validate actual content** -- Check response bodies, not just HTTP status codes
2. **Use `jq` for JSON validation** -- Parse and assert on specific fields
3. **Include negative tests** -- Verify that invalid inputs produce proper error responses
4. **Log both pass and fail** -- Every test must produce clear output regardless of outcome
5. **Use `set -euo pipefail`** -- Fail fast on unexpected errors
6. **Exit 1 on any failure** -- The challenge must return a non-zero exit code if any test fails

## Go-Native Challenges

Go-native challenges live in `tests/challenge/` and `internal/challenges/` and use the Challenges module assertion engine:

```go
package challenge

import (
    "testing"
    "net/http"
    "encoding/json"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestChallenge_DebateSystem_ProducesConsensus(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping challenge test")
    }

    // Send a real debate request
    payload := `{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What is 2+2?"}],
        "debate_config": {"rounds": 3, "topology": "mesh"}
    }`

    resp, err := http.Post("http://localhost:7061/v1/chat/completions",
        "application/json", strings.NewReader(payload))
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusOK, resp.StatusCode)

    var result map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&result)
    require.NoError(t, err)

    // Validate actual content, not just structure
    choices := result["choices"].([]interface{})
    require.NotEmpty(t, choices)

    content := choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)
    assert.Contains(t, content, "4", "Debate should reach consensus on 2+2=4")
}
```

### Userflow Challenges

HelixAgent includes 22 Go-native userflow challenges with a dependency graph, covering browser, mobile, desktop, API, gRPC, WebSocket, and build userflows:

```bash
# Run all userflow challenges
go test -v --run-challenges=userflow ./internal/challenges/...
```

## Assertion Engine

The Challenges module provides 19 built-in evaluators:

| Evaluator | Description |
|---|---|
| `equals` | Exact value match |
| `contains` | Substring or element containment |
| `matches` | Regular expression match |
| `greater_than` | Numeric comparison |
| `less_than` | Numeric comparison |
| `between` | Range check |
| `not_empty` | Non-empty string/collection |
| `json_path` | JSONPath expression evaluation |
| `status_code` | HTTP status code assertion |
| `response_time` | Latency threshold check |
| `header_present` | HTTP header existence |
| `header_value` | HTTP header value match |
| `body_contains` | Response body content check |
| `body_json` | JSON structure validation |
| `count` | Collection length assertion |
| `type_check` | Value type assertion |
| `schema` | JSON schema validation |
| `custom` | User-defined evaluator function |
| `composite` | Combines multiple evaluators |

## Challenge Runner

The runner orchestrates challenge execution with:

- Sequential or parallel execution modes
- Timeout enforcement per test and per challenge
- Resource limit enforcement (GOMAXPROCS, nice, ionice)
- Retry support for flaky infrastructure
- Dependency graph resolution (tests run in topological order)

### Running All Challenges

```bash
# Run all shell challenges
./challenges/scripts/run_all_challenges.sh

# Run specific challenge
./challenges/scripts/debate_orchestrator_challenge.sh

# Run Go-native challenges
go test -v ./tests/challenge/...
```

### Running with Resource Limits

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
    ./challenges/scripts/run_all_challenges.sh
```

## Reporting and Metrics

Challenges produce structured reports including:

- Test name, status (pass/fail), duration
- Failure details with expected vs. actual values
- Total pass/fail/skip counts
- Execution timestamp and environment info

Example summary output:

```
============================================
 Debate Orchestrator Challenge
============================================
 Passed: 58 / 61
 Failed: 3 / 61
 Duration: 45.2s
============================================
 FAILED TESTS:
   - test_mesh_topology_5_providers: expected 5 responses, got 4
   - test_convergence_detection: timeout after 30s
   - test_cross_debate_learning: knowledge not propagated
============================================
```

## CI Integration

Challenges are run as part of the CI validation pipeline:

```bash
# Full CI validation (includes challenges)
make ci-validate-all

# Challenges only
./challenges/scripts/run_all_challenges.sh
```

Challenges are designed to be non-interactive and fully automatable. They read configuration from environment variables and `.env` files, never from interactive prompts.

### Pre-Push Hook

```bash
# Runs unit tests and selected challenges before pushing
make ci-pre-push
```

## Creating a New Challenge

### Step-by-Step Process

1. **Identify the feature** to validate and its real-world use cases
2. **Create the script** in `challenges/scripts/`:

```bash
touch challenges/scripts/my_feature_challenge.sh
chmod +x challenges/scripts/my_feature_challenge.sh
```

3. **Write tests** that validate actual behavior (use the template above)
4. **Add to run_all_challenges.sh** so it runs with the full suite
5. **Test the challenge itself** -- run it and verify it catches real failures
6. **Document the challenge** -- add the test count to CLAUDE.md and challenge docs

### Validation Checklist

Before submitting a new challenge, verify:

- [ ] Challenge returns exit code 1 when any test fails
- [ ] Challenge validates actual data, not just return codes
- [ ] Challenge runs non-interactively (no prompts)
- [ ] Challenge respects resource limits
- [ ] Challenge includes both positive and negative tests
- [ ] Challenge output clearly shows pass/fail per test
- [ ] Challenge is added to `run_all_challenges.sh`

## Scoring

Challenges are categorized by difficulty for tracking and prioritization:

| Difficulty | Points | Examples |
|---|---|---|
| Easy | 5-10 | Single endpoint validation, config check |
| Medium | 15-25 | Multi-step workflow, provider integration |
| Hard | 30-50 | Full debate orchestration, cross-module |
| Expert | 50-100 | Distributed system validation, chaos testing |

## Challenge Directory Structure

```
challenges/
+-- scripts/
|   +-- run_all_challenges.sh                    # Master runner
|   +-- release_build_challenge.sh               # 25 tests
|   +-- unified_verification_challenge.sh        # 15 tests
|   +-- debate_orchestrator_challenge.sh          # 61 tests
|   +-- debate_performance_optimizer_challenge.sh # 36 tests
|   +-- helixmemory_challenge.sh                 # 80+ tests
|   +-- helixspecifier_challenge.sh              # 138 tests
|   +-- ... (40+ challenge scripts)
+-- README.md

tests/challenge/                                 # Go-native challenges

internal/challenges/                             # HelixAgent-specific challenges
+-- plugin_challenge.go
+-- infra_bridge_challenge.go
+-- shell_adapter_challenge.go
+-- userflow_*.go                                # 22 userflow challenges
```

## Troubleshooting

### Challenge Fails with "Connection Refused"

**Symptom:** `curl: (7) Failed to connect to localhost port 7061`

**Solutions:**
1. Ensure HelixAgent is running: `./bin/helixagent` (starts all containers automatically)
2. Wait for startup verification to complete (~2 minutes)
3. Check the server log: `tail -f /tmp/helixagent-server.log`

### Challenge Reports False Success

**Symptom:** Challenge passes but the feature is actually broken.

**Solutions:**
1. Add assertions that check response body content, not just HTTP status
2. Use `jq -e` to enforce non-null JSON values
3. Compare expected values against actual values explicitly
4. Add negative tests that verify error cases

### Challenge Hangs Indefinitely

**Symptom:** Script never completes.

**Solutions:**
1. Add timeouts to all `curl` calls: `curl --max-time 30`
2. Add a global timeout wrapper: `timeout 300 ./challenges/scripts/my_challenge.sh`
3. Check for interactive prompts (forbidden in challenges)
4. Verify infrastructure containers are healthy

### Permission Denied on Challenge Script

**Symptom:** `bash: ./challenges/scripts/my_challenge.sh: Permission denied`

**Solution:**
```bash
chmod +x challenges/scripts/my_challenge.sh
```

## Related Resources

- [User Manual 20: Testing Strategies](20-testing-strategies.md) -- Testing framework and conventions
- [User Manual 18: Performance Monitoring](18-performance-monitoring.md) -- Monitoring challenge metrics
- Challenges module: `challenges/`
- Challenge scripts: `challenges/scripts/`
- Go-native challenges: `tests/challenge/`
- Userflow challenges: `internal/challenges/`
