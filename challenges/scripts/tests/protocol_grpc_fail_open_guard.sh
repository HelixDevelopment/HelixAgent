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
#
# HXC-291 (2026-08-11, this guard's own defect). The harness below used to
# assume `require_helixagent_grpc`'s `case "$GRPC_IDENTITY" in` would see a
# bound GRPC_IDENTITY after STRIPPED was sourced. HXC-261 moved that
# assignment into main() (so all four probes share one identity lookup), and
# this guard's harness was never updated to supply it after main()'s
# invocation is stripped away. Under `set -u`, referencing GRPC_IDENTITY then
# killed the WHOLE guard process — nounset failures are fatal in a
# non-interactive shell regardless of `set -e`/`set +e`; they are not a
# normal command failure a caller can catch with `$?` — and the probe loop's
# `>/dev/null 2>&1` discarded the one diagnostic line that would have named
# the cause on sight. Both RED_MODE directions died identically: exit 1, zero
# output, no way to tell "the subject failed" from "the instrument broke".
#
# Fixed two ways: (1) the harness now performs main()'s own pre-probe setup
# itself, immediately after sourcing — calling the challenge's OWN
# grpc_identity() (never re-deriving its verdict) and mirroring its two
# evidence-path lines, which are path construction, not probing logic; (2) a
# self-check re-derives, from the SHIPPED main() body on every run (never a
# hardcoded guess), every RECOGNISED global main()'s body appears to assign
# before calling the probes, and fails LOUDLY and DISTINCTLY
# (INSTRUMENT-INVALID, never a RED/GREEN verdict) if this harness has not
# bound every one of them, OR if main() contains any assignment-shaped
# construct the self-check cannot classify (see the R1 correction below).
# NOTE (round 2, B-1 below): this static self-check is now a cheap
# fail-closed PRE-FILTER, not the sole defence — see the probe loop for the
# actual runtime-detection layer that replaced it as load-bearing.
#
# REVIEW (2026-08-12) — two BLOCKING findings on the fix above, both closed
# in this revision; see inline commentary at each site for the full account.
#
#   R1: the self-check's derivation recognised only bare, UPPERCASE,
#   EXACTLY-4-space-indented assignments and claimed (wrongly — bash has no
#   block scope) that anything nested deeper "never leaves main()'s own call
#   frame". A synthetic `if true; then GRPC_B_PROBE=…; fi` inside main(),
#   dereferenced by a probe, sailed past "self-check OK" and then killed the
#   guard mid-probe wearing the ORIGINAL HXC-291 signature — GUARD-FAIL exit
#   code, zero probe output. Fixed by broadening Pass 1 to any indentation
#   depth and adding a Pass 2 that fails CLOSED (INSTRUMENT-INVALID) the
#   moment main() contains any construct — declare/export/readonly/
#   printf -v/read/mapfile/readarray/let/((-arithmetic, a second assignment
#   after a semicolon — Pass 1's simple parser cannot classify, rather than
#   silently trusting a set it cannot vouch for.
#
#   R2: sourcing STRIPPED also sources challenge_framework.sh, whose OWN
#   `trap cleanup EXIT` (challenge_framework.sh:575) SILENTLY REPLACED this
#   guard's `trap 'rm -rf "$WORK"' EXIT` (bash traps are not stacked — the
#   last EXIT registration wins). $WORK leaked on every post-source exit.
#   `cleanup` → `stop_helixagent` is a no-op here (this guard never sets
#   HELIXAGENT_PID), so there was no host-safety exposure — but this
#   guard's forensic-preservation design was void. Fixed by re-arming a
#   trap, immediately after `set +e`, that removes $WORK ONLY on a clean
#   exit 0 and preserves it (printing its path) on any non-zero exit — the
#   leak that exposed R1 is deliberately kept on the FAIL/INVALID paths so a
#   future investigation is never working blind.
#
# REVIEW ROUND 2 (2026-08-12) — architecture change, not a patch.
#
#   B-1 (BLOCKING): R1's static self-check remained undecidable — eight
#   shapes create a real global, are visible to probes, and pass BOTH Pass 1
#   and Pass 2 silently (`: "${NAME:=x}"` defaulting, `for NAME in …` loop
#   variables, `getopts`, a HELPER FUNCTION main() calls that is outside the
#   scanned range — the original defect one level down, an assignment after
#   a `#` on the same line the comment-stripper truncates, `local a=1;
#   NAME=2` where the whole line is excluded by the `local` filter,
#   lowercase globals, `eval` of indirect content) WHILE nine benign shapes
#   made the same check falsely refuse a healthy subject. Proven live: the
#   `${:=}` escape plus a probe dereference printed "self-check OK" (false)
#   and then died mid-probe at exit 1 — the ORIGINAL HXC-291 signature, back
#   through the redesigned check. "Which globals does this function assign"
#   is undecidable by grepping bash; no amount of widening closes it. Fixed
#   by moving detection to RUNTIME instead: each probe call in the loop
#   below is now wrapped `( "$fn" )` — a subshell. A nounset death (or any
#   other fatal shell error) is then confined to the forked child; the
#   parent guard observes a non-zero `$?` exactly like any command failure,
#   and the pre-existing INSTRUMENT-INVALID branch reports it with the real
#   captured diagnostic. Every escape shape above is covered because none of
#   them need to be classified — the runtime either dereferences a bound
#   variable or it doesn't. Verified: record_assertion/record_skip
#   communicate via file appends, which cross a subshell fork boundary
#   (plain file I/O, not shell-memory state), so PASS/FAIL/SKIP recording is
#   unaffected; no probe leaves a bare non-`local` assignment for the parent
#   to read afterward (checked directly against the shipped source).
#
#   F-1: with the runtime layer load-bearing, the static self-check is
#   retained ONLY as a cheap fail-closed pre-filter. Its own prior wording
#   ("completeness is earned ... when Pass 2 also finds zero unrecognised
#   constructs") was ITSELF a smaller instance of the same defect R1
#   fixed — B-1's eight escape shapes falsify it directly. Reworded
#   throughout: the static pass REDUCES, it does not CLOSE, the gap; its
#   messages now say a global's NAME text was found, never that main()
#   provably "assigns" it.
#
#   A-1 (message correction only, logic unchanged this round): the same
#   nine over-catching shapes that make the static pre-filter noisy are
#   acknowledged in its comments rather than papered over; narrowing them
#   further is deferred, because the runtime layer (B-1) is now what a
#   healthy OR broken subject is actually judged by.

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

# ---- re-arm the EXIT cleanup trap (R2, reviewed 2026-08-12) ---------------
# challenge_framework.sh:575 runs `trap cleanup EXIT` at ITS OWN source time
# — and sourcing it (via the `source "$STRIPPED"` above, since the stripped
# copy itself sources the framework at its own top level) runs that
# statement IN THIS SHELL, because `source` never forks. Bash traps are not
# stacked: the LAST `trap ... EXIT` registration wins, so framework.sh's
# `trap cleanup EXIT` SILENTLY REPLACED the `trap 'rm -rf "$WORK"' EXIT` set
# at the top of this script. Measured empirically (isolated scratch tree,
# 2026-08-12): $WORK — the stripped challenge copy, source.log,
# probe_calls.log, every assertion — leaked on EVERY post-source exit,
# whether GUARD PASS, GUARD FAIL, or INSTRUMENT-INVALID, because this
# guard's own cleanup was silently gone from that point on.
#
# cleanup() only calls stop_helixagent(), which is a genuine no-op here:
# HELIXAGENT_PID is never set by this guard (main()'s invocation is
# stripped, so start_helixagent() — the only thing that ever sets it — never
# runs), so `[[ -n "$HELIXAGENT_PID" ]]` is false and nothing is killed. No
# host-safety risk from the framework's trap running once — but this
# guard's OWN forensic-preservation design was void, and this guard's exit
# behaviour was coupled to a cleanup contract it never chose.
#
# Re-armed here (do not fix naively): a plain re-arm of the original
# `rm -rf "$WORK"` trap would REMOVE the exact evidence that let R1 (the
# self-check under-match) be found in the first place — the leaked $WORK
# from this very defect is what a reviewer inspected to diagnose it. So the
# re-armed trap keeps that property on purpose: remove $WORK ONLY on a
# clean exit 0; on ANY non-zero exit (GUARD FAIL or INSTRUMENT-INVALID),
# preserve it and print its path, so a human or a future investigation can
# inspect source.log / probe_calls.log / the stripped challenge copy without
# re-running anything.
cleanup_on_success() {
    local rc=$?
    if [ "$rc" -eq 0 ]; then
        rm -rf "$WORK"
    else
        printf '%s\n' "" "PRESERVED FOR FORENSICS (exit $rc): $WORK" >&2
    fi
}
trap cleanup_on_success EXIT

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

# ---- supply what main() would have set before calling the probes ----------
# STRIPPED removes exactly one line — the trailing `main "$@"` invocation —
# so every function main() would have called is intact, but nothing main()
# ITSELF assigns before calling them has run. The kept fragment (the four
# probe functions + their shared precondition, require_helixagent_grpc) needs
# three such globals; see HXC-291 above.
#
# GRPC_IDENTITY is computed by calling the challenge's OWN grpc_identity()
# here — the exact call main() makes — never by re-deriving its verdict.
# GRPC_PROBE_EVIDENCE / GRPC_ERROR_EVIDENCE are one-line path constructions,
# not probing logic, so mirroring them verbatim carries no re-implementation
# risk. All three MUST be set in this order: grpc_identity() (via
# grpc_identity_probe, only reached when something answers the port) writes
# to GRPC_PROBE_EVIDENCE, so that path must exist before the call is made.
GRPC_PROBE_EVIDENCE="$OUTPUT_DIR/logs/grpc_identity_probe.txt"
GRPC_ERROR_EVIDENCE="$OUTPUT_DIR/logs/grpc_error_handling_probe.txt"
GRPC_IDENTITY="$(grpc_identity)"

# ---- self-check: does the harness above still cover everything main() sets? -
# Runs IMMEDIATELY after the assignments above and BEFORE anything — even
# this guard's own logging — dereferences their VALUES. A mutation that
# removes one of the three lines above must be caught here, by the existence
# test below, not by an incidental later reference crashing the whole
# process the same way HXC-291 did; ordering this block first is what makes
# that guarantee hold regardless of what a future edit adds after it.
#
# CORRECTION (R1, reviewed 2026-08-12): an earlier revision of this
# self-check claimed a 4-space-indented-only regex was enough because
# `local`-declared names "never leave main()'s own call frame" and anything
# nested deeper "does not match" — both claims were wrong. Bash has no
# BLOCK scope: a bare (non-`local`) assignment inside an `if`/`for`/`while`
# body nested ANY number of levels deep inside main() is exactly as GLOBAL
# as one written at main()'s own top level — nesting depth changes nothing
# about visibility, only `local` does. The reviewer's synthetic case proved
# it live: `if true; then GRPC_B_PROBE=…; fi` inside main(), dereferenced by
# a probe, produced `self-check OK: …` and then died mid-probe with exit 1
# — the GUARD-FAIL exit code, wearing the original HXC-291 signature,
# because the 4-space-exact regex silently missed the nested assignment.
#
# Fixed two ways, per the reviewer's fail-closed direction:
#
#   Pass 1 (below) is BROADENED, not merely "4 spaces": ANY positive amount
#   of leading whitespace (tabs included) in front of an UPPERCASE
#   identifier followed by `=` is now recognised, at ANY nesting depth.
#   `local NAME=...` lines are still excluded — correctly this time, not
#   because they are indented differently, but because `local` is bash's
#   ONLY mechanism that keeps an assignment inside main()'s own call frame;
#   a `local` line's target starts with the lowercase word `local`, not an
#   uppercase identifier, so it never matches this pattern regardless of
#   indentation.
#
#   Pass 1 itself was STILL not enough on its own re-review: anchoring to
#   the START of a line (even with the indentation requirement dropped)
#   still misses an assignment that appears mid-line — e.g. the reviewer's
#   OWN synthetic case, `if true; then GRPC_B_PROBE=…; fi`, is one physical
#   line whose assignment sits after `then `, not at line-start, and not
#   immediately after a bare `;` either (there is a keyword between them).
#   So Pass 1 below is not just "any indentation" but ANY POSITION on the
#   line: it finds every `NAME=` occurrence anywhere in a non-`local` line,
#   which catches line-start, nested-if/for/while at any depth, and
#   semicolon-chained or keyword-chained (`then`/`else`/`do`) assignments
#   uniformly, with no special-casing per shape.
#
#   Pass 2 (further below) remains, narrowed to what Pass 1's `NAME=`
#   text-adjacency literally cannot represent: `printf -v NAME` and
#   `read NAME` / `mapfile NAME` / `readarray NAME` never write a literal
#   `NAME=` — the assignment target is a bare positional/flag argument, not
#   `name=value` syntax — so no regex over that shape can find them. Pass 2
#   ALSO re-flags `declare`/`export`/`readonly`/`let`/`((` as a belt-and-
#   suspenders net for the (uncommon, but possible) styling where spaces
#   separate the name from `=` (`let NAME = value`, `(( NAME = value ))`),
#   which would not have the tight `NAME=` adjacency Pass 1 requires. Rather
#   than pretend either pass is complete on its own, the mere PRESENCE of
#   any Pass-2 construct refuses to certify MAIN_GLOBALS as the full
#   dependency set — this self-check goes INSTRUMENT-INVALID on sight
#   rather than proceeding on a set it cannot vouch for (§11.4.201). A
#   shrunken-but-non-empty set that LOOKS complete is more dangerous than an
#   empty one: the empty-set floor below already fails closed correctly (a
#   reviewer's own `{4}`→`{8}` break produced it, and exit 2 correctly
#   followed) — it was the PARTIAL under-match that slipped through
#   undetected, twice now (the original 4-space-exact regex, then a
#   line-start-anchored regex that still missed a mid-line assignment).
#
# HONEST SCOPE (corrected AGAIN, F-1 reviewed 2026-08-12): MAIN_GLOBALS is
# every RECOGNISED bare `NAME=value` global assignment inside main()'s
# body, found ANYWHERE on any non-`local` line — never claimed as "the full
# set of globals main() assigns" outright. The PRIOR wording here claimed
# "completeness is earned ... when Pass 2 also finds zero unrecognised
# constructs" — that claim is ITSELF false: B-1 above lists eight shapes
# (`: "${NAME:=x}"` defaulting, `for NAME in …`, `getopts`, a helper
# function main() calls, a same-line `#`-truncated second assignment,
# `local a=1; NAME=2`, lowercase globals, `eval` of indirect content) that
# create a real global, are invisible to Pass 1's `NAME=` text scan, AND do
# not match any Pass-2 keyword — so "Pass 2 found nothing" does NOT mean
# "nothing was missed." The honest statement is: Pass 1 + Pass 2 REDUCE,
# they do NOT CLOSE, the gap between "what this parser recognised" and
# "what main() actually assigns." This block is retained ONLY as a cheap,
# fast, fail-closed PRE-FILTER now that the probe loop's subshell isolation
# (B-1, below) is the load-bearing runtime defence — a static miss here is
# caught for real when the probe actually runs, never silently. This
# self-check cannot itself be defeated by the unbound-variable class it
# exists to catch: `[ -v "$_main_global" ]` is a plain existence test,
# never an expansion of the variable's VALUE, so it never triggers the
# nounset failure it is checking for.
MAIN_BODY_CODE="$WORK/main_body_code.sh"
sed -n '/^main() {/,/^}/p' "$CHALLENGE" | sed 's/[[:space:]]*#.*$//' > "$MAIN_BODY_CODE"
[ -s "$MAIN_BODY_CODE" ] || invalid "could not extract main()'s own body from $CHALLENGE; the self-check has nothing to verify against"

# Pass 1 — recognised bare-assignment globals, ANY position on ANY
# non-`local` line (not anchored to line-start, not indentation-dependent).
# `local NAME=...` lines are excluded by content, not by position: a line
# whose first non-whitespace token is the literal word `local` is skipped
# entirely, because `local` is bash's ONLY mechanism that keeps an
# assignment inside main()'s own call frame — a sibling function called
# directly by this guard (as every probe below is) could never see a truly
# `local` variable regardless of whether main() itself ever ran.
mapfile -t MAIN_GLOBALS < <(
    grep -v '^[[:space:]]*local[[:space:]]' "$MAIN_BODY_CODE" \
        | grep -oE '[A-Z][A-Z0-9_]*=' \
        | sed 's/=$//' \
        | sort -u
)
[ "${#MAIN_GLOBALS[@]}" -gt 0 ] || invalid "could not find any recognised assignment inside main() in $CHALLENGE; the self-check has nothing to verify against"

# Pass 2 — fail closed on assignment constructs Pass 1's `NAME=` text
# adjacency cannot represent (printf -v / read / mapfile / readarray have
# no `=` at all), plus a belt-and-suspenders re-flag of declare / export /
# readonly / let / (( for the spaced-out styling Pass 1's tight adjacency
# would miss.
MAIN_UNRECOGNISED="$WORK/main_unrecognised.txt"
grep -nE '\bdeclare\b|\bexport\b|\breadonly\b|\bprintf[[:space:]]+-v\b|\bread\b|\bmapfile\b|\breadarray\b|\blet\b|\(\(' "$MAIN_BODY_CODE" \
    > "$MAIN_UNRECOGNISED" || true
if [ -s "$MAIN_UNRECOGNISED" ]; then
    invalid "main() contains assignment-shaped construct(s) this self-check's" \
            "parser does not classify (declare/export/readonly/printf -v/" \
            "read/mapfile/readarray/let/((-arithmetic) — cannot vouch for" \
            "MAIN_GLOBALS completeness, failing closed rather than trusting" \
            "a possibly-incomplete set (R1, reviewed 2026-08-12). Flagged" \
            "line(s): $(cat "$MAIN_UNRECOGNISED")"
fi
log "self-check pre-flight OK: main()'s body contains no unrecognised assignment-shaped construct"

MAIN_GLOBALS_MISSING=()
for _main_global in "${MAIN_GLOBALS[@]}"; do
    [ -v "$_main_global" ] || MAIN_GLOBALS_MISSING+=("$_main_global")
done
if [ "${#MAIN_GLOBALS_MISSING[@]}" -gt 0 ]; then
    invalid "main()'s body appears to contain each of: ${MAIN_GLOBALS[*]/%/=}" \
            "(a syntactic pre-filter match, not a proven semantic assignment" \
            "— see B-1)," \
            "but this guard's harness never bound: ${MAIN_GLOBALS_MISSING[*]}." \
            "The kept fragment may depend on a main()-only global this guard" \
            "does not yet supply — the exact HXC-291 defect class. Fix the" \
            "harness above (supply the missing global), never the probe" \
            "functions."
fi
log "static pre-filter OK: harness supplies every RECOGNISED global main()'s" \
    "body appears to set before the probes (${MAIN_GLOBALS[*]}) — a" \
    "syntactic match, not a completeness proof (B-1/F-1); the subshell-" \
    "isolated runtime check on the probe loop below is the actual defence."

# Safe to dereference the VALUE now: the self-check above already proved
# GRPC_IDENTITY is bound, or exited INSTRUMENT-INVALID before reaching here.
# The `:-` default is defense-in-depth only — belt AND suspenders, not a
# substitute for the self-check owning the actual detection.
log "identity oracle (this guard's own call to the challenge's grpc_identity(), mirroring main()) => GRPC_IDENTITY=${GRPC_IDENTITY:-<unbound>}"

# ---- control needle (§11.4.201) -------------------------------------------
record_assertion "needle" "positive" "true"  "control needle: known PASSED" >/dev/null 2>&1
record_assertion "needle" "negative" "false" "control needle: known FAILED" >/dev/null 2>&1
grep -q '^needle|positive|PASSED|'  "$ASSERTIONS" || invalid "cannot observe a PASSED assertion it planted itself"
grep -q '^needle|negative|FAILED|'  "$ASSERTIONS" || invalid "cannot observe a FAILED assertion it planted itself"
log "control needle OK: guard can observe both PASSED and FAILED records"
: > "$ASSERTIONS"   # discard needle rows; only the challenge's rows are judged

# ---- run the challenge's four probes against the absent server ------------
# B-1 (reviewed 2026-08-12): the static self-check above (Pass 1 + Pass 2)
# CANNOT decide, by grepping bash, which globals main() assigns — eight
# shapes create a global, are visible to probes, and pass BOTH passes
# silently (`: "${NAME:=x}"` defaulting, `for NAME in …` loop variables,
# `getopts`, a HELPER FUNCTION called by main() outside the scanned range,
# an assignment after a `#` on the same line the comment-stripper truncates,
# `local a=1; NAME=2` where the whole line is excluded by the `local`
# filter, lowercase globals, `eval` of indirect content), while NINE benign
# shapes make the same check falsely refuse a healthy subject (a string
# containing `NAME=`, a URL query string, a heredoc body, a `case`/`[[ ==
# ]]` pattern, an `X=1 cmd` prefix assignment that creates no global, a
# split `local X` + `X=`, `$((…))`, the word "read" inside any string).
# Over-catching nine and under-catching eight at once is undecidable-by-
# construction, not a bug to patch with a wider regex — proven live: the
# `${:=}` escape plus a probe dereference printed "self-check OK" (FALSE)
# and then died mid-probe at exit 1, reproducing the ORIGINAL HXC-291
# signature through the redesigned static check.
#
# So detection MOVES to runtime, where it is decidable: each probe call is
# wrapped in a SUBSHELL `( "$fn" )`. A nounset death (or any other fatal
# shell error) inside the subshell terminates ONLY that forked child
# process — bash's documented behaviour for `set -u` failures in a
# non-interactive shell is fatal to the CURRENT shell, and a `( … )`
# subshell IS a distinct forked process, so the parent (this guard) is
# unaffected and observes the child's non-zero exit status via `$?`
# exactly like any other command failure. The existing INSTRUMENT-INVALID
# branch below — already the correct taxonomy (a probe's contract is
# unconditional `return 0`; anything else is a harness fault, never a
# subject verdict) — now fires with the real captured diagnostic (e.g.
# "line N: NAME: unbound variable") in every escape shape above, with no
# parser and no undecidable classification required.
#
# Verified safe: `record_assertion`/`record_skip` communicate via file
# appends to `$OUTPUT_DIR/logs/assertions.log` and evidence-file paths, and
# plain file I/O crosses a subshell fork boundary (it is not shell-memory
# state) — every probe's PASS/FAIL/SKIP still lands correctly. Checked that
# no probe leaves a bare (non-`local`) assignment for the parent to read
# afterward: the only non-`local`-prefixed assignments inside the four
# probe functions + require_helixagent_grpc are `resp=...` reassignments to
# a variable already `local`-declared earlier in the SAME function — they
# never escape their function's call frame with or without a subshell.
#
# Output is captured, never discarded — `>/dev/null 2>&1` previously
# swallowed exactly the diagnostic that would have named HXC-291 on sight.
PROBE_CALL_LOG="$OUTPUT_DIR/logs/probe_calls.log"
: > "$PROBE_CALL_LOG"
for fn in test_grpc_server_availability test_grpc_unary_call \
          test_grpc_streaming_call test_grpc_error_handling; do
    declare -F "$fn" >/dev/null || invalid "probe function $fn not found in challenge"
    echo "=== $fn ===" >>"$PROBE_CALL_LOG"
    ( "$fn" ) >>"$PROBE_CALL_LOG" 2>&1
    _fn_rc=$?
    echo "--- $fn exited $_fn_rc ---" >>"$PROBE_CALL_LOG"
    if [ "$_fn_rc" -ne 0 ]; then
        invalid "$fn exited $_fn_rc; every probe function's own contract is to" \
                "return 0 unconditionally, so a non-zero exit here means this" \
                "guard's harness (or the challenge's shape) is broken, never" \
                "that the subject failed. Captured output: $(cat "$PROBE_CALL_LOG")"
    fi
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

# R3 (reviewed 2026-08-12, LOW — taxonomy, not safety): a TOTAL<4 here is
# reported INSTRUMENT-INVALID, but by this point in the run it could equally
# be a SUBJECT defect wearing the harness code. The control needle above
# already proved this guard's recording channel (record_assertion /
# record_skip → assertions.log) is functional; each of the four probe
# functions has already been confirmed to exit 0 (the probe loop above goes
# INSTRUMENT-INVALID itself on any non-zero exit); so a TOTAL<4 at THIS
# point most plausibly means the CHALLENGE silently recorded nothing on one
# or more probes — the "silent-skip regression" class (§11.4.3 / §11.4.238)
# — not that this guard's own instrument is broken. Left as
# INSTRUMENT-INVALID deliberately rather than reclassified to a subject-side
# fail(): it is loud either way (a human or CI reading either verdict still
# investigates), and routing it to fail() here would change what "RED_MODE=1
# but TOTAL<4" means for the reproduce-the-historical-bug direction too — a
# change with its own blast radius that is out of scope for a LOW finding.
# If this line is ever revisited, the needle-proven-channel argument above
# is the reason a subject-side fail() would be the MORE PRECISE choice.
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
#
# R4 (reviewed 2026-08-12, observation — kept as belt-and-suspenders): this
# check cannot be falsified through record_skip() as currently written,
# because challenge_framework.sh's record_skip auto-brands any reason
# lacking the literal "SKIP-OK:" with the placeholder
# "SKIP-OK: #unmarked-skip-needs-ticket" BEFORE the row ever reaches
# assertions.log (challenge_framework.sh:393-395) — so a challenge that lost
# its REAL ticket reference upstream, but still calls record_skip, would
# still produce a row containing "SKIP-OK:" and PASS this check. What this
# check DOES still catch: a row that bypasses record_skip entirely (a
# probe hand-writing a `|SKIPPED|` line directly into assertions.log,
# outside the framework's own branding contract) with no SKIP-OK marker at
# all — a narrower but real class. Left in place rather than removed: it is
# a correct (if narrower-than-its-comment-implies) guard against that
# direct-write class, and removing a still-useful check to avoid an
# over-broad comment is the wrong trade. The comment above is now accurate
# about scope rather than implying full coverage of every possible
# missing-ticket path.
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
