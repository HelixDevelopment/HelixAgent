package junie

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/modelsdev"
	"dev.helix.agent/internal/utils"
)

// JunieCLIProvider implements the LLMProvider interface using Junie CLI
// from JetBrains. This is used when Junie API key is available.
//
// Junie CLI Features:
// - JSON output format for structured responses
// - Session management with --session-id
// - Multiple model support (sonnet, opus, gpt, gemini, grok)
// - BYOK (Bring Your Own Key) for multiple providers
// - ACP mode for IDE integration
type JunieCLIProvider struct {
	model           string
	cliPath         string
	cliAvailable    bool
	cliCheckOnce    sync.Once
	cliCheckErr     error
	timeout         time.Duration
	maxOutputTokens int
	sessionID       string
	apiKey          string

	// Dynamic model discovery
	availableModels     []string
	modelsDiscovered    bool
	modelsDiscoveryOnce sync.Once

	// terminalErrMu / terminalErr capture an unrecoverable auth/subscription
	// error (e.g. "403 Forbidden: No active JetBrains AI subscription found").
	// Once set, HealthCheck returns it directly without re-invoking the CLI —
	// retrying produces the same answer and floods logs. Cleared only by
	// process restart.
	terminalErrMu sync.RWMutex
	terminalErr   error
}

// isJunieTerminalAuthError reports whether stderr indicates an unrecoverable
// auth/subscription condition. These messages come from the junie CLI
// itself and indicate the user must take action (renew subscription,
// rotate API key) before the provider can possibly work again.
func isJunieTerminalAuthError(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "no active jetbrains ai subscription") ||
		strings.Contains(low, "403 forbidden") && strings.Contains(low, "jetbrains") ||
		strings.Contains(low, "401 unauthorized") ||
		(strings.Contains(low, "subscription") && strings.Contains(low, "expired"))
}

// markTerminalError records an unrecoverable auth error so future
// HealthCheck calls short-circuit.
func (p *JunieCLIProvider) markTerminalError(err error) {
	if err == nil {
		return
	}
	p.terminalErrMu.Lock()
	if p.terminalErr == nil {
		p.terminalErr = err
	}
	p.terminalErrMu.Unlock()
}

// terminalError returns the recorded terminal error, or nil.
func (p *JunieCLIProvider) terminalError() error {
	p.terminalErrMu.RLock()
	defer p.terminalErrMu.RUnlock()
	return p.terminalErr
}

// junieJSONResponse represents the JSON output from Junie CLI.
// The CLI emits two error shapes: a singular `error` string (legacy)
// and an `errors` array (current — used for auth-failure messages
// like "Junie: 403 Forbidden: No active JetBrains AI subscription
// found"). Both must be captured because the CLI exits 0 even when
// authentication fails and the auth banner is the only signal that
// the call did not actually produce a model response.
type junieJSONResponse struct {
	Result    string                 `json:"result"`
	SessionID string                 `json:"session_id"`
	Usage     map[string]interface{} `json:"usage"`
	Model     string                 `json:"model"`
	Error     string                 `json:"error,omitempty"`
	Errors    []string               `json:"errors,omitempty"`
}

// Known Junie models (aliases and BYOK providers)
// These are model aliases that Junie CLI supports
var knownJunieModels = []string{
	// Anthropic models (via alias)
	"sonnet",
	"opus",
	// OpenAI models (via alias)
	"gpt",
	"gpt-codex",
	// Google models (via alias)
	"gemini-pro",
	"gemini-flash",
	// xAI models (via alias)
	"grok",
	// Default selection
	"Default",
}

// BYOK provider model IDs (when using direct API keys)
var byokModels = map[string][]string{
	"anthropic": {
		"claude-opus-4-6",
		"claude-sonnet-4-6",
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-5-20250929",
		"claude-haiku-4-5-20251001",
		"claude-3-5-sonnet-20241022",
		"claude-3-opus-20240229",
	},
	"openai": {
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"o1-preview",
		"o1-mini",
	},
	"google": {
		"gemini-2.0-flash",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
	},
	"grok": {
		"grok-2",
		"grok-2-mini",
		"grok-2-vision",
	},
}

// JunieCLIConfig holds configuration for the CLI provider
type JunieCLIConfig struct {
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int
	APIKey          string
}

// DefaultJunieCLIConfig returns default configuration
func DefaultJunieCLIConfig() JunieCLIConfig {
	apiKey := os.Getenv("JUNIE_API_KEY")
	return JunieCLIConfig{
		Model:           "",
		Timeout:         180 * time.Second,
		MaxOutputTokens: 8192,
		APIKey:          apiKey,
	}
}

// NewJunieCLIProvider creates a new Junie CLI provider
func NewJunieCLIProvider(config JunieCLIConfig) *JunieCLIProvider {
	if config.Timeout == 0 {
		config.Timeout = 180 * time.Second
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 8192
	}
	if config.APIKey == "" {
		config.APIKey = os.Getenv("JUNIE_API_KEY")
	}

	p := &JunieCLIProvider{
		model:           config.Model,
		timeout:         config.Timeout,
		maxOutputTokens: config.MaxOutputTokens,
		apiKey:          config.APIKey,
	}

	if p.model == "" {
		p.model = p.GetBestAvailableModel()
	}

	return p
}

// NewJunieCLIProviderWithModel creates a CLI provider with a specific model
func NewJunieCLIProviderWithModel(model string) *JunieCLIProvider {
	config := DefaultJunieCLIConfig()
	config.Model = model
	return NewJunieCLIProvider(config)
}

// IsCLIAvailable checks if Junie CLI is installed and available
func (p *JunieCLIProvider) IsCLIAvailable() bool {
	p.cliCheckOnce.Do(func() {
		path, err := exec.LookPath("junie")
		if err != nil {
			p.cliCheckErr = fmt.Errorf("junie command not found in PATH: %w", err)
			p.cliAvailable = false
			return
		}
		p.cliPath = path

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, path, "--version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			p.cliCheckErr = fmt.Errorf("junie command failed: %w (output: %s)", err, string(output))
			p.cliAvailable = false
			return
		}

		if !IsJunieAuthenticated() && p.apiKey == "" {
			p.cliCheckErr = fmt.Errorf("junie CLI not authenticated and no JUNIE_API_KEY provided")
			p.cliAvailable = false
			return
		}

		p.cliAvailable = true
	})

	return p.cliAvailable
}

// GetCLIError returns the error from CLI availability check
func (p *JunieCLIProvider) GetCLIError() error {
	p.IsCLIAvailable()
	return p.cliCheckErr
}

// Complete implements the LLMProvider interface
func (p *JunieCLIProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("Junie CLI not available: %v", p.cliCheckErr)
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

	cmdCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	model := p.model
	if req.ModelParams.Model != "" {
		model = req.ModelParams.Model
	}
	if !utils.ValidateCommandArg(model) {
		return nil, fmt.Errorf("model name contains invalid characters")
	}

	args := []string{
		"--auth", p.apiKey,
		"--output-format", "json",
		"--task", prompt,
	}

	if model != "" && model != "Default" {
		args = append(args, "--model", model)
	}

	if p.sessionID != "" {
		if utils.ValidateCommandArg(p.sessionID) {
			args = append(args, "--session-id", p.sessionID)
		}
	}

	cmd := exec.CommandContext(cmdCtx, p.cliPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("junie CLI timed out after %v", p.timeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			stdoutStr := strings.TrimSpace(stdout.String())
			stderrStr := strings.TrimSpace(stderr.String())
			var errorDetail strings.Builder
			errorDetail.WriteString(fmt.Sprintf("exit code %d", exitErr.ExitCode()))
			if stderrStr != "" {
				errorDetail.WriteString(fmt.Sprintf(", stderr: %s", stderrStr))
			}
			if stdoutStr != "" {
				errorDetail.WriteString(fmt.Sprintf(", stdout: %s", stdoutStr))
			}
			return nil, fmt.Errorf("junie CLI failed: %s", errorDetail.String())
		}
		return nil, fmt.Errorf("junie CLI failed: %w (output: %s)", err, stderr.String())
	}

	rawOutput := stdout.String()
	if rawOutput == "" {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return nil, fmt.Errorf("junie CLI returned empty response with stderr: %s", stderrStr)
		}
		return nil, fmt.Errorf("junie CLI returned empty response")
	}

	// Junie's CLI exits 0 even when authentication fails — the only
	// signal is an `errors` array in the JSON. Detect this BEFORE
	// using the JSON as a model response, otherwise the auth banner
	// gets sent back to the client as if it were the assistant's
	// reply (Finding #46, observed 2026-04-26 13:24).
	if errMsg := junieJSONErrorMessage(rawOutput); errMsg != "" {
		err := fmt.Errorf("junie CLI failed: %s", errMsg)
		if isJunieTerminalAuthError(errMsg) {
			p.markTerminalError(err)
		}
		return nil, err
	}

	output, sessionID, metadata := p.parseJSONResponse(rawOutput)

	if sessionID != "" {
		if utils.ValidateCommandArg(sessionID) {
			p.sessionID = sessionID
		}
	}

	promptTokens := len(prompt) / 4
	completionTokens := len(output) / 4
	if usage, ok := metadata["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			promptTokens = int(pt)
		}
		if ct, ok := usage["completion_tokens"].(float64); ok {
			completionTokens = int(ct)
		}
	}

	return &models.LLMResponse{
		ID:           fmt.Sprintf("junie-cli-%s", sessionID),
		ProviderID:   "junie-cli",
		ProviderName: "junie-cli",
		Content:      output,
		FinishReason: "stop",
		TokensUsed:   promptTokens + completionTokens,
		ResponseTime: duration.Milliseconds(),
		CreatedAt:    time.Now(),
		Metadata: map[string]interface{}{
			"model":             model,
			"session_id":        sessionID,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"raw_metadata":      metadata,
			"source":            "junie-cli",
		},
	}, nil
}

// junieJSONErrorMessage parses a Junie CLI JSON output and returns a
// flattened error message if the response indicates failure (either
// the legacy singular `error` field or the `errors` array). Returns
// empty string when the output is a valid model response. Used to
// short-circuit Complete/CompleteStream before treating auth banners
// as model content (Finding #46).
func junieJSONErrorMessage(rawOutput string) string {
	rawOutput = strings.TrimSpace(rawOutput)
	if rawOutput == "" {
		return ""
	}
	var jsonResp junieJSONResponse
	if err := json.Unmarshal([]byte(rawOutput), &jsonResp); err != nil {
		return ""
	}
	// If Result is non-empty AND no errors, it's a real response.
	if jsonResp.Result != "" && jsonResp.Error == "" && len(jsonResp.Errors) == 0 {
		return ""
	}
	if len(jsonResp.Errors) > 0 {
		return strings.Join(jsonResp.Errors, "; ")
	}
	if jsonResp.Error != "" {
		return jsonResp.Error
	}
	// No Result, no Errors — degenerate; treat as empty error so caller
	// falls back to its existing empty-response handling.
	return ""
}

// parseJSONResponse extracts content from Junie CLI JSON output
func (p *JunieCLIProvider) parseJSONResponse(rawOutput string) (string, string, map[string]interface{}) {
	rawOutput = strings.TrimSpace(rawOutput)
	metadata := make(map[string]interface{})

	var jsonResp junieJSONResponse
	if err := json.Unmarshal([]byte(rawOutput), &jsonResp); err == nil {
		if jsonResp.Result != "" {
			metadata["usage"] = jsonResp.Usage
			metadata["model"] = jsonResp.Model
			if jsonResp.Error != "" {
				metadata["error"] = jsonResp.Error
			}
			return jsonResp.Result, jsonResp.SessionID, metadata
		}
	}

	return rawOutput, "", metadata
}

// CompleteStream implements streaming for Junie CLI.
// Short-circuits with the recorded terminal error if a previous call
// has determined the provider is unrecoverable (Finding #46) — without
// this guard, the auth-error banner streams back to the client as if
// it were the assistant's reply.
func (p *JunieCLIProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("Junie CLI not available: %v", p.cliCheckErr)
	}
	if termErr := p.terminalError(); termErr != nil {
		return nil, termErr
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

	cmdCtx, cancel := context.WithTimeout(ctx, p.timeout)

	args := []string{
		"--auth", p.apiKey,
		"--task", prompt,
	}

	if p.model != "" && p.model != "Default" {
		args = append(args, "--model", p.model)
	}

	cmd := exec.CommandContext(cmdCtx, p.cliPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start junie CLI: %w", err)
	}

	responseChan := make(chan *models.LLMResponse)

	go func() {
		defer close(responseChan)
		defer cancel()

		// Junie's CLI emits a single JSON object per call — there is no
		// true token stream. Buffer the entire stdout, then parse for
		// errors before emitting anything to the client. Without this
		// buffer the auth banner ("Junie: 403 Forbidden: No active
		// JetBrains AI subscription found") streams back to the client
		// line-by-line as if it were the model's response (Finding #46).
		scanner := bufio.NewScanner(stdout)
		var fullContent strings.Builder
		for scanner.Scan() {
			fullContent.WriteString(scanner.Text())
			fullContent.WriteString("\n")
		}
		_ = cmd.Wait() //nolint:errcheck

		raw := strings.TrimSpace(fullContent.String())
		if errMsg := junieJSONErrorMessage(raw); errMsg != "" {
			err := fmt.Errorf("junie CLI failed: %s", errMsg)
			if isJunieTerminalAuthError(errMsg) {
				p.markTerminalError(err)
			}
			// Signal failure with empty content + finish_reason="error"
			// + the error in metadata. The provider chain checks
			// finish_reason and falls through to the next provider when
			// it sees "error". Without this guard the auth banner gets
			// streamed to the client as if it were the model's reply.
			responseChan <- &models.LLMResponse{
				ProviderName: "junie-cli",
				FinishReason: "error",
				Metadata:     map[string]interface{}{"error": err.Error()},
			}
			return
		}

		// Real response — extract the content via the JSON parser, fall
		// back to the raw blob if not parseable JSON.
		content, _, _ := p.parseJSONResponse(raw)
		if content == "" {
			content = raw
		}
		responseChan <- &models.LLMResponse{
			Content:      content,
			ProviderName: "junie-cli",
			FinishReason: "stop",
		}
	}()

	return responseChan, nil
}

// HealthCheck checks if Junie CLI is available and working.
// If a previous call recorded a terminal auth/subscription error
// (Finding #45 — no active JetBrains AI subscription), this short-circuits
// without spawning another CLI invocation that we already know will fail.
func (p *JunieCLIProvider) HealthCheck() error {
	if !p.IsCLIAvailable() {
		return p.cliCheckErr
	}

	if termErr := p.terminalError(); termErr != nil {
		return termErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := p.Complete(ctx, &models.LLMRequest{
		Prompt: "Reply with just 'OK'",
		ModelParams: models.ModelParameters{
			MaxTokens: 10,
		},
	})
	if err != nil {
		if isJunieTerminalAuthError(err.Error()) {
			p.markTerminalError(err)
		}
		return err
	}

	if resp.Content == "" {
		return fmt.Errorf("health check returned empty response")
	}

	return nil
}

// GetCapabilities returns the capabilities of Junie CLI provider
func (p *JunieCLIProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:   p.GetAvailableModels(),
		SupportsStreaming: true,
		SupportsTools:     false,
		Limits: models.ModelLimits{
			MaxTokens:             8192,
			MaxConcurrentRequests: 1,
		},
	}
}

// ValidateConfig validates the configuration
func (p *JunieCLIProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if !p.IsCLIAvailable() {
		return false, []string{fmt.Sprintf("Junie CLI not available: %v", p.cliCheckErr)}
	}
	return true, nil
}

// GetName returns the provider name
func (p *JunieCLIProvider) GetName() string {
	return "junie-cli"
}

// GetProviderType returns the provider type
func (p *JunieCLIProvider) GetProviderType() string {
	return "junie"
}

// GetCurrentModel returns the current model
func (p *JunieCLIProvider) GetCurrentModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *JunieCLIProvider) SetModel(model string) {
	p.model = model
}

// IsJunieInstalled checks if Junie CLI is installed
func IsJunieInstalled() bool {
	_, err := exec.LookPath("junie")
	return err == nil
}

// IsJunieAuthenticated checks if Junie is logged in
func IsJunieAuthenticated() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	credPath := filepath.Join(homeDir, ".junie", "credentials.json")
	info, err := os.Stat(credPath)
	if err != nil || info.IsDir() {
		return false
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		return false
	}

	var creds map[string]interface{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return false
	}

	if token, ok := creds["api_token"].(string); ok && token != "" {
		return true
	}

	return false
}

// GetJuniePath returns the path to junie command if installed
func GetJuniePath() (string, error) {
	path, err := exec.LookPath("junie")
	if err != nil {
		return "", fmt.Errorf("junie command not found in PATH: %w", err)
	}
	return path, nil
}

// DiscoverModels attempts to discover available models
func (p *JunieCLIProvider) DiscoverModels() []string {
	p.modelsDiscoveryOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if p.IsCLIAvailable() {
			cliModels := p.discoverModelsFromCLI(ctx)
			if len(cliModels) > 0 {
				p.availableModels = cliModels
				p.modelsDiscovered = true
				return
			}
		}

		modelsDevModels := p.discoverModelsFromModelsDev(ctx)
		if len(modelsDevModels) > 0 {
			p.availableModels = modelsDevModels
			p.modelsDiscovered = true
			return
		}

		p.availableModels = knownJunieModels
	})

	return p.availableModels
}

// discoverModelsFromCLI tries to get models from Junie CLI
func (p *JunieCLIProvider) discoverModelsFromCLI(ctx context.Context) []string {
	commands := [][]string{
		{"--help"},
	}

	for _, args := range commands {
		cmd := exec.CommandContext(ctx, p.cliPath, args...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			models := parseJunieModelsOutput(string(output))
			if len(models) > 0 {
				return models
			}
		}
	}

	return nil
}

// discoverModelsFromModelsDev fetches models from models.dev API
func (p *JunieCLIProvider) discoverModelsFromModelsDev(ctx context.Context) []string {
	client := modelsdev.NewClient(nil)

	opts := &modelsdev.ListModelsOptions{
		Limit: 100,
	}

	providers := []string{"anthropic", "openai", "google", "x-ai"}
	var allModels []string

	for _, provider := range providers {
		resp, err := client.ListProviderModels(ctx, provider, opts)
		if err != nil || resp == nil {
			continue
		}

		for _, m := range resp.Models {
			if m.ID != "" {
				allModels = append(allModels, m.ID)
			}
		}
	}

	return allModels
}

// parseJunieModelsOutput parses CLI output to extract model names
func parseJunieModelsOutput(output string) []string {
	var models []string
	lines := strings.Split(output, "\n")

	modelPatterns := []string{"sonnet", "opus", "gpt", "gemini", "grok", "claude"}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for _, pattern := range modelPatterns {
			if strings.Contains(strings.ToLower(line), pattern) {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					modelName := parts[0]
					modelName = strings.Trim(modelName, ".,:-*")
					if len(modelName) > 2 {
						models = append(models, modelName)
					}
				}
				break
			}
		}
	}

	return models
}

// GetAvailableModels returns the list of available models
func (p *JunieCLIProvider) GetAvailableModels() []string {
	return p.DiscoverModels()
}

// IsModelAvailable checks if a specific model is available
func (p *JunieCLIProvider) IsModelAvailable(model string) bool {
	models := p.GetAvailableModels()
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}

// GetBestAvailableModel returns the best available model
func (p *JunieCLIProvider) GetBestAvailableModel() string {
	models := p.GetAvailableModels()

	modelPriority := []string{
		"opus",
		"sonnet",
		"gpt-4o",
		"gemini-pro",
		"grok",
		"gpt",
		"gemini-flash",
		"Default",
	}

	for _, preferred := range modelPriority {
		for _, model := range models {
			if strings.Contains(strings.ToLower(model), strings.ToLower(preferred)) {
				return model
			}
		}
	}

	if len(models) > 0 {
		return models[0]
	}
	return "Default"
}

// GetKnownJunieModels returns the list of known Junie models
func GetKnownJunieModels() []string {
	return knownJunieModels
}

// GetBYOKModels returns BYOK provider models
func GetBYOKModels() map[string][]string {
	return byokModels
}

// CanUseJunieCLI returns true if Junie CLI can be used
func CanUseJunieCLI() bool {
	if os.Getenv("JUNIE_API_KEY") != "" {
		return IsJunieInstalled()
	}
	return IsJunieInstalled() && IsJunieAuthenticated()
}

// DiscoverJunieModels is a standalone function to discover models
func DiscoverJunieModels() ([]string, error) {
	if !IsJunieInstalled() {
		return knownJunieModels, fmt.Errorf("junie CLI not installed, returning known models")
	}

	path, err := GetJuniePath()
	if err != nil {
		return knownJunieModels, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--help")
	output, err := cmd.CombinedOutput()
	if err == nil {
		models := parseJunieModelsOutput(string(output))
		if len(models) > 0 {
			return models, nil
		}
	}

	return knownJunieModels, fmt.Errorf("could not discover models from CLI, returning known models")
}
