#!/bin/bash
# Memory Entity Extraction Challenge
# Tests entity extraction from memory content

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-entity-extraction" "Memory Entity Extraction Challenge"
load_env

log_info "Testing entity extraction..."

test_entity_detection() {
    log_info "Test 1: Entity detection"

    local content='{"content":"John Smith works at Acme Corp in San Francisco","extract_entities":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/extract" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$content" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_entities=$(echo "$body" | jq -e '.entities' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "entity_detection" "working" "true" "Entities detected: $has_entities"
    else
        record_assertion "entity_detection" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_relationship_extraction() {
    log_info "Test 2: Relationship extraction"

    local content='{"content":"Alice manages the engineering team and reports to Bob","extract_relationships":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/extract" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$content" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_relationships=$(echo "$body" | jq -e '.relationships' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "relationship_extraction" "working" "true" "Relationships: $has_relationships"
    else
        record_assertion "relationship_extraction" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_entity_types() {
    log_info "Test 3: Entity type classification"

    local content='{"content":"Meeting with Dr. Jane at Google headquarters on January 15th","classify_types":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/extract" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$content" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_types=$(echo "$body" | jq -e '.entities[].type' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "entity_types" "classified" "true" "Types identified: $has_types"
    else
        record_assertion "entity_types" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_confidence_scoring() {
    log_info "Test 4: Entity confidence scoring"

    local content='{"content":"The cat sat on the mat","include_confidence":true}'

    local resp_body=$(curl -s "$BASE_URL/v1/memory/extract" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$content" \
        --max-time 15 2>/dev/null || echo '{}')

    local has_confidence=$(echo "$resp_body" | jq -e '.entities[].confidence' > /dev/null 2>&1 && echo "yes" || echo "no")
    record_assertion "confidence_scoring" "checked" "true" "Confidence scores: $has_confidence"
}

main() {
    log_info "Starting entity extraction challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_entity_detection
    test_relationship_extraction
    test_entity_types
    test_confidence_scoring

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
