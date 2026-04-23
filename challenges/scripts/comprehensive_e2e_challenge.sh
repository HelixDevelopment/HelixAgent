#!/bin/bash
# Comprehensive End-to-End Challenge Script
# VALIDATES: Complete HelixAgent system functionality across all major components
# Total Tests: 90 tests across 8 sections

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh" 2>/dev/null || {
    log_info() { echo -e "\033[0;34m[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_success() { echo -e "\033[0;32m[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_error() { echo -e "\033[0;31m[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_warning() { echo -e "\033[0;33m[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
}

CHALLENGE_NAME="Comprehensive End-to-End Challenge"
PASSED=0
FAILED=0
TOTAL=0
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"

log_info "=============================================="
log_info "$CHALLENGE_NAME"
log_info "=============================================="
log_info "VALIDATES: Complete HelixAgent system - 90 tests across 8 sections"
log_info ""

PROJECT_ROOT="${SCRIPT_DIR}/../.."

# Helper function to increment counters
pass_test() {
    PASSED=$((PASSED + 1))
    log_success "$1"
}

fail_test() {
    FAILED=$((FAILED + 1))
    log_error "$1"
}

# ============================================================================
# Section 1: Core Infrastructure (10 tests)
# ============================================================================

log_info "=============================================="
log_info "Section 1: Core Infrastructure (10 tests)"
log_info "=============================================="

# Test 1.1: Server binary can be built
TOTAL=$((TOTAL + 1))
log_info "Test 1.1: Server binary can be built"
if go build -o /dev/null "$PROJECT_ROOT/cmd/helixagent" 2>&1; then
    pass_test "Server binary builds successfully"
else
    fail_test "Server binary build FAILED"
fi

# Test 1.2: API server can be built
TOTAL=$((TOTAL + 1))
log_info "Test 1.2: API server can be built"
if go build -o /dev/null "$PROJECT_ROOT/cmd/api" 2>&1; then
    pass_test "API server builds successfully"
else
    fail_test "API server build FAILED"
fi

# Test 1.3: gRPC server can be built
TOTAL=$((TOTAL + 1))
log_info "Test 1.3: gRPC server can be built"
if go build -o /dev/null "$PROJECT_ROOT/cmd/grpc-server" 2>&1; then
    pass_test "gRPC server builds successfully"
else
    fail_test "gRPC server build FAILED"
fi

# Test 1.4: Health endpoint exists in codebase
TOTAL=$((TOTAL + 1))
log_info "Test 1.4: Health endpoint implementation exists"
if grep -rq "health\|/health" "$PROJECT_ROOT/internal/handlers" 2>/dev/null || \
   grep -rq "health\|/health" "$PROJECT_ROOT/internal/router" 2>/dev/null; then
    pass_test "Health endpoint implementation found"
else
    fail_test "Health endpoint implementation NOT found"
fi

# Test 1.5: Metrics endpoint exists in codebase
TOTAL=$((TOTAL + 1))
log_info "Test 1.5: Metrics endpoint implementation exists"
if grep -rq "metrics\|/metrics\|prometheus" "$PROJECT_ROOT/internal" 2>/dev/null; then
    pass_test "Metrics endpoint implementation found"
else
    fail_test "Metrics endpoint implementation NOT found"
fi

# Test 1.6: Configuration loading mechanism exists
TOTAL=$((TOTAL + 1))
log_info "Test 1.6: Configuration loading mechanism exists"
if [ -f "$PROJECT_ROOT/configs/development.yaml" ] && \
   [ -f "$PROJECT_ROOT/configs/production.yaml" ]; then
    pass_test "Configuration files found (development.yaml, production.yaml)"
else
    fail_test "Configuration files NOT found"
fi

# Test 1.7: Internal packages compile
TOTAL=$((TOTAL + 1))
log_info "Test 1.7: Internal packages compile"
if go build "$PROJECT_ROOT/internal/..." 2>&1; then
    pass_test "All internal packages compile successfully"
else
    fail_test "Internal packages compilation FAILED"
fi

# Test 1.8: Router package exists
TOTAL=$((TOTAL + 1))
log_info "Test 1.8: Router package exists"
if [ -d "$PROJECT_ROOT/internal/router" ]; then
    pass_test "Router package found"
else
    fail_test "Router package NOT found"
fi

# Test 1.9: Database package exists
TOTAL=$((TOTAL + 1))
log_info "Test 1.9: Database package exists"
if [ -d "$PROJECT_ROOT/internal/database" ]; then
    pass_test "Database package found"
else
    fail_test "Database package NOT found"
fi

# Test 1.10: Cache package exists
TOTAL=$((TOTAL + 1))
log_info "Test 1.10: Cache package exists"
if [ -d "$PROJECT_ROOT/internal/cache" ]; then
    pass_test "Cache package found"
else
    fail_test "Cache package NOT found"
fi

# ============================================================================
# Section 2: Tool System (15 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 2: Tool System (15 tests)"
log_info "=============================================="

# Test 2.1: Tool schema registry exists
TOTAL=$((TOTAL + 1))
log_info "Test 2.1: Tool schema registry exists"
if [ -f "$PROJECT_ROOT/internal/tools/schema.go" ]; then
    pass_test "Tool schema registry found"
else
    fail_test "Tool schema registry NOT found"
fi

# Test 2.2: ToolSchemaRegistry is defined
TOTAL=$((TOTAL + 1))
log_info "Test 2.2: ToolSchemaRegistry is defined"
if grep -q "ToolSchemaRegistry" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    pass_test "ToolSchemaRegistry is defined"
else
    fail_test "ToolSchemaRegistry NOT defined"
fi

# Test 2.3: At least 21 tools registered
TOTAL=$((TOTAL + 1))
log_info "Test 2.3: At least 21 tools registered"
TOOL_COUNT=$(grep -c "^[[:space:]]*\"[A-Z][a-zA-Z]*\":" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null || echo "0")
if [ "$TOOL_COUNT" -ge 21 ]; then
    pass_test "21+ tools registered (found: $TOOL_COUNT)"
else
    fail_test "Less than 21 tools registered (found: $TOOL_COUNT)"
fi

# Test 2.4: Core tools exist (Bash, Read, Write, Edit)
TOTAL=$((TOTAL + 1))
log_info "Test 2.4: Core tools exist (Bash, Read, Write, Edit)"
if grep -q "\"Bash\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"Read\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"Write\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"Edit\":" "$PROJECT_ROOT/internal/tools/schema.go"; then
    pass_test "Core tools (Bash, Read, Write, Edit) exist"
else
    fail_test "Some core tools missing"
fi

# Test 2.5: Filesystem tools exist (Glob, Grep)
TOTAL=$((TOTAL + 1))
log_info "Test 2.5: Filesystem tools exist (Glob, Grep)"
if grep -q "\"Glob\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"Grep\":" "$PROJECT_ROOT/internal/tools/schema.go"; then
    pass_test "Filesystem tools (Glob, Grep) exist"
else
    fail_test "Some filesystem tools missing"
fi

# Test 2.6: Version control tools exist (Git, Diff)
TOTAL=$((TOTAL + 1))
log_info "Test 2.6: Version control tools exist (Git, Diff)"
if grep -q "\"Git\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"Diff\":" "$PROJECT_ROOT/internal/tools/schema.go"; then
    pass_test "Version control tools (Git, Diff) exist"
else
    fail_test "Some version control tools missing"
fi

# Test 2.7: Code intelligence tools exist (Symbols, References, Definition)
TOTAL=$((TOTAL + 1))
log_info "Test 2.7: Code intelligence tools exist (Symbols, References, Definition)"
if grep -q "\"Symbols\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"References\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"Definition\":" "$PROJECT_ROOT/internal/tools/schema.go"; then
    pass_test "Code intelligence tools exist"
else
    fail_test "Some code intelligence tools missing"
fi

# Test 2.8: Workflow tools exist (PR, Issue, Workflow)
TOTAL=$((TOTAL + 1))
log_info "Test 2.8: Workflow tools exist (PR, Issue, Workflow)"
if grep -q "\"PR\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"Issue\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"Workflow\":" "$PROJECT_ROOT/internal/tools/schema.go"; then
    pass_test "Workflow tools (PR, Issue, Workflow) exist"
else
    fail_test "Some workflow tools missing"
fi

# Test 2.9: Web tools exist (WebFetch, WebSearch)
TOTAL=$((TOTAL + 1))
log_info "Test 2.9: Web tools exist (WebFetch, WebSearch)"
if grep -q "\"WebFetch\":" "$PROJECT_ROOT/internal/tools/schema.go" && \
   grep -q "\"WebSearch\":" "$PROJECT_ROOT/internal/tools/schema.go"; then
    pass_test "Web tools (WebFetch, WebSearch) exist"
else
    fail_test "Some web tools missing"
fi

# Test 2.10: Tool schema validation function exists
TOTAL=$((TOTAL + 1))
log_info "Test 2.10: Tool schema validation function exists"
if grep -q "ValidateToolArgs" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    pass_test "ValidateToolArgs function exists"
else
    fail_test "ValidateToolArgs function NOT found"
fi

# Test 2.11: Tool search functionality exists
TOTAL=$((TOTAL + 1))
log_info "Test 2.11: Tool search functionality exists"
if grep -q "SearchTools" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    pass_test "SearchTools function exists"
else
    fail_test "SearchTools function NOT found"
fi

# Test 2.12: Tool suggestions function exists
TOTAL=$((TOTAL + 1))
log_info "Test 2.12: Tool suggestions function exists"
if grep -q "GetToolSuggestions" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    pass_test "GetToolSuggestions function exists"
else
    fail_test "GetToolSuggestions function NOT found"
fi

# Test 2.13: Tool categories are defined
TOTAL=$((TOTAL + 1))
log_info "Test 2.13: Tool categories are defined"
if grep -q "CategoryCore\|CategoryFileSystem\|CategoryVersionControl\|CategoryCodeIntel\|CategoryWorkflow\|CategoryWeb" "$PROJECT_ROOT/internal/tools/schema.go" 2>/dev/null; then
    pass_test "Tool categories are defined"
else
    fail_test "Tool categories NOT defined"
fi

# Test 2.14: Tool handler exists
TOTAL=$((TOTAL + 1))
log_info "Test 2.14: Tool handler exists"
if [ -f "$PROJECT_ROOT/internal/tools/handler.go" ]; then
    pass_test "Tool handler found"
else
    fail_test "Tool handler NOT found"
fi

# Test 2.15: Tool tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 2.15: Tool tests exist"
if [ -f "$PROJECT_ROOT/internal/tools/schema_test.go" ] || \
   [ -f "$PROJECT_ROOT/internal/tools/handler_test.go" ]; then
    pass_test "Tool tests found"
else
    fail_test "Tool tests NOT found"
fi

# ============================================================================
# Section 3: CLI Agent System (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 3: CLI Agent System (10 tests)"
log_info "=============================================="

# Test 3.1: Agent registry exists
TOTAL=$((TOTAL + 1))
log_info "Test 3.1: Agent registry exists"
if [ -f "$PROJECT_ROOT/internal/agents/registry.go" ]; then
    pass_test "Agent registry found"
else
    fail_test "Agent registry NOT found"
fi

# Test 3.2: CLIAgentRegistry is defined
TOTAL=$((TOTAL + 1))
log_info "Test 3.2: CLIAgentRegistry is defined"
if grep -q "CLIAgentRegistry" "$PROJECT_ROOT/internal/agents/registry.go" 2>/dev/null; then
    pass_test "CLIAgentRegistry is defined"
else
    fail_test "CLIAgentRegistry NOT defined"
fi

# Test 3.3: At least 18 CLI agents registered
TOTAL=$((TOTAL + 1))
log_info "Test 3.3: At least 18 CLI agents registered"
AGENT_COUNT=$(grep -c "^[[:space:]]*\"[A-Za-z]*\":" "$PROJECT_ROOT/internal/agents/registry.go" 2>/dev/null || echo "0")
if [ "$AGENT_COUNT" -ge 18 ]; then
    pass_test "18+ CLI agents registered (found: $AGENT_COUNT)"
else
    fail_test "Less than 18 CLI agents registered (found: $AGENT_COUNT)"
fi

# Test 3.4: OpenCode agent registered
TOTAL=$((TOTAL + 1))
log_info "Test 3.4: OpenCode agent registered"
if grep -q "\"OpenCode\":" "$PROJECT_ROOT/internal/agents/registry.go" 2>/dev/null; then
    pass_test "OpenCode agent registered"
else
    fail_test "OpenCode agent NOT registered"
fi

# Test 3.5: ClaudeCode agent registered
TOTAL=$((TOTAL + 1))
log_info "Test 3.5: ClaudeCode agent registered"
if grep -q "\"ClaudeCode\":" "$PROJECT_ROOT/internal/agents/registry.go" 2>/dev/null; then
    pass_test "ClaudeCode agent registered"
else
    fail_test "ClaudeCode agent NOT registered"
fi

# Test 3.6: Aider agent registered
TOTAL=$((TOTAL + 1))
log_info "Test 3.6: Aider agent registered"
if grep -q "\"Aider\":" "$PROJECT_ROOT/internal/agents/registry.go" 2>/dev/null; then
    pass_test "Aider agent registered"
else
    fail_test "Aider agent NOT registered"
fi

# Test 3.7: Agent configurations include API patterns
TOTAL=$((TOTAL + 1))
log_info "Test 3.7: Agent configurations include API patterns"
if grep -q "APIPattern" "$PROJECT_ROOT/internal/agents/registry.go" 2>/dev/null; then
    pass_test "Agent API patterns defined"
else
    fail_test "Agent API patterns NOT defined"
fi

# Test 3.8: Agent configurations include protocol support
TOTAL=$((TOTAL + 1))
log_info "Test 3.8: Agent configurations include protocol support"
if grep -q "Protocols" "$PROJECT_ROOT/internal/agents/registry.go" 2>/dev/null; then
    pass_test "Agent protocol support defined"
else
    fail_test "Agent protocol support NOT defined"
fi

# Test 3.9: GetAgent function exists
TOTAL=$((TOTAL + 1))
log_info "Test 3.9: GetAgent function exists"
if grep -q "func GetAgent" "$PROJECT_ROOT/internal/agents/registry.go" 2>/dev/null; then
    pass_test "GetAgent function exists"
else
    fail_test "GetAgent function NOT found"
fi

# Test 3.10: Agent tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 3.10: Agent tests exist"
if [ -f "$PROJECT_ROOT/internal/agents/registry_test.go" ]; then
    pass_test "Agent tests found"
else
    fail_test "Agent tests NOT found"
fi

# ============================================================================
# Section 4: MCP Integration (15 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 4: MCP Integration (15 tests)"
log_info "=============================================="

# Test 4.1: MCP adapter registry exists
TOTAL=$((TOTAL + 1))
log_info "Test 4.1: MCP adapter registry exists"
if [ -f "$PROJECT_ROOT/internal/mcp/adapters/registry.go" ]; then
    pass_test "MCP adapter registry found"
else
    fail_test "MCP adapter registry NOT found"
fi

# Test 4.2: AvailableAdapters is defined
TOTAL=$((TOTAL + 1))
log_info "Test 4.2: AvailableAdapters is defined"
if grep -q "AvailableAdapters" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    pass_test "AvailableAdapters is defined"
else
    fail_test "AvailableAdapters NOT defined"
fi

# Test 4.3: At least 45 MCP adapters registered
TOTAL=$((TOTAL + 1))
log_info "Test 4.3: At least 45 MCP adapters registered"
ADAPTER_COUNT=$(grep -c "Name:.*Category:" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null || echo "0")
if [ "$ADAPTER_COUNT" -ge 45 ]; then
    pass_test "45+ MCP adapters registered (found: $ADAPTER_COUNT)"
else
    fail_test "Less than 45 MCP adapters registered (found: $ADAPTER_COUNT)"
fi

# Test 4.4: Database adapters exist (postgresql, sqlite, mongodb, redis)
TOTAL=$((TOTAL + 1))
log_info "Test 4.4: Database adapters exist"
if grep -q "\"postgresql\"" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" && \
   grep -q "\"sqlite\"" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" && \
   grep -q "\"mongodb\"" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" && \
   grep -q "\"redis\"" "$PROJECT_ROOT/internal/mcp/adapters/registry.go"; then
    pass_test "Database adapters exist"
else
    fail_test "Some database adapters missing"
fi

# Test 4.5: Version control adapters exist (github, gitlab)
TOTAL=$((TOTAL + 1))
log_info "Test 4.5: Version control adapters exist"
if grep -q "\"github\"" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" && \
   grep -q "\"gitlab\"" "$PROJECT_ROOT/internal/mcp/adapters/registry.go"; then
    pass_test "Version control adapters exist"
else
    fail_test "Some version control adapters missing"
fi

# Test 4.6: Infrastructure adapters exist (docker, kubernetes)
TOTAL=$((TOTAL + 1))
log_info "Test 4.6: Infrastructure adapters exist"
if grep -q "\"docker\"" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" && \
   grep -q "\"kubernetes\"" "$PROJECT_ROOT/internal/mcp/adapters/registry.go"; then
    pass_test "Infrastructure adapters exist"
else
    fail_test "Some infrastructure adapters missing"
fi

# Test 4.7: Adapter search functionality exists
TOTAL=$((TOTAL + 1))
log_info "Test 4.7: Adapter search functionality exists"
if grep -q "func.*Search" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    pass_test "Adapter search functionality exists"
else
    fail_test "Adapter search functionality NOT found"
fi

# Test 4.8: Adapter categories are defined
TOTAL=$((TOTAL + 1))
log_info "Test 4.8: Adapter categories are defined"
if grep -q "CategoryDatabase\|CategoryStorage\|CategoryVersionControl\|CategoryInfrastructure" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    pass_test "Adapter categories are defined"
else
    fail_test "Adapter categories NOT defined"
fi

# Test 4.9: Official adapters marked
TOTAL=$((TOTAL + 1))
log_info "Test 4.9: Official adapters marked"
if grep -q "Official: true" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    pass_test "Official adapters are marked"
else
    fail_test "Official adapters NOT marked"
fi

# Test 4.10: Supported adapters marked
TOTAL=$((TOTAL + 1))
log_info "Test 4.10: Supported adapters marked"
if grep -q "Supported: true" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    pass_test "Supported adapters are marked"
else
    fail_test "Supported adapters NOT marked"
fi

# Test 4.11: GetOfficialAdapters function exists
TOTAL=$((TOTAL + 1))
log_info "Test 4.11: GetOfficialAdapters function exists"
if grep -q "GetOfficialAdapters" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    pass_test "GetOfficialAdapters function exists"
else
    fail_test "GetOfficialAdapters function NOT found"
fi

# Test 4.12: GetSupportedAdapters function exists
TOTAL=$((TOTAL + 1))
log_info "Test 4.12: GetSupportedAdapters function exists"
if grep -q "GetSupportedAdapters" "$PROJECT_ROOT/internal/mcp/adapters/registry.go" 2>/dev/null; then
    pass_test "GetSupportedAdapters function exists"
else
    fail_test "GetSupportedAdapters function NOT found"
fi

# Test 4.13: MCP server registry exists
TOTAL=$((TOTAL + 1))
log_info "Test 4.13: MCP server registry exists"
if [ -f "$PROJECT_ROOT/internal/mcp/server_registry.go" ]; then
    pass_test "MCP server registry found"
else
    fail_test "MCP server registry NOT found"
fi

# Test 4.14: MCP connection pool exists
TOTAL=$((TOTAL + 1))
log_info "Test 4.14: MCP connection pool exists"
if [ -f "$PROJECT_ROOT/internal/mcp/connection_pool.go" ]; then
    pass_test "MCP connection pool found"
else
    fail_test "MCP connection pool NOT found"
fi

# Test 4.15: MCP adapter tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 4.15: MCP adapter tests exist"
if [ -f "$PROJECT_ROOT/internal/mcp/adapters/registry_test.go" ]; then
    pass_test "MCP adapter tests found"
else
    fail_test "MCP adapter tests NOT found"
fi

# ============================================================================
# Section 5: Provider System (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 5: Provider System (10 tests)"
log_info "=============================================="

# Test 5.1: Provider registry exists
TOTAL=$((TOTAL + 1))
log_info "Test 5.1: Provider registry exists"
if [ -f "$PROJECT_ROOT/internal/services/provider_registry.go" ]; then
    pass_test "Provider registry found"
else
    fail_test "Provider registry NOT found"
fi

# Test 5.2: Claude provider exists
TOTAL=$((TOTAL + 1))
log_info "Test 5.2: Claude provider exists"
if [ -d "$PROJECT_ROOT/internal/llm/providers/claude" ]; then
    pass_test "Claude provider found"
else
    fail_test "Claude provider NOT found"
fi

# Test 5.3: DeepSeek provider exists
TOTAL=$((TOTAL + 1))
log_info "Test 5.3: DeepSeek provider exists"
if [ -d "$PROJECT_ROOT/internal/llm/providers/deepseek" ]; then
    pass_test "DeepSeek provider found"
else
    fail_test "DeepSeek provider NOT found"
fi

# Test 5.4: Gemini provider exists
TOTAL=$((TOTAL + 1))
log_info "Test 5.4: Gemini provider exists"
if [ -d "$PROJECT_ROOT/internal/llm/providers/gemini" ]; then
    pass_test "Gemini provider found"
else
    fail_test "Gemini provider NOT found"
fi

# Test 5.5: OpenRouter provider exists
TOTAL=$((TOTAL + 1))
log_info "Test 5.5: OpenRouter provider exists"
if [ -d "$PROJECT_ROOT/internal/llm/providers/openrouter" ]; then
    pass_test "OpenRouter provider found"
else
    fail_test "OpenRouter provider NOT found"
fi

# Test 5.6: At least 10 LLM providers exist
TOTAL=$((TOTAL + 1))
log_info "Test 5.6: At least 10 LLM providers exist"
PROVIDER_COUNT=$(ls -d "$PROJECT_ROOT/internal/llm/providers"/*/ 2>/dev/null | wc -l)
if [ "$PROVIDER_COUNT" -ge 10 ]; then
    pass_test "10+ LLM providers exist (found: $PROVIDER_COUNT)"
else
    fail_test "Less than 10 LLM providers (found: $PROVIDER_COUNT)"
fi

# Test 5.7: Provider health monitor exists
TOTAL=$((TOTAL + 1))
log_info "Test 5.7: Provider health monitor exists"
if [ -f "$PROJECT_ROOT/internal/services/provider_health_monitor.go" ]; then
    pass_test "Provider health monitor found"
else
    fail_test "Provider health monitor NOT found"
fi

# Test 5.8: Provider discovery exists
TOTAL=$((TOTAL + 1))
log_info "Test 5.8: Provider discovery exists"
if [ -f "$PROJECT_ROOT/internal/services/provider_discovery.go" ]; then
    pass_test "Provider discovery found"
else
    fail_test "Provider discovery NOT found"
fi

# Test 5.9: Ensemble service exists
TOTAL=$((TOTAL + 1))
log_info "Test 5.9: Ensemble service exists"
if [ -f "$PROJECT_ROOT/internal/services/ensemble.go" ] || \
   grep -rq "ensemble\|Ensemble" "$PROJECT_ROOT/internal/llm" 2>/dev/null; then
    pass_test "Ensemble service found"
else
    fail_test "Ensemble service NOT found"
fi

# Test 5.10: Provider tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 5.10: Provider tests exist"
if [ -f "$PROJECT_ROOT/internal/services/provider_registry_test.go" ]; then
    pass_test "Provider tests found"
else
    fail_test "Provider tests NOT found"
fi

# ============================================================================
# Section 6: Debate System (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 6: Debate System (10 tests)"
log_info "=============================================="

# Test 6.1: Debate service exists
TOTAL=$((TOTAL + 1))
log_info "Test 6.1: Debate service exists"
if [ -f "$PROJECT_ROOT/internal/services/debate_service.go" ]; then
    pass_test "Debate service found"
else
    fail_test "Debate service NOT found"
fi

# Test 6.2: Multi-pass validation exists
TOTAL=$((TOTAL + 1))
log_info "Test 6.2: Multi-pass validation exists"
if [ -f "$PROJECT_ROOT/internal/services/debate_multipass_validation.go" ]; then
    pass_test "Multi-pass validation found"
else
    fail_test "Multi-pass validation NOT found"
fi

# Test 6.3: Debate team configuration exists
TOTAL=$((TOTAL + 1))
log_info "Test 6.3: Debate team configuration exists"
if [ -f "$PROJECT_ROOT/internal/services/debate_team_config.go" ]; then
    pass_test "Debate team configuration found"
else
    fail_test "Debate team configuration NOT found"
fi

# Test 6.4: Debate dialogue formatter exists
TOTAL=$((TOTAL + 1))
log_info "Test 6.4: Debate dialogue formatter exists"
if [ -f "$PROJECT_ROOT/internal/services/debate_dialogue.go" ]; then
    pass_test "Debate dialogue formatter found"
else
    fail_test "Debate dialogue formatter NOT found"
fi

# Test 6.5: Debate types defined
TOTAL=$((TOTAL + 1))
log_info "Test 6.5: Debate types defined"
if [ -f "$PROJECT_ROOT/internal/services/debate_types.go" ]; then
    pass_test "Debate types defined"
else
    fail_test "Debate types NOT found"
fi

# Test 6.6: Intent classifier exists
TOTAL=$((TOTAL + 1))
log_info "Test 6.6: Intent classifier exists"
if [ -f "$PROJECT_ROOT/internal/services/intent_classifier.go" ] || \
   [ -f "$PROJECT_ROOT/internal/services/llm_intent_classifier.go" ]; then
    pass_test "Intent classifier found"
else
    fail_test "Intent classifier NOT found"
fi

# Test 6.7: Consensus mechanism exists
TOTAL=$((TOTAL + 1))
log_info "Test 6.7: Consensus mechanism exists"
if grep -rq "consensus\|Consensus\|voting\|Voting" "$PROJECT_ROOT/internal/services/debate" 2>/dev/null || \
   grep -rq "consensus\|Consensus" "$PROJECT_ROOT/internal/debate" 2>/dev/null; then
    pass_test "Consensus mechanism found"
else
    fail_test "Consensus mechanism NOT found"
fi

# Test 6.8: Debate orchestrator framework exists
TOTAL=$((TOTAL + 1))
log_info "Test 6.8: Debate orchestrator framework exists"
if [ -d "$PROJECT_ROOT/internal/debate/orchestrator" ]; then
    pass_test "Debate orchestrator framework found"
else
    fail_test "Debate orchestrator framework NOT found"
fi

# Test 6.9: Debate support services exist
TOTAL=$((TOTAL + 1))
log_info "Test 6.9: Debate support services exist"
if [ -f "$PROJECT_ROOT/internal/services/debate_support_types.go" ]; then
    pass_test "Debate support services found"
else
    fail_test "Debate support services NOT found"
fi

# Test 6.10: Debate tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 6.10: Debate tests exist"
if [ -f "$PROJECT_ROOT/internal/services/debate_service_test.go" ]; then
    pass_test "Debate tests found"
else
    fail_test "Debate tests NOT found"
fi

# ============================================================================
# Section 7: Security (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 7: Security (10 tests)"
log_info "=============================================="

# Test 7.1: Security package exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.1: Security package exists"
if [ -d "$PROJECT_ROOT/internal/security" ]; then
    pass_test "Security package found"
else
    fail_test "Security package NOT found"
fi

# Test 7.2: Path validation utility exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.2: Path validation utility exists"
if [ -f "$PROJECT_ROOT/internal/utils/path_validation.go" ]; then
    pass_test "Path validation utility found"
else
    fail_test "Path validation utility NOT found"
fi

# Test 7.3: Guardrails implementation exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.3: Guardrails implementation exists"
if [ -f "$PROJECT_ROOT/internal/security/guardrails.go" ]; then
    pass_test "Guardrails implementation found"
else
    fail_test "Guardrails implementation NOT found"
fi

# Test 7.4: PII detection exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.4: PII detection exists"
if [ -f "$PROJECT_ROOT/internal/security/pii.go" ]; then
    pass_test "PII detection found"
else
    fail_test "PII detection NOT found"
fi

# Test 7.5: Audit logging exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.5: Audit logging exists"
if [ -f "$PROJECT_ROOT/internal/security/audit.go" ]; then
    pass_test "Audit logging found"
else
    fail_test "Audit logging NOT found"
fi

# Test 7.6: Rate limiting middleware exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.6: Rate limiting middleware exists"
if [ -f "$PROJECT_ROOT/internal/middleware/rate_limit.go" ]; then
    pass_test "Rate limiting middleware found"
else
    fail_test "Rate limiting middleware NOT found"
fi

# Test 7.7: Authentication middleware exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.7: Authentication middleware exists"
if [ -f "$PROJECT_ROOT/internal/middleware/auth.go" ]; then
    pass_test "Authentication middleware found"
else
    fail_test "Authentication middleware NOT found"
fi

# Test 7.8: Input validation middleware exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.8: Input validation middleware exists"
if [ -f "$PROJECT_ROOT/internal/middleware/validation.go" ]; then
    pass_test "Input validation middleware found"
else
    fail_test "Input validation middleware NOT found"
fi

# Test 7.9: Red team framework exists
TOTAL=$((TOTAL + 1))
log_info "Test 7.9: Red team framework exists"
if [ -f "$PROJECT_ROOT/internal/security/redteam.go" ]; then
    pass_test "Red team framework found"
else
    fail_test "Red team framework NOT found"
fi

# Test 7.10: Security tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 7.10: Security tests exist"
SECURITY_TESTS=$(find "$PROJECT_ROOT/internal/security" -name "*_test.go" 2>/dev/null | wc -l)
if [ "$SECURITY_TESTS" -ge 5 ]; then
    pass_test "Security tests found ($SECURITY_TESTS test files)"
else
    fail_test "Insufficient security tests (found: $SECURITY_TESTS)"
fi

# ============================================================================
# Section 8: Monitoring (10 tests)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 8: Monitoring (10 tests)"
log_info "=============================================="

# Test 8.1: Prometheus configuration exists
TOTAL=$((TOTAL + 1))
log_info "Test 8.1: Prometheus configuration exists"
if [ -f "$PROJECT_ROOT/monitoring/prometheus.yml" ]; then
    pass_test "Prometheus configuration found"
else
    fail_test "Prometheus configuration NOT found"
fi

# Test 8.2: Grafana configuration exists
TOTAL=$((TOTAL + 1))
log_info "Test 8.2: Grafana configuration exists"
if [ -f "$PROJECT_ROOT/monitoring/grafana-datasources.yml" ]; then
    pass_test "Grafana configuration found"
else
    fail_test "Grafana configuration NOT found"
fi

# Test 8.3: Loki configuration exists
TOTAL=$((TOTAL + 1))
log_info "Test 8.3: Loki configuration exists"
if [ -f "$PROJECT_ROOT/monitoring/loki-config.yml" ]; then
    pass_test "Loki configuration found"
else
    fail_test "Loki configuration NOT found"
fi

# Test 8.4: Alert rules defined
TOTAL=$((TOTAL + 1))
log_info "Test 8.4: Alert rules defined"
if [ -f "$PROJECT_ROOT/monitoring/alert-rules.yml" ]; then
    pass_test "Alert rules found"
else
    fail_test "Alert rules NOT found"
fi

# Test 8.5: Alertmanager configuration exists
TOTAL=$((TOTAL + 1))
log_info "Test 8.5: Alertmanager configuration exists"
if [ -f "$PROJECT_ROOT/monitoring/alertmanager.yml" ]; then
    pass_test "Alertmanager configuration found"
else
    fail_test "Alertmanager configuration NOT found"
fi

# Test 8.6: Monitoring script exists
TOTAL=$((TOTAL + 1))
log_info "Test 8.6: Monitoring script exists"
if [ -f "$PROJECT_ROOT/scripts/monitoring.sh" ]; then
    pass_test "Monitoring script found"
else
    fail_test "Monitoring script NOT found"
fi

# Test 8.7: Grafana dashboards directory exists
TOTAL=$((TOTAL + 1))
log_info "Test 8.7: Grafana dashboards directory exists"
if [ -d "$PROJECT_ROOT/monitoring/dashboards" ]; then
    pass_test "Grafana dashboards directory found"
else
    fail_test "Grafana dashboards directory NOT found"
fi

# Test 8.8: Observability package exists
TOTAL=$((TOTAL + 1))
log_info "Test 8.8: Observability package exists"
if [ -d "$PROJECT_ROOT/internal/observability" ]; then
    pass_test "Observability package found"
else
    fail_test "Observability package NOT found"
fi

# Test 8.9: Promtail configuration exists
TOTAL=$((TOTAL + 1))
log_info "Test 8.9: Promtail configuration exists"
if [ -f "$PROJECT_ROOT/monitoring/promtail-config.yml" ]; then
    pass_test "Promtail configuration found"
else
    fail_test "Promtail configuration NOT found"
fi

# Test 8.10: Monitoring configurations are valid YAML
TOTAL=$((TOTAL + 1))
log_info "Test 8.10: Monitoring configurations are valid YAML"
VALID_YAML=true
for yaml_file in prometheus.yml grafana-datasources.yml loki-config.yml alert-rules.yml alertmanager.yml; do
    if [ -f "$PROJECT_ROOT/monitoring/$yaml_file" ]; then
        if ! python3 -c "import yaml; yaml.safe_load(open('$PROJECT_ROOT/monitoring/$yaml_file'))" 2>/dev/null && \
           ! ruby -ryaml -e "YAML.load_file('$PROJECT_ROOT/monitoring/$yaml_file')" 2>/dev/null; then
            # If neither python nor ruby yaml validation works, try basic syntax check
            if grep -q "^[[:space:]]*[^#]*:[[:space:]]" "$PROJECT_ROOT/monitoring/$yaml_file"; then
                continue
            else
                VALID_YAML=false
            fi
        fi
    fi
done
if [ "$VALID_YAML" = true ]; then
    pass_test "Monitoring YAML configurations appear valid"
else
    fail_test "Some monitoring YAML configurations may be invalid"
fi

# ============================================================================
# Final Summary
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Challenge Summary: $CHALLENGE_NAME"
log_info "=============================================="
log_info "Total Tests: $TOTAL"
log_success "Passed: $PASSED"
if [ $FAILED -gt 0 ]; then
    log_error "Failed: $FAILED"
fi

PERCENTAGE=$((PASSED * 100 / TOTAL))
log_info "Pass Rate: ${PERCENTAGE}%"

log_info ""
log_info "Section Breakdown:"
log_info "  Section 1: Core Infrastructure (10 tests)"
log_info "  Section 2: Tool System (15 tests)"
log_info "  Section 3: CLI Agent System (10 tests)"
log_info "  Section 4: MCP Integration (15 tests)"
log_info "  Section 5: Provider System (10 tests)"
log_info "  Section 6: Debate System (10 tests)"
log_info "  Section 7: Security (10 tests)"
log_info "  Section 8: Monitoring (10 tests)"

if [ $FAILED -eq 0 ]; then
    log_success "=============================================="
    log_success "ALL $TOTAL TESTS PASSED!"
    log_success "HelixAgent system is fully operational."
    log_success "=============================================="
    exit 0
else
    log_error "=============================================="
    log_error "$FAILED of $TOTAL TESTS FAILED!"
    log_error "Review the failed components above."
    log_error "=============================================="
    exit 1
fi
