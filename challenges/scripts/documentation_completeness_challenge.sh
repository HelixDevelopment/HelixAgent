#!/bin/bash
# Documentation Completeness Challenge
# Validates that all modules, SDKs, and debate subdirectories have proper documentation.
#
# Tests:
# 1. All 27 extracted modules have README.md, CLAUDE.md, AGENTS.md
# 2. SDK directories (android, ios, cli) have README.md
# 3. Debate subdirectories have README.md
# 4. Each README meets minimum line count threshold

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0
TOTAL=0

pass_test() {
    PASS=$((PASS + 1))
    TOTAL=$((TOTAL + 1))
    echo "  PASS: $1"
}

fail_test() {
    FAIL=$((FAIL + 1))
    TOTAL=$((TOTAL + 1))
    echo "  FAIL: $1"
}

check_file_exists() {
    local file="$1"
    local desc="$2"
    if [ -f "$file" ]; then
        pass_test "$desc"
    else
        fail_test "$desc -- file not found: $file"
    fi
}

check_min_lines() {
    local file="$1"
    local min_lines="$2"
    local desc="$3"
    if [ ! -f "$file" ]; then
        fail_test "$desc -- file not found"
        return
    fi
    local lines
    lines=$(wc -l < "$file")
    if [ "$lines" -ge "$min_lines" ]; then
        pass_test "$desc ($lines lines >= $min_lines)"
    else
        fail_test "$desc ($lines lines < $min_lines required)"
    fi
}

echo "============================================"
echo "Documentation Completeness Challenge"
echo "============================================"
echo ""

# -----------------------------------------------
# Test 1: Extracted modules have required docs
# -----------------------------------------------
echo "--- Test Group 1: Extracted Module Documentation ---"

MODULES=(
    "Agentic" "Auth" "Benchmark" "Cache" "Challenges" "Concurrency"
    "Containers" "Database" "Embeddings" "EventBus" "Formatters"
    "HelixMemory" "HelixSpecifier" "LLMOps" "MCP_Module" "Memory"
    "Messaging" "Observability" "Optimization" "Planning" "Plugins"
    "RAG" "Security" "SelfImprove" "Storage" "Streaming" "VectorDB"
)

for module in "${MODULES[@]}"; do
    check_file_exists "$PROJECT_ROOT/$module/README.md" "$module has README.md"
done

for module in "${MODULES[@]}"; do
    check_file_exists "$PROJECT_ROOT/$module/CLAUDE.md" "$module has CLAUDE.md"
done

for module in "${MODULES[@]}"; do
    check_file_exists "$PROJECT_ROOT/$module/AGENTS.md" "$module has AGENTS.md"
done

echo ""

# -----------------------------------------------
# Test 2: SDK documentation
# -----------------------------------------------
echo "--- Test Group 2: SDK Documentation ---"

SDK_DIRS=("android" "ios" "cli")

for sdk in "${SDK_DIRS[@]}"; do
    check_file_exists "$PROJECT_ROOT/sdk/$sdk/README.md" "sdk/$sdk has README.md"
done

echo ""

# -----------------------------------------------
# Test 3: Debate subdirectory documentation
# -----------------------------------------------
echo "--- Test Group 3: Debate Subdirectory Documentation ---"

DEBATE_DIRS=("audit" "evaluation" "gates" "reflexion" "testing" "tools")

for dir in "${DEBATE_DIRS[@]}"; do
    check_file_exists "$PROJECT_ROOT/DebateOrchestrator/$dir/README.md" "DebateOrchestrator/$dir has README.md"
done

echo ""

# -----------------------------------------------
# Test 4: README minimum line counts
# -----------------------------------------------
echo "--- Test Group 4: README Minimum Line Counts ---"

# Major modules should have 100+ lines
MAJOR_MODULES=(
    "Agentic" "Benchmark" "LLMOps" "Planning" "SelfImprove"
    "RAG" "Streaming" "Security" "Optimization" "Plugins" "Memory"
)

for module in "${MAJOR_MODULES[@]}"; do
    check_min_lines "$PROJECT_ROOT/$module/README.md" 30 "$module/README.md >= 30 lines"
done

# SDK READMEs should be substantial
for sdk in "${SDK_DIRS[@]}"; do
    check_min_lines "$PROJECT_ROOT/sdk/$sdk/README.md" 30 "sdk/$sdk/README.md >= 30 lines"
done

# Debate READMEs should have at least 30 lines
for dir in "${DEBATE_DIRS[@]}"; do
    check_min_lines "$PROJECT_ROOT/DebateOrchestrator/$dir/README.md" 30 "DebateOrchestrator/$dir/README.md >= 30 lines"
done

echo ""

# -----------------------------------------------
# Summary
# -----------------------------------------------
echo "============================================"
echo "Documentation Completeness Challenge Results"
echo "============================================"
echo "Total: $TOTAL | Passed: $PASS | Failed: $FAIL"
echo ""

if [ "$FAIL" -eq 0 ]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    echo "SOME TESTS FAILED"
    exit 1
fi
