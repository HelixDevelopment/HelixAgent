# HelixAgent Configuration Guide

## 📋 Overview

This guide covers all configuration options for HelixAgent, from basic setup to advanced production configurations. HelixAgent is highly configurable to support different use cases and deployment scenarios.

---

## 🏗️ Configuration Files Structure

HelixAgent uses a layered configuration approach:

```
configs/
├── multi-provider.yaml      # Multi-provider ensemble configuration
├── test-multi-provider.yaml # Test configuration
└── (custom configs)         # Your custom configurations
```

---

## 🔧 Basic Configuration

### Environment Variables (.env)

Create a `.env` file in the project root:

```bash
# Server Configuration
PORT=7061
GIN_MODE=release        # release, debug
LOG_LEVEL=info          # debug, info, warn, error

# Security
JWT_SECRET=your-secure-jwt-secret-here
HELIXAGENT_API_KEY=your-super-secret-api-key-here

# Database (PostgreSQL)
DB_HOST=localhost
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=helixagent123
DB_NAME=helixagent_db
DB_SSL_MODE=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=helixagent123
REDIS_DB=0

# Feature Flags
GRAPHQL_ENABLED=false                   # GraphQL endpoint at /v1/graphql
ENABLE_PPROF=false                      # pprof profiling endpoints
COGNEE_ENABLED=false                    # Cognee knowledge graph (replaced by Mem0)
CONSTITUTION_WATCHER_ENABLED=false      # Auto-update Constitution on changes
USE_HELIX_LLM=true                      # HelixLLM submodule integration
LLM_VERIFIER_DISABLED=false             # Skip startup verification

# BigData Features
BIGDATA_ENABLE_INFINITE_CONTEXT=true
BIGDATA_ENABLE_DISTRIBUTED_MEMORY=false
BIGDATA_ENABLE_KNOWLEDGE_GRAPH=false
BIGDATA_ENABLE_ANALYTICS=false
BIGDATA_ENABLE_CROSS_LEARNING=true

# Monitoring
PROMETHEUS_ENABLED=true
PROMETHEUS_PORT=9090
JAEGER_ENDPOINT=http://localhost:14268/api/traces

# Rate Limiting
RATE_LIMIT_RPM=60
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080

# LLM Provider API Keys (at least one required)
OPENAI_API_KEY=sk-your-openai-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key
DEEPSEEK_API_KEY=sk-your-deepseek-key
GEMINI_API_KEY=your-gemini-api-key-here
GROQ_API_KEY=gsk-your-groq-key
MISTRAL_API_KEY=your-mistral-key
COHERE_API_KEY=your-cohere-key
PERPLEXITY_API_KEY=pplx-your-perplexity-key
OPENROUTER_API_KEY=sk-or-your-openrouter-key-here
ZAI_API_KEY=your-zai-key-here

# Search Providers
TAVILY_API_KEY=tvly-your-tavily-key
EXA_API_KEY=your-exa-key
```

---

## 🤖 LLM Provider Configuration

### Single Provider Setup

For simple use cases with one provider:

```yaml
# configs/single-provider.yaml
providers:
  claude:
    name: "Claude"
    type: "claude"
    enabled: true
    api_key: "${CLAUDE_API_KEY}"
    base_url: "https://api.anthropic.com"
    timeout: 30
    max_retries: 3
    models:
      - name: "claude-3-sonnet-20240229"
        display_name: "Claude 3 Sonnet"
        capabilities: ["chat", "reasoning", "coding"]
      - name: "claude-3-haiku-20240307"
        display_name: "Claude 3 Haiku"
        capabilities: ["chat", "fast"]
```

### Multi-Provider Ensemble Setup

For intelligent routing across multiple providers:

```yaml
# configs/multi-provider.yaml
providers:
  claude:
    name: "Claude"
    type: "claude"
    enabled: true
    api_key: "${CLAUDE_API_KEY}"
    models:
      - name: "claude-3-sonnet-20240229"
        display_name: "Claude 3 Sonnet"
        capabilities: ["chat", "reasoning", "coding"]
  
  deepseek:
    name: "DeepSeek"
    type: "deepseek"
    enabled: true
    api_key: "${DEEPSEEK_API_KEY}"
    models:
      - name: "deepseek-chat"
        display_name: "DeepSeek Chat"
        capabilities: ["chat", "coding"]
      - name: "deepseek-coder"
        display_name: "DeepSeek Coder"
        capabilities: ["coding"]
  
  gemini:
    name: "Gemini"
    type: "gemini"
    enabled: true
    api_key: "${GEMINI_API_KEY}"
    models:
      - name: "gemini-pro"
        display_name: "Gemini Pro"
        capabilities: ["chat", "multimodal"]
  
  openrouter:
    name: "OpenRouter"
    type: "openrouter"
    enabled: true
    api_key: "${OPENROUTER_API_KEY}"
    models:
      - name: "x-ai/grok-4"
        display_name: "Grok-4"
        capabilities: ["chat", "reasoning"]
      - name: "google/gemini-2.5-flash"
        display_name: "Gemini 2.5 Flash"
        capabilities: ["chat", "multimodal"]

ensemble:
  strategy: "confidence_weighted"  # confidence_weighted, majority_vote, random
  min_providers: 2
  confidence_threshold: 0.7
  fallback_to_best: true
  timeout_per_provider: 10
  max_total_timeout: 30
```

---

## 🎯 Use Case Configurations

### 1. **Development & Testing**

```yaml
# configs/development.yaml
environment: development
log_level: debug
database:
  host: localhost
  port: 5432
  name: helixagent_dev
  ssl_mode: disable

providers:
  claude:
    enabled: true
    timeout: 10
    max_retries: 2
  
  deepseek:
    enabled: true
    timeout: 10
    max_retries: 2

ensemble:
  strategy: "random"
  min_providers: 1
  timeout_per_provider: 5
```

### 2. **Production - High Availability**

```yaml
# configs/production.yaml
environment: production
log_level: info
database:
  host: ${DB_HOST}
  port: ${DB_PORT}
  name: ${DB_NAME}
  ssl_mode: require
  max_connections: 100
  connection_timeout: 30

cache:
  redis:
    host: ${REDIS_HOST}
    port: ${REDIS_PORT}
    password: ${REDIS_PASSWORD}
    db: 0
    ttl: 3600  # 1 hour

providers:
  claude:
    enabled: true
    timeout: 30
    max_retries: 5
    circuit_breaker:
      enabled: true
      failure_threshold: 5
      reset_timeout: 60
  
  deepseek:
    enabled: true
    timeout: 30
    max_retries: 5
    circuit_breaker:
      enabled: true
      failure_threshold: 5
      reset_timeout: 60
  
  gemini:
    enabled: true
    timeout: 30
    max_retries: 5
    circuit_breaker:
      enabled: true
      failure_threshold: 5
      reset_timeout: 60

ensemble:
  strategy: "confidence_weighted"
  min_providers: 2
  confidence_threshold: 0.8
  fallback_to_best: true
  timeout_per_provider: 15
  max_total_timeout: 45
  health_check_interval: 30
```

### 3. **Cost-Optimized Configuration**

```yaml
# configs/cost-optimized.yaml
providers:
  deepseek:  # Lower cost provider as primary
    enabled: true
    weight: 0.7
    models:
      - name: "deepseek-chat"
        cost_per_token: 0.000001
  
  claude:    # Higher quality for complex tasks
    enabled: true
    weight: 0.3
    models:
      - name: "claude-3-haiku-20240307"
        cost_per_token: 0.000003

ensemble:
  strategy: "cost_aware"
  min_providers: 1
  max_cost_per_request: 0.01
  quality_threshold: 0.6
```

### 4. **Latency-Optimized Configuration**

```yaml
# configs/low-latency.yaml
providers:
  claude:
    enabled: true
    timeout: 5
    priority: 1
    models:
      - name: "claude-3-haiku-20240307"  # Faster model
        avg_latency_ms: 800
  
  deepseek:
    enabled: true
    timeout: 5
    priority: 2
    models:
      - name: "deepseek-chat"
        avg_latency_ms: 1200

ensemble:
  strategy: "fastest_response"
  min_providers: 1
  max_wait_time_ms: 3000
  accept_partial_results: true
```

---

## 🔐 Security Configuration

### Authentication & Authorization

```yaml
# configs/secure.yaml
security:
  jwt:
    secret: ${JWT_SECRET}
    expiration_hours: 24
    refresh_expiration_days: 7
  
  rate_limiting:
    enabled: true
    requests_per_minute: 60
    burst_size: 10
    by_ip: true
    by_user: true
  
  cors:
    enabled: true
    allowed_origins:
      - "https://yourdomain.com"
      - "http://localhost:3000"
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["Content-Type", "Authorization"]
  
  api_keys:
    enabled: true
    rotation_days: 30
    max_keys_per_user: 5
```

### Database Encryption

```yaml
database:
  encryption:
    enabled: true
    algorithm: "aes-256-gcm"
    key: ${DB_ENCRYPTION_KEY}
  
  audit_logging:
    enabled: true
    retention_days: 90
```

---

## 📊 Monitoring & Metrics

```yaml
# configs/monitoring.yaml
monitoring:
  prometheus:
    enabled: true
    path: "/metrics"
    port: 9090
  
  health_checks:
    enabled: true
    interval_seconds: 30
    timeout_seconds: 5
  
  logging:
    level: "info"
    format: "json"
    output: "stdout"
    file:
      enabled: true
      path: "/var/log/helixagent.log"
      max_size_mb: 100
      max_backups: 10
      max_age_days: 30
  
  tracing:
    enabled: true
    service_name: "helixagent"
    exporter: "jaeger"  # or "zipkin", "otlp"
    endpoint: "http://localhost:14268/api/traces"
    sampling_rate: 0.1
```

---

## 🔌 Plugin Configuration

```yaml
# configs/plugins.yaml
plugins:
  enabled: true
  directory: "./plugins"
  auto_discovery: true
  hot_reload: false
  
  security:
    sandbox_enabled: true
    max_memory_mb: 512
    max_execution_time_seconds: 30
  
  dependencies:
    auto_install: false
    check_compatibility: true
  
  example_plugin:
    enabled: true
    config:
      api_key: "${EXAMPLE_PLUGIN_API_KEY}"
      endpoint: "https://api.example.com"
      timeout: 10
```

---

## 🚀 Deployment Configurations

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  helixagent:
    build: .
    ports:
      - "8080:7061"
    environment:
      - PORT=7061
      - ENVIRONMENT=production
      - DB_HOST=postgres
      - DB_PORT=5432
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis
    volumes:
      - ./configs:/app/configs
      - ./logs:/app/logs
  
  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=helixagent
      - POSTGRES_USER=helixagent
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres_data:/var/lib/postgresql/data
  
  redis:
    image: redis:7-alpine
    command: redis-server --requirepass password
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

### Kubernetes

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helixagent
spec:
  replicas: 3
  selector:
    matchLabels:
      app: helixagent
  template:
    metadata:
      labels:
        app: helixagent
    spec:
      containers:
      - name: helixagent
        image: helixagent:latest
        ports:
        - containerPort: 8080
        env:
        - name: ENVIRONMENT
          value: "production"
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: db-secret
              key: host
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

---

## 🔄 Configuration Management

### Environment-Specific Configs

```bash
# Load different configs based on environment
export ENVIRONMENT=production
export CONFIG_PATH="configs/${ENVIRONMENT}.yaml"

# Or use make targets
make run-dev      # Uses configs/development.yaml
make run-staging  # Uses configs/staging.yaml
make run-prod     # Uses configs/production.yaml
```

### Configuration Validation

```bash
# Validate configuration
make validate-config

# Test configuration
make test-config

# Generate configuration template
make config-template
```

---

## 🛠️ Advanced Configuration Options

### Custom Ensemble Strategies

```yaml
ensemble:
  strategy: "custom"
  custom_strategy:
    name: "quality_cost_balanced"
    parameters:
      quality_weight: 0.6
      cost_weight: 0.3
      latency_weight: 0.1
      min_quality_score: 0.7
      max_cost_per_token: 0.000005
```

### Provider Weighting

```yaml
providers:
  claude:
    weight: 0.4
    models:
      - name: "claude-3-sonnet-20240229"
        weight: 0.7
      - name: "claude-3-haiku-20240307"
        weight: 0.3
  
  deepseek:
    weight: 0.3
    models:
      - name: "deepseek-chat"
        weight: 0.5
      - name: "deepseek-coder"
        weight: 0.5
  
  gemini:
    weight: 0.3
```

### Request Routing Rules

```yaml
routing:
  rules:
    - condition: "request.prompt contains 'code'"
      action: "route_to"
      target: "deepseek"
      priority: "coding"
    
    - condition: "request.prompt length > 1000"
      action: "route_to"
      target: "claude"
      priority: "long_text"
    
    - condition: "request.model == 'gemini-pro'"
      action: "use_specific"
      target: "gemini"
    
    - condition: "time.hour between 9 and 17"
      action: "prefer"
      target: ["claude", "deepseek"]
      fallback: "gemini"
```

---

## 🧪 Testing Configuration

```yaml
# configs/test.yaml
environment: test
log_level: debug

providers:
  mock:
    name: "Mock Provider"
    type: "mock"
    enabled: true
    responses:
      - prompt: "Hello"
        response: "Hi there!"
      - prompt: "What is 2+2?"
        response: "2+2 equals 4"

ensemble:
  strategy: "test"
  min_providers: 1

testing:
  enabled: true
  fixtures_path: "./tests/fixtures"
  mock_external_apis: true
  record_requests: false
```

---

## 🔍 Configuration Validation

### Schema Validation

HelixAgent validates configuration against a schema:

```bash
# Check configuration syntax
go run ./cmd/validate-config/main.go --config configs/production.yaml

# Validate environment variables
make check-env

# Generate configuration documentation
make config-docs
```

### Health Check Configuration

```yaml
health:
  checks:
    - name: "database"
      type: "postgres"
      interval: 30
      timeout: 5
    
    - name: "redis"
      type: "redis"
      interval: 30
      timeout: 5
    
    - name: "providers"
      type: "providers"
      interval: 60
      timeout: 10
      required_providers: ["claude", "deepseek"]
  
  endpoints:
    - path: "/health"
      public: true
      checks: ["database", "redis"]
    
    - path: "/health/detailed"
      public: false
      checks: ["database", "redis", "providers"]
    
    - path: "/health/live"
      public: true
      checks: []
```

---

## 🧪 Challenge System Configuration

Configure the challenge validation system for testing:

```yaml
# configs/challenges.yaml
challenges:
  timeout: 60  # Default timeout for challenge tests (seconds)
  parallel_workers: 4  # Number of parallel test workers

  rags:
    enabled: true
    timeout: 60  # RAGS tests may need longer timeout
    cognee_url: "http://localhost:8000"
    qdrant_url: "http://localhost:6333"

  mcps:
    enabled: true
    timeout: 30
    adapters:
      - database
      - design
      - ai-services

  skills:
    enabled: true
    strict_validation: true  # Require real results, not just HTTP 200

# Running challenges
# ./challenges/scripts/run_all_challenges.sh
# ./challenges/scripts/rags_challenge.sh  # 147 tests
# ./challenges/scripts/mcps_challenge.sh  # 9 sections
```

---

## 🔍 MCP Tool Search Configuration

Configure MCP tool search for cross-adapter discovery:

```yaml
# configs/mcp.yaml
mcp:
  tool_search:
    enabled: true
    index_on_startup: true
    cache_ttl: 3600  # Cache search results for 1 hour

  adapters:
    database:
      enabled: true
      tools: ["query", "insert", "update", "delete"]

    design:
      enabled: true
      tools: ["figma_export", "sketch_convert"]

    ai_services:
      enabled: true
      tools: ["translate", "summarize", "classify"]

  search:
    algorithm: "semantic"  # semantic, keyword, hybrid
    max_results: 10
    min_score: 0.5
```

---

## ✨ Multi-Pass Validation Configuration

Configure multi-pass validation for AI debates:

```yaml
# configs/debate.yaml
debate:
  multi_pass_validation:
    enabled: true
    default_validation_timeout: 120  # seconds
    default_polish_timeout: 60
    max_validation_rounds: 3
    min_confidence_to_skip: 0.9
    show_phase_indicators: true

  phases:
    - name: "INITIAL RESPONSE"
      icon: "🔍"
      timeout: 60
    - name: "VALIDATION"
      icon: "✓"
      timeout: 60
    - name: "POLISH & IMPROVE"
      icon: "✨"
      timeout: 45
    - name: "FINAL CONCLUSION"
      icon: "📜"
      timeout: 30
```

---

## 📚 RAG System Configuration

Configure the hybrid RAG system:

```yaml
# configs/rag.yaml
rag:
  enabled: true

  retrieval:
    type: "hybrid"  # dense, sparse, hybrid
    top_k: 10
    rerank: true

  dense:
    model: "sentence-transformers/all-MiniLM-L6-v2"
    dimension: 384

  sparse:
    algorithm: "bm25"
    k1: 1.5
    b: 0.75

  reranker:
    model: "cross-encoder/ms-marco-MiniLM-L-6-v2"
    top_k: 5

  qdrant:
    url: "${QDRANT_URL:-http://localhost:6333}"
    collection: "helixagent_docs"

  cognee:
    url: "${COGNEE_URL:-http://localhost:8000}"
    auth_email: "${COGNEE_AUTH_EMAIL}"
    auth_password: "${COGNEE_AUTH_PASSWORD}"
```

---

## 📝 Configuration Best Practices

1. **Use Environment Variables** for secrets
2. **Version Control** configuration files (without secrets)
3. **Validate** configurations before deployment
4. **Test** configurations in staging first
5. **Document** custom configurations
6. **Monitor** configuration changes
7. **Backup** production configurations
8. **Use Templates** for similar environments

---

## 🆘 Troubleshooting Configuration Issues

### Common Issues:

1. **Missing API Keys**: Check `.env` file and environment variables
2. **Invalid YAML Syntax**: Use a YAML validator
3. **Port Conflicts**: Change `PORT` in `.env`
4. **Database Connection**: Verify DB credentials and network
5. **Provider Timeouts**: Increase `timeout` values

### Debug Commands:

```bash
# Show effective configuration
make show-config

# Test provider connectivity
make test-providers

# Validate all configurations
make validate-all

# Generate configuration report
make config-report
```

---

## 📚 Next Steps

- [Quick Start Guide](./quick-start-guide.md)
- [API Documentation](../api/api-documentation.md)
- [Troubleshooting Guide](./troubleshooting-guide.md)
- [Production Deployment](../deployment/production-deployment.md)

---

**Need Help?** Check the [troubleshooting guide](./troubleshooting-guide.md) or open an issue on GitHub.