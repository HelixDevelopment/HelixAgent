// Package cody provides Sourcegraph Cody agent integration.
// Cody: AI coding assistant with codebase intelligence.
package cody

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// codyBinary is the Sourcegraph Cody CLI executable looked up on PATH.
// Overridable in tests via the CODY_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff).
const codyBinary = "cody"

// getCodyBinOverride returns the test-only cody binary override, if set.
func getCodyBinOverride() string { return os.Getenv("CODY_BIN") }

// Cody provides Sourcegraph Cody integration
type Cody struct {
	*base.BaseIntegration
	config   *Config
	snippets []Snippet
}

// Config holds Cody configuration
type Config struct {
	base.BaseConfig
	SourcegraphURL string
	AccessToken    string
	Model          string
}

// Snippet represents a code snippet
type Snippet struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	File        string `json:"file"`
	Language    string `json:"language"`
	Description string `json:"description"`
}

// New creates a new Cody integration
func New() *Cody {
	info := agents.AgentInfo{
		Type:        agents.TypeCody,
		Name:        "Cody",
		Description: "AI coding assistant with codebase intelligence",
		Vendor:      "Sourcegraph",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_intelligence",
			"codebase_search",
			"chat",
			"code_explanation",
			"code_generation",
			"code_review",
			"symbol_search",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Cody{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			SourcegraphURL: "https://sourcegraph.com",
			Model:          "anthropic/claude-3-sonnet",
		},
		snippets: make([]Snippet, 0),
	}
}

// Initialize initializes Cody
func (c *Cody) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return c.loadSnippets()
}

// loadSnippets loads code snippets
func (c *Cody) loadSnippets() error {
	snippetsPath := filepath.Join(c.GetWorkDir(), "snippets.json")

	if _, err := os.Stat(snippetsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(snippetsPath)
	if err != nil {
		return fmt.Errorf("read snippets: %w", err)
	}

	return json.Unmarshal(data, &c.snippets)
}

// saveSnippets saves code snippets
func (c *Cody) saveSnippets() error {
	snippetsPath := filepath.Join(c.GetWorkDir(), "snippets.json")
	data, err := json.MarshalIndent(c.snippets, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snippets: %w", err)
	}
	return os.WriteFile(snippetsPath, data, 0644)
}

// Execute executes a command
func (c *Cody) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return c.chat(ctx, params)
	case "explain":
		return c.explain(ctx, params)
	case "generate":
		return c.generate(ctx, params)
	case "search":
		return c.search(ctx, params)
	case "review":
		return c.review(ctx, params)
	case "edit":
		return c.edit(ctx, params)
	case "symbol":
		return c.symbol(ctx, params)
	case "save_snippet":
		return c.saveSnippet(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveCodyBinary locates the cody CLI executable. Tests may inject a fake
// binary via the CODY_BIN environment variable (absolute path); otherwise the
// real `cody` command is resolved on PATH. Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001).
func (c *Cody) resolveCodyBinary() (string, error) {
	if bin := getCodyBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("cody binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(codyBinary)
	if err != nil {
		return "", fmt.Errorf("cody CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runCody invokes the Sourcegraph Cody CLI non-interactively and returns its
// textual output. The non-interactive form is `cody chat -m "<message>"`
// (optionally `--stdin`). The model is passed via `--model` when configured.
// Per CONST-035 the SRC_ACCESS_TOKEN / SRC_ENDPOINT credentials are forwarded
// from config when present; the CLI reads them from the environment.
func (c *Cody) runCody(ctx context.Context, message string) (string, error) {
	bin, err := c.resolveCodyBinary()
	if err != nil {
		return "", err
	}

	args := []string{"chat", "-m", message}
	if c.config != nil && c.config.Model != "" {
		args = append(args, "--model", c.config.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = c.GetWorkDir()
	cmd.Env = os.Environ()
	if c.config != nil {
		if c.config.AccessToken != "" {
			cmd.Env = append(cmd.Env, "SRC_ACCESS_TOKEN="+c.config.AccessToken)
		}
		if c.config.SourcegraphURL != "" {
			cmd.Env = append(cmd.Env, "SRC_ENDPOINT="+c.config.SourcegraphURL)
		}
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cody execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// chat performs chat by exec-ing the real cody CLI.
func (c *Cody) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	codeContext, _ := params["context"].(string)
	prompt := message
	if codeContext != "" {
		prompt = "Context:\n" + codeContext + "\n\n" + message
	}

	response, err := c.runCody(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message":  message,
		"context":  codeContext,
		"response": response,
		"model":    c.config.Model,
	}, nil
}

// explain explains code by exec-ing the real cody CLI.
func (c *Cody) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	explanation, err := c.runCody(ctx, "Explain the following code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":        code,
		"explanation": explanation,
		"model":       c.config.Model,
	}, nil
}

// generate generates code by exec-ing the real cody CLI.
func (c *Cody) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	language, _ := params["language"].(string)

	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	if language == "" {
		language = "go"
	}

	code, err := c.runCody(ctx, "Generate "+language+" code for: "+prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt":   prompt,
		"language": language,
		"code":     code,
		"model":    c.config.Model,
	}, nil
}

// search searches codebase by exec-ing the real cody CLI.
func (c *Cody) search(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query required")
	}

	results, err := c.runCody(ctx, "Search the codebase for: "+query)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"query":   query,
		"results": results,
		"model":   c.config.Model,
	}, nil
}

// review reviews code by exec-ing the real cody CLI.
func (c *Cody) review(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	review, err := c.runCody(ctx, "Review the following code and list issues:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":   code,
		"review": review,
		"model":  c.config.Model,
	}, nil
}

// edit edits code by exec-ing the real cody CLI.
func (c *Cody) edit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	instruction, _ := params["instruction"].(string)

	if file == "" || instruction == "" {
		return nil, fmt.Errorf("file and instruction required")
	}

	result, err := c.runCody(ctx, "Edit file "+file+" per instruction: "+instruction)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"file":        file,
		"instruction": instruction,
		"result":      result,
		"model":       c.config.Model,
	}, nil
}

// symbol searches for symbol by exec-ing the real cody CLI.
func (c *Cody) symbol(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	result, err := c.runCody(ctx, "Find the definition and references of symbol: "+name)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"symbol": name,
		"result": result,
		"model":  c.config.Model,
	}, nil
}

// saveSnippet saves a code snippet (local store; no LLM call).
func (c *Cody) saveSnippet(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	content, _ := params["content"].(string)
	description, _ := params["description"].(string)
	language, _ := params["language"].(string)

	if content == "" {
		return nil, fmt.Errorf("content required")
	}

	snippet := Snippet{
		ID:          fmt.Sprintf("snippet-%d", len(c.snippets)+1),
		Content:     content,
		Description: description,
		Language:    language,
	}

	c.snippets = append(c.snippets, snippet)

	if err := c.saveSnippets(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"snippet": snippet,
		"status":  "saved",
	}, nil
}

// IsAvailable checks availability
func (c *Cody) IsAvailable() bool {
	return c.config.AccessToken != ""
}

var _ agents.AgentIntegration = (*Cody)(nil)
