// Round-41 §11.4 anti-bluff tests for the MCPHTTPTransport.
//
// Test fixture strategy
// ---------------------
// Unlike the round-39 stdio tests (which re-exec the test
// binary as a subprocess via os.Executable), the HTTP transport
// tests use the stdlib's net/http/httptest package to spin up a
// REAL in-process HTTP server bound to a loopback TCP port.
// Every test below dials the server over real TCP sockets,
// exchanges real JSON-RPC frames, and asserts against real
// response bytes — there is no in-process short-circuit and no
// fake http.Client. This satisfies Article XI §11.9's
// positive-runtime-evidence requirement while staying within
// CONST-050(A) (fixtures live in *_test.go only and never leak
// into production).
//
// Anti-bluff anchors covered by this file:
//   * CONST-035 / Article XI §11.9 — positive runtime evidence
//     captured via real httptest.Server emitting real JSON-RPC
//     over real TCP sockets.
//   * CONST-050(A) — fixture lives in *_test.go only.
//   * CONST-050(B) — successful tool-call + endpoint-not-configured
//     + insecure-endpoint-rejected + HTTP 4xx + HTTP 5xx +
//     JSON-RPC error + sentinel-distinctness + SSE round-trip
//     paths all exercised.
//   * Round-29 sentinel preservation — paired-mutation test
//     asserts ErrMCPClientNotWired is distinct from
//     ErrMCPEndpointNotConfigured AND from
//     ErrMCPCommandNotConfigured.

package claude_code

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHTTPStubServer spins up an httptest.Server that implements
// just enough of MCP-over-HTTP to validate the round-41 wiring:
//
//   - `initialize`        — returns serverInfo + capabilities
//   - `notifications/initialized` — accepts (returns 202, no body)
//   - `tools/list`        — returns a single "echo" tool
//   - `tools/call`        — for tool="echo", echoes args.message;
//     for any other tool, returns a JSON-RPC
//     "unknown tool" error.
//
// The returned *atomic.Int32 increments on every request so
// tests can assert the transport actually hit the server (not
// just returned a cached / fabricated response).
func newHTTPStubServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var reqCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      *int64          `json:"id,omitempty"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      nil,
				"error": map[string]interface{}{
					"code":    -32700,
					"message": fmt.Sprintf("parse error: %v", err),
				},
			})
			return
		}

		// Notification (no id) — accept with 202.
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch req.Method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"result": map[string]interface{}{
					"protocolVersion": mcpProtocolVersion,
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{"listChanged": false},
					},
					"serverInfo": map[string]interface{}{
						"name":    "helix-mcp-http-stub",
						"version": "round-41-test",
					},
				},
			})
		case "tools/list":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "echo",
							"description": "echoes args.message back as text content",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"message": map[string]string{"type": "string"},
								},
								"required": []string{"message"},
							},
						},
					},
				},
			})
		case "tools/call":
			var p struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      *req.ID,
					"error": map[string]interface{}{
						"code":    -32602,
						"message": fmt.Sprintf("invalid params: %v", err),
					},
				})
				return
			}
			if p.Name != "echo" {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      *req.ID,
					"error": map[string]interface{}{
						"code":    -32601,
						"message": fmt.Sprintf("unknown tool: %s", p.Name),
					},
				})
				return
			}
			msg, _ := p.Arguments["message"].(string)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": "echo-http:" + msg},
					},
					"isError": false,
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": fmt.Sprintf("method not found: %s", req.Method),
				},
			})
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &reqCount
}

// TestMCPHTTPTransport_NewMCPHTTPTransport_EmptyEndpointReturnsSentinel
// is the paired-mutation test for the round-41
// ErrMCPEndpointNotConfigured sentinel: any future change that
// silently defaults the empty-Endpoint case to localhost or any
// other URL will fail loudly here.
func TestMCPHTTPTransport_NewMCPHTTPTransport_EmptyEndpointReturnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := NewMCPHTTPTransport(MCPHTTPConfig{})
	require.Error(t, err, "empty Endpoint must surface ErrMCPEndpointNotConfigured")
	require.ErrorIs(t, err, ErrMCPEndpointNotConfigured, "the error must wrap ErrMCPEndpointNotConfigured")
}

// TestMCPHTTPTransport_NewMCPHTTPTransport_WhitespaceEndpointReturnsSentinel
// asserts that an Endpoint consisting solely of whitespace is
// treated as empty (operator typed " " instead of pasting a
// URL). Silent acceptance would lead to a confusing "unsupported
// scheme" error downstream.
func TestMCPHTTPTransport_NewMCPHTTPTransport_WhitespaceEndpointReturnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := NewMCPHTTPTransport(MCPHTTPConfig{Endpoint: "   \t"})
	require.ErrorIs(t, err, ErrMCPEndpointNotConfigured)
}

// TestMCPHTTPTransport_SentinelDistinctness_AllFive asserts that
// every MCP sentinel is distinguishable via errors.Is from every
// other. Round-29 introduced ErrMCPClientNotWired; round-39
// added ErrMCPCommandNotConfigured; round-41 added
// ErrMCPEndpointNotConfigured + ErrMCPInsecureEndpointRejected;
// round-44 (this test's extension) adds
// ErrMCPSSEStreamPermanentlyDisconnected. The five sentinels
// mean five different things; conflating ANY pair would
// re-introduce a §11.4 disguised PASS-bluff (observability layer).
func TestMCPHTTPTransport_SentinelDistinctness_AllFive(t *testing.T) {
	t.Parallel()

	all := []struct {
		name string
		err  error
	}{
		{"ErrMCPClientNotWired", ErrMCPClientNotWired},
		{"ErrMCPCommandNotConfigured", ErrMCPCommandNotConfigured},
		{"ErrMCPEndpointNotConfigured", ErrMCPEndpointNotConfigured},
		{"ErrMCPInsecureEndpointRejected", ErrMCPInsecureEndpointRejected},
		{"ErrMCPSSEStreamPermanentlyDisconnected", ErrMCPSSEStreamPermanentlyDisconnected},
	}

	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			require.Falsef(t, errors.Is(a.err, b.err),
				"%s MUST NOT wrap or equal %s — sentinels must be paired-mutation-distinguishable", a.name, b.name)
		}
	}

	// Constructor-path symmetric assertions: empty-config
	// constructor errors must wrap ONLY their own sentinel.
	_, errHTTP := NewMCPHTTPTransport(MCPHTTPConfig{})
	require.Error(t, errHTTP)
	require.ErrorIs(t, errHTTP, ErrMCPEndpointNotConfigured)
	require.False(t, errors.Is(errHTTP, ErrMCPClientNotWired))
	require.False(t, errors.Is(errHTTP, ErrMCPCommandNotConfigured))
	require.False(t, errors.Is(errHTTP, ErrMCPSSEStreamPermanentlyDisconnected))

	_, errStdio := NewMCPStdioTransport(MCPStdioConfig{})
	require.Error(t, errStdio)
	require.ErrorIs(t, errStdio, ErrMCPCommandNotConfigured)
	require.False(t, errors.Is(errStdio, ErrMCPEndpointNotConfigured))
	require.False(t, errors.Is(errStdio, ErrMCPSSEStreamPermanentlyDisconnected))
}

// TestMCPHTTPTransport_NewMCPHTTPTransport_RejectsPlaintextByDefault
// proves the §11.4 security-layer guardrail: a plain http://
// endpoint is refused by default. AllowInsecure: true opts in
// (and the next test confirms it actually works).
func TestMCPHTTPTransport_NewMCPHTTPTransport_RejectsPlaintextByDefault(t *testing.T) {
	t.Parallel()

	_, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint: "http://example.com/mcp",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMCPInsecureEndpointRejected,
		"plain http:// endpoints MUST be refused unless AllowInsecure=true")
}

// TestMCPHTTPTransport_NewMCPHTTPTransport_RejectsUnsupportedScheme
// asserts that schemes other than http/https are rejected at
// construction.
func TestMCPHTTPTransport_NewMCPHTTPTransport_RejectsUnsupportedScheme(t *testing.T) {
	t.Parallel()

	_, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      "ftp://example.com/mcp",
		AllowInsecure: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported scheme",
		"non-http/https schemes must fail loudly")
}

// TestMCPHTTPTransport_CallTool_RealHTTPServer_RoundTrip is the
// core anti-bluff proof: spins up an httptest.Server, dials it
// over real TCP, runs the real MCP initialize handshake, sends
// a real tools/call, and asserts the decoded response matches
// what the server actually emitted.
func TestMCPHTTPTransport_CallTool_RealHTTPServer_RoundTrip(t *testing.T) {
	t.Parallel()

	srv, reqCount := newHTTPStubServer(t)

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true, // httptest.NewServer is http://127.0.0.1:<port>
		Timeout:       5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{
		Name:    "stub-http",
		Enabled: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := tr.CallTool(ctx, server, "echo", map[string]interface{}{
		"message": "round-41",
	})
	require.NoError(t, err, "real-HTTP-server round-trip must succeed")
	require.NotNil(t, got)
	require.False(t, got.IsError, "stub returns IsError=false for the happy path; non-false means a fabricated 'simulated' bluff has crept back in")
	require.Len(t, got.Content, 1, "stub returns exactly one Content item; deviation means we decoded the wrong shape")
	assert.Equal(t, "text", got.Content[0].Type)
	assert.Equal(t, "echo-http:round-41", got.Content[0].Text,
		"the round-trip text must match what the stub server *actually sent*. If this fails, either the JSON-RPC framing is wrong, the id correlation is wrong, or some intermediate code is fabricating content.")

	// Initialize (1 POST) + notifications/initialized (1 POST)
	// + tools/call (1 POST) = 3 requests minimum. Anything
	// lower means the transport short-circuited somewhere.
	assert.GreaterOrEqual(t, int(reqCount.Load()), 3,
		"transport must have hit the server at least 3 times (initialize + initialized + tools/call); fewer means it bluffed a response")
}

// TestMCPHTTPTransport_CallTool_InitializeOnce_AcrossManyCalls
// proves that the round-41 ensureInitialized handshake fires
// exactly once across many CallTool invocations. The stub
// server-side counter lets us count the actual initialize hits.
func TestMCPHTTPTransport_CallTool_InitializeOnce_AcrossManyCalls(t *testing.T) {
	t.Parallel()

	var initCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      *int64          `json:"id,omitempty"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			// notification
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			initCount.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"result": map[string]interface{}{
					"protocolVersion": mcpProtocolVersion,
				},
			})
		case "tools/call":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{{"type": "text", "text": "ok"}},
					"isError": false,
				},
			})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true,
		Timeout:       5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{Name: "init-once", Enabled: true}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < 5; i++ {
		_, err := tr.CallTool(ctx, server, "echo", map[string]interface{}{"i": i})
		require.NoError(t, err, "call %d", i)
	}

	assert.Equal(t, int32(1), initCount.Load(),
		"initialize MUST fire exactly once across 5 CallTool invocations; >1 means the once-guarantee broke")
}

// TestMCPHTTPTransport_CallTool_UnknownTool_SurfacesJSONRPCError
// exercises the JSON-RPC error path: the stub returns a method-
// not-found-style error for any tool name other than "echo".
// The transport MUST surface this as a real ToolCallResult
// {IsError:true, ...} (NOT swallow as nil error and NOT
// fabricate text content).
func TestMCPHTTPTransport_CallTool_UnknownTool_SurfacesJSONRPCError(t *testing.T) {
	t.Parallel()

	srv, _ := newHTTPStubServer(t)

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true,
		Timeout:       5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{Name: "stub-unknown-http", Enabled: true}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := tr.CallTool(ctx, server, "this_tool_does_not_exist", map[string]interface{}{})
	require.NoError(t, err, "JSON-RPC error responses are real responses, not transport failures")
	require.NotNil(t, got)
	require.True(t, got.IsError, "unknown tool must surface as IsError=true")
	require.Len(t, got.Content, 1)
	assert.Contains(t, got.Content[0].Text, "unknown tool",
		"the surfaced error must carry the server's message verbatim (proves we read a real server response, not a fabricated placeholder)")
}

// TestMCPHTTPTransport_CallTool_HTTP4xx_SurfacesError asserts
// that a 4xx response from the server (e.g. 401 Unauthorized,
// 404 Not Found) is surfaced as a non-nil error from CallTool —
// NOT silently swallowed and NOT treated as a successful empty
// response.
func TestMCPHTTPTransport_CallTool_HTTP4xx_SurfacesError(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true,
		Timeout:       5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{Name: "unauthorized", Enabled: true}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = tr.CallTool(ctx, server, "echo", nil)
	require.Error(t, err, "HTTP 4xx must surface as a non-nil error — silent success would be a §11.4 PASS-bluff")
	assert.Contains(t, err.Error(), "401",
		"the error message must carry the actual HTTP status so operators can diagnose")
}

// TestMCPHTTPTransport_CallTool_HTTP5xx_SurfacesError asserts
// that a 5xx response (server error) is also surfaced loudly.
func TestMCPHTTPTransport_CallTool_HTTP5xx_SurfacesError(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("oops"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true,
		Timeout:       5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{Name: "broken", Enabled: true}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = tr.CallTool(ctx, server, "echo", nil)
	require.Error(t, err, "HTTP 5xx must surface as a non-nil error")
	assert.Contains(t, err.Error(), "500",
		"the error message must carry the actual HTTP status")
}

// TestMCPHTTPTransport_CallTool_NetworkFailure_SurfacesError
// asserts that a connect-refused / unreachable endpoint
// surfaces a non-nil error (and does NOT block forever or
// fabricate success).
func TestMCPHTTPTransport_CallTool_NetworkFailure_SurfacesError(t *testing.T) {
	t.Parallel()

	// Pick a TCP port that is almost certainly closed. We
	// don't bind to it — the connect should refuse.
	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      "http://127.0.0.1:1/mcp",
		AllowInsecure: true,
		Timeout:       2 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{Name: "unreachable", Enabled: true}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = tr.CallTool(ctx, server, "echo", nil)
	require.Error(t, err, "unreachable endpoint must surface as a non-nil error")
}

// TestMCPHTTPTransport_BearerToken_IsSent asserts the bearer
// token from config actually lands as Authorization: Bearer
// <token> on the wire. This is the only way to prove the
// credential plumbing is real — without server-side observation
// the token could be silently dropped.
func TestMCPHTTPTransport_BearerToken_IsSent(t *testing.T) {
	t.Parallel()

	var capturedAuth atomic.Value // string

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		capturedAuth.Store(r.Header.Get("Authorization"))
		var req struct {
			ID     *int64 `json:"id,omitempty"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0", "id": *req.ID,
				"result": map[string]interface{}{"protocolVersion": mcpProtocolVersion},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0", "id": *req.ID,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{{"type": "text", "text": "ok"}},
				},
			})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	const token = "round-41-bearer-token-test"
	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true,
		BearerToken:   token,
		Timeout:       5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = tr.CallTool(ctx, &MCPServer{Name: "auth-test", Enabled: true}, "echo", nil)
	require.NoError(t, err)

	got, _ := capturedAuth.Load().(string)
	assert.Equal(t, "Bearer "+token, got,
		"the bearer token MUST land on the wire as Authorization: Bearer <token>; otherwise the credential plumbing is bluffing")
}

// TestMCPHTTPTransport_Integration_WithMCPIntegration_EndToEnd
// is the round-41-specific end-to-end proof: wire the HTTP
// transport into MCPIntegration via SetTransport, then call
// (*MCPIntegration).CallTool and assert it dispatches all the
// way through to the real HTTP server. Proves the round-29
// injection point accepts the round-41 real implementation
// alongside the round-39 stdio implementation.
func TestMCPHTTPTransport_Integration_WithMCPIntegration_EndToEnd(t *testing.T) {
	t.Parallel()

	srv, _ := newHTTPStubServer(t)

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true,
		Timeout:       5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp.json")
	mcp := NewMCPIntegration(configPath)
	require.NoError(t, mcp.LoadConfig())

	require.NoError(t, mcp.AddServer(&MCPServer{
		Name:    "http-echo-server",
		Enabled: true,
	}))

	mcp.SetTransport(tr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := mcp.CallTool(ctx, "http-echo-server", "echo", map[string]interface{}{
		"message": "end-to-end-http",
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, got.IsError)
	require.Len(t, got.Content, 1)
	assert.Equal(t, "echo-http:end-to-end-http", got.Content[0].Text,
		"end-to-end pipeline must deliver the real HTTP server's response verbatim — any deviation means a layer is bluffing")
}

// TestMCPHTTPTransport_EnableSSE_EmptyEndpoint_ReturnsError
// asserts that opting into SSE without configuring the
// SSEEndpoint fails loudly. Silent no-op would mask a
// configuration bug.
func TestMCPHTTPTransport_EnableSSE_EmptyEndpoint_ReturnsError(t *testing.T) {
	t.Parallel()

	srv, _ := newHTTPStubServer(t)

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true,
		// SSEEndpoint intentionally empty.
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ch, err := tr.EnableSSE(context.Background())
	require.Error(t, err)
	require.Nil(t, ch, "EnableSSE must NOT return a usable channel when SSEEndpoint is empty")
	require.Contains(t, err.Error(), "SSEEndpoint",
		"the error must name the missing config field")
}

// TestMCPHTTPTransport_SSE_RealServerPush_ReceivedByClient is
// the SSE end-to-end anti-bluff proof: spin up an httptest
// server that flushes a real SSE event stream, EnableSSE, and
// assert the client receives the pushed notification verbatim.
func TestMCPHTTPTransport_SSE_RealServerPush_ReceivedByClient(t *testing.T) {
	t.Parallel()

	// shutdown unblocks the handler at cleanup-time so
	// srv.Close() doesn't have to rely on the client's
	// connection-close signal landing first.
	shutdown := make(chan struct{})

	// SSE handler: write one notification, flush, then hold
	// the connection open until the ctx is cancelled. The
	// chunked-encoding flush is what makes this a real SSE
	// stream (not just a buffered response).
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "httptest.ResponseRecorder must implement http.Flusher for SSE")

		// Emit one real SSE frame: event=notifications/message,
		// data={...JSON-RPC notification payload...}.
		payload := `{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info","data":"hello-from-sse-round-41"}}`
		_, _ = fmt.Fprintf(w, "event: notifications/message\ndata: %s\n\n", payload)
		flusher.Flush()

		// Block until the client closes / ctx cancels OR
		// the test cleanup signals shutdown.
		select {
		case <-r.Context().Done():
		case <-shutdown:
		}
	})
	srv := httptest.NewServer(mux)
	// LIFO cleanup order: srv.Close runs AFTER close(shutdown).
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(shutdown) })

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp", // not used by EnableSSE
		SSEEndpoint:   srv.URL + "/sse",
		AllowInsecure: true,
		Timeout:       5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := tr.EnableSSE(ctx)
	require.NoError(t, err)
	require.NotNil(t, ch)

	select {
	case n, ok := <-ch:
		require.True(t, ok, "SSE channel closed before delivering the pushed frame")
		assert.Equal(t, "notifications/message", n.EventType,
			"the event-type field from the `event:` line must round-trip verbatim")
		assert.Equal(t, "notifications/message", n.Method,
			"the JSON-RPC method must be extracted from the data payload")
		assert.True(t, strings.Contains(string(n.Raw), "hello-from-sse-round-41"),
			"raw payload must carry the verbatim bytes the server sent (proves no fabrication)")
	case <-ctx.Done():
		t.Fatalf("did not receive SSE notification within %s — the SSE plumbing is broken", "5s")
	}
}

// TestMCPHTTPTransport_EnableSSE_Idempotent asserts that calling
// EnableSSE twice returns the same channel and does NOT spawn a
// second SSE goroutine.
func TestMCPHTTPTransport_EnableSSE_Idempotent(t *testing.T) {
	t.Parallel()

	shutdown := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-r.Context().Done():
		case <-shutdown:
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(shutdown) })

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		SSEEndpoint:   srv.URL + "/sse",
		AllowInsecure: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch1, err := tr.EnableSSE(ctx)
	require.NoError(t, err)

	ch2, err := tr.EnableSSE(ctx)
	require.NoError(t, err)

	// Reading from one MUST drain from both — they ARE the
	// same channel.
	assert.Equal(t, fmt.Sprintf("%p", ch1), fmt.Sprintf("%p", ch2),
		"EnableSSE must be idempotent: same channel pointer on every call")
}

// TestMCPHTTPTransport_CallTool_ContextCancellation_Honored
// proves that CallTool honours ctx cancellation when the
// server hangs without responding. Defence against the
// "deadlock disguised as success" failure mode.
func TestMCPHTTPTransport_CallTool_ContextCancellation_Honored(t *testing.T) {
	t.Parallel()

	// shutdown lets the test-cleanup unblock the handler
	// even if the client's connection-close signal got lost,
	// so srv.Close() doesn't hang waiting for an active
	// handler that the test no longer cares about.
	shutdown := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-shutdown:
		}
	})
	srv := httptest.NewServer(mux)
	// Order matters: close(shutdown) MUST run BEFORE srv.Close()
	// so the handler exits and lets srv.Close()'s connection
	// drain complete promptly. t.Cleanup runs LIFO, so register
	// srv.Close() FIRST and close(shutdown) SECOND.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(shutdown) })

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      srv.URL + "/mcp",
		AllowInsecure: true,
		Timeout:       30 * time.Second, // long timeout — ctx must win
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{Name: "hung-http", Enabled: true}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = tr.CallTool(ctx, server, "anything", nil)
	elapsed := time.Since(start)

	require.Error(t, err, "ctx-cancellation must surface as a real error")
	require.Less(t, elapsed, 5*time.Second,
		"ctx-cancellation must abort the call within seconds (took %s); longer means the http.Request is not wired to the ctx", elapsed)
}

// =============================================================
// ROUND-44 §11.4 anti-bluff tests
// -------------------------------------------------------------
// These tests prove the SSE auto-reconnect + Last-Event-ID retry
// wiring documented in mcp_http_transport.go (round-44 addendum).
// Every assertion uses a REAL httptest.Server connected over a
// REAL TCP socket; there is no fake clock and no in-process
// short-circuit. To keep test runtime modest, the tests pass
// short SSEInitialRetryInterval values (e.g. 20-50ms) via the
// new config knobs.
// =============================================================

// assertNoGoroutineLeak captures the current goroutine count
// and registers a t.Cleanup that asserts the count has not
// grown beyond a small delta after the test exits. Used by the
// round-44 SSE reconnect tests to prove ctx-cancellation tears
// down the listener goroutine (no leaks). Generous tolerance
// (+2) accommodates runtime-internal goroutines that wax and
// wane between samples.
func assertNoGoroutineLeak(t *testing.T) {
	t.Helper()
	// Force a GC + small sleep to let any in-flight runtime
	// goroutines park before sampling. Without this the
	// baseline can include short-lived workers that have
	// already exited by the assertion, producing flaky -1 or
	// +1 deltas.
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	t.Cleanup(func() {
		// Drain time for the runSSE goroutine to honour
		// ctx.Done and exit. Loop briefly rather than a
		// single Sleep so a fast machine isn't punished.
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			runtime.GC()
			if runtime.NumGoroutine() <= baseline+1 {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		got := runtime.NumGoroutine()
		// Allow +2 for runtime-internal flakiness; >+2
		// means a real leak (typically the SSE goroutine
		// pinned by a channel-send).
		if got > baseline+2 {
			t.Errorf("goroutine leak: baseline=%d, after-test=%d (delta=%d) — likely the SSE reconnect loop ignored ctx.Done",
				baseline, got, got-baseline)
		}
	})
}

// sseReconnectFixture wires an httptest server whose /sse
// handler is driven by a behaviour func that the test supplies.
// connNum (1-indexed) is the connection-attempt number — the
// behaviour func uses it to deliver different responses per
// reconnect (e.g. fail twice, succeed on attempt 3).
//
// The fixture captures every Last-Event-ID HTTP header received
// in lastEventIDs (one entry per connection attempt; "" when
// no header was sent) for later assertion.
type sseReconnectFixture struct {
	srv            *httptest.Server
	connCount      *atomic.Int32
	lastEventIDs   *[]string
	lastEventIDsMu *atomic.Bool // simple "writer lock" via CAS
}

func newSSEReconnectFixture(
	t *testing.T,
	handler func(connNum int, w http.ResponseWriter, r *http.Request),
) *sseReconnectFixture {
	t.Helper()
	var connCount atomic.Int32
	ids := make([]string, 0, 16)
	var lock atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		n := int(connCount.Add(1))

		// Capture Last-Event-ID under a CAS-protected
		// section so concurrent reconnect attempts don't
		// race on the slice append.
		for !lock.CompareAndSwap(false, true) {
			runtime.Gosched()
		}
		ids = append(ids, r.Header.Get("Last-Event-ID"))
		lock.Store(false)

		handler(n, w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &sseReconnectFixture{
		srv:            srv,
		connCount:      &connCount,
		lastEventIDs:   &ids,
		lastEventIDsMu: &lock,
	}
}

func (f *sseReconnectFixture) snapshotLastEventIDs() []string {
	for !f.lastEventIDsMu.CompareAndSwap(false, true) {
		runtime.Gosched()
	}
	defer f.lastEventIDsMu.Store(false)
	out := make([]string, len(*f.lastEventIDs))
	copy(out, *f.lastEventIDs)
	return out
}

// TestMCPHTTPTransport_SSE_AutoReconnect_AfterServerDisconnect
// proves that after the server closes the stream the client
// reconnects within the configured retry interval and delivers
// the next event from the second connection.
func TestMCPHTTPTransport_SSE_AutoReconnect_AfterServerDisconnect(t *testing.T) {
	t.Parallel()
	assertNoGoroutineLeak(t)

	fix := newSSEReconnectFixture(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		switch n {
		case 1:
			// Connection 1: emit one event, then return
			// (server-side disconnect).
			_, _ = fmt.Fprintf(w,
				"event: notifications/message\ndata: %s\n\n",
				`{"jsonrpc":"2.0","method":"notifications/message","params":{"seq":1}}`)
			if flusher != nil {
				flusher.Flush()
			}
			// Handler returns → connection closed.
		default:
			// Connection 2+: emit a second event, then
			// hold open until ctx cancels.
			_, _ = fmt.Fprintf(w,
				"event: notifications/message\ndata: %s\n\n",
				`{"jsonrpc":"2.0","method":"notifications/message","params":{"seq":2}}`)
			if flusher != nil {
				flusher.Flush()
			}
			<-r.Context().Done()
		}
	})

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:                  fix.srv.URL + "/mcp",
		SSEEndpoint:               fix.srv.URL + "/sse",
		AllowInsecure:             true,
		SSEInitialRetryInterval:   30 * time.Millisecond,
		SSEMaxRetryInterval:       200 * time.Millisecond,
		SSEMaxConsecutiveFailures: 5,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := tr.EnableSSE(ctx)
	require.NoError(t, err)

	// Drain two events; the second one MUST come from the
	// reconnected stream (proven by connCount >= 2).
	var got []MCPHTTPNotification
	for len(got) < 2 {
		select {
		case n, ok := <-ch:
			require.True(t, ok, "channel closed before second event arrived (after %d events)", len(got))
			// Skip the synthetic permanent-disconnect
			// frame if it slipped in — it should NOT in
			// this happy-path test.
			require.NotEqual(t, sseEventPermanentDisconnect, n.EventType,
				"unexpected give-up notification — happy-path test got the permanent-disconnect sentinel")
			got = append(got, n)
		case <-ctx.Done():
			t.Fatalf("did not receive 2 events within deadline; got %d", len(got))
		}
	}

	require.GreaterOrEqual(t, int(fix.connCount.Load()), 2,
		"server must have observed >= 2 connections (the auto-reconnect)")
	assert.Contains(t, string(got[0].Raw), `"seq":1`, "first event from connection 1")
	assert.Contains(t, string(got[1].Raw), `"seq":2`, "second event must come from the reconnected stream")
}

// TestMCPHTTPTransport_SSE_SendsLastEventIDOnReconnect proves
// the W3C/HTML5 EventSource Last-Event-ID behaviour: after the
// server emits `id: 5` and disconnects, the client's reconnect
// GET MUST carry `Last-Event-ID: 5` so the server can replay
// missed events.
func TestMCPHTTPTransport_SSE_SendsLastEventIDOnReconnect(t *testing.T) {
	t.Parallel()
	assertNoGoroutineLeak(t)

	fix := newSSEReconnectFixture(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		if n == 1 {
			// Emit id:5 with a payload, then disconnect.
			_, _ = fmt.Fprint(w,
				"id: 5\nevent: notifications/message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"first\":true}}\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			return // server-side disconnect
		}
		// On reconnect, the round-44 client should have
		// sent Last-Event-ID: 5. Emit a second event so
		// the test loop can complete.
		_, _ = fmt.Fprint(w,
			"event: notifications/message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"second\":true}}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	})

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:                  fix.srv.URL + "/mcp",
		SSEEndpoint:               fix.srv.URL + "/sse",
		AllowInsecure:             true,
		SSEInitialRetryInterval:   30 * time.Millisecond,
		SSEMaxRetryInterval:       200 * time.Millisecond,
		SSEMaxConsecutiveFailures: 5,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := tr.EnableSSE(ctx)
	require.NoError(t, err)

	// Drain at least 2 events.
	deadline := time.After(2 * time.Second)
	count := 0
loop:
	for count < 2 {
		select {
		case _, ok := <-ch:
			require.True(t, ok, "channel closed prematurely")
			count++
		case <-deadline:
			break loop
		}
	}
	require.Equal(t, 2, count, "must have received both events")

	ids := fix.snapshotLastEventIDs()
	require.GreaterOrEqual(t, len(ids), 2, "server must have observed >= 2 connection attempts (got %d)", len(ids))
	assert.Empty(t, ids[0], "first connection MUST NOT carry Last-Event-ID (none observed yet)")
	assert.Equal(t, "5", ids[1],
		"reconnect MUST carry Last-Event-ID: 5 (the latest id emitted by the server on connection 1) — anything else means the spec-compliant replay header is not wired")
}

// TestMCPHTTPTransport_SSE_RespectsRetryField proves that a
// server-supplied `retry: <ms>` field replaces the default
// 3-second interval. We send retry:50 + immediate disconnect
// and assert the reconnect lands well below the default.
func TestMCPHTTPTransport_SSE_RespectsRetryField(t *testing.T) {
	t.Parallel()
	assertNoGoroutineLeak(t)

	fix := newSSEReconnectFixture(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		if n == 1 {
			// Tell client to reconnect after 50ms, then
			// disconnect immediately.
			_, _ = fmt.Fprint(w, "retry: 50\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		// On reconnect, emit one event then block.
		_, _ = fmt.Fprint(w,
			"event: notifications/message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"after_retry\":true}}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	})

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      fix.srv.URL + "/mcp",
		SSEEndpoint:   fix.srv.URL + "/sse",
		AllowInsecure: true,
		// Default initial retry would be 3s; the test
		// proves the server's `retry:` field overrides it.
		// Set initial to 2s so we'd notice a regression
		// that ignores the server field.
		SSEInitialRetryInterval:   2 * time.Second,
		SSEMaxRetryInterval:       60 * time.Second,
		SSEMaxConsecutiveFailures: 5,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := tr.EnableSSE(ctx)
	require.NoError(t, err)

	start := time.Now()
	select {
	case n, ok := <-ch:
		require.True(t, ok)
		require.Contains(t, string(n.Raw), `"after_retry":true`)
	case <-ctx.Done():
		t.Fatalf("did not receive reconnected event within 2s; server `retry: 50` was probably ignored")
	}
	elapsed := time.Since(start)
	// 50ms server hint + jitter + processing slop; must be
	// well under the 2s SSEInitialRetryInterval default.
	require.Less(t, elapsed, 1500*time.Millisecond,
		"reconnect took %s; server's `retry: 50` field was ignored (would have respected the 2s SSEInitialRetryInterval instead)", elapsed)
}

// TestMCPHTTPTransport_SSE_ExponentialBackoff proves that
// consecutive reconnect failures grow the wait time (no fixed
// 50ms-forever loop). We use a server that refuses every
// connection for the first 3 attempts then accepts the 4th,
// and check the cumulative elapsed time exceeds the sum of the
// configured backoffs (with ample tolerance for jitter).
func TestMCPHTTPTransport_SSE_ExponentialBackoff(t *testing.T) {
	t.Parallel()
	assertNoGoroutineLeak(t)

	fix := newSSEReconnectFixture(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n <= 3 {
			// First 3 attempts: 503 (not text/event-stream).
			// Round-44 treats non-200 as a failure → backoff.
			http.Error(w, "try again later", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w,
			"event: notifications/message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"finally\":true}}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	})

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:                  fix.srv.URL + "/mcp",
		SSEEndpoint:               fix.srv.URL + "/sse",
		AllowInsecure:             true,
		SSEInitialRetryInterval:   40 * time.Millisecond,
		SSEMaxRetryInterval:       2 * time.Second,
		SSEMaxConsecutiveFailures: 10,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := tr.EnableSSE(ctx)
	require.NoError(t, err)

	start := time.Now()
	select {
	case n, ok := <-ch:
		require.True(t, ok)
		require.Contains(t, string(n.Raw), `"finally":true`)
	case <-ctx.Done():
		t.Fatalf("did not receive event after backoff-then-recovery within 2s")
	}
	elapsed := time.Since(start)

	// 4 connection attempts, with backoff after each of the
	// first 3 failures: 40ms + 80ms + 160ms ≈ 280ms minimum.
	// With ±25% jitter the floor drops to ~210ms. Anything
	// under ~150ms would mean exponential growth isn't
	// happening (a fixed 40ms-forever loop would land at
	// ~120ms total).
	require.Greater(t, elapsed, 150*time.Millisecond,
		"backoff total elapsed %s is too small; growth must be exponential (40+80+160ms baseline) not fixed", elapsed)
	require.GreaterOrEqual(t, int(fix.connCount.Load()), 4,
		"server must have observed 4 attempts (3 failures + 1 success)")
}

// TestMCPHTTPTransport_SSE_GivesUpAfterMaxFailures proves the
// hard cap: after SSEMaxConsecutiveFailures consecutive failed
// reconnect attempts, the goroutine emits the permanent-
// disconnect sentinel as the final notification and closes the
// channel. Silent infinite-loop reconnect would hide the
// failure from operators — §11.4 observability bluff.
func TestMCPHTTPTransport_SSE_GivesUpAfterMaxFailures(t *testing.T) {
	t.Parallel()
	assertNoGoroutineLeak(t)

	fix := newSSEReconnectFixture(t, func(n int, w http.ResponseWriter, r *http.Request) {
		// Always refuse — proves the give-up path fires.
		http.Error(w, "nope", http.StatusInternalServerError)
	})

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:                  fix.srv.URL + "/mcp",
		SSEEndpoint:               fix.srv.URL + "/sse",
		AllowInsecure:             true,
		SSEInitialRetryInterval:   10 * time.Millisecond,
		SSEMaxRetryInterval:       40 * time.Millisecond,
		SSEMaxConsecutiveFailures: 3,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := tr.EnableSSE(ctx)
	require.NoError(t, err)

	// Drain until we see the sentinel notification OR the
	// channel closes.
	var sawSentinel bool
	for {
		select {
		case n, ok := <-ch:
			if !ok {
				// Channel closed without sentinel —
				// allowed only if we got the sentinel
				// already.
				require.True(t, sawSentinel,
					"channel closed without emitting the permanent-disconnect sentinel — operators would never learn the SSE listener died")
				goto done
			}
			if n.EventType == sseEventPermanentDisconnect {
				sawSentinel = true
				assert.Equal(t, "$helix/sse_permanent_disconnect", n.Method,
					"sentinel notification must carry the synthetic JSON-RPC method")
				assert.Contains(t, string(n.Raw), "gave up",
					"sentinel Raw must carry the human-readable error text")
				// String-level equality with the sentinel
				// proves the operator-facing message is the
				// canonical one (consumers errors.Is the
				// sentinel via type, not via text).
				assert.Equal(t, ErrMCPSSEStreamPermanentlyDisconnected.Error(), string(n.Raw),
					"Raw must match the canonical sentinel string")
				// Sanity-check the structured payload too.
				var p struct {
					Sentinel string `json:"sentinel"`
					Message  string `json:"message"`
				}
				require.NoError(t, json.Unmarshal(n.Params, &p))
				assert.Equal(t, "ErrMCPSSEStreamPermanentlyDisconnected", p.Sentinel)
				assert.Equal(t, ErrMCPSSEStreamPermanentlyDisconnected.Error(), p.Message)
			}
		case <-ctx.Done():
			t.Fatalf("did not see permanent-disconnect sentinel within 2s; the give-up path is broken")
		}
	}
done:
	require.True(t, sawSentinel)
	assert.Equal(t, int32(3), fix.connCount.Load(),
		"server must have observed exactly 3 attempts (the configured SSEMaxConsecutiveFailures cap)")
}

// TestMCPHTTPTransport_SSE_HonoursContextCancel proves the
// goroutine wakes from the backoff sleep when ctx is cancelled.
// Without this guarantee a cancel mid-backoff would leave the
// goroutine pinned for up to maxRetry seconds.
func TestMCPHTTPTransport_SSE_HonoursContextCancel(t *testing.T) {
	t.Parallel()
	assertNoGoroutineLeak(t)

	fix := newSSEReconnectFixture(t, func(n int, w http.ResponseWriter, r *http.Request) {
		// Always refuse so the loop sits in the backoff wait.
		http.Error(w, "nope", http.StatusServiceUnavailable)
	})

	tr, err := NewMCPHTTPTransport(MCPHTTPConfig{
		Endpoint:      fix.srv.URL + "/mcp",
		SSEEndpoint:   fix.srv.URL + "/sse",
		AllowInsecure: true,
		// 5-second backoff cap so a slow exit would be
		// visibly slow — but ctx.Done should kill it
		// instantly.
		SSEInitialRetryInterval:   1 * time.Second,
		SSEMaxRetryInterval:       5 * time.Second,
		SSEMaxConsecutiveFailures: 100,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := tr.EnableSSE(ctx)
	require.NoError(t, err)

	// Let the first failed attempt land + backoff start.
	time.Sleep(150 * time.Millisecond)

	start := time.Now()
	cancel()

	// Wait for ch to close, with a 1s deadline. Without the
	// sleepOrAbort ctx-honour the goroutine would sit in a
	// 1s+ backoff and we'd see a slow drain.
	select {
	case _, ok := <-ch:
		if !ok {
			elapsed := time.Since(start)
			require.Less(t, elapsed, 500*time.Millisecond,
				"channel closed only after %s; ctx-cancel must propagate within ms (sleepOrAbort not honouring ctx)", elapsed)
			return
		}
		// Could be the sentinel if the consumer was slow;
		// keep draining briefly.
		select {
		case _, ok2 := <-ch:
			require.False(t, ok2, "expected channel to close after ctx cancel")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("channel did not close within 500ms of ctx cancel")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("channel did not close within 500ms of ctx cancel — sleepOrAbort is ignoring ctx")
	}
}

// TestComputeSSEBackoff_DoublingWithCap is a unit test for the
// pure backoff function. Confirms doubling growth, cap at
// retryInterval*8, then absolute cap at maxRetry. Jitter is
// ±25% so we assert on bounds, not exact values.
func TestComputeSSEBackoff_DoublingWithCap(t *testing.T) {
	t.Parallel()

	retryInterval := 100 * time.Millisecond
	maxRetry := 5 * time.Second

	cases := []struct {
		attempt int
		minWant time.Duration
		maxWant time.Duration
	}{
		{1, 75 * time.Millisecond, 125 * time.Millisecond},   // ~100ms ±25%
		{2, 150 * time.Millisecond, 250 * time.Millisecond},  // ~200ms ±25%
		{3, 300 * time.Millisecond, 500 * time.Millisecond},  // ~400ms ±25%
		{4, 600 * time.Millisecond, 1000 * time.Millisecond}, // ~800ms ±25%
		{5, 600 * time.Millisecond, 1000 * time.Millisecond}, // cap at 8*100=800ms ±25%
		{8, 600 * time.Millisecond, 1000 * time.Millisecond}, // still capped
	}

	for _, c := range cases {
		got := computeSSEBackoff(c.attempt, retryInterval, maxRetry)
		assert.GreaterOrEqualf(t, got, c.minWant,
			"attempt %d: got %s, want >= %s (lower jitter bound)", c.attempt, got, c.minWant)
		assert.LessOrEqualf(t, got, c.maxWant,
			"attempt %d: got %s, want <= %s (upper jitter bound)", c.attempt, got, c.maxWant)
	}
}

// TestParseSSERetryMs_HappyPathAndRejection asserts the
// spec-compliance of the `retry:` field parser.
func TestParseSSERetryMs_HappyPathAndRejection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in        string
		want      int64
		wantError bool
	}{
		{"100", 100, false},
		{"  250  ", 250, false},
		{"0", 0, false},
		{"", 0, true},
		{"  ", 0, true},
		{"abc", 0, true},
		{"100x", 0, true},
		{"-5", 0, true}, // minus sign → non-digit → reject
	}
	for _, tc := range tests {
		got, err := parseSSERetryMs(tc.in)
		if tc.wantError {
			require.Errorf(t, err, "input=%q expected error", tc.in)
		} else {
			require.NoErrorf(t, err, "input=%q expected no error", tc.in)
			assert.Equal(t, tc.want, got, "input=%q", tc.in)
		}
	}
}
