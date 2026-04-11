package helixllm

// Phase-2 unit tests for the HelixLLM adapter.
// These are hermetic — no real HelixLLM service required. A local httptest
// server stands in for the HelixLLM HTTP surface. Each test pins one
// invariant of the adapter's request construction, response parsing, or
// disabled-mode error handling.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAdapter(t *testing.T, endpoint, apiKey string) *Adapter {
	t.Helper()
	a, err := NewAdapter(Config{
		Endpoint:      endpoint,
		APIKey:        apiKey,
		Enabled:       true,
		TLSSkipVerify: true,
		Timeout:       2 * time.Second,
	})
	require.NoError(t, err)
	return a
}

func TestNewAdapter_DefaultsAndOverrides(t *testing.T) {
	a, err := NewAdapter(Config{Enabled: true})
	require.NoError(t, err)
	assert.NotEmpty(t, a.endpoint, "endpoint must resolve to a non-empty value")
	assert.True(t, a.enabled)
	assert.NotNil(t, a.httpClient)
	assert.Equal(t, defaultTimeout, a.httpClient.Timeout, "timeout defaults when unset")

	a2, err := NewAdapter(Config{
		Endpoint: "https://probe:9000",
		APIKey:   "key-x",
		Enabled:  true,
		Timeout:  7 * time.Second,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://probe:9000", a2.endpoint)
	assert.Equal(t, "key-x", a2.apiKey)
	assert.Equal(t, 7*time.Second, a2.httpClient.Timeout)
}

func TestAdapter_DisabledMode_AllCallsReturnError(t *testing.T) {
	a := &Adapter{enabled: false}
	ctx := context.Background()

	// Every call on a disabled adapter must return an error without
	// attempting any HTTP or container work.
	_, err := a.Health(ctx)
	assert.Error(t, err)
	_, err = a.ChatCompletion(ctx, &ChatCompletionRequest{})
	assert.Error(t, err)
	_, err = a.Embed(ctx, &EmbeddingRequest{})
	assert.Error(t, err)
	_, err = a.KnowledgeIngest(ctx, &KnowledgeIngestRequest{})
	assert.Error(t, err)
	_, err = a.KnowledgeQuery(ctx, &KnowledgeQueryRequest{})
	assert.Error(t, err)
	_, err = a.AgentChat(ctx, &AgentChatRequest{})
	assert.Error(t, err)
	_, err = a.GetModels(ctx)
	assert.Error(t, err)

	// Start/Stop in disabled mode must be no-ops (no error, no container call).
	assert.NoError(t, a.Start(ctx))
	assert.NoError(t, a.Stop(ctx))
	assert.False(t, a.IsEnabled())
}

func TestAdapter_Health_SuccessAndError(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		assert.Equal(t, "/internal/health", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:  "ok",
			Version: "1.0.0",
			Mode:    "local",
		})
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL, "abc-123")
	health, err := a.Health(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", health.Status)
	assert.Equal(t, "Bearer abc-123", capturedAuth,
		"adapter must forward API key as bearer when set")

	// Error path: server returns non-200
	srvErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srvErr.Close()

	a2 := newTestAdapter(t, srvErr.URL, "")
	_, err = a2.Health(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestAdapter_ChatCompletion_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		var got ChatCompletionRequest
		require.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, "qwen-1.5b", got.Model)
		require.Len(t, got.Messages, 1)
		assert.Equal(t, "hello", got.Messages[0].Content)

		_ = json.NewEncoder(w).Encode(ChatCompletionResponse{
			ID:    "resp-1",
			Model: got.Model,
			Choices: []Choice{{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "hi back",
				},
			}},
		})
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL, "")
	resp, err := a.ChatCompletion(context.Background(), &ChatCompletionRequest{
		Model: "qwen-1.5b",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "hi back", resp.Choices[0].Message.Content)
}

func TestAdapter_ChatCompletion_ServerError_IncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad model"}`))
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL, "")
	_, err := a.ChatCompletion(context.Background(), &ChatCompletionRequest{
		Model:    "nope",
		Messages: []Message{{Role: "user", Content: "x"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "bad model",
		"error message must include server-side body for debugging")
}

func TestAdapter_Embed_And_Knowledge_And_Agent(t *testing.T) {
	// Single server that routes by path to cover the remaining four endpoints
	// in one test for brevity. Each branch pins method, path, and response
	// decoding against the real types in types.go.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/embeddings":
			_ = json.NewEncoder(w).Encode(EmbeddingResponse{
				Object: "list",
				Model:  "nomic-embed",
				Data: []EmbeddingData{{
					Object:    "embedding",
					Index:     0,
					Embedding: []float64{0.1, 0.2, 0.3},
				}},
			})
		case "/internal/knowledge/ingest":
			_ = json.NewEncoder(w).Encode(KnowledgeIngestResponse{
				ID:         "ing-1",
				Collection: "default",
				ChunkCount: 4,
				Status:     "ok",
				DurationMs: 12,
			})
		case "/internal/knowledge/query":
			_ = json.NewEncoder(w).Encode(KnowledgeQueryResponse{
				Query: "probe",
				TopK:  1,
				Results: []KnowledgeResult{{
					Content:    "relevant doc",
					Score:      0.99,
					Collection: "default",
				}},
			})
		case "/v1/agents/chat":
			_ = json.NewEncoder(w).Encode(AgentChatResponse{
				SessionID: "sess-x",
				Response:  "pong",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL, "")
	ctx := context.Background()

	embed, err := a.Embed(ctx, &EmbeddingRequest{
		Model: "nomic-embed",
		Input: []string{"probe"},
	})
	require.NoError(t, err)
	require.Len(t, embed.Data, 1)
	assert.InDelta(t, 0.2, embed.Data[0].Embedding[1], 0.0001)

	ing, err := a.KnowledgeIngest(ctx, &KnowledgeIngestRequest{
		Collection: "default",
		Documents:  []string{"hello world"},
	})
	require.NoError(t, err)
	assert.Equal(t, 4, ing.ChunkCount)
	assert.Equal(t, "ok", ing.Status)

	q, err := a.KnowledgeQuery(ctx, &KnowledgeQueryRequest{
		Collection: "default",
		Query:      "probe",
		TopK:       1,
	})
	require.NoError(t, err)
	require.Len(t, q.Results, 1)
	assert.Equal(t, "relevant doc", q.Results[0].Content)

	ac, err := a.AgentChat(ctx, &AgentChatRequest{
		SessionID: "sess-x",
		Messages:  []Message{{Role: "user", Content: "ping"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "pong", ac.Response)
	assert.Equal(t, "sess-x", ac.SessionID)
}

func TestAdapter_GetModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_ = json.NewEncoder(w).Encode(ModelsResponse{
			Object: "list",
			Data: []ModelInfo{
				{ID: "qwen-1.5b", Object: "model"},
				{ID: "qwen-3b", Object: "model"},
			},
		})
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL, "")
	models, err := a.GetModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models.Data, 2)
	assert.Equal(t, "qwen-1.5b", models.Data[0].ID)
}

func TestAdapter_ContextCancellation(t *testing.T) {
	// Server hangs forever — adapter must honor caller's ctx cancellation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := a.Health(ctx)
	elapsed := time.Since(start)
	require.Error(t, err)
	// Must return shortly after ctx fired, not after the 2s httpClient timeout.
	assert.Less(t, elapsed, 1*time.Second,
		"adapter must abort on ctx.Done, not wait for transport timeout")
}

func TestAdapter_Start_Stop_WithoutContainers(t *testing.T) {
	a := &Adapter{enabled: true, containers: nil}
	err := a.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "containers adapter not configured")

	err = a.Stop(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "containers adapter not configured")

	_, err = a.Status(context.Background())
	require.Error(t, err)
}

func TestGetEnv_Helpers(t *testing.T) {
	t.Setenv("PHASE2_TEST_VAR", "")
	assert.Equal(t, "fallback", getEnv("PHASE2_TEST_VAR", "fallback"))
	t.Setenv("PHASE2_TEST_VAR", "real")
	assert.Equal(t, "real", getEnv("PHASE2_TEST_VAR", "fallback"))

	t.Setenv("PHASE2_BOOL_VAR", "")
	assert.True(t, getEnvBool("PHASE2_BOOL_VAR", true))
	assert.False(t, getEnvBool("PHASE2_BOOL_VAR", false))
	for _, v := range []string{"true", "1", "yes"} {
		t.Setenv("PHASE2_BOOL_VAR", v)
		assert.True(t, getEnvBool("PHASE2_BOOL_VAR", false), "value %q should parse true", v)
	}
	t.Setenv("PHASE2_BOOL_VAR", "no")
	assert.False(t, getEnvBool("PHASE2_BOOL_VAR", true))
}

// Malformed JSON must error cleanly (not panic) — exercises the
// defer resp.Body.Close() path and the json.Decoder error branch.
func TestAdapter_MalformedJSON_ErrorsButDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(w, strings.NewReader("{not json"))
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL, "")
	_, err := a.Health(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}
