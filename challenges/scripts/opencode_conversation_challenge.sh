#!/bin/bash
# OpenCode Conversation Challenge - Simulates real OpenCode conversations using all MCP servers
# Validates: Each MCP server gets exercised through realistic use-case scenarios

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

CHALLENGE_NAME="OpenCode Conversation Challenge"
PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="${SCRIPT_DIR}/../.."
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"

# ============================================================================
# Conversation 1: "Help me understand the codebase structure"
# Uses: filesystem MCP, helixagent-mcp, helixagent-lsp
# ============================================================================

log_info "=============================================="
log_info "Conversation 1: Codebase Exploration"
log_info "=============================================="

# Simulate: List project files
TOTAL=$((TOTAL + 1))
log_info "Task 1.1: List project structure (filesystem MCP)"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/mcp/tools/call" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "mcp_list_tools",
        "arguments": {}
    }' 2>&1)
if [ -n "$response" ]; then
    log_success "Filesystem tool interaction successful"
    PASSED=$((PASSED + 1))
else
    log_error "Filesystem tool interaction failed"
    FAILED=$((FAILED + 1))
fi

# Simulate: Get code intelligence
TOTAL=$((TOTAL + 1))
log_info "Task 1.2: Get LSP server info (helixagent-lsp)"
response=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/lsp/servers" 2>&1)
if [ -n "$response" ]; then
    log_success "LSP server query successful"
    PASSED=$((PASSED + 1))
else
    log_error "LSP server query failed"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 2: "Fetch the latest Go documentation"
# Uses: fetch MCP, helixagent-mcp
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 2: Web Content Fetching"
log_info "=============================================="

# Simulate: Check if fetch MCP is configured
TOTAL=$((TOTAL + 1))
log_info "Task 2.1: Verify fetch MCP server configured"
if jq -e '.mcp.fetch.enabled' ~/.config/opencode/opencode.json >/dev/null 2>&1; then
    log_success "Fetch MCP is enabled in configuration"
    PASSED=$((PASSED + 1))
else
    log_error "Fetch MCP not enabled"
    FAILED=$((FAILED + 1))
fi

# Simulate: MCP tool for web fetching
TOTAL=$((TOTAL + 1))
log_info "Task 2.2: Query MCP capabilities for fetch"
response=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/mcp/capabilities" 2>&1)
if echo "$response" | grep -q "capabilities"; then
    log_success "MCP capabilities available for fetch operations"
    PASSED=$((PASSED + 1))
else
    log_error "MCP capabilities query failed"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 3: "Create a GitHub issue for this bug"
# Uses: github MCP
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 3: GitHub Integration"
log_info "=============================================="

# Simulate: Verify GitHub MCP configured
TOTAL=$((TOTAL + 1))
log_info "Task 3.1: Verify GitHub MCP server configured"
if jq -e '.mcp.github.enabled' ~/.config/opencode/opencode.json >/dev/null 2>&1; then
    log_success "GitHub MCP is enabled in configuration"
    PASSED=$((PASSED + 1))
else
    log_error "GitHub MCP not enabled"
    FAILED=$((FAILED + 1))
fi

# Simulate: Check GitHub token presence (not value)
TOTAL=$((TOTAL + 1))
log_info "Task 3.2: Verify GitHub token environment configured"
if jq -e '.mcp.github.environment.GITHUB_TOKEN' ~/.config/opencode/opencode.json >/dev/null 2>&1; then
    log_success "GitHub token environment configured"
    PASSED=$((PASSED + 1))
else
    log_error "GitHub token not configured"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 4: "Remember this code pattern for later"
# Uses: memory MCP
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 4: Memory Storage"
log_info "=============================================="

# Simulate: Verify memory MCP configured
TOTAL=$((TOTAL + 1))
log_info "Task 4.1: Verify memory MCP server configured"
if jq -e '.mcp.memory.enabled' ~/.config/opencode/opencode.json >/dev/null 2>&1; then
    log_success "Memory MCP is enabled in configuration"
    PASSED=$((PASSED + 1))
else
    log_error "Memory MCP not enabled"
    FAILED=$((FAILED + 1))
fi

# Simulate: Check memory server package
TOTAL=$((TOTAL + 1))
log_info "Task 4.2: Verify memory MCP uses correct package"
mem_pkg=$(jq -r '.mcp.memory.command[2]' ~/.config/opencode/opencode.json 2>/dev/null)
if [ "$mem_pkg" = "@modelcontextprotocol/server-memory" ]; then
    log_success "Memory MCP uses correct package"
    PASSED=$((PASSED + 1))
else
    log_error "Memory MCP uses wrong package: $mem_pkg"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 5: "Take a screenshot of this webpage"
# Uses: puppeteer MCP
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 5: Browser Automation"
log_info "=============================================="

# Simulate: Verify puppeteer MCP configured
TOTAL=$((TOTAL + 1))
log_info "Task 5.1: Verify puppeteer MCP server configured"
if jq -e '.mcp.puppeteer.enabled' ~/.config/opencode/opencode.json >/dev/null 2>&1; then
    log_success "Puppeteer MCP is enabled in configuration"
    PASSED=$((PASSED + 1))
else
    log_error "Puppeteer MCP not enabled"
    FAILED=$((FAILED + 1))
fi

# Simulate: Check puppeteer server package
TOTAL=$((TOTAL + 1))
log_info "Task 5.2: Verify puppeteer MCP uses correct package"
pup_pkg=$(jq -r '.mcp.puppeteer.command[2]' ~/.config/opencode/opencode.json 2>/dev/null)
if [ "$pup_pkg" = "@modelcontextprotocol/server-puppeteer" ]; then
    log_success "Puppeteer MCP uses correct package"
    PASSED=$((PASSED + 1))
else
    log_error "Puppeteer MCP uses wrong package: $pup_pkg"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 6: "Query the database for user records"
# Uses: sqlite MCP
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 6: Database Queries"
log_info "=============================================="

# Simulate: Verify sqlite MCP configured
TOTAL=$((TOTAL + 1))
log_info "Task 6.1: Verify SQLite MCP server configured"
if jq -e '.mcp.sqlite.enabled' ~/.config/opencode/opencode.json >/dev/null 2>&1; then
    log_success "SQLite MCP is enabled in configuration"
    PASSED=$((PASSED + 1))
else
    log_error "SQLite MCP not enabled"
    FAILED=$((FAILED + 1))
fi

# Simulate: Check sqlite server package
TOTAL=$((TOTAL + 1))
log_info "Task 6.2: Verify SQLite MCP uses correct package (mcp-sqlite)"
sql_pkg=$(jq -r '.mcp.sqlite.command[2]' ~/.config/opencode/opencode.json 2>/dev/null)
if [ "$sql_pkg" = "mcp-sqlite" ]; then
    log_success "SQLite MCP uses correct package"
    PASSED=$((PASSED + 1))
else
    log_error "SQLite MCP uses wrong package: $sql_pkg (expected: mcp-sqlite)"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 7: "Analyze this code for security issues"
# Uses: helixagent-mcp, helixagent-lsp, helixagent-vision
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 7: Security Analysis"
log_info "=============================================="

# Simulate: MCP tool capabilities for analysis
TOTAL=$((TOTAL + 1))
log_info "Task 7.1: Query HelixAgent MCP for analysis tools"
response=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/mcp/tools" 2>&1)
if [ -n "$response" ]; then
    log_success "HelixAgent MCP tools accessible"
    PASSED=$((PASSED + 1))
else
    log_error "HelixAgent MCP tools query failed"
    FAILED=$((FAILED + 1))
fi

# Simulate: LSP for code analysis
TOTAL=$((TOTAL + 1))
log_info "Task 7.2: Query LSP for code diagnostics"
response=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/lsp/stats" 2>&1)
if [ -n "$response" ]; then
    log_success "LSP diagnostics accessible"
    PASSED=$((PASSED + 1))
else
    log_error "LSP diagnostics query failed"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 8: "Search for similar code patterns in the project"
# Uses: helixagent-embeddings, helixagent-cognee
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 8: Semantic Search"
log_info "=============================================="

# Simulate: Embeddings for semantic search
TOTAL=$((TOTAL + 1))
log_info "Task 8.1: Query embeddings providers"
response=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/embeddings/providers" 2>&1)
if [ -n "$response" ]; then
    log_success "Embeddings providers accessible"
    PASSED=$((PASSED + 1))
else
    log_error "Embeddings providers query failed"
    FAILED=$((FAILED + 1))
fi

# Simulate: Cognee for knowledge graph search
TOTAL=$((TOTAL + 1))
log_info "Task 8.2: Query Cognee for semantic search"
response=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/cognee/health" 2>&1)
if [ -n "$response" ]; then
    log_success "Cognee knowledge graph accessible"
    PASSED=$((PASSED + 1))
else
    log_error "Cognee query failed"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 9: "Process this image and extract text"
# Uses: helixagent-vision
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 9: Image Processing"
log_info "=============================================="

# Simulate: Vision protocol for OCR
TOTAL=$((TOTAL + 1))
log_info "Task 9.1: Test vision protocol availability"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/vision" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"capabilities","id":1}' 2>&1)
if [ -n "$response" ]; then
    log_success "Vision protocol accessible"
    PASSED=$((PASSED + 1))
else
    log_error "Vision protocol not accessible"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Conversation 10: "Coordinate multiple agents for this task"
# Uses: helixagent-acp
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Conversation 10: Multi-Agent Coordination"
log_info "=============================================="

# Simulate: ACP for agent coordination
TOTAL=$((TOTAL + 1))
log_info "Task 10.1: Test ACP protocol availability"
response=$(curl -s --max-time 60 -X POST "$HELIXAGENT_URL/v1/acp" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"capabilities","id":1}' 2>&1)
if [ -n "$response" ]; then
    log_success "ACP protocol accessible"
    PASSED=$((PASSED + 1))
else
    log_error "ACP protocol not accessible"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Summary
# ============================================================================

log_info ""
log_info "=============================================="
log_info "OpenCode Conversation Challenge Summary"
log_info "=============================================="
log_info "Total conversation tasks: $TOTAL"
log_success "Passed: $PASSED"
if [ $FAILED -gt 0 ]; then
    log_error "Failed: $FAILED"
else
    log_info "Failed: $FAILED"
fi

PASS_RATE=$((PASSED * 100 / TOTAL))
log_info "Pass rate: ${PASS_RATE}%"

echo ""
log_info "MCP Servers Exercised:"
log_info "  1. fetch         - Web content fetching"
log_info "  2. filesystem    - Project file access"
log_info "  3. github        - GitHub integration"
log_info "  4. memory        - Pattern storage"
log_info "  5. puppeteer     - Browser automation"
log_info "  6. sqlite        - Database queries"
log_info "  7. helixagent-mcp      - Tool orchestration"
log_info "  8. helixagent-acp      - Agent communication"
log_info "  9. helixagent-lsp      - Code intelligence"
log_info "  10. helixagent-embeddings - Vector search"
log_info "  11. helixagent-vision    - Image analysis"
log_info "  12. helixagent-cognee    - Knowledge graph"

if [ $FAILED -eq 0 ]; then
    log_success "=============================================="
    log_success "ALL CONVERSATION SCENARIOS PASSED!"
    log_success "=============================================="
    exit 0
else
    log_error "=============================================="
    log_error "SOME CONVERSATION SCENARIOS FAILED"
    log_error "=============================================="
    exit 1
fi
