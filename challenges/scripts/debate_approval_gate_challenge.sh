#!/bin/bash
# Debate Approval Gate Challenge
# Validates the approval gate mechanism for debate phases:
# ApprovalGate type, CheckGate/Approve/Reject methods, handler endpoints.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-approval-gate" "Debate Approval Gate Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Approval Gate Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: approval_gate.go exists"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/gates/approval_gate.go" ]; then
    record_assertion "gate_file" "exists" "true" "approval_gate.go exists"
else
    record_assertion "gate_file" "exists" "false" "approval_gate.go NOT found"
fi

log_info "Test 2: Gates package compiles"
if (cd "$PROJECT_ROOT/DebateOrchestrator" && go build ./gates/... 2>&1); then
    record_assertion "gates_compile" "true" "true" "Gates package compiles"
else
    record_assertion "gates_compile" "true" "false" "Gates package failed to compile"
fi

# --- Section 2: Core types and methods ---

log_info "Test 3: ApprovalGate type exists"
if grep -q "type ApprovalGate struct" "$PROJECT_ROOT/DebateOrchestrator/gates/approval_gate.go" 2>/dev/null; then
    record_assertion "approval_gate_type" "true" "true" "ApprovalGate type found"
else
    record_assertion "approval_gate_type" "true" "false" "ApprovalGate type NOT found"
fi

log_info "Test 4: CheckGate method exists"
if grep -q "func (g \*ApprovalGate) CheckGate" "$PROJECT_ROOT/DebateOrchestrator/gates/approval_gate.go" 2>/dev/null; then
    record_assertion "check_gate_method" "true" "true" "CheckGate method found"
else
    record_assertion "check_gate_method" "true" "false" "CheckGate method NOT found"
fi

log_info "Test 5: Approve method exists"
if grep -q "func (g \*ApprovalGate) Approve" "$PROJECT_ROOT/DebateOrchestrator/gates/approval_gate.go" 2>/dev/null; then
    record_assertion "approve_method" "true" "true" "Approve method found"
else
    record_assertion "approve_method" "true" "false" "Approve method NOT found"
fi

log_info "Test 6: Reject method exists"
if grep -q "func (g \*ApprovalGate) Reject" "$PROJECT_ROOT/DebateOrchestrator/gates/approval_gate.go" 2>/dev/null; then
    record_assertion "reject_method" "true" "true" "Reject method found"
else
    record_assertion "reject_method" "true" "false" "Reject method NOT found"
fi

log_info "Test 7: GateConfig type exists"
if grep -q "type GateConfig struct" "$PROJECT_ROOT/DebateOrchestrator/gates/approval_gate.go" 2>/dev/null; then
    record_assertion "gate_config_type" "true" "true" "GateConfig type found"
else
    record_assertion "gate_config_type" "true" "false" "GateConfig type NOT found"
fi

# --- Section 3: Handler endpoints ---

log_info "Test 8: Handler has approve endpoint"
if grep -q '/:id/approve' "$PROJECT_ROOT/internal/handlers/debate_handler.go" 2>/dev/null; then
    record_assertion "approve_endpoint" "true" "true" "approve endpoint found in handler"
else
    record_assertion "approve_endpoint" "true" "false" "approve endpoint NOT found in handler"
fi

log_info "Test 9: Handler has reject endpoint"
if grep -q '/:id/reject' "$PROJECT_ROOT/internal/handlers/debate_handler.go" 2>/dev/null; then
    record_assertion "reject_endpoint" "true" "true" "reject endpoint found in handler"
else
    record_assertion "reject_endpoint" "true" "false" "reject endpoint NOT found in handler"
fi

log_info "Test 10: Handler has gates endpoint"
if grep -q '/:id/gates' "$PROJECT_ROOT/internal/handlers/debate_handler.go" 2>/dev/null; then
    record_assertion "gates_endpoint" "true" "true" "gates endpoint found in handler"
else
    record_assertion "gates_endpoint" "true" "false" "gates endpoint NOT found in handler"
fi

# --- Section 4: Tests ---

log_info "Test 11: approval_gate_test.go exists"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/gates/approval_gate_test.go" ]; then
    record_assertion "gate_test_file" "exists" "true" "Test file found"
else
    record_assertion "gate_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 12: Approval gate tests pass"
if (cd "$PROJECT_ROOT/DebateOrchestrator" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./gates/ 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "gate_tests_pass" "pass" "true" "Approval gate tests passed"
else
    record_assertion "gate_tests_pass" "pass" "false" "Approval gate tests failed"
fi

# --- Finalize ---

if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
