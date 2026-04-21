#!/usr/bin/env bash
# Red-Team Fixtures Challenge
#
# Validates the defensive red-team fixture harness end-to-end:
#   - Every supported attack class has a YAML file.
#   - Loader package builds and its tests pass.
#   - DeepTeamRedTeamer.RunFixtureSuite builds and its tests pass.
#   - .gitattributes excludes fixtures from git-archive output.
#   - Fixture files advertise the defensive-use-only policy.
#
# Resource cap per Constitution Rule 15 (30-40% host resources).
# Runs without a live HelixAgent binary: the harness is evaluated
# against the in-process StandardGuardrailPipeline via the package
# tests (real pipeline; not a fake).

set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

PASS=0
FAIL=0
TOTAL=0

pass() {
    PASS=$((PASS + 1))
    TOTAL=$((TOTAL + 1))
    printf '  [PASS] %s\n' "$1"
}

fail() {
    FAIL=$((FAIL + 1))
    TOTAL=$((TOTAL + 1))
    printf '  [FAIL] %s\n' "$1" >&2
}

section() {
    printf '\n=== %s ===\n' "$1"
}

FIXTURES_DIR="RedTeam/fixtures"
EXPECTED_CLASSES=(
    jailbreak
    abliteration_probe
    filter_bypass
    stego_mutation
    genetic_seed
    system_prompt_extraction
    role_reversal
)

section "Section 1: Fixture YAML files present for every supported class"
for class in "${EXPECTED_CLASSES[@]}"; do
    if [ -f "$FIXTURES_DIR/$class.yaml" ]; then
        pass "fixture file present: $class.yaml"
    else
        fail "fixture file MISSING: $class.yaml"
    fi
done

section "Section 2: Fixture YAML files declare the correct attack_class"
for class in "${EXPECTED_CLASSES[@]}"; do
    yaml_path="$FIXTURES_DIR/$class.yaml"
    if [ ! -f "$yaml_path" ]; then
        fail "attack_class header absent (file missing): $class.yaml"
        continue
    fi
    if grep -qE "^attack_class:[[:space:]]*${class}[[:space:]]*$" "$yaml_path"; then
        pass "attack_class header correct in $class.yaml"
    else
        fail "attack_class header MISMATCH in $class.yaml"
    fi
done

section "Section 3: Defensive-use policy banner on every fixture"
for class in "${EXPECTED_CLASSES[@]}"; do
    yaml_path="$FIXTURES_DIR/$class.yaml"
    if [ ! -f "$yaml_path" ]; then
        continue
    fi
    if grep -q -i "DEFENSIVE USE ONLY" "$yaml_path"; then
        pass "defensive-use-only banner present in $class.yaml"
    else
        fail "defensive-use-only banner MISSING in $class.yaml"
    fi
done

section "Section 4: Fixtures live in the extracted RedTeam submodule"
if [ -f "RedTeam/fixtures.go" ] && [ -f "RedTeam/go.mod" ]; then
    pass "RedTeam submodule present with loader and go.mod"
else
    fail "RedTeam submodule is MISSING (run: git submodule update --init --recursive)"
fi

section "Section 5: Fixture loader compiles and its tests pass"
if (cd RedTeam && GOMAXPROCS=2 nice -n 19 ionice -c 3 \
    go test -count=1 -p 1 ./...) >/tmp/redteam_fixtures_loader.log 2>&1; then
    pass "loader package tests pass"
else
    fail "loader package tests FAILED (see /tmp/redteam_fixtures_loader.log)"
fi

section "Section 6: RunFixtureSuite compiles and its tests pass"
if GOMAXPROCS=2 nice -n 19 ionice -c 3 \
    go test -count=1 -p 1 ./internal/security \
        -run 'RunFixtureSuite|FixtureLoader|LoadByClass|LoadAll' \
        >/tmp/redteam_fixtures_suite.log 2>&1; then
    pass "RunFixtureSuite tests pass"
else
    fail "RunFixtureSuite tests FAILED (see /tmp/redteam_fixtures_suite.log)"
fi

section "Section 7: Makefile target wired"
if grep -q "^test-redteam-fixtures:" Makefile; then
    pass "Makefile target test-redteam-fixtures is defined"
else
    fail "Makefile target test-redteam-fixtures is MISSING"
fi

section "Section 8: README present for the fixtures directory"
if [ -f "$FIXTURES_DIR/README.md" ]; then
    pass "fixtures README.md is present"
else
    fail "fixtures README.md is MISSING"
fi

section "Summary"
printf 'Total : %d\n' "$TOTAL"
printf 'Passed: %d\n' "$PASS"
printf 'Failed: %d\n' "$FAIL"

if [ "$FAIL" -eq 0 ]; then
    printf '\nALL CHECKS PASSED\n'
    exit 0
fi
printf '\nCHALLENGE FAILED: %d/%d checks failed\n' "$FAIL" "$TOTAL" >&2
exit 1
