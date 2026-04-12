#!/bin/bash
#===============================================================================
# HELIXAGENT CLI-AGENT CHALLENGE — deepseek_cli
#===============================================================================
# Auto-generated wrapper around _cli_agent_challenge_common.sh.
# Exercises the 25-prompt suite against HelixAgent using the deepseek-cli
# CLI binary (skips cleanly when the binary is not installed locally).
#===============================================================================

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

AGENT_NAME="deepseek_cli"
AGENT_DISPLAY_NAME="deepseek-cli"
AGENT_BIN="deepseek"
# Default model id; override via AGENT_MODEL_ID env var before running.
AGENT_MODEL_ID="${AGENT_MODEL_ID:-helixagent-debate}"
# Optional CLI probe — default: "<bin> --help".
AGENT_CLI_PROBE="${AGENT_CLI_PROBE:-$AGENT_BIN --help}"

# shellcheck source=_cli_agent_challenge_common.sh
source "$SCRIPT_DIR/_cli_agent_challenge_common.sh"

cli_agent_run_challenge "$@"
