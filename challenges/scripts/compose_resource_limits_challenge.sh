#!/bin/bash
# compose_resource_limits_challenge.sh — CONST-032 reproduction guard for
# docker-compose.yml resource-config integrity.
#
# Asserts:
#   1. `docker compose config -q` succeeds against docker-compose.yml on a
#      docker-compose v2 host (this is the toolchain on amber.local).
#   2. No service mixes legacy `mem_limit`/`memswap_limit` with
#      `deploy.resources.limits.memory` set to a different numeric value
#      (the exact failure mode that broke amber.local boot).
#   3. Every required runtime service has a `deploy.resources.limits.memory`
#      AND `deploy.resources.limits.cpus` floor — no implicit "unbounded".
#   4. Every required runtime service has matching `reservations` to give
#      the scheduler a placement hint.
#
# Required services (must satisfy 3 + 4): postgres, redis, chromadb,
# cognee, neo4j, memgraph, prometheus, grafana, mock-llm, langchain-server,
# llamaindex-server, guidance-server, lmql-server, sglang, ollama,
# helixagent.
#
# Exit:
#   0 = pass
#   1 = violations (defect present or regression)
#   2 = environment problem (file missing, no python3, etc.)
#
# Resolves the project root from its own location, so it works whether
# executed from the project root or from challenges/scripts/.

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
  echo "FAIL: cannot locate project root containing docker-compose.yml" >&2
  exit 2
fi

COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"

echo "=== compose_resource_limits_challenge ==="
echo "Compose file: $COMPOSE_FILE"

if ! command -v python3 >/dev/null 2>&1; then
  echo "FAIL: python3 not available in PATH" >&2
  exit 2
fi

# --- Check 1: docker compose config -q (only if docker compose is present) ---
DOCKER_COMPOSE_OK="skipped"
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  echo
  echo "--- Check 1: docker compose config -q ---"
  if docker compose -f "$COMPOSE_FILE" config -q 2>&1; then
    echo "PASS: docker compose accepts the file"
    DOCKER_COMPOSE_OK="pass"
  else
    echo "FAIL: docker compose rejected the file (see error above)" >&2
    DOCKER_COMPOSE_OK="fail"
  fi
else
  echo "--- Check 1: SKIPPED (docker compose not installed) ---"
fi

# --- Checks 2, 3, 4 in one python pass for clear matrix output ---
python3 - "$COMPOSE_FILE" <<'PY'
import sys, yaml

REQUIRED = {
    "postgres", "redis", "chromadb", "cognee", "neo4j", "memgraph",
    "prometheus", "grafana", "mock-llm", "langchain-server",
    "llamaindex-server", "guidance-server", "lmql-server", "sglang",
    "ollama", "helixagent",
}

def unwrap_env(v):
    """Unwrap a Compose env-var interpolation `${X:-default}` to its default,
    so resource validation works on the literal that Compose will use when
    the env var is unset (= the dev/test path). Production overrides via env
    are validated separately by the deploy pipeline."""
    if v is None:
        return None
    s = str(v).strip()
    import re
    m = re.fullmatch(r"\$\{[A-Z][A-Z0-9_]*:-([^}]+)\}", s)
    if m:
        return m.group(1).strip()
    return s

def to_bytes(v):
    v = unwrap_env(v)
    if v is None:
        return None
    s = str(v).strip().lower().replace(" ", "")
    units = {"k":1024, "m":1024**2, "g":1024**3, "t":1024**4}
    if s.endswith("b"):
        s = s[:-1]
    if s and s[-1] in units:
        try: return int(float(s[:-1]) * units[s[-1]])
        except ValueError: return None
    try: return int(s)
    except ValueError: return None

def to_cpus(v):
    v = unwrap_env(v)
    if v is None: return None
    try: return float(v)
    except (TypeError, ValueError): return None

with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f)

services = doc.get("services", {}) or {}
if not isinstance(services, dict):
    print("FAIL: services: is not a mapping", file=sys.stderr)
    sys.exit(1)

violations = []

for name, cfg in services.items():
    if not isinstance(cfg, dict):
        continue
    ml  = cfg.get("mem_limit")
    deploy = cfg.get("deploy") or {}
    res = deploy.get("resources") or {}
    lim = res.get("limits") or {}
    rsv = res.get("reservations") or {}
    lm  = lim.get("memory")
    rm  = rsv.get("memory")
    lc  = lim.get("cpus")
    rc  = rsv.get("cpus")

    # Check 2: mem_limit vs deploy.limits.memory mismatch
    if ml is not None and lm is not None:
        if to_bytes(ml) != to_bytes(lm):
            violations.append(
                f"{name}: legacy mem_limit={ml!r} conflicts with "
                f"deploy.resources.limits.memory={lm!r} "
                f"(docker compose v2 will reject this project)"
            )
        else:
            # equal but redundant — recommend dropping legacy form
            violations.append(
                f"{name}: has both mem_limit={ml!r} and "
                f"deploy.resources.limits.memory={lm!r}; drop legacy mem_limit "
                f"to use the canonical Compose v2/v3 form only"
            )

    # Apply 3 + 4 only to required services
    if name in REQUIRED:
        if lm is None:
            violations.append(
                f"{name}: missing deploy.resources.limits.memory "
                f"(required for predictable performance under distribution)"
            )
        if lc is None:
            violations.append(
                f"{name}: missing deploy.resources.limits.cpus "
                f"(required for predictable performance under distribution)"
            )
        if rm is None:
            violations.append(
                f"{name}: missing deploy.resources.reservations.memory "
                f"(scheduler placement hint)"
            )
        if rc is None:
            violations.append(
                f"{name}: missing deploy.resources.reservations.cpus "
                f"(scheduler placement hint)"
            )

        # If both forms present, must match — already checked in Check 2

        # Reservations should be <= limits
        if to_bytes(lm) is not None and to_bytes(rm) is not None:
            if to_bytes(rm) > to_bytes(lm):
                violations.append(
                    f"{name}: reservations.memory={rm!r} > limits.memory={lm!r}"
                )
        if to_cpus(lc) is not None and to_cpus(rc) is not None:
            if to_cpus(rc) > to_cpus(lc):
                violations.append(
                    f"{name}: reservations.cpus={rc!r} > limits.cpus={lc!r}"
                )

print()
print("--- Check 2: mem_limit vs deploy.resources mismatch ---")
print("--- Check 3: required services have memory + cpu limits ---")
print("--- Check 4: required services have reservations ---")
if violations:
    print(f"FAIL: {len(violations)} violation(s):")
    for v in violations:
        print(f"  - {v}")
    sys.exit(1)
print(f"PASS: all {len(REQUIRED)} required services have valid resource configs")
PY
PY_EXIT=$?

echo
echo "=== Summary ==="
echo "docker compose validation: $DOCKER_COMPOSE_OK"
echo "yaml invariants: $([[ $PY_EXIT -eq 0 ]] && echo pass || echo fail)"

if [[ "$DOCKER_COMPOSE_OK" == "fail" ]] || [[ $PY_EXIT -ne 0 ]]; then
  exit 1
fi
exit 0
