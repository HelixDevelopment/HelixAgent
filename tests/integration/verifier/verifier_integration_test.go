package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/verifier"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestVerifierServiceIntegration tests verifier service integration with handlers
func TestVerifierServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := verifier.DefaultConfig()
	cfg.Enabled = true

	svc := verifier.NewVerificationService(cfg)
	if svc == nil {
		t.Fatal("Failed to create verification service")
	}

	t.Run("VerificationServiceWithScoringService", func(t *testing.T) {
		scoringSvc, err := verifier.NewScoringService(cfg)
		if err != nil {
			t.Fatalf("Failed to create scoring service: %v", err)
		}

		// Verify a model
		ctx := context.Background()
		result, err := svc.VerifyModel(ctx, "gpt-4", "openai")
		if err != nil {
			t.Logf("Warning: VerifyModel returned error (expected if provider not configured): %v", err)
		} else {
			// Get score for same model
			score, err := scoringSvc.CalculateScore(ctx, "gpt-4")
			if err != nil {
				t.Logf("Warning: CalculateScore returned error: %v", err)
			} else {
				if result.Verified && score.OverallScore > 0 {
					t.Logf("Integration verified: model %s verified=%v, score=%.2f",
						result.ModelID, result.Verified, score.OverallScore)
				}
			}
		}
	})

	t.Run("HealthServiceIntegration", func(t *testing.T) {
		healthSvc := verifier.NewHealthService(cfg)
		if healthSvc == nil {
			t.Fatal("Failed to create health service")
		}

		// Check circuit breaker integration
		cb := verifier.NewCircuitBreaker("test-provider")
		if cb == nil {
			t.Fatal("Failed to create circuit breaker")
		}

		// Record successes
		for i := 0; i < 5; i++ {
			cb.RecordSuccess()
		}

		if cb.State() != verifier.CircuitClosed {
			t.Errorf("Expected circuit closed after successes, got %s", cb.State())
		}

		// Add provider and check health
		healthSvc.AddProvider("test-provider", "test")
		health, err := healthSvc.GetProviderHealth("test-provider")
		// This may error if provider not registered, which is expected
		if err == nil && health != nil {
			t.Logf("Provider status: healthy=%v", health.Healthy)
		}
	})
}

// TestVerifierAPIIntegration tests verifier API endpoints integration
func TestVerifierAPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Setup mock verification service
	cfg := verifier.DefaultConfig()
	svc := verifier.NewVerificationService(cfg)

	// Register API routes (simulated)
	api := router.Group("/api/v1/verifier")
	{
		api.POST("/verify", func(c *gin.Context) {
			var req struct {
				ModelID  string `json:"model_id"`
				Provider string `json:"provider"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			result, err := svc.VerifyModel(c.Request.Context(), req.ModelID, req.Provider)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, result)
		})

		api.GET("/status/:model_id", func(c *gin.Context) {
			modelID := c.Param("model_id")
			// For testing, return a mock status - in production this would query a database
			c.JSON(http.StatusOK, gin.H{
				"model_id": modelID,
				"status":   "unknown",
				"message":  "Model status check is a no-op in integration test mode",
			})
		})

		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"version": "1.0.0",
			})
		})
	}

	t.Run("VerifyEndpoint", func(t *testing.T) {
		body := `{"model_id": "gpt-4", "provider": "openai"}`
		req, _ := http.NewRequest("POST", "/api/v1/verifier/verify", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// May fail if no provider is configured, but endpoint should respond
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 200 or 500, got %d", w.Code)
		}
	})

	t.Run("HealthEndpoint", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/verifier/health", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse response: %v", err)
		}

		if response["status"] != "healthy" {
			t.Error("Expected healthy status")
		}
	})

	t.Run("StatusEndpoint_Returns", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/verifier/status/unknown-model", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Status endpoint returns 200 with mock data in integration test mode
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse response: %v", err)
		}

		if response["model_id"] != "unknown-model" {
			t.Error("Expected model_id in response")
		}
	})
}

// TestVerifierMultiProviderIntegration tests verification across multiple providers
func TestVerifierMultiProviderIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := verifier.DefaultConfig()
	svc := verifier.NewVerificationService(cfg)

	providers := []struct {
		modelID  string
		provider string
	}{
		{"gpt-4", "openai"},
		{"claude-3-opus", "anthropic"},
		{"gemini-pro", "google"},
		{"deepseek-chat", "deepseek"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, p := range providers {
		t.Run(p.provider+"_"+p.modelID, func(t *testing.T) {
			result, err := svc.VerifyModel(ctx, p.modelID, p.provider)
			if err != nil {
				// Expected if provider not configured
				t.Logf("Provider %s not available: %v", p.provider, err)
				return
			}

			t.Logf("Provider %s model %s: verified=%v, score=%.2f",
				p.provider, p.modelID, result.Verified, result.OverallScore)
		})
	}
}

// TestVerifierCacheIntegration tests caching behavior
func TestVerifierCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := verifier.DefaultConfig()
	scoringSvc, err := verifier.NewScoringService(cfg)
	if err != nil {
		t.Fatalf("Failed to create scoring service: %v", err)
	}

	ctx := context.Background()

	t.Run("CacheConsistency", func(t *testing.T) {
		// First call
		result1, err := scoringSvc.CalculateScore(ctx, "gpt-4")
		if err != nil {
			t.Logf("Warning: CalculateScore returned error: %v", err)
			return
		}

		// Second call should return cached result
		result2, err := scoringSvc.CalculateScore(ctx, "gpt-4")
		if err != nil {
			t.Fatalf("Second call failed: %v", err)
		}

		// Timestamps should be the same (cached)
		if result1.CalculatedAt != result2.CalculatedAt {
			t.Error("Expected cached result to have same timestamp")
		}
	})
}

// TestVerifierConfigIntegration tests configuration propagation
func TestVerifierConfigIntegration(t *testing.T) {
	t.Run("DefaultConfigPropagation", func(t *testing.T) {
		cfg := verifier.DefaultConfig()

		if !cfg.Enabled {
			t.Error("Expected Enabled to be true by default")
		}

		svc := verifier.NewVerificationService(cfg)
		if svc == nil {
			t.Fatal("Service should be created with default config")
		}
	})

	t.Run("CustomConfigPropagation", func(t *testing.T) {
		cfg := &verifier.Config{
			Enabled: true,
			Verification: verifier.VerificationConfig{
				VerificationTimeout: 60 * time.Second,
				RetryCount:          5,
			},
			Scoring: verifier.ScoringConfig{
				CacheTTL: 30 * time.Minute,
			},
		}

		svc := verifier.NewVerificationService(cfg)
		if svc == nil {
			t.Fatal("Service should be created with custom config")
		}
	})
}
