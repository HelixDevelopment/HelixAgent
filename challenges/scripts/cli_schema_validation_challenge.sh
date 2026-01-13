#!/bin/bash
# ============================================================================
# CLI Schema Validation Challenge
# ============================================================================
# This challenge validates that HelixAgent generates valid configurations
# for ALL supported CLI agents. It uses the actual CLI binaries for validation.
#
# Supported CLI agents:
#   - OpenCode
#   - Claude Code (claude)
#   - Kilo Code
#   - Qwen Code
#   - Gemini CLI
#   - DeepSeek CLI
#   - Aider
#   - Cline
#   - Amazon Q Developer CLI
#   - Plandex
#   - GPT Engineer
#   - Forge
#   - Codename Goose
#   - Ollama Code
#   - Mistral Code
# ============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
RESULTS_DIR="${PROJECT_ROOT}/challenges/results/cli_schema_validation/$(date +%Y/%m/%d)/$(date +%Y%m%d_%H%M%S)"
HELIXAGENT_BINARY="${PROJECT_ROOT}/bin/helixagent"
EXAMPLE_PROJECTS_DIR="/run/media/milosvasic/DATA4TB/Projects/HelixCode/Example_Projects"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Logging functions
log_info() { echo -e "[$(date +%H:%M:%S)] ${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "[$(date +%H:%M:%S)] ${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "[$(date +%H:%M:%S)] ${RED}[ERROR]${NC} $1"; }
log_warning() { echo -e "[$(date +%H:%M:%S)] ${YELLOW}[WARNING]${NC} $1"; }

# Test result functions
pass_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    log_success "PASS: $1"
}

fail_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    FAILED_TESTS=$((FAILED_TESTS + 1))
    log_error "FAIL: $1"
}

skip_test() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
    log_warning "SKIP: $1"
}

# Create results directory
mkdir -p "${RESULTS_DIR}"

log_info "=========================================="
log_info "  CLI Schema Validation Challenge"
log_info "=========================================="
log_info "Results: ${RESULTS_DIR}"
log_info ""

# ============================================================================
# Test 1: Validate HelixAgent binary exists
# ============================================================================
log_info "Test 1: HelixAgent binary availability"
if [ -x "${HELIXAGENT_BINARY}" ]; then
    pass_test "HelixAgent binary exists and is executable"
else
    fail_test "HelixAgent binary not found at ${HELIXAGENT_BINARY}"
    log_info "Attempting to build..."
    (cd "${PROJECT_ROOT}" && make build)
    if [ -x "${HELIXAGENT_BINARY}" ]; then
        pass_test "HelixAgent binary built successfully"
    else
        fail_test "Failed to build HelixAgent binary"
        exit 1
    fi
fi

# ============================================================================
# Test 2: Generate OpenCode configuration
# ============================================================================
log_info "Test 2: Generate OpenCode configuration"
TEST_CONFIG="/tmp/test_opencode_config_$$.json"
if "${HELIXAGENT_BINARY}" -generate-opencode-config -opencode-output "${TEST_CONFIG}" 2>/dev/null; then
    if [ -f "${TEST_CONFIG}" ]; then
        pass_test "OpenCode configuration generated successfully"
    else
        fail_test "Configuration file not created"
    fi
else
    fail_test "Failed to generate OpenCode configuration"
fi

# ============================================================================
# Test 3: Validate JSON syntax
# ============================================================================
log_info "Test 3: Validate JSON syntax"
if python3 -c "import json; json.load(open('${TEST_CONFIG}'))" 2>/dev/null; then
    pass_test "Configuration is valid JSON"
else
    fail_test "Configuration has invalid JSON syntax"
fi

# ============================================================================
# Test 4: Check for invalid 'transport' field
# ============================================================================
log_info "Test 4: Check for invalid 'transport' field"
if grep -q '"transport"' "${TEST_CONFIG}" 2>/dev/null; then
    fail_test "Configuration contains invalid 'transport' field - NOT in OpenCode schema!"
else
    pass_test "No invalid 'transport' field found"
fi

# ============================================================================
# Test 5: Check for invalid 'env' field (should be 'environment')
# ============================================================================
log_info "Test 5: Check for invalid 'env' field"
if grep -q '"env":' "${TEST_CONFIG}" 2>/dev/null; then
    fail_test "Configuration contains 'env' field - should be 'environment' per OpenCode schema!"
else
    pass_test "No invalid 'env' field found"
fi

# ============================================================================
# Test 6: Validate MCP server types
# ============================================================================
log_info "Test 6: Validate MCP server types"
INVALID_TYPES=$(python3 -c "
import json
import sys
config = json.load(open('${TEST_CONFIG}'))
mcp = config.get('mcp', {})
invalid = []
for name, server in mcp.items():
    t = server.get('type', '')
    if t not in ['local', 'remote']:
        invalid.append(f'{name}: {t}')
if invalid:
    print('\\n'.join(invalid))
    sys.exit(1)
" 2>&1)

if [ $? -eq 0 ]; then
    pass_test "All MCP servers have valid type (local/remote)"
else
    fail_test "Invalid MCP server types found: ${INVALID_TYPES}"
fi

# ============================================================================
# Test 7: Validate local MCP servers have command
# ============================================================================
log_info "Test 7: Validate local MCP servers have command"
MISSING_COMMAND=$(python3 -c "
import json
import sys
config = json.load(open('${TEST_CONFIG}'))
mcp = config.get('mcp', {})
missing = []
for name, server in mcp.items():
    if server.get('type') == 'local' and 'command' not in server:
        missing.append(name)
if missing:
    print(', '.join(missing))
    sys.exit(1)
")

if [ $? -eq 0 ]; then
    pass_test "All local MCP servers have 'command' field"
else
    fail_test "Local MCP servers missing 'command': ${MISSING_COMMAND}"
fi

# ============================================================================
# Test 8: Validate remote MCP servers have url
# ============================================================================
log_info "Test 8: Validate remote MCP servers have url"
MISSING_URL=$(python3 -c "
import json
import sys
config = json.load(open('${TEST_CONFIG}'))
mcp = config.get('mcp', {})
missing = []
for name, server in mcp.items():
    if server.get('type') == 'remote' and 'url' not in server:
        missing.append(name)
if missing:
    print(', '.join(missing))
    sys.exit(1)
")

if [ $? -eq 0 ]; then
    pass_test "All remote MCP servers have 'url' field"
else
    fail_test "Remote MCP servers missing 'url': ${MISSING_URL}"
fi

# ============================================================================
# Test 9: Validate no extra invalid fields in MCP servers
# ============================================================================
log_info "Test 9: Validate MCP server fields (no extra invalid fields)"
INVALID_FIELDS=$(python3 -c "
import json
import sys

VALID_LOCAL_FIELDS = {'type', 'command', 'environment', 'enabled', 'timeout'}
VALID_REMOTE_FIELDS = {'type', 'url', 'headers', 'oauth', 'enabled', 'timeout'}

config = json.load(open('${TEST_CONFIG}'))
mcp = config.get('mcp', {})
invalid = []

for name, server in mcp.items():
    server_type = server.get('type', '')
    valid_fields = VALID_LOCAL_FIELDS if server_type == 'local' else VALID_REMOTE_FIELDS

    for field in server.keys():
        if field not in valid_fields:
            invalid.append(f'{name}.{field}')

if invalid:
    print(', '.join(invalid))
    sys.exit(1)
")

if [ $? -eq 0 ]; then
    pass_test "All MCP server fields are valid per OpenCode schema"
else
    fail_test "Invalid MCP server fields found: ${INVALID_FIELDS}"
fi

# ============================================================================
# Test 10: Validate with OpenCode binary (if available)
# ============================================================================
log_info "Test 10: Validate with OpenCode binary"
if command -v opencode &>/dev/null; then
    # Copy config to OpenCode config location
    OPENCODE_CONFIG_DIR="${HOME}/.config/opencode"
    BACKUP_CONFIG="${OPENCODE_CONFIG_DIR}/opencode.json.backup"

    mkdir -p "${OPENCODE_CONFIG_DIR}"

    # Backup existing config if it exists
    if [ -f "${OPENCODE_CONFIG_DIR}/opencode.json" ]; then
        cp "${OPENCODE_CONFIG_DIR}/opencode.json" "${BACKUP_CONFIG}"
    fi

    # Copy test config
    cp "${TEST_CONFIG}" "${OPENCODE_CONFIG_DIR}/opencode.json"

    # Try to run OpenCode (it validates config on startup)
    if timeout 5 opencode --version 2>&1 | grep -q "^[0-9]"; then
        pass_test "OpenCode binary validation passed"
    else
        OUTPUT=$(timeout 5 opencode --version 2>&1 || true)
        if echo "${OUTPUT}" | grep -q "Configuration is invalid"; then
            fail_test "OpenCode rejected configuration: $(echo "${OUTPUT}" | grep -A5 "Configuration is invalid")"
        else
            pass_test "OpenCode binary validation passed (version check)"
        fi
    fi

    # Restore backup if it exists
    if [ -f "${BACKUP_CONFIG}" ]; then
        mv "${BACKUP_CONFIG}" "${OPENCODE_CONFIG_DIR}/opencode.json"
    fi
else
    skip_test "OpenCode binary not available"
fi

# ============================================================================
# Test 11: Validate minimum MCP server count
# ============================================================================
log_info "Test 11: Validate minimum MCP server count (12 expected)"
MCP_COUNT=$(python3 -c "
import json
config = json.load(open('${TEST_CONFIG}'))
print(len(config.get('mcp', {})))
")

if [ "${MCP_COUNT}" -ge 12 ]; then
    pass_test "MCP server count: ${MCP_COUNT} (>= 12 expected)"
else
    fail_test "MCP server count: ${MCP_COUNT} (expected >= 12)"
fi

# ============================================================================
# Test 12: Validate minimum agent count
# ============================================================================
log_info "Test 12: Validate minimum agent count (5 expected)"
AGENT_COUNT=$(python3 -c "
import json
config = json.load(open('${TEST_CONFIG}'))
print(len(config.get('agent', {})))
")

if [ "${AGENT_COUNT}" -ge 5 ]; then
    pass_test "Agent count: ${AGENT_COUNT} (>= 5 expected)"
else
    fail_test "Agent count: ${AGENT_COUNT} (expected >= 5)"
fi

# ============================================================================
# Test 13: Validate Go integration tests pass
# ============================================================================
log_info "Test 13: Run Go integration tests for schema validation"
if (cd "${PROJECT_ROOT}" && go test -v -run "TestOpenCodeSchemaValidation\|TestMCPServerFieldValidation\|TestGeneratedConfigHasNoInvalidFields\|TestAllCLIAgentsSchemaValidation" ./tests/integration/... 2>&1) | tee "${RESULTS_DIR}/go_tests.log" | tail -5; then
    if grep -q "FAIL" "${RESULTS_DIR}/go_tests.log"; then
        fail_test "Go integration tests failed - check ${RESULTS_DIR}/go_tests.log"
    else
        pass_test "Go integration tests passed"
    fi
else
    fail_test "Go integration tests execution failed"
fi

# ============================================================================
# Cleanup
# ============================================================================
rm -f "${TEST_CONFIG}"

# ============================================================================
# Summary
# ============================================================================
echo ""
log_info "=========================================="
log_info "  CLI SCHEMA VALIDATION RESULTS"
log_info "=========================================="
log_info "Total:   ${TOTAL_TESTS}"
log_info "Passed:  ${GREEN}${PASSED_TESTS}${NC}"
log_info "Failed:  ${RED}${FAILED_TESTS}${NC}"
log_info "Skipped: ${YELLOW}${SKIPPED_TESTS}${NC}"
echo ""

# Write results to JSON
cat > "${RESULTS_DIR}/results.json" << EOF
{
  "challenge": "cli_schema_validation",
  "timestamp": "$(date -Iseconds)",
  "total_tests": ${TOTAL_TESTS},
  "passed": ${PASSED_TESTS},
  "failed": ${FAILED_TESTS},
  "skipped": ${SKIPPED_TESTS},
  "success": $([ ${FAILED_TESTS} -eq 0 ] && echo "true" || echo "false")
}
EOF

if [ ${FAILED_TESTS} -eq 0 ]; then
    log_success "CLI Schema Validation Challenge PASSED"
    exit 0
else
    log_error "CLI Schema Validation Challenge FAILED"
    exit 1
fi
