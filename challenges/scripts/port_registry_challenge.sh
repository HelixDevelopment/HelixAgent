#!/usr/bin/env bash
#
# Challenge: Port Registry Integrity
#
# Validates that the canonical port registry (internal/ports) remains
# coherent: no collisions, every port in 16-bit range at both prefixes,
# band discipline preserved, the running HelixAgent binary binds on
# the HTTP port it claims, and core env-var overrides round-trip.
#
# Unlike the Go unit tests which exercise the registry in isolation,
# this challenge verifies the LIVE behavior against a running
# HelixAgent instance (CONST-030: real infrastructure for non-unit
# tests).
#
# Usage:
#   ./challenges/scripts/port_registry_challenge.sh
#
# Expects HelixAgent running on $HELIXAGENT_PORT_HTTP (default 8100).
# Skips (not fails) health-endpoint checks when the binary is not up.

set -u
set -o pipefail

# Resource limits per CONST-015 / the host-safety rule.
if [ -z "${GOMAXPROCS:-}" ]; then
  export GOMAXPROCS=2
fi

readonly REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT" || exit 1

PORT_HTTP="${HELIXAGENT_PORT_HTTP:-8100}"
PORT_PREFIX="${HELIXAGENT_PORT_PREFIX:-8}"

fail_count=0
pass_count=0
skip_count=0

ok()   { printf '\033[32m[PASS]\033[0m %s\n' "$*"; pass_count=$((pass_count + 1)); }
skip() { printf '\033[33m[SKIP]\033[0m %s\n' "$*"; skip_count=$((skip_count + 1)); }
fail() { printf '\033[31m[FAIL]\033[0m %s\n' "$*"; fail_count=$((fail_count + 1)); }

printf '==[ Port Registry Integrity Challenge ]==\n\n'

# ---- 1. Registry package builds -------------------------------------------

if go build ./internal/ports/ >/dev/null 2>&1; then
  ok "internal/ports builds cleanly"
else
  fail "internal/ports fails to build"
fi

# ---- 2. Unit tests pass (fast repeat of go test) --------------------------

if go test -short -count=1 ./internal/ports/ >/dev/null 2>&1; then
  ok "internal/ports unit tests pass"
else
  fail "internal/ports unit tests fail"
fi

# ---- 3. Collision invariant holds ----------------------------------------
# Fails explicitly if two services share an offset.

if go test -short -count=1 -run TestOffsets_NoCollisions ./internal/ports/ >/dev/null 2>&1; then
  ok "no offset collisions in registry"
else
  fail "offset collision detected — two services share a port number"
fi

# ---- 4. 16-bit range invariant at both prefixes --------------------------

if go test -short -count=1 -run TestOffsets_FitIn16BitAtBothPrefixes ./internal/ports/ >/dev/null 2>&1; then
  ok "every port fits in 16 bits at both prefixes 8 and 9"
else
  fail "some port overflows 16 bits at prefix 8 or 9"
fi

# ---- 5. Band discipline (core ≤199, MCP 200-281, observability 300-312) --

if go test -short -count=1 -run TestOffsets_WithinExpectedBands ./internal/ports/ >/dev/null 2>&1; then
  ok "band discipline preserved (core ≤199, MCP 200-281, obs 300-312)"
else
  fail "band discipline violated — a service has an offset outside its expected band"
fi

# ---- 6. Every env var starts with HELIXAGENT_PORT_ -----------------------

if go test -short -count=1 -run TestEnvVarNames_AllStartWithHelixAgentPrefix ./internal/ports/ >/dev/null 2>&1; then
  ok "every registered service uses a HELIXAGENT_PORT_* env var"
else
  fail "env-var naming convention violation — a service uses a non-canonical name"
fi

# ---- 7. Prefix=9 alternate band shifts every port correctly --------------

if go test -short -count=1 -run TestPrefix_EnvNineShiftsBand ./internal/ports/ >/dev/null 2>&1; then
  ok "prefix=9 correctly shifts entire band"
else
  fail "prefix=9 does not uniformly shift the port band"
fi

# ---- 8. Invalid prefix falls back to default ----------------------------

if go test -short -count=1 -run TestPrefix_InvalidValuesFallBackToDefault ./internal/ports/ >/dev/null 2>&1; then
  ok "invalid prefix values fall back to DefaultPrefix"
else
  fail "invalid prefix handling is broken"
fi

# ---- 9. docs/development/port-registry.md exists and mentions core ports -

REG_DOC="$REPO_ROOT/docs/development/port-registry.md"
if [ -f "$REG_DOC" ]; then
  required_entries=(8100 8101 8102 8105 8120 8121 HELIXAGENT_PORT_PREFIX)
  missing=()
  for entry in "${required_entries[@]}"; do
    if ! grep -q "$entry" "$REG_DOC"; then
      missing+=("$entry")
    fi
  done
  if [ ${#missing[@]} -eq 0 ]; then
    ok "port-registry doc contains every core port + prefix reference"
  else
    fail "port-registry doc missing entries: ${missing[*]}"
  fi
else
  fail "docs/development/port-registry.md not present"
fi

# ---- 10. CLAUDE.md CONST-027 updated to 8100-band ------------------------

if grep -q 'HelixAgent HTTP \*\*8100\*\*' CLAUDE.md 2>/dev/null ||
   grep -q 'HelixAgent default ports live in the 81xx band' CLAUDE.md 2>/dev/null; then
  ok "CLAUDE.md CONST-027 reflects the 81xx port band"
else
  fail "CLAUDE.md CONST-027 still references pre-migration ports (7061)"
fi

# ---- 11. .env.example carries canonical HELIXAGENT_PORT_* entries --------

if grep -q '^HELIXAGENT_PORT_HTTP=' .env.example 2>/dev/null &&
   grep -q '^HELIXAGENT_PORT_POSTGRES=' .env.example 2>/dev/null; then
  ok ".env.example seeds canonical HELIXAGENT_PORT_* vars"
else
  fail ".env.example missing canonical HELIXAGENT_PORT_* entries"
fi

# ---- 12. Main docker-compose uses the new defaults -----------------------

if grep -q '\${PORT:-8100}:7061' docker-compose.yml 2>/dev/null &&
   grep -q 'NEO4J_HTTP:-8123' docker-compose.yml 2>/dev/null; then
  ok "docker-compose.yml uses 81xx defaults in port mappings"
else
  fail "docker-compose.yml still has pre-migration port defaults"
fi

# ---- 13. Live binary binds on the advertised HTTP port -------------------

if curl -sf --max-time 3 "http://localhost:$PORT_HTTP/v1/health" >/dev/null 2>&1; then
  ok "live HelixAgent binds on :$PORT_HTTP and /v1/health responds"
elif command -v pgrep >/dev/null 2>&1 &&
     pgrep -fo '\./bin/helixagent$' >/dev/null 2>&1; then
  # Process up — consult uptime. A healthy HelixAgent reaches
  # `/v1/health` within ~10 minutes even under remote distribution
  # with cold-cache hosts. Beyond that, something is wrong.
  pid=$(pgrep -fo '\./bin/helixagent$')
  etime_s=$(ps -o etimes= -p "$pid" 2>/dev/null | tr -d ' ')
  if [ -n "$etime_s" ] && [ "$etime_s" -lt 900 ]; then
    skip "HelixAgent is booting (up ${etime_s}s); live health check deferred"
  else
    fail "HelixAgent running ${etime_s:-?}s but /v1/health on :$PORT_HTTP does not respond"
  fi
else
  skip "HelixAgent not running — live port-bind check skipped"
fi

# ---- Report -------------------------------------------------------------

printf '\n'
printf 'Pass: %d | Fail: %d | Skip: %d\n' "$pass_count" "$fail_count" "$skip_count"
if [ "$fail_count" -eq 0 ]; then
  printf '\033[32mPort registry challenge: PASS\033[0m\n'
  exit 0
fi
printf '\033[31mPort registry challenge: FAIL\033[0m\n'
exit 1
