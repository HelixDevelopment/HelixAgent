# Module 15: HelixLLM and AgenticEnsemble

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 15: HelixLLM and AgenticEnsemble
- Duration: 90 minutes
- Unified AI Ensemble with Dual-Mode Operation

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Configure HelixLLM as a first-class LLM provider
- Understand AgenticEnsemble dual-mode architecture
- Implement tool-augmented reasoning (Reason mode)
- Configure autonomous task execution (Execute mode)
- Integrate HelixLLM's RAG capabilities with HelixAgent

---

## Slide 3: What is HelixLLM?

**A Local AI Ensemble Provider:**

- OpenAI-compatible API served locally
- RAG-augmented completions
- Embedding generation
- Analytics and event tracking
- Hot-reloading configuration via file watcher
- First-class provider in HelixAgent's provider registry

*Think of it as a self-hosted LLM gateway with built-in RAG*

---

## Slide 4: HelixLLM Architecture

**Submodule Structure:**

```
HelixLLM/
  +-- internal/
  |     +-- agents/       # Agent API layer
  |     +-- shared/
  |           +-- config/    # Hot-reloading config
  |           +-- analytics/ # Usage analytics
  |           +-- events/    # Event bus
  |           +-- health/    # Health caching
  +-- pkg/
        +-- api/         # OpenAI-compatible API
        +-- types/       # Shared types
```

---

## Slide 5: HelixLLM Provider Configuration

**Setting Up HelixLLM in HelixAgent:**

```bash
# Environment variables
HELIX_LLM_ENDPOINT=https://localhost:8443
HELIX_LLM_API_KEY=your-key
HELIX_LLM_MODEL=helixllm-default
HELIX_LLM_TLS_SKIP_VERIFY=false  # Set true for self-signed certs
```

```go
// Provider implementation
type Provider struct {
    endpoint      string
    apiKey        string
    model         string
    timeout       time.Duration
    tlsSkipVerify bool
    httpClient    *http.Client
}

// Endpoints served:
// /v1/chat/completions  - Chat completions
// /v1/embeddings        - Embedding generation
// /v1/models            - Model listing
// /internal/health      - Health check
```

---

## Slide 6: HelixLLM Adapter

**Bridge Layer:**

```
internal/adapters/helixllm/
  +-- adapter.go    # HelixAgent <-> HelixLLM bridge
  +-- types.go      # Type conversions
```

- Converts HelixAgent request types to HelixLLM format
- Maps HelixLLM responses back to HelixAgent models
- Handles TLS configuration for secure local communication
- Lazy initialization via `sync.Once`

---

## Slide 7: What is AgenticEnsemble?

**Dual-Mode Unified LLM:**

```
                Request
                  |
          +-------v-------+
          | Intent Classify|
          +---+-------+---+
              |       |
     +--------v-+   +-v--------+
     | REASON   |   | EXECUTE  |
     | MODE     |   | MODE     |
     +----+-----+   +----+-----+
          |              |
    +-----v-----+  +----v------+
    | Debate +  |  | Decompose |
    | Tool Loop |  | Dispatch  |
    +-----------+  | Verify    |
          |        +-----------+
          |              |
          +------+-------+
                 |
           Final Response
```

---

## Slide 8: Reason Mode

**Debate Service + Iterative Tool Resolution:**

```go
// In Reason mode, AgenticEnsemble:
// 1. Routes to debate service
// 2. Augments with iterative tool resolution
// 3. Tools are called in a loop until resolved
// 4. Final response synthesized from debate + tool results

type IterativeToolExecutor struct {
    // Executes tool calls in multiple rounds
    // until all tool references are resolved
}
```

**Best for:** Complex reasoning tasks that may require tool access (file reading, API calls, calculations)

---

## Slide 9: Execute Mode

**Task Decomposition + Agent Worker Pool:**

```go
// In Execute mode, AgenticEnsemble:
// 1. Decomposes request into subtasks via ExecutionPlanner
// 2. Dispatches subtasks to agent worker pool
// 3. Collects and verifies results via VerificationDebate
// 4. Synthesizes final response from verified results

type ExecutionPlanner struct {
    // Breaks complex tasks into executable steps
}

type VerificationDebate struct {
    // Multi-LLM verification of execution results
}
```

**Best for:** Multi-step tasks requiring autonomous execution

---

## Slide 10: AgenticEnsemble Configuration

**Creating an AgenticEnsemble:**

```go
ensemble := NewAgenticEnsemble(
    debateService,      // AI debate for reasoning
    intentClassifier,   // Automatic mode selection
    toolExecutor,       // Iterative tool resolution
    planner,            // Task decomposition
    verifier,           // Result verification
    providerRegistry,   // Access to all 43 providers
    config,             // AgenticEnsembleConfig
    logger,
)

// Nil dependencies are tolerated - graceful degradation
// If toolExecutor is nil, Reason mode skips tool resolution
// If planner is nil, Execute mode falls back to Reason mode
```

---

## Slide 11: Intent Classification

**Automatic Mode Selection:**

- LLM-based classification determines Reason vs Execute mode
- Pattern-based fallback when LLM classification is unavailable
- Zero hardcoding: classification prompts drive routing
- Override via API parameter for explicit control

---

## Slide 12: AgenticEnsemble API

**Using AgenticEnsemble via HelixAgent:**

```bash
# Standard chat completion (auto-routes through ensemble)
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [
      {"role": "user", "content": "Analyze this codebase and suggest improvements"}
    ]
  }'

# The ensemble automatically:
# 1. Classifies intent
# 2. Selects Reason or Execute mode
# 3. Orchestrates the appropriate pipeline
# 4. Returns verified, high-quality response
```

---

## Slide 13: Hands-On Lab

**Lab Exercise 15.1: HelixLLM and AgenticEnsemble**

Tasks:
1. Configure HelixLLM as a provider
2. Verify health check and model listing
3. Submit a reasoning task (Reason mode)
4. Submit a multi-step task (Execute mode)
5. Compare output quality between modes

Time: 45 minutes

---

## Slide 14: Module Summary

**Key Takeaways:**

- HelixLLM is a local AI ensemble provider with RAG capabilities
- AgenticEnsemble provides dual-mode operation: Reason + Execute
- Reason mode: debate + iterative tool resolution
- Execute mode: decompose + dispatch + verify
- Intent classification auto-selects the appropriate mode
- Graceful degradation when subsystems are unavailable
- Integrates seamlessly with all 43 providers

**Next: Module 16 - HTTP/3 (QUIC) and Brotli Compression**

---

## Speaker Notes

### Slide 7 Notes
The dual-mode architecture is the key innovation. Emphasize that most requests will use
Reason mode (augmented debate). Execute mode activates for tasks that explicitly require
multi-step autonomous execution.

### Slide 10 Notes
Highlight the graceful degradation pattern. This is a production-critical design decision:
if any dependency is unavailable, the system falls back rather than failing.

### Slide 12 Notes
Demo this live. Show how the same endpoint handles both simple questions (Reason mode)
and complex multi-step tasks (Execute mode) transparently.
