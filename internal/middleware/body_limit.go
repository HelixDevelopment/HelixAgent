package middleware

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

// DefaultMaxRequestBodySize caps the size of an inbound HTTP request body
// for any POST/PUT/PATCH/DELETE request passing through the router. This
// is a broad safety net against memory-exhaustion from maliciously large
// payloads — the typical HelixAgent request (chat completion, debate
// submission, embedding batch) is well under 1 MB, and legitimate large
// ingests (document upload, RAG batch) go through dedicated endpoints
// that can opt out via their own middleware chain.
//
// 10 MiB was chosen as the default based on: the largest legitimate chat
// completion request observed in production is ~2 MB (long conversation
// history plus base64 vision input); 10 MB gives a 5x headroom without
// exposing the process to attacker-controlled allocations larger than
// that. Override via the MAX_REQUEST_BODY_BYTES env var for callers that
// need something different (e.g. bulk ingestion workloads).
const DefaultMaxRequestBodySize int64 = 10 * 1024 * 1024

// BodyLimit returns a Gin middleware that rejects any request whose body
// exceeds maxBytes. The check is performed in two places:
//
//  1. Content-Length header — if present and above the cap, the request
//     is rejected with 413 Payload Too Large before any body read.
//  2. http.MaxBytesReader on r.Body — bounds the actual read so handlers
//     that call c.Bind / c.ShouldBindJSON / io.ReadAll see an error at
//     the cap rather than allocating an unbounded buffer.
//
// Methods without a body (GET, HEAD, OPTIONS) pass through unchanged.
//
// The env var MAX_REQUEST_BODY_BYTES (integer, in bytes) overrides
// maxBytes at middleware construction time. Values ≤0 disable the limit
// entirely — use with extreme caution.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	if env := os.Getenv("MAX_REQUEST_BODY_BYTES"); env != "" {
		if v, err := strconv.ParseInt(env, 10, 64); err == nil {
			maxBytes = v
		}
	}
	// A zero or negative cap disables the middleware's enforcement
	// entirely but still installs the no-op handler so callers can
	// wire it unconditionally.
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		// Content-Length fast path — cheap rejection for callers that
		// advertise a too-large payload up front.
		if cl := c.Request.ContentLength; cl > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":       "request body too large",
				"max_bytes":   maxBytes,
				"got_bytes":   cl,
				"retry_after": "never",
			})
			return
		}

		// Enforce a hard cap on the actual read. Any handler that calls
		// io.ReadAll(c.Request.Body) or c.ShouldBindJSON will now see
		// a MaxBytesError at the cap rather than allocating until OOM.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()

		// Post-handler check: if MaxBytesReader tripped and the handler
		// propagated the error, surface 413 explicitly rather than a
		// generic 400 so the client can distinguish size-limit from
		// malformed-body failures. We only overwrite the status if
		// nothing has been written yet (the handler may have already
		// set a 400 with a specific parse error).
		if !c.Writer.Written() {
			for _, ginErr := range c.Errors {
				var mbe *http.MaxBytesError
				if errors.As(ginErr.Err, &mbe) {
					c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
						"error":     "request body too large",
						"max_bytes": maxBytes,
					})
					return
				}
			}
		}
	}
}
