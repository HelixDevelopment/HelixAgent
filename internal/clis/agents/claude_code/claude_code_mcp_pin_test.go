package claude_code

import (
	"context"
	"strings"
	"testing"

	"dev.helix.agent/internal/clis/agents/base"
	"dev.helix.agent/internal/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// D-18 (BLUFF-001) pin guards for claude_code.handleMCP.
//
// RED-on-broken-artifact + polarity-switch guards per §11.4.115.
//
// Pre-fix, handleMCP("list") returned a HARDCODED server list string
// ("Available MCP servers: filesystem, github, memory, fetch, puppeteer")
// regardless of any real configuration, and handleMCP("call") returned a
// fabricated "Called <server>/<tool> via MCP" echo WITHOUT ever invoking the
// real MCP transport — a §11.4 contract-bluff at the agent/MCP boundary (the
// caller believes a tool ran when nothing was dispatched).
//
// The package already exposes a real MCP transport seam: mcp_integration.go's
// (*MCPIntegration).CallTool routes a real `tools/call` through the wired
// MCPTransport and returns ErrMCPClientNotWired when no transport is installed.
// The fix routes handleMCP through that seam:
//   - "list" reports the REAL configured servers (none configured → honest
//     "no MCP servers configured"), never the hardcoded fleet.
//   - "call" routes through CallTool → real transport result when wired, or the
//     honest ErrMCPClientNotWired when not wired — never the fabricated echo.

// stubTransport is a unit-test-only deterministic MCPTransport (CONST-050(A)).
// It records the call and returns a fixed, unforgeable marker so the test can
// prove the result flowed from the transport, not a template.
type stubTransport struct {
	called     bool
	lastServer string
	lastTool   string
	marker     string
}

func (s *stubTransport) CallTool(ctx context.Context, server *MCPServer, toolName string, args map[string]interface{}) (*services.ToolCallResult, error) {
	s.called = true
	s.lastServer = server.Name
	s.lastTool = toolName
	return &services.ToolCallResult{
		Content: []services.Content{{Type: "text", Text: s.marker}},
	}, nil
}

func newClaudeWithMCP(t *testing.T) (*ClaudeCode, context.Context) {
	t.Helper()
	cc := New()
	ctx := context.Background()
	require.NoError(t, cc.Initialize(ctx, &Config{BaseConfig: base.BaseConfig{WorkDir: t.TempDir()}}))
	return cc, ctx
}

func TestPin_HandleMCPList_NotHardcoded(t *testing.T) {
	cc, ctx := newClaudeWithMCP(t)

	if isRedMode() {
		// Reproduce the defect on the pre-fix artifact: list returned the
		// hardcoded fleet literal regardless of real configuration.
		res, err := cc.Execute(ctx, "mcp", map[string]interface{}{"action": "list"})
		require.NoError(t, err)
		resp := res.(*Response)
		assert.Contains(t, resp.Content, "filesystem, github, memory, fetch, puppeteer",
			"RED: pre-fix handleMCP list returns a hardcoded server fleet")
		return
	}

	// GREEN: with no real MCP servers configured, list MUST honestly report
	// that there are none — NEVER the hardcoded fleet.
	res, err := cc.Execute(ctx, "mcp", map[string]interface{}{"action": "list"})
	require.NoError(t, err)
	resp := res.(*Response)
	assert.NotContains(t, resp.Content, "filesystem, github, memory, fetch, puppeteer",
		"GREEN: the hardcoded server-fleet literal must be gone")
	assert.Contains(t, strings.ToLower(resp.Content), "no mcp servers",
		"GREEN: with nothing configured, list must honestly say no servers are configured")

	// And when a real server IS configured, list MUST reflect it.
	cc.mcp.servers["my-real-server"] = &MCPServer{Name: "my-real-server", Enabled: true}
	res2, err := cc.Execute(ctx, "mcp", map[string]interface{}{"action": "list"})
	require.NoError(t, err)
	resp2 := res2.(*Response)
	assert.Contains(t, resp2.Content, "my-real-server",
		"GREEN: list must report the REAL configured server")
}

func TestPin_HandleMCPCall_HonestErrorWhenNotWired(t *testing.T) {
	cc, ctx := newClaudeWithMCP(t)
	// Configure a real, enabled server but DO NOT wire a transport.
	cc.mcp.servers["fs"] = &MCPServer{Name: "fs", Enabled: true}

	if isRedMode() {
		// Reproduce the defect: call fabricated a "Called fs/read via MCP" echo
		// with no real dispatch.
		res, err := cc.Execute(ctx, "mcp", map[string]interface{}{
			"action": "call", "server": "fs", "tool": "read",
		})
		require.NoError(t, err)
		resp := res.(*Response)
		assert.Contains(t, resp.Content, "Called fs/read via MCP",
			"RED: pre-fix handleMCP call fabricates a 'Called .. via MCP' echo without dispatching")
		return
	}

	// GREEN: with no transport wired, call MUST surface the honest
	// ErrMCPClientNotWired — NEVER a fabricated success echo.
	_, err := cc.Execute(ctx, "mcp", map[string]interface{}{
		"action": "call", "server": "fs", "tool": "read",
	})
	require.Error(t, err, "GREEN: call without a wired transport must return an honest error, never a fabricated echo")
	assert.ErrorIs(t, err, ErrMCPClientNotWired,
		"GREEN: the honest error must be ErrMCPClientNotWired")
}

func TestPin_HandleMCPCall_RealTransportResult(t *testing.T) {
	if isRedMode() {
		t.Skip("SKIP-OK: real-transport dispatch path only exists post-fix")
	}
	cc, ctx := newClaudeWithMCP(t)
	cc.mcp.servers["fs"] = &MCPServer{Name: "fs", Enabled: true}

	const marker = "PIN-MCP-7c4d-REALDISPATCH"
	st := &stubTransport{marker: marker}
	cc.mcp.SetTransport(st)

	res, err := cc.Execute(ctx, "mcp", map[string]interface{}{
		"action": "call", "server": "fs", "tool": "read",
	})
	require.NoError(t, err)
	resp := res.(*Response)

	assert.True(t, st.called, "GREEN: call must dispatch through the real MCP transport")
	assert.Equal(t, "fs", st.lastServer)
	assert.Equal(t, "read", st.lastTool)
	assert.Contains(t, resp.Content, marker,
		"GREEN: the transport's result MUST flow through (unforgeable by any template)")
	assert.NotContains(t, resp.Content, "Called fs/read via MCP",
		"GREEN: the fabricated echo literal must be gone")
}
