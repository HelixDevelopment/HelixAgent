#!/bin/bash
# Container Runtime Compatibility Tests
# Tests for Docker/Podman compatibility
#
# Run: ./tests/container/container_runtime_test.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Test helper functions
test_pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    ((TESTS_PASSED++))
}

test_fail() {
    echo -e "  ${RED}✗${NC} $1"
    ((TESTS_FAILED++))
}

test_skip() {
    echo -e "  ${YELLOW}○${NC} $1 (skipped)"
    ((TESTS_SKIPPED++))
}

echo -e "${BLUE}=== Container Runtime Compatibility Tests ===${NC}"
echo ""

# Source the container runtime script
source "$PROJECT_ROOT/scripts/container-runtime.sh" 2>/dev/null || {
    echo -e "${RED}Failed to source container-runtime.sh${NC}"
    exit 1
}

# Test 1: Container runtime detection
echo -e "${BLUE}Test: Container Runtime Detection${NC}"
if [ -n "$CONTAINER_RUNTIME" ] && [ "$CONTAINER_RUNTIME" != "none" ]; then
    test_pass "Detected container runtime: $CONTAINER_RUNTIME"
else
    test_fail "No container runtime detected"
fi

# Test 2: Container command exists
echo -e "${BLUE}Test: Container Command Availability${NC}"
if [ -n "$CONTAINER_CMD" ] && command -v ${CONTAINER_CMD%% *} &> /dev/null; then
    test_pass "Container command available: $CONTAINER_CMD"
else
    test_fail "Container command not available"
fi

# Test 3: Compose command detection
echo -e "${BLUE}Test: Compose Command Detection${NC}"
if [ -n "$COMPOSE_CMD" ]; then
    test_pass "Compose command available: $COMPOSE_CMD"
else
    test_skip "No compose command available"
fi

# Test 4: Container runtime version
echo -e "${BLUE}Test: Container Runtime Version${NC}"
if version=$($CONTAINER_CMD --version 2>/dev/null); then
    test_pass "Version: $version"
else
    test_fail "Could not get runtime version"
fi

# Test 5: Can pull basic image
echo -e "${BLUE}Test: Pull Basic Image${NC}"
if $CONTAINER_CMD pull alpine:latest &> /dev/null; then
    test_pass "Successfully pulled alpine:latest"
else
    test_fail "Failed to pull alpine:latest"
fi

# Test 6: Can run container
echo -e "${BLUE}Test: Run Container${NC}"
if output=$($CONTAINER_CMD run --rm alpine:latest echo "test" 2>/dev/null); then
    if [ "$output" = "test" ]; then
        test_pass "Successfully ran container with output"
    else
        test_fail "Container ran but output mismatch"
    fi
else
    test_fail "Failed to run container"
fi

# Test 7: Dockerfile syntax validation
echo -e "${BLUE}Test: Dockerfile Syntax${NC}"
if [ -f "$PROJECT_ROOT/Dockerfile" ]; then
    # Check for common Dockerfile keywords
    if grep -q "^FROM" "$PROJECT_ROOT/Dockerfile" && \
       grep -q "^WORKDIR\|^RUN\|^CMD\|^ENTRYPOINT" "$PROJECT_ROOT/Dockerfile"; then
        test_pass "Dockerfile has valid structure"
    else
        test_fail "Dockerfile missing required keywords"
    fi
else
    test_fail "Dockerfile not found"
fi

# Test 8: docker-compose.yml syntax validation
echo -e "${BLUE}Test: docker-compose.yml Syntax${NC}"
if [ -f "$PROJECT_ROOT/docker-compose.yml" ]; then
    # Check for compose file structure
    if grep -q "^version:" "$PROJECT_ROOT/docker-compose.yml" || \
       grep -q "^services:" "$PROJECT_ROOT/docker-compose.yml"; then
        test_pass "docker-compose.yml has valid structure"
    else
        test_fail "docker-compose.yml missing required sections"
    fi
else
    test_fail "docker-compose.yml not found"
fi

# Test 9: Compose config validation (if compose available)
echo -e "${BLUE}Test: Compose Configuration Validation${NC}"
if [ -n "$COMPOSE_CMD" ]; then
    if $COMPOSE_CMD -f "$PROJECT_ROOT/docker-compose.yml" config &> /dev/null; then
        test_pass "docker-compose.yml is valid"
    else
        test_fail "docker-compose.yml validation failed"
    fi
else
    test_skip "Compose not available"
fi

# Test 10: Build image (dry-run style check)
echo -e "${BLUE}Test: Dockerfile Build Preparation${NC}"
if [ -f "$PROJECT_ROOT/Dockerfile" ] && [ -f "$PROJECT_ROOT/go.mod" ]; then
    test_pass "Build prerequisites available"
else
    test_fail "Missing build prerequisites"
fi

# Test 11: Network creation capability
echo -e "${BLUE}Test: Network Creation${NC}"
network_name="superagent-test-network-$$"
if $CONTAINER_CMD network create "$network_name" &> /dev/null; then
    test_pass "Can create network"
    $CONTAINER_CMD network rm "$network_name" &> /dev/null || true
else
    test_fail "Cannot create network"
fi

# Test 12: Volume creation capability
echo -e "${BLUE}Test: Volume Creation${NC}"
volume_name="superagent-test-volume-$$"
if $CONTAINER_CMD volume create "$volume_name" &> /dev/null; then
    test_pass "Can create volume"
    $CONTAINER_CMD volume rm "$volume_name" &> /dev/null || true
else
    test_fail "Cannot create volume"
fi

# Test 13: Port binding capability
echo -e "${BLUE}Test: Port Binding${NC}"
# Try to run a container with port mapping
if $CONTAINER_CMD run --rm -d --name test-port-$$ -p 18765:80 alpine:latest sleep 5 &> /dev/null; then
    test_pass "Can bind ports"
    $CONTAINER_CMD stop test-port-$$ &> /dev/null || true
    $CONTAINER_CMD rm -f test-port-$$ &> /dev/null || true
else
    test_fail "Cannot bind ports"
fi

# Test 14: Environment variable passing
echo -e "${BLUE}Test: Environment Variables${NC}"
if output=$($CONTAINER_CMD run --rm -e TEST_VAR=hello alpine:latest sh -c 'echo $TEST_VAR' 2>/dev/null); then
    if [ "$output" = "hello" ]; then
        test_pass "Environment variables work correctly"
    else
        test_fail "Environment variable value mismatch"
    fi
else
    test_fail "Cannot pass environment variables"
fi

# Test 15: Volume mount capability
echo -e "${BLUE}Test: Volume Mounting${NC}"
temp_dir=$(mktemp -d)
echo "test-content" > "$temp_dir/test.txt"
if output=$($CONTAINER_CMD run --rm -v "$temp_dir:/mnt" alpine:latest cat /mnt/test.txt 2>/dev/null); then
    if [ "$output" = "test-content" ]; then
        test_pass "Volume mounting works correctly"
    else
        test_fail "Volume content mismatch"
    fi
else
    test_fail "Cannot mount volumes"
fi
rm -rf "$temp_dir"

# Test 16: Health check support
echo -e "${BLUE}Test: Health Check Support${NC}"
if grep -q "HEALTHCHECK" "$PROJECT_ROOT/Dockerfile"; then
    test_pass "Dockerfile contains HEALTHCHECK"
else
    test_fail "Dockerfile missing HEALTHCHECK"
fi

# Test 17: Multi-stage build support
echo -e "${BLUE}Test: Multi-stage Build${NC}"
if grep -c "^FROM" "$PROJECT_ROOT/Dockerfile" | grep -q "^[2-9]"; then
    test_pass "Dockerfile uses multi-stage build"
else
    test_skip "Single-stage build detected"
fi

# Test 18: Non-root user in Dockerfile
echo -e "${BLUE}Test: Non-root User Configuration${NC}"
if grep -q "^USER" "$PROJECT_ROOT/Dockerfile"; then
    test_pass "Dockerfile switches to non-root user"
else
    test_fail "Dockerfile runs as root (security concern)"
fi

# Test 19: Compose profiles support
echo -e "${BLUE}Test: Compose Profiles${NC}"
if grep -q "profiles:" "$PROJECT_ROOT/docker-compose.yml"; then
    test_pass "docker-compose.yml uses profiles"
else
    test_skip "No profiles defined"
fi

# Test 20: Podman-specific tests (if using Podman)
echo -e "${BLUE}Test: Podman-Specific Features${NC}"
if [ "$CONTAINER_RUNTIME" = "podman" ]; then
    # Test podman socket
    if [ -S "/run/user/$(id -u)/podman/podman.sock" ]; then
        test_pass "Podman socket available for Docker compatibility"
    else
        test_skip "Podman socket not found"
    fi

    # Test rootless mode
    if podman info 2>/dev/null | grep -q "rootless: true"; then
        test_pass "Running in rootless mode (recommended)"
    else
        test_skip "Running in rootful mode"
    fi
else
    test_skip "Not using Podman"
fi

# Summary
echo ""
echo -e "${BLUE}=== Test Summary ===${NC}"
echo -e "  ${GREEN}Passed:${NC}  $TESTS_PASSED"
echo -e "  ${RED}Failed:${NC}  $TESTS_FAILED"
echo -e "  ${YELLOW}Skipped:${NC} $TESTS_SKIPPED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Please check the output above.${NC}"
    exit 1
fi
