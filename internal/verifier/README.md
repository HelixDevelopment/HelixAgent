# Verifier Package

The `verifier` package implements the unified startup verification pipeline for HelixAgent, providing LLM provider discovery, verification, scoring, and dynamic team selection.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Startup Verification Pipeline                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Load Config          ──▶  Environment, credentials          │
│         │                                                        │
│         ▼                                                        │
│  2. Discover Providers   ──▶  API Key, OAuth, Free              │
│         │                                                        │
│         ▼                                                        │
│  3. Verify Providers     ──▶  8-test pipeline (parallel)        │
│         │                                                        │
│         ▼                                                        │
│  4. Score Providers      ──▶  5-component weighted scoring      │
│         │                                                        │
│         ▼                                                        │
│  5. Rank by Score        ──▶  OAuth priority when close         │
│         │                                                        │
│         ▼                                                        │
│  6. Select Debate Team   ──▶  15 LLMs (5 primary + 10 fallback) │
│         │                                                        │
│         ▼                                                        │
│  7. Start Server         ──▶  Verified configuration            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### Startup Verifier (`startup.go`)

Orchestrates the startup verification pipeline:

```go
verifier := verifier.NewStartupVerifier(config, logger)

// Run full verification pipeline
result, err := verifier.Verify(ctx)

// Get verified providers
providers := result.VerifiedProviders

// Get debate team
team := result.DebateTeam
```

### Provider Discovery (`discovery.go`)

Discovers available LLM providers:

```go
discovery := verifier.NewDiscovery(config, logger)

// Discover all providers
providers, err := discovery.DiscoverAll(ctx)

// Discover by type
apiKeyProviders := discovery.DiscoverAPIKeyProviders(ctx)
oauthProviders := discovery.DiscoverOAuthProviders(ctx)
freeProviders := discovery.DiscoverFreeProviders(ctx)
```

### Provider Types (`provider_types.go`)

Unified provider and model types:

```go
type UnifiedProvider struct {
    Name         string
    Type         ProviderType  // APIKey, OAuth, Free
    Verified     bool
    Score        float64
    Models       []UnifiedModel
    Capabilities *models.ProviderCapabilities
}

type UnifiedModel struct {
    ID           string
    Name         string
    Provider     string
    Capabilities ModelCapabilities
}
```

### Health Verification (`health.go`)

Provider health checking:

```go
health := verifier.NewHealthChecker(config)

// Check single provider
status, err := health.Check(ctx, provider)

// Check all providers
statuses := health.CheckAll(ctx, providers)
```

### Scoring (`scoring.go`)

5-component weighted scoring:

```go
scorer := verifier.NewScorer(config)

// Score a provider
score := scorer.Score(ctx, provider, verificationResult)

// Scoring components:
// - ResponseSpeed:     25%
// - ModelEfficiency:   20%
// - CostEffectiveness: 25%
// - Capability:        20%
// - Recency:           10%
```

### Database (`database.go`)

Verification result persistence:

```go
db := verifier.NewDatabase(pgPool, logger)

// Store verification result
err := db.StoreResult(ctx, result)

// Get historical results
history, err := db.GetHistory(ctx, provider, limit)
```

### Config (`config.go`)

Verification configuration:

```go
config := verifier.Config{
    Timeout:             30 * time.Second,
    ParallelVerifications: 8,
    MinimumScore:        5.0,
    OAuthBonus:          0.5,
    FreeProviderRange:   []float64{6.0, 7.0},
}
```

### Metrics (`metrics.go`)

Verification metrics:

```go
metrics := verifier.NewMetrics()

// Record verification
metrics.RecordVerification(provider, success, duration)

// Get statistics
stats := metrics.GetStats()
```

## Adapters Subpackage (`adapters/`)

Provider-specific verification adapters:

### OAuth Adapter (`oauth_adapter.go`)

For Claude and Qwen OAuth verification:

```go
adapter := adapters.NewOAuthAdapter(config)

// Verify OAuth provider
result, err := adapter.Verify(ctx, provider, credentials)
```

### Free Adapter (`free_adapter.go`)

For Zen and free OpenRouter models:

```go
adapter := adapters.NewFreeAdapter(config)

// Verify free provider (reduced verification)
result, err := adapter.Verify(ctx, provider)
```

## Provider Types

| Type | Providers | Auth | Verification |
|------|-----------|------|--------------|
| **API Key** | DeepSeek, Gemini, Mistral, OpenRouter, ZAI, Cerebras | Bearer token | Full 8-test |
| **OAuth** | Claude, Qwen | OAuth2 tokens | Trust on API failure option |
| **Free** | Zen, OpenRouter :free | Anonymous/X-Device-ID | Reduced, 6.0-7.0 score range |

## Scoring Algorithm

| Component | Weight | Description |
|-----------|--------|-------------|
| ResponseSpeed | 25% | API response latency |
| ModelEfficiency | 20% | Token efficiency |
| CostEffectiveness | 25% | Cost per token |
| Capability | 20% | Model capability score |
| Recency | 10% | Model release date |

- OAuth providers: +0.5 bonus when verified
- Free providers: 6.0-7.0 score range
- Minimum score to be selected: 5.0

## Files

| File | Description |
|------|-------------|
| `startup.go` | Startup verification orchestrator |
| `discovery.go` | Provider discovery |
| `provider_types.go` | Unified type definitions |
| `health.go` | Health checking |
| `scoring.go` | Provider scoring |
| `database.go` | Result persistence |
| `config.go` | Configuration |
| `metrics.go` | Verification metrics |
| `service.go` | Verification service |
| `adapters/` | Provider-specific adapters |

## Usage

### Startup Verification

```go
// Create verifier
verifier := verifier.NewStartupVerifier(config, logger)

// Run verification (typically at application startup)
result, err := verifier.Verify(ctx)
if err != nil {
    log.Fatalf("Verification failed: %v", err)
}

// Use verified providers
registry := services.NewProviderRegistry(result.VerifiedProviders)

// Use debate team
debateService := services.NewDebateService(result.DebateTeam)
```

### Manual Provider Verification

```go
// Verify specific provider
health := verifier.NewHealthChecker(config)
status, err := health.Check(ctx, provider)

// Score provider
scorer := verifier.NewScorer(config)
score := scorer.Score(ctx, provider, status)
```

## Testing

```bash
go test -v ./internal/verifier/...
```

Tests cover:
- Discovery of all provider types
- Verification pipeline execution
- Scoring algorithm accuracy
- Health check scenarios
- Database operations
- Adapter functionality
