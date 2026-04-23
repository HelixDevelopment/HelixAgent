#!/bin/bash
# HelixAgent Challenges - Run All Challenges in Sequence
# Usage: ./scripts/run_all_challenges.sh [options]
#
# This script automatically:
# 1. Builds HelixAgent binary if needed
# 2. Starts all required infrastructure (HelixAgent server)
# 3. Runs all challenges
# 4. Reports final results

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Load environment from project root first (primary location for API keys)
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
fi

# Then load challenges-specific .env (can override or add settings)
if [ -f "$CHALLENGES_DIR/.env" ]; then
    set -a
    source "$CHALLENGES_DIR/.env"
    set +a
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_phase() { echo -e "${PURPLE}[PHASE]${NC} $1"; }

# Track started services for cleanup
HELIXAGENT_PID=""
STARTED_SERVICES=()

#===============================================================================
# CLEANUP HANDLER
#===============================================================================
cleanup() {
    print_info "Cleaning up..."

    # Stop HelixAgent if we started it
    if [ -n "$HELIXAGENT_PID" ] && kill -0 "$HELIXAGENT_PID" 2>/dev/null; then
        print_info "Stopping HelixAgent (PID: $HELIXAGENT_PID)..."
        kill "$HELIXAGENT_PID" 2>/dev/null || true
        wait "$HELIXAGENT_PID" 2>/dev/null || true
    fi

    # Remove PID file
    rm -f "$CHALLENGES_DIR/results/helixagent_challenges.pid"
}

trap cleanup EXIT

#===============================================================================
# INFRASTRUCTURE FUNCTIONS
#===============================================================================

# Build HelixAgent binary if needed
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
        if [ -x "$PROJECT_ROOT/bin/helixagent" ]; then
            print_success "HelixAgent built successfully"
            return 0
        elif [ -x "$PROJECT_ROOT/helixagent" ]; then
            print_success "HelixAgent built successfully"
            return 0
        fi
    fi

    print_error "Failed to build HelixAgent"
    return 1
}

# Check if HelixAgent is running
check_helixagent() {
    local port="${HELIXAGENT_PORT:-7061}"
    local host="${HELIXAGENT_HOST:-localhost}"

    if curl -s --max-time 60 "http://$host:$port/health" > /dev/null 2>&1; then
        return 0
    elif curl -s --max-time 60 "http://$host:$port/v1/models" > /dev/null 2>&1; then
        return 0
    fi
    return 1
}

# Start HelixAgent server
start_helixagent() {
    print_phase "Starting HelixAgent Server"

    local port="${HELIXAGENT_PORT:-7061}"
    local host="${HELIXAGENT_HOST:-localhost}"

    # Check if already running
    if check_helixagent; then
        print_success "HelixAgent already running on $host:$port"
        return 0
    fi

    # Find binary
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

    # Create results directory if needed
    mkdir -p "$CHALLENGES_DIR/results"

    # Start HelixAgent in background
    nohup "$binary" > "$CHALLENGES_DIR/results/helixagent_challenges.log" 2>&1 &
    HELIXAGENT_PID=$!
    echo $HELIXAGENT_PID > "$CHALLENGES_DIR/results/helixagent_challenges.pid"
    STARTED_SERVICES+=("helixagent")

    # Wait for startup (provider verification with real API calls takes ~120s, plus setup)
    print_info "Waiting for HelixAgent to start (provider verification takes ~2 minutes)..."
    local max_attempts=180
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
    print_error "Check log: $CHALLENGES_DIR/results/helixagent_challenges.log"
    cat "$CHALLENGES_DIR/results/helixagent_challenges.log" | tail -20
    return 1
}

# Start all required infrastructure
start_infrastructure() {
    print_info "=========================================="
    print_info "  Starting Infrastructure"
    print_info "=========================================="
    echo ""

    # Build HelixAgent if needed
    if ! build_helixagent; then
        print_error "Failed to build HelixAgent - cannot continue"
        exit 1
    fi

    # Start HelixAgent
    if ! start_helixagent; then
        print_error "Failed to start HelixAgent - cannot continue"
        exit 1
    fi

    echo ""
    print_success "All infrastructure started successfully"
    echo ""
}

# All 493 challenges in tiered dependency order
CHALLENGES=(
    #===========================================================================
    # TIER 1 (Critical): Infrastructure, boot, provider, build, container
    #===========================================================================

    # Infrastructure (no dependencies)
    "health_monitoring"
    "configuration_loading"
    "caching_layer"
    "database_operations"
    "authentication"
    "plugin_system"
    "sanity_check_challenge"
    "full_system_boot_challenge"
    "unified_service_boot_challenge"
    "comprehensive_infrastructure_challenge"
    "health_endpoints_comprehensive_challenge"
    "service_health_fixes_challenge"
    "configs_use_challenge"
    "database_connection_pool_challenge"
    "database_schema_validation_challenge"
    "database_module_challenge"
    "cache_invalidation_challenge"
    "cache_module_challenge"
    "redis_cache_validation_challenge"
    "sql_schema_challenge"

    # Container and build challenges
    "ci_container_build_challenge"
    "container_centralization_challenge"
    "container_placement_challenge"
    "container_remote_distribution_challenge"
    "remote_deployment_challenge"
    "remote_distribution_precedence_challenge"
    "remote_services_challenge"
    "release_build_challenge"
    "router_completeness_challenge"
    "resource_limits_challenge"

    # Security (depends on caching_layer, authentication)
    "rate_limiting"
    "input_validation"
    "security_scanning_challenge"
    "security_scanning_phase3_challenge"
    "security_api_key_validation_challenge"
    "security_audit_logging_challenge"
    "security_authentication_challenge"
    "security_authorization_challenge"
    "security_compliance_challenge"
    "security_cors_policies_challenge"
    "security_csrf_protection_challenge"
    "security_data_encryption_challenge"
    "security_ddos_protection_challenge"
    "security_input_validation_challenge"
    "security_jwt_tokens_challenge"
    "security_module_challenge"
    "security_oauth_flows_challenge"
    "security_output_sanitization_challenge"
    "security_penetration_testing_challenge"
    "security_rate_limiting_challenge"
    "security_secret_management_challenge"
    "security_secure_headers_challenge"
    "security_sql_injection_challenge"
    "security_vulnerability_scanning_challenge"
    "security_xss_prevention_challenge"
    "jwt_security_challenge"
    "csrf_protection_challenge"
    "sql_injection_challenge"
    "xss_prevention_challenge"
    "snyk_automated_scanning_challenge"
    "sonarqube_automated_scanning_challenge"

    # Providers (no dependencies)
    "provider_claude"
    "provider_deepseek"
    "provider_gemini"
    "provider_ollama"
    "provider_openrouter"
    "provider_qwen"
    "provider_zai"
    "cerebras_provider_challenge"
    "mistral_provider_challenge"
    "zai_provider_challenge"
    "zen_provider_challenge"
    "single_provider_challenge"
    "new_providers_validation_challenge"
    "ollama_explicit_enable_challenge"
    "claude_46_models_challenge"
    "claude_auth_challenge"

    # Core verification
    "provider_verification"
    "provider_reliability"
    "verification_failure_reasons"
    "subscription_detection"
    "provider_comprehensive"
    "provider_verification_comprehensive_challenge"
    "provider_url_consistency_challenge"
    "provider_auth_challenge"
    "provider_authentication_challenge"
    "provider_api_compatibility_challenge"
    "provider_capability_detection_challenge"
    "provider_cost_optimization_challenge"
    "provider_discovery_challenge"
    "provider_error_handling_challenge"
    "provider_failover_challenge"
    "provider_health_checks_challenge"
    "provider_load_balancing_challenge"
    "provider_model_listing_challenge"
    "provider_performance_monitoring_challenge"
    "provider_rate_limit_handling_challenge"
    "provider_retry_logic_challenge"
    "provider_scoring_challenge"
    "provider_selection_strategy_challenge"
    "provider_timeout_management_challenge"
    "integration_providers_challenge"
    "verified_provider_instance_challenge"
    "unified_verification_challenge"
    "verification_report_challenge"
    "llms_reevaluation_challenge"
    "llmsverifier_cliagents_challenge"
    "startup_scoring_challenge"
    "startup_verifier_debate_team_challenge"
    "capability_detection_challenge"

    # CLI proxy mechanism (OAuth/free providers)
    "cli_proxy"
    "advanced_provider_access"
    "oauth_credentials_challenge"
    "oauth_cli_fallback_challenge"
    "oauth_free_models_challenge"
    "oauth_provider_verification_challenge"
    "qwen_oauth_refresh_challenge"
    "free_provider_fallback_challenge"
    "zen_cli_facade_challenge"

    #===========================================================================
    # TIER 2 (Standard): Feature validation, protocol, debate, memory,
    #                     formatters, MCP, security challenges
    #===========================================================================

    # Protocols (no dependencies)
    "mcp_protocol"
    "lsp_protocol"
    "acp_protocol"
    "protocol_challenge"
    "protocol_comprehensive_challenge"
    "protocol_acp_integration_challenge"
    "protocol_anthropic_format_challenge"
    "protocol_authentication_challenge"
    "protocol_backward_compatibility_challenge"
    "protocol_cognee_integration_challenge"
    "protocol_custom_protocols_challenge"
    "protocol_embeddings_api_challenge"
    "protocol_error_formats_challenge"
    "protocol_forward_compatibility_challenge"
    "protocol_graphql_challenge"
    "protocol_grpc_challenge"
    "protocol_integrations_challenge"
    "protocol_json_rpc_challenge"
    "protocol_lsp_integration_challenge"
    "protocol_mcp_integration_challenge"
    "protocol_mcps_challenge"
    "protocol_missions_challenge"
    "protocol_openai_compatibility_challenge"
    "protocol_rest_api_challenge"
    "protocol_streaming_sse_challenge"
    "protocol_version_negotiation_challenge"
    "protocol_vision_api_challenge"
    "protocol_websocket_challenge"
    "toon_protocol_challenge"

    # Cloud integrations
    "cloud_aws_bedrock"
    "cloud_gcp_vertex"
    "cloud_azure_openai"

    # Core features (depends on provider_verification)
    "ensemble_voting"
    "embeddings_service"
    "streaming_responses"
    "model_metadata"
    "intent_based_ensemble_routing_challenge"
    "semantic_intent_challenge"
    "enhanced_intent_mechanism_challenge"
    "fallback_mechanism_challenge"
    "fallback_error_reporting_challenge"
    "fallback_visualization_challenge"
    "canned_response_fallback_challenge"
    "reliable_fallback_challenge"
    "multipass_validation_challenge"
    "llm_scoring_challenge"
    "llm_tool_generation_challenge"
    "feature_flags_challenge"
    "output_formatting_challenge"
    "dialogue_rendering_challenge"
    "followup_response_challenge"
    "event_driven_challenge"
    "parallel_execution_challenge"

    # Chat challenges
    "simple_chat_single_turn_challenge"
    "simple_chat_multi_turn_challenge"
    "chat_concurrent_requests_challenge"
    "chat_conversation_branching_challenge"
    "chat_error_handling_challenge"
    "chat_max_tokens_challenge"
    "chat_max_tokens_enforcement_challenge"
    "chat_model_selection_challenge"
    "chat_rate_limiting_challenge"
    "chat_response_caching_challenge"
    "chat_special_characters_challenge"
    "chat_system_message_challenge"
    "chat_temperature_control_challenge"
    "chat_temperature_variation_challenge"
    "chat_tool_calling_challenge"
    "chat_tool_function_calling_challenge"
    "chat_with_context_window_challenge"
    "streaming_chat_basic_challenge"
    "streaming_chat_cancellation_challenge"

    # Debate (depends on provider_verification)
    "ai_debate_formation"
    "ai_debate_workflow"
    "ai_debate_team_challenge"
    "ai_debate_verification_challenge"
    "debate_11_roles_challenge"
    "debate_adversarial_dynamics_challenge"
    "debate_approval_gate_challenge"
    "debate_benchmark_integration_challenge"
    "debate_comm_logging_challenge"
    "debate_condorcet_voting_challenge"
    "debate_confidence_scoring_challenge"
    "debate_consensus_building_challenge"
    "debate_critique_validation_challenge"
    "debate_deadlock_detection_challenge"
    "debate_dehallucination_challenge"
    "debate_dialogue_challenge"
    "debate_error_recovery_challenge"
    "debate_fallback_legacy_challenge"
    "debate_git_integration_challenge"
    "debate_integration_comprehensive_challenge"
    "debate_integration_language_detection_challenge"
    "debate_integration_metadata_challenge"
    "debate_integration_roles_challenge"
    "debate_integration_test_driven_challenge"
    "debate_integration_tools_challenge"
    "debate_integration_validation_challenge"
    "debate_model_diversity_challenge"
    "debate_multi_round_challenge"
    "debate_orchestrator_challenge"
    "debate_performance_optimization_challenge"
    "debate_performance_optimizer_challenge"
    "debate_persistence_challenge"
    "debate_position_assignment_challenge"
    "debate_provenance_audit_challenge"
    "debate_quality_metrics_challenge"
    "debate_reflexion_challenge"
    "debate_response_aggregation_challenge"
    "debate_round_orchestration_challenge"
    "debate_self_evolvement_challenge"
    "debate_synthesis_generation_challenge"
    "debate_team_dynamic_selection_challenge"
    "debate_team_formation_challenge"
    "debate_team_models_challenge"
    "debate_testing_integration_challenge"
    "debate_tool_triggering_challenge"
    "debate_tree_topology_challenge"
    "debate_voting_mechanism_challenge"
    "runtime_debate_system_challenge"
    "system_debate_validation_challenge"
    "test_driven_debate_challenge"
    "verified_providers_debate_integration_challenge"

    # Intent & Constitution (depends on debate)
    "constitution_watcher"
    "speckit_auto_activation"
    "constitution_management_challenge"
    "speckit_comprehensive_validation_challenge"

    # API (depends on provider_verification)
    "openai_compatibility"
    "grpc_api"
    "api_quality_test"
    "curl_api_challenge"
    "grpc_service_challenge"
    "graphql_integration_challenge"
    "new_endpoints_challenge"

    # Optimization (depends on embeddings)
    "optimization_semantic_cache"
    "optimization_structured_output"
    "optimization_module_challenge"

    # Integration
    "cognee_integration"
    "cognee_full_integration"
    "bigdata_integration"
    "cognee_verification_challenge"
    "bigdata_comprehensive_challenge"
    "bigdata_pipeline_challenge"
    "flink_integration_challenge"
    "integration_challenge"
    "junie_integration_challenge"
    "minio_storage_challenge"
    "qdrant_vector_challenge"

    # MCP challenges
    "mcp_all_servers_challenge"
    "mcp_comprehensive_challenge"
    "mcp_connectivity_challenge"
    "mcp_containerized_challenge"
    "mcp_full_system_challenge"
    "mcp_module_challenge"
    "mcps_challenge"
    "mcp_server_integration_challenge"
    "mcp_servers_challenge"
    "mcp_servers_usage_challenge"
    "mcp_sse_connectivity_challenge"
    "mcp_submodules_challenge"
    "mcp_validation_challenge"
    "external_mcp_servers_challenge"

    # Memory challenges
    "memory_system_challenge"
    "mem0_migration_challenge"
    "memory_cache_integration_challenge"
    "memory_chunking_strategies_challenge"
    "memory_consolidation_challenge"
    "memory_context_management_challenge"
    "memory_embedding_generation_challenge"
    "memory_entity_extraction_challenge"
    "memory_graph_navigation_challenge"
    "memory_hybrid_search_challenge"
    "memory_module_challenge"
    "memory_race_challenge"
    "memory_rag_pipeline_challenge"
    "memory_reranking_challenge"
    "memory_retrieval_challenge"
    "memory_safety_phase4_challenge"
    "memory_scoping_challenge"
    "memory_semantic_search_challenge"
    "memory_storage_challenge"
    "memory_vector_similarity_challenge"

    # Formatters challenges
    "formatters_comprehensive_challenge"
    "formatter_services_challenge"
    "formatters_module_challenge"

    # RAG challenges
    "rag_comprehensive_challenge"
    "rag_integration_challenge"
    "rag_module_challenge"
    "rags_challenge"

    # Messaging challenges
    "messaging_hybrid_challenge"
    "messaging_integration_challenge"
    "messaging_kafka_challenge"
    "messaging_migration_challenge"
    "messaging_module_challenge"
    "messaging_rabbitmq_challenge"

    # Error handling challenges
    "error_authentication_errors_challenge"
    "error_cache_failures_challenge"
    "error_circuit_breaker_challenge"
    "error_classification_challenge"
    "error_concurrent_errors_challenge"
    "error_connection_failures_challenge"
    "error_database_errors_challenge"
    "error_fallback_chains_challenge"
    "error_graceful_degradation_challenge"
    "error_logging_challenge"
    "error_malformed_requests_challenge"
    "error_network_errors_challenge"
    "error_provider_failures_challenge"
    "error_rate_limit_errors_challenge"
    "error_recovery_strategies_challenge"
    "error_resource_exhaustion_challenge"
    "error_retry_mechanisms_challenge"
    "error_timeout_handling_challenge"
    "error_user_feedback_challenge"
    "error_validation_errors_challenge"

    # Resilience
    "circuit_breaker"
    "error_handling"
    "concurrent_access"
    "graceful_shutdown"
    "circuit_breaker_metrics_challenge"
    "resilience_challenge"
    "resilience_auto_scaling_challenge"
    "resilience_backpressure_challenge"
    "resilience_bulkhead_isolation_challenge"
    "resilience_cascading_failures_challenge"
    "resilience_chaos_engineering_challenge"
    "resilience_circuit_breaker_challenge"
    "resilience_data_backup_challenge"
    "resilience_dependency_failures_challenge"
    "resilience_disaster_recovery_challenge"
    "resilience_failover_challenge"
    "resilience_graceful_shutdown_challenge"
    "resilience_health_monitoring_challenge"
    "resilience_network_partitions_challenge"
    "resilience_partial_failures_challenge"
    "resilience_resource_limits_challenge"
    "resilience_retry_logic_challenge"
    "resilience_self_healing_challenge"
    "resilience_service_degradation_challenge"
    "resilience_timeout_handling_challenge"
    "resilience_zero_downtime_challenge"

    # Session (depends on caching, auth)
    "session_management"
    "conversation_errors_fix_challenge"

    # Tool challenges
    "tool_call_argument_validation_challenge"
    "tool_call_validation_challenge"
    "tool_execution_challenge"
    "tool_handlers_comprehensive_challenge"
    "all_tools_validation_challenge"

    # Skills challenges
    "skills_challenge"
    "skills_comprehensive_challenge"
    "skills_system_challenge"

    # Validation (depends on main challenge)
    "opencode"
    "opencode_init"
    "opencode_cognee_e2e_challenge"
    "opencode_conversation_challenge"
    "opencode_mcp_status_challenge"
    "validation_pipeline_challenge"

    # CLI agent challenges
    "cli_agents_challenge"
    "cli_agent_config_challenge"
    "cli_agent_integration_challenge"
    "cli_agent_mcp_challenge"
    "cli_agent_plugin_e2e_challenge"
    "cli_agents_formatters_challenge"
    "cli_schema_validation_challenge"
    "cli_text_editor_challenge"
    "all_cli_agents_challenge"
    "all_agents_e2e_challenge"

    # Content generation challenges
    "content_generation_challenge"

    # Extracted module challenges (Phase 1-4)
    "auth_module_challenge"
    "concurrency_module_challenge"
    "eventbus_module_challenge"
    "observability_module_challenge"
    "storage_module_challenge"
    "streaming_module_challenge"
    "streaming_types_challenge"
    "embeddings_module_challenge"
    "vectordb_module_challenge"
    "plugins_module_challenge"
    "plugin_events_challenge"
    "plugin_integration_challenge"
    "plugin_transport_challenge"
    "plugin_ui_challenge"
    "challenge_module_integration_challenge"

    # AI/ML modules (Phase 5 extracted modules)
    "agentic_challenge"
    "agentic_module_challenge"
    "llmops_challenge"
    "llmops_module_challenge"
    "selfimprove_challenge"
    "selfimprove_module_challenge"
    "planning_challenge"
    "planning_module_challenge"
    "benchmark_challenge"
    "benchmark_module_challenge"

    # Cognitive modules (Phase 6)
    "helixmemory"

    # Specification modules (Phase 7)
    "helixspecifier"

    # Background task challenges
    "background_cli_rendering_challenge"
    "background_endless_process_challenge"
    "background_full_integration_challenge"
    "background_notifications_challenge"
    "background_resource_monitor_challenge"
    "background_stuck_detection_challenge"
    "background_task_queue_challenge"
    "background_worker_pool_challenge"

    # Observability and monitoring
    "monitoring_dashboard_challenge"
    "monitoring_system_challenge"
    "comprehensive_monitoring_challenge"
    "opentelemetry_tracing_challenge"
    "prometheus_metrics_challenge"

    # Documentation
    "documentation_completeness_challenge"
    "documentation_phase6_challenge"

    # Quality assurance
    "lazy_init_challenge"
    "adapter_coverage_challenge"
    "coverage_gate_challenge"
    "dead_code_elimination_challenge"
    "dead_code_verification_challenge"
    "goroutine_lifecycle_challenge"
    "race_condition_challenge"
    "pprof_memory_profiling_challenge"
    "lazy_loading_challenge"
    "lazy_loading_validation_challenge"
    "test_coverage_phase2_challenge"
    "test_coverage_completeness_challenge"
    "concurrency_safety_comprehensive_challenge"
    "concurrency_fixes_validation_challenge"
    "port_registry_challenge"
    "security_scan_validation_challenge"
    "documentation_completeness_validation_challenge"
    "helixagent_plugins_challenge"
    "helixqa_validation_challenge"
    "helixqa_integration_challenge"
    "system_compliance_challenge"
    "final_validation_phase8_challenge"
    "comprehensive_quality_challenge"
    "main_challenge"
    "ctop_challenge"
    "websearch_challenge"
    "website_phase7_challenge"
    "kimi_qwen_code_challenge"

    #===========================================================================
    # TIER 3 (Extended): Stress, performance, advanced AI, comprehensive,
    #                     simultaneous challenges
    #===========================================================================

    # Comprehensive provider challenges
    "cloudflare_comprehensive_challenge"
    "cohere_comprehensive_challenge"
    "gemini_comprehensive_challenge"
    "github_models_comprehensive_challenge"
    "groq_comprehensive_challenge"
    "openrouter_comprehensive_challenge"
    "venice_comprehensive_challenge"

    # Comprehensive feature challenges
    "comprehensive_e2e_challenge"
    "comprehensive_real_llm_challenge"
    "comprehensive_streaming_challenge"
    "e2e_workflow_challenge"
    "userflow_comprehensive_challenge"

    # Stress and performance
    "stress_responsiveness_challenge"
    "stress_resilience_challenge"
    "sustained_stress_challenge"
    "concurrent_load_stress_challenge"
    "rate_limit_stress_challenge"
    "performance_baseline_challenge"
    "performance_batch_processing_challenge"
    "performance_caching_effectiveness_challenge"
    "performance_circuit_breaker_impact_challenge"
    "performance_concurrent_load_challenge"
    "performance_connection_pooling_challenge"
    "performance_cpu_utilization_challenge"
    "performance_database_queries_challenge"
    "performance_debate_optimization_challenge"
    "performance_endurance_testing_challenge"
    "performance_fallback_overhead_challenge"
    "performance_latency_measurement_challenge"
    "performance_memory_usage_challenge"
    "performance_monitoring_overhead_challenge"
    "performance_phase5_challenge"
    "performance_provider_selection_speed_challenge"
    "performance_rate_limiting_impact_challenge"
    "performance_response_times_challenge"
    "performance_scaling_behavior_challenge"
    "performance_streaming_efficiency_challenge"
    "performance_stress_testing_challenge"
    "performance_throughput_testing_challenge"

    # Advanced AI features
    "advanced_ai_features_challenge"

    # All-providers simultaneous
    "all_providers_simultaneous_challenge"
)

# Parse options
VERBOSE=""
STOP_ON_FAILURE=true
SKIP_INFRA=false

while [ $# -gt 0 ]; do
    case "$1" in
        -v|--verbose)
            VERBOSE="--verbose"
            ;;
        --continue-on-failure)
            STOP_ON_FAILURE=false
            ;;
        --skip-infra)
            SKIP_INFRA=true
            ;;
        -h|--help)
            echo "Usage: $0 [--verbose] [--continue-on-failure] [--skip-infra]"
            echo ""
            echo "Options:"
            echo "  --verbose              Enable verbose output"
            echo "  --continue-on-failure  Continue even if a challenge fails"
            echo "  --skip-infra           Skip infrastructure startup (assume already running)"
            exit 0
            ;;
    esac
    shift
done

# Main execution
print_info "=========================================="
print_info "  HelixAgent - Run All Challenges"
print_info "=========================================="
print_info "Start time: $(date)"
print_info "Total challenges: ${#CHALLENGES[@]}"
echo ""

# Start infrastructure unless skipped
if [ "$SKIP_INFRA" = false ]; then
    start_infrastructure
else
    print_warning "Skipping infrastructure startup (--skip-infra)"
    if ! check_helixagent; then
        print_error "HelixAgent is not running! Cannot continue."
        print_error "Either start HelixAgent manually or remove --skip-infra"
        exit 1
    fi
    print_success "HelixAgent is running"
fi

echo ""

TOTAL_START=$(date +%s)
PASSED=0
FAILED=0

for challenge in "${CHALLENGES[@]}"; do
    print_info "----------------------------------------"
    print_info "Running: $challenge"
    print_info "----------------------------------------"

    if "$SCRIPT_DIR/run_challenges.sh" "$challenge" $VERBOSE; then
        PASSED=$((PASSED + 1))
        print_success "$challenge completed successfully"
    else
        FAILED=$((FAILED + 1))
        print_error "$challenge failed"

        if [ "$STOP_ON_FAILURE" = true ]; then
            print_error "Stopping due to failure. Use --continue-on-failure to continue."
            break
        fi
    fi
    echo ""
done

TOTAL_END=$(date +%s)
TOTAL_DURATION=$((TOTAL_END - TOTAL_START))

# Generate master summary
print_info "Generating master summary..."
"$SCRIPT_DIR/generate_report.sh" --master-only 2>/dev/null || true

# Final report
echo ""
print_info "=========================================="
print_info "  All Challenges Complete"
print_info "=========================================="
print_info "Total duration: ${TOTAL_DURATION}s"
print_info "Passed: $PASSED / ${#CHALLENGES[@]}"
print_info "Failed: $FAILED / ${#CHALLENGES[@]}"

if [ $FAILED -eq 0 ]; then
    print_success "All challenges passed!"
    exit 0
else
    print_error "$FAILED challenge(s) failed"
    exit 1
fi
