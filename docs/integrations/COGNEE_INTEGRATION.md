# Cognee Integration Guide

HelixAgent integrates with [Cognee](https://github.com/topoteretes/cognee) - an AI Memory Engine that provides knowledge graphs, vector search, temporal awareness, and advanced reasoning capabilities. This integration extends every LLM provider beyond standard boundaries with persistent memory and contextual intelligence.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Features](#features)
4. [Quick Start](#quick-start)
5. [Configuration](#configuration)
6. [API Endpoints](#api-endpoints)
7. [LLM Enhancement](#llm-enhancement)
8. [Docker Compose Setup](#docker-compose-setup)
9. [Performance Tuning](#performance-tuning)
10. [Best Practices](#best-practices)
11. [Troubleshooting](#troubleshooting)

---

## Overview

Cognee transforms HelixAgent into an intelligent agent with:

- **Persistent Memory**: Store and retrieve knowledge across sessions
- **Knowledge Graphs**: Build connected understanding of concepts
- **Semantic Search**: Find relevant information using vector similarity
- **Temporal Awareness**: Understand time-based context
- **Graph Reasoning**: Multi-hop queries across related entities
- **Code Intelligence**: Analyze and understand code structures
- **Feedback Loop**: Self-improvement through usage patterns

Every LLM request is automatically enhanced with relevant context from Cognee, and every response is stored for future reference - creating a continuously improving AI system.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        HelixAgent                                │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  LLM Request Flow                         │   │
│  │                                                           │   │
│  │  Request → Cognee Enhancement → LLM Provider → Response   │   │
│  │              ↓                              ↓             │   │
│  │      Search Memory                   Store Response       │   │
│  │      Get Insights                    Auto-Cognify         │   │
│  │      Graph Context                   Feedback Loop        │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Cognee Service                              │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐   │
│  │   Vector    │ │   Graph     │ │     Relational          │   │
│  │   Store     │ │   Store     │ │       Store             │   │
│  │  (ChromaDB) │ │(NetworkX/   │ │    (PostgreSQL)         │   │
│  │             │ │ Neo4j/      │ │                         │   │
│  │             │ │ Memgraph)   │ │                         │   │
│  └─────────────┘ └─────────────┘ └─────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Three-Tier Storage Architecture

1. **Vector Database (ChromaDB)**: Fast semantic similarity search
2. **Graph Database (NetworkX/Neo4j/Memgraph)**: Relationship traversal and reasoning
3. **Relational Database (PostgreSQL)**: Structured data and metadata

---

## Features

### 1. Memory Enhancement

Every LLM request is automatically enriched with relevant memories:

```go
// Request is enhanced before sending to LLM
enhancedReq := cogneeService.EnhanceRequest(ctx, originalReq)
// enhancedReq.Prompt now includes:
// - Relevant memories from previous interactions
// - Graph insights about mentioned entities
// - Temporal context if applicable
```

### 2. Auto-Cognify

Responses are automatically processed to extract and store knowledge:

```go
// After LLM response
cogneeService.ProcessResponse(ctx, req, resp)
// This:
// - Extracts entities and relationships
// - Builds knowledge graph connections
// - Updates vector embeddings
// - Records temporal metadata
```

### 3. Search Types

| Search Type | Description | Use Case |
|-------------|-------------|----------|
| `VECTOR` | Semantic similarity | Finding conceptually related content |
| `GRAPH` | Graph traversal | Exploring entity relationships |
| `INSIGHTS` | Derived knowledge | Getting summarized understanding |
| `GRAPH_COMPLETION` | Missing link prediction | Discovering implicit connections |

### 4. Graph Reasoning

Multi-hop queries across the knowledge graph:

```go
insights, _ := cogneeService.GetInsights(ctx, "quantum computing", "default", 5)
// Returns connected concepts, related entities, and inferred relationships
```

### 5. Code Intelligence

Analyze code structures and relationships:

```go
analysis, _ := cogneeService.ProcessCode(ctx, sourceCode, "go", "default")
// Returns:
// - Functions and their relationships
// - Import dependencies
// - Type hierarchies
// - Call graphs
```

### 6. Temporal Awareness

Time-aware context retrieval:

```go
// Memories are tagged with timestamps
// Searches can prioritize recent knowledge
// Temporal patterns are recognized
```

### 7. Feedback Loop

Self-improvement through explicit feedback:

```go
cogneeService.ProvideFeedback(ctx, memoryID, 0.95, "Highly relevant response")
// Adjusts relevance scores
// Improves future retrieval
// Optimizes ranking algorithms
```

---

## Quick Start

### 1. Start with Docker Compose

```bash
# Start core services with Cognee
docker-compose --profile default up -d

# This starts:
# - PostgreSQL (relational storage)
# - Redis (caching)
# - ChromaDB (vector storage)
# - Cognee (memory engine)
```

### 2. Configure Environment

```bash
# .env file
COGNEE_ENABLED=true
COGNEE_BASE_URL=http://cognee:8000
COGNEE_AUTO_COGNIFY=true

# LLM for Cognee's internal processing
LLM_API_KEY=your-api-key
COGNEE_LLM_PROVIDER=openai
COGNEE_LLM_MODEL=gpt-4o-mini
```

### 3. Use the API

```bash
# Add memory
curl -X POST http://localhost:7061/api/v1/cognee/memory \
  -H "Content-Type: application/json" \
  -d '{"content": "HelixAgent uses ensemble learning to combine multiple LLM responses."}'

# Search memory
curl -X POST http://localhost:7061/api/v1/cognee/search \
  -H "Content-Type: application/json" \
  -d '{"query": "ensemble learning", "limit": 5}'

# Any LLM request is automatically enhanced
curl -X POST http://localhost:7061/api/v1/llm/complete \
  -H "Content-Type: application/json" \
  -d '{"prompt": "How does HelixAgent work?", "model": "claude"}'
# Response is informed by stored knowledge about ensemble learning
```

---

## Configuration

### Environment Variables

#### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `COGNEE_ENABLED` | `true` | Enable Cognee integration |
| `COGNEE_BASE_URL` | `http://cognee:8000` | Cognee service URL |
| `COGNEE_API_KEY` | `` | Cognee API key (if required) |
| `COGNEE_AUTO_COGNIFY` | `true` | Auto-process new content |

#### Feature Flags

| Variable | Default | Description |
|----------|---------|-------------|
| `COGNEE_ENABLE_TEMPORAL` | `true` | Enable temporal awareness |
| `COGNEE_ENABLE_FEEDBACK` | `true` | Enable feedback loop |
| `COGNEE_ENABLE_CODE_PIPELINE` | `true` | Enable code intelligence |
| `COGNEE_ENABLE_GRAPH_REASONING` | `true` | Enable graph reasoning |
| `COGNEE_ENABLE_MULTI_HOP` | `true` | Enable multi-hop queries |
| `COGNEE_ENABLE_ENTITY_EXTRACTION` | `true` | Auto-extract entities |
| `COGNEE_ENABLE_RELATIONSHIP_INFERENCE` | `true` | Infer relationships |

#### Performance Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `COGNEE_BATCH_SIZE` | `50` | Batch processing size |
| `COGNEE_MAX_CONCURRENT` | `10` | Max concurrent operations |
| `COGNEE_CHUNK_SIZE` | `1024` | Text chunk size for processing |
| `COGNEE_OVERLAP_SIZE` | `128` | Chunk overlap for context |
| `COGNEE_EMBEDDING_BATCH_SIZE` | `32` | Embedding batch size |

#### Memory Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `COGNEE_MAX_MEMORY_SIZE` | `10000` | Max memories to store |
| `COGNEE_MEMORY_TTL` | `0` | Memory TTL (0 = forever) |
| `COGNEE_MEMORY_COMPRESSION` | `true` | Compress older memories |

#### Search Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `COGNEE_DEFAULT_SEARCH_LIMIT` | `10` | Default search results |
| `COGNEE_RELEVANCE_THRESHOLD` | `0.7` | Minimum relevance score |
| `COGNEE_SEARCH_TYPES` | `VECTOR,GRAPH,INSIGHTS,GRAPH_COMPLETION` | Enabled search types |

### Programmatic Configuration

```go
config := &services.CogneeServiceConfig{
    Enabled:                true,
    AutoCognify:            true,
    EnhancePrompts:         true,
    StoreResponses:         true,
    TemporalAwareness:      true,
    EnableFeedbackLoop:     true,
    EnableGraphReasoning:   true,
    EnableCodeIntelligence: true,
    DefaultDataset:         "production",
    RelevanceThreshold:     0.7,
    MaxSearchResults:       10,
    SearchTypes:            []string{"VECTOR", "GRAPH", "INSIGHTS"},
    Timeout:                30 * time.Second,
}

cogneeService := services.NewCogneeServiceWithConfig(config, logger)
```

### Provider Enhancement Configuration

```go
providerConfig := &services.CogneeProviderConfig{
    // Enhancement behavior
    EnhanceBeforeRequest:   true,
    StoreAfterResponse:     true,
    AutoCognifyResponses:   true,
    EnableGraphReasoning:   true,
    EnableCodeIntelligence: true,

    // Context settings
    MaxContextInjection: 2048,
    RelevanceThreshold:  0.7,
    ContextPrefix:       "## Relevant Knowledge:\n",
    ContextSuffix:       "\n\n---\n\n## Request:\n",

    // Dataset settings
    DefaultDataset:    "default",
    UseSessionDataset: true,
    UseUserDataset:    true,
    DatasetHierarchy:  []string{"session", "user", "global", "default"},

    // Performance settings
    AsyncEnhancement:   false,
    EnhancementTimeout: 5 * time.Second,
    CacheEnhancements:  true,
    CacheTTL:           30 * time.Minute,

    // Streaming settings
    EnhanceStreamingPrompt: true,
    StreamingBufferSize:    100,
}

enhanced := services.NewCogneeEnhancedProviderWithConfig(
    "claude-enhanced",
    claudeProvider,
    cogneeService,
    providerConfig,
    logger,
)
```

---

## API Endpoints

### Health Check

```http
GET /api/v1/cognee/health
```

Response:
```json
{
  "healthy": true,
  "ready": true,
  "features": {
    "temporal": true,
    "feedback": true,
    "code_pipeline": true,
    "graph_reasoning": true
  }
}
```

### Add Memory

```http
POST /api/v1/cognee/memory
Content-Type: application/json

{
  "content": "Knowledge to store",
  "dataset": "default",
  "metadata": {
    "source": "user_input",
    "session_id": "abc123"
  }
}
```

### Search Memory

```http
POST /api/v1/cognee/search
Content-Type: application/json

{
  "query": "Search query",
  "dataset": "default",
  "limit": 10,
  "search_types": ["VECTOR", "GRAPH"]
}
```

Response:
```json
{
  "results": [
    {
      "content": "Matched content",
      "score": 0.95,
      "metadata": {...}
    }
  ],
  "search_types_used": ["VECTOR", "GRAPH"],
  "total_results": 5
}
```

### Cognify Content

```http
POST /api/v1/cognee/cognify
Content-Type: application/json

{
  "content": "Content to process",
  "dataset": "default"
}
```

### Get Insights

```http
POST /api/v1/cognee/insights
Content-Type: application/json

{
  "query": "Topic to explore",
  "dataset": "default",
  "depth": 3
}
```

### Process Code

```http
POST /api/v1/cognee/code
Content-Type: application/json

{
  "code": "func main() { ... }",
  "language": "go",
  "dataset": "code-analysis"
}
```

### Provide Feedback

```http
POST /api/v1/cognee/feedback
Content-Type: application/json

{
  "memory_id": "mem-123",
  "relevance_score": 0.95,
  "feedback_text": "Highly relevant response"
}
```

### Dataset Management

```http
# Create dataset
POST /api/v1/cognee/datasets
{"name": "project-alpha", "description": "Project-specific knowledge"}

# List datasets
GET /api/v1/cognee/datasets

# Delete dataset
DELETE /api/v1/cognee/datasets/{name}
```

### Visualize Graph

```http
GET /api/v1/cognee/graph?dataset=default
```

### Get Statistics

```http
GET /api/v1/cognee/stats
```

Response:
```json
{
  "memories_stored": 1234,
  "searches_performed": 5678,
  "cognifications": 234,
  "enhancements": 789,
  "average_enhancement_time": "45ms",
  "average_search_time": "23ms"
}
```

---

## LLM Enhancement

### Automatic Provider Wrapping

All LLM providers can be automatically enhanced with Cognee:

```go
// Wrap all providers
providers := registry.ListProviders()
for _, name := range providers {
    provider, _ := registry.GetProvider(name)
    enhanced := services.NewCogneeEnhancedProvider(name, provider, cogneeService, logger)
    registry.UpdateProvider(name, enhanced)
}
```

### Enhancement Flow

1. **Request Received**: User sends LLM request
2. **Context Search**: Cognee searches for relevant memories
3. **Prompt Enhancement**: Relevant context is injected into prompt
4. **LLM Processing**: Enhanced request sent to LLM provider
5. **Response Storage**: Response is stored in Cognee
6. **Auto-Cognify**: Knowledge is extracted and graph is updated
7. **Response Returned**: Original response returned to user

### Message Enhancement

For chat-style APIs, Cognee enhances messages:

```go
// Original messages
messages := []models.Message{
    {Role: "user", Content: "Explain quantum computing"},
}

// After enhancement
enhancedMessages := []models.Message{
    {Role: "system", Content: "## Relevant Knowledge:\n- Previous discussion about qubits...\n- Related concepts: superposition, entanglement..."},
    {Role: "user", Content: "Explain quantum computing"},
}
```

---

## Docker Compose Setup

### Default Profile (Recommended)

```bash
docker-compose --profile default up -d
```

Starts:
- PostgreSQL (port 5432)
- Redis (port 6379)
- ChromaDB (port 8001)
- Cognee (port 8000)

### Full Profile

```bash
docker-compose --profile full up -d
```

Additionally starts:
- Neo4j graph database (ports 7474, 7687)
- Memgraph (ports 7688, 7445)
- Prometheus (port 9090)
- Grafana (port 3000)
- Mock LLM server (port 8090)
- Ollama (port 11434)

### Graph Database Options

#### NetworkX (Default - In-Memory)
```yaml
COGNEE_GRAPH_DATABASE: networkx
```
- No external service needed
- Best for development and small-scale use

#### Neo4j (Production)
```yaml
COGNEE_GRAPH_DATABASE: neo4j
COGNEE_GRAPH_DATABASE_URL: bolt://neo4j:7687
COGNEE_GRAPH_USERNAME: neo4j
COGNEE_GRAPH_PASSWORD: cognee123
```
- Persistent storage
- Advanced query capabilities
- Visualization UI at port 7474

#### Memgraph (High-Performance)
```yaml
COGNEE_GRAPH_DATABASE: memgraph
COGNEE_GRAPH_DATABASE_URL: bolt://memgraph:7687
COGNEE_GRAPH_USERNAME: memgraph
COGNEE_GRAPH_PASSWORD: cognee123
```
- In-memory with persistence
- Lower latency
- Compatible with Neo4j queries

### Resource Limits

```yaml
cognee:
  deploy:
    resources:
      limits:
        memory: 4G
      reservations:
        memory: 1G
```

---

## Performance Tuning

### Batch Processing

```env
COGNEE_BATCH_SIZE=100          # Increase for bulk operations
COGNEE_MAX_CONCURRENT=20       # Increase for parallel processing
COGNEE_EMBEDDING_BATCH_SIZE=64 # Increase for faster embedding
```

### Caching

```go
config := &CogneeProviderConfig{
    CacheEnhancements: true,
    CacheTTL:          30 * time.Minute,
}
```

### Async Enhancement

For non-blocking enhancement:

```go
config := &CogneeProviderConfig{
    AsyncEnhancement: true,
}
```

### Connection Pooling

Cognee uses HTTP connection pooling. Configure client:

```go
client := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
    },
    Timeout: 30 * time.Second,
}
```

### Memory Optimization

```env
COGNEE_MEMORY_COMPRESSION=true  # Compress older memories
COGNEE_MAX_MEMORY_SIZE=10000    # Limit total memories
COGNEE_CHUNK_SIZE=512           # Smaller chunks for precision
COGNEE_OVERLAP_SIZE=64          # Reduce overlap
```

---

## Best Practices

### 1. Dataset Organization

Organize knowledge into logical datasets:

```go
// Per-user knowledge
dataset := fmt.Sprintf("user_%s", userID)

// Per-session context
dataset := fmt.Sprintf("session_%s", sessionID)

// Project-specific
dataset := "project-alpha"

// Global shared knowledge
dataset := "global"
```

### 2. Relevance Threshold Tuning

```go
// Higher threshold = more precise, fewer results
config.RelevanceThreshold = 0.8

// Lower threshold = more results, potentially less relevant
config.RelevanceThreshold = 0.5
```

### 3. Context Injection Limits

```go
// Limit context to avoid overwhelming the LLM
config.MaxContextInjection = 2048  // characters
```

### 4. Feedback Loop Usage

Actively provide feedback to improve results:

```go
// After user indicates satisfaction
cogneeService.ProvideFeedback(ctx, memoryID, 1.0, "Perfect answer")

// After user indicates dissatisfaction
cogneeService.ProvideFeedback(ctx, memoryID, 0.2, "Not relevant")
```

### 5. Code Intelligence

For code-heavy applications:

```go
config := &CogneeServiceConfig{
    EnableCodeIntelligence: true,
}

// Process important code files
cogneeService.ProcessCode(ctx, sourceCode, "go", "codebase")
```

---

## Troubleshooting

### Cognee Not Healthy

```bash
# Check Cognee logs
docker logs helixagent-cognee

# Verify ChromaDB is running
curl http://localhost:8001/api/v1/heartbeat

# Check PostgreSQL connection
docker exec helixagent-postgres pg_isready
```

### Slow Enhancement

1. Check network latency to Cognee service
2. Reduce `MaxContextInjection` value
3. Enable `AsyncEnhancement`
4. Use caching: `CacheEnhancements = true`

### Memory Not Found

1. Verify dataset name matches
2. Check relevance threshold (lower if needed)
3. Confirm content was cognified: `AutoCognify = true`

### Graph Queries Failing

1. Ensure graph database is running
2. Check `EnableGraphReasoning = true`
3. Verify graph has sufficient data

### High Memory Usage

1. Set `COGNEE_MAX_MEMORY_SIZE` limit
2. Enable `COGNEE_MEMORY_COMPRESSION`
3. Use smaller `COGNEE_CHUNK_SIZE`

### Connection Refused

```bash
# Verify services are running
docker-compose ps

# Check network connectivity
docker exec helixagent-app curl http://cognee:8000/health
```

---

## Code Examples

### Complete Integration Example

```go
package main

import (
    "context"
    "dev.helix.agent/internal/services"
    "dev.helix.agent/internal/llm/providers/claude"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()

    // Create Cognee service
    cogneeConfig := &services.CogneeServiceConfig{
        Enabled:              true,
        BaseURL:              "http://localhost:8000",
        AutoCognify:          true,
        EnhancePrompts:       true,
        EnableGraphReasoning: true,
    }
    cogneeService := services.NewCogneeServiceWithConfig(cogneeConfig, logger)

    // Create LLM provider
    claudeProvider := claude.NewProvider(apiKey, logger)

    // Wrap with Cognee enhancement
    enhanced := services.NewCogneeEnhancedProvider(
        "claude-enhanced",
        claudeProvider,
        cogneeService,
        logger,
    )

    // Use enhanced provider
    ctx := context.Background()
    req := &models.LLMRequest{
        Prompt: "What is HelixAgent?",
    }

    // Cognee automatically:
    // 1. Searches for relevant context
    // 2. Enhances the prompt
    // 3. Stores the response
    // 4. Updates knowledge graph
    resp, _ := enhanced.Complete(ctx, req)

    fmt.Println(resp.Content)
}
```

### Batch Knowledge Import

```go
func importKnowledge(cogneeService *services.CogneeService, documents []string) error {
    ctx := context.Background()

    for _, doc := range documents {
        if err := cogneeService.AddMemory(ctx, doc, "imported", nil); err != nil {
            return err
        }
    }

    // Trigger cognification
    return cogneeService.Cognify(ctx, "imported")
}
```

### Custom Search

```go
func customSearch(cogneeService *services.CogneeService, query string) ([]services.MemoryResult, error) {
    ctx := context.Background()

    results, err := cogneeService.SearchMemory(ctx, query, "default", 20)
    if err != nil {
        return nil, err
    }

    // Filter by custom criteria
    var filtered []services.MemoryResult
    for _, r := range results {
        if r.Score >= 0.8 {
            filtered = append(filtered, r)
        }
    }

    return filtered, nil
}
```

---

## Summary

The Cognee integration provides HelixAgent with:

- **Automatic Enhancement**: Every LLM request is enriched with relevant context
- **Knowledge Persistence**: Responses are stored for future reference
- **Graph Intelligence**: Connected understanding of concepts
- **Self-Improvement**: Feedback loop optimizes retrieval
- **Code Understanding**: Deep analysis of code structures
- **Zero Configuration**: Works out of the box with `docker-compose --profile default up -d`

All features are pre-configured, activated, and ready for maximum performance.
