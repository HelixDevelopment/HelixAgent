#!/bin/bash
# no_session_termination_calls_challenge.sh — CONST-036 source-tree gate.
#
# Wraps check-no-session-termination-calls.sh. Asserts the project's
# source tree contains zero forbidden user-session-termination invocations.
# CONST-036 is the sibling of CONST-033 (no host-power-management):
# CONST-033 forbids host suspend/poweroff; CONST-036 forbids forcing the
# logged-in user to lose their session, which has the same blast radius
# (lost windows, lost terminals, killed AI agents, half-flushed builds).
#
# Resolves the scanner relative to its own location, so it works
# whether executed from the project root or from challenges/scripts/.
#
# Exit:
#   0 = clean
#   1 = violations
#   2 = scanner missing

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

find_project_root() {
  local d="$1"
  while [[ "$d" != "/" ]]; do
    if [[ -f "$d/scripts/host_power_management/check-no-session-termination-calls.sh" ]]; then
      echo "$d"; return 0
    fi
    d=$(dirname "$d")
  done
  return 1
}

PROJECT_ROOT=$(find_project_root "$SCRIPT_DIR" || true)
if [[ -z "${PROJECT_ROOT:-}" ]]; then
  echo "FAIL: cannot locate scripts/host_power_management/check-no-session-termination-calls.sh" >&2
  exit 2
fi

SCANNER="$PROJECT_ROOT/scripts/host_power_management/check-no-session-termination-calls.sh"
echo "=== no_session_termination_calls_challenge ==="
echo "Scanner: $SCANNER"
echo "Root:    $PROJECT_ROOT"
echo

bash "$SCANNER" "$PROJECT_ROOT"
rc=$?
echo
echo "=== summary: $([[ $rc -eq 0 ]] && echo PASS || echo FAIL) ==="
exit "$rc"
