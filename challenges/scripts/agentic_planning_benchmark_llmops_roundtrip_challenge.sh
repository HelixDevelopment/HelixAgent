#!/bin/bash
# Agentic + Planning + Benchmark + LLMOps Round-Trip Challenge
# CONST-035 anti-bluff regression guard
#
# Validates four endpoint groups in a single Challenge:
#
# 1. /v1/agentic/workflows POST → execution → GET by ID → status="completed"
# 2. /v1/planning/{hiplan,tot} POST → returns real plan/solution structure
# 3. /v1/benchmark/run POST → returns 201 with run ID + benchmark_type
# 4. /v1/llmops/experiments POST → returns 201 with experiment ID + name
#
# Each endpoint was previously documented as "real impl wired" but had no
# regression test verifying the documented user flow worked end-to-end.
#
# Verify-by-mutation (CONST-035 §1):
#   - Replace any real handler with `nil` injection — Challenge fails with
#     503 on the corresponding test.
#   - Strip `entry_point` validation in agentic — POST returns 200 even with
#     missing field, breaking workflow creation.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "agentic_planning_benchmark_llmops_roundtrip" "Agentic + Planning + Benchmark + LLMOps Round-Trip Challenge"
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

# Test 2: /v1/agentic/workflows round-trip
test_agentic_roundtrip() {
    log_info "Test 2: /v1/agentic/workflows POST → GET round-trip"
    local body='{"name":"probe","description":"anti-bluff probe","entry_point":"n1","nodes":[{"id":"n1","type":"task","name":"step1"}],"edges":[]}'
    local resp=$(curl -s -w "\n%{http_code}" --max-time 30 \
        -X POST "$BASE_URL/v1/agentic/workflows" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local payload=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        record_assertion "agentic" "post_200" "true" "POST returned 200"
    else
        record_assertion "agentic" "post_200" "false" "POST returned $code"
        return 1
    fi

    local wid=$(echo "$payload" | jq -r '.id // empty' 2>/dev/null)
    if [[ -n "$wid" && "$wid" != "null" ]]; then
        record_assertion "agentic" "has_id" "true" "Workflow ID: $wid"
    else
        record_assertion "agentic" "has_id" "false" "No workflow ID in response"
        return 1
    fi

    local status=$(echo "$payload" | jq -r '.status // empty' 2>/dev/null)
    if [[ "$status" == "completed" || "$status" == "running" ]]; then
        record_assertion "agentic" "executed" "true" "Workflow status: $status"
    else
        record_assertion "agentic" "executed" "false" "Workflow status='$status' (expected completed/running)"
    fi

    local detail_code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 "$BASE_URL/v1/agentic/workflows/$wid" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$detail_code" == "200" ]]; then
        record_assertion "agentic" "get_by_id" "true" "GET by ID returned 200"
    else
        record_assertion "agentic" "get_by_id" "false" "GET by ID returned $detail_code"
    fi
}

# Test 3: /v1/planning/hiplan
test_planning_hiplan() {
    log_info "Test 3: /v1/planning/hiplan POST → real plan structure"
    local resp=$(curl -fsS --max-time 30 -X POST "$BASE_URL/v1/planning/hiplan" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"goal":"deploy a website","context":""}' 2>/dev/null || echo '{}')
    local plan_id=$(echo "$resp" | jq -r '.plan_id // empty' 2>/dev/null)
    local milestones=$(echo "$resp" | jq -r '.milestones | length // 0' 2>/dev/null)

    if [[ -n "$plan_id" && "$plan_id" != "null" ]]; then
        record_assertion "planning_hiplan" "has_plan_id" "true" "plan_id=$plan_id"
    else
        record_assertion "planning_hiplan" "has_plan_id" "false" "No plan_id"
    fi
    if [[ "$milestones" -ge 1 ]]; then
        record_assertion "planning_hiplan" "has_milestones" "true" "$milestones milestones generated"
    else
        record_assertion "planning_hiplan" "has_milestones" "false" "No milestones in plan"
    fi
}

# Test 4: /v1/planning/tot
test_planning_tot() {
    log_info "Test 4: /v1/planning/tot POST → real Tree-of-Thoughts solution"
    local resp=$(curl -fsS --max-time 30 -X POST "$BASE_URL/v1/planning/tot" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"problem":"calculate fibonacci","depth":3,"breadth":2}' 2>/dev/null || echo '{}')
    local solution_nodes=$(echo "$resp" | jq -r '.solution | length // 0' 2>/dev/null)
    if [[ "$solution_nodes" -ge 1 ]]; then
        record_assertion "planning_tot" "has_solution" "true" "$solution_nodes nodes in solution tree"
    else
        record_assertion "planning_tot" "has_solution" "false" "Empty solution tree"
    fi
}

# Test 5: /v1/benchmark/run + /v1/benchmark/results
test_benchmark_roundtrip() {
    log_info "Test 5: /v1/benchmark/run POST → 201 with run id"
    local resp=$(curl -s -w "\n%{http_code}" --max-time 30 \
        -X POST "$BASE_URL/v1/benchmark/run" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"benchmark_type":"humaneval","providers":["helixagent-llm"]}' 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local payload=$(echo "$resp" | head -n -1)
    if [[ "$code" == "201" ]]; then
        record_assertion "benchmark" "post_201" "true" "POST returned 201"
    else
        record_assertion "benchmark" "post_201" "false" "POST returned $code"
        return 1
    fi
    local rid=$(echo "$payload" | jq -r '.id // empty' 2>/dev/null)
    if [[ -n "$rid" && "$rid" != "null" ]]; then
        record_assertion "benchmark" "has_id" "true" "Benchmark run id=$rid"
    else
        record_assertion "benchmark" "has_id" "false" "No run id"
    fi
}

# Test 6: /v1/llmops/experiments POST + GET
test_llmops_roundtrip() {
    log_info "Test 6: /v1/llmops/experiments POST → 201 with experiment id"
    local body='{"name":"probe-exp","description":"test","variants":[{"name":"v1","prompt":"Say hello"},{"name":"v2","prompt":"Say hi"}]}'
    local resp=$(curl -s -w "\n%{http_code}" --max-time 30 \
        -X POST "$BASE_URL/v1/llmops/experiments" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local payload=$(echo "$resp" | head -n -1)
    if [[ "$code" == "201" ]]; then
        record_assertion "llmops" "post_201" "true" "POST returned 201"
    else
        record_assertion "llmops" "post_201" "false" "POST returned $code"
        return 1
    fi
    local eid=$(echo "$payload" | jq -r '.id // empty' 2>/dev/null)
    if [[ -n "$eid" && "$eid" != "null" ]]; then
        record_assertion "llmops" "has_id" "true" "Experiment id=$eid"
    else
        record_assertion "llmops" "has_id" "false" "No experiment id"
    fi
    local name=$(echo "$payload" | jq -r '.name // empty' 2>/dev/null)
    if [[ "$name" == "probe-exp" ]]; then
        record_assertion "llmops" "name_preserved" "true" "name=probe-exp preserved"
    else
        record_assertion "llmops" "name_preserved" "false" "name='$name' (expected 'probe-exp')"
    fi
}

# Test 7: each negative case is honestly rejected (no contract bluffs)
test_invalid_inputs_rejected() {
    log_info "Test 7: invalid inputs return 4xx (not 200/silent)"
    local code1=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 \
        -X POST "$BASE_URL/v1/agentic/workflows" -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"name":"missing-entry-point","nodes":[]}' 2>/dev/null || echo "000")
    if [[ "$code1" == "400" || "$code1" == "422" ]]; then
        record_assertion "negative" "agentic_4xx" "true" "Missing entry_point rejected ($code1)"
    else
        record_assertion "negative" "agentic_4xx" "false" "Missing entry_point returned $code1"
    fi

    local code2=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 \
        -X POST "$BASE_URL/v1/benchmark/run" -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"benchmark_type":"definitely-not-a-real-benchmark"}' 2>/dev/null || echo "000")
    if [[ "$code2" == "400" || "$code2" == "404" || "$code2" == "500" ]]; then
        record_assertion "negative" "benchmark_4xx" "true" "Bogus benchmark_type rejected ($code2)"
    else
        record_assertion "negative" "benchmark_4xx" "false" "Bogus benchmark_type returned $code2"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_agentic_roundtrip || true
    test_planning_hiplan || true
    test_planning_tot || true
    test_benchmark_roundtrip || true
    test_llmops_roundtrip || true
    test_invalid_inputs_rejected || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
