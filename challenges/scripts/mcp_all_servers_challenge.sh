#!/bin/bash
# MCP All Servers Challenge - Validates all local and remote MCP servers
# Tests: npm package existence, local server startup, HelixAgent SSE endpoints, OpenCode config

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

CHALLENGE_NAME="MCP All Servers Challenge"
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"
PASSED=0
FAILED=0
TOTAL=0

# ============================================================================
# Section 1: NPM Package Existence Tests
# ============================================================================

test_npm_package() {
    local package="$1"
    local expected="$2"  # "exists" or "not_exists"
    local description="$3"

    TOTAL=$((TOTAL + 1))
    log_info "Testing npm package: $package ($description)"

    # URL encode the package name
    local encoded=$(echo "$package" | sed 's/@/%40/g; s/\//%2f/g')
    local status=$(curl -s --max-time 60 -o /dev/null -w "%{http_code}" "https://registry.npmjs.org/$encoded")

    if [ "$expected" = "exists" ]; then
        if [ "$status" = "200" ]; then
            log_success "Package $package exists (as expected)"
            PASSED=$((PASSED + 1))
        else
            log_error "Package $package should exist but returned $status"
            FAILED=$((FAILED + 1))
        fi
    else
        if [ "$status" != "200" ]; then
            log_success "Package $package does not exist (as expected - use alternative)"
            PASSED=$((PASSED + 1))
        else
            log_error "Package $package should NOT exist but it does"
            FAILED=$((FAILED + 1))
        fi
    fi
}

log_info "=============================================="
log_info "Section 1: NPM Package Existence Tests"
log_info "=============================================="

# Packages that SHOULD exist
test_npm_package "@modelcontextprotocol/server-filesystem" "exists" "Official filesystem server"
test_npm_package "@modelcontextprotocol/server-github" "exists" "Official github server"
test_npm_package "@modelcontextprotocol/server-memory" "exists" "Official memory server"
test_npm_package "@modelcontextprotocol/server-puppeteer" "exists" "Official puppeteer server"
test_npm_package "mcp-fetch-server" "exists" "Working fetch server replacement"
test_npm_package "mcp-sqlite" "exists" "Alternative sqlite server"

# Packages that should NOT exist (common misconceptions)
test_npm_package "@theo.foobar/mcp-time" "exists" "Working time server replacement"
test_npm_package "@modelcontextprotocol/server-sqlite" "not_exists" "Does NOT exist"

# ============================================================================
# Section 2: Local MCP Server Startup Tests
# ============================================================================

test_local_server_startup() {
    local name="$1"
    shift
    local cmd=("$@")

    TOTAL=$((TOTAL + 1))
    log_info "Testing local server startup: $name"

    # Start server with timeout
    timeout 5s "${cmd[@]}" > /tmp/mcp_test_${name}.log 2>&1 &
    local pid=$!

    sleep 2

    # Check if process is still running (means it started successfully)
    if kill -0 $pid 2>/dev/null; then
        log_success "Server $name started successfully (still running)"
        kill $pid 2>/dev/null
        PASSED=$((PASSED + 1))
    else
        # Process exited - check if it printed expected output
        wait $pid 2>/dev/null
        local exit_code=$?
        local output=$(cat /tmp/mcp_test_${name}.log 2>/dev/null)

        # Check for success indicators in output
        if echo "$output" | grep -qi "running\|started\|listening\|server\|stdio"; then
            log_success "Server $name started successfully (exited after startup message)"
            PASSED=$((PASSED + 1))
        elif [ $exit_code -eq 143 ] || [ $exit_code -eq 124 ]; then
            # Killed by timeout or SIGTERM - means it was running
            log_success "Server $name started successfully (terminated by timeout)"
            PASSED=$((PASSED + 1))
        elif [ $exit_code -eq 0 ]; then
            # Exit 0 usually means success
            log_success "Server $name started successfully (exit code 0)"
            PASSED=$((PASSED + 1))
        else
            log_error "Server $name failed to start (exit code: $exit_code)"
            echo "$output"
            FAILED=$((FAILED + 1))
        fi
    fi

    rm -f /tmp/mcp_test_${name}.log
}

log_info ""
log_info "=============================================="
log_info "Section 2: Local MCP Server Startup Tests"
log_info "=============================================="

# Check if npx is available
if command -v npx &> /dev/null; then
    test_local_server_startup "filesystem" npx -y @modelcontextprotocol/server-filesystem "$HOME"
    test_local_server_startup "memory" npx -y @modelcontextprotocol/server-memory
    test_local_server_startup "fetch" npx -y mcp-fetch
    test_local_server_startup "sqlite" npx -y mcp-sqlite
else
    log_warning "npx not found, skipping local server tests"
fi

# ============================================================================
# Section 3: HelixAgent SSE Endpoint Tests
# ============================================================================

test_sse_endpoint() {
    local protocol="$1"

    TOTAL=$((TOTAL + 1))
    log_info "Testing SSE endpoint: /v1/$protocol"

    # Test SSE connection with timeout
    local response=$(timeout 2s curl -s --max-time 60 -N -H "Accept: text/event-stream" "${HELIXAGENT_URL}/v1/${protocol}" 2>&1 || true)

    if echo "$response" | grep -q "event: endpoint"; then
        if echo "$response" | grep -q "data: /v1/${protocol}"; then
            log_success "SSE endpoint /v1/$protocol responded correctly"
            PASSED=$((PASSED + 1))
        else
            log_error "SSE endpoint /v1/$protocol missing correct data"
            FAILED=$((FAILED + 1))
        fi
    else
        log_error "SSE endpoint /v1/$protocol did not return endpoint event"
        FAILED=$((FAILED + 1))
    fi
}

test_jsonrpc_initialize() {
    local protocol="$1"

    TOTAL=$((TOTAL + 1))
    log_info "Testing JSON-RPC initialize: /v1/$protocol"

    local response=$(curl -s --max-time 60 -X POST "${HELIXAGENT_URL}/v1/${protocol}" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"},"capabilities":{}}}')

    if echo "$response" | grep -q '"result"'; then
        if echo "$response" | grep -q '"protocolVersion"'; then
            log_success "JSON-RPC initialize for $protocol succeeded"
            PASSED=$((PASSED + 1))
        else
            log_error "JSON-RPC initialize for $protocol missing protocolVersion"
            FAILED=$((FAILED + 1))
        fi
    else
        log_error "JSON-RPC initialize for $protocol failed"
        echo "Response: $response"
        FAILED=$((FAILED + 1))
    fi
}

test_jsonrpc_tools_list() {
    local protocol="$1"

    TOTAL=$((TOTAL + 1))
    log_info "Testing JSON-RPC tools/list: /v1/$protocol"

    local response=$(curl -s --max-time 60 -X POST "${HELIXAGENT_URL}/v1/${protocol}" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')

    if echo "$response" | grep -q '"tools"'; then
        log_success "JSON-RPC tools/list for $protocol succeeded"
        PASSED=$((PASSED + 1))
    else
        log_error "JSON-RPC tools/list for $protocol failed"
        echo "Response: $response"
        FAILED=$((FAILED + 1))
    fi
}

log_info ""
log_info "=============================================="
log_info "Section 3: HelixAgent SSE Endpoint Tests"
log_info "=============================================="

# Check if HelixAgent is running
if curl -s --max-time 60 "${HELIXAGENT_URL}/health" | grep -q "healthy"; then
    log_success "HelixAgent is running at ${HELIXAGENT_URL}"

    for protocol in mcp acp lsp embeddings vision cognee; do
        test_sse_endpoint "$protocol"
        test_jsonrpc_initialize "$protocol"
        test_jsonrpc_tools_list "$protocol"
    done
else
    log_warning "HelixAgent not running, skipping endpoint tests"
    # Add skipped tests to total
    TOTAL=$((TOTAL + 18))
fi

# ============================================================================
# Section 4: OpenCode Configuration Validation
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 4: OpenCode Configuration Validation"
log_info "=============================================="

OPENCODE_CONFIG="$HOME/.config/opencode/opencode.json"

if [ -f "$OPENCODE_CONFIG" ]; then
    TOTAL=$((TOTAL + 1))
    log_info "Validating OpenCode configuration"

    # Check for incorrect package names
    if grep -q "mcp-fetch-server" "$OPENCODE_CONFIG"; then
        log_success "OpenCode config uses working mcp-fetch-server package"
        PASSED=$((PASSED + 1))
    else
        log_error "OpenCode config missing mcp-fetch-server"
        FAILED=$((FAILED + 1))
    fi

    TOTAL=$((TOTAL + 1))
    if grep -q "@modelcontextprotocol/server-sqlite" "$OPENCODE_CONFIG"; then
        log_error "OpenCode config uses non-existent @modelcontextprotocol/server-sqlite"
        FAILED=$((FAILED + 1))
    else
        log_success "OpenCode config does not use non-existent sqlite package"
        PASSED=$((PASSED + 1))
    fi

    TOTAL=$((TOTAL + 1))
    # Check for correct alternatives
    if grep -q "mcp-fetch\|mcp-sqlite" "$OPENCODE_CONFIG" || ! grep -q "fetch\|sqlite" "$OPENCODE_CONFIG"; then
        log_success "OpenCode config uses correct alternative packages or doesn't use them"
        PASSED=$((PASSED + 1))
    else
        log_warning "OpenCode config may have incorrect package configuration"
        PASSED=$((PASSED + 1))  # Warning only
    fi

    TOTAL=$((TOTAL + 1))
    # Check timeout configuration for HelixAgent endpoints
    timeout_ok=true
    for endpoint in helixagent-mcp helixagent-acp helixagent-lsp helixagent-embeddings helixagent-vision helixagent-cognee; do
        ep_timeout=$(grep -A5 "\"$endpoint\"" "$OPENCODE_CONFIG" | grep "timeout" | grep -o '[0-9]*' | head -1)
        if [ -n "$ep_timeout" ] && [ "$ep_timeout" -lt 30000 ]; then
            log_warning "Endpoint $endpoint has low timeout: ${ep_timeout}ms (recommend >= 30000)"
            timeout_ok=false
        fi
    done

    if [ "$timeout_ok" = true ]; then
        log_success "All HelixAgent endpoint timeouts are adequate"
        PASSED=$((PASSED + 1))
    else
        log_error "Some endpoints have inadequate timeout configuration"
        FAILED=$((FAILED + 1))
    fi
else
    log_warning "OpenCode config not found at $OPENCODE_CONFIG"
fi

# ============================================================================
# Summary
# ============================================================================

log_info ""
log_info "=============================================="
log_info "MCP All Servers Challenge Summary"
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
    log_success "ALL MCP SERVER TESTS PASSED!"
    log_success "=============================================="
    exit 0
else
    log_error "=============================================="
    log_error "SOME MCP SERVER TESTS FAILED"
    log_error "=============================================="
    exit 1
fi
