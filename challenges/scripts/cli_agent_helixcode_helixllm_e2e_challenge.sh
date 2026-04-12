#!/bin/bash
#===============================================================================
# HELIXAGENT CLI-AGENT CHALLENGE — helixcode (HelixLLM E2E)
#===============================================================================
# Auto-generated wrapper. Tests the 5-step E2E flow through HelixLLM
# llama.cpp-only mode for the helixcode CLI agent.
#===============================================================================

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

AGENT_NAME="helixcode"
AGENT_DISPLAY_NAME="helixcode"
AGENT_MODEL_ID="${AGENT_MODEL_ID:-helixagent-debate}"

# shellcheck source=_cli_agent_helixllm_e2e_common.sh
source "$SCRIPT_DIR/_cli_agent_helixllm_e2e_common.sh"

cli_agent_helixllm_e2e "$@"
