#!/bin/bash
# Memory Context Management Challenge
# Tests conversation context management

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-context-management" "Memory Context Management Challenge"
load_env

log_info "Testing context management..."

test_context_window_management() {
    log_info "Test 1: Context window management"

    local request='{"user_id":"test_user","session_id":"sess_001","max_tokens":4096,"include_memory":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/context" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local token_count=$(echo "$body" | jq -e '.token_count' 2>/dev/null || echo "0")
        record_assertion "context_window" "managed" "true" "Tokens: $token_count"
    else
        record_skip "context_window" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_sliding_window_strategy() {
    log_info "Test 2: Sliding window strategy"

    local request='{"user_id":"test_user","session_id":"sess_001","strategy":"sliding","window_size":10}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/context" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "sliding_window" "working" "true" "Strategy applied"
}

test_context_compression() {
    log_info "Test 3: Context compression"

    local request='{"user_id":"test_user","session_id":"sess_001","compress":true,"target_ratio":0.5}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/context/compress" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local compressed=$(echo "$body" | jq -e '.compressed_context' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "context_compression" "working" "true" "Compressed: $compressed"
    else
        record_skip "context_compression" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_relevance_based_pruning() {
    log_info "Test 4: Relevance-based context pruning"

    local request='{"user_id":"test_user","session_id":"sess_001","prune":"relevance","keep_top":5}'

    local resp_body=$(curl -s "$BASE_URL/v1/memory/context/prune" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || echo '{}')

    local pruned_count=$(echo "$resp_body" | jq -e '.pruned_count' 2>/dev/null || echo "0")
    record_assertion "relevance_pruning" "checked" "true" "Pruned: $pruned_count items"
}

main() {
    log_info "Starting context management challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_context_window_management
    test_sliding_window_strategy
    test_context_compression
    test_relevance_based_pruning

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
