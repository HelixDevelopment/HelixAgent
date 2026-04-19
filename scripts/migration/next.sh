#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# scripts/migration/next.sh — identify the next-easiest migration site.
#
# Scoring rubric (lower = easier):
#   + test-coupling count × 10   (grep _test.go for .mu.|.<field>[)
#   + file size (KB)
#   + known-blocker tag          (huge penalty)

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

ALLOW="scripts/concurrency-audit-allowlist.txt"

# Known blockers.
is_blocker() {
    case "$1" in
        protocol_discovery.go|acp_client.go|mcp_client.go|acp_manager.go|boot_manager.go|\
protocol_federation.go|debate_service.go|concurrency_monitor.go|lsp_manager.go|\
provider_registry.go|debate_service_test.go|lsp_manager_test.go)
            return 0 ;;
        *) return 1 ;;
    esac
}

tmp=$(mktemp)

while IFS=: read -r file line name; do
    [[ -z "$file" ]] && continue
    [[ "$file" =~ ^[[:space:]]*# ]] && continue
    [[ "$file" == *_test.go ]] && continue
    [[ "$file" == *_test_helpers.go ]] && continue

    base=$(basename "$file")
    test_file="${file%.go}_test.go"

    coupling=0
    if [[ -f "$test_file" ]]; then
        coupling=$(grep -cE '\.mu\.(R?Lock|R?Unlock)|\.(sessions|agents|tools|cache|entries|providers|workers|handlers|connections|bindings|listeners|clients|managers|history|state|rules|subscriptions|tasks|breakers|nodes|metrics|services|reports|templates|config|items|data|records)\[' "$test_file" 2>/dev/null)
        coupling=${coupling:-0}
    fi

    size_kb=0
    if [[ -f "$file" ]]; then
        bytes=$(wc -c < "$file" 2>/dev/null)
        bytes=${bytes:-0}
        size_kb=$(( bytes / 1024 ))
    fi

    blocker=0
    if is_blocker "$base"; then
        blocker=1000
    fi

    score=$(( coupling * 10 + size_kb + blocker ))
    printf "%05d|%03d|%03dKB|%s:%s:%s\n" "$score" "$coupling" "$size_kb" "$file" "$line" "$name" >> "$tmp"
done < "$ALLOW"

echo
echo "RANK  SCORE  COUPLE  SIZE    SITE"
echo "----  -----  ------  -----   ----------------------------------------"
sort -n "$tmp" | grep -vE '^[[:space:]]*$' | head -15 | awk -F'|' '{
    rank = NR
    score = $1
    couple = $2
    size = $3
    site = $4
    printf " #%-3d  %5s   %4s   %-6s  %s\n", rank, score+0, couple+0, size, site
}'
echo
echo "Pick #1 unless you have reason not to."
echo "  score < 50    = Tier-A easy   (~12 min each)"
echo "  score 50-200  = Tier-B moderate (~25 min each)"
echo "  score > 1000  = known blocker (skip — dedicated session needed)"
echo

rm -f "$tmp"
