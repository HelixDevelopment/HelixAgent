#!/bin/bash
# mcp_servers_distribution_challenge.sh — CONST-032 reproduction guard for
# the MCP build-context distribution bug (BUGFIXES.md Issue #51).
#
# When `docker/mcp/docker-compose.mcp-servers.yml` uses `context: ../..`
# (project root) for every service, the orchestrator's main adapter
# (`internal/adapters/containers/adapter.go`) SKIPS those contexts (it
# refuses to ship the 27 GB project root to remote workers). The compose
# file is copied alone to the remote host, where its `dockerfile:
# docker/mcp/Dockerfile.mcp-server` reference fails to resolve because
# the project layout isn't reproduced — this is what broke MCP servers
# on amber.local + thinker.local during the 2026-04-27 boot.
#
# This Challenge asserts (statically, no docker required):
#
#   1. Every build context referenced by every service in
#      docker/mcp/docker-compose.mcp-servers.yml resolves to a path
#      INSIDE the compose file's parent directory tree, OR is a sibling
#      submodule that the orchestrator can ship as a focused build
#      context (NOT the whole project root).
#   2. Every dockerfile referenced (relative to its context) exists on
#      disk today.
#   3. No service uses `context: ../..` or any path that escapes to the
#      project root (which the adapter explicitly skips).
#
# Exit:
#   0 = pass (the orchestrator can ship every MCP build context)
#   1 = violations (defect present or regression)
#   2 = environment problem (file missing, no python3, no yaml lib, etc.)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

find_project_root() {
  local d="$1"
  while [[ "$d" != "/" ]]; do
    if [[ -f "$d/docker-compose.yml" && -d "$d/challenges/scripts" ]]; then
      echo "$d"; return 0
    fi
    d=$(dirname "$d")
  done
  return 1
}

PROJECT_ROOT=$(find_project_root "$SCRIPT_DIR" || true)
if [[ -z "${PROJECT_ROOT:-}" ]]; then
  echo "FAIL: cannot locate project root" >&2
  exit 2
fi

MCP_COMPOSE="$PROJECT_ROOT/docker/mcp/docker-compose.mcp-servers.yml"
echo "=== mcp_servers_distribution_challenge ==="
echo "Compose file: $MCP_COMPOSE"

if [[ ! -f "$MCP_COMPOSE" ]]; then
  echo "FAIL: $MCP_COMPOSE not found" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "FAIL: python3 not available" >&2
  exit 2
fi

python3 - "$PROJECT_ROOT" "$MCP_COMPOSE" <<'PY'
import os, sys, yaml

PROJECT_ROOT = os.path.abspath(sys.argv[1])
COMPOSE = os.path.abspath(sys.argv[2])
COMPOSE_DIR = os.path.dirname(COMPOSE)

with open(COMPOSE) as f:
    doc = yaml.safe_load(f)

services = doc.get("services") or {}
if not services:
    print("FAIL: no services in compose file", file=sys.stderr)
    sys.exit(1)

violations = []

# A "shippable" context is any path that's:
#   - INSIDE COMPOSE_DIR (the orchestrator already copies COMPOSE_DIR), OR
#   - A SIBLING DIRECTORY at the project root (the orchestrator can copy
#     it as a focused additional context — small enough to ship)
# NOT shippable: project root itself, parent of project root, anywhere
# else.
def classify_context(ctx_abs):
    if ctx_abs == PROJECT_ROOT:
        return "project_root"
    if ctx_abs == COMPOSE_DIR:
        return "compose_dir"
    if ctx_abs.startswith(COMPOSE_DIR + os.sep):
        return "inside_compose_dir"
    if ctx_abs.startswith(PROJECT_ROOT + os.sep):
        # It's a sub-path of project root, not project root itself.
        # Verify it's a "focused" sub-path (not requiring 27GB ship).
        rel = os.path.relpath(ctx_abs, PROJECT_ROOT)
        first = rel.split(os.sep)[0]
        # Heuristic: top-level dirs that are heavy and shouldn't be
        # shipped as a build context (they're ~GB-sized each).
        forbidden_top = {"vendor", "releases", "cli_agents", "node_modules"}
        if first in forbidden_top:
            return "forbidden_top"
        return "shippable_sub"
    return "outside_project"

CTX_OK = {"compose_dir", "inside_compose_dir", "shippable_sub"}

for name, cfg in services.items():
    if not isinstance(cfg, dict):
        continue
    build = cfg.get("build")
    if not build:
        continue
    if isinstance(build, str):
        build = {"context": build}

    ctx = build.get("context", ".")
    df  = build.get("dockerfile", "Dockerfile")

    ctx_abs = os.path.abspath(os.path.join(COMPOSE_DIR, ctx))
    klass = classify_context(ctx_abs)

    if klass == "project_root":
        violations.append(
            f"{name}: build.context={ctx!r} resolves to project root "
            f"({PROJECT_ROOT}); the orchestrator SKIPS project-root contexts "
            f"to avoid shipping 27 GB. Use a focused sub-context (e.g. "
            f"`../../MCP-Servers` or `../../MCP/submodules/<name>`) and "
            f"update the Dockerfile to use `COPY . .`."
        )
        continue
    if klass == "outside_project":
        violations.append(
            f"{name}: build.context={ctx!r} resolves OUTSIDE the project "
            f"({ctx_abs}); the orchestrator cannot ship paths outside "
            f"the project root."
        )
        continue
    if klass == "forbidden_top":
        violations.append(
            f"{name}: build.context={ctx!r} points at a heavy top-level "
            f"directory ({ctx_abs}) that should never be shipped as a "
            f"build context."
        )
        continue
    if klass not in CTX_OK:
        violations.append(f"{name}: unexpected context classification: {klass}")
        continue

    # Verify the Dockerfile exists at the resolved path. Compose v2
    # interprets `dockerfile:` as relative to the context.
    df_abs = os.path.abspath(os.path.join(ctx_abs, df))
    if not os.path.isfile(df_abs):
        violations.append(
            f"{name}: dockerfile {df!r} (resolved to {df_abs}) "
            f"does not exist on disk"
        )

if violations:
    print(f"FAIL: {len(violations)} violation(s):")
    for v in violations:
        print(f"  - {v}")
    sys.exit(1)

print(f"PASS: every build context in MCP servers compose is shippable + "
      f"every dockerfile resolves ({len(services)} services checked)")
PY
exit_code=$?

echo
echo "=== Summary ==="
if [[ $exit_code -eq 0 ]]; then
  echo "MCP servers distribution: PASS"
  exit 0
fi
echo "MCP servers distribution: FAIL"
exit 1
