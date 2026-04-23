#!/usr/bin/env bash
set -euo pipefail

# CI Container Build System Challenge
# Validates the CI/CD container build infrastructure is correctly configured

WORKSPACE="${WORKSPACE:-$(cd "$(dirname "$0")/../.." && pwd)}"
TOTAL=0
PASSED=0
FAILED=0

pass() { TOTAL=$((TOTAL+1)); PASSED=$((PASSED+1)); echo "[PASS] $1"; }
fail() { TOTAL=$((TOTAL+1)); FAILED=$((FAILED+1)); echo "[FAIL] $1"; }

echo "========================================"
echo "CI Container Build System Challenge"
echo "========================================"
echo ""

# --- Section 1: File Structure ---
echo "--- Section 1: File Structure ---"

[ -f "${WORKSPACE}/docker-compose.ci.yml" ] && pass "docker-compose.ci.yml exists" || fail "docker-compose.ci.yml missing"
[ -f "${WORKSPACE}/docker/ci/Dockerfile.ci-go" ] && pass "Dockerfile.ci-go exists" || fail "Dockerfile.ci-go missing"
[ -f "${WORKSPACE}/docker/ci/Dockerfile.ci-mobile" ] && pass "Dockerfile.ci-mobile exists" || fail "Dockerfile.ci-mobile missing"
[ -f "${WORKSPACE}/docker/ci/Dockerfile.ci-emulator" ] && pass "Dockerfile.ci-emulator exists" || fail "Dockerfile.ci-emulator missing"
[ -f "${WORKSPACE}/docker/ci/Dockerfile.ci-web" ] && pass "Dockerfile.ci-web exists" || fail "Dockerfile.ci-web missing"
[ -f "${WORKSPACE}/docker/ci/Dockerfile.ci-reporter" ] && pass "Dockerfile.ci-reporter exists" || fail "Dockerfile.ci-reporter missing"
[ -f "${WORKSPACE}/docker/ci/emulator-start.sh" ] && pass "emulator-start.sh exists" || fail "emulator-start.sh missing"
[ -x "${WORKSPACE}/ci/scripts/ci-entrypoint.sh" ] && pass "ci-entrypoint.sh executable" || fail "ci-entrypoint.sh not executable"
[ -x "${WORKSPACE}/ci/scripts/ci-go.sh" ] && pass "ci-go.sh executable" || fail "ci-go.sh not executable"
[ -x "${WORKSPACE}/ci/scripts/ci-mobile.sh" ] && pass "ci-mobile.sh executable" || fail "ci-mobile.sh not executable"
[ -x "${WORKSPACE}/ci/scripts/ci-web.sh" ] && pass "ci-web.sh executable" || fail "ci-web.sh not executable"
[ -x "${WORKSPACE}/ci/scripts/ci-report.sh" ] && pass "ci-report.sh executable" || fail "ci-report.sh not executable"
[ -x "${WORKSPACE}/ci/scripts/wait-for-services.sh" ] && pass "wait-for-services.sh executable" || fail "wait-for-services.sh not executable"
[ -x "${WORKSPACE}/ci/scripts/false-positive-check.sh" ] && pass "false-positive-check.sh executable" || fail "false-positive-check.sh not executable"
[ -x "${WORKSPACE}/ci/scripts/validate-artifacts.sh" ] && pass "validate-artifacts.sh executable" || fail "validate-artifacts.sh not executable"
[ -f "${WORKSPACE}/ci/thresholds.json" ] && pass "thresholds.json exists" || fail "thresholds.json missing"
[ -f "${WORKSPACE}/ci/reporter/aggregate.js" ] && pass "aggregate.js exists" || fail "aggregate.js missing"
[ -f "${WORKSPACE}/ci/reporter/dashboard-template.html" ] && pass "dashboard-template.html exists" || fail "dashboard-template.html missing"
[ -f "${WORKSPACE}/ci/reporter/package.json" ] && pass "reporter package.json exists" || fail "reporter package.json missing"

echo ""

# --- Section 2: Android Signing ---
echo "--- Section 2: Android Signing ---"

[ -f "${WORKSPACE}/keys/android/debug.keystore" ] && pass "debug.keystore exists" || fail "debug.keystore missing"
[ -f "${WORKSPACE}/keys/android/README.md" ] && pass "keystore README exists" || fail "keystore README missing"

if [ -f "${WORKSPACE}/keys/android/debug.keystore" ]; then
  KEYSTORE_SIZE=$(stat -c%s "${WORKSPACE}/keys/android/debug.keystore" 2>/dev/null || echo "0")
  [ "${KEYSTORE_SIZE}" -gt 1000 ] && pass "debug.keystore has valid size (${KEYSTORE_SIZE} bytes)" || fail "debug.keystore too small (${KEYSTORE_SIZE} bytes)"
fi

echo ""

# --- Section 3: Docker Compose Profiles ---
echo "--- Section 3: Docker Compose Profiles ---"

COMPOSE="${WORKSPACE}/docker-compose.ci.yml"
grep -q "go-ci" "${COMPOSE}" && pass "go-ci profile defined" || fail "go-ci profile not defined"
grep -q "mobile-ci" "${COMPOSE}" && pass "mobile-ci profile defined" || fail "mobile-ci profile not defined"
grep -q "web-ci" "${COMPOSE}" && pass "web-ci profile defined" || fail "web-ci profile not defined"
grep -qE "^\s+- report$" "${COMPOSE}" && pass "report profile defined" || fail "report profile not defined"

echo ""

# --- Section 4: Integration Services ---
echo "--- Section 4: Integration Services ---"

grep -q "ci-postgres" "${COMPOSE}" && pass "PostgreSQL service configured" || fail "PostgreSQL service missing"
grep -q "ci-redis" "${COMPOSE}" && pass "Redis service configured" || fail "Redis service missing"
grep -q "ci-mockllm" "${COMPOSE}" && pass "Mock LLM service configured" || fail "Mock LLM service missing"
grep -q "ci-chromadb" "${COMPOSE}" && pass "ChromaDB service configured" || fail "ChromaDB service missing"
grep -q "ci-qdrant" "${COMPOSE}" && pass "Qdrant service configured" || fail "Qdrant service missing"
grep -q "ci-kafka" "${COMPOSE}" && pass "Kafka service configured" || fail "Kafka service missing"
grep -q "ci-rabbitmq" "${COMPOSE}" && pass "RabbitMQ service configured" || fail "RabbitMQ service missing"
grep -q "ci-minio" "${COMPOSE}" && pass "MinIO service configured" || fail "MinIO service missing"
grep -q "ci-oauthmock" "${COMPOSE}" && pass "OAuth Mock service configured" || fail "OAuth Mock service missing"

echo ""

# --- Section 5: Health Checks ---
echo "--- Section 5: Health Checks ---"

grep -q "healthcheck" "${COMPOSE}" && pass "Health checks present in compose" || fail "No health checks found"
grep -c "healthcheck" "${COMPOSE}" | grep -qE '^[5-9]|^[1-9][0-9]' && pass "Multiple health checks (5+)" || fail "Insufficient health checks"

echo ""

# --- Section 6: Makefile Targets ---
echo "--- Section 6: Makefile Targets ---"

MAKEFILE="${WORKSPACE}/Makefile"
grep -q "ci-go:" "${MAKEFILE}" && pass "ci-go target exists" || fail "ci-go target missing"
grep -q "ci-mobile:" "${MAKEFILE}" && pass "ci-mobile target exists" || fail "ci-mobile target missing"
grep -q "ci-web:" "${MAKEFILE}" && pass "ci-web target exists" || fail "ci-web target missing"
grep -q "ci-report:" "${MAKEFILE}" && pass "ci-report target exists" || fail "ci-report target missing"
grep -q "ci-all:" "${MAKEFILE}" && pass "ci-all target exists" || fail "ci-all target missing"
grep -q "ci-clean:" "${MAKEFILE}" && pass "ci-clean target exists" || fail "ci-clean target missing"
grep -q "ci-build-images:" "${MAKEFILE}" && pass "ci-build-images target exists" || fail "ci-build-images target missing"

echo ""

# --- Section 7: Resource Control ---
echo "--- Section 7: Resource Control ---"

grep -q "CI_RESOURCE_LIMIT" "${WORKSPACE}/ci/scripts/ci-entrypoint.sh" && pass "Resource limit in entrypoint" || fail "No resource limit in entrypoint"
grep -q "GOMAXPROCS" "${WORKSPACE}/ci/scripts/ci-entrypoint.sh" && pass "GOMAXPROCS configured" || fail "GOMAXPROCS not configured"
grep -q "nice" "${WORKSPACE}/ci/scripts/ci-entrypoint.sh" && pass "nice priority configured" || fail "nice not configured"
grep -q "ionice" "${WORKSPACE}/ci/scripts/ci-entrypoint.sh" && pass "ionice configured" || fail "ionice not configured"

echo ""

# --- Section 8: False Positive Prevention ---
echo "--- Section 8: False Positive Prevention ---"

grep -q "validate_junit_xml" "${WORKSPACE}/ci/scripts/false-positive-check.sh" && pass "JUnit XML validation function exists" || fail "JUnit XML validation missing"
grep -q "validate_coverage" "${WORKSPACE}/ci/scripts/false-positive-check.sh" && pass "Coverage validation function exists" || fail "Coverage validation missing"
grep -q "validate_binary" "${WORKSPACE}/ci/scripts/false-positive-check.sh" && pass "Binary validation function exists" || fail "Binary validation missing"
grep -q "validate_apk_signing" "${WORKSPACE}/ci/scripts/false-positive-check.sh" && pass "APK signing validation exists" || fail "APK signing validation missing"
grep -q "write_fp_report" "${WORKSPACE}/ci/scripts/false-positive-check.sh" && pass "FP report writer exists" || fail "FP report writer missing"

echo ""

# --- Section 9: Podman Compatibility ---
echo "--- Section 9: Podman Compatibility ---"

# Modern podman-compose (>=1.0) honors Docker-style deploy: blocks
# for resource limits; the earlier "no deploy blocks" restriction
# was a leftover from older podman-compose. Presence is now the
# desired state (CI needs resource limits for host safety).
grep -q "deploy:" "${COMPOSE}" && pass "deploy blocks present (podman-compose >=1.0 honors them)" || fail "no deploy blocks — CI lacks resource limits"
grep -q "CI_IS_DOCKER" "${MAKEFILE}" && pass "Docker/Podman detection in Makefile" || fail "No Docker/Podman detection"
grep -q "podman wait" "${MAKEFILE}" && pass "podman wait for container completion" || fail "No podman wait support"
# Either runtime-side network_mode: host OR build-context network: host
# is sufficient to give Podman rootless npm/apt registry access.
if grep -q "network_mode: host" "${COMPOSE}" || grep -q "network: host" "${COMPOSE}"; then
  pass "Host networking present for Podman rootless"
else
  fail "No host networking for Podman rootless"
fi
grep -q "network: host" "${COMPOSE}" && pass "Host networking for image builds" || fail "No host networking for builds"

# npm install fallback (Podman rootless can't reach npm registry with bridge network)
grep -q 'npm install' "${WORKSPACE}/ci/scripts/ci-web.sh" && pass "Web CI uses npm install fallback" || fail "Web CI missing npm install fallback"
grep -q 'npm install' "${WORKSPACE}/ci/scripts/ci-mobile.sh" && pass "Mobile CI uses npm install fallback" || fail "Mobile CI missing npm install fallback"
grep -q 'npm install' "${WORKSPACE}/docker/ci/Dockerfile.ci-reporter" && pass "Reporter uses npm install" || fail "Reporter uses npm ci without lockfile"

echo ""

# --- Section 10: Thresholds Configuration ---
echo "--- Section 10: Thresholds Configuration ---"

if [ -f "${WORKSPACE}/ci/thresholds.json" ]; then
  jq empty "${WORKSPACE}/ci/thresholds.json" 2>/dev/null && pass "thresholds.json valid JSON" || fail "thresholds.json invalid JSON"
  jq -e '.go' "${WORKSPACE}/ci/thresholds.json" >/dev/null 2>&1 && pass "Go thresholds configured" || fail "Go thresholds missing"
  jq -e '.mobile' "${WORKSPACE}/ci/thresholds.json" >/dev/null 2>&1 && pass "Mobile thresholds configured" || fail "Mobile thresholds missing"
  jq -e '.web' "${WORKSPACE}/ci/thresholds.json" >/dev/null 2>&1 && pass "Web thresholds configured" || fail "Web thresholds missing"
fi

echo ""

# --- Section 11: Documentation ---
echo "--- Section 11: Documentation ---"

[ -f "${WORKSPACE}/docs/CI_BUILD_GUIDE.md" ] && pass "CI Build Guide exists" || fail "CI Build Guide missing"
grep -q "ci-all" "${WORKSPACE}/docs/CI_BUILD_GUIDE.md" && pass "Guide documents ci-all" || fail "Guide missing ci-all docs"
grep -q "CI_RESOURCE_LIMIT" "${WORKSPACE}/docs/CI_BUILD_GUIDE.md" && pass "Guide documents resource limits" || fail "Guide missing resource docs"
grep -qi "false.positive\|false-positive\|FP\|false_positive" "${WORKSPACE}/docs/CI_BUILD_GUIDE.md" 2>/dev/null && pass "Guide documents false positive prevention" || fail "Guide missing FP docs"

echo ""

# --- Section 12: Phase Pipeline Scripts ---
echo "--- Section 12: Phase Pipeline Scripts ---"

# Go pipeline checks
grep -q "wait-for-services" "${WORKSPACE}/ci/scripts/ci-go.sh" && pass "Go pipeline waits for services" || fail "Go pipeline missing service wait"
grep -q "gotestsum" "${WORKSPACE}/ci/scripts/ci-go.sh" && pass "Go pipeline uses gotestsum" || fail "Go pipeline missing gotestsum"
grep -q "false-positive-check" "${WORKSPACE}/ci/scripts/ci-go.sh" && pass "Go pipeline has FP checks" || fail "Go pipeline missing FP checks"
grep -q "validate-artifacts" "${WORKSPACE}/ci/scripts/ci-go.sh" && pass "Go pipeline validates artifacts" || fail "Go pipeline missing artifact validation"

# Mobile pipeline checks
grep -q "flutter" "${WORKSPACE}/ci/scripts/ci-mobile.sh" 2>/dev/null && pass "Mobile pipeline has Flutter" || fail "Mobile pipeline missing Flutter"
grep -q "Robolectric\|robolectric" "${WORKSPACE}/ci/scripts/ci-mobile.sh" 2>/dev/null && pass "Mobile pipeline has Robolectric" || fail "Mobile pipeline missing Robolectric"

# Web pipeline checks
grep -q "ng build\|angular" "${WORKSPACE}/ci/scripts/ci-web.sh" 2>/dev/null && pass "Web pipeline has Angular" || fail "Web pipeline missing Angular"
grep -q "Playwright\|playwright" "${WORKSPACE}/ci/scripts/ci-web.sh" 2>/dev/null && pass "Web pipeline has Playwright" || fail "Web pipeline missing Playwright"
grep -q "Lighthouse\|lighthouse" "${WORKSPACE}/ci/scripts/ci-web.sh" 2>/dev/null && pass "Web pipeline has Lighthouse" || fail "Web pipeline missing Lighthouse"

echo ""

# --- Section 13: Report Aggregator ---
echo "--- Section 13: Report Aggregator ---"

grep -q "fast-xml-parser" "${WORKSPACE}/ci/reporter/package.json" && pass "Reporter uses fast-xml-parser" || fail "Reporter missing XML parser"
grep -q "glob" "${WORKSPACE}/ci/reporter/package.json" && pass "Reporter uses glob" || fail "Reporter missing glob"
grep -q "results.json" "${WORKSPACE}/ci/reporter/aggregate.js" && pass "Reporter generates results.json" || fail "Reporter missing results output"
grep -q "summary.html" "${WORKSPACE}/ci/reporter/aggregate.js" && pass "Reporter generates dashboard" || fail "Reporter missing dashboard"
! grep -q "innerHTML" "${WORKSPACE}/ci/reporter/dashboard-template.html" && pass "Dashboard uses safe DOM methods" || fail "Dashboard uses innerHTML (XSS risk)"
! grep -q "execSync" "${WORKSPACE}/ci/reporter/aggregate.js" && pass "Aggregator uses execFileSync" || fail "Aggregator uses execSync (injection risk)"

echo ""

# --- Section 14: Network Configuration ---
echo "--- Section 14: Network Configuration ---"

grep -q "ci-network" "${COMPOSE}" && pass "CI network defined for Go CI services" || fail "CI network missing"
grep -q "driver: bridge" "${COMPOSE}" && pass "Bridge network driver for integration services" || fail "Not using bridge driver"

# FP check JSON sanitization
grep -q "tr -d" "${WORKSPACE}/ci/scripts/false-positive-check.sh" && pass "FP checks sanitize newlines" || fail "FP checks missing newline sanitization"

echo ""

# --- Summary ---
echo "========================================"
echo "CI Container Build Challenge Complete"
echo "========================================"
echo "Total:  ${TOTAL}"
echo "Passed: ${PASSED}"
echo "Failed: ${FAILED}"
echo ""

if [ "${FAILED}" -gt 0 ]; then
  echo "[RESULT] FAILED — ${FAILED} check(s) did not pass"
  exit 1
else
  echo "[RESULT] PASSED — All ${TOTAL} checks passed"
  exit 0
fi
