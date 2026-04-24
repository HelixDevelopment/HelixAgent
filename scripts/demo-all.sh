#!/usr/bin/env bash
# Run the acceptance demo for every project-owned module.
#
# For each CLAUDE.md under a project-owned module, extracts the first
# ```bash``` code block from the "### Acceptance demo for this module"
# section and executes it with a timeout. Reports per-module PASS / FAIL /
# TODO / NO-DEMO and exits non-zero unless every demo passes.
#
# This is the enforcement arm of the Definition of Done. Without this
# running as part of `make ci-validate-all`, the DoD is documentation
# with no teeth; with it, "done" means "every demo went green in the
# same session."
#
# Envars:
#   DEMO_TIMEOUT    per-demo timeout in seconds (default: 180)
#   DEMO_LOG_DIR    where per-module logs land (default: reports/demos)
#   DEMO_MODULES    space-separated subset to run (default: all)
#   DEMO_ALLOW_TODO pass "1" to treat TODO demos as warnings rather than failures
#                   (only for the transition period — should be unset in CI)

set -uo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

TIMEOUT="${DEMO_TIMEOUT:-180}"
LOG_DIR="${DEMO_LOG_DIR:-reports/demos}"
ALLOW_TODO="${DEMO_ALLOW_TODO:-0}"
mkdir -p "$LOG_DIR"

# Project-owned modules with CLAUDE.md files. Third-party trees (MCP/,
# cli_agents/*, external/*, mcp-servers/*) are excluded per Rule #10.
DEFAULT_MODULES=(
  Agentic Auth AutoTemp BackgroundTasks Benchmark BuildCheck Cache
  Challenges Challenges/Panoptic Claritas Concurrency Containers
  ConversationContext Database DebateOrchestrator DocProcessor Embeddings
  EventBus Formatters GandalfSolutions HelixLLM HelixMemory HelixQA
  HelixSpecifier HyperTune I-LLM LLMOps LLMOrchestrator LLMProvider
  LLMsVerifier LLMsVerifier/llm-verifier LeakHub MCP_Module Memory
  Messaging Models Normalize Observability Optimization Ouroborous Planning
  PliniusCommon Plugins RAG RedTeam Security SelfImprove SkillRegistry
  Storage Streaming ToolSchema VectorDB Veritas VisionEngine Toolkit Website
  pkg/api docker/acp docker/protocol-discovery docs/mcp-servers
)

if [ -n "${DEMO_MODULES:-}" ]; then
  # shellcheck disable=SC2206
  MODULES=($DEMO_MODULES)
else
  MODULES=("${DEFAULT_MODULES[@]}")
fi

pass=0; fail=0; todo=0; missing=0
fail_list=(); todo_list=(); missing_list=()

extract_demo() {
  # Extract the first ```bash...``` block after "### Acceptance demo for this module"
  # from the given CLAUDE.md. Emits the body of the code fence only.
  awk '
    /^### Acceptance demo for this module/ { state = 1; next }
    state == 1 && /^```bash/              { state = 2; next }
    state == 2 && /^```/                  { exit }
    state == 2                            { print }
  ' "$1"
}

is_todo() {
  # True if the extracted demo body is just a "# TODO" placeholder.
  grep -qE '^[[:space:]]*#[[:space:]]*TODO[[:space:]]*$' <<< "$1" && [ "$(printf '%s\n' "$1" | grep -cvE '^\s*$')" -le 1 ]
}

for mod in "${MODULES[@]}"; do
  md="$mod/CLAUDE.md"
  if [ ! -f "$md" ]; then
    echo "[NO-DEMO]  $mod (no CLAUDE.md at $md)"
    missing=$((missing + 1)); missing_list+=("$mod")
    continue
  fi
  demo=$(extract_demo "$md")
  if [ -z "$demo" ]; then
    echo "[NO-DEMO]  $mod (no bash block in acceptance-demo section)"
    missing=$((missing + 1)); missing_list+=("$mod")
    continue
  fi
  if is_todo "$demo"; then
    echo "[TODO]     $mod (demo still a placeholder)"
    todo=$((todo + 1)); todo_list+=("$mod")
    continue
  fi
  log="$LOG_DIR/${mod//\//_}.log"
  echo "[RUN]      $mod"
  if timeout "$TIMEOUT" bash -c "$demo" > "$log" 2>&1; then
    echo "[PASS]     $mod"
    pass=$((pass + 1))
  else
    rc=$?
    if [ "$rc" -eq 124 ]; then
      echo "[FAIL]     $mod (timeout after ${TIMEOUT}s — log: $log)"
    else
      echo "[FAIL]     $mod (exit $rc — log: $log)"
    fi
    fail=$((fail + 1)); fail_list+=("$mod")
  fi
done

echo
echo "================================================================"
echo "demo-all totals: PASS=$pass FAIL=$fail TODO=$todo NO-DEMO=$missing"
echo "================================================================"

[ "$fail" -gt 0 ]    && { echo "FAIL modules:"; printf '  - %s\n' "${fail_list[@]}"; }
[ "$todo" -gt 0 ]    && { echo "TODO modules:"; printf '  - %s\n' "${todo_list[@]}"; }
[ "$missing" -gt 0 ] && { echo "NO-DEMO modules:"; printf '  - %s\n' "${missing_list[@]}"; }

failed=0
if [ "$fail" -gt 0 ] || [ "$missing" -gt 0 ]; then
  failed=1
fi
if [ "$todo" -gt 0 ] && [ "$ALLOW_TODO" != "1" ]; then
  echo "TODO demos failing the run. Set DEMO_ALLOW_TODO=1 to treat as warnings during transition." >&2
  failed=1
fi
if [ "$failed" -eq 1 ]; then
  if [ "${DEMO_ALL_WARN_ONLY:-0}" = "1" ]; then
    echo "(warn-only mode — set DEMO_ALL_WARN_ONLY=0 to fail the build on demo failures)" >&2
    exit 0
  fi
  exit 1
fi
