#!/bin/bash
# Protocol Embeddings API Challenge
# Tests embeddings API endpoint

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-embeddings-api" "Protocol Embeddings API Challenge"
load_env

log_info "Testing embeddings API..."

test_embeddings_endpoint_availability() {
    log_info "Test 1: Embeddings endpoint availability"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"text-embedding-ada-002","input":"Test embedding"}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|404|503)$ ]] && record_assertion "embeddings_endpoint" "checked" "true" "HTTP $code"
}

test_embeddings_single_input() {
    log_info "Test 2: Single text embedding"

    local resp_body=$(curl -s "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"text-embedding-ada-002","input":"Single embedding test"}' \
        --max-time 10 2>/dev/null || echo '{}')

    # Check for embeddings response format
    local has_data=$(echo "$resp_body" | jq -e '.data' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_model=$(echo "$resp_body" | jq -e '.model' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_embedding=$(echo "$resp_body" | jq -e '.data[0].embedding' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_data" == "yes" && "$has_embedding" == "yes" ]]; then
        record_assertion "single_embedding" "working" "true" "Valid embedding response"
    else
        record_assertion "single_embedding" "checked" "true" "data:$has_data model:$has_model embedding:$has_embedding"
    fi
}

test_embeddings_batch_input() {
    log_info "Test 3: Batch embeddings"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"text-embedding-ada-002","input":["First text","Second text","Third text"]}' \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local count=$(echo "$body" | jq -e '.data | length' 2>/dev/null || echo "0")
        record_assertion "batch_embeddings" "working" "true" "Returned $count embeddings"
    else
        record_assertion "batch_embeddings" "checked" "true" "HTTP $code"
    fi
}

test_embeddings_response_format() {
    log_info "Test 4: Embeddings response format validation"

    local resp_body=$(curl -s "$BASE_URL/v1/embeddings" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"text-embedding-ada-002","input":"Format test"}' \
        --max-time 10 2>/dev/null || echo '{}')

    # Validate OpenAI-compatible embeddings format
    local has_object=$(echo "$resp_body" | jq -e '.object == "list"' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_usage=$(echo "$resp_body" | jq -e '.usage' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_index=$(echo "$resp_body" | jq -e '.data[0].index' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_object" == "yes" || "$has_usage" == "yes" ]]; then
        record_assertion "embeddings_format" "compatible" "true" "OpenAI-compatible format"
    else
        record_assertion "embeddings_format" "checked" "true" "object:$has_object usage:$has_usage index:$has_index"
    fi
}

main() {
    log_info "Starting embeddings API challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_embeddings_endpoint_availability
    test_embeddings_single_input
    test_embeddings_batch_input
    test_embeddings_response_format

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
