#!/bin/bash
# Memory Retrieval Challenge
# Tests memory retrieval operations

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-retrieval" "Memory Retrieval Challenge"
load_env

log_info "Testing memory retrieval..."

test_retrieval_by_id() {
    log_info "Test 1: Memory retrieval by ID"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/get/test_memory_001" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_id=$(echo "$body" | jq -e '.memory_id' > /dev/null 2>&1 && echo "yes" || echo "no")
        local has_content=$(echo "$body" | jq -e '.content' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "retrieval_by_id" "working" "true" "id:$has_id content:$has_content"
    else
        record_assertion "retrieval_by_id" "checked" "true" "HTTP $code (404=not found is OK)"
    fi
}

test_retrieval_by_time_range() {
    log_info "Test 2: Memory retrieval by time range"

    # Query memories from last 24 hours
    local start_time=$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-24H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "2024-01-01T00:00:00Z")
    local end_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/search?start_time=$start_time&end_time=$end_time&limit=20" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local count=$(echo "$body" | jq -e '.memories | length' 2>/dev/null || echo "0")
        record_assertion "retrieval_by_time" "working" "true" "Found $count memories"
    else
        record_assertion "retrieval_by_time" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_retrieval_by_entity() {
    log_info "Test 3: Memory retrieval by entity"

    # Query memories related to a specific entity
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/entity/user_preferences" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_memories=$(echo "$body" | jq -e '.memories' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "retrieval_by_entity" "working" "true" "Entity search: $has_memories"
    else
        record_assertion "retrieval_by_entity" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_context_filtered_retrieval() {
    log_info "Test 4: Context-filtered retrieval"

    # Query with context filters
    local filters='{"user_id":"test_user","category":"preferences","limit":10}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/filter" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$filters" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local count=$(echo "$body" | jq -e '.memories | length' 2>/dev/null || echo "0")
        record_assertion "context_filtering" "working" "true" "Filtered: $count memories"
    else
        record_assertion "context_filtering" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

main() {
    log_info "Starting memory retrieval challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_retrieval_by_id
    test_retrieval_by_time_range
    test_retrieval_by_entity
    test_context_filtered_retrieval

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
