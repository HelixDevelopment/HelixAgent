package junie

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/models"
)

// JunieACPProvider implements the LLMProvider interface using Junie's ACP
// (Agent Communication Protocol) over stdin/stdout.
//
// This provides a more powerful integration than CLI:
// - Session management (conversation history)
// - Streaming responses
// - Tool support
// - Full IDE-like integration
type JunieACPProvider struct {
	model       string
	timeout     time.Duration
	maxTokens   int
	cwd         string
	initialized bool
	apiKey      string

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

	// responses maps pending request IDs to their reply channels. The
	// store is lock-free for the reader goroutine's dispatch path
	// (readResponses) and the sender's register/unregister pair
	// (sendRequest).
	responses *safe.Store[int64, chan *junieACPResponse]
}

// ACP message types for Junie
type junieACPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type junieACPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *junieACPError  `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type junieACPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ACP Request/Response Types
type junieInitializeRequest struct {
	ClientCapabilities junieClientCapabilities `json:"clientCapabilities"`
}

type junieClientCapabilities struct {
	FileSystem bool `json:"fileSystem"`
}

type junieInitializeResponse struct {
	ProtocolVersion   int                    `json:"protocolVersion"`
	AgentInfo         junieAgentInfo         `json:"agentInfo"`
	AgentCapabilities junieAgentCapabilities `json:"agentCapabilities"`
	AuthMethods       []junieAuthMethod      `json:"authMethods"`
}

type junieAgentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type junieAgentCapabilities struct {
	LoadSession        bool                    `json:"loadSession"`
	PromptCapabilities juniePromptCapabilities `json:"promptCapabilities"`
}

type juniePromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type junieAuthMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type junieNewSessionRequest struct {
	CWD string `json:"cwd"`
}

type junieNewSessionResponse struct {
	SessionID string `json:"sessionId"`
}

type juniePromptRequest struct {
	SessionID string              `json:"sessionId"`
	Prompt    []junieContentBlock `json:"prompt"`
}

type junieContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type juniePromptResponse struct {
	StopReason string              `json:"stopReason"`
	Result     []junieContentBlock `json:"result"`
}

// JunieACPConfig holds configuration for the ACP provider
type JunieACPConfig struct {
	Model     string
	Timeout   time.Duration
	MaxTokens int
	CWD       string
	APIKey    string
}

// DefaultJunieACPConfig returns default configuration
func DefaultJunieACPConfig() JunieACPConfig {
	return JunieACPConfig{
		Model:     "sonnet",
		Timeout:   180 * time.Second,
		MaxTokens: 8192,
		CWD:       ".",
		APIKey:    os.Getenv("JUNIE_API_KEY"),
	}
}

// NewJunieACPProvider creates a new Junie ACP provider
func NewJunieACPProvider(config JunieACPConfig) *JunieACPProvider {
	if config.Timeout == 0 {
		config.Timeout = 180 * time.Second
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 8192
	}
	if config.CWD == "" {
		config.CWD = "."
	}
	if config.APIKey == "" {
		config.APIKey = os.Getenv("JUNIE_API_KEY")
	}

	return &JunieACPProvider{
		model:     config.Model,
		timeout:   config.Timeout,
		maxTokens: config.MaxTokens,
		cwd:       config.CWD,
		apiKey:    config.APIKey,
		responses: safe.NewStore[int64, chan *junieACPResponse](),
	}
}

// NewJunieACPProviderWithModel creates an ACP provider with a specific model
func NewJunieACPProviderWithModel(model string) *JunieACPProvider {
	config := DefaultJunieACPConfig()
	config.Model = model
	return NewJunieACPProvider(config)
}

// Start starts the Junie ACP process
func (p *JunieACPProvider) Start() error {
	p.startOnce.Do(func() {
		p.startErr = p.startProcess()
	})
	return p.startErr
}

func (p *JunieACPProvider) startProcess() error {
	p.mu.Lock()

	juniePath, err := exec.LookPath("junie")
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("junie command not found: %w", err)
	}

	p.cmd = exec.Command(juniePath, "--acp", "true")
	p.cmd.Dir = p.cwd

	if p.apiKey != "" {
		p.cmd.Env = append(os.Environ(), "JUNIE_API_KEY="+p.apiKey)
	}

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

	if err := p.cmd.Start(); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to start junie --acp: %w", err)
	}

	p.scanner = bufio.NewScanner(p.stdout)
	p.scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	p.isRunning = true

	go p.readResponses()

	p.mu.Unlock()

	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()

	time.Sleep(100 * time.Millisecond)

	if err := p.initializeWithContext(initCtx); err != nil {
		p.Stop()
		return fmt.Errorf("failed to initialize ACP: %w", err)
	}

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
func (p *JunieACPProvider) readResponses() {
	for p.scanner.Scan() {
		line := p.scanner.Text()
		if line == "" {
			continue
		}

		var resp junieACPResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}

		if resp.Method != "" {
			p.handleNotification(&resp)
			continue
		}

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

// handleNotification handles ACP notifications
func (p *JunieACPProvider) handleNotification(resp *junieACPResponse) {
	// Handle session/update notifications for streaming
	if resp.Method == "session/update" {
		// Parse notification and handle streaming updates
		// This could be used for real-time streaming in the future
		// For now, just log the notification
		log.Printf("[JunieACP] session/update notification: %v", resp)
	}
}

// sendRequest sends an ACP request and waits for response
func (p *JunieACPProvider) sendRequest(ctx context.Context, method string, params interface{}) (*junieACPResponse, error) {
	p.mu.Lock()
	running := p.isRunning
	p.mu.Unlock()
	if !running {
		return nil, fmt.Errorf("ACP process not running")
	}

	id := atomic.AddInt64(&p.requestID, 1)

	req := junieACPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	respCh := make(chan *junieACPResponse, 1)
	p.responses.Put(id, respCh)

	defer func() {
		p.responses.Delete(id)
	}()

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

// initializeWithContext sends the initialize request
func (p *JunieACPProvider) initializeWithContext(ctx context.Context) error {
	resp, err := p.sendRequest(ctx, "initialize", map[string]interface{}{})
	if err != nil {
		params := junieInitializeRequest{
			ClientCapabilities: junieClientCapabilities{
				FileSystem: false,
			},
		}
		resp, err = p.sendRequest(ctx, "initialize", params)
		if err != nil {
			return fmt.Errorf("ACP initialization failed: %w", err)
		}
	}

	var initResp junieInitializeResponse
	if err := json.Unmarshal(resp.Result, &initResp); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}

	return nil
}

// createSessionWithContext creates a new ACP session
func (p *JunieACPProvider) createSessionWithContext(ctx context.Context) error {
	params := junieNewSessionRequest{
		CWD: p.cwd,
	}

	resp, err := p.sendRequest(ctx, "session/new", params)
	if err != nil {
		resp, err = p.sendRequest(ctx, "session/new", map[string]interface{}{})
		if err != nil {
			return fmt.Errorf("session creation failed: %w", err)
		}
	}

	var sessionResp junieNewSessionResponse
	if err := json.Unmarshal(resp.Result, &sessionResp); err != nil {
		return fmt.Errorf("failed to parse session response: %w", err)
	}

	p.sessionID = sessionResp.SessionID
	return nil
}

// Stop stops the ACP process
func (p *JunieACPProvider) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.stdin.Close()      //nolint:errcheck
		_ = p.cmd.Process.Kill() //nolint:errcheck
		_ = p.cmd.Wait()         //nolint:errcheck
	}
	p.isRunning = false
	p.initialized = false
}

// IsAvailable checks if ACP is available
func (p *JunieACPProvider) IsAvailable() bool {
	_, err := exec.LookPath("junie")
	return err == nil
}

// Complete implements the LLMProvider interface
func (p *JunieACPProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ACP: %w", err)
	}

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

	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	params := juniePromptRequest{
		SessionID: p.sessionID,
		Prompt: []junieContentBlock{
			{Type: "text", Text: prompt},
		},
	}

	startTime := time.Now()
	resp, err := p.sendRequest(timeoutCtx, "session/prompt", params)
	duration := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("prompt request failed: %w", err)
	}

	var promptResp juniePromptResponse
	if err := json.Unmarshal(resp.Result, &promptResp); err != nil {
		return nil, fmt.Errorf("failed to parse prompt response: %w", err)
	}

	var content strings.Builder
	for _, block := range promptResp.Result {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	promptTokens := len(prompt) / 4
	completionTokens := content.Len() / 4

	return &models.LLMResponse{
		ID:           fmt.Sprintf("junie-acp-%d", time.Now().UnixNano()),
		ProviderID:   "junie-acp",
		ProviderName: "junie-acp",
		Content:      content.String(),
		FinishReason: promptResp.StopReason,
		TokensUsed:   promptTokens + completionTokens,
		ResponseTime: duration.Milliseconds(),
		CreatedAt:    time.Now(),
		Metadata: map[string]interface{}{
			"source":            "junie-acp",
			"session_id":        p.sessionID,
			"model":             p.model,
			"stop_reason":       promptResp.StopReason,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"latency":           duration.String(),
		},
	}, nil
}

// CompleteStream implements streaming completion using ACP
func (p *JunieACPProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)

	go func() {
		defer close(ch)

		resp, err := p.Complete(ctx, req)
		if err != nil {
			ch <- &models.LLMResponse{
				ProviderID:   "junie-acp",
				ProviderName: "junie-acp",
				Metadata: map[string]interface{}{
					"error": err.Error(),
				},
			}
			return
		}
		ch <- resp
	}()

	return ch, nil
}

// HealthCheck checks if the ACP process is healthy
func (p *JunieACPProvider) HealthCheck() error {
	if !p.IsAvailable() {
		return fmt.Errorf("junie command not available")
	}

	if err := p.Start(); err != nil {
		return fmt.Errorf("failed to start ACP: %w", err)
	}

	p.mu.Lock()
	running := p.isRunning
	p.mu.Unlock()
	if !running {
		return fmt.Errorf("ACP process not running")
	}

	return nil
}

// GetCapabilities returns the capabilities of the ACP provider
func (p *JunieACPProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:   knownJunieModels,
		SupportsStreaming: true,
		SupportsTools:     true,
		Limits: models.ModelLimits{
			MaxTokens:             p.maxTokens,
			MaxConcurrentRequests: 1,
		},
	}
}

// ValidateConfig validates the configuration
func (p *JunieACPProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if !p.IsAvailable() {
		return false, []string{"junie command not available"}
	}
	return true, nil
}

// GetName returns the provider name
func (p *JunieACPProvider) GetName() string {
	return "junie-acp"
}

// GetProviderType returns the provider type
func (p *JunieACPProvider) GetProviderType() string {
	return "junie"
}

// GetCurrentModel returns the current model
func (p *JunieACPProvider) GetCurrentModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *JunieACPProvider) SetModel(model string) {
	p.model = model
}

// IsJunieACPAvailable checks if Junie ACP is available
func IsJunieACPAvailable() bool {
	_, err := exec.LookPath("junie")
	return err == nil
}

// CanUseJunieACP returns true if Junie ACP can be used
func CanUseJunieACP() bool {
	if !IsJunieInstalled() {
		return false
	}
	if os.Getenv("JUNIE_API_KEY") == "" && !IsJunieAuthenticated() {
		return false
	}
	return testJunieACPAvailability()
}

// testJunieACPAvailability performs a quick test to see if junie --acp is supported
func testJunieACPAvailability() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	juniePath, err := exec.LookPath("junie")
	if err != nil {
		return false
	}

	cmd := exec.CommandContext(ctx, juniePath, "--acp", "true")

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
