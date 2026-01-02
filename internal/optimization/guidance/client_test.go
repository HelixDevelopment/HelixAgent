package guidance

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
		BaseURL: "http://localhost:8013",
		Timeout: 30 * time.Second,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8013", client.baseURL)
}

func TestNewClient_DefaultConfig(t *testing.T) {
	client := NewClient(nil)
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8013", client.baseURL)
}

func TestClient_GenerateWithGrammar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/grammar", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var req GrammarRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.NotEmpty(t, req.Prompt)
		assert.NotEmpty(t, req.Grammar)

		resp := &GrammarResponse{
			Text: `{"name":"John","age":30}`,
			Parsed: map[string]interface{}{
				"name": "John",
				"age":  30,
			},
			Valid: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	resp, err := client.GenerateWithGrammar(context.Background(), &GrammarRequest{
		Prompt: "Generate a JSON object with name and age:",
		Grammar: `
			start: object
			object: "{" pair ("," pair)* "}"
			pair: string ":" value
		`,
	})

	require.NoError(t, err)
	assert.Contains(t, resp.Text, "name")
	assert.Contains(t, resp.Text, "age")
	assert.True(t, resp.Valid)
}

func TestClient_GenerateFromTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/template", r.URL.Path)

		var req TemplateRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotEmpty(t, req.Template)

		resp := &TemplateResponse{
			FilledTemplate: "Name: John, Age: 30, City: NYC",
			GeneratedValues: map[string]string{
				"name": "John",
				"age":  "30",
				"city": "NYC",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateFromTemplate(context.Background(), &TemplateRequest{
		Template: `
			Name: {{gen "name" max_tokens=20}}
			Age: {{gen "age" pattern="[0-9]+"}}
			City: {{select "city" options=["NYC", "LA", "Chicago"]}}
		`,
		Variables: map[string]interface{}{
			"context": "Generate a person profile",
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp.GeneratedValues)
	assert.Equal(t, "John", resp.GeneratedValues["name"])
}

func TestClient_Select(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/select", r.URL.Path)

		var req SelectRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotEmpty(t, req.Prompt)
		assert.NotEmpty(t, req.Options)

		resp := &SelectResponse{
			Selected:  []string{"Go"},
			Reasoning: "Go is efficient and modern",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Select(context.Background(), &SelectRequest{
		Prompt:  "The best programming language is:",
		Options: []string{"Python", "JavaScript", "Go", "Rust"},
	})

	require.NoError(t, err)
	assert.Contains(t, resp.Selected, "Go")
}

func TestClient_SelectOne(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &SelectResponse{
			Selected:  []string{"Go"},
			Reasoning: "Best for systems programming",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	selected, err := client.SelectOne(context.Background(), "The best language is:", []string{"Python", "Go"})

	require.NoError(t, err)
	assert.Equal(t, "Go", selected)
}

func TestClient_GenerateWithRegex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/regex", r.URL.Path)

		var req RegexRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotEmpty(t, req.Pattern)

		resp := &RegexResponse{
			Text:        "(555) 123-4567",
			Matches:     true,
			MatchGroups: []string{"555", "123", "4567"},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateWithRegex(context.Background(), &RegexRequest{
		Prompt:  "Generate a phone number:",
		Pattern: `\(\d{3}\) \d{3}-\d{4}`,
	})

	require.NoError(t, err)
	assert.Equal(t, "(555) 123-4567", resp.Text)
	assert.True(t, resp.Matches)
}

func TestClient_GenerateJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/json_schema", r.URL.Path)

		var req JSONSchemaRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.NotNil(t, req.Schema)

		resp := &JSONSchemaResponse{
			JSON: map[string]interface{}{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   25,
			},
			Valid: true,
			Raw:   `{"name":"Alice","email":"alice@example.com","age":25}`,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":  map[string]interface{}{"type": "string"},
			"email": map[string]interface{}{"type": "string"},
			"age":   map[string]interface{}{"type": "integer"},
		},
	}

	resp, err := client.GenerateJSON(context.Background(), &JSONSchemaRequest{
		Prompt: "Generate a user profile:",
		Schema: schema,
	})

	require.NoError(t, err)
	assert.True(t, resp.Valid)
	assert.NotNil(t, resp.JSON)
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

	_, err := client.GenerateWithGrammar(context.Background(), &GrammarRequest{
		Prompt:  "test",
		Grammar: "start: word",
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

	_, err := client.SelectOne(ctx, "test", []string{"a", "b"})
	assert.Error(t, err)
}
