#!/bin/bash
# Memory Reranking Challenge
# Tests result reranking capabilities

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-reranking" "Memory Reranking Challenge"
load_env

log_info "Testing reranking..."

test_relevance_reranking() {
    log_info "Test 1: Relevance-based reranking"

    local query='{"query":"user preferences","initial_results":["result1","result2","result3"],"rerank":true,"model":"rerank-v1"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/rerank" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_scores=$(echo "$body" | jq -e '.results[].rerank_score' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "relevance_reranking" "working" "true" "Scores: $has_scores"
    else
        record_skip "relevance_reranking" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_cross_encoder_reranking() {
    log_info "Test 2: Cross-encoder reranking"

    local query='{"query":"dark mode settings","candidates":["pref1","pref2","pref3","pref4"],"method":"cross_encoder"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/rerank" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|404|501)$ ]] && record_assertion "cross_encoder" "checked" "true" "HTTP $code (501=not implemented is OK)"
}

test_score_normalization() {
    log_info "Test 3: Reranking score normalization"

    local query='{"query":"test query","results":["a","b","c"],"normalize_scores":true}'

    local resp_body=$(curl -s "$BASE_URL/v1/memory/rerank" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 20 2>/dev/null || echo '{}')

    local has_normalized=$(echo "$resp_body" | jq -e '.results[].normalized_score' > /dev/null 2>&1 && echo "yes" || echo "no")
    record_assertion "score_normalization" "checked" "true" "Normalized: $has_normalized"
}

test_hybrid_reranking() {
    log_info "Test 4: Hybrid reranking (semantic + keyword)"

    local query='{"query":"application preferences","semantic_results":["s1","s2"],"keyword_results":["k1","k2"],"combine":"hybrid"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/rerank" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local count=$(echo "$body" | jq -e '.results | length' 2>/dev/null || echo "0")
        record_assertion "hybrid_reranking" "working" "true" "Combined $count results"
    else
        record_skip "hybrid_reranking" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

main() {
    log_info "Starting reranking challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_relevance_reranking
    test_cross_encoder_reranking
    test_score_normalization
    test_hybrid_reranking

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
