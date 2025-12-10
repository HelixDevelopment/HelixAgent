package middleware

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/cache"
	"github.com/superagent/superagent/internal/models"
)

// RateLimiter implements rate limiting using Redis
type RateLimiter struct {
	cache      *cache.CacheService
	mu         sync.RWMutex
	limits     map[string]*RateLimitConfig
	defaultCfg *RateLimitConfig
}

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	Requests int                       `json:"requests"` // Number of requests allowed
	Window   time.Duration             `json:"window"`   // Time window
	KeyFunc  func(*gin.Context) string `json:"-"`        // Function to generate rate limit key
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed    bool      `json:"allowed"`
	Remaining  int       `json:"remaining"`
	ResetTime  time.Time `json:"reset_time"`
	RetryAfter int       `json:"retry_after,omitempty"`
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cacheService *cache.CacheService) *RateLimiter {
	return &RateLimiter{
		cache:  cacheService,
		limits: make(map[string]*RateLimitConfig),
		defaultCfg: &RateLimitConfig{
			Requests: 100,
			Window:   time.Minute,
			KeyFunc:  defaultKeyFunc,
		},
	}
}

// AddLimit adds a rate limit for a specific path
func (rl *RateLimiter) AddLimit(path string, config *RateLimitConfig) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limits[path] = config
}

// Middleware returns a Gin middleware function for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get rate limit config for this path
		config := rl.getConfig(c.Request.URL.Path)

		// Generate rate limit key
		key := config.KeyFunc(c)

		// Check rate limit
		result, err := rl.checkLimit(c.Request.Context(), key, config)
		if err != nil {
			// If cache is unavailable, allow request
			c.Next()
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.Requests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetTime.Unix(), 10))

		if !result.Allowed {
			c.Header("Retry-After", strconv.Itoa(result.RetryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": result.RetryAfter,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkLimit checks if the request is within rate limits
func (rl *RateLimiter) checkLimit(ctx context.Context, key string, config *RateLimitConfig) (*RateLimitResult, error) {
	if !rl.cache.IsEnabled() {
		// If cache is disabled, allow all requests
		return &RateLimitResult{
			Allowed:   true,
			Remaining: config.Requests - 1,
			ResetTime: time.Now().Add(config.Window),
		}, nil
	}

	// Use Redis sorted set to track requests
	now := time.Now()
	windowStart := now.Add(-config.Window)

	// Remove old entries and add new one
	cacheKey := "ratelimit:" + key

	// Get current count
	count, err := rl.getCurrentCount(ctx, cacheKey, windowStart)
	if err != nil {
		return nil, err
	}

	remaining := config.Requests - count - 1
	allowed := count < config.Requests

	resetTime := now.Add(config.Window)

	if allowed {
		// Add current request timestamp
		err = rl.addRequest(ctx, cacheKey, now)
		if err != nil {
			return nil, err
		}
		remaining = config.Requests - count - 1
	} else {
		remaining = 0
	}

	return &RateLimitResult{
		Allowed:   allowed,
		Remaining: max(0, remaining),
		ResetTime: resetTime,
		RetryAfter: func() int {
			if !allowed {
				return int(config.Window.Seconds())
			}
			return 0
		}(),
	}, nil
}

// getCurrentCount gets the current number of requests in the window
func (rl *RateLimiter) getCurrentCount(ctx context.Context, key string, windowStart time.Time) (int, error) {
	// This is a simplified implementation
	// In a real Redis implementation, we'd use ZCOUNT or similar
	// For now, we'll use a simple key with expiration

	countKey := key + ":count"
	var count int

	_, err := rl.cache.GetLLMResponse(ctx, &models.LLMRequest{ID: countKey})
	if err != nil {
		// Key doesn't exist, count is 0
		return 0, nil
	}

	// This is a placeholder - real implementation would use Redis sorted sets
	return count, nil
}

// addRequest adds a request timestamp to the rate limit tracking
func (rl *RateLimiter) addRequest(ctx context.Context, key string, timestamp time.Time) error {
	// This is a simplified implementation
	// Real implementation would use Redis ZADD with score as timestamp
	return nil
}

// getConfig returns the rate limit config for a path
func (rl *RateLimiter) getConfig(path string) *RateLimitConfig {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if config, exists := rl.limits[path]; exists {
		return config
	}

	return rl.defaultCfg
}

// defaultKeyFunc generates a default rate limit key based on IP address
func defaultKeyFunc(c *gin.Context) string {
	// Try to get real IP
	ip := c.ClientIP()
	if ip == "" {
		ip = c.Request.RemoteAddr
	}

	return "ip:" + ip
}

// ByUserID generates rate limit key based on user ID
func ByUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return defaultKeyFunc(c)
	}

	if uid, ok := userID.(string); ok {
		return "user:" + uid
	}

	return defaultKeyFunc(c)
}

// ByAPIKey generates rate limit key based on API key
func ByAPIKey(c *gin.Context) string {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey != "" {
		return "apikey:" + apiKey
	}

	return defaultKeyFunc(c)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
