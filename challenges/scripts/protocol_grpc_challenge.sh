#!/bin/bash
# Protocol gRPC Challenge
# Tests gRPC API support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"

# The gRPC port is resolved through the registry's OWN variable name and
# default, so this challenge cannot drift away from the server again.
#
# It previously read `${HELIXAGENT_GRPC_PORT:-7062}` — and both halves were
# wrong. `HELIXAGENT_GRPC_PORT` is a THIRD spelling that no Go source reads;
# the server resolves ports.HelixAgentGRPC, whose canonical variable name is
# HELIXAGENT_PORT_GRPC (internal/ports/ports.go:109). And 7062 was nobody's
# port: not the old hardcoded 50051, not the registry's 8112, not the HTTP
# 7061. So the challenge dialled an address no HelixAgent process has ever
# bound, found nothing, and — because every probe below also recorded
# success on absence — certified the gRPC protocol healthy without once
# contacting the gRPC server.
#
# 8112 is the registry default (core band, offset 112). It is duplicated here
# only because a bash challenge cannot call into Go; TestChallengeScript_GRPCPort
# MatchesRegistry in internal/ports asserts this literal equals
# ports.Default(ports.HelixAgentGRPC), so the two cannot silently diverge.
GRPC_PORT="${HELIXAGENT_PORT_GRPC:-8112}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# grpc_port_listening — single probe used by every test below.
#
# Consolidated deliberately: the port was previously probed by four separate
# `nc -z` call sites, and all four independently treated "nothing answered"
# as success. One helper means one place where absence is interpreted.
grpc_port_listening() {
    nc -z localhost "$GRPC_PORT" 2>/dev/null
}

# WHY ABSENCE IS A SKIP AND NOT A FAIL (§11.4.3, this challenge's contract).
# main() starts the HTTP binary via start_helixagent, but it neither starts
# nor builds a gRPC server — that is a SEPARATE binary, cmd/grpc-server. An
# absent listener is therefore an un-staged precondition rather than a proven
# product defect, so the honest verdict is SKIP-with-reason. finalize_challenge
# counts SKIPPED separately from PASSED, so a skip cannot inflate the score.
# What is NOT permitted is the third option this script used to take: calling
# absence success (§11.4.69 CM-NO-FAIL-OPEN-SKIP). When the server IS up, the
# probes below make real calls and a bad response is a real FAILED.
GRPC_ABSENT_REASON="gRPC server not listening on :$GRPC_PORT; this challenge does not start cmd/grpc-server (SKIP-OK: #grpc-server-not-started)"
GRPCURL_ABSENT_REASON="grpcurl is not installed, so no gRPC call can be made (SKIP-OK: #grpcurl-not-installed)"

init_challenge "protocol-grpc" "Protocol gRPC Challenge"
load_env

log_info "Testing gRPC protocol support..."

test_grpc_server_availability() {
    log_info "Test 1: gRPC server availability"

    # Probe site 1/4. Absence is SKIPPED, never success.
    if grpc_port_listening; then
        record_assertion "grpc_server" "available" "true" "Port $GRPC_PORT listening"
    else
        record_skip "grpc_server" "available" "$GRPC_ABSENT_REASON"
    fi
}

test_grpc_unary_call() {
    log_info "Test 2: gRPC unary call"

    # Probe site 2/4. Both preconditions skip honestly; only a REAL call verdicts.
    if ! command -v grpcurl > /dev/null 2>&1; then
        record_skip "grpc_unary" "working" "$GRPCURL_ABSENT_REASON"
        return
    fi
    if ! grpc_port_listening; then
        record_skip "grpc_unary" "working" "$GRPC_ABSENT_REASON"
        return
    fi

    # The server is up and grpcurl is present, so this call's outcome is a
    # real verdict about the product: a failure here is a FAILED, not a shrug.
    local resp
    resp=$(grpcurl -plaintext -max-time 10 \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"gRPC test"}],"max_tokens":10}' \
        localhost:$GRPC_PORT helixagent.ChatService/Complete 2>&1) || resp="ERROR: $resp"

    if [[ "$resp" =~ ERROR: || "$resp" =~ "connection refused" || "$resp" =~ Unimplemented ]]; then
        # `Unimplemented` is called out explicitly: a WRONG peer holding the
        # port (this project's Weaviate container answered exactly that on
        # the old :50051) completes a healthy HTTP/2 handshake, so it must
        # not be mistaken for a working HelixAgent.
        record_assertion "grpc_unary" "working" "false" "Unary call failed: $resp"
    else
        record_assertion "grpc_unary" "working" "true" "Unary call succeeded"
    fi
}

test_grpc_streaming_call() {
    log_info "Test 3: gRPC streaming call"

    # Probe site 3/4.
    if ! command -v grpcurl > /dev/null 2>&1; then
        record_skip "grpc_streaming" "working" "$GRPCURL_ABSENT_REASON"
        return
    fi
    if ! grpc_port_listening; then
        record_skip "grpc_streaming" "working" "$GRPC_ABSENT_REASON"
        return
    fi

    local resp
    resp=$(timeout 5 grpcurl -plaintext \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Stream test"}],"max_tokens":20,"stream":true}' \
        localhost:$GRPC_PORT helixagent.ChatService/CompleteStream 2>&1) || resp="ERROR: $resp"

    if [[ "$resp" =~ ERROR: || "$resp" =~ "connection refused" || "$resp" =~ Unimplemented ]]; then
        record_assertion "grpc_streaming" "working" "false" "Streaming call failed: $resp"
    else
        record_assertion "grpc_streaming" "working" "true" "Streaming call succeeded"
    fi
}

test_grpc_error_handling() {
    log_info "Test 4: gRPC error handling"

    # Probe site 4/4.
    if ! command -v grpcurl > /dev/null 2>&1; then
        record_skip "grpc_errors" "validated" "$GRPCURL_ABSENT_REASON"
        return
    fi
    if ! grpc_port_listening; then
        record_skip "grpc_errors" "validated" "$GRPC_ABSENT_REASON"
        return
    fi

    # Send an invalid request (missing the required model field): a correct
    # server MUST reject it with a gRPC error status. Anything else — including
    # a cheerful success — is a real failure of error handling.
    local resp
    resp=$(grpcurl -plaintext -max-time 5 \
        -d '{"messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        localhost:$GRPC_PORT helixagent.ChatService/Complete 2>&1) || true

    if [[ "$resp" =~ InvalidArgument || "$resp" =~ "Code:" ]]; then
        record_assertion "grpc_errors" "validated" "true" "Invalid request correctly rejected"
    else
        record_assertion "grpc_errors" "validated" "false" \
            "Invalid request was not rejected with a gRPC error status: $resp"
    fi
}

main() {
    log_info "Starting gRPC challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_grpc_server_availability
    test_grpc_unary_call
    test_grpc_streaming_call
    test_grpc_error_handling

    # Verdict over THREE states, not two.
    #
    # The previous line was `! grep -qs "|FAILED|" ... && PASSED || FAILED`,
    # which reported PASSED whenever nothing had failed — including the case
    # where nothing had been VERIFIED either. Combined with probes that
    # recorded success on absence, that is how this challenge reported green
    # without ever contacting a gRPC server.
    #
    # Zero-passed-with-skips is deliberately NOT reported as PASSED (that is
    # the §11.4.238 false-green) and NOT as FAILED either — per §11.4.201 a
    # false-positive refusal is its own bluff, and an un-staged precondition
    # is not a proven product defect. It is reported as the distinct third
    # state, SKIPPED, which finalize_challenge records in the results JSON
    # alongside the passed/failed/skipped tally.
    #
    # KNOWN CONSEQUENCE, chosen deliberately: finalize_challenge maps every
    # non-PASSED status to exit 1, so a fully-skipped run exits non-zero and
    # a caller that only inspects the exit code will read it as a failure.
    # That is the loud direction, and the correct one while the alternative
    # is a silent false-green. A genuine tri-state exit code is a
    # framework-level change affecting every challenge and is out of scope here.
    local log="$OUTPUT_DIR/logs/assertions.log"
    local n_failed n_passed
    n_failed=$(grep -c "|FAILED|" "$log" 2>/dev/null || true)
    n_passed=$(grep -c "|PASSED|" "$log" 2>/dev/null || true)
    [[ -z "$n_failed" ]] && n_failed=0
    [[ -z "$n_passed" ]] && n_passed=0

    if [[ "$n_failed" -gt 0 ]]; then
        finalize_challenge "FAILED"
    elif [[ "$n_passed" -gt 0 ]]; then
        finalize_challenge "PASSED"
    else
        log_warning "No gRPC assertion could be verified — reporting SKIPPED, not PASSED"
        finalize_challenge "SKIPPED"
    fi
}

main "$@"
