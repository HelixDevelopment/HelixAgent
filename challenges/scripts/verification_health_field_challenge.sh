#!/bin/bash
# /v1/verification/health Field-Naming Bluff Challenge — CONST-035 regression guard
#
# Validates that /v1/verification/health.total_providers reports the actual
# provider count (registered with ExtendedProviderRegistry), not
# stats.TotalVerifications (verification-attempt count). Was a field-naming
# bluff before commit b6a78977: clients saw `healthy_providers: 25,
# total_providers: 0` — contradictory and worse than uninformative.
#
# This Challenge MUST FAIL when:
#   - the handler reverts to using stats.TotalVerifications for total_providers
#   - the ExtendedProviderRegistry bridge is removed (so registry has 0 providers)
#   - GetAllProviderHealth() stops returning bridged providers
#
# Verify-by-mutation (CONST-035 §1):
#   1. Edit verification_handler.go: change `len(all)` back to `stats.TotalVerifications`.
#   2. Re-run this Challenge.
#   3. The "total_eq_healthy" assertion MUST fail.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "verification_health_field" "Verification Health Field-Naming Challenge"
load_env

log_info "Testing /v1/verification/health total_providers vs healthy_providers..."

# Test 1: Server alive
test_server_alive() {
    log_info "Test 1: server is up"
    if curl -fsS -m 5 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding to /v1/health"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Test 2: /v1/verification/health returns 200
test_endpoint_200() {
    log_info "Test 2: /v1/verification/health returns 200"
    local code=$(curl -s -o /tmp/vh_resp.json -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/verification/health" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        record_assertion "endpoint" "status_200" "true" "Returned 200"
    else
        record_assertion "endpoint" "status_200" "false" "Returned $code"
        return 1
    fi
}

# Test 3: total_providers must be ≥ healthy_providers — the field-naming bluff
test_total_ge_healthy() {
    log_info "Test 3: total_providers >= healthy_providers (no field-naming bluff)"
    local total=$(jq -r '.total_providers // 0' /tmp/vh_resp.json 2>/dev/null)
    local healthy=$(jq -r '.healthy_providers // 0' /tmp/vh_resp.json 2>/dev/null)
    if [[ "$total" -ge "$healthy" ]] && [[ "$total" -ge 1 ]]; then
        record_assertion "total_ge_healthy" "consistent" "true" "total=$total healthy=$healthy (consistent + non-zero)"
    else
        record_assertion "total_ge_healthy" "consistent" "false" "total=$total healthy=$healthy — FIELD-NAMING BLUFF (total cannot be < healthy)"
        return 1
    fi
}

# Test 4: total_providers should equal the provider list size from /v1/health/providers
test_total_matches_health_providers_list() {
    log_info "Test 4: total_providers matches /v1/health/providers count (cross-endpoint consistency)"
    local vh_total=$(jq -r '.total_providers // 0' /tmp/vh_resp.json 2>/dev/null)
    local hp_total=$(curl -fsS -m 5 "$BASE_URL/v1/health/providers" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null \
        | jq -r '.total // 0')
    if [[ "$vh_total" == "$hp_total" ]] && [[ "$vh_total" -ge 1 ]]; then
        record_assertion "cross_endpoint" "consistent" "true" "verification_health.total=$vh_total == health/providers.total=$hp_total"
    else
        record_assertion "cross_endpoint" "consistent" "false" "verification_health.total=$vh_total != health/providers.total=$hp_total — bridge inconsistency"
    fi
}

# Test 5: status field is honest "healthy" string
test_status_field() {
    log_info "Test 5: status field present and string"
    local status=$(jq -r '.status // empty' /tmp/vh_resp.json 2>/dev/null)
    if [[ -n "$status" ]]; then
        record_assertion "status_field" "present" "true" "status='$status'"
    else
        record_assertion "status_field" "present" "false" "status field missing"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_endpoint_200 || { finalize_challenge "FAILED"; exit 1; }
    test_total_ge_healthy || true
    test_total_matches_health_providers_list || true
    test_status_field || true

    rm -f /tmp/vh_resp.json

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
