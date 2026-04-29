#!/bin/bash
# ============================================================================
# HelixAgent CLI Agent Configuration Generator
# ============================================================================
# Generates configuration files for all 47+ supported CLI agents
# and copies them to proper filesystem locations
#
# Usage: ./generate-all-configs.sh [--install] [--dry-run] [--agent=NAME]
# ============================================================================

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Configuration
# Default port: 8100 per docs/development/port-registry.md and CLAUDE.md
# CLI Agent Config Rule #4 ("Both use http://localhost:8100/v1 as
# provider base URL"). The previous default of 7061 was stale and
# caused every generated CLI-agent config to point at a port HelixAgent
# does not listen on, breaking the install for end users.
HELIX_AGENT_URL="${HELIX_AGENT_URL:-http://localhost:8100}"
HELIX_AGENT_API_KEY="${HELIX_AGENT_API_KEY:-helixagent-local}"
LLMS_VERIFIER_URL="${LLMS_VERIFIER_URL:-http://localhost:8081}"

# Output directories
CONFIG_OUTPUT_DIR="$SCRIPT_DIR/configs/generated"
BACKUP_DIR="$SCRIPT_DIR/configs/backups/$(date +%Y%m%d_%H%M%S)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Parse arguments
DRY_RUN=false
INSTALL=false
SPECIFIC_AGENT=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --install)
            INSTALL=true
            shift
            ;;
        --agent=*)
            SPECIFIC_AGENT="${1#*=}"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [--install] [--dry-run] [--agent=NAME]"
            echo ""
            echo "Options:"
            echo "  --install     Install configs to system locations"
            echo "  --dry-run     Show what would be done without making changes"
            echo "  --agent=NAME  Generate config for specific agent only"
            echo ""
            echo "Environment variables:"
            echo "  HELIX_AGENT_URL      HelixAgent server URL (default: http://localhost:8080)"
            echo "  HELIX_AGENT_API_KEY  API key for HelixAgent"
            echo "  LLMS_VERIFIER_URL    LLMsVerifier URL (default: http://localhost:8081)"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Create output directories
mkdir -p "$CONFIG_OUTPUT_DIR"
if [[ "$INSTALL" == "true" ]]; then
    mkdir -p "$BACKUP_DIR"
fi

# ============================================================================
# CLI Agent Definitions
# ============================================================================

# Array of all supported CLI agents with their configurations
declare -A CLI_AGENTS

# Tier 1 - Primary Support
CLI_AGENTS["claude_code"]="TypeScript|~/.claude|settings.json"
CLI_AGENTS["aider"]="Python|~/.aider.conf.yml|yaml"
CLI_AGENTS["cline"]="TypeScript|~/.cline|config.json"
CLI_AGENTS["opencode"]="Go|~/.config/opencode|opencode.json"
CLI_AGENTS["kilo_code"]="TypeScript|~/.kilo-code|config.json"
CLI_AGENTS["gemini_cli"]="Python|~/.config/gemini-cli|config.yaml"
CLI_AGENTS["qwen_code"]="Python|~/.qwen|config.json"
CLI_AGENTS["deepseek_cli"]="Python|~/.deepseek|config.json"
CLI_AGENTS["forge"]="TypeScript|~/.forge|config.json"
CLI_AGENTS["codename_goose"]="Go|~/.config/goose|config.yaml"

# Tier 2 - Secondary Support
CLI_AGENTS["amazon_q"]="TypeScript|~/.aws/amazonq|config.json"
CLI_AGENTS["kiro"]="Python|~/.kiro/steering|config.yaml"
CLI_AGENTS["gpt_engineer"]="Python|~/.gpt-engineer|config.yaml"
CLI_AGENTS["mistral_code"]="Python|~/.mistral|config.json"
CLI_AGENTS["ollama_code"]="Python|~/.ollama-code|config.json"
CLI_AGENTS["plandex"]="Go|~/.plandex|config.json"
CLI_AGENTS["codex"]="TypeScript|~/.codex|config.json"
CLI_AGENTS["vtcode"]="TypeScript|~/.vtcode|config.json"
CLI_AGENTS["nanocoder"]="Python|~/.nanocoder|config.yaml"
CLI_AGENTS["gitmcp"]="TypeScript|~/.gitmcp|config.json"
CLI_AGENTS["taskweaver"]="Python|~/.taskweaver|config.yaml"
CLI_AGENTS["octogen"]="Python|~/.octogen|config.yaml"
CLI_AGENTS["fauxpilot"]="Python|~/.fauxpilot|config.json"
CLI_AGENTS["bridle"]="Go|~/.bridle|config.yaml"
CLI_AGENTS["agent_deck"]="TypeScript|~/.agent-deck|config.json"

# Tier 3 - Extended Support
CLI_AGENTS["claude_squad"]="TypeScript|~/.claude-squad|config.json"
CLI_AGENTS["codai"]="Python|~/.codai|config.yaml"
CLI_AGENTS["emdash"]="TypeScript|~/.emdash|config.json"
CLI_AGENTS["get_shit_done"]="Python|~/.gsd|config.yaml"
CLI_AGENTS["github_copilot_cli"]="TypeScript|~/.config/github-copilot|config.json"
CLI_AGENTS["github_spec_kit"]="TypeScript|~/.github-spec-kit|config.json"
CLI_AGENTS["gptme"]="Python|~/.config/gptme|config.yaml"
CLI_AGENTS["mobile_agent"]="Python|~/.mobile-agent|config.yaml"
CLI_AGENTS["multiagent_coding"]="Python|~/.multiagent|config.yaml"
CLI_AGENTS["noi"]="TypeScript|~/.noi|config.json"
CLI_AGENTS["openhands"]="Python|~/.openhands|config.yaml"
CLI_AGENTS["postgres_mcp"]="TypeScript|~/.postgres-mcp|config.json"
CLI_AGENTS["shai"]="Python|~/.shai|config.yaml"
CLI_AGENTS["snowcli"]="Python|~/.snowcli|config.yaml"
CLI_AGENTS["superset"]="Python|~/.superset-ai|config.yaml"
CLI_AGENTS["warp"]="Rust|~/.warp|config.yaml"
CLI_AGENTS["cheshire_cat"]="Python|~/.cheshire-cat|config.yaml"
CLI_AGENTS["conduit"]="Go|~/.conduit|config.yaml"
CLI_AGENTS["crush"]="TypeScript|~/.config/crush|crush.json"
CLI_AGENTS["helixcode"]="Go|~/.config/helixcode|config.json"

# ============================================================================
# Configuration Generation Functions
# ============================================================================

generate_json_config() {
    local agent_name="$1"
    local config_file="$2"

    cat > "$config_file" << EOF
{
  "version": "1.0.0",
  "agent": "${agent_name}",
  "generated_at": "$(date -Iseconds)",
  "helix_agent": {
    "enabled": true,
    "url": "${HELIX_AGENT_URL}",
    "api_key": "${HELIX_AGENT_API_KEY}",
    "timeout": 120000,
    "retry": {
      "max_attempts": 3,
      "backoff_ms": 1000
    }
  },
  "llms_verifier": {
    "enabled": true,
    "url": "${LLMS_VERIFIER_URL}",
    "verify_on_startup": true
  },
  "provider": {
    "type": "helix_ensemble",
    "model": "ai-debate-ensemble",
    "fallback_providers": ["deepseek", "gemini", "mistral"]
  },
  "features": {
    "ai_debate": true,
    "multi_pass_validation": true,
    "streaming": true,
    "tool_use": true,
    "mcp_support": true
  },
  "streaming": {
    "enabled": true,
    "mode": "sse",
    "buffer_size": 4096
  },
  "compression": {
    "enabled": true,
    "algorithm": "brotli",
    "level": 6
  },
  "events": {
    "subscribe": [
      "debate.started",
      "debate.response",
      "debate.validation",
      "debate.conclusion",
      "warning.*",
      "error.*",
      "background.*"
    ]
  },
  "plugins": {
    "enabled": true,
    "auto_load": true,
    "directory": "~/.helix-plugins/${agent_name}"
  },
  "logging": {
    "level": "info",
    "file": "~/.helix-logs/${agent_name}.log"
  }
}
EOF
}

generate_yaml_config() {
    local agent_name="$1"
    local config_file="$2"

    cat > "$config_file" << EOF
# HelixAgent Configuration for ${agent_name}
# Generated: $(date -Iseconds)

version: "1.0.0"
agent: ${agent_name}

helix_agent:
  enabled: true
  url: ${HELIX_AGENT_URL}
  api_key: ${HELIX_AGENT_API_KEY}
  timeout: 120000
  retry:
    max_attempts: 3
    backoff_ms: 1000

llms_verifier:
  enabled: true
  url: ${LLMS_VERIFIER_URL}
  verify_on_startup: true

provider:
  type: helix_ensemble
  model: ai-debate-ensemble
  fallback_providers:
    - deepseek
    - gemini
    - mistral

features:
  ai_debate: true
  multi_pass_validation: true
  streaming: true
  tool_use: true
  mcp_support: true

streaming:
  enabled: true
  mode: sse
  buffer_size: 4096

compression:
  enabled: true
  algorithm: brotli
  level: 6

events:
  subscribe:
    - debate.started
    - debate.response
    - debate.validation
    - debate.conclusion
    - warning.*
    - error.*
    - background.*

plugins:
  enabled: true
  auto_load: true
  directory: ~/.helix-plugins/${agent_name}

logging:
  level: info
  file: ~/.helix-logs/${agent_name}.log
EOF
}

# ============================================================================
# Agent-Specific Configuration Generators
# ============================================================================

generate_claude_code_config() {
    local output_dir="$CONFIG_OUTPUT_DIR/claude_code"
    mkdir -p "$output_dir"

    # Main settings.json
    cat > "$output_dir/settings.json" << EOF
{
  "version": "1.0.0",
  "providers": {
    "helix": {
      "type": "openai-compatible",
      "baseUrl": "${HELIX_AGENT_URL}/v1",
      "apiKey": "${HELIX_AGENT_API_KEY}",
      "model": "ai-debate-ensemble",
      "features": {
        "streaming": true,
        "tools": true
      }
    }
  },
  "defaultProvider": "helix",
  "features": {
    "aiDebate": true,
    "multiPassValidation": true,
    "verifier": {
      "enabled": true,
      "url": "${LLMS_VERIFIER_URL}"
    }
  },
  "plugins": {
    "enabled": true,
    "autoLoad": ["helix-integration", "debate-ui", "event-handler"]
  },
  "hooks": {
    "preRequest": [],
    "postResponse": ["helix-event-processor"]
  }
}
EOF

    # CLAUDE.md for project instructions
    cat > "$output_dir/CLAUDE.md" << EOF
# HelixAgent Integration

This project uses HelixAgent as the AI backend provider.

## Configuration

- **Provider**: HelixAgent Ensemble (AI Debate)
- **URL**: ${HELIX_AGENT_URL}
- **Features**: Multi-pass validation, streaming, tool use

## Available Models

- \`ai-debate-ensemble\` - Full AI debate with 15 LLMs
- \`helix-fast\` - Single provider for quick responses

## Plugins

The following HelixAgent plugins are installed:
- helix-integration
- debate-ui
- event-handler
EOF

    log_success "Generated Claude Code config: $output_dir"
}

generate_aider_config() {
    local output_dir="$CONFIG_OUTPUT_DIR/aider"
    mkdir -p "$output_dir"

    cat > "$output_dir/.aider.conf.yml" << EOF
# Aider configuration for HelixAgent integration
# Generated: $(date -Iseconds)

# Model configuration
model: ai-debate-ensemble
openai-api-base: ${HELIX_AGENT_URL}/v1
openai-api-key: ${HELIX_AGENT_API_KEY}

# HelixAgent specific
helix:
  enabled: true
  verifier_url: ${LLMS_VERIFIER_URL}
  ai_debate: true
  multi_pass_validation: true

# Git integration
auto-commits: true
dirty-commits: false
attribute-author: true
attribute-committer: true

# Features
stream: true
pretty: true
dark-mode: true

# Voice (if available)
voice-language: en

# Map tokens
map-tokens: 1024

# Auto-lint
auto-lint: true
lint-cmd: "golangci-lint run"

# Testing
auto-test: true
test-cmd: "go test ./..."
EOF

    log_success "Generated Aider config: $output_dir"
}

generate_cline_config() {
    local output_dir="$CONFIG_OUTPUT_DIR/cline"
    mkdir -p "$output_dir"

    cat > "$output_dir/config.json" << EOF
{
  "apiProvider": "openai-compatible",
  "openAiBaseUrl": "${HELIX_AGENT_URL}/v1",
  "openAiApiKey": "${HELIX_AGENT_API_KEY}",
  "openAiModelId": "ai-debate-ensemble",
  "helix": {
    "enabled": true,
    "verifierUrl": "${LLMS_VERIFIER_URL}",
    "aiDebate": true,
    "multiPassValidation": true,
    "streaming": true
  },
  "customInstructions": "You are integrated with HelixAgent AI Debate ensemble. Use multi-pass validation for important decisions.",
  "alwaysAllowReadOnly": true,
  "alwaysAllowWrite": false,
  "alwaysAllowExecute": false,
  "browserEnabled": true,
  "mcpEnabled": true,
  "soundEnabled": true,
  "diffEnabled": true,
  "checkpointsEnabled": true
}
EOF

    log_success "Generated Cline config: $output_dir"
}

generate_opencode_config() {
    local output_dir="$CONFIG_OUTPUT_DIR/opencode"
    mkdir -p "$output_dir"

    # Use port 8100 for HelixAgent (canonical default per port-registry.md)
    local helix_url="${HELIX_AGENT_URL:-http://localhost:8100}"
    local helix_api_key="${HELIX_AGENT_API_KEY:-helixagent-local}"

    # Generate config with correct OpenCode schema (mcpServers, providers, agents)
    # env is an array of strings like ["KEY=VALUE"], NOT an object
    # IMPORTANT: OpenCode v1.2.6+ requires "opencode.json" WITHOUT a leading
    # dot (the prior ".opencode.json" filename is no longer recognised — see
    # CLAUDE.md "CLI Agent Config Rules" rule #1).
    # IMPORTANT: Uses "local" provider which reads LOCAL_ENDPOINT env var for HelixAgent URL
    cat > "$output_dir/opencode.json" << 'OPENCODE_EOF'
{
  "providers": {
    "local": {
      "apiKey": "helixagent-local"
    }
  },
  "agents": {
    "coder": {
      "model": "local.helixagent-debate",
      "maxTokens": 8192
    },
    "task": {
      "model": "local.helixagent-debate",
      "maxTokens": 4096
    },
    "title": {
      "model": "local.helixagent-debate",
      "maxTokens": 80
    },
    "summarizer": {
      "model": "local.helixagent-debate",
      "maxTokens": 4096
    }
  },
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home"]
    },
    "fetch": {
      "command": "npx",
      "args": ["-y", "mcp-fetch-server"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": ["GITHUB_TOKEN=${GITHUB_TOKEN}"]
    },
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"]
    },
    "sqlite": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sqlite", "--db-path", "/tmp/helixagent.db"]
    },
    "puppeteer": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-puppeteer"]
    },
    "time": {
      "command": "npx",
      "args": ["-y", "@theo.foobar/mcp-time"]
    },
    "brave-search": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-brave-search"],
      "env": ["BRAVE_API_KEY=${BRAVE_API_KEY}"]
    },
    "google-maps": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-google-maps"],
      "env": ["GOOGLE_MAPS_API_KEY=${GOOGLE_MAPS_API_KEY}"]
    },
    "git": {
      "command": "npx",
      "args": ["-y", "mcp-git"]
    },
    "postgres": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost:5432/helixagent"]
    },
    "slack": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-slack"],
      "env": ["SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}", "SLACK_TEAM_ID=${SLACK_TEAM_ID}"]
    },
    "sequential-thinking": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sequential-thinking"]
    },
    "everart": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-everart"],
      "env": ["EVERART_API_KEY=${EVERART_API_KEY}"]
    },
    "exa": {
      "command": "npx",
      "args": ["-y", "exa-mcp-server"],
      "env": ["EXA_API_KEY=${EXA_API_KEY}"]
    },
    "linear": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-linear"],
      "env": ["LINEAR_API_KEY=${LINEAR_API_KEY}"]
    },
    "sentry": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sentry"],
      "env": ["SENTRY_AUTH_TOKEN=${SENTRY_AUTH_TOKEN}", "SENTRY_ORG=${SENTRY_ORG}"]
    },
    "notion": {
      "command": "npx",
      "args": ["-y", "@notionhq/notion-mcp-server"],
      "env": ["OPENAI_API_KEY=${OPENAI_API_KEY}"]
    },
    "figma": {
      "command": "npx",
      "args": ["-y", "figma-developer-mcp"],
      "env": ["FIGMA_API_KEY=${FIGMA_API_KEY}"]
    },
    "aws-kb-retrieval": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-aws-kb-retrieval"],
      "env": ["AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}"]
    },
    "gitlab": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-gitlab"],
      "env": ["GITLAB_TOKEN=${GITLAB_TOKEN}"]
    },
    "helixagent": {
      "type": "sse",
      "url": "HELIX_URL_PLACEHOLDER/v1/mcp/sse"
    },
    "helixagent-debate": {
      "type": "sse",
      "url": "HELIX_URL_PLACEHOLDER/v1/mcp/debate/sse"
    },
    "helixagent-rag": {
      "type": "sse",
      "url": "HELIX_URL_PLACEHOLDER/v1/mcp/rag/sse"
    },
    "helixagent-memory": {
      "type": "sse",
      "url": "HELIX_URL_PLACEHOLDER/v1/mcp/memory/sse"
    },
    "docker": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-docker"]
    },
    "kubernetes": {
      "command": "npx",
      "args": ["-y", "mcp-server-kubernetes"],
      "env": ["KUBECONFIG=${KUBECONFIG}"]
    },
    "redis": {
      "command": "npx",
      "args": ["-y", "mcp-server-redis"],
      "env": ["REDIS_URL=redis://localhost:6379"]
    },
    "mongodb": {
      "command": "npx",
      "args": ["-y", "mcp-server-mongodb"],
      "env": ["MONGODB_URI=mongodb://localhost:27017"]
    },
    "elasticsearch": {
      "command": "npx",
      "args": ["-y", "mcp-server-elasticsearch"],
      "env": ["ELASTICSEARCH_URL=http://localhost:9200"]
    },
    "qdrant": {
      "command": "npx",
      "args": ["-y", "mcp-server-qdrant"],
      "env": ["QDRANT_URL=http://localhost:6333"]
    },
    "chroma": {
      "command": "npx",
      "args": ["-y", "mcp-server-chroma"],
      "env": ["CHROMA_URL=http://localhost:8001"]
    },
    "jira": {
      "command": "npx",
      "args": ["-y", "mcp-server-atlassian"],
      "env": ["JIRA_URL=${JIRA_URL}", "JIRA_EMAIL=${JIRA_EMAIL}", "JIRA_API_TOKEN=${JIRA_API_TOKEN}"]
    },
    "asana": {
      "command": "npx",
      "args": ["-y", "mcp-server-asana"],
      "env": ["ASANA_ACCESS_TOKEN=${ASANA_ACCESS_TOKEN}"]
    },
    "google-drive": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-gdrive"],
      "env": ["GOOGLE_CREDENTIALS_PATH=${GOOGLE_CREDENTIALS_PATH}"]
    },
    "aws-s3": {
      "command": "npx",
      "args": ["-y", "mcp-server-s3"],
      "env": ["AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}"]
    },
    "datadog": {
      "command": "npx",
      "args": ["-y", "mcp-server-datadog"],
      "env": ["DD_API_KEY=${DD_API_KEY}", "DD_APP_KEY=${DD_APP_KEY}"]
    }
  },
  "contextPaths": [
    "CLAUDE.md",
    "CLAUDE.local.md",
    "opencode.md",
    ".github/copilot-instructions.md"
  ],
  "tui": {
    "theme": "opencode"
  }
}
OPENCODE_EOF

    # Replace placeholders with actual values
    sed -i "s|HELIX_URL_PLACEHOLDER|${helix_url}|g" "$output_dir/opencode.json"
    sed -i "s|HELIX_API_KEY_PLACEHOLDER|${helix_api_key}|g" "$output_dir/opencode.json"

    log_success "Generated OpenCode config with 37 MCPs: $output_dir"
}

generate_crush_config() {
    local output_dir="$CONFIG_OUTPUT_DIR/crush"
    mkdir -p "$output_dir"

    # Use port 8100 for HelixAgent (canonical default per port-registry.md)
    local helix_url="${HELIX_AGENT_URL:-http://localhost:8100}"
    local helix_api_key="${HELIX_AGENT_API_KEY:-helixagent-local}"

    # Generate config with 35+ MCPs
    cat > "$output_dir/crush.json" << 'CRUSH_EOF'
{
  "$schema": "https://charm.land/crush.json",
  "providers": {
    "helixagent": {
      "name": "HelixAgent AI Debate Ensemble",
      "type": "openai",
      "base_url": "HELIX_URL_PLACEHOLDER/v1",
      "api_key": "HELIX_API_KEY_PLACEHOLDER",
      "models": [
        {
          "id": "helixagent-debate",
          "name": "HelixAgent Debate Ensemble (35+ MCPs)",
          "cost_per_1m_in": 0,
          "cost_per_1m_out": 0,
          "context_window": 128000,
          "default_max_tokens": 8192,
          "can_reason": true,
          "supports_attachments": true,
          "streaming": true,
          "supports_brotli": true,
          "options": {
            "embeddings": true,
            "function_calls": true,
            "image_input": true,
            "image_output": true,
            "ocr": true,
            "pdf": true,
            "tool_use": true,
            "vision": true
          }
        }
      ]
    }
  },
  "mcp": {
    "filesystem": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-filesystem", "$HOME"], "enabled": true},
    "fetch": {"command": "npx", "args": ["-y", "mcp-fetch-server"], "enabled": true},
    "github": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-github"], "env": {"GITHUB_TOKEN": "${GITHUB_TOKEN}"}, "enabled": true},
    "memory": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-memory"], "enabled": true},
    "sqlite": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-sqlite", "--db-path", "$HOME/.helixagent/data/helixagent.db"], "enabled": true},
    "puppeteer": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-puppeteer"], "enabled": true},
    "time": {"command": "npx", "args": ["-y", "@theo.foobar/mcp-time"], "enabled": true},
    "brave-search": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-brave-search"], "env": {"BRAVE_API_KEY": "${BRAVE_API_KEY}"}, "enabled": true},
    "google-maps": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-google-maps"], "env": {"GOOGLE_MAPS_API_KEY": "${GOOGLE_MAPS_API_KEY}"}, "enabled": true},
    "git": {"command": "npx", "args": ["-y", "mcp-git"], "enabled": true},
    "postgres": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-postgres"], "env": {"POSTGRES_CONNECTION_STRING": "${POSTGRES_CONNECTION_STRING:-postgresql://localhost:5432/helixagent}"}, "enabled": true},
    "slack": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-slack"], "env": {"SLACK_BOT_TOKEN": "${SLACK_BOT_TOKEN}", "SLACK_TEAM_ID": "${SLACK_TEAM_ID}"}, "enabled": true},
    "sequential-thinking": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-sequential-thinking"], "enabled": true},
    "everart": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-everart"], "env": {"EVERART_API_KEY": "${EVERART_API_KEY}"}, "enabled": true},
    "exa": {"command": "npx", "args": ["-y", "exa-mcp-server"], "env": {"EXA_API_KEY": "${EXA_API_KEY}"}, "enabled": true},
    "linear": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-linear"], "env": {"LINEAR_API_KEY": "${LINEAR_API_KEY}"}, "enabled": true},
    "sentry": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-sentry"], "env": {"SENTRY_AUTH_TOKEN": "${SENTRY_AUTH_TOKEN}", "SENTRY_ORG": "${SENTRY_ORG}"}, "enabled": true},
    "notion": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-notion"], "env": {"NOTION_API_KEY": "${NOTION_API_KEY}"}, "enabled": true},
    "figma": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-figma"], "env": {"FIGMA_ACCESS_TOKEN": "${FIGMA_ACCESS_TOKEN}"}, "enabled": true},
    "aws-kb-retrieval": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-aws-kb-retrieval"], "env": {"AWS_ACCESS_KEY_ID": "${AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY": "${AWS_SECRET_ACCESS_KEY}"}, "enabled": true},
    "gitlab": {"command": "npx", "args": ["-y", "@anthropic-ai/mcp-server-gitlab"], "env": {"GITLAB_TOKEN": "${GITLAB_TOKEN}"}, "enabled": true},
    "helixagent": {"command": "curl", "args": ["-s", "-X", "POST", "HELIX_URL_PLACEHOLDER/v1/mcp", "-H", "Content-Type: application/json"], "enabled": true},
    "helixagent-debate": {"command": "curl", "args": ["-s", "-X", "POST", "HELIX_URL_PLACEHOLDER/v1/debates", "-H", "Content-Type: application/json"], "enabled": true},
    "helixagent-rag": {"command": "curl", "args": ["-s", "-X", "POST", "HELIX_URL_PLACEHOLDER/v1/rag/search", "-H", "Content-Type: application/json"], "enabled": true},
    "helixagent-memory": {"command": "curl", "args": ["-s", "-X", "POST", "HELIX_URL_PLACEHOLDER/v1/memory", "-H", "Content-Type: application/json"], "enabled": true},
    "docker": {"command": "npx", "args": ["-y", "mcp-server-docker"], "enabled": true},
    "kubernetes": {"command": "npx", "args": ["-y", "mcp-server-kubernetes"], "env": {"KUBECONFIG": "${KUBECONFIG:-$HOME/.kube/config}"}, "enabled": true},
    "redis": {"command": "npx", "args": ["-y", "mcp-server-redis"], "env": {"REDIS_URL": "${REDIS_URL:-redis://localhost:6379}"}, "enabled": true},
    "mongodb": {"command": "npx", "args": ["-y", "mcp-server-mongodb"], "env": {"MONGODB_URI": "${MONGODB_URI:-mongodb://localhost:27017}"}, "enabled": true},
    "elasticsearch": {"command": "npx", "args": ["-y", "mcp-server-elasticsearch"], "env": {"ELASTICSEARCH_URL": "${ELASTICSEARCH_URL:-http://localhost:9200}"}, "enabled": true},
    "qdrant": {"command": "npx", "args": ["-y", "mcp-server-qdrant"], "env": {"QDRANT_URL": "${QDRANT_URL:-http://localhost:6333}"}, "enabled": true},
    "chroma": {"command": "npx", "args": ["-y", "mcp-server-chroma"], "env": {"CHROMA_URL": "${CHROMA_URL:-http://localhost:8001}"}, "enabled": true},
    "jira": {"command": "npx", "args": ["-y", "mcp-server-jira"], "env": {"JIRA_URL": "${JIRA_URL}", "JIRA_EMAIL": "${JIRA_EMAIL}", "JIRA_API_TOKEN": "${JIRA_API_TOKEN}"}, "enabled": true},
    "asana": {"command": "npx", "args": ["-y", "mcp-server-asana"], "env": {"ASANA_ACCESS_TOKEN": "${ASANA_ACCESS_TOKEN}"}, "enabled": true},
    "google-drive": {"command": "npx", "args": ["-y", "mcp-server-google-drive"], "env": {"GOOGLE_CREDENTIALS_PATH": "${GOOGLE_CREDENTIALS_PATH}"}, "enabled": true},
    "aws-s3": {"command": "npx", "args": ["-y", "mcp-server-s3"], "env": {"AWS_ACCESS_KEY_ID": "${AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY": "${AWS_SECRET_ACCESS_KEY}"}, "enabled": true},
    "datadog": {"command": "npx", "args": ["-y", "mcp-server-datadog"], "env": {"DD_API_KEY": "${DD_API_KEY}", "DD_APP_KEY": "${DD_APP_KEY}"}, "enabled": true}
  },
  "lsp": {
    "helixagent-lsp": {"command": "curl", "args": ["-X", "POST", "HELIX_URL_PLACEHOLDER/v1/lsp", "-H", "Authorization: Bearer HELIX_API_KEY_PLACEHOLDER"], "enabled": true}
  },
  "options": {
    "default_provider": "helixagent",
    "default_model": "helixagent-debate"
  }
}
CRUSH_EOF

    # Replace placeholders with actual values
    sed -i "s|HELIX_URL_PLACEHOLDER|${helix_url}|g" "$output_dir/crush.json"
    sed -i "s|HELIX_API_KEY_PLACEHOLDER|${helix_api_key}|g" "$output_dir/crush.json"

    log_success "Generated Crush config with 35+ MCPs: $output_dir"
}

# ============================================================================
# Generic Config Generator
# ============================================================================

generate_agent_config() {
    local agent_name="$1"
    local agent_info="${CLI_AGENTS[$agent_name]}"

    if [[ -z "$agent_info" ]]; then
        log_error "Unknown agent: $agent_name"
        return 1
    fi

    IFS='|' read -r language config_path config_file <<< "$agent_info"

    local output_dir="$CONFIG_OUTPUT_DIR/$agent_name"
    mkdir -p "$output_dir"

    # Determine config format
    local config_format="${config_file##*.}"
    local output_file="$output_dir/$config_file"

    case "$config_format" in
        json)
            generate_json_config "$agent_name" "$output_file"
            ;;
        yaml|yml)
            generate_yaml_config "$agent_name" "$output_file"
            ;;
        *)
            generate_json_config "$agent_name" "$output_file"
            ;;
    esac

    log_success "Generated config for $agent_name: $output_file"
}

# ============================================================================
# Installation Functions
# ============================================================================

backup_existing_config() {
    local target_path="$1"
    local agent_name="$2"

    if [[ -e "$target_path" ]]; then
        local backup_path="$BACKUP_DIR/$agent_name"
        mkdir -p "$backup_path"
        cp -r "$target_path" "$backup_path/"
        log_info "Backed up existing config: $target_path -> $backup_path"
    fi
}

install_agent_config() {
    local agent_name="$1"
    local agent_info="${CLI_AGENTS[$agent_name]}"

    if [[ -z "$agent_info" ]]; then
        log_error "Unknown agent: $agent_name"
        return 1
    fi

    IFS='|' read -r language config_path config_file <<< "$agent_info"

    # Expand home directory
    local target_dir="${config_path/#\~/$HOME}"
    local source_dir="$CONFIG_OUTPUT_DIR/$agent_name"

    if [[ ! -d "$source_dir" ]]; then
        log_error "Config not generated for $agent_name"
        return 1
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[DRY-RUN] Would install $source_dir -> $target_dir"
        return 0
    fi

    # Backup existing config
    backup_existing_config "$target_dir" "$agent_name"

    # Create target directory
    mkdir -p "$target_dir"

    # Copy config files
    cp -r "$source_dir"/* "$target_dir/"

    # Create plugins directory
    mkdir -p "$HOME/.helix-plugins/$agent_name"

    # Create logs directory
    mkdir -p "$HOME/.helix-logs"

    log_success "Installed config for $agent_name: $target_dir"
}

# ============================================================================
# Main Execution
# ============================================================================

main() {
    log_info "=============================================="
    log_info "HelixAgent CLI Agent Configuration Generator"
    log_info "=============================================="
    log_info "HelixAgent URL: $HELIX_AGENT_URL"
    log_info "LLMsVerifier URL: $LLMS_VERIFIER_URL"
    log_info "Output directory: $CONFIG_OUTPUT_DIR"

    if [[ "$DRY_RUN" == "true" ]]; then
        log_warning "Running in DRY-RUN mode - no changes will be made"
    fi

    echo ""

    # Generate agent-specific configs first
    log_info "Generating agent-specific configurations..."
    generate_claude_code_config
    generate_aider_config
    generate_cline_config
    generate_opencode_config
    generate_crush_config

    # Generate generic configs for remaining agents
    log_info "Generating generic configurations..."

    local agents_to_process=()

    if [[ -n "$SPECIFIC_AGENT" ]]; then
        agents_to_process=("$SPECIFIC_AGENT")
    else
        agents_to_process=("${!CLI_AGENTS[@]}")
    fi

    local total=${#agents_to_process[@]}
    local current=0
    local success=0
    local failed=0

    for agent in "${agents_to_process[@]}"; do
        current=$((current + 1))

        # Skip agents with specific generators
        if [[ "$agent" == "claude_code" ]] || [[ "$agent" == "aider" ]] || \
           [[ "$agent" == "cline" ]] || [[ "$agent" == "opencode" ]] || \
           [[ "$agent" == "crush" ]]; then
            continue
        fi

        if generate_agent_config "$agent"; then
            success=$((success + 1))
        else
            failed=$((failed + 1))
        fi
    done

    echo ""
    log_info "=============================================="
    log_info "Configuration Generation Summary"
    log_info "=============================================="
    log_info "Total agents: $total"
    log_success "Generated: $((success + 5))"  # +5 for specific generators (claude_code, aider, cline, opencode, crush)
    if [[ $failed -gt 0 ]]; then
        log_error "Failed: $failed"
    fi

    # Install configs if requested
    if [[ "$INSTALL" == "true" ]]; then
        echo ""
        log_info "=============================================="
        log_info "Installing Configurations"
        log_info "=============================================="

        local install_success=0
        local install_failed=0

        for agent in "${agents_to_process[@]}"; do
            if install_agent_config "$agent"; then
                install_success=$((install_success + 1))
            else
                install_failed=$((install_failed + 1))
            fi
        done

        echo ""
        log_info "Installation Summary"
        log_success "Installed: $install_success"
        if [[ $install_failed -gt 0 ]]; then
            log_error "Failed: $install_failed"
        fi

        if [[ -d "$BACKUP_DIR" ]] && [[ "$(ls -A $BACKUP_DIR 2>/dev/null)" ]]; then
            log_info "Backups saved to: $BACKUP_DIR"
        fi
    fi

    echo ""
    log_success "Configuration generation complete!"
    log_info "Generated configs: $CONFIG_OUTPUT_DIR"

    if [[ "$INSTALL" != "true" ]]; then
        log_info "Run with --install to install configs to system locations"
    fi
}

main "$@"
