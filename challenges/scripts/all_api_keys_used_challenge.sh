#!/bin/bash
# All-API-Keys-Used Challenge
#
# CONST-032 reproduction script for the user requirement:
# "All API keys MUST BE used"
#
# Intent: when an operator configures an API key in .env (e.g.
# COHERE_API_KEY=sk-...), the corresponding provider MUST appear in
# /v1/providers — even if the key is stale, even if verification
# failed, even if the provider is on cooldown. Operators expect every
# configured credential to participate in the fallback chain at least
# once per circuit-breaker window. Filtering at boot (the verifier
# eliminating providers up-front) is acceptable ONLY if the binary
# also exposes that filtering decision via /v1/monitoring/status so
# operators can see WHY their key isn't being tried.
#
# Pass criteria (all must hold):
#   1. For every PROVIDER_API_KEY env var that resolves to a non-empty
#      value in the running shell, the corresponding provider name is
#      in /v1/providers.
#   2. /v1/monitoring/status reports a per-provider status entry for
#      EVERY provider in /v1/providers (no orphaned providers — if a
#      provider exists in the registry, monitoring must know about it).
#   3. The total /v1/providers count is ≥ the number of resolved
#      API key env vars (the registry should reflect all configured
#      providers, not a smaller filtered subset).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "all_api_keys_used" \
    "All Configured API Keys Are Registered (CONST-032 reproduction guard)"
load_env

# Map from provider env-var name → provider name as it appears in the
# binary's registry. Maintained explicitly because some env names don't
# match the registry name 1:1 (e.g. GITHUB_MODELS_TOKEN → github-models,
# CODESTRAL_API_KEY → codestral, VERTEX_API_KEY → vertex).
declare -A ENV_TO_PROVIDER=(
    [HUGGINGFACE_API_KEY]=huggingface
    [NVIDIA_API_KEY]=nvidia
    [CHUTES_API_KEY]=chutes
    [SILICONFLOW_API_KEY]=siliconflow
    [KIMI_API_KEY]=kimi
    [GEMINI_API_KEY]=gemini
    [OPENROUTER_API_KEY]=openrouter
    [DEEPSEEK_API_KEY]=deepseek
    [MISTRAL_API_KEY]=mistral
    [CODESTRAL_API_KEY]=codestral
    [CEREBRAS_API_KEY]=cerebras
    [CLOUDFLARE_API_KEY]=cloudflare
    [FIREWORKS_API_KEY]=fireworks
    [NOVITA_API_KEY]=novita
    [UPSTAGE_API_KEY]=upstage
    [HYPERBOLIC_API_KEY]=hyperbolic
    [ZAI_API_KEY]=zai
    [GITHUB_MODELS_TOKEN]=github-models
    [GITHUB_TOKEN]=github-models
    [GROQ_API_KEY]=groq
    [REPLICATE_API_KEY]=replicate
    [COHERE_API_KEY]=cohere
    [SAMBANOVA_API_KEY]=sambanova
    [TOGETHER_API_KEY]=together
    [VENICE_API_KEY]=venice
)

# Fetch /v1/providers + /v1/monitoring/status. Health monitor runs
# its first cycle ~30s after boot, so on a freshly-restarted binary
# some providers may be in /v1/providers but not yet in the monitor's
# tracked set. Retry up to 60s before failing so cold-start races
# don't produce spurious test failures.
PROVIDERS_JSON=""
MONITORING_JSON=""
deadline=$(( $(date +%s) + 60 ))
while [[ $(date +%s) -lt $deadline ]]; do
    PROVIDERS_JSON=$(curl -s -m 10 "$BASE_URL/v1/providers" 2>/dev/null || true)
    MONITORING_JSON=$(curl -s -m 10 "$BASE_URL/v1/monitoring/status" 2>/dev/null || true)
    [[ -z "$PROVIDERS_JSON" || -z "$MONITORING_JSON" ]] && { sleep 5; continue; }
    # Wait until provider count == monitor count (cold-start gap closes).
    pc=$(echo "$PROVIDERS_JSON" | python3 -c "import json,sys; print(len(json.load(sys.stdin).get('providers') or []))" 2>/dev/null || echo 0)
    mc=$(echo "$MONITORING_JSON" | python3 -c "import json,sys; print(len((json.load(sys.stdin).get('provider_health',{}).get('providers') or {})))" 2>/dev/null || echo 0)
    [[ "$pc" -gt 0 && "$pc" == "$mc" ]] && break
    sleep 5
done

if [[ -z "$PROVIDERS_JSON" || -z "$MONITORING_JSON" ]]; then
    record_assertion "transport" "endpoints_responded" "false" \
        "/v1/providers or /v1/monitoring/status returned empty"
    finalize_challenge "FAILED"
    exit 1
fi
record_assertion "transport" "endpoints_responded" "true" "endpoints responded"

# Parse names once for fast lookups.
REGISTERED_PROVIDERS=$(echo "$PROVIDERS_JSON" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print('\n'.join(p['name'] for p in d.get('providers', [])))
")
MONITORED_PROVIDERS=$(echo "$MONITORING_JSON" | python3 -c "
import json, sys
d = json.load(sys.stdin)
ph = d.get('provider_health', {})
print('\n'.join((ph.get('providers') or {}).keys()))
")

REGISTERED_COUNT=$(echo "$REGISTERED_PROVIDERS" | grep -c . || echo 0)
log_info "Registered providers (count=$REGISTERED_COUNT):"
echo "$REGISTERED_PROVIDERS" | head -10 | while read -r p; do log_info "  - $p"; done

# Assertion #1: every configured key has its provider registered.
declare -i configured_count=0
declare -i missing_count=0
declare -a missing_list=()
for env_var in "${!ENV_TO_PROVIDER[@]}"; do
    val="${!env_var:-}"
    [[ -z "$val" ]] && continue
    [[ "$val" == "<"* ]] && continue # placeholder strings
    configured_count+=1
    provider_name="${ENV_TO_PROVIDER[$env_var]}"
    if echo "$REGISTERED_PROVIDERS" | grep -qx "$provider_name"; then
        :
    else
        missing_count+=1
        missing_list+=("$env_var → $provider_name")
    fi
done

log_info "Configured API keys with provider mapping: $configured_count"
if [[ $missing_count -eq 0 ]]; then
    record_assertion "registry" "all_keys_have_provider" "true" \
        "all $configured_count configured keys map to a registered provider"
else
    detail=$(printf '%s; ' "${missing_list[@]}")
    record_assertion "registry" "all_keys_have_provider" "false" \
        "$missing_count configured keys have NO registered provider: $detail"
fi

# Assertion #2: every provider in the registry has a monitoring entry.
declare -i unmonitored=0
declare -a unmonitored_list=()
while read -r p; do
    [[ -z "$p" ]] && continue
    if echo "$MONITORED_PROVIDERS" | grep -qx "$p"; then
        :
    else
        unmonitored+=1
        unmonitored_list+=("$p")
    fi
done <<<"$REGISTERED_PROVIDERS"

if [[ $unmonitored -eq 0 ]]; then
    record_assertion "monitoring" "all_providers_monitored" "true" \
        "every registered provider has a monitoring entry"
else
    detail=$(printf '%s, ' "${unmonitored_list[@]}")
    record_assertion "monitoring" "all_providers_monitored" "false" \
        "$unmonitored providers have no monitoring entry: $detail"
fi

# Assertion #3: registry size is sane vs. configured key count.
if [[ $REGISTERED_COUNT -ge $configured_count ]]; then
    record_assertion "registry" "size_covers_configured" "true" \
        "registry has $REGISTERED_COUNT providers, configured keys $configured_count"
else
    record_assertion "registry" "size_covers_configured" "false" \
        "registry has $REGISTERED_COUNT but $configured_count keys configured — registry filtered some out at boot"
fi

record_metric "configured_keys" "$configured_count"
record_metric "registered_providers" "$REGISTERED_COUNT"
record_metric "missing_provider_mappings" "$missing_count"
record_metric "unmonitored_providers" "$unmonitored"

main() {
    local failed_count
    failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)
    failed_count=$(echo "$failed_count" | tr -d '[:space:]')
    [[ -z "$failed_count" ]] && failed_count=0
    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"
        exit 0
    else
        finalize_challenge "FAILED"
        exit 1
    fi
}

main "$@"
