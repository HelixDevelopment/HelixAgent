#!/bin/bash

# ============================================================================
# SuperAgent Integration Test Runner
# ============================================================================
# This script automatically starts all required dependencies (PostgreSQL, Redis)
# using Docker Compose, runs the tests, and cleans up afterwards.
#
# Usage:
#   ./scripts/run-integration-tests.sh [options]
#
# Options:
#   --keep         Keep containers running after tests
#   --verbose      Show detailed output
#   --coverage     Generate coverage report
#   --package PKG  Test specific package (e.g., ./internal/services/...)
#   --help         Show this help message
# ============================================================================

set -e

# Configuration
COMPOSE_FILE="docker-compose.integration.yml"
PROJECT_NAME="superagent-integration-tests"
MAX_WAIT_TIME=60
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default options
KEEP_CONTAINERS=false
VERBOSE=false
COVERAGE=false
TEST_PACKAGE="./..."

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --keep)
            KEEP_CONTAINERS=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --coverage)
            COVERAGE=true
            shift
            ;;
        --package)
            TEST_PACKAGE="$2"
            shift 2
            ;;
        --help)
            echo "SuperAgent Integration Test Runner"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --keep         Keep containers running after tests"
            echo "  --verbose      Show detailed output"
            echo "  --coverage     Generate coverage report"
            echo "  --package PKG  Test specific package (default: ./...)"
            echo "  --help         Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Change to project directory
cd "$PROJECT_DIR"

# Functions
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

cleanup() {
    # Determine docker compose command if not set (for trap context)
    if [ -z "$DOCKER_COMPOSE" ]; then
        if docker compose version &> /dev/null; then
            DOCKER_COMPOSE="docker compose"
        else
            DOCKER_COMPOSE="docker-compose"
        fi
    fi

    if [ "$KEEP_CONTAINERS" = false ]; then
        log_info "Stopping and removing test containers..."
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v --remove-orphans 2>/dev/null || true
        log_success "Cleanup complete"
    else
        log_warning "Containers are still running (--keep flag). Stop them with:"
        echo "  $DOCKER_COMPOSE -f $COMPOSE_FILE -p $PROJECT_NAME down -v"
    fi
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

wait_for_postgres() {
    log_info "Waiting for PostgreSQL to be ready..."
    local count=0
    while [ $count -lt $MAX_WAIT_TIME ]; do
        if $DOCKER_COMPOSE -f "$COMPOSE_FILE" -p "$PROJECT_NAME" exec -T postgres-test pg_isready -U superagent_test -d superagent_test_db > /dev/null 2>&1; then
            log_success "PostgreSQL is ready!"
            return 0
        fi
        count=$((count + 1))
        sleep 1
        if [ "$VERBOSE" = true ]; then
            echo -n "."
        fi
    done
    log_error "PostgreSQL failed to start within $MAX_WAIT_TIME seconds"
    return 1
}

wait_for_redis() {
    log_info "Waiting for Redis to be ready..."
    local count=0
    while [ $count -lt $MAX_WAIT_TIME ]; do
        if $DOCKER_COMPOSE -f "$COMPOSE_FILE" -p "$PROJECT_NAME" exec -T redis-test redis-cli -a test123 ping 2>/dev/null | grep -q "PONG"; then
            log_success "Redis is ready!"
            return 0
        fi
        count=$((count + 1))
        sleep 1
        if [ "$VERBOSE" = true ]; then
            echo -n "."
        fi
    done
    log_error "Redis failed to start within $MAX_WAIT_TIME seconds"
    return 1
}

initialize_database() {
    log_info "Initializing test database schema..."

    # Create a test-specific init script
    $DOCKER_COMPOSE -f "$COMPOSE_FILE" -p "$PROJECT_NAME" exec -T postgres-test psql -U superagent_test -d superagent_test_db <<'EOF'
-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    api_key VARCHAR(255) UNIQUE NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create user_sessions table
CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    context JSONB DEFAULT '{}',
    memory_id UUID,
    status VARCHAR(50) DEFAULT 'active',
    request_count INTEGER DEFAULT 0,
    last_activity TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create llm_providers table
CREATE TABLE IF NOT EXISTS llm_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    type VARCHAR(100) NOT NULL,
    api_key VARCHAR(255),
    base_url VARCHAR(500),
    model VARCHAR(255),
    weight DECIMAL(5,2) DEFAULT 1.0,
    enabled BOOLEAN DEFAULT TRUE,
    config JSONB DEFAULT '{}',
    health_status VARCHAR(50) DEFAULT 'unknown',
    response_time BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create llm_requests table
CREATE TABLE IF NOT EXISTS llm_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES user_sessions(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    messages JSONB NOT NULL DEFAULT '[]',
    model_params JSONB NOT NULL DEFAULT '{}',
    ensemble_config JSONB DEFAULT NULL,
    memory_enhanced BOOLEAN DEFAULT FALSE,
    memory JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    request_type VARCHAR(50) DEFAULT 'completion'
);

-- Create llm_responses table
CREATE TABLE IF NOT EXISTS llm_responses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id UUID REFERENCES llm_requests(id) ON DELETE CASCADE,
    provider_id UUID REFERENCES llm_providers(id) ON DELETE SET NULL,
    provider_name VARCHAR(100) NOT NULL,
    content TEXT NOT NULL,
    confidence DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    tokens_used INTEGER DEFAULT 0,
    response_time BIGINT DEFAULT 0,
    finish_reason VARCHAR(50) DEFAULT 'stop',
    metadata JSONB DEFAULT '{}',
    selected BOOLEAN DEFAULT FALSE,
    selection_score DECIMAL(5,2) DEFAULT 0.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create cognee_memories table
CREATE TABLE IF NOT EXISTS cognee_memories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES user_sessions(id) ON DELETE CASCADE,
    dataset_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(50) DEFAULT 'text',
    content TEXT NOT NULL,
    vector_id VARCHAR(255),
    graph_nodes JSONB DEFAULT '{}',
    search_key VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create model_metadata table for model catalog
CREATE TABLE IF NOT EXISTS model_metadata (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    model_id VARCHAR(255) UNIQUE NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    provider VARCHAR(100) NOT NULL,
    model_type VARCHAR(100),
    description TEXT,
    capabilities JSONB DEFAULT '[]',
    context_length INTEGER,
    max_tokens INTEGER,
    input_price DECIMAL(20,10),
    output_price DECIMAL(20,10),
    supports_streaming BOOLEAN DEFAULT FALSE,
    supports_function_calling BOOLEAN DEFAULT FALSE,
    supports_vision BOOLEAN DEFAULT FALSE,
    supports_json_mode BOOLEAN DEFAULT FALSE,
    quality_score DECIMAL(3,2),
    speed_score DECIMAL(3,2),
    cost_score DECIMAL(3,2),
    enabled BOOLEAN DEFAULT TRUE,
    metadata JSONB DEFAULT '{}',
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_session_token ON user_sessions(session_token);
CREATE INDEX IF NOT EXISTS idx_llm_providers_name ON llm_providers(name);
CREATE INDEX IF NOT EXISTS idx_llm_requests_session_id ON llm_requests(session_id);
CREATE INDEX IF NOT EXISTS idx_llm_responses_request_id ON llm_responses(request_id);
CREATE INDEX IF NOT EXISTS idx_model_metadata_provider ON model_metadata(provider);
CREATE INDEX IF NOT EXISTS idx_model_metadata_model_type ON model_metadata(model_type);

-- Insert test user
INSERT INTO users (username, email, password_hash, api_key, role)
VALUES (
    'testuser',
    'test@example.com',
    'test_hash',
    'sk-test-api-key',
    'user'
) ON CONFLICT (username) DO NOTHING;

-- Insert test LLM providers
INSERT INTO llm_providers (name, type, api_key, base_url, model, enabled)
VALUES
    ('openai', 'openai', 'test-key', 'https://api.openai.com/v1', 'gpt-4', true),
    ('anthropic', 'anthropic', 'test-key', 'https://api.anthropic.com/v1', 'claude-3-opus', true),
    ('ollama', 'ollama', '', 'http://localhost:11434', 'llama2', true)
ON CONFLICT (name) DO NOTHING;

EOF

    log_success "Database initialized successfully"
}

# Main execution
echo ""
echo "=============================================="
echo "  SuperAgent Integration Test Runner"
echo "=============================================="
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    log_error "Docker is not running. Please start Docker and try again."
    exit 1
fi

# Check if docker compose is available (v2 or v1)
DOCKER_COMPOSE=""
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE="docker compose"
    log_info "Using Docker Compose v2"
elif command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE="docker-compose"
    log_info "Using Docker Compose v1"
else
    log_error "Docker Compose is not installed. Please install it and try again."
    exit 1
fi

# Stop any existing containers from previous runs
log_info "Cleaning up any existing test containers..."
$DOCKER_COMPOSE -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v --remove-orphans 2>/dev/null || true

# Start the test containers
log_info "Starting test containers..."
if [ "$VERBOSE" = true ]; then
    $DOCKER_COMPOSE -f "$COMPOSE_FILE" -p "$PROJECT_NAME" up -d
else
    $DOCKER_COMPOSE -f "$COMPOSE_FILE" -p "$PROJECT_NAME" up -d > /dev/null 2>&1
fi

# Wait for services to be ready
wait_for_postgres
wait_for_redis

# Initialize the database
initialize_database

# Set environment variables for tests
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5433
export TEST_DB_USER=superagent_test
export TEST_DB_PASSWORD=test123
export TEST_DB_NAME=superagent_test_db
export TEST_DB_SSLMODE=disable

export TEST_REDIS_HOST=localhost
export TEST_REDIS_PORT=6380
export TEST_REDIS_PASSWORD=test123

export DB_HOST=localhost
export DB_PORT=5433
export DB_USER=superagent_test
export DB_PASSWORD=test123
export DB_NAME=superagent_test_db
export DB_SSLMODE=disable

export REDIS_HOST=localhost
export REDIS_PORT=6380
export REDIS_PASSWORD=test123

export INTEGRATION_TEST=true

log_info "Running tests for package: $TEST_PACKAGE"
echo ""

# Run the tests
TEST_EXIT_CODE=0
if [ "$COVERAGE" = true ]; then
    log_info "Running tests with coverage..."
    if [ "$VERBOSE" = true ]; then
        go test -v -race -coverprofile=coverage.out -covermode=atomic "$TEST_PACKAGE" || TEST_EXIT_CODE=$?
    else
        go test -race -coverprofile=coverage.out -covermode=atomic "$TEST_PACKAGE" || TEST_EXIT_CODE=$?
    fi

    if [ -f coverage.out ]; then
        log_info "Coverage summary:"
        go tool cover -func=coverage.out | tail -1
        go tool cover -html=coverage.out -o coverage.html
        log_success "Coverage report generated: coverage.html"
    fi
else
    if [ "$VERBOSE" = true ]; then
        go test -v -race "$TEST_PACKAGE" || TEST_EXIT_CODE=$?
    else
        go test -race "$TEST_PACKAGE" || TEST_EXIT_CODE=$?
    fi
fi

echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
    log_success "All tests passed!"
else
    log_error "Some tests failed (exit code: $TEST_EXIT_CODE)"
fi

exit $TEST_EXIT_CODE
