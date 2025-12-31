package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/cache"
)

// RateLimiter implements rate limiting with in-memory storage
// Falls back to in-memory when Redis is unavailable
type RateLimiter struct {
	cache      *cache.CacheService
	mu         sync.RWMutex
	limits     map[string]*RateLimitConfig
	defaultCfg *RateLimitConfig
	// In-memory storage for rate limit tracking
	buckets map[string]*tokenBucket
}

// tokenBucket implements a simple token bucket for rate limiting
type tokenBucket struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
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
	rl := &RateLimiter{
		cache:   cacheService,
		limits:  make(map[string]*RateLimitConfig),
		buckets: make(map[string]*tokenBucket),
		defaultCfg: &RateLimitConfig{
			Requests: 100,
			Window:   time.Minute,
			KeyFunc:  defaultKeyFunc,
		},
	}

	// Start cleanup goroutine for expired buckets
	go rl.cleanupExpiredBuckets()

	return rl
}

// NewRateLimiterWithConfig creates a rate limiter with custom default config
func NewRateLimiterWithConfig(cacheService *cache.CacheService, defaultConfig *RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		cache:      cacheService,
		limits:     make(map[string]*RateLimitConfig),
		buckets:    make(map[string]*tokenBucket),
		defaultCfg: defaultConfig,
	}

	if rl.defaultCfg.KeyFunc == nil {
		rl.defaultCfg.KeyFunc = defaultKeyFunc
	}

	// Start cleanup goroutine for expired buckets
	go rl.cleanupExpiredBuckets()

	return rl
}

// cleanupExpiredBuckets periodically removes stale token buckets
func (rl *RateLimiter) cleanupExpiredBuckets() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, bucket := range rl.buckets {
			bucket.mu.Lock()
			// Remove buckets that haven't been used in 10 minutes
			if now.Sub(bucket.lastRefill) > 10*time.Minute {
				delete(rl.buckets, key)
			}
			bucket.mu.Unlock()
		}
		rl.mu.Unlock()
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
		result, err := rl.checkLimit(key, config)
		if err != nil {
			// If rate limit check fails, allow request (fail open)
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

// checkLimit checks if the request is within rate limits using token bucket algorithm
func (rl *RateLimiter) checkLimit(key string, config *RateLimitConfig) (*RateLimitResult, error) {
	bucket := rl.getOrCreateBucket(key, config)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	now := time.Now()

	// Refill tokens based on time passed
	elapsed := now.Sub(bucket.lastRefill)
	tokensToAdd := int(elapsed / config.Window * time.Duration(config.Requests))
	if tokensToAdd > 0 {
		bucket.tokens = min(bucket.maxTokens, bucket.tokens+tokensToAdd)
		bucket.lastRefill = now
	}

	// Check if we have tokens available
	allowed := bucket.tokens > 0
	if allowed {
		bucket.tokens--
	}

	remaining := bucket.tokens
	resetTime := now.Add(config.Window)

	var retryAfter int
	if !allowed {
		// Calculate when the next token will be available
		retryAfter = int(config.Window.Seconds() / float64(config.Requests))
		if retryAfter < 1 {
			retryAfter = 1
		}
	}

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		ResetTime:  resetTime,
		RetryAfter: retryAfter,
	}, nil
}

// getOrCreateBucket gets or creates a token bucket for the given key
func (rl *RateLimiter) getOrCreateBucket(key string, config *RateLimitConfig) *tokenBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if bucket, exists := rl.buckets[key]; exists {
		return bucket
	}

	bucket := &tokenBucket{
		tokens:     config.Requests,
		maxTokens:  config.Requests,
		refillRate: config.Window,
		lastRefill: time.Now(),
	}
	rl.buckets[key] = bucket
	return bucket
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
