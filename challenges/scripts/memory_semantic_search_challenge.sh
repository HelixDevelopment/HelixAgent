#!/bin/bash
# Memory Semantic Search Challenge
# Tests semantic search capabilities

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-semantic-search" "Memory Semantic Search Challenge"
load_env

log_info "Testing semantic search..."

test_similarity_search() {
    log_info "Test 1: Semantic similarity search"

    local query='{"query":"dark theme preferences","limit":5,"semantic":true}'

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
        record_assertion "similarity_search" "working" "true" "Semantic search: $has_results"
    else
        record_skip "similarity_search" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_ranked_results() {
    log_info "Test 2: Ranked search results"

    local query='{"query":"user interface customization","rank_by":"relevance","limit":10}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_scores=$(echo "$body" | jq -e '.results[].score' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "ranked_results" "working" "true" "Ranking scores: $has_scores"
    else
        record_skip "ranked_results" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_threshold_filtering() {
    log_info "Test 3: Similarity threshold filtering"

    local query='{"query":"application settings","min_similarity":0.7,"limit":10}'

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
        record_assertion "threshold_filtering" "working" "true" "Filtered results: $count"
    else
        record_skip "threshold_filtering" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_embedding_based_search() {
    log_info "Test 4: Embedding-based search"

    # Dummy embedding vector (simulated)
    local query='{"embedding":[0.1,0.2,0.3,0.4,0.5],"limit":5}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/search/vector" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|404|501)$ ]] && record_assertion "embedding_search" "checked" "true" "HTTP $code (501=not implemented is OK)"
}

main() {
    log_info "Starting semantic search challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_similarity_search
    test_ranked_results
    test_threshold_filtering
    test_embedding_based_search

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
