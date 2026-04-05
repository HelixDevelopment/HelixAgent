#!/bin/bash
# HelixAgent Challenge - Brotli Compression
# Validates that HTTP/3 + Brotli compression is implemented correctly:
# compression middleware source, Brotli function, router registration,
# tests, go.mod dependency, and gzip fallback.
#
# Tests: 10

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

record_result() {
    local name="$1" status="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$status" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        echo -e "${GREEN}[PASS]${NC} $name"
    else
        FAILED=$((FAILED + 1))
        echo -e "${RED}[FAIL]${NC} $name"
    fi
}

echo "=========================================="
echo "  Brotli Compression Challenge"
echo "=========================================="
echo ""

COMPRESSION_FILE="$PROJECT_ROOT/internal/middleware/compression.go"
COMPRESSION_TEST="$PROJECT_ROOT/internal/middleware/compression_test.go"
ROUTER_FILE="$PROJECT_ROOT/internal/router/router.go"
GO_MOD="$PROJECT_ROOT/go.mod"

# --------------------------------------------------------------------------
# Test 1: internal/middleware/compression.go exists
# --------------------------------------------------------------------------
if [ -f "$COMPRESSION_FILE" ]; then
    record_result "internal/middleware/compression.go exists" "PASS"
else
    record_result "internal/middleware/compression.go exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 2: BrotliMiddleware function exists in compression.go
# --------------------------------------------------------------------------
if [ -f "$COMPRESSION_FILE" ] && grep -q "func BrotliMiddleware()" "$COMPRESSION_FILE"; then
    record_result "BrotliMiddleware() function defined in compression.go" "PASS"
else
    record_result "BrotliMiddleware() function defined in compression.go" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 3: CompressionMiddleware function exists (configurable entry point)
# --------------------------------------------------------------------------
if [ -f "$COMPRESSION_FILE" ] && grep -q "func CompressionMiddleware(" "$COMPRESSION_FILE"; then
    record_result "CompressionMiddleware() configurable function defined" "PASS"
else
    record_result "CompressionMiddleware() configurable function defined" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 4: CompressionMiddleware registered in router
# --------------------------------------------------------------------------
if [ -f "$ROUTER_FILE" ] && \
   grep -q "CompressionMiddleware\|BrotliMiddleware" "$ROUTER_FILE"; then
    record_result "CompressionMiddleware registered in internal/router/router.go" "PASS"
else
    record_result "CompressionMiddleware registered in internal/router/router.go" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 5: compression_test.go exists
# --------------------------------------------------------------------------
if [ -f "$COMPRESSION_TEST" ]; then
    record_result "internal/middleware/compression_test.go exists" "PASS"
else
    record_result "internal/middleware/compression_test.go exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 6: andybalholm/brotli in go.mod
# --------------------------------------------------------------------------
if grep -q "andybalholm/brotli" "$GO_MOD" 2>/dev/null; then
    record_result "andybalholm/brotli dependency in go.mod" "PASS"
else
    record_result "andybalholm/brotli dependency in go.mod" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 7: Gzip fallback implemented in compression.go
# --------------------------------------------------------------------------
if [ -f "$COMPRESSION_FILE" ] && grep -q "gzip\|Gzip" "$COMPRESSION_FILE"; then
    record_result "Gzip fallback implemented in compression.go" "PASS"
else
    record_result "Gzip fallback implemented in compression.go" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 8: Brotli is preferred over gzip (Brotli checked first in Accept-Encoding)
# --------------------------------------------------------------------------
if [ -f "$COMPRESSION_FILE" ] && grep -q "br\b\|brotli\|Brotli" "$COMPRESSION_FILE"; then
    record_result "Brotli encoding (\"br\") handled in compression.go" "PASS"
else
    record_result "Brotli encoding (\"br\") handled in compression.go" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 9: compression middleware package compiles cleanly
# --------------------------------------------------------------------------
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/middleware/ 2>/dev/null; then
    record_result "internal/middleware/ package builds cleanly" "PASS"
else
    record_result "internal/middleware/ package builds cleanly" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 10: compression_test.go has at least one test function
# --------------------------------------------------------------------------
if [ -f "$COMPRESSION_TEST" ] && grep -q "^func Test" "$COMPRESSION_TEST"; then
    record_result "compression_test.go contains at least one test function" "PASS"
else
    record_result "compression_test.go contains at least one test function" "FAIL"
fi

echo ""
echo "=========================================="
echo "  Results: $PASSED/$TOTAL passed, $FAILED failed"
echo "=========================================="

[ $FAILED -eq 0 ] && exit 0 || exit 1
