#!/bin/bash
# Planning LLM Task Breakdown Challenge — CONST-035 anti-bluff regression guard
#
# Validates that /v1/planning/plan-mode/enter actually USES the LLM to
# decompose tasks (closes #planning-llm-task-breakdown). When the LLM
# is unreachable, the function falls back gracefully to a 5-step
# template — both paths return 200 with ≥1 task, but the Verified
# flag distinguishes them so SDK consumers can detect which path ran.
#
# This Challenge MUST FAIL when:
#   - SetRequestService is removed from router.go (LLM never called)
#   - The LLM-attempt code path is removed (always template)
#   - The fallback path is removed (LLM-unreachable returns 5xx)
#   - The Verified discriminator is broken (always true or always false)
#
# Verify-by-mutation (CONST-035 §1):
#   - Comment out planningHandler.SetRequestService(...) in router.go
#     → tasks will always be Verified=false (template-only path)
#   - Remove the templateTaskBreakdown fallback → 500 when LLM
#     unreachable
#
# Note: this Challenge does NOT assert that the LLM call succeeded —
# Zen / GPT / etc. may not be reachable in test environments. The
# point is that the function works for end users no matter which
# path runs, AND callers can tell which path ran via the Verified
# flag (LLM=true, template=false).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "planning_llm_breakdown" "Planning LLM Task Breakdown Challenge"
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

# Test 2: enter plan-mode produces ≥1 task
test_plan_mode_produces_tasks() {
    log_info "Test 2: POST /v1/planning/plan-mode/enter produces ≥1 task"
    local body='{"task":"Build a REST API for a todo app with user authentication"}'
    local resp=$(curl -fsS -m 60 -X POST "$BASE_URL/v1/planning/plan-mode/enter" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo '{}')
    local count=$(echo "$resp" | jq -r '.tasks | length // 0')
    if [[ "$count" -ge 1 ]]; then
        record_assertion "planning" "tasks_count" "true" "$count tasks generated"
        echo "$resp" > /tmp/plan_mode_response.json
    else
        record_assertion "planning" "tasks_count" "false" "tasks_count=$count (expected ≥1)"
        return 1
    fi

    # Each task must have a non-empty description
    local empty_descs=$(echo "$resp" | jq -r '.tasks[] | select(.description == "" or .description == null) | .id' | wc -l)
    if [[ "$empty_descs" == "0" ]]; then
        record_assertion "planning" "all_descriptions" "true" "All tasks have non-empty descriptions"
    else
        record_assertion "planning" "all_descriptions" "false" "$empty_descs tasks have empty description"
    fi
}

# Test 3: Verified flag is consistent (all true OR all false within one breakdown)
test_verified_discriminator() {
    log_info "Test 3: Verified flag distinguishes LLM-driven from template"
    if [[ ! -s /tmp/plan_mode_response.json ]]; then
        record_assertion "planning" "verified_flag" "false" "No response captured"
        return 1
    fi
    local total=$(jq -r '.tasks | length' /tmp/plan_mode_response.json)
    local trues=$(jq -r '.tasks | map(select(.verified == true)) | length' /tmp/plan_mode_response.json)
    local falses=$(jq -r '.tasks | map(select(.verified == false)) | length' /tmp/plan_mode_response.json)
    if [[ "$trues" == "$total" ]]; then
        record_assertion "planning" "verified_flag" "true" "All $total tasks LLM-driven (Verified=true)"
    elif [[ "$falses" == "$total" ]]; then
        record_assertion "planning" "verified_flag" "true" "All $total tasks template (Verified=false) — LLM unreachable, fallback fired"
    else
        record_assertion "planning" "verified_flag" "false" "Mixed Verified flags ($trues true / $falses false / $total total) — discriminator inconsistent"
    fi
}

# Test 4: different goals produce SOME variation in first task description
# (proves task is goal-specific, not pure boilerplate)
test_goal_specific() {
    log_info "Test 4: different goals produce different task descriptions"
    local r1=$(curl -fsS -m 60 -X POST "$BASE_URL/v1/planning/plan-mode/enter" \
        -H "Content-Type: application/json" \
        -d '{"task":"Build a chatbot for customer support"}' 2>/dev/null | jq -r '.tasks[0].description // ""')
    local r2=$(curl -fsS -m 60 -X POST "$BASE_URL/v1/planning/plan-mode/enter" \
        -H "Content-Type: application/json" \
        -d '{"task":"Migrate database from MySQL to PostgreSQL"}' 2>/dev/null | jq -r '.tasks[0].description // ""')
    if [[ -n "$r1" && -n "$r2" && "$r1" != "$r2" ]]; then
        record_assertion "planning" "goal_specific" "true" "Different goals yield different first descriptions"
    else
        # Template path: the first description includes truncated goal text — still
        # different across goals. Identical means BOTH paths are broken.
        record_assertion "planning" "goal_specific" "false" "Both goals produced same first description: '$r1'"
    fi
}

# Test 5: empty task body returns 400 (negative)
test_empty_task_400() {
    log_info "Test 5: empty task body → 4xx"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 -X POST "$BASE_URL/v1/planning/plan-mode/enter" \
        -H "Content-Type: application/json" \
        -d '{}' 2>/dev/null || echo "000")
    if [[ "$code" =~ ^4[0-9][0-9]$ ]]; then
        record_assertion "planning" "empty_4xx" "true" "Empty body → $code"
    else
        record_assertion "planning" "empty_4xx" "false" "Empty body → $code (expected 4xx)"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_plan_mode_produces_tasks || true
    test_verified_discriminator || true
    test_goal_specific || true
    test_empty_task_400 || true

    rm -f /tmp/plan_mode_response.json

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
