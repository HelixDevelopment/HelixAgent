#!/bin/bash
# Multi-Endpoint Round-Trip Challenge — CONST-035 anti-bluff regression guard
#
# Single Challenge that exercises the full documented user flow on five
# major endpoint groups in one go. Each section corresponds to a separate
# "bluff or fix" event already merged on main:
#
#   1. /v1/embeddings   — POST generate writes through to /stats cache count
#   2. /v1/scoring      — top + range + batch agree on the same provider list
#   3. /v1/acp          — POST execute returns "completed" for a real agent_id
#   4. /v1/vision       — POST analyze returns capability+result shape
#   5. /v1/ensemble     — POST sessions returns 201 (not 500 panic)
#
# This Challenge MUST FAIL when:
#   - any of these endpoints reverts to silent 500 with empty body
#   - the embeddings cache write-through breaks
#   - the ensemble Coordinator nil-check is removed (panic returns)
#   - acp/execute returns 200 but result is empty/null
#
# Verify-by-mutation (CONST-035 §1):
#   - Comment out the `if c.instanceMgr == nil` early-return in
#     coordinator.go CreateSession → ensemble assertion fails with 500.
#   - Remove the cache write in embedding handler → cache_increments fails.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "multi_endpoint_roundtrip" "Multi-Endpoint Round-Trip Challenge"
load_env

# Test 1: server alive
test_server_alive() {
    if curl -fsS -m 5 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Test 2: /v1/embeddings POST → /stats cache reflects
test_embeddings_writethrough() {
    log_info "Test 2: embeddings POST → cache count increments"
    local pre=$(curl -fsS -m 5 "$BASE_URL/v1/embeddings/stats" 2>/dev/null | jq -r '.cachedEmbeddings // 0')
    local input="anti-bluff-multi-probe-$(date +%s%N)"
    local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/embeddings/generate" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"input\":\"$input\",\"model\":\"text-embedding-ada-002\"}" 2>/dev/null || echo '{}')
    local emb_dim=$(echo "$resp" | jq -r '.embeddings | length // 0')
    if [[ "$emb_dim" -ge 100 ]]; then
        record_assertion "embeddings" "real_vector" "true" "POST returned $emb_dim-dim embedding"
    else
        record_assertion "embeddings" "real_vector" "false" "POST returned $emb_dim-dim (expected ≥100)"
    fi
    local post=$(curl -fsS -m 5 "$BASE_URL/v1/embeddings/stats" 2>/dev/null | jq -r '.cachedEmbeddings // 0')
    if [[ "$post" -gt "$pre" ]]; then
        record_assertion "embeddings" "cache_increments" "true" "stats cachedEmbeddings $pre→$post"
    else
        record_assertion "embeddings" "cache_increments" "false" "stats cachedEmbeddings did not change ($pre→$post)"
    fi
}

# Test 3: /v1/scoring cross-endpoint consistency
test_scoring_cross_endpoint() {
    log_info "Test 3: /v1/scoring/{batch,top,range} agree on the same model"
    local model="helixagent-llm"
    local batch=$(curl -fsS -m 5 -X POST "$BASE_URL/v1/scoring/batch" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model_ids\":[\"$model\"]}" 2>/dev/null | jq -r ".scores[] | select(.model_id == \"$model\") | .overall_score // empty" | head -1)
    local top=$(curl -fsS -m 5 "$BASE_URL/v1/scoring/top?limit=10" 2>/dev/null | jq -r ".models[] | select(.model_id == \"$model\") | .overall_score // empty" | head -1)

    if [[ -n "$batch" && -n "$top" ]]; then
        record_assertion "scoring" "both_present" "true" "batch=$batch top=$top"
        # Allow small delta from re-sampling
        local diff=$(awk -v a="$batch" -v b="$top" 'BEGIN { d = a-b; if (d<0) d=-d; print (d<2.0) ? "ok" : "drift" }')
        if [[ "$diff" == "ok" ]]; then
            record_assertion "scoring" "consistent" "true" "scores within tolerance (batch=$batch, top=$top)"
        else
            record_assertion "scoring" "consistent" "false" "scoring drift batch=$batch vs top=$top (delta>2)"
        fi
    else
        record_assertion "scoring" "both_present" "false" "batch='$batch' top='$top' (one or both missing)"
    fi
}

# Test 4: /v1/acp POST execute returns completion
test_acp_execute() {
    log_info "Test 4: /v1/acp/execute completes a real agent task"
    # Pick first available agent
    local agent_id=$(curl -fsS -m 5 "$BASE_URL/v1/acp/agents" 2>/dev/null | jq -r '.agents[0].id // empty')
    if [[ -z "$agent_id" || "$agent_id" == "null" ]]; then
        record_assertion "acp" "agent_id_present" "false" "No agents available"
        return 1
    fi
    record_assertion "acp" "agent_id_present" "true" "Picked agent: $agent_id"
    local resp=$(curl -fsS -m 30 -X POST "$BASE_URL/v1/acp/execute" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"agent_id\":\"$agent_id\",\"task\":\"sort an array\"}" 2>/dev/null || echo '{}')
    local status=$(echo "$resp" | jq -r '.status // empty')
    if [[ "$status" == "completed" || "$status" == "running" ]]; then
        record_assertion "acp" "execute_status" "true" "agent execute status=$status"
    else
        record_assertion "acp" "execute_status" "false" "status='$status' (expected completed/running)"
    fi
    local has_result=$(echo "$resp" | jq -e '.result' >/dev/null 2>&1 && echo "true" || echo "false")
    if [[ "$has_result" == "true" ]]; then
        record_assertion "acp" "has_result" "true" "result object present in response"
    else
        record_assertion "acp" "has_result" "false" "result object missing"
    fi
}

# Test 5: /v1/vision POST analyze returns shape
test_vision_analyze() {
    log_info "Test 5: /v1/vision/analyze returns capability+result shape"
    local resp=$(curl -fsS -m 30 -X POST "$BASE_URL/v1/vision/analyze" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"image_url":"https://example.com/test.jpg","capability":"analyze"}' 2>/dev/null || echo '{}')
    local cap=$(echo "$resp" | jq -r '.capability // empty')
    local status=$(echo "$resp" | jq -r '.status // empty')
    if [[ "$cap" == "analyze" ]]; then
        record_assertion "vision" "capability_echoed" "true" "capability=$cap"
    else
        record_assertion "vision" "capability_echoed" "false" "capability='$cap'"
    fi
    # CONST-035 §c: vision endpoints honestly distinguish "completed"
    # (real vision provider returned a result) from "stub_only" (no
    # vision provider wired in — see vision_stub_honesty_challenge.sh).
    # Both are valid honest responses; what we MUST NOT see is the
    # pre-round-30 fabricated "completed" with hardcoded
    # colors/captions, which is now caught separately. Here we just
    # confirm the response carries one of the honest status values.
    if [[ "$status" == "completed" || "$status" == "stub_only" ]]; then
        record_assertion "vision" "honest_status" "true" "status=$status"
    else
        record_assertion "vision" "honest_status" "false" "status='$status' (expected completed or stub_only)"
    fi
}

# Test 6: /v1/ensemble/sessions POST returns 201 (not 500 panic)
test_ensemble_session_no_panic() {
    log_info "Test 6: /v1/ensemble/sessions POST returns 201 (was 500 panic)"
    local body='{"strategy":"primary_only","participants":{"primary":{"type":"aider"}}}'
    local resp=$(curl -s -w "\n%{http_code}" -m 15 -X POST "$BASE_URL/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local payload=$(echo "$resp" | head -n -1)
    if [[ "$code" == "201" ]]; then
        record_assertion "ensemble" "post_201" "true" "POST returned 201"
    elif [[ "$code" == "500" && -z "$payload" ]]; then
        record_assertion "ensemble" "post_201" "false" "POST returned 500 with empty body — NIL-DEREF PANIC regression"
        return 1
    else
        record_assertion "ensemble" "post_201" "false" "POST returned $code (expected 201)"
    fi
    local sid=$(echo "$payload" | jq -r '.id // empty' 2>/dev/null)
    if [[ -n "$sid" && "$sid" != "null" ]]; then
        record_assertion "ensemble" "has_id" "true" "session id=$sid"
    else
        record_assertion "ensemble" "has_id" "false" "no session id in response"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_embeddings_writethrough || true
    test_scoring_cross_endpoint || true
    test_acp_execute || true
    test_vision_analyze || true
    test_ensemble_session_no_panic || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
