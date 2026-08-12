package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/cache"
	"github.com/gin-gonic/gin"
)

// RateLimiter implements rate limiting with in-memory storage
// Falls back to in-memory when Redis is unavailable.
//
// limits: Pattern Alpha (*RateLimitConfig values are immutable).
// buckets: Pattern Beta at the outer level — the bucket pointer is
// stable after PutIfAbsent; mutable state (tokens, lastRefill) lives
// behind tokenBucket.mu.
type RateLimiter struct {
	cache      *cache.CacheService
	limits     *safe.Store[string, *RateLimitConfig]
	defaultCfg *RateLimitConfig
	// In-memory storage for rate limit tracking
	buckets *safe.Store[string, *tokenBucket]
	// Lifecycle management for cleanup goroutine
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
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
	ctx, cancel := context.WithCancel(context.Background())
	rl := &RateLimiter{
		cache:   cacheService,
		limits:  safe.NewStore[string, *RateLimitConfig](),
		buckets: safe.NewStore[string, *tokenBucket](),
		ctx:     ctx,
		cancel:  cancel,
		defaultCfg: &RateLimitConfig{
			Requests: 100,
			Window:   time.Minute,
			KeyFunc:  defaultKeyFunc,
		},
	}

	// Start cleanup goroutine for expired buckets with lifecycle tracking
	rl.wg.Add(1)
	go rl.cleanupExpiredBuckets()

	return rl
}

// NewRateLimiterWithConfig creates a rate limiter with custom default config
func NewRateLimiterWithConfig(cacheService *cache.CacheService, defaultConfig *RateLimitConfig) *RateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &RateLimiter{
		cache:      cacheService,
		limits:     safe.NewStore[string, *RateLimitConfig](),
		buckets:    safe.NewStore[string, *tokenBucket](),
		defaultCfg: defaultConfig,
		ctx:        ctx,
		cancel:     cancel,
	}

	if rl.defaultCfg.KeyFunc == nil {
		rl.defaultCfg.KeyFunc = defaultKeyFunc
	}

	// Start cleanup goroutine for expired buckets with lifecycle tracking
	rl.wg.Add(1)
	go rl.cleanupExpiredBuckets()

	return rl
}

// cleanupExpiredBuckets periodically removes stale token buckets.
// Exits when the RateLimiter context is cancelled via Stop().
func (rl *RateLimiter) cleanupExpiredBuckets() {
	defer rl.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			for _, key := range rl.buckets.Keys() {
				rl.buckets.Update(key, func(b *tokenBucket, ok bool) (*tokenBucket, bool) {
					if !ok {
						return nil, false
					}
					b.mu.Lock()
					expired := now.Sub(b.lastRefill) > 10*time.Minute
					b.mu.Unlock()
					if expired {
						return nil, false // delete
					}
					return b, true
				})
			}
		}
	}
}

// Stop gracefully stops the rate limiter cleanup goroutine and waits for it to finish.
func (rl *RateLimiter) Stop() {
	rl.cancel()
	rl.wg.Wait()
}

// AddLimit adds a rate limit for a specific path
func (rl *RateLimiter) AddLimit(path string, config *RateLimitConfig) {
	rl.limits.Put(path, config)
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
	if bucket, exists := rl.buckets.Get(key); exists {
		return bucket
	}
	bucket := &tokenBucket{
		tokens:     config.Requests,
		maxTokens:  config.Requests,
		refillRate: config.Window,
		lastRefill: time.Now(),
	}
	cached, _ := rl.buckets.PutIfAbsent(key, bucket)
	return cached
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
	if config, exists := rl.limits.Get(path); exists {
		return config
	}
	return rl.defaultCfg
}

// defaultKeyFunc generates a default rate limit key based on IP address.
//
// HXC-283: the caller's identity MUST be taken apart with the standard
// host/port routine (net.SplitHostPort), never a home-grown split and never
// by trusting an unparsed fallback string, so one real caller always yields
// exactly one rate-limit identity, regardless of the shape its address
// happens to arrive in.
//
// gin.Context.ClientIP() already runs net.SplitHostPort internally and also
// honours any configured trusted-proxy forwarding headers, so it is the
// preferred path whenever it resolves to a bare numeric IP. It legitimately
// returns "" not only for malformed input but also for entirely real
// addresses it does not itself resolve to a bare IP: an RFC-4007
// zone-qualified IPv6 literal such as "fe80::1%eth0" (net.ParseIP rejects
// the zone suffix), or a hostname-shaped RemoteAddr. The PRE-FIX code fell
// through to the raw, unparsed c.Request.RemoteAddr on that path, which
// leaked the ephemeral source port (and, for a bracketed IPv6 literal, the
// square brackets) into the identity — and since the source port differs on
// every TCP connection, the SAME caller derived a brand-new identity (and
// therefore a fresh, full token bucket) on every single request, bypassing
// the limit entirely. This is closed below by applying net.SplitHostPort
// ourselves whenever ClientIP() could not, rather than ever trusting the
// unparsed string.
func defaultKeyFunc(c *gin.Context) string {
	if ip := c.ClientIP(); ip != "" {
		return "ip:" + ip
	}

	raw := c.Request.RemoteAddr

	// ClientIP() failed. Take the address apart ourselves with the standard
	// routine rather than falling through to the raw string. This succeeds
	// for exactly the cases ClientIP() rejects but SplitHostPort itself can
	// still split cleanly: hostnames and zone-qualified IPv6 literals, both
	// with a port present. net.SplitHostPort always returns the host
	// UNBRACKETED, so no extra bracket-stripping is needed here.
	if host, _, err := net.SplitHostPort(raw); err == nil {
		if host == "" {
			// A port was present but the host portion was empty (e.g.
			// ":54321"). There is no caller identity to extract here at
			// all; see the "unknown" decision below for why this is
			// deliberately merged rather than left to leak the port.
			return "ip:unknown"
		}
		return "ip:" + host
	}

	// SplitHostPort itself failed: there is no ":port" suffix to remove at
	// all. This is not automatically an error in the caller's identity —
	// a bare host with no port needs nothing stripped from it — so it is
	// used AS-IS, deliberately, rather than collapsed onto a shared
	// placeholder that would merge it with every other malformed caller
	// (that collapse would itself be a new bypass: an attacker could then
	// share a bucket, and therefore a quota, with unrelated callers,
	// starving them, or with themselves reduce their own bucket contention
	// to one collective count in a way the limit was never sized for). The
	// one exception this file still strips is a redundant pair of brackets
	// around an otherwise-bare host ("[2001:db8::1]" with no port at all),
	// so that degenerate shape still resolves to the same identity as its
	// "[2001:db8::1]:<port>" sibling above, rather than a second, distinct
	// one that would split the same real caller in two.
	if raw == "" {
		// A genuinely empty address carries no identity to extract at all.
		// Rather than let that silently collapse onto the same blank
		// string every other malformed caller happens to also produce
		// (which is what the pre-fix fallback did by accident), every
		// caller with no address information is deliberately, explicitly
		// merged onto one shared, still-rate-limited "unknown" identity —
		// fail CLOSED (still throttled as a group) rather than fail OPEN
		// (no address, no limit).
		return "ip:unknown"
	}
	if len(raw) >= 2 && raw[0] == '[' && raw[len(raw)-1] == ']' {
		return "ip:" + raw[1:len(raw)-1]
	}
	return "ip:" + raw
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
func max(a, b int) int { //nolint:unused
	if a > b {
		return a
	}
	return b
}
