# LLM Optimization User Guide

This guide explains how to use HelixAgent's LLM optimization features to improve performance, reduce costs, and enhance output quality for large codebases.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Semantic Caching](#semantic-caching)
4. [Structured Output](#structured-output)
5. [Enhanced Streaming](#enhanced-streaming)
6. [External Optimization Services](#external-optimization-services)
7. [Configuration](#configuration)
8. [Best Practices](#best-practices)
9. [Troubleshooting](#troubleshooting)

---

## Overview

HelixAgent integrates 8 LLM optimization tools to provide:

| Capability | Tool | Benefit |
|------------|------|---------|
| **Semantic Caching** | GPTCache-inspired | Avoid redundant LLM calls, reduce costs 40-60% |
| **Structured Output** | Outlines-inspired | Guarantee valid JSON/regex patterns |
| **Enhanced Streaming** | llm-streaming | Word/sentence buffering, progress tracking |
| **Prefix Caching** | SGLang | Faster responses for repeated context |
| **Document Retrieval** | LlamaIndex + Cognee | Smart context from large codebases |
| **Task Decomposition** | LangChain | Break complex tasks into steps |
| **Grammar Constraints** | Guidance | CFG-based output control |
| **Query Language** | LMQL | Declarative LLM queries |

### Architecture

```
Client Request
    ↓
┌─────────────────────────────────────┐
│      Optimization Pipeline          │
├─────────────────────────────────────┤
│ 1. Semantic Cache Check             │ ← Cache hit? Return immediately
│ 2. Task Decomposition (LangChain)   │ ← Break into subtasks
│ 3. Context Retrieval (LlamaIndex)   │ ← Get relevant documents
│ 4. Prefix Warming (SGLang)          │ ← Pre-cache common prefixes
└─────────────────────────────────────┘
    ↓
┌─────────────────────────────────────┐
│       LLM Provider Ensemble         │
└─────────────────────────────────────┘
    ↓
┌─────────────────────────────────────┐
│      Response Optimization          │
├─────────────────────────────────────┤
│ 1. Structured Output Validation     │ ← Ensure valid JSON/schema
│ 2. Cache Storage                    │ ← Store for future reuse
│ 3. Enhanced Streaming               │ ← Word/sentence buffering
└─────────────────────────────────────┘
    ↓
Client Response
```

---

## Quick Start

### 1. Enable Optimization Services

Start the optimization services with Docker:

```bash
# Core services only
docker-compose up -d

# With optimization services (recommended)
docker-compose --profile optimization up -d

# With GPU support (for SGLang)
docker-compose --profile optimization-gpu up -d

# Everything
docker-compose --profile full up -d
```

### 2. Verify Services Are Running

```bash
# Check all services
docker-compose ps

# Check optimization services specifically
curl http://localhost:8011/health  # LangChain
curl http://localhost:8012/health  # LlamaIndex
curl http://localhost:8013/health  # Guidance
curl http://localhost:8014/health  # LMQL
curl http://localhost:30000/health # SGLang (if GPU available)
```

### 3. Make an Optimized Request

```bash
# Standard request with all optimizations enabled (default)
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "default",
    "messages": [{"role": "user", "content": "Explain async/await in JavaScript"}]
  }'
```

The response includes optimization metadata:

```json
{
  "id": "chatcmpl-abc123",
  "choices": [...],
  "usage": {...},
  "optimization": {
    "cache_hit": false,
    "cache_similarity": 0.0,
    "response_cached": true,
    "structured_validation": "passed"
  }
}
```

---

## Semantic Caching

Semantic caching avoids redundant LLM calls by finding similar previous queries.

### How It Works

1. Your query is converted to a vector embedding
2. We search for semantically similar cached queries
3. If similarity exceeds threshold (default 85%), return cached response
4. Otherwise, call LLM and cache the new response

### Example: Cache Hit

```bash
# First request - calls LLM, caches result
curl -X POST http://localhost:7061/v1/chat/completions \
  -d '{"messages": [{"role": "user", "content": "What is Python?"}]}'
# Response time: ~2000ms

# Similar request - returns cached result
curl -X POST http://localhost:7061/v1/chat/completions \
  -d '{"messages": [{"role": "user", "content": "Explain Python programming language"}]}'
# Response time: ~50ms (cache hit!)
```

### Configuration

```yaml
# configs/production.yaml
optimization:
  semantic_cache:
    enabled: true
    similarity_threshold: 0.85  # 0.0-1.0, higher = stricter matching
    max_entries: 50000
    ttl: "24h"                  # How long to keep cached entries
    embedding_model: "text-embedding-3-small"
```

### Cache Control

```bash
# Skip cache for this request
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "X-Skip-Cache: true" \
  -d '{"messages": [...]}'

# Force cache refresh
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "X-Cache-Refresh: true" \
  -d '{"messages": [...]}'

# Get cache stats
curl http://localhost:7061/v1/cache/stats
```

### Expected Savings

| Workload Type | Cache Hit Rate | Cost Reduction |
|---------------|----------------|----------------|
| Code Documentation | 60-70% | 50-60% |
| Repetitive Q&A | 70-80% | 60-70% |
| Code Review | 40-50% | 30-40% |
| Creative Tasks | 10-20% | 5-15% |

---

## Structured Output

Guarantee your LLM outputs valid JSON, match regex patterns, or conform to schemas.

### JSON Output with Schema

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "Generate a user profile"}],
    "response_format": {
      "type": "json_schema",
      "json_schema": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "age": {"type": "integer", "minimum": 0},
          "email": {"type": "string", "format": "email"}
        },
        "required": ["name", "age"]
      }
    }
  }'
```

Response is guaranteed to be valid JSON matching the schema:

```json
{
  "choices": [{
    "message": {
      "content": "{\"name\": \"Alice\", \"age\": 28, \"email\": \"alice@example.com\"}"
    }
  }],
  "optimization": {
    "structured_validation": "passed",
    "validation_attempts": 1
  }
}
```

### Regex Pattern Matching

```bash
# Generate a phone number in specific format
curl -X POST http://localhost:7061/v1/chat/completions \
  -d '{
    "messages": [{"role": "user", "content": "Generate a US phone number"}],
    "response_format": {
      "type": "regex",
      "pattern": "\\(\\d{3}\\) \\d{3}-\\d{4}"
    }
  }'
```

### Enum/Choice Selection

```bash
# Force selection from specific options
curl -X POST http://localhost:7061/v1/chat/completions \
  -d '{
    "messages": [{"role": "user", "content": "What is the best programming language?"}],
    "response_format": {
      "type": "enum",
      "values": ["Python", "JavaScript", "Go", "Rust"]
    }
  }'
```

---

## Enhanced Streaming

Get better streaming experience with word/sentence buffering and progress tracking.

### Word-Buffered Streaming

Receives complete words instead of character fragments:

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "X-Stream-Buffer: word" \
  -d '{
    "messages": [{"role": "user", "content": "Write a poem"}],
    "stream": true
  }'
```

### Sentence-Buffered Streaming

Receives complete sentences:

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "X-Stream-Buffer: sentence" \
  -d '{
    "messages": [{"role": "user", "content": "Explain quantum computing"}],
    "stream": true
  }'
```

### Progress Tracking

Include progress information in stream:

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "X-Stream-Progress: true" \
  -d '{
    "messages": [{"role": "user", "content": "Write a long story"}],
    "stream": true
  }'

# Stream includes progress events:
# data: {"type": "progress", "tokens_generated": 50, "estimated_total": 200}
```

---

## External Optimization Services

### LangChain: Task Decomposition

Break complex tasks into manageable subtasks:

```bash
# Direct API call
curl -X POST http://localhost:8011/decompose \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Build a REST API with authentication and database",
    "max_steps": 5
  }'

# Response:
{
  "subtasks": [
    {"id": 1, "description": "Set up project structure", "dependencies": [], "complexity": "low"},
    {"id": 2, "description": "Implement database layer", "dependencies": [1], "complexity": "medium"},
    {"id": 3, "description": "Create authentication system", "dependencies": [1, 2], "complexity": "high"},
    {"id": 4, "description": "Build API endpoints", "dependencies": [2, 3], "complexity": "medium"},
    {"id": 5, "description": "Add tests and documentation", "dependencies": [4], "complexity": "low"}
  ],
  "reasoning": "Decomposed into logical development phases with dependency tracking"
}
```

### LlamaIndex: Smart Document Retrieval

Query your codebase with advanced retrieval:

```bash
# Query with HyDE (Hypothetical Document Embeddings)
curl -X POST http://localhost:8012/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How does the authentication middleware work?",
    "top_k": 5,
    "query_transform": "hyde",
    "use_cognee": true
  }'

# Response:
{
  "answer": "The authentication middleware validates JWT tokens...",
  "sources": [
    {"content": "...", "score": 0.95, "metadata": {"file": "middleware/auth.go"}},
    {"content": "...", "score": 0.87, "metadata": {"file": "handlers/auth.go"}}
  ],
  "confidence": 0.92
}
```

### Guidance: Grammar-Constrained Generation

Generate text following specific grammars:

```bash
curl -X POST http://localhost:8013/grammar \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Generate a SQL query to find users",
    "grammar": "SELECT columns FROM table WHERE condition ORDER BY column"
  }'
```

### LMQL: Declarative Queries

Use LMQL for complex constrained generation:

```bash
curl -X POST http://localhost:8014/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "argmax \"Name: [NAME]\\nAge: [AGE]\" from \"Generate a person:\" where len(NAME) < 20 and AGE in [\"20\", \"30\", \"40\"]"
  }'
```

---

## Configuration

### Full Configuration Example

```yaml
# configs/production.yaml
optimization:
  enabled: true

  # Native Go components
  semantic_cache:
    enabled: true
    similarity_threshold: 0.85
    max_entries: 50000
    ttl: "24h"
    embedding_model: "text-embedding-3-small"
    eviction_policy: "lru_with_relevance"

  structured_output:
    enabled: true
    default_validator: "outlines"
    strict_mode: true
    retry_on_failure: true
    max_retries: 3

  streaming:
    enabled: true
    buffer_type: "word"
    progress_interval: "100ms"

  # External services
  sglang:
    enabled: true
    endpoint: "http://localhost:30000"
    fallback_on_unavailable: true

  llamaindex:
    enabled: true
    endpoint: "http://localhost:8012"
    use_cognee_index: true

  langchain:
    enabled: true
    endpoint: "http://localhost:8011"
    default_chain: "react"

  guidance:
    enabled: true
    endpoint: "http://localhost:8013"
    cache_programs: true

  lmql:
    enabled: true
    endpoint: "http://localhost:8014"
    cache_queries: true

  # Graceful degradation
  fallback:
    on_service_unavailable: "skip"
    health_check_interval: "30s"
    retry_unavailable_after: "5m"
```

### Environment Variables

```bash
# Enable/disable optimization
OPTIMIZATION_ENABLED=true

# Semantic cache
SEMANTIC_CACHE_ENABLED=true
SEMANTIC_CACHE_THRESHOLD=0.85
SEMANTIC_CACHE_MAX_ENTRIES=50000
SEMANTIC_CACHE_TTL=24h

# External services
SGLANG_ENABLED=true
SGLANG_ENDPOINT=http://localhost:30000
LLAMAINDEX_ENABLED=true
LLAMAINDEX_ENDPOINT=http://localhost:8012
LANGCHAIN_ENABLED=true
LANGCHAIN_ENDPOINT=http://localhost:8011
GUIDANCE_ENABLED=true
GUIDANCE_ENDPOINT=http://localhost:8013
LMQL_ENABLED=true
LMQL_ENDPOINT=http://localhost:8014
```

---

## Best Practices

### 1. Cache Tuning

**Start Conservative, Then Adjust:**

```yaml
# Start with high threshold (fewer cache hits, higher accuracy)
semantic_cache:
  similarity_threshold: 0.90

# Gradually lower if results are good
semantic_cache:
  similarity_threshold: 0.85
```

**Monitor Cache Performance:**

```bash
# Get cache statistics
curl http://localhost:7061/v1/cache/stats

# Expected output:
{
  "total_queries": 10000,
  "cache_hits": 6500,
  "cache_misses": 3500,
  "hit_rate": 0.65,
  "avg_similarity": 0.89,
  "memory_usage_mb": 256
}
```

### 2. Structured Output

**Always Use Schemas for APIs:**

```bash
# Good: Explicit schema
"response_format": {
  "type": "json_schema",
  "json_schema": {
    "type": "object",
    "properties": {
      "status": {"type": "string", "enum": ["success", "error"]},
      "data": {"type": "object"}
    },
    "required": ["status"]
  }
}

# Bad: Hoping for the best
"messages": [{"role": "user", "content": "Return JSON with status and data"}]
```

### 3. Task Decomposition

**Use for Complex Tasks:**

```bash
# Decompose before executing
curl -X POST http://localhost:8011/decompose \
  -d '{"task": "Refactor the authentication system to use OAuth2"}'

# Then execute each subtask in order
for subtask in subtasks:
    execute(subtask)
```

### 4. Context Retrieval

**Combine Cognee with LlamaIndex:**

```yaml
llamaindex:
  use_cognee_index: true  # Query Cognee's knowledge graph
  # LlamaIndex handles advanced retrieval (HyDE, reranking)
  # Cognee handles indexing (no duplication)
```

### 5. Graceful Degradation

**Configure Fallbacks:**

```yaml
fallback:
  on_service_unavailable: "skip"  # Continue without optimization
  health_check_interval: "30s"     # Check service health
  retry_unavailable_after: "5m"    # Retry failed services
```

---

## Troubleshooting

### Service Not Available

```bash
# Check service status
docker-compose ps | grep optimization

# Check service logs
docker-compose logs langchain-server
docker-compose logs llamaindex-server

# Restart specific service
docker-compose restart langchain-server
```

### Low Cache Hit Rate

1. **Lower the threshold:**
   ```yaml
   semantic_cache:
     similarity_threshold: 0.80  # Lower = more hits
   ```

2. **Check embedding quality:**
   ```bash
   # View cache entries
   curl http://localhost:7061/v1/cache/entries?limit=10
   ```

3. **Increase cache size:**
   ```yaml
   semantic_cache:
     max_entries: 100000
     ttl: "48h"
   ```

### Structured Output Failures

1. **Enable retries:**
   ```yaml
   structured_output:
     retry_on_failure: true
     max_retries: 5
   ```

2. **Check schema validity:**
   ```bash
   # Validate schema
   curl -X POST http://localhost:7061/v1/schema/validate \
     -d '{"schema": {...}}'
   ```

3. **Use simpler schemas:**
   - Remove complex regex patterns
   - Use broader type constraints
   - Add `additionalProperties: true`

### High Latency

1. **Check service health:**
   ```bash
   for port in 8011 8012 8013 8014; do
     echo "Port $port: $(curl -s -o /dev/null -w '%{time_total}' http://localhost:$port/health)"
   done
   ```

2. **Disable unused services:**
   ```yaml
   guidance:
     enabled: false  # If not using grammar constraints
   lmql:
     enabled: false  # If not using LMQL queries
   ```

3. **Check resource usage:**
   ```bash
   docker stats --no-stream
   ```

### SGLang GPU Issues

```bash
# Verify GPU access
docker-compose exec sglang nvidia-smi

# Check SGLang logs
docker-compose logs sglang

# Fallback to CPU-only optimization
sglang:
  enabled: false
  fallback_on_unavailable: true
```

---

## Performance Metrics

### Expected Improvements

| Metric | Without Optimization | With Optimization | Improvement |
|--------|---------------------|-------------------|-------------|
| Avg Response Time | 2000ms | 800ms | 60% faster |
| Cache Hit Rate | 0% | 65% | - |
| Cost per 1K requests | $10.00 | $4.00 | 60% cheaper |
| Structured Output Validity | 85% | 99.9% | 14.9% better |
| Context Retrieval Accuracy | 70% | 92% | 22% better |

### Monitoring

```bash
# Prometheus metrics available at:
curl http://localhost:7061/metrics | grep optimization

# Key metrics:
# optimization_cache_hit_total
# optimization_cache_miss_total
# optimization_validation_success_total
# optimization_validation_failure_total
# optimization_service_latency_seconds
```

---

## Related Documentation

- [Semantic Cache Deep Dive](../optimization/SEMANTIC_CACHE_GUIDE.md)
- [Structured Output Reference](../optimization/STRUCTURED_OUTPUT_GUIDE.md)
- [Streaming Configuration](../optimization/STREAMING_GUIDE.md)
- [SGLang Integration](../optimization/SGLANG_INTEGRATION.md)
- [LlamaIndex + Cognee](../optimization/LLAMAINDEX_COGNEE_GUIDE.md)
- [LangChain Chains](../optimization/LANGCHAIN_GUIDE.md)
- [Guidance & LMQL](../optimization/GUIDANCE_LMQL_GUIDE.md)
- [API Reference](../optimization/OPTIMIZATION_API_REFERENCE.md)
