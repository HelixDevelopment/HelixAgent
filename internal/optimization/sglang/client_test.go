package sglang

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

func TestNewClient(t *testing.T) {
	config := &ClientConfig{
		BaseURL: "http://localhost:30000",
		Timeout: 30 * time.Second,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:30000", client.baseURL)
}

func TestNewClient_DefaultConfig(t *testing.T) {
	client := NewClient(nil)
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:30000", client.baseURL)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "http://localhost:30000", config.BaseURL)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestClient_Complete(t *testing.T) {
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
		json.NewEncoder(w).Encode(resp)
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
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	result, err := client.CompleteSimple(context.Background(), "Hello")

	require.NoError(t, err)
	assert.Equal(t, "Simple response", result)
}

func TestClient_CompleteWithSystem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CompletionRequest
		json.NewDecoder(r.Body).Decode(&req)

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
		json.NewEncoder(w).Encode(resp)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WarmPrefix makes a completion request
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: ""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	_, err := client.GetSession(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestClient_ContinueSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: "I can help you with that!"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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
	client := NewClient(&ClientConfig{BaseURL: "http://localhost", Timeout: 5 * time.Second})

	client.sessions["to-delete"] = &Session{ID: "to-delete"}

	err := client.DeleteSession(context.Background(), "to-delete")

	require.NoError(t, err)
	_, err = client.GetSession(context.Background(), "to-delete")
	assert.Error(t, err)
}

func TestClient_WarmPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: ""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.WarmPrefix(context.Background(), "System: You are a helpful assistant.")

	require.NoError(t, err)
	assert.True(t, resp.Cached)
}

func TestClient_WarmPrefixes(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := &CompletionResponse{
			Choices: []CompletionChoice{
				{Message: Message{Role: "assistant", Content: ""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	err := client.WarmPrefixes(context.Background(), []string{
		"Prefix 1",
		"Prefix 2",
		"Prefix 3",
	})

	require.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestClient_CleanupSessions(t *testing.T) {
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		resp := &HealthResponse{
			Status:    "healthy",
			Model:     "llama-2-7b",
			GPUMemory: "8GB",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	health, err := client.Health(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestClient_IsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &HealthResponse{Status: "healthy"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	available := client.IsAvailable(context.Background())
	assert.True(t, available)
}

func TestClient_IsAvailable_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	available := client.IsAvailable(context.Background())
	assert.False(t, available)
}

func TestClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	assert.Error(t, err)
}

func TestClient_Timeout(t *testing.T) {
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
