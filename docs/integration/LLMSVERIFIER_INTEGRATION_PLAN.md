# LLMsVerifier Integration Plan for HelixAgent

## Executive Summary

This document provides a comprehensive, nano-level integration plan for incorporating LLMsVerifier (enterprise-grade LLM verification, monitoring, and optimization platform) into HelixAgent. The integration covers 30+ feature categories across 10 phases, ensuring 100% feature utilization with complete test coverage.

---

## Table of Contents

1. [Integration Overview](#1-integration-overview)
2. [Feature Inventory](#2-feature-inventory)
3. [Phase 1: Core Infrastructure](#phase-1-core-infrastructure)
4. [Phase 2: Provider Integration](#phase-2-provider-integration)
5. [Phase 3: Verification System](#phase-3-verification-system)
6. [Phase 4: Scoring Engine](#phase-4-scoring-engine)
7. [Phase 5: Health Monitoring & Failover](#phase-5-health-monitoring--failover)
8. [Phase 6: API & Protocol Integration](#phase-6-api--protocol-integration)
9. [Phase 7: Security & Authentication](#phase-7-security--authentication)
10. [Phase 8: Monitoring & Analytics](#phase-8-monitoring--analytics)
11. [Phase 9: SDKs & Client Integration](#phase-9-sdks--client-integration)
12. [Phase 10: Documentation & Training](#phase-10-documentation--training)
13. [Test Coverage Matrix](#test-coverage-matrix)
14. [Deployment Strategy](#deployment-strategy)

---

## 1. Integration Overview

### 1.1 Current State

**HelixAgent (HelixAgent)**:
- Go 1.23+ with Gin framework
- 7 LLM providers: Claude, DeepSeek, Gemini, Qwen, ZAI, Ollama, OpenRouter
- Ensemble orchestration with voting strategies
- OpenAI-compatible REST APIs + gRPC
- PostgreSQL + Redis
- MCP/LSP/ACP protocol support
- LLM Optimization framework (8 tools)

**LLMsVerifier**:
- Go 1.24+ with Gin framework
- 12+ LLM providers with dynamic model discovery
- Mandatory "Do you see my code?" verification
- 5-component weighted scoring system
- Circuit breaker failover
- SQLite with SQL Cipher encryption
- Multi-platform clients (CLI, TUI, Web, Mobile)
- Multi-language SDKs (Go, Python, JavaScript)
- Enterprise security (LDAP/SSO, RBAC)
- Comprehensive monitoring (Prometheus, Grafana)

### 1.2 Integration Goals

1. **Full Feature Utilization**: Every LLMsVerifier feature must be integrated and used
2. **100% Test Coverage**: All test types (unit, integration, e2e, security, stress, chaos)
3. **Zero Breaking Changes**: Existing HelixAgent functionality preserved
4. **Enhanced Capabilities**: Combined power of both systems
5. **Complete Documentation**: User guides, video courses, API references

### 1.3 Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         HelixAgent                               │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   API Layer     │  │  Services       │  │   Handlers      │  │
│  │  (Gin Router)   │  │  (Business)     │  │   (HTTP/gRPC)   │  │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘  │
│           │                    │                    │            │
│  ┌────────▼────────────────────▼────────────────────▼────────┐  │
│  │              LLMsVerifier Integration Layer                │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐  │  │
│  │  │Verifier  │ │ Scoring  │ │ Health   │ │ Providers    │  │  │
│  │  │Service   │ │ Engine   │ │ Checker  │ │ Registry     │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────┘  │  │
│  └────────────────────────────────────────────────────────────┘  │
│           │                    │                    │            │
│  ┌────────▼────────────────────▼────────────────────▼────────┐  │
│  │                 LLMsVerifier (Submodule)                   │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐  │  │
│  │  │Providers │ │Database  │ │   API    │ │ Monitoring   │  │  │
│  │  │  (12+)   │ │ (SQLite) │ │ Server   │ │ (Prometheus) │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────┘  │  │
│  └────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Feature Inventory

### 2.1 LLMsVerifier Features to Integrate

| # | Feature Category | Features | Priority |
|---|-----------------|----------|----------|
| 1 | **LLM Providers** | OpenAI, Anthropic, Google, Groq, Together AI, Mistral, xAI, Replicate, DeepSeek, Cerebras, Cloudflare Workers AI, SiliconFlow | P0 |
| 2 | **Verification System** | "Do you see my code?" test, 20+ capability tests, Code verification service | P0 |
| 3 | **Scoring Engine** | 5-component scoring, Score suffixes (SC:X.X), Batch scoring | P0 |
| 4 | **Health Monitoring** | Circuit breaker, Health checker, Latency router, Provider status tracking | P0 |
| 5 | **Failover** | Automatic provider switching, Recovery detection, Graceful degradation | P0 |
| 6 | **Database Layer** | SQLite with SQL Cipher, Connection pooling, Migrations, CRUD operations | P1 |
| 7 | **Configuration** | YAML/JSON configs, Environment variable substitution, Export formats | P1 |
| 8 | **REST API** | All endpoints (models, providers, verification, export, analytics) | P1 |
| 9 | **Authentication** | JWT, LDAP/SSO, RBAC, Compliance logging | P1 |
| 10 | **Event System** | Slack, Email, Telegram, Matrix, WebSocket, gRPC notifications | P2 |
| 11 | **Monitoring** | Prometheus metrics, Grafana dashboards, Advanced monitoring | P2 |
| 12 | **Analytics** | Usage analytics, Predictive analytics, Trend analysis | P2 |
| 13 | **Scheduling** | Periodic re-verification, Configurable intervals | P2 |
| 14 | **CLI Interface** | Model commands, Provider commands, Batch operations | P2 |
| 15 | **TUI Interface** | Interactive dashboard, Real-time updates | P3 |
| 16 | **Web Interface** | Angular frontend, WebSocket updates | P3 |
| 17 | **Go SDK** | Full API coverage, Async support | P2 |
| 18 | **Python SDK** | Async/await, Type hints | P2 |
| 19 | **JavaScript SDK** | TypeScript definitions | P2 |
| 20 | **ACP Protocol** | JSON-RPC, Tool calling, Context management | P1 |
| 21 | **Brotli Compression** | HTTP/3 support, Automatic detection | P2 |
| 22 | **Challenge System** | Provider discovery, Model verification, Config generation | P2 |
| 23 | **Models.dev Integration** | Dynamic model discovery, Pricing data | P1 |
| 24 | **Kubernetes Support** | Helm charts, K8s manifests, HPA | P2 |
| 25 | **Docker Support** | Dockerfile, docker-compose, Multi-stage builds | P2 |
| 26 | **Enhanced Features** | Context management, Vector DB, Supervision, Checkpointing | P3 |
| 27 | **Enterprise Features** | Multi-tenancy, Audit logging, Compliance | P3 |
| 28 | **Partner Integrations** | Third-party integrations | P3 |
| 29 | **Logging System** | Structured JSON, File rotation, Database logging | P2 |
| 30 | **Rate Limiting** | Per-IP, Per-user, Configurable limits | P1 |

---

## Phase 1: Core Infrastructure

### 1.1 Submodule Configuration

**Objective**: Establish proper Go module integration with LLMsVerifier submodule.

#### 1.1.1 Go Module Integration

**File**: `go.mod` (update)

```go
module dev.helix.agent

go 1.23

require (
    // Existing dependencies...
    llm-verifier v0.0.0
)

replace llm-verifier => ./LLMsVerifier/llm-verifier
```

#### 1.1.2 Package Structure

**New Packages**:
```
internal/
├── verifier/                    # LLMsVerifier integration layer
│   ├── service.go               # Main verification service
│   ├── scoring.go               # Scoring engine wrapper
│   ├── health.go                # Health checker wrapper
│   ├── providers.go             # Provider registry integration
│   ├── config.go                # Configuration adapter
│   ├── database.go              # Database integration
│   └── events.go                # Event system integration
├── verifier/adapters/           # Adapter implementations
│   ├── provider_adapter.go      # Provider interface adapter
│   ├── scoring_adapter.go       # Scoring interface adapter
│   └── notification_adapter.go  # Notification adapter
└── verifier/models/             # Shared data models
    ├── verification.go          # Verification models
    ├── scoring.go               # Scoring models
    └── provider.go              # Provider models
```

#### 1.1.3 Build System Updates

**File**: `Makefile` (additions)

```makefile
# LLMsVerifier targets
.PHONY: verifier-build verifier-test verifier-lint

verifier-build:
	cd LLMsVerifier && make build

verifier-test:
	cd LLMsVerifier && make test

verifier-lint:
	cd LLMsVerifier && make lint

# Combined targets
build-all: build verifier-build

test-all: test verifier-test

# Submodule management
submodule-update:
	git submodule update --init --recursive
	git submodule update --remote LLMsVerifier

submodule-status:
	git submodule status
```

### 1.2 Database Integration

**Objective**: Integrate LLMsVerifier SQLite database with HelixAgent PostgreSQL.

#### 1.2.1 Dual Database Strategy

**File**: `internal/verifier/database.go`

```go
package verifier

import (
    "llm-verifier/database"
    pgdb "dev.helix.agent/internal/database"
)

// DatabaseBridge bridges LLMsVerifier SQLite and HelixAgent PostgreSQL
type DatabaseBridge struct {
    verifierDB *database.Database  // LLMsVerifier SQLite
    helixagentDB *pgdb.Database    // HelixAgent PostgreSQL
}

// NewDatabaseBridge creates a new database bridge
func NewDatabaseBridge(verifierDBPath string, pgConfig *pgdb.Config) (*DatabaseBridge, error) {
    // Initialize LLMsVerifier database
    verifierDB, err := database.New(verifierDBPath)
    if err != nil {
        return nil, fmt.Errorf("failed to init verifier DB: %w", err)
    }

    // Connect to HelixAgent PostgreSQL
    helixagentDB, err := pgdb.New(pgConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to init helixagent DB: %w", err)
    }

    return &DatabaseBridge{
        verifierDB: verifierDB,
        helixagentDB: helixagentDB,
    }, nil
}

// SyncVerificationResults syncs verification results to PostgreSQL
func (db *DatabaseBridge) SyncVerificationResults() error {
    results, err := db.verifierDB.GetAllVerificationResults()
    if err != nil {
        return err
    }

    for _, result := range results {
        if err := db.helixagentDB.UpsertVerificationResult(result); err != nil {
            return err
        }
    }
    return nil
}
```

#### 1.2.2 PostgreSQL Schema Extensions

**File**: `internal/database/migrations/xxx_add_verification_tables.sql`

```sql
-- Verification Results Table
CREATE TABLE IF NOT EXISTS llmsverifier_results (
    id BIGSERIAL PRIMARY KEY,
    model_id VARCHAR(255) NOT NULL,
    provider_name VARCHAR(100) NOT NULL,
    verification_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    overall_score DECIMAL(5,2),
    code_capability_score DECIMAL(5,2),
    responsiveness_score DECIMAL(5,2),
    reliability_score DECIMAL(5,2),
    feature_richness_score DECIMAL(5,2),
    value_proposition_score DECIMAL(5,2),
    supports_code_generation BOOLEAN DEFAULT FALSE,
    supports_code_completion BOOLEAN DEFAULT FALSE,
    supports_code_review BOOLEAN DEFAULT FALSE,
    supports_streaming BOOLEAN DEFAULT FALSE,
    supports_reasoning BOOLEAN DEFAULT FALSE,
    avg_latency_ms INTEGER,
    p95_latency_ms INTEGER,
    throughput_rps DECIMAL(10,2),
    verified_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_llmsverifier_results_model_id ON llmsverifier_results(model_id);
CREATE INDEX idx_llmsverifier_results_provider ON llmsverifier_results(provider_name);
CREATE INDEX idx_llmsverifier_results_score ON llmsverifier_results(overall_score DESC);

-- Verification Scores Table
CREATE TABLE IF NOT EXISTS llmsverifier_scores (
    id BIGSERIAL PRIMARY KEY,
    model_id VARCHAR(255) NOT NULL,
    overall_score DECIMAL(5,2) NOT NULL,
    speed_score DECIMAL(5,2),
    efficiency_score DECIMAL(5,2),
    cost_score DECIMAL(5,2),
    capability_score DECIMAL(5,2),
    recency_score DECIMAL(5,2),
    score_suffix VARCHAR(20),
    data_source VARCHAR(50),
    calculated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_llmsverifier_scores_model_id ON llmsverifier_scores(model_id);
CREATE INDEX idx_llmsverifier_scores_overall ON llmsverifier_scores(overall_score DESC);

-- Provider Health Status Table
CREATE TABLE IF NOT EXISTS llmsverifier_provider_health (
    id BIGSERIAL PRIMARY KEY,
    provider_id VARCHAR(100) NOT NULL UNIQUE,
    provider_name VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'unknown',
    circuit_breaker_state VARCHAR(50) DEFAULT 'closed',
    failure_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    last_success_at TIMESTAMP WITH TIME ZONE,
    last_failure_at TIMESTAMP WITH TIME ZONE,
    avg_response_time_ms INTEGER,
    uptime_percentage DECIMAL(5,2),
    checked_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_llmsverifier_provider_health_provider ON llmsverifier_provider_health(provider_id);
CREATE INDEX idx_llmsverifier_provider_health_status ON llmsverifier_provider_health(status);

-- Verification Events Table
CREATE TABLE IF NOT EXISTS llmsverifier_events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'info',
    model_id VARCHAR(255),
    provider_id VARCHAR(100),
    message TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_llmsverifier_events_type ON llmsverifier_events(event_type);
CREATE INDEX idx_llmsverifier_events_created ON llmsverifier_events(created_at DESC);
```

### 1.3 Configuration Integration

**Objective**: Unified configuration system supporting both systems.

#### 1.3.1 Configuration Schema

**File**: `configs/verifier.yaml`

```yaml
# LLMsVerifier Configuration
verifier:
  enabled: true
  database:
    path: "./data/llm-verifier.db"
    encryption_enabled: true
    encryption_key: "${VERIFIER_DB_ENCRYPTION_KEY}"

  # Verification Settings
  verification:
    mandatory_code_check: true
    code_visibility_prompt: "Do you see my code?"
    verification_timeout: 60s
    retry_count: 3
    retry_delay: 5s

    # Test Categories
    tests:
      - existence
      - responsiveness
      - latency
      - streaming
      - function_calling
      - vision
      - embeddings
      - coding_capability
      - error_detection
      - rate_limit_detection

  # Scoring Configuration
  scoring:
    weights:
      response_speed: 0.25
      model_efficiency: 0.20
      cost_effectiveness: 0.25
      capability: 0.20
      recency: 0.10
    models_dev:
      enabled: true
      endpoint: "https://api.models.dev"
      cache_ttl: 24h

  # Health Checking
  health:
    check_interval: 30s
    timeout: 10s
    failure_threshold: 5
    recovery_threshold: 3
    circuit_breaker:
      enabled: true
      half_open_timeout: 60s

  # API Configuration
  api:
    enabled: true
    port: 8081
    base_path: "/api/v1/verifier"
    jwt_secret: "${VERIFIER_JWT_SECRET}"
    rate_limit:
      enabled: true
      requests_per_minute: 100

  # Event Notifications
  events:
    slack:
      enabled: false
      webhook_url: "${SLACK_WEBHOOK_URL}"
    email:
      enabled: false
      smtp_host: "${SMTP_HOST}"
      smtp_port: 587
    telegram:
      enabled: false
      bot_token: "${TELEGRAM_BOT_TOKEN}"
      chat_id: "${TELEGRAM_CHAT_ID}"
    websocket:
      enabled: true
      path: "/ws/verifier/events"

  # Monitoring
  monitoring:
    prometheus:
      enabled: true
      path: "/metrics/verifier"
    grafana:
      enabled: true
      dashboard_path: "./monitoring/grafana/verifier"

  # Brotli Compression
  brotli:
    enabled: true
    http3_support: true
    compression_level: 6

  # Challenge System
  challenges:
    enabled: true
    provider_discovery: true
    model_verification: true
    config_generation: true

  # Scheduling
  scheduling:
    re_verification:
      enabled: true
      interval: 24h
    score_recalculation:
      enabled: true
      interval: 12h

# Provider Configuration (merged with HelixAgent providers)
providers:
  # LLMsVerifier Additional Providers
  together_ai:
    enabled: true
    api_key: "${TOGETHER_API_KEY}"
    endpoint: "https://api.together.xyz/v1"

  mistral:
    enabled: true
    api_key: "${MISTRAL_API_KEY}"
    endpoint: "https://api.mistral.ai/v1"

  xai:
    enabled: true
    api_key: "${XAI_API_KEY}"
    endpoint: "https://api.x.ai/v1"

  replicate:
    enabled: true
    api_key: "${REPLICATE_API_KEY}"
    endpoint: "https://api.replicate.com/v1"

  cerebras:
    enabled: true
    api_key: "${CEREBRAS_API_KEY}"
    endpoint: "https://api.cerebras.ai/v1"

  cloudflare_workers_ai:
    enabled: true
    api_key: "${CLOUDFLARE_API_KEY}"
    account_id: "${CLOUDFLARE_ACCOUNT_ID}"
    endpoint: "https://api.cloudflare.com/client/v4/accounts/${CLOUDFLARE_ACCOUNT_ID}/ai/run"

  siliconflow:
    enabled: true
    api_key: "${SILICONFLOW_API_KEY}"
    endpoint: "https://api.siliconflow.cn/v1"

  groq:
    enabled: true
    api_key: "${GROQ_API_KEY}"
    endpoint: "https://api.groq.com/openai/v1"
```

#### 1.3.2 Configuration Loader

**File**: `internal/verifier/config.go`

```go
package verifier

import (
    "os"
    "time"

    "github.com/spf13/viper"
    "llm-verifier/config"
)

// Config represents the integrated verifier configuration
type Config struct {
    Enabled bool                    `yaml:"enabled"`
    Database DatabaseConfig         `yaml:"database"`
    Verification VerificationConfig `yaml:"verification"`
    Scoring ScoringConfig           `yaml:"scoring"`
    Health HealthConfig             `yaml:"health"`
    API APIConfig                   `yaml:"api"`
    Events EventsConfig             `yaml:"events"`
    Monitoring MonitoringConfig     `yaml:"monitoring"`
    Brotli BrotliConfig             `yaml:"brotli"`
    Challenges ChallengesConfig     `yaml:"challenges"`
    Scheduling SchedulingConfig     `yaml:"scheduling"`
}

// LoadConfig loads verifier configuration
func LoadConfig(configPath string) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(configPath)
    v.SetConfigType("yaml")

    // Environment variable substitution
    v.AutomaticEnv()

    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }

    var cfg Config
    if err := v.UnmarshalKey("verifier", &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}

// ToLLMVerifierConfig converts to LLMsVerifier native config
func (c *Config) ToLLMVerifierConfig() *config.Config {
    // Convert to LLMsVerifier format
    return &config.Config{
        // ... mapping
    }
}
```

### 1.4 Tests for Phase 1

**Test Types**: Unit, Integration

**Files**:
- `internal/verifier/database_test.go`
- `internal/verifier/config_test.go`
- `tests/integration/verifier/infrastructure_test.go`

```go
// internal/verifier/database_test.go
package verifier

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestDatabaseBridge_New(t *testing.T) {
    t.Run("successful initialization", func(t *testing.T) {
        bridge, err := NewDatabaseBridge(":memory:", testPgConfig())
        require.NoError(t, err)
        assert.NotNil(t, bridge)
    })

    t.Run("handles invalid verifier path", func(t *testing.T) {
        _, err := NewDatabaseBridge("/invalid/path/db.db", testPgConfig())
        assert.Error(t, err)
    })
}

func TestDatabaseBridge_SyncVerificationResults(t *testing.T) {
    bridge := setupTestBridge(t)

    // Insert test data
    insertTestVerificationResult(t, bridge.verifierDB)

    // Sync
    err := bridge.SyncVerificationResults()
    require.NoError(t, err)

    // Verify in PostgreSQL
    results, err := bridge.helixagentDB.GetVerificationResults()
    require.NoError(t, err)
    assert.Len(t, results, 1)
}
```

---

## Phase 2: Provider Integration

### 2.1 Unified Provider Registry

**Objective**: Merge LLMsVerifier 12+ providers with HelixAgent 7 providers.

#### 2.1.1 Provider Interface Extension

**File**: `internal/llm/provider_interface.go` (extend)

```go
package llm

import (
    "context"

    verifierProviders "llm-verifier/providers"
)

// ExtendedLLMProvider extends the base LLMProvider with verification capabilities
type ExtendedLLMProvider interface {
    LLMProvider

    // LLMsVerifier capabilities
    Verify(ctx context.Context, req *VerificationRequest) (*VerificationResult, error)
    GetScore(ctx context.Context) (*ScoringResult, error)
    GetCapabilities(ctx context.Context) (*ProviderCapabilities, error)
    HealthCheck(ctx context.Context) (*HealthStatus, error)

    // Dynamic model discovery
    DiscoverModels(ctx context.Context) ([]*ModelInfo, error)

    // Brotli support
    SupportsBrotli() bool
}

// ProviderCapabilities represents provider capabilities
type ProviderCapabilities struct {
    SupportsStreaming        bool     `json:"supports_streaming"`
    SupportsFunctionCalling  bool     `json:"supports_function_calling"`
    SupportsVision           bool     `json:"supports_vision"`
    SupportsEmbeddings       bool     `json:"supports_embeddings"`
    SupportsCodeGeneration   bool     `json:"supports_code_generation"`
    SupportsCodeCompletion   bool     `json:"supports_code_completion"`
    SupportsCodeReview       bool     `json:"supports_code_review"`
    SupportsReasoning        bool     `json:"supports_reasoning"`
    SupportsJSONMode         bool     `json:"supports_json_mode"`
    SupportsStructuredOutput bool     `json:"supports_structured_output"`
    SupportsBrotli           bool     `json:"supports_brotli"`
    SupportedLanguages       []string `json:"supported_languages"`
    MaxContextWindow         int      `json:"max_context_window"`
}

// HealthStatus represents provider health
type HealthStatus struct {
    Healthy       bool    `json:"healthy"`
    ResponseTime  int64   `json:"response_time_ms"`
    ErrorRate     float64 `json:"error_rate"`
    CircuitState  string  `json:"circuit_state"` // closed, open, half-open
    LastCheckedAt string  `json:"last_checked_at"`
}
```

#### 2.1.2 Provider Adapter Pattern

**File**: `internal/verifier/adapters/provider_adapter.go`

```go
package adapters

import (
    "context"

    "dev.helix.agent/internal/llm"
    verifierProviders "llm-verifier/providers"
)

// VerifierProviderAdapter wraps LLMsVerifier provider for HelixAgent
type VerifierProviderAdapter struct {
    provider    verifierProviders.Provider
    name        string
    modelID     string
    apiKey      string
    endpoint    string
    capabilities *llm.ProviderCapabilities
}

// NewVerifierProviderAdapter creates a new adapter
func NewVerifierProviderAdapter(
    providerType string,
    name string,
    apiKey string,
    endpoint string,
) (*VerifierProviderAdapter, error) {
    // Create underlying LLMsVerifier provider
    provider, err := verifierProviders.NewProvider(providerType, apiKey, endpoint)
    if err != nil {
        return nil, err
    }

    return &VerifierProviderAdapter{
        provider: provider,
        name:     name,
        apiKey:   apiKey,
        endpoint: endpoint,
    }, nil
}

// Complete implements LLMProvider.Complete
func (a *VerifierProviderAdapter) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    // Convert HelixAgent request to LLMsVerifier format
    verifierReq := convertToVerifierRequest(req)

    // Call LLMsVerifier provider
    verifierResp, err := a.provider.Complete(ctx, verifierReq)
    if err != nil {
        return nil, err
    }

    // Convert back to HelixAgent format
    return convertToHelixAgentResponse(verifierResp), nil
}

// CompleteStream implements LLMProvider.CompleteStream
func (a *VerifierProviderAdapter) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (<-chan *llm.StreamChunk, error) {
    // Stream implementation
    return a.provider.CompleteStream(ctx, convertToVerifierRequest(req))
}

// Verify implements ExtendedLLMProvider.Verify
func (a *VerifierProviderAdapter) Verify(ctx context.Context, req *llm.VerificationRequest) (*llm.VerificationResult, error) {
    return a.provider.Verify(ctx, req)
}

// HealthCheck implements ExtendedLLMProvider.HealthCheck
func (a *VerifierProviderAdapter) HealthCheck(ctx context.Context) (*llm.HealthStatus, error) {
    return a.provider.HealthCheck(ctx)
}

// DiscoverModels implements ExtendedLLMProvider.DiscoverModels
func (a *VerifierProviderAdapter) DiscoverModels(ctx context.Context) ([]*llm.ModelInfo, error) {
    return a.provider.DiscoverModels(ctx)
}
```

#### 2.1.3 Extended Provider Registry

**File**: `internal/services/extended_provider_registry.go`

```go
package services

import (
    "context"
    "sync"

    "dev.helix.agent/internal/llm"
    "dev.helix.agent/internal/verifier/adapters"
)

// ExtendedProviderRegistry manages all providers including LLMsVerifier providers
type ExtendedProviderRegistry struct {
    mu              sync.RWMutex
    providers       map[string]llm.ExtendedLLMProvider
    healthChecker   *HealthChecker
    scoringEngine   *ScoringEngine
    verifierService *VerifierService
}

// NewExtendedProviderRegistry creates a new extended registry
func NewExtendedProviderRegistry(cfg *Config) (*ExtendedProviderRegistry, error) {
    registry := &ExtendedProviderRegistry{
        providers: make(map[string]llm.ExtendedLLMProvider),
    }

    // Register HelixAgent native providers
    if err := registry.registerNativeProviders(cfg); err != nil {
        return nil, err
    }

    // Register LLMsVerifier providers
    if err := registry.registerVerifierProviders(cfg); err != nil {
        return nil, err
    }

    return registry, nil
}

// registerVerifierProviders registers all LLMsVerifier providers
func (r *ExtendedProviderRegistry) registerVerifierProviders(cfg *Config) error {
    verifierProviders := []struct {
        name     string
        typ      string
        apiKey   string
        endpoint string
    }{
        {"together_ai", "together", cfg.TogetherAPIKey, cfg.TogetherEndpoint},
        {"mistral", "mistral", cfg.MistralAPIKey, cfg.MistralEndpoint},
        {"xai", "xai", cfg.XAIAPIKey, cfg.XAIEndpoint},
        {"replicate", "replicate", cfg.ReplicateAPIKey, cfg.ReplicateEndpoint},
        {"cerebras", "cerebras", cfg.CerebrasAPIKey, cfg.CerebrasEndpoint},
        {"cloudflare_workers_ai", "cloudflare", cfg.CloudflareAPIKey, cfg.CloudflareEndpoint},
        {"siliconflow", "siliconflow", cfg.SiliconFlowAPIKey, cfg.SiliconFlowEndpoint},
        {"groq", "groq", cfg.GroqAPIKey, cfg.GroqEndpoint},
    }

    for _, p := range verifierProviders {
        if p.apiKey == "" {
            continue // Skip unconfigured providers
        }

        adapter, err := adapters.NewVerifierProviderAdapter(p.typ, p.name, p.apiKey, p.endpoint)
        if err != nil {
            return fmt.Errorf("failed to create %s provider: %w", p.name, err)
        }

        r.Register(p.name, adapter)
    }

    return nil
}

// GetVerifiedProviders returns only providers that passed verification
func (r *ExtendedProviderRegistry) GetVerifiedProviders(ctx context.Context) ([]llm.ExtendedLLMProvider, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var verified []llm.ExtendedLLMProvider
    for _, provider := range r.providers {
        result, err := provider.Verify(ctx, &llm.VerificationRequest{
            VerifyCode: true,
        })
        if err == nil && result.Verified {
            verified = append(verified, provider)
        }
    }

    return verified, nil
}

// GetProvidersByScore returns providers sorted by score
func (r *ExtendedProviderRegistry) GetProvidersByScore(ctx context.Context, minScore float64) ([]llm.ExtendedLLMProvider, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var scored []struct {
        provider llm.ExtendedLLMProvider
        score    float64
    }

    for _, provider := range r.providers {
        scoreResult, err := provider.GetScore(ctx)
        if err != nil {
            continue
        }
        if scoreResult.OverallScore >= minScore {
            scored = append(scored, struct {
                provider llm.ExtendedLLMProvider
                score    float64
            }{provider, scoreResult.OverallScore})
        }
    }

    // Sort by score descending
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].score > scored[j].score
    })

    result := make([]llm.ExtendedLLMProvider, len(scored))
    for i, s := range scored {
        result[i] = s.provider
    }

    return result, nil
}
```

### 2.2 Tests for Phase 2

**Test Types**: Unit, Integration

```go
// internal/verifier/adapters/provider_adapter_test.go
func TestVerifierProviderAdapter_Complete(t *testing.T) {
    tests := []struct {
        name     string
        provider string
        wantErr  bool
    }{
        {"together_ai success", "together", false},
        {"mistral success", "mistral", false},
        {"groq success", "groq", false},
        {"invalid provider", "invalid", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            adapter, err := NewVerifierProviderAdapter(tt.provider, "test", "test-key", "https://api.test.com")
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)

            resp, err := adapter.Complete(context.Background(), &llm.CompletionRequest{
                Prompt: "Hello",
            })
            require.NoError(t, err)
            assert.NotEmpty(t, resp.Content)
        })
    }
}

func TestExtendedProviderRegistry_GetVerifiedProviders(t *testing.T) {
    registry := setupTestRegistry(t)

    providers, err := registry.GetVerifiedProviders(context.Background())
    require.NoError(t, err)

    // All test providers should be verified
    for _, p := range providers {
        result, err := p.Verify(context.Background(), &llm.VerificationRequest{})
        require.NoError(t, err)
        assert.True(t, result.Verified)
    }
}
```

---

## Phase 3: Verification System

### 3.1 "Do You See My Code?" Verification Integration

**Objective**: Mandatory code visibility verification for all models.

#### 3.1.1 Verification Service

**File**: `internal/verifier/service.go`

```go
package verifier

import (
    "context"
    "fmt"
    "strings"
    "time"

    "llm-verifier/verification"
    "llm-verifier/database"
)

// VerificationService manages all verification operations
type VerificationService struct {
    verifier    *verification.Verifier
    codeVerifier *verification.CodeVerificationService
    db          *database.Database
    config      *Config
}

// NewVerificationService creates a new verification service
func NewVerificationService(db *database.Database, cfg *Config) *VerificationService {
    return &VerificationService{
        verifier:    verification.NewVerifier(db),
        codeVerifier: verification.NewCodeVerificationService(db),
        db:          db,
        config:      cfg,
    }
}

// VerifyModel performs complete model verification
func (s *VerificationService) VerifyModel(ctx context.Context, modelID string, provider string) (*VerificationResult, error) {
    result := &VerificationResult{
        ModelID:   modelID,
        Provider:  provider,
        StartedAt: time.Now(),
        Tests:     make([]TestResult, 0),
    }

    // 1. Mandatory "Do you see my code?" verification
    codeResult, err := s.verifyCodeVisibility(ctx, modelID, provider)
    if err != nil {
        result.Status = "failed"
        result.ErrorMessage = fmt.Sprintf("code visibility check failed: %v", err)
        return result, err
    }
    result.Tests = append(result.Tests, *codeResult)
    result.CodeVerified = codeResult.Passed

    // 2. Existence test
    existenceResult, err := s.verifyExistence(ctx, modelID, provider)
    result.Tests = append(result.Tests, *existenceResult)

    // 3. Responsiveness test
    responsivenessResult, err := s.verifyResponsiveness(ctx, modelID, provider)
    result.Tests = append(result.Tests, *responsivenessResult)

    // 4. Latency test
    latencyResult, err := s.verifyLatency(ctx, modelID, provider)
    result.Tests = append(result.Tests, *latencyResult)

    // 5. Streaming test
    streamingResult, err := s.verifyStreaming(ctx, modelID, provider)
    result.Tests = append(result.Tests, *streamingResult)

    // 6. Function calling test
    functionCallingResult, err := s.verifyFunctionCalling(ctx, modelID, provider)
    result.Tests = append(result.Tests, *functionCallingResult)

    // 7. Vision test (if applicable)
    visionResult, err := s.verifyVision(ctx, modelID, provider)
    result.Tests = append(result.Tests, *visionResult)

    // 8. Embeddings test
    embeddingsResult, err := s.verifyEmbeddings(ctx, modelID, provider)
    result.Tests = append(result.Tests, *embeddingsResult)

    // 9. Coding capability test (>80%)
    codingResult, err := s.verifyCodingCapability(ctx, modelID, provider)
    result.Tests = append(result.Tests, *codingResult)
    result.CodingCapabilityScore = codingResult.Score

    // 10. Error detection test
    errorResult, err := s.verifyErrorDetection(ctx, modelID, provider)
    result.Tests = append(result.Tests, *errorResult)

    // 11. Rate limit detection
    rateLimitResult, err := s.verifyRateLimitDetection(ctx, modelID, provider)
    result.Tests = append(result.Tests, *rateLimitResult)

    // Calculate overall status
    result.CompletedAt = time.Now()
    result.OverallScore = s.calculateOverallScore(result.Tests)

    if result.CodeVerified && result.OverallScore >= 60 {
        result.Status = "verified"
    } else {
        result.Status = "failed"
    }

    // Store result
    if err := s.storeVerificationResult(ctx, result); err != nil {
        return result, fmt.Errorf("failed to store result: %w", err)
    }

    return result, nil
}

// verifyCodeVisibility performs the mandatory "Do you see my code?" test
func (s *VerificationService) verifyCodeVisibility(ctx context.Context, modelID, provider string) (*TestResult, error) {
    result := &TestResult{
        Name:      "code_visibility",
        StartedAt: time.Now(),
    }

    // Code samples in multiple languages
    codeSamples := []struct {
        language string
        code     string
    }{
        {"python", `def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)`},
        {"go", `func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}`},
        {"javascript", `function fibonacci(n) {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}`},
        {"java", `public int fibonacci(int n) {
    if (n <= 1) return n;
    return fibonacci(n - 1) + fibonacci(n - 2);
}`},
        {"csharp", `public int Fibonacci(int n) {
    if (n <= 1) return n;
    return Fibonacci(n - 1) + Fibonacci(n - 2);
}`},
    }

    passedCount := 0
    totalTests := len(codeSamples)

    for _, sample := range codeSamples {
        prompt := fmt.Sprintf(`I'm showing you code. Look at this %s code:

%s

Do you see my code? Please respond with "Yes, I can see your code" if you can see it.`, sample.language, sample.code)

        response, err := s.callModel(ctx, modelID, provider, prompt)
        if err != nil {
            result.Details = append(result.Details, fmt.Sprintf("%s: error - %v", sample.language, err))
            continue
        }

        // Check for affirmative response
        if s.isAffirmativeCodeResponse(response) {
            passedCount++
            result.Details = append(result.Details, fmt.Sprintf("%s: passed", sample.language))
        } else {
            result.Details = append(result.Details, fmt.Sprintf("%s: failed - response: %s", sample.language, truncate(response, 100)))
        }
    }

    result.CompletedAt = time.Now()
    result.Score = float64(passedCount) / float64(totalTests) * 100
    result.Passed = result.Score >= 80 // Require 80% pass rate

    return result, nil
}

// isAffirmativeCodeResponse checks if the response confirms code visibility
func (s *VerificationService) isAffirmativeCodeResponse(response string) bool {
    response = strings.ToLower(response)

    affirmatives := []string{
        "yes, i can see",
        "yes i can see",
        "i can see your code",
        "i see your code",
        "i can see the code",
        "yes, i see",
        "yes i see",
        "affirmative",
        "visible",
        "i can view",
    }

    for _, phrase := range affirmatives {
        if strings.Contains(response, phrase) {
            return true
        }
    }

    return false
}

// BatchVerify verifies multiple models
func (s *VerificationService) BatchVerify(ctx context.Context, requests []*BatchVerificationRequest) ([]*VerificationResult, error) {
    results := make([]*VerificationResult, len(requests))
    errChan := make(chan error, len(requests))
    resultChan := make(chan *indexedResult, len(requests))

    for i, req := range requests {
        go func(index int, r *BatchVerificationRequest) {
            result, err := s.VerifyModel(ctx, r.ModelID, r.Provider)
            if err != nil {
                errChan <- err
                return
            }
            resultChan <- &indexedResult{index: index, result: result}
        }(i, req)
    }

    for i := 0; i < len(requests); i++ {
        select {
        case r := <-resultChan:
            results[r.index] = r.result
        case err := <-errChan:
            return nil, err
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }

    return results, nil
}
```

#### 3.1.2 Verification Handler

**File**: `internal/handlers/verification_handler.go`

```go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "dev.helix.agent/internal/verifier"
)

// VerificationHandler handles verification API endpoints
type VerificationHandler struct {
    service *verifier.VerificationService
}

// NewVerificationHandler creates a new verification handler
func NewVerificationHandler(service *verifier.VerificationService) *VerificationHandler {
    return &VerificationHandler{service: service}
}

// RegisterRoutes registers verification routes
func (h *VerificationHandler) RegisterRoutes(router *gin.RouterGroup) {
    v := router.Group("/verification")
    {
        v.POST("/verify", h.VerifyModel)
        v.POST("/verify/batch", h.BatchVerify)
        v.GET("/models/:model_id/status", h.GetVerificationStatus)
        v.GET("/results", h.GetVerificationResults)
        v.GET("/results/:id", h.GetVerificationResult)
        v.POST("/code-check", h.PerformCodeCheck)
    }
}

// VerifyModel triggers model verification
func (h *VerificationHandler) VerifyModel(c *gin.Context) {
    var req struct {
        ModelID  string `json:"model_id" binding:"required"`
        Provider string `json:"provider" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    result, err := h.service.VerifyModel(c.Request.Context(), req.ModelID, req.Provider)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, result)
}

// BatchVerify verifies multiple models
func (h *VerificationHandler) BatchVerify(c *gin.Context) {
    var req struct {
        Models []struct {
            ModelID  string `json:"model_id"`
            Provider string `json:"provider"`
        } `json:"models"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    requests := make([]*verifier.BatchVerificationRequest, len(req.Models))
    for i, m := range req.Models {
        requests[i] = &verifier.BatchVerificationRequest{
            ModelID:  m.ModelID,
            Provider: m.Provider,
        }
    }

    results, err := h.service.BatchVerify(c.Request.Context(), requests)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "total":   len(results),
        "results": results,
    })
}

// PerformCodeCheck performs standalone code visibility check
func (h *VerificationHandler) PerformCodeCheck(c *gin.Context) {
    var req struct {
        ModelID  string `json:"model_id" binding:"required"`
        Provider string `json:"provider" binding:"required"`
        Code     string `json:"code" binding:"required"`
        Language string `json:"language"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    result, err := h.service.PerformCodeCheck(c.Request.Context(), req.ModelID, req.Provider, req.Code, req.Language)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, result)
}
```

### 3.2 Tests for Phase 3

**Test Types**: Unit, Integration, E2E, Security

```go
// internal/verifier/service_test.go
func TestVerificationService_VerifyCodeVisibility(t *testing.T) {
    service := setupTestVerificationService(t)

    tests := []struct {
        name     string
        modelID  string
        provider string
        wantPass bool
    }{
        {"gpt-4 passes", "gpt-4", "openai", true},
        {"claude-3 passes", "claude-3-opus", "anthropic", true},
        {"gemini passes", "gemini-pro", "google", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := service.VerifyModel(context.Background(), tt.modelID, tt.provider)
            require.NoError(t, err)
            assert.Equal(t, tt.wantPass, result.CodeVerified)
        })
    }
}

func TestVerificationService_BatchVerify(t *testing.T) {
    service := setupTestVerificationService(t)

    requests := []*verifier.BatchVerificationRequest{
        {ModelID: "gpt-4", Provider: "openai"},
        {ModelID: "claude-3-opus", Provider: "anthropic"},
        {ModelID: "gemini-pro", Provider: "google"},
    }

    results, err := service.BatchVerify(context.Background(), requests)
    require.NoError(t, err)
    assert.Len(t, results, 3)

    for _, result := range results {
        assert.NotEmpty(t, result.Status)
        assert.NotEmpty(t, result.Tests)
    }
}

// tests/security/verification_security_test.go
func TestVerification_SQLInjection(t *testing.T) {
    handler := setupTestHandler(t)

    // Attempt SQL injection in model_id
    req := `{"model_id": "'; DROP TABLE models;--", "provider": "openai"}`
    resp := httptest.NewRecorder()

    c, _ := gin.CreateTestContext(resp)
    c.Request = httptest.NewRequest("POST", "/verification/verify", strings.NewReader(req))
    c.Request.Header.Set("Content-Type", "application/json")

    handler.VerifyModel(c)

    // Should return error, not execute SQL
    assert.Equal(t, http.StatusBadRequest, resp.Code)
}
```

---

## Phase 4: Scoring Engine

### 4.1 Scoring Integration

**Objective**: Integrate 5-component weighted scoring system.

#### 4.1.1 Scoring Service

**File**: `internal/verifier/scoring.go`

```go
package verifier

import (
    "context"
    "fmt"
    "time"

    "llm-verifier/scoring"
    "llm-verifier/database"
)

// ScoringService manages model scoring
type ScoringService struct {
    engine       *scoring.ScoringEngine
    modelsDevClient *scoring.ModelsDevClient
    db           *database.Database
    weights      *scoring.ScoreWeights
}

// NewScoringService creates a new scoring service
func NewScoringService(db *database.Database, cfg *Config) (*ScoringService, error) {
    modelsDevClient := scoring.NewModelsDevClient(cfg.Scoring.ModelsDevEndpoint)
    engine := scoring.NewScoringEngine(db, modelsDevClient, nil)

    return &ScoringService{
        engine:          engine,
        modelsDevClient: modelsDevClient,
        db:              db,
        weights: &scoring.ScoreWeights{
            ResponseSpeed:     cfg.Scoring.Weights.ResponseSpeed,
            ModelEfficiency:   cfg.Scoring.Weights.ModelEfficiency,
            CostEffectiveness: cfg.Scoring.Weights.CostEffectiveness,
            Capability:        cfg.Scoring.Weights.Capability,
            Recency:           cfg.Scoring.Weights.Recency,
        },
    }, nil
}

// CalculateScore calculates comprehensive score for a model
func (s *ScoringService) CalculateScore(ctx context.Context, modelID string) (*ScoringResult, error) {
    config := scoring.ScoringConfig{
        Weights: *s.weights,
    }

    score, err := s.engine.CalculateComprehensiveScore(ctx, modelID, config)
    if err != nil {
        return nil, err
    }

    return &ScoringResult{
        ModelID:      modelID,
        OverallScore: score.OverallScore,
        ScoreSuffix:  score.ScoreSuffix,
        Components: ScoreComponents{
            SpeedScore:      score.Components.SpeedScore,
            EfficiencyScore: score.Components.EfficiencyScore,
            CostScore:       score.Components.CostScore,
            CapabilityScore: score.Components.CapabilityScore,
            RecencyScore:    score.Components.RecencyScore,
        },
        CalculatedAt: score.LastCalculated,
    }, nil
}

// BatchCalculateScores calculates scores for multiple models
func (s *ScoringService) BatchCalculateScores(ctx context.Context, modelIDs []string) ([]*ScoringResult, error) {
    scores, err := s.engine.CalculateBatchScores(ctx, modelIDs, s.weights)
    if err != nil {
        return nil, err
    }

    results := make([]*ScoringResult, len(scores))
    for i, score := range scores {
        results[i] = &ScoringResult{
            ModelID:      score.ModelID,
            OverallScore: score.OverallScore,
            ScoreSuffix:  score.ScoreSuffix,
        }
    }

    return results, nil
}

// GetTopModels returns top scoring models
func (s *ScoringService) GetTopModels(ctx context.Context, limit int) ([]*ModelWithScore, error) {
    models, err := s.engine.GetTopModels(ctx, limit)
    if err != nil {
        return nil, err
    }

    results := make([]*ModelWithScore, len(models))
    for i, m := range models {
        results[i] = &ModelWithScore{
            ModelID:      m.ModelID,
            Name:         m.Name,
            Provider:     m.ProviderName,
            OverallScore: m.OverallScore,
            ScoreSuffix:  fmt.Sprintf("(SC:%.1f)", m.OverallScore),
        }
    }

    return results, nil
}

// UpdateWeights updates scoring weights
func (s *ScoringService) UpdateWeights(weights *scoring.ScoreWeights) {
    s.weights = weights
    s.engine.SetWeights(*weights)
}

// GetModelNameWithScore returns model name with score suffix
func (s *ScoringService) GetModelNameWithScore(ctx context.Context, modelID, modelName string) (string, error) {
    score, err := s.CalculateScore(ctx, modelID)
    if err != nil {
        return modelName, err
    }

    return fmt.Sprintf("%s %s", modelName, score.ScoreSuffix), nil
}
```

#### 4.1.2 Scoring Handler

**File**: `internal/handlers/scoring_handler.go`

```go
package handlers

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "dev.helix.agent/internal/verifier"
)

// ScoringHandler handles scoring API endpoints
type ScoringHandler struct {
    service *verifier.ScoringService
}

// RegisterRoutes registers scoring routes
func (h *ScoringHandler) RegisterRoutes(router *gin.RouterGroup) {
    s := router.Group("/scoring")
    {
        s.GET("/models/:model_id/score", h.GetModelScore)
        s.POST("/calculate", h.CalculateScore)
        s.POST("/batch", h.BatchCalculate)
        s.GET("/top", h.GetTopModels)
        s.PUT("/weights", h.UpdateWeights)
        s.GET("/weights", h.GetWeights)
    }
}

// GetModelScore returns model score
func (h *ScoringHandler) GetModelScore(c *gin.Context) {
    modelID := c.Param("model_id")

    result, err := h.service.CalculateScore(c.Request.Context(), modelID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, result)
}

// GetTopModels returns top scoring models
func (h *ScoringHandler) GetTopModels(c *gin.Context) {
    limitStr := c.DefaultQuery("limit", "10")
    limit, _ := strconv.Atoi(limitStr)

    models, err := h.service.GetTopModels(c.Request.Context(), limit)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "total":  len(models),
        "models": models,
    })
}
```

### 4.2 Tests for Phase 4

```go
func TestScoringService_CalculateScore(t *testing.T) {
    service := setupTestScoringService(t)

    result, err := service.CalculateScore(context.Background(), "gpt-4")
    require.NoError(t, err)

    assert.NotZero(t, result.OverallScore)
    assert.True(t, result.OverallScore >= 0 && result.OverallScore <= 10)
    assert.NotEmpty(t, result.ScoreSuffix)
    assert.Contains(t, result.ScoreSuffix, "SC:")
}

func TestScoringService_ComponentWeights(t *testing.T) {
    service := setupTestScoringService(t)

    // Verify weights sum to 1.0
    weights := service.GetWeights()
    sum := weights.ResponseSpeed + weights.ModelEfficiency +
           weights.CostEffectiveness + weights.Capability + weights.Recency
    assert.InDelta(t, 1.0, sum, 0.001)
}
```

---

## Phase 5: Health Monitoring & Failover

### 5.1 Health Checker Integration

**File**: `internal/verifier/health.go`

```go
package verifier

import (
    "context"
    "sync"
    "time"

    "llm-verifier/failover"
    "llm-verifier/database"
)

// HealthService manages provider health monitoring
type HealthService struct {
    checker        *failover.HealthChecker
    latencyRouter  *failover.LatencyRouter
    db             *database.Database
    circuitBreakers map[string]*failover.CircuitBreaker
    mu             sync.RWMutex
}

// NewHealthService creates a new health service
func NewHealthService(db *database.Database, cfg *Config) *HealthService {
    return &HealthService{
        checker:         failover.NewHealthChecker(db),
        latencyRouter:   failover.NewLatencyRouter(),
        db:              db,
        circuitBreakers: make(map[string]*failover.CircuitBreaker),
    }
}

// Start starts health monitoring
func (s *HealthService) Start() {
    s.checker.Start()
}

// Stop stops health monitoring
func (s *HealthService) Stop() {
    s.checker.Stop()
}

// AddProvider adds a provider to monitoring
func (s *HealthService) AddProvider(providerID string) {
    s.checker.AddProvider(providerID)

    s.mu.Lock()
    s.circuitBreakers[providerID] = failover.NewCircuitBreaker(providerID)
    s.mu.Unlock()
}

// GetHealthyProviders returns healthy providers
func (s *HealthService) GetHealthyProviders() []string {
    return s.checker.GetHealthyProviders()
}

// GetProviderHealth returns provider health status
func (s *HealthService) GetProviderHealth(providerID string) (*ProviderHealth, error) {
    cb := s.checker.GetCircuitBreaker(providerID)
    if cb == nil {
        return nil, fmt.Errorf("provider not found: %s", providerID)
    }

    return &ProviderHealth{
        ProviderID:     providerID,
        Healthy:        cb.IsAvailable(),
        CircuitState:   cb.State().String(),
        FailureCount:   cb.FailureCount(),
        SuccessCount:   cb.SuccessCount(),
        LastCheckedAt:  time.Now(),
    }, nil
}

// ExecuteWithFailover executes operation with automatic failover
func (s *HealthService) ExecuteWithFailover(ctx context.Context, providers []string, operation func(providerID string) error) error {
    for _, providerID := range providers {
        s.mu.RLock()
        cb := s.circuitBreakers[providerID]
        s.mu.RUnlock()

        if cb == nil || !cb.IsAvailable() {
            continue
        }

        err := cb.Call(func() error {
            return operation(providerID)
        })

        if err == nil {
            return nil // Success
        }

        // Try next provider
    }

    return fmt.Errorf("all providers failed")
}

// GetFastestProvider returns the fastest available provider
func (s *HealthService) GetFastestProvider(ctx context.Context, providers []string) (string, error) {
    return s.latencyRouter.GetFastest(providers)
}
```

### 5.2 Ensemble Enhancement with Failover

**File**: `internal/llm/ensemble_with_failover.go`

```go
package llm

import (
    "context"

    "dev.helix.agent/internal/verifier"
)

// EnsembleWithFailover enhances ensemble with health-aware routing
type EnsembleWithFailover struct {
    *Ensemble
    healthService *verifier.HealthService
}

// NewEnsembleWithFailover creates a new failover-aware ensemble
func NewEnsembleWithFailover(ensemble *Ensemble, health *verifier.HealthService) *EnsembleWithFailover {
    return &EnsembleWithFailover{
        Ensemble:      ensemble,
        healthService: health,
    }
}

// Complete performs completion with automatic failover
func (e *EnsembleWithFailover) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    healthyProviders := e.healthService.GetHealthyProviders()

    if len(healthyProviders) == 0 {
        return nil, fmt.Errorf("no healthy providers available")
    }

    var lastErr error
    err := e.healthService.ExecuteWithFailover(ctx, healthyProviders, func(providerID string) error {
        resp, err := e.Ensemble.CompleteWithProvider(ctx, req, providerID)
        if err != nil {
            lastErr = err
            return err
        }
        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("ensemble failed: %v (last error: %v)", err, lastErr)
    }

    return e.Ensemble.GetLastResponse(), nil
}
```

---

## Phase 6: API & Protocol Integration

### 6.1 OpenAI-Compatible API Extensions

**File**: `internal/handlers/openai_compat_extended.go`

```go
package handlers

// Extended OpenAI-compatible endpoints with verification
func (h *OpenAIHandler) RegisterExtendedRoutes(router *gin.RouterGroup) {
    // Standard OpenAI endpoints
    router.POST("/v1/chat/completions", h.ChatCompletions)
    router.POST("/v1/completions", h.Completions)
    router.GET("/v1/models", h.ListModels)
    router.GET("/v1/models/:model_id", h.GetModel)

    // Extended verification endpoints
    router.GET("/v1/models/:model_id/verification", h.GetModelVerification)
    router.GET("/v1/models/:model_id/score", h.GetModelScore)
    router.GET("/v1/models/verified", h.ListVerifiedModels)
    router.GET("/v1/models/top", h.ListTopModels)
}

// ListVerifiedModels returns only verified models
func (h *OpenAIHandler) ListVerifiedModels(c *gin.Context) {
    models, err := h.verifierService.GetVerifiedModels(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // Format as OpenAI models response
    data := make([]map[string]interface{}, len(models))
    for i, m := range models {
        data[i] = map[string]interface{}{
            "id":            m.ModelID,
            "object":        "model",
            "created":       m.CreatedAt.Unix(),
            "owned_by":      m.Provider,
            "verified":      true,
            "overall_score": m.OverallScore,
            "score_suffix":  m.ScoreSuffix,
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "object": "list",
        "data":   data,
    })
}
```

### 6.2 ACP Protocol Integration

**File**: `internal/handlers/acp_extended.go`

```go
package handlers

import (
    "llm-verifier/acp"
)

// ACPExtendedHandler handles extended ACP operations
type ACPExtendedHandler struct {
    acpService      *acp.Service
    verifierService *verifier.VerificationService
}

// RegisterRoutes registers extended ACP routes
func (h *ACPExtendedHandler) RegisterRoutes(router *gin.RouterGroup) {
    a := router.Group("/acp")
    {
        a.POST("/verify", h.VerifyACPSupport)
        a.GET("/config/:provider", h.GetACPConfig)
        a.PUT("/config/:provider", h.UpdateACPConfig)
        a.GET("/models", h.ListACPCapableModels)
        a.GET("/results/:model_id", h.GetACPResults)
        a.POST("/challenges", h.RunACPChallenges)
        a.POST("/jsonrpc", h.HandleJSONRPC)
    }
}

// HandleJSONRPC handles JSON-RPC requests for ACP
func (h *ACPExtendedHandler) HandleJSONRPC(c *gin.Context) {
    var req acp.JSONRPCRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, acp.NewErrorResponse(req.ID, -32600, "Invalid Request"))
        return
    }

    response, err := h.acpService.HandleRequest(c.Request.Context(), &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, acp.NewErrorResponse(req.ID, -32603, err.Error()))
        return
    }

    c.JSON(http.StatusOK, response)
}
```

---

## Phase 7: Security & Authentication

### 7.1 Extended Authentication

**File**: `internal/middleware/verifier_auth.go`

```go
package middleware

import (
    "llm-verifier/auth"
)

// VerifierAuthMiddleware provides extended authentication
type VerifierAuthMiddleware struct {
    authManager     *auth.AuthManager
    rbacManager     *auth.RBACManager
    ldapClient      *auth.LDAPClient
    complianceLog   *auth.ComplianceLogger
}

// NewVerifierAuthMiddleware creates new auth middleware
func NewVerifierAuthMiddleware(cfg *Config) *VerifierAuthMiddleware {
    return &VerifierAuthMiddleware{
        authManager:   auth.NewAuthManager(cfg.JWTSecret),
        rbacManager:   auth.NewRBACManager(),
        ldapClient:    auth.NewLDAPClient(cfg.LDAPConfig),
        complianceLog: auth.NewComplianceLogger(cfg.ComplianceDB),
    }
}

// Authenticate validates JWT or LDAP credentials
func (m *VerifierAuthMiddleware) Authenticate() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")

        // Try JWT first
        claims, err := m.authManager.ValidateToken(token)
        if err == nil {
            c.Set("user", claims)
            c.Set("auth_method", "jwt")
            c.Next()
            return
        }

        // Try LDAP
        if m.ldapClient.IsEnabled() {
            user, err := m.ldapClient.AuthenticateFromRequest(c.Request)
            if err == nil {
                c.Set("user", user)
                c.Set("auth_method", "ldap")
                c.Next()
                return
            }
        }

        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
    }
}

// RequireRole enforces RBAC
func (m *VerifierAuthMiddleware) RequireRole(roles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user := c.MustGet("user")

        if !m.rbacManager.HasAnyRole(user, roles) {
            m.complianceLog.LogAccessDenied(c.Request, user, roles)
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
            return
        }

        c.Next()
    }
}
```

---

## Phase 8: Monitoring & Analytics

### 8.1 Prometheus Metrics Integration

**File**: `internal/monitoring/verifier_metrics.go`

```go
package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Verification metrics
    verificationsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llmsverifier_verifications_total",
            Help: "Total number of model verifications",
        },
        []string{"provider", "model", "status"},
    )

    verificationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "llmsverifier_verification_duration_seconds",
            Help:    "Verification duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"provider", "model"},
    )

    // Scoring metrics
    modelScores = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "llmsverifier_model_score",
            Help: "Model overall score",
        },
        []string{"provider", "model"},
    )

    scoreComponentGauge = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "llmsverifier_score_component",
            Help: "Score component values",
        },
        []string{"model", "component"},
    )

    // Health metrics
    providerHealthGauge = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "llmsverifier_provider_health",
            Help: "Provider health status (1=healthy, 0=unhealthy)",
        },
        []string{"provider"},
    )

    circuitBreakerState = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "llmsverifier_circuit_breaker_state",
            Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
        },
        []string{"provider"},
    )

    // Brotli compression metrics
    brotliCompressionRatio = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "llmsverifier_brotli_compression_ratio",
            Help: "Brotli compression ratio",
        },
        []string{"endpoint"},
    )
)

// RecordVerification records a verification
func RecordVerification(provider, model, status string, duration float64) {
    verificationsTotal.WithLabelValues(provider, model, status).Inc()
    verificationDuration.WithLabelValues(provider, model).Observe(duration)
}

// RecordScore records model score
func RecordScore(provider, model string, score float64, components map[string]float64) {
    modelScores.WithLabelValues(provider, model).Set(score)

    for component, value := range components {
        scoreComponentGauge.WithLabelValues(model, component).Set(value)
    }
}

// RecordHealth records provider health
func RecordHealth(provider string, healthy bool, circuitState int) {
    healthValue := 0.0
    if healthy {
        healthValue = 1.0
    }
    providerHealthGauge.WithLabelValues(provider).Set(healthValue)
    circuitBreakerState.WithLabelValues(provider).Set(float64(circuitState))
}
```

### 8.2 Grafana Dashboard

**File**: `monitoring/grafana/dashboards/llmsverifier.json`

```json
{
  "dashboard": {
    "title": "LLMsVerifier Dashboard",
    "panels": [
      {
        "title": "Verification Success Rate",
        "type": "gauge",
        "targets": [
          {
            "expr": "sum(rate(llmsverifier_verifications_total{status=\"verified\"}[5m])) / sum(rate(llmsverifier_verifications_total[5m])) * 100"
          }
        ]
      },
      {
        "title": "Model Scores Distribution",
        "type": "histogram",
        "targets": [
          {
            "expr": "llmsverifier_model_score"
          }
        ]
      },
      {
        "title": "Provider Health",
        "type": "stat",
        "targets": [
          {
            "expr": "llmsverifier_provider_health"
          }
        ]
      },
      {
        "title": "Verification Duration P95",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(llmsverifier_verification_duration_seconds_bucket[5m]))"
          }
        ]
      }
    ]
  }
}
```

---

## Phase 9: SDKs & Client Integration

### 9.1 Go SDK Wrapper

**File**: `pkg/sdk/go/verifier_client.go`

```go
package sdk

import (
    "context"

    "llm-verifier/client"
)

// VerifierClient wraps LLMsVerifier client for HelixAgent
type VerifierClient struct {
    client *client.Client
}

// NewVerifierClient creates a new verifier client
func NewVerifierClient(serverURL string) *VerifierClient {
    return &VerifierClient{
        client: client.New(serverURL),
    }
}

// VerifyModel verifies a model
func (c *VerifierClient) VerifyModel(ctx context.Context, modelID string) (*VerificationResult, error) {
    return c.client.VerifyModel(modelID)
}

// GetScore gets model score
func (c *VerifierClient) GetScore(ctx context.Context, modelID string) (*ScoringResult, error) {
    return c.client.GetModelScore(modelID)
}

// ListVerifiedModels lists verified models
func (c *VerifierClient) ListVerifiedModels(ctx context.Context) ([]*Model, error) {
    return c.client.GetModels()
}
```

### 9.2 Python SDK Wrapper

**File**: `pkg/sdk/python/helixagent_verifier/__init__.py`

```python
"""HelixAgent Verifier SDK"""

from .client import VerifierClient
from .models import VerificationResult, ScoringResult, Model

__all__ = ['VerifierClient', 'VerificationResult', 'ScoringResult', 'Model']
```

**File**: `pkg/sdk/python/helixagent_verifier/client.py`

```python
import aiohttp
from typing import List, Optional
from .models import VerificationResult, ScoringResult, Model


class VerifierClient:
    """HelixAgent Verifier Client"""

    def __init__(self, base_url: str, api_key: Optional[str] = None):
        self.base_url = base_url.rstrip('/')
        self.api_key = api_key
        self._session: Optional[aiohttp.ClientSession] = None

    async def __aenter__(self):
        headers = {}
        if self.api_key:
            headers['Authorization'] = f'Bearer {self.api_key}'
        self._session = aiohttp.ClientSession(headers=headers)
        return self

    async def __aexit__(self, *args):
        if self._session:
            await self._session.close()

    async def verify_model(self, model_id: str, provider: str) -> VerificationResult:
        """Verify a model"""
        async with self._session.post(
            f'{self.base_url}/api/v1/verifier/verification/verify',
            json={'model_id': model_id, 'provider': provider}
        ) as resp:
            data = await resp.json()
            return VerificationResult(**data)

    async def get_score(self, model_id: str) -> ScoringResult:
        """Get model score"""
        async with self._session.get(
            f'{self.base_url}/api/v1/verifier/scoring/models/{model_id}/score'
        ) as resp:
            data = await resp.json()
            return ScoringResult(**data)

    async def list_verified_models(self) -> List[Model]:
        """List verified models"""
        async with self._session.get(
            f'{self.base_url}/api/v1/verifier/models/verified'
        ) as resp:
            data = await resp.json()
            return [Model(**m) for m in data.get('models', [])]

    async def get_top_models(self, limit: int = 10) -> List[Model]:
        """Get top scoring models"""
        async with self._session.get(
            f'{self.base_url}/api/v1/verifier/scoring/top',
            params={'limit': limit}
        ) as resp:
            data = await resp.json()
            return [Model(**m) for m in data.get('models', [])]
```

---

## Phase 10: Documentation & Training

### 10.1 Documentation Structure

```
docs/
├── integration/
│   ├── LLMSVERIFIER_INTEGRATION_PLAN.md (this file)
│   ├── QUICK_START.md
│   ├── CONFIGURATION_GUIDE.md
│   └── TROUBLESHOOTING.md
├── api/
│   ├── VERIFICATION_API.md
│   ├── SCORING_API.md
│   ├── HEALTH_API.md
│   └── OPENAPI_SPEC.yaml
├── guides/
│   ├── USER_GUIDE.md
│   ├── DEVELOPER_GUIDE.md
│   ├── DEPLOYMENT_GUIDE.md
│   └── SECURITY_GUIDE.md
├── video-course/
│   ├── 01_INTRODUCTION.md
│   ├── 02_VERIFICATION_SYSTEM.md
│   ├── 03_SCORING_ENGINE.md
│   ├── 04_HEALTH_MONITORING.md
│   ├── 05_API_INTEGRATION.md
│   ├── 06_SDKS.md
│   ├── 07_DEPLOYMENT.md
│   └── 08_ADVANCED_TOPICS.md
└── reference/
    ├── ARCHITECTURE.md
    ├── DATA_MODELS.md
    └── GLOSSARY.md
```

### 10.2 Video Course Outline

1. **Introduction to LLMsVerifier Integration** (15 min)
2. **Setting Up the Verification System** (20 min)
3. **Understanding the Scoring Engine** (25 min)
4. **Health Monitoring and Failover** (20 min)
5. **API Integration and Usage** (30 min)
6. **Using the SDKs** (20 min)
7. **Deployment and Operations** (25 min)
8. **Advanced Topics and Best Practices** (30 min)

---

## Test Coverage Matrix

### Required Test Coverage: 100%

| Component | Unit | Integration | E2E | Security | Stress | Chaos |
|-----------|------|-------------|-----|----------|--------|-------|
| Database Bridge | 100% | 100% | 100% | 100% | 100% | 100% |
| Provider Adapters | 100% | 100% | 100% | 100% | 100% | 100% |
| Verification Service | 100% | 100% | 100% | 100% | 100% | 100% |
| Scoring Service | 100% | 100% | 100% | 100% | 100% | 100% |
| Health Service | 100% | 100% | 100% | 100% | 100% | 100% |
| API Handlers | 100% | 100% | 100% | 100% | 100% | 100% |
| Authentication | 100% | 100% | 100% | 100% | 100% | 100% |
| Monitoring | 100% | 100% | 100% | 100% | 100% | 100% |
| SDKs | 100% | 100% | 100% | 100% | 100% | 100% |
| Configuration | 100% | 100% | 100% | 100% | 100% | 100% |

### Test Files Structure

```
tests/
├── unit/
│   └── verifier/
│       ├── database_test.go
│       ├── service_test.go
│       ├── scoring_test.go
│       ├── health_test.go
│       └── config_test.go
├── integration/
│   └── verifier/
│       ├── infrastructure_test.go
│       ├── providers_test.go
│       ├── verification_test.go
│       ├── scoring_test.go
│       └── api_test.go
├── e2e/
│   └── verifier/
│       ├── full_verification_flow_test.go
│       ├── scoring_flow_test.go
│       └── failover_flow_test.go
├── security/
│   └── verifier/
│       ├── sql_injection_test.go
│       ├── auth_bypass_test.go
│       ├── xss_test.go
│       └── rate_limiting_test.go
├── stress/
│   └── verifier/
│       ├── concurrent_verification_test.go
│       ├── high_load_test.go
│       └── resource_exhaustion_test.go
└── chaos/
    └── verifier/
        ├── provider_failure_test.go
        ├── network_partition_test.go
        └── database_failure_test.go
```

---

## Deployment Strategy

### Docker Compose (Development)

```yaml
version: '3.8'

services:
  helixagent:
    build: .
    ports:
      - "8080:7061"
    environment:
      - VERIFIER_ENABLED=true
      - VERIFIER_DB_PATH=/data/llm-verifier.db
    volumes:
      - verifier-data:/data
    depends_on:
      - postgres
      - redis

  verifier-api:
    build:
      context: ./LLMsVerifier
    ports:
      - "8081:7061"
    environment:
      - DATABASE_PATH=/data/llm-verifier.db

volumes:
  verifier-data:
```

### Kubernetes (Production)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helixagent-with-verifier
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: helixagent
          image: helixagent:latest
          env:
            - name: VERIFIER_ENABLED
              value: "true"
          volumeMounts:
            - name: verifier-data
              mountPath: /data
      volumes:
        - name: verifier-data
          persistentVolumeClaim:
            claimName: verifier-pvc
```

---

## Summary

This integration plan provides a comprehensive, phased approach to integrating all LLMsVerifier features into HelixAgent:

- **10 Phases** covering all aspects of integration
- **30+ Feature Categories** fully integrated
- **100% Test Coverage** across all test types
- **Complete Documentation** including video course materials
- **Production-Ready Deployment** configurations

Each phase includes:
- Detailed implementation files
- Code examples
- Test specifications
- Documentation requirements

The integration ensures backward compatibility while significantly enhancing HelixAgent's capabilities with enterprise-grade LLM verification, scoring, and monitoring features.
