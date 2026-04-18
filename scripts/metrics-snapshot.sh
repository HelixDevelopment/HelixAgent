#!/usr/bin/env bash
# scripts/metrics-snapshot.sh — capture a baseline metrics snapshot
#
# Snapshots:
#   - go test count per module
#   - challenge script count
#   - Prometheus /v1/monitoring/status snapshot (if HelixAgent is running)
#   - Phase-3 SLI gauges (if HELIX_MONITOR_URL is set)
#   - SHA of main repo + each own-module
#
# Intended to be invoked at phase boundaries to build a comparable history
# under reports/metrics-snapshots/<timestamp>/.
#
# Contract: CONST-019 non-interactive, CONST-022 resource capped,
# fully read-only.
#
# Usage:
#   make metrics-snapshot
#   ./scripts/metrics-snapshot.sh /path/to/output-dir
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-$ROOT/reports/metrics-snapshots/$(date -u +%Y-%m-%dT%H%M%SZ)}"
mkdir -p "$OUT"

BLUE=$'\033[0;34m'
GREEN=$'\033[0;32m'
NC=$'\033[0m'

export GOMAXPROCS=${GOMAXPROCS:-2}

echo -e "${BLUE}==>${NC} Snapshotting to $OUT"

# 1. Test counts per package (under internal/, cmd/, pkg/, tests/)
echo -e "${BLUE}==>${NC} Test inventory"
{
    echo "# Test inventory — $(date -u +%FT%TZ)"
    echo ""
    echo "## Go test files"
    find internal cmd pkg tests -name "*_test.go" -type f 2>/dev/null | wc -l | xargs -I{} echo "Total: {}"
    echo ""
    echo "## Test files by suffix"
    for suffix in _test.go _integration_test.go _e2e_test.go _security_test.go _stress_test.go _bench_test.go _benchmark_test.go _fuzz_test.go _race_test.go _chaos_test.go _load_test.go; do
        count=$(find internal cmd pkg tests -name "*${suffix}" -type f 2>/dev/null | wc -l)
        printf "%-32s %d\n" "$suffix" "$count"
    done
} > "$OUT/test-inventory.md"

# 2. Challenge scripts
echo -e "${BLUE}==>${NC} Challenge inventory"
{
    echo "# Challenge inventory — $(date -u +%FT%TZ)"
    echo ""
    find challenges/scripts -name "*.sh" -type f 2>/dev/null | wc -l | xargs -I{} echo "Total shell challenges: {}"
    echo ""
    echo "## Scripts"
    find challenges/scripts -name "*.sh" -type f 2>/dev/null | sort
} > "$OUT/challenge-inventory.md"

# 3. Main repo + submodule SHAs
echo -e "${BLUE}==>${NC} Submodule SHA pinboard"
{
    echo "# Submodule SHAs — $(date -u +%FT%TZ)"
    echo ""
    echo "## Main repo"
    printf "%s %s\n" "$(git rev-parse HEAD)" "$(git symbolic-ref --short HEAD)"
    echo ""
    echo "## Submodules"
    # Recursive traversal aborts on known-orphan third-party sub-submodules
    # (e.g. cli_agents/bridle/plugins/skill-enhancers/axiom). Fall back to
    # non-recursive when recursive fails.
    git submodule status --recursive 2>/dev/null | awk '{print $1, $2}' | sort -k2 \
      || git submodule status 2>/dev/null | awk '{print $1, $2}' | sort -k2 \
      || echo "(submodule status collection failed — see orphan note)"
} > "$OUT/submodule-shas.md" 2>/dev/null || true

# 4. Line counts per module (rough)
echo -e "${BLUE}==>${NC} Module LOC"
{
    echo "# Module line counts (approx., Go files only) — $(date -u +%FT%TZ)"
    echo ""
    for mod in Agentic Auth BackgroundTasks Benchmark BuildCheck Cache Challenges Concurrency Containers ConversationContext Database DebateOrchestrator DocProcessor Embeddings EventBus Formatters HelixLLM HelixMemory HelixQA HelixSpecifier LLMOps LLMOrchestrator LLMProvider LLMsVerifier Memory MCP_Module Messaging Models Observability Optimization Planning Plugins RAG Security SelfImprove SkillRegistry Storage Streaming ToolSchema VectorDB VisionEngine; do
        if [[ -d "$mod" ]]; then
            loc=$(find "$mod" -name "*.go" -type f -not -path "*/vendor/*" 2>/dev/null | xargs -r wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo 0)
            printf "%-24s %8d\n" "$mod" "${loc:-0}"
        fi
    done
} > "$OUT/module-loc.md"

# 5. Monitoring URL probe (best-effort)
if [[ -n "${HELIX_MONITOR_URL:-}" ]]; then
    echo -e "${BLUE}==>${NC} Scraping $HELIX_MONITOR_URL"
    timeout 10 curl -fsS "$HELIX_MONITOR_URL/metrics" > "$OUT/prometheus-scrape.txt" 2>/dev/null \
      || echo "(unreachable)" > "$OUT/prometheus-scrape.txt"
fi
if [[ -n "${HELIX_AGENT_URL:-}" ]]; then
    timeout 10 curl -fsS "$HELIX_AGENT_URL/v1/monitoring/status" > "$OUT/monitoring-status.json" 2>/dev/null \
      || echo '{"note":"unreachable"}' > "$OUT/monitoring-status.json"
fi

echo -e "${GREEN}✓${NC} snapshot written: $OUT"
