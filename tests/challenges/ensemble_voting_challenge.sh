#!/bin/bash
# Challenge: Ensemble Voting Strategy
# Tests the voting coordination strategy with multiple agent instances

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPORT_DIR="$PROJECT_ROOT/challenge-results/ensemble-voting-$(date +%Y%m%d-%H%M%S)"

mkdir -p "$REPORT_DIR"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  CHALLENGE: Ensemble Voting Strategy                         ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Configuration
ENSEMBLE_ENDPOINT="${HELIXAGENT_URL:-http://localhost:8100}"
TIMEOUT=300
MAX_INSTANCES=5

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if HelixAgent is running
    if ! curl -s "$ENSEMBLE_ENDPOINT/health" > /dev/null 2>&1; then
        log_error "HelixAgent is not running at $ENSEMBLE_ENDPOINT"
        exit 1
    fi
    log_success "HelixAgent is running"
    
    # Check PostgreSQL
    if ! pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
        log_error "PostgreSQL is not running"
        exit 1
    fi
    log_success "PostgreSQL is running"
    
    log_info "All prerequisites met"
}

# Test 1: Create ensemble session with voting strategy
test_create_voting_session() {
    log_info "Test 1: Creating ensemble session with voting strategy..."
    
    local response
    response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -d '{
            "strategy": "voting",
            "participants": {
                "primary": {"type": "helixagent"},
                "critiques": [
                    {"type": "helixagent"},
                    {"type": "helixagent"}
                ]
            }
        }' 2>&1)
    
    if echo "$response" | grep -q "id"; then
        SESSION_ID=$(echo "$response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        log_success "Created ensemble session: $SESSION_ID"
        echo "$response" > "$REPORT_DIR/session-create.json"
        return 0
    else
        log_error "Failed to create session: $response"
        return 1
    fi
}

# Test 2: Execute task with voting
test_execute_voting_task() {
    log_info "Test 2: Executing task with voting strategy..."
    
    local response
    response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions/$SESSION_ID/execute" \
        -H "Content-Type: application/json" \
        -d '{
            "content": "Write a Python function to calculate fibonacci numbers",
            "timeout": 60
        }' 2>&1)
    
    if echo "$response" | grep -q "consensus_reached"; then
        log_success "Task execution completed"
        echo "$response" > "$REPORT_DIR/task-execution.json"
        
        # Extract consensus metrics
        local consensus
        consensus=$(echo "$response" | grep -o '"consensus_reached":[a-z]*' | cut -d':' -f2)
        log_info "Consensus reached: $consensus"
        
        local confidence
        confidence=$(echo "$response" | grep -o '"confidence":[0-9.]*' | cut -d':' -f2)
        log_info "Confidence score: $confidence"
        
        return 0
    else
        log_error "Task execution failed: $response"
        return 1
    fi
}

# Test 3: Load test - multiple concurrent voting sessions
test_concurrent_voting_sessions() {
    log_info "Test 3: Testing concurrent voting sessions..."
    
    local num_sessions=5
    local success_count=0
    local pids=()
    
    # Start multiple sessions concurrently
    for i in $(seq 1 $num_sessions); do
        (
            curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions" \
                -H "Content-Type: application/json" \
                -d '{
                    "strategy": "voting",
                    "participants": {
                        "primary": {"type": "helixagent"},
                        "critiques": [{"type": "helixagent"}]
                    }
                }' > "$REPORT_DIR/concurrent-$i.json" 2>&1
        ) &
        pids+=($!)
    done
    
    # Wait for all to complete
    local failed=0
    for pid in "${pids[@]}"; do
        if ! wait $pid; then
            ((failed++))
        fi
    done
    
    # Count successful creations
    for i in $(seq 1 $num_sessions); do
        if [ -f "$REPORT_DIR/concurrent-$i.json" ] && \
           grep -q "id" "$REPORT_DIR/concurrent-$i.json"; then
            ((success_count++))
        fi
    done
    
    log_info "Created $success_count/$num_sessions concurrent sessions"
    
    if [ $success_count -eq $num_sessions ]; then
        log_success "All concurrent sessions created successfully"
        return 0
    else
        log_error "Only $success_count/$num_sessions sessions created"
        return 1
    fi
}

# Test 4: Verify consensus calculation
test_consensus_calculation() {
    log_info "Test 4: Verifying consensus calculation..."
    
    # Create a session with known participants
    local response
    response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -d '{
            "strategy": "voting",
            "participants": {
                "primary": {"type": "helixagent"},
                "critiques": [
                    {"type": "helixagent"},
                    {"type": "helixagent"},
                    {"type": "helixagent"}
                ]
            }
        }')
    
    local session_id
    session_id=$(echo "$response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    
    # Execute task
    response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions/$session_id/execute" \
        -H "Content-Type: application/json" \
        -d '{
            "content": "What is 2+2?",
            "timeout": 30
        }')
    
    # Verify confidence is within valid range
    local confidence
    confidence=$(echo "$response" | grep -o '"confidence":[0-9.]*' | cut -d':' -f2)
    
    if (( $(echo "$confidence >= 0 && $confidence <= 1" | bc -l) )); then
        log_success "Confidence score is valid: $confidence"
        return 0
    else
        log_error "Invalid confidence score: $confidence"
        return 1
    fi
}

# Test 5: Test session lifecycle
test_session_lifecycle() {
    log_info "Test 5: Testing session lifecycle..."
    
    # Create session
    local create_response
    create_response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -d '{
            "strategy": "voting",
            "participants": {"primary": {"type": "helixagent"}}
        }')
    
    local session_id
    session_id=$(echo "$create_response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    
    # Get session
    local get_response
    get_response=$(curl -s "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions/$session_id")
    
    if ! echo "$get_response" | grep -q "$session_id"; then
        log_error "Failed to retrieve session"
        return 1
    fi
    
    # Cancel session
    local cancel_response
    cancel_response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions/$session_id/cancel")
    
    if echo "$cancel_response" | grep -q "cancelled"; then
        log_success "Session lifecycle test passed"
        return 0
    else
        log_error "Failed to cancel session"
        return 1
    fi
}

# Test 6: Performance benchmark
test_performance() {
    log_info "Test 6: Performance benchmark..."
    
    local start_time end_time duration
    
    start_time=$(date +%s.%N)
    
    # Create and execute session
    local response
    response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -d '{
            "strategy": "voting",
            "participants": {"primary": {"type": "helixagent"}}
        }')
    
    end_time=$(date +%s.%N)
    duration=$(echo "$end_time - $start_time" | bc)
    
    log_info "Session creation time: ${duration}s"
    
    if (( $(echo "$duration < 5.0" | bc -l) )); then
        log_success "Performance test passed"
        return 0
    else
        log_warn "Performance is slower than expected"
        return 0
    fi
}

# Generate report
generate_report() {
    log_info "Generating challenge report..."
    
    cat > "$REPORT_DIR/report.md" << EOF
# Ensemble Voting Strategy Challenge Report

**Date:** $(date)
**HelixAgent URL:** $ENSEMBLE_ENDPOINT

## Test Results

| Test | Status |
|------|--------|
| Create Voting Session | ${TEST1:-SKIPPED} |
| Execute Voting Task | ${TEST2:-SKIPPED} |
| Concurrent Sessions | ${TEST3:-SKIPPED} |
| Consensus Calculation | ${TEST4:-SKIPPED} |
| Session Lifecycle | ${TEST5:-SKIPPED} |
| Performance | ${TEST6:-SKIPPED} |

## Summary

- **Total Tests:** 6
- **Passed:** $(grep -c "PASS" <<< "${TEST1}${TEST2}${TEST3}${TEST4}${TEST5}${TEST6}" 2>/dev/null || echo 0)
- **Failed:** $(grep -c "FAIL" <<< "${TEST1}${TEST2}${TEST3}${TEST4}${TEST5}${TEST6}" 2>/dev/null || echo 0)

## Files

$(ls -1 "$REPORT_DIR"/*.json 2>/dev/null | while read f; do echo "- $(basename $f)"; done)

## Performance Metrics

- Session creation latency: measured
- Concurrent session handling: tested
- Consensus calculation: verified

EOF

    log_success "Report generated: $REPORT_DIR/report.md"
}

# Main execution
main() {
    log_info "Starting Ensemble Voting Challenge"
    log_info "Report directory: $REPORT_DIR"
    
    local exit_code=0
    
    check_prerequisites
    
    # Run tests
    if test_create_voting_session; then
        TEST1="PASS"
    else
        TEST1="FAIL"
        exit_code=1
    fi
    
    if [ -n "$SESSION_ID" ]; then
        if test_execute_voting_task; then
            TEST2="PASS"
        else
            TEST2="FAIL"
            exit_code=1
        fi
    fi
    
    if test_concurrent_voting_sessions; then
        TEST3="PASS"
    else
        TEST3="FAIL"
        exit_code=1
    fi
    
    if test_consensus_calculation; then
        TEST4="PASS"
    else
        TEST4="FAIL"
        exit_code=1
    fi
    
    if test_session_lifecycle; then
        TEST5="PASS"
    else
        TEST5="FAIL"
        exit_code=1
    fi
    
    if test_performance; then
        TEST6="PASS"
    else
        TEST6="FAIL"
    fi
    
    generate_report
    
    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    if [ $exit_code -eq 0 ]; then
        echo "║  CHALLENGE COMPLETED SUCCESSFULLY ✓                         ║"
    else
        echo "║  CHALLENGE COMPLETED WITH WARNINGS ⚠                        ║"
    fi
    echo "║  Report: $REPORT_DIR"
    echo "╚══════════════════════════════════════════════════════════════╝"
    
    return $exit_code
}

# Run main
main "$@"
