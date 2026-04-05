#!/usr/bin/env bash
set -euo pipefail

# Ensemble Handler Challenge
# Validates that the ensemble handler is registered in the router,
# test files exist, and session/team management endpoints are present.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "ensemble_handler" "$@"

PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HANDLERS_PKG="$PROJECT_ROOT/internal/handlers"
ROUTER_FILE="$PROJECT_ROOT/internal/router/router.go"

record_result() {
    TOTAL=$((TOTAL + 1))
    test_start "$1"
    if [ "$2" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        test_pass
    else
        FAILED=$((FAILED + 1))
        test_fail "$1"
    fi
}

print_header "Ensemble Handler Challenge"
echo ""

# Test 1: ensemble_handler.go exists
if [ -f "$HANDLERS_PKG/ensemble_handler.go" ]; then
    record_result "ensemble_handler.go exists" "PASS"
else
    record_result "ensemble_handler.go exists" "FAIL"
fi

# Test 2: ensemble_handler registered in router
if grep -q "EnsembleHandler\|ensembleHandler" "$ROUTER_FILE" 2>/dev/null; then
    record_result "EnsembleHandler registered in router.go" "PASS"
else
    record_result "EnsembleHandler registered in router.go" "FAIL"
fi

# Test 3: ensemble_handler_test.go exists
if [ -f "$HANDLERS_PKG/ensemble_handler_test.go" ]; then
    record_result "ensemble_handler_test.go exists" "PASS"
else
    record_result "ensemble_handler_test.go exists" "FAIL"
fi

# Test 4: Handler compiles
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/handlers/... 2>/dev/null; then
    record_result "handlers package compiles" "PASS"
else
    record_result "handlers package compiles" "FAIL"
fi

# Test 5: Tests pass
if GOMAXPROCS=2 nice -n 19 go test -short -count=1 -p 1 -run "Ensemble" ./internal/handlers/... 2>/dev/null; then
    record_result "Ensemble handler tests pass" "PASS"
else
    record_result "Ensemble handler tests pass" "FAIL"
fi

# Test 6: /v1/ensemble routes in router.go
if grep -q '"/ensemble' "$ROUTER_FILE" 2>/dev/null || grep -q "ensemble" "$ROUTER_FILE" 2>/dev/null; then
    record_result "/v1/ensemble routes present in router.go" "PASS"
else
    record_result "/v1/ensemble routes present in router.go" "FAIL"
fi

# Test 7: Session management endpoints exist (CreateSession, ListSessions)
if grep -q "CreateSession\|ListSessions" "$HANDLERS_PKG/ensemble_handler.go" 2>/dev/null; then
    record_result "Session management endpoints (CreateSession, ListSessions) exist" "PASS"
else
    record_result "Session management endpoints (CreateSession, ListSessions) exist" "FAIL"
fi

# Test 8: Session execute/cancel endpoints exist
if grep -q "ExecuteSession\|CancelSession" "$HANDLERS_PKG/ensemble_handler.go" 2>/dev/null; then
    record_result "Session execute/cancel endpoints exist" "PASS"
else
    record_result "Session execute/cancel endpoints exist" "FAIL"
fi

# Test 9: Team management endpoints exist (CreateTeam, ListTeams)
if grep -q "CreateTeam\|ListTeams" "$HANDLERS_PKG/ensemble_handler.go" 2>/dev/null; then
    record_result "Team management endpoints (CreateTeam, ListTeams) exist" "PASS"
else
    record_result "Team management endpoints (CreateTeam, ListTeams) exist" "FAIL"
fi

# Test 10: Team execute endpoint exists
if grep -q "ExecuteTeam" "$HANDLERS_PKG/ensemble_handler.go" 2>/dev/null; then
    record_result "Team execute endpoint (ExecuteTeam) exists" "PASS"
else
    record_result "Team execute endpoint (ExecuteTeam) exists" "FAIL"
fi

# Test 11: RegisterRoutes method exists on EnsembleHandler
if grep -q "func.*EnsembleHandler.*RegisterRoutes\|func (h \*EnsembleHandler) RegisterRoutes" "$HANDLERS_PKG/ensemble_handler.go" 2>/dev/null; then
    record_result "EnsembleHandler.RegisterRoutes method exists" "PASS"
else
    record_result "EnsembleHandler.RegisterRoutes method exists" "FAIL"
fi

# Test 12: ensemble_handler_teams_test.go exists
if [ -f "$HANDLERS_PKG/ensemble_handler_teams_test.go" ]; then
    record_result "ensemble_handler_teams_test.go exists" "PASS"
else
    record_result "ensemble_handler_teams_test.go exists" "FAIL"
fi

# Test 13: Team CRUD — UpdateTeam, DeleteTeam endpoints exist
if grep -q "UpdateTeam\|DeleteTeam" "$HANDLERS_PKG/ensemble_handler.go" 2>/dev/null; then
    record_result "Team CRUD endpoints (UpdateTeam, DeleteTeam) exist" "PASS"
else
    record_result "Team CRUD endpoints (UpdateTeam, DeleteTeam) exist" "FAIL"
fi

# Test 14: AddAgentToTeam endpoint exists
if grep -q "AddAgentToTeam\|RemoveAgentFromTeam" "$HANDLERS_PKG/ensemble_handler.go" 2>/dev/null; then
    record_result "Agent membership endpoints (AddAgentToTeam, RemoveAgentFromTeam) exist" "PASS"
else
    record_result "Agent membership endpoints (AddAgentToTeam, RemoveAgentFromTeam) exist" "FAIL"
fi

# Test 15: GetSession and GetTeam endpoints exist
if grep -q "GetSession\|GetTeam" "$HANDLERS_PKG/ensemble_handler.go" 2>/dev/null; then
    record_result "GetSession and GetTeam endpoints exist" "PASS"
else
    record_result "GetSession and GetTeam endpoints exist" "FAIL"
fi

echo ""
print_summary "Ensemble Handler Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
