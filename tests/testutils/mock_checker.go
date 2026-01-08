// Package testutils provides utility functions for testing
package testutils

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

// MockConfig holds configuration for mock services
type MockConfig struct {
	MockLLMURL     string
	MockLLMEnabled bool
	PostgresURL    string
	RedisURL       string
	ServerURL      string
}

// GetMockConfig returns the mock service configuration from environment
func GetMockConfig() MockConfig {
	return MockConfig{
		MockLLMURL:     getEnvOrDefault("MOCK_LLM_URL", "http://localhost:18081"),
		MockLLMEnabled: os.Getenv("MOCK_LLM_ENABLED") == "true" || os.Getenv("CI") == "true",
		PostgresURL:    getEnvOrDefault("DATABASE_URL", "postgres://helixagent:helixagent123@localhost:15432/helixagent_db?sslmode=disable"),
		RedisURL:       getEnvOrDefault("REDIS_URL", "redis://:helixagent123@localhost:16379"),
		ServerURL:      getEnvOrDefault("SERVER_URL", "http://localhost:7061"),
	}
}

// IsMockLLMAvailable checks if the mock LLM server is running
func IsMockLLMAvailable() bool {
	cfg := GetMockConfig()
	return checkEndpoint(cfg.MockLLMURL + "/health")
}

// IsPostgresAvailable checks if PostgreSQL is running and accessible
func IsPostgresAvailable() bool {
	// Check environment variable first
	if os.Getenv("DB_HOST") == "" && os.Getenv("DATABASE_URL") == "" {
		return false
	}

	// Try to connect using pgx
	cfg := GetMockConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.PostgresURL)
	if err != nil {
		return false
	}
	defer conn.Close(ctx)

	return conn.Ping(ctx) == nil
}

// IsRedisAvailable checks if Redis is running
func IsRedisAvailable() bool {
	if os.Getenv("REDIS_HOST") == "" && os.Getenv("REDIS_URL") == "" {
		return false
	}
	// For simple check, assume available if environment is set
	// Real check would use redis client
	return true
}

// IsServerAvailable checks if the HelixAgent server is running
func IsServerAvailable() bool {
	return IsServerAvailableAt(GetMockConfig().ServerURL)
}

// IsServerAvailableAt checks if a server is running at the given URL
func IsServerAvailableAt(baseURL string) bool {
	return checkEndpoint(baseURL + "/health")
}

// IsDockerAvailable checks if Docker is available
func IsDockerAvailable() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	// Check Docker socket via API
	resp, err := client.Get("http://localhost:2375/version")
	if err == nil {
		resp.Body.Close()
		return true
	}
	// Also check if docker command exists (via /var/run/docker.sock)
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return true
	}
	return false
}

// IsTestInfrastructureAvailable checks if full test infrastructure is available
func IsTestInfrastructureAvailable() bool {
	return IsMockLLMAvailable() && IsPostgresAvailable() && IsRedisAvailable()
}

// IsFullTestEnvironment checks if we're running in a full test environment (CI or with infrastructure)
func IsFullTestEnvironment() bool {
	return os.Getenv("CI") == "true" || IsTestInfrastructureAvailable()
}

// GetMockLLMBaseURL returns the base URL for the mock LLM server
func GetMockLLMBaseURL() string {
	return getEnvOrDefault("MOCK_LLM_URL", "http://localhost:18081")
}

// GetMockAPIKey returns a mock API key for testing
func GetMockAPIKey() string {
	return "mock-api-key-for-testing"
}

// GetServerURL returns the server URL for testing
func GetServerURL() string {
	return getEnvOrDefault("SERVER_URL", "http://localhost:7061")
}

// GetDatabaseURL returns the database URL for testing
func GetDatabaseURL() string {
	return getEnvOrDefault("DATABASE_URL", "postgres://helixagent:helixagent123@localhost:15432/helixagent_db?sslmode=disable")
}

// RequireInfrastructure returns an error message if required infrastructure is not available
// Returns empty string if all infrastructure is available
func RequireInfrastructure(needs ...string) string {
	for _, need := range needs {
		switch need {
		case "database", "postgres":
			if !IsPostgresAvailable() {
				return fmt.Sprintf("Database not available. Set DB_HOST or DATABASE_URL environment variable.")
			}
		case "redis":
			if !IsRedisAvailable() {
				return fmt.Sprintf("Redis not available. Set REDIS_HOST or REDIS_URL environment variable.")
			}
		case "llm", "mock-llm":
			if !IsMockLLMAvailable() {
				return fmt.Sprintf("Mock LLM server not available at %s. Start with: docker-compose -f docker-compose.test.yml up -d mock-llm", GetMockLLMBaseURL())
			}
		case "server":
			if !IsServerAvailable() {
				return fmt.Sprintf("HelixAgent server not available at %s. Start with: make run-dev", GetServerURL())
			}
		case "docker":
			if !IsDockerAvailable() {
				return "Docker not available. Please install and start Docker."
			}
		}
	}
	return ""
}

// SkipIfNoInfrastructure skips the test if required infrastructure is not available
// but only when NOT in CI mode or full test mode
func SkipIfNoInfrastructure(t interface{ Skip(args ...interface{}) }, needs ...string) {
	// In CI or full test mode, never skip - fail instead if infrastructure is missing
	if os.Getenv("CI") == "true" || os.Getenv("FULL_TEST_MODE") == "true" {
		return // Don't skip, let the test run and fail if infrastructure is missing
	}

	// In local development, skip if infrastructure is not available
	if msg := RequireInfrastructure(needs...); msg != "" {
		t.Skip(msg)
	}
}

// FailIfNoInfrastructure fails the test if required infrastructure is not available
func FailIfNoInfrastructure(t interface {
	Fatalf(format string, args ...interface{})
}, needs ...string) {
	if msg := RequireInfrastructure(needs...); msg != "" {
		t.Fatalf("Required infrastructure not available: %s", msg)
	}
}

func checkEndpoint(url string) bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
