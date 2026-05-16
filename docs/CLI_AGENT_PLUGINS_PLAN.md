# CLI Agent Plugins Development Plan

**Version**: 1.0.0
**Date**: 2026-01-22
**Status**: Planning Phase

---

## Executive Summary

This document outlines the comprehensive development plan for creating plugins for all 47+ supported CLI agents. These plugins will extend CLI agent capabilities with:

- **HTTP/3** - Modern transport protocol with QUIC
- **TOON** - Text-based Object-Oriented Notation for structured communication
- **Brotli** - Advanced compression for efficient data transfer
- **Streaming** - Real-time response streaming
- **Events** - AI debate events, warnings, errors, background process notifications
- **UI/UX** - Enhanced terminal rendering and user experience

---

## Supported CLI Agents (47 Total)

### Tier 1 - Primary Support (10 agents)
| Agent | Language | Source Location |
|-------|----------|-----------------|
| Claude Code | TypeScript | `cli_agents/Claude_Code` |
| Aider | Python | `cli_agents/Aider` |
| Cline | TypeScript | `cli_agents/Cline` |
| OpenCode | Go | `cli_agents/OpenCode` |
| Kilo Code | TypeScript | `cli_agents/Kilo-Code` |
| Gemini CLI | Python | `cli_agents/Gemini_CLI` |
| Qwen Code | Python | `cli_agents/Qwen_Code` |
| DeepSeek CLI | Python | `cli_agents/DeepSeek_CLI` |
| Forge | TypeScript | `cli_agents/Forge` |
| Codename Goose | Go | `cli_agents/Codename_Goose` |

### Tier 2 - Secondary Support (15 agents)
| Agent | Language | Source Location |
|-------|----------|-----------------|
| Amazon Q Developer CLI | TypeScript | `cli_agents/Amazon-Q-Developer-CLI` |
| Kiro (Stark Kitty) | Python | `cli_agents/Stark-Kitty-Kiro-Cli` |
| GPT Engineer | Python | `cli_agents/GPT_Engineer` |
| Mistral Code | Python | `cli_agents/Mistral_Code` |
| Ollama Code | Python | `cli_agents/Ollama_Code` |
| Plandex | Go | `cli_agents/Plandex` |
| Codex | TypeScript | `cli_agents/Codex` |
| vtcode | TypeScript | `cli_agents/vtcode` |
| Nanocoder | Python | `cli_agents/Nanocoder` |
| GitMCP | TypeScript | `cli_agents/GitMCP` |
| TaskWeaver | Python | `cli_agents/TaskWeaver` |
| Octogen | Python | `cli_agents/Octogen` |
| FauxPilot | Python | `cli_agents/FauxPilot` |
| Bridle | Go | `cli_agents/Bridle` |
| Agent Deck | TypeScript | `cli_agents/Agent-Deck` |

### Tier 3 - Extended Support (22 agents)
| Agent | Source Location |
|-------|-----------------|
| Claude Squad | `cli_agents/Claude-Squad` |
| Codai | `cli_agents/Codai` |
| Emdash | `cli_agents/Emdash` |
| Get Shit Done | `cli_agents/Get-Shit-Done` |
| GitHub Copilot CLI | `cli_agents/GitHub-Copilot-CLI` |
| GitHub Spec Kit | `cli_agents/GitHub-Spec-Kit` |
| gptme | `cli_agents/gptme` |
| Linear | `cli_agents/Linear` (via MCP) |
| MobileAgent | `cli_agents/MobileAgent` |
| Multiagent Coding | `cli_agents/Multiagent-Coding-System` |
| Noi | `cli_agents/Noi` |
| OpenHands | `cli_agents/OpenHands` |
| Postgres MCP | `cli_agents/Postgres-MCP` |
| Shai | `cli_agents/Shai` |
| SnowCLI | `cli_agents/SnowCLI` |
| Superset | `cli_agents/Superset` |
| UI/UX Pro Max Skill | `cli_agents/ui-ux-pro-max-skill` |
| Warp | `cli_agents/Warp` |
| Cheshire Cat AI | `cli_agents/Cheshire-Cat-Ai` |
| Conduit | `cli_agents/Conduit` |
| Codex Skills | `cli_agents/Codex-Skills` |
| Claude Code Plugins | `cli_agents/Claude-Code-Plugins-And-Skills` |

---

## Plugin Architecture

### Core Plugin Structure

```
plugins/
├── core/
│   ├── http3/              # HTTP/3 transport layer
│   │   ├── client.go
│   │   ├── server.go
│   │   ├── quic_config.go
│   │   └── h3_handler.go
│   ├── toon/               # TOON protocol
│   │   ├── encoder.go
│   │   ├── decoder.go
│   │   ├── schema.go
│   │   └── validator.go
│   ├── brotli/             # Brotli compression
│   │   ├── compressor.go
│   │   ├── decompressor.go
│   │   └── streaming.go
│   ├── streaming/          # Response streaming
│   │   ├── sse.go
│   │   ├── websocket.go
│   │   ├── chunked.go
│   │   └── backpressure.go
│   └── events/             # Event system
│       ├── emitter.go
│       ├── subscriber.go
│       ├── types.go
│       └── handlers/
│           ├── debate.go
│           ├── warning.go
│           ├── error.go
│           └── background.go
├── agents/
│   ├── claude_code/
│   ├── aider/
│   ├── cline/
│   └── ... (one per agent)
└── registry/
    ├── loader.go
    ├── validator.go
    └── manager.go
```

### Plugin Interface

```go
// Plugin defines the interface all CLI agent plugins must implement
type Plugin interface {
    // Metadata
    Name() string
    Version() string
    AgentName() string

    // Lifecycle
    Initialize(ctx context.Context, config *PluginConfig) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error

    // Transport
    SetTransport(transport Transport) error

    // Events
    Subscribe(eventType EventType, handler EventHandler)
    Emit(event Event) error

    // Streaming
    StreamResponse(ctx context.Context, stream ResponseStream) error

    // UI/UX
    RenderOutput(output *Output) error
}

// Transport defines the transport layer interface
type Transport interface {
    // HTTP/3 support
    SupportsHTTP3() bool
    EnableHTTP3(config *HTTP3Config) error

    // Compression
    SetCompression(codec CompressionCodec) error

    // Streaming
    EnableStreaming(mode StreamingMode) error
}
```

---

## Feature Specifications

### 1. HTTP/3 Transport

**Purpose**: Modern transport protocol with QUIC for reduced latency and improved reliability.

**Implementation**:
```go
type HTTP3Config struct {
    // QUIC settings
    MaxIdleTimeout     time.Duration
    MaxIncomingStreams int64
    InitialStreamWindow uint64

    // TLS settings
    TLSConfig *tls.Config

    // Connection pooling
    MaxConnsPerHost int

    // 0-RTT support
    Enable0RTT bool
}
```

**Features**:
- Zero Round Trip Time (0-RTT) connection establishment
- Multiplexed streams without head-of-line blocking
- Built-in encryption (TLS 1.3)
- Connection migration for mobile/unstable networks
- Improved congestion control

### 2. TOON Protocol

**Purpose**: Text-based Object-Oriented Notation for structured AI communication.

**Schema**:
```go
type TOONMessage struct {
    Version   string         `toon:"v"`
    Type      MessageType    `toon:"t"`
    Timestamp int64          `toon:"ts"`
    Source    string         `toon:"src"`
    Target    string         `toon:"tgt"`
    Payload   TOONPayload    `toon:"p"`
    Metadata  TOONMetadata   `toon:"m"`
}

type TOONPayload struct {
    Action    string                 `toon:"a"`
    Data      map[string]interface{} `toon:"d"`
    Stream    bool                   `toon:"s"`
    Priority  int                    `toon:"pr"`
}

type TOONMetadata struct {
    DebateID      string   `toon:"did"`
    Round         int      `toon:"r"`
    Participants  []string `toon:"parts"`
    Confidence    float64  `toon:"conf"`
    ValidationPhase string `toon:"vp"`
}
```

### 3. Brotli Compression

**Purpose**: Advanced compression for efficient data transfer.

**Configuration**:
```go
type BrotliConfig struct {
    Quality           int  // 0-11, default 6
    LGWin             int  // window size 10-24, default 22
    EnableDictionary  bool // use shared dictionary
    StreamingMode     bool // enable streaming compression
    MinCompressSize   int  // minimum size to compress
}
```

**Compression Levels**:
| Level | Speed | Ratio | Use Case |
|-------|-------|-------|----------|
| 0-3 | Very Fast | ~2x | Real-time streaming |
| 4-6 | Fast | ~3x | General use (default) |
| 7-9 | Slow | ~4x | Batch processing |
| 10-11 | Very Slow | ~5x | Archival |

### 4. Streaming System

**Purpose**: Real-time response streaming for AI debate and LLM responses.

**Streaming Modes**:
```go
type StreamingMode int

const (
    StreamingDisabled StreamingMode = iota
    StreamingSSE                    // Server-Sent Events
    StreamingWebSocket              // Full-duplex WebSocket
    StreamingChunked                // HTTP Chunked Transfer
    StreamingGRPC                   // gRPC streaming
)
```

**Backpressure Handling**:
```go
type BackpressureConfig struct {
    HighWaterMark    int           // pause threshold
    LowWaterMark     int           // resume threshold
    BufferSize       int           // internal buffer
    DropPolicy       DropPolicy    // what to do when full
    OnPressure       func(level float64) // callback
}
```

### 5. Events System

**Purpose**: AI debate events, warnings, errors, and background process notifications.

#### Event Types

```go
type EventType string

const (
    // AI Debate Events
    EventDebateStarted        EventType = "debate.started"
    EventDebateRoundStarted   EventType = "debate.round.started"
    EventDebateResponse       EventType = "debate.response"
    EventDebateValidation     EventType = "debate.validation"
    EventDebatePolish         EventType = "debate.polish"
    EventDebateConclusion     EventType = "debate.conclusion"
    EventDebateEnded          EventType = "debate.ended"

    // Validation Phase Events
    EventValidationPhase      EventType = "validation.phase"
    EventValidationInitial    EventType = "validation.initial"
    EventValidationCross      EventType = "validation.cross"
    EventValidationPolish     EventType = "validation.polish"
    EventValidationFinal      EventType = "validation.final"

    // Warning Events
    EventWarningRateLimit     EventType = "warning.rate_limit"
    EventWarningLatency       EventType = "warning.latency"
    EventWarningQuota         EventType = "warning.quota"
    EventWarningDeprecation   EventType = "warning.deprecation"
    EventWarningConfidence    EventType = "warning.low_confidence"

    // Error Events
    EventErrorProvider        EventType = "error.provider"
    EventErrorTimeout         EventType = "error.timeout"
    EventErrorValidation      EventType = "error.validation"
    EventErrorFallback        EventType = "error.fallback"
    EventErrorCircuitBreaker  EventType = "error.circuit_breaker"

    // Background Process Events
    EventBackgroundStarted    EventType = "background.started"
    EventBackgroundProgress   EventType = "background.progress"
    EventBackgroundCompleted  EventType = "background.completed"
    EventBackgroundFailed     EventType = "background.failed"
    EventBackgroundCancelled  EventType = "background.cancelled"

    // Tool Events
    EventToolInvoked          EventType = "tool.invoked"
    EventToolCompleted        EventType = "tool.completed"
    EventToolFailed           EventType = "tool.failed"
)
```

#### Event Payload Examples

```go
// AI Debate Event
type DebateEvent struct {
    DebateID      string           `json:"debate_id"`
    Topic         string           `json:"topic"`
    Round         int              `json:"round"`
    TotalRounds   int              `json:"total_rounds"`
    Participant   *Participant     `json:"participant"`
    Response      string           `json:"response,omitempty"`
    Confidence    float64          `json:"confidence"`
    Phase         string           `json:"phase"` // initial, validation, polish, final
    PhaseIcon     string           `json:"phase_icon"`
    Timestamp     time.Time        `json:"timestamp"`
}

// Warning Event
type WarningEvent struct {
    Code        string            `json:"code"`
    Message     string            `json:"message"`
    Severity    WarningSeverity   `json:"severity"`
    Provider    string            `json:"provider,omitempty"`
    Threshold   float64           `json:"threshold,omitempty"`
    Current     float64           `json:"current,omitempty"`
    Suggestion  string            `json:"suggestion,omitempty"`
}

// Background Process Event
type BackgroundEvent struct {
    TaskID      string            `json:"task_id"`
    TaskType    string            `json:"task_type"`
    Status      TaskStatus        `json:"status"`
    Progress    float64           `json:"progress"` // 0-100
    Message     string            `json:"message,omitempty"`
    StartedAt   time.Time         `json:"started_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    CompletedAt *time.Time        `json:"completed_at,omitempty"`
    Result      interface{}       `json:"result,omitempty"`
    Error       string            `json:"error,omitempty"`
}
```

### 6. UI/UX Enhancements

**Purpose**: Enhanced terminal rendering and user experience.

#### Terminal Rendering

```go
type TerminalRenderer interface {
    // Phase indicators
    RenderPhaseIndicator(phase string, icon string) error

    // Progress bars
    RenderProgress(current, total int, label string) error

    // Debate visualization
    RenderDebateRound(round *DebateRound) error
    RenderParticipant(participant *Participant, response string) error
    RenderConsensus(consensus *Consensus) error

    // Streaming output
    StreamText(text string) error
    StreamJSON(data interface{}) error
    StreamMarkdown(md string) error

    // Status indicators
    RenderStatus(status Status, message string) error
    RenderWarning(warning *Warning) error
    RenderError(err error) error

    // Layout
    StartSection(title string) error
    EndSection() error
    Divider() error

    // Colors and styling
    SetTheme(theme *Theme) error
}
```

#### Phase Indicators

| Phase | Icon | Color | Description |
|-------|------|-------|-------------|
| Initial Response | `🔍` | Blue | Each AI provides initial perspective |
| Validation | `✓` | Green | Cross-validation of responses |
| Polish & Improve | `✨` | Yellow | Refinement based on feedback |
| Final Conclusion | `📜` | Purple | Synthesized consensus |

#### Progress Visualization

```
┌─────────────────────────────────────────────────────────┐
│ AI Debate: Should AI have consciousness?                │
├─────────────────────────────────────────────────────────┤
│ Phase: 🔍 Initial Response                              │
│ Round: 2/3                                              │
│ Progress: ████████████░░░░░░░░ 60%                      │
├─────────────────────────────────────────────────────────┤
│ Participants:                                           │
│  ✓ Claude (Opus) - Completed                           │
│  ⟳ DeepSeek - Processing...                            │
│  ○ Gemini - Waiting                                    │
└─────────────────────────────────────────────────────────┘
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (Weeks 1-2)

1. **HTTP/3 Transport Layer**
   - Implement QUIC client and server
   - Add 0-RTT support
   - Connection pooling and migration

2. **TOON Protocol**
   - Define schema and types
   - Implement encoder/decoder
   - Add validation

3. **Brotli Compression**
   - Streaming compression
   - Shared dictionary support
   - Auto-detection and fallback

### Phase 2: Events System (Weeks 3-4)

1. **Event Emitter/Subscriber**
   - Thread-safe event bus
   - Event filtering and routing
   - Persistence for replay

2. **Debate Events**
   - Phase transitions
   - Participant responses
   - Consensus building

3. **Background Process Events**
   - Task lifecycle
   - Progress tracking
   - Cancellation support

### Phase 3: Streaming (Weeks 5-6)

1. **SSE Implementation**
   - Event formatting
   - Reconnection handling
   - Keep-alive

2. **WebSocket Implementation**
   - Full-duplex communication
   - Binary and text modes
   - Ping/pong heartbeat

3. **Backpressure Handling**
   - Buffer management
   - Flow control
   - Drop policies

### Phase 4: UI/UX (Weeks 7-8)

1. **Terminal Renderer**
   - ANSI escape codes
   - Unicode support
   - Color themes

2. **Progress Indicators**
   - Spinners
   - Progress bars
   - Phase indicators

3. **Rich Output**
   - Markdown rendering
   - Syntax highlighting
   - Tables and boxes

### Phase 5: Agent Plugins (Weeks 9-12)

**Tier 1 Agents** (Weeks 9-10):
- Claude Code, Aider, Cline, OpenCode, Kilo Code
- Gemini CLI, Qwen Code, DeepSeek CLI, Forge, Codename Goose

**Tier 2 Agents** (Weeks 11-12):
- Amazon Q, Kiro, GPT Engineer, Mistral Code, Ollama Code
- Plandex, Codex, vtcode, Nanocoder, GitMCP
- TaskWeaver, Octogen, FauxPilot, Bridle, Agent Deck

### Phase 6: Testing & Documentation (Weeks 13-14)

1. **Unit Tests**
   - Core plugins: 100% coverage
   - Agent plugins: 90% coverage

2. **Integration Tests**
   - Cross-plugin communication
   - Event flow verification
   - Streaming reliability

3. **Documentation**
   - API reference
   - Plugin development guide
   - Agent integration guides

---

## Testing Strategy

### Unit Tests

```go
// Example: HTTP/3 transport test
func TestHTTP3Transport(t *testing.T) {
    config := &HTTP3Config{
        MaxIdleTimeout: 30 * time.Second,
        Enable0RTT:     true,
    }

    transport := NewHTTP3Transport(config)

    t.Run("connects successfully", func(t *testing.T) {
        err := transport.Connect(ctx, "https://example.com")
        assert.NoError(t, err)
    })

    t.Run("supports 0-RTT", func(t *testing.T) {
        assert.True(t, transport.Supports0RTT())
    })
}
```

### Integration Tests

```go
// Example: Full event flow test
func TestDebateEventFlow(t *testing.T) {
    plugin := NewClaudeCodePlugin()

    var events []Event
    plugin.Subscribe(EventDebateResponse, func(e Event) {
        events = append(events, e)
    })

    // Start debate
    debate := plugin.StartDebate(ctx, &DebateConfig{
        Topic:       "Test topic",
        Rounds:      3,
        Participants: []string{"claude", "deepseek"},
    })

    // Wait for completion
    <-debate.Done()

    // Verify event flow
    assert.Greater(t, len(events), 0)
    assert.Equal(t, EventDebateStarted, events[0].Type)
    assert.Equal(t, EventDebateEnded, events[len(events)-1].Type)
}
```

### Challenge Tests

```bash
# Plugin validation challenge
./challenges/scripts/plugin_validation_challenge.sh

# Event flow challenge
./challenges/scripts/event_flow_challenge.sh

# Streaming challenge
./challenges/scripts/streaming_challenge.sh
```

---

## Dependencies

### Go Dependencies

```go
// go.mod additions
require (
    github.com/quic-go/quic-go v0.41.0      // HTTP/3
    github.com/andybalholm/brotli v1.0.6     // Brotli compression
    github.com/gorilla/websocket v1.5.1      // WebSocket
    github.com/fatih/color v1.16.0           // Terminal colors
    github.com/charmbracelet/lipgloss v0.9.1 // Terminal styling
    github.com/charmbracelet/bubbletea v0.25.0 // TUI framework
)
```

### External Services

| Service | Purpose | Required |
|---------|---------|----------|
| Redis | Event pub/sub | Optional |
| Kafka | High-throughput events | Optional |
| Prometheus | Metrics | Optional |

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Plugin Coverage | 47/47 agents | Agent count |
| HTTP/3 Latency Reduction | 30% | Benchmark |
| Compression Ratio | 3x average | Data size |
| Event Delivery | 99.9% | Success rate |
| Test Coverage | 95%+ | Code coverage |
| Documentation | 100% | API coverage |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| HTTP/3 browser support | Fallback to HTTP/2 |
| Brotli decompression overhead | Adaptive compression levels |
| Event ordering | Sequence numbers + timestamps |
| Plugin compatibility | Strict interface versioning |
| Agent API changes | Adapter pattern |

---

## Appendix

### A. CLI Agent Source Locations

All agent source code available at:
```
/run/media/milosvasic/DATA4TB/Projects/helix_code/cli_agents/
```

### B. Existing Plugin Examples

Reference plugins at:
```
/run/media/milosvasic/DATA4TB/Projects/helix_code/cli_agents/Claude-Code-Plugins-And-Skills/plugins/
```

### C. Skills Reference

Reference skills at:
```
/run/media/milosvasic/DATA4TB/Projects/helix_code/cli_agents/Claude-Code-Plugins-And-Skills/skills/
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-01-22 | Claude Opus 4.5 | Initial plan |
