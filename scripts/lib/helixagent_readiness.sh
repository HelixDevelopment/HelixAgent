#!/bin/bash
# Readiness + test-execution assertions for scripts that START a HelixAgent
# and then run the integration suite against it.
#
# Sourced by scripts/run_complete_validation.sh and
# scripts/orchestrate_full_test.sh. Not executable on its own.
#
# ---------------------------------------------------------------------------
# WHY THIS EXISTS (§11.4.201 — assert the real condition, not "something
# answered"; §11.4.111 — resolve by identity, never by a hardcoded ordinal)
# ---------------------------------------------------------------------------
#
# Both callers used to gate readiness on:
#
#     curl -s http://localhost:8100/health > /dev/null 2>&1
#
# and then report "Integration tests passed" on `go test` exit 0. Measured on
# this host 2026-08-09, that chain certified a green run in which the guarded
# integration tests never executed:
#
#   * `llm-verifier` (pid 1827554) held :8100 and answered `404 page not found`
#   * the HelixAgent the script itself started was on :7061, answering
#     {"service":"helixagent","status":"healthy"}
#   * `curl -s` WITHOUT `-f` exits 0 on a 404 — measured: exit 0 on the
#     squatted port, exit 22 with `-f`. So readiness passed having never
#     reached the server the script had just started.
#   * the guarded tests probe the same default and SKIP (13 call sites);
#     `go test` exits 0 because skips are not failures — measured 3 SKIP /
#     0 PASS / 0 FAIL / exit 0
#   * the script logged "Integration tests passed"
#
# Three independent defects, so three independent assertions here. Fixing only
# the `-f` would still leave the script probing a port its own server does not
# bind; fixing only the port would still accept a foreign 404; and both
# together would still report success on a suite that ran nothing.
#
# ---------------------------------------------------------------------------
# WHY THE PORT IS DISCOVERED AND NOT HARDCODED (§11.4.6 — no guessing)
# ---------------------------------------------------------------------------
#
# There is no static source a caller could read to learn the port. Five
# in-tree sources disagree, and a sixth is invisible to the script:
#
#   1. internal/config/config.go:316   getEnv("PORT", "7061")   <-- what the
#      binary ACTUALLY binds: config.Load() reads $PORT, default 7061
#   2. internal/ports/ports.go         HelixAgentHTTP = 8100 ("was 7061") —
#      the registry the TESTS default to
#   3. configs/development.yaml:5      port: 7061 — but NOT loaded, because
#      these scripts start `./bin/helixagent` with no --config flag
#   4. cmd/helixagent/main.go:1737     appCfg default ServerPort "8100",
#      neutered at main.go:1819 by `if appCfg.ServerPort != "8100"` — so the
#      8100 default can never win
#   5. tests/integration/comprehensive_infrastructure_test.go:23
#      getEnv("HELIXAGENT_URL", "http://localhost:8100")
#   6. cmd/helixagent/main.go:2392-2396 — main() runs godotenv.Overload on
#      `.env.bak` and `.env` BEFORE config.Load(). `.env` is gitignored, so an
#      operator's PORT= line silently overrides anything the script exports.
#      A script that exported PORT and trusted it would be guessing.
#
# So the port is not knowable a priori — it is only knowable by OBSERVATION.
# helix_wait_ready therefore asks the kernel which TCP port the PID we started
# actually bound (ownership-verified per §11.4.174 — matched on our own pid,
# never a name match that could hit another project's process), then requires
# the responder there to identify itself as "helixagent" before declaring
# readiness. The discovered address is exported as HELIXAGENT_URL so the
# server, the readiness probe and the test target agree by construction
# instead of by coincidence.

# helixagent_service_name is the identity HelixAgent reports in /health:
#   {"service":"helixagent","status":"healthy"}
# A port number is not an identity (§11.4.111). This mirrors
# tests/integration/helixagent_identity.go:54 — keep the two in step.
HELIXAGENT_SERVICE_NAME="helixagent"

# helix_listen_ports_for_pid <pid>
#
# Echoes every TCP port the given PID is LISTENing on, one per line.
# Ownership is established by matching the pid field, never by process name:
# a name match ("helixagent") could select another checkout's process on a
# shared host (§11.4.174).
helix_listen_ports_for_pid() {
    local pid="$1"
    command -v ss > /dev/null 2>&1 || return 1
    # ss -ltnpH: TCP, listening, numeric, with process info, no header.
    # Field 4 is the local address (*:7061, 0.0.0.0:7061, [::]:7061).
    ss -ltnpH 2>/dev/null \
        | grep -F "pid=${pid}," \
        | awk '{print $4}' \
        | sed 's/.*://' \
        | grep -E '^[0-9]+$' \
        | sort -u
}

# helix_identifies_as_helixagent <base-url>
#
# Returns 0 only when the responder at <base-url>/health both answers with an
# HTTP success (curl -sf — NOT bare curl -s, which exits 0 on the foreign 404
# this whole file exists to catch) AND names itself as HelixAgent.
helix_identifies_as_helixagent() {
    local url="${1%/}"
    local body
    body="$(curl -sf --max-time 10 "${url}/health" 2>/dev/null)" || return 1
    printf '%s' "$body" | grep -q "\"service\"[[:space:]]*:[[:space:]]*\"${HELIXAGENT_SERVICE_NAME}\""
}

# helix_wait_ready <pid> [timeout_seconds]
#
# Waits until the HelixAgent started as <pid> is serving an identifying
# /health, then ECHOES the base URL it actually bound. Returns non-zero (with
# a diagnosis on stderr) if that never happens.
#
# CALLERS MUST EXPORT THE RESULT THEMSELVES:
#
#     HELIX_URL="$(helix_wait_ready "$PID" 60)" || return 1
#     export HELIXAGENT_URL="$HELIX_URL"
#
# This function deliberately does NOT export HELIXAGENT_URL itself. It is
# invoked in a command substitution to capture the URL, and a command
# substitution runs in a SUBSHELL — an `export` inside it is discarded when
# the subshell exits, leaving the parent with an empty HELIXAGENT_URL and the
# tests silently back on their :8100 default. That failure was observed while
# proving this file: readiness reported the correct discovered URL while
# HELIXAGENT_URL stayed empty. Echo-and-let-the-caller-export is the only
# form that cannot silently half-work.
#
# Callers MUST treat a non-zero return as fatal: it means the server they
# started never became addressable, so anything the suite reports afterwards
# is about some other process or about nothing at all.
helix_wait_ready() {
    local pid="$1"
    local timeout="${2:-60}"
    local i port url

    for (( i = 0; i < timeout; i++ )); do
        # The process must still be alive; a dead server never becomes ready,
        # and spinning the full timeout on a corpse hides the real cause.
        if ! kill -0 "$pid" 2> /dev/null; then
            echo "helix_wait_ready: process $pid exited before becoming ready" >&2
            return 1
        fi

        # A PID may bind several ports (HTTP/3 also serves UDP, which -t
        # excludes). Identity — not ordering — decides which one is HelixAgent.
        while read -r port; do
            [ -n "$port" ] || continue
            url="http://localhost:${port}"
            if helix_identifies_as_helixagent "$url"; then
                echo "$url"
                return 0
            fi
        done < <(helix_listen_ports_for_pid "$pid")

        # Fallback for hosts without `ss`: probe the same expression
        # config.Load() evaluates (internal/config/config.go:316). This is a
        # fallback, not a guess — the identity check still gates it, so a
        # wrong value fails loudly instead of passing silently.
        if ! command -v ss > /dev/null 2>&1; then
            url="http://localhost:${PORT:-7061}"
            if helix_identifies_as_helixagent "$url"; then
                echo "$url"
                return 0
            fi
        fi

        sleep 1
    done

    {
        echo "helix_wait_ready: no service identifying as \"${HELIXAGENT_SERVICE_NAME}\" appeared for pid $pid within ${timeout}s."
        echo "  Ports LISTENing on that pid: $(helix_listen_ports_for_pid "$pid" | tr '\n' ' ' || echo '<none/ss unavailable>')"
        echo "  A reachable-but-unidentified responder is NOT readiness: a foreign"
        echo "  service answering 404 satisfies bare 'curl -s' but proves nothing."
    } >&2
    return 1
}

# helix_count_executed_tests <go-test-log>
#
# Echoes the number of test cases that actually EXECUTED — `--- PASS` plus
# `--- FAIL`, at any indentation so subtests count. Skips are deliberately
# excluded: a skipped test asserted nothing.
# NOTE ON `grep -c`: it PRINTS "0" and ALSO exits 1 when there are no
# matches. An `|| echo 0` fallback therefore emits TWO values ("0" and "0"),
# which collapse to the string "0 0" through command substitution and make
# `[ "$n" -eq 0 ]` abort with "integer expression expected" — a non-zero
# status that reads as "not zero", so an all-skipped run was accepted as
# passing. That exact false-accept was observed while proving this file.
# `|| true` is the correct suppressor: grep has already printed the count.
helix_count_executed_tests() {
    local log="$1" count
    [ -f "$log" ] || { echo 0; return 0; }
    count="$(grep -cE '^[[:space:]]*--- (PASS|FAIL):' "$log" 2>/dev/null || true)"
    [ -n "$count" ] || count=0
    echo "$count"
}

# helix_assert_tests_executed <go-test-log> <label>
#
# Returns non-zero when the run executed ZERO test cases.
#
# `go test` exits 0 for an all-skipped package, so exit status alone cannot
# distinguish "everything passed" from "nothing ran" — measured: 3 SKIP,
# 0 PASS, 0 FAIL, exit 0. A suite that ran nothing is not a suite that
# passed, and reporting it as success is the §11.4 PASS-bluff this guards.
helix_assert_tests_executed() {
    local log="$1"
    local label="${2:-tests}"
    local executed skipped

    executed="$(helix_count_executed_tests "$log")"
    skipped="$(grep -cE '^[[:space:]]*--- SKIP:' "$log" 2>/dev/null || true)"
    [ -n "$skipped" ] || skipped=0

    if [ "$executed" -eq 0 ]; then
        {
            echo "$label executed ZERO test cases (${skipped} skipped) — nothing was validated."
            echo "  'go test' exits 0 on an all-skipped package, so a green exit here"
            echo "  certifies nothing. Check the skip reasons in: $log"
        } >&2
        return 1
    fi
    return 0
}
