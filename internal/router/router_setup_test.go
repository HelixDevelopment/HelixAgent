//go:build integration
// +build integration

package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// skipIfNoDatabase skips the test if database is not available
func skipIfNoDatabase(t *testing.T) {
	t.Helper()

	// Check for database environment variables
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}
}

// TestSetupRouter_Integration tests the actual SetupRouter function with a real database
func TestSetupRouter_Integration(t *testing.T) {
	skipIfNoDatabase(t)

	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret:      "test-secret-key-12345678901234567890",
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			TokenExpiry:    24 * time.Hour,
			Port:           "7061",
			Host:           "0.0.0.0",
			EnableCORS:     true,
			CORSOrigins:    []string{"*"},
			RequestLogging: true,
		},
		Database: config.DatabaseConfig{
			Host:           os.Getenv("DB_HOST"),
			Port:           os.Getenv("DB_PORT"),
			User:           os.Getenv("DB_USER"),
			Password:       os.Getenv("DB_PASSWORD"),
			Name:           os.Getenv("DB_NAME"),
			SSLMode:        "disable",
			MaxConnections: 10,
			ConnTimeout:    5 * time.Second,
		},
		Redis: config.RedisConfig{
			Host:     os.Getenv("REDIS_HOST"),
			Port:     os.Getenv("REDIS_PORT"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0,
			PoolSize: 10,
		},
		Cognee: config.CogneeConfig{
			Enabled: false,
		},
		ModelsDev: config.ModelsDevConfig{
			Enabled: false,
		},
		LLM: config.LLMConfig{
			DefaultTimeout: 30 * time.Second,
			MaxRetries:     3,
			Providers:      map[string]config.ProviderConfig{},
		},
		MCP: config.MCPConfig{
			Enabled: false,
		},
	}

	t.Run("creates router with all endpoints", func(t *testing.T) {
		router := SetupRouter(cfg)
		require.NotNil(t, router)

		// Test health endpoint
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test v1 health endpoint
		w = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/v1/health", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test models endpoint (public)
		w = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/v1/models", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test providers endpoint (public)
		w = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/v1/providers", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("protected endpoints require authentication", func(t *testing.T) {
		router := SetupRouter(cfg)
		require.NotNil(t, router)

		// Test completions endpoint without auth
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/v1/completions", nil)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		// Should return 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("auth endpoints accept requests", func(t *testing.T) {
		router := SetupRouter(cfg)
		require.NotNil(t, router)

		// Test login endpoint
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/v1/auth/login", nil)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		// Should return 400 (bad request) without proper JSON body
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnauthorized)
	})
}

// TestNewGinRouter_Integration tests NewGinRouter with actual database
func TestNewGinRouter_Integration(t *testing.T) {
	skipIfNoDatabase(t)

	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret:      "test-secret-key-12345678901234567890",
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			TokenExpiry:    24 * time.Hour,
			Port:           "7061",
			Host:           "0.0.0.0",
			EnableCORS:     true,
			CORSOrigins:    []string{"*"},
			RequestLogging: true,
		},
		Database: config.DatabaseConfig{
			Host:           os.Getenv("DB_HOST"),
			Port:           os.Getenv("DB_PORT"),
			User:           os.Getenv("DB_USER"),
			Password:       os.Getenv("DB_PASSWORD"),
			Name:           os.Getenv("DB_NAME"),
			SSLMode:        "disable",
			MaxConnections: 10,
			ConnTimeout:    5 * time.Second,
		},
		Redis: config.RedisConfig{
			Host:     os.Getenv("REDIS_HOST"),
			Port:     os.Getenv("REDIS_PORT"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0,
			PoolSize: 10,
		},
		Cognee: config.CogneeConfig{
			Enabled: false,
		},
		ModelsDev: config.ModelsDevConfig{
			Enabled: false,
		},
		LLM: config.LLMConfig{
			DefaultTimeout: 30 * time.Second,
			MaxRetries:     3,
			Providers:      map[string]config.ProviderConfig{},
		},
		MCP: config.MCPConfig{
			Enabled: false,
		},
	}

	t.Run("creates GinRouter with all options", func(t *testing.T) {
		customLog := logrus.New()
		customLog.SetLevel(logrus.WarnLevel)

		router := NewGinRouter(cfg, WithLogger(customLog), WithGinMode(gin.TestMode))
		require.NotNil(t, router)
		assert.NotNil(t, router.Engine())
		assert.Equal(t, customLog, router.log)
		assert.False(t, router.IsRunning())
	})

	t.Run("engine serves requests", func(t *testing.T) {
		router := NewGinRouter(cfg)
		require.NotNil(t, router)

		engine := router.Engine()
		require.NotNil(t, engine)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/health", nil)
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("request counter works", func(t *testing.T) {
		router := NewGinRouter(cfg)
		require.NotNil(t, router)

		// Make several requests
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/health", nil)
			router.ServeHTTP(w, req)
		}

		stats := router.GetStats()
		assert.Equal(t, int64(5), stats.RequestCount)
	})
}
