#!/bin/bash
# /v1/tasks Round-Trip Challenge — CONST-035 anti-bluff regression guard
#
# Validates the contract that POST /v1/tasks creates a real task and that
# every catalog endpoint reflects it. Was a contract-bluff site before
# 2026-04-30 commit 04a2999e: POST returned 202 with task ID and
# status="pending", but GET /v1/tasks listed zero tasks because the queue
# and repository were two separate stores with no write-through.
#
# This Challenge MUST FAIL when:
#   - the POST handler stops persisting to repository (regression)
#   - the InMemoryTaskRepository's GetByStatus returns empty for "" (regression)
#   - the route registration is removed
#
# Verify-by-mutation (CONST-035 §1):
#   1. Comment out the `repository.Create()` call in CreateTask.
#   2. Re-run this Challenge.
#   3. The "list_after_create" assertion MUST fail.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "tasks_roundtrip" "Tasks Round-Trip Challenge"
load_env

log_info "Testing /v1/tasks POST → GET round-trip..."

# Test 1: Health check — server is up
test_server_alive() {
    log_info "Test 1: server is up at $BASE_URL"
    if curl -fsS -m 5 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding to /v1/health"
    else
        record_assertion "server" "alive" "false" "Server not reachable at $BASE_URL"
        return 1
    fi
}

# Test 2: POST creates a task and returns 202 with id+status
test_create_task() {
    log_info "Test 2: POST /v1/tasks creates a task"

    local body='{"task_type":"verification","task_name":"roundtrip-probe","priority":"normal"}'
    local resp=$(curl -s -w "\n%{http_code}" -m 10 -X POST "$BASE_URL/v1/tasks" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local payload=$(echo "$resp" | head -n -1)

    if [[ "$code" == "202" ]]; then
        record_assertion "create" "status_202" "true" "POST returned 202 Accepted"
    else
        record_assertion "create" "status_202" "false" "POST returned $code (expected 202)"
        return 1
    fi

    local task_id=$(echo "$payload" | jq -r '.id // empty' 2>/dev/null)
    if [[ -n "$task_id" && "$task_id" != "null" ]]; then
        record_assertion "create" "has_id" "true" "Response carries task ID: $task_id"
        echo "$task_id" > /tmp/tasks_roundtrip_id
    else
        record_assertion "create" "has_id" "false" "Response missing task ID"
        return 1
    fi

    local status=$(echo "$payload" | jq -r '.status // empty' 2>/dev/null)
    if [[ "$status" == "pending" ]]; then
        record_assertion "create" "status_pending" "true" "New task is in pending status"
    else
        record_assertion "create" "status_pending" "false" "Status='$status' (expected 'pending')"
    fi
}

# Test 3: GET /v1/tasks lists the new task — the contract-bluff site
test_list_after_create() {
    log_info "Test 3: GET /v1/tasks lists the task we just created"

    local task_id=$(cat /tmp/tasks_roundtrip_id 2>/dev/null || echo "")
    if [[ -z "$task_id" ]]; then
        record_assertion "list_after_create" "task_id_present" "false" "No task_id from create step"
        return 1
    fi

    local resp=$(curl -fsS -m 5 "$BASE_URL/v1/tasks" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo '{}')

    local count=$(echo "$resp" | jq -r '.count // 0' 2>/dev/null)
    if [[ "$count" -ge 1 ]]; then
        record_assertion "list_after_create" "non_empty" "true" "List has $count tasks (expected ≥1)"
    else
        record_assertion "list_after_create" "non_empty" "false" "List is empty after create (count=$count) — CONTRACT BLUFF"
        return 1
    fi

    # Verify our specific task is in the list
    local found=$(echo "$resp" | jq -r --arg id "$task_id" '.tasks[] | select(.id == $id) | .id' 2>/dev/null)
    if [[ "$found" == "$task_id" ]]; then
        record_assertion "list_after_create" "found_by_id" "true" "Created task present in list"
    else
        record_assertion "list_after_create" "found_by_id" "false" "Created task $task_id not found in list"
    fi
}

# Test 4: GET /v1/tasks/:id returns the task by ID
test_get_by_id() {
    log_info "Test 4: GET /v1/tasks/:id returns task by ID"

    local task_id=$(cat /tmp/tasks_roundtrip_id 2>/dev/null || echo "")
    if [[ -z "$task_id" ]]; then
        record_assertion "get_by_id" "task_id_present" "false" "No task_id from create step"
        return 1
    fi

    local resp=$(curl -s -w "\n%{http_code}" -m 5 "$BASE_URL/v1/tasks/$task_id" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        record_assertion "get_by_id" "status_200" "true" "GET by ID returned 200"
    else
        record_assertion "get_by_id" "status_200" "false" "GET by ID returned $code"
        return 1
    fi

    local got_id=$(echo "$body" | jq -r '.id // empty' 2>/dev/null)
    if [[ "$got_id" == "$task_id" ]]; then
        record_assertion "get_by_id" "id_matches" "true" "Returned task ID matches"
    else
        record_assertion "get_by_id" "id_matches" "false" "Got id='$got_id', expected '$task_id'"
    fi

    local task_name=$(echo "$body" | jq -r '.task_name // empty' 2>/dev/null)
    if [[ "$task_name" == "roundtrip-probe" ]]; then
        record_assertion "get_by_id" "task_name_matches" "true" "Task name preserved"
    else
        record_assertion "get_by_id" "task_name_matches" "false" "Got task_name='$task_name'"
    fi
}

# Test 5: GET /v1/tasks/queue/stats reflects the new task
test_queue_stats() {
    log_info "Test 5: GET /v1/tasks/queue/stats reflects the new task"

    local resp=$(curl -fsS -m 5 "$BASE_URL/v1/tasks/queue/stats" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo '{}')

    local pending=$(echo "$resp" | jq -r '.pending_count // 0' 2>/dev/null)
    if [[ "$pending" -ge 1 ]]; then
        record_assertion "queue_stats" "pending_count_ge_1" "true" "pending_count=$pending (expected ≥1)"
    else
        record_assertion "queue_stats" "pending_count_ge_1" "false" "pending_count=$pending (expected ≥1)"
    fi
}

# Test 6: Negative — bogus id returns 404
test_bogus_id_404() {
    log_info "Test 6: GET /v1/tasks/<bogus> returns 404"

    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/tasks/zzzzz-not-a-real-task" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")

    if [[ "$code" == "404" ]]; then
        record_assertion "bogus_id" "returns_404" "true" "Bogus ID correctly returns 404"
    else
        record_assertion "bogus_id" "returns_404" "false" "Bogus ID returned $code (expected 404)"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_create_task
    test_list_after_create
    test_get_by_id
    test_queue_stats
    test_bogus_id_404

    rm -f /tmp/tasks_roundtrip_id

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
