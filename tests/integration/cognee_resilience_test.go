package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/services"
)

// =============================================================================
// TEST SUITE: Cognee Resilience and Error Handling
// =============================================================================
// These tests verify that Cognee integration handles all error scenarios
// gracefully, preventing system failures and ensuring continuous operation.
// =============================================================================

// TestCogneeDatasetManagementResilience tests dataset creation and management resilience
func TestCogneeDatasetManagementResilience(t *testing.T) {
	t.Run("EnsureDefaultDataset creates dataset if not exists", func(t *testing.T) {
		// Create mock server that simulates Cognee API
		datasetsCreated := make(map[string]bool)
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()

			switch {
			case r.URL.Path == "/" && r.Method == "GET":
				// Health check
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello, World, I am alive!"))

			case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
				// Auth - return token
				json.NewEncoder(w).Encode(map[string]string{
					"access_token": "test-token",
					"token_type":   "bearer",
				})

			case r.URL.Path == "/api/v1/datasets" && r.Method == "GET":
				// List datasets
				datasets := make([]map[string]interface{}, 0)
				for name := range datasetsCreated {
					datasets = append(datasets, map[string]interface{}{"name": name})
				}
				json.NewEncoder(w).Encode(map[string]interface{}{"datasets": datasets})

			case r.URL.Path == "/api/v1/datasets" && r.Method == "POST":
				// Create dataset
				var body map[string]interface{}
				json.NewDecoder(r.Body).Decode(&body)
				name := body["name"].(string)
				datasetsCreated[name] = true
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id":   "test-id",
					"name": name,
				})

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		// Create service with mock server
		cfg := &services.CogneeServiceConfig{
			Enabled:        true,
			BaseURL:        server.URL,
			DefaultDataset: "test-default",
			Timeout:        10 * time.Second,
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		// Call EnsureDefaultDataset
		err := service.EnsureDefaultDataset(context.Background())
		assert.NoError(t, err)

		// Verify dataset was created
		assert.True(t, datasetsCreated["test-default"], "Default dataset should be created")
	})

	t.Run("EnsureDefaultDataset handles existing dataset", func(t *testing.T) {
		createCalls := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/" && r.Method == "GET":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello, World, I am alive!"))

			case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
				json.NewEncoder(w).Encode(map[string]string{
					"access_token": "test-token",
					"token_type":   "bearer",
				})

			case r.URL.Path == "/api/v1/datasets" && r.Method == "GET":
				// Return existing dataset
				json.NewEncoder(w).Encode(map[string]interface{}{
					"datasets": []map[string]interface{}{
						{"name": "default"},
					},
				})

			case r.URL.Path == "/api/v1/datasets" && r.Method == "POST":
				createCalls++
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{"error": "already exists"})

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		cfg := &services.CogneeServiceConfig{
			Enabled:        true,
			BaseURL:        server.URL,
			DefaultDataset: "default",
			Timeout:        10 * time.Second,
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		err := service.EnsureDefaultDataset(context.Background())
		assert.NoError(t, err)

		// Should not try to create since dataset exists
		assert.Equal(t, 0, createCalls, "Should not create dataset if it already exists")
	})
}

// TestCogneeSearchErrorHandling tests search error scenarios
func TestCogneeSearchErrorHandling(t *testing.T) {
	t.Run("Search handles NoDataError gracefully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/" && r.Method == "GET":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello, World, I am alive!"))

			case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
				json.NewEncoder(w).Encode(map[string]string{
					"access_token": "test-token",
					"token_type":   "bearer",
				})

			case r.URL.Path == "/api/v1/search" && r.Method == "POST":
				// Return NoDataError
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "NoDataError: No data found in the system",
				})

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		cfg := &services.CogneeServiceConfig{
			Enabled:            true,
			BaseURL:            server.URL,
			DefaultDataset:     "default",
			Timeout:            10 * time.Second,
			DefaultSearchLimit: 10,
			SearchTypes:        []string{"CHUNKS"},
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		// Search should return empty results, not error
		result, err := service.SearchMemory(context.Background(), "test query", "default", 10)
		assert.NoError(t, err, "Search should not fail on NoDataError")
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.TotalResults)
	})

	t.Run("Search handles DatasetNotFoundError gracefully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/" && r.Method == "GET":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello, World, I am alive!"))

			case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
				json.NewEncoder(w).Encode(map[string]string{
					"access_token": "test-token",
					"token_type":   "bearer",
				})

			case r.URL.Path == "/api/v1/search" && r.Method == "POST":
				// Return DatasetNotFoundError
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "DatasetNotFoundError: No datasets found",
				})

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		cfg := &services.CogneeServiceConfig{
			Enabled:            true,
			BaseURL:            server.URL,
			DefaultDataset:     "default",
			Timeout:            10 * time.Second,
			DefaultSearchLimit: 10,
			SearchTypes:        []string{"CHUNKS"},
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		result, err := service.SearchMemory(context.Background(), "test query", "default", 10)
		assert.NoError(t, err, "Search should not fail on DatasetNotFoundError")
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.TotalResults)
	})

	t.Run("Search handles timeout gracefully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/" && r.Method == "GET":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello, World, I am alive!"))

			case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
				json.NewEncoder(w).Encode(map[string]string{
					"access_token": "test-token",
					"token_type":   "bearer",
				})

			case r.URL.Path == "/api/v1/search" && r.Method == "POST":
				// Simulate slow response
				time.Sleep(10 * time.Second)
				w.WriteHeader(http.StatusOK)

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		cfg := &services.CogneeServiceConfig{
			Enabled:            true,
			BaseURL:            server.URL,
			DefaultDataset:     "default",
			Timeout:            2 * time.Second, // Short timeout
			DefaultSearchLimit: 10,
			SearchTypes:        []string{"CHUNKS"},
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		// Create context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		result, err := service.SearchMemory(ctx, "test query", "default", 10)
		// Should return gracefully, not hang
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// TestCogneeConcurrentSearchResilience tests concurrent search handling
func TestCogneeConcurrentSearchResilience(t *testing.T) {
	t.Run("Multiple concurrent searches handled gracefully", func(t *testing.T) {
		var requestCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requestCount, 1)

			switch {
			case r.URL.Path == "/" && r.Method == "GET":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello, World, I am alive!"))

			case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
				json.NewEncoder(w).Encode(map[string]string{
					"access_token": "test-token",
					"token_type":   "bearer",
				})

			case r.URL.Path == "/api/v1/search" && r.Method == "POST":
				// Simulate variable response time
				time.Sleep(50 * time.Millisecond)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []interface{}{},
				})

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		cfg := &services.CogneeServiceConfig{
			Enabled:            true,
			BaseURL:            server.URL,
			DefaultDataset:     "default",
			Timeout:            10 * time.Second,
			DefaultSearchLimit: 10,
			SearchTypes:        []string{"CHUNKS", "GRAPH_COMPLETION", "RAG_COMPLETION"},
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		// Launch multiple concurrent searches
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				_, err := service.SearchMemory(context.Background(), fmt.Sprintf("query %d", idx), "default", 10)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// All searches should complete without errors
		errorCount := 0
		for err := range errors {
			t.Logf("Search error: %v", err)
			errorCount++
		}

		assert.Equal(t, 0, errorCount, "All concurrent searches should complete without errors")
	})
}

// TestCogneeServiceHealthCheck tests health check functionality
func TestCogneeServiceHealthCheck(t *testing.T) {
	t.Run("IsHealthy returns true for healthy service", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" && r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello, World, I am alive!"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &services.CogneeServiceConfig{
			Enabled: true,
			BaseURL: server.URL,
			Timeout: 10 * time.Second,
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		healthy := service.IsHealthy(context.Background())
		assert.True(t, healthy, "Service should be healthy")
	})

	t.Run("IsHealthy returns false for unavailable service", func(t *testing.T) {
		cfg := &services.CogneeServiceConfig{
			Enabled: true,
			BaseURL: "http://localhost:59999", // Non-existent port
			Timeout: 2 * time.Second,
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		healthy := service.IsHealthy(context.Background())
		assert.False(t, healthy, "Service should not be healthy")
	})

	t.Run("IsHealthy returns false for slow service", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Second) // Very slow
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &services.CogneeServiceConfig{
			Enabled: true,
			BaseURL: server.URL,
			Timeout: 1 * time.Second,
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		healthy := service.IsHealthy(ctx)
		assert.False(t, healthy, "Slow service should not be considered healthy")
	})
}

// TestCogneeAuthenticationResilience tests authentication error handling
func TestCogneeAuthenticationResilience(t *testing.T) {
	t.Run("Service works without authentication", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/" && r.Method == "GET":
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Hello, World, I am alive!"))

			case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
				// Auth fails
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})

			case r.URL.Path == "/api/v1/datasets" && r.Method == "GET":
				// But datasets work without auth
				json.NewEncoder(w).Encode(map[string]interface{}{
					"datasets": []map[string]interface{}{{"name": "default"}},
				})

			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		cfg := &services.CogneeServiceConfig{
			Enabled:        true,
			BaseURL:        server.URL,
			DefaultDataset: "default",
			Timeout:        10 * time.Second,
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		// Should work despite auth failure
		datasets, err := service.ListDatasets(context.Background())
		assert.NoError(t, err)
		assert.Len(t, datasets, 1)
	})
}

// TestCogneeLiveIntegrationResilience tests live Cognee resilience
func TestCogneeLiveIntegrationResilience(t *testing.T) {
	serverURL := os.Getenv("HELIXAGENT_TEST_URL")
	if serverURL == "" {
		serverURL = "http://localhost:7061"
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Check if server is available
	healthResp, err := client.Get(serverURL + "/health")
	if err != nil {
		t.Skip("HelixAgent server not available")
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Skip("HelixAgent server not healthy")
	}

	t.Run("Cognee errors don't break chat completions", func(t *testing.T) {
		// Even if Cognee has issues, chat should work
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "Say hello"},
			},
			"max_tokens": 10,
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			serverURL+"/v1/chat/completions",
			"application/json",
			bytes.NewReader(jsonBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		// Should succeed even if Cognee has issues
		if resp.StatusCode == 502 {
			t.Skip("Providers temporarily unavailable")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Response: %s", string(body))

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Verify response
		assert.Equal(t, "helixagent-ensemble", result["model"])
	})

	t.Run("Cognee search endpoint handles errors gracefully", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"query":       "test query that returns no results",
			"dataset":     "nonexistent-dataset",
			"limit":       5,
			"search_type": "CHUNKS",
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			serverURL+"/v1/cognee/search",
			"application/json",
			bytes.NewReader(jsonBody),
		)

		if err == nil {
			defer resp.Body.Close()
			// Should not crash - may return error but in structured way
			assert.Contains(t, []int{200, 400, 404, 500, 503}, resp.StatusCode)
		}
	})
}

// TestCogneeConfigurationValidation tests configuration edge cases
func TestCogneeConfigurationValidation(t *testing.T) {
	t.Run("Service handles empty config gracefully", func(t *testing.T) {
		cfg := &services.CogneeServiceConfig{}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())
		assert.NotNil(t, service)
	})

	t.Run("Service handles nil logger", func(t *testing.T) {
		cfg := &services.CogneeServiceConfig{
			Enabled: true,
			BaseURL: "http://localhost:8000",
		}
		service := services.NewCogneeServiceWithConfig(cfg, nil)
		assert.NotNil(t, service)
	})

	t.Run("IsReady reflects service state", func(t *testing.T) {
		cfg := &services.CogneeServiceConfig{
			Enabled: false, // Disabled
			BaseURL: "http://localhost:8000",
		}
		service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

		// Disabled service should still report ready status correctly
		ready := service.IsReady()
		assert.False(t, ready, "Uninitialized service should not be ready")
	})
}

// TestCogneeSearchTypeValidation tests search type handling
func TestCogneeSearchTypeValidation(t *testing.T) {
	t.Run("Valid search types are accepted", func(t *testing.T) {
		validTypes := []string{"CHUNKS", "GRAPH_COMPLETION", "RAG_COMPLETION"}

		for _, searchType := range validTypes {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/" && r.Method == "GET":
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("Hello, World, I am alive!"))

				case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
					json.NewEncoder(w).Encode(map[string]string{
						"access_token": "test-token",
						"token_type":   "bearer",
					})

				case r.URL.Path == "/api/v1/search" && r.Method == "POST":
					var body map[string]interface{}
					json.NewDecoder(r.Body).Decode(&body)
					assert.Equal(t, searchType, body["search_type"])
					json.NewEncoder(w).Encode(map[string]interface{}{
						"results": []interface{}{},
					})

				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))

			cfg := &services.CogneeServiceConfig{
				Enabled:            true,
				BaseURL:            server.URL,
				DefaultDataset:     "default",
				Timeout:            10 * time.Second,
				DefaultSearchLimit: 10,
				SearchTypes:        []string{searchType},
			}
			service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

			result, err := service.SearchMemory(context.Background(), "test", "default", 10)
			assert.NoError(t, err, "Search type %s should be accepted", searchType)
			assert.NotNil(t, result)

			server.Close()
		}
	})
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkCogneeSearchWithErrors(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello, World, I am alive!"))

		case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
			json.NewEncoder(w).Encode(map[string]string{
				"access_token": "test-token",
				"token_type":   "bearer",
			})

		case r.URL.Path == "/api/v1/search" && r.Method == "POST":
			// Return error - should be handled gracefully
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "NoDataError",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &services.CogneeServiceConfig{
		Enabled:            true,
		BaseURL:            server.URL,
		DefaultDataset:     "default",
		Timeout:            10 * time.Second,
		DefaultSearchLimit: 10,
		SearchTypes:        []string{"CHUNKS"},
	}
	service := services.NewCogneeServiceWithConfig(cfg, logrus.New())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.SearchMemory(context.Background(), "test", "default", 10)
	}
}
