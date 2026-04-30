#!/bin/bash
# Embeddings Input Validation Challenge — CONST-035 anti-bluff regression guard
#
# Validates that POST /v1/embeddings/generate properly REJECTS empty
# input AND accepts both HelixAgent's native {"text":"..."} payload
# and OpenAI's canonical {"input":"..."} payload.
#
# This Challenge MUST FAIL when:
#   - The empty-input 400 guard is removed (regression: empty body
#     used to return 200 with a 1536-dim "embedding" of empty string,
#     which is a structural bluff per CONST-035 §c).
#   - The OpenAI "input" alias is dropped (CLI agents following the
#     OpenAI Embeddings docs would see their input silently
#     swallowed).
#   - The native "text" field stops working (backward compat).
#
# Verify-by-mutation (CONST-035 §1):
#   - Comment out the strings.TrimSpace(req.Text) == "" check in
#     internal/handlers/embeddings.go → test_empty_body_400 fails.
#   - Remove the raw["input"] alias branch → test_openai_input_compat
#     fails (because the actual embedding will be for empty string, not
#     for "anti-bluff-openai-probe", and the cache count will not grow
#     between probes with different inputs).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "embeddings_input_validation" "Embeddings Input Validation Challenge"
load_env

# Test 1: server alive
test_server_alive() {
    if curl -fsS -m 10 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Test 2: POST {} returns 400 (was: 200 with fake embedding for empty string)
test_empty_body_400() {
    log_info "Test 2: POST {} → 400 (empty input rejected)"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 10 \
        -X POST "$BASE_URL/v1/embeddings/generate" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{}' 2>/dev/null || echo "000")
    if [[ "$code" == "400" ]]; then
        record_assertion "empty_body" "returns_400" "true" "Empty body → 400 (correct)"
    else
        record_assertion "empty_body" "returns_400" "false" "Empty body → $code (expected 400 — STRUCTURAL BLUFF if 200)"
    fi
}

# Test 3: POST {"text":""} returns 400 (whitespace also rejected)
test_empty_text_400() {
    log_info "Test 3: POST {\"text\":\"\"} → 400 (empty text rejected)"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 10 \
        -X POST "$BASE_URL/v1/embeddings/generate" \
        -H "Content-Type: application/json" \
        -d '{"text":""}' 2>/dev/null || echo "000")
    if [[ "$code" == "400" ]]; then
        record_assertion "empty_text" "returns_400" "true" "Empty text → 400 (correct)"
    else
        record_assertion "empty_text" "returns_400" "false" "Empty text → $code (expected 400)"
    fi
}

# Test 4: POST {"text":"hello"} returns 200 with real embedding
test_native_text_200() {
    log_info "Test 4: POST {\"text\":\"...\"} → 200 with real embedding"
    local resp=$(curl -fsS -m 15 -X POST "$BASE_URL/v1/embeddings/generate" \
        -H "Content-Type: application/json" \
        -d '{"text":"native-text-field-probe","model":"text-embedding-ada-002"}' 2>/dev/null || echo '{}')
    local dim=$(echo "$resp" | jq -r '.embeddings | length // 0' 2>/dev/null)
    if [[ "$dim" -ge 100 ]]; then
        record_assertion "native_text" "returns_200" "true" "Native text → ${dim}-dim embedding"
    else
        record_assertion "native_text" "returns_200" "false" "Native text → ${dim}-dim (expected ≥100)"
    fi
}

# Test 5: POST {"input":"hello"} returns 200 with real embedding (OpenAI compat)
test_openai_input_compat() {
    log_info "Test 5: POST {\"input\":\"...\"} → 200 with real embedding (OpenAI compat)"
    local resp=$(curl -fsS -m 15 -X POST "$BASE_URL/v1/embeddings/generate" \
        -H "Content-Type: application/json" \
        -d '{"input":"openai-input-field-probe","model":"text-embedding-ada-002"}' 2>/dev/null || echo '{}')
    local dim=$(echo "$resp" | jq -r '.embeddings | length // 0' 2>/dev/null)
    if [[ "$dim" -ge 100 ]]; then
        record_assertion "openai_input" "returns_200" "true" "OpenAI input → ${dim}-dim embedding"
    else
        record_assertion "openai_input" "returns_200" "false" "OpenAI input → ${dim}-dim (expected ≥100)"
    fi
}

# Test 6: distinct inputs produce distinct cache entries (verifies caching writes
# are real, not no-ops). Mutation: revert the alias branch and BOTH probes go to
# empty string → same cache key → cache count fails to grow.
test_distinct_inputs_grow_cache() {
    log_info "Test 6: distinct inputs grow the cache (proves caching is real)"
    local pre=$(curl -fsS -m 10 "$BASE_URL/v1/embeddings/stats" 2>/dev/null | jq -r '.cachedEmbeddings // 0')
    local ts=$(date +%s%N)
    curl -fsS -m 15 -X POST "$BASE_URL/v1/embeddings/generate" \
        -H "Content-Type: application/json" \
        -d "{\"input\":\"cache-grow-input-${ts}\"}" >/dev/null 2>&1 || true
    curl -fsS -m 15 -X POST "$BASE_URL/v1/embeddings/generate" \
        -H "Content-Type: application/json" \
        -d "{\"text\":\"cache-grow-text-${ts}\"}" >/dev/null 2>&1 || true
    local post=$(curl -fsS -m 10 "$BASE_URL/v1/embeddings/stats" 2>/dev/null | jq -r '.cachedEmbeddings // 0')
    if [[ "$post" -gt "$pre" ]]; then
        record_assertion "cache_grows" "post_gt_pre" "true" "Cache grew $pre → $post"
    else
        record_assertion "cache_grows" "post_gt_pre" "false" "Cache stayed at $pre — input fields silently dropped"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_empty_body_400 || true
    test_empty_text_400 || true
    test_native_text_200 || true
    test_openai_input_compat || true
    test_distinct_inputs_grow_cache || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
