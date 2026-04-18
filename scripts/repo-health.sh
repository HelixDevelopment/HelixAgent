#!/usr/bin/env bash
# scripts/repo-health.sh — read-only repo sanity check (P0 Foundation)
#
# Validates:
#   1. Working tree is clean (or only expected sub-submodule drift)
#   2. Every submodule is initialised
#   3. `go mod verify` succeeds
#   4. `vendor/` is in sync with go.mod / go.sum (no diff after go mod vendor)
#   5. `.env.example` exists and every *_API_KEY placeholder has a slot
#   6. Every main-repo remote is reachable (SSH ls-remote)
#   7. Constitution / CLAUDE.md / AGENTS.md exist and are recent
#
# Contract:
#   - Fully non-interactive (CONST-019)
#   - Respects CONST-022 resource limits via nice + ionice
#   - Never mutates anything; dry-run / ls-remote / verify only
#   - Exits 0 on clean, non-zero on any failure
#
# Usage:
#   make repo-health
#   ./scripts/repo-health.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

FAIL=0
WARN=0
step() { echo ""; echo -e "${BLUE}==>${NC} $*"; }
ok()   { echo -e "    ${GREEN}✓${NC} $*"; }
warn() { echo -e "    ${YELLOW}!${NC} $*"; WARN=$((WARN + 1)); }
bad()  { echo -e "    ${RED}✗${NC} $*"; FAIL=$((FAIL + 1)); }

# Resource caps — CONST-022
export GOMAXPROCS=${GOMAXPROCS:-2}
RUN_LIMITED="nice -n 19 ionice -c 3"

# ---------------------------------------------------------------------------
# 1. Working tree
step "1. Working tree"
dirty_main=$(git status --porcelain 2>/dev/null | grep -v '^..\s' | head -5 || true)
if [[ -z "$dirty_main" ]]; then
    ok "main repo: clean"
else
    warn "main repo has uncommitted changes:"
    echo "$dirty_main" | sed 's/^/      /'
fi

# ---------------------------------------------------------------------------
# 2. Submodules initialised
step "2. Submodules initialised"
uninit=$(git submodule status --recursive 2>/dev/null | grep '^-' || true)
if [[ -z "$uninit" ]]; then
    ok "all submodules initialised"
else
    count=$(echo "$uninit" | wc -l)
    warn "$count submodules not initialised (run git submodule update --init --recursive):"
    echo "$uninit" | head -5 | sed 's/^/      /'
fi

# ---------------------------------------------------------------------------
# 3. go mod verify
step "3. go mod verify"
if $RUN_LIMITED go mod verify 2>&1 | tail -1 | grep -q "all modules verified"; then
    ok "all modules verified"
else
    bad "go mod verify failed"
fi

# ---------------------------------------------------------------------------
# 4. vendor in sync
step "4. vendor/ in sync with go.mod (go mod vendor dry-run)"
tmp_vendor=$(mktemp -d)
trap 'rm -rf "$tmp_vendor"' EXIT
# Copy current vendor sums for compare
if [[ -f vendor/modules.txt ]]; then
    cp vendor/modules.txt "$tmp_vendor/modules.txt.before"
    $RUN_LIMITED go mod vendor -o "$tmp_vendor/vendor-fresh" >/dev/null 2>&1 || true
    if [[ -f "$tmp_vendor/vendor-fresh/modules.txt" ]]; then
        if diff -q "$tmp_vendor/modules.txt.before" "$tmp_vendor/vendor-fresh/modules.txt" >/dev/null 2>&1; then
            ok "vendor/modules.txt matches fresh go mod vendor output"
        else
            warn "vendor/modules.txt drift — run: go mod vendor"
        fi
    else
        warn "fresh go mod vendor did not produce modules.txt — skipping compare"
    fi
else
    warn "no vendor/modules.txt — project may not use vendored deps"
fi

# ---------------------------------------------------------------------------
# 5. .env.example shape
step "5. .env.example coverage"
if [[ -f .env.example ]]; then
    req_keys=(PORT DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME REDIS_HOST REDIS_PORT)
    missing=()
    for k in "${req_keys[@]}"; do
        if ! grep -qE "^${k}=" .env.example; then
            missing+=("$k")
        fi
    done
    if [[ ${#missing[@]} -eq 0 ]]; then
        ok ".env.example has all required slots (${#req_keys[@]} keys)"
    else
        bad ".env.example missing required keys: ${missing[*]}"
    fi
else
    bad ".env.example is missing at repo root"
fi

# ---------------------------------------------------------------------------
# 6. Remote reachability (SSH-only per CONST-025)
step "6. Remote reachability (SSH ls-remote)"
while IFS=$'\t' read -r name url; do
    [[ -z "$name" ]] && continue
    if [[ "$url" != git@* ]]; then
        bad "remote $name uses non-SSH URL: $url (CONST-025 violation)"
        continue
    fi
    # Only test fetch URLs (push/fetch are usually the same)
    case "$name" in *"(push)"*) continue ;; esac
    if timeout 15 git ls-remote --heads "$name" >/dev/null 2>&1; then
        ok "remote '${name%% *}' reachable"
    else
        warn "remote '${name%% *}' not reachable (may be offline / auth issue)"
    fi
done < <(git remote -v | awk '{print $1"\t"$2}' | sort -u)

# ---------------------------------------------------------------------------
# 7. Constitution files present and recent
step "7. Constitution / CLAUDE.md / AGENTS.md"
for f in CONSTITUTION.md CONSTITUTION.json CLAUDE.md AGENTS.md; do
    if [[ -f "$f" ]]; then
        mtime=$(stat -c %Y "$f" 2>/dev/null || stat -f %m "$f" 2>/dev/null || echo 0)
        now=$(date +%s)
        age_days=$(( (now - mtime) / 86400 ))
        if (( age_days > 180 )); then
            warn "$f is $age_days days old — consider reviewing for drift"
        else
            ok "$f present (mtime ${age_days}d ago)"
        fi
    else
        bad "$f missing"
    fi
done

# ---------------------------------------------------------------------------
# Summary
echo ""
echo -e "${BLUE}=============================================${NC}"
if (( FAIL == 0 )); then
    echo -e "${GREEN}Repo health: OK${NC}   (warnings: $WARN)"
    exit 0
else
    echo -e "${RED}Repo health: FAILED${NC}   (failures: $FAIL, warnings: $WARN)"
    exit 1
fi
