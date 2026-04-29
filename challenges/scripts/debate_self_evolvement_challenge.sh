#!/bin/bash
# Debate Self-Evolvement Challenge
# Validates the self-evolvement pre-debate validation phase:
# SelfEvolvementPhase type, self-test generation, iterative refinement.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-self-evolvement" "Debate Self-Evolvement Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Self-Evolvement Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: self_evolvement.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/protocol/self_evolvement.go" ]; then
    record_assertion "self_evolve_file" "exists" "true" "self_evolvement.go exists"
else
    record_assertion "self_evolve_file" "exists" "false" "self_evolvement.go NOT found"
fi

log_info "Test 2: Protocol package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/debate/protocol/... 2>&1); then
    record_assertion "protocol_compile" "true" "true" "Protocol package compiles"
else
    record_assertion "protocol_compile" "true" "false" "Protocol package failed to compile"
fi

# --- Section 2: Core types ---

log_info "Test 3: SelfEvolvementPhase type exists"
if grep -q "type SelfEvolvementPhase struct" "$PROJECT_ROOT/internal/debate/protocol/self_evolvement.go" 2>/dev/null; then
    record_assertion "self_evolve_phase" "true" "true" "SelfEvolvementPhase type found"
else
    record_assertion "self_evolve_phase" "true" "false" "SelfEvolvementPhase type NOT found"
fi

log_info "Test 4: SelfEvolvementConfig type exists"
if grep -q "type SelfEvolvementConfig struct" "$PROJECT_ROOT/internal/debate/protocol/self_evolvement.go" 2>/dev/null; then
    record_assertion "self_evolve_config" "true" "true" "SelfEvolvementConfig type found"
else
    record_assertion "self_evolve_config" "true" "false" "SelfEvolvementConfig type NOT found"
fi

log_info "Test 5: SelfTestResult type exists"
if grep -q "type SelfTestResult struct" "$PROJECT_ROOT/internal/debate/protocol/self_evolvement.go" 2>/dev/null; then
    record_assertion "self_test_result" "true" "true" "SelfTestResult type found"
else
    record_assertion "self_test_result" "true" "false" "SelfTestResult type NOT found"
fi

log_info "Test 6: NewSelfEvolvementPhase constructor exists"
if grep -q "func NewSelfEvolvementPhase" "$PROJECT_ROOT/internal/debate/protocol/self_evolvement.go" 2>/dev/null; then
    record_assertion "self_evolve_constructor" "true" "true" "NewSelfEvolvementPhase constructor found"
else
    record_assertion "self_evolve_constructor" "true" "false" "NewSelfEvolvementPhase constructor NOT found"
fi

log_info "Test 7: Execute method exists on SelfEvolvementPhase"
if grep -q "func (s \*SelfEvolvementPhase) Execute" "$PROJECT_ROOT/internal/debate/protocol/self_evolvement.go" 2>/dev/null; then
    record_assertion "self_evolve_execute" "true" "true" "Execute method found"
else
    record_assertion "self_evolve_execute" "true" "false" "Execute method NOT found"
fi

log_info "Test 8: PhaseSelfEvolvement defined in topology"
if grep -q 'PhaseSelfEvolvement.*DebatePhase.*=.*"self_evolvement"' "$PROJECT_ROOT/internal/debate/topology/topology.go" 2>/dev/null; then
    record_assertion "phase_self_evolve_defined" "true" "true" "PhaseSelfEvolvement defined"
else
    record_assertion "phase_self_evolve_defined" "true" "false" "PhaseSelfEvolvement NOT defined"
fi

# --- Section 3: Tests ---

log_info "Test 9: self_evolvement_test.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/protocol/self_evolvement_test.go" ]; then
    record_assertion "self_evolve_test_file" "exists" "true" "Test file found"
else
    record_assertion "self_evolve_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 10: Self-evolvement tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/protocol/ -run "TestSelfEvolve|TestSelfTest|TestNewSelfEvolve" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "self_evolve_tests_pass" "pass" "true" "Self-evolvement tests passed"
else
    record_assertion "self_evolve_tests_pass" "pass" "false" "Self-evolvement tests failed"
fi

# --- Finalize ---

FAILURES=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)
if [ "$FAILURES" -eq 0 ]; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
