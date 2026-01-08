# HelixAgent Comprehensive Architecture

## Executive Summary

HelixAgent is an AI-powered ensemble LLM service that acts as a **Virtual LLM Provider** - exposing a single unified model that internally leverages multiple top-performing language models through an AI debate mechanism. This document provides complete clarity on the architecture, ensuring no misunderstandings about how the system operates.

---

## Core Architecture Principles

### 1. HelixAgent as a Virtual LLM Provider

**CRITICAL CONCEPT**: HelixAgent is NOT a traditional LLM aggregator. It presents itself as a **single LLM provider** with **ONE virtual model** - the AI Debate Ensemble.

```
┌─────────────────────────────────────────────────────────────────┐
│                     EXTERNAL VIEW                               │
│    HelixAgent appears as ONE provider with ONE model            │
│    Similar to how OpenAI or Anthropic expose their APIs         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    INTERNAL IMPLEMENTATION                       │
│    Multiple LLMs collaborate via AI debate to produce           │
│    consensus-driven, high-quality responses                     │
└─────────────────────────────────────────────────────────────────┘
```

### 2. The Virtual Model - AI Debate Ensemble

The single exposed model (`helixagent-debate` or `helixagent/helixagent-debate`) is backed by:

- **5 Primary LLMs**: Top-scoring verified models from different providers
- **2-3 Fallbacks per Primary**: Backup models for resilience
- **Confidence-Weighted Voting**: Responses are aggregated based on quality scores

```
helixagent/helixagent-debate
├── Primary 1: Gemini Pro (Score: 8.5)
│   ├── Fallback 1.1: Gemini 1.5 Pro
│   └── Fallback 1.2: DeepSeek Coder
├── Primary 2: DeepSeek Chat (Score: 8.1)
│   └── Fallback 2.1: Mistral Large
├── Primary 3: Llama 3 70B (Score: 7.5)
├── Primary 4: Llama 3.1 70B/Cerebras (Score: 7.2)
└── Primary 5: Llama 3 70B/Fireworks (Score: 7.1)
```

---

## LLMsVerifier Integration

### What is LLMsVerifier?

LLMsVerifier is a **verification system** that:
1. Tests LLM providers with real API calls
2. Scores models based on capabilities and performance
3. Provides data for selecting debate group members

### REAL Data - No Samples or Stubs

**CRITICAL REQUIREMENT**: ALL data must come from REAL API verification.

- **NO hardcoded scores**
- **NO sample/demonstration data**
- **NO cached test data**
- **NO stub responses**

Every model score is obtained through actual API calls to the provider.

### Verification Flow

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   API Keys in    │────▶│  LLMsVerifier    │────▶│  Verified JSON   │
│   Environment    │     │  (Real API Calls)│     │  with Real Scores│
└──────────────────┘     └──────────────────┘     └──────────────────┘
                                                           │
                                                           ▼
                         ┌──────────────────────────────────────────────┐
                         │    Dynamic Debate Group Formation            │
                         │    - Top 5 primaries by score                │
                         │    - 2-3 fallbacks per primary               │
                         │    - Provider diversity ensured              │
                         └──────────────────────────────────────────────┘
```

---

## Challenge System

### Overview

The challenge system validates HelixAgent's functionality through 39 comprehensive tests:

| Category | Count | Description |
|----------|-------|-------------|
| Infrastructure | 8 | Health, caching, database, config, plugins, sessions, shutdown |
| Providers | 7 | Claude, DeepSeek, Gemini, Ollama, OpenRouter, Qwen, ZAI |
| Protocols | 3 | MCP, LSP, ACP |
| Security | 3 | Authentication, rate limiting, input validation |
| Core | 7 | Provider verification, ensemble, debate, embeddings, streaming, metadata, quality |
| Cloud | 3 | AWS Bedrock, GCP Vertex, Azure OpenAI |
| Optimization | 2 | Semantic cache, structured output |
| Integration | 1 | Cognee |
| Resilience | 3 | Circuit breaker, error handling, concurrent access |
| API | 2 | OpenAI compatibility, gRPC |

### Auto-Start Infrastructure

**CRITICAL**: ALL infrastructure starts automatically when needed.

```bash
# When running any challenge, HelixAgent auto-starts if not running:
# 1. Detects if HelixAgent is running on port 8080
# 2. If not running, builds binary if needed
# 3. Starts HelixAgent with required environment
# 4. Waits for health check to pass
# 5. Runs challenge tests
# 6. Stops HelixAgent when done
```

### Auto-Start Implementation

```bash
auto_start_helixagent() {
    # Check if already running
    if curl -s "http://localhost:$port/health" > /dev/null 2>&1; then
        return 0
    fi

    # Find or build binary (prefers bin/helixagent)
    if [[ -x "$PROJECT_ROOT/bin/helixagent" ]]; then
        binary="$PROJECT_ROOT/bin/helixagent"
    fi

    # Set JWT_SECRET if not set
    if [[ -z "$JWT_SECRET" ]]; then
        export JWT_SECRET="helixagent-test-secret-key-$(date +%s)"
    fi

    # Start HelixAgent
    PORT=$port GIN_MODE=release JWT_SECRET="$JWT_SECRET" "$binary" &

    # Wait for startup (up to 30 seconds)
    while ! curl -s "http://localhost:$port/health" > /dev/null 2>&1; do
        sleep 1
    done
}
```

### Optional vs Required Tests

Tests are categorized as:

- **Required**: Must pass for challenge to succeed (code existence checks)
- **Optional**: Don't fail challenge if endpoint unavailable (API endpoint tests)

```bash
# Required test - fails challenge if assertion fails
run_api_test "/health" "GET" "" "200" "Health check"

# Optional test - logs skip if endpoint unavailable
run_optional_api_test "/v1/cache/stats" "GET" "" "200" "Cache stats"
```

---

## Container Runtime Support

### Docker and Podman

HelixAgent supports both Docker and Podman container runtimes:

```bash
# Auto-detection
RUNTIME="$(command -v docker 2>/dev/null || command -v podman 2>/dev/null)"

# For Podman rootless mode
systemctl --user enable --now podman.socket
```

### Infrastructure Components

| Service | Purpose | Default Port |
|---------|---------|--------------|
| PostgreSQL | Database | 15432 |
| Redis | Cache | 16379 |
| HelixAgent | Main API | 8080 |
| LLMsVerifier | Verification | 8081 |

---

## OpenCode Configuration Schema

The Main Challenge generates an OpenCode-compatible configuration:

```json
{
  "$schema": "https://opencode.sh/schema.json",
  "username": "HelixAgent AI Ensemble",
  "features": {
    "streaming": true,
    "tool_calling": true,
    "embeddings": true,
    "vision": true,
    "mcp": true,
    "lsp": true,
    "acp": true,
    "debate_mode": true
  },
  "metadata": {
    "generator": "HelixAgent Main Challenge",
    "verification_method": "real_api_verification",
    "debate_group": {
      "primary_members": 5,
      "fallbacks_per_member": 2,
      "strategy": "confidence_weighted",
      "consensus_threshold": 0.7,
      "average_score": 7.75
    }
  },
  "provider": {
    "helixagent": {
      "options": {
        "apiKey": "${HELIXAGENT_API_KEY}",
        "baseURL": "http://localhost:7061/v1"
      },
      "models": {
        "helixagent-debate": {
          "name": "HelixAgent AI Debate Group",
          "maxTokens": 128000,
          "supports_streaming": true,
          "underlying_models": {
            "primary": ["gemini-pro", "deepseek-chat", "..."],
            "fallbacks": ["gemini-1.5-pro", "..."]
          }
        }
      }
    }
  },
  "mcp": {
    "helixagent-tools": {
      "type": "http",
      "url": "http://localhost:7061/v1/mcp"
    },
    "filesystem": { "type": "stdio", "command": ["npx", "..."] },
    "github": { "type": "stdio", "command": ["npx", "..."] },
    "memory": { "type": "stdio", "command": ["npx", "..."] }
  }
}
```

---

## Protocol Support

### MCP (Model Context Protocol)

- **Endpoint**: `/v1/mcp/tools`, `/v1/mcp/execute`
- **Purpose**: Tool calling and execution
- **Configuration**: Via `mcp` section in config

### LSP (Language Server Protocol)

- **Endpoint**: `/v1/lsp/servers`
- **Purpose**: Code intelligence (gopls, typescript, python)
- **Configuration**: Via `lsp` section in config

### ACP (Agent Communication Protocol)

- **Endpoint**: `/v1/acp/servers`
- **Purpose**: Agent-to-agent communication
- **Configuration**: Via `acp` section in config

---

## Main Challenge Workflow

### Phase 1: Infrastructure
- Auto-detect container runtime (Docker/Podman)
- Start PostgreSQL, Redis if needed
- Build HelixAgent binary if needed

### Phase 2: Provider Verification
- Start LLMsVerifier server
- Verify all providers with REAL API calls
- Generate `providers_verified.json`

### Phase 3: Model Benchmarking
- Test each available model
- Score based on capabilities and performance
- Generate `models_scored.json`

### Phase 4: Debate Group Formation
- Select top 5 models as primaries
- Assign 2-3 fallbacks per primary
- Ensure provider diversity
- Generate `debate_group.json`

### Phase 5: System Verification
- Start HelixAgent with debate configuration
- Verify as OpenAI-compatible endpoint
- Generate `system_verification.json`

### Phase 6: OpenCode Configuration
- Generate complete configuration
- Copy to user's Downloads folder
- Generate `opencode.json`

### Phase 7: Final Report
- Generate master summary
- Show completion status

---

## Security Considerations

### Environment Variables

- `JWT_SECRET`: Required for API authentication
- `HELIXAGENT_API_KEY`: API key for HelixAgent access
- `*_API_KEY`: Provider-specific API keys

### Git-Ignored Files

- `.env` - Contains API keys
- `results/` - Challenge results
- `opencode.json` - Contains API keys
- `*.log` - Log files

### Git-Versioned Templates

- `.env.example` - Template with placeholders
- `opencode.json.example` - Redacted version

---

## Quick Reference

### Run All Challenges

```bash
./challenges/scripts/run_all_challenges.sh
# Expected: 39/39 PASSED
```

### Run Main Challenge

```bash
./challenges/scripts/main_challenge.sh
# Generates: /home/user/Downloads/opencode-helix-agent.json
```

### Run Tests

```bash
make test
# Expected: All tests pass
```

### Build Everything

```bash
make build                    # HelixAgent
cd LLMsVerifier && make build # LLMsVerifier
```

---

## Important Clarifications

### What HelixAgent IS:

- A Virtual LLM Provider exposing ONE model
- An AI debate orchestrator combining multiple LLMs
- An OpenAI-compatible API server
- A protocol gateway (MCP, LSP, ACP)

### What HelixAgent is NOT:

- A simple load balancer
- A multi-model router
- A proxy to other APIs
- A collection of separate model endpoints

### Key Points:

1. **ONE model endpoint**: `helixagent/helixagent-debate`
2. **REAL verification**: All scores from actual API calls
3. **Auto-start**: Infrastructure starts automatically
4. **Both runtimes**: Docker and Podman supported
5. **39 challenges**: All must pass for production readiness

---

*Last Updated: 2026-01-05*
*Version: 1.0.0*
