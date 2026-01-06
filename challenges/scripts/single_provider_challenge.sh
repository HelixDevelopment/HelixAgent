#!/bin/bash
#===============================================================================
# SUPERAGENT SINGLE-PROVIDER MULTI-INSTANCE CHALLENGE
#===============================================================================
# This challenge validates the single-provider multi-instance debate mode.
# It specifically tests scenarios where only ONE LLM provider is available,
# and SuperAgent must use that provider with multiple instances/models
# to fulfill all debate group roles.
#
# The challenge:
# 1. Discovers available providers
# 2. Selects only ONE provider for testing
# 3. Configures single-provider multi-instance mode
# 4. Runs debates with 3, 5, and N participants
# 5. Validates diversity in responses
# 6. Measures quality and consensus
# 7. Generates comprehensive reports
#
# IMPORTANT: This is a REAL integration test - NO MOCKS!
#
# Usage:
#   ./challenges/scripts/single_provider_challenge.sh [options]
#
# Options:
#   --provider NAME  Specify which provider to test (default: auto-detect best)
#   --participants N Number of debate participants (default: 5)
#   --rounds N       Number of debate rounds (default: 2)
#   --verbose        Enable verbose logging
#   --help           Show this help message
#
#===============================================================================

set -e

#===============================================================================
# CONFIGURATION
#===============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Timestamps
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
YEAR=$(date +%Y)
MONTH=$(date +%m)
DAY=$(date +%d)

# Directories
RESULTS_BASE="$CHALLENGES_DIR/results/single_provider_challenge"
RESULTS_DIR="$RESULTS_BASE/$YEAR/$MONTH/$DAY/$TIMESTAMP"
LOGS_DIR="$RESULTS_DIR/logs"
OUTPUT_DIR="$RESULTS_DIR/results"

# Log files
MAIN_LOG="$LOGS_DIR/single_provider_challenge.log"
DEBATE_LOG="$LOGS_DIR/debate_output.log"
API_LOG="$LOGS_DIR/api_responses.log"
ERROR_LOG="$LOGS_DIR/errors.log"

# Binary paths
SUPERAGENT_BINARY="$PROJECT_ROOT/bin/superagent"

# Test configuration
SUPERAGENT_PORT="${SUPERAGENT_PORT:-8080}"
SUPERAGENT_HOST="${SUPERAGENT_HOST:-localhost}"

# Default options
PROVIDER=""
NUM_PARTICIPANTS=5
NUM_ROUNDS=2
VERBOSE=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

#===============================================================================
# LOGGING FUNCTIONS
#===============================================================================

log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
    if [ -d "$(dirname "$MAIN_LOG")" ]; then
        echo -e "$msg" | tee -a "$MAIN_LOG"
    else
        echo -e "$msg"
    fi
}

log_info() {
    log "${BLUE}[INFO]${NC} $*"
}

log_success() {
    log "${GREEN}[SUCCESS]${NC} $*"
}

log_warning() {
    log "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
    log "${RED}[ERROR]${NC} $*"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$ERROR_LOG" 2>/dev/null || true
}

log_phase() {
    if [ -d "$(dirname "$MAIN_LOG")" ]; then
        echo "" | tee -a "$MAIN_LOG"
    else
        echo ""
    fi
    log "${PURPLE}========================================${NC}"
    log "${PURPLE}  $*${NC}"
    log "${PURPLE}========================================${NC}"
    if [ -d "$(dirname "$MAIN_LOG")" ]; then
        echo "" | tee -a "$MAIN_LOG"
    else
        echo ""
    fi
}

#===============================================================================
# HELPER FUNCTIONS
#===============================================================================

usage() {
    cat << EOF
${GREEN}SuperAgent Single-Provider Multi-Instance Challenge${NC}

${BLUE}Usage:${NC}
    $0 [options]

${BLUE}Options:${NC}
    ${YELLOW}--provider NAME${NC}   Specify which provider to test (default: auto-detect best)
    ${YELLOW}--participants N${NC}  Number of debate participants (default: 5)
    ${YELLOW}--rounds N${NC}        Number of debate rounds (default: 2)
    ${YELLOW}--verbose${NC}         Enable verbose logging
    ${YELLOW}--help${NC}            Show this help message

${BLUE}What this challenge tests:${NC}
    1. Single provider with multiple model instances
    2. Temperature diversity for response variation
    3. System prompt diversity for unique perspectives
    4. Debate quality in degraded mode
    5. Consensus building with identical model base

${BLUE}Diversity Mechanisms Tested:${NC}
    - Model diversity (if provider supports multiple models)
    - Temperature diversity (0.6 - 1.0 range)
    - System prompt diversity (unique role perspectives)
    - Role-based argumentation styles

${BLUE}Output:${NC}
    Results stored in: ${YELLOW}$RESULTS_BASE/<date>/<timestamp>/${NC}

EOF
}

setup_directories() {
    log_info "Creating directory structure..."
    mkdir -p "$LOGS_DIR"
    mkdir -p "$OUTPUT_DIR"
    touch "$ERROR_LOG"
    log_success "Directories created: $RESULTS_DIR"
}

load_environment() {
    log_info "Loading environment variables..."

    if [ -f "$PROJECT_ROOT/.env" ]; then
        set -a
        source "$PROJECT_ROOT/.env"
        set +a
        log_info "Loaded .env from project root"
    fi
}

check_superagent_running() {
    log_info "Checking if SuperAgent is running..."

    if curl -s "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/health" > /dev/null 2>&1 || \
       curl -s "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/v1/models" > /dev/null 2>&1; then
        log_success "SuperAgent is running on port $SUPERAGENT_PORT"
        return 0
    else
        return 1
    fi
}

start_superagent() {
    log_info "Starting SuperAgent..."

    if [ ! -x "$SUPERAGENT_BINARY" ]; then
        log_warning "SuperAgent binary not found, building..."
        (cd "$PROJECT_ROOT" && make build)
    fi

    "$SUPERAGENT_BINARY" > "$LOGS_DIR/superagent.log" 2>&1 &
    local pid=$!
    echo $pid > "$OUTPUT_DIR/superagent.pid"

    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if check_superagent_running; then
            log_success "SuperAgent started (PID: $pid)"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done

    log_error "SuperAgent failed to start"
    return 1
}

#===============================================================================
# PHASE 1: PROVIDER DISCOVERY
#===============================================================================

phase1_provider_discovery() {
    log_phase "PHASE 1: Provider Discovery"

    local discovery_result="$OUTPUT_DIR/provider_discovery.json"

    log_info "Discovering available providers..."

    # Call the discovery endpoint
    local response=$(curl -s "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/v1/providers/discovery" 2>&1)

    if [ -z "$response" ]; then
        log_warning "Discovery endpoint not available, using verification endpoint..."
        response=$(curl -s "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/v1/providers/verify" 2>&1)
    fi

    if [ -z "$response" ]; then
        log_error "Unable to discover providers"
        return 1
    fi

    echo "$response" > "$discovery_result"

    # Parse provider info
    local providers=$(echo "$response" | python3 -c "
import json
import sys
try:
    d = json.load(sys.stdin)
    providers = d.get('providers', [])
    healthy = [p for p in providers if p.get('status') == 'healthy' or p.get('verified') == True]

    if healthy:
        # Sort by score
        healthy.sort(key=lambda x: x.get('score', 0), reverse=True)
        for p in healthy:
            print(f\"{p.get('provider', p.get('name', 'unknown'))}:{p.get('score', 0)}\")
except Exception as e:
    print(f'ERROR:{e}', file=sys.stderr)
" 2>&1)

    if [[ "$providers" == ERROR:* ]]; then
        log_error "Failed to parse provider info: $providers"
        return 1
    fi

    # Store provider list
    echo "$providers" > "$OUTPUT_DIR/healthy_providers.txt"

    local provider_count=$(echo "$providers" | wc -l)
    log_info "Found $provider_count healthy provider(s)"

    if [ $provider_count -eq 0 ]; then
        log_error "No healthy providers available"
        return 1
    fi

    # Select provider for testing
    if [ -z "$PROVIDER" ]; then
        PROVIDER=$(echo "$providers" | head -1 | cut -d: -f1)
        log_info "Auto-selected best provider: $PROVIDER"
    else
        if echo "$providers" | grep -q "^$PROVIDER:"; then
            log_info "Using specified provider: $PROVIDER"
        else
            log_warning "Specified provider '$PROVIDER' not healthy, using best available"
            PROVIDER=$(echo "$providers" | head -1 | cut -d: -f1)
            log_info "Selected provider: $PROVIDER"
        fi
    fi

    export SELECTED_PROVIDER=$PROVIDER

    log_success "Provider discovery completed. Selected: $PROVIDER"
}

#===============================================================================
# PHASE 2: SINGLE-PROVIDER DEBATE TESTS
#===============================================================================

run_single_provider_debate() {
    local test_name=$1
    local num_participants=$2
    local topic=$3
    local output_file=$4

    log_info "Running debate: $test_name"
    log_info "  Provider: $SELECTED_PROVIDER"
    log_info "  Participants: $num_participants"
    log_info "  Rounds: $NUM_ROUNDS"
    log_info "  Topic: $topic"

    # Build participant config - all using the same provider
    local participants="["
    for i in $(seq 1 $num_participants); do
        if [ $i -gt 1 ]; then
            participants+=","
        fi
        local roles=("analyst" "proposer" "critic" "mediator" "debater" "opponent" "moderator" "strategist")
        local role_idx=$(( (i - 1) % ${#roles[@]} ))
        local role=${roles[$role_idx]}

        participants+="{\"participant_id\":\"p$i\",\"name\":\"Participant $i\",\"role\":\"$role\",\"llm_provider\":\"$SELECTED_PROVIDER\"}"
    done
    participants+="]"

    # Create debate request
    local debate_request=$(cat << DEBATEREQUEST
{
    "debate_id": "single-provider-$test_name-$TIMESTAMP",
    "topic": "$topic",
    "max_rounds": $NUM_ROUNDS,
    "timeout": "300s",
    "strategy": "confidence_weighted",
    "participants": $participants,
    "metadata": {
        "test_name": "$test_name",
        "mode": "single_provider",
        "challenge": "single_provider_challenge"
    }
}
DEBATEREQUEST
)

    # Submit debate
    local start_time=$(date +%s%N)
    local response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $SUPERAGENT_API_KEY" \
        -d "$debate_request" \
        "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/v1/debates" 2>&1)
    local end_time=$(date +%s%N)
    local duration_ms=$(( (end_time - start_time) / 1000000 ))

    echo "$response" > "$output_file"
    echo "Request:" >> "$DEBATE_LOG"
    echo "$debate_request" >> "$DEBATE_LOG"
    echo "Response:" >> "$DEBATE_LOG"
    echo "$response" >> "$DEBATE_LOG"
    echo "" >> "$DEBATE_LOG"

    # Analyze response
    local analysis=$(echo "$response" | python3 -c "
import json
import sys
try:
    d = json.load(sys.stdin)

    success = d.get('success', False)
    quality = d.get('quality_score', 0)
    final_score = d.get('final_score', 0)
    responses = d.get('all_responses', [])
    consensus = d.get('consensus', {})
    metadata = d.get('metadata', {})

    # Check for single-provider mode
    mode = metadata.get('mode', 'unknown')
    provider_used = metadata.get('provider', 'unknown')
    models_used = metadata.get('models_used', [])
    effective_diversity = metadata.get('effective_diversity', 0)

    # Calculate diversity metrics
    unique_content_hashes = set()
    for r in responses:
        content = r.get('content', '')[:100]  # First 100 chars
        unique_content_hashes.add(hash(content))

    content_diversity = len(unique_content_hashes) / max(len(responses), 1)

    print(f'SUCCESS:{success}')
    print(f'QUALITY:{quality}')
    print(f'FINAL_SCORE:{final_score}')
    print(f'RESPONSES:{len(responses)}')
    print(f'MODE:{mode}')
    print(f'PROVIDER:{provider_used}')
    print(f'MODELS:{len(models_used)}')
    print(f'DIVERSITY:{effective_diversity}')
    print(f'CONTENT_DIVERSITY:{content_diversity}')
    print(f'CONSENSUS:{consensus.get(\"reached\", False)}')

except Exception as e:
    print(f'ERROR:{e}')
" 2>&1)

    # Parse analysis
    local success=$(echo "$analysis" | grep "^SUCCESS:" | cut -d: -f2)
    local quality=$(echo "$analysis" | grep "^QUALITY:" | cut -d: -f2)
    local diversity=$(echo "$analysis" | grep "^DIVERSITY:" | cut -d: -f2)
    local responses=$(echo "$analysis" | grep "^RESPONSES:" | cut -d: -f2)
    local mode=$(echo "$analysis" | grep "^MODE:" | cut -d: -f2)

    if [ "$success" = "True" ]; then
        log_success "  Result: PASSED"
        log_info "    Quality: $quality"
        log_info "    Diversity: $diversity"
        log_info "    Responses: $responses"
        log_info "    Mode: $mode"
        log_info "    Duration: ${duration_ms}ms"
        return 0
    else
        log_error "  Result: FAILED"
        log_error "    $(echo "$analysis" | head -1)"
        return 1
    fi
}

phase2_debate_tests() {
    log_phase "PHASE 2: Single-Provider Debate Tests"

    local tests_passed=0
    local tests_failed=0

    # Test 1: 3-participant debate
    log_info ""
    log_info "Test 1: Three Participant Debate"
    if run_single_provider_debate "3p" 3 \
        "What are the key considerations for building a successful startup?" \
        "$OUTPUT_DIR/debate_3p.json"; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi

    # Test 2: 5-participant debate
    log_info ""
    log_info "Test 2: Five Participant Debate"
    if run_single_provider_debate "5p" 5 \
        "How should governments regulate artificial intelligence?" \
        "$OUTPUT_DIR/debate_5p.json"; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi

    # Test 3: Custom participant count
    log_info ""
    log_info "Test 3: Custom Participant Debate ($NUM_PARTICIPANTS participants)"
    if run_single_provider_debate "custom" $NUM_PARTICIPANTS \
        "What programming practices lead to maintainable code?" \
        "$OUTPUT_DIR/debate_custom.json"; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi

    # Test 4: Technical topic
    log_info ""
    log_info "Test 4: Technical Topic Debate"
    if run_single_provider_debate "tech" 5 \
        "Compare microservices vs monolithic architecture for a new project." \
        "$OUTPUT_DIR/debate_tech.json"; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi

    # Test 5: Ethical topic
    log_info ""
    log_info "Test 5: Ethical Topic Debate"
    if run_single_provider_debate "ethics" 5 \
        "What ethical considerations should guide AI development?" \
        "$OUTPUT_DIR/debate_ethics.json"; then
        tests_passed=$((tests_passed + 1))
    else
        tests_failed=$((tests_failed + 1))
    fi

    export TESTS_PASSED=$tests_passed
    export TESTS_FAILED=$tests_failed
    export TESTS_TOTAL=$((tests_passed + tests_failed))

    log_info ""
    log_info "Debate Tests Summary: $tests_passed/$TESTS_TOTAL passed"

    if [ $tests_passed -ge 3 ]; then
        log_success "Debate testing PASSED (at least 3/5 tests successful)"
        return 0
    else
        log_warning "Debate testing: only $tests_passed/5 tests passed"
        return 1
    fi
}

#===============================================================================
# PHASE 3: DIVERSITY ANALYSIS
#===============================================================================

phase3_diversity_analysis() {
    log_phase "PHASE 3: Diversity Analysis"

    local diversity_report="$OUTPUT_DIR/diversity_analysis.json"

    log_info "Analyzing response diversity across all debates..."

    python3 - "$OUTPUT_DIR" "$diversity_report" << 'DIVERSITYSCRIPT'
import json
import sys
import os
from pathlib import Path

output_dir = sys.argv[1]
report_file = sys.argv[2]

analysis = {
    "debates_analyzed": 0,
    "total_responses": 0,
    "average_diversity": 0,
    "average_quality": 0,
    "diversity_by_debate": [],
    "issues_found": [],
    "recommendations": []
}

# Analyze each debate output
debate_files = list(Path(output_dir).glob("debate_*.json"))

if not debate_files:
    print("No debate files found")
    analysis["issues_found"].append("No debate output files found")
else:
    total_diversity = 0
    total_quality = 0

    for f in debate_files:
        try:
            with open(f) as df:
                debate = json.load(df)

            metadata = debate.get("metadata", {})
            effective_diversity = metadata.get("effective_diversity", 0)
            quality_score = debate.get("quality_score", 0)
            responses = debate.get("all_responses", [])

            # Calculate content diversity
            contents = [r.get("content", "")[:200] for r in responses]
            unique_starts = len(set(contents))
            content_diversity = unique_starts / max(len(contents), 1)

            # Check for low diversity issues
            if effective_diversity < 0.3:
                analysis["issues_found"].append(f"{f.name}: Low effective diversity ({effective_diversity:.2f})")

            if content_diversity < 0.5:
                analysis["issues_found"].append(f"{f.name}: Low content diversity ({content_diversity:.2f})")

            analysis["diversity_by_debate"].append({
                "file": f.name,
                "effective_diversity": effective_diversity,
                "content_diversity": content_diversity,
                "quality_score": quality_score,
                "response_count": len(responses)
            })

            total_diversity += effective_diversity
            total_quality += quality_score
            analysis["debates_analyzed"] += 1
            analysis["total_responses"] += len(responses)

        except Exception as e:
            print(f"Error processing {f}: {e}")
            analysis["issues_found"].append(f"Error processing {f.name}: {str(e)}")

    if analysis["debates_analyzed"] > 0:
        analysis["average_diversity"] = total_diversity / analysis["debates_analyzed"]
        analysis["average_quality"] = total_quality / analysis["debates_analyzed"]

# Generate recommendations
if analysis["average_diversity"] < 0.4:
    analysis["recommendations"].append("Consider increasing temperature diversity range")
if analysis["average_diversity"] < 0.3:
    analysis["recommendations"].append("Add more distinct system prompts for each participant")
if analysis["average_quality"] < 0.5:
    analysis["recommendations"].append("Review debate topic complexity and prompt engineering")

# Write report
with open(report_file, 'w') as f:
    json.dump(analysis, f, indent=2)

print(f"Debates analyzed: {analysis['debates_analyzed']}")
print(f"Average diversity: {analysis['average_diversity']:.2f}")
print(f"Average quality: {analysis['average_quality']:.2f}")
print(f"Issues found: {len(analysis['issues_found'])}")

# Exit code based on diversity threshold
if analysis["average_diversity"] >= 0.3:
    sys.exit(0)
else:
    sys.exit(1)
DIVERSITYSCRIPT

    local exit_code=$?

    if [ $exit_code -eq 0 ]; then
        log_success "Diversity analysis PASSED"
        return 0
    else
        log_warning "Diversity analysis: below threshold"
        return 1
    fi
}

#===============================================================================
# PHASE 4: GENERATE REPORT
#===============================================================================

phase4_generate_report() {
    log_phase "PHASE 4: Generate Report"

    local summary_file="$OUTPUT_DIR/challenge_summary.json"
    local report_file="$OUTPUT_DIR/single_provider_challenge_report.md"

    # Load diversity analysis
    local avg_diversity=0
    local avg_quality=0
    if [ -f "$OUTPUT_DIR/diversity_analysis.json" ]; then
        avg_diversity=$(python3 -c "import json; d=json.load(open('$OUTPUT_DIR/diversity_analysis.json')); print(d.get('average_diversity', 0))" 2>/dev/null || echo "0")
        avg_quality=$(python3 -c "import json; d=json.load(open('$OUTPUT_DIR/diversity_analysis.json')); print(d.get('average_quality', 0))" 2>/dev/null || echo "0")
    fi

    # Determine overall success
    local overall_success=false
    if [ "${TESTS_PASSED:-0}" -ge 3 ]; then
        overall_success=true
    fi

    # Generate JSON summary
    cat > "$summary_file" << EOF
{
    "challenge": "Single-Provider Multi-Instance Debate",
    "timestamp": "$(date -Iseconds)",
    "configuration": {
        "provider": "$SELECTED_PROVIDER",
        "participants": $NUM_PARTICIPANTS,
        "rounds": $NUM_ROUNDS
    },
    "results": {
        "tests_passed": ${TESTS_PASSED:-0},
        "tests_failed": ${TESTS_FAILED:-0},
        "tests_total": ${TESTS_TOTAL:-0},
        "average_diversity": $avg_diversity,
        "average_quality": $avg_quality,
        "overall_success": $overall_success
    },
    "results_directory": "$RESULTS_DIR"
}
EOF

    # Generate markdown report
    cat > "$report_file" << EOF
# Single-Provider Multi-Instance Challenge Report

**Generated**: $(date '+%Y-%m-%d %H:%M:%S')
**Status**: $([ "$overall_success" = "true" ] && echo "PASSED" || echo "NEEDS ATTENTION")

## Configuration

| Setting | Value |
|---------|-------|
| Provider | $SELECTED_PROVIDER |
| Participants | $NUM_PARTICIPANTS |
| Rounds | $NUM_ROUNDS |

## Test Results

| Test | Participants | Status |
|------|-------------|--------|
| 3-Participant Debate | 3 | $([ -f "$OUTPUT_DIR/debate_3p.json" ] && python3 -c "import json; d=json.load(open('$OUTPUT_DIR/debate_3p.json')); print('PASSED' if d.get('success') else 'FAILED')" 2>/dev/null || echo "N/A") |
| 5-Participant Debate | 5 | $([ -f "$OUTPUT_DIR/debate_5p.json" ] && python3 -c "import json; d=json.load(open('$OUTPUT_DIR/debate_5p.json')); print('PASSED' if d.get('success') else 'FAILED')" 2>/dev/null || echo "N/A") |
| Custom Participant | $NUM_PARTICIPANTS | $([ -f "$OUTPUT_DIR/debate_custom.json" ] && python3 -c "import json; d=json.load(open('$OUTPUT_DIR/debate_custom.json')); print('PASSED' if d.get('success') else 'FAILED')" 2>/dev/null || echo "N/A") |
| Technical Topic | 5 | $([ -f "$OUTPUT_DIR/debate_tech.json" ] && python3 -c "import json; d=json.load(open('$OUTPUT_DIR/debate_tech.json')); print('PASSED' if d.get('success') else 'FAILED')" 2>/dev/null || echo "N/A") |
| Ethics Topic | 5 | $([ -f "$OUTPUT_DIR/debate_ethics.json" ] && python3 -c "import json; d=json.load(open('$OUTPUT_DIR/debate_ethics.json')); print('PASSED' if d.get('success') else 'FAILED')" 2>/dev/null || echo "N/A") |

**Tests Passed**: ${TESTS_PASSED:-0}/${TESTS_TOTAL:-0}

## Diversity Metrics

| Metric | Value |
|--------|-------|
| Average Effective Diversity | $(printf "%.2f" $avg_diversity) |
| Average Quality Score | $(printf "%.2f" $avg_quality) |

## Single-Provider Mode Details

The single-provider multi-instance mode creates diverse perspectives by:

1. **Temperature Diversity**: Each participant uses a slightly different temperature (0.6 - 1.0)
2. **System Prompt Diversity**: Unique perspectives assigned to each participant
3. **Role Assignment**: Different debate roles (analyst, critic, proposer, etc.)
4. **Model Diversity**: Uses multiple models from the same provider if available

### Participant Roles Used

- Analyst: Data-driven, evidence-focused perspective
- Proposer: Creative, innovative solutions
- Critic: Challenge assumptions, identify weaknesses
- Mediator: Find common ground, practical solutions
- Debater: Balanced argumentation
- Opponent: Counter-arguments and alternatives
- Moderator: Keep discussion focused
- Strategist: Long-term implications

## Results Location

\`\`\`
$RESULTS_DIR/
├── logs/
│   ├── single_provider_challenge.log
│   ├── debate_output.log
│   └── errors.log
└── results/
    ├── provider_discovery.json
    ├── debate_3p.json
    ├── debate_5p.json
    ├── debate_custom.json
    ├── debate_tech.json
    ├── debate_ethics.json
    ├── diversity_analysis.json
    └── challenge_summary.json
\`\`\`

## Conclusion

$(if [ "$overall_success" = "true" ]; then
    echo "The single-provider multi-instance mode is working correctly."
    echo "SuperAgent can successfully conduct debates with diverse perspectives"
    echo "even when only one LLM provider is available."
else
    echo "Some tests did not pass. Review the debate outputs and diversity analysis"
    echo "for more details on what needs improvement."
fi)

---
*Generated by SuperAgent Single-Provider Challenge*
EOF

    # Print summary
    echo ""
    log_info "=========================================="
    log_info "  CHALLENGE SUMMARY"
    log_info "=========================================="
    echo ""
    log_info "Provider:         $SELECTED_PROVIDER"
    log_info "Tests Passed:     ${TESTS_PASSED:-0}/${TESTS_TOTAL:-0}"
    log_info "Avg Diversity:    $(printf "%.2f" $avg_diversity)"
    log_info "Avg Quality:      $(printf "%.2f" $avg_quality)"
    log_info "Overall:          $([ "$overall_success" = "true" ] && echo "${GREEN}PASSED${NC}" || echo "${RED}FAILED${NC}")"
    echo ""
    log_info "Results: $RESULTS_DIR"
    log_info "Report: $report_file"

    if [ "$overall_success" = "true" ]; then
        log_success "Single-Provider Challenge PASSED"
        return 0
    else
        log_warning "Single-Provider Challenge: needs attention"
        return 1
    fi
}

#===============================================================================
# MAIN EXECUTION
#===============================================================================

main() {
    # Parse arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            --provider)
                PROVIDER="$2"
                shift 2
                ;;
            --participants)
                NUM_PARTICIPANTS="$2"
                shift 2
                ;;
            --rounds)
                NUM_ROUNDS="$2"
                shift 2
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    START_TIME=$(date '+%Y-%m-%d %H:%M:%S')

    # Setup
    setup_directories
    load_environment

    log_phase "SINGLE-PROVIDER MULTI-INSTANCE CHALLENGE"
    log_info "Start time: $START_TIME"
    log_info "Results directory: $RESULTS_DIR"

    # Check/Start SuperAgent
    if ! check_superagent_running; then
        if ! start_superagent; then
            log_error "Failed to start SuperAgent"
            exit 1
        fi
    fi

    # Execute phases
    phase1_provider_discovery
    phase2_debate_tests
    phase3_diversity_analysis
    phase4_generate_report
}

main "$@"
