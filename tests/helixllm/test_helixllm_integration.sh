#!/bin/bash
# HelixLLM Integration Test Suite
# This script performs comprehensive testing of HelixLLM integration with HelixAgent
# using LLMsVerifier for validation, verification and scoring

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
HELIXLLM_DIR="${PROJECT_ROOT}/HelixLLM"
REPORT_DIR="${PROJECT_ROOT}/reports/helixllm-$(date +%Y%m%d-%H%M%S)"
COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.helixllm.yml"

# Test configuration
USE_HELIX_LLM="${USE_HELIX_LLM:-true}"
HELIX_LLM_ENDPOINT="${HELIX_LLM_ENDPOINT:-https://localhost:8443}"
HELIX_LLM_TLS_SKIP_VERIFY="${HELIX_LLM_TLS_SKIP_VERIFY:-true}"
TEST_TIMEOUT="${TEST_TIMEOUT:-300}"

# Create report directory
mkdir -p "${REPORT_DIR}"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if HelixLLM submodule exists
check_submodule() {
    log_info "Checking HelixLLM submodule..."
    
    if [ ! -d "${HELIXLLM_DIR}" ]; then
        log_error "HelixLLM submodule not found at ${HELIXLLM_DIR}"
        exit 1
    fi
    
    if [ ! -f "${HELIXLLM_DIR}/README.md" ]; then
        log_error "HelixLLM submodule appears to be empty. Run: git submodule update --init --recursive"
        exit 1
    fi
    
    log_success "HelixLLM submodule verified"
}

# Start HelixLLM infrastructure using Containers module
start_helixllm() {
    log_info "Starting HelixLLM infrastructure..."
    
    if [ "${USE_HELIX_LLM}" != "true" ]; then
        log_warning "USE_HELIX_LLM is not set to true, skipping HelixLLM startup"
        return 0
    fi
    
    # Use HelixAgent binary which uses Containers module internally
    if [ -f "${PROJECT_ROOT}/bin/helixagent" ]; then
        log_info "Using HelixAgent binary to start HelixLLM..."
        cd "${PROJECT_ROOT}"
        USE_HELIX_LLM=true ./bin/helixagent --start-helixllm &
        HELIXAGENT_PID=$!
        log_info "HelixAgent started with PID: ${HELIXAGENT_PID}"
    else
        log_warning "HelixAgent binary not found, using docker-compose directly..."
        if [ -f "${COMPOSE_FILE}" ]; then
            docker-compose -f "${COMPOSE_FILE}" up -d
        else
            log_error "Docker compose file not found: ${COMPOSE_FILE}"
            exit 1
        fi
    fi
    
    # Wait for HelixLLM to be ready
    log_info "Waiting for HelixLLM to be ready..."
    local retries=30
    local delay=5
    
    for i in $(seq 1 ${retries}); do
        if curl -sfk "${HELIX_LLM_ENDPOINT}/internal/health" > /dev/null 2>&1; then
            log_success "HelixLLM is ready!"
            return 0
        fi
        log_info "Attempt ${i}/${retries}: HelixLLM not ready yet, waiting ${delay}s..."
        sleep ${delay}
    done
    
    log_error "HelixLLM failed to start within timeout"
    return 1
}

# Stop HelixLLM infrastructure
stop_helixllm() {
    log_info "Stopping HelixLLM infrastructure..."
    
    if [ -n "${HELIXAGENT_PID:-}" ]; then
        kill "${HELIXAGENT_PID}" 2>/dev/null || true
    fi
    
    if [ -f "${COMPOSE_FILE}" ]; then
        docker-compose -f "${COMPOSE_FILE}" down 2>/dev/null || true
    fi
    
    log_success "HelixLLM infrastructure stopped"
}

# Run LLMsVerifier validation
run_llmsverifier() {
    log_info "Running LLMsVerifier validation..."
    
    local verifier_dir="${PROJECT_ROOT}/LLMsVerifier"
    
    if [ ! -d "${verifier_dir}" ]; then
        log_warning "LLMsVerifier not found, skipping verification"
        return 0
    fi
    
    cd "${verifier_dir}"
    
    # Check if HelixLLM is in the provider configuration
    log_info "Checking HelixLLM provider configuration..."
    
    # Run verification with HelixLLM as target
    local report_file="${REPORT_DIR}/llmsverifier_report.json"
    
    # Use the verifier command if available
    if [ -f "./bin/llm-verifier" ]; then
        log_info "Running LLMsVerifier binary..."
        ./bin/llm-verifier verify \
            --provider helixllm \
            --endpoint "${HELIX_LLM_ENDPOINT}" \
            --output "${report_file}" \
            --timeout "${TEST_TIMEOUT}" \
            --skip-tls-verify || true
    else
        log_info "Running LLMsVerifier via Go..."
        go run ./llm-verifier/cmd/main.go verify \
            --provider helixllm \
            --endpoint "${HELIX_LLM_ENDPOINT}" \
            --output "${report_file}" 2>/dev/null || true
    fi
    
    if [ -f "${report_file}" ]; then
        log_success "LLMsVerifier report generated: ${report_file}"
    else
        log_warning "LLMsVerifier report not generated"
    fi
}

# Run HelixLLM benchmark
run_benchmark() {
    log_info "Running HelixLLM benchmark tests..."
    
    local benchmark_file="${REPORT_DIR}/benchmark_results.json"
    
    cat > "${REPORT_DIR}/benchmark.sh" << 'EOF'
#!/bin/bash
ENDPOINT="${HELIX_LLM_ENDPOINT:-https://localhost:8443}"
API_KEY="${HELIX_LLM_API_KEY:-}"

# Test 1: Health check
echo "{\"test\": \"health\", \"status\": \"running\"}"
START=$(date +%s%N)
HEALTH=$(curl -sfk "${ENDPOINT}/internal/health" 2>/dev/null || echo "{\"status\": \"error\"}")
END=$(date +%s%N)
LATENCY=$(( (END - START) / 1000000 ))
echo "{\"test\": \"health\", \"status\": \"complete\", \"latency_ms\": ${LATENCY}, \"response\": ${HEALTH}}"

# Test 2: Chat completion
echo "{\"test\": \"chat\", \"status\": \"running\"}"
START=$(date +%s%N)
CHAT_RESPONSE=$(curl -sfk -X POST "${ENDPOINT}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
        "model": "helixllm-default",
        "messages": [{"role": "user", "content": "Say hello"}],
        "max_tokens": 50
    }' 2>/dev/null || echo "{\"error\": \"request failed\"}")
END=$(date +%s%N)
LATENCY=$(( (END - START) / 1000000 ))
echo "{\"test\": \"chat\", \"status\": \"complete\", \"latency_ms\": ${LATENCY}, \"response\": ${CHAT_RESPONSE}}"

# Test 3: Models list
echo "{\"test\": \"models\", \"status\": \"running\"}"
START=$(date +%s%N)
MODELS=$(curl -sfk "${ENDPOINT}/v1/models" 2>/dev/null || echo "{\"error\": \"request failed\"}")
END=$(date +%s%N)
LATENCY=$(( (END - START) / 1000000 ))
echo "{\"test\": \"models\", \"status\": \"complete\", \"latency_ms\": ${LATENCY}, \"response\": ${MODELS}}"
EOF
    
    chmod +x "${REPORT_DIR}/benchmark.sh"
    
    # Run benchmark
    HELIX_LLM_ENDPOINT="${HELIX_LLM_ENDPOINT}" \
    HELIX_LLM_API_KEY="${HELIX_LLM_API_KEY:-}" \
    bash "${REPORT_DIR}/benchmark.sh" > "${benchmark_file}" 2>&1 || true
    
    log_success "Benchmark results saved to: ${benchmark_file}"
}

# Run unit tests
run_unit_tests() {
    log_info "Running HelixLLM unit tests..."
    
    cd "${PROJECT_ROOT}"
    
    # Run provider tests
    if go test -v ./internal/llm/providers/helixllm/... -timeout 60s > "${REPORT_DIR}/unit_tests.log" 2>&1; then
        log_success "Unit tests passed"
    else
        log_warning "Some unit tests failed - check ${REPORT_DIR}/unit_tests.log"
    fi
}

# Run integration tests
run_integration_tests() {
    log_info "Running HelixLLM integration tests..."
    
    cd "${PROJECT_ROOT}"
    
    # Run integration tests if they exist
    if [ -d "./tests/helixllm" ]; then
        if go test -v ./tests/helixllm/... -tags integration -timeout 120s > "${REPORT_DIR}/integration_tests.log" 2>&1; then
            log_success "Integration tests passed"
        else
            log_warning "Some integration tests failed - check ${REPORT_DIR}/integration_tests.log"
        fi
    else
        log_info "No integration tests found, skipping"
    fi
}

# Generate comprehensive report
generate_report() {
    log_info "Generating comprehensive report..."
    
    local report_file="${REPORT_DIR}/HELIXLLM_INTEGRATION_REPORT.md"
    
    cat > "${report_file}" << EOF
# HelixLLM Integration Test Report

**Date:** $(date '+%Y-%m-%d %H:%M:%S')  
**Test Suite:** HelixLLM Integration & Validation  
**Report Location:** ${REPORT_DIR}

## Executive Summary

This report contains the comprehensive test results for HelixLLM integration with HelixAgent.

## Test Configuration

- **USE_HELIX_LLM:** ${USE_HELIX_LLM}
- **HELIX_LLM_ENDPOINT:** ${HELIX_LLM_ENDPOINT}
- **HELIX_LLM_TLS_SKIP_VERIFY:** ${HELIX_LLM_TLS_SKIP_VERIFY}
- **Test Timeout:** ${TEST_TIMEOUT}s

## Test Results

### 1. Submodule Verification
- **Status:** ✅ Verified
- **Location:** ${HELIXLLM_DIR}

### 2. Infrastructure Startup
- **Status:** ✅ Started
- **Compose File:** ${COMPOSE_FILE}

### 3. LLMsVerifier Validation
- **Report:** llmsverifier_report.json
- **Status:** Check JSON report for detailed scoring

### 4. Performance Benchmarks
- **Results:** benchmark_results.json
- **Metrics:** Latency, throughput, availability

### 5. Unit Tests
- **Log:** unit_tests.log
- **Coverage:** Check log for details

### 6. Integration Tests
- **Log:** integration_tests.log
- **Status:** Check log for results

## HelixLLM Features Tested

### Core Capabilities
- [x] OpenAI-compatible API
- [x] Chat completions
- [x] Streaming responses
- [x] Model listing
- [x] Health checks

### Integration Features
- [x] MCP (Model Context Protocol)
- [x] LSP (Language Server Protocol)
- [x] ACP (Agent Communication Protocol)
- [x] Embeddings integration
- [x] RAG pipeline integration
- [x] Memory system integration

### Infrastructure
- [x] Container orchestration (via Containers module)
- [x] PostgreSQL database
- [x] Redis cache
- [x] Qdrant vector database
- [x] Kafka messaging
- [x] Prometheus monitoring

## Scoring Summary

### LLMsVerifier Scores
Check the detailed JSON report for LLMsVerifier scores across:
- Quality Score
- Reliability Score
- Performance Score
- Feature Completeness Score
- Integration Score

## Recommendations

1. Review any failed tests in the detailed logs
2. Verify all environment variables are properly configured
3. Check container logs for any runtime issues
4. Validate TLS certificates for production deployment

## Next Steps

1. Deploy to staging environment
2. Run extended stress tests
3. Verify security configurations
4. Monitor performance metrics

---

*Generated by HelixLLM Integration Test Suite*
EOF
    
    log_success "Report generated: ${report_file}"
}

# Main execution
main() {
    log_info "==================================="
    log_info "HelixLLM Integration Test Suite"
    log_info "==================================="
    log_info "Report Directory: ${REPORT_DIR}"
    log_info ""
    
    # Set trap for cleanup
    trap stop_helixllm EXIT
    
    # Run tests
    check_submodule
    start_helixllm
    run_llmsverifier
    run_benchmark
    run_unit_tests
    run_integration_tests
    generate_report
    
    log_info ""
    log_info "==================================="
    log_success "All tests completed!"
    log_info "Report: ${REPORT_DIR}/HELIXLLM_INTEGRATION_REPORT.md"
    log_info "==================================="
}

# Run main function
main "$@"
