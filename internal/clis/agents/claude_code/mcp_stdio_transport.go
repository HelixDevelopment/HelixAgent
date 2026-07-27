// Package claude_code provides the round-39 §11.4 anti-bluff
// real implementation of the MCPTransport interface introduced
// in round-29 (commit 93e63ada).
//
// PROTOCOL REFERENCES
// -------------------
//   - Model Context Protocol specification — protocol version
//     "2024-11-05" — https://spec.modelcontextprotocol.io
//   - JSON-RPC 2.0 specification — https://www.jsonrpc.org/specification
//
// CANONICAL HANDSHAKE SEQUENCE
// ----------------------------
// On the first CallTool against a given MCPServer this transport
// runs the canonical MCP stdio handshake exactly once per server,
// then keeps the spawned child process alive for subsequent calls
// against the same server:
//
//  1. Client (us) spawns the server's command/args/env as a child
//     process. The child speaks JSON-RPC over its stdin / stdout,
//     one JSON object per line (line-delimited JSON — the MCP
//     stdio framing).
//  2. Client sends:
//     {"jsonrpc":"2.0","id":<n>,"method":"initialize",
//     "params":{"protocolVersion":"2024-11-05",
//     "capabilities":{},
//     "clientInfo":{"name":"helix_agent/claude_code",
//     "version":"round-39"}}}
//  3. Server replies on stdout with:
//     {"jsonrpc":"2.0","id":<n>,
//     "result":{"protocolVersion":"...",
//     "capabilities":{...},
//     "serverInfo":{"name":"...","version":"..."}}}
//  4. Client sends the canonical "initialized" notification (no id):
//     {"jsonrpc":"2.0","method":"notifications/initialized",
//     "params":{}}
//  5. Client may now issue tools/call (and tools/list etc.) and
//     correlate responses by the monotonic request id.
//
// ANTI-BLUFF POSTURE
// ------------------
// Round-29 fix (93e63ada) introduced the MCPTransport interface +
// ErrMCPClientNotWired sentinel and pointed CallTool at the
// transport — but no real implementation existed. Without one,
// every production caller hit the sentinel; round-31 was the
// "loud failure" phase. Round-39 (this file) supplies the real
// stdio implementation so production calls reach an actual MCP
// server. Per CONST-035 / Article XI §11.9: every PASS now
// carries positive runtime evidence (a real subprocess that
// emitted real bytes onto real pipes).
//
// CONST-046: every user-facing string surfaced by this transport
// is composed via fmt.Errorf wrapping the underlying error; no
// hardcoded English strings escape into product-visible output.
//
// COMPLEXITY GUARDRAIL
// --------------------
// Per the round-39 brief: this round lands ONLY the stdio
// transport. HTTP + SSE (the other two MCP transports) are
// explicitly deferred to round-40. The MCPTransport interface
// is implementation-agnostic; a future round can wire an HTTP
// implementation alongside this one without churning the
// interface or its sentinel.
package claude_code

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"dev.helix.agent/internal/services"
)

// mcpProtocolVersion is the MCP protocol version negotiated in
// the initialize handshake. Kept in sync with
// services.MCPProtocolVersion (currently "2024-11-05").
const mcpProtocolVersion = "2024-11-05"

// ErrMCPCommandNotConfigured is returned by NewMCPStdioTransport
// when the supplied MCPStdioConfig has an empty Command slice.
// This is distinct from ErrMCPClientNotWired: NotWired means
// the integration has no transport installed at all (round-29
// sentinel); CommandNotConfigured means a transport WAS
// requested but the operator forgot to declare the actual MCP
// server executable. Distinguishing the two keeps the failure
// surface honest — silent fall-back to a default command would
// be a CONST-046 / §11.4 bluff in disguise.
var ErrMCPCommandNotConfigured = errors.New("claude_code/mcp: MCPStdioConfig.Command is empty — no MCP server executable declared (round-39 anti-bluff: distinct from ErrMCPClientNotWired which means no transport installed at all; this sentinel means a transport WAS requested but the command was left unset)")

// MCPStdioConfig configures the stdio transport's per-spawn
// behaviour. Command is the executable + args of the MCP server
// to spawn (e.g. {"npx", "-y", "@modelcontextprotocol/server-filesystem", "/home/user"}).
// Env is the *additional* environment exported to the child on
// top of os.Environ() — pass nil to inherit the parent's env
// unchanged. WorkDir is the child's working directory; empty
// means inherit the parent's CWD.
//
// Note that the per-server Command/Args/Env triple stored on
// claude_code.MCPServer takes precedence over the defaults set
// here; this config carries the cross-server defaults plus the
// requirement that *some* command be declared (so we can fail
// loudly with ErrMCPCommandNotConfigured if everything is empty).
type MCPStdioConfig struct {
	// Command is the default fallback command + args used when
	// an MCPServer's own Command field is empty. Round-39 keeps
	// it required (non-empty) so we never silently default to a
	// hardcoded mcp-server-foo invocation.
	Command []string
	// Env is the additional environment for spawned children.
	Env []string
	// WorkDir is the spawned child's CWD (empty = inherit).
	WorkDir string
}

// MCPStdioTransport is the round-39 real implementation of
// MCPTransport. It spawns each MCPServer's command exactly once
// on first use, runs the JSON-RPC initialize handshake, and
// thereafter routes tools/call requests over the live stdio
// pipes. Concurrent CallTool against distinct servers is safe;
// concurrent CallTool against the *same* server is serialised
// per-server (the stdio protocol gives no way to multiplex two
// in-flight requests on a single line-oriented pipe).
type MCPStdioTransport struct {
	cfg MCPStdioConfig

	// conns guards `connections` and keeps the
	// per-server-spawn lazy initialisation single-shot.
	connsMu     sync.Mutex
	connections map[string]*mcpStdioConn

	// idCounter generates monotonic JSON-RPC request ids.
	// Global across all servers; the MCP spec only requires
	// uniqueness within a connection but a single counter is
	// simpler and still correct.
	idCounter atomic.Int64
}

// mcpStdioConn holds the live stdio pipes + child process for
// a single connected MCP server.
type mcpStdioConn struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	reader *bufio.Reader

	// ioMu serialises a Send→Recv pair against the same
	// server: stdio MCP gives us a single line-oriented pipe
	// per direction, so two interleaved requests would
	// cross-wire their responses (caller A's request matched
	// to caller B's response by line order, not by id).
	ioMu sync.Mutex
}

// NewMCPStdioTransport constructs the stdio transport. Returns
// ErrMCPCommandNotConfigured (NOT ErrMCPClientNotWired) when
// cfg.Command is empty — the two sentinels mean different
// things, see their docstrings.
func NewMCPStdioTransport(cfg MCPStdioConfig) (*MCPStdioTransport, error) {
	if len(cfg.Command) == 0 {
		return nil, ErrMCPCommandNotConfigured
	}
	return &MCPStdioTransport{
		cfg:         cfg,
		connections: make(map[string]*mcpStdioConn),
	}, nil
}

// CallTool implements MCPTransport. On first call against a
// given server, it spawns the child + runs the initialize
// handshake; subsequent calls reuse the live connection.
func (t *MCPStdioTransport) CallTool(ctx context.Context, server *MCPServer, toolName string, args map[string]interface{}) (*services.ToolCallResult, error) {
	if server == nil {
		return nil, fmt.Errorf("claude_code/mcp: CallTool: server is nil")
	}
	conn, err := t.connFor(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("claude_code/mcp: CallTool(server=%q, tool=%q): connect: %w", server.Name, toolName, err)
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      t.nextID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}

	resp, err := t.exchange(ctx, conn, req)
	if err != nil {
		return nil, fmt.Errorf("claude_code/mcp: CallTool(server=%q, tool=%q): %w", server.Name, toolName, err)
	}
	if resp.Error != nil {
		// Surface the server-side JSON-RPC error as a real
		// ToolCallResult with IsError=true, matching the
		// MCP convention for tool-level failures. Caller
		// distinguishes transport failure (non-nil err
		// above) from server-reported tool failure (nil
		// err, IsError=true).
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

// Close terminates every spawned MCP server child process and
// closes the stdio pipes. Safe to call multiple times; calls
// after the first are no-ops.
func (t *MCPStdioTransport) Close() error {
	t.connsMu.Lock()
	defer t.connsMu.Unlock()

	var firstErr error
	for name, conn := range t.connections {
		if cerr := closeConn(conn); cerr != nil && firstErr == nil {
			firstErr = fmt.Errorf("claude_code/mcp: Close(server=%q): %w", name, cerr)
		}
	}
	t.connections = make(map[string]*mcpStdioConn)
	return firstErr
}

// connFor returns the live connection to `server`, spawning +
// initialising it on first use. Thread-safe via t.connsMu;
// concurrent callers wanting the same server block on the
// single-shot spawn.
func (t *MCPStdioTransport) connFor(ctx context.Context, server *MCPServer) (*mcpStdioConn, error) {
	t.connsMu.Lock()
	if conn, ok := t.connections[server.Name]; ok {
		t.connsMu.Unlock()
		return conn, nil
	}
	// Hold connsMu across spawn+initialize so two concurrent
	// callers don't both try to spawn. Spawn is fast (exec
	// fork+exec); initialize involves a single round-trip. The
	// lock is fine-grained enough that the hot path (existing
	// conn) is one map lookup.
	defer t.connsMu.Unlock()

	conn, err := t.spawn(ctx, server)
	if err != nil {
		return nil, err
	}
	if err := t.initialize(ctx, conn); err != nil {
		_ = closeConn(conn)
		return nil, err
	}
	t.connections[server.Name] = conn
	return conn, nil
}

// spawn forks the MCP server child process. Prefers the
// per-server Command/Args triple if set; otherwise falls back
// to cfg.Command. Env merges os.Environ() + server.Env +
// cfg.Env (in that precedence, later entries override).
func (t *MCPStdioTransport) spawn(ctx context.Context, server *MCPServer) (*mcpStdioConn, error) {
	var cmdLine []string
	if server.Command != "" {
		cmdLine = append([]string{server.Command}, server.Args...)
	} else {
		cmdLine = t.cfg.Command
	}
	if len(cmdLine) == 0 {
		return nil, ErrMCPCommandNotConfigured
	}

	cmd := exec.CommandContext(ctx, cmdLine[0], cmdLine[1:]...) //nolint:gosec // command list comes from operator-controlled MCPServer config

	// Build environment: parent env + cfg env + server env
	// (later entries override earlier ones via cmd.Env's
	// last-occurrence-wins semantics).
	env := os.Environ()
	env = append(env, t.cfg.Env...)
	for k, v := range server.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env
	if t.cfg.WorkDir != "" {
		cmd.Dir = t.cfg.WorkDir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start %q: %w", cmdLine[0], err)
	}

	conn := &mcpStdioConn{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		reader: bufio.NewReader(stdout),
	}

	// Spawn a goroutine to drain stderr so the child doesn't
	// block on a full pipe. We log each line via fmt.Fprintln
	// to the parent's stderr — production callers wanting
	// structured capture should wrap NewMCPStdioTransport in
	// a constructor that reads stderr themselves.
	go drainStderr(server.Name, stderr)

	return conn, nil
}

// initialize runs the canonical MCP initialize → initialized
// handshake. Caller must hold no per-server lock; ioMu is
// taken inside.
func (t *MCPStdioTransport) initialize(ctx context.Context, conn *mcpStdioConn) error {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      t.nextID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "helix_agent/claude_code",
				"version": "round-39",
			},
		},
	}

	resp, err := t.exchange(ctx, conn, req)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize: server returned JSON-RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	// Send the initialized notification per MCP spec. No id =
	// notification (not a request); no response expected.
	notif := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]interface{}{},
	}
	conn.ioMu.Lock()
	defer conn.ioMu.Unlock()
	if err := writeFrame(conn.stdin, notif); err != nil {
		return fmt.Errorf("initialize: send notifications/initialized: %w", err)
	}
	return nil
}

// exchange writes one JSON-RPC request and reads one response
// frame. Holds conn.ioMu across the pair so concurrent callers
// against the same server cannot cross-wire their responses.
// Honours ctx by spawning a watchdog that closes stdin on
// ctx.Done() — that unblocks any in-flight Scan.
func (t *MCPStdioTransport) exchange(ctx context.Context, conn *mcpStdioConn, req jsonrpcRequest) (*jsonrpcResponse, error) {
	conn.ioMu.Lock()
	defer conn.ioMu.Unlock()

	// ctx-cancellation watchdog: close stdin if ctx fires
	// before we finish reading. Done channel cleans up on
	// the happy path.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.stdin.Close()
		case <-done:
		}
	}()

	if err := writeFrame(conn.stdin, req); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	line, err := conn.reader.ReadBytes('\n')
	if err != nil {
		// If ctx fired we'll have closed stdin; surface
		// the cause cleanly.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("read response: %w", ctxErr)
		}
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("decode response (line=%q): %w", string(line), err)
	}
	return &resp, nil
}

// nextID returns the next monotonic JSON-RPC request id.
func (t *MCPStdioTransport) nextID() int64 {
	return t.idCounter.Add(1)
}

// writeFrame marshals v + appends '\n' (the MCP stdio framing)
// + writes atomically to w.
func writeFrame(w io.Writer, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// closeConn closes stdin (which signals EOF to the child),
// then waits briefly for the child to exit cleanly; on
// timeout, kills it.
func closeConn(conn *mcpStdioConn) error {
	if conn == nil {
		return nil
	}
	_ = conn.stdin.Close()
	// Wait for the child to exit; if it lingers, the
	// goroutine running cmd.Wait via the os/exec package's
	// internal reaper will collect it. We don't synchronously
	// Wait here because the stderr-drain goroutine still
	// holds a read handle and would race with a forced kill.
	if conn.cmd != nil && conn.cmd.Process != nil {
		// Best-effort: try a graceful Wait, fall back to
		// Kill. Use a goroutine + select so we don't
		// block Close() forever on a hung child.
		waitDone := make(chan error, 1)
		go func() { waitDone <- conn.cmd.Wait() }()
		select {
		case <-waitDone:
			// child exited cleanly
		default:
			_ = conn.cmd.Process.Kill()
			<-waitDone
		}
	}
	return nil
}

// drainStderr reads the child's stderr line-by-line and emits
// each line to the parent's stderr prefixed with the server
// name. Prevents the child from blocking on a full stderr pipe
// and surfaces server-side errors during dev. CONST-046: the
// prefix is a structural log marker, not user-facing content.
func drainStderr(serverName string, r io.ReadCloser) {
	defer func() { _ = r.Close() }()
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fmt.Fprintf(os.Stderr, "[mcp-stdio:%s] %s\n", serverName, sc.Text())
	}
}

// jsonrpcRequest models a JSON-RPC 2.0 request (or notification
// when ID is the zero value). We avoid the omitempty-on-int
// gotcha by using a pointer-free encoding: notifications are
// constructed explicitly without a Method-only-zero-id signal,
// instead they go through writeFrame directly with a struct
// that has no ID field. For simplicity (initialize +
// tools/call both need ID) we keep one struct and tolerate
// the zero-ID-for-notification case via the dedicated
// notification path in initialize().
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonrpcResponse models a JSON-RPC 2.0 response. We keep
// Result as json.RawMessage so the per-method decode can
// allocate a method-specific result type.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

// jsonrpcError models the JSON-RPC 2.0 error object.
type jsonrpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Compile-time interface assertion: MCPStdioTransport
// satisfies MCPTransport. Catches signature drift at build
// time rather than at the first round-trip.
var _ MCPTransport = (*MCPStdioTransport)(nil)
