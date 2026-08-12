#!/usr/bin/env bash
#
# RED/GREEN polarity guard for challenges/scripts/challenge_framework.sh's
# results-directory infrastructure (HXC-272).
#
# THE DEFECTS THIS GUARDS (measured pre-fix)
#
# DEFECT 1 — shared results folder. init_challenge() used to name OUTPUT_DIR
# after `date +%Y%m%d_%H%M%S` alone — second resolution, nothing appended
# for uniqueness. Two challenges (or two runs of the same challenge) that
# started within the same wall-clock second computed the IDENTICAL
# OUTPUT_DIR and therefore shared one assertions.log. finalize_challenge()
# decides its own verdict by `grep -c`'ing PASSED/FAILED/SKIPPED lines out
# of that shared log, so a same-second collision let one run inherit a
# neighbour's outcomes in BOTH directions:
#
#   - inherited FAILED lines could flip a healthy run red (wasteful, but
#     safe-side), and
#   - inherited PASSED lines — the DANGEROUS direction — could flip a run
#     that verified NOTHING into a reported success. A false-success vector
#     sitting in infrastructure every automated check uses is the same
#     class of defect as a check that cannot fail.
#
# DEFECT 2 — stale "latest" pointer, and a stale by-recency listing. The
# SAME function's `ln -sf "$OUTPUT_DIR" ".../latest"` does not repoint an
# EXISTING `latest` symlink-to-a-directory: `ln` dereferences it, discovers
# a directory, and drops a NEW link named basename(OUTPUT_DIR) INSIDE that
# directory instead. From the second run of a day onward `latest` never
# moves, and every one of those writes bumps the FIRST run's directory
# mtime too, so a recency-by-mtime listing (`ls -td .../*/ | head -1`, the
# recipe CATALOG.md documents) also keeps returning the first run forever.
#
# TWO PHASES, because the two defects have different failure shapes and
# this guard proves each cleanly rather than only their combined effect.
#
#   PHASE 1 (defect 1, both directions). Three runs of the SAME challenge ID
#   forced to an IDENTICAL TIMESTAMP — the same-second collision itself —
#   prove the log-sharing/verdict-contamination shape in both directions
#   (see the per-role rationale below).
#
#   PHASE 2 (defect 2, in isolation). Two runs of a DIFFERENT challenge ID
#   forced to two DIFFERENT, chronologically-later TIMESTAMPs — so they do
#   NOT collide on defect 1 even pre-fix — isolate defect 2's own mechanism:
#   does `latest` actually repoint, and does the first run's directory stay
#   pristine (no nested link dropped inside it), which is what a
#   recency-by-mtime listing depends on.
#
# POLARITY (§11.4.115) — one source, two roles:
#
#   RED_MODE=1 — reproduce both defects on the pre-fix artifact (point
#       CHALLENGE_FRAMEWORK_SCRIPT at a pre-fix copy to replay the exact
#       historical shape). A pass here means "the bugs are faithfully
#       reproducible" — proof this guard can actually see them.
#
#   RED_MODE=0 (default, post-fix) — the standing regression guard: full
#       isolation in phase 1, and a correctly-repointed `latest` +
#       untouched first-run directory in phase 2.
#
# WHY THREE RUNS, IN THIS ORDER, IN PHASE 1 (failing, passing, empty).
#
# Run order matters for demonstrating the pre-fix contamination cleanly:
#   1. "failing"  records one FAILED assertion, finalizes FAILED.
#   2. "passing"  records three PASSED assertions, finalizes PASSED.
#   3. "empty"    records ZERO assertions — a run that verifies nothing —
#                 finalizes PASSED (the label a caller might use for a probe
#                 that found no applicable work).
# Pre-fix, all three collide into ONE shared results folder and assertions
# log. Because each run's OWN results.json is computed from the log's state
# AT THE MOMENT IT FINALIZES:
#   - "passing" finalizes AFTER "failing" already wrote to the shared log,
#     so its OWN results.json shows assertions_failed=1 despite "passing"
#     never itself failing anything — Direction 1 (a healthy run turned red
#     by a neighbour's failure).
#   - "empty" finalizes AFTER BOTH "failing" and "passing" already wrote to
#     the shared log, so its OWN results.json shows assertions_passed=3
#     despite recording literally nothing — Direction 2, the DANGEROUS one:
#     a run that achieved nothing reports a false success.
# Post-fix each run's results.json shows EXACTLY its own counts:
#   failing -> passed=0 failed=1 | passing -> passed=3 failed=0 |
#   empty   -> passed=0 failed=0
#
# INSTRUMENT VALIDITY (§11.4.201). The forced-TIMESTAMP date shim is
# validated before use: it must return the frozen value for the exact
# `date +%Y%m%d_%H%M%S` invocation init_challenge makes, while still
# resolving every OTHER date(1) call through the real binary (log
# timestamps, `date +%N`, `date -Iseconds`, `date +%s`) so nothing else in
# the framework observes a frozen clock. A guard whose stub could not be
# trusted could not be trusted to prove either polarity.

set -uo pipefail

RED_MODE="${RED_MODE:-0}"
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPTS_DIR="$(cd "$HERE/.." && pwd)"
FRAMEWORK="${CHALLENGE_FRAMEWORK_SCRIPT:-$SCRIPTS_DIR/challenge_framework.sh}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log() { printf '%s\n' "$*"; }
fail() { log "GUARD FAIL: $*"; exit 1; }
invalid() { log "INSTRUMENT-INVALID: $*"; exit 2; }

# ---- preconditions ---------------------------------------------------------
[ -f "$FRAMEWORK" ] || invalid "framework not found: $FRAMEWORK"
command -v mktemp >/dev/null 2>&1 || invalid "mktemp is required"
REAL_DATE="$(command -v date)"
[ -n "$REAL_DATE" ] || invalid "date is required"
export REAL_DATE

# ---- sandbox: a throwaway PROJECT_ROOT under $WORK, never the real repo ---
#
# Every real results folder this run touches lives under
# $WORK/sandbox/challenges/results — never challenges/results/ in the real
# checkout, which holds real historical evidence this guard must not
# disturb. The framework is symlinked in (not copied) so this exercises the
# SHIPPED script verbatim, matching protocol_grpc_fail_open_guard.sh's
# established convention.
SANDBOX="$WORK/sandbox"
mkdir -p "$SANDBOX/challenges/scripts"
ln -s "$FRAMEWORK" "$SANDBOX/challenges/scripts/challenge_framework.sh"

# ---- date shim: freezes ONLY the +%Y%m%d_%H%M%S call, per-invocation ------
#
# Real same-second collisions depend on winning a wall-clock race, which is
# exactly the flakiness §11.4.50 forbids leaning on. This PATH-shimmed
# `date` reads the value to freeze from $FROZEN_TIMESTAMP IN THE
# ENVIRONMENT AT CALL TIME (not baked into the shim script text), so the
# SAME shim serves both phases: phase 1 exports an identical value across
# three processes to force a collision; phase 2 exports two DIFFERENT,
# chronologically-later values across two processes to force NON-collision
# while still testing defect 2 in isolation. Every other date(1) invocation
# passes through to the real binary unchanged.
mkdir -p "$WORK/bin"
cat > "$WORK/bin/date" <<'DATE_SHIM_EOF'
#!/usr/bin/env bash
if [ "$*" = "+%Y%m%d_%H%M%S" ]; then
    printf '%s\n' "${FROZEN_TIMESTAMP:?FROZEN_TIMESTAMP not set for the date shim}"
else
    exec "${REAL_DATE:?REAL_DATE not set for the date shim}" "$@"
fi
DATE_SHIM_EOF
chmod +x "$WORK/bin/date"

# Validate the shim before trusting it (§11.4.201): it must freeze the exact
# call under test and pass everything else through.
SHIMMED="$(PATH="$WORK/bin:$PATH" FROZEN_TIMESTAMP="19700101_000000" date +%Y%m%d_%H%M%S)"
[ "$SHIMMED" = "19700101_000000" ] || invalid "date shim did not freeze +%Y%m%d_%H%M%S (got '$SHIMMED')"
PASSTHROUGH="$(PATH="$WORK/bin:$PATH" FROZEN_TIMESTAMP="19700101_000000" date +%s)"
case "$PASSTHROUGH" in
    ''|*[!0-9]*) invalid "date shim broke pass-through for other formats (date +%s gave '$PASSTHROUGH')" ;;
esac
log "date shim OK: +%Y%m%d_%H%M%S freezable per-invocation, other formats pass through"

# ---- one-role runner: sources the SHIPPED framework, drives one role ------
RUNNER="$SANDBOX/challenges/scripts/run_one_role.sh"
cat > "$RUNNER" <<'RUNNER_EOF'
#!/usr/bin/env bash
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$HERE/challenge_framework.sh"
set +e

: "${CID:?CID (challenge id) not set}"
init_challenge "$CID" "HXC-272 isolation guard: role=${ROLE:-unset}"

# Announce OUTPUT_DIR BEFORE calling finalize_challenge, not after.
# finalize_challenge deliberately re-enables `set -e` internally (see its
# own "Drainage report 2026-04-25 Finding #18" comment: it exists precisely
# so a non-PASSED status aborts the CALLER under `set -e`, the fix for
# standalone challenge scripts exiting 0 despite failed assertions). Once
# this harness's own `set +e` is overridden that way, a FAILED role's
# `finalize_challenge` call ends this process on the spot — by design, not
# by accident — so nothing scripted after it is guaranteed to run.
echo "OUTPUT_DIR_ANNOUNCED=$OUTPUT_DIR"

case "${ROLE:-}" in
    failing)
        record_assertion "probe" "one" "false" "deliberate failure"
        finalize_challenge "FAILED" >/dev/null 2>&1
        ;;
    passing)
        record_assertion "probe" "one"   "true" "ok"
        record_assertion "probe" "two"   "true" "ok"
        record_assertion "probe" "three" "true" "ok"
        finalize_challenge "PASSED" >/dev/null 2>&1
        ;;
    empty)
        # Achieves nothing: zero record_assertion calls.
        finalize_challenge "PASSED" >/dev/null 2>&1
        ;;
    *)
        echo "run_one_role.sh: unknown ROLE '${ROLE:-}'" >&2
        exit 2
        ;;
esac
RUNNER_EOF
chmod +x "$RUNNER"

# run_role <challenge-id> <role> <frozen-timestamp> — runs one challenge
# process, forced to the given TIMESTAMP, and echoes its announced
# OUTPUT_DIR.
run_role() {
    local cid="$1" role="$2" ts="$3" out
    out="$(PATH="$WORK/bin:$PATH" FROZEN_TIMESTAMP="$ts" REAL_DATE="$REAL_DATE" CID="$cid" ROLE="$role" bash "$RUNNER" 2>&1)"
    printf '%s' "$out" | sed -n 's/^OUTPUT_DIR_ANNOUNCED=//p' | tail -1
}

results_json_of() { cat "$1/results/$2_results.json" 2>/dev/null; }
field() {
    # field <json> <key> — pulls an integer field out of the flat JSON
    # finalize_challenge emits, without a jq dependency (matching the
    # framework's own "no jq dependency" convention).
    printf '%s\n' "$1" | grep -oE "\"$2\": *[0-9]+" | grep -oE '[0-9]+$' | head -1
}

# =============================================================================
# PHASE 1 — same-second collision, both directions (defect 1)
# =============================================================================
CID1="hxc272-iso-test"
TS1="20260101_000000"

log ""
log "=== PHASE 1: failing -> passing -> empty, all forced to TIMESTAMP=$TS1 ==="
DIR_FAILING="$(run_role "$CID1" failing "$TS1")"
[ -n "$DIR_FAILING" ] || invalid "failing role announced no OUTPUT_DIR"
DIR_PASSING="$(run_role "$CID1" passing "$TS1")"
[ -n "$DIR_PASSING" ] || invalid "passing role announced no OUTPUT_DIR"
DIR_EMPTY="$(run_role "$CID1" empty "$TS1")"
[ -n "$DIR_EMPTY" ] || invalid "empty role announced no OUTPUT_DIR"

log "  failing -> $DIR_FAILING"
log "  passing -> $DIR_PASSING"
log "  empty   -> $DIR_EMPTY"

JSON_FAILING="$(results_json_of "$DIR_FAILING" "$CID1")"
JSON_PASSING="$(results_json_of "$DIR_PASSING" "$CID1")"
JSON_EMPTY="$(results_json_of "$DIR_EMPTY" "$CID1")"
[ -n "$JSON_FAILING" ] || invalid "no results.json produced for the failing role"
[ -n "$JSON_PASSING" ] || invalid "no results.json produced for the passing role"
[ -n "$JSON_EMPTY" ]   || invalid "no results.json produced for the empty role"

PASSED_FAILING="$(field "$JSON_FAILING" assertions_passed)"
FAILED_FAILING="$(field "$JSON_FAILING" assertions_failed)"
PASSED_PASSING="$(field "$JSON_PASSING" assertions_passed)"
FAILED_PASSING="$(field "$JSON_PASSING" assertions_failed)"
PASSED_EMPTY="$(field "$JSON_EMPTY" assertions_passed)"
FAILED_EMPTY="$(field "$JSON_EMPTY" assertions_failed)"

log ""
log "  failing results.json: passed=$PASSED_FAILING failed=$FAILED_FAILING"
log "  passing results.json: passed=$PASSED_PASSING failed=$FAILED_PASSING"
log "  empty   results.json: passed=$PASSED_EMPTY failed=$FAILED_EMPTY"
log ""

DIRS_COLLIDE=0
[ "$DIR_FAILING" = "$DIR_PASSING" ] && DIRS_COLLIDE=1
[ "$DIR_PASSING" = "$DIR_EMPTY" ] && DIRS_COLLIDE=1
[ "$DIR_FAILING" = "$DIR_EMPTY" ] && DIRS_COLLIDE=1

# =============================================================================
# PHASE 2 — stale "latest" pointer + stale by-recency listing, in isolation
# (defect 2). Two DIFFERENT, chronologically-LATER timestamps so phase 2
# does not depend on (or get confused with) phase 1's collision behaviour —
# even pre-fix these two land in DIFFERENT directories, isolating defect 2's
# own nested-symlink mechanism.
# =============================================================================
CID2="hxc272-iso-test-seq"
TS2A="20260101_000010"
TS2B="20260101_000020"

log ""
log "=== PHASE 2: two DIFFERENT-timestamp runs of a second challenge ID ==="
log "    (isolating the latest-pointer defect from the collision defect)"
DIR2_FIRST="$(run_role "$CID2" passing "$TS2A")"
[ -n "$DIR2_FIRST" ] || invalid "phase-2 first run announced no OUTPUT_DIR"
DIR2_SECOND="$(run_role "$CID2" passing "$TS2B")"
[ -n "$DIR2_SECOND" ] || invalid "phase-2 second run announced no OUTPUT_DIR"

log "  first  (TS=$TS2A) -> $DIR2_FIRST"
log "  second (TS=$TS2B) -> $DIR2_SECOND"

[ "$DIR2_FIRST" != "$DIR2_SECOND" ] || \
    invalid "phase 2's two runs landed in the SAME directory despite different forced" \
            "timestamps — the harness itself is broken, not the framework"

LATEST2_LINK="$SANDBOX/challenges/results/${CID2}/latest"
LATEST2_TARGET=""
[ -L "$LATEST2_LINK" ] && LATEST2_TARGET="$(readlink "$LATEST2_LINK")"

# The pre-fix nested-symlink shape: the SECOND run's own basename appears as
# an entry INSIDE the FIRST run's directory, because `ln -sf` dereferenced
# `latest` (pointing at the first run) and dropped the new link there.
NESTED_ENTRY="$DIR2_FIRST/$(basename "$DIR2_SECOND")"

# ---- verdicts ---------------------------------------------------------------
if [ "$RED_MODE" = "1" ]; then
    log ""
    log "RED_MODE=1: expecting the pre-fix collision (phase 1) AND the pre-fix" \
        "stale-pointer/nested-symlink shape (phase 2)"

    PHASE1_RED=0
    [ "$DIRS_COLLIDE" = "1" ] && [ "${PASSED_EMPTY:-0}" -ge 1 ] && PHASE1_RED=1

    PHASE2_RED=0
    [ "$LATEST2_TARGET" = "$DIR2_FIRST" ] && [ -e "$NESTED_ENTRY" ] && PHASE2_RED=1

    if [ "$PHASE1_RED" = "1" ] && [ "$PHASE2_RED" = "1" ]; then
        log "RED CONFIRMED (phase 1): results folders collided (three same-second runs"
        log "               shared a directory) and the empty run's own results.json"
        log "               reports assertions_passed=$PASSED_EMPTY despite recording"
        log "               nothing — the dangerous false-success direction."
        log "RED CONFIRMED (phase 2): 'latest' still names the FIRST run ($DIR2_FIRST)"
        log "               and the SECOND run's pointer was dropped INSIDE it at"
        log "               $NESTED_ENTRY — exactly the nested-symlink shape reported."
        exit 0
    fi
    fail "RED_MODE=1 but the pre-fix shape was not fully observed" \
         "(phase1_red=$PHASE1_RED phase2_red=$PHASE2_RED — already fixed, or this" \
         "guard is looking in the wrong place)"
fi

log ""
log "RED_MODE=0: expecting full isolation (phase 1) and a correctly-repointed" \
    "latest pointer with an untouched first-run directory (phase 2)"

# --- phase 1 assertions ------------------------------------------------------
if [ "$DIRS_COLLIDE" = "1" ]; then
    fail "two same-second runs of the SAME challenge ID shared a results folder:" \
         "failing=$DIR_FAILING passing=$DIR_PASSING empty=$DIR_EMPTY"
fi

# Direction 1 (wasteful-but-safe-side): a neighbour's real failure must not
# leak into another run's own verdict.
if [ "${FAILED_PASSING:-0}" != "0" ]; then
    fail "the PASSING run's own results.json reports assertions_failed=$FAILED_PASSING;" \
         "it inherited the FAILING neighbour's failure through a shared log" \
         "(Direction 1 — inherited FAILURE lines flip a healthy run red)"
fi

# Direction 2 (the DANGEROUS one): a run that recorded nothing must never
# report a neighbour's successes as its own.
if [ "${PASSED_EMPTY:-0}" != "0" ]; then
    fail "the EMPTY run recorded ZERO assertions but its own results.json reports" \
         "assertions_passed=$PASSED_EMPTY; it inherited a neighbour's successes through" \
         "a shared log — a run that achieved nothing reported a FALSE SUCCESS" \
         "(Direction 2 — the false-success vector §11.4.5/§11.4.69 forbid)"
fi
if [ "${FAILED_EMPTY:-0}" != "0" ]; then
    fail "the EMPTY run recorded ZERO assertions but its own results.json reports" \
         "assertions_failed=$FAILED_EMPTY; it inherited a neighbour's failure through a" \
         "shared log"
fi

# Sanity: each run's OWN work is still counted correctly (not merely zeroed
# out by an over-eager fix).
[ "${PASSED_FAILING:-0}" = "0" ] || fail "failing role unexpectedly recorded a PASSED assertion"
[ "${FAILED_FAILING:-0}" = "1" ] || fail "failing role's own failure was not counted (failed=$FAILED_FAILING)"
[ "${PASSED_PASSING:-0}" = "3" ] || fail "passing role's own three successes were not counted (passed=$PASSED_PASSING)"

log "phase 1 OK: each of the three same-second runs kept its own, uncontaminated verdict"

# "empty" ran last in phase 1, so BOTH consumers of "the most recent run"
# must agree it is the newest: the maintained symlink, and an independent
# recency-by-mtime listing over the run directories (the SECOND consumer of
# the same broken state per the item — a fix that only repairs the pointer
# while mtime-ordering still returns the first run leaves half the defect
# live).
LATEST_LINK="$SANDBOX/challenges/results/${CID1}/latest"
[ -L "$LATEST_LINK" ] || fail "no 'latest' symlink was created at $LATEST_LINK"
LATEST_TARGET="$(readlink "$LATEST_LINK")"
[ "$LATEST_TARGET" = "$DIR_EMPTY" ] || \
    fail "'latest' points at '$LATEST_TARGET', not the genuinely most recent run '$DIR_EMPTY'"
log "phase 1 latest pointer OK: names the genuinely most recent run ($DIR_EMPTY)"

# Exclude "latest" itself from the glob (it is a symlink-to-directory and
# would otherwise appear in a trailing-slash directory glob too) by matching
# only run directories, which all start with the numeric TIMESTAMP.
NEWEST_BY_MTIME="$(cd "$SANDBOX/challenges/results/${CID1}" && ls -td [0-9]*/ 2>/dev/null | head -1)"
NEWEST_BY_MTIME="${NEWEST_BY_MTIME%/}"
NEWEST_BY_MTIME_ABS="$SANDBOX/challenges/results/${CID1}/$NEWEST_BY_MTIME"
[ "$NEWEST_BY_MTIME_ABS" = "$DIR_EMPTY" ] || \
    fail "a recency-by-mtime listing (the CATALOG.md 'find latest results directory'" \
         "recipe) resolves to '$NEWEST_BY_MTIME_ABS', not the genuinely most recent run" \
         "'$DIR_EMPTY' — the pointer consumer and the by-recency-listing consumer must" \
         "BOTH agree"
log "phase 1 by-recency listing OK: ls -td also names the genuinely most recent run"

# --- phase 2 assertions ------------------------------------------------------
[ -L "$LATEST2_LINK" ] || fail "phase 2: no 'latest' symlink was created at $LATEST2_LINK"
[ "$LATEST2_TARGET" = "$DIR2_SECOND" ] || \
    fail "phase 2: 'latest' points at '$LATEST2_TARGET', not the second (chronologically" \
         "later) run '$DIR2_SECOND' — the pointer never advanced past the first run"
log "phase 2 latest pointer OK: advanced from the first run to the second"

[ ! -e "$NESTED_ENTRY" ] || \
    fail "phase 2: the second run's pointer was dropped INSIDE the first run's own" \
         "directory at '$NESTED_ENTRY' — the exact nested-symlink shape the item" \
         "describes ('later runs' pointers nested inside it')"
log "phase 2 OK: the first run's directory received no nested entry from the second run"

NEWEST2_BY_MTIME="$(cd "$SANDBOX/challenges/results/${CID2}" && ls -td [0-9]*/ 2>/dev/null | head -1)"
NEWEST2_BY_MTIME="${NEWEST2_BY_MTIME%/}"
NEWEST2_BY_MTIME_ABS="$SANDBOX/challenges/results/${CID2}/$NEWEST2_BY_MTIME"
[ "$NEWEST2_BY_MTIME_ABS" = "$DIR2_SECOND" ] || \
    fail "phase 2: a recency-by-mtime listing resolves to '$NEWEST2_BY_MTIME_ABS', not" \
         "the genuinely most recent run '$DIR2_SECOND'"
log "phase 2 by-recency listing OK: ls -td also names the genuinely most recent run"

log ""
log "GUARD PASS: phase 1 — three same-second runs stayed fully isolated (verdicts"
log "            independent in both directions); phase 2 — the latest pointer and"
log "            a by-mtime listing both advanced to the genuinely newest run, and"
log "            the first run's own directory was left untouched."
exit 0
