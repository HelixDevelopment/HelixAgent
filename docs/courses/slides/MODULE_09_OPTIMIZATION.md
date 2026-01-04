# Module 9: Optimization Features

## Presentation Slides Outline

---

## Slide 1: Title Slide

**SuperAgent: Multi-Provider AI Orchestration**

- Module 9: Optimization Features
- Duration: 75 minutes
- LLM Performance and Efficiency

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Implement LLM optimization techniques
- Configure semantic caching
- Use structured output generation
- Apply streaming enhancements

---

## Slide 3: Optimization Framework

**8 Integrated Optimization Tools:**

| Package | Type | Purpose |
|---------|------|---------|
| gptcache | Native Go | Semantic caching |
| outlines | Native Go | Structured output |
| streaming | Native Go | Enhanced streaming |
| sglang | HTTP Client | Prefix caching |
| llamaindex | HTTP Client | Document retrieval |
| langchain | HTTP Client | Task decomposition |
| guidance | HTTP Client | CFG constraints |
| lmql | HTTP Client | Query language |

---

## Slide 4: Optimization Architecture

**System Design:**

```
Request --> +------------------+
            |  Cache Check     |
            +--------+---------+
                     |
              +------v------+
              | Optimization |
              |   Service    |
              +------+------+
                     |
   +--------+--------+--------+--------+
   |        |        |        |        |
+--v--+ +---v---+ +--v--+ +---v---+ +--v--+
|Cache| |Struct | |Stream| |Prefix| |RAG  |
+-----+ +-------+ +------+ +------+ +-----+
```

---

## Slide 5: Semantic Caching with GPTCache

**What is Semantic Caching?**

- Cache similar queries, not just exact matches
- Vector similarity comparison
- Reduces API calls by 30-50%
- Faster response times

*"If someone asked something similar, use that answer"*

---

## Slide 6: GPTCache Configuration

**Setting Up Semantic Cache:**

```yaml
optimization:
  semantic_cache:
    enabled: true
    similarity_threshold: 0.85
    max_entries: 10000
    ttl: 24h
    eviction_policy: lru
```

---

## Slide 7: GPTCache Usage

**Programmatic Usage:**

```go
import "github.com/superagent/superagent/internal/optimization/gptcache"

cache := gptcache.NewCache(config)

// Check cache
if result, found := cache.Get(embedding); found {
    return result // Cache hit!
}

// Generate and cache
response := llm.Generate(prompt)
cache.Set(embedding, response)
```

---

## Slide 8: Cache Performance

**Performance Metrics:**

| Metric | Without Cache | With Cache |
|--------|---------------|------------|
| Avg Latency | 800ms | 50ms |
| API Calls | 100% | 40% |
| Cost | $100 | $40 |

*Actual savings depend on query patterns*

---

## Slide 9: Structured Output with Outlines

**What is Structured Output?**

- Force LLM to output valid JSON
- Schema validation
- Regex pattern matching
- Choice constraints

*"Guarantee the format you need"*

---

## Slide 10: Outlines Configuration

**Schema Definition:**

```go
import "github.com/superagent/superagent/internal/optimization/outlines"

schema := outlines.ObjectSchema(map[string]*outlines.JSONSchema{
    "name":   outlines.StringSchema(),
    "age":    outlines.IntegerSchema(),
    "active": outlines.BooleanSchema(),
})

result, err := svc.GenerateStructured(ctx, prompt, schema, llmFunc)
```

---

## Slide 11: Outlines Examples

**Practical Use Cases:**

```go
// Extract entities
schema := outlines.ObjectSchema(map[string]*outlines.JSONSchema{
    "entities": outlines.ArraySchema(
        outlines.ObjectSchema(map[string]*outlines.JSONSchema{
            "name": outlines.StringSchema(),
            "type": outlines.StringSchema(),
        }),
    ),
})

// Force choice
schema := outlines.ChoiceSchema([]string{
    "positive", "negative", "neutral",
})
```

---

## Slide 12: Enhanced Streaming

**Streaming Improvements:**

- Word/sentence buffering
- Progress tracking
- Rate limiting
- Real-time updates

---

## Slide 13: Streaming Configuration

**Setting Up Enhanced Streaming:**

```yaml
optimization:
  streaming:
    enabled: true
    buffer_type: word  # word, sentence, none
    progress_callback: true
    rate_limit_tokens_per_sec: 100
```

---

## Slide 14: Streaming Usage

**Programmatic Streaming:**

```go
import "github.com/superagent/superagent/internal/optimization/streaming"

streamer := streaming.NewStreamer(config)

// Create channels
inChan := make(chan string)
outChan, getResult := streamer.Stream(ctx, inChan, progressCallback)

// Consume output
for chunk := range outChan {
    fmt.Print(chunk)
}

// Get final result
result := getResult()
```

---

## Slide 15: SGLang Prefix Caching

**RadixAttention for Prefix Reuse:**

- Cache computed attention states
- Reuse for similar prompts
- GPU memory optimization
- Session management

*Requires GPU for best performance*

---

## Slide 16: SGLang Configuration

**Setting Up SGLang:**

```yaml
optimization:
  sglang:
    enabled: true
    endpoint: http://localhost:30000
    model: meta-llama/Llama-2-7b
    gpu_required: true
```

```bash
# Start SGLang server (GPU required)
docker-compose --profile optimization-gpu up -d
```

---

## Slide 17: LlamaIndex for Retrieval

**Document Retrieval Features:**

- HyDE (Hypothetical Document Embeddings)
- Reranking for relevance
- Cognee integration
- Multi-modal support

---

## Slide 18: LlamaIndex Configuration

**Setting Up Retrieval:**

```yaml
optimization:
  llamaindex:
    enabled: true
    endpoint: http://localhost:8012
    index_path: /data/indexes
    cognee_sync: true
    reranking:
      enabled: true
      model: cross-encoder
```

---

## Slide 19: LangChain for Decomposition

**Task Decomposition:**

- Break complex tasks into steps
- Chain execution
- ReAct agents
- Tool integration

---

## Slide 20: LangChain Configuration

**Setting Up Task Decomposition:**

```yaml
optimization:
  langchain:
    enabled: true
    endpoint: http://localhost:8011
    agent_type: react
    max_iterations: 10
    tools:
      - search
      - calculator
      - code_interpreter
```

---

## Slide 21: Guidance for Constraints

**CFG-Based Generation:**

- Context-free grammar constraints
- Regex pattern matching
- Template-based generation
- Deterministic output

---

## Slide 22: LMQL Query Language

**Declarative Constraints:**

```yaml
optimization:
  lmql:
    enabled: true
    endpoint: http://localhost:8014
    decoding: argmax
    max_tokens: 1000
```

Example LMQL query:
```
argmax "Q: {question}\nA: [ANSWER]"
where len(ANSWER) < 100
```

---

## Slide 23: Docker Services

**Starting Optimization Services:**

```bash
# CPU-only optimization
docker-compose --profile optimization up -d

# With GPU support (SGLang)
docker-compose --profile optimization-gpu up -d

# Services started:
# - langchain-server (port 8011)
# - llamaindex-server (port 8012)
# - guidance-server (port 8013)
# - lmql-server (port 8014)
# - sglang (port 30000, GPU only)
```

---

## Slide 24: Complete Optimization Example

**Using Optimization Service:**

```go
import "github.com/superagent/superagent/internal/optimization"

config := optimization.DefaultConfig()
svc, err := optimization.NewService(config)

// Optimize request
optimized, err := svc.OptimizeRequest(ctx, prompt, embedding)

// Generate structured output
result, err := svc.GenerateStructured(ctx, prompt, schema, llmFunc)

// Enhanced streaming
outChan, getResult := svc.StreamEnhanced(ctx, inChan, progressCallback)
```

---

## Slide 25: Optimization Metrics

**Measuring Impact:**

| Metric | Tool | Improvement |
|--------|------|-------------|
| Latency | Cache | 50-90% faster |
| Accuracy | Structured | 100% valid format |
| Cost | Cache | 30-50% savings |
| Throughput | Streaming | 3x UX improvement |

---

## Slide 26: Best Practices

**Optimization Guidelines:**

| Practice | Recommendation |
|----------|----------------|
| Start Simple | Enable cache first |
| Measure | Use metrics to validate |
| Tune Gradually | Adjust thresholds |
| Monitor | Watch for cache misses |
| Test | Verify output quality |

---

## Slide 27: Hands-On Lab

**Lab Exercise 9.1: Optimization Implementation**

Tasks:
1. Enable semantic caching
2. Configure structured output schemas
3. Test enhanced streaming
4. Measure optimization improvements
5. Compare before/after metrics

Time: 30 minutes

---

## Slide 28: Module Summary

**Key Takeaways:**

- 8 optimization tools integrated
- Semantic caching reduces costs 30-50%
- Structured output guarantees format
- Enhanced streaming improves UX
- SGLang for GPU prefix caching
- LlamaIndex for document retrieval
- Start simple, measure, optimize

**Next: Module 10 - Security Best Practices**

---

## Speaker Notes

### Slide 5 Notes
Draw a diagram showing how semantic cache works. Two similar questions should return the same cached answer.

### Slide 9 Notes
Show a real example of structured output. Demonstrate how it prevents malformed JSON responses.

### Slide 25 Notes
Share actual metrics from production systems if available. These numbers can vary significantly based on use case.
