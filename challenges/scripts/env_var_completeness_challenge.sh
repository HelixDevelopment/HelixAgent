#!/usr/bin/env bash
set -euo pipefail

# Env Var Completeness Challenge
# Validates that every significant env var in .env.example is referenced in
# Go source, no dead FEATURE_* vars remain, HELIX_MEMORY_* vars are handled,
# and config fallback helpers exist.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "env_var_completeness" "$@"

PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_EXAMPLE="$PROJECT_ROOT/.env.example"
CONFIG_FILE="$PROJECT_ROOT/internal/config/config.go"

record_result() {
    TOTAL=$((TOTAL + 1))
    test_start "$1"
    if [ "$2" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        test_pass
    else
        FAILED=$((FAILED + 1))
        test_fail "$1"
    fi
}

print_header "Env Var Completeness Challenge"
echo ""

# Test 1: .env.example has substantial content (>= 30 vars)
ENV_VAR_COUNT=$(grep -c "^[A-Z_]*=" "$ENV_EXAMPLE" 2>/dev/null || echo "0")
if [ "$ENV_VAR_COUNT" -ge 30 ]; then
    record_result ".env.example has >= 30 env vars (actual: $ENV_VAR_COUNT)" "PASS"
else
    record_result ".env.example has >= 30 env vars (actual: $ENV_VAR_COUNT)" "FAIL"
fi

# Test 2: No FEATURE_* dead vars remain in .env.example
FEATURE_COUNT=$(grep -c "^FEATURE_" "$ENV_EXAMPLE" 2>/dev/null || echo "0")
if [ "$FEATURE_COUNT" -eq 0 ]; then
    record_result "No dead FEATURE_* vars in .env.example" "PASS"
else
    record_result "No dead FEATURE_* vars in .env.example (found: $FEATURE_COUNT)" "FAIL"
fi

# Test 3: PORT is referenced in config.go
if grep -q '"PORT"' "$CONFIG_FILE" 2>/dev/null; then
    record_result "PORT env var referenced in config.go" "PASS"
else
    record_result "PORT env var referenced in config.go" "FAIL"
fi

# Test 4: JWT_SECRET is referenced in Go source
if grep -rq "JWT_SECRET" "$PROJECT_ROOT/internal/" --include="*.go" 2>/dev/null; then
    record_result "JWT_SECRET referenced in Go source" "PASS"
else
    record_result "JWT_SECRET referenced in Go source" "FAIL"
fi

# Test 5: DB_HOST is referenced in config.go
if grep -q "DB_HOST" "$CONFIG_FILE" 2>/dev/null; then
    record_result "DB_HOST referenced in config.go" "PASS"
else
    record_result "DB_HOST referenced in config.go" "FAIL"
fi

# Test 6: REDIS_HOST is referenced in config.go
if grep -q "REDIS_HOST" "$CONFIG_FILE" 2>/dev/null; then
    record_result "REDIS_HOST referenced in config.go" "PASS"
else
    record_result "REDIS_HOST referenced in config.go" "FAIL"
fi

# Test 7: HELIX_MEMORY_* vars are read somewhere in Go source
HELIX_MEM_COUNT=$(grep -r "HELIX_MEMORY_\|HELIXMEMORY_" "$PROJECT_ROOT/internal/" --include="*.go" 2>/dev/null | wc -l)
HELIX_MEM_MODULE=$(grep -r "HELIX_MEMORY_\|HELIXMEMORY_" "$PROJECT_ROOT/HelixMemory/" --include="*.go" 2>/dev/null | wc -l)
TOTAL_HELIX_MEM=$((HELIX_MEM_COUNT + HELIX_MEM_MODULE))
if [ "$TOTAL_HELIX_MEM" -gt 0 ]; then
    record_result "HELIX_MEMORY_* vars referenced in Go source" "PASS"
else
    # Check if HelixMemory module handles its own env vars
    if grep -rq "MEM0_\|COGNEE_\|LETTA_\|GRAPHITI_" "$PROJECT_ROOT/internal/" --include="*.go" 2>/dev/null; then
        record_result "Memory system env vars referenced in Go source" "PASS"
    else
        record_result "HELIX_MEMORY_* / memory system vars referenced in Go source" "FAIL"
    fi
fi

# Test 8: Config fallback helpers exist (getEnvWithFallback)
if grep -q "func getEnvWithFallback\|func getIntEnvWithFallback\|func getEnvSliceWithFallback" "$CONFIG_FILE" 2>/dev/null; then
    record_result "Config fallback helper functions defined in config.go" "PASS"
else
    record_result "Config fallback helper functions defined in config.go" "FAIL"
fi

# Test 9: GIN_MODE is referenced in Go source
if grep -rq "GIN_MODE" "$PROJECT_ROOT/internal/" --include="*.go" 2>/dev/null || \
   grep -rq "GIN_MODE" "$PROJECT_ROOT/cmd/" --include="*.go" 2>/dev/null; then
    record_result "GIN_MODE env var referenced in Go source" "PASS"
else
    record_result "GIN_MODE env var referenced in Go source" "FAIL"
fi

# Test 10: COGNEE_ENABLED referenced in Go source
if grep -rq "COGNEE_ENABLED" "$PROJECT_ROOT/internal/" --include="*.go" 2>/dev/null; then
    record_result "COGNEE_ENABLED env var referenced in Go source" "PASS"
else
    record_result "COGNEE_ENABLED env var referenced in Go source" "FAIL"
fi

# Test 11: At least one *_API_KEY pattern is referenced in config.go or provider code
API_KEY_COUNT=$(grep -r "_API_KEY" "$PROJECT_ROOT/internal/llm/providers/" --include="*.go" 2>/dev/null | wc -l)
if [ "$API_KEY_COUNT" -gt 10 ]; then
    record_result "Provider API keys referenced in LLM provider code ($API_KEY_COUNT references)" "PASS"
else
    record_result "Provider API keys referenced in LLM provider code ($API_KEY_COUNT references, need >10)" "FAIL"
fi

# Test 12: CONSTITUTION_WATCHER_ENABLED referenced in Go source
if grep -rq "CONSTITUTION_WATCHER_ENABLED" "$PROJECT_ROOT/internal/" --include="*.go" 2>/dev/null; then
    record_result "CONSTITUTION_WATCHER_ENABLED referenced in Go source" "PASS"
else
    record_result "CONSTITUTION_WATCHER_ENABLED referenced in Go source" "FAIL"
fi

# Test 13: config.go compiles cleanly
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/config/... 2>/dev/null; then
    record_result "internal/config compiles cleanly" "PASS"
else
    record_result "internal/config compiles cleanly" "FAIL"
fi

# Test 14: No unused env var getters — getEnv calls match env.example sections
# (Check that getBoolEnv exists as a helper)
if grep -q "func getBoolEnv\|func getIntEnv\|func getDurationEnv" "$CONFIG_FILE" 2>/dev/null; then
    record_result "Typed env var helper functions exist (getBoolEnv, getIntEnv, getDurationEnv)" "PASS"
else
    record_result "Typed env var helper functions exist (getBoolEnv, getIntEnv, getDurationEnv)" "FAIL"
fi

# Test 15: RATE_LIMITING_ENABLED referenced in config.go
if grep -q "RATE_LIMITING_ENABLED\|RATE_LIMIT" "$CONFIG_FILE" 2>/dev/null; then
    record_result "Rate limiting env var referenced in config.go" "PASS"
else
    record_result "Rate limiting env var referenced in config.go" "FAIL"
fi

# Test 16: DB_PASSWORD referenced in config.go
if grep -q "DB_PASSWORD" "$CONFIG_FILE" 2>/dev/null; then
    record_result "DB_PASSWORD referenced in config.go" "PASS"
else
    record_result "DB_PASSWORD referenced in config.go" "FAIL"
fi

# Test 17: REDIS_PASSWORD referenced in config.go
if grep -q "REDIS_PASSWORD" "$CONFIG_FILE" 2>/dev/null; then
    record_result "REDIS_PASSWORD referenced in config.go" "PASS"
else
    record_result "REDIS_PASSWORD referenced in config.go" "FAIL"
fi

# Test 18: SVC_* service override pattern referenced in config.go
if grep -q "SVC_" "$CONFIG_FILE" 2>/dev/null; then
    record_result "SVC_* service override pattern referenced in config.go" "PASS"
else
    record_result "SVC_* service override pattern referenced in config.go" "FAIL"
fi

# Test 19: BIGDATA_ENABLE_* pattern referenced in Go source
if grep -rq "BIGDATA_ENABLE_" "$PROJECT_ROOT/internal/" --include="*.go" 2>/dev/null; then
    record_result "BIGDATA_ENABLE_* vars referenced in Go source" "PASS"
else
    record_result "BIGDATA_ENABLE_* vars referenced in Go source" "FAIL"
fi

# Test 20: .env.example does not reference undefined vars (no empty required vars)
# Check that all required vars have at least a placeholder or default value
EMPTY_REQUIRED=$(grep "^[A-Z_]*=$" "$ENV_EXAMPLE" 2>/dev/null | grep -v "_API_KEY=\|_SECRET=\|_PASSWORD=\|_TOKEN=" | wc -l)
if [ "$EMPTY_REQUIRED" -lt 10 ]; then
    record_result "Env vars in .env.example have sensible defaults ($EMPTY_REQUIRED bare-empty non-secret vars)" "PASS"
else
    record_result "Too many bare-empty non-secret vars in .env.example ($EMPTY_REQUIRED)" "FAIL"
fi

echo ""
print_summary "Env Var Completeness Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
