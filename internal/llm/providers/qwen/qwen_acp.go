package qwen

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/models"
)

// QwenACPProvider implements the LLMProvider interface using Qwen Code's ACP
// (Agent Communication Protocol) over stdin/stdout.
//
// This provides a more powerful integration than CLI:
// - Session management (conversation history)
// - Streaming responses
// - Proper authentication handling
// - Tool support
type QwenACPProvider struct {
	model       string
	timeout     time.Duration
	maxTokens   int
	cwd         string
	initialized bool

	// Process management
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	scanner   *bufio.Scanner
	mu        sync.Mutex
	requestID int64
	sessionID string
	isRunning bool
	startOnce sync.Once
	startErr  error

	// Response channels for async handling. CONST-029: safe.Store
	// serialises Put/Delete/Get without a caller-held lock.
	responses *safe.Store[int64, chan *acpResponse]

	// Streaming channel for session/update notifications. streamMu is a
	// scalar-mutex survivor (not Pattern-A — guards a chan, not a
	// map/slice) so no migration is required.
	streamCh chan string
	streamMu sync.RWMutex
}

// ACP Protocol Version
const acpProtocolVersion = 1

// ACP Message Types
type acpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type acpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *acpError       `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"` // For notifications
	Params  json.RawMessage `json:"params,omitempty"` // For notifications
}

type acpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ACP Request/Response Types
type initializeRequest struct {
	ClientCapabilities clientCapabilities `json:"clientCapabilities"`
}

type clientCapabilities struct {
	FileSystem bool `json:"fileSystem"`
}

type initializeResponse struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentInfo         agentInfo         `json:"agentInfo"`
	AgentCapabilities agentCapabilities `json:"agentCapabilities"`
	AuthMethods       []authMethod      `json:"authMethods"`
}

type agentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type agentCapabilities struct {
	LoadSession        bool               `json:"loadSession"`
	PromptCapabilities promptCapabilities `json:"promptCapabilities"`
}

type promptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type authMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type newSessionRequest struct {
	CWD string `json:"cwd"`
}

type newSessionResponse struct {
	SessionID string `json:"sessionId"`
}

type promptRequest struct {
	SessionID string         `json:"sessionId"`
	Prompt    []contentBlock `json:"prompt"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type promptResponse struct {
	StopReason string         `json:"stopReason"`
	Result     []contentBlock `json:"result"`
}

// QwenACPConfig holds configuration for the ACP provider
type QwenACPConfig struct {
	Model     string
	Timeout   time.Duration
	MaxTokens int
	CWD       string
}

// DefaultQwenACPConfig returns default configuration
func DefaultQwenACPConfig() QwenACPConfig {
	return QwenACPConfig{
		Model:     "qwen-turbo",
		Timeout:   180 * time.Second,
		MaxTokens: 8192,
		CWD:       ".",
	}
}

// NewQwenACPProvider creates a new Qwen ACP provider
func NewQwenACPProvider(config QwenACPConfig) *QwenACPProvider {
	if config.Timeout == 0 {
		config.Timeout = 180 * time.Second
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 8192
	}
	if config.CWD == "" {
		config.CWD = "."
	}

	return &QwenACPProvider{
		model:     config.Model,
		timeout:   config.Timeout,
		maxTokens: config.MaxTokens,
		cwd:       config.CWD,
		responses: safe.NewStore[int64, chan *acpResponse](),
	}
}

// NewQwenACPProviderWithModel creates an ACP provider with a specific model
func NewQwenACPProviderWithModel(model string) *QwenACPProvider {
	config := DefaultQwenACPConfig()
	config.Model = model
	return NewQwenACPProvider(config)
}

// Start starts the Qwen ACP process
func (p *QwenACPProvider) Start() error {
	p.startOnce.Do(func() {
		p.startErr = p.startProcess()
	})
	return p.startErr
}

func (p *QwenACPProvider) startProcess() error {
	// CONCURRENCY FIX: Only hold mutex during process setup, release before calling
	// methods that also need the mutex (initializeWithContext, createSessionWithContext)
	p.mu.Lock()

	// Check if qwen command exists
	qwenPath, err := exec.LookPath("qwen")
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("qwen command not found: %w", err)
	}

	// Start qwen in ACP mode
	p.cmd = exec.Command(qwenPath, "--acp")
	p.cmd.Dir = p.cwd

	// Set up pipes
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := p.cmd.Start(); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to start qwen --acp: %w", err)
	}

	p.scanner = bufio.NewScanner(p.stdout)
	p.scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	p.isRunning = true

	// Start response reader goroutine
	go p.readResponses()

	// Release mutex before calling methods that need it
	p.mu.Unlock()

	// Initialize the ACP connection with timeout
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()

	// Wait for readResponses to start before initializing
	time.Sleep(100 * time.Millisecond)

	// Initialize with context (this calls sendRequest which needs the mutex)
	if err := p.initializeWithContext(initCtx); err != nil {
		p.Stop()
		return fmt.Errorf("failed to initialize ACP: %w", err)
	}

	// Create a new session with context (this also calls sendRequest)
	if err := p.createSessionWithContext(initCtx); err != nil {
		p.Stop()
		return fmt.Errorf("failed to create session: %w", err)
	}

	p.mu.Lock()
	p.initialized = true
	p.mu.Unlock()
	return nil
}

// readResponses reads responses from stdout in a goroutine
func (p *QwenACPProvider) readResponses() {
	for p.scanner.Scan() {
		line := p.scanner.Text()
		if line == "" {
			continue
		}

		var resp acpResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}

		// Handle notifications (no ID)
		if resp.Method != "" {
			p.handleNotification(&resp)
			continue
		}

		// Handle responses (has ID)
		ch, ok := p.responses.Get(resp.ID)
		if ok {
			select {
			case ch <- &resp:
			default:
			}
		}
	}

	p.mu.Lock()
	p.isRunning = false
	p.mu.Unlock()
}

// sessionUpdateParams represents the params of a session/update notification.
type sessionUpdateParams struct {
	Content []contentBlock `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
}

// handleNotification handles ACP notifications
func (p *QwenACPProvider) handleNotification(resp *acpResponse) {
	if resp.Method == "session/update" {
		p.streamMu.RLock()
		ch := p.streamCh
		p.streamMu.RUnlock()

		if ch == nil {
			return
		}

		// Parse the notification params to extract streaming text
		var params sessionUpdateParams
		if resp.Params != nil {
			if err := json.Unmarshal(resp.Params, &params); err != nil {
				log.Printf("[QwenACP] failed to parse session/update params: %v", err)
				return
			}
		}

		// Extract text from content blocks or direct text field
		text := params.Text
		if text == "" {
			for _, block := range params.Content {
				if block.Type == "text" && block.Text != "" {
					text += block.Text
				}
			}
		}

		if text != "" {
			select {
			case ch <- text:
			default:
				// Channel full, drop token to avoid blocking the reader goroutine
			}
		}
	}
}

// sendRequest sends an ACP request and waits for response
func (p *QwenACPProvider) sendRequest(ctx context.Context, method string, params interface{}) (*acpResponse, error) {
	// CONCURRENCY FIX: Check isRunning under mutex to avoid race with readResponses
	p.mu.Lock()
	running := p.isRunning
	p.mu.Unlock()
	if !running {
		return nil, fmt.Errorf("ACP process not running")
	}

	id := atomic.AddInt64(&p.requestID, 1)

	req := acpRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Create response channel
	respCh := make(chan *acpResponse, 1)
	p.responses.Put(id, respCh)
	defer func() {
		p.responses.Delete(id)
	}()

	// Send request
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	p.mu.Lock()
	_, err = fmt.Fprintf(p.stdin, "%s\n", reqBytes)
	p.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("ACP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// initializeWithContext sends the initialize request with provided context
func (p *QwenACPProvider) initializeWithContext(ctx context.Context) error {
	// Try with empty params first (some ACP versions don't expect params)
	resp, err := p.sendRequest(ctx, "initialize", map[string]interface{}{})
	if err != nil {
		// If empty params fail, try with client capabilities
		params := initializeRequest{
			ClientCapabilities: clientCapabilities{
				FileSystem: false, // We don't need filesystem access
			},
		}
		resp, err = p.sendRequest(ctx, "initialize", params)
		if err != nil {
			return fmt.Errorf("ACP initialization failed (tried both empty and full params): %w", err)
		}
	}

	var initResp initializeResponse
	if err := json.Unmarshal(resp.Result, &initResp); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}

	if initResp.ProtocolVersion != acpProtocolVersion {
		return fmt.Errorf("unsupported protocol version: %d", initResp.ProtocolVersion)
	}

	return nil
}

// createSessionWithContext creates a new ACP session with provided context
func (p *QwenACPProvider) createSessionWithContext(ctx context.Context) error {
	// Try with CWD param first
	params := newSessionRequest{
		CWD: p.cwd,
	}

	resp, err := p.sendRequest(ctx, "session/new", params)
	if err != nil {
		// If CWD param fails, try with empty params
		resp, err = p.sendRequest(ctx, "session/new", map[string]interface{}{})
		if err != nil {
			return fmt.Errorf("session creation failed (tried both with and without CWD): %w", err)
		}
	}

	var sessionResp newSessionResponse
	if err := json.Unmarshal(resp.Result, &sessionResp); err != nil {
		return fmt.Errorf("failed to parse session response: %w", err)
	}

	p.sessionID = sessionResp.SessionID
	return nil
}

// Stop stops the ACP process
func (p *QwenACPProvider) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.stdin.Close()
		_ = p.cmd.Process.Kill() //nolint:errcheck
		_ = p.cmd.Wait()         //nolint:errcheck
	}
	p.isRunning = false
	p.initialized = false
}

// IsAvailable checks if ACP is available
func (p *QwenACPProvider) IsAvailable() bool {
	_, err := exec.LookPath("qwen")
	return err == nil
}

// Complete implements the LLMProvider interface
func (p *QwenACPProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// Ensure ACP is started
	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ACP: %w", err)
	}

	// Build prompt from messages
	var promptBuilder strings.Builder
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			promptBuilder.WriteString("System: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
		case "user":
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n")
		case "assistant":
			promptBuilder.WriteString("Assistant: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
		}
	}

	prompt := promptBuilder.String()
	if prompt == "" && req.Prompt != "" {
		prompt = req.Prompt
	}

	if prompt == "" {
		return nil, fmt.Errorf("no prompt provided")
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Send prompt request
	params := promptRequest{
		SessionID: p.sessionID,
		Prompt: []contentBlock{
			{Type: "text", Text: prompt},
		},
	}

	startTime := time.Now()
	resp, err := p.sendRequest(timeoutCtx, "session/prompt", params)
	duration := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("prompt request failed: %w", err)
	}

	var promptResp promptResponse
	if err := json.Unmarshal(resp.Result, &promptResp); err != nil {
		return nil, fmt.Errorf("failed to parse prompt response: %w", err)
	}

	// Extract text from result
	var content strings.Builder
	for _, block := range promptResp.Result {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	// Estimate token count
	promptTokens := len(prompt) / 4
	completionTokens := content.Len() / 4

	return &models.LLMResponse{
		ID:           fmt.Sprintf("qwen-acp-%d", time.Now().UnixNano()),
		ProviderID:   "qwen-acp",
		ProviderName: "qwen-acp",
		Content:      content.String(),
		FinishReason: promptResp.StopReason,
		TokensUsed:   promptTokens + completionTokens,
		ResponseTime: duration.Milliseconds(),
		CreatedAt:    time.Now(),
		Metadata: map[string]interface{}{
			"source":            "qwen-acp",
			"session_id":        p.sessionID,
			"model":             p.model,
			"stop_reason":       promptResp.StopReason,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"latency":           duration.String(),
		},
	}, nil
}

// CompleteStream implements streaming completion using ACP session/update notifications.
func (p *QwenACPProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ACP: %w", err)
	}

	// Set up the streaming channel to receive session/update notifications
	streamCh := make(chan string, 64)
	p.streamMu.Lock()
	p.streamCh = streamCh
	p.streamMu.Unlock()

	outCh := make(chan *models.LLMResponse, 64)

	go func() {
		defer close(outCh)
		defer func() {
			p.streamMu.Lock()
			p.streamCh = nil
			p.streamMu.Unlock()
		}()

		// Build prompt (same logic as Complete)
		var promptBuilder strings.Builder
		for _, msg := range req.Messages {
			switch msg.Role {
			case "system":
				promptBuilder.WriteString("System: ")
				promptBuilder.WriteString(msg.Content)
				promptBuilder.WriteString("\n\n")
			case "user":
				promptBuilder.WriteString(msg.Content)
				promptBuilder.WriteString("\n")
			case "assistant":
				promptBuilder.WriteString("Assistant: ")
				promptBuilder.WriteString(msg.Content)
				promptBuilder.WriteString("\n\n")
			}
		}
		prompt := promptBuilder.String()
		if prompt == "" && req.Prompt != "" {
			prompt = req.Prompt
		}
		if prompt == "" {
			outCh <- &models.LLMResponse{
				ProviderID:   "qwen-acp",
				ProviderName: "qwen-acp",
				Metadata:     map[string]interface{}{"error": "no prompt provided"},
			}
			return
		}

		startTime := time.Now()

		// Send prompt request (the final response comes back via sendRequest,
		// but streaming tokens arrive as session/update notifications)
		params := promptRequest{
			SessionID: p.sessionID,
			Prompt:    []contentBlock{{Type: "text", Text: prompt}},
		}

		// Launch the request in a goroutine so we can simultaneously
		// consume streaming tokens from streamCh
		type completeResult struct {
			resp *acpResponse
			err  error
		}
		doneCh := make(chan completeResult, 1)
		go func() {
			timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
			defer cancel()
			resp, err := p.sendRequest(timeoutCtx, "session/prompt", params)
			select {
			case doneCh <- completeResult{resp, err}:
			case <-ctx.Done():
				// Outer goroutine already exited due to context cancellation;
				// drain the buffered channel to avoid a phantom write and return.
			}
		}()

		// Forward streaming tokens until the final response arrives
		for {
			select {
			case <-ctx.Done():
				// Drain doneCh so the inner goroutine is not blocked on a send.
				select {
				case <-doneCh:
				default:
				}
				return
			case token, ok := <-streamCh:
				if !ok {
					return
				}
				outCh <- &models.LLMResponse{
					ID:           fmt.Sprintf("qwen-acp-stream-%d", time.Now().UnixNano()),
					ProviderID:   "qwen-acp",
					ProviderName: "qwen-acp",
					Content:      token,
					Metadata: map[string]interface{}{
						"source":     "qwen-acp",
						"session_id": p.sessionID,
						"model":      p.model,
						"streaming":  true,
					},
				}
			case result := <-doneCh:
				// Drain any remaining tokens
				for {
					select {
					case token, ok := <-streamCh:
						if !ok {
							goto done
						}
						outCh <- &models.LLMResponse{
							ID:           fmt.Sprintf("qwen-acp-stream-%d", time.Now().UnixNano()),
							ProviderID:   "qwen-acp",
							ProviderName: "qwen-acp",
							Content:      token,
							Metadata:     map[string]interface{}{"streaming": true},
						}
					default:
						goto done
					}
				}
			done:
				duration := time.Since(startTime)
				if result.err != nil {
					outCh <- &models.LLMResponse{
						ProviderID:   "qwen-acp",
						ProviderName: "qwen-acp",
						Metadata:     map[string]interface{}{"error": result.err.Error()},
					}
					return
				}

				// Parse the final response for metadata
				var promptResp promptResponse
				if result.resp != nil && result.resp.Result != nil {
					if err := json.Unmarshal(result.resp.Result, &promptResp); err == nil {
						var content strings.Builder
						for _, block := range promptResp.Result {
							if block.Type == "text" {
								content.WriteString(block.Text)
							}
						}
						promptTokens := len(prompt) / 4
						completionTokens := content.Len() / 4
						// Send final chunk with complete metadata
						outCh <- &models.LLMResponse{
							ID:           fmt.Sprintf("qwen-acp-%d", time.Now().UnixNano()),
							ProviderID:   "qwen-acp",
							ProviderName: "qwen-acp",
							Content:      content.String(),
							FinishReason: promptResp.StopReason,
							TokensUsed:   promptTokens + completionTokens,
							ResponseTime: duration.Milliseconds(),
							CreatedAt:    time.Now(),
							Metadata: map[string]interface{}{
								"source":            "qwen-acp",
								"session_id":        p.sessionID,
								"model":             p.model,
								"stop_reason":       promptResp.StopReason,
								"prompt_tokens":     promptTokens,
								"completion_tokens": completionTokens,
								"latency":           duration.String(),
								"streaming":         false,
							},
						}
					}
				}
				return
			}
		}
	}()

	return outCh, nil
}

// HealthCheck checks if the ACP process is healthy
func (p *QwenACPProvider) HealthCheck() error {
	if !p.IsAvailable() {
		return fmt.Errorf("qwen command not available")
	}

	if err := p.Start(); err != nil {
		return fmt.Errorf("failed to start ACP: %w", err)
	}

	// CONCURRENCY FIX: Check isRunning under mutex
	p.mu.Lock()
	running := p.isRunning
	p.mu.Unlock()
	if !running {
		return fmt.Errorf("ACP process not running")
	}

	return nil
}

// GetCapabilities returns the capabilities of the ACP provider
func (p *QwenACPProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"qwen-plus",
			"qwen-turbo",
			"qwen-max",
			"qwen2.5-72b-instruct",
			"qwen2.5-32b-instruct",
			"qwen2.5-coder-32b-instruct",
		},
		SupportsStreaming: true,
		SupportsTools:     false, // ACP tool support could be added
		Limits: models.ModelLimits{
			MaxTokens:             p.maxTokens,
			MaxConcurrentRequests: 1, // Single session for now
		},
	}
}

// ValidateConfig validates the configuration
func (p *QwenACPProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if !p.IsAvailable() {
		return false, []string{"qwen command not available"}
	}
	return true, nil
}

// GetName returns the provider name
func (p *QwenACPProvider) GetName() string {
	return "qwen-acp"
}

// GetProviderType returns the provider type
func (p *QwenACPProvider) GetProviderType() string {
	return "qwen"
}

// GetCurrentModel returns the current model
func (p *QwenACPProvider) GetCurrentModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *QwenACPProvider) SetModel(model string) {
	p.model = model
}

// IsQwenACPAvailable checks if Qwen ACP is available
func IsQwenACPAvailable() bool {
	_, err := exec.LookPath("qwen")
	return err == nil
}

// CanUseQwenACP returns true if Qwen ACP can be used
// This checks: 1) qwen CLI installed, 2) qwen CLI authenticated, 3) ACP mode works
func CanUseQwenACP() bool {
	if !IsQwenCodeInstalled() {
		return false
	}
	if !IsQwenCodeAuthenticated() {
		return false
	}
	// Quick test if ACP mode is supported
	return testQwenACPAvailability()
}

// testQwenACPAvailability performs a quick test to see if qwen --acp is supported
func testQwenACPAvailability() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	qwenPath, err := exec.LookPath("qwen")
	if err != nil {
		return false
	}

	// Try to start qwen --acp and see if it responds
	cmd := exec.CommandContext(ctx, qwenPath, "--acp")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false
	}

	if err := cmd.Start(); err != nil {
		return false
	}

	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill() //nolint:errcheck
		}
		_ = cmd.Wait() //nolint:errcheck
	}()

	// Try to send a simple request and see if we get a response
	// Just check if the process accepts input without hanging
	testReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientCapabilities":{"fileSystem":false}}}`

	done := make(chan bool, 1)
	go func() {
		_, err := fmt.Fprintf(stdin, "%s\n", testReq)
		if err != nil {
			done <- false
			return
		}

		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			done <- true
		} else {
			done <- false
		}
	}()

	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		return false
	case <-time.After(3 * time.Second):
		return false
	}
}
