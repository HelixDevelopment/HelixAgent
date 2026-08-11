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

# The service this server actually serves.
#
# Every probe below used to address `helixagent.ChatService` — a service that
# exists in NO proto, NO generated stub, and NO registration anywhere in this
# repository. cmd/grpc-server/main.go registers exactly two services,
# pb.RegisterLLMFacadeServer and pb.RegisterLLMProviderServer, whose wire names
# are `llm.LLMFacade` and `llm.LLMProvider` (pkg/api/llm-facade_grpc.pb.go
# ServiceName). So every "real call" this challenge made was addressed to a
# service the server could only answer with Unimplemented — the call could
# never have succeeded even against a perfectly healthy server.
GRPC_SERVICE="llm.LLMFacade"
GRPC_PROTO="specs/001-helix-agent/contracts/llm-facade.proto"

# A method deliberately chosen NOT to exist. See grpc_identity below: asking a
# known service for an unknown method is what makes the peer identify itself,
# instantly and without executing any business logic.
GRPC_PROBE_METHOD="HelixAgentChallengeIdentityProbe"
GRPC_PROBE_TIMEOUT="${GRPC_PROBE_TIMEOUT:-6}"

# The identity probe is RETRIED before it may conclude "not-grpc", because a
# healthy HelixAgent answers TCP long before it answers gRPC.
#
# cmd/grpc-server calls net.Listen FIRST, then builds the provider registry
# (services.NewProviderRegistry → initAutoDiscovery), and only then calls
# Serve. Between Listen and Serve the kernel completes the TCP handshake from
# the accept backlog while NO gRPC handler exists yet, so a single-shot probe
# sees "something accepted, nothing spoke gRPC" and would verdict not-grpc on a
# perfectly healthy server. Measured on this host with provider API keys
# present, that window was ~18 s (first discovery log 19:58:01 → "listening on"
# banner 19:58:19), and a 6 s probe inside it returned
# "Failed to dial target host: context deadline exceeded".
#
# Only the not-grpc verdict is retried. `absent` is decided by nc and is an
# honest SKIP; `foreign-grpc` and `helixagent` are positive identifications
# that cannot become truer by waiting.
GRPC_PROBE_ATTEMPTS="${GRPC_PROBE_ATTEMPTS:-4}"
GRPC_PROBE_RETRY_DELAY="${GRPC_PROBE_RETRY_DELAY:-5}"

# TEST 4's vector: an input the SERVER ITSELF rejects.
#
# `name` is required by cmd/grpc-server/main.go AddProvider, which answers
# InvalidArgument "provider name is required" BEFORE mutating any state — so
# this probe is side-effect-free and leaves no provider registered.
#
# The expected message is the server's OWN words, duplicated here only because
# bash cannot call into Go. TestChallengeScript_ErrorVectorMatchesServer in
# internal/challenges/protocol_grpc_oracle_test.go asserts these literals still
# match the Go source, so the vector cannot silently drift away from the
# handler it targets.
GRPC_INVALID_METHOD="AddProvider"
GRPC_INVALID_PAYLOAD='{"type":"openai","model":"gpt-4"}'
GRPC_INVALID_EXPECT_CODE="InvalidArgument"
GRPC_INVALID_EXPECT_MSG="provider name is required"

# WHY ABSENCE IS A SKIP AND NOT A FAIL (§11.4.3, this challenge's contract).
# main() starts the HTTP binary via start_helixagent, but it neither starts
# nor builds a gRPC server — that is a SEPARATE binary, cmd/grpc-server, with
# its own main(). An absent listener is therefore an un-staged precondition
# rather than a proven product defect, so the honest verdict is SKIP-with-
# reason. finalize_challenge counts SKIPPED separately from PASSED, so a skip
# cannot inflate the score.
#
# A peer that IS present but is NOT our gRPC server is the opposite case: that
# is a proven defect and FAILs (see grpc_identity).
GRPC_ABSENT_REASON="gRPC server not listening on :$GRPC_PORT; this challenge does not start cmd/grpc-server (SKIP-OK: #grpc-server-not-started)"
GRPCURL_ABSENT_REASON="grpcurl is not installed, so the RPC-semantics tier cannot run (SKIP-OK: #grpcurl-not-installed)"

init_challenge "protocol-grpc" "Protocol gRPC Challenge"
load_env

log_info "Testing gRPC protocol support..."

# grpc_identity — the single oracle every test below consults.
#
# ANTI-BLUFF, and the whole point of this function: `nc -z` proves that
# something accepted a TCP connection. It does NOT prove that the something is
# a gRPC server, and it certainly does not prove it is OUR gRPC server. This
# challenge used to record a PASS on exactly that — a bare `nc -z` — which is
# how it could certify "gRPC protocol PASSED" against a Python socket that
# never spoke one byte of gRPC. This project has already been bitten twice by
# treating an open port as an identity: a Weaviate container answering on the
# old :50051, and a teardown guard that probed :8100 and reached an entirely
# different process.
#
# So the port is contacted with a REAL gRPC request over HTTP/2 and the peer's
# own answer decides. The request asks a service we DO serve for a method that
# deliberately does not exist, because grpc-go answers that from its service
# registry immediately — no business logic, no provider calls, no timeout
# exposure — and the answer names what the peer knows:
#
#   our server      -> grpc-status 12, "unknown method <M> for service llm.LLMFacade"
#   a foreign gRPC  -> grpc-status 12, "unknown service llm.LLMFacade"
#   a non-gRPC peer -> no application/grpc response at all
#
# Echoes exactly one of: absent | not-grpc | foreign-grpc | helixagent
grpc_identity() {
    # nc is used ONLY to tell "nothing there" from "something there". It never
    # decides a verdict on its own — that distinction is this fix.
    if ! nc -z localhost "$GRPC_PORT" 2>/dev/null; then
        echo "absent"
        return
    fi

    # Retry ONLY not-grpc, to survive the Listen→Serve warmup window described
    # at GRPC_PROBE_ATTEMPTS above. Any positive identification short-circuits.
    local attempt verdict
    for attempt in $(seq 1 "$GRPC_PROBE_ATTEMPTS"); do
        verdict="$(grpc_identity_probe "$attempt")"
        if [[ "$verdict" != "not-grpc" ]]; then
            echo "$verdict"
            return
        fi
        if [[ "$attempt" -lt "$GRPC_PROBE_ATTEMPTS" ]]; then
            sleep "$GRPC_PROBE_RETRY_DELAY"
        fi
    done

    echo "not-grpc"
}

# grpc_identity_probe — ONE probe attempt. Echoes the same four-value verdict.
grpc_identity_probe() {
    local attempt="$1"

    # A zero-length gRPC message: 1 compression byte + 4 big-endian length bytes.
    local frame="$OUTPUT_DIR/logs/grpc_empty_frame.bin"
    printf '\x00\x00\x00\x00\x00' > "$frame"

    local hdrs
    hdrs=$(curl -s --http2-prior-knowledge --max-time "$GRPC_PROBE_TIMEOUT" \
        -o /dev/null -D - -X POST \
        "http://127.0.0.1:$GRPC_PORT/$GRPC_SERVICE/$GRPC_PROBE_METHOD" \
        -H 'content-type: application/grpc' \
        -H 'te: trailers' \
        --data-binary @"$frame" 2>&1) || true

    # Captured evidence per §11.4.5 — the peer's own words, kept for the run.
    # EVERY attempt is appended, so a not-grpc verdict shows the full sequence
    # it was reached through rather than only the last look.
    local redirect=">>"
    [[ "$attempt" == "1" ]] && redirect=">"
    {
        echo "=== attempt $attempt/$GRPC_PROBE_ATTEMPTS ($(date -Iseconds)) ==="
        echo "probe: POST http://127.0.0.1:$GRPC_PORT/$GRPC_SERVICE/$GRPC_PROBE_METHOD"
        echo "--- response headers ---"
        printf '%s\n' "$hdrs"
    } | if [[ "$redirect" == ">" ]]; then cat > "$GRPC_PROBE_EVIDENCE"; else cat >> "$GRPC_PROBE_EVIDENCE"; fi

    # No gRPC content-type => whatever holds the port, it is not a gRPC server.
    if ! printf '%s' "$hdrs" | grep -qi 'content-type:[[:space:]]*application/grpc'; then
        echo "not-grpc"
        return
    fi

    # It speaks gRPC but has never heard of our service => wrong gRPC server.
    if printf '%s' "$hdrs" | grep -qi 'unknown service'; then
        echo "foreign-grpc"
        return
    fi

    # It rejected the bogus method *for our service* => it serves our service.
    if printf '%s' "$hdrs" | grep -qiF "for service $GRPC_SERVICE"; then
        echo "helixagent"
        return
    fi

    # Spoke gRPC but did not identify our service. Not provably ours (§11.4.6).
    echo "foreign-grpc"
}

# require_helixagent_grpc — shared precondition for the RPC-semantics tests.
#
# Returns 0 only when the identity oracle proved OUR gRPC server is on the
# port. Every other state is recorded honestly here, once, so no caller can
# accidentally treat "not ours" as a reason to shrug.
require_helixagent_grpc() {
    local assertion_type="$1" target="$2"

    case "$GRPC_IDENTITY" in
        helixagent) return 0 ;;
        absent)
            record_skip "$assertion_type" "$target" "$GRPC_ABSENT_REASON"
            ;;
        not-grpc)
            record_assertion "$assertion_type" "$target" "false" \
                "Port $GRPC_PORT is held by a peer that does not speak gRPC; evidence: $GRPC_PROBE_EVIDENCE"
            ;;
        foreign-grpc)
            record_assertion "$assertion_type" "$target" "false" \
                "A gRPC server answered on :$GRPC_PORT but does not serve $GRPC_SERVICE; evidence: $GRPC_PROBE_EVIDENCE"
            ;;
    esac
    return 1
}

test_grpc_server_availability() {
    log_info "Test 1: gRPC server identity on :$GRPC_PORT"

    # This assertion PASSes only on positive, captured, sink-side evidence
    # that OUR gRPC server answered — never on a TCP accept (§11.4.69).
    if require_helixagent_grpc "grpc_server" "available"; then
        record_assertion "grpc_server" "available" "true" \
            "$GRPC_SERVICE answered a real gRPC request on :$GRPC_PORT; evidence: $GRPC_PROBE_EVIDENCE"
    fi
}

test_grpc_unary_call() {
    log_info "Test 2: gRPC unary call"

    # `return 0`, never a bare `return`: this script runs under `set -e`, so a
    # test function that returned the precondition's non-zero status would kill
    # the whole challenge mid-run — skipping the remaining tests AND
    # finalize_challenge, leaving results.json stuck at "running". The
    # precondition has already recorded its own honest verdict; the function's
    # job here is only to stop doing more work.
    require_helixagent_grpc "grpc_unary" "working" || return 0

    if ! command -v grpcurl > /dev/null 2>&1; then
        record_skip "grpc_unary" "working" "$GRPCURL_ABSENT_REASON"
        return
    fi

    # -proto/-import-path are REQUIRED: cmd/grpc-server calls grpc.NewServer()
    # without registering the reflection service, so a reflection-based grpcurl
    # invocation (what this challenge used to issue) cannot resolve any method.
    local resp
    resp=$(grpcurl -plaintext -max-time 10 \
        -import-path "$PROJECT_ROOT/specs/001-helix-agent/contracts" \
        -proto "$PROJECT_ROOT/$GRPC_PROTO" \
        -d '{"prompt":"gRPC challenge probe","messages":[{"role":"user","content":"gRPC test"}],"model_params":{"max_tokens":10}}' \
        "localhost:$GRPC_PORT" "$GRPC_SERVICE/Complete" 2>&1) || resp="ERROR: $resp"

    if [[ "$resp" =~ ERROR: || "$resp" =~ "connection refused" || "$resp" =~ Unimplemented ]]; then
        record_assertion "grpc_unary" "working" "false" "Unary call failed: $resp"
    else
        record_assertion "grpc_unary" "working" "true" "Unary call succeeded: ${resp:0:200}"
    fi
}

test_grpc_streaming_call() {
    log_info "Test 3: gRPC streaming call"

    require_helixagent_grpc "grpc_streaming" "working" || return 0

    if ! command -v grpcurl > /dev/null 2>&1; then
        record_skip "grpc_streaming" "working" "$GRPCURL_ABSENT_REASON"
        return
    fi

    local resp
    resp=$(timeout 15 grpcurl -plaintext \
        -import-path "$PROJECT_ROOT/specs/001-helix-agent/contracts" \
        -proto "$PROJECT_ROOT/$GRPC_PROTO" \
        -d '{"prompt":"Stream test","messages":[{"role":"user","content":"Stream test"}],"model_params":{"max_tokens":20}}' \
        "localhost:$GRPC_PORT" "$GRPC_SERVICE/CompleteStream" 2>&1) || resp="ERROR: $resp"

    if [[ "$resp" =~ ERROR: || "$resp" =~ "connection refused" || "$resp" =~ Unimplemented ]]; then
        record_assertion "grpc_streaming" "working" "false" "Streaming call failed: $resp"
    else
        record_assertion "grpc_streaming" "working" "true" "Streaming call succeeded: ${resp:0:200}"
    fi
}

test_grpc_error_handling() {
    log_info "Test 4: gRPC error handling"

    require_helixagent_grpc "grpc_errors" "validated" || return 0

    if ! command -v grpcurl > /dev/null 2>&1; then
        record_skip "grpc_errors" "validated" "$GRPCURL_ABSENT_REASON"
        return
    fi

    # WHY THIS IS NOT AN UNKNOWN-FIELD REQUEST ANY MORE.
    #
    # This probe used to send `{"this_field_does_not_exist_in_the_schema":true}`
    # and accept InvalidArgument / "Code:" / "unknown field" as proof of server
    # validation. Measured against a healthy server, that vector NEVER REACHED
    # THE SERVER: with a `-proto`-sourced schema grpcurl converts JSON to
    # protobuf LOCALLY, so an unknown field fails client-side —
    #   error getting request data: message type llm.CompletionRequest has no
    #   known field named this_field_does_not_exist_in_the_schema
    # — and no request is ever put on the wire. A server instrumented with a
    # logging interceptor recorded SERVER-SAW-UNARY for a schema-valid payload
    # and NOTHING AT ALL for the unknown-field payload.
    #
    # That makes widening the pattern to match grpcurl's wording the one change
    # that must not be made: it would certify grpcurl's JSON parser rather than
    # HelixAgent (§11.4.226 — the evidence class at closure is what decides).
    # The old comment's premise was also wrong on the protocol: proto3 tolerates
    # unknown fields in the BINARY wire format by design, so "a correct server
    # MUST reject it" was never a valid gRPC conformance claim.
    #
    # The vector is therefore now server-observable: AddProvider with the
    # required `name` omitted. It is schema-valid, so grpcurl transmits it; the
    # handler rejects it at cmd/grpc-server/main.go AddProvider before touching
    # any state. And the assertion demands the SERVER'S OWN MESSAGE — a string
    # that exists only in our Go source — so a client-side parse failure or a
    # foreign gRPC peer cannot produce it. Matching that literal IS the proof
    # the request reached the handler.
    local resp
    resp=$(grpcurl -plaintext -max-time 10 \
        -import-path "$PROJECT_ROOT/specs/001-helix-agent/contracts" \
        -proto "$PROJECT_ROOT/$GRPC_PROTO" \
        -d "$GRPC_INVALID_PAYLOAD" \
        "localhost:$GRPC_PORT" "$GRPC_SERVICE/$GRPC_INVALID_METHOD" 2>&1) || true

    printf '%s\n' \
        "probe: $GRPC_SERVICE/$GRPC_INVALID_METHOD $GRPC_INVALID_PAYLOAD" \
        "expect: $GRPC_INVALID_EXPECT_CODE / $GRPC_INVALID_EXPECT_MSG" \
        "--- grpcurl output ---" "$resp" > "$GRPC_ERROR_EVIDENCE"

    # BOTH must hold. The code alone is not enough (a foreign peer answers
    # InvalidArgument for its own unrelated reasons); the message alone is the
    # server-reach proof. Together they identify OUR handler's rejection.
    if printf '%s' "$resp" | grep -qF "$GRPC_INVALID_EXPECT_CODE" \
        && printf '%s' "$resp" | grep -qF "$GRPC_INVALID_EXPECT_MSG"; then
        record_assertion "grpc_errors" "validated" "true" \
            "$GRPC_SERVICE/$GRPC_INVALID_METHOD rejected a semantically invalid request with $GRPC_INVALID_EXPECT_CODE (\"$GRPC_INVALID_EXPECT_MSG\"); evidence: $GRPC_ERROR_EVIDENCE"
    else
        record_assertion "grpc_errors" "validated" "false" \
            "$GRPC_SERVICE/$GRPC_INVALID_METHOD did not reject a request missing the required 'name' field with $GRPC_INVALID_EXPECT_CODE \"$GRPC_INVALID_EXPECT_MSG\"; got: ${resp:0:300}"
    fi
}

main() {
    log_info "Starting gRPC challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    # Probe ONCE, then every test consults the same verdict. Consolidated
    # deliberately: the port was previously probed by four separate `nc -z`
    # call sites, and all four independently treated "nothing answered" as
    # success. One oracle means one place where the peer is identified.
    GRPC_PROBE_EVIDENCE="$OUTPUT_DIR/logs/grpc_identity_probe.txt"
    GRPC_ERROR_EVIDENCE="$OUTPUT_DIR/logs/grpc_error_handling_probe.txt"
    GRPC_IDENTITY="$(grpc_identity)"
    log_info "gRPC identity on :$GRPC_PORT => $GRPC_IDENTITY"

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
