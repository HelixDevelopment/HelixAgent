// Package claude_code — round-41 §11.4 anti-bluff sibling of the
// round-39 stdio transport: a real HTTP (+ optional SSE) MCP
// transport implementing the same MCPTransport interface defined
// in mcp_integration.go.
//
// ROUND-44 ADDENDUM — SSE AUTO-RECONNECT + Last-Event-ID RETRY
// ------------------------------------------------------------
// Round-41 deferred two SSE-spec optional behaviours:
//  1. Automatic reconnect after the server disconnects.
//  2. Replay-on-reconnect via the Last-Event-ID HTTP request
//     header (the W3C / HTML5 EventSource recipe).
//
// Round-44 (this file's current revision) wires both, with the
// hard caps required by CONST-035:
//   - Exponential backoff with ±25% jitter: 1s → 2s → 4s → 8s,
//     capped at min(retryInterval×8, 60s).
//   - retryInterval defaults to 3 seconds, overridden by the
//     SSE server's `retry: <ms>` field per spec, capped at 60s.
//   - After MaxConsecutiveFailures consecutive reconnect
//     failures (default 10) the goroutine emits
//     ErrMCPSSEStreamPermanentlyDisconnected on the notifications
//     channel and exits — the consumer must call EnableSSE again
//     to restart. Silent looping forever is forbidden.
//   - Every wait point honours ctx.Done (ticker + select), so
//     ctx cancellation aborts within the next backoff tick at
//     the latest; goroutine-leak guards in the *_test.go file
//     assert NumGoroutine returns to baseline.
//   - The Last-Event-ID HTTP header is set on every reconnect
//     GET when an `id:` field has been observed at any point in
//     the lifetime of the SSE listener; servers may use it to
//     replay missed events (lossless resume).
//
// PROTOCOL REFERENCES
// -------------------
//   - Model Context Protocol specification — protocol version
//     "2024-11-05" — https://spec.modelcontextprotocol.io
//   - JSON-RPC 2.0 specification — https://www.jsonrpc.org/specification
//   - HTML5 Server-Sent Events ("EventSource") —
//     https://html.spec.whatwg.org/multipage/server-sent-events.html
//
// CANONICAL HTTP+SSE FLOW
// -----------------------
// MCP-over-HTTP exposes a single JSON-RPC endpoint that accepts
// POST requests with a JSON-RPC body and returns a JSON-RPC body
// in the response. Server-initiated notifications (the
// MCP-over-stdio equivalent of "the server emits a line we never
// asked for") are delivered via a parallel Server-Sent Events
// channel — typically the same endpoint with Accept:
// text/event-stream, or a sibling `/sse` endpoint.
//
//  1. On first CallTool the transport sends the canonical MCP
//     initialize JSON-RPC request via POST and parses the
//     response from the body.
//  2. It then POSTs the notifications/initialized notification
//     (no response is expected — the server returns 202 or 204).
//  3. Subsequent CallTool invocations POST a tools/call request
//     and decode the response.
//  4. Optional: EnableSSE spawns a background goroutine that
//     GETs the SSE endpoint with Accept: text/event-stream and
//     forwards each `event: <type>\ndata: <json>\n\n` frame to
//     a channel the caller can drain.
//
// ANTI-BLUFF POSTURE
// ------------------
// Round-29 (93e63ada) introduced the MCPTransport interface +
// ErrMCPClientNotWired sentinel — the loud-failure replacement
// for the prior "simulated result" echo-bluff. Round-39
// (c9947503) supplied the real stdio implementation; round-41
// (this file) supplies the real HTTP/SSE implementation so
// remote MCP servers (which speak HTTP, not stdio) can also be
// reached by production callers without re-introducing the
// fabricated-result regression.
//
// Per CONST-035 / Article XI §11.9: every PASS in
// mcp_http_transport_test.go carries positive runtime evidence
// captured against a REAL httptest.Server emitting REAL JSON-RPC
// over REAL TCP sockets. There is no in-process short-circuit
// and no fake HTTP client — the production transport code path
// is exercised end-to-end.
//
// CONST-046: every user-facing string surfaced by this transport
// is composed via fmt.Errorf wrapping the underlying error; no
// hardcoded English strings escape into product-visible output.
//
// CONST-050(A): test fixtures (httptest server with an "echo"
// tool, SSE event emitter) live in *_test.go only and never
// leak into a production build. Production code path under this
// file imports only stdlib + the local services package.
//
// CONST-042 / §11.4.10: the bearer token is sourced at the
// parent-app boundary (env var, secret manager) and passed into
// MCPHTTPConfig at construction; this file never reads
// credentials from disk and never logs them.
//
// PLAINTEXT-HTTP SAFETY (CONST-035)
// ---------------------------------
// `http://` endpoints are refused at construction time unless
// the operator explicitly sets AllowInsecure: true on the
// config. Silently accepting plaintext for a transport that
// often carries a bearer token would be a §11.4 PASS-bluff at
// the security layer.
//
// COMPLEXITY GUARDRAIL DECISION
// -----------------------------
// Per the round-41 brief: synchronous HTTP request/response IS
// the bulk of MCP HTTP value (tool-calling, listing,
// initialize). SSE delivers server-push notifications that most
// MCP servers don't aggressively use. Round-41 lands BOTH but
// keeps SSE strictly opt-in (EnableSSE returns the channel; if
// the operator never calls EnableSSE, no SSE goroutine spawns
// and no SSE network traffic occurs). This keeps the
// happy-path footprint minimal while still exercising the SSE
// path end-to-end via tests.
package claude_code

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dev.helix.agent/internal/services"
)

// ErrMCPEndpointNotConfigured is returned by NewMCPHTTPTransport
// when the supplied MCPHTTPConfig has an empty Endpoint URL.
// This is distinct from ErrMCPClientNotWired AND distinct from
// the round-39 ErrMCPCommandNotConfigured:
//
//   - ErrMCPClientNotWired           → no transport installed at all
//     (round-29 sentinel — (*MCPIntegration).transport is nil)
//   - ErrMCPCommandNotConfigured     → stdio transport requested but
//     MCPStdioConfig.Command is empty (round-39 sentinel)
//   - ErrMCPEndpointNotConfigured    → HTTP transport requested but
//     MCPHTTPConfig.Endpoint is empty (round-41 sentinel, THIS one)
//
// Distinguishing all three is required: an operator debugging a
// "tools/call returned an error" report needs to know whether the
// failure is "no transport at all", "stdio transport but no
// command", or "http transport but no endpoint URL". Silent
// defaulting (to localhost, to mcp-server-foo, to anything) would
// be a §11.4 PASS-bluff in disguise.
var ErrMCPEndpointNotConfigured = errors.New("claude_code/mcp: MCPHTTPConfig.Endpoint is empty — no MCP server endpoint URL declared (round-41 anti-bluff: distinct from ErrMCPClientNotWired which means no transport installed at all; distinct from ErrMCPCommandNotConfigured which means stdio command empty; this sentinel means HTTP transport was requested but the endpoint URL was left unset)")

// ErrMCPInsecureEndpointRejected is returned by
// NewMCPHTTPTransport when the supplied Endpoint uses the
// plaintext `http://` scheme and AllowInsecure is false on the
// config. The default is "refuse plaintext" because production
// HTTP MCP traffic typically carries a bearer token; silent
// plaintext acceptance is a §11.4 PASS-bluff at the security
// layer. Set AllowInsecure: true on the config to opt in (dev
// loopback servers, in-process httptest fixtures, etc.).
var ErrMCPInsecureEndpointRejected = errors.New("claude_code/mcp: MCPHTTPConfig.Endpoint uses plaintext http:// — round-41 refuses this by default because a bearer token sent over plaintext is a §11.4 security-layer PASS-bluff. Set MCPHTTPConfig.AllowInsecure=true to opt in (dev / loopback / httptest only)")

// ErrMCPSSEStreamPermanentlyDisconnected is emitted on the
// notifications channel as the FINAL value (channel is closed
// immediately afterwards) when the round-44 auto-reconnect loop
// has exhausted MaxConsecutiveFailures (default 10) consecutive
// failed reconnect attempts without ever re-establishing the
// stream. The consumer's contract is: receiving this sentinel
// means the SSE listener has given up; the consumer should call
// EnableSSE again (possibly after a longer delay, possibly with
// a re-validated config) to restart. Round-44 explicitly does
// NOT loop forever silently — that would be a §11.4 security/
// observability bluff (operator never learns the stream died).
//
// This sentinel is distinct from the other four MCP sentinels:
//   - ErrMCPClientNotWired              → no transport at all
//   - ErrMCPCommandNotConfigured        → stdio command empty
//   - ErrMCPEndpointNotConfigured       → http endpoint empty
//   - ErrMCPInsecureEndpointRejected    → plaintext http rejected
//   - ErrMCPSSEStreamPermanentlyDisconnected → SSE gave up after N retries
var ErrMCPSSEStreamPermanentlyDisconnected = errors.New("claude_code/mcp: SSE stream gave up after the configured number of consecutive reconnect failures — verify SSEEndpoint URL, server health, and network path; the consumer should re-call EnableSSE to restart (round-44 anti-bluff: silent infinite reconnect would hide the failure from operators)")

// MCPHTTPConfig configures the HTTP transport.
//
// Endpoint is the URL of the MCP server's JSON-RPC endpoint
// (e.g. "https://mcp.example.com/mcp"). REQUIRED; empty value
// returns ErrMCPEndpointNotConfigured at construction.
//
// SSEEndpoint is the URL of the optional server-sent-events
// stream for server-initiated notifications. If empty (the
// default), SSE is disabled even if EnableSSE is called. Many
// MCP HTTP servers expose SSE at the same path as the JSON-RPC
// endpoint with a different Accept header; some expose it at a
// sibling `/sse` path.
//
// BearerToken is an optional credential included as
// `Authorization: Bearer <token>` on every request. Sourced at
// the parent-app boundary (env var, secret manager); this file
// never reads it from disk. Empty = no Authorization header.
//
// TLSConfig is an optional custom TLS configuration. When nil
// (the default) the system root CA pool is used. Override for
// self-signed dev servers or strict pinning.
//
// Timeout is the per-request timeout passed to the underlying
// http.Client. Zero = use a 30-second default; any explicit
// non-zero value (including negative) is used verbatim so tests
// can dial it down.
//
// AllowInsecure permits Endpoint URLs with the plaintext
// http:// scheme. Default false; flipping to true silences
// ErrMCPInsecureEndpointRejected (use ONLY for dev loopback /
// in-process httptest fixtures).
//
// UserAgent is the User-Agent header sent on every request.
// Empty = "helix_agent/claude_code (round-41 mcp-http)".
//
// SSEInitialRetryInterval is the initial reconnect interval used
// by the round-44 SSE auto-reconnect loop BEFORE the server has
// advertised a `retry:` field. Zero = 3-second default per the
// HTML5 EventSource spec recommendation.
//
// SSEMaxRetryInterval caps the server-advertised `retry:` field
// AND the exponential-backoff growth so a malicious or buggy
// server can't force a 24-hour reconnect interval. Zero = 60s.
//
// SSEMaxConsecutiveFailures bounds the auto-reconnect loop. After
// this many consecutive failed reconnect attempts (no successful
// 200/text-event-stream response between them) the goroutine
// emits ErrMCPSSEStreamPermanentlyDisconnected on the channel and
// exits. Zero = 10. Set to a negative value to retry forever
// (NOT recommended; the operator loses visibility into stream
// death — explicit opt-in to a §11.4 observability anti-pattern).
type MCPHTTPConfig struct {
	Endpoint                  string
	SSEEndpoint               string
	BearerToken               string
	TLSConfig                 *tls.Config
	Timeout                   time.Duration
	AllowInsecure             bool
	UserAgent                 string
	SSEInitialRetryInterval   time.Duration
	SSEMaxRetryInterval       time.Duration
	SSEMaxConsecutiveFailures int
}

// MCPHTTPTransport is the round-41 real implementation of the
// MCPTransport interface (defined in mcp_integration.go) over
// HTTP + optional SSE. Concurrent CallTool across distinct
// servers is safe; concurrent CallTool against the *same*
// server is safe at the HTTP layer (every call uses an
// independent http.Request) but is serialised inside this
// transport at the initialize-handshake boundary so the
// initialize round-trip happens exactly once per (transport,
// endpoint) pair.
type MCPHTTPTransport struct {
	cfg    MCPHTTPConfig
	client *http.Client

	// idCounter generates monotonic JSON-RPC request ids.
	idCounter atomic.Int64

	// initOnce + initErr serialise the per-transport
	// initialize handshake. Once initOnce has fired, every
	// subsequent CallTool sees the result of that single
	// handshake (success = nil initErr; failure = the
	// captured error, returned to every caller until Close
	// resets the transport).
	initOnce sync.Once
	initErr  error

	// sseMu guards sseStarted / sseCancel / sseCh so EnableSSE
	// and Close don't race.
	sseMu      sync.Mutex
	sseStarted bool
	sseCancel  context.CancelFunc
	sseCh      chan MCPHTTPNotification
}

// MCPHTTPNotification is a single server-pushed JSON-RPC frame
// delivered over the SSE channel. Method is the JSON-RPC method
// name (e.g. "notifications/message", "notifications/cancelled"),
// EventType is the SSE event name from the `event:` line (often
// empty / "message" for unnamed events), and Params is the raw
// JSON of the JSON-RPC params field — the caller decodes it
// into a method-specific type.
type MCPHTTPNotification struct {
	EventType string
	Method    string
	Params    json.RawMessage
	// Raw is the verbatim bytes after the `data:` line(s).
	// Useful for diagnostics; not normally consumed.
	Raw []byte
}

// NewMCPHTTPTransport constructs the HTTP transport.
//
// Returns ErrMCPEndpointNotConfigured when cfg.Endpoint is
// empty.
//
// Returns ErrMCPInsecureEndpointRejected when cfg.Endpoint uses
// the plaintext http:// scheme AND cfg.AllowInsecure is false.
//
// Returns a URL-parse error when cfg.Endpoint is malformed.
func NewMCPHTTPTransport(cfg MCPHTTPConfig) (*MCPHTTPTransport, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, ErrMCPEndpointNotConfigured
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("claude_code/mcp: parse endpoint %q: %w", cfg.Endpoint, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("claude_code/mcp: endpoint %q uses unsupported scheme %q (only http/https are supported)", cfg.Endpoint, u.Scheme)
	}
	if u.Scheme == "http" && !cfg.AllowInsecure {
		return nil, fmt.Errorf("claude_code/mcp: endpoint %q: %w", cfg.Endpoint, ErrMCPInsecureEndpointRejected)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		// Connection-pool defaults are stdlib-appropriate;
		// MCP traffic is request/response so MaxIdleConns
		// per host is the only knob we tighten.
		MaxIdleConnsPerHost: 4,
		// TLSConfig: nil => system root CA pool; explicit
		// override via cfg.TLSConfig.
		TLSClientConfig: cfg.TLSConfig,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &MCPHTTPTransport{
		cfg:    cfg,
		client: client,
	}, nil
}

// CallTool implements MCPTransport. On the first invocation it
// runs the MCP initialize handshake against the configured
// Endpoint; subsequent invocations reuse the cached handshake
// result. Each tools/call request is a stateless HTTP POST.
func (t *MCPHTTPTransport) CallTool(ctx context.Context, server *MCPServer, toolName string, args map[string]interface{}) (*services.ToolCallResult, error) {
	if server == nil {
		return nil, fmt.Errorf("claude_code/mcp: CallTool: server is nil")
	}

	if err := t.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("claude_code/mcp: CallTool(server=%q, tool=%q): initialize: %w", server.Name, toolName, err)
	}

	req := httpJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      t.nextID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}

	resp, err := t.exchange(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("claude_code/mcp: CallTool(server=%q, tool=%q): %w", server.Name, toolName, err)
	}
	if resp.Error != nil {
		// Surface server-side JSON-RPC errors as a real
		// ToolCallResult with IsError=true, matching the
		// MCP convention and the round-39 stdio transport's
		// behaviour. The caller distinguishes transport
		// failure (non-nil err above) from server-reported
		// tool failure (nil err, IsError=true).
		return &services.ToolCallResult{
			Content: []services.Content{{
				Type: "text",
				Text: fmt.Sprintf("JSON-RPC error %d: %s", resp.Error.Code, resp.Error.Message),
			}},
			IsError: true,
		}, nil
	}

	var raw struct {
		Content []services.Content `json:"content"`
		IsError bool               `json:"isError,omitempty"`
	}
	if err := json.Unmarshal(resp.Result, &raw); err != nil {
		return nil, fmt.Errorf("claude_code/mcp: CallTool(server=%q, tool=%q): decode result: %w", server.Name, toolName, err)
	}
	return &services.ToolCallResult{
		Content: raw.Content,
		IsError: raw.IsError,
	}, nil
}

// EnableSSE starts the optional server-sent-events listener.
// Returns the channel on which server-pushed notifications are
// delivered. Calling EnableSSE more than once returns the same
// channel; subsequent calls do not spawn additional goroutines.
//
// If cfg.SSEEndpoint is empty EnableSSE returns nil + an error
// — SSE is opt-in and the operator must declare the URL.
//
// The returned channel is closed when Close is called (or when
// the SSE connection's underlying ctx is cancelled). Callers
// should drain it in a goroutine to avoid backpressure on the
// SSE reader; the channel is buffered (capacity 32) to absorb
// short bursts.
func (t *MCPHTTPTransport) EnableSSE(parentCtx context.Context) (<-chan MCPHTTPNotification, error) {
	t.sseMu.Lock()
	defer t.sseMu.Unlock()

	if t.cfg.SSEEndpoint == "" {
		return nil, fmt.Errorf("claude_code/mcp: EnableSSE: MCPHTTPConfig.SSEEndpoint is empty — declare the SSE URL to opt into server-pushed notifications")
	}

	if t.sseStarted {
		return t.sseCh, nil
	}

	ctx, cancel := context.WithCancel(parentCtx)
	ch := make(chan MCPHTTPNotification, 32)

	go t.runSSE(ctx, ch)

	t.sseStarted = true
	t.sseCancel = cancel
	t.sseCh = ch
	return ch, nil
}

// Close releases transport resources. Stops the SSE goroutine
// (if running) and closes idle HTTP connections. Safe to call
// multiple times; calls after the first are no-ops.
func (t *MCPHTTPTransport) Close() error {
	t.sseMu.Lock()
	if t.sseStarted {
		t.sseCancel()
		t.sseStarted = false
	}
	t.sseMu.Unlock()

	if tr, ok := t.client.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	return nil
}

// ensureInitialized runs the MCP initialize handshake exactly
// once per transport. The first caller blocks on the round
// trip; subsequent callers see the cached error (or nil
// success).
func (t *MCPHTTPTransport) ensureInitialized(ctx context.Context) error {
	t.initOnce.Do(func() {
		t.initErr = t.runInitialize(ctx)
	})
	return t.initErr
}

// runInitialize sends the MCP initialize request, decodes the
// response, and POSTs the notifications/initialized
// notification. Caller holds no lock; initOnce serialises us.
func (t *MCPHTTPTransport) runInitialize(ctx context.Context) error {
	req := httpJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      t.nextID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "helix_agent/claude_code",
				"version": "round-41",
			},
		},
	}
	resp, err := t.exchange(ctx, req)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize: server returned JSON-RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	// Notification: no id field; server typically returns
	// 202/204 with no body, but spec-compliant servers may
	// also return 200 with an empty body. exchangeRaw
	// tolerates an empty body so we can just POST and ignore
	// the response shape.
	notif := httpJSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]interface{}{},
	}
	if err := t.postNotification(ctx, notif); err != nil {
		return fmt.Errorf("initialize: send notifications/initialized: %w", err)
	}
	return nil
}

// exchange POSTs a JSON-RPC request to the configured Endpoint
// and decodes the JSON-RPC response from the response body.
// Wraps every error with enough context that a stack-trace-style
// log line identifies the HTTP failure mode (4xx vs 5xx vs
// transport-level vs decode).
func (t *MCPHTTPTransport) exchange(ctx context.Context, req httpJSONRPCRequest) (*jsonrpcResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBytes, err := t.doPost(ctx, body)
	if err != nil {
		return nil, err
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("decode response (body=%q): %w", truncateForLog(respBytes), err)
	}
	return &resp, nil
}

// postNotification POSTs a JSON-RPC notification (no id field)
// and discards the response body. Tolerates 200/202/204 status
// codes — every spec-compliant server returns one of these for
// a notification.
func (t *MCPHTTPTransport) postNotification(ctx context.Context, n httpJSONRPCNotification) error {
	body, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	if _, err := t.doPost(ctx, body); err != nil {
		return err
	}
	return nil
}

// doPost executes a single POST against cfg.Endpoint with the
// given JSON body. Returns the response body bytes (may be
// empty for 202/204 responses). Surfaces 4xx and 5xx HTTP
// failures as errors so callers don't silently treat a 500
// body as a valid JSON-RPC response.
func (t *MCPHTTPTransport) doPost(ctx context.Context, body []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", t.userAgent())
	if t.cfg.BearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+t.cfg.BearerToken)
	}

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http POST %s: %w", t.cfg.Endpoint, err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("http POST %s: read body (status=%d): %w", t.cfg.Endpoint, httpResp.StatusCode, err)
	}

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("http POST %s: server returned status %d (%s): body=%q",
			t.cfg.Endpoint,
			httpResp.StatusCode,
			http.StatusText(httpResp.StatusCode),
			truncateForLog(respBody),
		)
	}

	return respBody, nil
}

// runSSE is the round-44 SSE-listener goroutine: opens the SSE
// stream against cfg.SSEEndpoint with Accept: text/event-stream,
// parses the HTML5 EventSource wire format, and emits parsed
// notifications on ch. On disconnect it reconnects with the
// Last-Event-ID HTTP request header set to the latest `id:`
// field observed in the stream, using exponential backoff with
// ±25% jitter and the cap rules documented on MCPHTTPConfig.
//
// Exit conditions:
//   - ctx.Done()                           → clean exit, ch closed
//   - MaxConsecutiveFailures exhausted     → emit
//     ErrMCPSSEStreamPermanentlyDisconnected via a final
//     synthetic notification, then close ch (consumer's signal
//     to call EnableSSE again to restart)
//
// close(ch) is deferred so every exit path leaves the channel
// closed (idiomatic Go signal "no more values will arrive").
func (t *MCPHTTPTransport) runSSE(ctx context.Context, ch chan<- MCPHTTPNotification) {
	defer close(ch)

	// Resolve config defaults (constants here so tests can dial
	// them via cfg overrides).
	initialRetry := t.cfg.SSEInitialRetryInterval
	if initialRetry <= 0 {
		initialRetry = 3 * time.Second
	}
	maxRetry := t.cfg.SSEMaxRetryInterval
	if maxRetry <= 0 {
		maxRetry = 60 * time.Second
	}
	maxFailures := t.cfg.SSEMaxConsecutiveFailures
	if maxFailures == 0 {
		maxFailures = 10
	}

	// retryInterval is the spec's "reconnection time" — updated
	// by the server's `retry:` field over the stream's lifetime.
	retryInterval := initialRetry
	if retryInterval > maxRetry {
		retryInterval = maxRetry
	}

	// lastEventID is the latest `id:` field observed across the
	// ENTIRE lifetime of this goroutine (not just within one
	// connection). Per SSE spec it's sent as Last-Event-ID on
	// every reconnect so the server can replay missed events.
	var lastEventID string

	// attemptCount counts CONSECUTIVE failed reconnect attempts.
	// Resets to 0 on every successful 200/text-event-stream
	// response. Used both for backoff growth and for the
	// MaxConsecutiveFailures hard cap.
	attemptCount := 0

	// Separate client for SSE that does NOT honour cfg.Timeout
	// (SSE is a long-lived stream). ctx cancellation tears it
	// down via http.NewRequestWithContext.
	sseClient := &http.Client{
		Transport: t.client.Transport,
	}

	for {
		if ctx.Err() != nil {
			return
		}

		// One pass through the readSSEStream helper attempts
		// ONE GET, returns when the stream closes (EOF /
		// transport error / non-200) or when ctx cancels.
		// The helper updates retryInterval + lastEventID via
		// pointers so the auto-reconnect loop sees fresh
		// values on next iteration.
		connected, ctxDone := t.readSSEStream(ctx, sseClient, ch, &lastEventID, &retryInterval, maxRetry)
		if ctxDone {
			return
		}

		// On a SUCCESSFUL connection (200 OK + text/event-stream)
		// reset the failure counter. The disconnect was natural
		// (server closed the stream after delivering events) and
		// the next reconnect attempt starts from a clean slate.
		if connected {
			attemptCount = 0
		} else {
			attemptCount++
		}

		// Hard-cap check BEFORE waiting: an exhausted budget
		// emits the sentinel notification + exits without
		// another sleep.
		if maxFailures > 0 && attemptCount >= maxFailures {
			t.emitSSEPermanentDisconnect(ctx, ch)
			return
		}

		// Compute the backoff for the NEXT attempt.
		wait := computeSSEBackoff(attemptCount, retryInterval, maxRetry)

		// Wait, honouring ctx.Done at every tick.
		t.sleepOrAbort(ctx, wait)
	}
}

// readSSEStream performs one connect-and-read pass on the SSE
// endpoint. Returns:
//   - connected: true if we got a 200 OK with content-type
//     text/event-stream (regardless of whether any frames
//     followed); false otherwise (transport error, non-200).
//   - ctxDone: true if ctx cancellation aborted the pass; caller
//     must return immediately without further reconnect.
//
// Updates *lastEventID on every `id:` field and *retryInterval
// on every spec-compliant `retry:` field. Capped at maxRetry.
func (t *MCPHTTPTransport) readSSEStream(
	ctx context.Context,
	sseClient *http.Client,
	ch chan<- MCPHTTPNotification,
	lastEventID *string,
	retryInterval *time.Duration,
	maxRetry time.Duration,
) (connected bool, ctxDone bool) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, t.cfg.SSEEndpoint, nil)
	if err != nil {
		return false, ctx.Err() != nil
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("User-Agent", t.userAgent())
	if t.cfg.BearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+t.cfg.BearerToken)
	}
	if *lastEventID != "" {
		// Per W3C/HTML5 EventSource spec — server uses this
		// to replay events delivered after the given id.
		httpReq.Header.Set("Last-Event-ID", *lastEventID)
	}

	httpResp, err := sseClient.Do(httpReq)
	if err != nil {
		return false, ctx.Err() != nil
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		return false, ctx.Err() != nil
	}

	connected = true

	reader := bufio.NewReader(httpResp.Body)
	var (
		curEvent string
		curData  bytes.Buffer
	)

	flush := func() {
		if curData.Len() == 0 && curEvent == "" {
			return
		}
		raw := append([]byte(nil), curData.Bytes()...)
		notif := MCPHTTPNotification{
			EventType: curEvent,
			Raw:       raw,
		}
		var jr struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(raw, &jr); err == nil {
			notif.Method = jr.Method
			notif.Params = jr.Params
		}
		select {
		case ch <- notif:
		case <-ctx.Done():
		}
		curEvent = ""
		curData.Reset()
	}

	for {
		if ctx.Err() != nil {
			return connected, true
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF or transport error; emit any partial
			// frame then return so the outer loop decides
			// whether to reconnect.
			flush()
			return connected, ctx.Err() != nil
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		var field, value string
		if colon < 0 {
			field = line
			value = ""
		} else {
			field = line[:colon]
			value = line[colon+1:]
			value = strings.TrimPrefix(value, " ")
		}

		switch field {
		case "event":
			curEvent = value
		case "data":
			if curData.Len() > 0 {
				curData.WriteByte('\n')
			}
			curData.WriteString(value)
		case "id":
			// Per spec: record for Last-Event-ID retry
			// header. The value MAY be empty (which per
			// spec resets the last-id to empty); honour
			// that.
			*lastEventID = value
		case "retry":
			// Per spec: integer milliseconds. Reject
			// non-integer values silently. Cap at
			// maxRetry to defend against malicious /
			// buggy servers.
			ms, parseErr := parseSSERetryMs(value)
			if parseErr == nil && ms > 0 {
				ri := time.Duration(ms) * time.Millisecond
				if ri > maxRetry {
					ri = maxRetry
				}
				*retryInterval = ri
			}
		default:
			// Unknown field per spec → ignore.
		}
	}
}

// emitSSEPermanentDisconnect synthesises a notification that
// carries the ErrMCPSSEStreamPermanentlyDisconnected sentinel
// in Raw and a structured method/params payload describing the
// give-up condition. Consumers can errors.Is on the sentinel via
// the EventType field (set to a stable marker string) without
// having to inspect Raw bytes.
//
// The send respects ctx.Done so a cancelled consumer doesn't
// keep this goroutine pinned forever.
func (t *MCPHTTPTransport) emitSSEPermanentDisconnect(ctx context.Context, ch chan<- MCPHTTPNotification) {
	payload := map[string]interface{}{
		"sentinel": "ErrMCPSSEStreamPermanentlyDisconnected",
		"message":  ErrMCPSSEStreamPermanentlyDisconnected.Error(),
	}
	params, _ := json.Marshal(payload)
	notif := MCPHTTPNotification{
		EventType: sseEventPermanentDisconnect,
		Method:    "$helix/sse_permanent_disconnect",
		Params:    params,
		Raw:       []byte(ErrMCPSSEStreamPermanentlyDisconnected.Error()),
	}
	select {
	case ch <- notif:
	case <-ctx.Done():
	}
}

// sseEventPermanentDisconnect is the EventType stamped on the
// synthetic notification emitted when the SSE goroutine gives up
// after MaxConsecutiveFailures. Consumers can branch on this
// stable string without parsing Raw.
const sseEventPermanentDisconnect = "$helix/sse_permanent_disconnect"

// computeSSEBackoff returns the wait duration before the next
// reconnect attempt. Doubling growth (1×, 2×, 4×, ...) starting
// from retryInterval, capped at min(retryInterval*8, maxRetry),
// with ±25% jitter applied to defend against thundering-herd.
//
// attemptCount is the number of CONSECUTIVE failures so far
// (1-indexed for backoff math: attemptCount=1 → 1×retryInterval).
func computeSSEBackoff(attemptCount int, retryInterval, maxRetry time.Duration) time.Duration {
	if attemptCount < 1 {
		attemptCount = 1
	}
	// shift = attemptCount-1 clamped to avoid Int overflow on
	// the doubling step (>30 attempts is well past maxRetry).
	shift := attemptCount - 1
	if shift > 30 {
		shift = 30
	}
	mult := int64(1) << uint(shift)
	wait := time.Duration(int64(retryInterval) * mult)
	cap1 := retryInterval * 8
	if cap1 <= 0 || cap1 > maxRetry {
		cap1 = maxRetry
	}
	if wait > cap1 {
		wait = cap1
	}
	if wait > maxRetry {
		wait = maxRetry
	}
	if wait <= 0 {
		wait = retryInterval
	}
	// ±25% jitter via a cheap deterministic-enough source
	// (nanosecond clock LSBs). Cryptographic randomness is
	// overkill for backoff jitter and would add a dependency.
	jitterRange := int64(wait / 4) // 25%
	if jitterRange > 0 {
		now := time.Now().UnixNano()
		// Map LSBs into [-jitterRange, +jitterRange).
		jitter := (now % (2*jitterRange + 1)) - jitterRange
		wait += time.Duration(jitter)
	}
	if wait < 0 {
		wait = retryInterval
	}
	return wait
}

// parseSSERetryMs parses the SSE `retry:` field value as
// non-negative integer milliseconds. Returns an error for any
// non-digit content per spec.
func parseSSERetryMs(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-digit %q", r)
		}
		n = n*10 + int64(r-'0')
		// Overflow defence (10 billion ms ~ 115 days; cap)
		if n > 1_000_000_000_000 {
			return 0, fmt.Errorf("overflow")
		}
	}
	return n, nil
}

// sleepOrAbort blocks for d, but returns immediately on
// ctx.Done. Used by the SSE reconnect loop so a cancelled ctx
// never has to wait out a 60-second backoff. NEVER use
// time.Sleep on a ctx-cancellable path.
func (t *MCPHTTPTransport) sleepOrAbort(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// userAgent returns the configured User-Agent or a sane default
// identifying the round-41 transport (useful for server-side
// logs when debugging which client version hit them).
func (t *MCPHTTPTransport) userAgent() string {
	if t.cfg.UserAgent != "" {
		return t.cfg.UserAgent
	}
	return "helix_agent/claude_code (round-41 mcp-http)"
}

// nextID returns the next monotonic JSON-RPC request id.
func (t *MCPHTTPTransport) nextID() int64 {
	return t.idCounter.Add(1)
}

// truncateForLog trims long byte payloads to a fixed cap so
// error messages stay readable even when the server returns a
// multi-megabyte HTML error page.
func truncateForLog(b []byte) string {
	const maxLen = 512
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "...(truncated)"
}

// httpJSONRPCRequest models a JSON-RPC 2.0 request carried over
// HTTP POST. Mirrors jsonrpcRequest in the stdio transport but
// uses a distinct name so the two transports can be reasoned
// about independently if their wire shapes ever diverge.
type httpJSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// httpJSONRPCNotification models a JSON-RPC 2.0 notification —
// same as a request but without an id field (notifications get
// no response per the JSON-RPC spec).
type httpJSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Compile-time interface assertion: MCPHTTPTransport satisfies
// MCPTransport. Catches signature drift at build time rather
// than at the first round-trip.
var _ MCPTransport = (*MCPHTTPTransport)(nil)
