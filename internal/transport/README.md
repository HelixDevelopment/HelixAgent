# Transport Package

The transport package provides HTTP client and server transport layer functionality.

## Overview

This package handles:
- HTTP client configuration for provider APIs
- Request/response serialization
- Retry logic and circuit breakers
- Connection pooling

## Components

### HTTP Client

Configurable HTTP client for API calls:

```go
client := transport.NewHTTPClient(transport.ClientConfig{
    Timeout:             30 * time.Second,
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
})
```

### Retry Configuration

Built-in retry logic with exponential backoff:

```go
client := transport.NewHTTPClient(transport.ClientConfig{
    RetryCount:     3,
    RetryWaitTime:  100 * time.Millisecond,
    RetryMaxWait:   2 * time.Second,
    RetryCondition: transport.DefaultRetryCondition,
})
```

### Circuit Breaker

Prevents cascading failures:

```go
client := transport.NewHTTPClientWithCircuitBreaker(
    transport.CircuitBreakerConfig{
        Threshold:   5,
        Timeout:     30 * time.Second,
        HalfOpenMax: 3,
    },
)
```

## Usage

### Making Requests

```go
// GET request
resp, err := client.Get(ctx, url, headers)

// POST request with JSON body
resp, err := client.PostJSON(ctx, url, body, headers)

// Streaming request
stream, err := client.StreamRequest(ctx, url, body, headers)
```

### Response Handling

```go
resp, err := client.Get(ctx, url, nil)
if err != nil {
    return err
}
defer resp.Body.Close()

var result MyResponse
if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
    return err
}
```

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `Timeout` | Request timeout | 30s |
| `MaxIdleConns` | Max idle connections | 100 |
| `MaxIdleConnsPerHost` | Max idle per host | 10 |
| `IdleConnTimeout` | Idle connection timeout | 90s |
| `RetryCount` | Number of retries | 3 |
| `RetryWaitTime` | Initial retry wait | 100ms |

## Streaming Support

For SSE (Server-Sent Events) streaming:

```go
stream, err := client.StreamSSE(ctx, url, body, headers)
if err != nil {
    return err
}

for event := range stream.Events() {
    if event.Error != nil {
        return event.Error
    }
    fmt.Println(event.Data)
}
```

## Custom Transport

Create custom transport for specialized needs:

```go
transport := &http.Transport{
    TLSClientConfig: &tls.Config{
        InsecureSkipVerify: false,
    },
    Proxy: http.ProxyFromEnvironment,
}
client := transport.NewHTTPClientWithTransport(transport)
```
