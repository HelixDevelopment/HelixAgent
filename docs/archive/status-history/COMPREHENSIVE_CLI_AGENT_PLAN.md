# Comprehensive CLI Agent Integration Plan
## HelixAgent Project

**Date:** 2026-04-03  
**Current State:** 49 CLI agent directories, 48 submodules registered, 1 empty (crush)  
**Goal:** Complete CLI agent ecosystem with in-depth analysis, provider compatibility matrix, and full HelixAgent integration

---

## Phase 0: Preparation & Discovery

### 0.1 CLI Agent Repository Status Audit
**Duration:** 1 day  
**Status:** ✅ COMPLETED

Current inventory:
- **Total directories:** 49
- **Active submodules:** 48
- **Empty directories:** 1 (crush - needs submodule added)
- **Missing from user's request:** 
  - ✅ zero-cli → Found as "zeroshot" (@covibes/zeroshot)
  - ⚠️ x-cmd/codex → Part of x-cmd main repo (module, not standalone)
  - ⚠️ x deepseek init → Part of x-cmd main repo (command, not standalone)
  - ⚠️ cli-agent → Multiple candidates (needs clarification)
  - ❌ xela-cli → NOT FOUND on GitHub/GitLab

### 0.2 Research Additional CLI Agents
**Duration:** 2 days

Search for and identify any additional CLI agents not yet in the ecosystem:
- Check awesome-cli-coding-agents list (80+ agents catalogued)
- Search for emerging agents (2025-2026 releases)
- Verify all major provider-native CLIs are included

**Potential additions from research:**
1. **zeroshot** - Multi-agent orchestration CLI (@covibes/zeroshot)
2. **x-cmd** - Modular toolkit with codex/deepseek modules
3. **cli-ai** (@fmdz387) - Agentic AI terminal assistant
4. **pi** - Minimal terminal coding harness
5. **roo-code** - VS Code extension + CLI (22.7k stars)
6. **continue** - IDE extension + CLI (32k stars)
7. **swe-agent** - SWE-bench agent (18.8k stars)
8. **goose** (already have codename-goose, check if different)
9. **opencode** (already have opencode-cli)
10. **open-interpreter** - General purpose CLI agent (63k stars)

---

## Phase 1: Submodule Completion

### 1.1 Add Missing Submodules
**Duration:** 2 days  
**Dependencies:** Phase 0.2

| CLI Agent | Repository URL | Priority | Notes |
|-----------|---------------|----------|-------|
| crush | https://github.com/charmbracelet/crush.git | HIGH | Currently empty directory |
| zeroshot | https://github.com/covibes/zeroshot.git | HIGH | Multi-agent orchestrator |
| x-cmd | https://github.com/x-cmd/x-cmd.git | MEDIUM | Modular toolkit (includes codex/deepseek modules) |
| cli-ai | https://github.com/fmdz387/cli-ai.git | MEDIUM | Agentic terminal assistant |
| pi | https://github.com/pi-mono/pi.git | MEDIUM | Minimal coding harness |
| roo-code | https://github.com/RooVetGit/Roo-Code.git | MEDIUM | VS Code + CLI |
| continue | https://github.com/continuedev/continue.git | MEDIUM | IDE + CLI |
| swe-agent | https://github.com/SWE-agent/SWE-agent.git | LOW | Academic/research focused |
| open-interpreter | https://github.com/OpenInterpreter/open-interpreter.git | MEDIUM | General purpose agent |

**Action items:**
```bash
# Add each as submodule
git submodule add <url> cli_agents/<name>
```

### 1.2 Initialize All Submodules
**Duration:** 1 day

```bash
git submodule update --init --recursive
```

Verify all submodules are properly cloned and accessible.

---

## Phase 2: Comprehensive CLI Agent Analysis

### 2.1 Analysis Framework Definition
**Duration:** 1 day

Each CLI agent will be analyzed across these dimensions:

#### Core Analysis Template
```markdown
# CLI Agent Analysis: <Name>

## 1. Basic Information
- **Repository:** <URL>
- **Language/Stack:** <Primary language>
- **License:** <License type>
- **GitHub Stars:** <Count>
- **Maintenance Status:** <Active/Stale/Deprecated>

## 2. Provider Support Matrix
| Provider | Status | Models | Notes |
|----------|--------|--------|-------|
| OpenAI | ✅/❌ | GPT-5, Codex | |
| Anthropic | ✅/❌ | Claude 3/4 | |
| Google | ✅/❌ | Gemini 2.5 | |
| DeepSeek | ✅/❌ | V3, R1 | |
| Qwen | ✅/❌ | Qwen3-Coder | |
| Local/Ollama | ✅/❌ | Various | |
| OpenRouter | ✅/❌ | Aggregated | |
| Mistral | ✅/❌ | Codestral | |
| Groq | ✅/❌ | Fast inference | |
| Z.AI/GLM | ✅/❌ | GLM-4 | |
| xAI/Grok | ✅/❌ | Grok-3 | |

## 3. Feature Analysis
### 3.1 Core Capabilities
- [ ] Code generation
- [ ] Code editing
- [ ] File operations
- [ ] Shell command execution
- [ ] Git integration
- [ ] Test execution
- [ ] LSP integration
- [ ] MCP support
- [ ] Multi-file editing
- [ ] Context management

### 3.2 Unique Features
<What makes this agent special>

### 3.3 Architecture
- **Execution Model:** Local/Cloud/Hybrid
- **Sandboxing:** None/OS-level/Container
- **Session Management:** Persistent/Ephemeral
- **Context Window:** <Token limit>

## 4. API & Integration Points
### 4.1 CLI Commands
<Complete command reference>

### 4.2 Configuration Format
<Config file structure>

### 4.3 Environment Variables
<Required/supported env vars>

### 4.4 Programmatic API
<If available>

## 5. HelixAgent Integration Analysis
### 5.1 Compatibility Score
<Rating 1-10>

### 5.2 Integration Complexity
<Low/Medium/High>

### 5.3 Recommended Use Cases
<When to use this agent>

### 5.4 Provider Pairing Recommendations
<Best model for this agent>

## 6. Power Features to Port
<Features that could enhance HelixAgent>

## 7. Configuration Export Template
<HelixAgent-compatible config>
```

### 2.2 Execute Agent Analysis
**Duration:** 14 days (parallel work)  
**Total Agents to Analyze:** 60+ (49 existing + new additions)

**Priority Tiers:**

**Tier 1 (Core Agents) - Days 1-3:**
1. claude-code
2. codex
3. gemini-cli
4. aider
5. opencode-cli
6. qwen-code
7. openhands
8. cline
9. kilo-code
10. gptme

**Tier 2 (Major Agents) - Days 4-7:**
11. crush
12. zeroshot
13. x-cmd
14. amazon-q
15. copilot-cli
16. kiro-cli
17. forge
18. plandex
19. gpt-engineer
20. codename-goose

**Tier 3 (Specialized Agents) - Days 8-11:**
21-40. Remaining agents (pi, roo-code, continue, swe-agent, open-interpreter, and all others)

**Tier 4 (MCP/Utility) - Days 12-14:**
41-60+. MCP servers and utility agents

### 2.3 Synthesize Provider Compatibility Matrix
**Duration:** 2 days (parallel with 2.2)

Create master matrix showing:
- Which CLI agent works best with which provider
- Known compatibility issues (e.g., OpenCode slowdowns with DeepSeek/Z.AI)
- Optimal pairings (Claude Code → Claude, Gemini CLI → Gemini, Qwen Code → Qwen)
- Performance characteristics per pairing

---

## Phase 3: HelixAgent Integration Implementation

### 3.1 Configuration Export System
**Duration:** 5 days

For each CLI agent, create:
1. **HelixAgent Config Template** (YAML/JSON)
2. **Environment Setup Script**
3. **Docker Compose Integration** (if applicable)
4. **Feature Flag Mapping**

Example for aichat (already done, replicate for all):
```yaml
# cli_agents_configs/<agent>.yaml
helixagent:
  endpoint: "http://localhost:8080"
  api_key: "${HELIX_API_KEY}"
  model: "helix-ensemble"
  timeout: 120
  features:
    - mcp
    - rag
    - tools
```

### 3.2 Internal Integration Adapters
**Duration:** 10 days

Create Go integration adapters in `internal/clis/agents/`:

**Structure:**
```
internal/clis/agents/
├── <agent-name>/
│   ├── <agent-name>.go          # Main integration
│   ├── config.go                # Configuration types
│   ├── executor.go              # Command execution
│   ├── mcp_integration.go       # MCP protocol support
│   └── <agent-name>_test.go     # Tests
```

**Priority order:**
1. Tier 1 agents (10 agents × 1 day each)

### 3.3 Bash Tools Extraction
**Duration:** 5 days

Continue extracting and unifying bash tools from CLI agents:
- Extract unique tools from each agent
- Standardize with argc metadata
- Add to `tools/bash/`
- Document in unified tools registry

### 3.4 Provider Routing Logic
**Duration:** 5 days

Implement intelligent provider selection:
- Route Claude Code requests → Anthropic API
- Route Gemini CLI requests → Google AI
- Route Qwen Code requests → Alibaba Cloud
- Route Aider/OpenCode → Best available based on task type
- Fallback chain for resilience

---

## Phase 4: Documentation & Quality Assurance

### 4.1 Comprehensive Documentation
**Duration:** 7 days

Create documentation for each agent:
1. **README.md** - Quick start
2. **INTEGRATION.md** - HelixAgent-specific integration
3. **API.md** - Complete API reference
4. **PROVIDER_MATRIX.md** - Provider compatibility
5. **ARCHITECTURE.md** - Internal diagrams (Mermaid)
6. **EXAMPLES.md** - Usage examples

### 4.2 Test Suite Development
**Duration:** 7 days

For each integrated agent:
- Unit tests for adapter
- Integration tests with real CLI
- Provider compatibility tests
- Feature capability tests
- HelixAgent end-to-end tests

### 4.3 HelixQA Test Bank Expansion
**Duration:** 3 days

Expand `HelixQA/` test banks:
- Add test cases for each new CLI agent
- Validate exported configurations
- Test provider/model combinations
- Performance benchmarks

---

## Phase 5: Advanced Integration Features

### 5.1 Multi-Agent Orchestration
**Duration:** 7 days

Implement features inspired by zeroshot and similar:
- Multi-agent workflow support
- Agent-to-agent communication
- Task delegation between agents
- Parallel agent execution

### 5.2 Feature Port from Best-in-Class
**Duration:** 10 days

Port the best features from analyzed agents:
- **From Aider:** Git integration, auto-commit, multi-file editing
- **From Claude Code:** Extended thinking, plan mode, hooks
- **From Codex:** Sandboxing, OS-level security, subagent workflows
- **From OpenCode:** Provider flexibility, LSP integration
- **From Gemini CLI:** Large context window handling
- **From Qwen Code:** Web search integration
- **From Crush:** Beautiful TUI patterns, cross-platform support

### 5.3 Unified Configuration Management
**Duration:** 5 days

Create centralized configuration system:
- Single config file for all CLI agents
- Environment-specific overrides
- Secret management integration
- Dynamic reconfiguration

---

## Phase 6: Performance Optimization & Enterprise Features

### 6.1 Performance Optimization
**Duration:** 5 days

- Lazy loading for CLI agent integrations
- Connection pooling for provider APIs
- Response caching
- Streaming optimization
- Resource usage monitoring

### 6.2 Enterprise Features
**Duration:** 5 days

- Role-based access control per agent
- Usage quotas and rate limiting
- Audit logging
- Compliance reporting
- SSO integration

### 6.3 Provider-Specific Optimizations
**Duration:** 5 days

Based on Phase 2 analysis:
- Optimize DeepSeek/Z.AI integrations (fix slowdowns)
- Implement provider-specific retry logic
- Configure optimal timeout values per provider
- Set up provider health checks

---

## Phase 7: Final Integration & Validation

### 7.1 Master Integration Testing
**Duration:** 5 days

- End-to-end testing of all 60+ agents
- Provider compatibility validation
- Configuration export verification
- Integration adapter testing
- Performance benchmarking

### 7.2 Documentation Finalization
**Duration:** 3 days

- Complete AGENTS.md updates
- Generate API documentation
- Create video/quick-start guides
- Finalize architecture diagrams

### 7.3 Release Preparation
**Duration:** 2 days

- Version tagging
- Release notes
- Migration guides
- Deployment checklist

---

## Summary Timeline

| Phase | Duration | Cumulative |
|-------|----------|------------|
| Phase 0: Preparation | 3 days | 3 days |
| Phase 1: Submodule Completion | 3 days | 6 days |
| Phase 2: Agent Analysis | 16 days | 22 days |
| Phase 3: Integration Implementation | 25 days | 47 days |
| Phase 4: Documentation & QA | 17 days | 64 days |
| Phase 5: Advanced Features | 22 days | 86 days |
| Phase 6: Optimization & Enterprise | 15 days | 101 days |
| Phase 7: Final Validation | 10 days | 111 days |

**Total Estimated Duration: ~111 days (4 months)**

With parallel workstreams, this can be reduced to **60-75 days (2.5-3 months)**.

---

## Immediate Next Steps (Week 1)

1. **Confirm xela-cli** - User to provide correct repository URL if it exists
2. **Select cli-agent variant** - Choose which cli-agent implementation to add
3. **Add missing submodules:**
   - crush (charmbracelet/crush)
   - zeroshot (covibes/zeroshot)
   - x-cmd (x-cmd/x-cmd)
   - cli-ai (fmdz387/cli-ai)
4. **Begin Tier 1 agent analysis** while submodules are being added

---

## Deliverables Checklist

- [ ] 60+ CLI agents as submodules
- [ ] In-depth analysis for each agent (60+ reports)
- [ ] Provider compatibility matrix
- [ ] 60+ HelixAgent config exports
- [ ] 60+ Go integration adapters
- [ ] Unified bash tools library (extracted from all agents)
- [ ] Comprehensive documentation (420+ pages)
- [ ] Test suite (1000+ tests)
- [ ] Multi-agent orchestration features
- [ ] Best-in-class feature ports
- [ ] Enterprise-ready configuration system
- [ ] Performance optimizations
- [ ] Production deployment guide

