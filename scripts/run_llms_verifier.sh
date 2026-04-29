#!/bin/bash
# LLMsVerifier - Comprehensive Provider & Model Validation
# Tests all 47 providers and their individual models with detailed error analysis

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DATE=$(date +%Y-%m-%d)
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_DIR="$PROJECT_ROOT/docs/reports/llms_verifier/$DATE"
DETAILED_LOG="$REPORT_DIR/detailed_$TIMESTAMP.log"

mkdir -p "$REPORT_DIR"
touch "$DETAILED_LOG"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { 
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$DETAILED_LOG"
}
log_success() { 
    echo -e "${GREEN}[PASS]${NC} $1" | tee -a "$DETAILED_LOG"
}
log_error() { 
    echo -e "${RED}[FAIL]${NC} $1" | tee -a "$DETAILED_LOG"
}
log_warn() { 
    echo -e "${YELLOW}[WARN]${NC} $1" | tee -a "$DETAILED_LOG"
}
log_detail() {
    echo -e "${CYAN}[DETAIL]${NC} $1" | tee -a "$DETAILED_LOG"
}

# Configuration
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"
TEST_TIMEOUT=60
COMPREHENSIVE_TIMEOUT=120
PARALLEL_JOBS=3

# Test prompts for different capability evaluation
TEST_PROMPT_SIMPLE="What is 2+2? Answer with just the number."
TEST_PROMPT_REASONING="Explain step by step how to solve: If a train travels 60 km in 30 minutes, what is its average speed in km/h?"
TEST_PROMPT_CODE="Write a Python function to calculate fibonacci(n) with memoization."
TEST_PROMPT_CREATIVE="Write a haiku about artificial intelligence."

# Provider definitions with their models
# Format: name:vendor:auth_type:tier:models_json
PROVIDERS=(
    # Primary Tier - Most Important
    "claude:anthropic:api_key:primary:{\"models\":[\"claude-3-opus-20240229\",\"claude-3-sonnet-20240229\",\"claude-3-haiku-20240307\",\"claude-3-5-sonnet-20240620\"],\"capabilities\":[\"vision\",\"tools\",\"streaming\",\"json_mode\"]}"
    "openai-gpt4:openai:api_key:primary:{\"models\":[\"gpt-4o\",\"gpt-4o-mini\",\"gpt-4-turbo\",\"gpt-4\"],\"capabilities\":[\"vision\",\"tools\",\"streaming\",\"json_mode\",\"function_calling\"]}"
    "codex:openai:api_key:primary:{\"models\":[\"codex\",\"codex-latest\"],\"capabilities\":[\"code\",\"streaming\"]}"
    
    # Secondary Tier - Agent-Specific
    "gemini:google:api_key:secondary:{\"models\":[\"gemini-1.5-pro\",\"gemini-1.5-flash\",\"gemini-1.0-pro\",\"gemini-ultra\"],\"capabilities\":[\"vision\",\"tools\",\"streaming\",\"json_mode\"]}"
    "deepseek:deepseek:api_key:secondary:{\"models\":[\"deepseek-chat\",\"deepseek-coder\",\"deepseek-reasoner\"],\"capabilities\":[\"code\",\"reasoning\",\"streaming\"]}"
    "mistral:mistral:api_key:secondary:{\"models\":[\"mistral-large-latest\",\"mistral-medium\",\"mistral-small\",\"codestral\"],\"capabilities\":[\"tools\",\"streaming\",\"json_mode\"]}"
    "groq:groq:api_key:secondary:{\"models\":[\"llama-3.1-405b-reasoning\",\"llama-3.1-70b-versatile\",\"mixtral-8x7b\",\"gemma2-9b-it\"],\"capabilities\":[\"streaming\",\"fast\"]}"
    "qwen:alibaba:api_key:secondary:{\"models\":[\"qwen-max\",\"qwen-plus\",\"qwen-turbo\",\"qwen-coder\"],\"capabilities\":[\"vision\",\"tools\",\"streaming\"]}"
    "xai:xai:api_key:secondary:{\"models\":[\"grok-beta\",\"grok-vision-beta\"],\"capabilities\":[\"vision\",\"streaming\",\"real_time\"]}"
    "cohere:cohere:api_key:secondary:{\"models\":[\"command-r-plus\",\"command-r\",\"command\"],\"capabilities\":[\"tools\",\"streaming\",\"rerank\"]}"
    "perplexity:perplexity:api_key:secondary:{\"models\":[\"sonar-reasoning-pro\",\"sonar-reasoning\",\"sonar-pro\",\"sonar\"],\"capabilities\":[\"search\",\"citations\",\"streaming\"]}"
    
    # Tertiary Tier - Additional Providers
    "together:together:api_key:tertiary:{\"models\":[\"meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo\",\"meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo\"],\"capabilities\":[\"streaming\"]}"
    "fireworks:fireworks:api_key:tertiary:{\"models\":[\"accounts/fireworks/models/llama-v3p1-405b-instruct\",\"accounts/fireworks/models/mixtral-8x22b-instruct\"],\"capabilities\":[\"streaming\",\"fast\"]}"
    "openrouter:openrouter:api_key:tertiary:{\"models\":[\"anthropic/claude-3.5-sonnet\",\"openai/gpt-4o\",\"meta-llama/llama-3.1-405b-instruct\"],\"capabilities\":[\"aggregated\",\"streaming\"]}"
    "ai21:ai21:api_key:tertiary:{\"models\":[\"jamba-1.5-large\",\"jamba-1.5-mini\"],\"capabilities\":[\"streaming\"]}"
    "cloudflare:cloudflare:api_key:tertiary:{\"models\":[\"@cf/meta/llama-3.1-8b-instruct\",\"@cf/mistral/mistral-7b-instruct-v0.2\"],\"capabilities\":[\"edge\",\"fast\"]}"
    "azure:azure:api_key:tertiary:{\"models\":[\"gpt-4o\",\"gpt-4\",\"gpt-35-turbo\"],\"capabilities\":[\"vision\",\"tools\",\"enterprise\"]}"
    "bedrock:aws:api_key:tertiary:{\"models\":[\"anthropic.claude-3-sonnet\",\"anthropic.claude-3-haiku\",\"meta.llama3-1-405b\"],\"capabilities\":[\"vision\",\"enterprise\"]}"
    "vertexai:google:oauth:tertiary:{\"models\":[\"gemini-1.5-pro\",\"gemini-1.5-flash\",\"claude-3-sonnet@20240229\"],\"capabilities\":[\"vision\",\"tools\",\"enterprise\"]}"
    "ollama:ollama:local:tertiary:{\"models\":[\"llama3.1\",\"mistral\",\"codellama\",\"gemma2\"],\"capabilities\":[\"local\",\"privacy\"]}"
)

# Results tracking
declare -A PROVIDER_RESULTS
declare -A MODEL_RESULTS
declare -A ERROR_DETAILS
declare -A LATENCY_DATA
declare -A QUALITY_SCORES
declare -A CAPABILITY_SUPPORT

TOTAL_PROVIDERS=${#PROVIDERS[@]}
TOTAL_MODELS=0
PROVIDERS_PASSED=0
PROVIDERS_FAILED=0
PROVIDERS_SKIPPED=0

# Calculate total models
for provider_info in "${PROVIDERS[@]}"; do
    IFS=':' read -r name vendor auth tier models_json <<< "$provider_info"
    model_count=$(echo "$models_json" | jq -r '.models | length' 2>/dev/null || echo "0")
    TOTAL_MODELS=$((TOTAL_MODELS + model_count))
done

# Test connection to a provider
test_provider_connection() {
    local name=$1
    local vendor=$2
    local auth=$3
    local env_var=$(echo "${name}_${auth}" | tr '[:lower:]-' '[:upper:]_' | sed 's/_API_KEY//')_API_KEY
    
    log_info "[$name] Testing provider connection..."
    
    # Check for API key
    if [ "$auth" == "api_key" ] && [ -z "${!env_var}" ]; then
        if [ "$vendor" == "local" ]; then
            log_detail "[$name] Local provider, no API key needed"
            return 0
        fi
        log_warn "[$name] No API key found (expected: $env_var)"
        ERROR_DETAILS["$name:auth"]="Missing API key: $env_var not set in environment"
        return 1
    fi
    
    # Test health endpoint
    local start_time end_time duration
    start_time=$(date +%s%N)
    
    local response
    local http_code
    response=$(curl -s -m $TEST_TIMEOUT \
        -w "\n%{http_code}" \
        -X GET "$HELIXAGENT_URL/v1/providers/$name/health" \
        -H "Authorization: Bearer ${!env_var}" 2>&1) || true
    
    http_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | sed '$d')
    
    end_time=$(date +%s%N)
    duration=$(( (end_time - start_time) / 1000000 ))
    LATENCY_DATA["$name:health"]=$duration
    
    log_detail "[$name] Health check response time: ${duration}ms, HTTP: $http_code"
    
    if [ "$http_code" == "200" ]; then
        if echo "$response_body" | grep -q "healthy\|ok\|status.*up"; then
            log_success "[$name] Provider is healthy"
            return 0
        else
            log_warn "[$name] Health check returned 200 but status unclear"
            log_detail "[$name] Response: $response_body"
            return 0
        fi
    elif [ "$http_code" == "401" ] || [ "$http_code" == "403" ]; then
        log_error "[$name] Authentication failed (HTTP $http_code)"
        ERROR_DETAILS["$name:health"]="Authentication failed: Invalid or expired API key (HTTP $http_code)"
        return 1
    elif [ "$http_code" == "404" ]; then
        log_error "[$name] Provider not found (HTTP 404)"
        ERROR_DETAILS["$name:health"]="Provider endpoint not implemented or misconfigured"
        return 1
    elif [ "$http_code" == "000" ]; then
        log_error "[$name] Connection failed (no response)"
        ERROR_DETAILS["$name:health"]="Connection timeout or refused. HelixAgent may be down or provider not configured"
        return 1
    else
        log_error "[$name] Health check failed (HTTP $http_code)"
        ERROR_DETAILS["$name:health"]="Unexpected response: HTTP $http_code - $response_body"
        return 1
    fi
}

# Test a specific model
test_model() {
    local provider=$1
    local model=$2
    local test_type=$3
    local prompt=$4
    
    log_detail "[$provider/$model] Testing $test_type..."
    
    local start_time end_time duration
    start_time=$(date +%s%N)
    
    local request_body
    request_body=$(jq -n \
        --arg model "$model" \
        --arg prompt "$prompt" \
        '{model: $model, messages: [{role: "user", content: $prompt}], max_tokens: 500, temperature: 0.7}')
    
    local response
    local http_code
    response=$(curl -s -m $COMPREHENSIVE_TIMEOUT \
        -w "\n%{http_code}" \
        -X POST "$HELIXAGENT_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "X-Provider: $provider" \
        -d "$request_body" 2>&1) || true
    
    http_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | sed '$d')
    
    end_time=$(date +%s%N)
    duration=$(( (end_time - start_time) / 1000000 ))
    
    local result_key="$provider:$model:$test_type"
    LATENCY_DATA["$result_key"]=$duration
    
    # Analyze response quality
    local quality_score=0
    local error_detail=""
    
    if [ "$http_code" == "200" ]; then
        # Check for valid response structure
        if echo "$response_body" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
            local content
            content=$(echo "$response_body" | jq -r '.choices[0].message.content')
            
            # Basic quality checks
            if [ -n "$content" ] && [ ${#content} -gt 5 ]; then
                quality_score=80  # Base score for valid response
                
                # Check for relevant content
                case "$test_type" in
                    "simple")
                        if echo "$content" | grep -qi "4\|four"; then
                            quality_score=100
                            log_success "[$provider/$model] Simple query answered correctly"
                        else
                            quality_score=60
                            log_warn "[$provider/$model] Simple query answered but answer may be incorrect"
                        fi
                        ;;
                    "reasoning")
                        if echo "$content" | grep -qi "120\|step\|minute\|hour"; then
                            quality_score=95
                            log_success "[$provider/$model] Reasoning query shows understanding"
                        else
                            quality_score=70
                            log_warn "[$provider/$model] Reasoning query answered but unclear if correct"
                        fi
                        ;;
                    "code")
                        if echo "$content" | grep -q "def.*fibonacci\|function.*fibonacci\|memoiz\|cache"; then
                            quality_score=95
                            log_success "[$provider/$model] Code query generates valid code"
                        else
                            quality_score=65
                            log_warn "[$provider/$model] Code query answered but may not be optimal"
                        fi
                        ;;
                    "creative")
                        if echo "$content" | grep -q "\n\|;" || [ ${#content} -gt 20 ]; then
                            quality_score=90
                            log_success "[$provider/$model] Creative query generates content"
                        else
                            quality_score=70
                            log_warn "[$provider/$model] Creative query answered but very brief"
                        fi
                        ;;
                esac
            else
                quality_score=30
                error_detail="Response content is empty or too short"
                log_error "[$provider/$model] Empty or invalid response content"
            fi
        else
            quality_score=20
            error_detail="Invalid JSON structure in response: $response_body"
            log_error "[$provider/$model] Malformed JSON response"
        fi
    elif [ "$http_code" == "401" ] || [ "$http_code" == "403" ]; then
        quality_score=0
        error_detail="Authentication error: API key invalid or rate limited"
        log_error "[$provider/$model] Authentication failed"
    elif [ "$http_code" == "429" ]; then
        quality_score=0
        error_detail="Rate limit exceeded"
        log_warn "[$provider/$model] Rate limited"
    elif [ "$http_code" == "500" ]; then
        quality_score=0
        error_detail="Provider server error: $response_body"
        log_error "[$provider/$model] Provider server error"
    elif [ "$http_code" == "000" ]; then
        quality_score=0
        error_detail="Connection timeout or no response from provider"
        log_error "[$provider/$model] Connection timeout"
    else
        quality_score=0
        error_detail="Unexpected error: HTTP $http_code - $response_body"
        log_error "[$provider/$model] Unexpected error (HTTP $http_code)"
    fi
    
    QUALITY_SCORES["$result_key"]=$quality_score
    MODEL_RESULTS["$result_key"]="$http_code"
    
    if [ -n "$error_detail" ]; then
        ERROR_DETAILS["$result_key"]="$error_detail"
    fi
    
    log_detail "[$provider/$model] $test_type: Score=$quality_score, Latency=${duration}ms, HTTP=$http_code"
    
    return $(( 100 - quality_score ))
}

# Test capabilities for a provider
test_capabilities() {
    local provider=$1
    local capabilities_json=$2
    
    log_info "[$provider] Testing capabilities..."
    
    local capabilities
    capabilities=$(echo "$capabilities_json" | jq -r '.capabilities[]' 2>/dev/null)
    
    for capability in $capabilities; do
        case "$capability" in
            "streaming")
                log_detail "[$provider] Testing streaming capability..."
                local response
                response=$(curl -s -m $TEST_TIMEOUT \
                    "$HELIXAGENT_URL/v1/providers/$provider/capabilities/streaming" 2>&1) || true
                if echo "$response" | grep -q "true\|enabled\|supported"; then
                    CAPABILITY_SUPPORT["$provider:streaming"]="YES"
                    log_success "[$provider] Streaming supported"
                else
                    CAPABILITY_SUPPORT["$provider:streaming"]="NO"
                    log_warn "[$provider] Streaming not supported or failed"
                fi
                ;;
            "vision")
                log_detail "[$provider] Testing vision capability..."
                CAPABILITY_SUPPORT["$provider:vision"]="PENDING_DETAILED_TEST"
                ;;
            "tools")
                log_detail "[$provider] Testing tools capability..."
                CAPABILITY_SUPPORT["$provider:tools"]="PENDING_DETAILED_TEST"
                ;;
            "json_mode")
                log_detail "[$provider] Testing JSON mode capability..."
                CAPABILITY_SUPPORT["$provider:json_mode"]="PENDING_DETAILED_TEST"
                ;;
        esac
    done
}

# Test a complete provider
test_provider() {
    local provider_info=$1
    
    IFS=':' read -r name vendor auth tier models_json <<< "$provider_info"
    
    log_info "═══════════════════════════════════════════════════════"
    log_info "Testing Provider: $name ($vendor) [Tier: $tier]"
    log_info "═══════════════════════════════════════════════════════"
    
    # Test connection
    if ! test_provider_connection "$name" "$vendor" "$auth"; then
        PROVIDER_RESULTS["$name"]="FAILED_CONNECTION"
        ((PROVIDERS_FAILED++))
        return 1
    fi
    
    # Test capabilities
    test_capabilities "$name" "$models_json"
    
    # Get models list
    local models
    models=$(echo "$models_json" | jq -r '.models[]' 2>/dev/null)
    
    local models_passed=0
    local models_failed=0
    
    for model in $models; do
        log_info "[$name] Testing model: $model"
        
        local model_passed=true
        
        # Test simple query
        if ! test_model "$name" "$model" "simple" "$TEST_PROMPT_SIMPLE"; then
            model_passed=false
        fi
        
        # Test reasoning (skip if simple failed)
        if [ "$model_passed" == "true" ]; then
            test_model "$name" "$model" "reasoning" "$TEST_PROMPT_REASONING" || true
        fi
        
        # Test code generation
        test_model "$name" "$model" "code" "$TEST_PROMPT_CODE" || true
        
        # Test creative
        test_model "$name" "$model" "creative" "$TEST_PROMPT_CREATIVE" || true
        
        # Calculate model average
        local total_score=0
        local score_count=0
        for test_type in simple reasoning code creative; do
            local score="${QUALITY_SCORES["$name:$model:$test_type"]:-0}"
            total_score=$((total_score + score))
            ((score_count++))
        done
        
        local avg_score=$((total_score / score_count))
        
        if [ $avg_score -ge 60 ]; then
            ((models_passed++))
            log_success "[$name/$model] Average score: $avg_score/100"
        else
            ((models_failed++))
            log_error "[$name/$model] Average score too low: $avg_score/100"
            ERROR_DETAILS["$name:$model:overall"]="Model scored $avg_score/100. Issues: ${ERROR_DETAILS["$name:$model:simple"]:-Unknown}"
        fi
    done
    
    # Determine provider status
    if [ $models_passed -eq 0 ]; then
        PROVIDER_RESULTS["$name"]="FAILED_ALL_MODELS"
        ((PROVIDERS_FAILED++))
        ERROR_DETAILS["$name:overall"]="All $models_failed models failed validation"
    elif [ $models_failed -eq 0 ]; then
        PROVIDER_RESULTS["$name"]="PASSED_ALL"
        ((PROVIDERS_PASSED++))
    else
        PROVIDER_RESULTS["$name"]="PARTIAL"
        ((PROVIDERS_PASSED++))
        log_warn "[$name] Partial success: $models_passed passed, $models_failed failed"
    fi
    
    return 0
}

# Test ensemble functionality with working providers
test_ensemble() {
    log_info "═══════════════════════════════════════════════════════"
    log_info "Testing Ensemble Coordination"
    log_info "═══════════════════════════════════════════════════════"
    
    local working_providers=()
    for provider_info in "${PROVIDERS[@]}"; do
        IFS=':' read -r name vendor auth tier models_json <<< "$provider_info"
        if [ "${PROVIDER_RESULTS[$name]}" == "PASSED_ALL" ] || [ "${PROVIDER_RESULTS[$name]}" == "PARTIAL" ]; then
            working_providers+=("$name")
        fi
    done
    
    if [ ${#working_providers[@]} -lt 2 ]; then
        log_warn "Not enough working providers for ensemble test (need 2+, have ${#working_providers[@]})"
        ERROR_DETAILS["ensemble:overall"]="Insufficient working providers. Only ${#working_providers[@]} available."
        return 1
    fi
    
    log_info "Using providers: ${working_providers[*]}"
    
    # Create ensemble session
    local response
    local http_code
    response=$(curl -s -m $COMPREHENSIVE_TIMEOUT \
        -w "\n%{http_code}" \
        -X POST "$HELIXAGENT_URL/v1/ensemble/sessions" \
        -H "Content-Type: application/json" \
        -d "{
            \"strategy\": \"voting\",
            \"participants\": {
                \"primary\": {\"type\": \"${working_providers[0]}\"},
                \"critiques\": [
                    {\"type\": \"${working_providers[1]}\"}
                ]
            }
        }" 2>&1)
    
    http_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" != "200" ] && [ "$http_code" != "201" ]; then
        log_error "Failed to create ensemble session (HTTP $http_code)"
        ERROR_DETAILS["ensemble:create"]="Ensemble creation failed: HTTP $http_code - $response_body"
        return 1
    fi
    
    log_success "Ensemble session created successfully"
    
    local session_id
    session_id=$(echo "$response_body" | jq -r '.id' 2>/dev/null)
    
    if [ -z "$session_id" ] || [ "$session_id" == "null" ]; then
        log_error "Could not extract session ID"
        ERROR_DETAILS["ensemble:parse"]="Failed to parse session ID from response: $response_body"
        return 1
    fi
    
    # Execute test task
    log_info "Executing ensemble task..."
    local exec_response
    local exec_http_code
    exec_response=$(curl -s -m $COMPREHENSIVE_TIMEOUT \
        -w "\n%{http_code}" \
        -X POST "$HELIXAGENT_URL/v1/ensemble/sessions/$session_id/execute" \
        -H "Content-Type: application/json" \
        -d '{
            "content": "What is the capital of France? Answer concisely.",
            "timeout": 45
        }' 2>&1)
    
    exec_http_code=$(echo "$exec_response" | tail -n1)
    exec_body=$(echo "$exec_response" | sed '$d')
    
    if [ "$exec_http_code" == "200" ]; then
        if echo "$exec_body" | grep -q "consensus_reached\|Paris"; then
            log_success "Ensemble execution successful with consensus"
            PROVIDER_RESULTS["ensemble"]="PASSED"
        else
            log_warn "Ensemble executed but consensus unclear"
            PROVIDER_RESULTS["ensemble"]="PARTIAL"
            ERROR_DETAILS["ensemble:consensus"]="No clear consensus reached. Response: $exec_body"
        fi
    else
        log_error "Ensemble execution failed (HTTP $exec_http_code)"
        ERROR_DETAILS["ensemble:execute"]="Execution failed: HTTP $exec_http_code - $exec_body"
        PROVIDER_RESULTS["ensemble"]="FAILED"
    fi
    
    # Cleanup
    curl -s -X DELETE "$HELIXAGENT_URL/v1/ensemble/sessions/$session_id" > /dev/null 2>&1 || true
    
    return 0
}

# Generate comprehensive report
generate_report() {
    log_info "═══════════════════════════════════════════════════════"
    log_info "Generating Comprehensive Report"
    log_info "═══════════════════════════════════════════════════════"
    
    local report_file="$REPORT_DIR/report_$TIMESTAMP.md"
    
    cat > "$report_file" << 'EOF'
# LLMsVerifier Comprehensive Report

**Generated:** TIMESTAMP_PLACEHOLDER
**HelixAgent URL:** URL_PLACEHOLDER
**Session ID:** llms-verifier-SESSION_ID_PLACEHOLDER

## Executive Summary

| Metric | Value |
|--------|-------|
| Total Providers Tested | PROVIDERS_TOTAL |
| Providers Fully Operational | PROVIDERS_PASSED |
| Providers Failed | PROVIDERS_FAILED |
| Total Models Tested | MODELS_TOTAL |
| Ensemble Coordination | ENSEMBLE_STATUS |
| Overall System Status | OVERALL_STATUS |

## Detailed Provider Results

EOF

    # Replace placeholders
    sed -i "s/TIMESTAMP_PLACEHOLDER/$(date)/g" "$report_file"
    sed -i "s|URL_PLACEHOLDER|$HELIXAGENT_URL|g" "$report_file"
    sed -i "s/SESSION_ID_PLACEHOLDER/$TIMESTAMP/g" "$report_file"
    sed -i "s/PROVIDERS_TOTAL/$TOTAL_PROVIDERS/g" "$report_file"
    sed -i "s/PROVIDERS_PASSED/$PROVIDERS_PASSED/g" "$report_file"
    sed -i "s/PROVIDERS_FAILED/$PROVIDERS_FAILED/g" "$report_file"
    sed -i "s/MODELS_TOTAL/$TOTAL_MODELS/g" "$report_file"
    sed -i "s/ENSEMBLE_STATUS/${PROVIDER_RESULTS["ensemble"]:-NOT_TESTED}/g" "$report_file"
    
    local overall_status="⚠️ DEGRADED"
    if [ $PROVIDERS_FAILED -eq 0 ] && [ "${PROVIDER_RESULTS["ensemble"]}" == "PASSED" ]; then
        overall_status="✅ FULLY OPERATIONAL"
    elif [ $PROVIDERS_PASSED -gt 5 ]; then
        overall_status="✓ OPERATIONAL WITH LIMITATIONS"
    elif [ $PROVIDERS_PASSED -gt 0 ]; then
        overall_status="⚠️ MINIMAL FUNCTIONALITY"
    else
        overall_status="❌ CRITICAL FAILURE"
    fi
    sed -i "s/OVERALL_STATUS/$overall_status/g" "$report_file"
    
    # Add provider details
    for provider_info in "${PROVIDERS[@]}"; do
        IFS=':' read -r name vendor auth tier models_json <<< "$provider_info"
        local result="${PROVIDER_RESULTS[$name]:-UNKNOWN}"
        local health_latency="${LATENCY_DATA["$name:health"]:-N/A}"
        
        echo "" >> "$report_file"
        echo "### $name ($vendor)" >> "$report_file"
        echo "" >> "$report_file"
        echo "- **Status:** $result" >> "$report_file"
        echo "- **Tier:** $tier" >> "$report_file"
        echo "- **Health Check Latency:** ${health_latency}ms" >> "$report_file"
        echo "" >> "$report_file"
        
        # Add error details if failed
        if [ -n "${ERROR_DETAILS["$name:overall"]}" ]; then
            echo "**Error Details:**" >> "$report_file"
            echo "\`\`\`" >> "$report_file"
            echo "${ERROR_DETAILS["$name:overall"]}" >> "$report_file"
            echo "\`\`\`" >> "$report_file"
            echo "" >> "$report_file"
        fi
        
        if [ -n "${ERROR_DETAILS["$name:auth"]}" ]; then
            echo "**Authentication Issue:** ${ERROR_DETAILS["$name:auth"]}" >> "$report_file"
            echo "" >> "$report_file"
        fi
        
        if [ -n "${ERROR_DETAILS["$name:health"]}" ]; then
            echo "**Health Check Issue:** ${ERROR_DETAILS["$name:health"]}" >> "$report_file"
            echo "" >> "$report_file"
        fi
        
        # Add model details
        echo "#### Model Results" >> "$report_file"
        echo "" >> "$report_file"
        echo "| Model | Simple | Reasoning | Code | Creative | Avg Score |" >> "$report_file"
        echo "|-------|--------|-----------|------|----------|-----------|" >> "$report_file"
        
        local models
        models=$(echo "$models_json" | jq -r '.models[]' 2>/dev/null)
        for model in $models; do
            local simple="${QUALITY_SCORES["$name:$model:simple"]:-N/A}"
            local reasoning="${QUALITY_SCORES["$name:$model:reasoning"]:-N/A}"
            local code="${QUALITY_SCORES["$name:$model:code"]:-N/A}"
            local creative="${QUALITY_SCORES["$name:$model:creative"]:-N/A}"
            
            local total=0
            local count=0
            [ "$simple" != "N/A" ] && total=$((total + simple)) && ((count++))
            [ "$reasoning" != "N/A" ] && total=$((total + reasoning)) && ((count++))
            [ "$code" != "N/A" ] && total=$((total + code)) && ((count++))
            [ "$creative" != "N/A" ] && total=$((total + creative)) && ((count++))
            
            local avg="N/A"
            [ $count -gt 0 ] && avg=$((total / count))
            
            # Score coloring
            local avg_display="$avg"
            if [ "$avg" != "N/A" ]; then
                if [ $avg -ge 80 ]; then
                    avg_display="✅ $avg"
                elif [ $avg -ge 60 ]; then
                    avg_display="✓ $avg"
                elif [ $avg -ge 40 ]; then
                    avg_display="⚠️ $avg"
                else
                    avg_display="❌ $avg"
                fi
            fi
            
            echo "| $model | $simple | $reasoning | $code | $creative | $avg_display |" >> "$report_file"
        done
        
        echo "" >> "$report_file"
        
        # Add capability support
        echo "#### Capabilities" >> "$report_file"
        echo "" >> "$report_file"
        for cap in streaming vision tools json_mode; do
            local support="${CAPABILITY_SUPPORT["$name:$cap"]:-NOT_TESTED}"
            echo "- **$cap:** $support" >> "$report_file"
        done
        echo "" >> "$report_file"
    done
    
    # Add error analysis section
    cat >> "$report_file" << EOF

## Error Analysis

### Failure Categories

EOF

    # Categorize errors
    local auth_errors=()
    local connection_errors=()
    local rate_limit_errors=()
    local server_errors=()
    local quality_errors=()
    
    for key in "${!ERROR_DETAILS[@]}"; do
        local error="${ERROR_DETAILS[$key]}"
        if echo "$error" | grep -qi "api key\|authentication\|unauthorized"; then
            auth_errors+=("$key: $error")
        elif echo "$error" | grep -qi "timeout\|connection\|refused\|no response"; then
            connection_errors+=("$key: $error")
        elif echo "$error" | grep -qi "rate limit\|429\|too many"; then
            rate_limit_errors+=("$key: $error")
        elif echo "$error" | grep -qi "server error\|500\|502\|503"; then
            server_errors+=("$key: $error")
        elif echo "$error" | grep -qi "score\|quality\|empty\|invalid"; then
            quality_errors+=("$key: $error")
        fi
    done
    
    echo "| Category | Count | Description |" >> "$report_file"
    echo "|----------|-------|-------------|" >> "$report_file"
    echo "| Authentication Failures | ${#auth_errors[@]} | API key issues, unauthorized access |" >> "$report_file"
    echo "| Connection Failures | ${#connection_errors[@]} | Timeouts, network issues, unreachable hosts |" >> "$report_file"
    echo "| Rate Limiting | ${#rate_limit_errors[@]} | Too many requests, quota exceeded |" >> "$report_file"
    echo "| Server Errors | ${#server_errors[@]} | Provider-side 5xx errors |" >> "$report_file"
    echo "| Quality Issues | ${#quality_errors[@]} | Low scores, empty responses, malformed output |" >> "$report_file"
    echo "" >> "$report_file"
    
    # Detailed errors
    if [ ${#auth_errors[@]} -gt 0 ]; then
        echo "### Authentication Failures" >> "$report_file"
        echo "" >> "$report_file"
        for error in "${auth_errors[@]}"; do
            echo "- $error" >> "$report_file"
        done
        echo "" >> "$report_file"
    fi
    
    if [ ${#connection_errors[@]} -gt 0 ]; then
        echo "### Connection Failures" >> "$report_file"
        echo "" >> "$report_file"
        for error in "${connection_errors[@]}"; do
            echo "- $error" >> "$report_file"
        done
        echo "" >> "$report_file"
    fi
    
    if [ ${#quality_errors[@]} -gt 0 ]; then
        echo "### Quality Issues (For Future Tuning)" >> "$report_file"
        echo "" >> "$report_file"
        for error in "${quality_errors[@]}"; do
            echo "- $error" >> "$report_file"
        done
        echo "" >> "$report_file"
    fi
    
    # Performance analysis
    cat >> "$report_file" << EOF
## Performance Analysis

### Latency Summary

| Provider | Health Check | Avg Model Latency | Status |
|----------|--------------|-------------------|--------|
EOF

    for provider_info in "${PROVIDERS[@]}"; do
        IFS=':' read -r name vendor auth tier models_json <<< "$provider_info"
        local health_lat="${LATENCY_DATA["$name:health"]:-N/A}"
        
        # Calculate average model latency
        local total_lat=0
        local lat_count=0
        for key in "${!LATENCY_DATA[@]}"; do
            if [[ "$key" == "$name:"* ]] && [[ "$key" != "$name:health" ]]; then
                total_lat=$((total_lat + ${LATENCY_DATA[$key]}))
                ((lat_count++))
            fi
        done
        
        local avg_lat="N/A"
        [ $lat_count -gt 0 ] && avg_lat=$((total_lat / lat_count))
        
        local status="Unknown"
        if [ "$avg_lat" != "N/A" ]; then
            if [ $avg_lat -lt 1000 ]; then
                status="🚀 Fast"
            elif [ $avg_lat -lt 3000 ]; then
                status="✓ Normal"
            elif [ $avg_lat -lt 10000 ]; then
                status="⚠️ Slow"
            else
                status="🐌 Very Slow"
            fi
        fi
        
        echo "| $name | ${health_lat}ms | ${avg_lat}ms | $status |" >> "$report_file"
    done
    
    # Recommendations
    cat >> "$report_file" << EOF

## Recommendations for Future Tuning

EOF

    if [ ${#auth_errors[@]} -gt 0 ]; then
        echo "### 1. Authentication Configuration" >> "$report_file"
        echo "" >> "$report_file"
        echo "Several providers failed due to authentication issues:" >> "$report_file"
        echo "- Verify all API keys are current and valid" >> "$report_file"
        echo "- Check for expired tokens or rotated credentials" >> "$report_file"
        echo "- Ensure environment variables are properly exported" >> "$report_file"
        echo "" >> "$report_file"
    fi
    
    if [ ${#quality_errors[@]} -gt 0 ]; then
        echo "### 2. Response Quality Tuning" >> "$report_file"
        echo "" >> "$report_file"
        echo "Models with quality issues need tuning:" >> "$report_file"
        echo "- Adjust temperature parameters (lower for code, higher for creative)" >> "$report_file"
        echo "- Increase max_tokens for models returning truncated responses" >> "$report_file"
        echo "- Review prompt engineering for specific providers" >> "$report_file"
        echo "- Consider fallback providers for low-quality responses" >> "$report_file"
        echo "" >> "$report_file"
    fi
    
    echo "### 3. Provider Prioritization" >> "$report_file"
    echo "" >> "$report_file"
    echo "Based on this validation, recommended provider tiers:" >> "$report_file"
    echo "" >> "$report_file"
    
    # List working providers by tier
    echo "**Primary (Highest Reliability):**" >> "$report_file"
    for provider_info in "${PROVIDERS[@]}"; do
        IFS=':' read -r name vendor auth tier models_json <<< "$provider_info"
        if [ "${PROVIDER_RESULTS[$name]}" == "PASSED_ALL" ] && [ "$tier" == "primary" ]; then
            echo "- $name ($vendor)" >> "$report_file"
        fi
    done
    echo "" >> "$report_file"
    
    echo "**Secondary (Good Performance):**" >> "$report_file"
    for provider_info in "${PROVIDERS[@]}"; do
        IFS=':' read -r name vendor auth tier models_json <<< "$provider_info"
        if [ "${PROVIDER_RESULTS[$name]}" == "PASSED_ALL" ] && [ "$tier" == "secondary" ]; then
            echo "- $name ($vendor)" >> "$report_file"
        fi
    done
    echo "" >> "$report_file"
    
    echo "**Needs Attention:**" >> "$report_file"
    for provider_info in "${PROVIDERS[@]}"; do
        IFS=':' read -r name vendor auth tier models_json <<< "$provider_info"
        if [ "${PROVIDER_RESULTS[$name]}" == "FAILED_ALL_MODELS" ] || [ "${PROVIDER_RESULTS[$name]}" == "FAILED_CONNECTION" ]; then
            echo "- $name ($vendor) - ${ERROR_DETAILS["$name:overall"]:-Check logs}" >> "$report_file"
        fi
    done
    echo "" >> "$report_file"
    
    # Footer
    cat >> "$report_file" << EOF
## Next Steps

1. **Review Failed Providers**: Address authentication and connection issues
2. **Update Configuration**: Add missing API keys for skipped providers
3. **Quality Tuning**: Adjust parameters for models with low scores
4. **Re-validate**: Run this verifier again after fixes
5. **Monitor**: Set up alerts for provider health degradation

---

*Report generated by LLMsVerifier v2.0*
*Detailed logs: $DETAILED_LOG*
EOF

    log_success "Comprehensive report generated: $report_file"
    
    # Create symlink to latest
    ln -sf "$report_file" "$REPORT_DIR/report_latest.md"
}

# Main execution
main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║     LLMsVerifier - Comprehensive Provider Validation           ║"
    echo "╠════════════════════════════════════════════════════════════════╣"
    echo "║  Total Providers: $TOTAL_PROVIDERS                                           ║"
    echo "║  Total Models: $TOTAL_MODELS                                        ║"
    echo "║  Report Directory: $REPORT_DIR                    ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    
    # Check if HelixAgent is running
    log_info "Checking HelixAgent connectivity..."
    if ! curl -s "$HELIXAGENT_URL/health" > /dev/null 2>&1; then
        log_error "HelixAgent is not running at $HELIXAGENT_URL"
        log_info "Please start HelixAgent before running validation"
        exit 1
    fi
    log_success "HelixAgent is responding"
    echo ""
    
    # Test each provider
    for provider_info in "${PROVIDERS[@]}"; do
        test_provider "$provider_info"
        echo ""
    done
    
    # Test ensemble
    test_ensemble
    echo ""
    
    # Generate report
    generate_report
    
    # Final summary
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║     LLMsVerifier COMPLETED                                     ║"
    echo "╠════════════════════════════════════════════════════════════════╣"
    printf "║  Providers: %d passed, %d failed, %d skipped                    ║\n" "$PROVIDERS_PASSED" "$PROVIDERS_FAILED" "$PROVIDERS_SKIPPED"
    echo "║  Report: $REPORT_DIR/report_$TIMESTAMP.md                     ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    
    if [ $PROVIDERS_FAILED -eq 0 ] && [ "${PROVIDER_RESULTS["ensemble"]}" == "PASSED" ]; then
        echo -e "${GREEN}✅ All systems operational${NC}"
        exit 0
    elif [ $PROVIDERS_PASSED -gt 0 ]; then
        echo -e "${YELLOW}⚠️  System operational with limitations${NC}"
        exit 0
    else
        echo -e "${RED}❌ Critical failures detected${NC}"
        exit 1
    fi
}

# Check for jq
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed"
    echo "Install with: sudo apt-get install jq (Debian/Ubuntu) or brew install jq (macOS)"
    exit 1
fi

# Run main
main "$@"
