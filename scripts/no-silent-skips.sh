#!/usr/bin/env bash
# Fail if any test skip directive is present without a SKIP-OK: #<ticket> annotation.
#
# Part of the Definition of Done enforcement arm (see docs/development/definition-of-done.md).
# A skipped test is invisible debt — this script makes that debt loud. If a skip is
# genuinely needed, annotate it with a ticket reference and keep the work tracked:
#
#   t.Skip("flake under race — SKIP-OK: #1234")
#
# Wired into `make no-silent-skips` and therefore `make ci-validate-all`.

set -uo pipefail

cd "$(dirname "$0")/.."

# Skip patterns across Go / Kotlin / Java / TS / JS / Python.
PATTERNS='t\.Skip\(|@Ignore\b|\bxit\(|\.skip\(|@pytest\.mark\.skip|@unittest\.skip'

# File extensions to scan.
INCLUDES=(--include='*.go' --include='*.kt' --include='*.java'
          --include='*.ts' --include='*.tsx' --include='*.js' --include='*.jsx'
          --include='*.py')

# Third-party / vendored / external trees — excluded per Rule #10.
EXCLUDES=(--exclude-dir=.git --exclude-dir=vendor --exclude-dir=node_modules
          --exclude-dir=external --exclude-dir=MCP --exclude-dir=cli_agents
          --exclude-dir=mcp-servers --exclude-dir=releases
          --exclude-dir=reports --exclude-dir=test-results)

violations=$(grep -rnE "$PATTERNS" "${INCLUDES[@]}" "${EXCLUDES[@]}" . 2>/dev/null \
             | grep -v -E 'SKIP-OK: #[0-9]+' || true)

if [ -n "$violations" ]; then
  count=$(printf '%s\n' "$violations" | wc -l | tr -d ' ')
  echo "⚠️  $count silent-skip violation(s) detected." >&2
  echo "" >&2
  printf '%s\n' "$violations" | head -30 >&2
  if [ "$count" -gt 30 ]; then
    echo "... ($((count - 30)) more — re-run '$0' without head)" >&2
  fi
  echo "" >&2
  echo "Annotate each with a trailing '// SKIP-OK: #<ticket>' (or '# SKIP-OK: #<ticket>') comment" >&2
  echo "so the skip is tracked, or remove the skip if it is no longer needed." >&2
  if [ "${NO_SILENT_SKIPS_WARN_ONLY:-0}" = "1" ]; then
    echo "" >&2
    echo "(warn-only mode — set NO_SILENT_SKIPS_WARN_ONLY=0 to fail the build on violations)" >&2
    exit 0
  fi
  exit 1
fi

echo "no-silent-skips: OK (no unannotated skip directives found)"
