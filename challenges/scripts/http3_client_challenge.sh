#!/bin/bash
#
# HTTP/3 Client Challenge Script
# Validates HTTP/3 client implementation
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Challenge metadata
CHALLENGE_NAME="HTTP/3 Client Implementation"
CHALLENGE_POINTS=100

log_info "Starting $CHALLENGE_NAME Challenge..."
log_info "Target: $CHALLENGE_POINTS points"
echo ""

# Track score
SCORE=0
TOTAL_CHECKS=10

# Check 1: HTTP/3 client file exists
log_info "Check 1: Verifying HTTP/3 client implementation..."
if [ -f "$PROJECT_ROOT/internal/transport/http3_client.go" ]; then
    log_info "✓ http3_client.go exists"
    SCORE=$((SCORE + 10))
else
    log_error "✗ http3_client.go not found"
fi

# Check 2: HTTP/3 client test file exists
log_info "Check 2: Verifying test coverage..."
if [ -f "$PROJECT_ROOT/internal/transport/http3_client_test.go" ]; then
    log_info "✓ http3_client_test.go exists"
    SCORE=$((SCORE + 10))
else
    log_error "✗ http3_client_test.go not found"
fi

# Check 3: HTTP/3 round-tripper implementation
log_info "Check 3: Checking HTTP/3 round-tripper..."
if [ -f "$PROJECT_ROOT/internal/transport/http3_client.go" ]; then
    if grep -q "http3.RoundTripper" "$PROJECT_ROOT/internal/transport/http3_client.go" || \
       grep -q "http3.Client" "$PROJECT_ROOT/internal/transport/http3_client.go"; then
        log_info "✓ HTTP/3 round-tripper implemented"
        SCORE=$((SCORE + 10))
    else
        log_error "✗ HTTP/3 round-tripper not found"
    fi
else
    log_error "✗ http3_client.go not found"
fi

# Check 4: Brotli compression support
log_info "Check 4: Checking Brotli compression..."
if [ -f "$PROJECT_ROOT/internal/transport/http3_client.go" ]; then
    if grep -q "brotli" "$PROJECT_ROOT/internal/transport/http3_client.go" || \
       grep -q "Brotli" "$PROJECT_ROOT/internal/transport/http3_client.go"; then
        log_info "✓ Brotli compression supported"
        SCORE=$((SCORE + 10))
    else
        log_error "✗ Brotli compression not found"
    fi
else
    log_error "✗ http3_client.go not found"
fi

# Check 5: Fallback mechanism
log_info "Check 5: Checking fallback to HTTP/2/HTTP/1.1..."
if [ -f "$PROJECT_ROOT/internal/transport/http3_client.go" ]; then
    if grep -q "fallback" "$PROJECT_ROOT/internal/transport/http3_client.go" || \
       grep -q "Fallback" "$PROJECT_ROOT/internal/transport/http3_client.go"; then
        log_info "✓ Fallback mechanism implemented"
        SCORE=$((SCORE + 10))
    else
        log_error "✗ Fallback mechanism not found"
    fi
else
    log_error "✗ http3_client.go not found"
fi

# Check 6: Retry logic
log_info "Check 6: Checking retry logic..."
if [ -f "$PROJECT_ROOT/internal/transport/http3_client.go" ]; then
    if grep -q "retry" "$PROJECT_ROOT/internal/transport/http3_client.go" || \
       grep -q "Retry" "$PROJECT_ROOT/internal/transport/http3_client.go"; then
        log_info "✓ Retry logic implemented"
        SCORE=$((SCORE + 10))
    else
        log_error "✗ Retry logic not found"
    fi
else
    log_error "✗ http3_client.go not found"
fi

# Check 7: LLM providers updated
log_info "Check 7: Checking LLM provider updates..."
PROVIDERS_UPDATED=0
for provider in claude openai gemini anthropic deepseek; do
    if [ -f "$PROJECT_ROOT/internal/llm/providers/$provider/$provider.go" ]; then
        if grep -q "http3" "$PROJECT_ROOT/internal/llm/providers/$provider/$provider.go" || \
           grep -q "HTTP3" "$PROJECT_ROOT/internal/llm/providers/$provider/$provider.go" || \
           grep -q "transport.NewHTTP3Client" "$PROJECT_ROOT/internal/llm/providers/$provider/$provider.go"; then
            PROVIDERS_UPDATED=$((PROVIDERS_UPDATED + 1))
        fi
    fi
done
if [ $PROVIDERS_UPDATED -ge 3 ]; then
    log_info "✓ LLM providers updated ($PROVIDERS_UPDATED/5+)"
    SCORE=$((SCORE + 10))
else
    log_error "✗ Insufficient provider updates ($PROVIDERS_UPDATED, need 3+)"
fi

# Check 8: Code compiles
log_info "Check 8: Verifying compilation..."
cd "$PROJECT_ROOT"
if go build ./internal/transport/... 2>/dev/null; then
    log_info "✓ Transport package compiles"
    SCORE=$((SCORE + 10))
else
    log_error "✗ Compilation failed"
fi

# Check 9: Unit tests pass
log_info "Check 9: Running unit tests..."
cd "$PROJECT_ROOT"
if go test -short ./internal/transport/... 2>/dev/null; then
    log_info "✓ Unit tests pass"
    SCORE=$((SCORE + 10))
else
    log_error "✗ Unit tests failed"
fi

# Check 10: Test coverage
log_info "Check 10: Verifying test coverage..."
cd "$PROJECT_ROOT"
COVERAGE=$(go test -cover ./internal/transport/... 2>/dev/null | grep -oE '[0-9]+\.?[0-9]*%' | head -1 | tr -d '%')
if [ -n "$$COVERAGE" ] && [ "${COVERAGE%.*}" -ge 70 ]; then
    log_info "✓ Test coverage is ${COVERAGE}% (>= 70%)"
    SCORE=$((SCORE + 10))
else
    log_warn "⚠ Test coverage is ${COVERAGE}% (target: 70%)"
    SCORE=$((SCORE + 5))
fi

echo ""
echo "========================================"
log_info "Challenge Complete!"
echo "Score: $SCORE/$CHALLENGE_POINTS points"
echo "========================================"

# Return appropriate exit code
if [ $SCORE -ge 80 ]; then
    log_info "✓ CHALLENGE PASSED!"
    exit 0
else
    log_error "✗ CHALLENGE FAILED"
    exit 1
fi
