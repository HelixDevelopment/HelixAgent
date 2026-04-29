#!/bin/bash
# Debate Benchmark Integration Challenge
# Validates the benchmark bridge connecting debate results to evaluation:
# BenchmarkBridge, EvaluationScore, DebateBenchmarkSuite types.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-benchmark-integration" "Debate Benchmark Integration Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Benchmark Integration Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: benchmark_bridge.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/evaluation/benchmark_bridge.go" ]; then
    record_assertion "bridge_file" "exists" "true" "benchmark_bridge.go exists"
else
    record_assertion "bridge_file" "exists" "false" "benchmark_bridge.go NOT found"
fi

log_info "Test 2: Evaluation package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/debate/evaluation/... 2>&1); then
    record_assertion "evaluation_compile" "true" "true" "Evaluation package compiles"
else
    record_assertion "evaluation_compile" "true" "false" "Evaluation package failed to compile"
fi

# --- Section 2: Core types ---

log_info "Test 3: BenchmarkBridge type exists"
if grep -q "type BenchmarkBridge struct" "$PROJECT_ROOT/internal/debate/evaluation/benchmark_bridge.go" 2>/dev/null; then
    record_assertion "benchmark_bridge_type" "true" "true" "BenchmarkBridge type found"
else
    record_assertion "benchmark_bridge_type" "true" "false" "BenchmarkBridge type NOT found"
fi

log_info "Test 4: EvaluationScore type exists"
if grep -q "type EvaluationScore struct" "$PROJECT_ROOT/internal/debate/evaluation/benchmark_bridge.go" 2>/dev/null; then
    record_assertion "evaluation_score_type" "true" "true" "EvaluationScore type found"
else
    record_assertion "evaluation_score_type" "true" "false" "EvaluationScore type NOT found"
fi

log_info "Test 5: DebateBenchmarkSuite type exists"
if grep -q "type DebateBenchmarkSuite struct" "$PROJECT_ROOT/internal/debate/evaluation/benchmark_bridge.go" 2>/dev/null; then
    record_assertion "benchmark_suite_type" "true" "true" "DebateBenchmarkSuite type found"
else
    record_assertion "benchmark_suite_type" "true" "false" "DebateBenchmarkSuite type NOT found"
fi

log_info "Test 6: NewBenchmarkBridge constructor exists"
if grep -q "func NewBenchmarkBridge" "$PROJECT_ROOT/internal/debate/evaluation/benchmark_bridge.go" 2>/dev/null; then
    record_assertion "bridge_constructor" "true" "true" "NewBenchmarkBridge constructor found"
else
    record_assertion "bridge_constructor" "true" "false" "NewBenchmarkBridge constructor NOT found"
fi

log_info "Test 7: EvaluateDebateResult method exists"
if grep -q "func (b \*BenchmarkBridge) EvaluateDebateResult" "$PROJECT_ROOT/internal/debate/evaluation/benchmark_bridge.go" 2>/dev/null; then
    record_assertion "evaluate_result_method" "true" "true" "EvaluateDebateResult method found"
else
    record_assertion "evaluate_result_method" "true" "false" "EvaluateDebateResult method NOT found"
fi

log_info "Test 8: BenchmarkProblem type exists"
if grep -q "type BenchmarkProblem struct" "$PROJECT_ROOT/internal/debate/evaluation/benchmark_bridge.go" 2>/dev/null; then
    record_assertion "benchmark_problem_type" "true" "true" "BenchmarkProblem type found"
else
    record_assertion "benchmark_problem_type" "true" "false" "BenchmarkProblem type NOT found"
fi

# --- Section 3: Tests ---

log_info "Test 9: benchmark_bridge_test.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/evaluation/benchmark_bridge_test.go" ]; then
    record_assertion "bridge_test_file" "exists" "true" "Test file found"
else
    record_assertion "bridge_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 10: Benchmark bridge tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/evaluation/ 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "bridge_tests_pass" "pass" "true" "Benchmark bridge tests passed"
else
    record_assertion "bridge_tests_pass" "pass" "false" "Benchmark bridge tests failed"
fi

# --- Finalize ---

FAILURES=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)
if [ "$FAILURES" -eq 0 ]; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
