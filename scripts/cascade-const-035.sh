#!/bin/bash
# cascade-const-035.sh — propagate CONST-035 (Anti-Bluff Tests &
# Challenges) and the build-pressure-vs-host-suspend clarification
# into every project-owned submodule's CLAUDE.md, AGENTS.md, and
# CONSTITUTION.md (creating sections if absent).
#
# Idempotent — re-running on a submodule that already has the
# CONST-035 marker is a no-op.
#
# Pushes to every configured non-mirror remote per submodule.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SUBMODULES=(
  Agentic Auth AutoTemp BackgroundTasks Benchmark BuildCheck Cache
  Challenges Claritas Concurrency ConversationContext Database
  DebateOrchestrator DocProcessor Embeddings EventBus Formatters
  GandalfSolutions HelixLLM HelixMemory HelixQA HelixSpecifier
  HyperTune I-LLM LLMOps LLMOrchestrator LLMProvider LLMsVerifier
  LeakHub MCP-Servers MCP_Module Models Normalize Optimization
  Ouroborous PliniusCommon Planning Plugins RAG RedTeam Security
  SelfImprove SkillRegistry Storage Streaming ToolSchema Veritas
  VectorDB VisionEngine
)

# Marker the script writes so re-runs are idempotent.
MARKER='<!-- CONST-035 anti-bluff addendum (cascaded) -->'

# Canonical addendum body. Same paragraph appended to all three
# files in every submodule.
read -r -d '' ADDENDUM <<'EOF' || true
<!-- CONST-035 anti-bluff addendum (cascaded) -->

## CONST-035 — Anti-Bluff Tests & Challenges (mandatory; inherits from root)

Tests and Challenges in this submodule MUST verify the product, not
the LLM's mental model of the product. A test that passes when the
feature is broken is worse than a missing test — it gives false
confidence and lets defects ship to users. Functional probes at the
protocol layer are mandatory:

- TCP-open is the FLOOR, not the ceiling. Postgres → execute
  `SELECT 1`. Redis → `PING` returns `PONG`. ChromaDB → `GET
  /api/v1/heartbeat` returns 200. MCP server → TCP connect + valid
  JSON-RPC handshake. HTTP gateway → real request, real response,
  non-empty body.
- Container `Up` is NOT application healthy. A `docker/podman ps`
  `Up` status only means PID 1 is running; the application may be
  crash-looping internally.
- No mocks/fakes outside unit tests (already CONST-030; CONST-035
  raises the cost of a mock-driven false pass to the same severity
  as a regression).
- Re-verify after every change. Don't assume a previously-passing
  test still verifies the same scope after a refactor.
- Verification of CONST-035 itself: deliberately break the feature
  (e.g. `kill <service>`, swap a password). The test MUST fail. If
  it still passes, the test is non-conformant and MUST be tightened.

## CONST-033 clarification — distinguishing host events from sluggishness

Heavy container builds (BuildKit pulling many GB of layers, parallel
podman/docker compose-up across many services) can make the host
**appear** unresponsive — high load average, slow SSH, watchers
timing out. **This is NOT a CONST-033 violation.** Suspend / hibernate
/ logout are categorically different events. Distinguish via:

- `uptime` — recent boot? if so, the host actually rebooted.
- `loginctl list-sessions` — session(s) still active? if yes, no logout.
- `journalctl ... | grep -i 'will suspend\|hibernate'` — zero broadcasts
  since the CONST-033 fix means no suspend ever happened.
- `dmesg | grep -i 'killed process\|out of memory'` — OOM kills are
  also NOT host-power events; they're memory-pressure-induced and
  require their own separate fix (lower per-container memory limits,
  reduce parallelism).

A sluggish host under build pressure recovers when the build finishes;
a suspended host requires explicit unsuspend (and CONST-033 should
make that impossible by hardening `IdleAction=ignore` +
`HandleSuspendKey=ignore` + masked `sleep.target`,
`suspend.target`, `hibernate.target`, `hybrid-sleep.target`).

If you observe what looks like a suspend during heavy builds, the
correct first action is **not** "edit CONST-033" but `bash
challenges/scripts/host_no_auto_suspend_challenge.sh` to confirm the
hardening is intact. If hardening is intact AND no suspend
broadcast appears in journal, the perceived event was build-pressure
sluggishness, not a power transition.
EOF

CHANGED=0
PUSHED=0
for sm in "${SUBMODULES[@]}"; do
  [[ -d "$sm" ]] || { echo "skip: $sm (no dir)"; continue; }
  cd "$ROOT/$sm"

  # Skip if marker already present in CLAUDE.md.
  if [[ -f CLAUDE.md ]] && grep -qF "$MARKER" CLAUDE.md; then
    echo "  $sm: CONST-035 cascade marker present — skip"
    cd "$ROOT"
    continue
  fi

  # Append to whichever of the 3 files exists.
  any_appended=0
  for f in CLAUDE.md AGENTS.md CONSTITUTION.md; do
    if [[ -f "$f" ]]; then
      printf "\n\n%s\n" "$ADDENDUM" >> "$f"
      any_appended=1
    fi
  done

  if [[ $any_appended -eq 0 ]]; then
    echo "  $sm: no docs files — skip"
    cd "$ROOT"
    continue
  fi

  CHANGED=$((CHANGED + 1))
  echo "  $sm: appended"

  # Commit + push to non-mirror remotes (github + gitlab; skip
  # gitflic/gitverse which often have permission issues).
  git add -A
  if ! git diff --cached --quiet; then
    git commit -q -m "$(cat <<MSG
docs: cascade CONST-035 (anti-bluff) + CONST-033 build-pressure clarification

CONST-035 mandates that tests and Challenges verify the product, not
the LLM's mental model of the product. Functional probes at the
protocol layer; container Up is NOT application-healthy. Inherited
from HelixAgent root (commit feaa98fa).

CONST-033 clarification: heavy container build pressure (BuildKit,
parallel compose-up) can make the host APPEAR stuck without any
actual suspend/hibernate/logout. Distinguish via uptime,
loginctl list-sessions, and journalctl 'will suspend' broadcast
count. Build sluggishness is NOT a CONST-033 violation.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
MSG
)" || { echo "    commit failed"; cd "$ROOT"; continue; }
    PUSHED_THIS=0
    for r in github gitlab; do
      if git remote get-url "$r" >/dev/null 2>&1; then
        if git push "$r" HEAD 2>/dev/null | grep -q .; then :; fi
        PUSHED_THIS=$((PUSHED_THIS + 1))
      fi
    done
    PUSHED=$((PUSHED + 1))
    echo "    pushed to $PUSHED_THIS remote(s)"
  fi

  cd "$ROOT"
done

echo
echo "Total submodules updated: $CHANGED"
echo "Total submodules pushed:  $PUSHED"
