# HelixAgent Debate API Documentation

> **Status: Planned Features**
>
> This document describes the AI Debate API endpoints that are being developed.
> The debate services exist internally (`internal/services/debate_*.go`) but are
> **not yet exposed as HTTP endpoints**.
>
> For currently available API endpoints, see [api-documentation.md](./api-documentation.md)
> which documents the OpenAI-compatible endpoints that are fully implemented.

## Overview

The HelixAgent Debate API will provide comprehensive endpoints for AI-powered debates with multi-provider support, Cognee AI enhancement, and advanced monitoring capabilities.

## Base URL

```
Production: https://api.helixagent.ai/v1
Development: http://localhost:7061/v1
```

## Authentication

All API requests require authentication using Bearer tokens:

```http
Authorization: Bearer YOUR_API_TOKEN
```

## Content Types

All requests and responses use `application/json` unless otherwise specified.

---

## Core Endpoints

### Debates

#### Create Debate

**POST** `/debates`

Creates a new AI debate with specified participants and configuration.

**Request Body:**
```json
{
  "debateId": "climate-debate-001",
  "topic": "Should governments implement carbon taxes?",
  "participants": [
    {
      "participantId": "ai-economist",
      "name": "AI Economist",
      "role": "proponent",
      "llmProvider": "claude",
      "llmModel": "claude-3-opus-20240229",
      "maxRounds": 3,
      "timeout": 120,
      "weight": 1.0
    },
    {
      "participantId": "ai-policy-expert",
      "name": "AI Policy Expert",
      "role": "opponent",
      "llmProvider": "deepseek",
      "llmModel": "deepseek-chat",
      "maxRounds": 3,
      "timeout": 120,
      "weight": 1.0
    }
  ],
  "maxRounds": 3,
  "timeout": 600,
  "strategy": "structured",
  "enableCognee": true,
  "metadata": {
    "category": "policy",
    "difficulty": "advanced"
  }
}
```

**Response (201 Created):**
```json
{
  "debateId": "climate-debate-001",
  "status": "created",
  "startTime": "2024-01-15T10:30:00Z",
  "estimatedEndTime": "2024-01-15T10:40:00Z",
  "message": "Debate created successfully"
}
```

**Error Response (400 Bad Request):**
```json
{
  "error": "INVALID_REQUEST",
  "message": "Invalid debate configuration",
  "details": {
    "field": "participants",
    "reason": "At least 2 participants required"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### Get Debate Details

**GET** `/debates/{debateId}`

Retrieves comprehensive information about a specific debate.

**Path Parameters:**
- `debateId` (string, required): Unique identifier for the debate

**Response (200 OK):**
```json
{
  "debateId": "climate-debate-001",
  "topic": "Should governments implement carbon taxes?",
  "status": "completed",
  "startTime": "2024-01-15T10:30:00Z",
  "endTime": "2024-01-15T10:38:00Z",
  "duration": 480,
  "totalRounds": 3,
  "currentRound": 3,
  "participants": [
    {
      "participantId": "ai-economist",
      "name": "AI Economist",
      "role": "proponent",
      "status": "completed",
      "currentResponse": "Based on economic analysis, carbon taxes are the most efficient...",
      "responseTime": 8500,
      "error": null
    }
  ],
  "consensus": {
    "achieved": true,
    "confidence": 0.78,
    "agreementLevel": 0.65,
    "finalPosition": "Carbon taxes should be implemented with careful consideration",
    "keyPoints": [
      "Economic efficiency of market-based solutions",
      "Need for comprehensive policy framework",
      "Importance of equitable implementation"
    ],
    "disagreements": [
      "Optimal tax rate determination",
      "Impact on low-income households"
    ],
    "timestamp": "2024-01-15T10:38:00Z"
  },
  "qualityScore": 0.85,
  "success": true,
  "errorMessage": null
}
```

#### Get Debate Status

**GET** `/debates/{debateId}/status`

Retrieves real-time status of an ongoing debate.

**Response (200 OK):**
```json
{
  "debateId": "climate-debate-001",
  "status": "running",
  "currentRound": 2,
  "totalRounds": 3,
  "startTime": "2024-01-15T10:30:00Z",
  "estimatedEndTime": "2024-01-15T10:40:00Z",
  "participants": [
    {
      "participantId": "ai-economist",
      "name": "AI Economist",
      "status": "responding",
      "currentResponse": "Building on the previous point...",
      "responseTime": 3200,
      "error": null
    }
  ]
}
```

#### Get Debate Results

**GET** `/debates/{debateId}/results`

Retrieves final results and consensus information.

**Response (200 OK):**
```json
{
  "debateId": "climate-debate-001",
  "startTime": "2024-01-15T10:30:00Z",
  "endTime": "2024-01-15T10:38:00Z",
  "duration": 480,
  "totalRounds": 3,
  "participants": [
    {
      "participantId": "ai-economist",
      "name": "AI Economist",
      "role": "proponent",
      "round": 3,
      "response": "In conclusion, carbon taxes represent the most economically efficient...",
      "confidence": 0.92,
      "qualityScore": 0.88,
      "responseTime": 8500,
      "llmProvider": "claude",
      "llmModel": "claude-3-opus-20240229",
      "timestamp": "2024-01-15T10:37:00Z"
    }
  ],
  "consensus": {
    "achieved": true,
    "confidence": 0.78,
    "agreementLevel": 0.65,
    "finalPosition": "Carbon taxes should be implemented with careful consideration",
    "keyPoints": [
      "Economic efficiency of market-based solutions",
      "Need for comprehensive policy framework",
      "Importance of equitable implementation"
    ],
    "disagreements": [
      "Optimal tax rate determination",
      "Impact on low-income households"
    ],
    "timestamp": "2024-01-15T10:38:00Z"
  },
  "qualityScore": 0.85,
  "success": true,
  "errorMessage": null
}
```

#### Stream Debate Progress

**GET** `/debates/{debateId}/stream`

Streams real-time debate progress and participant responses via Server-Sent Events (SSE).

**Response (200 OK, text/event-stream):**
```
event: round_started
data: {"round": 2, "timestamp": "2024-01-15T10:35:00Z"}

event: participant_response
data: {"participantId": "ai-economist", "response": "Based on economic analysis...", "timestamp": "2024-01-15T10:36:00Z"}

event: round_completed
data: {"round": 2, "timestamp": "2024-01-15T10:38:00Z"}
```

**Query Parameters:**
- `includeResponses` (boolean, optional): Include full response text in events (default: false)

#### Generate Debate Report

**GET** `/debates/{debateId}/report`

Creates a comprehensive report for the debate.

**Query Parameters:**
- `format` (string, optional): Report format - `json`, `pdf`, or `html` (default: `json`)

**Response (200 OK):**
```json
{
  "reportId": "report-climate-debate-001",
  "debateId": "climate-debate-001",
  "generatedAt": "2024-01-15T10:40:00Z",
  "summary": "The debate on carbon taxation resulted in consensus with 78% confidence...",
  "keyFindings": [
    "Both participants agreed on the economic efficiency of carbon taxes",
    "Implementation challenges were extensively discussed",
    "Consensus was reached on the need for comprehensive policy frameworks"
  ],
  "recommendations": [
    "Implement carbon taxes with careful consideration of social impacts",
    "Establish clear policy frameworks before implementation",
    "Consider phased implementation to assess impacts"
  ],
  "metrics": {
    "duration": 480,
    "totalRounds": 3,
    "qualityScore": 0.85,
    "throughput": 0.375,
    "latency": 8850,
    "errorRate": 0.0,
    "resourceUsage": {
      "cpu": 0.15,
      "memory": 104857600,
      "network": 5242880
    }
  }
}
```

### Providers

#### List Available Providers

**GET** `/providers`

Retrieves all configured LLM providers and their capabilities.

**Response (200 OK):**
```json
{
  "providers": [
    {
      "providerId": "claude-001",
      "name": "Anthropic Claude",
      "type": "claude",
      "status": "active",
      "capabilities": {
        "supportedModels": [
          "claude-3-opus-20240229",
          "claude-3-sonnet-20240229",
          "claude-3-haiku-20240307"
        ],
        "supportedFeatures": [
          "long_context",
          "vision",
          "tools",
          "function_calling"
        ],
        "supportedRequestTypes": [
           "completion",
           "streaming"
         ],
         "supportsStreaming": true,
        "supportsStreaming": true,
        "supportsFunctionCalling": true,
        "supportsVision": true,
        "supportsTools": true,
        "limits": {
          "maxTokens": 200000,
          "maxInputLength": 200000,
          "maxOutputLength": 4096,
          "maxConcurrentRequests": 50
        },
        "metadata": {
          "provider": "anthropic",
          "version": "2024-02-29"
        }
      },
      "responseTime": 8500,
      "lastHealthCheck": "2024-01-15T10:45:00Z"
    }
  ]
}
```

#### Get Provider Health Status

**GET** `/providers/{providerId}/health`

Retrieves health status of a specific LLM provider including circuit breaker state.

**Response (200 OK):**
```json
{
  "providerId": "claude-001",
  "status": "healthy",
  "lastCheck": "2024-01-15T10:45:00Z",
  "responseTime": 8200,
  "errorCount": 2,
  "successRate": 0.996,
  "circuitBreaker": {
    "state": "closed",
    "failureCount": 0,
    "lastFailure": null,
    "recoveryTimeout": "60s",
    "nextRetry": null
  }
}
```

**Circuit Breaker States:**
- `closed`: Normal operation, requests flow through
- `open`: Circuit breaker tripped, requests fail fast
- `half-open`: Testing recovery, limited requests allowed

### History

#### Get Debate History

**GET** `/history`

Retrieves historical debate data with filtering options.

**Query Parameters:**
- `startTime` (string, optional): Start time (ISO 8601)
- `endTime` (string, optional): End time (ISO 8601)
- `participantIds` (array, optional): Filter by participant IDs
- `minQualityScore` (number, optional): Minimum quality score (0-1)
- `maxQualityScore` (number, optional): Maximum quality score (0-1)
- `limit` (integer, optional): Maximum results (1-100, default: 50)
- `offset` (integer, optional): Offset for pagination (default: 0)

**Response (200 OK):**
```json
{
  "debates": [
    {
      "debateId": "climate-debate-001",
      "topic": "Should governments implement carbon taxes?",
      "startTime": "2024-01-15T10:30:00Z",
      "endTime": "2024-01-15T10:38:00Z",
      "duration": 480,
      "totalRounds": 3,
      "participants": ["AI Economist", "AI Policy Expert"],
      "consensus": true,
      "qualityScore": 0.85
    }
  ],
  "totalCount": 156,
  "limit": 50,
  "offset": 0
}
```

### Monitoring

#### Get System Metrics

**GET** `/metrics`

Retrieves performance and system metrics.

**Query Parameters:**
- `timeRange` (string, optional): Time range - `1h`, `6h`, `24h`, `7d`, `30d` (default: `24h`)

**Response (200 OK):**
```json
{
  "timeRange": "24h",
  "totalDebates": 47,
  "averageDuration": 420,
  "averageQualityScore": 0.82,
  "successRate": 0.957,
  "providerMetrics": [
    {
      "providerId": "claude-001",
      "totalRequests": 156,
      "successCount": 154,
      "errorCount": 2,
      "averageResponseTime": 8250,
      "successRate": 0.987
    }
  ]
}
```

---

## Error Handling

### Error Response Format

All error responses follow a consistent format:

```json
{
  "error": "ERROR_CODE",
  "message": "Human-readable error message",
  "details": {
    "field": "specific_field",
    "reason": "Detailed explanation"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Common Error Codes

| Error Code | Description | HTTP Status |
|------------|-------------|-------------|
| `INVALID_REQUEST` | Request validation failed | 400 |
| `UNAUTHORIZED` | Authentication required | 401 |
| `FORBIDDEN` | Insufficient permissions | 403 |
| `NOT_FOUND` | Resource not found | 404 |
| `RATE_LIMIT_EXCEEDED` | Too many requests | 429 |
| `PROVIDER_ERROR` | LLM provider error | 502 |
| `INTERNAL_ERROR` | Internal server error | 500 |

### Error Handling Examples

**Validation Error:**
```json
{
  "error": "INVALID_REQUEST",
  "message": "Invalid debate configuration",
  "details": {
    "field": "participants",
    "reason": "At least 2 participants required"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Provider Error:**
```json
{
  "error": "PROVIDER_ERROR",
  "message": "LLM provider unavailable",
  "details": {
    "provider": "claude",
    "reason": "Rate limit exceeded"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

---

## Rate Limiting

### Rate Limits

- **Authenticated requests**: 1000 requests per hour
- **Debate creation**: 50 debates per hour
- **Provider requests**: Provider-specific limits apply

### Rate Limit Headers

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 856
X-RateLimit-Reset: 1705323600
```

---

## Webhooks

### Webhook Events

Configure webhooks to receive real-time updates:

**Debate Events:**
- `debate.created` - New debate created
- `debate.started` - Debate started
- `debate.round_completed` - Round completed
- `debate.completed` - Debate completed
- `debate.failed` - Debate failed

**Provider Events:**
- `provider.health_changed` - Provider health status changed
- `provider.rate_limit_hit` - Provider rate limit exceeded

### Webhook Payload

```json
{
  "event": "debate.completed",
  "timestamp": "2024-01-15T10:38:00Z",
  "data": {
    "debateId": "climate-debate-001",
    "status": "completed",
    "qualityScore": 0.85,
    "consensus": {
      "achieved": true,
      "confidence": 0.78
    }
  }
}
```

---

## SDKs and Libraries

### Official SDKs

- **Go**: `dev.helix.agent-go`
- **Python**: `pip install helixagent-ai`
- **JavaScript**: `npm install helixagent-ai`
- **TypeScript**: `npm install @helixagent/ai`

### Example Usage

#### Python SDK
```python
from helixagent_ai import HelixAgentClient

client = HelixAgentClient(api_key="your-api-key")

# Create a debate
debate = client.debates.create({
    "debateId": "climate-debate-001",
    "topic": "Should governments implement carbon taxes?",
    "participants": [
        {
            "participantId": "ai-economist",
            "name": "AI Economist",
            "role": "proponent",
            "llmProvider": "claude",
            "llmModel": "claude-3-opus-20240229"
        }
    ],
    "maxRounds": 3,
    "timeout": 300,
    "enableCognee": True
})

# Monitor debate status
status = client.debates.get_status("climate-debate-001")
print(f"Current round: {status['currentRound']}")
```

#### JavaScript SDK
```javascript
import { HelixAgentClient } from '@helixagent/ai';

const client = new HelixAgentClient({
  apiKey: 'your-api-key',
  baseURL: 'https://api.helixagent.ai/v1'
});

// Create and monitor a debate
const debate = await client.debates.create({
  debateId: 'climate-debate-001',
  topic: 'Should governments implement carbon taxes?',
  participants: [
    {
      participantId: 'ai-economist',
      name: 'AI Economist',
      role: 'proponent',
      llmProvider: 'claude',
      llmModel: 'claude-3-opus-20240229'
    }
  ],
  maxRounds: 3,
  timeout: 300,
  enableCognee: true
});

// Poll for status updates
const status = await client.debates.getStatus('climate-debate-001');
console.log(`Current round: ${status.currentRound}`);
```

---

## Best Practices

### Request Optimization

1. **Batch Operations**: Use bulk endpoints when available
2. **Caching**: Cache provider capabilities and health status
3. **Timeouts**: Set appropriate timeouts for long-running operations
4. **Retry Logic**: Implement exponential backoff for transient errors

### Security

1. **API Key Storage**: Never expose API keys in client-side code
2. **HTTPS Only**: Always use HTTPS for production requests
3. **Rate Limiting**: Implement client-side rate limiting
4. **Input Validation**: Validate all inputs before sending to API

### Performance

1. **Connection Pooling**: Reuse HTTP connections
2. **Compression**: Enable gzip compression for large payloads
3. **Async Processing**: Use webhooks for long-running operations
4. **Caching**: Cache frequently accessed data

---

## Support

### Getting Help

- **Documentation**: https://docs.helixagent.ai
- **API Status**: https://status.helixagent.ai
- **Support Email**: support@helixagent.ai
- **Community**: https://community.helixagent.ai

### Status Codes

Monitor API status at: https://status.helixagent.ai

### Changelog

Stay updated with API changes: https://docs.helixagent.ai/changelog

---

*Last Updated: January 2026*
*API Version: 1.0.0*