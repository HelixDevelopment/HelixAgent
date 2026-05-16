#!/bin/bash
# Remote Distribution Precedence Challenge

set -e
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent

echo "========================================"
echo "Remote Distribution Precedence Challenge"
echo "========================================"
echo ""

PASSED=0
FAILED=0

# Test 1
echo "Test 1: Global remote enabled ignores individual SVC_*_REMOTE=false..."
export CONTAINERS_REMOTE_ENABLED=true
export SVC_POSTGRESQL_REMOTE=false
export SVC_REDIS_REMOTE=false
if go test ./internal/config/ -run "TestContainersRemoteEnabledPrecedence/Global_remote_enabled_ignores_individual_false" > /dev/null 2>&1; then
    echo "  ✅ PASS"
    PASSED=$((PASSED + 1))
else
    echo "  ❌ FAIL"
    FAILED=$((FAILED + 1))
fi

# Test 2
echo "Test 2: LoadServicesFromEnv preserves Remote=true..."
if go test ./internal/config/ -run "TestLoadServicesFromEnvPreservesRemote" > /dev/null 2>&1; then
    echo "  ✅ PASS"
    PASSED=$((PASSED + 1))
else
    echo "  ❌ FAIL"
    FAILED=$((FAILED + 1))
fi

# Test 3
echo "Test 3: All config tests pass..."
if go test ./internal/config/ > /dev/null 2>&1; then
    echo "  ✅ PASS"
    PASSED=$((PASSED + 1))
else
    echo "  ❌ FAIL"
    FAILED=$((FAILED + 1))
fi

# Test 4
echo "Test 4: containers/.env has correct setting..."
if grep -q "CONTAINERS_REMOTE_ENABLED=true" containers/.env; then
    echo "  ✅ PASS"
    PASSED=$((PASSED + 1))
else
    echo "  ❌ FAIL"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "Results: $PASSED/4 passed"
[ $FAILED -eq 0 ] && echo "✅ ALL TESTS PASSED!" && exit 0 || echo "❌ FAILED" && exit 1
