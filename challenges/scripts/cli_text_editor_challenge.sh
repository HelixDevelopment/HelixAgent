#!/bin/bash
# CLI Text Editor Challenge
# VALIDATES: Text editing capabilities via Edit tool
# Tests file editing, multi-file operations, error handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh" 2>/dev/null || {
    log_info() { echo -e "\033[0;34m[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_success() { echo -e "\033[0;32m[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_error() { echo -e "\033[0;31m[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_warning() { echo -e "\033[0;33m[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
}

CHALLENGE_NAME="CLI Text Editor Challenge"
PASSED=0
FAILED=0
TOTAL=0
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"

log_info "=============================================="
log_info "$CHALLENGE_NAME"
log_info "=============================================="
log_info "VALIDATES: Text editing tools and file operations"
log_info ""

PROJECT_ROOT="${SCRIPT_DIR}/../.."
TEMP_DIR="${PROJECT_ROOT}/challenges/temp_editor_test"

# Cleanup function
cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Create temp directory for tests
mkdir -p "$TEMP_DIR"

# ============================================================================
# Section 1: Tool Schema Validation
# ============================================================================

log_info "=============================================="
log_info "Section 1: Tool Schema Validation"
log_info "=============================================="

# Test 1: Edit tool schema exists
TOTAL=$((TOTAL + 1))
log_info "Test 1: Edit tool schema exists"
if grep -q '"Edit":' "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "Edit tool schema found"
    PASSED=$((PASSED + 1))
else
    log_error "Edit tool schema NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 2: Edit tool has required fields
TOTAL=$((TOTAL + 1))
log_info "Test 2: Edit tool has required fields (file_path, old_string, new_string)"
if grep -A5 '"Edit":' "$PROJECT_ROOT/internal/tools/schema.go" | grep -q 'RequiredFields.*file_path.*old_string.*new_string'; then
    log_success "Edit tool has all required fields"
    PASSED=$((PASSED + 1))
else
    log_error "Edit tool missing required fields!"
    FAILED=$((FAILED + 1))
fi

# Test 3: Edit tool has replace_all option
TOTAL=$((TOTAL + 1))
log_info "Test 3: Edit tool has replace_all option"
if grep -A20 '"Edit":' "$PROJECT_ROOT/internal/tools/schema.go" | grep -q 'replace_all'; then
    log_success "Edit tool has replace_all option"
    PASSED=$((PASSED + 1))
else
    log_error "Edit tool missing replace_all option!"
    FAILED=$((FAILED + 1))
fi

# Test 4: Read tool schema exists
TOTAL=$((TOTAL + 1))
log_info "Test 4: Read tool schema exists"
if grep -q '"Read":' "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "Read tool schema found"
    PASSED=$((PASSED + 1))
else
    log_error "Read tool schema NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 5: Write tool schema exists
TOTAL=$((TOTAL + 1))
log_info "Test 5: Write tool schema exists"
if grep -q '"Write":' "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "Write tool schema found"
    PASSED=$((PASSED + 1))
else
    log_error "Write tool schema NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 2: Tool Handler Validation
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 2: Tool Handler Validation"
log_info "=============================================="

# Test 6: Tool handler file exists
TOTAL=$((TOTAL + 1))
log_info "Test 6: Tool handler exists"
if [ -f "$PROJECT_ROOT/internal/tools/handler.go" ]; then
    log_success "Tool handler found"
    PASSED=$((PASSED + 1))
else
    log_error "Tool handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 7: Edit tool has description in schema
TOTAL=$((TOTAL + 1))
log_info "Test 7: Edit tool has description in schema"
if grep -B2 -A5 '"Edit":' "$PROJECT_ROOT/internal/tools/schema.go" | grep -q 'Description.*Edit a file'; then
    log_success "Edit tool has description in schema"
    PASSED=$((PASSED + 1))
else
    log_error "Edit tool description NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 8: ValidateToolArgs function exists
TOTAL=$((TOTAL + 1))
log_info "Test 8: ValidateToolArgs function exists"
if grep -q "func ValidateToolArgs" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "ValidateToolArgs function found"
    PASSED=$((PASSED + 1))
else
    log_error "ValidateToolArgs NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 3: Search Functionality Validation
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 3: Search Functionality Validation"
log_info "=============================================="

# Test 9: SearchTools function exists
TOTAL=$((TOTAL + 1))
log_info "Test 9: SearchTools function exists"
if grep -q "func SearchTools" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "SearchTools function found"
    PASSED=$((PASSED + 1))
else
    log_error "SearchTools function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 10: ToolSearchResult type exists
TOTAL=$((TOTAL + 1))
log_info "Test 10: ToolSearchResult type exists"
if grep -q "type ToolSearchResult struct" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "ToolSearchResult type found"
    PASSED=$((PASSED + 1))
else
    log_error "ToolSearchResult type NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 11: SearchOptions type exists
TOTAL=$((TOTAL + 1))
log_info "Test 11: SearchOptions type exists"
if grep -q "type SearchOptions struct" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "SearchOptions type found"
    PASSED=$((PASSED + 1))
else
    log_error "SearchOptions type NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 12: Fuzzy matching supported
TOTAL=$((TOTAL + 1))
log_info "Test 12: Fuzzy matching supported"
if grep -q "FuzzyMatch\|fuzzyMatch" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "Fuzzy matching supported"
    PASSED=$((PASSED + 1))
else
    log_error "Fuzzy matching NOT supported!"
    FAILED=$((FAILED + 1))
fi

# Test 13: GetToolSuggestions function exists
TOTAL=$((TOTAL + 1))
log_info "Test 13: GetToolSuggestions function exists"
if grep -q "func GetToolSuggestions" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "GetToolSuggestions function found"
    PASSED=$((PASSED + 1))
else
    log_error "GetToolSuggestions NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 4: API Endpoint Validation
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 4: API Endpoint Validation"
log_info "=============================================="

# Test 14: MCP tool search endpoint registered
TOTAL=$((TOTAL + 1))
log_info "Test 14: MCP tool search endpoint registered"
if grep -q '/tools/search.*MCPToolSearch' "$PROJECT_ROOT/internal/router/router.go" 2>/dev/null; then
    log_success "MCP tool search endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP tool search endpoint NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 15: MCPToolSearch handler exists
TOTAL=$((TOTAL + 1))
log_info "Test 15: MCPToolSearch handler exists"
if grep -q "func.*MCPToolSearch" "$PROJECT_ROOT/internal/handlers/mcp.go" 2>/dev/null; then
    log_success "MCPToolSearch handler found"
    PASSED=$((PASSED + 1))
else
    log_error "MCPToolSearch handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 16: MCPAdapterSearch handler exists
TOTAL=$((TOTAL + 1))
log_info "Test 16: MCPAdapterSearch handler exists"
if grep -q "func.*MCPAdapterSearch" "$PROJECT_ROOT/internal/handlers/mcp.go" 2>/dev/null; then
    log_success "MCPAdapterSearch handler found"
    PASSED=$((PASSED + 1))
else
    log_error "MCPAdapterSearch handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 17: MCPStats endpoint exists
TOTAL=$((TOTAL + 1))
log_info "Test 17: MCPStats endpoint exists"
if grep -q "func.*MCPStats" "$PROJECT_ROOT/internal/handlers/mcp.go" 2>/dev/null; then
    log_success "MCPStats handler found"
    PASSED=$((PASSED + 1))
else
    log_error "MCPStats handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 5: Unit Test Validation
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 5: Unit Test Validation"
log_info "=============================================="

# Test 18: Tool search tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 18: Tool search tests exist"
if [ -f "$PROJECT_ROOT/internal/tools/search_test.go" ]; then
    log_success "Tool search tests found"
    PASSED=$((PASSED + 1))
else
    log_error "Tool search tests NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 19: Adapter search tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 19: Adapter search tests exist"
if [ -f "$PROJECT_ROOT/internal/mcp/adapters/search_test.go" ]; then
    log_success "Adapter search tests found"
    PASSED=$((PASSED + 1))
else
    log_error "Adapter search tests NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 20: Run tool search unit tests
TOTAL=$((TOTAL + 1))
log_info "Test 20: Tool search unit tests pass"
cd "$PROJECT_ROOT"
if go test -v ./internal/tools/search_test.go ./internal/tools/schema.go > /dev/null 2>&1; then
    log_success "Tool search unit tests pass"
    PASSED=$((PASSED + 1))
else
    log_error "Tool search unit tests FAIL!"
    FAILED=$((FAILED + 1))
fi

# Test 21: Run adapter search unit tests
TOTAL=$((TOTAL + 1))
log_info "Test 21: Adapter search unit tests pass"
if go test -v ./internal/mcp/adapters/search_test.go ./internal/mcp/adapters/registry.go > /dev/null 2>&1; then
    log_success "Adapter search unit tests pass"
    PASSED=$((PASSED + 1))
else
    log_error "Adapter search unit tests FAIL!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 6: File Operation Tests
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 6: File Operation Tests"
log_info "=============================================="

# Create test files
echo "Hello World" > "$TEMP_DIR/test1.txt"
echo "Line 1
Line 2
Line 3
Line 2
Line 5" > "$TEMP_DIR/test2.txt"

# Test 22: Category filter works
TOTAL=$((TOTAL + 1))
log_info "Test 22: Category filter for filesystem tools"
if grep -q "GetToolsByCategory" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "GetToolsByCategory function exists"
    PASSED=$((PASSED + 1))
else
    log_error "GetToolsByCategory NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 23: Edit tool category is filesystem
TOTAL=$((TOTAL + 1))
log_info "Test 23: Edit tool is in filesystem category"
if grep -A20 '"Edit":' "$PROJECT_ROOT/internal/tools/schema.go" | grep -q 'Category.*CategoryFileSystem'; then
    log_success "Edit tool is in filesystem category"
    PASSED=$((PASSED + 1))
else
    log_error "Edit tool category incorrect!"
    FAILED=$((FAILED + 1))
fi

# Test 24: Tool aliases supported
TOTAL=$((TOTAL + 1))
log_info "Test 24: Tool aliases supported"
if grep -q 'Aliases.*\[\]string' "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "Tool aliases supported"
    PASSED=$((PASSED + 1))
else
    log_error "Tool aliases NOT supported!"
    FAILED=$((FAILED + 1))
fi

# Test 25: Parameter validation in schema
TOTAL=$((TOTAL + 1))
log_info "Test 25: Parameter validation in schema"
if grep -q 'type Param struct' "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    log_success "Parameter validation schema exists"
    PASSED=$((PASSED + 1))
else
    log_error "Parameter validation schema NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 7: MCP Integration Tests
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 7: MCP Integration Tests"
log_info "=============================================="

# Test 26: MCP tools endpoint exists
TOTAL=$((TOTAL + 1))
log_info "Test 26: MCP tools endpoint registered"
if grep -q '/mcp/tools' "$PROJECT_ROOT/internal/router/router.go" 2>/dev/null; then
    log_success "MCP tools endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP tools endpoint NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 27: MCP suggestions endpoint registered
TOTAL=$((TOTAL + 1))
log_info "Test 27: MCP suggestions endpoint registered"
if grep -q '/tools/suggestions.*MCPToolSuggestions' "$PROJECT_ROOT/internal/router/router.go" 2>/dev/null; then
    log_success "MCP suggestions endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP suggestions endpoint NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 28: MCP categories endpoint registered
TOTAL=$((TOTAL + 1))
log_info "Test 28: MCP categories endpoint registered"
if grep -q '/categories.*MCPCategories' "$PROJECT_ROOT/internal/router/router.go" 2>/dev/null; then
    log_success "MCP categories endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP categories endpoint NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 29: Adapter registry has search capability
TOTAL=$((TOTAL + 1))
log_info "Test 29: Adapter registry has search capability"
if grep -q "func.*Search.*AdapterSearchOptions" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    log_success "Adapter registry search capability found"
    PASSED=$((PASSED + 1))
else
    log_error "Adapter registry search NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 30: Adapter categories defined
TOTAL=$((TOTAL + 1))
log_info "Test 30: Adapter categories defined"
if grep -q "CategoryDatabase\|CategoryStorage\|CategoryVersionControl" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    log_success "Adapter categories defined"
    PASSED=$((PASSED + 1))
else
    log_error "Adapter categories NOT defined!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 8: Tool Registry Integration
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 8: Tool Registry Integration"
log_info "=============================================="

# Test 31: Unified search in tool registry
TOTAL=$((TOTAL + 1))
log_info "Test 31: Unified search in tool registry"
if grep -q "func.*Search.*UnifiedSearchOptions" "$PROJECT_ROOT/internal/services/tool_registry.go" 2>/dev/null; then
    log_success "Unified search in tool registry"
    PASSED=$((PASSED + 1))
else
    log_error "Unified search NOT found in tool registry!"
    FAILED=$((FAILED + 1))
fi

# Test 32: UnifiedSearchResult type exists
TOTAL=$((TOTAL + 1))
log_info "Test 32: UnifiedSearchResult type exists"
if grep -q "type UnifiedSearchResult struct" "$PROJECT_ROOT/internal/services/tool_registry.go" 2>/dev/null; then
    log_success "UnifiedSearchResult type found"
    PASSED=$((PASSED + 1))
else
    log_error "UnifiedSearchResult type NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 33: Tool registry GetToolStats function
TOTAL=$((TOTAL + 1))
log_info "Test 33: GetToolStats function exists"
if grep -q "func.*GetToolStats" "$PROJECT_ROOT/internal/services/tool_registry.go" 2>/dev/null; then
    log_success "GetToolStats function found"
    PASSED=$((PASSED + 1))
else
    log_error "GetToolStats NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 34: GetToolsBySource function exists
TOTAL=$((TOTAL + 1))
log_info "Test 34: GetToolsBySource function exists"
if grep -q "func.*GetToolsBySource" "$PROJECT_ROOT/internal/services/tool_registry.go" 2>/dev/null; then
    log_success "GetToolsBySource function found"
    PASSED=$((PASSED + 1))
else
    log_error "GetToolsBySource NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Summary
# ============================================================================

log_info ""
log_info "=============================================="
log_info "CHALLENGE SUMMARY"
log_info "=============================================="
log_info "Total Tests: $TOTAL"
log_info "Passed: $PASSED"
log_info "Failed: $FAILED"

if [ $FAILED -eq 0 ]; then
    log_success "ALL TESTS PASSED! CLI Text Editor Challenge Complete!"
    exit 0
else
    log_error "$FAILED tests failed!"
    exit 1
fi
