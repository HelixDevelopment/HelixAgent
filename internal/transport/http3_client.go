package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/quic-go/quic-go/http3"
)

// HTTP3ClientConfig holds configuration for the HTTP/3 client
type HTTP3ClientConfig struct {
	// EnableHTTP3 enables HTTP/3 support (QUIC)
	EnableHTTP3 bool
	// EnableHTTP2 enables HTTP/2 fallback
	EnableHTTP2 bool
	// EnableBrotli enables Brotli compression
	EnableBrotli bool
	// Timeout is the total request timeout
	Timeout time.Duration
	// DialTimeout is the connection establishment timeout
	DialTimeout time.Duration
	// IdleConnTimeout is the idle connection timeout
	IdleConnTimeout time.Duration
	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int
	// MaxRetries is the maximum number of retries for failed requests
	MaxRetries int
	// RetryDelay is the initial delay between retries
	RetryDelay time.Duration
	// MaxRetryDelay is the maximum delay between retries
	MaxRetryDelay time.Duration
	// RetryMultiplier is the exponential backoff multiplier
	RetryMultiplier float64
	// TLSConfig for custom TLS settings
	TLSConfig *tls.Config
}

// DefaultHTTP3ClientConfig returns default configuration for HTTP/3 client
func DefaultHTTP3ClientConfig() *HTTP3ClientConfig {
	return &HTTP3ClientConfig{
		EnableHTTP3:     true,
		EnableHTTP2:     true,
		EnableBrotli:    true,
		Timeout:         120 * time.Second,
		DialTimeout:     30 * time.Second,
		IdleConnTimeout: 90 * time.Second,
		MaxIdleConns:    100,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
		MaxRetryDelay:   30 * time.Second,
		RetryMultiplier: 2.0,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h3", "h2", "http/1.1"},
		},
	}
}

// HTTP3Client is an HTTP client with HTTP/3 support and fallback to HTTP/2/HTTP/1.1
// It embeds *http.Client for full compatibility with standard HTTP clients
type HTTP3Client struct {
	*http.Client
	config         *HTTP3ClientConfig
	http3Transport *http3.Transport
	httpTransport  *http.Transport
	mu             sync.RWMutex
}

// NewHTTP3Client creates a new HTTP/3 client with fallback support
func NewHTTP3Client(config *HTTP3ClientConfig) *HTTP3Client {
	if config == nil {
		config = DefaultHTTP3ClientConfig()
	}

	client := &HTTP3Client{
		config: config,
	}

	// Configure TLS
	tlsConfig := config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h3", "h2", "http/1.1"},
		}
	}

	// Create HTTP/3 transport if enabled
	if config.EnableHTTP3 {
		client.http3Transport = &http3.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	// Create standard HTTP transport with HTTP/2 support
	client.httpTransport = &http.Transport{
		TLSClientConfig:       tlsConfig,
		IdleConnTimeout:       config.IdleConnTimeout,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConns,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Create a round tripper that tries HTTP/3 first, then falls back
	roundTripper := &http3RoundTripper{
		http3Transport: client.http3Transport,
		httpTransport:  client.httpTransport,
		enableHTTP3:    config.EnableHTTP3,
		enableBrotli:   config.EnableBrotli,
	}

	client.Client = &http.Client{
		Transport: roundTripper,
		Timeout:   config.Timeout,
	}

	return client
}

// HTTPClient returns the underlying *http.Client for use with existing code
// This allows the HTTP3Client to be used anywhere an *http.Client is expected
func (c *HTTP3Client) HTTPClient() *http.Client {
	return c.Client
}

// Do performs an HTTP request with HTTP/3 support and fallback
func (c *HTTP3Client) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithRetry(req, c.config.MaxRetries)
}

// DoWithRetry performs an HTTP request with retry logic
func (c *HTTP3Client) DoWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	delay := c.config.RetryDelay

	// Add Brotli compression header if enabled
	if c.config.EnableBrotli && req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "br, gzip")
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-req.Context().Done():
			return nil, fmt.Errorf("request context cancelled: %w", req.Context().Err())
		default:
		}

		resp, err := c.doAttempt(req)
		if err == nil {
			// Check for HTTP error status codes that should trigger retry
			if c.isRetryableStatus(resp.StatusCode) && attempt < maxRetries {
				resp.Body.Close()
				lastErr = fmt.Errorf("HTTP %d: retryable status", resp.StatusCode)
				c.waitWithJitter(req.Context(), delay)
				delay = c.nextDelay(delay)
				continue
			}
			return resp, nil
		}

		lastErr = err

		// Don't retry on context cancellation
		if req.Context().Err() != nil {
			return nil, fmt.Errorf("request context cancelled: %w", req.Context().Err())
		}

		// Check if error is retryable
		if !c.isRetryableError(err) || attempt >= maxRetries {
			return nil, err
		}

		// Wait before retry with jitter
		c.waitWithJitter(req.Context(), delay)
		delay = c.nextDelay(delay)
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}

// doAttempt performs a single HTTP request attempt
func (c *HTTP3Client) doAttempt(req *http.Request) (*http.Response, error) {
	// Clone request to allow retries
	body, err := c.cloneBody(req)
	if err != nil {
		return nil, err
	}
	if body != nil {
		defer body.Close()
	}

	// Create new request with cloned body
	newReq := req.Clone(req.Context())
	if body != nil {
		newReq.Body = io.NopCloser(body)
	}

	// Use the embedded client's transport
	resp, err := c.Client.Transport.RoundTrip(newReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Get performs an HTTP GET request
func (c *HTTP3Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	return c.Do(req)
}

// Post performs an HTTP POST request
func (c *HTTP3Client) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	return c.Do(req)
}

// PostJSON performs an HTTP POST request with JSON content
func (c *HTTP3Client) PostJSON(ctx context.Context, url string, body []byte) (*http.Response, error) {
	return c.Post(ctx, url, "application/json", bytes.NewReader(body))
}

// SetTimeout sets the client timeout
func (c *HTTP3Client) SetTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.Timeout = timeout
	c.Client.Timeout = timeout
}

// GetTimeout returns the current timeout
func (c *HTTP3Client) GetTimeout() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.Timeout
}

// Close closes all idle connections
func (c *HTTP3Client) Close() error {
	c.httpTransport.CloseIdleConnections()
	if c.http3Transport != nil {
		return c.http3Transport.Close()
	}
	return nil
}

// cloneBody clones the request body for retries
func (c *HTTP3Client) cloneBody(req *http.Request) (io.ReadCloser, error) {
	if req.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))

	return io.NopCloser(bytes.NewReader(body)), nil
}

// isRetryableError checks if an error should trigger a retry
func (c *HTTP3Client) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"no such host",
		"timeout",
		"temporary",
		"broken pipe",
		"protocol error",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	return false
}

// isRetryableStatus checks if an HTTP status code should trigger a retry
func (c *HTTP3Client) isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// waitWithJitter waits for the specified duration plus random jitter
func (c *HTTP3Client) waitWithJitter(ctx context.Context, delay time.Duration) {
	jitter := time.Duration(rand.Float64() * 0.1 * float64(delay)) //nolint:G404 // jitter doesn't require crypto randomness // #nosec G404 -- non-security random (jitter/test data/load spread); math/rand is the correct choice
	select {
	case <-ctx.Done():
	case <-time.After(delay + jitter):
	}
}

// nextDelay calculates the next delay using exponential backoff
func (c *HTTP3Client) nextDelay(currentDelay time.Duration) time.Duration {
	nextDelay := time.Duration(float64(currentDelay) * c.config.RetryMultiplier)
	if nextDelay > c.config.MaxRetryDelay {
		nextDelay = c.config.MaxRetryDelay
	}
	return nextDelay
}

// GetConfig returns a copy of the current configuration
func (c *HTTP3Client) GetConfig() HTTP3ClientConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.config
}

// http3RoundTripper implements http.RoundTripper with HTTP/3 support
type http3RoundTripper struct {
	http3Transport *http3.Transport
	httpTransport  *http.Transport
	enableHTTP3    bool
	enableBrotli   bool
}

// RoundTrip implements http.RoundTripper
func (rt *http3RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add Brotli compression header if enabled
	if rt.enableBrotli && req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "br, gzip")
	}

	// Try HTTP/3 first if enabled
	if rt.enableHTTP3 && rt.http3Transport != nil {
		resp, err := rt.http3Transport.RoundTrip(req)
		if err == nil {
			return rt.decompressResponse(resp), nil
		}
	}

	// Fallback to HTTP/2 or HTTP/1.1
	resp, err := rt.httpTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	return rt.decompressResponse(resp), nil
}

// decompressResponse decompresses the response body if needed
func (rt *http3RoundTripper) decompressResponse(resp *http.Response) *http.Response {
	if resp == nil || resp.Body == nil {
		return resp
	}

	encoding := resp.Header.Get("Content-Encoding")
	if encoding == "" {
		return resp
	}

	switch encoding {
	case "br":
		if rt.enableBrotli {
			resp.Body = &brotliReadCloser{
				reader: brotli.NewReader(resp.Body),
				closer: resp.Body,
			}
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
		}
	}

	return resp
}

// brotliReadCloser wraps a brotli reader with Close support
type brotliReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (r *brotliReadCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *brotliReadCloser) Close() error {
	return r.closer.Close()
}

// HTTP3RoundTripper implements http.RoundTripper with HTTP/3 support (public version)
type HTTP3RoundTripper struct {
	http3Transport *http3.Transport
	httpTransport  *http.Transport
	enableHTTP3    bool
}

// NewHTTP3RoundTripper creates a new HTTP/3 round tripper with fallback
func NewHTTP3RoundTripper(enableHTTP3 bool) *HTTP3RoundTripper {
	rt := &HTTP3RoundTripper{
		enableHTTP3: enableHTTP3,
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h3", "h2", "http/1.1"},
	}

	if enableHTTP3 {
		rt.http3Transport = &http3.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	rt.httpTransport = &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2", "http/1.1"},
		},
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return rt
}

// RoundTrip implements http.RoundTripper
func (rt *HTTP3RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Try HTTP/3 first if enabled
	if rt.enableHTTP3 && rt.http3Transport != nil {
		resp, err := rt.http3Transport.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
	}

	// Fallback to HTTP/2 or HTTP/1.1
	return rt.httpTransport.RoundTrip(req)
}

// Close closes all idle connections
func (rt *HTTP3RoundTripper) Close() error {
	rt.httpTransport.CloseIdleConnections()
	if rt.http3Transport != nil {
		return rt.http3Transport.Close()
	}
	return nil
}

// GlobalHTTP3Client is a global HTTP/3 client instance
var (
	globalHTTP3Client     *HTTP3Client
	globalHTTP3ClientOnce sync.Once
)

// GetGlobalHTTP3Client returns the global HTTP/3 client instance
func GetGlobalHTTP3Client() *HTTP3Client {
	globalHTTP3ClientOnce.Do(func() {
		globalHTTP3Client = NewHTTP3Client(nil)
	})
	return globalHTTP3Client
}

// SetGlobalHTTP3Client sets the global HTTP/3 client instance
func SetGlobalHTTP3Client(client *HTTP3Client) {
	globalHTTP3Client = client
}

// ResetGlobalHTTP3Client resets the global HTTP/3 client to default
func ResetGlobalHTTP3Client() {
	globalHTTP3Client = NewHTTP3Client(nil)
}
