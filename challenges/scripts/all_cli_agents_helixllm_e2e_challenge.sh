#!/bin/bash
#===============================================================================
# ALL CLI AGENTS — HELIXLLM E2E CHALLENGE
#===============================================================================
# Runs the 5-step HelixLLM E2E flow for ALL 53 CLI agents sequentially.
# Each agent tests: hello, codebase awareness, file access, tool calling,
# chat quality — all through HelixLLM llama.cpp-only mode.
#
# Tests: 53 agents x 5 steps = 265 steps
# Requires: HelixLLM on https://localhost:8444 or HelixAgent on :7061
#===============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
PURPLE='\033[0;35m'
NC='\033[0m'

AGENTS_PASSED=0
AGENTS_FAILED=0

echo -e "${PURPLE}=====================================================${NC}"
echo -e "${PURPLE}  ALL CLI AGENTS — HELIXLLM E2E CHALLENGE${NC}"
echo -e "${PURPLE}=====================================================${NC}"

for script in "$SCRIPT_DIR"/cli_agent_*_helixllm_e2e_challenge.sh; do
    [ -f "$script" ] || continue
    agent_name=$(basename "$script" | sed 's/cli_agent_//; s/_helixllm_e2e_challenge.sh//')

    local_log="/tmp/helixllm_e2e_${agent_name}.log"
    if GOMAXPROCS=2 nice -n 19 bash "$script" > "$local_log" 2>&1; then
        AGENTS_PASSED=$((AGENTS_PASSED + 1))
        echo -e "${GREEN}PASS${NC}: $agent_name"
    else
        AGENTS_FAILED=$((AGENTS_FAILED + 1))
        echo -e "${RED}FAIL${NC}: $agent_name"
    fi
done

TOTAL=$((AGENTS_PASSED + AGENTS_FAILED))

echo ""
echo -e "${PURPLE}=====================================================${NC}"
echo -e "${PURPLE}  SUMMARY: $AGENTS_PASSED passed, $AGENTS_FAILED failed (of $TOTAL)${NC}"
echo -e "${PURPLE}=====================================================${NC}"

if [ "$AGENTS_FAILED" -eq 0 ]; then
    echo -e "${GREEN}ALL CLI AGENTS HELIXLLM E2E CHALLENGE PASSED${NC}"
    exit 0
else
    echo -e "${RED}CHALLENGE FAILED: $AGENTS_FAILED agents failed${NC}"
    exit 1
fi
