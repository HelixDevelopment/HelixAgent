# Optimization Package

The `optimization` package provides LLM request optimization, caching, and streaming enhancements to improve performance and reduce costs.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Optimization Pipeline                         │
│  (Pre-processing, Caching, Post-processing)                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────────┐ │
│  │   GPT Cache    │  │    Outlines    │  │     Streaming      │ │
│  │                │  │   (Structured) │  │                    │ │
│  │  Semantic      │  │                │  │  Buffering         │ │
│  │  Caching       │  │  JSON Schema   │  │  Rate Limiting     │ │
│  └────────────────┘  └────────────────┘  │  SSE Writer        │ │
│                                          └────────────────────┘ │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────────┐ │
│  │   LangChain    │  │  LlamaIndex    │  │     SGLang         │ │
│  │                │  │                │  │                    │ │
│  │  Chain         │  │  RAG           │  │  Structured        │ │
│  │  Optimization  │  │  Optimization  │  │  Generation        │ │
│  └────────────────┘  └────────────────┘  └────────────────────┘ │
│                                                                  │
│  ┌────────────────┐  ┌────────────────┐                         │
│  │   Guidance     │  │     LMQL       │                         │
│  │                │  │                │                         │
│  │  Template      │  │  Query         │                         │
│  │  Optimization  │  │  Language      │                         │
│  └────────────────┘  └────────────────┘                         │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│  Optimizer │ Pipeline │ Metrics │ Config                        │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### Optimizer (`optimizer.go`)

Central optimization coordinator:

```go
optimizer := optimization.NewOptimizer(config, logger)

// Optimize a request before sending to LLM
optimizedReq, err := optimizer.OptimizeRequest(ctx, request)

// Process response for caching
err = optimizer.ProcessResponse(ctx, request, response)
```

### Pipeline (`pipeline.go`)

Chain multiple optimizations:

```go
pipeline := optimization.NewPipeline(
    optimization.WithGPTCache(cacheConfig),
    optimization.WithOutlines(schemaConfig),
    optimization.WithStreaming(streamConfig),
)

response, err := pipeline.Execute(ctx, request)
```

### Metrics (`metrics.go`)

Performance tracking:

```go
metrics := optimization.NewMetrics()

// Record optimization stats
metrics.RecordCacheHit()
metrics.RecordCacheMiss()
metrics.RecordLatency(duration)
metrics.RecordTokensSaved(count)

// Get statistics
stats := metrics.GetStats()
```

### Config (`config.go`)

Optimization configuration:

```go
config := optimization.Config{
    EnableCache:     true,
    CacheTTL:        time.Hour,
    EnableStreaming: true,
    BufferSize:      1024,
    RateLimit:       100,
}
```

## Subpackages

### GPT Cache (`gptcache/`)

Semantic caching for LLM responses:

```go
cache := gptcache.NewCache(gptcache.Config{
    EmbeddingModel: "text-embedding-ada-002",
    SimilarityThreshold: 0.95,
    TTL: time.Hour,
})

// Check cache
cached, hit := cache.Get(ctx, request)
if hit {
    return cached, nil
}

// Store in cache
cache.Set(ctx, request, response)
```

### Outlines (`outlines/`)

Structured output generation:

```go
outliner := outlines.NewOutliner(outlines.Config{
    Schema: jsonSchema,
    StrictMode: true,
})

// Ensure response matches schema
validated, err := outliner.ValidateResponse(response)
```

### Streaming (`streaming/`)

Stream processing utilities:

```go
// Buffer types
buffer := streaming.NewCharacterBuffer(100)
buffer := streaming.NewWordBuffer(50)
buffer := streaming.NewSentenceBuffer()
buffer := streaming.NewParagraphBuffer()
buffer := streaming.NewTokenBuffer(100)

// Rate limiter
limiter := streaming.NewRateLimiter(streaming.RateLimiterConfig{
    TokensPerSecond: 100,
    BurstSize:       50,
})

// SSE writer
writer := streaming.NewSSEWriter(w, streaming.SSEConfig{
    RetryInterval: 3000,
    KeepAlive:     true,
})

// Enhanced streamer
streamer := streaming.NewEnhancedStreamer(streaming.StreamerConfig{
    Buffer:      buffer,
    RateLimiter: limiter,
    Writer:      writer,
})
```

### LangChain (`langchain/`)

LangChain optimization integration:

```go
optimizer := langchain.NewOptimizer(langchain.Config{
    ChainType: "stuff",
    MaxTokens: 4096,
})

optimized, err := optimizer.Optimize(ctx, chain)
```

### LlamaIndex (`llamaindex/`)

LlamaIndex RAG optimization:

```go
optimizer := llamaindex.NewOptimizer(llamaindex.Config{
    IndexType: "vector",
    ChunkSize: 512,
})

optimized, err := optimizer.Optimize(ctx, index)
```

### SGLang (`sglang/`)

Structured generation optimization:

```go
optimizer := sglang.NewOptimizer(sglang.Config{
    Grammar: grammar,
    MaxTokens: 1000,
})

response, err := optimizer.Generate(ctx, prompt)
```

### Guidance (`guidance/`)

Template-based optimization:

```go
optimizer := guidance.NewOptimizer(guidance.Config{
    Template: template,
    Variables: vars,
})

response, err := optimizer.Execute(ctx, request)
```

### LMQL (`lmql/`)

Query language optimization:

```go
optimizer := lmql.NewOptimizer(lmql.Config{
    Query: query,
    Constraints: constraints,
})

response, err := optimizer.Execute(ctx, request)
```

## Files

| File | Description |
|------|-------------|
| `config.go` | Configuration structures |
| `optimizer.go` | Central optimizer |
| `pipeline.go` | Optimization pipeline |
| `metrics.go` | Performance metrics |
| `gptcache/` | Semantic caching |
| `outlines/` | Structured output |
| `streaming/` | Stream processing |
| `langchain/` | LangChain integration |
| `llamaindex/` | LlamaIndex integration |
| `sglang/` | SGLang integration |
| `guidance/` | Guidance templates |
| `lmql/` | LMQL queries |

## Testing

```bash
go test -v ./internal/optimization/...
```

Tests cover:
- Cache hit/miss scenarios
- Pipeline execution
- Streaming buffers and rate limiting
- Schema validation
- Performance metrics
- Concurrent access
