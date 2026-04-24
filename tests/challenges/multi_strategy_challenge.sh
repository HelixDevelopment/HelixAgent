#!/bin/bash
# Challenge: Multi-Strategy Ensemble Coordination
# Tests all 7 coordination strategies under various conditions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPORT_DIR="$PROJECT_ROOT/challenge-results/multi-strategy-$(date +%Y%m%d-%H%M%S)"

mkdir -p "$REPORT_DIR"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  CHALLENGE: Multi-Strategy Ensemble Coordination             ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

ENSEMBLE_ENDPOINT="${HELIXAGENT_URL:-http://localhost:8100}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Test strategies
STRATEGIES=("voting" "debate" "consensus" "pipeline" "parallel" "sequential" "expert_panel")

# Test each strategy
test_strategy() {
    local strategy=$1
    log_info "Testing strategy: $strategy"
    
    local response
    response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -d "{
            \"strategy\": \"$strategy\",
            \"participants\": {
                \"primary\": {\"type\": \"helixagent\"},
                \"critiques\": [
                    {\"type\": \"helixagent\"},
                    {\"type\": \"helixagent\"}
                ]
            }
        }" 2>&1)
    
    if echo "$response" | grep -q "id"; then
        local session_id
        session_id=$(echo "$response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        log_success "Created $strategy session: $session_id"
        echo "$session_id" >> "$REPORT_DIR/${strategy}_sessions.txt"
        
        # Execute task
        local exec_response
        exec_response=$(curl -s -X POST "$ENSEMBLE_ENDPOINT/v1/ensemble/sessions/$session_id/execute" \
            -H "Content-Type: application/json" \
            -d '{
                "content": "Write a function to reverse a string",
                "timeout": 60
            }' 2>&1)
        
        echo "$exec_response" > "$REPORT_DIR/${strategy}_result.json"
        
        if echo "$exec_response" | grep -q "consensus_reached"; then
            log_success "$strategy execution completed"
            return 0
        else
            log_error "$strategy execution failed"
            return 1
        fi
    else
        log_error "Failed to create $strategy session"
        return 1
    fi
}

# Main test loop
main() {
    log_info "Starting Multi-Strategy Challenge"
    log_info "Testing all 7 coordination strategies"
    
    local passed=0
    local failed=0
    
    for strategy in "${STRATEGIES[@]}"; do
        if test_strategy "$strategy"; then
            ((passed++))
        else
            ((failed++))
        fi
        echo ""
    done
    
    # Generate report
    cat > "$REPORT_DIR/report.md" << EOF
# Multi-Strategy Ensemble Challenge Report

**Date:** $(date)
**Strategies Tested:** 7

## Results

| Strategy | Status |
|----------|--------|
$(for s in "${STRATEGIES[@]}"; do 
    if grep -q "$s" "$REPORT_DIR/${s}_sessions.txt" 2>/dev/null; then
        echo "| $s | PASS |"
    else
        echo "| $s | FAIL |"
    fi
done)

## Summary

- **Passed:** $passed
- **Failed:** $failed
- **Total:** 7

## Files

$(ls -1 "$REPORT_DIR"/*.json 2>/dev/null | while read f; do echo "- $(basename $f)"; done)

EOF

    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    if [ $failed -eq 0 ]; then
        echo "║  ALL STRATEGIES PASSED ✓                                    ║"
    else
        echo "║  CHALLENGE COMPLETED WITH $failed FAILURES                  ║"
    fi
    echo "║  Report: $REPORT_DIR"
    echo "╚══════════════════════════════════════════════════════════════╝"
    
    return $failed
}

main "$@"
