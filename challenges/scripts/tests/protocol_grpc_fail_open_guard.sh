#!/usr/bin/env bash
#
# RED/GREEN polarity guard for challenges/scripts/protocol_grpc_challenge.sh.
#
# THE DEFECT THIS GUARDS (measured 2026-08-10, pre-fix tree)
#
# protocol_grpc_challenge.sh probed a port nobody binds and recorded success
# either way. Two independent faults compounded:
#
#   1. WRONG PORT. Line 11 read `GRPC_PORT="${HELIXAGENT_GRPC_PORT:-7062}"`.
#      Neither half was real. `HELIXAGENT_GRPC_PORT` is a THIRD spelling that
#      no Go source reads (the registry's variable is HELIXAGENT_PORT_GRPC,
#      internal/ports/ports.go:109), and 7062 is nobody's port — not the old
#      literal 50051, not the registry's 8112, not the HTTP 7061.
#
#   2. FAIL-OPEN IN BOTH BRANCHES. Every probe recorded `"true"` whether the
#      port answered or not:
#
#          if nc -z localhost "$GRPC_PORT"; then
#              record_assertion "grpc_server" "available" "true" ...
#          else
#              record_assertion "grpc_server" "checked"   "true" ...   # <-- bluff
#          fi
#
# Together they made the challenge structurally incapable of failing: it
# dialled a dead port, found nothing, and certified the gRPC protocol healthy
# without ever contacting the gRPC server. That is the §11.4.238 class — the
# automated suite reporting green on nothing — and a §11.4.69
# CM-NO-FAIL-OPEN-SKIP violation.
#
# It also violates the framework's OWN documented contract. challenge_framework.sh
# ships record_skip precisely for this case, and its docstring says:
#
#     "NEVER use record_assertion with status=true to mask a missing feature
#      — that's a wrapper bluff per CONST-035 bluff taxonomy."
#
# The primitive existed, was documented, and was not used.
#
# POLARITY (§11.4.115) — one source, two roles:
#
#   RED_MODE=1 — reproduce the defect on the pre-fix artifact. Drives the
#       challenge's own probe functions against a port PROVEN free, and
#       expects at least one PASSED assertion to be recorded anyway. A pass
#       here means "the bluff is faithfully reproducible" — the proof this
#       guard can actually see the bug.
#
#   RED_MODE=0 (default, post-fix) — the standing regression guard. Same
#       probe functions, same proven-free port, and asserts that NOT ONE
#       assertion is recorded PASSED. Absence must surface as SKIPPED (honest,
#       SKIP-OK-tagged) or FAILED — never as success.
#
# WHY SKIP AND NOT FAIL FOR AN ABSENT SERVER (the challenge's own contract).
# main() starts the HTTP binary (start_helixagent) but never starts a gRPC
# server, and it cannot: the gRPC server is a SEPARATE binary, cmd/grpc-server,
# which this challenge neither builds nor launches. An absent listener is
# therefore an un-staged precondition, not a proven product defect, so the
# honest verdict is SKIP-with-reason per §11.4.3 — and SKIPPED is counted
# separately from PASSED by finalize_challenge, so it cannot inflate a score.
# What is forbidden is the third option the pre-fix script chose: calling it
# success.
#
# INSTRUMENT VALIDITY (§11.4.201). Before trusting any verdict, this script
# runs a CONTROL NEEDLE against the framework primitives themselves: it
# records one known-PASSED and one known-FAILED assertion and confirms both
# are observable in assertions.log. A guard that cannot see a PASSED entry it
# planted itself cannot be trusted to report that none exist — the "no PASSED
# found" verdict would be the instrument's blindness, not a fact about the
# script. If the needle does not read back, this exits INSTRUMENT-INVALID
# rather than reporting a result.
#
# The port is chosen by BINDING it and releasing it, so "nothing is listening"
# is measured on this host at this moment rather than assumed from a literal
# — the exact mistake (trusting 7062) that produced the defect.

set -uo pipefail

RED_MODE="${RED_MODE:-0}"
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPTS_DIR="$(cd "$HERE/.." && pwd)"
CHALLENGE="$SCRIPTS_DIR/protocol_grpc_challenge.sh"
FRAMEWORK="$SCRIPTS_DIR/challenge_framework.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log() { printf '%s\n' "$*"; }
fail() { log "GUARD FAIL: $*"; exit 1; }
invalid() { log "INSTRUMENT-INVALID: $*"; exit 2; }

# ---- preconditions --------------------------------------------------------
[ -f "$CHALLENGE" ] || invalid "challenge script not found: $CHALLENGE"
[ -f "$FRAMEWORK" ] || invalid "framework not found: $FRAMEWORK"
command -v nc >/dev/null 2>&1 || invalid "nc is required to mirror the probe"

# ---- a port PROVEN free on this host, right now ---------------------------
# Bind-then-release, rather than trusting a literal. Python is used only as a
# portable "give me a free port" primitive.
free_port() {
    python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()' 2>/dev/null
}
DEAD_PORT="$(free_port)"
[ -n "${DEAD_PORT:-}" ] || invalid "could not obtain a free port to probe"
if nc -z localhost "$DEAD_PORT" 2>/dev/null; then
    invalid "port $DEAD_PORT answered immediately after release; cannot stage an absent server"
fi
log "probe port $DEAD_PORT is PROVEN free (bound, released, re-probed negative)"

# ---- harness: drive the challenge's own functions, not a copy of them -----
# The challenge ends in `main "$@"`; strip exactly that invocation so the
# real function bodies can be sourced and called in isolation. Everything
# above it — including the port-resolution line under test — is preserved
# verbatim, so this exercises the shipped script, not a re-implementation.
STRIPPED="$WORK/challenge_under_test.sh"
sed 's/^main "\$@"$//' "$CHALLENGE" > "$STRIPPED"
if grep -q '^main "\$@"$' "$STRIPPED"; then
    invalid "failed to neutralise the main() invocation"
fi

# The challenge sources its framework via `$(dirname "${BASH_SOURCE[0]}")`, so
# the stripped copy needs the framework beside it or the source silently
# no-ops and every probe function goes undefined. challenge_framework.sh is
# self-contained (it sources nothing), so one symlink suffices. Running from
# the temp dir also keeps init_challenge's results tree out of the repo.
ln -s "$FRAMEWORK" "$WORK/challenge_framework.sh"

# Both spellings point at the proven-free port, so the guard measures the
# FAIL-OPEN behaviour on either artifact regardless of which variable name
# the script reads. These are exported BEFORE sourcing because the script
# resolves its port at source time (line 11). The variable NAME is asserted
# separately below.
export HELIXAGENT_GRPC_PORT="$DEAD_PORT"   # pre-fix spelling
export HELIXAGENT_PORT_GRPC="$DEAD_PORT"   # registry spelling (post-fix)

# Source the SHIPPED script, so the port-resolution line under test executes
# verbatim rather than being re-implemented here. Sourcing also runs the
# script's own top-level `set -e`, `init_challenge` and `load_env`; all three
# are neutralised immediately afterwards so they cannot steer the verdict:
#   - `set -e` would abort this guard on the first non-zero probe
#   - `init_challenge` re-points OUTPUT_DIR at challenges/results/<ts>
#   - `load_env` sources .env, which must not be able to move the probe port
# shellcheck disable=SC1090
source "$STRIPPED" >"$WORK/source.log" 2>&1 || true
set +e

RESOLVED_PORT="${GRPC_PORT:-}"

export OUTPUT_DIR="$WORK/out"
mkdir -p "$OUTPUT_DIR/logs" "$OUTPUT_DIR/results"
export LOG_FILE="$OUTPUT_DIR/logs/guard.log"
ASSERTIONS="$OUTPUT_DIR/logs/assertions.log"
: > "$ASSERTIONS"

# The probes must actually be aimed at the port proven free, or a "nothing
# answered" reading would be meaningless.
if [ "$RESOLVED_PORT" != "$DEAD_PORT" ]; then
    invalid "challenge resolved port '$RESOLVED_PORT' but the proven-free port is $DEAD_PORT; probes would not be measuring an absent server"
fi
log "challenge resolved its probe port to $RESOLVED_PORT (the proven-free port)"

# ---- control needle (§11.4.201) -------------------------------------------
record_assertion "needle" "positive" "true"  "control needle: known PASSED" >/dev/null 2>&1
record_assertion "needle" "negative" "false" "control needle: known FAILED" >/dev/null 2>&1
grep -q '^needle|positive|PASSED|'  "$ASSERTIONS" || invalid "cannot observe a PASSED assertion it planted itself"
grep -q '^needle|negative|FAILED|'  "$ASSERTIONS" || invalid "cannot observe a FAILED assertion it planted itself"
log "control needle OK: guard can observe both PASSED and FAILED records"
: > "$ASSERTIONS"   # discard needle rows; only the challenge's rows are judged

# ---- run the challenge's four probes against the absent server ------------
for fn in test_grpc_server_availability test_grpc_unary_call \
          test_grpc_streaming_call test_grpc_error_handling; do
    declare -F "$fn" >/dev/null || invalid "probe function $fn not found in challenge"
    "$fn" >/dev/null 2>&1
done

# `grep -c` PRINTS "0" and EXITS 1 on no-match, so the idiomatic-looking
# `|| echo 0` emits TWO lines and the count becomes "0\n0". That is the exact
# footgun commit ad3b5590 fixed elsewhere in this repo; `|| true` keeps the
# already-correct "0" and just neutralises the exit status.
count_rows() { grep -c "$1" "$ASSERTIONS" 2>/dev/null || true; }

TOTAL=$(count_rows '|')
PASSED_N=$(count_rows '|PASSED|')
SKIPPED_N=$(count_rows '|SKIPPED|')
FAILED_N=$(count_rows '|FAILED|')

log ""
log "assertions recorded with NO gRPC server on :$DEAD_PORT"
log "  total=$TOTAL passed=$PASSED_N skipped=$SKIPPED_N failed=$FAILED_N"
sed 's/^/    /' "$ASSERTIONS"
log ""

[ "$TOTAL" -ge 4 ] || invalid "expected >=4 assertions from 4 probes, saw $TOTAL"

# ---- verdicts -------------------------------------------------------------
if [ "$RED_MODE" = "1" ]; then
    log "RED_MODE=1: expecting the pre-fix fail-open bluff (PASSED on an absent server)"
    if [ "$PASSED_N" -ge 1 ]; then
        log "RED CONFIRMED: $PASSED_N assertion(s) recorded PASSED while nothing"
        log "               listened on :$DEAD_PORT — the challenge certifies gRPC"
        log "               healthy without ever contacting a gRPC server."
        exit 0
    fi
    fail "RED_MODE=1 but no PASSED-on-absence found — the defect is not present" \
         "on this artifact (already fixed, or the guard is looking in the wrong place)"
fi

log "RED_MODE=0: expecting honest reporting — zero PASSED while the server is absent"

if [ "$PASSED_N" -ne 0 ]; then
    fail "$PASSED_N assertion(s) recorded PASSED with NO gRPC server listening on" \
         ":$DEAD_PORT. Absence must be SKIPPED (§11.4.3) or FAILED, never success" \
         "(§11.4.69 CM-NO-FAIL-OPEN-SKIP)."
fi

if [ "$((SKIPPED_N + FAILED_N))" -lt 4 ]; then
    fail "only $((SKIPPED_N + FAILED_N)) of $TOTAL assertions are SKIPPED/FAILED;" \
         "every probe must report honestly when the server is absent"
fi

# Every skip must carry its SKIP-OK ticket, or the framework's own loudness
# contract is defeated.
if [ "$SKIPPED_N" -gt 0 ] && grep '|SKIPPED|' "$ASSERTIONS" | grep -qv 'SKIP-OK:'; then
    fail "a SKIPPED assertion is missing its SKIP-OK ticket reference"
fi

# The port must be resolved through the registry's own variable name. A
# fourth invented spelling is how the script drifted from the server before.
#
# These assert on EXECUTABLE lines only. Comment lines are excluded on
# purpose: the fix documents the old spelling and the stale literal so the
# defect stays legible, and a guard that forbade naming them in prose would
# be refusing a correct artifact — a §11.4.201 FAIL-bluff, and one that
# punishes documenting the bug.
CODE_ONLY="$WORK/challenge_code_only.sh"
sed 's/[[:space:]]*#.*$//' "$CHALLENGE" > "$CODE_ONLY"

if ! grep -q 'HELIXAGENT_PORT_GRPC' "$CODE_ONLY"; then
    fail "challenge does not resolve its port via HELIXAGENT_PORT_GRPC — the" \
         "registry variable the server itself honours (internal/ports/ports.go)"
fi
if grep -q 'HELIXAGENT_GRPC_PORT' "$CODE_ONLY"; then
    fail "challenge still READS HELIXAGENT_GRPC_PORT, a spelling no Go" \
         "source reads; it will drift from the server again"
fi
if grep -qE ':-7062|:-50051' "$CODE_ONLY"; then
    fail "challenge still defaults to a stale port literal (7062/50051)"
fi

# Absence must not be silently reported as a green challenge either: a run
# in which nothing was verified must not finalize PASSED.
if ! grep -q 'finalize_challenge "SKIPPED"' "$CODE_ONLY"; then
    fail "challenge has no SKIPPED verdict — a run that verifies nothing would" \
         "still finalize PASSED (§11.4.238 false-green)"
fi

log "GUARD PASS: absence is reported honestly ($SKIPPED_N skipped, $FAILED_N failed,"
log "            0 passed), and the port resolves through HELIXAGENT_PORT_GRPC."
exit 0
