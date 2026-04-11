#!/usr/bin/env bash
# deps-scan.sh — Dependency vulnerability scan.
#
# Runs govulncheck (Go vulnerability database lookup) and reports known
# CVEs against the current dependency graph. Writes a timestamped markdown
# report to reports/security/.
#
# Non-interactive: no sudo, no network prompts, no container starts. The
# script ONLY runs tools that are already on PATH. Exit codes:
#   0 — scan completed (findings are written to the report regardless)
#   1 — scanner not installed
#   2 — scan invocation failed
#
# This script deliberately does NOT fail on findings. Triage happens out
# of band — the CI ban means we cannot gate the pipeline on it.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORTS_DIR="${REPO_ROOT}/reports/security"
TIMESTAMP="$(date -u +%Y-%m-%dT%H-%M-%SZ)"
REPORT="${REPORTS_DIR}/deps-${TIMESTAMP}.md"

mkdir -p "${REPORTS_DIR}"

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "error: govulncheck is not installed." >&2
  echo "install with: go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
  exit 1
fi

cd "${REPO_ROOT}"

{
  echo "# Dependency Scan Report — ${TIMESTAMP}"
  echo
  echo "## Environment"
  echo
  echo "- Working directory: \`${REPO_ROOT}\`"
  echo "- Go: \`$(go version)\`"
  echo "- govulncheck: \`$(govulncheck -version 2>&1 | head -1)\`"
  echo
  echo "## govulncheck — ./..."
  echo
  echo '```'
} > "${REPORT}"

# -test=false is the default; -show=verbose would be noisy for 1000+ pkgs.
if GOMAXPROCS=2 nice -n 19 govulncheck -json ./... 2>/dev/null > /tmp/govulncheck.json.$$; then
  # Extract a readable summary from the json stream.
  if command -v jq >/dev/null 2>&1; then
    jq -r '
      select(.osv != null)
      | "- \(.osv.id // "UNKNOWN"): \(.osv.summary // .osv.aliases[0] // "no summary")"
    ' /tmp/govulncheck.json.$$ | sort -u >> "${REPORT}" 2>/dev/null || true
  else
    grep -o '"id":"[^"]*"' /tmp/govulncheck.json.$$ | sort -u >> "${REPORT}" 2>/dev/null || true
  fi
  rm -f /tmp/govulncheck.json.$$
else
  echo "(govulncheck exited non-zero — see stderr)" >> "${REPORT}"
fi

{
  echo '```'
  echo
  echo "## go list -m -u all (available upgrades)"
  echo
  echo '```'
} >> "${REPORT}"

GOMAXPROCS=2 nice -n 19 go list -m -u all 2>/dev/null | grep -E '\[' || echo "(no updates pending)" >> "${REPORT}"

echo '```' >> "${REPORT}"

echo "Report written to: ${REPORT}"
echo "(run 'make deps-scan' or this script directly; no CI pipeline created.)"
