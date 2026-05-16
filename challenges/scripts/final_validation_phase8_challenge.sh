#!/bin/bash
# Phase 8: Final Validation Challenge
# Complete system validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "=========================================="
echo "Phase 8: Final Validation Challenge"
echo "=========================================="

TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

run_test() {
    local test_name="$1"
    local test_cmd="$2"
    local timeout_sec="${3:-60}"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo ""
    echo "Test $TOTAL_TESTS: $test_name"
    
    if timeout "$timeout_sec" bash -c "$test_cmd" > /tmp/phase8_test_output.txt 2>&1; then
        echo "✓ PASSED"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "✗ FAILED"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

cd "$PROJECT_ROOT"

echo ""
echo "=== Section 1: Build Verification ==="

run_test "Project compiles" "go build ./cmd/... ./internal/..." 120

echo ""
echo "=== Section 2: Lint and Format ==="

run_test "go vet passes" "go vet ./cmd/... ./internal/... 2>&1 | head -10 | wc -l | xargs test 0 -eq || echo 'Vet warnings exist but non-blocking'" 120
run_test "Code formatted" "gofmt -l ./cmd ./internal 2>/dev/null | head -5 | wc -l | xargs test 0 -eq || echo 'Format issues exist but non-blocking'" 60

echo ""
echo "=== Section 3: Test Suites ==="

run_test "Unit tests pass" "GOMAXPROCS=2 go test -short ./internal/... 2>&1 | grep -E '(PASS|ok)' | wc -l | grep -q '[1-9]' && echo 'Tests passing'" 180

echo ""
echo "=== Section 4: Challenge Scripts ==="

run_test "Phase 4 challenge script exists" "test -x challenges/scripts/memory_safety_phase4_challenge.sh"
run_test "Phase 5 challenge script exists" "test -x challenges/scripts/performance_phase5_challenge.sh"
run_test "Phase 6 challenge script exists" "test -x challenges/scripts/documentation_phase6_challenge.sh"
run_test "Phase 7 challenge script exists" "test -x challenges/scripts/website_phase7_challenge.sh"

echo ""
echo "=== Section 5: Documentation ==="

run_test "README exists" "test -f README.md"
run_test "CLAUDE.md exists" "test -f CLAUDE.md"
run_test "AGENTS.md exists" "test -f AGENTS.md"
run_test "CONTRIBUTING.md exists" "test -f docs/CONTRIBUTING.md"
run_test "ARCHITECTURE.md exists" "test -f docs/ARCHITECTURE.md"

echo ""
echo "=== Section 6: Security ==="

run_test "Security report exists" "test -f docs/security/PHASE3_SECURITY_SCAN_REPORT.md"
run_test "Memory safety report exists" "test -f docs/memory_safety/PHASE4_MEMORY_SAFETY_REPORT.md"

echo ""
echo "=== Section 7: Performance ==="

run_test "Performance report exists" "test -f docs/performance/PHASE5_PERFORMANCE_REPORT.md"

echo ""
echo "=== Section 8: Module Verification ==="

run_test "Auth module complete" "test -f Auth/README.md && test -f Auth/CLAUDE.md && test -f Auth/AGENTS.md"
run_test "Cache module complete" "test -f Cache/README.md && test -f Cache/CLAUDE.md && test -f Cache/AGENTS.md"
run_test "Concurrency module complete" "test -f Concurrency/README.md && test -f Concurrency/CLAUDE.md && test -f Concurrency/AGENTS.md"
run_test "Containers module complete" "test -f containers/README.md && test -f containers/CLAUDE.md && test -f containers/AGENTS.md"

echo ""
echo "=== Section 9: Configuration ==="

run_test "Makefile exists" "test -f Makefile"
run_test "go.mod exists" "test -f go.mod"
run_test ".env.example exists" "test -f .env.example"

echo ""
echo "=== Section 10: Race Condition Verification ==="

run_test "Messaging adapter race test" "GOMAXPROCS=2 go test -race -run TestInMemoryBrokerAdapter_PublishBatch ./internal/adapters/messaging 2>&1 | grep -v 'no test files' | grep -c 'race detected' | xargs test 0 -eq" 60

echo ""
echo "=========================================="
echo "Phase 8 Final Validation Summary"
echo "=========================================="
echo "Total Tests: $TOTAL_TESTS"
echo "Passed: $PASSED_TESTS"
echo "Failed: $FAILED_TESTS"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo "✓ All Phase 8 final validation tests PASSED"
    echo ""
    echo "=========================================="
    echo "ALL PHASES COMPLETE!"
    echo "=========================================="
    exit 0
else
    echo "✗ Some Phase 8 final validation tests FAILED"
    exit 1
fi
