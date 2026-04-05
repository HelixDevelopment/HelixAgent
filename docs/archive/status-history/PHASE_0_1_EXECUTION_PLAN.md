# Phase 0-1: Immediate Execution Plan
## Submodule Addition & Initial Analysis

**Execution Date:** 2026-04-03  
**Estimated Duration:** 5 days  
**Goal:** Complete submodule setup and begin agent analysis

---

## Day 1: Repository Setup & Missing Submodules

### Task 1.1: Add Crush Submodule (EMPTY DIRECTORY - PRIORITY HIGH)

Crush is currently an empty directory (0 bytes). This is Charm's CLI agent.

```bash
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent

# Remove empty directory first
rm -rf cli_agents/crush

# Add as proper submodule
git submodule add https://github.com/charmbracelet/crush.git cli_agents/crush

# Initialize
git submodule update --init cli_agents/crush
```

**Repository Info:**
- **URL:** https://github.com/charmbracelet/crush
- **Stars:** 21.6k+
- **Language:** Go
- **License:** Charm License (proprietary, not open source)
- **Features:** Multi-provider, LSP-aware, beautiful TUI, MCP support

### Task 1.2: Add Zeroshot Submodule (Multi-Agent Orchestrator)

```bash
git submodule add https://github.com/covibes/zeroshot.git cli_agents/zeroshot
git submodule update --init cli_agents/zeroshot
```

**Repository Info:**
- **URL:** https://github.com/covibes/zeroshot
- **Package:** @covibes/zeroshot (npm)
- **Language:** TypeScript
- **Features:** Planner + Implementer + Validators workflow, blind validation, supports Claude/Codex/Gemini/OpenCode

### Task 1.3: Add x-cmd Submodule (Modular Toolkit)

```bash
git submodule add https://github.com/x-cmd/x-cmd.git cli_agents/x-cmd
git submodule update --init cli_agents/x-cmd
```

**Repository Info:**
- **URL:** https://github.com/x-cmd/x-cmd
- **Language:** Shell/AWK
- **Features:** 100+ modules, includes `x codex` and `x deepseek` commands
- **Note:** This is a modular toolkit, not a single CLI agent

### Task 1.4: Add cli-ai Submodule

```bash
git submodule add https://github.com/fmdz387/cli-ai.git cli_agents/cli-ai
git submodule update --init cli_agents/cli-ai
```

**Repository Info:**
- **URL:** https://github.com/fmdz387/cli-ai
- **Package:** @fmdzc/cli-ai (npm)
- **Features:** Agentic AI assistant, Anthropic/OpenAI/OpenRouter support

### Task 1.5: Verify All Submodules

```bash
# Check status
git submodule status | wc -l
ls -la cli_agents/ | wc -l

# Verify no empty directories remain
find cli_agents -type d -empty

# Commit changes
git add .gitmodules
git commit -m "Add missing CLI agent submodules: crush, zeroshot, x-cmd, cli-ai"
```

---

## Day 2-3: Additional High-Value Agents (Optional Expansion)

If approved, add these additional high-value agents:

```bash
# Pi - Minimal coding harness
git submodule add https://github.com/pi-mono/pi.git cli_agents/pi

# Roo Code - VS Code + CLI (58k stars for VS Code extension)
git submodule add https://github.com/RooVetGit/Roo-Code.git cli_agents/roo-code

# Continue - IDE + CLI (32k stars)
git submodule add https://github.com/continuedev/continue.git cli_agents/continue

# Open Interpreter - General purpose (63k stars)
git submodule add https://github.com/OpenInterpreter/open-interpreter.git cli_agents/open-interpreter

# SWE-agent - Academic/research (18.8k stars)
git submodule add https://github.com/SWE-agent/SWE-agent.git cli_agents/swe-agent
```

---

## Day 2-5: Begin Tier 1 Agent Analysis

### Analysis Schedule (4-5 agents per day)

**Day 2: Claude Code & Codex**

**Claude Code Analysis:**
```markdown
# CLI Agent Analysis: Claude Code

## 1. Basic Information
- **Repository:** https://github.com/anthropics/claude-code
- **Language/Stack:** TypeScript/Node.js
- **License:** Proprietary (Anthropic)
- **GitHub Stars:** 79.5k+
- **Maintenance Status:** Very Active (Anthropic official)

## 2. Provider Support Matrix
| Provider | Status | Models | Notes |
|----------|--------|--------|-------|
| Anthropic | ✅ Native | Claude Opus 4.6, Sonnet 4.6, Haiku | Primary provider |
| OpenAI | ❌ | - | Not supported |
| Google | ❌ | - | Not supported |
| DeepSeek | ❌ | - | Not supported |
| Local/Ollama | ❌ | - | Not supported |
| OpenRouter | ❌ | - | Not supported |

**Vendor Lock-in:** HIGH - Claude Code ONLY works with Anthropic models

## 3. Feature Analysis
### 3.1 Core Capabilities
- [x] Code generation
- [x] Code editing
- [x] File operations
- [x] Shell command execution
- [x] Git integration
- [x] Test execution
- [ ] LSP integration (limited)
- [x] MCP support
- [x] Multi-file editing
- [x] Context management (200K-1M tokens)

### 3.2 Unique Features
- **Extended Thinking:** Deep reasoning mode
- **Plan Mode:** Review changes before execution
- **Hooks:** 17 lifecycle events for customization
- **Agent Teams:** Parallel development with multiple instances
- **Voice Mode:** Hands-free operation
- **GitHub Integration:** @claude mentions on PRs
- **Remote Control:** Control from other devices

### 3.3 Architecture
- **Execution Model:** Local (client-server to Anthropic API)
- **Sandboxing:** Application-layer (hooks)
- **Session Management:** Persistent with resume
- **Context Window:** Up to 1M tokens (Opus 4.6 beta)

## 4. API & Integration Points
### 4.1 CLI Commands
```
claude                    # Start interactive session
claude --help             # Show help
claude --version          # Show version
claude config             # Configure settings
claude plugins            # Manage plugins
/plugin install <name>    # Install plugin
/plugin marketplace       # Browse marketplace
```

### 4.2 Configuration Format
Location: `~/.claude/settings.json`
```json
{
  "autoUpdater": true,
  "theme": "dark",
  "editor": "vim",
  "hooks": {
    "preToolUse": "./scripts/pre-hook.sh"
  }
}
```

Project config: `.claude/settings.json`

### 4.3 Environment Variables
```bash
ANTHROPIC_API_KEY         # API key
CLAUDE_CONFIG_DIR         # Config directory
CLAUDE_EDITOR             # Preferred editor
```

### 4.4 Plugin System
- Marketplace: Custom registries
- Hook events: 17 lifecycle events
- Skills: SKILL.md standard support

## 5. HelixAgent Integration Analysis
### 5.1 Compatibility Score: 8/10
- Excellent agentic capabilities
- Strong MCP support
- Limited to Claude models only

### 5.2 Integration Complexity: Medium
- Well-documented API
- Plugin system for extensions
- Requires Anthropic API key

### 5.3 Recommended Use Cases
- Complex refactoring tasks
- Multi-file changes
- Deep reasoning required
- Git workflow automation
- Teams already using Claude

### 5.4 Provider Pairing Recommendations
**OPTIMAL:** Claude Opus 4.6 (1M context, best reasoning)
**BALANCED:** Claude Sonnet 4.6 (faster, cheaper)
**FAST:** Claude Haiku 4.5 (quick tasks)

## 6. Power Features to Port
1. **Plan Mode** - Review before execute
2. **Hooks System** - Lifecycle event interception
3. **Agent Teams** - Multi-instance coordination
4. **Extended Thinking** - Deep reasoning toggle
5. **Voice Mode** - Speech input

## 7. Configuration Export Template
```yaml
# cli_agents_configs/claude-code.yaml
helixagent:
  endpoint: "http://localhost:8080"
  api_key: "${HELIX_API_KEY}"
  
claude_code:
  provider: anthropic
  models:
    default: claude-sonnet-4-6
    available:
      - claude-opus-4-6
      - claude-sonnet-4-6
      - claude-haiku-4-5
  features:
    - extended_thinking
    - plan_mode
    - mcp
    - hooks
    - agent_teams
  context_window: 200000  # Sonnet default
  timeout: 300
  hooks:
    enabled: true
    events:
      - pre_tool_use
      - post_tool_use
      - pre_edit
      - post_edit
```
```

**Codex Analysis:**
(Similar detailed analysis for OpenAI Codex CLI)

---

**Day 3: Gemini CLI & Qwen Code**

**Gemini CLI Analysis:**
- **Provider:** Google-only (similar lock-in to Claude Code)
- **Free Tier:** 1,000 requests/day, 60 req/min
- **Context:** 1M tokens (largest available)
- **License:** Apache 2.0 (open source!)
- **Unique:** Google Search grounding, multimodal

**Qwen Code Analysis:**
- **Provider:** Alibaba Qwen models
- **Free:** 2,000 requests/day via Qwen OAuth
- **Context:** 256K-1M tokens
- **License:** Apache 2.0
- **Unique:** Web search integration, Zed editor integration

---

**Day 4: Aider & OpenCode**

**Aider Analysis:**
- **Providers:** 75+ (most flexible!)
- **Unique:** Auto-git commits, repo mapping, voice mode
- **License:** Apache 2.0
- **Stars:** 42k+

**OpenCode Analysis:**
- **Providers:** 75+ via Models.dev
- **Subscription Piggybacking:** Can use Copilot/ChatGPT Plus
- **Unique:** Multi-session agents, LSP integration
- **Issues:** Known slowdowns with DeepSeek/Z.AI

---

**Day 5: OpenHands, Cline, Kilo-Code, Gptme**

Continue with remaining Tier 1 agents...

---

## Analysis Output Structure

Each analysis will be saved to:
```
cli_agents_analysis/
├── tier1/
│   ├── claude-code.md
│   ├── codex.md
│   ├── gemini-cli.md
│   ├── qwen-code.md
│   ├── aider.md
│   ├── opencode-cli.md
│   ├── openhands.md
│   ├── cline.md
│   ├── kilo-code.md
│   └── gptme.md
├── tier2/
│   └── (10 agents)
├── tier3/
│   └── (20 agents)
└── tier4/
    └── (20+ agents)
```

---

## Command Reference

### Submodule Management
```bash
# Add new submodule
git submodule add <url> cli_agents/<name>

# Initialize all
git submodule update --init --recursive

# Update specific submodule
git submodule update --remote cli_agents/<name>

# Remove submodule (if needed)
git submodule deinit cli_agents/<name>
git rm cli_agents/<name>
rm -rf .git/modules/cli_agents/<name>
```

### Analysis Documentation
```bash
# Create analysis directory structure
mkdir -p cli_agents_analysis/{tier1,tier2,tier3,tier4}

# Template for new analysis
cp templates/agent-analysis-template.md cli_agents_analysis/tier1/<agent>.md
```

---

## Success Criteria

By end of Day 5:
- [ ] 53+ submodules added (49 existing + 4 new minimum)
- [ ] All submodules initialized and verified
- [ ] 10 Tier 1 agent analyses completed
- [ ] Provider compatibility matrix started
- [ ] Configuration export templates created for Tier 1

---

## Blockers & Decisions Needed

1. **xela-cli:** Cannot find on GitHub/GitLab - need user to provide URL
2. **cli-agent:** Multiple implementations - need user to select:
   - Option A: fmdz387/cli-ai (TypeScript, mature)
   - Option B: Mujhtech/agent-cli (Go, lightweight)
   - Option C: basnijholt/agent-cli (Python, local-first)
3. **Additional agents:** Approve list of 5-10 additional agents to add

