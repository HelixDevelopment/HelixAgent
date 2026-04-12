#!/bin/bash
#===============================================================================
# HELIXAGENT CLI-AGENT CHALLENGE — continue_dev (HelixLLM E2E)
#===============================================================================
# Auto-generated wrapper. Tests the 5-step E2E flow through HelixLLM
# llama.cpp-only mode for the continue-dev CLI agent.
#===============================================================================

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

AGENT_NAME="continue_dev"
AGENT_DISPLAY_NAME="continue-dev"
AGENT_MODEL_ID="${AGENT_MODEL_ID:-helixagent-debate}"

# shellcheck source=_cli_agent_helixllm_e2e_common.sh
source "$SCRIPT_DIR/_cli_agent_helixllm_e2e_common.sh"

cli_agent_helixllm_e2e "$@"
