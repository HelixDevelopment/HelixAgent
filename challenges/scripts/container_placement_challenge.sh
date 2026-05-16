#!/bin/bash
# Container Placement Challenge - Verifies containers are on correct host

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CONTAINERS_ENV="$PROJECT_ROOT/containers/.env"
TOTAL=0
PASSED=0
FAILED=0

pass() { echo -e "${GREEN}✓ PASS${NC}: $1"; PASSED=$((PASSED + 1)); TOTAL=$((TOTAL + 1)); }
fail() { echo -e "${RED}✗ FAIL${NC}: $1"; FAILED=$((FAILED + 1)); TOTAL=$((TOTAL + 1)); }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

echo "══════════════════════════════════════════════════════════════════"
echo "       CONTAINER PLACEMENT CHALLENGE"
echo "══════════════════════════════════════════════════════════════════"

# Parse configuration
REMOTE_ENABLED=false
REMOTE_HOST=""

if [[ -f "$CONTAINERS_ENV" ]]; then
    while IFS='=' read -r k v; do
        [[ "$k" =~ ^#.*$ || -z "$k" ]] && continue
        v="${v%\"}"; v="${v#\"}"
        case "$k" in
            CONTAINERS_REMOTE_ENABLED) [[ "${v,,}" == "true" ]] && REMOTE_ENABLED=true ;;
            CONTAINERS_REMOTE_HOST_*_ADDRESS) REMOTE_HOST="$v" ;;
        esac
    done < "$CONTAINERS_ENV"
fi

info "Remote enabled: $REMOTE_ENABLED"
info "Remote host: ${REMOTE_HOST:-N/A}"

# Count containers
LOCAL_COUNT=$(podman ps --format "{{.Names}}" 2>/dev/null | grep "helixagent" | wc -l | tr -d ' ')

REMOTE_COUNT=0
if [[ -n "$REMOTE_HOST" ]] && command -v ssh &>/dev/null; then
    REMOTE_COUNT=$(ssh -o ConnectTimeout=5 -o BatchMode=yes "$REMOTE_HOST" \
        "podman ps --format '{{.Names}}' | grep helixagent | wc -l" 2>/dev/null | tr -d ' ' || echo "0")
fi

info "Local containers: $LOCAL_COUNT"
info "Remote containers: $REMOTE_COUNT"

# Tests
echo ""
info "=== Tests ==="

# Test 1: Config file exists
if [[ -f "$CONTAINERS_ENV" ]]; then
    pass "containers/.env exists"
else
    fail "containers/.env NOT found"
fi

# Test 2: Deployment script exists
if [[ -f "$PROJECT_ROOT/scripts/deploy-containers.sh" ]]; then
    pass "deploy-containers.sh exists"
else
    fail "deploy-contements.sh NOT found"
fi

# Test 3: Makefile uses deploy script
if grep -q "deploy-containers.sh" "$PROJECT_ROOT/Makefile" 2>/dev/null; then
    pass "Makefile uses deploy-containers.sh"
else
    fail "Makefile does NOT use deploy-containers.sh"
fi

# Test 4: Container placement matches config
if [[ "$REMOTE_ENABLED" == "true" ]]; then
    if [[ "$LOCAL_COUNT" == "0" ]] && [[ "$REMOTE_COUNT" -gt 0 ]]; then
        pass "REMOTE mode: $REMOTE_COUNT containers on remote, 0 locally"
    elif [[ "$LOCAL_COUNT" -gt 0 ]]; then
        fail "REMOTE mode: $LOCAL_COUNT containers running LOCALLY (should be 0)"
    else
        fail "REMOTE mode: no containers found anywhere"
    fi
else
    if [[ "$LOCAL_COUNT" -gt 0 ]]; then
        pass "LOCAL mode: $LOCAL_COUNT containers running locally"
    else
        fail "LOCAL mode: no containers found"
    fi
fi

# Test 5: No duplicate containers
if [[ "$LOCAL_COUNT" -gt 0 ]] && [[ "$REMOTE_COUNT" -gt 0 ]]; then
    fail "DUPLICATE: containers on both local AND remote"
else
    pass "No duplicate container instances"
fi

# Summary
echo ""
echo "══════════════════════════════════════════════════════════════════"
echo "SUMMARY: $PASSED/$TOTAL passed, $FAILED failed"
echo "══════════════════════════════════════════════════════════════════"

if [[ $FAILED -eq 0 ]]; then
    echo -e "${GREEN}ALL TESTS PASSED${NC}"
    exit 0
else
    echo -e "${RED}CHALLENGE FAILED${NC}"
    exit 1
fi
