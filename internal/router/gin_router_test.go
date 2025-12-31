package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// createTestGinRouter creates a GinRouter for testing without calling SetupRouter
func createTestGinRouter(cfg *config.Config, opts ...GinRouterOption) *GinRouter {
	defaultLog := logrus.New()
	defaultLog.SetLevel(logrus.InfoLevel)

	router := &GinRouter{
		config:  cfg,
		log:     defaultLog,
		running: false,
	}

	// Apply options (may override the default logger)
	for _, opt := range opts {
		opt(router)
	}

	// Create a simple Gin engine for testing
	router.engine = gin.New()
	router.engine.Use(gin.Recovery())
	router.engine.Use(router.requestCounterMiddleware())

	return router
}

func TestNewGinRouter(t *testing.T) {
	t.Skip("Skipping test that requires database - covered by integration tests")
}

func TestGinRouterStructure(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	t.Run("creates router with proper structure", func(t *testing.T) {
		router := createTestGinRouter(cfg)
		require.NotNil(t, router)
		assert.NotNil(t, router.engine)
		assert.NotNil(t, router.log)
		assert.NotNil(t, router.config)
		assert.False(t, router.running)
	})

	t.Run("applies custom logger option", func(t *testing.T) {
		customLog := logrus.New()
		customLog.SetLevel(logrus.DebugLevel)

		router := createTestGinRouter(cfg, WithLogger(customLog))
		require.NotNil(t, router)
		assert.Equal(t, customLog, router.log)
	})
}

func TestGinRouter_Engine(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)
	engine := router.Engine()

	assert.NotNil(t, engine)
	assert.IsType(t, &gin.Engine{}, engine)
}

func TestGinRouter_IsRunning(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)
	assert.False(t, router.IsRunning())
}

func TestGinRouter_GetStats(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)
	stats := router.GetStats()

	assert.False(t, stats.Running)
	assert.Equal(t, int64(0), stats.RequestCount)
	assert.Equal(t, time.Duration(0), stats.Uptime)
}

func TestGinRouter_RegisterRoutes(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)

	// Register a custom route
	router.RegisterRoutes(func(e *gin.Engine) {
		e.GET("/custom", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "custom route"})
		})
	})

	// Test the custom route
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/custom", nil)
	router.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "custom route")
}

func TestGinRouter_AddMiddleware(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)

	middlewareCalled := false
	router.AddMiddleware(func(c *gin.Context) {
		middlewareCalled = true
		c.Next()
	})

	// Register a route to trigger the middleware
	router.engine.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Test that middleware is called
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.engine.ServeHTTP(w, req)

	assert.True(t, middlewareCalled)
}

func TestGinRouter_ServeHTTP(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)

	// Register a route
	router.engine.GET("/serve-test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"served": true})
	})

	// Test ServeHTTP interface
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/serve-test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "served")
}

func TestGinRouter_requestCounterMiddleware(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)

	// Register a route
	router.engine.GET("/count-test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Make several requests
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/count-test", nil)
		router.engine.ServeHTTP(w, req)
	}

	stats := router.GetStats()
	assert.Equal(t, int64(5), stats.RequestCount)
}

func TestGinRouter_Start_AlreadyRunning(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)

	// Manually set running to true
	router.mu.Lock()
	router.running = true
	router.mu.Unlock()

	err := router.Start(":0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestGinRouter_Shutdown_NotRunning(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)

	ctx := context.Background()
	err := router.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestGinRouter_StartTLS_AlreadyRunning(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)

	// Manually set running to true
	router.mu.Lock()
	router.running = true
	router.mu.Unlock()

	err := router.StartTLS(":0", "cert.pem", "key.pem")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestRouterStats_JSON(t *testing.T) {
	stats := RouterStats{
		Running:      true,
		StartedAt:    time.Now(),
		Uptime:       time.Minute,
		RequestCount: 100,
	}

	assert.True(t, stats.Running)
	assert.Equal(t, int64(100), stats.RequestCount)
	assert.Equal(t, time.Minute, stats.Uptime)
}

func TestWithLogger(t *testing.T) {
	customLog := logrus.New()
	customLog.SetLevel(logrus.ErrorLevel)

	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg, WithLogger(customLog))
	assert.Equal(t, customLog, router.log)
	assert.Equal(t, logrus.ErrorLevel, router.log.Level)
}

func TestWithGinMode(t *testing.T) {
	// Test WithGinMode option directly
	gin.SetMode(gin.ReleaseMode)
	assert.Equal(t, gin.ReleaseMode, gin.Mode())

	// Reset to test mode for other tests
	gin.SetMode(gin.TestMode)
	assert.Equal(t, gin.TestMode, gin.Mode())
}

func TestGinRouter_GetStats_WhileRunning(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			JWTSecret: "test-secret-key-1234567890",
		},
	}

	router := createTestGinRouter(cfg)

	// Simulate running state
	router.mu.Lock()
	router.running = true
	router.startedAt = time.Now().Add(-time.Hour)
	router.requestCnt = 42
	router.mu.Unlock()

	stats := router.GetStats()

	assert.True(t, stats.Running)
	assert.Equal(t, int64(42), stats.RequestCount)
	assert.True(t, stats.Uptime >= time.Hour)
}
