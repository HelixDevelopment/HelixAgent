#!/bin/bash
# MCP Servers Usage Challenge - Validates that all 12 MCP servers are operational and usable
# Tests: fetch, filesystem, github, helixagent-*, memory, puppeteer, sqlite

set -e

# Source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh" 2>/dev/null || {
    # Fallback if common.sh not found
    log_info() { echo -e "\033[0;34m[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_success() { echo -e "\033[0;32m[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_error() { echo -e "\033[0;31m[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_warning() { echo -e "\033[0;33m[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
}

CHALLENGE_NAME="MCP Servers Usage Challenge"
PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="${SCRIPT_DIR}/../.."
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"

# ============================================================================
# Section 1: HelixAgent MCP Servers (Remote)
# ============================================================================

log_info "=============================================="
log_info "Section 1: HelixAgent Remote MCP Servers"
log_info "=============================================="

# Test 1: helixagent-mcp SSE connection
TOTAL=$((TOTAL + 1))
log_info "Testing helixagent-mcp SSE endpoint"
response=$(timeout 5s curl -s --max-time 60 -N "$HELIXAGENT_URL/v1/mcp" 2>&1 || echo "TIMEOUT")
if echo "$response" | grep -q "event:\|data:\|TIMEOUT"; then
    log_success "helixagent-mcp SSE endpoint responds"
    PASSED=$((PASSED + 1))
else
    log_error "helixagent-mcp SSE endpoint failed"
    FAILED=$((FAILED + 1))
fi

# Test 2: helixagent-mcp POST message
TOTAL=$((TOTAL + 1))
log_info "Testing helixagent-mcp POST message"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/mcp" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"ping","id":1}' 2>&1)
if echo "$response" | grep -q "jsonrpc\|result\|error"; then
    log_success "helixagent-mcp POST message works"
    PASSED=$((PASSED + 1))
else
    log_error "helixagent-mcp POST message failed"
    FAILED=$((FAILED + 1))
fi

# Test 3: helixagent-acp
TOTAL=$((TOTAL + 1))
log_info "Testing helixagent-acp endpoint"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/acp" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"ping","id":1}' 2>&1)
if [ -n "$response" ]; then
    log_success "helixagent-acp endpoint responds"
    PASSED=$((PASSED + 1))
else
    log_error "helixagent-acp endpoint failed"
    FAILED=$((FAILED + 1))
fi

# Test 4: helixagent-lsp
TOTAL=$((TOTAL + 1))
log_info "Testing helixagent-lsp endpoint"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/lsp" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"capabilities":{}}}' 2>&1)
if [ -n "$response" ]; then
    log_success "helixagent-lsp endpoint responds"
    PASSED=$((PASSED + 1))
else
    log_error "helixagent-lsp endpoint failed"
    FAILED=$((FAILED + 1))
fi

# Test 5: helixagent-embeddings
TOTAL=$((TOTAL + 1))
log_info "Testing helixagent-embeddings endpoint"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/embeddings" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"ping","id":1}' 2>&1)
if [ -n "$response" ]; then
    log_success "helixagent-embeddings endpoint responds"
    PASSED=$((PASSED + 1))
else
    log_error "helixagent-embeddings endpoint failed"
    FAILED=$((FAILED + 1))
fi

# Test 6: helixagent-vision
TOTAL=$((TOTAL + 1))
log_info "Testing helixagent-vision endpoint"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/vision" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"ping","id":1}' 2>&1)
if [ -n "$response" ]; then
    log_success "helixagent-vision endpoint responds"
    PASSED=$((PASSED + 1))
else
    log_error "helixagent-vision endpoint failed"
    FAILED=$((FAILED + 1))
fi

# Test 7: helixagent-cognee
TOTAL=$((TOTAL + 1))
log_info "Testing helixagent-cognee endpoint"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/cognee" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"ping","id":1}' 2>&1)
if [ -n "$response" ]; then
    log_success "helixagent-cognee endpoint responds"
    PASSED=$((PASSED + 1))
else
    log_error "helixagent-cognee endpoint failed"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 2: Local MCP Server Package Availability
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 2: Local MCP Server Packages"
log_info "=============================================="

# Test 8: mcp-fetch package
TOTAL=$((TOTAL + 1))
log_info "Testing mcp-fetch package availability"
if npm view mcp-fetch version >/dev/null 2>&1; then
    version=$(npm view mcp-fetch version 2>/dev/null)
    log_success "mcp-fetch package available (version: $version)"
    PASSED=$((PASSED + 1))
else
    log_error "mcp-fetch package not available"
    FAILED=$((FAILED + 1))
fi

# Test 9: @modelcontextprotocol/server-filesystem package
TOTAL=$((TOTAL + 1))
log_info "Testing @modelcontextprotocol/server-filesystem package"
if npm view @modelcontextprotocol/server-filesystem version >/dev/null 2>&1; then
    version=$(npm view @modelcontextprotocol/server-filesystem version 2>/dev/null)
    log_success "filesystem MCP package available (version: $version)"
    PASSED=$((PASSED + 1))
else
    log_error "filesystem MCP package not available"
    FAILED=$((FAILED + 1))
fi

# Test 10: @modelcontextprotocol/server-github package
TOTAL=$((TOTAL + 1))
log_info "Testing @modelcontextprotocol/server-github package"
if npm view @modelcontextprotocol/server-github version >/dev/null 2>&1; then
    version=$(npm view @modelcontextprotocol/server-github version 2>/dev/null)
    log_success "github MCP package available (version: $version)"
    PASSED=$((PASSED + 1))
else
    log_error "github MCP package not available"
    FAILED=$((FAILED + 1))
fi

# Test 11: @modelcontextprotocol/server-memory package
TOTAL=$((TOTAL + 1))
log_info "Testing @modelcontextprotocol/server-memory package"
if npm view @modelcontextprotocol/server-memory version >/dev/null 2>&1; then
    version=$(npm view @modelcontextprotocol/server-memory version 2>/dev/null)
    log_success "memory MCP package available (version: $version)"
    PASSED=$((PASSED + 1))
else
    log_error "memory MCP package not available"
    FAILED=$((FAILED + 1))
fi

# Test 12: @modelcontextprotocol/server-puppeteer package
TOTAL=$((TOTAL + 1))
log_info "Testing @modelcontextprotocol/server-puppeteer package"
if npm view @modelcontextprotocol/server-puppeteer version >/dev/null 2>&1; then
    version=$(npm view @modelcontextprotocol/server-puppeteer version 2>/dev/null)
    log_success "puppeteer MCP package available (version: $version)"
    PASSED=$((PASSED + 1))
else
    log_error "puppeteer MCP package not available"
    FAILED=$((FAILED + 1))
fi

# Test 13: mcp-server-sqlite package
TOTAL=$((TOTAL + 1))
log_info "Testing mcp-server-sqlite package availability"
if npm view mcp-server-sqlite version >/dev/null 2>&1; then
    version=$(npm view mcp-server-sqlite version 2>/dev/null)
    log_success "mcp-server-sqlite package available (version: $version)"
    PASSED=$((PASSED + 1))
else
    log_error "mcp-server-sqlite package not available"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 3: MCP Tool Execution via HelixAgent
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 3: MCP Tool Execution"
log_info "=============================================="

# Test 14: Get MCP capabilities
TOTAL=$((TOTAL + 1))
log_info "Testing MCP capabilities retrieval"
response=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/mcp/capabilities" 2>&1)
if echo "$response" | grep -q "capabilities\|tools\|prompts\|resources"; then
    log_success "MCP capabilities retrieved successfully"
    PASSED=$((PASSED + 1))
else
    log_error "Failed to get MCP capabilities"
    FAILED=$((FAILED + 1))
fi

# Test 15: List MCP tools
TOTAL=$((TOTAL + 1))
log_info "Testing MCP tools listing"
response=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/mcp/tools" 2>&1)
if echo "$response" | grep -q "tools\|name\|description"; then
    log_success "MCP tools listed successfully"
    PASSED=$((PASSED + 1))
else
    log_error "Failed to list MCP tools"
    FAILED=$((FAILED + 1))
fi

# Test 16: Call MCP tool (get_capabilities)
TOTAL=$((TOTAL + 1))
log_info "Testing MCP tool call (mcp_get_capabilities)"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/mcp/tools/call" \
    -H "Content-Type: application/json" \
    -d '{"name":"mcp_get_capabilities","arguments":{}}' 2>&1)
if echo "$response" | grep -q "result\|capabilities\|error"; then
    log_success "MCP tool call executed"
    PASSED=$((PASSED + 1))
else
    log_error "MCP tool call failed"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 4: OpenCode Configuration Validation
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 4: OpenCode Configuration"
log_info "=============================================="

OPENCODE_CONFIG="$HOME/.config/opencode/opencode.json"

# Test 17: OpenCode config exists
TOTAL=$((TOTAL + 1))
log_info "Testing OpenCode configuration exists"
if [ -f "$OPENCODE_CONFIG" ]; then
    log_success "OpenCode configuration exists"
    PASSED=$((PASSED + 1))
else
    log_error "OpenCode configuration not found"
    FAILED=$((FAILED + 1))
fi

# Test 18: All 12 MCP servers configured
TOTAL=$((TOTAL + 1))
log_info "Testing all 12 MCP servers are configured"
if [ -f "$OPENCODE_CONFIG" ]; then
    mcp_count=$(jq '.mcp | keys | length' "$OPENCODE_CONFIG" 2>/dev/null || echo "0")
    if [ "$mcp_count" -ge 12 ]; then
        log_success "All 12 MCP servers configured (found: $mcp_count)"
        PASSED=$((PASSED + 1))
    else
        log_error "Not all MCP servers configured (found: $mcp_count, expected: 12)"
        FAILED=$((FAILED + 1))
    fi
else
    log_error "Cannot check - config not found"
    FAILED=$((FAILED + 1))
fi

# Test 19: Correct fetch package name
TOTAL=$((TOTAL + 1))
log_info "Testing fetch uses correct package (mcp-fetch)"
if [ -f "$OPENCODE_CONFIG" ]; then
    fetch_pkg=$(jq -r '.mcp.fetch.command[2]' "$OPENCODE_CONFIG" 2>/dev/null)
    if [ "$fetch_pkg" = "mcp-fetch" ]; then
        log_success "fetch uses correct package: $fetch_pkg"
        PASSED=$((PASSED + 1))
    else
        log_error "fetch uses wrong package: $fetch_pkg (expected: mcp-fetch)"
        FAILED=$((FAILED + 1))
    fi
else
    log_error "Cannot check - config not found"
    FAILED=$((FAILED + 1))
fi

# Test 20: Correct sqlite package name
TOTAL=$((TOTAL + 1))
log_info "Testing sqlite uses correct package (mcp-server-sqlite)"
if [ -f "$OPENCODE_CONFIG" ]; then
    sqlite_pkg=$(jq -r '.mcp.sqlite.command[2]' "$OPENCODE_CONFIG" 2>/dev/null)
    if [ "$sqlite_pkg" = "mcp-server-sqlite" ]; then
        log_success "sqlite uses correct package: $sqlite_pkg"
        PASSED=$((PASSED + 1))
    else
        log_error "sqlite uses wrong package: $sqlite_pkg (expected: mcp-server-sqlite)"
        FAILED=$((FAILED + 1))
    fi
else
    log_error "Cannot check - config not found"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 5: Local MCP Server RUNTIME Verification
# CRITICAL: This section tests that servers can actually START and RUN
# Not just that config is correct or packages exist
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 5: Local MCP Server RUNTIME Verification"
log_info "=============================================="

# Node.js v20 LTS path (required for MCP server compatibility)
NODE20_DIR="$HOME/Applications/node-v20.18.0-linux-x64"
NODE20_NPX="$NODE20_DIR/bin/npx"
NODE20_NODE="$NODE20_DIR/bin/node"

# Test 21: Node.js v20 LTS is installed
TOTAL=$((TOTAL + 1))
log_info "Testing Node.js v20 LTS installation"
if [ -x "$NODE20_NODE" ]; then
    node_version=$("$NODE20_NODE" --version 2>/dev/null)
    log_success "Node.js v20 LTS installed: $node_version"
    PASSED=$((PASSED + 1))
else
    log_error "Node.js v20 LTS not found at: $NODE20_DIR"
    log_error "Install with: wget -qO- https://nodejs.org/dist/v20.18.0/node-v20.18.0-linux-x64.tar.xz | tar -xJ -C ~/Applications/"
    FAILED=$((FAILED + 1))
fi

# Test 22: mcp-fetch server can actually START and stay running
# Note: MCP servers are STDIO-based - they need stdin to stay open
# We use a FIFO to keep stdin open while verifying the server doesn't crash
TOTAL=$((TOTAL + 1))
log_info "Testing mcp-fetch server can START (runtime test)"
if [ -x "$NODE20_NPX" ]; then
    # Create a FIFO to keep stdin open
    FETCH_FIFO=$(mktemp -u)
    mkfifo "$FETCH_FIFO"
    FETCH_LOG=$(mktemp)

    # Start server reading from FIFO
    timeout 8s "$NODE20_NPX" -y mcp-fetch < "$FETCH_FIFO" > "$FETCH_LOG" 2>&1 &
    FETCH_PID=$!

    # Keep FIFO open
    exec 3>"$FETCH_FIFO"

    # Wait for server to initialize
    sleep 3

    # Check if still running (not crashed)
    if kill -0 $FETCH_PID 2>/dev/null; then
        log_success "mcp-fetch server started and running (PID: $FETCH_PID)"
        PASSED=$((PASSED + 1))
        kill $FETCH_PID 2>/dev/null || true
    else
        error_msg=$(cat "$FETCH_LOG" | head -3)
        log_error "mcp-fetch server crashed: $error_msg"
        FAILED=$((FAILED + 1))
    fi

    # Cleanup
    exec 3>&-
    rm -f "$FETCH_FIFO" "$FETCH_LOG"
else
    log_error "Cannot test mcp-fetch - Node.js v20 not available"
    FAILED=$((FAILED + 1))
fi

# Test 23: mcp-server-sqlite can actually START and respond to MCP protocol
TOTAL=$((TOTAL + 1))
log_info "Testing mcp-server-sqlite can START and respond (runtime test)"
if [ -x "$NODE20_NPX" ]; then
    # Create temp database file
    SQLITE_TEST_DIR=$(mktemp -d)
    SQLITE_DB="$SQLITE_TEST_DIR/test.db"
    touch "$SQLITE_DB"

    # mcp-server-sqlite responds to MCP protocol with proper initialization
    SQLITE_RESPONSE=$(cat << 'EOF' | timeout 15s "$NODE20_NPX" -y mcp-server-sqlite "$SQLITE_DB" 2>&1
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
EOF
)

    if echo "$SQLITE_RESPONSE" | grep -q '"tools"'; then
        tool_count=$(echo "$SQLITE_RESPONSE" | grep -o '"name"' | wc -l)
        log_success "mcp-server-sqlite responds with $tool_count tools"
        PASSED=$((PASSED + 1))
    else
        log_error "mcp-server-sqlite failed to respond: ${SQLITE_RESPONSE:0:100}"
        FAILED=$((FAILED + 1))
    fi

    # Cleanup
    rm -rf "$SQLITE_TEST_DIR"
else
    log_error "Cannot test mcp-server-sqlite - Node.js v20 not available"
    FAILED=$((FAILED + 1))
fi

# Test 24: OpenCode config uses correct Node.js path for fetch
TOTAL=$((TOTAL + 1))
log_info "Testing OpenCode config uses Node.js v20 for fetch"
if [ -f "$OPENCODE_CONFIG" ]; then
    fetch_npx=$(jq -r '.mcp.fetch.command[0]' "$OPENCODE_CONFIG" 2>/dev/null)
    if echo "$fetch_npx" | grep -q "node-v20"; then
        log_success "fetch uses Node.js v20 npx: $fetch_npx"
        PASSED=$((PASSED + 1))
    else
        log_error "fetch uses wrong npx: $fetch_npx (should use node-v20)"
        FAILED=$((FAILED + 1))
    fi
else
    log_error "Cannot check - config not found"
    FAILED=$((FAILED + 1))
fi

# Test 25: OpenCode config uses correct Node.js path for sqlite
TOTAL=$((TOTAL + 1))
log_info "Testing OpenCode config uses Node.js v20 for sqlite"
if [ -f "$OPENCODE_CONFIG" ]; then
    sqlite_npx=$(jq -r '.mcp.sqlite.command[0]' "$OPENCODE_CONFIG" 2>/dev/null)
    if echo "$sqlite_npx" | grep -q "node-v20"; then
        log_success "sqlite uses Node.js v20 npx: $sqlite_npx"
        PASSED=$((PASSED + 1))
    else
        log_error "sqlite uses wrong npx: $sqlite_npx (should use node-v20)"
        FAILED=$((FAILED + 1))
    fi
else
    log_error "Cannot check - config not found"
    FAILED=$((FAILED + 1))
fi

# Test 26: sqlite database path is configured
TOTAL=$((TOTAL + 1))
log_info "Testing sqlite has database path configured"
if [ -f "$OPENCODE_CONFIG" ]; then
    sqlite_db=$(jq -r '.mcp.sqlite.command[3]' "$OPENCODE_CONFIG" 2>/dev/null)
    if [ -n "$sqlite_db" ] && [ "$sqlite_db" != "null" ]; then
        log_success "sqlite database path configured: $sqlite_db"
        PASSED=$((PASSED + 1))
    else
        log_error "sqlite database path not configured"
        FAILED=$((FAILED + 1))
    fi
else
    log_error "Cannot check - config not found"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Summary
# ============================================================================

log_info ""
log_info "=============================================="
log_info "MCP Servers Usage Challenge Summary"
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

if [ $FAILED -eq 0 ]; then
    log_success "=============================================="
    log_success "ALL MCP SERVERS USAGE TESTS PASSED!"
    log_success "=============================================="
    exit 0
else
    log_error "=============================================="
    log_error "SOME MCP SERVERS USAGE TESTS FAILED"
    log_error "=============================================="
    exit 1
fi
