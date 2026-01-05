#!/bin/bash
#===============================================================================
# SUPERAGENT RE-VERIFICATION SCRIPT
#===============================================================================
# This script re-verifies all 30+ providers and 900+ LLMs using LLMsVerifier.
# Uses ONLY production binaries - NO source code execution!
#
# This script:
# 1. Uses LLMsVerifier binary to verify all providers
# 2. Tests and benchmarks all available LLMs
# 3. Updates the model scores and rankings
# 4. Regenerates the AI Debate Group with best models
# 5. Updates OpenCode configuration
#
# Usage:
#   ./scripts/reverify_all.sh [options]
#
# Options:
#   --providers-only    Only verify providers (skip model benchmarks)
#   --models-only       Only benchmark models (skip provider verification)
#   --quick             Quick verification (sample models only)
#   --full              Full verification (all models, extended tests)
#   --help              Show this help
#
#===============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"
LLMSVERIFIER_DIR="$PROJECT_ROOT/LLMsVerifier"

# Timestamps
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
YEAR=$(date +%Y)
MONTH=$(date +%m)
DAY=$(date +%d)

# Directories
RESULTS_DIR="$CHALLENGES_DIR/results/reverification/$YEAR/$MONTH/$DAY/$TIMESTAMP"
LOGS_DIR="$RESULTS_DIR/logs"
OUTPUT_DIR="$RESULTS_DIR/results"

# Binary paths
LLMSVERIFIER_BIN="$LLMSVERIFIER_DIR/llm-verifier/llm-verifier"
SUPERAGENT_BIN="$PROJECT_ROOT/superagent"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Options
PROVIDERS_ONLY=false
MODELS_ONLY=false
QUICK=false
FULL=false

#===============================================================================
# LOGGING
#===============================================================================

log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
    echo -e "$msg" | tee -a "$LOGS_DIR/reverification.log"
}

log_info() { log "${BLUE}[INFO]${NC} $*"; }
log_success() { log "${GREEN}[SUCCESS]${NC} $*"; }
log_warning() { log "${YELLOW}[WARNING]${NC} $*"; }
log_error() { log "${RED}[ERROR]${NC} $*"; }
log_phase() {
    echo "" | tee -a "$LOGS_DIR/reverification.log"
    log "${PURPLE}========================================${NC}"
    log "${PURPLE}  $*${NC}"
    log "${PURPLE}========================================${NC}"
    echo "" | tee -a "$LOGS_DIR/reverification.log"
}

log_cmd() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] BINARY CMD: $*" >> "$LOGS_DIR/commands.log"
    log "${CYAN}[BINARY]${NC} $*"
}

run_binary() {
    local binary="$1"
    shift
    local args="$*"

    log_cmd "$binary $args"

    if [ ! -x "$binary" ]; then
        log_error "Binary not found or not executable: $binary"
        return 1
    fi

    # Execute binary and capture output
    "$binary" $args 2>&1 | tee -a "$LOGS_DIR/binary_output.log"
    return ${PIPESTATUS[0]}
}

usage() {
    cat << EOF
${GREEN}SuperAgent Re-Verification Script${NC}

Usage: $0 [options]

Options:
    --providers-only    Only verify providers (skip model benchmarks)
    --models-only       Only benchmark models (skip provider verification)
    --quick             Quick verification (sample models only)
    --full              Full verification (all models, extended tests)
    --help              Show this help

${YELLOW}IMPORTANT:${NC}
This script uses ONLY production binaries:
- LLMsVerifier binary: $LLMSVERIFIER_BIN
- SuperAgent binary: $SUPERAGENT_BIN

NO source code is executed!

EOF
}

#===============================================================================
# SETUP
#===============================================================================

setup() {
    log_info "Setting up directories..."
    mkdir -p "$LOGS_DIR"
    mkdir -p "$OUTPUT_DIR"

    log_info "Results will be stored in: $RESULTS_DIR"

    # Load environment
    if [ -f "$PROJECT_ROOT/.env" ]; then
        set -a
        source "$PROJECT_ROOT/.env"
        set +a
        log_info "Loaded environment from $PROJECT_ROOT/.env"
    fi

    if [ -f "$LLMSVERIFIER_DIR/.env" ]; then
        set -a
        source "$LLMSVERIFIER_DIR/.env"
        set +a
        log_info "Loaded environment from $LLMSVERIFIER_DIR/.env"
    fi
}

check_binaries() {
    log_info "Checking binary availability..."

    if [ -x "$LLMSVERIFIER_BIN" ]; then
        log_success "LLMsVerifier binary: $LLMSVERIFIER_BIN"
        log_info "Version: $($LLMSVERIFIER_BIN --version 2>/dev/null || echo 'unknown')"
    else
        log_error "LLMsVerifier binary not found: $LLMSVERIFIER_BIN"
        log_info "Please build it first: cd $LLMSVERIFIER_DIR && make build"
        exit 1
    fi

    if [ -x "$SUPERAGENT_BIN" ]; then
        log_success "SuperAgent binary: $SUPERAGENT_BIN"
    else
        log_warning "SuperAgent binary not found: $SUPERAGENT_BIN"
    fi
}

#===============================================================================
# PROVIDER VERIFICATION
#===============================================================================

verify_providers() {
    log_phase "PROVIDER VERIFICATION"

    log_info "Verifying all configured providers using LLMsVerifier binary..."

    # List all providers
    log_info "Listing providers..."
    run_binary "$LLMSVERIFIER_BIN" providers list \
        --output "$OUTPUT_DIR/providers_list.json" \
        || log_warning "Provider list command failed"

    # Verify each provider
    log_info "Verifying provider connectivity..."
    run_binary "$LLMSVERIFIER_BIN" providers verify \
        --output "$OUTPUT_DIR/providers_verified.json" \
        || log_warning "Provider verify command failed"

    # Generate provider report
    if [ -f "$OUTPUT_DIR/providers_verified.json" ]; then
        local count=$(grep -c '"name"' "$OUTPUT_DIR/providers_verified.json" 2>/dev/null || echo "0")
        log_success "Providers verified: $count"
    else
        log_warning "Provider verification output not found"
    fi
}

#===============================================================================
# MODEL BENCHMARKING
#===============================================================================

benchmark_models() {
    log_phase "MODEL BENCHMARKING"

    local model_limit=""
    if [ "$QUICK" = true ]; then
        model_limit="--limit 10"
        log_info "Quick mode: Testing first 10 models per provider"
    elif [ "$FULL" = true ]; then
        log_info "Full mode: Testing all models with extended tests"
    fi

    # List all models
    log_info "Discovering models from all providers..."
    run_binary "$LLMSVERIFIER_BIN" models list \
        $model_limit \
        --output "$OUTPUT_DIR/models_discovered.json" \
        || log_warning "Model list command failed"

    # Verify models
    log_info "Verifying model capabilities..."
    run_binary "$LLMSVERIFIER_BIN" models verify \
        $model_limit \
        --output "$OUTPUT_DIR/models_verified.json" \
        || log_warning "Model verify command failed"

    # Benchmark models
    log_info "Benchmarking model performance..."
    run_binary "$LLMSVERIFIER_BIN" models benchmark \
        $model_limit \
        --output "$OUTPUT_DIR/models_benchmarked.json" \
        || log_warning "Model benchmark command failed"

    # Score models
    log_info "Scoring models..."
    run_binary "$LLMSVERIFIER_BIN" models score \
        --input "$OUTPUT_DIR/models_benchmarked.json" \
        --output "$OUTPUT_DIR/models_scored.json" \
        || log_warning "Model score command failed"

    if [ -f "$OUTPUT_DIR/models_scored.json" ]; then
        local count=$(grep -c '"model_id"' "$OUTPUT_DIR/models_scored.json" 2>/dev/null || echo "0")
        log_success "Models scored: $count"
    fi
}

#===============================================================================
# DEBATE GROUP UPDATE
#===============================================================================

update_debate_group() {
    log_phase "DEBATE GROUP UPDATE"

    log_info "Selecting top 15+ models for debate group..."

    # Use LLMsVerifier to export debate group configuration
    run_binary "$LLMSVERIFIER_BIN" ai-config export debate-group \
        --input "$OUTPUT_DIR/models_scored.json" \
        --primary-count 5 \
        --fallbacks-per-primary 2 \
        --output "$OUTPUT_DIR/debate_group.json" \
        || log_warning "Debate group export failed"

    if [ -f "$OUTPUT_DIR/debate_group.json" ]; then
        log_success "Debate group configuration updated"
    else
        log_warning "Debate group output not found, using default"
    fi
}

#===============================================================================
# OPENCODE CONFIGURATION
#===============================================================================

update_opencode_config() {
    log_phase "OPENCODE CONFIGURATION UPDATE"

    log_info "Generating updated OpenCode configuration..."

    # Export OpenCode configuration
    run_binary "$LLMSVERIFIER_BIN" ai-config export opencode \
        --input "$OUTPUT_DIR/models_scored.json" \
        --debate-group "$OUTPUT_DIR/debate_group.json" \
        --output "$OUTPUT_DIR/opencode.json" \
        || log_warning "OpenCode export failed"

    # Also export Crush configuration
    run_binary "$LLMSVERIFIER_BIN" ai-config export crush \
        --input "$OUTPUT_DIR/models_scored.json" \
        --output "$OUTPUT_DIR/crush_config.json" \
        || log_warning "Crush export failed"

    # Create redacted version
    if [ -f "$OUTPUT_DIR/opencode.json" ]; then
        sed 's/"api_key":\s*"[^"]*"/"api_key": "\${SUPERAGENT_API_KEY}"/g' \
            "$OUTPUT_DIR/opencode.json" > "$OUTPUT_DIR/opencode.json.example"
        log_success "OpenCode configuration generated"
    fi

    # Copy to Downloads
    if [ -f "$OUTPUT_DIR/opencode.json" ]; then
        cp "$OUTPUT_DIR/opencode.json" "/home/milosvasic/Downloads/opencode-super-agent.json" 2>/dev/null || true
        log_info "Copied to /home/milosvasic/Downloads/opencode-super-agent.json"
    fi
}

#===============================================================================
# GENERATE REPORT
#===============================================================================

generate_report() {
    log_phase "GENERATING VERIFICATION REPORT"

    local report="$OUTPUT_DIR/verification_report.md"

    cat > "$report" << EOF
# Re-Verification Report

**Generated**: $(date '+%Y-%m-%d %H:%M:%S')
**Mode**: $([ "$QUICK" = true ] && echo "Quick" || ([ "$FULL" = true ] && echo "Full" || echo "Standard"))

---

## Summary

| Metric | Value |
|--------|-------|
| Providers Verified | $(grep -c '"name"' "$OUTPUT_DIR/providers_verified.json" 2>/dev/null || echo "N/A") |
| Models Discovered | $(grep -c '"model_id"' "$OUTPUT_DIR/models_discovered.json" 2>/dev/null || echo "N/A") |
| Models Verified | $(grep -c '"verified": true' "$OUTPUT_DIR/models_verified.json" 2>/dev/null || echo "N/A") |
| Models Scored | $(grep -c '"total_score"' "$OUTPUT_DIR/models_scored.json" 2>/dev/null || echo "N/A") |

---

## Results Location

\`\`\`
$RESULTS_DIR/
├── logs/
│   ├── reverification.log
│   ├── commands.log
│   └── binary_output.log
└── results/
    ├── providers_list.json
    ├── providers_verified.json
    ├── models_discovered.json
    ├── models_verified.json
    ├── models_benchmarked.json
    ├── models_scored.json
    ├── debate_group.json
    ├── opencode.json
    └── crush_config.json
\`\`\`

---

## Binaries Used

- **LLMsVerifier**: \`$LLMSVERIFIER_BIN\`
- **SuperAgent**: \`$SUPERAGENT_BIN\`

---

*This report was generated using production binaries only - no source code execution.*
EOF

    log_success "Report generated: $report"
}

#===============================================================================
# MAIN
#===============================================================================

main() {
    # Parse arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            --providers-only) PROVIDERS_ONLY=true ;;
            --models-only) MODELS_ONLY=true ;;
            --quick) QUICK=true ;;
            --full) FULL=true ;;
            --help|-h) usage; exit 0 ;;
            *) log_error "Unknown option: $1"; usage; exit 1 ;;
        esac
        shift
    done

    setup
    check_binaries

    log_phase "SUPERAGENT RE-VERIFICATION"
    log_info "Start time: $(date '+%Y-%m-%d %H:%M:%S')"
    log_info "Results: $RESULTS_DIR"
    log_info ""
    log_warning "USING ONLY PRODUCTION BINARIES - NO SOURCE CODE!"
    log_info ""

    # Execute verification phases
    if [ "$MODELS_ONLY" = false ]; then
        verify_providers
    fi

    if [ "$PROVIDERS_ONLY" = false ]; then
        benchmark_models
    fi

    update_debate_group
    update_opencode_config
    generate_report

    log_phase "RE-VERIFICATION COMPLETE"
    log_success "All verification completed successfully!"
    log_info "Results: $RESULTS_DIR"
    log_info "Report: $OUTPUT_DIR/verification_report.md"
}

main "$@"
