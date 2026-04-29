#!/bin/bash
# Debate Git Integration Challenge
# Validates the git tool for debate sessions:
# GitTool type, worktree management, snapshot commits, cleanup.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-git-integration" "Debate Git Integration Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Git Integration Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: git_tool.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/tools/git_tool.go" ]; then
    record_assertion "git_tool_file" "exists" "true" "git_tool.go exists"
else
    record_assertion "git_tool_file" "exists" "false" "git_tool.go NOT found"
fi

log_info "Test 2: Tools package compiles"
if (cd "$PROJECT_ROOT" && go build ./internal/debate/tools/... 2>&1); then
    record_assertion "tools_compile" "true" "true" "Tools package compiles"
else
    record_assertion "tools_compile" "true" "false" "Tools package failed to compile"
fi

# --- Section 2: Core types and methods ---

log_info "Test 3: GitTool type exists"
if grep -q "type GitTool struct" "$PROJECT_ROOT/internal/debate/tools/git_tool.go" 2>/dev/null; then
    record_assertion "git_tool_type" "true" "true" "GitTool type found"
else
    record_assertion "git_tool_type" "true" "false" "GitTool type NOT found"
fi

log_info "Test 4: GitToolConfig type exists"
if grep -q "type GitToolConfig struct" "$PROJECT_ROOT/internal/debate/tools/git_tool.go" 2>/dev/null; then
    record_assertion "git_tool_config" "true" "true" "GitToolConfig type found"
else
    record_assertion "git_tool_config" "true" "false" "GitToolConfig type NOT found"
fi

log_info "Test 5: NewGitTool constructor exists"
if grep -q "func NewGitTool" "$PROJECT_ROOT/internal/debate/tools/git_tool.go" 2>/dev/null; then
    record_assertion "git_tool_constructor" "true" "true" "NewGitTool constructor found"
else
    record_assertion "git_tool_constructor" "true" "false" "NewGitTool constructor NOT found"
fi

log_info "Test 6: CreateWorktree method exists"
if grep -q "func (g \*GitTool) CreateWorktree" "$PROJECT_ROOT/internal/debate/tools/git_tool.go" 2>/dev/null; then
    record_assertion "create_worktree_method" "true" "true" "CreateWorktree method found"
else
    record_assertion "create_worktree_method" "true" "false" "CreateWorktree method NOT found"
fi

log_info "Test 7: CommitSnapshot method exists"
if grep -q "func (g \*GitTool) CommitSnapshot" "$PROJECT_ROOT/internal/debate/tools/git_tool.go" 2>/dev/null; then
    record_assertion "commit_snapshot_method" "true" "true" "CommitSnapshot method found"
else
    record_assertion "commit_snapshot_method" "true" "false" "CommitSnapshot method NOT found"
fi

log_info "Test 8: Cleanup method exists"
if grep -q "func (g \*GitTool) Cleanup" "$PROJECT_ROOT/internal/debate/tools/git_tool.go" 2>/dev/null; then
    record_assertion "cleanup_method" "true" "true" "Cleanup method found"
else
    record_assertion "cleanup_method" "true" "false" "Cleanup method NOT found"
fi

log_info "Test 9: WorktreeInfo type exists"
if grep -q "type WorktreeInfo struct" "$PROJECT_ROOT/internal/debate/tools/git_tool.go" 2>/dev/null; then
    record_assertion "worktree_info_type" "true" "true" "WorktreeInfo type found"
else
    record_assertion "worktree_info_type" "true" "false" "WorktreeInfo type NOT found"
fi

# --- Section 3: Tests ---

log_info "Test 10: git_tool_test.go exists"
if [ -f "$PROJECT_ROOT/internal/debate/tools/git_tool_test.go" ]; then
    record_assertion "git_tool_test_file" "exists" "true" "Test file found"
else
    record_assertion "git_tool_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 11: Git tool tests pass (if git available)"
if command -v git &>/dev/null; then
    if (cd "$PROJECT_ROOT" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./internal/debate/tools/ -run "TestGit|TestNewGit|TestWorktree|TestSnapshot" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
        record_assertion "git_tool_tests_pass" "pass" "true" "Git tool tests passed"
    else
        record_assertion "git_tool_tests_pass" "pass" "false" "Git tool tests failed"
    fi
else
    record_assertion "git_tool_tests_pass" "pass" "true" "Git not available, skipped"
fi

# --- Finalize ---

FAILURES=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)
if [ "$FAILURES" -eq 0 ]; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
