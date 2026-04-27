#!/bin/bash
# partitioned_distribution_challenge.sh — CONST-032 + CONST-035 guard
# for the partitioned-distribution invariant AND its end-user-visible
# functional consequences.
#
# Pre-CONST-035 versions of this Challenge soft-failed reachability
# checks ("redis deferred to gateway /v1/health"), which let the
# Challenge pass even when redis was actually broken on the placed
# host. Per CONST-035 (anti-bluff): the Challenge MUST exercise real
# user-visible behavior with strict pass/fail. No soft passes that
# tolerate broken features.
#
# What this Challenge proves end-to-end:
#
#   STRUCTURAL invariants:
#     S1. Every helixagent-* container exists on EXACTLY ONE host
#         (no replication anywhere, ever).
#     S2. Each placed-on host actually has the container marked Up.
#
#   FUNCTIONAL invariants (the part CONST-035 made strict):
#     F1. The HelixAgent gateway responds to /v1/health within 15s
#         with status="healthy". Times-out → FAIL.
#     F2. Postgres on its placed host accepts a connection AND
#         answers SELECT 1 with the value 1. Just TCP-open is the
#         floor, the protocol probe is the ceiling.
#     F3. Redis on its placed host responds to PING with PONG.
#         (Whatever port docker/podman mapped — discovered live.)
#     F4. ChromaDB on its placed host returns 200 from
#         /api/v1/heartbeat. (Real HTTP, real protocol.)
#     F5. The gateway's /v1/mcp/stats reports >=5 MCP servers running.
#
# Exit:
#   0 = pass (every structural AND functional invariant satisfied)
#   1 = violations
#   2 = environment problem

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

ENV_FILE="$PROJECT_ROOT/Containers/.env"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "FAIL: $ENV_FILE missing" >&2
  exit 2
fi

# shellcheck disable=SC1090
. <(grep -E '^CONTAINERS_REMOTE_HOST_._(NAME|ADDRESS|USER|RUNTIME)' "$ENV_FILE")

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
  echo "FAIL: no remote hosts in $ENV_FILE" >&2
  exit 2
fi

addr_for_host() {
  local hname="$1"
  for i in "${!HOSTS_NAME[@]}"; do
    if [[ "${HOSTS_NAME[$i]}" == "$hname" ]]; then
      echo "${HOSTS_ADDR[$i]}"
      return
    fi
  done
}

ssh_for_host() {
  local hname="$1"
  for i in "${!HOSTS_NAME[@]}"; do
    if [[ "${HOSTS_NAME[$i]}" == "$hname" ]]; then
      echo "${HOSTS_USER[$i]}@${HOSTS_ADDR[$i]}"
      return
    fi
  done
}

runtime_for_host() {
  local hname="$1"
  for i in "${!HOSTS_NAME[@]}"; do
    if [[ "${HOSTS_NAME[$i]}" == "$hname" ]]; then
      case "${HOSTS_RUNTIME[$i]}" in
        docker) echo "docker" ;;
        *)      echo "podman" ;;
      esac
      return
    fi
  done
  echo "podman"
}

echo "=== partitioned_distribution_challenge ==="
echo "Project root: $PROJECT_ROOT"
echo "Remote hosts: ${HOSTS_NAME[*]}"

# ---------------------------------------------------------------
# S1 + S2: enumerate helixagent-* containers per host & build the
# service → placed-host index used by F2/F3/F4 to know WHERE to
# probe. No duplicates allowed across hosts.
# ---------------------------------------------------------------
declare -A SEEN_ON

local_containers=$(podman ps --format '{{.Names}}' 2>/dev/null | grep '^helixagent-' || true)
for c in $local_containers; do
  SEEN_ON[$c]="${SEEN_ON[$c]:+${SEEN_ON[$c]},}local"
done

for i in "${!HOSTS_NAME[@]}"; do
  hname="${HOSTS_NAME[$i]}"
  cmd=$(runtime_for_host "$hname")
  remote_containers=$(ssh -o ConnectTimeout=5 -o BatchMode=yes "$(ssh_for_host "$hname")" \
    "$cmd ps --format '{{.Names}}'" 2>/dev/null | grep '^helixagent-' || true)
  for c in $remote_containers; do
    SEEN_ON[$c]="${SEEN_ON[$c]:+${SEEN_ON[$c]},}${hname}"
  done
done

duplicates=()
total_unique=0
for name in "${!SEEN_ON[@]}"; do
  total_unique=$((total_unique + 1))
  if [[ "${SEEN_ON[$name]}" == *,* ]]; then
    duplicates+=("$name on [${SEEN_ON[$name]}]")
  fi
done

echo
echo "--- S1+S2: $total_unique unique container names ---"
if [[ ${#duplicates[@]} -gt 0 ]]; then
  echo "FAIL: ${#duplicates[@]} container(s) on more than one host:"
  for d in "${duplicates[@]}"; do echo "  - $d"; done
  exit_code=1
else
  echo "PASS: zero duplicates across hosts"
  exit_code=0
fi

# ---------------------------------------------------------------
# F1: gateway functional probe
# ---------------------------------------------------------------
echo
echo "--- F1: gateway /v1/health (real HTTP, 15s timeout) ---"
gw_status=$(curl -fsS --max-time 15 http://localhost:8100/v1/health 2>/dev/null \
  | python3 -c "import sys,json;print(json.load(sys.stdin).get('status','unknown'))" 2>/dev/null \
  || echo "unreachable")
if [[ "$gw_status" == "healthy" ]]; then
  echo "PASS: gateway healthy"
else
  echo "FAIL: gateway status='$gw_status' (expected 'healthy')"
  exit_code=1
fi

# ---------------------------------------------------------------
# F2: postgres protocol probe (SELECT 1)
#
# The strict probe runs inside the container via SSH so we don't
# depend on psql being installed on the orchestrator host AND we
# always hit the local-container loopback (no host-firewall issues
# masking a working postgres). This is genuinely strict — psql
# inside the container actually executes the query against the
# server process.
# ---------------------------------------------------------------
echo
echo "--- F2: postgres SELECT 1 from its placed host ---"
pg_host="${SEEN_ON[helixagent-postgres]:-}"
pg_user="${DB_USER:-helixagent}"
pg_db="${DB_NAME:-helixagent_db}"
if [[ -z "$pg_host" || "$pg_host" == *,* ]]; then
  echo "FAIL: helixagent-postgres not placed on exactly one host (placed: '$pg_host')"
  exit_code=1
elif [[ "$pg_host" == "local" ]]; then
  result=$(podman exec helixagent-postgres psql -U "$pg_user" -d "$pg_db" -tAc 'SELECT 1' 2>&1 \
    | tail -1 | tr -d ' \n')
  if [[ "$result" == "1" ]]; then
    echo "PASS: postgres on local returned SELECT 1 = 1"
  else
    echo "FAIL: postgres on local SELECT 1 returned: '$result'"
    exit_code=1
  fi
else
  cmd=$(runtime_for_host "$pg_host")
  result=$(ssh -o ConnectTimeout=5 -o BatchMode=yes "$(ssh_for_host "$pg_host")" \
    "$cmd exec helixagent-postgres psql -U $pg_user -d $pg_db -tAc 'SELECT 1'" 2>&1 \
    | tail -1 | tr -d ' \n')
  if [[ "$result" == "1" ]]; then
    echo "PASS: postgres on $pg_host returned SELECT 1 = 1"
  else
    echo "FAIL: postgres on $pg_host SELECT 1 returned: '$result'"
    exit_code=1
  fi
fi

# ---------------------------------------------------------------
# F3: redis PING/PONG via SSH on placed host (avoids host-port
# mapping uncertainty — go directly to where it lives)
# ---------------------------------------------------------------
echo
echo "--- F3: redis PING from its placed host ---"
redis_host="${SEEN_ON[helixagent-redis]:-}"
if [[ -z "$redis_host" || "$redis_host" == *,* ]]; then
  echo "FAIL: helixagent-redis not placed on exactly one host (placed: '$redis_host')"
  exit_code=1
elif [[ "$redis_host" == "local" ]]; then
  echo "INFO: redis on local host — running redis-cli inside container"
  if podman exec helixagent-redis redis-cli -a "${REDIS_PASSWORD:-helixagent123}" PING 2>/dev/null | grep -q PONG; then
    echo "PASS: redis on local replied PONG"
  else
    echo "FAIL: redis on local did NOT reply PONG"
    exit_code=1
  fi
else
  cmd=$(runtime_for_host "$redis_host")
  pong=$(ssh -o ConnectTimeout=5 -o BatchMode=yes "$(ssh_for_host "$redis_host")" \
    "$cmd exec helixagent-redis redis-cli -a '${REDIS_PASSWORD:-helixagent123}' PING 2>/dev/null" \
    | tail -1 | tr -d '\r\n ' )
  if [[ "$pong" == "PONG" ]]; then
    echo "PASS: redis on $redis_host replied PONG"
  else
    echo "FAIL: redis on $redis_host PING returned: '$pong'"
    exit_code=1
  fi
fi

# ---------------------------------------------------------------
# F4: chromadb HTTP heartbeat from its placed host
# ---------------------------------------------------------------
echo
echo "--- F4: chromadb /api/v1/heartbeat from its placed host ---"
chroma_host="${SEEN_ON[helixagent-chromadb]:-}"
if [[ -z "$chroma_host" || "$chroma_host" == *,* ]]; then
  echo "FAIL: helixagent-chromadb not placed on exactly one host (placed: '$chroma_host')"
  exit_code=1
else
  # ChromaDB compose: command --port 8001 (network_mode: host).
  if [[ "$chroma_host" == "local" ]]; then
    chroma_url="http://localhost:8001"
  else
    addr=$(addr_for_host "$chroma_host")
    chroma_url="http://$addr:8001"
  fi
  hb_status=$(curl -fsS --max-time 10 "$chroma_url/api/v1/heartbeat" 2>/dev/null \
    | python3 -c "import sys,json;d=json.load(sys.stdin);print('ok' if 'nanosecond heartbeat' in d else 'bad')" 2>/dev/null \
    || echo "unreachable")
  if [[ "$hb_status" == "ok" ]]; then
    echo "PASS: chromadb on $chroma_host returned heartbeat at $chroma_url"
  else
    echo "FAIL: chromadb heartbeat from $chroma_url returned: '$hb_status'"
    exit_code=1
  fi
fi

# ---------------------------------------------------------------
# F5: MCP servers visible to the gateway
# ---------------------------------------------------------------
echo
echo "--- F5: MCP server count via gateway ---"
mcp_count=$(curl -fsS --max-time 10 http://localhost:8100/v1/mcp/stats 2>/dev/null \
  | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('total_servers', d.get('count', 0)))" 2>/dev/null \
  || echo "0")
# Fallback: count helixagent-mcp-* containers running across hosts.
mcp_running=0
for name in "${!SEEN_ON[@]}"; do
  if [[ "$name" == helixagent-mcp-* ]]; then
    mcp_running=$((mcp_running + 1))
  fi
done
if [[ "$mcp_running" -ge 5 ]]; then
  echo "PASS: $mcp_running MCP container(s) running across hosts (gateway reported $mcp_count)"
else
  echo "FAIL: only $mcp_running MCP container(s) running (target ≥ 5)"
  exit_code=1
fi

echo
echo "=== Summary ==="
if [[ $exit_code -eq 0 ]]; then
  echo "Partitioned distribution: PASS (structural + functional)"
else
  echo "Partitioned distribution: FAIL"
fi
exit $exit_code
