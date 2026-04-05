# Tier 1 CLI Agent Analysis
## Comprehensive Analysis for HelixAgent Integration

**Date:** 2026-04-03
**Scope:** Top-tier CLI agents with enterprise-grade features

---

## Executive Summary

| Agent | Publisher | Technology | Key Strength | MCP | HelixAgent Priority |
|-------|-----------|------------|--------------|-----|---------------------|
| **claude-code** | Anthropic | Node.js | Code understanding, agentic workflows | ❌ | 🔴 Critical |
| **codex** | OpenAI | Node.js | ChatGPT integration, IDE ecosystem | ❌ | 🔴 Critical |
| **gemini-cli** | Google | Node.js | Free tier, search grounding, MCP | ✅ | 🟡 High |
| **qwen-code** | Alibaba/Qwen | Node.js | Multi-protocol, free tier | ❌ | 🟡 High |
| **aider** | Aider | Python | Git-native, multi-file editing | ❌ | 🟡 High |

---

## 1. Claude Code (Anthropic)

### Overview
**Repository:** `cli_agents/claude-code` (git@github.com:anthropics/claude-code.git)  
**NPM Package:** `@anthropic-ai/claude-code`  
**Language:** TypeScript/Node.js  
**License:** Commercial (Anthropic ToS)

### Installation Methods
| Method | Command |
|--------|---------|
| Mac/Linux (Recommended) | `curl -fsSL https://claude.ai/install.sh \| bash` |
| Homebrew | `brew install --cask claude-code` |
| Windows | `irm https://claude.ai/install.ps1 \| iex` |
| WinGet | `winget install Anthropic.ClaudeCode` |
| NPM (Deprecated) | `npm install -g @anthropic-ai/claude-code` |

### Key Features
- **Codebase Understanding:** Deep analysis of large codebases
- **Natural Language Interface:** Execute tasks via conversational commands
- **Git Integration:** Full git workflow support (commit, push, branch management)
- **Sandboxed Execution:** Safe command execution with approval workflows
- **Multi-modal:** Can process images and diagrams (Claude 3.5+)
- **Agentic Workflows:** Complex multi-step tasks with tool usage

### Architecture
```
claude-code/
├── .claude/              # Configuration
├── .claude-plugin/       # Plugin system
├── plugins/              # Official plugins
├── examples/             # Example prompts
└── .github/              # GitHub integration
```

### Authentication
- Requires Anthropic API key or ChatGPT plan login
- OAuth-based authentication with browser flow

### HelixAgent Integration Strategy
1. **Protocol Adapter:** Bridge to Claude Code's internal communication protocol
2. **Tool Extraction:** Leverage Claude's tool system for HelixAgent capabilities
3. **Context Sharing:** Bidirectional context synchronization
4. **Plugin System:** Integrate HelixAgent as a Claude Code plugin

### API Endpoints (Inferred)
- `/health` - Service health check
- `/v1/conversations` - Conversation management
- `/v1/tools` - Tool execution interface
- `/v1/context` - Context synchronization

---

## 2. Codex CLI (OpenAI)

### Overview
**Repository:** `cli_agents/codex` (git@github.com:openai/codex.git)  
**NPM Package:** `@openai/codex`  
**Language:** TypeScript/Node.js (with Bazel build system)  
**License:** Apache 2.0

### Installation Methods
| Method | Command |
|--------|---------|
| NPM | `npm install -g @openai/codex` |
| Homebrew | `brew install --cask codex` |
| GitHub Release | Download platform-specific binary |

### Key Features
- **ChatGPT Integration:** Native integration with ChatGPT plans
- **IDE Support:** VS Code, Cursor, Windsurf extensions
- **Desktop App:** Run `codex app` for GUI experience
- **Multi-platform:** macOS, Linux, Windows support
- **Cloud Agent:** Connects to Codex Web at chatgpt.com/codex

### Architecture
```
codex/
├── codex-cli/            # Main CLI implementation
├── .codex/               # Configuration
├── docs/                 # Documentation
├── .bazelrc              # Bazel build configuration
└── BUILD.bazel           # Build targets
```

### Authentication
- **Sign in with ChatGPT:** Recommended for Plus/Pro/Team/Edu/Enterprise plans
- **API Key:** Requires additional setup for programmatic access

### HelixAgent Integration Strategy
1. **OpenAI API Bridge:** Codex uses standard OpenAI API format
2. **Model Routing:** Route HelixAgent requests through Codex CLI
3. **Completion Protocol:** Implement OpenAI-compatible endpoints
4. **Streaming Support:** SSE-based streaming for real-time responses

### API Endpoints (OpenAI Compatible)
- `POST /v1/chat/completions` - Chat completions
- `POST /v1/completions` - Text completions
- `GET /v1/models` - Available models
- `POST /v1/embeddings` - Embeddings

---

## 3. Gemini CLI (Google)

### Overview
**Repository:** `cli_agents/gemini-cli` (git@github.com:google-gemini/gemini-cli.git)  
**NPM Package:** `@google/gemini-cli`  
**Language:** TypeScript/Node.js  
**License:** Apache 2.0

### Installation Methods
| Method | Command |
|--------|---------|
| NPX | `npx @google/gemini-cli` |
| NPM | `npm install -g @google/gemini-cli` |
| Homebrew | `brew install gemini-cli` |
| MacPorts | `sudo port install gemini-cli` |
| Anaconda | `conda create -n gemini_env -c conda-forge nodejs && npm install -g @google/gemini-cli` |

### Key Features
- **Free Tier:** 60 requests/min, 1,000 requests/day with Google account
- **Gemini 3 Models:** 1M token context window
- **Built-in Tools:** Google Search, file operations, shell, web fetch
- **MCP Support:** Model Context Protocol for custom integrations
- **Checkpointing:** Save and resume conversations
- **GitHub Action:** Automated PR reviews and issue triage

### Architecture
```
gemini-cli/
├── docs/                 # Documentation
├── evals/                # Evaluation framework
├── docs-site/            # Documentation website
├── .gcp/                 # GCP configuration
└── GEMINI.md             # Project configuration
```

### Authentication Options
1. **Google OAuth** - 60 req/min, 1,000 req/day, automatic model updates
2. **Gemini API Key** - 1,000 req/day, model selection, usage-based billing
3. **Vertex AI** - Enterprise features, scalable, Google Cloud integration

### MCP Server Integration
Configure in `~/.gemini/settings.json`:
```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@anthropic-ai/mcp-server-github"]
    }
  }
}
```

### HelixAgent Integration Strategy
1. **MCP Bridge:** Gemini CLI speaks MCP natively - direct integration
2. **Provider Registry:** Add Gemini as a HelixAgent provider
3. **Context Files:** Support GEMINI.md for project context
4. **Multi-auth:** Support OAuth, API key, and Vertex AI

### API Endpoints (Gemini API)
- `POST /v1beta/models/{model}:generateContent` - Generate content
- `POST /v1beta/models/{model}:streamGenerateContent` - Streaming
- `GET /v1beta/models` - List models
- `POST /v1beta/models/{model}:countTokens` - Token counting

---

## 4. Qwen Code (Alibaba/Qwen)

### Overview
**Repository:** `cli_agents/qwen-code` (git@github.com:QwenLM/qwen-code.git)  
**NPM Package:** `@qwen-code/qwen-code`  
**Language:** TypeScript/Node.js  
**License:** Apache 2.0

### Installation Methods
| Method | Command |
|--------|---------|
| Linux/macOS | `curl -fsSL https://qwen-code-assets.oss-cn-hangzhou.aliyuncs.com/installation/install-qwen.sh \| bash` |
| Windows | `curl -fsSL -o %TEMP%\install-qwen.bat ... && %TEMP%\install-qwen.bat` |
| NPM | `npm install -g @qwen-code/qwen-code@latest` |
| Homebrew | `brew install qwen-code` |

### Key Features
- **Multi-protocol:** OpenAI, Anthropic, Gemini, and Qwen APIs
- **Free Tier:** 1,000 free requests/day with Qwen OAuth
- **Qwen3-Coder:** Optimized for coding tasks
- **Skills System:** Built-in skills and subagents
- **Multi-language:** CLI in 6+ languages (EN, ZH, DE, FR, JA, RU, PT-BR)
- **IDE Support:** VS Code, Zed, JetBrains integrations

### Architecture
```
qwen-code/
├── docs/                 # Documentation
├── docs-site/            # Documentation website
├── .aoneci/              # CI configuration
├── .gcp/                 # GCP integration
└── settings.json         # User configuration
```

### Authentication Methods
1. **Qwen OAuth** - Free 1,000 req/day, browser-based login
2. **API Key** - Supports multiple providers:
   - OpenAI-compatible (Bailian, ModelScope, OpenAI, OpenRouter)
   - Anthropic (Claude models)
   - Google GenAI (Gemini models)

### Configuration (`~/.qwen/settings.json`)
```json
{
  "modelProviders": {
    "openai": [
      {
        "id": "qwen3-coder-plus",
        "name": "qwen3-coder-plus",
        "baseUrl": "https://dashscope.aliyuncs.com/compatible-mode/v1",
        "envKey": "DASHSCOPE_API_KEY"
      }
    ]
  },
  "env": {
    "DASHSCOPE_API_KEY": "sk-xxx"
  },
  "defaultModel": "qwen3-coder-plus"
}
```

### HelixAgent Integration Strategy
1. **Multi-provider:** Leverage Qwen's multi-provider architecture
2. **OpenAI Bridge:** Use OpenAI-compatible API for integration
3. **Skills Export:** Map Qwen Skills to HelixAgent tools
4. **Subagent Routing:** Route HelixAgent tasks to Qwen subagents

---

## 5. Aider (Multi-provider coding assistant)

### Overview
**Repository:** `cli_agents/aider` (git@github.com:Aider-AI/aider.git)  
**PyPI Package:** `aider-chat`  
**Language:** Python  
**License:** Apache 2.0

### Key Features
- **Git-native:** Automatic git commits with meaningful messages
- **Multi-file editing:** Simultaneous edits across multiple files
- **Multi-model:** Supports 20+ LLM providers
- **Repo-map:** Intelligent codebase mapping
- **Voice-to-code:** Speech recognition for coding
- **Test-driven:** Can run tests and iterate on failures

### Supported Providers
OpenAI, Anthropic, Google, Azure, DeepSeek, Groq, Cohere, Mistral, and 15+ more.

### HelixAgent Integration Strategy
1. **Repository Map:** Share Aider's repo-map with HelixAgent
2. **Multi-provider:** Route through Aider's provider abstraction
3. **Git Bridge:** Integrate Aider's git automation
4. **Voice Input:** Support voice-to-code feature

---

## Comparison Matrix

| Feature | Claude Code | Codex | Gemini CLI | Qwen Code | Aider |
|---------|-------------|-------|------------|-----------|-------|
| **Publisher** | Anthropic | OpenAI | Google | Alibaba | Community |
| **License** | Commercial | Apache 2.0 | Apache 2.0 | Apache 2.0 | Apache 2.0 |
| **Language** | Node.js | Node.js | Node.js | Node.js | Python |
| **MCP Support** | ❌ | ❌ | ✅ | ❌ | ❌ |
| **Free Tier** | ❌ | ❌ | ✅ | ✅ | ✅ |
| **Multi-provider** | ❌ | ❌ | ❌ | ✅ | ✅ |
| **IDE Integration** | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Git Integration** | ✅ | ✅ | ✅ | ✅ | ✅ (Native) |
| **Search Grounding** | ❌ | ❌ | ✅ | ❌ | ❌ |
| **Streaming** | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Tool Use** | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Subagents** | ✅ | ✅ | ❌ | ✅ | ❌ |

---

## HelixAgent Integration Priority

### Phase 1: Critical (This Week)
1. **Claude Code** - Most mature, enterprise adoption
2. **Codex** - OpenAI ecosystem integration

### Phase 2: High Priority (Next 2 Weeks)
1. **Gemini CLI** - MCP native, free tier, Google ecosystem
2. **Qwen Code** - Multi-protocol, Alibaba Cloud integration
3. **Aider** - Git-native, multi-provider

### Phase 3: Medium Priority (Next Month)
1. **cline** - VS Code native
2. **roo-code** - Open source alternative
3. **continue** - Universal IDE support

---

## Technical Integration Patterns

### Pattern 1: MCP Bridge (Gemini CLI)
```go
// Gemini CLI speaks MCP natively
mcpClient := NewMCPClient("gemini-cli")
mcpClient.Connect("stdio", []string{"gemini", "--mcp"})
```

### Pattern 2: OpenAI API Bridge (Codex, Qwen)
```go
// Implement OpenAI-compatible endpoints
router.POST("/v1/chat/completions", handleOpenAICompletion)
router.GET("/v1/models", handleOpenAIModels)
```

### Pattern 3: Process Wrapper (Claude Code)
```go
// Wrap Claude CLI as subprocess
claudeCmd := exec.Command("claude", "--json")
stdin, _ := claudeCmd.StdinPipe()
stdout, _ := claudeCmd.StdoutPipe()
// Communicate via JSON-RPC or similar
```

### Pattern 4: Python Import (Aider)
```go
// Use Python bindings or subprocess
aiderCmd := exec.Command("python", "-m", "aider", "--message", prompt)
```

---

## API Documentation Requirements

For each Tier 1 agent, we need:

1. **Authentication Methods**
   - OAuth flows
   - API key formats
   - Token refresh mechanisms

2. **Endpoint Specifications**
   - Request/response schemas
   - Error handling
   - Rate limits

3. **Streaming Protocols**
   - SSE format
   - WebSocket support
   - Chunking strategies

4. **Tool/Function Calling**
   - Schema definitions
   - Execution models
   - Result formats

5. **Context Management**
   - Session state
   - Token counting
   - Checkpointing

---

## Next Steps

1. ✅ **Analysis Complete** - All Tier 1 agents documented
2. 🔄 **Provider API Docs** - Fetch OpenAI, Anthropic, Google, Qwen APIs
3. ⏳ **Integration Design** - Design HelixAgent bridge for each agent
4. ⏳ **Implementation** - Build adapters and MCP bridges
5. ⏳ **Testing** - Validate integrations with real providers

---

**Document Version:** 1.0  
**Last Updated:** 2026-04-03  
**Author:** HelixAgent Team
