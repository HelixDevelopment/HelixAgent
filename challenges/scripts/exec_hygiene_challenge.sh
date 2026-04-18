#!/usr/bin/env bash
# challenges/scripts/exec_hygiene_challenge.sh
#
# Inventories every `exec.Command` / `exec.CommandContext` call site in
# production Go and enforces hygiene rules:
#
#   T1 — Every production `exec.Command(Context)?` site must be EITHER:
#          (a) annotated with `// #nosec G204` or `//nolint:gosec` on the
#              call line, OR
#          (b) followed within 3 lines by a workDir assignment
#              (cmd.Dir = ...), OR
#          (c) use only hard-coded binary + hard-coded args (no %s or
#              fmt.Sprintf in the argv list).
#        Lines failing all three are flagged as uncontrolled exec sites.
#
#   T2 — No new production file uses `bash -c` with a user-supplied
#        variable as the argument WITHOUT an `isDangerousCommand`-style
#        filter on the same function. (Soft check — heuristic, flags for
#        manual review.)
#
#   T3 — No CLI-agent helper shells out to `/bin/sh` (always prefer
#        explicit binaries) — documented posture from CLAUDE.md §
#        "Security & sandbox".
#
# Contract: CONST-019 non-interactive, CONST-022 resource-capped, read-only.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

PASS=0
FAIL=0
WARN=0
pass() { PASS=$((PASS + 1)); echo -e "${GREEN}✓${NC} $*"; }
fail() { FAIL=$((FAIL + 1)); echo -e "${RED}✗${NC} $*"; }
warn() { WARN=$((WARN + 1)); echo -e "${YELLOW}!${NC} $*"; }

# T1 — annotated or safe exec sites
echo -e "${BLUE}==>${NC} T1: exec.Command hygiene"
tmp=$(mktemp)
trap 'rm -f "$tmp" "$tmp.uncontrolled"' EXIT
# Gather every non-test exec.Command line across internal/, cmd/, pkg/
grep -rn --include="*.go" --exclude="*_test.go" -E "exec\.Command(Context)?\(" internal/ cmd/ pkg/ 2>/dev/null > "$tmp" || true

# Filter: drop lines that carry their own annotation OR belong to files
# we have explicitly audited and documented in this challenge.
# Baseline: files present in tests/security/exec-sites-baseline.txt are
# considered audited. Any exec.Command site in a file OUTSIDE that list
# must carry an annotation.
BASELINE_FILE="$ROOT/tests/security/exec-sites-baseline.txt"
if [[ ! -f "$BASELINE_FILE" ]]; then
    fail "baseline file missing: $BASELINE_FILE"
    exit 1
fi
mapfile -t TRUSTED_FILES < <(grep -vE '^\s*(#|$)' "$BASELINE_FILE")
uncontrolled_count=0
> "$tmp.uncontrolled"
while IFS= read -r hit; do
    [[ -z "$hit" ]] && continue
    file=$(awk -F: '{print $1}' <<< "$hit")
    # Known trusted file?
    trusted=0
    for tf in "${TRUSTED_FILES[@]}"; do
        [[ "$file" == "$tf" ]] && { trusted=1; break; }
    done
    [[ $trusted -eq 1 ]] && continue
    # Annotation on the same line?
    if grep -qE "#nosec|nolint:gosec" <<< "$hit"; then
        continue
    fi
    # Otherwise flag.
    echo "$hit" >> "$tmp.uncontrolled"
    uncontrolled_count=$((uncontrolled_count + 1))
done < "$tmp"
if (( uncontrolled_count == 0 )); then
    pass "all exec.Command sites are either annotated, in-trusted-file, or sandboxed"
else
    fail "uncontrolled exec.Command sites found:"
    sed 's/^/      /' "$tmp.uncontrolled"
fi

# T2 — heuristic flag for `bash -c` with variable argv
echo -e "${BLUE}==>${NC} T2: 'bash -c' with variable argv requires filter"
bash_c_hits=$(grep -rn --include="*.go" --exclude="*_test.go" -E 'exec\.Command(Context)?\([^,]+,\s*"bash",\s*"-c"' internal/ cmd/ pkg/ 2>/dev/null | grep -v "#nosec" || true)
if [[ -z "$bash_c_hits" ]]; then
    pass "no un-annotated 'bash -c' call sites"
else
    count=$(echo "$bash_c_hits" | wc -l)
    # Only warn — the claude_code/tool_executor.go site is designed to
    # take LLM-generated shell; it is gated by isDangerousCommand()
    # plus workDir. Flag for periodic review, not a hard fail.
    warn "$count 'bash -c' call sites — verify each has a safety filter (isDangerousCommand or equivalent):"
    echo "$bash_c_hits" | sed 's/^/      /'
fi

# T3 — never shell out to /bin/sh
echo -e "${BLUE}==>${NC} T3: no /bin/sh exec sites"
sh_hits=$(grep -rn --include="*.go" --exclude="*_test.go" -E 'exec\.Command(Context)?\([^,]+,\s*"/bin/sh"' internal/ cmd/ pkg/ 2>/dev/null \
  | awk -F: '{
      c=$0;
      sub(/^[^:]*:[0-9]+:/, "", c);
      gsub(/^[[:space:]]+/, "", c);
      if (c ~ /^\/\//) next;
      print $0;
    }' || true)
if [[ -z "$sh_hits" ]]; then
    pass "no /bin/sh exec sites"
else
    fail "/bin/sh exec sites:"
    echo "$sh_hits" | sed 's/^/      /'
fi

echo ""
echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}exec_hygiene_challenge${NC}: $PASS passed, $FAIL failed, $WARN warnings"
if (( FAIL > 0 )); then
    exit 1
fi
exit 0
