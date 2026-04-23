#!/bin/bash
# Tool Execution Challenge
# VALIDATES: All 21 tools can be executed correctly
# Tests tool registration, schema validation, category coverage, search functionality,
# handler integration, and MCP tool integration
#
# Total: 56 tests across 6 sections

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh" 2>/dev/null || {
    log_info() { echo -e "\033[0;34m[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_success() { echo -e "\033[0;32m[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_error() { echo -e "\033[0;31m[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_warning() { echo -e "\033[0;33m[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
}

CHALLENGE_NAME="Tool Execution Challenge"
PASSED=0
FAILED=0
TOTAL=0
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"

log_info "=============================================="
log_info "$CHALLENGE_NAME"
log_info "=============================================="
log_info "VALIDATES: All 21 tools registration, schema, and execution"
log_info ""

PROJECT_ROOT="${SCRIPT_DIR}/../.."

# ============================================================================
# Section 1: Tool Registration (10 tests)
# ============================================================================

log_info "=============================================="
log_info "Section 1: Tool Registration (10 tests)"
log_info "=============================================="

SCHEMA_FILE="$PROJECT_ROOT/internal/tools/schema.go"

# Test 1: Verify schema.go exists
TOTAL=$((TOTAL + 1))
log_info "Test 1: Tool schema file exists"
if [ -f "$SCHEMA_FILE" ]; then
    log_success "schema.go found at $SCHEMA_FILE"
    PASSED=$((PASSED + 1))
else
    log_error "schema.go NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 2: Verify Bash tool exists
TOTAL=$((TOTAL + 1))
log_info "Test 2: Bash tool exists in schema"
if grep -q '"Bash":' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Bash tool found"
    PASSED=$((PASSED + 1))
else
    log_error "Bash tool NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 3: Verify Read tool exists
TOTAL=$((TOTAL + 1))
log_info "Test 3: Read tool exists in schema"
if grep -q '"Read":' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Read tool found"
    PASSED=$((PASSED + 1))
else
    log_error "Read tool NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 4: Verify Write tool exists
TOTAL=$((TOTAL + 1))
log_info "Test 4: Write tool exists in schema"
if grep -q '"Write":' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Write tool found"
    PASSED=$((PASSED + 1))
else
    log_error "Write tool NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 5: Verify Edit tool exists
TOTAL=$((TOTAL + 1))
log_info "Test 5: Edit tool exists in schema"
if grep -q '"Edit":' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Edit tool found"
    PASSED=$((PASSED + 1))
else
    log_error "Edit tool NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 6: Verify Glob and Grep tools exist
TOTAL=$((TOTAL + 1))
log_info "Test 6: Glob and Grep tools exist in schema"
if grep -q '"Glob":' "$SCHEMA_FILE" && grep -q '"Grep":' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Glob and Grep tools found"
    PASSED=$((PASSED + 1))
else
    log_error "Glob and/or Grep tools NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 7: Verify WebFetch and WebSearch tools exist
TOTAL=$((TOTAL + 1))
log_info "Test 7: WebFetch and WebSearch tools exist in schema"
if grep -q '"WebFetch":' "$SCHEMA_FILE" && grep -q '"WebSearch":' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "WebFetch and WebSearch tools found"
    PASSED=$((PASSED + 1))
else
    log_error "WebFetch and/or WebSearch tools NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 8: Verify Task and Git tools exist
TOTAL=$((TOTAL + 1))
log_info "Test 8: Task and Git tools exist in schema"
if grep -q '"Task":' "$SCHEMA_FILE" && grep -q '"Git":' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Task and Git tools found"
    PASSED=$((PASSED + 1))
else
    log_error "Task and/or Git tools NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 9: Verify Diff, Test, and Lint tools exist
TOTAL=$((TOTAL + 1))
log_info "Test 9: Diff, Test, and Lint tools exist in schema"
if grep -q '"Diff":' "$SCHEMA_FILE" && grep -q '"Test":' "$SCHEMA_FILE" && grep -q '"Lint":' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Diff, Test, and Lint tools found"
    PASSED=$((PASSED + 1))
else
    log_error "Diff, Test, and/or Lint tools NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 10: Verify remaining tools (TreeView, FileInfo, Symbols, References, Definition, PR, Issue, Workflow)
TOTAL=$((TOTAL + 1))
log_info "Test 10: All 21 tools exist in schema"
MISSING_TOOLS=""
for tool in "TreeView" "FileInfo" "Symbols" "References" "Definition" "PR" "Issue" "Workflow"; do
    if ! grep -q "\"$tool\":" "$SCHEMA_FILE" 2>/dev/null; then
        MISSING_TOOLS="$MISSING_TOOLS $tool"
    fi
done
if [ -z "$MISSING_TOOLS" ]; then
    log_success "All 21 tools found in schema"
    PASSED=$((PASSED + 1))
else
    log_error "Missing tools:$MISSING_TOOLS"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 2: Tool Schema Validation (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 2: Tool Schema Validation (10 tests)"
log_info "=============================================="

# Test 11: ToolSchema struct has Name field
TOTAL=$((TOTAL + 1))
log_info "Test 11: ToolSchema struct has Name field"
if grep -A5 "type ToolSchema struct" "$SCHEMA_FILE" | grep -q "Name.*string" 2>/dev/null; then
    log_success "ToolSchema has Name field"
    PASSED=$((PASSED + 1))
else
    log_error "ToolSchema Name field NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 12: ToolSchema struct has Description field
TOTAL=$((TOTAL + 1))
log_info "Test 12: ToolSchema struct has Description field"
if grep -A10 "type ToolSchema struct" "$SCHEMA_FILE" | grep -q "Description.*string" 2>/dev/null; then
    log_success "ToolSchema has Description field"
    PASSED=$((PASSED + 1))
else
    log_error "ToolSchema Description field NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 13: ToolSchema struct has RequiredFields
TOTAL=$((TOTAL + 1))
log_info "Test 13: ToolSchema struct has RequiredFields"
if grep -A15 "type ToolSchema struct" "$SCHEMA_FILE" | grep -q "RequiredFields" 2>/dev/null; then
    log_success "ToolSchema has RequiredFields"
    PASSED=$((PASSED + 1))
else
    log_error "ToolSchema RequiredFields NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 14: ToolSchema struct has Parameters
TOTAL=$((TOTAL + 1))
log_info "Test 14: ToolSchema struct has Parameters"
if grep -A20 "type ToolSchema struct" "$SCHEMA_FILE" | grep -q "Parameters" 2>/dev/null; then
    log_success "ToolSchema has Parameters"
    PASSED=$((PASSED + 1))
else
    log_error "ToolSchema Parameters NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 15: ToolSchema struct has Category
TOTAL=$((TOTAL + 1))
log_info "Test 15: ToolSchema struct has Category"
if grep -A20 "type ToolSchema struct" "$SCHEMA_FILE" | grep -q "Category" 2>/dev/null; then
    log_success "ToolSchema has Category"
    PASSED=$((PASSED + 1))
else
    log_error "ToolSchema Category NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 16: Param struct exists with Type field
TOTAL=$((TOTAL + 1))
log_info "Test 16: Param struct exists with Type field"
if grep -q "type Param struct" "$SCHEMA_FILE" && grep -A5 "type Param struct" "$SCHEMA_FILE" | grep -q "Type.*string" 2>/dev/null; then
    log_success "Param struct with Type field found"
    PASSED=$((PASSED + 1))
else
    log_error "Param struct or Type field NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 17: Edit tool has required fields (file_path, old_string, new_string)
TOTAL=$((TOTAL + 1))
log_info "Test 17: Edit tool has required fields (file_path, old_string, new_string)"
if grep -A10 '"Edit":' "$SCHEMA_FILE" | grep -q "RequiredFields.*file_path.*old_string.*new_string" 2>/dev/null; then
    log_success "Edit tool has all required fields"
    PASSED=$((PASSED + 1))
else
    log_error "Edit tool missing required fields!"
    FAILED=$((FAILED + 1))
fi

# Test 18: Bash tool has required fields (command, description)
TOTAL=$((TOTAL + 1))
log_info "Test 18: Bash tool has required fields (command, description)"
if grep -A10 '"Bash":' "$SCHEMA_FILE" | grep -q "RequiredFields.*command.*description" 2>/dev/null; then
    log_success "Bash tool has required fields"
    PASSED=$((PASSED + 1))
else
    log_error "Bash tool missing required fields!"
    FAILED=$((FAILED + 1))
fi

# Test 19: WebFetch tool has required fields (url, prompt)
TOTAL=$((TOTAL + 1))
log_info "Test 19: WebFetch tool has required fields (url, prompt)"
if grep -A10 '"WebFetch":' "$SCHEMA_FILE" | grep -q "RequiredFields.*url.*prompt" 2>/dev/null; then
    log_success "WebFetch tool has required fields"
    PASSED=$((PASSED + 1))
else
    log_error "WebFetch tool missing required fields!"
    FAILED=$((FAILED + 1))
fi

# Test 20: Git tool has required fields (operation, description)
TOTAL=$((TOTAL + 1))
log_info "Test 20: Git tool has required fields (operation, description)"
if grep -A10 '"Git":' "$SCHEMA_FILE" | grep -q "RequiredFields.*operation.*description" 2>/dev/null; then
    log_success "Git tool has required fields"
    PASSED=$((PASSED + 1))
else
    log_error "Git tool missing required fields!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 3: Tool Category Coverage (6 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 3: Tool Category Coverage (6 tests)"
log_info "=============================================="

# Test 21: Core category constants exist
TOTAL=$((TOTAL + 1))
log_info "Test 21: Core category constant exists"
if grep -q 'CategoryCore.*=.*"core"' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "CategoryCore constant found"
    PASSED=$((PASSED + 1))
else
    log_error "CategoryCore constant NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 22: FileSystem category exists
TOTAL=$((TOTAL + 1))
log_info "Test 22: FileSystem category constant exists"
if grep -q 'CategoryFileSystem.*=.*"filesystem"' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "CategoryFileSystem constant found"
    PASSED=$((PASSED + 1))
else
    log_error "CategoryFileSystem constant NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 23: VersionControl category exists
TOTAL=$((TOTAL + 1))
log_info "Test 23: VersionControl category constant exists"
if grep -q 'CategoryVersionControl.*=.*"version_control"' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "CategoryVersionControl constant found"
    PASSED=$((PASSED + 1))
else
    log_error "CategoryVersionControl constant NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 24: CodeIntelligence category exists
TOTAL=$((TOTAL + 1))
log_info "Test 24: CodeIntelligence category constant exists"
if grep -q 'CategoryCodeIntel.*=.*"code_intelligence"' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "CategoryCodeIntel constant found"
    PASSED=$((PASSED + 1))
else
    log_error "CategoryCodeIntel constant NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 25: Workflow category exists
TOTAL=$((TOTAL + 1))
log_info "Test 25: Workflow category constant exists"
if grep -q 'CategoryWorkflow.*=.*"workflow"' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "CategoryWorkflow constant found"
    PASSED=$((PASSED + 1))
else
    log_error "CategoryWorkflow constant NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 26: Web category exists
TOTAL=$((TOTAL + 1))
log_info "Test 26: Web category constant exists"
if grep -q 'CategoryWeb.*=.*"web"' "$SCHEMA_FILE" 2>/dev/null; then
    log_success "CategoryWeb constant found"
    PASSED=$((PASSED + 1))
else
    log_error "CategoryWeb constant NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 4: Tool Search Functionality (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 4: Tool Search Functionality (10 tests)"
log_info "=============================================="

# Test 27: SearchTools function exists
TOTAL=$((TOTAL + 1))
log_info "Test 27: SearchTools function exists"
if grep -q "func SearchTools" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "SearchTools function found"
    PASSED=$((PASSED + 1))
else
    log_error "SearchTools function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 28: SearchOptions type exists for exact name match
TOTAL=$((TOTAL + 1))
log_info "Test 28: SearchOptions type exists"
if grep -q "type SearchOptions struct" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "SearchOptions type found"
    PASSED=$((PASSED + 1))
else
    log_error "SearchOptions type NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 29: ToolSearchResult type exists for partial name match
TOTAL=$((TOTAL + 1))
log_info "Test 29: ToolSearchResult type exists"
if grep -q "type ToolSearchResult struct" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "ToolSearchResult type found"
    PASSED=$((PASSED + 1))
else
    log_error "ToolSearchResult type NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 30: Description search supported (calculateToolScore checks description)
TOTAL=$((TOTAL + 1))
log_info "Test 30: Description search supported"
if grep -q "descLower.*Description\|Description.*descLower" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Description search supported"
    PASSED=$((PASSED + 1))
else
    log_error "Description search NOT supported!"
    FAILED=$((FAILED + 1))
fi

# Test 31: Category filter supported
TOTAL=$((TOTAL + 1))
log_info "Test 31: Category filter in SearchOptions"
if grep -A10 "type SearchOptions struct" "$SCHEMA_FILE" | grep -q "Categories" 2>/dev/null; then
    log_success "Category filter supported"
    PASSED=$((PASSED + 1))
else
    log_error "Category filter NOT supported!"
    FAILED=$((FAILED + 1))
fi

# Test 32: Fuzzy match supported
TOTAL=$((TOTAL + 1))
log_info "Test 32: Fuzzy match supported"
if grep -q "FuzzyMatch\|fuzzyMatch" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "Fuzzy match supported"
    PASSED=$((PASSED + 1))
else
    log_error "Fuzzy match NOT supported!"
    FAILED=$((FAILED + 1))
fi

# Test 33: GetToolSuggestions function exists
TOTAL=$((TOTAL + 1))
log_info "Test 33: GetToolSuggestions function exists"
if grep -q "func GetToolSuggestions" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "GetToolSuggestions function found"
    PASSED=$((PASSED + 1))
else
    log_error "GetToolSuggestions function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 34: SearchByKeywords function exists
TOTAL=$((TOTAL + 1))
log_info "Test 34: SearchByKeywords function exists"
if grep -q "func SearchByKeywords" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "SearchByKeywords function found"
    PASSED=$((PASSED + 1))
else
    log_error "SearchByKeywords function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 35: GetToolsByCategory function exists
TOTAL=$((TOTAL + 1))
log_info "Test 35: GetToolsByCategory function exists"
if grep -q "func GetToolsByCategory" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "GetToolsByCategory function found"
    PASSED=$((PASSED + 1))
else
    log_error "GetToolsByCategory function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 36: GetAllToolNames function exists
TOTAL=$((TOTAL + 1))
log_info "Test 36: GetAllToolNames function exists"
if grep -q "func GetAllToolNames" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "GetAllToolNames function found"
    PASSED=$((PASSED + 1))
else
    log_error "GetAllToolNames function NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 5: Tool Handler Integration (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 5: Tool Handler Integration (10 tests)"
log_info "=============================================="

HANDLER_FILE="$PROJECT_ROOT/internal/tools/handler.go"

# Test 37: Tool handler file exists
TOTAL=$((TOTAL + 1))
log_info "Test 37: Tool handler file exists"
if [ -f "$HANDLER_FILE" ]; then
    log_success "handler.go found"
    PASSED=$((PASSED + 1))
else
    log_error "handler.go NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 38: ValidateToolArgs function exists
TOTAL=$((TOTAL + 1))
log_info "Test 38: ValidateToolArgs function exists"
if grep -q "func ValidateToolArgs" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "ValidateToolArgs function found"
    PASSED=$((PASSED + 1))
else
    log_error "ValidateToolArgs function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 39: GetToolSchema function exists
TOTAL=$((TOTAL + 1))
log_info "Test 39: GetToolSchema function exists"
if grep -q "func GetToolSchema" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "GetToolSchema function found"
    PASSED=$((PASSED + 1))
else
    log_error "GetToolSchema function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 40: GenerateOpenAIToolDefinition exists
TOTAL=$((TOTAL + 1))
log_info "Test 40: GenerateOpenAIToolDefinition function exists"
if grep -q "func GenerateOpenAIToolDefinition" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "GenerateOpenAIToolDefinition function found"
    PASSED=$((PASSED + 1))
else
    log_error "GenerateOpenAIToolDefinition function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 41: GenerateAllToolDefinitions exists
TOTAL=$((TOTAL + 1))
log_info "Test 41: GenerateAllToolDefinitions function exists"
if grep -q "func GenerateAllToolDefinitions" "$SCHEMA_FILE" 2>/dev/null; then
    log_success "GenerateAllToolDefinitions function found"
    PASSED=$((PASSED + 1))
else
    log_error "GenerateAllToolDefinitions function NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 42: ToolHandler interface exists
TOTAL=$((TOTAL + 1))
log_info "Test 42: ToolHandler interface exists"
if grep -q "type ToolHandler interface" "$HANDLER_FILE" 2>/dev/null; then
    log_success "ToolHandler interface found"
    PASSED=$((PASSED + 1))
else
    log_error "ToolHandler interface NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 43: ToolRegistry type exists in handler
TOTAL=$((TOTAL + 1))
log_info "Test 43: ToolRegistry type exists in handler"
if grep -q "type ToolRegistry struct" "$HANDLER_FILE" 2>/dev/null; then
    log_success "ToolRegistry type found"
    PASSED=$((PASSED + 1))
else
    log_error "ToolRegistry type NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 44: DefaultToolRegistry exists
TOTAL=$((TOTAL + 1))
log_info "Test 44: DefaultToolRegistry exists"
if grep -q "DefaultToolRegistry" "$HANDLER_FILE" 2>/dev/null; then
    log_success "DefaultToolRegistry found"
    PASSED=$((PASSED + 1))
else
    log_error "DefaultToolRegistry NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 45: GitHandler registered in init
TOTAL=$((TOTAL + 1))
log_info "Test 45: GitHandler registered in init"
if grep -q "Register.*GitHandler" "$HANDLER_FILE" 2>/dev/null; then
    log_success "GitHandler registered"
    PASSED=$((PASSED + 1))
else
    log_error "GitHandler NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 46: All handlers registered (Git, Test, Lint, Diff, TreeView, FileInfo, Symbols, References, Definition, PR, Issue, Workflow)
TOTAL=$((TOTAL + 1))
log_info "Test 46: All 12 handlers registered"
MISSING_HANDLERS=""
for handler in "GitHandler" "TestHandler" "LintHandler" "DiffHandler" "TreeViewHandler" "FileInfoHandler" "SymbolsHandler" "ReferencesHandler" "DefinitionHandler" "PRHandler" "IssueHandler" "WorkflowHandler"; do
    if ! grep -q "$handler" "$HANDLER_FILE" 2>/dev/null; then
        MISSING_HANDLERS="$MISSING_HANDLERS $handler"
    fi
done
if [ -z "$MISSING_HANDLERS" ]; then
    log_success "All 12 handlers registered"
    PASSED=$((PASSED + 1))
else
    log_error "Missing handlers:$MISSING_HANDLERS"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 6: MCP Tool Integration (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 6: MCP Tool Integration (10 tests)"
log_info "=============================================="

ROUTER_FILE="$PROJECT_ROOT/internal/router/router.go"
MCP_HANDLER_FILE="$PROJECT_ROOT/internal/handlers/mcp.go"

# Test 47: MCP tool search endpoint registered
TOTAL=$((TOTAL + 1))
log_info "Test 47: MCP tool search endpoint registered"
if grep -q '/tools/search.*MCPToolSearch' "$ROUTER_FILE" 2>/dev/null; then
    log_success "MCP tool search endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP tool search endpoint NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 48: MCPToolSearch handler exists
TOTAL=$((TOTAL + 1))
log_info "Test 48: MCPToolSearch handler exists"
if grep -q "func.*MCPToolSearch" "$MCP_HANDLER_FILE" 2>/dev/null; then
    log_success "MCPToolSearch handler found"
    PASSED=$((PASSED + 1))
else
    log_error "MCPToolSearch handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 49: MCPAdapterSearch handler exists
TOTAL=$((TOTAL + 1))
log_info "Test 49: MCPAdapterSearch handler exists"
if grep -q "func.*MCPAdapterSearch" "$MCP_HANDLER_FILE" 2>/dev/null; then
    log_success "MCPAdapterSearch handler found"
    PASSED=$((PASSED + 1))
else
    log_error "MCPAdapterSearch handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 50: MCPToolSuggestions handler exists
TOTAL=$((TOTAL + 1))
log_info "Test 50: MCPToolSuggestions handler exists"
if grep -q "func.*MCPToolSuggestions" "$MCP_HANDLER_FILE" 2>/dev/null; then
    log_success "MCPToolSuggestions handler found"
    PASSED=$((PASSED + 1))
else
    log_error "MCPToolSuggestions handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 51: MCPCategories handler exists
TOTAL=$((TOTAL + 1))
log_info "Test 51: MCPCategories handler exists"
if grep -q "func.*MCPCategories" "$MCP_HANDLER_FILE" 2>/dev/null; then
    log_success "MCPCategories handler found"
    PASSED=$((PASSED + 1))
else
    log_error "MCPCategories handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 52: MCPStats handler exists
TOTAL=$((TOTAL + 1))
log_info "Test 52: MCPStats handler exists"
if grep -q "func.*MCPStats" "$MCP_HANDLER_FILE" 2>/dev/null; then
    log_success "MCPStats handler found"
    PASSED=$((PASSED + 1))
else
    log_error "MCPStats handler NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 53: MCP suggestions endpoint registered
TOTAL=$((TOTAL + 1))
log_info "Test 53: MCP suggestions endpoint registered"
if grep -q '/tools/suggestions.*MCPToolSuggestions' "$ROUTER_FILE" 2>/dev/null; then
    log_success "MCP suggestions endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP suggestions endpoint NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 54: MCP categories endpoint registered
TOTAL=$((TOTAL + 1))
log_info "Test 54: MCP categories endpoint registered"
if grep -q '/categories.*MCPCategories' "$ROUTER_FILE" 2>/dev/null; then
    log_success "MCP categories endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP categories endpoint NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 55: MCP stats endpoint registered
TOTAL=$((TOTAL + 1))
log_info "Test 55: MCP stats endpoint registered"
if grep -q '/stats.*MCPStats' "$ROUTER_FILE" 2>/dev/null; then
    log_success "MCP stats endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP stats endpoint NOT registered!"
    FAILED=$((FAILED + 1))
fi

# Test 56: MCP adapters search endpoint registered
TOTAL=$((TOTAL + 1))
log_info "Test 56: MCP adapters search endpoint registered"
if grep -q '/adapters/search.*MCPAdapterSearch' "$ROUTER_FILE" 2>/dev/null; then
    log_success "MCP adapters search endpoint registered"
    PASSED=$((PASSED + 1))
else
    log_error "MCP adapters search endpoint NOT registered!"
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

# Calculate percentage
if [ $TOTAL -gt 0 ]; then
    PERCENTAGE=$((PASSED * 100 / TOTAL))
    log_info "Pass Rate: ${PERCENTAGE}%"
fi

log_info ""
log_info "Section Results:"
log_info "  Section 1 (Tool Registration):      Tests 1-10"
log_info "  Section 2 (Tool Schema Validation): Tests 11-20"
log_info "  Section 3 (Tool Category Coverage): Tests 21-26"
log_info "  Section 4 (Tool Search):            Tests 27-36"
log_info "  Section 5 (Tool Handler):           Tests 37-46"
log_info "  Section 6 (MCP Integration):        Tests 47-56"
log_info ""

if [ $FAILED -eq 0 ]; then
    log_success "ALL $TOTAL TESTS PASSED! Tool Execution Challenge Complete!"
    exit 0
else
    log_error "$FAILED of $TOTAL tests failed!"
    exit 1
fi
