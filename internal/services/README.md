# Services Package

The `services` package contains core business logic for HelixAgent, including provider management, AI debate orchestration, protocol handling, and various support services.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Provider Registry                            │
│  (Unified provider management with credential handling)          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────┐    ┌────────────────────────────────┐ │
│  │    Debate System     │    │     Protocol Services          │ │
│  │                      │    │                                │ │
│  │  ┌────────────────┐  │    │  ┌───────────┐ ┌───────────┐  │ │
│  │  │ Debate Service │  │    │  │MCP Client │ │LSP Manager│  │ │
│  │  └────────────────┘  │    │  └───────────┘ └───────────┘  │ │
│  │  ┌────────────────┐  │    │  ┌───────────┐ ┌───────────┐  │ │
│  │  │ Team Config    │  │    │  │ACP Client │ │Unified PM │  │ │
│  │  └────────────────┘  │    │  └───────────┘ └───────────┘  │ │
│  │  ┌────────────────┐  │    │                                │ │
│  │  │ Multi-Pass     │  │    └────────────────────────────────┘ │
│  │  │ Validation     │  │                                       │
│  │  └────────────────┘  │    ┌────────────────────────────────┐ │
│  │  ┌────────────────┐  │    │     Support Services           │ │
│  │  │ Intent         │  │    │                                │ │
│  │  │ Classifier     │  │    │  Plugin System │ Cache Factory │ │
│  │  └────────────────┘  │    │  Embedding Mgr │ Memory Service│ │
│  └──────────────────────┘    │  Context Mgr   │ Tool Registry │ │
│                              └────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Core Services

### Provider Registry (`provider_registry.go`)

Unified interface for managing LLM providers:

```go
registry := services.NewProviderRegistry(config, logger)

// Register a provider
registry.RegisterProvider("claude", claudeProvider)

// Get available providers
providers := registry.GetAvailableProviders()

// Get provider by name
provider, err := registry.GetProvider("claude")
```

### Ensemble Service (`ensemble.go`)

Orchestrates multi-provider responses:

```go
ensemble := services.NewEnsembleService(registry, config)

// Run ensemble with voting strategy
response, err := ensemble.Execute(ctx, request, services.VotingStrategyConfidence)
```

### Context Manager (`context_manager.go`)

Manages conversation context and history:

```go
manager := services.NewContextManager(config)

// Add message to context
manager.AddMessage(sessionID, message)

// Get context for request
context := manager.GetContext(sessionID, maxTokens)
```

## AI Debate System

### Debate Service (`debate_service.go`)

Multi-round debate between LLM providers:

```go
debateService := services.NewDebateService(registry, config, logger)

debate, err := debateService.StartDebate(ctx, &services.DebateRequest{
    Topic:        "Should AI have consciousness?",
    Participants: []string{"claude", "gemini", "deepseek"},
    MaxRounds:    3,
})
```

### Debate Team Config (`debate_team_config.go`)

Dynamic team selection based on LLMsVerifier scores:

```go
config := services.NewDebateTeamConfig(verifier, logger)

// Select best providers for debate
team := config.SelectTeam(ctx, 5) // 5 positions
```

### Multi-Pass Validation (`debate_multipass_validation.go`)

Validates and polishes debate responses:

```go
validator := services.NewMultiPassValidator(config)

result, err := validator.Validate(ctx, &services.ValidationInput{
    InitialResponse: response,
    EnablePolish:    true,
    MaxRounds:       3,
})
```

### Intent Classifier (`llm_intent_classifier.go`, `intent_classifier.go`)

Semantic intent detection using LLM:

```go
classifier := services.NewLLMIntentClassifier(provider, logger)

intent, err := classifier.Classify(ctx, "Yes, let's do all points!")
// Returns: confirmation, refusal, question, request, clarification, unclear
```

## Protocol Services

### MCP Client (`mcp_client.go`)

Model Context Protocol integration:

```go
client := services.NewMCPClient(config)
err := client.Connect(ctx, serverURL)

// Execute MCP tool
result, err := client.ExecuteTool(ctx, "read_file", params)
```

### LSP Manager (`lsp_manager.go`)

Language Server Protocol support:

```go
manager := services.NewLSPManager(config, logger)

// Start language server
err := manager.StartServer(ctx, "gopls")

// Get completions
completions, err := manager.GetCompletions(ctx, document, position)
```

### ACP Client (`acp_client.go`)

Agent Communication Protocol:

```go
client := services.NewACPClient(config)

// Send message to agent
response, err := client.SendMessage(ctx, agentID, message)
```

## Support Services

### Plugin System (`plugin_system.go`)

Hot-reloadable plugin management:

```go
pluginSystem := services.NewPluginSystem(config, logger)

// Load plugin
err := pluginSystem.LoadPlugin(ctx, "analyzer")

// Execute plugin
result, err := pluginSystem.Execute(ctx, "analyzer", params)
```

### Embedding Manager (`embedding_manager.go`)

Vector embedding generation:

```go
manager := services.NewEmbeddingManager(providers, config)

// Generate embeddings
embeddings, err := manager.Embed(ctx, []string{"text to embed"})
```

### Memory Service (`memory_service.go`)

Conversation memory and retrieval:

```go
memory := services.NewMemoryService(embeddingManager, vectorStore, config)

// Store memory
err := memory.Store(ctx, sessionID, content)

// Retrieve relevant memories
memories, err := memory.Retrieve(ctx, sessionID, query, limit)
```

### Cognee Services

RAG and knowledge graph integration:

- `cognee_service.go` - Core Cognee API integration
- `cognee_enhanced_provider.go` - LLM provider with RAG augmentation

## Monitoring Services

| Service | Description |
|---------|-------------|
| `circuit_breaker_monitor.go` | Tracks circuit breaker states |
| `oauth_token_monitor.go` | Monitors OAuth token expiration |
| `provider_health_monitor.go` | Provider health tracking |
| `fallback_chain_validator.go` | Validates fallback provider chains |

## Files

| File | Description |
|------|-------------|
| `provider_registry.go` | Unified provider management |
| `ensemble.go` | Multi-provider ensemble execution |
| `context_manager.go` | Conversation context handling |
| `debate_service.go` | AI debate orchestration |
| `debate_team_config.go` | Dynamic team selection |
| `debate_multipass_validation.go` | Response validation |
| `intent_classifier.go` | Pattern-based intent detection |
| `llm_intent_classifier.go` | LLM-based intent detection |
| `mcp_client.go` | MCP protocol client |
| `lsp_manager.go` | LSP protocol manager |
| `acp_client.go` | ACP protocol client |
| `plugin_system.go` | Plugin management |
| `embedding_manager.go` | Vector embeddings |
| `memory_service.go` | Conversation memory |
| `cognee_service.go` | Cognee integration |

## Testing

```bash
go test -v ./internal/services/...
```

Tests cover:
- Provider registration and retrieval
- Ensemble voting strategies
- Debate lifecycle and multi-pass validation
- Intent classification accuracy
- Protocol client operations
- Plugin loading and execution
- Memory storage and retrieval
