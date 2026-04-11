#!/usr/bin/env bash
# secrets-scan.sh — Repository secret scanning via gitleaks.
#
# Detects committed credentials, API keys, private keys, and tokens. Runs
# over the working tree AND the git history (full repo scan). Results go
# to reports/security/secrets-<timestamp>.{sarif,md}.
#
# Non-interactive: no sudo, no network. Only reads the local git repo.
# Exit codes:
#   0 — no secrets found (report still generated)
#   1 — gitleaks not installed
#   2 — scan invocation failed
#   3 — secrets found (details in report)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORTS_DIR="${REPO_ROOT}/reports/security"
TIMESTAMP="$(date -u +%Y-%m-%dT%H-%M-%SZ)"
SARIF="${REPORTS_DIR}/secrets-${TIMESTAMP}.sarif"
REPORT="${REPORTS_DIR}/secrets-${TIMESTAMP}.md"

mkdir -p "${REPORTS_DIR}"

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "error: gitleaks is not installed." >&2
  echo "install: https://github.com/gitleaks/gitleaks#installing" >&2
  exit 1
fi

cd "${REPO_ROOT}"

{
  echo "# Secrets Scan Report — ${TIMESTAMP}"
  echo
  echo "## Environment"
  echo
  echo "- Repo: \`${REPO_ROOT}\`"
  echo "- gitleaks: \`$(gitleaks version 2>&1 | head -1)\`"
  echo
  echo "## Results"
  echo
} > "${REPORT}"

set +e
GOMAXPROCS=2 nice -n 19 gitleaks detect \
  --source="${REPO_ROOT}" \
  --report-format=sarif \
  --report-path="${SARIF}" \
  --no-banner \
  --redact \
  --exit-code=3 \
  > /tmp/gitleaks.out.$$ 2>&1
SCAN_EXIT=$?
set -e

case ${SCAN_EXIT} in
  0)
    echo "✓ No secrets detected." >> "${REPORT}"
    ;;
  3)
    echo "**⚠ Secrets found.** See SARIF: \`${SARIF}\`" >> "${REPORT}"
    echo >> "${REPORT}"
    echo '```' >> "${REPORT}"
    tail -60 /tmp/gitleaks.out.$$ >> "${REPORT}" 2>/dev/null || true
    echo '```' >> "${REPORT}"
    ;;
  *)
    echo "gitleaks exited ${SCAN_EXIT} — scan may be incomplete." >> "${REPORT}"
    cat /tmp/gitleaks.out.$$ >> "${REPORT}" 2>/dev/null || true
    rm -f /tmp/gitleaks.out.$$
    echo "Report: ${REPORT}"
    exit 2
    ;;
esac

rm -f /tmp/gitleaks.out.$$

echo "Report written to: ${REPORT}"
echo "SARIF: ${SARIF}"

# Surface a non-zero exit code if secrets were found — callers can decide
# what to do (CI ban means we cannot automate gating, but local use should
# still propagate failure so humans notice).
if [ "${SCAN_EXIT}" -eq 3 ]; then
  exit 3
fi
