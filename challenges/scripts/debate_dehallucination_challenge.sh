#!/bin/bash
# Debate Dehallucination Challenge
# Validates the Communicative Dehallucination pre-debate phase:
# DehallucationPhase type, clarification protocol, 8-phase configuration.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-dehallucination" "Debate Dehallucination Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Dehallucination Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: dehallucination.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/protocol/dehallucination.go" ]; then
    record_assertion "dehallu_file" "exists" "true" "dehallucination.go exists"
else
    record_assertion "dehallu_file" "exists" "false" "dehallucination.go NOT found"
fi

log_info "Test 2: Protocol package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/debate/protocol/... 2>&1); then
    record_assertion "protocol_compile" "true" "true" "Protocol package compiles"
else
    record_assertion "protocol_compile" "true" "false" "Protocol package failed to compile"
fi

# --- Section 2: Core types ---

log_info "Test 3: DehallucationPhase type exists"
if grep -q "type DehallucationPhase struct" "$PROJECT_ROOT/internal/debate/protocol/dehallucination.go" 2>/dev/null; then
    record_assertion "dehallu_phase_type" "true" "true" "DehallucationPhase type found"
else
    record_assertion "dehallu_phase_type" "true" "false" "DehallucationPhase type NOT found"
fi

log_info "Test 4: DehallucationConfig type exists"
if grep -q "type DehallucationConfig struct" "$PROJECT_ROOT/internal/debate/protocol/dehallucination.go" 2>/dev/null; then
    record_assertion "dehallu_config_type" "true" "true" "DehallucationConfig type found"
else
    record_assertion "dehallu_config_type" "true" "false" "DehallucationConfig type NOT found"
fi

log_info "Test 5: ClarificationRequest type exists"
if grep -q "type ClarificationRequest struct" "$PROJECT_ROOT/internal/debate/protocol/dehallucination.go" 2>/dev/null; then
    record_assertion "clarification_req_type" "true" "true" "ClarificationRequest type found"
else
    record_assertion "clarification_req_type" "true" "false" "ClarificationRequest type NOT found"
fi

log_info "Test 6: NewDehallucationPhase constructor exists"
if grep -q "func NewDehallucationPhase" "$PROJECT_ROOT/internal/debate/protocol/dehallucination.go" 2>/dev/null; then
    record_assertion "dehallu_constructor" "true" "true" "NewDehallucationPhase constructor found"
else
    record_assertion "dehallu_constructor" "true" "false" "NewDehallucationPhase constructor NOT found"
fi

# --- Section 3: Protocol has 8 phases configured ---

log_info "Test 7: PhaseDehallucination defined in topology"
if grep -q 'PhaseDehallucination.*DebatePhase.*=.*"dehallucination"' "$PROJECT_ROOT/internal/debate/topology/topology.go" 2>/dev/null; then
    record_assertion "phase_dehallu_defined" "true" "true" "PhaseDehallucination defined"
else
    record_assertion "phase_dehallu_defined" "true" "false" "PhaseDehallucination NOT defined"
fi

log_info "Test 8: Protocol has 8 debate phases"
PHASE_COUNT=$(grep -c 'Phase[A-Z].*DebatePhase.*=' "$PROJECT_ROOT/internal/debate/topology/topology.go" 2>/dev/null || echo "0")
if [ "$PHASE_COUNT" -ge 8 ]; then
    record_assertion "eight_phases" "true" "true" "Found $PHASE_COUNT phases (need 8+)"
else
    record_assertion "eight_phases" "true" "false" "Only $PHASE_COUNT phases found (need 8)"
fi

# --- Section 4: Tests ---

log_info "Test 9: dehallucination_test.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/protocol/dehallucination_test.go" ]; then
    record_assertion "dehallu_test_file" "exists" "true" "Test file found"
else
    record_assertion "dehallu_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 10: Dehallucination tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/protocol/ -run "TestDehallu|TestClarif|TestNewDehallu" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "dehallu_tests_pass" "pass" "true" "Dehallucination tests passed"
else
    record_assertion "dehallu_tests_pass" "pass" "false" "Dehallucination tests failed"
fi

# --- Finalize ---

if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
