# Agents Package

The `agents` package provides a registry for CLI agents that can be used with HelixAgent. It maintains metadata about 18+ coding assistants and AI tools.

## Supported Agents

| Agent | Description |
|-------|-------------|
| OpenCode | Zen's CLI coding assistant |
| Crush | Codeium's AI coding agent |
| HelixCode | HelixAgent's native CLI |
| Kiro | Amazon's AI coding assistant |
| Aider | AI pair programming in terminal |
| ClaudeCode | Anthropic's Claude CLI |
| Cline | VS Code AI coding extension |
| CodenameGoose | Block's AI coding agent |
| DeepSeekCLI | DeepSeek's command-line tool |
| Forge | Open-source AI coding agent |
| GeminiCLI | Google's Gemini CLI |
| GPTEngineer | AI code generation tool |
| KiloCode | Lightweight AI coding CLI |
| MistralCode | Mistral AI's coding assistant |
| OllamaCode | Local LLM coding assistant |
| Plandex | AI project management tool |
| QwenCode | Alibaba's Qwen CLI |
| AmazonQ | AWS AI assistant |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Agent Registry                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    Agent Metadata                          │  │
│  │                                                            │  │
│  │  - Name         - Command        - Capabilities           │  │
│  │  - Version      - Config Path    - Supported Models       │  │
│  │  - Description  - Environment    - Integration Points     │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Agent Metadata

Each agent entry contains:

```go
type AgentInfo struct {
    Name            string            // Agent identifier
    DisplayName     string            // Human-readable name
    Description     string            // Agent description
    Version         string            // Current version
    Command         string            // CLI command to invoke
    ConfigPath      string            // Default config location
    Environment     map[string]string // Required env vars
    Capabilities    []string          // Supported features
    SupportedModels []string          // Compatible LLM models
}
```

## Usage

### Get Agent Information

```go
import "dev.helix.agent/internal/agents"

// Get registry instance
registry := agents.GetRegistry()

// Get specific agent
agent, exists := registry.Get("ClaudeCode")
if exists {
    fmt.Printf("Command: %s\n", agent.Command)
    fmt.Printf("Config: %s\n", agent.ConfigPath)
}
```

### List All Agents

```go
// Get all registered agents
allAgents := registry.List()
for _, agent := range allAgents {
    fmt.Printf("%s: %s\n", agent.Name, agent.Description)
}
```

### Check Agent Capabilities

```go
// Check if agent supports streaming
agent, _ := registry.Get("ClaudeCode")
for _, cap := range agent.Capabilities {
    if cap == "streaming" {
        fmt.Println("Agent supports streaming")
    }
}
```

### Get Compatible Agents for Model

```go
// Find agents that support a specific model
compatibleAgents := registry.GetByModel("claude-3-opus")
```

## Files

| File | Description |
|------|-------------|
| `registry.go` | Agent registry and metadata |
| `registry_test.go` | Registry tests |

## Integration

The agents registry is used by:
- Provider discovery to detect installed CLI tools
- Configuration to read agent-specific settings
- Debate team selection to understand agent capabilities

## Testing

```bash
go test -v ./internal/agents/...
```

Tests cover:
- Registry initialization
- Agent lookup
- Capability querying
- Model compatibility checks
