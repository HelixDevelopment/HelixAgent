#!/bin/bash
# Challenge Framework - Common functions for all challenges
# BINARY ONLY - NO SOURCE CODE EXECUTION

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Globals
CHALLENGE_ID=""
CHALLENGE_NAME=""
OUTPUT_DIR=""
LOG_FILE=""
RESULTS_FILE=""
START_TIME=""
HELIXAGENT_PID=""
PROJECT_ROOT=""

# Initialize challenge
init_challenge() {
    CHALLENGE_ID="$1"
    CHALLENGE_NAME="$2"

    # Get project root
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

    # Create output directory with timestamp
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    OUTPUT_DIR="$PROJECT_ROOT/challenges/results/${CHALLENGE_ID}/${TIMESTAMP}"
    mkdir -p "$OUTPUT_DIR/logs" "$OUTPUT_DIR/results"

    LOG_FILE="$OUTPUT_DIR/logs/${CHALLENGE_ID}.log"
    RESULTS_FILE="$OUTPUT_DIR/results/${CHALLENGE_ID}_results.json"
    START_TIME=$(date +%s)

    # Initialize results JSON
    echo "{\"challenge_id\": \"$CHALLENGE_ID\", \"name\": \"$CHALLENGE_NAME\", \"status\": \"running\", \"start_time\": \"$(date -Iseconds)\", \"assertions\": [], \"metrics\": {}}" > "$RESULTS_FILE"

    log_info "Starting challenge: $CHALLENGE_NAME"
    log_info "Output directory: $OUTPUT_DIR"
}

# Logging functions
log_info() {
    local msg="[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${BLUE}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

log_success() {
    local msg="[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${GREEN}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

log_warning() {
    local msg="[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${YELLOW}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

log_section() {
    local msg="=== $1 ==="
    echo ""
    echo -e "${CYAN}$msg${NC}"
    if [[ -n "$LOG_FILE" ]] && [[ -f "$LOG_FILE" ]]; then
        echo "" >> "$LOG_FILE"
        echo "$msg" >> "$LOG_FILE"
    fi
}

log_pass() {
    local msg="[PASS] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${GREEN}$msg${NC}"
    if [[ -n "$LOG_FILE" ]] && [[ -f "$LOG_FILE" ]]; then
        echo "$msg" >> "$LOG_FILE"
    fi
}

log_fail() {
    local msg="[FAIL] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${RED}$msg${NC}"
    if [[ -n "$LOG_FILE" ]] && [[ -f "$LOG_FILE" ]]; then
        echo "$msg" >> "$LOG_FILE"
    fi
}

log_error() {
    local msg="[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${RED}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

# Load environment variables.
# Uses `set +u` while sourcing because production .env files can contain
# bash-variable substitutions like `FOO=$Bar` where `Bar` is intentionally
# unset on this host (treated as empty by HelixAgent's env loader). Under
# `set -u`, sourcing those lines would fatally fail. We restore the
# caller's `-u` setting after.
load_env() {
    local env_file="$PROJECT_ROOT/.env"
    local prev_u
    case "$-" in *u*) prev_u=1 ;; *) prev_u=0 ;; esac
    set +u
    if [[ -f "$env_file" ]]; then
        set -a
        source "$env_file"
        set +a
        log_info "Loaded environment from $env_file"
    else
        env_file="$PROJECT_ROOT/challenges/.env"
        if [[ -f "$env_file" ]]; then
            set -a
            source "$env_file"
            set +a
            log_info "Loaded environment from $env_file"
        else
            log_warning "No .env file found"
        fi
    fi
    [[ "$prev_u" == "1" ]] && set -u
    return 0
}

# Detect container runtime
detect_container_runtime() {
    if command -v docker &> /dev/null && docker ps &> /dev/null; then
        echo "docker"
    elif command -v podman &> /dev/null && podman ps &> /dev/null; then
        echo "podman"
    else
        log_error "No container runtime (docker/podman) available"
        return 1
    fi
}

# Check binary exists
check_binary() {
    local binary="$1"
    local name="$2"

    if [[ ! -x "$binary" ]]; then
        log_error "$name binary not found or not executable: $binary"
        return 1
    fi
    log_info "$name binary found: $binary"
    return 0
}

# Get HelixAgent binary path
get_helixagent_binary() {
    local binary="$PROJECT_ROOT/helixagent"
    if [[ ! -x "$binary" ]]; then
        binary="$PROJECT_ROOT/bin/helixagent"
    fi
    if [[ ! -x "$binary" ]]; then
        log_error "HelixAgent binary not found. Run: make build"
        return 1
    fi
    echo "$binary"
}

# Get LLMsVerifier binary path
get_verifier_binary() {
    local binary="$PROJECT_ROOT/LLMsVerifier/llm-verifier/llm-verifier"
    if [[ ! -x "$binary" ]]; then
        binary="$PROJECT_ROOT/LLMsVerifier/bin/llm-verifier"
    fi
    if [[ ! -x "$binary" ]]; then
        log_error "LLMsVerifier binary not found. Run: cd LLMsVerifier && make build"
        return 1
    fi
    echo "$binary"
}

# Start HelixAgent
start_helixagent() {
    # Default port: HELIXAGENT_PORT, then 8100 (canonical per
    # docs/development/port-registry.md). The previous default of 7061
    # was stale and caused start_helixagent to spawn a NEW instance on
    # an unused port even when one was already running on 8100,
    # producing false-positive "HelixAgent stopped" cleanup messages
    # observed in the 2026-04-29 sweep.
    local port="${1:-${HELIXAGENT_PORT:-8100}}"
    local config="${2:-$PROJECT_ROOT/configs/production.yaml}"

    local binary=$(get_helixagent_binary) || return 1

    log_info "Starting HelixAgent on port $port..."

    # Check if already running
    if curl -s --max-time 60 "http://localhost:$port/health" > /dev/null 2>&1; then
        log_info "HelixAgent already running on port $port"
        return 0
    fi

    # Start HelixAgent
    PORT=$port "$binary" > "$OUTPUT_DIR/logs/helixagent.log" 2>&1 &
    HELIXAGENT_PID=$!
    echo "$HELIXAGENT_PID" > "$OUTPUT_DIR/helixagent.pid"

    # Wait for startup
    local max_wait=30
    local waited=0
    while ! curl -s --max-time 60 "http://localhost:$port/health" > /dev/null 2>&1; do
        sleep 1
        waited=$((waited + 1))
        if [[ $waited -ge $max_wait ]]; then
            log_error "HelixAgent failed to start within ${max_wait}s"
            return 1
        fi
    done

    log_success "HelixAgent started (PID: $HELIXAGENT_PID)"
    return 0
}

# Stop HelixAgent
stop_helixagent() {
    if [[ -n "$HELIXAGENT_PID" ]] && kill -0 "$HELIXAGENT_PID" 2>/dev/null; then
        log_info "Stopping HelixAgent (PID: $HELIXAGENT_PID)..."
        kill "$HELIXAGENT_PID" 2>/dev/null || true
        wait "$HELIXAGENT_PID" 2>/dev/null || true
        log_info "HelixAgent stopped"
    fi

    # Also check for pid file
    if [[ -f "$OUTPUT_DIR/helixagent.pid" ]]; then
        local pid=$(cat "$OUTPUT_DIR/helixagent.pid")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
        fi
        rm -f "$OUTPUT_DIR/helixagent.pid"
    fi
}

# Start infrastructure (Docker/Podman)
start_infrastructure() {
    local runtime=$(detect_container_runtime) || return 1
    local compose_file="$PROJECT_ROOT/docker-compose.yml"

    if [[ ! -f "$compose_file" ]]; then
        log_warning "No docker-compose.yml found, skipping infrastructure"
        return 0
    fi

    log_info "Starting infrastructure with $runtime..."

    if [[ "$runtime" == "docker" ]]; then
        docker-compose -f "$compose_file" up -d postgres redis 2>&1 | tee -a "$LOG_FILE"
    else
        podman-compose -f "$compose_file" up -d postgres redis 2>&1 | tee -a "$LOG_FILE"
    fi

    # Wait for services
    sleep 5
    log_success "Infrastructure started"
}

# Stop infrastructure
stop_infrastructure() {
    local runtime=$(detect_container_runtime) || return 0
    local compose_file="$PROJECT_ROOT/docker-compose.yml"

    if [[ -f "$compose_file" ]]; then
        log_info "Stopping infrastructure..."
        if [[ "$runtime" == "docker" ]]; then
            docker-compose -f "$compose_file" down 2>&1 | tee -a "$LOG_FILE" || true
        else
            podman-compose -f "$compose_file" down 2>&1 | tee -a "$LOG_FILE" || true
        fi
    fi
}

# Make API request to HelixAgent
api_request() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local port="${HELIXAGENT_PORT:-8100}"

    local url="http://localhost:$port$endpoint"
    local response_file="$OUTPUT_DIR/logs/api_response_$(date +%s%N).json"

    if [[ -n "$data" ]]; then
        curl -s --max-time 60 -X "$method" "$url" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$data" \
            -o "$response_file" \
            -w "%{http_code}"
    else
        curl -s --max-time 60 -X "$method" "$url" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -o "$response_file" \
            -w "%{http_code}"
    fi

    local http_code=$?
    cat "$response_file"
    return $http_code
}

# Assertion tracking (no jq dependency)
ASSERTION_LOG=""
METRIC_LOG=""

# Record assertion result
record_assertion() {
    local assertion_type="$1"
    local target="$2"
    local passed="$3"
    local message="$4"

    local status="PASSED"
    [[ "$passed" != "true" ]] && status="FAILED"

    # Append to assertion log (simple text format)
    echo "${assertion_type}|${target}|${status}|${message}" >> "$OUTPUT_DIR/logs/assertions.log"
    ASSERTION_LOG="${ASSERTION_LOG}${assertion_type}|${target}|${status}|${message}\n"

    if [[ "$status" == "PASSED" ]]; then
        log_success "Assertion $assertion_type ($target): PASSED"
    else
        log_error "Assertion $assertion_type ($target): FAILED - $message"
    fi
}

# Record metric
record_metric() {
    local name="$1"
    local value="$2"

    # Append to metric log
    echo "${name}=${value}" >> "$OUTPUT_DIR/logs/metrics.log"
    METRIC_LOG="${METRIC_LOG}${name}=${value}\n"

    log_info "Metric $name: $value"
}

# Finalize challenge
finalize_challenge() {
    local status="$1"
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))

    # Count passed/failed assertions from log file.
    # Drainage report 2026-04-25 Finding #16: the original
    #   passed=$(grep -c ... || echo 0)
    # produced multi-line output ("0\n0") in some shell-mode interactions
    # when grep had zero matches (grep -c outputs "0" + exits 1; the OR
    # then ALSO emits "0", and the outer command-substitution preserves
    # the inner newline). Interpolated into the JSON heredoc this produced
    # invalid JSON like:
    #   "assertions_failed": 0
    #   0
    #   }
    # making the file unparseable, breaking the master-summary aggregator
    # and any downstream automation. Confirmed in production output:
    #   xxd → "_failed": 0.0.}.   (two zeros separated by newline)
    # Fix: capture grep output once with set +e so the OR fallback is not
    # needed; default to 0 only if the result is empty.
    local passed=0
    local failed=0
    if [[ -f "$OUTPUT_DIR/logs/assertions.log" ]]; then
        set +e
        passed=$(grep -c "|PASSED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null)
        failed=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null)
        set -e
        [[ -z "$passed" ]] && passed=0
        [[ -z "$failed" ]] && failed=0
    fi

    # Create results JSON (simple format)
    cat > "$RESULTS_FILE" << EOF
{
  "challenge_id": "$CHALLENGE_ID",
  "name": "$CHALLENGE_NAME",
  "status": "$status",
  "start_time": "$(date -d "@$START_TIME" -Iseconds 2>/dev/null || date -Iseconds)",
  "end_time": "$(date -Iseconds)",
  "duration_seconds": $duration,
  "assertions_passed": $passed,
  "assertions_failed": $failed
}
EOF

    # Create summary report
    local report_file="$OUTPUT_DIR/results/${CHALLENGE_ID}_report.md"
    cat > "$report_file" << EOF
# Challenge Report: $CHALLENGE_NAME

**Challenge ID:** $CHALLENGE_ID
**Status:** $status
**Duration:** ${duration}s
**Timestamp:** $(date -Iseconds)

## Assertions

| Type | Target | Status |
|------|--------|--------|
EOF

    # Add assertions from log file
    if [[ -f "$OUTPUT_DIR/logs/assertions.log" ]]; then
        while IFS='|' read -r type target assertion_status message; do
            echo "| $type | $target | $assertion_status |" >> "$report_file"
        done < "$OUTPUT_DIR/logs/assertions.log"
    fi

    cat >> "$report_file" << EOF

**Passed:** $passed
**Failed:** $failed

## Metrics

EOF

    # Add metrics from log file
    if [[ -f "$OUTPUT_DIR/logs/metrics.log" ]]; then
        while IFS='=' read -r name value; do
            echo "- **$name:** $value" >> "$report_file"
        done < "$OUTPUT_DIR/logs/metrics.log"
    fi

    cat >> "$report_file" << EOF

## Output Directory

\`$OUTPUT_DIR\`
EOF

    # Create symlink to latest
    mkdir -p "$PROJECT_ROOT/challenges/results/${CHALLENGE_ID}"
    ln -sf "$OUTPUT_DIR" "$PROJECT_ROOT/challenges/results/${CHALLENGE_ID}/latest"

    if [[ "$status" == "PASSED" ]]; then
        log_success "Challenge $CHALLENGE_NAME completed: $status (${duration}s)"
        log_success "Assertions: $passed passed, $failed failed"
    else
        log_error "Challenge $CHALLENGE_NAME completed: $status (${duration}s)"
        log_error "Assertions: $passed passed, $failed failed"
    fi

    log_info "Results: $RESULTS_FILE"
    log_info "Report: $report_file"

    # Drainage report 2026-04-25 Finding #18: propagate status as exit code
    # so callers (and `set -e`) see real PASS/FAIL. Previously this function
    # always returned 0, causing standalone challenge scripts to exit 0
    # even when assertions failed — which then bubbled up through
    # generic_challenge.sh's meta-layer as "Challenge ... PASSED" and
    # produced false-green tallies like "498/498 PASSED" while real
    # assertions had failed.
    #
    # Two calling conventions are accepted (preserve backward compatibility
    # for skills_comprehensive_challenge.sh + cli_agent_integration_challenge.sh
    # which pass numeric "0"/"1" instead of the "PASSED"/"FAILED" strings):
    #   PASSED, "0"  → success (return 0)
    #   anything else → failure (return 1)
    case "$status" in
        PASSED|0) return 0 ;;
        *) return 1 ;;
    esac
}

# Cleanup on exit
cleanup() {
    stop_helixagent
}

trap cleanup EXIT

# Export all functions
export -f init_challenge log_info log_success log_warning log_error
export -f load_env detect_container_runtime check_binary
export -f get_helixagent_binary get_verifier_binary
export -f start_helixagent stop_helixagent
export -f start_infrastructure stop_infrastructure
export -f api_request record_assertion record_metric finalize_challenge
