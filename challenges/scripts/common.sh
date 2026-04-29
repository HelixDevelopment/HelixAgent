#!/bin/bash
# HelixAgent Challenges - Common Functions
# Source this file in challenge runner scripts

# Colors
export RED='\033[0;31m'
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export BLUE='\033[0;34m'
export NC='\033[0m'

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $(date +%H:%M:%S) $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $(date +%H:%M:%S) $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $(date +%H:%M:%S) $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) $1"; }

# JSON logging
log_json() {
    local event=$1
    local data=$2
    local timestamp=$(date -Iseconds)
    echo "{\"event\":\"$event\",\"timestamp\":\"$timestamp\",$data}" >> "$LOG_FILE"
}

# Get script directory
get_script_dir() {
    cd "$(dirname "${BASH_SOURCE[1]}")" && pwd
}

# Get challenges root directory
get_challenges_dir() {
    local script_dir=$(get_script_dir)
    # Navigate up from challenge_runners/*/run.sh or scripts/*.sh
    echo "$(cd "$script_dir/../.." 2>/dev/null || cd "$script_dir/.." && pwd)"
}

# Setup challenge environment
setup_challenge() {
    local challenge_name=$1
    export CHALLENGE_NAME="$challenge_name"
    export CHALLENGES_DIR=$(get_challenges_dir)
    export PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

    # Parse arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            --results-dir=*)
                export RESULTS_DIR="${1#*=}"
                ;;
            --verbose)
                export VERBOSE=true
                ;;
        esac
        shift
    done

    # Load environment from project root first (primary location for API keys).
    # Temporarily disable `set -u` so .env entries like FOO=$BAR (where BAR is
    # undefined) don't abort the challenge under `set -euo pipefail`. Auto-export
    # (`set -a`) is preserved so any vars assigned in .env propagate to subprocs.
    if [ -f "$PROJECT_ROOT/.env" ]; then
        set -a
        set +u
        source "$PROJECT_ROOT/.env"
        set -u
        set +a
    fi

    # Then load challenges-specific .env (can override or add settings)
    if [ -f "$CHALLENGES_DIR/.env" ]; then
        set -a
        set +u
        source "$CHALLENGES_DIR/.env"
        set -u
        set +a
    fi

    # Setup logging
    export LOG_FILE="${RESULTS_DIR:-/tmp}/logs/challenge.log"
    mkdir -p "$(dirname "$LOG_FILE")"

    log_info "Challenge setup complete: $CHALLENGE_NAME"
    log_json "setup_complete" "\"challenge\":\"$CHALLENGE_NAME\""
}

# Finalize challenge
finalize_challenge() {
    local exit_code=${1:-0}
    log_json "challenge_finalized" "\"exit_code\":$exit_code"
    log_info "Challenge finalized with exit code: $exit_code"
    return $exit_code
}

# Redact API key for safe logging
redact_api_key() {
    local key=$1
    if [ ${#key} -le 8 ]; then
        echo "*****"
    else
        echo "${key:0:4}$(printf '*%.0s' $(seq 1 $((${#key}-4))))"
    fi
}

# Check if provider is configured
is_provider_configured() {
    local provider=$1
    local var_name="${provider^^}_API_KEY"
    [ -n "${!var_name}" ]
}

# Get latest results directory for a challenge
get_latest_results() {
    local challenge=$1
    local results_base="$CHALLENGES_DIR/results/$challenge"
    find "$results_base" -maxdepth 4 -type d -name "[0-9]*_[0-9]*" 2>/dev/null | sort -r | head -1
}

# Wait for file with timeout
wait_for_file() {
    local file=$1
    local timeout=${2:-60}
    local elapsed=0

    while [ ! -f "$file" ] && [ $elapsed -lt $timeout ]; do
        sleep 1
        elapsed=$((elapsed + 1))
    done

    [ -f "$file" ]
}

# Safe JSON write (atomic)
write_json_file() {
    local file=$1
    local content=$2
    local tmp_file="${file}.tmp"

    echo "$content" > "$tmp_file"
    mv "$tmp_file" "$file"
}

# Extract value from JSON (simple)
json_get() {
    local json=$1
    local key=$2
    echo "$json" | grep -o "\"$key\":[^,}]*" | cut -d: -f2- | tr -d '"' | tr -d ' '
}

# Challenge test framework functions
print_header() {
    local name=$1
    echo "=================================================="
    echo "$name"
    echo "=================================================="
    echo ""
}

test_start() {
    local name=$1
    echo -n "Testing: $name ... "
}

test_pass() {
    echo -e "${GREEN}✓ PASS${NC}"
}

test_fail() {
    local message=$1
    echo -e "${RED}✗ FAIL${NC}: $message"
}

test_warn() {
    local message=$1
    echo -e "${YELLOW}⚠ WARNING${NC}: $message"
}

print_summary() {
    local name=$1
    local passed=$2
    local failed=$3
    echo ""
    echo "=================================================="
    echo "Summary: $name"
    echo "Passed: $passed"
    echo "Failed: $failed"
    if [ "$failed" -eq 0 ]; then
        echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
    else
        echo -e "${RED}✗ SOME TESTS FAILED${NC}"
    fi
    echo "=================================================="
}
