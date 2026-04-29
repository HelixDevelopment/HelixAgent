#!/bin/bash
# Debate Reflexion Framework Challenge
# Validates the Reflexion iterative self-improvement framework:
# episodic memory, reflection generator, reflexion loop, accumulated wisdom,
# and wiring into DebateService.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-reflexion" "Debate Reflexion Framework Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Reflexion Framework Challenge"
log_info "=============================================="

# --- Section 1: Package existence and compilation ---

log_info "Test 1: Reflexion package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/debate/reflexion/... 2>&1); then
    record_assertion "reflexion_compile" "true" "true" "Reflexion package compiles"
else
    record_assertion "reflexion_compile" "true" "false" "Reflexion package failed to compile"
fi

log_info "Test 2: EpisodicMemoryBuffer type exists"
if grep -q "type EpisodicMemoryBuffer struct" "$PROJECT_ROOT/internal/debate/reflexion/episodic_memory.go" 2>/dev/null; then
    record_assertion "episodic_memory_buffer" "true" "true" "EpisodicMemoryBuffer type found"
else
    record_assertion "episodic_memory_buffer" "true" "false" "EpisodicMemoryBuffer type NOT found"
fi

log_info "Test 3: ReflectionGenerator type exists"
if grep -q "type ReflectionGenerator struct" "$PROJECT_ROOT/internal/debate/reflexion/reflection_generator.go" 2>/dev/null; then
    record_assertion "reflection_generator" "true" "true" "ReflectionGenerator type found"
else
    record_assertion "reflection_generator" "true" "false" "ReflectionGenerator type NOT found"
fi

log_info "Test 4: ReflexionLoop type exists"
if grep -q "type ReflexionLoop struct" "$PROJECT_ROOT/internal/debate/reflexion/reflexion_loop.go" 2>/dev/null; then
    record_assertion "reflexion_loop" "true" "true" "ReflexionLoop type found"
else
    record_assertion "reflexion_loop" "true" "false" "ReflexionLoop type NOT found"
fi

log_info "Test 5: AccumulatedWisdom type exists"
if grep -q "type AccumulatedWisdom struct" "$PROJECT_ROOT/internal/debate/reflexion/accumulated_wisdom.go" 2>/dev/null; then
    record_assertion "accumulated_wisdom" "true" "true" "AccumulatedWisdom type found"
else
    record_assertion "accumulated_wisdom" "true" "false" "AccumulatedWisdom type NOT found"
fi

# --- Section 2: Tests pass ---

log_info "Test 6: Episodic memory tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/reflexion/ -run "TestEpisodic|TestNewEpisodic|TestMemoryBuffer" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "episodic_memory_tests" "pass" "true" "Episodic memory tests passed"
else
    record_assertion "episodic_memory_tests" "pass" "false" "Episodic memory tests failed"
fi

log_info "Test 7: Reflection generator tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/reflexion/ -run "TestReflection|TestGenerate" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "reflection_generator_tests" "pass" "true" "Reflection generator tests passed"
else
    record_assertion "reflection_generator_tests" "pass" "false" "Reflection generator tests failed"
fi

log_info "Test 8: Reflexion loop tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/reflexion/ -run "TestReflexionLoop|TestLoop" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "reflexion_loop_tests" "pass" "true" "Reflexion loop tests passed"
else
    record_assertion "reflexion_loop_tests" "pass" "false" "Reflexion loop tests failed"
fi

log_info "Test 9: Accumulated wisdom tests pass"
if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/reflexion/ -run "TestAccumulated|TestWisdom" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "accumulated_wisdom_tests" "pass" "true" "Accumulated wisdom tests passed"
else
    record_assertion "accumulated_wisdom_tests" "pass" "false" "Accumulated wisdom tests failed"
fi

# --- Section 3: Integration with DebateService ---

log_info "Test 10: DebateService has reflexionMemory field"
if grep -q "reflexionMemory.*\*reflexion.EpisodicMemoryBuffer" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    record_assertion "debate_svc_reflexion_memory" "true" "true" "reflexionMemory wired in DebateService"
else
    record_assertion "debate_svc_reflexion_memory" "true" "false" "reflexionMemory NOT wired in DebateService"
fi

log_info "Test 11: DebateService has reflexionGenerator field"
if grep -q "reflexionGenerator.*\*reflexion.ReflectionGenerator" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    record_assertion "debate_svc_reflexion_gen" "true" "true" "reflexionGenerator wired in DebateService"
else
    record_assertion "debate_svc_reflexion_gen" "true" "false" "reflexionGenerator NOT wired in DebateService"
fi

log_info "Test 12: DebateService has reflexionLoop field"
if grep -q "reflexionLoop.*\*reflexion.ReflexionLoop" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    record_assertion "debate_svc_reflexion_loop" "true" "true" "reflexionLoop wired in DebateService"
else
    record_assertion "debate_svc_reflexion_loop" "true" "false" "reflexionLoop NOT wired in DebateService"
fi

# --- Finalize ---

FAILURES=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)
if [ "$FAILURES" -eq 0 ]; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
