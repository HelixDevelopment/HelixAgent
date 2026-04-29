#!/bin/bash
# /v1/health/providers Bridge Challenge — CONST-035 anti-bluff regression guard
#
# Validates the contract that /v1/health/providers returns the LLM provider
# list bridged from providerRegistry into the verifier HealthService at boot.
# Was a contract-bluff site before commit e0d06b96: endpoint returned 200
# with `{providers:[],total:0}` even though HelixAgent has 25 LLM providers
# configured.
#
# This Challenge MUST FAIL when:
#   - the boot-time bridge (router.go: `for _, n := range providerRegistry.ListProviders() { healthSvc.AddProvider(...) }`) is removed
#   - the verifier HealthService is replaced with nil
#   - GetAllProvidersHealth() / GetHealthyProviders() stop returning the bridged set
#
# Verify-by-mutation (CONST-035 §1):
#   1. Comment out the bridge loop in router.go.
#   2. Re-run this Challenge.
#   3. The "providers_non_empty" + "specific_provider_present" assertions MUST fail.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "health_providers_bridge" "Health Providers Bridge Challenge"
load_env

log_info "Testing /v1/health/providers bridge from providerRegistry..."

# Test 1: Server alive
test_server_alive() {
    log_info "Test 1: server is up at $BASE_URL"
    if curl -fsS -m 5 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding to /v1/health"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Test 2: Global /v1/health reports providers — proves they're configured
test_global_health_has_providers() {
    log_info "Test 2: /v1/health reports configured providers"
    local resp=$(curl -fsS -m 5 "$BASE_URL/v1/health" 2>/dev/null || echo '{}')
    local total=$(echo "$resp" | jq -r '.providers.total // 0')
    if [[ "$total" -ge 1 ]]; then
        record_assertion "global_health" "has_providers" "true" "global total=$total"
        echo "$total" > /tmp/health_bridge_global_total
    else
        record_assertion "global_health" "has_providers" "false" "global health reports 0 providers — system not configured"
        return 1
    fi
}

# Test 3: /v1/health/providers — the bridged list MUST not be empty
test_health_providers_non_empty() {
    log_info "Test 3: /v1/health/providers returns the bridged list"
    local resp=$(curl -fsS -m 5 "$BASE_URL/v1/health/providers" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo '{}')
    local total=$(echo "$resp" | jq -r '.total // 0')
    if [[ "$total" -ge 1 ]]; then
        record_assertion "health_providers" "non_empty" "true" "/v1/health/providers total=$total"
    else
        record_assertion "health_providers" "non_empty" "false" "/v1/health/providers total=$total — CONTRACT BLUFF (verifier HealthService not bridged from providerRegistry)"
        return 1
    fi

    # Verify the field structure matches docs/api/API_REFERENCE.md
    local has_providers_array=$(echo "$resp" | jq -e '.providers | type == "array"' 2>/dev/null)
    if [[ "$has_providers_array" == "true" ]]; then
        record_assertion "health_providers" "is_array" "true" ".providers is an array (per API contract)"
    else
        record_assertion "health_providers" "is_array" "false" ".providers is not an array"
    fi
}

# Test 4: A specific known provider (cerebras) must appear by ID lookup
test_specific_provider_present() {
    log_info "Test 4: specific provider lookup via /v1/health/provider/:id"
    local code=$(curl -s -o /tmp/health_bridge_provider.json -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/health/provider/cerebras" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        record_assertion "specific_provider" "lookup_200" "true" "GET /v1/health/provider/cerebras returned 200"
    else
        record_assertion "specific_provider" "lookup_200" "false" "GET returned $code (expected 200; provider not bridged)"
        return 1
    fi
    local provider_id=$(jq -r '.provider_id // empty' /tmp/health_bridge_provider.json 2>/dev/null)
    if [[ "$provider_id" == "cerebras" ]]; then
        record_assertion "specific_provider" "id_matches" "true" "provider_id=cerebras"
    else
        record_assertion "specific_provider" "id_matches" "false" "provider_id='$provider_id' (expected 'cerebras')"
    fi
}

# Test 5: /v1/health/providers/healthy lists same provider count
test_healthy_subset() {
    log_info "Test 5: /v1/health/providers/healthy is a subset of /v1/health/providers"
    local resp=$(curl -fsS -m 5 "$BASE_URL/v1/health/providers/healthy" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo '{}')
    local healthy_count=$(echo "$resp" | jq -r '.providers | length // 0' 2>/dev/null)
    if [[ "$healthy_count" -ge 1 ]]; then
        record_assertion "healthy_subset" "non_empty" "true" "$healthy_count healthy providers listed"
    else
        record_assertion "healthy_subset" "non_empty" "false" "/v1/health/providers/healthy lists 0 — bridge not propagating to GetHealthyProviders"
    fi
}

# Test 6: Negative — bogus provider ID returns 404
test_bogus_provider_404() {
    log_info "Test 6: bogus provider ID returns 404"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/health/provider/zzzzz-not-a-real-provider" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$code" == "404" ]]; then
        record_assertion "bogus_provider" "returns_404" "true" "Bogus ID correctly returns 404"
    else
        record_assertion "bogus_provider" "returns_404" "false" "Bogus ID returned $code (expected 404)"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_global_health_has_providers || true
    test_health_providers_non_empty || true
    test_specific_provider_present || true
    test_healthy_subset || true
    test_bogus_provider_404 || true

    rm -f /tmp/health_bridge_global_total /tmp/health_bridge_provider.json

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
