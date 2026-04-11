package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newBodyLimitRouter builds a tiny Gin router with the body-limit
// middleware installed and a POST /echo handler that attempts to read
// the whole body. Used by every test below.
func newBodyLimitRouter(t *testing.T, maxBytes int64) *gin.Engine {
	t.Helper()
	r := gin.New()
	r.Use(BodyLimit(maxBytes))
	r.POST("/echo", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			// Propagate the error via c.Errors so the post-handler
			// MaxBytesError detector can see it and surface 413.
			_ = c.Error(err)
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", body)
	})
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	return r
}

func TestBodyLimit_SmallRequestAccepted(t *testing.T) {
	r := newBodyLimitRouter(t, 1024)

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hello"))
	req.ContentLength = 5
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello", w.Body.String())
}

func TestBodyLimit_ExactSizeAccepted(t *testing.T) {
	// A body exactly at the cap must succeed — MaxBytesReader permits
	// reads up to AND including n bytes.
	r := newBodyLimitRouter(t, 16)

	payload := strings.Repeat("a", 16)
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(payload))
	req.ContentLength = 16
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, payload, w.Body.String())
}

func TestBodyLimit_ContentLengthOverCapRejected(t *testing.T) {
	// Fast-path rejection: a declared Content-Length above the cap
	// must produce 413 WITHOUT touching the body.
	r := newBodyLimitRouter(t, 16)

	payload := strings.Repeat("b", 17)
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(payload))
	req.ContentLength = 17
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "too large")
	assert.Contains(t, w.Body.String(), "max_bytes")
}

func TestBodyLimit_ActualReadOverCapRejected(t *testing.T) {
	// A client that lies about Content-Length (or sends a chunked
	// request with no declared length) must still be caught by the
	// MaxBytesReader wrapping of r.Body.
	r := newBodyLimitRouter(t, 8)

	payload := strings.Repeat("c", 32)
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(payload))
	req.ContentLength = -1 // simulate chunked / unknown length
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Either the post-handler 413 detector wins, or the handler's own
	// error propagation via c.Error surfaces it. Both are acceptable
	// outcomes so long as the request is NOT treated as successful.
	assert.NotEqual(t, http.StatusOK, w.Code,
		"handler should not return 200 when the body read exceeded the cap")
	if w.Code != http.StatusRequestEntityTooLarge {
		// Some handlers return 400 on body-read error. The important
		// invariant is that the response is not 200 and no truncated
		// data made it through.
		assert.Less(t, w.Code, 500,
			"unexpected status %d: want 413 or 4xx", w.Code)
	}
}

func TestBodyLimit_GetRequestPassthrough(t *testing.T) {
	// GET / HEAD / OPTIONS have no body and must bypass the check
	// entirely — a 100-GB GET request with no Content-Length header
	// is a nonsense case but must not produce 413.
	r := newBodyLimitRouter(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}

func TestBodyLimit_ZeroCapDisablesMiddleware(t *testing.T) {
	// A zero or negative cap opts out entirely — used by endpoints
	// that stream large uploads and manage their own bounds.
	r := newBodyLimitRouter(t, 0)

	payload := strings.Repeat("z", 100_000)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(payload)))
	req.ContentLength = int64(len(payload))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, len(payload), w.Body.Len())
}

func TestBodyLimit_EnvOverride(t *testing.T) {
	// MAX_REQUEST_BODY_BYTES env var overrides the compile-time cap.
	t.Setenv("MAX_REQUEST_BODY_BYTES", "4")

	r := gin.New()
	r.Use(BodyLimit(1024)) // compile-time cap is 1024, env overrides to 4
	r.POST("/echo", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		c.Data(http.StatusOK, "text/plain", body)
	})

	payload := strings.Repeat("q", 5)
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(payload))
	req.ContentLength = 5
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code,
		"env var MAX_REQUEST_BODY_BYTES=4 should force rejection of the 5-byte body")
}

func TestBodyLimit_DefaultConstantIsSensible(t *testing.T) {
	// Pin the default cap so a future change is a visible decision,
	// not a silent regression. 10 MiB should be enough for every
	// legitimate chat completion payload.
	assert.Equal(t, int64(10*1024*1024), DefaultMaxRequestBodySize)
}
