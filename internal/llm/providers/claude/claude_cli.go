package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/modelsdev"
	"dev.helix.agent/internal/utils"
)

// ClaudeCLIProvider implements the LLMProvider interface using Claude Code CLI
// This is used when OAuth credentials are present but the API rejects them
// (OAuth tokens from Claude Code are product-restricted)
//
// Features:
// - JSON output format for structured responses
// - Session continuity with --resume
// - Tool auto-approval with --allowedTools
type ClaudeCLIProvider struct {
	model           string
	cliPath         string // Path to claude CLI binary
	cliAvailable    bool
	cliCheckOnce    sync.Once
	cliCheckErr     error
	timeout         time.Duration
	maxOutputTokens int
	sessionID       string // For conversation continuity
	// Dynamic model discovery
	availableModels     []string
	modelsDiscovered    bool
	modelsDiscoveryOnce sync.Once
}

// claudeJSONResponse represents the JSON output from Claude CLI
// Format: {"result": "...", "session_id": "...", "usage": {...}}
type claudeJSONResponse struct {
	Result    string                 `json:"result"`
	SessionID string                 `json:"session_id"`
	Usage     map[string]interface{} `json:"usage"`
	Model     string                 `json:"model"`
}

// Known Claude models (fallback if discovery fails)
// These are kept updated based on Anthropic's documentation
var knownClaudeModels = []string{
	"claude-opus-4-6",
	"claude-sonnet-4-6",
	"claude-opus-4-5-20251101",
	"claude-sonnet-4-5-20250929",
	"claude-haiku-4-5-20251001",
	"claude-sonnet-4-20250514",
	"claude-3-5-sonnet-20241022",
	"claude-3-5-haiku-20241022",
	"claude-3-opus-20240229",
	"claude-3-sonnet-20240229",
	"claude-3-haiku-20240307",
}

// ClaudeCLIConfig holds configuration for the CLI provider
type ClaudeCLIConfig struct {
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int
}

// DefaultClaudeCLIConfig returns default configuration
// Model is initially empty - will be discovered dynamically
func DefaultClaudeCLIConfig() ClaudeCLIConfig {
	return ClaudeCLIConfig{
		Model:           "",                // Will be discovered dynamically
		Timeout:         180 * time.Second, // Increased for Claude 4.6
		MaxOutputTokens: 4096,
	}
}

// NewClaudeCLIProvider creates a new Claude Code CLI provider
func NewClaudeCLIProvider(config ClaudeCLIConfig) *ClaudeCLIProvider {
	if config.Timeout == 0 {
		config.Timeout = 180 * time.Second // Increased for Claude 4.6
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 4096
	}

	p := &ClaudeCLIProvider{
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

// NewClaudeCLIProviderWithModel creates a CLI provider with a specific model
func NewClaudeCLIProviderWithModel(model string) *ClaudeCLIProvider {
	config := DefaultClaudeCLIConfig()
	config.Model = model
	return NewClaudeCLIProvider(config)
}

// IsCLIAvailable checks if Claude Code CLI is installed and available
// NOTE: Claude Code CLI doesn't have an "auth status" command, so we check
// credential files directly to verify authentication
func (p *ClaudeCLIProvider) IsCLIAvailable() bool {
	// Allow CLI usage when running as HelixAgent server (non-interactive).
	// The nested session check only matters for interactive CLI-to-CLI calls
	// where launching claude inside claude would cause recursion.
	// When running as an HTTP server, we're a separate process that can safely
	// invoke claude CLI even if the server was started from a Claude Code session.
	if os.Getenv("HELIXAGENT_SERVER_MODE") != "" || os.Getenv("GIN_MODE") != "" {
		// Server mode — skip nested session check
	} else if IsInsideClaudeCodeSession() {
		p.cliCheckErr = fmt.Errorf("Claude Code CLI not available: cannot run inside another Claude Code session")
		p.cliAvailable = false
		return false
	}
	p.cliCheckOnce.Do(func() {
		// Check for claude command in PATH
		path, err := exec.LookPath("claude")
		if err != nil {
			p.cliCheckErr = fmt.Errorf("claude command not found in PATH: %w", err)
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
			p.cliCheckErr = fmt.Errorf("claude command failed: %w (output: %s)", err, string(output))
			p.cliAvailable = false
			return
		}

		// Check if user is authenticated by verifying credential files
		// (Claude Code CLI doesn't have an "auth status" command)
		if !IsClaudeCodeAuthenticated() {
			p.cliCheckErr = fmt.Errorf("claude CLI not authenticated: credential file missing or expired")
			p.cliAvailable = false
			return
		}

		p.cliAvailable = true
	})

	return p.cliAvailable
}

// GetCLIError returns the error from CLI availability check
func (p *ClaudeCLIProvider) GetCLIError() error {
	p.IsCLIAvailable() // Ensure check is done
	return p.cliCheckErr
}

// Complete implements the LLMProvider interface
func (p *ClaudeCLIProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("Claude Code CLI not available: %v", p.cliCheckErr)
	}

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

	// NOTE: Prompt validation removed - exec.CommandContext properly escapes arguments
	// The prompt is passed as a separate argument to the -p flag, not concatenated into the command string
	// Therefore, command injection is not possible even with special characters

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Determine model to use
	model := p.model
	if req.ModelParams.Model != "" {
		model = req.ModelParams.Model
	}
	// Validate model name for command injection safety (if ever used in commands)
	if !utils.ValidateCommandArg(model) {
		return nil, fmt.Errorf("model name contains invalid characters")
	}

	// Build claude command arguments
	// Use --output-format json for structured output with session metadata
	// NOTE: Claude Code CLI does NOT support --max-tokens; it manages tokens internally
	args := []string{
		"-p", prompt, // Print mode - non-interactive
		"--output-format", "json", // JSON output with session info
	}

	// Continue existing session if we have one
	if p.sessionID != "" {
		if !utils.ValidateCommandArg(p.sessionID) {
			// Invalid session ID, clear it
			p.sessionID = ""
		} else {
			args = append(args, "--resume", p.sessionID)
		}
	}

	// Execute claude command
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
			return nil, fmt.Errorf("claude CLI timed out after %v", p.timeout)
		}
		// Check for exit status 1 specifically (common Claude CLI error)
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Build comprehensive error message with all available information
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
			// Check for common Claude CLI issues
			if exitErr.ExitCode() == 1 && stderrStr == "" && stdoutStr == "" {
				errorDetail.WriteString(" (possible causes: session conflict, authentication issue, or Claude CLI internal error)")
			}
			return nil, fmt.Errorf("claude CLI failed: %s", errorDetail.String())
		}
		// Build comprehensive error message with both stdout and stderr
		// Claude CLI may output errors to either stream
		stdoutStr := strings.TrimSpace(stdout.String())
		stderrStr := strings.TrimSpace(stderr.String())
		var errorDetail strings.Builder
		if stderrStr != "" {
			errorDetail.WriteString(stderrStr)
		}
		if stdoutStr != "" {
			if errorDetail.Len() > 0 {
				errorDetail.WriteString(" | stdout: ")
			}
			errorDetail.WriteString(stdoutStr)
		}
		if errorDetail.Len() == 0 {
			errorDetail.WriteString("(no output captured)")
		}
		return nil, fmt.Errorf("claude CLI failed: %w (output: %s)", err, errorDetail.String())
	}

	rawOutput := stdout.String()
	if rawOutput == "" {
		// Check if stderr has content (might be error message)
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return nil, fmt.Errorf("claude CLI returned empty response with stderr: %s", stderrStr)
		}
		return nil, fmt.Errorf("claude CLI returned empty response (no stdout or stderr)")
	}

	// Parse JSON output
	output, sessionID, metadata := p.parseJSONResponse(rawOutput)

	// Store session ID for conversation continuity
	if sessionID != "" {
		if !utils.ValidateCommandArg(sessionID) {
			// Invalid session ID, ignore it
			sessionID = ""
		} else {
			p.sessionID = sessionID
		}
	}

	// Calculate tokens from metadata or estimate
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
		ID:           fmt.Sprintf("claude-cli-%s", sessionID),
		ProviderID:   "claude-cli",
		ProviderName: "claude-cli",
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
		},
	}, nil
}

// parseJSONResponse extracts content from Claude CLI JSON output
// Returns: content, sessionID, metadata
func (p *ClaudeCLIProvider) parseJSONResponse(rawOutput string) (string, string, map[string]interface{}) {
	rawOutput = strings.TrimSpace(rawOutput)
	metadata := make(map[string]interface{})

	// Try to parse as JSON
	var jsonResp claudeJSONResponse
	if err := json.Unmarshal([]byte(rawOutput), &jsonResp); err == nil {
		if jsonResp.Result != "" {
			metadata["usage"] = jsonResp.Usage
			metadata["model"] = jsonResp.Model
			return jsonResp.Result, jsonResp.SessionID, metadata
		}
	}

	// Fallback: return raw output if JSON parsing fails
	// This handles cases where --output-format json might not be supported
	return rawOutput, "", metadata
}

// CompleteStream implements streaming for Claude CLI (reads output line by line)
func (p *ClaudeCLIProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("Claude Code CLI not available: %v", p.cliCheckErr)
	}

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

	// NOTE: Prompt validation removed - exec.CommandContext properly escapes arguments
	// The prompt is passed as a separate argument to the -p flag, not concatenated into the command string
	// Therefore, command injection is not possible even with special characters

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, p.timeout)

	// Build claude command arguments for streaming
	// NOTE: Claude Code CLI does NOT support --max-tokens or --model flags
	// It uses the model configured in user settings
	args := []string{
		"-p", prompt,
	}

	cmd := exec.CommandContext(cmdCtx, p.cliPath, args...) // #nosec G204

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start claude CLI: %w", err)
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
				ProviderName: "claude-cli",
				FinishReason: "",
			}
		}

		// Wait for command to finish
		_ = cmd.Wait() //nolint:errcheck

		// Send final response
		responseChan <- &models.LLMResponse{
			Content:      fullContent.String(),
			ProviderName: "claude-cli",
			FinishReason: "stop",
		}
	}()

	return responseChan, nil
}

// HealthCheck checks if Claude CLI is available and working
func (p *ClaudeCLIProvider) HealthCheck() error {
	if !p.IsCLIAvailable() {
		return p.cliCheckErr
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
		return err
	}

	if resp.Content == "" {
		return fmt.Errorf("health check returned empty response")
	}

	return nil
}

// GetCapabilities returns the capabilities of Claude CLI provider
func (p *ClaudeCLIProvider) GetCapabilities() *models.ProviderCapabilities {
	// Available models through Claude Code CLI
	supportedModels := []string{
		"claude-opus-4-6",
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-5-20250929",
		"claude-haiku-4-5-20251001",
		"claude-opus-4-20250514",
		"claude-sonnet-4-20250514",
		"claude-3-5-sonnet-20241022",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
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
func (p *ClaudeCLIProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if !p.IsCLIAvailable() {
		return false, []string{fmt.Sprintf("Claude Code CLI not available: %v", p.cliCheckErr)}
	}
	return true, nil
}

// GetName returns the provider name
func (p *ClaudeCLIProvider) GetName() string {
	return "claude-cli"
}

// GetProviderType returns the provider type
func (p *ClaudeCLIProvider) GetProviderType() string {
	return "claude"
}

// GetCurrentModel returns the current model
func (p *ClaudeCLIProvider) GetCurrentModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *ClaudeCLIProvider) SetModel(model string) {
	p.model = model
}

// IsClaudeCodeInstalled is a standalone function to check if Claude Code is installed
func IsClaudeCodeInstalled() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// IsInsideClaudeCodeSession returns true if we're running inside a Claude Code session.
// Claude Code cannot be launched inside another Claude Code session.
func IsInsideClaudeCodeSession() bool {
	// Claude Code sets CLAUDECODE env var when running
	if os.Getenv("CLAUDECODE") != "" {
		return true
	}
	// Also check CLAUDE_CODE_ENTRYPOINT
	if os.Getenv("CLAUDE_CODE_ENTRYPOINT") != "" {
		return true
	}
	return false
}

// GetClaudeCodePath returns the path to claude command if installed
func GetClaudeCodePath() (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude command not found in PATH: %w", err)
	}
	return path, nil
}

// IsClaudeCodeAuthenticated checks if Claude Code is logged in
// NOTE: Claude Code CLI doesn't have an "auth status" command, so we check
// credential files directly instead of running CLI commands
func IsClaudeCodeAuthenticated() bool {
	// Check if credential file exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	credPath := homeDir + "/.claude/.credentials.json"
	info, err := os.Stat(credPath)
	if err != nil || info.IsDir() {
		return false
	}

	// Read and parse credentials to check if they're valid
	data, err := os.ReadFile(credPath)
	if err != nil {
		return false
	}

	// Check that the file has valid JSON content (at minimum)
	var creds map[string]interface{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return false
	}

	// Check for required fields in credentials
	// Claude credentials have claudeAiOauth with accessToken and expiresAt
	if claudeOAuth, ok := creds["claudeAiOauth"].(map[string]interface{}); ok {
		if accessToken, ok := claudeOAuth["accessToken"].(string); ok && accessToken != "" {
			// Check if token is not expired
			if expiresAt, ok := claudeOAuth["expiresAt"].(float64); ok {
				expiryTime := time.UnixMilli(int64(expiresAt))
				if time.Now().Before(expiryTime) {
					return true // Valid credentials with non-expired token
				}
			}
		}
	}

	return false
}

// CanUseClaudeOAuth returns true if Claude OAuth can be used via CLI
// This checks: 1) OAuth credentials exist, 2) claude CLI installed, 3) claude CLI authenticated
func CanUseClaudeOAuth() bool {
	// Check if OAuth is enabled and credentials exist
	if os.Getenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS") != "true" {
		return false
	}

	// Check for credential file
	homeDir, _ := os.UserHomeDir() //nolint:errcheck
	credPath := homeDir + "/.claude/.credentials.json"
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return false
	}

	// Check if CLI is installed and authenticated
	return IsClaudeCodeInstalled() && IsClaudeCodeAuthenticated()
}

// DiscoverModels attempts to discover available models using 3-tier system:
// 1. Primary: Query Claude CLI for available models
// 2. Fallback 1: Query models.dev API for Anthropic/Claude models
// 3. Fallback 2: Use hardcoded known models
func (p *ClaudeCLIProvider) DiscoverModels() []string {
	if p.modelsDiscovered {
		return p.availableModels
	}
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
		p.availableModels = knownClaudeModels
	})

	return p.availableModels
}

// discoverModelsFromCLI tries to get models from Claude CLI
func (p *ClaudeCLIProvider) discoverModelsFromCLI(ctx context.Context) []string {
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
			models := parseModelsOutput(string(output))
			if len(models) > 0 {
				return models
			}
		}
	}

	return nil
}

// discoverModelsFromModelsDev fetches Claude/Anthropic models from models.dev API
func (p *ClaudeCLIProvider) discoverModelsFromModelsDev(ctx context.Context) []string {
	client := modelsdev.NewClient(nil)

	// Search for Claude models
	opts := &modelsdev.ListModelsOptions{
		Limit: 50,
	}

	// Try to list provider models
	resp, err := client.ListProviderModels(ctx, "anthropic", opts)
	if err != nil {
		return nil
	}

	if resp == nil || len(resp.Models) == 0 {
		return nil
	}

	var models []string
	for _, m := range resp.Models {
		if m.ID != "" && strings.HasPrefix(m.ID, "claude") {
			models = append(models, m.ID)
		}
	}

	return models
}

// parseModelsOutput parses CLI output to extract model names
func parseModelsOutput(output string) []string {
	var models []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for lines containing model identifiers
		// Common patterns: "claude-*", model names with version dates
		if strings.HasPrefix(line, "claude-") ||
			strings.Contains(line, "claude-opus") ||
			strings.Contains(line, "claude-sonnet") ||
			strings.Contains(line, "claude-haiku") {
			// Extract the model name (first word or before whitespace)
			parts := strings.Fields(line)
			if len(parts) > 0 {
				modelName := parts[0]
				// Remove any trailing punctuation
				modelName = strings.Trim(modelName, ".,:-*")
				if strings.HasPrefix(modelName, "claude-") && len(modelName) > 7 {
					models = append(models, modelName)
				}
			}
		}
	}

	return models
}

// GetAvailableModels returns the list of available models (discovered or known)
func (p *ClaudeCLIProvider) GetAvailableModels() []string {
	return p.DiscoverModels()
}

// IsModelAvailable checks if a specific model is available
func (p *ClaudeCLIProvider) IsModelAvailable(model string) bool {
	models := p.GetAvailableModels()
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}

// GetBestAvailableModel returns the best available model (prefers opus > sonnet > haiku, newer versions over older)
func (p *ClaudeCLIProvider) GetBestAvailableModel() string {
	models := p.GetAvailableModels()

	// Priority order with version preference: 4.6 > 4.5 > 4.x
	// Format: model-version (e.g., "claude-sonnet-4-6", "claude-opus-4-5-20251101")
	modelPriority := []string{
		"claude-opus-4-6",
		"claude-sonnet-4-6",
		"claude-opus-4-5",
		"claude-sonnet-4-5",
		"claude-haiku-4-5",
		"claude-opus-4",
		"claude-sonnet-4",
	}

	for _, preferred := range modelPriority {
		for _, model := range models {
			if strings.Contains(model, preferred) {
				return model
			}
		}
	}

	// Fallback: check for opus > sonnet > haiku in any model
	priorities := []string{"opus", "sonnet", "haiku"}
	for _, priority := range priorities {
		for _, model := range models {
			if strings.Contains(model, priority) {
				return model
			}
		}
	}

	// Return first available model or default to Sonnet 4.6
	if len(models) > 0 {
		return models[0]
	}
	return "claude-sonnet-4-6" // Default to latest Sonnet
}

// GetKnownModels returns the list of known Claude models (static fallback)
func GetKnownClaudeModels() []string {
	return knownClaudeModels
}

// DiscoverClaudeModels is a standalone function to discover models without creating a provider
func DiscoverClaudeModels() ([]string, error) {
	if !IsClaudeCodeInstalled() {
		return knownClaudeModels, fmt.Errorf("claude CLI not installed, returning known models")
	}

	path, err := GetClaudeCodePath()
	if err != nil {
		return knownClaudeModels, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Try the models command
	cmd := exec.CommandContext(ctx, path, "models") // #nosec G204
	output, err := cmd.CombinedOutput()
	if err == nil {
		models := parseModelsOutput(string(output))
		if len(models) > 0 {
			return models, nil
		}
	}

	return knownClaudeModels, fmt.Errorf("could not discover models from CLI, returning known models")
}
