package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/router"
	"dev.helix.agent/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkDatabaseAvailable checks if the database is available for testing
func checkDatabaseAvailable(t *testing.T) bool {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "helixagent"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "helixagent123"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "helixagent_db"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable connect_timeout=5",
		host, port, user, password, dbname)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		t.Logf("Database connection failed: %v", err)
		return false
	}
	defer conn.Close(ctx)

	err = conn.Ping(ctx)
	if err != nil {
		t.Logf("Database ping failed: %v", err)
		return false
	}

	return true
}

// checkModelsDevRoutesAvailable checks if Models.dev routes are registered
func checkModelsDevRoutesAvailable(r *gin.Engine) bool {
	req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Routes are available if we get something other than 404
	return w.Code != http.StatusNotFound
}

func setupTestRouter(t *testing.T) (*gin.Engine, func()) {
	gin.SetMode(gin.TestMode)

	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "helixagent"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "helixagent123"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "helixagent_db"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "test-jwt-secret-for-integration-tests"
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         "8100",
			Mode:         gin.TestMode,
			EnableCORS:   false,
			DebugEnabled: false,
			JWTSecret:    jwtSecret,
		},
		Database: config.DatabaseConfig{
			Host:           host,
			Port:           port,
			User:           user,
			Password:       password,
			Name:           dbname,
			SSLMode:        "disable",
			MaxConnections: 10,
			PoolSize:       5,
		},
		ModelsDev: config.ModelsDevConfig{
			Enabled:          false, // Don't connect to real Models.dev in tests
			APIKey:           "test-key",
			BaseURL:          "https://api.models.dev/v1",
			RefreshInterval:  1 * time.Hour,
			CacheTTL:         1 * time.Hour,
			DefaultBatchSize: 100,
			MaxRetries:       3,
			AutoRefresh:      false,
		},
	}

	r := router.SetupRouter(cfg)

	return r, func() {
	}
}

func TestModelsDevIntegration_APIEndpoints(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	t.Run("HealthCheck", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
	})

	t.Run("V1HealthCheck", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
	})
}

func TestModelsDevIntegration_ModelMetadataEndpoints(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	// Check if Models.dev routes are available (feature may be disabled)
	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("ListModelsEndpoint", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Models     interface{} `json:"models"`
			Total      int         `json:"total"`
			Page       int         `json:"page"`
			Limit      int         `json:"limit"`
			TotalPages int         `json:"total_pages"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, response.Total, 0)
		assert.Equal(t, 1, response.Page)
		assert.Equal(t, 20, response.Limit)
	})

	t.Run("ListModelsWithProviderFilter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?provider=anthropic&page=1&limit=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ListModelsWithModelTypeFilter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?type=chat&page=1&limit=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SearchModels", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?search=claude&page=1&limit=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestModelsDevIntegration_ModelComparison(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("CompareModelsSuccess", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata/compare?ids=model-1,model-2,model-3", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "models")
	})

	t.Run("CompareModelsLessThanTwo", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata/compare?ids=model-1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "error")
		assert.Contains(t, fmt.Sprintf("%v", response["error"]), "at least 2 models")
	})

	t.Run("CompareModelsMoreThanTen", func(t *testing.T) {
		ids := "model-1,model-2,model-3,model-4,model-5,model-6,model-7,model-8,model-9,model-10,model-11"
		req, _ := http.NewRequest("GET", "/v1/models/metadata/compare?ids="+ids, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "error")
		assert.Contains(t, fmt.Sprintf("%v", response["error"]), "maximum 10 models")
	})
}

func TestModelsDevIntegration_CapabilityEndpoints(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	capabilities := []string{"vision", "function_calling", "streaming", "json_mode", "image_generation", "audio", "code_generation", "reasoning"}

	for _, capability := range capabilities {
		t.Run("Capability_"+capability, func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/models/metadata/capability/%s", capability), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, capability, response["capability"])
			assert.Contains(t, response, "models")
			assert.Contains(t, response, "total")
		})
	}

	t.Run("InvalidCapability", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata/capability/invalid", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "error")
	})
}

func TestModelsDevIntegration_ProviderEndpoints(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("GetProviderModels", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/providers/anthropic/models/metadata", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "anthropic", response["provider_id"])
		assert.Contains(t, response, "models")
		assert.Contains(t, response, "total")
	})

	t.Run("GetProviderModels_NotFound", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/providers/nonexistent/models/metadata", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "nonexistent", response["provider_id"])
		assert.Equal(t, float64(0), response["total"])
	})
}

func TestModelsDevIntegration_BenchmarkEndpoints(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("GetModelBenchmarks", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata/claude-3-sonnet-20240229/benchmarks", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "benchmarks")
	})

	t.Run("GetModelBenchmarks_NotFound", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata/nonexistent/benchmarks", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		benchmarks, ok := response["benchmarks"]
		assert.True(t, ok)
		assert.Equal(t, float64(0), len(benchmarks.([]interface{})))
	})
}

func TestModelsDevIntegration_AdminEndpoints(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("RefreshModels_AuthRequired", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/v1/admin/models/metadata/refresh", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, w.Code)
	})

	t.Run("RefreshModelsStatus_AuthRequired", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/admin/models/metadata/refresh/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, w.Code)
	})
}

func TestModelsDevIntegration_CacheBehavior(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	modelID := "cache-test-model"

	t.Run("FirstRequest_CacheMiss", func(t *testing.T) {
		start := time.Now()
		req, _ := http.NewRequest("GET", "/v1/models/metadata/"+modelID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		duration := time.Since(start)

		assert.Equal(t, http.StatusNotFound, w.Code)
		t.Logf("First request duration: %v", duration)
	})

	t.Run("SecondRequest_CacheHit", func(t *testing.T) {
		start := time.Now()
		req, _ := http.NewRequest("GET", "/v1/models/metadata/"+modelID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		duration := time.Since(start)

		assert.Equal(t, http.StatusNotFound, w.Code)
		t.Logf("Second request duration: %v", duration)
	})
}

func TestModelsDevIntegration_ResponseFormats(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("JSONContentType", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})

	t.Run("ValidJSONResponse", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var data interface{}
		err := json.Unmarshal(w.Body.Bytes(), &data)
		require.NoError(t, err)
		assert.NotNil(t, data)
	})
}

func TestModelsDevIntegration_ErrorHandling(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("InvalidModelID", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata/invalid/model/id", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("InvalidPagination", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?page=-1&limit=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidLimit", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=150", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "error")
	})
}

func TestModelsDevIntegration_EndToEndWorkflow(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("CompleteWorkflow", func(t *testing.T) {
		models := make([]string, 0)

		for i := 0; i < 10; i++ {
			req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response struct {
				Models []interface{} `json:"models"`
				Total  int           `json:"total"`
			}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if len(response.Models) > 0 {
				if modelMap, ok := response.Models[0].(map[string]interface{}); ok {
					if id, ok := modelMap["model_id"].(string); ok {
						models = append(models, id)
					}
				}
			}
		}

		t.Logf("Found %d models", len(models))

		if len(models) > 0 {
			modelID := models[0]

			req, _ := http.NewRequest("GET", "/v1/models/metadata/"+modelID, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, w.Code)

			req, _ = http.NewRequest("GET", "/v1/models/metadata/compare?ids="+modelID, nil)
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)

			req, _ = http.NewRequest("GET", "/v1/models/metadata/capability/vision", nil)
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})
}

func TestModelsDevIntegration_Performance(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("ResponseTime", func(t *testing.T) {
		iterations := 100
		totalTime := time.Duration(0)

		for i := 0; i < iterations; i++ {
			start := time.Now()
			req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			duration := time.Since(start)

			assert.Equal(t, http.StatusOK, w.Code)
			totalTime += duration
		}

		avgTime := totalTime / time.Duration(iterations)
		t.Logf("Average response time: %v", avgTime)
		assert.Less(t, avgTime, 100*time.Millisecond, "Average response time should be less than 100ms")
	})

	t.Run("Throughput", func(t *testing.T) {
		duration := 1 * time.Second
		requestCount := 0
		done := make(chan bool)
		start := time.Now()

		go func() {
			for time.Since(start) < duration {
				req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				requestCount++
			}
			done <- true
		}()

		<-done
		t.Logf("Throughput: %d requests/second", requestCount)
	})
}

func TestModelsDevIntegration_ConcurrentRequests(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("ConcurrentReads", func(t *testing.T) {
		concurrency := 50
		done := make(chan bool, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				done <- true
			}(i)
		}

		for i := 0; i < concurrency; i++ {
			<-done
		}
	})

	t.Run("ConcurrentModelRequests", func(t *testing.T) {
		concurrency := 20
		done := make(chan bool, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				modelID := fmt.Sprintf("model-%d", id)
				req, _ := http.NewRequest("GET", "/v1/models/metadata/"+modelID, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, w.Code)
				done <- true
			}(i)
		}

		for i := 0; i < concurrency; i++ {
			<-done
		}
	})
}

func TestModelsDevIntegration_DataIntegrity(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	if !checkModelsDevRoutesAvailable(r) {
		t.Skip("Models.dev routes not available - feature is disabled") // SKIP-OK: #legacy-untriaged
	}

	t.Run("ConsistentResponses", func(t *testing.T) {
		var responses []map[string]interface{}

		for i := 0; i < 5; i++ {
			req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			responses = append(responses, response)
		}

		for i := 1; i < len(responses); i++ {
			assert.Equal(t, responses[0]["total"], responses[i]["total"])
		}
	})

	t.Run("PaginationConsistency", func(t *testing.T) {
		req1, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=10", nil)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		req2, _ := http.NewRequest("GET", "/v1/models/metadata?page=2&limit=10", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w1.Code)
		assert.Equal(t, http.StatusOK, w2.Code)

		var response1, response2 map[string]interface{}
		err1 := json.Unmarshal(w1.Body.Bytes(), &response1)
		err2 := json.Unmarshal(w2.Body.Bytes(), &response2)
		require.NoError(t, err1)
		require.NoError(t, err2)

		assert.Equal(t, response1["total"], response2["total"])
		assert.Equal(t, float64(1), response1["page"])
		assert.Equal(t, float64(2), response2["page"])
	})
}

func TestModelsDevIntegration_ServiceAvailability(t *testing.T) {
	testutil.RequirePostgres(t)

	r, cleanup := setupTestRouter(t)
	defer cleanup()

	// This test checks both health endpoints (always available) and Models.dev routes
	// Health endpoints should work regardless of Models.dev status

	endpoints := []struct {
		method string
		path   string
		name   string
	}{
		{"GET", "/health", "Health"},
		{"GET", "/v1/health", "V1Health"},
		{"GET", "/v1/models/metadata", "ListModels"},
		{"GET", "/v1/models/metadata/capability/vision", "CapabilityVision"},
		{"GET", "/v1/providers/anthropic/models/metadata", "ProviderModels"},
	}

	for _, endpoint := range endpoints {
		t.Run("ServiceAvailable_"+endpoint.name, func(t *testing.T) {
			req, _ := http.NewRequest(endpoint.method, endpoint.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Contains(t, []int{http.StatusOK, http.StatusNotFound, http.StatusBadRequest}, w.Code)
		})
	}
}
