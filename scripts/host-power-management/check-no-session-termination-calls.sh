#!/bin/bash
# check-no-session-termination-calls.sh — CONST-036 static scanner.
#
# Walks the project tree and fails if ANY file invokes a command that
# terminates the currently-logged-in user's session, kills the user
# manager, or otherwise indirectly forces the user to lose in-flight
# work. CONST-036 is the sibling of CONST-033 (host-power-management):
# CONST-033 forbids host-level power transitions; CONST-036 forbids
# session/user-level terminations that have the same end effect for
# the user (lost windows, lost terminals, killed AI agents,
# half-flushed builds).
#
# Forbidden (non-exhaustive):
#   loginctl  terminate-user|terminate-session|kill-user|kill-session
#   systemctl stop user@<UID>     # kills the user manager + every child
#   systemctl kill user@<UID>
#   gnome-session-quit            # ends the GNOME session
#   pkill -KILL -u $USER          # nukes everything as the user
#   killall -u $USER
#   killall -KILL -u $USER
#   dbus-send … org.gnome.SessionManager.Logout
#   dbus-send … org.gnome.SessionManager.Shutdown / Reboot
#   busctl call … org.gnome.SessionManager.Logout/Shutdown/Reboot
#   echo … > /sys/power/state    # direct kernel power transition
#   /usr/bin/poweroff             # standalone poweroff binary
#   /usr/bin/reboot               # standalone reboot binary
#   /usr/bin/halt                 # standalone halt binary
#
# Usage:
#   bash check-no-session-termination-calls.sh [project_root]
#
# Exit:
#   0 = clean
#   1 = one or more violations found (printed)
#   2 = invocation error

set -uo pipefail
ROOT="${1:-.}"

if [[ ! -d "$ROOT" ]]; then
  echo "ERROR: $ROOT is not a directory" >&2
  exit 2
fi

EXCLUDE_DIRS=(
  ".git" ".svn" ".hg"
  "node_modules" "vendor" "third_party" "Upstreams" "upstreams"
  "cli_agents" "MCP" "MCP_Module/submodules"
  ".cache" ".gradle" ".idea" ".vscode" ".venv" "venv" "__pycache__"
  "build" "dist" "target" "out" "bin" "obj"
  "releases"
  "opensource"
  "external"
)

# Governance docs and the scanner itself ARE allowed to mention these
# patterns by name (descriptive, not invocation).
EXCLUDE_PATHS=(
  "host-power-management/"
  "no_session_termination_calls_challenge.sh"
  "host_no_auto_suspend_challenge.sh"
  "no_suspend_calls_challenge.sh"
  "HOST_POWER_MANAGEMENT.md"
  "SESSION_LOSS_2026-04-28.md"
  "CONSTITUTION.md"
  "CONSTITUTION.json"
  "AGENTS.md"
  "CLAUDE.md"
  "QWEN.md"
  "GEMINI.md"
  "/docs/issues/fixed/BUGFIXES.md"
  "/CHANGELOG.md"
)

# Forbidden grep -E patterns.
FORBIDDEN=(
  '\bloginctl[[:space:]]+(terminate-user|terminate-session|kill-user|kill-session)\b'
  '\bsystemctl[[:space:]]+(stop|kill)[[:space:]]+user@'
  '\bgnome-session-quit\b'
  '\bpkill[[:space:]]+([-]+[a-zA-Z]+[[:space:]]+)*-u[[:space:]]+("?\$\{?USER\}?"?|"?\$\{?LOGNAME\}?"?|[a-zA-Z][a-zA-Z0-9_-]*)'
  '\bkillall[[:space:]]+([-]+[a-zA-Z]+[[:space:]]+)*-u[[:space:]]+("?\$\{?USER\}?"?|"?\$\{?LOGNAME\}?"?|[a-zA-Z][a-zA-Z0-9_-]*)'
  'org\.gnome\.SessionManager\.(Logout|Shutdown|Reboot)'
  '>[[:space:]]*/sys/power/state'
  '\becho[[:space:]]+.*>[[:space:]]*/sys/power/state'
  '/usr/(bin|sbin)/(poweroff|reboot|halt)\b'
)

EXCL_ARGS=()
for d in "${EXCLUDE_DIRS[@]}"; do EXCL_ARGS+=( --exclude-dir="$d" ); done
PATTERN=$(IFS='|'; echo "${FORBIDDEN[*]}")

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

grep -RInE "$PATTERN" "${EXCL_ARGS[@]}" -- "$ROOT" 2>/dev/null > "$TMP" || true

VIOLATIONS=$(awk -v EXCLUDE_PATHS_PIPED="$(IFS='|'; echo "${EXCLUDE_PATHS[*]}")" '
  BEGIN {
    n = split(EXCLUDE_PATHS_PIPED, arr, "|")
    for (i=1;i<=n;i++) ex[i] = arr[i]
    excount = n
  }
  {
    skip = 0
    for (i=1;i<=excount;i++) {
      if (ex[i] != "" && index($0, ex[i]) > 0) { skip = 1; break }
    }
    if (!skip) print
  }
' "$TMP")

if [[ -z "$VIOLATIONS" ]]; then
  echo "OK: no forbidden session-termination calls in $ROOT"
  exit 0
fi

echo "FAIL: forbidden session-termination invocations (CONST-036):"
echo "$VIOLATIONS"
echo
echo "If a hit is a legitimate non-user-session context (e.g. a"
echo "container's internal init, a service-account user manager,"
echo "a documentation example), add the file path to EXCLUDE_PATHS"
echo "at the top of this script."
exit 1
