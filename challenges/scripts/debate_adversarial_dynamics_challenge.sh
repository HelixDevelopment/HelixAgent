#!/bin/bash
# Debate Adversarial Dynamics Challenge
# Validates the Red/Blue Team adversarial protocol:
# attack reports, defense reports, protocol orchestration.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-adversarial-dynamics" "Debate Adversarial Dynamics Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Adversarial Dynamics Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: adversarial.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/agents/adversarial.go" ]; then
    record_assertion "adversarial_file" "exists" "true" "adversarial.go exists"
else
    record_assertion "adversarial_file" "exists" "false" "adversarial.go NOT found"
fi

log_info "Test 2: Adversarial package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/debate/agents/... 2>&1); then
    record_assertion "adversarial_compile" "true" "true" "Adversarial package compiles"
else
    record_assertion "adversarial_compile" "true" "false" "Adversarial package failed to compile"
fi

# --- Section 2: Core types ---

log_info "Test 3: AttackReport type exists"
if grep -q "type AttackReport struct" "$PROJECT_ROOT/internal/debate/agents/adversarial.go" 2>/dev/null; then
    record_assertion "attack_report_type" "true" "true" "AttackReport type found"
else
    record_assertion "attack_report_type" "true" "false" "AttackReport type NOT found"
fi

log_info "Test 4: DefenseReport type exists"
if grep -q "type DefenseReport struct" "$PROJECT_ROOT/internal/debate/agents/adversarial.go" 2>/dev/null; then
    record_assertion "defense_report_type" "true" "true" "DefenseReport type found"
else
    record_assertion "defense_report_type" "true" "false" "DefenseReport type NOT found"
fi

log_info "Test 5: AdversarialProtocol type exists"
if grep -q "type AdversarialProtocol struct" "$PROJECT_ROOT/internal/debate/agents/adversarial.go" 2>/dev/null; then
    record_assertion "adversarial_protocol_type" "true" "true" "AdversarialProtocol type found"
else
    record_assertion "adversarial_protocol_type" "true" "false" "AdversarialProtocol type NOT found"
fi

log_info "Test 6: Vulnerability type exists"
if grep -q "type Vulnerability struct" "$PROJECT_ROOT/internal/debate/agents/adversarial.go" 2>/dev/null; then
    record_assertion "vulnerability_type" "true" "true" "Vulnerability type found"
else
    record_assertion "vulnerability_type" "true" "false" "Vulnerability type NOT found"
fi

log_info "Test 7: NewAdversarialProtocol constructor exists"
if grep -q "func NewAdversarialProtocol" "$PROJECT_ROOT/internal/debate/agents/adversarial.go" 2>/dev/null; then
    record_assertion "adversarial_constructor" "true" "true" "NewAdversarialProtocol constructor found"
else
    record_assertion "adversarial_constructor" "true" "false" "NewAdversarialProtocol constructor NOT found"
fi

log_info "Test 8: Execute method exists on AdversarialProtocol"
if grep -q "func (ap \*AdversarialProtocol) Execute" "$PROJECT_ROOT/internal/debate/agents/adversarial.go" 2>/dev/null; then
    record_assertion "adversarial_execute" "true" "true" "Execute method found"
else
    record_assertion "adversarial_execute" "true" "false" "Execute method NOT found"
fi

# --- Section 3: Test file and test execution ---

log_info "Test 9: adversarial_test.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/agents/adversarial_test.go" ]; then
    record_assertion "adversarial_test_file" "exists" "true" "Test file found"
else
    record_assertion "adversarial_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 10: Adversarial tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/agents/ -run "TestAdversarial|TestAttack|TestDefense|TestRedBlue" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "adversarial_tests_pass" "pass" "true" "Adversarial tests passed"
else
    record_assertion "adversarial_tests_pass" "pass" "false" "Adversarial tests failed"
fi

# --- Finalize ---

FAILURES=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)
if [ "$FAILURES" -eq 0 ]; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
