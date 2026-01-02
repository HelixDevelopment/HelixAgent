# GPTCache - Deep Analysis Report

## Repository Overview

- **Repository**: https://github.com/zilliztech/GPTCache
- **Language**: Python
- **Purpose**: Semantic cache for LLM queries to reduce redundant API calls
- **License**: Apache 2.0

## Core Architecture

### Directory Structure

```
gptcache/
├── adapter/           # LLM provider adapters (OpenAI, Anthropic, etc.)
├── embedding/         # Embedding generation (OpenAI, Hugging Face, etc.)
├── similarity/        # Similarity evaluation algorithms
├── manager/           # Cache management and eviction
│   ├── eviction/      # LRU, TTL, and relevance-based eviction
│   └── scalar_data/   # Scalar storage backends
├── processor/         # Query pre/post processing
└── utils/             # Utility functions
```

### Key Components

#### 1. Similarity Evaluation (`gptcache/similarity/`)

**Core Algorithm: Cosine Similarity**

```python
# From gptcache/similarity/simple.py
def cosine_similarity(vec1: np.ndarray, vec2: np.ndarray) -> float:
    """Compute cosine similarity between two vectors."""
    dot_product = np.dot(vec1, vec2)
    norm1 = np.linalg.norm(vec1)
    norm2 = np.linalg.norm(vec2)
    if norm1 == 0 or norm2 == 0:
        return 0.0
    return dot_product / (norm1 * norm2)
```

**Euclidean Distance**

```python
def euclidean_distance(vec1: np.ndarray, vec2: np.ndarray) -> float:
    """Compute Euclidean distance between two vectors."""
    return np.linalg.norm(vec1 - vec2)
```

**Similarity Threshold Logic**

```python
# From gptcache/similarity/evaluation.py
class SearchDistanceEvaluation:
    def __init__(self, max_distance: float = 0.3):
        self.max_distance = max_distance

    def evaluation(self, distance: float) -> float:
        """Convert distance to similarity score [0, 1]."""
        if distance > self.max_distance:
            return 0.0
        return 1.0 - (distance / self.max_distance)
```

#### 2. Embedding Management (`gptcache/embedding/`)

**L2 Normalization**

```python
# From gptcache/embedding/base.py
def normalize_embedding(embedding: np.ndarray) -> np.ndarray:
    """L2 normalize embedding vector."""
    norm = np.linalg.norm(embedding)
    if norm == 0:
        return embedding
    return embedding / norm
```

**Embedding Interface**

```python
class BaseEmbedding:
    def to_embeddings(self, data: str) -> np.ndarray:
        """Convert text to embedding vector."""
        raise NotImplementedError

    @property
    def dimension(self) -> int:
        """Return embedding dimension."""
        raise NotImplementedError
```

#### 3. Cache Manager (`gptcache/manager/`)

**Cache Entry Structure**

```python
@dataclass
class CacheData:
    question: str
    answer: str
    embedding: np.ndarray
    metadata: Dict[str, Any]
    created_at: float
    accessed_at: float
    access_count: int
```

**Cache Operations**

```python
class CacheManager:
    def __init__(self,
                 vector_store: VectorStore,
                 scalar_store: ScalarStore,
                 eviction: EvictionStrategy):
        self.vector_store = vector_store
        self.scalar_store = scalar_store
        self.eviction = eviction

    def get(self, embedding: np.ndarray, threshold: float) -> Optional[CacheData]:
        """Search for similar cached query."""
        results = self.vector_store.search(embedding, top_k=1)
        if results and results[0].distance < threshold:
            data = self.scalar_store.get(results[0].id)
            self.eviction.update_access(results[0].id)
            return data
        return None

    def put(self, question: str, answer: str, embedding: np.ndarray):
        """Store query-response pair."""
        entry_id = self.vector_store.add(embedding)
        self.scalar_store.put(entry_id, CacheData(
            question=question,
            answer=answer,
            embedding=embedding,
            metadata={},
            created_at=time.time(),
            accessed_at=time.time(),
            access_count=1
        ))
        self.eviction.add(entry_id)
```

#### 4. Eviction Strategies (`gptcache/manager/eviction/`)

**LRU Eviction**

```python
class LRUEviction:
    def __init__(self, max_size: int):
        self.max_size = max_size
        self.order = collections.OrderedDict()

    def add(self, key: str):
        if key in self.order:
            self.order.move_to_end(key)
        else:
            self.order[key] = True
            if len(self.order) > self.max_size:
                oldest = next(iter(self.order))
                del self.order[oldest]
                return oldest
        return None

    def update_access(self, key: str):
        if key in self.order:
            self.order.move_to_end(key)
```

**TTL Eviction**

```python
class TTLEviction:
    def __init__(self, ttl_seconds: int):
        self.ttl = ttl_seconds
        self.entries = {}  # key -> created_at

    def add(self, key: str):
        self.entries[key] = time.time()

    def get_expired(self) -> List[str]:
        now = time.time()
        expired = []
        for key, created_at in self.entries.items():
            if now - created_at > self.ttl:
                expired.append(key)
        return expired
```

**Relevance-Based Eviction**

```python
class RelevanceEviction:
    """Evict based on access frequency and recency."""

    def __init__(self, max_size: int, decay_factor: float = 0.95):
        self.max_size = max_size
        self.decay = decay_factor
        self.scores = {}  # key -> relevance_score

    def update_access(self, key: str):
        # Decay all scores
        for k in self.scores:
            self.scores[k] *= self.decay
        # Boost accessed key
        self.scores[key] = self.scores.get(key, 0) + 1.0

    def get_eviction_candidate(self) -> str:
        return min(self.scores, key=self.scores.get)
```

### Vector Store Backends

**FAISS Integration**

```python
class FAISSVectorStore:
    def __init__(self, dimension: int):
        self.index = faiss.IndexFlatL2(dimension)
        self.id_map = {}

    def add(self, embedding: np.ndarray) -> str:
        entry_id = str(uuid.uuid4())
        idx = self.index.ntotal
        self.index.add(embedding.reshape(1, -1))
        self.id_map[idx] = entry_id
        return entry_id

    def search(self, embedding: np.ndarray, top_k: int) -> List[SearchResult]:
        distances, indices = self.index.search(
            embedding.reshape(1, -1), top_k
        )
        return [
            SearchResult(id=self.id_map[idx], distance=dist)
            for idx, dist in zip(indices[0], distances[0])
            if idx >= 0
        ]
```

## Go Port Strategy

### Core Components to Implement

```go
// internal/optimization/gptcache/semantic_cache.go

package gptcache

import (
    "context"
    "math"
    "sync"
    "time"
)

// CacheEntry represents a cached query-response pair
type CacheEntry struct {
    ID          string
    Question    string
    Answer      string
    Embedding   []float64
    Metadata    map[string]any
    CreatedAt   time.Time
    AccessedAt  time.Time
    AccessCount int
}

// SemanticCache provides semantic similarity-based caching
type SemanticCache struct {
    mu          sync.RWMutex
    entries     map[string]*CacheEntry
    embeddings  [][]float64
    entryIDs    []string
    eviction    EvictionStrategy
    config      *Config
}

// Config holds cache configuration
type Config struct {
    SimilarityThreshold float64
    MaxEntries          int
    TTL                 time.Duration
    EmbeddingDimension  int
}

// Get searches for a semantically similar cached entry
func (c *SemanticCache) Get(ctx context.Context, embedding []float64, threshold float64) (*CacheEntry, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    bestMatch := -1
    bestSimilarity := 0.0

    for i, cached := range c.embeddings {
        similarity := CosineSimilarity(embedding, cached)
        if similarity > bestSimilarity && similarity >= threshold {
            bestSimilarity = similarity
            bestMatch = i
        }
    }

    if bestMatch < 0 {
        return nil, nil
    }

    entry := c.entries[c.entryIDs[bestMatch]]
    entry.AccessedAt = time.Now()
    entry.AccessCount++
    c.eviction.UpdateAccess(entry.ID)

    return entry, nil
}

// Set stores a query-response pair in the cache
func (c *SemanticCache) Set(ctx context.Context, question, answer string, embedding []float64) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    entry := &CacheEntry{
        ID:          generateID(),
        Question:    question,
        Answer:      answer,
        Embedding:   embedding,
        CreatedAt:   time.Now(),
        AccessedAt:  time.Now(),
        AccessCount: 1,
    }

    c.entries[entry.ID] = entry
    c.embeddings = append(c.embeddings, embedding)
    c.entryIDs = append(c.entryIDs, entry.ID)

    // Check for eviction
    if evicted := c.eviction.Add(entry.ID); evicted != "" {
        c.removeEntry(evicted)
    }

    return nil
}
```

### Similarity Functions

```go
// internal/optimization/gptcache/similarity.go

package gptcache

import "math"

// CosineSimilarity computes cosine similarity between two vectors
func CosineSimilarity(vec1, vec2 []float64) float64 {
    if len(vec1) != len(vec2) {
        return 0
    }

    var dot, norm1, norm2 float64
    for i := range vec1 {
        dot += vec1[i] * vec2[i]
        norm1 += vec1[i] * vec1[i]
        norm2 += vec2[i] * vec2[i]
    }

    norm1 = math.Sqrt(norm1)
    norm2 = math.Sqrt(norm2)

    if norm1 == 0 || norm2 == 0 {
        return 0
    }

    return dot / (norm1 * norm2)
}

// EuclideanDistance computes Euclidean distance between two vectors
func EuclideanDistance(vec1, vec2 []float64) float64 {
    if len(vec1) != len(vec2) {
        return math.MaxFloat64
    }

    var sum float64
    for i := range vec1 {
        diff := vec1[i] - vec2[i]
        sum += diff * diff
    }

    return math.Sqrt(sum)
}

// NormalizeL2 performs L2 normalization on a vector
func NormalizeL2(vec []float64) []float64 {
    var norm float64
    for _, v := range vec {
        norm += v * v
    }
    norm = math.Sqrt(norm)

    if norm == 0 {
        return vec
    }

    result := make([]float64, len(vec))
    for i, v := range vec {
        result[i] = v / norm
    }
    return result
}
```

### Eviction Strategies

```go
// internal/optimization/gptcache/eviction.go

package gptcache

import (
    "container/list"
    "sync"
    "time"
)

// EvictionStrategy defines the eviction interface
type EvictionStrategy interface {
    Add(key string) (evicted string)
    UpdateAccess(key string)
    Remove(key string)
}

// LRUEviction implements LRU eviction policy
type LRUEviction struct {
    mu      sync.Mutex
    maxSize int
    order   *list.List
    index   map[string]*list.Element
}

func NewLRUEviction(maxSize int) *LRUEviction {
    return &LRUEviction{
        maxSize: maxSize,
        order:   list.New(),
        index:   make(map[string]*list.Element),
    }
}

func (e *LRUEviction) Add(key string) string {
    e.mu.Lock()
    defer e.mu.Unlock()

    if elem, exists := e.index[key]; exists {
        e.order.MoveToFront(elem)
        return ""
    }

    e.index[key] = e.order.PushFront(key)

    if e.order.Len() > e.maxSize {
        oldest := e.order.Back()
        if oldest != nil {
            evicted := oldest.Value.(string)
            e.order.Remove(oldest)
            delete(e.index, evicted)
            return evicted
        }
    }

    return ""
}

func (e *LRUEviction) UpdateAccess(key string) {
    e.mu.Lock()
    defer e.mu.Unlock()

    if elem, exists := e.index[key]; exists {
        e.order.MoveToFront(elem)
    }
}

func (e *LRUEviction) Remove(key string) {
    e.mu.Lock()
    defer e.mu.Unlock()

    if elem, exists := e.index[key]; exists {
        e.order.Remove(elem)
        delete(e.index, key)
    }
}

// TTLEviction implements TTL-based eviction
type TTLEviction struct {
    mu      sync.Mutex
    ttl     time.Duration
    entries map[string]time.Time
}

func NewTTLEviction(ttl time.Duration) *TTLEviction {
    e := &TTLEviction{
        ttl:     ttl,
        entries: make(map[string]time.Time),
    }
    go e.cleanupLoop()
    return e
}

func (e *TTLEviction) Add(key string) string {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.entries[key] = time.Now()
    return ""
}

func (e *TTLEviction) UpdateAccess(key string) {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.entries[key] = time.Now()
}

func (e *TTLEviction) Remove(key string) {
    e.mu.Lock()
    defer e.mu.Unlock()
    delete(e.entries, key)
}

func (e *TTLEviction) GetExpired() []string {
    e.mu.Lock()
    defer e.mu.Unlock()

    now := time.Now()
    var expired []string
    for key, created := range e.entries {
        if now.Sub(created) > e.ttl {
            expired = append(expired, key)
        }
    }
    return expired
}

func (e *TTLEviction) cleanupLoop() {
    ticker := time.NewTicker(time.Minute)
    for range ticker.C {
        expired := e.GetExpired()
        for _, key := range expired {
            e.Remove(key)
        }
    }
}
```

## Integration with SuperAgent

### RequestService Integration

```go
// In internal/services/request_service.go

func (s *RequestService) ProcessRequest(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
    // 1. Generate embedding for query
    embedding, err := s.embeddingManager.GetEmbedding(ctx, req.Prompt)
    if err != nil {
        // Log but don't fail - continue without cache
        log.Printf("Failed to get embedding: %v", err)
    }

    // 2. Check semantic cache
    if embedding != nil {
        cached, err := s.semanticCache.Get(ctx, embedding, s.config.SimilarityThreshold)
        if err == nil && cached != nil {
            return &models.LLMResponse{
                Content:   cached.Answer,
                FromCache: true,
                CacheKey:  cached.ID,
            }, nil
        }
    }

    // 3. Process request normally
    response, err := s.processWithProviders(ctx, req)
    if err != nil {
        return nil, err
    }

    // 4. Store in semantic cache
    if embedding != nil {
        s.semanticCache.Set(ctx, req.Prompt, response.Content, embedding)
    }

    return response, nil
}
```

## Performance Considerations

### Optimization Techniques

1. **SIMD for Vector Operations**: Use `gonum/blas` for vectorized similarity computation
2. **Index Structures**: For large caches (>10K entries), implement approximate nearest neighbor search
3. **Batch Processing**: Batch embedding generation for multiple queries
4. **Memory Pool**: Reuse float64 slices for embeddings

### Benchmarks (Expected)

| Operation | Entries | Time |
|-----------|---------|------|
| Get (linear scan) | 1,000 | ~1ms |
| Get (linear scan) | 10,000 | ~10ms |
| Get (with index) | 100,000 | ~5ms |
| Set | N/A | ~0.1ms |

## Test Coverage Requirements

```go
// tests/optimization/unit/gptcache/semantic_cache_test.go

func TestSemanticCache_Get_ExactMatch(t *testing.T)
func TestSemanticCache_Get_SimilarMatch(t *testing.T)
func TestSemanticCache_Get_NoMatch(t *testing.T)
func TestSemanticCache_Get_BelowThreshold(t *testing.T)
func TestSemanticCache_Set_New(t *testing.T)
func TestSemanticCache_Set_Eviction(t *testing.T)
func TestSemanticCache_Concurrent(t *testing.T)

func TestCosineSimilarity_Identical(t *testing.T)
func TestCosineSimilarity_Orthogonal(t *testing.T)
func TestCosineSimilarity_ZeroVector(t *testing.T)
func TestCosineSimilarity_DifferentLength(t *testing.T)

func TestLRUEviction_Basic(t *testing.T)
func TestLRUEviction_UpdateAccess(t *testing.T)
func TestLRUEviction_MaxSize(t *testing.T)

func TestTTLEviction_Expiry(t *testing.T)
func TestTTLEviction_Refresh(t *testing.T)
```

## Conclusion

GPTCache is an excellent candidate for native Go implementation. The core algorithms (cosine similarity, LRU eviction, TTL management) are straightforward mathematical operations without Python-specific dependencies. The main integration point is embedding generation, which can leverage the existing `EmbeddingManager` in SuperAgent.

**Estimated Implementation Time**: 1 week
**Risk Level**: Low
**Dependencies**: None (uses existing EmbeddingManager)
