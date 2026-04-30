#!/bin/bash
# HTTP Status-Code Correctness Challenge — CONST-035 anti-bluff regression guard
#
# Validates that every documented endpoint returns the right HTTP status
# class for each input class. REST conventions:
#   2xx — success (operation completed for the caller)
#   4xx — caller's fault (bad input, missing field, unauthorized, not found)
#   5xx — server's fault (panic, dependency failure, real error)
#
# Returning 5xx for bad input triggers SDK retry storms and masks real
# user-error feedback. Returning 200 with empty/fabricated content for
# nonexistent IDs is a structural bluff that prevents callers from
# detecting their own bugs.
#
# This Challenge is the regression guard for these fixes:
#   - commit 7647e621 (embeddings/search 500→400, cognee 500→503)
#   - commit ec5902f7 (benchmark/run 500→400, embeddings/index silent-200→400)
#
# Verify-by-mutation (CONST-035 §1):
#   - Remove the strings.Contains routing in benchmark_handler.go →
#     benchmark.bad_input_400 fails
#   - Remove the binding:"required" on embeddings/index Content →
#     embeddings.index_empty_400 fails
#   - Revert cogneeStatusFromError to always return 500 →
#     cognee.disabled_503 fails

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "http_status_correctness" "HTTP Status-Code Correctness Challenge"
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

# Helper: assert that POST endpoint returns expected status for given body
assert_post_status() {
    local label="$1"
    local target="$2"
    local path="$3"
    local body="$4"
    local expected="$5"
    local description="$6"

    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 10 -X POST "$BASE_URL$path" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo "000")
    if [[ "$code" =~ $expected ]]; then
        record_assertion "$label" "$target" "true" "$path → $code (expected $expected): $description"
    else
        record_assertion "$label" "$target" "false" "$path → $code (expected $expected): $description"
    fi
}

# Test 2: /v1/embeddings/search returns 400 for missing query/vector (was 500)
test_embeddings_search_400() {
    log_info "Test 2: /v1/embeddings/search 500→400 for bad input"
    assert_post_status "embeddings" "search_bad_input_400" \
        "/v1/embeddings/search" "{}" "^(400|422)$" \
        "missing query+vector should be 400, not 500"
}

# Test 3: /v1/cognee/cognify returns 503 when service disabled (was 500)
test_cognee_disabled_503() {
    log_info "Test 3: /v1/cognee/cognify 500→503 when service disabled"
    assert_post_status "cognee" "disabled_503" \
        "/v1/cognee/cognify" "{}" "^(503|200)$" \
        "service-disabled should be 503; if enabled, 200 is also OK"
}

# Test 4: /v1/benchmark/run returns 400 for unknown benchmark_type (was 500)
test_benchmark_unknown_type_400() {
    log_info "Test 4: /v1/benchmark/run 500→400 for unknown benchmark_type"
    assert_post_status "benchmark" "bad_type_400" \
        "/v1/benchmark/run" '{"benchmark_type":"definitely-not-real"}' "^4[0-9][0-9]$" \
        "unknown benchmark_type should be 400, not 500"
}

# Test 5: /v1/embeddings/index returns 400 for empty body (was silent 200)
test_embeddings_index_empty_400() {
    log_info "Test 5: /v1/embeddings/index silent-200→400 for empty body"
    assert_post_status "embeddings" "index_empty_400" \
        "/v1/embeddings/index" "{}" "^(400|422)$" \
        "empty doc should be 400, not silent 200"
}

# Test 6: /v1/embeddings/batch-index returns 400 for empty body (was silent 200)
test_embeddings_batch_index_empty_400() {
    log_info "Test 6: /v1/embeddings/batch-index silent-200→400 for empty body"
    assert_post_status "embeddings" "batch_empty_400" \
        "/v1/embeddings/batch-index" "{}" "^(400|422)$" \
        "empty batch should be 400, not silent 200"
}

# Test 7: full sweep — all known POST endpoints with `{}` should return 4xx (not 5xx)
#         (handler must validate input before doing real work)
test_full_sweep_no_5xx_for_empty() {
    log_info "Test 7: full POST-with-{} sweep — no 5xx for client-side errors"
    local POST_PATHS=(
        "/v1/agentic/workflows"
        "/v1/llmops/evaluate"
        "/v1/llmops/prompts"
        "/v1/planning/hiplan"
        "/v1/planning/mcts"
        "/v1/planning/tot"
        "/v1/qa/sessions"
        "/v1/qa/discover"
        "/v1/scoring/batch"
        "/v1/scoring/compare"
        "/v1/verification/model"
        "/v1/verification/batch"
        "/v1/verification/code-visibility"
        "/v1/health/provider"
        "/v1/format"
    )
    local violations=""
    local total=${#POST_PATHS[@]}
    local clean=0
    for path in "${POST_PATHS[@]}"; do
        local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 -X POST "$BASE_URL$path" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{}' 2>/dev/null || echo "000")
        if [[ "$code" =~ ^4[0-9][0-9]$ ]] || [[ "$code" =~ ^2[0-9][0-9]$ ]]; then
            clean=$((clean + 1))
        else
            violations="${violations}\n  - $path → $code"
        fi
    done
    if [[ "$clean" == "$total" ]]; then
        record_assertion "full_sweep" "no_5xx_for_empty" "true" "$total POST endpoints all 2xx/4xx for empty body"
    else
        record_assertion "full_sweep" "no_5xx_for_empty" "false" "$clean/$total clean. Violations:$(printf '%b' "$violations")"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_embeddings_search_400 || true
    test_cognee_disabled_503 || true
    test_benchmark_unknown_type_400 || true
    test_embeddings_index_empty_400 || true
    test_embeddings_batch_index_empty_400 || true
    test_full_sweep_no_5xx_for_empty || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
