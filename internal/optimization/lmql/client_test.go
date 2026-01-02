package lmql

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
		BaseURL: "http://localhost:8014",
		Timeout: 30 * time.Second,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8014", client.baseURL)
}

func TestNewClient_DefaultConfig(t *testing.T) {
	client := NewClient(nil)
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8014", client.baseURL)
}

func TestClient_ExecuteQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/query", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var req QueryRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.NotEmpty(t, req.Query)

		resp := &QueryResponse{
			Result: map[string]interface{}{
				"NAME": "John Smith",
				"AGE":  "30",
			},
			RawOutput:            "Name: John Smith\nAge: 30",
			ConstraintsSatisfied: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	resp, err := client.ExecuteQuery(context.Background(), &QueryRequest{
		Query: `
			argmax
				"Name: [NAME]\n"
				"Age: [AGE]\n"
			from
				"Generate a person profile:"
			where
				len(NAME) < 20 and
				AGE in ["20", "30", "40", "50"]
		`,
		Variables: map[string]interface{}{
			"context": "business professional",
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp.Result)
	assert.Equal(t, "John Smith", resp.Result["NAME"])
	assert.True(t, resp.ConstraintsSatisfied)
}

func TestClient_GenerateConstrained(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/constrained", r.URL.Path)

		var req ConstrainedRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotEmpty(t, req.Prompt)
		assert.NotEmpty(t, req.Constraints)

		resp := &ConstrainedResponse{
			Text:         "Paris is the capital of France.",
			AllSatisfied: true,
			ConstraintsChecked: []ConstraintResult{
				{Type: "max_length", Value: "50", Satisfied: true},
				{Type: "contains", Value: "Paris", Satisfied: true},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateConstrained(context.Background(), &ConstrainedRequest{
		Prompt: "The capital of France is",
		Constraints: []Constraint{
			{Type: "max_length", Value: "50"},
			{Type: "contains", Value: "Paris"},
			{Type: "not_contains", Value: "London"},
		},
	})

	require.NoError(t, err)
	assert.True(t, resp.AllSatisfied)
	assert.Contains(t, resp.Text, "Paris")
}

func TestClient_GenerateWithMaxLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ConstrainedRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify max_length constraint
		hasMaxLength := false
		for _, c := range req.Constraints {
			if c.Type == "max_length" {
				hasMaxLength = true
				assert.Equal(t, "100", c.Value)
			}
		}
		assert.True(t, hasMaxLength)

		resp := &ConstrainedResponse{
			Text:         "A short response.",
			AllSatisfied: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateWithMaxLength(context.Background(), "Write something short", 100)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Text)
}

func TestClient_GenerateContaining(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ConstrainedRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify contains constraints
		assert.GreaterOrEqual(t, len(req.Constraints), 2)

		resp := &ConstrainedResponse{
			Text:         "This is an important key point.",
			AllSatisfied: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateContaining(context.Background(), "Write a sentence", []string{"important", "key"})

	require.NoError(t, err)
	assert.Contains(t, resp.Text, "important")
}

func TestClient_GenerateWithPattern(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ConstrainedRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify regex constraint
		hasRegex := false
		for _, c := range req.Constraints {
			if c.Type == "regex" {
				hasRegex = true
			}
		}
		assert.True(t, hasRegex)

		resp := &ConstrainedResponse{
			Text:         "2024-01-15",
			AllSatisfied: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateWithPattern(context.Background(), "Generate a date", `\d{4}-\d{2}-\d{2}`)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Text)
}

func TestClient_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)

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

	_, err := client.ExecuteQuery(context.Background(), &QueryRequest{Query: "test"})
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

	_, err := client.ExecuteQuery(ctx, &QueryRequest{Query: "test"})
	assert.Error(t, err)
}
