// Package continueagent provides Continue.dev CLI agent integration for HelixAgent.
package continueagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sourcegraph/jsonrpc2"
)

// LSPClient provides Language Server Protocol client functionality.
// Ported from Continue.dev's LSP integration.
//
// Concurrency model (CONST-029): handlers is a *safe.Store; requestID
// is atomic.Uint64 (dropped the mu that previously guarded an
// increment).
type LSPClient struct {
	// Server configuration
	serverCmd  string
	serverArgs []string

	// Connection
	server *jsonrpc2.Conn

	// State
	initialized bool
	rootPath    string

	// Capabilities
	serverCapabilities ServerCapabilities

	// Request tracking
	requestID atomic.Uint64

	// Notification handlers
	handlers *safe.Store[string, NotificationHandler]

	// Control
	ctx    context.Context
	cancel context.CancelFunc
}

// ServerCapabilities represents LSP server capabilities.
type ServerCapabilities struct {
	TextDocumentSync                 interface{}                      `json:"textDocumentSync"`
	CompletionProvider               *CompletionOptions               `json:"completionProvider"`
	HoverProvider                    bool                             `json:"hoverProvider"`
	SignatureHelpProvider            *SignatureHelpOptions            `json:"signatureHelpProvider"`
	DefinitionProvider               bool                             `json:"definitionProvider"`
	ReferencesProvider               bool                             `json:"referencesProvider"`
	DocumentHighlightProvider        bool                             `json:"documentHighlightProvider"`
	DocumentSymbolProvider           bool                             `json:"documentSymbolProvider"`
	CodeActionProvider               interface{}                      `json:"codeActionProvider"`
	CodeLensProvider                 *CodeLensOptions                 `json:"codeLensProvider"`
	DocumentFormattingProvider       bool                             `json:"documentFormattingProvider"`
	DocumentRangeFormattingProvider  bool                             `json:"documentRangeFormattingProvider"`
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider"`
	RenameProvider                   interface{}                      `json:"renameProvider"`
	ExecuteCommandProvider           *ExecuteCommandOptions           `json:"executeCommandProvider"`
	SelectionRangeProvider           interface{}                      `json:"selectionRangeProvider"`
}

// CompletionOptions represents completion options.
type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider"`
	TriggerCharacters []string `json:"triggerCharacters"`
}

// SignatureHelpOptions represents signature help options.
type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}

// CodeLensOptions represents code lens options.
type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider"`
}

// DocumentOnTypeFormattingOptions represents on-type formatting options.
type DocumentOnTypeFormattingOptions struct {
	FirstTriggerCharacter string   `json:"firstTriggerCharacter"`
	MoreTriggerCharacter  []string `json:"moreTriggerCharacter"`
}

// ExecuteCommandOptions represents execute command options.
type ExecuteCommandOptions struct {
	Commands []string `json:"commands"`
}

// NotificationHandler handles LSP notifications.
type NotificationHandler func(method string, params json.RawMessage)

// NewLSPClient creates a new LSP client.
func NewLSPClient(serverCmd string, serverArgs []string, rootPath string) *LSPClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &LSPClient{
		serverCmd:  serverCmd,
		serverArgs: serverArgs,
		rootPath:   rootPath,
		handlers:   safe.NewStore[string, NotificationHandler](),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the LSP server and connects to it.
func (c *LSPClient) Start(ctx context.Context) error {
	// Start server process
	cmd := exec.CommandContext(ctx, c.serverCmd, c.serverArgs...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	// Create JSON-RPC connection
	stream := jsonrpc2.NewBufferedStream(&stdioConn{stdin, stdout}, jsonrpc2.VSCodeObjectCodec{})
	c.server = jsonrpc2.NewConn(ctx, stream, jsonrpc2.HandlerWithError(c.handle))

	// Log stderr
	go c.logStderr(stderr)

	// Initialize
	if err := c.initialize(ctx); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	return nil
}

// Stop stops the LSP client.
func (c *LSPClient) Stop() error {
	if c.server != nil {
		// Send shutdown request
		c.server.Notify(c.ctx, "shutdown", nil)
		c.server.Notify(c.ctx, "exit", nil)

		c.server.Close()
	}

	c.cancel()
	return nil
}

// IsInitialized returns whether the client is initialized.
func (c *LSPClient) IsInitialized() bool {
	return c.initialized
}

// GetCapabilities returns server capabilities.
func (c *LSPClient) GetCapabilities() ServerCapabilities {
	return c.serverCapabilities
}

// RegisterHandler registers a notification handler.
func (c *LSPClient) RegisterHandler(method string, handler NotificationHandler) {
	c.handlers.Put(method, handler)
}

// TextDocumentDidOpen notifies the server a document was opened.
func (c *LSPClient) TextDocumentDidOpen(ctx context.Context, uri, languageID string, version int, text string) error {
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    version,
			Text:       text,
		},
	}

	return c.server.Notify(ctx, "textDocument/didOpen", params)
}

// TextDocumentDidChange notifies the server of document changes.
func (c *LSPClient) TextDocumentDidChange(ctx context.Context, uri string, version int, changes []TextDocumentContentChangeEvent) error {
	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: version,
		},
		ContentChanges: changes,
	}

	return c.server.Notify(ctx, "textDocument/didChange", params)
}

// TextDocumentDidClose notifies the server a document was closed.
func (c *LSPClient) TextDocumentDidClose(ctx context.Context, uri string) error {
	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	return c.server.Notify(ctx, "textDocument/didClose", params)
}

// Completion requests completions at a position.
func (c *LSPClient) Completion(ctx context.Context, uri string, line, character int) (*CompletionList, error) {
	params := CompletionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position: Position{
				Line:      line,
				Character: character,
			},
		},
	}

	var result CompletionList
	if err := c.call(ctx, "textDocument/completion", params, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Hover requests hover information.
func (c *LSPClient) Hover(ctx context.Context, uri string, line, character int) (*Hover, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position: Position{
			Line:      line,
			Character: character,
		},
	}

	var result Hover
	if err := c.call(ctx, "textDocument/hover", params, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Definition requests definition location.
func (c *LSPClient) Definition(ctx context.Context, uri string, line, character int) ([]Location, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position: Position{
			Line:      line,
			Character: character,
		},
	}

	var result []Location
	if err := c.call(ctx, "textDocument/definition", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// References requests references to a symbol.
func (c *LSPClient) References(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]Location, error) {
	params := ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position: Position{
				Line:      line,
				Character: character,
			},
		},
		Context: ReferenceContext{
			IncludeDeclaration: includeDeclaration,
		},
	}

	var result []Location
	if err := c.call(ctx, "textDocument/references", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// DocumentSymbols requests document symbols.
func (c *LSPClient) DocumentSymbols(ctx context.Context, uri string) ([]DocumentSymbol, error) {
	params := DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	var result []DocumentSymbol
	if err := c.call(ctx, "textDocument/documentSymbol", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// CodeAction requests code actions.
func (c *LSPClient) CodeAction(ctx context.Context, uri string, range_ Range, context CodeActionContext) ([]CodeAction, error) {
	params := CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        range_,
		Context:      context,
	}

	var result []CodeAction
	if err := c.call(ctx, "textDocument/codeAction", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Formatting requests document formatting.
func (c *LSPClient) Formatting(ctx context.Context, uri string, options FormattingOptions) ([]TextEdit, error) {
	params := DocumentFormattingParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Options:      options,
	}

	var result []TextEdit
	if err := c.call(ctx, "textDocument/formatting", params, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Rename requests a rename.
func (c *LSPClient) Rename(ctx context.Context, uri string, line, character int, newName string) (*WorkspaceEdit, error) {
	params := RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position: Position{
				Line:      line,
				Character: character,
			},
		},
		NewName: newName,
	}

	var result WorkspaceEdit
	if err := c.call(ctx, "textDocument/rename", params, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Internal methods

func (c *LSPClient) initialize(ctx context.Context) error {
	params := InitializeParams{
		ProcessID: int32(time.Now().Unix()), // #nosec G115 -- integer conversion bounded by reachable resource limits; overflow is mathematically unreachable
		RootPath:  c.rootPath,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Synchronization: TextDocumentSyncClientCapabilities{
					DynamicRegistration: false,
					WillSave:            true,
					WillSaveWaitUntil:   true,
					DidSave:             true,
				},
				Completion: CompletionClientCapabilities{
					DynamicRegistration: false,
					CompletionItem: CompletionItemCapabilities{
						SnippetSupport: true,
					},
				},
				Hover: HoverClientCapabilities{
					DynamicRegistration: false,
					ContentFormat:       []string{"markdown", "plaintext"},
				},
				Definition: DefinitionClientCapabilities{
					DynamicRegistration: false,
					LinkSupport:         true,
				},
			},
		},
		WorkspaceFolders: []WorkspaceFolder{
			{
				URI:  "file://" + c.rootPath,
				Name: filepath.Base(c.rootPath),
			},
		},
	}

	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return err
	}

	c.serverCapabilities = result.Capabilities
	c.initialized = true

	// Send initialized notification
	return c.server.Notify(ctx, "initialized", InitializedParams{})
}

func (c *LSPClient) call(ctx context.Context, method string, params, result interface{}) error {
	c.requestID.Add(1)
	return c.server.Call(ctx, method, params, result)
}

func (c *LSPClient) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
	if req.Notif {
		// Handle notification
		if handler, ok := c.handlers.Get(req.Method); ok {
			if req.Params != nil {
				handler(req.Method, *req.Params)
			}
		}
		return nil, nil
	}

	// Return method not found for requests we don't handle
	return nil, &jsonrpc2.Error{
		Code:    jsonrpc2.CodeMethodNotFound,
		Message: fmt.Sprintf("method not found: %s", req.Method),
	}
}

func (c *LSPClient) logStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		// Log server stderr
		_ = scanner.Text()
	}
}

// stdioConn wraps stdin/stdout for jsonrpc2
type stdioConn struct {
	io.Writer
	io.Reader
}

func (c *stdioConn) Close() error {
	return nil
}

// LSP types

// InitializeParams represents initialize request params.
type InitializeParams struct {
	ProcessID             int32              `json:"processId"`
	RootPath              string             `json:"rootPath"`
	InitializationOptions interface{}        `json:"initializationOptions,omitempty"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	Trace                 string             `json:"trace,omitempty"`
	WorkspaceFolders      []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
}

// InitializeResult represents initialize response.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// InitializedParams represents initialized notification params.
type InitializedParams struct{}

// ClientCapabilities represents client capabilities.
type ClientCapabilities struct {
	Workspace    WorkspaceClientCapabilities    `json:"workspace,omitempty"`
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

// WorkspaceClientCapabilities represents workspace capabilities.
type WorkspaceClientCapabilities struct {
	ApplyEdit bool `json:"applyEdit,omitempty"`
}

// TextDocumentClientCapabilities represents text document capabilities.
type TextDocumentClientCapabilities struct {
	Synchronization TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`
	Completion      CompletionClientCapabilities       `json:"completion,omitempty"`
	Hover           HoverClientCapabilities            `json:"hover,omitempty"`
	Definition      DefinitionClientCapabilities       `json:"definition,omitempty"`
}

// TextDocumentSyncClientCapabilities represents sync capabilities.
type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

// CompletionClientCapabilities represents completion capabilities.
type CompletionClientCapabilities struct {
	DynamicRegistration bool                       `json:"dynamicRegistration,omitempty"`
	CompletionItem      CompletionItemCapabilities `json:"completionItem,omitempty"`
}

// CompletionItemCapabilities represents completion item capabilities.
type CompletionItemCapabilities struct {
	SnippetSupport bool `json:"snippetSupport,omitempty"`
}

// HoverClientCapabilities represents hover capabilities.
type HoverClientCapabilities struct {
	DynamicRegistration bool     `json:"dynamicRegistration,omitempty"`
	ContentFormat       []string `json:"contentFormat,omitempty"`
}

// DefinitionClientCapabilities represents definition capabilities.
type DefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// WorkspaceFolder represents a workspace folder.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// TextDocumentItem represents a text document.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentIdentifier represents a text document identifier.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// VersionedTextDocumentIdentifier represents a versioned text document.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// Position represents a position in a document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a range in a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location in a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextEdit represents a text edit.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// TextDocumentContentChangeEvent represents a content change.
type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// CompletionList represents completion items.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// CompletionItem represents a completion item.
type CompletionItem struct {
	Label               string      `json:"label"`
	Kind                int         `json:"kind,omitempty"`
	Detail              string      `json:"detail,omitempty"`
	Documentation       string      `json:"documentation,omitempty"`
	SortText            string      `json:"sortText,omitempty"`
	FilterText          string      `json:"filterText,omitempty"`
	InsertText          string      `json:"insertText,omitempty"`
	InsertTextFormat    int         `json:"insertTextFormat,omitempty"`
	TextEdit            *TextEdit   `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit  `json:"additionalTextEdits,omitempty"`
	Command             *Command    `json:"command,omitempty"`
	Data                interface{} `json:"data,omitempty"`
}

// Command represents a command.
type Command struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// Hover represents hover information.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// MarkupContent represents markup content.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// DocumentSymbol represents a document symbol.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// CodeAction represents a code action.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	Command     *Command       `json:"command,omitempty"`
}

// Diagnostic represents a diagnostic.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// WorkspaceEdit represents workspace edits.
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes,omitempty"`
	DocumentChanges []TextDocumentEdit    `json:"documentChanges,omitempty"`
}

// TextDocumentEdit represents text document edits.
type TextDocumentEdit struct {
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        []TextEdit                      `json:"edits"`
}

// FormattingOptions represents formatting options.
type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

// Params types

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type CompletionParams struct {
	TextDocumentPositionParams
	Context CompletionContext `json:"context,omitempty"`
}

type CompletionContext struct {
	TriggerKind      int    `json:"triggerKind"`
	TriggerCharacter string `json:"triggerCharacter,omitempty"`
}

type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Only        []string     `json:"only,omitempty"`
}

type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}
