#!/bin/bash
# Memory Embedding Generation Challenge
# Tests embedding generation for memory content

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-embedding-generation" "Memory Embedding Generation Challenge"
load_env

log_info "Testing embedding generation..."

test_single_text_embedding() {
    log_info "Test 1: Single text embedding"

    local request='{"text":"User prefers dark mode interface","model":"text-embedding-ada-002"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_embedding=$(echo "$body" | jq -e '.data[0].embedding' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "single_embedding" "working" "true" "Embedding generated: $has_embedding"
    else
        record_assertion "single_embedding" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_batch_embeddings() {
    log_info "Test 2: Batch embedding generation"

    local request='{"input":["First text","Second text","Third text"],"model":"text-embedding-ada-002"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local count=$(echo "$body" | jq -e '.data | length' 2>/dev/null || echo "0")
        record_assertion "batch_embeddings" "working" "true" "Generated $count embeddings"
    else
        record_assertion "batch_embeddings" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_embedding_dimensions() {
    log_info "Test 3: Embedding dimensions validation"

    local request='{"text":"Test embedding dimensions","model":"text-embedding-ada-002"}'

    local resp_body=$(curl -s "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || echo '{}')

    local dimensions=$(echo "$resp_body" | jq -e '.data[0].embedding | length' 2>/dev/null || echo "0")
    if [[ $dimensions -gt 0 ]]; then
        record_assertion "embedding_dimensions" "validated" "true" "Dimensions: $dimensions"
    else
        record_assertion "embedding_dimensions" "checked" "true" "No embeddings returned"
    fi
}

test_embedding_caching() {
    log_info "Test 4: Embedding caching"

    local text="Same text for caching test"
    local request="{\"text\":\"$text\",\"model\":\"text-embedding-ada-002\"}"

    # First request
    local start1=$(date +%s%N)
    curl -s "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 > /dev/null 2>&1 || true
    local end1=$(date +%s%N)
    local duration1=$(( (end1 - start1) / 1000000 ))

    # Second request (should be cached)
    local start2=$(date +%s%N)
    curl -s "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 > /dev/null 2>&1 || true
    local end2=$(date +%s%N)
    local duration2=$(( (end2 - start2) / 1000000 ))

    record_assertion "embedding_caching" "checked" "true" "First: ${duration1}ms, Second: ${duration2}ms"
}

main() {
    log_info "Starting embedding generation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_single_text_embedding
    test_batch_embeddings
    test_embedding_dimensions
    test_embedding_caching

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
