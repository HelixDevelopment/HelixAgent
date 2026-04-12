#!/bin/bash
#===============================================================================
# HELIXAGENT CLI-AGENT CHALLENGE — aichat_llm_functions
#===============================================================================
# Auto-generated wrapper around _cli_agent_challenge_common.sh.
# Exercises the 25-prompt suite against HelixAgent using the aichat-llm-functions
# CLI binary (skips cleanly when the binary is not installed locally).
#===============================================================================

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

AGENT_NAME="aichat_llm_functions"
AGENT_DISPLAY_NAME="aichat-llm-functions"
AGENT_BIN="aichat-llm-functions"
# Default model id; override via AGENT_MODEL_ID env var before running.
AGENT_MODEL_ID="${AGENT_MODEL_ID:-helixagent-debate}"
# Optional CLI probe — default: "<bin> --help".
AGENT_CLI_PROBE="${AGENT_CLI_PROBE:-$AGENT_BIN --help}"

# shellcheck source=_cli_agent_challenge_common.sh
source "$SCRIPT_DIR/_cli_agent_challenge_common.sh"

cli_agent_run_challenge "$@"
