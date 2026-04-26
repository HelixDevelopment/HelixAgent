package qwen

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/modelsdev"
	"dev.helix.agent/internal/utils"
)

// QwenCLIProvider implements the LLMProvider interface using Qwen Code CLI
// This is used when OAuth credentials are present but the API rejects them
// (OAuth tokens from Qwen Code are product-restricted)
type QwenCLIProvider struct {
	model           string
	cliPath         string // Path to qwen CLI binary
	cliAvailable    bool
	cliCheckOnce    sync.Once
	cliCheckErr     error
	timeout         time.Duration
	maxOutputTokens int
	// Dynamic model discovery
	availableModels     []string
	modelsDiscovered    bool
	modelsDiscoveryOnce sync.Once

	// terminalErrMu / terminalErr capture an unrecoverable auth/subscription
	// error reported by the CLI (e.g. "Qwen OAuth free tier was discontinued").
	// Once set, HealthCheck returns it directly without re-invoking the CLI —
	// retrying produces the same answer and floods logs. Cleared only by
	// process restart.
	terminalErrMu sync.RWMutex
	terminalErr   error
}

// isQwenTerminalAuthError reports whether stderr indicates an unrecoverable
// auth/subscription condition. These messages come from the qwen CLI itself
// and indicate the user must take action (rotate credentials, switch plan)
// before the provider can possibly work again.
func isQwenTerminalAuthError(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "oauth free tier was discontinued") ||
		strings.Contains(low, "no active") && strings.Contains(low, "subscription") ||
		strings.Contains(low, "run /auth to switch") ||
		strings.Contains(low, "401 unauthorized") ||
		strings.Contains(low, "403 forbidden")
}

// markTerminalError records an unrecoverable auth error so future
// HealthCheck calls short-circuit instead of re-invoking the CLI.
func (p *QwenCLIProvider) markTerminalError(err error) {
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
func (p *QwenCLIProvider) terminalError() error {
	p.terminalErrMu.RLock()
	defer p.terminalErrMu.RUnlock()
	return p.terminalErr
}

// Known Qwen models (fallback if discovery fails)
// These are kept updated based on Alibaba/DashScope documentation
var knownQwenModels = []string{
	"qwen-max",
	"qwen-max-latest",
	"qwen-plus",
	"qwen-plus-latest",
	"qwen-turbo",
	"qwen-turbo-latest",
	"qwen2.5-72b-instruct",
	"qwen2.5-32b-instruct",
	"qwen2.5-14b-instruct",
	"qwen2.5-7b-instruct",
	"qwen2.5-coder-32b-instruct",
	"qwen-long",
	"qwen-vl-max",
	"qwen-vl-plus",
}

// QwenCLIConfig holds configuration for the CLI provider
type QwenCLIConfig struct {
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int
}

// DefaultQwenCLIConfig returns default configuration
// Model is initially empty - will be discovered dynamically
func DefaultQwenCLIConfig() QwenCLIConfig {
	return QwenCLIConfig{
		Model:           "", // Will be discovered dynamically
		Timeout:         120 * time.Second,
		MaxOutputTokens: 4096,
	}
}

// NewQwenCLIProvider creates a new Qwen Code CLI provider
func NewQwenCLIProvider(config QwenCLIConfig) *QwenCLIProvider {
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 4096
	}

	p := &QwenCLIProvider{
		model:           config.Model,
		timeout:         config.Timeout,
		maxOutputTokens: config.MaxOutputTokens,
	}

	// If no model specified, discover the best available model
	if p.model == "" {
		p.model = p.GetBestAvailableModel()
	}

	return p
}

// NewQwenCLIProviderWithModel creates a CLI provider with a specific model
func NewQwenCLIProviderWithModel(model string) *QwenCLIProvider {
	config := DefaultQwenCLIConfig()
	config.Model = model
	return NewQwenCLIProvider(config)
}

// IsCLIAvailable checks if Qwen Code CLI is installed and available
func (p *QwenCLIProvider) IsCLIAvailable() bool {
	p.cliCheckOnce.Do(func() {
		// Check for qwen command in PATH
		path, err := exec.LookPath("qwen")
		if err != nil {
			p.cliCheckErr = fmt.Errorf("qwen command not found in PATH: %w", err)
			p.cliAvailable = false
			return
		}
		p.cliPath = path

		// Verify it works by checking version
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, path, "--version") // #nosec G204
		output, err := cmd.CombinedOutput()
		if err != nil {
			p.cliCheckErr = fmt.Errorf("qwen command failed: %w (output: %s)", err, string(output))
			p.cliAvailable = false
			return
		}

		// Check if OAuth credentials exist (don't make API call - may hit quota)
		credReader := oauth_credentials.GetGlobalReader()
		if !credReader.HasValidQwenCredentials() {
			p.cliCheckErr = fmt.Errorf("qwen CLI not authenticated: no valid OAuth credentials found")
			p.cliAvailable = false
			return
		}

		p.cliAvailable = true
	})

	return p.cliAvailable
}

// GetCLIError returns the error from CLI availability check
func (p *QwenCLIProvider) GetCLIError() error {
	p.IsCLIAvailable() // Ensure check is done
	return p.cliCheckErr
}

// Complete implements the LLMProvider interface
func (p *QwenCLIProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("Qwen Code CLI not available: %v", p.cliCheckErr)
	}

	// NOTE: Message content validation removed - exec.CommandContext properly escapes arguments
	// The prompt is passed as a separate argument to the -p flag, not concatenated into the command string
	// Therefore, command injection is not possible even with special characters like (){}$|&

	// Build the prompt from messages
	var promptBuilder strings.Builder
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			promptBuilder.WriteString("System: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
		case "user":
			promptBuilder.WriteString("Human: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
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

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Determine model to use
	model := p.model
	if req.ModelParams.Model != "" {
		model = req.ModelParams.Model
	}
	// Validate model name for command injection safety (controlled identifiers, not user content)
	if !utils.ValidateCommandArg(model) {
		return nil, fmt.Errorf("model name contains invalid characters")
	}

	// Build qwen command arguments
	// NOTE: Qwen Code CLI does NOT support --max-tokens; it manages tokens internally
	args := []string{
		"-p", prompt, // Print mode - just returns the response
		"--model", model,
	}

	// Execute qwen command
	cmd := exec.CommandContext(cmdCtx, p.cliPath, args...) // #nosec G204

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		// Check for context cancellation
		if cmdCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("qwen CLI timed out after %v", p.timeout)
		}
		return nil, fmt.Errorf("qwen CLI failed: %w (stderr: %s)", err, stderr.String())
	}

	output := stdout.String()
	if output == "" {
		return nil, fmt.Errorf("qwen CLI returned empty response")
	}

	// Estimate token count (rough approximation: 4 chars per token)
	promptTokens := len(prompt) / 4
	completionTokens := len(output) / 4

	return &models.LLMResponse{
		Content:      output,
		ProviderName: "qwen-cli",
		FinishReason: "stop",
		TokensUsed:   promptTokens + completionTokens,
		ResponseTime: duration.Milliseconds(),
		Metadata: map[string]interface{}{
			"model":             model,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
		},
	}, nil
}

// CompleteStream implements streaming for Qwen CLI
func (p *QwenCLIProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("Qwen Code CLI not available: %v", p.cliCheckErr)
	}

	// NOTE: Message content validation removed - exec.CommandContext properly escapes arguments
	// The prompt is passed as a separate argument to the -p flag, not concatenated into the command string
	// Therefore, command injection is not possible even with special characters like (){}$|&

	// Build the prompt from messages
	var promptBuilder strings.Builder
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			promptBuilder.WriteString("System: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
		case "user":
			promptBuilder.WriteString("Human: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
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

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, p.timeout)

	// Determine model to use
	model := p.model
	if req.ModelParams.Model != "" {
		model = req.ModelParams.Model
	}
	// Validate model name for command injection safety (controlled identifiers, not user content)
	if !utils.ValidateCommandArg(model) {
		cancel()
		return nil, fmt.Errorf("model name contains invalid characters")
	}

	// Build qwen command arguments
	// NOTE: Qwen Code CLI does NOT support --max-tokens
	args := []string{
		"-p", prompt,
		"--model", model,
	}

	cmd := exec.CommandContext(cmdCtx, p.cliPath, args...) // #nosec G204

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start qwen CLI: %w", err)
	}

	// Create response channel
	responseChan := make(chan *models.LLMResponse)

	// Read output and send chunks in goroutine
	go func() {
		defer close(responseChan)
		defer cancel()

		scanner := bufio.NewScanner(stdout)
		var fullContent strings.Builder

		for scanner.Scan() {
			line := scanner.Text()
			fullContent.WriteString(line)
			fullContent.WriteString("\n")

			responseChan <- &models.LLMResponse{
				Content:      line,
				ProviderName: "qwen-cli",
				FinishReason: "",
			}
		}

		// Wait for command to finish
		_ = cmd.Wait() //nolint:errcheck

		// Send final response
		responseChan <- &models.LLMResponse{
			Content:      fullContent.String(),
			ProviderName: "qwen-cli",
			FinishReason: "stop",
		}
	}()

	return responseChan, nil
}

// HealthCheck checks if Qwen CLI is available and working.
// If a previous call recorded a terminal auth/subscription error
// (Finding #44 — Qwen OAuth free tier discontinued), this short-circuits
// without spawning another CLI invocation that we already know will fail.
func (p *QwenCLIProvider) HealthCheck() error {
	if !p.IsCLIAvailable() {
		return p.cliCheckErr
	}

	if termErr := p.terminalError(); termErr != nil {
		return termErr
	}

	// Try a minimal prompt to verify it works
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := p.Complete(ctx, &models.LLMRequest{
		Prompt: "Reply with just 'OK'",
		ModelParams: models.ModelParameters{
			MaxTokens: 10,
		},
	})
	if err != nil {
		if isQwenTerminalAuthError(err.Error()) {
			p.markTerminalError(err)
		}
		return err
	}

	if resp.Content == "" {
		return fmt.Errorf("health check returned empty response")
	}

	return nil
}

// GetCapabilities returns the capabilities of Qwen CLI provider
func (p *QwenCLIProvider) GetCapabilities() *models.ProviderCapabilities {
	// Available models through Qwen Code CLI
	supportedModels := []string{
		"qwen-plus",
		"qwen-turbo",
		"qwen-max",
		"qwen2.5-72b-instruct",
		"qwen2.5-32b-instruct",
		"qwen2.5-14b-instruct",
		"qwen2.5-7b-instruct",
		"qwen2.5-coder-32b-instruct",
		"qwen2.5-coder-14b-instruct",
	}

	return &models.ProviderCapabilities{
		SupportedModels:   supportedModels,
		SupportsStreaming: true,
		SupportsTools:     false, // CLI doesn't support tools directly
		Limits: models.ModelLimits{
			MaxTokens:             8192,
			MaxConcurrentRequests: 1, // CLI doesn't support concurrent requests
		},
	}
}

// ValidateConfig validates the configuration
func (p *QwenCLIProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if !p.IsCLIAvailable() {
		return false, []string{fmt.Sprintf("Qwen Code CLI not available: %v", p.cliCheckErr)}
	}
	return true, nil
}

// GetName returns the provider name
func (p *QwenCLIProvider) GetName() string {
	return "qwen-cli"
}

// GetProviderType returns the provider type
func (p *QwenCLIProvider) GetProviderType() string {
	return "qwen"
}

// GetCurrentModel returns the current model
func (p *QwenCLIProvider) GetCurrentModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *QwenCLIProvider) SetModel(model string) {
	p.model = model
}

// IsQwenCodeInstalled is a standalone function to check if Qwen Code is installed
func IsQwenCodeInstalled() bool {
	_, err := exec.LookPath("qwen")
	return err == nil
}

// GetQwenCodePath returns the path to qwen command if installed
func GetQwenCodePath() (string, error) {
	path, err := exec.LookPath("qwen")
	if err != nil {
		return "", fmt.Errorf("qwen command not found in PATH: %w", err)
	}
	return path, nil
}

// IsQwenCodeAuthenticated checks if Qwen Code is logged in
// Fixed: Check OAuth credentials directly instead of making API call (avoids quota issues)
func IsQwenCodeAuthenticated() bool {
	// First check if CLI is installed
	_, err := GetQwenCodePath()
	if err != nil {
		return false
	}

	// Check OAuth credentials file directly (no API call)
	credReader := oauth_credentials.GetGlobalReader()
	return credReader.HasValidQwenCredentials()
}

// CanUseQwenOAuth returns true if Qwen OAuth can be used via CLI
// This checks: 1) OAuth credentials exist, 2) qwen CLI installed, 3) qwen CLI authenticated
func CanUseQwenOAuth() bool {
	// Check if OAuth is enabled and credentials exist
	if os.Getenv("QWEN_CODE_USE_OAUTH_CREDENTIALS") != "true" {
		return false
	}

	// Check for credential file
	homeDir, _ := os.UserHomeDir() //nolint:errcheck
	credPath := homeDir + "/.qwen/oauth_creds.json"
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return false
	}

	// Check if CLI is installed and authenticated
	return IsQwenCodeInstalled() && IsQwenCodeAuthenticated()
}

// DiscoverModels attempts to discover available models using 3-tier system:
// 1. Primary: Query Qwen CLI for available models
// 2. Fallback 1: Query models.dev API for Qwen/Alibaba models
// 3. Fallback 2: Use hardcoded known models
func (p *QwenCLIProvider) DiscoverModels() []string {
	p.modelsDiscoveryOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Tier 1: Try CLI discovery
		if p.IsCLIAvailable() {
			cliModels := p.discoverModelsFromCLI(ctx)
			if len(cliModels) > 0 {
				p.availableModels = cliModels
				p.modelsDiscovered = true
				return
			}
		}

		// Tier 2: Try models.dev API
		modelsDevModels := p.discoverModelsFromModelsDev(ctx)
		if len(modelsDevModels) > 0 {
			p.availableModels = modelsDevModels
			p.modelsDiscovered = true
			return
		}

		// Tier 3: Fallback to known models
		p.availableModels = knownQwenModels
	})

	return p.availableModels
}

// discoverModelsFromCLI tries to get models from Qwen CLI
func (p *QwenCLIProvider) discoverModelsFromCLI(ctx context.Context) []string {
	// Try different commands that might list models
	commands := [][]string{
		{"models"},
		{"models", "list"},
		{"model", "list"},
		{"--list-models"},
	}

	for _, args := range commands {
		cmd := exec.CommandContext(ctx, p.cliPath, args...) // #nosec G204
		output, err := cmd.CombinedOutput()
		if err == nil {
			models := parseQwenModelsOutput(string(output))
			if len(models) > 0 {
				return models
			}
		}
	}

	return nil
}

// discoverModelsFromModelsDev fetches Qwen models from models.dev API
func (p *QwenCLIProvider) discoverModelsFromModelsDev(ctx context.Context) []string {
	client := modelsdev.NewClient(nil)

	// Search for Qwen models
	opts := &modelsdev.ListModelsOptions{
		Limit: 50,
	}

	// Try to list provider models (Alibaba/Qwen)
	resp, err := client.ListProviderModels(ctx, "alibaba", opts)
	if err != nil {
		// Try with "qwen" as provider ID
		resp, err = client.ListProviderModels(ctx, "qwen", opts)
		if err != nil {
			return nil
		}
	}

	if resp == nil || len(resp.Models) == 0 {
		return nil
	}

	var models []string
	for _, m := range resp.Models {
		if m.ID != "" && strings.HasPrefix(m.ID, "qwen") {
			models = append(models, m.ID)
		}
	}

	return models
}

// parseQwenModelsOutput parses CLI output to extract model names
func parseQwenModelsOutput(output string) []string {
	var models []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for lines containing model identifiers
		// Common patterns: "qwen-*", "qwen2.5-*"
		if strings.HasPrefix(line, "qwen") ||
			strings.Contains(line, "qwen-max") ||
			strings.Contains(line, "qwen-plus") ||
			strings.Contains(line, "qwen-turbo") ||
			strings.Contains(line, "qwen2") {
			// Extract the model name (first word or before whitespace)
			parts := strings.Fields(line)
			if len(parts) > 0 {
				modelName := parts[0]
				// Remove any trailing punctuation
				modelName = strings.Trim(modelName, ".,:-*")
				if strings.HasPrefix(modelName, "qwen") && len(modelName) > 4 {
					models = append(models, modelName)
				}
			}
		}
	}

	return models
}

// GetAvailableModels returns the list of available models (discovered or known)
func (p *QwenCLIProvider) GetAvailableModels() []string {
	return p.DiscoverModels()
}

// IsModelAvailable checks if a specific model is available
func (p *QwenCLIProvider) IsModelAvailable(model string) bool {
	models := p.GetAvailableModels()
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}

// GetBestAvailableModel returns the best available model (prefers max > plus > turbo)
func (p *QwenCLIProvider) GetBestAvailableModel() string {
	models := p.GetAvailableModels()

	// Priority order: max > plus > turbo > coder > other
	priorities := []string{"qwen-max", "qwen-plus", "qwen-turbo", "coder", "qwen2.5"}

	for _, priority := range priorities {
		for _, model := range models {
			if strings.Contains(model, priority) {
				return model
			}
		}
	}

	// Return first available model or default
	if len(models) > 0 {
		return models[0]
	}
	return "qwen-plus"
}
