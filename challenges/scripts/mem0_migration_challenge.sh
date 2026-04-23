#!/bin/bash

# Mem0 Migration Challenge
# Validates that Cognee → Mem0 migration is complete and working correctly
# This challenge ensures the migration cannot regress

set -e

CHALLENGE_NAME="Mem0 Migration Validation"
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=0

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_test() {
    local test_num=$1
    local test_name=$2
    echo -e "${YELLOW}Test $test_num: $test_name${NC}"
}

pass() {
    echo -e "${GREEN}✓ PASS${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
}

fail() {
    local msg=$1
    echo -e "${RED}✗ FAIL: $msg${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
}

echo "=========================================="
echo "$CHALLENGE_NAME"
echo "=========================================="
echo ""

# Test 1: Cognee disabled in config defaults
print_test 1 "Cognee disabled by default in config.go"
if grep -q 'getBoolEnv("COGNEE_ENABLED", false)' internal/config/config.go; then
    pass
else
    fail "COGNEE_ENABLED default is not false in config.go"
fi

# Test 2: AutoCognify disabled in config defaults
print_test 2 "AutoCognify disabled by default in config.go"
if grep -q 'getBoolEnv("COGNEE_AUTO_COGNIFY", false)' internal/config/config.go; then
    pass
else
    fail "COGNEE_AUTO_COGNIFY default is not false in config.go"
fi

# Test 3: Cognee disabled in DefaultServicesConfig
print_test 3 "Cognee service disabled in DefaultServicesConfig"
if grep -A 15 "Cognee: ServiceEndpoint{" internal/config/config.go | grep -q "Enabled:.*false"; then
    pass
else
    fail "Cognee service not disabled in DefaultServicesConfig"
fi

# Test 4: Cognee not required in DefaultServicesConfig
print_test 4 "Cognee service not required in DefaultServicesConfig"
if grep -A 15 "Cognee: ServiceEndpoint{" internal/config/config.go | grep -q "Required:.*false"; then
    pass
else
    fail "Cognee service still marked as required in DefaultServicesConfig"
fi

# Test 5: Memory field added to Config struct
print_test 5 "Memory field exists in Config struct"
if grep -q "Memory.*memory.MemoryConfig" internal/config/config.go; then
    pass
else
    fail "Memory field not found in Config struct"
fi

# Test 6: Memory service checks Cognee.Enabled
print_test 6 "Memory service checks if Cognee is enabled"
if grep -q "!cfg.Cognee.Enabled" internal/services/memory_service.go; then
    pass
else
    fail "Memory service doesn't check cfg.Cognee.Enabled"
fi

# Test 7: Mem0 configuration in development.yaml
print_test 7 "Mem0 memory configuration exists in development.yaml"
if grep -q "^memory:" configs/development.yaml; then
    pass
else
    fail "Mem0 memory configuration not found in development.yaml"
fi

# Test 8: Cognee disabled in development.yaml main section
print_test 8 "Cognee disabled in development.yaml (main cognee section)"
if grep -A 5 "^cognee:" configs/development.yaml | grep -q "enabled: false"; then
    pass
else
    fail "Cognee not disabled in main section of development.yaml"
fi

# Test 9: Cognee disabled in ai_debate section
print_test 9 "Cognee disabled in ai_debate section of development.yaml"
if awk '/^ai_debate:/{found=1} found && /cognee:/{cfound=1} cfound && /enabled: false/{print; exit}' configs/development.yaml | grep -q "enabled: false"; then
    pass
else
    fail "Cognee not disabled in ai_debate section of development.yaml"
fi

# Test 10: Cognee service disabled in services section
print_test 10 "Cognee service disabled in services section of development.yaml"
if awk '/^services:/{found=1} found && /^  cognee:/{cfound=1} cfound && /enabled: false/{print; exit}' configs/development.yaml | grep -q "enabled: false"; then
    pass
else
    fail "Cognee service not disabled in services section of development.yaml"
fi

# Test 11: Cognee service not required in services section
print_test 11 "Cognee service not required in services section of development.yaml"
if awk '/^services:/{found=1} found && /^  cognee:/{cfound=1} cfound && /required: false/{print; exit}' configs/development.yaml | grep -q "required: false"; then
    pass
else
    fail "Cognee service still required in services section of development.yaml"
fi

# Test 12: Mem0 uses PostgreSQL backend
print_test 12 "Mem0 configured to use PostgreSQL backend"
if grep -A 20 "^memory:" configs/development.yaml | grep -q "storage_type: \"postgres\""; then
    pass
else
    fail "Mem0 not configured with PostgreSQL backend"
fi

# Test 13: Mem0 entity graph enabled
print_test 13 "Mem0 entity graph enabled"
if grep -A 20 "^memory:" configs/development.yaml | grep -q "enable_graph: true"; then
    pass
else
    fail "Mem0 entity graph not enabled"
fi

# Test 14: Mem0 embedding model configured
print_test 14 "Mem0 embedding model configured"
if grep -A 20 "^memory:" configs/development.yaml | grep -q "embedding_model:"; then
    pass
else
    fail "Mem0 embedding model not configured"
fi

# Test 15: CogneeService.SearchMemory checks enabled flag
print_test 15 "CogneeService.SearchMemory returns early when disabled"
if grep -A 10 "func.*SearchMemory" internal/services/cognee_service.go | grep -q "!s.config.Enabled"; then
    pass
else
    fail "SearchMemory doesn't check if Cognee is disabled"
fi

# Test 16: CogneeService.EnhanceRequest checks enabled flag
print_test 16 "CogneeService.EnhanceRequest returns early when disabled"
if grep -A 10 "func.*EnhanceRequest" internal/services/cognee_service.go | grep -q "!s.config.Enabled"; then
    pass
else
    fail "EnhanceRequest doesn't check if Cognee is disabled"
fi

# Test 17: Build succeeds
print_test 17 "HelixAgent builds successfully"
if make build > /dev/null 2>&1; then
    pass
else
    fail "HelixAgent build failed"
fi

# Test 18: Binary exists
print_test 18 "HelixAgent binary exists"
if [ -f bin/helixagent ]; then
    pass
else
    fail "Binary not found at bin/helixagent"
fi

# Test 19: Check if HelixAgent is running
print_test 19 "HelixAgent process is running"
if pgrep -f "bin/helixagent" > /dev/null; then
    pass
else
    fail "HelixAgent not running"
fi

# Test 20: Check for Cognee errors in recent logs
print_test 20 "No Cognee connection errors in recent logs"
if [ -f /tmp/helixagent_v2.log ]; then
    COGNEE_ERRORS=$(tail -200 /tmp/helixagent_v2.log | grep -iE "dial tcp.*:8000|Search error.*cognee" | wc -l)
    if [ "$COGNEE_ERRORS" -eq 0 ]; then
        pass
    else
        fail "Found $COGNEE_ERRORS Cognee connection errors in logs"
    fi
else
    echo "  (Skipping - log file not found)"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
fi

# Test 21: Health endpoint responds
print_test 21 "Health endpoint is accessible"
if timeout 5 curl -sf http://localhost:8100/health > /dev/null 2>&1; then
    pass
else
    fail "Health endpoint not responding"
fi

# Test 22: Health status is healthy
print_test 22 "Health status is 'healthy'"
HEALTH_RESPONSE=$(timeout 5 curl -s --max-time 60 http://localhost:8100/health 2>/dev/null || echo '{"status":"unreachable"}')
if echo "$HEALTH_RESPONSE" | grep -q '"status":"healthy"'; then
    pass
else
    fail "Health status is not healthy: $HEALTH_RESPONSE"
fi

# Test 23: Cognee container not in required services
print_test 23 "Cognee is not required for HelixAgent operation"
# Cognee may be running for backwards compatibility, but it must NOT be required.
# Verify that HelixAgent config marks Cognee as not-required (Mem0 is primary).
if awk '/^services:/{found=1} found && /^  cognee:/{cfound=1} cfound && /required: false/{print; exit}' configs/development.yaml | grep -q "required: false"; then
    pass
else
    fail "Cognee service is marked as required in development.yaml - migration incomplete"
fi

# Test 24: PostgreSQL container running (for Mem0)
print_test 24 "PostgreSQL container running (required for Mem0)"
if podman ps 2>/dev/null | grep -q helixagent-postgres; then
    pass
else
    fail "PostgreSQL container not running (required for Mem0)"
fi

# Test 25: Redis container running
print_test 25 "Redis container running"
if podman ps 2>/dev/null | grep -q helixagent-redis; then
    pass
else
    fail "Redis container not running"
fi

# Test 26: Documentation mentions Mem0 as primary
print_test 26 "Documentation updated to mention Mem0 as primary"
if grep -q "Mem0" CLAUDE.md; then
    pass
else
    fail "CLAUDE.md not updated to mention Mem0"
fi

# Test 27: Cognee disabled comment in config
print_test 27 "Config has comment explaining Cognee is disabled"
if grep -i "disabled.*mem0" configs/development.yaml; then
    pass
else
    fail "No comment explaining Cognee → Mem0 migration"
fi

# Test 28: Mem0 comment in config
print_test 28 "Config has comment marking Mem0 as PRIMARY"
if grep -i "PRIMARY.*MEMORY.*PROVIDER" configs/development.yaml; then
    pass
else
    fail "No comment marking Mem0 as primary memory provider"
fi

# Test 29: No hardcoded Cognee enabling
print_test 29 "No hardcoded 'Enabled: true' for Cognee in Go code"
if grep -n "Cognee.*Enabled.*true" internal/config/config.go | grep -v "getBoolEnv"; then
    fail "Found hardcoded Enabled: true for Cognee"
else
    pass
fi

# Test 30: No hardcoded AutoCognify enabling
print_test 30 "No hardcoded 'AutoCognify: true' for Cognee in Go code"
if grep -n "AutoCognify.*true" internal/config/config.go | grep -v "getBoolEnv"; then
    fail "Found hardcoded AutoCognify: true"
else
    pass
fi

echo ""
echo "=========================================="
echo "RESULTS"
echo "=========================================="
echo "Total Tests: $TOTAL_TESTS"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
    echo "Mem0 migration is complete and validated!"
    exit 0
else
    echo ""
    echo -e "${RED}✗ SOME TESTS FAILED${NC}"
    echo "Mem0 migration validation failed!"
    exit 1
fi
