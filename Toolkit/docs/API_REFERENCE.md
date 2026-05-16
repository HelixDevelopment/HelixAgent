# Toolkit - API Reference

**Module:** `github.com/HelixDevelopment/helix_agent/Toolkit`

## Core Interfaces (`pkg/toolkit`)

### Provider

Every AI provider must implement this interface:

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
    Rerank(ctx context.Context, req RerankRequest) (RerankResponse, error)
    DiscoverModels(ctx context.Context) ([]ModelInfo, error)
    ValidateConfig(config map[string]interface{}) error
}
```

### Agent

```go
type Agent interface {
    Name() string
    Execute(ctx context.Context, task string, config interface{}) (string, error)
    ValidateConfig(config interface{}) error
    Capabilities() []string
}
```

### ProviderFactory

```go
type ProviderFactory func(config map[string]interface{}) (Provider, error)
```

## Toolkit Struct

```go
func NewToolkit() *Toolkit
func (t *Toolkit) RegisterProvider(name string, provider Provider)
func (t *Toolkit) GetProvider(name string) (Provider, error)
func (t *Toolkit) ListProviders() []string
func (t *Toolkit) RegisterAgent(name string, agent Agent)
func (t *Toolkit) GetAgent(name string) (Agent, error)
func (t *Toolkit) ListAgents() []string
```

## Global Factory Registry

```go
func RegisterProviderFactory(name string, factory ProviderFactory)
func CreateProvider(name string, config map[string]interface{}) (Provider, error)
func ListRegisteredProviders() []string
```

## Request/Response Types

### ChatRequest / ChatResponse

```go
type ChatRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
    Temperature float64   `json:"temperature,omitempty"`
    TopP        float64   `json:"top_p,omitempty"`
    Stop        []string  `json:"stop,omitempty"`
}

type ChatResponse struct {
    ID      string   `json:"id"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
    Model   string   `json:"model"`
}
```

### EmbeddingRequest / EmbeddingResponse

```go
type EmbeddingRequest struct {
    Model  string   `json:"model"`
    Input  []string `json:"input"`
    Format string   `json:"encoding_format,omitempty"`
}

type EmbeddingResponse struct {
    Data  []Embedding `json:"data"`
    Usage Usage       `json:"usage"`
    Model string      `json:"model"`
}
```

### RerankRequest / RerankResponse

```go
type RerankRequest struct {
    Model     string   `json:"model"`
    Query     string   `json:"query"`
    Documents []string `json:"documents"`
    TopN      int      `json:"top_n,omitempty"`
}

type RerankResponse struct {
    Results []RerankResult `json:"results"`
    Usage   Usage          `json:"usage"`
}
```

## Supporting Types

### Message / Choice / Usage

```go
type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type Choice struct {
    Index        int     `json:"index"`
    Message      Message `json:"message"`
    FinishReason string  `json:"finish_reason"`
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

### ModelInfo / ModelCapabilities

```go
type ModelInfo struct {
    ID           string            `json:"id"`
    OwnedBy      string            `json:"owned_by"`
    Capabilities ModelCapabilities  `json:"capabilities"`
    Category     ModelCategory      `json:"category"`
    ContextWindow int              `json:"context_window"`
}

type ModelCapabilities struct {
    Chat            bool `json:"chat"`
    Embedding       bool `json:"embedding"`
    Rerank          bool `json:"rerank"`
    Vision          bool `json:"vision"`
    Audio           bool `json:"audio"`
    Video           bool `json:"video"`
    FunctionCalling bool `json:"function_calling"`
}
```

## Rate Limiting (`pkg/toolkit/common/ratelimit`)

### TokenBucket

```go
func NewTokenBucket(config TokenBucketConfig) *TokenBucket
func (tb *TokenBucket) Allow() bool
func (tb *TokenBucket) Wait(ctx context.Context) error

type TokenBucketConfig struct {
    Capacity   int
    RefillRate float64
}
```

### SlidingWindowLimiter

```go
func NewSlidingWindowLimiter(window time.Duration, limit int) *SlidingWindowLimiter
func (s *SlidingWindowLimiter) Allow() bool
```

### PerKeyLimiter

```go
func NewPerKeyLimiter(config TokenBucketConfig) *PerKeyLimiter
func (p *PerKeyLimiter) Allow(key string) bool
```

### CircuitBreaker

```go
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker
func (cb *CircuitBreaker) Allow() bool
func (cb *CircuitBreaker) RecordSuccess()
func (cb *CircuitBreaker) RecordFailure()

type CircuitBreakerConfig struct {
    FailureThreshold int
    SuccessThreshold int
    Timeout          time.Duration
}
```

## Authentication (`Commons/auth`)

```go
func NewAPIKeyAuth(apiKey string) *AuthManager
func NewOAuth2Refresher(clientID, clientSecret, tokenURL string, opts ...Option) *OAuth2Refresher
func NewAuthManager(apiKey string, refresher TokenRefresher) *AuthManager
func (m *AuthManager) GetAuthHeader(ctx context.Context) (string, error)
func NewMiddleware(manager *AuthManager) *Middleware
func (m *Middleware) WrapClient(client *http.Client) *http.Client
```

## Configuration (`Commons/config`)

```go
type Config map[string]interface{}
func (c Config) GetString(key string) (string, bool)
func (c Config) GetIntWithDefault(key string, def int) int
func (c Config) GetBoolWithDefault(key string, def bool) bool
func (c Config) GetFloatWithDefault(key string, def float64) float64
func (c Config) LoadFromEnv(prefix string)

func NewValidator() *Validator
func Required(field string) ValidateFunc
func MinLength(field string, min int) ValidateFunc
func OneOf(field string, values ...string) ValidateFunc
```

## Errors (`Commons/errors`)

| Error Type | Description |
|------------|-------------|
| `ProviderError` | Provider-specific with code and status |
| `APIError` | Parsed API error response |
| `RateLimitError` | Rate limit with retry-after |
| `AuthenticationError` | Authentication failures |
| `NetworkError` | Network connectivity errors |
| `TimeoutError` | Operation timeouts |
| `ValidationError` | Field validation failures |

```go
func NewErrorHandler(provider string) *ErrorHandler
func IsRetryable(err error) bool
func IsRateLimit(err error) bool
func IsAuth(err error) bool
func GetRetryAfter(err error) int
```

## Response Parsing (`Commons/response`)

```go
func (*JSONParser) ParseJSON(resp *http.Response, v interface{}) error
func NewStreamingParser(onData func([]byte) error, onError func(error)) *StreamingParser
func (*StreamingParser) ParseStream(resp *http.Response) error
func NewPaginationParser(hasNext func(map[string]interface{}) bool,
    nextURL func(map[string]interface{}) string) *PaginationParser
```
