// NOTE: Overlap evaluation (2026-03-23)
//
// This package (internal/http) provides CLIENT-SIDE HTTP functionality:
//   - pool.go: HTTP/1.1 connection pooling, per-host client management, retry
//     with exponential backoff, request metrics, global singleton pool.
//   - quic_client.go: HTTP/3 QUIC client via quic-go, HTTP/2 fallback,
//     HTTP3ProviderTransport (http.RoundTripper), QUIC-specific metrics.
//
// Related SERVER-SIDE packages exist but serve a different role:
//   - internal/transport/http3.go: HTTP/3 server with self-signed cert gen.
//   - internal/router/quic_server.go: Dual-stack HTTP/3 + HTTP/1.1/2 server.
//
// Conclusion: NOT duplicated. Client vs server. Both are needed.
// Bridged via internal/adapters/http/adapter.go.
package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// PoolConfig holds configuration for the HTTP client pool
type PoolConfig struct {
	// Connection settings
	MaxIdleConns        int
	MaxConnsPerHost     int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration

	// Timeouts
	DialTimeout           time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	ExpectContinueTimeout time.Duration

	// Keep-alive
	DisableKeepAlives  bool
	DisableCompression bool
	KeepAliveInterval  time.Duration

	// TLS
	TLSConfig          *tls.Config
	InsecureSkipVerify bool

	// Retry
	RetryCount     int
	RetryWaitMin   time.Duration
	RetryWaitMax   time.Duration
	RetryCondition func(*http.Response, error) bool
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxIdleConns:          100,
		MaxConnsPerHost:       10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DialTimeout:           30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		KeepAliveInterval:     30 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		RetryCount:            3,
		RetryWaitMin:          100 * time.Millisecond,
		RetryWaitMax:          2 * time.Second,
	}
}

// PoolMetrics tracks HTTP client pool statistics
type PoolMetrics struct {
	TotalRequests     int64
	SuccessRequests   int64
	FailedRequests    int64
	RetryCount        int64
	TotalLatencyUs    int64
	RequestCount      int64
	ActiveRequests    int64
	ConnectionsReused int64
}

// AverageLatency returns the average request latency
func (m *PoolMetrics) AverageLatency() time.Duration {
	count := atomic.LoadInt64(&m.RequestCount)
	if count == 0 {
		return 0
	}
	totalUs := atomic.LoadInt64(&m.TotalLatencyUs)
	return time.Duration(totalUs/count) * time.Microsecond
}

// HTTPClientPool manages reusable HTTP clients per host
type HTTPClientPool struct {
	clients   map[string]*http.Client
	mu        sync.RWMutex
	config    *PoolConfig
	metrics   *PoolMetrics
	transport *http.Transport
}

// NewHTTPClientPool creates a new HTTP client pool
func NewHTTPClientPool(config *PoolConfig) *HTTPClientPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	// Create base transport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAliveInterval,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		DisableCompression:    config.DisableCompression,
	}

	if config.TLSConfig != nil {
		transport.TLSClientConfig = config.TLSConfig
	} else if config.InsecureSkipVerify {
		// User explicitly requested to skip TLS verification (e.g., for internal services with self-signed certs)
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 - intentional config option
	}

	return &HTTPClientPool{
		clients:   make(map[string]*http.Client),
		config:    config,
		metrics:   &PoolMetrics{},
		transport: transport,
	}
}

// GetClient returns an HTTP client for the given host
// If a client doesn't exist for the host, a new one is created
func (p *HTTPClientPool) GetClient(host string) *http.Client {
	p.mu.RLock()
	client, exists := p.clients[host]
	p.mu.RUnlock()

	if exists {
		return client
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists = p.clients[host]; exists {
		return client
	}

	// Create new client with a cloned transport for per-host isolation.
	// Clone() copies TLS config and dial settings but gives each host its
	// own idle-connection pool, preventing one host's connection state from
	// leaking into another's.
	client = &http.Client{
		Transport: p.transport.Clone(),
		Timeout:   p.config.ResponseHeaderTimeout + p.config.DialTimeout,
	}

	p.clients[host] = client
	return client
}

// GetClientForURL returns an HTTP client for the given URL
func (p *HTTPClientPool) GetClientForURL(urlStr string) (*http.Client, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return p.GetClient(parsed.Host), nil
}

// Do executes an HTTP request with retry logic
func (p *HTTPClientPool) Do(req *http.Request) (*http.Response, error) {
	return p.DoWithContext(req.Context(), req)
}

// DoWithContext executes an HTTP request with retry logic and context
func (p *HTTPClientPool) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	atomic.AddInt64(&p.metrics.TotalRequests, 1)
	atomic.AddInt64(&p.metrics.ActiveRequests, 1)
	defer atomic.AddInt64(&p.metrics.ActiveRequests, -1)

	startTime := time.Now()
	client := p.GetClient(req.URL.Host)

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= p.config.RetryCount; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			atomic.AddInt64(&p.metrics.FailedRequests, 1)
			return nil, ctx.Err()
		default:
		}

		// Clone request for retry
		reqCopy := req.Clone(ctx)
		if req.Body != nil && req.GetBody != nil {
			body, err2 := req.GetBody()
			if err2 != nil {
				return nil, fmt.Errorf("failed to get request body: %w", err2)
			}
			reqCopy.Body = body
		}

		resp, err = client.Do(reqCopy)

		// Check if we should retry
		shouldRetry := false
		if p.config.RetryCondition != nil {
			shouldRetry = p.config.RetryCondition(resp, err)
		} else {
			shouldRetry = p.defaultRetryCondition(resp, err)
		}

		if !shouldRetry {
			break
		}

		// Close response body before retry
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body) //nolint:errcheck
			_ = resp.Body.Close()
		}

		if attempt < p.config.RetryCount {
			atomic.AddInt64(&p.metrics.RetryCount, 1)

			// Calculate retry delay with exponential backoff
			delay := p.calculateRetryDelay(attempt)
			select {
			case <-ctx.Done():
				atomic.AddInt64(&p.metrics.FailedRequests, 1)
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	// Update metrics
	duration := time.Since(startTime)
	atomic.AddInt64(&p.metrics.TotalLatencyUs, duration.Microseconds())
	atomic.AddInt64(&p.metrics.RequestCount, 1)

	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		atomic.AddInt64(&p.metrics.FailedRequests, 1)
	} else {
		atomic.AddInt64(&p.metrics.SuccessRequests, 1)
	}

	return resp, err
}

// defaultRetryCondition determines if a request should be retried
func (p *HTTPClientPool) defaultRetryCondition(resp *http.Response, err error) bool {
	// Retry on connection errors
	if err != nil {
		return true
	}

	// Retry on server errors (5xx) and some client errors
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout,     // 504
			http.StatusBadGateway:         // 502
			return true
		}
	}

	return false
}

// calculateRetryDelay calculates the delay before the next retry
func (p *HTTPClientPool) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff with jitter
	delay := p.config.RetryWaitMin * time.Duration(1<<uint(attempt)) // #nosec G115 - attempt count is small
	if delay > p.config.RetryWaitMax {
		delay = p.config.RetryWaitMax
	}
	return delay
}

// Get performs a GET request
func (p *HTTPClientPool) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return p.Do(req)
}

// Post performs a POST request
func (p *HTTPClientPool) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return p.Do(req)
}

// PostJSON performs a POST request with JSON content type
func (p *HTTPClientPool) PostJSON(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	return p.Post(ctx, url, "application/json", body)
}

// Metrics returns current pool metrics
func (p *HTTPClientPool) Metrics() *PoolMetrics {
	return &PoolMetrics{
		TotalRequests:     atomic.LoadInt64(&p.metrics.TotalRequests),
		SuccessRequests:   atomic.LoadInt64(&p.metrics.SuccessRequests),
		FailedRequests:    atomic.LoadInt64(&p.metrics.FailedRequests),
		RetryCount:        atomic.LoadInt64(&p.metrics.RetryCount),
		TotalLatencyUs:    atomic.LoadInt64(&p.metrics.TotalLatencyUs),
		RequestCount:      atomic.LoadInt64(&p.metrics.RequestCount),
		ActiveRequests:    atomic.LoadInt64(&p.metrics.ActiveRequests),
		ConnectionsReused: atomic.LoadInt64(&p.metrics.ConnectionsReused),
	}
}

// ClientCount returns the number of clients in the pool
func (p *HTTPClientPool) ClientCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}

// CloseIdleConnections closes any idle connections
func (p *HTTPClientPool) CloseIdleConnections() {
	p.transport.CloseIdleConnections()
}

// Close closes the pool and all connections
func (p *HTTPClientPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.transport.CloseIdleConnections()
	p.clients = make(map[string]*http.Client)
	return nil
}

// GlobalPool is the default global HTTP client pool
var (
	GlobalPool     *HTTPClientPool
	globalPoolMu   sync.RWMutex
	globalPoolOnce sync.Once
)

func ensureGlobalPool() {
	globalPoolOnce.Do(func() {
		GlobalPool = NewHTTPClientPool(nil)
	})
}

// InitGlobalPool initializes the global HTTP client pool with custom config.
// Must be called before any concurrent access (typically during startup).
func InitGlobalPool(config *PoolConfig) {
	globalPoolMu.Lock()
	defer globalPoolMu.Unlock()
	if GlobalPool != nil {
		_ = GlobalPool.Close()
	}
	GlobalPool = NewHTTPClientPool(config)
	// Reset Once so ensureGlobalPool won't overwrite
	globalPoolOnce = sync.Once{}
	globalPoolOnce.Do(func() {}) // mark as done
}

// getGlobalPool returns the global pool safely
func getGlobalPool() *HTTPClientPool {
	globalPoolMu.RLock()
	p := GlobalPool
	globalPoolMu.RUnlock()
	if p != nil {
		return p
	}
	ensureGlobalPool()
	globalPoolMu.RLock()
	p = GlobalPool
	globalPoolMu.RUnlock()
	return p
}

// Get performs a GET request using the global pool
func Get(ctx context.Context, url string) (*http.Response, error) {
	pool := getGlobalPool()
	return pool.Get(ctx, url)
}

// PostJSON performs a POST request with JSON content using the global pool
func PostJSON(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	pool := getGlobalPool()
	return pool.PostJSON(ctx, url, body)
}

// HostClient provides a client pre-configured for a specific host
type HostClient struct {
	pool    *HTTPClientPool
	host    string
	baseURL string
	headers map[string]string
	mu      sync.RWMutex
}

// NewHostClient creates a client for a specific host
func NewHostClient(pool *HTTPClientPool, baseURL string) (*HostClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	return &HostClient{
		pool:    pool,
		host:    parsed.Host,
		baseURL: baseURL,
		headers: make(map[string]string),
	}, nil
}

// SetHeader sets a default header for all requests
func (c *HostClient) SetHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headers[key] = value
}

// Do performs a request with default headers
func (c *HostClient) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	c.mu.RUnlock()

	return c.pool.Do(req)
}

// Get performs a GET request
func (c *HostClient) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, path, nil)
}

// Post performs a POST request
func (c *HostClient) Post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, path, body)
}

// PostJSON performs a POST request with JSON content type
func (c *HostClient) PostJSON(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	c.SetHeader("Content-Type", "application/json")
	return c.Post(ctx, path, body)
}
