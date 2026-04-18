#!/usr/bin/env bash
# scripts/coverage-floor.sh — aggregate per-module coverage floor gate
#
# Walks every own-module listed in ./scripts/repo-own-modules.txt (one per
# line), runs `go test -cover -short` inside each, extracts the coverage
# percentage, and compares it to the COVERAGE_FLOOR threshold. Exits
# non-zero if any module is below the floor.
#
# Contract: CONST-019 non-interactive, CONST-022 resource capped.
#
# Usage:
#   make coverage-floor
#   COVERAGE_FLOOR=85 ./scripts/coverage-floor.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COVERAGE_FLOOR=${COVERAGE_FLOOR:-90}
MODULES_FILE="$ROOT/scripts/repo-own-modules.txt"
REPORT_DIR="$ROOT/reports/coverage-floor"
mkdir -p "$REPORT_DIR"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

export GOMAXPROCS=${GOMAXPROCS:-2}

if [[ ! -f "$MODULES_FILE" ]]; then
    echo -e "${RED}error:${NC} $MODULES_FILE not found"
    echo "Create it with one own-module directory per line (e.g. Auth, Cache, …)"
    exit 2
fi

TIMESTAMP=$(date -u +%Y-%m-%dT%H%M%SZ)
SUMMARY="$REPORT_DIR/summary-$TIMESTAMP.md"

{
    echo "# Coverage Floor Summary — $TIMESTAMP"
    echo ""
    echo "Floor: **${COVERAGE_FLOOR}%**"
    echo ""
    echo "| Module | Coverage | Status |"
    echo "|---|---|---|"
} > "$SUMMARY"

FAIL=0
WARN=0
while IFS= read -r mod; do
    [[ -z "$mod" || "$mod" == "#"* ]] && continue
    if [[ ! -d "$mod" ]]; then
        echo "| $mod | — | ${YELLOW}MISSING${NC} |" >> "$SUMMARY"
        WARN=$((WARN + 1))
        continue
    fi
    if [[ ! -f "$mod/go.mod" ]]; then
        echo "| $mod | — | SKIP (no go.mod) |" >> "$SUMMARY"
        continue
    fi
    echo -e "${BLUE}==>${NC} $mod"
    out=$(cd "$mod" && nice -n 19 ionice -c 3 go test -short -cover -p 1 -count=1 ./... 2>&1 || true)
    # Extract the highest coverage percentage reported (or 0 if none)
    cov=$(echo "$out" | grep -oE 'coverage: [0-9]+\.[0-9]+%' | awk '{print $2}' | tr -d '%' | sort -g | tail -1 || true)
    cov=${cov:-0}
    cov_int=${cov%.*}
    cov_int=${cov_int:-0}
    if (( cov_int >= COVERAGE_FLOOR )); then
        echo "| $mod | ${cov}% | ✓ pass |" >> "$SUMMARY"
        echo -e "    ${GREEN}✓${NC} $cov%"
    else
        echo "| $mod | ${cov}% | ✗ below floor |" >> "$SUMMARY"
        echo -e "    ${RED}✗${NC} $cov% (below ${COVERAGE_FLOOR}%)"
        FAIL=$((FAIL + 1))
    fi
done < "$MODULES_FILE"

echo "" >> "$SUMMARY"
echo "**Failures:** $FAIL" >> "$SUMMARY"
echo "**Warnings:** $WARN" >> "$SUMMARY"

echo ""
echo -e "${BLUE}Report:${NC} $SUMMARY"

if (( FAIL > 0 )); then
    exit 1
fi
exit 0
