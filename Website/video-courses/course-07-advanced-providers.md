# Video Course 07: Advanced Provider Configuration

## Course Overview

**Duration:** 4 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 02 (AI Debate)

Master advanced LLM provider configuration, including multi-provider orchestration, OAuth authentication, scoring algorithms, and dynamic provider selection.

---

## Module 1: Provider Architecture Deep Dive

### Video 1.1: Provider Registry Internals (20 min)

**Topics:**
- Provider interface design
- Registration lifecycle
- Capability discovery
- Health monitoring internals

**Architecture:**
```
┌─────────────────────────────────────────────────────────────┐
│                    Provider Registry                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │ Claude  │  │DeepSeek │  │ Gemini  │  │ Mistral │  ...   │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘        │
│       │            │            │            │              │
│  ┌────▼────────────▼────────────▼────────────▼────┐        │
│  │            Unified Provider Interface           │        │
│  │  - Complete(ctx, request) (response, error)    │        │
│  │  - CompleteStream(ctx, request) (stream, err)  │        │
│  │  - HealthCheck(ctx) error                      │        │
│  │  - GetCapabilities() Capabilities              │        │
│  └────────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

**Code Example:**
```go
type LLMProvider interface {
    Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
    CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.StreamChunk, error)
    HealthCheck(ctx context.Context) error
    GetCapabilities() *models.ProviderCapabilities
    ValidateConfig() error
}
```

### Video 1.2: Provider Types and Authentication (25 min)

**Topics:**
- API Key providers (DeepSeek, Gemini, Mistral, OpenRouter, ZAI, Cerebras)
- OAuth providers (Claude, Qwen)
- Free providers (Zen, OpenRouter :free models)
- Authentication flow differences

**Provider Types Table:**
| Type | Providers | Auth Method | Score Range |
|------|-----------|-------------|-------------|
| API Key | DeepSeek, Gemini, Mistral, OpenRouter, ZAI, Cerebras | Bearer Token | 5.0-10.0 |
| OAuth | Claude, Qwen | OAuth2 Tokens | 5.0-10.0 (+0.5 bonus) |
| Free | Zen, OpenRouter :free | Anonymous/X-Device-ID | 6.0-7.0 (capped) |

**Configuration:**
```bash
# API Key providers
export DEEPSEEK_API_KEY=sk-your-key
export GEMINI_API_KEY=AIza-your-key
export MISTRAL_API_KEY=your-key

# OAuth providers (from CLI tools)
export CLAUDE_CODE_USE_OAUTH_CREDENTIALS=true
export QWEN_CODE_USE_OAUTH_CREDENTIALS=true

# Free providers
export ZEN_ENABLED=true
```

### Video 1.3: OAuth Credential Management (30 min)

**Topics:**
- OAuth token sources
- Credential file locations
- Token refresh handling
- Limitations and workarounds

**OAuth Credential Locations:**
```
Claude: ~/.claude/.credentials.json
Qwen:   ~/.qwen/oauth_creds.json
```

**Important Limitation:**
OAuth tokens from CLI tools are product-restricted:
- Claude OAuth tokens only work with Claude Code
- Qwen OAuth tokens only work with Qwen Portal

**Solution for API Access:**
```bash
# Get standard API keys for general API access:
# Claude: https://console.anthropic.com/
# Qwen: https://dashscope.aliyuncs.com/
```

**Code Example:**
```go
// internal/auth/oauth_credentials/claude.go
func LoadClaudeCredentials() (*ClaudeCredentials, error) {
    homeDir, _ := os.UserHomeDir()
    credPath := filepath.Join(homeDir, ".claude", ".credentials.json")

    data, err := os.ReadFile(credPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read credentials: %w", err)
    }

    var creds ClaudeCredentials
    if err := json.Unmarshal(data, &creds); err != nil {
        return nil, fmt.Errorf("failed to parse credentials: %w", err)
    }

    return &creds, nil
}
```

---

## Module 2: LLMsVerifier Scoring System

### Video 2.1: Verification Pipeline (25 min)

**Topics:**
- 8-test verification suite
- Test categories (Required vs Optional)
- Parallel verification execution
- Score computation

**8-Test Verification Suite:**
| Test | Description | Category |
|------|-------------|----------|
| 1. Health Check | Basic API connectivity | Required |
| 2. Model List | Can list available models | Required |
| 3. Simple Completion | Basic chat completion | Required |
| 4. Streaming | SSE streaming support | Optional |
| 5. Tool Calling | Function/tool support | Optional |
| 6. Context Length | Token limit validation | Optional |
| 7. Response Time | Latency measurement | Scoring |
| 8. Error Handling | Graceful error responses | Scoring |

**Verification Status:**
- **Fully Verified** (8/8 tests): Score 8.0-10.0
- **Partially Verified** (Required tests): Score 6.0-8.0
- **Minimally Verified** (Health check only): Score 5.0-6.0
- **Failed**: Excluded from selection

### Video 2.2: 5-Component Scoring Algorithm (30 min)

**Topics:**
- Score component breakdown
- Weight configuration
- Bonus calculations
- Score normalization

**Scoring Components:**
| Component | Weight | Description |
|-----------|--------|-------------|
| Response Speed | 25% | API response latency (lower is better) |
| Model Efficiency | 20% | Token efficiency in responses |
| Cost Effectiveness | 25% | Cost per token |
| Capability | 20% | Model capability score |
| Recency | 10% | Model release date |

**Calculation Code:**
```go
func CalculateScore(provider *Provider) float64 {
    baseScore := (
        provider.ResponseSpeed     * 0.25 +
        provider.ModelEfficiency   * 0.20 +
        provider.CostEffectiveness * 0.25 +
        provider.Capability        * 0.20 +
        provider.Recency           * 0.10
    )

    // OAuth bonus
    if provider.AuthType == "oauth" && provider.IsVerified {
        baseScore += 0.5
    }

    // Free provider cap
    if provider.IsFree {
        if baseScore > 7.0 {
            baseScore = 7.0
        }
    }

    return baseScore
}
```

### Video 2.3: Customizing Scoring Weights (20 min)

**Topics:**
- Environment variable configuration
- YAML configuration
- Use case optimization
- Score monitoring

**Environment Configuration:**
```bash
export SCORE_WEIGHT_SPEED=0.25
export SCORE_WEIGHT_EFFICIENCY=0.20
export SCORE_WEIGHT_COST=0.25
export SCORE_WEIGHT_CAPABILITY=0.20
export SCORE_WEIGHT_RECENCY=0.10
```

**YAML Configuration:**
```yaml
# configs/verifier.yaml
scoring:
  weights:
    response_speed: 0.25
    model_efficiency: 0.20
    cost_effectiveness: 0.25
    capability: 0.20
    recency: 0.10
  bonuses:
    oauth_verified: 0.5
  caps:
    free_provider_max: 7.0
```

---

## Module 3: Dynamic Provider Selection

### Video 3.1: Startup Verification Flow (25 min)

**Topics:**
- Startup pipeline sequence
- Provider discovery
- Parallel verification
- Team selection

**Startup Pipeline:**
```
1. Load Config & Environment
2. Initialize StartupVerifier (Scoring + Verification)
3. Discover ALL Providers (API Key + OAuth + Free)
4. Verify ALL Providers in Parallel (8-test pipeline)
5. Score ALL Verified Providers (5-component weighted)
6. Rank by Score (OAuth priority when scores close)
7. Select AI Debate Team (15 LLMs: 5 primary + 10 fallback)
8. Start Server with Verified Configuration
```

**Code Path:**
```go
// internal/verifier/startup.go
func (sv *StartupVerifier) VerifyAll(ctx context.Context) (*VerificationResult, error) {
    // 1. Discover providers
    providers := sv.discoverProviders()

    // 2. Parallel verification
    results := sv.verifyParallel(ctx, providers)

    // 3. Score providers
    scored := sv.scoreProviders(results)

    // 4. Rank and select
    ranked := sv.rankProviders(scored)

    return &VerificationResult{
        Providers: ranked,
        Team:      sv.selectDebateTeam(ranked),
    }, nil
}
```

### Video 3.2: AI Debate Team Selection (30 min)

**Topics:**
- Team composition (5 positions x 3 LLMs)
- Primary vs fallback selection
- OAuth priority rules
- Diversity requirements

**Team Structure:**
```
Position 1 (Lead Pro)      Position 2 (Lead Con)
┌──────────────────┐      ┌──────────────────┐
│ Primary: Claude  │      │ Primary: Qwen    │
│ Fallback: Deep.  │      │ Fallback: OpenR. │
│ Fallback: Gemini │      │ Fallback: Mistral│
└──────────────────┘      └──────────────────┘

Position 3 (Analyst)      Position 4 (Synthesizer)
┌──────────────────┐      ┌──────────────────┐
│ Primary: Deep.   │      │ Primary: Gemini  │
│ Fallback: Gemini │      │ Fallback: OpenR. │
│ Fallback: ZAI    │      │ Fallback: Cerebras│
└──────────────────┘      └──────────────────┘

Position 5 (Moderator)
┌──────────────────┐
│ Primary: OpenR.  │
│ Fallback: Mistral│
│ Fallback: Zen    │
└──────────────────┘

Total: 5 positions x 3 LLMs = 15 LLMs
```

### Video 3.3: Fallback Mechanisms (25 min)

**Topics:**
- Provider failure detection
- Automatic fallback chain
- Circuit breaker patterns
- Recovery procedures

**Fallback Code:**
```go
func (e *Ensemble) executeWithFallback(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
    providers := e.getOrderedProviders()

    var lastErr error
    for _, provider := range providers {
        response, err := provider.Complete(ctx, req)
        if err == nil && response.Content != "" {
            return response, nil
        }
        lastErr = err

        // Log fallback
        log.Printf("Provider %s failed, trying next: %v", provider.Name(), err)
    }

    return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}
```

---

## Module 4: Multi-Provider Orchestration

### Video 4.1: Ensemble Strategies (30 min)

**Topics:**
- Voting strategies
- Confidence weighting
- Parallel execution
- Result aggregation

**Strategies:**
| Strategy | Description | Use Case |
|----------|-------------|----------|
| HighestConfidence | Select response with highest confidence | Simple queries |
| MajorityVote | Select most common response | Factual questions |
| WeightedAverage | Weight by provider score | Complex analysis |
| Debate | Multi-round discussion | Deep understanding |

**Code Example:**
```go
type VotingStrategy interface {
    SelectResponse(responses []*models.LLMResponse) (*models.LLMResponse, error)
}

type HighestConfidenceStrategy struct{}

func (s *HighestConfidenceStrategy) SelectResponse(responses []*models.LLMResponse) (*models.LLMResponse, error) {
    var best *models.LLMResponse
    for _, r := range responses {
        if best == nil || r.Confidence > best.Confidence {
            best = r
        }
    }
    return best, nil
}
```

### Video 4.2: Load Balancing and Rate Limiting (25 min)

**Topics:**
- Per-provider rate limits
- Token bucket implementation
- Request distribution
- Quota management

**Rate Limit Configuration:**
```yaml
providers:
  deepseek:
    rate_limit:
      requests_per_minute: 60
      tokens_per_minute: 100000
  gemini:
    rate_limit:
      requests_per_minute: 30
      tokens_per_minute: 50000
```

### Video 4.3: Cost Optimization (20 min)

**Topics:**
- Cost tracking per provider
- Budget-aware routing
- Token estimation
- Cost reporting

**Cost Configuration:**
```yaml
providers:
  claude:
    cost:
      input_per_1k_tokens: 0.003
      output_per_1k_tokens: 0.015
  deepseek:
    cost:
      input_per_1k_tokens: 0.0001
      output_per_1k_tokens: 0.0002
```

---

## Module 5: Provider Health Monitoring

### Video 5.1: Health Check Implementation (25 min)

**Topics:**
- Health check endpoints
- Latency monitoring
- Error rate tracking
- Health status propagation

**Health Status Types:**
| Status | Description | Action |
|--------|-------------|--------|
| Healthy | All checks pass | Use normally |
| Degraded | High latency/errors | Use with caution |
| Unhealthy | Critical failures | Automatic fallback |
| Unknown | Not yet checked | Initial verification |

### Video 5.2: Real-Time Monitoring Dashboard (20 min)

**Topics:**
- Prometheus metrics
- Grafana dashboards
- Alert configuration
- Trend analysis

**Key Metrics:**
```
helixagent_provider_requests_total
helixagent_provider_request_duration_seconds
helixagent_provider_errors_total
helixagent_provider_health_status
helixagent_provider_score
```

### Video 5.3: Alerting and Incident Response (20 min)

**Topics:**
- Alert thresholds
- PagerDuty integration
- Runbook automation
- Post-incident review

---

## Hands-on Labs

### Lab 1: Multi-Provider Setup
Configure all 10 providers and observe verification scores.

### Lab 2: Custom Scoring
Modify scoring weights and observe team selection changes.

### Lab 3: Failover Testing
Simulate provider failures and verify fallback behavior.

### Lab 4: Cost Optimization
Configure budget constraints and monitor cost distribution.

---

## Resources

- [Provider Configuration Guide](/docs/guides/configuration-guide.md)
- [LLMsVerifier Documentation](/docs/guides/llms-verifier.md)
- [Startup Verification](/docs/architecture/startup-verification.md)
- [HelixAgent GitHub](https://dev.helix.agent)

---

## Course Completion

Congratulations! You've completed the Advanced Provider Configuration course. You should now be able to:

- Configure all provider types (API Key, OAuth, Free)
- Understand and customize the scoring algorithm
- Manage dynamic provider selection
- Implement multi-provider orchestration
- Monitor provider health effectively
