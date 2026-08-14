#!/usr/bin/env bash
# =============================================================================
# HXC-334 regression guard for docker/mcp/mcp_healthcheck.sh
# =============================================================================
# Constitution anchors: §11.4.115 (RED-baseline-on-the-broken-artifact +
# polarity switch), §11.4.135 (standing regression guard registered in the same
# commit as the fix), §1.1 (paired mutation), §11.4.50 (determinism),
# §11.4.69 (captured evidence path per verdict).
#
# ONE SOURCE, TWO ROLES — flip with RED_MODE:
#
#   RED_MODE=1  (default)  reproduce BOTH historical defects on the REAL broken
#                          artifacts and assert the defects are PRESENT. This is
#                          the proof the guard is not blind: if these assertions
#                          do not fire, the guard cannot be trusted to catch a
#                          regression either.
#   RED_MODE=0             assert both defects are ABSENT from the CURRENT
#                          artifact. This is the standing GREEN guard.
#
# THE TWO DEFECTS GUARDED
#   D1 socket-liveness-is-not-server-liveness (the shipped `nc -z` check).
#      These images run `socat TCP-LISTEN:9000,reuseaddr,fork SYSTEM:"$CMD"`, so
#      socat accepts the TCP connection whether or not $CMD works. `nc -z`
#      therefore reports healthy for a server that is silent on every connection
#      and for one that prints CLI usage text instead of speaking MCP.
#   D2 interpreter-dependent verdict (the first-attempt fix, frozen in
#      testdata/legacy_mcp_healthcheck.pre_hxc334_round2.sh). Its `nc` branch
#      validated with three FLAT greps while its node/python3 branches parsed
#      `result.protocolVersion` structurally, so a reply carrying
#      `protocolVersion` at TOP level got opposite verdicts from the same server
#      depending on which interpreter the image shipped.
#
# USAGE
#   ./mcp_healthcheck_test.sh                 # RED (defects present on legacy)
#   RED_MODE=0 ./mcp_healthcheck_test.sh      # GREEN guard (standing)
#   ./mcp_healthcheck_test.sh --prove-mutation  # §1.1: `exit 0` must kill it
#
# This test binds only to 127.0.0.1 on an ephemeral port and speaks to its own
# python fixture server. It NEVER touches a container, a real MCP server, a
# provider, or any credential.
# =============================================================================

set -uo pipefail

RED_MODE=${RED_MODE:-1}
PROVE_MUTATION=0
[ "${1:-}" = "--prove-mutation" ] && PROVE_MUTATION=1

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HC="${MCP_HC_SCRIPT:-$SCRIPT_DIR/mcp_healthcheck.sh}"
LEGACY="$SCRIPT_DIR/testdata/legacy_mcp_healthcheck.pre_hxc334_round2.sh"
PROBE_TIMEOUT="${MCP_HC_TEST_TIMEOUT:-2}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE_DIR="${MCP_HC_EVIDENCE_DIR:-$SCRIPT_DIR/../../qa-results/mcp_healthcheck/$TS}"
mkdir -p "$EVIDENCE_DIR"
MATRIX_TSV="$EVIDENCE_DIR/fixture_matrix.tsv"
SUMMARY="$EVIDENCE_DIR/summary.txt"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
PASSED=0; FAILED=0; SKIPPED=0

pass()    { PASSED=$((PASSED+1)); echo -e "  ${GREEN}[PASS]${NC} $1"; }
fail()    { FAILED=$((FAILED+1)); echo -e "  ${RED}[FAIL]${NC} $1"; }
skipmsg() { SKIPPED=$((SKIPPED+1)); echo -e "  ${YELLOW}[SKIP]${NC} $1"; }
section() { echo -e "\n${BLUE}=== $1 ===${NC}"; }

TMPROOT="$(mktemp -d)"
FIXTURE_PID=""
cleanup() {
    if [ -n "$FIXTURE_PID" ] && kill -0 "$FIXTURE_PID" 2>/dev/null; then
        kill "$FIXTURE_PID" 2>/dev/null
        wait "$FIXTURE_PID" 2>/dev/null
    fi
    rm -rf "$TMPROOT"
}
trap cleanup EXIT INT TERM

# -----------------------------------------------------------------------------
# Fixture server: accepts on an ephemeral loopback port and replies with one
# canned payload per connection (or accepts-then-closes for the silent case,
# which is exactly what socat does in front of a dead server).
# -----------------------------------------------------------------------------
cat >"$TMPROOT/fixture_server.py" <<'PYEOF'
import os, socket, sys, threading, time
mode = sys.argv[1]
delay = float(os.environ.get("FIXTURE_DELAY", "0"))
payload = os.environ.get("FIXTURE_PAYLOAD", "")
srv = socket.socket()
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("127.0.0.1", 0))
srv.listen(16)
sys.stdout.write("%d\n" % srv.getsockname()[1])
sys.stdout.flush()

def serve(conn):
    try:
        conn.settimeout(2)
        try:
            conn.recv(65536)
        except Exception:
            pass
        if mode == "silent":
            return
        if mode == "eofkill":
            # Emulate `socat TCP-LISTEN:...,fork SYSTEM:"$CMD"` in front of a
            # server that needs a moment to answer: the instant the CLIENT
            # half-closes its write side, socat tears the connection down and
            # the in-flight reply is lost. A client that keeps its write side
            # open (node/python3) gets the reply; one that sends-then-EOFs
            # (printf | nc) loses the race. This is the real mcp-redis
            # behaviour, reproduced deterministically.
            conn.settimeout(delay if delay > 0 else 1.5)
            try:
                if conn.recv(1) == b"":
                    return  # client half-closed -> tear down, no reply
            except Exception:
                pass  # client kept the socket open -> fall through and answer
        elif delay > 0:
            time.sleep(delay)
        try:
            conn.sendall((payload + "\n").encode())
        except Exception:
            pass
    finally:
        try:
            conn.close()
        except Exception:
            pass

while True:
    try:
        c, _ = srv.accept()
    except Exception:
        break
    threading.Thread(target=serve, args=(c,), daemon=True).start()
PYEOF

start_fixture() {  # $1=payload; "__SILENT__" = accept-then-close; "__SLOW__<json>" = reply after a delay
    local payload="$1" mode="reply" delay="0"
    [ "$payload" = "__SILENT__" ] && { mode="silent"; payload=""; }
    case "$payload" in
        __SLOW__*)    delay="${MCP_HC_TEST_SLOW_DELAY:-1.5}"; payload="${payload#__SLOW__}" ;;
        __EOFKILL__*) mode="eofkill"; delay="${MCP_HC_TEST_SLOW_DELAY:-1.5}"; payload="${payload#__EOFKILL__}" ;;
    esac
    local portfile="$TMPROOT/port.$$"
    FIXTURE_PAYLOAD="$payload" FIXTURE_DELAY="$delay" python3 "$TMPROOT/fixture_server.py" "$mode" >"$portfile" 2>"$TMPROOT/fixture.err" &
    FIXTURE_PID=$!
    local i=0
    FIXTURE_PORT=""
    while [ $i -lt 100 ]; do
        FIXTURE_PORT="$(head -n1 "$portfile" 2>/dev/null)"
        [ -n "$FIXTURE_PORT" ] && break
        sleep 0.05; i=$((i+1))
    done
    [ -n "$FIXTURE_PORT" ]
}

stop_fixture() {
    if [ -n "$FIXTURE_PID" ] && kill -0 "$FIXTURE_PID" 2>/dev/null; then
        kill "$FIXTURE_PID" 2>/dev/null
        wait "$FIXTURE_PID" 2>/dev/null
    fi
    FIXTURE_PID=""
}

# -----------------------------------------------------------------------------
# PATH-isolation sandbox. The legacy artifact has no implementation selector, so
# the only way to force each of its branches is to hide the tools it prefers.
# The sandbox is a bin dir of symlinks; scripts run under `env -i` so no shell
# function (e.g. a `grep` alias/function shadowing the real binary) can leak in.
# -----------------------------------------------------------------------------
SANDBOX_BASE="$TMPROOT/sandbox"
build_sandbox() {  # $@ = interpreter names to EXPOSE (nc/node/python3)
    local name="$1"; shift
    local bin="$SANDBOX_BASE/$name/bin"
    rm -rf "$SANDBOX_BASE/$name"; mkdir -p "$bin"
    local t p
    for t in grep head sed cat tr env; do
        p="$(command -v "$t" 2>/dev/null)"
        [ -n "$p" ] && ln -sf "$p" "$bin/$t"
    done
    for t in "$@"; do
        p="$(command -v "$t" 2>/dev/null)"
        [ -n "$p" ] && ln -sf "$p" "$bin/$t"
    done
    printf '%s' "$bin"
}

# -----------------------------------------------------------------------------
# Fixtures: NAME|EXPECT|PAYLOAD  (EXPECT is the required exit code of a CORRECT
# checker: 0 = genuinely-healthy MCP server, 1 = must be reported unhealthy)
# -----------------------------------------------------------------------------
FIXTURES=(
'good_canonical|0|{"jsonrpc":"2.0","id":424242,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"x","version":"1"}}}'
'good_capabilities_first|0|{"jsonrpc":"2.0","id":424242,"result":{"capabilities":{},"protocolVersion":"2024-11-05"}}'
'good_whitespace|0|{ "jsonrpc" : "2.0" , "id" : 424242 , "result" : { "protocolVersion" : "2024-11-05" } }'
'good_brace_inside_string|0|{"jsonrpc":"2.0","id":424242,"result":{"protocolVersion":"2024-11-05","note":"} { \"protocolVersion\": decoy"}}'
'bad_flat_protocolversion|1|{"jsonrpc":"2.0","id":424242,"result":{},"protocolVersion":"2024-11-05"}'
'bad_nested_one_level_too_deep|1|{"jsonrpc":"2.0","id":424242,"result":{"serverInfo":{"protocolVersion":"2024-11-05"}}}'
'bad_result_null|1|{"jsonrpc":"2.0","id":424242,"result":null,"protocolVersion":"2024-11-05"}'
'bad_result_array|1|{"jsonrpc":"2.0","id":424242,"result":[{"protocolVersion":"2024-11-05"}]}'
'bad_jsonrpc_error|1|{"jsonrpc":"2.0","id":424242,"error":{"code":-32601,"message":"Method not found"}}'
'bad_wrong_id|1|{"jsonrpc":"2.0","id":999,"result":{"protocolVersion":"2024-11-05"}}'
'bad_id_quoted|1|{"jsonrpc":"2.0","id":"424242","result":{"protocolVersion":"2024-11-05"}}'
'bad_protocolversion_empty|1|{"jsonrpc":"2.0","id":424242,"result":{"protocolVersion":""}}'
'bad_protocolversion_null|1|{"jsonrpc":"2.0","id":424242,"result":{"protocolVersion":null}}'
'bad_cli_usage_text|1|Usage: github-mcp-server [command]'
'bad_silent_accept_then_close|1|__SILENT__'
# D3 guard — a HEALTHY server that simply answers a little slowly. Some netcat
# builds close the socket the moment stdin hits EOF, so the reply lands after the
# close and a perfectly good server is reported unhealthy. Found on the real
# fleet: mcp-redis replies in 0.56s and was falsely marked unhealthy by the nc
# transport while node/python3 both said healthy. Instant-reply fixtures cannot
# catch this — the delay is the whole point of this fixture.
'good_slow_reply|0|__SLOW__{"jsonrpc":"2.0","id":424242,"result":{"protocolVersion":"2024-11-05","capabilities":{}}}'
'good_reply_lost_if_client_half_closes|0|__EOFKILL__{"jsonrpc":"2.0","id":424242,"result":{"protocolVersion":"2024-11-05","capabilities":{}}}'
)

TRANSPORTS=(nc node python3)
VALIDATORS=(node python3 awk)

run_hc() {  # $1=script $2=port $3=transport $4=validator -> exit code
    MCP_HC_HOST=127.0.0.1 \
    MCP_HC_PORT="$2" \
    MCP_HC_TIMEOUT="$PROBE_TIMEOUT" \
    MCP_HC_TRANSPORT="$3" \
    MCP_HC_VALIDATOR="$4" \
        sh "$1" >/dev/null 2>&1
    return $?
}

echo -e "${BLUE}=====================================================${NC}"
echo -e "${BLUE}HXC-334 MCP healthcheck regression guard${NC}"
echo -e "${BLUE}=====================================================${NC}"
echo "RED_MODE          : $RED_MODE  (1=reproduce defects on legacy, 0=standing green guard)"
echo "checker under test: $HC"
echo "legacy RED artifact: $LEGACY"
echo "evidence dir      : $EVIDENCE_DIR"

# -----------------------------------------------------------------------------
section "Section 0: preconditions (a missing tool must SKIP loudly, never pass)"
# -----------------------------------------------------------------------------
PRECOND_OK=1
for t in python3 nc node awk sh; do
    if command -v "$t" >/dev/null 2>&1; then
        pass "host has $t ($(command -v "$t"))"
    else
        fail "host lacks $t — the 3x3 matrix cannot be proven without it"
        PRECOND_OK=0
    fi
done
if [ -x "$HC" ] || [ -r "$HC" ]; then pass "checker present: $HC"; else fail "checker missing: $HC"; PRECOND_OK=0; fi
if [ -r "$LEGACY" ]; then pass "legacy RED artifact present"; else fail "legacy RED artifact missing: $LEGACY"; PRECOND_OK=0; fi

if [ "$PRECOND_OK" -ne 1 ]; then
    echo -e "\n${RED}Preconditions failed — refusing to report a verdict on an invalid instrument.${NC}"
    exit 1
fi

# -----------------------------------------------------------------------------
# §1.1 PAIRED MUTATION MODE
# -----------------------------------------------------------------------------
# A guard that cannot be killed is decorative. This mode mutates the checker into
# the crudest possible always-healthy bluff (`exit 0` before it does anything)
# and requires the GREEN guard to FAIL on it. It also requires the GREEN guard to
# PASS on the unmutated checker in the same run, so a guard that fails for an
# unrelated reason cannot masquerade as a successful mutation kill.
if [ "$PROVE_MUTATION" -eq 1 ]; then
    section "Section M: paired §1.1 mutation — 'exit 0' MUST kill the guard"

    MUTANT="$TMPROOT/mutated_mcp_healthcheck.sh"
    {
        head -n 1 "$HC"
        echo 'exit 0   # §1.1 PAIRED MUTATION — always-healthy bluff, must be caught'
        tail -n +2 "$HC"
    } >"$MUTANT"
    chmod 0755 "$MUTANT"
    echo "    mutant written: $MUTANT (line 2 = 'exit 0')"

    echo "    [1/2] running GREEN guard against the UNMUTATED checker ..."
    RED_MODE=0 MCP_HC_SCRIPT="$HC" \
        MCP_HC_EVIDENCE_DIR="$EVIDENCE_DIR/mutation_control" \
        bash "${BASH_SOURCE[0]}" >"$TMPROOT/control.log" 2>&1
    CONTROL_EC=$?

    echo "    [2/2] running GREEN guard against the MUTATED checker ..."
    RED_MODE=0 MCP_HC_SCRIPT="$MUTANT" \
        MCP_HC_EVIDENCE_DIR="$EVIDENCE_DIR/mutation_mutant" \
        bash "${BASH_SOURCE[0]}" >"$TMPROOT/mutant.log" 2>&1
    MUTANT_EC=$?

    cp "$TMPROOT/control.log" "$EVIDENCE_DIR/mutation_control.log" 2>/dev/null
    cp "$TMPROOT/mutant.log"  "$EVIDENCE_DIR/mutation_mutant.log"  2>/dev/null

    echo "    control (unmutated) exit=$CONTROL_EC   mutant ('exit 0') exit=$MUTANT_EC"
    echo "    mutant failures:"
    grep -c '\[FAIL\]' "$TMPROOT/mutant.log" 2>/dev/null | sed 's/^/      /'
    grep '\[FAIL\]' "$TMPROOT/mutant.log" 2>/dev/null | head -6 | sed 's/^/      /'

    if [ "$CONTROL_EC" -eq 0 ]; then
        pass "control: GREEN guard PASSES on the unmutated checker (exit 0)"
    else
        fail "control: GREEN guard FAILED on the unmutated checker (exit $CONTROL_EC) — a mutation 'kill' would be meaningless"
    fi
    if [ "$MUTANT_EC" -ne 0 ]; then
        pass "MUTATION KILLED: guard FAILED on the 'exit 0' mutant (exit $MUTANT_EC)"
    else
        fail "MUTATION SURVIVED: guard PASSED on an always-exit-0 checker — the guard is decorative"
    fi

    echo -e "\n${BLUE}=== Summary (mutation mode) ===${NC}"
    echo "passed=$PASSED failed=$FAILED"
    echo "control log: $EVIDENCE_DIR/mutation_control.log"
    echo "mutant  log: $EVIDENCE_DIR/mutation_mutant.log"
    if [ "$FAILED" -eq 0 ]; then
        echo -e "${GREEN}MUTATION PROOF: PASS${NC}"
        exit 0
    fi
    echo -e "${RED}MUTATION PROOF: FAIL${NC}"
    exit 1
fi

# -----------------------------------------------------------------------------
section "Section 1: PATH-isolation positive control"
# -----------------------------------------------------------------------------
# The legacy-branch forcing below is only meaningful if the sandbox really hides
# tools. Prove it BOTH ways: a hidden tool must be absent AND an exposed tool
# must be present and resolve to a real binary (not a shell function).
SB_NCONLY="$(build_sandbox nconly nc)"
SB_NODEONLY="$(build_sandbox nodeonly node)"
SB_PYONLY="$(build_sandbox pyonly python3)"

iso_probe() { env -i PATH="$1" /bin/sh -c "command -v $2 || echo __ABSENT__"; }

if [ "$(iso_probe "$SB_NODEONLY" nc)" = "__ABSENT__" ]; then
    pass "sandbox hides nc (node-only sandbox)"
else
    fail "sandbox LEAKS nc — branch forcing would be invalid"
fi
if [ "$(iso_probe "$SB_PYONLY" nc)" = "__ABSENT__" ] && [ "$(iso_probe "$SB_PYONLY" node)" = "__ABSENT__" ]; then
    pass "sandbox hides nc and node (python3-only sandbox)"
else
    fail "sandbox LEAKS nc or node — branch forcing would be invalid"
fi
GREP_IN_SB="$(iso_probe "$SB_NCONLY" grep)"
case "$GREP_IN_SB" in
    /*) pass "grep inside sandbox resolves to a real binary: $GREP_IN_SB" ;;
    *)  fail "grep inside sandbox did not resolve to an absolute path: '$GREP_IN_SB'" ;;
esac
if [ "$(iso_probe "$SB_NCONLY" nc)" != "__ABSENT__" ]; then
    pass "sandbox exposes nc when asked (nc-only sandbox)"
else
    fail "sandbox failed to expose nc — positive control broken"
fi

# -----------------------------------------------------------------------------
section "Section 2: fixture matrix — 3 transports x 3 validators x ${#FIXTURES[@]} fixtures"
# -----------------------------------------------------------------------------
printf 'fixture\ttransport\tvalidator\texpected\tactual\tverdict\n' >"$MATRIX_TSV"
MATRIX_DISAGREEMENTS=0
MATRIX_WRONG=0

for entry in "${FIXTURES[@]}"; do
    fname="${entry%%|*}"; rest="${entry#*|}"
    fexp="${rest%%|*}"; fpayload="${rest#*|}"

    if ! start_fixture "$fpayload"; then
        fail "fixture server failed to start for $fname"
        continue
    fi

    verdicts=""
    combo_bad=0
    for tx in "${TRANSPORTS[@]}"; do
        for val in "${VALIDATORS[@]}"; do
            run_hc "$HC" "$FIXTURE_PORT" "$tx" "$val"
            ec=$?
            verdict="OK"
            if [ "$ec" -ne "$fexp" ]; then verdict="WRONG"; combo_bad=1; MATRIX_WRONG=$((MATRIX_WRONG+1)); fi
            printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$fname" "$tx" "$val" "$fexp" "$ec" "$verdict" >>"$MATRIX_TSV"
            verdicts="$verdicts $ec"
        done
    done
    stop_fixture

    uniq_verdicts="$(printf '%s\n' $verdicts | sort -u | tr '\n' ',' | sed 's/,$//')"
    if [ "$(printf '%s\n' $verdicts | sort -u | wc -l)" -ne 1 ]; then
        fail "$fname: branches DISAGREE (verdicts: $uniq_verdicts) — §11.4.50 determinism defect"
        MATRIX_DISAGREEMENTS=$((MATRIX_DISAGREEMENTS+1))
    elif [ "$combo_bad" -ne 0 ]; then
        fail "$fname: all 9 combos agree on $uniq_verdicts but expected $fexp"
    else
        pass "$fname: all 9 transport/validator combos returned $fexp"
    fi
done

if [ "$MATRIX_DISAGREEMENTS" -eq 0 ]; then
    pass "EQUIVALENCE: no fixture split the implementations (0 disagreements across $(( ${#FIXTURES[@]} * 9 )) runs)"
else
    fail "EQUIVALENCE BROKEN: $MATRIX_DISAGREEMENTS fixture(s) produced interpreter-dependent verdicts"
fi

# -----------------------------------------------------------------------------
section "Section 3: auto-selection cross-check (no env override)"
# -----------------------------------------------------------------------------
# Section 2 forces the implementation. This section proves the UNFORCED script
# is also correct under each PATH shape a real image can present.
AUTO_FIXTURES=('good_canonical|0' 'bad_flat_protocolversion|1' 'bad_silent_accept_then_close|1')
for af in "${AUTO_FIXTURES[@]}"; do
    aname="${af%%|*}"; aexp="${af##*|}"
    apayload=""
    for entry in "${FIXTURES[@]}"; do
        [ "${entry%%|*}" = "$aname" ] && { r="${entry#*|}"; apayload="${r#*|}"; break; }
    done
    start_fixture "$apayload" || { fail "fixture server failed for $aname"; continue; }
    for sbname in nconly nodeonly pyonly; do
        sb="$SANDBOX_BASE/$sbname/bin"
        # awk must be present for the awk-validator path to exist in nc-only shape
        p="$(command -v awk)"; [ -n "$p" ] && ln -sf "$p" "$sb/awk"
        env -i PATH="$sb" MCP_HC_HOST=127.0.0.1 MCP_HC_PORT="$FIXTURE_PORT" \
            MCP_HC_TIMEOUT="$PROBE_TIMEOUT" /bin/sh "$HC" >/dev/null 2>&1
        ec=$?
        if [ "$ec" -eq "$aexp" ]; then
            pass "auto-select [$sbname] $aname -> $ec (expected $aexp)"
        else
            fail "auto-select [$sbname] $aname -> $ec (expected $aexp)"
        fi
    done
    stop_fixture
done

# -----------------------------------------------------------------------------
section "Section 4: RED/GREEN polarity (RED_MODE=$RED_MODE)"
# -----------------------------------------------------------------------------
# D1 — socket-liveness-is-not-server-liveness, against the SHIPPED `nc -z` check.
for d1 in 'bad_silent_accept_then_close' 'bad_cli_usage_text'; do
    payload=""
    for entry in "${FIXTURES[@]}"; do
        [ "${entry%%|*}" = "$d1" ] && { r="${entry#*|}"; payload="${r#*|}"; break; }
    done
    start_fixture "$payload" || { fail "fixture server failed for $d1"; continue; }

    nc -z 127.0.0.1 "$FIXTURE_PORT" >/dev/null 2>&1
    legacy_ec=$?
    run_hc "$HC" "$FIXTURE_PORT" nc awk
    new_ec=$?
    stop_fixture

    if [ "$RED_MODE" = "1" ]; then
        if [ "$legacy_ec" -eq 0 ]; then
            pass "D1 REPRODUCED on [$d1]: shipped 'nc -z' returned 0 (false-PASS present)"
        else
            fail "D1 NOT reproduced on [$d1]: 'nc -z' returned $legacy_ec — RED test is blind, investigate before trusting the guard"
        fi
    else
        if [ "$new_ec" -eq 1 ]; then
            pass "D1 ABSENT on [$d1]: current checker returned 1 (was 0 under 'nc -z')"
        else
            fail "D1 REGRESSED on [$d1]: current checker returned $new_ec, expected 1"
        fi
    fi
done

# D2 — interpreter-dependent verdict, against the FROZEN first-attempt artifact.
D2_PAYLOAD=""
for entry in "${FIXTURES[@]}"; do
    [ "${entry%%|*}" = "bad_flat_protocolversion" ] && { r="${entry#*|}"; D2_PAYLOAD="${r#*|}"; break; }
done
if start_fixture "$D2_PAYLOAD"; then
    for sbname in nconly nodeonly pyonly; do
        sb="$SANDBOX_BASE/$sbname/bin"
        env -i PATH="$sb" MCP_HC_PORT="$FIXTURE_PORT" /bin/sh "$LEGACY" >/dev/null 2>&1
        eval "LEG_$sbname=\$?"
    done
    stop_fixture

    # shellcheck disable=SC2154
    echo "    legacy verdicts on flat-protocolVersion: nc=$LEG_nconly node=$LEG_nodeonly python3=$LEG_pyonly"
    if [ "$RED_MODE" = "1" ]; then
        if [ "$LEG_nconly" -eq 0 ] && [ "$LEG_nodeonly" -eq 1 ] && [ "$LEG_pyonly" -eq 1 ]; then
            pass "D2 REPRODUCED: legacy nc branch says healthy(0), node/python3 say unhealthy(1) — same server, opposite verdict"
        else
            fail "D2 NOT reproduced (nc=$LEG_nconly node=$LEG_nodeonly py=$LEG_pyonly) — RED test is blind, investigate"
        fi
    else
        if [ "$LEG_nconly" -eq 0 ] && [ "$LEG_nodeonly" -eq 1 ]; then
            pass "D2 historical split still demonstrable on the frozen legacy artifact (guard remains meaningful)"
        else
            skipmsg "legacy artifact no longer splits — guard's historical anchor changed, review testdata"
        fi
        if [ "$MATRIX_DISAGREEMENTS" -eq 0 ]; then
            pass "D2 ABSENT from current checker: 0 interpreter-dependent verdicts across the whole matrix"
        else
            fail "D2 REGRESSED: current checker produced $MATRIX_DISAGREEMENTS interpreter-dependent verdict(s)"
        fi
    fi
else
    fail "fixture server failed for D2"
fi

# -----------------------------------------------------------------------------
section "Summary"
# -----------------------------------------------------------------------------
{
    echo "run_utc=$TS"
    echo "red_mode=$RED_MODE"
    echo "checker=$HC"
    echo "fixtures=${#FIXTURES[@]}"
    echo "matrix_runs=$(( ${#FIXTURES[@]} * 9 ))"
    echo "matrix_wrong_verdicts=$MATRIX_WRONG"
    echo "matrix_disagreements=$MATRIX_DISAGREEMENTS"
    echo "passed=$PASSED"
    echo "failed=$FAILED"
    echo "skipped=$SKIPPED"
} >"$SUMMARY"

echo "passed=$PASSED failed=$FAILED skipped=$SKIPPED"
echo "matrix TSV : $MATRIX_TSV"
echo "summary    : $SUMMARY"

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}GUARD RESULT: PASS${NC}  (evidence: $EVIDENCE_DIR)"
    exit 0
fi
echo -e "${RED}GUARD RESULT: FAIL${NC}  ($FAILED assertion(s) failed; evidence: $EVIDENCE_DIR)"
exit 1
