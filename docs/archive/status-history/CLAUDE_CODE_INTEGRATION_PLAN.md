# Claude Code Full Integration Plan

## Executive Summary

This document outlines the multi-phase incorporation of all Claude Code CLI features into HelixAgent. The goal is to expose HelixAgent as the central model hub, supporting all 47+ CLI agents with maximum API integration for Claude Code (subscription and pay-as-you-go plans).

**Source:** `cli_agents/claude-code-source` - Comprehensive analysis of 1,914 TypeScript files

---

## Phase Overview

### Phase 1: Analysis & Planning ✅ STARTED
- Document all Claude Code APIs, features, and architecture
- Identify integration points with HelixAgent
- Create implementation roadmap

### Phase 2: Core API Integration
- Anthropic Messages API (`/v1/messages`)
- Files API (`/v1/files/{id}/content`)
- OAuth API (`/v1/oauth/token`, `/api/oauth/*`)
- Account & Usage API (`/api/oauth/usage`, `/api/oauth/account/*`)
- Bootstrap API (`/api/claude_cli/bootstrap`)

### Phase 3: Advanced Features
- MCP Proxy API integration
- LSP (Language Server Protocol) support
- ACP (Agent Communication Protocol) support
- Embeddings API
- Vision capabilities
- Web search

### Phase 4: Tool System
- Port 50+ tools from TypeScript to Go
- File tools (read, write, edit, glob, grep)
- Shell tools (bash, PowerShell)
- AI tools (agent, brief, web search, web fetch)
- MCP tools
- Task tools

### Phase 5: Internal Features
- BUDDY System (Terminal Tamagotchi)
- KAIROS (Always-on Assistant)
- Dream System (Memory Consolidation)
- Undercover Mode
- Coordinator Mode (Multi-Agent)
- ULTRAPLAN (Remote Planning)

### Phase 6: LLMsVerifier Integration
- Full provider support for Claude Code
- Multiple strategy support per provider
- Default strategy selection
- Subscription plan detection
- Pay-as-you-go support

### Phase 7: Testing & Documentation
- Comprehensive test coverage
- API documentation
- Feature documentation
- Integration guides

### Phase 8: Final Integration & Deployment
- All submodules updated
- Main repo pushed to all upstreams
- Validation complete

---

## API Integration Details

### 1. Anthropic Messages API

**Endpoint:** `POST /v1/messages`
**Base URL:** `https://api.anthropic.com`

**Features to Implement:**
- Streaming support (`text/event-stream`)
- Tool use (function calling)
- Multi-modal inputs (text, images)
- System prompts
- Temperature, top_p, top_k sampling
- Beta headers support:
  - `interleaved-thinking-2025-05-14`
  - `context-1m-2025-08-07`
  - `structured-outputs-2025-12-15`
  - `web-search-2025-03-05`
  - `fast-mode-2026-02-01`

**Models:**
- `claude-opus-4-1-20251001`
- `claude-sonnet-4-5-20251001`
- `claude-sonnet-4-6-20251001`
- `claude-haiku-4-5-20251001`

### 2. OAuth API

**Endpoints:**
- `POST /v1/oauth/token` - Token exchange/refresh
- `POST /api/oauth/claude_cli/create_api_key` - API key creation
- `GET /api/oauth/claude_cli/roles` - Get user roles

**Scopes:**
```
Console OAuth: ['org:create_api_key', 'user:profile']
Claude.ai OAuth: ['user:profile', 'user:inference', 'user:sessions:claude_code', 
                  'user:mcp_servers', 'user:file_upload']
```

### 3. Usage & Billing API

**Endpoints:**
- `GET /api/oauth/usage` - Rate limits and usage
- `GET /api/oauth/account/settings` - Account settings
- `PATCH /api/oauth/account/settings` - Update settings
- `GET /api/claude_code_grove` - Grove configuration

**Rate Limit Windows:**
- 5-hour rolling window
- 7-day rolling window
- Model-specific limits (Opus, Sonnet)
- Extra usage credits

### 4. Bootstrap API

**Endpoint:** `GET /api/claude_cli/bootstrap`

**Features:**
- Feature flags (44 flags documented)
- Client configuration
- Model options
- Announcements
- Minimum version checking

### 5. MCP Proxy API

**Endpoint:** `/{server_id}` (various MCP servers)
**Base URL:** `https://mcp-proxy.anthropic.com/v1/mcp`

**Supported MCP Servers:**
- GitHub MCP
- File system MCP
- Web search MCP
- Custom MCP servers

---

## Tool System Architecture

### Tool Categories (50+ Tools)

#### File Tools
| Tool | Purpose | Implementation Priority |
|------|---------|------------------------|
| FileReadTool | Read file contents | High |
| FileWriteTool | Write/create files | High |
| FileEditTool | Edit existing files | High |
| GlobTool | Find files by pattern | High |
| GrepTool | Search file contents | High |

#### Shell Tools
| Tool | Purpose | Implementation Priority |
|------|---------|------------------------|
| BashTool | Execute bash commands | High |
| PowerShellTool | Execute PowerShell | Medium |

#### AI Tools
| Tool | Purpose | Implementation Priority |
|------|---------|------------------------|
| AgentTool | Spawn sub-agents | High |
| BriefTool | Summarize content | Medium |
| WebSearchTool | Search the web | High |
| WebFetchTool | Fetch web content | Medium |

#### MCP Tools
| Tool | Purpose | Implementation Priority |
|------|---------|------------------------|
| MCPTool | Call MCP tools | High |
| ListMcpResourcesTool | List MCP resources | Medium |
| ReadMcpResourceTool | Read MCP resources | Medium |

#### Task Tools
| Tool | Purpose | Implementation Priority |
|------|---------|------------------------|
| TaskCreateTool | Create background tasks | Medium |
| TaskListTool | List tasks | Low |
| TaskStopTool | Stop tasks | Medium |

---

## Internal Features

### BUDDY System (Terminal Tamagotchi)

**Features:**
- Mulberry32 PRNG for species determination
- 5 rarity tiers (Common to Legendary)
- Shiny variants (1% chance)
- 5 stats: DEBUGGING, PATIENCE, CHAOS, WISDOM, SNARK
- 5-line ASCII art sprites
- Animation system
- Speech bubble comments

**Implementation:**
- Go port of PRNG
- ASCII art renderer
- Stats system
- Animation frames

### KAIROS (Always-On Assistant)

**Features:**
- Append-only daily logs
- Tick system for proactive actions
- 15-second blocking budget
- Brief mode for concise responses
- Exclusive tools: SendUserFile, PushNotification, SubscribePR

**Implementation:**
- Background task scheduler
- Log rotation
- Budget tracking
- Notification system

### Dream System (Memory Consolidation)

**Features:**
- Three-gate trigger system (Time, Session, Lock)
- Four-phase process
- Forked subagent for processing
- Memory synthesis

**Implementation:**
- Trigger evaluation
- Background processing
- Memory file management

---

## LLMsVerifier Integration

### Provider Strategy System

Each provider supports multiple strategies:

```go
type ProviderStrategy struct {
    Name        string
    IsDefault   bool
    Features    []string
    AuthType    string  // "oauth", "api_key"
    Endpoints   map[string]string
}
```

### Claude Code Strategies

| Strategy | Description | Default |
|----------|-------------|---------|
| `claude_code_standard` | Basic API integration | No |
| `claude_code_full` | Full API + all features | Yes |
| `claude_code_oauth` | OAuth-based authentication | No |
| `claude_code_api_key` | API key authentication | No |

### Strategy Selection Logic

1. Check subscription plan (Pro/Max/Team/Enterprise)
2. Detect authentication method (OAuth vs API key)
3. Select appropriate strategy
4. Fall back to standard if needed

---

## Implementation Roadmap

### Week 1: Core APIs
- [ ] Messages API with streaming
- [ ] OAuth authentication
- [ ] Usage & billing tracking
- [ ] Bootstrap configuration

### Week 2: Tool System
- [ ] File tools (read, write, edit, glob, grep)
- [ ] Shell tools (bash, PowerShell)
- [ ] AI tools (agent, brief, web search)
- [ ] MCP tool integration

### Week 3: Advanced Features
- [ ] MCP Proxy API
- [ ] LSP support
- [ ] ACP support
- [ ] Embeddings API
- [ ] Vision capabilities

### Week 4: Internal Features
- [ ] BUDDY system
- [ ] KAIROS implementation
- [ ] Dream system
- [ ] Multi-agent coordination

### Week 5: Integration & Testing
- [ ] LLMsVerifier integration
- [ ] Comprehensive tests
- [ ] Documentation
- [ ] Performance optimization

### Week 6: Deployment
- [ ] Final testing
- [ ] All submodules pushed
- [ ] Main repo pushed
- [ ] Validation complete

---

## Technical Specifications

### Go Implementation Details

**Package Structure:**
```
internal/clis/claude/
├── api/              # API client implementations
│   ├── messages.go   # Messages API
│   ├── oauth.go      # OAuth flow
│   ├── files.go      # Files API
│   ├── usage.go      # Usage tracking
│   └── bootstrap.go  # Bootstrap config
├── tools/            # Tool implementations
│   ├── file/         # File tools
│   ├── shell/        # Shell tools
│   ├── ai/           # AI tools
│   └── mcp/          # MCP tools
├── features/         # Internal features
│   ├── buddy/        # BUDDY system
│   ├── kairos/       # KAIROS assistant
│   └── dream/        # Dream system
├── strategy/         # Provider strategies
│   ├── standard.go
│   ├── full.go
│   ├── oauth.go
│   └── api_key.go
└── integration.go    # Main integration point
```

### TypeScript to Go Porting Guide

| TypeScript | Go Equivalent |
|------------|---------------|
| `interface` | `struct` |
| `type` | `type` alias or `interface` |
| `async/await` | Goroutines + channels |
| `Promise<T>` | `chan T` or callback |
| `Map<K,V>` | `map[K]V` |
| `Array<T>` | `[]T` |
| `optional (? )` | Pointer `*T` |
| `union types` | `interface{}` with type switch |

---

## Testing Strategy

### Unit Tests
- API client tests
- Tool execution tests
- Feature flag tests

### Integration Tests
- End-to-end API flows
- OAuth flow tests
- Tool system integration

### LLMsVerifier Tests
- Provider strategy tests
- Rate limit handling
- Error scenario tests

---

## Documentation Plan

1. **API Documentation** - All endpoints documented
2. **Tool Reference** - Complete tool catalog
3. **Feature Guide** - Internal features explained
4. **Integration Guide** - How to use with HelixAgent
5. **Migration Guide** - From standalone to integrated

---

## Success Criteria

- [ ] All 50+ tools ported and functional
- [ ] All APIs integrated with proper error handling
- [ ] LLMsVerifier shows 100% Claude Code support
- [ ] All 47 CLI agents work through HelixAgent
- [ ] Tests pass with >90% coverage
- [ ] Documentation complete
- [ ] All repos pushed to upstreams

---

**Document Version:** 1.0  
**Last Updated:** 2026-04-03  
**Status:** Phase 1 In Progress
