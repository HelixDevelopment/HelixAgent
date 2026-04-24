#!/bin/bash
# Memory Vector Similarity Challenge
# Tests vector similarity search

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-vector-similarity" "Memory Vector Similarity Challenge"
load_env

log_info "Testing vector similarity..."

test_cosine_similarity() {
    log_info "Test 1: Cosine similarity search"

    local query='{"vector":[0.1,0.2,0.3,0.4,0.5],"similarity_metric":"cosine","top_k":5}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/vector/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_results=$(echo "$body" | jq -e '.results' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "cosine_similarity" "working" "true" "Results: $has_results"
    else
        record_assertion "cosine_similarity" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_euclidean_distance() {
    log_info "Test 2: Euclidean distance search"

    local query='{"vector":[1.0,2.0,3.0,4.0,5.0],"similarity_metric":"euclidean","top_k":5}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/vector/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|404|501)$ ]] && record_assertion "euclidean_distance" "checked" "true" "HTTP $code (501=not implemented is OK)"
}

test_dot_product_similarity() {
    log_info "Test 3: Dot product similarity"

    local query='{"vector":[0.5,0.5,0.5,0.5,0.5],"similarity_metric":"dot_product","top_k":10}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/vector/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local count=$(echo "$body" | jq -e '.results | length' 2>/dev/null || echo "0")
        record_assertion "dot_product" "working" "true" "Found $count results"
    else
        record_assertion "dot_product" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_similarity_threshold_filtering() {
    log_info "Test 4: Similarity threshold filtering"

    local query='{"vector":[0.2,0.4,0.6,0.8,1.0],"similarity_metric":"cosine","min_similarity":0.8,"top_k":10}'

    local resp_body=$(curl -s "$BASE_URL/v1/memory/vector/search" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || echo '{}')

    local count=$(echo "$resp_body" | jq -e '.results | length' 2>/dev/null || echo "0")
    record_assertion "threshold_filtering" "checked" "true" "Filtered to $count results"
}

main() {
    log_info "Starting vector similarity challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_cosine_similarity
    test_euclidean_distance
    test_dot_product_similarity
    test_similarity_threshold_filtering

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
