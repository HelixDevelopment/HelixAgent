#!/bin/bash
# HelixAgent Challenges - Monitored Challenge Execution
# Runs all challenges with comprehensive system monitoring
#
# Features:
# - Real-time resource monitoring (CPU, memory, disk, network)
# - Log collection from all components
# - Memory leak detection
# - Warning/error detection and analysis
# - Comprehensive report generation
# - Automatic issue investigation
#
# Usage: ./run_monitored_challenges.sh [options]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$SCRIPT_DIR/lib"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Source monitoring library
source "$LIB_DIR/monitoring_lib.sh"
source "$LIB_DIR/report_generator.sh"

# Colors (for this script)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

print_banner() {
    echo -e "${CYAN}"
    echo "╔══════════════════════════════════════════════════════════════════════════════╗"
    echo "║                                                                              ║"
    echo "║   ██╗  ██╗███████╗██╗     ██╗██╗  ██╗ █████╗  ██████╗ ███████╗███╗   ██╗████████╗   ║"
    echo "║   ██║  ██║██╔════╝██║     ██║╚██╗██╔╝██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝   ║"
    echo "║   ███████║█████╗  ██║     ██║ ╚███╔╝ ███████║██║  ███╗█████╗  ██╔██╗ ██║   ██║      ║"
    echo "║   ██╔══██║██╔══╝  ██║     ██║ ██╔██╗ ██╔══██║██║   ██║██╔══╝  ██║╚██╗██║   ██║      ║"
    echo "║   ██║  ██║███████╗███████╗██║██╔╝ ██╗██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║      ║"
    echo "║   ╚═╝  ╚═╝╚══════╝╚══════╝╚═╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝      ║"
    echo "║                                                                              ║"
    echo "║              MONITORED CHALLENGE EXECUTION SYSTEM v1.0.0                     ║"
    echo "║                                                                              ║"
    echo "╠══════════════════════════════════════════════════════════════════════════════╣"
    echo "║  Comprehensive monitoring with:                                              ║"
    echo "║    • Real-time resource monitoring (CPU, memory, disk, network)              ║"
    echo "║    • Log collection from all components                                      ║"
    echo "║    • Memory leak detection                                                   ║"
    echo "║    • Warning/error detection and analysis                                    ║"
    echo "║    • Automatic issue investigation                                           ║"
    echo "║    • Comprehensive HTML/JSON reports                                         ║"
    echo "╚══════════════════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_phase() { echo -e "${PURPLE}[PHASE]${NC} $1"; }
print_monitor() { echo -e "${CYAN}[MONITOR]${NC} $1"; }

#===============================================================================
# CONFIGURATION
#===============================================================================

# Challenge list (same as run_all_challenges.sh)
CHALLENGES=(
    "health_monitoring"
    "configuration_loading"
    "caching_layer"
    "database_operations"
    "authentication"
    "plugin_system"
    "rate_limiting"
    "input_validation"
    "provider_claude"
    "provider_deepseek"
    "provider_gemini"
    "provider_ollama"
    "provider_openrouter"
    "provider_qwen"
    "provider_zai"
    "provider_verification"
    "mcp_protocol"
    "lsp_protocol"
    "acp_protocol"
    "cloud_aws_bedrock"
    "cloud_gcp_vertex"
    "cloud_azure_openai"
    "ensemble_voting"
    "embeddings_service"
    "streaming_responses"
    "model_metadata"
    "ai_debate_formation"
    "ai_debate_workflow"
    "openai_compatibility"
    "grpc_api"
    "api_quality_test"
    "optimization_semantic_cache"
    "optimization_structured_output"
    "cognee_integration"
    "circuit_breaker"
    "error_handling"
    "concurrent_access"
    "graceful_shutdown"
    "session_management"
    "opencode"
    "opencode_init"
    "protocol_challenge"
    "curl_api_challenge"
    "cli_agents_challenge"
    "content_generation_challenge"
)

# Options
VERBOSE=false
STOP_ON_FAILURE=true
SKIP_INFRA=false
GENERATE_REPORT=true
MEMORY_CHECK_INTERVAL=10  # Check memory every N challenges
INVESTIGATE_ISSUES=true

#===============================================================================
# PARSE ARGUMENTS
#===============================================================================

while [ $# -gt 0 ]; do
    case "$1" in
        -v|--verbose)
            VERBOSE=true
            ;;
        --continue-on-failure)
            STOP_ON_FAILURE=false
            ;;
        --skip-infra)
            SKIP_INFRA=true
            ;;
        --no-report)
            GENERATE_REPORT=false
            ;;
        --no-investigate)
            INVESTIGATE_ISSUES=false
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -v, --verbose           Enable verbose output"
            echo "  --continue-on-failure   Continue even if a challenge fails"
            echo "  --skip-infra            Skip infrastructure startup"
            echo "  --no-report             Skip report generation"
            echo "  --no-investigate        Skip automatic issue investigation"
            echo "  -h, --help              Show this help"
            exit 0
            ;;
        *)
            print_warning "Unknown option: $1"
            ;;
    esac
    shift
done

#===============================================================================
# TRACKED ISSUES FOR INVESTIGATION
#===============================================================================

declare -a DETECTED_ISSUES=()
declare -a APPLIED_FIXES=()

record_detected_issue() {
    local severity="$1"
    local component="$2"
    local description="$3"

    DETECTED_ISSUES+=("[$severity] $component: $description")
    mon_record_issue "$severity" "$component" "$description" ""
}

record_applied_fix() {
    local issue="$1"
    local fix="$2"
    local test="$3"

    APPLIED_FIXES+=("$issue -> $fix (test: $test)")
    mon_record_fix "$issue" "$fix" "$test"
}

#===============================================================================
# INFRASTRUCTURE
#===============================================================================

HELIXAGENT_PID=""
STARTED_SERVICES=()

cleanup() {
    print_info "Cleaning up..."

    # Stop background monitoring
    mon_stop_background_monitoring 2>/dev/null || true

    # Stop HelixAgent if we started it
    if [ -n "$HELIXAGENT_PID" ] && kill -0 "$HELIXAGENT_PID" 2>/dev/null; then
        print_info "Stopping HelixAgent (PID: $HELIXAGENT_PID)..."
        kill "$HELIXAGENT_PID" 2>/dev/null || true
        wait "$HELIXAGENT_PID" 2>/dev/null || true
    fi

    rm -f "$CHALLENGES_DIR/results/helixagent_challenges.pid"
}

trap cleanup EXIT

build_helixagent() {
    print_phase "Building HelixAgent Binary"

    local binary=""
    if [ -x "$PROJECT_ROOT/bin/helixagent" ]; then
        binary="$PROJECT_ROOT/bin/helixagent"
    elif [ -x "$PROJECT_ROOT/helixagent" ]; then
        binary="$PROJECT_ROOT/helixagent"
    fi

    if [ -n "$binary" ]; then
        print_success "HelixAgent binary found: $binary"
        return 0
    fi

    print_info "Building HelixAgent..."
    if (cd "$PROJECT_ROOT" && make build 2>&1); then
        if [ -x "$PROJECT_ROOT/bin/helixagent" ] || [ -x "$PROJECT_ROOT/helixagent" ]; then
            print_success "HelixAgent built successfully"
            return 0
        fi
    fi

    print_error "Failed to build HelixAgent"
    return 1
}

check_helixagent() {
    local port="${HELIXAGENT_PORT:-7061}"
    local host="${HELIXAGENT_HOST:-localhost}"

    curl -s "http://$host:$port/health" > /dev/null 2>&1
}

start_helixagent() {
    print_phase "Starting HelixAgent Server"

    local port="${HELIXAGENT_PORT:-7061}"
    local host="${HELIXAGENT_HOST:-localhost}"

    if check_helixagent; then
        print_success "HelixAgent already running on $host:$port"

        # Track existing HelixAgent process
        local existing_pid=$(pgrep -f "helixagent" | head -1)
        if [ -n "$existing_pid" ]; then
            mon_track_process "helixagent" "$existing_pid"
        fi
        return 0
    fi

    local binary=""
    if [ -x "$PROJECT_ROOT/bin/helixagent" ]; then
        binary="$PROJECT_ROOT/bin/helixagent"
    elif [ -x "$PROJECT_ROOT/helixagent" ]; then
        binary="$PROJECT_ROOT/helixagent"
    fi

    if [ -z "$binary" ]; then
        print_error "HelixAgent binary not found"
        return 1
    fi

    print_info "Starting HelixAgent from: $binary"

    mkdir -p "$CHALLENGES_DIR/results"

    nohup "$binary" > "$CHALLENGES_DIR/results/helixagent_challenges.log" 2>&1 &
    HELIXAGENT_PID=$!
    echo $HELIXAGENT_PID > "$CHALLENGES_DIR/results/helixagent_challenges.pid"
    STARTED_SERVICES+=("helixagent")

    # Track the new process
    mon_track_process "helixagent" "$HELIXAGENT_PID"

    print_info "Waiting for HelixAgent to start..."
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if check_helixagent; then
            print_success "HelixAgent started successfully (PID: $HELIXAGENT_PID)"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done

    print_error "HelixAgent failed to start within ${max_attempts}s"
    cat "$CHALLENGES_DIR/results/helixagent_challenges.log" | tail -20
    return 1
}

start_infrastructure() {
    print_info "=========================================="
    print_info "  Starting Infrastructure"
    print_info "=========================================="
    echo ""

    if ! build_helixagent; then
        print_error "Failed to build HelixAgent - cannot continue"
        record_detected_issue "CRITICAL" "build" "Failed to build HelixAgent binary"
        exit 1
    fi

    if ! start_helixagent; then
        print_error "Failed to start HelixAgent - cannot continue"
        record_detected_issue "CRITICAL" "startup" "Failed to start HelixAgent server"
        exit 1
    fi

    echo ""
    print_success "All infrastructure started successfully"
    echo ""
}

#===============================================================================
# ISSUE INVESTIGATION
#===============================================================================

investigate_log_errors() {
    print_monitor "Investigating detected errors..."

    local error_count=0
    local investigated=0

    # Check HelixAgent logs for specific error patterns
    local log_file="$CHALLENGES_DIR/results/helixagent_challenges.log"
    if [ -f "$log_file" ]; then
        # Check for tool_choice format errors
        if grep -q "tool_choice.*valid dictionary" "$log_file" 2>/dev/null; then
            record_detected_issue "ERROR" "claude_provider" "tool_choice format error - string instead of object"
            # This was already fixed, but record if it reappears
            error_count=$((error_count + 1))
            investigated=$((investigated + 1))
        fi

        # Check for nil pointer dereference
        if grep -q "nil pointer dereference" "$log_file" 2>/dev/null; then
            record_detected_issue "ERROR" "runtime" "nil pointer dereference detected"
            error_count=$((error_count + 1))
        fi

        # Check for connection refused
        if grep -q "connection refused" "$log_file" 2>/dev/null; then
            record_detected_issue "WARNING" "network" "connection refused - service may be down"
            error_count=$((error_count + 1))
        fi

        # Check for context deadline exceeded
        if grep -q "context deadline exceeded" "$log_file" 2>/dev/null; then
            record_detected_issue "WARNING" "timeout" "context deadline exceeded - slow response"
            error_count=$((error_count + 1))
        fi

        # Check for OAuth token issues
        if grep -q "token expired\|invalid token\|unauthorized" "$log_file" 2>/dev/null; then
            record_detected_issue "WARNING" "auth" "OAuth token issue detected"
            error_count=$((error_count + 1))
        fi
    fi

    print_monitor "Investigated $investigated issues, found $error_count total errors"
    return $error_count
}

perform_memory_check() {
    print_monitor "Performing memory leak check..."

    if mon_detect_memory_leaks; then
        print_success "No memory leaks detected"
    else
        print_warning "Potential memory leak detected!"
        record_detected_issue "WARNING" "memory" "Potential memory leak detected during challenge execution"
    fi
}

#===============================================================================
# CHALLENGE EXECUTION
#===============================================================================

run_single_challenge() {
    local challenge="$1"
    local challenge_num="$2"
    local total_challenges="$3"

    print_info "----------------------------------------"
    print_info "[$challenge_num/$total_challenges] Running: $challenge"
    print_info "----------------------------------------"

    mon_log "INFO" "Starting challenge: $challenge"

    local start_time=$(date +%s)
    local result=0
    local verbose_flag=""
    [ "$VERBOSE" = true ] && verbose_flag="--verbose"

    # Run the challenge
    if "$CHALLENGES_DIR/scripts/run_challenges.sh" "$challenge" $verbose_flag 2>&1; then
        result=0
        print_success "$challenge completed successfully"
        mon_log "INFO" "Challenge passed: $challenge"
    else
        result=1
        print_error "$challenge failed"
        mon_log "ERROR" "Challenge failed: $challenge"
        record_detected_issue "ERROR" "challenge" "Challenge $challenge failed"
    fi

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    # Log challenge result
    mon_log_json "challenge_result" "\"challenge\":\"$challenge\",\"status\":$([ $result -eq 0 ] && echo '\"pass\"' || echo '\"fail\"'),\"duration\":$duration"

    # Periodic memory check
    if [ $((challenge_num % MEMORY_CHECK_INTERVAL)) -eq 0 ]; then
        perform_memory_check
    fi

    # Investigate any errors after each challenge
    if [ "$INVESTIGATE_ISSUES" = true ]; then
        investigate_log_errors
    fi

    return $result
}

#===============================================================================
# MAIN EXECUTION
#===============================================================================

main() {
    print_banner

    print_info "Start time: $(date)"
    print_info "Total challenges: ${#CHALLENGES[@]}"
    print_info "Stop on failure: $STOP_ON_FAILURE"
    print_info "Generate report: $GENERATE_REPORT"
    echo ""

    # Initialize monitoring
    print_phase "Initializing Monitoring System"
    mon_init "challenges"

    # Start background resource monitoring
    mon_start_background_monitoring

    # Collect initial logs
    mon_collect_all_logs

    # Start infrastructure unless skipped
    if [ "$SKIP_INFRA" = false ]; then
        start_infrastructure
    else
        print_warning "Skipping infrastructure startup (--skip-infra)"
        if ! check_helixagent; then
            print_error "HelixAgent is not running! Cannot continue."
            record_detected_issue "CRITICAL" "infrastructure" "HelixAgent not running and --skip-infra specified"
            mon_finalize 1
            exit 1
        fi
        print_success "HelixAgent is running"

        # Track existing process
        local existing_pid=$(pgrep -f "helixagent" | head -1)
        if [ -n "$existing_pid" ]; then
            mon_track_process "helixagent" "$existing_pid"
        fi
    fi

    echo ""
    print_phase "Running Challenges with Monitoring"
    echo ""

    local total_start=$(date +%s)
    local passed=0
    local failed=0
    local challenge_num=0

    for challenge in "${CHALLENGES[@]}"; do
        challenge_num=$((challenge_num + 1))

        if run_single_challenge "$challenge" "$challenge_num" "${#CHALLENGES[@]}"; then
            passed=$((passed + 1))
        else
            failed=$((failed + 1))

            if [ "$STOP_ON_FAILURE" = true ]; then
                print_error "Stopping due to failure. Use --continue-on-failure to continue."
                break
            fi
        fi
        echo ""
    done

    local total_end=$(date +%s)
    local total_duration=$((total_end - total_start))

    # Final log collection
    print_phase "Collecting Final Logs"
    mon_collect_all_logs

    # Final analysis
    print_phase "Final Analysis"
    mon_analyze_all_logs
    perform_memory_check
    mon_check_file_descriptors
    mon_check_goroutine_leaks

    # Finalize monitoring
    local exit_code=0
    [ "$failed" -gt 0 ] && exit_code=1
    mon_finalize $exit_code

    # Generate comprehensive report
    if [ "$GENERATE_REPORT" = true ]; then
        print_phase "Generating Comprehensive Report"
        generate_comprehensive_report "$MON_LOG_DIR" "$MON_REPORT_DIR"
    fi

    # Print final summary
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║              MONITORED CHALLENGE EXECUTION COMPLETE              ║${NC}"
    echo -e "${CYAN}╠══════════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${CYAN}║${NC} Duration:        ${total_duration}s"
    echo -e "${CYAN}║${NC} Challenges Run:  $challenge_num / ${#CHALLENGES[@]}"

    if [ "$passed" -gt 0 ]; then
        echo -e "${CYAN}║${NC} ${GREEN}Passed:          $passed${NC}"
    fi

    if [ "$failed" -gt 0 ]; then
        echo -e "${CYAN}║${NC} ${RED}Failed:          $failed${NC}"
    fi

    echo -e "${CYAN}╠══════════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${CYAN}║${NC} ${MON_RED}Errors:          $MON_ERRORS_COUNT${NC}"
    echo -e "${CYAN}║${NC} ${MON_YELLOW}Warnings:        $MON_WARNINGS_COUNT${NC}"
    echo -e "${CYAN}║${NC} ${MON_GREEN}Fixes Applied:   $MON_FIXES_COUNT${NC}"
    echo -e "${CYAN}╠══════════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${CYAN}║${NC} Session ID:      $MON_SESSION_ID"
    echo -e "${CYAN}║${NC} Logs:            $MON_LOG_DIR"
    echo -e "${CYAN}║${NC} Report:          $MON_REPORT_DIR/report.html"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════════╝${NC}"

    # List detected issues
    if [ ${#DETECTED_ISSUES[@]} -gt 0 ]; then
        echo ""
        echo -e "${YELLOW}Detected Issues:${NC}"
        for issue in "${DETECTED_ISSUES[@]}"; do
            echo -e "  ${RED}•${NC} $issue"
        done
    fi

    # List applied fixes
    if [ ${#APPLIED_FIXES[@]} -gt 0 ]; then
        echo ""
        echo -e "${GREEN}Applied Fixes:${NC}"
        for fix in "${APPLIED_FIXES[@]}"; do
            echo -e "  ${GREEN}•${NC} $fix"
        done
    fi

    # Final status
    echo ""
    if [ "$failed" -eq 0 ] && [ "$MON_ERRORS_COUNT" -eq 0 ]; then
        echo -e "${GREEN}╔══════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║     ALL CHALLENGES PASSED - NO ERRORS OR WARNINGS DETECTED       ║${NC}"
        echo -e "${GREEN}╚══════════════════════════════════════════════════════════════════╝${NC}"
        exit 0
    elif [ "$failed" -eq 0 ] && [ "$MON_WARNINGS_COUNT" -gt 0 ]; then
        echo -e "${YELLOW}╔══════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${YELLOW}║     ALL CHALLENGES PASSED - BUT WARNINGS WERE DETECTED          ║${NC}"
        echo -e "${YELLOW}╚══════════════════════════════════════════════════════════════════╝${NC}"
        exit 0
    else
        echo -e "${RED}╔══════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${RED}║     CHALLENGES FAILED OR ERRORS DETECTED - SEE REPORT           ║${NC}"
        echo -e "${RED}╚══════════════════════════════════════════════════════════════════╝${NC}"
        exit 1
    fi
}

# Run main
main
