#!/bin/bash
#===============================================================================
# HELIXAGENT CLI-AGENT CHALLENGE — open_interpreter
#===============================================================================
# Auto-generated wrapper around _cli_agent_challenge_common.sh.
# Exercises the 25-prompt suite against HelixAgent using the open-interpreter
# CLI binary (skips cleanly when the binary is not installed locally).
#===============================================================================

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

AGENT_NAME="open_interpreter"
AGENT_DISPLAY_NAME="open-interpreter"
AGENT_BIN="interpreter"
# Default model id; override via AGENT_MODEL_ID env var before running.
AGENT_MODEL_ID="${AGENT_MODEL_ID:-helixagent-debate}"
# Optional CLI probe — default: "<bin> --help".
AGENT_CLI_PROBE="${AGENT_CLI_PROBE:-$AGENT_BIN --help}"

# shellcheck source=_cli_agent_challenge_common.sh
source "$SCRIPT_DIR/_cli_agent_challenge_common.sh"

cli_agent_run_challenge "$@"
