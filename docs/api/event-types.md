# Event Types Reference

Complete catalog of events published to Kafka topics by HelixAgent.

## Event Structure

All events follow this base structure:

```json
{
    "id": "evt_abc123",
    "type": "event.type.name",
    "source": "helixagent",
    "subject": "optional-subject-id",
    "timestamp": "2024-01-15T10:30:00Z",
    "trace_id": "trace_xyz789",
    "data": { ... },
    "metadata": { ... }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique event identifier |
| `type` | string | Yes | Event type (dot-notation) |
| `source` | string | Yes | Service that published event |
| `subject` | string | No | Related entity ID |
| `timestamp` | ISO8601 | Yes | Event creation time |
| `trace_id` | string | No | Distributed tracing ID |
| `data` | object | Yes | Event-specific payload |
| `metadata` | object | No | Additional context |

## LLM Events

### Topic: `helixagent.events.llm.responses`

#### `llm.request.started`

Published when an LLM request begins.

```json
{
    "type": "llm.request.started",
    "subject": "req_123",
    "data": {
        "request_id": "req_123",
        "provider": "claude",
        "model": "claude-3-opus",
        "prompt_tokens": 150,
        "max_tokens": 1000,
        "temperature": 0.7
    }
}
```

#### `llm.request.completed`

Published when an LLM request completes successfully.

```json
{
    "type": "llm.request.completed",
    "subject": "req_123",
    "data": {
        "request_id": "req_123",
        "provider": "claude",
        "model": "claude-3-opus",
        "prompt_tokens": 150,
        "completion_tokens": 500,
        "total_tokens": 650,
        "latency_ms": 2500,
        "finish_reason": "stop"
    }
}
```

#### `llm.request.failed`

Published when an LLM request fails.

```json
{
    "type": "llm.request.failed",
    "subject": "req_123",
    "data": {
        "request_id": "req_123",
        "provider": "claude",
        "model": "claude-3-opus",
        "error_code": "RATE_LIMITED",
        "error_message": "Rate limit exceeded",
        "retry_after": 60
    }
}
```

#### `llm.request.streamed`

Published for each streaming chunk (high volume).

```json
{
    "type": "llm.request.streamed",
    "subject": "req_123",
    "data": {
        "request_id": "req_123",
        "chunk_index": 5,
        "content": "Hello, how can I help",
        "finish_reason": null
    }
}
```

## Debate Events

### Topic: `helixagent.events.debate.rounds`

#### `debate.created`

Published when a new debate is created.

```json
{
    "type": "debate.created",
    "subject": "dbt_456",
    "data": {
        "debate_id": "dbt_456",
        "topic": "Should AI have rights?",
        "participants": [
            {"provider": "claude", "model": "claude-3-opus", "position": "proponent"},
            {"provider": "gpt-4", "model": "gpt-4-turbo", "position": "opponent"},
            {"provider": "gemini", "model": "gemini-pro", "position": "moderator"}
        ],
        "max_rounds": 5,
        "enable_multi_pass": true
    }
}
```

#### `debate.round.started`

Published when a debate round begins.

```json
{
    "type": "debate.round.started",
    "subject": "dbt_456",
    "data": {
        "debate_id": "dbt_456",
        "round_number": 1,
        "participant": {
            "provider": "claude",
            "model": "claude-3-opus",
            "position": "proponent"
        }
    }
}
```

#### `debate.round.completed`

Published when a debate round completes.

```json
{
    "type": "debate.round.completed",
    "subject": "dbt_456",
    "data": {
        "debate_id": "dbt_456",
        "round_number": 1,
        "participant": {
            "provider": "claude",
            "model": "claude-3-opus",
            "position": "proponent"
        },
        "content": "I believe AI systems should...",
        "confidence": 0.85,
        "tokens_used": 450,
        "latency_ms": 3200
    }
}
```

#### `debate.validation.started`

Published when multi-pass validation begins.

```json
{
    "type": "debate.validation.started",
    "subject": "dbt_456",
    "data": {
        "debate_id": "dbt_456",
        "phase": "VALIDATION",
        "rounds_to_validate": 3
    }
}
```

#### `debate.validation.completed`

Published when validation phase completes.

```json
{
    "type": "debate.validation.completed",
    "subject": "dbt_456",
    "data": {
        "debate_id": "dbt_456",
        "phase": "VALIDATION",
        "issues_found": 2,
        "improvements_suggested": 3
    }
}
```

#### `debate.completed`

Published when an entire debate concludes.

```json
{
    "type": "debate.completed",
    "subject": "dbt_456",
    "data": {
        "debate_id": "dbt_456",
        "status": "completed",
        "total_rounds": 5,
        "consensus": "Partial agreement on AI rights framework",
        "confidence": 0.78,
        "winner": null,
        "multi_pass_result": {
            "phases_completed": 4,
            "quality_improvement": 0.15
        },
        "total_tokens": 12500,
        "duration_ms": 45000
    }
}
```

## Verification Events

### Topic: `helixagent.events.verification.results`

#### `verification.started`

Published when provider verification begins.

```json
{
    "type": "verification.started",
    "subject": "ver_789",
    "data": {
        "verification_id": "ver_789",
        "provider": "deepseek",
        "model": "deepseek-chat",
        "test_count": 8
    }
}
```

#### `verification.test.completed`

Published for each verification test result.

```json
{
    "type": "verification.test.completed",
    "subject": "ver_789",
    "data": {
        "verification_id": "ver_789",
        "test_name": "basic_completion",
        "passed": true,
        "latency_ms": 1200,
        "error": null
    }
}
```

#### `verification.completed`

Published when all verification tests complete.

```json
{
    "type": "verification.completed",
    "subject": "ver_789",
    "data": {
        "verification_id": "ver_789",
        "provider": "deepseek",
        "model": "deepseek-chat",
        "tests_passed": 7,
        "tests_failed": 1,
        "overall_score": 8.2,
        "components": {
            "response_speed": 8.5,
            "model_efficiency": 7.8,
            "cost_effectiveness": 9.0,
            "capability": 8.0,
            "recency": 7.5
        }
    }
}
```

#### `provider.scored`

Published when a provider score is calculated.

```json
{
    "type": "provider.scored",
    "subject": "provider_deepseek",
    "data": {
        "provider": "deepseek",
        "score": 8.2,
        "rank": 3,
        "score_breakdown": {
            "response_speed": {"value": 8.5, "weight": 0.25},
            "model_efficiency": {"value": 7.8, "weight": 0.20},
            "cost_effectiveness": {"value": 9.0, "weight": 0.25},
            "capability": {"value": 8.0, "weight": 0.20},
            "recency": {"value": 7.5, "weight": 0.10}
        }
    }
}
```

## Provider Health Events

### Topic: `helixagent.events.provider.health`

#### `provider.health.check`

Published periodically for each provider.

```json
{
    "type": "provider.health.check",
    "subject": "provider_claude",
    "data": {
        "provider": "claude",
        "status": "healthy",
        "latency_ms": 250,
        "error_rate": 0.001,
        "last_success": "2024-01-15T10:29:00Z"
    }
}
```

#### `provider.status.changed`

Published when provider status changes.

```json
{
    "type": "provider.status.changed",
    "subject": "provider_claude",
    "data": {
        "provider": "claude",
        "previous_status": "healthy",
        "current_status": "degraded",
        "reason": "elevated_error_rate",
        "error_rate": 0.05
    }
}
```

#### `provider.discovered`

Published when a new provider is discovered.

```json
{
    "type": "provider.discovered",
    "subject": "provider_newllm",
    "data": {
        "provider": "newllm",
        "type": "api_key",
        "models": ["newllm-base", "newllm-pro"],
        "capabilities": {
            "streaming": true,
            "tools": false,
            "vision": false
        }
    }
}
```

## Audit Events

### Topic: `helixagent.events.audit`

#### `audit.api.request`

Published for each API request (compliance logging).

```json
{
    "type": "audit.api.request",
    "data": {
        "request_id": "req_abc",
        "method": "POST",
        "path": "/v1/chat/completions",
        "user_id": "usr_123",
        "ip_address": "192.168.1.1",
        "user_agent": "helix_agent/1.0",
        "status_code": 200,
        "latency_ms": 2500
    }
}
```

#### `audit.auth.event`

Published for authentication events.

```json
{
    "type": "audit.auth.event",
    "data": {
        "event": "login_success",
        "user_id": "usr_123",
        "method": "api_key",
        "ip_address": "192.168.1.1",
        "user_agent": "helix_agent/1.0"
    }
}
```

#### `audit.admin.action`

Published for administrative actions.

```json
{
    "type": "audit.admin.action",
    "data": {
        "action": "provider_disabled",
        "admin_id": "admin_456",
        "target": "provider_ollama",
        "reason": "deprecated",
        "ip_address": "192.168.1.100"
    }
}
```

## Error Events

### Topic: `helixagent.events.errors`

#### `error.unhandled`

Published for unhandled errors.

```json
{
    "type": "error.unhandled",
    "data": {
        "error_id": "err_xyz",
        "service": "debate-service",
        "error_type": "panic",
        "message": "runtime error: index out of range",
        "stack_trace": "...",
        "context": {
            "debate_id": "dbt_456",
            "round": 3
        }
    }
}
```

#### `error.rate_limit`

Published when rate limiting triggers.

```json
{
    "type": "error.rate_limit",
    "data": {
        "user_id": "usr_123",
        "endpoint": "/v1/chat/completions",
        "limit": 100,
        "window": "1m",
        "retry_after": 30
    }
}
```

## Metrics Events

### Topic: `helixagent.events.metrics`

#### `metrics.system`

Published periodically with system metrics.

```json
{
    "type": "metrics.system",
    "data": {
        "cpu_percent": 45.2,
        "memory_percent": 62.1,
        "goroutines": 150,
        "open_connections": 25,
        "active_requests": 10
    }
}
```

#### `metrics.provider`

Published periodically with provider metrics.

```json
{
    "type": "metrics.provider",
    "subject": "provider_claude",
    "data": {
        "provider": "claude",
        "requests_total": 15000,
        "requests_success": 14950,
        "requests_failed": 50,
        "latency_p50": 1200,
        "latency_p95": 2500,
        "latency_p99": 4000,
        "tokens_used": 2500000
    }
}
```

## Stream Events

### Topic: `helixagent.stream.tokens`

High-throughput token streaming:

```json
{
    "type": "stream.token",
    "subject": "req_123",
    "data": {
        "request_id": "req_123",
        "index": 42,
        "token": " the",
        "cumulative_text": "Hello, I am the"
    }
}
```

### Topic: `helixagent.stream.sse`

Server-Sent Events relay:

```json
{
    "type": "stream.sse",
    "subject": "client_abc",
    "data": {
        "client_id": "client_abc",
        "event": "message",
        "data": "{\"content\": \"Hello\"}",
        "id": "msg_123"
    }
}
```

## Consuming Events

### Go Client

```go
import "dev.helix.agent/internal/messaging/kafka"

broker := kafka.NewBroker(kafka.DefaultConfig())
broker.Connect(ctx)

// Subscribe to debate events
broker.Subscribe(ctx, "helixagent.events.debate.rounds", func(ctx context.Context, msg *messaging.Message) error {
    var event Event
    json.Unmarshal(msg.Payload, &event)

    switch event.Type {
    case "debate.completed":
        // Handle debate completion
    case "debate.round.completed":
        // Handle round completion
    }

    return nil
})
```

### Kafka Console Consumer

```bash
docker exec helixagent-kafka kafka-console-consumer \
    --bootstrap-server localhost:9092 \
    --topic helixagent.events.debate.rounds \
    --from-beginning \
    --property print.key=true
```
