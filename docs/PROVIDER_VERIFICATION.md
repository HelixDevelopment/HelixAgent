# Provider Verification System

## Overview

The Provider Verification System ensures that all LLM providers in the debate group are properly configured and functional before being used in the ensemble. This system provides:

1. **Real-time API Testing**: Verifies providers with actual API calls
2. **Health Status Tracking**: Maintains provider health status over time
3. **Error Categorization**: Distinguishes between rate limiting, auth failures, and other errors
4. **Circuit Breaker Integration**: Works with circuit breakers to prevent cascading failures

## Architecture Flow

```
API Keys (.env) → Provider Registration → Verification → Debate AI Group → Ensemble Response
```

## Provider Health Status

Providers can have the following health statuses:

| Status | Description |
|--------|-------------|
| `healthy` | Provider is verified and working |
| `rate_limited` | Provider quota exceeded (temporary) |
| `auth_failed` | Invalid or expired API key |
| `unhealthy` | Other errors (network, API issues) |
| `unknown` | Not yet verified |

## API Endpoints

### Verify All Providers
```bash
POST /v1/providers/verify
```

Triggers verification of all registered providers with actual API calls.

**Response:**
```json
{
  "providers": [
    {
      "provider": "deepseek",
      "status": "healthy",
      "verified": true,
      "response_time_ms": 2000,
      "tested_at": "2026-01-06T12:00:00Z"
    },
    {
      "provider": "gemini",
      "status": "rate_limited",
      "verified": false,
      "error": "rate limited or quota exceeded",
      "response_time_ms": 800,
      "tested_at": "2026-01-06T12:00:00Z"
    }
  ],
  "summary": {
    "total": 3,
    "healthy": 1,
    "rate_limited": 1,
    "auth_failed": 0,
    "unhealthy": 1
  },
  "ensemble_operational": true,
  "tested_at": "2026-01-06T12:00:00Z"
}
```

### Verify Single Provider
```bash
POST /v1/providers/{provider_name}/verify
```

**Response:**
```json
{
  "provider": "deepseek",
  "status": "healthy",
  "verified": true,
  "response_time_ms": 2000,
  "tested_at": "2026-01-06T12:00:00Z"
}
```

### Get All Verification Results
```bash
GET /v1/providers/verification
```

Returns cached verification results for all providers.

### Get Single Provider Verification
```bash
GET /v1/providers/{provider_name}/verification
```

Returns cached verification result for a specific provider.

## Go API Usage

### ProviderRegistry Methods

```go
// Verify a single provider
result := registry.VerifyProvider(ctx, "deepseek")
if result.Verified {
    // Provider is working
}

// Verify all providers
results := registry.VerifyAllProviders(ctx)
for _, r := range results {
    log.Printf("Provider %s: %s", r.Provider, r.Status)
}

// Check if provider is healthy
if registry.IsProviderHealthy("deepseek") {
    // Safe to use
}

// Get list of healthy providers
healthyProviders := registry.GetHealthyProviders()
```

## Test Suite

### Running Tests

```bash
# Run all debate group tests
go test -v ./tests/challenge -run TestDebateGroup -timeout 300s

# Run comprehensive tests
go test -v ./tests/challenge -run TestDebateGroupComprehensive -timeout 300s

# Run verification script
./challenges/scripts/verify_debate_group.sh --verbose
```

### Test Coverage

The test suite covers:

1. **Debate Group Sizes**
   - Single provider (fallback mode)
   - Dual provider (minimum viable)
   - Triple provider (standard)
   - All available providers

2. **Fallback Scenarios**
   - With fallback enabled
   - Without fallback (strict mode)
   - Various minimum provider counts

3. **Voting Strategies**
   - confidence_weighted
   - majority_vote
   - quality_weighted
   - best_of_n

4. **Provider Combinations**
   - All individual providers
   - All pairwise combinations
   - Full ensemble

5. **Concurrent Operations**
   - Multiple parallel debates
   - Throughput under load

6. **Edge Cases**
   - Empty messages
   - Long messages
   - Invalid strategies
   - High minimum providers

## Circuit Breaker Behavior

The system uses circuit breakers to protect against cascading failures:

1. **Closed State**: Normal operation, requests pass through
2. **Open State**: After repeated failures, requests are blocked
3. **Half-Open State**: After timeout, limited requests are allowed

When a circuit breaker is open, the provider status will show as `unhealthy` with the error "circuit breaker is open".

## Troubleshooting

### All Providers Showing as Unhealthy

1. Check if circuit breakers are open (too many recent failures)
2. Restart the server to reset circuit breakers
3. Verify API keys in `.env` file
4. Check provider rate limits

### Rate Limited Providers

- Wait for quota to reset (usually daily)
- Consider upgrading API plan
- Provider will auto-recover when quota resets

### Auth Failed Providers

- Verify API key is correct in `.env`
- Check if API key is active and has proper permissions
- Regenerate API key if necessary

## Configuration

### Environment Variables

```bash
# Provider API Keys
DEEPSEEK_API_KEY=sk-xxx
GEMINI_API_KEY=xxx
OPENROUTER_API_KEY=sk-xxx

# HelixAgent Server
HELIXAGENT_URL=http://localhost:7061
```

### Circuit Breaker Settings

Configure in `configs/production.yaml`:

```yaml
circuit_breaker:
  failure_threshold: 5
  recovery_timeout: 30s
  half_open_max_requests: 3
```

## Best Practices

1. **Run Verification on Startup**: Call `VerifyAllProviders()` during application startup
2. **Periodic Health Checks**: Implement background verification every 5-10 minutes
3. **Monitor Ensemble Status**: Alert when `ensemble_operational` becomes false
4. **Handle Degraded Mode**: Design your application to work with fewer providers
5. **Log Provider Errors**: Track rate limiting patterns to optimize API usage
