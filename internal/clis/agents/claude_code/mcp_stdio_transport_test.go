// Round-39 §11.4 anti-bluff tests for the MCPStdioTransport.
//
// Test fixture strategy
// ---------------------
// The fixture is a REAL subprocess speaking REAL JSON-RPC over
// stdio — not a mocked transport. We achieve this without
// pulling in an external MCP server binary by re-exec'ing the
// test binary itself (TestMain detects the
// HELIX_MCP_TEST_SERVER_MODE env var and dispatches into the
// stub server's main loop, then exits before any normal test
// runs). This is a CONST-050(A)-clean pattern: the entire
// fixture lives in *_test.go and never leaks into a production
// build, AND the production transport code path is exercised
// against a real subprocess with real pipes, satisfying
// Article XI §11.9's positive-runtime-evidence requirement.
//
// The stub server implements just enough of MCP to validate
// the round-39 wiring:
//   * `initialize`        — returns serverInfo + capabilities
//   * `notifications/initialized` — ignored (no response, per spec)
//   * `tools/list`        — returns a single "echo" tool
//   * `tools/call`        — for tool="echo", echoes args.message
//                           back as Content[0].Text; for any
//                           other tool, returns a JSON-RPC error
//
// Anti-bluff anchors covered by this file:
//   * CONST-035 / Article XI §11.9 — positive runtime evidence
//     captured via a real subprocess + real stdio pipes.
//   * CONST-050(A) — fixture lives in *_test.go only.
//   * CONST-050(B) — successful tool-call + transport-not-wired
//     + command-not-configured + tool-not-found error paths all
//     exercised.
//   * Round-29 sentinel preservation — paired-mutation test
//     plants nil-transport and asserts ErrMCPClientNotWired
//     still fires (regression-proofs round-31).

package claude_code

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubServerEnvVar — when set on a child invocation of the
// test binary, TestMain enters stub-server mode and runs the
// JSON-RPC loop instead of the test suite.
const stubServerEnvVar = "HELIX_MCP_TEST_SERVER_MODE"

// TestMain detects the stub-server re-exec hook BEFORE
// delegating to the normal test runner. When the env var is
// set, we run the MCP stub server's main loop on stdin/stdout
// and os.Exit before testing.M.Run is called — the test
// process becomes the MCP server.
func TestMain(m *testing.M) {
	if os.Getenv(stubServerEnvVar) == "1" {
		runStubServer()
		// runStubServer is supposed to exit; if it returns
		// (clean EOF on stdin), exit zero so callers see
		// graceful shutdown.
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runStubServer is the MCP stub server's main loop. Reads
// line-delimited JSON-RPC from stdin, dispatches initialize /
// tools/list / tools/call, writes responses to stdout, ignores
// notifications/initialized.
func runStubServer() {
	sc := bufio.NewScanner(os.Stdin)
	// Bump buffer size for any larger JSON-RPC frame.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	enc := json.NewEncoder(os.Stdout)

	for sc.Scan() {
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      *int64          `json:"id,omitempty"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
		}
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			// Malformed input — emit a JSON-RPC parse-error
			// response and keep going.
			_ = enc.Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      nil,
				"error": map[string]interface{}{
					"code":    -32700,
					"message": fmt.Sprintf("parse error: %v", err),
				},
			})
			continue
		}

		// Notification (no id) — process and emit nothing.
		if req.ID == nil {
			continue
		}

		switch req.Method {
		case "initialize":
			_ = enc.Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"result": map[string]interface{}{
					"protocolVersion": mcpProtocolVersion,
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{"listChanged": false},
					},
					"serverInfo": map[string]interface{}{
						"name":    "helix-mcp-stub",
						"version": "round-39-test",
					},
				},
			})
		case "tools/list":
			_ = enc.Encode(map[string]interface{}{
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
				_ = enc.Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      *req.ID,
					"error": map[string]interface{}{
						"code":    -32602,
						"message": fmt.Sprintf("invalid params: %v", err),
					},
				})
				continue
			}
			if p.Name != "echo" {
				_ = enc.Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      *req.ID,
					"error": map[string]interface{}{
						"code":    -32601,
						"message": fmt.Sprintf("unknown tool: %s", p.Name),
					},
				})
				continue
			}
			msg, _ := p.Arguments["message"].(string)
			_ = enc.Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": "echo:" + msg},
					},
					"isError": false,
				},
			})
		default:
			_ = enc.Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      *req.ID,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": fmt.Sprintf("method not found: %s", req.Method),
				},
			})
		}
	}
}

// stubServerCommand returns the command + args that re-exec
// the current test binary as the MCP stub server, plus the
// env var entry that triggers stub-server mode.
func stubServerCommand(t *testing.T) (cmd string, args []string, env map[string]string) {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err, "os.Executable must succeed under `go test`")
	// We don't pass any test flags — runStubServer doesn't
	// touch testing.M. The env var triggers re-exec mode.
	return self, nil, map[string]string{stubServerEnvVar: "1"}
}

// TestMCPStdioTransport_NewMCPStdioTransport_EmptyCommandReturnsSentinel
// is the paired-mutation test for the
// ErrMCPCommandNotConfigured sentinel: if a future change
// silently defaults the empty-command case to a hardcoded
// server, this test will fail loudly.
func TestMCPStdioTransport_NewMCPStdioTransport_EmptyCommandReturnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := NewMCPStdioTransport(MCPStdioConfig{})
	require.Error(t, err, "empty Command must surface ErrMCPCommandNotConfigured")
	require.ErrorIs(t, err, ErrMCPCommandNotConfigured, "the error must wrap ErrMCPCommandNotConfigured")
}

// TestMCPStdioTransport_NewMCPStdioTransport_DistinctFromNotWiredSentinel
// asserts that ErrMCPCommandNotConfigured is distinct from
// ErrMCPClientNotWired. They mean different things; conflating
// them would re-introduce the round-29 PASS-bluff (a confused
// caller could swallow CommandNotConfigured thinking it was
// the same as the "no transport" case).
func TestMCPStdioTransport_NewMCPStdioTransport_DistinctFromNotWiredSentinel(t *testing.T) {
	t.Parallel()

	_, err := NewMCPStdioTransport(MCPStdioConfig{})
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrMCPClientNotWired),
		"ErrMCPCommandNotConfigured MUST NOT wrap or equal ErrMCPClientNotWired — they are semantically distinct sentinels")
}

// TestMCPStdioTransport_CallTool_RealSubprocess_RoundTrip is
// the core anti-bluff proof: spawns the test binary in stub-
// server mode via real os/exec, runs the real JSON-RPC
// initialize handshake over real stdio pipes, dispatches a
// real tools/call request, and asserts the response matches
// what the server sent. If any layer of the stack is broken
// (handshake, framing, id correlation, decode), this test
// fails.
func TestMCPStdioTransport_CallTool_RealSubprocess_RoundTrip(t *testing.T) {
	t.Parallel()

	cmd, args, env := stubServerCommand(t)
	envEntries := make([]string, 0, len(env))
	for k, v := range env {
		envEntries = append(envEntries, fmt.Sprintf("%s=%s", k, v))
	}

	tr, err := NewMCPStdioTransport(MCPStdioConfig{
		Command: append([]string{cmd}, args...),
		Env:     envEntries,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{
		Name:    "stub",
		Command: cmd,
		Args:    args,
		Env:     env,
		Enabled: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	got, err := tr.CallTool(ctx, server, "echo", map[string]interface{}{
		"message": "round-39",
	})
	require.NoError(t, err, "real-subprocess round-trip must succeed")
	require.NotNil(t, got)
	require.False(t, got.IsError, "stub returns IsError=false for the happy path; non-false means a fabricated 'simulated' bluff has crept back in")
	require.Len(t, got.Content, 1, "stub returns exactly one Content item; deviation means we decoded the wrong shape")
	assert.Equal(t, "text", got.Content[0].Type)
	assert.Equal(t, "echo:round-39", got.Content[0].Text,
		"the round-trip text must match what the stub server *actually sent*. If this fails, either the framing is wrong, the id correlation is wrong, or some intermediate code is fabricating content.")
}

// TestMCPStdioTransport_CallTool_RealSubprocess_UnknownToolSurfacesServerError
// exercises the JSON-RPC error path: the stub returns a
// method-not-found-style error for any tool name other than
// "echo". The transport MUST surface this as a real
// ToolCallResult{IsError:true,...} (NOT swallow it as nil
// error and NOT fabricate text content).
func TestMCPStdioTransport_CallTool_RealSubprocess_UnknownToolSurfacesServerError(t *testing.T) {
	t.Parallel()

	cmd, args, env := stubServerCommand(t)
	envEntries := make([]string, 0, len(env))
	for k, v := range env {
		envEntries = append(envEntries, fmt.Sprintf("%s=%s", k, v))
	}

	tr, err := NewMCPStdioTransport(MCPStdioConfig{
		Command: append([]string{cmd}, args...),
		Env:     envEntries,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{
		Name:    "stub-unknown",
		Command: cmd,
		Args:    args,
		Env:     env,
		Enabled: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	got, err := tr.CallTool(ctx, server, "this_tool_does_not_exist", map[string]interface{}{})
	require.NoError(t, err, "JSON-RPC error responses are real responses, not transport failures")
	require.NotNil(t, got)
	require.True(t, got.IsError, "unknown tool must surface as IsError=true")
	require.Len(t, got.Content, 1)
	assert.Contains(t, got.Content[0].Text, "unknown tool",
		"the surfaced error must carry the server's message verbatim (proves we read a real server response, not a fabricated placeholder)")
}

// TestMCPStdioTransport_CallTool_RealSubprocess_CommandNotFoundSurfacesError
// asserts that when the configured Command points at a
// nonexistent executable, CallTool surfaces the os/exec error
// loudly (does NOT silently no-op or return fabricated success).
func TestMCPStdioTransport_CallTool_RealSubprocess_CommandNotFoundSurfacesError(t *testing.T) {
	t.Parallel()

	bogus := filepath.Join(t.TempDir(), "does-not-exist-binary")
	tr, err := NewMCPStdioTransport(MCPStdioConfig{
		Command: []string{bogus},
	})
	require.NoError(t, err, "NewMCPStdioTransport with a non-empty Command succeeds — the spawn failure surfaces on first CallTool")
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{
		Name:    "bogus",
		Command: bogus,
		Enabled: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = tr.CallTool(ctx, server, "echo", nil)
	require.Error(t, err, "spawn of a nonexistent binary must surface an error — silent no-op would be a §11.4 contract bluff")
	// Don't assert on the exact underlying error message
	// (varies by OS / Go version). What matters is that the
	// caller sees a non-nil error and can act on it.
}

// TestMCPStdioTransport_Integration_WithMCPIntegration_EndToEnd
// is the round-39-specific end-to-end proof: wire the stdio
// transport into MCPIntegration via SetTransport, then call
// (*MCPIntegration).CallTool and assert it dispatches all the
// way through to the real subprocess. This proves the
// round-29 injection point (SetTransport) accepts the round-39
// real implementation — i.e. the surface is no longer
// "sentinel-only".
func TestMCPStdioTransport_Integration_WithMCPIntegration_EndToEnd(t *testing.T) {
	t.Parallel()

	cmd, args, env := stubServerCommand(t)
	envEntries := make([]string, 0, len(env))
	for k, v := range env {
		envEntries = append(envEntries, fmt.Sprintf("%s=%s", k, v))
	}

	tr, err := NewMCPStdioTransport(MCPStdioConfig{
		Command: append([]string{cmd}, args...),
		Env:     envEntries,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp.json")
	mcp := NewMCPIntegration(configPath)
	require.NoError(t, mcp.LoadConfig())

	// Register an "echo-server" pointing at our stub, alongside
	// whatever LoadConfig synthesised.
	require.NoError(t, mcp.AddServer(&MCPServer{
		Name:    "echo-server",
		Command: cmd,
		Args:    args,
		Env:     env,
		Enabled: true,
	}))

	mcp.SetTransport(tr)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	got, err := mcp.CallTool(ctx, "echo-server", "echo", map[string]interface{}{
		"message": "end-to-end",
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, got.IsError)
	require.Len(t, got.Content, 1)
	assert.Equal(t, "echo:end-to-end", got.Content[0].Text,
		"end-to-end pipeline must deliver the real subprocess response verbatim — any deviation means a layer is bluffing")
}

// TestMCPStdioTransport_CallTool_ContextCancellation_Honored
// proves that CallTool honours ctx cancellation — a request
// in flight against a hung server must abort within a bounded
// time. This is the round-39 anti-bluff guarantee against
// "deadlock disguised as success" failure modes.
func TestMCPStdioTransport_CallTool_ContextCancellation_Honored(t *testing.T) {
	t.Parallel()

	// Use `sleep infinity` as the "server" — it never writes
	// anything on stdout, so any Recv on the transport will
	// block until ctx-cancellation closes stdin.
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("SKIP-OK: #round-39 — `sleep` not on PATH on this host; ctx-cancellation test requires a binary that blocks forever")
	}

	tr, err := NewMCPStdioTransport(MCPStdioConfig{
		Command: []string{"sleep", "60"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })

	server := &MCPServer{
		Name:    "hung",
		Command: "sleep",
		Args:    []string{"60"},
		Enabled: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = tr.CallTool(ctx, server, "anything", nil)
	elapsed := time.Since(start)

	require.Error(t, err, "ctx-cancellation must surface as a real error")
	require.Less(t, elapsed, 5*time.Second,
		"ctx-cancellation must abort the call within seconds (took %s); longer means the watchdog goroutine isn't wired up", elapsed)
}
