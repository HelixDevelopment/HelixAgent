#!/usr/bin/env bash
# Bulk-apply the DoD drop-in to a list of sibling projects.
#
# For each project:
#   1. Copies scripts/no-silent-skips.sh + scripts/demo-all.sh
#   2. Appends a DoD clause + placeholder acceptance demo to CLAUDE.md (idempotent)
#   3. Appends gate targets to Makefile (idempotent) — creates one if absent
#   4. Commits with a standard message (idempotent — no-op if no changes)
#   5. Pushes to every configured remote
#
# Usage:
#   bulk-apply.sh /path/to/projects-root [project-name ...]
#
# If no project names are given, reads the default list below.

set -uo pipefail

SRC="$(cd "$(dirname "$0")" && pwd)"
TARGET_ROOT="${1:?usage: bulk-apply.sh <projects-root> [project-name ...]}"
shift || true

DEFAULT_TARGETS=(
  # Go
  Auth Concurrency Config Database DocProcessor Document Formatters
  HelixCode HelixTranslate LLMOrchestrator LLMProvider My-Patreon-Manager
  RateLimiter Security Storage VisionEngine
  # Gradle / KMP
  Auth-KMP Bear-Mail Concurrency-KMP Config-KMP Database-KMP Document-KMP
  Formatters-KMP RateLimiter-KMP Security-KMP Storage-KMP UI-Components-KMP
  # Node
  CCode-Private
)

if [ "$#" -gt 0 ]; then
  TARGETS=("$@")
else
  TARGETS=("${DEFAULT_TARGETS[@]}")
fi

ok=0; changed=0; skipped=0; failed=0
ok_list=(); changed_list=(); skipped_list=(); failed_list=()

append_if_missing() {
  local file="$1" marker="$2" block_file="$3"
  if [ -f "$file" ] && grep -q -F "$marker" "$file"; then
    return 1  # already present
  fi
  mkdir -p "$(dirname "$file")"
  [ -f "$file" ] || touch "$file"
  printf '\n' >> "$file"
  cat "$block_file" >> "$file"
  return 0
}

apply_one() {
  local proj="$1"
  local dir="$TARGET_ROOT/$proj"
  [ -d "$dir/.git" ] || { echo "[SKIP] $proj (not a git repo)"; skipped=$((skipped+1)); skipped_list+=("$proj"); return; }
  (
    cd "$dir" || exit 2
    local did=0

    # 1. Scripts.
    if [ ! -f scripts/no-silent-skips.sh ] || ! cmp -s "$SRC/scripts/no-silent-skips.sh" scripts/no-silent-skips.sh; then
      mkdir -p scripts && cp "$SRC/scripts/no-silent-skips.sh" scripts/ && chmod +x scripts/no-silent-skips.sh
      did=1
    fi
    if [ ! -f scripts/demo-all.sh ] || ! cmp -s "$SRC/scripts/demo-all.sh" scripts/demo-all.sh; then
      mkdir -p scripts && cp "$SRC/scripts/demo-all.sh" scripts/ && chmod +x scripts/demo-all.sh
      did=1
    fi

    # 2. CLAUDE.md clause.
    local marker="## Definition of Done"
    if [ -f CLAUDE.md ] && ! grep -q -F "$marker" CLAUDE.md; then
      cat >> CLAUDE.md <<'CLAUSE'

## Definition of Done

A change is NOT done because code compiles and tests pass. "Done" requires pasted
terminal output from a real run of the real system, produced in the same session as
the change. Coverage and passing suites measure the LLM's model of the product, not
the product.

1. **No self-certification.** *Verified, tested, working, complete, fixed, passing*
   are forbidden in commits, PRs, and agent replies without accompanying pasted
   output from a same-session real-system run.
2. **Demo before code.** Every task begins with the runnable acceptance demo below.
3. **Real system.** Demos run against real artifacts — built binaries, live
   databases, instrumented devices — not mocks/stubs/in-memory fakes.
4. **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `it.skip` without a trailing
   `SKIP-OK: #<ticket>` annotation fails `make ci-validate-all`.
5. **Contract tests on every seam.** Any change touching a module↔module boundary
   runs one roundtrip test asserting the wire format on both sides.
6. **Evidence in the PR.** PR body contains a fenced `## Demo` block with exact
   command(s) + output.

### Acceptance demo for this module

```bash
# TODO — replace with a 10-line real-system demo. See examples in
# HelixAgent/docs/development/dod-dropin/templates/CLAUDE_md_clause.md
```
CLAUSE
      did=1
    elif [ ! -f CLAUDE.md ]; then
      cat > CLAUDE.md <<CLAUSE
# CLAUDE.md

Guidance for Claude Code on this project.

## Definition of Done

A change is NOT done because code compiles and tests pass. "Done" requires pasted
terminal output from a real run of the real system, produced in the same session as
the change.

1. No self-certification without pasted output from a same-session real-system run.
2. Demo before code.
3. Real system every time — no mocks/stubs as proof-of-done.
4. Skips are loud — \`SKIP-OK: #<ticket>\` or bust.
5. Contract tests on every seam.
6. Evidence in the PR.

### Acceptance demo for this module

\`\`\`bash
# TODO — replace with a 10-line real-system demo.
\`\`\`
CLAUSE
      did=1
    fi

    # 3. Makefile targets.
    if [ -f Makefile ]; then
      if ! grep -q 'no-silent-skips-warn:' Makefile; then
        cat >> Makefile <<'MAKE'

# Definition of Done gates — portable drop-in from HelixAgent
.PHONY: no-silent-skips no-silent-skips-warn demo-all demo-all-warn demo-one ci-validate-all

no-silent-skips:
	@bash scripts/no-silent-skips.sh

no-silent-skips-warn:
	@NO_SILENT_SKIPS_WARN_ONLY=1 bash scripts/no-silent-skips.sh

demo-all:
	@bash scripts/demo-all.sh

demo-all-warn:
	@DEMO_ALL_WARN_ONLY=1 DEMO_ALLOW_TODO=1 bash scripts/demo-all.sh

demo-one:
	@DEMO_MODULES="$(MOD)" bash scripts/demo-all.sh

ci-validate-all: no-silent-skips-warn demo-all-warn
	@echo "ci-validate-all: all gates executed"
MAKE
        did=1
      fi
    else
      cat > Makefile <<'MAKE'
.PHONY: no-silent-skips no-silent-skips-warn demo-all demo-all-warn demo-one ci-validate-all

no-silent-skips:
	@bash scripts/no-silent-skips.sh

no-silent-skips-warn:
	@NO_SILENT_SKIPS_WARN_ONLY=1 bash scripts/no-silent-skips.sh

demo-all:
	@bash scripts/demo-all.sh

demo-all-warn:
	@DEMO_ALL_WARN_ONLY=1 DEMO_ALLOW_TODO=1 bash scripts/demo-all.sh

demo-one:
	@DEMO_MODULES="$(MOD)" bash scripts/demo-all.sh

ci-validate-all: no-silent-skips-warn demo-all-warn
	@echo "ci-validate-all: all gates executed"
MAKE
      did=1
    fi

    # 4. .gitignore for reports/demos
    if ! grep -q 'reports/demos/' .gitignore 2>/dev/null; then
      printf '\nreports/demos/\n' >> .gitignore
      did=1
    fi

    # 5. Commit + push.
    if [ "$did" -eq 1 ] && [ -n "$(git status --porcelain)" ]; then
      git add scripts/no-silent-skips.sh scripts/demo-all.sh Makefile CLAUDE.md .gitignore 2>/dev/null
      if ! git diff --cached --quiet; then
        git commit -q -m "chore(dod): install Definition of Done gates (warn-only)

Portable drop-in from HelixAgent/docs/development/dod-dropin/.

- scripts/no-silent-skips.sh: detects un-annotated test skip directives.
- scripts/demo-all.sh: auto-discovers every CLAUDE.md acceptance demo
  and runs it end-to-end with a timeout.
- Makefile: wires \\\`make ci-validate-all\\\` to run the gates in warn-only
  mode during the transition.
- CLAUDE.md: adds the six-clause Definition of Done and a placeholder
  acceptance-demo block. Replace the TODO with a real 10-line demo that
  proves THIS project works end-to-end against its real dependencies.

Rationale: high test coverage does not prove the product works.
See HelixAgent/docs/development/definition-of-done.md.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
"
        # Push to every configured remote on the current branch.
        local branch
        branch=$(git symbolic-ref --short HEAD 2>/dev/null || echo "main")
        while read -r remote _; do
          [ -z "$remote" ] && continue
          git push -q "$remote" "$branch" 2>/dev/null || true
        done < <(git remote -v | awk '{print $1}' | sort -u | while read -r r; do echo "$r"; done)
        echo "[CHANGE] $proj (committed + pushed)"
        return 0
      fi
    fi
    echo "[OK]     $proj (already up to date)"
    return 3
  )
  local rc=$?
  case "$rc" in
    0) changed=$((changed+1)); changed_list+=("$proj") ;;
    2) failed=$((failed+1)); failed_list+=("$proj") ;;
    *) ok=$((ok+1)); ok_list+=("$proj") ;;
  esac
}

for proj in "${TARGETS[@]}"; do
  apply_one "$proj"
done

echo
echo "================================================================"
echo "bulk-apply totals: CHANGED=$changed OK=$ok SKIPPED=$skipped FAILED=$failed"
echo "================================================================"
[ "$changed" -gt 0 ]  && { echo "CHANGED:"; printf '  - %s\n' "${changed_list[@]}"; }
[ "$ok" -gt 0 ]       && { echo "OK (already installed):"; printf '  - %s\n' "${ok_list[@]}"; }
[ "$skipped" -gt 0 ]  && { echo "SKIPPED:"; printf '  - %s\n' "${skipped_list[@]}"; }
[ "$failed" -gt 0 ]   && { echo "FAILED:"; printf '  - %s\n' "${failed_list[@]}"; }
[ "$failed" -eq 0 ]
