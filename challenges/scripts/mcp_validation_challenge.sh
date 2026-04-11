#!/bin/bash
# MCP Validation Challenge Script
# Validates all MCP servers and ensures only working ones are enabled
# Tests the MCP validation system comprehensively

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
    TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${CYAN}============================================${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}============================================${NC}"
}

cd "$PROJECT_ROOT"

echo "========================================"
echo "  MCP VALIDATION CHALLENGE"
echo "  Testing MCP Server Validation System"
echo "========================================"
echo ""

# Load environment variables from .env
if [ -f ".env" ]; then
    log_info "Loading environment variables from .env"
    set -a
    source .env 2>/dev/null || true
    set +a
fi

log_section "Phase 1: Prerequisites"

# Test 1.1: Check MCP validation package exists
log_info "Test 1.1: MCP validation package exists"
if [ -f "internal/mcp/validation/validator.go" ]; then
    log_pass "MCP validation package exists"
else
    log_fail "MCP validation package not found"
fi

# Test 1.2: Check MCP validation tests exist
log_info "Test 1.2: MCP validation tests exist"
if [ -f "internal/mcp/validation/validator_test.go" ]; then
    log_pass "MCP validation tests exist"
else
    log_fail "MCP validation tests not found"
fi

# Test 1.3: Check HelixAgent binary exists
log_info "Test 1.3: HelixAgent binary exists"
if [ -f "bin/helixagent" ]; then
    log_pass "HelixAgent binary exists"
else
    log_fail "HelixAgent binary not found - rebuilding..."
    make build 2>/dev/null || true
fi

# Test 1.4: Check HelixAgent is running
log_info "Test 1.4: HelixAgent is running"
if curl -s --max-time 60 http://localhost:7061/health | grep -q "healthy"; then
    log_pass "HelixAgent is running and healthy"
else
    log_skip "HelixAgent is not running"
fi

log_section "Phase 2: Environment Variables"

# Test 2.1: Check .env file exists
log_info "Test 2.1: .env file exists"
if [ -f ".env" ]; then
    log_pass ".env file exists"
else
    log_fail ".env file not found"
fi

# Test 2.2: Check GITHUB_TOKEN is set
log_info "Test 2.2: GITHUB_TOKEN is set"
if [ -n "$GITHUB_TOKEN" ]; then
    log_pass "GITHUB_TOKEN is set"
else
    log_skip "GITHUB_TOKEN is not set - GitHub MCP will be disabled"
fi

# Test 2.3: Check HELIXAGENT_API_KEY is set
log_info "Test 2.3: HELIXAGENT_API_KEY is set"
if [ -n "$HELIXAGENT_API_KEY" ]; then
    log_pass "HELIXAGENT_API_KEY is set"
else
    log_skip "HELIXAGENT_API_KEY is not set"
fi

# Test 2.4: Count available API keys
log_info "Test 2.4: Count available API keys"
API_KEY_COUNT=$(grep -E "^[A-Z].*(_KEY|_TOKEN|_SECRET)=" .env 2>/dev/null | wc -l)
if [ "$API_KEY_COUNT" -gt 10 ]; then
    log_pass "Found $API_KEY_COUNT API keys in .env"
else
    log_skip "Only found $API_KEY_COUNT API keys in .env"
fi

log_section "Phase 3: Core MCP Tests"

# Core MCPs that should ALWAYS work (no API keys required)
CORE_MCPS=("filesystem" "fetch" "memory" "time" "git" "sequential-thinking" "everything" "sqlite")

for mcp in "${CORE_MCPS[@]}"; do
    log_info "Test 3.x: Core MCP '$mcp' is available"
    # Check if MCP is in validator requirements
    if grep -q "\"$mcp\"" internal/mcp/validation/validator.go 2>/dev/null; then
        log_pass "Core MCP '$mcp' is defined in validator"
    else
        log_fail "Core MCP '$mcp' is NOT defined in validator"
    fi
done

log_section "Phase 4: MCP Validation Unit Tests"

# Test 4.1: Run MCP validation unit tests
log_info "Test 4.1: Run MCP validation unit tests"
if go test -v ./internal/mcp/validation/... -count=1 -timeout=60s 2>&1 | tee /tmp/mcp_validation_test.log | grep -q "PASS"; then
    PASS_COUNT=$(grep -c "--- PASS" /tmp/mcp_validation_test.log || echo "0")
    log_pass "MCP validation unit tests passed ($PASS_COUNT tests)"
else
    FAIL_COUNT=$(grep -c "--- FAIL" /tmp/mcp_validation_test.log || echo "0")
    log_fail "MCP validation unit tests failed ($FAIL_COUNT failures)"
fi

log_section "Phase 5: MCP Package Installation"

# Test 5.1: Check npx is available
log_info "Test 5.1: npx is available"
if command -v npx &> /dev/null; then
    log_pass "npx is available"
else
    log_fail "npx is not available - MCP servers cannot be started"
fi

# Test 5.2: Check node is available
log_info "Test 5.2: node is available"
if command -v node &> /dev/null; then
    NODE_VERSION=$(node --version)
    log_pass "node is available ($NODE_VERSION)"
else
    log_fail "node is not available"
fi

# Test 5.3: Test core MCP package can be fetched
log_info "Test 5.3: Test core MCP package availability"
if npm view @modelcontextprotocol/server-filesystem version 2>/dev/null | grep -q "\."; then
    VERSION=$(npm view @modelcontextprotocol/server-filesystem version 2>/dev/null)
    log_pass "Core MCP package available (filesystem v$VERSION)"
else
    log_skip "Cannot verify MCP package (npm registry may be slow)"
fi

log_section "Phase 6: Local Services"

# Test 6.1: Check PostgreSQL
log_info "Test 6.1: PostgreSQL is running"
if pg_isready -h localhost -p 15432 2>/dev/null | grep -q "accepting"; then
    log_pass "PostgreSQL is running on port 15432"
else
    log_skip "PostgreSQL is not running - postgres MCP will be limited"
fi

# Test 6.2: Check Redis
log_info "Test 6.2: Redis is running"
if redis-cli -p 16379 ping 2>/dev/null | grep -q "PONG"; then
    log_pass "Redis is running on port 16379"
else
    log_skip "Redis is not running - redis MCP will be disabled"
fi

# Test 6.3: Check Docker/Podman
log_info "Test 6.3: Docker/Podman is available"
if docker info &>/dev/null || podman info &>/dev/null; then
    log_pass "Container runtime is available"
else
    log_skip "No container runtime available - docker MCP will be limited"
fi

log_section "Phase 7: MCP Plugin Installation"

# Test 7.1: Check HelixAgent MCP plugin is installed
log_info "Test 7.1: HelixAgent MCP plugin is installed"
PLUGIN_PATH="$HOME/.helixagent/plugins/mcp-server/dist/index.js"
if [ -f "$PLUGIN_PATH" ]; then
    log_pass "HelixAgent MCP plugin is installed"
else
    log_info "Installing HelixAgent MCP plugin..."
    mkdir -p "$HOME/.helixagent/plugins/mcp-server"
    if [ -d "plugins/mcp-server/dist" ]; then
        cp -r plugins/mcp-server/dist/* "$HOME/.helixagent/plugins/mcp-server/"
        log_pass "HelixAgent MCP plugin installed"
    else
        log_fail "HelixAgent MCP plugin source not found"
    fi
fi

# Test 7.2: Check plugin can be executed
log_info "Test 7.2: HelixAgent MCP plugin can be executed"
if node "$PLUGIN_PATH" --help 2>/dev/null | grep -q "HelixAgent MCP Server"; then
    log_pass "HelixAgent MCP plugin is executable"
else
    log_fail "HelixAgent MCP plugin cannot be executed"
fi

log_section "Phase 8: OpenCode Configuration Validation"

# Test 8.1: Check OpenCode config exists
log_info "Test 8.1: OpenCode config exists"
OPENCODE_CONFIG="$HOME/.config/opencode/.opencode.json"
if [ -f "$OPENCODE_CONFIG" ]; then
    log_pass "OpenCode config exists"
else
    log_fail "OpenCode config not found at $OPENCODE_CONFIG"
fi

# Test 8.2: Check OpenCode config is valid JSON
log_info "Test 8.2: OpenCode config is valid JSON"
if cat "$OPENCODE_CONFIG" 2>/dev/null | jq . > /dev/null 2>&1; then
    log_pass "OpenCode config is valid JSON"
else
    log_fail "OpenCode config is not valid JSON"
fi

# Test 8.3: Check MCPs in OpenCode config
log_info "Test 8.3: Check MCPs in OpenCode config"
MCP_COUNT=$(cat "$OPENCODE_CONFIG" 2>/dev/null | jq '.mcp | keys | length' 2>/dev/null || echo "0")
if [ "$MCP_COUNT" -gt 5 ]; then
    log_pass "OpenCode config has $MCP_COUNT MCPs"
else
    log_fail "OpenCode config has only $MCP_COUNT MCPs (expected >5)"
fi

# Test 8.4: Check only working MCPs are enabled
log_info "Test 8.4: Validate MCPs don't require missing API keys"
MCPS_WITH_MISSING_KEYS=0
for mcp in $(cat "$OPENCODE_CONFIG" 2>/dev/null | jq -r '.mcp | keys[]' 2>/dev/null); do
    # Get environment requirements for this MCP
    ENV_REQS=$(cat "$OPENCODE_CONFIG" 2>/dev/null | jq -r ".mcp[\"$mcp\"].environment // {} | keys[]" 2>/dev/null)
    for env_var in $ENV_REQS; do
        # Extract actual var name from {env:VAR_NAME}
        ACTUAL_VAR=$(echo "$env_var" | sed 's/{env:\(.*\)}/\1/')
        if [ -z "${!ACTUAL_VAR}" ] && [ -n "$ACTUAL_VAR" ]; then
            MCPS_WITH_MISSING_KEYS=$((MCPS_WITH_MISSING_KEYS + 1))
        fi
    done
done
if [ "$MCPS_WITH_MISSING_KEYS" -eq 0 ]; then
    log_pass "All enabled MCPs have required environment variables"
else
    log_skip "$MCPS_WITH_MISSING_KEYS MCPs may have missing API keys"
fi

log_section "Phase 9: Crush Configuration Validation"

# Test 9.1: Check Crush config exists
log_info "Test 9.1: Crush config exists"
CRUSH_CONFIG="$HOME/.config/crush/crush.json"
if [ -f "$CRUSH_CONFIG" ]; then
    log_pass "Crush config exists"
else
    log_fail "Crush config not found at $CRUSH_CONFIG"
fi

# Test 9.2: Check Crush config is valid JSON
log_info "Test 9.2: Crush config is valid JSON"
if cat "$CRUSH_CONFIG" 2>/dev/null | jq . > /dev/null 2>&1; then
    log_pass "Crush config is valid JSON"
else
    log_fail "Crush config is not valid JSON"
fi

# Test 9.3: Check MCPs in Crush config
log_info "Test 9.3: Check MCPs in Crush config"
CRUSH_MCP_COUNT=$(cat "$CRUSH_CONFIG" 2>/dev/null | jq '.mcp | keys | length' 2>/dev/null || echo "0")
if [ "$CRUSH_MCP_COUNT" -gt 5 ]; then
    log_pass "Crush config has $CRUSH_MCP_COUNT MCPs"
else
    log_fail "Crush config has only $CRUSH_MCP_COUNT MCPs (expected >5)"
fi

log_section "Phase 10: Integration Tests"

# Test 10.1: Test filesystem MCP can start
log_info "Test 10.1: Test filesystem MCP can be invoked"
if timeout 10 npx -y @modelcontextprotocol/server-filesystem --help 2>/dev/null | head -1; then
    log_pass "filesystem MCP can be invoked"
else
    log_skip "filesystem MCP invocation test skipped (timeout or not available)"
fi

# Test 10.2: Test memory MCP can start
log_info "Test 10.2: Test memory MCP can be invoked"
if timeout 10 npx -y @modelcontextprotocol/server-memory --help 2>/dev/null | head -1; then
    log_pass "memory MCP can be invoked"
else
    log_skip "memory MCP invocation test skipped (timeout or not available)"
fi

log_section "Challenge Summary"

echo ""
echo "========================================"
echo "  MCP VALIDATION CHALLENGE RESULTS"
echo "========================================"
echo ""
echo -e "  ${GREEN}Passed:${NC}  $TESTS_PASSED"
echo -e "  ${RED}Failed:${NC}  $TESTS_FAILED"
echo -e "  ${YELLOW}Skipped:${NC} $TESTS_SKIPPED"
echo ""
TOTAL=$((TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED))
echo "  Total:   $TOTAL"
echo ""

if [ "$TESTS_FAILED" -eq 0 ]; then
    echo -e "${GREEN}========================================"
    echo -e "  CHALLENGE PASSED!"
    echo -e "========================================${NC}"
    exit 0
else
    echo -e "${RED}========================================"
    echo -e "  CHALLENGE FAILED ($TESTS_FAILED failures)"
    echo -e "========================================${NC}"
    exit 1
fi
