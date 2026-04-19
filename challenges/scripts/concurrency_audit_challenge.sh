#!/usr/bin/env bash
# challenges/scripts/concurrency_audit_challenge.sh
#
# Enforces CONST-029 — Concurrent-Safe Containers.
#
# A "Pattern-A" struct pairs a sync.Mutex/RWMutex with a bare map[...]
# or []... field, requiring mutex discipline by convention. CONST-029
# prohibits this pattern in new code; existing sites are allowlisted
# and migrate per docs/development/concurrency-playbook.md.
#
# Assertions:
#
#   T1 — The audit tool exists and is executable.
#   T2 — The allowlist file exists.
#   T3 — A fresh --all scan produces a non-empty set of Pattern-A hits
#        (sanity: the awk-state-machine is actually matching something).
#   T4 — The normal (allowlisted) run exits 0 — no unapproved new
#        Pattern-A introductions since the last allowlist snapshot.
#   T5 — The audit catches a synthetic new violation (negative test).
#   T6 — The audit script refuses to touch vendored / third-party trees.
#
# Contract: CONST-019 non-interactive, CONST-022 resource-capped, read-only
# (except for T5 which creates+removes a temp probe file).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
BLUE=$'\033[0;34m'
YELLOW=$'\033[0;33m'
NC=$'\033[0m'

PASS=0
FAIL=0
pass() { PASS=$((PASS + 1)); echo -e "${GREEN}✓${NC} $*"; }
fail() { FAIL=$((FAIL + 1)); echo -e "${RED}✗${NC} $*"; }
info() { echo -e "${BLUE}==>${NC} $*"; }

AUDIT="./scripts/concurrency-audit.sh"
ALLOWLIST="./scripts/concurrency-audit-allowlist.txt"

info "T1 — audit tool exists and is executable"
if [[ -x "$AUDIT" ]]; then
    pass "T1: $AUDIT is executable"
else
    fail "T1: $AUDIT missing or non-executable"
fi

info "T2 — allowlist file exists"
if [[ -f "$ALLOWLIST" ]]; then
    count=$(grep -vE '^[[:space:]]*(#|$)' "$ALLOWLIST" | wc -l)
    pass "T2: $ALLOWLIST exists with $count entries"
else
    fail "T2: $ALLOWLIST missing — run '$AUDIT --update-allowlist' to seed"
fi

info "T3 — fresh --all scan finds Pattern-A hits"
if [[ -x "$AUDIT" ]]; then
    # --all prints violations to stderr; capture both streams.
    all_output=$("$AUDIT" --all 2>&1 || true)
    # The success-case line has "0 new"; the all-case can be either
    # a summary count or a multi-line violation listing. We want to
    # confirm that there's at least one struct the awk matcher caught.
    if echo "$all_output" | grep -qE 'NEW Pattern-A violation|Pattern-A struct'; then
        pass "T3: audit state-machine matches at least one struct"
    else
        fail "T3: audit produced no hits on --all — matcher may be broken"
        echo "$all_output" | head -5 | sed 's/^/    /'
    fi
else
    fail "T3: skipped — audit tool not executable"
fi

info "T4 — normal run is green (no new violations)"
if [[ -x "$AUDIT" ]]; then
    if normal_out=$("$AUDIT" 2>&1); then
        if echo "$normal_out" | grep -qE '0 new'; then
            pass "T4: $(echo "$normal_out" | grep -E 'OK|0 new' | head -1 | sed 's/^concurrency-audit: //')"
        else
            fail "T4: audit passed but summary line unexpected"
            echo "$normal_out" | head -5 | sed 's/^/    /'
        fi
    else
        fail "T4: audit reports NEW violations — check output above"
        echo "$normal_out" | head -20 | sed 's/^/    /'
    fi
fi

info "T5 — negative test: audit catches a synthetic new violation"
probe_dir="$ROOT/internal/_concurrency_audit_probe"
probe_file="$probe_dir/probe.go"
cleanup_probe() { rm -rf "$probe_dir" 2>/dev/null || true; }
trap cleanup_probe EXIT
mkdir -p "$probe_dir"
cat > "$probe_file" <<'EOF'
package concurrencyauditprobe

import "sync"

type ProbeNewPatternAViolation struct {
	mu    sync.RWMutex
	items map[string]int
}
EOF

if "$AUDIT" >/dev/null 2>&1; then
    fail "T5: audit ignored a synthetic new violation — matcher is broken"
else
    probe_out=$("$AUDIT" 2>&1 || true)
    if echo "$probe_out" | grep -q 'ProbeNewPatternAViolation'; then
        pass "T5: audit correctly flagged the synthetic probe"
    else
        fail "T5: audit exited non-zero but did not name the probe struct"
        echo "$probe_out" | head -10 | sed 's/^/    /'
    fi
fi
cleanup_probe
trap - EXIT

# Double-check cleanup didn't leak state.
if "$AUDIT" >/dev/null 2>&1; then
    pass "T5b: audit green again after probe removed"
else
    fail "T5b: probe cleanup failed — audit still reports violations"
fi

info "T6 — audit skips vendored and third-party trees"
skipped_out=$("$AUDIT" --all 2>&1 || true)
leaked=""
for tree in vendor cli_agents MCP MCP_Module; do
    if echo "$skipped_out" | grep -qE "^  ${tree}/|:${tree}/"; then
        leaked="${leaked}${tree} "
    fi
done
if [[ -z "$leaked" ]]; then
    pass "T6: vendored/third-party trees excluded"
else
    fail "T6: audit leaked into excluded trees: $leaked"
fi

echo
if [[ $FAIL -eq 0 ]]; then
    echo -e "${GREEN}ALL PASSED${NC}: $PASS/$((PASS + FAIL))"
    exit 0
else
    echo -e "${RED}FAILED${NC}: $FAIL of $((PASS + FAIL))"
    exit 1
fi
