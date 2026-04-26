#!/bin/bash
# AGENTS.md Claims Accuracy Challenge
#
# CONST-032 reproduction guard for AGENTS.md drift.
#
# OpenCode's `init` flow (and any future agent) writes AGENTS.md based
# on what it observes in the codebase, but it doesn't always re-verify
# numeric claims against ground truth — leading to stale assertions
# like "47+ providers" when the real count is 51, or "7 binaries"
# when cmd/ actually has 8 directories.
#
# This challenge validates that EVERY numeric / structural claim in
# AGENTS.md matches what the filesystem says RIGHT NOW. It also
# verifies that the cascade-mandated "Universal Mandatory Constraints"
# section (committed across all submodules + sibling repos) is still
# present at the AGENTS.md root after any rewrite.
#
# Pass criteria:
#   1. Provider count: claim in AGENTS.md must equal
#      `ls internal/llm/providers/ | grep -v common | wc -l`
#   2. Binary count:  claim in AGENTS.md must equal
#      `ls cmd/ | wc -l`
#   3. AGENTS.md contains the canonical "## Universal Mandatory Constraints"
#      section (preserves the cascade work)
#   4. AGENTS.md contains references to all CONST-030, CONST-031,
#      CONST-032 (the latest mandatory rules)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

PROJECT_ROOT_REAL="$(cd "$SCRIPT_DIR/../.." && pwd)"
AGENTS_FILE="$PROJECT_ROOT_REAL/AGENTS.md"

init_challenge "agents_md_claims_accuracy" \
    "AGENTS.md Claims Accuracy Challenge (CONST-032 reproduction guard)"
load_env

if [[ ! -f "$AGENTS_FILE" ]]; then
    record_assertion "presence" "agents_md_exists" "false" \
        "AGENTS.md not found at $AGENTS_FILE"
    finalize_challenge "FAILED"
    exit 1
fi
record_assertion "presence" "agents_md_exists" "true" "AGENTS.md found"

# --- Test 1: provider count claim accuracy ---
actual_providers=$(ls "$PROJECT_ROOT_REAL/internal/llm/providers/" 2>/dev/null | grep -v common | wc -l)
log_info "Actual provider count: $actual_providers"

# Find the largest claimed count in AGENTS.md (e.g. 47, 50+, etc.)
claimed_providers=$(grep -oE "[0-9]+\+?\s*(LLM\s+)?provider" "$AGENTS_FILE" \
    | grep -oE "^[0-9]+" | sort -nu | tail -1)
claimed_providers=${claimed_providers:-0}
log_info "Largest provider count claimed in AGENTS.md: $claimed_providers"

# Tolerance: claim must be ≤ actual AND ≥ actual-2 (small drift OK,
# but not understating by more than 2). Overstating is always a fail.
if [[ "$claimed_providers" -gt "$actual_providers" ]]; then
    record_assertion "claims" "provider_count_accurate" "false" \
        "AGENTS.md claims $claimed_providers providers but only $actual_providers exist (overstated)"
elif [[ $((actual_providers - claimed_providers)) -gt 2 ]]; then
    record_assertion "claims" "provider_count_accurate" "false" \
        "AGENTS.md claims $claimed_providers but actual is $actual_providers (understated by $((actual_providers - claimed_providers)))"
else
    record_assertion "claims" "provider_count_accurate" "true" \
        "claim $claimed_providers vs actual $actual_providers (within tolerance)"
fi

# --- Test 2: binary count claim accuracy ---
actual_binaries=$(ls -d "$PROJECT_ROOT_REAL/cmd/"*/ 2>/dev/null | wc -l)
log_info "Actual cmd/ directory count: $actual_binaries"

# Find any claim like "8 binaries", "7 apps", "9 entry points"
claimed_binaries=$(grep -oE "[0-9]+\s*(binaries|apps|entry\s*points|application[s]?)" "$AGENTS_FILE" \
    | grep -oE "^[0-9]+" | sort -nu | tail -1)
claimed_binaries=${claimed_binaries:-0}
log_info "Largest binary count claimed in AGENTS.md: $claimed_binaries"

if [[ "$claimed_binaries" -ne "$actual_binaries" ]]; then
    record_assertion "claims" "binary_count_accurate" "false" \
        "AGENTS.md claims $claimed_binaries cmd binaries but cmd/ has $actual_binaries directories"
else
    record_assertion "claims" "binary_count_accurate" "true" \
        "claim $claimed_binaries matches actual $actual_binaries"
fi

# --- Test 3: Universal Mandatory Constraints section present ---
if grep -qE "^##\s+Universal Mandatory Constraints" "$AGENTS_FILE"; then
    record_assertion "structure" "universal_constraints_section" "true" \
        "Universal Mandatory Constraints section present"
else
    record_assertion "structure" "universal_constraints_section" "false" \
        "Universal Mandatory Constraints section MISSING — the cascade work was undone"
fi

# --- Test 4: latest CONST rules referenced ---
missing_consts=()
for c in CONST-030 CONST-031 CONST-032; do
    if ! grep -q "$c" "$AGENTS_FILE"; then
        missing_consts+=("$c")
    fi
done
if [[ ${#missing_consts[@]} -eq 0 ]]; then
    record_assertion "structure" "latest_consts_referenced" "true" \
        "all of CONST-030, CONST-031, CONST-032 referenced"
else
    record_assertion "structure" "latest_consts_referenced" "false" \
        "missing references: ${missing_consts[*]}"
fi

record_metric "actual_providers" "$actual_providers"
record_metric "claimed_providers" "$claimed_providers"
record_metric "actual_binaries" "$actual_binaries"
record_metric "claimed_binaries" "$claimed_binaries"

main() {
    local failed_count
    failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)
    failed_count=$(echo "$failed_count" | tr -d '[:space:]')
    [[ -z "$failed_count" ]] && failed_count=0
    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"; exit 0
    else
        finalize_challenge "FAILED"; exit 1
    fi
}

main "$@"
