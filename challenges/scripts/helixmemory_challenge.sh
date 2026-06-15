#!/bin/bash
# HelixMemory Unified Cognitive Memory Engine Challenge
# Validates the HelixMemory module: code structure, compilation, tests,
# build tags, adapter integration, and all 12 power features.

set -e

# Source framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

init_challenge "helixmemory" "HelixMemory Unified Cognitive Memory Engine"
load_env

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

run_test() {
    local test_name="$1"
    local test_cmd="$2"
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    if eval "$test_cmd" >> "$LOG_FILE" 2>&1; then
        log_success "PASS: $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        record_assertion "test" "$test_name" "true" ""
        return 0
    else
        log_error "FAIL: $test_name"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        record_assertion "test" "$test_name" "false" "Test command failed"
        return 1
    fi
}

# ============================================================================
# SECTION 1: SUBMODULE AND MODULE STRUCTURE
# ============================================================================
log_info "Section 1: Submodule and Module Structure"

run_test "HelixMemory submodule directory exists" \
    "test -d '$PROJECT_ROOT/HelixMemory'"

run_test "HelixMemory go.mod exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/go.mod'"

run_test "HelixMemory module name correct" \
    "grep -q 'module digital.vasic.helixmemory' '$PROJECT_ROOT/HelixMemory/go.mod'"

run_test "HelixMemory depends on digital.vasic.memory" \
    "grep -q 'digital.vasic.memory' '$PROJECT_ROOT/HelixMemory/go.mod'"

run_test "Main go.mod has helixmemory require" \
    "grep -q 'digital.vasic.helixmemory' '$PROJECT_ROOT/go.mod'"

run_test "Main go.mod has helixmemory replace directive" \
    "grep -q 'replace digital.vasic.helixmemory => ./HelixMemory' '$PROJECT_ROOT/go.mod'"

run_test "CLAUDE.md exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/CLAUDE.md'"

run_test "AGENTS.md exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/AGENTS.md'"

run_test "README.md exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/README.md'"

run_test "Docker compose exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/docker/docker-compose.yml'"

# ============================================================================
# SECTION 2: PACKAGE STRUCTURE (ALL REQUIRED PACKAGES)
# ============================================================================
log_info "Section 2: Package Structure"

run_test "pkg/types package exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/types/types.go'"

run_test "pkg/config package exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/config/config.go'"

run_test "pkg/fusion engine exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/fusion/engine.go'"

run_test "pkg/routing router exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/routing/router.go'"

run_test "pkg/provider unified exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/provider/unified.go'"

run_test "pkg/provider adapter exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/provider/adapter.go'"

run_test "pkg/consolidation exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/consolidation/consolidation.go'"

run_test "pkg/metrics exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/metrics/metrics.go'"

# ============================================================================
# SECTION 3: ALL FOUR MEMORY BACKENDS
# ============================================================================
log_info "Section 3: Memory Backend Clients"

run_test "Mem0 client exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/clients/mem0/client.go'"

run_test "Cognee client exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/clients/cognee/client.go'"

run_test "Letta client exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/clients/letta/client.go'"

run_test "Graphiti client exists" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/clients/graphiti/client.go'"

run_test "Mem0 client implements MemoryProvider" \
    "grep -q 'func.*Client.*Name().*MemorySource' '$PROJECT_ROOT/HelixMemory/pkg/clients/mem0/client.go'"

run_test "Cognee client implements MemoryProvider" \
    "grep -q 'func.*Client.*Name().*MemorySource' '$PROJECT_ROOT/HelixMemory/pkg/clients/cognee/client.go'"

run_test "Letta client implements MemoryProvider" \
    "grep -q 'func.*Client.*Name().*MemorySource' '$PROJECT_ROOT/HelixMemory/pkg/clients/letta/client.go'"

run_test "Graphiti client implements TemporalProvider" \
    "grep -q 'func.*Client.*SearchTemporal' '$PROJECT_ROOT/HelixMemory/pkg/clients/graphiti/client.go'"

# ============================================================================
# SECTION 4: ALL 12 POWER FEATURES
# ============================================================================
log_info "Section 4: Power Features"

FEATURES=(
    "codebase_dna/dna.go"
    "procedural/procedural.go"
    "mesh/mesh.go"
    "temporal/temporal.go"
    "debate_memory/debate.go"
    "context_window/context.go"
    "cross_project/transfer.go"
    "mcp_bridge/bridge.go"
    "code_gen/codegen.go"
    "confidence/scoring.go"
    "quality_loop/quality.go"
    "snapshots/snapshots.go"
)

for feature in "${FEATURES[@]}"; do
    name=$(echo "$feature" | cut -d'/' -f1)
    run_test "Power feature: $name" \
        "test -f '$PROJECT_ROOT/HelixMemory/pkg/features/$feature'"
done

# ============================================================================
# SECTION 5: TEST COVERAGE (ALL PACKAGES HAVE TESTS)
# ============================================================================
log_info "Section 5: Test Coverage"

run_test "Mem0 client tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/clients/mem0/client_test.go'"

run_test "Cognee client tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/clients/cognee/client_test.go'"

run_test "Letta client tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/clients/letta/client_test.go'"

run_test "Graphiti client tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/clients/graphiti/client_test.go'"

run_test "Fusion engine tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/fusion/engine_test.go'"

run_test "Routing tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/routing/router_test.go'"

run_test "Provider tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/provider/unified_test.go'"

run_test "Adapter tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/provider/adapter_test.go'"

run_test "Consolidation tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/consolidation/consolidation_test.go'"

run_test "Metrics tests exist" \
    "test -f '$PROJECT_ROOT/HelixMemory/pkg/metrics/metrics_test.go'"

# Check all power features have tests
for feature in "${FEATURES[@]}"; do
    name=$(echo "$feature" | cut -d'/' -f1)
    test_file="${feature%.go}_test.go"
    run_test "Test: $name" \
        "test -f '$PROJECT_ROOT/HelixMemory/pkg/features/$test_file'"
done

# ============================================================================
# SECTION 6: COMPILATION
# ============================================================================
log_info "Section 6: Compilation"

BUILD_START=$(date +%s%N)
run_test "HelixMemory builds (go build ./...)" \
    "cd '$PROJECT_ROOT/HelixMemory' && go build ./..."
BUILD_END=$(date +%s%N)
BUILD_DURATION_MS=$(( (BUILD_END - BUILD_START) / 1000000 ))
record_metric "helixmemory_build_time_ms" "$BUILD_DURATION_MS"

run_test "Main project builds (default = HelixMemory)" \
    "cd '$PROJECT_ROOT' && go build ./cmd/... ./internal/..."

run_test "Main project builds (opt-out nohelixmemory tag)" \
    "cd '$PROJECT_ROOT' && go build -tags nohelixmemory ./cmd/... ./internal/..."

# ============================================================================
# SECTION 7: UNIT TESTS
# ============================================================================
log_info "Section 7: Unit Test Execution"

TEST_START=$(date +%s%N)
# Use -v to get individual test pass/fail lines for accurate counting
TEST_OUTPUT=$(cd "$PROJECT_ROOT/HelixMemory" && GOMAXPROCS=2 go test -count=1 -race -p 1 -v ./... 2>&1) || true
TEST_END=$(date +%s%N)
TEST_DURATION_MS=$(( (TEST_END - TEST_START) / 1000000 ))
record_metric "helixmemory_test_time_ms" "$TEST_DURATION_MS"

# Count packages that passed
PASSED_PKGS=$(echo "$TEST_OUTPUT" | grep -c "^ok" || true)
FAILED_PKGS=$(echo "$TEST_OUTPUT" | grep -c "^FAIL" || true)
NO_TEST_PKGS=$(echo "$TEST_OUTPUT" | grep -c "no test files" || true)

record_metric "passed_packages" "$PASSED_PKGS"
record_metric "failed_packages" "$FAILED_PKGS"
record_metric "no_test_packages" "$NO_TEST_PKGS"

run_test "All HelixMemory tests pass ($PASSED_PKGS packages)" \
    "test '$FAILED_PKGS' -eq 0"

# Count individual tests from verbose output
TOTAL_TESTS=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS" || true)
record_metric "total_tests" "$TOTAL_TESTS"

run_test "Minimum 250 unit tests exist (found: $TOTAL_TESTS)" \
    "test '$TOTAL_TESTS' -ge 250"

# ============================================================================
# SECTION 8: BUILD TAG CONDITIONAL SELECTION
# ============================================================================
log_info "Section 8: Build Tag Conditional Selection"

run_test "factory_standard.go exists" \
    "test -f '$PROJECT_ROOT/internal/adapters/memory/factory_standard.go'"

run_test "factory_helixmemory.go exists" \
    "test -f '$PROJECT_ROOT/internal/adapters/memory/factory_helixmemory.go'"

run_test "factory_standard.go has correct build tag (opt-out)" \
    "grep -q '//go:build nohelixmemory' '$PROJECT_ROOT/internal/adapters/memory/factory_standard.go'"

run_test "factory_helixmemory.go has correct build tag (default)" \
    "grep -q '//go:build !nohelixmemory' '$PROJECT_ROOT/internal/adapters/memory/factory_helixmemory.go'"

run_test "factory_helixmemory.go imports HelixMemory" \
    "grep -q 'digital.vasic.helixmemory' '$PROJECT_ROOT/internal/adapters/memory/factory_helixmemory.go'"

run_test "Adapter tests pass (default = HelixMemory)" \
    "cd '$PROJECT_ROOT' && GOMAXPROCS=2 go test -count=1 -race -p 1 ./internal/adapters/memory/"

run_test "Adapter tests pass (opt-out nohelixmemory tag)" \
    "cd '$PROJECT_ROOT' && GOMAXPROCS=2 go test -count=1 -race -p 1 -tags nohelixmemory ./internal/adapters/memory/"

run_test "HelixMemory is DEFAULT (IsHelixMemoryEnabled=true)" \
    "cd '$PROJECT_ROOT' && go test -count=1 -run TestHelixMemoryIsDefault ./internal/adapters/memory/"

run_test "Debate service imports HelixMemory adapter" \
    "grep -q 'memoryadapter.*internal/adapters/memory' '$PROJECT_ROOT/internal/services/debate_service.go'"

run_test "Debate service has memoryAdapter field + SetMemoryAdapter setter" \
    "grep -qE 'memoryAdapter[[:space:]]+interface' '$PROJECT_ROOT/internal/services/debate_service.go' && grep -qE 'SetMemoryAdapter\\(adapter \\*memoryadapter\\.StoreAdapter\\)' '$PROJECT_ROOT/internal/services/debate_service.go'"

run_test "Debate service has IsHelixMemoryActive method" \
    "grep -q 'func.*DebateService.*IsHelixMemoryActive' '$PROJECT_ROOT/internal/services/debate_service.go'"

run_test "Debate service has analyzeWithHelixMemory method" \
    "grep -q 'func.*DebateService.*analyzeWithHelixMemory' '$PROJECT_ROOT/internal/services/debate_service.go'"

# ============================================================================
# SECTION 9: FUSION ENGINE VALIDATION
# ============================================================================
log_info "Section 9: Fusion Engine Validation"

run_test "Fusion dedup threshold defined (0.92)" \
    "grep -q '0.92' '$PROJECT_ROOT/HelixMemory/pkg/config/config.go'"

run_test "Fusion uses 4 scoring weights" \
    "grep -cE 'Weight.*float64' '$PROJECT_ROOT/HelixMemory/pkg/config/config.go' | grep -q '[4-9]'"

run_test "Fusion engine has Fuse method" \
    "grep -q 'func.*Engine.*Fuse' '$PROJECT_ROOT/HelixMemory/pkg/fusion/engine.go'"

run_test "Router has RouteWrite and RouteRead" \
    "grep -q 'func.*Router.*RouteWrite' '$PROJECT_ROOT/HelixMemory/pkg/routing/router.go' && grep -q 'func.*Router.*RouteRead' '$PROJECT_ROOT/HelixMemory/pkg/routing/router.go'"

# ============================================================================
# SECTION 10: INTERFACES AND CONTRACTS
# ============================================================================
log_info "Section 10: Interfaces and Contracts"

run_test "MemoryProvider interface defined" \
    "grep -q 'type MemoryProvider interface' '$PROJECT_ROOT/HelixMemory/pkg/types/types.go'"

run_test "CoreMemoryProvider interface defined" \
    "grep -q 'type CoreMemoryProvider interface' '$PROJECT_ROOT/HelixMemory/pkg/types/types.go'"

run_test "TemporalProvider interface defined" \
    "grep -q 'type TemporalProvider interface' '$PROJECT_ROOT/HelixMemory/pkg/types/types.go'"

run_test "ConsolidationProvider interface defined" \
    "grep -q 'type ConsolidationProvider interface' '$PROJECT_ROOT/HelixMemory/pkg/types/types.go'"

run_test "MemoryStoreAdapter implements MemoryStore" \
    "grep -q 'func.*MemoryStoreAdapter.*Add' '$PROJECT_ROOT/HelixMemory/pkg/provider/adapter.go' && grep -q 'func.*MemoryStoreAdapter.*Search' '$PROJECT_ROOT/HelixMemory/pkg/provider/adapter.go'"

# ============================================================================
# SECTION 11: CONFIGURATION
# ============================================================================
log_info "Section 11: Configuration"

run_test "Config reads HELIX_MEMORY_LETTA_ENDPOINT" \
    "grep -q 'HELIX_MEMORY_LETTA_ENDPOINT' '$PROJECT_ROOT/HelixMemory/pkg/config/config.go'"

run_test "Config reads HELIX_MEMORY_MEM0_ENDPOINT" \
    "grep -q 'HELIX_MEMORY_MEM0_ENDPOINT' '$PROJECT_ROOT/HelixMemory/pkg/config/config.go'"

run_test "Config reads HELIX_MEMORY_COGNEE_ENDPOINT" \
    "grep -q 'HELIX_MEMORY_COGNEE_ENDPOINT' '$PROJECT_ROOT/HelixMemory/pkg/config/config.go'"

run_test "Config reads HELIX_MEMORY_GRAPHITI_ENDPOINT" \
    "grep -q 'HELIX_MEMORY_GRAPHITI_ENDPOINT' '$PROJECT_ROOT/HelixMemory/pkg/config/config.go'"

run_test "Config has circuit breaker settings" \
    "grep -q 'CircuitBreakerThreshold' '$PROJECT_ROOT/HelixMemory/pkg/config/config.go'"

# ============================================================================
# SUMMARY
# ============================================================================
log_info "Challenge complete"

record_metric "total_tests" "$TESTS_TOTAL"
record_metric "passed_tests" "$TESTS_PASSED"
record_metric "failed_tests" "$TESTS_FAILED"

log_info "Results: $TESTS_PASSED/$TESTS_TOTAL passed, $TESTS_FAILED failed"

if [[ $TESTS_FAILED -eq 0 ]]; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
