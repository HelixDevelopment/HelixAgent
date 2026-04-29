#!/bin/bash
# Debate Condorcet Voting Challenge
# Validates advanced voting methods: Condorcet, Plurality, Unanimous,
# CondorcetMatrix, and AutoSelectMethod.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-condorcet-voting" "Debate Condorcet Voting Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Condorcet Voting Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: weighted_voting.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/voting/weighted_voting.go" ]; then
    record_assertion "voting_file" "exists" "true" "weighted_voting.go exists"
else
    record_assertion "voting_file" "exists" "false" "weighted_voting.go NOT found"
fi

log_info "Test 2: Voting package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/debate/voting/... 2>&1); then
    record_assertion "voting_compile" "true" "true" "Voting package compiles"
else
    record_assertion "voting_compile" "true" "false" "Voting package failed to compile"
fi

# --- Section 2: Voting methods ---

log_info "Test 3: CalculateCondorcet method exists"
if grep -q "func (wvs \*WeightedVotingSystem) CalculateCondorcet" "$PROJECT_ROOT/internal/debate/voting/weighted_voting.go" 2>/dev/null; then
    record_assertion "calculate_condorcet" "true" "true" "CalculateCondorcet method found"
else
    record_assertion "calculate_condorcet" "true" "false" "CalculateCondorcet method NOT found"
fi

log_info "Test 4: CalculatePlurality method exists"
if grep -q "func (wvs \*WeightedVotingSystem) CalculatePlurality" "$PROJECT_ROOT/internal/debate/voting/weighted_voting.go" 2>/dev/null; then
    record_assertion "calculate_plurality" "true" "true" "CalculatePlurality method found"
else
    record_assertion "calculate_plurality" "true" "false" "CalculatePlurality method NOT found"
fi

log_info "Test 5: CalculateUnanimous method exists"
if grep -q "func (wvs \*WeightedVotingSystem) CalculateUnanimous" "$PROJECT_ROOT/internal/debate/voting/weighted_voting.go" 2>/dev/null; then
    record_assertion "calculate_unanimous" "true" "true" "CalculateUnanimous method found"
else
    record_assertion "calculate_unanimous" "true" "false" "CalculateUnanimous method NOT found"
fi

log_info "Test 6: CondorcetMatrix type exists"
if grep -q "type CondorcetMatrix struct" "$PROJECT_ROOT/internal/debate/voting/weighted_voting.go" 2>/dev/null; then
    record_assertion "condorcet_matrix" "true" "true" "CondorcetMatrix type found"
else
    record_assertion "condorcet_matrix" "true" "false" "CondorcetMatrix type NOT found"
fi

log_info "Test 7: AutoSelectMethod exists"
if grep -q "func (wvs \*WeightedVotingSystem) AutoSelectMethod" "$PROJECT_ROOT/internal/debate/voting/weighted_voting.go" 2>/dev/null; then
    record_assertion "auto_select_method" "true" "true" "AutoSelectMethod found"
else
    record_assertion "auto_select_method" "true" "false" "AutoSelectMethod NOT found"
fi

log_info "Test 8: WeightedVotingSystem type exists"
if grep -q "type WeightedVotingSystem struct" "$PROJECT_ROOT/internal/debate/voting/weighted_voting.go" 2>/dev/null; then
    record_assertion "weighted_voting_system" "true" "true" "WeightedVotingSystem type found"
else
    record_assertion "weighted_voting_system" "true" "false" "WeightedVotingSystem type NOT found"
fi

# --- Section 3: Tests ---

log_info "Test 9: weighted_voting_test.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/voting/weighted_voting_test.go" ]; then
    record_assertion "voting_test_file" "exists" "true" "Test file found"
else
    record_assertion "voting_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 10: Voting tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/voting/ 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "voting_tests_pass" "pass" "true" "Voting tests passed"
else
    record_assertion "voting_tests_pass" "pass" "false" "Voting tests failed"
fi

# --- Finalize ---

FAILURES=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)
if [ "$FAILURES" -eq 0 ]; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
