#!/bin/bash
# Silent-200 on Bogus ID Challenge — CONST-035 anti-bluff regression guard
#
# Validates that GET-by-id endpoints return 404 (not silent 200) when the
# id doesn't exist. Closes the structural-bluff bug fixed in commit
# 7e245080 where /v1/tasks/:id/logs, /v1/tasks/:id/resources, and
# /v1/health/provider/:id/available returned 200 with empty/default
# content for ANY id, including ones that had never been registered.
#
# This Challenge MUST FAIL when:
#   - any of the listed handlers reverts to skipping the existence check
#   - any documented GET-by-id endpoint returns 200 for a guaranteed
#     bogus id
#
# Verify-by-mutation (CONST-035 §1):
#   - Remove the GetByID early-return in GetTaskLogs handler →
#     tasks_logs_404 fails
#   - Remove the GetCircuitBreaker check in IsProviderAvailable →
#     provider_available_404 fails

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "silent_200_on_bogus_id" "Silent-200 on Bogus ID Challenge"
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

# Helper: assert GET endpoint returns 404 (not 200) for bogus ID
assert_404_for_bogus() {
    local label="$1"
    local target="$2"
    local path="$3"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 "$BASE_URL$path" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$code" == "404" ]]; then
        record_assertion "$label" "$target" "true" "$path → 404 (correct, was silent 200)"
    elif [[ "$code" == "200" ]]; then
        record_assertion "$label" "$target" "false" "$path → 200 — STRUCTURAL BLUFF (caller can't tell exists vs not)"
    else
        record_assertion "$label" "$target" "false" "$path → $code (expected 404)"
    fi
}

# Test 2: tasks/logs/resources endpoints — bogus id MUST 404
test_tasks_bogus_id_404() {
    log_info "Test 2: /v1/tasks/<bogus> family returns 404"
    assert_404_for_bogus "tasks" "logs_404"      "/v1/tasks/zzz-bogus-task-id/logs"
    assert_404_for_bogus "tasks" "resources_404" "/v1/tasks/zzz-bogus-task-id/resources"
    # Also verify the previously-correct ones still return 404
    assert_404_for_bogus "tasks" "status_404"    "/v1/tasks/zzz-bogus-task-id/status"
    assert_404_for_bogus "tasks" "events_404"    "/v1/tasks/zzz-bogus-task-id/events"
    assert_404_for_bogus "tasks" "detail_404"    "/v1/tasks/zzz-bogus-task-id"
}

# Test 3: health/provider/:id/available bogus → 404
test_provider_available_404() {
    log_info "Test 3: /v1/health/provider/<bogus>/available returns 404"
    assert_404_for_bogus "health" "available_404" "/v1/health/provider/zzz-bogus-provider/available"
}

# Test 4: real provider /available still returns 200 (positive control)
test_real_provider_available_200() {
    log_info "Test 4: /v1/health/provider/cerebras/available still returns 200 (positive control)"
    local code=$(curl -s -o /tmp/avail.json -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/health/provider/cerebras/available" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        record_assertion "health" "real_provider_200" "true" "Real provider returns 200 (not 404)"
        local prov=$(jq -r '.provider_id // empty' /tmp/avail.json 2>/dev/null)
        if [[ "$prov" == "cerebras" ]]; then
            record_assertion "health" "real_provider_id_matches" "true" "provider_id=cerebras"
        else
            record_assertion "health" "real_provider_id_matches" "false" "provider_id='$prov'"
        fi
    else
        record_assertion "health" "real_provider_200" "false" "Real provider returned $code (expected 200)"
    fi
    rm -f /tmp/avail.json
}

# Test 5: real task → 200 with logs (positive control)
test_real_task_200() {
    log_info "Test 5: real task created via POST returns 200 from /logs (positive control)"
    local body='{"task_type":"verification","task_name":"silent-200-probe","priority":"normal"}'
    local task_id=$(curl -s -X POST "$BASE_URL/v1/tasks" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" | jq -r '.id // empty')

    if [[ -z "$task_id" ]]; then
        record_assertion "real_task" "created" "false" "Could not create task for positive control"
        return 1
    fi

    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/tasks/$task_id/logs" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        record_assertion "real_task" "logs_200" "true" "Real task /logs returns 200 (not 404)"
    else
        record_assertion "real_task" "logs_200" "false" "Real task returned $code"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_tasks_bogus_id_404 || true
    test_provider_available_404 || true
    test_real_provider_available_200 || true
    test_real_task_200 || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
