# Module 3: Configuration

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 3: Configuration
- Duration: 60 minutes
- Master All Configuration Options

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Master HelixAgent configuration architecture
- Configure environment variables and YAML files
- Set up provider-specific configurations
- Implement multi-environment configurations
- Apply secrets management best practices

---

## Slide 3: Configuration Architecture

**Configuration Sources (Priority Order):**

1. Command-line flags (highest)
2. Environment variables
3. Configuration files (YAML)
4. Default values (lowest)

*Higher priority sources override lower ones*

---

## Slide 4: Configuration Files

**Available Configuration Files:**

```
configs/
  +-- development.yaml   # Local development
  +-- production.yaml    # Production settings
  +-- multi-provider.yaml # Multi-provider example
  +-- ai-debate.yaml     # AI Debate configuration
```

---

## Slide 5: Environment Variables

**Setting Environment Variables:**

```bash
# From .env file
cp .env.example .env
nano .env

# Or export directly
export PORT=8080
export GIN_MODE=release
export JWT_SECRET=your-secret-key
```

---

## Slide 6: Core Server Configuration

**Server Settings:**

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | HTTP server port |
| GIN_MODE | release | debug/release/test |
| JWT_SECRET | - | JWT signing key |
| LOG_LEVEL | info | Logging verbosity |

---

## Slide 7: Database Configuration

**PostgreSQL Settings:**

```yaml
database:
  host: ${DB_HOST}
  port: ${DB_PORT}
  user: ${DB_USER}
  password: ${DB_PASSWORD}
  name: ${DB_NAME}
  ssl_mode: disable
  max_connections: 100
  idle_connections: 10
```

---

## Slide 8: Redis Configuration

**Cache Settings:**

```yaml
redis:
  host: ${REDIS_HOST}
  port: ${REDIS_PORT}
  password: ${REDIS_PASSWORD}
  db: 0
  pool_size: 100
  min_idle_conns: 10
```

---

## Slide 9: Provider API Keys

**Required API Keys:**

```bash
# Anthropic Claude
CLAUDE_API_KEY=sk-ant-...

# Google Gemini
GEMINI_API_KEY=AIza...

# DeepSeek
DEEPSEEK_API_KEY=sk-...

# OpenRouter
OPENROUTER_API_KEY=sk-or-...

# Qwen
QWEN_API_KEY=...

# ZAI
ZAI_API_KEY=...
```

---

## Slide 10: Claude Configuration

**Anthropic Claude Settings:**

```yaml
providers:
  claude:
    api_key: ${CLAUDE_API_KEY}
    model: claude-3-5-sonnet-20241022
    base_url: https://api.anthropic.com/v1
    timeout: 30s
    max_retries: 3
    temperature: 0.7
    max_tokens: 4096
```

---

## Slide 11: Gemini Configuration

**Google Gemini Settings:**

```yaml
providers:
  gemini:
    api_key: ${GEMINI_API_KEY}
    model: gemini-pro
    base_url: https://generativelanguage.googleapis.com
    timeout: 30s
    temperature: 0.7
    max_tokens: 4096
```

---

## Slide 12: DeepSeek Configuration

**DeepSeek Settings:**

```yaml
providers:
  deepseek:
    api_key: ${DEEPSEEK_API_KEY}
    model: deepseek-coder
    base_url: https://api.deepseek.com/v1
    timeout: 30s
    temperature: 0.1
    max_tokens: 4096
```

---

## Slide 13: Ollama Configuration

**Local LLM with Ollama:**

```yaml
providers:
  ollama:
    enabled: ${OLLAMA_ENABLED}
    base_url: ${OLLAMA_BASE_URL}
    model: ${OLLAMA_MODEL}
    timeout: 60s
    # No API key required for local deployment
```

```bash
# Environment variables
OLLAMA_ENABLED=true
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama2
```

---

## Slide 14: Multi-Provider YAML Example

**Complete Provider Configuration:**

```yaml
providers:
  enabled:
    - claude
    - gemini
    - deepseek

  default_provider: claude
  fallback_chain:
    - claude
    - gemini
    - deepseek

  routing:
    strategy: performance  # performance, cost, random
    health_check_interval: 30s
```

---

## Slide 15: Ensemble Configuration

**Voting Strategy Settings:**

```yaml
ensemble:
  enabled: true
  strategy: confidence_weighted
  min_providers: 2
  max_providers: 5
  timeout: 45s

  voting:
    type: confidence_weighted  # majority, weighted, consensus
    consensus_threshold: 0.75
```

---

## Slide 16: AI Debate Configuration

**Debate System Settings:**

```yaml
ai_debate:
  enabled: true
  maximal_repeat_rounds: 3
  debate_timeout: 300000
  consensus_threshold: 0.75

  debate_strategy: structured
  voting_strategy: confidence_weighted
  enable_cognee: true
```

---

## Slide 17: Participant Configuration

**Debate Participant Settings:**

```yaml
participants:
  - name: "Analyst"
    role: "Primary Analyst"
    enabled: true
    weight: 1.5
    priority: 1
    debate_style: analytical
    argumentation_style: logical

    llms:
      - name: "Claude Primary"
        provider: claude
        model: claude-3-sonnet
        temperature: 0.1
```

---

## Slide 18: Cognee Integration

**Knowledge Graph Settings:**

```yaml
cognee_config:
  enabled: true
  enhance_responses: true
  analyze_consensus: true
  generate_insights: true
  dataset_name: "ai_debate_enhancement"
  enhancement_strategy: hybrid
  memory_integration: true
```

---

## Slide 19: Optimization Configuration

**Performance Optimization Settings:**

```yaml
optimization:
  enabled: true

  semantic_cache:
    enabled: true
    similarity_threshold: 0.85
    max_entries: 10000
    ttl: 24h

  streaming:
    enabled: true
    buffer_type: word

  sglang:
    enabled: true
    endpoint: http://localhost:30000
```

---

## Slide 20: Rate Limiting Configuration

**Request Rate Limits:**

```yaml
rate_limiting:
  enabled: true

  global:
    requests_per_second: 100
    burst: 200

  per_user:
    requests_per_minute: 60
    burst: 10

  per_provider:
    claude:
      requests_per_second: 10
    gemini:
      requests_per_second: 15
```

---

## Slide 21: Logging Configuration

**Logging Settings:**

```yaml
logging:
  level: info  # debug, info, warn, error
  format: json  # json, text
  output: stdout

  file:
    enabled: true
    path: /var/log/helixagent
    max_size: 100M
    max_backups: 5
    max_age: 30
```

---

## Slide 22: Cloud Provider Configuration

**AWS Bedrock:**

```yaml
cloud:
  aws:
    region: ${AWS_REGION}
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
    bedrock:
      enabled: true
      models:
        - claude-3-sonnet
        - titan-text
```

---

## Slide 23: Cloud Provider - GCP

**GCP Vertex AI:**

```yaml
cloud:
  gcp:
    project_id: ${GCP_PROJECT_ID}
    location: ${GCP_LOCATION}
    access_token: ${GOOGLE_ACCESS_TOKEN}
    vertex_ai:
      enabled: true
      models:
        - gemini-pro
        - palm-2
```

---

## Slide 24: Azure OpenAI Configuration

**Azure OpenAI:**

```yaml
cloud:
  azure:
    endpoint: ${AZURE_OPENAI_ENDPOINT}
    api_key: ${AZURE_OPENAI_API_KEY}
    api_version: ${AZURE_OPENAI_API_VERSION}
    deployments:
      - gpt-4
      - gpt-35-turbo
```

---

## Slide 25: Environment-Specific Configs

**Development vs Production:**

| Setting | Development | Production |
|---------|-------------|------------|
| GIN_MODE | debug | release |
| LOG_LEVEL | debug | info |
| DB_SSL_MODE | disable | require |
| RATE_LIMIT | 1000/s | 100/s |

---

## Slide 26: Secrets Management

**Best Practices:**

1. Never commit secrets to version control
2. Use environment variables for sensitive data
3. Consider secrets managers (Vault, AWS Secrets)
4. Rotate API keys regularly
5. Use separate keys per environment

```bash
# .gitignore
.env
*.key
secrets/
```

---

## Slide 27: Configuration Validation

**Validating Configuration:**

```bash
# Check configuration syntax
make validate-config

# Test with dry run
./bin/helixagent --config configs/production.yaml --dry-run

# Verify provider connections
curl http://localhost:7061/v1/providers/health
```

---

## Slide 28: Hands-On Lab

**Lab Exercise 3.1: Custom Configuration**

Tasks:
1. Create a custom configuration file
2. Configure at least 2 providers
3. Set up ensemble voting
4. Configure rate limiting
5. Test configuration validation

Time: 25 minutes

---

## Slide 29: Module Summary

**Key Takeaways:**

- Configuration priority: CLI > ENV > YAML > defaults
- Provider-specific settings for each LLM
- Ensemble and AI Debate require separate config
- Cloud providers for enterprise deployments
- Always use environment variables for secrets
- Validate configuration before deployment

**Next: Module 4 - LLM Provider Integration**

---

## Speaker Notes

### Slide 3 Notes
Explain the configuration precedence clearly. This is important for understanding how to override settings in different environments.

### Slide 9 Notes
Emphasize never committing API keys to version control. Demonstrate using environment variables.

### Slide 16 Notes
Walk through the AI Debate configuration in detail. This is a complex feature that requires careful configuration.

### Slide 26 Notes
Security is critical. Discuss secret management options for different scales of deployment.
