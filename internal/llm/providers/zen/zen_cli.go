package zen

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/modelsdev"
	"dev.helix.agent/internal/utils"
)

// ZenCLIProvider implements the LLMProvider interface using OpenCode CLI
// This is used as a fallback when direct Zen API fails for a model
// The CLI facade allows using models that don't work with direct API calls
type ZenCLIProvider struct {
	model           string
	cliPath         string // Path to opencode CLI binary
	cliAvailable    bool
	cliCheckOnce    sync.Once
	cliCheckErr     error
	timeout         time.Duration
	maxOutputTokens int
	// Dynamic model discovery
	availableModels     []string
	modelsDiscovered    bool
	modelsDiscoveryOnce sync.Once
	// failedAPIModels tracks models that failed direct API validation.
	// CONST-029: safe.Store — lookups/inserts are atomic without any
	// caller-held lock.
	failedAPIModels *safe.Store[string, bool]
}

// ZenCLIConfig holds configuration for the CLI provider
type ZenCLIConfig struct {
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int
}

// Known Zen/OpenCode models (fallback if discovery fails)
// Updated 2026-02: Verified working free models from Zen API
var knownZenModels = []string{
	"big-pickle",
	"gpt-5-nano",
	"glm-4.7",
	"kimi-k2",
	"gemini-3-flash",
	// Note: qwen3-coder removed from free tier 2026-02
}

// DefaultZenCLIConfig returns default configuration
// Model is initially empty - will be discovered dynamically
func DefaultZenCLIConfig() ZenCLIConfig {
	return ZenCLIConfig{
		Model:           "", // Will be discovered dynamically
		Timeout:         120 * time.Second,
		MaxOutputTokens: 4096,
	}
}

// NewZenCLIProvider creates a new OpenCode CLI provider
func NewZenCLIProvider(config ZenCLIConfig) *ZenCLIProvider {
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 4096
	}

	p := &ZenCLIProvider{
		model:           config.Model,
		timeout:         config.Timeout,
		maxOutputTokens: config.MaxOutputTokens,
		failedAPIModels: safe.NewStore[string, bool](),
	}

	// Note: Model discovery is lazy - only triggered when GetBestAvailableModel() is called
	// or when the model is actually needed for a request.
	// This avoids slow initialization when the provider is created during test setup.

	return p
}

// NewZenCLIProviderWithModel creates a CLI provider with a specific model
func NewZenCLIProviderWithModel(model string) *ZenCLIProvider {
	config := DefaultZenCLIConfig()
	config.Model = model
	return NewZenCLIProvider(config)
}

// NewZenCLIProviderWithUnavailableCLI creates a provider for testing with unavailable CLI
// This properly initializes the sync.Once state to prevent re-checking
func NewZenCLIProviderWithUnavailableCLI(model string, err error) *ZenCLIProvider {
	p := &ZenCLIProvider{
		model:           model,
		timeout:         120 * time.Second,
		maxOutputTokens: 4096,
		cliAvailable:    false,
		cliCheckErr:     err,
		failedAPIModels: safe.NewStore[string, bool](),
	}
	// Force the sync.Once to be completed so IsCLIAvailable() returns our set values
	p.cliCheckOnce.Do(func() {})
	return p
}

// IsCLIAvailable checks if OpenCode CLI is installed and available
func (p *ZenCLIProvider) IsCLIAvailable() bool {
	p.cliCheckOnce.Do(func() {
		// Check for opencode command in PATH
		path, err := exec.LookPath("opencode")
		if err != nil {
			p.cliCheckErr = fmt.Errorf("opencode command not found in PATH: %w", err)
			p.cliAvailable = false
			return
		}
		p.cliPath = path

		// Verify it works by checking version
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, path, "--version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			p.cliCheckErr = fmt.Errorf("opencode command failed: %w (output: %s)", err, string(output))
			p.cliAvailable = false
			return
		}

		p.cliAvailable = true
	})

	return p.cliAvailable
}

// GetCLIError returns the error from CLI availability check
func (p *ZenCLIProvider) GetCLIError() error {
	p.IsCLIAvailable() // Ensure check is done
	return p.cliCheckErr
}

// MarkModelAsFailedAPI marks a model as having failed direct API validation
func (p *ZenCLIProvider) MarkModelAsFailedAPI(model string) {
	p.failedAPIModels.Put(model, true)
}

// IsModelFailedAPI checks if a model has failed direct API validation
func (p *ZenCLIProvider) IsModelFailedAPI(model string) bool {
	v, _ := p.failedAPIModels.Get(model)
	return v
}

// ShouldUseCLIFacade determines if CLI facade should be used for a model
func (p *ZenCLIProvider) ShouldUseCLIFacade(model string) bool {
	return p.IsModelFailedAPI(model) && p.IsCLIAvailable()
}

// Complete implements the LLMProvider interface
func (p *ZenCLIProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("OpenCode CLI not available: %v", p.cliCheckErr)
	}

	// NOTE: Message content validation removed - exec.CommandContext properly escapes arguments
	// The prompt is passed as a separate argument, not concatenated into the command string
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

	// Build opencode command arguments
	// Format: opencode run [message] --format json --model provider/model
	args := []string{
		"run",
		prompt,             // Message as positional argument
		"--format", "json", // JSON output format for structured parsing
		"--model", model, // Specify model to use
	}

	// Execute opencode command
	cmd := exec.CommandContext(cmdCtx, p.cliPath, args...)

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
			return nil, fmt.Errorf("opencode CLI timed out after %v", p.timeout)
		}
		return nil, fmt.Errorf("opencode CLI failed: %w (stderr: %s)", err, stderr.String())
	}

	rawOutput := stdout.String()
	if rawOutput == "" {
		return nil, fmt.Errorf("opencode CLI returned empty response")
	}

	// Parse JSON output format: {"response": "...content..."}
	output := p.parseJSONResponse(rawOutput)

	// Estimate token count (rough approximation: 4 chars per token)
	promptTokens := len(prompt) / 4
	completionTokens := len(output) / 4

	return &models.LLMResponse{
		ID:           fmt.Sprintf("zen-cli-%d", time.Now().UnixNano()),
		ProviderID:   "zen-cli",
		ProviderName: "zen-cli",
		Content:      output,
		TokensUsed:   promptTokens + completionTokens,
		ResponseTime: duration.Milliseconds(),
		CreatedAt:    time.Now(),
		Metadata: map[string]interface{}{
			"source":            "opencode-cli",
			"cli_path":          p.cliPath,
			"facade":            true,
			"model":             model,
			"latency":           duration.String(),
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
		},
	}, nil
}

// openCodeJSONResponse represents the JSON output format from OpenCode CLI
// Format: {"response": "...content..."}
type openCodeJSONResponse struct {
	Response string `json:"response"`
}

// parseJSONResponse extracts content from OpenCode CLI JSON output
// If parsing fails, returns the raw output as-is
func (p *ZenCLIProvider) parseJSONResponse(rawOutput string) string {
	rawOutput = strings.TrimSpace(rawOutput)

	// Try to parse as JSON
	var jsonResp openCodeJSONResponse
	if err := json.Unmarshal([]byte(rawOutput), &jsonResp); err == nil {
		if jsonResp.Response != "" {
			return jsonResp.Response
		}
	}

	// Fallback: return raw output if JSON parsing fails
	// This handles cases where -f json might not be supported
	return rawOutput
}

// CompleteStream implements streaming completion using CLI
func (p *ZenCLIProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("OpenCode CLI not available: %v", p.cliCheckErr)
	}

	ch := make(chan *models.LLMResponse, 10)

	go func() {
		defer close(ch)

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
			ch <- &models.LLMResponse{
				ProviderID:   "zen-cli",
				ProviderName: "zen-cli",
				Metadata: map[string]interface{}{
					"error": "no prompt provided",
				},
			}
			return
		}

		// NOTE: Prompt content validation removed - exec.CommandContext properly escapes arguments
		// The prompt is passed as a separate argument, not concatenated into the command string
		// Therefore, command injection is not possible even with special characters like (){}$|&

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
			ch <- &models.LLMResponse{
				ProviderID:   "zen-cli",
				ProviderName: "zen-cli",
				Metadata: map[string]interface{}{
					"error": "model name contains invalid characters",
				},
			}
			return
		}

		// Build opencode command arguments with streaming
		// Format: opencode run [message] --format json --model provider/model
		args := []string{
			"run",
			prompt,             // Message as positional argument
			"--format", "json", // JSON output for streaming
			"--model", model, // Specify model to use
		}

		// Note: OpenCode run doesn't support --max-tokens or --stream flags
		// Streaming is default with --format json

		// Execute opencode command
		cmd := exec.CommandContext(cmdCtx, p.cliPath, args...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			ch <- &models.LLMResponse{
				ProviderID:   "zen-cli",
				ProviderName: "zen-cli",
				Metadata: map[string]interface{}{
					"error": fmt.Sprintf("failed to create stdout pipe: %v", err),
				},
			}
			return
		}

		startTime := time.Now()
		if err := cmd.Start(); err != nil {
			ch <- &models.LLMResponse{
				ProviderID:   "zen-cli",
				ProviderName: "zen-cli",
				Metadata: map[string]interface{}{
					"error": fmt.Sprintf("failed to start opencode command: %v", err),
				},
			}
			return
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-cmdCtx.Done():
				return
			default:
				chunk := scanner.Text()
				ch <- &models.LLMResponse{
					ID:           fmt.Sprintf("zen-cli-%d", time.Now().UnixNano()),
					ProviderID:   "zen-cli",
					ProviderName: "zen-cli",
					Content:      chunk,
					CreatedAt:    time.Now(),
					ResponseTime: time.Since(startTime).Milliseconds(),
					Metadata: map[string]interface{}{
						"source": "opencode-cli",
						"stream": true,
						"facade": true,
						"model":  model,
					},
				}
			}
		}

		if err := cmd.Wait(); err != nil && cmdCtx.Err() == nil {
			ch <- &models.LLMResponse{
				ProviderID:   "zen-cli",
				ProviderName: "zen-cli",
				Metadata: map[string]interface{}{
					"error": fmt.Sprintf("opencode CLI exited with error: %v", err),
				},
			}
		}
	}()

	return ch, nil
}

// HealthCheck implements the LLMProvider interface
func (p *ZenCLIProvider) HealthCheck() error {
	if !p.IsCLIAvailable() {
		return fmt.Errorf("OpenCode CLI not available: %v", p.cliCheckErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.cliPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opencode CLI health check failed: %w", err)
	}

	return nil
}

// GetCapabilities implements the LLMProvider interface
func (p *ZenCLIProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:         p.GetAvailableModels(),
		SupportsStreaming:       true,
		SupportsFunctionCalling: false, // CLI doesn't support tools
		SupportsVision:          false,
		SupportsTools:           false,
		SupportedFeatures: []string{
			"text_completion",
			"chat",
			"streaming",
		},
		SupportedRequestTypes: []string{
			"text_completion",
			"chat",
		},
		Limits: models.ModelLimits{
			MaxTokens:             p.maxOutputTokens,
			MaxInputLength:        32000,
			MaxOutputLength:       p.maxOutputTokens,
			MaxConcurrentRequests: 1, // CLI is sequential
		},
		Metadata: map[string]string{
			"provider":    "OpenCode Zen (CLI Facade)",
			"cli_command": "opencode",
			"facade":      "true",
		},
	}
}

// ValidateConfig implements the LLMProvider interface
func (p *ZenCLIProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string

	if !p.IsCLIAvailable() {
		errors = append(errors, fmt.Sprintf("OpenCode CLI not available: %v", p.cliCheckErr))
	}

	return len(errors) == 0, errors
}

// GetName implements the LLMProvider interface
func (p *ZenCLIProvider) GetName() string {
	return "zen-cli"
}

// GetProviderType implements the LLMProvider interface
func (p *ZenCLIProvider) GetProviderType() string {
	return "zen"
}

// GetCurrentModel returns the current model
func (p *ZenCLIProvider) GetCurrentModel() string {
	return p.model
}

// SetModel sets the model to use
func (p *ZenCLIProvider) SetModel(model string) {
	p.model = model
}

// IsOpenCodeInstalled is a standalone function to check if OpenCode is installed
// IsOpenCodeInstalled reports whether the `opencode` CLI is both
// present on PATH AND actually runnable on this host. The previous
// implementation returned true whenever `exec.LookPath("opencode")`
// succeeded — but on resource-constrained hosts (CI runners,
// CONST-022 `nice -n 19 / ionice -c 3` test budget) the binary can be
// on PATH yet get SIGKILL'd before it can execute, producing
// "installed-but-unusable" state that diverges from what
// ZenCLIProvider.IsCLIAvailable() observes internally.
//
// To keep this package-level helper agreeing with the instance-level
// ZenCLIProvider.IsCLIAvailable()`, we now run the same `--version`
// probe with the same 10-second timeout. Tests branch on this
// function; if it disagreed with IsCLIAvailable() the assertions
// would race.
func IsOpenCodeInstalled() bool {
	if _, err := exec.LookPath("opencode"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "opencode", "--version") // #nosec G204 // hard-coded binary + no user input
	return cmd.Run() == nil
}

// GetOpenCodePath returns the path to opencode command if installed
func GetOpenCodePath() (string, error) {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return "", fmt.Errorf("opencode command not found in PATH: %w", err)
	}
	return path, nil
}

// DiscoverModels attempts to discover available models using 3-tier system:
// 1. Primary: Query OpenCode CLI for available models
// 2. Fallback 1: Query models.dev API for Zen models
// 3. Fallback 2: Use hardcoded known models
func (p *ZenCLIProvider) DiscoverModels() []string {
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
		p.availableModels = knownZenModels
	})

	return p.availableModels
}

// discoverModelsFromCLI tries to get models from OpenCode CLI
func (p *ZenCLIProvider) discoverModelsFromCLI(ctx context.Context) []string {
	// Try different commands that might list models
	commands := [][]string{
		{"models"},
		{"models", "list"},
		{"model", "list"},
		{"--list-models"},
	}

	for _, args := range commands {
		cmd := exec.CommandContext(ctx, p.cliPath, args...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			models := parseZenModelsOutput(string(output))
			if len(models) > 0 {
				return models
			}
		}
	}

	return nil
}

// parseZenModelsOutput parses CLI output to extract model names
func parseZenModelsOutput(output string) []string {
	var models []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for Zen model identifiers
		// Common patterns: "big-pickle", "grok-code", "glm-4.7-free", "gpt-5-nano"
		if strings.Contains(line, "pickle") ||
			strings.Contains(line, "grok") ||
			strings.Contains(line, "glm") ||
			strings.Contains(line, "gpt-5") ||
			strings.HasPrefix(line, "opencode/") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				modelName := parts[0]
				modelName = strings.Trim(modelName, ".,:-*")
				if len(modelName) > 3 {
					models = append(models, modelName)
				}
			}
		}
	}

	return models
}

// discoverModelsFromModelsDev fetches Zen models from models.dev API
func (p *ZenCLIProvider) discoverModelsFromModelsDev(ctx context.Context) []string {
	client := modelsdev.NewClient(nil)

	// Search for OpenCode/Zen models
	opts := &modelsdev.ListModelsOptions{
		Limit: 50,
	}

	// Try to list provider models
	resp, err := client.ListProviderModels(ctx, "opencode", opts)
	if err != nil {
		return nil
	}

	if resp == nil || len(resp.Models) == 0 {
		return nil
	}

	var models []string
	for _, m := range resp.Models {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	return models
}

// GetAvailableModels returns the list of available models (discovered or known)
func (p *ZenCLIProvider) GetAvailableModels() []string {
	return p.DiscoverModels()
}

// IsModelAvailable checks if a specific model is available
func (p *ZenCLIProvider) IsModelAvailable(model string) bool {
	models := p.GetAvailableModels()
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}

// GetBestAvailableModel returns the best available model
func (p *ZenCLIProvider) GetBestAvailableModel() string {
	models := p.GetAvailableModels()

	// Priority order: big-pickle > gpt-5 > glm > qwen > kimi
	priorities := []string{"big-pickle", "gpt-5", "glm", "qwen", "kimi"}

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
	return DefaultZenModel
}

// GetKnownZenModels returns the list of known Zen models (static fallback)
func GetKnownZenModels() []string {
	return knownZenModels
}

// DiscoverZenModels is a standalone function to discover models without creating a provider
func DiscoverZenModels() ([]string, error) {
	if !IsOpenCodeInstalled() {
		return knownZenModels, fmt.Errorf("opencode CLI not installed, returning known models")
	}

	path, err := GetOpenCodePath()
	if err != nil {
		return knownZenModels, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Try the models command
	cmd := exec.CommandContext(ctx, path, "models")
	output, err := cmd.CombinedOutput()
	if err == nil {
		models := parseZenModelsOutput(string(output))
		if len(models) > 0 {
			return models, nil
		}
	}

	return knownZenModels, fmt.Errorf("could not discover models from CLI, returning known models")
}
