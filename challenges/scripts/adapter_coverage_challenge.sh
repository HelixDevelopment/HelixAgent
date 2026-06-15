#!/usr/bin/env bash
set -euo pipefail

# Adapter Coverage Challenge
# Validates that all internal/adapters/* packages have adequate test coverage.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "adapter_coverage" "$@"

PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MIN_COVERAGE=50

print_header "Adapter Coverage Challenge"
echo "Validates test coverage for all adapter packages (minimum: ${MIN_COVERAGE}%)"
echo ""

# =============================================================================
# Discover adapter packages
# =============================================================================

ADAPTER_PKGS=()
for dir in "$PROJECT_ROOT"/internal/adapters/*/; do
    [ -d "$dir" ] || continue
    # Check if directory has Go files
    if ls "$dir"*.go > /dev/null 2>&1; then
        pkg_name=$(basename "$dir")
        ADAPTER_PKGS+=("$pkg_name")
    fi
done

if [ ${#ADAPTER_PKGS[@]} -eq 0 ]; then
    log_error "No adapter packages found"
    exit 1
fi

echo "Found ${#ADAPTER_PKGS[@]} adapter packages: ${ADAPTER_PKGS[*]}"
echo ""

# =============================================================================
# Test each adapter package for coverage
# =============================================================================

for pkg in "${ADAPTER_PKGS[@]}"; do
    TOTAL=$((TOTAL + 1))
    test_start "Coverage for internal/adapters/$pkg (>= ${MIN_COVERAGE}%)"

    coverage_output=$(GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -cover -count=1 -timeout=3m \
        "$PROJECT_ROOT/internal/adapters/$pkg/..." 2>&1 || true)

    # Extract coverage percentage
    coverage_pct=$(echo "$coverage_output" | sed -nE 's/.*coverage: ([0-9]+\.[0-9]+).*/\1/p' | head -1); coverage_pct=${coverage_pct:-0}

    if [ -z "$coverage_pct" ]; then
        # Check if package has no test files
        if echo "$coverage_output" | grep -q "no test files"; then
            test_fail "No test files (0% coverage)"
            FAILED=$((FAILED + 1))
            continue
        elif echo "$coverage_output" | grep -q "build failed\|FAIL"; then
            test_fail "Build/test failure"
            FAILED=$((FAILED + 1))
            continue
        fi
        coverage_pct="0"
    fi

    # Compare coverage (integer comparison)
    coverage_int=$(echo "$coverage_pct" | cut -d. -f1)
    if [ "$coverage_int" -ge "$MIN_COVERAGE" ]; then
        test_pass
        echo "    Coverage: ${coverage_pct}%"
        PASSED=$((PASSED + 1))
    else
        test_fail "Coverage ${coverage_pct}% is below ${MIN_COVERAGE}% threshold"
        FAILED=$((FAILED + 1))
    fi
done

# =============================================================================
# Test: At least one adapter package exists
# =============================================================================

TOTAL=$((TOTAL + 1))
test_start "At least one adapter package exists"
if [ ${#ADAPTER_PKGS[@]} -gt 0 ]; then
    test_pass
    PASSED=$((PASSED + 1))
else
    test_fail "No adapter packages found"
    FAILED=$((FAILED + 1))
fi

# =============================================================================
# Test: All adapter packages compile
# =============================================================================

TOTAL=$((TOTAL + 1))
test_start "All adapter packages compile"
compile_failures=0
for pkg in "${ADAPTER_PKGS[@]}"; do
    if ! GOMAXPROCS=2 nice -n 19 ionice -c 3 go build "$PROJECT_ROOT/internal/adapters/$pkg/..." > /dev/null 2>&1; then
        compile_failures=$((compile_failures + 1))
    fi
done

if [ "$compile_failures" -eq 0 ]; then
    test_pass
    PASSED=$((PASSED + 1))
else
    test_fail "$compile_failures packages failed to compile"
    FAILED=$((FAILED + 1))
fi

# =============================================================================
# Summary
# =============================================================================

print_summary "Adapter Coverage Challenge" "$PASSED" "$FAILED"

finalize_challenge "$FAILED"
exit "$FAILED"
