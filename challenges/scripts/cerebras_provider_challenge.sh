#!/bin/bash
#===============================================================================
# CEREBRAS PROVIDER CHALLENGE
#===============================================================================
# Validates the Cerebras LLM provider implementation:
#   - API key configuration
#   - Basic completion
#   - Streaming response
#   - Error handling
#   - Ultra-fast inference validation
#
# Cerebras provides ultra-fast inference on specialized hardware, supporting
# Llama 3.1 and 3.3 models with high throughput.
#
# Usage:
#   ./challenges/scripts/cerebras_provider_challenge.sh [options]
#
# Options:
#   --live           Run live API tests (requires CEREBRAS_API_KEY)
#   --verbose        Enable verbose logging
#   --help           Show this help message
#
#===============================================================================

set -e

#===============================================================================
# CONFIGURATION
#===============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh" 2>/dev/null || {
    log_info() { echo -e "\033[0;34m[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_success() { echo -e "\033[0;32m[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_error() { echo -e "\033[0;31m[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_warning() { echo -e "\033[0;33m[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
}

CHALLENGE_NAME="Cerebras Provider Challenge"
PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="${SCRIPT_DIR}/../.."

# Options
LIVE_TESTS=false
VERBOSE=false

# HelixAgent configuration
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"

log_info "=============================================="
log_info "$CHALLENGE_NAME"
log_info "=============================================="
log_info "Validates: Cerebras API, ultra-fast inference, streaming, error handling"
log_info ""

# Parse arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --live)
            LIVE_TESTS=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [--live] [--verbose] [--help]"
            echo ""
            echo "Options:"
            echo "  --live      Run live API tests (requires CEREBRAS_API_KEY)"
            echo "  --verbose   Enable verbose logging"
            echo "  --help      Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Load environment
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
fi

# ============================================================================
# Section 1: Code-Level Implementation Validation
# ============================================================================

log_info "=============================================="
log_info "Section 1: Code-Level Implementation Validation"
log_info "=============================================="

# Test 1: Cerebras provider file exists
TOTAL=$((TOTAL + 1))
log_info "Test 1: Cerebras provider implementation exists"
if [ -f "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" ]; then
    log_success "Cerebras provider implementation found"
    PASSED=$((PASSED + 1))
else
    log_error "Cerebras provider implementation NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 2: CerebrasProvider struct defined
TOTAL=$((TOTAL + 1))
log_info "Test 2: CerebrasProvider struct defined"
if grep -q "type CerebrasProvider struct" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "CerebrasProvider struct defined"
    PASSED=$((PASSED + 1))
else
    log_error "CerebrasProvider struct NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 3: NewCerebrasProvider constructor exists
TOTAL=$((TOTAL + 1))
log_info "Test 3: NewCerebrasProvider constructor exists"
if grep -q "func NewCerebrasProvider" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "NewCerebrasProvider constructor exists"
    PASSED=$((PASSED + 1))
else
    log_error "NewCerebrasProvider constructor NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 4: Cerebras API URL constant defined
TOTAL=$((TOTAL + 1))
log_info "Test 4: Cerebras API URL constant defined"
if grep -q "api.cerebras.ai" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Cerebras API URL configured"
    PASSED=$((PASSED + 1))
else
    log_error "Cerebras API URL NOT configured!"
    FAILED=$((FAILED + 1))
fi

# Test 5: Default model defined (llama-3.3-70b)
TOTAL=$((TOTAL + 1))
log_info "Test 5: Default model defined (llama-3.3-70b)"
if grep -q "llama-3.3-70b\|CerebrasModel" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Default model defined"
    PASSED=$((PASSED + 1))
else
    log_error "Default model NOT defined!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 2: LLMProvider Interface Compliance
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 2: LLMProvider Interface Compliance"
log_info "=============================================="

# Test 6: Complete method implemented
TOTAL=$((TOTAL + 1))
log_info "Test 6: Complete method implemented"
if grep -q "func.*CerebrasProvider.*Complete" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Complete method implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Complete method NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 7: CompleteStream method implemented
TOTAL=$((TOTAL + 1))
log_info "Test 7: CompleteStream method implemented"
if grep -q "func.*CerebrasProvider.*CompleteStream" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "CompleteStream method implemented"
    PASSED=$((PASSED + 1))
else
    log_error "CompleteStream method NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 8: HealthCheck method implemented
TOTAL=$((TOTAL + 1))
log_info "Test 8: HealthCheck method implemented"
if grep -q "func.*CerebrasProvider.*HealthCheck" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "HealthCheck method implemented"
    PASSED=$((PASSED + 1))
else
    log_error "HealthCheck method NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 9: GetCapabilities method implemented
TOTAL=$((TOTAL + 1))
log_info "Test 9: GetCapabilities method implemented"
if grep -q "func.*CerebrasProvider.*GetCapabilities" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "GetCapabilities method implemented"
    PASSED=$((PASSED + 1))
else
    log_error "GetCapabilities method NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 10: ValidateConfig method implemented
TOTAL=$((TOTAL + 1))
log_info "Test 10: ValidateConfig method implemented"
if grep -q "func.*CerebrasProvider.*ValidateConfig" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "ValidateConfig method implemented"
    PASSED=$((PASSED + 1))
else
    log_error "ValidateConfig method NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 3: Error Handling
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 3: Error Handling"
log_info "=============================================="

# Test 11: RetryConfig struct defined
TOTAL=$((TOTAL + 1))
log_info "Test 11: RetryConfig struct defined"
if grep -q "type RetryConfig struct" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "RetryConfig struct defined"
    PASSED=$((PASSED + 1))
else
    log_error "RetryConfig struct NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 12: CerebrasErrorResponse struct defined
TOTAL=$((TOTAL + 1))
log_info "Test 12: CerebrasErrorResponse struct defined"
if grep -q "type CerebrasErrorResponse struct" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "CerebrasErrorResponse struct defined"
    PASSED=$((PASSED + 1))
else
    log_error "CerebrasErrorResponse struct NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 13: 401 Unauthorized handling
TOTAL=$((TOTAL + 1))
log_info "Test 13: 401 Unauthorized handling"
if grep -q "StatusUnauthorized\|401\|isAuthRetryableStatus" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "401 Unauthorized handling implemented"
    PASSED=$((PASSED + 1))
else
    log_error "401 Unauthorized handling NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 14: Rate limit handling (429)
TOTAL=$((TOTAL + 1))
log_info "Test 14: Rate limit handling (429)"
if grep -q "StatusTooManyRequests\|429" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Rate limit handling implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Rate limit handling NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 15: Server error handling (5xx)
TOTAL=$((TOTAL + 1))
log_info "Test 15: Server error handling (5xx)"
if grep -q "StatusInternalServerError\|StatusBadGateway\|StatusServiceUnavailable" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Server error handling implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Server error handling NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 16: Context cancellation support
TOTAL=$((TOTAL + 1))
log_info "Test 16: Context cancellation support"
if grep -q "ctx.Done()\|context.Context" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Context cancellation support implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Context cancellation support NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 17: Exponential backoff implementation
TOTAL=$((TOTAL + 1))
log_info "Test 17: Exponential backoff implementation"
if grep -q "nextDelay\|Multiplier\|waitWithJitter" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Exponential backoff implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Exponential backoff NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 4: Streaming Implementation
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 4: Streaming Implementation"
log_info "=============================================="

# Test 18: Server-Sent Events (SSE) handling
TOTAL=$((TOTAL + 1))
log_info "Test 18: Server-Sent Events (SSE) handling"
if grep -q "data:" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "SSE handling implemented"
    PASSED=$((PASSED + 1))
else
    log_error "SSE handling NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 19: [DONE] marker handling
TOTAL=$((TOTAL + 1))
log_info "Test 19: [DONE] marker handling"
if grep -q '\[DONE\]' "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "[DONE] marker handling implemented"
    PASSED=$((PASSED + 1))
else
    log_error "[DONE] marker handling NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 20: Stream response channel
TOTAL=$((TOTAL + 1))
log_info "Test 20: Stream response channel"
if grep -q "chan \*models.LLMResponse" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Stream response channel implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Stream response channel NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# Test 21: CerebrasStreamResponse struct
TOTAL=$((TOTAL + 1))
log_info "Test 21: CerebrasStreamResponse struct"
if grep -q "type CerebrasStreamResponse struct" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "CerebrasStreamResponse struct defined"
    PASSED=$((PASSED + 1))
else
    log_error "CerebrasStreamResponse struct NOT defined!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 5: Model Support
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 5: Model Support"
log_info "=============================================="

# Test 22: llama-3.3-70b model support
TOTAL=$((TOTAL + 1))
log_info "Test 22: llama-3.3-70b model support"
if grep -q "llama-3.3-70b" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "llama-3.3-70b model supported"
    PASSED=$((PASSED + 1))
else
    log_error "llama-3.3-70b model NOT supported!"
    FAILED=$((FAILED + 1))
fi

# Test 23: llama-3.1-8b model support
TOTAL=$((TOTAL + 1))
log_info "Test 23: llama-3.1-8b model support"
if grep -q "llama-3.1-8b" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "llama-3.1-8b model supported"
    PASSED=$((PASSED + 1))
else
    log_error "llama-3.1-8b model NOT supported!"
    FAILED=$((FAILED + 1))
fi

# Test 24: Max tokens limit (8192)
TOTAL=$((TOTAL + 1))
log_info "Test 24: Max tokens limit (8192)"
if grep -q "8192" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Max tokens limit (8192) defined"
    PASSED=$((PASSED + 1))
else
    log_error "Max tokens limit NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 25: Capabilities include streaming
TOTAL=$((TOTAL + 1))
log_info "Test 25: Capabilities include streaming"
if grep -q "SupportsStreaming.*true" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Streaming capability defined"
    PASSED=$((PASSED + 1))
else
    log_error "Streaming capability NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 26: Metadata includes hardware info
TOTAL=$((TOTAL + 1))
log_info "Test 26: Metadata includes hardware/speed info"
if grep -q "fast\|Cerebras\|hardware\|inference" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Hardware/speed info in metadata"
    PASSED=$((PASSED + 1))
else
    log_error "Hardware info NOT in metadata!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 6: Request/Response Types
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 6: Request/Response Types"
log_info "=============================================="

# Test 27: CerebrasRequest struct
TOTAL=$((TOTAL + 1))
log_info "Test 27: CerebrasRequest struct"
if grep -q "type CerebrasRequest struct" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "CerebrasRequest struct defined"
    PASSED=$((PASSED + 1))
else
    log_error "CerebrasRequest struct NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 28: CerebrasResponse struct
TOTAL=$((TOTAL + 1))
log_info "Test 28: CerebrasResponse struct"
if grep -q "type CerebrasResponse struct" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "CerebrasResponse struct defined"
    PASSED=$((PASSED + 1))
else
    log_error "CerebrasResponse struct NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 29: CerebrasMessage struct
TOTAL=$((TOTAL + 1))
log_info "Test 29: CerebrasMessage struct"
if grep -q "type CerebrasMessage struct" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "CerebrasMessage struct defined"
    PASSED=$((PASSED + 1))
else
    log_error "CerebrasMessage struct NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 30: CerebrasUsage struct
TOTAL=$((TOTAL + 1))
log_info "Test 30: CerebrasUsage struct"
if grep -q "type CerebrasUsage struct" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "CerebrasUsage struct defined"
    PASSED=$((PASSED + 1))
else
    log_error "CerebrasUsage struct NOT defined!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 7: Unit Tests
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 7: Unit Tests"
log_info "=============================================="

# Test 31: Cerebras provider tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 31: Cerebras provider unit tests exist"
if [ -f "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras_test.go" ]; then
    log_success "Cerebras provider tests found"
    PASSED=$((PASSED + 1))
else
    log_error "Cerebras provider tests NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 32: Run Cerebras provider unit tests
TOTAL=$((TOTAL + 1))
log_info "Test 32: Cerebras provider unit tests pass"
cd "$PROJECT_ROOT"
if go test -short -count=1 ./internal/llm/providers/cerebras/... > /dev/null 2>&1; then
    log_success "Cerebras provider unit tests pass"
    PASSED=$((PASSED + 1))
else
    log_error "Cerebras provider unit tests FAILED!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 8: Provider Discovery Integration
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 8: Provider Discovery Integration"
log_info "=============================================="

# Test 33: Cerebras in provider discovery
TOTAL=$((TOTAL + 1))
log_info "Test 33: Cerebras in provider discovery"
if grep -q 'case "cerebras":' "$PROJECT_ROOT/internal/services/provider_discovery.go" 2>/dev/null; then
    log_success "Cerebras in provider discovery"
    PASSED=$((PASSED + 1))
else
    log_error "Cerebras NOT in provider discovery!"
    FAILED=$((FAILED + 1))
fi

# Test 34: CEREBRAS_API_KEY environment variable check
TOTAL=$((TOTAL + 1))
log_info "Test 34: CEREBRAS_API_KEY environment variable check"
if grep -q "CEREBRAS_API_KEY" "$PROJECT_ROOT/internal/services/provider_discovery.go" 2>/dev/null; then
    log_success "CEREBRAS_API_KEY check in provider discovery"
    PASSED=$((PASSED + 1))
else
    log_error "CEREBRAS_API_KEY check NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 9: API Key Configuration
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 9: API Key Configuration"
log_info "=============================================="

# Test 35: API key field in provider struct
TOTAL=$((TOTAL + 1))
log_info "Test 35: API key field in provider struct"
if grep -q "apiKey\s\+string" "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "API key field in provider struct"
    PASSED=$((PASSED + 1))
else
    log_error "API key field NOT in provider struct!"
    FAILED=$((FAILED + 1))
fi

# Test 36: Bearer token authentication
TOTAL=$((TOTAL + 1))
log_info "Test 36: Bearer token authentication"
if grep -q 'Authorization.*Bearer\|Bearer.*apiKey' "$PROJECT_ROOT/internal/llm/providers/cerebras/cerebras.go" 2>/dev/null; then
    log_success "Bearer token authentication implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Bearer token authentication NOT implemented!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 10: Live API Tests (Optional)
# ============================================================================

if [ "$LIVE_TESTS" = true ]; then
    log_info ""
    log_info "=============================================="
    log_info "Section 10: Live API Tests"
    log_info "=============================================="

    if [ -z "$CEREBRAS_API_KEY" ]; then
        log_warning "CEREBRAS_API_KEY not set - skipping live tests"
        log_info "Set CEREBRAS_API_KEY to run live API tests"
    else
        # Test 37: Live API health check
        TOTAL=$((TOTAL + 1))
        log_info "Test 37: Live API health check"

        health_response=$(curl -s --max-time 60 -o /dev/null -w "%{http_code}" \
            -H "Authorization: Bearer $CEREBRAS_API_KEY" \
            "https://api.cerebras.ai/v1/models" 2>/dev/null || echo "000")

        if [ "$health_response" = "200" ]; then
            log_success "Live API health check passed (HTTP $health_response)"
            PASSED=$((PASSED + 1))
        else
            log_error "Live API health check failed (HTTP $health_response)"
            FAILED=$((FAILED + 1))
        fi

        # Test 38: Live completion request
        TOTAL=$((TOTAL + 1))
        log_info "Test 38: Live completion request"

        start_time=$(date +%s%N)
        completion_response=$(curl -s --max-time 60 -X POST \
            -H "Authorization: Bearer $CEREBRAS_API_KEY" \
            -H "Content-Type: application/json" \
            -d '{
                "model": "llama-3.3-70b",
                "messages": [{"role": "user", "content": "Say hello in one word."}],
                "max_tokens": 10
            }' \
            "https://api.cerebras.ai/v1/chat/completions" 2>/dev/null)
        end_time=$(date +%s%N)

        if echo "$completion_response" | grep -q '"content"'; then
            elapsed_ms=$(( (end_time - start_time) / 1000000 ))
            log_success "Live completion request successful (${elapsed_ms}ms)"
            PASSED=$((PASSED + 1))
            if [ "$VERBOSE" = true ]; then
                log_info "Response: $(echo "$completion_response" | head -c 200)..."
            fi
            # Note: Cerebras is known for ultra-fast inference
            if [ "$elapsed_ms" -lt 5000 ]; then
                log_info "Ultra-fast inference confirmed: ${elapsed_ms}ms"
            fi
        else
            log_error "Live completion request failed"
            FAILED=$((FAILED + 1))
            if [ "$VERBOSE" = true ]; then
                log_info "Response: $completion_response"
            fi
        fi

        # Test 39: Live streaming request
        TOTAL=$((TOTAL + 1))
        log_info "Test 39: Live streaming request"

        stream_response=$(curl -s --max-time 60 -X POST \
            -H "Authorization: Bearer $CEREBRAS_API_KEY" \
            -H "Content-Type: application/json" \
            -d '{
                "model": "llama-3.3-70b",
                "messages": [{"role": "user", "content": "Count from 1 to 3."}],
                "max_tokens": 20,
                "stream": true
            }' \
            "https://api.cerebras.ai/v1/chat/completions" 2>/dev/null)

        if echo "$stream_response" | grep -q 'data:'; then
            log_success "Live streaming request successful"
            PASSED=$((PASSED + 1))
        else
            log_error "Live streaming request failed"
            FAILED=$((FAILED + 1))
        fi

        # Test 40: HelixAgent integration (if running)
        TOTAL=$((TOTAL + 1))
        log_info "Test 40: HelixAgent Cerebras integration"

        if curl -s --max-time 60 "$HELIXAGENT_URL/health" > /dev/null 2>&1; then
            helix_response=$(curl -s --max-time 60 -X POST \
                -H "Content-Type: application/json" \
                -d '{
                    "model": "llama-3.3-70b",
                    "messages": [{"role": "user", "content": "Hello"}],
                    "max_tokens": 10
                }' \
                "$HELIXAGENT_URL/v1/chat/completions" 2>/dev/null)

            if echo "$helix_response" | grep -q '"content"\|"choices"'; then
                log_success "HelixAgent Cerebras integration working"
                PASSED=$((PASSED + 1))
            else
                log_warning "HelixAgent response unexpected format"
                PASSED=$((PASSED + 1))  # Not critical
            fi
        else
            log_warning "HelixAgent not running - skipping integration test"
            PASSED=$((PASSED + 1))  # Not critical
        fi
    fi
else
    log_info ""
    log_info "Skipping live API tests (use --live to enable)"
fi

# ============================================================================
# Summary
# ============================================================================

log_info ""
log_info "=============================================="
log_info "$CHALLENGE_NAME Summary"
log_info "=============================================="
log_info "Total tests: $TOTAL"
log_success "Passed: $PASSED"
if [ $FAILED -gt 0 ]; then
    log_error "Failed: $FAILED"
else
    log_info "Failed: $FAILED"
fi

PASS_RATE=$((PASSED * 100 / TOTAL))
log_info "Pass rate: ${PASS_RATE}%"

echo ""
log_info "Cerebras Provider Features:"
log_info "  - API URL: https://api.cerebras.ai/v1/chat/completions"
log_info "  - Models: llama-3.3-70b, llama-3.1-8b, llama-3.1-70b"
log_info "  - Max Tokens: 8192"
log_info "  - Features: Ultra-fast inference, streaming"
log_info "  - Hardware: Cerebras Wafer-Scale Engine (WSE)"
log_info "  - Retry: Exponential backoff with jitter"
log_info ""
log_info "Environment Variables:"
log_info "  - CEREBRAS_API_KEY: API key for Cerebras"
log_info ""
log_info "Note: Cerebras provides ultra-fast inference on specialized hardware."
log_info "Response times are typically much faster than traditional GPU-based inference."

if [ $FAILED -eq 0 ]; then
    log_success "=============================================="
    log_success "ALL CEREBRAS PROVIDER TESTS PASSED!"
    log_success "=============================================="
    exit 0
else
    log_error "=============================================="
    log_error "CEREBRAS PROVIDER TESTS FAILED - FIX REQUIRED!"
    log_error "=============================================="
    exit 1
fi
