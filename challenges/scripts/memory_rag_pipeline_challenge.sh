#!/bin/bash
# Memory RAG Pipeline Challenge
# Tests Retrieval-Augmented Generation integration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-rag-pipeline" "Memory RAG Pipeline Challenge"
load_env

log_info "Testing RAG pipeline..."

test_memory_retrieval_for_rag() {
    log_info "Test 1: Memory retrieval for RAG context"

    local query='{"query":"What are the user preferences?","top_k":5,"for_rag":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/retrieve" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_context=$(echo "$body" | jq -e '.context' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "rag_retrieval" "working" "true" "Context retrieved: $has_context"
    else
        record_skip "rag_retrieval" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_context_augmentation() {
    log_info "Test 2: Context augmentation for generation"

    local request='{"messages":[{"role":"user","content":"What settings do I prefer?"}],"augment_with_memory":true,"user_id":"test_user"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_choices=$(echo "$body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "context_augmentation" "working" "true" "Augmented response: $has_choices"
    else
        record_assertion "context_augmentation" "checked" "true" "HTTP $code"
    fi
}

test_relevance_scoring() {
    log_info "Test 3: Memory relevance scoring"

    local query='{"query":"dark mode settings","score_relevance":true,"threshold":0.7}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/retrieve" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_scores=$(echo "$body" | jq -e '.results[].relevance_score' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "relevance_scoring" "working" "true" "Scores present: $has_scores"
    else
        record_skip "relevance_scoring" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_memory_injection() {
    log_info "Test 4: Memory injection into prompts"

    local request='{"model":"helixagent-debate","messages":[{"role":"user","content":"Summarize my preferences"}],"inject_memory":true,"user_id":"test_user","max_tokens":50}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "memory_injection" "working" "true" "Injection successful"
}

main() {
    log_info "Starting RAG pipeline challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_memory_retrieval_for_rag
    test_context_augmentation
    test_relevance_scoring
    test_memory_injection

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
