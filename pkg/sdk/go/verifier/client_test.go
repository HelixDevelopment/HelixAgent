package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		client := NewClient(ClientConfig{})
		if client == nil {
			t.Fatal("NewClient returned nil")
		}
		if client.baseURL != "http://localhost:8081" {
			t.Errorf("expected default baseURL, got %s", client.baseURL)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		client := NewClient(ClientConfig{
			BaseURL: "http://custom.api.com",
			APIKey:  "test-key",
			Timeout: 60 * time.Second,
		})
		if client.baseURL != "http://custom.api.com" {
			t.Errorf("expected custom baseURL, got %s", client.baseURL)
		}
		if client.apiKey != "test-key" {
			t.Error("apiKey not set correctly")
		}
	})

	t.Run("custom http client", func(t *testing.T) {
		customClient := &http.Client{Timeout: 120 * time.Second}
		client := NewClient(ClientConfig{
			HTTPClient: customClient,
		})
		if client.httpClient != customClient {
			t.Error("custom HTTP client not used")
		}
	})
}

func setupMockServer(handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)
	client := NewClient(ClientConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
	})
	return server, client
}

func TestVerifyModel(t *testing.T) {
	t.Run("successful verification", func(t *testing.T) {
		server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" || r.URL.Path != "/api/v1/verifier/verify" {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}

			var req VerificationRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.ModelID != "gpt-4" {
				t.Errorf("expected model_id gpt-4, got %s", req.ModelID)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VerificationResult{
				ModelID:      "gpt-4",
				Provider:     "openai",
				Verified:     true,
				Score:        9.0,
				OverallScore: 9.2,
				ScoreSuffix:  "(SC:9.2)",
				CodeVisible:  true,
				Tests:        map[string]bool{"code_visibility": true},
			})
		})
		defer server.Close()

		result, err := client.VerifyModel(context.Background(), VerificationRequest{
			ModelID:  "gpt-4",
			Provider: "openai",
		})
		if err != nil {
			t.Fatalf("VerifyModel failed: %v", err)
		}
		if !result.Verified {
			t.Error("expected verified to be true")
		}
		if result.OverallScore != 9.2 {
			t.Errorf("expected score 9.2, got %f", result.OverallScore)
		}
	})

	t.Run("verification with tests", func(t *testing.T) {
		server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
			var req VerificationRequest
			json.NewDecoder(r.Body).Decode(&req)
			if len(req.Tests) != 1 || req.Tests[0] != "code_visibility" {
				t.Errorf("expected tests [code_visibility], got %v", req.Tests)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VerificationResult{Verified: true})
		})
		defer server.Close()

		result, err := client.VerifyModel(context.Background(), VerificationRequest{
			ModelID:  "gpt-4",
			Provider: "openai",
			Tests:    []string{"code_visibility"},
		})
		if err != nil {
			t.Fatalf("VerifyModel failed: %v", err)
		}
		if !result.Verified {
			t.Error("expected verified to be true")
		}
	})
}

func TestBatchVerify(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/verifier/verify/batch" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BatchVerifyResult{
			Results: []VerificationResult{
				{ModelID: "gpt-4", Verified: true},
				{ModelID: "claude-3", Verified: true},
			},
			Summary: struct {
				Total    int `json:"total"`
				Verified int `json:"verified"`
				Failed   int `json:"failed"`
			}{Total: 2, Verified: 2, Failed: 0},
		})
	})
	defer server.Close()

	req := BatchVerifyRequest{
		Models: []struct {
			ModelID  string `json:"model_id"`
			Provider string `json:"provider"`
		}{
			{ModelID: "gpt-4", Provider: "openai"},
			{ModelID: "claude-3", Provider: "anthropic"},
		},
	}

	result, err := client.BatchVerify(context.Background(), req)
	if err != nil {
		t.Fatalf("BatchVerify failed: %v", err)
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
	if result.Summary.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Summary.Total)
	}
}

func TestGetVerificationStatus(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/verifier/status/gpt-4" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VerificationResult{
			ModelID:  "gpt-4",
			Verified: true,
		})
	})
	defer server.Close()

	result, err := client.GetVerificationStatus(context.Background(), "gpt-4")
	if err != nil {
		t.Fatalf("GetVerificationStatus failed: %v", err)
	}
	if result.ModelID != "gpt-4" {
		t.Errorf("expected model_id gpt-4, got %s", result.ModelID)
	}
}

func TestTestCodeVisibility(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/verifier/test/code-visibility" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var req CodeVisibilityRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Language != "python" {
			t.Errorf("expected language python, got %s", req.Language)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CodeVisibilityResult{
			ModelID:     "gpt-4",
			CodeVisible: true,
			Language:    "python",
			Confidence:  0.95,
		})
	})
	defer server.Close()

	result, err := client.TestCodeVisibility(context.Background(), CodeVisibilityRequest{
		ModelID:  "gpt-4",
		Provider: "openai",
		Language: "python",
	})
	if err != nil {
		t.Fatalf("TestCodeVisibility failed: %v", err)
	}
	if !result.CodeVisible {
		t.Error("expected code_visible to be true")
	}
	if result.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", result.Confidence)
	}
}

func TestGetModelScore(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/verifier/scores/gpt-4" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ScoreResult{
			ModelID:      "gpt-4",
			ModelName:    "GPT-4",
			OverallScore: 9.2,
			ScoreSuffix:  "(SC:9.2)",
			Components: ScoreComponents{
				SpeedScore:      9.0,
				EfficiencyScore: 8.5,
				CostScore:       7.0,
				CapabilityScore: 9.5,
				RecencyScore:    8.0,
			},
			DataSource: "models.dev",
		})
	})
	defer server.Close()

	result, err := client.GetModelScore(context.Background(), "gpt-4")
	if err != nil {
		t.Fatalf("GetModelScore failed: %v", err)
	}
	if result.OverallScore != 9.2 {
		t.Errorf("expected score 9.2, got %f", result.OverallScore)
	}
	if result.Components.SpeedScore != 9.0 {
		t.Errorf("expected speed_score 9.0, got %f", result.Components.SpeedScore)
	}
}

func TestGetTopModels(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("limit") != "5" {
			t.Errorf("expected limit=5, got %s", r.URL.Query().Get("limit"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TopModelsResult{
			Models: []ModelWithScore{
				{ModelID: "gpt-4", Name: "GPT-4", OverallScore: 9.5, Rank: 1},
				{ModelID: "claude-3", Name: "Claude 3", OverallScore: 9.3, Rank: 2},
			},
			Total: 2,
		})
	})
	defer server.Close()

	result, err := client.GetTopModels(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetTopModels failed: %v", err)
	}
	if len(result.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(result.Models))
	}
	if result.Models[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", result.Models[0].Rank)
	}
}

func TestGetProviderHealth(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/verifier/health/providers/openai" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProviderHealth{
			ProviderID:    "openai",
			ProviderName:  "OpenAI",
			Healthy:       true,
			CircuitState:  "closed",
			FailureCount:  0,
			SuccessCount:  100,
			AvgResponseMs: 250,
			UptimePercent: 99.9,
		})
	})
	defer server.Close()

	result, err := client.GetProviderHealth(context.Background(), "openai")
	if err != nil {
		t.Fatalf("GetProviderHealth failed: %v", err)
	}
	if !result.Healthy {
		t.Error("expected healthy to be true")
	}
	if result.CircuitState != "closed" {
		t.Errorf("expected circuit_state closed, got %s", result.CircuitState)
	}
}

func TestGetAllProvidersHealth(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/verifier/health/providers" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Providers []ProviderHealth `json:"providers"`
		}{
			Providers: []ProviderHealth{
				{ProviderID: "openai", Healthy: true},
				{ProviderID: "anthropic", Healthy: true},
			},
		})
	})
	defer server.Close()

	providers, err := client.GetAllProvidersHealth(context.Background())
	if err != nil {
		t.Fatalf("GetAllProvidersHealth failed: %v", err)
	}
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestGetHealthyProviders(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/verifier/health/healthy" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Providers []string `json:"providers"`
		}{
			Providers: []string{"openai", "anthropic", "google"},
		})
	})
	defer server.Close()

	providers, err := client.GetHealthyProviders(context.Background())
	if err != nil {
		t.Fatalf("GetHealthyProviders failed: %v", err)
	}
	if len(providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(providers))
	}
}

func TestGetModelNameWithScore(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/verifier/scores/gpt-4/name" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			NameWithScore string `json:"name_with_score"`
		}{
			NameWithScore: "GPT-4 (SC:9.2)",
		})
	})
	defer server.Close()

	name, err := client.GetModelNameWithScore(context.Background(), "gpt-4")
	if err != nil {
		t.Fatalf("GetModelNameWithScore failed: %v", err)
	}
	if name != "GPT-4 (SC:9.2)" {
		t.Errorf("expected 'GPT-4 (SC:9.2)', got '%s'", name)
	}
}

func TestHealth(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/verifier/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "healthy",
			"version": "1.0.0",
		})
	})
	defer server.Close()

	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if health["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", health["status"])
	}
}

func TestErrorHandling(t *testing.T) {
	t.Run("401 unauthorized", func(t *testing.T) {
		server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
		})
		defer server.Close()

		_, err := client.Health(context.Background())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("404 not found", func(t *testing.T) {
		server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		})
		defer server.Close()

		_, err := client.GetVerificationStatus(context.Background(), "unknown")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("500 server error", func(t *testing.T) {
		server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal error"}`))
		})
		defer server.Close()

		_, err := client.Health(context.Background())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestContextCancellation(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Health(ctx)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestAuthorizationHeader(t *testing.T) {
	server, client := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got '%s'", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer server.Close()

	_, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
}

func TestVerificationRequestFields(t *testing.T) {
	req := VerificationRequest{
		ModelID:  "gpt-4",
		Provider: "openai",
		Tests:    []string{"test1", "test2"},
	}

	if req.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if req.Provider != "openai" {
		t.Error("Provider mismatch")
	}
	if len(req.Tests) != 2 {
		t.Error("Tests length mismatch")
	}
}

func TestVerificationResultFields(t *testing.T) {
	result := VerificationResult{
		ModelID:          "gpt-4",
		Provider:         "openai",
		Verified:         true,
		Score:            9.0,
		OverallScore:     9.5,
		ScoreSuffix:      "(SC:9.5)",
		CodeVisible:      true,
		Tests:            map[string]bool{"test1": true},
		VerificationTime: 1500,
		Message:          "Success",
	}

	if result.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if !result.Verified {
		t.Error("Verified should be true")
	}
	if result.VerificationTime != 1500 {
		t.Error("VerificationTime mismatch")
	}
}

func TestScoreComponentsFields(t *testing.T) {
	components := ScoreComponents{
		SpeedScore:      9.0,
		EfficiencyScore: 8.5,
		CostScore:       7.0,
		CapabilityScore: 9.5,
		RecencyScore:    8.0,
	}

	if components.SpeedScore != 9.0 {
		t.Error("SpeedScore mismatch")
	}
	if components.CapabilityScore != 9.5 {
		t.Error("CapabilityScore mismatch")
	}
}

func TestModelWithScoreFields(t *testing.T) {
	model := ModelWithScore{
		ModelID:      "gpt-4",
		Name:         "GPT-4",
		Provider:     "openai",
		OverallScore: 9.5,
		ScoreSuffix:  "(SC:9.5)",
		Rank:         1,
	}

	if model.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if model.Rank != 1 {
		t.Error("Rank mismatch")
	}
}

func TestProviderHealthFields(t *testing.T) {
	health := ProviderHealth{
		ProviderID:    "openai",
		ProviderName:  "OpenAI",
		Healthy:       true,
		CircuitState:  "closed",
		FailureCount:  0,
		SuccessCount:  100,
		AvgResponseMs: 250,
		UptimePercent: 99.9,
		LastCheckedAt: "2024-01-15T10:30:00Z",
	}

	if health.ProviderID != "openai" {
		t.Error("ProviderID mismatch")
	}
	if !health.Healthy {
		t.Error("Healthy should be true")
	}
	if health.UptimePercent != 99.9 {
		t.Error("UptimePercent mismatch")
	}
}
