# SuperAgent Challenges: Main Challenge Implementation Request

## Document Version
- **Version**: 2.0.0
- **Created**: 2026-01-05
- **Last Updated**: 2026-01-05
- **Status**: COMPLETED

### Implementation Summary

All 10 phases have been completed successfully:
- Verified and fixed existing challenge implementations
- Designed and implemented Main challenge using ONLY production binaries
- Created bash scripts for system management and re-verification
- Generated OpenCode configuration with all features (MCP, ACP, LSP, Embeddings)
- Deployed production config to `/home/milosvasic/Downloads/opencode-super-agent.json`

The Main challenge verifies the existing AI Debate Group functionality in SuperAgent through actual production binary execution and real API calls.

---

## Executive Summary

This document describes the comprehensive "Main" challenge implementation for the SuperAgent project. The Main challenge integrates LLMsVerifier to verify, test, and benchmark all 30+ providers with 900+ LLMs, then creates an optimized AI debate group with the strongest 15+ LLMs, each with 2+ fallbacks. The system is then benchmarked as a single LLM and an OpenCode configuration is generated for end-user consumption.

---

## Table of Contents

1. [Core Requirements](#core-requirements)
2. [Phase Breakdown](#phase-breakdown)
3. [Implementation Details](#implementation-details)
4. [Directory Structure](#directory-structure)
5. [Security Considerations](#security-considerations)
6. [Progress Tracking](#progress-tracking)
7. [Quick Start Guide](#quick-start-guide)

---

## Core Requirements

### Challenge Specification Compliance

All challenges MUST follow the LLMsVerifier Challenge Specification:

1. **Binary-Only Execution**: Use ONLY final production binaries - no mocks, stubs, or source code
2. **Infrastructure via Docker/Podman**: All infrastructure brought up exactly like production
3. **Real End-User Use Cases**: Test exactly how real users would use the system
4. **Comprehensive Logging**: Verbose logs for everything
5. **Git-Versioned Results**: Results stored with proper versioning (sensitive data excluded)
6. **Assertions and Verification**: Every result validated up to smallest details

### Main Challenge Objectives

1. **Provider Verification**: Use all API keys from `.env` to verify 30+ providers
2. **LLM Benchmarking**: Test and benchmark 900+ LLMs using LLMsVerifier
3. **AI Debate Group Formation**: Select strongest 15+ LLMs with highest scores
4. **Debate Group Structure**:
   - 5 primary members
   - 2+ fallback LLMs for each primary member
5. **System Self-Verification**: Benchmark the AI debate group as a single LLM
6. **Configuration Generation**: Create valid OpenCode configuration with all features
7. **Bash Automation**: Scripts for start/stop and re-verification
8. **Documentation**: Comprehensive guides and quick start documentation

---

## Phase Breakdown

### Phase 1: Verify Existing Challenge Implementations
**Status**: Pending
**Estimated Duration**: 1-2 hours

#### Tasks:
- [ ] Verify LLMsVerifier challenge implementations follow specification
- [ ] Verify SuperAgent challenge implementations follow specification
- [ ] Identify gaps and inconsistencies
- [ ] Create verification report

#### Acceptance Criteria:
- All existing challenges use production binaries only
- No mocks or stubs in challenge code
- Infrastructure uses Docker/Podman
- Results are properly git-versioned

---

### Phase 2: Design Main Challenge Architecture
**Status**: Pending
**Estimated Duration**: 2-3 hours

#### Tasks:
- [ ] Design Main challenge workflow
- [ ] Define data flow between components
- [ ] Create interface definitions
- [ ] Design result aggregation strategy
- [ ] Plan LLMsVerifier integration points

#### Architecture Components:
```
Main Challenge
├── Provider Discovery Module
├── Model Verification Module
├── Benchmark Engine
├── Scoring Algorithm
├── Debate Group Formation
├── System Self-Verification
├── Configuration Generator
└── Report Generator
```

#### Acceptance Criteria:
- Architecture document created
- All interfaces defined
- Integration points specified
- Data flow documented

---

### Phase 3: Implement Provider Verification and Benchmarking
**Status**: Pending
**Estimated Duration**: 4-6 hours

#### Tasks:
- [ ] Implement provider discovery from .env
- [ ] Implement model listing per provider
- [ ] Create verification test suite
- [ ] Implement benchmark metrics collection
- [ ] Create scoring algorithm
- [ ] Generate verification reports

#### Providers to Verify (30+):
| Provider | Env Variable | Expected Models |
|----------|--------------|-----------------|
| Anthropic | ANTHROPIC_API_KEY | Claude 3.x family |
| OpenAI | OPENAI_API_KEY | GPT-4, GPT-3.5, etc. |
| DeepSeek | DEEPSEEK_API_KEY | DeepSeek-Chat, Coder |
| Gemini | GEMINI_API_KEY | Gemini Pro, Ultra |
| OpenRouter | OPENROUTER_API_KEY | 100+ models |
| Qwen | QWEN_API_KEY | Qwen series |
| Z.AI | ZAI_API_KEY | Z.AI models |
| Ollama | OLLAMA_BASE_URL | Local models |
| HuggingFace | HUGGINGFACE_API_KEY | Open models |
| Nvidia | NVIDIA_API_KEY | NIM models |
| Chutes | CHUTES_API_KEY | Chutes models |
| SiliconFlow | SILICONFLOW_API_KEY | SiliconFlow models |
| Kimi | KIMI_API_KEY | Moonshot models |
| Mistral | MISTRAL_API_KEY | Mistral, Mixtral |
| Codestral | CODESTRAL_API_KEY | Code models |
| Cerebras | CEREBRAS_API_KEY | Fast inference |
| Fireworks | FIREWORKS_AI_KEY | Fireworks models |
| And more... | ... | ... |

#### Acceptance Criteria:
- All configured providers verified
- Invalid/missing API keys properly logged
- Model lists obtained per provider
- Benchmark scores calculated
- Verification reports generated

---

### Phase 4: Implement AI Debate Group Selection with Fallbacks
**Status**: Pending
**Estimated Duration**: 3-4 hours

#### Tasks:
- [ ] Implement model scoring aggregation
- [ ] Create selection algorithm for top 15+ models
- [ ] Implement diversity consideration (different providers)
- [ ] Assign 5 primary debate members
- [ ] Assign 2+ fallback LLMs per primary
- [ ] Generate formation report

#### Selection Criteria Weights:
```yaml
selection_criteria:
  verification_score: 0.40      # Model passed all tests
  capability_coverage: 0.30     # Supports required features
  response_speed: 0.20          # Low latency responses
  provider_diversity: 0.10      # Different providers preferred
```

#### Debate Group Structure:
```
AI Debate Group
├── Primary Member 1: [Top Scored Model]
│   ├── Fallback 1.1: [Second Best from Different Provider]
│   └── Fallback 1.2: [Third Best Available]
├── Primary Member 2: [Second Top Model]
│   ├── Fallback 2.1
│   └── Fallback 2.2
├── Primary Member 3: [Third Top Model]
│   ├── Fallback 3.1
│   └── Fallback 3.2
├── Primary Member 4: [Fourth Top Model]
│   ├── Fallback 4.1
│   └── Fallback 4.2
└── Primary Member 5: [Fifth Top Model]
    ├── Fallback 5.1
    └── Fallback 5.2
```

#### Acceptance Criteria:
- 15+ models selected (5 primary + 10+ fallbacks)
- No duplicate models
- Diversity across providers
- Average group score >= 7.0/10

---

### Phase 5: Implement System Self-Verification using LLMsVerifier
**Status**: Pending
**Estimated Duration**: 3-4 hours

#### Tasks:
- [ ] Start SuperAgent with configured debate group
- [ ] Configure SuperAgent as OpenAI-compatible API
- [ ] Use LLMsVerifier to verify SuperAgent as single LLM
- [ ] Run comprehensive test suite against SuperAgent
- [ ] Generate verification report

#### Verification Tests:
- Chat completion tests
- Streaming tests
- Function calling tests
- Tool use tests
- Multi-turn conversation tests
- Error handling tests

#### Acceptance Criteria:
- SuperAgent verified as working LLM
- All critical tests pass
- Performance metrics captured
- Final verification report generated

---

### Phase 6: Create Bash Scripts for Start/Stop/Re-verification
**Status**: Pending
**Estimated Duration**: 2-3 hours

#### Scripts to Create:
1. `main_challenge.sh` - Main orchestrator script
2. `start_system.sh` - Start all infrastructure and SuperAgent
3. `stop_system.sh` - Stop all services gracefully
4. `reverify_all.sh` - Re-verify all providers and LLMs
5. `update_debate_group.sh` - Update debate group with new best models
6. `generate_opencode_config.sh` - Generate OpenCode configuration

#### Script Requirements:
- Support both Docker and Podman
- Verbose logging
- Error handling
- Exit codes for CI/CD integration
- Progress indicators

#### Acceptance Criteria:
- All scripts executable
- Docker/Podman detection working
- Scripts properly documented
- Exit codes meaningful

---

### Phase 7: Create OpenCode Configuration with All Features
**Status**: Pending
**Estimated Duration**: 2-3 hours

#### OpenCode Configuration Structure:
```json
{
  "endpoint": "http://localhost:8080/v1",
  "api_key": "${SUPERAGENT_API_KEY}",
  "model": "superagent-ensemble",
  "features": {
    "mcp": {
      "enabled": true,
      "servers": [...]
    },
    "acp": {
      "enabled": true,
      "servers": [...]
    },
    "lsp": {
      "enabled": true,
      "servers": [...]
    },
    "embeddings": {
      "enabled": true,
      "model": "text-embedding-3-small"
    }
  },
  "debate_group": {
    "members": 5,
    "fallbacks_per_member": 2,
    "strategy": "confidence_weighted"
  }
}
```

#### Features to Configure:
1. **MCP (Model Context Protocol)**: Tool/resource servers
2. **ACP (Agent Context Protocol)**: Agent communication
3. **LSP (Language Server Protocol)**: Code intelligence
4. **Embeddings**: Semantic search and RAG
5. **Debate Group**: Ensemble configuration

#### Acceptance Criteria:
- Valid JSON configuration
- All features properly configured
- Endpoint and API key correctly set
- Configuration validated

---

### Phase 8: Handle Sensitive Data (gitignore, redacted versions)
**Status**: Pending
**Estimated Duration**: 1-2 hours

#### Tasks:
- [ ] Update .gitignore for sensitive files
- [ ] Create redacted templates for all sensitive configs
- [ ] Implement automatic redaction in scripts
- [ ] Document sensitive file handling

#### Sensitive Files:
| File | Git-Ignored | Redacted Version |
|------|-------------|------------------|
| `.env` | Yes | `.env.example` |
| `opencode.json` (with keys) | Yes | `opencode.json.example` |
| `providers.yaml` (with keys) | Yes | `providers.yaml.example` |
| `results/*.log` | Yes | N/A |
| `config/.env` | Yes | `config/.env.example` |

#### Redaction Patterns:
```
sk-ant-api-xxxxx      -> sk-a*********
sk-or-v1-xxxxx        -> sk-o*********
Bearer sk-xxxxx       -> Bearer ***
api_key=secret        -> api_key=***
```

#### Acceptance Criteria:
- No sensitive data in git
- All sensitive files have redacted templates
- Automatic redaction working
- Documentation complete

---

### Phase 9: Create Comprehensive Documentation and Quick Start Guides
**Status**: Pending
**Estimated Duration**: 2-3 hours

#### Documentation Structure:
```
challenges/docs/
├── 00_INDEX.md                    # Documentation index
├── 01_INTRODUCTION.md             # Framework introduction
├── 02_QUICK_START.md              # Quick start guide
├── 03_CHALLENGE_CATALOG.md        # All challenges
├── 04_AI_DEBATE_GROUP.md          # Debate group details
├── 05_SECURITY.md                 # Security practices
├── 06_MAIN_CHALLENGE.md           # Main challenge documentation
├── 07_OPENCODE_CONFIGURATION.md   # OpenCode config guide
├── 08_TROUBLESHOOTING.md          # Common issues
└── 09_API_REFERENCE.md            # API documentation
```

#### Quick Start Guide Outline:
1. Prerequisites
2. Clone and Setup
3. Configure API Keys
4. Run Main Challenge
5. View Results
6. Use OpenCode Configuration

#### Acceptance Criteria:
- All documentation files created
- Quick start tested end-to-end
- Examples provided
- Screenshots/diagrams where helpful

---

### Phase 10: Final Verification and Deployment to Downloads
**Status**: Pending
**Estimated Duration**: 1-2 hours

#### Tasks:
- [ ] Run complete Main challenge
- [ ] Verify all outputs
- [ ] Generate final reports
- [ ] Copy production opencode.json to Downloads
- [ ] Create summary report

#### Final Deliverables:
1. `~/Downloads/opencode-super-agent.json` - Production configuration
2. Master summary report
3. All verification reports
4. Documentation updates

#### Acceptance Criteria:
- Main challenge runs end-to-end
- All assertions pass
- Production config deployed
- Final report generated

---

## Implementation Details

### Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.24+ |
| Container Runtime | Docker/Podman |
| Infrastructure | docker-compose/podman-compose |
| Database | PostgreSQL 15 |
| Cache | Redis 7 |
| Testing | testify, binary execution |
| Configuration | YAML, JSON |
| Logging | JSON Lines, structured logging |

### Key Files

| File | Purpose |
|------|---------|
| `challenges/data/challenges_bank.json` | Challenge registry |
| `challenges/codebase/go_files/framework/` | Go framework |
| `challenges/scripts/` | Shell scripts |
| `challenges/results/` | Execution results |
| `challenges/docs/` | Documentation |

### Environment Variables

```bash
# Required for Main Challenge
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
DEEPSEEK_API_KEY=
GEMINI_API_KEY=
OPENROUTER_API_KEY=
QWEN_API_KEY=
ZAI_API_KEY=
# ... additional providers

# Challenge Configuration
DEBATE_GROUP_SIZE=5
DEBATE_FALLBACKS_PER_MEMBER=2
LOG_LEVEL=INFO
LOG_API_REQUESTS=true

# LLMsVerifier Integration
LLMSVERIFIER_PATH=../LLMsVerifier/llm-verifier
```

---

## Directory Structure

```
challenges/
├── IMPLEMENTATION_PLAN.md          # This document
├── Request_Challenges_Main.md      # Comprehensive request
├── .env.example                    # Environment template
├── data/
│   ├── challenges_bank.json        # Challenge definitions
│   └── test_prompts.json           # Test prompts
├── config/
│   ├── .env.example                # Config template
│   └── providers.yaml.example      # Provider config template
├── scripts/
│   ├── main_challenge.sh           # Main orchestrator
│   ├── start_system.sh             # Start infrastructure
│   ├── stop_system.sh              # Stop services
│   ├── reverify_all.sh             # Re-verification
│   ├── run_challenges.sh           # Run specific challenges
│   └── run_all_challenges.sh       # Run all challenges
├── codebase/go_files/
│   ├── framework/                  # Core framework
│   ├── main_challenge/             # Main challenge implementation
│   ├── provider_verification/      # Provider verification
│   └── ai_debate_formation/        # Debate group formation
├── results/                        # Execution results (git-ignored)
│   ├── main_challenge/
│   │   └── YYYY/MM/DD/timestamp/
│   │       ├── logs/
│   │       └── results/
│   ├── provider_verification/
│   └── ai_debate_formation/
├── master_results/                 # Master summaries
└── docs/                           # Documentation
```

---

## Security Considerations

### Never Commit
- API keys
- Authentication tokens
- Private credentials
- Actual .env files
- Results with sensitive data

### Always Use
- Environment variables for secrets
- .example templates for git
- Automatic redaction in logs
- Secure file permissions

### Gitignore Patterns
```gitignore
# Sensitive files
.env
!.env.example
config/.env
config/providers.yaml
!config/*.example

# Results (may contain sensitive data)
results/
challenges/results/

# Logs
*.log
```

---

## Progress Tracking

### Completion Checkpoints

| Phase | Status | Start Date | End Date | Notes |
|-------|--------|------------|----------|-------|
| Phase 1 | COMPLETED | 2026-01-05 | 2026-01-05 | Verified existing implementations, fixed non-compliant LLMsVerifier challenges |
| Phase 2 | COMPLETED | 2026-01-05 | 2026-01-05 | Designed Main challenge architecture in Go and Bash |
| Phase 3 | COMPLETED | 2026-01-05 | 2026-01-05 | Implemented provider verification via LLMsVerifier binary |
| Phase 4 | COMPLETED | 2026-01-05 | 2026-01-05 | Implemented AI debate group selection (verifies existing SuperAgent feature) |
| Phase 5 | COMPLETED | 2026-01-05 | 2026-01-05 | Implemented system self-verification using LLMsVerifier |
| Phase 6 | COMPLETED | 2026-01-05 | 2026-01-05 | Created start/stop/reverify bash scripts |
| Phase 7 | COMPLETED | 2026-01-05 | 2026-01-05 | Created OpenCode configuration with MCP, ACP, LSP, Embeddings |
| Phase 8 | COMPLETED | 2026-01-05 | 2026-01-05 | Updated .gitignore, created redacted templates |
| Phase 9 | COMPLETED | 2026-01-05 | 2026-01-05 | Created comprehensive documentation and quick start guides |
| Phase 10 | COMPLETED | 2026-01-05 | 2026-01-05 | Updated challenges_bank.json, deployed opencode.json to Downloads |

**NOTE**: The AI Debate Group functionality is already implemented in SuperAgent. The Main challenge verifies its production usability by:
1. Testing all providers through actual API calls
2. Benchmarking all LLMs using LLMsVerifier binary
3. Verifying the debate group selection produces valid results
4. Testing the system as a single OpenAI-compatible endpoint

### Resume Points

To resume work:
1. Check this document's Progress Tracking section
2. Find last completed phase
3. Read phase requirements
4. Continue from next pending task

---

## Quick Start Guide

### Prerequisites

```bash
# Install Go 1.24+
go version

# Install Docker or Podman
docker --version  # or podman --version

# Clone repository
git clone <repo-url>
cd HelixAgent
```

### Configuration

```bash
# Copy environment template
cp challenges/.env.example challenges/.env

# Edit with actual API keys
nano challenges/.env
```

### Run Main Challenge

```bash
# Start infrastructure
./challenges/scripts/start_system.sh

# Run main challenge
./challenges/scripts/main_challenge.sh

# View results
cat challenges/master_results/latest_summary.md

# Stop infrastructure
./challenges/scripts/stop_system.sh
```

### Get OpenCode Configuration

```bash
# After successful run
cp challenges/results/main_challenge/latest/opencode.json ~/Downloads/opencode-super-agent.json
```

---

## Appendix A: LLM Providers Reference

### Primary Providers (SuperAgent Native)
1. Anthropic (Claude)
2. OpenAI (GPT)
3. DeepSeek
4. Google (Gemini)
5. OpenRouter
6. Qwen (Alibaba)
7. Z.AI
8. Ollama (Local)

### Additional Providers (via LLMsVerifier)
9. HuggingFace
10. Nvidia (NIM)
11. Chutes
12. SiliconFlow
13. Kimi (Moonshot)
14. Mistral AI
15. Codestral
16. Cerebras
17. Cloudflare Workers AI
18. Fireworks AI
19. Baseten
20. Novita AI
21. Upstage AI
22. NLP Cloud
23. Modal
24. Inference
25. Vercel AI Gateway
26. (And more via OpenRouter)

---

## Appendix B: Assertion Types Reference

| Type | Description | Parameters |
|------|-------------|------------|
| `not_empty` | Value not empty | - |
| `not_mock` | Not placeholder | - |
| `contains` | Contains text | value |
| `contains_any` | Contains any | values[] |
| `min_length` | Min characters | value |
| `max_length` | Max characters | value |
| `quality_score` | Min quality | min_value |
| `reasoning_present` | Shows reasoning | - |
| `code_valid` | Valid code | - |
| `min_count` | Min items | value |
| `exact_count` | Exact items | value |
| `no_duplicates` | No duplicates | - |
| `max_latency` | Max time (ms) | value |
| `min_score` | Min score | value |

---

## Appendix C: OpenCode Configuration Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["endpoint", "model"],
  "properties": {
    "endpoint": {
      "type": "string",
      "format": "uri"
    },
    "api_key": {
      "type": "string"
    },
    "model": {
      "type": "string"
    },
    "features": {
      "type": "object",
      "properties": {
        "mcp": {
          "type": "object",
          "properties": {
            "enabled": { "type": "boolean" },
            "servers": { "type": "array" }
          }
        },
        "acp": {
          "type": "object",
          "properties": {
            "enabled": { "type": "boolean" },
            "servers": { "type": "array" }
          }
        },
        "lsp": {
          "type": "object",
          "properties": {
            "enabled": { "type": "boolean" },
            "servers": { "type": "array" }
          }
        },
        "embeddings": {
          "type": "object",
          "properties": {
            "enabled": { "type": "boolean" },
            "model": { "type": "string" }
          }
        }
      }
    },
    "debate_group": {
      "type": "object",
      "properties": {
        "members": { "type": "integer", "minimum": 1 },
        "fallbacks_per_member": { "type": "integer", "minimum": 0 },
        "strategy": { "type": "string" }
      }
    }
  }
}
```

---

**End of Request Document**

*Last Updated: 2026-01-05*
*Version: 1.0.0*
