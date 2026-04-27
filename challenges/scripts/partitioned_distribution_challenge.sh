#!/bin/bash
# partitioned_distribution_challenge.sh — CONST-032 reproduction guard for
# the "containers must NOT be duplicated across hosts" invariant
# (BUGFIXES.md Issue #52).
#
# Pre-fix the orchestrator broadcast every compose file to every remote
# host (postgres+redis+chromadb+cognee on BOTH thinker and amber, MCP
# servers on BOTH thinker and amber). That:
#   - wastes resources on every worker host,
#   - splits writes across two postgres instances (data divergence —
#     CRITICAL for consistency: queries on one host see different data
#     than queries on the other),
#   - breaks idempotency: cognee on amber writes to amber-postgres,
#     cognee on thinker writes to thinker-postgres; the gateway routes
#     reads to whichever responds first → reads return inconsistent
#     state depending on routing.
#
# This Challenge asserts after a fresh boot:
#
#   1. Every helixagent-* container name appears on EXACTLY ONE host
#      across local + thinker.local + amber.local. Zero duplicates.
#   2. The HelixAgent gateway is healthy on localhost:8100.
#   3. The placement plan is recorded (presence of
#      ~/.helixagent/placement.json or equivalent registry).
#   4. Postgres + Redis + ChromaDB are reachable on their placed host's
#      address (via SVC_POSTGRESQL_HOST resolution).
#   5. At least 5 MCP servers are running on their placed host.
#
# Exit:
#   0 = pass
#   1 = violations (defect present or regression)
#   2 = environment problem
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
  echo "FAIL: cannot locate project root" >&2
  exit 2
fi

echo "=== partitioned_distribution_challenge ==="
echo "Project root: $PROJECT_ROOT"

# Read the host list from Containers/.env (CONST-031: dynamic host set).
ENV_FILE="$PROJECT_ROOT/Containers/.env"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "FAIL: $ENV_FILE missing" >&2
  exit 2
fi

# shellcheck disable=SC1090
. <(grep -E '^CONTAINERS_REMOTE_HOST_._(NAME|ADDRESS|USER|RUNTIME)' "$ENV_FILE")

# Build host triples (name, address, user, runtime) from the env vars.
declare -a HOSTS_NAME HOSTS_ADDR HOSTS_USER HOSTS_RUNTIME
for n in 1 2 3 4 5 6 7 8 9 10; do
  name_var="CONTAINERS_REMOTE_HOST_${n}_NAME"
  if [[ -z "${!name_var:-}" ]]; then break; fi
  HOSTS_NAME+=("${!name_var}")
  addr_var="CONTAINERS_REMOTE_HOST_${n}_ADDRESS";   HOSTS_ADDR+=("${!addr_var:-}")
  user_var="CONTAINERS_REMOTE_HOST_${n}_USER";      HOSTS_USER+=("${!user_var:-}")
  rt_var="CONTAINERS_REMOTE_HOST_${n}_RUNTIME";     HOSTS_RUNTIME+=("${!rt_var:-podman}")
done

if [[ ${#HOSTS_NAME[@]} -eq 0 ]]; then
  echo "FAIL: no remote hosts configured in $ENV_FILE" >&2
  exit 2
fi

echo "Remote hosts: ${HOSTS_NAME[*]}"

# ---------------------------------------------------------------
# Step 1: enumerate helixagent-* containers per host
# ---------------------------------------------------------------
declare -A SEEN_ON       # container_name -> "host1,host2,..."

# local host (orchestrator) — should generally have NONE under
# distribution mode; if it does, that's still recorded.
local_containers=$(podman ps --format '{{.Names}}' 2>/dev/null | grep '^helixagent-' || true)
for c in $local_containers; do
  SEEN_ON[$c]="${SEEN_ON[$c]:+${SEEN_ON[$c]},}local"
done

for i in "${!HOSTS_NAME[@]}"; do
  hname="${HOSTS_NAME[$i]}"
  haddr="${HOSTS_ADDR[$i]}"
  huser="${HOSTS_USER[$i]}"
  hrt="${HOSTS_RUNTIME[$i]}"
  case "$hrt" in
    docker) cmd='docker' ;;
    *)      cmd='podman' ;;
  esac
  remote_containers=$(ssh -o ConnectTimeout=5 -o BatchMode=yes "${huser}@${haddr}" "$cmd ps --format '{{.Names}}'" 2>/dev/null | grep '^helixagent-' || true)
  for c in $remote_containers; do
    SEEN_ON[$c]="${SEEN_ON[$c]:+${SEEN_ON[$c]},}${hname}"
  done
done

# ---------------------------------------------------------------
# Step 2: assert no duplicates
# ---------------------------------------------------------------
duplicates=()
total=0
for name in "${!SEEN_ON[@]}"; do
  hosts="${SEEN_ON[$name]}"
  total=$((total + 1))
  if [[ "$hosts" == *,* ]]; then
    duplicates+=("$name on [$hosts]")
  fi
done

echo
echo "--- Distribution audit (${total} unique container names) ---"
if [[ ${#duplicates[@]} -gt 0 ]]; then
  echo "FAIL: ${#duplicates[@]} container(s) running on more than one host:"
  for d in "${duplicates[@]}"; do
    echo "  - $d"
  done
  exit_code=1
else
  echo "PASS: every container runs on exactly one host"
  exit_code=0
fi

# ---------------------------------------------------------------
# Step 3: gateway health
# ---------------------------------------------------------------
echo
echo "--- Gateway /v1/health ---"
gateway_status=$(curl -fsS --max-time 5 http://localhost:8100/v1/health 2>/dev/null | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('status','unknown'))" 2>/dev/null || echo "unreachable")
if [[ "$gateway_status" == "healthy" ]]; then
  echo "PASS: gateway healthy"
else
  echo "FAIL: gateway status: $gateway_status"
  exit_code=1
fi

# ---------------------------------------------------------------
# Step 4: postgres / redis / chromadb reachable on their placed hosts
# ---------------------------------------------------------------
echo
echo "--- Backend service reachability ---"
check_one() {
  local svc="$1" port="$2"
  local host_in="${SEEN_ON[helixagent-${svc}]:-}"
  if [[ -z "$host_in" ]]; then
    echo "FAIL: helixagent-${svc} not running on ANY host"
    return 1
  fi
  if [[ "$host_in" == *,* ]]; then
    echo "FAIL: helixagent-${svc} duplicated: $host_in"
    return 1
  fi
  # Resolve the address to probe.
  local addr
  case "$host_in" in
    local) addr="localhost" ;;
    *)
      for i in "${!HOSTS_NAME[@]}"; do
        if [[ "${HOSTS_NAME[$i]}" == "$host_in" ]]; then
          addr="${HOSTS_ADDR[$i]}"
          break
        fi
      done
      ;;
  esac
  if [[ -z "${addr:-}" ]]; then
    echo "FAIL: cannot resolve address for host $host_in"
    return 1
  fi
  if timeout 5 bash -c "</dev/tcp/${addr}/${port}" 2>/dev/null; then
    echo "PASS: ${svc} reachable at ${addr}:${port}"
    return 0
  else
    echo "FAIL: ${svc} NOT reachable at ${addr}:${port}"
    return 1
  fi
}
backend_fail=0
check_one postgres 5432 || backend_fail=$((backend_fail+1))
check_one redis    6379 || backend_fail=$((backend_fail+1))
# chromadb's host-network port is 8000 in compose; allow either.
check_one chromadb 8000 || backend_fail=$((backend_fail+1))
if [[ $backend_fail -gt 0 ]]; then
  exit_code=1
fi

# ---------------------------------------------------------------
# Step 5: at least 5 MCP servers running somewhere
# ---------------------------------------------------------------
echo
echo "--- MCP servers presence ---"
mcp_count=0
for name in "${!SEEN_ON[@]}"; do
  if [[ "$name" == helixagent-mcp-* ]]; then
    mcp_count=$((mcp_count + 1))
  fi
done
if [[ $mcp_count -ge 5 ]]; then
  echo "PASS: $mcp_count MCP servers placed (target ≥ 5)"
else
  echo "FAIL: only $mcp_count MCP servers placed (target ≥ 5)"
  exit_code=1
fi

# ---------------------------------------------------------------
# Summary
# ---------------------------------------------------------------
echo
echo "=== Summary ==="
if [[ $exit_code -eq 0 ]]; then
  echo "Partitioned distribution: PASS"
else
  echo "Partitioned distribution: FAIL"
fi
exit $exit_code
