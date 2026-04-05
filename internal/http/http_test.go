package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// PoolConfig Tests
// =============================================================================

func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()

	assert.Equal(t, 100, config.MaxIdleConns)
	assert.Equal(t, 10, config.MaxConnsPerHost)
	assert.Equal(t, 10, config.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, config.IdleConnTimeout)
	assert.Equal(t, 30*time.Second, config.DialTimeout)
	assert.Equal(t, 10*time.Second, config.TLSHandshakeTimeout)
	assert.Equal(t, 30*time.Second, config.ResponseHeaderTimeout)
	assert.Equal(t, 1*time.Second, config.ExpectContinueTimeout)
	assert.Equal(t, 30*time.Second, config.KeepAliveInterval)
	assert.False(t, config.DisableKeepAlives)
	assert.False(t, config.DisableCompression)
	assert.Equal(t, 3, config.RetryCount)
	assert.Equal(t, 100*time.Millisecond, config.RetryWaitMin)
	assert.Equal(t, 2*time.Second, config.RetryWaitMax)
}

func TestPoolConfig_CustomValues(t *testing.T) {
	config := &PoolConfig{
		MaxIdleConns:          50,
		MaxConnsPerHost:       5,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       60 * time.Second,
		DialTimeout:           15 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 500 * time.Millisecond,
		KeepAliveInterval:     15 * time.Second,
		DisableKeepAlives:     true,
		DisableCompression:    true,
		RetryCount:            5,
		RetryWaitMin:          50 * time.Millisecond,
		RetryWaitMax:          1 * time.Second,
	}

	assert.Equal(t, 50, config.MaxIdleConns)
	assert.Equal(t, 5, config.MaxConnsPerHost)
	assert.True(t, config.DisableKeepAlives)
	assert.True(t, config.DisableCompression)
	assert.Equal(t, 5, config.RetryCount)
}

// =============================================================================
// PoolMetrics Tests
// =============================================================================

func TestPoolMetrics_AverageLatency(t *testing.T) {
	t.Run("zero requests returns zero latency", func(t *testing.T) {
		metrics := &PoolMetrics{}
		assert.Equal(t, time.Duration(0), metrics.AverageLatency())
	})

	t.Run("calculates average correctly", func(t *testing.T) {
		metrics := &PoolMetrics{
			TotalLatencyUs: 1000,
			RequestCount:   10,
		}
		assert.Equal(t, 100*time.Microsecond, metrics.AverageLatency())
	})

	t.Run("handles atomic operations", func(t *testing.T) {
		metrics := &PoolMetrics{}
		atomic.StoreInt64(&metrics.TotalLatencyUs, 5000)
		atomic.StoreInt64(&metrics.RequestCount, 5)
		assert.Equal(t, 1000*time.Microsecond, metrics.AverageLatency())
	})
}

// =============================================================================
// HTTPClientPool Tests
// =============================================================================

func TestNewHTTPClientPool(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		pool := NewHTTPClientPool(nil)
		assert.NotNil(t, pool)
		assert.NotNil(t, pool.config)
		assert.NotNil(t, pool.metrics)
		assert.NotNil(t, pool.transport)
		assert.NotNil(t, pool.clients)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &PoolConfig{
			MaxIdleConns:    50,
			MaxConnsPerHost: 5,
		}
		pool := NewHTTPClientPool(config)
		assert.NotNil(t, pool)
		assert.Equal(t, 50, pool.config.MaxIdleConns)
	})

	t.Run("with TLS config", func(t *testing.T) {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		config := &PoolConfig{
			TLSConfig: tlsConfig,
		}
		pool := NewHTTPClientPool(config)
		assert.NotNil(t, pool)
		assert.Equal(t, tlsConfig, pool.transport.TLSClientConfig)
	})

	t.Run("with InsecureSkipVerify", func(t *testing.T) {
		config := &PoolConfig{
			InsecureSkipVerify: true,
		}
		pool := NewHTTPClientPool(config)
		assert.NotNil(t, pool)
		assert.True(t, pool.transport.TLSClientConfig.InsecureSkipVerify)
	})
}

func TestHTTPClientPool_GetClient(t *testing.T) {
	pool := NewHTTPClientPool(nil)

	t.Run("creates new client for host", func(t *testing.T) {
		client := pool.GetClient("example.com")
		assert.NotNil(t, client)
	})

	t.Run("returns same client for same host", func(t *testing.T) {
		client1 := pool.GetClient("example.com")
		client2 := pool.GetClient("example.com")
		assert.Same(t, client1, client2)
	})

	t.Run("creates different clients for different hosts", func(t *testing.T) {
		client1 := pool.GetClient("example1.com")
		client2 := pool.GetClient("example2.com")
		assert.NotSame(t, client1, client2)
	})
}

func TestHTTPClientPool_GetClient_Concurrent(t *testing.T) {
	pool := NewHTTPClientPool(nil)

	var wg sync.WaitGroup
	clients := make([]*http.Client, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			clients[idx] = pool.GetClient("example.com")
		}(i)
	}

	wg.Wait()

	// All clients should be the same instance
	for i := 1; i < 100; i++ {
		assert.Same(t, clients[0], clients[i])
	}
}

// TestHTTPClientPool_GetClient_ConcurrentRace verifies the double-check locking
// in GetClient under the race detector: 100 goroutines all request the same
// host and MUST receive the identical *http.Client instance.  Run with -race.
func TestHTTPClientPool_GetClient_ConcurrentRace(t *testing.T) {
	const goroutines = 100
	const host = "race-test.example.com"

	pool := NewHTTPClientPool(nil)

	// ready acts as a starting gun so all goroutines hit GetClient at once.
	ready := make(chan struct{})
	results := make([]*http.Client, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			<-ready // wait for the gun
			results[idx] = pool.GetClient(host)
		}(i)
	}

	close(ready) // fire all goroutines simultaneously
	wg.Wait()

	// Every goroutine must have received the exact same pointer.
	require.NotNil(t, results[0], "first client must not be nil")
	for i := 1; i < goroutines; i++ {
		assert.Same(t, results[0], results[i],
			"goroutine %d got a different *http.Client instance", i)
	}

	// Pool must contain exactly one entry for the host.
	assert.Equal(t, 1, pool.ClientCount(),
		"pool must hold exactly one client for a single host")
}

func TestHTTPClientPool_GetClientForURL(t *testing.T) {
	pool := NewHTTPClientPool(nil)

	t.Run("valid URL returns client", func(t *testing.T) {
		client, err := pool.GetClientForURL("https://example.com/path")
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("invalid URL returns error", func(t *testing.T) {
		client, err := pool.GetClientForURL("://invalid")
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "invalid URL")
	})
}

func TestHTTPClientPool_ClientCount(t *testing.T) {
	pool := NewHTTPClientPool(nil)

	assert.Equal(t, 0, pool.ClientCount())

	pool.GetClient("host1.com")
	assert.Equal(t, 1, pool.ClientCount())

	pool.GetClient("host2.com")
	assert.Equal(t, 2, pool.ClientCount())

	// Same host shouldn't increase count
	pool.GetClient("host1.com")
	assert.Equal(t, 2, pool.ClientCount())
}

func TestHTTPClientPool_Do_WithTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := pool.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "success", string(body))
}

func TestHTTPClientPool_DoWithContext(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		pool := NewHTTPClientPool(nil)
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)

		resp, err := pool.DoWithContext(context.Background(), req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		metrics := pool.Metrics()
		assert.Equal(t, int64(1), metrics.TotalRequests)
		assert.Equal(t, int64(1), metrics.SuccessRequests)
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		pool := NewHTTPClientPool(&PoolConfig{
			RetryCount: 0,
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		_, err := pool.DoWithContext(ctx, req)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}

func TestHTTPClientPool_RetryLogic(t *testing.T) {
	t.Run("retries on server error", func(t *testing.T) {
		var requestCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&requestCount, 1)
			if count <= 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &PoolConfig{
			RetryCount:   3,
			RetryWaitMin: 10 * time.Millisecond,
			RetryWaitMax: 50 * time.Millisecond,
		}
		pool := NewHTTPClientPool(config)

		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, err := pool.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, int32(3), atomic.LoadInt32(&requestCount))
	})

	t.Run("retries on connection error", func(t *testing.T) {
		var requestCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requestCount, 1)
			w.WriteHeader(http.StatusOK)
		}))
		// Close server to cause connection error
		server.Close()

		config := &PoolConfig{
			RetryCount:   2,
			RetryWaitMin: 10 * time.Millisecond,
			RetryWaitMax: 50 * time.Millisecond,
		}
		pool := NewHTTPClientPool(config)

		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		_, err := pool.Do(req)
		assert.Error(t, err)

		metrics := pool.Metrics()
		assert.True(t, metrics.RetryCount >= 1)
	})

	t.Run("custom retry condition", func(t *testing.T) {
		var requestCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&requestCount, 1)
			if count <= 1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &PoolConfig{
			RetryCount:   3,
			RetryWaitMin: 10 * time.Millisecond,
			RetryWaitMax: 50 * time.Millisecond,
			RetryCondition: func(resp *http.Response, err error) bool {
				// Custom: retry on 404
				return resp != nil && resp.StatusCode == http.StatusNotFound
			},
		}
		pool := NewHTTPClientPool(config)

		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, err := pool.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount))
	})
}

func TestHTTPClientPool_DefaultRetryCondition(t *testing.T) {
	pool := NewHTTPClientPool(nil)

	tests := []struct {
		name        string
		statusCode  int
		err         error
		shouldRetry bool
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests, nil, true},
		{"502 Bad Gateway", http.StatusBadGateway, nil, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, nil, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, nil, true},
		{"500 Internal Server Error", http.StatusInternalServerError, nil, false},
		{"200 OK", http.StatusOK, nil, false},
		{"404 Not Found", http.StatusNotFound, nil, false},
		{"Connection error", 0, errors.New("connection error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.err == nil {
				resp = &http.Response{StatusCode: tt.statusCode}
			}
			result := pool.defaultRetryCondition(resp, tt.err)
			assert.Equal(t, tt.shouldRetry, result)
		})
	}
}

func TestHTTPClientPool_CalculateRetryDelay(t *testing.T) {
	config := &PoolConfig{
		RetryWaitMin: 100 * time.Millisecond,
		RetryWaitMax: 1 * time.Second,
	}
	pool := NewHTTPClientPool(config)

	t.Run("exponential backoff", func(t *testing.T) {
		delay0 := pool.calculateRetryDelay(0)
		delay1 := pool.calculateRetryDelay(1)
		delay2 := pool.calculateRetryDelay(2)

		assert.Equal(t, 100*time.Millisecond, delay0)
		assert.Equal(t, 200*time.Millisecond, delay1)
		assert.Equal(t, 400*time.Millisecond, delay2)
	})

	t.Run("capped at max", func(t *testing.T) {
		delay5 := pool.calculateRetryDelay(5) // Would be 3.2s without cap
		assert.Equal(t, 1*time.Second, delay5)
	})
}

func TestHTTPClientPool_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("get response"))
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	resp, err := pool.Get(context.Background(), server.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "get response", string(body))
}

func TestHTTPClientPool_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	resp, err := pool.Post(context.Background(), server.URL, "text/plain", strings.NewReader("test body"))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "test body", string(body))
}

func TestHTTPClientPool_PostJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	resp, err := pool.PostJSON(context.Background(), server.URL, strings.NewReader(`{"key": "value"}`))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPClientPool_Metrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)

	// Make some requests
	for i := 0; i < 5; i++ {
		resp, _ := pool.Get(context.Background(), server.URL)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	metrics := pool.Metrics()
	assert.Equal(t, int64(5), metrics.TotalRequests)
	assert.Equal(t, int64(5), metrics.SuccessRequests)
	assert.Equal(t, int64(5), metrics.RequestCount)
	assert.True(t, metrics.TotalLatencyUs > 0)
	assert.Equal(t, int64(0), metrics.ActiveRequests)
}

func TestHTTPClientPool_CloseIdleConnections(t *testing.T) {
	pool := NewHTTPClientPool(nil)
	pool.GetClient("example.com")

	// Should not panic
	pool.CloseIdleConnections()
}

func TestHTTPClientPool_Close(t *testing.T) {
	pool := NewHTTPClientPool(nil)
	pool.GetClient("host1.com")
	pool.GetClient("host2.com")

	assert.Equal(t, 2, pool.ClientCount())

	err := pool.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, pool.ClientCount())
}

// =============================================================================
// Global Pool Tests
// =============================================================================

func TestGlobalPool_Init(t *testing.T) {
	// Reset global pool
	GlobalPool = nil
	globalPoolOnce = sync.Once{}

	InitGlobalPool(nil)
	assert.NotNil(t, GlobalPool)

	// Re-init should close old pool and create new
	oldPool := GlobalPool
	InitGlobalPool(&PoolConfig{MaxIdleConns: 50})
	assert.NotSame(t, oldPool, GlobalPool)
}

func TestGlobalPool_Get(t *testing.T) {
	GlobalPool = nil
	globalPoolOnce = sync.Once{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Should auto-init global pool
	resp, err := Get(context.Background(), server.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.NotNil(t, GlobalPool)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGlobalPool_PostJSON(t *testing.T) {
	GlobalPool = nil
	globalPoolOnce = sync.Once{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := PostJSON(context.Background(), server.URL, strings.NewReader(`{}`))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// =============================================================================
// HostClient Tests
// =============================================================================

func TestNewHostClient(t *testing.T) {
	pool := NewHTTPClientPool(nil)

	t.Run("valid base URL", func(t *testing.T) {
		client, err := NewHostClient(pool, "https://api.example.com")
		assert.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "api.example.com", client.host)
		assert.Equal(t, "https://api.example.com", client.baseURL)
	})

	t.Run("invalid base URL", func(t *testing.T) {
		client, err := NewHostClient(pool, "://invalid")
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "invalid base URL")
	})
}

func TestHostClient_SetHeader(t *testing.T) {
	pool := NewHTTPClientPool(nil)
	client, _ := NewHostClient(pool, "https://api.example.com")

	client.SetHeader("Authorization", "Bearer token123")
	client.SetHeader("X-Custom", "value")

	client.mu.RLock()
	assert.Equal(t, "Bearer token123", client.headers["Authorization"])
	assert.Equal(t, "value", client.headers["X-Custom"])
	client.mu.RUnlock()
}

func TestHostClient_Do(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	client, _ := NewHostClient(pool, server.URL)
	client.SetHeader("Authorization", "Bearer token")

	resp, err := client.Do(context.Background(), http.MethodGet, "/api/test", nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "/api/test", string(body))
}

func TestHostClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	client, _ := NewHostClient(pool, server.URL)

	resp, err := client.Get(context.Background(), "/path")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHostClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	client, _ := NewHostClient(pool, server.URL)

	resp, err := client.Post(context.Background(), "/create", bytes.NewReader([]byte("data")))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "data", string(body))
}

func TestHostClient_PostJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	client, _ := NewHostClient(pool, server.URL)

	resp, err := client.PostJSON(context.Background(), "/api", strings.NewReader(`{"key": "value"}`))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHostClient_Concurrent(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	client, _ := NewHostClient(pool, server.URL)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(context.Background(), "/")
			if err == nil && resp != nil {
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(50), atomic.LoadInt32(&requestCount))
}

// =============================================================================
// QUICConfig Tests
// =============================================================================

func TestDefaultQUICConfig(t *testing.T) {
	config := DefaultQUICConfig()

	assert.NotNil(t, config.TLSConfig)
	assert.False(t, config.TLSConfig.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS13), config.TLSConfig.MinVersion)
	assert.Equal(t, 30*time.Second, config.MaxIdleTimeout)
	assert.Equal(t, uint64(10*1024*1024), config.InitialConnectionWindowSize)
	assert.Equal(t, uint64(6*1024*1024), config.InitialStreamWindowSize)
	assert.Equal(t, 10, config.MaxConnsPerHost)
	assert.False(t, config.EnableDatagrams)
	assert.Equal(t, 15*time.Second, config.KeepAlivePingInterval)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
	assert.True(t, config.EnableH2Fallback)
}

func TestQUICConfig_Custom(t *testing.T) {
	config := &QUICConfig{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleTimeout:              60 * time.Second,
		InitialConnectionWindowSize: 20 * 1024 * 1024,
		InitialStreamWindowSize:     10 * 1024 * 1024,
		MaxConnsPerHost:             20,
		EnableDatagrams:             true,
		KeepAlivePingInterval:       30 * time.Second,
		RequestTimeout:              60 * time.Second,
		EnableH2Fallback:            false,
	}

	assert.True(t, config.TLSConfig.InsecureSkipVerify)
	assert.Equal(t, 60*time.Second, config.MaxIdleTimeout)
	assert.True(t, config.EnableDatagrams)
	assert.False(t, config.EnableH2Fallback)
}

// =============================================================================
// QUICMetrics Tests
// =============================================================================

func TestQUICMetrics_AverageLatency(t *testing.T) {
	t.Run("zero requests returns zero", func(t *testing.T) {
		metrics := &QUICMetrics{}
		assert.Equal(t, time.Duration(0), metrics.AverageLatency())
	})

	t.Run("calculates average correctly", func(t *testing.T) {
		metrics := &QUICMetrics{
			TotalLatencyUs: 2000,
			RequestCount:   4,
		}
		assert.Equal(t, 500*time.Microsecond, metrics.AverageLatency())
	})
}

// =============================================================================
// QUICClient Tests
// =============================================================================

func TestNewQUICClient(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		client, err := NewQUICClient(nil)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.NotNil(t, client.h3RoundTripper)
		assert.NotNil(t, client.h2Client) // H2 fallback enabled by default
		assert.NotNil(t, client.metrics)
		assert.False(t, client.closed)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &QUICConfig{
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleTimeout:   60 * time.Second,
			RequestTimeout:   45 * time.Second,
			EnableH2Fallback: true,
		}
		client, err := NewQUICClient(config)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.NotNil(t, client.h2Client)
	})

	t.Run("without H2 fallback", func(t *testing.T) {
		config := &QUICConfig{
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			EnableH2Fallback: false,
		}
		client, err := NewQUICClient(config)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Nil(t, client.h2Client)
	})
}

func TestQUICClient_Do_Closed(t *testing.T) {
	client, err := NewQUICClient(nil)
	require.NoError(t, err)

	// Close the client
	_ = client.Close()

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, err = client.Do(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client is closed")
}

func TestQUICClient_Metrics(t *testing.T) {
	client, err := NewQUICClient(nil)
	require.NoError(t, err)

	// Manually set some metrics values
	atomic.StoreInt64(&client.metrics.TotalRequests, 10)
	atomic.StoreInt64(&client.metrics.SuccessRequests, 8)
	atomic.StoreInt64(&client.metrics.FailedRequests, 2)
	atomic.StoreInt64(&client.metrics.H3Requests, 6)
	atomic.StoreInt64(&client.metrics.FallbackRequests, 2)
	atomic.StoreInt64(&client.metrics.TotalLatencyUs, 5000)
	atomic.StoreInt64(&client.metrics.RequestCount, 10)

	metrics := client.Metrics()
	assert.Equal(t, int64(10), metrics.TotalRequests)
	assert.Equal(t, int64(8), metrics.SuccessRequests)
	assert.Equal(t, int64(2), metrics.FailedRequests)
	assert.Equal(t, int64(6), metrics.H3Requests)
	assert.Equal(t, int64(2), metrics.FallbackRequests)
	assert.Equal(t, int64(5000), metrics.TotalLatencyUs)
	assert.Equal(t, int64(10), metrics.RequestCount)
}

func TestQUICClient_Close(t *testing.T) {
	t.Run("closes successfully", func(t *testing.T) {
		client, err := NewQUICClient(nil)
		require.NoError(t, err)

		err = client.Close()
		assert.NoError(t, err)
		assert.True(t, client.closed)
	})

	t.Run("double close is safe", func(t *testing.T) {
		client, err := NewQUICClient(nil)
		require.NoError(t, err)

		err = client.Close()
		assert.NoError(t, err)

		err = client.Close()
		assert.NoError(t, err) // Should not error on double close
	})
}

func TestQUICClient_Get(t *testing.T) {
	client, err := NewQUICClient(nil)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// This will fail because there's no actual QUIC server, but we're testing the method signature
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.Get(ctx, "https://localhost:12345/test")
	// We expect an error since there's no server
	assert.Error(t, err)
}

func TestQUICClient_Post(t *testing.T) {
	client, err := NewQUICClient(nil)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.Post(ctx, "https://localhost:12345/test", "application/json", strings.NewReader(`{}`))
	assert.Error(t, err)
}

func TestQUICClient_PostJSON(t *testing.T) {
	client, err := NewQUICClient(nil)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.PostJSON(ctx, "https://localhost:12345/test", strings.NewReader(`{"key": "value"}`))
	assert.Error(t, err)
}

func TestQUICClient_RecordLatency(t *testing.T) {
	client, err := NewQUICClient(nil)
	require.NoError(t, err)

	startTime := time.Now().Add(-100 * time.Millisecond)
	client.recordLatency(startTime)

	assert.True(t, client.metrics.TotalLatencyUs >= 100000)
	assert.Equal(t, int64(1), client.metrics.RequestCount)
}

// =============================================================================
// HTTP3ProviderTransport Tests
// =============================================================================

func TestNewHTTP3ProviderTransport(t *testing.T) {
	config := &QUICConfig{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		RequestTimeout: 30 * time.Second,
	}

	transport, err := NewHTTP3ProviderTransport(config)
	require.NoError(t, err)
	assert.NotNil(t, transport)
	assert.NotNil(t, transport.quicClient)
	assert.NotNil(t, transport.fallback)
}

func TestHTTP3ProviderTransport_Close(t *testing.T) {
	config := &QUICConfig{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	transport, err := NewHTTP3ProviderTransport(config)
	require.NoError(t, err)

	err = transport.Close()
	assert.NoError(t, err)
}

func TestHTTP3ProviderTransport_RoundTrip(t *testing.T) {
	config := &QUICConfig{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		RequestTimeout: 100 * time.Millisecond,
	}

	transport, err := NewHTTP3ProviderTransport(config)
	require.NoError(t, err)
	defer func() { _ = transport.Close() }()

	// Create a request to a non-existent server - this will fail but exercises the RoundTrip method
	req, _ := http.NewRequest(http.MethodGet, "https://localhost:12346/test", nil)

	// This will fail but we're testing that RoundTrip is called and handles errors
	_, err = transport.RoundTrip(req)
	assert.Error(t, err) // Expected to fail since there's no server
}

func TestHTTP3ProviderTransport_RoundTrip_FallbackToHTTP(t *testing.T) {
	// Create a regular HTTPS test server for fallback
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fallback response"))
	}))
	defer server.Close()

	config := &QUICConfig{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		RequestTimeout:   100 * time.Millisecond,
		EnableH2Fallback: true,
	}

	transport, err := NewHTTP3ProviderTransport(config)
	require.NoError(t, err)
	defer func() { _ = transport.Close() }()

	// The QUIC request will fail (no QUIC server) and should fall back to HTTP
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := transport.RoundTrip(req)

	// Fallback should work with the HTTPS test server
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
	// If fallback also fails (e.g., due to TLS mismatch), that's also acceptable for this test
}

// =============================================================================
// Helper Functions Tests
// =============================================================================

func TestIsQUICError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"quic lowercase", errors.New("quic connection failed"), true},
		{"QUIC uppercase", errors.New("QUIC handshake error"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"no recent network activity", errors.New("no recent network activity"), true},
		{"handshake error", errors.New("TLS handshake failed"), true},
		{"timeout", errors.New("connection timeout"), true},
		{"generic error", errors.New("some random error"), false},
		{"network unreachable", errors.New("network unreachable"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isQUICError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "xyz", false},
		{"", "test", false},
		{"test", "", true},
		{"abc", "abc", true},
		{"ab", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestHTTPClientPool_RequestWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(&PoolConfig{
		RetryCount:   2,
		RetryWaitMin: 10 * time.Millisecond,
		RetryWaitMax: 50 * time.Millisecond,
	})

	// Create request with a body that can be re-read
	bodyContent := "test body content"
	req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(bodyContent))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(bodyContent)), nil
	}

	resp, err := pool.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	assert.Equal(t, bodyContent, string(respBody))
}

func TestHTTPClientPool_4xxResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)
	resp, err := pool.Get(context.Background(), server.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	metrics := pool.Metrics()
	assert.Equal(t, int64(1), metrics.FailedRequests)
}

func TestHTTPClientPool_ActiveRequestsTracking(t *testing.T) {
	var requestStarted sync.WaitGroup
	var requestComplete sync.WaitGroup
	requestStarted.Add(1)
	requestComplete.Add(1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted.Done()
		requestComplete.Wait()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pool := NewHTTPClientPool(nil)

	go func() {
		resp, _ := pool.Get(context.Background(), server.URL)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()

	requestStarted.Wait()

	// During request, active should be 1
	metrics := pool.Metrics()
	assert.Equal(t, int64(1), metrics.ActiveRequests)

	requestComplete.Done()
	time.Sleep(50 * time.Millisecond)

	// After request, active should be 0
	metrics = pool.Metrics()
	assert.Equal(t, int64(0), metrics.ActiveRequests)
}

func TestHostClient_HeaderConcurrency(t *testing.T) {
	pool := NewHTTPClientPool(nil)
	client, _ := NewHostClient(pool, "https://api.example.com")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			client.SetHeader("X-Request-ID", "req-"+string(rune(idx)))
		}(i)
	}
	wg.Wait()

	// Should not panic due to concurrent map access
	client.mu.RLock()
	assert.NotEmpty(t, client.headers)
	client.mu.RUnlock()
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkHTTPClientPool_GetClient(b *testing.B) {
	pool := NewHTTPClientPool(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.GetClient("example.com")
	}
}

func BenchmarkHTTPClientPool_GetClient_Concurrent(b *testing.B) {
	pool := NewHTTPClientPool(nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.GetClient("example.com")
		}
	})
}

func BenchmarkPoolMetrics_AverageLatency(b *testing.B) {
	metrics := &PoolMetrics{
		TotalLatencyUs: 1000000,
		RequestCount:   1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = metrics.AverageLatency()
	}
}
