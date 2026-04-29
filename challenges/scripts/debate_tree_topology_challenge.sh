#!/bin/bash
# Debate Tree Topology Challenge
# Validates the hierarchical tree topology for debate agents:
# TreeTopology type, TreeNode, factory integration, and tests.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

export GOMAXPROCS=2

init_challenge "debate-tree-topology" "Debate Tree Topology Challenge"
load_env

PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

log_info "=============================================="
log_info "Debate Tree Topology Challenge"
log_info "=============================================="

# --- Section 1: File existence and compilation ---

log_info "Test 1: tree.go exists"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/topology/tree.go" ]; then
    record_assertion "tree_file" "exists" "true" "tree.go exists"
else
    record_assertion "tree_file" "exists" "false" "tree.go NOT found"
fi

log_info "Test 2: Topology package compiles"
# DebateOrchestrator is an extracted Go submodule (its own go.mod); must
# cd into it to run go build / go test, can't reach across modules from
# the parent directory.
if (cd "$PROJECT_ROOT/DebateOrchestrator" && go build ./topology/... 2>&1); then
    record_assertion "topology_compile" "true" "true" "Topology package compiles"
else
    record_assertion "topology_compile" "true" "false" "Topology package failed to compile"
fi

# --- Section 2: Core types and methods ---

log_info "Test 3: TreeTopology type exists"
if grep -q "type TreeTopology struct" "$PROJECT_ROOT/DebateOrchestrator/topology/tree.go" 2>/dev/null; then
    record_assertion "tree_topology_type" "true" "true" "TreeTopology type found"
else
    record_assertion "tree_topology_type" "true" "false" "TreeTopology type NOT found"
fi

log_info "Test 4: TreeNode type exists"
if grep -q "type TreeNode struct" "$PROJECT_ROOT/DebateOrchestrator/topology/tree.go" 2>/dev/null; then
    record_assertion "tree_node_type" "true" "true" "TreeNode type found"
else
    record_assertion "tree_node_type" "true" "false" "TreeNode type NOT found"
fi

log_info "Test 5: NewTreeTopology constructor exists"
if grep -q "func NewTreeTopology" "$PROJECT_ROOT/DebateOrchestrator/topology/tree.go" 2>/dev/null; then
    record_assertion "tree_constructor" "true" "true" "NewTreeTopology constructor found"
else
    record_assertion "tree_constructor" "true" "false" "NewTreeTopology constructor NOT found"
fi

log_info "Test 6: GetNextPhase method exists on TreeTopology"
if grep -q "func (t \*TreeTopology) GetNextPhase" "$PROJECT_ROOT/DebateOrchestrator/topology/tree.go" 2>/dev/null; then
    record_assertion "tree_get_next_phase" "true" "true" "GetNextPhase method found"
else
    # May inherit from BaseTopology
    if grep -q "BaseTopology" "$PROJECT_ROOT/DebateOrchestrator/topology/tree.go" 2>/dev/null; then
        record_assertion "tree_get_next_phase" "true" "true" "Inherits GetNextPhase from BaseTopology"
    else
        record_assertion "tree_get_next_phase" "true" "false" "GetNextPhase NOT found"
    fi
fi

# --- Section 3: Factory integration ---

log_info "Test 7: Factory creates tree topology"
if grep -q "TopologyTree" "$PROJECT_ROOT/DebateOrchestrator/topology/factory.go" 2>/dev/null; then
    record_assertion "factory_tree" "true" "true" "Factory supports TopologyTree"
else
    record_assertion "factory_tree" "true" "false" "Factory does NOT support TopologyTree"
fi

log_info "Test 8: Factory calls NewTreeTopology"
if grep -q "NewTreeTopology" "$PROJECT_ROOT/DebateOrchestrator/topology/factory.go" 2>/dev/null; then
    record_assertion "factory_new_tree" "true" "true" "Factory calls NewTreeTopology"
else
    record_assertion "factory_new_tree" "true" "false" "Factory does NOT call NewTreeTopology"
fi

# --- Section 4: Tests ---

log_info "Test 9: tree_test.go exists"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/topology/tree_test.go" ]; then
    record_assertion "tree_test_file" "exists" "true" "Test file found"
else
    record_assertion "tree_test_file" "exists" "false" "Test file NOT found"
fi

log_info "Test 10: Tree topology tests pass"
if (cd "$PROJECT_ROOT/DebateOrchestrator" && nice -n 19 ionice -c 3 go test -short -count=1 -p 1 -timeout 120s ./topology/ -run "TestTree|TestNewTree" 2>&1 | tail -5 | grep -q "^ok\|PASS"); then
    record_assertion "tree_tests_pass" "pass" "true" "Tree topology tests passed"
else
    record_assertion "tree_tests_pass" "pass" "false" "Tree topology tests failed"
fi

# --- Finalize ---

if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
    finalize_challenge "PASSED"
else
    finalize_challenge "FAILED"
fi
