package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/cache"
	"dev.helix.agent/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// HXC-283 — the rate-limit key derived from a caller's remote address must be
// taken apart with the standard net.SplitHostPort routine, never a home-grown
// split and never by trusting an unparsed fallback string, so ONE real caller
// yields EXACTLY ONE identity across every request shape (old-style,
// modern-style, bracketed, zone-carrying and hostname), and no caller's
// identity retains port or bracket punctuation.
//
// §11.4.115 RED-baseline-on-the-broken-artifact + polarity switch.
//
//	RED_MODE=1        — assert the DEFECT IS PRESENT (reproduces on the
//	                     pre-fix tree; kept as a manual reproduction tool,
//	                     never the standing GREEN oracle — see §11.4.115).
//	RED_MODE unset/"0" — assert the defect is ABSENT. THIS IS THE DEFAULT.
//
// THE PRODUCT DEFECT (HXC-283, verified — not assumed — against this exact
// tree; see docs/research/hxc283_addr_split/ for the full investigation).
//
// defaultKeyFunc() delegates address parsing to gin.Context.ClientIP(),
// which is itself correct (it runs net.SplitHostPort internally — verified
// against github.com/gin-gonic/gin@v1.12.0 context.go:1027-1033). The bug is
// NOT a literal "split at the last colon" in this file; it is that
// ClientIP() legitimately returns "" not only for malformed input but also
// for entirely REAL modern-style addresses it does not itself resolve to a
// bare numeric IP — an RFC-4007 zone-qualified IPv6 literal such as
// "fe80::1%eth0" (net.ParseIP rejects the zone suffix), or a hostname-shaped
// RemoteAddr — and on that fallback the pre-fix code did:
//
//	ip := c.ClientIP()
//	if ip == "" {
//	    ip = c.Request.RemoteAddr   // raw, UNPARSED: port + brackets leak
//	}
//	return "ip:" + ip
//
// — never applying net.SplitHostPort itself. CONSEQUENCE, empirically
// confirmed below: a bracketed IPv6 literal with no port keeps its square
// brackets ("ip:[2001:db8::1]"), while the identical address WITH a port
// strips them via ClientIP() ("ip:2001:db8::1") — one real caller, two
// identities (IDENTITY-SPLITTING). Worse: a zone-carrying or hostname caller
// falls into this branch on EVERY request, and the raw fallback ALSO retains
// the ephemeral TCP source port, which differs on every connection — so that
// caller gets a brand-new identity (and therefore a fresh, full token
// bucket) on every single request: the limiter never throttles it at all.
// This is the "lets an abusive caller through by splitting their requests
// across identities" bypass direction, and it is the direction this defect
// actually takes for every realistic address form in the census below — see
// TestDefaultKeyFunc_SameCallerAcrossPortsGetsOneBucket for the sharpest,
// end-to-end proof. A secondary IDENTITY-MERGING manifestation exists too
// (an empty RemoteAddr, or one with an empty host, collapses onto the SAME
// blank "ip:" key as any other caller in that state) — see
// TestDefaultKeyFunc_UnknownCallersDeliberatelyShareOneBucket for how the fix
// handles that deliberately rather than by accident.
// ---------------------------------------------------------------------------

// callerKeyFor builds a minimal gin.Context carrying the given RemoteAddr and
// returns whatever defaultKeyFunc (the live production function) derives
// from it.
func callerKeyFor(remoteAddr string) string {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	req := httptest.NewRequest("GET", "/x", nil)
	req.RemoteAddr = remoteAddr
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), router)
	c.Request = req
	return defaultKeyFunc(c)
}

// TestDefaultKeyFunc_AddressFormCensus is the STEP-1/STEP-2 reproduce-then-
// confirm test (§11.4.146): every address form the investigation identified
// as reaching defaultKeyFunc, with the verified pre-fix (red) and required
// post-fix (green) key for each. Every "red" value below was captured by
// running this exact input through the live, unmodified defaultKeyFunc
// before any fix was applied (see the CAPTURED RED BASELINE comment at the
// bottom of this file) — it is not a guessed/expected value.
func TestDefaultKeyFunc_AddressFormCensus(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	tests := []struct {
		name       string
		remoteAddr string
		red        string // verified pre-fix output (the defect, present)
		green      string // required post-fix output (the defect, absent)
	}{
		// --- old-style (IPv4) ---------------------------------------------
		{"ipv4_with_port", "203.0.113.5:54321", "ip:203.0.113.5", "ip:203.0.113.5"},
		{"ipv4_no_port", "203.0.113.5", "ip:203.0.113.5", "ip:203.0.113.5"},

		// --- modern-style (IPv6), bracketed --------------------------------
		{"ipv6_bracketed_with_port", "[2001:db8::1]:54321", "ip:2001:db8::1", "ip:2001:db8::1"},
		{"ipv6_bracketed_no_port", "[2001:db8::1]", "ip:[2001:db8::1]", "ip:2001:db8::1"},

		// --- modern-style (IPv6), unbracketed -------------------------------
		{"ipv6_unbracketed_no_port", "2001:db8::1", "ip:2001:db8::1", "ip:2001:db8::1"},
		{"ipv6_unbracketed_ambiguous_trailing_digits", "2001:db8::1:54321", "ip:2001:db8::1:54321", "ip:2001:db8::1:54321"},

		// --- zone-carrying (RFC 4007 link-local) ---------------------------
		{"ipv6_zone_bracketed_with_port", "[fe80::1%eth0]:54321", "ip:[fe80::1%eth0]:54321", "ip:fe80::1%eth0"},
		{"ipv6_zone_bracketed_no_port", "[fe80::1%eth0]", "ip:[fe80::1%eth0]", "ip:fe80::1%eth0"},
		{"ipv6_zone_unbracketed_no_port", "fe80::1%eth0", "ip:fe80::1%eth0", "ip:fe80::1%eth0"},

		// --- hostname callers -----------------------------------------------
		{"hostname_with_port", "example.internal:8443", "ip:example.internal:8443", "ip:example.internal"},
		{"hostname_no_port", "example.internal", "ip:example.internal", "ip:example.internal"},

		// --- boundary / malformed --------------------------------------------
		{"empty_remote_addr", "", "ip:", "ip:unknown"},
		{"empty_host_with_port", ":54321", "ip::54321", "ip:unknown"},
		{"garbage_no_colon", "@", "ip:@", "ip:@"},

		// Empty brackets, no port ("[]"): SplitHostPort fails (no ":port"
		// suffix, "missing port in address"), so this falls to the raw-string
		// branch; the bracket-strip guard then strips "[" and "]" leaving an
		// empty string. Pins the len(raw)>=2 boundary the reviewer's M2
		// mutation (>=2 -> >=3) targets: at >=3 a 2-byte "[]" no longer
		// satisfies the guard and falls through to the raw "ip:[]" (the red
		// value below) instead of the stripped "ip:" (the green value). Both
		// values were CAPTURED by running this exact input through the live
		// defaultKeyFunc pre-fix and post-fix (see the HXC-283 gap-fix
		// follow-up report) - neither is a guessed/expected value.
		{"empty_brackets_no_port", "[]", "ip:[]", "ip:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := callerKeyFor(tt.remoteAddr)
			if redMode {
				assert.Equal(t, tt.red, got,
					"RED_MODE=1: RemoteAddr %q expected to reproduce the pre-fix "+
						"key %q; got %q — defect did not reproduce under this "+
						"forcing, this is a FINDING not evidence of a fix",
					tt.remoteAddr, tt.red, got)
				return
			}
			assert.Equal(t, tt.green, got,
				"RemoteAddr %q must derive key %q (net.SplitHostPort, no leaked "+
					"port/brackets); got %q", tt.remoteAddr, tt.green, got)
		})
	}
}

// TestDefaultKeyFunc_SameCallerAcrossPortsGetsOneBucket is the sharpest proof
// of the bypass direction: it simulates the SAME real caller making several
// requests over several TCP connections (each with a different ephemeral
// source port, which is exactly what happens on the wire) via a hostname
// RemoteAddr and via a zone-carrying IPv6 RemoteAddr — the two forms the
// census shows ALWAYS miss gin's ClientIP() fast path. Pre-fix, every
// connection derives a DIFFERENT key (the leaked port varies per
// connection), so the caller never re-uses a bucket and is never throttled.
// Post-fix, every connection derives the SAME key.
func TestDefaultKeyFunc_SameCallerAcrossPortsGetsOneBucket(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	callers := []struct {
		name  string
		ports []string // same host, different remote addr per "connection"
	}{
		{
			name: "hostname_caller",
			ports: []string{
				"example.internal:8443",
				"example.internal:9001",
				"example.internal:51234",
			},
		},
		{
			name: "zone_ipv6_caller",
			ports: []string{
				"[fe80::1%eth0]:8443",
				"[fe80::1%eth0]:9001",
				"[fe80::1%eth0]:51234",
			},
		},
	}

	for _, caller := range callers {
		t.Run(caller.name, func(t *testing.T) {
			keys := make(map[string]struct{})
			for _, addr := range caller.ports {
				keys[callerKeyFor(addr)] = struct{}{}
			}

			if redMode {
				assert.Greaterf(t, len(keys), 1,
					"RED_MODE=1: caller %s expected to fragment across %d "+
						"connections into more than one bucket key (observed "+
						"keys=%v) — this IS the bypass: a wrong-direction "+
						"finding here means the defect did not reproduce",
					caller.name, len(caller.ports), keys)
				return
			}

			assert.Lenf(t, keys, 1,
				"the same caller (%s) reaching us over %d different TCP "+
					"connections must derive exactly ONE rate-limit identity, "+
					"not %d (observed keys=%v) — a caller that changes identity "+
					"per connection is never throttled",
				caller.name, len(caller.ports), len(keys), keys)
		})
	}
}

// TestDefaultKeyFunc_UnknownCallersDeliberatelyShareOneBucket documents the
// error-path decision explicitly, per the task's requirement to decide
// deliberately what identity an unparseable address maps to rather than let
// it silently collapse. Both a genuinely-empty RemoteAddr and a
// SplitHostPort-parseable-but-empty-host RemoteAddr (":54321") are collapsed
// onto a single, explicitly-labelled "ip:unknown" bucket post-fix — this is
// intentional and FAIL-CLOSED (still rate-limited, as one shared identity),
// not an accidental new merge introduced by the fix: the pre-fix code
// ALREADY collapsed these onto a shared identity (the blank "ip:"), just
// without a deliberate label, and without doing it for ":54321" at all — see
// the census above for both rows.
func TestDefaultKeyFunc_UnknownCallersDeliberatelyShareOneBucket(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	keyEmpty := callerKeyFor("")
	keyEmptyHost := callerKeyFor(":54321")

	if redMode {
		// Pre-fix: both collapse, but NOT onto the same key as each other —
		// "" derives "ip:" while ":54321" derives "ip::54321" (the port
		// leaks because SplitHostPort succeeds here) — i.e. the merge is
		// accidental and inconsistent, not deliberate.
		assert.Equal(t, "ip:", keyEmpty)
		assert.Equal(t, "ip::54321", keyEmptyHost)
		assert.NotEqual(t, keyEmpty, keyEmptyHost,
			"RED_MODE=1: pre-fix, an empty RemoteAddr and an empty-host "+
				"RemoteAddr with a port must NOT already share one key — if "+
				"they do, the pre-fix baseline captured here is stale")
		return
	}

	assert.Equal(t, "ip:unknown", keyEmpty)
	assert.Equal(t, "ip:unknown", keyEmptyHost)
}

// TestRateLimiterMiddleware_SameCallerReconnectingBypassesLimit is the
// end-to-end proof, driven through the real RateLimiter.Middleware() (not
// the pure key function), that the identity-split lets an abusive caller
// through: it exhausts a 3-request limit for a zone-carrying IPv6 caller,
// then re-issues the SAME logical request over three MORE distinct source
// ports. Pre-fix every reconnect derives a fresh identity and is accepted
// (200) regardless of the configured limit — the limiter provides zero
// protection for this caller. Post-fix every reconnect shares the exhausted
// caller's bucket and is correctly rejected (429).
func TestRateLimiterMiddleware_SameCallerReconnectingBypassesLimit(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	redMode := os.Getenv("RED_MODE") == "1"

	cfg := &config.Config{Redis: config.RedisConfig{Host: "localhost", Port: "6379"}}
	cacheService, _ := cache.NewCacheService(cfg)
	limiter := NewRateLimiterWithConfig(cacheService, &RateLimitConfig{
		Requests: 3,
		Window:   time.Minute,
		KeyFunc:  defaultKeyFunc,
	})
	defer limiter.Stop()

	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	sameCallerNewPortEachTime := func(port int) int {
		req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
		req.RemoteAddr = fmt.Sprintf("[fe80::1%%eth0]:%d", port)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	// Exhaust the 3-request limit for this caller.
	var lastCode int
	for i := 0; i < 3; i++ {
		lastCode = sameCallerNewPortEachTime(40000 + i)
	}
	require.Equal(t, http.StatusOK, lastCode, "the 3 requests within the configured limit must be accepted")

	// The 4th request from a DIFFERENT source port, same real caller.
	bypassCode := sameCallerNewPortEachTime(40099)

	if redMode {
		require.Equal(t, http.StatusOK, bypassCode,
			"RED_MODE=1: the 4th request from a new source port for the SAME "+
				"caller is expected to bypass the exhausted limit (200) — if it "+
				"was rejected, the defect did not reproduce under this forcing")
		return
	}

	require.Equal(t, http.StatusTooManyRequests, bypassCode,
		"the 4th request is the SAME real caller reconnecting on a new source "+
			"port and MUST still be rejected — a 200 here means the caller's "+
			"identity changed across connections and the limit was bypassed")
}

// TestRateLimiterMiddleware_ConcurrentSameCallerDifferentPorts is the
// concurrent-load case: rate limiting is concurrent by nature, and a
// per-caller bucket keyed on a mis-parsed identity is exactly where this
// bites hardest — N simultaneous connections from the SAME abusive caller,
// each with a distinct source port, hammering the limiter at once. Pre-fix
// every one of them lands in its own fresh bucket and is accepted regardless
// of the configured limit. Post-fix exactly `Requests` of them are accepted
// and the rest are rejected, proven under N >= 100 concurrent goroutines per
// §11.4.85.
func TestRateLimiterMiddleware_ConcurrentSameCallerDifferentPorts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redMode := os.Getenv("RED_MODE") == "1"

	const (
		limit       = 5
		concurrency = 100
	)

	cfg := &config.Config{Redis: config.RedisConfig{Host: "localhost", Port: "6379"}}
	cacheService, _ := cache.NewCacheService(cfg)
	limiter := NewRateLimiterWithConfig(cacheService, &RateLimitConfig{
		Requests: limit,
		Window:   time.Minute,
		KeyFunc:  defaultKeyFunc,
	})
	defer limiter.Stop()

	router := gin.New()
	router.Use(limiter.Middleware())
	router.GET("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	var (
		wg       sync.WaitGroup
		accepted atomic.Int64
		rejected atomic.Int64
	)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
			// SAME real caller (same hostname), a different ephemeral source
			// port per simulated connection - exactly what a real burst from
			// one abusive client looks like on the wire.
			req.RemoteAddr = fmt.Sprintf("api-gateway.internal:%d", 30000+port)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			switch w.Code {
			case http.StatusOK:
				accepted.Add(1)
			case http.StatusTooManyRequests:
				rejected.Add(1)
			}
		}(i)
	}
	wg.Wait()

	t.Logf("concurrent same-caller burst: accepted=%d rejected=%d (limit=%d, concurrency=%d)",
		accepted.Load(), rejected.Load(), limit, concurrency)

	if redMode {
		require.Equal(t, int64(concurrency), accepted.Load(),
			"RED_MODE=1: every one of the %d concurrent connections from the "+
				"SAME caller is expected to fragment into its own fresh bucket "+
				"and be accepted, entirely defeating the configured limit of %d — "+
				"observed accepted=%d rejected=%d; if fewer than %d were "+
				"accepted, the defect did not reproduce under this forcing",
			concurrency, limit, accepted.Load(), rejected.Load(), concurrency)
		return
	}

	require.Equal(t, int64(limit), accepted.Load(),
		"exactly the configured limit (%d) of the %d concurrent requests from "+
			"the SAME caller must be accepted regardless of source-port "+
			"variance; observed accepted=%d rejected=%d",
		limit, concurrency, accepted.Load(), rejected.Load())
	require.Equal(t, int64(concurrency-limit), rejected.Load())
}

// ---------------------------------------------------------------------------
// CAPTURED RED BASELINE (2026-08-12, pre-fix tree, dev.helix.agent
// internal/middleware/rate_limit.go sha256 00056a3f6ae761b8f185d2059199b25
// 0e3cb213d7648458d09ebb87d63795967):
//
//	RED_MODE=1 go test -run 'TestDefaultKeyFunc_AddressFormCensus|TestDefaultKeyFunc_SameCallerAcrossPortsGetsOneBucket|TestDefaultKeyFunc_UnknownCallersDeliberatelyShareOneBucket|TestRateLimiterMiddleware_SameCallerReconnectingBypassesLimit|TestRateLimiterMiddleware_ConcurrentSameCallerDifferentPorts' -v -count=1 ./internal/middleware/
//	  -> ok  dev.helix.agent/internal/middleware  4.177s   PASS (all five
//	     reproduce the defect verbatim); the concurrent-burst reproduction
//	     logged "concurrent same-caller burst: accepted=100 rejected=0
//	     (limit=5, concurrency=100)" — a 5-request limit provided ZERO
//	     protection for a single reconnecting caller.
//
//	go test (RED_MODE unset, i.e. the default) -run '<same>' -v -count=1 ./internal/middleware/
//	  -> FAIL  dev.helix.agent/internal/middleware  4.180s. Observed failures:
//	     TestDefaultKeyFunc_AddressFormCensus: ipv6_bracketed_no_port
//	     (want "ip:2001:db8::1" got "ip:[2001:db8::1]"), ipv6_zone_bracketed_
//	     with_port (want "ip:fe80::1%eth0" got "ip:[fe80::1%eth0]:54321"),
//	     ipv6_zone_bracketed_no_port (want "ip:fe80::1%eth0" got
//	     "ip:[fe80::1%eth0]"), hostname_with_port (want "ip:example.internal"
//	     got "ip:example.internal:8443"), empty_remote_addr (want
//	     "ip:unknown" got "ip:"), empty_host_with_port (want "ip:unknown" got
//	     "ip::54321"). TestDefaultKeyFunc_SameCallerAcrossPortsGetsOneBucket:
//	     hostname_caller and zone_ipv6_caller both fragmented into 3 distinct
//	     keys instead of 1 (e.g. "map[ip:example.internal:51234:{}
//	     ip:example.internal:8443:{} ip:example.internal:9001:{}]").
//	     TestDefaultKeyFunc_UnknownCallersDeliberatelyShareOneBucket: got
//	     "ip:" and "ip::54321" instead of "ip:unknown" for both.
//	     TestRateLimiterMiddleware_ConcurrentSameCallerDifferentPorts:
//	     "accepted=100" (want 5) — every one of 100 concurrent connections
//	     from ONE caller was accepted. TestRateLimiterMiddleware_
//	     SameCallerReconnectingBypassesLimit: the 4th request (new source
//	     port, same caller, limit already exhausted) got 200 (want 429).
//
// The RED_MODE=0 (default) failure BEFORE the fix, flipping to PASS AFTER
// it (see the fix in rate_limit.go, same commit), is the proof this is not a
// blind test written to agree with the new code.
// ---------------------------------------------------------------------------
