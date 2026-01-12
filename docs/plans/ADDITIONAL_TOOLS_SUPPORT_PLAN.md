# Additional Tools Support Plan for HelixAgent

## Executive Summary

This plan outlines the implementation strategy for expanding HelixAgent's tool support beyond the current 9 tools to enable comprehensive CLI agent integration with OpenCode, Crush, HelixCode, and Kilo Code.

## Current State

### Currently Supported Tools (9)

| Tool | Required Fields | Description |
|------|-----------------|-------------|
| **Bash/shell** | `command`, `description` | Execute shell commands |
| **Read** | `file_path` | Read file contents |
| **Write** | `file_path`, `content` | Create/write files |
| **Edit** | `file_path`, `old_string`, `new_string` | Modify file content |
| **Glob** | `pattern` | File pattern matching |
| **Grep** | `pattern` | Content search |
| **WebFetch** | `url`, `prompt` | Fetch web content |
| **WebSearch** | `query` | Search the web |
| **Task** | `prompt`, `description`, `subagent_type` | Delegate to subagents |

### Current Architecture

Tool calls flow through two main functions:
1. `generateActionToolCalls()` - Pattern matching on user intent
2. `extractActionsFromSynthesis()` - Parse debate consensus for actions

---

## Proposed New Tools (12 Additional)

### Phase 1: Core Development Tools (Priority: Critical)

#### 1. Git Tool
**Purpose**: Version control operations without raw bash exposure

```go
// Required Fields
type GitToolArgs struct {
    Operation   string   `json:"operation"`   // clone, commit, push, pull, branch, merge, checkout, status, diff, log
    Arguments   []string `json:"arguments"`   // Additional arguments
    Description string   `json:"description"` // Human-readable description
}

// Example usage patterns
{"operation": "commit", "arguments": ["-m", "Fix bug #123"], "description": "Commit bug fix"}
{"operation": "push", "arguments": ["origin", "main"], "description": "Push to remote main branch"}
{"operation": "branch", "arguments": ["feature/new-api"], "description": "Create new feature branch"}
```

**Implementation Location**: `internal/handlers/openai_compatible.go`
- Add to `extractToolArguments()`
- Add pattern matching in `generateActionToolCalls()` for git-related queries
- Keywords: "commit", "push", "pull", "branch", "merge", "checkout", "git status", "diff"

#### 2. Test Tool
**Purpose**: Intelligent test execution with coverage and reporting

```go
type TestToolArgs struct {
    TestPath    string   `json:"test_path"`    // Path or pattern for tests
    TestType    string   `json:"test_type"`    // unit, integration, e2e, benchmark
    Options     []string `json:"options"`      // Additional flags
    Coverage    bool     `json:"coverage"`     // Generate coverage report
    Description string   `json:"description"`  // Human-readable description
}

// Example usage patterns
{"test_path": "./...", "test_type": "unit", "coverage": true, "description": "Run all unit tests with coverage"}
{"test_path": "./tests/integration/...", "test_type": "integration", "description": "Run integration tests"}
```

#### 3. Lint Tool
**Purpose**: Code quality analysis with auto-fix capability

```go
type LintToolArgs struct {
    Path        string `json:"path"`         // File or directory path
    Linter      string `json:"linter"`       // golangci-lint, eslint, pylint, etc.
    AutoFix     bool   `json:"auto_fix"`     // Automatically fix issues
    Config      string `json:"config"`       // Custom config file path
    Description string `json:"description"`  // Human-readable description
}
```

### Phase 2: File Intelligence Tools (Priority: High)

#### 4. Diff Tool
**Purpose**: Show changes between versions/files

```go
type DiffToolArgs struct {
    FilePath    string `json:"file_path"`     // File to diff
    Mode        string `json:"mode"`          // git, staged, file-compare
    CompareWith string `json:"compare_with"`  // Revision, branch, or file
    Context     int    `json:"context"`       // Lines of context
    Description string `json:"description"`
}
```

#### 5. TreeView Tool
**Purpose**: Directory structure visualization

```go
type TreeViewToolArgs struct {
    Path        string   `json:"path"`         // Root directory
    MaxDepth    int      `json:"max_depth"`    // How deep to traverse
    IgnorePatterns []string `json:"ignore_patterns"` // Patterns to ignore
    ShowHidden  bool     `json:"show_hidden"`  // Include hidden files
    Description string   `json:"description"`
}
```

#### 6. FileInfo Tool
**Purpose**: Get detailed file metadata

```go
type FileInfoToolArgs struct {
    FilePath    string `json:"file_path"`
    IncludeStats bool  `json:"include_stats"` // Size, lines, functions
    IncludeGit   bool  `json:"include_git"`   // Git history info
    Description  string `json:"description"`
}
```

### Phase 3: Code Intelligence Tools (Priority: Medium)

#### 7. Symbols Tool
**Purpose**: Extract code symbols (functions, classes, types)

```go
type SymbolsToolArgs struct {
    FilePath    string   `json:"file_path"`    // File to analyze
    SymbolTypes []string `json:"symbol_types"` // function, class, type, const, var
    Recursive   bool     `json:"recursive"`    // Search subdirectories
    Description string   `json:"description"`
}
```

#### 8. References Tool
**Purpose**: Find all references to a symbol

```go
type ReferencesToolArgs struct {
    Symbol      string `json:"symbol"`        // Symbol name to find
    FilePath    string `json:"file_path"`     // Starting file (optional)
    IncludeDecl bool   `json:"include_decl"`  // Include declaration
    Description string `json:"description"`
}
```

#### 9. Definition Tool
**Purpose**: Go to definition of a symbol

```go
type DefinitionToolArgs struct {
    Symbol      string `json:"symbol"`        // Symbol name
    FilePath    string `json:"file_path"`     // Context file
    Line        int    `json:"line"`          // Context line number
    Description string `json:"description"`
}
```

### Phase 4: Workflow Tools (Priority: Medium)

#### 10. PR Tool
**Purpose**: Pull request management

```go
type PRToolArgs struct {
    Action      string `json:"action"`        // create, list, view, approve, merge
    Title       string `json:"title"`         // PR title (for create)
    Body        string `json:"body"`          // PR description
    BaseBranch  string `json:"base_branch"`   // Target branch
    PRNumber    int    `json:"pr_number"`     // For view/approve/merge
    Description string `json:"description"`
}
```

#### 11. Issue Tool
**Purpose**: Issue tracking integration

```go
type IssueToolArgs struct {
    Action      string   `json:"action"`       // create, list, view, close, comment
    Title       string   `json:"title"`        // Issue title
    Body        string   `json:"body"`         // Issue body
    Labels      []string `json:"labels"`       // Issue labels
    IssueNumber int      `json:"issue_number"` // For view/close/comment
    Description string   `json:"description"`
}
```

#### 12. Workflow Tool
**Purpose**: CI/CD workflow management

```go
type WorkflowToolArgs struct {
    Action      string `json:"action"`        // run, list, view, cancel
    WorkflowID  string `json:"workflow_id"`   // Workflow identifier
    Branch      string `json:"branch"`        // Target branch
    RunID       int    `json:"run_id"`        // For view/cancel
    Description string `json:"description"`
}
```

---

## Implementation Plan

### Step 1: Tool Schema Registry

Create a centralized tool schema registry:

```go
// internal/tools/schema.go

type ToolSchema struct {
    Name           string   `json:"name"`
    Description    string   `json:"description"`
    RequiredFields []string `json:"required_fields"`
    OptionalFields []string `json:"optional_fields"`
    Aliases        []string `json:"aliases"`
}

var ToolSchemaRegistry = map[string]*ToolSchema{
    "Bash": {
        Name:           "Bash",
        Description:    "Execute shell commands",
        RequiredFields: []string{"command", "description"},
        Aliases:        []string{"bash", "shell", "Shell"},
    },
    "Git": {
        Name:           "Git",
        Description:    "Version control operations",
        RequiredFields: []string{"operation", "description"},
        OptionalFields: []string{"arguments"},
        Aliases:        []string{"git"},
    },
    // ... more tools
}
```

### Step 2: Tool Handler Interface

```go
// internal/tools/handler.go

type ToolHandler interface {
    Name() string
    Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
    ValidateArgs(args map[string]interface{}) error
    GenerateArgs(context string) map[string]interface{}
}

type ToolRegistry struct {
    handlers map[string]ToolHandler
    schemas  map[string]*ToolSchema
}
```

### Step 3: Pattern Matching Updates

Update `generateActionToolCalls()` with new patterns:

```go
// Git patterns
if containsAny(topicLower, []string{"commit", "push", "pull", "merge", "branch", "checkout"}) {
    // Generate Git tool call
}

// Test patterns
if containsAny(topicLower, []string{"run test", "execute test", "test coverage", "unit test"}) {
    // Generate Test tool call
}

// Lint patterns
if containsAny(topicLower, []string{"lint", "check code", "code quality", "fix style"}) {
    // Generate Lint tool call
}
```

### Step 4: MCP Integration

Expose new tools via MCP protocol:

```go
// internal/handlers/mcp.go

func (h *MCPHandler) MCPTools(c *gin.Context) {
    tools := []map[string]interface{}{}

    for name, schema := range ToolSchemaRegistry {
        tools = append(tools, map[string]interface{}{
            "name":        name,
            "description": schema.Description,
            "inputSchema": generateInputSchema(schema),
        })
    }

    c.JSON(http.StatusOK, gin.H{"tools": tools})
}
```

---

## Files to Modify/Create

### New Files
- `internal/tools/schema.go` - Tool schema definitions
- `internal/tools/handler.go` - Tool handler interface
- `internal/tools/git_tool.go` - Git tool implementation
- `internal/tools/test_tool.go` - Test tool implementation
- `internal/tools/lint_tool.go` - Lint tool implementation
- `internal/tools/diff_tool.go` - Diff tool implementation
- `internal/tools/treeview_tool.go` - TreeView tool implementation
- `internal/tools/fileinfo_tool.go` - FileInfo tool implementation
- `internal/tools/symbols_tool.go` - Symbols tool implementation
- `internal/tools/references_tool.go` - References tool implementation
- `internal/tools/definition_tool.go` - Definition tool implementation
- `internal/tools/pr_tool.go` - PR tool implementation
- `internal/tools/issue_tool.go` - Issue tool implementation
- `internal/tools/workflow_tool.go` - Workflow tool implementation

### Modified Files
- `internal/handlers/openai_compatible.go` - Add pattern matching and extractToolArguments cases
- `internal/handlers/mcp.go` - Expose new tools via MCP
- `internal/handlers/openai_compatible_test.go` - Add tests for new tools
- `tests/integration/tool_call_api_validation_test.go` - Add integration tests
- `challenges/scripts/tool_call_validation_challenge.sh` - Update schema validation

---

## Testing Strategy

### Unit Tests
For each new tool:
1. Schema validation tests
2. Argument extraction tests
3. Pattern matching tests
4. Description generation tests

### Integration Tests
1. API response validation (required fields present)
2. Tool execution tests (mocked)
3. Pattern recognition tests
4. Multi-tool workflow tests

### Challenge Tests
1. Extended `tool_call_validation_challenge.sh`
2. New `additional_tools_challenge.sh`

---

## Rollout Phases

### Phase 1 (Week 1-2)
- Implement Tool Schema Registry
- Implement Git, Test, Lint tools
- Update pattern matching
- Add unit tests

### Phase 2 (Week 3-4)
- Implement Diff, TreeView, FileInfo tools
- Add MCP integration
- Add integration tests

### Phase 3 (Week 5-6)
- Implement Symbols, References, Definition tools
- LSP integration for code intelligence

### Phase 4 (Week 7-8)
- Implement PR, Issue, Workflow tools
- GitHub/GitLab API integration
- End-to-end testing

---

## CLI Agent Compatibility

### OpenCode
- Full tool_calls support via streaming API
- Snake_case parameter naming
- System-reminder filtering

### Crush
- Same streaming compatibility
- Tool confirmation workflow support

### HelixCode
- Local tool execution support
- Extended timeout for long-running tools

### Kilo Code
- Lightweight tool subset support
- Fast response optimization

---

## Success Metrics

1. **All tool calls have required fields** - 100% validation pass rate
2. **Pattern matching accuracy** - >90% correct tool selection
3. **Integration test coverage** - >80% code coverage for new tools
4. **CLI agent compatibility** - Works with OpenCode, Crush, HelixCode, Kilo Code
5. **Challenge pass rate** - All tool validation challenges pass

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing tool calls | Comprehensive regression tests |
| Pattern matching conflicts | Priority-ordered matching |
| Performance degradation | Lazy loading, caching |
| Security vulnerabilities | Input sanitization, command validation |

---

## Conclusion

This plan provides a structured approach to expanding HelixAgent's tool support from 9 to 21 tools, enabling comprehensive CLI agent integration while maintaining backward compatibility and high quality through extensive testing.
