package continueagent

// Phase-2 unit tests for continueagent.LSPClient.
//
// The LSP client spawns a real language-server subprocess via exec.Command,
// which is not something we can (or should) do in a hermetic unit test.
// These tests therefore pin the parts of the package that CAN be validated
// without IO: constructor state, handler registration, stdioConn behaviour,
// and JSON round-tripping of the LSP type surface (which carries the only
// real risk of drift between the wire protocol and Go struct tags).

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLSPClient_Defaults(t *testing.T) {
	c := NewLSPClient("gopls", []string{"-rpc.trace"}, "/tmp/project")
	require.NotNil(t, c)
	assert.Equal(t, "gopls", c.serverCmd)
	assert.Equal(t, []string{"-rpc.trace"}, c.serverArgs)
	assert.Equal(t, "/tmp/project", c.rootPath)
	assert.False(t, c.initialized, "fresh client must not claim initialized")
	assert.False(t, c.IsInitialized())
	assert.NotNil(t, c.handlers)
	assert.NotNil(t, c.ctx)
	assert.NotNil(t, c.cancel)

	// Capabilities on a fresh client must be the zero value — no server
	// contact has happened yet.
	caps := c.GetCapabilities()
	assert.False(t, caps.HoverProvider)
	assert.False(t, caps.DefinitionProvider)
}

func TestLSPClient_RegisterHandler(t *testing.T) {
	c := NewLSPClient("gopls", nil, "/tmp")
	called := false
	c.RegisterHandler("textDocument/publishDiagnostics", func(method string, params json.RawMessage) {
		called = true
		assert.Equal(t, "textDocument/publishDiagnostics", method)
	})

	// Handler is stored under the exact method key.
	handler, ok := c.handlers.Get("textDocument/publishDiagnostics")
	require.True(t, ok, "handler must be registered")
	require.NotNil(t, handler)

	// Direct invocation (we can't drive jsonrpc2 here) proves the closure
	// was stored with the right signature.
	handler("textDocument/publishDiagnostics", json.RawMessage(`{}`))
	assert.True(t, called)

	// Re-registering overwrites, not appends.
	c.RegisterHandler("textDocument/publishDiagnostics", func(string, json.RawMessage) {})
	assert.Equal(t, 1, c.handlers.Len())
}

func TestStdioConn_CloseIsNoOp(t *testing.T) {
	// stdioConn.Close() intentionally returns nil (it's a wrapper type
	// whose underlying reader/writer are the child process's stdio
	// pipes, closed by the exec.Cmd lifecycle). Pin that contract.
	c := &stdioConn{}
	assert.NoError(t, c.Close())
}

func TestLSPTypes_MarshalRoundTrip(t *testing.T) {
	// The entire LSP type surface is the public contract with the
	// language server. Any accidental json tag drift shows up here.
	cases := []struct {
		name   string
		value  interface{}
		expect string
	}{
		{
			name:   "Position",
			value:  Position{Line: 10, Character: 4},
			expect: `{"line":10,"character":4}`,
		},
		{
			name: "Range",
			value: Range{
				Start: Position{Line: 1, Character: 2},
				End:   Position{Line: 3, Character: 4},
			},
			expect: `{"start":{"line":1,"character":2},"end":{"line":3,"character":4}}`,
		},
		{
			name:   "TextDocumentIdentifier",
			value:  TextDocumentIdentifier{URI: "file:///tmp/a.go"},
			expect: `{"uri":"file:///tmp/a.go"}`,
		},
		{
			name:   "WorkspaceFolder",
			value:  WorkspaceFolder{URI: "file:///tmp", Name: "tmp"},
			expect: `{"uri":"file:///tmp","name":"tmp"}`,
		},
		{
			name:   "CompletionItem_LabelOnly",
			value:  CompletionItem{Label: "println"},
			expect: `{"label":"println"}`,
		},
		{
			name:   "InitializedParams_Empty",
			value:  InitializedParams{},
			expect: `{}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			require.NoError(t, err)
			assert.JSONEq(t, tc.expect, string(data))
		})
	}
}

func TestLSPTypes_UnmarshalInitializeResult(t *testing.T) {
	// A real gopls-style initialize response. Must decode without loss
	// into InitializeResult + ServerCapabilities.
	payload := `{
		"capabilities": {
			"textDocumentSync": 1,
			"hoverProvider": true,
			"definitionProvider": true,
			"referencesProvider": true,
			"documentFormattingProvider": true,
			"renameProvider": true,
			"completionProvider": {
				"resolveProvider": false,
				"triggerCharacters": [".", ":"]
			}
		}
	}`
	var result InitializeResult
	require.NoError(t, json.Unmarshal([]byte(payload), &result))

	assert.True(t, result.Capabilities.HoverProvider)
	assert.True(t, result.Capabilities.DefinitionProvider)
	assert.True(t, result.Capabilities.ReferencesProvider)
	assert.True(t, result.Capabilities.DocumentFormattingProvider)
	require.NotNil(t, result.Capabilities.CompletionProvider)
	assert.Equal(t, []string{".", ":"}, result.Capabilities.CompletionProvider.TriggerCharacters)
}

func TestLSPTypes_CompletionListRoundTrip(t *testing.T) {
	original := CompletionList{
		IsIncomplete: true,
		Items: []CompletionItem{
			{Label: "Println", Kind: 3, Detail: "fmt.Println"},
			{Label: "Sprintf", Kind: 3, Detail: "fmt.Sprintf"},
		},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded CompletionList
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.True(t, decoded.IsIncomplete)
	require.Len(t, decoded.Items, 2)
	assert.Equal(t, "Println", decoded.Items[0].Label)
	assert.Equal(t, 3, decoded.Items[0].Kind)
}

func TestLSPTypes_WorkspaceEditWithChanges(t *testing.T) {
	edit := WorkspaceEdit{
		Changes: map[string][]TextEdit{
			"file:///tmp/a.go": {
				{
					Range: Range{
						Start: Position{Line: 0, Character: 0},
						End:   Position{Line: 0, Character: 10},
					},
					NewText: "replacement",
				},
			},
		},
	}
	data, err := json.Marshal(edit)
	require.NoError(t, err)

	var decoded WorkspaceEdit
	require.NoError(t, json.Unmarshal(data, &decoded))
	edits, ok := decoded.Changes["file:///tmp/a.go"]
	require.True(t, ok)
	require.Len(t, edits, 1)
	assert.Equal(t, "replacement", edits[0].NewText)
}
