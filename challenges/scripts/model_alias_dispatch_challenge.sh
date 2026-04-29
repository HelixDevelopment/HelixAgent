#!/bin/bash
# Model Alias Dispatch Challenge — CONST-035 anti-bluff regression guard
#
# Validates that EVERY model ID listed in /v1/models actually dispatches
# to a working code path. Closes the contract bluff documented in commit
# 35582f7d where `helixagent-ensemble` was advertised but rejected with
# 404. Per CLAUDE.md "Mandatory Development Standards" rule 27 §c "Full
# usability": a CLI agent / SDK consumer following the documented model
# IDs must succeed without having to know which of N internal aliases
# the dispatcher actually accepts.
#
# This Challenge MUST FAIL when:
#   - any advertised model ID returns 404 / 503 / 5xx
#   - the response model is empty
#   - a non-listed bogus model returns 200 (means the 404 fast-fail
#     guard has regressed)
#
# Verify-by-mutation (CONST-035 §1):
#   - Remove a canonical alias from openai_compatible.go ChatCompletions
#     switch arm → Challenge correctly fails on the orphaned ID.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "model_alias_dispatch" "Model Alias Dispatch Challenge"
load_env

log_info "Testing every advertised model ID dispatches to a working path..."

# Test 1: server alive
test_server_alive() {
    if curl -fsS -m 5 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Test 2: /v1/models lists ≥ 1 model
test_models_list_non_empty() {
    log_info "Test 2: /v1/models returns non-empty model list"
    local resp=$(curl -fsS -m 5 "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo '{}')
    local count=$(echo "$resp" | jq -r '.data | length // 0')
    if [[ "$count" -ge 1 ]]; then
        record_assertion "models_list" "non_empty" "true" "/v1/models has $count entries"
        echo "$resp" | jq -r '.data[].id' > /tmp/model_ids.txt
    else
        record_assertion "models_list" "non_empty" "false" "/v1/models returned 0 entries"
        return 1
    fi
}

# Test 3: each advertised model ID dispatches successfully
test_each_model_dispatches() {
    log_info "Test 3: each advertised model ID returns 200 with non-empty response"

    if [[ ! -s /tmp/model_ids.txt ]]; then
        record_assertion "dispatch" "model_ids_present" "false" "No model IDs from /v1/models"
        return 1
    fi

    local total=0
    local successful=0
    local failures=""
    while IFS= read -r model_id; do
        [[ -z "$model_id" ]] && continue
        total=$((total + 1))
        local body=$(printf '{"model":%s,"messages":[{"role":"user","content":"Reply with exactly: PONG"}],"max_tokens":5}' "$(printf '%s' "$model_id" | jq -R .)")
        local resp=$(curl -s -w "\n%{http_code}" --max-time 75 -X POST "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$body" 2>/dev/null || true)
        local code=$(echo "$resp" | tail -n1)
        local payload=$(echo "$resp" | head -n -1)
        local content_len=$(echo "$payload" | jq -r '.choices[0].message.content // ""' 2>/dev/null | wc -c)
        if [[ "$code" == "200" ]] && [[ "$content_len" -ge 1 ]]; then
            successful=$((successful + 1))
        else
            failures="${failures}\n  - $model_id: HTTP $code, content_len=$content_len"
        fi
    done < /tmp/model_ids.txt

    if [[ "$successful" == "$total" ]] && [[ "$total" -ge 1 ]]; then
        record_assertion "dispatch" "all_models_work" "true" "All $total advertised model IDs dispatched successfully"
    else
        record_assertion "dispatch" "all_models_work" "false" "$successful/$total models dispatched. Failures:$(printf '%b' "$failures")"
    fi
}

# Test 4: bogus model returns 404 (negative)
test_bogus_model_404() {
    log_info "Test 4: bogus model returns 404"
    local body='{"model":"definitely-not-a-real-model-zzzzz","messages":[{"role":"user","content":"hi"}],"max_tokens":5}'
    local code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 \
        -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo "000")
    if [[ "$code" == "404" ]]; then
        record_assertion "bogus_model" "returns_404" "true" "Bogus model correctly 404"
    else
        record_assertion "bogus_model" "returns_404" "false" "Bogus model returned $code (expected 404)"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_models_list_non_empty || { finalize_challenge "FAILED"; exit 1; }
    test_each_model_dispatches || true
    test_bogus_model_404 || true

    rm -f /tmp/model_ids.txt

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
