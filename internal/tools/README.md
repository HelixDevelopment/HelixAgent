# Tools Package

The `tools` package provides a schema registry and handler system for 21 tools that LLMs can use during conversations. All tool parameters use **snake_case** naming convention.

## Supported Tools

| Tool | Description | Required Parameters |
|------|-------------|---------------------|
| `Bash` | Execute shell commands | `command`, `description` |
| `Read` | Read file contents | `file_path` |
| `Write` | Write file contents | `file_path`, `content` |
| `Edit` | Edit file with replacements | `file_path`, `old_string`, `new_string` |
| `Glob` | Find files by pattern | `pattern` |
| `Grep` | Search file contents | `pattern` |
| `WebFetch` | Fetch URL content | `url`, `prompt` |
| `WebSearch` | Search the web | `query` |
| `Git` | Git operations | `operation`, `description` |
| `Task` | Background task execution | `command` |
| `TodoWrite` | Task list management | `todos` |
| `AskUserQuestion` | User interaction | `questions` |
| `EnterPlanMode` | Planning mode | - |
| `ExitPlanMode` | Exit planning | - |
| `Skill` | Invoke skills | `skill` |
| `NotebookEdit` | Jupyter notebook editing | `notebook_path`, `new_source` |
| `KillShell` | Terminate shell | `shell_id` |
| `TaskOutput` | Get task output | `task_id` |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Tool Schema Registry                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Schema Definitions                    │    │
│  │  (JSON Schema for each tool's parameters)               │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Tool Handler                          │    │
│  │  (Validation + Execution)                               │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Schema Registry (`schema.go`)

Defines JSON schemas for all tools:

```go
registry := tools.NewSchemaRegistry()

// Get tool schema
schema, exists := registry.GetSchema("Bash")
if exists {
    // schema contains JSON schema definition
}

// List all tools
allTools := registry.List()

// Validate tool parameters
valid, errors := registry.Validate("Bash", params)
```

### Schema Structure

```go
type ToolSchema struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]ParamSchema `json:"parameters"`
    Required    []string               `json:"required"`
}

type ParamSchema struct {
    Type        string      `json:"type"`
    Description string      `json:"description"`
    Default     interface{} `json:"default,omitempty"`
    Enum        []string    `json:"enum,omitempty"`
}
```

## Tool Handler (`handler.go`)

Executes tool calls:

```go
handler := tools.NewHandler(config, services, logger)

// Execute a tool
result, err := handler.Execute(ctx, &tools.ToolCall{
    Name: "Bash",
    Parameters: map[string]interface{}{
        "command":     "ls -la",
        "description": "List files in current directory",
    },
})

// Result contains tool output
fmt.Println(result.Output)
```

### Tool Call Structure

```go
type ToolCall struct {
    Name       string                 `json:"name"`
    Parameters map[string]interface{} `json:"parameters"`
}

type ToolResult struct {
    Name    string      `json:"name"`
    Output  interface{} `json:"output"`
    Error   string      `json:"error,omitempty"`
    Success bool        `json:"success"`
}
```

## Parameter Naming Convention

**All parameters use snake_case:**

```go
// Correct
{"file_path": "/path/to/file", "old_string": "foo", "new_string": "bar"}

// Incorrect (will fail validation)
{"filePath": "/path/to/file", "oldString": "foo", "newString": "bar"}
```

## Usage Examples

### Bash Tool

```go
result, err := handler.Execute(ctx, &tools.ToolCall{
    Name: "Bash",
    Parameters: map[string]interface{}{
        "command":     "go test ./...",
        "description": "Run all tests",
        "timeout":     60000,
    },
})
```

### Read Tool

```go
result, err := handler.Execute(ctx, &tools.ToolCall{
    Name: "Read",
    Parameters: map[string]interface{}{
        "file_path": "/path/to/file.go",
        "limit":     100,
        "offset":    0,
    },
})
```

### Edit Tool

```go
result, err := handler.Execute(ctx, &tools.ToolCall{
    Name: "Edit",
    Parameters: map[string]interface{}{
        "file_path":  "/path/to/file.go",
        "old_string": "func oldName()",
        "new_string": "func newName()",
    },
})
```

### WebFetch Tool

```go
result, err := handler.Execute(ctx, &tools.ToolCall{
    Name: "WebFetch",
    Parameters: map[string]interface{}{
        "url":    "https://example.com/api/docs",
        "prompt": "Extract the API endpoints",
    },
})
```

## Files

| File | Description |
|------|-------------|
| `schema.go` | Tool schema definitions |
| `handler.go` | Tool execution handler |
| `schema_test.go` | Schema validation tests |

## Testing

```bash
go test -v ./internal/tools/...
```

Tests cover:
- Schema validation for all tools
- Parameter type checking
- Required field validation
- Handler execution
- Error handling
