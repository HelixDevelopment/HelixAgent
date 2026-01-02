package langchain

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
		BaseURL: "http://localhost:8011",
		Timeout: 30 * time.Second,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8011", client.baseURL)
}

func TestNewClient_DefaultConfig(t *testing.T) {
	client := NewClient(nil)
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8011", client.baseURL)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "http://localhost:8011", config.BaseURL)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestClient_Decompose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/decompose", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var req DecomposeRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.NotEmpty(t, req.Task)

		resp := &DecomposeResponse{
			Subtasks: []Subtask{
				{ID: 1, Description: "Setup project structure", Dependencies: []int{}, Complexity: "low"},
				{ID: 2, Description: "Implement database layer", Dependencies: []int{1}, Complexity: "medium"},
				{ID: 3, Description: "Create API endpoints", Dependencies: []int{1, 2}, Complexity: "high"},
			},
			Reasoning: "Task broken down into logical steps with dependency tracking",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	resp, err := client.Decompose(context.Background(), &DecomposeRequest{
		Task:     "Build a REST API with database integration",
		MaxSteps: 5,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Subtasks, 3)
	assert.Equal(t, "Setup project structure", resp.Subtasks[0].Description)
	assert.Equal(t, "low", resp.Subtasks[0].Complexity)
}

func TestClient_ExecuteChain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chain", r.URL.Path)

		var req ChainRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := &ChainResponse{
			Result: "This is a summarized version of the input text.",
			Steps: []ChainStep{
				{Step: "parse", Input: req.Prompt, Output: "parsed"},
				{Step: "summarize", Input: "parsed", Output: "summarized"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.ExecuteChain(context.Background(), &ChainRequest{
		ChainType: "summarize",
		Prompt:    "Long document text here...",
		Variables: map[string]interface{}{"max_length": 100},
	})

	require.NoError(t, err)
	assert.Contains(t, resp.Result, "summarized")
	assert.Len(t, resp.Steps, 2)
}

func TestClient_RunReActAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/react", r.URL.Path)

		resp := &ReActResponse{
			Answer: "The weather in New York is currently 72°F with partly cloudy skies.",
			ReasoningTrace: []ReActStep{
				{
					Iteration:   1,
					Thought:     "I need to check the weather in New York",
					Action:      "weather",
					ActionInput: "New York",
					Observation: "Temperature: 72°F, Conditions: Partly cloudy",
				},
			},
			ToolsUsed:  []string{"weather"},
			Iterations: 1,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.RunReActAgent(context.Background(), &ReActRequest{
		Goal:           "Find the current weather in New York",
		AvailableTools: []string{"weather", "search"},
		MaxIterations:  5,
	})

	require.NoError(t, err)
	assert.Contains(t, resp.Answer, "72°F")
	assert.Len(t, resp.ReasoningTrace, 1)
	assert.Equal(t, "weather", resp.ReasoningTrace[0].Action)
}

func TestClient_Summarize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/summarize", r.URL.Path)

		resp := struct {
			Summary string `json:"summary"`
		}{
			Summary: "Key points from the document.",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	summary, err := client.Summarize(context.Background(), "A very long document with many paragraphs...", 100)

	require.NoError(t, err)
	assert.NotEmpty(t, summary)
	assert.Equal(t, "Key points from the document.", summary)
}

func TestClient_Transform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/transform", r.URL.Path)

		resp := struct {
			Result string `json:"result"`
		}{
			Result: "This is a formal business communication.",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	result, err := client.Transform(context.Background(), "hey whats up dude", "Convert to formal business language")

	require.NoError(t, err)
	assert.Contains(t, result, "formal")
}

func TestClient_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		resp := &HealthResponse{
			Status:       "healthy",
			Version:      "1.0.0",
			LLMAvailable: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	health, err := client.Health(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.True(t, health.LLMAvailable)
}

func TestClient_IsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			resp := &HealthResponse{Status: "healthy"}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
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

	_, err := client.Decompose(context.Background(), &DecomposeRequest{Task: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
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

	_, err := client.Decompose(ctx, &DecomposeRequest{Task: "test"})
	assert.Error(t, err)
}

func TestClient_Decompose_WithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DecomposeRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "production web application", req.Context)

		resp := &DecomposeResponse{
			Subtasks:  []Subtask{{ID: 1, Description: "Task with context"}},
			Reasoning: "Applied context",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Decompose(context.Background(), &DecomposeRequest{
		Task:    "Build API",
		Context: "production web application",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Subtasks)
}

func TestClient_ExecuteChain_WithVariables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChainRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotNil(t, req.Variables)
		assert.Equal(t, "custom_value", req.Variables["custom_key"])

		resp := &ChainResponse{Result: "result with variables"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.ExecuteChain(context.Background(), &ChainRequest{
		ChainType: "custom",
		Prompt:    "test",
		Variables: map[string]interface{}{"custom_key": "custom_value"},
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Result)
}
