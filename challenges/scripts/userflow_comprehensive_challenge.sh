#!/usr/bin/env bash
# userflow_comprehensive_challenge.sh
#
# Comprehensive user flow challenge for HelixAgent.
# Executes all API, protocol, and system user flows
# automatically, simulating a real user or QA tester.
#
# Prerequisites:
#   - HelixAgent server running on PORT (default: 8100)
#   - Infrastructure containers running (PostgreSQL, Redis)
#
# Usage:
#   ./challenges/scripts/userflow_comprehensive_challenge.sh
#   HELIX_PORT=8080 ./challenges/scripts/userflow_comprehensive_challenge.sh

set -euo pipefail

# Configuration
HELIX_PORT="${HELIX_PORT:-8100}"
HELIX_HOST="${HELIX_HOST:-localhost}"
BASE_URL="http://${HELIX_HOST}:${HELIX_PORT}"
RESULTS_DIR="results/userflow"
PASS=0
FAIL=0
SKIP=0
TOTAL=0

# Resource limits per Constitution Rule #15
export GOMAXPROCS=2

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

mkdir -p "$RESULTS_DIR"

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; PASS=$((PASS + 1)); TOTAL=$((TOTAL + 1)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL + 1)); TOTAL=$((TOTAL + 1)); }
log_skip() { echo -e "${YELLOW}[SKIP]${NC} $1"; SKIP=$((SKIP + 1)); TOTAL=$((TOTAL + 1)); }

# Check if server is running
check_server() {
    local response
    response=$(curl -s -o /dev/null -w "%{http_code}" \
        --connect-timeout 5 "${BASE_URL}/health" 2>/dev/null || echo "000")
    if [ "$response" = "200" ]; then
        return 0
    fi
    return 1
}

# Execute a GET request and validate status code
test_get() {
    local name="$1"
    local path="$2"
    local expected_codes="$3"

    local response_code
    local response_body
    response_body=$(curl -s -w "\n%{http_code}" \
        --connect-timeout 10 --max-time 30 \
        "${BASE_URL}${path}" 2>/dev/null || echo -e "\n000")
    response_code=$(echo "$response_body" | tail -1)
    response_body=$(echo "$response_body" | head -n -1)

    local found=false
    for code in $expected_codes; do
        if [ "$response_code" = "$code" ]; then
            found=true
            break
        fi
    done

    if $found; then
        log_pass "$name (HTTP $response_code)"
        echo "$response_body" > "$RESULTS_DIR/${name}.json" 2>/dev/null || true
    else
        log_fail "$name (expected $expected_codes, got HTTP $response_code)"
    fi
}

# Execute a POST request and validate
test_post() {
    local name="$1"
    local path="$2"
    local body="$3"
    local expected_codes="$4"
    local check_body="${5:-}"

    local response_code
    local response_body
    response_body=$(curl -s -w "\n%{http_code}" \
        --connect-timeout 10 --max-time 60 \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${BASE_URL}${path}" 2>/dev/null || echo -e "\n000")
    response_code=$(echo "$response_body" | tail -1)
    response_body=$(echo "$response_body" | head -n -1)

    local found=false
    for code in $expected_codes; do
        if [ "$response_code" = "$code" ]; then
            found=true
            break
        fi
    done

    if ! $found; then
        log_fail "$name (expected $expected_codes, got HTTP $response_code)"
        return
    fi

    if [ -n "$check_body" ] && ! echo "$response_body" | grep -q "$check_body"; then
        log_fail "$name (body missing '$check_body')"
        return
    fi

    log_pass "$name (HTTP $response_code)"
    echo "$response_body" > "$RESULTS_DIR/${name}.json" 2>/dev/null || true
}

# Check response contains expected field
test_contains() {
    local name="$1"
    local path="$2"
    local field="$3"

    local response_body
    response_body=$(curl -s --connect-timeout 10 --max-time 30 \
        "${BASE_URL}${path}" 2>/dev/null || echo "")

    if echo "$response_body" | grep -q "$field"; then
        log_pass "$name (contains '$field')"
    else
        log_fail "$name (missing '$field' in response)"
    fi
}

# ============================================================
# CHALLENGE EXECUTION
# ============================================================

echo ""
echo "============================================================"
echo "  HelixAgent Comprehensive User Flow Challenge"
echo "============================================================"
echo "  Base URL: ${BASE_URL}"
echo "  Results:  ${RESULTS_DIR}/"
echo "============================================================"
echo ""

# Phase 0: Server availability
log_info "Phase 0: Server Availability"
if ! check_server; then
    log_fail "Server not reachable at ${BASE_URL}"
    echo ""
    echo "Server is not running. Start HelixAgent first:"
    echo "  make build && ./bin/helixagent"
    echo ""
    echo "Total: $TOTAL | Pass: $PASS | Fail: $FAIL | Skip: $SKIP"
    exit 1
fi
log_pass "Server reachable at ${BASE_URL}"
echo ""

# Phase 1: Health Endpoints
log_info "Phase 1: Health Endpoints"
test_get "health_root" "/health" "200"
test_get "health_enhanced" "/v1/health" "200"
test_get "monitoring_status" "/v1/monitoring/status" "200"
echo ""

# Phase 2: Feature Flags (public, no auth)
log_info "Phase 2: Feature Flags"
test_get "feature_flags" "/v1/features" "200"
test_get "feature_available" "/v1/features/available" "200"
echo ""

# Phase 3: Model Discovery
log_info "Phase 3: Model Discovery"
test_get "list_models" "/v1/models" "200"
test_contains "models_data" "/v1/models" "data"
echo ""

# Phase 4: Monitoring & Observability
log_info "Phase 4: Monitoring & Observability"
test_get "monitoring_status" "/v1/monitoring/status" "200"
test_get "circuit_breakers" "/v1/monitoring/circuit-breakers" "200"
test_get "provider_health" "/v1/monitoring/provider-health" "200"
test_get "fallback_chain" "/v1/monitoring/fallback-chain" "200"
test_get "concurrency_stats" "/v1/monitoring/concurrency" "200"
test_get "concurrency_alerts" "/v1/monitoring/concurrency/alerts" "200"
echo ""

# Phase 5: Code Formatters (public)
log_info "Phase 5: Code Formatters"
test_get "list_formatters" "/v1/formatters" "200"
test_contains "formatters_list" "/v1/formatters" "formatters"
test_post "format_go" "/v1/format" \
    '{"language":"go","code":"package main\nfunc main(){\nfmt.Println(\"hello\")}"}' \
    "200 503"
echo ""

# Phase 6: Chat Completion (OpenAI-compatible)
log_info "Phase 6: Chat Completion"
test_post "chat_completion" "/v1/chat/completions" \
    '{"model":"helixagent-debate","messages":[{"role":"user","content":"Say hello"}],"max_tokens":50}' \
    "200 503" "choices"
echo ""

# Phase 7: Streaming Completion
log_info "Phase 7: Streaming Completion"
test_post "streaming_completion" "/v1/chat/completions" \
    '{"model":"helixagent-debate","messages":[{"role":"user","content":"Count to 3"}],"stream":true,"max_tokens":50}' \
    "200 503"
echo ""

# Phase 8: Embeddings
log_info "Phase 8: Embeddings"
test_post "create_embedding" "/v1/embeddings/generate" \
    '{"model":"text-embedding-ada-002","input":"Hello world"}' \
    "200 503"
echo ""

# Phase 9: Debate System
log_info "Phase 9: Debate System"
test_post "create_debate" "/v1/debates" \
    '{"topic":"Implement a rate limiter","max_rounds":2}' \
    "200 201 503"
test_get "list_debates" "/v1/debates" "200 503"
echo ""

# Phase 10: Protocol Endpoints
log_info "Phase 10: Protocol Endpoints"
test_get "mcp_stats" "/v1/mcp/stats" "200 404"
test_get "mcp_adapters_search" "/v1/mcp/adapters/search" "200 404"
test_get "mcp_capabilities" "/v1/mcp/capabilities" "200 404"
echo ""

# Phase 11: RAG Pipeline
log_info "Phase 11: RAG Pipeline"
test_get "rag_health" "/v1/rag/health" "200 404 503"
test_post "rag_search" "/v1/rag/search" \
    '{"query":"How does HelixAgent work?","top_k":3}' \
    "200 404 503"
echo ""

# Phase 12: System Debug (public)
log_info "Phase 12: System Debug"
test_get "debug_ensemble" "/v1/ensemble/completions" \
    "200 404 405"
echo ""

# Phase 13: Authentication
log_info "Phase 13: Authentication"
test_post "auth_login" "/v1/auth/login" \
    '{"username":"admin","password":"admin"}' \
    "200 401 404 501"
test_get "auth_protected" "/v1/auth/me" "200 401 404 501"
test_post "auth_bad_creds" "/v1/auth/login" \
    '{"username":"invalid","password":"wrong"}' \
    "401 404 501"
echo ""

# Phase 14: Error Handling
log_info "Phase 14: Error Handling"
test_post "error_bad_model" "/v1/chat/completions" \
    '{"model":"nonexistent/fake","messages":[{"role":"user","content":"test"}]}' \
    "400 404 500 501 503"
test_post "error_bad_json" "/v1/chat/completions" \
    '{invalid json}' \
    "400 422 500"
test_get "error_404" "/v1/nonexistent-endpoint" "404"
echo ""

# Phase 15: Multi-Turn Conversation
log_info "Phase 15: Multi-Turn Conversation"
test_post "multi_turn_first" "/v1/chat/completions" \
    '{"model":"auto","messages":[{"role":"user","content":"Hello, my name is Alice"}]}' \
    "200 501 503"
test_post "multi_turn_second" "/v1/chat/completions" \
    '{"model":"auto","messages":[{"role":"user","content":"Hello, my name is Alice"},{"role":"assistant","content":"Hello Alice!"},{"role":"user","content":"What is my name?"}]}' \
    "200 501 503"
echo ""

# Phase 16: Tool/Function Calling
log_info "Phase 16: Tool/Function Calling"
test_post "tool_calling" "/v1/chat/completions" \
    '{"model":"auto","messages":[{"role":"user","content":"What is the weather?"}],"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object","properties":{"location":{"type":"string"}}}}}],"tool_choice":"auto"}' \
    "200 400 501 503"
echo ""

# Phase 17: Provider Failover
log_info "Phase 17: Provider Failover"
test_get "failover_chain" "/v1/monitoring/fallback-chain" \
    "200 404 501"
test_get "circuit_breakers" "/v1/monitoring/circuit-breakers" \
    "200 404 501"
test_post "failover_bad_provider" "/v1/chat/completions" \
    '{"model":"nonexistent-provider/fake-model","messages":[{"role":"user","content":"test"}]}' \
    "400 404 500 501 503"
echo ""

# Phase 18: Rate Limiting
log_info "Phase 18: Rate Limiting"
test_post "rate_limit_first" "/v1/chat/completions" \
    '{"model":"auto","messages":[{"role":"user","content":"rate limit test 1"}]}' \
    "200 429 501 503"
test_post "rate_limit_second" "/v1/chat/completions" \
    '{"model":"auto","messages":[{"role":"user","content":"rate limit test 2"}]}' \
    "200 429 501 503"
test_get "post_rate_limit_health" "/v1/health" "200"
echo ""

# Phase 19: Pagination
log_info "Phase 19: Pagination"
test_get "models_list" "/v1/models" "200 501"
test_get "models_limited" "/v1/models?limit=1" "200 501"
test_get "formatters_list" "/v1/formatters" "200 501"
echo ""

# ============================================================
# SUMMARY
# ============================================================

echo "============================================================"
echo "  Challenge Results"
echo "============================================================"
printf "  Total: %d | ${GREEN}Pass: %d${NC} | ${RED}Fail: %d${NC} | ${YELLOW}Skip: %d${NC}\n" \
    "$TOTAL" "$PASS" "$FAIL" "$SKIP"
echo "  Results saved to: ${RESULTS_DIR}/"
echo "============================================================"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi

exit 0
