package sglang

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()
	config := &ClientConfig{
		BaseURL: "http://localhost:30000",
		Timeout: 30 * time.Second,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:30000", client.baseURL)
}

func TestNewClient_DefaultConfig(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:30000", client.baseURL)
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "http://localhost:30000", config.BaseURL)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestClient_Complete(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var req CompletionRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.NotEmpty(t, req.Messages)

		resp := &CompletionResponse{
			ID:      "cmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []CompletionChoice{
				{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "Hello! How can I help you today?",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 15,
				TotalTokens:      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	resp, err := client.Complete(context.Background(), &CompletionRequest{
		Model: "llama-2-7b",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "assistant", resp.Choices[0].Message.Role)
	assert.Contains(t, resp.Choices[0].Message.Content, "Hello")
}

func TestClient_CompleteSimple(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			ID: "cmpl-simple",
			Choices: []CompletionChoice{
				{
					Message: Message{Role: "assistant", Content: "Simple response"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	result, err := client.CompleteSimple(context.Background(), "Hello")

	require.NoError(t, err)
	assert.Equal(t, "Simple response", result)
}

func TestClient_CompleteWithSystem(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify system message is included
		assert.Equal(t, 2, len(req.Messages))
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "user", req.Messages[1].Role)

		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "Response with system prompt"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	result, err := client.CompleteWithSystem(context.Background(),
		"You are a helpful assistant",
		"Hello")

	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestClient_CreateSession(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WarmPrefix makes a completion request
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: ""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	session, err := client.CreateSession(context.Background(), "session-123", "You are a helpful assistant")

	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "session-123", session.ID)
	assert.Equal(t, "You are a helpful assistant", session.SystemPrompt)
}

func TestClient_GetSession(t *testing.T) {
	t.Parallel()
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	// Create a session first
	client.sessions["test-session"] = &Session{
		ID:           "test-session",
		SystemPrompt: "Test prompt",
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
	}

	session, err := client.GetSession(context.Background(), "test-session")

	require.NoError(t, err)
	assert.Equal(t, "test-session", session.ID)
}

func TestClient_GetSession_NotFound(t *testing.T) {
	t.Parallel()
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	_, err := client.GetSession(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestClient_ContinueSession(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "I can help you with that!"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	// Create session first
	client.sessions["session-123"] = &Session{
		ID:           "session-123",
		SystemPrompt: "You are helpful",
		History:      []Message{},
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
	}

	response, err := client.ContinueSession(context.Background(), "session-123", "Help me with coding")

	require.NoError(t, err)
	assert.Contains(t, response, "help")

	// Verify history was updated
	session, _ := client.GetSession(context.Background(), "session-123")
	assert.Len(t, session.History, 2)
}

func TestClient_DeleteSession(t *testing.T) {
	t.Parallel()
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	client.sessions["to-delete"] = &Session{ID: "to-delete"}

	err := client.DeleteSession(context.Background(), "to-delete")

	require.NoError(t, err)
	_, err = client.GetSession(context.Background(), "to-delete")
	assert.Error(t, err)
}

func TestClient_WarmPrefix(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: ""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.WarmPrefix(context.Background(), "System: You are a helpful assistant.")

	require.NoError(t, err)
	assert.True(t, resp.Cached)
}

func TestClient_WarmPrefixes(t *testing.T) {
	t.Parallel()
	var callCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: ""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	err := client.WarmPrefixes(context.Background(), []string{
		"Prefix 1",
		"Prefix 2",
		"Prefix 3",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(3), callCount.Load())
}

func TestClient_CleanupSessions(t *testing.T) {
	t.Parallel()
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	// Add stale and fresh sessions
	oldTime := time.Now().Add(-2 * time.Hour)
	client.sessions["stale"] = &Session{ID: "stale", LastUsedAt: oldTime}
	client.sessions["fresh"] = &Session{ID: "fresh", LastUsedAt: time.Now()}

	removed := client.CleanupSessions(context.Background(), 1*time.Hour)

	assert.Equal(t, 1, removed)
	_, err := client.GetSession(context.Background(), "stale")
	assert.Error(t, err)
	_, err = client.GetSession(context.Background(), "fresh")
	assert.NoError(t, err)
}

func TestClient_Health(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		resp := &HealthResponse{
			Status:    "healthy",
			Model:     "llama-2-7b",
			GPUMemory: "8GB",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	health, err := client.Health(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestClient_IsAvailable(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &HealthResponse{Status: "healthy"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	available := client.IsAvailable(context.Background())
	assert.True(t, available)
}

func TestClient_IsAvailable_Unhealthy(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	available := client.IsAvailable(context.Background())
	assert.False(t, available)
}

func TestClient_ErrorHandling(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	assert.Error(t, err)
}

func TestClient_Timeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 50 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Complete(ctx, &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	assert.Error(t, err)
}

// TestClient_SessionHistory tests session history management
func TestClient_SessionHistory(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "Response " + string(rune(len(req.Messages)))}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	// Create session
	client.sessions["history-test"] = &Session{
		ID:           "history-test",
		SystemPrompt: "You are helpful",
		History:      []Message{},
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
	}

	// Multiple interactions
	for i := 0; i < 5; i++ {
		_, err := client.ContinueSession(context.Background(), "history-test", fmt.Sprintf("Message %d", i))
		require.NoError(t, err)
	}

	session, _ := client.GetSession(context.Background(), "history-test")
	assert.Equal(t, 10, len(session.History)) // 5 user messages + 5 assistant responses
}

// TestClient_ConcurrentSessions tests concurrent session access
func TestClient_ConcurrentSessions(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "Concurrent response"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// Create multiple sessions
	for i := 0; i < 10; i++ {
		_, _ = client.CreateSession(ctx, fmt.Sprintf("session-%d", i), "System prompt")
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent session operations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", idx%10)
			_, err := client.ContinueSession(ctx, sessionID, "Hello")
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	var errCount int
	for range errors {
		errCount++
	}
	assert.Equal(t, 0, errCount)
}

// TestClient_CompleteWithOptions tests completion with various options
func TestClient_CompleteWithOptions(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify request options
		assert.NotZero(t, req.Temperature)
		assert.NotZero(t, req.MaxTokens)
		assert.NotEmpty(t, req.Model)

		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "Response with options"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Complete(context.Background(), &CompletionRequest{
		Model:       "test-model",
		Messages:    []Message{{Role: "user", Content: "test"}},
		Temperature: 0.9,
		MaxTokens:   100,
		TopP:        0.95,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Choices)
}

// TestClient_ListSessions tests session listing
func TestClient_ListSessions(t *testing.T) {
	t.Parallel()
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	// Create sessions
	for i := 0; i < 5; i++ {
		client.sessions[fmt.Sprintf("session-%d", i)] = &Session{
			ID:         fmt.Sprintf("session-%d", i),
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
		}
	}

	ctx := context.Background()
	sessions := client.ListSessions(ctx)
	assert.Len(t, sessions, 5)
}

// TestClient_SessionExpiry tests session cleanup based on age
func TestClient_SessionExpiry(t *testing.T) {
	t.Parallel()
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	// Create old sessions
	oldTime := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 3; i++ {
		client.sessions[fmt.Sprintf("old-%d", i)] = &Session{
			ID:         fmt.Sprintf("old-%d", i),
			LastUsedAt: oldTime,
		}
	}

	// Create recent sessions
	for i := 0; i < 2; i++ {
		client.sessions[fmt.Sprintf("new-%d", i)] = &Session{
			ID:         fmt.Sprintf("new-%d", i),
			LastUsedAt: time.Now(),
		}
	}

	// Cleanup sessions older than 1 hour
	removed := client.CleanupSessions(context.Background(), 1*time.Hour)
	assert.Equal(t, 3, removed)
	assert.Equal(t, 2, len(client.sessions))
}

// TestClient_WarmPrefixWithLongContent tests prefix warming with long content
func TestClient_WarmPrefixWithLongContent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify long prefix was sent
		assert.True(t, len(req.Messages[0].Content) > 100)

		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: ""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	longPrefix := strings.Repeat("This is a long system prompt that provides extensive context. ", 20)
	resp, err := client.WarmPrefix(context.Background(), longPrefix)

	require.NoError(t, err)
	assert.True(t, resp.Cached)
}

// TestClient_RetryOnNetworkError tests retry behavior on network errors
func TestClient_RetryOnNetworkError(t *testing.T) {
	t.Parallel()
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			// Simulate temporary error
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "Success after retry"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	// This will fail since we don't have built-in retry
	_, err := client.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	assert.Error(t, err)
	assert.Equal(t, 1, callCount)
}

// TestClient_EmptyResponse tests handling of empty responses
func TestClient_EmptyResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			Choices: []CompletionChoice{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	result, err := client.CompleteSimple(context.Background(), "test")
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestClient_ContinueSession_NotFound tests continuing non-existent session
func TestClient_ContinueSession_NotFound(t *testing.T) {
	t.Parallel()
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	_, err := client.ContinueSession(context.Background(), "nonexistent", "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

// TestClient_DeleteSession_NotFound tests deleting non-existent session
func TestClient_DeleteSession_NotFound(t *testing.T) {
	t.Parallel()
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	err := client.DeleteSession(context.Background(), "nonexistent")
	assert.Error(t, err)
}

// TestClient_Health_MalformedResponse tests handling of malformed health response
func TestClient_Health_MalformedResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.Health(context.Background())
	assert.Error(t, err)
}

// BenchmarkClient_Complete benchmarks completion requests
func BenchmarkClient_Complete(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "Response"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Complete(ctx, &CompletionRequest{
			Messages: []Message{{Role: "user", Content: "test"}},
		})
	}
}

// BenchmarkClient_SessionContinue benchmarks session continuation
func BenchmarkClient_SessionContinue(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "Response"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	client.sessions["bench-session"] = &Session{
		ID:           "bench-session",
		SystemPrompt: "You are helpful",
		History:      []Message{},
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ContinueSession(ctx, "bench-session", "Hello")
	}
}
