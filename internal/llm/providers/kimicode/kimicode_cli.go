package kimicode

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
	"dev.helix.agent/internal/utils"
)

const (
	KimiCodeDefaultModel   = "kimi-for-coding"
	KimiCodeMaxContext     = 262144
	KimiCodeMaxOutput      = 32768
	KimiCodeAPIURL         = "https://api.kimi.com/coding/v1"
	KimiCodeCredentialPath = ".kimi/credentials/kimi-code.json" // #nosec G101 -- not a credential (map key / config label / env-var reference)
)

var knownKimiCodeModels = []string{
	"kimi-for-coding",
}

type KimiCodeCLIProvider struct {
	model               string
	cliPath             string
	cliAvailable        bool
	cliCheckOnce        sync.Once
	cliCheckErr         error
	timeout             time.Duration
	maxOutputTokens     int
	sessionID           string
	availableModels     []string
	modelsDiscoveryOnce sync.Once
}

type kimiCodeJSONResponse struct {
	Role    string                 `json:"role"`
	Content []kimiCodeContentBlock `json:"content"`
}

type kimiCodeContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Think     string `json:"think,omitempty"`
	Encrypted string `json:"encrypted,omitempty"`
}

type kimiCodeCredential struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresAt    float64 `json:"expires_at"`
	Scope        string  `json:"scope"`
	TokenType    string  `json:"token_type"`
}

type KimiCodeCLIConfig struct {
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int
}

func DefaultKimiCodeCLIConfig() KimiCodeCLIConfig {
	return KimiCodeCLIConfig{
		Model:           KimiCodeDefaultModel,
		Timeout:         180 * time.Second,
		MaxOutputTokens: KimiCodeMaxOutput,
	}
}

func NewKimiCodeCLIProvider(config KimiCodeCLIConfig) *KimiCodeCLIProvider {
	if config.Timeout == 0 {
		config.Timeout = 180 * time.Second
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = KimiCodeMaxOutput
	}
	if config.Model == "" {
		config.Model = KimiCodeDefaultModel
	}

	return &KimiCodeCLIProvider{
		model:           config.Model,
		timeout:         config.Timeout,
		maxOutputTokens: config.MaxOutputTokens,
	}
}

func NewKimiCodeCLIProviderWithModel(model string) *KimiCodeCLIProvider {
	config := DefaultKimiCodeCLIConfig()
	config.Model = model
	return NewKimiCodeCLIProvider(config)
}

func (p *KimiCodeCLIProvider) IsCLIAvailable() bool {
	if IsInsideKimiCodeSession() {
		p.cliCheckErr = fmt.Errorf("Kimi Code CLI not available: cannot run inside another Kimi Code session")
		p.cliAvailable = false
		return false
	}
	p.cliCheckOnce.Do(func() {
		path, err := exec.LookPath("kimi")
		if err != nil {
			p.cliCheckErr = fmt.Errorf("kimi command not found in PATH: %w", err)
			p.cliAvailable = false
			return
		}
		p.cliPath = path

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, path, "--version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			p.cliCheckErr = fmt.Errorf("kimi command failed: %w (output: %s)", err, string(output))
			p.cliAvailable = false
			return
		}

		if !IsKimiCodeAuthenticated() {
			p.cliCheckErr = fmt.Errorf("kimi CLI not authenticated: credential file missing or expired")
			p.cliAvailable = false
			return
		}

		p.cliAvailable = true
	})

	return p.cliAvailable
}

func (p *KimiCodeCLIProvider) GetCLIError() error {
	p.IsCLIAvailable()
	return p.cliCheckErr
}

func (p *KimiCodeCLIProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("Kimi Code CLI not available: %v", p.cliCheckErr)
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
		"--print",
		"--output-format", "stream-json",
		"-p", prompt,
	}

	if p.sessionID != "" {
		if utils.ValidateCommandArg(p.sessionID) {
			args = append(args, "--session", p.sessionID)
		} else {
			p.sessionID = ""
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
			return nil, fmt.Errorf("kimi CLI timed out after %v", p.timeout)
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
			return nil, fmt.Errorf("kimi CLI failed: %s", errorDetail.String())
		}
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
		return nil, fmt.Errorf("kimi CLI failed: %w (output: %s)", err, errorDetail.String())
	}

	rawOutput := stdout.String()
	if rawOutput == "" {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return nil, fmt.Errorf("kimi CLI returned empty response with stderr: %s", stderrStr)
		}
		return nil, fmt.Errorf("kimi CLI returned empty response (no stdout or stderr)")
	}

	output, thinking := p.parseJSONResponse(rawOutput)

	promptTokens := len(prompt) / 4
	completionTokens := len(output) / 4

	metadata := map[string]interface{}{
		"model":             model,
		"session_id":        p.sessionID,
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
	}
	if thinking != "" {
		metadata["thinking"] = thinking
	}

	return &models.LLMResponse{
		ID:           fmt.Sprintf("kimi-code-cli-%d", time.Now().UnixNano()),
		ProviderID:   "kimi-code-cli",
		ProviderName: "kimi-code-cli",
		Content:      output,
		FinishReason: "stop",
		TokensUsed:   promptTokens + completionTokens,
		ResponseTime: duration.Milliseconds(),
		CreatedAt:    time.Now(),
		Metadata:     metadata,
	}, nil
}

func (p *KimiCodeCLIProvider) parseJSONResponse(rawOutput string) (string, string) {
	rawOutput = strings.TrimSpace(rawOutput)
	var thinking strings.Builder
	var text strings.Builder

	lines := strings.Split(rawOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var resp kimiCodeJSONResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			text.WriteString(line)
			text.WriteString("\n")
			continue
		}

		for _, block := range resp.Content {
			switch block.Type {
			case "think":
				if block.Think != "" {
					thinking.WriteString(block.Think)
					thinking.WriteString("\n")
				}
			case "text":
				if block.Text != "" {
					text.WriteString(block.Text)
				}
			}
		}
	}

	return text.String(), thinking.String()
}

func (p *KimiCodeCLIProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if !p.IsCLIAvailable() {
		return nil, fmt.Errorf("Kimi Code CLI not available: %v", p.cliCheckErr)
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
		"--print",
		"--output-format", "stream-json",
		"-p", prompt,
	}

	cmd := exec.CommandContext(cmdCtx, p.cliPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start kimi CLI: %w", err)
	}

	responseChan := make(chan *models.LLMResponse)

	go func() {
		defer close(responseChan)
		defer cancel()

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		var fullContent strings.Builder

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var resp kimiCodeJSONResponse
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				responseChan <- &models.LLMResponse{
					Content:      line,
					ProviderName: "kimi-code-cli",
					FinishReason: "",
				}
				fullContent.WriteString(line)
				fullContent.WriteString("\n")
				continue
			}

			for _, block := range resp.Content {
				if block.Type == "text" && block.Text != "" {
					responseChan <- &models.LLMResponse{
						Content:      block.Text,
						ProviderName: "kimi-code-cli",
						FinishReason: "",
					}
					fullContent.WriteString(block.Text)
				}
			}
		}

		_ = cmd.Wait() //nolint:errcheck

		responseChan <- &models.LLMResponse{
			Content:      fullContent.String(),
			ProviderName: "kimi-code-cli",
			FinishReason: "stop",
		}
	}()

	return responseChan, nil
}

func (p *KimiCodeCLIProvider) HealthCheck() error {
	if !p.IsCLIAvailable() {
		return p.cliCheckErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

func (p *KimiCodeCLIProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:   knownKimiCodeModels,
		SupportsStreaming: true,
		SupportsTools:     false,
		Limits: models.ModelLimits{
			MaxTokens:             KimiCodeMaxOutput,
			MaxConcurrentRequests: 1,
		},
	}
}

func (p *KimiCodeCLIProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if !p.IsCLIAvailable() {
		return false, []string{fmt.Sprintf("Kimi Code CLI not available: %v", p.cliCheckErr)}
	}
	return true, nil
}

func (p *KimiCodeCLIProvider) GetName() string {
	return "kimi-code-cli"
}

func (p *KimiCodeCLIProvider) GetProviderType() string {
	return "kimi-code"
}

func (p *KimiCodeCLIProvider) GetCurrentModel() string {
	return p.model
}

func (p *KimiCodeCLIProvider) SetModel(model string) {
	p.model = model
}

func (p *KimiCodeCLIProvider) DiscoverModels() []string {
	p.modelsDiscoveryOnce.Do(func() {
		p.availableModels = knownKimiCodeModels
	})
	return p.availableModels
}

func (p *KimiCodeCLIProvider) GetAvailableModels() []string {
	return p.DiscoverModels()
}

func (p *KimiCodeCLIProvider) IsModelAvailable(model string) bool {
	models := p.GetAvailableModels()
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}

func (p *KimiCodeCLIProvider) GetBestAvailableModel() string {
	models := p.GetAvailableModels()
	if len(models) > 0 {
		return models[0]
	}
	return KimiCodeDefaultModel
}

func IsKimiCodeInstalled() bool {
	_, err := exec.LookPath("kimi")
	return err == nil
}

func IsInsideKimiCodeSession() bool {
	if os.Getenv("KIMI_CODE_SESSION") != "" {
		return true
	}
	if os.Getenv("KIMI_SESSION_ID") != "" {
		return true
	}
	return false
}

func IsKimiCodeAuthenticated() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	credPath := filepath.Join(homeDir, KimiCodeCredentialPath)
	data, err := os.ReadFile(credPath)
	if err != nil {
		return false
	}

	var creds kimiCodeCredential
	if err := json.Unmarshal(data, &creds); err != nil {
		return false
	}

	if creds.AccessToken == "" {
		return false
	}

	if creds.ExpiresAt > 0 {
		expiryTime := time.Unix(int64(creds.ExpiresAt), 0)
		if time.Now().After(expiryTime) {
			return false
		}
	}

	return true
}

func CanUseKimiCodeCLI() bool {
	if os.Getenv("KIMI_CODE_USE_OAUTH_CREDENTIALS") != "true" {
		return false
	}

	homeDir, _ := os.UserHomeDir() //nolint:errcheck
	credPath := filepath.Join(homeDir, KimiCodeCredentialPath)
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return false
	}

	return IsKimiCodeInstalled() && IsKimiCodeAuthenticated()
}

func GetKnownKimiCodeModels() []string {
	return knownKimiCodeModels
}
