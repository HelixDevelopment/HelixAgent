#!/usr/bin/env bash
# challenges/scripts/repo_hygiene_challenge.sh — validates repo hygiene
# contracts enforced by Phase P0 of the unfinished-work plan.
#
# Asserts:
#   T1 - scripts/repo-health.sh exists and is executable
#   T2 - scripts/coverage-floor.sh exists and is executable
#   T3 - scripts/metrics-snapshot.sh exists and is executable
#   T4 - scripts/repo-own-modules.txt exists and lists the 41 expected own-modules
#   T5 - Every own-module directory listed in repo-own-modules.txt is present
#   T6 - No forbidden CI pipeline files exist
#       (.github/workflows/, .gitlab-ci.yml, Jenkinsfile, .travis.yml, .circleci/)
#   T7 - Every git remote in the main repo uses SSH (CONST-025)
#   T8 - Makefile has the four new P0 targets
#   T9 - repo-health.sh exits zero when invoked
#
# Contract: CONST-019 non-interactive, CONST-022 resource capped,
# CONST-023 no CI infrastructure.
#
# Usage:
#   ./challenges/scripts/repo_hygiene_challenge.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

export GOMAXPROCS=${GOMAXPROCS:-2}

PASS=0
FAIL=0
EXPECTED_OWN_MODULES=41

pass() { PASS=$((PASS + 1)); echo -e "${GREEN}✓${NC} $*"; }
fail() { FAIL=$((FAIL + 1)); echo -e "${RED}✗${NC} $*"; }

echo -e "${BLUE}==>${NC} T1: scripts/repo-health.sh"
[[ -x "$ROOT/scripts/repo-health.sh" ]] && pass "repo-health.sh exists + executable" \
    || fail "repo-health.sh missing or not executable"

echo -e "${BLUE}==>${NC} T2: scripts/coverage-floor.sh"
[[ -x "$ROOT/scripts/coverage-floor.sh" ]] && pass "coverage-floor.sh exists + executable" \
    || fail "coverage-floor.sh missing or not executable"

echo -e "${BLUE}==>${NC} T3: scripts/metrics-snapshot.sh"
[[ -x "$ROOT/scripts/metrics-snapshot.sh" ]] && pass "metrics-snapshot.sh exists + executable" \
    || fail "metrics-snapshot.sh missing or not executable"

echo -e "${BLUE}==>${NC} T4: scripts/repo-own-modules.txt"
if [[ -f "$ROOT/scripts/repo-own-modules.txt" ]]; then
    count=$(grep -cE '^[A-Za-z]' "$ROOT/scripts/repo-own-modules.txt" || echo 0)
    if (( count == EXPECTED_OWN_MODULES )); then
        pass "repo-own-modules.txt lists $count own-modules (expected $EXPECTED_OWN_MODULES)"
    else
        fail "repo-own-modules.txt lists $count, expected $EXPECTED_OWN_MODULES"
    fi
else
    fail "repo-own-modules.txt missing"
fi

echo -e "${BLUE}==>${NC} T5: Every own-module directory present"
if [[ -f "$ROOT/scripts/repo-own-modules.txt" ]]; then
    missing=0
    while IFS= read -r mod; do
        [[ -z "$mod" || "$mod" == "#"* ]] && continue
        if [[ ! -d "$ROOT/$mod" ]]; then
            fail "own-module $mod missing from working tree"
            missing=$((missing + 1))
        fi
    done < "$ROOT/scripts/repo-own-modules.txt"
    if (( missing == 0 )); then
        pass "all own-module directories present"
    fi
fi

echo -e "${BLUE}==>${NC} T6: No forbidden CI pipeline files"
forbidden_hits=0
for p in .github/workflows .gitlab-ci.yml Jenkinsfile .travis.yml .circleci; do
    if [[ -e "$ROOT/$p" ]]; then
        fail "forbidden CI path exists: $p (CONST-023 violation)"
        forbidden_hits=$((forbidden_hits + 1))
    fi
done
if (( forbidden_hits == 0 )); then
    pass "no forbidden CI pipeline files"
fi

echo -e "${BLUE}==>${NC} T7: Every main-repo remote uses SSH"
while IFS= read -r line; do
    name=$(awk '{print $1}' <<< "$line")
    url=$(awk '{print $2}' <<< "$line")
    [[ -z "$url" ]] && continue
    if [[ "$url" != git@* ]]; then
        fail "remote '$name' uses non-SSH URL: $url"
    fi
done < <(git -C "$ROOT" remote -v | sort -u)
# Only pass if no fails were recorded in this block
ssh_failed=$(git -C "$ROOT" remote -v | awk '$2 !~ /^git@/ {print $0}' | wc -l)
if (( ssh_failed == 0 )); then
    pass "all remotes SSH-only (CONST-025)"
fi

echo -e "${BLUE}==>${NC} T8: Makefile P0 targets"
missing_targets=()
for tgt in repo-health coverage-floor metrics-snapshot security-gates-all; do
    if ! grep -qE "^${tgt}:" "$ROOT/Makefile"; then
        missing_targets+=("$tgt")
    fi
done
if (( ${#missing_targets[@]} == 0 )); then
    pass "Makefile has all 4 P0 targets"
else
    fail "Makefile missing targets: ${missing_targets[*]}"
fi

echo -e "${BLUE}==>${NC} T9: repo-health.sh exits zero"
if "$ROOT/scripts/repo-health.sh" >/dev/null 2>&1; then
    pass "repo-health.sh clean run"
else
    fail "repo-health.sh non-zero exit"
fi

echo ""
echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}repo_hygiene_challenge${NC}: $PASS passed, $FAIL failed"
if (( FAIL > 0 )); then
    exit 1
fi
exit 0
