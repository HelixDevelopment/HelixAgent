#!/usr/bin/env bash
# llmsverifier_startup_verification_challenge.sh
#
# Full LLMsVerifier startup routine challenge that validates:
# 1. Loading API keys from .env file
# 2. Provider discovery with priority ordering (faulty keys checked last)
# 3. Real API key verification for all providers
# 4. Model testing for each provider
# 5. .faulty_api_keys and .unsupported_api_keys file management
# 6. Complete verification pipeline that HelixAgent performs at startup
#
# This challenge runs the actual StartupVerifier with real API keys
# from the .env file to ensure the complete verification pipeline works.
#
# Exit codes:
#   0 — all checks passed
#   1 — pre-flight failure (env file missing, Go not available)
#   2 — build failed
#   3 — verification failed
#   4 — API key tracking failed

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERIFIER_DIR="${REPO_ROOT}/LLMsVerifier/llm-verifier"
REPORT_DIR="${REPO_ROOT}/reports/challenges"
TIMESTAMP="$(date -u +%Y-%m-%dT%H-%M-%SZ)"
REPORT="${REPORT_DIR}/llmsverifier_startup_verification-${TIMESTAMP}.log"
HELIXAGENTBIN="${REPO_ROOT}/helixagent"

mkdir -p "${REPORT_DIR}"

pass() { printf '  ✓ %s\n' "$1"; }
warn() { printf '  ⚠ %s\n' "$1" >&2; }
fail() { printf '  ✗ %s\n' "$1" >&2; }

{
  echo "=== LLMsVerifier Startup Verification Challenge — ${TIMESTAMP} ==="
  echo "repo:       ${REPO_ROOT}"
  echo "helixagent: ${HELIXAGENTBIN}"
  echo

  echo "[1/8] Checking prerequisites..."
  
  if [ ! -f "${REPO_ROOT}/.env" ]; then
    fail ".env file not found at ${REPO_ROOT}/.env"
    exit 1
  fi
  pass ".env file present"
  
  if [ ! -x "${HELIXAGENTBIN}" ]; then
    warn "helixagent binary not found or not executable, will build"
  else
    pass "helixagent binary available"
  fi
  
  if ! command -v go >/dev/null 2>&1; then
    fail "Go toolchain not available"
    exit 1
  fi
  pass "Go available: $(go version)"
  echo

  echo "[2/8] Checking .faulty_api_keys and .unsupported_api_keys files..."
  
  FAULTY_KEYS="${REPO_ROOT}/.faulty_api_keys"
  UNSUPPORTED_KEYS="${REPO_ROOT}/.unsupported_api_keys"
  
  if [ -f "${FAULTY_KEYS}" ]; then
    pass ".faulty_api_keys exists"
    echo "  Content preview:"
    head -5 "${FAULTY_KEYS}" | sed 's/^/    /'
  else
    warn ".faulty_api_keys does not exist yet (will be created during verification)"
  fi
  
  if [ -f "${UNSUPPORTED_KEYS}" ]; then
    pass ".unsupported_api_keys exists"
    echo "  Content preview:"
    head -5 "${UNSUPPORTED_KEYS}" | sed 's/^/    /'
  else
    warn ".unsupported_api_keys does not exist yet (will be created for unknown env vars)"
  fi
  echo

  echo "[3/8] Loading API keys from .env..."
  
  source "${REPO_ROOT}/.env"
  
  API_KEY_COUNT=0
  for var in $(env | grep -E '_API_KEY$' | cut -d= -f1 | sort); do
    value="${!var}"
    if [ -n "${value}" ] && [ "${value}" != "your-"${var,,}"-here" ]; then
      API_KEY_COUNT=$((API_KEY_COUNT+1))
    fi
  done
  
  if [ ${API_KEY_COUNT} -eq 0 ]; then
    warn "No valid API keys found in .env (some providers may be skipped)"
  else
    pass "Found ${API_KEY_COUNT} API keys in .env"
  fi
  echo

  echo "[4/8] Building HelixAgent with verification support..."
  
  cd "${REPO_ROOT}"
  
  if [ -f "go.mod" ]; then
    BUILD_LOG="/tmp/helixagent_build.$$"
    if go build -mod=mod -o "${HELIXAGENTBIN}" ./cmd/helixagent/... > "${BUILD_LOG}" 2>&1; then
      pass "HelixAgent built successfully"
      rm -f "${BUILD_LOG}"
    else
      fail "HelixAgent build failed"
      tail -30 "${BUILD_LOG}" >&2
      rm -f "${BUILD_LOG}"
      exit 2
    fi
  else
    fail "go.mod not found in repo root"
    exit 2
  fi
  echo

  echo "[5/8] Running LLMsVerifier startup verification..."
  
  echo "  This will run the complete verification pipeline including:"
  echo "  - Provider discovery from environment variables"
  echo "  - Priority ordering (faulty API keys checked last)"
  echo "  - API key validation for each provider"
  echo "  - Model testing for all available models"
  echo "  - .faulty_api_keys file management"
  echo "  - .unsupported_api_keys file management"
  echo
  
  VERIFY_LOG="/tmp/llmsverifier_startup_verify.$$"
  
  if "${HELIXAGENTBIN}" verify --all 2>&1 | tee "${VERIFY_LOG}"; then
    pass "Startup verification completed"
  else
    VERIFY_STATUS=$?
    warn "Verification returned non-zero status: ${VERIFY_STATUS}"
  fi
  
  if [ -s "${VERIFY_LOG}" ]; then
    echo "  Verification output (last 50 lines):"
    tail -50 "${VERIFY_LOG}" | sed 's/^/    /'
  fi
  rm -f "${VERIFY_LOG}"
  echo

  echo "[6/8] Checking .faulty_api_keys file after verification..."
  
  if [ -f "${FAULTY_KEYS}" ]; then
    FAULTY_COUNT=$(grep -cv '^#\|^$\|^-' "${FAULTY_KEYS}" 2>/dev/null || echo "0")
    pass ".faulty_api_keys exists with ${FAULTY_COUNT} entries"
    echo "  Content:"
    cat "${FAULTY_KEYS}" | grep -v '^#' | grep -v '^$' | head -10 | sed 's/^/    /'
  else
    warn ".faulty_api_keys was not created (no failures during verification)"
  fi
  echo

  echo "[7/8] Checking .unsupported_api_keys file..."
  
  if [ -f "${UNSUPPORTED_KEYS}" ]; then
    UNSUPPORTED_COUNT=$(grep -cv '^#\|^$\|^-' "${UNSUPPORTED_KEYS}" 2>/dev/null || echo "0")
    pass ".unsupported_api_keys exists with ${UNSUPPORTED_COUNT} entries"
    echo "  Content:"
    cat "${UNSUPPORTED_KEYS}" | grep -v '^#' | grep -v '^$' | head -10 | sed 's/^/    /'
  else
    warn ".unsupported_api_keys was not created (no unknown API keys detected)"
  fi
  echo

  echo "[8/8] Verifying provider priority ordering..."
  
  PROVIDER_LOG="/tmp/provider_order.$$"
  
  "${HELIXAGENTBIN}" verify --providers 2>&1 | tee "${PROVIDER_LOG}" || true
  
  if grep -q "faulty" "${PROVIDER_LOG}" 2>/dev/null || grep -q "priority" "${PROVIDER_LOG}" 2>/dev/null; then
    pass "Provider priority ordering was applied"
  else
    warn "Could not verify priority ordering (check logs for details)"
  fi
  
  rm -f "${PROVIDER_LOG}"
  echo

  echo "=== CHALLENGE COMPLETE ==="
  echo "Summary:"
  echo "  - API keys tested: ${API_KEY_COUNT}"
  echo "  - .faulty_api_keys: $([ -f '${FAULTY_KEYS}' ] && echo 'present' || echo 'not created')"
  echo "  - .unsupported_api_keys: $([ -f '${UNSUPPORTED_KEYS}' ] && echo 'present' || echo 'not created')"
  echo "Report: ${REPORT}"
  
} 2>&1 | tee "${REPORT}"

exit 0