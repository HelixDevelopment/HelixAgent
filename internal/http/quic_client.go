package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// QUICConfig holds configuration for QUIC/HTTP3 client
type QUICConfig struct {
	// TLS configuration (required for QUIC)
	TLSConfig *tls.Config
	// Maximum idle timeout for connections
	MaxIdleTimeout time.Duration
	// Initial connection window size
	InitialConnectionWindowSize uint64
	// Initial stream window size
	InitialStreamWindowSize uint64
	// Maximum connections per host
	MaxConnsPerHost int
	// Enable datagram support
	EnableDatagrams bool
	// Keep alive ping interval (0 to disable)
	KeepAlivePingInterval time.Duration
	// Request timeout
	RequestTimeout time.Duration
	// Enable HTTP/2 fallback
	EnableH2Fallback bool
}

// DefaultQUICConfig returns default QUIC configuration
func DefaultQUICConfig() *QUICConfig {
	return &QUICConfig{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS13,
		},
		MaxIdleTimeout:              30 * time.Second,
		InitialConnectionWindowSize: 10 * 1024 * 1024, // 10 MB
		InitialStreamWindowSize:     6 * 1024 * 1024,  // 6 MB
		MaxConnsPerHost:             10,
		EnableDatagrams:             false,
		KeepAlivePingInterval:       15 * time.Second,
		RequestTimeout:              30 * time.Second,
		EnableH2Fallback:            true,
	}
}

// QUICMetrics tracks QUIC client statistics
type QUICMetrics struct {
	TotalRequests     int64
	SuccessRequests   int64
	FailedRequests    int64
	H3Requests        int64
	FallbackRequests  int64
	TotalLatencyUs    int64
	RequestCount      int64
	ActiveConnections int64
}

// AverageLatency returns the average request latency
func (m *QUICMetrics) AverageLatency() time.Duration {
	count := atomic.LoadInt64(&m.RequestCount)
	if count == 0 {
		return 0
	}
	totalUs := atomic.LoadInt64(&m.TotalLatencyUs)
	return time.Duration(totalUs/count) * time.Microsecond
}

// QUICClient provides HTTP/3 transport for internal API calls
type QUICClient struct {
	h3RoundTripper *http3.Transport
	h2Client       *http.Client
	config         *QUICConfig
	metrics        *QUICMetrics
	mu             sync.RWMutex
	closed         bool
}

// NewQUICClient creates a new QUIC/HTTP3 client
func NewQUICClient(config *QUICConfig) (*QUICClient, error) {
	if config == nil {
		config = DefaultQUICConfig()
	}

	// Create QUIC transport
	quicConfig := &quic.Config{
		MaxIdleTimeout:                 config.MaxIdleTimeout,
		InitialConnectionReceiveWindow: config.InitialConnectionWindowSize,
		InitialStreamReceiveWindow:     config.InitialStreamWindowSize,
		EnableDatagrams:                config.EnableDatagrams,
		KeepAlivePeriod:                config.KeepAlivePingInterval,
	}

	h3Transport := &http3.Transport{
		TLSClientConfig: config.TLSConfig,
		QUICConfig:      quicConfig,
	}

	// Create HTTP/2 fallback client
	var h2Client *http.Client
	if config.EnableH2Fallback {
		h2Transport := &http.Transport{
			TLSClientConfig:   config.TLSConfig,
			ForceAttemptHTTP2: true,
		}
		h2Client = &http.Client{
			Transport: h2Transport,
			Timeout:   config.RequestTimeout,
		}
	}

	return &QUICClient{
		h3RoundTripper: h3Transport,
		h2Client:       h2Client,
		config:         config,
		metrics:        &QUICMetrics{},
	}, nil
}

// Do performs an HTTP request using HTTP/3 with fallback to HTTP/2
func (c *QUICClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client is closed")
	}
	c.mu.RUnlock()

	atomic.AddInt64(&c.metrics.TotalRequests, 1)
	startTime := time.Now()

	// Try HTTP/3 first
	resp, err := c.doHTTP3(req)
	if err == nil {
		atomic.AddInt64(&c.metrics.H3Requests, 1)
		atomic.AddInt64(&c.metrics.SuccessRequests, 1)
		c.recordLatency(startTime)
		return resp, nil
	}

	// Fallback to HTTP/2 if enabled
	if c.config.EnableH2Fallback && c.h2Client != nil {
		//nolint:gosec // G704: HTTP/2 fallback for outbound provider call — URL is caller-supplied via the same gated adapter layer as the primary QUIC path
		resp, err = c.h2Client.Do(req)
		if err == nil {
			atomic.AddInt64(&c.metrics.FallbackRequests, 1)
			atomic.AddInt64(&c.metrics.SuccessRequests, 1)
			c.recordLatency(startTime)
			return resp, nil
		}
	}

	atomic.AddInt64(&c.metrics.FailedRequests, 1)
	c.recordLatency(startTime)
	return nil, err
}

// doHTTP3 performs the request using HTTP/3
func (c *QUICClient) doHTTP3(req *http.Request) (*http.Response, error) {
	// Create context with timeout
	ctx := req.Context()
	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	req = req.WithContext(ctx)

	// Use HTTP/3 transport
	return c.h3RoundTripper.RoundTrip(req)
}

// recordLatency records request latency
func (c *QUICClient) recordLatency(startTime time.Time) {
	duration := time.Since(startTime)
	atomic.AddInt64(&c.metrics.TotalLatencyUs, duration.Microseconds())
	atomic.AddInt64(&c.metrics.RequestCount, 1)
}

// Get performs an HTTP GET request
func (c *QUICClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post performs an HTTP POST request
func (c *QUICClient) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// PostJSON performs a POST request with JSON content type
func (c *QUICClient) PostJSON(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	return c.Post(ctx, url, "application/json", body)
}

// Metrics returns current client metrics
func (c *QUICClient) Metrics() *QUICMetrics {
	return &QUICMetrics{
		TotalRequests:     atomic.LoadInt64(&c.metrics.TotalRequests),
		SuccessRequests:   atomic.LoadInt64(&c.metrics.SuccessRequests),
		FailedRequests:    atomic.LoadInt64(&c.metrics.FailedRequests),
		H3Requests:        atomic.LoadInt64(&c.metrics.H3Requests),
		FallbackRequests:  atomic.LoadInt64(&c.metrics.FallbackRequests),
		TotalLatencyUs:    atomic.LoadInt64(&c.metrics.TotalLatencyUs),
		RequestCount:      atomic.LoadInt64(&c.metrics.RequestCount),
		ActiveConnections: atomic.LoadInt64(&c.metrics.ActiveConnections),
	}
}

// Close closes the QUIC client and releases resources
func (c *QUICClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// Close HTTP/3 transport
	if c.h3RoundTripper != nil {
		_ = c.h3RoundTripper.Close()
	}

	// Close HTTP/2 client transport
	if c.h2Client != nil {
		if transport, ok := c.h2Client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	return nil
}

// HTTP3ProviderTransport wraps provider HTTP calls with QUIC support
type HTTP3ProviderTransport struct {
	quicClient *QUICClient
	fallback   *http.Client
}

// NewHTTP3ProviderTransport creates a new HTTP/3 provider transport
func NewHTTP3ProviderTransport(config *QUICConfig) (*HTTP3ProviderTransport, error) {
	quicClient, err := NewQUICClient(config)
	if err != nil {
		return nil, err
	}

	// Create fallback HTTP/1.1/2 client
	fallback := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: config.TLSConfig,
		},
		Timeout: config.RequestTimeout,
	}

	return &HTTP3ProviderTransport{
		quicClient: quicClient,
		fallback:   fallback,
	}, nil
}

// RoundTrip implements http.RoundTripper for provider calls
func (t *HTTP3ProviderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Try QUIC first
	//nolint:gosec // G704: transport-level forward of an already-validated outbound request; SSRF defence applies at the adapter construction layer
	resp, err := t.quicClient.Do(req)
	if err != nil && isQUICError(err) {
		// Fallback to HTTP/2 or HTTP/1.1
		return t.fallback.Do(req)
	}
	return resp, err
}

// Close closes the transport
func (t *HTTP3ProviderTransport) Close() error {
	return t.quicClient.Close()
}

// isQUICError checks if an error is QUIC-specific
func isQUICError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common QUIC errors
	errStr := err.Error()
	quicErrors := []string{
		"quic",
		"QUIC",
		"connection refused",
		"no recent network activity",
		"handshake",
		"timeout",
	}
	for _, qErr := range quicErrors {
		if contains(errStr, qErr) {
			return true
		}
	}
	return false
}

// contains checks if s contains substr (simple implementation)
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
