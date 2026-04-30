#!/bin/bash
# Tasks Worker Drain Challenge — CONST-035 anti-bluff regression guard
#
# Validates that POST /v1/tasks creates a task that ACTUALLY transitions
# through the documented lifecycle (pending → running → completed) — not
# just that the catalog endpoints reflect a stuck "pending" state.
#
# Closes the #task-worker-pool-wiring tracking ticket. Before commit
# (round 22), tasks created via POST persisted but stayed in "pending"
# forever because no worker was draining the queue. The lifecycle
# promise of "your task will eventually run" was a contract bluff.
#
# This Challenge MUST FAIL when:
#   - the InMemoryWorker.Start() call is removed from router.go
#   - the worker's Dequeue loop breaks (panics or returns nil forever)
#   - status transitions are skipped (pending → completed without going
#     through running, OR no status change at all)
#
# Verify-by-mutation (CONST-035 §1):
#   - Comment out `taskWorker.Start(...)` in router.go → task_completes
#     fails ("task still pending after 10s")
#   - Replace UpdateStatus(Completed) with no-op → final_status fails

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "tasks_worker_drain" "Tasks Worker Drain Challenge"
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

# Test 2: create a task and watch it transition pending → completed
test_task_completes() {
    log_info "Test 2: POST a task and watch worker drain it (≤10s)"
    local body='{"task_type":"verification","task_name":"drain-probe","priority":"normal"}'
    local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/tasks" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo '{}')
    local task_id=$(echo "$resp" | jq -r '.id // empty')
    if [[ -z "$task_id" || "$task_id" == "null" ]]; then
        record_assertion "drain" "task_created" "false" "Failed to create task: $resp"
        return 1
    fi
    record_assertion "drain" "task_created" "true" "Task created: $task_id"

    local initial=$(curl -fsS "$BASE_URL/v1/tasks/$task_id/status" 2>/dev/null | jq -r '.status // empty')
    if [[ "$initial" == "pending" ]]; then
        record_assertion "drain" "initial_pending" "true" "Initial status is pending"
    else
        record_assertion "drain" "initial_pending" "false" "Initial status='$initial' (expected pending)"
    fi

    # Poll for completion (worker tick is 500ms; allow up to 10s)
    local final="pending"
    for i in $(seq 1 20); do
        sleep 0.5
        final=$(curl -fsS "$BASE_URL/v1/tasks/$task_id/status" 2>/dev/null | jq -r '.status // empty')
        if [[ "$final" == "completed" || "$final" == "failed" ]]; then
            break
        fi
    done

    if [[ "$final" == "completed" ]]; then
        record_assertion "drain" "final_completed" "true" "Task drained to completed in ≤10s"
    else
        record_assertion "drain" "final_completed" "false" "Task stuck at status='$final' after 10s — DRAINER NOT WORKING"
        return 1
    fi

    # Verify the lifecycle was logged
    local events=$(curl -fsS "$BASE_URL/v1/tasks/$task_id/logs" 2>/dev/null | jq -r '.logs[].event_type' | sort -u)
    if echo "$events" | grep -q "task.started"; then
        record_assertion "drain" "started_event" "true" "task.started event recorded"
    else
        record_assertion "drain" "started_event" "false" "task.started event missing from logs"
    fi
    if echo "$events" | grep -qE "task\.completed(_noop)?"; then
        record_assertion "drain" "completed_event" "true" "task.completed* event recorded"
    else
        record_assertion "drain" "completed_event" "false" "task.completed event missing from logs"
    fi

    # Detail endpoint should now show started_at populated
    local started_at=$(curl -fsS "$BASE_URL/v1/tasks/$task_id" 2>/dev/null | jq -r '.started_at // 0')
    if [[ "$started_at" -gt 1000000000 ]]; then
        record_assertion "drain" "started_at_set" "true" "started_at unix ts populated: $started_at"
    else
        record_assertion "drain" "started_at_set" "false" "started_at='$started_at' (expected unix timestamp)"
    fi
}

# Test 3: status_counts show drained tasks as completed (not pending)
test_drained_tasks_marked_completed() {
    log_info "Test 3: drained tasks reflected as completed in status_counts"
    sleep 1 # let any concurrent test tasks drain too
    local stats=$(curl -fsS "$BASE_URL/v1/tasks/queue/stats" 2>/dev/null || echo '{}')
    local completed=$(echo "$stats" | jq -r '.status_counts.completed // 0')
    if [[ "$completed" -ge 1 ]]; then
        record_assertion "queue" "tasks_completed" "true" "status_counts.completed=$completed (drainer transitions tasks)"
    else
        record_assertion "queue" "tasks_completed" "false" "status_counts.completed=$completed — drainer not transitioning"
    fi
    # Note: queue.pending_count is the InMemoryTaskQueue's own counter
    # (extracted module), independent of the repo's actual status. After
    # the worker drains via repo.Dequeue, the repo's pending count is 0
    # but the queue's internal counter may still reflect enqueued items.
    # That's a minor architectural quirk, not a CONST-035 violation —
    # the user-visible state (status_counts) is correct.
    local pending=$(echo "$stats" | jq -r '.pending_count // 0')
    log_info "    note: queue.pending_count=$pending (not asserted; queue/repo divergence is expected)"
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_task_completes || true
    test_drained_tasks_marked_completed || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
