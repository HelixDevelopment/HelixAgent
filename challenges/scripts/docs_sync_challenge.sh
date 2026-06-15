#!/bin/bash
# Documentation Sync Challenge
# Validates that governance documents (CLAUDE.md, AGENTS.md, MODULES.md)
# are synchronized with consistent module counts and key sections.
#
# Tests:
# 1. Module count in CLAUDE.md matches MODULES.md
# 2. AGENTS.md module count references match
# 3. Key sections present in all 3 governance docs
# 4. New modules (Phase 8+) referenced consistently
# 5. No stale module counts (27, 35, etc.)

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

echo "============================================"
echo "Documentation Sync Challenge"
echo "============================================"
echo ""

CLAUDE_MD="$PROJECT_ROOT/CLAUDE.md"
AGENTS_MD="$PROJECT_ROOT/AGENTS.md"
MODULES_MD="$PROJECT_ROOT/docs/MODULES.md"

# -----------------------------------------------
# Precondition: All governance files exist
# -----------------------------------------------
echo "--- Precondition: Governance Files Exist ---"

for f in "$CLAUDE_MD" "$AGENTS_MD" "$MODULES_MD"; do
    if [ -f "$f" ]; then
        pass_test "$(basename "$f") exists"
    else
        fail_test "$(basename "$f") exists -- BLOCKING, cannot continue"
        echo ""
        echo "RESULT: FAILED (missing governance file)"
        exit 1
    fi
done

# -----------------------------------------------
# Test 1: Module count in CLAUDE.md matches MODULES.md
# -----------------------------------------------
echo ""
echo "--- Test Group 1: CLAUDE.md Module Count Matches MODULES.md ---"

# Extract module count from MODULES.md table rows
MODULES_COUNT=$(grep -cE '^\| [0-9]+ \|' "$MODULES_MD" 2>/dev/null || echo "0")

# Extract module count from CLAUDE.md "XX extracted modules" reference
CLAUDE_COUNT=$(sed -nE 's/.*\*\*([0-9]+) extracted modules\*\*.*/\1/p' "$CLAUDE_MD" 2>/dev/null | head -1); CLAUDE_COUNT=${CLAUDE_COUNT:-0}

if [ "$MODULES_COUNT" -eq "$CLAUDE_COUNT" ] && [ "$MODULES_COUNT" -gt 0 ]; then
    pass_test "CLAUDE.md module count ($CLAUDE_COUNT) matches MODULES.md table ($MODULES_COUNT)"
else
    fail_test "CLAUDE.md module count ($CLAUDE_COUNT) does not match MODULES.md table ($MODULES_COUNT)"
fi

# Also check MODULES.md total line
MODULES_TOTAL_LINE=$(sed -nE 's/.*Total: ([0-9]+) modules.*/\1/p' "$MODULES_MD" 2>/dev/null | head -1); MODULES_TOTAL_LINE=${MODULES_TOTAL_LINE:-0}
if [ "$MODULES_TOTAL_LINE" -eq "$MODULES_COUNT" ]; then
    pass_test "MODULES.md 'Total' line ($MODULES_TOTAL_LINE) matches table row count ($MODULES_COUNT)"
else
    fail_test "MODULES.md 'Total' line ($MODULES_TOTAL_LINE) does not match table row count ($MODULES_COUNT)"
fi

# -----------------------------------------------
# Test 2: AGENTS.md module count references match
# -----------------------------------------------
echo ""
echo "--- Test Group 2: AGENTS.md Module Count ---"

# AGENTS.md should reference the same module count
AGENTS_COUNT=$(sed -nE 's/(^|.*[^0-9])([0-9]+) extracted modules.*/\2/p' "$AGENTS_MD" 2>/dev/null | head -1); AGENTS_COUNT=${AGENTS_COUNT:-0}

if [ "$AGENTS_COUNT" -eq "$MODULES_COUNT" ] && [ "$AGENTS_COUNT" -gt 0 ]; then
    pass_test "AGENTS.md module count ($AGENTS_COUNT) matches MODULES.md ($MODULES_COUNT)"
else
    fail_test "AGENTS.md module count ($AGENTS_COUNT) does not match MODULES.md ($MODULES_COUNT)"
fi

# -----------------------------------------------
# Test 3: Key sections present in governance docs
# -----------------------------------------------
echo ""
echo "--- Test Group 3: Key Sections in Governance Docs ---"

# CLAUDE.md must have these sections
CLAUDE_SECTIONS=("Extracted Modules" "Project Overview" "Mandatory Development Standards" "Architecture" "Build & Run" "Testing")
for section in "${CLAUDE_SECTIONS[@]}"; do
    if grep -qi "$section" "$CLAUDE_MD" 2>/dev/null; then
        pass_test "CLAUDE.md contains section: $section"
    else
        fail_test "CLAUDE.md missing section: $section"
    fi
done

# AGENTS.md must have these sections
AGENTS_SECTIONS=("Project Overview" "Mandatory Development Standards")
for section in "${AGENTS_SECTIONS[@]}"; do
    if grep -qi "$section" "$AGENTS_MD" 2>/dev/null; then
        pass_test "AGENTS.md contains section: $section"
    else
        fail_test "AGENTS.md missing section: $section"
    fi
done

# MODULES.md must have these sections
MODULES_SECTIONS=("Module Index" "Foundation" "Infrastructure" "Services" "Integration" "AI/ML" "Cognitive" "Specification" "Pre-existing")
for section in "${MODULES_SECTIONS[@]}"; do
    if grep -qi "$section" "$MODULES_MD" 2>/dev/null; then
        pass_test "MODULES.md contains section: $section"
    else
        fail_test "MODULES.md missing section: $section"
    fi
done

# -----------------------------------------------
# Test 4: New modules referenced consistently
# -----------------------------------------------
echo ""
echo "--- Test Group 4: New Modules Referenced Consistently ---"

NEW_MODULES=("DocProcessor" "HelixQA" "LLMOrchestrator" "VisionEngine" "LLMsVerifier" "MCP-Servers")

for mod in "${NEW_MODULES[@]}"; do
    # Must be in MODULES.md
    if grep -q "$mod" "$MODULES_MD" 2>/dev/null; then
        pass_test "$mod referenced in MODULES.md"
    else
        fail_test "$mod referenced in MODULES.md"
    fi

    # Must be in CLAUDE.md
    if grep -q "$mod" "$CLAUDE_MD" 2>/dev/null; then
        pass_test "$mod referenced in CLAUDE.md"
    else
        fail_test "$mod referenced in CLAUDE.md"
    fi
done

# -----------------------------------------------
# Test 5: No stale module counts
# -----------------------------------------------
echo ""
echo "--- Test Group 5: No Stale Module Counts ---"

# Check for old counts that should have been updated
STALE_PATTERNS=("27 extracted modules" "35 extracted modules" "30 extracted modules")
for pattern in "${STALE_PATTERNS[@]}"; do
    if grep -q "$pattern" "$CLAUDE_MD" 2>/dev/null; then
        fail_test "CLAUDE.md has stale count: '$pattern'"
    else
        pass_test "CLAUDE.md does not contain stale: '$pattern'"
    fi

    if grep -q "$pattern" "$AGENTS_MD" 2>/dev/null; then
        fail_test "AGENTS.md has stale count: '$pattern'"
    else
        pass_test "AGENTS.md does not contain stale: '$pattern'"
    fi
done

# -----------------------------------------------
# Summary
# -----------------------------------------------
echo ""
echo "============================================"
echo "Documentation Sync Challenge Summary"
echo "============================================"
echo "PASSED: $PASS"
echo "FAILED: $FAIL"
echo "TOTAL:  $TOTAL"
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo "RESULT: FAILED ($FAIL failures)"
    exit 1
else
    echo "RESULT: PASSED (all $PASS tests passed)"
    exit 0
fi
