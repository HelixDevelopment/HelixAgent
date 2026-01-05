# Main SuperAgent Challenge - Documentation

## Overview

The **Main** challenge is the comprehensive challenge that:
1. Verifies all 30+ LLM providers using API keys from `.env`
2. Tests and benchmarks 900+ LLMs using LLMsVerifier
3. Selects the strongest 15+ LLMs with highest scores
4. Forms an AI debate group with 5 primary members + 2 fallbacks each
5. Verifies the complete system as a single LLM using LLMsVerifier
6. Generates OpenCode configuration with all features (MCP, ACP, LSP, Embeddings)

**CRITICAL**: This challenge uses ONLY production binaries - NO source code execution!

---

## Quick Start

### Prerequisites

```bash
# 1. Ensure binaries are built
make build                    # SuperAgent binary
cd LLMsVerifier && make build # LLMsVerifier binary

# 2. Configure API keys
cp challenges/.env.example challenges/.env
nano challenges/.env  # Add your API keys

# 3. Install container runtime
docker --version   # or podman --version
```

### Run the Main Challenge

```bash
# Start the Main challenge
./challenges/scripts/main_challenge.sh

# View results
cat challenges/master_results/latest_summary.md

# Your OpenCode config is ready at:
cat /home/milosvasic/Downloads/opencode-super-agent.json
```

### Options

```bash
# Verbose mode
./challenges/scripts/main_challenge.sh --verbose

# Skip infrastructure (if already running)
./challenges/scripts/main_challenge.sh --skip-infra

# Skip final system verification
./challenges/scripts/main_challenge.sh --skip-verify

# Dry run (show commands without executing)
./challenges/scripts/main_challenge.sh --dry-run
```

---

## Challenge Phases

### Phase 1: Infrastructure Setup
- Detects container runtime (Docker or Podman)
- Starts infrastructure services (PostgreSQL, Redis, etc.)
- Waits for services to be ready

### Phase 2: Provider Verification
- Loads API keys from environment
- Uses LLMsVerifier binary to verify each provider
- Logs invalid or missing API keys
- Outputs: `providers_verified.json`

### Phase 3: Model Benchmarking
- Discovers all models from verified providers
- Runs verification tests on each model
- Calculates scores based on capabilities and performance
- Outputs: `models_scored.json`

### Phase 4: AI Debate Group Formation
- Selects top 15+ models by score
- Assigns 5 primary members
- Assigns 2 fallbacks for each primary
- Ensures provider diversity
- Outputs: `debate_group.json`, `member_assignments.json`

### Phase 5: System Verification
- Starts SuperAgent with debate group configuration
- Uses LLMsVerifier to verify SuperAgent as OpenAI-compatible API
- Runs comprehensive test suite
- Outputs: `system_verification.json`

### Phase 6: OpenCode Configuration
- Generates OpenCode configuration with all features:
  - MCP servers (filesystem, github, memory)
  - ACP servers
  - LSP servers (gopls, typescript, python)
  - Embeddings (text-embedding-3-small)
- Creates redacted version for git
- Copies to Downloads
- Outputs: `opencode.json`, `opencode.json.example`

### Phase 7: Final Report
- Generates master summary
- Creates symlink to latest summary
- Shows completion status

---

## Scripts Reference

| Script | Purpose |
|--------|---------|
| `main_challenge.sh` | Main orchestrator - runs all phases |
| `start_system.sh` | Start infrastructure and SuperAgent |
| `stop_system.sh` | Stop all services gracefully |
| `reverify_all.sh` | Re-verify all providers and update debate group |
| `run_challenges.sh` | Run specific challenges from bank |
| `run_all_challenges.sh` | Run all challenges in sequence |

---

## Output Files

```
challenges/results/main_challenge/YYYY/MM/DD/timestamp/
├── logs/
│   ├── main_challenge.log      # Main execution log
│   ├── provider_verification.log
│   ├── model_benchmark.log
│   ├── debate_formation.log
│   ├── system_verification.log
│   └── commands.log            # All binary commands executed
└── results/
    ├── providers_verified.json
    ├── models_scored.json
    ├── debate_group.json
    ├── member_assignments.json
    ├── system_verification.json
    ├── opencode.json            # WITH API keys (git-ignored)
    └── opencode.json.example    # REDACTED for git
```

---

## Assertions

The Main challenge validates:

| Assertion | Description |
|-----------|-------------|
| `min_count:providers:5` | At least 5 providers verified |
| `min_count:models:15` | At least 15 models scored |
| `exact_count:primary_members:5` | Exactly 5 primary debate members |
| `exact_count:fallbacks:2` | Exactly 2 fallbacks per primary |
| `min_score:7.0` | Average group score >= 7.0 |
| `system_verified:true` | System passes all verification tests |
| `file_exists:opencode.json` | Config copied to Downloads |

---

## Metrics

| Metric | Description |
|--------|-------------|
| `providers_total` | Total providers attempted |
| `providers_verified` | Successfully verified providers |
| `models_total` | Total models discovered |
| `models_verified` | Successfully verified models |
| `top_15_average_score` | Average score of top 15 models |
| `debate_group_average_score` | Average score of debate group |
| `total_duration_seconds` | Total challenge duration |

---

## Debate Group Structure

```
AI Debate Group (15+ models total)
├── Primary 1: [Highest Scored Model]
│   ├── Fallback 1.1: [2nd Best from Different Provider]
│   └── Fallback 1.2: [3rd Best Available]
├── Primary 2: [2nd Top Model]
│   ├── Fallback 2.1
│   └── Fallback 2.2
├── Primary 3: [3rd Top Model]
│   ├── Fallback 3.1
│   └── Fallback 3.2
├── Primary 4: [4th Top Model]
│   ├── Fallback 4.1
│   └── Fallback 4.2
└── Primary 5: [5th Top Model]
    ├── Fallback 5.1
    └── Fallback 5.2
```

### Selection Criteria

| Criterion | Weight |
|-----------|--------|
| Verification Score | 40% |
| Capability Coverage | 30% |
| Response Speed | 20% |
| Provider Diversity | 10% |

---

## OpenCode Configuration

The generated `opencode.json` includes:

```json
{
  "endpoint": "http://localhost:8080/v1",
  "api_key": "${SUPERAGENT_API_KEY}",
  "model": "superagent-ensemble",
  "features": {
    "mcp": {
      "enabled": true,
      "servers": ["filesystem", "github", "memory"]
    },
    "acp": {
      "enabled": true,
      "servers": []
    },
    "lsp": {
      "enabled": true,
      "servers": ["gopls", "typescript-language-server", "pylsp"]
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

---

## Security

### Git-Ignored Files
- `opencode.json` (contains API keys)
- All files in `results/` directory
- `*.log` files
- `.env` files

### Git-Versioned Files
- `opencode.json.example` (redacted)
- `.env.example` (template)
- `master_summary.md` (reports)

### Redaction Patterns
```
sk-ant-api-xxxxx      -> sk-a*********
sk-or-v1-xxxxx        -> sk-o*********
Bearer sk-xxxxx       -> Bearer ***
```

---

## Troubleshooting

### Binary Not Found
```bash
# Build SuperAgent
make build

# Build LLMsVerifier
cd LLMsVerifier && make build
```

### No API Keys
```bash
# Check environment
grep "_API_KEY" .env

# Reload environment
source .env
```

### Container Runtime Issues
```bash
# Check Docker
docker ps

# Check Podman
podman ps

# Start Podman socket
systemctl --user enable --now podman.socket
```

### Challenge Failed
1. Check logs: `challenges/results/main_challenge/.../logs/`
2. Check binary output: `binary_output.log`
3. Check commands: `commands.log`

---

## Re-Verification

To update the debate group with new models:

```bash
# Full re-verification
./challenges/scripts/reverify_all.sh

# Quick mode (sample models only)
./challenges/scripts/reverify_all.sh --quick

# Providers only
./challenges/scripts/reverify_all.sh --providers-only
```

---

*Last Updated: 2026-01-05*
