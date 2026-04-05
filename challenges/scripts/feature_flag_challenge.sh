#!/usr/bin/env bash
set -euo pipefail

# Feature Flag Challenge
# Validates dead code cleanup: FEATURE_* env vars removed from .env.example,
# and that surviving env vars (ALLOWED_ORIGINS, DB_SSL_MODE, RATE_LIMIT_RPM)
# are properly read in config.go.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "feature_flag" "$@"

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

print_header "Feature Flag Challenge"
echo ""

# Test 1: .env.example exists
if [ -f "$ENV_EXAMPLE" ]; then
    record_result ".env.example exists" "PASS"
else
    record_result ".env.example exists" "FAIL"
fi

# Test 2: No FEATURE_* vars in .env.example
FEATURE_COUNT=$(grep -c "^FEATURE_" "$ENV_EXAMPLE" 2>/dev/null || echo "0")
if [ "$FEATURE_COUNT" -eq 0 ]; then
    record_result "No FEATURE_* dead vars in .env.example" "PASS"
else
    record_result "No FEATURE_* dead vars in .env.example (found: $FEATURE_COUNT)" "FAIL"
fi

# Test 3: ALLOWED_ORIGINS is present in .env.example
if grep -q "ALLOWED_ORIGINS" "$ENV_EXAMPLE" 2>/dev/null; then
    record_result "ALLOWED_ORIGINS present in .env.example" "PASS"
else
    record_result "ALLOWED_ORIGINS present in .env.example" "FAIL"
fi

# Test 4: ALLOWED_ORIGINS is read in config.go
if grep -q "ALLOWED_ORIGINS\|CORS_ALLOWED_ORIGINS" "$CONFIG_FILE" 2>/dev/null; then
    record_result "ALLOWED_ORIGINS read in config.go" "PASS"
else
    record_result "ALLOWED_ORIGINS read in config.go" "FAIL"
fi

# Test 5: DB_SSL_MODE present in .env.example
if grep -q "DB_SSL_MODE\|DB_SSLMODE" "$ENV_EXAMPLE" 2>/dev/null; then
    record_result "DB_SSL_MODE present in .env.example" "PASS"
else
    record_result "DB_SSL_MODE present in .env.example" "FAIL"
fi

# Test 6: DB_SSL_MODE is read in config.go
if grep -q "DB_SSL_MODE\|DB_SSLMODE" "$CONFIG_FILE" 2>/dev/null; then
    record_result "DB_SSL_MODE read in config.go" "PASS"
else
    record_result "DB_SSL_MODE read in config.go" "FAIL"
fi

# Test 7: RATE_LIMIT_RPM present in .env.example
if grep -q "RATE_LIMIT_RPM\|RATE_LIMIT_REQUESTS" "$ENV_EXAMPLE" 2>/dev/null; then
    record_result "RATE_LIMIT_RPM present in .env.example" "PASS"
else
    record_result "RATE_LIMIT_RPM present in .env.example" "FAIL"
fi

# Test 8: RATE_LIMIT_RPM is read in config.go
if grep -q "RATE_LIMIT_RPM\|RATE_LIMIT_REQUESTS" "$CONFIG_FILE" 2>/dev/null; then
    record_result "RATE_LIMIT_RPM read in config.go" "PASS"
else
    record_result "RATE_LIMIT_RPM read in config.go" "FAIL"
fi

# Test 9: config.go compiles
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/config/... 2>/dev/null; then
    record_result "internal/config package compiles" "PASS"
else
    record_result "internal/config package compiles" "FAIL"
fi

# Test 10: No dead FEATURE_ vars referenced in Go source (they should be removed)
FEATURE_GO_COUNT=$(grep -r "FEATURE_" "$PROJECT_ROOT/internal/" --include="*.go" 2>/dev/null | grep -v "_test.go" | wc -l)
if [ "$FEATURE_GO_COUNT" -eq 0 ]; then
    record_result "No FEATURE_* dead vars referenced in production Go code" "PASS"
else
    record_result "No FEATURE_* dead vars in production Go code (found: $FEATURE_GO_COUNT references)" "FAIL"
fi

# Test 11: getEnvWithFallback or similar fallback helper exists in config.go
if grep -q "getEnvWithFallback\|getIntEnvWithFallback\|getEnvSliceWithFallback" "$CONFIG_FILE" 2>/dev/null; then
    record_result "Config fallback helper functions exist in config.go" "PASS"
else
    record_result "Config fallback helper functions exist in config.go" "FAIL"
fi

# Test 12: PORT env var is referenced in config.go
if grep -q '"PORT"' "$CONFIG_FILE" 2>/dev/null; then
    record_result "PORT env var referenced in config.go" "PASS"
else
    record_result "PORT env var referenced in config.go" "FAIL"
fi

echo ""
print_summary "Feature Flag Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
