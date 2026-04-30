#!/bin/bash
# Ensemble Session Lifecycle Challenge — CONST-035 anti-bluff regression guard
#
# Validates that POST /v1/ensemble/sessions creates a REAL session
# (status: "creating" or "active", not "created_without_instances")
# AND that GET /v1/ensemble/sessions lists it back. Closes the
# regression guard for #ensemble-instance-manager-wiring.
#
# This Challenge MUST FAIL when:
#   - The InstanceManager wiring is reverted in router.go (back to nil)
#   - The clis.NewInstanceManager nil-db acceptance is reverted
#   - Any of the 9 c.db / m.db nil-guards is removed (re-introduces panic)
#   - GET /v1/ensemble/sessions stops listing in-memory sessions
#
# Verify-by-mutation (CONST-035 §1):
#   - Replace ensembleInstanceMgr with nil in router.go → status reverts
#     to "created_without_instances"
#   - Re-add the "database connection required" hard error in
#     NewInstanceManager → init fails, sessions return
#     created_without_instances

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "ensemble_session_lifecycle" "Ensemble Session Lifecycle Challenge"
load_env

# Test 1: server alive
test_server_alive() {
    if curl -fsS -m 5 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Test 2: session POST returns real instance status (not stub)
test_session_creates_real() {
    log_info "Test 2: POST /v1/ensemble/sessions returns real session status"
    local body='{"strategy":"primary_only","participants":{"primary":{"type":"aider"}}}'
    local resp=$(curl -s -w "\n%{http_code}" -m 30 -X POST "$BASE_URL/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local payload=$(echo "$resp" | head -n -1)

    if [[ "$code" == "201" ]]; then
        record_assertion "session" "post_201" "true" "POST returned 201"
    else
        record_assertion "session" "post_201" "false" "POST returned $code"
        return 1
    fi

    local sid=$(echo "$payload" | jq -r '.id // empty' 2>/dev/null)
    if [[ -n "$sid" && "$sid" != "null" ]]; then
        record_assertion "session" "has_id" "true" "session id=$sid"
        echo "$sid" > /tmp/ensemble_session_id
    else
        record_assertion "session" "has_id" "false" "no session id"
        return 1
    fi

    local status=$(echo "$payload" | jq -r '.status // empty' 2>/dev/null)
    if [[ "$status" == "creating" || "$status" == "created" || "$status" == "active" ]]; then
        record_assertion "session" "real_status" "true" "status=$status (real instance, not stub)"
    elif [[ "$status" == "created_without_instances" ]]; then
        record_assertion "session" "real_status" "false" "status='created_without_instances' — INSTANCE-MANAGER NOT WIRED (regression to commit before 5b634b35)"
        return 1
    else
        record_assertion "session" "real_status" "false" "Unexpected status='$status'"
    fi
}

# Test 3: GET /v1/ensemble/sessions lists the new session
test_list_includes_session() {
    log_info "Test 3: GET /v1/ensemble/sessions includes new session"
    local sid=$(cat /tmp/ensemble_session_id 2>/dev/null || echo "")
    if [[ -z "$sid" ]]; then
        record_assertion "session" "list_skip" "false" "No session id from create step"
        return 1
    fi
    local resp=$(curl -fsS -m 5 "$BASE_URL/v1/ensemble/sessions" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo '[]')
    local found=$(echo "$resp" | jq -r --arg id "$sid" '.[] | select(.id == $id) | .id' 2>/dev/null | head -1)
    if [[ "$found" == "$sid" ]]; then
        record_assertion "session" "list_found" "true" "Session $sid present in list"
    else
        record_assertion "session" "list_found" "false" "Session not found in list"
    fi
}

# Test 4: invalid agent type → 4xx (negative)
test_invalid_agent_type() {
    log_info "Test 4: invalid agent type returns 4xx (not 500 panic)"
    local body='{"strategy":"primary_only","participants":{"primary":{"type":"definitely-not-an-agent-type"}}}'
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 10 -X POST "$BASE_URL/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo "000")
    # Either 4xx (good) or 500 with body (acceptable — error wrapped properly).
    # The bug is 500 with EMPTY body indicating a panic.
    if [[ "$code" =~ ^4[0-9][0-9]$ ]]; then
        record_assertion "session" "invalid_agent_4xx" "true" "Invalid agent type → $code (correct)"
    elif [[ "$code" == "500" ]]; then
        # Check that the body is non-empty (i.e., real error, not panic)
        local body=$(curl -s -m 5 -X POST "$BASE_URL/v1/ensemble/sessions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"strategy":"primary_only","participants":{"primary":{"type":"definitely-not-an-agent-type"}}}' 2>/dev/null || echo "")
        if [[ -n "$body" ]]; then
            record_assertion "session" "invalid_agent_4xx" "true" "Invalid agent type → 500 with error body (acceptable: '$body')"
        else
            record_assertion "session" "invalid_agent_4xx" "false" "Invalid agent type → 500 with EMPTY body — NIL-DEREF PANIC"
        fi
    else
        record_assertion "session" "invalid_agent_4xx" "false" "Invalid agent type → $code"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_session_creates_real || true
    test_list_includes_session || true
    test_invalid_agent_type || true

    rm -f /tmp/ensemble_session_id

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
