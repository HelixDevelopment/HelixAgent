#!/bin/bash
# Provider live-proof dispatcher for HelixAgent.
#
# Gap-closure (2026-07-08): this script previously dispatched to
# `./tests/providers/<name>_test.go` — a directory that has never existed in
# this submodule (`ls tests/providers/` returns "No such file or directory").
# Every invocation, per-provider or "all", failed immediately with a Go
# "no such file or directory" / "matched no packages" error, not a clean
# per-provider PASS/FAIL/SKIP. Its own `check_key` gate was also disconnected
# from the (nonexistent) test run: it only printed a warning and never
# short-circuited anything (§11.4.6/§11.4.174 audit finding).
#
# This script is rebuilt as a thin, honest dispatcher to the REAL per-provider
# CONST-039 live-proof harness, which lives in the sibling `helix_code` Go
# module (`helix_code/internal/llm/provider_live_proof_test.go`, build tag
# `providerlive`) — see that file's header for the full design. It:
#   - genuinely constructs each provider and makes a real HTTP call with a
#     fresh nonce challenge when a key/local-server is present;
#   - emits an honest, isolated SKIP ("SKIP: no-key" / "SKIP: unreachable")
#     per provider when absent — never a suite-level FAIL-on-absence;
#   - captures request/response evidence under
#     docs/qa/<run-id>/provider_coverage/<provider>/ at the repo root.
#
# Usage:
#   ./scripts/test_providers.sh              # run every CONST-039 provider
#   ./scripts/test_providers.sh openai        # run just one provider
#   ./scripts/test_providers.sh anthropic
#   ...
#
# Keys are read from the environment (populate via the project's `.env`
# convention — see helix_code/internal/llm/keyrecognition.go for the full
# multi-alias table; this script only prints a cosmetic presence summary
# for the operator, it is NOT the actual gate the Go harness uses).

set -e

# Resolve this script's own directory to an ABSOLUTE path FIRST, before any
# `cd`. $0 is a path relative to the caller's original working directory;
# resolving it again *after* the `cd` below (which changes the working
# directory to submodules/helix_agent) silently double-relativizes it and
# points HELIX_CODE_DIR at a nonexistent nested path
# (submodules/helix_agent/submodules/helix_agent/helix_code) — the exact bug
# caught in this rewrite's own no-keys proof run (§11.4.102/§11.4.146: root-
# caused via `bash -x` trace before landing this fix, not guessed).
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# HELIX_CODE_DIR resolves the sibling helix_code Go module that hosts the
# real harness, computed from the absolute SCRIPT_DIR (order-independent of
# any subsequent `cd`), so the script fails loudly (not silently) if the
# layout ever changes.
HELIX_CODE_DIR="$(cd "$SCRIPT_DIR/../../../helix_code" 2>/dev/null && pwd)"

cd "$SCRIPT_DIR/.."

echo "=== HelixAgent -> HelixCode Provider Live-Proof Dispatcher ==="
echo

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Cosmetic key-presence summary for the operator. This is NOT the real
# honest-SKIP gate (that lives in keyrecognition.go's IsProviderKeyPresent,
# consumed directly by the Go harness below) — it is only a quick visual
# sanity check before dispatch.
check_key() {
    local key_name=$1
    local key_value=$2
    if [ -z "$key_value" ]; then
        echo -e "${YELLOW}⚠ $key_name not set${NC}"
        return 1
    fi
    echo -e "${GREEN}✓ $key_name found${NC}"
    return 0
}

echo "Checking API keys (cosmetic summary only; the real gate is per-provider inside the harness)..."
# `|| true` on every call: check_key intentionally `return 1`s when a key is
# absent (that is not a script error) — under `set -e` an un-guarded non-zero
# return from a plain statement (not part of an if/&&/||) aborts the whole
# script immediately. This is the exact "absent key silently kills the
# dispatcher before it ever runs the harness" bug this rewrite fixes, caught
# via `bash -x` trace + `set -e` interaction analysis before landing.
check_key "OPENAI_API_KEY" "$OPENAI_API_KEY" || true
check_key "ANTHROPIC_API_KEY" "$ANTHROPIC_API_KEY" || true
check_key "GEMINI_API_KEY" "$GEMINI_API_KEY" || true
check_key "DEEPSEEK_API_KEY" "$DEEPSEEK_API_KEY" || true
check_key "GROQ_API_KEY" "$GROQ_API_KEY" || true
check_key "MISTRAL_API_KEY" "$MISTRAL_API_KEY" || true
check_key "XAI_API_KEY" "$XAI_API_KEY" || true
check_key "OPENROUTER_API_KEY" "$OPENROUTER_API_KEY" || true
echo "(Ollama/Llama.cpp are local providers gated by server reachability, not a key.)"
echo

if [ -z "$HELIX_CODE_DIR" ] || [ ! -d "$HELIX_CODE_DIR" ]; then
    echo -e "${RED}ERROR: could not resolve the sibling helix_code module directory.${NC}"
    echo "Expected it at: $(cd "$(dirname "$0")/../../.." 2>/dev/null && pwd)/helix_code"
    echo "This dispatcher requires the real harness at helix_code/internal/llm/provider_live_proof_test.go."
    exit 1
fi

RUN_FILTER=""
if [ -n "$1" ]; then
    PROVIDER=$1
    case $PROVIDER in
        openai|anthropic|gemini|deepseek|groq|mistral|xai|openrouter|ollama|llamacpp)
            RUN_FILTER="-run TestProviderLiveProof/${PROVIDER}"
            echo "Running live-proof harness for: $PROVIDER"
            ;;
        claude)
            RUN_FILTER="-run TestProviderLiveProof/anthropic"
            echo "Running live-proof harness for: anthropic (alias 'claude')"
            ;;
        *)
            echo -e "${RED}Unknown provider: $PROVIDER${NC}"
            echo "Valid providers: openai, anthropic, gemini, deepseek, groq, mistral, xai, openrouter, ollama, llamacpp"
            exit 1
            ;;
    esac
else
    RUN_FILTER="-run TestProviderLiveProof"
    echo "Running the full CONST-039 provider live-proof harness (all 10 providers)."
    echo "Providers with no key/local-server configured emit an honest SKIP; no API cost is incurred for those."
fi

echo
# `STATUS=0; ( ... ) || STATUS=$?` (not a bare `( ... )` line) so a genuine
# harness FAIL is captured into STATUS instead of triggering `set -e` and
# skipping the summary/exit-code lines below (same class of bug fixed above
# for check_key — a non-zero-returning bare statement aborts immediately
# under `set -e` unless it is the operand of `||`/`&&`/an if-condition).
# -timeout must EXCEED the harness's worst-case aggregate of its own per-subtest
# context.WithTimeout bounds (8 hosted x 45s + 2 local x 60s = 480s); otherwise
# `go test` panics the whole process instead of letting each provider subtest
# FAIL cleanly and preserve its per-provider evidence.
STATUS=0
(
    cd "$HELIX_CODE_DIR" && \
    go test -tags=providerlive -v -count=1 -timeout=600s $RUN_FILTER ./internal/llm/
) || STATUS=$?

echo
if [ $STATUS -eq 0 ]; then
    echo -e "${GREEN}=== Provider live-proof harness completed (PASS/SKIP per provider — see output above) ===${NC}"
else
    echo -e "${RED}=== Provider live-proof harness reported a failure (see output above) ===${NC}"
fi
exit $STATUS
