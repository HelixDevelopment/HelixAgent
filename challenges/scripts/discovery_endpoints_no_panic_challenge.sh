#!/bin/bash
# /v1/discovery Endpoint No-Panic Challenge — CONST-035 anti-bluff regression guard
#
# Validates that EVERY /v1/discovery/* endpoint returns a well-formed
# response (200 or honest 4xx/5xx with body) instead of panicking with
# 500 + empty body. Closes the bug fixed in commit bdad3927 where 4
# handlers had nil-pointer-dereference panics when discoveryService
# was nil (registry-only mode).
#
# This Challenge MUST FAIL when:
#   - any handler reverts to dereferencing h.discoveryService without nil-check
#   - any endpoint returns 500 with empty body
#   - the response body is missing the documented top-level fields
#
# Verify-by-mutation (CONST-035 §1):
#   1. Remove the `if h.discoveryService == nil` early-return from any of
#      GetSelectedModels / GetDiscoveryStats / GetEnsembleModels.
#   2. Re-run this Challenge.
#   3. The corresponding `*_returns_200` assertion MUST fail with a 500.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "discovery_endpoints_no_panic" "Discovery Endpoints No-Panic Challenge"
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

# Test 2: each documented endpoint returns 200 with non-empty body
probe_endpoint() {
    local path="$1"
    local label="$2"
    local resp_code=$(curl -s -o /tmp/disc_resp.json -w "%{http_code}" -m 5 "$BASE_URL$path" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    local body_len=$(wc -c < /tmp/disc_resp.json)

    if [[ "$resp_code" == "200" ]] && [[ "$body_len" -ge 10 ]]; then
        record_assertion "$label" "returns_200" "true" "$path → 200 with body_len=$body_len"
        return 0
    elif [[ "$resp_code" == "500" ]] && [[ "$body_len" -lt 10 ]]; then
        record_assertion "$label" "returns_200" "false" "$path → 500 with empty body — NIL-DEREF PANIC (CONST-035 §c violation)"
        return 1
    else
        record_assertion "$label" "returns_200" "false" "$path → $resp_code body_len=$body_len"
        return 1
    fi
}

test_models() {
    log_info "Test 2: /v1/discovery/models"
    probe_endpoint "/v1/discovery/models" "discovery_models"
}

test_models_selected() {
    log_info "Test 3: /v1/discovery/models/selected (was 500 nil-deref)"
    probe_endpoint "/v1/discovery/models/selected" "discovery_selected" || return 1

    # Field-shape assertion: must have models, total, source
    local has_models=$(jq -e '.models | type == "array"' /tmp/disc_resp.json >/dev/null 2>&1 && echo "true" || echo "false")
    local has_source=$(jq -e '.source != null' /tmp/disc_resp.json >/dev/null 2>&1 && echo "true" || echo "false")
    if [[ "$has_models" == "true" ]] && [[ "$has_source" == "true" ]]; then
        record_assertion "discovery_selected" "shape_correct" "true" "models[] + source present"
    else
        record_assertion "discovery_selected" "shape_correct" "false" "missing models[] or source field"
    fi
}

test_stats() {
    log_info "Test 4: /v1/discovery/stats (was 500 nil-deref)"
    probe_endpoint "/v1/discovery/stats" "discovery_stats" || return 1

    local has_total=$(jq -e '.stats.total_discovered != null' /tmp/disc_resp.json >/dev/null 2>&1 && echo "true" || echo "false")
    if [[ "$has_total" == "true" ]]; then
        record_assertion "discovery_stats" "has_total_discovered" "true" "stats.total_discovered field present"
    else
        record_assertion "discovery_stats" "has_total_discovered" "false" "stats.total_discovered missing"
    fi
}

test_ensemble() {
    log_info "Test 5: /v1/discovery/ensemble (was 500 nil-deref)"
    probe_endpoint "/v1/discovery/ensemble" "discovery_ensemble" || return 1

    local has_models=$(jq -e '.models | type == "array"' /tmp/disc_resp.json >/dev/null 2>&1 && echo "true" || echo "false")
    if [[ "$has_models" == "true" ]]; then
        record_assertion "discovery_ensemble" "shape_correct" "true" "models[] present"
    else
        record_assertion "discovery_ensemble" "shape_correct" "false" "models[] missing"
    fi
}

# Test 6: /v1/discovery/debate-model returns 503 (not 500) when service unwired
test_debate_model() {
    log_info "Test 6: /v1/discovery/debate-model returns 503 honestly (not 500 panic)"
    local code=$(curl -s -o /tmp/disc_resp.json -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/discovery/debate-model" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    local body_len=$(wc -c < /tmp/disc_resp.json)

    # Either 200 (with metadata) or 503 (honest unavailable) — both valid.
    # 500 with empty body is the panic we're guarding against.
    if [[ "$code" == "500" ]] && [[ "$body_len" -lt 10 ]]; then
        record_assertion "debate_model" "no_panic" "false" "Returned 500 with empty body — NIL-DEREF PANIC"
    elif [[ "$code" == "200" || "$code" == "503" || "$code" == "404" ]]; then
        record_assertion "debate_model" "no_panic" "true" "Returned $code (honest response, not panic)"
    else
        record_assertion "debate_model" "no_panic" "false" "Unexpected status $code"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_models || true
    test_models_selected || true
    test_stats || true
    test_ensemble || true
    test_debate_model || true

    rm -f /tmp/disc_resp.json

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
