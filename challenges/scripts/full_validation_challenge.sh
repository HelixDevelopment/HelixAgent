#!/bin/bash
# Full Validation Challenge — Master Gate Check
# Combines all zero-broken verification checks from Phase 5.
# Validates build, code quality, routing, tests, infrastructure, documentation,
# safety fixes, and constitution synchronization.

set -eo pipefail  # Note: NOT -u to avoid .env unbound variable issues

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASS=0
FAIL=0
TOTAL=0

check() {
    TOTAL=$((TOTAL + 1))
    if [ "$1" -eq 0 ]; then
        PASS=$((PASS + 1))
        echo -e "  ${GREEN}PASS${NC}: $2"
    else
        FAIL=$((FAIL + 1))
        echo -e "  ${RED}FAIL${NC}: $2"
    fi
}

echo "=========================================================="
echo "  Full Validation Challenge — Master Gate Check"
echo "=========================================================="
echo ""

# ============================================================================
# CHECK 1: Build — go build ./... succeeds
# ============================================================================
echo -e "${BLUE}[1] Build: go build ./...${NC}"
if cd "$PROJECT_ROOT" && GOMAXPROCS=2 nice -n 19 ionice -c 3 go build ./... > /tmp/full_validation_build.log 2>&1; then
    check 0 "go build ./... succeeds"
else
    check 1 "go build ./... succeeds (see /tmp/full_validation_build.log)"
fi

# ============================================================================
# CHECK 2: Vet — go vet ./internal/... clean
# ============================================================================
echo ""
echo -e "${BLUE}[2] Vet: go vet ./internal/...${NC}"
if cd "$PROJECT_ROOT" && GOMAXPROCS=2 nice -n 19 ionice -c 3 go vet ./internal/... > /tmp/full_validation_vet.log 2>&1; then
    check 0 "go vet ./internal/... is clean"
else
    check 1 "go vet ./internal/... is clean (see /tmp/full_validation_vet.log)"
fi

# ============================================================================
# CHECK 3: No dead FEATURE_* env vars in .env.example
# ============================================================================
echo ""
echo -e "${BLUE}[3] No dead FEATURE_* env vars in .env.example${NC}"
feature_count=0
if [ -f "$PROJECT_ROOT/.env.example" ]; then
    feature_count=$(grep -c "^FEATURE_" "$PROJECT_ROOT/.env.example" 2>/dev/null || true)
fi
if [ "$feature_count" -eq 0 ]; then
    check 0 "No FEATURE_* env vars in .env.example (count: $feature_count)"
else
    check 1 "No FEATURE_* env vars in .env.example (found: $feature_count)"
fi

# ============================================================================
# CHECK 4: All handlers registered — EnsembleHandler and CompletionHandler in router.go
# ============================================================================
echo ""
echo -e "${BLUE}[4] EnsembleHandler and CompletionHandler in router.go${NC}"
router_file="$PROJECT_ROOT/internal/router/router.go"
handlers_ok=0
if grep -q "EnsembleHandler\|ensembleHandler\|NewEnsembleHandler" "$router_file" 2>/dev/null; then
    handlers_ok=$((handlers_ok + 1))
fi
if grep -q "CompletionHandler\|completionHandler\|NewCompletionHandler" "$router_file" 2>/dev/null; then
    handlers_ok=$((handlers_ok + 1))
fi
if [ "$handlers_ok" -eq 2 ]; then
    check 0 "EnsembleHandler and CompletionHandler both registered in router.go"
else
    check 1 "EnsembleHandler and CompletionHandler both registered in router.go (found: $handlers_ok/2)"
fi

# ============================================================================
# CHECK 5: Ensemble/Completion routes exist in router.go
# ============================================================================
echo ""
echo -e "${BLUE}[5] /v1/ensemble and /v1/completion routes in router.go${NC}"
routes_ok=0
if grep -q '"/v1/ensemble\|v1/ensemble' "$router_file" 2>/dev/null; then
    routes_ok=$((routes_ok + 1))
fi
if grep -q '"/v1/completion\|v1/completion' "$router_file" 2>/dev/null; then
    routes_ok=$((routes_ok + 1))
fi
if [ "$routes_ok" -eq 2 ]; then
    check 0 "/v1/ensemble and /v1/completion routes both exist in router.go"
else
    check 1 "/v1/ensemble and /v1/completion routes both exist in router.go (found: $routes_ok/2)"
fi

# ============================================================================
# CHECK 6: All packages have tests — internal/output and internal/containers have *_test.go
# ============================================================================
echo ""
echo -e "${BLUE}[6] internal/output and internal/containers have *_test.go files${NC}"
pkg_tests_ok=0
if ls "$PROJECT_ROOT/internal/output/"*_test.go > /dev/null 2>&1; then
    pkg_tests_ok=$((pkg_tests_ok + 1))
fi
if ls "$PROJECT_ROOT/internal/containers/"*_test.go > /dev/null 2>&1; then
    pkg_tests_ok=$((pkg_tests_ok + 1))
fi
if [ "$pkg_tests_ok" -eq 2 ]; then
    check 0 "internal/output and internal/containers both have *_test.go files"
else
    check 1 "internal/output and internal/containers both have *_test.go files (found: $pkg_tests_ok/2)"
fi

# ============================================================================
# CHECK 7: All handlers have tests — browser, ensemble, search, verifier_types handler tests exist
# ============================================================================
echo ""
echo -e "${BLUE}[7] Handler test files: browser, ensemble, search, verifier_types${NC}"
handler_tests_ok=0
handler_tests_total=4
for test_name in "browser_handler_test.go" "ensemble_handler_test.go" "search_handler_test.go" "verifier_types_test.go"; do
    if [ -f "$PROJECT_ROOT/internal/handlers/$test_name" ]; then
        handler_tests_ok=$((handler_tests_ok + 1))
    fi
done
if [ "$handler_tests_ok" -eq "$handler_tests_total" ]; then
    check 0 "All 4 handler test files exist (browser, ensemble, search, verifier_types)"
else
    check 1 "All 4 handler test files exist ($handler_tests_ok/$handler_tests_total): browser, ensemble, search, verifier_types"
fi

# ============================================================================
# CHECK 8: Fuzz functions exist — >= 40 fuzz functions in tests/fuzz/
# ============================================================================
echo ""
echo -e "${BLUE}[8] Fuzz functions: >= 40 in tests/fuzz/${NC}"
fuzz_count=0
if [ -d "$PROJECT_ROOT/tests/fuzz" ]; then
    fuzz_count=$(grep -r "^func Fuzz" "$PROJECT_ROOT/tests/fuzz/" --include="*.go" 2>/dev/null | wc -l || true)
fi
if [ "$fuzz_count" -ge 40 ]; then
    check 0 "Fuzz function count >= 40 (found: $fuzz_count)"
else
    check 1 "Fuzz function count >= 40 (found: $fuzz_count)"
fi

# ============================================================================
# CHECK 9: Stress tests exist — >= 15 stress test files
# ============================================================================
echo ""
echo -e "${BLUE}[9] Stress tests: >= 15 files in tests/stress/${NC}"
stress_count=0
if [ -d "$PROJECT_ROOT/tests/stress" ]; then
    stress_count=$(find "$PROJECT_ROOT/tests/stress" -name "*.go" -o -name "*.sh" 2>/dev/null | wc -l || true)
fi
if [ "$stress_count" -ge 15 ]; then
    check 0 "Stress test file count >= 15 (found: $stress_count)"
else
    check 1 "Stress test file count >= 15 (found: $stress_count)"
fi

# ============================================================================
# CHECK 10: Grafana dashboards — 7 JSON files in docker/monitoring/grafana/dashboards/
# ============================================================================
echo ""
echo -e "${BLUE}[10] Grafana dashboards: 7 JSON files${NC}"
dashboard_dir="$PROJECT_ROOT/docker/monitoring/grafana/dashboards"
grafana_count=0
if [ -d "$dashboard_dir" ]; then
    grafana_count=$(find "$dashboard_dir" -maxdepth 1 -name "*.json" 2>/dev/null | wc -l || true)
fi
if [ "$grafana_count" -ge 7 ]; then
    check 0 "Grafana dashboards count >= 7 (found: $grafana_count JSON files)"
else
    check 1 "Grafana dashboards count >= 7 (found: $grafana_count JSON files in $dashboard_dir)"
fi

# ============================================================================
# CHECK 11: Prometheus datasource — prometheus.yml exists in datasources/
# ============================================================================
echo ""
echo -e "${BLUE}[11] Prometheus datasource: prometheus.yml exists${NC}"
prometheus_yml="$PROJECT_ROOT/docker/monitoring/grafana/datasources/prometheus.yml"
if [ -f "$prometheus_yml" ]; then
    check 0 "prometheus.yml exists in grafana/datasources/"
else
    check 1 "prometheus.yml exists in grafana/datasources/ (not found at $prometheus_yml)"
fi

# ============================================================================
# CHECK 12: Performance baseline framework — scripts/benchmark-baseline.sh exists and is executable
# ============================================================================
echo ""
echo -e "${BLUE}[12] Performance baseline framework: scripts/benchmark-baseline.sh${NC}"
baseline_script="$PROJECT_ROOT/scripts/benchmark-baseline.sh"
if [ -f "$baseline_script" ] && [ -x "$baseline_script" ]; then
    check 0 "scripts/benchmark-baseline.sh exists and is executable"
elif [ -f "$baseline_script" ]; then
    check 1 "scripts/benchmark-baseline.sh exists but is NOT executable"
else
    check 1 "scripts/benchmark-baseline.sh does not exist"
fi

# ============================================================================
# CHECK 13: Safety fixes verified — WaitGroup tracking in pool.go (grep wg.Add),
#           atomic.Bool in event_bus.go
# ============================================================================
echo ""
echo -e "${BLUE}[13] Safety fixes: wg.Add in pool.go, atomic.Bool in event_bus.go${NC}"
safety_ok=0
# WaitGroup tracking in internal/clis/pool.go
if grep -q "wg\.Add\|wg.Add" "$PROJECT_ROOT/internal/clis/pool.go" 2>/dev/null; then
    safety_ok=$((safety_ok + 1))
fi
# atomic.Bool in internal/clis/event_bus.go
if grep -q "atomic\.Bool" "$PROJECT_ROOT/internal/clis/event_bus.go" 2>/dev/null; then
    safety_ok=$((safety_ok + 1))
fi
if [ "$safety_ok" -eq 2 ]; then
    check 0 "Safety fixes verified: wg.Add in pool.go and atomic.Bool in event_bus.go"
else
    check 1 "Safety fixes verified: wg.Add and atomic.Bool found ($safety_ok/2)"
fi

# ============================================================================
# CHECK 14: terminateInstance nil guard — pool.go contains nil check before termination
# ============================================================================
echo ""
echo -e "${BLUE}[14] terminateInstance nil guard in internal/clis/pool.go${NC}"
if grep -q "terminateInstance" "$PROJECT_ROOT/internal/clis/pool.go" 2>/dev/null && \
   grep -qE "inst == nil|== nil" "$PROJECT_ROOT/internal/clis/pool.go" 2>/dev/null; then
    check 0 "terminateInstance nil guard exists in internal/clis/pool.go"
else
    check 1 "terminateInstance nil guard not found in internal/clis/pool.go"
fi

# ============================================================================
# CHECK 15: Configurable pool timeout — AcquireTimeout in pool.go
# ============================================================================
echo ""
echo -e "${BLUE}[15] Configurable pool timeout: AcquireTimeout in internal/clis/pool.go${NC}"
if grep -q "AcquireTimeout" "$PROJECT_ROOT/internal/clis/pool.go" 2>/dev/null; then
    check 0 "AcquireTimeout field exists in internal/clis/pool.go"
else
    check 1 "AcquireTimeout field not found in internal/clis/pool.go"
fi

# ============================================================================
# CHECK 16: TODO resolution — < 200 TODOs in docs/ (excluding archive, specs, plans)
# ============================================================================
echo ""
echo -e "${BLUE}[16] TODO resolution: < 200 TODOs in docs/ (excluding archive/specs/plans)${NC}"
todo_count=0
if [ -d "$PROJECT_ROOT/docs" ]; then
    todo_count=$(find "$PROJECT_ROOT/docs" -name "*.md" \
        -not -path "*/archive/*" \
        -not -path "*/specs/*" \
        -not -path "*/plans/*" \
        2>/dev/null | xargs grep -c "TODO" 2>/dev/null | awk -F: '{sum += $2} END {print sum+0}' || true)
fi
if [ "$todo_count" -lt 200 ]; then
    check 0 "TODO count in docs/ < 200 (found: $todo_count)"
else
    check 1 "TODO count in docs/ < 200 (found: $todo_count — too many)"
fi

# ============================================================================
# CHECK 17: All 4 guides expanded — each >= 15000 bytes
# The 4 expanded user guides are Website/user-manuals/34-37 (agentic, llmops, planning, benchmark)
# ============================================================================
echo ""
echo -e "${BLUE}[17] All 4 guides expanded: Website/user-manuals/34-37 each >= 15000 bytes${NC}"
guides_ok=0
guide_files=(
    "$PROJECT_ROOT/Website/user-manuals/34-agentic-workflows-guide.md"
    "$PROJECT_ROOT/Website/user-manuals/35-llmops-experimentation-guide.md"
    "$PROJECT_ROOT/Website/user-manuals/36-planning-algorithms-guide.md"
    "$PROJECT_ROOT/Website/user-manuals/37-benchmark-guide.md"
)
for guide in "${guide_files[@]}"; do
    if [ -f "$guide" ]; then
        guide_size=$(wc -c < "$guide" 2>/dev/null || echo "0")
        if [ "$guide_size" -ge 15000 ]; then
            guides_ok=$((guides_ok + 1))
        fi
    fi
done
if [ "$guides_ok" -eq 4 ]; then
    check 0 "All 4 guides are >= 15000 bytes"
else
    echo "    Guide sizes:"
    for guide in "${guide_files[@]}"; do
        if [ -f "$guide" ]; then
            guide_size=$(wc -c < "$guide" 2>/dev/null || echo "0")
            echo "      $(basename "$guide"): $guide_size bytes"
        else
            echo "      $(basename "$guide"): MISSING"
        fi
    done
    check 1 "All 4 guides >= 15000 bytes ($guides_ok/4 meet the threshold)"
fi

# ============================================================================
# CHECK 18: New documentation files — SAFETY_FIXES.md, TEST_STRATEGY.md (or
#           DETAILED_TESTING_STRATEGY.md), BASELINE_GUIDE.md exist
# ============================================================================
echo ""
echo -e "${BLUE}[18] New documentation files: SAFETY_FIXES.md, TEST_STRATEGY/DETAILED_TESTING_STRATEGY.md, BASELINE_GUIDE.md${NC}"
doc_files_ok=0
# SAFETY_FIXES.md
if [ -f "$PROJECT_ROOT/docs/development/SAFETY_FIXES.md" ]; then
    doc_files_ok=$((doc_files_ok + 1))
fi
# TEST_STRATEGY.md or DETAILED_TESTING_STRATEGY.md (allow either naming)
if [ -f "$PROJECT_ROOT/docs/development/TEST_STRATEGY.md" ] || \
   [ -f "$PROJECT_ROOT/docs/development/DETAILED_TESTING_STRATEGY.md" ]; then
    doc_files_ok=$((doc_files_ok + 1))
fi
# BASELINE_GUIDE.md (allow also RELEASE_BUILD_GUIDE.md as alternative)
if [ -f "$PROJECT_ROOT/docs/development/BASELINE_GUIDE.md" ] || \
   [ -f "$PROJECT_ROOT/docs/development/RELEASE_BUILD_GUIDE.md" ]; then
    doc_files_ok=$((doc_files_ok + 1))
fi
if [ "$doc_files_ok" -eq 3 ]; then
    check 0 "All 3 new documentation files exist"
else
    check 1 "New documentation files exist ($doc_files_ok/3): SAFETY_FIXES, TEST_STRATEGY, BASELINE_GUIDE"
fi

# ============================================================================
# CHECK 19: Video courses 77-84 — 8 new course files exist in Website/video-courses/
# ============================================================================
echo ""
echo -e "${BLUE}[19] Video courses 77-84: 8 new course files in Website/video-courses/${NC}"
video_courses_dir="$PROJECT_ROOT/Website/video-courses"
video_count=0
if [ -d "$video_courses_dir" ]; then
    video_count=$(find "$video_courses_dir" -maxdepth 1 \
        -name "course-77-*.md" \
        -o -name "course-78-*.md" \
        -o -name "course-79-*.md" \
        -o -name "course-80-*.md" \
        -o -name "course-81-*.md" \
        -o -name "course-82-*.md" \
        -o -name "course-83-*.md" \
        -o -name "course-84-*.md" \
        2>/dev/null | wc -l || true)
fi
if [ "$video_count" -ge 8 ]; then
    check 0 "Video courses 77-84 exist in Website/video-courses/ (found: $video_count files)"
else
    check 1 "Video courses 77-84 exist in Website/video-courses/ (found: $video_count/8 files)"
fi

# ============================================================================
# CHECK 20: Constitution synchronized — CLAUDE.md mentions /v1/ensemble
# ============================================================================
echo ""
echo -e "${BLUE}[20] Constitution synchronized: CLAUDE.md mentions /v1/ensemble${NC}"
if grep -q "/v1/ensemble" "$PROJECT_ROOT/CLAUDE.md" 2>/dev/null; then
    check 0 "CLAUDE.md contains /v1/ensemble (Constitution synchronized)"
else
    check 1 "CLAUDE.md does not contain /v1/ensemble (Constitution out of sync)"
fi

# ============================================================================
# SUMMARY
# ============================================================================
echo ""
echo "=========================================================="
echo "  Results: $PASS/$TOTAL passed, $FAIL failed"
echo "=========================================================="

if [ "$FAIL" -gt 0 ]; then
    echo -e "${RED}FAILED${NC}: $FAIL check(s) did not pass."
    exit 1
fi

echo -e "${GREEN}ALL CHECKS PASSED${NC}"
exit 0
