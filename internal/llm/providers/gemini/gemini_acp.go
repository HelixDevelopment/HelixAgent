package gemini

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

// GeminiACPProvider implements the LLMProvider interface using Gemini's ACP
// (Agent Communication Protocol) over stdin/stdout.
//
// This provides a more powerful integration than CLI:
// - Session management (conversation history)
// - Streaming responses
// - Tool support
// - Full IDE-like integration
type GeminiACPProvider struct {
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
	responses *safe.Store[int64, chan *geminiACPResponse]
}

// ACP message types for Gemini
type geminiACPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type geminiACPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *geminiACPError `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type geminiACPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ACP Request/Response Types
type geminiInitializeRequest struct {
	ClientCapabilities geminiClientCapabilities `json:"clientCapabilities"`
}

type geminiClientCapabilities struct {
	FileSystem bool `json:"fileSystem"`
}

type geminiInitializeResponse struct {
	ProtocolVersion   int                     `json:"protocolVersion"`
	AgentInfo         geminiAgentInfo         `json:"agentInfo"`
	AgentCapabilities geminiAgentCapabilities `json:"agentCapabilities"`
	AuthMethods       []geminiAuthMethod      `json:"authMethods"`
}

type geminiAgentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type geminiAgentCapabilities struct {
	LoadSession        bool                     `json:"loadSession"`
	PromptCapabilities geminiPromptCapabilities `json:"promptCapabilities"`
}

type geminiPromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type geminiAuthMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type geminiNewSessionRequest struct {
	CWD string `json:"cwd"`
}

type geminiNewSessionResponse struct {
	SessionID string `json:"sessionId"`
}

type geminiPromptRequest struct {
	SessionID string               `json:"sessionId"`
	Prompt    []geminiContentBlock `json:"prompt"`
}

type geminiContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type geminiPromptResponse struct {
	StopReason string               `json:"stopReason"`
	Result     []geminiContentBlock `json:"result"`
}

// GeminiACPConfig holds configuration for the ACP provider
type GeminiACPConfig struct {
	Model     string
	Timeout   time.Duration
	MaxTokens int
	CWD       string
	APIKey    string
}

// Known Gemini models available via CLI
var knownGeminiACPModels = []string{
	"gemini-2.0-flash",
	"gemini-2.5-flash",
	"gemini-2.5-pro",
	"gemini-3-flash-preview",
	"gemini-3-pro-preview",
}

// DefaultGeminiACPConfig returns default configuration
func DefaultGeminiACPConfig() GeminiACPConfig {
	return GeminiACPConfig{
		Model:     "gemini-2.0-flash",
		Timeout:   180 * time.Second,
		MaxTokens: 8192,
		CWD:       ".",
		APIKey:    os.Getenv("GEMINI_API_KEY"),
	}
}

// NewGeminiACPProvider creates a new Gemini ACP provider
func NewGeminiACPProvider(config GeminiACPConfig) *GeminiACPProvider {
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
		config.APIKey = os.Getenv("GEMINI_API_KEY")
	}

	return &GeminiACPProvider{
		model:     config.Model,
		timeout:   config.Timeout,
		maxTokens: config.MaxTokens,
		cwd:       config.CWD,
		apiKey:    config.APIKey,
		responses: safe.NewStore[int64, chan *geminiACPResponse](),
	}
}

// NewGeminiACPProviderWithModel creates an ACP provider with a specific model
func NewGeminiACPProviderWithModel(model string) *GeminiACPProvider {
	config := DefaultGeminiACPConfig()
	config.Model = model
	return NewGeminiACPProvider(config)
}

// Start starts the Gemini ACP process
func (p *GeminiACPProvider) Start() error {
	p.startOnce.Do(func() {
		p.startErr = p.startProcess()
	})
	return p.startErr
}

func (p *GeminiACPProvider) startProcess() error {
	p.mu.Lock()

	geminiPath, err := exec.LookPath("gemini")
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("gemini command not found: %w", err)
	}

	p.cmd = exec.Command(geminiPath, "--experimental-acp")
	p.cmd.Dir = p.cwd

	if p.apiKey != "" {
		p.cmd.Env = append(os.Environ(), "GEMINI_API_KEY="+p.apiKey)
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
		return fmt.Errorf("failed to start gemini --experimental-acp: %w", err)
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
func (p *GeminiACPProvider) readResponses() {
	for p.scanner.Scan() {
		line := p.scanner.Text()
		if line == "" {
			continue
		}

		var resp geminiACPResponse
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
func (p *GeminiACPProvider) handleNotification(resp *geminiACPResponse) {
	// Handle session/update notifications for streaming
	if resp.Method == "session/update" {
		// Parse notification and handle streaming updates
		// This could be used for real-time streaming in the future
		// For now, just log the notification
		log.Printf("[GeminiACP] session/update notification: %v", resp)
	}
}

// sendRequest sends an ACP request and waits for response
func (p *GeminiACPProvider) sendRequest(ctx context.Context, method string, params interface{}) (*geminiACPResponse, error) {
	p.mu.Lock()
	running := p.isRunning
	p.mu.Unlock()
	if !running {
		return nil, fmt.Errorf("ACP process not running")
	}

	id := atomic.AddInt64(&p.requestID, 1)

	req := geminiACPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	respCh := make(chan *geminiACPResponse, 1)
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
func (p *GeminiACPProvider) initializeWithContext(ctx context.Context) error {
	resp, err := p.sendRequest(ctx, "initialize", map[string]interface{}{})
	if err != nil {
		params := geminiInitializeRequest{
			ClientCapabilities: geminiClientCapabilities{
				FileSystem: false,
			},
		}
		resp, err = p.sendRequest(ctx, "initialize", params)
		if err != nil {
			return fmt.Errorf("ACP initialization failed: %w", err)
		}
	}

	var initResp geminiInitializeResponse
	if err := json.Unmarshal(resp.Result, &initResp); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}

	return nil
}

// createSessionWithContext creates a new ACP session
func (p *GeminiACPProvider) createSessionWithContext(ctx context.Context) error {
	params := geminiNewSessionRequest{
		CWD: p.cwd,
	}

	resp, err := p.sendRequest(ctx, "session/new", params)
	if err != nil {
		resp, err = p.sendRequest(ctx, "session/new", map[string]interface{}{})
		if err != nil {
			return fmt.Errorf("session creation failed: %w", err)
		}
	}

	var sessionResp geminiNewSessionResponse
	if err := json.Unmarshal(resp.Result, &sessionResp); err != nil {
		return fmt.Errorf("failed to parse session response: %w", err)
	}

	p.sessionID = sessionResp.SessionID
	return nil
}

// Stop stops the ACP process
func (p *GeminiACPProvider) Stop() {
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
func (p *GeminiACPProvider) IsAvailable() bool {
	_, err := exec.LookPath("gemini")
	return err == nil
}

// Complete implements the LLMProvider interface
func (p *GeminiACPProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
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

	params := geminiPromptRequest{
		SessionID: p.sessionID,
		Prompt: []geminiContentBlock{
			{Type: "text", Text: prompt},
		},
	}

	startTime := time.Now()
	resp, err := p.sendRequest(timeoutCtx, "session/prompt", params)
	duration := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("prompt request failed: %w", err)
	}

	var promptResp geminiPromptResponse
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
		ID:           fmt.Sprintf("gemini-acp-%d", time.Now().UnixNano()),
		ProviderID:   "gemini-acp",
		ProviderName: "gemini-acp",
		Content:      content.String(),
		FinishReason: promptResp.StopReason,
		TokensUsed:   promptTokens + completionTokens,
		ResponseTime: duration.Milliseconds(),
		CreatedAt:    time.Now(),
		Metadata: map[string]interface{}{
			"source":            "gemini-acp",
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
func (p *GeminiACPProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)

	go func() {
		defer close(ch)

		resp, err := p.Complete(ctx, req)
		if err != nil {
			ch <- &models.LLMResponse{
				ProviderID:   "gemini-acp",
				ProviderName: "gemini-acp",
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
func (p *GeminiACPProvider) HealthCheck() error {
	if !p.IsAvailable() {
		return fmt.Errorf("gemini command not available")
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
func (p *GeminiACPProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:   knownGeminiACPModels,
		SupportsStreaming: true,
		SupportsTools:     true,
		Limits: models.ModelLimits{
			MaxTokens:             p.maxTokens,
			MaxConcurrentRequests: 1,
		},
	}
}

// ValidateConfig validates the configuration
func (p *GeminiACPProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if !p.IsAvailable() {
		return false, []string{"gemini command not available"}
	}
	return true, nil
}

// GetName returns the provider name
func (p *GeminiACPProvider) GetName() string {
	return "gemini-acp"
}

// GetProviderType returns the provider type
func (p *GeminiACPProvider) GetProviderType() string {
	return "gemini"
}

// GetCurrentModel returns the current model
func (p *GeminiACPProvider) GetCurrentModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *GeminiACPProvider) SetModel(model string) {
	p.model = model
}

// IsGeminiACPAvailable checks if Gemini ACP is available
func IsGeminiACPAvailable() bool {
	_, err := exec.LookPath("gemini")
	return err == nil
}

// CanUseGeminiACP returns true if Gemini ACP can be used
func CanUseGeminiACP() bool {
	if !IsGeminiCLIInstalled() {
		return false
	}
	if os.Getenv("GEMINI_API_KEY") == "" && !IsGeminiCLIAuthenticated() {
		return false
	}
	return testGeminiACPAvailability()
}

// testGeminiACPAvailability performs a quick test to see if gemini --experimental-acp is supported
func testGeminiACPAvailability() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	geminiPath, err := exec.LookPath("gemini")
	if err != nil {
		return false
	}

	cmd := exec.CommandContext(ctx, geminiPath, "--experimental-acp")

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
		_ = stdin.Close() //nolint:errcheck
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
			select {
			case done <- false:
			case <-ctx.Done():
			}
			return
		}

		scanner := bufio.NewScanner(stdout)
		var result bool
		if scanner.Scan() {
			result = true
		}
		select {
		case done <- result:
		case <-ctx.Done():
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
