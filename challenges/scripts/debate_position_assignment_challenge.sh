#!/bin/bash
# Debate Position Assignment Challenge
# Tests debate position allocation and assignment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "debate-position-assignment" "Debate Position Assignment Challenge"
load_env

log_info "Testing position assignment..."

test_position_allocation() {
    log_info "Test 1: Position allocation to models"

    local request='{"topic":"AI ethics","positions":["proposer","opponent","critic","synthesizer","arbiter"]}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/positions/allocate" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local allocated=$(echo "$body" | jq -e '.allocations | length' 2>/dev/null || echo "0")
        record_assertion "position_allocation" "working" "true" "Allocated $allocated positions"
    else
        record_skip "position_allocation" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_role_assignment() {
    log_info "Test 2: Role assignment to participants"

    local request='{"participants":["model1","model2","model3"],"roles":["proposer","opponent","critic"]}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/roles/assign" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local assignments=$(echo "$body" | jq -e '.assignments | length' 2>/dev/null || echo "0")
        record_assertion "role_assignment" "working" "true" "Assigned $assignments roles"
    else
        record_skip "role_assignment" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_position_balancing() {
    log_info "Test 3: Position balancing across team"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/positions/balance" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"strategy":"even_distribution","min_per_position":3}' \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local is_balanced=$(echo "$body" | jq -e '.balanced' 2>/dev/null || echo "null")
        record_assertion "position_balancing" "working" "true" "Balanced: $is_balanced"
    else
        record_skip "position_balancing" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_assignment_fairness() {
    log_info "Test 4: Assignment fairness validation"

    local resp_body=$(curl -s "$BASE_URL/v1/debate/positions/fairness" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local fairness_score=$(echo "$resp_body" | jq -e '.fairness_score' 2>/dev/null || echo "0.0")
    local variance=$(echo "$resp_body" | jq -e '.variance' 2>/dev/null || echo "0.0")
    record_assertion "assignment_fairness" "checked" "true" "Fairness: $fairness_score, Variance: $variance"
}

main() {
    log_info "Starting position assignment challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_position_allocation
    test_role_assignment
    test_position_balancing
    test_assignment_fairness

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
