#!/bin/bash
# HelixAgent Challenge - HelixQA Validation
# Validates HelixQA test infrastructure, test banks, session execution

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

if [ -f "$SCRIPT_DIR/challenge_framework.sh" ]; then
    source "$SCRIPT_DIR/challenge_framework.sh"
    init_challenge "helixqa-validation" "HelixQA Validation"
    load_env
    FRAMEWORK_LOADED=true
else
    FRAMEWORK_LOADED=false
fi

PASSED=0
FAILED=0

record_result() {
    local name="$1" status="$2"
    if [ "$FRAMEWORK_LOADED" = true ]; then
        if [ "$status" = "PASS" ]; then
            record_assertion "test" "$name" "true" "$name"
        else
            record_assertion "test" "$name" "false" "$name"
        fi
    fi
    if [ "$status" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        echo -e "\033[0;32m[PASS]\033[0m $name"
    else
        FAILED=$((FAILED + 1))
        echo -e "\033[0;31m[FAIL]\033[0m $name"
    fi
}

# Test 1: HelixQA binary exists
if [ -f "$PROJECT_ROOT/bin/helixqa" ]; then
    record_result "HelixQA binary exists" "PASS"
else
    record_result "HelixQA binary exists" "FAIL"
fi

# Test 2: HelixQA binary is executable
if [ -x "$PROJECT_ROOT/bin/helixqa" ]; then
    record_result "HelixQA binary is executable" "PASS"
else
    record_result "HelixQA binary is executable" "FAIL"
fi

# Test 3: HelixQA version command works
if "$PROJECT_ROOT/bin/helixqa" version 2>/dev/null | grep -q "helixqa"; then
    record_result "HelixQA version command works" "PASS"
else
    record_result "HelixQA version command works" "FAIL"
fi

# Test 4: API test bank exists (JSON)
if [ -f "$PROJECT_ROOT/qa-banks/helixagent-api.json" ]; then
    record_result "API test bank (JSON) exists" "PASS"
else
    record_result "API test bank (JSON) exists" "FAIL"
fi

# Test 5: Comprehensive test bank exists
if [ -f "$PROJECT_ROOT/qa-banks/helixagent-comprehensive.json" ]; then
    record_result "Comprehensive test bank exists" "PASS"
else
    record_result "Comprehensive test bank exists" "FAIL"
fi

# Test 6: Stress test bank exists
if [ -f "$PROJECT_ROOT/qa-banks/helixagent-stress.json" ]; then
    record_result "Stress test bank exists" "PASS"
else
    record_result "Stress test bank exists" "FAIL"
fi

# Test 7: API test bank is valid JSON
if python3 -c "import json; json.load(open('$PROJECT_ROOT/qa-banks/helixagent-api.json'))" 2>/dev/null; then
    record_result "API test bank is valid JSON" "PASS"
else
    record_result "API test bank is valid JSON" "FAIL"
fi

# Test 8: API test bank has 10+ challenges
count=$(python3 -c "import json; print(len(json.load(open('$PROJECT_ROOT/qa-banks/helixagent-api.json')).get('challenges',[])))" 2>/dev/null)
if [ "${count:-0}" -ge 10 ]; then
    record_result "API test bank has 10+ challenges (${count})" "PASS"
else
    record_result "API test bank has 10+ challenges (${count:-0})" "FAIL"
fi

# Test 9: Comprehensive bank has 25+ challenges
count=$(python3 -c "import json; print(len(json.load(open('$PROJECT_ROOT/qa-banks/helixagent-comprehensive.json')).get('challenges',[])))" 2>/dev/null)
if [ "${count:-0}" -ge 25 ]; then
    record_result "Comprehensive bank has 25+ challenges (${count})" "PASS"
else
    record_result "Comprehensive bank has 25+ challenges (${count:-0})" "FAIL"
fi

# Test 10: Session results directory exists
if [ -d "$PROJECT_ROOT/qa-results/sessions" ]; then
    sessions=$(find "$PROJECT_ROOT/qa-results/sessions" -maxdepth 1 -type d | wc -l)
    record_result "Session results directory exists (${sessions} sessions)" "PASS"
else
    record_result "Session results directory exists" "FAIL"
fi

# Test 11: At least one session report exists
reports=$(find "$PROJECT_ROOT/qa-results/sessions" -name "qa-report.md" 2>/dev/null | wc -l)
if [ "${reports}" -ge 1 ]; then
    record_result "Session reports exist (${reports} reports)" "PASS"
else
    record_result "Session reports exist (0 reports)" "FAIL"
fi

# Test 12: At least one autonomous session log exists
logs=$(find "$PROJECT_ROOT/qa-results/sessions" -name "session.log" 2>/dev/null | wc -l)
if [ "${logs}" -ge 1 ]; then
    record_result "Autonomous session logs exist (${logs} logs)" "PASS"
else
    record_result "Autonomous session logs exist (0 logs)" "FAIL"
fi

# Test 13: Video recordings are git-ignored
if grep -q "qa-results/\*\*/\*.mp4" "$PROJECT_ROOT/.gitignore" 2>/dev/null; then
    record_result "Video recordings git-ignored (.mp4)" "PASS"
else
    record_result "Video recordings git-ignored (.mp4)" "FAIL"
fi

# Test 14: HelixQA autonomous command has stub agent wiring
if grep -q "stubAgent" "$PROJECT_ROOT/../HelixQA/cmd/helixqa/main.go" 2>/dev/null; then
    record_result "Autonomous command has stub agent wiring" "PASS"
else
    record_result "Autonomous command has stub agent wiring" "FAIL"
fi

# Test 15: HelixQA module compiles
if (cd "$PROJECT_ROOT/../HelixQA" && go build ./... 2>/dev/null); then
    record_result "HelixQA module compiles" "PASS"
else
    record_result "HelixQA module compiles" "FAIL"
fi

echo ""
echo "=========================================="
echo "  HelixQA Validation: ${PASSED} passed, ${FAILED} failed"
echo "=========================================="

if [ "$FRAMEWORK_LOADED" = true ]; then
    finalize_challenge
fi

[ "$FAILED" -eq 0 ] && exit 0 || exit 1
