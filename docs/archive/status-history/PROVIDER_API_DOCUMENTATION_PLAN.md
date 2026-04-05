# Provider API Documentation & Integration Plan
## HelixAgent - Cutting-Edge Provider Support

**Date:** 2026-04-03  
**Goal:** Complete API documentation for all 20+ providers with cutting-edge protocol support

---

## Providers to Document

### Tier 1: Major Cloud Providers (Priority: CRITICAL)

| Provider | API Docs URL | Status | HTTP3 | Brotli | Streaming | Toon |
|----------|-------------|--------|-------|--------|-----------|------|
| OpenAI | https://platform.openai.com/docs | ⏳ | ✅ | ✅ | ✅ | ⏳ |
| Anthropic | https://docs.anthropic.com | ⏳ | ✅ | ✅ | ✅ | ⏳ |
| Google (Gemini) | https://ai.google.dev/docs | ⏳ | ✅ | ✅ | ✅ | ⏳ |
| DeepSeek | https://platform.deepseek.com/docs | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| Qwen (Alibaba) | https://help.aliyun.com/document_detail/xxx.html | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |

### Tier 2: Secondary Providers (Priority: HIGH)

| Provider | API Docs URL | Status | HTTP3 | Brotli | Streaming | Toon |
|----------|-------------|--------|-------|--------|-----------|------|
| Mistral AI | https://docs.mistral.ai | ⏳ | ⏳ | ✅ | ✅ | ⏳ |
| Groq | https://console.groq.com/docs | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| Cohere | https://docs.cohere.com | ⏳ | ⏳ | ✅ | ✅ | ⏳ |
| AI21 Labs | https://docs.ai21.com | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| Together AI | https://docs.together.ai | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| Fireworks AI | https://docs.fireworks.ai | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |

### Tier 3: Open Source/Local (Priority: MEDIUM)

| Provider | API Docs URL | Status | HTTP3 | Brotli | Streaming | Toon |
|----------|-------------|--------|-------|--------|-----------|------|
| Ollama | https://github.com/ollama/ollama/blob/main/docs/api.md | ⏳ | ❌ | ❌ | ✅ | ⏳ |
| LM Studio | https://lmstudio.ai/docs/local-api | ⏳ | ❌ | ❌ | ✅ | ⏳ |
| vLLM | https://docs.vllm.ai/en/latest/serving/openai_compatible_server.html | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| llama.cpp | https://github.com/ggerganov/llama.cpp/blob/master/examples/server/README.md | ⏳ | ❌ | ❌ | ✅ | ⏳ |

### Tier 4: Aggregators & Specialized (Priority: MEDIUM)

| Provider | API Docs URL | Status | HTTP3 | Brotli | Streaming | Toon |
|----------|-------------|--------|-------|--------|-----------|------|
| OpenRouter | https://openrouter.ai/docs | ⏳ | ⏳ | ✅ | ✅ | ⏳ |
| Perplexity | https://docs.perplexity.ai | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| Cloudflare Workers AI | https://developers.cloudflare.com/workers-ai/ | ⏳ | ✅ | ✅ | ✅ | ⏳ |
| NVIDIA NIM | https://docs.nvidia.com/nim/ | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |

### Tier 5: Regional/Chinese Providers (Priority: HIGH for DeepSeek/Z.AI fix)

| Provider | API Docs URL | Status | HTTP3 | Brotli | Streaming | Toon |
|----------|-------------|--------|-------|--------|-----------|------|
| Z.AI (Zhipu/GLM) | https://open.bigmodel.cn/dev/api | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| MiniMax | https://www.minimaxi.com/document | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| Moonshot AI (Kimi) | https://platform.moonshot.cn/docs | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| Baidu (ERNIE) | https://cloud.baidu.com/doc/WENXINWORKSHOP/s/clntwmv7t | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |
| ByteDance (Doubao) | https://www.volcengine.com/docs/82379 | ⏳ | ⏳ | ⏳ | ✅ | ⏳ |

---

## Documentation Structure for Each Provider

For each provider, create a comprehensive documentation file at:
`docs/providers/<provider-name>/`

```
docs/providers/
├── openai/
│   ├── README.md                 # Overview
│   ├── endpoints.md              # All API endpoints
│   ├── models.md                 # Available models
│   ├── authentication.md         # Auth methods
│   ├── requests/                 # Request examples
│   │   ├── chat-completions.md
│   │   ├── embeddings.md
│   │   ├── fine-tuning.md
│   │   └── ...
│   ├── responses/                # Response schemas
│   │   ├── chat-completions.md
│   │   ├── embeddings.md
│   │   └── ...
│   ├── streaming.md              # Streaming implementation
│   ├── advanced/                 # Cutting-edge features
│   │   ├── http3-quic.md
│   │   ├── brotli-compression.md
│   │   ├── toon-encoding.md
│   │   └── cronet-integration.md
│   └── sdk-integration.md        # HelixAgent integration
├── anthropic/
│   └── ... (same structure)
├── google-gemini/
│   └── ...
└── ... (all providers)
```

---

## Phase 1: Fetch & Document Core APIs (Week 1-2)

### 1.1 OpenAI API Documentation

**Base URL:** `https://api.openai.com/v1`

**Endpoints to Document:**
```yaml
chat_completions:
  endpoint: POST /chat/completions
  description: Create chat completion
  streaming: true
  models: [gpt-5, gpt-5-mini, gpt-5-nano, gpt-4o, ...]
  
embeddings:
  endpoint: POST /embeddings
  description: Create embeddings
  models: [text-embedding-3-small, text-embedding-3-large, ...]
  
images:
  endpoint: POST /images/generations
  description: Generate images (DALL-E)
  
audio:
  speech: POST /audio/speech
  transcription: POST /audio/transcriptions
  translation: POST /audio/translations
  
files:
  list: GET /files
  upload: POST /files
  retrieve: GET /files/{file_id}
  delete: DELETE /files/{file_id}
  
fine_tuning:
  create: POST /fine_tuning/jobs
  list: GET /fine_tuning/jobs
  retrieve: GET /fine_tuning/jobs/{job_id}
  cancel: POST /fine_tuning/jobs/{job_id}/cancel
  
batch:
  create: POST /batches
  list: GET /batches
  retrieve: GET /batches/{batch_id}
  cancel: POST /batches/{batch_id}/cancel
  
assistants:
  create: POST /assistants
  list: GET /assistants
  retrieve: GET /assistants/{assistant_id}
  modify: POST /assistants/{assistant_id}
  delete: DELETE /assistants/{assistant_id}
  
  threads:
    create: POST /threads
    retrieve: GET /threads/{thread_id}
    modify: POST /threads/{thread_id}
    delete: DELETE /threads/{thread_id}
    
    messages:
      create: POST /threads/{thread_id}/messages
      list: GET /threads/{thread_id}/messages
      retrieve: GET /threads/{thread_id}/messages/{message_id}
      modify: POST /threads/{thread_id}/messages/{message_id}
      
    runs:
      create: POST /threads/{thread_id}/runs
      list: GET /threads/{thread_id}/runs
      retrieve: GET /threads/{thread_id}/runs/{run_id}
      modify: POST /threads/{thread_id}/runs/{run_id}
      submit_tool: POST /threads/{thread_id}/runs/{run_id}/submit_tool_outputs
      cancel: POST /threads/{thread_id}/runs/{run_id}/cancel
      
  files:
    create: POST /assistants/{assistant_id}/files
    list: GET /assistants/{assistant_id}/files
    retrieve: GET /assistants/{assistant_id}/files/{file_id}
    delete: DELETE /assistants/{assistant_id}/files/{file_id}

vector_stores:
  create: POST /vector_stores
  list: GET /vector_stores
  retrieve: GET /vector_stores/{vector_store_id}
  modify: POST /vector_stores/{vector_store_id}
  delete: DELETE /vector_stores/{vector_store_id}
  
  files:
    create: POST /vector_stores/{vector_store_id}/files
    list: GET /vector_stores/{vector_store_id}/files
    retrieve: GET /vector_stores/{vector_store_id}/files/{file_id}
    delete: DELETE /vector_stores/{vector_store_id}/files/{file_id}

uploads:
  create: POST /uploads
  complete: POST /uploads/{upload_id}/complete
  cancel: POST /uploads/{upload_id}/cancel
  parts:
    create: POST /uploads/{upload_id}/parts
```

### 1.2 Anthropic API Documentation

**Base URL:** `https://api.anthropic.com/v1`

**Endpoints:**
```yaml
messages:
  endpoint: POST /messages
  description: Create message
  streaming: true (via streaming parameter)
  models: [claude-opus-4-6, claude-sonnet-4-6, claude-haiku-4-5, ...]
  
message_batches:
  create: POST /messages/batches
  retrieve: GET /messages/batches/{batch_id}
  list: GET /messages/batches
  cancel: POST /messages/batches/{batch_id}/cancel
  results: GET /messages/batches/{batch_id}/results
  
models:
  list: GET /models
  retrieve: GET /models/{model_id}
```

### 1.3 Google Gemini API Documentation

**Base URLs:**
- `https://generativelanguage.googleapis.com/v1beta`
- `https://aiplatform.googleapis.com/v1` (Vertex AI)

**Endpoints:**
```yaml
models:
  list: GET /models
  retrieve: GET /models/{model}
  
generate_content:
  endpoint: POST /models/{model}:generateContent
  streaming: POST /models/{model}:streamGenerateContent
  models: [gemini-2.5-pro, gemini-2.5-flash, ...]
  
embeddings:
  endpoint: POST /models/{model}:embedContent
  batch: POST /models/{model}:batchEmbedContents
  
cached_content:
  create: POST /cachedContents
  list: GET /cachedContents
  retrieve: GET /cachedContents/{name}
  delete: DELETE /cachedContents/{name}
  
files:
  upload: POST /files
  list: GET /files
  retrieve: GET /files/{name}
  delete: DELETE /files/{name}
  
tuning:
  create: POST /tunedModels
  list: GET /tunedModels
  retrieve: GET /tunedModels/{name}
  transfer: POST /tunedModels/{name}:transferOwnership
  delete: DELETE /tunedModels/{name}
  operations:
    list: GET /tunedModels/{name}/operations
    retrieve: GET /tunedModels/{name}/operations/{operation}
```

### 1.4 DeepSeek API Documentation

**Base URL:** `https://api.deepseek.com/v1`

**Endpoints:**
```yaml
chat_completions:
  endpoint: POST /chat/completions
  streaming: true
  models: [deepseek-chat, deepseek-reasoner, ...]
  
completions:
  endpoint: POST /completions
  
models:
  list: GET /models
  
user:
  balance: GET /user/balance
```

---

## Phase 2: Cutting-Edge Protocol Implementation (Week 3-4)

### 2.1 HTTP/3 (QUIC) Support

**Implementation Strategy:**
```go
// internal/http/quic_client.go
package http

import (
    "github.com/quic-go/quic-go"
    "github.com/quic-go/quic-go/http3"
)

type QUICClient struct {
    client *http3.Client
    config *QUICConfig
}

type QUICConfig struct {
    Enable0RTT        bool
    MaxStreams        int
    ConnectionTimeout time.Duration
}

func NewQUICClient(config *QUICConfig) *QUICClient {
    return &QUICClient{
        client: &http3.Client{
            // HTTP/3 configuration
        },
        config: config,
    }
}

// Provider support matrix for HTTP/3
var HTTP3Providers = map[string]bool{
    "openai":     true,
    "anthropic":  true,
    "google":     true,
    "cloudflare": true,
    "deepseek":   false, // Check actual support
    "qwen":       false,
}
```

**Fallback Strategy:**
1. Attempt HTTP/3 connection
2. If failed, fall back to HTTP/2
3. If failed, fall back to HTTP/1.1 with keep-alive

### 2.2 Brotli Compression

**Implementation:**
```go
// internal/http/compression.go
package http

import (
    "github.com/andybalholm/brotli"
)

type BrotliTransport struct {
    Base http.RoundTripper
}

func (t *BrotliTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    req.Header.Set("Accept-Encoding", "br, gzip, deflate")
    resp, err := t.Base.RoundTrip(req)
    if err != nil {
        return nil, err
    }
    
    switch resp.Header.Get("Content-Encoding") {
    case "br":
        resp.Body = io.NopCloser(brotli.NewReader(resp.Body))
        resp.Header.Del("Content-Encoding")
    case "gzip":
        resp.Body, _ = gzip.NewReader(resp.Body)
        resp.Header.Del("Content-Encoding")
    }
    
    return resp, nil
}
```

### 2.3 Toon (Binary JSON) Encoding

**Toon** is a compact binary serialization format. Implementation:

```go
// internal/encoding/toon/toon.go
package toon

import (
    "github.com/tinylib/msgp/msgp" // MessagePack-based
)

// ToonEncoder encodes to compact binary format
type Encoder struct {
    w io.Writer
}

func (e *Encoder) Encode(v interface{}) error {
    // Implement Toon encoding
    // Could use MessagePack, CBOR, or custom format
    return msgp.Encode(e.w, v)
}

// ToonDecoder decodes from compact binary format
type Decoder struct {
    r io.Reader
}

func (d *Decoder) Decode(v interface{}) error {
    return msgp.Decode(d.r, v)
}

// HTTP transport with Toon support
type ToonTransport struct {
    Base http.RoundTripper
}

func (t *ToonTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Add Toon headers
    req.Header.Set("Accept", "application/toon, application/json")
    req.Header.Set("Content-Type", "application/toon")
    
    return t.Base.RoundTrip(req)
}
```

**Note:** Toon is not widely supported. Primary fallback is JSON.

### 2.4 Cronet Integration (Android/Chrome Networking)

For mobile/desktop apps:

```go
// internal/http/cronet.go
// +build android

package http

/*
#cgo LDFLAGS: -lcronet
#include <cronet/cronet.h>
*/
import "C"

// CronetClient uses Chrome's networking stack
type CronetClient struct {
    engine C.Cronet_EnginePtr
}

func NewCronetClient() *CronetClient {
    // Initialize Cronet engine
    params := C.Cronet_EngineParams_Create()
    C.Cronet_EngineParams_user_agent_set(params, C.CString("HelixAgent/1.0"))
    C.Cronet_EngineParams_enable_http2_set(params, C.true)
    C.Cronet_EngineParams_enable_quic_set(params, C.true)
    
    engine := C.Cronet_Engine_Create()
    C.Cronet_Engine_StartWithParams(engine, params)
    
    return &CronetClient{engine: engine}
}
```

---

## Phase 3: Advanced Streaming & Real-Time (Week 5)

### 3.1 Server-Sent Events (SSE) Optimization

```go
// internal/streaming/sse.go
package streaming

type SSEConfig struct {
    ReconnectTime     time.Duration
    MaxRetries        int
    Compression       string // "gzip", "br", "none"
    MultiplexStreams  bool   // Enable stream multiplexing
}

type OptimizedSSEClient struct {
    config SSEConfig
    client *http.Client
}

func (c *OptimizedSSEClient) StreamWithFallback(ctx context.Context, req *http.Request) (<-chan SSEEvent, error) {
    // Try HTTP/3 first
    if c.supportsHTTP3(req.URL.Host) {
        return c.streamHTTP3(ctx, req)
    }
    // Fall back to HTTP/2 with SSE
    return c.streamHTTP2(ctx, req)
}
```

### 3.2 WebSocket Support (for Real-time APIs)

```go
// internal/streaming/websocket.go
package streaming

type WebSocketConfig struct {
    Compression       string
    MessageSizeLimit  int64
    PingInterval      time.Duration
    EnableMultiplexing bool
}
```

### 3.3 gRPC Support (for Google/Vertex AI)

```go
// internal/grpc/client.go
package grpc

import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

type ProviderGRPCClient struct {
    conn *grpc.ClientConn
}

func NewProviderGRPCClient(endpoint string, creds credentials.TransportCredentials) (*ProviderGRPCClient, error) {
    conn, err := grpc.Dial(endpoint,
        grpc.WithTransportCredentials(creds),
        grpc.WithDefaultCallOptions(
            grpc.UseCompressor("gzip"),
        ),
    )
    if err != nil {
        return nil, err
    }
    return &ProviderGRPCClient{conn: conn}, nil
}
```

---

## Phase 4: Provider-Specific Optimizations (Week 6)

### 4.1 DeepSeek/Z.AI Stability Fix

Based on research showing slowdowns with OpenCode:

```go
// internal/providers/deepseek/optimized_client.go
package deepseek

type OptimizedDeepSeekClient struct {
    baseClient *http.Client
    
    // Optimization parameters
    connectionPoolSize int
    retryConfig        RetryConfig
    compression        string
    
    // Circuit breaker for stability
    circuitBreaker *CircuitBreaker
}

type RetryConfig struct {
    MaxRetries      int
    BaseDelay       time.Duration
    MaxDelay        time.Duration
    ExponentialBase float64
}

func NewOptimizedClient() *OptimizedDeepSeekClient {
    return &OptimizedDeepSeekClient{
        connectionPoolSize: 100,
        retryConfig: RetryConfig{
            MaxRetries:      5,
            BaseDelay:       100 * time.Millisecond,
            MaxDelay:        5 * time.Second,
            ExponentialBase: 2.0,
        },
        compression:    "br", // Brotli
        circuitBreaker: NewCircuitBreaker(5, 30*time.Second),
    }
}

// Request optimization for DeepSeek
func (c *OptimizedDeepSeekClient) OptimizeRequest(req *http.Request) *http.Request {
    // Add keep-alive headers
    req.Header.Set("Connection", "keep-alive")
    req.Header.Set("Keep-Alive", "timeout=60, max=1000")
    
    // Enable compression
    req.Header.Set("Accept-Encoding", "br, gzip")
    
    // Use HTTP/2 multiplexing
    req.Header.Set("X-HTTP2-Scheme", "https")
    
    return req
}
```

### 4.2 Provider Health Monitoring

```go
// internal/providers/health_monitor.go
package providers

type HealthMonitor struct {
    providers map[string]*ProviderHealth
}

type ProviderHealth struct {
    Name              string
    Latency           time.Duration
    SuccessRate       float64
    HTTP3Support      bool
    BrotliSupport     bool
    StreamingSupport  bool
    LastChecked       time.Time
    Status            HealthStatus
}

type HealthStatus int

const (
    StatusHealthy HealthStatus = iota
    StatusDegraded
    StatusUnhealthy
)

func (m *HealthMonitor) CheckProvider(name string) *ProviderHealth {
    // Perform health check
    // Test HTTP/3, compression, streaming
    // Return health status
}
```

---

## Implementation Timeline

| Week | Focus | Deliverables |
|------|-------|--------------|
| 1 | OpenAI API Docs | Complete OpenAI endpoint documentation |
| 2 | Anthropic + Google | Complete Anthropic & Gemini docs |
| 3 | HTTP/3 + Brotli | QUIC client, Brotli compression |
| 4 | Toon + Cronet | Binary encoding, mobile optimization |
| 5 | Streaming | SSE optimization, WebSocket, gRPC |
| 6 | DeepSeek/Z.AI Fix | Stability improvements, health monitoring |
| 7 | Remaining Providers | Document all 20+ providers |
| 8 | Integration | Integrate into HelixAgent LLMProvider |

---

## Directory Structure

```
docs/providers/
├── README.md                          # Provider overview
├── comparison-matrix.md               # Feature comparison
├── http3-support.md                   # HTTP/3 availability
├── 
├── openai/
│   ├── README.md
│   ├── endpoints.md
│   ├── models.md
│   ├── authentication.md
│   ├── requests/
│   │   ├── chat-completions.md
│   │   ├── embeddings.md
│   │   ├── images.md
│   │   ├── audio.md
│   │   ├── files.md
│   │   ├── fine-tuning.md
│   │   ├── batch.md
│   │   ├── assistants.md
│   │   ├── vector-stores.md
│   │   └── uploads.md
│   ├── responses/
│   │   └── (same structure)
│   ├── streaming.md
│   ├── advanced/
│   │   ├── http3-quic.md
│   │   ├── brotli-compression.md
│   │   └── multiplexing.md
│   └── sdk-integration.md
│
├── anthropic/
│   └── (same structure)
│
├── google-gemini/
│   └── (same structure)
│
├── deepseek/
│   ├── README.md
│   ├── endpoints.md
│   ├── models.md
│   ├── stability-optimization.md    # DeepSeek-specific optimizations
│   └── troubleshooting.md            # Known issues & workarounds
│
├── z-ai/
│   ├── README.md
│   ├── endpoints.md
│   ├── models.md
│   └── stability-optimization.md    # Z.AI-specific optimizations
│
└── [all other providers]/
    └── (same structure)
```

---

## Success Criteria

- [ ] All 20+ providers documented with complete API reference
- [ ] HTTP/3 support implemented for compatible providers
- [ ] Brotli compression enabled where supported
- [ ] Toon encoding implemented (with JSON fallback)
- [ ] DeepSeek/Z.AI slowdown issues resolved
- [ ] Streaming optimized for all providers
- [ ] Health monitoring system in place
- [ ] SDK integration guide for each provider
