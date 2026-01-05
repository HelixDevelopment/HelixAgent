# SuperAgent Challenges System

A comprehensive challenge framework for testing, verifying, and validating LLM providers, AI debate groups, and API quality.

## Key Concepts

### SuperAgent as Virtual LLM Provider

SuperAgent presents itself as a **single LLM provider** with **ONE virtual model** - the AI Debate Ensemble. The underlying implementation leverages multiple top-performing LLMs through consensus-driven voting.

### Real Data Only - No Stubs

**CRITICAL**: ALL verification data comes from REAL API calls. No hardcoded scores, no sample data, no cached demonstrations.

### Auto-Start Infrastructure

**ALL infrastructure starts automatically** when needed:
- SuperAgent binary is built if not present
- SuperAgent server auto-starts if not running
- Docker/Podman containers start automatically

## Overview

The SuperAgent Challenges System provides:

- **Automated Provider Verification**: Test and score all configured LLM providers with REAL API calls
- **AI Debate Group Formation**: Create optimized groups of top-performing verified models
- **API Quality Testing**: Validate response quality with comprehensive assertions
- **Auto-Start Infrastructure**: SuperAgent and containers start automatically when needed
- **Dual Container Runtime**: Supports both Docker and Podman
- **Comprehensive Logging**: Full audit trail of all API communications
- **Execution Reports**: Detailed reports and master summaries with history tracking

## Quick Start

```bash
# 1. Configure API keys in project root .env
cp .env.example .env
nano .env  # Add your API keys + JWT_SECRET

# 2. Build SuperAgent (optional - auto-builds if needed)
make build

# 3. Run all 39 challenges (auto-starts everything)
./challenges/scripts/run_all_challenges.sh

# 4. View results
cat challenges/master_results/latest_summary.md
```

## All 39 Challenges

The system includes 39 comprehensive challenges:

| Category | Challenges | Count |
|----------|------------|-------|
| Infrastructure | health_monitoring, caching_layer, database_operations, configuration_loading, plugin_system, session_management, graceful_shutdown | 7 |
| Providers | provider_claude, provider_deepseek, provider_gemini, provider_ollama, provider_openrouter, provider_qwen, provider_zai | 7 |
| Protocols | mcp_protocol, lsp_protocol, acp_protocol | 3 |
| Security | authentication, rate_limiting, input_validation | 3 |
| Core | provider_verification, ensemble_voting, ai_debate_formation, ai_debate_workflow, embeddings_service, streaming_responses, model_metadata, api_quality_test | 8 |
| Cloud | cloud_aws_bedrock, cloud_gcp_vertex, cloud_azure_openai | 3 |
| Optimization | optimization_semantic_cache, optimization_structured_output | 2 |
| Integration | cognee_integration | 1 |
| Resilience | circuit_breaker, error_handling, concurrent_access | 3 |
| API | openai_compatibility, grpc_api | 2 |

**Total: 39 challenges - ALL must pass**

## Directory Structure

```
challenges/
├── README.md                   # This file
├── docs/                       # Documentation
│   ├── 00_INDEX.md             # Documentation index
│   ├── 01_INTRODUCTION.md      # Framework introduction
│   └── ...                     # Additional docs
├── config/                     # Configuration files
│   ├── .env.example            # Environment template (git-versioned)
│   └── providers.yaml.example  # Provider config template
├── data/                       # Challenge definitions
│   ├── challenges_bank.json    # Challenge registry
│   └── test_prompts.json       # Test prompts by category
├── scripts/                    # Shell scripts
│   ├── common.sh               # Shared functions
│   ├── run_challenges.sh       # Run specific challenges
│   ├── run_all_challenges.sh   # Run all challenges
│   ├── verify_config.sh        # Validate configuration
│   ├── generate_report.sh      # Generate reports
│   └── cleanup_results.sh      # Clean old results
├── codebase/go_files/          # Go implementation
│   ├── framework/              # Core framework (61 tests)
│   │   ├── types.go            # Core types
│   │   ├── interfaces.go       # Interface definitions
│   │   ├── registry.go         # Challenge registry
│   │   ├── runner.go           # Challenge execution
│   │   ├── assertions.go       # Assertion engine (15+ evaluators)
│   │   ├── env.go              # Environment handling
│   │   ├── logger.go           # Logging system
│   │   ├── reporter.go         # Report generation
│   │   └── integration_test.go # Integration tests
│   ├── provider_verification/  # Provider verification challenge
│   ├── ai_debate_formation/    # Debate group formation challenge
│   └── api_quality_test/       # API quality testing challenge
└── results/                    # Execution results (git-ignored)
    └── <timestamp>/            # Per-run results
```

## Available Challenges

| Challenge | Description | Category |
|-----------|-------------|----------|
| `provider_verification` | Verify all LLM providers and score models | Core |
| `ai_debate_formation` | Form AI debate group from top models | Core |
| `api_quality_test` | Test API quality with assertions | Validation |

## Framework Components

### Core Framework (`codebase/go_files/framework/`)

| File | Purpose | Tests |
|------|---------|-------|
| `types.go` | Core types for challenges, results, assertions | - |
| `interfaces.go` | Challenge, Registry, Runner, Logger interfaces | - |
| `registry.go` | Challenge registration with dependency ordering | 7 |
| `runner.go` | Challenge execution lifecycle | - |
| `assertions.go` | 15+ assertion evaluators | 14 |
| `env.go` | Secure environment handling with redaction | 12 |
| `logger.go` | JSON/Console/Multi/Redacting loggers | 13 |
| `reporter.go` | Markdown/JSON reports with history | 14 |
| `integration_test.go` | Full workflow integration tests | 6 |

### Assertion Types

- `not_empty` - Value is not empty
- `not_mock` - Response is not mocked/placeholder
- `contains` / `contains_any` - Contains expected substrings
- `min_length` / `max_length` - Length constraints
- `quality_score` - Meets quality threshold
- `reasoning_present` - Contains reasoning indicators
- `code_valid` - Contains valid code structure
- `response_time` / `max_latency` - Time constraints
- `format_valid` - Valid JSON format
- `no_errors` - No error indicators
- `min_count` / `no_duplicates` - Array constraints

### Logging System

| Logger | Purpose |
|--------|---------|
| `JSONLogger` | JSON Lines format to files |
| `ConsoleLogger` | Colored terminal output |
| `MultiLogger` | Write to multiple destinations |
| `RedactingLogger` | Automatic secret redaction |
| `NullLogger` | Discard all output |

## Security

**IMPORTANT**: Never commit API keys or sensitive information!

### Protected Files (git-ignored)
- `config/.env` - Contains actual API keys
- `config/providers.yaml` - Actual configuration
- `results/` - All execution results and logs

### Git-versioned Templates
- `config/.env.example` - Environment template with placeholders
- `config/providers.yaml.example` - Configuration template

### Automatic Redaction
- API keys: `sk-ant-api-123...` → `sk-a*********`
- Authorization headers: `Bearer sk-...` → `Bearer ***`
- URL parameters: `?api_key=secret` → `?api_key=***`

## Configuration

### Environment Variables (.env)

```bash
# Provider API Keys
ANTHROPIC_API_KEY=sk-ant-api-...
OPENAI_API_KEY=sk-...
OPENROUTER_API_KEY=sk-or-v1-...
DEEPSEEK_API_KEY=sk-...
GEMINI_API_KEY=...

# SuperAgent Configuration
SUPERAGENT_BASE_URL=http://localhost:8080
SUPERAGENT_API_KEY=...
```

### Provider Configuration (providers.yaml)

```yaml
providers:
  anthropic:
    enabled: true
    model: claude-3-opus-20240229
    weight: 1.0
  openai:
    enabled: true
    model: gpt-4-turbo
    weight: 1.0

scoring:
  response_time_weight: 0.3
  quality_weight: 0.5
  reliability_weight: 0.2

debate_group:
  primary_count: 5
  fallbacks_per_primary: 2
  diversity_preference: true
```

## Running Tests

```bash
# Run all framework tests
cd challenges/codebase/go_files/framework
go test -v ./...

# Run integration tests only
go test -v -run Integration ./...

# Run with coverage
go test -cover ./...
```

## Documentation

- [00_INDEX.md](docs/00_INDEX.md) - Documentation index
- [01_INTRODUCTION.md](docs/01_INTRODUCTION.md) - Framework introduction
- [02_QUICK_START.md](docs/02_QUICK_START.md) - Quick start guide
- [03_CHALLENGE_CATALOG.md](docs/03_CHALLENGE_CATALOG.md) - All challenges
- [04_AI_DEBATE_GROUP.md](docs/04_AI_DEBATE_GROUP.md) - Debate group details
- [05_SECURITY.md](docs/05_SECURITY.md) - Security practices

## Auto-Start Infrastructure

### How It Works

When running any challenge, the system automatically:

1. **Checks if SuperAgent is running** on port 8080
2. **Builds binary if needed** (`make build`)
3. **Starts SuperAgent** with required environment variables
4. **Waits for health check** to pass (up to 30 seconds)
5. **Runs challenge tests**
6. **Stops SuperAgent** when done

### Required Environment Variables

```bash
# In project root .env file:
JWT_SECRET=your-secret-key-at-least-32-characters
```

### Container Runtime Support

Both Docker and Podman are supported:

```bash
# Docker (default)
docker ps

# Podman (auto-detected if docker unavailable)
podman ps

# For Podman rootless mode
systemctl --user enable --now podman.socket
```

### Test Types

- **Required Tests**: Must pass (e.g., code existence checks)
- **Optional Tests**: Don't fail if endpoint unavailable (e.g., `/v1/cache/stats`)

## Main Challenge

The Main Challenge is the comprehensive orchestration that:

1. Verifies ALL providers using LLMsVerifier with REAL API calls
2. Benchmarks and scores available models
3. Forms an AI Debate Group with 5 primaries + fallbacks
4. Generates OpenCode configuration

```bash
./challenges/scripts/main_challenge.sh
# Output: /home/user/Downloads/opencode-super-agent.json
```

See [06_MAIN_CHALLENGE.md](docs/06_MAIN_CHALLENGE.md) for detailed documentation.

## Architecture Documentation

For comprehensive architecture details, see:
- [SUPERAGENT_COMPREHENSIVE_ARCHITECTURE.md](../docs/architecture/SUPERAGENT_COMPREHENSIVE_ARCHITECTURE.md)

## License

Part of the SuperAgent project.
