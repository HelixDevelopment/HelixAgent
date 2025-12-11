package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/cache"
	"github.com/superagent/superagent/internal/config"
)

func TestDefaultKeyFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/test", func(c *gin.Context) {
		key := defaultKeyFunc(c)
		c.JSON(http.StatusOK, gin.H{"key": key})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
}

func TestByUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/test", func(c *gin.Context) {
		// Test with user ID set
		c.Set("user_id", "user123")
		key := ByUserID(c)
		c.JSON(http.StatusOK, gin.H{"key": key})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
}

func TestByAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/test", func(c *gin.Context) {
		key := ByAPIKey(c)
		c.JSON(http.StatusOK, gin.H{"key": key})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "test-api-key-123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
}

func TestMaxFunction(t *testing.T) {
	testCases := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{"a_greater", 10, 5, 10},
		{"b_greater", 5, 10, 10},
		{"equal", 10, 10, 10},
		{"negative", -5, -10, -5},
		{"mixed", -5, 10, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := max(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("max(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestNewRateLimiter(t *testing.T) {
	// Create a mock config
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       0,
		},
	}

	// Create cache service (will be disabled since Redis is not running)
	cacheService, err := cache.NewCacheService(cfg)
	if err == nil {
		t.Fatal("Expected error when creating cache service without Redis")
	}

	// The cache service should still be created but disabled
	if cacheService == nil {
		t.Fatal("Expected cache service instance even with Redis error")
	}

	// Create rate limiter with disabled cache
	limiter := NewRateLimiter(cacheService)
	if limiter == nil {
		t.Fatal("Expected rate limiter instance, got nil")
	}

	// Check default configuration
	if limiter.defaultCfg == nil {
		t.Fatal("Expected default configuration")
	}

	if limiter.defaultCfg.Requests != 100 {
		t.Errorf("Expected default 100 requests, got %d", limiter.defaultCfg.Requests)
	}

	if limiter.defaultCfg.Window != time.Minute {
		t.Errorf("Expected default 1 minute window, got %v", limiter.defaultCfg.Window)
	}
}

func TestAddLimit(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       0,
		},
	}

	cacheService, _ := cache.NewCacheService(cfg)
	limiter := NewRateLimiter(cacheService)

	// Add a limit for a specific path
	limiter.AddLimit("/api/test", &RateLimitConfig{
		Requests: 10,
		Window:   30 * time.Second,
		KeyFunc:  defaultKeyFunc,
	})

	// Verify the limit was added
	config := limiter.getConfig("/api/test")
	if config == nil {
		t.Fatal("Expected configuration for /api/test")
	}

	if config.Requests != 10 {
		t.Errorf("Expected 10 requests, got %d", config.Requests)
	}

	if config.Window != 30*time.Second {
		t.Errorf("Expected 30 second window, got %v", config.Window)
	}

	// Test getting default config for unknown path
	defaultConfig := limiter.getConfig("/unknown/path")
	if defaultConfig == nil {
		t.Fatal("Expected default configuration for unknown path")
	}

	if defaultConfig.Requests != 100 {
		t.Errorf("Expected default 100 requests for unknown path, got %d", defaultConfig.Requests)
	}
}

func TestGetConfig(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       0,
		},
	}

	cacheService, _ := cache.NewCacheService(cfg)
	limiter := NewRateLimiter(cacheService)

	// Test cases
	testCases := []struct {
		name     string
		path     string
		addLimit bool
		expected int
	}{
		{"DefaultPath", "/default", false, 100},
		{"CustomPath", "/custom", true, 50},
		{"NestedPath", "/api/v1/users", true, 20},
		{"WildcardMatch", "/api/v1/users/123", false, 100}, // Should use default
	}

	// Add custom limits
	limiter.AddLimit("/custom", &RateLimitConfig{
		Requests: 50,
		Window:   time.Minute,
		KeyFunc:  defaultKeyFunc,
	})

	limiter.AddLimit("/api/v1/users", &RateLimitConfig{
		Requests: 20,
		Window:   2 * time.Minute,
		KeyFunc:  ByUserID,
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := limiter.getConfig(tc.path)
			if config == nil {
				t.Fatal("Expected configuration")
			}

			if config.Requests != tc.expected {
				t.Errorf("Expected %d requests for %s, got %d", tc.expected, tc.path, config.Requests)
			}
		})
	}
}

func TestRateLimitConfig(t *testing.T) {
	config := &RateLimitConfig{
		Requests: 100,
		Window:   time.Minute,
		KeyFunc:  defaultKeyFunc,
	}

	if config.Requests != 100 {
		t.Errorf("Expected 100 requests, got %d", config.Requests)
	}

	if config.Window != time.Minute {
		t.Errorf("Expected 1 minute window, got %v", config.Window)
	}

	if config.KeyFunc == nil {
		t.Error("Expected KeyFunc to be set")
	}
}

func TestRateLimitResult(t *testing.T) {
	now := time.Now()
	result := &RateLimitResult{
		Allowed:    true,
		Remaining:  5,
		ResetTime:  now.Add(time.Minute),
		RetryAfter: 0,
	}

	if !result.Allowed {
		t.Error("Expected request to be allowed")
	}

	if result.Remaining != 5 {
		t.Errorf("Expected 5 remaining requests, got %d", result.Remaining)
	}

	if result.ResetTime.Before(now) {
		t.Error("Reset time should be in the future")
	}

	if result.RetryAfter != 0 {
		t.Errorf("Expected RetryAfter 0 for allowed request, got %d", result.RetryAfter)
	}
}

func TestKeyFuncs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setup    func(*gin.Context)
		expected string
	}{
		{
			name: "defaultKeyFunc with IP",
			setup: func(c *gin.Context) {
				c.Request.RemoteAddr = "192.168.1.1:8080"
			},
			expected: "ip:192.168.1.1",
		},
		{
			name: "ByUserID with user ID",
			setup: func(c *gin.Context) {
				c.Set("user_id", "test-user-123")
			},
			expected: "user:test-user-123",
		},
		{
			name: "ByAPIKey with API key",
			setup: func(c *gin.Context) {
				c.Request.Header.Set("X-API-Key", "api-key-456")
			},
			expected: "apikey:api-key-456",
		},
		{
			name: "ByUserID falls back to default",
			setup: func(c *gin.Context) {
				// Don't set user_id
				c.Request.RemoteAddr = "10.0.0.1:9090"
			},
			expected: "ip:10.0.0.1",
		},
		{
			name: "ByAPIKey falls back to default",
			setup: func(c *gin.Context) {
				// Don't set API key
				c.Request.RemoteAddr = "172.16.0.1:7070"
			},
			expected: "ip:172.16.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			var actualKey string

			router.GET("/test", func(c *gin.Context) {
				tt.setup(c)

				switch tt.name {
				case "defaultKeyFunc with IP":
					actualKey = defaultKeyFunc(c)
				case "ByUserID with user ID", "ByUserID falls back to default":
					actualKey = ByUserID(c)
				case "ByAPIKey with API key", "ByAPIKey falls back to default":
					actualKey = ByAPIKey(c)
				}

				c.JSON(http.StatusOK, gin.H{"key": actualKey})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", w.Code)
			}

			if actualKey != tt.expected {
				t.Errorf("Expected key %q, got %q", tt.expected, actualKey)
			}
		})
	}
}
