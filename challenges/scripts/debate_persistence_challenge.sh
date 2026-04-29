#!/bin/bash
# Debate Persistence Challenge
# Validates database schemas and repositories for debate persistence:
# SQL schema files, repository implementations, CREATE TABLE statements.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-persistence" "Debate Persistence Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Persistence Challenge"
log_info "=============================================="

# --- Section 1: SQL schema files ---

log_info "Test 1: debate_sessions.sql exists"
if [ -f "$PROJECT_ROOT/sql/schema/debate_sessions.sql" ]; then
    record_assertion "sessions_sql" "exists" "true" "debate_sessions.sql exists"
else
    record_assertion "sessions_sql" "exists" "false" "debate_sessions.sql NOT found"
fi

log_info "Test 2: debate_turns.sql exists"
if [ -f "$PROJECT_ROOT/sql/schema/debate_turns.sql" ]; then
    record_assertion "turns_sql" "exists" "true" "debate_turns.sql exists"
else
    record_assertion "turns_sql" "exists" "false" "debate_turns.sql NOT found"
fi

log_info "Test 3: debate_system.sql exists"
if [ -f "$PROJECT_ROOT/sql/schema/debate_system.sql" ]; then
    record_assertion "system_sql" "exists" "true" "debate_system.sql exists"
else
    record_assertion "system_sql" "exists" "false" "debate_system.sql NOT found"
fi

# --- Section 2: CREATE TABLE statements ---

log_info "Test 4: debate_sessions.sql has CREATE TABLE"
if grep -qi "CREATE TABLE" "$PROJECT_ROOT/sql/schema/debate_sessions.sql" 2>/dev/null; then
    record_assertion "sessions_create_table" "true" "true" "CREATE TABLE found in debate_sessions.sql"
else
    record_assertion "sessions_create_table" "true" "false" "CREATE TABLE NOT found in debate_sessions.sql"
fi

log_info "Test 5: debate_turns.sql has CREATE TABLE"
if grep -qi "CREATE TABLE" "$PROJECT_ROOT/sql/schema/debate_turns.sql" 2>/dev/null; then
    record_assertion "turns_create_table" "true" "true" "CREATE TABLE found in debate_turns.sql"
else
    record_assertion "turns_create_table" "true" "false" "CREATE TABLE NOT found in debate_turns.sql"
fi

log_info "Test 6: debate_system.sql has CREATE TABLE"
if grep -qi "CREATE TABLE" "$PROJECT_ROOT/sql/schema/debate_system.sql" 2>/dev/null; then
    record_assertion "system_create_table" "true" "true" "CREATE TABLE found in debate_system.sql"
else
    record_assertion "system_create_table" "true" "false" "CREATE TABLE NOT found in debate_system.sql"
fi

# --- Section 3: Repository files ---

log_info "Test 7: debate_session_repository.go exists"
if [ -f "$PROJECT_ROOT/internal/database/debate_session_repository.go" ]; then
    record_assertion "session_repo_file" "exists" "true" "debate_session_repository.go exists"
else
    record_assertion "session_repo_file" "exists" "false" "debate_session_repository.go NOT found"
fi

log_info "Test 8: debate_turn_repository.go exists"
if [ -f "$PROJECT_ROOT/internal/database/debate_turn_repository.go" ]; then
    record_assertion "turn_repo_file" "exists" "true" "debate_turn_repository.go exists"
else
    record_assertion "turn_repo_file" "exists" "false" "debate_turn_repository.go NOT found"
fi

log_info "Test 9: debate_log_repository.go exists"
if [ -f "$PROJECT_ROOT/internal/database/debate_log_repository.go" ]; then
    record_assertion "log_repo_file" "exists" "true" "debate_log_repository.go exists"
else
    record_assertion "log_repo_file" "exists" "false" "debate_log_repository.go NOT found"
fi

# --- Section 4: Repository types have methods ---

log_info "Test 10: DebateSessionRepository type exists"
if grep -q "type DebateSessionRepository struct\|type DebateSessionRepository interface" "$PROJECT_ROOT/internal/database/debate_session_repository.go" 2>/dev/null; then
    record_assertion "session_repo_type" "true" "true" "DebateSessionRepository type found"
else
    record_assertion "session_repo_type" "true" "false" "DebateSessionRepository type NOT found"
fi

log_info "Test 11: DebateTurnRepository type exists"
if grep -q "type DebateTurnRepository struct\|type DebateTurnRepository interface" "$PROJECT_ROOT/internal/database/debate_turn_repository.go" 2>/dev/null; then
    record_assertion "turn_repo_type" "true" "true" "DebateTurnRepository type found"
else
    record_assertion "turn_repo_type" "true" "false" "DebateTurnRepository type NOT found"
fi

log_info "Test 12: DebateLogRepository type exists"
if grep -q "type DebateLogRepository struct\|type DebateLogRepository interface" "$PROJECT_ROOT/internal/database/debate_log_repository.go" 2>/dev/null; then
    record_assertion "log_repo_type" "true" "true" "DebateLogRepository type found"
else
    record_assertion "log_repo_type" "true" "false" "DebateLogRepository type NOT found"
fi

# --- Section 5: Database package compiles ---

log_info "Test 13: Database package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/database/... 2>&1); then
    record_assertion "database_compile" "true" "true" "Database package compiles"
else
    record_assertion "database_compile" "true" "false" "Database package failed to compile"
fi

# --- Finalize ---

if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
