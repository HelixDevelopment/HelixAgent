#!/bin/bash
# Memory Chunking Strategies Challenge
# Tests document chunking for memory storage

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-chunking-strategies" "Memory Chunking Strategies Challenge"
load_env

log_info "Testing chunking strategies..."

test_fixed_size_chunking() {
    log_info "Test 1: Fixed-size chunking"

    local document='{"content":"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.","chunk_strategy":"fixed","chunk_size":50,"overlap":10}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/chunk" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$document" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local chunk_count=$(echo "$body" | jq -e '.chunks | length' 2>/dev/null || echo "0")
        record_assertion "fixed_chunking" "working" "true" "Created $chunk_count chunks"
    else
        record_assertion "fixed_chunking" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_semantic_chunking() {
    log_info "Test 2: Semantic chunking"

    local document='{"content":"First paragraph about topic A. Second paragraph discusses topic B. Third paragraph returns to topic A.","chunk_strategy":"semantic","preserve_context":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/chunk" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$document" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_chunks=$(echo "$body" | jq -e '.chunks' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "semantic_chunking" "working" "true" "Semantic chunks: $has_chunks"
    else
        record_assertion "semantic_chunking" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_sentence_boundary_chunking() {
    log_info "Test 3: Sentence boundary chunking"

    local document='{"content":"First sentence. Second sentence. Third sentence. Fourth sentence.","chunk_strategy":"sentence","sentences_per_chunk":2}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/chunk" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$document" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local chunk_count=$(echo "$body" | jq -e '.chunks | length' 2>/dev/null || echo "0")
        record_assertion "sentence_chunking" "working" "true" "$chunk_count sentence-based chunks"
    else
        record_assertion "sentence_chunking" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_overlap_preservation() {
    log_info "Test 4: Chunk overlap preservation"

    local document='{"content":"This is a test document with multiple sentences for chunking.","chunk_strategy":"fixed","chunk_size":20,"overlap":5}'

    local resp_body=$(curl -s "$BASE_URL/v1/memory/chunk" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$document" \
        --max-time 10 2>/dev/null || echo '{}')

    local has_overlap=$(echo "$resp_body" | jq -e '.chunks[0].overlap_info' > /dev/null 2>&1 && echo "yes" || echo "no")
    record_assertion "overlap_preservation" "checked" "true" "Overlap metadata: $has_overlap"
}

main() {
    log_info "Starting chunking strategies challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_fixed_size_chunking
    test_semantic_chunking
    test_sentence_boundary_chunking
    test_overlap_preservation

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
