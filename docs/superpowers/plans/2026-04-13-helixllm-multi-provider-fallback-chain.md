# HelixLLM Multi-Provider Fallback Chain Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace HelixLLM's local-only llama.cpp primary provider with a scored multi-provider fallback chain using free cloud models, keeping llama.cpp as last-resort fallback.

**Architecture:** New `internal/fallback/` package sits between Gateway and Brain. Gateway calls `FallbackChain.Complete()` which iterates scored providers (Chutes, OpenRouter, HuggingFace, Nvidia, Cerebras, SambaNova, Together) in LLMsVerifier-ranked order, with reactive 429 failover + proactive rate-limit header parsing. LlamaCpp is always the last entry. A `MemoryAdapter` syncs persistent memories to HelixAgent's HelixMemory.

**Tech Stack:** Go 1.25, `net/http`, `encoding/json`, `sync`, LLMsVerifier scoring packages, existing Brain `Provider` interface.

**Spec:** `docs/superpowers/specs/2026-04-13-helixllm-multi-provider-fallback-chain-design.md`

---

## File Structure

### New Files (all paths relative to `HelixLLM/`)

```
internal/brain/openai_compat_provider.go         — Shared base for all OpenAI-compatible providers
internal/brain/openai_compat_provider_test.go     — Tests for the shared base
internal/brain/chutes_provider.go                 — Chutes provider (embeds base)
internal/brain/chutes_provider_test.go
internal/brain/openrouter_provider.go             — OpenRouter provider (free model filter)
internal/brain/openrouter_provider_test.go
internal/brain/huggingface_provider.go            — HuggingFace provider
internal/brain/huggingface_provider_test.go
internal/brain/nvidia_provider.go                 — Nvidia NIM provider
internal/brain/nvidia_provider_test.go
internal/brain/cerebras_provider.go               — Cerebras provider
internal/brain/cerebras_provider_test.go
internal/brain/sambanova_provider.go              — SambaNova provider
internal/brain/sambanova_provider_test.go
internal/brain/together_provider.go               — Together provider
internal/brain/together_provider_test.go
internal/fallback/chain.go                        — FallbackChain orchestrator
internal/fallback/chain_test.go
internal/fallback/chain_entry.go                  — ChainEntry type + EntryStatus enum
internal/fallback/rate_limit.go                   — RateLimitTracker + RateLimitState + header parsing
internal/fallback/rate_limit_test.go
internal/fallback/circuit_breaker.go              — CircuitBreaker (open/close/half-open)
internal/fallback/circuit_breaker_test.go
internal/fallback/scorer_bridge.go                — LLMsVerifier hybrid integration
internal/fallback/scorer_bridge_test.go
internal/fallback/memory_adapter.go               — HelixMemory sync adapter
internal/fallback/memory_adapter_test.go
tests/integration/fallback_chain_integration_test.go
tests/integration/scorer_bridge_integration_test.go
tests/integration/memory_sync_integration_test.go
tests/e2e/multi_provider_e2e_test.go
tests/e2e/rate_limit_rotation_e2e_test.go
tests/security/fallback_security_test.go
tests/stress/fallback_stress_test.go
tests/benchmark/fallback_benchmark_test.go
challenges/scripts/multi_provider_fallback_challenge.sh
challenges/scripts/helixllm_memory_sync_challenge.sh
```

### Modified Files

```
internal/brain/brain.go:30-42       — Add 7 new key fields to Config struct
internal/brain/brain.go:44-94       — Register 7 new providers in New()
internal/shared/config/config.go:45-61  — Add 7 env var fields to LLMConfig
internal/gateway/openai.go:34       — Change HandleChatCompletions param from *brain.Brain to fallback.Completer interface
cmd/helixllm/main.go:251-316       — Initialize FallbackChain, wire between Brain and Gateway
.env.example                        — Add 7 provider key env vars + fallback config vars
```

---

### Task 1: OpenAICompatibleProvider Base

**Files:**
- Create: `internal/brain/openai_compat_provider.go`
- Create: `internal/brain/openai_compat_provider_test.go`

- [ ] **Step 1: Write the failing test for request building**

```go
// internal/brain/openai_compat_provider_test.go
package brain

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/pkg/types"
)

func TestOpenAICompatProvider_Complete_SendsCorrectRequest(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "chatcmpl-test",
			"model": "test-model",
			"choices": []map[string]interface{}{
				{
					"message":       map[string]string{"role": "assistant", "content": "hello"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	p := NewOpenAICompatProvider(OpenAICompatConfig{
		Name:       "test",
		BaseURL:    srv.URL,
		APIKey:     "test-key-123",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",
	})

	req := &types.InternalChatRequest{
		Model: "test-model",
		Messages: []types.InternalMessage{
			{Role: "user", Content: "hi"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := p.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-key-123", receivedAuth)
	assert.Equal(t, "test-model", receivedBody["model"])
	assert.Equal(t, "hello", resp.Message.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, "test", resp.Provider.Name)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestOpenAICompatProvider_Complete_SendsCorrectRequest ./internal/brain/ -count=1`
Expected: FAIL — `NewOpenAICompatProvider` undefined

- [ ] **Step 3: Write the OpenAICompatProvider implementation**

```go
// internal/brain/openai_compat_provider.go
package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"dev.helix.llm/pkg/types"
)

// OpenAICompatConfig configures a provider that speaks the OpenAI chat completions API.
type OpenAICompatConfig struct {
	Name       string // e.g., "chutes", "openrouter"
	BaseURL    string // e.g., "https://llm.chutes.ai/v1"
	APIKey     string
	AuthHeader string // e.g., "Authorization"
	AuthPrefix string // e.g., "Bearer "
	Timeout    time.Duration
}

// OpenAICompatProvider is a shared base for providers that implement the
// OpenAI-compatible /v1/chat/completions and /v1/models endpoints.
type OpenAICompatProvider struct {
	name       string
	baseURL    string
	apiKey     string
	authHeader string
	authPrefix string
	httpClient *http.Client

	mu            sync.RWMutex
	models        []string
	modelsFetched time.Time
}

func NewOpenAICompatProvider(cfg OpenAICompatConfig) *OpenAICompatProvider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &OpenAICompatProvider{
		name:       cfg.Name,
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		authHeader: cfg.AuthHeader,
		authPrefix: cfg.AuthPrefix,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// apiRequest mirrors the OpenAI ChatCompletion request format.
type apiRequest struct {
	Model       string             `json:"model"`
	Messages    []apiMessage       `json:"messages"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream"`
	Tools       []types.InternalTool `json:"tools,omitempty"`
	ToolChoice  interface{}        `json:"tool_choice,omitempty"`
}

type apiMessage struct {
	Role       string         `json:"role"`
	Content    interface{}    `json:"content,omitempty"`
	ToolCalls  []apiToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type apiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function apiToolFunction `json:"function"`
}

type apiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type apiResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []apiChoice  `json:"choices"`
	Usage   apiUsage     `json:"usage"`
}

type apiChoice struct {
	Message      apiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (p *OpenAICompatProvider) Complete(ctx context.Context, req *types.InternalChatRequest) (*types.InternalChatResponse, error) {
	apiReq := p.toAPIRequest(req)
	apiReq.Stream = false

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", p.name, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set(p.authHeader, p.authPrefix+p.apiKey)
	}

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: http request: %w", p.name, err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read response: %w", p.name, err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, &ProviderError{
			Provider:   p.name,
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
			Headers:    httpResp.Header,
		}
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", p.name, err)
	}

	return p.toInternalResponse(&apiResp), nil
}

// ProviderError carries HTTP status and headers so the fallback chain can
// distinguish rate limits (429) from server errors (5xx).
type ProviderError struct {
	Provider   string
	StatusCode int
	Body       string
	Headers    http.Header
}

func (e *ProviderError) Error() string {
	preview := e.Body
	if len(preview) > 200 {
		preview = preview[:200]
	}
	return fmt.Sprintf("%s: HTTP %d: %s", e.Provider, e.StatusCode, preview)
}

func (p *OpenAICompatProvider) CompleteStream(ctx context.Context, req *types.InternalChatRequest) (<-chan types.StreamChunk, error) {
	apiReq := p.toAPIRequest(req)
	apiReq.Stream = true

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", p.name, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set(p.authHeader, p.authPrefix+p.apiKey)
	}

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: http request: %w", p.name, err)
	}

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, &ProviderError{
			Provider:   p.name,
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
			Headers:    httpResp.Header,
		}
	}

	ch := make(chan types.StreamChunk, 32)
	go func() {
		defer close(ch)
		defer httpResp.Body.Close()
		p.readSSEStream(httpResp.Body, ch)
	}()
	return ch, nil
}

func (p *OpenAICompatProvider) Models() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.models
}

func (p *OpenAICompatProvider) Name() string {
	return p.name
}

func (p *OpenAICompatProvider) Available() bool {
	return p.apiKey != ""
}

// FetchModels queries /v1/models and updates the cached model list.
// filterFn optionally filters models (e.g., OpenRouter `:free` suffix).
func (p *OpenAICompatProvider) FetchModels(ctx context.Context, filterFn func(id string) bool) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("%s: create models request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set(p.authHeader, p.authPrefix+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s: fetch models: %w", p.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: models endpoint returned %d", p.name, resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("%s: decode models: %w", p.name, err)
	}

	var models []string
	for _, m := range result.Data {
		if filterFn == nil || filterFn(m.ID) {
			models = append(models, m.ID)
		}
	}

	p.mu.Lock()
	p.models = models
	p.modelsFetched = time.Now()
	p.mu.Unlock()
	return nil
}

// SetModels sets the model list directly (for providers with known models).
func (p *OpenAICompatProvider) SetModels(models []string) {
	p.mu.Lock()
	p.models = models
	p.modelsFetched = time.Now()
	p.mu.Unlock()
}

func (p *OpenAICompatProvider) toAPIRequest(req *types.InternalChatRequest) apiRequest {
	msgs := make([]apiMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := apiMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, apiToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: apiToolFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		msgs[i] = msg
	}
	return apiRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
	}
}

func (p *OpenAICompatProvider) toInternalResponse(resp *apiResponse) *types.InternalChatResponse {
	out := &types.InternalChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: types.InternalUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Provider: types.Provider{Name: p.name},
	}
	if len(resp.Choices) > 0 {
		c := resp.Choices[0]
		out.FinishReason = c.FinishReason
		out.Message = types.InternalMessage{
			Role: c.Message.Role,
		}
		if s, ok := c.Message.Content.(string); ok {
			out.Message.Content = s
		}
		for _, tc := range c.Message.ToolCalls {
			out.Message.ToolCalls = append(out.Message.ToolCalls, types.InternalToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: types.InternalToolFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}
	return out
}

func (p *OpenAICompatProvider) readSSEStream(r io.Reader, ch chan<- types.StreamChunk) {
	buf := make([]byte, 4096)
	var partial []byte
	for {
		n, err := r.Read(buf)
		if n > 0 {
			partial = append(partial, buf[:n]...)
			for {
				idx := bytes.IndexByte(partial, '\n')
				if idx < 0 {
					break
				}
				line := string(partial[:idx])
				partial = partial[idx+1:]

				if len(line) == 0 || line == "\r" {
					continue
				}
				const prefix = "data: "
				if len(line) < len(prefix) {
					continue
				}
				data := line[len(prefix):]
				if data == "[DONE]" {
					return
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
						FinishReason *string `json:"finish_reason"`
					} `json:"choices"`
				}
				if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
					sc := types.StreamChunk{Content: chunk.Choices[0].Delta.Content}
					if chunk.Choices[0].FinishReason != nil {
						sc.FinishReason = *chunk.Choices[0].FinishReason
					}
					ch <- sc
				}
			}
		}
		if err != nil {
			return
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestOpenAICompatProvider_Complete_SendsCorrectRequest ./internal/brain/ -count=1`
Expected: PASS

- [ ] **Step 5: Write tests for streaming, model discovery, error handling, and availability**

```go
// Append to internal/brain/openai_compat_provider_test.go

func TestOpenAICompatProvider_CompleteStream_ReturnsChunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`{"choices":[{"delta":{"content":"hel"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":"lo"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":""},"finish_reason":"stop"}]}`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	p := NewOpenAICompatProvider(OpenAICompatConfig{
		Name: "test", BaseURL: srv.URL, APIKey: "k", AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	})

	ch, err := p.CompleteStream(context.Background(), &types.InternalChatRequest{
		Model:    "m",
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)

	var content string
	for chunk := range ch {
		content += chunk.Content
	}
	assert.Equal(t, "hello", content)
}

func TestOpenAICompatProvider_Complete_Returns429AsProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate_limit exceeded"}}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatProvider(OpenAICompatConfig{
		Name: "test", BaseURL: srv.URL, APIKey: "k", AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	})

	_, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model: "m", Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.Error(t, err)
	var pe *ProviderError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, 429, pe.StatusCode)
	assert.Equal(t, "60", pe.Headers.Get("Retry-After"))
}

func TestOpenAICompatProvider_FetchModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{
				{"id": "model-a"},
				{"id": "model-b:free"},
				{"id": "model-c"},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenAICompatProvider(OpenAICompatConfig{
		Name: "test", BaseURL: srv.URL, APIKey: "k", AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	})

	// Without filter
	err := p.FetchModels(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"model-a", "model-b:free", "model-c"}, p.Models())

	// With filter
	err = p.FetchModels(context.Background(), func(id string) bool {
		return len(id) > 5 && id[len(id)-5:] == ":free"
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"model-b:free"}, p.Models())
}

func TestOpenAICompatProvider_Available(t *testing.T) {
	p := NewOpenAICompatProvider(OpenAICompatConfig{Name: "test", APIKey: "k"})
	assert.True(t, p.Available())

	p2 := NewOpenAICompatProvider(OpenAICompatConfig{Name: "test", APIKey: ""})
	assert.False(t, p2.Available())
}
```

- [ ] **Step 6: Run all tests to verify**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestOpenAICompatProvider ./internal/brain/ -count=1`
Expected: 4 PASS

- [ ] **Step 7: Commit**

```bash
cd HelixLLM && git add internal/brain/openai_compat_provider.go internal/brain/openai_compat_provider_test.go
git commit -m "feat(brain): add OpenAICompatProvider shared base for multi-provider fallback"
```

---

### Task 2: Chutes Provider

**Files:**
- Create: `internal/brain/chutes_provider.go`
- Create: `internal/brain/chutes_provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/brain/chutes_provider_test.go
package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/pkg/types"
)

func TestChutesProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-chutes-key", r.Header.Get("Authorization"))
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "chatcmpl-chutes", "model": "deepseek-ai/DeepSeek-V3-0324",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "from chutes"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	p := NewChutesProvider("test-chutes-key", srv.URL+"/v1")
	assert.Equal(t, "chutes", p.Name())
	assert.True(t, p.Available())

	resp, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model: "deepseek-ai/DeepSeek-V3-0324",
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "from chutes", resp.Message.Content)
	assert.Equal(t, "chutes", resp.Provider.Name)
}

func TestChutesProvider_FetchModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]string{
					{"id": "deepseek-ai/DeepSeek-V3-0324"},
					{"id": "Qwen/Qwen3-235B-A22B"},
				},
			})
			return
		}
	}))
	defer srv.Close()

	p := NewChutesProvider("key", srv.URL+"/v1")
	err := p.FetchModels(context.Background(), nil)
	require.NoError(t, err)
	assert.Contains(t, p.Models(), "deepseek-ai/DeepSeek-V3-0324")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestChutesProvider ./internal/brain/ -count=1`
Expected: FAIL — `NewChutesProvider` undefined

- [ ] **Step 3: Write the Chutes provider**

```go
// internal/brain/chutes_provider.go
package brain

// ChutesProvider wraps OpenAICompatProvider for https://llm.chutes.ai/v1.
type ChutesProvider struct {
	*OpenAICompatProvider
}

func NewChutesProvider(apiKey, baseURL string) *ChutesProvider {
	if baseURL == "" {
		baseURL = "https://llm.chutes.ai/v1"
	}
	return &ChutesProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:       "chutes",
			BaseURL:    baseURL,
			APIKey:     apiKey,
			AuthHeader: "Authorization",
			AuthPrefix: "Bearer ",
		}),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestChutesProvider ./internal/brain/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/chutes_provider.go internal/brain/chutes_provider_test.go
git commit -m "feat(brain): add Chutes provider using OpenAICompatProvider base"
```

---

### Task 3: OpenRouter Provider (with Free Model Filter)

**Files:**
- Create: `internal/brain/openrouter_provider.go`
- Create: `internal/brain/openrouter_provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/brain/openrouter_provider_test.go
package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/pkg/types"
)

func TestOpenRouterProvider_FetchModels_FiltersFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{
				{"id": "openai/gpt-4o"},
				{"id": "deepseek/deepseek-chat-v3-0324:free"},
				{"id": "qwen/qwen3-235b-a22b:free"},
				{"id": "anthropic/claude-3.5-sonnet"},
			},
		})
	}))
	defer srv.Close()

	p := NewOpenRouterProvider("key", srv.URL+"/api/v1")
	err := p.DiscoverFreeModels(context.Background())
	require.NoError(t, err)
	models := p.Models()
	assert.Len(t, models, 2)
	assert.Contains(t, models, "deepseek/deepseek-chat-v3-0324:free")
	assert.Contains(t, models, "qwen/qwen3-235b-a22b:free")
}

func TestOpenRouterProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-or-key", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "gen-or", "model": "deepseek/deepseek-chat-v3-0324:free",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "free reply"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer srv.Close()

	p := NewOpenRouterProvider("test-or-key", srv.URL+"/api/v1")
	resp, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model: "deepseek/deepseek-chat-v3-0324:free",
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "free reply", resp.Message.Content)
	assert.Equal(t, "openrouter", resp.Provider.Name)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestOpenRouterProvider ./internal/brain/ -count=1`
Expected: FAIL — `NewOpenRouterProvider` undefined

- [ ] **Step 3: Write the OpenRouter provider**

```go
// internal/brain/openrouter_provider.go
package brain

import (
	"context"
	"strings"
)

// OpenRouterProvider wraps OpenAICompatProvider for https://openrouter.ai/api/v1.
// DiscoverFreeModels filters for models with the ":free" suffix.
type OpenRouterProvider struct {
	*OpenAICompatProvider
}

func NewOpenRouterProvider(apiKey, baseURL string) *OpenRouterProvider {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	return &OpenRouterProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:       "openrouter",
			BaseURL:    baseURL,
			APIKey:     apiKey,
			AuthHeader: "Authorization",
			AuthPrefix: "Bearer ",
		}),
	}
}

// DiscoverFreeModels fetches all models and keeps only those ending in ":free".
func (p *OpenRouterProvider) DiscoverFreeModels(ctx context.Context) error {
	return p.FetchModels(ctx, func(id string) bool {
		return strings.HasSuffix(id, ":free")
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestOpenRouterProvider ./internal/brain/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/openrouter_provider.go internal/brain/openrouter_provider_test.go
git commit -m "feat(brain): add OpenRouter provider with :free model filtering"
```

---

### Task 4: HuggingFace Provider

**Files:**
- Create: `internal/brain/huggingface_provider.go`
- Create: `internal/brain/huggingface_provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/brain/huggingface_provider_test.go
package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/pkg/types"
)

func TestHuggingFaceProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-hf-key", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "chatcmpl-hf", "model": "meta-llama/Llama-3.1-70B-Instruct",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "hf reply"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	p := NewHuggingFaceProvider("test-hf-key", srv.URL+"/v1")
	assert.Equal(t, "huggingface", p.Name())

	resp, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model:    "meta-llama/Llama-3.1-70B-Instruct",
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "hf reply", resp.Message.Content)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestHuggingFaceProvider ./internal/brain/ -count=1`
Expected: FAIL — `NewHuggingFaceProvider` undefined

- [ ] **Step 3: Write the HuggingFace provider**

```go
// internal/brain/huggingface_provider.go
package brain

import "context"

// HuggingFaceProvider wraps OpenAICompatProvider for https://router.huggingface.co/v1.
type HuggingFaceProvider struct {
	*OpenAICompatProvider
}

func NewHuggingFaceProvider(apiKey, baseURL string) *HuggingFaceProvider {
	if baseURL == "" {
		baseURL = "https://router.huggingface.co/v1"
	}
	return &HuggingFaceProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:       "huggingface",
			BaseURL:    baseURL,
			APIKey:     apiKey,
			AuthHeader: "Authorization",
			AuthPrefix: "Bearer ",
		}),
	}
}

// DiscoverFreeModels fetches models and filters by text-generation pipeline.
func (p *HuggingFaceProvider) DiscoverFreeModels(ctx context.Context) error {
	return p.FetchModels(ctx, func(id string) bool {
		// HuggingFace router exposes only models available for free inference;
		// all models returned by the /v1/models endpoint are usable.
		return true
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestHuggingFaceProvider ./internal/brain/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/huggingface_provider.go internal/brain/huggingface_provider_test.go
git commit -m "feat(brain): add HuggingFace provider for free inference API"
```

---

### Task 5: Nvidia, Cerebras, SambaNova, Together Providers

**Files:**
- Create: `internal/brain/nvidia_provider.go`, `internal/brain/nvidia_provider_test.go`
- Create: `internal/brain/cerebras_provider.go`, `internal/brain/cerebras_provider_test.go`
- Create: `internal/brain/sambanova_provider.go`, `internal/brain/sambanova_provider_test.go`
- Create: `internal/brain/together_provider.go`, `internal/brain/together_provider_test.go`

- [ ] **Step 1: Write failing tests for all four providers**

```go
// internal/brain/nvidia_provider_test.go
package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/pkg/types"
)

func TestNvidiaProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer nv-key", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "nv-1", "model": "meta/llama-3.1-70b-instruct",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "nvidia reply"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	p := NewNvidiaProvider("nv-key", srv.URL+"/v1")
	assert.Equal(t, "nvidia", p.Name())
	resp, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model: "meta/llama-3.1-70b-instruct", Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "nvidia reply", resp.Message.Content)
}
```

```go
// internal/brain/cerebras_provider_test.go
package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/pkg/types"
)

func TestCerebrasProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer cb-key", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "cb-1", "model": "llama-3.3-70b",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "cerebras reply"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	p := NewCerebrasProvider("cb-key", srv.URL+"/v1")
	assert.Equal(t, "cerebras", p.Name())
	resp, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model: "llama-3.3-70b", Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "cerebras reply", resp.Message.Content)
}
```

```go
// internal/brain/sambanova_provider_test.go
package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/pkg/types"
)

func TestSambaNovaProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer sn-key", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "sn-1", "model": "Meta-Llama-3.1-70B-Instruct",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "sambanova reply"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	p := NewSambaNovaProvider("sn-key", srv.URL+"/v1")
	assert.Equal(t, "sambanova", p.Name())
	resp, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model: "Meta-Llama-3.1-70B-Instruct", Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "sambanova reply", resp.Message.Content)
}
```

```go
// internal/brain/together_provider_test.go
package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/pkg/types"
)

func TestTogetherProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tog-key", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "tog-1", "model": "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "together reply"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	}))
	defer srv.Close()

	p := NewTogetherProvider("tog-key", srv.URL+"/v1")
	assert.Equal(t, "together", p.Name())
	resp, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model: "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "together reply", resp.Message.Content)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run 'TestNvidiaProvider|TestCerebrasProvider|TestSambaNovaProvider|TestTogetherProvider' ./internal/brain/ -count=1`
Expected: FAIL — constructors undefined

- [ ] **Step 3: Write all four providers**

```go
// internal/brain/nvidia_provider.go
package brain

// NvidiaProvider wraps OpenAICompatProvider for https://integrate.api.nvidia.com/v1.
type NvidiaProvider struct{ *OpenAICompatProvider }

func NewNvidiaProvider(apiKey, baseURL string) *NvidiaProvider {
	if baseURL == "" {
		baseURL = "https://integrate.api.nvidia.com/v1"
	}
	return &NvidiaProvider{OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
		Name: "nvidia", BaseURL: baseURL, APIKey: apiKey, AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	})}
}
```

```go
// internal/brain/cerebras_provider.go
package brain

// CerebrasProvider wraps OpenAICompatProvider for https://api.cerebras.ai/v1.
type CerebrasProvider struct{ *OpenAICompatProvider }

func NewCerebrasProvider(apiKey, baseURL string) *CerebrasProvider {
	if baseURL == "" {
		baseURL = "https://api.cerebras.ai/v1"
	}
	return &CerebrasProvider{OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
		Name: "cerebras", BaseURL: baseURL, APIKey: apiKey, AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	})}
}
```

```go
// internal/brain/sambanova_provider.go
package brain

// SambaNovaProvider wraps OpenAICompatProvider for https://api.sambanova.ai/v1.
type SambaNovaProvider struct{ *OpenAICompatProvider }

func NewSambaNovaProvider(apiKey, baseURL string) *SambaNovaProvider {
	if baseURL == "" {
		baseURL = "https://api.sambanova.ai/v1"
	}
	return &SambaNovaProvider{OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
		Name: "sambanova", BaseURL: baseURL, APIKey: apiKey, AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	})}
}
```

```go
// internal/brain/together_provider.go
package brain

// TogetherProvider wraps OpenAICompatProvider for https://api.together.xyz/v1.
type TogetherProvider struct{ *OpenAICompatProvider }

func NewTogetherProvider(apiKey, baseURL string) *TogetherProvider {
	if baseURL == "" {
		baseURL = "https://api.together.xyz/v1"
	}
	return &TogetherProvider{OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
		Name: "together", BaseURL: baseURL, APIKey: apiKey, AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	})}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run 'TestNvidiaProvider|TestCerebrasProvider|TestSambaNovaProvider|TestTogetherProvider' ./internal/brain/ -count=1`
Expected: 4 PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/brain/nvidia_provider.go internal/brain/nvidia_provider_test.go \
  internal/brain/cerebras_provider.go internal/brain/cerebras_provider_test.go \
  internal/brain/sambanova_provider.go internal/brain/sambanova_provider_test.go \
  internal/brain/together_provider.go internal/brain/together_provider_test.go
git commit -m "feat(brain): add Nvidia, Cerebras, SambaNova, Together providers"
```

---

### Task 6: Config and Env Var Loading

**Files:**
- Modify: `internal/shared/config/config.go:45-61` — add 7 env var fields to `LLMConfig`
- Modify: `internal/brain/brain.go:30-42` — add 7 key fields to brain `Config`

- [ ] **Step 1: Add env var fields to LLMConfig**

In `internal/shared/config/config.go`, add after line 55 (`AnthropicKey`):

```go
	ChutesKey      string `env:"HELIX_LLM_CHUTES_KEY"`
	OpenRouterKey  string `env:"HELIX_LLM_OPENROUTER_KEY"`
	HuggingFaceKey string `env:"HELIX_LLM_HUGGINGFACE_KEY"`
	NvidiaKey      string `env:"HELIX_LLM_NVIDIA_KEY"`
	CerebrasKey    string `env:"HELIX_LLM_CEREBRAS_KEY"`
	SambaNovaKey   string `env:"HELIX_LLM_SAMBANOVA_KEY"`
	TogetherKey    string `env:"HELIX_LLM_TOGETHER_KEY"`
	// Fallback chain config
	VerifierURL          string `env:"HELIX_LLM_VERIFIER_URL" default:"http://localhost:7061"`
	ScoreRefreshInterval string `env:"HELIX_LLM_SCORE_REFRESH_INTERVAL" default:"5m"`
	MemorySyncEnabled    bool   `env:"HELIX_LLM_MEMORY_SYNC_ENABLED" default:"false"`
	MemoryURL            string `env:"HELIX_LLM_MEMORY_URL" default:"http://localhost:7061"`
```

- [ ] **Step 2: Add key fields to brain Config**

In `internal/brain/brain.go`, add after line 39 (`AnthropicBaseURL`):

```go
	ChutesKey      string
	OpenRouterKey  string
	HuggingFaceKey string
	NvidiaKey      string
	CerebrasKey    string
	SambaNovaKey   string
	TogetherKey    string
```

- [ ] **Step 3: Verify compilation**

Run: `cd HelixLLM && GOMAXPROCS=2 go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
cd HelixLLM && git add internal/shared/config/config.go internal/brain/brain.go
git commit -m "feat(config): add env vars for 7 free cloud providers and fallback chain"
```

---

### Task 7: Register Providers in Brain.New()

**Files:**
- Modify: `internal/brain/brain.go:44-94` — add provider registration blocks

- [ ] **Step 1: Add registration blocks in Brain.New()**

In `internal/brain/brain.go`, after the Anthropic registration block (around line 88), add:

```go
	if cfg.ChutesKey != "" {
		p := NewChutesProvider(cfg.ChutesKey, "")
		b.providers["chutes"] = p
		b.router.Register("chutes", p)
	}

	if cfg.OpenRouterKey != "" {
		p := NewOpenRouterProvider(cfg.OpenRouterKey, "")
		b.providers["openrouter"] = p
		b.router.Register("openrouter", p)
	}

	if cfg.HuggingFaceKey != "" {
		p := NewHuggingFaceProvider(cfg.HuggingFaceKey, "")
		b.providers["huggingface"] = p
		b.router.Register("huggingface", p)
	}

	if cfg.NvidiaKey != "" {
		p := NewNvidiaProvider(cfg.NvidiaKey, "")
		b.providers["nvidia"] = p
		b.router.Register("nvidia", p)
	}

	if cfg.CerebrasKey != "" {
		p := NewCerebrasProvider(cfg.CerebrasKey, "")
		b.providers["cerebras"] = p
		b.router.Register("cerebras", p)
	}

	if cfg.SambaNovaKey != "" {
		p := NewSambaNovaProvider(cfg.SambaNovaKey, "")
		b.providers["sambanova"] = p
		b.router.Register("sambanova", p)
	}

	if cfg.TogetherKey != "" {
		p := NewTogetherProvider(cfg.TogetherKey, "")
		b.providers["together"] = p
		b.router.Register("together", p)
	}
```

- [ ] **Step 2: Add Providers() accessor to Brain**

Append to `internal/brain/brain.go`:

```go
// Providers returns a copy of the registered providers map.
func (b *Brain) Providers() map[string]Provider {
	out := make(map[string]Provider, len(b.providers))
	for k, v := range b.providers {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 3: Verify compilation and existing tests pass**

Run: `cd HelixLLM && GOMAXPROCS=2 go build ./... && GOMAXPROCS=2 go test ./internal/brain/ -count=1 -p 1`
Expected: Build succeeds, all tests pass

- [ ] **Step 4: Commit**

```bash
cd HelixLLM && git add internal/brain/brain.go
git commit -m "feat(brain): register 7 new free cloud providers in Brain.New()"
```

---

### Task 8: CircuitBreaker

**Files:**
- Create: `internal/fallback/circuit_breaker.go`
- Create: `internal/fallback/circuit_breaker_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fallback/circuit_breaker_test.go
package fallback

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_StartsClose(t *testing.T) {
	cb := NewCircuitBreaker(3, 2*time.Minute)
	assert.Equal(t, StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_OpensAfterConsecutiveFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 2*time.Minute)
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateClosed, cb.State())
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())
	assert.False(t, cb.Allow())
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(3, 2*time.Minute)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateClosed, cb.State()) // still closed, count reset
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())

	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())
	assert.True(t, cb.Allow()) // one probe allowed
}

func TestCircuitBreaker_HalfOpen_SuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.RecordSuccess()
	assert.Equal(t, StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_HalfOpen_FailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())
	assert.False(t, cb.Allow())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestCircuitBreaker ./internal/fallback/ -count=1`
Expected: FAIL — package doesn't exist

- [ ] **Step 3: Write CircuitBreaker implementation**

```go
// internal/fallback/circuit_breaker.go
package fallback

import (
	"sync"
	"time"
)

// CircuitState represents the circuit breaker state.
type CircuitState int

const (
	StateClosed   CircuitState = iota // normal operation
	StateOpen                         // rejecting requests
	StateHalfOpen                     // allowing one probe
)

// CircuitBreaker tracks consecutive failures and opens the circuit to prevent
// cascading failures. Thread-safe.
type CircuitBreaker struct {
	maxFailures int
	openTimeout time.Duration

	mu                sync.Mutex
	consecutiveErrors int
	state             CircuitState
	openedAt          time.Time
}

func NewCircuitBreaker(maxFailures int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		openTimeout: openTimeout,
		state:       StateClosed,
	}
}

// State returns the current circuit state. If the circuit is open and the
// timeout has elapsed, it transitions to half-open.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateOpen && time.Since(cb.openedAt) >= cb.openTimeout {
		cb.state = StateHalfOpen
	}
	return cb.state
}

// Allow returns true if a request should be attempted.
func (cb *CircuitBreaker) Allow() bool {
	s := cb.State()
	return s == StateClosed || s == StateHalfOpen
}

// RecordSuccess records a successful request. Resets failure count and closes
// the circuit if it was half-open.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveErrors = 0
	cb.state = StateClosed
}

// RecordFailure records a failed request. If consecutive failures reach the
// threshold, opens the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveErrors++
	if cb.consecutiveErrors >= cb.maxFailures || cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.openedAt = time.Now()
		cb.consecutiveErrors = 0
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestCircuitBreaker ./internal/fallback/ -count=1`
Expected: 6 PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/fallback/circuit_breaker.go internal/fallback/circuit_breaker_test.go
git commit -m "feat(fallback): add CircuitBreaker with open/close/half-open transitions"
```

---

### Task 9: RateLimitTracker

**Files:**
- Create: `internal/fallback/rate_limit.go`
- Create: `internal/fallback/rate_limit_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fallback/rate_limit_test.go
package fallback

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitTracker_ParseHeaders_Standard(t *testing.T) {
	rt := NewRateLimitTracker(5, 1000)
	h := http.Header{}
	h.Set("x-ratelimit-remaining-requests", "10")
	h.Set("x-ratelimit-remaining-tokens", "5000")
	h.Set("x-ratelimit-reset", "1713000000") // Unix timestamp

	rt.UpdateFromHeaders("provider-a", h)
	state := rt.Get("provider-a")

	assert.Equal(t, 10, state.RemainingRequests)
	assert.Equal(t, 5000, state.RemainingTokens)
	assert.False(t, rt.ShouldSkip("provider-a"))
}

func TestRateLimitTracker_ShouldSkip_LowRequests(t *testing.T) {
	rt := NewRateLimitTracker(5, 1000)
	h := http.Header{}
	h.Set("x-ratelimit-remaining-requests", "3") // below threshold of 5
	h.Set("x-ratelimit-remaining-tokens", "50000")

	rt.UpdateFromHeaders("provider-a", h)
	assert.True(t, rt.ShouldSkip("provider-a"))
}

func TestRateLimitTracker_ShouldSkip_LowTokens(t *testing.T) {
	rt := NewRateLimitTracker(5, 1000)
	h := http.Header{}
	h.Set("x-ratelimit-remaining-requests", "100")
	h.Set("x-ratelimit-remaining-tokens", "500") // below threshold of 1000

	rt.UpdateFromHeaders("provider-a", h)
	assert.True(t, rt.ShouldSkip("provider-a"))
}

func TestRateLimitTracker_ShouldSkip_UnknownProvider(t *testing.T) {
	rt := NewRateLimitTracker(5, 1000)
	assert.False(t, rt.ShouldSkip("unknown"))
}

func TestRateLimitTracker_ParseHeaders_RetryAfter(t *testing.T) {
	rt := NewRateLimitTracker(5, 1000)
	h := http.Header{}
	h.Set("Retry-After", "120")

	cooldown := rt.ParseRetryAfter(h)
	assert.InDelta(t, 120, cooldown.Seconds(), 1)
}

func TestRateLimitTracker_ParseHeaders_NoHeaders(t *testing.T) {
	rt := NewRateLimitTracker(5, 1000)
	rt.UpdateFromHeaders("provider-b", http.Header{})
	assert.False(t, rt.ShouldSkip("provider-b")) // no data = don't skip
}

func TestRateLimitTracker_ExponentialBackoff(t *testing.T) {
	rt := NewRateLimitTracker(5, 1000)
	d1 := rt.NextBackoff("provider-a")
	assert.Equal(t, 60*time.Second, d1)
	d2 := rt.NextBackoff("provider-a")
	assert.Equal(t, 120*time.Second, d2)
	d3 := rt.NextBackoff("provider-a")
	assert.Equal(t, 240*time.Second, d3)
	d4 := rt.NextBackoff("provider-a")
	assert.Equal(t, 480*time.Second, d4)
	d5 := rt.NextBackoff("provider-a")
	assert.Equal(t, 15*time.Minute, d5) // capped at 15m

	rt.ResetBackoff("provider-a")
	d6 := rt.NextBackoff("provider-a")
	assert.Equal(t, 60*time.Second, d6)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestRateLimitTracker ./internal/fallback/ -count=1`
Expected: FAIL — `NewRateLimitTracker` undefined

- [ ] **Step 3: Write RateLimitTracker implementation**

```go
// internal/fallback/rate_limit.go
package fallback

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

const maxBackoff = 15 * time.Minute

// RateLimitState holds parsed rate limit data from response headers.
type RateLimitState struct {
	RemainingRequests int
	RemainingTokens   int
	ResetAt           time.Time
	LastUpdated       time.Time
}

// RateLimitTracker parses rate limit headers and tracks per-provider state.
type RateLimitTracker struct {
	minRequests int
	minTokens   int

	mu       sync.RWMutex
	states   map[string]*RateLimitState
	backoffs map[string]int // exponential backoff attempt count
}

func NewRateLimitTracker(minRequests, minTokens int) *RateLimitTracker {
	return &RateLimitTracker{
		minRequests: minRequests,
		minTokens:   minTokens,
		states:      make(map[string]*RateLimitState),
		backoffs:    make(map[string]int),
	}
}

// UpdateFromHeaders parses standard rate limit headers and stores the state.
func (rt *RateLimitTracker) UpdateFromHeaders(provider string, h http.Header) {
	remaining := parseIntHeader(h, "x-ratelimit-remaining-requests",
		"x-ratelimit-remaining", "ratelimit-remaining")
	tokens := parseIntHeader(h, "x-ratelimit-remaining-tokens")
	resetAt := parseResetHeader(h, "x-ratelimit-reset", "ratelimit-reset")

	// Only store if we got at least one useful header.
	if remaining < 0 && tokens < 0 {
		return
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.states[provider] = &RateLimitState{
		RemainingRequests: max(remaining, 0),
		RemainingTokens:   max(tokens, 0),
		ResetAt:           resetAt,
		LastUpdated:       time.Now(),
	}
}

// Get returns the current state for a provider. Returns zero state if unknown.
func (rt *RateLimitTracker) Get(provider string) RateLimitState {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if s, ok := rt.states[provider]; ok {
		return *s
	}
	return RateLimitState{}
}

// ShouldSkip returns true if the provider is approaching its rate limit.
func (rt *RateLimitTracker) ShouldSkip(provider string) bool {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	s, ok := rt.states[provider]
	if !ok {
		return false // no data = assume OK
	}
	return s.RemainingRequests < rt.minRequests || s.RemainingTokens < rt.minTokens
}

// ParseRetryAfter parses the Retry-After header and returns a duration.
func (rt *RateLimitTracker) ParseRetryAfter(h http.Header) time.Duration {
	val := h.Get("Retry-After")
	if val == "" {
		return 0
	}
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(val); err == nil {
		return time.Until(t)
	}
	return 0
}

// NextBackoff returns the next exponential backoff duration for the provider
// and increments the attempt counter. Base: 60s, factor: 2x, cap: 15m.
func (rt *RateLimitTracker) NextBackoff(provider string) time.Duration {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	attempt := rt.backoffs[provider]
	rt.backoffs[provider] = attempt + 1

	d := 60 * time.Second
	for i := 0; i < attempt; i++ {
		d *= 2
		if d > maxBackoff {
			return maxBackoff
		}
	}
	return d
}

// ResetBackoff resets the backoff counter for a provider.
func (rt *RateLimitTracker) ResetBackoff(provider string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.backoffs, provider)
}

func parseIntHeader(h http.Header, keys ...string) int {
	for _, k := range keys {
		if v := h.Get(k); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}
	return -1
}

func parseResetHeader(h http.Header, keys ...string) time.Time {
	for _, k := range keys {
		if v := h.Get(k); v != "" {
			if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
				return time.Unix(ts, 0)
			}
		}
	}
	return time.Time{}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestRateLimitTracker ./internal/fallback/ -count=1`
Expected: 7 PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/fallback/rate_limit.go internal/fallback/rate_limit_test.go
git commit -m "feat(fallback): add RateLimitTracker with header parsing and exponential backoff"
```

---

### Task 10: ChainEntry Type

**Files:**
- Create: `internal/fallback/chain_entry.go`

- [ ] **Step 1: Write ChainEntry type and EntryStatus**

```go
// internal/fallback/chain_entry.go
package fallback

import (
	"time"
)

// EntryStatus represents the health status of a chain entry.
type EntryStatus int

const (
	EntryActive    EntryStatus = iota // normal operation
	EntryExhausted                    // rate limited, cooling down
	EntryCircuitOpen                  // circuit breaker tripped
)

func (s EntryStatus) String() string {
	switch s {
	case EntryActive:
		return "active"
	case EntryExhausted:
		return "exhausted"
	case EntryCircuitOpen:
		return "circuit_open"
	default:
		return "unknown"
	}
}

// ChainEntry wraps a provider name with fallback-chain state.
type ChainEntry struct {
	ProviderName   string
	ModelID        string
	Score          float64
	Status         EntryStatus
	CooldownUntil  time.Time
	CircuitBreaker *CircuitBreaker
	IsLocalFallback bool // true for llama.cpp — always sorted last
}

// Available returns true if this entry can accept a request right now.
func (e *ChainEntry) Available() bool {
	if e.Status == EntryExhausted && time.Now().After(e.CooldownUntil) {
		e.Status = EntryActive
	}
	if e.Status == EntryExhausted {
		return false
	}
	if e.CircuitBreaker != nil && !e.CircuitBreaker.Allow() {
		return false
	}
	return true
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd HelixLLM && GOMAXPROCS=2 go build ./internal/fallback/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
cd HelixLLM && git add internal/fallback/chain_entry.go
git commit -m "feat(fallback): add ChainEntry type with status and availability checks"
```

---

### Task 11: FallbackChain Core

**Files:**
- Create: `internal/fallback/chain.go`
- Create: `internal/fallback/chain_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fallback/chain_test.go
package fallback

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/internal/brain"
	"dev.helix.llm/pkg/types"
)

// mockProvider implements brain.Provider for testing.
type mockProvider struct {
	name      string
	available bool
	models    []string
	resp      *types.InternalChatResponse
	err       error
}

func (m *mockProvider) Complete(ctx context.Context, req *types.InternalChatRequest) (*types.InternalChatResponse, error) {
	return m.resp, m.err
}
func (m *mockProvider) CompleteStream(ctx context.Context, req *types.InternalChatRequest) (<-chan types.StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan types.StreamChunk, 1)
	ch <- types.StreamChunk{Content: m.resp.Message.Content, FinishReason: "stop"}
	close(ch)
	return ch, nil
}
func (m *mockProvider) Models() []string  { return m.models }
func (m *mockProvider) Name() string      { return m.name }
func (m *mockProvider) Available() bool   { return m.available }

func TestChain_Complete_UsesFirstAvailableEntry(t *testing.T) {
	providers := map[string]brain.Provider{
		"fast": &mockProvider{
			name: "fast", available: true, models: []string{"model-a"},
			resp: &types.InternalChatResponse{Message: types.InternalMessage{Content: "fast reply"}},
		},
		"slow": &mockProvider{
			name: "slow", available: true, models: []string{"model-b"},
			resp: &types.InternalChatResponse{Message: types.InternalMessage{Content: "slow reply"}},
		},
	}

	chain := NewChain(providers, NewRateLimitTracker(5, 1000))
	chain.SetEntries([]ChainEntry{
		{ProviderName: "fast", ModelID: "model-a", Score: 90, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
		{ProviderName: "slow", ModelID: "model-b", Score: 70, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
	})

	resp, err := chain.Complete(context.Background(), &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "fast reply", resp.Message.Content)
}

func TestChain_Complete_FallsBackOn429(t *testing.T) {
	providers := map[string]brain.Provider{
		"primary": &mockProvider{
			name: "primary", available: true, models: []string{"m1"},
			err: &brain.ProviderError{Provider: "primary", StatusCode: 429, Headers: http.Header{}},
		},
		"secondary": &mockProvider{
			name: "secondary", available: true, models: []string{"m2"},
			resp: &types.InternalChatResponse{Message: types.InternalMessage{Content: "secondary reply"}},
		},
	}

	chain := NewChain(providers, NewRateLimitTracker(5, 1000))
	chain.SetEntries([]ChainEntry{
		{ProviderName: "primary", ModelID: "m1", Score: 90, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
		{ProviderName: "secondary", ModelID: "m2", Score: 70, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
	})

	resp, err := chain.Complete(context.Background(), &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "secondary reply", resp.Message.Content)
}

func TestChain_Complete_FallsBackOn5xx(t *testing.T) {
	providers := map[string]brain.Provider{
		"broken": &mockProvider{
			name: "broken", available: true, models: []string{"m1"},
			err: &brain.ProviderError{Provider: "broken", StatusCode: 500, Headers: http.Header{}},
		},
		"ok": &mockProvider{
			name: "ok", available: true, models: []string{"m2"},
			resp: &types.InternalChatResponse{Message: types.InternalMessage{Content: "ok reply"}},
		},
	}

	chain := NewChain(providers, NewRateLimitTracker(5, 1000))
	chain.SetEntries([]ChainEntry{
		{ProviderName: "broken", ModelID: "m1", Score: 90, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
		{ProviderName: "ok", ModelID: "m2", Score: 70, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
	})

	resp, err := chain.Complete(context.Background(), &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok reply", resp.Message.Content)
}

func TestChain_Complete_LlamaCppAlwaysLast(t *testing.T) {
	providers := map[string]brain.Provider{
		"llamacpp": &mockProvider{
			name: "llamacpp", available: true, models: []string{"local"},
			resp: &types.InternalChatResponse{Message: types.InternalMessage{Content: "local reply"}},
		},
		"cloud": &mockProvider{
			name: "cloud", available: true, models: []string{"m1"},
			err: &brain.ProviderError{Provider: "cloud", StatusCode: 429, Headers: http.Header{}},
		},
	}

	chain := NewChain(providers, NewRateLimitTracker(5, 1000))
	chain.SetEntries([]ChainEntry{
		{ProviderName: "cloud", ModelID: "m1", Score: 90, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
		{ProviderName: "llamacpp", ModelID: "local", Score: 0, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0), IsLocalFallback: true},
	})

	resp, err := chain.Complete(context.Background(), &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "local reply", resp.Message.Content)
}

func TestChain_Complete_AllExhausted_ReturnsError(t *testing.T) {
	providers := map[string]brain.Provider{
		"a": &mockProvider{name: "a", available: true, err: errors.New("fail")},
	}

	chain := NewChain(providers, NewRateLimitTracker(5, 1000))
	chain.SetEntries([]ChainEntry{
		{ProviderName: "a", ModelID: "m1", Score: 90, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
	})

	_, err := chain.Complete(context.Background(), &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all providers exhausted")
}

func TestChain_Complete_ConcurrentSafety(t *testing.T) {
	providers := map[string]brain.Provider{
		"safe": &mockProvider{
			name: "safe", available: true, models: []string{"m1"},
			resp: &types.InternalChatResponse{Message: types.InternalMessage{Content: "ok"}},
		},
	}

	chain := NewChain(providers, NewRateLimitTracker(5, 1000))
	chain.SetEntries([]ChainEntry{
		{ProviderName: "safe", ModelID: "m1", Score: 90, Status: EntryActive, CircuitBreaker: NewCircuitBreaker(3, 0)},
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := chain.Complete(context.Background(), &types.InternalChatRequest{
				Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
			})
			assert.NoError(t, err)
			assert.Equal(t, "ok", resp.Message.Content)
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestChain ./internal/fallback/ -count=1`
Expected: FAIL — `NewChain` undefined

- [ ] **Step 3: Write FallbackChain implementation**

```go
// internal/fallback/chain.go
package fallback

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"dev.helix.llm/internal/brain"
	"dev.helix.llm/pkg/types"
)

// Chain routes requests through a scored list of providers with automatic
// failover on rate limits and errors. LlamaCpp is always the last resort.
type Chain struct {
	providers   map[string]brain.Provider
	entries     []ChainEntry
	rateLimiter *RateLimitTracker
	mu          sync.RWMutex
}

func NewChain(providers map[string]brain.Provider, rl *RateLimitTracker) *Chain {
	return &Chain{
		providers:   providers,
		rateLimiter: rl,
	}
}

// SetEntries replaces the ordered entry list. Thread-safe.
func (c *Chain) SetEntries(entries []ChainEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = entries
}

// Entries returns a snapshot of current entries. Thread-safe.
func (c *Chain) Entries() []ChainEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ChainEntry, len(c.entries))
	copy(out, c.entries)
	return out
}

// Complete tries each entry in order until one succeeds.
func (c *Chain) Complete(ctx context.Context, req *types.InternalChatRequest) (*types.InternalChatResponse, error) {
	entries := c.Entries()
	var lastErr error

	for i := range entries {
		entry := &entries[i]
		if !entry.Available() {
			continue
		}
		if c.rateLimiter.ShouldSkip(entry.ProviderName) && !entry.IsLocalFallback {
			slog.Debug("skipping provider (rate limit approaching)", "provider", entry.ProviderName)
			continue
		}

		provider, ok := c.providers[entry.ProviderName]
		if !ok || !provider.Available() {
			continue
		}

		// Override model in request to this entry's model.
		reqCopy := *req
		if entry.ModelID != "" {
			reqCopy.Model = entry.ModelID
		}

		resp, err := provider.Complete(ctx, &reqCopy)
		if err != nil {
			lastErr = err
			c.handleError(entry, err)
			continue
		}

		// Success — record it and return.
		entry.CircuitBreaker.RecordSuccess()
		c.rateLimiter.ResetBackoff(entry.ProviderName)
		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers exhausted, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("all providers exhausted: no available entries")
}

// CompleteStream tries each entry in order for streaming requests.
func (c *Chain) CompleteStream(ctx context.Context, req *types.InternalChatRequest) (<-chan types.StreamChunk, error) {
	entries := c.Entries()
	var lastErr error

	for i := range entries {
		entry := &entries[i]
		if !entry.Available() {
			continue
		}
		if c.rateLimiter.ShouldSkip(entry.ProviderName) && !entry.IsLocalFallback {
			continue
		}

		provider, ok := c.providers[entry.ProviderName]
		if !ok || !provider.Available() {
			continue
		}

		reqCopy := *req
		if entry.ModelID != "" {
			reqCopy.Model = entry.ModelID
		}

		ch, err := provider.CompleteStream(ctx, &reqCopy)
		if err != nil {
			lastErr = err
			c.handleError(entry, err)
			continue
		}

		entry.CircuitBreaker.RecordSuccess()
		c.rateLimiter.ResetBackoff(entry.ProviderName)
		return ch, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers exhausted, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("all providers exhausted: no available entries")
}

func (c *Chain) handleError(entry *ChainEntry, err error) {
	var pe *brain.ProviderError
	if !isProviderError(err, &pe) {
		entry.CircuitBreaker.RecordFailure()
		slog.Warn("provider error (non-HTTP)", "provider", entry.ProviderName, "err", err)
		return
	}

	if pe.StatusCode == http.StatusTooManyRequests {
		retryAfter := c.rateLimiter.ParseRetryAfter(pe.Headers)
		if retryAfter == 0 {
			retryAfter = c.rateLimiter.NextBackoff(entry.ProviderName)
		}
		entry.Status = EntryExhausted
		entry.CooldownUntil = time.Now().Add(retryAfter)
		slog.Warn("provider rate limited",
			"provider", entry.ProviderName,
			"cooldown", retryAfter,
			"until", entry.CooldownUntil.Format(time.RFC3339))
	} else if pe.StatusCode >= 500 {
		entry.CircuitBreaker.RecordFailure()
		slog.Warn("provider server error",
			"provider", entry.ProviderName,
			"status", pe.StatusCode)
	}
}

func isProviderError(err error, target **brain.ProviderError) bool {
	var pe *brain.ProviderError
	if ok := errors.As(err, &pe); ok {
		*target = pe
		return true
	}
	return false
}
```

Note: Add `"errors"` to the import block.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestChain ./internal/fallback/ -count=1`
Expected: 6 PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/fallback/chain.go internal/fallback/chain_test.go
git commit -m "feat(fallback): add FallbackChain with scored provider iteration and 429/5xx failover"
```

---

### Task 12: ScorerBridge (LLMsVerifier Integration)

**Files:**
- Create: `internal/fallback/scorer_bridge.go`
- Create: `internal/fallback/scorer_bridge_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fallback/scorer_bridge_test.go
package fallback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScorerBridge_StaticFallbackScores(t *testing.T) {
	sb := NewScorerBridge(ScorerBridgeConfig{})
	scores := sb.StaticFallbackScores()

	// OpenRouter should be highest in static fallback
	assert.Greater(t, scores["openrouter"], scores["chutes"])
	assert.Greater(t, scores["chutes"], scores["huggingface"])
	// All should be > 0
	for name, score := range scores {
		assert.Greater(t, score, 0.0, "provider %s should have positive score", name)
	}
}

func TestScorerBridge_FetchScores_FromAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"scores": map[string]interface{}{
				"chutes":     map[string]float64{"total": 85.0},
				"openrouter": map[string]float64{"total": 92.0},
				"nvidia":     map[string]float64{"total": 78.0},
			},
		})
	}))
	defer srv.Close()

	sb := NewScorerBridge(ScorerBridgeConfig{VerifierURL: srv.URL})
	scores, err := sb.FetchScores(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, 92.0, scores["openrouter"], 0.1)
	assert.InDelta(t, 85.0, scores["chutes"], 0.1)
	assert.InDelta(t, 78.0, scores["nvidia"], 0.1)
}

func TestScorerBridge_FetchScores_Unreachable_ReturnsStaticFallback(t *testing.T) {
	sb := NewScorerBridge(ScorerBridgeConfig{VerifierURL: "http://localhost:1"})
	scores, err := sb.FetchScores(context.Background())
	require.NoError(t, err) // graceful degradation, no error
	assert.Greater(t, scores["openrouter"], 0.0)
}

func TestScorerBridge_BuildEntries_LlamaCppAlwaysLast(t *testing.T) {
	sb := NewScorerBridge(ScorerBridgeConfig{})
	scores := map[string]float64{
		"llamacpp":   50.0,
		"openrouter": 90.0,
		"chutes":     80.0,
	}
	models := map[string]string{
		"llamacpp":   "local-model",
		"openrouter": "deepseek:free",
		"chutes":     "qwen3",
	}

	entries := sb.BuildEntries(scores, models)
	require.Len(t, entries, 3)
	assert.Equal(t, "openrouter", entries[0].ProviderName) // highest score
	assert.Equal(t, "chutes", entries[1].ProviderName)
	assert.Equal(t, "llamacpp", entries[2].ProviderName)   // always last
	assert.True(t, entries[2].IsLocalFallback)
}

func TestScorerBridge_RefreshLoop_UpdatesEntries(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]interface{}{
			"scores": map[string]interface{}{
				"chutes": map[string]float64{"total": float64(80 + callCount)},
			},
		})
	}))
	defer srv.Close()

	sb := NewScorerBridge(ScorerBridgeConfig{
		VerifierURL:     srv.URL,
		RefreshInterval: 50 * time.Millisecond,
	})

	chain := NewChain(nil, NewRateLimitTracker(5, 1000))
	chain.SetEntries([]ChainEntry{
		{ProviderName: "chutes", ModelID: "m1", Score: 80},
	})

	ctx, cancel := context.WithCancel(context.Background())
	sb.StartRefreshLoop(ctx, chain, map[string]string{"chutes": "m1"})
	time.Sleep(120 * time.Millisecond)
	cancel()
	sb.Wait()

	assert.Greater(t, callCount, 0)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestScorerBridge ./internal/fallback/ -count=1`
Expected: FAIL — `NewScorerBridge` undefined

- [ ] **Step 3: Write ScorerBridge implementation**

```go
// internal/fallback/scorer_bridge.go
package fallback

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ScorerBridgeConfig configures the LLMsVerifier integration.
type ScorerBridgeConfig struct {
	VerifierURL     string        // e.g., "http://localhost:7061"
	RefreshInterval time.Duration // default 5m
}

// ScorerBridge fetches provider scores from LLMsVerifier and builds
// ordered ChainEntry lists.
type ScorerBridge struct {
	verifierURL     string
	refreshInterval time.Duration
	httpClient      *http.Client
	wg              sync.WaitGroup
}

func NewScorerBridge(cfg ScorerBridgeConfig) *ScorerBridge {
	interval := cfg.RefreshInterval
	if interval == 0 {
		interval = 5 * time.Minute
	}
	return &ScorerBridge{
		verifierURL:     cfg.VerifierURL,
		refreshInterval: interval,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

// StaticFallbackScores returns hardcoded scores used when LLMsVerifier is
// unreachable. Ordering: OpenRouter > Chutes > HuggingFace > Nvidia > Cerebras > SambaNova > Together > llamacpp.
func (sb *ScorerBridge) StaticFallbackScores() map[string]float64 {
	return map[string]float64{
		"openrouter":  90.0,
		"chutes":      85.0,
		"huggingface": 80.0,
		"nvidia":      75.0,
		"cerebras":    70.0,
		"sambanova":   65.0,
		"together":    60.0,
		"llamacpp":    10.0,
	}
}

// FetchScores queries LLMsVerifier's API for live scores. On failure, returns
// static fallback scores (graceful degradation).
func (sb *ScorerBridge) FetchScores(ctx context.Context) (map[string]float64, error) {
	if sb.verifierURL == "" {
		return sb.StaticFallbackScores(), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		sb.verifierURL+"/api/scores", nil)
	if err != nil {
		return sb.StaticFallbackScores(), nil
	}

	resp, err := sb.httpClient.Do(req)
	if err != nil {
		slog.Warn("scorer bridge: verifier unreachable, using static fallback", "err", err)
		return sb.StaticFallbackScores(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("scorer bridge: verifier returned non-200", "status", resp.StatusCode)
		return sb.StaticFallbackScores(), nil
	}

	var result struct {
		Scores map[string]struct {
			Total float64 `json:"total"`
		} `json:"scores"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Warn("scorer bridge: decode error", "err", err)
		return sb.StaticFallbackScores(), nil
	}

	scores := make(map[string]float64, len(result.Scores))
	for name, s := range result.Scores {
		scores[name] = s.Total
	}
	return scores, nil
}

// BuildEntries creates a sorted []ChainEntry from scores and model selections.
// LlamaCpp is always placed last regardless of score.
func (sb *ScorerBridge) BuildEntries(scores map[string]float64, models map[string]string) []ChainEntry {
	var cloud []ChainEntry
	var local []ChainEntry

	for provider, model := range models {
		score := scores[provider]
		entry := ChainEntry{
			ProviderName:    provider,
			ModelID:         model,
			Score:           score,
			Status:          EntryActive,
			CircuitBreaker:  NewCircuitBreaker(3, 2*time.Minute),
			IsLocalFallback: provider == "llamacpp",
		}
		if provider == "llamacpp" {
			local = append(local, entry)
		} else {
			cloud = append(cloud, entry)
		}
	}

	sort.Slice(cloud, func(i, j int) bool {
		return cloud[i].Score > cloud[j].Score
	})

	return append(cloud, local...)
}

// StartRefreshLoop periodically fetches new scores and reorders the chain.
func (sb *ScorerBridge) StartRefreshLoop(ctx context.Context, chain *Chain, models map[string]string) {
	sb.wg.Add(1)
	go func() {
		defer sb.wg.Done()
		ticker := time.NewTicker(sb.refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				scores, _ := sb.FetchScores(ctx)
				entries := sb.BuildEntries(scores, models)
				chain.SetEntries(entries)
				slog.Info("scorer bridge: refreshed chain entries",
					"count", len(entries),
					"top", fmt.Sprintf("%s (%.1f)", entries[0].ProviderName, entries[0].Score))
			}
		}
	}()
}

// Wait blocks until the refresh loop exits.
func (sb *ScorerBridge) Wait() {
	sb.wg.Wait()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestScorerBridge ./internal/fallback/ -count=1`
Expected: 4 PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/fallback/scorer_bridge.go internal/fallback/scorer_bridge_test.go
git commit -m "feat(fallback): add ScorerBridge for LLMsVerifier integration with static fallback"
```

---

### Task 13: MemoryAdapter

**Files:**
- Create: `internal/fallback/memory_adapter.go`
- Create: `internal/fallback/memory_adapter_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fallback/memory_adapter_test.go
package fallback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryAdapter_BuffersAndFlushes(t *testing.T) {
	var receivedEntries []MemoryEntry
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var entries []MemoryEntry
		json.NewDecoder(r.Body).Decode(&entries)
		mu.Lock()
		receivedEntries = append(receivedEntries, entries...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ma := NewMemoryAdapter(MemoryAdapterConfig{
		HelixMemoryURL: srv.URL,
		SyncInterval:   50 * time.Millisecond,
		Enabled:        true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	ma.Start(ctx)

	ma.Queue(MemoryEntry{Content: "user likes Go", Type: "preference"})
	ma.Queue(MemoryEntry{Content: "project uses Gin", Type: "fact"})

	time.Sleep(100 * time.Millisecond)
	cancel()
	ma.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, receivedEntries, 2)
	assert.Equal(t, "user likes Go", receivedEntries[0].Content)
	assert.Equal(t, "project uses Gin", receivedEntries[1].Content)
}

func TestMemoryAdapter_CapsAt1000Entries(t *testing.T) {
	ma := NewMemoryAdapter(MemoryAdapterConfig{
		HelixMemoryURL: "http://localhost:1", // unreachable
		SyncInterval:   1 * time.Hour,         // won't fire
		Enabled:        true,
	})

	for i := 0; i < 1100; i++ {
		ma.Queue(MemoryEntry{Content: "entry"})
	}

	ma.mu.Lock()
	count := len(ma.pending)
	ma.mu.Unlock()
	assert.LessOrEqual(t, count, 1000)
}

func TestMemoryAdapter_DisabledNoOp(t *testing.T) {
	ma := NewMemoryAdapter(MemoryAdapterConfig{Enabled: false})
	ma.Queue(MemoryEntry{Content: "should be ignored"})

	ma.mu.Lock()
	count := len(ma.pending)
	ma.mu.Unlock()
	assert.Equal(t, 0, count)
}

func TestMemoryAdapter_GracefulDegradation_UnreachableServer(t *testing.T) {
	ma := NewMemoryAdapter(MemoryAdapterConfig{
		HelixMemoryURL: "http://localhost:1",
		SyncInterval:   50 * time.Millisecond,
		Enabled:        true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	ma.Start(ctx)

	ma.Queue(MemoryEntry{Content: "test"})
	time.Sleep(100 * time.Millisecond)
	cancel()
	ma.Wait()

	// Should not panic, entries remain in buffer
	ma.mu.Lock()
	count := len(ma.pending)
	ma.mu.Unlock()
	assert.Equal(t, 1, count) // still buffered, not lost
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestMemoryAdapter ./internal/fallback/ -count=1`
Expected: FAIL — `NewMemoryAdapter` undefined

- [ ] **Step 3: Write MemoryAdapter implementation**

```go
// internal/fallback/memory_adapter.go
package fallback

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const maxPendingMemories = 1000

// MemoryEntry represents a persistent memory to sync to HelixMemory.
type MemoryEntry struct {
	Content   string `json:"content"`
	Type      string `json:"type"`      // "preference", "fact", "learning"
	SessionID string `json:"session_id"`
}

// MemoryAdapterConfig configures the HelixMemory sync adapter.
type MemoryAdapterConfig struct {
	HelixMemoryURL string
	SyncInterval   time.Duration
	Enabled        bool
}

// MemoryAdapter buffers persistent memories and flushes them to HelixMemory
// in batches. Non-blocking on the request path.
type MemoryAdapter struct {
	url          string
	syncInterval time.Duration
	enabled      bool
	httpClient   *http.Client

	mu      sync.Mutex
	pending []MemoryEntry
	wg      sync.WaitGroup
}

func NewMemoryAdapter(cfg MemoryAdapterConfig) *MemoryAdapter {
	interval := cfg.SyncInterval
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &MemoryAdapter{
		url:          cfg.HelixMemoryURL,
		syncInterval: interval,
		enabled:      cfg.Enabled,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Queue adds a memory entry to the pending buffer. Non-blocking.
func (ma *MemoryAdapter) Queue(entry MemoryEntry) {
	if !ma.enabled {
		return
	}
	ma.mu.Lock()
	defer ma.mu.Unlock()
	if len(ma.pending) >= maxPendingMemories {
		// Drop oldest entry
		ma.pending = ma.pending[1:]
	}
	ma.pending = append(ma.pending, entry)
}

// Start begins the background flush loop.
func (ma *MemoryAdapter) Start(ctx context.Context) {
	if !ma.enabled {
		return
	}
	ma.wg.Add(1)
	go func() {
		defer ma.wg.Done()
		ticker := time.NewTicker(ma.syncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ma.flush()
			}
		}
	}()
}

// Wait blocks until the flush loop exits.
func (ma *MemoryAdapter) Wait() {
	ma.wg.Wait()
}

func (ma *MemoryAdapter) flush() {
	ma.mu.Lock()
	if len(ma.pending) == 0 {
		ma.mu.Unlock()
		return
	}
	batch := make([]MemoryEntry, len(ma.pending))
	copy(batch, ma.pending)
	ma.mu.Unlock()

	body, err := json.Marshal(batch)
	if err != nil {
		slog.Warn("memory adapter: marshal error", "err", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, ma.url+"/v1/memory/entities", bytes.NewReader(body))
	if err != nil {
		slog.Warn("memory adapter: create request error", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ma.httpClient.Do(req)
	if err != nil {
		slog.Warn("memory adapter: flush failed (server unreachable)", "err", err)
		return // keep pending entries for next attempt
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success — clear flushed entries
		ma.mu.Lock()
		if len(ma.pending) >= len(batch) {
			ma.pending = ma.pending[len(batch):]
		} else {
			ma.pending = nil
		}
		ma.mu.Unlock()
		slog.Info("memory adapter: flushed entries", "count", len(batch))
	} else {
		slog.Warn("memory adapter: flush returned non-2xx", "status", resp.StatusCode)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd HelixLLM && GOMAXPROCS=2 go test -v -run TestMemoryAdapter ./internal/fallback/ -count=1`
Expected: 4 PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add internal/fallback/memory_adapter.go internal/fallback/memory_adapter_test.go
git commit -m "feat(fallback): add MemoryAdapter for HelixMemory persistent memory sync"
```

---

### Task 14: Gateway + main.go Wiring

**Files:**
- Modify: `internal/gateway/openai.go:34` — change `HandleChatCompletions` to accept a `Completer` interface
- Modify: `cmd/helixllm/main.go:251-316` — initialize FallbackChain and wire it

- [ ] **Step 1: Define Completer interface in gateway package**

Add to `internal/gateway/openai.go` (before `HandleChatCompletions`):

```go
// Completer abstracts both brain.Brain and fallback.Chain so the gateway
// handler works with either.
type Completer interface {
	Complete(ctx context.Context, req *types.InternalChatRequest) (*types.InternalChatResponse, error)
	CompleteStream(ctx context.Context, req *types.InternalChatRequest) (<-chan types.StreamChunk, error)
}
```

- [ ] **Step 2: Update HandleChatCompletions signature**

Change the first parameter of `HandleChatCompletions` from `b *brain.Brain` to `b Completer`:

```go
func HandleChatCompletions(b Completer, toolMgr *ToolManager, ragHook func(*types.InternalChatRequest) *types.InternalChatRequest) gin.HandlerFunc {
```

- [ ] **Step 3: Update RouterOptions to use Completer**

In the `RouterOptions` struct, change `Brain *brain.Brain` to `Brain Completer`:

```go
type RouterOptions struct {
	APIKeys     []string
	RateLimit   int
	Brain       Completer
	// ... rest of fields unchanged
}
```

- [ ] **Step 4: Verify gateway compiles**

Run: `cd HelixLLM && GOMAXPROCS=2 go build ./internal/gateway/`
Expected: No errors (Brain already satisfies Completer since it has Complete and CompleteStream)

- [ ] **Step 5: Wire FallbackChain in main.go**

In `cmd/helixllm/main.go`, after `brainSvc := brain.New(...)` (around line 263), add:

```go
	// Build the fallback chain — wraps Brain with scored multi-provider routing.
	scorerBridge := fallback.NewScorerBridge(fallback.ScorerBridgeConfig{
		VerifierURL:     cfg.LLM.VerifierURL,
		RefreshInterval: parseDuration(cfg.LLM.ScoreRefreshInterval, 5*time.Minute),
	})

	// Discover models from all providers.
	providerModels := discoverProviderModels(ctx, brainSvc)

	// Score and build initial chain entries.
	scores, _ := scorerBridge.FetchScores(ctx)
	entries := scorerBridge.BuildEntries(scores, providerModels)

	rateLimiter := fallback.NewRateLimitTracker(5, 1000)
	fallbackChain := fallback.NewChain(brainSvc.Providers(), rateLimiter)
	fallbackChain.SetEntries(entries)

	// Start background score refresh.
	scorerBridge.StartRefreshLoop(ctx, fallbackChain, providerModels)

	// Initialize memory adapter.
	memAdapter := fallback.NewMemoryAdapter(fallback.MemoryAdapterConfig{
		HelixMemoryURL: cfg.LLM.MemoryURL,
		SyncInterval:   30 * time.Second,
		Enabled:        cfg.LLM.MemorySyncEnabled,
	})
	memAdapter.Start(ctx)

	// Log the initial chain ordering.
	for i, e := range entries {
		slog.Info("fallback chain entry", "rank", i+1, "provider", e.ProviderName,
			"model", e.ModelID, "score", e.Score, "local", e.IsLocalFallback)
	}
```

Then change the `gateway.RegisterRoutes` call to pass `fallbackChain` instead of `brainSvc`:

```go
	gateway.RegisterRoutes(srv.Router(), gateway.RouterOptions{
		// ...
		Brain: fallbackChain, // was: brainSvc
		// ...
	})
```

- [ ] **Step 6: Add helper functions in main.go**

```go
func discoverProviderModels(ctx context.Context, b *brain.Brain) map[string]string {
	models := make(map[string]string)
	for name, p := range b.Providers() {
		modelList := p.Models()
		if len(modelList) > 0 {
			models[name] = modelList[0] // use first/best model
		}
	}
	return models
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}
```

- [ ] **Step 7: Add import for fallback package in main.go**

Add `"dev.helix.llm/internal/fallback"` to the import block.

- [ ] **Step 8: Verify full build**

Run: `cd HelixLLM && GOMAXPROCS=2 go build ./...`
Expected: No errors

- [ ] **Step 9: Commit**

```bash
cd HelixLLM && git add internal/gateway/openai.go cmd/helixllm/main.go
git commit -m "feat: wire FallbackChain between Gateway and Brain in main startup"
```

---

### Task 15: Update .env.example

**Files:**
- Modify: `.env.example`

- [ ] **Step 1: Add new env vars**

Append to the LLM section of `.env.example`:

```bash
# Free Cloud Providers (multi-provider fallback chain)
HELIX_LLM_CHUTES_KEY=
HELIX_LLM_OPENROUTER_KEY=
HELIX_LLM_HUGGINGFACE_KEY=
HELIX_LLM_NVIDIA_KEY=
HELIX_LLM_CEREBRAS_KEY=
HELIX_LLM_SAMBANOVA_KEY=
HELIX_LLM_TOGETHER_KEY=

# Fallback Chain Configuration
HELIX_LLM_VERIFIER_URL=http://localhost:7061
HELIX_LLM_SCORE_REFRESH_INTERVAL=5m

# HelixMemory Sync
HELIX_LLM_MEMORY_SYNC_ENABLED=false
HELIX_LLM_MEMORY_URL=http://localhost:7061
```

- [ ] **Step 2: Commit**

```bash
cd HelixLLM && git add .env.example
git commit -m "docs: add env vars for free cloud providers and fallback chain config"
```

---

### Task 16: Integration Tests

**Files:**
- Create: `tests/integration/fallback_chain_integration_test.go`
- Create: `tests/integration/scorer_bridge_integration_test.go`
- Create: `tests/integration/memory_sync_integration_test.go`

- [ ] **Step 1: Write fallback chain integration test**

```go
// tests/integration/fallback_chain_integration_test.go
package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/internal/brain"
	"dev.helix.llm/internal/fallback"
	"dev.helix.llm/pkg/types"
)

func TestFallbackChain_Integration_RealProviders(t *testing.T) {
	if os.Getenv("HELIX_INTEGRATION_TESTS") != "true" {
		t.Skip("set HELIX_INTEGRATION_TESTS=true to run")
	}

	cfg := brain.Config{
		ChutesKey:     os.Getenv("HELIX_LLM_CHUTES_KEY"),
		OpenRouterKey: os.Getenv("HELIX_LLM_OPENROUTER_KEY"),
		LlamaCppURL:   os.Getenv("HELIX_LLM_LOCAL_RPC_HOST"),
	}
	b := brain.New(cfg)

	sb := fallback.NewScorerBridge(fallback.ScorerBridgeConfig{})
	scores := sb.StaticFallbackScores()

	providerModels := make(map[string]string)
	for name, p := range b.Providers() {
		models := p.Models()
		if len(models) > 0 {
			providerModels[name] = models[0]
		}
	}

	entries := sb.BuildEntries(scores, providerModels)
	rl := fallback.NewRateLimitTracker(5, 1000)
	chain := fallback.NewChain(b.Providers(), rl)
	chain.SetEntries(entries)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := chain.Complete(ctx, &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "Say hello in one word."}},
		MaxTokens: 10,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Message.Content)
	t.Logf("Response from %s: %s", resp.Provider.Name, resp.Message.Content)
}
```

- [ ] **Step 2: Write scorer bridge integration test**

```go
// tests/integration/scorer_bridge_integration_test.go
package integration

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/internal/fallback"
)

func TestScorerBridge_Integration_LiveVerifier(t *testing.T) {
	verifierURL := os.Getenv("HELIX_LLM_VERIFIER_URL")
	if verifierURL == "" {
		t.Skip("set HELIX_LLM_VERIFIER_URL to run")
	}

	sb := fallback.NewScorerBridge(fallback.ScorerBridgeConfig{VerifierURL: verifierURL})
	scores, err := sb.FetchScores(context.Background())
	require.NoError(t, err)
	assert.Greater(t, len(scores), 0)
	t.Logf("Fetched %d provider scores from verifier", len(scores))
}
```

- [ ] **Step 3: Write memory sync integration test**

```go
// tests/integration/memory_sync_integration_test.go
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/internal/fallback"
)

func TestMemoryAdapter_Integration_RealEndpoint(t *testing.T) {
	memoryURL := os.Getenv("HELIX_LLM_MEMORY_URL")
	if memoryURL == "" {
		t.Skip("set HELIX_LLM_MEMORY_URL to run")
	}

	ma := fallback.NewMemoryAdapter(fallback.MemoryAdapterConfig{
		HelixMemoryURL: memoryURL,
		SyncInterval:   1 * time.Second,
		Enabled:        true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	ma.Start(ctx)

	ma.Queue(fallback.MemoryEntry{Content: "integration test memory", Type: "fact"})
	time.Sleep(2 * time.Second)
	cancel()
	ma.Wait()
}

func TestMemoryAdapter_Integration_MockEndpoint(t *testing.T) {
	var received []fallback.MemoryEntry
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var entries []fallback.MemoryEntry
		json.NewDecoder(r.Body).Decode(&entries)
		mu.Lock()
		received = append(received, entries...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ma := fallback.NewMemoryAdapter(fallback.MemoryAdapterConfig{
		HelixMemoryURL: srv.URL,
		SyncInterval:   100 * time.Millisecond,
		Enabled:        true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	ma.Start(ctx)
	ma.Queue(fallback.MemoryEntry{Content: "test", Type: "fact"})
	time.Sleep(200 * time.Millisecond)
	cancel()
	ma.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1)
	assert.Equal(t, "test", received[0].Content)
}
```

- [ ] **Step 4: Run unit tests to verify no regressions**

Run: `cd HelixLLM && GOMAXPROCS=2 go test ./internal/... -count=1 -p 1 -short`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add tests/integration/fallback_chain_integration_test.go \
  tests/integration/scorer_bridge_integration_test.go \
  tests/integration/memory_sync_integration_test.go
git commit -m "test: add integration tests for fallback chain, scorer bridge, memory sync"
```

---

### Task 17: E2E Tests

**Files:**
- Create: `tests/e2e/multi_provider_e2e_test.go`
- Create: `tests/e2e/rate_limit_rotation_e2e_test.go`

- [ ] **Step 1: Write multi-provider E2E test**

```go
// tests/e2e/multi_provider_e2e_test.go
package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiProvider_E2E_FullFlow(t *testing.T) {
	baseURL := os.Getenv("HELIX_LLM_URL")
	if baseURL == "" {
		t.Skip("set HELIX_LLM_URL to run E2E tests (e.g., http://localhost:8443)")
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model": "auto", // let the fallback chain decide
		"messages": []map[string]string{
			{"role": "user", "content": "What is 2+2? Answer with just the number."},
		},
		"max_tokens": 10,
	})

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	choices := result["choices"].([]interface{})
	require.NotEmpty(t, choices)
	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	assert.Contains(t, message["content"], "4")
}
```

- [ ] **Step 2: Write rate limit rotation E2E test**

```go
// tests/e2e/rate_limit_rotation_e2e_test.go
package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.llm/internal/brain"
	"dev.helix.llm/internal/fallback"
	"dev.helix.llm/pkg/types"
)

func TestRateLimitRotation_E2E(t *testing.T) {
	// Primary provider returns 429 on first request, then works.
	primaryCalls := 0
	primarySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls++
		if primaryCalls == 1 {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"rate_limit"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"p1","model":"m1","choices":[{"message":{"role":"assistant","content":"primary"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer primarySrv.Close()

	secondarySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"s1","model":"m2","choices":[{"message":{"role":"assistant","content":"secondary"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer secondarySrv.Close()

	providers := map[string]brain.Provider{
		"primary": brain.NewOpenAICompatProvider(brain.OpenAICompatConfig{
			Name: "primary", BaseURL: primarySrv.URL, APIKey: "k",
			AuthHeader: "Authorization", AuthPrefix: "Bearer ",
		}),
		"secondary": brain.NewOpenAICompatProvider(brain.OpenAICompatConfig{
			Name: "secondary", BaseURL: secondarySrv.URL, APIKey: "k",
			AuthHeader: "Authorization", AuthPrefix: "Bearer ",
		}),
	}

	rl := fallback.NewRateLimitTracker(5, 1000)
	chain := fallback.NewChain(providers, rl)
	chain.SetEntries([]fallback.ChainEntry{
		{ProviderName: "primary", ModelID: "m1", Score: 90, Status: fallback.EntryActive, CircuitBreaker: fallback.NewCircuitBreaker(3, 2*time.Minute)},
		{ProviderName: "secondary", ModelID: "m2", Score: 70, Status: fallback.EntryActive, CircuitBreaker: fallback.NewCircuitBreaker(3, 2*time.Minute)},
	})

	ctx := context.Background()
	req := &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "test"}},
	}

	// First request: primary returns 429, should fall to secondary
	resp, err := chain.Complete(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "secondary", resp.Message.Content)
}
```

- [ ] **Step 3: Run unit tests to verify no regressions**

Run: `cd HelixLLM && GOMAXPROCS=2 go test ./internal/... -count=1 -p 1 -short`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
cd HelixLLM && git add tests/e2e/multi_provider_e2e_test.go tests/e2e/rate_limit_rotation_e2e_test.go
git commit -m "test: add E2E tests for multi-provider flow and rate limit rotation"
```

---

### Task 18: Security, Stress, Benchmark Tests

**Files:**
- Create: `tests/security/fallback_security_test.go`
- Create: `tests/stress/fallback_stress_test.go`
- Create: `tests/benchmark/fallback_benchmark_test.go`

- [ ] **Step 1: Write security tests**

```go
// tests/security/fallback_security_test.go
package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"dev.helix.llm/internal/brain"
	"dev.helix.llm/internal/fallback"
	"dev.helix.llm/pkg/types"
)

func TestSecurity_APIKeyNotLeakedInErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	p := brain.NewOpenAICompatProvider(brain.OpenAICompatConfig{
		Name: "test", BaseURL: srv.URL, APIKey: "sk-secret-key-12345",
		AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	})

	_, err := p.Complete(context.Background(), &types.InternalChatRequest{
		Model: "m", Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "sk-secret-key-12345")
}

func TestSecurity_RateLimitHeaderInjection(t *testing.T) {
	rt := fallback.NewRateLimitTracker(5, 1000)
	h := http.Header{}
	h.Set("x-ratelimit-remaining-requests", "not-a-number; DROP TABLE users")
	rt.UpdateFromHeaders("attacker", h)
	assert.False(t, rt.ShouldSkip("attacker")) // malformed header = ignore
}
```

- [ ] **Step 2: Write stress test**

```go
// tests/stress/fallback_stress_test.go
package stress

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dev.helix.llm/internal/brain"
	"dev.helix.llm/internal/fallback"
	"dev.helix.llm/pkg/types"
)

func TestStress_100ConcurrentRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond) // simulate latency
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "s1", "model": "m1",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7},
		})
	}))
	defer srv.Close()

	providers := map[string]brain.Provider{
		"fast": brain.NewOpenAICompatProvider(brain.OpenAICompatConfig{
			Name: "fast", BaseURL: srv.URL, APIKey: "k",
			AuthHeader: "Authorization", AuthPrefix: "Bearer ",
		}),
	}

	chain := fallback.NewChain(providers, fallback.NewRateLimitTracker(5, 1000))
	chain.SetEntries([]fallback.ChainEntry{
		{ProviderName: "fast", ModelID: "m1", Score: 90, Status: fallback.EntryActive,
			CircuitBreaker: fallback.NewCircuitBreaker(3, 2*time.Minute)},
	})

	var success int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			resp, err := chain.Complete(ctx, &types.InternalChatRequest{
				Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
			})
			if err == nil && resp.Message.Content == "ok" {
				atomic.AddInt64(&success, 1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(100), success)
}
```

- [ ] **Step 3: Write benchmark test**

```go
// tests/benchmark/fallback_benchmark_test.go
package benchmark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.llm/internal/brain"
	"dev.helix.llm/internal/fallback"
	"dev.helix.llm/pkg/types"
)

func BenchmarkChain_Complete_SingleProvider(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "b1", "model": "m1",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7},
		})
	}))
	defer srv.Close()

	providers := map[string]brain.Provider{
		"fast": brain.NewOpenAICompatProvider(brain.OpenAICompatConfig{
			Name: "fast", BaseURL: srv.URL, APIKey: "k",
			AuthHeader: "Authorization", AuthPrefix: "Bearer ",
		}),
	}
	chain := fallback.NewChain(providers, fallback.NewRateLimitTracker(5, 1000))
	chain.SetEntries([]fallback.ChainEntry{
		{ProviderName: "fast", ModelID: "m1", Score: 90, Status: fallback.EntryActive,
			CircuitBreaker: fallback.NewCircuitBreaker(3, 2*time.Minute)},
	})

	req := &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.Complete(context.Background(), req)
	}
}

func BenchmarkChain_Complete_FallbackPath(b *testing.B) {
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limit"}`))
	}))
	defer failSrv.Close()

	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "b2", "model": "m2",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "ok"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7},
		})
	}))
	defer okSrv.Close()

	providers := map[string]brain.Provider{
		"fail": brain.NewOpenAICompatProvider(brain.OpenAICompatConfig{
			Name: "fail", BaseURL: failSrv.URL, APIKey: "k",
			AuthHeader: "Authorization", AuthPrefix: "Bearer ",
		}),
		"ok": brain.NewOpenAICompatProvider(brain.OpenAICompatConfig{
			Name: "ok", BaseURL: okSrv.URL, APIKey: "k",
			AuthHeader: "Authorization", AuthPrefix: "Bearer ",
		}),
	}
	chain := fallback.NewChain(providers, fallback.NewRateLimitTracker(5, 1000))

	req := &types.InternalChatRequest{
		Messages: []types.InternalMessage{{Role: "user", Content: "hi"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset entries each iteration since 429 marks exhausted
		chain.SetEntries([]fallback.ChainEntry{
			{ProviderName: "fail", ModelID: "m1", Score: 90, Status: fallback.EntryActive,
				CircuitBreaker: fallback.NewCircuitBreaker(3, 2*time.Minute)},
			{ProviderName: "ok", ModelID: "m2", Score: 70, Status: fallback.EntryActive,
				CircuitBreaker: fallback.NewCircuitBreaker(3, 2*time.Minute)},
		})
		chain.Complete(context.Background(), req)
	}
}
```

- [ ] **Step 4: Run all tests**

Run: `cd HelixLLM && GOMAXPROCS=2 go test ./internal/... ./tests/... -count=1 -p 1 -short`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
cd HelixLLM && git add tests/security/fallback_security_test.go tests/stress/fallback_stress_test.go tests/benchmark/fallback_benchmark_test.go
git commit -m "test: add security, stress, and benchmark tests for fallback chain"
```

---

### Task 19: Challenge Scripts

**Files:**
- Create: `challenges/scripts/multi_provider_fallback_challenge.sh`
- Create: `challenges/scripts/helixllm_memory_sync_challenge.sh`

- [ ] **Step 1: Write multi-provider fallback challenge**

```bash
#!/usr/bin/env bash
# challenges/scripts/multi_provider_fallback_challenge.sh
# Validates the multi-provider fallback chain in HelixLLM.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PASS=0
FAIL=0
TOTAL=0

pass() { ((PASS++)); ((TOTAL++)); echo "  PASS: $1"; }
fail() { ((FAIL++)); ((TOTAL++)); echo "  FAIL: $1"; }
check() { if eval "$2" >/dev/null 2>&1; then pass "$1"; else fail "$1"; fi; }

echo "=== Multi-Provider Fallback Chain Challenge ==="

# 1. Verify new provider files exist
echo "--- Provider Files ---"
for provider in chutes openrouter huggingface nvidia cerebras sambanova together; do
    check "Provider file: ${provider}_provider.go" \
        "test -f '$PROJECT_ROOT/HelixLLM/internal/brain/${provider}_provider.go'"
    check "Provider test: ${provider}_provider_test.go" \
        "test -f '$PROJECT_ROOT/HelixLLM/internal/brain/${provider}_provider_test.go'"
done

# 2. Verify shared base exists
echo "--- Shared Base ---"
check "OpenAICompatProvider base exists" \
    "test -f '$PROJECT_ROOT/HelixLLM/internal/brain/openai_compat_provider.go'"
check "OpenAICompatProvider tests exist" \
    "test -f '$PROJECT_ROOT/HelixLLM/internal/brain/openai_compat_provider_test.go'"

# 3. Verify fallback package
echo "--- Fallback Package ---"
for file in chain.go chain_entry.go rate_limit.go circuit_breaker.go scorer_bridge.go memory_adapter.go; do
    check "Fallback file: $file" \
        "test -f '$PROJECT_ROOT/HelixLLM/internal/fallback/$file'"
done

# 4. Verify env vars in config
echo "--- Config ---"
check "ChutesKey in config" \
    "grep -q 'HELIX_LLM_CHUTES_KEY' '$PROJECT_ROOT/HelixLLM/internal/shared/config/config.go'"
check "OpenRouterKey in config" \
    "grep -q 'HELIX_LLM_OPENROUTER_KEY' '$PROJECT_ROOT/HelixLLM/internal/shared/config/config.go'"

# 5. Verify Brain registers new providers
echo "--- Brain Registration ---"
check "Brain registers chutes" \
    "grep -q 'ChutesKey' '$PROJECT_ROOT/HelixLLM/internal/brain/brain.go'"
check "Brain registers openrouter" \
    "grep -q 'OpenRouterKey' '$PROJECT_ROOT/HelixLLM/internal/brain/brain.go'"

# 6. Verify gateway uses Completer interface
echo "--- Gateway Wiring ---"
check "Gateway uses Completer interface" \
    "grep -q 'Completer' '$PROJECT_ROOT/HelixLLM/internal/gateway/openai.go'"

# 7. Verify FallbackChain in main.go
echo "--- Main Wiring ---"
check "main.go imports fallback package" \
    "grep -q 'fallback' '$PROJECT_ROOT/HelixLLM/cmd/helixllm/main.go'"
check "main.go creates FallbackChain" \
    "grep -q 'NewChain' '$PROJECT_ROOT/HelixLLM/cmd/helixllm/main.go'"

# 8. Run unit tests
echo "--- Unit Tests ---"
cd "$PROJECT_ROOT/HelixLLM"
if GOMAXPROCS=2 nice -n 19 go test ./internal/brain/ ./internal/fallback/ -count=1 -p 1 -short -timeout 120s; then
    pass "Unit tests pass"
else
    fail "Unit tests failed"
fi

# 9. Verify build
echo "--- Build ---"
if GOMAXPROCS=2 nice -n 19 go build ./...; then
    pass "Build succeeds"
else
    fail "Build failed"
fi

echo ""
echo "=== Results: $PASS/$TOTAL passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
```

- [ ] **Step 2: Write memory sync challenge**

```bash
#!/usr/bin/env bash
# challenges/scripts/helixllm_memory_sync_challenge.sh
# Validates the HelixMemory sync adapter in HelixLLM.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PASS=0
FAIL=0
TOTAL=0

pass() { ((PASS++)); ((TOTAL++)); echo "  PASS: $1"; }
fail() { ((FAIL++)); ((TOTAL++)); echo "  FAIL: $1"; }
check() { if eval "$2" >/dev/null 2>&1; then pass "$1"; else fail "$1"; fi; }

echo "=== HelixLLM Memory Sync Challenge ==="

# 1. Verify adapter files
echo "--- Memory Adapter Files ---"
check "memory_adapter.go exists" \
    "test -f '$PROJECT_ROOT/HelixLLM/internal/fallback/memory_adapter.go'"
check "memory_adapter_test.go exists" \
    "test -f '$PROJECT_ROOT/HelixLLM/internal/fallback/memory_adapter_test.go'"

# 2. Verify config
echo "--- Config ---"
check "HELIX_LLM_MEMORY_SYNC_ENABLED in config" \
    "grep -q 'HELIX_LLM_MEMORY_SYNC_ENABLED' '$PROJECT_ROOT/HelixLLM/internal/shared/config/config.go'"
check "HELIX_LLM_MEMORY_URL in config" \
    "grep -q 'HELIX_LLM_MEMORY_URL' '$PROJECT_ROOT/HelixLLM/internal/shared/config/config.go'"

# 3. Verify buffer cap
check "Buffer cap at 1000" \
    "grep -q 'maxPendingMemories.*=.*1000' '$PROJECT_ROOT/HelixLLM/internal/fallback/memory_adapter.go'"

# 4. Run unit tests
echo "--- Unit Tests ---"
cd "$PROJECT_ROOT/HelixLLM"
if GOMAXPROCS=2 nice -n 19 go test -v -run TestMemoryAdapter ./internal/fallback/ -count=1 -timeout 60s; then
    pass "Memory adapter unit tests pass"
else
    fail "Memory adapter unit tests failed"
fi

echo ""
echo "=== Results: $PASS/$TOTAL passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
```

- [ ] **Step 3: Make scripts executable and commit**

```bash
cd HelixLLM && chmod +x challenges/scripts/multi_provider_fallback_challenge.sh challenges/scripts/helixllm_memory_sync_challenge.sh
git add challenges/scripts/multi_provider_fallback_challenge.sh challenges/scripts/helixllm_memory_sync_challenge.sh
git commit -m "test: add challenge scripts for multi-provider fallback and memory sync"
```

---

### Task 20: Final Verification and Documentation

**Files:**
- Modify: `HelixLLM/README.md` — update architecture section
- Run: full test suite

- [ ] **Step 1: Run full build**

Run: `cd HelixLLM && GOMAXPROCS=2 nice -n 19 go build ./...`
Expected: No errors

- [ ] **Step 2: Run all unit tests**

Run: `cd HelixLLM && GOMAXPROCS=2 nice -n 19 go test ./internal/... -count=1 -p 1 -short -timeout 120s`
Expected: All PASS

- [ ] **Step 3: Run challenge scripts**

Run: `GOMAXPROCS=2 nice -n 19 ./HelixLLM/challenges/scripts/multi_provider_fallback_challenge.sh`
Expected: All checks pass

Run: `GOMAXPROCS=2 nice -n 19 ./HelixLLM/challenges/scripts/helixllm_memory_sync_challenge.sh`
Expected: All checks pass

- [ ] **Step 4: Run race detector on fallback package**

Run: `cd HelixLLM && GOMAXPROCS=2 nice -n 19 go test -race ./internal/fallback/ -count=1 -p 1 -timeout 120s`
Expected: No race conditions detected

- [ ] **Step 5: Update HelixLLM README**

Add a "Multi-Provider Fallback Chain" section to `HelixLLM/README.md`:

```markdown
## Multi-Provider Fallback Chain

HelixLLM routes requests through a scored chain of free cloud providers:

1. **Auto-discovery**: Discovers available models from all configured providers (Chutes, OpenRouter, HuggingFace, Nvidia, Cerebras, SambaNova, Together)
2. **Scoring**: Ranks providers using LLMsVerifier scores (refreshed every 5 minutes)
3. **Fallback**: On rate limit (429) or server error (5xx), automatically rotates to next provider
4. **Local fallback**: llama.cpp is always the last resort, guaranteed to be available
5. **Rate limit tracking**: Parses response headers to proactively skip providers approaching limits

### Configuration

Set API keys for providers in `.env`:

```env
HELIX_LLM_CHUTES_KEY=your-key
HELIX_LLM_OPENROUTER_KEY=your-key
HELIX_LLM_HUGGINGFACE_KEY=your-key
HELIX_LLM_NVIDIA_KEY=your-key
HELIX_LLM_CEREBRAS_KEY=your-key
HELIX_LLM_SAMBANOVA_KEY=your-key
HELIX_LLM_TOGETHER_KEY=your-key
```

The chain automatically discovers and ranks available models. No manual model configuration needed.
```

- [ ] **Step 6: Commit**

```bash
cd HelixLLM && git add README.md
git commit -m "docs: update README with multi-provider fallback chain documentation"
```

- [ ] **Step 7: Final commit in main repo**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
git add HelixLLM
git commit -m "chore: update HelixLLM — multi-provider fallback chain with 7 free cloud providers"
```
