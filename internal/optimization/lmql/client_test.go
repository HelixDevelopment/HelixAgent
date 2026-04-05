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
	t.Parallel()
	config := &ClientConfig{
		BaseURL: "http://localhost:8014",
		Timeout: 30 * time.Second,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8014", client.baseURL)
}

func TestNewClient_DefaultConfig(t *testing.T) {
	t.Parallel()
	client := NewClient(nil)
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8014", client.baseURL)
}

func TestClient_ExecuteQuery(t *testing.T) {
	t.Parallel()
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
		_ = json.NewEncoder(w).Encode(resp)
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
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/constrained", r.URL.Path)

		var req ConstrainedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
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
		_ = json.NewEncoder(w).Encode(resp)
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
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ConstrainedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

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
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateWithMaxLength(context.Background(), "Write something short", 100)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Text)
}

func TestClient_GenerateContaining(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ConstrainedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify contains constraints
		assert.GreaterOrEqual(t, len(req.Constraints), 2)

		resp := &ConstrainedResponse{
			Text:         "This is an important key point.",
			AllSatisfied: true,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateContaining(context.Background(), "Write a sentence", []string{"important", "key"})

	require.NoError(t, err)
	assert.Contains(t, resp.Text, "important")
}

func TestClient_GenerateWithPattern(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ConstrainedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

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
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateWithPattern(context.Background(), "Generate a date", `\d{4}-\d{2}-\d{2}`)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Text)
}

func TestClient_Health(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)

		resp := &HealthResponse{
			Status:       "healthy",
			Version:      "1.0.0",
			LLMAvailable: true,
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

	_, err := client.ExecuteQuery(context.Background(), &QueryRequest{Query: "test"})
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

	_, err := client.ExecuteQuery(ctx, &QueryRequest{Query: "test"})
	assert.Error(t, err)
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "http://localhost:8014", config.BaseURL)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestClient_Decode(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/decode", r.URL.Path)

		var req DecodingRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.NotEmpty(t, req.Prompt)
		assert.NotEmpty(t, req.Strategy)

		resp := &DecodingResponse{
			Outputs:      []string{"Hello, world!", "Hi there!"},
			StrategyUsed: req.Strategy,
			Metadata: map[string]interface{}{
				"total_tokens": 10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Decode(context.Background(), &DecodingRequest{
		Prompt:      "Say hello",
		Strategy:    "sample",
		NumSamples:  2,
		Temperature: 0.8,
	})

	require.NoError(t, err)
	assert.Len(t, resp.Outputs, 2)
	assert.Equal(t, "sample", resp.StrategyUsed)
}

func TestClient_Decode_DefaultStrategy(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DecodingRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "argmax", req.Strategy) // Default strategy

		resp := &DecodingResponse{
			Outputs:      []string{"Result"},
			StrategyUsed: req.Strategy,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.Decode(context.Background(), &DecodingRequest{
		Prompt: "Test prompt",
	})

	require.NoError(t, err)
	assert.Equal(t, "argmax", resp.StrategyUsed)
}

func TestClient_DecodeGreedy(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DecodingRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "argmax", req.Strategy)

		resp := &DecodingResponse{
			Outputs:      []string{"Greedy output"},
			StrategyUsed: "argmax",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	result, err := client.DecodeGreedy(context.Background(), "Generate text")

	require.NoError(t, err)
	assert.Equal(t, "Greedy output", result)
}

func TestClient_DecodeGreedy_NoOutput(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &DecodingResponse{
			Outputs:      []string{},
			StrategyUsed: "argmax",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	_, err := client.DecodeGreedy(context.Background(), "Generate text")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no output generated")
}

func TestClient_DecodeSample(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DecodingRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "sample", req.Strategy)
		assert.Equal(t, 5, req.NumSamples)
		assert.Equal(t, 0.9, req.Temperature)

		resp := &DecodingResponse{
			Outputs:      []string{"Sample 1", "Sample 2", "Sample 3", "Sample 4", "Sample 5"},
			StrategyUsed: "sample",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	results, err := client.DecodeSample(context.Background(), "Generate samples", 5, 0.9)

	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestClient_DecodeSample_Defaults(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DecodingRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 3, req.NumSamples)    // Default
		assert.Equal(t, 0.7, req.Temperature) // Default

		resp := &DecodingResponse{
			Outputs:      []string{"S1", "S2", "S3"},
			StrategyUsed: "sample",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	results, err := client.DecodeSample(context.Background(), "Generate samples", 0, 0)

	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestClient_DecodeBeam(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DecodingRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "beam", req.Strategy)
		assert.Equal(t, 5, req.BeamWidth)

		resp := &DecodingResponse{
			Outputs:      []string{"Beam 1", "Beam 2", "Beam 3", "Beam 4", "Beam 5"},
			StrategyUsed: "beam",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	results, err := client.DecodeBeam(context.Background(), "Generate with beam search", 5)

	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestClient_DecodeBeam_DefaultWidth(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DecodingRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 3, req.BeamWidth) // Default

		resp := &DecodingResponse{
			Outputs:      []string{"B1", "B2", "B3"},
			StrategyUsed: "beam",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	results, err := client.DecodeBeam(context.Background(), "Beam search", 0)

	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestClient_ScoreCompletions(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/score", r.URL.Path)

		resp := &ScoreResponse{
			Prompt: "The capital of France is",
			Scores: map[string]float64{
				"Paris":  0.95,
				"London": 0.15,
				"Berlin": 0.20,
			},
			Ranking: []int{0, 2, 1},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	result, err := client.ScoreCompletions(context.Background(),
		"The capital of France is",
		[]string{"Paris", "London", "Berlin"})

	require.NoError(t, err)
	assert.Equal(t, 0.95, result.Scores["Paris"])
	assert.Greater(t, result.Scores["Paris"], result.Scores["London"])
}

func TestClient_SelectBestCompletion(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &ScoreResponse{
			Prompt: "What is 2+2?",
			Scores: map[string]float64{
				"4":  0.98,
				"5":  0.01,
				"3":  0.01,
				"22": 0.0,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	best, err := client.SelectBestCompletion(context.Background(),
		"What is 2+2?",
		[]string{"4", "5", "3", "22"})

	require.NoError(t, err)
	assert.Equal(t, "4", best)
}

func TestClient_ExecuteQuery_DefaultValues(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req QueryRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 0.7, req.Temperature) // Default
		assert.Equal(t, 500, req.MaxTokens)   // Default

		resp := &QueryResponse{
			Result:               map[string]interface{}{"answer": "test"},
			ConstraintsSatisfied: true,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.ExecuteQuery(context.Background(), &QueryRequest{
		Query: "test query",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestClient_GenerateConstrained_DefaultTemperature(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ConstrainedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, 0.7, req.Temperature) // Default

		resp := &ConstrainedResponse{
			Text:         "Result",
			AllSatisfied: true,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})

	resp, err := client.GenerateConstrained(context.Background(), &ConstrainedRequest{
		Prompt:      "Test",
		Constraints: []Constraint{{Type: "max_length", Value: "50"}},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}
