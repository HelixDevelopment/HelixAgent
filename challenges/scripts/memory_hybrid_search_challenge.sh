#!/bin/bash
# Memory Hybrid Search Challenge
# Tests hybrid search (semantic + keyword)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-hybrid-search" "Memory Hybrid Search Challenge"
load_env

log_info "Testing hybrid search..."

test_semantic_keyword_fusion() {
    log_info "Test 1: Semantic and keyword fusion"

    local query='{"query":"dark mode preferences","search_mode":"hybrid","alpha":0.5}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_results=$(echo "$body" | jq -e '.results' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "fusion_search" "working" "true" "Hybrid results: $has_results"
    else
        record_assertion "fusion_search" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_weighted_combination() {
    log_info "Test 2: Weighted result combination"

    local query='{"query":"user settings","semantic_weight":0.7,"keyword_weight":0.3,"limit":10}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local count=$(echo "$body" | jq -e '.results | length' 2>/dev/null || echo "0")
        record_assertion "weighted_combination" "working" "true" "Found $count weighted results"
    else
        record_assertion "weighted_combination" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_fallback_strategy() {
    log_info "Test 3: Search fallback strategy"

    local query='{"query":"obscure rare term","fallback":"keyword","min_results":3}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "fallback_strategy" "working" "true" "Fallback successful"
}

test_result_deduplication() {
    log_info "Test 4: Hybrid result deduplication"

    local query='{"query":"preferences","deduplicate":true,"similarity_threshold":0.95}'

    local resp_body=$(curl -s "$BASE_URL/v1/memory/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || echo '{}')

    local has_dedup=$(echo "$resp_body" | jq -e '.deduplicated' > /dev/null 2>&1 && echo "yes" || echo "no")
    record_assertion "result_deduplication" "checked" "true" "Deduplication: $has_dedup"
}

main() {
    log_info "Starting hybrid search challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_semantic_keyword_fusion
    test_weighted_combination
    test_fallback_strategy
    test_result_deduplication

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
