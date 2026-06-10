// Package amazonq provides Amazon Q CLI agent integration.
// Amazon Q: AWS-powered AI coding assistant.
package amazonq

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// amazonqBinaries are the Amazon Q Developer CLI executables looked up on PATH,
// in preference order. The CLI ships as `q` (Amazon Q Developer) and, after the
// 2026 rename, as `kiro-cli`. Tests may inject a fake binary via the AMAZONQ_BIN
// environment variable to prove real exec is wired (anti-bluff).
var amazonqBinaries = []string{"q", "kiro-cli", "amazon-q"}

// getAmazonQBinOverride returns the test-only Amazon Q binary override, if set.
func getAmazonQBinOverride() string { return os.Getenv("AMAZONQ_BIN") }

// AmazonQ provides Amazon Q integration
type AmazonQ struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Amazon Q configuration
type Config struct {
	base.BaseConfig
	AWSProfile      string
	Region          string
	EnableTransform bool
}

// New creates a new Amazon Q integration
func New() *AmazonQ {
	info := agents.AgentInfo{
		Type:        agents.TypeAmazonQ,
		Name:        "Amazon Q",
		Description: "AWS AI coding assistant",
		Vendor:      "Amazon",
		Version:     "1.0.0",
		Capabilities: []string{
			"aws_integration",
			"code_generation",
			"code_explanation",
			"chat",
			"code_transformation",
			"security_scan",
			"documentation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &AmazonQ{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Region:          "us-east-1",
			EnableTransform: true,
		},
	}
}

// Initialize initializes Amazon Q
func (a *AmazonQ) Initialize(ctx context.Context, config interface{}) error {
	if err := a.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		a.config = cfg
	}

	return nil
}

// Execute executes a command
func (a *AmazonQ) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !a.IsStarted() {
		if err := a.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return a.chat(ctx, params)
	case "generate":
		return a.generate(ctx, params)
	case "explain":
		return a.explain(ctx, params)
	case "transform":
		return a.transform(ctx, params)
	case "scan":
		return a.scan(ctx, params)
	case "docs":
		return a.docs(ctx, params)
	case "status":
		return a.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveAmazonQBinary locates the Amazon Q CLI executable. Tests may inject a
// fake binary via the AMAZONQ_BIN environment variable (absolute path);
// otherwise `q`, then `kiro-cli`, then `amazon-q` are resolved on PATH. Returns
// an honest error when none is available — NEVER a fabricated success (BLUFF-001).
func (a *AmazonQ) resolveAmazonQBinary() (string, error) {
	if bin := getAmazonQBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("amazon-q binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	for _, name := range amazonqBinaries {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("amazon-q CLI not found on PATH (tried %s)", strings.Join(amazonqBinaries, ", "))
}

// runAmazonQ invokes the Amazon Q Developer CLI non-interactively and returns
// its textual output. The non-interactive form is
// `q chat --no-interactive --trust-all-tools "<message>"`. The AWS profile and
// region are forwarded via the environment when configured.
func (a *AmazonQ) runAmazonQ(ctx context.Context, message string) (string, error) {
	bin, err := a.resolveAmazonQBinary()
	if err != nil {
		return "", err
	}

	args := []string{"chat", "--no-interactive", "--trust-all-tools", message}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = a.GetWorkDir()
	cmd.Env = os.Environ()
	if a.config != nil {
		if a.config.AWSProfile != "" {
			cmd.Env = append(cmd.Env, "AWS_PROFILE="+a.config.AWSProfile)
		}
		if a.config.Region != "" {
			cmd.Env = append(cmd.Env, "AWS_REGION="+a.config.Region)
		}
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("amazon-q execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// chat performs chat by exec-ing the real Amazon Q CLI.
func (a *AmazonQ) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	response, err := a.runAmazonQ(ctx, message)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message":  message,
		"response": response,
		"region":   a.config.Region,
	}, nil
}

// generate generates code by exec-ing the real Amazon Q CLI.
func (a *AmazonQ) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	language, _ := params["language"].(string)

	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	if language == "" {
		language = "go"
	}

	code, err := a.runAmazonQ(ctx, "Generate "+language+" code for: "+prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt":   prompt,
		"language": language,
		"code":     code,
	}, nil
}

// explain explains code by exec-ing the real Amazon Q CLI.
func (a *AmazonQ) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	explanation, err := a.runAmazonQ(ctx, "Explain the following code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":        code,
		"explanation": explanation,
	}, nil
}

// transform transforms code by exec-ing the real Amazon Q CLI.
func (a *AmazonQ) transform(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	target, _ := params["target"].(string)

	if code == "" || target == "" {
		return nil, fmt.Errorf("code and target required")
	}

	transformed, err := a.runAmazonQ(ctx, "Transform the following code to "+target+":\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":        code,
		"target":      target,
		"transformed": transformed,
	}, nil
}

// scan scans for security issues by exec-ing the real Amazon Q CLI.
func (a *AmazonQ) scan(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	scan, err := a.runAmazonQ(ctx, "Perform a security scan of the following code and report findings:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code": code,
		"scan": scan,
	}, nil
}

// docs generates documentation by exec-ing the real Amazon Q CLI.
func (a *AmazonQ) docs(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	docs, err := a.runAmazonQ(ctx, "Generate documentation for the following code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code": code,
		"docs": docs,
	}, nil
}

// status returns status
func (a *AmazonQ) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":         a.IsAvailable(),
		"region":            a.config.Region,
		"transform_enabled": a.config.EnableTransform,
	}, nil
}

// IsAvailable checks availability
func (a *AmazonQ) IsAvailable() bool {
	return a.config.AWSProfile != ""
}

var _ agents.AgentIntegration = (*AmazonQ)(nil)
