#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# no-mocks-above-unit.sh — flag in-process fakes used in non-unit tests.
#
# Mocks/stubs/fakes are permitted ONLY in unit tests (under tests/unit/ or
# *_test.go run with `go test -short`). Every other test type — integration,
# e2e, security, stress, chaos, challenge, benchmark, load, performance,
# pentest, automation, compliance, container, helixqa, precondition, race,
# monitoring, helixllm, standalone, optimization, fuzz — MUST hit the real
# running HelixAgent system with REAL containers, REAL Postgres, REAL Redis,
# REAL HTTP. See CONST-030 in CLAUDE.md and docs/development/definition-of-done.md.
#
# This is the gate that catches the "100% test coverage but the product
# doesn't work" pathology at its source: integration tests that prove the
# mock works, not that the system works.
#
# Exit 0 if no new violations beyond the allowlist; exit 1 otherwise.
#
# Usage:
#   scripts/no-mocks-above-unit.sh                   # normal audit (uses allowlist)
#   scripts/no-mocks-above-unit.sh --all             # report every hit, ignore allowlist
#   scripts/no-mocks-above-unit.sh --update-allowlist # rewrite allowlist to current state
#
# Annotations:
#   // MOCK-OK: #<ticket-or-tag>   — permanently grandfather a single site
#                                    (e.g. when a real upstream is offline in CI).
#                                    Mirror the tag in docs/issues/MOCK_CATEGORIES.md.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ALLOWLIST="${ROOT}/scripts/no-mocks-above-unit-allowlist.txt"
REPORT_ALL=0
UPDATE_ALLOWLIST=0

for arg in "$@"; do
    case "$arg" in
        --all) REPORT_ALL=1 ;;
        --update-allowlist) UPDATE_ALLOWLIST=1 ;;
        -h|--help)
            sed -n '1,30p' "$0" | sed 's/^# //;s/^#//'
            exit 0
            ;;
        *) echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

cd "$ROOT"

# Non-unit test directories (the "above the unit-test line" scope).
SCAN_DIRS=(
    tests/integration
    tests/e2e
    tests/security
    tests/stress
    tests/chaos
    tests/challenge
    tests/benchmark
    tests/benchmarks
    tests/load
    tests/performance
    tests/pentest
    tests/automation
    tests/compliance
    tests/container
    tests/helixqa
    tests/precondition
    tests/race
    tests/monitoring
    tests/helixllm
    tests/standalone
    tests/optimization
    tests/fuzz
)

# Patterns that indicate an in-process fake or mock substituted for real infra.
# Each pattern is one fake-substitution class:
#   httptest.NewServer / NewTLSServer  → in-process HTTP server (not ./bin/helixagent)
#   httptest.NewRecorder               → direct handler invocation (not real HTTP)
#   sqlmock                            → fake DB driver (not real Postgres)
#   gomock / mockgen                   → mock framework
#   miniredis / redismock              → in-process Redis (not the real container)
#   NewMockXxxx(                       → generated/handwritten mock constructor
#   "/mocks" or "/mock" import         → importing a mock package
#   testify/mock                       → testify's mock helper
PATTERNS='httptest\.NewServer\(|httptest\.NewTLSServer\(|httptest\.NewRecorder\(|sqlmock\.|gomock\.|mockgen|miniredis\.|redismock\.|NewMock[A-Z][A-Za-z0-9_]*\(|"[^"]*/mocks?"|testify/mock'

# Anything matching this on the same line is permanently allowed (orthogonal
# to the bulk allowlist file — MOCK-OK is for documented per-site permission).
ALLOW_MARKER='MOCK-OK: #[a-zA-Z0-9_-]+'

INCLUDES=(--include='*.go')
EXCLUDES=(--exclude-dir=testutils --exclude-dir=fixtures --exclude-dir=testdata
          --exclude-dir=vendor --exclude-dir=node_modules)

# Filter to dirs that actually exist.
existing_dirs=()
for d in "${SCAN_DIRS[@]}"; do
    [ -d "$d" ] && existing_dirs+=("$d")
done

if [ ${#existing_dirs[@]} -eq 0 ]; then
    echo "no-mocks-above-unit: no non-unit test directories present — nothing to scan"
    exit 0
fi

# Collect raw hits. `grep -oE` returns just the matched token (not the whole
# line) so allowlist entries are stable across whitespace/wrapping changes.
# Format per line: path:line:matched-token
raw=$(grep -rnoE "$PATTERNS" "${INCLUDES[@]}" "${EXCLUDES[@]}" "${existing_dirs[@]}" 2>/dev/null || true)

# Drop comment-only matches and MOCK-OK-annotated lines. Both filters need the
# original *line text*, which `grep -oE` doesn't include — so we reconstruct
# by re-grepping each hit's source line.
filtered=$(printf '%s\n' "$raw" | awk -F: -v allow="$ALLOW_MARKER" '
    NF < 3 { next }
    {
        path=$1; line=$2;
        # rebuild the matched-token (everything from $3 onward joined by ":")
        token=$3; for (i=4;i<=NF;i++) token=token ":" $i;
        cmd="sed -n " line "p \"" path "\"";
        cmd | getline src;
        close(cmd);
        # skip comment-only lines
        if (src ~ /^[[:space:]]*\/\//) next;
        # skip MOCK-OK-annotated lines
        if (src ~ allow) next;
        printf "%s:%s:%s\n", path, line, token;
    }
')

# Sort unique for deterministic output.
hits=$(printf '%s\n' "$filtered" | sed '/^$/d' | sort -u || true)

# --update-allowlist: rewrite the allowlist file with current hits and exit.
if [ "$UPDATE_ALLOWLIST" -eq 1 ]; then
    {
        echo "# no-mocks-above-unit allowlist — non-unit test sites pending real-infra migration"
        echo "# Format: <relative-path>:<line>:<matched-token>"
        echo "# See docs/development/definition-of-done.md and CONST-030 in CLAUDE.md."
        echo "# Generated by scripts/no-mocks-above-unit.sh --update-allowlist"
        echo "#"
        echo "# Drainage rule: this file should only ever shrink. PRs that grow it"
        echo "# need explicit justification (e.g. genuine upstream-offline edge case)."
        echo "# Per-site permanent permissions belong on the line itself as"
        echo "#   // MOCK-OK: #<ticket-or-tag>"
        echo "# rather than in this bulk allowlist."
        echo "#"
        printf '%s\n' "$hits"
    } > "$ALLOWLIST"
    count=$(printf '%s\n' "$hits" | grep -c . || true)
    echo "no-mocks-above-unit: allowlist refreshed with ${count} entries → ${ALLOWLIST}"
    exit 0
fi

# Load allowlist (ignoring comments / blanks).
allow=""
if [ -f "$ALLOWLIST" ]; then
    allow=$(grep -vE '^[[:space:]]*(#|$)' "$ALLOWLIST" || true)
fi

# Compute "new" violations.
if [ "$REPORT_ALL" -eq 1 ]; then
    new_hits="$hits"
else
    new_hits=$(comm -23 <(printf '%s\n' "$hits" | sort) <(printf '%s\n' "$allow" | sort) | sed '/^$/d' || true)
fi

count=$(printf '%s\n' "$new_hits" | grep -c . || true)
total=$(printf '%s\n' "$hits" | grep -c . || true)
allowed=$(printf '%s\n' "$allow" | grep -c . || true)

if [ "$count" -eq 0 ]; then
    echo "no-mocks-above-unit: OK — ${total} site(s) total, ${allowed} allowlisted, 0 new."
    exit 0
fi

echo "no-mocks-above-unit: ${count} NEW violation(s) detected." >&2
echo "    (in-process fakes used in non-unit tests — see CONST-030)" >&2
echo "    (${total} total sites, ${allowed} pre-existing allowlisted)" >&2
echo "" >&2
printf '%s\n' "$new_hits" | head -30 | sed 's/^/  /' >&2
if [ "$count" -gt 30 ]; then
    echo "  ... ($((count - 30)) more — re-run '$0 --all' to see everything)" >&2
fi

# Per-class breakdown of NEW hits — helps the LLM prioritise the fix.
echo "" >&2
echo "By class (NEW violations only):" >&2
{
    echo -n "  httptest.NewServer/TLSServer/Recorder : "
    printf '%s\n' "$new_hits" | grep -cE 'httptest\.New(Server|TLSServer|Recorder)' | tr -d '\n'; echo
    echo -n "  sqlmock                                : "
    printf '%s\n' "$new_hits" | grep -cE 'sqlmock\.' | tr -d '\n'; echo
    echo -n "  gomock / mockgen                       : "
    printf '%s\n' "$new_hits" | grep -cE 'gomock\.|mockgen' | tr -d '\n'; echo
    echo -n "  miniredis / redismock                  : "
    printf '%s\n' "$new_hits" | grep -cE 'miniredis\.|redismock\.' | tr -d '\n'; echo
    echo -n "  NewMockXxx() constructors              : "
    printf '%s\n' "$new_hits" | grep -cE 'NewMock[A-Z][A-Za-z0-9_]*\(' | tr -d '\n'; echo
    echo -n "  imports of /mocks or /mock packages    : "
    printf '%s\n' "$new_hits" | grep -cE '"[^"]*/mocks?"' | tr -d '\n'; echo
    echo -n "  testify/mock                           : "
    printf '%s\n' "$new_hits" | grep -cE 'testify/mock' | tr -d '\n'; echo
} >&2

echo "" >&2
echo "Fix options:" >&2
echo "  1. Replace the in-process fake with a real-artifact roundtrip (preferred):" >&2
echo "     boot ./bin/helixagent (which boots all real containers per the" >&2
echo "     Mandatory Container Orchestration Flow), call it via http.Client" >&2
echo "     against http://localhost:\$HELIXAGENT_PORT_HTTP/..." >&2
echo "  2. If the test is genuinely unit-level, move it under tests/unit/ or" >&2
echo "     to an internal/**/_test.go file run with -short." >&2
echo "  3. If a real substitute is genuinely unreachable for this single site," >&2
echo "     annotate the line: // MOCK-OK: #<ticket-or-tag>" >&2
echo "  4. If this is a legitimate pre-existing site queued for later migration," >&2
echo "     run: scripts/no-mocks-above-unit.sh --update-allowlist" >&2
echo "     and justify the addition in your PR description." >&2
echo "" >&2
echo "Playbook: docs/development/definition-of-done.md" >&2
exit 1
