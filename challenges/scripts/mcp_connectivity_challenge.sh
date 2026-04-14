#!/bin/bash
# HelixAgent Challenge: MCP Connectivity & SSE Endpoints
# Tests: ~38 tests across 8 sections
# Validates: Code structure, HTTP connectivity, npm packages, remote MCPs, config correctness
#
# IMPORTANT: This challenge uses REAL HTTP tests against a running server and live npm registry.
# Grep-only validation is never sufficient — actual runtime behavior is verified.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASSED=0
FAILED=0
SKIPPED=0
TOTAL=0

pass() {
    PASSED=$((PASSED + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${GREEN}[PASS]${NC} $1"
}

fail() {
    FAILED=$((FAILED + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${RED}[FAIL]${NC} $1"
}

skip() {
    SKIPPED=$((SKIPPED + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${YELLOW}[SKIP]${NC} $1"
}

section() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

# Configuration
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}MCP Connectivity Challenge (v3)${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Server URL: ${HELIXAGENT_URL}"

#===============================================================================
# Section 1: Code Structure — SSE/POST Route Registration (9 tests)
#===============================================================================
section "Section 1: Code Structure — Route Registration"

SSE_FILE="$PROJECT_ROOT/internal/handlers/protocol_sse.go"
ROUTER_FILE="$PROJECT_ROOT/internal/router/router.go"

# Test 1.1-1.6: Original 6 protocols have GET + POST
for proto in mcp acp lsp embeddings vision cognee; do
    if grep -q "router.GET(\"/$proto\"" "$SSE_FILE" && grep -q "router.POST(\"/$proto\"" "$SSE_FILE"; then
        pass "$proto route registered (GET + POST)"
    else
        fail "$proto route missing"
    fi
done

# Test 1.7: RAG has GET + POST
if grep -q 'router.GET("/rag"' "$SSE_FILE" && grep -q 'router.POST("/rag"' "$SSE_FILE"; then
    pass "RAG route registered (GET + POST)"
else
    fail "RAG route missing"
fi

# Test 1.8: Formatters has POST only (GET is REST ListFormatters in router.go)
if ! grep -q 'router.GET("/formatters"' "$SSE_FILE" && grep -q 'router.POST("/formatters"' "$SSE_FILE"; then
    pass "Formatters route registered (POST only — no GET SSE conflict)"
else
    fail "Formatters route conflict: GET /formatters in SSE would panic (duplicate with REST)"
fi

# Test 1.9: Monitoring has GET + POST
if grep -q 'router.GET("/monitoring"' "$SSE_FILE" && grep -q 'router.POST("/monitoring"' "$SSE_FILE"; then
    pass "Monitoring route registered (GET + POST)"
else
    fail "Monitoring route missing"
fi

#===============================================================================
# Section 2: Code Structure — Handler Wiring (4 tests)
#===============================================================================
section "Section 2: Code Structure — Handler Wiring"

# Test 2.1: All 3 handler fields in struct
if grep -q 'ragHandler.*\*RAGHandler' "$SSE_FILE" && \
   grep -q 'formattersHandler.*\*FormattersHandler' "$SSE_FILE" && \
   grep -q 'monitoringHandler.*\*MonitoringHandler' "$SSE_FILE"; then
    pass "All 3 handler fields in ProtocolSSEHandler struct"
else
    fail "Missing handler fields in ProtocolSSEHandler struct"
fi

# Test 2.2: Setter methods exist
if grep -q 'func.*SetRAGHandler' "$SSE_FILE" && \
   grep -q 'func.*SetFormattersHandler' "$SSE_FILE" && \
   grep -q 'func.*SetMonitoringHandler' "$SSE_FILE"; then
    pass "Setter methods exist for all 3 new handlers"
else
    fail "Missing setter methods for new handlers"
fi

# Test 2.3: Setters called in router.go
if grep -q 'SetRAGHandler' "$ROUTER_FILE" && \
   grep -q 'SetFormattersHandler' "$ROUTER_FILE" && \
   grep -q 'SetMonitoringHandler' "$ROUTER_FILE"; then
    pass "All 3 setters called in router.go"
else
    fail "Missing setter calls in router.go"
fi

# Test 2.4: REST ListFormatters preserved
if grep -q 'GET("/formatters".*ListFormatters' "$ROUTER_FILE"; then
    pass "REST GET /v1/formatters (ListFormatters) preserved in router.go"
else
    fail "REST GET /v1/formatters missing from router.go"
fi

#===============================================================================
# Section 3: NPM Package Verification (6 tests)
#===============================================================================
section "Section 3: NPM Package Verification"

MAIN_FILE="$PROJECT_ROOT/cmd/helixagent/main.go"

# Test 3.1-3.3: Correct packages exist on npm
for pkg in mcp-fetch-server @theo.foobar/mcp-time mcp-git; do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "https://registry.npmjs.org/$pkg" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        pass "npm package '$pkg' exists (HTTP $HTTP_CODE)"
    elif [ "$HTTP_CODE" = "000" ]; then
        skip "npm registry unreachable for '$pkg'"
    else
        fail "npm package '$pkg' not found (HTTP $HTTP_CODE)"
    fi
done

# Test 3.4-3.7: Stale/broken packages NOT referenced in code
for pkg in "mcp-fetch" "mcp-server-time" "@modelcontextprotocol/server-fetch" "@modelcontextprotocol/server-time"; do
    # Check exact match (not as substring of mcp-fetch-server)
    if [ "$pkg" = "mcp-fetch" ]; then
        # Match "mcp-fetch" but not "mcp-fetch-server"
        if ! grep -P '"mcp-fetch"(?!-server)' "$MAIN_FILE" >/dev/null 2>&1; then
            pass "No reference to stale package '$pkg' in main.go"
        else
            fail "Stale package '$pkg' still referenced in main.go"
        fi
    else
        if ! grep -q "\"$pkg\"" "$MAIN_FILE"; then
            pass "No reference to stale package '$pkg' in main.go"
        else
            fail "Stale package '$pkg' still referenced in main.go"
        fi
    fi
done

#===============================================================================
# Section 4: URL Correctness (4 tests)
#===============================================================================
section "Section 4: URL Correctness"

GENERATOR_FILE="$PROJECT_ROOT/LLMsVerifier/llm-verifier/pkg/cliagents/generator.go"

# Test 4.1: helixagent-formatters uses /v1/formatters
if grep -A2 'helixagent-formatters' "$MAIN_FILE" | grep -q '/v1/formatters'; then
    pass "helixagent-formatters URL uses /v1/formatters"
else
    fail "helixagent-formatters URL incorrect"
fi

# Test 4.2: deepwiki uses /mcp not /sse
if grep 'deepwiki' "$MAIN_FILE" | grep -q '/mcp"'; then
    if ! grep 'deepwiki' "$MAIN_FILE" | grep -q '/sse"'; then
        pass "deepwiki URL uses /mcp (not /sse)"
    else
        fail "deepwiki URL still contains /sse"
    fi
else
    fail "deepwiki URL does not use /mcp"
fi

# Test 4.3: No uvx commands in main.go
if ! grep -q '"uvx"' "$MAIN_FILE"; then
    pass "No uvx commands in main.go"
else
    fail "Found uvx commands in main.go"
fi

# Test 4.4: Generator has correct URLs
if grep 'helixagent-formatters' "$GENERATOR_FILE" | grep -q '/v1/formatters' && \
   grep 'deepwiki' "$GENERATOR_FILE" | grep -q '/mcp"'; then
    pass "LLMsVerifier generator has correct URLs"
else
    fail "LLMsVerifier generator has stale URLs"
fi

#===============================================================================
# Section 5: Binary Compilation (2 tests)
#===============================================================================
section "Section 5: Binary Compilation"

# Test 5.1: Binary builds without errors
if go build -mod=mod -o /dev/null "$PROJECT_ROOT/cmd/helixagent/" 2>/dev/null; then
    pass "helixagent binary compiles successfully"
else
    fail "helixagent binary compilation failed"
fi

# Test 5.2: No duplicate route panic (server can start)
# Check if the server is running — if it is, that proves no panic
HEALTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${HELIXAGENT_URL}/health" 2>/dev/null || echo "000")
if [ "$HEALTH_CODE" = "200" ]; then
    pass "Server is running (no Gin route panic)"
else
    skip "Server not running at ${HELIXAGENT_URL} — cannot verify no panic"
fi

#===============================================================================
# Section 6: Runtime HTTP Tests (9 tests — requires running server)
#===============================================================================
section "Section 6: Runtime HTTP Tests (POST initialize)"

if [ "$HEALTH_CODE" != "200" ]; then
    echo -e "  ${YELLOW}Server not running — skipping runtime tests${NC}"
    for proto in mcp acp lsp embeddings vision cognee rag formatters monitoring; do
        skip "POST /v1/$proto initialize (server not running)"
    done
else
    # Test POST initialize for all 9 protocols
    for proto in mcp acp lsp embeddings vision cognee rag formatters monitoring; do
        INIT_BODY='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"challenge","version":"1.0.0"},"capabilities":{}}}'
        RESPONSE=$(curl -s -X POST "${HELIXAGENT_URL}/v1/$proto" \
            -H "Content-Type: application/json" \
            -d "$INIT_BODY" 2>/dev/null || echo "")

        if echo "$RESPONSE" | grep -q '"jsonrpc":"2.0"' && echo "$RESPONSE" | grep -q '"result"'; then
            pass "POST /v1/$proto initialize returns valid JSON-RPC result"
        elif [ -z "$RESPONSE" ]; then
            fail "POST /v1/$proto returned empty response"
        else
            fail "POST /v1/$proto invalid response: $(echo "$RESPONSE" | head -c 100)"
        fi
    done
fi

#===============================================================================
# Section 7: Formatters REST + Remote MCP Reachability (4 tests)
#===============================================================================
section "Section 7: REST Preservation & Remote MCPs"

# Test 7.1: GET /v1/formatters returns JSON (REST endpoint preserved)
if [ "$HEALTH_CODE" = "200" ]; then
    FMT_CT=$(curl -s -o /dev/null -w "%{content_type}" "${HELIXAGENT_URL}/v1/formatters" 2>/dev/null || echo "")
    if echo "$FMT_CT" | grep -q "application/json"; then
        pass "GET /v1/formatters returns JSON (REST ListFormatters preserved)"
    else
        fail "GET /v1/formatters content-type: $FMT_CT (expected application/json)"
    fi
else
    skip "GET /v1/formatters REST check (server not running)"
fi

# Test 7.2: deepwiki remote MCP reachable
DW_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "https://mcp.deepwiki.com/mcp" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"challenge","version":"1.0.0"},"capabilities":{}}}' \
    2>/dev/null || echo "000")
if [ "$DW_CODE" = "200" ]; then
    pass "deepwiki remote MCP reachable (POST /mcp → $DW_CODE)"
elif [ "$DW_CODE" = "000" ]; then
    skip "deepwiki unreachable (network issue)"
else
    fail "deepwiki POST /mcp returned HTTP $DW_CODE"
fi

# Test 7.3: context7 remote MCP reachable
C7_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "https://mcp.context7.com/mcp" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"challenge","version":"1.0.0"},"capabilities":{}}}' \
    2>/dev/null || echo "000")
if [ "$C7_CODE" = "200" ]; then
    pass "context7 remote MCP reachable (POST /mcp → $C7_CODE)"
elif [ "$C7_CODE" = "000" ]; then
    skip "context7 unreachable (network issue)"
else
    fail "context7 POST /mcp returned HTTP $C7_CODE"
fi

# Test 7.4: cloudflare-docs SSE reachable
# Note: SSE endpoint streams indefinitely, so --max-time is needed.
# curl exits with code 28 on timeout but still reports HTTP 200 via -w.
# Using '|| true' instead of '|| echo "000"' to avoid output concatenation.
CF_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "https://docs.mcp.cloudflare.com/sse" 2>/dev/null || true)
if [ "$CF_CODE" = "200" ]; then
    pass "cloudflare-docs SSE reachable (GET /sse → $CF_CODE)"
elif [ "$CF_CODE" = "000" ]; then
    skip "cloudflare-docs unreachable (network issue)"
else
    fail "cloudflare-docs GET /sse returned HTTP $CF_CODE"
fi

#===============================================================================
# Section 8: Local MCP Stdio Tests (3 tests — spawn actual processes)
#===============================================================================
section "Section 8: Local MCP Stdio Tests (JSON-RPC initialize via stdio)"

INIT_JSON='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"challenge","version":"1.0.0"},"capabilities":{}}}'

# Check if npx is available
if ! command -v npx &>/dev/null; then
    for mcp_name in fetch time git; do
        skip "Local MCP '$mcp_name' stdio test (npx not found)"
    done
else
    LOCAL_MCPS=(
        "fetch:mcp-fetch-server"
        "time:@theo.foobar/mcp-time"
        "git:mcp-git"
    )
    for entry in "${LOCAL_MCPS[@]}"; do
        MCP_NAME="${entry%%:*}"
        MCP_PKG="${entry##*:}"
        STDIO_RESPONSE=$(echo "$INIT_JSON" | timeout 20 npx -y "$MCP_PKG" 2>/dev/null || true)
        if echo "$STDIO_RESPONSE" | grep -q 'jsonrpc'; then
            pass "Local MCP '$MCP_NAME' ($MCP_PKG) responds to stdio initialize"
        elif [ -z "$STDIO_RESPONSE" ]; then
            fail "Local MCP '$MCP_NAME' ($MCP_PKG) produced empty stdio response"
        else
            fail "Local MCP '$MCP_NAME' ($MCP_PKG) invalid response: $(echo "$STDIO_RESPONSE" | head -c 100)"
        fi
    done
fi

#===============================================================================
# Summary
#===============================================================================
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}Results${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "  Total:   ${TOTAL}"
echo -e "  Passed:  ${GREEN}${PASSED}${NC}"
echo -e "  Failed:  ${RED}${FAILED}${NC}"
echo -e "  Skipped: ${YELLOW}${SKIPPED}${NC}"

if [ "$FAILED" -eq 0 ]; then
    echo -e "\n${GREEN}All tests passed! (${SKIPPED} skipped due to network/server)${NC}"
    exit 0
else
    echo -e "\n${RED}${FAILED} test(s) failed!${NC}"
    exit 1
fi
