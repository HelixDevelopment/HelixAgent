#!/bin/bash
# Debate Provenance Audit Challenge
# Validates the audit trail and provenance tracking:
# ProvenanceTracker, AuditEntry, AuditTrail, 14 event types, handler endpoint.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-provenance-audit" "Debate Provenance Audit Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Provenance Audit Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: provenance.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/audit/provenance.go" ]; then
    record_assertion "provenance_file" "exists" "true" "provenance.go exists"
else
    record_assertion "provenance_file" "exists" "false" "provenance.go NOT found"
fi

log_info "Test 2: Audit package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/debate/audit/... 2>&1); then
    record_assertion "audit_compile" "true" "true" "Audit package compiles"
else
    record_assertion "audit_compile" "true" "false" "Audit package failed to compile"
fi

# --- Section 2: Core types ---

log_info "Test 3: ProvenanceTracker type exists"
if grep -q "type ProvenanceTracker struct" "$PROJECT_ROOT/internal/debate/audit/provenance.go" 2>/dev/null; then
    record_assertion "provenance_tracker_type" "true" "true" "ProvenanceTracker type found"
else
    record_assertion "provenance_tracker_type" "true" "false" "ProvenanceTracker type NOT found"
fi

log_info "Test 4: AuditEntry type exists"
if grep -q "type AuditEntry struct" "$PROJECT_ROOT/internal/debate/audit/provenance.go" 2>/dev/null; then
    record_assertion "audit_entry_type" "true" "true" "AuditEntry type found"
else
    record_assertion "audit_entry_type" "true" "false" "AuditEntry type NOT found"
fi

log_info "Test 5: AuditTrail type exists"
if grep -q "type AuditTrail struct" "$PROJECT_ROOT/internal/debate/audit/provenance.go" 2>/dev/null; then
    record_assertion "audit_trail_type" "true" "true" "AuditTrail type found"
else
    record_assertion "audit_trail_type" "true" "false" "AuditTrail type NOT found"
fi

log_info "Test 6: EventType type defined"
if grep -q "type EventType string" "$PROJECT_ROOT/internal/debate/audit/provenance.go" 2>/dev/null; then
    record_assertion "event_type_defined" "true" "true" "EventType type found"
else
    record_assertion "event_type_defined" "true" "false" "EventType type NOT found"
fi

# --- Section 3: 14 event types ---

log_info "Test 7: 14 event types defined"
EVENT_COUNT=$(grep -c 'Event[A-Z].*EventType.*=' "$PROJECT_ROOT/internal/debate/audit/provenance.go" 2>/dev/null || echo "0")
if [ "$EVENT_COUNT" -ge 14 ]; then
    record_assertion "event_type_count" "true" "true" "Found $EVENT_COUNT event types (need 14+)"
else
    record_assertion "event_type_count" "true" "false" "Only $EVENT_COUNT event types (need 14)"
fi

log_info "Test 8: Core event types present"
CORE_EVENTS=0
for evt in "prompt_sent" "response_received" "vote_cast" "phase_started" "debate_started" "error_occurred"; do
    if grep -q "\"$evt\"" "$PROJECT_ROOT/internal/debate/audit/provenance.go" 2>/dev/null; then
        CORE_EVENTS=$((CORE_EVENTS + 1))
    fi
done
if [ "$CORE_EVENTS" -ge 6 ]; then
    record_assertion "core_events_present" "true" "true" "Found $CORE_EVENTS/6 core event types"
else
    record_assertion "core_events_present" "true" "false" "Only $CORE_EVENTS/6 core event types"
fi

# --- Section 4: Handler endpoint ---

log_info "Test 9: Handler has audit endpoint"
if grep -q '/:id/audit' "$PROJECT_ROOT/internal/handlers/debate_handler.go" 2>/dev/null; then
    record_assertion "audit_endpoint" "true" "true" "audit endpoint found in handler"
else
    record_assertion "audit_endpoint" "true" "false" "audit endpoint NOT found in handler"
fi

log_info "Test 10: GetDebateAudit handler method exists"
if grep -q "func.*GetDebateAudit" "$PROJECT_ROOT/internal/handlers/debate_handler.go" 2>/dev/null; then
    record_assertion "audit_handler_method" "true" "true" "GetDebateAudit handler found"
else
    record_assertion "audit_handler_method" "true" "false" "GetDebateAudit handler NOT found"
fi

# --- Section 5: Tests ---

log_info "Test 11: provenance_test.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/audit/provenance_test.go" ]; then
    record_assertion "provenance_test_file" "exists" "true" "Test file found"
else
    record_assertion "provenance_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 12: Provenance tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/audit/ 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "provenance_tests_pass" "pass" "true" "Provenance tests passed"
else
    record_assertion "provenance_tests_pass" "pass" "false" "Provenance tests failed"
fi

# --- Finalize ---

if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
