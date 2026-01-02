# LLM Optimization Tools - Portability Assessment

This document provides a detailed assessment of which components from each analyzed repository can be natively ported to Go versus requiring HTTP bridges to Python services.

## Executive Summary

| Tool | Portability | Strategy | Complexity | Priority |
|------|-------------|----------|------------|----------|
| GPTCache | **Native Go Port** | Full port | Medium | P0 |
| Outlines | **Native Go Port** | Full port | Medium-High | P0 |
| llm-streaming | **Native Go Port** | Full port | Low | P1 |
| SGLang | **HTTP Bridge** | Docker service | High | P1 |
| LlamaIndex | **HTTP Bridge** | Docker + Cognee adapter | High | P1 |
| LangChain | **HTTP Bridge** | Docker service | High | P2 |
| Guidance | **HTTP Bridge** | Docker service | Very High | P2 |
| LMQL | **HTTP Bridge** | Docker service | Very High | P3 |

---

## Detailed Analysis

### 1. GPTCache - NATIVE GO PORT

**Recommendation: Full Native Port**

#### Portable Components (Go Implementation)

| Component | Python Source | Go Implementation | Complexity |
|-----------|---------------|-------------------|------------|
| Semantic Similarity | `gptcache/similarity/` | `cosine_similarity.go` | Low |
| L2 Normalization | `gptcache/embedding/` | `normalize.go` | Low |
| LRU Eviction | `gptcache/manager/eviction/` | `eviction_lru.go` | Low |
| TTL Eviction | `gptcache/manager/eviction/` | `eviction_ttl.go` | Low |
| Cache Manager | `gptcache/manager/` | `cache_manager.go` | Medium |
| Embedding Adapter | `gptcache/embedding/` | Use existing `EmbeddingManager` | Low |

#### Core Algorithms to Port

```python
# From gptcache/similarity/simple.py
def cosine_similarity(vec1, vec2):
    dot_product = np.dot(vec1, vec2)
    norm1 = np.linalg.norm(vec1)
    norm2 = np.linalg.norm(vec2)
    return dot_product / (norm1 * norm2)
```

**Go Equivalent:**
```go
func CosineSimilarity(vec1, vec2 []float64) float64 {
    var dot, norm1, norm2 float64
    for i := range vec1 {
        dot += vec1[i] * vec2[i]
        norm1 += vec1[i] * vec1[i]
        norm2 += vec2[i] * vec2[i]
    }
    return dot / (math.Sqrt(norm1) * math.Sqrt(norm2))
}
```

#### Dependencies
- **Vector Search**: Use existing `EmbeddingManager` or integrate with Cognee's vector store
- **Embedding Generation**: Leverage existing provider embeddings
- **Storage**: Redis (already integrated) or in-memory

#### Effort Estimate
- Core similarity functions: 1-2 days
- Cache manager with eviction: 2-3 days
- Integration with RequestService: 1-2 days
- **Total: ~1 week**

---

### 2. Outlines - NATIVE GO PORT

**Recommendation: Full Native Port**

#### Portable Components (Go Implementation)

| Component | Python Source | Go Implementation | Complexity |
|-----------|---------------|-------------------|------------|
| JSON Schema Parser | `outlines/types/` | `schema_parser.go` | Medium |
| Token Masking | `outlines/generate/` | `token_mask.go` | Medium |
| Regex FSM | `outlines/fsm/regex.py` | `regex_fsm.go` | High |
| Schema Validation | `outlines/types/` | `validator.go` | Medium |

#### Core Algorithm: Token Masking

```python
# From outlines/generate/generator.py (simplified)
def get_allowed_tokens(schema, current_state):
    allowed = []
    for token_id, token in tokenizer.vocab.items():
        if schema.accepts(current_state + token):
            allowed.append(token_id)
    return allowed
```

**Go Approach:**
- Pre-compute allowed token sets for each JSON schema state
- Use bitmask for efficient token filtering
- Cache computed masks per schema

#### JSON Schema Constraint Engine

Key files to port:
- `outlines/types/dicts.py` - Object constraints
- `outlines/types/arrays.py` - Array constraints
- `outlines/types/enums.py` - Enum constraints
- `outlines/fsm/json_schema.py` - FSM construction

#### Go Implementation Strategy

```go
type SchemaConstraint interface {
    AllowedTokens(state *GenerationState) []int
    Validate(output string) error
    NextState(state *GenerationState, token string) *GenerationState
}

type JSONSchemaEngine struct {
    schema     *JSONSchema
    tokenizer  Tokenizer
    stateCache map[string][]int // cached allowed tokens per state
}
```

#### Dependencies
- **Tokenizer**: Need Go tokenizer or HTTP call to provider
- **JSON Schema**: Use `encoding/json` with custom validation

#### Effort Estimate
- JSON Schema parser: 2-3 days
- Token masking engine: 3-4 days
- FSM for regex patterns: 3-4 days
- Integration: 2 days
- **Total: ~2 weeks**

---

### 3. llm-streaming - NATIVE GO PORT

**Recommendation: Full Native Port**

#### Portable Components (Go Implementation)

| Component | Python Source | Go Implementation | Complexity |
|-----------|---------------|-------------------|------------|
| SSE Handler | `llm_streaming/sse.py` | `sse_handler.go` | Low |
| Token Buffer | `llm_streaming/buffer.py` | `token_buffer.go` | Low |
| Word Buffer | N/A | `word_buffer.go` | Low |
| Sentence Buffer | N/A | `sentence_buffer.go` | Low |
| Progress Tracker | N/A | `progress_tracker.go` | Low |

#### Core Pattern: Buffered Streaming

```go
type StreamBuffer interface {
    Write(token string) (flush []string, err error)
    Flush() []string
}

type WordBuffer struct {
    buffer    strings.Builder
    delimiter string
}

func (b *WordBuffer) Write(token string) ([]string, error) {
    b.buffer.WriteString(token)
    content := b.buffer.String()

    // Check for word boundaries
    if idx := strings.LastIndex(content, " "); idx > 0 {
        words := content[:idx+1]
        b.buffer.Reset()
        b.buffer.WriteString(content[idx+1:])
        return []string{words}, nil
    }
    return nil, nil
}
```

#### Integration Points
- Wrap existing `CompleteStream()` in all providers
- Add SSE improvements for web clients
- Progress callbacks for long operations

#### Effort Estimate
- Buffer implementations: 1 day
- Progress tracking: 1 day
- Provider integration: 1-2 days
- **Total: 3-4 days**

---

### 4. SGLang - HTTP BRIDGE REQUIRED

**Recommendation: HTTP Bridge + Docker Service**

#### Why Not Native Port?
- **CUDA Dependencies**: RadixAttention requires GPU kernel operations
- **KV-Cache Management**: Deep integration with PyTorch/vLLM internals
- **Model Loading**: Requires Python ML frameworks

#### Portable to Go (Client Only)

| Component | Go Implementation | Notes |
|-----------|-------------------|-------|
| HTTP Client | `sglang_client.go` | REST API calls |
| Session Manager | `session_manager.go` | Track prefix sessions |
| Prefix Tree (metadata) | `prefix_tree.go` | Local prefix tracking |

#### HTTP Bridge API

```go
type SGLangClient struct {
    baseURL    string
    httpClient *http.Client
    sessions   map[string]*PrefixSession
}

func (c *SGLangClient) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
func (c *SGLangClient) WarmPrefix(ctx context.Context, prefix string) (*PrefixSession, error)
func (c *SGLangClient) ExtendSession(ctx context.Context, sessionID string, text string) error
```

#### Docker Service Configuration

```yaml
sglang:
  image: lmsysorg/sglang:latest
  ports:
    - "30000:30000"
  environment:
    - CUDA_VISIBLE_DEVICES=0
  deploy:
    resources:
      reservations:
        devices:
          - driver: nvidia
            count: 1
            capabilities: [gpu]
```

#### Effort Estimate
- Go HTTP client: 2 days
- Session management: 1 day
- Docker configuration: 1 day
- Integration tests: 1-2 days
- **Total: ~1 week**

---

### 5. LlamaIndex - HTTP BRIDGE REQUIRED

**Recommendation: HTTP Bridge + Cognee Adapter**

#### Why Not Native Port?
- **Embedding Models**: Requires sentence-transformers, PyTorch
- **Re-ranking Models**: Cross-encoder models need Python ML stack
- **Query Transformations**: HyDE requires LLM calls with specific prompts

#### Portable to Go (Partial)

| Component | Go Implementation | Notes |
|-----------|-------------------|-------|
| Query Fusion Logic | `query_fusion.go` | Generate multiple queries |
| Result Merging | `result_merger.go` | Reciprocal rank fusion |
| Cognee Adapter | `cognee_adapter.go` | Query Cognee's index |

#### Cognee-Primary Architecture

```go
// LlamaIndex queries Cognee's existing index
type LlamaIndexClient struct {
    endpoint      string
    cogneeService *CogneeService
}

func (c *LlamaIndexClient) QueryWithFusion(ctx context.Context, query string, opts QueryOptions) (*QueryResult, error) {
    // 1. Generate query variations (via Python service)
    variations := c.generateQueryVariations(query)

    // 2. Query Cognee for each variation
    var results []*CogneeSearchResult
    for _, q := range variations {
        r, _ := c.cogneeService.SearchMemory(ctx, q, opts.TopK)
        results = append(results, r...)
    }

    // 3. Merge results with reciprocal rank fusion
    return c.mergeResults(results), nil
}
```

#### HTTP Bridge API

```python
# services/llamaindex/server.py
@app.post("/query/fusion")
async def query_fusion(request: QueryRequest):
    """Generate query variations and perform fusion retrieval"""

@app.post("/query/hyde")
async def query_hyde(request: HyDERequest):
    """Hypothetical document embedding query"""

@app.post("/rerank")
async def rerank(request: RerankRequest):
    """Re-rank results using cross-encoder"""
```

#### Effort Estimate
- Python service: 2-3 days
- Go client: 2 days
- Cognee adapter: 2 days
- Integration: 2 days
- **Total: ~1.5 weeks**

---

### 6. LangChain - HTTP BRIDGE REQUIRED

**Recommendation: HTTP Bridge**

#### Why Not Native Port?
- **Tool Ecosystem**: 100s of pre-built tools require Python
- **Agent Frameworks**: ReAct, Plan-and-Execute deeply Python-based
- **Memory Systems**: Complex Python class hierarchies

#### Portable to Go (Concepts Only)

| Concept | Go Implementation | Notes |
|---------|-------------------|-------|
| Chain Interface | `chain.go` | Abstract chain pattern |
| Simple Sequential | `sequential_chain.go` | Linear chain execution |
| Tool Interface | `tool.go` | Tool abstraction |

#### HTTP Bridge for Complex Patterns

```python
# services/langchain/server.py
@app.post("/chain/execute")
async def execute_chain(request: ChainRequest):
    """Execute a predefined chain"""

@app.post("/agent/react")
async def react_agent(request: AgentRequest):
    """Run ReAct agent loop"""

@app.post("/decompose")
async def decompose_task(request: DecomposeRequest):
    """Break task into subtasks"""
```

#### Go Client

```go
type LangChainClient struct {
    endpoint string
}

func (c *LangChainClient) ExecuteChain(ctx context.Context, chainType string, input map[string]any) (*ChainResult, error)
func (c *LangChainClient) RunReActAgent(ctx context.Context, task string, tools []string) (*AgentResult, error)
func (c *LangChainClient) DecomposeTask(ctx context.Context, task string) ([]SubTask, error)
```

#### Effort Estimate
- Python service: 3-4 days
- Go client: 2 days
- Predefined chains: 2-3 days
- Integration: 2 days
- **Total: ~1.5 weeks**

---

### 7. Guidance - HTTP BRIDGE REQUIRED

**Recommendation: HTTP Bridge**

#### Why Not Native Port?
- **Model Integration**: Deep hooks into transformer generation
- **Grammar Engine**: Complex CFG parsing requires Python AST
- **Interleaved Execution**: Python generator-based control flow

#### Very Limited Go Port Possible

Only metadata and program definitions can be in Go:

```go
type GuidanceProgram struct {
    Template    string
    Variables   map[string]any
    Constraints []Constraint
}

// Must call Python for execution
func (c *GuidanceClient) Execute(ctx context.Context, program *GuidanceProgram) (*GuidanceResult, error)
```

#### HTTP Bridge API

```python
# services/guidance/server.py
@app.post("/execute")
async def execute_program(request: ProgramRequest):
    """Execute a guidance program"""
    program = guidance(request.template)
    result = program(**request.variables)
    return {"output": str(result)}
```

#### Effort Estimate
- Python service: 2-3 days
- Go client: 1-2 days
- Program definitions: 2 days
- Integration: 1-2 days
- **Total: ~1 week**

---

### 8. LMQL - HTTP BRIDGE REQUIRED

**Recommendation: HTTP Bridge**

#### Why Not Native Port?
- **Query Parser**: Custom Python-based parser for LMQL syntax
- **Constraint Solver**: Complex constraint propagation logic
- **Speculative Execution**: Deep integration with tokenizer internals

#### Go Port: Query Builder Only

```go
type LMQLQuery struct {
    Prompt      string
    Constraints []Constraint
    Variables   []Variable
}

func NewQuery(prompt string) *LMQLQueryBuilder {
    return &LMQLQueryBuilder{prompt: prompt}
}

func (b *LMQLQueryBuilder) Where(constraint string) *LMQLQueryBuilder
func (b *LMQLQueryBuilder) Build() *LMQLQuery
```

#### HTTP Bridge API

```python
# services/lmql/server.py
@app.post("/execute")
async def execute_query(request: QueryRequest):
    """Execute an LMQL query"""
    result = await lmql.run(request.query, **request.params)
    return {"output": result}
```

#### Effort Estimate
- Python service: 2-3 days
- Go client + query builder: 2 days
- Integration: 1-2 days
- **Total: ~1 week**

---

## Implementation Priority Matrix

### P0 - Critical (Native Go - Week 1-2)

| Component | Rationale |
|-----------|-----------|
| GPTCache Semantic Cache | Reduces API calls by 40-60% for repeated queries |
| Outlines Structured Output | Eliminates retry loops, ensures valid JSON |

### P1 - High (Week 2-4)

| Component | Rationale |
|-----------|-----------|
| llm-streaming Enhancements | Improved UX, immediate feedback |
| SGLang Prefix Caching | Major latency reduction for multi-turn |
| LlamaIndex + Cognee | Enhanced context retrieval |

### P2 - Medium (Week 4-6)

| Component | Rationale |
|-----------|-----------|
| LangChain Task Decomposition | Complex project handling |
| Guidance CFG Constraints | Advanced output control |

### P3 - Low (Week 6+)

| Component | Rationale |
|-----------|-----------|
| LMQL Query Language | Specialized use cases |

---

## Resource Requirements

### Native Go Development
- 1 Go developer: 3-4 weeks for core components
- Testing: 1 week additional

### Python Services
- 1 Python developer: 2-3 weeks for all services
- Docker configuration: 2-3 days

### Infrastructure
- Redis (already available)
- PostgreSQL (already available)
- GPU server for SGLang (optional but recommended)

---

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Go tokenizer accuracy | Use provider's tokenizer via API |
| Python service latency | Local Docker, connection pooling |
| SGLang GPU requirement | Graceful degradation without GPU |
| Cognee sync conflicts | Cognee-primary architecture |

---

## Conclusion

The recommended approach is:

1. **Native Go Ports** (GPTCache, Outlines, llm-streaming): These provide the highest value with reasonable effort. The core algorithms are mathematically simple and have no Python-specific dependencies.

2. **HTTP Bridges** (SGLang, LlamaIndex, LangChain, Guidance, LMQL): These require deep Python/ML integration that cannot be reasonably ported. Docker services provide clean isolation and easy scaling.

3. **Cognee Integration**: LlamaIndex should query Cognee's existing index rather than creating duplicate storage. This aligns with the user's Cognee-primary requirement.

The total implementation is estimated at 6-8 weeks for a full integration, with the most critical components (semantic caching, structured output) deliverable in the first 2 weeks.
