#!/bin/bash
# HelixAgent Challenges - Challenge Runner Script
# Usage: ./scripts/run_challenges.sh [challenge_name] [options]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
VERBOSE=false
TIMEOUT=600
DRY_RUN=false

# Print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Show usage
usage() {
    cat << EOF
HelixAgent Challenges Runner

Usage: $0 <challenge_name> [options]

Available Challenges (41 total):
  Core: provider_verification, ai_debate_formation, api_quality_test,
        ensemble_voting, ai_debate_workflow, embeddings_service,
        streaming_responses, model_metadata, oauth_credentials

  Providers: provider_claude, provider_deepseek, provider_gemini,
             provider_ollama, provider_openrouter, provider_qwen, provider_zai

  Protocols: mcp_protocol, lsp_protocol, acp_protocol

  Cloud: cloud_aws_bedrock, cloud_gcp_vertex, cloud_azure_openai

  Security: authentication, rate_limiting, input_validation

  Resilience: circuit_breaker, error_handling, concurrent_access, graceful_shutdown

  Infrastructure: health_monitoring, caching_layer, database_operations,
                  plugin_system, session_management, configuration_loading

  Optimization: optimization_semantic_cache, optimization_structured_output

  Integration: cognee_integration

  API: openai_compatibility, grpc_api

  Validation: opencode

  Master: main

Options:
  -v, --verbose            Enable verbose output
  -t, --timeout SECONDS    Set timeout (default: 600)
  -d, --dry-run            Show what would be done without executing
  -h, --help               Show this help message

Examples:
  $0 provider_verification
  $0 ai_debate_formation --verbose
  $0 api_quality_test --timeout=900

EOF
    exit 0
}

# Parse arguments
parse_args() {
    if [ $# -eq 0 ]; then
        usage
    fi

    CHALLENGE_NAME="$1"
    shift

    while [ $# -gt 0 ]; do
        case "$1" in
            -v|--verbose)
                VERBOSE=true
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift
                ;;
            --timeout=*)
                TIMEOUT="${1#*=}"
                ;;
            -d|--dry-run)
                DRY_RUN=true
                ;;
            -h|--help)
                usage
                ;;
            *)
                print_error "Unknown option: $1"
                usage
                ;;
        esac
        shift
    done
}

# Load environment variables
load_env() {
    local env_loaded=false

    # First try project root .env (primary location for API keys)
    if [ -f "$PROJECT_ROOT/.env" ]; then
        print_info "Loading environment from $PROJECT_ROOT/.env"
        set -a
        source "$PROJECT_ROOT/.env"
        set +a
        env_loaded=true
    fi

    # Then load challenges-specific .env (can override or add settings)
    if [ -f "$CHALLENGES_DIR/.env" ]; then
        print_info "Loading environment from $CHALLENGES_DIR/.env"
        set -a
        source "$CHALLENGES_DIR/.env"
        set +a
        env_loaded=true
    fi

    if [ "$env_loaded" = false ]; then
        print_warning ".env file not found. Using system environment variables."
    fi
}

# Create results directory
create_results_dir() {
    local challenge=$1
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local year=$(date +%Y)
    local month=$(date +%m)
    local day=$(date +%d)

    RESULTS_DIR="$CHALLENGES_DIR/results/$challenge/$year/$month/$day/$timestamp"

    mkdir -p "$RESULTS_DIR/logs"
    mkdir -p "$RESULTS_DIR/results"
    mkdir -p "$RESULTS_DIR/config"

    print_info "Results directory: $RESULTS_DIR"
}

# Find latest results directory for a challenge
find_latest_results() {
    local challenge=$1
    local base_path="$CHALLENGES_DIR/results/$challenge"

    if [ ! -d "$base_path" ]; then
        return 1
    fi

    # Find the most recent timestamp directory that contains a results subdirectory
    local latest=$(find "$base_path" -type d -name "results" 2>/dev/null | \
                   head -n 1 | \
                   xargs -I {} dirname {})

    if [ -n "$latest" ] && [ -d "$latest/results" ]; then
        echo "$latest"
        return 0
    fi

    return 1
}

# Check dependencies from challenges_bank.json
check_dependencies() {
    local challenge=$1
    local bank_file="$CHALLENGES_DIR/data/challenges_bank.json"

    if [ -f "$bank_file" ]; then
        # Get dependencies from bank
        local deps=$(jq -r ".challenges[] | select(.id == \"$challenge\") | .dependencies[]?" "$bank_file" 2>/dev/null)

        if [ -n "$deps" ]; then
            for dep in $deps; do
                local dep_results=$(find_latest_results "$dep")
                if [ -z "$dep_results" ]; then
                    print_warning "Dependency $dep not found - challenge may fail"
                else
                    print_info "Found $dep results at: $dep_results"
                    export DEPENDENCY_DIR="$dep_results"
                fi
            done
        fi
    else
        # Fallback to static checks
        case "$challenge" in
            ai_debate_formation|ensemble_voting|embeddings_service|streaming_responses|model_metadata|circuit_breaker)
                PROVIDER_VERIFICATION_RESULTS=$(find_latest_results "provider_verification")
                if [ -n "$PROVIDER_VERIFICATION_RESULTS" ]; then
                    print_info "Found provider_verification results at: $PROVIDER_VERIFICATION_RESULTS"
                    export DEPENDENCY_DIR="$PROVIDER_VERIFICATION_RESULTS"
                fi
                ;;
            api_quality_test|ai_debate_workflow)
                AI_DEBATE_RESULTS=$(find_latest_results "ai_debate_formation")
                if [ -n "$AI_DEBATE_RESULTS" ]; then
                    print_info "Found ai_debate_formation results at: $AI_DEBATE_RESULTS"
                    export DEPENDENCY_DIR="$AI_DEBATE_RESULTS"
                fi
                ;;
            rate_limiting|session_management)
                CACHE_RESULTS=$(find_latest_results "caching_layer")
                if [ -n "$CACHE_RESULTS" ]; then
                    print_info "Found caching_layer results at: $CACHE_RESULTS"
                    export DEPENDENCY_DIR="$CACHE_RESULTS"
                fi
                ;;
            optimization_semantic_cache)
                EMBED_RESULTS=$(find_latest_results "embeddings_service")
                if [ -n "$EMBED_RESULTS" ]; then
                    print_info "Found embeddings_service results at: $EMBED_RESULTS"
                    export DEPENDENCY_DIR="$EMBED_RESULTS"
                fi
                ;;
        esac
    fi
}

# Run the challenge
run_challenge() {
    local challenge=$1

    print_info "Running challenge: $challenge"
    print_info "Timeout: ${TIMEOUT}s"

    if [ "$DRY_RUN" = true ]; then
        print_warning "DRY RUN - Not executing"
        return 0
    fi

    # Create results directory
    create_results_dir "$challenge"

    # Log start
    local start_time=$(date +%s)
    echo "{\"event\":\"challenge_started\",\"challenge\":\"$challenge\",\"timestamp\":\"$(date -Iseconds)\"}" >> "$RESULTS_DIR/logs/challenge.log"

    # Use generic runner for all challenges (more robust, doesn't require HelixAgent running)
    print_info "Using generic challenge runner..."

    local generic_runner="$SCRIPT_DIR/generic_challenge.sh"

    if [ -f "$generic_runner" ]; then
        print_info "Using generic runner for: $challenge"
        bash "$generic_runner" "$challenge" --results-dir="$RESULTS_DIR" ${VERBOSE:+--verbose}
        local exit_code=$?
    else
        print_error "Generic runner not found: $generic_runner"
        exit 1
    fi

    # Log completion
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo "{\"event\":\"challenge_completed\",\"challenge\":\"$challenge\",\"duration_seconds\":$duration,\"exit_code\":$exit_code,\"timestamp\":\"$(date -Iseconds)\"}" >> "$RESULTS_DIR/logs/challenge.log"

    if [ $exit_code -eq 0 ]; then
        print_success "Challenge completed successfully in ${duration}s"
        print_info "Results: $RESULTS_DIR"
    else
        print_error "Challenge failed with exit code $exit_code"
        exit $exit_code
    fi
}

# Main
main() {
    parse_args "$@"

    print_info "HelixAgent Challenges Runner"
    print_info "Challenge: $CHALLENGE_NAME"

    # Validate challenge name using static list (no jq dependency)
    case "$CHALLENGE_NAME" in
        main|provider_verification|ai_debate_formation|api_quality_test|\
        ensemble_voting|ai_debate_workflow|embeddings_service|streaming_responses|\
        model_metadata|provider_claude|provider_deepseek|provider_gemini|\
        provider_ollama|provider_openrouter|provider_qwen|provider_zai|\
        mcp_protocol|lsp_protocol|acp_protocol|cloud_aws_bedrock|cloud_gcp_vertex|\
        cloud_azure_openai|authentication|rate_limiting|input_validation|\
        circuit_breaker|error_handling|concurrent_access|graceful_shutdown|\
        health_monitoring|caching_layer|database_operations|plugin_system|\
        session_management|configuration_loading|optimization_semantic_cache|\
        optimization_structured_output|cognee_integration|openai_compatibility|grpc_api|\
        opencode|oauth_credentials)
            ;;
        *)
            print_error "Unknown challenge: $CHALLENGE_NAME"
            usage
            ;;
    esac

    load_env
    check_dependencies "$CHALLENGE_NAME"
    run_challenge "$CHALLENGE_NAME"
}

main "$@"
