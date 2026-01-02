# SGLang - Deep Analysis Report

## Repository Overview

- **Repository**: https://github.com/sgl-project/sglang
- **Language**: Python/CUDA
- **Purpose**: High-performance serving framework with RadixAttention prefix caching
- **License**: Apache 2.0

## Core Architecture

### Directory Structure

```
sglang/
├── python/sglang/
│   ├── srt/                    # SGL Runtime
│   │   ├── server.py           # HTTP server
│   │   ├── managers/           # Request management
│   │   │   ├── schedule_batch.py
│   │   │   └── radix_cache.py  # RadixAttention implementation
│   │   ├── model_executor/     # Model inference
│   │   └── memory_pool/        # KV-cache memory management
│   ├── lang/                   # SGLang DSL
│   │   ├── ir.py               # Intermediate representation
│   │   └── interpreter.py      # Program execution
│   └── api/                    # Client API
├── rust/                       # Rust components
└── test/                       # Test suite
```

### Key Components

#### 1. RadixAttention Cache (`srt/managers/radix_cache.py`)

**Radix Tree Data Structure**

```python
class RadixNode:
    """Node in the radix tree for prefix caching."""

    def __init__(self, key: bytes = b"", value: Any = None):
        self.key = key          # Partial key at this node
        self.value = value      # KV-cache reference if leaf
        self.children = {}      # Children nodes
        self.ref_count = 0      # Reference count for eviction
        self.last_access = 0    # For LRU eviction


class RadixCache:
    """Radix tree-based KV-cache for prefix sharing."""

    def __init__(self, max_memory_gb: float):
        self.root = RadixNode()
        self.max_memory = max_memory_gb * 1024**3
        self.current_memory = 0
        self.token_to_kv = {}   # token_ids -> KV-cache location

    def insert(self, token_ids: List[int], kv_cache: KVCache) -> str:
        """Insert a sequence and its KV-cache."""
        key = self._tokens_to_key(token_ids)
        node = self._find_or_create_node(key)
        node.value = kv_cache
        node.ref_count += 1
        node.last_access = time.time()

        self.current_memory += kv_cache.memory_size
        self._evict_if_needed()

        return node.id

    def match_prefix(self, token_ids: List[int]) -> Tuple[int, Optional[KVCache]]:
        """Find longest matching prefix in cache."""
        key = self._tokens_to_key(token_ids)
        matched_len = 0
        matched_kv = None

        current = self.root
        pos = 0

        while pos < len(key):
            # Find child that matches
            found = False
            for child_key, child_node in current.children.items():
                common_len = self._common_prefix_len(key[pos:], child_key)
                if common_len > 0:
                    matched_len += common_len
                    if child_node.value is not None:
                        matched_kv = child_node.value
                        child_node.last_access = time.time()
                    current = child_node
                    pos += common_len
                    found = True
                    break

            if not found:
                break

        return matched_len, matched_kv

    def _find_or_create_node(self, key: bytes) -> RadixNode:
        """Navigate or create path in tree."""
        current = self.root
        pos = 0

        while pos < len(key):
            remaining = key[pos:]
            found = False

            for child_key, child_node in current.children.items():
                common_len = self._common_prefix_len(remaining, child_key)

                if common_len == len(child_key):
                    # Full match, continue down
                    current = child_node
                    pos += common_len
                    found = True
                    break
                elif common_len > 0:
                    # Partial match, split node
                    new_node = RadixNode(child_key[:common_len])
                    child_node.key = child_key[common_len:]
                    new_node.children[child_node.key] = child_node
                    current.children[child_key[:common_len]] = new_node
                    del current.children[child_key]
                    current = new_node
                    pos += common_len
                    found = True
                    break

            if not found:
                # Create new node
                new_node = RadixNode(remaining)
                current.children[remaining] = new_node
                return new_node

        return current

    def _evict_if_needed(self):
        """Evict least recently used entries if over memory limit."""
        while self.current_memory > self.max_memory:
            # Find LRU leaf node with ref_count == 0
            lru_node = self._find_lru_evictable()
            if lru_node is None:
                break
            self._evict_node(lru_node)

    def _tokens_to_key(self, token_ids: List[int]) -> bytes:
        """Convert token IDs to bytes key."""
        return b"".join(t.to_bytes(4, 'big') for t in token_ids)

    def _common_prefix_len(self, a: bytes, b: bytes) -> int:
        """Find length of common prefix."""
        min_len = min(len(a), len(b))
        for i in range(min_len):
            if a[i] != b[i]:
                return i
        return min_len
```

#### 2. KV-Cache Memory Pool (`srt/memory_pool/`)

```python
class KVCachePool:
    """Memory pool for KV-cache blocks."""

    def __init__(self, num_layers: int, num_heads: int, head_dim: int,
                 max_seq_len: int, dtype: torch.dtype, device: str):
        self.num_layers = num_layers
        self.num_heads = num_heads
        self.head_dim = head_dim
        self.block_size = 16  # Tokens per block

        # Pre-allocate blocks
        self.num_blocks = max_seq_len // self.block_size
        self.k_cache = torch.empty(
            (self.num_blocks, num_layers, self.block_size, num_heads, head_dim),
            dtype=dtype, device=device
        )
        self.v_cache = torch.empty_like(self.k_cache)

        # Free block tracking
        self.free_blocks = list(range(self.num_blocks))

    def allocate(self, num_tokens: int) -> List[int]:
        """Allocate blocks for tokens."""
        num_blocks_needed = (num_tokens + self.block_size - 1) // self.block_size
        if len(self.free_blocks) < num_blocks_needed:
            raise MemoryError("Insufficient KV-cache memory")

        allocated = []
        for _ in range(num_blocks_needed):
            allocated.append(self.free_blocks.pop())
        return allocated

    def free(self, block_ids: List[int]):
        """Return blocks to pool."""
        self.free_blocks.extend(block_ids)

    def copy_from_prefix(self, src_blocks: List[int], dst_blocks: List[int],
                         prefix_len: int):
        """Copy KV-cache from prefix to new request."""
        num_full_blocks = prefix_len // self.block_size

        for i in range(num_full_blocks):
            self.k_cache[dst_blocks[i]] = self.k_cache[src_blocks[i]]
            self.v_cache[dst_blocks[i]] = self.v_cache[src_blocks[i]]

        # Handle partial block
        partial_tokens = prefix_len % self.block_size
        if partial_tokens > 0:
            src_idx = num_full_blocks
            dst_idx = num_full_blocks
            self.k_cache[dst_blocks[dst_idx], :, :partial_tokens] = \
                self.k_cache[src_blocks[src_idx], :, :partial_tokens]
            self.v_cache[dst_blocks[dst_idx], :, :partial_tokens] = \
                self.v_cache[src_blocks[src_idx], :, :partial_tokens]
```

#### 3. Batch Scheduler (`srt/managers/schedule_batch.py`)

```python
class BatchScheduler:
    """Schedule requests into batches with prefix sharing."""

    def __init__(self, radix_cache: RadixCache, kv_pool: KVCachePool):
        self.radix_cache = radix_cache
        self.kv_pool = kv_pool
        self.pending_requests = []
        self.running_requests = []

    def add_request(self, request: Request):
        """Add request to pending queue."""
        # Find prefix match
        prefix_len, prefix_kv = self.radix_cache.match_prefix(request.token_ids)
        request.prefix_len = prefix_len
        request.prefix_kv = prefix_kv
        self.pending_requests.append(request)

    def schedule_batch(self, max_batch_tokens: int) -> Batch:
        """Create next batch to process."""
        batch_requests = []
        batch_tokens = 0

        # Sort by prefix length to maximize sharing
        self.pending_requests.sort(key=lambda r: r.prefix_len, reverse=True)

        for request in self.pending_requests[:]:
            new_tokens = len(request.token_ids) - request.prefix_len
            if batch_tokens + new_tokens <= max_batch_tokens:
                batch_requests.append(request)
                batch_tokens += new_tokens
                self.pending_requests.remove(request)

        return Batch(batch_requests)
```

#### 4. Session Management API

```python
class SGLangSession:
    """Multi-turn conversation session with prefix caching."""

    def __init__(self, server_url: str):
        self.server_url = server_url
        self.session_id = str(uuid.uuid4())
        self.conversation_history = []
        self.prefix_token_ids = []

    async def generate(self, prompt: str, **kwargs) -> str:
        """Generate with prefix caching."""
        # Tokenize new prompt
        new_tokens = self._tokenize(prompt)

        # Full token sequence
        all_tokens = self.prefix_token_ids + new_tokens

        response = await self._call_server({
            "session_id": self.session_id,
            "token_ids": all_tokens,
            "prefix_len": len(self.prefix_token_ids),
            **kwargs
        })

        # Update prefix for next turn
        output_tokens = response["token_ids"]
        self.prefix_token_ids = all_tokens + output_tokens
        self.conversation_history.append({
            "input": prompt,
            "output": response["text"]
        })

        return response["text"]

    async def fork(self) -> "SGLangSession":
        """Create a new session sharing current prefix."""
        new_session = SGLangSession(self.server_url)
        new_session.prefix_token_ids = self.prefix_token_ids.copy()
        new_session.conversation_history = self.conversation_history.copy()
        return new_session
```

## Go Client Implementation

Since SGLang requires GPU and deep PyTorch integration, we implement an HTTP client only.

### Client Implementation

```go
// internal/optimization/sglang/client.go

package sglang

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// SGLangClient communicates with SGLang server
type SGLangClient struct {
    baseURL    string
    httpClient *http.Client
    sessions   map[string]*Session
}

// Config holds client configuration
type Config struct {
    BaseURL        string
    Timeout        time.Duration
    MaxConnections int
}

// NewSGLangClient creates a new client
func NewSGLangClient(config *Config) *SGLangClient {
    return &SGLangClient{
        baseURL: config.BaseURL,
        httpClient: &http.Client{
            Timeout: config.Timeout,
            Transport: &http.Transport{
                MaxIdleConns:        config.MaxConnections,
                MaxIdleConnsPerHost: config.MaxConnections,
            },
        },
        sessions: make(map[string]*Session),
    }
}

// GenerateRequest is the request for generation
type GenerateRequest struct {
    SessionID     string   `json:"session_id,omitempty"`
    Prompt        string   `json:"prompt"`
    MaxTokens     int      `json:"max_tokens,omitempty"`
    Temperature   float64  `json:"temperature,omitempty"`
    TopP          float64  `json:"top_p,omitempty"`
    StopSequences []string `json:"stop,omitempty"`
    Stream        bool     `json:"stream,omitempty"`
}

// GenerateResponse is the response from generation
type GenerateResponse struct {
    Text         string  `json:"text"`
    TokenCount   int     `json:"token_count"`
    PrefixHit    bool    `json:"prefix_hit"`
    PrefixLen    int     `json:"prefix_len"`
    Latency      float64 `json:"latency_ms"`
}

// Generate sends a generation request
func (c *SGLangClient) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/generate", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
    }

    var result GenerateResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// GenerateStream sends a streaming generation request
func (c *SGLangClient) GenerateStream(ctx context.Context, req *GenerateRequest) (<-chan *StreamChunk, error) {
    req.Stream = true

    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/generate", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Accept", "text/event-stream")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        resp.Body.Close()
        return nil, fmt.Errorf("server error: %d", resp.StatusCode)
    }

    out := make(chan *StreamChunk)

    go func() {
        defer close(out)
        defer resp.Body.Close()

        decoder := json.NewDecoder(resp.Body)
        for {
            var chunk StreamChunk
            if err := decoder.Decode(&chunk); err != nil {
                if err != io.EOF {
                    out <- &StreamChunk{Error: err}
                }
                return
            }

            select {
            case out <- &chunk:
            case <-ctx.Done():
                return
            }

            if chunk.Done {
                return
            }
        }
    }()

    return out, nil
}

// StreamChunk represents a streaming chunk
type StreamChunk struct {
    Text  string `json:"text"`
    Done  bool   `json:"done"`
    Error error  `json:"-"`
}
```

### Session Management

```go
// internal/optimization/sglang/session_manager.go

package sglang

import (
    "context"
    "sync"

    "github.com/google/uuid"
)

// Session represents a multi-turn conversation session
type Session struct {
    ID                string
    ConversationHistory []Turn
    PrefixLen         int
    mu                sync.Mutex
}

// Turn represents a conversation turn
type Turn struct {
    Input  string
    Output string
}

// SessionManager manages SGLang sessions
type SessionManager struct {
    client   *SGLangClient
    sessions map[string]*Session
    mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(client *SGLangClient) *SessionManager {
    return &SessionManager{
        client:   client,
        sessions: make(map[string]*Session),
    }
}

// CreateSession creates a new session
func (m *SessionManager) CreateSession() *Session {
    m.mu.Lock()
    defer m.mu.Unlock()

    session := &Session{
        ID:                uuid.New().String(),
        ConversationHistory: []Turn{},
    }
    m.sessions[session.ID] = session
    return session
}

// GetSession retrieves a session
func (m *SessionManager) GetSession(id string) (*Session, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    session, ok := m.sessions[id]
    return session, ok
}

// Generate generates within a session context
func (m *SessionManager) Generate(ctx context.Context, sessionID string, prompt string, opts ...GenerateOption) (*GenerateResponse, error) {
    session, ok := m.GetSession(sessionID)
    if !ok {
        return nil, fmt.Errorf("session not found: %s", sessionID)
    }

    session.mu.Lock()
    defer session.mu.Unlock()

    req := &GenerateRequest{
        SessionID: sessionID,
        Prompt:    prompt,
    }

    for _, opt := range opts {
        opt(req)
    }

    resp, err := m.client.Generate(ctx, req)
    if err != nil {
        return nil, err
    }

    // Update session history
    session.ConversationHistory = append(session.ConversationHistory, Turn{
        Input:  prompt,
        Output: resp.Text,
    })
    session.PrefixLen += resp.TokenCount

    return resp, nil
}

// ForkSession creates a copy of a session
func (m *SessionManager) ForkSession(sourceID string) (*Session, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    source, ok := m.sessions[sourceID]
    if !ok {
        return nil, fmt.Errorf("session not found: %s", sourceID)
    }

    source.mu.Lock()
    defer source.mu.Unlock()

    newSession := &Session{
        ID:                uuid.New().String(),
        ConversationHistory: make([]Turn, len(source.ConversationHistory)),
        PrefixLen:         source.PrefixLen,
    }
    copy(newSession.ConversationHistory, source.ConversationHistory)

    m.sessions[newSession.ID] = newSession
    return newSession, nil
}

// DeleteSession removes a session
func (m *SessionManager) DeleteSession(id string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.sessions, id)
}

// GenerateOption configures generation
type GenerateOption func(*GenerateRequest)

// WithMaxTokens sets max tokens
func WithMaxTokens(n int) GenerateOption {
    return func(r *GenerateRequest) {
        r.MaxTokens = n
    }
}

// WithTemperature sets temperature
func WithTemperature(t float64) GenerateOption {
    return func(r *GenerateRequest) {
        r.Temperature = t
    }
}
```

### Prefix Cache Tracker (Local Metadata)

```go
// internal/optimization/sglang/prefix_tree.go

package sglang

import (
    "sync"
    "time"
)

// PrefixNode is a node in the local prefix tracking tree
type PrefixNode struct {
    Key        []byte
    SessionIDs []string
    Children   map[byte]*PrefixNode
    LastAccess time.Time
    RefCount   int
}

// PrefixTree tracks prefix sharing locally
type PrefixTree struct {
    root *PrefixNode
    mu   sync.RWMutex
}

// NewPrefixTree creates a new prefix tree
func NewPrefixTree() *PrefixTree {
    return &PrefixTree{
        root: &PrefixNode{
            Children: make(map[byte]*PrefixNode),
        },
    }
}

// RegisterPrefix registers a session's prefix
func (t *PrefixTree) RegisterPrefix(sessionID string, tokenHash []byte) {
    t.mu.Lock()
    defer t.mu.Unlock()

    current := t.root
    for _, b := range tokenHash {
        child, ok := current.Children[b]
        if !ok {
            child = &PrefixNode{
                Key:      []byte{b},
                Children: make(map[byte]*PrefixNode),
            }
            current.Children[b] = child
        }
        current = child
    }

    current.SessionIDs = append(current.SessionIDs, sessionID)
    current.RefCount++
    current.LastAccess = time.Now()
}

// FindSharingSessions finds sessions that share a prefix
func (t *PrefixTree) FindSharingSessions(tokenHash []byte) []string {
    t.mu.RLock()
    defer t.mu.RUnlock()

    var sessions []string
    current := t.root

    for _, b := range tokenHash {
        child, ok := current.Children[b]
        if !ok {
            break
        }
        sessions = append(sessions, child.SessionIDs...)
        current = child
    }

    return sessions
}

// GetPrefixStats returns statistics about prefix sharing
func (t *PrefixTree) GetPrefixStats() *PrefixStats {
    t.mu.RLock()
    defer t.mu.RUnlock()

    stats := &PrefixStats{}
    t.countNodes(t.root, stats, 0)
    return stats
}

func (t *PrefixTree) countNodes(node *PrefixNode, stats *PrefixStats, depth int) {
    stats.TotalNodes++
    if len(node.SessionIDs) > 0 {
        stats.ActivePrefixes++
        stats.TotalSessions += len(node.SessionIDs)
    }
    if depth > stats.MaxDepth {
        stats.MaxDepth = depth
    }

    for _, child := range node.Children {
        t.countNodes(child, stats, depth+1)
    }
}

// PrefixStats contains prefix tree statistics
type PrefixStats struct {
    TotalNodes     int
    ActivePrefixes int
    TotalSessions  int
    MaxDepth       int
}
```

### Health Check

```go
// internal/optimization/sglang/health.go

package sglang

import (
    "context"
    "fmt"
    "net/http"
    "time"
)

// HealthStatus represents SGLang server health
type HealthStatus struct {
    Available     bool
    Latency       time.Duration
    GPUMemoryUsed float64
    GPUMemoryFree float64
    QueueLength   int
    Error         string
}

// HealthCheck checks SGLang server health
func (c *SGLangClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
    start := time.Now()

    req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
    if err != nil {
        return &HealthStatus{
            Available: false,
            Error:     err.Error(),
        }, nil
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return &HealthStatus{
            Available: false,
            Error:     err.Error(),
        }, nil
    }
    defer resp.Body.Close()

    latency := time.Since(start)

    if resp.StatusCode != http.StatusOK {
        return &HealthStatus{
            Available: false,
            Latency:   latency,
            Error:     fmt.Sprintf("unhealthy status: %d", resp.StatusCode),
        }, nil
    }

    var status HealthStatus
    if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
        status = HealthStatus{Available: true}
    }

    status.Latency = latency
    status.Available = true
    return &status, nil
}

// WaitForReady waits for the server to become available
func (c *SGLangClient) WaitForReady(ctx context.Context, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return fmt.Errorf("timeout waiting for SGLang server")
        case <-ticker.C:
            status, _ := c.HealthCheck(ctx)
            if status.Available {
                return nil
            }
        }
    }
}
```

## Docker Configuration

```yaml
# docker-compose.yml addition
services:
  sglang:
    image: lmsysorg/sglang:latest
    ports:
      - "30000:30000"
    environment:
      - CUDA_VISIBLE_DEVICES=0
    profiles:
      - optimization
      - gpu
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    volumes:
      - sglang-cache:/root/.cache/huggingface
    command: >
      python -m sglang.launch_server
      --model-path meta-llama/Llama-3-8b-instruct
      --host 0.0.0.0
      --port 30000

volumes:
  sglang-cache:
```

## Test Coverage Requirements

```go
// tests/optimization/unit/sglang/client_test.go

func TestSGLangClient_Generate(t *testing.T)
func TestSGLangClient_GenerateStream(t *testing.T)
func TestSGLangClient_HealthCheck(t *testing.T)
func TestSGLangClient_WaitForReady(t *testing.T)

func TestSessionManager_CreateSession(t *testing.T)
func TestSessionManager_Generate(t *testing.T)
func TestSessionManager_ForkSession(t *testing.T)
func TestSessionManager_DeleteSession(t *testing.T)

func TestPrefixTree_RegisterPrefix(t *testing.T)
func TestPrefixTree_FindSharingSessions(t *testing.T)
func TestPrefixTree_GetPrefixStats(t *testing.T)

// tests/optimization/integration/sglang_integration_test.go
func TestSGLang_Integration_MultiTurn(t *testing.T)
func TestSGLang_Integration_PrefixSharing(t *testing.T)
func TestSGLang_Integration_SessionFork(t *testing.T)
```

## Conclusion

SGLang cannot be natively ported to Go due to its deep integration with CUDA and PyTorch for KV-cache management. However, a robust Go client can effectively leverage SGLang's capabilities through HTTP.

**Key Benefits**:
- RadixAttention provides 2-5x speedup for multi-turn conversations
- Prefix caching reduces redundant computation significantly
- Session management enables efficient conversation handling

**Estimated Implementation Time**: 1 week (Go client)
**Risk Level**: Medium (requires GPU infrastructure)
**Dependencies**: Docker with GPU support, model weights
