# HelixLLM Multi-Model Fleet Design

**Date:** 2026-04-10
**Status:** Approved
**Scope:** Replace Qwen3-Coder-30B-A3B with a lightweight model fleet using llama.cpp native router mode

## Problem Statement

HelixLLM currently runs a single Qwen3-Coder-30B-A3B (MoE, 3B active) model that produces artifacts (`????`) at large contexts with Q4_K_M quantization. The project's core purpose is an efficient coding LLM tool that relies on very light local models. A fleet of purpose-built small models (1.5B-3B) with intelligent routing will provide faster inference (180-250+ TPS vs current speeds), better stability, and lower VRAM usage.

## Design Decisions

| Decision | Choice | Alternatives Rejected |
|----------|--------|----------------------|
| Model serving | llama.cpp native router mode (single process, multi-model) | Multiple containers (more orchestration), hot-swap (load latency) |
| Model selection | Replace Qwen3-Coder entirely with lightweight fleet | Tiered coexistence (unnecessary complexity), phased migration (delays benefits) |
| Primary models | Qwen2.5-Coder 1.5B + 3B (Apache-2.0) | Arch-Function (katanemo-research license), xLAM (CC-BY-NC) |
| Embedding model | nomic-embed-text-v1.5 (Apache-2.0, 90MB) | nomic-embed-v2-moe (larger, shorter context), nomic-embed-code (7B, overkill) |
| Routing | Heuristic complexity analysis (<5ms, no LLM call) | LLM-based classification (too slow), static routing (no intelligence) |
| Hardware detection | nvidia-smi + /proc/cpuinfo + /proc/meminfo | Go GPU libraries (less portable), hardcoded profiles (inflexible) |

## Architecture Overview

```
CLI Agents (OpenCode, Crush, Claude Code, etc.)
  |
  | HTTPS :8443 (HTTP/3 + TLS 1.3)
  v
HelixLLM Gateway (Go, Gin)
  |
  v
Brain Layer
  ├── ComplexityAnalyzer.Analyze(req) -> tier
  ├── ModelRegistry.BestAvailable(tier) -> model ID
  ├── Router.Route(req) -> LlamaCppProvider
  └── LlamaCppProvider -> HTTP :8080
        |
        v
llama-server (router mode, single process)
  ├── qwen2.5-coder-1.5b-instruct Q4_K_M  (~1GB VRAM, fast tier)
  ├── qwen2.5-coder-3b-instruct Q4_K_M    (~2GB VRAM, balanced tier)
  └── nomic-embed-text-v1.5 Q4_K_M        (~90MB VRAM, embeddings)
```

Total VRAM: ~3.1GB on 6GB GPU, leaving ~3GB headroom.

## Component Specifications

### 1. Model Fleet & Registry

**Package:** `internal/brain/models/`

**Files:**
- `catalog.go` — Model definitions with capabilities, VRAM requirements, BFCL scores
- `registry.go` — Runtime tracking of model status (unloaded/loading/loaded/error)
- `preset.go` — Generates llama.cpp INI config from catalog + hardware profile

**Model Definition struct:**

```go
type ModelTier string
const (
    TierFast     ModelTier = "fast"      // 1-2B, 180-300 TPS
    TierBalanced ModelTier = "balanced"  // 3B, 120-160 TPS
    TierPowerful ModelTier = "powerful"  // 7-8B, 45-75 TPS
)

type ModelDefinition struct {
    ID              string
    Name            string
    HuggingFaceRepo string
    Filename        string
    Parameters      int64
    Quantization    string
    VRAMRequired    int64
    ContextLength   int
    Tier            ModelTier
    BFCLScore       float64
    TPSEstimate     [2]int
    License         string
    SupportsTools   bool
    RequiresJinja   bool
    Architecture    string
    IsEmbedding     bool
    EmbeddingDims   int
}
```

**Default catalog (4 models):**

| Model | Tier | VRAM | TPS | License | Purpose |
|-------|------|------|-----|---------|---------|
| Qwen2.5-Coder-1.5B-Instruct Q4_K_M | fast | ~1GB | 180-250 | Apache-2.0 | Primary: fast tool calls |
| Qwen2.5-Coder-3B-Instruct Q4_K_M | balanced | ~2GB | 120-160 | Apache-2.0 | Moderate complexity |
| Functionary-small-v3.2 Q4_K_M | powerful | ~5GB | 45-65 | MIT | Complex reasoning (optional) |
| nomic-embed-text-v1.5 Q4_K_M | — | ~90MB | — | Apache-2.0 | Embeddings, 768 dims |

The catalog is data-driven. Users add custom models via JSON config or `HELIX_MODELS_CATALOG` env var.

**Runtime registry tracks:**

```go
type RuntimeModel struct {
    Definition  ModelDefinition
    Status      ModelStatus  // unloaded | loading | loaded | error
    LoadedAt    time.Time
    LastUsed    time.Time
    FilePath    string
    Downloaded  bool
}
```

### 2. Task Complexity Analyzer

**File:** `internal/brain/complexity.go`

Examines incoming requests and assigns a complexity score (0-10) using heuristics. Must complete in <5ms — no LLM calls.

**Scoring signals:**

| Signal | Points | Rationale |
|--------|--------|-----------|
| Tool count > 3 | +2 | Multi-tool orchestration |
| Tool count 1-3 | +1 | Standard tool calling |
| Message content > 2000 chars | +2 | Long context |
| Message content > 500 chars | +1 | Medium context |
| Keywords: analyze, compare, explain, refactor, architect | +1 each (max 3) | Reasoning-heavy |
| Conversation turns > 5 | +1 | Multi-turn context |
| System prompt > 1000 chars | +1 | Complex instructions |
| Explicit model in request | override | User override |

**Routing thresholds:**
- Score 0-2 → simple → fast tier (1.5B)
- Score 3-5 → moderate → balanced tier (3B)
- Score 6+ → complex → powerful tier (8B if available, else 3B)

**Output:**

```go
type ComplexityResult struct {
    Level      ComplexityLevel  // simple | moderate | complex
    Score      int
    Reasons    []string
    TargetTier ModelTier
}
```

**Fallback chain:** requested tier → not available → next lower tier → cloud fallback (existing OpenAI/Anthropic providers).

### 3. Hardware Auto-Profiler

**Package:** `internal/shared/hardware/`

**File:** `profiler.go`

Detects GPU, CPU, and RAM at startup. Produces a `HardwareProfile` used to select preset config and filter available models.

**Detection methods:**

| Resource | Method | Fallback |
|----------|--------|----------|
| GPU | `nvidia-smi --query-gpu=memory.total,memory.free,name,compute_cap --format=csv,noheader` | No GPU → cpu_only profile |
| CPU | Parse `/proc/cpuinfo` for model, cores, avx2/avx512 flags | `runtime.NumCPU()` |
| RAM | Parse `/proc/meminfo` for MemTotal, MemAvailable | `runtime.MemStats` |

**Preset profiles:**

| Profile | VRAM | Max Models | GPU Layers | Context | Batch | Models Enabled |
|---------|------|------------|------------|---------|-------|----------------|
| `consumer_6gb` | 4-6GB | 2 | -1 (all) | 4096 | 512 | 1.5B + 3B |
| `consumer_8gb` | 6-8GB | 3 | -1 (all) | 8192 | 1024 | 1.5B + 3B + embed |
| `high_end` | 8+GB | 3 | -1 (all) | 16384 | 1024 | 1.5B + 3B + 8B |
| `cpu_only` | 0 | 1 | 0 | 2048 | 256 | 1.5B only |

**Thread calculation:**
- Inference: `max(2, physical_cores - 2)`
- Batch: `physical_cores`

**Output struct:**

```go
type HardwareProfile struct {
    GPU               GPUProfile   // Available, Name, VRAMTotal, VRAMFree, ComputeCap
    CPU               CPUProfile   // Model, Cores, Threads, AVX2, AVX512
    RAM               RAMProfile   // Total, Available
    RecommendedModels []string
    PresetProfile     string       // consumer_6gb, cpu_only, etc.
}
```

### 4. Model Downloader

**File:** `internal/brain/downloader.go`

Downloads GGUF files from HuggingFace at first boot or on demand.

**Mechanism:**
- HTTPS GET from `https://huggingface.co/{repo}/resolve/main/{filename}`
- Streams to temp file, atomic rename on completion
- SHA256 verification if checksum in catalog
- Resume support via `Range` header
- Progress reported via channel (structured logging)
- Respects `HF_TOKEN` env var for gated models

**Model directory layout:**

```
/models/                                          (HELIX_MODELS_DIR)
├── qwen2.5-coder-1.5b-instruct-q4_k_m.gguf     (~1 GB)
├── qwen2.5-coder-3b-instruct-q4_k_m.gguf        (~2 GB)
├── nomic-embed-text-v1.5-q4_k_m.gguf             (~90 MB)
└── manifest.json                                  (download state)
```

**Manifest tracks:** model ID, filename, SHA256, size, download timestamp, verification status.

### 5. Container & llama-server Integration

**File:** `HelixLLM/container/Containerfile.llamacpp-router`

**Multi-stage build:**

| Stage | Base | Purpose |
|-------|------|---------|
| Builder | `nvidia/cuda:12.6-devel-ubuntu24.04` | Compile llama.cpp with CUDA + RPC |
| Runtime | `nvidia/cuda:12.6-runtime-ubuntu24.04` | Minimal runtime |

**CMake flags:**

```
GGML_CUDA=ON
GGML_CUDA_FA_ALL_QUANTS=ON
GGML_NATIVE=OFF
GGML_RPC=ON
BUILD_SHARED_LIBS=OFF
CMAKE_BUILD_TYPE=Release
```

**Entrypoint:**

```bash
llama-server \
    --host 0.0.0.0 --port 8080 \
    --models-dir /models \
    --models-max ${HELIX_MODELS_MAX:-3} \
    --models-preset /config/presets.ini \
    --models-autoload \
    --threads ${HELIX_THREADS:-14} \
    --threads-batch ${HELIX_THREADS_BATCH:-16} \
    --metrics
```

**Process management** (`internal/brain/server.go`):

Two deployment modes:
- **Embedded** (`HELIX_MODE=full`): HelixLLM spawns llama-server as child process. Writes presets.ini, spawns process, health-check loop, SIGTERM + WaitGroup on shutdown.
- **External** (`HELIX_MODE=gateway`): llama-server in separate container, managed by HelixAgent container orchestration. HelixLLM connects to configured `HELIX_LLM_LOCAL_URL`.

**Preset INI generator** (`internal/brain/models/preset.go`):

Takes filtered models + hardware profile, produces:

```ini
[global]
flash-attn = on
n-threads = 14
n-threads-batch = 16
# Qwen2.5-Coder models use ChatML format internally.
# The jinja chat-template flag enables native tool calling support in llama.cpp.

[qwen2.5-coder-1.5b-instruct-q4_k_m]
model = /models/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf
n-gpu-layers = -1
ctx-size = 4096
n-batch = 512
chat-template = jinja

[qwen2.5-coder-3b-instruct-q4_k_m]
model = /models/qwen2.5-coder-3b-instruct-q4_k_m.gguf
n-gpu-layers = -1
ctx-size = 4096
n-batch = 512
chat-template = jinja

[nomic-embed-text-v1.5-q4_k_m]
model = /models/nomic-embed-text-v1.5-q4_k_m.gguf
n-gpu-layers = -1
embedding = on
```

Dynamic adjustments: `n-gpu-layers` reduced if VRAM tight, `ctx-size` scaled by VRAM (6GB→4096, 8GB→8192, 12GB+→16384), `n-batch` scaled similarly.

### 6. Layer Integration

**Changes per layer:**

| Layer | Changes | Scope |
|-------|---------|-------|
| Gateway | Add `/v1/models/{id}/download`, `/v1/hardware` endpoints. Update `/v1/models` for per-model status. Remove tool budget truncation. | Small |
| Brain | Add complexity analyzer, model registry, preset generator, server manager, downloader. Update router for complexity-based selection. Update llamacpp provider to pass model field. | Large |
| Knowledge | Add `LlamaEmbedder` that calls llama-server `/v1/embeddings` with nomic model. | Medium |
| Agents | No changes. ReAct loop works transparently. | None |
| Control | Add model file distribution for remote hosts. | Small |
| Shared | Add hardware profiler, model catalog config loading. | Medium |

**New configuration env vars:**

```bash
HELIX_MODELS_DIR=/models
HELIX_MODELS_AUTO_DOWNLOAD=true
HELIX_MODELS_MAX=3
HELIX_MODELS_CATALOG=default
HELIX_GPU_LAYERS=-1                    # override auto
HELIX_CONTEXT_SIZE=4096                # override auto
HELIX_COMPLEXITY_ENABLED=true
HELIX_COMPLEXITY_DEFAULT_TIER=fast
HELIX_LLAMA_SERVER_EMBEDDED=true
HELIX_LLAMA_SERVER_PORT=8080
HF_TOKEN=hf_xxx                       # optional, gated repos
```

**Boot sequence:**

1. Load config (env vars)
2. `HardwareProfiler.Detect()` → HardwareProfile
3. `ModelCatalog.Load()` → filter by profile
4. `ModelDownloader.EnsureModels()` → download missing GGUFs
5. `PresetGenerator.Generate()` → write presets.ini
6. `LlamaServer.Start()` → spawn llama-server
7. `LlamaServer.HealthCheck()` → wait for models loaded
8. `KnowledgeLayer.Init()` → LlamaEmbedder with nomic
9. `GatewayLayer.Start()` → HTTP/3 on :8443
10. Ready

**Complete request flow:**

```
CLI Agent → HTTPS :8443 /v1/chat/completions
  → Gateway auth + middleware
    → ComplexityAnalyzer.Analyze(req) → tier=fast
      → ModelRegistry.BestAvailable(fast) → qwen2.5-coder-1.5b
        → Router.Route(req) → LlamaCppProvider
          → HTTP POST :8080/v1/chat/completions {model: "qwen2.5-coder-1.5b-instruct-q4_k_m"}
            → llama-server router dispatches to 1.5B
              → response (possibly with tool_calls)
                → Agents.ReAct loop if tools
                  → final response → SSE stream to CLI agent
```

### 7. Testing Strategy

**Unit tests (per package):**

| Package | Tests |
|---------|-------|
| `brain/complexity` | Simple/moderate/complex scoring, keyword detection, tool count, model override, conversation length |
| `brain/models/catalog` | Load default, filter by profile, add custom model |
| `brain/models/registry` | BestAvailable per tier, fallback chain, status transitions |
| `brain/models/preset` | INI generation for each preset profile, partial GPU offload |
| `brain/downloader` | Download success + SHA256, resume with Range header, skip existing |
| `brain/server` | Embedded start, graceful shutdown, health check |
| `shared/hardware/profiler` | GPU detection, no-GPU fallback, preset selection, thread calc |
| `knowledge/llama_embedder` | Single embed, batch embed, server-down error |

**Integration tests:**

```
tests/integration/multi_model_routing_test.go
  - Simple request → fast model
  - Complex request → balanced model
  - Fallback when tier unavailable
  - Explicit model override
  - Embedding → nomic model

tests/integration/model_lifecycle_test.go
  - Boot downloads models
  - Preset generated correctly
  - Health reports all models
  - Model load/unload API
```

**Challenge script:** `challenges/scripts/multi_model_fleet_challenge.sh` — 15 tests covering hardware detection, catalog loading, preset generation, llama-server startup, routing correctness, download, health, VRAM bounds, TPS threshold, streaming, graceful shutdown.

**Performance benchmarks:**

| Metric | Target | Method |
|--------|--------|--------|
| 1.5B TPS | 180+ | Generate 256 tokens |
| 3B TPS | 120+ | Generate 256 tokens |
| Complexity analysis | <5ms | Benchmark 1000 requests |
| Routing decision | <1ms | Benchmark model selection |
| TTFT (1.5B) | <200ms | First SSE chunk timing |
| Embedding throughput | 10+ docs/sec | Index 100 documents |

## New File Summary

```
internal/brain/complexity.go              # Task complexity analyzer
internal/brain/server.go                  # llama-server process management
internal/brain/downloader.go              # HuggingFace GGUF downloader
internal/brain/models/catalog.go          # Model definitions & catalog
internal/brain/models/registry.go         # Runtime model tracking
internal/brain/models/preset.go           # llama.cpp INI preset generator
internal/shared/hardware/profiler.go      # GPU/CPU/RAM detection
internal/knowledge/llama_embedder.go      # Local embedding via llama-server
HelixLLM/container/Containerfile.llamacpp-router  # CUDA multi-model container

# Tests (1:1 with source)
internal/brain/complexity_test.go
internal/brain/server_test.go
internal/brain/downloader_test.go
internal/brain/models/catalog_test.go
internal/brain/models/registry_test.go
internal/brain/models/preset_test.go
internal/shared/hardware/profiler_test.go
internal/knowledge/llama_embedder_test.go
tests/integration/multi_model_routing_test.go
tests/integration/model_lifecycle_test.go
challenges/scripts/multi_model_fleet_challenge.sh
```

## Out of Scope (Future Work)

- Advanced RAG (hybrid search, cross-encoder re-ranking, MMR) — separate design
- Expanded 17-tool system with sandboxed execution — separate design
- Redis KV cache for context persistence — separate design
- Prometheus/Grafana observability stack — separate design
- Speculative decoding — requires llama.cpp draft model support maturation
- Production Docker Compose stack (nginx, Redis, ChromaDB) — separate design

## References

- [llama.cpp Router Mode](https://huggingface.co/blog/ggml-org/model-management-in-llamacpp)
- [BFCL Leaderboard](https://gorilla.cs.berkeley.edu/leaderboard.html)
- [katanemo/Arch-Function-1.5B.gguf](https://huggingface.co/katanemo/Arch-Function-1.5B.gguf)
- [Qwen/Qwen2.5-Coder-1.5B-Instruct](https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct)
- [nomic-ai/nomic-embed-text-v1.5-GGUF](https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF)
- [meetkai/functionary-small-v3.2-GGUF](https://huggingface.co/meetkai/functionary-small-v3.2-GGUF)
- HelixLLM research: `HelixLLM/docs/research/tooling/full_plan/`
