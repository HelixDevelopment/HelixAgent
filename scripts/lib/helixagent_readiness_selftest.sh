#!/bin/bash
# Self-test for scripts/lib/helixagent_readiness.sh.
#
# Run directly, or via `make selftest-readiness` (which `make ci-validate-all`
# invokes). Exits 0 when the instrument is proven to discriminate, non-zero
# otherwise.
#
# ---------------------------------------------------------------------------
# WHY THIS EXISTS (§11.4.107(10) — an analyzer that cannot be shown to
# discriminate is not evidence; §11.4.224 — the test-first floor covers bash)
# ---------------------------------------------------------------------------
#
# helixagent_readiness.sh decides whether the complete-validation runner is
# allowed to believe its own results. It shipped with no automated
# self-validation, and three separate one-line mutations were measured on
# 2026-08-09 to invert user-visible verdicts while NOTHING in the repository
# failed and shellcheck stayed silent:
#
#   M-A  delete the identity grep         -> an unidentified 200-responder
#                                            (a mock-LLM-shaped
#                                            {"status":"healthy"}) is blessed
#                                            as a ready HelixAgent
#   M-B  pid-match -> process-NAME match  -> helix_wait_ready(pid_of_A) blessed
#                                            process B's server. This is
#                                            precisely the §11.4.174
#                                            cross-process mis-attribution the
#                                            pid-match exists to prevent.
#   M-C  count SKIPs as executed          -> the all-skipped run is accepted,
#                                            i.e. the exact false-green the
#                                            helper was written to kill
#
# So the file has two halves, and BOTH are required (§11.4.107(10)):
#
#   GOLDEN-GOOD  the 9 probes below, run against the real helper. They must
#                all produce the expected verdict.
#   GOLDEN-BAD   each mutation is applied to a COPY of the helper and the
#                probe that is supposed to catch it is re-run. The mutant MUST
#                produce the wrong verdict. A probe that still returns the
#                right answer under its mutant is blind, and a blind probe is
#                reported as a FAILURE here — not silently counted as a pass.
#
# The golden-bad half is also a drift detector: each mutation is located by a
# fixed string in the helper. If the helper is edited so a needle no longer
# matches, that is a FAILURE, not a skip — the self-test refuses to claim it
# validated a mutation it could not apply.
#
# ---------------------------------------------------------------------------
# FIXTURES (§12 host safety — nothing here touches a live service)
# ---------------------------------------------------------------------------
#
# Three loopback HTTP fixtures are started, each binding port 0 so the KERNEL
# allocates an ephemeral port. No port is hardcoded, so a run can never
# collide with the operator's live helixagent (:7061) or llm-verifier (:8100),
# and two concurrent runs of this file cannot collide with each other.
# Fixtures are reaped BY PID; `pkill -f` is never used because its pattern
# matches this script's own argv (§11.4.174, §11.4.201).
#
#   identity  200 + {"service":"helixagent","status":"healthy"}  -> ready
#   squatter  404 "404 page not found"                           -> the
#             foreign responder that satisfies bare `curl -s`
#   anon200   200 + {"status":"healthy"}, no service field       -> reachable
#             but unidentified; the M-A trap

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPER="$SCRIPT_DIR/helixagent_readiness.sh"

PASS_COUNT=0
FAIL_COUNT=0
FIXTURE_PIDS=()
WORK=""

# --------------------------------------------------------------------------
# Reporting
# --------------------------------------------------------------------------

report() {
    # report <verdict> <label> <detail>
    local verdict="$1" label="$2" detail="$3"
    if [ "$verdict" = "PASS" ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        printf '  PASS  %-46s %s\n' "$label" "$detail"
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        printf '  FAIL  %-46s %s\n' "$label" "$detail"
    fi
}

expect() {
    # expect <label> <expected> <actual>
    local label="$1" want="$2" got="$3"
    if [ "$want" = "$got" ]; then
        report PASS "$label" "got: $got"
    else
        report FAIL "$label" "want: $want | got: $got"
    fi
}

die() {
    echo "helixagent_readiness_selftest: $*" >&2
    exit 1
}

cleanup() {
    local pid
    for pid in "${FIXTURE_PIDS[@]:-}"; do
        [ -n "$pid" ] || continue
        # By PID only. A pattern kill would match this script's own argv.
        kill "$pid" 2> /dev/null || true
    done
    for pid in "${FIXTURE_PIDS[@]:-}"; do
        [ -n "$pid" ] || continue
        wait "$pid" 2> /dev/null || true
    done
    [ -n "$WORK" ] && [ -d "$WORK" ] && rm -rf "$WORK"
}
trap cleanup EXIT

# --------------------------------------------------------------------------
# Preconditions — a gate that cannot run has NOT passed (§11.4.6)
# --------------------------------------------------------------------------

[ -f "$HELPER" ] || die "helper not found at $HELPER"

for tool in python3 ss curl awk sed grep tr; do
    command -v "$tool" > /dev/null 2>&1 || die "required tool '$tool' is not on PATH — cannot run"
done

# `ss` is a hard requirement, not a nicety. Without it helix_wait_ready falls
# back to \${PORT:-7061} — which on this host is the operator's LIVE
# helixagent. Probing that would both touch a live service and let the
# squatter/anon probes pass for the wrong reason.

# shellcheck source=./helixagent_readiness.sh
source "$HELPER"

WORK="$(mktemp -d)" || die "mktemp failed"

# --------------------------------------------------------------------------
# Fixture servers
# --------------------------------------------------------------------------

cat > "$WORK/fixture_server.py" << 'PYEOF'
import http.server, json, sys

mode = sys.argv[1]  # identity | squatter | anon200


class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if mode == "squatter":
            body = b"404 page not found\n"
            self.send_response(404)
        elif mode == "anon200":
            body = json.dumps({"status": "healthy"}).encode()
            self.send_response(200)
        else:
            body = json.dumps({"service": "helixagent", "status": "healthy"}).encode()
            self.send_response(200)
        self.send_header("Content-Type", "text/plain" if mode == "squatter" else "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *a):
        pass


# Port 0: the kernel picks a free ephemeral port. Never a fixed port, so this
# cannot collide with a live service or with a concurrent run of this file.
srv = http.server.HTTPServer(("127.0.0.1", 0), H)
print(srv.server_address[1], flush=True)
srv.serve_forever()
PYEOF

# Results land in these globals rather than on stdout. Returning them through
# a command substitution would run the whole function in a SUBSHELL, so the
# `FIXTURE_PIDS+=` below would be discarded on subshell exit and cleanup would
# reap nothing — measured while proving this file: three runs leaked nine
# fixture servers, and a leaked identity fixture then answered a later run's
# M-B probe and made it report a false FAIL (§11.4.14 cleanup, §11.4.174
# ownership).
FIXTURE_PID=""
FIXTURE_PORT=""

start_fixture() {
    # start_fixture <mode> -> sets FIXTURE_PID / FIXTURE_PORT
    local mode="$1"
    local portfile="$WORK/$mode.port"
    python3 "$WORK/fixture_server.py" "$mode" > "$portfile" 2> "$WORK/$mode.err" &
    local pid=$!
    FIXTURE_PIDS+=("$pid")

    local i port=""
    for (( i = 0; i < 100; i++ )); do
        if [ -s "$portfile" ]; then
            port="$(tr -d '[:space:]' < "$portfile")"
            [ -n "$port" ] && break
        fi
        kill -0 "$pid" 2> /dev/null || die "fixture '$mode' died before binding: $(cat "$WORK/$mode.err")"
        sleep 0.1
    done
    [ -n "$port" ] || die "fixture '$mode' never reported a port"
    FIXTURE_PID="$pid"
    FIXTURE_PORT="$port"
}

start_fixture identity
ID_PID="$FIXTURE_PID"
ID_PORT="$FIXTURE_PORT"
start_fixture squatter
SQ_PID="$FIXTURE_PID"
SQ_PORT="$FIXTURE_PORT"
start_fixture anon200
AN_PID="$FIXTURE_PID"
AN_PORT="$FIXTURE_PORT"

ID_URL="http://localhost:${ID_PORT}"

echo "helixagent_readiness.sh self-test"
echo "  helper   : $HELPER"
echo "  fixtures : identity pid=$ID_PID port=$ID_PORT | squatter pid=$SQ_PID port=$SQ_PORT | anon200 pid=$AN_PID port=$AN_PORT"
echo ""

# PORT is pinned to the squatter for the ss-backed probes. With `ss` present
# the fallback branch is unreachable, so this is belt-and-braces: if `ss` ever
# vanishes mid-run, probe (a) fails loudly instead of the run silently
# probing — or worse, blessing — the operator's live :7061.
export PORT="$SQ_PORT"

# --------------------------------------------------------------------------
# Test-log fixtures for the executed-count assertions
# --------------------------------------------------------------------------

printf -- '--- SKIP: TestA (0.00s)\n--- SKIP: TestB (0.00s)\n--- SKIP: TestC (0.00s)\n' \
    > "$WORK/log_allskip.txt"
printf -- '--- PASS: TestA (0.01s)\n    --- PASS: TestA/sub (0.00s)\n--- FAIL: TestB (0.01s)\n--- SKIP: TestC (0.00s)\n' \
    > "$WORK/log_mixed.txt"

# --------------------------------------------------------------------------
# Minimal PATH for the ss-absent fallback probes
# --------------------------------------------------------------------------

FAKEBIN="$WORK/fakebin"
mkdir -p "$FAKEBIN"
for t in curl grep awk sed sort tr sleep cat; do
    src="$(command -v "$t" 2> /dev/null)" || continue
    ln -sf "$src" "$FAKEBIN/$t"
done
[ -x "$FAKEBIN/curl" ] || die "could not stage curl into the fallback PATH"
[ -e "$FAKEBIN/ss" ] && die "fallback PATH must not contain ss"

# --------------------------------------------------------------------------
# Probes. Each echoes "<rc>|<stdout>" so a verdict and its URL are one value.
# --------------------------------------------------------------------------

probe_wait_ready() {
    # probe_wait_ready <pid> <timeout>
    local out rc
    out="$(helix_wait_ready "$1" "$2" 2> /dev/null)"
    rc=$?
    echo "${rc}|${out}"
}

probe_wait_ready_no_ss() {
    # probe_wait_ready_no_ss <pid> <timeout> <PORT>
    (
        PATH="$FAKEBIN"
        PORT="$3"
        hash -r
        export PATH PORT
        out="$(helix_wait_ready "$1" "$2" 2> /dev/null)"
        rc=$?
        echo "${rc}|${out}"
    )
}

probe_assert_executed() {
    # probe_assert_executed <log>
    helix_assert_tests_executed "$1" selftest 2> /dev/null
    echo "$?"
}

# --------------------------------------------------------------------------
# GOLDEN-GOOD — the real helper must produce every expected verdict
# --------------------------------------------------------------------------

echo "GOLDEN-GOOD (real helper, 9 probes)"

expect "a. wait_ready(identity) is ready + echoes URL" \
    "0|${ID_URL}" "$(probe_wait_ready "$ID_PID" 10)"

expect "b. wait_ready(squatter 404) is NOT ready" \
    "1|" "$(probe_wait_ready "$SQ_PID" 3)"

expect "c. wait_ready(anon 200, no identity) is NOT ready" \
    "1|" "$(probe_wait_ready "$AN_PID" 3)"

expect "d1. ss-absent fallback, PORT=squatter -> NOT ready" \
    "1|" "$(probe_wait_ready_no_ss "$SQ_PID" 2 "$SQ_PORT")"

expect "d2. ss-absent fallback, PORT=identity -> ready" \
    "0|${ID_URL}" "$(probe_wait_ready_no_ss "$ID_PID" 3 "$ID_PORT")"

expect "e1. assert_tests_executed(all-skip) rejects" \
    "1" "$(probe_assert_executed "$WORK/log_allskip.txt")"

expect "e2. assert_tests_executed(mixed) accepts" \
    "0" "$(probe_assert_executed "$WORK/log_mixed.txt")"

expect "e3. count_executed_tests(mixed) counts PASS+FAIL only" \
    "3" "$(helix_count_executed_tests "$WORK/log_mixed.txt")"

expect "e4. count_executed_tests(missing file) is 0" \
    "0" "$(helix_count_executed_tests "$WORK/does_not_exist.txt")"

# --------------------------------------------------------------------------
# GOLDEN-BAD — each mutation must be CAUGHT by the probe that guards it
# --------------------------------------------------------------------------

mutate_helper() {
    # mutate_helper <dst> <needle> <replacement-line>
    # Locates <needle> by fixed string and replaces that whole line. A needle
    # that no longer matches is a hard failure: the helper drifted and this
    # mutation is no longer validating anything.
    local dst="$1" needle="$2" replacement="$3" line
    line="$(grep -nF -- "$needle" "$HELPER" | head -1 | cut -d: -f1)"
    if [ -z "$line" ]; then
        return 1
    fi
    REPLACEMENT="$replacement" awk -v n="$line" \
        'NR == n { print ENVIRON["REPLACEMENT"]; next } { print }' "$HELPER" > "$dst"
    # The mutant must actually differ, otherwise the control is vacuous.
    ! cmp -s "$HELPER" "$dst"
}

echo ""
echo "GOLDEN-BAD (mutants must be caught — a probe that still passes is blind)"

# M-A: delete the identity grep. helix_identifies_as_helixagent then accepts
# any HTTP success, so the anon-200 fixture — reachable, unidentified — is
# blessed as a ready HelixAgent. Probe (c) must flip 1 -> 0.
MUT_A="$WORK/mutant_A.sh"
if mutate_helper "$MUT_A" 'printf '"'"'%s'"'"' "$body" | grep -q' \
    '    return 0  # MUTANT M-A: identity grep deleted'; then
    got="$(
        # shellcheck source=/dev/null  # mutant copy, generated at run time
        source "$MUT_A"
        out="$(helix_wait_ready "$AN_PID" 3 2> /dev/null)"
        echo "$?|${out}"
    )"
    if [ "${got%%|*}" = "0" ]; then
        report PASS "M-A caught: anon-200 wrongly blessed by mutant" "mutant rc=0 (real helper rc=1)"
    else
        report FAIL "M-A NOT caught — probe (c) is blind" "mutant rc=${got%%|*}, expected 0"
    fi
else
    report FAIL "M-A could not be applied" "needle absent from helper — helper drifted or already mutated"
fi

# M-B: pid-match -> process-NAME match in helix_listen_ports_for_pid. Asked
# about the squatter's pid, the mutant returns every python3 listener's port,
# including the identity fixture's, and blesses a DIFFERENT process's server.
# Probe (b) must flip 1 -> 0 AND hand back a URL that is not the squatter's
# own port — the §11.4.174 cross-process mis-attribution made visible.
#
# The assertion deliberately does NOT demand the identity fixture's exact
# port: any other python3 listener on the host that answers an identifying
# /health would satisfy the mutant first, and that is the same violation.
# Pinning the exact URL made this control report a false FAIL when an
# unrelated python3 listener won the race.
MUT_B="$WORK/mutant_B.sh"
# NOTE: the replacement must keep the trailing backslash as its LAST
# character — it is a pipeline continuation. A trailing comment after it
# would escape a space instead and break the pipeline.
if mutate_helper "$MUT_B" '| grep -F "pid=${pid}," \' \
    '        | grep -F "python3" \'; then
    got="$(
        # shellcheck source=/dev/null  # mutant copy, generated at run time
        source "$MUT_B"
        out="$(helix_wait_ready "$SQ_PID" 5 2> /dev/null)"
        echo "$?|${out}"
    )"
    got_rc="${got%%|*}"
    got_url="${got#*|}"
    if [ "$got_rc" = "0" ] && [ -n "$got_url" ] && [ "$got_url" != "http://localhost:${SQ_PORT}" ]; then
        report PASS "M-B caught: squatter pid blessed by foreign port" "mutant returned $got_url for pid $SQ_PID (own port $SQ_PORT)"
    else
        report FAIL "M-B NOT caught — probe (b) is blind" "want rc=0 + a URL other than http://localhost:${SQ_PORT}, got $got"
    fi
else
    report FAIL "M-B could not be applied" "needle absent from helper — helper drifted or already mutated"
fi

# M-C: count SKIP as executed. The all-skipped run — go test exit 0, zero
# assertions — is then accepted as a suite that ran. Probe (e1) must flip
# 1 -> 0.
MUT_C="$WORK/mutant_C.sh"
if mutate_helper "$MUT_C" "--- (PASS|FAIL):" \
    "    count=\"\$(grep -cE '^[[:space:]]*--- (PASS|FAIL|SKIP):' \"\$log\" 2>/dev/null || true)\"  # MUTANT M-C"; then
    got="$(
        # shellcheck source=/dev/null  # mutant copy, generated at run time
        source "$MUT_C"
        helix_assert_tests_executed "$WORK/log_allskip.txt" selftest 2> /dev/null
        echo "$?"
    )"
    if [ "$got" = "0" ]; then
        report PASS "M-C caught: all-skip run accepted by mutant" "mutant rc=0 (real helper rc=1)"
    else
        report FAIL "M-C NOT caught — probe (e1) is blind" "mutant rc=$got, expected 0"
    fi
else
    report FAIL "M-C could not be applied" "needle absent from helper — helper drifted or already mutated"
fi

# --------------------------------------------------------------------------

echo ""
echo "─────────────────────────────────────────────────────────────────"
if [ "$FAIL_COUNT" -eq 0 ]; then
    echo "helixagent_readiness.sh self-test: PASS ($PASS_COUNT checks, 0 failures)"
    exit 0
fi
echo "helixagent_readiness.sh self-test: FAIL ($FAIL_COUNT of $((PASS_COUNT + FAIL_COUNT)) checks failed)"
exit 1
