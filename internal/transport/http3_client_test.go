package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTP3Client(t *testing.T) {
	tests := []struct {
		name   string
		config *HTTP3ClientConfig
		want   *HTTP3Client
	}{
		{
			name:   "default config",
			config: nil,
		},
		{
			name: "custom config",
			config: &HTTP3ClientConfig{
				EnableHTTP3:  false,
				EnableHTTP2:  true,
				EnableBrotli: false,
				Timeout:      60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTP3Client(tt.config)
			require.NotNil(t, client)
			assert.NotNil(t, client.Client)
			assert.NotNil(t, client.HTTPClient())
		})
	}
}

func TestHTTP3Client_Do(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	}))
	defer server.Close()

	client := NewHTTP3Client(nil)
	defer client.Close()

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"status":"ok"`)
}

func TestHTTP3Client_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"method":"GET"}`)) //nolint:errcheck
	}))
	defer server.Close()

	client := NewHTTP3Client(nil)
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTP3Client_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, `{"test":"data"}`, string(body))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"created"}`)) //nolint:errcheck
	}))
	defer server.Close()

	client := NewHTTP3Client(nil)
	defer client.Close()

	ctx := context.Background()
	body := strings.NewReader(`{"test":"data"}`)
	resp, err := client.Post(ctx, server.URL, "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHTTP3Client_PostJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, `{"key":"value"}`, string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTP3Client(nil)
	defer client.Close()

	ctx := context.Background()
	resp, err := client.PostJSON(ctx, server.URL, []byte(`{"key":"value"}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTP3Client_Retry(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultHTTP3ClientConfig()
	config.MaxRetries = 3
	config.RetryDelay = 10 * time.Millisecond
	client := NewHTTP3Client(config)
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, attemptCount)
}

func TestHTTP3Client_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTP3Client(nil)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestHTTP3Client_DecompressBrotli(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []byte(`{"compressed":true,"data":"test data"}`)

		w.Header().Set("Content-Encoding", "br")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		bw := brotli.NewWriter(w)
		bw.Write(data) //nolint:errcheck
		bw.Close()     //nolint:errcheck
	}))
	defer server.Close()

	config := DefaultHTTP3ClientConfig()
	config.EnableBrotli = true
	client := NewHTTP3Client(config)
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Contains(t, string(body), `"compressed":true`)
}

func TestHTTP3Client_SetTimeout(t *testing.T) {
	client := NewHTTP3Client(nil)
	defer client.Close()

	newTimeout := 60 * time.Second
	client.SetTimeout(newTimeout)

	assert.Equal(t, newTimeout, client.GetTimeout())
}

func TestHTTP3Client_GetConfig(t *testing.T) {
	config := &HTTP3ClientConfig{
		Timeout:    90 * time.Second,
		MaxRetries: 5,
	}
	client := NewHTTP3Client(config)
	defer client.Close()

	gotConfig := client.GetConfig()
	assert.Equal(t, config.Timeout, gotConfig.Timeout)
	assert.Equal(t, config.MaxRetries, gotConfig.MaxRetries)
}

func TestDefaultHTTP3ClientConfig(t *testing.T) {
	config := DefaultHTTP3ClientConfig()

	assert.True(t, config.EnableHTTP3)
	assert.True(t, config.EnableHTTP2)
	assert.True(t, config.EnableBrotli)
	assert.Equal(t, 120*time.Second, config.Timeout)
	assert.Equal(t, 30*time.Second, config.DialTimeout)
	assert.Equal(t, 100, config.MaxIdleConns)
	assert.Equal(t, 3, config.MaxRetries)
	assert.NotNil(t, config.TLSConfig)
}

func TestHTTP3Client_isRetryableError(t *testing.T) {
	client := NewHTTP3Client(nil)
	defer client.Close()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"connection refused", assert.AnError, false},
		{"timeout", &timeoutError{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.err != nil && tt.err != assert.AnError {
				err = tt.err
			} else if tt.name == "connection refused" {
				err = assert.AnError
			} else if tt.name == "nil error" {
				err = nil
			}
			got := client.isRetryableError(err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHTTP3Client_nextDelay(t *testing.T) {
	client := NewHTTP3Client(nil)
	defer client.Close()

	tests := []struct {
		currentDelay time.Duration
		expectedMax  time.Duration
	}{
		{1 * time.Second, 2 * time.Second},
		{15 * time.Second, 30 * time.Second},
		{20 * time.Second, 30 * time.Second}, // Should cap at MaxRetryDelay
	}

	for _, tt := range tests {
		t.Run(tt.currentDelay.String(), func(t *testing.T) {
			next := client.nextDelay(tt.currentDelay)
			assert.LessOrEqual(t, next, tt.expectedMax)
			assert.GreaterOrEqual(t, next, tt.currentDelay)
		})
	}
}

func TestHTTP3RoundTripper(t *testing.T) {
	t.Run("with HTTP/3 enabled", func(t *testing.T) {
		rt := NewHTTP3RoundTripper(true)
		require.NotNil(t, rt)
		defer rt.Close()
	})

	t.Run("with HTTP/3 disabled", func(t *testing.T) {
		rt := NewHTTP3RoundTripper(false)
		require.NotNil(t, rt)
		defer rt.Close()
	})
}

func TestGetGlobalHTTP3Client(t *testing.T) {
	// Reset to ensure clean state
	globalHTTP3Client = nil
	globalHTTP3ClientOnce = sync.Once{}

	client1 := GetGlobalHTTP3Client()
	require.NotNil(t, client1)

	client2 := GetGlobalHTTP3Client()
	require.NotNil(t, client2)

	// Should be the same instance
	assert.Equal(t, client1, client2)
}

func TestSetGlobalHTTP3Client(t *testing.T) {
	customClient := NewHTTP3Client(&HTTP3ClientConfig{
		Timeout: 5 * time.Minute,
	})

	SetGlobalHTTP3Client(customClient)
	assert.Equal(t, customClient, globalHTTP3Client)

	// Reset for other tests
	ResetGlobalHTTP3Client()
}

func TestResetGlobalHTTP3Client(t *testing.T) {
	ResetGlobalHTTP3Client()
	require.NotNil(t, globalHTTP3Client)
	assert.Equal(t, 120*time.Second, globalHTTP3Client.config.Timeout)
}

// timeoutError is a mock error that implements timeout
type timeoutError struct{}

func (e *timeoutError) Error() string { return "timeout" }
func (e *timeoutError) Timeout() bool { return true }

func TestBrotliReadCloser(t *testing.T) {
	data := []byte("Hello, World!")

	// Compress data
	var compressed strings.Builder
	bw := brotli.NewWriter(&compressed)
	bw.Write(data) //nolint:errcheck
	bw.Close()     //nolint:errcheck

	// Create reader
	pr := &mockReadCloser{Reader: strings.NewReader(compressed.String())}
	br := &brotliReadCloser{
		reader: brotli.NewReader(pr),
		closer: pr,
	}

	// Read decompressed data
	result, err := io.ReadAll(br)
	require.NoError(t, err)
	assert.Equal(t, data, result)

	// Close
	err = br.Close()
	assert.NoError(t, err)
	assert.True(t, pr.closed)
}

type mockReadCloser struct {
	*strings.Reader
	closed bool
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}
