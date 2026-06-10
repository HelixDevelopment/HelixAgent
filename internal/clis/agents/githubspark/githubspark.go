// Package githubspark provides GitHub Spark agent integration.
// GitHub Spark: AI-powered micro-app builder for quick prototyping.
package githubspark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// GitHubSpark provides GitHub Spark integration
type GitHubSpark struct {
	*base.BaseIntegration
	config *Config
	sparks []Spark
}

// Config holds GitHub Spark configuration
type Config struct {
	base.BaseConfig
	GitHubToken       string
	AutoPublish       bool
	DefaultVisibility string
}

// Spark represents a Spark app
type Spark struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
	URL         string `json:"url"`
	Status      string `json:"status"`
}

// New creates a new GitHub Spark integration
func New() *GitHubSpark {
	info := agents.AgentInfo{
		Type:        agents.TypeGitHubSpark,
		Name:        "GitHub Spark",
		Description: "AI-powered micro-app builder",
		Vendor:      "GitHub",
		Version:     "1.0.0",
		Capabilities: []string{
			"micro_app_generation",
			"quick_prototyping",
			"github_integration",
			"auto_deploy",
			"component_reuse",
			"template_system",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GitHubSpark{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			AutoPublish:       false,
			DefaultVisibility: "public",
		},
		sparks: make([]Spark, 0),
	}
}

// Initialize initializes GitHub Spark
func (g *GitHubSpark) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return g.loadSparks()
}

// loadSparks loads spark list
func (g *GitHubSpark) loadSparks() error {
	sparksPath := filepath.Join(g.GetWorkDir(), "sparks.json")

	if _, err := os.Stat(sparksPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(sparksPath)
	if err != nil {
		return fmt.Errorf("read sparks: %w", err)
	}

	return json.Unmarshal(data, &g.sparks)
}

// saveSparks saves spark list
func (g *GitHubSpark) saveSparks() error {
	sparksPath := filepath.Join(g.GetWorkDir(), "sparks.json")
	data, err := json.MarshalIndent(g.sparks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sparks: %w", err)
	}
	return os.WriteFile(sparksPath, data, 0644)
}

// Execute executes a command
func (g *GitHubSpark) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "create":
		return g.createSpark(ctx, params)
	case "edit":
		return g.editSpark(ctx, params)
	case "publish":
		return g.publishSpark(ctx, params)
	case "list":
		return g.listSparks(ctx)
	case "clone":
		return g.cloneSpark(ctx, params)
	case "share":
		return g.shareSpark(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// createSpark creates a new Spark app
func (g *GitHubSpark) createSpark(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	description, _ := params["description"].(string)
	template, _ := params["template"].(string)
	if template == "" {
		template = "blank"
	}

	spark := Spark{
		ID:          fmt.Sprintf("spark-%d", len(g.sparks)+1),
		Name:        name,
		Description: description,
		Repository:  fmt.Sprintf("github.com/user/%s", name),
		URL:         fmt.Sprintf("https://spark.github.com/%s", name),
		Status:      "created",
	}

	g.sparks = append(g.sparks, spark)

	if err := g.saveSparks(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"spark":    spark,
		"template": template,
		"files": []string{
			"index.html",
			"style.css",
			"script.js",
		},
		"status": "created",
	}, nil
}

// errNoHeadlessCLI is returned by the AI-edit command: GitHub Spark is a
// web-only AI micro-app builder with NO official headless coding-agent CLI to
// exec. Per the 2026-06-10 blocked-agent research there is no scriptable
// single-prompt AI-edit invocation, so reporting fabricated "changes" would be a
// BLUFF-001 violation. The local spark registry CRUD (create/list/clone/publish/
// share) is genuine local state and remains functional; only the AI-prompt edit
// returns this error.
var errNoHeadlessCLI = fmt.Errorf(
	"github spark is a web-only AI builder with no headless CLI; prompt-driven edits " +
		"require the Spark web UI — this dispatch cannot apply AI changes without fabricating")

// editSpark validates input then returns an honest no-headless-CLI error rather
// than fabricating an AI-applied change list (BLUFF-001).
func (g *GitHubSpark) editSpark(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sparkID, _ := params["spark_id"].(string)
	prompt, _ := params["prompt"].(string)

	if sparkID == "" || prompt == "" {
		return nil, fmt.Errorf("spark_id and prompt required")
	}

	var spark *Spark
	for i := range g.sparks {
		if g.sparks[i].ID == sparkID {
			spark = &g.sparks[i]
			break
		}
	}

	if spark == nil {
		return nil, fmt.Errorf("spark not found: %s", sparkID)
	}

	return nil, errNoHeadlessCLI
}

// publishSpark publishes a Spark app
func (g *GitHubSpark) publishSpark(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sparkID, _ := params["spark_id"].(string)
	if sparkID == "" {
		return nil, fmt.Errorf("spark_id required")
	}

	var spark *Spark
	for i := range g.sparks {
		if g.sparks[i].ID == sparkID {
			spark = &g.sparks[i]
			break
		}
	}

	if spark == nil {
		return nil, fmt.Errorf("spark not found: %s", sparkID)
	}

	spark.Status = "published"

	if err := g.saveSparks(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"spark":  spark,
		"url":    spark.URL,
		"status": "published",
	}, nil
}

// listSparks lists all Spark apps
func (g *GitHubSpark) listSparks(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"sparks": g.sparks,
		"count":  len(g.sparks),
	}, nil
}

// cloneSpark clones a Spark app
func (g *GitHubSpark) cloneSpark(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sparkID, _ := params["spark_id"].(string)
	newName, _ := params["new_name"].(string)

	if sparkID == "" || newName == "" {
		return nil, fmt.Errorf("spark_id and new_name required")
	}

	var original *Spark
	for i := range g.sparks {
		if g.sparks[i].ID == sparkID {
			original = &g.sparks[i]
			break
		}
	}

	if original == nil {
		return nil, fmt.Errorf("spark not found: %s", sparkID)
	}

	newSpark := Spark{
		ID:          fmt.Sprintf("spark-%d", len(g.sparks)+1),
		Name:        newName,
		Description: original.Description,
		Repository:  fmt.Sprintf("github.com/user/%s", newName),
		URL:         fmt.Sprintf("https://spark.github.com/%s", newName),
		Status:      "created",
	}

	g.sparks = append(g.sparks, newSpark)

	if err := g.saveSparks(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"original": original,
		"clone":    newSpark,
		"status":   "cloned",
	}, nil
}

// shareSpark shares a Spark app
func (g *GitHubSpark) shareSpark(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sparkID, _ := params["spark_id"].(string)
	if sparkID == "" {
		return nil, fmt.Errorf("spark_id required")
	}

	var spark *Spark
	for i := range g.sparks {
		if g.sparks[i].ID == sparkID {
			spark = &g.sparks[i]
			break
		}
	}

	if spark == nil {
		return nil, fmt.Errorf("spark not found: %s", sparkID)
	}

	return map[string]interface{}{
		"spark":      spark,
		"share_url":  spark.URL,
		"embed_code": fmt.Sprintf("<iframe src=\"%s\"></iframe>", spark.URL),
	}, nil
}

// IsAvailable checks availability
func (g *GitHubSpark) IsAvailable() bool {
	return g.config.GitHubToken != ""
}

var _ agents.AgentIntegration = (*GitHubSpark)(nil)
