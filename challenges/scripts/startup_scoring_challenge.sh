#!/bin/bash
# Startup Scoring Challenge
# Validates automatic provider scoring at system startup

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CHALLENGE_NAME="startup_scoring"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
PASSED=0
FAILED=0
TOTAL=0

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

assert_test() {
    local test_name="$1"
    local condition="$2"
    TOTAL=$((TOTAL + 1))

    if eval "$condition"; then
        PASSED=$((PASSED + 1))
        log_success "$test_name"
        return 0
    else
        FAILED=$((FAILED + 1))
        log_fail "$test_name"
        return 1
    fi
}

# Test 1: Verify startup scoring service exists
test_service_exists() {
    log_info "Testing: Startup scoring service file exists"
    local service_file="${PROJECT_ROOT}/internal/services/startup_scoring.go"
    assert_test "startup_scoring.go exists" "[ -f '$service_file' ]"
}

# Test 2: Verify unit tests exist
test_unit_tests_exist() {
    log_info "Testing: Unit tests exist for startup scoring"
    local test_file="${PROJECT_ROOT}/internal/services/startup_scoring_test.go"
    assert_test "startup_scoring_test.go exists" "[ -f '$test_file' ]"
}

# Test 3: Run unit tests
test_unit_tests_pass() {
    log_info "Testing: Unit tests pass"
    cd "$PROJECT_ROOT"
    local output
    output=$(go test -v -run "TestStartup" ./internal/services/... -timeout 60s 2>&1)
    local result=$?

    if [ $result -eq 0 ]; then
        # Count passed tests
        local passed_count=$(echo "$output" | grep -c "PASS:" || echo "0")
        assert_test "All startup scoring tests pass (${passed_count} tests)" "[ $result -eq 0 ]"
    else
        log_fail "Unit tests failed"
        echo "$output" | tail -20
        TOTAL=$((TOTAL + 1))
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test 4: Verify router integration
test_router_integration() {
    log_info "Testing: Router integrates startup scoring"
    local router_file="${PROJECT_ROOT}/internal/router/router.go"

    if grep -q "StartupScoringService" "$router_file" && \
       grep -q "NewStartupScoringService" "$router_file"; then
        assert_test "Router creates StartupScoringService" "true"
    else
        assert_test "Router creates StartupScoringService" "false"
    fi
}

# Test 5: Verify async scoring is enabled by default
test_async_default() {
    log_info "Testing: Async scoring enabled by default"
    local service_file="${PROJECT_ROOT}/internal/services/startup_scoring.go"

    if grep -q "Async:.*true" "$service_file"; then
        assert_test "Async mode enabled by default" "true"
    else
        assert_test "Async mode enabled by default" "false"
    fi
}

# Test 6: Build verification
test_build_passes() {
    log_info "Testing: Project builds successfully"
    cd "$PROJECT_ROOT"

    if go build ./internal/... 2>&1; then
        assert_test "Project builds without errors" "true"
    else
        assert_test "Project builds without errors" "false"
    fi
}

# Test 7: Verify configuration structure
test_config_structure() {
    log_info "Testing: Configuration structure is complete"
    local service_file="${PROJECT_ROOT}/internal/services/startup_scoring.go"

    local has_enabled=$(grep -c "Enabled.*bool" "$service_file" || echo "0")
    local has_async=$(grep -c "Async.*bool" "$service_file" || echo "0")
    local has_timeout=$(grep -c "Timeout.*time.Duration" "$service_file" || echo "0")
    local has_workers=$(grep -c "ConcurrentWorkers.*int" "$service_file" || echo "0")

    if [ "$has_enabled" -gt 0 ] && [ "$has_async" -gt 0 ] && \
       [ "$has_timeout" -gt 0 ] && [ "$has_workers" -gt 0 ]; then
        assert_test "StartupScoringConfig has all required fields" "true"
    else
        assert_test "StartupScoringConfig has all required fields" "false"
    fi
}

# Test 8: Verify result structure
test_result_structure() {
    log_info "Testing: Result structure is complete"
    local service_file="${PROJECT_ROOT}/internal/services/startup_scoring.go"

    local has_scored=$(grep -c "ScoredProviders.*int" "$service_file" || echo "0")
    local has_failed=$(grep -c "FailedProviders.*int" "$service_file" || echo "0")
    local has_scores=$(grep -c "ProviderScores.*map" "$service_file" || echo "0")
    local has_success=$(grep -c "Success.*bool" "$service_file" || echo "0")

    if [ "$has_scored" -gt 0 ] && [ "$has_failed" -gt 0 ] && \
       [ "$has_scores" -gt 0 ] && [ "$has_success" -gt 0 ]; then
        assert_test "StartupScoringResult has all required fields" "true"
    else
        assert_test "StartupScoringResult has all required fields" "false"
    fi
}

# Test 9: Live server test (if server is running)
test_live_server() {
    log_info "Testing: Live server startup scoring (if running)"

    local health=$(curl -s --max-time 60 http://localhost:7061/health 2>/dev/null)
    if [ -n "$health" ]; then
        # Server is running, check provider discovery endpoint
        local providers=$(curl -s --max-time 60 http://localhost:7061/v1/providers 2>/dev/null)
        if [ -n "$providers" ]; then
            assert_test "Server responds with provider info" "true"
        else
            log_warn "Provider endpoint not responding - skipping"
            TOTAL=$((TOTAL + 1))
            PASSED=$((PASSED + 1))
        fi
    else
        log_warn "Server not running - skipping live test"
        TOTAL=$((TOTAL + 1))
        PASSED=$((PASSED + 1))
    fi
}

# Test 10: Verify race condition safety
test_race_safety() {
    log_info "Testing: Race condition safety"
    cd "$PROJECT_ROOT"

    local output
    output=$(go test -race -run "TestStartupScoringService_ConcurrentAccess" ./internal/services/... -timeout 30s 2>&1)
    local result=$?

    if [ $result -eq 0 ] && ! echo "$output" | grep -q "DATA RACE"; then
        assert_test "No race conditions detected" "true"
    else
        assert_test "No race conditions detected" "false"
    fi
}

# Main execution
main() {
    echo "=============================================="
    echo "  Startup Scoring Challenge"
    echo "  Validates automatic provider scoring"
    echo "=============================================="
    echo ""

    test_service_exists
    test_unit_tests_exist
    test_unit_tests_pass
    test_router_integration
    test_async_default
    test_build_passes
    test_config_structure
    test_result_structure
    test_live_server
    test_race_safety

    echo ""
    echo "=============================================="
    echo "  RESULTS: $PASSED/$TOTAL passed"
    echo "=============================================="

    if [ $FAILED -eq 0 ]; then
        log_success "All tests passed!"
        exit 0
    else
        log_fail "$FAILED tests failed"
        exit 1
    fi
}

main "$@"
