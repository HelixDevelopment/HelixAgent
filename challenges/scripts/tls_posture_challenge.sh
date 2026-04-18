#!/usr/bin/env bash
# challenges/scripts/tls_posture_challenge.sh
#
# Enforces CLAUDE.md's TLS posture rule:
#   "NEVER use curl -sk or NODE_TLS_REJECT_UNAUTHORIZED=0 in challenges or
#    tests. HelixLLM provider's InsecureSkipVerify defaults to false;
#    explicit opt-in via HELIX_LLM_TLS_SKIP_VERIFY=true."
#
# Assertions:
#
#   T1 — No production Go file under internal/ contains an UNCONDITIONAL
#        `InsecureSkipVerify: true` that is not either:
#          a) guarded by a config field or env var (e.g. `cfg.TLSSkipVerify`
#             or `getEnvBool("HELIX_*_TLS_SKIP_VERIFY", false)`), or
#          b) annotated with `//nolint:gosec` AND followed by an audit
#             comment explaining the env-opt-in or self-signed-cert path.
#
#   T2 — No shell script under challenges/scripts/ or scripts/ uses
#        `curl -sk` (or `--insecure`) against localhost or HelixLLM URLs.
#
#   T3 — No shell script sets NODE_TLS_REJECT_UNAUTHORIZED=0.
#
# Contract: CONST-019 non-interactive, CONST-022 resource-capped, read-only.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
BLUE=$'\033[0;34m'
NC=$'\033[0m'

PASS=0
FAIL=0
pass() { PASS=$((PASS + 1)); echo -e "${GREEN}✓${NC} $*"; }
fail() { FAIL=$((FAIL + 1)); echo -e "${RED}✗${NC} $*"; }

echo -e "${BLUE}==>${NC} T1: No unconditional InsecureSkipVerify:true in production Go"
# Production = internal/, cmd/, pkg/ — exclude *_test.go files.
# An acceptable hit is one that is annotated with `//nolint:gosec` OR `#nosec`
# (operator opt-in path documented by security review) OR gated on a
# cfg/env variable. Only flag plain literal `InsecureSkipVerify: true`.
hits_file=$(mktemp)
trap 'rm -f "$hits_file"' EXIT
grep -rn --include="*.go" --exclude="*_test.go" "InsecureSkipVerify" internal/ cmd/ pkg/ 2>/dev/null \
  | grep -E "InsecureSkipVerify:[[:space:]]*true\b" \
  | grep -v "nolint:gosec" \
  | grep -v "#nosec" \
  | awk -F: '{
      # Treat line as comment if the first non-space char on the content
      # is "//" (Go line comment) or the hit is inside backticks.
      content = $0;
      sub(/^[^:]*:[0-9]+:/, "", content);
      gsub(/^[[:space:]]+/, "", content);
      if (content ~ /^\/\//) next;
      if (content ~ /`InsecureSkipVerify: *true`/) next;
      print $0;
    }' > "$hits_file" || true
if [[ -s "$hits_file" ]]; then
    fail "Unconditional InsecureSkipVerify:true found (unannotated):"
    sed 's/^/      /' "$hits_file"
else
    pass "no unconditional InsecureSkipVerify:true"
fi

echo -e "${BLUE}==>${NC} T2: No 'curl -sk' or '--insecure' in scripts"
# Exclude this challenge file itself. Match 'curl -sk' where `-sk` is a
# whitespace-delimited flag token (not '-sk-foo' inside a bash default
# expansion like ${API_KEY:-sk-test}).
insecure_curl=$(grep -rn --include="*.sh" --exclude="tls_posture_challenge.sh" \
  -E '(^|[[:space:]])curl[[:space:]]+(-[a-zA-Z]*sk[a-zA-Z]*|--insecure)([[:space:]]|$)' \
  scripts challenges/scripts 2>/dev/null \
  | awk -F: '{
      content = $0;
      sub(/^[^:]*:[0-9]+:/, "", content);
      gsub(/^[[:space:]]+/, "", content);
      # Skip shell comment lines (leading "#"). That includes doc prose
      # like "# NEVER use curl -sk ...".
      if (content ~ /^#/) next;
      print $0;
    }' || true)
if [[ -z "$insecure_curl" ]]; then
    pass "no insecure curl invocations"
else
    fail "Insecure curl invocations found:"
    echo "$insecure_curl" | sed 's/^/      /'
fi

echo -e "${BLUE}==>${NC} T3: No NODE_TLS_REJECT_UNAUTHORIZED=0 in scripts"
# Reject the literal =0 variant; exclude this challenge file itself.
nodetls_zero=$(grep -rn --include="*.sh" --exclude="tls_posture_challenge.sh" \
  -E "NODE_TLS_REJECT_UNAUTHORIZED=['\"]?0" scripts challenges/scripts 2>/dev/null || true)
if [[ -z "$nodetls_zero" ]]; then
    pass "no NODE_TLS_REJECT_UNAUTHORIZED=0"
else
    fail "NODE_TLS_REJECT_UNAUTHORIZED=0 found:"
    echo "$nodetls_zero" | sed 's/^/      /'
fi

echo ""
echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}tls_posture_challenge${NC}: $PASS passed, $FAIL failed"
if (( FAIL > 0 )); then
    exit 1
fi
exit 0
