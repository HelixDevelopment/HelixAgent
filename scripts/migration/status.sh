#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# scripts/migration/status.sh — one-command status for the CONST-029
# Pattern-A migration campaign. Print:
#   - HEAD commit + whether pushes are current
#   - allowlist size (current vs. baseline 254)
#   - % drained
#   - recent migration commits
#   - blockers (sites we know need bigger work)
#   - next-candidate queue (Tier-A first)
#
# Read-only. Safe to run any time. Zero side effects.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

ALLOW="scripts/concurrency-audit-allowlist.txt"
BASELINE=254

BLUE=$'\033[1;34m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[0;33m'
RED=$'\033[0;31m'
DIM=$'\033[2m'
NC=$'\033[0m'

section() { printf "\n${BLUE}==>${NC} %s\n" "$*"; }

section "CONST-029 migration campaign status"
echo

# HEAD + push state.
head=$(git rev-parse HEAD)
head_short=$(git rev-parse --short HEAD)
branch=$(git branch --show-current)
msg=$(git log -1 --format=%s)
echo "  HEAD:   ${head_short}  (${branch})"
echo "  last:   ${msg}"
echo "  sha:    ${head}"
echo

# Per-remote sync check.
section "Remote push sync"
for remote in github gitlab githubhelixdevelopment; do
    if ! git ls-remote --quiet "$remote" HEAD >/dev/null 2>&1; then
        echo "  ${DIM}${remote}${NC}  (not reachable — skip)"
        continue
    fi
    remote_head=$(git ls-remote "$remote" "refs/heads/${branch}" 2>/dev/null | awk '{print $1}')
    if [[ -z "$remote_head" ]]; then
        echo "  ${DIM}${remote}${NC}  (no ${branch} on remote)"
    elif [[ "$remote_head" == "$head" ]]; then
        echo "  ${GREEN}✓${NC} ${remote}  in sync"
    else
        echo "  ${YELLOW}⚠${NC}  ${remote}  out of sync (remote: ${remote_head:0:8})"
    fi
done

# Allowlist counts.
section "Allowlist progress"
current=$(grep -vcE '^[[:space:]]*(#|$)' "$ALLOW" 2>/dev/null || echo 0)
drained=$((BASELINE - current))
if [[ $BASELINE -gt 0 ]]; then
    pct=$(awk -v d="$drained" -v b="$BASELINE" 'BEGIN { printf "%.1f", d*100/b }')
else
    pct="0.0"
fi
echo "  baseline:  ${BASELINE}"
echo "  remaining: ${current}"
echo "  drained:   ${drained}  (${pct}%)"

# Per-package breakdown of remaining.
section "Remaining by package (top 10)"
grep -vE '^[[:space:]]*(#|$)' "$ALLOW" 2>/dev/null \
    | awk -F: '{ n=split($1,a,"/"); print a[n-1] }' \
    | sort | uniq -c | sort -rn | head -10 \
    | awk '{ printf "  %4s  %s\n", $1, $2 }'

# Recent migration commits.
section "Recent migration commits"
git log --oneline --grep='^migrate(' -15 | awk '{ print "  " $0 }' | head -15

# Known blockers.
section "Known blockers (need dedicated session, skip in quick-drain)"
cat <<'EOF'
  ACPDiscoveryClient       internal/services/protocol_discovery.go  — 60+ test-file direct accesses
  LSPClient                internal/services/acp_client.go          — multi-map joint atomicity + long file
  MCPClient                internal/services/mcp_client.go          — HTTP transport + protocol state
  ACPManager / ACPClient   internal/services/acp_manager.go         — compound protocol state
  BootManager              internal/services/boot_manager.go        — backward-compat exposed Results field
  ProtocolDiscovery        internal/services/protocol_federation.go — 20+ test-internal accesses (tried, reverted)
  DebateService            internal/services/debate_service.go      — 6+ internal test accesses
  ConcurrencyMonitor       internal/services/concurrency_monitor.go — 20+ test accesses (flagged earlier)
EOF

# Next-candidate queue.
section "Next-candidate queue (Tier-A first — see migration-resume-kit.md §4)"
cat <<'EOF'
  Quick wins (TEST-MOCK deletions, ~5 min total, 26 sites once test-matcher updates):
    — defer to the end; requires audit-matcher tweak to include _test.go.
  Tier-A prod sites (~12 min each, start here):
    internal/cache/invalidation.go:TagBasedInvalidation   (Tier 3, worked example in §6 of the spec)
    internal/cache/invalidation.go:EventDrivenInvalidation
    internal/cache/expiration.go:ExpirationManager
    internal/plugins/registry.go:Registry
    internal/plugins/lifecycle.go:LifecycleManager
    internal/messaging/inmemory/queue.go:SimpleQueue
    internal/messaging/inmemory/queue.go:DelayedQueue
    internal/features/features.go:Registry
    internal/skills/registry.go:Registry
    internal/formatters/registry.go:FormatterRegistry
    internal/formatters/cache.go:FormatterCache
    internal/notifications/polling_store.go:PollingStore
    internal/streaming/state_store.go:InMemoryStateStore
    internal/streaming/types.go:MpscStream
    internal/verifier/adapters/extended_registry.go:ExtendedProviderRegistry

  To pick an arbitrary next site by coupling, run:
      ./scripts/migration/next.sh
EOF

echo
section "Resume one-liner"
cat <<'EOF'
  Next session prompt (paste into Claude):

    Resume CONST-029 Pattern-A migration per
    docs/superpowers/specs/2026-04-19-migration-resume-kit.md.
    Run scripts/migration/status.sh first. Drain the Tier-A queue
    in order, one commit per site, push to all 4 remotes after each.
    Stop when 3 consecutive sites have test-coupling blockers.
EOF
echo
